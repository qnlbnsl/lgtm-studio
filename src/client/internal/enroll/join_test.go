package enroll

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/state"
)

// callBudget is a ceiling for one call, so that a broken client fails a test and does not
// hang it. It is never a delay that a test waits for.
const callBudget = 20 * time.Second

// Obviously false values. None of them is a real secret.
const (
	fakeTokenID   = "0193a2b4-0000-7000-8000-000000000001"
	fakeMachineID = "0193a2b4-5c6d-7e8f-9a0b-1c2d3e4f5a6b"
)

var (
	fakeSecret     = bytes.Repeat([]byte{0x33}, 32)
	serverKey      = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x01}, 32))
	otherServerKey = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x05}, 32))
)

// fakeClock replaces the two hooks of the package. Waiting advances the clock and returns
// at once, so the tests of the retry cadence take no real time.
type fakeClock struct {
	mu    sync.Mutex
	now   time.Time
	waits []time.Duration
}

func useFakeClock(t *testing.T) *fakeClock {
	t.Helper()
	c := &fakeClock{now: time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)}
	oldNow, oldSleep := joinNow, joinSleep
	joinNow = func() time.Time {
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.now
	}
	joinSleep = func(ctx context.Context, d time.Duration) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		c.waits = append(c.waits, d)
		c.now = c.now.Add(d)
		return nil
	}
	t.Cleanup(func() { joinNow, joinSleep = oldNow, oldSleep })
	return c
}

func (c *fakeClock) waited() []time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]time.Duration(nil), c.waits...)
}

func newFolder(t *testing.T) *state.Folder {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := state.Open(filepath.Join(root, "state"))
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

type joinHandler func(context.Context, *connect.Request[linkv1.JoinRequest]) (*connect.Response[linkv1.JoinResponse], error)

type fakeLink struct {
	linkv1connect.UnimplementedLinkServiceHandler
	join  joinHandler
	calls atomic.Int32
}

func (l *fakeLink) Join(ctx context.Context, req *connect.Request[linkv1.JoinRequest]) (*connect.Response[linkv1.JoinResponse], error) {
	l.calls.Add(1)
	return l.join(ctx, req)
}

// startLink serves the generated handler over cleartext HTTP/2 on a loopback listener.
func startLink(t *testing.T, join joinHandler) (*fakeLink, string) {
	t.Helper()
	link := &fakeLink{join: join}
	mux := http.NewServeMux()
	mux.Handle(linkv1connect.NewLinkServiceHandler(link))
	return link, serve(t, mux)
}

func serve(t *testing.T, h http.Handler) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot listen on loopback: %v", err)
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

func serverDigest(key ed25519.PrivateKey) []byte {
	sum := sha256.Sum256(key.Public().(ed25519.PublicKey))
	return sum[:]
}

// tokenSpec is the content of a join token that a test builds.
type tokenSpec struct {
	ID, Address string
	Secret      []byte
	ServerSHA   []byte
	CertSHA     []byte
}

func goodSpec(address string) tokenSpec {
	return tokenSpec{ID: fakeTokenID, Address: address, Secret: fakeSecret, ServerSHA: serverDigest(serverKey)}
}

func (s tokenSpec) text(t *testing.T) string {
	t.Helper()
	raw, err := proto.Marshal(&linkv1.JoinToken{
		TokenId: s.ID, Secret: s.Secret, Address: s.Address,
		ServerKeySha256: s.ServerSHA, CertificateKeySha256: s.CertSHA,
	})
	if err != nil {
		t.Fatal(err)
	}
	return "lgtm1_" + base64.RawURLEncoding.EncodeToString(raw)
}

func goodReply() *connect.Response[linkv1.JoinResponse] {
	return connect.NewResponse(&linkv1.JoinResponse{
		MachineId:       fakeMachineID,
		ServerPublicKey: serverKey.Public().(ed25519.PublicKey),
	})
}

func answerGood(context.Context, *connect.Request[linkv1.JoinRequest]) (*connect.Response[linkv1.JoinResponse], error) {
	return goodReply(), nil
}

func refuse(code connect.Code, reason string) joinHandler {
	return func(context.Context, *connect.Request[linkv1.JoinRequest]) (*connect.Response[linkv1.JoinResponse], error) {
		err := connect.NewError(code, errors.New("A sentence from the server that a client must not repeat."))
		if reason != "" {
			err.Meta().Set("Lgtm-Join-Refusal", reason)
		}
		return nil, err
	}
}

// join runs Join with a ceiling on its time and a fake clock.
func join(t *testing.T, f *state.Folder, address, token, host string, onRetry func()) (Server, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), callBudget)
	defer cancel()
	return Join(ctx, f, address, token, host, onRetry)
}

// code returns the diagnostic code of a join error, whether it is a value or a pointer.
func code(err error) (string, bool) {
	var v JoinError
	if errors.As(err, &v) {
		return v.Code, true
	}
	var p *JoinError
	if errors.As(err, &p) && p != nil {
		return p.Code, true
	}
	return "", false
}

func wantCode(t *testing.T, err error, want string) {
	t.Helper()
	got, ok := code(err)
	if !ok || got != want {
		t.Fatalf("error %v: code %q (is a JoinError: %v), want %q", err, got, ok, want)
	}
}

func fileExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func wantNoLeak(t *testing.T, err error, secrets ...string) {
	t.Helper()
	if err == nil {
		return
	}
	for _, shown := range []string{err.Error(), fmt.Sprintf("%v", err), fmt.Sprintf("%+v", err), fmt.Sprintf("%#v", err)} {
		for _, s := range secrets {
			if s != "" && strings.Contains(shown, s) {
				t.Errorf("the error shows %q: %s", s, shown)
			}
		}
		if strings.Contains(shown, "A sentence from the server") {
			t.Errorf("the error repeats the text of the server: %s", shown)
		}
	}
}

func secretsOf(token string) []string {
	return []string{token, strings.TrimPrefix(token, "lgtm1_"), base64.StdEncoding.EncodeToString(fakeSecret),
		base64.RawURLEncoding.EncodeToString(fakeSecret), strings.Repeat("3", 32)}
}

func TestJoinCommitVerifiedEnrollment(t *testing.T) {
	useFakeClock(t)
	f := newFolder(t)
	var seen *linkv1.JoinRequest
	link, url := startLink(t, func(_ context.Context, req *connect.Request[linkv1.JoinRequest]) (*connect.Response[linkv1.JoinResponse], error) {
		seen = req.Msg
		return goodReply(), nil
	})
	token := goodSpec(url).text(t)

	got, err := join(t, f, url, token, "host.example.test", nil)
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	if link.calls.Load() != 1 {
		t.Fatalf("%d requests, want 1", link.calls.Load())
	}
	if seen.GetTokenId() != fakeTokenID || !bytes.Equal(seen.GetTokenSecret(), fakeSecret) || seen.GetHostName() != "host.example.test" {
		t.Fatalf("request %v does not carry the token, its secret and the host name", seen)
	}
	key, err := Load(f.Paths().Key)
	if err != nil {
		t.Fatalf("the machine key after Join: %v", err)
	}
	if !bytes.Equal(seen.GetMachinePublicKey(), key.PublicKey()) {
		t.Fatal("the request carries another public key than the one on disk")
	}
	want := Server{
		Address: url, MachineID: fakeMachineID,
		ClientPublicKey: key.PublicKey(), PublicKey: serverKey.Public().(ed25519.PublicKey),
	}
	if got.Address != want.Address || got.MachineID != want.MachineID ||
		!bytes.Equal(got.ClientPublicKey, want.ClientPublicKey) || !bytes.Equal(got.PublicKey, want.PublicKey) ||
		len(got.CertificateKeySHA256) != 0 {
		t.Fatalf("Join returned %+v, want %+v", got, want)
	}

	// Join returns the enrollment and does not publish it.
	if fileExists(f.Paths().Identity) {
		t.Fatal("Join wrote identity.json; SaveServer alone publishes it")
	}
	if _, err := LoadServer(f); !errors.Is(err, ErrEnrollmentMissing) {
		t.Fatalf("LoadServer before SaveServer gave %v, want ErrEnrollmentMissing", err)
	}

	if err := SaveServer(f, got); err != nil {
		t.Fatalf("SaveServer: %v", err)
	}
	info, err := os.Lstat(f.Paths().Identity)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("identity.json after SaveServer: %v, %v", info, err)
	}
	var fields map[string]any
	data, _ := os.ReadFile(f.Paths().Identity)
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("identity.json is not JSON: %v", err)
	}
	if fields["version"] != float64(1) || fields["address"] != url || fields["machineId"] != fakeMachineID ||
		fields["clientPublicKey"] != base64.StdEncoding.EncodeToString(key.PublicKey()) ||
		fields["serverPublicKey"] != base64.StdEncoding.EncodeToString(serverKey.Public().(ed25519.PublicKey)) {
		t.Fatalf("identity.json fields are %v", fields)
	}
	if v, present := fields["certificateKeySHA256"]; present && v != "" {
		t.Fatalf("a loopback enrollment holds the certificate pin %v", v)
	}
	loaded, err := LoadServer(f)
	if err != nil || loaded.Address != got.Address || loaded.MachineID != got.MachineID ||
		!bytes.Equal(loaded.ClientPublicKey, got.ClientPublicKey) || !bytes.Equal(loaded.PublicKey, got.PublicKey) {
		t.Fatalf("LoadServer gave %+v, %v", loaded, err)
	}

	// An enrollment with a certificate pin keeps the pin through a save and a load.
	pin := bytes.Repeat([]byte{0x44}, 32)
	f2 := newFolder(t)
	pinned := got
	pinned.CertificateKeySHA256 = pin
	if err := SaveServer(f2, pinned); err != nil {
		t.Fatalf("SaveServer with a pin: %v", err)
	}
	if l, err := LoadServer(f2); err != nil || !bytes.Equal(l.CertificateKeySHA256, pin) {
		t.Fatalf("LoadServer with a pin gave %+v, %v", l, err)
	}

	// A reply with a bad machine id or a bad server key yields no enrollment.
	badReplies := map[string]*linkv1.JoinResponse{
		"empty id":         {MachineId: "", ServerPublicKey: serverKey.Public().(ed25519.PublicKey)},
		"not a uuid":       {MachineId: "machine-1", ServerPublicKey: serverKey.Public().(ed25519.PublicKey)},
		"uppercase uuid":   {MachineId: strings.ToUpper(fakeMachineID), ServerPublicKey: serverKey.Public().(ed25519.PublicKey)},
		"braces":           {MachineId: "{" + fakeMachineID + "}", ServerPublicKey: serverKey.Public().(ed25519.PublicKey)},
		"short key":        {MachineId: fakeMachineID, ServerPublicKey: serverKey.Public().(ed25519.PublicKey)[:31]},
		"long key":         {MachineId: fakeMachineID, ServerPublicKey: append(bytes.Clone(serverKey.Public().(ed25519.PublicKey)), 0)},
		"no key":           {MachineId: fakeMachineID},
		"another key":      {MachineId: fakeMachineID, ServerPublicKey: otherServerKey.Public().(ed25519.PublicKey)},
		"key of 32 zeroes": {MachineId: fakeMachineID, ServerPublicKey: make([]byte, 32)},
	}
	for name, reply := range badReplies {
		t.Run("bad reply/"+name, func(t *testing.T) {
			f := newFolder(t)
			link, url := startLink(t, func(context.Context, *connect.Request[linkv1.JoinRequest]) (*connect.Response[linkv1.JoinResponse], error) {
				return connect.NewResponse(reply), nil
			})
			got, err := join(t, f, url, goodSpec(url).text(t), "host.example.test", nil)
			if err == nil {
				t.Fatalf("Join accepted the reply and returned %+v", got)
			}
			if link.calls.Load() != 1 {
				t.Fatalf("a bad reply was tried %d times, want 1", link.calls.Load())
			}
			if fileExists(f.Paths().Identity) {
				t.Fatal("a bad reply left identity.json")
			}
			if name == "another key" || name == "key of 32 zeroes" {
				wantCode(t, err, "server_key_mismatch")
			}
		})
	}
}

func TestJoinReusesDurableKey(t *testing.T) {
	for _, lost := range []string{"unavailable error", "aborted reply"} {
		t.Run(lost, func(t *testing.T) {
			useFakeClock(t)
			f := newFolder(t)
			var mu sync.Mutex
			var keys [][]byte
			var onDisk [][]byte
			var requests []*linkv1.JoinRequest
			_, url := startLink(t, func(_ context.Context, req *connect.Request[linkv1.JoinRequest]) (*connect.Response[linkv1.JoinResponse], error) {
				mu.Lock()
				defer mu.Unlock()
				// The key must be on disk before the first request leaves.
				if c, err := Load(f.Paths().Key); err == nil {
					onDisk = append(onDisk, c.PublicKey())
				} else {
					onDisk = append(onDisk, nil)
				}
				keys = append(keys, bytes.Clone(req.Msg.GetMachinePublicKey()))
				requests = append(requests, req.Msg)
				if len(keys) == 1 {
					// The server commits, and the reply is lost.
					if lost == "aborted reply" {
						panic(http.ErrAbortHandler)
					}
					return nil, connect.NewError(connect.CodeUnavailable, errors.New("reply lost"))
				}
				return goodReply(), nil
			})
			token := goodSpec(url).text(t)
			retries := 0
			got, err := join(t, f, url, token, "host.example.test", func() { retries++ })
			if err != nil {
				t.Fatalf("Join: %v", err)
			}
			mu.Lock()
			defer mu.Unlock()
			if len(keys) != 2 {
				t.Fatalf("%d requests, want 2", len(keys))
			}
			if retries != 1 {
				t.Fatalf("onRetry was called %d times, want once", retries)
			}
			if len(onDisk[0]) != ed25519.PublicKeySize || !bytes.Equal(onDisk[0], keys[0]) {
				t.Fatal("the key was not on disk when the first request arrived")
			}
			if !bytes.Equal(keys[0], keys[1]) || !bytes.Equal(keys[1], got.ClientPublicKey) {
				t.Fatal("the retry used another key than the first request")
			}
			for _, r := range requests {
				if r.GetTokenId() != fakeTokenID || !bytes.Equal(r.GetTokenSecret(), fakeSecret) {
					t.Fatal("a retry changed the token")
				}
			}
			entries, _ := os.ReadDir(f.Paths().Dir)
			var keyFiles int
			for _, e := range entries {
				if e.Name() == state.KeyFile {
					keyFiles++
				}
			}
			if keyFiles != 1 {
				t.Fatalf("%d key files", keyFiles)
			}
		})
	}

	t.Run("a new process reuses the pending key", func(t *testing.T) {
		useFakeClock(t)
		f := newFolder(t)
		var keys [][]byte
		_, url := startLink(t, func(_ context.Context, req *connect.Request[linkv1.JoinRequest]) (*connect.Response[linkv1.JoinResponse], error) {
			keys = append(keys, bytes.Clone(req.Msg.GetMachinePublicKey()))
			if len(keys) == 1 {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("stop"))
			}
			return goodReply(), nil
		})
		token := goodSpec(url).text(t)
		if _, err := join(t, f, url, token, "h", nil); err == nil {
			t.Fatal("the first Join should stop on invalid_argument")
		}
		if _, err := join(t, f, url, token, "h", nil); err != nil {
			t.Fatalf("the second Join: %v", err)
		}
		if len(keys) != 2 || !bytes.Equal(keys[0], keys[1]) || len(keys[0]) != ed25519.PublicKeySize {
			t.Fatal("a new Join did not reuse the key that the first one left on disk")
		}
	})

	t.Run("a committed identity prevents a second join", func(t *testing.T) {
		useFakeClock(t)
		f := newFolder(t)
		link, url := startLink(t, answerGood)
		token := goodSpec(url).text(t)
		got, err := join(t, f, url, token, "h", nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := SaveServer(f, got); err != nil {
			t.Fatal(err)
		}
		before, _ := os.ReadFile(f.Paths().Key)
		link.calls.Store(0)
		if _, err := join(t, f, url, token, "h", nil); !errors.Is(err, ErrEnrollmentExists) {
			t.Fatalf("a second Join gave %v, want ErrEnrollmentExists", err)
		}
		if link.calls.Load() != 0 {
			t.Fatalf("a second Join sent %d requests", link.calls.Load())
		}
		if after, _ := os.ReadFile(f.Paths().Key); !bytes.Equal(before, after) {
			t.Fatal("a second Join changed the key")
		}
	})

	t.Run("an invalid identity file also prevents a join", func(t *testing.T) {
		useFakeClock(t)
		f := newFolder(t)
		link, url := startLink(t, answerGood)
		if err := os.WriteFile(f.Paths().Identity, []byte("not json"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := join(t, f, url, goodSpec(url).text(t), "h", nil); !errors.Is(err, ErrEnrollmentExists) {
			t.Fatalf("Join over an invalid identity file gave %v, want ErrEnrollmentExists", err)
		}
		if link.calls.Load() != 0 || fileExists(f.Paths().Key) {
			t.Fatalf("Join over an invalid identity file sent %d requests or made a key", link.calls.Load())
		}
		if data, _ := os.ReadFile(f.Paths().Identity); string(data) != "not json" {
			t.Fatal("Join changed the identity file")
		}
	})
}

func TestJoinRejectsAddressAndRedirect(t *testing.T) {
	t.Run("a token for another origin", func(t *testing.T) {
		useFakeClock(t)
		f := newFolder(t)
		linkA, urlA := startLink(t, answerGood)
		linkB, urlB := startLink(t, answerGood)
		token := goodSpec(urlA).text(t)
		for _, address := range []string{urlB, "http://127.0.0.1:1", "https://example.test", "http://localhost:" + strings.TrimPrefix(urlA, "http://127.0.0.1:")} {
			if got, err := join(t, f, address, token, "h", nil); err == nil {
				t.Fatalf("Join to %s with a token for %s returned %+v", address, urlA, got)
			} else {
				wantNoLeak(t, err, secretsOf(token)...)
			}
		}
		if linkA.calls.Load() != 0 || linkB.calls.Load() != 0 {
			t.Fatalf("requests reached %d and %d times", linkA.calls.Load(), linkB.calls.Load())
		}
		if fileExists(f.Paths().Identity) {
			t.Fatal("a refused address left identity.json")
		}
	})

	t.Run("a redirect cannot reach a second server", func(t *testing.T) {
		useFakeClock(t)
		f := newFolder(t)
		linkB, urlB := startLink(t, answerGood)
		var hits atomic.Int32
		urlA := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			http.Redirect(w, r, urlB+r.URL.Path, http.StatusTemporaryRedirect)
		}))
		token := goodSpec(urlA).text(t)
		got, err := join(t, f, urlA, token, "h", nil)
		if err == nil {
			t.Fatalf("Join followed a redirect and returned %+v", got)
		}
		wantNoLeak(t, err, secretsOf(token)...)
		if linkB.calls.Load() != 0 {
			t.Fatalf("the redirect target received %d requests", linkB.calls.Load())
		}
		if hits.Load() == 0 {
			t.Fatal("the first server was never asked")
		}
		if fileExists(f.Paths().Identity) {
			t.Fatal("a redirect left identity.json")
		}
	})

	t.Run("a signer other than the token's yields no identity", func(t *testing.T) {
		useFakeClock(t)
		f := newFolder(t)
		link, url := startLink(t, func(context.Context, *connect.Request[linkv1.JoinRequest]) (*connect.Response[linkv1.JoinResponse], error) {
			return connect.NewResponse(&linkv1.JoinResponse{MachineId: fakeMachineID, ServerPublicKey: otherServerKey.Public().(ed25519.PublicKey)}), nil
		})
		token := goodSpec(url).text(t)
		got, err := join(t, f, url, token, "h", nil)
		if err == nil {
			t.Fatalf("Join accepted a wrong signer: %+v", got)
		}
		wantCode(t, err, "server_key_mismatch")
		wantNoLeak(t, err, secretsOf(token)...)
		if link.calls.Load() != 1 || fileExists(f.Paths().Identity) {
			t.Fatalf("%d requests; identity file present: %v", link.calls.Load(), fileExists(f.Paths().Identity))
		}
	})

	t.Run("an oversized reply yields no identity and no echo", func(t *testing.T) {
		useFakeClock(t)
		f := newFolder(t)
		huge := strings.Repeat("A", 70<<10)
		link, url := startLink(t, func(context.Context, *connect.Request[linkv1.JoinRequest]) (*connect.Response[linkv1.JoinResponse], error) {
			return connect.NewResponse(&linkv1.JoinResponse{MachineId: huge, ServerPublicKey: serverKey.Public().(ed25519.PublicKey)}), nil
		})
		token := goodSpec(url).text(t)
		got, err := join(t, f, url, token, "h", nil)
		if err == nil {
			t.Fatalf("Join accepted a reply of 70 KiB: %+v", got)
		}
		if _, ok := code(err); !ok {
			t.Fatalf("the error %v carries no fixed diagnostic", err)
		}
		if strings.Contains(err.Error(), huge[:64]) {
			t.Fatal("the error repeats the reply")
		}
		wantNoLeak(t, err, secretsOf(token)...)
		if link.calls.Load() != 1 || fileExists(f.Paths().Identity) {
			t.Fatalf("%d requests; identity file present: %v", link.calls.Load(), fileExists(f.Paths().Identity))
		}
	})

	t.Run("a bad token reaches no server", func(t *testing.T) {
		useFakeClock(t)
		link, url := startLink(t, answerGood)
		good := goodSpec(url)
		mutate := func(edit func(*tokenSpec)) string {
			s := good
			edit(&s)
			return s.text(t)
		}
		raw := good.text(t)
		bad := map[string]string{
			"empty":                  "",
			"no prefix":              strings.TrimPrefix(raw, "lgtm1_"),
			"another prefix":         "lgtm2_" + strings.TrimPrefix(raw, "lgtm1_"),
			"padded base64":          raw + "=",
			"not base64":             "lgtm1_!!!!",
			"standard alphabet":      "lgtm1_" + base64.StdEncoding.EncodeToString([]byte(strings.Repeat("\xfb\xff", 40))),
			"not a message":          "lgtm1_" + base64.RawURLEncoding.EncodeToString([]byte{0xff, 0xff, 0xff}),
			"over 4096 bytes":        mutate(func(s *tokenSpec) { s.Address = url + "/" + strings.Repeat("a", 4100) }),
			"uppercase id":           mutate(func(s *tokenSpec) { s.ID = strings.ToUpper(s.ID) }),
			"id not a uuid":          mutate(func(s *tokenSpec) { s.ID = "token-1" }),
			"empty id":               mutate(func(s *tokenSpec) { s.ID = "" }),
			"short secret":           mutate(func(s *tokenSpec) { s.Secret = s.Secret[:31] }),
			"long secret":            mutate(func(s *tokenSpec) { s.Secret = append(bytes.Clone(s.Secret), 0) }),
			"no secret":              mutate(func(s *tokenSpec) { s.Secret = nil }),
			"short server digest":    mutate(func(s *tokenSpec) { s.ServerSHA = s.ServerSHA[:31] }),
			"no server digest":       mutate(func(s *tokenSpec) { s.ServerSHA = nil }),
			"bad certificate digest": mutate(func(s *tokenSpec) { s.CertSHA = []byte{1, 2, 3} }),
			"address with a path":    mutate(func(s *tokenSpec) { s.Address = url + "/x" }),
			"address without scheme": mutate(func(s *tokenSpec) { s.Address = strings.TrimPrefix(url, "http://") }),
			"empty address":          mutate(func(s *tokenSpec) { s.Address = "" }),
			"plain http off loopback": mutate(func(s *tokenSpec) {
				s.Address = "http://example.test"
			}),
			"https without a pin": mutate(func(s *tokenSpec) { s.Address = "https://example.test" }),
		}
		for name, token := range bad {
			f := newFolder(t)
			address := url
			if name == "plain http off loopback" {
				address = "http://example.test"
			}
			if name == "https without a pin" {
				address = "https://example.test"
			}
			if got, err := join(t, f, address, token, "h", nil); err == nil {
				t.Errorf("%s: Join returned %+v", name, got)
			} else if token != "" {
				wantNoLeak(t, err, token)
			}
			if fileExists(f.Paths().Identity) {
				t.Errorf("%s: identity.json was written", name)
			}
		}
		if link.calls.Load() != 0 {
			t.Fatalf("bad tokens caused %d requests", link.calls.Load())
		}
	})
}

func TestJoinRejectsExpiredAndRedactsRefusal(t *testing.T) {
	// Each of the five reasons stops at once and shows only its fixed code.
	for _, reason := range []string{"token_unknown", "token_expired", "token_revoked", "token_used", "machine_removed"} {
		t.Run("reason "+reason, func(t *testing.T) {
			clock := useFakeClock(t)
			f := newFolder(t)
			link, url := startLink(t, refuse(connect.CodePermissionDenied, reason))
			token := goodSpec(url).text(t)
			retries := 0
			got, err := join(t, f, url, token, "h", func() { retries++ })
			if err == nil {
				t.Fatalf("Join returned %+v", got)
			}
			wantCode(t, err, reason)
			wantNoLeak(t, err, secretsOf(token)...)
			if link.calls.Load() != 1 || retries != 0 || len(clock.waited()) != 0 {
				t.Fatalf("%d requests, %d retries, waits %v; a final refusal is not retried", link.calls.Load(), retries, clock.waited())
			}
			if fileExists(f.Paths().Identity) {
				t.Fatal("a refusal left identity.json")
			}
			if !fileExists(f.Paths().Key) {
				t.Fatal("a refusal removed the machine key")
			}
		})
	}

	// A missing or unrecognized reason falls back to token_refused, and both stops of Join hold.
	stops := []struct {
		name   string
		code   connect.Code
		reason string
		want   string
	}{
		{"permission_denied without metadata", connect.CodePermissionDenied, "", "token_refused"},
		{"permission_denied with an unknown reason", connect.CodePermissionDenied, "hunter2-not-a-reason", "token_refused"},
		{"permission_denied with an empty-looking reason", connect.CodePermissionDenied, " ", "token_refused"},
		{"permission_denied with a long reason", connect.CodePermissionDenied, strings.Repeat("x", 300), "token_refused"},
		{"permission_denied with a reason in another case", connect.CodePermissionDenied, "TOKEN_USED", "token_refused"},
		{"invalid_argument without metadata", connect.CodeInvalidArgument, "", ""},
		{"invalid_argument with an unknown reason", connect.CodeInvalidArgument, "hunter2-not-a-reason", ""},
		{"invalid_argument with a known reason", connect.CodeInvalidArgument, "token_used", ""},
	}
	for _, c := range stops {
		t.Run(c.name, func(t *testing.T) {
			clock := useFakeClock(t)
			f := newFolder(t)
			link, url := startLink(t, refuse(c.code, c.reason))
			token := goodSpec(url).text(t)
			retries := 0
			got, err := join(t, f, url, token, "h", func() { retries++ })
			if err == nil {
				t.Fatalf("Join returned %+v", got)
			}
			if c.want != "" {
				wantCode(t, err, c.want)
			} else if got, ok := code(err); !ok || got == "" {
				t.Fatalf("error %v carries no fixed diagnostic", err)
			}
			wantNoLeak(t, err, append(secretsOf(token), "hunter2-not-a-reason", strings.Repeat("x", 300))...)
			if link.calls.Load() != 1 || retries != 0 || len(clock.waited()) != 0 {
				t.Fatalf("%d requests, %d retries, waits %v; the code is a stop", link.calls.Load(), retries, clock.waited())
			}
		})
	}

	t.Run("the reply is lost and the token has expired", func(t *testing.T) {
		clock := useFakeClock(t)
		f := newFolder(t)
		var n atomic.Int32
		link, url := startLink(t, func(ctx context.Context, req *connect.Request[linkv1.JoinRequest]) (*connect.Response[linkv1.JoinResponse], error) {
			if n.Add(1) == 1 {
				return nil, connect.NewError(connect.CodeUnavailable, errors.New("reply lost"))
			}
			return refuse(connect.CodePermissionDenied, "token_expired")(ctx, req)
		})
		token := goodSpec(url).text(t)
		retries := 0
		got, err := join(t, f, url, token, "h", func() { retries++ })
		if err == nil {
			t.Fatalf("Join returned %+v", got)
		}
		wantCode(t, err, "token_expired")
		if link.calls.Load() != 2 || retries != 1 || len(clock.waited()) != 1 {
			t.Fatalf("%d requests, %d retries, waits %v", link.calls.Load(), retries, clock.waited())
		}
		if !fileExists(f.Paths().Key) || fileExists(f.Paths().Identity) {
			t.Fatal("after a lost reply and an expiry the key must stay and no identity may exist")
		}
	})

	t.Run("the retries have a fixed total budget", func(t *testing.T) {
		clock := useFakeClock(t)
		f := newFolder(t)
		link, url := startLink(t, refuse(connect.CodeUnavailable, ""))
		token := goodSpec(url).text(t)
		retries := 0
		start := joinNow()
		got, err := join(t, f, url, token, "h", func() { retries++ })
		if err == nil {
			t.Fatalf("Join returned %+v", got)
		}
		wantNoLeak(t, err, secretsOf(token)...)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("the budget ended in a context error: %v", err)
		}
		waits := clock.waited()
		if len(waits) < 10 {
			t.Fatalf("only %d waits: %v", len(waits), waits)
		}
		for i, want := range []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 15 * time.Second} {
			if waits[i] != want {
				t.Fatalf("wait %d is %v, want %v; waits %v", i, waits[i], want, waits[:6])
			}
		}
		var total time.Duration
		for _, w := range waits {
			if w > 15*time.Second || w <= 0 {
				t.Fatalf("a wait of %v", w)
			}
			total += w
		}
		if elapsed := joinNow().Sub(start); elapsed != total || total > 15*time.Minute || total < 15*time.Minute-15*time.Second {
			t.Fatalf("the waits add up to %v and the clock moved %v; the budget is 15 minutes", total, elapsed)
		}
		if calls := int(link.calls.Load()); calls < len(waits) || calls > len(waits)+1 {
			t.Fatalf("%d requests for %d waits", calls, len(waits))
		}
		if retries != 1 {
			t.Fatalf("onRetry was called %d times, want once", retries)
		}
	})

	t.Run("a cancelled context ends the retries", func(t *testing.T) {
		useFakeClock(t)
		f := newFolder(t)
		ctx, cancel := context.WithCancel(context.Background())
		link, url := startLink(t, func(context.Context, *connect.Request[linkv1.JoinRequest]) (*connect.Response[linkv1.JoinResponse], error) {
			cancel()
			return nil, connect.NewError(connect.CodeUnavailable, errors.New("down"))
		})
		finished := make(chan error, 1)
		go func() {
			_, err := Join(ctx, f, url, goodSpec(url).text(t), "h", nil)
			finished <- err
		}()
		select {
		case err := <-finished:
			if err == nil {
				t.Fatal("Join succeeded after its context was cancelled")
			}
		case <-time.After(callBudget):
			t.Fatal("Join did not stop after its context was cancelled")
		}
		if link.calls.Load() != 1 {
			t.Fatalf("%d requests after the cancellation", link.calls.Load())
		}
	})
}

func TestLoadServerRejectsUnsafeFile(t *testing.T) {
	good := Server{
		Address: "http://127.0.0.1:8787", MachineID: fakeMachineID,
		ClientPublicKey: bytes.Repeat([]byte{0x07}, 32), PublicKey: serverKey.Public().(ed25519.PublicKey),
	}
	fields := func(edit func(map[string]any)) []byte {
		m := map[string]any{
			"version":         1,
			"address":         good.Address,
			"machineId":       good.MachineID,
			"clientPublicKey": base64.StdEncoding.EncodeToString(good.ClientPublicKey),
			"serverPublicKey": base64.StdEncoding.EncodeToString(good.PublicKey),
		}
		if edit != nil {
			edit(m)
		}
		data, err := json.Marshal(m)
		if err != nil {
			panic(err)
		}
		return data
	}

	t.Run("a good file loads", func(t *testing.T) {
		f := newFolder(t)
		if err := os.WriteFile(f.Paths().Identity, fields(nil), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := LoadServer(f)
		if err != nil || got.Address != good.Address || got.MachineID != good.MachineID ||
			!bytes.Equal(got.ClientPublicKey, good.ClientPublicKey) || !bytes.Equal(got.PublicKey, good.PublicKey) {
			t.Fatalf("LoadServer gave %+v, %v", got, err)
		}
	})

	t.Run("an absent file", func(t *testing.T) {
		f := newFolder(t)
		if _, err := LoadServer(f); !errors.Is(err, ErrEnrollmentMissing) {
			t.Fatalf("LoadServer gave %v, want ErrEnrollmentMissing", err)
		}
	})

	invalid := map[string]func(t *testing.T, f *state.Folder){
		"a symbolic link to a good file": func(t *testing.T, f *state.Folder) {
			target := filepath.Join(filepath.Dir(f.Paths().Dir), "elsewhere.json")
			if err := os.WriteFile(target, fields(nil), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, f.Paths().Identity); err != nil {
				t.Fatal(err)
			}
		},
		"a folder":     func(t *testing.T, f *state.Folder) { _ = os.Mkdir(f.Paths().Identity, 0o700) },
		"a named pipe": func(t *testing.T, f *state.Folder) { _ = syscall.Mkfifo(f.Paths().Identity, 0o600) },
		"mode 0644":    func(t *testing.T, f *state.Folder) { write(t, f, fields(nil), 0o644) },
		"mode 0640":    func(t *testing.T, f *state.Folder) { write(t, f, fields(nil), 0o640) },
		"mode 0604":    func(t *testing.T, f *state.Folder) { write(t, f, fields(nil), 0o604) },
		"empty":        func(t *testing.T, f *state.Folder) { write(t, f, nil, 0o600) },
		"not JSON":     func(t *testing.T, f *state.Folder) { write(t, f, []byte("not json"), 0o600) },
		"JSON that is not an object": func(t *testing.T, f *state.Folder) {
			write(t, f, []byte(`["a"]`), 0o600)
		},
		"trailing bytes": func(t *testing.T, f *state.Folder) { write(t, f, append(fields(nil), []byte(` {}`)...), 0o600) },
		"over the size bound": func(t *testing.T, f *state.Folder) {
			write(t, f, fields(func(m map[string]any) { m["padding"] = strings.Repeat("x", 1<<20) }), 0o600)
		},
		"version 0": func(t *testing.T, f *state.Folder) {
			write(t, f, fields(func(m map[string]any) { m["version"] = 0 }), 0o600)
		},
		"version 2": func(t *testing.T, f *state.Folder) {
			write(t, f, fields(func(m map[string]any) { m["version"] = 2 }), 0o600)
		},
		"version absent": func(t *testing.T, f *state.Folder) {
			write(t, f, fields(func(m map[string]any) { delete(m, "version") }), 0o600)
		},
		"version as text": func(t *testing.T, f *state.Folder) {
			write(t, f, fields(func(m map[string]any) { m["version"] = "1" }), 0o600)
		},
		"client key of 31 bytes": func(t *testing.T, f *state.Folder) {
			write(t, f, fields(func(m map[string]any) {
				m["clientPublicKey"] = base64.StdEncoding.EncodeToString(good.ClientPublicKey[:31])
			}), 0o600)
		},
		"client key of 33 bytes": func(t *testing.T, f *state.Folder) {
			write(t, f, fields(func(m map[string]any) {
				m["clientPublicKey"] = base64.StdEncoding.EncodeToString(append(bytes.Clone(good.ClientPublicKey), 1))
			}), 0o600)
		},
		"server key of 31 bytes": func(t *testing.T, f *state.Folder) {
			write(t, f, fields(func(m map[string]any) { m["serverPublicKey"] = base64.StdEncoding.EncodeToString(good.PublicKey[:31]) }), 0o600)
		},
		"server key absent": func(t *testing.T, f *state.Folder) {
			write(t, f, fields(func(m map[string]any) { delete(m, "serverPublicKey") }), 0o600)
		},
		"key without padding": func(t *testing.T, f *state.Folder) {
			write(t, f, fields(func(m map[string]any) { m["serverPublicKey"] = base64.RawStdEncoding.EncodeToString(good.PublicKey) }), 0o600)
		},
		"key in the URL alphabet": func(t *testing.T, f *state.Folder) {
			write(t, f, fields(func(m map[string]any) {
				m["serverPublicKey"] = base64.URLEncoding.EncodeToString(bytes.Repeat([]byte{0xfb}, 32))
			}), 0o600)
		},
		"machine id in upper case": func(t *testing.T, f *state.Folder) {
			write(t, f, fields(func(m map[string]any) { m["machineId"] = strings.ToUpper(good.MachineID) }), 0o600)
		},
		"machine id absent": func(t *testing.T, f *state.Folder) {
			write(t, f, fields(func(m map[string]any) { delete(m, "machineId") }), 0o600)
		},
		"address with a path": func(t *testing.T, f *state.Folder) {
			write(t, f, fields(func(m map[string]any) { m["address"] = "http://127.0.0.1:8787/x" }), 0o600)
		},
		"plain http off loopback": func(t *testing.T, f *state.Folder) {
			write(t, f, fields(func(m map[string]any) { m["address"] = "http://example.test" }), 0o600)
		},
		"address absent": func(t *testing.T, f *state.Folder) {
			write(t, f, fields(func(m map[string]any) { delete(m, "address") }), 0o600)
		},
		"https without a pin": func(t *testing.T, f *state.Folder) {
			write(t, f, fields(func(m map[string]any) { m["address"] = "https://example.test" }), 0o600)
		},
		"pin of 31 bytes": func(t *testing.T, f *state.Folder) {
			write(t, f, fields(func(m map[string]any) {
				m["address"] = "https://example.test"
				m["certificateKeySHA256"] = base64.StdEncoding.EncodeToString(make([]byte, 31))
			}), 0o600)
		},
		"a pin with plain http": func(t *testing.T, f *state.Folder) {
			write(t, f, fields(func(m map[string]any) {
				m["certificateKeySHA256"] = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32))
			}), 0o600)
		},
	}
	for name, setup := range invalid {
		t.Run(name, func(t *testing.T) {
			f := newFolder(t)
			setup(t, f)
			if got, err := LoadServer(f); !errors.Is(err, ErrEnrollmentInvalid) {
				t.Fatalf("LoadServer gave %+v, %v; want ErrEnrollmentInvalid", got, err)
			} else if errors.Is(err, ErrEnrollmentMissing) {
				t.Fatalf("an unsafe file was reported as missing: %v", err)
			}
		})
	}
}

func write(t *testing.T, f *state.Folder, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(f.Paths().Identity, data, mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(f.Paths().Identity, mode); err != nil {
		t.Fatal(err)
	}
}

func TestJoinSyncFailureKeepsKey(t *testing.T) {
	useFakeClock(t)
	f := newFolder(t)
	link, url := startLink(t, answerGood)
	token := goodSpec(url).text(t)

	// The folder cannot be synced, so the new key is in place but may not be durable.
	restore := state.SetSyncFolderForTest(func(string) error { return errors.New("injected sync failure") })
	defer restore()

	if got, err := join(t, f, url, token, "h", nil); err == nil {
		t.Fatalf("Join went on after the key sync failed: %+v", got)
	} else {
		wantNoLeak(t, err, secretsOf(token)...)
	}
	if link.calls.Load() != 0 {
		t.Fatalf("%d requests left before the key was synced", link.calls.Load())
	}
	key, err := Load(f.Paths().Key)
	if err != nil {
		t.Fatalf("the key is not kept: %v", err)
	}
	pending := key.PublicKey()

	// A new process with the pending key confirms the sync before its first attempt.
	if got, err := join(t, f, url, token, "h", nil); err == nil {
		t.Fatalf("a second Join went on over an unsynced key: %+v", got)
	}
	if link.calls.Load() != 0 {
		t.Fatalf("%d requests left over an unsynced pending key", link.calls.Load())
	}

	// With the sync working again the same key is sent.
	restore()
	var sent []byte
	link2, url2 := startLink(t, func(_ context.Context, req *connect.Request[linkv1.JoinRequest]) (*connect.Response[linkv1.JoinResponse], error) {
		sent = bytes.Clone(req.Msg.GetMachinePublicKey())
		return goodReply(), nil
	})
	got, err := join(t, f, url2, goodSpec(url2).text(t), "h", nil)
	if err != nil {
		t.Fatalf("Join after the sync works: %v", err)
	}
	if link2.calls.Load() != 1 || !bytes.Equal(sent, pending) || !bytes.Equal(got.ClientPublicKey, pending) {
		t.Fatal("Join did not send the key that was kept")
	}
}

func TestSaveServerSyncFailureKeepsFile(t *testing.T) {
	f := newFolder(t)
	key, err := Create(f.Paths().Key)
	if err != nil {
		t.Fatal(err)
	}
	server := Server{
		Address: "http://127.0.0.1:8787", MachineID: fakeMachineID,
		ClientPublicKey: key.PublicKey(), PublicKey: serverKey.Public().(ed25519.PublicKey),
	}
	restore := state.SetSyncFolderForTest(func(string) error { return errors.New("injected sync failure") })
	defer restore()
	if err := SaveServer(f, server); err == nil {
		t.Fatal("SaveServer reported success although the folder could not be synced")
	}
	if !fileExists(f.Paths().Identity) || !fileExists(f.Paths().Key) {
		t.Fatal("a failed sync must keep identity.json and the key")
	}
	restore()
	if got, err := LoadServer(f); err != nil || got.MachineID != fakeMachineID {
		t.Fatalf("the kept file does not load: %+v, %v", got, err)
	}
}

func TestJoinHostNameFollowsWireRules(t *testing.T) {
	sent := func(t *testing.T, name string) string {
		t.Helper()
		useFakeClock(t)
		f := newFolder(t)
		var host string
		var calls int
		_, url := startLink(t, func(_ context.Context, req *connect.Request[linkv1.JoinRequest]) (*connect.Response[linkv1.JoinResponse], error) {
			calls++
			host = req.Msg.GetHostName()
			return goodReply(), nil
		})
		if _, err := join(t, f, url, goodSpec(url).text(t), name, nil); err != nil {
			t.Fatalf("Join: %v", err)
		}
		if calls != 1 {
			t.Fatalf("%d requests", calls)
		}
		return host
	}
	check := func(t *testing.T, got string) {
		t.Helper()
		if !utf8.ValidString(got) {
			t.Errorf("the sent name %q is not valid UTF-8", got)
		}
		if strings.ContainsRune(got, 0) {
			t.Errorf("the sent name %q holds a zero byte", got)
		}
		if len(got) > 255 {
			t.Errorf("the sent name has %d bytes, the limit is 255", len(got))
		}
	}
	long := func(unit string, n int) string { return strings.Repeat(unit, n) }

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "host.example.test", "host.example.test"},
		{"empty", "", ""},
		{"zero bytes are omitted", "a\x00b\x00\x00c", "abc"},
		{"only zero bytes", "\x00\x00", ""},
		{"exactly 255 bytes", long("a", 255), long("a", 255)},
		{"256 ASCII bytes", long("a", 256), long("a", 255)},
		{"a two-byte character across the limit", long("a", 254) + "é", long("a", 254)},
		{"a two-byte character that fits", long("a", 253) + "é", long("a", 253) + "é"},
		{"two-byte characters only", long("é", 200), long("é", 127)},
		{"three-byte characters that fill the limit", long("€", 100), long("€", 85)},
		{"a three-byte character across the limit", long("a", 254) + "€", long("a", 254)},
		{"a three-byte character split after one byte", long("a", 253) + "€", long("a", 253)},
		{"a four-byte character across the limit", long("a", 253) + "😀", long("a", 253)},
		{"four-byte characters only", long("😀", 80), long("😀", 63)},
		// Zero bytes are omitted first, and only then is the name cut.
		{"zero bytes do not use up the limit", "a\x00" + long("b", 255), "a" + long("b", 254)},
		{"zero bytes before a cut character", long("a", 253) + "\x00\x00é", long("a", 253) + "é"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := sent(t, c.in)
			check(t, got)
			if got != c.want {
				t.Fatalf("the sent name is %q (%d bytes), want %q (%d bytes)", got, len(got), c.want, len(c.want))
			}
		})
	}

	t.Run("bytes that are not UTF-8", func(t *testing.T) {
		for _, in := range []string{"ab\xffcd", "\xc3", long("a", 254) + "\xc3", "\xf0\x9f\x98", long("é", 130) + "\xe2\x82"} {
			got := sent(t, in)
			check(t, got)
		}
		if got := sent(t, "ab\xffcd"); !strings.HasPrefix(got, "ab") || !strings.HasSuffix(got, "cd") {
			t.Errorf("the valid characters were lost: %q", got)
		}
	})
}
