package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link/machinetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func TestShutdownWaitsForHandlerAfterDrainTimeout(t *testing.T) {
	cfg, _ := appConfig(t)
	order := &reviewDrainOrder{Handler: slog.NewTextHandler(io.Discard, nil)}
	cfg.Postgres.ConnConfig.Tracer = order
	cfg.Postgres.BeforeClose = func(conn *pgx.Conn) {
		// A canceled query can leave a connection for the pool to discard.
		if !conn.IsClosed() && order.handlerStarted.Load() {
			order.poolClosed.Store(true)
			if !order.handlerFinished.Load() {
				order.closedEarly.Store(true)
			}
		}
	}
	held, release := storetest.Signal(t)
	entered := make(chan struct{}, 1)
	server, err := StartWithLinkSeams(t.Context(), cfg, slog.New(order), LinkSeams{
		AfterProof: func(context.Context, string) {
			order.handlerStarted.Store(true)
			entered <- struct{}{}
			// The handler stays in this callback after its context is canceled.
			<-held
		},
	})
	if err != nil {
		t.Fatal("The shutdown fixture could not start.")
	}
	t.Cleanup(func() { _ = server.Close() })
	defer release()

	httpClient := appClient(t)
	httpClient.Timeout = 0
	signIn(t, httpClient, cfg)
	clients := lgtmv1connect.NewClientServiceClient(httpClient, cfg.PublicURL)
	token, err := clients.CreateClientToken(t.Context(), connect.NewRequest(&webv1.CreateClientTokenRequest{}))
	if err != nil {
		t.Fatal("The fixture could not create a join token.")
	}
	link := linkv1connect.NewLinkServiceClient(httpClient, cfg.PublicURL)
	machine, err := machinetest.Join(t.Context(), link, token.Msg.GetToken(), "Held shutdown handler")
	if err != nil {
		t.Fatal("The fixture machine could not join.")
	}
	opening := storetest.Start(t, func() error {
		ctx, cancel := context.WithTimeout(t.Context(), storetest.ObservationTimeout)
		defer cancel()
		stream, _, err := machine.Connect(ctx, link)
		if stream != nil {
			_ = stream.CloseRequest()
			_ = stream.CloseResponse()
		}
		return err
	})
	storetest.ReceiveFrom(t, opening, "the proved handler blocked in its callback", entered)
	stopping := storetest.Start(t, server.Close)
	storetest.Poll(t, "shutdown closing the listener", func() bool {
		conn, err := net.DialTimeout("tcp", cfg.ListenAddr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
		}
		return errors.Is(err, syscall.ECONNREFUSED)
	})

	// The drain deadline starts before the listener closes, so the handler outlives it.
	timer := time.NewTimer(drainTimeout + time.Second)
	defer timer.Stop()
	select {
	case <-stopping.Done():
	case <-timer.C:
	}
	release()
	if err := opening.Wait(t); err == nil {
		t.Error("The held Open received a Welcome during shutdown.")
	}
	if err := stopping.Wait(t); err != nil {
		t.Error("The server did not close cleanly.")
	}
	if !order.unlocked.Load() || !order.poolClosed.Load() {
		t.Fatal("The fixture did not observe the instance unlock and Store close.")
	}
	if order.unlockedEarly.Load() || order.closedEarly.Load() {
		t.Errorf("Shutdown released resources before the held handler finished after the drain deadline. Instance lock released early: %t. Store closed early: %t.",
			order.unlockedEarly.Load(), order.closedEarly.Load())
	}
}

type reviewDrainOrder struct {
	slog.Handler
	handlerStarted, handlerFinished atomic.Bool
	unlocked, unlockedEarly         atomic.Bool
	poolClosed, closedEarly         atomic.Bool
}

func (o *reviewDrainOrder) Handle(ctx context.Context, record slog.Record) error {
	if record.Message == "link_stream_closed" {
		o.handlerFinished.Store(true)
	}
	return o.Handler.Handle(ctx, record)
}

func (o *reviewDrainOrder) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(data.SQL, "SELECT pg_advisory_unlock(") &&
		conn.Config().RuntimeParams["application_name"] == "lgtm-server-instance" {
		o.unlocked.Store(true)
		o.unlockedEarly.Store(!o.handlerFinished.Load())
	}
	return ctx
}

func (*reviewDrainOrder) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
