package link

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

func replayFacts(at time.Time) *linkv1.Event {
	return &linkv1.Event{
		HappenedAt: timestamppb.New(at),
		Report: &linkv1.Event_Facts{Facts: &linkv1.Facts{
			Os: "Linux", Cores: 7, MemoryBytes: 1<<30 + 1, ClientVersion: "test-client",
		}},
	}
}

func storedReport(t *testing.T, event *linkv1.Event) []byte {
	t.Helper()
	body, err := proto.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	return append([]byte{1}, body...)
}

func appendLinkFixture(t *testing.T, store *Store, payload []byte) uint64 {
	t.Helper()
	seq, err := store.Append(payload)
	if err != nil {
		t.Fatal(err)
	}
	return seq
}

func startReplay(t *testing.T, store *Store, through uint64) *replay {
	t.Helper()
	cursor, err := newReplay(store, through)
	if err != nil || cursor == nil {
		t.Fatalf("newReplay at %d: cursor present = %v, error = %v", through, cursor != nil, err)
	}
	return cursor
}

func nextReplay(t *testing.T, cursor *replay, want uint64) *linkv1.Event {
	t.Helper()
	event, err := cursor.next()
	if err != nil || event == nil || event.GetNumber() != want {
		t.Fatalf("next report number = %d (present = %v), error = %v, want %d", event.GetNumber(), event != nil, err, want)
	}
	return event
}

func TestReplayPreservesSequenceAndCaptureTime(t *testing.T) {
	folder, _ := newState(t)
	store := mustCreate(t, folder, StoreOptions{})
	reports := []*linkv1.Event{
		replayFacts(tNow),
		{
			HappenedAt: timestamppb.New(tNow.Add(time.Second)),
			Report: &linkv1.Event_HarnessesFound{HarnessesFound: &linkv1.HarnessesFound{
				Harnesses: []*linkv1.Harness{{Name: "codex", Version: proto.String("1.2.3")}},
			}},
		},
		replayFacts(tNow.Add(2 * time.Second)),
	}
	payloads := make([][]byte, 0, len(reports))
	for i, report := range reports {
		before := proto.Clone(report)
		payload, err := encodeReport(report)
		decoded := new(linkv1.Event)
		if err != nil || len(payload) < 2 || payload[0] != 1 || proto.Unmarshal(payload[1:], decoded) != nil ||
			!proto.Equal(decoded, before) || !proto.Equal(report, before) {
			t.Fatalf("encodeReport %d did not produce the versioned unnumbered report without mutation: %v", i, err)
		}
		payloads = append(payloads, payload)
		appendLinkFixture(t, store, payload)
	}
	first := startReplay(t, store, 1)
	nextReplay(t, first, 2)
	if store.Len() != 2 || store.Next() != 4 {
		t.Fatalf("welcome and handoff left length %d, next %d; want 2, 4", store.Len(), store.Next())
	}
	store = reopen(t, folder, store, StoreOptions{})
	again := startReplay(t, store, 1)
	for i, report := range reports[1:] {
		number := uint64(i + 2)
		want := proto.CloneOf(report)
		want.Number = number
		if got := nextReplay(t, again, number); !proto.Equal(got, want) {
			t.Fatalf("report %d changed its payload or capture time", number)
		}
	}
	if end, err := again.next(); end != nil || err != nil {
		t.Fatalf("exhausted cursor returned a report or error: %v", err)
	}
	queued, err := store.Read(1, 3)
	if err != nil || len(queued) != 2 {
		t.Fatalf("reading unacknowledged reports: count %d, error %v", len(queued), err)
	}
	for i, event := range queued {
		if event.Seq != uint64(i+2) || !bytes.Equal(event.Payload, payloads[i+1]) {
			t.Fatalf("handoff changed the stored record at position %d", i)
		}
	}
}

func TestReplayRejectsImpossibleMarks(t *testing.T) {
	folder, _ := newState(t)
	store := mustCreate(t, folder, StoreOptions{})
	for range 3 {
		appendLinkFixture(t, store, storedReport(t, replayFacts(tNow)))
	}
	if _, err := newReplay(store, store.Next()); !errors.Is(err, ErrAckAhead) {
		t.Errorf("impossible welcome: %v, want ErrAckAhead", err)
	}
	cursor := startReplay(t, store, 1)
	nextReplay(t, cursor, 2)
	if err := cursor.ack(2); err != nil {
		t.Errorf("immediate acknowledgement of handed-off report: %v", err)
	}
	if err := cursor.ack(1); err != nil {
		t.Errorf("lower repeat acknowledgement: %v", err)
	}
	if err := cursor.ack(3); !errors.Is(err, ErrAckAhead) {
		t.Errorf("acknowledgement of unsent report: %v, want ErrAckAhead", err)
	}
	store = reopen(t, folder, store, StoreOptions{})
	pending, err := store.Read(1, 3)
	if err != nil || len(pending) != 1 || pending[0].Seq != 3 {
		t.Fatalf("expected only report 3 to survive the acknowledgements: count %d, error %v", len(pending), err)
	}
}

func TestReplayRejectsDamagedPayload(t *testing.T) {
	wantEvent := replayFacts(tNow)
	numbered := proto.CloneOf(wantEvent)
	numbered.Number = 1
	cases := []struct {
		name string
		body []byte
	}{
		{"version", append([]byte{2}, storedReport(t, wantEvent)[1:]...)},
		{"unparseable", []byte{1, 0xff}},
		{"missing report", storedReport(t, &linkv1.Event{HappenedAt: timestamppb.New(tNow)})},
		{"missing timestamp", storedReport(t, &linkv1.Event{Report: wantEvent.Report})},
		{"already numbered", storedReport(t, numbered)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			folder, _ := newState(t)
			store := mustCreate(t, folder, StoreOptions{})
			appendLinkFixture(t, store, tc.body)
			appendLinkFixture(t, store, storedReport(t, wantEvent))
			before := readBytes(t, folder.Paths().Store)
			cursor := startReplay(t, store, 0)
			if event, err := cursor.next(); event != nil || !errors.Is(err, ErrStoreDamaged) {
				t.Errorf("damaged payload yielded a report or the wrong error: %v", err)
			}
			store = reopen(t, folder, store, StoreOptions{})
			if !bytes.Equal(before, readBytes(t, folder.Paths().Store)) || store.Len() != 2 {
				t.Fatal("damaged payload handling changed the file")
			}
		})
	}
}

func linkRefusal(reason string) error {
	err := connect.NewError(connect.CodeFailedPrecondition, errors.New("fixture refusal"))
	err.Meta().Set("Lgtm-Link-Refusal", reason)
	return err
}

func TestReplayResumesAboveFreshWelcomeAfterGap(t *testing.T) {
	for _, protocol := range protocols {
		t.Run(protocol.name, func(t *testing.T) {
			folder, _ := newState(t)
			store := mustCreate(t, folder, StoreOptions{})
			for i := range 4 {
				appendLinkFixture(t, store, storedReport(t, replayFacts(tNow.Add(time.Duration(i)*time.Second))))
			}

			// The first stream loses the acknowledgement for 2. The fixture drops
			// report 3 after handoff, so report 4 causes a gap. The next welcome confirms only the committed prefix.
			broken := goodWelcome()
			broken.LastEventNumber = 1
			fresh := goodWelcome()
			fresh.LastEventNumber = 2
			var streamCount atomic.Int32
			serverErrors := make(chan error, 2)
			_, client, server, credential := startScene(t, protocol, func(s *scene, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
				streamNumber := streamCount.Add(1)
				welcome, want := broken, []uint64{2, 4}
				if streamNumber == 2 {
					welcome, want = fresh, []uint64{3, 4}
				}
				if err := honest(welcome)(s, stream); err != nil {
					return err
				}
				for _, n := range want {
					frame, err := stream.Receive()
					if err != nil {
						return err
					}
					if frame.GetEvent().GetNumber() != n {
						err := fmt.Errorf("stream %d: report number %d, want %d", streamNumber, frame.GetEvent().GetNumber(), n)
						serverErrors <- err
						return connect.NewError(connect.CodeInvalidArgument, err)
					}
				}
				if streamNumber == 1 {
					return linkRefusal("event_gap")
				}
				return stream.Send(&linkv1.OpenResponse{Kind: &linkv1.OpenResponse_EventAck{
					EventAck: &linkv1.EventAck{Through: 4},
				}})
			})

			ctx, cancel := context.WithTimeout(t.Context(), openCeiling)
			defer cancel()
			for _, last := range []uint64{1, 2} {
				session, err := openSession(ctx, client, server, credential, fixtureRunIdentity)
				if err != nil {
					t.Fatal(err)
				}
				defer session.close()
				cursor := startReplay(t, store, session.welcome.GetLastEventNumber())
				for n := last + 1; n <= 4; n++ {
					event := nextReplay(t, cursor, n)
					if last == 1 && n == 3 {
						continue
					}
					if err := session.send(&linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Event{Event: event}}); err != nil {
						t.Fatal(err)
					}
				}
				reply, err := session.receive()
				select {
				case serverErr := <-serverErrors:
					t.Fatal(serverErr)
				default:
				}
				if last == 1 {
					if connect.CodeOf(err) != connect.CodeFailedPrecondition {
						t.Fatalf("event gap: %v", err)
					}
					if _, stop := isTerminal(err); stop {
						t.Fatal("event gap was terminal")
					}
				} else {
					if err != nil || reply.GetEventAck().GetThrough() != 4 {
						t.Fatalf("fresh stream did not acknowledge replay: %v", err)
					}
					if err := cursor.ack(reply.GetEventAck().GetThrough()); err != nil || store.Len() != 0 {
						t.Fatalf("acknowledging replay: length %d, error %v", store.Len(), err)
					}
				}
				_ = session.close()
			}
		})
	}
}
