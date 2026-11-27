package link

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

var (
	ErrUnknownMachine = store.NewDomainError("The server does not know this machine. Join it again.")
	ErrNotCurrent     = store.NewDomainError("The stream is no longer current.")
	ErrInvalidMessage = store.NewDomainError("The link message broke a rule of the contract.")
)

type State struct {
	store     *store.Store
	registry  *machines.Registry
	hub       *Hub
	afterLock func(context.Context, string)

	commandSettled map[CommandKind]func(context.Context, *store.Tx, Settlement) error
	reportApplied  map[ReportKind]func(context.Context, *store.Tx, string, *linkv1.Event) error
	lifecycleHooks []PartHooks
}

type Stream struct {
	ID              string
	Generation      int64
	Standing        machines.Machine
	Mark            uint64
	replayThrough   uint64
	ServerTime      time.Time
	ConnectionToken string
}

func NewState(s *store.Store, r *machines.Registry, h *Hub) *State {
	return &State{
		store:          s,
		registry:       r,
		hub:            h,
		commandSettled: make(map[CommandKind]func(context.Context, *store.Tx, Settlement) error),
		reportApplied:  make(map[ReportKind]func(context.Context, *store.Tx, string, *linkv1.Event) error),
	}
}

func (s *State) BecomeCurrent(ctx context.Context, machineID, streamID, address string, announcement RunAnnouncement) (Stream, error) {
	var stream Stream
	var recoveryRefusal bool
	err := s.store.Write(ctx, func(ctx context.Context, tx *store.Tx) error {
		machine, err := s.registry.Lock(ctx, tx, machineID)
		if err == machines.ErrNoMachine {
			return ErrUnknownMachine
		}
		if err != nil {
			return err
		}
		s.locked(ctx, machineID)
		run, err := classifyRun(ctx, tx, machineID, announcement)
		if err == errRunBehindMark {
			recoveryRefusal = true
			if _, err := tx.Exec(ctx, `
UPDATE link_machines SET recovery_set_generation = stream_generation WHERE machine_id = $1`, machineID); err != nil {
				return err
			}
			if !run.recoverySet {
				return s.registry.Touch(ctx, tx, machineID)
			}
			return nil
		}
		if err != nil {
			return err
		}
		stream = Stream{
			ID: streamID, Standing: machine, Mark: uint64(run.mark), replayThrough: uint64(run.replayThrough),
		}
		if machine.Approval == machines.ApprovalApproved {
			var token [32]byte
			rand.Read(token[:])
			stream.ConnectionToken = base64.RawURLEncoding.EncodeToString(token[:])
		}
		err = tx.QueryRow(ctx, `
INSERT INTO link_machines (machine_id, stream_generation, admitted_run_id) VALUES ($1, 1, $2)
ON CONFLICT (machine_id) DO UPDATE SET
    stream_generation = link_machines.stream_generation + 1, admitted_run_id = EXCLUDED.admitted_run_id
RETURNING stream_generation, clock_timestamp()`, machineID, announcement.ID).
			Scan(&stream.Generation, &stream.ServerTime)
		if err != nil {
			return err
		}
		if run.unseen {
			if _, err := tx.Exec(ctx, `
INSERT INTO link_runs (machine_id, run_id, replay_through) VALUES ($1, $2, $3)`,
				machineID, announcement.ID, run.replayThrough); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `
INSERT INTO link_connections (machine_id, stream_id, generation, connected_at, last_heard_at, address)
VALUES ($1, $2, $3, $4, $4, $5)
ON CONFLICT (machine_id) DO UPDATE SET
    stream_id = EXCLUDED.stream_id, generation = EXCLUDED.generation,
    connected_at = EXCLUDED.connected_at, last_heard_at = EXCLUDED.last_heard_at,
    address = EXCLUDED.address, cpu_percent = NULL, memory_percent = NULL,
    facts_received = false, harnesses_received = false`, machineID, streamID, stream.Generation, stream.ServerTime, address)
		if err != nil {
			return err
		}
		ready := run.mark >= run.replayThrough
		if err := s.dispatchAdmission(ctx, tx, Admission{
			MachineID: machineID, StreamID: streamID, RunID: announcement.ID,
			FirstEventNumber: announcement.FirstEventNumber, Ready: ready,
		}); err != nil {
			return err
		}
		if ready {
			if err := s.promoteRun(ctx, tx, machineID, stream.Mark); err != nil {
				return err
			}
		}
		return s.registry.Touch(ctx, tx, machineID)
	})
	if recoveryRefusal {
		if err != nil {
			return Stream{}, &recoveryDiagnosticError{err}
		}
		return Stream{}, ErrInvalidMessage
	}
	if err != nil {
		return Stream{}, err
	}
	return stream, nil
}

func (s *State) CheckCurrent(ctx context.Context, machineID, streamID string) (machines.Approval, error) {
	var approval machines.Approval
	err := s.store.Read(ctx, func(ctx context.Context, tx *store.ReadTx) error {
		machine, err := s.registry.Get(ctx, tx, machineID)
		if err == machines.ErrNoMachine {
			return ErrUnknownMachine
		}
		if err != nil {
			return err
		}
		if err := checkCurrentStream(ctx, tx, machineID, streamID); err != nil {
			return err
		}
		approval = machine.Approval
		return nil
	})
	if err != nil {
		return "", err
	}
	return approval, nil
}

func (s *State) noteWelcome(ctx context.Context, machineID, streamID string) error {
	return s.store.Write(ctx, func(ctx context.Context, tx *store.Tx) error {
		if _, err := s.registry.Lock(ctx, tx, machineID); err != nil {
			if err == machines.ErrNoMachine {
				return ErrUnknownMachine
			}
			return err
		}
		if err := checkCurrentStream(ctx, tx, machineID, streamID); err != nil {
			return err
		}
		return s.registry.Touch(ctx, tx, machineID)
	})
}

func checkCurrentStream(ctx context.Context, q machines.Querier, machineID, streamID string) error {
	var current bool
	if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT FROM link_connections WHERE machine_id = $1 AND stream_id = $2)`, machineID, streamID).Scan(&current); err != nil {
		return err
	}
	if !current {
		return ErrNotCurrent
	}
	return nil
}

func (s *State) Heartbeat(ctx context.Context, machineID, streamID string, hb *linkv1.Heartbeat) error {
	return s.store.Write(ctx, func(ctx context.Context, tx *store.Tx) error {
		_, err := s.registry.Lock(ctx, tx, machineID)
		if err == machines.ErrNoMachine {
			return ErrUnknownMachine
		}
		if err != nil {
			return err
		}
		s.locked(ctx, machineID)
		if err := checkCurrentStream(ctx, tx, machineID, streamID); err != nil {
			return err
		}
		if hb == nil || hb.GetCpuPercent() > 100 || hb.GetMemoryPercent() > 100 {
			return ErrInvalidMessage
		}
		_, err = tx.Exec(ctx, `
UPDATE link_connections SET cpu_percent = $2, memory_percent = $3, last_heard_at = clock_timestamp()
WHERE machine_id = $1`, machineID, hb.CpuPercent, hb.MemoryPercent)
		if err != nil {
			return err
		}
		return s.registry.Touch(ctx, tx, machineID)
	})
}

func (s *State) Disconnect(ctx context.Context, machineID, streamID string) (bool, error) {
	var changed bool
	err := s.store.Write(ctx, func(ctx context.Context, tx *store.Tx) error {
		_, err := s.registry.TryLock(ctx, tx, machineID)
		if err == machines.ErrNoMachine {
			return nil
		}
		if err != nil {
			return err
		}
		s.locked(ctx, machineID)
		var heardAt time.Time
		var address string
		err = tx.QueryRow(ctx, `
DELETE FROM link_connections WHERE machine_id = $1 AND stream_id = $2
RETURNING last_heard_at, address`, machineID, streamID).Scan(&heardAt, &address)
		if err == pgx.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		if err := s.registry.NoteSeen(ctx, tx, machineID, heardAt, address); err != nil {
			return err
		}
		changed = true
		return s.registry.Touch(ctx, tx, machineID)
	})
	return err == nil && changed, err
}

func (s *State) ClearConnections(ctx context.Context) error {
	type connection struct{ machineID, streamID string }
	var stale []connection
	err := s.store.Read(ctx, func(ctx context.Context, tx *store.ReadTx) error {
		rows, err := tx.Query(ctx, `SELECT machine_id, stream_id FROM link_connections ORDER BY machine_id`)
		if err != nil {
			return err
		}
		stale, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (connection, error) {
			var c connection
			err := row.Scan(&c.machineID, &c.streamID)
			return c, err
		})
		return err
	})
	if err != nil {
		return err
	}
	for _, c := range stale {
		if _, err := s.Disconnect(ctx, c.machineID, c.streamID); err != nil {
			return err
		}
	}
	return nil
}

func (s *State) Presence(ctx context.Context, q machines.Querier, ids []string) (map[string]machines.Presence, error) {
	out := make(map[string]machines.Presence)
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := q.Query(ctx, `
SELECT m.machine_id, COALESCE(c.stream_id, ''), COALESCE(c.generation, 0),
       c.connected_at, c.last_heard_at, COALESCE(c.address, ''), c.cpu_percent, c.memory_percent,
       COALESCE(c.facts_received, false), COALESCE(c.harnesses_received, false),
       m.recovery_set_generation IS NOT NULL
FROM link_machines m
LEFT JOIN link_connections c ON c.machine_id = m.machine_id
WHERE m.machine_id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	watching := machines.WatchingPresence(ctx)
	for rows.Next() {
		var id string
		var p machines.Presence
		var connectedAt, lastHeardAt *time.Time
		if err := rows.Scan(&id, &p.StreamID, &p.Generation, &connectedAt, &lastHeardAt, &p.Address,
			&p.CPUPercent, &p.MemoryPercent, &p.FactsReceived, &p.HarnessesReceived, &p.EventHistoryNeedsRecovery); err != nil {
			return nil, err
		}
		if p.StreamID != "" {
			p.ConnectedAt, p.LastHeardAt = *connectedAt, *lastHeardAt
			p.Held = s.hub.holds(id, p.StreamID, watching)
		}
		out[id] = p
	}
	return out, rows.Err()
}

func (s *State) Forget(ctx context.Context, tx *store.Tx, machineID string) error {
	if _, err := tx.Exec(ctx, `
UPDATE link_commands
SET outcome = 'removed', finished_at = clock_timestamp(), payload = NULL
WHERE machine_id = $1 AND outcome IS NULL`, machineID); err != nil {
		return err
	}
	var streamID string
	err := tx.QueryRow(ctx, `DELETE FROM link_connections WHERE machine_id = $1 RETURNING stream_id`, machineID).Scan(&streamID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM link_machines WHERE machine_id = $1`, machineID); err != nil {
		return err
	}
	s.hub.recordRemoval(machineID, streamID)
	return nil
}

func (s *State) locked(ctx context.Context, machineID string) {
	if s.afterLock != nil {
		s.afterLock(ctx, machineID)
	}
}
