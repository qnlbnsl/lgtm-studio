package link

import (
	"context"
	"time"

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

type PartHooks struct {
	CommandSettled map[CommandKind]func(context.Context, *store.Tx, Settlement) error
	ReportApplied  map[ReportKind]func(context.Context, *store.Tx, string, *linkv1.Event) error
	StreamAdmitted func(context.Context, *store.Tx, Admission) error
	RunChanged     func(context.Context, *store.Tx, RunChange) error
	MachineRemoved func(context.Context, *store.Tx, string) error
}

// EnqueueCommand is a stub from the test writer.
func (*State) EnqueueCommand(context.Context, *store.Tx, string, *linkv1.Command, time.Duration) (string, error) {
	return "", nil
}

// RegisterPart is a stub from the test writer.
func (*State) RegisterPart(PartHooks) error {
	return nil
}

// dispatchSettlement is a stub from the test writer.
func (*State) dispatchSettlement(context.Context, *store.Tx, CommandKind, Settlement) error {
	return nil
}

// dispatchReport is a stub from the test writer.
func (*State) dispatchReport(context.Context, *store.Tx, ReportKind, string, *linkv1.Event) error {
	return nil
}

// dispatchAdmission is a stub from the test writer.
func (*State) dispatchAdmission(context.Context, *store.Tx, Admission) error {
	return nil
}

// dispatchRunChange is a stub from the test writer.
func (*State) dispatchRunChange(context.Context, *store.Tx, RunChange) error {
	return nil
}

// dispatchRemoval is a stub from the test writer.
func (*State) dispatchRemoval(context.Context, *store.Tx, string) error {
	return nil
}
