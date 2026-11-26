// Package config finds the state folder and reads the settings of the client.
package config

import "errors"

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

// StateDir is a stub of the test writer.
func StateDir(getenv func(string) string) (string, error) {
	return "", errors.New("stub")
}

// DefaultSettings is a stub of the test writer.
func DefaultSettings(stateDir string) Settings {
	return Settings{}
}

// LoadSettings is a stub of the test writer.
func LoadSettings(path, home, stateDir string) (Settings, error) {
	return Settings{}, nil
}
