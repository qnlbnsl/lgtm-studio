package servertest_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"connectrpc.com/connect"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/servertest"
)

func requireFixture(t *testing.T, server *servertest.Server) string {
	t.Helper()
	if server == nil || server.HTTPClient == nil || server.HTTPClient.Jar == nil {
		t.Fatal("The fixture did not provide an HTTP client with a cookie jar.")
	}
	address, err := url.Parse(server.URL)
	if err != nil || address.Scheme != "http" || address.Port() == "" {
		t.Fatal("The fixture did not provide a plaintext server URL with a port.")
	}
	ip := net.ParseIP(address.Hostname())
	if ip == nil || !ip.IsLoopback() || len(server.SigningPublicKey) != ed25519.PublicKeySize {
		t.Fatal("The fixture did not provide a loopback address and an Ed25519 public key.")
	}
	return address.Host
}

func requireGetServer(t *testing.T, service lgtmv1connect.SessionServiceClient, signedIn bool) {
	t.Helper()
	ctx, cancel := contextWithRequestBudget(t)
	defer cancel()
	res, err := service.GetServer(ctx, connect.NewRequest(&webv1.GetServerRequest{}))
	if err != nil || res.Msg.GetSignedIn() != signedIn {
		t.Fatalf("The generated client did not report a signed-in state of %t.", signedIn)
	}
}

func signInFixture(t *testing.T, service lgtmv1connect.SessionServiceClient, password string) {
	t.Helper()
	ctx, cancel := contextWithRequestBudget(t)
	defer cancel()
	res, err := service.SignIn(ctx, connect.NewRequest(&webv1.SignInRequest{Password: password}))
	if err != nil || res.Msg.GetResult() != webv1.SignInResult_SIGN_IN_RESULT_SIGNED_IN {
		t.Fatal("The generated client could not sign in to the fixture.")
	}
	requireGetServer(t, service, true)
}

func contextWithRequestBudget(t *testing.T) (context.Context, context.CancelFunc) {
	return context.WithTimeout(t.Context(), 5*time.Second)
}

func requireHTTP2(t *testing.T, server *servertest.Server) {
	t.Helper()
	ctx, cancel := contextWithRequestBudget(t)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/healthz", nil)
	if err != nil {
		t.Fatal("The fixture URL could not be used for a health request.")
	}
	res, err := server.HTTPClient.Do(req)
	if err != nil {
		t.Fatal("The fixture did not answer a cleartext HTTP/2 health request.")
	}
	defer res.Body.Close()
	if res.ProtoMajor != 2 || res.StatusCode != http.StatusNoContent {
		t.Errorf("The fixture returned HTTP/%d and status %d, want HTTP/2 and 204.", res.ProtoMajor, res.StatusCode)
	}
}

func TestStartAndRestartServer(t *testing.T) {
	const password = "fake-servertest-owner-password"
	first := servertest.Start(t, servertest.Options{Password: password})
	address := requireFixture(t, first)
	originalURL := first.URL
	originalKey := bytes.Clone(first.SigningPublicKey)
	requireHTTP2(t, first)
	firstClient := lgtmv1connect.NewSessionServiceClient(first.HTTPClient, first.URL)
	requireGetServer(t, firstClient, false)
	signInFixture(t, firstClient, password)

	first.Stop(t)
	first.Stop(t)
	conn, err := net.DialTimeout("tcp", address, time.Second)
	if err == nil {
		_ = conn.Close()
		t.Fatal("Stopping the fixture left its listener open.")
	}

	first.Restart(t)
	if requireFixture(t, first) != address || first.URL != originalURL || !bytes.Equal(first.SigningPublicKey, originalKey) {
		t.Error("Restart changed the fixture address, URL or public signing key.")
	}
	requireGetServer(t, firstClient, true)
	requireHTTP2(t, first)
	grpcClient := lgtmv1connect.NewSessionServiceClient(first.HTTPClient, first.URL, connect.WithGRPC())
	requireGetServer(t, grpcClient, true)

	firstEnv := first.Environment()
	firstDatabase := firstEnv["LGTM_POSTGRES_DATABASE"]
	if firstDatabase == "" || firstEnv["LGTM_WEB_PASSWORD"] != password || firstEnv["LGTM_POSTGRES_PASSWORD_FILE"] == "" {
		t.Fatal("The fixture did not export the database and password settings used by the command.")
	}
	firstEnv["LGTM_POSTGRES_DATABASE"] = "fake-mutation-database"
	delete(firstEnv, "LGTM_WEB_PASSWORD")
	firstEnv["LGTM_OPENBAO_ADDR"] = "http://example.test"
	again := first.Environment()
	if again["LGTM_POSTGRES_DATABASE"] != firstDatabase || again["LGTM_WEB_PASSWORD"] != password ||
		again["LGTM_OPENBAO_ADDR"] == "" || again["LGTM_OPENBAO_ADDR"] == firstEnv["LGTM_OPENBAO_ADDR"] {
		t.Error("Changing an exported environment map affected a later copy.")
	}
	requireGetServer(t, firstClient, true)

	second := servertest.Start(t, servertest.Options{Password: password})
	requireFixture(t, second)
	secondDatabase := second.Environment()["LGTM_POSTGRES_DATABASE"]
	if secondDatabase == "" || secondDatabase == firstDatabase {
		t.Error("The second fixture did not receive a separate database.")
	}
	secondClient := lgtmv1connect.NewSessionServiceClient(second.HTTPClient, second.URL)
	requireGetServer(t, secondClient, false)
	// Use the same password and explicitly send the first fixture's live cookie to the second.
	// This distinguishes database isolation from separate jars or different password bindings.
	firstURL, err := url.Parse(first.URL)
	if err != nil {
		t.Fatal("The first fixture did not provide a usable URL.")
	}
	cookies := first.HTTPClient.Jar.Cookies(firstURL)
	if len(cookies) == 0 {
		t.Fatal("The original fixture's live session did not leave a cookie in its jar.")
	}
	request := connect.NewRequest(&webv1.GetServerRequest{})
	for _, cookie := range cookies {
		request.Header().Add("Cookie", cookie.String())
	}
	ctx, cancel := contextWithRequestBudget(t)
	defer cancel()
	response, err := secondClient.GetServer(ctx, request)
	if err != nil || response.Msg.GetSignedIn() {
		t.Error("The second database accepted the first fixture's live session cookie.")
	}

	signInFixture(t, secondClient, password)
	requireGetServer(t, firstClient, true)
}
