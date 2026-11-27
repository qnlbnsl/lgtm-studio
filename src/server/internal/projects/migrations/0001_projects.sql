CREATE TABLE repositories (
    id text PRIMARY KEY,
    name text NOT NULL CHECK (name <> ''),
    main_branch text NOT NULL CHECK (main_branch <> ''),
    remote text CHECK (remote <> ''),
    first_commit text CHECK (first_commit <> ''),
    remote_omitted boolean NOT NULL,
    CONSTRAINT repositories_remote_omitted CHECK (NOT remote_omitted OR remote IS NULL)
);

CREATE TABLE repository_folders (
    id text PRIMARY KEY,
    repository_id text NOT NULL REFERENCES repositories (id) ON DELETE RESTRICT,
    machine_id text NOT NULL,
    resolved_path text COLLATE "C" NOT NULL CHECK (octet_length(resolved_path) BETWEEN 1 AND 8192)
);

CREATE INDEX repository_folders_repository ON repository_folders (repository_id);
-- A resolved path can exceed the index entry limit.
CREATE INDEX repository_folders_machine ON repository_folders (machine_id);

CREATE TABLE folder_operations (
    id text PRIMARY KEY,
    machine_id text NOT NULL,
    request_key text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('inspect', 'add')),
    target_repository_id text,
    command_id text NOT NULL UNIQUE,
    state text NOT NULL CHECK (state IN ('open', 'succeeded', 'failed')),
    reply bytea,
    failure text,
    retention_started_at timestamptz,
    CONSTRAINT folder_operations_target CHECK (kind = 'add' OR target_repository_id IS NULL),
    CONSTRAINT folder_operations_result CHECK (
        (state = 'open' AND reply IS NULL AND failure IS NULL AND retention_started_at IS NULL)
        OR (state = 'succeeded' AND reply IS NOT NULL AND failure IS NULL)
        OR (state = 'failed' AND reply IS NULL AND failure IS NOT NULL)
    )
);

CREATE INDEX folder_operations_open_machine
    ON folder_operations (machine_id) WHERE state = 'open';
CREATE INDEX folder_operations_terminal_retention
    ON folder_operations (retention_started_at, id) WHERE state <> 'open';
