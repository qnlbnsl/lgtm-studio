package link

import (
	"context"
	"errors"
	"slices"

	"connectrpc.com/connect"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
)

const joinRefusalHeader = "Lgtm-Join-Refusal"

var (
	errInvalidLinkMessage  = errors.New("The link message broke a rule of the contract.")
	errDatabaseUnavailable = errors.New("The server cannot reach its database, try again.")
)

// Join exchanges a join token for the server's public key and a machine identifier.
func (s *Service) Join(ctx context.Context, req *connect.Request[linkv1.JoinRequest]) (*connect.Response[linkv1.JoinResponse], error) {
	if !validJoinRequest(req.Msg) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errInvalidLinkMessage)
	}
	joined, err := s.tokens.Redeem(ctx, machines.Redemption{
		TokenID:   req.Msg.GetTokenId(),
		Secret:    req.Msg.GetTokenSecret(),
		PublicKey: req.Msg.GetMachinePublicKey(),
		HostName:  req.Msg.GetHostName(),
	})
	if err == machines.ErrInvalidHostName {
		return nil, connect.NewError(connect.CodeInvalidArgument, errInvalidLinkMessage)
	}
	if reason, ok := machines.RefusalOf(err); ok {
		refused := connect.NewError(connect.CodePermissionDenied, err)
		refused.Meta().Set(joinRefusalHeader, string(reason))
		return nil, refused
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, errDatabaseUnavailable)
	}
	res := connect.NewResponse(&linkv1.JoinResponse{
		MachineId:       joined.MachineID,
		ServerPublicKey: slices.Clone(s.serverKey),
	})
	res.Header().Set("Cache-Control", "no-store")
	return res, nil
}
