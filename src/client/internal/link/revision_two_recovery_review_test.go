package link

import (
	"bytes"
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"connectrpc.com/connect"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/harness"
)

func TestRevisionTwoHandshakeFencesBeforeClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		closing, resumeClose := make(chan struct{}), make(chan struct{})
		finishClose := sync.OnceFunc(func() { close(resumeClose) })
		defer finishClose()
		p := protocols[0]
		p.opts = append([]connect.ClientOption{connect.WithInterceptors(&handshakeCloseProbe{
			UnaryInterceptorFunc: func(next connect.UnaryFunc) connect.UnaryFunc { return next },
			closing:              closing,
			resume:               resumeClose,
		})}, p.opts...)
		f := newRunPeerFixture(t, p)
		f.seed(t)

		started, releaseScan := make(chan struct{}), make(chan struct{})
		finishScan := sync.OnceFunc(func() { close(releaseScan) })
		defer finishScan()
		var scans int
		runReportProbes.scan = func(context.Context, harness.ScanOptions) []harness.Found {
			scans++
			if scans == 2 {
				close(started)
				<-releaseScan
			}
			return nil
		}

		cancel, done := f.start(t)
		defer cancel()
		first := f.next(t, 0)
		f.welcome(t, first, runWelcome(approvedApproval, 1, 0))
		for number := uint64(1); number <= 2; number++ {
			if event := eventOnLeg(t, first); event.GetNumber() != number {
				t.Fatal("approved startup did not publish the expected two reports")
			}
			first.send(t, ackEvent(number))
		}
		first.send(t, commandFrame(scanCommand(3, 1, time.Now().Add(time.Minute))))
		receiveNow(t, started, "the requested scan never reached its held probe")

		first.refuse <- peerError(connect.CodeUnavailable, "", "")
		second := f.next(t, 0)
		f.peer.prove(t, second, second.hello(t))
		before := readBytes(t, f.file())
		welcome := runWelcome(waitingApproval, 2, 2)
		welcome.LinkRevision = 1
		second.send(t, welcomeFrame(welcome))
		receiveNow(t, closing, "the client did not start closing the incompatible handshake")

		// Let the earlier scan finish while the rejected handshake is still closing.
		finishScan()
		synctest.Wait()
		if !bytes.Equal(before, readBytes(t, f.file())) {
			t.Error("the earlier scan changed saved history or its receipt while the incompatible handshake was closing")
		}

		finishClose()
		requireRecoveryStop(t, f, done, true)
	})
}

type handshakeCloseProbe struct {
	connect.UnaryInterceptorFunc
	streams atomic.Int32
	closing chan<- struct{}
	resume  <-chan struct{}
}

func (p *handshakeCloseProbe) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		if p.streams.Add(1) == 2 {
			return &heldHandshakeClose{StreamingClientConn: conn, probe: p}
		}
		return conn
	}
}

type heldHandshakeClose struct {
	connect.StreamingClientConn
	probe *handshakeCloseProbe
}

func (c *heldHandshakeClose) CloseResponse() error {
	close(c.probe.closing)
	<-c.probe.resume
	return c.StreamingClientConn.CloseResponse()
}
