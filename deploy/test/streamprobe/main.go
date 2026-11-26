// Package main is a stub of the test writer for the stream probe command.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	mode := flag.String("mode", "", "serve, dial, or shaped")
	addr := flag.String("addr", ":8080", "address to listen on")
	url := flag.String("url", "", "address to dial")
	shape := flag.String("shape", "", "quiet-up or quiet-down")
	every := flag.Duration("every", 2*time.Second, "time between messages")
	total := flag.Duration("total", 20*time.Second, "active duration")
	gap := flag.Duration("gap", 0, "silent duration")
	flag.Parse()

	switch *mode {
	case "serve":
		serve(*addr)
	case "dial":
		dial(*url, *every, *total, *gap)
	case "shaped":
		shaped(*url, *shape, *total)
	default:
		fmt.Fprintln(os.Stderr, "Use -mode serve, dial, or shaped.")
		os.Exit(2)
	}
}

// serve is a stub of the test writer.
func serve(_ string) {}

// dial is a stub of the test writer.
func dial(_ string, _, _, _ time.Duration) {}

// shaped is a stub of the test writer.
func shaped(_, _ string, _ time.Duration) {}
