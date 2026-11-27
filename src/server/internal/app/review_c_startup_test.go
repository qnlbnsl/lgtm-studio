package app_test

import (
	"errors"
	"net"
	"syscall"
	"testing"
	"time"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/app"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store/storetest"
	"github.com/qnlbnsl/lgtm-studio/src/server/servertest"
)

func TestStartupClearsStalePresenceBeforeListening(t *testing.T) {
	fixture := servertest.Start(t, servertest.Options{Password: machinesFixturePassword})
	cfg, observer := fixtureLinkConfig(t, fixture)
	clients := fixture.Clients(t)
	machine := joinAppLink(t, fixture, clients, "Held startup cleanup")
	approveAppLink(t, clients, machine.ID, "Held startup cleanup")
	fixture.Stop(t)
	seedStale(t, observer, machine.ID, 1, 0)

	const gate int64 = 166801
	storetest.Exec(t, observer, `
CREATE FUNCTION hold_startup_presence_delete() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(166801::bigint);
    RETURN OLD;
END;
$$;
CREATE TRIGGER hold_startup_presence_delete BEFORE DELETE ON link_connections
FOR EACH ROW EXECUTE FUNCTION hold_startup_presence_delete();`)
	release := storetest.HoldAdvisoryGate(t, observer, gate)
	defer release()

	type startupResult struct {
		server *app.Server
		err    error
	}
	starting := storetest.Start(t, func() startupResult {
		server, err := app.Start(t.Context(), cfg, nil)
		return startupResult{server, err}
	})
	t.Cleanup(func() {
		if result := starting.Wait(t); result.server != nil {
			_ = result.server.Close()
		}
	})
	storetest.Poll(t, "the stale connection deletion waiting in PostgreSQL", func() bool {
		return storetest.Scalar[bool](t, observer, `
SELECT EXISTS (SELECT FROM pg_stat_activity
WHERE datname = current_database() AND $1 = ANY(pg_blocking_pids(pid)))`, int32(observer.PgConn().PID()))
	})

	conn, err := net.DialTimeout("tcp", cfg.ListenAddr, time.Second)
	if err == nil {
		_ = conn.Close()
		t.Error("The server accepted a connection while the stale presence deletion was blocked.")
	} else if !errors.Is(err, syscall.ECONNREFUSED) {
		t.Errorf("The startup probe did not receive a connection refusal: %v", err)
	}

	release()
	result := starting.Wait(t)
	if result.err != nil || result.server == nil {
		t.Fatalf("Startup did not complete after the stale presence deletion was released: %v", result.err)
	}
	if storetest.Scalar[bool](t, observer, "SELECT EXISTS (SELECT FROM link_connections WHERE machine_id = $1)", machine.ID) {
		t.Error("Startup left the stale connection row in PostgreSQL.")
	}
	conn, err = net.DialTimeout("tcp", cfg.ListenAddr, time.Second)
	if err != nil {
		t.Fatal("The server did not listen after clearing stale presence.")
	}
	_ = conn.Close()
}
