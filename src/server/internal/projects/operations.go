package projects

import (
	"context"

	"connectrpc.com/connect"

	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
)

// InspectFolder is the test writer's stub for starting or replaying an inspection.
func (*Service) InspectFolder(context.Context, *connect.Request[webv1.InspectFolderRequest]) (*connect.Response[webv1.InspectFolderResponse], error) {
	return nil, nil
}

// AddRepository is the test writer's stub for starting or replaying an attachment.
func (*Service) AddRepository(context.Context, *connect.Request[webv1.AddRepositoryRequest]) (*connect.Response[webv1.AddRepositoryResponse], error) {
	return nil, nil
}
