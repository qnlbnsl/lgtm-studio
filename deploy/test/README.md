# Wave 1 stack checks

The checks drive the local Docker Compose stack and use the same application
programming interface (API) as the website. Run them against a disposable
development stack. Several checks interrupt connections, and C4 changes the
password temporarily, which ends all existing website sessions.

Use Linux with Bash 4.4 or newer, curl, Docker Compose, and jq for JavaScript
Object Notation (JSON). Use Go 1.26 or newer for the tests and temporary
fixtures. C5 runs `psql` inside the PostgreSQL container.

From the repository root, export the stack's `LGTM_WEB_PASSWORD`. Set
`LGTM_WEB_PORT` if the stack uses a different loopback port. Export
`LGTM_TEST_POSTGRES` with a connection string for a real, disposable PostgreSQL
database that the host's Go tests can reach.

```bash
deploy/test/checks.sh
deploy/test/checks.sh --only C1,C12
```

C1 starts the stack. When selecting a subset without C1, start the services
that the selected checks need first. C12 uses the database named in
`LGTM_TEST_POSTGRES`. C13 can exercise the vault portion with just `postgres`
and `openbao` running. It reports a failure until it can also observe the
server loading its signing key.

Each selected check prints one Test Anything Protocol (TAP) result line with
the check number from the design. The script runs selected checks in design
order. It exits with status zero only when all selected checks pass, status
one when a selected check fails, and status two for invalid arguments.

The machine checks create disposable enrollments and use temporary client
state. They stop their fixture clients and attempt to remove their test
records and sessions after the check. The restart checks restore or bring
back the services they changed. C13 recreates the running containers and
keeps their named volumes.

C8 uses one open `WatchClients` stream for both state transitions. C9 opens a
new stream after the server restart and requires a new connection time for
the same machine. C11 compares numbered events recorded as buffered by the
client during the outage with the server's record of committed applications.

C12 runs all Go packages with test caching disabled. Its executable mutation
hook must exit zero only after establishing that removing the feed lock makes
the feed-order test fail for the intended reason. The hook is responsible for
isolating or restoring its changes. A missing hook makes C12 fail.

## Names and observations the checks expect

Chosen names are provisional. The sprint plans can adopt them or update this
harness before it is frozen.

| Item | Name or observation | Source |
| --- | --- | --- |
| Compose file | The script reads `deploy/compose.yaml`. | Assignment |
| Stack start command | The script runs the executable `deploy/up.sh`. | Assignment |
| Compose services | The script uses `postgres`, `openbao`, `server`, and `web`. | Design |
| Website address | The default is `http://127.0.0.1:8787`. | Design |
| Website port setting | `LGTM_WEB_PORT` sets the published web port and the port used by the checks. | Chosen |
| Website password | `LGTM_WEB_PASSWORD` sets the password. C2 removes the variable from the server container, and C4 overrides it for one recreation. | Design |
| Go test database | `LGTM_TEST_POSTGRES` supplies the connection string to the Go tests and the mutation hook. | Design |
| PostgreSQL account and database | `POSTGRES_USER` and `POSTGRES_DB` select the account and database for C5 inside the container. They both default to `lgtm` in the script. | Chosen |
| Client commands | The checks build `lgtm-client` from `src/client/cmd/lgtm-client/` and run `lgtm-client join <address> <token>` and `lgtm-client run`. | Design |
| Server commands | Compose starts `lgtm-server` from the image's default command. The design also calls for a server token command for the installer. The working name is `lgtm-server join-token`. The checks create tokens through the Machines API. | The binary and token command are specified by the design. The default invocation and `join-token` spelling are chosen. |
| Client state folder | The normal folder is `lgtm-client` under the user's `.local/state/` directory. The checks set the freedesktop state-directory variable `XDG_STATE_HOME` to a temporary directory and expect the client to use its `lgtm-client/` child. | The normal folder is specified by the design. The override is chosen. |
| Temporary files | `TMPDIR` selects the base directory for disposable fixtures. It defaults to `/tmp`. | Standard Linux convention used by the checks. |
| Client state used for C10 | The fixture's `identity.json` contains `clientPublicKey` and `serverPublicKey` in the same encoding. C10 substitutes the fixture's own public key for its saved server public key. | Chosen |
| Join-token expiry used for C5 | The `client_join_tokens` table has an `id` column and an `expires_at` timestamp used when the server checks the token. The check sets only its own token's expiry to the past. | Chosen |
| Join and link diagnostics | The client emits one JSON object per line. The `msg` value `join_refused` uses the `code` values `token_used` and `token_expired`. The `msg` value `link_refused` uses `unknown_machine` for the server's refusal and `server_key_mismatch` for the client's refusal. | Chosen |
| Replay observations used for C11 | The client logs `msg: link_event_buffered` with a numeric `sequence` after each event is stored durably. The server logs `msg: link_event_applied` with `machine_id` and numeric `sequence` for every committed application in commit order, including an erroneous repeat. These logs use one JSON object per line. At least three events are enqueued while the server is stopped. | The numbered durable events are specified by the design. The log fields and minimum observed sample are chosen. |
| OpenBao status | The checks invoke `bao status -format=json` inside `openbao` and read `initialized` and `sealed`. | Chosen observation using the OpenBao command. |
| Signing-key load used for C13 | The server logs one JSON object with `msg: link_signing_key_loaded`, `source: openbao`, and the public `fingerprint` after reading the signing key from the vault. The check compares fingerprints across container recreation. | The vault-backed signing key is specified by the design. The log fields are chosen. |
| Website mount point | The document at `/` includes `id="app"` and a script element. | Existing website |
| Website API | The checks use the current `lgtm.v1.SessionService` and `lgtm.v1.ClientService` Protocol Buffer calls. They read `all.clients` from the first `WatchClients` message and `saved` from later messages. | Current contract |
| Change-feed mutation hook | The executable is `deploy/test/mutate-feed-lock.sh`. | Assignment |
