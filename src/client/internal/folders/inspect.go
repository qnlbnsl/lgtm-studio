package folders

import (
	"context"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

// Inspect is a stub of the test writer.
func Inspect(ctx context.Context, path, home string) (*linkv1.FolderFacts, linkv1.FolderRefusal) {
	return nil, linkv1.FolderRefusal_FOLDER_REFUSAL_UNSPECIFIED
}
