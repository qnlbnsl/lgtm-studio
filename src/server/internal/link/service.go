// Package link answers the calls that a machine makes on the server.
package link

import (
	"crypto/ed25519"
	"slices"

	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// Service handles machine joins.
type Service struct {
	linkv1connect.UnimplementedLinkServiceHandler
	store     *store.Store
	tokens    *machines.Tokens
	serverKey ed25519.PublicKey
}

// NewService creates the link service for one server.
func NewService(s *store.Store, t *machines.Tokens, serverKey ed25519.PublicKey) *Service {
	return &Service{store: s, tokens: t, serverKey: slices.Clone(serverKey)}
}
