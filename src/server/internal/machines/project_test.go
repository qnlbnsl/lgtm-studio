package machines_test

import (
	"context"
	"math"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	lgtmv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
)

func TestProjectClient(t *testing.T) {
	e := newRegEnv(t)
	waiting := e.create(t, 0x21, "waiting-box", true)
	t.Run("waiting", func(t *testing.T) {
		want := &lgtmv1.Client{
			Id: waiting.ID, Name: "waiting-box", TokenId: "token-waiting-box", HostsServer: true,
			State: lgtmv1.ClientState_CLIENT_STATE_WAITING_FOR_APPROVAL,
		}
		if got := machines.Project(e.mustGet(t, waiting.ID)); !proto.Equal(got, want) {
			t.Fatalf("Project returned %v, want waiting machine %v.", got, want)
		}
	})
	if err := e.write(t, func(ctx context.Context, tx *store.Tx) error {
		return e.reg.SetApproved(ctx, tx, waiting.ID, "Approved Box")
	}); err != nil {
		t.Fatalf("Approving the fixture machine failed: %v", err)
	}
	t.Run("approved without presence", func(t *testing.T) {
		want := &lgtmv1.Client{
			Id: waiting.ID, Name: "Approved Box", HostsServer: true,
			State: lgtmv1.ClientState_CLIENT_STATE_OFFLINE,
		}
		if got := machines.Project(e.mustGet(t, waiting.ID)); !proto.Equal(got, want) {
			t.Fatalf("Project returned %v, want offline machine %v.", got, want)
		}
	})

	version := "1.4.2"
	harnesses, err := proto.Marshal(&linkv1.HarnessesFound{Harnesses: []*linkv1.Harness{
		{Name: "codex", Version: &version}, {Name: "claude"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	lastSeen := time.Date(2026, time.November, 26, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		bytes  uint64
		wantGB uint32
	}{
		{"round half up", 1610612736, 2},
		{"round below half down", 429496729, 0},
		{"cap the largest value", math.MaxUint64, math.MaxUint32},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts, err := proto.Marshal(&linkv1.Facts{
				Os: "Linux", Cores: 8, MemoryBytes: tc.bytes, ClientVersion: "0.3.1", Workspace: "workspace", AgentLimit: 3,
			})
			if err != nil {
				t.Fatal(err)
			}
			_, err = e.obs.Exec(t.Context(), `
UPDATE machines SET facts = $2, harnesses = $3, harnesses_scanned = true,
    last_seen_at = $4, last_address = $5 WHERE id = $1`, waiting.ID, facts, harnesses, lastSeen, "192.0.2.14")
			if err != nil {
				t.Fatalf("Setting the fixture's reported facts failed: %v", err)
			}
			want := &lgtmv1.Client{
				Id: waiting.ID, Name: "Approved Box", HostsServer: true,
				State: lgtmv1.ClientState_CLIENT_STATE_OFFLINE, Address: "192.0.2.14", LastSeen: timestamppb.New(lastSeen),
				Os: "Linux", Cores: 8, MemoryGigabytes: tc.wantGB, Version: "0.3.1", Workspace: "workspace", AgentLimit: 3,
				HarnessesScanned: true, Harnesses: []*lgtmv1.InstalledHarness{
					{Harness: "codex", Version: &version}, {Harness: "claude"},
				},
			}
			if got := machines.Project(e.mustGet(t, waiting.ID)); !proto.Equal(got, want) {
				t.Fatalf("Project returned %v, want the offline machine with its stored facts %v.", got, want)
			}
		})
	}
}
