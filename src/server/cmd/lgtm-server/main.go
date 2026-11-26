// Command lgtm-server serves the website and the machine link. Its health command checks a running server.
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/app"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/config"
)

const (
	exitOK     = 0
	exitFailed = 1
	exitUsage  = 2

	defaultListenAddr = "127.0.0.1:8080"
	healthTimeout     = 2 * time.Second

	lineUsage     = "Usage: lgtm-server [health]"
	lineJoinToken = "The join-token command is unavailable in this build."
)

// retiredSettings are names that earlier builds read. The command warns about them by name only.
var retiredSettings = []string{"LGTM_POSTGRES_URL", "LGTM_SERVER_LISTEN_ADDR"}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.LookupEnv, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run dispatches the command line. No argument serves until ctx ends.
func run(ctx context.Context, args []string, lookup func(string) (string, bool), stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return serve(ctx, lookup, stdout, stderr)
	}
	switch {
	case args[0] == "health" && len(args) == 1:
		return health(ctx, lookup)
	case args[0] == "join-token":
		fmt.Fprintln(stderr, lineJoinToken)
		return exitUsage
	default:
		fmt.Fprintln(stderr, lineUsage)
		return exitUsage
	}
}

// serve starts the application and waits for it to stop.
// Standard output carries the structured log of a running server, and standard error the startup diagnostics.
func serve(ctx context.Context, lookup func(string) (string, bool), stdout, stderr io.Writer) int {
	for _, name := range retiredSettings {
		if _, present := lookup(name); present {
			fmt.Fprintf(stderr, "%s is no longer read, remove it.\n", name)
		}
	}
	cfg, err := config.Load(lookup)
	if err != nil {
		// The application reports the database settings, after it has handled the password.
		cfg.Postgres = nil
	}
	log := slog.New(slog.NewJSONHandler(stdout, &slog.HandlerOptions{AddSource: false}))
	server, err := app.Start(ctx, cfg, log)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitFailed
	}
	err = server.Wait()
	_ = server.Close()
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitFailed
	}
	return exitOK
}

// health sends one GET /healthz to the local listener and succeeds only on 204.
// It reads only LGTM_LISTEN_ADDR, maps a wildcard host to loopback, and uses no proxy and no redirect.
func health(ctx context.Context, lookup func(string) (string, bool)) int {
	addr, ok := lookup("LGTM_LISTEN_ADDR")
	if !ok {
		addr = defaultListenAddr
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return exitFailed
	}
	host = localHost(host)

	ctx, cancel := context.WithTimeout(ctx, healthTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+net.JoinHostPort(host, port)+"/healthz", nil)
	if err != nil {
		return exitFailed
	}
	client := &http.Client{
		Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true},
		Timeout:   healthTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	res, err := client.Do(req)
	if err != nil {
		return exitFailed
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		return exitFailed
	}
	return exitOK
}

// localHost maps an empty or unspecified listener host to the loopback address of the same family.
func localHost(host string) string {
	if host == "" {
		return "127.0.0.1"
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || !ip.IsUnspecified() {
		return host
	}
	if ip.Is6() && !ip.Is4In6() {
		return "::1"
	}
	return "127.0.0.1"
}
