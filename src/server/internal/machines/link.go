package machines

import (
	"context"
	"time"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// Presence describes the saved connection, its liveness and the event history warning.
type Presence struct {
	StreamID                         string
	Generation                       int64
	ConnectedAt, LastHeardAt         time.Time
	Address                          string
	CPUPercent, MemoryPercent        *uint32
	FactsReceived, HarnessesReceived bool
	Held                             bool
	EventHistoryNeedsRecovery        bool
}

type Link interface {
	Presence(context.Context, Querier, []string) (map[string]Presence, error)
	Forget(context.Context, *store.Tx, string) error
}

// CommandQueue saves a command in the caller's machine write.
type CommandQueue interface {
	Enqueue(context.Context, *store.Tx, string, *linkv1.Command, time.Duration) error
}
