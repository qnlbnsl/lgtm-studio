// Package config finds the state folder and reads the settings of the client.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// StateDirVariable names the variable that overrides the state folder.
const StateDirVariable = "LGTM_CLIENT_STATE_DIR"

var (
	ErrNoStateDir      = errors.New("config: no state folder can be found")
	ErrSettingsInvalid = errors.New("config: settings are invalid")
)

// Settings holds the owner's choices.
type Settings struct {
	RepositoriesFolder string
	AgentLimit         uint32
}

// StateDir returns the configured state folder or the default under HOME.
func StateDir(getenv func(string) string) (string, error) {
	if dir := getenv(StateDirVariable); dir != "" {
		if !filepath.IsAbs(dir) {
			return "", fmt.Errorf("%w: %s must be an absolute path", ErrNoStateDir, StateDirVariable)
		}
		return dir, nil
	}
	home := getenv("HOME")
	if !filepath.IsAbs(home) {
		return "", ErrNoStateDir
	}
	return filepath.Join(home, ".local", "state", "lgtm-client"), nil
}

// DefaultSettings returns the choices used when the owner omits them.
func DefaultSettings(stateDir string) Settings {
	return Settings{
		RepositoriesFolder: filepath.Join(stateDir, "repositories"),
		AgentLimit:         4,
	}
}

// LoadSettings reads the versioned settings file and fills in omitted choices.
func LoadSettings(path, home, stateDir string) (Settings, error) {
	settings := DefaultSettings(stateDir)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return settings, nil
	}
	if err != nil {
		return Settings{}, err
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return Settings{}, fmt.Errorf("%w: %v", ErrSettingsInvalid, err)
	}
	var version int
	if err := json.Unmarshal(fields["version"], &version); err != nil || version != 1 {
		return Settings{}, fmt.Errorf("%w: version must be 1", ErrSettingsInvalid)
	}
	for name, value := range fields {
		switch name {
		case "version":
		case "repositories_folder":
			var folder string
			if err := json.Unmarshal(value, &folder); err != nil {
				return Settings{}, fmt.Errorf("%w: repositories_folder must be a path", ErrSettingsInvalid)
			}
			if relative, ok := strings.CutPrefix(folder, "~/"); ok {
				folder = filepath.Join(home, relative)
			}
			if !filepath.IsAbs(folder) {
				return Settings{}, fmt.Errorf("%w: repositories_folder must resolve to an absolute path", ErrSettingsInvalid)
			}
			settings.RepositoriesFolder = folder
		case "agent_limit":
			var limit uint32
			if err := json.Unmarshal(value, &limit); err != nil || limit < 1 || limit > 64 {
				return Settings{}, fmt.Errorf("%w: agent_limit must be from 1 to 64", ErrSettingsInvalid)
			}
			settings.AgentLimit = limit
		default:
			return Settings{}, fmt.Errorf("%w: unknown field %q", ErrSettingsInvalid, name)
		}
	}
	return settings, nil
}
