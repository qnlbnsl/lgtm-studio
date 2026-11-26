package link

import (
	"context"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// These declarations are stubs of the test writer.
var (
	ErrEventNumberZero = store.NewDomainError("The event number is outside the allowed range.")
	ErrEventGap        = store.NewDomainError("The event leaves a gap after the server's mark.")
)

type EventResult struct {
	Mark    uint64
	Applied bool
}

func (*State) ApplyEvent(context.Context, string, string, *linkv1.Event) (EventResult, error) {
	return EventResult{}, nil
}
