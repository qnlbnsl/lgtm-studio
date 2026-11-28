package link

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link/machinetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

const (
	admissionRevision = 2
	maxRunFirst       = uint64(1) << 63
	rejectedRunMarker = "fixture-credential-in-rejected-run"
)

func TestRevisionTwoProofAndRunAdmission(t *testing.T) {
	for _, protocol := range handshakeProtocols {
		t.Run(protocol.name, func(t *testing.T) {
			for _, tc := range []struct {
				name     string
				revision uint32
				mark     uint64
				first    uint64
				runID    string
				proof    string
				code     connect.Code
				sentence string
				reason   string
			}{
				{
					name: "RevisionOneRefusal", revision: 1, mark: 5,
					code: connect.CodeFailedPrecondition, sentence: "The server speaks revision 2 of the link.",
					reason: "link_revision_mismatch",
				},
				{
					name: "BadProofPrecedesBehindMark", revision: admissionRevision, mark: 5, first: 5, proof: "bad",
					code: connect.CodeUnauthenticated, sentence: openSignature,
				},
				{
					name: "BadProofPrecedesMalformedHistory", revision: admissionRevision, mark: 5,
					runID: rejectedRunMarker, proof: "bad",
					code: connect.CodeUnauthenticated, sentence: openSignature,
				},
				{
					name: "MalformedProofPrecedesHistory", revision: admissionRevision, mark: 5, first: 5, proof: "short",
					code: connect.CodeInvalidArgument, sentence: openInvalid,
				},
				{name: "Welcome", revision: admissionRevision, mark: 5, first: 6},
				{name: "InclusiveMaximum", revision: admissionRevision, mark: maxRunFirst - 1, first: maxRunFirst},
				{
					name: "InvalidRunVersion", revision: admissionRevision, first: 1,
					runID: "0193a2b4-5c6d-4e8f-9a0b-1c2d3e4f5a6b", code: connect.CodeInvalidArgument, sentence: openInvalid,
				},
				{
					name: "NoncanonicalRun", revision: admissionRevision, first: 1,
					runID: "0193A2B4-5c6d-7e8f-9a0b-1c2d3e4f5a6b", code: connect.CodeInvalidArgument, sentence: openInvalid,
				},
				{
					name: "PrivateRejection", revision: admissionRevision, first: 1,
					runID: rejectedRunMarker, code: connect.CodeInvalidArgument, sentence: openInvalid,
				},
				{
					name: "ZeroFirstEvent", revision: admissionRevision,
					code: connect.CodeInvalidArgument, sentence: openInvalid,
				},
				{
					name: "AboveMaximum", revision: admissionRevision, mark: maxRunFirst - 1, first: maxRunFirst + 1,
					code: connect.CodeInvalidArgument, sentence: openInvalid,
				},
				{
					name: "AtMaximumMark", revision: admissionRevision, mark: maxRunFirst - 1, first: maxRunFirst - 1,
					code: connect.CodeInvalidArgument, sentence: openInvalid,
				},
			} {
				t.Run(tc.name, func(t *testing.T) {
					logs := new(handshakeLog)
					f := newHandshakeFixture(t, handshakeFixtureOptions{logger: slog.New(slog.NewTextHandler(logs, nil))})
					m, key := f.register(t, "revision two peer")
					runID := tc.runID
					if runID == "" {
						runID = fixtureStreamID(t)
					}
					seedAdmissionMark(t, f, m.ID, tc.mark)
					before := readAdmissionRows(t, f, m.ID)
					head := f.head(t)
					client := f.wire(t, handshakeWireOptions{}, protocol.options...)
					s := startHandshake(t, client, nil)
					hello := helloFrame(m.ID, tc.revision, 1)
					if tc.revision == 1 {
						runID = ""
					}
					hello.GetHello().RunId = runID
					hello.GetHello().FirstEventNumber = tc.first
					sendHandshake(t, s, hello)
					challenge := requireChallenge(t, s, f.serverKey, hello.GetHello())
					proof := proofFrame(key, hello.GetHello(), challenge)
					switch tc.proof {
					case "bad":
						proof.GetProof().MachineSignature[0] ^= 1
					case "short":
						proof.GetProof().MachineSignature = []byte{1}
					}
					sendHandshake(t, s, proof)
					if tc.code == 0 {
						requireAdmissionWelcome(t, s, tc.mark)
						requireAdmissionRun(t, f, m.ID, runID, runID, tc.first)
						return
					}

					terminal := receiveOpenFrame(s)
					checkOpenEnd(t, terminal.frame, terminal.err, tc.code, tc.sentence, tc.reason)
					if tc.name == "AtMaximumMark" {
						// Item 4 may record a diagnostic for a verified, behind-mark refusal.
						if f.connectionCount(t, m.ID) != 0 || !storetest.Scalar[bool](t, f.obs, `SELECT EXISTS (
							SELECT FROM link_machines WHERE machine_id = $1 AND event_mark = $2 AND stream_generation = 0)`,
							m.ID, int64(tc.mark)) {
							t.Error("The behind-mark run changed the generation or mark, or acquired a connection.")
						}
						requireAdmissionHistory(t, f, m.ID, 0)
						return
					}
					requireAdmissionUnchanged(t, f, m.ID, head, before)
					if tc.runID == rejectedRunMarker {
						var refusal *connect.Error
						if errors.As(terminal.err, &refusal) &&
							strings.Contains(fmt.Sprint(refusal.Meta(), s.ResponseHeader(), s.ResponseTrailer()), rejectedRunMarker) {
							t.Error("The refusal metadata leaked a synthetic credential marker.")
						}
						if strings.Contains(readAdmissionRows(t, f, m.ID), rejectedRunMarker) || logs.contains(rejectedRunMarker) {
							t.Error("The rejected run leaked a synthetic credential marker into durable rows or logs.")
						}
					}
				})
			}
			t.Run("ScriptedProcessRepeatsItsPair", func(t *testing.T) {
				f := newHandshakeFixture(t, handshakeFixtureOptions{})
				server := machinetest.NewServer(t, f.route(), 5*time.Second)
				client := linkv1connect.NewLinkServiceClient(handshakeHTTPClient(t), server.URL, protocol.options...)
				token, err := f.tokens.Create(t.Context(), machines.TokenOptions{})
				if err != nil {
					t.Fatal("The scripted peer's join token could not be created.")
				}
				peer, err := machinetest.Join(t.Context(), client, token.Text, "revision two scripted peer")
				if err != nil || peer == nil {
					t.Fatal("The scripted peer could not join.")
				}
				if !streamVersion7.MatchString(peer.RunID) || peer.RunID == peer.ID || peer.FirstEventNumber != 1 {
					t.Fatal("Join did not initialize a distinct version 7 run with first number 1.")
				}
				run := RunAnnouncement{ID: peer.RunID, FirstEventNumber: peer.FirstEventNumber}
				for _, mark := range []uint64{0, 5} {
					if mark != 0 {
						storetest.Exec(t, f.obs, "UPDATE link_machines SET event_mark = $2 WHERE machine_id = $1", peer.ID, int64(mark))
					}
					ctx, cancel := context.WithTimeout(t.Context(), storetest.ObservationTimeout)
					defer cancel()
					s, welcome, err := peer.Connect(ctx, client)
					if err != nil || s == nil || welcome.GetLinkRevision() != admissionRevision || welcome.GetLastEventNumber() != mark {
						t.Fatal("The scripted peer did not receive the revision 2 Welcome and committed mark.")
					}
					requireAdmissionRun(t, f, peer.ID, run.ID, run.ID, run.FirstEventNumber)
					finishAdmissionPeer(t, s)
				}
				if peer.RunID != run.ID || peer.FirstEventNumber != run.FirstEventNumber {
					t.Error("Reconnect changed the scripted process's run pair.")
				}
				requireAdmissionHistory(t, f, peer.ID, 1)
				peer.RunID, peer.FirstEventNumber = fixtureStreamID(t), 6
				ctx, cancel := context.WithTimeout(t.Context(), storetest.ObservationTimeout)
				defer cancel()
				s, welcome, err := peer.Connect(ctx, client)
				if err != nil || s == nil || welcome.GetLinkRevision() != admissionRevision || welcome.GetLastEventNumber() != 5 {
					t.Fatal("The scripted peer's explicit fresh run did not receive Welcome.")
				}
				requireAdmissionRun(t, f, peer.ID, peer.RunID, peer.RunID, 6)
				requireAdmissionHistory(t, f, peer.ID, 2)
				finishAdmissionPeer(t, s)
			})
		})
	}
}

func TestRunAdmissionPersistsAcrossRestart(t *testing.T) {
	t.Run("PendingRun", func(t *testing.T) {
		f := newHandshakeFixture(t, handshakeFixtureOptions{})
		m, key := f.register(t, "pending run")
		seedAdmissionMark(t, f, m.ID, 5)
		a, b, c := fixtureStreamID(t), fixtureStreamID(t), fixtureStreamID(t)
		client := f.wire(t, handshakeWireOptions{})
		attempt := func(run string, first uint64) *openClientStream {
			t.Helper()
			return submitAdmission(t, f, client, m.ID, key, run, first)
		}

		ready := attempt(a, 6)
		requireAdmissionWelcome(t, ready, 5)
		requireAdmissionRun(t, f, m.ID, a, a, 6)
		finishAdmissionPeer(t, ready)
		pending := attempt(b, 10)
		requireAdmissionWelcome(t, pending, 5)
		requireAdmissionRun(t, f, m.ID, b, a, 10)
		finishAdmissionPeer(t, pending)

		f = restartAdmissionFixture(t, f)
		requireAdmissionRun(t, f, m.ID, b, a, 10)
		client = f.wire(t, handshakeWireOptions{})
		head, before := f.head(t), readAdmissionRows(t, f, m.ID)
		requireOpenEnd(t, attempt(a, 6), connect.CodeInvalidArgument, openInvalid, "")
		requireOpenEnd(t, attempt(b, 11), connect.CodeInvalidArgument, openInvalid, "")
		requireAdmissionUnchanged(t, f, m.ID, head, before)

		pending = attempt(b, 10)
		requireAdmissionWelcome(t, pending, 5)
		awaitAdmissionPublication(t, f, m.ID)
		row := f.current(t, m.ID)
		command := seedResultCommand(t, f.presenceFixture, m.ID, row.StreamID, "restart", 2, true, row.ConnectedAt.Add(2*time.Minute))
		head, before = f.head(t), readAdmissionRows(t, f, m.ID)
		requireOpenEnd(t, attempt(c, 9), connect.CodeInvalidArgument, openInvalid, "")
		requireAdmissionUnchanged(t, f, m.ID, head, before)
		sendHandshake(t, pending, heartbeatFrame(nil, nil))
		if got := receiveOpenFrame(pending); got.err != nil || got.frame.GetHeartbeatAck() == nil {
			t.Fatal("The incumbent could not acknowledge a heartbeat after the pending-fence refusal.")
		}
		finishAdmissionPeer(t, pending)
		replacement := attempt(c, 10)
		requireAdmissionWelcome(t, replacement, 5)
		requireAdmissionRun(t, f, m.ID, c, a, 10)
		finishAdmissionPeer(t, replacement)

		f = restartAdmissionFixture(t, f)
		client = f.wire(t, handshakeWireOptions{})
		head, before = f.head(t), readAdmissionRows(t, f, m.ID)
		requireOpenEnd(t, attempt(b, 10), connect.CodeInvalidArgument, openInvalid, "")
		requireAdmissionUnchanged(t, f, m.ID, head, before)
		requireAdmissionRun(t, f, m.ID, c, a, 10)
		requireAdmissionHistory(t, f, m.ID, 3)
		if result := readCommandResult(t, f.presenceFixture, command); result.outcome != "" || !result.hasPayload {
			t.Error("Restart or a fenced admission settled the pending command.")
		}
	})

	t.Run("EqualReadyBoundary", func(t *testing.T) {
		f := newHandshakeFixture(t, handshakeFixtureOptions{})
		m, key := f.register(t, "equal ready boundary")
		seedAdmissionMark(t, f, m.ID, 5)
		a, b := fixtureStreamID(t), fixtureStreamID(t)
		client := f.wire(t, handshakeWireOptions{})
		for _, id := range []string{a, b} {
			s := submitAdmission(t, f, client, m.ID, key, id, 6)
			requireAdmissionWelcome(t, s, 5)
			requireAdmissionRun(t, f, m.ID, id, id, 6)
			finishAdmissionPeer(t, s)
		}
		for _, phase := range []string{"BeforeRestart", "AfterRestart"} {
			t.Run(phase, func(t *testing.T) {
				if phase == "AfterRestart" {
					f = restartAdmissionFixture(t, f)
					client = f.wire(t, handshakeWireOptions{})
				}
				head, before := f.head(t), readAdmissionRows(t, f, m.ID)
				requireOpenEnd(t, submitAdmission(t, f, client, m.ID, key, a, 6), connect.CodeInvalidArgument, openInvalid, "")
				requireAdmissionUnchanged(t, f, m.ID, head, before)
				requireAdmissionRun(t, f, m.ID, b, b, 6)
				requireAdmissionHistory(t, f, m.ID, 2)
			})
		}
	})

	t.Run("StagedWatermark", func(t *testing.T) {
		f := newHandshakeFixture(t, handshakeFixtureOptions{})
		m, key := f.register(t, "staged watermark")
		f.approve(t, m.ID)
		seedAdmissionMark(t, f, m.ID, 5)
		client := f.wire(t, handshakeWireOptions{})
		a, b := fixtureStreamID(t), fixtureStreamID(t)
		incumbent := submitAdmission(t, f, client, m.ID, key, a, 6)
		requireAdmissionWelcome(t, incumbent, 5)
		awaitAdmissionPublication(t, f, m.ID)
		row := f.current(t, m.ID)
		for _, watermark := range []int64{7, 8} {
			command := seedResultCommand(t, f.presenceFixture, m.ID, row.StreamID, "restart", 1, true, row.ConnectedAt.Add(2*time.Minute))
			storetest.Exec(t, f.obs, "UPDATE link_commands SET staged_outcome = 'done', staged_event_number = $2 WHERE id = $1", command, watermark)
		}
		other, otherKey := f.register(t, "independent staged watermark peer")
		if err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
			return f.r.SetApproved(ctx, tx, other.ID, "Other staged watermark machine")
		}); err != nil {
			t.Fatal("The fixture could not approve the independent signed peer.")
		}
		seedAdmissionMark(t, f, other.ID, 5)
		otherPeer := submitAdmission(t, f, client, other.ID, otherKey, fixtureStreamID(t), 6)
		requireAdmissionWelcome(t, otherPeer, 5)
		finishAdmissionPeer(t, otherPeer)
		head, before := f.head(t), readAdmissionRows(t, f, m.ID)
		requireOpenEnd(t, submitAdmission(t, f, client, m.ID, key, b, 8), connect.CodeInvalidArgument, openInvalid, "")
		requireAdmissionUnchanged(t, f, m.ID, head, before)
		requireAdmissionHistory(t, f, m.ID, 1)
		replacement := submitAdmission(t, f, client, m.ID, key, b, 9)
		requireAdmissionWelcome(t, replacement, 5)
		requireAdmissionRun(t, f, m.ID, b, a, 9)
		requireOpenEnd(t, incumbent, connect.CodeAborted, openReplaced, "")
		requireAdmissionHistory(t, f, m.ID, 2)
	})

	t.Run("AdmissionCallbacks", testAdmissionCallbacks)
}

func testAdmissionCallbacks(t *testing.T) {
	f := newHandshakeFixture(t, handshakeFixtureOptions{})
	m, key := f.register(t, "admission callbacks")
	seedAdmissionMark(t, f, m.ID, 5)
	ready, rolledBack := fixtureStreamID(t), fixtureStreamID(t)
	replacement, pending := fixtureStreamID(t), fixtureStreamID(t)
	storetest.Exec(t, f.obs, `CREATE TABLE fixture_admission_hooks (seq bigserial PRIMARY KEY, event text NOT NULL)`)
	err := f.state.RegisterPart(PartHooks{
		StreamAdmitted: func(ctx context.Context, tx *store.Tx, a Admission) error {
			var readyRun string
			err := tx.QueryRow(ctx, `SELECT COALESCE(m.ready_run_id, '')
				FROM link_machines m JOIN link_runs r ON r.machine_id = m.machine_id AND r.run_id = m.admitted_run_id
				JOIN link_connections c ON c.machine_id = m.machine_id AND c.generation = m.stream_generation
				WHERE m.machine_id = $1 AND r.run_id = $2 AND c.stream_id = $3 AND r.replay_through = $4`,
				a.MachineID, a.RunID, a.StreamID, int64(a.FirstEventNumber-1)).Scan(&readyRun)
			if err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `INSERT INTO fixture_admission_hooks (event) VALUES ($1)`,
				fmt.Sprintf("admit/%s/%s/%d/%t", a.RunID, readyRun, a.FirstEventNumber, a.Ready))
			return err
		},
		RunChanged: func(ctx context.Context, tx *store.Tx, change RunChange) error {
			var readyRun string
			if err := tx.QueryRow(ctx, `SELECT ready_run_id FROM link_machines WHERE machine_id = $1`, change.MachineID).
				Scan(&readyRun); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `INSERT INTO fixture_admission_hooks (event) VALUES ($1)`,
				fmt.Sprintf("ready/%s/%s/%s/%d", change.RunID, change.PreviousRunID, readyRun, change.FirstEventNumber))
			if err == nil && change.RunID == rolledBack {
				return errors.New("The fixture refused the ready callback.")
			}
			return err
		},
	})
	if err != nil {
		t.Fatal("The admission callbacks could not be registered.")
	}
	client := f.wire(t, handshakeWireOptions{})
	open := func(runID string, first uint64) *openClientStream {
		t.Helper()
		return submitAdmission(t, f, client, m.ID, key, runID, first)
	}
	first := open(ready, 6)
	requireAdmissionWelcome(t, first, 5)
	finishAdmissionPeer(t, first)
	storetest.Exec(t, f.obs, "UPDATE link_machines SET event_mark = 6 WHERE machine_id = $1", m.ID)
	reconnect := open(ready, 6)
	requireAdmissionWelcome(t, reconnect, 6)
	awaitAdmissionPublication(t, f, m.ID)
	head, before := f.head(t), readAdmissionRows(t, f, m.ID)
	requireOpenEnd(t, open(rolledBack, 7), connect.CodeUnavailable, openDatabase, "")
	requireAdmissionUnchanged(t, f, m.ID, head, before)
	next := open(replacement, 7)
	requireAdmissionWelcome(t, next, 6)
	requireOpenEnd(t, reconnect, connect.CodeAborted, openReplaced, "")
	finishAdmissionPeer(t, next)
	waiting := open(pending, 10)
	requireAdmissionWelcome(t, waiting, 6)
	finishAdmissionPeer(t, waiting)

	want := []string{
		"admit/" + ready + "//6/true",
		"ready/" + ready + "//" + ready + "/6",
		"admit/" + ready + "/" + ready + "/6/true",
		"admit/" + replacement + "/" + ready + "/7/true",
		"ready/" + replacement + "/" + ready + "/" + replacement + "/7",
		"admit/" + pending + "/" + replacement + "/10/false",
	}
	got := storetest.Rows[string](t, f.obs, `SELECT event FROM fixture_admission_hooks ORDER BY seq`)
	if !slices.Equal(got, want) {
		t.Error("Admission and ready callbacks did not commit in order with the expected state and rollback.")
	}
	if !storetest.Scalar[bool](t, f.obs, `SELECT count(*) = 2 AND count(DISTINCT xmin::text) = 1
		FROM (SELECT xmin FROM fixture_admission_hooks ORDER BY seq LIMIT 2) first_admission`) {
		t.Error("The first ready callback and its admission callback committed in different writes.")
	}
}

func submitAdmission(t *testing.T, f *handshakeFixture, client linkv1connect.LinkServiceClient, machineID string, key ed25519.PrivateKey, runID string, first uint64) *openClientStream {
	t.Helper()
	s := startHandshake(t, client, nil)
	hello := helloFrame(machineID, admissionRevision, 1)
	hello.GetHello().RunId = runID
	hello.GetHello().FirstEventNumber = first
	sendHandshake(t, s, hello)
	challenge := requireChallenge(t, s, f.serverKey, hello.GetHello())
	sendHandshake(t, s, proofFrame(key, hello.GetHello(), challenge))
	return s
}

func seedAdmissionMark(t *testing.T, f *handshakeFixture, machineID string, mark uint64) {
	t.Helper()
	storetest.Exec(t, f.obs, "INSERT INTO link_machines (machine_id, event_mark) VALUES ($1, $2)", machineID, int64(mark))
}

func requireAdmissionWelcome(t *testing.T, s *openClientStream, mark uint64) {
	t.Helper()
	w := requireWelcome(t, s)
	if w.GetLinkRevision() != admissionRevision || w.GetLastEventNumber() != mark {
		t.Error("Welcome did not carry revision 2 and the committed event mark.")
	}
}

func awaitAdmissionPublication(t *testing.T, f *handshakeFixture, machineID string) {
	t.Helper()
	streamID := storetest.Scalar[string](t, f.obs, `SELECT stream_id FROM link_connections WHERE machine_id = $1`, machineID)
	awaitHeld(t, f.hub, machineID, streamID)
}

func requireAdmissionRun(t *testing.T, f *handshakeFixture, machineID, admittedID, readyID string, first uint64) {
	t.Helper()
	if !storetest.Scalar[bool](t, f.obs, `SELECT EXISTS (
		SELECT FROM link_machines m JOIN link_runs r ON r.machine_id = m.machine_id AND r.run_id = m.admitted_run_id
		WHERE m.machine_id = $1 AND r.run_id = $2 AND COALESCE(m.ready_run_id, '') = $3 AND r.replay_through = $4)`,
		machineID, admittedID, readyID, int64(first-1)) {
		t.Fatal("The durable admitted pair or ready run did not match the expected history.")
	}
}

func requireAdmissionHistory(t *testing.T, f *handshakeFixture, machineID string, count int64) {
	t.Helper()
	if got := storetest.Scalar[int64](t, f.obs, "SELECT count(*) FROM link_runs WHERE machine_id = $1", machineID); got != count {
		t.Errorf("The machine retained %d admitted runs. Want %d.", got, count)
	}
}

func readAdmissionRows(t *testing.T, f *handshakeFixture, machineID string) string {
	t.Helper()
	return storetest.Scalar[string](t, f.obs, `SELECT jsonb_build_object(
		'machine', (SELECT to_jsonb(m) FROM link_machines m WHERE machine_id = $1),
		'connection', (SELECT to_jsonb(c) FROM link_connections c WHERE machine_id = $1),
		'runs', (SELECT jsonb_agg(to_jsonb(r) ORDER BY run_id) FROM link_runs r WHERE machine_id = $1),
		'commands', (SELECT jsonb_agg(to_jsonb(c) ORDER BY seq) FROM link_commands c WHERE machine_id = $1)
	)::text`, machineID)
}

func requireAdmissionUnchanged(t *testing.T, f *handshakeFixture, machineID string, head int64, before string) {
	t.Helper()
	if readAdmissionRows(t, f, machineID) != before {
		t.Error("The refused admission changed the durable machine, connection, run history or command ledger.")
	}
	f.requireSaved(t, head)
}

func restartAdmissionFixture(t *testing.T, f *handshakeFixture) *handshakeFixture {
	t.Helper()
	f.s.Close()
	cfg, err := pgxpool.ParseConfig("")
	if err != nil {
		t.Fatal("The reopened test store could not initialize its configuration.")
	}
	cfg.ConnConfig = f.obs.Config().Copy()
	s, err := store.Open(t.Context(), cfg, store.ConnectionPolicy{})
	if err != nil {
		t.Fatal("The test store could not reopen the same database.")
	}
	t.Cleanup(s.Close)
	r, hub := machines.NewRegistry(), NewHub()
	state := NewState(s, r, hub)
	lock, err := s.AcquireServerInstance(t.Context(), storetest.ObservationTimeout)
	if err != nil {
		t.Fatal("The restarted process could not take the server instance lock.")
	}
	t.Cleanup(func() { _ = lock.Close(context.Background()) })
	if err := state.ClearConnections(t.Context()); err != nil {
		t.Fatal("The restarted process could not clear the old connections.")
	}
	_ = lock.Close(t.Context())
	tokens := machines.NewTokens(s, r, machines.Issuer{PublicURL: "http://127.0.0.1:8787", ServerKey: f.serverKey})
	svc := NewStreamingService(t.Context(), s, tokens, r, state, hub, f.svc.streaming.key, f.svc.streaming.options)
	return &handshakeFixture{presenceFixture: &presenceFixture{s: s, r: r, obs: f.obs, state: state, hub: hub},
		svc: svc, serverKey: f.serverKey}
}

func finishAdmissionPeer(t *testing.T, s *openClientStream) {
	t.Helper()
	if err := s.CloseRequest(); err != nil {
		t.Fatal("The scripted peer could not close its request stream.")
	}
	if got := receiveOpenFrame(s); !errors.Is(got.err, io.EOF) {
		t.Fatalf("The admitted stream did not finish normally. The receive code was %v.", connect.CodeOf(got.err))
	}
}
