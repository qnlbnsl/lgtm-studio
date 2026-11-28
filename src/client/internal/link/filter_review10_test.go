package link

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
)

func TestOutgoingFilterScreensTextDuringOverlappingSignInRefreshes(t *testing.T) {
	const first, replacement = "review10-synthetic-first", "review10-synthetic-replacement"
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "sign-in")
	if err := os.WriteFile(path, []byte(first), 0o600); err != nil {
		t.Fatal(err)
	}
	next := filepath.Join(dir, "replacement")
	if err := os.WriteFile(next, []byte(replacement), 0o600); err != nil {
		t.Fatal(err)
	}

	synctest.Test(t, func(t *testing.T) {
		sampled := make(chan string, 1)
		releaseFirst := make(chan struct{})
		release := sync.OnceFunc(func() { close(releaseFirst) })
		defer release()
		var firstRead atomic.Bool
		filter := NewRefreshingOutgoingFilter(func() []string {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Error("the refresh could not read the disposable sign-in file")
				return nil
			}
			value := string(data)
			if firstRead.CompareAndSwap(false, true) {
				sampled <- value
				<-releaseFirst
			}
			return []string{value}
		})
		firstDone := make(chan struct{})
		go func() {
			defer close(firstDone)
			filter.WithTextMask(func(mask func(string) string) { _ = mask("first refresh") })
		}()
		synctest.Wait()
		select {
		case value := <-sampled:
			if value != first {
				t.Fatal("the first refresh did not sample the original sign-in file")
			}
		default:
			t.Fatal("the first refresh did not reach the file read")
		}
		if err := os.Rename(next, path); err != nil {
			t.Fatal(err)
		}

		input := "before " + first + " / " + replacement + " after"
		emitted := make(chan string, 1)
		secondDone := make(chan struct{})
		go func() {
			defer close(secondDone)
			filter.WithTextMask(func(mask func(string) string) { emitted <- mask(input) })
		}()
		synctest.Wait()
		checkEmission := func() {
			t.Helper()
			select {
			case got := <-emitted:
				if strings.Contains(got, first) || strings.Contains(got, replacement) {
					t.Error("dynamic text exposed a sampled sign-in value during overlapping refreshes")
				}
			default:
			}
		}
		checkEmission()
		release()
		<-firstDone
		<-secondDone
		checkEmission()

		const want = "before [redacted] / [redacted] after"
		var got string
		filter.WithTextMask(func(mask func(string) string) { got = mask(input) })
		if got != want {
			t.Error("after both refreshes, the filter did not mask both sign-in values and preserve the surrounding text")
		}
	})
}
