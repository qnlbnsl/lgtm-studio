// Package vaulttest supplies a loopback OpenBao fixture with a persisted key.
package vaulttest

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// Start serves a single AppRole and the same signing seed for every authorized read.
func Start(t testing.TB, seed [32]byte) (address string, credentialFile string) {
	t.Helper()
	const role = "fixture-role"
	const secret = "fixture-secret"
	const token = "fixture-batch-token"

	credentialFile = filepath.Join(t.TempDir(), "openbao.json")
	if err := os.WriteFile(credentialFile, []byte(`{"role_id":"`+role+`","secret_id":"`+secret+`"}`+"\n"), 0o400); err != nil {
		t.Fatal("The OpenBao fixture credential could not be written.")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/auth/approle/login":
			var credentials map[string]string
			body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
			if err != nil || json.Unmarshal(body, &credentials) != nil ||
				len(credentials) != 2 || credentials["role_id"] != role || credentials["secret_id"] != secret {
				http.Error(w, "Invalid fixture credentials.", http.StatusForbidden)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"auth": map[string]any{
				"client_token": token, "lease_duration": 900, "renewable": false,
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/lgtm/data/server/link/signing-key":
			if r.Header.Get("X-Vault-Token") != token {
				http.Error(w, "Invalid fixture token.", http.StatusForbidden)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"data":     map[string]any{"private_seed_b64": base64.StdEncoding.EncodeToString(seed[:])},
				"metadata": map[string]any{"version": 1},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server.URL, credentialFile
}
