package projects

import (
	"context"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// ApplyFolderSeen is a stub of the test writer.
func (s *Service) ApplyFolderSeen(context.Context, *store.Tx, string, *linkv1.FolderSeen) error {
	return nil
}

// SettleCommand is a stub of the test writer.
func (s *Service) SettleCommand(context.Context, *store.Tx, string, string, string) error {
	return nil
}

// attachRepository is a stub of the test writer for the item 5 seam.
func (s *Service) attachRepository(context.Context, *store.Tx, string, *string, *webv1.FolderFacts) (*webv1.AddRepositoryResponse, error) {
	return nil, store.NewDomainError("The repository could not be attached.")
}
