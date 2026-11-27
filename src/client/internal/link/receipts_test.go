package link

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

func receiptJSON(expires time.Time, state string, result []byte) string {
	if state == "started" && len(result) == 0 {
		// A valid started receipt has no result field at all.
		return fmt.Sprintf(`{"version":1,"expires_at":%q,"state":"started"}`, expires.UTC().Format(time.RFC3339))
	}
	return fmt.Sprintf(`{"version":1,"expires_at":%q,"state":%q,"result":%q}`,
		expires.UTC().Format(time.RFC3339), state, base64.StdEncoding.EncodeToString(result))
}

func cmdAt(n int, ttl time.Duration) Command {
	return Command{ID: uid(n), ExpiresAt: tNow.Add(ttl)}
}

func TestBeginRunsAndReplays(t *testing.T) {
	f, _ := newState(t)
	s := mustCreate(t, f, StoreOptions{})
	c := cmdAt(1, time.Hour)
	if d, r, err := s.Begin(c, tNow); err != nil || d != DecisionRun || len(r) != 0 {
		t.Fatalf("first Begin gave %v, %q, %v", d, r, err)
	}
	if d, _, err := s.Begin(c, tNow); err != nil || d != DecisionRunning {
		t.Fatalf("second Begin gave %v, %v", d, err)
	}
	result := []byte{byte(commandDone)}
	if err := s.Finish(c.ID, result); err != nil {
		t.Fatal(err)
	}
	check := func(s *Store) {
		t.Helper()
		d, r, err := s.Begin(c, tNow)
		if err != nil || d != DecisionReplay || !bytes.Equal(r, result) {
			t.Fatalf("Begin after Finish gave %v, %q, %v", d, r, err)
		}
	}
	check(s)
	r := s
	// The result is copied, so a change of it does not change the receipt.
	_, got, _ := r.Begin(c, tNow)
	got[0] = 'X'
	check(r)
	s = reopen(t, f, s, StoreOptions{})
	check(s)
	if d, _, _ := s.Begin(cmdAt(2, time.Hour), tNow); d != DecisionRun {
		t.Fatalf("a new id gave %v", d)
	}
}

func TestBeginWritesStartedBeforeReturn(t *testing.T) {
	f, _ := newState(t)
	s := mustCreate(t, f, StoreOptions{})
	c := cmdAt(1, time.Hour)
	if d, _, err := s.Begin(c, tNow); err != nil || d != DecisionRun {
		t.Fatalf("Begin gave %v, %v", d, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	raw, ok := readReceipts(t, f.Paths().Store)[c.ID]
	if !ok {
		t.Fatal("no receipt was stored when Begin returned DecisionRun")
	}
	var got map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &got) != nil {
		t.Fatal("the receipt is not valid JSON")
	}
	if string(got["version"]) != "2" || string(got["state"]) != `"started"` ||
		string(got["standing"]) != "false" || string(got["last_event_number"]) != "null" || got["result"] != nil {
		t.Error("the start did not store revision 2, nonstanding status and an explicit null watermark without a result")
	}
	if string(got["expires_at"]) != fmt.Sprintf("%q", c.ExpiresAt.UTC().Format(time.RFC3339)) {
		t.Error("the start did not store the expiry in Coordinated Universal Time")
	}
}

func TestStartedFromEarlierRunIsInterrupted(t *testing.T) {
	t.Run("recovered start", func(t *testing.T) {
		folder, _ := newState(t)
		store := mustCreate(t, folder, StoreOptions{})
		expiry := tNow.Add(time.Hour)
		started := scanCommand(1, 1, expiry)
		if d, _, err := store.Begin(receiptCommand(started), tNow); err != nil || d != DecisionRun {
			t.Fatalf("Begin gave %v, %v", d, err)
		}
		appendLinkFixture(t, store, storedReport(t, replayFacts(tNow)))
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
		legacy := scanCommand(2, 1, expiry)
		legacyRaw := receiptJSON(expiry, "started", nil)
		putReceipts(t, folder.Paths().Store, map[string]string{legacy.GetId(): legacyRaw})
		store = mustOpen(t, folder, StoreOptions{})
		requireReceiptPreparation(t, store)
		fixture := newCommandFixture(t, store, nil, nil, nil)
		for _, tc := range []struct {
			cmd  *linkv1.Command
			mark uint64
		}{{started, 1}, {legacy, 0}} {
			cmd := attemptCommand(tc.cmd, 2)
			requireContinues(t, fixture, cmd)
			requireReceiptReply(t, fixture, cmd, commandInterrupted, tc.mark)
			if err := store.Finish(cmd.GetId(), []byte{byte(commandDone)}); !errors.Is(err, ErrNotRunning) {
				t.Errorf("Finish of an interrupted command gave %v", err)
			}
		}
		requireRevisionTwoReceipt(t, receiptOnDisk(t, store, started.GetId()), "interrupted", false, expiry, 1, "")
		if receiptOnDisk(t, store, legacy.GetId()) != legacyRaw {
			t.Error("the original revision 1 started receipt was rewritten")
		}
	})

	exp := tNow.Add(time.Hour)
	bigResult := bytes.Repeat([]byte{'r'}, 64*1024)
	for _, tc := range []struct {
		name, raw string
	}{
		{"invalid JSON", `{`},
		{"revision two start with a result", fmt.Sprintf(`{"version":2,"expires_at":%q,"state":"started","result":""}`, exp.Format(time.RFC3339))},
		{"legacy unknown state", fmt.Sprintf(`{"version":1,"expires_at":%q,"state":"done","result":""}`, exp.Format(time.RFC3339))},
		{"legacy zero expiry", `{"version":1,"expires_at":"0001-01-01T00:00:00Z","state":"started","result":""}`},
		{"legacy unexpected result", receiptJSON(exp, "started", []byte("a result"))},
		{"legacy invalid base64", fmt.Sprintf(`{"version":1,"expires_at":%q,"state":"finished","result":"!!not base64!!"}`, exp.Format(time.RFC3339))},
		{"legacy oversized result", receiptJSON(exp, "finished", bigResult)},
		{"legacy missing expiry", `{"version":1,"state":"started","result":""}`},
		{"empty value", ``},
	} {
		t.Run(tc.name, func(t *testing.T) {
			folder, _ := newState(t)
			opts := StoreOptions{MaxResultBytes: 1024}
			store := mustCreate(t, folder, opts)
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			id := scanCommand(1, 1, exp).GetId()
			putReceipts(t, folder.Paths().Store, map[string]string{id: tc.raw})
			store, err := OpenStore(folder, opts)
			if err == nil {
				err = store.prepareRun()
				if closeErr := store.Close(); closeErr != nil {
					t.Fatal(closeErr)
				}
			}
			if !errors.Is(err, ErrStoreDamaged) {
				t.Error("damaged metadata did not fault with ErrStoreDamaged")
			}
			if got, ok := readReceipts(t, folder.Paths().Store)[id]; !ok || got != tc.raw {
				t.Error("the damaged receipt was changed or removed")
			}
		})
	}
}

func TestOnlyRunAndFinishWrite(t *testing.T) {
	f, _ := newState(t)
	s := mustCreate(t, f, StoreOptions{})
	var commits int
	var mu sync.Mutex
	restore := swapCommit(func(db *bolt.DB, fn func(*bolt.Tx) error) error {
		mu.Lock()
		commits++
		mu.Unlock()
		return db.Update(fn)
	})
	defer restore()
	count := func() int { mu.Lock(); defer mu.Unlock(); return commits }

	c := cmdAt(1, time.Hour)
	s.Begin(c, tNow)
	if count() != 1 {
		t.Fatalf("Begin of a new command made %d commits", count())
	}
	s.Begin(c, tNow)
	if count() != 1 {
		t.Fatalf("a Begin that gave Running made a commit")
	}
	s.Begin(Command{ID: uid(2), ExpiresAt: tNow.Add(-time.Hour)}, tNow)
	if count() != 1 {
		t.Fatalf("a Begin that gave Expired made a commit")
	}
	if err := s.Finish(c.ID, []byte{byte(commandDone)}); err != nil {
		t.Fatal(err)
	}
	if count() != 2 {
		t.Fatalf("Finish made %d commits in all", count())
	}
	s.Begin(c, tNow)
	if count() != 2 {
		t.Fatalf("a Begin that gave Replay made a commit")
	}
	s.Finish(c.ID, nil)
	s.Finish(uid(99), nil)
	if count() != 2 {
		t.Fatalf("a refused Finish made a commit")
	}
}

func TestBeginRefusesExpiredAndBadInput(t *testing.T) {
	f, _ := newState(t)
	s := mustCreate(t, f, StoreOptions{MaxResultBytes: 1})
	if d, _, err := s.Begin(Command{ID: uid(1), ExpiresAt: tNow.Add(-time.Second)}, tNow); err != nil || d != DecisionExpired {
		t.Fatalf("an expired command gave %v, %v", d, err)
	}
	if d, _, err := s.Begin(Command{ID: uid(1), ExpiresAt: tNow}, tNow); err != nil || d != DecisionExpired {
		t.Fatalf("a command that expires now gave %v, %v", d, err)
	}
	// An expired command left no receipt, so a later valid one with the same id runs.
	if d, _, err := s.Begin(Command{ID: uid(1), ExpiresAt: tNow.Add(time.Hour)}, tNow); err != nil || d != DecisionRun {
		t.Fatalf("the same id with a later expiry gave %v, %v", d, err)
	}
	bad := []Command{
		{ID: "", ExpiresAt: tNow.Add(time.Hour)},
		{ID: "not-an-id", ExpiresAt: tNow.Add(time.Hour)},
		{ID: "00000002-0000-4000-8000-00000000000G", ExpiresAt: tNow.Add(time.Hour)},
		{ID: "00000002-0000-4000-8000-0000000000AB", ExpiresAt: tNow.Add(time.Hour)},
		{ID: "00000002000040008000000000000002", ExpiresAt: tNow.Add(time.Hour)},
		{ID: uid(3), ExpiresAt: time.Time{}},
	}
	for _, c := range bad {
		if d, _, err := s.Begin(c, tNow); !errors.Is(err, ErrInvalidCommand) || d == DecisionRun {
			t.Errorf("Begin(%q, %v) gave %v, %v", c.ID, c.ExpiresAt, d, err)
		}
	}
	// A refused command stores nothing.
	if err := s.Finish(uid(3), nil); !errors.Is(err, ErrNotRunning) {
		t.Errorf("Finish of a refused command gave %v", err)
	}
	if err := s.Finish("not-an-id", nil); err == nil {
		t.Error("Finish with an invalid id gave no error")
	}
	// Finish rules.
	if err := s.Finish(uid(1), bytes.Repeat([]byte{'x'}, 2)); !errors.Is(err, ErrResultTooLarge) {
		t.Errorf("a result of 2 bytes gave %v", err)
	}
	if err := s.Finish(uid(1), []byte{byte(commandDone)}); err != nil {
		t.Errorf("a result of 1 byte gave %v", err)
	}
	if err := s.Finish(uid(1), []byte{byte(commandFailed)}); !errors.Is(err, ErrNotRunning) {
		t.Errorf("a second Finish gave %v", err)
	}
	if err := s.Finish(uid(77), nil); !errors.Is(err, ErrNotRunning) {
		t.Errorf("Finish of an unknown id gave %v", err)
	}
}

func TestFinishPrunesByAge(t *testing.T) {
	folder, _ := newState(t)
	store := mustCreate(t, folder, StoreOptions{})
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	old := wallClock
	wallClock = func() time.Time { return tNow.Add(365 * 24 * time.Hour) }
	t.Cleanup(func() { wallClock = old })
	finished := func(age time.Duration) string {
		return receiptJSON(tNow.Add(-age), "finished", []byte{byte(commandDone)})
	}
	fixtures := map[string]string{
		uid(1): finished(25 * time.Hour),
		uid(2): finished(23 * time.Hour),
		uid(3): finished(24 * time.Hour),
		uid(4): receiptJSON(tNow.Add(-48*time.Hour), "started", nil),
		uid(5): finished(72 * time.Hour),
	}
	for i := 0; i < 2000; i++ {
		fixtures[uid(1000+i)] = finished(time.Duration(i%1400) * time.Minute)
	}
	putReceipts(t, folder.Paths().Store, fixtures)
	store = mustOpen(t, folder, StoreOptions{})
	requireReceiptPreparation(t, store)
	f := newCommandFixture(t, store, nil, nil, nil)
	write := func(n int) {
		t.Helper()
		cmd := scanCommand(n, 1, tNow.Add(2*365*24*time.Hour))
		cmd.Action = &linkv1.Command_Restart{Restart: &linkv1.Restart{}}
		if restart, err := f.handler.handle(t.Context(), cmd); err != nil || !restart {
			t.Fatal("the receipt age trigger did not finish")
		}
		f.reply(t, cmd, commandDone)
		requireReceipt(t, store, receiptCommand(cmd), DecisionReplay, []byte{byte(commandDone)})
	}

	setReceiptClock(f, tNow, 5*time.Second, 7*time.Second, false)
	write(5000)
	for id, raw := range fixtures {
		if receiptOnDisk(t, store, id) != raw {
			t.Error("a write before a trusted sample pruned or changed a receipt")
			break
		}
	}

	setReceiptClock(f, tNow, 5*time.Second, 7*time.Second, true)
	write(5001)
	for _, id := range []string{uid(1), uid(5)} {
		if receiptOnDisk(t, store, id) != "" {
			t.Error("the verified lower bound did not prune an ordinary receipt over a day old")
		}
	}
	for _, id := range []string{uid(2), uid(3), uid(4)} {
		if receiptOnDisk(t, store, id) != fixtures[id] {
			t.Error("pruning crossed the 24-hour equality boundary or removed the old started receipt")
		}
	}

	f.elapsed.Add(int64(time.Second))
	write(5002)
	if receiptOnDisk(t, store, uid(3)) != "" {
		t.Error("pruning did not advance with elapsed time after receipt of Welcome")
	}
	if receiptOnDisk(t, store, uid(4)) != fixtures[uid(4)] {
		t.Error("pruning removed the old started receipt")
	}
	requireReceipt(t, store, Command{ID: uid(4), ExpiresAt: tNow.Add(time.Hour)}, DecisionInterrupted, nil)
	for i := 0; i < 2000; i++ {
		id := uid(1000 + i)
		if receiptOnDisk(t, store, id) != fixtures[id] {
			t.Fatalf("the receipt %d within a day was removed or changed", i)
		}
	}
}

func TestFinishCommitErrorKeepsStoredState(t *testing.T) {
	f, _ := newState(t)
	s := mustCreate(t, f, StoreOptions{})
	c := cmdAt(1, time.Hour)
	if d, _, _ := s.Begin(c, tNow); d != DecisionRun {
		t.Fatalf("Begin gave %v", d)
	}
	restore := swapCommit(func(db *bolt.DB, fn func(*bolt.Tx) error) error {
		return errors.New("the commit failed")
	})
	err := s.Finish(c.ID, []byte{byte(commandDone)})
	restore()
	if !errors.Is(err, ErrStoreFailed) {
		t.Fatalf("Finish gave %v", err)
	}
	if _, _, err := s.Begin(cmdAt(2, time.Hour), tNow); !errors.Is(err, ErrStoreFailed) {
		t.Fatalf("Begin after the failure gave %v", err)
	}
	s = reopen(t, f, s, StoreOptions{})
	if d, _, err := s.Begin(c, tNow); err != nil || d != DecisionInterrupted {
		t.Fatalf("after the failed Finish Begin gave %v, %v", d, err)
	}
}

func TestConcurrentBeginRunsOnce(t *testing.T) {
	f, _ := newState(t)
	s := mustCreate(t, f, StoreOptions{})
	c := cmdAt(1, time.Hour)
	const workers = 16
	var mu sync.Mutex
	counts := map[Decision]int{}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d, _, err := s.Begin(c, tNow)
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			counts[d]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	if counts[DecisionRun] != 1 || counts[DecisionRunning] != workers-1 {
		t.Fatalf("decisions were %v", counts)
	}
}
