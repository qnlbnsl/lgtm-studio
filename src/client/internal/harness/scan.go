package harness

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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
func DefaultScanOptions(getenv func(string) string) ScanOptions {
	return ScanOptions{
		SearchPath: filepath.SplitList(getenv("PATH")),
		Home:       getenv("HOME"),
		Timeout:    DefaultTimeout,
	}
}

// Scan looks for the four harnesses and returns one result for each, in the order of Definitions.
func Scan(ctx context.Context, opts ScanOptions) []Found {
	dirs := cleanSearchPath(opts.SearchPath)
	env := []string{"PATH=" + strings.Join(dirs, string(os.PathListSeparator)), "HOME=" + opts.Home, "LANG=C"}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	defs := Definitions()
	found := make([]Found, len(defs))
	var scans sync.WaitGroup
	for i, def := range defs {
		scans.Go(func() {
			budget, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			found[i] = scanHarness(budget, def, dirs, opts.Home, env)
		})
	}
	scans.Wait()
	return found
}

func cleanSearchPath(paths []string) []string {
	var dirs []string
	seen := make(map[string]bool, len(paths))
	for _, dir := range paths {
		if !filepath.IsAbs(dir) {
			continue
		}
		dir = filepath.Clean(dir)
		if !seen[dir] {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

func scanHarness(ctx context.Context, def Definition, dirs []string, home string, env []string) Found {
	result := Found{Harness: def.Name, Problem: ProblemNotFound}
	for _, program := range def.Programs {
		for _, dir := range dirs {
			if ctx.Err() != nil {
				return result
			}
			path := filepath.Join(dir, program)
			if _, err := os.Lstat(path); err != nil {
				continue
			}
			version, problem := readVersion(ctx, def, path, home, env)
			if problem == ProblemNone {
				return Found{Harness: def.Name, Installed: true, Version: version, Path: path}
			}
			if result.Problem == ProblemNotFound {
				result.Problem = problem
				result.Path = path
			}
		}
	}
	return result
}

func readVersion(ctx context.Context, def Definition, path, home string, env []string) (string, Problem) {
	cmd := exec.CommandContext(ctx, path, "--version")
	cmd.Dir = home
	cmd.Env = env
	cmd.WaitDelay = WaitDelay
	var output firstLine
	cmd.Stdout = &output
	err := cmd.Run()
	if err != nil && !errors.Is(err, exec.ErrWaitDelay) {
		var exit *exec.ExitError
		if ctx.Err() != nil && (!errors.As(err, &exit) || exit.ExitCode() < 0) {
			return "", ProblemTimedOut
		}
		return "", ProblemFailed
	}
	if !output.overflow {
		line := bytes.TrimSuffix(output.data[:output.size], []byte("\r"))
		if match := def.Version.FindSubmatch(line); len(match) > 1 {
			return string(match[1]), ProblemNone
		}
	}
	return "", ProblemNoVersion
}

// firstLine captures at most four kibibytes while allowing the child to finish writing.
type firstLine struct {
	data     [4 * 1024]byte
	size     int
	done     bool
	overflow bool
}

func (w *firstLine) Write(p []byte) (int, error) {
	if !w.done {
		space := len(w.data) - w.size
		prefix := p[:min(len(p), space)]
		if newline := bytes.IndexByte(prefix, '\n'); newline >= 0 {
			prefix = prefix[:newline]
			w.done = true
		} else if len(p) > space {
			w.done = true
			w.overflow = true
		}
		w.size += copy(w.data[w.size:], prefix)
	}
	return len(p), nil
}
