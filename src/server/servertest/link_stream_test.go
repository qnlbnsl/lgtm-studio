package servertest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptrace"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/config"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link/machinetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/servertest"
)

type fixtureLinkLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *fixtureLinkLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *fixtureLinkLog) entries() []linkLogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	var entries []linkLogEntry
	for line := range strings.SplitSeq(l.buf.String(), "\n") {
		if line == "" {
			continue
		}
		var entry linkLogEntry
		if json.Unmarshal([]byte(line), &entry) == nil {
			entries = append(entries, entry)
		}
	}
	return entries
}

type linkLogEntry struct {
	Message    string `json:"msg"`
	Machine    string `json:"machine_id"`
	Generation int64  `json:"generation"`
	Sequence   uint64 `json:"sequence"`
	Code       string `json:"code"`
}

func fixtureLinkObserver(t *testing.T, server *servertest.Server) *pgx.Conn {
	t.Helper()
	env := server.Environment()
	cfg, err := config.Load(func(key string) (string, bool) { value, ok := env[key]; return value, ok })
	if err != nil {
		t.Fatal("The running fixture's database settings could not be loaded.")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	obs, err := pgx.ConnectConfig(ctx, cfg.Postgres.ConnConfig.Copy())
	if err != nil {
		t.Fatal("The fixture's independent database observer could not connect.")
	}
	t.Cleanup(func() { _ = obs.Close(context.Background()) })
	return obs
}

func TestLinkSeamsReachTheService(t *testing.T) {
	opts := servertest.Options{Password: "fixture stream password"}
	options := reflect.ValueOf(&opts).Elem()
	seams, output := options.FieldByName("LinkSeams"), options.FieldByName("Log")
	if !seams.IsValid() || !output.IsValid() {
		t.Fatal("The fixture did not expose the planned link seams and log sink.")
	}
	logs := new(fixtureLinkLog)
	output.Set(reflect.ValueOf(io.Writer(logs)))
	type ackBarrier struct {
		ctx     context.Context
		id      string
		mark    uint64
		release chan struct{}
	}
	proofs, admissions := make(chan string, 4), make(chan string, 4)
	acks := make(chan ackBarrier, 4)
	seams.FieldByName("AfterProof").Set(reflect.ValueOf(func(_ context.Context, id string) { proofs <- id }))
	seams.FieldByName("AfterAdmission").Set(reflect.ValueOf(func(_ context.Context, id string) { admissions <- id }))
	seams.FieldByName("BeforeEventAck").Set(reflect.ValueOf(func(ctx context.Context, id string, mark uint64) {
		barrier := ackBarrier{ctx: ctx, id: id, mark: mark, release: make(chan struct{})}
		select {
		case acks <- barrier:
		case <-t.Context().Done():
			return
		}
		select {
		case <-barrier.release:
		case <-t.Context().Done():
		}
	}))
	server := servertest.Start(t, opts)
	clients := server.Clients(t)
	reply, err := clients.CreateClientToken(t.Context(), connect.NewRequest(&webv1.CreateClientTokenRequest{}))
	if err != nil {
		t.Fatal("The signed-in fixture could not issue a join token.")
	}
	machine, err := machinetest.Join(t.Context(), server.Link(), reply.Msg.GetToken(), "seams machine")
	if err != nil || machine == nil {
		t.Fatal("The scripted machine could not join the fixture.")
	}
	if _, err := clients.ApproveClient(t.Context(), connect.NewRequest(&webv1.ApproveClientRequest{Id: machine.ID, Name: "Seams machine"})); err != nil {
		t.Fatal("The signed-in fixture could not approve the scripted machine.")
	}
	observer := fixtureLinkObserver(t, server)
	storetest.Exec(t, observer, `CREATE TABLE fixture_link_report_audit (prior bigint NOT NULL);
		CREATE FUNCTION fixture_link_report_written() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			INSERT INTO fixture_link_report_audit(prior)
			SELECT event_mark FROM link_machines WHERE machine_id = NEW.id;
			RETURN NEW;
		END $$;
		CREATE TRIGGER fixture_link_report_written AFTER UPDATE OF facts ON machines
			FOR EACH ROW EXECUTE FUNCTION fixture_link_report_written()`)
	var sentCookie atomic.Bool
	for mark := uint64(1); mark <= 2; mark++ {
		if mark == 2 {
			server.Restart(t)
			server.Clients(t)
		}
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()
		ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{WroteHeaderField: func(field string, _ []string) {
			if strings.EqualFold(field, "Cookie") {
				sentCookie.Store(true)
			}
		}})
		stream, welcome, err := machine.Connect(ctx, server.Link())
		if err != nil || stream == nil || welcome == nil || welcome.LastEventNumber != mark-1 {
			t.Fatal("The fixture stream did not open at its committed mark.")
		}
		t.Cleanup(func() { _ = stream.CloseResponse(); _ = stream.CloseRequest() })
		if sentCookie.Load() {
			t.Fatal("The signed-in fixture sent a website session cookie on Open.")
		}
		if storetest.Receive(t, "AfterProof reaching the fixture", proofs) != machine.ID ||
			storetest.Receive(t, "AfterAdmission reaching the fixture", admissions) != machine.ID {
			t.Fatal("A handshake seam reported a different machine.")
		}
		if mark == 2 {
			repeat := &linkv1.Event{Number: 1,
				Report: &linkv1.Event_Facts{Facts: &linkv1.Facts{Os: "Replay sentinel", Cores: 1}}}
			if err := stream.Send(&linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Event{Event: repeat}}); err != nil {
				t.Fatal("The fixture could not replay the report that committed before the cut.")
			}
			type repeatResponse struct {
				ack uint64
				err error
			}
			replay := storetest.Start(t, func() repeatResponse {
				r, err := stream.Receive()
				return repeatResponse{r.GetEventAck().GetThrough(), err}
			})
			var answer repeatResponse
			select {
			case <-replay.Done():
				answer = replay.Result()
			case b := <-acks:
				close(b.release)
				t.Fatal("A repeated report invoked the mark-advancing acknowledgement seam.")
			case <-ctx.Done():
				t.Fatal("A repeated report did not finish before the request deadline.")
			}
			if answer.err != nil || answer.ack != 1 {
				t.Fatal("A repeated report did not receive the previous committed mark.")
			}
		}
		event := &linkv1.Event{Number: mark, Report: &linkv1.Event_Facts{Facts: &linkv1.Facts{Os: "Fixture report", Cores: 1}}}
		if err := stream.Send(&linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Event{Event: event}}); err != nil {
			t.Fatal("The fixture could not send its event.")
		}
		type answer struct {
			frame *linkv1.OpenResponse
			err   error
		}
		read := storetest.Start(t, func() answer { r, e := stream.Receive(); return answer{r, e} })
		barrier := storetest.Receive(t, "the committed acknowledgement barrier", acks)
		var release sync.Once
		resume := func() { release.Do(func() { close(barrier.release) }) }
		defer resume()
		audit := storetest.Rows[int64](t, observer, "SELECT prior FROM fixture_link_report_audit ORDER BY prior")
		if len(audit) != int(mark) || audit[len(audit)-1] != int64(mark-1) {
			t.Fatal("The independent report audit did not contain one effect per committed event.")
		}
		if barrier.id != machine.ID || barrier.mark != mark ||
			storetest.Scalar[int64](t, observer, "SELECT event_mark FROM link_machines WHERE machine_id=$1", machine.ID) != int64(mark) {
			t.Fatal("The acknowledgement barrier did not report the independently committed machine mark.")
		}
		select {
		case <-read.Done():
			t.Fatal("The fixture received an answer before the acknowledgement seam released.")
		case <-time.After(250 * time.Millisecond):
		}
		if mark == 1 {
			cancel()
			storetest.Receive(t, "the acknowledgement barrier seeing the stream cancellation", barrier.ctx.Done())
			resume()
			r := read.Wait(t)
			if r.frame != nil || r.err == nil {
				t.Fatal("The cut stream received an acknowledgement after its cancellation.")
			}
		} else {
			resume()
			r := read.Wait(t)
			if r.err != nil || r.frame.GetEventAck().GetThrough() != mark {
				t.Fatal("The fixture did not receive the acknowledgement after restart and seam release.")
			}
			_ = stream.CloseRequest()
		}
		storetest.Poll(t, "the fixture capturing the opened, applied and closed diagnostics", func() bool {
			var opened, applied, closed int
			for _, entry := range logs.entries() {
				if entry.Machine != machine.ID {
					continue
				}
				switch entry.Message {
				case "link_stream_opened":
					if entry.Generation > 0 {
						opened++
					}
				case "link_event_applied":
					if entry.Sequence > 0 && entry.Sequence <= mark {
						applied++
					}
				case "link_stream_closed":
					if entry.Code != "" {
						closed++
					}
				}
			}
			return opened == int(mark) && applied == int(mark) && closed == int(mark)
		})
	}
}
