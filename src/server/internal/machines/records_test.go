package machines_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	linkv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"
	linkv1connect "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1/linkv1connect"
	webv1 "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"
	"github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1/lgtmv1connect"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/api"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/auth"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/link"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest/migrated"
)

const (
	recordsIssuerURL = "http://127.0.0.1:8787"
	unknownRecordID  = "00000000-0000-7000-8000-000000000001"
)

type recordsFixture struct {
	*regEnv
	tokens  *machines.Tokens
	clients lgtmv1connect.ClientServiceClient
	link    linkv1connect.LinkServiceClient
	http    *http.Client
	url     string
	cookie  *http.Cookie
}

func newRecordsFixture(t *testing.T) *recordsFixture {
	t.Helper()
	s := migrated.NewStore(t, "machines_records", machines.Migrations(), auth.Migrations())
	reg := machines.NewRegistry()
	key := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	tokens := machines.NewTokens(s, reg, machines.Issuer{PublicURL: recordsIssuerURL, ServerKey: key})
	password, err := auth.NewPassword("records fixture password")
	if err != nil {
		t.Fatal(err)
	}
	sessions := auth.NewSessions(s, password)
	token, err := sessions.Issue(t.Context())
	if err != nil {
		t.Fatalf("Issuing the record fixture's session failed: %v", err)
	}
	origins, err := auth.NewOriginPolicy(recordsIssuerURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	router := api.NewHandler(api.Options{
		Origins: origins, Sessions: sessions,
		Clients: machines.NewService(s, sessions, reg, tokens),
		Link:    link.NewService(s, tokens, key),
	})
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	server := httptest.NewUnstartedServer(router)
	server.Config.Protocols = protocols
	server.Start()
	t.Cleanup(server.Close)
	client := &http.Client{Transport: &http.Transport{Protocols: protocols}, Timeout: 15 * time.Second}
	t.Cleanup(client.CloseIdleConnections)
	return &recordsFixture{
		regEnv: &regEnv{s: s, reg: reg}, tokens: tokens, http: client, url: server.URL,
		clients: lgtmv1connect.NewClientServiceClient(client, server.URL),
		link:    linkv1connect.NewLinkServiceClient(client, server.URL),
		cookie:  &http.Cookie{Name: auth.CookieName, Value: token},
	}
}

func recordRequest[T any](f *recordsFixture, msg *T, key string) *connect.Request[T] {
	req := connect.NewRequest(msg)
	req.Header().Set("Cookie", f.cookie.String())
	if key != "" {
		req.Header().Set(store.RequestKeyHeader, key)
	}
	return req
}

func requireRecordError(t *testing.T, err error, code connect.Code, sentence string) {
	t.Helper()
	var callErr *connect.Error
	if !errors.As(err, &callErr) || callErr.Code() != code || callErr.Message() != sentence {
		t.Fatalf("The call returned %v, want %s with %q.", err, code, sentence)
	}
}

func recordScalar[T any](t *testing.T, f *recordsFixture, statement string, args ...any) T {
	t.Helper()
	var value T
	if err := f.s.Read(t.Context(), func(ctx context.Context, tx *store.ReadTx) error {
		return tx.QueryRow(ctx, statement, args...).Scan(&value)
	}); err != nil {
		t.Fatalf("Reading a fixture value failed: %v", err)
	}
	return value
}

func (f *recordsFixture) issue(t *testing.T, hostsServer bool) *linkv1.JoinToken {
	t.Helper()
	token, err := f.tokens.Create(t.Context(), machines.TokenOptions{HostsServer: hostsServer})
	if err != nil {
		t.Fatalf("Issuing a fixture token failed: %v", err)
	}
	decoded, err := machines.DecodeToken(token.Text)
	if err != nil {
		t.Fatal("The fixture token could not be decoded.")
	}
	return decoded
}

func (f *recordsFixture) join(t *testing.T, token *linkv1.JoinToken, key byte) string {
	t.Helper()
	reply, err := f.link.Join(t.Context(), connect.NewRequest(&linkv1.JoinRequest{
		TokenId: token.GetTokenId(), TokenSecret: token.GetSecret(),
		MachinePublicKey: bytes.Repeat([]byte{key}, ed25519.PublicKeySize), HostName: "joined.example.test",
	}))
	if err != nil || reply.Msg.GetMachineId() == "" {
		t.Fatalf("Joining the fixture machine did not produce an identifier: %v", err)
	}
	return reply.Msg.GetMachineId()
}

func (f *recordsFixture) watch(t *testing.T) *connect.ServerStreamForClient[webv1.WatchClientsResponse] {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)
	stream, err := f.clients.WatchClients(ctx, recordRequest(f, &webv1.WatchClientsRequest{}, ""))
	if err != nil {
		t.Fatalf("Opening the record watch failed: %v", err)
	}
	t.Cleanup(func() { _ = stream.Close() })
	if !stream.Receive() || stream.Msg().GetAll() == nil {
		t.Fatalf("The record watch did not start with a snapshot: %v", stream.Err())
	}
	return stream
}

func requireRecordRemoval(t *testing.T, stream *connect.ServerStreamForClient[webv1.WatchClientsResponse], id string) {
	t.Helper()
	if !stream.Receive() {
		t.Fatalf("The record watch ended before the removal: %v", stream.Err())
	}
	if got := stream.Msg().GetRemovedId(); got != id {
		t.Errorf("The watch removed %q, want %q.", got, id)
	}
}

func TestCreateClientToken(t *testing.T) {
	t.Run("request key is refused before issuance", func(t *testing.T) {
		f := newRecordsFixture(t)
		for _, key := range []string{"record-key", ""} {
			req := recordRequest(f, &webv1.CreateClientTokenRequest{}, "")
			req.Header().Set(store.RequestKeyHeader, key)
			_, err := f.clients.CreateClientToken(t.Context(), req)
			requireRecordError(t, err, connect.CodeInvalidArgument, "This call does not accept a request key.")
		}
		if count := recordScalar[int64](t, f, "SELECT count(*) FROM client_join_tokens"); count != 0 {
			t.Errorf("A rejected request left %d token rows.", count)
		}
	})

	t.Run("reply and join", func(t *testing.T) {
		f := newRecordsFixture(t)
		before := recordScalar[time.Time](t, f, "SELECT clock_timestamp()")
		reply, err := f.clients.CreateClientToken(t.Context(), recordRequest(f, &webv1.CreateClientTokenRequest{}, ""))
		if err != nil {
			t.Fatalf("Creating a website token failed: %v", err)
		}
		after := recordScalar[time.Time](t, f, "SELECT clock_timestamp()")
		token, err := machines.DecodeToken(reply.Msg.GetToken())
		if err != nil {
			t.Fatal("The website token did not decode.")
		}
		if token.GetTokenId() == "" || token.GetTokenId() != reply.Msg.GetTokenId() ||
			token.GetAddress() != recordsIssuerURL || token.GetAddress() != reply.Msg.GetAddress() {
			t.Error("The decoded token identifier or address did not match the reply.")
		}
		created := recordScalar[time.Time](t, f, "SELECT created_at FROM client_join_tokens WHERE id = $1", token.GetTokenId())
		expires := reply.Msg.GetExpiresAt()
		if expires == nil || !expires.IsValid() || !expires.AsTime().Equal(created.Add(15*time.Minute)) ||
			created.Before(before.Add(-2*time.Second)) || created.After(after.Add(2*time.Second)) {
			t.Error("The reply did not expire 15 minutes after its token was created during the call.")
		}
		if got := reply.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("The token reply has Cache-Control %q, want no-store.", got)
		}
		id := f.join(t, token, 0x35)
		if f.mustGet(t, id).HostsServer {
			t.Error("A token issued by the website marked the joined machine as hosting the server.")
		}
	})
}

func TestRevokeClientToken(t *testing.T) {
	t.Run("unused and unknown tokens", func(t *testing.T) {
		f := newRecordsFixture(t)
		token := f.issue(t, false)
		for _, id := range []string{unknownRecordID, token.GetTokenId()} {
			_, err := f.clients.RevokeClientToken(t.Context(), recordRequest(f, &webv1.RevokeClientTokenRequest{TokenId: id}, ""))
			if err != nil {
				t.Fatalf("Revoking a token failed: %v", err)
			}
		}
		_, err := f.tokens.Redeem(t.Context(), machines.Redemption{
			TokenID: token.GetTokenId(), Secret: token.GetSecret(),
			PublicKey: bytes.Repeat([]byte{0x31}, ed25519.PublicKeySize), HostName: "revoked.example.test",
		})
		if reason, ok := machines.RefusalOf(err); !ok || reason != "token_revoked" {
			t.Errorf("Redeeming a revoked token returned %v, want token_revoked.", err)
		}
	})

	t.Run("used by a waiting machine", func(t *testing.T) {
		f := newRecordsFixture(t)
		token := f.issue(t, false)
		id := f.join(t, token, 0x32)
		stream := f.watch(t)
		_, err := f.clients.RevokeClientToken(t.Context(), recordRequest(f, &webv1.RevokeClientTokenRequest{
			TokenId: token.GetTokenId(),
		}, "revoke-waiting"))
		if err != nil {
			t.Fatalf("Revoking a waiting machine's token failed: %v", err)
		}
		if _, err := f.get(t, id); !errors.Is(err, machines.ErrNoMachine) {
			t.Errorf("The waiting machine survived token revocation: %v", err)
		}
		requireRecordRemoval(t, stream, id)
	})

	t.Run("approval holds the machine lock", func(t *testing.T) {
		f := newRecordsFixture(t)
		token := f.issue(t, false)
		id := f.join(t, token, 0x33)
		var blocker int32
		release, approval := holdWrite(t, f.s, func(ctx context.Context, tx *store.Tx) error {
			if err := tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&blocker); err != nil {
				return err
			}
			return f.reg.SetApproved(ctx, tx, id, "Box")
		})
		defer release()

		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()
		revocation := storetest.Start(t, func() error {
			_, err := f.clients.RevokeClientToken(ctx, recordRequest(f, &webv1.RevokeClientTokenRequest{TokenId: token.GetTokenId()}, ""))
			return err
		})
		storetest.Poll(t, "the revocation waiting for the approval's row lock", func() bool {
			select {
			case <-revocation.Done():
				t.Fatalf("Revocation returned before the approval committed: %v", revocation.Result())
			default:
			}
			// The observer gets its own PostgreSQL connection while the two writes are open.
			return recordScalar[bool](t, f, `
SELECT EXISTS (
	SELECT FROM pg_stat_activity
	WHERE datname = current_database() AND wait_event_type = 'Lock'
	AND $1 = ANY (pg_blocking_pids(pid))
)`, blocker)
		})
		release()
		if err := approval.Wait(t); err != nil {
			t.Fatalf("Committing approval failed: %v", err)
		}
		if err := revocation.Wait(t); err != nil {
			t.Fatalf("Revocation after approval failed: %v", err)
		}
		m := f.mustGet(t, id)
		if m.Approval != machines.ApprovalApproved || m.Name != "Box" || m.StateRevision != 2 {
			t.Error("The approval that committed first did not survive revocation.")
		}
		if !recordScalar[bool](t, f, "SELECT revoked_at IS NOT NULL FROM client_join_tokens WHERE id = $1", token.GetTokenId()) {
			t.Error("The token remained live after revocation.")
		}
	})
}

func TestRemoveClient(t *testing.T) {
	f := newRecordsFixture(t)
	waiting := f.create(t, 0x41, "waiting.example.test", false)
	hostsServer := f.join(t, f.issue(t, true), 0x42)
	if !f.mustGet(t, hostsServer).HostsServer {
		t.Fatal("The fixture machine did not receive the hosts-server mark.")
	}
	if err := f.write(t, func(ctx context.Context, tx *store.Tx) error {
		return f.reg.SetApproved(ctx, tx, hostsServer, "Server")
	}); err != nil {
		t.Fatalf("Approving the machine that hosts the server failed: %v", err)
	}
	stream := f.watch(t)

	for range 2 {
		_, err := f.clients.RemoveClient(t.Context(), recordRequest(f, &webv1.RemoveClientRequest{Id: waiting.ID}, "remove-waiting"))
		if err != nil {
			t.Fatalf("Removing a waiting machine with the same key failed: %v", err)
		}
	}
	requireRecordRemoval(t, stream, waiting.ID)
	if count := recordScalar[int64](t, f, "SELECT count(*) FROM feed WHERE topic = 'machines' AND change = 'removed' AND key = $1", waiting.ID); count != 1 {
		t.Errorf("Replaying a removal left %d removal events, want 1.", count)
	}
	_, err := f.clients.RemoveClient(t.Context(), recordRequest(f, &webv1.RemoveClientRequest{Id: hostsServer}, ""))
	if err != nil {
		t.Fatalf("Removing the machine that hosts the server failed: %v", err)
	}
	requireRecordRemoval(t, stream, hostsServer)
	for _, id := range []string{waiting.ID, hostsServer} {
		if _, err := f.get(t, id); !errors.Is(err, machines.ErrNoMachine) {
			t.Errorf("The removed machine %s remained in the registry: %v", id, err)
		}
	}
	_, err = f.clients.RemoveClient(t.Context(), recordRequest(f, &webv1.RemoveClientRequest{Id: unknownRecordID}, ""))
	requireRecordError(t, err, connect.CodeNotFound, "No machine has that id.")
}

func TestApproveClient(t *testing.T) {
	f := newRecordsFixture(t)
	waiting := f.create(t, 0x41, "waiting.example.test", false)
	other := f.create(t, 0x42, "other.example.test", false)

	t.Run("approval is offline and writes only the machine", func(t *testing.T) {
		beforeTables := recordTables(t, f, "machines", "feed")
		beforeFeed := recordScalar[int64](t, f, "SELECT COALESCE(max(position), 0) FROM feed")
		reply, err := f.clients.ApproveClient(t.Context(), recordRequest(f, &webv1.ApproveClientRequest{Id: waiting.ID, Name: " Box "}, ""))
		if err != nil {
			t.Fatalf("Approving a waiting machine failed: %v", err)
		}
		client := reply.Msg.GetClient()
		if client.GetId() != waiting.ID || client.GetName() != "Box" ||
			client.GetState() != webv1.ClientState_CLIENT_STATE_OFFLINE {
			t.Error("Approval did not return Box offline.")
		}
		m := f.mustGet(t, waiting.ID)
		if m.Approval != machines.ApprovalApproved || m.Name != "Box" || m.StateRevision != 2 {
			t.Error("The stored approval did not match the returned machine.")
		}
		wantFeed := "machines:saved:" + waiting.ID
		if events := recordScalar[string](t, f, `
SELECT COALESCE(string_agg(topic || ':' || change || ':' || key, ',' ORDER BY position), '')
FROM feed WHERE position > $1`, beforeFeed); events != wantFeed {
			t.Errorf("Approval left feed events %q, want %q.", events, wantFeed)
		}
		if !maps.Equal(recordTables(t, f, "machines", "feed"), beforeTables) {
			t.Error("Approval changed another table, including any command table.")
		}
	})

	for _, tc := range []struct {
		label, id, name, sentence string
		code                      connect.Code
	}{
		{"already approved", waiting.ID, "Second", "This machine does not wait for approval.", connect.CodeFailedPrecondition},
		{"unknown", unknownRecordID, "Unknown", "No machine has that id.", connect.CodeNotFound},
		{"empty name", other.ID, " ", "A machine needs a name of 1 to 64 characters.", connect.CodeInvalidArgument},
		{"taken name", other.ID, "box", "Another machine already has that name.", connect.CodeAlreadyExists},
	} {
		t.Run(tc.label, func(t *testing.T) {
			_, err := f.clients.ApproveClient(t.Context(), recordRequest(f, &webv1.ApproveClientRequest{Id: tc.id, Name: tc.name}, ""))
			requireRecordError(t, err, tc.code, tc.sentence)
		})
	}
}

func TestRenameClient(t *testing.T) {
	f := newRecordsFixture(t)
	m := f.create(t, 0x41, "waiting.example.test", false)
	_, err := f.clients.RenameClient(t.Context(), recordRequest(f, &webv1.RenameClientRequest{Id: m.ID, Name: "Early"}, ""))
	requireRecordError(t, err, connect.CodeFailedPrecondition, "Approve this machine before you rename it.")
	if err := f.write(t, func(ctx context.Context, tx *store.Tx) error {
		return f.reg.SetApproved(ctx, tx, m.ID, "Box")
	}); err != nil {
		t.Fatalf("Approving the rename fixture failed: %v", err)
	}
	for _, name := range []string{" New name ", "New name"} {
		_, err := f.clients.RenameClient(t.Context(), recordRequest(f, &webv1.RenameClientRequest{Id: m.ID, Name: name}, ""))
		if err != nil {
			t.Fatalf("Renaming the approved machine failed: %v", err)
		}
		if got := f.mustGet(t, m.ID); got.Name != "New name" || got.StateRevision != 3 {
			t.Errorf("The name is %q at revision %d, want New name at revision 3.", got.Name, got.StateRevision)
		}
	}
}

// recordTables reads the fixture's tables in one snapshot so an unexpected command table is covered.
func recordTables(t *testing.T, f *recordsFixture, exclude ...string) map[string]string {
	t.Helper()
	snapshot := make(map[string]string)
	err := f.s.Read(t.Context(), func(ctx context.Context, tx *store.ReadTx) error {
		rows, err := tx.Query(ctx, "SELECT tablename FROM pg_tables WHERE schemaname = current_schema()")
		if err != nil {
			return err
		}
		tables, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return err
		}
		for _, table := range tables {
			if slices.Contains(exclude, table) {
				continue
			}
			statement := "SELECT COALESCE(jsonb_agg(to_jsonb(r) ORDER BY to_jsonb(r)), '[]'::jsonb)::text FROM " +
				pgx.Identifier{table}.Sanitize() + " AS r"
			var contents string
			if err := tx.QueryRow(ctx, statement).Scan(&contents); err != nil {
				return err
			}
			snapshot[table] = contents
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Reading the record fixture's tables failed: %v", err)
	}
	return snapshot
}

func TestScanAndReconnectRefused(t *testing.T) {
	f := newRecordsFixture(t)
	waiting := f.create(t, 0x41, "waiting.example.test", false)
	approved := f.create(t, 0x42, "approved.example.test", false)
	if err := f.write(t, func(ctx context.Context, tx *store.Tx) error {
		return f.reg.SetApproved(ctx, tx, approved.ID, "Box")
	}); err != nil {
		t.Fatalf("Approving the refusal fixture failed: %v", err)
	}

	for _, tc := range []struct {
		label, id, scan, reconnect string
		code                       connect.Code
	}{
		{"waiting", waiting.ID, "This machine is not online.", "This machine is not connected.", connect.CodeFailedPrecondition},
		{"approved", approved.ID, "This machine is not online.", "This machine is not connected.", connect.CodeFailedPrecondition},
		{"unknown", unknownRecordID, "No machine has that id.", "No machine has that id.", connect.CodeNotFound},
	} {
		t.Run(tc.label+"/scan", func(t *testing.T) {
			before := recordTables(t, f)
			_, err := f.clients.ScanClientHarnesses(t.Context(), recordRequest(f, &webv1.ScanClientHarnessesRequest{Id: tc.id}, "scan-fixture"))
			requireRecordError(t, err, tc.code, tc.scan)
			if !maps.Equal(recordTables(t, f), before) {
				t.Error("A refused scan changed the database.")
			}
		})
		t.Run(tc.label+"/reconnect", func(t *testing.T) {
			before := recordTables(t, f)
			_, err := f.clients.ReconnectClient(t.Context(), recordRequest(f, &webv1.ReconnectClientRequest{Id: tc.id}, "reconnect-fixture"))
			requireRecordError(t, err, tc.code, tc.reconnect)
			if !maps.Equal(recordTables(t, f), before) {
				t.Error("A refused reconnect changed the database.")
			}
		})
	}
}

func TestKeyedCallsReplay(t *testing.T) {
	t.Run("taken name and key conflict", func(t *testing.T) {
		f := newRecordsFixture(t)
		first := f.create(t, 0x41, "first.example.test", false)
		second := f.create(t, 0x42, "second.example.test", false)
		key := "first-approval"
		req := &webv1.ApproveClientRequest{Id: first.ID, Name: " Box "}
		if _, err := f.clients.ApproveClient(t.Context(), recordRequest(f, req, key)); err != nil {
			t.Fatalf("Creating the first keyed approval failed: %v", err)
		}
		taken := &webv1.ApproveClientRequest{Id: second.ID, Name: "box"}
		_, err := f.clients.ApproveClient(t.Context(), recordRequest(f, taken, "taken-name"))
		requireRecordError(t, err, connect.CodeAlreadyExists, "Another machine already has that name.")
		taken.Name = "Box"
		_, err = f.clients.ApproveClient(t.Context(), recordRequest(f, taken, key))
		requireRecordError(t, err, connect.CodeAlreadyExists, "That request key was already used for a different request.")

		// Reading the receipt through WriteOnce independently checks the caller, procedure and versioned field framing.
		fingerprint := binary.BigEndian.AppendUint64(nil, uint64(len(first.ID)))
		fingerprint = append(fingerprint, first.ID...)
		fingerprint = binary.BigEndian.AppendUint64(fingerprint, uint64(len("Box")))
		fingerprint = append(fingerprint, "Box"...)
		var rewrote bool
		_, err = f.s.WriteOnce(t.Context(), store.Request{
			Key: key, Caller: "owner", Procedure: lgtmv1connect.ClientServiceApproveClientProcedure,
			ReceiptSafe: true, FingerprintVersion: 1, Fingerprints: map[int16][]byte{1: fingerprint},
		}, func(context.Context, *store.Tx) ([]byte, error) {
			rewrote = true
			return nil, errors.New("The expected approval receipt was absent.")
		})
		if err != nil || rewrote {
			t.Errorf("The approval did not leave the expected version 1 receipt: %v", err)
		}
	})

	t.Run("lock timeout can be retried", func(t *testing.T) {
		f := newRecordsFixture(t)
		m := f.create(t, 0x41, "blocked.example.test", false)
		// The advisory lock stays held until the request returns. The trigger supplies a short
		// database lock timeout so this check needs no ordering assumption about client and server timers.
		recordFixtureSQL(t, f, `
CREATE FUNCTION records_wait_for_lock() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
	PERFORM set_config('lock_timeout', '20ms', true);
	PERFORM pg_advisory_xact_lock(304201);
	RETURN NEW;
END;
$$;
CREATE TRIGGER records_wait_for_lock BEFORE UPDATE ON machines
FOR EACH ROW EXECUTE FUNCTION records_wait_for_lock();`)
		release, holder := holdWrite(t, f.s, func(ctx context.Context, tx *store.Tx) error {
			_, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(304201)")
			return err
		})
		defer release()

		req := &webv1.ApproveClientRequest{Id: m.ID, Name: "Box"}
		_, err := f.clients.ApproveClient(t.Context(), recordRequest(f, req, "lock-timeout"))
		requireRecordError(t, err, connect.CodeAborted, "The database was busy, retry the request.")
		if got := f.mustGet(t, m.ID); got.Approval != machines.ApprovalWaiting || got.StateRevision != 1 {
			t.Error("The aborted request changed the waiting machine.")
		}
		release()
		if err := holder.Wait(t); err != nil {
			t.Fatalf("Releasing the fixture lock failed: %v", err)
		}
		reply, err := f.clients.ApproveClient(t.Context(), recordRequest(f, req, "lock-timeout"))
		if err != nil || reply.Msg.GetClient().GetName() != "Box" {
			t.Fatalf("Retrying the same key did not approve the machine: %v", err)
		}
		if got := f.mustGet(t, m.ID); got.Approval != machines.ApprovalApproved || got.StateRevision != 2 {
			t.Error("The retried approval did not leave the machine approved at revision 2.")
		}
	})

	t.Run("a lost approval reply survives rename and removal", func(t *testing.T) {
		f := newRecordsFixture(t)
		m := f.create(t, 0x41, "lost-reply.example.test", false)
		// The address changes during commit, after the handler must have projected the reply.
		// A projection read outside the write transaction would see the new address.
		recordFixtureSQL(t, f, `
CREATE FUNCTION records_set_commit_address() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
	UPDATE machines SET last_address = '192.0.2.45' WHERE id = NEW.id;
	RETURN NEW;
END;
$$;
CREATE CONSTRAINT TRIGGER records_set_commit_address AFTER UPDATE ON machines
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
WHEN (OLD.approval IS DISTINCT FROM NEW.approval)
EXECUTE FUNCTION records_set_commit_address();`)
		const key = "lost-approval-reply"
		makeRequest := func(name string) *http.Request {
			body, err := json.Marshal(map[string]string{"id": m.ID, "name": name})
			if err != nil {
				t.Fatal(err)
			}
			req := f.jsonRequest(t, lgtmv1connect.ClientServiceApproveClientProcedure, string(body), true)
			req.Header.Set(store.RequestKeyHeader, key)
			return req
		}
		dropped := errors.New("The fixture discarded the response before the caller received it.")
		var firstWire []byte
		droppingClient := &http.Client{
			Timeout: 15 * time.Second,
			Transport: recordRoundTripper(func(req *http.Request) (*http.Response, error) {
				res, err := f.http.Transport.RoundTrip(req)
				if err != nil {
					return nil, err
				}
				defer res.Body.Close()
				if res.StatusCode != http.StatusOK {
					return nil, fmt.Errorf("approval returned HTTP %d before response loss", res.StatusCode)
				}
				firstWire, err = io.ReadAll(res.Body)
				if err != nil {
					return nil, err
				}
				return nil, dropped
			}),
		}
		if res, err := droppingClient.Do(makeRequest(" Box ")); !errors.Is(err, dropped) {
			if res != nil {
				_ = res.Body.Close()
			}
			t.Fatalf("The original approval did not commit and produce a discarded reply: %v", err)
		}
		var first struct {
			Client struct {
				Name    string `json:"name"`
				Address string `json:"address"`
			} `json:"client"`
		}
		if err := json.Unmarshal(firstWire, &first); err != nil || first.Client.Name != "Box" || first.Client.Address != "" {
			t.Error("The original reply was not projected from the approval transaction before commit.")
		}
		if got := f.mustGet(t, m.ID); got.LastAddress != "192.0.2.45" {
			t.Fatal("The approval did not reach the fixture's deferred commit trigger.")
		}

		_, err := f.clients.RenameClient(t.Context(), recordRequest(f, &webv1.RenameClientRequest{Id: m.ID, Name: "Renamed"}, ""))
		if err != nil {
			t.Fatalf("Renaming after the lost approval reply failed: %v", err)
		}
		_, err = f.clients.RemoveClient(t.Context(), recordRequest(f, &webv1.RemoveClientRequest{Id: m.ID}, ""))
		if err != nil {
			t.Fatalf("Removing after the lost approval reply failed: %v", err)
		}
		res, err := f.http.Do(makeRequest("Box"))
		if err != nil {
			t.Fatalf("Replaying the approval failed: %v", err)
		}
		defer res.Body.Close()
		replay, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatalf("Reading the replay failed: %v", err)
		}
		if res.StatusCode != http.StatusOK || !bytes.Equal(replay, firstWire) {
			t.Errorf("The retry returned HTTP %d or different reply bytes after rename and removal.", res.StatusCode)
		}
		if _, err := f.get(t, m.ID); !errors.Is(err, machines.ErrNoMachine) {
			t.Errorf("The approval replay restored the removed machine: %v", err)
		}
		if got := recordScalar[string](t, f, `
SELECT string_agg(change, ',' ORDER BY position) FROM feed WHERE topic = 'machines' AND key = $1`, m.ID); got != "saved,saved,saved,removed" {
			t.Errorf("The replay left the machine feed history %q, want creation, approval, rename and removal.", got)
		}
		if count := recordScalar[int64](t, f, "SELECT count(*) FROM receipts WHERE request_key = $1", key); count != 1 {
			t.Errorf("The replay left %d approval receipts, want 1.", count)
		}
	})
}

func recordFixtureSQL(t *testing.T, f *recordsFixture, statement string) {
	t.Helper()
	if err := f.write(t, func(ctx context.Context, tx *store.Tx) error {
		_, err := tx.Exec(ctx, statement)
		return err
	}); err != nil {
		t.Fatalf("Installing a record fixture trigger failed: %v", err)
	}
}

type recordRoundTripper func(*http.Request) (*http.Response, error)

func (f recordRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

var recordProcedures = []string{
	lgtmv1connect.ClientServiceCreateClientTokenProcedure,
	lgtmv1connect.ClientServiceRevokeClientTokenProcedure,
	lgtmv1connect.ClientServiceApproveClientProcedure,
	lgtmv1connect.ClientServiceRenameClientProcedure,
	lgtmv1connect.ClientServiceRemoveClientProcedure,
	lgtmv1connect.ClientServiceScanClientHarnessesProcedure,
	lgtmv1connect.ClientServiceReconnectClientProcedure,
}

func (f *recordsFixture) jsonRequest(t *testing.T, procedure, body string, signedIn bool) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, f.url+procedure, strings.NewReader(body))
	if err != nil {
		t.Fatalf("Preparing a browser request failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")
	req.Header.Set("Origin", recordsIssuerURL)
	if signedIn {
		req.AddCookie(f.cookie)
	}
	return req
}

func TestRecordCallsNeedSessionAndRedactMalformedJSON(t *testing.T) {
	f := newRecordsFixture(t)
	before := recordTables(t, f)
	const malformed = `{"submitted_field_marker":submitted_syntax_marker}`
	for _, procedure := range recordProcedures {
		_, method, _ := strings.Cut(procedure[1:], "/")
		t.Run(method, func(t *testing.T) {
			checkRecordJSONError(t, f, procedure, "{}", false, http.StatusUnauthorized, connect.CodeUnauthenticated)
			checkRecordJSONError(t, f, procedure, malformed, true, http.StatusBadRequest, connect.CodeInvalidArgument,
				"submitted_field_marker", "submitted_syntax_marker")
		})
	}
	if !maps.Equal(recordTables(t, f), before) {
		t.Error("Unsigned or malformed record requests changed the database.")
	}
}

func checkRecordJSONError(t *testing.T, f *recordsFixture, procedure, body string, signedIn bool, status int, code connect.Code, markers ...string) {
	t.Helper()
	res, err := f.http.Do(f.jsonRequest(t, procedure, body, signedIn))
	if err != nil {
		t.Fatalf("Sending a browser request failed: %v", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("Reading a browser response failed: %v", err)
	}
	for _, marker := range markers {
		if bytes.Contains(raw, []byte(marker)) {
			t.Error("The error response body echoed submitted bytes.")
		}
		for _, fields := range []http.Header{res.Header, res.Trailer} {
			for name, values := range fields {
				if strings.Contains(name, marker) || strings.Contains(strings.Join(values, "\n"), marker) {
					t.Error("The error response metadata echoed submitted bytes.")
				}
			}
		}
	}
	var result struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(raw, &result); err != nil || res.StatusCode != status || result.Code != code.String() {
		t.Errorf("The browser error returned HTTP %d and %q, want HTTP %d and %q.", res.StatusCode, result.Code, status, code)
	}
}
