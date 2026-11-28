package projects_test

import (
	"testing"

	"connectrpc.com/connect"
)

func TestProjectMachineRemovalFailsOpenOperationsWithoutCommands(t *testing.T) {
	f := newRemovalFixture(t, false)
	inspect := f.admit(t, "inspect without a link command", false)
	add := f.admit(t, "add without a link command", true)
	f.disconnect(t)
	f.exec(t, "DELETE FROM link_commands WHERE machine_id = $1", f.machine)
	if !repositoryObserve[bool](t, f.reportFixture, `
SELECT NOT EXISTS (SELECT FROM link_commands WHERE machine_id = $1)
   AND (SELECT count(*) FROM folder_operations WHERE id IN ($2, $3) AND state = 'open') = 2`,
		f.machine, inspect.id, add.id) {
		t.Fatal("The fixture must leave both operations open without Link commands.")
	}

	requireRemovalSuccess(t, f.remove(t.Context(), f.machine, ""))
	t.Run("Inspect", func(t *testing.T) {
		f.requireFailure(t, inspect, "machine_removed", connect.CodeNotFound, removalMachineMessage)
	})
	t.Run("Add", func(t *testing.T) {
		f.requireFailure(t, add, "machine_removed", connect.CodeNotFound, removalMachineMessage)
	})
}
