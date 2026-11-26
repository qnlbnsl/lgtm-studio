// Package link holds what the client keeps for its link to the server: the events it must send and the receipts of the commands it ran.
package link

import (
	"errors"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/state"
)

// This file is a stub of the test writer. The builder replaces every body.

// StoreOptions are the limits of a store. A 0 means the default, and a negative value is refused.
type StoreOptions struct {
	MaxEvents      int
	MaxBytes       int64
	MaxEventBytes  int
	MaxResultBytes int
}

// Store is the one file client.db with the events and the receipts.
type Store struct{}

// The errors of the store.
var (
	ErrOutboxFull        = errors.New("link: outbox is full")
	ErrEventTooLarge     = errors.New("link: event is too large")
	ErrAckAhead          = errors.New("link: acknowledgement is ahead of the last number")
	ErrInvalidCommand    = errors.New("link: invalid command")
	ErrNotRunning        = errors.New("link: command is not running")
	ErrResultTooLarge    = errors.New("link: result is too large")
	ErrStoreFailed       = errors.New("link: store has failed")
	ErrStoreDamaged      = errors.New("link: store is damaged")
	ErrStoreUnsafe       = errors.New("link: store file is unsafe")
	ErrStoreMissing      = errors.New("link: store is missing")
	ErrSequenceExhausted = errors.New("link: event numbers are exhausted")
	ErrStoreClosed       = errors.New("link: store is closed")
)

// commit is the one way every write reaches the file. Tests replace it.
var commit = func(db *bolt.DB, fn func(*bolt.Tx) error) error { return db.Update(fn) }

// wallClock is the machine's clock, used for pruning. Tests replace it.
var wallClock = time.Now

// createHook is called at the steps temp-created, temp-initialized, placed and dir-synced while a store is made.
var createHook func(step string)

// CreateStore makes client.db when it is missing and otherwise opens it.
func CreateStore(f *state.Folder, opts StoreOptions) (*Store, error) {
	return nil, errors.New("stub of the test writer")
}

// OpenStore opens an existing client.db only, and returns ErrStoreMissing when there is none.
func OpenStore(f *state.Folder, opts StoreOptions) (*Store, error) {
	return nil, errors.New("stub of the test writer")
}

// Close closes Changed, the file and the claim, and returns nil the second time.
func (s *Store) Close() error { return nil }

// noSync tells whether the file was opened without syncing. It is the unexported accessor that the tests read.
func (s *Store) noSync() bool { return true }
