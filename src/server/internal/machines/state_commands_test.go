package machines_test

import (
	"context"
	"crypto/ed25519"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/api"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

type stateQueueFixture struct {
	*link.State
	reject atomic.Bool
}

func (q *stateQueueFixture) Enqueue(ctx context.Context, tx *store.Tx, id string, action *linkv1.Command, ttl time.Duration) error {
	if err := q.State.Enqueue(ctx, tx, id, action, ttl); err != nil {
		return err
	}
	if q.reject.Load() {
		return errors.New("The fixture refused the notice after its insert.")
	}
	return nil
}

type stateCommitGate struct {
	armed   atomic.Bool
	writer  atomic.Uint32
	arrived chan int32
	hold    <-chan struct{}
	release func()
}

func newStateCommitGate(t *testing.T) *stateCommitGate {
	t.Helper()
	hold, release := storetest.Signal(t)
	return &stateCommitGate{arrived: make(chan int32, 1), hold: hold, release: release}
}

func (g *stateCommitGate) TraceQueryStart(ctx context.Context, conn *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
	sql := strings.ToLower(strings.TrimSpace(q.SQL))
	pid := conn.PgConn().PID()
	if g.armed.Load() && strings.HasPrefix(sql, "insert into receipts") {
		g.writer.Store(pid)
	}
	if sql == "commit" && g.writer.Load() == pid && g.armed.CompareAndSwap(true, false) {
		g.arrived <- int32(pid)
		select {
		case <-g.hold:
		case <-ctx.Done():
		}
	}
	return ctx
}

func (*stateCommitGate) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

type stateCommandsFixture struct {
	*streamRecordsFixture
	queue   *stateQueueFixture
	old     lgtmv1connect.ClientServiceClient
	project atomic.Bool
}

func newStateCommandsFixture(t *testing.T, tracer pgx.QueryTracer) *stateCommandsFixture {
	t.Helper()
	cfg := storetest.NewDatabase(t, "public")
	obs, err := pgx.ConnectConfig(t.Context(), cfg.ConnConfig.Copy())
	if err != nil {
		t.Fatal("The independent notice observer could not connect.")
	}
	t.Cleanup(func() { _ = obs.Close(context.Background()) })
	cfg.ConnConfig.Tracer = tracer
	s, err := store.Open(t.Context(), cfg, store.ConnectionPolicy{})
	if err != nil {
		t.Fatal("The notice fixture store could not open.")
	}
	t.Cleanup(s.Close)
	if s.Migrate(t.Context(), machines.Migrations(), auth.Migrations(), link.Migrations()) != nil {
		t.Fatal("The notice fixture could not migrate.")
	}
	reg := machines.NewRegistry()
	public := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	tokens := machines.NewTokens(s, reg, machines.Issuer{PublicURL: recordsIssuerURL, ServerKey: public})
	password, err := auth.NewPassword("state notice fixture password")
	if err != nil {
		t.Fatal("The notice fixture password could not be initialized.")
	}
	sessions := auth.NewSessions(s, password)
	cookie, err := sessions.Issue(t.Context())
	if err != nil {
		t.Fatal("The notice fixture session could not be issued.")
	}
	origins, err := auth.NewOriginPolicy(recordsIssuerURL, nil)
	if err != nil {
		t.Fatal("The notice fixture origin is invalid.")
	}
	hub := link.NewHub()
	state := link.NewState(s, reg, hub)
	queue := &stateQueueFixture{State: state}
	fixture := &stateCommandsFixture{queue: queue}
	presence := &projectionLink{State: state, load: func(ctx context.Context, tx machines.Querier, ids []string) (map[string]machines.Presence, error) {
		if fixture.project.Load() {
			return nil, errors.New("The fixture refused the approval projection.")
		}
		return state.Presence(ctx, tx, ids)
	}}
	base := machines.NewService(s, sessions, reg, tokens).WithLink(presence)
	queued := base.WithCommands(queue)
	if queued == nil {
		t.Fatal("Attaching the notice queue did not yield a Machines service.")
	}
	newRoutes := func(svc *machines.Service) *httptest.Server {
		handler := api.NewHandler(api.Options{Origins: origins, Sessions: sessions, Clients: svc, Link: link.NewService(s, tokens, public)})
		server := httptest.NewServer(handler)
		t.Cleanup(server.Close)
		return server
	}
	legacy := newRoutes(machines.NewService(s, sessions, reg, tokens).WithLink(presence))
	server := newRoutes(queued)
	client := server.Client()
	client.Timeout = storetest.ObservationTimeout
	fixture.old = lgtmv1connect.NewClientServiceClient(legacy.Client(), legacy.URL)
	fixture.streamRecordsFixture = &streamRecordsFixture{
		recordsFixture: &recordsFixture{
			regEnv: &regEnv{s: s, reg: reg}, tokens: tokens, http: client, url: server.URL,
			clients: lgtmv1connect.NewClientServiceClient(client, server.URL),
			link:    linkv1connect.NewLinkServiceClient(client, server.URL),
			cookie:  &http.Cookie{Name: auth.CookieName, Value: cookie},
		}, obs: obs, hub: hub, presence: presence,
	}
	return fixture
}

func (f *stateCommandsFixture) digest(t *testing.T) string {
	t.Helper()
	return storetest.Scalar[string](t, f.obs, `SELECT md5(jsonb_build_array(
		(SELECT jsonb_agg(to_jsonb(m) ORDER BY id) FROM machines m),
		(SELECT jsonb_agg(to_jsonb(c) ORDER BY seq) FROM link_commands c),
		(SELECT jsonb_agg(to_jsonb(r) ORDER BY principal_hash, request_key) FROM receipts r),
		(SELECT jsonb_agg(to_jsonb(f) ORDER BY position) FROM feed f)
	)::text)`)
}

func (f *stateCommandsFixture) noticeCount(t *testing.T, id string) int {
	t.Helper()
	return storetest.Scalar[int](t, f.obs, "SELECT count(*) FROM link_commands WHERE machine_id = $1", id)
}

func (f *stateCommandsFixture) requireNotice(t *testing.T, id string, offset int, name string, revision uint64) {
	t.Helper()
	var payload []byte
	var initial bool
	err := f.obs.QueryRow(t.Context(), `SELECT payload, action = 'set_state' AND attempt = 1
		AND sent_stream_id IS NULL AND NOT dispatched AND outcome IS NULL AND finished_at IS NULL
		AND expires_at - created_at = interval '1 hour' FROM link_commands
		WHERE machine_id = $1 ORDER BY seq OFFSET $2 LIMIT 1`, id, offset).Scan(&payload, &initial)
	if err != nil || !initial {
		t.Error("The state change did not save the expected one-hour, unassigned notice.")
		return
	}
	want := &linkv1.Command{Action: &linkv1.Command_SetState{SetState: &linkv1.SetState{
		State: &linkv1.MachineState{Approval: linkv1.Approval_APPROVAL_APPROVED, Name: name, Revision: revision},
	}}}
	var got linkv1.Command
	if proto.Unmarshal(payload, &got) != nil || !proto.Equal(&got, want) {
		t.Error("The notice did not carry exactly the approved name and revision.")
	}
}

func (f *stateCommandsFixture) requireWakes(t *testing.T, head int64, machine string) {
	t.Helper()
	if !storetest.Scalar[bool](t, f.obs, `SELECT count(*) = 2
		AND count(*) FILTER (WHERE topic = 'machines' AND scope = '' AND key = $2 AND change = 'saved') = 1
		AND count(*) FILTER (WHERE topic = 'machine_outbox' AND scope = '' AND key = $2 AND change = 'saved') = 1
		FROM feed WHERE position > $1`, head, machine) {
		t.Error("The state change did not write exactly one machines and one outbox reference.")
	}
}

func (f *stateCommandsFixture) approveNamed(t *testing.T, client lgtmv1connect.ClientServiceClient, id, name, key string) (*webv1.ApproveClientResponse, error) {
	t.Helper()
	r, err := client.ApproveClient(t.Context(), recordRequest(f.recordsFixture, &webv1.ApproveClientRequest{Id: id, Name: name}, key))
	if err != nil {
		return nil, err
	}
	return r.Msg, nil
}

func (f *stateCommandsFixture) renameNamed(t *testing.T, client lgtmv1connect.ClientServiceClient, id, name, key string) error {
	t.Helper()
	r, err := client.RenameClient(t.Context(), recordRequest(f.recordsFixture, &webv1.RenameClientRequest{Id: id, Name: name}, key))
	if err == nil && (r == nil || !proto.Equal(r.Msg, &webv1.RenameClientResponse{})) {
		return errors.New("Rename did not return its empty response.")
	}
	return err
}

func stateCommitPID(t *testing.T, g *stateCommitGate, task *storetest.Task[error]) int32 {
	t.Helper()
	select {
	case pid := <-g.arrived:
		return pid
	case <-task.Done():
		t.Fatal("The owner route returned before reaching the held commit.")
	case <-time.After(storetest.ObservationTimeout):
		t.Fatal("The owner route did not reach its real commit.")
	}
	return 0
}

func TestApproveQueuesStateAtomically(t *testing.T) {
	t.Run("enqueue and projection failures", func(t *testing.T) {
		f := newStateCommandsFixture(t, nil)
		id := f.join(t, f.issue(t, false))
		before := f.digest(t)
		f.queue.reject.Store(true)
		_, err := f.approveNamed(t, f.clients, id, "  Approved station  ", "reject-approval-enqueue")
		if connect.CodeOf(err) != connect.CodeUnavailable || f.digest(t) != before {
			t.Error("A queue failure did not preserve the waiting record, queue, feed and receipts.")
		}
		f.queue.reject.Store(false)
		f.project.Store(true)
		_, err = f.approveNamed(t, f.clients, id, "  Approved station  ", "reject-approval-projection")
		if connect.CodeOf(err) != connect.CodeUnavailable || f.digest(t) != before {
			t.Error("A projection failure did not roll back approval, notice, feed and receipt.")
		}
	})

	t.Run("rejected real commit", func(t *testing.T) {
		gate := newStateCommitGate(t)
		f := newStateCommandsFixture(t, gate)
		id := f.join(t, f.issue(t, false))
		before := f.digest(t)
		gate.armed.Store(true)
		call := storetest.Start(t, func() error {
			_, err := f.approveNamed(t, f.clients, id, "  Approved station  ", "reject-approval-commit")
			return err
		})
		defer gate.release()
		pid := stateCommitPID(t, gate, call)
		if f.digest(t) != before {
			t.Error("Approval, its notice, feed or receipt became visible before the commit.")
		}
		if !storetest.Scalar[bool](t, f.obs, "SELECT pg_terminate_backend($1, 1000)", pid) {
			t.Fatal("PostgreSQL did not terminate the held approval transaction.")
		}
		gate.release()
		if connect.CodeOf(call.Wait(t)) != connect.CodeUnavailable || f.digest(t) != before {
			t.Error("The rejected approval commit did not preserve waiting standing, the queue, feed and receipts.")
		}
		if _, err := f.approveNamed(t, f.clients, id, "Approved station", "reject-approval-commit"); err != nil || f.noticeCount(t, id) != 1 {
			t.Error("Retrying the rolled-back approval did not commit one notice.")
		}
	})

	t.Run("lost reply replays after rename and removal", func(t *testing.T) {
		f := newStateCommandsFixture(t, nil)
		id := f.join(t, f.issue(t, false))
		f.admit(t, id, 1)
		head := f.head(t)
		var discarded bool
		drop := &http.Client{Timeout: storetest.ObservationTimeout, Transport: recordRoundTripper(func(req *http.Request) (*http.Response, error) {
			response, err := http.DefaultTransport.RoundTrip(req)
			if err != nil {
				return nil, err
			}
			defer response.Body.Close()
			_, err = io.Copy(io.Discard, response.Body)
			discarded = err == nil && response.StatusCode == http.StatusOK
			return nil, errors.New("The fixture discarded the approval reply.")
		})}
		const key = "lost-state-approval"
		_, err := f.approveNamed(t, lgtmv1connect.NewClientServiceClient(drop, f.url), id, "  Approved station  ", key)
		if err == nil || !discarded {
			t.Fatal("The fixture did not discard a successful approval reply.")
		}
		var saved []byte
		if f.obs.QueryRow(t.Context(), "SELECT reply FROM receipts WHERE request_key = $1", key).Scan(&saved) != nil {
			t.Fatal("The successful approval did not store its reply.")
		}
		var original webv1.ApproveClientResponse
		if proto.Unmarshal(saved, &original) != nil || original.GetClient().GetId() != id ||
			original.GetClient().GetName() != "Approved station" || original.GetClient().GetTokenId() != "" ||
			original.GetClient().GetState() != webv1.ClientState_CLIENT_STATE_JOINING {
			t.Error("The receipt did not contain the normalized approved projection from the live stream.")
		}
		if !storetest.Scalar[bool](t, f.obs, "SELECT name = 'Approved station' AND approval = 'approved' AND state_revision = 2 FROM machines WHERE id = $1", id) ||
			f.noticeCount(t, id) != 1 {
			t.Error("Approval did not save the approved revision and exactly one notice.")
		}
		f.requireNotice(t, id, 0, "Approved station", 2)
		f.requireWakes(t, head, id)
		if f.renameNamed(t, f.clients, id, "Later station", "later-state-rename") != nil {
			t.Fatal("The station could not be renamed after its lost approval reply.")
		}
		if _, err := f.clients.RemoveClient(t.Context(), recordRequest(f.recordsFixture, &webv1.RemoveClientRequest{Id: id}, "later-state-removal")); err != nil {
			t.Fatal("The station could not be removed after its lost approval reply.")
		}
		before := f.digest(t)
		replay, err := f.approveNamed(t, f.clients, id, "Approved station", key)
		if err != nil || !proto.Equal(replay, &original) || f.digest(t) != before {
			t.Error("Replaying normalized approval after removal did not return the saved projection without a new notice.")
		}
		_, err = f.approveNamed(t, f.clients, id, "Different station", key)
		if connect.CodeOf(err) != connect.CodeAlreadyExists || f.digest(t) != before {
			t.Error("A different approval body did not conflict with the saved request.")
		}
	})
}

func TestRenameQueuesStateOnce(t *testing.T) {
	t.Run("changed unchanged and replayed names", func(t *testing.T) {
		f := newStateCommandsFixture(t, nil)
		id := f.join(t, f.issue(t, false))
		if _, err := f.approveNamed(t, f.old, id, "Original station", "legacy-approval"); err != nil {
			t.Fatal("The record-only service could not approve the fixture.")
		}
		const firstKey = "queued-first-rename"
		head := f.head(t)
		if f.renameNamed(t, f.clients, id, "  First rename  ", firstKey) != nil || f.noticeCount(t, id) != 1 {
			t.Error("Changing the approved name did not enqueue exactly one notice.")
		}
		f.requireNotice(t, id, 0, "First rename", 3)
		f.requireWakes(t, head, id)
		head = f.head(t)
		if f.renameNamed(t, f.clients, id, "First rename", "unchanged-rename") != nil ||
			f.noticeCount(t, id) != 1 || f.head(t) != head ||
			storetest.Scalar[int](t, f.obs, "SELECT state_revision FROM machines WHERE id = $1", id) != 3 {
			t.Error("An unchanged name advanced the revision or wrote a notice or feed reference.")
		}
		if f.renameNamed(t, f.clients, id, "Second rename", "second-rename") != nil {
			t.Fatal("The second rename did not succeed.")
		}
		f.requireNotice(t, id, 1, "Second rename", 4)
		const oldKey = "part-a-rename"
		if f.renameNamed(t, f.old, id, "Part A rename", oldKey) != nil {
			t.Fatal("The record-only service could not save a version-one rename receipt.")
		}
		if f.renameNamed(t, f.clients, id, "Last rename", "last-rename") != nil {
			t.Fatal("The last rename did not succeed.")
		}
		f.requireNotice(t, id, 2, "Last rename", 6)
		before := f.digest(t)
		if f.renameNamed(t, f.clients, id, "First rename", firstKey) != nil ||
			f.renameNamed(t, f.clients, id, "Part A rename", oldKey) != nil || f.digest(t) != before {
			t.Error("A saved rename did not replay its empty reply without changing the current name or queue.")
		}
		if connect.CodeOf(f.renameNamed(t, f.clients, id, "Different rename", firstKey)) != connect.CodeAlreadyExists || f.digest(t) != before {
			t.Error("Reusing a rename key with a different body did not conflict without writes.")
		}
		if _, err := f.clients.RemoveClient(t.Context(), recordRequest(f.recordsFixture, &webv1.RemoveClientRequest{Id: id}, "remove-renamed-station")); err != nil {
			t.Fatal("The renamed fixture could not be removed.")
		}
		before = f.digest(t)
		if f.renameNamed(t, f.clients, id, "First rename", firstKey) != nil || f.digest(t) != before {
			t.Error("A saved rename did not replay after removal without writing a notice.")
		}
	})

	t.Run("waiting missing and failed queue", func(t *testing.T) {
		f := newStateCommandsFixture(t, nil)
		id := f.join(t, f.issue(t, false))
		before := f.digest(t)
		if connect.CodeOf(f.renameNamed(t, f.clients, id, "Waiting rename", "waiting-rename")) != connect.CodeFailedPrecondition ||
			connect.CodeOf(f.renameNamed(t, f.clients, unknownRecordID, "Missing rename", "missing-rename")) != connect.CodeNotFound ||
			f.digest(t) != before {
			t.Error("A waiting or missing machine did not refuse rename without writes.")
		}
		if _, err := f.approveNamed(t, f.old, id, "Original station", "legacy-approval"); err != nil {
			t.Fatal("The record-only service could not approve the fixture.")
		}
		before = f.digest(t)
		f.queue.reject.Store(true)
		if connect.CodeOf(f.renameNamed(t, f.clients, id, "Refused rename", "failed-rename")) != connect.CodeUnavailable ||
			f.digest(t) != before {
			t.Error("A failed rename notice did not roll back the name, revision, feed and receipt.")
		}
	})

	t.Run("rename waits for approval", func(t *testing.T) {
		gate := newStateCommitGate(t)
		f := newStateCommandsFixture(t, gate)
		id := f.join(t, f.issue(t, false))
		gate.armed.Store(true)
		approve := storetest.Start(t, func() error {
			_, err := f.approveNamed(t, f.clients, id, "Approved station", "held-state-approval")
			return err
		})
		defer gate.release()
		pid := stateCommitPID(t, gate, approve)
		rename := storetest.Start(t, func() error {
			return f.renameNamed(t, f.clients, id, "Renamed station", "competing-state-rename")
		})
		defer gate.release()
		storetest.Poll(t, "the owner's rename waiting on approval's machine lock", func() bool {
			select {
			case <-rename.Done():
				t.Fatal("Rename returned before the concurrent approval released its machine lock.")
			default:
			}
			return storetest.Scalar[bool](t, f.obs, `SELECT EXISTS (
				SELECT FROM pg_stat_activity WHERE datname = current_database()
				AND wait_event_type = 'Lock' AND lower(query) LIKE '%for no key update%'
				AND $1 = ANY (pg_blocking_pids(pid)))`, pid)
		})
		if f.noticeCount(t, id) != 0 {
			t.Error("A notice became visible before the approval released its machine lock.")
		}
		gate.release()
		if approve.Wait(t) != nil || rename.Wait(t) != nil || f.noticeCount(t, id) != 2 {
			t.Error("The owner routes did not commit one notice each in approval-then-rename order.")
		}
		f.requireNotice(t, id, 0, "Approved station", 2)
		f.requireNotice(t, id, 1, "Renamed station", 3)
	})
}
