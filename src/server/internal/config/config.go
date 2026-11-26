package config

import "github.com/jackc/pgx/v5/pgxpool"

// This file holds a stub of the test writer. The builder of item 1 replaces it.

// Config is the server's settings.
type Config struct {
	ListenAddr            string
	PublicURL             string
	AllowedOrigins        []string
	Password              string
	Postgres              *pgxpool.Config
	AllowPrivateStack     bool
	OpenBaoAddr           string
	OpenBaoCredentialFile string
}

// Load is a stub of the test writer.
func Load(lookup func(string) (string, bool)) (Config, error) { return Config{}, nil }
