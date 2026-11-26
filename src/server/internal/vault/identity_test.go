package vault_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest/migrated"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/vault"
)

func TestLinkIdentityRefusesReplacement(t *testing.T) {
	t.Run("migration", testIdentityMigration)
	t.Run("initialization_disabled", func(t *testing.T) {
		s := newVaultStore(t)
		state := &persistedKey{seed: fixtureSeed, version: 1}
		wire := startWireVault(t, nil, state.key)
		identity, err := vault.LoadLinkIdentity(t.Context(), s, wire.open(t), false)
		assertVaultFailure(t, identity, err)
		assertMethods(t, wire.calls())
	})
	t.Run("pin_read_fails", func(t *testing.T) {
		s := migrated.NewStore(t, "public")
		state := &persistedKey{seed: fixtureSeed, version: 1}
		wire := startWireVault(t, nil, state.key)
		identity, err := vault.LoadLinkIdentity(t.Context(), s, wire.open(t), true)
		assertVaultFailure(t, identity, err)
		assertMethods(t, wire.calls())
	})

	for _, scenario := range []struct {
		name    string
		version int
	}{
		{name: "replacement", version: 8},
		{name: "absence"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			s, observer := pinDatabase(t)
			state := &persistedKey{seed: fixtureSeed, version: 7}
			wire := startWireVault(t, nil, state.key)

			identity, err := vault.LoadLinkIdentity(t.Context(), s, wire.open(t), true)
			assertIdentity(t, identity, err, fixtureSeed, 7)
			if t.Failed() {
				return
			}
			assertPin(t, observer, fingerprint(fixtureSeed))
			restarted, err := vault.LoadLinkIdentity(t.Context(), s, wire.open(t), false)
			assertIdentity(t, restarted, err, fixtureSeed, 7)
			assertPin(t, observer, fingerprint(fixtureSeed))

			state.set([32]byte{9, 8, 7}, scenario.version)
			bad, err := vault.LoadLinkIdentity(t.Context(), s, wire.open(t), true)
			assertVaultFailure(t, bad, err)
			assertPin(t, observer, fingerprint(fixtureSeed))

			state.set(fixtureSeed, 7)
			restored, err := vault.LoadLinkIdentity(t.Context(), s, wire.open(t), true)
			assertIdentity(t, restored, err, fixtureSeed, 7)
			assertPin(t, observer, fingerprint(fixtureSeed))
			assertMethods(t, wire.calls(), loginRequest, readRequest, loginRequest, readRequest,
				loginRequest, readRequest, loginRequest, readRequest)
		})
	}
	t.Run("interrupted_first_pin", testInterruptedFirstPin)
}

func pinDatabase(t *testing.T) (*store.Store, *pgx.Conn) {
	t.Helper()
	config := storetest.NewDatabase(t, "public")
	s, err := store.Open(t.Context(), config, store.ConnectionPolicy{})
	if err != nil {
		t.Fatal("The PostgreSQL fixture store could not be opened.")
	}
	t.Cleanup(s.Close)
	if err := s.Migrate(t.Context(), vault.Migrations()); err != nil {
		t.Fatalf("The vault migration could not be applied. %v", err)
	}
	observer, err := pgx.ConnectConfig(t.Context(), config.ConnConfig.Copy())
	if err != nil {
		t.Fatal("The independent PostgreSQL observer could not connect.")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := observer.Close(ctx); err != nil {
			t.Error("The independent PostgreSQL observer could not close.")
		}
	})
	return s, observer
}

func testIdentityMigration(t *testing.T) {
	s, observer := pinDatabase(t)
	if err := s.Migrate(t.Context(), vault.Migrations()); err != nil {
		t.Fatalf("Reapplying the vault migration failed. %v", err)
	}
	columns := storetest.Rows[string](t, observer, `
SELECT column_name || ':' || data_type || ':' || is_nullable
FROM information_schema.columns
WHERE table_schema = current_schema() AND table_name = 'server_link_identity'
ORDER BY ordinal_position`)
	want := []string{"singleton:boolean:NO", "fingerprint:text:NO"}
	if !reflect.DeepEqual(columns, want) {
		t.Fatalf("The public pin table columns are %v, want %v.", columns, want)
	}
	version := storetest.Scalar[int](t, observer, "SELECT version FROM schema_migrations WHERE part = 'vault'")
	if version != 1 {
		t.Errorf("The vault migration ledger version is %d, want 1.", version)
	}
	const insert = "INSERT INTO server_link_identity (singleton, fingerprint) VALUES ($1, $2)"
	for _, scenario := range []struct {
		name        string
		singleton   bool
		fingerprint string
	}{
		{name: "false_singleton", fingerprint: fingerprint(fixtureSeed)},
		{name: "short_fingerprint", singleton: true, fingerprint: fingerprint(fixtureSeed)[:63]},
		{name: "uppercase_fingerprint", singleton: true, fingerprint: "A" + fingerprint(fixtureSeed)[1:]},
		{name: "nonhex_fingerprint", singleton: true, fingerprint: "g" + fingerprint(fixtureSeed)[1:]},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			_, err := observer.Exec(t.Context(), insert, scenario.singleton, scenario.fingerprint)
			assertSQLState(t, err, "23514")
		})
	}
	if t.Failed() {
		return
	}
	storetest.Exec(t, observer, insert, true, fingerprint(fixtureSeed))
	_, err := observer.Exec(t.Context(), insert, true, fingerprint([32]byte{1}))
	assertSQLState(t, err, "23505")
}

func assertSQLState(t *testing.T, err error, want string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != want {
		t.Errorf("The insert returned %v, want PostgreSQL error code %s.", err, want)
	}
}

func assertPin(t *testing.T, observer *pgx.Conn, want string) {
	t.Helper()
	rows := storetest.Rows[string](t, observer, "SELECT fingerprint FROM server_link_identity")
	if !reflect.DeepEqual(rows, []string{want}) {
		t.Error("PostgreSQL must hold exactly the established public fingerprint.")
	}
	refs := storetest.Rows[string](t, observer, "SELECT topic || ':' || scope || ':' || key || ':' || change FROM feed")
	if !reflect.DeepEqual(refs, []string{"server_link_identity::link:saved"}) {
		t.Error("PostgreSQL must hold exactly one saved feed reference for the public pin.")
	}
}

func testInterruptedFirstPin(t *testing.T) {
	s, observer := pinDatabase(t)
	state := &persistedKey{}
	wire := startWireVault(t, nil, state.key)

	// Fail the feed write after insertion of the pin, inside the same transaction.
	// The sequence records that the trigger ran even though the transaction rolled back.
	storetest.Exec(t, observer, `
CREATE SEQUENCE fixture_pin_feed_attempt;
CREATE FUNCTION fixture_refuse_pin_feed() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.topic = 'server_link_identity' THEN
        PERFORM nextval('fixture_pin_feed_attempt');
        RAISE EXCEPTION 'The fixture interrupted the pin transaction.' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER fixture_refuse_pin_feed BEFORE INSERT ON feed
FOR EACH ROW EXECUTE FUNCTION fixture_refuse_pin_feed();`)
	identity, err := vault.LoadLinkIdentity(t.Context(), s, wire.open(t), true)
	assertVaultFailure(t, identity, err)
	calls := wire.calls()
	assertMethods(t, calls, loginRequest, readRequest, createRequest, readRequest)
	if !storetest.Scalar[bool](t, observer, "SELECT is_called FROM fixture_pin_feed_attempt") {
		t.Error("The first load must reach the interrupted pin transaction after storing the vault key.")
	}
	if len(calls) != 4 || t.Failed() {
		return
	}
	if n := storetest.Scalar[int](t, observer, "SELECT count(*) FROM server_link_identity"); n != 0 {
		t.Errorf("The interrupted transaction left %d public pin rows.", n)
	}
	if n := storetest.Scalar[int](t, observer, "SELECT count(*) FROM feed"); n != 0 {
		t.Errorf("The interrupted transaction left %d feed rows.", n)
	}

	storetest.Exec(t, observer, "DROP TRIGGER fixture_refuse_pin_feed ON feed")
	seed := calls[2].seed
	restarted, err := vault.LoadLinkIdentity(t.Context(), s, wire.open(t), true)
	assertIdentity(t, restarted, err, seed, 1)
	assertPin(t, observer, fingerprint(seed))
	assertMethods(t, wire.calls(), loginRequest, readRequest, createRequest, readRequest, loginRequest, readRequest)
}
