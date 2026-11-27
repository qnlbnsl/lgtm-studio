package linktest

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	bolt "go.etcd.io/bbolt"
	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/types/known/timestamppb"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
)

func TestLinkCommandAttemptsAcrossReconnect(t *testing.T) {
	requirePostgres(t)
	t.Run("active scan", func(t *testing.T) {
		f := newCommandFixture(t, relayControls{})
		_, firstLeg := f.startApproved(t, 0)
		firstGeneration := generation(t, f)
		f.probe.arm(t)
		requestScan(t, f)
		first := waitForCommand(t, firstLeg, scanAction, 0)
		f.probe.heldPID(t)
		f.probe.setVersion(t, "2.0.0")

		firstLeg.cut()
		receive(t, firstLeg.done, "the first command stream to close")
		retryLeg := receive(t, f.relay.legs, "a new stream on the same client state")
		retry := waitForCommand(t, retryLeg, scanAction, 0)
		requireRetry(t, first, retry)
		if generation(t, f) <= firstGeneration {
			t.Fatal("The retried scan did not arrive on a newer stream generation.")
		}
		waitForCommittedInventory(t, f, "2.0.0")
		f.probe.release(t, "3.0.0")
		waitForResult(t, retryLeg, retry, commandFailed)
		waitForStoredResult(t, f.db, retry, "failed")
		if slices.ContainsFunc(firstLeg.observe().results, func(result noticeResult) bool { return result.id == first.id }) {
			t.Fatal("The cut stream received a result for the held scan.")
		}
		f.probe.requireRuns(t, 3)

		f.probe.setVersion(t, "4.0.0")
		requestScan(t, f)
		later := waitForCommand(t, retryLeg, scanAction, 1)
		if later.id == first.id || later.attempt != 1 {
			t.Fatal("A later scan did not begin as a new command.")
		}
		waitForResult(t, retryLeg, later, commandDone)
		waitForStoredResult(t, f.db, later, "done")
		waitForInventory(t, f, "4.0.0")
		f.probe.requireRuns(t, 4)
	})
	for _, tc := range []struct {
		name string
		age  time.Duration
	}{
		{name: "completed scan"},
		{name: "replay at twenty-two hours and thirty minutes", age: 22*time.Hour + 30*time.Minute},
		{name: "no retry at the twenty-three hour cutoff", age: 23 * time.Hour},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testCompletedScanReplay(t, tc.age)
		})
	}
}

func testCompletedScanReplay(t *testing.T, age time.Duration) {
	t.Helper()
	var delayWelcome atomic.Bool
	f := newCommandFixture(t, relayControls{
		beforeReply: func(ctx context.Context, reply *linkv1.OpenResponse) bool {
			if reply.GetWelcome() == nil || !delayWelcome.Swap(false) {
				return true
			}
			delay := time.NewTimer(8 * time.Second)
			defer delay.Stop()
			select {
			case <-delay.C:
				return true
			case <-ctx.Done():
				return false
			}
		},
	})
	run, firstLeg := f.startApproved(t, 0)
	f.probe.setVersion(t, "2.0.0")
	f.dropNext.Store(true)
	requestScan(t, f)
	first := waitForCommand(t, firstLeg, scanAction, 0)
	waitForResult(t, firstLeg, first, commandDone)
	waitForInventory(t, f, "2.0.0")
	run.stop(t)
	receive(t, firstLeg.done, "the completed scan's original stream to close")
	requirePending(t, f.db, first)
	requireReceipt(t, f.opts, first, commandDone)
	f.probe.requireRuns(t, 2)

	if age != 0 {
		f.server.Stop(t)
		first.expiry = scalar[time.Time](t, f.db, `
UPDATE link_commands SET expires_at=clock_timestamp()-($2 * interval '1 second')
WHERE id=$1 RETURNING expires_at`, first.id, age.Seconds())
		withClosedClientDB(t, f.opts, true, func(db *bolt.DB) error {
			return db.Update(func(tx *bolt.Tx) error {
				receipt, err := commandReceipt(tx, first.id)
				if err != nil {
					return err
				}
				receipt.ExpiresAt = first.expiry
				data, err := json.Marshal(receipt)
				if err != nil {
					return err
				}
				return tx.Bucket([]byte("receipts")).Put([]byte(first.id), data)
			})
		})
		requireReceipt(t, f.opts, first, commandDone)
		f.server.Restart(t)
		f.page = f.server.Clients(t)
		delayWelcome.Store(age < 23*time.Hour)
	}
	f.probe.setVersion(t, "3.0.0")
	run = startClient(t, f.opts)
	retryLeg := receive(t, f.relay.legs, "the relaunched client's stream")
	waitForInventory(t, f, "3.0.0")
	if age >= 23*time.Hour {
		// Delivery of a later scan proves the live worker selected past the old row.
		requestScan(t, f)
		next := waitForCommand(t, retryLeg, scanAction, 0)
		if next.id == first.id || next.attempt != 1 {
			t.Fatal("The server retried a command at or past the twenty-three hour cutoff.")
		}
		waitForResult(t, retryLeg, next, commandDone)
		waitForStoredResult(t, f.db, next, "done")
		requirePending(t, f.db, first)
		f.probe.requireRuns(t, 4)
		return
	}
	retry := waitForCommand(t, retryLeg, scanAction, 0)
	requireRetry(t, first, retry)
	waitForResult(t, retryLeg, retry, commandDone)
	waitForStoredResult(t, f.db, retry, "done")
	run.stop(t)
	requireReceipt(t, f.opts, retry, commandDone)
	f.probe.requireRuns(t, 3)
}

func TestLinkRestartReplaysAfterDroppedResult(t *testing.T) {
	requirePostgres(t)
	f := newCommandFixture(t, relayControls{})
	firstRun, firstLeg := f.startApproved(t, 75)
	f.dropNext.Store(true)
	requestRestart(t, f)
	first := waitForCommand(t, firstLeg, restartAction, 0)
	waitForResult(t, firstLeg, first, commandDone)
	firstRun.expectExit(t, 75)
	receive(t, firstLeg.done, "the restarted client's first stream to close")
	requirePending(t, f.db, first)
	requireReceipt(t, f.opts, first, commandDone)

	relaunched := startClient(t, f.opts)
	retryLeg := receive(t, f.relay.legs, "the stream from the relaunched client")
	retry := waitForCommand(t, retryLeg, restartAction, 0)
	requireRetry(t, first, retry)
	waitForResult(t, retryLeg, retry, commandDone)
	waitForStoredResult(t, f.db, retry, "done")
	waitForClient(t, f.page, f.id, webv1.ClientState_CLIENT_STATE_ONLINE)
	// A subsequent command must still run on the replay stream.
	requestScan(t, f)
	scan := waitForCommand(t, retryLeg, scanAction, 0)
	waitForResult(t, retryLeg, scan, commandDone)
	waitForStoredResult(t, f.db, scan, "done")
	relaunched.stop(t)
	requireReceipt(t, f.opts, retry, commandDone)
}

func TestLinkSameStreamApproval(t *testing.T) {
	requirePostgres(t)
	for _, tc := range []struct {
		name string
		age  time.Duration
	}{
		{name: "unaged notice"},
		{name: "expired notice", age: 5 * time.Second},
		{name: "notice expired more than a day ago", age: 25 * time.Hour},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const machineName = "Approval fixture"
			reports := make(chan struct{})
			release := sync.OnceFunc(func() { close(reports) })
			defer release()
			f := newCommandFixture(t, relayControls{releaseReports: reports})
			if tc.age != 0 {
				ageInsertedCommand(t, f, stateAction, tc.age)
			}
			run := startClient(t, f.opts)
			leg := receive(t, f.relay.legs, "the waiting machine's stream")
			welcome := waitForWelcome(t, leg)
			if welcome.approval != linkv1.Approval_APPROVAL_WAITING {
				t.Fatal("The waiting client did not receive waiting standing in Welcome.")
			}
			originalGeneration := generation(t, f)
			if connect.CodeOf(f.scan(t.Context())) != connect.CodeFailedPrecondition ||
				connect.CodeOf(f.restart(t.Context())) != connect.CodeFailedPrecondition {
				t.Fatal("The signed-in action routes did not both refuse the waiting machine.")
			}
			if len(leg.observe().commands) != 0 ||
				scalar[int](t, f.db, "SELECT count(*) FROM link_commands WHERE machine_id=$1", f.id) != 0 {
				t.Fatal("A refused waiting-machine action queued or delivered a command.")
			}
			approvedAt := time.Now()
			approve(t, f.page, f.id, machineName)
			notice := waitForCommand(t, leg, stateAction, 0)
			if notice.attempt != 1 || notice.revision <= welcome.revision {
				t.Fatal("The original stream did not receive the first delivery of the new standing revision.")
			}
			if tc.age != 0 && (notice.expiry.After(approvedAt.Add(-tc.age+time.Second)) ||
				notice.expiry.Before(approvedAt.Add(-tc.age-time.Second))) {
				t.Fatal("The first state delivery did not carry the expiry placed by the fixture.")
			}
			row := waitForClient(t, f.page, f.id, webv1.ClientState_CLIENT_STATE_JOINING)
			if row.GetHarnessesScanned() || mark(t, f.db, f.id) != welcome.mark {
				t.Fatal("The joining observation did not precede the approved reports.")
			}
			release()
			waitForResult(t, leg, notice, commandDone)
			waitForStoredResult(t, f.db, notice, "done")
			waitForInventory(t, f, "1.0.0")
			if time.Since(approvedAt) > 30*time.Second || generation(t, f) != originalGeneration {
				t.Fatal("Approval did not reach online on the original generation within thirty seconds.")
			}
			baseline := leg.observe()
			if !slices.ContainsFunc(baseline.reports, func(report reportDelivery) bool { return report.facts }) ||
				!slices.ContainsFunc(baseline.reports, func(report reportDelivery) bool { return report.harnesses }) {
				t.Fatal("The original stream did not send both approved reports.")
			}

			repeated := injectCommand(t, leg, &linkv1.Command{
				Id: notice.id, Attempt: notice.attempt + 1, ExpiresAt: timestamppb.New(notice.expiry),
				Action: &linkv1.Command_SetState{SetState: &linkv1.SetState{State: &linkv1.MachineState{
					Approval: linkv1.Approval_APPROVAL_APPROVED, Revision: notice.revision, Name: machineName,
				}}},
			})
			waitForResult(t, leg, repeated, commandDone)
			// The requested report is a positive control after the duplicate notice.
			f.probe.setVersion(t, "2.0.0")
			requestScan(t, f)
			scan := waitForCommand(t, leg, scanAction, 0)
			waitForResult(t, leg, scan, commandDone)
			waitForStoredResult(t, f.db, scan, "done")
			waitForInventory(t, f, "2.0.0")
			run.stop(t)
			receive(t, leg.done, "the approval stream to close after the client stops")
			f.probe.requireRuns(t, 2)
			assertOneInventory(t, baseline, leg.observe())
		})
	}
}

func assertOneInventory(t *testing.T, before, after legObservation) {
	t.Helper()
	var facts, inventories int
	for _, event := range after.reports[len(before.reports):] {
		if event.facts {
			facts++
		}
		if event.harnesses {
			inventories++
		}
	}
	if facts != 0 || inventories != 1 {
		t.Fatalf("After the repeated revision, the original stream sent %d extra facts and %d inventory reports, want zero facts and one requested inventory.", facts, inventories)
	}
}

func ageInsertedCommand(t *testing.T, f *commandFixture, action string, age time.Duration) {
	t.Helper()
	execute(t, f.db, `
CREATE TABLE linktest_command_age (machine_id text, action text, age_seconds bigint);
CREATE FUNCTION linktest_age_command() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE age bigint;
BEGIN
    SELECT age_seconds INTO age FROM linktest_command_age
    WHERE machine_id = NEW.machine_id AND action = NEW.action;
    IF FOUND THEN
        NEW.expires_at := clock_timestamp() - age * interval '1 second';
        DELETE FROM linktest_command_age WHERE machine_id = NEW.machine_id AND action = NEW.action;
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER linktest_age_command BEFORE INSERT ON link_commands
FOR EACH ROW EXECUTE FUNCTION linktest_age_command();
`)
	execute(t, f.db, "INSERT INTO linktest_command_age VALUES ($1,$2,$3)", f.id, action, int64(age.Seconds()))
}

func TestLinkScanDoesNotDelayRestart(t *testing.T) {
	requirePostgres(t)
	t.Run("restart bypasses the queued scan", func(t *testing.T) {
		restartAtRelay := make(chan struct{})
		passRestart := make(chan struct{})
		release := sync.OnceFunc(func() { close(passRestart) })
		arrived := sync.OnceFunc(func() { close(restartAtRelay) })
		defer release()
		f := newCommandFixture(t, relayControls{
			beforeReply: func(ctx context.Context, reply *linkv1.OpenResponse) bool {
				if reply.GetCommand().GetRestart() == nil {
					return true
				}
				arrived()
				select {
				case <-passRestart:
					return true
				case <-ctx.Done():
					return false
				}
			},
		})
		run, leg := f.startApproved(t, 75)
		f.probe.arm(t)
		started := time.Now()
		requestScan(t, f)
		first := waitForCommand(t, leg, scanAction, 0)
		pid := f.probe.heldPID(t)
		requestScan(t, f)
		requestRestart(t, f)
		receive(t, restartAtRelay, "the restart to reach the relay ahead of the queued scan")
		requireUnclaimedScan(t, f, first)
		release()
		restart := waitForCommand(t, leg, restartAction, 0)
		waitForResult(t, leg, first, commandFailed)
		waitForResult(t, leg, restart, commandDone)
		// The held child has no release input. It must be reaped before its normal timeout.
		if !errors.Is(unix.Kill(pid, 0), unix.ESRCH) || time.Since(started) >= 3*time.Second {
			t.Fatal("The restart did not cancel the held probe within its normal scan budget.")
		}
		run.expectExit(t, 75)
		waitForStoredResult(t, f.db, restart, "done")
		requireReceipt(t, f.opts, first, commandFailed)
		requireReceipt(t, f.opts, restart, commandDone)
		f.probe.requireRuns(t, 2)
	})

	t.Run("the queued scan follows completion on the same stream", func(t *testing.T) {
		f := newCommandFixture(t, relayControls{})
		_, leg := f.startApproved(t, 0)
		f.probe.arm(t)
		requestScan(t, f)
		first := waitForCommand(t, leg, scanAction, 0)
		f.probe.heldPID(t)
		f.probe.setVersion(t, "3.0.0")
		requestScan(t, f)
		requireUnclaimedScan(t, f, first)
		f.probe.release(t, "2.0.0")
		waitForResult(t, leg, first, commandDone)
		waitForStoredResult(t, f.db, first, "done")
		second := waitForCommand(t, leg, scanAction, 1)
		if second.id == first.id || second.attempt != 1 {
			t.Fatal("The next delivery was not the queued scan's first attempt.")
		}
		waitForResult(t, leg, second, commandDone)
		waitForStoredResult(t, f.db, second, "done")
		waitForInventory(t, f, "3.0.0")
		f.probe.requireRuns(t, 3)
	})
}

func requireUnclaimedScan(t *testing.T, f *commandFixture, first commandDelivery) {
	t.Helper()
	if !scalar[bool](t, f.db, `
SELECT count(*) = 1 AND bool_and(sent_stream_id IS NULL AND NOT dispatched AND outcome IS NULL AND attempt=1)
FROM link_commands WHERE machine_id=$1 AND action='scan_harnesses' AND id<>$2`, f.id, first.id) {
		t.Fatal("The second scan was not queued and unclaimed while the first scan was held.")
	}
}

func TestLinkBusyScanExpiry(t *testing.T) {
	requirePostgres(t)
	f := newCommandFixture(t, relayControls{})
	run, leg := f.startApproved(t, 0)
	ageInsertedCommand(t, f, scanAction, 5*time.Second)
	requestScan(t, f)
	expired := waitForCommand(t, leg, scanAction, 0)
	if !expired.expiry.Before(time.Now()) {
		t.Fatal("The first scan was not expired when the client received it.")
	}
	waitForResult(t, leg, expired, commandExpired)
	waitForStoredResult(t, f.db, expired, "expired")
	f.probe.requireRuns(t, 1)

	readTx := observeClientTransactions(t, f.opts)
	waitFor(t, time.Second, "the initial reports to leave the client outbox", func() bool {
		_, queued := readTx()
		return queued == 0
	})
	f.probe.arm(t)
	requestScan(t, f)
	active := waitForCommand(t, leg, scanAction, 1)
	f.probe.heldPID(t)
	before, _ := readTx()
	replayed := injectCommand(t, leg, expiredScan(expired.id, expired.attempt+1, expired.expiry))
	waitForResult(t, leg, replayed, commandExpired)
	if after, _ := readTx(); after != before {
		t.Fatalf("The saved expiry advanced the client database transaction from %d to %d.", before, after)
	}

	// The real server defers the next scan while the active scan remains pending.
	// Deliver a distinct expired scan at the transport boundary to exercise the client.
	newExpiry := time.Now().Add(-5 * time.Second).UTC()
	fresh := injectCommand(t, leg, expiredScan("0195977c-0000-7000-8000-000000000019", 1, newExpiry))
	fresh.expiry = newExpiry
	waitForResult(t, leg, fresh, commandExpired)
	if after, _ := readTx(); after != before+1 {
		t.Fatalf("The new expiry advanced the client database transaction from %d to %d, want one commit.", before, after)
	}
	f.probe.requireRuns(t, 2)
	f.probe.release(t, "2.0.0")
	waitForResult(t, leg, active, commandDone)
	waitForStoredResult(t, f.db, active, "done")
	waitForInventory(t, f, "2.0.0")
	run.stop(t)
	requireReceipt(t, f.opts, expired, commandExpired)
	requireReceipt(t, f.opts, fresh, commandExpired)
}

func expiredScan(id string, attempt uint32, expiry time.Time) *linkv1.Command {
	return &linkv1.Command{
		Id: id, Attempt: attempt, ExpiresAt: timestamppb.New(expiry),
		Action: &linkv1.Command_ScanHarnesses{ScanHarnesses: &linkv1.ScanHarnesses{}},
	}
}
