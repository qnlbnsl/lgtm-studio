package link

import (
	"context"
	"time"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// These are stubs of the test writer for the command queue declarations.
const (
	outboxTopic      = "machine_outbox"
	CommandGrace     = 24 * time.Hour
	CommandRetention = 7 * 24 * time.Hour
)

// Enqueue is a stub of the test writer.
func (*State) Enqueue(ctx context.Context, tx *store.Tx, machineID string, action *linkv1.Command, ttl time.Duration) error {
	return nil
}
