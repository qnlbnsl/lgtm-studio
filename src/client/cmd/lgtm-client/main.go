// Command lgtm-client is the program that runs on each machine.
package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/qnlbnsl/lgtm-studio/src/client/internal/app"
)

// version is set by an installer with -ldflags "-X main.version=<v>".
var version = "0.0.0-dev"

// The optional reader preserves callers that only supply the output streams.
func run(args []string, stdout, stderr io.Writer, input ...io.Reader) int {
	stdin := io.Reader(os.Stdin)
	if len(input) > 0 {
		stdin = input[0]
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runContext(ctx, args, stdin, stdout, stderr)
}

func runContext(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts := app.Options{Version: version, Stderr: stderr}
	switch len(args) {
	case 1:
		switch args[0] {
		case "version":
			fmt.Fprintln(stdout, version)
			return 0
		case "run":
			return app.Run(ctx, opts)
		}
	case 3:
		if args[0] == "join" {
			token := args[2]
			if token == "-" {
				token = readJoinToken(stdin)
			}
			if token != "" && len(token) <= maxJoinTokenBytes {
				if err := app.Join(ctx, opts, args[1], token); err != nil {
					return 1
				}
				return 0
			}
		}
	}
	fmt.Fprintln(stderr, "Usage: lgtm-client {join|run|version}")
	return 2
}

const maxJoinTokenBytes = 4096

func readJoinToken(stdin io.Reader) string {
	if stdin == nil {
		return ""
	}
	const maxLineBytes = maxJoinTokenBytes + len("\r\n")
	reader := bufio.NewReaderSize(io.LimitReader(stdin, int64(maxLineBytes)), maxLineBytes)
	line, err := reader.ReadSlice('\n')
	switch err {
	case nil:
		line = bytes.TrimSuffix(line[:len(line)-1], []byte("\r"))
	case io.EOF:
	default:
		return ""
	}
	if len(line) > maxJoinTokenBytes {
		return ""
	}
	return string(line)
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, os.Stdin))
}
