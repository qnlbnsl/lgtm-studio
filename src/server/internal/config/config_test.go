package config

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	webPassword = "  web pass:word with spaces ü "
	hexPassword = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

// writeSecret writes a database password file with the given content and mode.
func writeSecret(t *testing.T, content string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "postgres.password")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func lookupOf(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		v, ok := values[name]
		return v, ok
	}
}

func mustLoad(t *testing.T, values map[string]string) Config {
	t.Helper()
	cfg, err := Load(lookupOf(values))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Postgres == nil || cfg.Postgres.ConnConfig == nil {
		t.Fatalf("Load returned no database configuration")
	}
	return cfg
}

// dbPassword reads the database password without panicking on a missing configuration.
func dbPassword(cfg Config) string {
	if cfg.Postgres == nil || cfg.Postgres.ConnConfig == nil {
		return "<no database configuration>"
	}
	return cfg.Postgres.ConnConfig.Password
}

func verifiesServer(c *tls.Config, host string) bool {
	return c != nil && !c.InsecureSkipVerify && strings.EqualFold(c.ServerName, host)
}

func TestReadServerSettings(t *testing.T) {
	secret := writeSecret(t, hexPassword+"\n", 0o440)

	t.Run("loopback defaults", func(t *testing.T) {
		cfg := mustLoad(t, map[string]string{
			"LGTM_POSTGRES_PASSWORD_FILE": secret,
			"LGTM_WEB_PASSWORD":           webPassword,
		})
		if cfg.ListenAddr != "127.0.0.1:8080" {
			t.Errorf("ListenAddr = %q, want 127.0.0.1:8080", cfg.ListenAddr)
		}
		if cfg.PublicURL != "http://127.0.0.1:8080" {
			t.Errorf("PublicURL = %q, want a plaintext origin from the loopback listener", cfg.PublicURL)
		}
		if len(cfg.AllowedOrigins) != 0 {
			t.Errorf("AllowedOrigins = %q, want none", cfg.AllowedOrigins)
		}
		if cfg.AllowPrivateStack {
			t.Errorf("AllowPrivateStack is on by default")
		}
		if cfg.Password != webPassword {
			t.Errorf("Password = %q, want the literal bytes %q", cfg.Password, webPassword)
		}
		if cfg.OpenBaoAddr != "" || cfg.OpenBaoCredentialFile != "" {
			t.Errorf("the OpenBao settings = %q, %q, want them unset", cfg.OpenBaoAddr, cfg.OpenBaoCredentialFile)
		}
		conn := cfg.Postgres.ConnConfig
		if conn.Host != "127.0.0.1" || conn.Port != 5432 || conn.Database != "lgtm_app" || conn.User != "lgtm_app" {
			t.Errorf("database endpoint = %s:%d database %q user %q, want 127.0.0.1:5432 lgtm_app lgtm_app",
				conn.Host, conn.Port, conn.Database, conn.User)
		}
		if conn.Password != hexPassword {
			t.Errorf("the database password was not read from the file without its line feed")
		}
		if conn.TLSConfig != nil {
			t.Errorf("literal loopback should use sslmode=disable, got a TLS configuration")
		}
		if cfg.Postgres.MaxConns != 8 {
			t.Errorf("MaxConns = %d, want 8", cfg.Postgres.MaxConns)
		}
	})

	t.Run("listener address derives the public origin", func(t *testing.T) {
		cfg := mustLoad(t, map[string]string{
			"LGTM_POSTGRES_PASSWORD_FILE": secret,
			"LGTM_LISTEN_ADDR":            "127.0.0.1:9191",
		})
		if cfg.ListenAddr != "127.0.0.1:9191" || cfg.PublicURL != "http://127.0.0.1:9191" {
			t.Errorf("listener and public origin = %q, %q, want 127.0.0.1:9191 and http://127.0.0.1:9191", cfg.ListenAddr, cfg.PublicURL)
		}
	})

	t.Run("complete Compose environment", func(t *testing.T) {
		cfg := mustLoad(t, map[string]string{
			"LGTM_WEB_PASSWORD":            webPassword,
			"LGTM_LISTEN_ADDR":             ":8080",
			"LGTM_PUBLIC_URL":              "http://127.0.0.1:18787",
			"LGTM_POSTGRES_HOST":           "postgres",
			"LGTM_POSTGRES_PORT":           "5432",
			"LGTM_POSTGRES_DATABASE":       "lgtm_app",
			"LGTM_POSTGRES_USER":           "lgtm_app",
			"LGTM_POSTGRES_PASSWORD_FILE":  secret,
			"LGTM_ALLOW_PRIVATE_STACK":     "true",
			"LGTM_OPENBAO_CREDENTIAL_FILE": "/run/lgtm/openbao.json",
			"LGTM_OPENBAO_ADDR":            "http://openbao:8200",
		})
		if cfg.ListenAddr != ":8080" {
			t.Errorf("ListenAddr = %q, want :8080", cfg.ListenAddr)
		}
		if cfg.PublicURL != "http://127.0.0.1:18787" {
			t.Errorf("PublicURL = %q, want the nondefault published port", cfg.PublicURL)
		}
		if !cfg.AllowPrivateStack {
			t.Errorf("AllowPrivateStack = false, want true")
		}
		if cfg.Password != webPassword {
			t.Errorf("Password = %q, want the literal bytes %q", cfg.Password, webPassword)
		}
		if cfg.OpenBaoAddr != "http://openbao:8200" || cfg.OpenBaoCredentialFile != "/run/lgtm/openbao.json" {
			t.Errorf("OpenBao settings = %q, %q", cfg.OpenBaoAddr, cfg.OpenBaoCredentialFile)
		}
		conn := cfg.Postgres.ConnConfig
		if conn.Host != "postgres" || conn.Port != 5432 || conn.Database != "lgtm_app" || conn.User != "lgtm_app" {
			t.Errorf("database endpoint = %s:%d database %q user %q", conn.Host, conn.Port, conn.Database, conn.User)
		}
		if conn.TLSConfig != nil {
			t.Errorf("the opted-in private service name should use sslmode=disable")
		}
		if cfg.Postgres.MaxConns != 8 {
			t.Errorf("MaxConns = %d, want 8", cfg.Postgres.MaxConns)
		}
	})

	t.Run("explicit database settings", func(t *testing.T) {
		cfg := mustLoad(t, map[string]string{
			"LGTM_POSTGRES_PASSWORD_FILE": secret,
			"LGTM_POSTGRES_HOST":          "[::1]",
			"LGTM_POSTGRES_PORT":          "15432",
			"LGTM_POSTGRES_DATABASE":      "other_db",
			"LGTM_POSTGRES_USER":          "other_user",
		})
		conn := cfg.Postgres.ConnConfig
		if conn.Port != 15432 || conn.Database != "other_db" || conn.User != "other_user" {
			t.Errorf("database = port %d database %q user %q", conn.Port, conn.Database, conn.User)
		}
		if !strings.Contains(conn.Host, "::1") || conn.TLSConfig != nil {
			t.Errorf("an IPv6 loopback host %q should use sslmode=disable, TLS = %v", conn.Host, conn.TLSConfig)
		}
	})

	t.Run("opt-in decides plaintext to the private service", func(t *testing.T) {
		with := map[string]string{
			"LGTM_POSTGRES_PASSWORD_FILE": secret,
			"LGTM_POSTGRES_HOST":          "postgres",
			"LGTM_ALLOW_PRIVATE_STACK":    "true",
		}
		if cfg := mustLoad(t, with); cfg.Postgres.ConnConfig.TLSConfig != nil {
			t.Errorf("with the opt-in, the service name postgres should be plaintext")
		}
		without := map[string]string{
			"LGTM_POSTGRES_PASSWORD_FILE": secret,
			"LGTM_POSTGRES_HOST":          "postgres",
		}
		cfg, err := Load(lookupOf(without))
		if err == nil && cfg.Postgres != nil && !verifiesServer(cfg.Postgres.ConnConfig.TLSConfig, "postgres") {
			t.Errorf("without the opt-in, the host postgres must use verified TLS or be refused")
		}
		off := map[string]string{
			"LGTM_POSTGRES_PASSWORD_FILE": secret,
			"LGTM_POSTGRES_HOST":          "postgres",
			"LGTM_ALLOW_PRIVATE_STACK":    "false",
		}
		cfg, err = Load(lookupOf(off))
		if err == nil && cfg.Postgres != nil && !verifiesServer(cfg.Postgres.ConnConfig.TLSConfig, "postgres") {
			t.Errorf("with the opt-in set to false, the host postgres must use verified TLS or be refused")
		}
	})

	t.Run("other hosts verify TLS even with the opt-in", func(t *testing.T) {
		for _, host := range []string{"db", "openbao", "postgres.example.test", "db.internal", "pg", "192.0.2.10", "postgresql", "Postgres"} {
			for _, optIn := range []string{"", "true"} {
				values := map[string]string{
					"LGTM_POSTGRES_PASSWORD_FILE": secret,
					"LGTM_POSTGRES_HOST":          host,
				}
				if optIn != "" {
					values["LGTM_ALLOW_PRIVATE_STACK"] = optIn
				}
				cfg, err := Load(lookupOf(values))
				if err != nil {
					continue // a refusal is as safe as verified TLS
				}
				if cfg.Postgres == nil || !verifiesServer(cfg.Postgres.ConnConfig.TLSConfig, cfg.Postgres.ConnConfig.Host) {
					t.Errorf("host %q with opt-in %q does not use verified TLS", host, optIn)
				}
			}
		}
	})

	t.Run("web password stays optional for reconciliation", func(t *testing.T) {
		for name, values := range map[string]map[string]string{
			"unset": {"LGTM_POSTGRES_PASSWORD_FILE": secret},
			"empty": {"LGTM_POSTGRES_PASSWORD_FILE": secret, "LGTM_WEB_PASSWORD": ""},
		} {
			cfg, err := Load(lookupOf(values))
			if err != nil {
				t.Errorf("%s: a missing web password failed the load: %v", name, err)
				continue
			}
			if cfg.Password != "" {
				t.Errorf("%s: Password = %q, want it absent", name, cfg.Password)
			}
			if cfg.Postgres == nil {
				t.Errorf("%s: the database configuration was not assembled", name)
			}
		}
	})

	t.Run("web password keeps its bytes", func(t *testing.T) {
		for _, value := range []string{" ", "  lead", "trail  ", "tab\tinside", "ünï", "a\nb", strings.Repeat("x", 5000), "\xff\xfe"} {
			cfg := mustLoad(t, map[string]string{"LGTM_POSTGRES_PASSWORD_FILE": secret, "LGTM_WEB_PASSWORD": value})
			if cfg.Password != value {
				t.Errorf("Password for %d bytes was changed", len(value))
			}
		}
	})

	t.Run("malformed origin configuration is retained", func(t *testing.T) {
		cfg := mustLoad(t, map[string]string{
			"LGTM_POSTGRES_PASSWORD_FILE":  secret,
			"LGTM_PUBLIC_URL":              "not a url at all",
			"LGTM_ALLOWED_ORIGINS":         " https://studio.example.org , not-an-origin,http://x.example.test/path ",
			"LGTM_OPENBAO_ADDR":            "::not an address::",
			"LGTM_OPENBAO_CREDENTIAL_FILE": "relative/credential.json",
		})
		if cfg.PublicURL != "not a url at all" {
			t.Errorf("PublicURL = %q, want the raw value", cfg.PublicURL)
		}
		want := []string{"https://studio.example.org", "not-an-origin", "http://x.example.test/path"}
		if strings.Join(cfg.AllowedOrigins, "|") != strings.Join(want, "|") {
			t.Errorf("AllowedOrigins = %q, want %q", cfg.AllowedOrigins, want)
		}
		if cfg.OpenBaoAddr != "::not an address::" || cfg.OpenBaoCredentialFile != "relative/credential.json" {
			t.Errorf("OpenBao settings = %q, %q, want the raw values", cfg.OpenBaoAddr, cfg.OpenBaoCredentialFile)
		}
	})

	t.Run("password file protection", func(t *testing.T) {
		refused := map[string]os.FileMode{
			"world readable":  0o644,
			"world writable":  0o666,
			"group writable":  0o660,
			"other read only": 0o404,
			"all access":      0o777,
		}
		for name, mode := range refused {
			path := writeSecret(t, hexPassword+"\n", mode)
			cfg, err := Load(lookupOf(map[string]string{"LGTM_POSTGRES_PASSWORD_FILE": path, "LGTM_WEB_PASSWORD": webPassword}))
			if err == nil {
				t.Errorf("a %s file (%#o) was accepted", name, mode)
			}
			if cfg.Password != webPassword {
				t.Errorf("a %s file lost the web password for reconciliation", name)
			}
		}
		for name, mode := range map[string]os.FileMode{"0440": 0o440, "0400": 0o400, "0600": 0o600} {
			path := writeSecret(t, hexPassword+"\n", mode)
			if cfg := mustLoad(t, map[string]string{"LGTM_POSTGRES_PASSWORD_FILE": path}); cfg.Postgres.ConnConfig.Password != hexPassword {
				t.Errorf("a file with mode %s was read wrongly", name)
			}
		}

		real := writeSecret(t, hexPassword+"\n", 0o440)
		link := filepath.Join(t.TempDir(), "link.password")
		if err := os.Symlink(real, link); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(lookupOf(map[string]string{"LGTM_POSTGRES_PASSWORD_FILE": link})); err == nil {
			t.Errorf("a symbolic link to a protected file was followed")
		}

		dir := t.TempDir()
		if _, err := Load(lookupOf(map[string]string{"LGTM_POSTGRES_PASSWORD_FILE": dir})); err == nil {
			t.Errorf("a directory was accepted as the password file")
		}
		missing := filepath.Join(t.TempDir(), "absent")
		if _, err := Load(lookupOf(map[string]string{"LGTM_POSTGRES_PASSWORD_FILE": missing})); err == nil {
			t.Errorf("a missing password file was accepted")
		}
		if _, err := Load(lookupOf(map[string]string{})); err == nil {
			t.Errorf("an unset password file setting was accepted")
		}
		if _, err := Load(lookupOf(map[string]string{"LGTM_POSTGRES_PASSWORD_FILE": ""})); err == nil {
			t.Errorf("an empty password file setting was accepted")
		}
	})

	t.Run("password file format", func(t *testing.T) {
		compose := func(content string) (Config, error) {
			return Load(lookupOf(map[string]string{
				"LGTM_POSTGRES_PASSWORD_FILE": writeSecret(t, content, 0o440),
				"LGTM_POSTGRES_HOST":          "postgres",
				"LGTM_ALLOW_PRIVATE_STACK":    "true",
			}))
		}
		standalone := func(content string) (Config, error) {
			return Load(lookupOf(map[string]string{"LGTM_POSTGRES_PASSWORD_FILE": writeSecret(t, content, 0o440)}))
		}

		if cfg, err := compose(hexPassword + "\n"); err != nil || dbPassword(cfg) != hexPassword {
			t.Errorf("the exact Compose format was refused: %v", err)
		}
		for name, content := range map[string]string{
			"no line feed":       hexPassword,
			"upper case":         strings.ToUpper(hexPassword) + "\n",
			"too short":          hexPassword[:63] + "\n",
			"too long":           hexPassword + "0\n",
			"not hexadecimal":    strings.Repeat("g", 64) + "\n",
			"two line feeds":     hexPassword + "\n\n",
			"carriage return":    hexPassword + "\r\n",
			"empty":              "",
			"second line":        hexPassword + "\nextra\n",
			"leading whitespace": " " + hexPassword[1:] + "\n",
		} {
			if _, err := compose(content); err == nil {
				t.Errorf("Compose accepted a password file with %s", name)
			}
		}

		if cfg, err := standalone("a plain secret\n"); err != nil || dbPassword(cfg) != "a plain secret" {
			t.Errorf("standalone refused one nonempty line: %v", err)
		}
		if cfg, err := standalone("no final line feed"); err != nil || dbPassword(cfg) != "no final line feed" {
			t.Errorf("standalone refused a password without a final line feed: %v", err)
		}
		for name, content := range map[string]string{
			"empty file":   "",
			"only newline": "\n",
			"two lines":    "first\nsecond\n",
			"blank first":  "\nsecret\n",
		} {
			if _, err := standalone(content); err == nil {
				t.Errorf("standalone accepted a password file with %s", name)
			}
		}
	})
}

func TestSettingsErrorsRedactValues(t *testing.T) {
	secret := writeSecret(t, hexPassword+"\n", 0o440)
	base := func() map[string]string {
		return map[string]string{
			"LGTM_POSTGRES_PASSWORD_FILE": secret,
			"LGTM_WEB_PASSWORD":           webPassword,
		}
	}

	cases := []struct {
		name    string
		setting string
		value   string
		extra   map[string]string
	}{
		{"port not a number", "LGTM_POSTGRES_PORT", "port-value-7f3a", nil},
		{"port out of range", "LGTM_POSTGRES_PORT", "70001", nil},
		{"port zero", "LGTM_POSTGRES_PORT", "0", nil},
		{"private stack flag", "LGTM_ALLOW_PRIVATE_STACK", "perhaps-7f3a", nil},
		{"private stack flag in upper case words", "LGTM_ALLOW_PRIVATE_STACK", "YES-7f3a", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			values := base()
			values[c.setting] = c.value
			cfg, err := Load(lookupOf(values))
			if err == nil {
				t.Fatalf("%s=%q was accepted", c.setting, c.value)
			}
			message := err.Error()
			if !strings.Contains(message, c.setting) {
				t.Errorf("the error %q does not name %s", message, c.setting)
			}
			for _, leaked := range []string{webPassword, hexPassword, "postgres://", "postgresql://", "password="} {
				if strings.Contains(message, leaked) {
					t.Errorf("the error %q contains %q", message, leaked)
				}
			}
			if strings.Contains(c.value, "7f3a") && strings.Contains(message, c.value) {
				t.Errorf("the error %q repeats the invalid value", message)
			}
			if cfg.Password != webPassword {
				t.Errorf("a database error lost the web password: %q", cfg.Password)
			}
		})
	}

	t.Run("password file formats", func(t *testing.T) {
		const leaked = "leaked-secret-9c1d"
		for name, setup := range map[string]func() map[string]string{
			"world readable": func() map[string]string {
				v := base()
				v["LGTM_POSTGRES_PASSWORD_FILE"] = writeSecret(t, leaked+"\n", 0o644)
				return v
			},
			"two lines": func() map[string]string {
				v := base()
				v["LGTM_POSTGRES_PASSWORD_FILE"] = writeSecret(t, leaked+"\nsecond-9c1d\n", 0o440)
				return v
			},
			"Compose format": func() map[string]string {
				v := base()
				v["LGTM_POSTGRES_PASSWORD_FILE"] = writeSecret(t, leaked+"\n", 0o440)
				v["LGTM_POSTGRES_HOST"] = "postgres"
				v["LGTM_ALLOW_PRIVATE_STACK"] = "true"
				return v
			},
			"missing file": func() map[string]string {
				v := base()
				v["LGTM_POSTGRES_PASSWORD_FILE"] = filepath.Join(t.TempDir(), "absent-9c1d")
				return v
			},
		} {
			t.Run(name, func(t *testing.T) {
				values := setup()
				cfg, err := Load(lookupOf(values))
				if err == nil {
					t.Fatalf("the bad password file was accepted")
				}
				message := err.Error()
				if !strings.Contains(message, "LGTM_POSTGRES_PASSWORD_FILE") {
					t.Errorf("the error %q does not name LGTM_POSTGRES_PASSWORD_FILE", message)
				}
				for _, secretText := range []string{leaked, "second-9c1d", webPassword, "postgres://", "postgresql://"} {
					if strings.Contains(message, secretText) {
						t.Errorf("the error %q contains %q", message, secretText)
					}
				}
				if cfg.Password != webPassword {
					t.Errorf("a database error lost the web password: %q", cfg.Password)
				}
			})
		}
	})

	t.Run("later-stage settings survive a database error", func(t *testing.T) {
		values := base()
		values["LGTM_POSTGRES_PORT"] = "bad"
		values["LGTM_PUBLIC_URL"] = "https://app.example.test"
		values["LGTM_ALLOWED_ORIGINS"] = "https://studio.example.org"
		values["LGTM_LISTEN_ADDR"] = "127.0.0.1:9000"
		values["LGTM_OPENBAO_ADDR"] = "http://openbao:8200"
		values["LGTM_OPENBAO_CREDENTIAL_FILE"] = "/run/lgtm/openbao.json"
		cfg, err := Load(lookupOf(values))
		if err == nil {
			t.Fatalf("an invalid port was accepted")
		}
		if cfg.Password != webPassword || cfg.PublicURL != "https://app.example.test" ||
			len(cfg.AllowedOrigins) != 1 || cfg.ListenAddr != "127.0.0.1:9000" ||
			cfg.OpenBaoAddr != "http://openbao:8200" || cfg.OpenBaoCredentialFile != "/run/lgtm/openbao.json" {
			t.Errorf("the raw settings were not filled after a database error: %+v", cfg)
		}
	})

	t.Run("web password is not echoed even when it is the invalid value", func(t *testing.T) {
		values := base()
		values["LGTM_POSTGRES_PORT"] = webPassword
		_, err := Load(lookupOf(values))
		if err == nil {
			t.Fatalf("a port equal to the web password was accepted")
		}
		if strings.Contains(err.Error(), strings.TrimSpace(webPassword)) {
			t.Errorf("the error %q repeats the web password", err.Error())
		}
	})
}
