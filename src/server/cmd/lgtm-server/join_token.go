package main

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"io"

	"github.com/qnlbnsl/lgtm-studio/src/server/internal/config"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/machines"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/store"
	"github.com/qnlbnsl/lgtm-studio/src/server/internal/vault"
)

func joinTokenCommand(ctx context.Context, args []string, lookup func(string) (string, bool), stdout, stderr io.Writer) int {
	hostsServer := len(args) == 1 && args[0] == "--hosts-server"
	if len(args) != 0 && !hostsServer {
		fmt.Fprintln(stderr, "usage: lgtm-server join-token [--hosts-server]")
		return exitUsage
	}

	cfg, err := config.Load(lookup)
	if err != nil {
		fmt.Fprintln(stderr, "Could not load the server settings.")
		return exitFailed
	}
	s, err := store.Open(ctx, cfg.Postgres, store.ConnectionPolicy{AllowPrivateStackPostgres: cfg.AllowPrivateStack})
	if err != nil {
		fmt.Fprintln(stderr, "Could not open the PostgreSQL store.")
		return exitFailed
	}
	defer s.Close()

	client, err := vault.Open(cfg.OpenBaoAddr, cfg.OpenBaoCredentialFile, cfg.AllowPrivateStack)
	if err != nil {
		fmt.Fprintln(stderr, "Could not open the OpenBao vault.")
		return exitFailed
	}
	identity, err := vault.LoadLinkIdentity(ctx, s, client, false)
	if err != nil {
		fmt.Fprintln(stderr, "Could not load the server identity from the vault.")
		return exitFailed
	}

	tokens := machines.NewTokens(s, machines.NewRegistry(), machines.Issuer{
		PublicURL: cfg.PublicURL,
		ServerKey: identity.SigningKey.Public().(ed25519.PublicKey),
	})
	token, err := tokens.Create(ctx, machines.TokenOptions{HostsServer: hostsServer})
	if err != nil {
		fmt.Fprintln(stderr, "Could not create a join token.")
		return exitFailed
	}

	line := token.Text + "\n"
	if n, err := io.WriteString(stdout, line); err != nil || n != len(line) {
		fmt.Fprintln(stderr, "Could not write the join token to standard output.")
		return exitFailed
	}
	return exitOK
}
