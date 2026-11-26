package vault_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/vault"
)

func TestOpenRefusesCredentialSymlink(t *testing.T) {
	const address = "https://example.test"
	file := credentialFile(t, credentials, 0o600)
	if client, err := vault.Open(address, file, false); err != nil || client == nil {
		t.Fatalf("Open rejected the direct path to the credential fixture. %v", err)
	}
	link := filepath.Join(t.TempDir(), "credential-link.json")
	if err := os.Symlink(file, link); err != nil {
		t.Fatalf("The fixture could not create a symbolic link. %v", err)
	}
	if client, err := vault.Open(address, link, false); client != nil || !errors.Is(err, vault.ErrCredential) {
		t.Errorf("Open returned %v for a symbolic link, want ErrCredential and no client.", err)
	}
}

func TestOpenRefusesCredentialOwnedByAnotherUser(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Changing the fixture file to another owner requires the superuser.")
	}
	const address = "https://example.test"
	file := credentialFile(t, credentials, 0o600)
	if client, err := vault.Open(address, file, false); err != nil || client == nil {
		t.Fatalf("Open rejected the credential fixture before its owner changed. %v", err)
	}
	if err := os.Chown(file, 1, -1); err != nil {
		t.Fatalf("The fixture could not change the file owner. %v", err)
	}
	if client, err := vault.Open(address, file, false); client != nil || !errors.Is(err, vault.ErrCredential) {
		t.Errorf("Open returned %v for a credential owned by another user, want ErrCredential and no client.", err)
	}
}
