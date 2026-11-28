package projects

import (
	"context"
	"time"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// CommandLink provides command admission and connection presence for Projects.
type CommandLink interface {
	EnqueueCommand(context.Context, *store.Tx, string, *linkv1.Command, time.Duration) (string, error)
	Presence(context.Context, machines.Querier, []string) (map[string]machines.Presence, error)
}

// Service handles calls about repositories and their folders.
type Service struct {
	lgtmv1connect.UnimplementedProjectServiceHandler
	store         *store.Store
	sessions      *auth.Sessions
	registry      *machines.Registry
	commands      CommandLink
	follower      *store.Follower
	watchShutdown context.Context
}

// NewService creates a Projects service for one server.
func NewService(db *store.Store, sessions *auth.Sessions, registry *machines.Registry, commands CommandLink, follower *store.Follower, shutdown context.Context) *Service {
	return &Service{
		store: db, sessions: sessions, registry: registry, commands: commands,
		follower: follower, watchShutdown: shutdown,
	}
}
