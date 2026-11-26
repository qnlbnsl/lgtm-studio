package servertest

import (
	"testing"

	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
)

// Clients is a stub of the test writer.
func (s *Server) Clients(t testing.TB) lgtmv1connect.ClientServiceClient { return nil }

// Link is a stub of the test writer.
func (s *Server) Link() linkv1connect.LinkServiceClient { return nil }
