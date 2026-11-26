package link

import (
	"context"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// These declarations are stubs of the test writer.
type CommandHooks interface {
	SendNext(context.Context, string, string, func(*linkv1.Command) error) (bool, error)
	AcceptResult(context.Context, string, string, *linkv1.CommandResult) error
}

func (*Service) streamContext(ctx context.Context) (context.Context, context.CancelCauseFunc) {
	return context.WithCancelCause(ctx)
}
func (*Service) queueDisconnect(string, string) {}
func peerAddress(string, bool) string           { return "" }

// These optional controls are stubs of the test writer. They observe the two
// sides of subscription setup and let the test hold delivery after Start.
// subscribed runs before the head read. started runs after Start and the
// recovery wake, before consuming the first feed row.
type runnerControlsKey struct{}

type runnerControls struct {
	subscribed func(*store.Subscription)
	started    func(*store.Subscription)
}
