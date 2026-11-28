package projects

import (
	"context"

	"connectrpc.com/connect"

	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
)

// WatchRepositories is a stub written by the test writer.
func (s *Service) WatchRepositories(ctx context.Context, req *connect.Request[webv1.WatchRepositoriesRequest], stream *connect.ServerStream[webv1.WatchRepositoriesResponse]) error {
	return s.UnimplementedProjectServiceHandler.WatchRepositories(ctx, req, stream)
}
