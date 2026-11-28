package projects

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

const (
	messageFolderNoAnswer  = "The machine did not answer the folder request."
	messageProjectInternal = "The server could not complete the request."
)

var (
	errFolderMachineUnavailable = store.NewDomainError("The machine is not connected.")
	errFolderWaitExpired        = errors.New(messageFolderNoAnswer)
)

func projectError(code connect.Code, message string) error {
	return connect.NewError(code, errors.New(message))
}

func invalidFolderInput() error {
	return projectError(connect.CodeInvalidArgument, "The machine, repository, or folder path is invalid.")
}

func folderWaitError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return errFolderWaitExpired
	}
	return err
}

func folderOperationError(err error) error {
	switch {
	case errors.Is(err, store.ErrInvalidRequestKey):
		return connect.NewError(connect.CodeInvalidArgument, store.ErrInvalidRequestKey)
	case errors.Is(err, store.ErrRequestKeyConflict):
		return connect.NewError(connect.CodeAlreadyExists, store.ErrRequestKeyConflict)
	case errors.Is(err, store.ErrRequestFingerprintUnavailable):
		return connect.NewError(connect.CodeFailedPrecondition, store.ErrRequestFingerprintUnavailable)
	case errors.Is(err, machines.ErrNoMachine):
		return projectError(connect.CodeNotFound, "The machine was not found.")
	case errors.Is(err, errFolderMachineUnavailable):
		return connect.NewError(connect.CodeUnavailable, errFolderMachineUnavailable)
	case errors.Is(err, errFolderWaitExpired):
		return connect.NewError(connect.CodeDeadlineExceeded, errFolderWaitExpired)
	case errors.Is(err, context.Canceled):
		return projectError(connect.CodeCanceled, "The request was canceled.")
	case errors.Is(err, context.DeadlineExceeded):
		return projectError(connect.CodeDeadlineExceeded, "The request timed out.")
	}
	var dbErr *store.Error
	if errors.As(err, &dbErr) {
		if dbErr.Outcome() == store.OutcomeUnknown {
			return projectError(connect.CodeUnavailable, "The database did not confirm whether the write committed.")
		}
		if dbErr.Retryable() {
			return projectError(connect.CodeAborted, "The database was busy, retry the request.")
		}
	}
	return projectError(connect.CodeUnavailable, "The server cannot reach its database, try again.")
}

func storedFolderFailure(failure string) error {
	switch failure {
	case "not_a_folder":
		return projectError(connect.CodeFailedPrecondition, "The path is not a folder.")
	case "not_a_git_repository":
		return projectError(connect.CodeFailedPrecondition, "The folder is not a Git repository.")
	case "not_readable":
		return projectError(connect.CodeFailedPrecondition, "The folder cannot be read.")
	case "path_not_usable":
		return projectError(connect.CodeFailedPrecondition, "The folder path cannot be used.")
	case "repository_not_found":
		return projectError(connect.CodeNotFound, "The repository was not found.")
	case "folder_owned_elsewhere":
		return projectError(connect.CodeAlreadyExists, "The folder is already saved for another repository.")
	case "repository_mismatch":
		return projectError(connect.CodeFailedPrecondition, "The folder does not belong to the selected repository.")
	case "no_answer":
		return projectError(connect.CodeDeadlineExceeded, messageFolderNoAnswer)
	case "not_taken":
		return projectError(connect.CodeDeadlineExceeded, "The machine did not take the folder command.")
	case "unknown":
		return projectError(connect.CodeDeadlineExceeded, "The outcome of the folder request is unknown.")
	case "machine_removed":
		return projectError(connect.CodeNotFound, "The machine was removed.")
	default:
		return projectError(connect.CodeInternal, messageProjectInternal)
	}
}
