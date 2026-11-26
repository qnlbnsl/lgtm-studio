package machines_test

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

// keyOf is a false machine key made of one repeated byte.
func keyOf(b byte) []byte { return bytes.Repeat([]byte{b}, 32) }

// redeem uses a token with one key and one host name.
func (e *tokEnv) redeem(msg *linkv1.JoinToken, key []byte, host string) (machines.Joined, error) {
	return e.tokens.Redeem(context.Background(), machines.Redemption{
		TokenID: msg.GetTokenId(), Secret: msg.GetSecret(), PublicKey: key, HostName: host,
	})
}

func (e *tokEnv) mustRedeem(t *testing.T, msg *linkv1.JoinToken, key []byte, host string) machines.Joined {
	t.Helper()
	joined, err := e.redeem(msg, key, host)
	if err != nil {
		t.Fatalf("Redeem failed: %v", err)
	}
	return joined
}

func requireRefusal(t *testing.T, what string, err error, want machines.Refusal) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: Redeem succeeded, want the refusal %q", what, want)
		return
	}
	got, ok := machines.RefusalOf(err)
	if !ok {
		t.Errorf("%s: the error %v gives no refusal, want %q", what, err, want)
		return
	}
	if got != want {
		t.Errorf("%s: the refusal is %q, want %q", what, got, want)
	}
}

// expire moves the token's expiry into the past.
func (e *tokEnv) expire(t *testing.T, id string) {
	t.Helper()
	storetest.Exec(t, e.obs, `UPDATE client_join_tokens SET expires_at = clock_timestamp() - interval '1 second' WHERE id = $1`, id)
}

// revoke revokes the token and leaves its machine alone.
func (e *tokEnv) revoke(t *testing.T, id string) {
	t.Helper()
	err := e.write(t, func(ctx context.Context, tx *store.Tx) error {
		_, err := e.tokens.Revoke(ctx, tx, id)
		return err
	})
	if err != nil {
		t.Fatalf("Revoke failed: %v", err)
	}
}

func (e *tokEnv) machineCount(t *testing.T) int64 {
	t.Helper()
	return storetest.Scalar[int64](t, e.obs, "SELECT count(*) FROM machines")
}

// state is everything that a refused use must leave unchanged.
type state struct {
	machines int64
	tokens   int64
	feedHead int64
	row      *tokenRow
}

func (e *tokEnv) snapshot(t *testing.T, tokenID string) state {
	t.Helper()
	s := state{machines: e.machineCount(t), tokens: e.tokenCount(t), feedHead: e.feedHead(t)}
	if storetest.Scalar[int64](t, e.obs, "SELECT count(*) FROM client_join_tokens WHERE id = $1", tokenID) == 1 {
		row := e.row(t, tokenID)
		s.row = &row
	}
	return s
}

func (e *tokEnv) requireUnchanged(t *testing.T, what, tokenID string, before state) {
	t.Helper()
	if after := e.snapshot(t, tokenID); !reflect.DeepEqual(before, after) {
		t.Errorf("%s: the refusal changed the database (machines %d to %d, feed head %d to %d, token row changed: %v)",
			what, before.machines, after.machines, before.feedHead, after.feedHead, !reflect.DeepEqual(before.row, after.row))
	}
}

// holdToken locks the token's row in a transaction of the test, until release is called.
// Every use that starts meanwhile waits on the lock. The release function is idempotent.
func (e *tokEnv) holdToken(t *testing.T, id string) (tx pgx.Tx, release func()) {
	t.Helper()
	tx, err := e.obs.Begin(t.Context())
	if err != nil {
		t.Fatalf("beginning the hold failed: %v", err)
	}
	if _, err := tx.Exec(t.Context(), `SELECT 1 FROM client_join_tokens WHERE id = $1 FOR UPDATE`, id); err != nil {
		t.Fatalf("locking the token row failed: %v", err)
	}
	done := false
	release = func() {
		if done {
			return
		}
		done = true
		if err := tx.Commit(context.Background()); err != nil {
			t.Errorf("releasing the hold failed: %v", err)
		}
	}
	t.Cleanup(release)
	return tx, release
}

// waitForWaiters waits until n backends wait on a lock.
func waitForWaiters(t *testing.T, tx pgx.Tx, n int64) {
	t.Helper()
	storetest.Poll(t, "the uses to wait on the token's row", func() bool {
		var waiting int64
		// Inside a transaction PostgreSQL keeps one activity snapshot, so clear it on every poll.
		if _, err := tx.Exec(t.Context(), `SELECT pg_stat_clear_snapshot()`); err != nil {
			return false
		}
		err := tx.QueryRow(t.Context(), `
SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(&waiting)
		return err == nil && waiting >= n
	})
}

type redeemResult struct {
	joined machines.Joined
	err    error
}

func (e *tokEnv) startRedeem(t *testing.T, msg *linkv1.JoinToken, key []byte, host string) *storetest.Task[redeemResult] {
	t.Helper()
	return storetest.Start(t, func() redeemResult {
		joined, err := e.redeem(msg, key, host)
		return redeemResult{joined, err}
	})
}

func TestRedeemCreatesWaitingMachine(t *testing.T) {
	e := newTokEnv(t, loopbackURL)
	tok, msg := e.create(t, machines.TokenOptions{HostsServer: true})
	key := keyOf(0x31)

	joined := e.mustRedeem(t, msg, key, "box")
	if joined.Replay {
		t.Error("a first use reports a replay")
	}
	if !uuidV7.MatchString(joined.MachineID) {
		t.Fatalf("the machine id is %q, want a version 7 identifier", joined.MachineID)
	}

	m := e.mustGet(t, joined.MachineID)
	if m.Approval != machines.ApprovalWaiting || m.StateRevision != 1 {
		t.Errorf("the machine is %q at revision %d, want waiting at revision 1", m.Approval, m.StateRevision)
	}
	if m.HostName != "box" || m.Name != "" {
		t.Errorf("the host name is %q and the name %q, want box and none", m.HostName, m.Name)
	}
	if m.TokenID != tok.ID || !m.HostsServer || !bytes.Equal(m.PublicKey, key) {
		t.Errorf("the machine keeps token %q, hosts_server %v and key %x, want %q, true and %x", m.TokenID, m.HostsServer, m.PublicKey, tok.ID, key)
	}

	row := e.row(t, tok.ID)
	if !bytes.Equal(row.usedKey, key) || row.usedBy == nil || *row.usedBy != joined.MachineID || row.usedAt == nil {
		t.Errorf("the token row holds used_key %x, used_by %v and used_at %v, want the key, the machine and a time", row.usedKey, row.usedBy, row.usedAt)
	}

	// The machine and the token reach the feed, the machine as saved.
	var machineSaved, tokenSaved bool
	for _, entry := range e.feed(t) {
		machineSaved = machineSaved || (entry.Topic == "machines" && entry.Key == joined.MachineID && entry.Change == "saved")
		tokenSaved = tokenSaved || (entry.Topic == "client_join_tokens" && entry.Key == tok.ID && entry.Change == "saved")
	}
	if !machineSaved {
		t.Error("the feed holds no saved row of the new machine")
	}
	if !tokenSaved {
		t.Error("the feed holds no saved row of the token")
	}

	// A token made without the mark gives a machine without it.
	_, plain := e.create(t, machines.TokenOptions{})
	other := e.mustRedeem(t, plain, keyOf(0x32), "plain")
	if e.mustGet(t, other.MachineID).HostsServer {
		t.Error("a token without the mark gave a machine that hosts the server")
	}
}

func TestRedeemReplay(t *testing.T) {
	e := newTokEnv(t, loopbackURL)
	tok, msg := e.create(t, machines.TokenOptions{})
	key := keyOf(0x31)
	first := e.mustRedeem(t, msg, key, "box")

	before := e.snapshot(t, tok.ID)
	again := e.mustRedeem(t, msg, key, "other")
	if !again.Replay || again.MachineID != first.MachineID {
		t.Errorf("a replay answered %+v, want machine %q with Replay true", again, first.MachineID)
	}
	e.requireUnchanged(t, "replay", tok.ID, before)
	if n := e.machineCount(t); n != 1 {
		t.Errorf("%d machines exist after a replay, want 1", n)
	}
	if m := e.mustGet(t, first.MachineID); m.HostName != "box" {
		t.Errorf("the host name is %q after a replay with another one, want box", m.HostName)
	}

	// A replay never reads the host name, so a broken one is no error.
	broken := e.mustRedeem(t, msg, key, strings.Repeat("h", 256))
	if !broken.Replay || broken.MachineID != first.MachineID {
		t.Errorf("a replay with a 256-byte host name answered %+v, want the first machine", broken)
	}

	// The same token with another key is refused, and the first key still gets the same answer.
	_, err := e.redeem(msg, keyOf(0x33), "box")
	requireRefusal(t, "another key after a use", err, machines.RefusalUsed)
	third := e.mustRedeem(t, msg, key, "box")
	if !third.Replay || third.MachineID != first.MachineID {
		t.Errorf("the first key answered %+v after another key was refused, want the first machine", third)
	}
	e.requireUnchanged(t, "refused key", tok.ID, before)
}

func TestRedeemRefusals(t *testing.T) {
	key := keyOf(0x31)
	type scenario struct {
		name string
		want machines.Refusal
		// prepare changes the token and returns the redemption to try.
		prepare func(t *testing.T, e *tokEnv, tok machines.NewToken, msg *linkv1.JoinToken) machines.Redemption
	}
	use := func(msg *linkv1.JoinToken, key []byte) machines.Redemption {
		return machines.Redemption{TokenID: msg.GetTokenId(), Secret: msg.GetSecret(), PublicKey: key, HostName: "box"}
	}
	wrongSecret := func(msg *linkv1.JoinToken) []byte {
		secret := bytes.Clone(msg.GetSecret())
		secret[0] ^= 0xff
		return secret
	}
	scenarios := []scenario{
		{"unknown identifier", machines.RefusalUnknown, func(t *testing.T, e *tokEnv, tok machines.NewToken, msg *linkv1.JoinToken) machines.Redemption {
			r := use(msg, key)
			r.TokenID = randomID(t)
			return r
		}},
		{"wrong secret", machines.RefusalUnknown, func(t *testing.T, e *tokEnv, tok machines.NewToken, msg *linkv1.JoinToken) machines.Redemption {
			r := use(msg, key)
			r.Secret = wrongSecret(msg)
			return r
		}},
		{"wrong secret on an expired token", machines.RefusalUnknown, func(t *testing.T, e *tokEnv, tok machines.NewToken, msg *linkv1.JoinToken) machines.Redemption {
			e.expire(t, tok.ID)
			r := use(msg, key)
			r.Secret = wrongSecret(msg)
			return r
		}},
		{"wrong secret on a revoked token", machines.RefusalUnknown, func(t *testing.T, e *tokEnv, tok machines.NewToken, msg *linkv1.JoinToken) machines.Redemption {
			e.revoke(t, tok.ID)
			r := use(msg, key)
			r.Secret = wrongSecret(msg)
			return r
		}},
		{"wrong secret on a used token", machines.RefusalUnknown, func(t *testing.T, e *tokEnv, tok machines.NewToken, msg *linkv1.JoinToken) machines.Redemption {
			e.mustRedeem(t, msg, key, "box")
			r := use(msg, key)
			r.Secret = wrongSecret(msg)
			return r
		}},
		{"expired", machines.RefusalExpired, func(t *testing.T, e *tokEnv, tok machines.NewToken, msg *linkv1.JoinToken) machines.Redemption {
			e.expire(t, tok.ID)
			return use(msg, key)
		}},
		{"expired at the very instant", machines.RefusalExpired, func(t *testing.T, e *tokEnv, tok machines.NewToken, msg *linkv1.JoinToken) machines.Redemption {
			// "At or before the clock" is expired: an expiry of now is no longer valid.
			storetest.Exec(t, e.obs, `UPDATE client_join_tokens SET expires_at = clock_timestamp() WHERE id = $1`, tok.ID)
			return use(msg, key)
		}},
		{"revoked", machines.RefusalRevoked, func(t *testing.T, e *tokEnv, tok machines.NewToken, msg *linkv1.JoinToken) machines.Redemption {
			e.revoke(t, tok.ID)
			return use(msg, key)
		}},
		{"revoked and expired", machines.RefusalExpired, func(t *testing.T, e *tokEnv, tok machines.NewToken, msg *linkv1.JoinToken) machines.Redemption {
			e.revoke(t, tok.ID)
			e.expire(t, tok.ID)
			return use(msg, key)
		}},
		{"used with another key", machines.RefusalUsed, func(t *testing.T, e *tokEnv, tok machines.NewToken, msg *linkv1.JoinToken) machines.Redemption {
			e.mustRedeem(t, msg, key, "box")
			return use(msg, keyOf(0x32))
		}},
		{"used with another key and expired", machines.RefusalExpired, func(t *testing.T, e *tokEnv, tok machines.NewToken, msg *linkv1.JoinToken) machines.Redemption {
			e.mustRedeem(t, msg, key, "box")
			e.expire(t, tok.ID)
			return use(msg, keyOf(0x32))
		}},
		{"used with another key and revoked", machines.RefusalRevoked, func(t *testing.T, e *tokEnv, tok machines.NewToken, msg *linkv1.JoinToken) machines.Redemption {
			e.mustRedeem(t, msg, key, "box")
			e.revoke(t, tok.ID)
			return use(msg, keyOf(0x32))
		}},
		{"machine removed", machines.RefusalMachineRemoved, func(t *testing.T, e *tokEnv, tok machines.NewToken, msg *linkv1.JoinToken) machines.Redemption {
			joined := e.mustRedeem(t, msg, key, "box")
			if err := e.write(t, func(ctx context.Context, tx *store.Tx) error { return e.reg.Delete(ctx, tx, joined.MachineID) }); err != nil {
				t.Fatalf("deleting the machine failed: %v", err)
			}
			return use(msg, key)
		}},
		{"replay after expiry", machines.RefusalExpired, func(t *testing.T, e *tokEnv, tok machines.NewToken, msg *linkv1.JoinToken) machines.Redemption {
			e.mustRedeem(t, msg, key, "box")
			e.expire(t, tok.ID)
			return use(msg, key)
		}},
		{"replay after revocation of an approved machine", machines.RefusalRevoked, func(t *testing.T, e *tokEnv, tok machines.NewToken, msg *linkv1.JoinToken) machines.Redemption {
			joined := e.mustRedeem(t, msg, key, "box")
			if err := e.write(t, func(ctx context.Context, tx *store.Tx) error {
				return e.reg.SetApproved(ctx, tx, joined.MachineID, "Box")
			}); err != nil {
				t.Fatalf("approving the machine failed: %v", err)
			}
			e.revoke(t, tok.ID)
			return use(msg, key)
		}},
		{"removed machine and expiry", machines.RefusalExpired, func(t *testing.T, e *tokEnv, tok machines.NewToken, msg *linkv1.JoinToken) machines.Redemption {
			joined := e.mustRedeem(t, msg, key, "box")
			if err := e.write(t, func(ctx context.Context, tx *store.Tx) error { return e.reg.Delete(ctx, tx, joined.MachineID) }); err != nil {
				t.Fatalf("deleting the machine failed: %v", err)
			}
			e.expire(t, tok.ID)
			return use(msg, key)
		}},
		{"expired token with a broken host name", machines.RefusalExpired, func(t *testing.T, e *tokEnv, tok machines.NewToken, msg *linkv1.JoinToken) machines.Redemption {
			// Only step 6 reads the host name, so a refusal comes first.
			e.expire(t, tok.ID)
			r := use(msg, key)
			r.HostName = strings.Repeat("h", 256)
			return r
		}},
	}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			e := newTokEnv(t, loopbackURL)
			tok, msg := e.create(t, machines.TokenOptions{})
			in := sc.prepare(t, e, tok, msg)
			before := e.snapshot(t, tok.ID)
			_, err := e.tokens.Redeem(t.Context(), in)
			requireRefusal(t, sc.name, err, sc.want)
			e.requireUnchanged(t, sc.name, tok.ID, before)
		})
	}
}

func TestRedeemHostNameRules(t *testing.T) {
	cases := []struct {
		name, host string
		ok         bool
	}{
		{"255 bytes", strings.Repeat("h", 255), true},
		{"256 bytes", strings.Repeat("h", 256), false},
		{"a zero byte", "bo\x00x", false},
		{"invalid UTF-8", "bo\xffx", false},
		{"empty", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newTokEnv(t, loopbackURL)
			tok, msg := e.create(t, machines.TokenOptions{})
			before := e.snapshot(t, tok.ID)
			joined, err := e.redeem(msg, keyOf(0x31), c.host)
			if c.ok {
				if err != nil || !uuidV7.MatchString(joined.MachineID) {
					t.Fatalf("a host name of %s answered %+v with %v, want a machine", c.name, joined, err)
				}
				return
			}
			if !errors.Is(err, machines.ErrInvalidHostName) {
				t.Fatalf("a host name of %s answered %v, want ErrInvalidHostName", c.name, err)
			}
			if _, isRefusal := machines.RefusalOf(err); isRefusal {
				t.Error("a broken host name gives a refusal reason, want none")
			}
			e.requireUnchanged(t, c.name, tok.ID, before)
			// The token is not spent: a good host name works at once.
			if again := e.mustRedeem(t, msg, keyOf(0x31), "box"); again.Replay || !uuidV7.MatchString(again.MachineID) {
				t.Errorf("the next use answered %+v, want a first use", again)
			}
		})
	}
}

func TestRedeemConcurrentUses(t *testing.T) {
	t.Run("two keys", func(t *testing.T) {
		e := newTokEnv(t, loopbackURL)
		tok, msg := e.create(t, machines.TokenOptions{})
		tx, release := e.holdToken(t, tok.ID)
		keys := [][]byte{keyOf(0x31), keyOf(0x32)}
		tasks := []*storetest.Task[redeemResult]{e.startRedeem(t, msg, keys[0], "one"), e.startRedeem(t, msg, keys[1], "two")}
		waitForWaiters(t, tx, 2)
		release()
		results := []redeemResult{tasks[0].Wait(t), tasks[1].Wait(t)}

		winners := -1
		for i, r := range results {
			if r.err == nil {
				if winners != -1 {
					t.Fatalf("both keys joined: %+v and %+v", results[0].joined, results[1].joined)
				}
				winners = i
			}
		}
		if winners == -1 {
			t.Fatalf("no key joined: %v and %v", results[0].err, results[1].err)
		}
		requireRefusal(t, "the loser", results[1-winners].err, machines.RefusalUsed)
		if n := e.machineCount(t); n != 1 {
			t.Errorf("%d machines exist, want 1", n)
		}
		if row := e.row(t, tok.ID); !bytes.Equal(row.usedKey, keys[winners]) {
			t.Errorf("the token is bound to key %x, want the winner's %x", row.usedKey, keys[winners])
		}
	})

	t.Run("identical uses", func(t *testing.T) {
		e := newTokEnv(t, loopbackURL)
		tok, msg := e.create(t, machines.TokenOptions{})
		tx, release := e.holdToken(t, tok.ID)
		tasks := []*storetest.Task[redeemResult]{e.startRedeem(t, msg, keyOf(0x31), "one"), e.startRedeem(t, msg, keyOf(0x31), "one")}
		waitForWaiters(t, tx, 2)
		release()
		a, b := tasks[0].Wait(t), tasks[1].Wait(t)
		if a.err != nil || b.err != nil {
			t.Fatalf("identical uses failed: %v and %v", a.err, b.err)
		}
		if a.joined.MachineID != b.joined.MachineID || !uuidV7.MatchString(a.joined.MachineID) {
			t.Errorf("identical uses answered %q and %q, want one machine", a.joined.MachineID, b.joined.MachineID)
		}
		if a.joined.Replay == b.joined.Replay {
			t.Errorf("Replay is %v and %v, want exactly one first use", a.joined.Replay, b.joined.Replay)
		}
		if n := e.machineCount(t); n != 1 {
			t.Errorf("%d machines exist, want 1", n)
		}
	})

	t.Run("a use that waits past the expiry", func(t *testing.T) {
		e := newTokEnv(t, loopbackURL)
		tok, msg := e.create(t, machines.TokenOptions{})
		storetest.Exec(t, e.obs, `UPDATE client_join_tokens SET expires_at = clock_timestamp() + interval '2 seconds' WHERE id = $1`, tok.ID)
		first := e.mustRedeem(t, msg, keyOf(0x31), "box")

		tx, release := e.holdToken(t, tok.ID)
		second := e.startRedeem(t, msg, keyOf(0x31), "box")
		waitForWaiters(t, tx, 1)
		storetest.Poll(t, "the expiry to pass", func() bool {
			var passed bool
			err := tx.QueryRow(t.Context(), `SELECT clock_timestamp() > expires_at FROM client_join_tokens WHERE id = $1`, tok.ID).Scan(&passed)
			return err == nil && passed
		})
		release()
		r := second.Wait(t)
		requireRefusal(t, "the use that waited", r.err, machines.RefusalExpired)
		if n := e.machineCount(t); n != 1 || e.mustGet(t, first.MachineID).ID == "" {
			t.Errorf("the first machine or the count changed: %d machines", n)
		}
	})
}

func TestRedeemRollsBack(t *testing.T) {
	e := newTokEnv(t, loopbackURL)
	tok, msg := e.create(t, machines.TokenOptions{})
	storetest.Exec(t, e.obs, `
CREATE FUNCTION fail_machine_insert() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'the test refuses this insert'; END $$`)
	storetest.Exec(t, e.obs, `CREATE TRIGGER fail_machine_insert BEFORE INSERT ON machines FOR EACH ROW EXECUTE FUNCTION fail_machine_insert()`)

	before := e.snapshot(t, tok.ID)
	_, err := e.redeem(msg, keyOf(0x31), "box")
	if err == nil {
		t.Fatal("Redeem succeeded although the insert into machines fails")
	}
	if reason, ok := machines.RefusalOf(err); ok {
		t.Errorf("a database failure gives the refusal %q, want none", reason)
	}
	e.requireUnchanged(t, "failed insert", tok.ID, before)
	if row := e.row(t, tok.ID); row.usedKey != nil || row.usedBy != nil || row.usedAt != nil {
		t.Errorf("the token row holds used_key %x, used_by %v, used_at %v after a failed use, want all empty", row.usedKey, row.usedBy, row.usedAt)
	}

	storetest.Exec(t, e.obs, `DROP TRIGGER fail_machine_insert ON machines`)
	if joined := e.mustRedeem(t, msg, keyOf(0x31), "box"); joined.Replay || !uuidV7.MatchString(joined.MachineID) {
		t.Errorf("the same request after the failure answered %+v, want a first use", joined)
	}
}

func TestRedeemStoreClosed(t *testing.T) {
	e := newTokEnv(t, loopbackURL)
	_, msg := e.create(t, machines.TokenOptions{})
	e.s.Close()
	_, err := e.redeem(msg, keyOf(0x31), "box")
	if err == nil {
		t.Fatal("Redeem succeeded on a closed store")
	}
	if reason, ok := machines.RefusalOf(err); ok {
		t.Errorf("a closed store gives the refusal %q, want none", reason)
	}
}

func TestRefusalOf(t *testing.T) {
	want := map[error]machines.Refusal{
		machines.ErrTokenUnknown:   machines.RefusalUnknown,
		machines.ErrTokenExpired:   machines.RefusalExpired,
		machines.ErrTokenRevoked:   machines.RefusalRevoked,
		machines.ErrTokenUsed:      machines.RefusalUsed,
		machines.ErrMachineRemoved: machines.RefusalMachineRemoved,
	}
	words := map[machines.Refusal]string{
		machines.RefusalUnknown: "token_unknown", machines.RefusalExpired: "token_expired", machines.RefusalRevoked: "token_revoked",
		machines.RefusalUsed: "token_used", machines.RefusalMachineRemoved: "machine_removed",
	}
	sentences := map[machines.Refusal]string{
		machines.RefusalUnknown:        "The server does not know this join token.",
		machines.RefusalExpired:        "This join token has expired. Make a new one.",
		machines.RefusalRevoked:        "This join token was revoked. Make a new one.",
		machines.RefusalUsed:           "This join token was already used. Make a new one.",
		machines.RefusalMachineRemoved: "The machine that used this join token was removed. Make a new token.",
	}
	for err, refusal := range want {
		got, ok := machines.RefusalOf(err)
		if !ok || got != refusal {
			t.Errorf("RefusalOf(%v) is %q, %v, want %q", err, got, ok, refusal)
		}
		if string(refusal) != words[refusal] {
			t.Errorf("the refusal %q has the word %q, want %q", refusal, string(refusal), words[refusal])
		}
		if err.Error() != sentences[refusal] {
			t.Errorf("the sentence of %q is %q, want %q", refusal, err.Error(), sentences[refusal])
		}
	}
	for _, other := range []error{nil, errors.New("another"), machines.ErrInvalidHostName, machines.ErrNoMachine} {
		if got, ok := machines.RefusalOf(other); ok {
			t.Errorf("RefusalOf(%v) gives the refusal %q, want none", other, got)
		}
	}
}

// TestSecretCompareIsConstantTime reads the source of join.go. A runtime test cannot see the
// difference between bytes.Equal and subtle.ConstantTimeCompare, so this test reads the code.
func TestSecretCompareIsConstantTime(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "join.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing join.go failed: %v", err)
	}
	render := func(n ast.Node) string {
		var b strings.Builder
		_ = printer.Fprint(&b, fset, n)
		return b.String()
	}
	secretWords := []string{"verifier", "digest", "secret", "sha256", "hash", "sum"}
	mentionsSecret := func(n ast.Node) bool {
		text := strings.ToLower(render(n))
		for _, w := range secretWords {
			if strings.Contains(text, w) {
				return true
			}
		}
		return false
	}

	constantTime := 0
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			switch name := render(n.Fun); name {
			case "subtle.ConstantTimeCompare":
				if len(n.Args) == 2 && mentionsSecret(n) {
					constantTime++
				}
			case "bytes.Equal", "bytes.Compare", "slices.Equal", "slices.Compare", "reflect.DeepEqual", "strings.Compare", "strings.EqualFold":
				t.Errorf("join.go calls %s at %s, which does not compare in constant time", name, fset.Position(n.Pos()))
			}
		case *ast.BinaryExpr:
			if (n.Op == token.EQL || n.Op == token.NEQ || n.Op == token.LSS || n.Op == token.GTR) && (mentionsSecret(n.X) || mentionsSecret(n.Y)) {
				t.Errorf("join.go compares %q with %s at %s, which does not compare in constant time", render(n), n.Op, fset.Position(n.Pos()))
			}
		}
		return true
	})
	if constantTime == 0 {
		t.Error("join.go does not call subtle.ConstantTimeCompare on the token's verifiers")
	}
}
