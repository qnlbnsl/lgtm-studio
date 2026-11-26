CREATE TABLE web_sessions (
    verifier bytea PRIMARY KEY CHECK (octet_length(verifier) = 32),
    created_at timestamptz NOT NULL CHECK (isfinite(created_at)),
    password_salt bytea NOT NULL CHECK (octet_length(password_salt) = 16),
    password_binding bytea NOT NULL CHECK (octet_length(password_binding) = 32)
);
