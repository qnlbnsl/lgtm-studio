package machines

import (
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// Service handles the Machines page's calls.
type Service struct {
	lgtmv1connect.UnimplementedClientServiceHandler
	store    *store.Store
	sessions *auth.Sessions
	registry *Registry
	tokens   *Tokens
	follower *store.Follower
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
