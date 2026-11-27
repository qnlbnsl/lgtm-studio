package link

import (
	"context"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

// inspectFolder is a test writer stub for the worker's call to folders.Inspect.
var inspectFolder = func(context.Context, string, string) (*linkv1.FolderFacts, linkv1.FolderRefusal) {
	return nil, linkv1.FolderRefusal_FOLDER_REFUSAL_UNSPECIFIED
}
