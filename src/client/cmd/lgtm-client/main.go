// Command lgtm-client is the program that runs on each machine.
package main

import (
	"io"
	"os"
)

// version is set by an installer with -ldflags "-X main.version=<v>".
var version = "0.0.0-dev"

// run is a stub of the test writer. The builder of item 1 replaces it.
func run(args []string, stdout, stderr io.Writer) int {
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
