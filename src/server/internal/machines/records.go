package machines

import (
	"context"
	"encoding/binary"
	"errors"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	lgtmv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

var errRenameWaiting = store.NewDomainError("Approve this machine before you rename it.")

// CreateClientToken issues a fresh join token for a machine that does not host the server.
func (s *Service) CreateClientToken(ctx context.Context, req *connect.Request[lgtmv1.CreateClientTokenRequest]) (*connect.Response[lgtmv1.CreateClientTokenResponse], error) {
	if _, present := req.Header()[http.CanonicalHeaderKey(store.RequestKeyHeader)]; present {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("This call does not accept a request key."))
	}
	token, err := s.tokens.Create(ctx, TokenOptions{})
	if err != nil {
		return nil, recordError(err)
	}
	res := connect.NewResponse(&lgtmv1.CreateClientTokenResponse{
		TokenId: token.ID, Token: token.Text, ExpiresAt: timestamppb.New(token.ExpiresAt), Address: token.Address,
	})
	res.Header().Set("Cache-Control", "no-store")
	return res, nil
}

// RevokeClientToken revokes the token and removes the machine if it still waits for approval.
func (s *Service) RevokeClientToken(ctx context.Context, req *connect.Request[lgtmv1.RevokeClientTokenRequest]) (*connect.Response[lgtmv1.RevokeClientTokenResponse], error) {
	id := req.Msg.GetTokenId()
	write := recordRequest(req.Header(), lgtmv1connect.ClientServiceRevokeClientTokenProcedure, id)
	err := s.writeRecordChange(ctx, write, func(ctx context.Context, tx *store.Tx) error {
		machineID, err := s.tokens.Revoke(ctx, tx, id)
		if err != nil || machineID == "" {
			return err
		}
		deleted, err := s.registry.DeleteWaiting(ctx, tx, machineID)
		if err != nil || !deleted || s.link == nil {
			return err
		}
		return s.link.Forget(ctx, tx, machineID)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&lgtmv1.RevokeClientTokenResponse{}), nil
}

// RemoveClient deletes a machine of any standing.
func (s *Service) RemoveClient(ctx context.Context, req *connect.Request[lgtmv1.RemoveClientRequest]) (*connect.Response[lgtmv1.RemoveClientResponse], error) {
	id := req.Msg.GetId()
	write := recordRequest(req.Header(), lgtmv1connect.ClientServiceRemoveClientProcedure, id)
	err := s.writeRecordChange(ctx, write, func(ctx context.Context, tx *store.Tx) error {
		if err := s.registry.Delete(ctx, tx, id); err != nil {
			return err
		}
		if s.link != nil {
			return s.link.Forget(ctx, tx, id)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&lgtmv1.RemoveClientResponse{}), nil
}

// ApproveClient approves a waiting machine and returns its projection from the same write.
func (s *Service) ApproveClient(ctx context.Context, req *connect.Request[lgtmv1.ApproveClientRequest]) (*connect.Response[lgtmv1.ApproveClientResponse], error) {
	name, err := NormalizeName(req.Msg.GetName())
	if err != nil {
		return nil, recordError(err)
	}
	id := req.Msg.GetId()
	write := recordRequest(req.Header(), lgtmv1connect.ClientServiceApproveClientProcedure, id, name)
	reply, err := s.store.WriteOnce(ctx, write, func(ctx context.Context, tx *store.Tx) ([]byte, error) {
		if err := s.registry.SetApproved(ctx, tx, id, name); err != nil {
			return nil, err
		}
		machine, err := s.registry.Get(ctx, tx, id)
		if err != nil {
			return nil, err
		}
		if err := s.enqueueStanding(ctx, tx, machine); err != nil {
			return nil, err
		}
		client, err := s.projectOne(ctx, tx, machine)
		if err != nil {
			return nil, err
		}
		return proto.Marshal(&lgtmv1.ApproveClientResponse{Client: client})
	})
	if err != nil {
		return nil, recordError(err)
	}
	var response lgtmv1.ApproveClientResponse
	if err := proto.Unmarshal(reply, &response); err != nil {
		return nil, recordError(err)
	}
	return connect.NewResponse(&response), nil
}

// RenameClient changes the name of an approved machine.
func (s *Service) RenameClient(ctx context.Context, req *connect.Request[lgtmv1.RenameClientRequest]) (*connect.Response[lgtmv1.RenameClientResponse], error) {
	name, err := NormalizeName(req.Msg.GetName())
	if err != nil {
		return nil, recordError(err)
	}
	id := req.Msg.GetId()
	write := recordRequest(req.Header(), lgtmv1connect.ClientServiceRenameClientProcedure, id, name)
	err = s.writeRecordChange(ctx, write, func(ctx context.Context, tx *store.Tx) error {
		machine, err := s.registry.Lock(ctx, tx, id)
		if err != nil {
			return err
		}
		if machine.Approval == ApprovalWaiting {
			return errRenameWaiting
		}
		changed, err := s.registry.SetName(ctx, tx, id, name)
		if err != nil || !changed || s.commands == nil {
			return err
		}
		machine, err = s.registry.Get(ctx, tx, id)
		if err != nil {
			return err
		}
		return s.enqueueStanding(ctx, tx, machine)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&lgtmv1.RenameClientResponse{}), nil
}

func (s *Service) enqueueStanding(ctx context.Context, tx *store.Tx, machine Machine) error {
	if s.commands == nil {
		return nil
	}
	command := &linkv1.Command{Action: &linkv1.Command_SetState{
		SetState: &linkv1.SetState{State: Standing(machine)},
	}}
	return s.commands.Enqueue(ctx, tx, machine.ID, command, time.Hour)
}

func recordRequest(header http.Header, procedure string, fields ...string) store.Request {
	key := header.Get(store.RequestKeyHeader)
	if key == "" {
		return store.Request{}
	}
	var fingerprint []byte
	for _, field := range fields {
		fingerprint = binary.BigEndian.AppendUint64(fingerprint, uint64(len(field)))
		fingerprint = append(fingerprint, field...)
	}
	const version = 1
	return store.Request{
		Key: key, Caller: "owner", Procedure: procedure, ReceiptSafe: true,
		FingerprintVersion: version, Fingerprints: map[int16][]byte{version: fingerprint},
	}
}

func (s *Service) writeRecordChange(ctx context.Context, req store.Request, change func(context.Context, *store.Tx) error) error {
	_, err := s.store.WriteOnce(ctx, req, func(ctx context.Context, tx *store.Tx) ([]byte, error) {
		return nil, change(ctx, tx)
	})
	if err != nil {
		return recordError(err)
	}
	return nil
}

func recordError(err error) error {
	switch err {
	case ErrNoMachine:
		return connect.NewError(connect.CodeNotFound, err)
	case ErrNameTaken, store.ErrRequestKeyConflict:
		return connect.NewError(connect.CodeAlreadyExists, err)
	case ErrNotWaiting, errRenameWaiting, errNotOnline, errNotConnected, store.ErrRequestFingerprintUnavailable:
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case ErrInvalidName, store.ErrInvalidRequestKey, store.ErrInvalidRequest:
		return connect.NewError(connect.CodeInvalidArgument, err)
	case context.Canceled:
		return connect.NewError(connect.CodeCanceled, err)
	case context.DeadlineExceeded:
		return connect.NewError(connect.CodeDeadlineExceeded, err)
	}
	var dbErr *store.Error
	if errors.As(err, &dbErr) && dbErr.Retryable() {
		return connect.NewError(connect.CodeAborted, errors.New("The database was busy, retry the request."))
	}
	return connect.NewError(connect.CodeUnavailable, errors.New("The server cannot reach its database, try again."))
}
