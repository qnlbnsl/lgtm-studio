package link

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestFolderCommandHeldStartObserver(t *testing.T) {
	for _, name := range []string{"expiry", "cancellation"} {
		t.Run(name, func(t *testing.T) {
			withFolderInspector(t, func(t *testing.T, calls <-chan *folderReadCall) {
				folder, _ := newState(t)
				store := mustCreate(t, folder, StoreOptions{})
				f := newCommandFixture(t, store, nil, nil, nil)
				cmd := folderTestCommand(1)
				cmd.ExpiresAt = timestamppb.New(tNow.Add(time.Second))
				disk := observeFolderDisk(t, "", cmd)

				started := make(chan context.Context, 1)
				held := make(chan struct{})
				release := sync.OnceFunc(func() { close(held) })
				defer release()
				hooks := folderInspectionHooks{onStart: func(ctx context.Context, _, _ string) {
					started <- ctx
					<-held
				}}
				ctx, cancel := context.WithCancel(context.WithValue(t.Context(), folderInspectionHooksKey{}, hooks))
				defer cancel()
				if restart, err := f.handler.handle(ctx, cmd); restart || err != nil {
					t.Fatalf("handling the folder command returned restart %t and error %v", restart, err)
				}
				readCtx := receiveNow(t, started, "the inspection did not reach the start observer")
				requireEmpty(t, calls, "the reader ran before the observer returned")
				if disk.record(t, cmd).receipt.State != receiptStarted {
					t.Fatal("the start observer did not hold a started command")
				}

				if name == "cancellation" {
					cancel()
					receiveNow(t, readCtx.Done(), "process cancellation did not reach the held observer")
					release()
					requireEmpty(t, calls, "the reader ran after the held observer's context was canceled")
					f.handler.stopFolder()
					return
				}

				// Advance only the authenticated clock so the reader still has its full budget.
				f.elapsed.Add(int64(2 * time.Second))
				if readCtx.Err() != nil {
					t.Fatal("the read budget ended before the held observer was released")
				}
				release()
				synctest.Wait()
				select {
				case call := <-calls:
					t.Error("the reader ran after the held observer crossed the command expiry")
					call.result <- folderReadResult{facts: folderTestFacts()}
				default:
				}
				inspection := receiveNow(t, f.handler.folderCompleted, "the expired inspection did not complete")
				if err := f.handler.finishFolder(ctx, inspection); err != nil {
					t.Fatalf("settling the expired inspection returned %v", err)
				}
				record := disk.record(t, cmd)
				if next := disk.position(t).next; record.reportCommits != 0 || next != 1 {
					t.Errorf("the expired inspection committed %d reports and left next event number %d, want zero reports and next event number 1", record.reportCommits, next)
				}
				reply := receiveNow(t, f.replies, "the expired command did not return a result")
				requireFolderCommandResult(t, reply.result, cmd, commandExpired, 0)
				requireFolderReceipt(t, record, commandExpired, 0)
			})
		})
	}
}
