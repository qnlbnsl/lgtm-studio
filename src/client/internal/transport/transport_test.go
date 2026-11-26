package transport

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// callBudget bounds one request so that a broken transport fails the test and
// does not hang it. It is a ceiling, never a delay the tests wait for.
const callBudget = 20 * time.Second

// probe is a handler that counts the requests it receives.
type probe struct {
	calls      atomic.Int32
	protoMajor atomic.Int32
}

func (p *probe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.calls.Add(1)
	p.protoMajor.Store(int32(r.ProtoMajor))
	_, _ = io.WriteString(w, "ok")
}

// startPlain starts a cleartext HTTP/2 server on the given loopback address.
func startPlain(t *testing.T, network, address string, h http.Handler) string {
	t.Helper()
	l, err := net.Listen(network, address)
	if err != nil {
		t.Skipf("cannot listen on %s %s: %v", network, address, err)
	}
	srv := httptest.NewUnstartedServer(h)
	_ = srv.Listener.Close()
	srv.Listener = l
	srv.Config.Protocols = new(http.Protocols)
	srv.Config.Protocols.SetUnencryptedHTTP2(true)
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.Start()
	t.Cleanup(srv.Close)
	return srv.URL
}

// leafSpec describes a certificate that a test server presents.
type leafSpec struct {
	key       *ecdsa.PrivateKey
	dns       []string
	ips       []net.IP
	notBefore time.Time
	notAfter  time.Time
	serial    int64
}

func newKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// pinOf returns the SHA-256 digest of the DER Subject Public Key Info of key.
func pinOf(t *testing.T, key *ecdsa.PrivateKey) []byte {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(der)
	return sum[:]
}

func validSpec(key *ecdsa.PrivateKey) leafSpec {
	return leafSpec{
		key:       key,
		dns:       []string{"localhost"},
		ips:       []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		notBefore: time.Now().Add(-time.Hour),
		notAfter:  time.Now().Add(24 * time.Hour),
		serial:    1,
	}
}

// selfSigned returns the DER bytes of a self-signed certificate.
func selfSigned(t *testing.T, s leafSpec) []byte {
	t.Helper()
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(s.serial),
		Subject:               pkix.Name{CommonName: "transport test"},
		NotBefore:             s.notBefore,
		NotAfter:              s.notAfter,
		DNSNames:              s.dns,
		IPAddresses:           s.ips,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &s.key.PublicKey, s.key)
	if err != nil {
		t.Fatal(err)
	}
	return der
}

// startTLS starts an HTTPS server with HTTP/2 that presents the given chain.
func startTLS(t *testing.T, chain [][]byte, key *ecdsa.PrivateKey, h http.Handler) string {
	t.Helper()
	srv := httptest.NewUnstartedServer(h)
	srv.EnableHTTP2 = true
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: chain, PrivateKey: key}}}
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv.URL
}

// get sends one GET request through a client from NewHTTPClient. An error from
// the constructor and an error from the request both come back as err, because
// the plan lets a bad pin fail at either place. It never returns a nil client
// without an error.
func get(t *testing.T, origin string, pin []byte) (*http.Response, string, error) {
	t.Helper()
	c, err := NewHTTPClient(origin, pin)
	if err != nil {
		return nil, "", err
	}
	if c == nil {
		t.Fatalf("NewHTTPClient(%q) returned no client and no error", origin)
	}
	defer c.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), callBudget)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+"/ping", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, string(body), nil
}

// wantServed asserts that the request reached the handler over HTTP/2.
func wantServed(t *testing.T, origin string, pin []byte, p *probe) {
	t.Helper()
	resp, body, err := get(t, origin, pin)
	if err != nil {
		t.Fatalf("request to %s: %v", origin, err)
	}
	if resp.StatusCode != http.StatusOK || body != "ok" {
		t.Fatalf("response = %d %q, want 200 \"ok\"", resp.StatusCode, body)
	}
	if got := p.calls.Load(); got != 1 {
		t.Fatalf("handler calls = %d, want 1", got)
	}
	if got := p.protoMajor.Load(); got != 2 {
		t.Fatalf("handler saw HTTP/%d, want HTTP/2", got)
	}
	if resp.ProtoMajor != 2 {
		t.Fatalf("response is HTTP/%d, want HTTP/2", resp.ProtoMajor)
	}
}

// wantRefused asserts that the request failed and no handler ran.
func wantRefused(t *testing.T, origin string, pin []byte, p *probe) error {
	t.Helper()
	resp, _, err := get(t, origin, pin)
	if err == nil {
		t.Fatalf("request to %s succeeded with status %d, want a refusal", origin, resp.StatusCode)
	}
	if got := p.calls.Load(); got != 0 {
		t.Fatalf("handler calls = %d, want 0 after a refusal", got)
	}
	return err
}

func TestOriginAndDialPolicy(t *testing.T) {
	t.Run("accepted origins are canonical", func(t *testing.T) {
		cases := []struct{ raw, want string }{
			{"https://example.test", "https://example.test"},
			{"HTTPS://Example.TEST", "https://example.test"},
			{"https://example.test:443", "https://example.test"},
			{"https://example.test:8443", "https://example.test:8443"},
			{"https://example.test:80", "https://example.test:80"},
			{"https://192.0.2.7:8443", "https://192.0.2.7:8443"},
			{"https://[2001:DB8::1]:443", "https://[2001:db8::1]"},
			{"https://[2001:db8::1]:8443", "https://[2001:db8::1]:8443"},
			{"http://127.0.0.1", "http://127.0.0.1"},
			{"http://127.0.0.1:80", "http://127.0.0.1"},
			{"http://127.0.0.1:8080", "http://127.0.0.1:8080"},
			{"http://[::1]:8080", "http://[::1]:8080"},
			{"http://[::1]:80", "http://[::1]"},
			{"http://localhost:3000", "http://localhost:3000"},
			{"HTTP://LocalHost:3000", "http://localhost:3000"},
			{"http://localhost:80", "http://localhost"},
			{"http://localhost:443", "http://localhost:443"},
		}
		for _, c := range cases {
			got, err := ParseOrigin(c.raw)
			if err != nil {
				t.Errorf("ParseOrigin(%q) error: %v", c.raw, err)
				continue
			}
			if got != c.want {
				t.Errorf("ParseOrigin(%q) = %q, want %q", c.raw, got, c.want)
				continue
			}
			again, err := ParseOrigin(got)
			if err != nil || again != got {
				t.Errorf("ParseOrigin(%q) = %q, %v; the canonical form must be stable", got, again, err)
			}
		}
	})

	t.Run("explicit default port equals omitted port", func(t *testing.T) {
		pairs := [][2]string{
			{"https://example.test:443", "https://example.test"},
			{"http://127.0.0.1:80", "http://127.0.0.1"},
			{"http://[::1]:80", "http://[::1]"},
			{"HTTPS://EXAMPLE.test", "https://example.test"},
		}
		for _, p := range pairs {
			a, errA := ParseOrigin(p[0])
			b, errB := ParseOrigin(p[1])
			if errA != nil || errB != nil {
				t.Errorf("ParseOrigin(%q), ParseOrigin(%q) errors: %v, %v", p[0], p[1], errA, errB)
				continue
			}
			if a == "" || a != b {
				t.Errorf("ParseOrigin(%q) = %q, ParseOrigin(%q) = %q, want equal and nonempty", p[0], a, p[1], b)
			}
		}
	})

	rejected := []string{
		"",
		"example.test",
		"//example.test",
		"example.test:443",
		"https://",
		"https:///",
		"ftp://example.test",
		"ws://example.test",
		"file:///etc/passwd",
		"https://user@example.test",
		"https://user:secret@example.test",
		"http://127.0.0.1@example.test",
		"http://localhost@example.test",
		"https://example.test/path",
		"https://example.test/join",
		"https://example.test?x=1",
		"https://example.test#fragment",
		" https://example.test",
		"https://example.test ",
		"https://exa mple.test",
		"https://example.test\n",
		"https://example.test\t",
		"https://example.test:0",
		"https://example.test:65536",
		"https://example.test:99999",
		"https://example.test:abc",
		"https://example.test:-1",
		"https://example.test:+443",
		"http://127.0.0.1:0",
		// Plain HTTP is allowed only for a loopback literal and localhost.
		"http://example.test",
		"http://example.test:80",
		"http://192.0.2.1",
		"http://192.0.2.1:8080",
		"http://10.2.3.4:8080",
		"http://0.0.0.0",
		"http://[::]",
		"http://[2001:db8::1]",
		"http://[::ffff:192.0.2.1]",
		// Deceptive hostnames and numeric forms.
		"http://localhost.example.test",
		"http://127.0.0.1.example.test",
		"http://example.test.127.0.0.1",
		"http://127.1",
		"http://0x7f.0.0.1",
		"http://2130706433",
		"http://0177.0.0.1",
		// Scoped addresses.
		"http://[fe80::1%25eth0]:80",
		"https://[fe80::1%25eth0]:443",
		"https://[::1%25lo]",
	}

	t.Run("rejected origins", func(t *testing.T) {
		for _, raw := range rejected {
			got, err := ParseOrigin(raw)
			if err == nil {
				t.Errorf("ParseOrigin(%q) = %q, want an error", raw, got)
			}
		}
	})

	t.Run("no client for a rejected origin", func(t *testing.T) {
		for _, raw := range rejected {
			c, err := NewHTTPClient(raw, nil)
			if err == nil {
				t.Errorf("NewHTTPClient(%q) returned %v and no error, want an error", raw, c)
			}
		}
	})

	t.Run("pin and scheme must agree", func(t *testing.T) {
		pin := make([]byte, 32)
		if c, err := NewHTTPClient("http://127.0.0.1:9", pin); err == nil {
			t.Errorf("plain loopback with a pin returned %v and no error, want an error", c)
		}
		if c, err := NewHTTPClient("http://192.0.2.1:9", nil); err == nil {
			t.Errorf("plain non-loopback origin returned %v and no error, want an error", c)
		}
		if c, err := NewHTTPClient("http://127.0.0.1:9", nil); err != nil || c == nil {
			t.Errorf("plain loopback without a pin = %v, %v, want a client", c, err)
		}
	})

	t.Run("plain IPv4 loopback uses HTTP/2", func(t *testing.T) {
		p := &probe{}
		origin := startPlain(t, "tcp4", "127.0.0.1:0", p)
		wantServed(t, origin, nil, p)
	})

	t.Run("plain IPv6 loopback uses HTTP/2", func(t *testing.T) {
		p := &probe{}
		origin := startPlain(t, "tcp6", "[::1]:0", p)
		wantServed(t, origin, nil, p)
	})

	t.Run("localhost reaches an IPv4 loopback listener", func(t *testing.T) {
		if _, err := net.LookupHost("localhost"); err != nil {
			t.Skipf("localhost does not resolve: %v", err)
		}
		p := &probe{}
		raw := startPlain(t, "tcp4", "127.0.0.1:0", p)
		origin := "http://localhost:" + raw[strings.LastIndex(raw, ":")+1:]
		wantServed(t, origin, nil, p)
	})

	t.Run("no environment proxy", func(t *testing.T) {
		var accepted atomic.Int32
		l, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				conn, err := l.Accept()
				if err != nil {
					return
				}
				accepted.Add(1)
				_ = conn.Close()
			}
		}()
		t.Cleanup(func() {
			_ = l.Close()
			<-done
		})
		proxy := "http://" + l.Addr().String()
		for _, name := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy"} {
			t.Setenv(name, proxy)
		}
		t.Setenv("NO_PROXY", "")
		t.Setenv("no_proxy", "")

		p := &probe{}
		origin := startPlain(t, "tcp4", "127.0.0.1:0", p)
		wantServed(t, origin, nil, p)
		if got := accepted.Load(); got != 0 {
			t.Fatalf("the configured proxy accepted %d connections, want 0", got)
		}

		c, err := NewHTTPClient(origin, nil)
		if err != nil || c == nil {
			t.Fatalf("NewHTTPClient = %v, %v", c, err)
		}
		if tr, ok := c.Transport.(*http.Transport); ok && tr.Proxy != nil {
			t.Fatal("the transport has a proxy function, want none")
		}
	})
}

func TestPinnedTransport(t *testing.T) {
	t.Run("correct pin reaches the handler over HTTP/2", func(t *testing.T) {
		key := newKey(t)
		p := &probe{}
		origin := startTLS(t, [][]byte{selfSigned(t, validSpec(key))}, key, p)
		wantServed(t, origin, pinOf(t, key), p)
	})

	t.Run("correct pin for a host name", func(t *testing.T) {
		if _, err := net.LookupHost("localhost"); err != nil {
			t.Skipf("localhost does not resolve: %v", err)
		}
		key := newKey(t)
		p := &probe{}
		raw := startTLS(t, [][]byte{selfSigned(t, validSpec(key))}, key, p)
		origin := "https://localhost:" + raw[strings.LastIndex(raw, ":")+1:]
		wantServed(t, origin, pinOf(t, key), p)
	})

	t.Run("wrong pin sends nothing", func(t *testing.T) {
		key, other := newKey(t), newKey(t)
		p := &probe{}
		origin := startTLS(t, [][]byte{selfSigned(t, validSpec(key))}, key, p)
		err := wantRefused(t, origin, pinOf(t, other), p)
		if !errors.Is(err, ErrCertificateKeyMismatch) {
			t.Fatalf("error = %v, want ErrCertificateKeyMismatch", err)
		}
	})

	t.Run("pin of the whole certificate is not the key pin", func(t *testing.T) {
		key := newKey(t)
		p := &probe{}
		der := selfSigned(t, validSpec(key))
		origin := startTLS(t, [][]byte{der}, key, p)
		sum := sha256.Sum256(der)
		wantRefused(t, origin, sum[:], p)
	})

	t.Run("missing pin sends nothing", func(t *testing.T) {
		key := newKey(t)
		p := &probe{}
		origin := startTLS(t, [][]byte{selfSigned(t, validSpec(key))}, key, p)
		wantRefused(t, origin, nil, p)
		wantRefused(t, origin, []byte{}, p)
	})

	t.Run("pin of the wrong length sends nothing", func(t *testing.T) {
		key := newKey(t)
		p := &probe{}
		origin := startTLS(t, [][]byte{selfSigned(t, validSpec(key))}, key, p)
		good := pinOf(t, key)
		wantRefused(t, origin, good[:31], p)
		wantRefused(t, origin, append(append([]byte{}, good...), 0), p)
	})

	t.Run("expired leaf sends nothing", func(t *testing.T) {
		key := newKey(t)
		p := &probe{}
		spec := validSpec(key)
		spec.notBefore = time.Now().Add(-48 * time.Hour)
		spec.notAfter = time.Now().Add(-time.Hour)
		origin := startTLS(t, [][]byte{selfSigned(t, spec)}, key, p)
		wantRefused(t, origin, pinOf(t, key), p)
	})

	t.Run("leaf that is not yet valid sends nothing", func(t *testing.T) {
		key := newKey(t)
		p := &probe{}
		spec := validSpec(key)
		spec.notBefore = time.Now().Add(time.Hour)
		spec.notAfter = time.Now().Add(48 * time.Hour)
		origin := startTLS(t, [][]byte{selfSigned(t, spec)}, key, p)
		wantRefused(t, origin, pinOf(t, key), p)
	})

	t.Run("wrong host name sends nothing", func(t *testing.T) {
		key := newKey(t)
		p := &probe{}
		spec := validSpec(key)
		spec.dns = []string{"other.example.test"}
		spec.ips = nil
		origin := startTLS(t, [][]byte{selfSigned(t, spec)}, key, p)
		wantRefused(t, origin, pinOf(t, key), p)
	})

	t.Run("wrong host name for a named origin sends nothing", func(t *testing.T) {
		if _, err := net.LookupHost("localhost"); err != nil {
			t.Skipf("localhost does not resolve: %v", err)
		}
		key := newKey(t)
		p := &probe{}
		spec := validSpec(key)
		spec.dns = []string{"other.example.test"}
		spec.ips = []net.IP{net.IPv4(127, 0, 0, 1)}
		raw := startTLS(t, [][]byte{selfSigned(t, spec)}, key, p)
		origin := "https://localhost:" + raw[strings.LastIndex(raw, ":")+1:]
		wantRefused(t, origin, pinOf(t, key), p)
	})

	t.Run("pinned key in another chain position does not count", func(t *testing.T) {
		caKey, leafKey := newKey(t), newKey(t)
		caSpec := validSpec(caKey)
		caSpec.dns, caSpec.ips = nil, nil
		caTmpl := &x509.Certificate{
			SerialNumber:          big.NewInt(100),
			Subject:               pkix.Name{CommonName: "transport test authority"},
			NotBefore:             caSpec.notBefore,
			NotAfter:              caSpec.notAfter,
			KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
			BasicConstraintsValid: true,
			IsCA:                  true,
		}
		caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
		if err != nil {
			t.Fatal(err)
		}
		caCert, err := x509.ParseCertificate(caDER)
		if err != nil {
			t.Fatal(err)
		}
		leafTmpl := &x509.Certificate{
			SerialNumber:          big.NewInt(101),
			Subject:               pkix.Name{CommonName: "transport test leaf"},
			NotBefore:             caSpec.notBefore,
			NotAfter:              caSpec.notAfter,
			IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1)},
			DNSNames:              []string{"localhost"},
			KeyUsage:              x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			BasicConstraintsValid: true,
		}
		leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &leafKey.PublicKey, caKey)
		if err != nil {
			t.Fatal(err)
		}
		p := &probe{}
		origin := startTLS(t, [][]byte{leafDER, caDER}, leafKey, p)
		// The pin names the authority key. Only the leaf key may satisfy it.
		wantRefused(t, origin, pinOf(t, caKey), p)
		// The leaf key itself is the trust anchor, with no chain to an authority.
		wantServed(t, origin, pinOf(t, leafKey), p)
	})

	t.Run("renewed leaf with the same key is accepted", func(t *testing.T) {
		key := newKey(t)
		pin := pinOf(t, key)

		first := &probe{}
		firstOrigin := startTLS(t, [][]byte{selfSigned(t, validSpec(key))}, key, first)
		wantServed(t, firstOrigin, pin, first)

		renewed := validSpec(key)
		renewed.serial = 2
		renewed.notBefore = time.Now().Add(-30 * time.Minute)
		renewed.notAfter = time.Now().Add(90 * 24 * time.Hour)
		second := &probe{}
		secondOrigin := startTLS(t, [][]byte{selfSigned(t, renewed)}, key, second)
		wantServed(t, secondOrigin, pin, second)
	})

	t.Run("a redirect cannot make a second handler run", func(t *testing.T) {
		key := newKey(t)
		der := selfSigned(t, validSpec(key))
		pin := pinOf(t, key)

		type target struct {
			name  string
			start func(h http.Handler) string
			pin   []byte
		}
		targets := []target{
			{"plain loopback", func(h http.Handler) string { return startPlain(t, "tcp4", "127.0.0.1:0", h) }, nil},
			{"pinned HTTPS", func(h http.Handler) string { return startTLS(t, [][]byte{der}, key, h) }, pin},
		}
		for _, tg := range targets {
			for _, method := range []string{http.MethodGet, http.MethodPost} {
				for _, code := range []int{http.StatusFound, http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
					second := &probe{}
					secondOrigin := tg.start(second)
					var redirects atomic.Int32
					firstOrigin := tg.start(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						redirects.Add(1)
						http.Redirect(w, r, secondOrigin+"/target", code)
					}))

					c, err := NewHTTPClient(firstOrigin, tg.pin)
					if err != nil || c == nil {
						t.Fatalf("%s: NewHTTPClient = %v, %v", tg.name, c, err)
					}
					ctx, cancel := context.WithTimeout(context.Background(), callBudget)
					req, err := http.NewRequestWithContext(ctx, method, firstOrigin+"/join", strings.NewReader("secret"))
					if err != nil {
						cancel()
						t.Fatal(err)
					}
					resp, err := c.Do(req)
					if err == nil {
						if resp.StatusCode != code {
							t.Errorf("%s %s %d: status = %d, want the redirect itself", tg.name, method, code, resp.StatusCode)
						}
						_ = resp.Body.Close()
					}
					cancel()
					c.CloseIdleConnections()
					if got := redirects.Load(); got != 1 {
						t.Errorf("%s %s %d: first handler calls = %d, want 1", tg.name, method, code, got)
					}
					if got := second.calls.Load(); got != 0 {
						t.Errorf("%s %s %d: redirect target calls = %d, want 0", tg.name, method, code, got)
					}
				}
			}
		}
	})
}
