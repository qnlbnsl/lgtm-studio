CREATE TABLE feed (
    position bigint GENERATED ALWAYS AS IDENTITY (START WITH 1 CACHE 1) PRIMARY KEY,
    topic text NOT NULL CHECK (topic <> ''),
    scope text NOT NULL,
    key text NOT NULL CHECK (key <> ''),
    change text NOT NULL CHECK (change IN ('saved', 'removed')),
    written_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    retention_started_at timestamptz
);

CREATE TABLE feed_retention (
    singleton boolean PRIMARY KEY CHECK (singleton),
    pruned_through bigint NOT NULL CHECK (pruned_through >= 0)
);

INSERT INTO feed_retention (singleton, pruned_through) VALUES (true, 0);

CREATE TABLE receipts (
    principal_hash bytea NOT NULL CHECK (octet_length(principal_hash) = 32),
    request_key text NOT NULL CHECK (octet_length(request_key) BETWEEN 1 AND 256),
    fingerprint_version smallint NOT NULL CHECK (fingerprint_version > 0),
    request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
    reply bytea NOT NULL CHECK (octet_length(reply) <= 65536),
    written_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    retention_started_at timestamptz,
    PRIMARY KEY (principal_hash, request_key)
);

CREATE INDEX receipts_retention_idx
    ON receipts (retention_started_at NULLS FIRST, principal_hash, request_key);

CREATE FUNCTION feed_check_write() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    CASE TG_OP
        WHEN 'INSERT' THEN
            IF current_setting('lgtm.feed_flush', true) IS DISTINCT FROM 'on' THEN
                RAISE EXCEPTION 'The feed requires a store flush.'
                    USING ERRCODE = '23514', CONSTRAINT = 'feed_requires_store_flush';
            END IF;
        WHEN 'UPDATE', 'DELETE' THEN
            IF current_setting('lgtm.feed_prune', true) IS DISTINCT FROM 'on' THEN
                RAISE EXCEPTION 'The feed requires store pruning.'
                    USING ERRCODE = '23514', CONSTRAINT = 'feed_requires_store_prune';
            END IF;
        WHEN 'TRUNCATE' THEN
            RAISE EXCEPTION 'The feed cannot be truncated.'
                USING ERRCODE = '23514', CONSTRAINT = 'feed_refuses_truncate';
    END CASE;
    RETURN NULL;
END;
$$;

CREATE TRIGGER feed_check_write
    BEFORE INSERT OR UPDATE OR DELETE OR TRUNCATE ON feed
    FOR EACH STATEMENT EXECUTE FUNCTION feed_check_write();

CREATE FUNCTION feed_check_delete_watermark() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    floor_position bigint;
BEGIN
    EXECUTE format('SELECT pruned_through FROM %I.feed_retention WHERE singleton', TG_TABLE_SCHEMA)
        INTO floor_position;
    IF floor_position IS NULL OR OLD.position > floor_position THEN
        RAISE EXCEPTION 'The feed position exceeds the pruning watermark.'
            USING ERRCODE = '23514', CONSTRAINT = 'feed_delete_requires_watermark';
    END IF;
    RETURN OLD;
END;
$$;

CREATE TRIGGER feed_check_delete_watermark
    BEFORE DELETE ON feed
    FOR EACH ROW EXECUTE FUNCTION feed_check_delete_watermark();
