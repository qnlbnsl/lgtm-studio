package link

import (
	"context"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// AcceptResult is a stub of the test writer.
func (*State) AcceptResult(ctx context.Context, machineID, streamID string, result *linkv1.CommandResult) error {
	return nil
}

// lockCommandStream is a stub of the test writer.
func (*State) lockCommandStream(ctx context.Context, tx *store.Tx, machineID, streamID string) (machines.Machine, error) {
	return machines.Machine{}, nil
}
