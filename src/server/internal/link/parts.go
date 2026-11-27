package link

import (
	"context"
	"errors"
	"maps"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

type CommandKind string

const CommandLookAtFolder CommandKind = "look_at_folder"

type ReportKind string

const ReportFolderSeen ReportKind = "folder_seen"

type SettlementOutcome string

const (
	OutcomeDone        SettlementOutcome = "done"
	OutcomeFailed      SettlementOutcome = "failed"
	OutcomeExpired     SettlementOutcome = "expired"
	OutcomeUnsupported SettlementOutcome = "unsupported"
	OutcomeInterrupted SettlementOutcome = "interrupted"
	OutcomeTimedOut    SettlementOutcome = "timed_out"
	OutcomeRemoved     SettlementOutcome = "removed"
)

type Settlement struct {
	MachineID       string
	Command         *linkv1.Command
	Outcome         SettlementOutcome
	LastEventNumber *uint64
}

type Admission struct {
	MachineID, StreamID, RunID string
	FirstEventNumber           uint64
	Ready                      bool
}

type RunChange struct {
	MachineID, PreviousRunID, RunID string
	FirstEventNumber                uint64
}

// PartHooks run inside the Link write with its context and transaction.
// An error aborts the write, and a later attempt may call the hook again.
// Hooks may write part records, record feed changes, and enqueue for the locked machine.
// Hooks must avoid nested writes, additional machine locks, client waits and external work.
type PartHooks struct {
	CommandSettled map[CommandKind]func(context.Context, *store.Tx, Settlement) error
	ReportApplied  map[ReportKind]func(context.Context, *store.Tx, string, *linkv1.Event) error
	StreamAdmitted func(context.Context, *store.Tx, Admission) error
	RunChanged     func(context.Context, *store.Tx, RunChange) error
	MachineRemoved func(context.Context, *store.Tx, string) error
}

var errDuplicatePartRoute = errors.New("A link part route is already registered.")

// RegisterPart installs a part's routes and lifecycle hooks before Link workers start.
func (s *State) RegisterPart(hooks PartHooks) error {
	for kind := range hooks.CommandSettled {
		if _, exists := s.commandSettled[kind]; exists {
			return errDuplicatePartRoute
		}
	}
	for kind := range hooks.ReportApplied {
		if _, exists := s.reportApplied[kind]; exists {
			return errDuplicatePartRoute
		}
	}
	maps.Copy(s.commandSettled, hooks.CommandSettled)
	maps.Copy(s.reportApplied, hooks.ReportApplied)
	s.lifecycleHooks = append(s.lifecycleHooks, PartHooks{
		StreamAdmitted: hooks.StreamAdmitted,
		RunChanged:     hooks.RunChanged,
		MachineRemoved: hooks.MachineRemoved,
	})
	return nil
}

func (s *State) dispatchSettlement(ctx context.Context, tx *store.Tx, kind CommandKind, settlement Settlement) error {
	if hook := s.commandSettled[kind]; hook != nil {
		return hook(ctx, tx, settlement)
	}
	return nil
}

func (s *State) dispatchReport(ctx context.Context, tx *store.Tx, kind ReportKind, machineID string, event *linkv1.Event) error {
	if hook := s.reportApplied[kind]; hook != nil {
		return hook(ctx, tx, machineID, event)
	}
	return nil
}

func (s *State) dispatchAdmission(ctx context.Context, tx *store.Tx, admission Admission) error {
	for _, hooks := range s.lifecycleHooks {
		if hooks.StreamAdmitted != nil {
			if err := hooks.StreamAdmitted(ctx, tx, admission); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *State) dispatchRunChange(ctx context.Context, tx *store.Tx, change RunChange) error {
	for _, hooks := range s.lifecycleHooks {
		if hooks.RunChanged != nil {
			if err := hooks.RunChanged(ctx, tx, change); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *State) dispatchRemoval(ctx context.Context, tx *store.Tx, machineID string) error {
	for _, hooks := range s.lifecycleHooks {
		if hooks.MachineRemoved != nil {
			if err := hooks.MachineRemoved(ctx, tx, machineID); err != nil {
				return err
			}
		}
	}
	return nil
}
