package link

import (
	"bytes"
	"context"
	"errors"
	"runtime"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/enroll"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
)

func TestReviewFAckCeilingUsesReceipt(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	firstSent := make(chan struct{})
	releaseSender := make(chan struct{})
	sendReplies := make(chan struct{})

	peer, client, server, credential := startScene(t, protocols[0], func(s *scene, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error {
		if err := honest(runWelcome(approvedApproval, 1, 0))(s, stream); err != nil {
			return err
		}
		for {
			frame, err := stream.Receive()
			if err != nil {
				return err
			}
			if frame.GetEvent().GetNumber() != 1 {
				continue
			}
			select {
			case <-sendReplies:
			case <-ctx.Done():
				return ctx.Err()
			}
			if err := stream.Send(ackHeartbeat()); err != nil {
				return err
			}
			if err := stream.Send(ackEvent(2)); err != nil {
				return err
			}
		}
	})
	oldClient := newRunLinkClient
	newRunLinkClient = func(enroll.Server) (linkv1connect.LinkServiceClient, func(), error) {
		return client, nil, nil
	}
	t.Cleanup(func() { newRunLinkClient = oldClient })

	type interval struct{ before, after time.Time }
	receipt := make(chan interval, 1)
	useSessionIO(t, func(ctx context.Context, _ *session, wire runSessionIO) runSessionIO {
		send, receive := wire.send, wire.receive
		wire.send = func(frame *linkv1.OpenRequest) error {
			err := send(frame)
			if err == nil && frame.GetEvent().GetNumber() == 1 {
				close(firstSent)
				select {
				case <-releaseSender:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return err
		}
		var before time.Time
		wire.receive = func() (*linkv1.OpenResponse, error) {
			if !before.IsZero() {
				// Entry into the next receive means the previous frame was queued.
				receipt <- interval{before, time.Now()}
				before = time.Time{}
			}
			frame, err := receive()
			if err == nil && frame.GetEventAck().GetThrough() == 2 {
				before = time.Now()
			}
			return frame, err
		}
		return wire
	})

	folder, _ := newState(t)
	store := mustCreate(t, folder, StoreOptions{})
	for range 2 {
		if _, err := store.Append(fixtureFactsPayload(t, "saved")); err != nil {
			t.Fatal("could not seed the replay")
		}
	}
	acked := watchFixtureAcks(t)
	log := new(reportLog)
	opts := RunOptions{Server: server, Credential: credential, Folder: folder}
	r := &clientRun{ctx: ctx, cancel: cancel, opts: opts, store: store, logger: log.logger()}
	r.reports = newReporterWithContextAppend(opts.Settings, opts.Version, opts.ScanOptions, store.appendContext, r.logger, fixtureReportProbes())
	r.commands = newCommandHandler(store, r.reports.scanHarnesses, r.queueResult, nil, r.logger)
	done := make(chan error, 1)
	joined := make(chan struct{})
	go func() {
		defer close(joined)
		done <- r.attempt(ctx, func() {})
	}()
	defer func() {
		cancel()
		<-joined
		_ = r.stopOutage()
	}()

	receiveWithin(t, firstSent, time.Second, "the first event did not reach the sender")
	live := r.live
	arrived := func() interval {
		store.mu.Lock()
		defer store.mu.Unlock()
		close(releaseSender)
		// The owner has passed the reply check once it is blocked reading the
		// next event. Both replies must arrive before that event is handed off.
		waitForReviewFReplayRead(t, ctx)
		close(sendReplies)
		stamp := receiveWithin(t, receipt, time.Second, "the premature acknowledgement was not queued")
		time.Sleep(time.Millisecond)
		return stamp
	}()

	var err error
	select {
	case err = <-done:
	case <-acked:
		t.Fatal("the owner applied an acknowledgement that arrived before event two was handed off")
	case <-time.After(time.Second):
		t.Fatal("the queued acknowledgement did not stop the stream with impossible_ack")
	}
	if live.replay.ceiling != 2 {
		t.Fatal("the owner did not hand off event two before processing the acknowledgement")
	}
	if !errors.Is(r.fault, ErrAckAhead) {
		t.Fatal("the queued acknowledgement did not stop the stream with impossible_ack")
	}
	var end streamEnd
	if !errors.As(err, &end) || end.at.Before(arrived.before) || end.at.After(arrived.after) {
		t.Fatal("the stream end was outside the acknowledgement receipt interval")
	}
	events, readErr := store.Read(1, 2)
	if readErr != nil || len(events) != 2 || events[0].Seq != 1 || events[1].Seq != 2 {
		t.Fatal("the premature acknowledgement removed saved events")
	}
	_, _ = stopForStore(r.logger, r.fault)
	f := runPeerFixture{peer: &wirePeer{keys: peer.keys}, log: log}
	if records := f.diagnostics(t); !slices.ContainsFunc(records, func(record sessionDiagnostic) bool {
		return record.Msg == "link_stopped" && record.Code == "impossible_ack"
	}) {
		t.Fatal("the store stop did not produce the impossible_ack diagnostic")
	}
}

func waitForReviewFReplayRead(t *testing.T, ctx context.Context) {
	t.Helper()
	buf := make([]byte, 64<<10)
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		// The store mutex has no read hook, so wait for the blocked read itself.
		n := runtime.Stack(buf, true)
		for stack := range bytes.SplitSeq(buf[:n], []byte("\n\n")) {
			if bytes.Contains(stack, []byte("(*replay).next(")) && bytes.Contains(stack, []byte("(*Store).Read(")) {
				return
			}
		}
		select {
		case <-tick.C:
		case <-ctx.Done():
			t.Fatal("the owner did not block while reading the next event")
		}
	}
}
