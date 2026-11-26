package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestStateDirRule(t *testing.T) {
	got, err := StateDir(env(map[string]string{
		"LGTM_CLIENT_STATE_DIR": "/a/b", "XDG_STATE_HOME": "/x", "HOME": "/h",
	}))
	if err != nil || got != "/a/b" {
		t.Fatalf("with the variable: got %q, %v; want /a/b", got, err)
	}

	got, err = StateDir(env(map[string]string{"XDG_STATE_HOME": "/x", "HOME": "/h"}))
	if err != nil || got != "/h/.local/state/lgtm-client" {
		t.Fatalf("without the variable: got %q, %v; want /h/.local/state/lgtm-client", got, err)
	}

	got, err = StateDir(env(map[string]string{"LGTM_CLIENT_STATE_DIR": "", "HOME": "/h"}))
	if err != nil || got != "/h/.local/state/lgtm-client" {
		t.Fatalf("with an empty variable: got %q, %v; want the default", got, err)
	}

	if got, err = StateDir(env(map[string]string{"LGTM_CLIENT_STATE_DIR": "rel", "HOME": "/h"})); err == nil {
		t.Fatalf("a relative variable gave %q and no error", got)
	}
	if got, err = StateDir(env(map[string]string{"LGTM_CLIENT_STATE_DIR": "rel/dir"})); err == nil {
		t.Fatalf("a relative variable gave %q and no error", got)
	}

	for name, m := range map[string]map[string]string{
		"relative home": {"HOME": "rel"},
		"no variable":   {},
		"empty home":    {"HOME": ""},
	} {
		got, err := StateDir(env(m))
		if !errors.Is(err, ErrNoStateDir) {
			t.Errorf("%s: got %q, %v; want ErrNoStateDir", name, got, err)
		}
	}
}

func writeSettings(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadSettings(t *testing.T) {
	const home, stateDir = "/home/u", "/state/dir"

	def := DefaultSettings(stateDir)
	if def.RepositoriesFolder != "/state/dir/repositories" || def.AgentLimit != 4 {
		t.Fatalf("DefaultSettings = %+v, want /state/dir/repositories and 4", def)
	}

	got, err := LoadSettings(filepath.Join(t.TempDir(), "missing.json"), home, stateDir)
	if err != nil || got != def {
		t.Fatalf("missing file: got %+v, %v; want the defaults %+v", got, err, def)
	}

	got, err = LoadSettings(writeSettings(t, `{"version":1,"repositories_folder":"~/repos","agent_limit":8}`), home, stateDir)
	if err != nil || got.RepositoriesFolder != "/home/u/repos" || got.AgentLimit != 8 {
		t.Fatalf("full file: got %+v, %v; want /home/u/repos and 8", got, err)
	}

	got, err = LoadSettings(writeSettings(t, `{"version":1,"repositories_folder":"/srv/repos","agent_limit":64}`), home, stateDir)
	if err != nil || got.RepositoriesFolder != "/srv/repos" || got.AgentLimit != 64 {
		t.Fatalf("absolute folder: got %+v, %v; want /srv/repos and 64", got, err)
	}

	got, err = LoadSettings(writeSettings(t, `{"version":1,"agent_limit":1}`), home, stateDir)
	if err != nil || got.RepositoriesFolder != def.RepositoriesFolder || got.AgentLimit != 1 {
		t.Fatalf("only the limit: got %+v, %v; want the default folder and 1", got, err)
	}

	got, err = LoadSettings(writeSettings(t, `{"version":1}`), home, stateDir)
	if err != nil || got != def {
		t.Fatalf("version only: got %+v, %v; want the defaults", got, err)
	}

	bad := map[string]string{
		"not json":        `this is not json`,
		"empty file":      ``,
		"version 2":       `{"version":2}`,
		"no version":      `{"agent_limit":4}`,
		"unknown field":   `{"version":1,"agentLimit":4}`,
		"relative folder": `{"version":1,"repositories_folder":"repos"}`,
		"limit 0":         `{"version":1,"agent_limit":0}`,
		"limit 65":        `{"version":1,"agent_limit":65}`,
	}
	for name, text := range bad {
		got, err := LoadSettings(writeSettings(t, text), home, stateDir)
		if !errors.Is(err, ErrSettingsInvalid) {
			t.Errorf("%s: got %+v, %v; want ErrSettingsInvalid", name, got, err)
		}
	}
}
