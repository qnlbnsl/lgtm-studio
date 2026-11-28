package link

import (
	"context"
	"errors"
	"time"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/folders"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const folderInspectionBudget = 5 * time.Second

var inspectFolder = folders.Inspect

type activeFolder struct {
	attempt     commandAttempt
	operationID string
	cancel      context.CancelFunc
}

type folderInspection struct {
	facts   *linkv1.FolderFacts
	refusal linkv1.FolderRefusal
	expired bool
}

func (h *commandHandler) startFolder(ctx context.Context, attempt commandAttempt, action *linkv1.LookAtFolder, clock serverClock, admission, expiry time.Time) {
	ctx, cancel := context.WithTimeout(ctx, folderInspectionBudget)
	h.folder = &activeFolder{attempt: attempt, operationID: action.GetOperationId(), cancel: cancel}
	path, home, filter := action.GetPath(), h.folderHome, h.filter
	reads, completed, inspect := h.folderReads, h.folderCompleted, inspectFolder
	canStart := func() bool { return clock.canStart(admission, expiry) }

	go func() {
		defer cancel()
		unreadable := folderInspection{refusal: linkv1.FolderRefusal_FOLDER_REFUSAL_NOT_READABLE}
		select {
		case reads <- struct{}{}:
		default:
			// Retired calls keep their permits until the filesystem returns.
			unreadable.expired = !canStart()
			completed <- unreadable
			return
		}

		read := make(chan folderInspection, 1)
		go func() {
			result := unreadable
			unsafePath := filter.folderPathUnsafe(path, home)
			switch {
			case ctx.Err() != nil:
			case !canStart():
				result.expired = true
			case unsafePath:
				result.refusal = linkv1.FolderRefusal_FOLDER_REFUSAL_PATH_NOT_USABLE
			default:
				result.facts, result.refusal = inspect(ctx, path, home)
			}
			<-reads
			read <- result
		}()

		result := unreadable
		select {
		case <-ctx.Done():
		case candidate := <-read:
			if ctx.Err() == nil {
				result = candidate
			}
		}
		completed <- result
	}()
}

func (h *commandHandler) finishFolder(ctx context.Context, inspection folderInspection) error {
	folder := h.folder
	folder.cancel()
	h.folder = nil
	if ctx.Err() != nil {
		return nil
	}
	if inspection.expired {
		return h.finishCommand(ctx, folder.attempt, outcomeExpired)
	}

	seen := &linkv1.FolderSeen{CommandId: folder.attempt.id, OperationId: folder.operationID}
	if inspection.facts != nil {
		seen.Result = &linkv1.FolderSeen_Facts{Facts: inspection.facts}
	} else {
		seen.Result = &linkv1.FolderSeen_Refusal{Refusal: inspection.refusal}
	}
	event := &linkv1.Event{HappenedAt: timestamppb.Now(), Report: &linkv1.Event_FolderSeen{FolderSeen: seen}}
	_, err := h.store.appendFilteredReport(ctx, h.filter, event)
	switch {
	case err == nil:
		return h.finishCommand(ctx, folder.attempt, outcomeDone)
	case errors.Is(err, ErrOutboxFull), errors.Is(err, ErrEventTooLarge):
		return h.finishCommand(ctx, folder.attempt, outcomeFailed)
	case errors.Is(err, context.Canceled) && ctx.Err() != nil:
		return nil
	default:
		return err
	}
}

// stopFolder joins the deadline wrapper. A read may still hold its permit.
func (h *commandHandler) stopFolder() {
	if h.folder != nil {
		h.folder.cancel()
		<-h.folderCompleted
		h.folder = nil
	}
}
