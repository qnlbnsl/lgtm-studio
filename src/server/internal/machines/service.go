package machines

import (
	"context"

	lgtmv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// Service handles the Machines page's calls.
type Service struct {
	lgtmv1connect.UnimplementedClientServiceHandler
	store         *store.Store
	sessions      *auth.Sessions
	registry      *Registry
	tokens        *Tokens
	follower      *store.Follower
	link          Link
	watchShutdown context.Context
}

// NewService creates a Machines service for one server.
func NewService(s *store.Store, sessions *auth.Sessions, r *Registry, t *Tokens) *Service {
	return &Service{store: s, sessions: sessions, registry: r, tokens: t}
}

// WithFollower returns a service that uses the caller's shared feed follower.
func (s *Service) WithFollower(follower *store.Follower) *Service {
	service := *s
	service.follower = follower
	return &service
}

// WithWatchShutdown returns a service that ends its watches when shutdown is canceled.
func (s *Service) WithWatchShutdown(shutdown context.Context) *Service {
	service := *s
	service.watchShutdown = shutdown
	return &service
}

// WithLink returns a service that uses the link for projection and removal.
func (s *Service) WithLink(link Link) *Service {
	service := *s
	service.link = link
	return &service
}

func (s *Service) loadPresence(ctx context.Context, q Querier, ids []string) (map[string]Presence, error) {
	if s.link == nil || len(ids) == 0 {
		return nil, nil
	}
	return s.link.Presence(ctx, q, ids)
}

func (s *Service) projectOne(ctx context.Context, q Querier, machine Machine) (*lgtmv1.Client, error) {
	rows, err := s.loadPresence(ctx, q, []string{machine.ID})
	if err != nil {
		return nil, err
	}
	presence := rows[machine.ID]
	return ProjectWithPresence(machine, &presence), nil
}

func (s *Service) WithCommands(CommandQueue) *Service { return s } // STUB s3-commands-tests-b
