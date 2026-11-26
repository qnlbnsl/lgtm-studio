package link

import (
	"bytes"
	"encoding/binary"
	"math"

	bolt "go.etcd.io/bbolt"
)

// Event is one numbered event waiting to be sent.
type Event struct {
	Seq     uint64
	Payload []byte
}

// Append stores a payload and returns its number after the commit.
func (s *Store) Append(payload []byte) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return 0, err
	}
	if len(payload) > s.opts.MaxEventBytes {
		return 0, ErrEventTooLarge
	}
	n := s.events.next
	if n > math.MaxInt64 {
		return 0, ErrSequenceExhausted
	}
	size := int64(len(payload))
	if s.events.count >= s.opts.MaxEvents || size > s.opts.MaxBytes-s.events.bytes {
		return 0, ErrOutboxFull
	}
	key, next := numberKey(n), numberKey(n+1)
	if err := s.update(func(tx *bolt.Tx) error {
		if err := tx.Bucket(eventsBucket).Put(key[:], payload); err != nil {
			return err
		}
		return tx.Bucket(metaBucket).Put(nextKey, next[:])
	}); err != nil {
		return 0, err
	}
	s.events.next++
	s.events.count++
	s.events.bytes += size
	select {
	case s.changed <- struct{}{}:
	default:
	}
	return n, nil
}

// Read returns, in order, at most limit events whose number is at least from.
func (s *Store) Read(from uint64, limit int) ([]Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return nil, err
	}
	if limit <= 0 || from >= s.events.next || s.events.count == 0 {
		return nil, nil
	}
	out := make([]Event, 0, min(limit, s.events.count))
	start := numberKey(from)
	err := s.db.View(func(tx *bolt.Tx) error {
		cursor := tx.Bucket(eventsBucket).Cursor()
		for key, payload := cursor.Seek(start[:]); key != nil && len(out) < limit; key, payload = cursor.Next() {
			out = append(out, Event{Seq: binary.BigEndian.Uint64(key), Payload: bytes.Clone(payload)})
		}
		return nil
	})
	return out, err
}

// Ack removes every stored event up to and including n.
func (s *Store) Ack(n uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return err
	}
	if n >= s.events.next {
		return ErrAckAhead
	}
	first := s.events.next - uint64(s.events.count)
	if n < first {
		return nil
	}
	var size int64
	if err := s.update(func(tx *bolt.Tx) error {
		events := tx.Bucket(eventsBucket)
		for seq := first; seq <= n; seq++ {
			key := numberKey(seq)
			size += int64(len(events.Get(key[:])))
			if err := events.Delete(key[:]); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	s.events.count -= int(n - first + 1)
	s.events.bytes -= size
	return nil
}

// Next is the number the next event gets.
func (s *Store) Next() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.events.next
}

// Len is the number of stored events.
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.events.count
}

// Bytes is the sum of the lengths of the stored payloads.
func (s *Store) Bytes() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.events.bytes
}

// Changed signals appends without blocking and is closed by Close.
func (s *Store) Changed() <-chan struct{} { return s.changed }
