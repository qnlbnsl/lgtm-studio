package linktest

import (
	"context"
	"encoding/binary"
	"slices"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/proto"

	"github.com/qnlbnsl/lgtm-studio/src/client/clienttest"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/servertest"
)

// The stopped client receives the payload format that it would have kept from a prior process.
func seedPairedEvents(t *testing.T, opts clienttest.Options, events ...[]byte) {
	t.Helper()
	withClosedClientDB(t, opts, true, func(db *bolt.DB) error {
		return db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte("events"))
			for i, payload := range events {
				if err := bucket.Put(binary.BigEndian.AppendUint64(nil, uint64(i+1)), payload); err != nil {
					return err
				}
			}
			return tx.Bucket([]byte("meta")).Put([]byte("next"), binary.BigEndian.AppendUint64(nil, uint64(len(events)+1)))
		})
	})
}

func retainedPairedEvent(t *testing.T, event *linkv1.Event) []byte {
	t.Helper()
	if event.GetNumber() != 0 {
		t.Fatal("A retained event must omit its wire number.")
	}
	body, err := proto.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	return append([]byte{1}, body...)
}

func TestSprintFourWaitingReplay(t *testing.T) {
	requirePostgres(t)
	atFinalAck, release := make(chan struct{}), make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	defer unblock()
	server := servertest.Start(t, servertest.Options{Password: fixturePassword, LinkSeams: servertest.LinkSeams{
		BeforeEventAck: func(ctx context.Context, _ string, number uint64) {
			if number == 2 {
				close(atFinalAck)
				select {
				case <-release:
				case <-ctx.Done():
				}
			}
		},
	}})
	page := server.Clients(t)
	r := startRelay(t, server.Link())
	opts := newClient(t, new(diagnostics))
	id := join(t, page, opts, r.address)
	db := openObserver(t, server)
	// Accept an operation while approved, then prepare a waiting Welcome for a
	// retained report carrying that exact operation and command identity.
	approve(t, page, id, "Waiting replay fixture")
	script := pairedScript{server: server, opts: opts, id: id, db: db}
	approved := script.attempt(t, newPairedID(), 1)
	requirePairedWelcome(t, approved, 0)
	settlePairedApproval(t, db, approved)
	projects := projectService(server)
	call := beginAdd(t, projects, id, "waiting-report", t.TempDir())
	op := waitProjectOperation(t, db, "waiting-report")
	readPairedFolderCommand(t, approved, op)
	waitFor(t, 5*time.Second, "the scripted folder command to be dispatched", func() bool {
		return scalar[bool](t, db, "SELECT dispatched FROM link_commands WHERE id=$1", op.command)
	})
	// Keep the accepted operation open without redelivering its command after approval.
	execute(t, db, "UPDATE link_commands SET expires_at=clock_timestamp()-interval '25 hours' WHERE id=$1", op.command)
	execute(t, db, "UPDATE machines SET approval='waiting',state_revision=state_revision+1 WHERE id=$1", id)
	operation, command := op.id, op.command
	seedPairedEvents(t, opts,
		retainedPairedEvent(t, pairedFacts(0, "retained waiting facts")),
		retainedPairedEvent(t, pairedFolder(0, operation, command)),
	)
	run := startClient(t, opts)
	leg := receive(t, r.legs, "the waiting stream")
	receive(t, atFinalAck, "the consumed waiting folder event to reach the acknowledgement barrier")
	trace := leg.observe()
	if !trace.welcomed || trace.mark != 0 || trace.approval != linkv1.Approval_APPROVAL_WAITING ||
		trace.hello.machineID != id || trace.hello.revision != 2 || trace.hello.runID == "" || trace.hello.first != 3 ||
		!slices.Equal(trace.events, []uint64{1, 2}) || len(trace.commands) != 0 ||
		len(trace.reports) != 2 || trace.reports[1].folder != (folderReport{2, operation, command}) ||
		mark(t, db, id) != 2 {
		t.Fatalf("The waiting stream did not commit just the retained prefix before approval: %+v.", trace)
	}
	var facts linkv1.Facts
	if err := proto.Unmarshal(scalar[[]byte](t, db, "SELECT facts FROM machines WHERE id=$1", id), &facts); err != nil ||
		facts.GetClientVersion() != "retained waiting facts" {
		t.Fatal("The retained Facts did not take effect on the waiting stream.")
	}
	waitForAck(t, leg, 1)
	requireOpenProject(t, readProjectOperation(t, db, "waiting-report"))
	requireProjectRows(t, db, 0)
	requireProjectSaveCount(t, db, 0)
	if slices.Contains(leg.observe().acks, uint64(2)) {
		t.Fatal("The held folder acknowledgement reached the waiting client.")
	}
	approve(t, page, id, "Waiting replay fixture")
	unblock()

	standing := waitForCommand(t, leg, stateAction, 0)
	waitForResult(t, leg, standing, commandDone)
	waitForStoredResult(t, db, standing, "done")
	waitForClient(t, page, id, webv1.ClientState_CLIENT_STATE_ONLINE)
	trace = leg.observe()
	firstAck := slices.Index(trace.sends, frameSend{kind: "ack", number: 1})
	finalAck := slices.Index(trace.sends, frameSend{kind: "ack", number: 2})
	stateSend := slices.Index(trace.sends, frameSend{kind: "command", commandID: standing.id, attempt: standing.attempt})
	if !(0 <= firstAck && firstAck < finalAck && finalAck < stateSend) {
		t.Fatalf("Both retained acknowledgements must precede SetState on the original stream: %v.", trace.sends)
	}
	run.stop(t)
	requireAcknowledgedOutbox(t, opts, 2)
	requireOpenProject(t, readProjectOperation(t, db, "waiting-report"))
	remove(t, page, id)
	requireAddRefusal(t, finishAdd(t, call), connect.CodeNotFound, "The machine was removed.")
	t.Run("StreamFencing", testPairedProjectStreamFencing)
}

func testPairedProjectStreamFencing(t *testing.T) {
	root, _, _ := gitCheckout(t)
	f := newProjectFixture(t, servertest.Options{}, relayControls{})
	run, leg := f.startApproved(t, 0)
	baseline := drainPairedInventory(t, &f.commandFixture, leg, "1.0.0")
	run.stop(t)
	target := pairedScript{server: f.server, opts: f.opts, id: f.id, db: f.db}
	replaced := target.attempt(t, newPairedID(), baseline+1)
	requirePairedWelcome(t, replaced, baseline)
	call := beginAdd(t, f.projects, f.id, "fenced-report", root)
	op := waitProjectOperation(t, f.db, "fenced-report")
	readPairedFolderCommand(t, replaced, op)

	otherOptions := newClient(t, new(diagnostics))
	otherID := join(t, f.page, otherOptions, f.server.URL)
	approve(t, f.page, otherID, "Other machine fixture")
	other := pairedScript{server: f.server, opts: otherOptions, id: otherID, db: f.db}
	otherStream := other.attempt(t, newPairedID(), 1)
	requirePairedWelcome(t, otherStream, 0)
	settlePairedApproval(t, f.db, otherStream)
	sendPairedFrame(t, otherStream, pairedEvent(projectFolderFacts(1, op, root)))
	requireProjectAck(t, otherStream, 1)
	requireOpenProject(t, readProjectOperation(t, f.db, "fenced-report"))
	requireProjectRows(t, f.db, 0)

	current := target.attempt(t, newPairedID(), baseline+1)
	requirePairedWelcome(t, current, baseline)
	readPairedFolderCommand(t, current, op)
	report := projectFolderFacts(baseline+1, op, root)
	_ = replaced.Send(pairedEvent(report))
	requirePairedEnd(t, replaced, connect.CodeAborted, "")
	if mark(t, f.db, f.id) != baseline {
		t.Fatal("A report on a replaced stream advanced the target machine's mark.")
	}
	requireOpenProject(t, readProjectOperation(t, f.db, "fenced-report"))
	requireProjectRows(t, f.db, 0)
	sendPairedFrame(t, current, pairedEvent(report))
	requireProjectAck(t, current, baseline+1)
	requireAddReply(t, finishAdd(t, call), f.id, root)
	requireProjectSaveCount(t, f.db, 1)

	// Finish the first Link command so the next one can be delivered.
	prior := scalar[uint32](t, f.db, "SELECT attempt FROM link_commands WHERE id=$1", op.command)
	waitFor(t, 5*time.Second, "the replacement's folder dispatch marker", func() bool {
		return scalar[bool](t, f.db, "SELECT dispatched FROM link_commands WHERE id=$1", op.command)
	})
	sendPairedFrame(t, current, pairedResult(op.command, prior, baseline+1))
	waitForStoredResult(t, f.db, commandDelivery{id: op.command, attempt: prior}, "done")
	removedCall := beginAdd(t, f.projects, f.id, "removed-stream-report", root)
	removedOp := waitProjectOperation(t, f.db, "removed-stream-report")
	readPairedFolderCommand(t, current, removedOp)
	remove(t, f.page, f.id)
	_ = current.Send(pairedEvent(projectFolderFacts(baseline+2, removedOp, root)))
	requirePairedEnd(t, current, connect.CodeFailedPrecondition, "unknown_machine")
	requireAddRefusal(t, finishAdd(t, removedCall), connect.CodeNotFound, "The machine was removed.")
	after := readProjectOperation(t, f.db, "removed-stream-report")
	if after.id != removedOp.id || after.command != removedOp.command || after.state != "failed" || after.failure != "machine_removed" {
		t.Fatal("A removed stream changed the removed operation's identity or failure.")
	}
	requireProjectRows(t, f.db, 0)
	requireProjectSaveCount(t, f.db, 1)
}
