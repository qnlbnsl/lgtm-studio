package folders

import (
	"context"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

// readResolvedCheckout is a stub of the test writer for the repair of item 4 (post-4 4).
// The builder replaces it with the part of discovery that follows the resolution of the
// typed path: it validates the resolved paths and then reads the checkout's metadata.
// readCheckout must call it, and the builder deletes this file.
func readResolvedCheckout(ctx context.Context, paths checkoutPaths, physical string) (checkoutPaths, *linkv1.FolderFacts, linkv1.FolderRefusal) {
	return checkoutPaths{}, nil, linkv1.FolderRefusal_FOLDER_REFUSAL_UNSPECIFIED
}
