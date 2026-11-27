package app_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/servertest"
)

func TestServerRoutesStoredCommands(t *testing.T) {
	server := servertest.Start(t, servertest.Options{Password: machinesFixturePassword})
	_, obs := fixtureLinkConfig(t, server)
	owner := server.Clients(t)
	machine := joinAppLink(t, server, owner, "waiting for a command")
	open, welcome := applicationLinkStream(t, machine, server.Link())
	if welcome.GetState().GetApproval() != linkv1.Approval_APPROVAL_WAITING || welcome.GetConnectionToken() != "" {
		t.Fatal("The machine did not start on a waiting stream.")
	}

	watch := fixtureMachinesWatch(t, owner)
	initial := fixtureMachinesSnapshot(t, watch)
	if len(initial) != 1 || initial[0].GetId() != machine.ID || initial[0].GetState() != webv1.ClientState_CLIENT_STATE_WAITING_FOR_APPROVAL {
		t.Fatal("The signed-in watch did not start with the waiting machine.")
	}
	reply, err := owner.ApproveClient(appOpenContext(t), connect.NewRequest(&webv1.ApproveClientRequest{Id: machine.ID, Name: "  Command machine  "}))
	if err != nil || reply.Msg.GetClient().GetName() != "Command machine" ||
		reply.Msg.GetClient().GetState() != webv1.ClientState_CLIENT_STATE_JOINING {
		t.Fatal("The owner route did not approve the waiting stream and return its joining record.")
	}
	if !storetest.Scalar[bool](t, obs, `SELECT count(*) = 1 FROM link_commands
		WHERE machine_id = $1 AND action = 'set_state' AND outcome IS NULL`, machine.ID) {
		t.Fatal("The application approval route did not persist one state notice.")
	}
	notice := appReceiveCommand(t, open, "The application link did not deliver the queued state notice on the original stream.")
	standing := notice.GetSetState().GetState()
	if notice.GetAttempt() != 1 || standing.GetApproval() != linkv1.Approval_APPROVAL_APPROVED ||
		standing.GetName() != "Command machine" || standing.GetRevision() != 2 {
		t.Fatal("The original stream did not receive the approved standing on its first attempt.")
	}
	appWatchMachine(t, watch, machine.ID, "The signed-in watch did not see the live machine join after approval.", func(m *webv1.Client) bool {
		return m.GetState() == webv1.ClientState_CLIENT_STATE_JOINING && !m.GetHarnessesScanned()
	})
	appSendCommandDone(t, open, notice)
	appLinkEvent(t, open, 1)
	appWatchMachine(t, watch, machine.ID, "Approved facts did not appear while the machine was still joining.", func(m *webv1.Client) bool {
		return m.GetState() == webv1.ClientState_CLIENT_STATE_JOINING && m.GetOs() == "Startup report" && m.GetCores() == 2
	})
	appCommandInventory(t, open, 2, "1.0")
	appWatchMachine(t, watch, machine.ID, "Both fresh reports did not bring the original stream online.", func(m *webv1.Client) bool {
		return m.GetState() == webv1.ClientState_CLIENT_STATE_ONLINE && m.GetOs() == "Startup report" && appHasHarnessVersion(m, "1.0")
	})
	if !storetest.Scalar[bool](t, obs, "SELECT generation = 1 FROM link_connections WHERE machine_id = $1", machine.ID) {
		t.Fatal("The approved machine changed streams before it came online.")
	}
	if _, err := owner.ScanClientHarnesses(appOpenContext(t), connect.NewRequest(&webv1.ScanClientHarnessesRequest{Id: machine.ID})); err != nil {
		t.Fatal("The signed-in scan route did not accept the online machine.")
	}
	scan := appReceiveCommand(t, open, "The signed-in scan did not reach the online machine.")
	if scan.GetScanHarnesses() == nil || scan.GetAttempt() != 1 {
		t.Fatal("The signed-in scan reached the machine with the wrong action or attempt.")
	}
	appCommandInventory(t, open, 3, "2.0")
	appSendCommandDone(t, open, scan)
	appWatchMachine(t, watch, machine.ID, "The signed-in watch did not publish the inventory reported for the scan.", func(m *webv1.Client) bool {
		return m.GetState() == webv1.ClientState_CLIENT_STATE_ONLINE && appHasHarnessVersion(m, "2.0")
	})
	appAwaitCommandDone(t, obs, machine.ID, scan, "The application link did not persist the scan result.")

	if _, err := owner.ReconnectClient(appOpenContext(t), connect.NewRequest(&webv1.ReconnectClientRequest{Id: machine.ID})); err != nil {
		t.Fatal("The application restart route did not accept the connected machine.")
	}
	first := appReceiveCommand(t, open, "The queued restart did not reach the original stream.")
	if first.GetRestart() == nil || first.GetAttempt() != 1 {
		t.Fatal("The original stream did not receive the restart at attempt one.")
	}
	appPollCommand(t, "The original restart attempt did not commit its dispatch marker.", func(ctx context.Context) bool {
		return appRestartPending(ctx, obs, machine.ID, first)
	})
	_ = watch.Close()
	server.Stop(t)
	_ = open.CloseRequest()
	requireAppOpenStatus(t, open, connect.CodeUnavailable, "The server is stopping.")
	if !appRestartPending(t.Context(), obs, machine.ID, first) {
		t.Fatal("Stopping the application changed the dispatched pending restart.")
	}

	server.Restart(t)
	reconnected, newWelcome := applicationLinkStream(t, machine, server.Link())
	if newWelcome.GetState().GetApproval() != linkv1.Approval_APPROVAL_APPROVED ||
		newWelcome.GetConnectionToken() == "" || newWelcome.GetLastEventNumber() != 3 {
		t.Fatal("The restarted application did not admit the approved machine with its report mark.")
	}
	retry := appReceiveCommand(t, reconnected, "The restarted application did not deliver the pending restart again.")
	if retry.GetRestart() == nil || retry.GetId() != first.GetId() ||
		!retry.GetExpiresAt().AsTime().Equal(first.GetExpiresAt().AsTime()) || retry.GetAttempt() != first.GetAttempt()+1 {
		t.Fatal("The retry did not use the original command and expiry with the next attempt.")
	}
	appSendCommandDone(t, reconnected, retry)
	appAwaitCommandDone(t, obs, machine.ID, retry, "The restarted application did not persist the matching restart result.")
}

func appReceiveCommand(t *testing.T, stream *appOpenStream, failure string) *linkv1.Command {
	t.Helper()
	frame, err := stream.Receive()
	cmd := frame.GetCommand()
	if err != nil || cmd.GetId() == "" || cmd.GetExpiresAt().CheckValid() != nil {
		t.Fatal(failure)
	}
	return cmd
}

func appSendCommandDone(t *testing.T, stream *appOpenStream, command *linkv1.Command) {
	t.Helper()
	sendAppOpen(t, stream, &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_CommandResult{CommandResult: &linkv1.CommandResult{
		CommandId: command.GetId(), Attempt: command.GetAttempt(), Outcome: linkv1.CommandOutcome_COMMAND_OUTCOME_DONE,
	}}})
}

func appCommandInventory(t *testing.T, stream *appOpenStream, number uint64, version string) {
	t.Helper()
	harnesses := &linkv1.HarnessesFound{Harnesses: []*linkv1.Harness{{Name: "codex", Version: &version}}}
	sendAppOpen(t, stream, &linkv1.OpenRequest{Kind: &linkv1.OpenRequest_Event{Event: &linkv1.Event{
		Number: number, Report: &linkv1.Event_HarnessesFound{HarnessesFound: harnesses},
	}}})
	frame, err := stream.Receive()
	if err != nil || frame.GetEventAck().GetThrough() != number {
		t.Fatal("The application did not acknowledge the numbered inventory report.")
	}
}

func appWatchMachine(t *testing.T, watch *connect.ServerStreamForClient[webv1.WatchClientsResponse], id, failure string, match func(*webv1.Client) bool) {
	t.Helper()
	for watch.Receive() {
		if m := watch.Msg().GetSaved(); m.GetId() == id && match(m) {
			return
		}
	}
	t.Fatal(failure)
}

func appHasHarnessVersion(machine *webv1.Client, version string) bool {
	harnesses := machine.GetHarnesses()
	return machine.GetHarnessesScanned() && len(harnesses) == 1 &&
		harnesses[0].GetHarness() == "codex" && harnesses[0].GetVersion() == version
}

func appAwaitCommandDone(t *testing.T, obs *pgx.Conn, machine string, command *linkv1.Command, failure string) {
	t.Helper()
	appPollCommand(t, failure, func(ctx context.Context) bool {
		var done bool
		err := obs.QueryRow(ctx, `SELECT EXISTS (SELECT FROM link_commands
			WHERE machine_id = $1 AND id = $2 AND attempt = $3 AND dispatched
				AND outcome = 'done' AND finished_at IS NOT NULL AND payload IS NULL)`,
			machine, command.GetId(), command.GetAttempt()).Scan(&done)
		return err == nil && done
	})
}

func appRestartPending(ctx context.Context, obs *pgx.Conn, machine string, command *linkv1.Command) bool {
	var pending bool
	err := obs.QueryRow(ctx, `SELECT EXISTS (SELECT FROM link_commands
		WHERE machine_id = $1 AND id = $2 AND attempt = $3 AND expires_at = $4 AND dispatched
			AND outcome IS NULL AND finished_at IS NULL AND payload IS NOT NULL)`,
		machine, command.GetId(), command.GetAttempt(), command.GetExpiresAt().AsTime()).Scan(&pending)
	return err == nil && pending
}

func appPollCommand(t *testing.T, failure string, observe func(context.Context) bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for !observe(ctx) {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal(failure)
		}
	}
}

func TestServerCommandSweepLifecycle(t *testing.T) {
	t.Run("startup pass", func(t *testing.T) {
		server := servertest.Start(t, servertest.Options{Password: machinesFixturePassword})
		_, obs := fixtureLinkConfig(t, server)
		action, standing := seedStoppedCommandSweep(t, server, obs)
		server.Restart(t)
		appPollCommand(t, "The application did not time out the due action on its first command sweep.", func(ctx context.Context) bool {
			var finished bool
			err := obs.QueryRow(ctx, `SELECT EXISTS (SELECT FROM link_commands
				WHERE id = $1 AND outcome = 'timed_out' AND finished_at IS NOT NULL AND payload IS NULL)`, action).Scan(&finished)
			return err == nil && finished
		})
		if !appCommandStillQueued(t, obs, standing) {
			t.Fatal("The startup sweep did not retain the pending state notice.")
		}
	})

	t.Run("shutdown cancels blocked sweep", func(t *testing.T) {
		server := servertest.Start(t, servertest.Options{Password: machinesFixturePassword})
		_, obs := fixtureLinkConfig(t, server)
		action, standing := seedStoppedCommandSweep(t, server, obs)
		_, locker := fixtureLinkConfig(t, server)
		tx, err := locker.Begin(t.Context())
		if err != nil {
			t.Fatal("The sweep blocker could not start its independent transaction.")
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		storetest.Exec(t, tx, "LOCK TABLE link_commands IN ACCESS EXCLUSIVE MODE")
		server.Restart(t)
		var waiter int32
		appPollCommand(t, "The application sweep did not reach its blocked due-row query.", func(ctx context.Context) bool {
			err := obs.QueryRow(ctx, `SELECT pid FROM pg_stat_activity
				WHERE datname = current_database() AND wait_event_type = 'Lock'
					AND query LIKE '%SELECT expires_at, machine_id, seq%'
					AND $1 = ANY (pg_blocking_pids(pid))`, int32(locker.PgConn().PID())).Scan(&waiter)
			return err == nil
		})

		stopped := storetest.Start(t, func() struct{} { server.Stop(t); return struct{}{} })
		select {
		case <-stopped.Done():
		case <-time.After(5 * time.Second):
			_ = tx.Rollback(context.Background())
			t.Fatal("Application shutdown did not cancel and join the blocked command sweep.")
		}
		if storetest.Scalar[bool](t, obs, `SELECT EXISTS (
			SELECT FROM pg_stat_activity WHERE datname = current_database() AND
				((pid = $1 AND wait_event_type = 'Lock') OR application_name = 'lgtm-server-instance'))`, waiter) {
			t.Fatal("Application shutdown did not release its blocked query and instance session.")
		}
		var bothPending bool
		err = tx.QueryRow(t.Context(), `SELECT count(*) = 2 FROM link_commands WHERE id IN ($1, $2)
			AND outcome IS NULL AND finished_at IS NULL AND payload IS NOT NULL`, action, standing).Scan(&bothPending)
		if err != nil || !bothPending {
			t.Fatal("The canceled sweep did not leave both queued commands intact.")
		}
		if tx.Rollback(t.Context()) != nil {
			t.Fatal("The sweep fixture could not release its table lock.")
		}
		server.Restart(t)
		appPollCommand(t, "The restarted application did not recover the interrupted sweep.", func(ctx context.Context) bool {
			var finished bool
			err := obs.QueryRow(ctx, `SELECT EXISTS (SELECT FROM link_commands
				WHERE id = $1 AND outcome = 'timed_out' AND finished_at IS NOT NULL AND payload IS NULL)`, action).Scan(&finished)
			return err == nil && finished
		})
		if !appCommandStillQueued(t, obs, standing) {
			t.Fatal("The restarted application lost the queued state notice.")
		}
	})
}

func seedStoppedCommandSweep(t *testing.T, server *servertest.Server, obs *pgx.Conn) (action, standing string) {
	t.Helper()
	if !storetest.Scalar[bool](t, obs, `SELECT EXISTS (SELECT FROM schema_migrations WHERE part = 'link' AND version = 3)
		AND to_regclass('link_commands') IS NOT NULL`) {
		t.Fatal("The application did not apply Link migration version 3 during startup.")
	}
	owner := server.Clients(t)
	machine := joinAppLink(t, server, owner, "stopped sweep")
	approveAppLink(t, owner, machine.ID, "Offline sweep machine")
	server.Stop(t)

	seed := func(kind string, envelope *linkv1.Command) string {
		id, err := store.NewID()
		payload, marshalErr := proto.Marshal(envelope)
		if err != nil || marshalErr != nil {
			t.Fatal("The sweep fixture could not prepare a queued row.")
		}
		_, err = obs.Exec(t.Context(), `INSERT INTO link_commands (id, machine_id, action, payload, created_at, expires_at)
			VALUES ($1, $2, $3, $4, clock_timestamp() - interval '26 hours', clock_timestamp() - interval '25 hours')`,
			id, machine.ID, kind, payload)
		if err != nil || !appCommandStillQueued(t, obs, id) {
			t.Fatal("The stopped application fixture did not save its aged pending command.")
		}
		return id
	}
	return seed("restart", &linkv1.Command{Action: &linkv1.Command_Restart{Restart: &linkv1.Restart{}}}),
		seed("set_state", &linkv1.Command{Action: &linkv1.Command_SetState{SetState: &linkv1.SetState{State: &linkv1.MachineState{
			Approval: linkv1.Approval_APPROVAL_APPROVED, Name: "Offline sweep machine", Revision: 2,
		}}}})
}

func appCommandStillQueued(t *testing.T, obs *pgx.Conn, id string) bool {
	t.Helper()
	return storetest.Scalar[bool](t, obs, `SELECT EXISTS (SELECT FROM link_commands WHERE id = $1
		AND outcome IS NULL AND finished_at IS NULL AND payload IS NOT NULL)`, id)
}

func TestServerCommandOwnerBoundary(t *testing.T) {
	server := servertest.Start(t, servertest.Options{Password: machinesFixturePassword})
	_, obs := fixtureLinkConfig(t, server)
	owner := server.Clients(t)
	waiting := joinAppLink(t, server, owner, "waiting authorization")
	online := joinAppLink(t, server, owner, "online authorization")
	const replayName = "Previously approved"
	replayKey := ownerCommandKey(t)
	replay := func(ctx context.Context, client lgtmv1connect.ClientServiceClient, headers http.Header) error {
		_, err := client.ApproveClient(ctx, ownerCommandRequest(&webv1.ApproveClientRequest{Id: online.ID, Name: replayName}, replayKey, headers))
		return err
	}
	if replay(appOpenContext(t), owner, nil) != nil || storetest.Scalar[int64](t, obs, "SELECT count(*) FROM receipts") != 1 {
		t.Fatal("The owner-boundary fixture did not save the prior authorized approval receipt.")
	}
	// Remove the setup notice before opening the stream so inventory is the only incoming report.
	storetest.Exec(t, obs, "DELETE FROM link_commands WHERE machine_id = $1 AND action = 'set_state'", online.ID)
	live, approved := applicationLinkStream(t, online, server.Link())
	if approved.GetState().GetApproval() != linkv1.Approval_APPROVAL_APPROVED || approved.GetConnectionToken() == "" {
		t.Fatal("The owner-boundary fixture did not obtain an approved machine connection token.")
	}
	appLinkEvent(t, live, 1)
	appCommandInventory(t, live, 2, "1.0")
	watch := fixtureMachinesWatch(t, owner)
	var canApprove, canAct bool
	for _, m := range fixtureMachinesSnapshot(t, watch) {
		canApprove = canApprove || (m.GetId() == waiting.ID && m.GetState() == webv1.ClientState_CLIENT_STATE_WAITING_FOR_APPROVAL)
		canAct = canAct || (m.GetId() == online.ID && m.GetState() == webv1.ClientState_CLIENT_STATE_ONLINE)
	}
	_ = watch.Close()
	if !canApprove || !canAct {
		t.Fatal("The owner-boundary fixture did not produce machines eligible for all four routes.")
	}

	address, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal("The owner-boundary fixture could not parse its listener address.")
	}
	var cookie string
	for _, entry := range server.HTTPClient.Jar.Cookies(address) {
		if entry.Name == auth.CookieName {
			cookie = entry.String()
		}
	}
	if cookie == "" {
		t.Fatal("The owner-boundary fixture did not have a live owner cookie.")
	}
	unsigned := lgtmv1connect.NewClientServiceClient(&http.Client{Transport: server.HTTPClient.Transport}, server.URL)
	approveKey, renameKey, scanKey, restartKey := ownerCommandKey(t), ownerCommandKey(t), ownerCommandKey(t), ownerCommandKey(t)
	type commandCall func(context.Context, lgtmv1connect.ClientServiceClient, http.Header) error
	routes := []struct {
		name string
		call commandCall
	}{
		{"approval", func(ctx context.Context, c lgtmv1connect.ClientServiceClient, h http.Header) error {
			_, err := c.ApproveClient(ctx, ownerCommandRequest(&webv1.ApproveClientRequest{Id: waiting.ID, Name: "New approval"}, approveKey, h))
			return err
		}},
		{"rename", func(ctx context.Context, c lgtmv1connect.ClientServiceClient, h http.Header) error {
			_, err := c.RenameClient(ctx, ownerCommandRequest(&webv1.RenameClientRequest{Id: online.ID, Name: "Changed name"}, renameKey, h))
			return err
		}},
		{"scan", func(ctx context.Context, c lgtmv1connect.ClientServiceClient, h http.Header) error {
			_, err := c.ScanClientHarnesses(ctx, ownerCommandRequest(&webv1.ScanClientHarnessesRequest{Id: online.ID}, scanKey, h))
			return err
		}},
		{"restart", func(ctx context.Context, c lgtmv1connect.ClientServiceClient, h http.Header) error {
			_, err := c.ReconnectClient(ctx, ownerCommandRequest(&webv1.ReconnectClientRequest{Id: online.ID}, restartKey, h))
			return err
		}},
		{"keyed approval replay", replay},
	}

	before := appOwnerCommandDigest(t, obs)
	modes := []struct {
		name    string
		headers http.Header
		want    connect.Code
	}{
		{"anonymous", nil, connect.CodeUnauthenticated},
		{"machine token", http.Header{"Lgtm-Link-Token": {approved.GetConnectionToken()}}, connect.CodeUnauthenticated},
		{"foreign origin", http.Header{"Cookie": {cookie}, "Origin": {"http://10.2.3.4"}}, connect.CodePermissionDenied},
		{"signed out cookie", http.Header{"Cookie": {cookie}}, connect.CodeUnauthenticated},
	}
	for _, mode := range modes {
		if mode.name == "signed out cookie" {
			session := lgtmv1connect.NewSessionServiceClient(server.HTTPClient, server.URL)
			if _, err := session.SignOut(appOpenContext(t), connect.NewRequest(&webv1.SignOutRequest{})); err != nil {
				t.Fatal("The fixture could not revoke the original owner cookie.")
			}
		}
		t.Run(mode.name, func(t *testing.T) {
			beforeDenial := appOwnerCommandDigest(t, obs)
			for _, route := range routes {
				if err := route.call(appOpenContext(t), unsigned, mode.headers); err == nil || connect.CodeOf(err) != mode.want {
					t.Errorf("The %s route did not deny %s with %s.", route.name, mode.name, mode.want)
				}
			}
			if appOwnerCommandDigest(t, obs) != beforeDenial {
				t.Fatal("Denied calls changed a command, receipt, registry record, or feed reference.")
			}
		})
	}
	t.Run("signed-in owner", func(t *testing.T) {
		owner := server.Clients(t)
		if err := routes[2].call(appOpenContext(t), owner, nil); err != nil {
			t.Fatal("The application did not queue the authorized owner's scan.")
		}
		if cmd := appReceiveCommand(t, live, "The authorized owner's scan did not reach the machine."); cmd.GetScanHarnesses() == nil || cmd.GetAttempt() != 1 {
			t.Fatal("The authorized owner did not send a scan at attempt one.")
		}
		if err := routes[2].call(appOpenContext(t), owner, nil); err != nil {
			t.Fatal("The application did not replay the authorized owner's request.")
		}
		if !storetest.Scalar[bool](t, obs, `SELECT
			(SELECT count(*) FROM link_commands) = 1 AND
			(SELECT count(*) FROM link_commands WHERE machine_id = $1 AND action = 'scan_harnesses') = 1 AND
			(SELECT count(*) FROM receipts) = 2 AND
			(SELECT count(*) FROM feed WHERE topic = 'machine_outbox' AND key = $1) = $2 + 1`,
			online.ID, before.outboxWakes) {
			t.Fatal("The authorized scan and its replay did not leave exactly one command, receipt, and wake.")
		}
	})
}

func ownerCommandKey(t *testing.T) string {
	t.Helper()
	key, err := store.NewID()
	if err != nil {
		t.Fatal("The owner-boundary fixture could not prepare a request key.")
	}
	return key
}

func ownerCommandRequest[T any](body *T, key string, headers http.Header) *connect.Request[T] {
	req := connect.NewRequest(body)
	req.Header().Set(store.RequestKeyHeader, key)
	for name, values := range headers {
		for _, value := range values {
			req.Header().Add(name, value)
		}
	}
	return req
}

type ownerCommandDigest struct {
	digest      string
	outboxWakes int64
}

func appOwnerCommandDigest(t *testing.T, obs *pgx.Conn) ownerCommandDigest {
	t.Helper()
	var result ownerCommandDigest
	err := obs.QueryRow(t.Context(), `SELECT md5(
		(SELECT coalesce(jsonb_agg(to_jsonb(c) ORDER BY seq)::text, '') FROM link_commands c) ||
		(SELECT coalesce(jsonb_agg(to_jsonb(r) ORDER BY principal_hash, request_key)::text, '') FROM receipts r) ||
		(SELECT coalesce(jsonb_agg(to_jsonb(m) ORDER BY id)::text, '') FROM machines m) ||
		(SELECT coalesce(jsonb_agg(to_jsonb(f) ORDER BY position)::text, '') FROM feed f)),
		(SELECT count(*) FROM feed WHERE topic = 'machine_outbox')`).Scan(&result.digest, &result.outboxWakes)
	if err != nil {
		t.Fatal("The owner-boundary fixture could not observe its committed state.")
	}
	return result
}
