package link

import (
	"encoding/base64"
	"encoding/json"
	"time"

	bolt "go.etcd.io/bbolt"
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
	receiptStarted  = "started"
	receiptFinished = "finished"
	receiptAge      = 24 * time.Hour
)

type receipt struct {
	Version   int             `json:"version"`
	ExpiresAt time.Time       `json:"expires_at"`
	State     string          `json:"state"`
	Result    json.RawMessage `json:"result,omitempty"`
}

func readReceipt(data []byte, maxResult int) (receipt, []byte, bool) {
	var r receipt
	if err := json.Unmarshal(data, &r); err != nil || r.Version != 1 || r.ExpiresAt.IsZero() {
		return receipt{}, nil, false
	}
	switch r.State {
	case receiptStarted:
		return r, nil, len(r.Result) == 0
	case receiptFinished:
		if len(r.Result) == 0 || r.Result[0] != '"' {
			return receipt{}, nil, false
		}
		var encoded string
		if err := json.Unmarshal(r.Result, &encoded); err != nil {
			return receipt{}, nil, false
		}
		result, err := base64.StdEncoding.DecodeString(encoded)
		return r, result, err == nil && len(result) <= maxResult
	default:
		return receipt{}, nil, false
	}
}

// Begin records a new command before returning DecisionRun.
func (s *Store) Begin(c Command, now time.Time) (Decision, []byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return 0, nil, err
	}
	if !validCommandID(c.ID) || c.ExpiresAt.IsZero() || now.IsZero() {
		return 0, nil, ErrInvalidCommand
	}

	key := []byte(c.ID)
	var decision Decision
	var result []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(receiptsBucket)
		data := bucket.Get(key)
		if data == nil && bucket.Bucket(key) == nil {
			return nil
		}
		decision = DecisionInterrupted
		r, payload, valid := readReceipt(data, s.opts.MaxResultBytes)
		if valid {
			if r.State == receiptFinished {
				decision, result = DecisionReplay, payload
			} else if _, running := s.running[c.ID]; running {
				decision = DecisionRunning
			}
		}
		return nil
	})
	if err != nil || decision != 0 {
		return decision, result, err
	}
	if !now.Before(c.ExpiresAt) {
		return DecisionExpired, nil, nil
	}

	data, err := json.Marshal(receipt{Version: 1, ExpiresAt: c.ExpiresAt.UTC(), State: receiptStarted})
	if err != nil {
		return 0, nil, ErrInvalidCommand
	}
	if err := s.update(func(tx *bolt.Tx) error {
		return tx.Bucket(receiptsBucket).Put(key, data)
	}); err != nil {
		return 0, nil, err
	}
	s.running[c.ID] = c.ExpiresAt
	return DecisionRun, nil, nil
}

// Finish stores the command result and prunes expired finished receipts.
func (s *Store) Finish(id string, result []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return err
	}
	expires, running := s.running[id]
	if !running {
		return ErrNotRunning
	}
	if len(result) > s.opts.MaxResultBytes {
		return ErrResultTooLarge
	}
	encoded, err := json.Marshal(base64.StdEncoding.EncodeToString(result))
	if err != nil {
		return err
	}
	data, err := json.Marshal(receipt{Version: 1, ExpiresAt: expires.UTC(), State: receiptFinished, Result: encoded})
	if err != nil {
		return err
	}
	cutoff := wallClock().Add(-receiptAge)
	if err := s.update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(receiptsBucket)
		if err := bucket.Put([]byte(id), data); err != nil {
			return err
		}
		return pruneReceipts(bucket, cutoff, s.opts.MaxResultBytes)
	}); err != nil {
		return err
	}
	delete(s.running, id)
	return nil
}

func pruneReceipts(bucket *bolt.Bucket, cutoff time.Time, maxResult int) error {
	var expired []string
	err := bucket.ForEach(func(key, data []byte) error {
		if r, _, valid := readReceipt(data, maxResult); valid && r.State == receiptFinished && r.ExpiresAt.Before(cutoff) {
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
