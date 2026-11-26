package vault

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestVaultLoginRejectsUntrustedCertificate(t *testing.T) {
	server, logins := startTLSLoginFixture(t)
	client, _ := openTransportFixture(t, server.URL)

	if _, err := client.currentToken(t.Context()); !errors.Is(err, ErrUnavailable) {
		t.Errorf("Login with an untrusted certificate returned %v, want ErrUnavailable.", err)
	}
	if count := logins.Load(); count != 0 {
		t.Errorf("The untrusted server received %d login requests, want zero.", count)
	}

	trusted, transport := openTransportFixture(t, server.URL)
	transport.TLSClientConfig.RootCAs = fixtureRootCAs(server)
	if token, err := trusted.currentToken(t.Context()); err != nil || token == "" {
		t.Fatalf("Login with the fixture certificate trusted failed. %v", err)
	}
}

func TestVaultLoginIgnoresEnvironmentProxy(t *testing.T) {
	// Go caches proxy environment variables on first use in each process.
	const child = "LGTM_TEST_VAULT_PROXY_CHILD"
	if os.Getenv(child) != "1" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(t.Context(), executable, "-test.run=^TestVaultLoginIgnoresEnvironmentProxy$")
		cmd.Env = append(os.Environ(), child+"=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("The isolated proxy test failed. %v\n%s", err, output)
		}
		return
	}

	server, logins := startTLSLoginFixture(t)
	var proxyConnections atomic.Int32
	proxy := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	proxy.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			proxyConnections.Add(1)
		}
	}
	proxy.Start()
	t.Cleanup(proxy.Close)
	t.Setenv("HTTPS_PROXY", proxy.URL)
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")

	// The documentation hostname makes the request eligible for proxy discovery.
	// The dialer routes it to the loopback fixture without performing a DNS lookup.
	client, transport := openTransportFixture(t, "https://example.test")
	transport.TLSClientConfig.RootCAs = fixtureRootCAs(server)
	transport.TLSClientConfig.ServerName = "127.0.0.1"
	dial := transport.DialContext
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		switch addr {
		case "example.test:443":
			return dial(ctx, network, server.Listener.Addr().String())
		case proxy.Listener.Addr().String():
			return dial(ctx, network, addr)
		default:
			return nil, fmt.Errorf("The vault fixture received an unexpected dial address %q.", addr)
		}
	}

	_, err := client.currentToken(t.Context())
	if count := proxyConnections.Load(); count != 0 {
		t.Errorf("The environment proxy received %d connections, want zero.", count)
	}
	if count := logins.Load(); err != nil || count != 1 {
		t.Errorf("The vault received %d direct logins, want one successful login. %v", count, err)
	}
}

func startTLSLoginFixture(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var logins atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != loginPath {
			t.Error("The fixture received an unexpected vault request.")
			w.WriteHeader(http.StatusNotFound)
			return
		}
		logins.Add(1)
		io.WriteString(w, `{"auth":{"client_token":"fixture-token","lease_duration":900}}`)
	}))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.StartTLS()
	t.Cleanup(server.Close)
	return server, &logins
}

func openTransportFixture(t *testing.T, address string) (*Client, *http.Transport) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "credential.json")
	if err := os.WriteFile(file, []byte(`{"role_id":"fixture-role","secret_id":"fixture-secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	client, err := Open(address, file, false)
	if err != nil {
		t.Fatalf("The fixture client could not open. %v", err)
	}
	t.Cleanup(client.http.CloseIdleConnections)
	return client, client.http.Transport.(*http.Transport)
}

func fixtureRootCAs(server *httptest.Server) *x509.CertPool {
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	return roots
}
