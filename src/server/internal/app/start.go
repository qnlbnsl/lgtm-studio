package app

import (
	"context"
	"crypto/ed25519"
	"errors"
	"log/slog"
	"net"
	"time"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/config"
)

// These are the test writer's stubs for application startup and lifecycle.
type Server struct{}

type phaseBudgets struct {
	admission, migration, prerequisites, probe time.Duration
}

func Start(ctx context.Context, cfg config.Config, log *slog.Logger) (*Server, error) {
	return start(ctx, cfg, log, phaseBudgets{})
}

func start(context.Context, config.Config, *slog.Logger, phaseBudgets) (*Server, error) {
	return nil, errors.New("The application startup stub has not been implemented.")
}

func (*Server) Addr() net.Addr                      { return nil }
func (*Server) SigningPublicKey() ed25519.PublicKey { return nil }
func (*Server) Wait() error                         { return nil }
func (*Server) Close() error                        { return nil }
