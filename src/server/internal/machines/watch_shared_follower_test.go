package machines

import (
	"context"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"

	lgtmv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/api"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

func TestWatchClientsSharedFollower(t *testing.T) {
	f := newClientsWatchFixture(t)
	initial := f.create(t, "initial machine")[0]
	token, err := f.sessions.Issue(t.Context())
	if err != nil {
		t.Fatal("Issuing the second session failed.")
	}
	secondCookie := (&http.Cookie{Name: auth.CookieName, Value: token}).String()
	follower, err := store.StartFollowerWithLifetime(t.Context(), context.WithoutCancel(t.Context()), f.store, nil)
	if err != nil {
		t.Fatal("Starting the shared follower failed.")
	}
	t.Cleanup(follower.Close)
	origins, err := auth.NewOriginPolicy("http://127.0.0.1:8787", nil)
	if err != nil {
		t.Fatal("Creating the watch origin policy failed.")
	}
	service := NewService(f.store, f.sessions, f.registry, nil).WithFollower(follower).WithWatchShutdown(t.Context())
	handler := api.NewHandler(api.Options{Sessions: f.sessions, Origins: origins, Clients: service})
	httpClient, url := serveClientsWatchTest(t, handler, 0)
	client := lgtmv1connect.NewClientServiceClient(httpClient, url)

	var streams [2]*connect.ServerStreamForClient[lgtmv1.WatchClientsResponse]
	var positions [2]uint64
	for i, cookie := range []string{f.cookie, secondCookie} {
		stream, err := openClientsWatch(t, client, cookie, 15*time.Second)
		if err != nil {
			t.Fatalf("Opening watch %d failed.", i+1)
		}
		if !stream.Receive() {
			t.Fatalf("Watch %d ended with code %s before the snapshot.", i+1, connect.CodeOf(stream.Err()))
		}
		snapshot := stream.Msg()
		list := snapshot.GetAll().GetClients()
		if snapshot.GetPosition() == nil || len(list) != 1 || list[0].GetId() != initial.ID {
			t.Fatalf("Watch %d did not receive the initial machine and its snapshot position.", i+1)
		}
		streams[i] = stream
		positions[i] = snapshot.GetPosition().GetValue()
	}

	requireSaved := func(i int, want Machine) {
		t.Helper()
		if !streams[i].Receive() {
			t.Fatalf("Watch %d ended with code %s before the saved machine.", i+1, connect.CodeOf(streams[i].Err()))
		}
		msg := streams[i].Msg()
		if msg.GetSaved().GetId() != want.ID || msg.GetPosition().GetValue() <= positions[i] {
			t.Fatalf("Watch %d did not receive the saved machine at a later position.", i+1)
		}
		positions[i] = msg.GetPosition().GetValue()
	}
	added := f.create(t, "live machine")[0]
	requireSaved(0, added)
	requireSaved(1, added)

	if err := f.sessions.Revoke(t.Context(), f.verifier); err != nil {
		t.Fatal("Revoking the first session failed.")
	}
	trigger := f.create(t, "machine that prompts reauthorization")[0]
	if streams[0].Receive() || connect.CodeOf(streams[0].Err()) != connect.CodeUnauthenticated {
		t.Fatal("The first watch did not end with unauthenticated after revocation.")
	}
	requireSaved(1, trigger)
	requireSaved(1, f.create(t, "machine added after the first watch ended")[0])

	follower.Close()
	if streams[1].Receive() || connect.CodeOf(streams[1].Err()) != connect.CodeCanceled {
		t.Fatal("The remaining watch did not end with canceled when its owner closed the shared follower.")
	}
}
