package link

import (
	"context"
	"regexp"

	"github.com/jackc/pgx/v5"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

// RunAnnouncement identifies a client process and the first event of that process.
type RunAnnouncement struct {
	ID               string
	FirstEventNumber uint64
}

var (
	runUUID          = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	errRunBehindMark = store.NewDomainError(ErrInvalidMessage.Error())
)

type runAdmission struct {
	mark          int64
	replayThrough int64
	unseen        bool
}

// classifyRun reads the durable fences with the machine row locked.
func classifyRun(ctx context.Context, tx *store.Tx, machineID string, announcement RunAnnouncement) (runAdmission, error) {
	if !runUUID.MatchString(announcement.ID) || announcement.FirstEventNumber == 0 ||
		announcement.FirstEventNumber > 1<<63 {
		return runAdmission{}, ErrInvalidMessage
	}
	run := runAdmission{replayThrough: int64(announcement.FirstEventNumber - 1)}
	var latestID string
	var knownBoundary *int64
	var pendingBoundary int64
	err := tx.QueryRow(ctx, `
SELECT m.event_mark, COALESCE(m.admitted_run_id, ''), known.replay_through,
       CASE WHEN m.admitted_run_id IS DISTINCT FROM m.ready_run_id
            THEN COALESCE(latest.replay_through, 0) ELSE 0 END
FROM link_machines m
LEFT JOIN link_runs known ON known.machine_id = m.machine_id AND known.run_id = $2
LEFT JOIN link_runs latest ON latest.machine_id = m.machine_id AND latest.run_id = m.admitted_run_id
WHERE m.machine_id = $1`, machineID, announcement.ID).
		Scan(&run.mark, &latestID, &knownBoundary, &pendingBoundary)
	if err != nil && err != pgx.ErrNoRows {
		return runAdmission{}, err
	}
	if knownBoundary != nil {
		if latestID != announcement.ID || *knownBoundary != run.replayThrough {
			return runAdmission{}, ErrInvalidMessage
		}
		return run, nil
	}

	if run.replayThrough < run.mark {
		return runAdmission{}, errRunBehindMark
	}
	var stagedWatermark int64
	if err := tx.QueryRow(ctx, `
SELECT COALESCE(MAX(staged_event_number), 0)
FROM link_commands
WHERE machine_id = $1 AND staged_event_number IS NOT NULL`, machineID).Scan(&stagedWatermark); err != nil {
		return runAdmission{}, err
	}
	if run.replayThrough < stagedWatermark || run.replayThrough < pendingBoundary {
		return runAdmission{}, ErrInvalidMessage
	}
	run.unseen = true
	return run, nil
}

// promoteRun makes the latest admitted run ready when the mark covers its replay prefix.
// The caller holds the machine lock and has established the current stream.
func (s *State) promoteRun(ctx context.Context, tx *store.Tx, machineID string, mark uint64) error {
	change := RunChange{MachineID: machineID}
	var replayThrough int64
	err := tx.QueryRow(ctx, `
SELECT m.admitted_run_id, COALESCE(m.ready_run_id, ''), r.replay_through
FROM link_machines m
JOIN link_runs r ON r.machine_id = m.machine_id AND r.run_id = m.admitted_run_id
WHERE m.machine_id = $1 AND m.ready_run_id IS DISTINCT FROM m.admitted_run_id AND r.replay_through <= $2`,
		machineID, mark).Scan(&change.RunID, &change.PreviousRunID, &replayThrough)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE link_machines SET ready_run_id = $2 WHERE machine_id = $1`,
		machineID, change.RunID); err != nil {
		return err
	}
	change.FirstEventNumber = uint64(replayThrough) + 1
	return s.dispatchRunChange(ctx, tx, change)
}
