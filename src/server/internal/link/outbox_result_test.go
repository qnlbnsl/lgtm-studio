package link

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log"
	"log/slog"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
)

func TestAcceptResultRules(t *testing.T) {
	f := newPresenceFixture(t, nil)
	a := f.machine(t, bytes.Repeat([]byte{1}, 32), "result station")
	b := f.machine(t, bytes.Repeat([]byte{2}, 32), "other result station")
	waiting := f.machine(t, bytes.Repeat([]byte{3}, 32), "waiting result station")
	f.approve(t, a.ID)
	// Approved names must be distinct.
	if err := f.s.Write(t.Context(), func(ctx context.Context, tx *store.Tx) error {
		return f.r.SetApproved(ctx, tx, b.ID, "Other approved machine")
	}); err != nil {
		t.Fatal("The other result station could not be approved.")
	}
	old := f.admit(t, a.ID, 1)
	other := f.admit(t, b.ID, 1)
	waitingStream := f.admit(t, waiting.ID, 1)
	future := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp() + interval '2 minutes'")
	const done = linkv1.CommandOutcome_COMMAND_OUTCOME_DONE

	id := seedResultCommand(t, f, a.ID, old.ID, "scan_harnesses", 2, true, future)
	for _, tc := range []struct {
		name      string
		machineID string
		streamID  string
		commandID string
		attempt   uint32
	}{
		{"older attempt", a.ID, old.ID, id, 1},
		{"later attempt", a.ID, old.ID, id, 3},
		{"different command identifier", a.ID, old.ID, fixtureStreamID(t), 2},
		{"different machine and stream", b.ID, other.ID, id, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			head := f.head(t)
			requireResultError(t, f, tc.machineID, tc.streamID, commandResult(tc.commandID, tc.attempt, done), nil)
			requirePendingResult(t, f, id)
			f.requireSaved(t, head)
		})
	}

	t.Run("different machine with exact attempt and stream", func(t *testing.T) {
		// This association makes the machine identifier the only mismatched predicate.
		id := seedResultCommand(t, f, a.ID, other.ID, "scan_harnesses", 2, true, future)
		head := f.head(t)
		requireResultError(t, f, b.ID, other.ID, commandResult(id, 2, done), nil)
		requirePendingResult(t, f, id)
		f.requireSaved(t, head)
	})

	current := f.admit(t, a.ID, 2)
	t.Run("new current stream before claim", func(t *testing.T) {
		head := f.head(t)
		requireResultError(t, f, a.ID, current.ID, commandResult(id, 2, done), nil)
		requirePendingResult(t, f, id)
		f.requireSaved(t, head)
	})
	t.Run("displaced stream", func(t *testing.T) {
		head := f.head(t)
		requireResultError(t, f, a.ID, old.ID, commandResult(id, 2, done), ErrNotCurrent)
		requirePendingResult(t, f, id)
		f.requireSaved(t, head)
	})
	t.Run("stream belonging to another machine", func(t *testing.T) {
		requireResultError(t, f, a.ID, other.ID, commandResult(id, 2, done), ErrNotCurrent)
		requirePendingResult(t, f, id)
	})
	t.Run("machine missing", func(t *testing.T) {
		requireResultError(t, f, fixtureStreamID(t), old.ID, commandResult(id, 2, done), ErrUnknownMachine)
		requirePendingResult(t, f, id)
	})
	t.Run("waiting current stream", func(t *testing.T) {
		id := seedResultCommand(t, f, waiting.ID, waitingStream.ID, "set_state", 1, true, future)
		head := f.head(t)
		requireResultError(t, f, waiting.ID, waitingStream.ID, nil, nil)
		requireResultError(t, f, waiting.ID, waitingStream.ID, commandResult(id, 1, done), nil)
		requirePendingResult(t, f, id)
		f.requireSaved(t, head)
	})

	t.Run("connection missing", func(t *testing.T) {
		if changed, err := f.state.Disconnect(t.Context(), waiting.ID, waitingStream.ID); err != nil || !changed {
			t.Fatal("The waiting stream could not be disconnected.")
		}
		head := f.head(t)
		requireResultError(t, f, waiting.ID, waitingStream.ID, nil, ErrNotCurrent)
		f.requireSaved(t, head)
	})

	for _, tc := range []struct {
		name   string
		result *linkv1.CommandResult
	}{
		{"nil result", nil},
		{"empty identifier", commandResult("", 1, done)},
		{"noncanonical identifier", commandResult("00000000-0000-7000-8000-00000000000A", 1, done)},
		{"zero attempt", commandResult(id, 0, done)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			head := f.head(t)
			requireResultError(t, f, a.ID, current.ID, tc.result, ErrInvalidMessage)
			requirePendingResult(t, f, id)
			f.requireSaved(t, head)
		})
	}

	t.Run("claimed but not dispatched", func(t *testing.T) {
		id := seedResultCommand(t, f, a.ID, current.ID, "scan_harnesses", 2, false, future)
		head := f.head(t)
		result := commandResult(id, 2, done)
		requireResultError(t, f, a.ID, current.ID, result, nil)
		requirePendingResult(t, f, id)
		outboxFixtureExec(t, f.obs, "UPDATE link_commands SET dispatched = true WHERE id = $1", id)
		start := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
		requireResultError(t, f, a.ID, current.ID, result, nil)
		requireFinishedResult(t, f, id, "done", start)
		f.requireSaved(t, head)
	})

	for _, tc := range []struct {
		name    string
		action  string
		outcome linkv1.CommandOutcome
		want    string
	}{
		{"done", "scan_harnesses", done, "done"},
		{"failed", "scan_harnesses", linkv1.CommandOutcome_COMMAND_OUTCOME_FAILED, "failed"},
		{"machine expiry", "scan_harnesses", linkv1.CommandOutcome_COMMAND_OUTCOME_EXPIRED, "expired"},
		{"unsupported", "restart", linkv1.CommandOutcome_COMMAND_OUTCOME_UNSUPPORTED, "unsupported"},
		{"interrupted", "restart", linkv1.CommandOutcome_COMMAND_OUTCOME_INTERRUPTED, "interrupted"},
		{"unspecified becomes failed", "scan_harnesses", linkv1.CommandOutcome_COMMAND_OUTCOME_UNSPECIFIED, "failed"},
		{"unknown becomes failed", "scan_harnesses", linkv1.CommandOutcome(97), "failed"},
		{"state expiry becomes failed", "set_state", linkv1.CommandOutcome_COMMAND_OUTCOME_EXPIRED, "failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := seedResultCommand(t, f, a.ID, current.ID, tc.action, 2, true, future)
			head := f.head(t)
			start := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
			requireResultError(t, f, a.ID, current.ID, commandResult(id, 2, tc.outcome), nil)
			requireFinishedResult(t, f, id, tc.want, start)
			f.requireSaved(t, head)
		})
	}

	t.Run("late result with reserved field and duplicate", func(t *testing.T) {
		past := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp() - interval '25 hours'")
		id := seedResultCommand(t, f, a.ID, current.ID, "scan_harnesses", 2, true, past)
		sibling := seedResultCommand(t, f, a.ID, current.ID, "scan_harnesses", 2, true, past)
		before := outboxPartBRecords(t, f)
		metadata := func() string {
			return storetest.Scalar[string](t, f.obs, `SELECT md5((to_jsonb(c) - 'payload' - 'outcome' - 'finished_at')::text)
				FROM link_commands c WHERE id = $1`, id)
		}
		savedMetadata := metadata()
		publicBefore := resultMachineProjection(t, f, a.ID)
		logs := new(handshakeLog)
		previous, output, flags := slog.Default(), log.Writer(), log.Flags()
		slog.SetDefault(slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
		t.Cleanup(func() {
			slog.SetDefault(previous)
			log.SetOutput(output)
			log.SetFlags(flags)
		})
		const marker = "reserved result field fixture"
		result := commandResult(id, 2, done)
		unknown := protowire.AppendString(protowire.AppendTag(nil, 4, protowire.BytesType), marker)
		result.ProtoReflect().SetUnknown(unknown)

		requireNoOtherChanges := func() {
			t.Helper()
			requirePendingResult(t, f, sibling)
			if outboxPartBRecords(t, f) != before || metadata() != savedMetadata {
				t.Error("The reserved result or duplicate changed other command fields, Part B records or the feed.")
			}
			publicAfter := resultMachineProjection(t, f, a.ID)
			if !bytes.Equal(publicAfter, publicBefore) || bytes.Contains(publicAfter, []byte(marker)) {
				t.Error("The reserved result or duplicate changed the Machines watch projection.")
			}
			for _, encoding := range []string{marker, hex.EncodeToString([]byte(marker)), base64.StdEncoding.EncodeToString(unknown)} {
				if logs.contains(encoding) {
					t.Error("The reserved result field reached a diagnostic.")
				}
			}
		}

		start := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
		requireResultError(t, f, a.ID, current.ID, result, nil)
		finished := requireFinishedResult(t, f, id, "done", start)
		requireNoOtherChanges()
		requireResultError(t, f, a.ID, current.ID, commandResult(id, 2, linkv1.CommandOutcome_COMMAND_OUTCOME_FAILED), nil)
		after := readCommandResult(t, f, id)
		if after.outcome != "done" || after.finished == nil || !after.finished.Equal(finished) || after.hasPayload {
			t.Error("The duplicate changed the first outcome, finish time or cleared payload.")
		}
		requireNoOtherChanges()
	})
}

func commandResult(id string, attempt uint32, outcome linkv1.CommandOutcome) *linkv1.CommandResult {
	return &linkv1.CommandResult{CommandId: id, Attempt: attempt, Outcome: outcome}
}

func requireResultError(t *testing.T, f *presenceFixture, machineID, streamID string, result *linkv1.CommandResult, want error) {
	t.Helper()
	if err := f.state.AcceptResult(t.Context(), machineID, streamID, result); !errors.Is(err, want) {
		t.Error("AcceptResult did not return the expected fence or validation result.")
	}
}

func seedResultCommand(t *testing.T, f *presenceFixture, machineID, streamID, action string, attempt int64, dispatched bool, expiry time.Time) string {
	t.Helper()
	id := fixtureStreamID(t)
	payload := []byte{}
	if action == "set_state" {
		state := &linkv1.SetState{State: &linkv1.MachineState{
			Approval: linkv1.Approval_APPROVAL_APPROVED, Name: "Approved machine", Revision: 2,
		}}
		var err error
		payload, err = proto.Marshal(state)
		if err != nil {
			t.Fatal("The state notice fixture did not encode.")
		}
	}
	outboxFixtureExec(t, f.obs, `INSERT INTO link_commands
		(id, machine_id, action, payload, created_at, expires_at, attempt, sent_stream_id, dispatched)
		VALUES ($1, $2, $3, $4, $5::timestamptz - interval '2 minutes', $5, $6, $7, $8)`,
		id, machineID, action, payload, expiry, attempt, streamID, dispatched)
	return id
}

type commandResultRow struct {
	outcome    string
	finished   *time.Time
	hasPayload bool
}

func readCommandResult(t *testing.T, f *presenceFixture, id string) commandResultRow {
	t.Helper()
	var row commandResultRow
	err := f.obs.QueryRow(t.Context(), `SELECT coalesce(outcome, ''), finished_at, payload IS NOT NULL
		FROM link_commands WHERE id = $1`, id).Scan(&row.outcome, &row.finished, &row.hasPayload)
	if err != nil {
		t.Fatal("The seeded command row could not be read.")
	}
	return row
}

func requirePendingResult(t *testing.T, f *presenceFixture, id string) {
	t.Helper()
	if row := readCommandResult(t, f, id); row.outcome != "" || row.finished != nil || !row.hasPayload {
		t.Error("A result that should be ignored changed the pending command.")
	}
}

func requireFinishedResult(t *testing.T, f *presenceFixture, id, want string, start time.Time) time.Time {
	t.Helper()
	row := readCommandResult(t, f, id)
	end := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
	if row.outcome != want || row.finished == nil || row.finished.Before(start) || row.finished.After(end) || row.hasPayload {
		t.Fatalf("The dispatched result did not store %s and a database finish time with no payload.", want)
	}
	return *row.finished
}

func resultMachineProjection(t *testing.T, f *presenceFixture, id string) []byte {
	t.Helper()
	var encoded []byte
	err := f.s.Read(t.Context(), func(ctx context.Context, tx *store.ReadTx) error {
		machine, err := f.r.Get(ctx, tx, id)
		if err != nil {
			return err
		}
		presences, err := f.state.Presence(ctx, tx, []string{id})
		if err != nil {
			return err
		}
		presence := presences[id]
		encoded, err = proto.Marshal(machines.ProjectWithPresence(machine, &presence))
		return err
	})
	if err != nil {
		t.Fatal("The Machines watch projection could not be read.")
	}
	return encoded
}

func TestResultAndReplacementFormOneLine(t *testing.T) {
	for _, tc := range []struct {
		name             string
		replacementFirst bool
	}{
		{"result then replacement", false},
		{"replacement then result", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPresenceFixture(t, nil)
			m := f.machine(t, bytes.Repeat([]byte{1}, 32), "result ordering")
			f.approve(t, m.ID)
			old := f.admit(t, m.ID, 1)
			next := fixtureStreamID(t)
			start := storetest.Scalar[time.Time](t, f.obs, "SELECT clock_timestamp()")
			id := seedResultCommand(t, f, m.ID, old.ID, "scan_harnesses", 2, true, start.Add(2*time.Minute))
			head := f.head(t)
			admit := func(ctx context.Context) error {
				replacement, err := admitFixtureRun(f.state, ctx, m.ID, next, presenceAddress)
				if err != nil || replacement.ID != next || replacement.Generation != 2 {
					return errors.New("The replacement did not acquire the next stream generation.")
				}
				return nil
			}
			result := func(ctx context.Context) error {
				err := f.state.AcceptResult(ctx, m.ID, old.ID, commandResult(id, 2, linkv1.CommandOutcome_COMMAND_OUTCOME_DONE))
				switch {
				case err == nil:
					return nil
				case errors.Is(err, ErrNotCurrent):
					return ErrNotCurrent
				default:
					return errors.New("The dispatched result returned an unexpected error.")
				}
			}
			if tc.replacementFirst {
				fingerprint := func() string {
					return storetest.Scalar[string](t, f.obs, `SELECT md5(to_jsonb(c)::text) FROM link_commands c WHERE id = $1`, id)
				}
				before := fingerprint()
				if err := checkOneLine(t, f, admit, result, false); !errors.Is(err, ErrNotCurrent) {
					t.Error("The old result did not return ErrNotCurrent after the replacement committed first.")
				}
				if fingerprint() != before {
					t.Error("The old result changed the command after the replacement committed first.")
				}
			} else {
				if err := checkOneLine(t, f, result, admit, false); err != nil {
					t.Error("Replacement failed after the current result committed first.")
				}
				requireFinishedResult(t, f, id, "done", start)
			}
			f.requireSaved(t, head, m.ID)
			if current := f.current(t, m.ID); current.StreamID != next || current.Generation != 2 {
				t.Error("The result changed the replacement stream.")
			}
		})
	}
}
