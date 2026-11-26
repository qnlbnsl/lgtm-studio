// Package link answers the calls that a machine makes on the server.
package link

import (
	"crypto/ed25519"

	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// Service is a stub of the test writer for item 4 (task s3-machines-tests-b).
// It answers every call as unimplemented. The builder replaces it and adds Join.
type Service struct {
	linkv1connect.UnimplementedLinkServiceHandler
	store     *store.Store
	tokens    *machines.Tokens
	serverKey ed25519.PublicKey
}

// NewService keeps its arguments. This is the signature that the plan gives.
func NewService(s *store.Store, t *machines.Tokens, serverKey ed25519.PublicKey) *Service {
	return &Service{store: s, tokens: t, serverKey: serverKey}
}
