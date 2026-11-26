package config

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// These tests close the gaps that the review of the sign-in settings found
// (post-a 2 and post-a 3). Each one passes on the code as it is.

func gapValues(t *testing.T) map[string]string {
	t.Helper()
	return map[string]string{"LGTM_POSTGRES_PASSWORD_FILE": writeSecret(t, hexPassword+"\n", 0o440)}
}

func TestRemoteDatabaseHostLoadsWithVerifiedTLS(t *testing.T) {
	for _, host := range []string{"db.example.test", "localhost", "LocalHost"} {
		t.Run(host, func(t *testing.T) {
			values := gapValues(t)
			values["LGTM_POSTGRES_HOST"] = host
			cfg := mustLoad(t, values)
			conn := cfg.Postgres.ConnConfig
			if conn.Host != host {
				t.Errorf("Host = %q, want %q", conn.Host, host)
			}
			tls := conn.TLSConfig
			if tls == nil {
				t.Fatalf("host %q has no TLS configuration, so it would connect in plaintext", host)
			}
			if tls.InsecureSkipVerify {
				t.Errorf("host %q skips certificate verification", host)
			}
			if tls.ServerName != host {
				t.Errorf("ServerName = %q, want %q", tls.ServerName, host)
			}
		})
	}
}

func TestPGServiceIsRefused(t *testing.T) {
	// A real service file makes the driver accept the service, so that only the
	// explicit check of the loader can refuse it.
	file := filepath.Join(t.TempDir(), "pg_service.conf")
	if err := os.WriteFile(file, []byte("[some-service]\napplication_name=from-service\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PGSERVICEFILE", file)
	t.Setenv("PGSERVICE", "some-service")
	if cfg, err := Load(lookupOf(gapValues(t))); err == nil {
		t.Fatalf("Load accepted PGSERVICE and returned a database configuration: %v", cfg.Postgres != nil)
	}
}

func TestPublicURLIsDerivedOnlyFromALoopbackListener(t *testing.T) {
	for _, listen := range []string{"0.0.0.0:8080", ":8080", "[::]:8080", "192.0.2.10:8080", "app.example.test:8080"} {
		t.Run(listen, func(t *testing.T) {
			values := gapValues(t)
			values["LGTM_LISTEN_ADDR"] = listen
			cfg := mustLoad(t, values)
			if cfg.PublicURL != "" {
				t.Errorf("PublicURL = %q, want none for the listener %q", cfg.PublicURL, listen)
			}
		})
	}
	for listen, want := range map[string]string{
		"[::1]:9000":     "http://[::1]:9000",
		"localhost:9000": "http://localhost:9000",
	} {
		values := gapValues(t)
		values["LGTM_LISTEN_ADDR"] = listen
		if cfg := mustLoad(t, values); cfg.PublicURL != want {
			t.Errorf("listener %q gave PublicURL %q, want %q", listen, cfg.PublicURL, want)
		}
	}
}

func TestNamedPipeAsPasswordFileIsRefusedWithoutBlocking(t *testing.T) {
	path := filepath.Join(t.TempDir(), "postgres.password")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skipf("this file system cannot make a named pipe: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := Load(lookupOf(map[string]string{"LGTM_POSTGRES_PASSWORD_FILE": path}))
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Load accepted a named pipe as the password file")
		}
	case <-time.After(5 * time.Second):
		// Unblock the stuck open so that the goroutine can finish.
		if f, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			f.Close()
		}
		t.Fatal("Load blocked on a named pipe instead of refusing it")
	}
}
