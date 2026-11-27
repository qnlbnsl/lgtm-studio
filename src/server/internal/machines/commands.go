package machines

import (
	"context"
	"time"

	"connectrpc.com/connect"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	lgtmv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

var (
	errNotOnline    = store.NewDomainError("This machine is not online.")
	errNotConnected = store.NewDomainError("This machine is not connected.")
)

// ScanClientHarnesses queues a harness scan for an online machine.
func (s *Service) ScanClientHarnesses(ctx context.Context, req *connect.Request[lgtmv1.ScanClientHarnessesRequest]) (*connect.Response[lgtmv1.ScanClientHarnessesResponse], error) {
	id := req.Msg.GetId()
	write := recordRequest(req.Header(), lgtmv1connect.ClientServiceScanClientHarnessesProcedure, id)
	command := &linkv1.Command{Action: &linkv1.Command_ScanHarnesses{ScanHarnesses: &linkv1.ScanHarnesses{}}}
	if err := s.queueConnectedAction(ctx, write, id, command, errNotOnline); err != nil {
		return nil, err
	}
	return connect.NewResponse(&lgtmv1.ScanClientHarnessesResponse{}), nil
}

// ReconnectClient queues a restart for an approved machine with a live connection.
func (s *Service) ReconnectClient(ctx context.Context, req *connect.Request[lgtmv1.ReconnectClientRequest]) (*connect.Response[lgtmv1.ReconnectClientResponse], error) {
	id := req.Msg.GetId()
	write := recordRequest(req.Header(), lgtmv1connect.ClientServiceReconnectClientProcedure, id)
	command := &linkv1.Command{Action: &linkv1.Command_Restart{Restart: &linkv1.Restart{}}}
	if err := s.queueConnectedAction(ctx, write, id, command, errNotConnected); err != nil {
		return nil, err
	}
	return connect.NewResponse(&lgtmv1.ReconnectClientResponse{}), nil
}

func (s *Service) queueConnectedAction(ctx context.Context, write store.Request, id string, command *linkv1.Command, refusal error) error {
	return s.writeRecordChange(ctx, write, func(ctx context.Context, tx *store.Tx) error {
		machine, err := s.registry.Lock(ctx, tx, id)
		if err != nil {
			return err
		}
		if machine.Approval != ApprovalApproved || s.commands == nil || s.link == nil {
			return refusal
		}
		rows, err := s.link.Presence(ctx, tx, []string{id})
		if err != nil {
			return err
		}
		presence := rows[id]
		if !presence.Held {
			return refusal
		}
		if command.GetScanHarnesses() != nil && (!presence.FactsReceived || !presence.HarnessesReceived) {
			return refusal
		}
		return s.commands.Enqueue(ctx, tx, id, command, 2*time.Minute)
	})
}
