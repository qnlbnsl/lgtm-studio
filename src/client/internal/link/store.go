// Package link holds the events and command receipts that the client keeps for the server.
package link

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	bolt "go.etcd.io/bbolt"
	boltErrors "go.etcd.io/bbolt/errors"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/state"
)

// StoreOptions are the limits of a store. A zero value uses the default.
type StoreOptions struct {
	MaxEvents      int
	MaxBytes       int64
	MaxEventBytes  int
	MaxResultBytes int
}

type eventSnapshot struct {
	next  uint64
	count int
	bytes int64
}

// Store holds the open database and its last committed event counts.
type Store struct {
	mu      sync.Mutex
	db      *bolt.DB
	release func()
	failed  error
	fenced  bool
	opts    StoreOptions
	events  eventSnapshot
	changed chan struct{}
	running map[string]time.Time
}

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

var (
	metaBucket     = []byte("meta")
	eventsBucket   = []byte("events")
	receiptsBucket = []byte("receipts")
	versionKey     = []byte("version")
	nextKey        = []byte("next")
	storeVersion   = []byte("1")
)

// commit is the one way every application transaction writes the file. Tests replace it.
var commit = func(db *bolt.DB, fn func(*bolt.Tx) error) error { return db.Update(fn) }

// wallClock lets the receipt tests set a local time independently of the server's time.
var wallClock = time.Now

// createHook observes the four steps of store creation in the crash tests.
var createHook func(step string)

const storeLockTimeout = time.Second

// CreateStore makes client.db when it is missing and otherwise opens it.
func CreateStore(f *state.Folder, opts StoreOptions) (*Store, error) {
	return openStore(f, opts, true)
}

// OpenStore opens an existing client.db only.
func OpenStore(f *state.Folder, opts StoreOptions) (*Store, error) {
	return openStore(f, opts, false)
}

func openStore(f *state.Folder, opts StoreOptions, create bool) (_ *Store, err error) {
	opts, err = withStoreDefaults(opts)
	if err != nil {
		return nil, err
	}
	paths, release, err := f.Claim("store")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			release()
		}
	}()

	info, err := os.Lstat(paths.Store)
	if errors.Is(err, os.ErrNotExist) && create {
		if err := createStoreFile(paths); err != nil {
			return nil, err
		}
		info, err = os.Lstat(paths.Store)
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%s: %w", state.StoreFile, ErrStoreMissing)
	}
	if err != nil {
		return nil, storeOpenError(err)
	}
	if err := checkStoreFile(info); err != nil {
		return nil, storeOpenError(err)
	}

	events, err := inspectStore(paths.Store, info)
	if err != nil {
		return nil, storeOpenError(err)
	}
	db, _, err := openExistingBolt(paths.Store, info, false)
	if err != nil {
		return nil, storeOpenError(err)
	}
	if err := state.SyncDir(paths.Dir); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{
		db: db, release: release, opts: opts, events: events,
		changed: make(chan struct{}, 1),
		running: make(map[string]time.Time),
	}, nil
}

func withStoreDefaults(opts StoreOptions) (StoreOptions, error) {
	if opts.MaxEvents < 0 || opts.MaxBytes < 0 || opts.MaxEventBytes < 0 || opts.MaxResultBytes < 0 {
		return StoreOptions{}, errors.New("link: store limits must be nonnegative")
	}
	if opts.MaxEvents == 0 {
		opts.MaxEvents = 10_000
	}
	if opts.MaxBytes == 0 {
		opts.MaxBytes = 64 << 20
	}
	if opts.MaxEventBytes == 0 {
		opts.MaxEventBytes = 1 << 20
	}
	if opts.MaxResultBytes == 0 {
		opts.MaxResultBytes = 64 << 10
	}
	return opts, nil
}

func createStoreFile(paths state.Paths) error {
	f, err := reserveStoreTemp(paths.Dir)
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	creationStep("temp-created")

	db, err := bolt.Open(temp, 0o600, &bolt.Options{
		Timeout:  storeLockTimeout,
		OpenFile: func(string, int, os.FileMode) (*os.File, error) { return f, nil },
	})
	if err != nil {
		return err
	}
	err = commit(db, initializeStore)
	if err := errors.Join(err, db.Close()); err != nil {
		return err
	}
	creationStep("temp-initialized")
	if err := os.Link(temp, paths.Store); err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return err
	}
	creationStep("placed")
	if err := os.Remove(temp); err != nil {
		return err
	}
	if err := state.SyncDir(paths.Dir); err != nil {
		return err
	}
	creationStep("dir-synced")
	return nil
}

func reserveStoreTemp(dir string) (*os.File, error) {
	for {
		var random [8]byte
		rand.Read(random[:])
		path := filepath.Join(dir, state.TempPrefix+hex.EncodeToString(random[:])+".db")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
		if !errors.Is(err, os.ErrExist) {
			return f, err
		}
	}
}

func initializeStore(tx *bolt.Tx) error {
	for _, name := range [][]byte{metaBucket, eventsBucket, receiptsBucket} {
		if _, err := tx.CreateBucket(name); err != nil {
			return err
		}
	}
	meta := tx.Bucket(metaBucket)
	if err := meta.Put(versionKey, storeVersion); err != nil {
		return err
	}
	one := numberKey(1)
	return meta.Put(nextKey, one[:])
}

func creationStep(step string) {
	if createHook != nil {
		createHook(step)
	}
}

func checkStoreFile(info os.FileInfo) error {
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 ||
		info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
		return ErrStoreUnsafe
	}
	if info.Size() == 0 {
		return ErrStoreDamaged
	}
	return nil
}

func openExistingBolt(path string, info os.FileInfo, readOnly bool) (db *bolt.DB, size int64, err error) {
	var file *os.File
	// bbolt can panic when memory-mapped metadata is damaged.
	old := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(old)
	defer func() {
		if recover() != nil {
			if file != nil {
				_ = file.Close()
			}
			db, err = nil, ErrStoreDamaged
		}
	}()

	db, err = bolt.Open(path, 0o600, &bolt.Options{
		Timeout:  storeLockTimeout,
		ReadOnly: readOnly,
		// Suppress bbolt's automatic write on open when it reconstructs a freelist.
		NoFreelistSync: true,
		OpenFile: func(path string, flags int, mode os.FileMode) (*os.File, error) {
			file, err = os.OpenFile(path, flags&^os.O_CREATE|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, mode)
			if err != nil {
				return nil, err
			}
			current, statErr := file.Stat()
			if statErr == nil {
				statErr = checkStoreFile(current)
			}
			if statErr == nil && !os.SameFile(info, current) {
				statErr = ErrStoreUnsafe
			}
			if statErr != nil {
				_ = file.Close()
				return nil, statErr
			}
			size = current.Size()
			return file, nil
		},
	})
	if err == nil && !readOnly {
		db.NoFreelistSync = false
	}
	return db, size, err
}

func inspectStore(path string, info os.FileInfo) (snapshot eventSnapshot, err error) {
	db, fileSize, err := openExistingBolt(path, info, true)
	if err != nil {
		return eventSnapshot{}, err
	}
	defer func() { err = errors.Join(err, db.Close()) }()

	old := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(old)
	defer func() {
		if recover() != nil {
			err = ErrStoreDamaged
		}
	}()
	err = db.View(func(tx *bolt.Tx) error {
		// A truncated file can still have readable metadata. Check its size
		// before bbolt follows any pointers into the mapped pages.
		if size := tx.Size(); size <= 0 || size > fileSize {
			return ErrStoreDamaged
		}
		var checkErr error
		snapshot, checkErr = inspectBuckets(tx)
		return checkErr
	})
	return snapshot, err
}

func inspectBuckets(tx *bolt.Tx) (eventSnapshot, error) {
	meta, events := tx.Bucket(metaBucket), tx.Bucket(eventsBucket)
	if meta == nil || events == nil || tx.Bucket(receiptsBucket) == nil || string(meta.Get(versionKey)) != string(storeVersion) {
		return eventSnapshot{}, ErrStoreDamaged
	}
	next := meta.Get(nextKey)
	if len(next) != 8 {
		return eventSnapshot{}, ErrStoreDamaged
	}
	snapshot := eventSnapshot{next: binary.BigEndian.Uint64(next)}
	if snapshot.next == 0 || snapshot.next > math.MaxInt64+1 {
		return eventSnapshot{}, ErrStoreDamaged
	}
	var previous uint64
	cursor := events.Cursor()
	for key, payload := cursor.First(); key != nil; key, payload = cursor.Next() {
		if len(key) != 8 || payload == nil {
			return eventSnapshot{}, ErrStoreDamaged
		}
		n := binary.BigEndian.Uint64(key)
		if n == 0 || n >= snapshot.next || (previous != 0 && n != previous+1) ||
			int64(len(payload)) > math.MaxInt64-snapshot.bytes {
			return eventSnapshot{}, ErrStoreDamaged
		}
		previous = n
		snapshot.count++
		snapshot.bytes += int64(len(payload))
	}
	if previous != 0 && previous != snapshot.next-1 {
		return eventSnapshot{}, ErrStoreDamaged
	}
	return snapshot, nil
}

// checkRunReceipts checks the saved envelopes before the first dial. Sealing
// interrupted receipts waits for a Welcome that passes replay validation.
func (s *Store) checkRunReceipts() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(receiptsBucket).ForEach(func(key, data []byte) error {
			r, _, valid := readReceipt(data, s.opts.MaxResultBytes)
			if !validCommandID(string(key)) || !valid || r.watermark() >= s.events.next {
				return ErrStoreDamaged
			}
			return nil
		})
	})
}

func storeOpenError(err error) error {
	var reason error
	switch {
	case errors.Is(err, boltErrors.ErrTimeout):
		reason = state.ErrStoreOpen
	case errors.Is(err, ErrStoreUnsafe), errors.Is(err, syscall.ELOOP):
		reason = ErrStoreUnsafe
	case errors.Is(err, ErrStoreDamaged), errors.Is(err, boltErrors.ErrInvalid),
		errors.Is(err, boltErrors.ErrChecksum), errors.Is(err, boltErrors.ErrVersionMismatch):
		reason = ErrStoreDamaged
	default:
		reason = ErrStoreFailed
	}
	return fmt.Errorf("%s: %w", state.StoreFile, reason)
}

// Close closes Changed, the database and the claim.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	close(s.changed)
	err := s.db.Close()
	s.db = nil
	s.release()
	return err
}

func (s *Store) ready() error {
	if s.db == nil {
		return ErrStoreClosed
	}
	return s.failed
}

func (s *Store) update(fn func(*bolt.Tx) error) error {
	return s.updateContext(context.Background(), fn)
}

func (s *Store) fenceWrites() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fenced = true
}

func (s *Store) updateContext(ctx context.Context, fn func(*bolt.Tx) error) error {
	if s.fenced {
		return context.Canceled
	}
	var refused error
	if err := commit(s.db, func(tx *bolt.Tx) error {
		// The store lock is held. Check cancellation before admitting the write.
		refused = ctx.Err()
		if refused != nil {
			return refused
		}
		return fn(tx)
	}); err != nil {
		if refused != nil && err == refused {
			return refused
		}
		s.failed = fmt.Errorf("%s: %w: %w", state.StoreFile, ErrStoreFailed, err)
		return s.failed
	}
	return nil
}

// noSync reports whether commits bypass the database sync.
func (s *Store) noSync() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db != nil && s.db.NoSync
}

func numberKey(n uint64) [8]byte {
	var key [8]byte
	binary.BigEndian.PutUint64(key[:], n)
	return key
}
