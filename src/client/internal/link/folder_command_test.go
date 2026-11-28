package link

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math/bits"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"connectrpc.com/connect"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/enroll"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/harness"
	"github.com/qnlbnsl/lgtm-studio/src/client/internal/sysinfo"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
)

const (
	folderNotReadable = linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_READABLE
	folderNotGit      = linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_A_GIT_REPOSITORY
	folderUnusable    = linkv1.FolderRefusal_FOLDER_REFUSAL_PATH_NOT_USABLE
	folderReadBudget  = 5 * time.Second
)

func TestLookAtFolderLifecycle(t *testing.T) {
	t.Run("waiting_then_approved_and_retried_on_new_stream", testFolderRetryAcrossStreams)
	t.Run("second_expiry_check", testFolderExpiryBeforeWorker)
	t.Run("busy_folder_does_not_block_scan", testFolderSlotAlongsideScan)
	t.Run("timeout_offline_then_restart", testFolderTimeoutOffline)
	t.Run("timeout_with_full_outbox", testFolderTimeoutWhenFull)
	t.Run("two_retired_reads_bound_new_work", testFolderRetiredReadLimit)
	t.Run("uncertain_report_commit", testFolderUncertainCommit)
	t.Run("refusals_and_outgoing_filter", testFolderRefusalsAndFilter)
	t.Run("invalid_operation_id", testInvalidFolderOperation)
}

func TestLookAtFolderEventBudget(t *testing.T) {
	limits := readFolderLinkLimits(t)
	t.Run("numbered_exact_limit_with_full_send_window", func(t *testing.T) {
		testNumberedFolderEvent(t, limits, 0)
	})
	t.Run("numbered_one_byte_over", func(t *testing.T) {
		testNumberedFolderEvent(t, limits, 1)
	})
	t.Run("full_buffer_keeps_watermark", func(t *testing.T) {
		testFullFolderBuffer(t, false)
	})
	for _, extra := range []int{0, 1} {
		name := "command_at_limit"
		if extra == 1 {
			name = "command_one_byte_over"
		}
		t.Run(name, func(t *testing.T) { testFolderCommandSize(t, limits, extra) })
	}
	t.Run("outer_read_and_send_cap", func(t *testing.T) {
		testFolderOuterMessageSize(t, limits.messageBytes)
	})
}

type folderReadResult struct {
	facts   *linkv1.FolderFacts
	refusal linkv1.FolderRefusal
}

type folderReadCall struct {
	ctx        context.Context
	path, home string
	result     chan folderReadResult
	returned   chan struct{}
}

func withFolderInspector(t *testing.T, body func(*testing.T, <-chan *folderReadCall)) {
	t.Helper()
	synctest.Test(t, func(t *testing.T) {
		calls := make(chan *folderReadCall, 16)
		stop := make(chan struct{})
		defer close(stop)
		original := inspectFolder
		t.Cleanup(func() { inspectFolder = original })
		inspectFolder = func(ctx context.Context, path, home string) (*linkv1.FolderFacts, linkv1.FolderRefusal) {
			call := &folderReadCall{ctx: ctx, path: path, home: home,
				result: make(chan folderReadResult, 1), returned: make(chan struct{})}
			defer close(call.returned)
			select {
			case calls <- call:
			case <-stop:
				return nil, folderNotReadable
			}
			// Only the fixture releases this read. Its context can expire independently.
			select {
			case result := <-call.result:
				return result.facts, result.refusal
			case <-stop:
				return nil, folderNotReadable
			}
		}
		body(t, calls)
	})
}

func folderTestCommand(n int) *linkv1.Command {
	cmd := scanCommand(n, 1, time.Now().Add(time.Minute))
	op := scanCommand(n+100, 1, time.Now()).GetId()
	cmd.Action = &linkv1.Command_LookAtFolder{LookAtFolder: &linkv1.LookAtFolder{
		OperationId: op, Path: "~/work/repo",
	}}
	return cmd
}

func folderTestFacts() *linkv1.FolderFacts {
	return &linkv1.FolderFacts{
		ResolvedPath: "/work/repo", Name: "repo", MainBranch: "main",
		CheckedOutBranch: proto.String("topic"), FirstCommit: proto.String("object-id"),
		Remote: proto.String("https://git.example.test/team/repo"), Clean: false,
	}
}

func newFolderPeer(t *testing.T, first uint64, count int) *runPeerFixture {
	t.Helper()
	f := newRunPeerFixture(t, protocols[0])
	f.opts.ScanOptions.Home = "/work/home"
	runReportProbes.readFacts = func(string) (sysinfo.Facts, error) {
		return sysinfo.Facts{}, errReadFacts
	}
	runReportProbes.scan = func(ctx context.Context, _ harness.ScanOptions) []harness.Found {
		<-ctx.Done()
		return nil
	}
	store := mustCreate(t, f.opts.Folder, StoreOptions{})
	payload := fixtureFactsPayload(t, "retained")
	// Seed large buffers in one transaction. Run opens the resulting store itself.
	err := store.db.Update(func(tx *bolt.Tx) error {
		for seq := first; seq < first+uint64(count); seq++ {
			key := numberKey(seq)
			if err := tx.Bucket(eventsBucket).Put(key[:], payload); err != nil {
				return err
			}
		}
		next := numberKey(first + uint64(count))
		return tx.Bucket(metaBucket).Put(nextKey, next[:])
	})
	if err != nil || store.Close() != nil {
		t.Fatal("could not seed and close the folder command store")
	}
	return f
}

type folderDiskRecord struct {
	receipt          receipt
	report           *linkv1.Event
	reportNumber     uint64
	reportCommits    int
	finishedTooEarly bool
}

type folderDiskPosition struct {
	first, next uint64
	faulted     bool
	laterWrites int
}

type folderDiskProbe struct {
	mu sync.Mutex
	folderDiskPosition
	records map[string]*folderDiskRecord
}

func observeFolderDisk(t *testing.T, faultID string, cmds ...*linkv1.Command) *folderDiskProbe {
	t.Helper()
	p := &folderDiskProbe{records: make(map[string]*folderDiskRecord)}
	for _, cmd := range cmds {
		p.records[cmd.GetId()] = new(folderDiskRecord)
	}
	t.Cleanup(swapCommit(func(db *bolt.DB, write func(*bolt.Tx) error) error {
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.faulted {
			p.laterWrites++
		}
		if err := db.Update(write); err != nil {
			return err
		}
		if err := db.View(func(tx *bolt.Tx) error {
			next := binary.BigEndian.Uint64(tx.Bucket(metaBucket).Get(nextKey))
			events := tx.Bucket(eventsBucket).Cursor()
			p.first = next
			if key, _ := events.First(); key != nil {
				p.first = binary.BigEndian.Uint64(key)
			}
			// Observe the receipt before counting any report from this commit.
			for id, record := range p.records {
				var got receipt
				if data := tx.Bucket(receiptsBucket).Get([]byte(id)); data != nil {
					if err := json.Unmarshal(data, &got); err != nil {
						return errors.New("could not decode the observed command receipt")
					}
				}
				if got.State == receiptFinished && record.receipt.State != receiptFinished && record.reportCommits == 0 {
					record.finishedTooEarly = true
				}
				record.receipt = got
			}
			if key, data := events.Last(); key != nil && next != p.next && len(data) > 1 {
				event := new(linkv1.Event)
				if data[0] != reportVersion || proto.Unmarshal(data[1:], event) != nil {
					return errors.New("could not decode the observed outbox event")
				}
				if record := p.records[event.GetFolderSeen().GetCommandId()]; record != nil {
					record.report = event
					record.reportNumber = binary.BigEndian.Uint64(key)
					record.reportCommits++
				}
			}
			p.next = next
			return nil
		}); err != nil {
			return err
		}
		if record := p.records[faultID]; record != nil && record.reportCommits == 1 && !p.faulted {
			p.faulted = true
			return errors.New("fixture could not confirm the report commit")
		}
		return nil
	}))
	return p
}

func (p *folderDiskProbe) record(t *testing.T, cmd *linkv1.Command) folderDiskRecord {
	t.Helper()
	synctest.Wait()
	p.mu.Lock()
	defer p.mu.Unlock()
	return *p.records[cmd.GetId()]
}

func (p *folderDiskProbe) position(t *testing.T) folderDiskPosition {
	t.Helper()
	synctest.Wait()
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.folderDiskPosition
}

func requireFolderDiskReport(t *testing.T, p *folderDiskProbe, cmd *linkv1.Command, number uint64, want folderReadResult) {
	t.Helper()
	record := p.record(t, cmd)
	if record.finishedTooEarly {
		t.Fatal("the finished receipt was committed before its folder report")
	}
	if record.reportCommits != 1 || record.reportNumber != number {
		t.Fatalf("observed %d folder report commits at number %d, want one at %d", record.reportCommits, record.reportNumber, number)
	}
	if record.report.GetNumber() != 0 || !record.report.GetHappenedAt().IsValid() {
		t.Fatal("the folder report was stored with a number or without a valid timestamp")
	}
	requireFolderReport(t, record.report.GetFolderSeen(), cmd, want)
}

func requireFolderReport(t *testing.T, got *linkv1.FolderSeen, cmd *linkv1.Command, want folderReadResult) {
	t.Helper()
	expected := &linkv1.FolderSeen{OperationId: cmd.GetLookAtFolder().GetOperationId(), CommandId: cmd.GetId()}
	if want.facts != nil {
		expected.Result = &linkv1.FolderSeen_Facts{Facts: want.facts}
	} else {
		expected.Result = &linkv1.FolderSeen_Refusal{Refusal: want.refusal}
	}
	if !proto.Equal(got, expected) {
		t.Fatal("the folder report did not preserve the expected correlation and facts or fixed refusal")
	}
}

func requireFolderReceipt(t *testing.T, record folderDiskRecord, outcome linkv1.CommandOutcome, watermark uint64) {
	t.Helper()
	got := record.receipt
	if got.State != receiptFinished || got.LastEventNumber == nil ||
		*got.LastEventNumber != watermark || len(got.Result) != 1 || got.Result[0] != byte(outcome) {
		t.Fatal("the durable folder receipt did not retain the expected outcome and watermark")
	}
}

func requireFolderCommandResult(t *testing.T, got *linkv1.CommandResult, cmd *linkv1.Command, outcome linkv1.CommandOutcome, watermark uint64) {
	t.Helper()
	want := &linkv1.CommandResult{
		CommandId: cmd.GetId(), Attempt: cmd.GetAttempt(), Outcome: outcome, LastEventNumber: watermark,
	}
	if !proto.Equal(got, want) {
		t.Fatalf("result has matching command identifier %t, attempt %d, outcome %s and watermark %d. Expected attempt %d, outcome %s and watermark %d",
			got.GetCommandId() == want.CommandId, got.GetAttempt(), got.GetOutcome(), got.GetLastEventNumber(),
			want.Attempt, want.Outcome, want.LastEventNumber)
	}
}

func nextFolderResult(t *testing.T, leg *wireLeg, cmd *linkv1.Command) (*linkv1.CommandResult, *linkv1.Event) {
	t.Helper()
	var report *linkv1.Event
	for {
		frame := leg.next(t)
		if result := frame.GetCommandResult(); result.GetCommandId() == cmd.GetId() {
			return result, report
		}
		switch {
		case frame.GetHeartbeat() != nil:
			leg.send(t, ackHeartbeat())
		case frame.GetEvent().GetFolderSeen().GetCommandId() == cmd.GetId():
			if report != nil {
				t.Fatal("the peer received multiple reports for the same folder command")
			}
			report = frame.GetEvent()
		}
	}
}

func requireFolderWireReport(t *testing.T, leg *wireLeg, cmd *linkv1.Command, report *linkv1.Event, number uint64, want folderReadResult) *linkv1.Event {
	t.Helper()
	if report == nil {
		report = eventOnLeg(t, leg)
	}
	if report.GetNumber() != number || !report.GetHappenedAt().IsValid() {
		t.Fatal("the wire report lacked its durable number or a valid timestamp")
	}
	requireFolderReport(t, report.GetFolderSeen(), cmd, want)
	return report
}

func requireFolderStart(t *testing.T, calls <-chan *folderReadCall, cmd *linkv1.Command) *folderReadCall {
	t.Helper()
	call := receiveNow(t, calls, "the admitted folder command did not start an inspection")
	if call.path != cmd.GetLookAtFolder().GetPath() || call.home != "/work/home" {
		t.Fatal("the worker did not receive the exact typed path and the configured home")
	}
	return call
}

type folderStartObservation struct {
	ctx                    context.Context
	operationID, commandID string
}

func observeFolderStarts(f *runPeerFixture) <-chan folderStartObservation {
	starts := make(chan folderStartObservation, 16)
	f.opts.OnFolderInspectionStart = func(ctx context.Context, operationID, commandID string) {
		starts <- folderStartObservation{ctx, operationID, commandID}
	}
	return starts
}

func requireFolderStartObservation(t *testing.T, start folderStartObservation, cmd *linkv1.Command, call *folderReadCall) {
	t.Helper()
	if start.operationID != cmd.GetLookAtFolder().GetOperationId() || start.commandID != cmd.GetId() {
		t.Error("the inspection start observer received the wrong operation or command identifier")
	}
	if start.ctx == nil || start.ctx.Done() != call.ctx.Done() || start.ctx.Err() != nil {
		t.Error("the inspection start observer did not receive the live read context")
		return
	}
	gotDeadline, gotOK := start.ctx.Deadline()
	wantDeadline, wantOK := call.ctx.Deadline()
	if !gotOK || !wantOK || !gotDeadline.Equal(wantDeadline) {
		t.Error("the inspection start observer did not receive the worker deadline")
	}
}

func requireFolderStreamClosed(t *testing.T, leg *wireLeg) {
	t.Helper()
	receiveNow(t, leg.closed, "the client kept a stream open after rejecting an invalid or blocked folder command")
}

func watchFolderDataHandoffs(t *testing.T) <-chan struct{} {
	t.Helper()
	data := make(chan struct{}, 1)
	useSessionIO(t, func(_ context.Context, _ *session, wire runSessionIO) runSessionIO {
		send := wire.send
		wire.send = func(frame *linkv1.OpenRequest) error {
			if frame.GetCommandResult() != nil || frame.GetEvent() != nil {
				select {
				case data <- struct{}{}:
				default:
				}
			}
			return send(frame)
		}
		return wire
	})
	return data
}

func testFolderRetryAcrossStreams(t *testing.T) {
	withFolderInspector(t, func(t *testing.T, calls <-chan *folderReadCall) {
		f := newFolderPeer(t, 1, 1)
		starts := observeFolderStarts(f)
		observe := f.opts.OnFolderInspectionStart
		allowRead := make(chan struct{})
		f.opts.OnFolderInspectionStart = func(ctx context.Context, operationID, commandID string) {
			observe(ctx, operationID, commandID)
			select {
			case <-allowRead:
			case <-ctx.Done():
			}
		}
		cmd := folderTestCommand(1)
		p := observeFolderDisk(t, "", cmd)
		cancel, done := f.start(t)
		defer cancel()
		waiting := f.next(t, 0)
		f.welcome(t, waiting, runWelcome(waitingApproval, 1, 0))
		if got := eventOnLeg(t, waiting).GetNumber(); got != 1 {
			t.Fatal("the waiting stream did not replay the retained event")
		}
		waiting.send(t, commandFrame(cmd))
		requireFolderStreamClosed(t, waiting)
		requireEmpty(t, calls, "the waiting client inspected a folder")
		requireEmpty(t, starts, "the waiting command notified the inspection start observer")
		if record := p.record(t, cmd); record.receipt.State != "" || record.reportCommits != 0 {
			t.Fatal("the waiting client wrote a receipt or report")
		}

		approved := f.next(t, 0)
		f.welcome(t, approved, runWelcome(approvedApproval, 2, 0))
		if got := eventOnLeg(t, approved).GetNumber(); got != 1 {
			t.Fatal("the approved stream did not receive the retained event")
		}
		time.Sleep(reportInterval - time.Second)
		approved.send(t, commandFrame(cmd))
		start := receiveNow(t, starts, "the start observer did not run before the inspection")
		requireEmpty(t, calls, "the read began while the start observer was held")
		close(allowRead)
		call := requireFolderStart(t, calls, cmd)
		requireFolderStartObservation(t, start, cmd, call)
		approved.send(t, ackEvent(1))
		if p.position(t).first != 2 || p.record(t, cmd).receipt.State != receiptStarted {
			t.Fatal("the held read blocked the acknowledgement or did not have a started receipt")
		}
		time.Sleep(time.Second)
		if beat := approved.next(t).GetHeartbeat(); beat == nil || beat.GetCpuPercent() != 17 {
			t.Fatal("the held read blocked an approved heartbeat")
		}
		approved.send(t, ackHeartbeat())

		approved.refuse <- connect.NewError(connect.CodeUnavailable, errors.New("fixture disconnect"))
		replacement := f.next(t, 0)
		f.welcome(t, replacement, runWelcome(approvedApproval, 2, 1))
		latest := attemptCommand(cmd, 3)
		replacement.send(t, commandFrame(latest))
		replacement.send(t, commandFrame(attemptCommand(cmd, 2)))
		requireEmpty(t, calls, "another attempt started the held inspection a second time")
		requireEmpty(t, starts, "another attempt counted the held read a second time")
		if call.ctx.Err() != nil {
			t.Fatal("disconnect canceled the process-scoped inspection")
		}
		want := folderReadResult{facts: folderTestFacts()}
		call.result <- want
		got, report := nextFolderResult(t, replacement, latest)
		requireFolderDiskReport(t, p, latest, 2, want)
		requireFolderCommandResult(t, got, latest, commandDone, 2)
		requireFolderWireReport(t, replacement, latest, report, 2, want)
		requireFolderReceipt(t, p.record(t, latest), commandDone, 2)
		replacement.send(t, ackEvent(2))

		replay := attemptCommand(cmd, 4)
		replacement.send(t, commandFrame(replay))
		got, report = nextFolderResult(t, replacement, replay)
		requireFolderCommandResult(t, got, replay, commandDone, 2)
		requireEmpty(t, calls, "replaying the finished folder receipt started an inspection")
		requireEmpty(t, starts, "replaying the finished folder receipt counted as an inspection start")
		if report != nil || p.record(t, replay).reportCommits != 1 || p.position(t).next != 3 {
			t.Fatal("replaying the finished folder receipt appended a second report")
		}
		cancel()
		f.stop(t, done, nil)
	})
}

func testFolderExpiryBeforeWorker(t *testing.T) {
	withFolderInspector(t, func(t *testing.T, calls <-chan *folderReadCall) {
		// Move the authenticated clock to the expiry during the start-receipt write.
		folder, _ := newState(t)
		store := mustCreate(t, folder, StoreOptions{})
		f := newCommandFixture(t, store, nil, nil, nil)
		cmd := folderTestCommand(1)
		cmd.ExpiresAt = timestamppb.New(tNow.Add(time.Second))
		var earlyStarts atomic.Int32
		hooks := folderInspectionHooks{onStart: func(context.Context, string, string) { earlyStarts.Add(1) }}
		ctx := context.WithValue(t.Context(), folderInspectionHooksKey{}, hooks)
		restore := sync.OnceFunc(swapCommit(func(db *bolt.DB, write func(*bolt.Tx) error) error {
			err := db.Update(write)
			f.elapsed.Add(int64(time.Second))
			return err
		}))
		defer restore()
		if restart, err := f.handler.handle(ctx, cmd); restart || err != nil {
			t.Fatal("the folder command could not reach the worker expiry check")
		}
		requireEmpty(t, calls, "a folder command that expired during admission reached the worker")
		if got := earlyStarts.Load(); got != 0 {
			t.Fatalf("a folder command that expired during admission notified the observer %d times, want zero", got)
		}
		restore()

		// The authenticated stream must also decline work already stale on arrival.
		peer := newFolderPeer(t, 1, 0)
		starts := observeFolderStarts(peer)
		stale := folderTestCommand(2)
		stale.ExpiresAt = timestamppb.New(time.Now().Add(-time.Second))
		fresh := folderTestCommand(3)
		p := observeFolderDisk(t, "", stale, fresh)
		cancel, done := peer.start(t)
		defer cancel()
		leg := peer.next(t, 0)
		peer.welcome(t, leg, runWelcome(approvedApproval, 1, 0))
		leg.send(t, commandFrame(stale))
		got, report := nextFolderResult(t, leg, stale)
		requireEmpty(t, calls, "an expired folder command reached the worker")
		requireEmpty(t, starts, "an expired command counted as an inspection start")
		requireFolderCommandResult(t, got, stale, commandExpired, 0)
		if report != nil || p.record(t, stale).reportCommits != 0 {
			t.Fatal("an expired folder command produced a report")
		}
		leg.send(t, commandFrame(fresh))
		call := requireFolderStart(t, calls, fresh)
		start := receiveNow(t, starts, "the fresh command did not notify the inspection start observer")
		requireFolderStartObservation(t, start, fresh, call)
		requireEmpty(t, starts, "the fresh command counted as more than one inspection")
		want := folderReadResult{refusal: folderNotGit}
		call.result <- want
		got, report = nextFolderResult(t, leg, fresh)
		requireFolderDiskReport(t, p, fresh, 1, want)
		requireFolderCommandResult(t, got, fresh, commandDone, 1)
		requireFolderWireReport(t, leg, fresh, report, 1, want)
		cancel()
		peer.stop(t, done, nil)
	})
}

func testFolderSlotAlongsideScan(t *testing.T) {
	withFolderInspector(t, func(t *testing.T, calls <-chan *folderReadCall) {
		f := newFolderPeer(t, 1, 0)
		// Let the startup scan block. The concurrent command scan can complete.
		var scans atomic.Int32
		startup := make(chan struct{})
		runReportProbes.scan = func(ctx context.Context, _ harness.ScanOptions) []harness.Found {
			if scans.Add(1) == 1 {
				close(startup)
				<-ctx.Done()
			}
			return nil
		}
		first, busy := folderTestCommand(1), folderTestCommand(2)
		p := observeFolderDisk(t, "", first, busy)
		cancel, done := f.start(t)
		defer cancel()
		leg := f.next(t, 0)
		f.welcome(t, leg, runWelcome(approvedApproval, 1, 0))
		receiveNow(t, startup, "the peer did not start its startup inventory scan")
		leg.send(t, commandFrame(first))
		call := requireFolderStart(t, calls, first)
		scan := scanCommand(50, 1, time.Now().Add(time.Minute))
		leg.send(t, commandFrame(scan))
		result, _ := nextFolderResult(t, leg, scan)
		requireFolderCommandResult(t, result, scan, commandDone, 1)
		leg.send(t, commandFrame(busy))
		requireFolderStreamClosed(t, leg)
		requireEmpty(t, calls, "a different folder command used the occupied slot")
		if record := p.record(t, busy); record.receipt.State != "" || record.reportCommits != 0 {
			t.Fatal("a different folder command wrote to the store while the slot was busy")
		}
		if call.ctx.Err() != nil {
			t.Fatal("the busy retry canceled the original inspection")
		}
		cancel()
		f.stop(t, done, nil)
		receiveNow(t, call.ctx.Done(), "process cancellation did not cancel the folder read")
	})
}

func testFolderTimeoutOffline(t *testing.T) {
	withFolderInspector(t, func(t *testing.T, calls <-chan *folderReadCall) {
		f := newFolderPeer(t, 1, 0)
		expiring, running := folderTestCommand(1), folderTestCommand(2)
		restart := scanCommand(3, 1, time.Now().Add(time.Minute))
		restart.Action = &linkv1.Command_Restart{Restart: &linkv1.Restart{}}
		p := observeFolderDisk(t, "", expiring, running, restart)
		cancel, done := f.start(t)
		defer cancel()
		leg := f.next(t, 0)
		f.welcome(t, leg, runWelcome(approvedApproval, 1, 0))
		leg.send(t, commandFrame(expiring))
		retired := requireFolderStart(t, calls, expiring)
		leg.refuse <- connect.NewError(connect.CodeUnavailable, errors.New("fixture disconnect"))
		replacement := f.next(t, 0)

		time.Sleep(folderReadBudget - time.Nanosecond)
		if record := p.record(t, expiring); record.receipt.State != receiptStarted || record.reportCommits != 0 {
			t.Fatal("the folder deadline completed before the five-second budget")
		}
		time.Sleep(time.Nanosecond)
		timedOut := folderReadResult{refusal: folderNotReadable}
		requireFolderDiskReport(t, p, expiring, 1, timedOut)
		requireFolderReceipt(t, p.record(t, expiring), commandDone, 1)
		receiveNow(t, retired.ctx.Done(), "the five-second budget did not cancel the read")
		requireEmpty(t, retired.returned, "the retired read returned without being released")

		f.welcome(t, replacement, runWelcome(approvedApproval, 1, 0))
		retry := attemptCommand(expiring, 2)
		replacement.send(t, commandFrame(retry))
		got, report := nextFolderResult(t, replacement, retry)
		requireFolderCommandResult(t, got, retry, commandDone, 1)
		requireFolderWireReport(t, replacement, retry, report, 1, timedOut)
		requireEmpty(t, calls, "the timed-out receipt started another inspection")

		replacement.send(t, commandFrame(running))
		active := requireFolderStart(t, calls, running)
		retired.result <- folderReadResult{facts: folderTestFacts()}
		receiveNow(t, retired.returned, "the test could not release the retired read")
		if p.record(t, expiring).reportCommits != 1 || p.position(t).next != 2 {
			t.Fatal("a timed-out read appended a second report after returning late")
		}

		replacement.send(t, commandFrame(restart))
		got, _ = nextFolderResult(t, replacement, restart)
		requireFolderCommandResult(t, got, restart, commandDone, 1)
		requireFolderReceipt(t, p.record(t, restart), commandDone, 1)
		receiveNow(t, active.ctx.Done(), "restart did not cancel the active read")
		requireEmpty(t, active.returned, "the fixture released the active read before restart")
		replacement.refuse <- nil
		result := receiveWithin(t, done, 3*time.Second, "restart waited for the stuck filesystem call")
		if result.outcome != RunRestart || result.err != nil {
			t.Fatal("the client did not exit with the normal restart outcome")
		}
		active.result <- folderReadResult{facts: folderTestFacts()}
		receiveNow(t, active.returned, "the test could not release the read canceled by restart")
		if p.record(t, running).reportCommits != 0 || p.position(t).next != 2 {
			t.Fatal("the read canceled by restart appended a folder report")
		}
	})
}

func testFolderTimeoutWhenFull(t *testing.T) { testFullFolderBuffer(t, true) }

func testFullFolderBuffer(t *testing.T, timeout bool) {
	withFolderInspector(t, func(t *testing.T, calls <-chan *folderReadCall) {
		opts, err := withStoreDefaults(StoreOptions{})
		if err != nil {
			t.Fatal("could not read the outbox capacity")
		}
		capacity := uint64(opts.MaxEvents)
		f := newFolderPeer(t, 1, opts.MaxEvents)
		var starts <-chan folderStartObservation
		var appends chan uint64
		if timeout {
			starts = observeFolderStarts(f)
			appends = make(chan uint64, 1)
			f.opts.AfterFolderReport = func(_ context.Context, number uint64) error {
				appends <- number
				return nil
			}
		}
		cmd := folderTestCommand(1)
		p := observeFolderDisk(t, "", cmd)
		cancel, done := f.start(t)
		defer cancel()
		leg := f.next(t, 0)
		f.welcome(t, leg, runWelcome(approvedApproval, 1, 0))
		leg.send(t, commandFrame(cmd))
		call := requireFolderStart(t, calls, cmd)
		if timeout {
			start := receiveNow(t, starts, "a full outbox suppressed the inspection start observation")
			requireFolderStartObservation(t, start, cmd, call)
			time.Sleep(folderReadBudget)
		} else {
			call.result <- folderReadResult{facts: folderTestFacts()}
		}
		got, report := nextFolderResult(t, leg, cmd)
		requireFolderCommandResult(t, got, cmd, commandFailed, capacity)
		requireFolderReceipt(t, p.record(t, cmd), commandFailed, capacity)
		if report != nil || p.record(t, cmd).reportCommits != 0 || p.position(t).next != capacity+1 {
			t.Fatal("a full outbox consumed a number or held a folder report")
		}
		if timeout {
			requireEmpty(t, starts, "the full-outbox command counted as more than one inspection")
			requireEmpty(t, appends, "the append callback ran without a durable folder report")
		}
		leg.send(t, ackEvent(1))
		if position := p.position(t); position.first != 2 || position.next != capacity+1 {
			t.Fatal("freeing capacity flushed a failed folder command as a pending snapshot")
		}
		cancel()
		f.stop(t, done, nil)
	})
}

func testFolderRetiredReadLimit(t *testing.T) {
	withFolderInspector(t, func(t *testing.T, calls <-chan *folderReadCall) {
		f := newFolderPeer(t, 1, 0)
		first, second, third, fourth := folderTestCommand(1), folderTestCommand(2), folderTestCommand(3), folderTestCommand(4)
		p := observeFolderDisk(t, "", first, second, third, fourth)
		cancel, done := f.start(t)
		defer cancel()
		leg := f.next(t, 0)
		f.welcome(t, leg, runWelcome(approvedApproval, 1, 0))
		var retired []*folderReadCall
		want := folderReadResult{refusal: folderNotReadable}
		for i, cmd := range []*linkv1.Command{first, second, third} {
			leg.send(t, commandFrame(cmd))
			if i < 2 {
				retired = append(retired, requireFolderStart(t, calls, cmd))
				time.Sleep(folderReadBudget)
			} else {
				requireEmpty(t, calls, "a third unresolved read reached the worker")
			}
			got, report := nextFolderResult(t, leg, cmd)
			number := uint64(i + 1)
			requireFolderDiskReport(t, p, cmd, number, want)
			requireFolderCommandResult(t, got, cmd, commandDone, number)
			requireFolderWireReport(t, leg, cmd, report, number, want)
			leg.send(t, ackEvent(number))
		}
		retired[0].result <- folderReadResult{facts: folderTestFacts()}
		receiveNow(t, retired[0].returned, "the fixture could not release one retired read")
		requireEmpty(t, retired[1].returned, "the other retired read returned early")
		leg.send(t, commandFrame(fourth))
		requireFolderStart(t, calls, fourth)
		if p.record(t, first).reportCommits != 1 || p.position(t).next != 4 {
			t.Fatal("a late retired read affected the durable event sequence")
		}
		cancel()
		f.stop(t, done, nil)
	})
}

func testFolderUncertainCommit(t *testing.T) {
	withFolderInspector(t, func(t *testing.T, calls <-chan *folderReadCall) {
		f := newFolderPeer(t, 1, 0)
		cmd := folderTestCommand(1)
		p := observeFolderDisk(t, cmd.GetId(), cmd)
		handoffs := watchFolderDataHandoffs(t)
		cancel, done := f.start(t)
		defer cancel()
		leg := f.next(t, 0)
		f.welcome(t, leg, runWelcome(approvedApproval, 1, 0))
		leg.send(t, commandFrame(cmd))
		call := requireFolderStart(t, calls, cmd)
		call.result <- folderReadResult{facts: folderTestFacts()}
		f.stop(t, done, ErrStoreFailed)
		record := p.record(t, cmd)
		position := p.position(t)
		if !position.faulted || position.laterWrites != 0 || record.reportCommits != 1 || position.next != 2 {
			t.Fatal("an uncertain folder commit did not stop further store writes")
		}
		if record.receipt.State != receiptStarted || record.receipt.LastEventNumber != nil {
			t.Fatal("an uncertain folder commit finalized the command receipt")
		}
		requireEmpty(t, handoffs, "an uncertain commit produced a result or handed off a report")
		requireEmpty(t, f.peer.opens, "the client redialed after an uncertain folder commit")
	})
}

func TestAfterFolderReport(t *testing.T) {
	withFolderInspector(t, func(t *testing.T, calls <-chan *folderReadCall) {
		const oldInventory = "1.0.0"
		const newInventory = "2.0.0"
		f := newFolderPeer(t, 1, 0)
		starts := observeFolderStarts(f)
		inventoryStarted, allowInventory := make(chan struct{}), make(chan struct{})
		var scans atomic.Int32
		runReportProbes.scan = func(ctx context.Context, _ harness.ScanOptions) []harness.Found {
			version := newInventory
			if scans.Add(1) == 1 {
				close(inventoryStarted)
				select {
				case <-allowInventory:
				case <-ctx.Done():
					return nil
				}
				version = oldInventory
			}
			return []harness.Found{{Harness: harness.Claude, Installed: true, Version: version}}
		}
		appends := make(chan uint64, 2)
		interrupt := make(chan struct{})
		f.opts.AfterFolderReport = func(ctx context.Context, number uint64) error {
			appends <- number
			select {
			case <-interrupt:
				return errors.New("fixture interruption after the durable report")
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		cmd := folderTestCommand(1)
		probe := observeFolderDisk(t, "", cmd)
		handoffs := watchFolderDataHandoffs(t)
		cancel, done := f.start(t)
		defer cancel()
		leg := f.next(t, 0)
		f.welcome(t, leg, runWelcome(approvedApproval, 1, 0))
		receiveNow(t, inventoryStarted, "the startup inventory probe did not arm")
		leg.send(t, commandFrame(cmd))
		call := requireFolderStart(t, calls, cmd)
		want := folderReadResult{facts: folderTestFacts()}
		call.result <- want
		reportNumber := receiveNow(t, appends, "the append callback did not observe the folder report")
		if reportNumber != 1 {
			t.Fatal("the append callback did not receive the durable folder report number")
		}
		requireFolderDiskReport(t, probe, cmd, reportNumber, want)
		if record := probe.record(t, cmd); record.receipt.State != receiptStarted || record.receipt.LastEventNumber != nil {
			t.Fatal("the receipt was no longer an unsealed start during the append callback")
		}
		requireEmpty(t, handoffs, "a folder report or command result was handed off while the append callback was held")

		close(allowInventory)
		position := probe.position(t)
		bufferedInventory := false
		for _, entry := range f.diagnostics(t) {
			bufferedInventory = bufferedInventory || entry.Msg == "link_event_buffered" && entry.Sequence == reportNumber+1
		}
		if !bufferedInventory || position.next != reportNumber+2 {
			t.Fatal("startup inventory could not append while the folder callback was held")
		}
		requireEmpty(t, handoffs, "a report or result was handed off before the interruption")
		close(interrupt)
		stopped := receiveNow(t, done, "the append callback error did not stop the client run")
		if stopped.outcome == RunRestart {
			t.Error("the append callback error requested a restart")
		}
		if stopped.err != nil {
			f.checkOutput(t, stopped.err.Error())
		}
		requireEmpty(t, handoffs, "the interrupted run handed off a report or result")
		requireEmpty(t, appends, "the interrupted run invoked the append callback twice")
		requireEmpty(t, f.peer.opens, "the append callback error caused a reconnect")
		start := receiveNow(t, starts, "the interrupted run missed its inspection start observation")
		if start.operationID != cmd.GetLookAtFolder().GetOperationId() || start.commandID != cmd.GetId() {
			t.Error("the interrupted run reported the wrong inspection start identifiers")
		}
		requireEmpty(t, starts, "the interrupted run counted more than one inspection")

		watermark := readInterruptedFolderWatermark(t, f.file(), cmd, reportNumber, oldInventory)
		f.opts.AfterFolderReport = nil
		var mark uint64
		for _, attempt := range []uint32{3, 5} {
			retry := attemptCommand(cmd, attempt)
			// Stop each process before the next one opens the same durable store.
			func() {
				cancel, done := f.start(t)
				defer cancel()
				leg := f.next(t, 0)
				f.welcome(t, leg, runWelcome(approvedApproval, 1, mark))
				if mark == 0 {
					requireFolderWireReport(t, leg, retry, nil, reportNumber, want)
					leg.send(t, ackEvent(reportNumber))
					old := eventOnLeg(t, leg)
					if old.GetNumber() != watermark || !folderInventoryHasVersion(old, oldInventory) {
						t.Fatal("the reopened run did not replay the old inventory at the dead process watermark")
					}
					leg.send(t, ackEvent(watermark))
					mark = watermark
				}
				fresh := eventOnLeg(t, leg)
				if fresh.GetNumber() != mark+1 || !folderInventoryHasVersion(fresh, newInventory) {
					t.Fatal("the reopened run did not append fresh inventory beyond the dead process watermark")
				}
				mark = fresh.GetNumber()
				leg.send(t, ackEvent(mark))
				leg.send(t, commandFrame(retry))
				result, report := nextFolderResult(t, leg, retry)
				requireFolderCommandResult(t, result, retry, commandInterrupted, watermark)
				if report != nil || probe.record(t, cmd).reportCommits != 1 {
					t.Fatal("recovering an interrupted command produced another folder report")
				}
				requireEmpty(t, calls, "recovering an interrupted command started another inspection")
				requireEmpty(t, starts, "recovering an interrupted command counted as another inspection start")
				cancel()
				f.stop(t, done, nil)
			}()
		}
	})
}

func readInterruptedFolderWatermark(t *testing.T, path string, cmd *linkv1.Command, reportNumber uint64, version string) uint64 {
	t.Helper()
	db, err := bolt.Open(path, 0o600, &bolt.Options{ReadOnly: true, Timeout: time.Second})
	if err != nil {
		t.Fatal("the stopped run did not release its database")
	}
	defer db.Close()
	var watermark uint64
	if err := db.View(func(tx *bolt.Tx) error {
		var saved receipt
		if err := json.Unmarshal(tx.Bucket(receiptsBucket).Get([]byte(cmd.GetId())), &saved); err != nil ||
			saved.Version != 2 || saved.State != receiptStarted || saved.LastEventNumber != nil || len(saved.Result) != 0 {
			return errors.New("the closed store did not contain an unsealed started receipt")
		}
		next := tx.Bucket(metaBucket).Get(nextKey)
		if len(next) != 8 || binary.BigEndian.Uint64(next) != reportNumber+2 {
			return errors.New("the closed store counter did not distinguish the folder report from the last durable event")
		}
		watermark = binary.BigEndian.Uint64(next) - 1
		key := numberKey(watermark)
		inventory, err := decodeReportCandidate(tx.Bucket(eventsBucket).Get(key[:]))
		if err != nil || !folderInventoryHasVersion(inventory, version) {
			return errors.New("the closed store lacked the expected inventory at the dead process watermark")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return watermark
}

func folderInventoryHasVersion(event *linkv1.Event, version string) bool {
	for _, item := range event.GetHarnessesFound().GetHarnesses() {
		if item.GetName() == string(harness.Claude) && item.GetVersion() == version {
			return true
		}
	}
	return false
}

func testFolderRefusalsAndFilter(t *testing.T) {
	for _, tc := range []struct {
		name   string
		secret string
		input  folderReadResult
		want   folderReadResult
	}{
		{
			name: "fixed_refusal", input: folderReadResult{refusal: folderNotGit},
			want: folderReadResult{refusal: folderNotGit},
		},
		{
			name: "unsafe_branch", secret: "branch-fixture-credential",
			input: folderReadResult{facts: folderTestFacts()}, want: folderReadResult{refusal: folderUnusable},
		},
		{
			name: "unsafe_origin", secret: "origin-fixture-credential",
			input: folderReadResult{facts: folderTestFacts()}, want: folderReadResult{facts: folderTestFacts()},
		},
		{
			name: "unsafe_typed_path", secret: "work/repo",
			want: folderReadResult{refusal: folderUnusable},
		},
		{
			name: "unsafe_expanded_path", secret: "/work/home/work",
			want: folderReadResult{refusal: folderUnusable},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withFolderInspector(t, func(t *testing.T, calls <-chan *folderReadCall) {
				f := newFolderPeer(t, 1, 0)
				starts := observeFolderStarts(f)
				f.opts.Filter = NewOutgoingFilter()
				if tc.input.facts == nil {
					f.opts.Filter.AddSecrets(tc.secret)
				}
				cmd := folderTestCommand(1)
				p := observeFolderDisk(t, "", cmd)
				cancel, done := f.start(t)
				defer cancel()
				leg := f.next(t, 0)
				f.welcome(t, leg, runWelcome(approvedApproval, 1, 0))
				leg.send(t, commandFrame(cmd))
				if tc.input == (folderReadResult{}) {
					requireEmpty(t, calls, "an unsafe typed or expanded path reached the worker")
				} else {
					call := requireFolderStart(t, calls, cmd)
					// Register the value after admission so only the final append gate knows it.
					f.opts.Filter.AddSecrets(tc.secret)
					switch tc.name {
					case "unsafe_branch":
						tc.input.facts.MainBranch = tc.secret
					case "unsafe_origin":
						tc.input.facts.Remote = proto.String("https://git.example.test/" + tc.secret)
						tc.want.facts.Remote = nil
						tc.want.facts.RemoteOmitted = true
					}
					call.result <- tc.input
				}
				got, report := nextFolderResult(t, leg, cmd)
				if tc.input == (folderReadResult{}) {
					requireEmpty(t, starts, "an unsafe typed or expanded path notified the inspection start observer")
				}
				requireFolderDiskReport(t, p, cmd, 1, tc.want)
				requireFolderCommandResult(t, got, cmd, commandDone, 1)
				requireFolderWireReport(t, leg, cmd, report, 1, tc.want)
				cancel()
				f.stop(t, done, nil)
				checkFolderLogs(t, f, tc.secret)
			})
		})
	}
}

func testInvalidFolderOperation(t *testing.T) {
	for _, mode := range []string{"invalid", "known_unsafe", "operation_becomes_unsafe", "command_becomes_unsafe"} {
		t.Run(mode, func(t *testing.T) {
			withFolderInspector(t, func(t *testing.T, calls <-chan *folderReadCall) {
				f := newFolderPeer(t, 1, 0)
				f.opts.Filter = NewOutgoingFilter()
				cmd := folderTestCommand(1)
				secret := cmd.GetLookAtFolder().GetOperationId()
				switch mode {
				case "invalid":
					cmd.GetLookAtFolder().OperationId = "invalid-operation"
				case "known_unsafe":
					f.opts.Filter.AddSecrets(secret)
				case "command_becomes_unsafe":
					secret = cmd.GetId()
				}
				p := observeFolderDisk(t, "", cmd)
				handoffs := watchFolderDataHandoffs(t)
				cancel, done := f.start(t)
				defer cancel()
				leg := f.next(t, 0)
				f.welcome(t, leg, runWelcome(approvedApproval, 1, 0))
				leg.send(t, commandFrame(cmd))
				afterStart := strings.HasSuffix(mode, "_becomes_unsafe")
				if afterStart {
					call := requireFolderStart(t, calls, cmd)
					f.opts.Filter.AddSecrets(secret)
					call.result <- folderReadResult{facts: folderTestFacts()}
				} else {
					requireEmpty(t, calls, "an invalid or unsafe operation entered the folder worker")
				}
				requireFolderStreamClosed(t, leg)
				requireEmpty(t, handoffs, "the invalid-command stream disclosed a report or result")
				record := p.record(t, cmd)
				if record.reportCommits != 0 || record.receipt.State == receiptFinished ||
					(!afterStart && record.receipt.State != "") {
					t.Fatal("unsafe correlation reached a durable report or an unexpected receipt")
				}
				checkFolderLogs(t, f, secret)
				foundInvalid := false
				for _, diagnostic := range f.diagnostics(t) {
					foundInvalid = foundInvalid || diagnostic.Code == "invalid_frame"
				}
				if !foundInvalid {
					t.Fatal("the rejected folder identifiers did not produce the fixed invalid-frame diagnostic")
				}
				cancel()
				f.stop(t, done, nil)
			})
		})
	}
}

func checkFolderLogs(t *testing.T, f *runPeerFixture, extra string) {
	t.Helper()
	f.log.mu.Lock()
	log := f.log.buffer.String()
	f.log.mu.Unlock()
	f.checkOutput(t, log)
	checkNoFixtureSecrets(t, log, extra)
}

type folderLinkLimits struct {
	messageBytes int
	eventBytes   int
	window       uint64
}

func readFolderLinkLimits(t *testing.T) folderLinkLimits {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "testdata", "link-v2-limits.json"))
	if err != nil {
		t.Fatal("could not read the reviewed link limit fixture")
	}
	var wire struct {
		MessageBytes int    `json:"link_message_bytes"`
		EventBytes   int    `json:"command_event_bytes"`
		Window       uint64 `json:"unacked_events"`
	}
	if err := json.Unmarshal(data, &wire); err != nil || wire.MessageBytes <= wire.EventBytes || wire.EventBytes <= 0 || wire.Window < 2 {
		t.Fatal("the reviewed link limit fixture has invalid byte or window limits")
	}
	return folderLinkLimits{messageBytes: wire.MessageBytes, eventBytes: wire.EventBytes, window: wire.Window}
}

func folderFactsAtSize(t *testing.T, cmd *linkv1.Command, number uint64, size int) *linkv1.FolderFacts {
	t.Helper()
	facts := folderTestFacts()
	event := &linkv1.Event{
		Number: number, HappenedAt: timestamppb.Now(),
		Report: &linkv1.Event_FolderSeen{FolderSeen: &linkv1.FolderSeen{
			OperationId: cmd.GetLookAtFolder().GetOperationId(), CommandId: cmd.GetId(),
			Result: &linkv1.FolderSeen_Facts{Facts: facts},
		}},
	}
	facts.MainBranch = strings.Repeat("b", size-proto.Size(event))
	for delta := size - proto.Size(event); delta != 0; delta = size - proto.Size(event) {
		facts.MainBranch = strings.Repeat("b", len(facts.MainBranch)+delta)
	}
	encoded, err := proto.Marshal(event)
	if err != nil || len(encoded) != size || bits.Len64(number) <= 7 {
		t.Fatal("the fixture did not encode the exact numbered event size with a multi-byte number")
	}
	return facts
}

func testNumberedFolderEvent(t *testing.T, limits folderLinkLimits, extra int) {
	withFolderInspector(t, func(t *testing.T, calls <-chan *folderReadCall) {
		const candidate = uint64(128)
		first := candidate - limits.window - 1
		f := newFolderPeer(t, first, int(limits.window+1))
		assertHandoffs := watchRevisionTwoEventHandoffs(t)
		cmd := folderTestCommand(1)
		p := observeFolderDisk(t, "", cmd)
		cancel, done := f.start(t)
		defer cancel()
		leg := f.next(t, 0)
		f.welcome(t, leg, runWelcome(approvedApproval, 1, first-1))
		for i := range limits.window {
			if got := eventOnLeg(t, leg).GetNumber(); got != first+i {
				t.Fatal("the filled send window did not hand off the retained events in order")
			}
		}
		assertHandoffs(t, limits.window)
		leg.send(t, ackEvent(first))
		if got := eventOnLeg(t, leg).GetNumber(); got != candidate-1 {
			t.Fatal("one acknowledgement did not release the next retained event")
		}
		assertHandoffs(t, limits.window+1)

		input := folderReadResult{facts: folderFactsAtSize(t, cmd, candidate, limits.eventBytes+extra)}
		leg.send(t, commandFrame(cmd))
		call := requireFolderStart(t, calls, cmd)
		call.result <- input
		got, report := nextFolderResult(t, leg, cmd)
		if report != nil {
			t.Fatal("the folder event exceeded the full send window")
		}
		want := input
		if extra != 0 {
			want = folderReadResult{refusal: folderUnusable}
			if p.record(t, cmd).report.GetFolderSeen().GetRefusal() != folderUnusable {
				t.Fatal("facts one byte over the numbered event limit did not become a fixed refusal")
			}
		}
		requireFolderDiskReport(t, p, cmd, candidate, want)
		requireFolderCommandResult(t, got, cmd, commandDone, candidate)
		requireFolderReceipt(t, p.record(t, cmd), commandDone, candidate)
		if p.position(t).next != candidate+1 {
			t.Fatal("numbered budget admission consumed more than one event number")
		}
		assertHandoffs(t, limits.window+1)
		leg.send(t, ackEvent(first+1))
		wire := requireFolderWireReport(t, leg, cmd, nil, candidate, want)
		if extra == 0 {
			body, err := proto.Marshal(wire)
			if err != nil || len(body) != limits.eventBytes {
				t.Fatal("the exact-limit numbered folder event did not reach the peer at the contracted size")
			}
		}
		assertHandoffs(t, limits.window+2)
		cancel()
		f.stop(t, done, nil)
	})
}

func padFolderMessage(t *testing.T, message proto.Message, size int) {
	t.Helper()
	// An unknown field exercises the complete encoding while keeping the typed path valid.
	const field = 100
	base := proto.Size(message)
	for n := size - base; n >= 0; n-- {
		if base+protowire.SizeTag(field)+protowire.SizeBytes(n) != size {
			continue
		}
		padding := protowire.AppendTag(nil, field, protowire.BytesType)
		padding = protowire.AppendBytes(padding, make([]byte, n))
		message.ProtoReflect().SetUnknown(padding)
		body, err := proto.Marshal(message)
		if err != nil || len(body) != size {
			t.Fatal("the padded fixture did not encode to the requested size")
		}
		return
	}
	t.Fatal("could not produce the requested encoded size")
}

func testFolderCommandSize(t *testing.T, limits folderLinkLimits, extra int) {
	withFolderInspector(t, func(t *testing.T, calls <-chan *folderReadCall) {
		f := newFolderPeer(t, 1, 0)
		cmd := folderTestCommand(1)
		padFolderMessage(t, cmd, limits.eventBytes+extra)
		frame := commandFrame(cmd)
		if proto.Size(frame) > limits.messageBytes {
			t.Fatal("the command boundary fixture exceeds the outer message cap")
		}
		p := observeFolderDisk(t, "", cmd)
		cancel, done := f.start(t)
		defer cancel()
		leg := f.next(t, 0)
		f.welcome(t, leg, runWelcome(approvedApproval, 1, 0))
		leg.send(t, frame)
		if extra == 0 {
			call := requireFolderStart(t, calls, cmd)
			want := folderReadResult{refusal: folderNotGit}
			call.result <- want
			got, report := nextFolderResult(t, leg, cmd)
			requireFolderDiskReport(t, p, cmd, 1, want)
			requireFolderCommandResult(t, got, cmd, commandDone, 1)
			requireFolderWireReport(t, leg, cmd, report, 1, want)
		} else {
			requireEmpty(t, calls, "an oversized command reached the folder worker")
			if record := p.record(t, cmd); record.receipt.State != "" || record.reportCommits != 0 {
				t.Fatal("an oversized command crossed the receipt or report write boundary")
			}
			requireFolderStreamClosed(t, leg)
		}
		cancel()
		f.stop(t, done, nil)
	})
}

func testFolderOuterMessageSize(t *testing.T, limit int) {
	exactIn, longIn := ackHeartbeat(), ackHeartbeat()
	padFolderMessage(t, exactIn, limit)
	padFolderMessage(t, longIn, limit+1)
	peerResult := make(chan error, 1)
	handler := connect.NewBidiStreamHandler(linkv1connect.LinkServiceOpenProcedure,
		func(_ context.Context, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
			first, err := stream.Receive()
			if err != nil || first.GetHeartbeat() == nil || proto.Size(first) != limit {
				peerResult <- errors.New("the peer did not receive the exact-size request")
				return nil
			}
			peerResult <- nil
			if err := stream.Send(exactIn); err != nil {
				return err
			}
			return stream.Send(longIn)
		})
	server := httptest.NewUnstartedServer(handler)
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	pin := sha256.Sum256(server.Certificate().RawSubjectPublicKeyInfo)
	client, retire, err := newRunLinkClient(enroll.Server{Address: server.URL, CertificateKeySHA256: pin[:]})
	if err != nil {
		t.Fatal("could not start the production link client for the outer byte cap")
	}
	defer retire()
	stream := client.Open(ctx)
	defer stream.CloseResponse()
	defer stream.CloseRequest()
	exactOut := &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Heartbeat{Heartbeat: &linkv1.Heartbeat{}}}
	longOut := proto.CloneOf(exactOut)
	padFolderMessage(t, exactOut, limit)
	padFolderMessage(t, longOut, limit+1)
	if err := stream.Send(exactOut); err != nil {
		t.Fatal("the send cap rejected a complete message at the reviewed limit")
	}
	if err := stream.Send(longOut); connect.CodeOf(err) != connect.CodeResourceExhausted {
		t.Error("the send cap accepted a complete message one byte above the reviewed limit")
	}
	got, err := stream.Receive()
	if err != nil || got.GetHeartbeatAck() == nil || proto.Size(got) != limit {
		t.Fatal("the receive cap rejected a complete message at the reviewed limit")
	}
	if _, err := stream.Receive(); connect.CodeOf(err) != connect.CodeResourceExhausted {
		t.Error("the receive cap accepted a complete message one byte above the reviewed limit")
	}
	select {
	case err := <-peerResult:
		if err != nil {
			t.Fatal("the loopback peer did not decode the exact-limit request")
		}
	case <-ctx.Done():
		t.Fatal("the loopback peer did not observe the exact-limit request")
	}
}
