package linktest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/qnlbnsl/lgtm-studio/src/client/clienttest"
	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/server/servertest"
)

const fixturePassword = "combined fixture password"

func TestLinkJoinReplay(t *testing.T) {
	requirePostgres(t)
	server := servertest.Start(t, servertest.Options{Password: fixturePassword})
	page := server.Clients(t)
	r := startRelay(t, server.Link())
	r.dropJoin.Store(true)
	logs := new(diagnostics)
	opts := newClient(t, logs)
	token := issueToken(t, page, r.address)

	ctx, cancel := context.WithTimeout(t.Context(), 9*time.Second)
	defer cancel()
	watch, err := page.WatchClients(ctx, connect.NewRequest(&webv1.WatchClientsRequest{}))
	if err != nil {
		t.Fatal("The watch could not open before enrollment.")
	}
	defer watch.Close()
	if !watch.Receive() || watch.Msg().GetAll() == nil || len(watch.Msg().GetAll().GetClients()) != 0 {
		t.Fatal("The open watch did not begin with an empty snapshot.")
	}

	if clienttest.Join(ctx, opts, r.address, token) != nil {
		t.Fatalf("The client did not retry the lost committed reply successfully (%s).", logs.joinFailureCode())
	}
	first := receive(t, r.joins, "the committed join whose reply was dropped")
	second := receive(t, r.joins, "the committed join retry")
	select {
	case <-r.joins:
		t.Fatal("The client reached the server more than twice for one enrollment.")
	default:
	}
	saved := readIdentity(t, opts)
	if first != second || saved.MachineID != first.id || sha256.Sum256(saved.ClientPublicKey) != first.keyDigest {
		t.Fatal("The retry did not replay the committed machine and enroll with the original key.")
	}
	if !bytes.Equal(saved.ServerPublicKey, server.SigningPublicKey) {
		t.Fatal("The identity did not save the fixture's signing key.")
	}
	keyFile, err := os.Lstat(filepath.Join(opts.StateDir, "machine.key"))
	if err != nil || !keyFile.Mode().IsRegular() || keyFile.Mode().Perm() != 0o600 {
		t.Fatal("The enrolled folder did not hold a private machine key.")
	}
	if !watch.Receive() {
		t.Fatal("The open watch did not receive the committed waiting row.")
	}
	row := watch.Msg().GetSaved()
	if row.GetId() != first.id || row.GetState() != webv1.ClientState_CLIENT_STATE_WAITING_FOR_APPROVAL {
		t.Fatal("The open watch did not project the enrolled identity as waiting.")
	}
	rows, ok := snapshot(t.Context(), page)
	if !ok || len(rows) != 1 || rows[0].GetId() != first.id || rows[0].GetState() != row.GetState() {
		t.Fatal("A new watch did not show exactly one waiting row after the replay.")
	}
	db := openObserver(t, server)
	if scalar[int64](t, db, "SELECT count(*) FROM machines") != 1 {
		t.Fatal("The replay created more than one durable machine row.")
	}
	if scalar[int64](t, db, "SELECT count(*) FROM feed WHERE key=$1 AND change='saved'", first.id) != 1 {
		t.Fatal("The replay published more than one waiting-row change.")
	}
}

func TestLinkPresenceAcrossRestart(t *testing.T) {
	requirePostgres(t)
	server := servertest.Start(t, servertest.Options{Password: fixturePassword})
	page := server.Clients(t)
	opts := newClient(t, new(diagnostics))
	id := join(t, page, opts, server.URL)
	approve(t, page, id, "Presence fixture")

	run := startClient(t, opts)
	first := waitForClient(t, page, id, webv1.ClientState_CLIENT_STATE_ONLINE)
	if first.GetConnectedAt() == nil {
		t.Fatal("The first online row had no connection time.")
	}
	run.stop(t)
	offline := waitForClient(t, page, id, webv1.ClientState_CLIENT_STATE_OFFLINE)
	if offline.GetConnectedAt() != nil {
		t.Fatal("The offline row still carried a current connection time.")
	}

	run = startClient(t, opts)
	before := waitForClient(t, page, id, webv1.ClientState_CLIENT_STATE_ONLINE)
	db := openObserver(t, server)
	generation := scalar[int64](t, db, "SELECT generation FROM link_connections WHERE machine_id=$1", id)
	signingKey := bytes.Clone(server.SigningPublicKey)
	server.Restart(t)
	page = server.Clients(t)
	after := waitForClient(t, page, id, webv1.ClientState_CLIENT_STATE_ONLINE)
	newGeneration := scalar[int64](t, db, "SELECT generation FROM link_connections WHERE machine_id=$1", id)
	if newGeneration <= generation || after.GetConnectedAt() == nil ||
		!after.GetConnectedAt().AsTime().After(before.GetConnectedAt().AsTime()) ||
		after.GetName() != before.GetName() || !bytes.Equal(server.SigningPublicKey, signingKey) {
		t.Fatal("Restart did not keep the identity and give it a newer current stream.")
	}
	if scalar[int64](t, db, "SELECT count(*) FROM machines") != 1 {
		t.Fatal("Reconnect added a second machine identity.")
	}
	run.stop(t)
}

func TestLinkKeysAndRemoval(t *testing.T) {
	requirePostgres(t)
	server := servertest.Start(t, servertest.Options{Password: fixturePassword})
	page := server.Clients(t)
	r := startRelay(t, server.Link())

	removedLog := new(diagnostics)
	removed := newClient(t, removedLog)
	id := join(t, page, removed, r.address)
	approve(t, page, id, "Removal fixture")
	run := startClient(t, removed)
	live := receive(t, r.legs, "the approved client's first connection")
	waitForClient(t, page, id, webv1.ClientState_CLIENT_STATE_ONLINE)
	run.stop(t)
	receive(t, live.done, "the first link to close")
	remove(t, page, id)

	run = startClient(t, removed)
	removedLeg := receive(t, r.legs, "the removed machine's connection attempt")
	receive(t, removedLeg.done, "the removed machine's terminal reply")
	run.expectCleanExit(t)
	removedTrace := removedLeg.observe()
	if removedTrace.challenges != 1 || !removedTrace.unknownMachine || removedTrace.welcomed ||
		!removedLog.has("link_refused", "unknown_machine") {
		t.Fatal("The removed machine did not receive and record the authenticated unknown_machine refusal.")
	}
	if rows, ok := snapshot(t.Context(), page); !ok || len(rows) != 0 {
		t.Fatal("The removed identity remained in the website watch.")
	}

	wrongKeyLog := new(diagnostics)
	wrongKey := newClient(t, wrongKeyLog)
	id = join(t, page, wrongKey, r.address)
	approve(t, page, id, "Wrong-key fixture")
	run = startClient(t, wrongKey)
	working := receive(t, r.legs, "the machine's connection before changing its saved key")
	waitForClient(t, page, id, webv1.ClientState_CLIENT_STATE_ONLINE)
	run.stop(t)
	receive(t, working.done, "the machine's working link to close")
	waitForClient(t, page, id, webv1.ClientState_CLIENT_STATE_OFFLINE)

	replaceSavedServerKey(t, wrongKey)
	run = startClient(t, wrongKey)
	for range 2 {
		wrong := receive(t, r.legs, "a connection attempt with the wrong saved key")
		receive(t, wrong.done, "the wrong-key link to close")
		trace := wrong.observe()
		if trace.challenges != 1 || trace.proofs != 0 || trace.welcomed {
			t.Fatal("The wrong-key attempt did not reject the challenge with zero Proof frames.")
		}
	}
	if !wrongKeyLog.has("link_refused", "server_key_mismatch") {
		t.Fatal("The wrong-key client did not record server_key_mismatch.")
	}
	run.stop(t)
}

func replaceSavedServerKey(t *testing.T, opts clienttest.Options) {
	t.Helper()
	identity := readIdentity(t, opts)
	if len(identity.ClientPublicKey) == 0 || bytes.Equal(identity.ClientPublicKey, identity.ServerPublicKey) {
		t.Fatal("The wrong-key fixture did not have distinct signing keys.")
	}
	path := filepath.Join(opts.StateDir, "identity.json")
	body, err := os.ReadFile(path)
	var fields map[string]json.RawMessage
	if err != nil || json.Unmarshal(body, &fields) != nil {
		t.Fatal("Could not load the fixture identity to replace its saved pin.")
	}
	fields["serverPublicKey"] = fields["clientPublicKey"]
	body, err = json.Marshal(fields)
	if err != nil || os.WriteFile(path, body, 0o600) != nil {
		t.Fatal("Could not replace the saved server key.")
	}
	saved := readIdentity(t, opts)
	if !bytes.Equal(saved.ServerPublicKey, identity.ClientPublicKey) {
		t.Fatal("The replacement key was not saved correctly.")
	}
}

func TestLinkRemovalNoticeAndRefusal(t *testing.T) {
	requirePostgres(t)
	server := servertest.Start(t, servertest.Options{Password: fixturePassword})
	page := server.Clients(t)
	r := startRelay(t, server.Link())
	logs := new(diagnostics)
	opts := newClient(t, logs)
	id := join(t, page, opts, r.address)
	approve(t, page, id, "Notice fixture")
	run := startClient(t, opts)
	first := receive(t, r.legs, "the connection before a removal notice")
	waitForClient(t, page, id, webv1.ClientState_CLIENT_STATE_ONLINE)
	waitFor(t, 5*time.Second, "a heartbeat before the notice", func() bool {
		return first.observe().heartbeats > 0
	})

	sendRemovalNotice(t, first, "0195977c-0000-7000-8000-000000000101", 1)
	before := first.observe()
	waitFor(t, 15*time.Second, "a subsequent heartbeat on the noticed connection", func() bool {
		current := first.observe()
		return current.heartbeats > before.heartbeats && current.emptyHeartbeats > before.emptyHeartbeats
	})
	sendRemovalNotice(t, first, "0195977c-0000-7000-8000-000000000102", 2)
	first.cut()
	second := receive(t, r.legs, "the fresh dial after an ordinary disconnect")
	waitFor(t, 10*time.Second, "an approved Welcome on the fresh dial", func() bool {
		return second.observe().approval == linkv1.Approval_APPROVAL_APPROVED
	})

	sendRemovalNotice(t, second, "0195977c-0000-7000-8000-000000000103", 3)
	remove(t, page, id)
	run.expectCleanExit(t)
	receive(t, second.done, "the real removal to close the current stream")
	ending := second.observe()
	verified := ending.challenges == 1 && ending.unknownMachine
	if !verified {
		// A final refusal may follow a new Challenge on the next dial.
		select {
		case next := <-r.legs:
			receive(t, next.done, "the fresh challenge and removal refusal")
			trace := next.observe()
			verified = trace.challenges == 1 && trace.unknownMachine && !trace.welcomed
		default:
		}
	}
	if !verified || !logs.has("link_refused", "unknown_machine") {
		t.Fatal("Only the verified removal refusal should end the client run.")
	}
}

func sendRemovalNotice(t *testing.T, leg *relayLeg, id string, bump uint64) {
	t.Helper()
	trace := leg.observe()
	if !trace.welcomed || trace.approval != linkv1.Approval_APPROVAL_APPROVED {
		t.Fatal("The removal notice did not start from an approved Welcome.")
	}
	command := &linkv1.Command{
		Id: id, Attempt: 1, ExpiresAt: timestamppb.New(time.Now().Add(time.Minute)),
		Action: &linkv1.Command_SetState{SetState: &linkv1.SetState{
			State: &linkv1.MachineState{Approval: linkv1.Approval_APPROVAL_REMOVED, Revision: trace.revision + bump},
		}},
	}
	select {
	case leg.inject <- command:
	case <-leg.done:
		t.Fatal("The stream closed before the removal notice could be delivered.")
	case <-time.After(5 * time.Second):
		t.Fatal("The stream did not accept the removal notice.")
	}
	want := noticeResult{id: id, attempt: 1, outcome: linkv1.CommandOutcome_COMMAND_OUTCOME_DONE}
	waitFor(t, 5*time.Second, "a done result for the removal notice", func() bool {
		for _, result := range leg.observe().results {
			if result.id == want.id && result.attempt == want.attempt && result.outcome == want.outcome {
				return true
			}
		}
		return false
	})
}
