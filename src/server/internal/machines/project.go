package machines

import (
	"math"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	lgtmv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
)

// Project converts a durable machine record to the Machines page's contract.
func Project(machine Machine) *lgtmv1.Client {
	client := &lgtmv1.Client{
		Id:               machine.ID,
		Name:             machine.Name,
		State:            lgtmv1.ClientState_CLIENT_STATE_OFFLINE,
		HostsServer:      machine.HostsServer,
		Address:          machine.LastAddress,
		HarnessesScanned: machine.HarnessesScanned,
	}
	if machine.Approval == ApprovalWaiting {
		client.State = lgtmv1.ClientState_CLIENT_STATE_WAITING_FOR_APPROVAL
		client.Name = machine.HostName
		client.TokenId = machine.TokenID
	}
	if machine.LastSeenAt != nil {
		client.LastSeen = timestamppb.New(*machine.LastSeenAt)
	}

	var facts linkv1.Facts
	if err := proto.Unmarshal(machine.Facts, &facts); err == nil {
		client.Os = facts.GetOs()
		client.Version = facts.GetClientVersion()
		client.Workspace = facts.GetWorkspace()
		client.AgentLimit = facts.GetAgentLimit()
		client.Cores = facts.GetCores()
		const bytesPerGigabyte = 1 << 30
		memory := facts.GetMemoryBytes()
		whole := memory / bytesPerGigabyte
		if memory%bytesPerGigabyte >= bytesPerGigabyte/2 {
			whole++
		}
		client.MemoryGigabytes = uint32(min(whole, math.MaxUint32))
	}
	var harnesses linkv1.HarnessesFound
	if err := proto.Unmarshal(machine.Harnesses, &harnesses); err == nil {
		for _, harness := range harnesses.GetHarnesses() {
			client.Harnesses = append(client.Harnesses, &lgtmv1.InstalledHarness{
				Harness: harness.GetName(),
				Version: harness.Version,
			})
		}
	}
	return client
}
