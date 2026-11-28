package link

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/api"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func TestRecoveryWarningFollowsLaterReadyAdmission(t *testing.T) {
	t.Run("LiveWatchTrafficRestartAndRemoval", func(t *testing.T) {
		f := newServerStreamFixture(t, handshakeFixtureOptions{})
		m, key := f.register(t, "recovery incumbent")
		f.approve(t, m.ID)
		seedAdmissionMark(t, f, m.ID, 10)
		client := steadyWire(t, f, streamTestOptions{timeout: storetest.ObservationTimeout})
		incumbent := submitAdmission(t, f, client, m.ID, key, fixtureStreamID(t), 11)
		requireAdmissionWelcome(t, incumbent, 10)
		recoveryHeartbeat(t, incumbent)
		current := f.current(t, m.ID)

		web := newRecoveryMachinesClient(t, f)
		watch := web.watch(t)
		requireRecoveryClient(t, recoverySnapshot(t, watch, m.ID), m.ID, webv1.ClientState_CLIENT_STATE_JOINING, false)
		head := f.head(t)
		requireOpenEnd(t, submitAdmission(t, f, client, m.ID, key, fixtureStreamID(t), 9), connect.CodeInvalidArgument, openInvalid, "")
		requireRecoveryFence(t, f, m.ID, current.Generation)
		f.requireSaved(t, head, m.ID)
		requireRecoveryClient(t, recoveryWatchAt(t, watch, f.head(t)).GetSaved(), m.ID, webv1.ClientState_CLIENT_STATE_JOINING, true)
		requireRecoveryIncumbent(t, f, m.ID, current)

		head = f.head(t)
		requireOpenEnd(t, submitAdmission(t, f, client, m.ID, key, fixtureStreamID(t), 10), connect.CodeInvalidArgument, openInvalid, "")
		f.requireSaved(t, head)

		recoveryHeartbeat(t, incumbent)
		requireRecoveryFence(t, f, m.ID, current.Generation)
		requireRecoveryClient(t, recoveryWatchAt(t, watch, f.head(t)).GetSaved(), m.ID, webv1.ClientState_CLIENT_STATE_JOINING, true)

		sendHandshake(t, incumbent, eventFrame(factsEvent(11, "recovery fixture facts")))
		requireEventAck(t, incumbent, 11)
		requireRecoveryFence(t, f, m.ID, current.Generation)
		applied := recoveryWatchAt(t, watch, f.head(t)).GetSaved()
		requireRecoveryClient(t, applied, m.ID, webv1.ClientState_CLIENT_STATE_JOINING, true)
		if applied.GetOs() != "recovery fixture facts" {
			t.Fatal("The incumbent's valid Facts did not reach the Machines watch.")
		}

		sendHandshake(t, incumbent, eventFrame(&linkv1.Event{Number: 12}))
		requireEventAck(t, incumbent, 12)
		requireRecoveryFence(t, f, m.ID, current.Generation)
		consumed := recoveryWatchAt(t, watch, f.head(t)).GetSaved()
		requireRecoveryClient(t, consumed, m.ID, webv1.ClientState_CLIENT_STATE_JOINING, true)
		if consumed.GetOs() != "recovery fixture facts" {
			t.Fatal("The consumed event changed the incumbent's saved Facts.")
		}

		other, otherKey := f.register(t, "unrelated peer")
		unrelated := submitAdmission(t, f, client, other.ID, otherKey, fixtureStreamID(t), 1)
		requireAdmissionWelcome(t, unrelated, 0)
		awaitAdmissionPublication(t, f, other.ID)
		snapshot := web.watch(t)
		requireRecoveryClient(t, recoverySnapshot(t, snapshot, m.ID), m.ID, webv1.ClientState_CLIENT_STATE_JOINING, true)
		waiting := webv1.ClientState_CLIENT_STATE_WAITING_FOR_APPROVAL
		requireRecoveryClient(t, recoverySnapshotClient(t, snapshot.Msg(), other.ID), other.ID, waiting, false)
		closeRecoveryWatch(t, snapshot)

		recoveryHeartbeat(t, unrelated)
		requireRecoveryClient(t, recoveryWatchAt(t, watch, f.head(t)).GetSaved(), other.ID, waiting, false)
		requireRecoveryFence(t, f, m.ID, current.Generation)
		finishAdmissionPeer(t, unrelated)

		disconnectRecoveryPeer(t, f, incumbent, m.ID, current.StreamID)
		offline := recoveryWatchAt(t, watch, f.head(t)).GetSaved()
		closeRecoveryWatch(t, watch)
		web.server.Close()

		f = restartAdmissionFixture(t, f)
		web = newRecoveryMachinesClient(t, f)
		watch = web.watch(t)
		requireRecoveryOffline(t, recoverySnapshot(t, watch, m.ID), m.ID)
		requireRecoveryOffline(t, offline, m.ID)
		if err := web.remove(t, m.ID); err != nil {
			t.Fatalf("The authenticated removal failed with code %v.", connect.CodeOf(err))
		}
		if recoveryWatchAt(t, watch, f.head(t)).GetRemovedId() != m.ID {
			t.Fatal("The Machines watch did not publish the removed machine identifier.")
		}
		if storetest.Scalar[bool](t, f.obs, "SELECT EXISTS (SELECT FROM link_machines WHERE machine_id = $1)", m.ID) {
			t.Fatal("Machine removal left the Link row carrying the recovery warning.")
		}
	})

	t.Run("WaitingMachineWithoutConnection", func(t *testing.T) {
		f := newServerStreamFixture(t, handshakeFixtureOptions{})
		m, key := f.register(t, "waiting recovery peer")
		seedAdmissionMark(t, f, m.ID, 10)
		web := newRecoveryMachinesClient(t, f)
		watch := web.watch(t)
		requireRecoveryClient(t, recoverySnapshot(t, watch, m.ID), m.ID, webv1.ClientState_CLIENT_STATE_WAITING_FOR_APPROVAL, false)
		client := steadyWire(t, f, streamTestOptions{timeout: storetest.ObservationTimeout})
		requireOpenEnd(t, submitAdmission(t, f, client, m.ID, key, fixtureStreamID(t), 9), connect.CodeInvalidArgument, openInvalid, "")
		requireRecoveryFence(t, f, m.ID, 0)
		waiting := recoveryWatchAt(t, watch, f.head(t)).GetSaved()
		requireRecoveryClient(t, waiting, m.ID, webv1.ClientState_CLIENT_STATE_WAITING_FOR_APPROVAL, true)
		if waiting.GetConnectedAt() != nil || f.connectionCount(t, m.ID) != 0 {
			t.Fatal("A refused candidate granted liveness to a waiting machine.")
		}
		snapshot := web.watch(t)
		requireRecoveryClient(t, recoverySnapshot(t, snapshot, m.ID), m.ID, webv1.ClientState_CLIENT_STATE_WAITING_FOR_APPROVAL, true)
	})

	t.Run("ReadyRunReconnectClears", func(t *testing.T) {
		f := newServerStreamFixture(t, handshakeFixtureOptions{})
		m, key := f.register(t, "ready reconnect peer")
		seedAdmissionMark(t, f, m.ID, 10)
		client := steadyWire(t, f, streamTestOptions{timeout: storetest.ObservationTimeout})
		run := fixtureStreamID(t)
		incumbent := submitAdmission(t, f, client, m.ID, key, run, 11)
		requireAdmissionWelcome(t, incumbent, 10)
		recoveryHeartbeat(t, incumbent)
		current := f.current(t, m.ID)
		requireOpenEnd(t, submitAdmission(t, f, client, m.ID, key, fixtureStreamID(t), 9), connect.CodeInvalidArgument, openInvalid, "")
		requireRecoveryFence(t, f, m.ID, current.Generation)
		disconnectRecoveryPeer(t, f, incumbent, m.ID, current.StreamID)

		web := newRecoveryMachinesClient(t, f)
		watch := web.watch(t)
		requireRecoveryClient(t, recoverySnapshot(t, watch, m.ID), m.ID, webv1.ClientState_CLIENT_STATE_WAITING_FOR_APPROVAL, true)
		reconnect := submitAdmission(t, f, client, m.ID, key, run, 11)
		requireAdmissionWelcome(t, reconnect, 10)
		requireRecoveryFence(t, f, m.ID, -1)
		if !watch.Receive() {
			t.Fatalf("The Machines watch did not publish the clearing admission. The code was %v.", connect.CodeOf(watch.Err()))
		}
		requireRecoveryClient(t, watch.Msg().GetSaved(), m.ID, webv1.ClientState_CLIENT_STATE_WAITING_FOR_APPROVAL, false)
		requireAdmissionRun(t, f, m.ID, run, run, 11)
	})

	t.Run("DifferentRunReadyAtAdmissionClears", func(t *testing.T) {
		f := newServerStreamFixture(t, handshakeFixtureOptions{})
		m, key := f.register(t, "new ready run peer")
		seedAdmissionMark(t, f, m.ID, 10)
		client := steadyWire(t, f, streamTestOptions{timeout: storetest.ObservationTimeout})
		incumbent := submitAdmission(t, f, client, m.ID, key, fixtureStreamID(t), 11)
		requireAdmissionWelcome(t, incumbent, 10)
		awaitAdmissionPublication(t, f, m.ID)
		current := f.current(t, m.ID)
		requireOpenEnd(t, submitAdmission(t, f, client, m.ID, key, fixtureStreamID(t), 9), connect.CodeInvalidArgument, openInvalid, "")
		requireRecoveryFence(t, f, m.ID, current.Generation)

		web := newRecoveryMachinesClient(t, f)
		watch := web.watch(t)
		waiting := webv1.ClientState_CLIENT_STATE_WAITING_FOR_APPROVAL
		requireRecoveryClient(t, recoverySnapshot(t, watch, m.ID), m.ID, waiting, true)
		run := fixtureStreamID(t)
		replacement := submitAdmission(t, f, client, m.ID, key, run, 11)
		requireAdmissionWelcome(t, replacement, 10)
		requireAdmissionRun(t, f, m.ID, run, run, 11)
		requireRecoveryFence(t, f, m.ID, -1)
		requireRecoveryClient(t, recoveryWatchAt(t, watch, f.head(t)).GetSaved(), m.ID, waiting, false)
	})

	for _, scenario := range []struct {
		name                string
		refuseBeforeReplay  bool
		refreshDuringReplay bool
	}{
		{name: "PendingIncumbentCannotClear"},
		{name: "LaterPendingRunClearsAtReadiness", refuseBeforeReplay: true},
		{name: "RefusalDuringReplayRefreshesFence", refuseBeforeReplay: true, refreshDuringReplay: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			f := newServerStreamFixture(t, handshakeFixtureOptions{})
			m, key := f.register(t, "pending recovery peer")
			seedAdmissionMark(t, f, m.ID, 10)
			finalRead := make(chan struct{}, 1)
			hold, release := storetest.Signal(t)
			defer release()
			client := steadyWire(t, f, streamTestOptions{timeout: storetest.ObservationTimeout, controls: &streamControls{
				afterReceive: func(frame *linkv1.OpenRequest, err error) {
					if scenario.refreshDuringReplay && err == nil && frame.GetEvent().GetNumber() == 12 {
						finalRead <- struct{}{}
						<-hold
					}
				},
			}})
			refuse := func() {
				t.Helper()
				requireOpenEnd(t, submitAdmission(t, f, client, m.ID, key, fixtureStreamID(t), 9), connect.CodeInvalidArgument, openInvalid, "")
			}
			var fence int64
			if scenario.refuseBeforeReplay {
				refuse()
				requireRecoveryFence(t, f, m.ID, fence)
			}
			run := fixtureStreamID(t)
			pending := submitAdmission(t, f, client, m.ID, key, run, 13)
			requireAdmissionWelcome(t, pending, 10)
			recoveryHeartbeat(t, pending)
			current := f.current(t, m.ID)
			requireAdmissionRun(t, f, m.ID, run, "", 13)
			if !scenario.refuseBeforeReplay {
				refuse()
				fence = current.Generation
			}
			requireRecoveryFence(t, f, m.ID, fence)
			web := newRecoveryMachinesClient(t, f)
			watch := web.watch(t)
			waiting := webv1.ClientState_CLIENT_STATE_WAITING_FOR_APPROVAL
			requireRecoveryClient(t, recoverySnapshot(t, watch, m.ID), m.ID, waiting, true)
			sendHandshake(t, pending, eventFrame(factsEvent(11, "pending run facts")))
			requireEventAck(t, pending, 11)
			requireRecoveryFence(t, f, m.ID, fence)
			requireRecoveryClient(t, recoveryWatchAt(t, watch, f.head(t)).GetSaved(), m.ID, waiting, true)

			head := f.head(t)
			sendHandshake(t, pending, eventFrame(harnessEvent(12, &linkv1.Harness{Name: "codex"})))
			if scenario.refreshDuringReplay {
				storetest.Receive(t, "the last replay frame held before application", finalRead)
				refuse()
				f.requireSaved(t, head)
				fence = current.Generation
				requireRecoveryFence(t, f, m.ID, fence)
				release()
			}
			requireEventAck(t, pending, 12)
			f.requireSaved(t, head, m.ID)
			requireAdmissionRun(t, f, m.ID, run, run, 13)

			if scenario.refuseBeforeReplay && !scenario.refreshDuringReplay {
				requireRecoveryFence(t, f, m.ID, -1)
				requireRecoveryClient(t, recoveryWatchAt(t, watch, f.head(t)).GetSaved(), m.ID, waiting, false)
				return
			}
			requireRecoveryFence(t, f, m.ID, fence)
			requireRecoveryClient(t, recoveryWatchAt(t, watch, f.head(t)).GetSaved(), m.ID, waiting, true)
			recoveryHeartbeat(t, pending)
			requireRecoveryFence(t, f, m.ID, fence)
			disconnectRecoveryPeer(t, f, pending, m.ID, current.StreamID)
			later := submitAdmission(t, f, client, m.ID, key, run, 13)
			requireAdmissionWelcome(t, later, 12)
			requireRecoveryFence(t, f, m.ID, -1)
		})
	}

	t.Run("NonqualifyingRefusals", func(t *testing.T) {
		testRecoveryRefusalControls(t)
	})
}

func testRecoveryRefusalControls(t *testing.T) {
	t.Helper()
	for _, scenario := range []string{"StagedOnly", "PendingOnly", "Superseded", "ChangedPair", "MalformedPair", "BadProof", "WrongRevision"} {
		t.Run(scenario, func(t *testing.T) {
			f := newServerStreamFixture(t, handshakeFixtureOptions{})
			m, key := f.register(t, "recovery refusal control")
			seedAdmissionMark(t, f, m.ID, 10)
			client := steadyWire(t, f, streamTestOptions{timeout: storetest.ObservationTimeout})
			run := fixtureStreamID(t)
			peer := submitAdmission(t, f, client, m.ID, key, run, 11)
			requireAdmissionWelcome(t, peer, 10)
			recoveryHeartbeat(t, peer)
			current := f.current(t, m.ID)
			candidateRun, first := fixtureStreamID(t), uint64(9)
			switch scenario {
			case "StagedOnly":
				id := seedResultCommand(t, f.presenceFixture, m.ID, current.StreamID, "restart", 1, true, current.ConnectedAt.Add(2*time.Minute))
				storetest.Exec(t, f.obs, "UPDATE link_commands SET staged_outcome = 'done', staged_event_number = 13 WHERE id = $1", id)
				first = 13
			case "PendingOnly", "Superseded":
				later := submitAdmission(t, f, client, m.ID, key, fixtureStreamID(t), 15)
				requireAdmissionWelcome(t, later, 10)
				requireOpenEnd(t, peer, connect.CodeAborted, openReplaced, "")
				recoveryHeartbeat(t, later)
				peer = later
				first = 14
				if scenario == "Superseded" {
					sendHandshake(t, peer, eventFrame(factsEvent(11, "superseded fixture facts")))
					requireEventAck(t, peer, 11)
					candidateRun, first = run, 11
				}
			case "ChangedPair":
				candidateRun = run
			case "MalformedPair":
				candidateRun = rejectedRunMarker
			}

			head, before := f.head(t), readAdmissionRows(t, f, m.ID)
			hello := helloFrame(m.ID, admissionRevision, 2)
			hello.GetHello().RunId, hello.GetHello().FirstEventNumber = candidateRun, first
			if scenario == "WrongRevision" {
				hello.GetHello().LinkRevision = 1
			}
			candidate := startHandshake(t, client, nil)
			sendHandshake(t, candidate, hello)
			challenge := requireChallenge(t, candidate, f.serverKey, hello.GetHello())
			proof := proofFrame(key, hello.GetHello(), challenge)
			switch scenario {
			case "WrongRevision":
				requireOpenEnd(t, candidate, connect.CodeFailedPrecondition, "The server speaks revision 2 of the link.", "link_revision_mismatch")
			case "BadProof":
				proof.GetProof().MachineSignature[0] ^= 1
				sendHandshake(t, candidate, proof)
				requireOpenEnd(t, candidate, connect.CodeUnauthenticated, openSignature, "")
			default:
				sendHandshake(t, candidate, proof)
				requireOpenEnd(t, candidate, connect.CodeInvalidArgument, openInvalid, "")
			}
			requireRecoveryFence(t, f, m.ID, -1)
			requireAdmissionUnchanged(t, f, m.ID, head, before)
			recoveryHeartbeat(t, peer)
			current = f.current(t, m.ID)
			requireOpenEnd(t, submitAdmission(t, f, client, m.ID, key, fixtureStreamID(t), 9), connect.CodeInvalidArgument, openInvalid, "")
			requireRecoveryFence(t, f, m.ID, current.Generation)
		})
	}
}

func TestRecoveryDiagnosticCommitReplyLost(t *testing.T) {
	for _, committed := range []bool{true, false} {
		name := "BeforeCommitReachesServer"
		if committed {
			name = "AfterServerCommits"
		}
		t.Run(name, func(t *testing.T) {
			cut := make(chan struct{}, 1)
			hold, release := storetest.Signal(t)
			defer release()
			var faultNextProof, faultNextCommit atomic.Bool
			f := newServerStreamFixture(t, handshakeFixtureOptions{
				seams: Seams{AfterProof: func(context.Context, string) {
					if faultNextProof.CompareAndSwap(true, false) {
						faultNextCommit.Store(true)
					}
				}},
				configure: func(cfg *pgxpool.Config) {
					dial := cfg.ConnConfig.DialFunc
					cfg.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
						conn, err := dial(ctx, network, address)
						if err != nil {
							return nil, err
						}
						return &recoveryCommitConn{admissionReplyConn: &admissionReplyConn{Conn: conn, arrived: cut, release: hold}}, nil
					}
					cfg.ConnConfig.Tracer = handshakeQueryTrace(func(ctx context.Context, conn *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
						if strings.EqualFold(strings.TrimSpace(d.SQL), "commit") && faultNextCommit.CompareAndSwap(true, false) {
							pipe := conn.PgConn().Conn().(*recoveryCommitConn)
							if committed {
								pipe.armed.Store(true)
							} else {
								pipe.dropWrite.Store(true)
							}
						}
						return ctx
					})
				},
			})
			m, key := f.register(t, "diagnostic commit incumbent")
			seedAdmissionMark(t, f, m.ID, 10)
			client := steadyWire(t, f, streamTestOptions{timeout: storetest.ObservationTimeout})
			incumbent := submitAdmission(t, f, client, m.ID, key, fixtureStreamID(t), 11)
			requireAdmissionWelcome(t, incumbent, 10)
			recoveryHeartbeat(t, incumbent)
			current := f.current(t, m.ID)
			head := f.head(t)

			faultNextProof.Store(true)
			candidate := submitAdmission(t, f, client, m.ID, key, fixtureStreamID(t), 9)
			reply := make(chan openReceiveResult, 1)
			go func() { reply <- receiveOpenFrame(candidate) }()
			select {
			case <-cut:
			case <-reply:
				t.Fatal("The candidate ended before its diagnostic reached the commit boundary.")
			case <-time.After(storetest.ObservationTimeout):
				t.Fatal("The diagnostic did not reach the controlled commit boundary.")
			}

			if committed {
				requireRecoveryFence(t, f, m.ID, current.Generation)
				f.requireSaved(t, head, m.ID)
			}
			release()
			terminal := storetest.Receive(t, "the refusal after the lost commit reply", reply)
			checkOpenEnd(t, terminal.frame, terminal.err, connect.CodeUnavailable, openDatabase, "")
			requireRecoveryIncumbent(t, f, m.ID, current)
			if committed {
				requireRecoveryFence(t, f, m.ID, current.Generation)
				f.requireSaved(t, head, m.ID)
			} else {
				requireRecoveryFence(t, f, m.ID, -1)
				f.requireSaved(t, head)
			}
			recoveryHeartbeat(t, incumbent)
			requireRecoveryIncumbent(t, f, m.ID, current)
			if committed {
				finishAdmissionPeer(t, incumbent)
				restarted := restartAdmissionFixture(t, f)
				requireRecoveryFence(t, restarted, m.ID, current.Generation)
				web := newRecoveryMachinesClient(t, restarted)
				requireRecoveryClient(t, recoverySnapshot(t, web.watch(t), m.ID), m.ID, webv1.ClientState_CLIENT_STATE_WAITING_FOR_APPROVAL, true)
			}
		})
	}
}

type recoveryCommitConn struct {
	*admissionReplyConn
	dropWrite atomic.Bool
}

func (c *recoveryCommitConn) Write(p []byte) (int, error) {
	if c.dropWrite.CompareAndSwap(true, false) {
		_ = c.Conn.Close()
		c.arrived <- struct{}{}
		<-c.release
		return 0, io.ErrClosedPipe
	}
	return c.Conn.Write(p)
}

func recoveryHeartbeat(t *testing.T, peer *openClientStream) {
	t.Helper()
	sendHandshake(t, peer, heartbeatFrame(nil, nil))
	requireHeartbeatAck(t, peer)
}

func disconnectRecoveryPeer(t *testing.T, f *handshakeFixture, peer *openClientStream, machineID, streamID string) {
	t.Helper()
	finishAdmissionPeer(t, peer)
	// Drive the deferred disconnect before inspecting the offline record.
	if removed, err := f.state.Disconnect(t.Context(), machineID, streamID); err != nil || !removed {
		t.Fatal("The fixture could not remove the incumbent's saved connection.")
	}
}

func requireRecoveryIncumbent(t *testing.T, f *handshakeFixture, id string, incumbent machines.Presence) {
	t.Helper()
	got := f.current(t, id)
	if !got.Held || got.Generation != incumbent.Generation || got.StreamID != incumbent.StreamID {
		t.Fatal("The refusal displaced the incumbent's held connection or generation.")
	}
	generation := storetest.Scalar[int64](t, f.obs, "SELECT stream_generation FROM link_machines WHERE machine_id = $1", id)
	if generation != incumbent.Generation {
		t.Fatal("The refusal changed the machine's durable stream generation.")
	}
}

func requireRecoveryFence(t *testing.T, f *handshakeFixture, id string, want int64) {
	t.Helper()
	got := storetest.Scalar[int64](t, f.obs,
		"SELECT COALESCE(recovery_set_generation, -1) FROM link_machines WHERE machine_id = $1", id)
	if got != want {
		t.Fatalf("The saved recovery generation was %d, want %d. A generation of -1 represents a clear flag.", got, want)
	}
}

type recoveryMachinesClient struct {
	client lgtmv1connect.ClientServiceClient
	server *httptest.Server
	cookie string
}

func newRecoveryMachinesClient(t *testing.T, f *handshakeFixture) recoveryMachinesClient {
	t.Helper()
	if err := f.s.Migrate(t.Context(), auth.Migrations(), machines.Migrations(), Migrations()); err != nil {
		t.Fatal("The fixture could not apply the session migration.")
	}
	password, err := auth.NewPassword("recovery fixture password")
	if err != nil {
		t.Fatal("The fixture could not initialize the owner password.")
	}
	sessions := auth.NewSessions(f.s, password)
	token, err := sessions.Issue(t.Context())
	if err != nil {
		t.Fatal("The fixture could not issue an owner session.")
	}
	origins, err := auth.NewOriginPolicy("http://127.0.0.1:8787", nil)
	if err != nil {
		t.Fatal("The fixture could not initialize the origin policy.")
	}
	service := machines.NewService(f.s, sessions, f.r, f.tokens).WithLink(f.state)
	server := httptest.NewServer(api.NewHandler(api.Options{Origins: origins, Sessions: sessions, Clients: service}))
	t.Cleanup(server.Close)
	return recoveryMachinesClient{
		client: lgtmv1connect.NewClientServiceClient(server.Client(), server.URL),
		server: server,
		cookie: (&http.Cookie{Name: auth.CookieName, Value: token}).String(),
	}
}

func (w recoveryMachinesClient) watch(t *testing.T) *connect.ServerStreamForClient[webv1.WatchClientsResponse] {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), storetest.ObservationTimeout)
	t.Cleanup(cancel)
	req := connect.NewRequest(&webv1.WatchClientsRequest{})
	req.Header().Set("Cookie", w.cookie)
	watch, err := w.client.WatchClients(ctx, req)
	if err != nil {
		t.Fatalf("The authenticated Machines watch failed with code %v.", connect.CodeOf(err))
	}
	t.Cleanup(func() { _ = watch.Close() })
	return watch
}

func (w recoveryMachinesClient) remove(t *testing.T, id string) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), storetest.ObservationTimeout)
	defer cancel()
	req := connect.NewRequest(&webv1.RemoveClientRequest{Id: id})
	req.Header().Set("Cookie", w.cookie)
	req.Header().Set(store.RequestKeyHeader, fixtureStreamID(t))
	_, err := w.client.RemoveClient(ctx, req)
	return err
}

func closeRecoveryWatch(t *testing.T, watch *connect.ServerStreamForClient[webv1.WatchClientsResponse]) {
	t.Helper()
	if err := watch.Close(); err != nil {
		t.Fatal("The fixture could not close the authenticated Machines watch.")
	}
}

func recoverySnapshot(t *testing.T, watch *connect.ServerStreamForClient[webv1.WatchClientsResponse], id string) *webv1.Client {
	t.Helper()
	if !watch.Receive() {
		t.Fatalf("The Machines watch did not publish a snapshot. The code was %v.", connect.CodeOf(watch.Err()))
	}
	return recoverySnapshotClient(t, watch.Msg(), id)
}

func recoverySnapshotClient(t *testing.T, snapshot *webv1.WatchClientsResponse, id string) *webv1.Client {
	t.Helper()
	for _, row := range snapshot.GetAll().GetClients() {
		if row.GetId() == id {
			return row
		}
	}
	t.Fatal("The Machines snapshot did not contain the expected machine.")
	return nil
}

func recoveryWatchAt(t *testing.T, watch *connect.ServerStreamForClient[webv1.WatchClientsResponse], position int64) *webv1.WatchClientsResponse {
	t.Helper()
	for watch.Receive() {
		p := watch.Msg().GetPosition().GetValue()
		if p == uint64(position) {
			return watch.Msg()
		}
		if p > uint64(position) {
			t.Fatal("The Machines watch passed the position of the expected change.")
		}
	}
	t.Fatalf("The Machines watch did not publish the expected change. The code was %v.", connect.CodeOf(watch.Err()))
	return nil
}

func requireRecoveryClient(t *testing.T, row *webv1.Client, id string, state webv1.ClientState, flag bool) {
	t.Helper()
	if row.GetId() != id || row.GetState() != state {
		t.Fatalf("The Machines watch did not project the expected machine in state %s.", state)
	}
	if row.GetEventHistoryNeedsRecovery() != flag {
		t.Fatalf("The Machines watch projected the recovery warning as %t, want %t.", row.GetEventHistoryNeedsRecovery(), flag)
	}
}

func requireRecoveryOffline(t *testing.T, row *webv1.Client, id string) {
	t.Helper()
	requireRecoveryClient(t, row, id, webv1.ClientState_CLIENT_STATE_OFFLINE, true)
	if row.GetConnectedAt() != nil || row.CpuPercent != nil || row.MemoryPercent != nil {
		t.Fatal("The offline warning brought live connection fields into the Machines watch.")
	}
}
