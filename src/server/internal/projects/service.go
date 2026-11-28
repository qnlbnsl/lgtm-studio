package projects

import (
	"context"
	"time"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// CommandLink is the test writer's stub for the Projects Link boundary.
type CommandLink interface {
	EnqueueCommand(context.Context, *store.Tx, string, *linkv1.Command, time.Duration) (string, error)
	Presence(context.Context, machines.Querier, []string) (map[string]machines.Presence, error)
}

// Service is the test writer's stub for Projects.
type Service struct{}

// NewService is the test writer's stub for the Projects constructor.
func NewService(*store.Store, *auth.Sessions, *machines.Registry, CommandLink, *store.Follower, context.Context) *Service {
	return &Service{}
}
