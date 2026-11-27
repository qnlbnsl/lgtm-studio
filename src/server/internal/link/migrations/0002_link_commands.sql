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
    seq bigint GENERATED ALWAYS AS IDENTITY UNIQUE,
    outcome text,
    CONSTRAINT link_commands_action CHECK (action IN ('scan_harnesses', 'set_state', 'restart')),
    CONSTRAINT link_commands_attempt_range CHECK (attempt BETWEEN 1 AND 4294967295),
    CONSTRAINT link_commands_unassigned CHECK (sent_stream_id IS NOT NULL OR (attempt = 1 AND NOT dispatched)),
    CONSTRAINT link_commands_outcome CHECK (outcome IN ('done', 'failed', 'expired', 'unsupported', 'interrupted', 'timed_out', 'removed')),
    CONSTRAINT link_commands_result_dispatched CHECK (dispatched OR outcome IS NULL OR outcome IN ('timed_out', 'removed')),
    CONSTRAINT link_commands_state_outcome CHECK (action <> 'set_state' OR outcome NOT IN ('expired', 'timed_out')),
    CONSTRAINT link_commands_terminal_pair CHECK (
        (outcome IS NULL AND finished_at IS NULL AND payload IS NOT NULL)
        OR (outcome IS NOT NULL AND finished_at IS NOT NULL AND payload IS NULL)
    )
);

CREATE INDEX link_commands_pending
    ON link_commands (machine_id, seq) WHERE outcome IS NULL;
CREATE INDEX link_commands_expirable
    ON link_commands (expires_at, machine_id, seq) WHERE outcome IS NULL AND action <> 'set_state';
CREATE INDEX link_commands_finished
    ON link_commands (finished_at, machine_id, seq) WHERE outcome IS NOT NULL;
