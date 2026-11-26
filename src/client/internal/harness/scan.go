package harness

import (
	"context"
	"time"
)

// Problem says why a harness is not installed. It stays on the machine.
type Problem string

// The problems a scan can report.
const (
	ProblemNone      Problem = ""
	ProblemNotFound  Problem = "not found"
	ProblemTimedOut  Problem = "timed out"
	ProblemFailed    Problem = "failed to run"
	ProblemNoVersion Problem = "no version in output"
)

// Found is the result of the scan for one harness. Only Harness and Version leave the machine.
type Found struct {
	Harness   Name
	Installed bool
	Version   string
	Path      string
	Problem   Problem
}

// ScanOptions say where to look. A Timeout of 0 means DefaultTimeout.
type ScanOptions struct {
	SearchPath []string
	Home       string
	Timeout    time.Duration
}

// The limits of one scan.
const (
	DefaultTimeout = 3 * time.Second
	WaitDelay      = time.Second
)

// DefaultScanOptions splits PATH and reads HOME.
// This body is a stub of the test writer.
func DefaultScanOptions(getenv func(string) string) ScanOptions {
	return ScanOptions{}
}

// Scan looks for the four harnesses and returns one result for each, in the order of Definitions.
// This body is a stub of the test writer.
func Scan(ctx context.Context, opts ScanOptions) []Found {
	return nil
}
