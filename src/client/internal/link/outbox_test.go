package link

import (
	"bytes"
	"errors"
	"math"
	"sync"
	"testing"
	"time"
)

func TestAppendReadAndReopen(t *testing.T) {
	f, _ := newState(t)
	s := mustCreate(t, f, StoreOptions{})
	changed := s.Changed()
	input := []byte("alpha")
	n, err := s.Append(input)
	if err != nil || n != 1 {
		t.Fatalf("first Append gave %d, %v", n, err)
	}
	select {
	case <-changed:
	case <-time.After(time.Second):
		t.Fatal("Changed did not signal after Append")
	}
	input[0] = 'X' // the store must have copied the payload
	for _, p := range []string{"beta", "", "gamma"} {
		if _, err := s.Append([]byte(p)); err != nil {
			t.Fatal(err)
		}
	}
	check := func(s *Store) {
		t.Helper()
		all, err := s.Read(0, 10)
		if err != nil || len(all) != 4 {
			t.Fatalf("Read gave %v, %v", all, err)
		}
		want := []string{"alpha", "beta", "", "gamma"}
		for i, e := range all {
			if e.Seq != uint64(i+1) || string(e.Payload) != want[i] {
				t.Fatalf("event %d is %d %q", i, e.Seq, e.Payload)
			}
		}
		from3, _ := s.Read(3, 10)
		if len(from3) != 2 || from3[0].Seq != 3 {
			t.Fatalf("Read from 3 gave %v", from3)
		}
		two, _ := s.Read(1, 2)
		if len(two) != 2 || two[1].Seq != 2 {
			t.Fatalf("Read with limit 2 gave %v", two)
		}
		if none, err := s.Read(5, 10); err != nil || len(none) != 0 {
			t.Fatalf("Read beyond the end gave %v, %v", none, err)
		}
		if s.Next() != 5 || s.Len() != 4 || s.Bytes() != 14 {
			t.Fatalf("next %d, length %d, bytes %d", s.Next(), s.Len(), s.Bytes())
		}
	}
	check(s)
	// Changing a returned payload must not change the store.
	got, _ := s.Read(1, 1)
	got[0].Payload[0] = 'Z'
	if again, _ := s.Read(1, 1); string(again[0].Payload) != "alpha" {
		t.Fatalf("a changed result changed the store: %q", again[0].Payload)
	}
	s = reopen(t, f, s, StoreOptions{})
	check(s)
	if n, err := s.Append([]byte("delta")); err != nil || n != 5 {
		t.Fatalf("Append after reopen gave %d, %v", n, err)
	}
}

func TestAckRemovesUpToNumber(t *testing.T) {
	f, _ := newState(t)
	s := mustCreate(t, f, StoreOptions{})
	for _, p := range []string{"aa", "bbb", "cccc", "d"} {
		if _, err := s.Append([]byte(p)); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Ack(0); err != nil || s.Len() != 4 {
		t.Fatalf("Ack(0) gave %v with length %d", err, s.Len())
	}
	if err := s.Ack(2); err != nil {
		t.Fatal(err)
	}
	if s.Len() != 2 || s.Next() != 5 || s.Bytes() != 5 {
		t.Fatalf("after Ack(2): length %d, next %d, bytes %d", s.Len(), s.Next(), s.Bytes())
	}
	if ev, _ := s.Read(0, 10); len(ev) != 2 || ev[0].Seq != 3 {
		t.Fatalf("after Ack(2) Read gave %v", ev)
	}
	if err := s.Ack(1); err != nil || s.Len() != 2 {
		t.Fatalf("an older Ack gave %v with length %d", err, s.Len())
	}
	s = reopen(t, f, s, StoreOptions{})
	if s.Len() != 2 || s.Next() != 5 {
		t.Fatalf("after reopen: length %d, next %d", s.Len(), s.Next())
	}
	if err := s.Ack(4); err != nil {
		t.Fatal(err)
	}
	if s.Len() != 0 || s.Bytes() != 0 || s.Next() != 5 {
		t.Fatalf("after Ack(4): length %d, bytes %d, next %d", s.Len(), s.Bytes(), s.Next())
	}
	// The numbers are never reused after the outbox is empty, also not after a reopen.
	s = reopen(t, f, s, StoreOptions{})
	if n, err := s.Append([]byte("e")); err != nil || n != 5 {
		t.Fatalf("Append after emptying gave %d, %v", n, err)
	}
}

func TestAckRefusesAhead(t *testing.T) {
	f, _ := newState(t)
	s := mustCreate(t, f, StoreOptions{})
	for i := 0; i < 3; i++ {
		if _, err := s.Append([]byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	for _, n := range []uint64{4, 100, math.MaxUint64} {
		if err := s.Ack(n); !errors.Is(err, ErrAckAhead) {
			t.Errorf("Ack(%d) gave %v", n, err)
		}
	}
	if s.Len() != 3 {
		t.Fatalf("a refused Ack removed events: length %d", s.Len())
	}
	if err := s.Ack(3); err != nil || s.Len() != 0 {
		t.Fatalf("Ack(3) gave %v with length %d", err, s.Len())
	}
	if err := s.Ack(4); !errors.Is(err, ErrAckAhead) {
		t.Errorf("Ack(4) on an empty outbox gave %v", err)
	}

	// At the wire limit the last number is still readable and acknowledgeable, also
	// after a reopen, and the number above it is refused as ahead.
	const last = uint64(math.MaxInt64)
	f2, _ := newState(t)
	s2 := mustCreate(t, f2, StoreOptions{})
	if err := s2.Close(); err != nil {
		t.Fatal(err)
	}
	setNext(t, f2.Paths().Store, last)
	s2 = mustOpen(t, f2, StoreOptions{})
	if n, err := s2.Append([]byte("last")); err != nil || n != last {
		t.Fatalf("the last legal Append gave %d, %v", n, err)
	}
	for _, n := range []uint64{last + 1, last + 2, math.MaxUint64} {
		if err := s2.Ack(n); !errors.Is(err, ErrAckAhead) {
			t.Errorf("Ack(%d) after the last number gave %v", n, err)
		}
	}
	if s2.Len() != 1 {
		t.Fatalf("a refused Ack removed the last event: length %d", s2.Len())
	}
	s2 = reopen(t, f2, s2, StoreOptions{})
	events, err := s2.Read(last, 10)
	if err != nil || len(events) != 1 || events[0].Seq != last || string(events[0].Payload) != "last" {
		t.Fatalf("after reopen the last event reads as %v, %v", events, err)
	}
	if _, err := s2.Append([]byte("x")); !errors.Is(err, ErrSequenceExhausted) {
		t.Fatalf("Append after reopen gave %v", err)
	}
	if err := s2.Ack(last + 1); !errors.Is(err, ErrAckAhead) {
		t.Errorf("Ack(%d) after reopen gave %v", last+1, err)
	}
	if err := s2.Ack(last); err != nil || s2.Len() != 0 || s2.Bytes() != 0 {
		t.Fatalf("Ack(%d) after reopen gave %v with length %d, bytes %d", last, err, s2.Len(), s2.Bytes())
	}
	s2 = reopen(t, f2, s2, StoreOptions{})
	if _, err := s2.Append([]byte("x")); !errors.Is(err, ErrSequenceExhausted) {
		t.Fatalf("Append after the last event was acknowledged gave %v", err)
	}
	if err := s2.Ack(last + 1); !errors.Is(err, ErrAckAhead) {
		t.Errorf("Ack(%d) on the empty exhausted outbox gave %v", last+1, err)
	}
}

func TestAppendRefusesWhenFull(t *testing.T) {
	f, _ := newState(t)
	s := mustCreate(t, f, StoreOptions{MaxEvents: 2, MaxBytes: 10, MaxEventBytes: 6})
	if _, err := s.Append(bytes.Repeat([]byte{'a'}, 7)); !errors.Is(err, ErrEventTooLarge) {
		t.Fatalf("a 7 byte event gave %v", err)
	}
	if n, err := s.Append(bytes.Repeat([]byte{'a'}, 6)); err != nil || n != 1 {
		t.Fatalf("a 6 byte event gave %d, %v", n, err)
	}
	if _, err := s.Append(bytes.Repeat([]byte{'b'}, 5)); !errors.Is(err, ErrOutboxFull) {
		t.Fatalf("an event over MaxBytes gave %v", err)
	}
	if s.Next() != 2 || s.Len() != 1 {
		t.Fatalf("a refused event changed the store: next %d, length %d", s.Next(), s.Len())
	}
	if n, err := s.Append(bytes.Repeat([]byte{'b'}, 4)); err != nil || n != 2 {
		t.Fatalf("a 4 byte event gave %d, %v", n, err)
	}
	if _, err := s.Append(nil); !errors.Is(err, ErrOutboxFull) {
		t.Fatalf("a third event over MaxEvents gave %v", err)
	}
	if err := s.Ack(1); err != nil {
		t.Fatal(err)
	}
	if n, err := s.Append([]byte("c")); err != nil || n != 3 {
		t.Fatalf("Append after Ack gave %d, %v", n, err)
	}

	// A refusal because of the limits does not fail the store.
	if _, err := s.Append([]byte("z")); errors.Is(err, ErrStoreFailed) {
		t.Fatal("a full outbox failed the store")
	}

	// The numbers stop at the wire limit of 2^63 - 1, long before they wrap around.
	for _, c := range []struct {
		start    uint64
		accepted int
	}{
		{math.MaxUint64, 0},
		{math.MaxInt64 + 2, 0},
		{math.MaxInt64 + 1, 0},
		{math.MaxInt64, 1},
		{math.MaxInt64 - 1, 2},
	} {
		f2, _ := newState(t)
		s2 := mustCreate(t, f2, StoreOptions{})
		if err := s2.Close(); err != nil {
			t.Fatal(err)
		}
		setNext(t, f2.Paths().Store, c.start)
		s2 = mustOpen(t, f2, StoreOptions{})
		for i := 0; i < c.accepted; i++ {
			if n, err := s2.Append([]byte("x")); err != nil || n != c.start+uint64(i) {
				t.Fatalf("start %d: Append gave %d, %v", c.start, n, err)
			}
		}
		before, err := s2.Read(0, 10)
		if err != nil {
			t.Fatal(err)
		}
		next, size := s2.Next(), s2.Bytes()
		for i := 0; i < 2; i++ {
			if _, err := s2.Append([]byte("x")); !errors.Is(err, ErrSequenceExhausted) {
				t.Fatalf("start %d: the next Append gave %v", c.start, err)
			}
		}
		if s2.Len() != c.accepted || s2.Next() != next || s2.Bytes() != size {
			t.Fatalf("start %d: a refused Append changed the outbox: length %d, next %d, bytes %d",
				c.start, s2.Len(), s2.Next(), s2.Bytes())
		}
		if after, err := s2.Read(0, 10); err != nil || len(after) != len(before) {
			t.Fatalf("start %d: Read after the refusals gave %v, %v", c.start, after, err)
		}
		s2 = reopen(t, f2, s2, StoreOptions{})
		if _, err := s2.Append([]byte("x")); !errors.Is(err, ErrSequenceExhausted) {
			t.Fatalf("start %d: Append after a reopen gave %v", c.start, err)
		}
		if s2.Len() != c.accepted || s2.Next() != next {
			t.Fatalf("start %d: after a reopen length %d, next %d", c.start, s2.Len(), s2.Next())
		}
	}
}

func TestConcurrentAppendsAreUnique(t *testing.T) {
	f, _ := newState(t)
	s := mustCreate(t, f, StoreOptions{})
	const workers, each = 8, 25
	var mu sync.Mutex
	got := map[uint64]string{}
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < each; i++ {
				p := []byte{byte('a' + w), byte(i)}
				n, err := s.Append(p)
				if err != nil {
					t.Error(err)
					return
				}
				mu.Lock()
				if _, dup := got[n]; dup {
					t.Errorf("number %d was given twice", n)
				}
				got[n] = string(p)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(got) != workers*each {
		t.Fatalf("%d numbers for %d events", len(got), workers*each)
	}
	events, err := s.Read(0, 1000)
	if err != nil || len(events) != workers*each {
		t.Fatalf("Read gave %d events, %v", len(events), err)
	}
	for i, e := range events {
		if e.Seq != uint64(i+1) || got[e.Seq] != string(e.Payload) {
			t.Fatalf("event %d is %d %q, the writer was told %q", i, e.Seq, e.Payload, got[e.Seq])
		}
	}
}

func TestReadPayloadsSurviveGrowth(t *testing.T) {
	f, _ := newState(t)
	s := mustCreate(t, f, StoreOptions{MaxEvents: 100000, MaxBytes: 1 << 30})
	if _, err := s.Append([]byte("first payload")); err != nil {
		t.Fatal(err)
	}
	early, err := s.Read(0, 1)
	if err != nil || len(early) != 1 {
		t.Fatalf("Read gave %v, %v", early, err)
	}
	// Free the page of the event and write other bytes, so that a kept bbolt slice would change.
	if err := s.Ack(1); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		if _, err := s.Append(bytes.Repeat([]byte{byte('A' + i)}, 13)); err != nil {
			t.Fatal(err)
		}
		if err := s.Ack(uint64(i + 2)); err != nil {
			t.Fatal(err)
		}
	}
	if string(early[0].Payload) != "first payload" {
		t.Fatalf("an earlier result changed to %q after the page was reused", early[0].Payload)
	}
	// Enough data to grow and remap the file several times.
	big := bytes.Repeat([]byte{'g'}, 64*1024)
	for i := 0; i < 200; i++ {
		if _, err := s.Append(big); err != nil {
			t.Fatal(err)
		}
	}
	if string(early[0].Payload) != "first payload" {
		t.Fatalf("an earlier result changed to %q", early[0].Payload)
	}
}
