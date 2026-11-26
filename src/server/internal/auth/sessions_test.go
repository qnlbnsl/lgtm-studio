package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

const (
	sessionGateKey int64 = 841941100951
	// settleTime is how long a test waits before it checks that something did not happen.
	// A short wait can only hide a failure, never invent one.
	settleTime = 200 * time.Millisecond
	// promptly bounds the wait for something that must not wait for a held derivation.
	promptly = 5 * time.Second
)

// deriveProbe wraps Sessions.deriveKey to count work and to hold chosen derivations.
type deriveProbe struct {
	mu       sync.Mutex
	started  int
	finished int
	bySalt   map[[16]byte]*deriveGate
	next     *deriveGate
}

type deriveGate struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
	hit     sync.Once
}

func newDeriveGate(t *testing.T) *deriveGate {
	g := &deriveGate{entered: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(g.open)
	return g
}

func (g *deriveGate) open() { g.once.Do(func() { close(g.release) }) }

func installProbe(s *Sessions) *deriveProbe {
	p := &deriveProbe{bySalt: map[[16]byte]*deriveGate{}}
	real := s.deriveKey
	s.deriveKey = func(pw Password, salt [16]byte) ([32]byte, error) {
		p.mu.Lock()
		p.started++
		g := p.bySalt[salt]
		if g == nil && p.next != nil {
			g, p.next = p.next, nil
		}
		p.mu.Unlock()
		if g != nil {
			g.hit.Do(func() { close(g.entered) })
			<-g.release
		}
		key, err := real(pw, salt)
		p.mu.Lock()
		p.finished++
		p.mu.Unlock()
		return key, err
	}
	return p
}

func (p *deriveProbe) holdSalt(g *deriveGate, salt [16]byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.bySalt[salt] = g
}

func (p *deriveProbe) holdNext(g *deriveGate) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.next = g
}

func (p *deriveProbe) counts() (started, finished int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.started, p.finished
}

func issueN(t *testing.T, s *Sessions, n int) ([]string, []Verifier) {
	t.Helper()
	tokens := make([]string, n)
	verifiers := make([]Verifier, n)
	for i := range tokens {
		token, err := s.Issue(t.Context())
		if err != nil {
			t.Fatalf("issuing token %d failed: %v", i, err)
		}
		v, ok := TokenVerifier(token)
		if !ok {
			t.Fatalf("token %d (%q) is not canonical", i, token)
		}
		tokens[i], verifiers[i] = token, v
	}
	return tokens, verifiers
}

func checkInRead(s *Sessions, st *store.Store, ctx context.Context, v Verifier) error {
	return st.Read(ctx, func(ctx context.Context, tx *store.ReadTx) error {
		return s.CheckInRead(ctx, tx, v)
	})
}

func requireNoSession(t *testing.T, what string, err error) {
	t.Helper()
	if !errors.Is(err, ErrNoSession) {
		t.Fatalf("%s: got %v, want ErrNoSession", what, err)
	}
}

func requireLive(t *testing.T, f sessionFixture, s *Sessions, what string, v Verifier) {
	t.Helper()
	if err := s.Check(t.Context(), v); err != nil {
		t.Fatalf("%s: Check refused a live session: %v", what, err)
	}
	if err := checkInRead(s, f.s, t.Context(), v); err != nil {
		t.Fatalf("%s: CheckInRead refused a live session: %v", what, err)
	}
}

func requireGone(t *testing.T, f sessionFixture, s *Sessions, what string, v Verifier) {
	t.Helper()
	requireNoSession(t, what+" (Check)", s.Check(t.Context(), v))
	requireNoSession(t, what+" (CheckInRead)", checkInRead(s, f.s, t.Context(), v))
}

type issued struct {
	token string
	err   error
}

func startIssue(t *testing.T, s *Sessions, ctx context.Context) *storetest.Task[issued] {
	t.Helper()
	return storetest.Start(t, func() issued {
		token, err := s.Issue(ctx)
		return issued{token, err}
	})
}

func startCheck(t *testing.T, s *Sessions, ctx context.Context, v Verifier) *storetest.Task[error] {
	t.Helper()
	return storetest.Start(t, func() error { return s.Check(ctx, v) })
}

// finishesPromptly fails the test unless the task is done within the bound.
func finishesPromptly[V any](t *testing.T, what string, task *storetest.Task[V]) V {
	t.Helper()
	select {
	case <-task.Done():
		return task.Result()
	case <-time.After(promptly):
		t.Fatalf("%s did not finish within %v", what, promptly)
		var zero V
		return zero
	}
}

func TestSessionLifecycle(t *testing.T) {
	pw := mustPassword(t, "lifecycle password")
	f := newSessionFixture(t, fixtureNow)
	s := NewSessions(f.s, pw)

	tokens, verifiers := issueN(t, s, SessionLimit+1)
	seen := map[string]bool{}
	for i, token := range tokens {
		if len(token) != 43 {
			t.Fatalf("token %d has %d characters, want 43", i, len(token))
		}
		raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
		if err != nil || len(raw) != 32 {
			t.Fatalf("token %d is not 32 bytes of unpadded URL-safe base64: %v", i, err)
		}
		if seen[token] || bytes.Equal(raw, make([]byte, 32)) {
			t.Fatalf("token %d repeats or is all zero", i)
		}
		seen[token] = true
		want := Verifier(sha256.Sum256(raw))
		if verifiers[i] != want {
			t.Fatalf("TokenVerifier(token %d) is not the SHA-256 of the decoded token", i)
		}
	}

	// Exactly the eight newest sessions remain, and their rows carry independent bindings.
	requireStored(t, f.obs, "after nine sign-ins", verifiers[1:]...)
	var previous time.Time
	for i, v := range verifiers[1:] {
		row, ok := readRow(t, f.obs, v)
		if !ok {
			t.Fatalf("row %d is missing", i+1)
		}
		if row.createdAt.Before(fixtureNow) || !row.createdAt.After(previous) {
			t.Fatalf("row %d was created at %v: want a time at or after the clock and after the previous row (%v)", i+1, row.createdAt, previous)
		}
		previous = row.createdAt
		want, err := bindPassword(pw, row.salt, row.verifier, row.createdAt)
		if err != nil || !equalDigest(row.binding, want) {
			t.Fatalf("row %d does not carry the binding of the item 2 layout (%v)", i+1, err)
		}
	}
	requireGone(t, f, s, "the evicted first token", verifiers[0])
	for i := 1; i < len(verifiers); i++ {
		requireLive(t, f, s, fmt.Sprintf("token %d", i), verifiers[i])
	}

	// Revoking one session leaves the others alone and cannot repeat.
	if err := s.Revoke(t.Context(), verifiers[4]); err != nil {
		t.Fatalf("Revoke of a live session failed: %v", err)
	}
	requireGone(t, f, s, "the revoked token", verifiers[4])
	requireNoSession(t, "revoking again", s.Revoke(t.Context(), verifiers[4]))
	for i := 1; i < len(verifiers); i++ {
		if i != 4 {
			requireLive(t, f, s, fmt.Sprintf("token %d after the revoke", i), verifiers[i])
		}
	}

	// The newest session is live one microsecond before its expiry and not at it.
	newest := verifiers[len(verifiers)-1]
	row, _ := readRow(t, f.obs, newest)
	expiry := row.createdAt.Add(sessionLifetime)
	f.setClock(expiry.Add(-time.Microsecond))
	requireLive(t, f, s, "one microsecond before the expiry", newest)
	f.setClock(expiry)
	requireGone(t, f, s, "at the expiry", newest)
	requireNoSession(t, "revoking at the expiry", s.Revoke(t.Context(), newest))
	f.setClock(expiry.Add(time.Microsecond))
	requireGone(t, f, s, "one microsecond after the expiry", newest)
}

func TestTokenVerifierRefusesNoncanonicalText(t *testing.T) {
	raw := bytes.Repeat([]byte{0xA5}, 32)
	good := base64.RawURLEncoding.EncodeToString(raw)
	if v, ok := TokenVerifier(good); !ok || v != Verifier(sha256.Sum256(raw)) {
		t.Fatalf("a canonical token was refused or hashed wrongly")
	}
	last := good[len(good)-1]
	alphabet := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	// The final character of 32 bytes carries two spare bits. Another character with the same
	// high bits but spare bits set decodes to the same bytes in a lenient decoder.
	trailing := string(alphabet[(strings.IndexByte(alphabet, last)+1)%64])
	bad := map[string]string{
		"empty":            "",
		"padded":           good + "=",
		"too short":        good[:42],
		"too long":         good + "A",
		"standard plus":    strings.Replace(good, good[:1], "+", 1),
		"standard slash":   good[:10] + "/" + good[11:],
		"space":            good[:10] + " " + good[11:],
		"newline":          good + "\n",
		"spare bits set":   good[:42] + trailing,
		"standard padding": base64.StdEncoding.EncodeToString(raw),
	}
	for name, token := range bad {
		if _, ok := TokenVerifier(token); ok {
			t.Errorf("%s: TokenVerifier accepted %q", name, token)
		}
	}
}

func TestSessionIssueSerializes(t *testing.T) {
	pw := mustPassword(t, "serialize password")
	f := newSessionFixture(t, fixtureNow)
	s := NewSessions(f.s, pw)
	probe := installProbe(s)
	original, originalVerifiers := issueN(t, s, SessionLimit-1)
	_ = original

	storetest.Exec(t, f.obs, `
CREATE FUNCTION gate_session_insert() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(`+fmt.Sprint(sessionGateKey)+`);
    RETURN NEW;
END;
$$`)
	storetest.Exec(t, f.obs, "CREATE TRIGGER gate_insert BEFORE INSERT ON web_sessions FOR EACH ROW EXECUTE FUNCTION gate_session_insert()")
	release := storetest.HoldAdvisoryGate(t, f.obs, sessionGateKey)
	defer release()
	holder := storetest.Scalar[int32](t, f.obs, "SELECT pg_backend_pid()")
	key := storetest.BigintKey(sessionGateKey)

	waiting := func() int {
		return storetest.Scalar[int](t, f.obs, `
SELECT count(*) FROM pg_locks WHERE locktype = 'advisory' AND NOT granted
  AND classid::bigint = $1 AND objid::bigint = $2 AND objsubid = $3`,
			int64(key.ClassID), int64(key.ObjectID), int16(key.SubID))
	}

	// Writer A reaches its insert and waits there while it holds the table lock.
	a := startIssue(t, s, t.Context())
	var backendA int32
	storetest.Poll(t, "writer A waiting at the insert trigger", func() bool {
		backendA = storetest.Scalar[int32](t, f.obs, `
SELECT COALESCE(max(pid), 0) FROM pg_locks WHERE locktype = 'advisory' AND NOT granted
  AND classid::bigint = $1 AND objid::bigint = $2 AND objsubid = $3`,
			int64(key.ClassID), int64(key.ObjectID), int16(key.SubID))
		return backendA != 0
	})
	storetest.WaitForBlock(t, f.obs, backendA, holder, key)
	if !storetest.Scalar[bool](t, f.obs, `
SELECT EXISTS (SELECT 1 FROM pg_locks WHERE locktype = 'relation' AND relation = 'web_sessions'::regclass
  AND pid = $1 AND granted AND mode = 'ShareRowExclusiveLock')`, backendA) {
		t.Fatal("writer A waits at its insert without holding SHARE ROW EXCLUSIVE on web_sessions")
	}

	// Writer B finishes its derivation, then waits for A's table lock instead of reaching the insert.
	_, finishedBefore := probe.counts()
	b := startIssue(t, s, t.Context())
	storetest.Poll(t, "writer B finishing its binding derivation", func() bool {
		_, finished := probe.counts()
		return finished > finishedBefore
	})
	storetest.Poll(t, "writer B waiting on writer A's table lock", func() bool {
		if n := waiting(); n != 1 {
			t.Fatalf("%d writers wait at the insert trigger, want only writer A: writer B reached its insert without the table lock", n)
		}
		select {
		case <-b.Done():
			t.Fatalf("writer B returned (%v) while writer A still held the table lock", b.Result())
		default:
		}
		return storetest.Scalar[bool](t, f.obs, `
SELECT EXISTS (SELECT 1 FROM pg_locks b WHERE b.locktype = 'relation' AND b.relation = 'web_sessions'::regclass
  AND NOT b.granted AND b.mode = 'ShareRowExclusiveLock' AND $1 = ANY (pg_blocking_pids(b.pid)))`, backendA)
	})
	if got := storetest.Scalar[int](t, f.obs, "SELECT count(*) FROM web_sessions"); got != SessionLimit-1 {
		t.Fatalf("web_sessions holds %d rows while writer A has not committed, want %d", got, SessionLimit-1)
	}

	release()
	resultA, resultB := a.Wait(t), b.Wait(t)
	if resultA.err != nil || resultB.err != nil || resultA.token == "" || resultB.token == "" {
		t.Fatalf("the writers returned (%q, %v) and (%q, %v), want two tokens", resultA.token, resultA.err, resultB.token, resultB.err)
	}
	verifierA, _ := TokenVerifier(resultA.token)
	verifierB, _ := TokenVerifier(resultB.token)
	rowA, okA := readRow(t, f.obs, verifierA)
	rowB, okB := readRow(t, f.obs, verifierB)
	if !okA || !okB || !rowA.createdAt.Before(rowB.createdAt) {
		t.Fatalf("writer A's row (%v, %v) must exist and precede writer B's row (%v, %v)", rowA.createdAt, okA, rowB.createdAt, okB)
	}
	requireStored(t, f.obs, "after both sign-ins", append(slices.Clone(originalVerifiers[1:]), verifierA, verifierB)...)
	requireGone(t, f, s, "the oldest original token", originalVerifiers[0])
	for i, v := range originalVerifiers[1:] {
		requireLive(t, f, s, fmt.Sprintf("original token %d", i+1), v)
	}
	requireLive(t, f, s, "writer A's token", verifierA)
	requireLive(t, f, s, "writer B's token", verifierB)
}

func TestSessionWriteRollsBack(t *testing.T) {
	pw := mustPassword(t, "rollback password")
	f := newSessionFixture(t, fixtureNow)
	s := NewSessions(f.s, pw)
	_, verifiers := issueN(t, s, SessionLimit)
	head := feedHead(t, f.obs)
	storetest.Exec(t, f.obs, `
CREATE FUNCTION refuse_session_eviction() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'The fixture refuses every eviction.' USING ERRCODE = '23514';
END;
$$`)
	storetest.Exec(t, f.obs, "CREATE TRIGGER refuse_eviction BEFORE DELETE ON web_sessions FOR EACH ROW EXECUTE FUNCTION refuse_session_eviction()")

	token, err := s.Issue(t.Context())
	if err == nil {
		t.Fatalf("Issue reported success (token %q) although its transaction could not commit", token)
	}
	if token != "" {
		t.Fatalf("Issue returned the token %q together with the error %v", token, err)
	}
	requireStored(t, f.obs, "after the failed sign-in", verifiers...)
	if got := feedHead(t, f.obs); got != head {
		t.Fatalf("the feed head moved from %d to %d on a rolled back sign-in", head, got)
	}
}

func TestCheckRejectsForeignBinding(t *testing.T) {
	pw := mustPassword(t, "own password")
	other := mustPassword(t, "another password")
	f := newSessionFixture(t, fixtureNow)
	s := NewSessions(f.s, pw)

	foreign := validRow(t, other, verifierOf(0xA1), fixtureNow)
	insertRow(t, f.obs, foreign)
	requireGone(t, f, s, "a row bound to another password", foreign.verifier)
	requireNoSession(t, "revoking a row bound to another password", s.Revoke(t.Context(), foreign.verifier))
	if _, ok := readRow(t, f.obs, foreign.verifier); !ok {
		t.Fatal("the refused revoke deleted the foreign row")
	}

	// A warm cache must not outlive a change of any column of the stored row.
	changes := map[string]string{
		"salt":       "UPDATE web_sessions SET password_salt = $2 WHERE verifier = $1",
		"binding":    "UPDATE web_sessions SET password_binding = $2 WHERE verifier = $1",
		"created at": "UPDATE web_sessions SET created_at = created_at + interval '1 second' WHERE verifier = $1 AND $2::bytea IS NOT NULL",
	}
	for name, update := range changes {
		_, verifiers := issueN(t, s, 1)
		v := verifiers[0]
		requireLive(t, f, s, "the fresh token ("+name+")", v)
		var value []byte
		switch name {
		case "salt":
			value = bytes.Repeat([]byte{0x5A}, 16)
		case "binding":
			value = bytes.Repeat([]byte{0x5A}, 32)
		default:
			value = []byte{0}
		}
		storetest.Exec(t, f.obs, update, v[:], value)
		requireGone(t, f, s, "the warm token after its "+name+" changed", v)
		requireNoSession(t, "revoking after the "+name+" changed", s.Revoke(t.Context(), v))
		if _, ok := readRow(t, f.obs, v); !ok {
			t.Fatalf("the refused revoke deleted the row whose %s changed", name)
		}
	}
}

func TestSessionIssueTieAndBackwardClock(t *testing.T) {
	pw := mustPassword(t, "tie password")
	T := fixtureNow

	t.Run("ties, a future row and an outlier", func(t *testing.T) {
		f := newSessionFixture(t, T)
		var (
			a, b, c, d = verifierOf(1), verifierOf(2), verifierOf(3), verifierOf(4)
			e, g, h    = verifierOf(5), verifierOf(7), verifierOf(8)
			ee         = verifierOf(6)
			outlier    = verifierOf(9)
		)
		insertRow(t, f.obs, junkRow(t, a, T.Add(-5*time.Second)))
		insertRow(t, f.obs, junkRow(t, b, T.Add(-5*time.Second)))
		insertRow(t, f.obs, junkRow(t, c, T.Add(-4*time.Second)))
		insertRow(t, f.obs, junkRow(t, d, T.Add(-3*time.Second)))
		insertRow(t, f.obs, junkRow(t, e, T.Add(-2*time.Second)))
		insertRow(t, f.obs, junkRow(t, ee, T.Add(-time.Second)))
		insertRow(t, f.obs, junkRow(t, g, T))
		insertRow(t, f.obs, junkRow(t, h, T.Add(30*time.Second)))
		insertRow(t, f.obs, junkRow(t, outlier, T.Add(2*time.Minute)))
		head := feedHead(t, f.obs)

		token, err := NewSessions(f.s, pw).Issue(t.Context())
		if err != nil {
			t.Fatalf("Issue failed: %v", err)
		}
		fresh, _ := TokenVerifier(token)
		requireStored(t, f.obs, "after the sign-in", b, c, d, e, ee, g, h, fresh)
		row, _ := readRow(t, f.obs, fresh)
		if want := T.Add(30*time.Second + time.Microsecond); !row.createdAt.Equal(want) {
			t.Fatalf("the new row was created at %v, want one microsecond after the newest surviving row (%v)", row.createdAt, want)
		}

		var removed []string
		saved := 0
		for _, entry := range feedEntries(t, f.obs)[head:] {
			if entry.Topic != "web_sessions" || entry.Scope != "" {
				t.Fatalf("unexpected feed entry %+v", entry)
			}
			switch entry.Change {
			case "removed":
				removed = append(removed, entry.Key)
			case "saved":
				saved++
				if entry.Key != feedKeyOf(fresh) {
					t.Fatalf("the saved reference %q is not the new row's", entry.Key)
				}
			}
		}
		if want := []string{feedKeyOf(outlier), feedKeyOf(a)}; !slices.Equal(removed, want) {
			t.Fatalf("the removal references are %v, want the outlier then the evicted tie loser %v", removed, want)
		}
		if saved != 1 {
			t.Fatalf("the feed holds %d saved references, want 1", saved)
		}
	})

	t.Run("a row at the sample", func(t *testing.T) {
		f := newSessionFixture(t, T)
		insertRow(t, f.obs, junkRow(t, verifierOf(1), T))
		token, err := NewSessions(f.s, pw).Issue(t.Context())
		if err != nil {
			t.Fatalf("Issue failed: %v", err)
		}
		fresh, _ := TokenVerifier(token)
		row, _ := readRow(t, f.obs, fresh)
		if want := T.Add(time.Microsecond); !row.createdAt.Equal(want) {
			t.Fatalf("the new row was created at %v, want %v: strictly after the row at the sample", row.createdAt, want)
		}
	})

	t.Run("only older rows and an empty table", func(t *testing.T) {
		f := newSessionFixture(t, T)
		s := NewSessions(f.s, pw)
		token, err := s.Issue(t.Context())
		if err != nil {
			t.Fatalf("Issue on an empty table failed: %v", err)
		}
		first, _ := TokenVerifier(token)
		row, _ := readRow(t, f.obs, first)
		if !row.createdAt.Equal(T) {
			t.Fatalf("the first row was created at %v, want the sample %v", row.createdAt, T)
		}
		f.setClock(T.Add(time.Hour))
		token, err = s.Issue(t.Context())
		if err != nil {
			t.Fatalf("Issue failed: %v", err)
		}
		second, _ := TokenVerifier(token)
		row, _ = readRow(t, f.obs, second)
		if want := T.Add(time.Hour); !row.createdAt.Equal(want) {
			t.Fatalf("the second row was created at %v, want the sample %v", row.createdAt, want)
		}
	})
}

func TestSessionsUseDatabaseClock(t *testing.T) {
	pw := mustPassword(t, "clock password")
	far := time.Date(2041, 5, 6, 7, 8, 9, 123456000, time.UTC)

	t.Run("expiry follows the database clock", func(t *testing.T) {
		f := newSessionFixture(t, far)
		s := NewSessions(f.s, pw)
		_, verifiers := issueN(t, s, 2)
		first, second := verifiers[0], verifiers[1]
		row, ok := readRow(t, f.obs, first)
		if !ok || !row.createdAt.Equal(far) {
			t.Fatalf("the first row was created at %v, want the database time %v", row.createdAt, far)
		}
		expiry := row.createdAt.Add(sessionLifetime)
		f.setClock(expiry.Add(-time.Microsecond))
		requireLive(t, f, s, "the first token before its expiry", first)
		f.setClock(expiry)
		requireGone(t, f, s, "the first token at its expiry", first)
		requireNoSession(t, "revoking the expired first token", s.Revoke(t.Context(), first))
		requireLive(t, f, s, "the second token, created one microsecond later", second)
		f.setClock(expiry.Add(time.Microsecond))
		requireGone(t, f, s, "the second token at its expiry", second)
		requireNoSession(t, "revoking the expired second token", s.Revoke(t.Context(), second))
	})

	t.Run("a row exactly one minute ahead refuses the sign-in", func(t *testing.T) {
		f := newSessionFixture(t, far)
		ahead := junkRow(t, verifierOf(7), far.Add(time.Minute))
		insertRow(t, f.obs, ahead)
		head := feedHead(t, f.obs)
		token, err := NewSessions(f.s, pw).Issue(t.Context())
		if !errors.Is(err, ErrSessionClock) {
			t.Fatalf("Issue returned (%q, %v), want ErrSessionClock", token, err)
		}
		if token != "" {
			t.Fatalf("Issue returned the token %q with the clock refusal", token)
		}
		requireStored(t, f.obs, "after the clock refusal", ahead.verifier)
		if got := feedHead(t, f.obs); got != head {
			t.Fatalf("the feed head moved from %d to %d on a refused sign-in", head, got)
		}
	})
}

func TestSessionBindingCache(t *testing.T) {
	pw := mustPassword(t, "cache password")
	f2 := newSessionFixture(t, fixtureNow)
	s := NewSessions(f2.s, pw)
	probe := installProbe(s)
	_, first8 := issueN(t, s, SessionLimit)
	if got := s.cachedVerifiers(); len(got) != 0 {
		t.Fatalf("the cache holds %d entries after sign-ins only, want 0", len(got))
	}
	for i, id := range first8 {
		if err := s.Check(t.Context(), id); err != nil {
			t.Fatalf("warming token %d failed: %v", i, err)
		}
	}
	if err := s.Check(t.Context(), first8[0]); err != nil {
		t.Fatalf("touching the first token failed: %v", err)
	}
	wantOrder := append(slices.Clone(first8[1:]), first8[0])
	if got := s.cachedVerifiers(); !slices.Equal(got, wantOrder) {
		t.Fatalf("the cache order is %s, want least recently used first %s", shortList(got), shortList(wantOrder))
	}

	// The ninth sign-in evicts the first row from the database and leaves the cache untouched.
	ninth, err := s.Issue(t.Context())
	if err != nil {
		t.Fatalf("the ninth sign-in failed: %v", err)
	}
	ninthVerifier, _ := TokenVerifier(ninth)
	if got := s.cachedVerifiers(); !slices.Equal(got, wantOrder) {
		t.Fatalf("issuance changed the cache to %s, want it untouched %s", shortList(got), shortList(wantOrder))
	}

	// One hundred concurrent checks of the new token derive once.
	startedBefore, finishedBefore := probe.counts()
	var group sync.WaitGroup
	errs := make([]error, 100)
	for i := range errs {
		group.Add(1)
		go func() {
			defer group.Done()
			errs[i] = s.Check(t.Context(), ninthVerifier)
		}()
	}
	group.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent check %d failed: %v", i, err)
		}
	}
	started, finished := probe.counts()
	if started-startedBefore != 1 || finished-finishedBefore != 1 {
		t.Fatalf("100 concurrent checks of one new token ran %d derivations (%d finished), want exactly 1", started-startedBefore, finished-finishedBefore)
	}
	wantAfter := append(slices.Clone(first8[2:]), first8[0], ninthVerifier)
	if got := s.cachedVerifiers(); !slices.Equal(got, wantAfter) {
		t.Fatalf("the cache is %s, want eight entries without the second token, least recently used first %s", shortList(got), shortList(wantAfter))
	}

	// Checking the session that the database evicted drops its entry.
	requireNoSession(t, "the database-evicted first token", s.Check(t.Context(), first8[0]))
	if got := s.cachedVerifiers(); slices.Contains(got, first8[0]) || len(got) != SessionLimit-1 {
		t.Fatalf("the cache is %s after the evicted row was checked, want seven entries without it", shortList(got))
	}

	// A held cold derivation blocks neither a warm hit nor a canceled waiter.
	cold := NewSessions(f2.s, pw)
	coldProbe := installProbe(cold)
	if err := cold.Check(t.Context(), first8[2]); err != nil {
		t.Fatalf("warming the second instance failed: %v", err)
	}
	holdRow, _ := readRow(t, f2.obs, first8[3])
	gate := newDeriveGate(t)
	coldProbe.holdSalt(gate, holdRow.salt)
	held := startCheck(t, cold, t.Context(), first8[3])
	storetest.ReceiveFrom(t, held, "the cold derivation reaching its hold", gate.entered)
	started, _ = coldProbe.counts()

	if err := finishesPromptly(t, "a warm hit behind a held derivation", startCheck(t, cold, t.Context(), first8[2])); err != nil {
		t.Fatalf("the warm hit failed: %v", err)
	}
	waiterCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	waiter := startCheck(t, cold, waiterCtx, first8[4])
	time.Sleep(settleTime)
	cancel()
	if err := finishesPromptly(t, "a canceled waiter behind a held derivation", waiter); !errors.Is(err, context.Canceled) {
		t.Fatalf("the canceled waiter returned %v, want context.Canceled", err)
	}
	if after, _ := coldProbe.counts(); after != started {
		t.Fatalf("the canceled waiter started a derivation (%d, want %d)", after, started)
	}
	select {
	case <-held.Done():
		t.Fatal("the held derivation finished before its release")
	default:
	}
	gate.open()
	if err := held.Wait(t); err != nil {
		t.Fatalf("the held check failed after its release: %v", err)
	}
}

func TestSessionIssueSlot(t *testing.T) {
	pw := mustPassword(t, "slot password")
	f := newSessionFixture(t, fixtureNow)
	s := NewSessions(f.s, pw)
	probe := installProbe(s)
	_, old := issueN(t, s, 1)

	gate := newDeriveGate(t)
	probe.holdNext(gate)
	first := startIssue(t, s, t.Context())
	storetest.ReceiveFrom(t, first, "the first issuance reaching its held derivation", gate.entered)
	started, _ := probe.counts()

	// Verification has its own slot: a cold check on the same instance is not held behind issuance.
	if err := finishesPromptly(t, "a cold check behind a held issuance", startCheck(t, s, t.Context(), old[0])); err != nil {
		t.Fatalf("the cold check failed: %v", err)
	}
	started++

	// A second issuance waits for the issuance slot, and its context ends the wait.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	second := startIssue(t, s, ctx)
	time.Sleep(settleTime)
	cancel()
	result := finishesPromptly(t, "a canceled second issuance", second)
	if !errors.Is(result.err, context.Canceled) || result.token != "" {
		t.Fatalf("the canceled issuance returned (%q, %v), want an empty token and context.Canceled", result.token, result.err)
	}
	if after, _ := probe.counts(); after != started {
		t.Fatalf("the waiting issuance ran a derivation (%d started, want %d): one issuance derives at a time", after, started)
	}

	gate.open()
	done := first.Wait(t)
	if done.err != nil || done.token == "" {
		t.Fatalf("the held issuance returned (%q, %v) after its release", done.token, done.err)
	}
	fresh, _ := TokenVerifier(done.token)
	requireStored(t, f.obs, "after the held issuance", old[0], fresh)
	if got := s.cachedVerifiers(); !slices.Equal(got, []Verifier{old[0]}) {
		t.Fatalf("the cache is %s, want only the checked token: issuance leaves the cache untouched", shortList(got))
	}
}

func TestRevokeVerifiesBeforeLock(t *testing.T) {
	requireEqualDigestCall(t, "sessions.go")
	pw := mustPassword(t, "revoke password")
	f := newSessionFixture(t, fixtureNow)
	issuer := NewSessions(f.s, pw)
	_, target := issueN(t, issuer, 1)
	row, _ := readRow(t, f.obs, target[0])

	revoker := NewSessions(f.s, pw)
	probe := installProbe(revoker)
	gate := newDeriveGate(t)
	defer gate.open()
	probe.holdSalt(gate, row.salt)
	revoke := storetest.Start(t, func() error { return revoker.Revoke(t.Context(), target[0]) })
	storetest.ReceiveFrom(t, revoke, "the revoking derivation reaching its hold", gate.entered)

	// A separate sign-in must finish while revocation's derivation is held.
	other := startIssue(t, issuer, t.Context())
	done := finishesPromptly(t, "a sign-in while a revoke derives", other)
	if done.err != nil {
		t.Fatalf("the sign-in during the held derivation failed: %v", done.err)
	}
	otherVerifier, _ := TokenVerifier(done.token)

	// Another connection changes the row that the revoke already verified.
	storetest.Exec(t, f.obs, "SET lock_timeout = '2s'")
	storetest.Exec(t, f.obs, "UPDATE web_sessions SET password_binding = $2 WHERE verifier = $1",
		target[0][:], bytes.Repeat([]byte{0x3C}, 32))
	gate.open()
	requireNoSession(t, "revoking a row that changed after its verification", revoke.Wait(t))
	if _, ok := readRow(t, f.obs, target[0]); !ok {
		t.Fatal("revoke deleted a row that no longer matched the verified tuple")
	}
	if err := issuer.Check(t.Context(), otherVerifier); err != nil {
		t.Fatalf("the unrelated session was lost: %v", err)
	}
}

func TestSessionFeedRefs(t *testing.T) {
	pw := mustPassword(t, "feed password")
	f := newSessionFixture(t, fixtureNow)
	s := NewSessions(f.s, pw)

	token, err := s.Issue(t.Context())
	if err != nil {
		t.Fatalf("Issue failed: %v", err)
	}
	v, ok := TokenVerifier(token)
	if !ok {
		t.Fatalf("the token %q is not canonical", token)
	}
	row, ok := readRow(t, f.obs, v)
	if !ok {
		t.Fatal("the row is missing")
	}
	if err := s.Revoke(t.Context(), v); err != nil {
		t.Fatalf("Revoke failed: %v", err)
	}

	entries := feedEntries(t, f.obs)
	want := []feedEntry{
		{"web_sessions", "", feedKeyOf(v), "saved"},
		{"web_sessions", "", feedKeyOf(v), "removed"},
	}
	if !slices.Equal(entries, want) {
		t.Fatalf("the feed holds %+v, want %+v", entries, want)
	}

	raw, _ := base64.RawURLEncoding.DecodeString(token)
	forbidden := map[string]string{
		"the encoded token":   token,
		"the raw token":       hex.EncodeToString(raw),
		"the verifier":        hex.EncodeToString(v[:]),
		"the password":        "feed password",
		"the binding":         hex.EncodeToString(row.binding[:]),
		"the salt":            hex.EncodeToString(row.salt[:]),
		"the base64 verifier": base64.RawURLEncoding.EncodeToString(v[:]),
	}
	for _, entry := range entries {
		if len(entry.Key) != 64 || entry.Key != strings.ToLower(entry.Key) {
			t.Fatalf("the reference key %q is not lowercase hexadecimal of 32 bytes", entry.Key)
		}
		for name, secret := range forbidden {
			if strings.Contains(entry.Key, secret) || strings.Contains(entry.Topic+entry.Scope, secret) {
				t.Fatalf("a feed reference contains %s", name)
			}
		}
	}
	if bytes.Equal(row.verifier[:], raw) || bytes.Contains(append(append(row.verifier[:0:0], row.salt[:]...), row.binding[:]...), raw) {
		t.Fatal("the stored row contains the raw token")
	}
}
