// Package harness finds the coding agent programs that are installed on this machine.
package harness

import "regexp"

// Name is the name of a harness, as the contract spells it in InstalledHarness.harness.
type Name string

// The four harnesses of wave 1.
const (
	Claude Name = "claude"
	Codex  Name = "codex"
	Gemini Name = "gemini"
	Cursor Name = "cursor"
)

// Definition says which programs a harness may be installed as, and what its version line looks like.
type Definition struct {
	Name     Name
	Programs []string
	Version  *regexp.Regexp
}

// Definitions returns the four harnesses in the order Claude, Codex, Gemini, Cursor.
func Definitions() []Definition {
	return []Definition{
		{Claude, []string{"claude"}, regexp.MustCompile(`^(\d+\.\d+\.\d+) \(Claude Code\)$`)},
		{Codex, []string{"codex"}, regexp.MustCompile(`^codex-cli (\d+\.\d+\.\d+(?:-[0-9A-Za-z.]+)?)$`)},
		{Gemini, []string{"gemini"}, regexp.MustCompile(`^(\d+\.\d+\.\d+(?:-[0-9A-Za-z.]+)?)$`)},
		{Cursor, []string{"cursor-agent", "agent"}, regexp.MustCompile(`^(\d{4}\.\d{1,2}\.\d{1,2}-[0-9a-f]{6,12})$`)},
	}
}
