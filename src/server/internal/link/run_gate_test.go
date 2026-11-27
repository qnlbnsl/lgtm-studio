package link

import (
	"context"
	"encoding/binary"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func TestRunBarrierDoesNotUseCommandPermit(t *testing.T) {
	t.Run("PendingRun", func(t *testing.T) { testRunBarrierPermits(t, false) })
	t.Run("PendingReconnect", func(t *testing.T) { testRunBarrierPermits(t, true) })
	t.Run("EmptyPrefixAndReadyReconnect", testEmptyRunPrefix)
	t.Run("ClaimFence", testGatedClaim)
	t.Run("SendFence", testGatedSend)
}

type runGatePermitHolds struct {
	releases map[string]<-chan struct{}
	entered  chan string
}

func newRunGatePermitHolds() *runGatePermitHolds {
	return &runGatePermitHolds{releases: make(map[string]<-chan struct{}), entered: make(chan string, 3)}
}

func (h *runGatePermitHolds) beforeSend(ctx context.Context, machineID, _ string) error {
	if release := h.releases[machineID]; release != nil {
		select {
		case h.entered <- machineID:
		case <-ctx.Done():
			return context.Cause(ctx)
		}
		select {
		case <-release:
		case <-ctx.Done():
			return context.Cause(ctx)
		}
	}
	return nil
}

func (h *runGatePermitHolds) fill(t *testing.T, f *handshakeFixture, client linkv1connect.LinkServiceClient) func() {
	t.Helper()
	type permitHolder struct {
		id      string
		row     claimFixtureRow
		release func()
		open    func() *openClientStream
	}
	var holders []permitHolder
	for _, label := range []string{"first permit", "second permit", "third permit"} {
		machine, key := f.register(t, label)
		if err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
			return f.r.SetApproved(ctx, tx, machine.ID, label)
		}); err != nil {
			t.Fatal("The fixture could not approve its independent permit holder.")
		}
		channel, release := storetest.Signal(t)
		h.releases[machine.ID] = channel
		row := queueSendRestart(t, f.presenceFixture, machine.ID, 0)
		holders = append(holders, permitHolder{machine.ID, row, release, func() *openClientStream {
			peer := submitAdmission(t, f, client, machine.ID, key, machine.ID, 1)
			requireAdmissionWelcome(t, peer, 0)
			return peer
		}})
	}
	var firstPeer *openClientStream
	for i, holder := range holders {
		peer := holder.open()
		if got := storetest.Receive(t, "an independent writer holding its command permit", h.entered); got != holder.id {
			t.Fatal("The permit came from the wrong machine.")
		}
		if i == 0 {
			firstPeer = peer
		}
	}
	return func() {
		holders[0].release()
		requireRunGateRestart(t, firstPeer, holders[0].row, 1)
	}
}

func testRunBarrierPermits(t *testing.T, reconnect bool) {
	ackHeld := make(chan struct{}, 1)
	ackRelease, releaseAck := storetest.Signal(t)
	defer releaseAck()
	permits := newRunGatePermitHolds()
	f := newServerStreamFixture(t, handshakeFixtureOptions{seams: Seams{
		BeforeCommandSend: permits.beforeSend,
		BeforeEventAck: func(ctx context.Context, _ string, mark uint64) {
			if mark == 9 {
				ackHeld <- struct{}{}
				select {
				case <-ackRelease:
				case <-ctx.Done():
				}
			}
		},
	}})
	f.svc.streaming.options.Commands = f.state
	client := steadyWire(t, f, streamTestOptions{})
	m, key := f.register(t, "replay target")
	f.approve(t, m.ID)
	seedAdmissionMark(t, f, m.ID, 5)
	runA, runB := fixtureStreamID(t), fixtureStreamID(t)
	a := submitAdmission(t, f, client, m.ID, key, runA, 6)
	requireAdmissionWelcome(t, a, 5)
	finishAdmissionPeer(t, a)

	freePermit := permits.fill(t, f, client)

	first := queueSendRestart(t, f.presenceFixture, m.ID, 0)
	second := queueSendRestart(t, f.presenceFixture, m.ID, 0)
	open := func(mark uint64) *openClientStream {
		t.Helper()
		peer := submitAdmission(t, f, client, m.ID, key, runB, 10)
		requireAdmissionWelcome(t, peer, mark)
		awaitAdmissionPublication(t, f, m.ID)
		return peer
	}
	target := open(5)
	sendHandshake(t, target, eventFrame(factsEvent(6, "Retained event six")))
	requireEventAck(t, target, 6)
	if reconnect {
		target = open(6)
	}
	sendHandshake(t, target, heartbeatFrame(nil, nil))
	requireHeartbeatAck(t, target)
	for _, n := range []uint64{7, 8} {
		sendHandshake(t, target, eventFrame(factsEvent(n, "Retained event")))
		requireEventAck(t, target, n)
	}
	requireAdmissionRun(t, f, m.ID, runB, runA, 10)
	sendHandshake(t, target, eventFrame(factsEvent(9, "Last retained event")))
	storetest.Receive(t, "the final replay acknowledgement reaching its hold", ackHeld)
	used, targetWaiting := runGateSlots(f.svc, m.ID)
	if used != 3 || targetWaiting {
		t.Fatal("The replay writer entered the command permit queue before its final acknowledgement.")
	}
	for _, row := range []claimFixtureRow{first, second} {
		requireClaimStored(t, f.presenceFixture, row.id, "", 1)
	}

	releaseAck()
	requireEventAck(t, target, 9)
	freePermit()
	requireRunGateRestart(t, target, first, 1)
	requireRunGateRestart(t, target, second, 1)
	requireAdmissionRun(t, f, m.ID, runB, runB, 10)
}

func runGateSlots(s *Service, machineID string) (used int, waiting bool) {
	s.commandSlots.mu.Lock()
	defer s.commandSlots.mu.Unlock()
	for entry := s.commandSlots.waiters.Front(); entry != nil; entry = entry.Next() {
		if entry.Value.(*commandSession).handle.machineID == machineID {
			waiting = true
			break
		}
	}
	return s.commandSlots.used, waiting
}

func testEmptyRunPrefix(t *testing.T) {
	permits := newRunGatePermitHolds()
	f := newServerStreamFixture(t, handshakeFixtureOptions{seams: Seams{BeforeCommandSend: permits.beforeSend}})
	f.svc.streaming.options.Commands = f.state
	m, key := f.register(t, "empty replay prefix")
	f.approve(t, m.ID)
	seedAdmissionMark(t, f, m.ID, 5)
	run := fixtureStreamID(t)
	row := queueSendRestart(t, f.presenceFixture, m.ID, 0)
	client := steadyWire(t, f, streamTestOptions{})
	freePermit := permits.fill(t, f, client)
	peer := submitAdmission(t, f, client, m.ID, key, run, 6)
	requireAdmissionWelcome(t, peer, 5)
	storetest.Poll(t, "the empty-prefix writer entering the saturated permit queue after Welcome", func() bool {
		used, waiting := runGateSlots(f.svc, m.ID)
		return used == 3 && waiting
	})
	requireClaimStored(t, f.presenceFixture, row.id, "", 1)
	freePermit()
	requireRunGateRestart(t, peer, row, 1)
	sendHandshake(t, peer, eventFrame(factsEvent(6, "Event after the empty prefix")))
	requireEventAck(t, peer, 6)

	reconnect := submitAdmission(t, f, client, m.ID, key, run, 6)
	requireAdmissionWelcome(t, reconnect, 6)
	requireRunGateRestart(t, reconnect, row, 2)
	requireAdmissionRun(t, f, m.ID, run, run, 6)
}

func requireRunGateRestart(t *testing.T, peer *openClientStream, row claimFixtureRow, attempt uint32) {
	t.Helper()
	frame, err := peer.Receive()
	if err != nil || frame.GetCommand().GetRestart() == nil {
		t.Fatal("The ready stream did not receive the next stored restart.")
	}
	row.requireFrame(t, frame.GetCommand(), attempt)
}

func pendingCommandFixture(t *testing.T) (*presenceFixture, string, string, claimFixtureRow) {
	t.Helper()
	f := newServerStreamFixture(t, handshakeFixtureOptions{})
	m, _ := f.register(t, "pending command fence")
	f.approve(t, m.ID)
	seedAdmissionMark(t, f, m.ID, 5)
	stream := fixtureStreamID(t)
	run := fixtureStreamID(t)
	if _, err := f.state.BecomeCurrent(t.Context(), m.ID, stream, presenceAddress, RunAnnouncement{ID: run, FirstEventNumber: 10}); err != nil {
		t.Fatal("The pending command fixture could not admit its stream.")
	}
	requireAdmissionRun(t, f, m.ID, run, "", 10)
	return f.presenceFixture, m.ID, stream, queueSendRestart(t, f.presenceFixture, m.ID, 0)
}

func testGatedClaim(t *testing.T) {
	f, machine, stream, row := pendingCommandFixture(t)
	want := commandCursor{lastSeq: 71, scanHead: fixtureStreamID(t), deferredScan: 73}
	cursor := want
	command, more, err := f.state.claimNext(t.Context(), machine, stream, &cursor)
	if err != nil || command != nil || more {
		t.Error("The claim fence did not return an idle turn for the pending stream.")
	}
	if cursor != want {
		t.Error("The claim fence changed the pending stream's sentinel cursor.")
	}
	requireClaimStored(t, f, row.id, "", 1)
}

func testGatedSend(t *testing.T) {
	f, machine, stream, row := pendingCommandFixture(t)
	storetest.Exec(t, f.obs, "UPDATE link_commands SET sent_stream_id = $2, attempt = 2 WHERE id = $1", row.id, stream)
	command := proto.CloneOf(row.action)
	command.Id, command.Attempt = row.id, 2
	writes := 0
	sent, err := f.state.sendClaimed(t.Context(), machine, stream, command, func(context.Context) error {
		writes++
		return nil
	})
	if err != nil || sent || writes != 0 {
		t.Error("The send fence let a current assignment reach the transport while the run was pending.")
	}
	requireDispatch(t, f, row, stream, 2, false)
}

func TestWaitingReplayAcknowledgesBeforeStanding(t *testing.T) {
	t.Run("ApproveAtConsumedEvent", testWaitingReplayApproval)
	t.Run("Gap", testWaitingReplayGap)
}

type runGateAckWriter struct {
	http.ResponseWriter
	flushed *atomic.Bool
	size    int
	lastAck bool
}

func (w *runGateAckWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *runGateAckWriter) Write(p []byte) (int, error) {
	// The Connect stream writes the uncompressed envelope prefix and its payload separately.
	if w.size != 0 {
		frame := new(linkv1.OpenResponse)
		w.lastAck = len(p) == w.size && proto.Unmarshal(p, frame) == nil && frame.GetEventAck().GetThrough() == 7
		w.size = 0
	} else if len(p) == 5 && p[0] == 0 {
		w.size = int(binary.BigEndian.Uint32(p[1:]))
	}
	return w.ResponseWriter.Write(p)
}

func (w *runGateAckWriter) FlushError() error {
	err := http.NewResponseController(w.ResponseWriter).Flush()
	if err == nil && w.lastAck {
		w.flushed.Store(true)
	}
	w.lastAck = false
	return err
}

func testWaitingReplayApproval(t *testing.T) {
	var flushed atomic.Bool
	var earlyClaimReads atomic.Int32
	advances := make(chan uint64, 8)
	release, resume := storetest.Signal(t)
	defer resume()
	var claims atomic.Int32
	logs := new(handshakeLog)
	f := newServerStreamFixture(t, handshakeFixtureOptions{
		logger: slog.New(slog.NewJSONHandler(logs, nil)),
		configure: func(cfg *pgxpool.Config) {
			cfg.ConnConfig.Tracer = handshakeQueryTrace(func(ctx context.Context, _ *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
				sql := strings.ToLower(q.SQL)
				if !flushed.Load() && strings.Contains(sql, "from link_commands") && strings.Contains(sql, "order by seq") {
					earlyClaimReads.Add(1)
				}
				return ctx
			})
		},
		seams: Seams{
			BeforeEventAck: func(ctx context.Context, _ string, mark uint64) {
				advances <- mark
				if mark == 7 {
					select {
					case <-release:
					case <-ctx.Done():
					}
				}
			},
			BeforeCommandSend: func(context.Context, string, string) error {
				claims.Add(1)
				return nil
			},
		},
	})
	f.svc.streaming.options.Commands = f.state
	m, key := f.register(t, "waiting replay")
	machinesClient := newRecoveryMachinesClient(t, f)
	seedAdmissionMark(t, f, m.ID, 5)
	installReportAudit(t, f.presenceFixture)
	run := fixtureStreamID(t)
	client := steadyWire(t, f, streamTestOptions{
		clientOptions: []connect.ClientOption{connect.WithAcceptCompression("gzip", nil, nil)},
		wrap: func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(&runGateAckWriter{ResponseWriter: w, flushed: &flushed}, r)
			})
		},
	})
	peer := submitAdmission(t, f, client, m.ID, key, run, 8)
	if w := requireWelcome(t, peer); w.GetLastEventNumber() != 5 || w.GetState().GetApproval() != linkv1.Approval_APPROVAL_WAITING {
		t.Fatal("Welcome did not describe the waiting machine and its replay starting mark.")
	}

	sendHandshake(t, peer, eventFrame(factsEvent(6, "Waiting replay facts")))
	requireEventAck(t, peer, 6)
	if mark := storetest.Receive(t, "the applied Facts acknowledgement observation", advances); mark != 6 {
		t.Fatal("The observer did not see the committed Facts mark.")
	}
	requireRunGateFacts(t, f, m.ID, "Waiting replay facts")
	sendHandshake(t, peer, eventFrame(factsEvent(6, "Repeat must not apply")))
	requireEventAck(t, peer, 6)

	sendHandshake(t, peer, eventFrame(harnessEvent(7, &linkv1.Harness{Name: "codex", Version: proto.String("1")})))
	if mark := storetest.Receive(t, "the consumed inventory acknowledgement observation", advances); mark != 7 {
		t.Fatal("The observer received a repeat or failed to observe the consumed inventory event.")
	}
	if eventMark(t, f.presenceFixture, m.ID) != 7 {
		t.Error("The consumed inventory report did not commit its mark before the acknowledgement observation.")
	}
	if !storetest.Scalar[bool](t, f.obs, "SELECT harnesses IS NULL AND NOT harnesses_scanned FROM machines WHERE id = $1", m.ID) {
		t.Error("Waiting replay changed the machine's inventory.")
	}
	reportAudit(t, f.presenceFixture, 5)
	watch := machinesClient.watch(t)
	waiting := recoverySnapshot(t, watch, m.ID)
	closeRecoveryWatch(t, watch)
	if waiting.GetState() != webv1.ClientState_CLIENT_STATE_WAITING_FOR_APPROVAL ||
		waiting.GetOs() != "Waiting replay facts" || waiting.GetHarnessesScanned() || len(waiting.GetHarnesses()) != 0 {
		t.Error("The Machines watch did not show the waiting Facts and unchanged inventory.")
	}
	owner := machines.NewService(f.s, nil, f.r, f.tokens).WithLink(f.state).WithCommands(f.state)
	reply, err := owner.ApproveClient(t.Context(), connect.NewRequest(&webv1.ApproveClientRequest{Id: m.ID, Name: "Approved during replay"}))
	if err != nil || reply.Msg.GetClient().GetId() != m.ID {
		t.Fatal("The fixture could not approve the machine while its final acknowledgement was held.")
	}
	if claims.Load() != 0 || !storetest.Scalar[bool](t, f.obs, `
		SELECT count(*) = 1 AND bool_and(sent_stream_id IS NULL AND attempt = 1 AND NOT dispatched)
		FROM link_commands WHERE machine_id = $1 AND action = 'set_state'`, m.ID) {
		t.Error("Approval claimed its standing notice while the final acknowledgement was held.")
	}
	resume()
	ack := receiveOpenFrame(peer)
	if ack.err != nil || ack.frame.GetEventAck().GetThrough() != 7 {
		t.Fatal("The final replay acknowledgement did not precede the approved standing notice.")
	}
	frame := receiveOpenFrame(peer)
	standing := frame.frame.GetCommand().GetSetState().GetState()
	if frame.err != nil || standing.GetApproval() != linkv1.Approval_APPROVAL_APPROVED || standing.GetName() != "Approved during replay" {
		t.Fatal("The first frame after the final replay acknowledgement was not the approved standing notice.")
	}
	if !flushed.Load() {
		t.Fatal("The fixture did not observe the final replay acknowledgement flush.")
	}
	if earlyClaimReads.Load() != 0 {
		t.Error("Waiting replay entered the claim path before the final acknowledgement flushed.")
	}

	sendHandshake(t, peer, eventFrame(harnessEvent(7, &linkv1.Harness{Name: "codex", Version: proto.String("2")})))
	requireEventAck(t, peer, 7)
	sendHandshake(t, peer, heartbeatFrame(nil, nil))
	requireHeartbeatAck(t, peer)
	select {
	case <-advances:
		t.Fatal("A replay repeat was observed as a new committed advance.")
	default:
	}
	reportAudit(t, f.presenceFixture, 5)
	appliedDiagnostics(t, logs, m.ID, 6, 7)
}

func testWaitingReplayGap(t *testing.T) {
	var observes atomic.Int32
	f := newServerStreamFixture(t, handshakeFixtureOptions{seams: Seams{
		BeforeEventAck: func(context.Context, string, uint64) { observes.Add(1) },
	}})
	m, key := f.register(t, "waiting replay gap")
	seedAdmissionMark(t, f, m.ID, 5)
	peer := submitAdmission(t, f, steadyWire(t, f, streamTestOptions{}), m.ID, key, fixtureStreamID(t), 8)
	requireAdmissionWelcome(t, peer, 5)
	sendHandshake(t, peer, eventFrame(factsEvent(7, "Skipped an event")))
	requireOpenEnd(t, peer, connect.CodeFailedPrecondition, ErrEventGap.Error(), "event_gap")
	if eventMark(t, f.presenceFixture, m.ID) != 5 || observes.Load() != 0 {
		t.Fatal("The waiting replay gap advanced the mark or reached the acknowledgement observer.")
	}
}

func requireRunGateFacts(t *testing.T, f *handshakeFixture, machineID, want string) {
	t.Helper()
	raw := storetest.Scalar[[]byte](t, f.obs, "SELECT facts FROM machines WHERE id = $1", machineID)
	facts := new(linkv1.Facts)
	if err := proto.Unmarshal(raw, facts); err != nil || facts.GetOs() != want {
		t.Fatal("The independent reader did not see the expected Facts report.")
	}
}

func TestReadyRunNoticeIsTransactional(t *testing.T) {
	t.Run("FactsAndReadyRollback", testRunReadyRollback)
	t.Run("LatestAdmittedRun", testLatestAdmittedRun)
	t.Run("AdmissionOrder", testRunReadyAdmission)
	t.Run("AdmissionEnqueuesBehindBarrier", testEnqueueWhileRunPending)
	t.Run("ReplacementWhileAckHeld", func(t *testing.T) { testStaleReplayAck(t, false) })
	t.Run("RemovalWhileAckHeld", func(t *testing.T) { testStaleReplayAck(t, true) })
	t.Run("PrecommitFaultAndReplay", testReplayPrecommitFault)
}

type readyRunObservation struct {
	change        RunChange
	facts         string
	mark, replay  int64
	admittedRunID string
	readyRunID    string
}

func testRunReadyRollback(t *testing.T) {
	var failHook atomic.Bool
	var observedAcks atomic.Int32
	seen := make(chan readyRunObservation, 8)
	f := newServerStreamFixture(t, handshakeFixtureOptions{seams: Seams{
		BeforeEventAck: func(context.Context, string, uint64) { observedAcks.Add(1) },
	}})
	m, key := f.register(t, "ready callback rollback")
	f.approve(t, m.ID)
	seedAdmissionMark(t, f, m.ID, 5)
	runA, runB := fixtureStreamID(t), fixtureStreamID(t)
	storetest.Exec(t, f.obs, "CREATE TABLE fixture_ready_run (run_id text PRIMARY KEY)")
	if err := f.state.RegisterPart(PartHooks{RunChanged: func(ctx context.Context, tx *store.Tx, change RunChange) error {
		if change.RunID == runA {
			return nil
		}
		o := readyRunObservation{change: change}
		var raw []byte
		err := tx.QueryRow(ctx, `
			SELECT machine.facts, m.event_mark, r.replay_through, m.admitted_run_id, coalesce(m.ready_run_id, '')
			FROM link_machines m JOIN link_runs r ON r.machine_id = m.machine_id AND r.run_id = $2
			JOIN machines machine ON machine.id = m.machine_id WHERE m.machine_id = $1`,
			change.MachineID, change.RunID).Scan(&raw, &o.mark, &o.replay, &o.admittedRunID, &o.readyRunID)
		if err != nil {
			return err
		}
		facts := new(linkv1.Facts)
		if err := proto.Unmarshal(raw, facts); err != nil {
			return err
		}
		o.facts = facts.GetOs()
		if _, err := tx.Exec(ctx, "INSERT INTO fixture_ready_run (run_id) VALUES ($1)", change.RunID); err != nil {
			return err
		}
		seen <- o
		if failHook.Load() {
			return errors.New("The fixture stopped the readiness write.")
		}
		return nil
	}}); err != nil {
		t.Fatal("The fixture could not register its ready-run callback.")
	}
	client := steadyWire(t, f, streamTestOptions{})
	a := submitAdmission(t, f, client, m.ID, key, runA, 6)
	requireAdmissionWelcome(t, a, 5)
	sendHandshake(t, a, eventFrame(factsEvent(6, "Prior facts")))
	requireEventAck(t, a, 6)
	finishAdmissionPeer(t, a)

	open := func(mark uint64) *openClientStream {
		t.Helper()
		peer := submitAdmission(t, f, client, m.ID, key, runB, 8)
		requireAdmissionWelcome(t, peer, mark)
		awaitAdmissionPublication(t, f, m.ID)
		return peer
	}
	want := readyRunObservation{
		change: RunChange{MachineID: m.ID, RunID: runB, PreviousRunID: runA, FirstEventNumber: 8},
		facts:  "Facts in final replay event", mark: 7, replay: 7, admittedRunID: runB, readyRunID: runB,
	}
	requireCallback := func() {
		t.Helper()
		select {
		case got := <-seen:
			if got != want {
				t.Error("The ready callback did not see the new Facts, mark and admitted run in its write.")
			}
		default:
			t.Error("The event transaction did not call the ready hook.")
		}
	}
	peer := open(6)
	head := f.head(t)
	fault := factsEvent(7, want.facts)
	failHook.Store(true)
	sendHandshake(t, peer, eventFrame(fault))
	rejected := receiveOpenFrame(peer)
	requireCallback()
	if rejected.frame != nil || connect.CodeOf(rejected.err) != connect.CodeUnavailable {
		t.Error("A ready callback failure did not end Open without acknowledging the final event.")
	}
	if eventMark(t, f.presenceFixture, m.ID) != 6 || f.head(t) != head || observedAcks.Load() != 1 ||
		storetest.Scalar[int64](t, f.obs, "SELECT count(*) FROM fixture_ready_run") != 0 {
		t.Fatal("A failed ready callback advanced the mark, inserted its sentinel, published feed or reached the acknowledgement observer.")
	}
	requireRunGateFacts(t, f, m.ID, "Prior facts")
	requireAdmissionRun(t, f, m.ID, runB, runA, 8)

	failHook.Store(false)
	peer = open(6)
	head = f.head(t)
	sendHandshake(t, peer, eventFrame(fault))
	requireEventAck(t, peer, 7)
	requireCallback()
	requireRunGateFacts(t, f, m.ID, want.facts)
	requireAdmissionRun(t, f, m.ID, runB, runB, 8)
	if eventMark(t, f.presenceFixture, m.ID) != 7 {
		t.Error("The mark was not durable when the replay acknowledgement arrived.")
	}
	f.requireSaved(t, head, m.ID)
	sendHandshake(t, peer, eventFrame(fault))
	requireEventAck(t, peer, 7)
	if got := storetest.Rows[string](t, f.obs, "SELECT run_id FROM fixture_ready_run"); !slices.Equal(got, []string{runB}) {
		t.Error("The replay and repeat did not commit exactly one run transition.")
	}
	select {
	case <-seen:
		t.Error("The repeat called the ready hook again.")
	default:
	}
}

func testLatestAdmittedRun(t *testing.T) {
	f := newServerStreamFixture(t, handshakeFixtureOptions{})
	m, _ := f.register(t, "latest pending run")
	seedAdmissionMark(t, f, m.ID, 5)
	oldStream, currentStream := fixtureStreamID(t), fixtureStreamID(t)
	oldRun, currentRun := fixtureStreamID(t), fixtureStreamID(t)
	for _, admission := range []struct{ stream, run string }{{oldStream, oldRun}, {currentStream, currentRun}} {
		pair := RunAnnouncement{ID: admission.run, FirstEventNumber: 7}
		if _, err := f.state.BecomeCurrent(t.Context(), m.ID, admission.stream, presenceAddress, pair); err != nil {
			t.Fatal("The fixture could not admit one of the equal-boundary pending runs.")
		}
	}
	report := factsEvent(6, "Final event shared by two announced boundaries")
	result, err := f.state.ApplyEvent(t.Context(), m.ID, oldStream, report)
	if !errors.Is(err, ErrNotCurrent) || result != (EventResult{}) || eventMark(t, f.presenceFixture, m.ID) != 5 {
		t.Fatal("The superseded pending stream advanced the mark or reached readiness.")
	}
	requireAdmissionRun(t, f, m.ID, currentRun, "", 7)
	requireEvent(t, f.presenceFixture, m.ID, currentStream, report, 6, true)
	requireAdmissionHistory(t, f, m.ID, 2)
	requireAdmissionRun(t, f, m.ID, currentRun, currentRun, 7)
}

func testRunReadyAdmission(t *testing.T) {
	f := newServerStreamFixture(t, handshakeFixtureOptions{})
	m, key := f.register(t, "ready on admission")
	seedAdmissionMark(t, f, m.ID, 5)
	run := fixtureStreamID(t)
	storetest.Exec(t, f.obs, "CREATE TABLE fixture_gate_hooks (seq bigint GENERATED ALWAYS AS IDENTITY, step text NOT NULL)")
	if err := f.state.RegisterPart(PartHooks{
		StreamAdmitted: func(ctx context.Context, tx *store.Tx, admission Admission) error {
			if !admission.Ready || admission.RunID != run || admission.FirstEventNumber != 6 || admission.MachineID != m.ID {
				return errors.New("The fixture saw the wrong empty-prefix admission.")
			}
			_, err := tx.Exec(ctx, "INSERT INTO fixture_gate_hooks (step) VALUES ('admitted')")
			return err
		},
		RunChanged: func(ctx context.Context, tx *store.Tx, change RunChange) error {
			if change != (RunChange{MachineID: m.ID, RunID: run, FirstEventNumber: 6}) {
				return errors.New("The fixture saw the wrong immediate run change.")
			}
			_, err := tx.Exec(ctx, "INSERT INTO fixture_gate_hooks (step) VALUES ('ready')")
			return err
		},
	}); err != nil {
		t.Fatal("The fixture could not register the admission callbacks.")
	}
	client := steadyWire(t, f, streamTestOptions{})
	peer := submitAdmission(t, f, client, m.ID, key, run, 6)
	requireAdmissionWelcome(t, peer, 5)
	finishAdmissionPeer(t, peer)
	peer = submitAdmission(t, f, client, m.ID, key, run, 6)
	requireAdmissionWelcome(t, peer, 5)
	if got := storetest.Rows[string](t, f.obs, "SELECT step FROM fixture_gate_hooks ORDER BY seq"); !slices.Equal(got, []string{"admitted", "ready", "admitted"}) {
		t.Error("The immediate ready callback did not follow the first admission exactly once.")
	}
	if !storetest.Scalar[bool](t, f.obs, `SELECT count(*) = 2 AND count(DISTINCT xmin::text) = 1
		FROM (SELECT xmin FROM fixture_gate_hooks ORDER BY seq LIMIT 2) first_admission`) {
		t.Error("The first admission and the ready callback did not commit in the same transaction.")
	}
}

func testEnqueueWhileRunPending(t *testing.T) {
	finalHeld := make(chan struct{}, 1)
	release, resume := storetest.Signal(t)
	defer resume()
	f := newServerStreamFixture(t, handshakeFixtureOptions{seams: Seams{
		BeforeEventAck: func(ctx context.Context, _ string, mark uint64) {
			if mark == 6 {
				finalHeld <- struct{}{}
				select {
				case <-release:
				case <-ctx.Done():
				}
			}
		},
	}})
	f.svc.streaming.options.Commands = f.state
	m, key := f.register(t, "admission enqueues during replay")
	f.approve(t, m.ID)
	seedAdmissionMark(t, f, m.ID, 5)
	run := fixtureStreamID(t)
	enqueued := make(chan string, 1)
	if err := f.state.RegisterPart(PartHooks{StreamAdmitted: func(ctx context.Context, tx *store.Tx, admission Admission) error {
		if admission.Ready || admission.RunID != run {
			return errors.New("The fixture saw the wrong pending admission.")
		}
		id, err := enqueuePartRestart(ctx, tx, f.state, admission.MachineID)
		if err == nil {
			enqueued <- id
		}
		return err
	}}); err != nil {
		t.Fatal("The fixture could not register the enqueueing admission callback.")
	}
	peer := submitAdmission(t, f, steadyWire(t, f, streamTestOptions{}), m.ID, key, run, 7)
	requireAdmissionWelcome(t, peer, 5)
	id := storetest.Receive(t, "the admission callback enqueueing a restart", enqueued)
	sendHandshake(t, peer, heartbeatFrame(nil, nil))
	requireHeartbeatAck(t, peer)
	sendHandshake(t, peer, eventFrame(factsEvent(6, "The final retained Facts")))
	storetest.Receive(t, "the committed event reaching its held acknowledgement", finalHeld)
	requireClaimStored(t, f.presenceFixture, id, "", 1)
	resume()
	requireEventAck(t, peer, 6)
	frame := receiveOpenFrame(peer)
	if command := frame.frame.GetCommand(); frame.err != nil || command.GetRestart() == nil || command.GetId() != id || command.GetAttempt() != 1 {
		t.Fatal("The admission callback's restart did not follow the final replay acknowledgement.")
	}
}

func testStaleReplayAck(t *testing.T, removed bool) {
	held := make(chan struct{}, 1)
	release, resume := storetest.Signal(t)
	defer resume()
	f := newServerStreamFixture(t, handshakeFixtureOptions{seams: Seams{
		BeforeEventAck: func(ctx context.Context, _ string, mark uint64) {
			if mark == 32 {
				held <- struct{}{}
				select {
				case <-release:
				case <-ctx.Done():
				}
			}
		},
	}})
	f.svc.streaming.options.Commands = f.state
	m, key := f.register(t, "obsolete replay writer")
	f.approve(t, m.ID)
	seedAdmissionMark(t, f, m.ID, 31)
	installReportAudit(t, f.presenceFixture)
	run := fixtureStreamID(t)
	client := steadyWire(t, f, streamTestOptions{})
	peer := submitAdmission(t, f, client, m.ID, key, run, 33)
	requireAdmissionWelcome(t, peer, 31)
	awaitAdmissionPublication(t, f, m.ID)
	originalStream := f.current(t, m.ID).StreamID
	sendHandshake(t, peer, eventFrame(factsEvent(32, "Committed replay")))
	storetest.Receive(t, "the final replay acknowledgement reaching its hold", held)
	if eventMark(t, f.presenceFixture, m.ID) != 32 {
		t.Fatal("The held acknowledgement did not follow a committed event.")
	}
	row := queueSendRestart(t, f.presenceFixture, m.ID, 0)
	// The durable identity changes while the original handle is still live.
	if removed {
		owner := machines.NewService(f.s, nil, f.r, f.tokens).WithLink(f.state)
		if _, err := owner.RemoveClient(t.Context(), connect.NewRequest(&webv1.RemoveClientRequest{Id: m.ID})); err != nil {
			t.Fatal("The fixture could not remove the machine while holding its acknowledgement.")
		}
	} else {
		if _, err := f.state.BecomeCurrent(t.Context(), m.ID, fixtureStreamID(t), presenceAddress, RunAnnouncement{ID: run, FirstEventNumber: 33}); err != nil {
			t.Fatal("The fixture could not replace the durable stream while holding its acknowledgement.")
		}
	}
	if !f.hub.Holds(m.ID, originalStream) {
		t.Fatal("The fixture lost its original handle before testing the durable fence.")
	}
	resume()
	if removed {
		requireOpenEnd(t, peer, connect.CodeFailedPrecondition, openUnknown, "unknown_machine")
		if !storetest.Scalar[bool](t, f.obs, "SELECT outcome = 'removed' AND sent_stream_id IS NULL AND NOT dispatched FROM link_commands WHERE id = $1", row.id) {
			t.Error("The removed stream claimed or dispatched the queued restart.")
		}
		reportAudit(t, f.presenceFixture, 31)
		return
	}
	requireOpenEnd(t, peer, connect.CodeAborted, openReplaced, "")
	requireClaimStored(t, f.presenceFixture, row.id, "", 1)
	current := submitAdmission(t, f, client, m.ID, key, run, 33)
	requireAdmissionWelcome(t, current, 32)
	requireRunGateRestart(t, current, row, 1)
	sendHandshake(t, current, eventFrame(factsEvent(33, "First current-run report")))
	requireEventAck(t, current, 33)
	reportAudit(t, f.presenceFixture, 31, 32)
}

func testReplayPrecommitFault(t *testing.T) {
	var inject atomic.Bool
	var observedAcks atomic.Int32
	inject.Store(true)
	f := newServerStreamFixture(t, handshakeFixtureOptions{seams: Seams{
		BeforeEventCommit: func(_ context.Context, _ string, number uint64) error {
			if number == 32 && inject.Load() {
				return errors.New("The fixture stopped the event before commit.")
			}
			return nil
		},
		BeforeEventAck: func(context.Context, string, uint64) { observedAcks.Add(1) },
	}})
	f.svc.streaming.options.Commands = f.state
	m, key := f.register(t, "event commit fault during replay")
	f.approve(t, m.ID)
	seedAdmissionMark(t, f, m.ID, 31)
	installReportAudit(t, f.presenceFixture)
	run := fixtureStreamID(t)
	row := queueSendRestart(t, f.presenceFixture, m.ID, 0)
	client := steadyWire(t, f, streamTestOptions{})
	open := func() *openClientStream {
		t.Helper()
		peer := submitAdmission(t, f, client, m.ID, key, run, 33)
		requireAdmissionWelcome(t, peer, 31)
		return peer
	}
	report := factsEvent(32, "Retried replay")
	peer := open()
	head := f.head(t)
	sendHandshake(t, peer, eventFrame(report))
	requireOpenEnd(t, peer, connect.CodeUnavailable, openDatabase, "")
	if f.head(t) != head || eventMark(t, f.presenceFixture, m.ID) != 31 || observedAcks.Load() != 0 {
		t.Error("The failed event changed the feed or mark or reached the acknowledgement observer.")
	}
	reportAudit(t, f.presenceFixture)
	requireClaimStored(t, f.presenceFixture, row.id, "", 1)
	requireAdmissionRun(t, f, m.ID, run, "", 33)
	inject.Store(false)
	peer = open()
	sendHandshake(t, peer, eventFrame(report))
	requireEventAck(t, peer, 32)
	requireRunGateRestart(t, peer, row, 1)
	sendHandshake(t, peer, eventFrame(factsEvent(33, "Report after replay retry")))
	requireEventAck(t, peer, 33)
	reportAudit(t, f.presenceFixture, 31, 32)
	requireAdmissionRun(t, f, m.ID, run, run, 33)
}
