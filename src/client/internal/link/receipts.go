package link

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"time"
	"unicode"

	bolt "go.etcd.io/bbolt"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

// Command is what the store needs to know about a command from the server.
type Command struct {
	ID        string
	ExpiresAt time.Time
}

// Decision says what the caller must do with a command.
type Decision int

const (
	DecisionRun Decision = 1 + iota
	DecisionRunning
	DecisionReplay
	DecisionInterrupted
	DecisionExpired
)

const (
	receiptStarted     = "started"
	receiptInterrupted = "interrupted"
	receiptFinished    = "finished"
	receiptAge         = 24 * time.Hour
)

type receipt struct {
	Version         int       `json:"version"`
	ExpiresAt       time.Time `json:"expires_at"`
	State           string    `json:"state"`
	Standing        bool      `json:"standing"`
	LastEventNumber *uint64   `json:"last_event_number"`
	Result          []byte    `json:"result,omitempty"`
}

type savedResult struct {
	outcome         linkv1.CommandOutcome
	lastEventNumber uint64
}

func (r savedResult) replayBytes(decision Decision) []byte {
	if decision == DecisionReplay {
		return []byte{byte(r.outcome)}
	}
	return nil
}

func readReceipt(data []byte, maxResult int) (receipt, []byte, bool) {
	if !uniqueReceiptFields(data) {
		return receipt{}, nil, false
	}

	// Raw fields distinguish absence from explicit null in both formats.
	var fields struct {
		receipt
		Standing        json.RawMessage `json:"standing"`
		LastEventNumber json.RawMessage `json:"last_event_number"`
		Result          json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(data, &fields); err != nil || fields.ExpiresAt.IsZero() {
		return receipt{}, nil, false
	}
	r := fields.receipt
	switch r.Version {
	case 1:
		if fields.Standing != nil || fields.LastEventNumber != nil || r.State == receiptInterrupted {
			return receipt{}, nil, false
		}
	case 2:
		switch string(fields.Standing) {
		case "true":
			r.Standing = true
		case "false":
		default:
			return receipt{}, nil, false
		}
		if r.State == receiptStarted {
			if string(fields.LastEventNumber) != "null" || r.Standing {
				return receipt{}, nil, false
			}
		} else {
			err := json.Unmarshal(fields.LastEventNumber, &r.LastEventNumber)
			if err != nil || r.LastEventNumber == nil || *r.LastEventNumber > math.MaxInt64 {
				return receipt{}, nil, false
			}
		}
	default:
		return receipt{}, nil, false
	}

	switch r.State {
	case receiptStarted, receiptInterrupted:
		return r, nil, fields.Result == nil && !r.Standing
	case receiptFinished:
		if len(fields.Result) == 0 || fields.Result[0] != '"' {
			return receipt{}, nil, false
		}
		if err := json.Unmarshal(fields.Result, &r.Result); err != nil ||
			len(r.Result) > maxResult || !validReceiptOutcome(r.Result) {
			return receipt{}, nil, false
		}
		if r.Standing && linkv1.CommandOutcome(r.Result[0]) != outcomeDone {
			return receipt{}, nil, false
		}
		return r, r.Result, true
	default:
		return receipt{}, nil, false
	}
}

func uniqueReceiptFields(data []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return false
	}
	seen := make(map[string]struct{})
	var value json.RawMessage
	for decoder.More() {
		token, err := decoder.Token()
		name, ok := token.(string)
		if err != nil || !ok {
			return false
		}
		// Go's JSON decoder matches struct fields using simple Unicode case folding.
		name = strings.Map(func(r rune) rune {
			smallest := r
			for candidate := unicode.SimpleFold(r); candidate != r; candidate = unicode.SimpleFold(candidate) {
				smallest = min(smallest, candidate)
			}
			return smallest
		}, name)
		if _, duplicate := seen[name]; duplicate {
			return false
		}
		seen[name] = struct{}{}
		if err := decoder.Decode(&value); err != nil {
			return false
		}
	}
	token, err := decoder.Token()
	return err == nil && token == json.Delim('}')
}

func validReceiptOutcome(result []byte) bool {
	return len(result) == 1 && result[0] >= byte(outcomeDone) && result[0] <= byte(outcomeInterrupted)
}

func (r receipt) watermark() uint64 {
	if r.LastEventNumber != nil {
		return *r.LastEventNumber
	}
	return 0
}

// prepareRun seals the starts left by the previous process before new appends.
func (s *Store) prepareRun() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return err
	}
	watermark := s.events.next - 1
	if watermark > math.MaxInt64 {
		return ErrStoreDamaged
	}

	type pendingReceipt struct {
		key  string
		data []byte
	}
	var pending []pendingReceipt
	if err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(receiptsBucket).ForEach(func(key, data []byte) error {
			r, _, valid := readReceipt(data, s.opts.MaxResultBytes)
			if !validCommandID(string(key)) || !valid || r.watermark() > watermark {
				return ErrStoreDamaged
			}
			if _, running := s.running[string(key)]; running || r.Version != 2 || r.State != receiptStarted {
				return nil
			}
			r.State = receiptInterrupted
			r.LastEventNumber = &watermark
			sealed, err := json.Marshal(r)
			if err != nil {
				return ErrStoreDamaged
			}
			pending = append(pending, pendingReceipt{key: string(key), data: sealed})
			return nil
		})
	}); err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}
	return s.update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(receiptsBucket)
		for _, r := range pending {
			if err := bucket.Put([]byte(r.key), r.data); err != nil {
				return err
			}
		}
		return nil
	})
}

// Begin records a new command before returning DecisionRun.
func (s *Store) Begin(c Command, now time.Time) (Decision, []byte, error) {
	decision, result, err := s.begin(c, now)
	return decision, result.replayBytes(decision), err
}

func (s *Store) begin(c Command, now time.Time) (Decision, savedResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return 0, savedResult{}, err
	}
	if !validReceiptCommand(c) || now.IsZero() {
		return 0, savedResult{}, ErrInvalidCommand
	}

	decision, result, err := s.findReceipt(c.ID)
	if err != nil || decision != 0 {
		return decision, result, err
	}
	if !now.Before(c.ExpiresAt) {
		return DecisionExpired, savedResult{}, nil
	}

	data, err := json.Marshal(receipt{Version: 2, ExpiresAt: c.ExpiresAt.UTC(), State: receiptStarted})
	if err != nil {
		return 0, savedResult{}, ErrInvalidCommand
	}
	if err := s.update(func(tx *bolt.Tx) error {
		return tx.Bucket(receiptsBucket).Put([]byte(c.ID), data)
	}); err != nil {
		return 0, savedResult{}, err
	}
	s.running[c.ID] = c.ExpiresAt
	return DecisionRun, savedResult{}, nil
}

// Decline records an expired command without reserving a start.
func (s *Store) Decline(c Command, result []byte) (Decision, []byte, error) {
	decision, saved, err := s.recordResult(c, result, false, time.Time{})
	if decision == 0 && err == nil {
		decision = DecisionExpired
	}
	return decision, saved.replayBytes(decision), err
}

// recordResult returns an existing decision or records a result without a start.
func (s *Store) recordResult(c Command, result []byte, standing bool, pruneTime time.Time) (Decision, savedResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return 0, savedResult{}, err
	}
	if !validReceiptCommand(c) {
		return 0, savedResult{}, ErrInvalidCommand
	}
	decision, saved, err := s.findReceipt(c.ID)
	if err != nil || decision != 0 {
		return decision, saved, err
	}
	saved, err = s.writeFinishedReceipt(c.ID, c.ExpiresAt, result, standing, pruneTime)
	return 0, saved, err
}

// findReceipt looks up a receipt while the store lock is held.
func (s *Store) findReceipt(id string) (Decision, savedResult, error) {
	key := []byte(id)
	var decision Decision
	var result savedResult
	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(receiptsBucket)
		data := bucket.Get(key)
		if data == nil && bucket.Bucket(key) == nil {
			return nil
		}
		r, payload, valid := readReceipt(data, s.opts.MaxResultBytes)
		if !valid || r.watermark() >= s.events.next {
			return ErrStoreDamaged
		}
		switch r.State {
		case receiptFinished:
			decision = DecisionReplay
			result = savedResult{outcome: linkv1.CommandOutcome(payload[0]), lastEventNumber: r.watermark()}
		case receiptInterrupted:
			decision = DecisionInterrupted
			result = savedResult{outcome: outcomeInterrupted, lastEventNumber: r.watermark()}
		case receiptStarted:
			if _, running := s.running[id]; running {
				decision = DecisionRunning
			} else {
				decision = DecisionInterrupted
				if r.Version == 1 {
					result.outcome = outcomeInterrupted
				}
				// An unsealed revision 2 start has no result to send until preparation.
			}
		}
		return nil
	})
	return decision, result, err
}

// Finish stores the command result.
func (s *Store) Finish(id string, result []byte) error {
	_, err := s.finish(id, result, time.Time{})
	return err
}

func (s *Store) finish(id string, result []byte, pruneTime time.Time) (savedResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return savedResult{}, err
	}
	expires, running := s.running[id]
	if !running {
		return savedResult{}, ErrNotRunning
	}
	saved, err := s.writeFinishedReceipt(id, expires, result, false, pruneTime)
	if err != nil {
		return savedResult{}, err
	}
	delete(s.running, id)
	return saved, nil
}

// writeFinishedReceipt saves and prunes receipts in one commit with the lock held.
func (s *Store) writeFinishedReceipt(id string, expires time.Time, result []byte, standing bool, pruneTime time.Time) (savedResult, error) {
	if len(result) > s.opts.MaxResultBytes {
		return savedResult{}, ErrResultTooLarge
	}
	if !validReceiptOutcome(result) || (standing && linkv1.CommandOutcome(result[0]) != outcomeDone) {
		return savedResult{}, ErrInvalidCommand
	}
	watermark := s.events.next - 1
	if watermark > math.MaxInt64 {
		return savedResult{}, ErrStoreDamaged
	}
	data, err := json.Marshal(receipt{
		Version: 2, ExpiresAt: expires.UTC(), State: receiptFinished,
		Standing: standing, LastEventNumber: &watermark, Result: result,
	})
	if err != nil {
		return savedResult{}, ErrInvalidCommand
	}
	if err := s.update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(receiptsBucket)
		if !pruneTime.IsZero() {
			if err := pruneReceipts(bucket, pruneTime.Add(-receiptAge), s.opts.MaxResultBytes, watermark); err != nil {
				return err
			}
		}
		return bucket.Put([]byte(id), data)
	}); err != nil {
		return savedResult{}, err
	}
	return savedResult{outcome: linkv1.CommandOutcome(result[0]), lastEventNumber: watermark}, nil
}

func pruneReceipts(bucket *bolt.Bucket, cutoff time.Time, maxResult int, watermark uint64) error {
	var expired []string
	err := bucket.ForEach(func(key, data []byte) error {
		r, _, valid := readReceipt(data, maxResult)
		if !valid || r.watermark() > watermark {
			return ErrStoreDamaged
		}
		if r.State == receiptFinished && !r.Standing && r.ExpiresAt.Before(cutoff) {
			expired = append(expired, string(key))
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, key := range expired {
		if err := bucket.Delete([]byte(key)); err != nil {
			return err
		}
	}
	return nil
}

func validReceiptCommand(c Command) bool {
	return validCommandID(c.ID) && !c.ExpiresAt.IsZero()
}

func validCommandID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i := range len(id) {
		c := id[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
				return false
			}
		}
	}
	return true
}
