package store

import (
	"bytes"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"testing"
	"testing/iotest"
	"time"
)

// decodeID checks the canonical shape of an identifier and returns its sixteen bytes.
func decodeID(t *testing.T, id string) []byte {
	t.Helper()
	if len(id) != 36 || id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		t.Fatalf("identifier %q is not in the form 8-4-4-4-12", id)
	}
	if id != strings.ToLower(id) {
		t.Fatalf("identifier %q is not lowercase", id)
	}
	raw, err := hex.DecodeString(strings.ReplaceAll(id, "-", ""))
	if err != nil || len(raw) != 16 {
		t.Fatalf("identifier %q is not hexadecimal: %v", id, err)
	}
	return raw
}

func idMillis(raw []byte) int64 {
	var ms int64
	for _, b := range raw[:6] {
		ms = ms<<8 | int64(b)
	}
	return ms
}

func TestNewIDVersionAndTime(t *testing.T) {
	// Ten bytes are the 80 bits of which 74 stay after the version and variant.
	random := []byte{0xF5, 0xA6, 0xFB, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77}
	const base = int64(0x0123456789AB)

	first, err := uuidV7(time.UnixMilli(base), bytes.NewReader(random))
	if err != nil {
		t.Fatalf("the first identifier failed: %v", err)
	}
	second, err := uuidV7(time.UnixMilli(base+1), bytes.NewReader(random))
	if err != nil {
		t.Fatalf("the second identifier failed: %v", err)
	}
	if want := "01234567-89ab-75a6-bb11-223344556677"; first != want {
		t.Fatalf("first identifier = %q, want %q", first, want)
	}
	if want := "01234567-89ac-75a6-bb11-223344556677"; second != want {
		t.Fatalf("second identifier = %q, want %q", second, want)
	}
	raw := decodeID(t, first)
	if idMillis(raw) != base {
		t.Fatalf("timestamp = %#x, want %#x", idMillis(raw), base)
	}
	if raw[6]>>4 != 7 {
		t.Fatalf("version nibble = %d, want 7", raw[6]>>4)
	}
	if raw[8]>>6 != 0b10 {
		t.Fatalf("variant bits = %b, want 10", raw[8]>>6)
	}
	if !(first < second) {
		t.Fatalf("lexical order does not follow time: %q is not before %q", first, second)
	}

	// A carry across a byte boundary must keep the order, which a little-endian timestamp breaks.
	low, err := uuidV7(time.UnixMilli(0x0123456789FF), bytes.NewReader(random))
	if err != nil {
		t.Fatal(err)
	}
	high, err := uuidV7(time.UnixMilli(0x012345678A00), bytes.NewReader(random))
	if err != nil {
		t.Fatal(err)
	}
	if !(low < high) {
		t.Fatalf("lexical order breaks at a byte boundary: %q is not before %q", low, high)
	}
	if !strings.HasPrefix(high, "01234567-8a00-7") {
		t.Fatalf("identifier %q does not carry the timestamp in big-endian order", high)
	}

	// Real identifiers from concurrent goroutines are well formed, current and unique.
	const goroutines, each = 16, 256
	before := time.Now().UnixMilli()
	var mu sync.Mutex
	seen := make(map[string]struct{}, goroutines*each)
	var wg sync.WaitGroup
	failures := make(chan string, goroutines)
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range each {
				id, err := NewID()
				if err != nil {
					failures <- "NewID failed: " + err.Error()
					return
				}
				mu.Lock()
				seen[id] = struct{}{}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	close(failures)
	for failure := range failures {
		t.Fatal(failure)
	}
	after := time.Now().UnixMilli()
	if len(seen) != goroutines*each {
		t.Fatalf("got %d distinct identifiers out of %d", len(seen), goroutines*each)
	}
	for id := range seen {
		raw := decodeID(t, id)
		if raw[6]>>4 != 7 || raw[8]>>6 != 0b10 {
			t.Fatalf("identifier %q has the wrong version or variant", id)
		}
		if ms := idMillis(raw); ms < before || ms > after {
			t.Fatalf("identifier %q carries time %d outside the test window %d to %d", id, ms, before, after)
		}
	}
}

func TestUUIDv7EntropyFailure(t *testing.T) {
	at := time.UnixMilli(0x0123456789AB)
	readers := map[string]func() *bytes.Reader{
		"nine bytes": func() *bytes.Reader { return bytes.NewReader(make([]byte, 9)) },
		"no bytes":   func() *bytes.Reader { return bytes.NewReader(nil) },
	}
	for name, reader := range readers {
		t.Run(name, func(t *testing.T) {
			id, err := uuidV7(at, reader())
			if err == nil {
				t.Fatalf("a short random source returned no error and identifier %q", id)
			}
			if id != "" {
				t.Fatalf("a failed generation returned identifier %q", id)
			}
		})
	}
	t.Run("reader error", func(t *testing.T) {
		id, err := uuidV7(at, iotest.ErrReader(errors.New("entropy source failed")))
		if err == nil || id != "" {
			t.Fatalf("a failing random source returned identifier %q and error %v", id, err)
		}
	})
}
