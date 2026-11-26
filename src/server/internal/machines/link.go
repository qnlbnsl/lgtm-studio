package machines

import (
	"context"
	"time"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// Presence and Link are declarations of the test writer for item 3.
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

// The following registry operations are stubs of the test writer. Move them to store.go when building item 3.
func (*Registry) Lock(context.Context, *store.Tx, string) (Machine, error) {
	return Machine{}, nil
}
func (*Registry) TryLock(context.Context, *store.Tx, string) (Machine, error) {
	return Machine{}, nil
}
func (*Registry) ApplyFacts(context.Context, *store.Tx, string, *linkv1.Facts) error {
	return nil
}
func (*Registry) ApplyHarnesses(context.Context, *store.Tx, string, *linkv1.HarnessesFound) error {
	return nil
}
func (*Registry) NoteSeen(context.Context, *store.Tx, string, time.Time, string) error {
	return nil
}
func (*Registry) Touch(context.Context, *store.Tx, string) error { return nil }
func Standing(Machine) *linkv1.MachineState                      { return nil }
