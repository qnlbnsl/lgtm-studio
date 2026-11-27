ALTER TABLE link_machines
    ADD COLUMN admitted_run_id text,
    ADD COLUMN ready_run_id text,
    ADD COLUMN recovery_set_generation bigint CHECK (recovery_set_generation >= 0);

CREATE TABLE link_runs (
    machine_id text NOT NULL REFERENCES link_machines (machine_id) ON DELETE CASCADE,
    run_id text NOT NULL,
    replay_through bigint NOT NULL CHECK (replay_through >= 0),
    PRIMARY KEY (machine_id, run_id)
);

ALTER TABLE link_commands
    DROP CONSTRAINT link_commands_action,
    ADD CONSTRAINT link_commands_action CHECK (action IN ('scan_harnesses', 'set_state', 'restart', 'look_at_folder')),
    ADD COLUMN staged_outcome text,
    ADD COLUMN staged_event_number bigint,
    ADD CONSTRAINT link_commands_stage CHECK (
        (staged_outcome IS NULL AND staged_event_number IS NULL)
        OR (
            staged_outcome IS NOT NULL AND staged_event_number IS NOT NULL
            AND staged_event_number > 0
            AND staged_outcome IN ('done', 'failed', 'expired', 'unsupported', 'interrupted')
            AND (action <> 'set_state' OR staged_outcome <> 'expired')
            AND dispatched AND sent_stream_id IS NOT NULL AND sent_stream_id <> ''
            AND outcome IS NULL AND finished_at IS NULL AND payload IS NOT NULL
        )
    );

DROP INDEX link_commands_pending;
CREATE INDEX link_commands_pending
    ON link_commands (machine_id, seq) WHERE outcome IS NULL AND staged_outcome IS NULL;

DROP INDEX link_commands_expirable;
CREATE INDEX link_commands_expirable
    ON link_commands (expires_at, machine_id, seq)
    WHERE outcome IS NULL AND staged_outcome IS NULL AND action <> 'set_state';

CREATE INDEX link_commands_staged
    ON link_commands (machine_id, staged_event_number, seq) WHERE staged_event_number IS NOT NULL;
