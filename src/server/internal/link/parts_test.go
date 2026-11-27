package link

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

var _ machines.CommandQueue = (*State)(nil)

const partCommandTTL = 2 * time.Minute

func enqueuePartRestart(ctx context.Context, tx *store.Tx, s *State, machineID string) (string, error) {
	id, err := s.EnqueueCommand(ctx, tx, machineID, restartAction(), partCommandTTL)
	if err != nil {
		return "", err
	}
	if !streamVersion7.MatchString(id) {
		return "", store.NewDomainError("Transactional enqueue did not return a version 7 command identifier.")
	}
	return id, nil
}

func requirePartOutboxWakes(t *testing.T, f *presenceFixture, head int64, machineID string, want int64) {
	t.Helper()
	wakes := storetest.Scalar[int64](t, f.obs, `SELECT count(*) FROM feed
		WHERE position > $1 AND topic = 'machine_outbox' AND scope = '' AND key = $2 AND change = 'saved'`, head, machineID)
	total := storetest.Scalar[int64](t, f.obs, `SELECT count(*) FROM feed WHERE position > $1`, head)
	if wakes != want || total != want {
		t.Fatalf("The caller writes committed %d matching outbox wakes and %d total references. Want %d of each.", wakes, total, want)
	}
}

func requirePartRestartDelivery(t *testing.T, f *handshakeFixture, m machines.Machine, key ed25519.PrivateKey, want []string) {
	t.Helper()
	var got []string
	var expiries []time.Time
	err := f.obs.QueryRow(t.Context(), `SELECT array_agg(id ORDER BY seq), array_agg(expires_at ORDER BY seq)
		FROM link_commands WHERE machine_id = $1`, m.ID).Scan(&got, &expiries)
	if err != nil {
		t.Fatal("The observer could not read the committed command identifiers and expiries.")
	}
	if !slices.Equal(got, want) {
		t.Fatalf("The queue has %d commands and does not match the caller identifiers in their expected order. Want %d commands.", len(got), len(want))
	}
	const label = "part enqueue"
	probe := newCommandWireProbe()
	probe.flushing = make(chan commandFlush, len(want)+1)
	probe.controls = newSteadyClock().controls()
	server := commandListener(t, f, map[string]*commandWireProbe{label: probe})
	// The bulk case drains more frames than the ordinary command stream fixture.
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	stream := integrationCommandClient(t, server.URL).Open(ctx)
	stream.RequestHeader().Set("X-Command-Fixture", label)
	t.Cleanup(func() {
		cancel()
		_ = stream.CloseRequest()
		_ = stream.CloseResponse()
	})
	submitProof(t, f, stream, m.ID, key, 1)
	requireWelcome(t, stream)
	for i, id := range want {
		frame, err := stream.Receive()
		if err != nil || frame.GetCommand() == nil {
			t.Fatalf("The stream did not deliver command %d of %d. The receive code was %v.", i+1, len(want), connect.CodeOf(err))
		}
		row := claimFixtureRow{id: id, expires: expiries[i], action: restartAction()}
		row.requireFrame(t, frame.GetCommand(), 1)
	}
	sendHandshake(t, stream, heartbeatFrame(nil, nil))
	frame, err := stream.Receive()
	if err != nil || frame.GetHeartbeatAck() == nil {
		t.Fatal("The stream did not acknowledge a heartbeat after delivering exactly the committed commands.")
	}
}

func TestPartEnqueueIsAtomicAndOrdered(t *testing.T) {
	t.Run("same-machine commit order", testPartEnqueueCommitOrder)
	t.Run("bulk enqueue records one wake", testPartEnqueueBulk)
	t.Run("bulk rollback leaves nothing to send", testPartEnqueueRollback)
	t.Run("a removed machine refuses enqueue inside its removal write", testPartEnqueueMissingMachine)
	t.Run("the Machines adapter shares the caller wake", testPartEnqueueAdapter)
}

func testPartEnqueueCommitOrder(t *testing.T) {
	f := newServerStreamFixture(t, handshakeFixtureOptions{})
	m, key := f.register(t, "ordered part commands")
	f.approve(t, m.ID)
	outboxFixtureExec(t, f.obs, `CREATE TABLE link_part_enqueue_fixture (caller text PRIMARY KEY, command_id text NOT NULL)`)
	head := f.head(t)

	type writeResult struct {
		id  string
		err error
	}
	firstPID, secondPID := make(chan int32, 1), make(chan int32, 1)
	held, release := storetest.Signal(t)
	defer release()
	start := func(caller string, beforeLock, afterStore chan<- int32) *storetest.Task[writeResult] {
		return storetest.Start(t, func() writeResult {
			var id string
			err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
				var pid int32
				if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
					return err
				}
				if beforeLock != nil {
					beforeLock <- pid
				}
				if _, err := f.r.Lock(ctx, tx, m.ID); err != nil {
					return err
				}
				queuedID, err := enqueuePartRestart(ctx, tx, f.state, m.ID)
				if err != nil {
					return err
				}
				id = queuedID
				if _, err := tx.Exec(ctx, `INSERT INTO link_part_enqueue_fixture VALUES ($1, $2)`, caller, id); err != nil {
					return err
				}
				if afterStore != nil {
					afterStore <- pid
					select {
					case <-held:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
				return nil
			})
			return writeResult{id: id, err: err}
		})
	}
	first := start("A", nil, firstPID)
	var aPID int32
	select {
	case aPID = <-firstPID:
	case <-first.Done():
		t.Fatal("Caller A did not store a valid command identifier in its write.")
	case <-time.After(storetest.ObservationTimeout):
		t.Fatal("Caller A did not reach its barrier after storing a command and sentinel.")
	}
	if outboxCount(t, f.presenceFixture) != 0 || storetest.Scalar[int64](t, f.obs, `SELECT count(*) FROM link_part_enqueue_fixture`) != 0 {
		t.Fatal("Caller A's uncommitted command or sentinel became visible.")
	}
	requirePartOutboxWakes(t, f.presenceFixture, head, m.ID, 0)
	seq := storetest.Scalar[int64](t, f.obs, `SELECT last_value FROM link_commands_seq_seq`)
	second := start("B", secondPID, nil)
	bPID := storetest.Receive(t, "caller B starting its transaction", secondPID)
	storetest.WaitOnRow(t, f.obs, bPID, aPID)
	if !storetest.Scalar[bool](t, f.obs, `SELECT query ILIKE '%FROM machines%FOR NO KEY UPDATE%' FROM pg_stat_activity WHERE pid = $1`, bPID) {
		t.Fatal("Caller B did not wait on the machine row as its first domain lock.")
	}
	if storetest.Scalar[int64](t, f.obs, `SELECT last_value FROM link_commands_seq_seq`) != seq {
		t.Fatal("Caller B allocated a command sequence before acquiring the machine lock.")
	}
	release()
	a, b := first.Wait(t), second.Wait(t)
	if a.err != nil || b.err != nil || a.id == b.id {
		t.Fatal("The ordered caller writes did not commit two distinct command identifiers.")
	}
	want := []string{a.id, b.id}
	if coupled := storetest.Rows[string](t, f.obs, `SELECT command_id FROM link_part_enqueue_fixture ORDER BY caller`); !slices.Equal(coupled, want) {
		t.Fatal("The caller records did not commit the exact identifiers returned by enqueue.")
	}
	requirePartOutboxWakes(t, f.presenceFixture, head, m.ID, 2)
	requirePartRestartDelivery(t, f, m, key, want)
}

func testPartEnqueueBulk(t *testing.T) {
	f := newServerStreamFixture(t, handshakeFixtureOptions{})
	m, key := f.register(t, "bulk part commands")
	f.approve(t, m.ID)
	head := f.head(t)
	var ids []string
	err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		if _, err := f.r.Lock(ctx, tx, m.ID); err != nil {
			return err
		}
		for range store.MaxWriteChanges + 1 {
			id, err := enqueuePartRestart(ctx, tx, f.state, m.ID)
			if err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return nil
	})
	if err != nil || len(ids) != store.MaxWriteChanges+1 {
		t.Fatal("A single caller write could not store and return every identifier above the feed reference limit.")
	}
	if !storetest.Scalar[bool](t, f.obs, `SELECT bool_and(expires_at - created_at = interval '2 minutes')
		FROM link_commands WHERE machine_id = $1`, m.ID) {
		t.Fatal("The bulk enqueue changed the requested command lifetime.")
	}
	requirePartOutboxWakes(t, f.presenceFixture, head, m.ID, 1)
	requirePartRestartDelivery(t, f, m, key, ids)
}

func testPartEnqueueRollback(t *testing.T) {
	f := newServerStreamFixture(t, handshakeFixtureOptions{})
	m, key := f.register(t, "aborted part commands")
	f.approve(t, m.ID)
	outboxFixtureExec(t, f.obs, `CREATE TABLE link_part_enqueue_fixture (caller text PRIMARY KEY, command_id text NOT NULL)`)
	head := f.head(t)
	abort := store.NewDomainError("The fixture aborted the caller write.")
	err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		if _, err := f.r.Lock(ctx, tx, m.ID); err != nil {
			return err
		}
		for range store.MaxWriteChanges + 1 {
			id, err := enqueuePartRestart(ctx, tx, f.state, m.ID)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO link_part_enqueue_fixture VALUES ($1, $1)`, id); err != nil {
				return err
			}
		}
		return abort
	})
	if !errors.Is(err, abort) {
		t.Fatal("The bulk caller write did not reach its forced rollback after every enqueue succeeded.")
	}
	if outboxCount(t, f.presenceFixture) != 0 || storetest.Scalar[int64](t, f.obs, `SELECT count(*) FROM link_part_enqueue_fixture`) != 0 {
		t.Fatal("The caller rollback left command rows or caller records committed.")
	}
	requirePartOutboxWakes(t, f.presenceFixture, head, m.ID, 0)
	requirePartRestartDelivery(t, f, m, key, nil)
}

func testPartEnqueueMissingMachine(t *testing.T) {
	f := newPresenceFixture(t, nil)
	m := f.machine(t, bytes.Repeat([]byte{1}, 32), "removed part machine")
	head := f.head(t)
	var refused bool
	err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		if _, err := f.r.Lock(ctx, tx, m.ID); err != nil {
			return err
		}
		if err := f.r.Delete(ctx, tx, m.ID); err != nil {
			return err
		}
		id, err := f.state.EnqueueCommand(ctx, tx, m.ID, restartAction(), partCommandTTL)
		refused = id == "" && errors.Is(err, ErrUnknownMachine)
		return nil
	})
	if err != nil || !refused {
		t.Error("The removal write could not handle an unknown-machine refusal with an empty command identifier.")
	}
	want := []string{"machines//" + m.ID + "/removed"}
	refs := storetest.Rows[string](t, f.obs, `SELECT topic || '/' || scope || '/' || key || '/' || change
		FROM feed WHERE position > $1 ORDER BY position`, head)
	if outboxCount(t, f) != 0 || !slices.Equal(refs, want) ||
		storetest.Scalar[bool](t, f.obs, `SELECT EXISTS (SELECT FROM machines WHERE id = $1)`, m.ID) {
		t.Error("Removal did not commit alone, without a command or an outbox wake.")
	}
}

func testPartEnqueueAdapter(t *testing.T) {
	f := newPresenceFixture(t, nil)
	m := f.machine(t, bytes.Repeat([]byte{1}, 32), "legacy part adapter")
	head := f.head(t)
	var queue machines.CommandQueue = f.state
	err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		if _, err := f.r.Lock(ctx, tx, m.ID); err != nil {
			return err
		}
		if _, err := enqueuePartRestart(ctx, tx, f.state, m.ID); err != nil {
			return err
		}
		return queue.Enqueue(ctx, tx, m.ID, restartAction(), partCommandTTL)
	})
	if err != nil || outboxCount(t, f) != 2 {
		t.Fatal("The identifier-returning queue and the Machines adapter did not both store a command.")
	}
	requirePartOutboxWakes(t, f, head, m.ID, 1)
}

func TestLinkPartRegistration(t *testing.T) {
	f := newPresenceFixture(t, nil)
	m := f.machine(t, bytes.Repeat([]byte{1}, 32), "part callbacks")
	outboxFixtureExec(t, f.obs, `CREATE TABLE link_part_hook_fixture (
		seq bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, callback text NOT NULL UNIQUE)`)
	head := f.head(t)

	commandID, operationID, streamID, runID := fixtureStreamID(t), fixtureStreamID(t), fixtureStreamID(t), fixtureStreamID(t)
	expiry := timestamppb.New(time.Date(2026, time.November, 27, 12, 0, 0, 0, time.UTC))
	var zero uint64
	folder := Settlement{
		MachineID: m.ID, Outcome: OutcomeDone, LastEventNumber: &zero,
		Command: &linkv1.Command{Id: commandID, Attempt: 3, ExpiresAt: expiry,
			Action: &linkv1.Command_LookAtFolder{LookAtFolder: &linkv1.LookAtFolder{OperationId: operationID, Path: "/fixture/repository"}}},
	}
	restart := Settlement{MachineID: m.ID, Outcome: OutcomeTimedOut, Command: restartAction()}
	restart.Command.Id, restart.Command.Attempt, restart.Command.ExpiresAt = fixtureStreamID(t), 1, expiry
	scan := Settlement{MachineID: m.ID, Outcome: OutcomeTimedOut, Command: scanAction()}
	standing := Settlement{MachineID: m.ID, Outcome: OutcomeRemoved, Command: standingAction()}
	folderEvent := &linkv1.Event{Number: 1, HappenedAt: expiry, Report: &linkv1.Event_FolderSeen{FolderSeen: &linkv1.FolderSeen{
		OperationId: operationID, CommandId: commandID,
		Result: &linkv1.FolderSeen_Refusal{Refusal: linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_A_FOLDER},
	}}}
	factsEvent := &linkv1.Event{Number: 2, HappenedAt: expiry, Report: &linkv1.Event_Facts{Facts: &linkv1.Facts{Cores: 2}}}
	harnessEvent := &linkv1.Event{Number: 3, HappenedAt: expiry,
		Report: &linkv1.Event_HarnessesFound{HarnessesFound: &linkv1.HarnessesFound{}}}
	admission := Admission{MachineID: m.ID, StreamID: streamID, RunID: runID, FirstEventNumber: 1, Ready: true}
	run := RunChange{MachineID: m.ID, RunID: runID, FirstEventNumber: 1}

	var callerContext context.Context
	var callerTx *store.Tx
	var callerName string
	var calls []string
	var failAdmission bool
	abort := store.NewDomainError("The first part refused the fixture admission.")
	wrongCall := store.NewDomainError("The dispatcher called the wrong hook or changed its inputs.")
	record := func(ctx context.Context, tx *store.Tx, name string) error {
		if ctx != callerContext || tx != callerTx {
			return wrongCall
		}
		var callerVisible bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT FROM link_part_hook_fixture WHERE callback = $1)`, callerName).Scan(&callerVisible); err != nil {
			return err
		}
		if !callerVisible {
			return wrongCall
		}
		if _, err := tx.Exec(ctx, `INSERT INTO link_part_hook_fixture (callback) VALUES ($1)`, name); err != nil {
			return err
		}
		calls = append(calls, name)
		return tx.Record(store.ChangeRef{Topic: "fixture_part", Key: name, Change: store.Saved})
	}
	settle := func(name string, want Settlement) func(context.Context, *store.Tx, Settlement) error {
		return func(ctx context.Context, tx *store.Tx, got Settlement) error {
			if got.MachineID != want.MachineID || got.Outcome != want.Outcome || !proto.Equal(got.Command, want.Command) {
				return wrongCall
			}
			if (got.LastEventNumber == nil) != (want.LastEventNumber == nil) ||
				(got.LastEventNumber != nil && *got.LastEventNumber != *want.LastEventNumber) {
				return wrongCall
			}
			return record(ctx, tx, name)
		}
	}
	report := func(name string, want *linkv1.Event) func(context.Context, *store.Tx, string, *linkv1.Event) error {
		return func(ctx context.Context, tx *store.Tx, machineID string, event *linkv1.Event) error {
			if machineID != m.ID || !proto.Equal(event, want) {
				return wrongCall
			}
			return record(ctx, tx, name)
		}
	}
	hooks := func(part string) PartHooks {
		return PartHooks{
			StreamAdmitted: func(ctx context.Context, tx *store.Tx, got Admission) error {
				if got != admission {
					return wrongCall
				}
				if err := record(ctx, tx, part+"-admission"); err != nil {
					return err
				}
				if part == "first" && failAdmission {
					return abort
				}
				return nil
			},
			RunChanged: func(ctx context.Context, tx *store.Tx, got RunChange) error {
				if got != run {
					return wrongCall
				}
				return record(ctx, tx, part+"-run")
			},
			MachineRemoved: func(ctx context.Context, tx *store.Tx, machineID string) error {
				if machineID != m.ID {
					return wrongCall
				}
				return record(ctx, tx, part+"-removal")
			},
		}
	}

	first := hooks("first")
	first.CommandSettled = map[CommandKind]func(context.Context, *store.Tx, Settlement) error{
		CommandLookAtFolder: settle("first-command", folder),
	}
	first.ReportApplied = map[ReportKind]func(context.Context, *store.Tx, string, *linkv1.Event) error{
		ReportFolderSeen: report("first-report", folderEvent),
	}
	second := hooks("second")
	second.CommandSettled = map[CommandKind]func(context.Context, *store.Tx, Settlement) error{
		"restart": settle("second-command", restart),
	}
	second.ReportApplied = map[ReportKind]func(context.Context, *store.Tx, string, *linkv1.Event) error{
		"facts": report("second-report", factsEvent),
	}
	if f.state.RegisterPart(first) != nil || f.state.RegisterPart(second) != nil {
		t.Fatal("Two parts with different routes could not register.")
	}
	first.CommandSettled[CommandLookAtFolder] = settle("changed-command", folder)
	first.CommandSettled["set_state"] = settle("added-command", standing)
	delete(first.ReportApplied, ReportFolderSeen)

	duplicateCommand := hooks("duplicate-command")
	duplicateCommand.CommandSettled = map[CommandKind]func(context.Context, *store.Tx, Settlement) error{
		CommandLookAtFolder: settle("duplicated-command", folder),
	}
	duplicateCommand.ReportApplied = map[ReportKind]func(context.Context, *store.Tx, string, *linkv1.Event) error{
		"harnesses_found": report("partially-registered-report", harnessEvent),
	}
	if f.state.RegisterPart(duplicateCommand) == nil {
		t.Error("Registration accepted a duplicate command route.")
	}
	duplicateReport := hooks("duplicate-report")
	duplicateReport.CommandSettled = map[CommandKind]func(context.Context, *store.Tx, Settlement) error{
		"scan_harnesses": settle("partially-registered-command", scan),
	}
	duplicateReport.ReportApplied = map[ReportKind]func(context.Context, *store.Tx, string, *linkv1.Event) error{
		ReportFolderSeen: report("duplicated-report", folderEvent),
	}
	if f.state.RegisterPart(duplicateReport) == nil {
		t.Error("Registration accepted a duplicate report route.")
	}

	write := func(name string, dispatch func(context.Context, *store.Tx) error) error {
		return f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
			callerContext, callerTx, callerName = ctx, tx, name
			if _, err := f.r.Lock(ctx, tx, m.ID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO link_part_hook_fixture (callback) VALUES ($1)`, name); err != nil {
				return err
			}
			return dispatch(ctx, tx)
		})
	}
	failAdmission = true
	err := write("aborted-caller", func(ctx context.Context, tx *store.Tx) error { return f.state.dispatchAdmission(ctx, tx, admission) })
	if !errors.Is(err, abort) || !slices.Equal(calls, []string{"first-admission"}) {
		t.Error("The first admission error did not stop dispatch and reach the caller.")
	}
	if f.head(t) != head || storetest.Scalar[int64](t, f.obs, `SELECT count(*) FROM link_part_hook_fixture`) != 0 {
		t.Error("The failing part did not roll back its sentinel, the caller sentinel and the feed together.")
	}

	failAdmission, calls = false, nil
	err = write("caller", func(ctx context.Context, tx *store.Tx) error {
		for _, route := range []struct {
			kind       CommandKind
			settlement Settlement
		}{{"look_at_folder", folder}, {"restart", restart}, {"scan_harnesses", scan}, {"set_state", standing}} {
			value := route.settlement
			value.Command = proto.CloneOf(value.Command)
			if value.LastEventNumber != nil {
				number := *value.LastEventNumber
				value.LastEventNumber = &number
			}
			if err := f.state.dispatchSettlement(ctx, tx, route.kind, value); err != nil {
				return err
			}
		}
		for _, route := range []struct {
			kind  ReportKind
			event *linkv1.Event
		}{{"folder_seen", folderEvent}, {"facts", factsEvent}, {"harnesses_found", harnessEvent}} {
			if err := f.state.dispatchReport(ctx, tx, route.kind, m.ID, proto.CloneOf(route.event)); err != nil {
				return err
			}
		}
		if err := f.state.dispatchAdmission(ctx, tx, admission); err != nil {
			return err
		}
		if err := f.state.dispatchRunChange(ctx, tx, run); err != nil {
			return err
		}
		return f.state.dispatchRemoval(ctx, tx, m.ID)
	})
	wantCalls := []string{"first-command", "second-command", "first-report", "second-report",
		"first-admission", "second-admission", "first-run", "second-run", "first-removal", "second-removal"}
	if err != nil || !slices.Equal(calls, wantCalls) {
		t.Error("Dispatch did not keep the original typed routes and both sets of lifecycle hooks in registration order.")
	}
	rows := storetest.Rows[string](t, f.obs, `SELECT callback FROM link_part_hook_fixture ORDER BY seq`)
	feed := storetest.Rows[string](t, f.obs, `SELECT key FROM feed
		WHERE position > $1 AND topic = 'fixture_part' AND scope = '' AND change = 'saved' ORDER BY position`, head)
	if !slices.Equal(rows, append([]string{"caller"}, wantCalls...)) || !slices.Equal(feed, wantCalls) {
		t.Error("The registered hooks, caller sentinel and feed did not commit together in dispatch order.")
	}
}
