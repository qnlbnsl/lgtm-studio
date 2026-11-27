package contracts_test

import (
	"bytes"
	"encoding/json"
	"testing"

	lgtmv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestWelcomeMarkIsCheckedBeforeChangingSavedHistory(t *testing.T) {
	events := readLines(t, "events.proto")
	header := normalize(commentBefore(t, events, "syntax ="))
	requireWrittenRules(t, "events.proto", header, [][2]string{
		{"the client checks the Welcome mark against saved history before forgetting an event or advancing its saved acknowledgement",
			`the client (checks|validates) the welcome(\.last_event_number| mark) against its saved (event )?history before it forgets (any |an )?event or advances its saved acknowledgement\.`},
		{"an incompatible Welcome mark leaves all state on disk unchanged, including the retained event bytes and saved acknowledgement",
			`an incompatible welcome mark leaves all (local )?state on disk unchanged, including the retained event bytes and (its )?saved acknowledgement\.`},
	})
}

func TestLatestRunIsReadyAtAdmissionAndCallsBackOnlyOnce(t *testing.T) {
	open := normalize(commentBefore(t, readLines(t, "link.proto"), "rpc Open("))
	requireWrittenRules(t, "LinkService.Open", open, [][2]string{
		{"the latest admitted run is ready at admission at or above its boundary minus one",
			`the latest admitted run is ready (at|on) admission (if|when) the committed (event )?mark is (at or (past|above)|at least) first_event_number\s*-\s*1[,.]`},
		{"the readiness callback runs only at a run's first transition to ready",
			`the server calls the readiness callback( (within|in) the transaction)? only on a run's first transition to ready\.`},
	})
}

func TestRemovalEndsReplayingStreamWithoutSendingACommand(t *testing.T) {
	open := normalize(commentBefore(t, readLines(t, "link.proto"), "rpc Open("))
	requireWrittenRules(t, "LinkService.Open", open, [][2]string{
		{"removal ends a stream that is still replaying with the existing refusal and without a command",
			`on removal, the server ends a stream that is still replaying with failed_precondition and unknown_machine without (sending )?a command\.`},
		{"best-effort removal SetState goes only to a ready stream",
			`the server may send a best-effort setstate with approval_removed only to a ready stream before it ends (that|the) stream\.`},
	})
}

func TestClientRecoveryFlagKeepsItsWatchJSONName(t *testing.T) {
	const jsonName = "eventHistoryNeedsRecovery"
	client := &lgtmv1.Client{EventHistoryNeedsRecovery: true}
	flag := messageField(t, client.ProtoReflect(), "event_history_needs_recovery")
	if got := flag.JSONName(); got != jsonName {
		t.Errorf("the recovery flag JSON name is %q, want %q", got, jsonName)
	}

	for _, tc := range []struct {
		arm      string
		watch    *lgtmv1.WatchClientsResponse
		wantJSON string
	}{
		{
			arm: "all",
			watch: &lgtmv1.WatchClientsResponse{
				Change: &lgtmv1.WatchClientsResponse_All{
					All: &lgtmv1.ClientList{Clients: []*lgtmv1.Client{client}},
				},
			},
			wantJSON: `{"all":{"clients":[{"eventHistoryNeedsRecovery":true}]}}`,
		},
		{
			arm: "saved",
			watch: &lgtmv1.WatchClientsResponse{
				Change: &lgtmv1.WatchClientsResponse_Saved{Saved: client},
			},
			wantJSON: `{"saved":{"eventHistoryNeedsRecovery":true}}`,
		},
	} {
		t.Run(tc.arm, func(t *testing.T) {
			raw, err := protojson.Marshal(tc.watch)
			if err != nil {
				t.Fatal(err)
			}
			var compact bytes.Buffer
			if err := json.Compact(&compact, raw); err != nil {
				t.Fatal(err)
			}
			if got := compact.String(); got != tc.wantJSON {
				t.Errorf("the watch sends %s, want %s", got, tc.wantJSON)
			}

			var decoded lgtmv1.WatchClientsResponse
			if err := protojson.Unmarshal(raw, &decoded); err != nil {
				t.Fatal(err)
			}
			if !proto.Equal(tc.watch, &decoded) {
				t.Errorf("the watch JSON round trip returned %v, want %v", &decoded, tc.watch)
			}
		})
	}
}
