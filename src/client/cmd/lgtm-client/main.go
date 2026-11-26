// Command lgtm-client is the program that runs on each machine.
package main

import (
	"fmt"
	"io"
	"os"
)

// version is set by an installer with -ldflags "-X main.version=<v>".
var version = "0.0.0-dev"

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "version":
			fmt.Fprintln(stdout, version)
			return 0
		case "join", "run":
			fmt.Fprintf(stderr, "The %s command is not built yet.\n", args[0])
			return 1
		}
	}
	fmt.Fprintln(stderr, "Usage: lgtm-client {join|run|version}")
	return 2
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
