-- This is a stub of the test writer. It only provides the fixture columns.
CREATE TABLE link_commands (
    id text PRIMARY KEY,
    machine_id text NOT NULL,
    action text NOT NULL,
    payload bytea,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    attempt bigint NOT NULL DEFAULT 1,
    sent_stream_id text,
    dispatched boolean NOT NULL DEFAULT false,
    finished_at timestamptz,
    seq bigint GENERATED ALWAYS AS IDENTITY,
    outcome text
);
