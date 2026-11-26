package vault_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest/migrated"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/vault"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/vault/vaulttest"
)

const (
	loginPath     = "/v1/auth/approle/login"
	keyPath       = "/v1/lgtm/data/server/link/signing-key"
	roleID        = "fixture-role-id"
	secretID      = "fixture-secret-id"
	tokenPrefix   = "hvb.fixture-token-"
	sensitiveBody = "fixture-sensitive-response"
	credentials   = `{"role_id":"` + roleID + `","secret_id":"` + secretID + `"}` + "\n"
)

var fixtureSeed = [32]byte{0xfb, 0xff, 3, 4, 5, 6, 7, 8}

type wireRequest struct {
	method string
	path   string
	token  string
	seed   [32]byte
}

type wireVault struct {
	mu       sync.Mutex
	requests []wireRequest
	logins   int
	address  string
	file     string
}

func startWireVault(t *testing.T, login func(http.ResponseWriter, *http.Request, int), key func(http.ResponseWriter, *http.Request, wireRequest)) *wireVault {
	t.Helper()
	w := &wireVault{file: credentialFile(t, credentials, 0o400)}
	handler := http.HandlerFunc(func(out http.ResponseWriter, r *http.Request) {
		out.Header().Set("Content-Type", "application/json")
		call := wireRequest{method: r.Method, path: r.URL.Path, token: r.Header.Get("X-Vault-Token")}
		var validCreate bool
		if call.method == http.MethodPost && call.path == keyPath {
			call.seed, validCreate = readCreate(out, r)
		}
		w.mu.Lock()
		w.requests = append(w.requests, call)
		loginNumber := w.logins
		if call.path == loginPath {
			w.logins++
			loginNumber = w.logins
		}
		w.mu.Unlock()

		if r.URL.RawQuery != "" {
			t.Error("The vault request had an unexpected query string.")
			out.WriteHeader(http.StatusBadRequest)
			return
		}
		switch {
		case call.path == loginPath && call.method == http.MethodPost:
			var fields map[string]string
			body, err := io.ReadAll(http.MaxBytesReader(out, r.Body, 64<<10))
			if err != nil || json.Unmarshal(body, &fields) != nil || len(fields) != 2 ||
				fields["role_id"] != roleID || fields["secret_id"] != secretID || call.token != "" {
				t.Error("AppRole login must send exactly the two credential fields without a vault token.")
				out.WriteHeader(http.StatusBadRequest)
				return
			}
			if login == nil {
				replyLogin(out, loginNumber, 900)
				return
			}
			login(out, r, loginNumber)
		case call.path == keyPath && (call.method == http.MethodGet || call.method == http.MethodPost):
			if !strings.HasPrefix(call.token, tokenPrefix) {
				t.Error("The signing-key request did not use the AppRole token header.")
				out.WriteHeader(http.StatusForbidden)
				return
			}
			if call.method == http.MethodPost && !validCreate {
				t.Error("A signing-key write must explicitly use options.cas=0 and one canonical 32-byte seed.")
				out.WriteHeader(http.StatusBadRequest)
				return
			}
			key(out, r, call)
		default:
			t.Error("The server sent an unexpected OpenBao path or method.")
			out.WriteHeader(http.StatusNotFound)
		}
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	w.address = server.URL
	return w
}

func readCreate(w http.ResponseWriter, r *http.Request) ([32]byte, bool) {
	var seed [32]byte
	var doc struct {
		Options struct {
			CAS *int `json:"cas"`
		} `json:"options"`
		Data struct {
			Seed string `json:"private_seed_b64"`
		} `json:"data"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil || doc.Options.CAS == nil || *doc.Options.CAS != 0 {
		return seed, false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return seed, false
	}
	data, err := base64.StdEncoding.DecodeString(doc.Data.Seed)
	if err != nil || len(data) != len(seed) || base64.StdEncoding.EncodeToString(data) != doc.Data.Seed {
		return seed, false
	}
	copy(seed[:], data)
	return seed, true
}

func replyLogin(w http.ResponseWriter, number, leaseSeconds int) {
	fmt.Fprintf(w, `{"auth":{"client_token":%q,"lease_duration":%d,"renewable":false,"policies":["lgtm-server"]}}`,
		tokenNumber(number), leaseSeconds)
}

func tokenNumber(number int) string {
	return fmt.Sprintf("%s%d", tokenPrefix, number)
}

func replySeed(w http.ResponseWriter, encoded string, version int) {
	fmt.Fprintf(w, `{"data":{"data":{"private_seed_b64":%q},"metadata":{"version":%d}}}`, encoded, version)
}

func replyAbsent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	io.WriteString(w, `{"errors":[]}`)
}

func (w *wireVault) calls() []wireRequest {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]wireRequest(nil), w.requests...)
}

func (w *wireVault) open(t *testing.T) *vault.Client {
	t.Helper()
	return openClient(t, w.address, w.file)
}

func openClient(t *testing.T, address, file string) *vault.Client {
	t.Helper()
	c, err := vault.Open(address, file, false)
	if err != nil || c == nil {
		t.Fatal("Open did not accept the protected loopback credential.")
	}
	return c
}

func credentialFile(t *testing.T, contents string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "openbao.json")
	if err := os.WriteFile(path, []byte(contents), mode); err != nil {
		t.Fatal("The fixture could not write an OpenBao credential.")
	}
	return path
}

func newVaultStore(t *testing.T) *store.Store {
	t.Helper()
	return migrated.NewStore(t, "public", vault.Migrations())
}

func assertMethods(t *testing.T, calls []wireRequest, want ...string) {
	t.Helper()
	var methods []string
	for _, call := range calls {
		methods = append(methods, call.method+" "+call.path)
	}
	if !reflect.DeepEqual(methods, want) {
		t.Errorf("OpenBao requests = %v, want %v.", methods, want)
	}
}

func fingerprint(seed [32]byte) string {
	key := ed25519.NewKeyFromSeed(seed[:])
	sum := sha256.Sum256(key.Public().(ed25519.PublicKey))
	return hex.EncodeToString(sum[:])
}

func assertIdentity(t *testing.T, identity vault.LinkIdentity, err error, seed [32]byte, version int) {
	t.Helper()
	if err != nil {
		t.Fatal("LoadLinkIdentity failed to load the expected stored signing key.")
	}
	if !bytes.Equal(identity.SigningKey, ed25519.NewKeyFromSeed(seed[:])) {
		t.Error("LoadLinkIdentity returned a key other than the persisted signing key.")
	}
	if want := fingerprint(seed); identity.Fingerprint != want {
		t.Error("The identity fingerprint differs from the lowercase SHA-256 of the public key.")
	}
	if identity.VaultVersion != version {
		t.Errorf("The identity version is %d, want %d.", identity.VaultVersion, version)
	}
}

func assertVaultFailure(t *testing.T, identity vault.LinkIdentity, err error, otherSecrets ...string) {
	t.Helper()
	if err == nil {
		t.Error("LoadLinkIdentity accepted an OpenBao or pin failure.")
	} else {
		assertRedacted(t, err, otherSecrets...)
	}
	if len(identity.SigningKey) != 0 || identity.Fingerprint != "" || identity.VaultVersion != 0 {
		t.Error("A failed load returned signing identity material.")
	}
}

func assertRedacted(t *testing.T, err error, otherSecrets ...string) {
	t.Helper()
	secrets := append([]string{roleID, secretID, tokenPrefix, sensitiveBody, base64.StdEncoding.EncodeToString(fixtureSeed[:])}, otherSecrets...)
	for _, value := range secrets {
		if value != "" && strings.Contains(err.Error(), value) {
			t.Error("The error exposes a credential or a value from the OpenBao response.")
			return
		}
	}
}

type persistedKey struct {
	mu      sync.Mutex
	seed    [32]byte
	version int
}

func (p *persistedKey) set(seed [32]byte, version int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seed, p.version = seed, version
}

func (p *persistedKey) key(w http.ResponseWriter, r *http.Request, call wireRequest) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if r.Method == http.MethodGet {
		if p.version == 0 {
			replyAbsent(w)
			return
		}
		replySeed(w, base64.StdEncoding.EncodeToString(p.seed[:]), p.version)
		return
	}
	if p.version != 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	p.seed, p.version = call.seed, 1
	replyCreated(w)
}

func replyCreated(w http.ResponseWriter) {
	io.WriteString(w, `{"data":{"version":1}}`)
}

const (
	loginRequest  = "POST " + loginPath
	readRequest   = "GET " + keyPath
	createRequest = "POST " + keyPath
)

func TestLinkSigningKeyPersistence(t *testing.T) {
	t.Run("fingerprint", func(t *testing.T) {
		pub := ed25519.NewKeyFromSeed(fixtureSeed[:]).Public().(ed25519.PublicKey)
		if vault.SigningKeyFingerprint(pub) != fingerprint(fixtureSeed) {
			t.Error("SigningKeyFingerprint must return lowercase hexadecimal SHA-256 of the raw public key.")
		}
	})
	t.Run("create_and_restart", func(t *testing.T) {
		s := newVaultStore(t)
		state := &persistedKey{}
		wire := startWireVault(t, nil, state.key)
		first, err := vault.LoadLinkIdentity(t.Context(), s, wire.open(t), true)
		calls := wire.calls()
		assertMethods(t, calls, loginRequest, readRequest, createRequest, readRequest)
		if len(calls) != 4 || calls[2].method != http.MethodPost {
			t.Fatal("The first load did not attempt one creation followed by a read.")
		}
		seed := calls[2].seed
		assertIdentity(t, first, err, seed, 1)
		second, err := vault.LoadLinkIdentity(t.Context(), s, wire.open(t), true)
		assertIdentity(t, second, err, seed, 1)
		calls = wire.calls()
		assertMethods(t, calls, loginRequest, readRequest, createRequest, readRequest, loginRequest, readRequest)
		if len(calls) == 6 &&
			(calls[1].token != tokenNumber(1) || calls[2].token != tokenNumber(1) ||
				calls[3].token != tokenNumber(1) || calls[5].token != tokenNumber(2)) {
			t.Error("Each load must use the token from its own AppRole login.")
		}
	})
	t.Run("shared_fixture", func(t *testing.T) {
		s := newVaultStore(t)
		address, file := vaulttest.Start(t, fixtureSeed)
		identity, err := vault.LoadLinkIdentity(t.Context(), s, openClient(t, address, file), true)
		assertIdentity(t, identity, err, fixtureSeed, 1)
	})
}

func TestLinkSigningKeyCreateRace(t *testing.T) {
	for _, scenario := range []struct {
		name         string
		conflictCode int
		dropWinner   bool
	}{
		{name: "version_conflict", conflictCode: http.StatusBadRequest},
		{name: "denied_conflict", conflictCode: http.StatusForbidden},
		{name: "lost_conflict_reply"},
		{name: "lost_success_reply", conflictCode: http.StatusBadRequest, dropWinner: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			firstStore, secondStore := newVaultStore(t), newVaultStore(t)
			var mu sync.Mutex
			firstReads := make(map[string]bool)
			bothAbsent := make(chan struct{})
			var winner [32]byte
			created := false
			wire := startWireVault(t, nil, func(w http.ResponseWriter, r *http.Request, call wireRequest) {
				mu.Lock()
				if r.Method == http.MethodGet && !firstReads[call.token] {
					firstReads[call.token] = true
					if len(firstReads) == 2 {
						close(bothAbsent)
					}
					mu.Unlock()
					select {
					case <-bothAbsent:
						replyAbsent(w)
					case <-r.Context().Done():
					}
					return
				}
				if r.Method == http.MethodGet {
					current, exists := winner, created
					mu.Unlock()
					if !exists {
						replyAbsent(w)
						return
					}
					replySeed(w, base64.StdEncoding.EncodeToString(current[:]), 1)
					return
				}
				isWinner := !created
				if isWinner {
					winner, created = call.seed, true
				}
				mu.Unlock()
				switch {
				case !isWinner && scenario.conflictCode == 0, isWinner && scenario.dropWinner:
					dropReply(t, w)
				case !isWinner:
					w.WriteHeader(scenario.conflictCode)
					io.WriteString(w, `{"errors":["fixture compare-and-set conflict"]}`)
				default:
					replyCreated(w)
				}
			})

			type result struct {
				identity vault.LinkIdentity
				err      error
			}
			startLoad := func(s *store.Store, c *vault.Client) *storetest.Task[result] {
				return storetest.Start(t, func() result {
					ctx, cancel := context.WithTimeout(t.Context(), 12*time.Second)
					defer cancel()
					identity, err := vault.LoadLinkIdentity(ctx, s, c, true)
					return result{identity: identity, err: err}
				})
			}
			first := startLoad(firstStore, wire.open(t))
			second := startLoad(secondStore, wire.open(t))
			firstResult, secondResult := first.Wait(t), second.Wait(t)

			calls := wire.calls()
			var writes []wireRequest
			for _, call := range calls {
				if call.path == keyPath && call.method == http.MethodPost {
					writes = append(writes, call)
				}
			}
			if len(writes) != 2 {
				t.Errorf("The competing loads issued %d create requests, want two.", len(writes))
			} else if writes[0].seed == writes[1].seed {
				t.Error("The competing loads submitted the same candidate seed.")
			}
			for _, token := range []string{tokenNumber(1), tokenNumber(2)} {
				var requests []wireRequest
				for _, call := range calls {
					if call.token == token {
						requests = append(requests, call)
					}
				}
				assertMethods(t, requests, readRequest, createRequest, readRequest)
			}
			if len(calls) != 8 {
				t.Errorf("The two loads issued %d total requests, want eight.", len(calls))
			}
			mu.Lock()
			stored := winner
			mu.Unlock()
			assertIdentity(t, firstResult.identity, firstResult.err, stored, 1)
			assertIdentity(t, secondResult.identity, secondResult.err, stored, 1)
		})
	}
	t.Run("denied_and_still_absent", func(t *testing.T) {
		s := newVaultStore(t)
		wire := startWireVault(t, nil, func(w http.ResponseWriter, r *http.Request, _ wireRequest) {
			if r.Method == http.MethodPost {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			replyAbsent(w)
		})
		identity, err := vault.LoadLinkIdentity(t.Context(), s, wire.open(t), true)
		assertVaultFailure(t, identity, err)
		assertMethods(t, wire.calls(), loginRequest, readRequest, createRequest, readRequest)
	})
}

func dropReply(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	conn, _, err := http.NewResponseController(w).Hijack()
	if err != nil {
		t.Error("The fixture could not discard the first write reply.")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_ = conn.Close()
}

func TestOpenBaoLeaseAndFailures(t *testing.T) {
	t.Run("credential_and_address", testOpenValidation)
	t.Run("lease_refresh", testLeaseRefresh)
	t.Run("expired_lease", testExpiredLease)

	t.Run("login_refused", func(t *testing.T) {
		s := newVaultStore(t)
		wire := startWireVault(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, `{"errors":[%q]}`, sensitiveBody)
		}, unusedKey)
		identity, err := vault.LoadLinkIdentity(t.Context(), s, wire.open(t), true)
		assertVaultFailure(t, identity, err)
		assertMethods(t, wire.calls(), loginRequest)
	})

	t.Run("nonpositive_lease", func(t *testing.T) {
		s := newVaultStore(t)
		wire := startWireVault(t, func(w http.ResponseWriter, _ *http.Request, n int) {
			replyLogin(w, n, 0)
		}, unusedKey)
		identity, err := vault.LoadLinkIdentity(t.Context(), s, wire.open(t), true)
		assertVaultFailure(t, identity, err)
		assertMethods(t, wire.calls(), loginRequest)
	})

	t.Run("read_failures", testVaultReadFailures)
	t.Run("redirects", testVaultRedirects)
	t.Run("transport_refusal", func(t *testing.T) {
		s := newVaultStore(t)
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal("The fixture could not allocate a loopback address.")
		}
		address := "http://" + listener.Addr().String()
		if err := listener.Close(); err != nil {
			t.Fatal("The fixture could not close its loopback listener.")
		}
		identity, err := vault.LoadLinkIdentity(t.Context(), s, openClient(t, address, credentialFile(t, credentials, 0o400)), true)
		assertVaultFailure(t, identity, err)
	})
	t.Run("request_deadline", func(t *testing.T) {
		s := newVaultStore(t)
		wire := startWireVault(t, nil, func(_ http.ResponseWriter, r *http.Request, _ wireRequest) {
			<-r.Context().Done()
		})
		ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
		defer cancel()
		start := time.Now()
		identity, err := vault.LoadLinkIdentity(ctx, s, wire.open(t), true)
		elapsed := time.Since(start)
		assertVaultFailure(t, identity, err)
		assertMethods(t, wire.calls(), loginRequest, readRequest)
		if elapsed > 7*time.Second {
			t.Errorf("The vault read took %v and exceeded the five-second request budget plus two seconds of tolerance.", elapsed)
		}
	})
}

func unusedKey(w http.ResponseWriter, _ *http.Request, _ wireRequest) {
	w.WriteHeader(http.StatusInternalServerError)
}

func testOpenValidation(t *testing.T) {
	const loopback = "http://127.0.0.1:1"
	for _, scenario := range []struct {
		name string
		body string
		mode os.FileMode
	}{
		{name: "duplicate", body: `{"role_id":"fixture-role-id","role_id":"fixture-role-id","secret_id":"fixture-secret-id"}`},
		{name: "other_field", body: `{"role_id":"fixture-role-id","secret_id":"fixture-secret-id","other":1}`},
		{name: "wrong_field_case", body: `{"ROLE_ID":"fixture-role-id","secret_id":"fixture-secret-id"}`},
		{name: "empty", body: `{"role_id":"fixture-role-id","secret_id":""}`},
		{name: "missing", body: `{"secret_id":"fixture-secret-id"}`},
		{name: "wrong_type", body: `{"role_id":42,"secret_id":"fixture-secret-id"}`},
		{name: "malformed", body: `{"role_id":"fixture-role-id","secret_id":"fixture-secret-id"`},
		{name: "other_value", body: credentials + "{}"},
		{name: "other_users_can_read", body: credentials, mode: 0o644},
		{name: "group_can_read", body: credentials, mode: 0o440},
		{name: "owner_can_execute", body: credentials, mode: 0o700},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			mode := scenario.mode
			if mode == 0 {
				mode = 0o400
			}
			file := credentialFile(t, scenario.body, mode)
			if err := os.Chmod(file, mode); err != nil {
				t.Fatal("The fixture could not set the credential file mode.")
			}
			assertOpenFailure(t, loopback, file, false)
		})
	}
	t.Run("directory", func(t *testing.T) {
		assertOpenFailure(t, loopback, t.TempDir(), false)
	})
	for _, scenario := range []struct {
		name, addr string
		optIn      bool
	}{
		{name: "empty_address"},
		{name: "wrong_scheme", addr: "ftp://127.0.0.1:8200"},
		{name: "nonliteral_loopback", addr: "http://localhost:8200"},
		{name: "stack_without_opt_in", addr: "http://openbao:8200"},
		{name: "stack_wrong_port", addr: "http://openbao:8201", optIn: true},
		{name: "other_stack_host", addr: "http://not-openbao:8200", optIn: true},
		{name: "private_address", addr: "http://10.2.3.4:8200", optIn: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			assertOpenFailure(t, scenario.addr, credentialFile(t, credentials, 0o400), scenario.optIn)
		})
	}

	for _, mode := range []os.FileMode{0o400, 0o600} {
		file := credentialFile(t, credentials, mode)
		for _, address := range []string{loopback, "http://[::1]:1", "http://openbao:8200", "https://vault.example"} {
			if c, err := vault.Open(address, file, true); err != nil || c == nil {
				t.Errorf("Open rejected approved address %s with mode %o.", address, mode)
			}
		}
	}
}

func assertOpenFailure(t *testing.T, addr, file string, optIn bool) {
	t.Helper()
	client, err := vault.Open(addr, file, optIn)
	if err == nil || client != nil {
		t.Error("Open accepted an invalid credential or address.")
	}
	if err != nil {
		assertRedacted(t, err)
	}
}

func testLeaseRefresh(t *testing.T) {
	for _, scenario := range []struct {
		name  string
		lease int
		delay time.Duration
	}{
		{name: "minute_left", lease: 60},
		{name: "lease_includes_login_time", lease: 62, delay: 2250 * time.Millisecond},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			s := newVaultStore(t)
			state := &persistedKey{seed: fixtureSeed, version: 7}
			wire := startWireVault(t, func(w http.ResponseWriter, r *http.Request, n int) {
				lease := 900
				if n == 1 {
					lease = scenario.lease
					if scenario.delay != 0 {
						timer := time.NewTimer(scenario.delay)
						defer timer.Stop()
						select {
						case <-timer.C:
						case <-r.Context().Done():
							return
						}
					}
				}
				replyLogin(w, n, lease)
			}, state.key)
			client := wire.open(t)
			identity, err := vault.LoadLinkIdentity(t.Context(), s, client, true)
			assertIdentity(t, identity, err, fixtureSeed, 7)

			// Concurrent established-pin reads must share the refreshed login.
			begin := make(chan struct{})
			load := func() bool {
				<-begin
				got, err := vault.LoadLinkIdentity(t.Context(), s, client, true)
				return err == nil && got.Fingerprint == fingerprint(fixtureSeed) && got.VaultVersion == 7
			}
			first := storetest.Start(t, load)
			second := storetest.Start(t, load)
			close(begin)
			if !first.Wait(t) || !second.Wait(t) {
				t.Error("Both loads must succeed after refreshing the AppRole token.")
			}
			calls := wire.calls()
			assertMethods(t, calls, loginRequest, readRequest, loginRequest, readRequest, readRequest)
			if len(calls) == 5 && (calls[1].token != tokenNumber(1) ||
				calls[3].token != tokenNumber(2) || calls[4].token != tokenNumber(2)) {
				t.Error("Both reads inside the refresh window must use the fresh token.")
			}
		})
	}
}

func testExpiredLease(t *testing.T) {
	s := newVaultStore(t)
	state := &persistedKey{seed: fixtureSeed, version: 1}
	wire := startWireVault(t, func(w http.ResponseWriter, _ *http.Request, n int) {
		if n == 1 {
			replyLogin(w, n, 2)
			return
		}
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, `{"errors":[%q]}`, sensitiveBody)
	}, state.key)
	client := wire.open(t)
	identity, err := vault.LoadLinkIdentity(t.Context(), s, client, true)
	assertIdentity(t, identity, err, fixtureSeed, 1)
	time.Sleep(2100 * time.Millisecond)
	identity, err = vault.LoadLinkIdentity(t.Context(), s, client, true)
	assertVaultFailure(t, identity, err)
	assertMethods(t, wire.calls(), loginRequest, readRequest, loginRequest)
}

func testVaultReadFailures(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString(fixtureSeed[:])
	document := func(seed string, version int) string {
		return fmt.Sprintf(`{"data":{"data":{"private_seed_b64":%q},"metadata":{"version":%d}}}`, seed, version)
	}
	for _, scenario := range []struct {
		name   string
		status int
		body   string
	}{
		{name: "refused", status: http.StatusForbidden, body: fmt.Sprintf(`{"errors":[%q]}`, sensitiveBody)},
		{name: "sealed", status: http.StatusServiceUnavailable, body: `{"errors":["OpenBao is sealed", "` + sensitiveBody + `"]}`},
		{name: "deleted", status: http.StatusNotFound, body: `{"errors":[],"data":{"data":null,"metadata":{"version":7,"deletion_time":"2026-11-26T12:00:00Z","destroyed":false}}}`},
		{name: "destroyed", status: http.StatusNotFound, body: `{"errors":[],"data":{"data":null,"metadata":{"version":7,"deletion_time":"","destroyed":true}}}`},
		{name: "malformed_json", status: http.StatusOK, body: `{"data":"` + sensitiveBody},
		{name: "missing_seed", status: http.StatusOK, body: `{"data":{"data":{},"metadata":{"version":1}}}`},
		{name: "short_seed", status: http.StatusOK, body: document(base64.StdEncoding.EncodeToString(fixtureSeed[:31]), 1)},
		{name: "noncanonical_seed", status: http.StatusOK, body: document(encoded[:8]+"\n"+encoded[8:], 1)},
		{name: "nonstandard_seed", status: http.StatusOK, body: document(base64.URLEncoding.EncodeToString(fixtureSeed[:]), 1)},
		{name: "zero_version", status: http.StatusOK, body: document(encoded, 0)},
		{name: "oversized", status: http.StatusOK, body: `{"padding":"` + strings.Repeat("x", 64<<10) + `",` + document(encoded, 1)[1:]},
		{name: "oversized_after_object", status: http.StatusOK, body: document(encoded, 1) + strings.Repeat(" ", 64<<10)},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			s := newVaultStore(t)
			wire := startWireVault(t, nil, func(w http.ResponseWriter, _ *http.Request, _ wireRequest) {
				w.WriteHeader(scenario.status)
				io.WriteString(w, scenario.body)
			})
			identity, err := vault.LoadLinkIdentity(t.Context(), s, wire.open(t), true)
			assertVaultFailure(t, identity, err)
			assertMethods(t, wire.calls(), loginRequest, readRequest)
		})
	}
}

func testVaultRedirects(t *testing.T) {
	for _, stage := range []string{"login", "read"} {
		t.Run(stage, func(t *testing.T) {
			s := newVaultStore(t)
			destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				t.Error("OpenBao requests must never follow a redirect.")
				w.WriteHeader(http.StatusForbidden)
			}))
			t.Cleanup(destination.Close)
			redirect := func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, destination.URL+r.URL.Path, http.StatusTemporaryRedirect)
			}
			wire := startWireVault(t, func(w http.ResponseWriter, r *http.Request, n int) {
				if stage == "login" {
					redirect(w, r)
					return
				}
				replyLogin(w, n, 900)
			}, func(w http.ResponseWriter, r *http.Request, _ wireRequest) {
				redirect(w, r)
			})
			identity, err := vault.LoadLinkIdentity(t.Context(), s, wire.open(t), true)
			assertVaultFailure(t, identity, err)
			want := []string{loginRequest}
			if stage == "read" {
				want = append(want, readRequest)
			}
			assertMethods(t, wire.calls(), want...)
		})
	}
}
