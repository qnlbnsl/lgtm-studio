package folders

import (
	"context"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
)

// Inspect reports local checkout facts after establishing the working tree's status.
func Inspect(ctx context.Context, path, home string) (*linkv1.FolderFacts, linkv1.FolderRefusal) {
	checkout, facts, refusal := readCheckout(ctx, path, home)
	if refusal != folderOK {
		return nil, refusal
	}
	clean, err := workingTreeClean(ctx, checkout)
	if err != nil {
		return nil, folderNotReadable
	}
	facts.Clean = clean
	return facts, folderOK
}
