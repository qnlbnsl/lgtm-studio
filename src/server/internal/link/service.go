// Package link answers the calls that a machine makes on the server.
package link

import (
	"context"
	"crypto/ed25519"
	"slices"
	"sync/atomic"
	"time"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// Service handles machine joins and streams.
type Service struct {
	store        *store.Store
	tokens       *machines.Tokens
	serverKey    ed25519.PublicKey
	streaming    *streamingService
	commandSlots commandSlots
}

type streamingService struct {
	life     context.Context
	registry *machines.Registry
	state    *State
	hub      *Hub
	key      ed25519.PrivateKey
	options  Options

	running        atomic.Bool
	cleanup        disconnectQueue
	cleanupRetry   time.Duration
	cleanupTimeout time.Duration
	feedRetry      time.Duration
	feedTimeout    time.Duration
}

// NewService creates the link service for one server.
func NewService(s *store.Store, t *machines.Tokens, serverKey ed25519.PublicKey) *Service {
	return &Service{store: s, tokens: t, serverKey: slices.Clone(serverKey)}
}
