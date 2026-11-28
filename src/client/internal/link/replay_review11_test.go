package link

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func TestReplayAtTailWhileAppendingReview11(t *testing.T) {
	// Two scheduler threads let the readers and writers compete for the next lock.
	previous := runtime.GOMAXPROCS(2)
	defer runtime.GOMAXPROCS(previous)

	folder, _ := newState(t)
	store := mustCreate(t, folder, StoreOptions{})
	// The race concerns committed rows in this process. Disk synchronization
	// would limit how many appends can compete with replay construction.
	store.db.NoSync = true
	payload := storedReport(t, replayFacts(tNow))
	appendLinkFixture(t, store, payload)

	const writers, attempts = 3, 6000
	start := make(chan struct{})
	stop := make(chan struct{})
	writerErrors := make(chan error, writers)
	var appends atomic.Int64
	var running sync.WaitGroup
	for range writers {
		running.Go(func() {
			<-start
			for {
				select {
				case <-stop:
					return
				default:
				}
				if _, err := store.Append(payload); err != nil {
					if !errors.Is(err, ErrOutboxFull) {
						writerErrors <- err
						return
					}
				} else {
					appends.Add(1)
				}
				runtime.Gosched()
			}
		})
	}
	defer func() {
		close(stop)
		running.Wait()
		close(writerErrors)
		for err := range writerErrors {
			t.Errorf("the append fixture failed: %v", err)
		}
	}()
	close(start)

	deadline := time.Now().Add(5 * time.Second)
	for i := range attempts {
		through := store.Next() - 1
		if _, err := newReplayWithFilter(store, through, nil); err != nil {
			t.Fatalf("building a replay at watermark %d while appending valid reports failed on attempt %d after %d concurrent appends: %v",
				through, i+1, appends.Load(), err)
		}
		if time.Now().After(deadline) {
			break
		}
		runtime.Gosched()
	}
	if appends.Load() == 0 {
		t.Fatal("the writers did not append during the replay stress case")
	}
}

func TestReplayValidationRejectsUnscreenedRetainedHistoryReview11(t *testing.T) {
	folder, _ := newState(t)
	store := mustCreate(t, folder, StoreOptions{})
	if got := appendLinkFixture(t, store, storedReport(t, replayFacts(tNow))); got != 1 {
		t.Fatalf("the fixture appended event %d, want 1", got)
	}
	filter := NewOutgoingFilter()
	if err := store.validateReplay(0, filter, false); !errors.Is(err, ErrStoreDamaged) {
		t.Errorf("validating event 1 beyond an unscreened ceiling of 0 returned %v, want %v", err, ErrStoreDamaged)
	}
}

func TestFolderPathSampleIsRegisteredBeforeTextEmissionReview11(t *testing.T) {
	const first, replacement = "review11-synthetic-path-first", "review11-synthetic-path-replacement"
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path, next := filepath.Join(dir, "sign-in"), filepath.Join(dir, "replacement")
	if err := os.WriteFile(path, []byte(first), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(next, []byte(replacement), 0o600); err != nil {
		t.Fatal(err)
	}

	synctest.Test(t, func(t *testing.T) {
		sampled := make(chan struct{})
		release := make(chan struct{})
		markSampled := sync.OnceFunc(func() { close(sampled) })
		releaseSample := sync.OnceFunc(func() { close(release) })
		defer releaseSample()
		filter := NewRefreshingOutgoingFilter(func() []string {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("could not read the disposable sign-in file: %v", err)
				return nil
			}
			value := string(data)
			if value == first {
				markSampled()
				<-release
			}
			return []string{value}
		})
		pathUnsafe := make(chan bool, 1)
		go func() {
			pathUnsafe <- filter.folderPathUnsafe(filepath.Join(dir, first), dir)
		}()
		synctest.Wait()
		select {
		case <-sampled:
		default:
			t.Fatal("the folder path check did not sample the original sign-in file")
		}
		if err := os.Rename(next, path); err != nil {
			t.Fatal(err)
		}

		input := "before " + first + " / " + replacement + " after"
		emitted := make(chan string, 1)
		textStarted := make(chan struct{})
		pathOwnsLock := signInReadHoldsFilterLock(filter)
		go func() {
			close(textStarted)
			emitted <- filter.MaskText(input)
		}()
		<-textStarted
		if !pathOwnsLock {
			synctest.Wait()
		}
		releaseSample()
		if got := <-emitted; strings.Contains(got, first) || strings.Contains(got, replacement) {
			t.Error("the text gate emitted a value sampled by the folder path check before its registration")
		}
		if !<-pathUnsafe {
			t.Error("the folder path check did not refuse the path containing its sample")
		}
		if got := filter.MaskText(input); got != "before [redacted] / [redacted] after" {
			t.Error("the text gate did not mask both values after registration and preserve the surrounding text")
		}
	})
}
