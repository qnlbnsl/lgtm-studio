package link

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/config"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/harness"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

func interceptScanAppend(t *testing.T, intercept func(*bolt.DB, func(*bolt.Tx) error) error) {
	t.Helper()
	writes := 0
	restore := swapCommit(func(db *bolt.DB, write func(*bolt.Tx) error) error {
		writes++
		// The start receipt is the first transaction. The inventory append follows it.
		if writes == 2 {
			return intercept(db, write)
		}
		return db.Update(write)
	})
	t.Cleanup(restore)
}

func TestCommandScanCancellationAtReportCommit(t *testing.T) {
	for _, tc := range []struct {
		name         string
		cancelBefore bool
		wantOutcome  linkv1.CommandOutcome
		wantNext     uint64
		wantLength   int
	}{
		{name: "before write admission", cancelBefore: true, wantOutcome: commandFailed, wantNext: 1, wantLength: 0},
		{name: "after successful commit", wantOutcome: commandDone, wantNext: 2, wantLength: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				folder, _ := newState(t)
				store := mustCreate(t, folder, StoreOptions{})
				log := new(reportLog)
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				interceptScanAppend(t, func(db *bolt.DB, write func(*bolt.Tx) error) error {
					if tc.cancelBefore {
						cancel()
					}
					err := db.Update(write)
					if !tc.cancelBefore && err == nil {
						cancel()
					}
					return err
				})
				r := newReporterWithContextAppend(config.Settings{}, "v", harness.ScanOptions{}, store.appendContext, log.logger(), fixtureReportProbes())
				f := newCommandFixture(t, store, r.scanHarnesses, nil, log.logger())
				cmd := scanCommand(1, 1, tNow.Add(time.Minute))
				if stop, err := f.handler.handle(ctx, cmd); stop || err != nil {
					t.Fatalf("scan admission: restart %v, error %v", stop, err)
				}
				if err := completeScan(t, f); err != nil {
					t.Fatalf("finishing scan: %v", err)
				}
				if ctx.Err() == nil {
					t.Fatal("the inventory commit hook did not cancel the scan")
				}

				store = reopen(t, folder, store, StoreOptions{})
				if next, length := store.Next(), store.Len(); next != tc.wantNext || length != tc.wantLength {
					t.Errorf("reopened outbox next = %d, length = %d. Want %d, %d", next, length, tc.wantNext, tc.wantLength)
				}
				f.reply(t, cmd, tc.wantOutcome)
				requireReceipt(t, store, receiptCommand(cmd), DecisionReplay, []byte{byte(tc.wantOutcome)})
				if tc.cancelBefore {
					log.sequences(t)
				} else {
					if event, _ := readReport(t, store, 1); event.GetHarnessesFound() == nil {
						t.Error("the committed inventory did not survive reopening")
					}
					log.sequences(t, 1)
				}
			})
		})
	}
}

func TestCommandScanAppendFailureRedactsStatePath(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		folder, dir := newState(t)
		store := mustCreate(t, folder, StoreOptions{})
		log := new(reportLog)
		interceptScanAppend(t, func(*bolt.DB, func(*bolt.Tx) error) error {
			return &os.PathError{Op: "write", Path: filepath.Join(dir, "client.db"), Err: errors.New("fixture write failed")}
		})
		r := newReporter(config.Settings{}, "v", harness.ScanOptions{}, store.Append, log.logger(), fixtureReportProbes())
		var reportErr error
		f := newCommandFixture(t, store, func(ctx context.Context, requested bool) (bool, error) {
			committed, err := r.scanHarnesses(ctx, requested)
			reportErr = err
			return committed, err
		}, nil, log.logger())
		cmd := scanCommand(1, 1, tNow.Add(time.Minute))
		requireContinues(t, f, cmd)
		commandErr := completeScan(t, f)

		for _, boundary := range []struct {
			name string
			err  error
		}{{"reporter", reportErr}, {"command handler", commandErr}} {
			if !errors.Is(boundary.err, ErrStoreFailed) {
				t.Errorf("%s error does not match ErrStoreFailed", boundary.name)
			}
			if boundary.err != nil && strings.Contains(boundary.err.Error(), dir) {
				t.Errorf("%s error text contains the state folder path", boundary.name)
			}
		}
		decoder := json.NewDecoder(strings.NewReader(log.buffer.String()))
		for {
			token, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal("a log record is not valid JSON")
			}
			if field, ok := token.(string); ok && strings.Contains(field, dir) {
				t.Error("a log field contains the state folder path")
			}
		}
		log.sequences(t)
		f.noReply(t)
	})
}
