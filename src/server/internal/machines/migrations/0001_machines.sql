CREATE TABLE machines (
    id text PRIMARY KEY CHECK (id ~ '^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
    public_key bytea NOT NULL CHECK (octet_length(public_key) = 32), host_name text NOT NULL CHECK (octet_length(host_name) <= 255),
    name text NOT NULL DEFAULT '' CHECK (octet_length(name) <= 64), approval text NOT NULL CHECK (approval IN ('waiting', 'approved')),
    state_revision bigint NOT NULL CHECK (state_revision >= 1), hosts_server boolean NOT NULL DEFAULT false, token_id text NOT NULL,
    joined_at timestamptz NOT NULL DEFAULT clock_timestamp(), approved_at timestamptz,
    facts bytea, harnesses bytea, harnesses_scanned boolean NOT NULL DEFAULT false,
    last_seen_at timestamptz, last_address text NOT NULL DEFAULT '');
CREATE UNIQUE INDEX machines_approved_name ON machines (lower(name)) WHERE approval = 'approved';
