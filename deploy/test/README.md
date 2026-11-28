# Wave 1 stack checks

The checks drive the local Docker Compose stack and use the same application
programming interface (API) as the website. Run them against a disposable
development stack. Several checks interrupt connections, and C4 changes the
password temporarily, which ends all existing website sessions.

Use Linux with Bash 4.4 or newer, curl, Docker Compose, and jq for JavaScript
Object Notation (JSON). Use Go 1.26 or newer for the tests and temporary
fixtures. C5 runs `psql` inside the PostgreSQL container.

From the repository root, export the stack's `LGTM_WEB_PASSWORD` for checks
that sign in. The script discovers the published port from Compose when
`LGTM_WEB_PORT` is unset or empty. Export
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

## Run C12 with disposable PostgreSQL

Run the check with the wrapper from the repository root.

```bash
deploy/test/with-postgres.sh -- deploy/test/checks.sh --only C12
```

The wrapper starts the pinned PostgreSQL image on a dynamically assigned
loopback port. It sets `LGTM_TEST_POSTGRES` for the command after `--`.
The test login owns the `lgtm_store_test` maintenance database and can create
separate databases for tests. The wrapper returns the command's exit status
and removes the container, storage, and password files when the command exits
or the wrapper receives an interrupt or termination signal.

## Run the Sprint 4 release checks

Use the [Sprint 4 runner prerequisites](README-s4.md#run-the-automated-checks).
Run these commands from the repository root. Use the reviewed revision 2
contract commit as the history base and the integrated release commit as the
candidate.

```bash
(
    set -e
    set +x
    export S4_CONTRACT_COMMIT=89506b7c
    export S4_CANDIDATE_COMMIT="$(git rev-parse HEAD)"
    export LGTM_REV1_COMMIT=f6cbea30
    git merge-base --is-ancestor "$S4_CONTRACT_COMMIT" "$S4_CANDIDATE_COMMIT"
    bash deploy/test/sprint4-fixture-test.sh
    bash deploy/test/sprint4-fixture-review-test.sh
    bash deploy/test/sprint4-compose.sh "$S4_CANDIDATE_COMMIT"

    source deploy/test/sprint4-fixture.sh
    umask 022
    sprint4_fixture_open "$PWD" "$S4_CANDIDATE_COMMIT"
    sprint4_select 4
    sprint4_fixture_run bash "$S4_CANDIDATE_ROOT/deploy/up.sh"
    cd "$S4_CANDIDATE_ROOT"
    pnpm install --frozen-lockfile
    sprint4_fixture_ready
    sprint4_fixture_run bash deploy/test/checks-s4.sh --only R1,R2,R4,R11,R12
    sprint4_fixture_ready
    sprint4_fixture_run bash deploy/test/checks-s4.sh --only R13
    sprint4_fixture_close
)
```

The Compose check must print `ok 1 - s4_projects_uses_existing_stack`.
It verifies the Compose model and signed-in Projects discovery through the
deployed website. Record the candidate commit, both private image tags and
their image identifiers. Record `phase server done`, `phase web done`, and
the selected loopback address before running the repository checks.

Keep the exit status and the result line for each of R1, R2, R4, R11, R12,
and R13. R12 observes removal while the deployed page is open. R13 compares
one website build in the in-page sample, the mock server, and the real
candidate contexts. The [joint release plan](../../.plans/plans/go-s4-stack-checks/plan.md#rollout-order)
lists the remaining release gates.

## Run the deployed Sprint 4 repository check

The owner runs R14 on the candidate's deployed `web` image. That image
builds the website with `VITE_LGTM_SERVER=/`. Use a real, approved machine
and two distinct Git checkouts on it. Choose checkouts that will save as
separate repositories. Give one checkout an uncommitted change.

1. Open the address printed by the launcher and sign in. The deployed
   website should open without a **Server address** prompt. In the browser's
   network panel, confirm that `GetServer` uses the same scheme, host, and
   port as the page. Confirm that its response advertises
   `lgtm.v1.ProjectService` at website contract revision 1.
2. Wait for the approved machine to show as online on the Machines page.
   Open **Add repository** from the sidebar. Choose that machine and enter
   the first checkout's path.
3. Compare **Base branch**, **Remote**, and **Status** with the first
   checkout. **Base branch** uses the remote's default branch when the
   checkout knows it. Otherwise it uses the checked-out branch or falls
   back to `main`. **Remote** shows the `origin` address without credentials
   when available. **Status** distinguishes a clean working tree from one
   with uncommitted changes. Save the repository.
4. On the Machines page, open **Add repository** from that machine's menu.
   Check that the dialog has selected the machine. Enter the second
   checkout's path. Compare the same three rows with the second checkout,
   then save it.
5. For each save, record the repository identifier and the folder identifier
   from `AddRepository`. Record the machine identifier and resolved checkout
   path from the saved folder.
6. Reload the website. Verify that both repositories appear in the sidebar
   tree. Match both saved identities, their machine, and their resolved paths
   against the new authenticated `WatchRepositories` snapshot.

For the post-build removal observation, use the same disposable stack and a
second disposable machine. Attach a folder for one saved repository to the
second machine before removing the first machine. Reopen the authenticated
repository watch after removal. Verify that the replacement snapshot keeps
the repository and folder on the second machine. Verify that it omits the
two folders from the removed machine and the repository left without a
folder or project. Keep the snapshot with the browser observations.
Record the owner's R14 result with the candidate commit and the deployed
same-origin request. Keep credentials out of the evidence.

## Names and observations the checks expect

Chosen names are provisional. The sprint plans can adopt them or update this
harness before it is frozen.

| Item | Name or observation | Source |
| --- | --- | --- |
| Compose file | The script uses `deploy/compose.yaml` when `COMPOSE_FILE` is unset or empty. Otherwise, it uses the files in `COMPOSE_FILE`. C2 adds its temporary override after those files. | Stack web plan |
| Stack start command | The script runs the executable `deploy/up.sh`. | Assignment |
| Compose services | The script uses `postgres`, `openbao`, `server`, and `web`. | Design |
| Website address | The website uses Hypertext Transfer Protocol (HTTP) on loopback. The default port is 8787. | Design |
| Website port setting | `LGTM_WEB_PORT` sets the published web port. When the shell leaves it unset or empty, the script looks up the running project's published port with `docker compose port web 8080` using the same Compose files as the launcher. | Stack web plan |
| Website password | `LGTM_WEB_PASSWORD` sets the password. C2 removes the variable from the server container and requires the server to report that it revoked sessions before exiting. C4 overrides the password for one recreation and checks the original session and the replacement sign-in. | Design and stack web plan |
| Sign-in fixtures | The script sends passwords to `jq` through standard input with shell tracing disabled. The cookie jars live in the private fixture directory. Cleanup attempts to end any session before deleting the directory. | Stack client plan |
| Go test database | `LGTM_TEST_POSTGRES` supplies the connection string to the Go tests and the mutation hook. | Design |
| PostgreSQL account and database | The sprint 1 stack uses `lgtm_app` for both the C5 account and database. The C5 query helper must connect over loopback as user and group 70 and read `/run/lgtm-db/postgres-app.password` inside the container. The function `psql_in_stack` does this and keeps its bound arguments and standard input. | Sprint 1 stack plan |
| Client commands | The checks build `lgtm-client` from `src/client/cmd/lgtm-client/` and run `lgtm-client join <address> <token>` and `lgtm-client run`. | Design |
| Server commands | Compose starts `lgtm-server` from the image's default command. The design also calls for a server token command for the installer. The working name is `lgtm-server join-token`. The checks create tokens through the Machines API. | The binary and token command are specified by the design. The default invocation and `join-token` spelling are chosen. |
| Client state folder | The normal folder is `lgtm-client` under the user's `.local/state/` directory. The checks set `LGTM_CLIENT_STATE_DIR` to the absolute path of the fixture's `lgtm-client` directory for both client commands. | Design and client configuration |
| Temporary files | `TMPDIR` selects the base directory for disposable fixtures. It defaults to `/tmp`. | Standard Linux convention used by the checks. |
| Client state used for C10 | The fixture's `identity.json` contains `clientPublicKey` and `serverPublicKey` in the same encoding. C10 substitutes the fixture's own public key for its saved server public key. | Chosen |
| Join-token expiry used for C5 | The `client_join_tokens` table has an `id` column and an `expires_at` timestamp used when the server checks the token. The check sets only its own token's expiry to the past. | Chosen |
| Join and link diagnostics | The client emits one JSON object per line. The `msg` value `join_refused` uses the `code` values `token_used` and `token_expired`. The `msg` value `link_refused` uses `unknown_machine` for the server's refusal and `server_key_mismatch` for the client's refusal. | Chosen |
| Replay observations used for C11 | The client logs `msg: link_event_buffered` with a numeric `sequence` after each event is stored durably. The server logs `msg: link_event_applied` with `machine_id` and numeric `sequence` for every committed application in commit order, including an erroneous repeat. These logs use one JSON object per line. At least three events are enqueued while the server is stopped. | The numbered durable events are specified by the design. The log fields and minimum observed sample are chosen. |
| OpenBao status | The checks invoke `bao status -format=json` inside `openbao` and read `initialized` and `sealed`. | Chosen observation using the OpenBao command. |
| Signing-key load used for C13 | The server logs one JSON object with `msg: link_signing_key_loaded`, `source: openbao`, and the public `fingerprint` after reading the signing key from the vault. The check compares fingerprints across container recreation. | Server sign-in plan |
| Website mount point | The document at `/` includes `id="app"` and a script element. | Existing website |
| Website API | The checks use the current `lgtm.v1.SessionService` and `lgtm.v1.ClientService` Protocol Buffer calls. They read `all.clients` from the first `WatchClients` message and `saved` from later messages. | Current contract |
| Change-feed mutation hook | The executable is `deploy/test/mutate-feed-lock.sh`. | Assignment |
