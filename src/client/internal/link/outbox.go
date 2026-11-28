package link

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"math"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/proto"
)

// Event is one numbered event waiting to be sent.
type Event struct {
	Seq     uint64
	Payload []byte
}

type preparedReport func(context.Context) (uint64, error)

// Append stores a payload and returns its number after the commit.
func (s *Store) Append(payload []byte) (uint64, error) {
	return s.appendContext(context.Background(), payload)
}

// A folder settlement can hand the screened append back to the link owner.
// The refresh worker has no authority to append on that path.
func (s *Store) appendFilteredReport(ctx context.Context, filter *OutgoingFilter, event *linkv1.Event, prepared ...*preparedReport) (uint64, error) {
	screen := reportScreen(event)
	if len(prepared) != 0 {
		filter.refreshSecrets()
		unlock, err := filter.lockAfterRefresh(ctx, screen)
		if err != nil {
			return 0, err
		}
		unlock()
		*prepared[0] = func(appendCtx context.Context) (uint64, error) {
			unlock := filter.lockRegisteredSecrets()
			defer unlock()
			if ctx.Err() != nil || !filter.canScreen(screen) {
				return 0, errScreeningUnavailable
			}
			return s.appendScreenedReport(appendCtx, filter, event)
		}
		return 0, nil
	}
	unlock, err := filter.lockSecrets(ctx, screen)
	if err != nil {
		return 0, err
	}
	defer unlock()
	return s.appendScreenedReport(ctx, filter, event)
}

func (s *Store) appendScreenedReport(ctx context.Context, filter *OutgoingFilter, event *linkv1.Event) (uint64, error) {
	payload, err := filter.encodeFilteredReport(event)
	if err != nil {
		return 0, err
	}
	report, err := decodeReportCandidate(payload)
	if err != nil {
		return 0, errInvalidReport
	}
	number, err := s.appendAtNext(ctx, func(number uint64) ([]byte, error) {
		report.Number = number
		if proto.Size(report) <= eventWireLimit {
			return payload, nil
		}
		seen := report.GetFolderSeen()
		if seen.GetFacts() == nil {
			return nil, ErrEventTooLarge
		}
		seen.Result = &linkv1.FolderSeen_Refusal{Refusal: linkv1.FolderRefusal_FOLDER_REFUSAL_PATH_NOT_USABLE}
		if proto.Size(report) > eventWireLimit {
			return nil, ErrEventTooLarge
		}
		report.Number = 0
		return encodeReport(report)
	})
	if errors.Is(err, ErrSequenceExhausted) && s.Len() != 0 {
		// Let the last legal event drain before stopping for exhaustion.
		return 0, ErrOutboxFull
	}
	return number, err
}

func (s *Store) appendContext(ctx context.Context, payload []byte) (uint64, error) {
	return s.appendAtNext(ctx, func(uint64) ([]byte, error) { return payload, nil })
}

// appendAtNext prepares and commits a payload using the same proposed number.
func (s *Store) appendAtNext(ctx context.Context, prepare func(uint64) ([]byte, error)) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return 0, err
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	n := s.events.next
	payload, err := prepare(n)
	if err != nil {
		return 0, err
	}
	if len(payload) > s.opts.MaxEventBytes {
		return 0, ErrEventTooLarge
	}
	if n > math.MaxInt64 {
		return 0, ErrSequenceExhausted
	}
	size := int64(len(payload))
	if s.events.count >= s.opts.MaxEvents || size > s.opts.MaxBytes-s.events.bytes {
		return 0, ErrOutboxFull
	}
	key, next := numberKey(n), numberKey(n+1)
	if err := s.updateContext(ctx, func(tx *bolt.Tx) error {
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
