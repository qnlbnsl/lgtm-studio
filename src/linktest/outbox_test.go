package linktest

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	"github.com/qnlbnsl/lgtm-studio/src/client/clienttest"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/servertest"
)

const (
	commitLock    = 377113
	unknownMarker = "unused report field marker"
)

type ackGate struct {
	id      string
	mark    uint64
	entered chan context.Context
	release chan struct{}
}

func TestLinkOutboxReplayAfterServerFailure(t *testing.T) {
	requirePostgres(t)
	var activeGate atomic.Pointer[ackGate]
	serverLog := &diagnostics{marker: unknownMarker}
	server := servertest.Start(t, servertest.Options{
		Password: fixturePassword,
		Log:      serverLog,
		LinkSeams: servertest.LinkSeams{
			BeforeEventAck: func(ctx context.Context, id string, mark uint64) {
				if gate := activeGate.Load(); gate != nil && gate.id == id && gate.mark == mark {
					gate.entered <- ctx
					select {
					case <-gate.release:
					case <-t.Context().Done():
					}
				}
			},
		},
	})
	page := server.Clients(t)
	var holdDrain atomic.Bool
	drain := make(chan struct{})
	releaseDrain := sync.OnceFunc(func() { close(drain) })
	defer releaseDrain()
	r := startRelayWithControls(t, server.Link(), relayControls{releaseReports: func() <-chan struct{} {
		if holdDrain.Load() {
			return drain
		}
		return nil
	}})
	logs := new(diagnostics)
	opts := newClient(t, logs)
	id := join(t, page, opts, r.address)
	approve(t, page, id, "Outbox fixture")
	db := openObserver(t, server)

	run := startClient(t, opts)
	live := receive(t, r.legs, "the baseline stream")
	waitForClient(t, page, id, webv1.ClientState_CLIENT_STATE_ONLINE)
	base := mark(t, db, id)
	if base < 2 {
		t.Fatal("The approved baseline did not commit its two readiness reports.")
	}
	waitForAck(t, live, base)
	installAudit(t, db, id, base+1)
	checkAudit(t, db, base, base)

	r.blockNew.Store(true)
	server.Stop(t)
	receive(t, live.done, "the server shutdown to close the live stream")
	waitFor(t, 45*time.Second, "three facts to be buffered while the server is stopped", func() bool {
		return len(logs.bufferedAfter(base)) >= 3
	})
	run.stop(t)
	outage := readPending(t, opts)
	checkPending(t, logs, outage, base)
	if len(outage) < 3 {
		t.Fatal("The client did not durably queue at least three reports during the outage.")
	}
	for _, report := range outage {
		if !report.facts {
			t.Fatal("An outage report was not a saved Facts event.")
		}
	}

	// Cut the first replay after its database transaction reaches the commit barrier.
	server.Restart(t)
	execute(t, db, "SELECT pg_advisory_lock($1)", int64(commitLock))
	r.blockNew.Store(false)
	run = startClient(t, opts)
	precommit := receive(t, r.legs, "the first replay stream")
	var pid int32
	waitFor(t, 10*time.Second, "the report transaction to wait at commit", func() bool {
		pid = scalar[int32](t, db, `SELECT coalesce((
			SELECT pid FROM pg_locks
			WHERE locktype='advisory' AND database=(
				SELECT oid FROM pg_database WHERE datname=current_database()
			) AND classid=0 AND objid=$1 AND objsubid=1 AND NOT granted
		), 0)`, uint32(commitLock))
		return pid != 0
	})
	first := precommit.observe()
	if !first.welcomed || first.mark != base || len(first.events) == 0 || first.events[0] != base+1 {
		t.Fatal("The first recovery did not replay the next report after its Welcome mark.")
	}
	checkAudit(t, db, base, base)
	r.blockNew.Store(true)
	precommit.cut()
	if !scalar[bool](t, db, "SELECT pg_terminate_backend($1, 4000)", pid) {
		t.Fatal("The observer did not terminate the blocked report backend.")
	}
	execute(t, db, "SELECT pg_advisory_unlock($1)", int64(commitLock))
	receive(t, precommit.done, "the precommit cut to close the stream")
	run.stop(t)
	if slices.Contains(precommit.observe().acks, base+1) {
		t.Fatal("The rolled-back report reached the client as acknowledged.")
	}
	if mark(t, db, id) != base {
		t.Fatal("The cut at commit advanced the durable mark.")
	}
	checkAudit(t, db, base, base)
	checkPending(t, logs, readPending(t, opts), base)

	// This replay commits, but its acknowledgement is held until the stream is canceled.
	gate := &ackGate{id: id, mark: base + 1, entered: make(chan context.Context, 1), release: make(chan struct{})}
	release := sync.OnceFunc(func() { close(gate.release) })
	defer release()
	activeGate.Store(gate)
	server.Restart(t)
	r.blockNew.Store(false)
	run = startClient(t, opts)
	postcommit := receive(t, r.legs, "the replay with the acknowledgement barrier")
	ackContext := receive(t, gate.entered, "the committed report to reach the acknowledgement barrier")
	second := postcommit.observe()
	if !second.welcomed || second.mark != base || len(second.events) == 0 || second.events[0] != base+1 {
		t.Fatal("The next Welcome did not replay the report whose transaction rolled back.")
	}
	if mark(t, db, id) != base+1 {
		t.Fatal("The held acknowledgement was reached before the report committed.")
	}
	checkAudit(t, db, base, base+1)
	r.blockNew.Store(true)
	postcommit.cut()
	receive(t, ackContext.Done(), "the acknowledgement hook's stream to be canceled")
	release()
	activeGate.Store(nil)
	receive(t, postcommit.done, "the postcommit cut to close the stream")
	run.stop(t)
	if slices.Contains(postcommit.observe().acks, base+1) {
		t.Fatal("The cut stream forwarded the held acknowledgement.")
	}
	pending := readPending(t, opts)
	checkPending(t, logs, pending, base)

	// A Welcome carrying the committed mark replaces the lost acknowledgement.
	// Hold the replay at the relay until both fresh readiness reports
	// are buffered. On the live link the periodic tick sends only heartbeats.
	final := pending[len(pending)-1].number + 2
	holdDrain.Store(true)
	server.Restart(t)
	page = server.Clients(t)
	r.blockNew.Store(false)
	run = startClient(t, opts)
	recovered := receive(t, r.legs, "the final recovery stream")
	waitFor(t, 10*time.Second, "both fresh readiness reports to be buffered", func() bool {
		return slices.Contains(logs.bufferedAfter(base), final)
	})
	releaseDrain()
	waitForAck(t, recovered, final)
	waitForClientToProcessReplies(t, recovered)
	waitForClient(t, page, id, webv1.ClientState_CLIENT_STATE_ONLINE)
	run.stop(t)
	receive(t, recovered.done, "the recovered link to close")
	receive(t, recovered.requestsDone, "the recovered link's last client frame")
	if got := mark(t, db, id); got != final {
		t.Fatalf("The recovered server application committed mark %d, want %d.", got, final)
	}
	trace := recovered.observe()
	if !trace.welcomed || trace.mark != base+1 ||
		!slices.Equal(trace.events, sequenceRange(base+2, final)) ||
		!slices.Equal(trace.acks, sequenceRange(base+2, final)) {
		t.Fatalf("Recovery from mark %d through %d sent events %v and received acknowledgements %v.",
			trace.mark, final, trace.events, trace.acks)
	}
	if !slices.Equal(trace.reports[len(trace.reports)-2:], []reportDelivery{{facts: true}, {harnesses: true}}) {
		t.Fatal("The final drain did not end with one fresh facts report and one fresh inventory report.")
	}
	checkAudit(t, db, base, final)
	if pending := readPending(t, opts); len(pending) != 0 {
		t.Fatal("The stopped client's recovered outbox was not empty.")
	}

	checkScriptedReplay(t, server, db, opts, id, base, final, serverLog)
}

func mark(t *testing.T, db *pgx.Conn, id string) uint64 {
	t.Helper()
	return scalar[uint64](t, db, "SELECT event_mark FROM link_machines WHERE machine_id=$1", id)
}

func installAudit(t *testing.T, db *pgx.Conn, id string, next uint64) {
	t.Helper()
	execute(t, db, `
CREATE TABLE linktest_target (machine_id text PRIMARY KEY, number bigint NOT NULL);
CREATE TABLE linktest_audit (ordinal bigserial PRIMARY KEY, prior bigint NOT NULL);
CREATE FUNCTION linktest_audit_report() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO linktest_audit(prior)
    SELECT lm.event_mark FROM link_machines lm JOIN linktest_target target USING (machine_id)
    WHERE lm.machine_id = NEW.id;
    RETURN NEW;
END $$;
CREATE TRIGGER linktest_audit_report_write
AFTER UPDATE OF facts, harnesses ON machines FOR EACH ROW EXECUTE FUNCTION linktest_audit_report();
CREATE FUNCTION linktest_wait_at_commit() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM linktest_target WHERE machine_id = NEW.machine_id AND number = NEW.event_mark
    ) THEN
        PERFORM pg_advisory_xact_lock(377113::bigint);
    END IF;
    RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER linktest_commit_barrier
AFTER UPDATE ON link_machines DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
WHEN (OLD.event_mark < NEW.event_mark) EXECUTE FUNCTION linktest_wait_at_commit();
`)
	execute(t, db, "INSERT INTO linktest_target VALUES ($1,$2)", id, next)
}

func checkAudit(t *testing.T, db *pgx.Conn, base, through uint64) {
	t.Helper()
	priors := scalar[[]uint64](t, db, "SELECT coalesce(array_agg(prior ORDER BY ordinal), '{}') FROM linktest_audit")
	if want := sequenceRange(base, through-1); !slices.Equal(priors, want) {
		t.Fatalf("The audit read prior marks %v, want %v for one ordered effect per report.", priors, want)
	}
}

func sequenceRange(first, last uint64) []uint64 {
	var numbers []uint64
	for number := first; number <= last; number++ {
		numbers = append(numbers, number)
	}
	return numbers
}

func waitForAck(t *testing.T, leg *relayLeg, number uint64) {
	t.Helper()
	waitFor(t, 10*time.Second, "the expected report acknowledgement", func() bool {
		return slices.Contains(leg.observe().acks, number)
	})
}

type pendingReport struct {
	number uint64
	facts  bool
}

func readPending(t *testing.T, opts clienttest.Options) []pendingReport {
	t.Helper()
	file := filepath.Join(opts.StateDir, "client.db")
	db, err := bolt.Open(file, 0o600, &bolt.Options{ReadOnly: true, Timeout: time.Second})
	if err != nil {
		t.Fatal("Could not open the stopped client's client.db for inspection.")
	}
	defer db.Close()
	var pending []pendingReport
	err = db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("events"))
		if bucket == nil {
			return errors.New("the events bucket is missing")
		}
		return bucket.ForEach(func(key, body []byte) error {
			var event linkv1.Event
			if len(key) != 8 || len(body) < 2 || body[0] != 1 || proto.Unmarshal(body[1:], &event) != nil ||
				event.GetNumber() != 0 || (event.GetFacts() == nil && event.GetHarnessesFound() == nil) {
				return errors.New("the stored report is invalid")
			}
			pending = append(pending, pendingReport{binary.BigEndian.Uint64(key), event.GetFacts() != nil})
			return nil
		})
	})
	if err != nil {
		t.Fatal("Could not read the stopped client's sequenced outbox.")
	}
	return pending
}

func checkPending(t *testing.T, logs *diagnostics, pending []pendingReport, base uint64) {
	t.Helper()
	var numbers []uint64
	for _, report := range pending {
		numbers = append(numbers, report.number)
	}
	buffered := logs.bufferedAfter(base)
	if len(numbers) == 0 || numbers[0] != base+1 || !slices.Equal(numbers, buffered) ||
		!slices.Equal(numbers, sequenceRange(base+1, numbers[len(numbers)-1])) {
		t.Fatalf("The closed outbox held %v after mark %d, and the client recorded buffered numbers %v.", numbers, base, buffered)
	}
}

func checkScriptedReplay(t *testing.T, server *servertest.Server, db *pgx.Conn, opts clienttest.Options, id string, base, current uint64, logs *diagnostics) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	stream := server.Link().Open(ctx)
	defer stream.CloseRequest()
	defer stream.CloseResponse()
	welcome := authenticateScriptedLink(t, stream, server.SigningPublicKey, opts, id, current+1)
	if welcome.GetLastEventNumber() != current {
		t.Fatal("The scripted peer did not see the final committed mark in Welcome.")
	}

	facts := &linkv1.Facts{Os: "Scripted fixture", Cores: 1}
	repeat := &linkv1.Event{Number: current, Report: &linkv1.Event_Facts{Facts: facts}}
	send := func(event *linkv1.Event) (*linkv1.OpenResponse, error) {
		if err := stream.Send(&linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Event{Event: event}}); err != nil {
			t.Fatal("The scripted peer could not send its test report.")
		}
		for {
			frame, err := stream.Receive()
			if err != nil || frame.GetCommand() == nil {
				return frame, err
			}
			wire, marshalErr := proto.Marshal(frame)
			if marshalErr != nil || bytes.Contains(wire, []byte(unknownMarker)) {
				t.Fatal("An unknown report field reached a wire reply.")
			}
			cmd := frame.GetCommand()
			if cmd.GetSetState() == nil {
				t.Fatal("The scripted peer received an unexpected action while waiting for a report acknowledgement.")
			}
			result := &linkv1.CommandResult{
				CommandId: cmd.GetId(), Attempt: cmd.GetAttempt(), Outcome: linkv1.CommandOutcome_COMMAND_OUTCOME_DONE,
			}
			if err := stream.Send(&linkv1.OpenRequest{Kind: &linkv1.OpenRequest_CommandResult{CommandResult: result}}); err != nil {
				t.Fatal("The scripted peer could not acknowledge the stored state notice.")
			}
		}
	}
	frame, err := send(repeat)
	if err != nil || frame.GetEventAck().GetThrough() != current || mark(t, db, id) != current {
		t.Fatal("The scripted repeat did not get the existing mark without applying an effect.")
	}
	checkAudit(t, db, base, current)

	unknown := protowire.AppendTag(nil, 100, protowire.BytesType)
	unknown = protowire.AppendString(unknown, unknownMarker)
	facts.ProtoReflect().SetUnknown(unknown)
	next := &linkv1.Event{Number: current + 1, Report: &linkv1.Event_Facts{Facts: facts}}
	next.ProtoReflect().SetUnknown(unknown)
	frame, err = send(next)
	if err != nil || frame.GetEventAck().GetThrough() != current+1 {
		t.Fatal("The report with an unknown field was not consumed in sequence.")
	}
	reply, err := proto.Marshal(frame)
	if err != nil || bytes.Contains(reply, []byte(unknownMarker)) {
		t.Fatal("An unknown report field reached a wire reply.")
	}
	saved := scalar[[]byte](t, db, "SELECT facts FROM machines WHERE id=$1", id)
	var recorded linkv1.Facts
	if proto.Unmarshal(saved, &recorded) != nil || recorded.GetOs() != facts.GetOs() ||
		bytes.Contains(saved, []byte(unknownMarker)) || len(recorded.ProtoReflect().GetUnknown()) != 0 {
		t.Fatal("The database did not keep the known facts and discard the unknown field.")
	}

	gap := &linkv1.Event{Number: current + 3, Report: &linkv1.Event_Facts{Facts: facts}}
	_, err = send(gap)
	var coded *connect.Error
	if !errors.As(err, &coded) || coded.Code() != connect.CodeFailedPrecondition ||
		coded.Meta().Get("Lgtm-Link-Refusal") != "event_gap" ||
		strings.Contains(coded.Error(), unknownMarker) || mark(t, db, id) != current+1 {
		t.Fatal("The scripted gap did not return event_gap with the committed mark unchanged.")
	}
	checkAudit(t, db, base, current+1)
	logs.mu.Lock()
	marked := logs.marked
	logs.mu.Unlock()
	if marked {
		t.Fatal("An unknown report field reached the server log.")
	}
}

func TestLinkRevisionOneRefusal(t *testing.T) {
	requirePostgres(t)
	testRevisionOneRefusal(t)
}

func testRevisionOneRefusal(t *testing.T) {
	t.Helper()
	server := servertest.Start(t, servertest.Options{Password: fixturePassword})
	opts := newClient(t, new(diagnostics))
	id := join(t, server.Clients(t), opts, server.URL)
	db := openObserver(t, server)
	const priorMark = 5
	execute(t, db, "INSERT INTO link_machines (machine_id, event_mark) VALUES ($1, $2)", id, priorMark)
	priorGeneration := scalar[uint64](t, db, "SELECT stream_generation FROM link_machines WHERE machine_id=$1", id)

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	stream := server.Link().Open(ctx)
	defer stream.CloseRequest()
	defer stream.CloseResponse()
	hello := &linkv1.Hello{MachineId: id, LinkRevision: 1}
	challenge := scriptedChallenge(t, stream, server.SigningPublicKey, hello)
	if err := sendScriptedProof(stream, readMachineKey(t, opts), hello, challenge); err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("The revision 1 peer could not send Proof (%s).", connect.CodeOf(err))
	}
	frame, err := stream.Receive()
	if frame != nil || connect.CodeOf(err) != connect.CodeFailedPrecondition ||
		!slices.Equal(stream.ResponseTrailer().Values("Lgtm-Link-Refusal"), []string{"link_revision_mismatch"}) {
		t.Fatalf("After the signed Challenge the revision 1 stream got another frame or the wrong refusal (%s).", connect.CodeOf(err))
	}
	if mark(t, db, id) != priorMark ||
		scalar[uint64](t, db, "SELECT stream_generation FROM link_machines WHERE machine_id=$1", id) != priorGeneration ||
		scalar[bool](t, db, "SELECT EXISTS (SELECT FROM link_connections WHERE machine_id=$1)", id) ||
		scalar[bool](t, db, "SELECT EXISTS (SELECT FROM link_runs WHERE machine_id=$1)", id) {
		t.Fatal("The refused revision 1 stream changed admission or event state.")
	}
}

func authenticateScriptedLink(t *testing.T, stream *connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse],
	serverKey ed25519.PublicKey, opts clienttest.Options, id string, firstEventNumber uint64,
) *linkv1.Welcome {
	t.Helper()
	hello := &linkv1.Hello{MachineId: id, LinkRevision: 2, FirstEventNumber: firstEventNumber, RunId: newPairedID()}
	submitScriptedHello(t, stream, serverKey, opts, hello)
	frame, err := stream.Receive()
	if err != nil || frame.GetWelcome().GetLinkRevision() != 2 {
		t.Fatalf("The scripted peer did not receive a revision 2 Welcome (%s).", connect.CodeOf(err))
	}
	return frame.GetWelcome()
}

func newPairedID() string {
	var run [16]byte
	binary.BigEndian.PutUint64(run[:8], uint64(time.Now().UnixMilli())<<16)
	rand.Read(run[6:])
	run[6] = 0x70 | run[6]&0x0f
	run[8] = 0x80 | run[8]&0x3f
	return fmt.Sprintf("%x-%x-%x-%x-%x", run[:4], run[4:6], run[6:8], run[8:10], run[10:])
}

func submitScriptedHello(t *testing.T, stream *connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse],
	serverKey ed25519.PublicKey, opts clienttest.Options, hello *linkv1.Hello,
) {
	t.Helper()
	challenge := scriptedChallenge(t, stream, serverKey, hello)
	if err := sendScriptedProof(stream, readMachineKey(t, opts), hello, challenge); err != nil {
		t.Fatal("The scripted peer could not send Proof.")
	}
}

func scriptedChallenge(t *testing.T, stream *connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse],
	serverKey ed25519.PublicKey, hello *linkv1.Hello,
) *linkv1.Challenge {
	t.Helper()
	hello.MachineNonce = make([]byte, 32)
	rand.Read(hello.MachineNonce)
	if err := stream.Send(&linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Hello{Hello: hello}}); err != nil {
		t.Fatal("The scripted peer could not send Hello.")
	}
	frame, err := stream.Receive()
	challenge := frame.GetChallenge()
	if err != nil || len(challenge.GetServerNonce()) != 32 || !ed25519.Verify(serverKey,
		proofText("server", hello.GetMachineId(), hello.GetMachineNonce(), challenge.GetServerNonce()), challenge.GetServerSignature()) {
		t.Fatal("The scripted peer did not receive a verifiable Challenge.")
	}
	return challenge
}

func sendScriptedProof(stream *connect.BidiStreamForClient[linkv1.OpenRequest, linkv1.OpenResponse],
	key ed25519.PrivateKey, hello *linkv1.Hello, challenge *linkv1.Challenge,
) error {
	signature := ed25519.Sign(key, proofText("machine", hello.GetMachineId(), hello.GetMachineNonce(), challenge.GetServerNonce()))
	return stream.Send(&linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Proof{
		Proof: &linkv1.Proof{MachineSignature: signature},
	}})
}

func proofText(role, id string, machineNonce, serverNonce []byte) []byte {
	text := append([]byte("lgtm-link-v1 "+role+"\x00"), byte(len(id)))
	text = append(text, id...)
	text = append(text, machineNonce...)
	return append(text, serverNonce...)
}

func readMachineKey(t *testing.T, opts clienttest.Options) ed25519.PrivateKey {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(opts.StateDir, "machine.key"))
	block, _ := pem.Decode(data)
	if err != nil || block == nil {
		t.Fatal("Could not read the fixture's private signing key.")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	private, ok := key.(ed25519.PrivateKey)
	if err != nil || !ok {
		t.Fatal("Could not decode the fixture's private signing key.")
	}
	return private
}
