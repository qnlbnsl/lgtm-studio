package folders

import (
	"context"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

// This type is a stub of the test writer.
type checkoutPaths struct {
	root, gitDir, commonDir, indexPath string
}

// This type is a stub of the test writer.
type statusSnapshot struct {
	root, gitDir string
	cleanup      func()
}

// This function is a stub of the test writer.
func readCheckout(ctx context.Context, path, home string) (checkoutPaths, *linkv1.FolderFacts, linkv1.FolderRefusal) {
	return checkoutPaths{}, nil, linkv1.FolderRefusal_FOLDER_REFUSAL_UNSPECIFIED
}

// This function is a stub of the test writer.
func captureStatus(ctx context.Context, checkout checkoutPaths) (statusSnapshot, error) {
	return statusSnapshot{}, nil
}
