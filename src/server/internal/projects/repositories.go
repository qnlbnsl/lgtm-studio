package projects

import (
	"context"

	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
)

const repositoryRefreshKey = "refresh"

// loadRepository is a stub written by the test writer.
func loadRepository(context.Context, machines.Querier, string) (*webv1.Repository, bool, error) {
	return nil, false, nil
}
