package machines

import (
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// Service is a stub of the test writer.
type Service struct {
	lgtmv1connect.UnimplementedClientServiceHandler
}

// NewService is a stub of the test writer.
func NewService(_ *store.Store, _ *auth.Sessions, _ *Registry, _ *Tokens) *Service {
	return &Service{}
}
