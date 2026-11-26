package machines

import (
	"context"
	"time"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// Presence describes the saved connection and the process that holds it.
type Presence struct {
	StreamID                         string
	Generation                       int64
	ConnectedAt, LastHeardAt         time.Time
	Address                          string
	CPUPercent, MemoryPercent        *uint32
	FactsReceived, HarnessesReceived bool
	Held                             bool
}

type Link interface {
	Presence(context.Context, Querier, []string) (map[string]Presence, error)
	Forget(context.Context, *store.Tx, string) error
}
