package projects

import (
	"context"
	"encoding/binary"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

const (
	folderCommandTTL   = 20 * time.Second
	folderWaitLimit    = 25 * time.Second
	folderPollInterval = 250 * time.Millisecond
	maxTypedPathBytes  = 4096
)

var projectID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type folderRequest struct {
	kind      string
	procedure string
	machineID string
	typedPath string
	targetID  *string
}

type folderResult struct {
	state   string
	reply   []byte
	failure string
}

// InspectFolder starts or replays an inspection of a folder on a machine.
func (s *Service) InspectFolder(ctx context.Context, req *connect.Request[webv1.InspectFolderRequest]) (*connect.Response[webv1.InspectFolderResponse], error) {
	var response webv1.InspectFolderResponse
	if err := s.runFolderOperation(ctx, req.Header(), folderRequest{
		kind: "inspect", procedure: lgtmv1connect.ProjectServiceInspectFolderProcedure,
		machineID: req.Msg.GetClientId(), typedPath: req.Msg.GetPath(),
	}, &response); err != nil {
		return nil, err
	}
	return connect.NewResponse(&response), nil
}

// AddRepository starts or replays an operation to save or attach a repository folder.
func (s *Service) AddRepository(ctx context.Context, req *connect.Request[webv1.AddRepositoryRequest]) (*connect.Response[webv1.AddRepositoryResponse], error) {
	if req.Msg == nil {
		return nil, invalidFolderInput()
	}
	var response webv1.AddRepositoryResponse
	if err := s.runFolderOperation(ctx, req.Header(), folderRequest{
		kind: "add", procedure: lgtmv1connect.ProjectServiceAddRepositoryProcedure,
		machineID: req.Msg.GetClientId(), typedPath: req.Msg.GetPath(), targetID: req.Msg.RepositoryId,
	}, &response); err != nil {
		return nil, err
	}
	return connect.NewResponse(&response), nil
}

func (s *Service) runFolderOperation(ctx context.Context, header http.Header, request folderRequest, response proto.Message) error {
	if !projectID.MatchString(request.machineID) || !validTypedFolderPath(request.typedPath) ||
		(request.targetID != nil && !projectID.MatchString(*request.targetID)) {
		return invalidFolderInput()
	}
	keys := header.Values(store.RequestKeyHeader)
	if len(keys) != 1 {
		return folderOperationError(store.ErrInvalidRequestKey)
	}
	// Store checks the key's text and byte limit before opening its transaction.
	key := keys[0]
	result := folderResult{state: "open"}
	savedID, err := s.store.WriteOnceRetained(ctx, request.receipt(key),
		func(ctx context.Context, tx *store.Tx) ([]byte, error) {
			return s.startFolderOperation(ctx, tx, key, request)
		},
		func(ctx context.Context, tx *store.Tx, savedID []byte) error {
			var err error
			result, err = readFolderResult(ctx, tx, string(savedID))
			return err
		},
	)
	if err != nil {
		return folderOperationError(err)
	}
	if result.state == "open" {
		result, err = s.waitFolderOperation(ctx, string(savedID))
		if err != nil {
			return folderOperationError(err)
		}
	}
	switch result.state {
	case "succeeded":
		if proto.Unmarshal(result.reply, response) == nil {
			return nil
		}
	case "failed":
		return storedFolderFailure(result.failure)
	}
	return projectError(connect.CodeInternal, messageProjectInternal)
}

func validTypedFolderPath(value string) bool {
	return len(value) <= maxTypedPathBytes && utf8.ValidString(value) && strings.IndexByte(value, 0) < 0 &&
		(path.IsAbs(value) || value == "~" || strings.HasPrefix(value, "~/"))
}

func (r folderRequest) receipt(key string) store.Request {
	fields := []string{r.machineID, r.typedPath}
	if r.kind == "add" {
		target := ""
		if r.targetID != nil {
			target = *r.targetID
		}
		fields = append(fields, target)
	}
	var fingerprint []byte
	for _, field := range fields {
		fingerprint = binary.BigEndian.AppendUint64(fingerprint, uint64(len(field)))
		fingerprint = append(fingerprint, field...)
	}
	const version = 1
	return store.Request{
		Key: key, Caller: "owner", Procedure: r.procedure, ReceiptSafe: true,
		FingerprintVersion: version, Fingerprints: map[int16][]byte{version: fingerprint},
	}
}

func (s *Service) startFolderOperation(ctx context.Context, tx *store.Tx, key string, request folderRequest) ([]byte, error) {
	machine, err := s.registry.Lock(ctx, tx, request.machineID)
	if err != nil {
		return nil, err
	}
	if machine.Approval != machines.ApprovalApproved || s.commands == nil {
		return nil, errFolderMachineUnavailable
	}
	presence, err := s.commands.Presence(ctx, tx, []string{request.machineID})
	if err != nil {
		return nil, err
	}
	if !presence[request.machineID].Held {
		return nil, errFolderMachineUnavailable
	}
	id, err := store.NewID()
	if err != nil {
		return nil, err
	}
	command := &linkv1.Command{Action: &linkv1.Command_LookAtFolder{LookAtFolder: &linkv1.LookAtFolder{
		OperationId: id, Path: request.typedPath,
	}}}
	commandID, err := s.commands.EnqueueCommand(ctx, tx, request.machineID, command, folderCommandTTL)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO folder_operations (id, machine_id, request_key, kind, target_repository_id, command_id, state)
VALUES ($1, $2, $3, $4, $5, $6, 'open')`, id, request.machineID, key, request.kind, request.targetID, commandID)
	if err != nil {
		return nil, err
	}
	return []byte(id), nil
}

func readFolderResult(ctx context.Context, q machines.Querier, id string) (folderResult, error) {
	var result folderResult
	// Scanning BYTEA into a byte slice copies the reply while the read transaction is open.
	err := q.QueryRow(ctx, `
SELECT state, reply, COALESCE(failure, '') FROM folder_operations WHERE id = $1`, id).
		Scan(&result.state, &result.reply, &result.failure)
	return result, err
}

func (s *Service) waitFolderOperation(ctx context.Context, id string) (folderResult, error) {
	ctx, cancel := context.WithTimeout(ctx, folderWaitLimit)
	defer cancel()
	ticker := time.NewTicker(folderPollInterval)
	defer ticker.Stop()
	for {
		var result folderResult
		err := s.store.Read(ctx, func(ctx context.Context, tx *store.ReadTx) error {
			var err error
			result, err = readFolderResult(ctx, tx, id)
			return err
		})
		if err := ctx.Err(); err != nil {
			return folderResult{}, folderWaitError(err)
		}
		if err != nil || result.state != "open" {
			return result, err
		}
		select {
		case <-ctx.Done():
			return folderResult{}, folderWaitError(ctx.Err())
		case <-ticker.C:
		}
	}
}
