CREATE TABLE client_join_tokens (
    id text PRIMARY KEY, secret_sha256 bytea NOT NULL CHECK (octet_length(secret_sha256) = 32),
    created_at timestamptz NOT NULL, expires_at timestamptz NOT NULL,
    hosts_server boolean NOT NULL DEFAULT false, revoked_at timestamptz, used_at timestamptz,
    used_key bytea CHECK (used_key IS NULL OR octet_length(used_key) = 32), used_by text);
