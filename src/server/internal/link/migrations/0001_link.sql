CREATE TABLE link_machines (
    machine_id text PRIMARY KEY,
    event_mark bigint NOT NULL DEFAULT 0 CHECK (event_mark >= 0),
    stream_generation bigint NOT NULL DEFAULT 0
);

CREATE TABLE link_connections (
    machine_id text PRIMARY KEY,
    stream_id text NOT NULL UNIQUE,
    generation bigint NOT NULL,
    connected_at timestamptz NOT NULL,
    last_heard_at timestamptz NOT NULL,
    address text NOT NULL DEFAULT '',
    cpu_percent smallint CHECK (cpu_percent BETWEEN 0 AND 100),
    memory_percent smallint CHECK (memory_percent BETWEEN 0 AND 100),
    facts_received boolean NOT NULL DEFAULT false,
    harnesses_received boolean NOT NULL DEFAULT false
);
