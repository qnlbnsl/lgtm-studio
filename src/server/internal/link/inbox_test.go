package link

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func factsEvent(n uint64, os string) *linkv1.Event {
	return &linkv1.Event{Number: n, Report: &linkv1.Event_Facts{Facts: &linkv1.Facts{Os: os, Cores: 1}}}
}

func harnessEvent(n uint64, entries ...*linkv1.Harness) *linkv1.Event {
	return &linkv1.Event{Number: n, Report: &linkv1.Event_HarnessesFound{HarnessesFound: &linkv1.HarnessesFound{Harnesses: entries}}}
}

func eventSQL(t *testing.T, f *presenceFixture, query string, args ...any) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	if _, err := f.obs.Exec(ctx, query, args...); err != nil {
		t.Fatalf("The event database setup failed (%T).", err)
	}
}

func eventMark(t *testing.T, f *presenceFixture, id string) uint64 {
	t.Helper()
	return uint64(storetest.Scalar[int64](t, f.obs, "SELECT event_mark FROM link_machines WHERE machine_id = $1", id))
}

func requireEvent(t *testing.T, f *presenceFixture, id, stream string, event *linkv1.Event, mark uint64, advanced bool) {
	t.Helper()
	got, err := f.state.ApplyEvent(t.Context(), id, stream, event)
	if err != nil || got != (EventResult{Mark: mark, Applied: advanced}) {
		t.Fatalf("Event %d did not return mark %d and applied=%t.", event.Number, mark, advanced)
	}
	if eventMark(t, f, id) != mark {
		t.Fatal("The returned event mark differs from the committed mark.")
	}
}

func installReportAudit(t *testing.T, f *presenceFixture) {
	t.Helper()
	eventSQL(t, f, `
		CREATE TABLE event_test_audit (ordinal bigint GENERATED ALWAYS AS IDENTITY, prior bigint NOT NULL);
		CREATE FUNCTION event_test_audit_write() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
		  INSERT INTO event_test_audit(prior) SELECT event_mark FROM link_machines WHERE machine_id = NEW.id;
		  RETURN NEW;
		END $$;
		CREATE TRIGGER event_test_audit_write AFTER UPDATE OF facts, harnesses ON machines
		FOR EACH ROW EXECUTE FUNCTION event_test_audit_write()`)
}

func reportAudit(t *testing.T, f *presenceFixture, want ...int64) {
	t.Helper()
	got := storetest.Rows[int64](t, f.obs, "SELECT prior FROM event_test_audit ORDER BY ordinal")
	if !slices.Equal(got, want) {
		t.Fatalf("The committed report audit was %v, want %v.", got, want)
	}
}

func TestEventTable(t *testing.T) {
	invalid := factsEvent(43, "has\x00zero")
	unknown := &linkv1.Event{Number: 43}
	unknown.ProtoReflect().SetUnknown([]byte{0x62, 0x00})
	accepted := factsEvent(43, strings.Repeat("é", 32))
	accepted.GetFacts().Workspace = strings.Repeat("é", 128)
	accepted.GetFacts().ClientVersion = strings.Repeat("x", 64)
	accepted.HappenedAt = &timestamppb.Timestamp{Seconds: math.MaxInt64}
	accepted.GetFacts().ProtoReflect().SetUnknown([]byte{0x78, 0x01})
	zeroCores := factsEvent(43, "fixture")
	zeroCores.GetFacts().Cores = 0
	longWorkspace := factsEvent(43, "fixture")
	longWorkspace.GetFacts().Workspace = strings.Repeat("é", 129)
	longVersion := factsEvent(43, "fixture")
	longVersion.GetFacts().ClientVersion = strings.Repeat("v", 65)
	tooMany := make([]*linkv1.Harness, 17)
	for i := range tooMany {
		tooMany[i] = &linkv1.Harness{Name: "codex"}
	}
	cases := []struct {
		name            string
		mark            uint64
		event           *linkv1.Event
		refusal         error
		advance, effect bool
	}{
		{"zero", 42, factsEvent(0, "valid"), ErrEventNumberZero, false, false},
		{"signed overflow", 42, factsEvent(1<<63, "valid"), ErrEventNumberZero, false, false},
		{"unsigned maximum", 42, factsEvent(math.MaxUint64, "valid"), ErrEventNumberZero, false, false},
		{"invalid repeat", 42, factsEvent(42, "\x00"), nil, false, false},
		{"invalid gap", 42, factsEvent(44, "\x00"), ErrEventGap, false, false},
		{"ceiling repeat", math.MaxInt64, factsEvent(math.MaxInt64, "\x00"), nil, false, false},
		{"below ceiling repeat", math.MaxInt64, factsEvent(math.MaxInt64-1, "valid"), nil, false, false},
		{"advance to ceiling", math.MaxInt64 - 1, factsEvent(math.MaxInt64, "valid"), nil, true, true},
		{"missing report", 42, &linkv1.Event{Number: 43}, nil, true, false},
		{"unknown report", 42, unknown, nil, true, false},
		{"zero byte", 42, invalid, nil, true, false},
		{"zero cores", 42, zeroCores, nil, true, false},
		{"long operating system", 42, factsEvent(43, strings.Repeat("é", 33)), nil, true, false},
		{"long client version", 42, longVersion, nil, true, false},
		{"long workspace", 42, longWorkspace, nil, true, false},
		{"too many harnesses", 42, harnessEvent(43, tooMany...), nil, true, false},
		{"unknown harness", 42, harnessEvent(43, &linkv1.Harness{Name: "other"}), nil, true, false},
		{"long harness version", 42, harnessEvent(43, &linkv1.Harness{Name: "codex", Version: proto.String(strings.Repeat("v", 65))}), nil, true, false},
		{"present empty version and repeated name", 42, harnessEvent(43, &linkv1.Harness{Name: "codex", Version: proto.String("")}, &linkv1.Harness{Name: "codex"}), nil, true, true},
		{"decoded limits and discarded timestamp", 42, accepted, nil, true, true},
	}
	f := newPresenceFixture(t, nil)
	m := f.machine(t, bytes.Repeat([]byte{1}, 32), "event table")
	f.approve(t, m.ID)
	installReportAudit(t, f)
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stream := f.admit(t, m.ID, int64(i+1))
			eventSQL(t, f, "UPDATE link_machines SET event_mark = $1 WHERE machine_id = $2", int64(tc.mark), m.ID)
			eventSQL(t, f, "UPDATE machines SET facts = NULL, harnesses = NULL, harnesses_scanned = false WHERE id = $1", m.ID)
			eventSQL(t, f, "TRUNCATE event_test_audit")
			before := f.head(t)
			prior := f.current(t, m.ID).LastHeardAt
			if tc.refusal != nil {
				if _, err := f.state.ApplyEvent(t.Context(), m.ID, stream.ID, tc.event); !errors.Is(err, tc.refusal) {
					t.Fatal("The event was not refused by its number before payload validation.")
				}
				if eventMark(t, f, m.ID) != tc.mark {
					t.Fatal("A refusal moved the mark.")
				}
			} else {
				mark := tc.mark
				if tc.advance {
					mark = tc.event.Number
				}
				requireEvent(t, f, m.ID, stream.ID, tc.event, mark, tc.advance)
			}
			after := f.current(t, m.ID)
			if tc.effect {
				reportAudit(t, f, int64(tc.mark))
				if after.FactsReceived != (tc.event.GetFacts() != nil) || after.HarnessesReceived != (tc.event.GetHarnessesFound() != nil) {
					t.Fatal("The applied event did not set exactly its readiness flag.")
				}
				if tc.event.GetFacts() != nil {
					raw := storetest.Scalar[[]byte](t, f.obs, "SELECT facts FROM machines WHERE id = $1", m.ID)
					want := proto.Clone(tc.event.GetFacts()).(*linkv1.Facts)
					want.ProtoReflect().SetUnknown(nil)
					got := new(linkv1.Facts)
					if proto.Unmarshal(raw, got) != nil || !proto.Equal(got, want) {
						t.Fatal("The stored facts differ from the declared fields.")
					}
				} else {
					raw := storetest.Scalar[[]byte](t, f.obs, "SELECT harnesses FROM machines WHERE id = $1", m.ID)
					got := new(linkv1.HarnessesFound)
					if proto.Unmarshal(raw, got) != nil || !proto.Equal(got, tc.event.GetHarnessesFound()) {
						t.Fatal("The stored inventory lost a repeated name or empty optional version.")
					}
				}
			} else {
				reportAudit(t, f)
				if after.FactsReceived || after.HarnessesReceived || !after.LastHeardAt.Equal(prior) {
					t.Fatal("A non-effect event changed readiness or contact time.")
				}
			}
			if tc.advance {
				f.requireSaved(t, before, m.ID)
			} else if f.head(t) != before {
				t.Fatal("A repeat or refusal published a feed change.")
			}
		})
	}
}

func TestApplyNextEvent(t *testing.T) {
	f := newPresenceFixture(t, nil)
	m := f.machine(t, bytes.Repeat([]byte{1}, 32), "contact")
	f.approve(t, m.ID)
	s := f.admit(t, m.ID, 1)
	installReportAudit(t, f)
	eventSQL(t, f, "UPDATE link_connections SET last_heard_at = '2000-01-01' WHERE machine_id = $1", m.ID)
	before := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
	head := f.head(t)
	requireEvent(t, f, m.ID, s.ID, factsEvent(1, "First"), 1, true)
	f.requireSaved(t, head, m.ID)
	seen := f.current(t, m.ID)
	after := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
	if seen.LastHeardAt.Before(before) || seen.LastHeardAt.After(after) || !seen.FactsReceived {
		t.Fatal("The applied report did not update contact time within the database clock bounds and set readiness.")
	}
	requireEvent(t, f, m.ID, s.ID, factsEvent(1, "Changed"), 1, false)
	requireEvent(t, f, m.ID, s.ID, &linkv1.Event{Number: 2}, 2, true)
	reportAudit(t, f, 0)
	if !f.current(t, m.ID).LastHeardAt.Equal(seen.LastHeardAt) {
		t.Fatal("A repeated or consumed event refreshed contact time.")
	}
	if deleted, err := f.state.Disconnect(t.Context(), m.ID, s.ID); err != nil || !deleted {
		t.Fatal("The stream did not disconnect.")
	}
	if got := storetest.Scalar[time.Time](t, f.obs, "SELECT last_seen_at FROM machines WHERE id = $1", m.ID); !got.Equal(seen.LastHeardAt) {
		t.Fatal("Disconnect did not save the report's last contact time.")
	}
}

type eventCommitKey struct{}

func TestEffectAndMarkCommitTogether(t *testing.T) {
	atCommit := make(chan uint32, 1)
	release, resume := storetest.Signal(t)
	f := newPresenceFixture(t, func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = handshakeQueryTrace(func(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
			if ctx.Value(eventCommitKey{}) == true && strings.EqualFold(data.SQL, "commit") {
				atCommit <- conn.PgConn().PID()
				select {
				case <-release:
				case <-ctx.Done():
				}
			}
			return ctx
		})
	})
	m := f.machine(t, bytes.Repeat([]byte{1}, 32), "atomic event")
	f.approve(t, m.ID)
	s := f.admit(t, m.ID, 1)
	installReportAudit(t, f)
	eventSQL(t, f, `CREATE FUNCTION event_test_reject_mark() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'test mark write rejected'; END $$;
		CREATE TRIGGER event_test_reject_mark BEFORE UPDATE OF event_mark ON link_machines
		FOR EACH ROW EXECUTE FUNCTION event_test_reject_mark()`)
	head := f.head(t)
	unchanged := func() {
		t.Helper()
		if eventMark(t, f, m.ID) != 0 || f.head(t) != head || f.current(t, m.ID).FactsReceived ||
			storetest.Scalar[bool](t, f.obs, "SELECT facts IS NOT NULL FROM machines WHERE id = $1", m.ID) {
			t.Fatal("An uncommitted event changed the machine, mark, readiness or feed.")
		}
		reportAudit(t, f)
	}
	event := factsEvent(1, "Atomic report")
	if _, err := f.state.ApplyEvent(t.Context(), m.ID, s.ID, event); err == nil {
		t.Fatal("Rejecting the mark write did not reject the event.")
	}
	unchanged()
	eventSQL(t, f, "DROP TRIGGER event_test_reject_mark ON link_machines")
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	task := storetest.Start(t, func() error {
		_, err := f.state.ApplyEvent(context.WithValue(ctx, eventCommitKey{}, true), m.ID, s.ID, event)
		return err
	})
	defer func() { resume(); task.Wait(t) }()
	pid := storetest.ReceiveFrom(t, task, "the event's commit boundary", atCommit)
	unchanged()
	if !storetest.Scalar[bool](t, f.obs, "SELECT pg_terminate_backend($1, 5000)", int32(pid)) {
		t.Fatal("PostgreSQL did not terminate the held event transaction.")
	}
	resume()
	if task.Wait(t) == nil {
		t.Fatal("The event succeeded after its backend was terminated before commit.")
	}
	unchanged()
	requireEvent(t, f, m.ID, s.ID, event, 1, true)
	reportAudit(t, f, 0)
	f.requireSaved(t, head, m.ID)
}

func TestWaitingMachineFactsAndReadiness(t *testing.T) {
	f := newPresenceFixture(t, nil)
	m := f.machine(t, bytes.Repeat([]byte{1}, 32), "waiting reports")
	s := f.admit(t, m.ID, 1)
	installReportAudit(t, f)
	requireEvent(t, f, m.ID, s.ID, factsEvent(1, "Waiting facts"), 1, true)
	requireEvent(t, f, m.ID, s.ID, harnessEvent(2), 2, true)
	reportAudit(t, f, 0)
	f.approve(t, m.ID)
	p := f.current(t, m.ID)
	if p.FactsReceived || p.HarnessesReceived {
		t.Fatal("A waiting report counted toward approved readiness.")
	}
	requireEvent(t, f, m.ID, s.ID, harnessEvent(3), 3, true)
	p = f.current(t, m.ID)
	if p.FactsReceived || !p.HarnessesReceived {
		t.Fatal("Fresh inventory reused waiting facts as approved readiness.")
	}
	requireEvent(t, f, m.ID, s.ID, factsEvent(4, "Approved facts"), 4, true)
	p = f.current(t, m.ID)
	if !p.FactsReceived || !p.HarnessesReceived {
		t.Fatal("Two fresh approved reports did not make the stream ready.")
	}
	reportAudit(t, f, 0, 2, 3)
}

func TestEventFormsOneLine(t *testing.T) {
	for _, admissionFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("admission first %t", admissionFirst), func(t *testing.T) {
			f := newPresenceFixture(t, nil)
			m := f.machine(t, bytes.Repeat([]byte{1}, 32), "ordered event")
			a := f.admit(t, m.ID, 1)
			b := fixtureStreamID(t)
			eventSQL(t, f, "UPDATE link_machines SET event_mark = 40 WHERE machine_id = $1", m.ID)
			installReportAudit(t, f)
			admit := func(ctx context.Context) error {
				got, err := admitFixtureRun(f.state, ctx, m.ID, b, "")
				want := uint64(41)
				if admissionFirst {
					want = 40
				}
				if err == nil && got.Mark != want {
					return errors.New("The replacement read the wrong ordered mark.")
				}
				return err
			}
			apply := func(ctx context.Context) error {
				got, err := f.state.ApplyEvent(ctx, m.ID, a.ID, factsEvent(41, "Ordered report"))
				if err == nil && got != (EventResult{Mark: 41, Applied: true}) {
					return errors.New("The current event did not advance its mark.")
				}
				return err
			}
			if admissionFirst {
				if err := checkOneLine(t, f, admit, apply, false); !errors.Is(err, ErrNotCurrent) {
					t.Fatal("The displaced stream did not return the stale result.")
				}
				reportAudit(t, f)
			} else {
				if err := checkOneLine(t, f, apply, admit, false); err != nil {
					t.Fatal("Replacement did not observe the committed event.")
				}
				reportAudit(t, f, 40)
			}
		})
	}
}

func TestHundredEventsInOrder(t *testing.T) {
	f := newPresenceFixture(t, nil)
	m := f.machine(t, bytes.Repeat([]byte{1}, 32), "ordered reports")
	a := f.admit(t, m.ID, 1)
	installReportAudit(t, f)
	want := make([]int64, 0, 100)
	for n := uint64(1); n <= 100; n++ {
		requireEvent(t, f, m.ID, a.ID, factsEvent(n, "Report"), n, true)
		want = append(want, int64(n-1))
	}
	f.admit(t, m.ID, 2)
	if _, err := f.state.ApplyEvent(t.Context(), m.ID, a.ID, factsEvent(101, "Displaced")); !errors.Is(err, ErrNotCurrent) {
		t.Fatal("The displaced stream's event was not fenced.")
	}
	if eventMark(t, f, m.ID) != 100 {
		t.Fatal("A displaced stream advanced the mark.")
	}
	reportAudit(t, f, want...)
}
