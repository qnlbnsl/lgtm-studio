package link

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	boltErrors "go.etcd.io/bbolt/errors"
)

func TestRevisionTwoStoreOpenErrorClassification(t *testing.T) {
	for _, cause := range []error{boltErrors.ErrInvalid, boltErrors.ErrChecksum, boltErrors.ErrVersionMismatch} {
		err := storeOpenError(fmt.Errorf("open metadata: %w", cause))
		if !errors.Is(err, ErrStoreDamaged) || errors.Is(err, ErrStoreFailed) {
			t.Errorf("storeOpenError(%v) = %v, want ErrStoreDamaged", cause, err)
		}
	}

	err := storeOpenError(&os.PathError{Op: "open", Path: "client.db", Err: syscall.EIO})
	if !errors.Is(err, ErrStoreFailed) || errors.Is(err, ErrStoreDamaged) {
		t.Errorf("storeOpenError for an input/output error = %v, want ErrStoreFailed", err)
	}
}

func TestRevisionTwoStoreOpenPermissionFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can open the fixture despite its file mode")
	}
	for _, tc := range []struct {
		name  string
		mode  os.FileMode
		flags int
	}{
		{name: "unreadable", mode: 0o200, flags: os.O_RDONLY},
		{name: "read-only", mode: 0o400, flags: os.O_RDWR},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := newRunPeerFixture(t, protocols[0])
				dials := countRecoveryDials(t)
				f.seed(t, fixtureFactsPayload(t, "saved"))
				path := f.file()
				before := readBytes(t, path)
				if err := os.Chmod(path, tc.mode); err != nil {
					t.Fatal("could not restrict access to the fixture store")
				}
				file, err := os.OpenFile(path, tc.flags, 0)
				if file != nil {
					_ = file.Close()
				}
				if !errors.Is(err, os.ErrPermission) {
					t.Fatalf("fixture open returned %v, want a permission error", err)
				}

				store, err := OpenStore(f.opts.Folder, StoreOptions{})
				if store != nil {
					_ = store.Close()
				}
				if store != nil || !errors.Is(err, ErrStoreFailed) || errors.Is(err, ErrStoreDamaged) {
					t.Errorf("OpenStore returned a store = %t and error %v, want no store and ErrStoreFailed", store != nil, err)
				}

				ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
				defer cancel()
				outcome, err := Run(ctx, f.opts)
				if outcome != RunStopped || !errors.Is(err, ErrStoreFailed) || errors.Is(err, ErrStoreDamaged) {
					t.Errorf("Run returned outcome %v and error %v, want RunStopped and ErrStoreFailed", outcome, err)
				}
				if got := dials.Load(); got != 0 {
					t.Errorf("store open failure initialized transport %d times, want zero", got)
				}
				plainFailures := 0
				for _, line := range f.diagnostics(t) {
					message := strings.ToLower(line.Message)
					if strings.Contains(message, "restor") || strings.Contains(message, "recover") || strings.Contains(message, "enroll") {
						t.Error("the client gave a history recovery instruction for a permission error")
					}
					if line.Level == "ERROR" && line.Msg == "link_stopped" && line.Code == "store_failed" && line.Message == "Could not use client.db." {
						plainFailures++
					}
				}
				if plainFailures != 1 {
					t.Errorf("the client logged %d plain store failures, want one", plainFailures)
				}

				if err := os.Chmod(path, 0o600); err != nil {
					t.Fatal("could not restore fixture access to check its bytes")
				}
				if !bytes.Equal(before, readBytes(t, path)) {
					t.Error("the open failure changed the saved database bytes")
				}
			})
		})
	}
}
