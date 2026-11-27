package machines_test

import (
	"testing"

	"connectrpc.com/connect"
)

func TestActionRequestKeyConflictsAcrossRoutes(t *testing.T) {
	f := newStateCommandsFixture(t, nil)
	id := f.join(t, f.issue(t, false))
	approveActionMachine(t, f, id)
	row, _ := f.admit(t, id, 1)
	f.apply(t, id, row.ID, 1, true)
	f.apply(t, id, row.ID, 2, false)

	const key = "action-shared-across-routes"
	head := f.head(t)
	scan, restart := actionRoutes[0], actionRoutes[1]
	if scan.call(t.Context(), f.recordsFixture, f.clients, id, key) != nil || f.noticeCount(t, id) != 1 {
		t.Fatal("The first action did not succeed and queue exactly one scan.")
	}
	requireQueuedAction(t, f, scan, id, key, head)

	before, head := f.digest(t), f.head(t)
	err := restart.call(t.Context(), f.recordsFixture, f.clients, id, key)
	if err == nil || connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Error("The restart did not refuse the scan's request key with already_exists.")
	}
	if f.noticeCount(t, id) != 1 || f.head(t) != head || f.digest(t) != before {
		t.Error("The conflicting restart changed the command count, stored receipt or feed position.")
	}
}
