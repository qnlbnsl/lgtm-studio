package vault_test

import (
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/vault"
)

func TestLinkIdentityRefusesConflictingPinWrite(t *testing.T) {
	s, observer := pinDatabase(t)
	winner := fingerprint([32]byte{1})
	state := &persistedKey{seed: fixtureSeed, version: 1}
	pinCommitted := make(chan struct{})
	wire := startWireVault(t, nil, func(w http.ResponseWriter, r *http.Request, call wireRequest) {
		// Commit the competing pin after the load reads PostgreSQL and before it writes the pin.
		if _, err := observer.Exec(r.Context(), `
INSERT INTO server_link_identity (singleton, fingerprint) VALUES (true, $1)`, winner); err != nil {
			t.Errorf("The fixture could not commit the competing pin. %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		close(pinCommitted)
		state.key(w, r, call)
	})

	identity, err := vault.LoadLinkIdentity(t.Context(), s, wire.open(t), true)
	if !errors.Is(err, vault.ErrReplaced) {
		t.Errorf("LoadLinkIdentity returned %v after a conflicting pin won, want ErrReplaced.", err)
	}
	assertVaultFailure(t, identity, err)
	assertMethods(t, wire.calls(), loginRequest, readRequest)
	select {
	case <-pinCommitted:
	default:
		t.Fatal("The fixture did not commit the competing pin.")
	}
	pins := storetest.Rows[string](t, observer, "SELECT fingerprint FROM server_link_identity")
	if !slices.Equal(pins, []string{winner}) {
		t.Error("PostgreSQL must hold exactly the competing writer's pin.")
	}
	if count := storetest.Scalar[int](t, observer, "SELECT count(*) FROM feed"); count != 0 {
		t.Errorf("The refused load left %d feed rows, want zero.", count)
	}
}
