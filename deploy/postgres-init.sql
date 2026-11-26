-- Reconcile through the final listener so interrupted database creation can resume.
SELECT pg_advisory_lock(1246516301);
SET log_statement = 'none';
SET log_min_error_statement = 'panic';

DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_hba_file_rules)
       OR EXISTS (SELECT FROM pg_hba_file_rules
                  WHERE error IS NOT NULL OR auth_method IS DISTINCT FROM 'scram-sha-256') THEN
        RAISE EXCEPTION 'pg_hba must contain only valid SCRAM rules';
    END IF;
    IF EXISTS (SELECT FROM pg_roles
               WHERE rolname IN ('lgtm_app', 'lgtm_bao')
                 AND (NOT rolcanlogin OR rolsuper OR rolcreatedb OR rolcreaterole
                      OR rolreplication OR rolbypassrls))
       OR EXISTS (SELECT FROM pg_auth_members m JOIN pg_roles r ON r.oid = m.member
                  WHERE r.rolname IN ('lgtm_app', 'lgtm_bao')) THEN
        RAISE EXCEPTION 'invalid PostgreSQL service login';
    END IF;
    IF EXISTS (SELECT FROM pg_database
               WHERE datname IN ('lgtm_app', 'lgtm_bao') AND pg_get_userbyid(datdba) <> datname) THEN
        RAISE EXCEPTION 'invalid PostgreSQL service database owner';
    END IF;
END
$$;

\getenv app_password LGTM_APP_PASSWORD
\getenv bao_password LGTM_BAO_PASSWORD
SELECT format('CREATE ROLE lgtm_app LOGIN PASSWORD %L', :'app_password')
WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'lgtm_app') \gexec
SELECT format('CREATE ROLE lgtm_bao LOGIN PASSWORD %L', :'bao_password')
WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'lgtm_bao') \gexec

SELECT 'CREATE DATABASE lgtm_app OWNER lgtm_app'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'lgtm_app') \gexec
SELECT 'CREATE DATABASE lgtm_bao OWNER lgtm_bao'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'lgtm_bao') \gexec

REVOKE CONNECT ON DATABASE postgres, template1, lgtm_app, lgtm_bao FROM PUBLIC;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
\connect template1
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
\connect lgtm_app
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
\connect lgtm_bao
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
