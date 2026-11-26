package app

import (
	"errors"
	"log/slog"
	"testing"

	"connectrpc.com/connect"

	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func TestMachinesWatchReceivesFixedShutdownSentence(t *testing.T) {
	cfg, _ := appConfig(t)
	server, err := Start(t.Context(), cfg, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal("The application could not start.")
	}
	t.Cleanup(func() { _ = server.Close() })

	client := appClient(t)
	client.Timeout = storetest.ObservationTimeout
	signIn(t, client, cfg)
	machines := lgtmv1connect.NewClientServiceClient(client, cfg.PublicURL)
	watch, err := machines.WatchClients(t.Context(), connect.NewRequest(&webv1.WatchClientsRequest{}))
	if err != nil {
		t.Fatal("The signed-in Machines watch could not open.")
	}
	defer watch.Close()
	if !watch.Receive() || watch.Msg().GetAll() == nil {
		t.Fatal("The Machines watch did not receive a snapshot before shutdown.")
	}

	stopping := storetest.Start(t, server.Close)
	for watch.Receive() {
	}
	var wireErr *connect.Error
	err = watch.Err()
	if !connect.IsWireError(err) || !errors.As(err, &wireErr) || wireErr.Code() != connect.CodeUnavailable {
		t.Fatalf("The Machines watch ended with %s, want a server-sent unavailable error.", connect.CodeOf(err))
	}
	const sentence = "The server is stopping."
	if wireErr.Message() != sentence {
		t.Errorf("The Machines watch did not receive the exact stopping sentence %q.", sentence)
	}
	if stopping.Wait(t) != nil {
		t.Error("The application did not close cleanly.")
	}
}
