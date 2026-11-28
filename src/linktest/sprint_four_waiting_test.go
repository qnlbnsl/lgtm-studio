package linktest

import (
	"context"
	"encoding/binary"
	"slices"
	"sync"
	"testing"

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
	operation, command := newPairedID(), newPairedID()
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
}
