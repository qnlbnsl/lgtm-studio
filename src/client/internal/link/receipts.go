package link

import "time"

// This file is a stub of the test writer. The builder replaces every body.

// Command is what the store needs to know about a command from the server.
type Command struct {
	ID        string
	ExpiresAt time.Time
}

// Decision says what the caller must do with a command.
type Decision int

// The decisions of Begin.
const (
	DecisionRun Decision = 1 + iota
	DecisionRunning
	DecisionReplay
	DecisionInterrupted
	DecisionExpired
)

// Begin decides what to do with a command, and writes a started receipt before it returns DecisionRun.
func (s *Store) Begin(c Command, now time.Time) (Decision, []byte, error) { return 0, nil, nil }

// Finish writes the result of a command that this process is running.
func (s *Store) Finish(id string, result []byte) error { return nil }
