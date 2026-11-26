package vault_test

import (
	"encoding/base64"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/vault"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/vault/vaulttest"
)

func TestLinkIdentityReportsKeyAdoption(t *testing.T) {
	t.Run("adopted before the first pin", func(t *testing.T) {
		s := newVaultStore(t)
		address, file := vaulttest.Start(t, fixtureSeed)
		adopted, err := vault.LoadLinkIdentity(t.Context(), s, openClient(t, address, file), true)
		assertIdentity(t, adopted, err, fixtureSeed, 1)
		if !adopted.Adopted() {
			t.Error("The first pin did not report adopting the key already in OpenBao.")
		}
		loaded, err := vault.LoadLinkIdentity(t.Context(), s, openClient(t, address, file), true)
		assertIdentity(t, loaded, err, fixtureSeed, 1)
		if loaded.Adopted() {
			t.Error("Loading the same key under its established pin must not report adoption again.")
		}
	})
	t.Run("created then pinned", func(t *testing.T) {
		s := newVaultStore(t)
		state := &persistedKey{}
		wire := startWireVault(t, nil, state.key)
		created, err := vault.LoadLinkIdentity(t.Context(), s, wire.open(t), true)
		if err != nil {
			t.Fatal("Loading the first identity failed.")
		}
		if created.Adopted() {
			t.Error("A load that installed its own key incorrectly reported an adoption.")
		}
		assertMethods(t, wire.calls(), loginRequest, readRequest, createRequest, readRequest)
	})
	t.Run("adopted a competing create", func(t *testing.T) {
		s := newVaultStore(t)
		var reads atomic.Int32
		wire := startWireVault(t, nil, func(w http.ResponseWriter, r *http.Request, _ wireRequest) {
			if r.Method == http.MethodPost {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if reads.Add(1) == 1 {
				replyAbsent(w)
				return
			}
			replySeed(w, base64.StdEncoding.EncodeToString(fixtureSeed[:]), 1)
		})
		adopted, err := vault.LoadLinkIdentity(t.Context(), s, wire.open(t), true)
		assertIdentity(t, adopted, err, fixtureSeed, 1)
		if !adopted.Adopted() {
			t.Error("The load did not report adopting the competing key after losing its create attempt.")
		}
		assertMethods(t, wire.calls(), loginRequest, readRequest, createRequest, readRequest)
	})
}
