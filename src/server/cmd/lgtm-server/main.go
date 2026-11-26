package main

import (
	"context"
	"io"
	"os"
)

// These are the test writer's stubs for the server command.
func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.LookupEnv, os.Stdout, os.Stderr))
}

func run(context.Context, []string, func(string) (string, bool), io.Writer, io.Writer) int {
	return 1
}
