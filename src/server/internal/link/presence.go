package link

import (
	"context"
	"time"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// These declarations are stubs of the test writer.
var (
	ErrUnknownMachine = store.NewDomainError("The server does not know this machine. Join it again.")
	ErrNotCurrent     = store.NewDomainError("The stream is no longer current.")
	ErrInvalidMessage = store.NewDomainError("The link message broke a rule of the contract.")
)

type State struct {
	afterLock func(context.Context, string)
}

type Stream struct {
	ID              string
	Generation      int64
	Standing        machines.Machine
	Mark            uint64
	ServerTime      time.Time
	ConnectionToken string
}

func NewState(*store.Store, *machines.Registry, *Hub) *State { return &State{} }
func (*State) BecomeCurrent(context.Context, string, string, string) (Stream, error) {
	return Stream{}, nil
}
func (*State) CheckCurrent(context.Context, string, string) (machines.Approval, error) {
	return "", nil
}
func (*State) Heartbeat(context.Context, string, string, *linkv1.Heartbeat) error { return nil }
func (*State) Disconnect(context.Context, string, string) (bool, error)           { return false, nil }
func (*State) ClearConnections(context.Context) error                             { return nil }
func (*State) Presence(context.Context, machines.Querier, []string) (map[string]machines.Presence, error) {
	return nil, nil
}
func (*State) Forget(context.Context, *store.Tx, string) error { return nil }
