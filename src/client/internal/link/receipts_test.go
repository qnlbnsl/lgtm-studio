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
)

func receiptJSON(expires time.Time, state string, result []byte) string {
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
	result := []byte("the result \x00 with bytes")
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
	// The file is read while the store is still open, through a copy.
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	got := readReceipts(t, f.Paths().Store)
	raw, ok := got[c.ID]
	if !ok {
		t.Fatal("no receipt was stored when Begin returned DecisionRun")
	}
	var v struct {
		Version   int    `json:"version"`
		ExpiresAt string `json:"expires_at"`
		State     string `json:"state"`
		Result    string `json:"result"`
	}
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("the receipt is not JSON: %v", err)
	}
	if v.Version != 1 || v.State != "started" || v.Result != "" {
		t.Fatalf("the receipt is %s", raw)
	}
	if exp, err := time.Parse(time.RFC3339, v.ExpiresAt); err != nil || !exp.Equal(c.ExpiresAt) || v.ExpiresAt[len(v.ExpiresAt)-1] != 'Z' {
		t.Fatalf("expires_at is %q: %v", v.ExpiresAt, err)
	}
}

func TestStartedFromEarlierRunIsInterrupted(t *testing.T) {
	f, _ := newState(t)
	s := mustCreate(t, f, StoreOptions{})
	started := cmdAt(1, time.Hour)
	if d, _, _ := s.Begin(started, tNow); d != DecisionRun {
		t.Fatalf("Begin gave %v", d)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	bigResult := bytes.Repeat([]byte{'r'}, 64*1024)
	exp := tNow.Add(time.Hour)
	bad := map[string]string{
		uid(10): `{`,
		uid(11): fmt.Sprintf(`{"version":2,"expires_at":%q,"state":"started","result":""}`, exp.Format(time.RFC3339)),
		uid(12): fmt.Sprintf(`{"version":1,"expires_at":%q,"state":"done","result":""}`, exp.Format(time.RFC3339)),
		uid(13): `{"version":1,"expires_at":"0001-01-01T00:00:00Z","state":"started","result":""}`,
		uid(14): receiptJSON(exp, "started", []byte("a result")),
		uid(15): fmt.Sprintf(`{"version":1,"expires_at":%q,"state":"finished","result":"!!not base64!!"}`, exp.Format(time.RFC3339)),
		uid(16): receiptJSON(exp, "finished", bigResult),
		uid(17): `{"version":1,"state":"started","result":""}`,
		uid(18): ``,
	}
	putReceipts(t, f.Paths().Store, bad)

	// The clock is far ahead, so that a wrong pruner would remove every one of these receipts.
	old := wallClock
	wallClock = func() time.Time { return tNow.Add(10 * 365 * 24 * time.Hour) }
	t.Cleanup(func() { wallClock = old })

	s = mustOpen(t, f, StoreOptions{MaxResultBytes: 1024})
	d, _, err := s.Begin(started, tNow)
	if err != nil || d != DecisionInterrupted {
		t.Fatalf("a started receipt of an earlier run gave %v, %v", d, err)
	}
	for id := range bad {
		d, r, err := s.Begin(Command{ID: id, ExpiresAt: exp}, tNow)
		if err != nil || d != DecisionInterrupted || len(r) != 0 {
			t.Errorf("the invalid receipt %.12s gave %v, %q, %v", id, d, r, err)
		}
	}
	// Finish is not allowed for a command that this process did not start.
	if err := s.Finish(started.ID, []byte("late")); !errors.Is(err, ErrNotRunning) {
		t.Errorf("Finish of an interrupted command gave %v", err)
	}
	// Interrupted is computed, so the stored value does not change.
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	stored := readReceipts(t, f.Paths().Store)
	if stored[started.ID] != receiptJSON(started.ExpiresAt, "started", nil) && !bytes.Contains([]byte(stored[started.ID]), []byte(`"started"`)) {
		t.Errorf("the started receipt was changed to %s", stored[started.ID])
	}
	for id, v := range bad {
		if stored[id] != v {
			t.Errorf("the invalid receipt %.12s was changed or removed", id)
		}
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
	if err := s.Finish(c.ID, []byte("r")); err != nil {
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
	s := mustCreate(t, f, StoreOptions{MaxResultBytes: 8})
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
	if err := s.Finish(uid(1), bytes.Repeat([]byte{'x'}, 9)); !errors.Is(err, ErrResultTooLarge) {
		t.Errorf("a result of 9 bytes gave %v", err)
	}
	if err := s.Finish(uid(1), bytes.Repeat([]byte{'x'}, 8)); err != nil {
		t.Errorf("a result of 8 bytes gave %v", err)
	}
	if err := s.Finish(uid(1), []byte("again")); !errors.Is(err, ErrNotRunning) {
		t.Errorf("a second Finish gave %v", err)
	}
	if err := s.Finish(uid(77), nil); !errors.Is(err, ErrNotRunning) {
		t.Errorf("Finish of an unknown id gave %v", err)
	}
}

func TestFinishPrunesByAge(t *testing.T) {
	f, _ := newState(t)
	s := mustCreate(t, f, StoreOptions{})
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	now := tNow
	old := wallClock
	wallClock = func() time.Time { return now }
	t.Cleanup(func() { wallClock = old })
	fin := func(exp time.Time) string { return receiptJSON(exp, "finished", []byte("r")) }
	fixtures := map[string]string{
		uid(1): fin(now.Add(-25 * time.Hour)),
		uid(2): fin(now.Add(-23 * time.Hour)),
		uid(3): fin(now.Add(-24 * time.Hour)),
		uid(4): receiptJSON(now.Add(-48*time.Hour), "started", nil),
		uid(5): fin(now.Add(-72 * time.Hour)),
		uid(6): `{`,
	}
	// A large number of receipts inside the day must all stay.
	for i := 0; i < 2000; i++ {
		fixtures[uid(1000+i)] = fin(now.Add(-time.Duration(i%1400) * time.Minute))
	}
	putReceipts(t, f.Paths().Store, fixtures)
	s = mustOpen(t, f, StoreOptions{})
	c := cmdAt(5000, time.Hour)
	if d, _, err := s.Begin(c, now); err != nil || d != DecisionRun {
		t.Fatalf("Begin gave %v, %v", d, err)
	}
	if err := s.Finish(c.ID, []byte("ok")); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	stored := readReceipts(t, f.Paths().Store)
	for _, id := range []string{uid(1), uid(5)} {
		if _, ok := stored[id]; ok {
			t.Errorf("the finished receipt %.12s, over a day old, was kept", id)
		}
	}
	for _, id := range []string{uid(2), uid(3), uid(4), uid(6), c.ID} {
		if _, ok := stored[id]; !ok {
			t.Errorf("the receipt %.12s was removed", id)
		}
	}
	for i := 0; i < 2000; i++ {
		if _, ok := stored[uid(1000+i)]; !ok {
			t.Fatalf("the receipt %d within a day was removed", i)
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
	err := s.Finish(c.ID, []byte("r"))
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
