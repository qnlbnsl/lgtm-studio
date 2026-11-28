package app

import (
	"context"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/projects"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

func registerProjectHooks(state *link.State, service *projects.Service) error {
	return state.RegisterPart(link.PartHooks{
		ReportApplied: map[link.ReportKind]func(context.Context, *store.Tx, string, *linkv1.Event) error{
			link.ReportFolderSeen: func(ctx context.Context, tx *store.Tx, machineID string, event *linkv1.Event) error {
				return service.ApplyFolderSeen(ctx, tx, machineID, event.GetFolderSeen())
			},
		},
		CommandSettled: map[link.CommandKind]func(context.Context, *store.Tx, link.Settlement) error{
			link.CommandLookAtFolder: func(ctx context.Context, tx *store.Tx, settlement link.Settlement) error {
				return service.SettleCommand(ctx, tx, settlement.MachineID, settlement.Command.GetId(), string(settlement.Outcome))
			},
		},
		MachineRemoved: service.RemoveMachine,
	})
}
