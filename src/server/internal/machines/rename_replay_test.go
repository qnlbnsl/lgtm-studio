package machines_test

import (
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
)

func TestRenameClientReplaysSavedReply(t *testing.T) {
	f := newRecordsFixture(t)
	m := f.create(t, 0x41, "rename-replay.example.test", false)
	approve := &webv1.ApproveClientRequest{Id: m.ID, Name: "Original"}
	if _, err := f.clients.ApproveClient(t.Context(), recordRequest(f, approve, "")); err != nil {
		t.Fatalf("Approving the rename fixture failed: %v", err)
	}

	const key = "rename-lost-reply"
	first := &webv1.RenameClientRequest{Id: m.ID, Name: "First rename"}
	original, err := f.clients.RenameClient(t.Context(), recordRequest(f, first, key))
	if err != nil {
		t.Fatalf("The first keyed rename failed: %v", err)
	}
	if got := f.mustGet(t, m.ID); got.Name != first.GetName() || got.StateRevision != 3 {
		t.Fatalf("The first keyed rename left name %q and revision %d, want First rename at revision 3.", got.Name, got.StateRevision)
	}

	newer := &webv1.RenameClientRequest{Id: m.ID, Name: "Newer name"}
	if _, err := f.clients.RenameClient(t.Context(), recordRequest(f, newer, "")); err != nil {
		t.Fatalf("The independent rename failed: %v", err)
	}
	beforeReplay := f.mustGet(t, m.ID)
	if beforeReplay.Name != newer.GetName() || beforeReplay.StateRevision != 4 {
		t.Fatalf("The independent rename left name %q and revision %d, want Newer name at revision 4.", beforeReplay.Name, beforeReplay.StateRevision)
	}

	replay, err := f.clients.RenameClient(t.Context(), recordRequest(f, first, key))
	if err != nil || !proto.Equal(replay.Msg, original.Msg) {
		t.Fatalf("Replaying the first keyed rename did not return its saved success reply: %v", err)
	}
	if got := f.mustGet(t, m.ID); got.Name != beforeReplay.Name || got.StateRevision != beforeReplay.StateRevision {
		t.Errorf("The rename replay left name %q and revision %d, want %q and revision %d.",
			got.Name, got.StateRevision, beforeReplay.Name, beforeReplay.StateRevision)
	}

	conflict := &webv1.RenameClientRequest{Id: m.ID, Name: "Conflicting name"}
	_, err = f.clients.RenameClient(t.Context(), recordRequest(f, conflict, key))
	requireRecordError(t, err, connect.CodeAlreadyExists, "That request key was already used for a different request.")
	if got := f.mustGet(t, m.ID); got.Name != beforeReplay.Name || got.StateRevision != beforeReplay.StateRevision {
		t.Errorf("The refused rename left name %q and revision %d, want %q and revision %d.",
			got.Name, got.StateRevision, beforeReplay.Name, beforeReplay.StateRevision)
	}
}
