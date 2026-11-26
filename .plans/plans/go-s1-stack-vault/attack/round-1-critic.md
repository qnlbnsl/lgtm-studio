# Round 1 review of `go-s1-stack-vault`, seat `critic`

- Task identifier: review-go-s1-stack-vault-r1-critic
- Nonce: 0cd98f
- Plan text reviewed: `plan.md` as it stood on 2026-11-26, the same text as `attack/plan-v1.md`.

## Verdict

REVISE

## Top finding

The plan's PostgreSQL role setup, as written, makes OpenBao exit at start-up with "permission denied for schema public", and the proof of work item 1 cannot catch this because it runs as the PostgreSQL superuser.

## Findings

### critic-r1 1. The vault role cannot create its table as the plan describes the roles

- Severity: MAJOR.
- Tied to: D10.
- Evidence: `.plans/plans/go-s1-stack-vault/plan.md:125` gives `lgtm_bao` no database administrator rights and says only to "grant each service login connect access to its database". It names no database owner and no schema grant. `plan.md:81` and `scouting.md:115-136` run the proof as the superuser `postgres`.
- Failure scenario: I reproduced it on `postgres:16.16-alpine` and `openbao/openbao:2.7.3`. I made the role `lgtm_bao` with login and no other rights, made the database `lgtm_bao` owned by `postgres`, revoked public connect and granted connect to the role. OpenBao then exited with status 1 and the message "error initializing storage of type postgresql: failed to create tables: ... permission denied for schema public". PostgreSQL 16 does not let a role create objects in the `public` schema of a database it does not own. The same setup with `ALTER DATABASE lgtm_bao OWNER TO lgtm_bao` started normally. A builder who follows the plan text ships a stack where OpenBao never starts. The frozen test `postgres_roles_are_isolated` would catch it, but only after the builder has guessed the missing rule.
- Fix: In work item 2, state that `lgtm_app` owns database `lgtm_app` and `lgtm_bao` owns database `lgtm_bao`, and that neither owns the other's database. State that no role other than `postgres` has any right on the other database or on `template1`. Add a sentence to work item 1 that the proof must create its role and database in the same way, with a login role that is not a superuser, so the proof exercises the production grants. Add to `postgres_roles_are_isolated` a line that OpenBao itself starts against `lgtm_bao` with the generated role.

### critic-r1 2. The tests of items 2, 5 and 6 need hooks that the plan does not define

- Severity: MAJOR.
- Tied to: plan.
- Evidence: `plan.md:130` ("second database creation paused during first start"), `plan.md:212` ("a test-only observer of the root token and publish boundary"), `plan.md:213` ("a recording transport in front of real OpenBao"), `plan.md:214` ("an injected interruption at the request boundary"), `plan.md:231` ("records the phase boundaries"). The epic requires a test writer who does not build, and the lens question asks whether the tests can be written from the item and the design alone.
- Failure scenario: The test writer for `deploy/test/vault-init.sh` must make `openbao-init.sh` stop just after token revocation and before publication. The plan names no environment variable, file, signal or log line that does this. The test writer either invents a hook that the builder must then add to a production script, or the builder invents it and the frozen test is wrong. The same holds for pausing the second database creation, for observing the publish boundary, and for recording the phases of two overlapping launchers. The tests cannot be frozen before the code exists, which breaks the tests-first rule of the loop.
- Fix: For each of the five behaviours, either name the exact external means or remove the need for a hook. Examples that need no hook in production code: kill the job container with `docker kill` when a named line appears in its output, and require that line in the plan; make the launcher print one fixed line at the start and the end of each phase (`phase seal start`, `phase seal done` and so on) and have the serialization test read them; run the recording proxy as a test-owned container on the `vault` network that the test points `BAO_ADDR` at. List these lines and names in the work item so the test writer can use them.

### critic-r1 3. Several half-finished and lost states end in a dead end with no step for the person

- Severity: MAJOR.
- Tied to: D10, C1.
- Evidence: `plan.md:202` stops with `vault is initialized but bootstrap state is incomplete` and says "Do not attempt another initialization or discard the database". `plan.md:204` fails closed when the published file is missing with no resumable state. `plan.md:225` documents only the missing-key diagnostic. `plan.md:115-117` does not say what `stack-prepare database` does when PostgreSQL's volume has `PG_VERSION` but a password file is missing, or when the files exist but the volume does not.
- Failure scenario: (1) The first start is killed during PostgreSQL's first initialization. The official image skips its init scripts on the next start because the data folder already exists, so the roles and databases never appear. `up --wait --wait-timeout 60` then ends with a bare timeout. (2) The first start is killed between the vault's `PUT /v1/sys/init` and the durable save of the response. The vault is initialized, and the root token and recovery share exist nowhere. Every later run prints the incomplete-state diagnostic and no instruction. (3) A person removes `deploy/.state/server/` or runs `git clean -fdx`, because the default state folder is inside the checkout and ignored by git. The vault data persists in the named volume while the credential, the seal key and the passwords are gone. In each case the person has no documented way back and may delete the wrong thing.
- Fix: Add a table to work item 6 with one row for each combination of these four facts: state folder present or not, PostgreSQL volume present or not, vault database empty or not, credential published or not. For each row, give the action of `up.sh` and the exact message. For the three dead ends above, give a reset procedure in `deploy/README.md` with the exact commands, and say plainly which data it destroys. On first run, have the PostgreSQL init script write a completion marker as its last statement, and have `up.sh` print a diagnostic that names the reset step when PostgreSQL stays unhealthy and the marker is absent. Add a test for each row that can be reached without root.

### critic-r1 4. The state folder is made of files that the operator cannot read, change or remove

- Severity: MAJOR.
- Tied to: plan.
- Evidence: `plan.md:119-123` sets owners 70, 10001 and 10002 with modes 0400 or 0440. `plan.md:137` and `plan.md:200` make `admin/` and `server/` owned by root with mode 0700. The ownership and mode of `bao/` and of the state root after setup are not given. `plan.md:221` creates the state root as the operator. `plan.md:145`, `plan.md:229` and `plan.md:243` have host-side tests compare "identity and content fingerprints", move the key "into test staging" and tear down "only their own ... state".
- Failure scenario: The operator, who is not root, runs `bash deploy/up.sh`. Containers running as root fill the state root with files that belong to other users. The host test then cannot `stat` or hash `bao/unseal.key`, cannot move it for `missing_key_preserves_existing_vault`, and cannot `rm -rf` its own temporary state at teardown, because a root-owned directory with mode 0700 cannot be emptied. Every temporary state folder under the system temporary directory leaks, and a person who wants to start over has to use `sudo`. The README guide for restoring the key "with its original ownership and mode" cannot be followed without root either, and the plan does not say so.
- Fix: State the owner, group and mode of every directory under the state root, including `bao/` and the root itself. State which operations of `up.sh` and of the tests run through a short-lived helper container as root (for example a `stack-prepare` subcommand `inspect` that prints fingerprints, and one `reset` that removes the state), and require the tests to use only those. Say in `deploy/README.md` which commands need `sudo`. Add a test that two fresh test projects can be torn down by the operator with no leftover under the temporary directory.

### critic-r1 5. The PostgreSQL health check is not authenticated, and the plan builds three tests on that claim

- Severity: MINOR.
- Tied to: D4.
- Evidence: `plan.md:104` ("two authenticated network queries"), `plan.md:125` ("an authenticated `SELECT 1` through PostgreSQL's loopback network listener"), `plan.md:130`, `plan.md:156`.
- Failure scenario: I started `postgres:16.16-alpine` with `POSTGRES_HOST_AUTH_METHOD=scram-sha-256` and connected over the loopback address with no password. It answered `select 1`. The generated `pg_hba.conf` has `trust` for `local` sockets and for both loopback ranges, and the scram line applies only to other addresses. The probe is therefore not authenticated. It still proves readiness, because the temporary start-up server listens on a socket only. But the mounted password files of the application and OpenBao roles serve the probe for nothing, and a builder who tries to make the probe use the password sees no difference. The test `postgres_waits_for_both_databases` rests on "disabling the second login" making the probe fail, which depends on the role flag and not on authentication.
- Fix: Reword the probe as "a query over the TCP listener of the final server, as each service role, against its own database". Say that the loopback listener trusts local connections and that the isolation of roles is proven by the test that connects from another container over the `data` network. Drop the mount of the two role password files into the PostgreSQL container if the init script is their only reader.

### critic-r1 6. Work item 1 writes outside the Stack team's folder, and its facts can be recorded now

- Severity: MINOR.
- Tied to: D10, C13.
- Evidence: `plan.md:73` has the builder "append the sanitized result" to `scouting.md`. The scope file `.plans/scopes/stack/infra/scope.json` lists `deploy/**` as the only editable path and `.plans/**` as read-only. The epic's table of teams gives the Stack team `deploy/` only. `plan.md:16` says Docker access was denied during planning.
- Failure scenario: The builder of item 1 has no editable file for its result, and `proof.sh`, `proof.test.sh` and `bao.hcl` are deleted afterwards, so the criteria 1a and 1b name tests that nobody can re-run at the exact commit. Meanwhile Docker is available on the host used for this review, and the two facts have been observed (see answer a), so the plan's treatment of them as unproven is out of date.
- Fix: Have the orchestrator or planner paste the observed result into `scouting.md` (the output lines and digests are in answer a below) and mark item 1's proof as run. If a retained proof is wanted, put it under `deploy/test/` as `proof-storage-and-seal.sh` so the Stack team may own it, and name that file in criteria 1a and 1b.

### critic-r1 7. Work item 5 is rated medium but holds a six-file recovery state machine, four tests and a compose service

- Severity: MINOR.
- Tied to: plan.
- Evidence: `plan.md:292` rates item 5 medium. `plan.md:200-217` describes `admin/init-response.json`, `admin/recovery.key`, `admin/root.token`, `admin/server-secret-id.pending`, `admin/server-openbao.pending.json` and `admin/root.revoked`, four resumable phases, two diagnostics and four tests.
- Failure scenario: One builder must hold the whole recovery state machine in mind, and a failed mutation run says little about which phase is wrong. The review of the code then carries the same load.
- Fix: Split item 5 into 5a (initialize, save the response, recovery key and root token, and the incomplete-state diagnostic; tests `bootstrap_refuses_incomplete_state` and a new `bootstrap_saves_material_before_continuing`) and 5b (stage, provision, revoke, publish, idempotent second run; tests `bootstrap_revokes_root_before_publish`, `bootstrap_is_idempotent` and `bootstrap_resumes_after_root_revocation`). Rate each medium. Give each its own file section in `openbao-init.sh`, or two scripts, so the two builders do not write the same file at the same time.

### critic-r1 8. The shared test library is created by item 2 but used by items that may run alongside it

- Severity: MINOR.
- Tied to: plan.
- Evidence: `plan.md:115` lists `deploy/test/vault-testlib.sh` among the files of item 2. `plan.md:185` has item 4 "use the fixture pattern from work item 1 so the test can run before item 2". `plan.md:291` says item 4 can run alongside items 2 and 3. Items 3, 5 and 6 also need fixtures, and the loop freezes tests.
- Failure scenario: The tests of item 4 need start-up helpers for a disposable vault. If they are written in `vault-testlib.sh`, two items write one file at the same time. If they are written elsewhere, the helpers exist twice. Later items need new helpers in a file that was frozen when item 2 merged.
- Fix: Make a separate first item that only creates `deploy/test/vault-testlib.sh` with the functions every test file uses, named in the plan (create project, start PostgreSQL, start OpenBao, run as user, cleanup). Make items 2 and 4 depend on it, and give later items their own helper files.

### critic-r1 9. The plan does not say how the server credential is replaced

- Severity: MINOR.
- Tied to: D9.
- Evidence: `plan.md:206` says the secret identifier lasts until revoked and "can be replaced by an administrator using the retained recovery share after the vault is open". `plan.md:187` sets `secret_id_ttl=0` and `secret_id_num_uses=0`. `plan.md:225` mentions the recovery share only as something to protect.
- Failure scenario: The credential file leaks, is corrupted, or is deleted. The plan states that replacement is possible and names no command, no script, no test and no README step. With the root token revoked, the person has to work out `generate-root` with the recovery share and then a second provisioning by hand. In the meantime item 5's published-file check fails closed.
- Fix: Either add a small subcommand (for example `bash deploy/up.sh --rotate-server-credential`) with a test, or record in `decisions.md`, with a date, that rotation is outside sprint 1 and delete the sentence at `plan.md:206`. In both cases add the exact operator steps to `deploy/README.md`.

### critic-r1 10. Images are pinned by tag only, the helper image installs unpinned packages, and a direct `compose up` creates directories

- Severity: MINOR.
- Tied to: R10.
- Evidence: `plan.md:115` and `plan.md:141` pin `postgres:16.16-alpine` and `openbao/openbao:2.7.3` by tag. `plan.md:185` adds `curl` and `jq` to the helper image without versions. `decisions.md:14` promises to record resolved digests but the compose file does not use them. `plan.md:123` mounts single files into containers. `deploy/test/checks.sh:633` runs `compose up -d --force-recreate` directly on the compose file. Health check interval, timeout, retries and start period are not given.
- Failure scenario: On 2026-11-26 `postgres:16.16-alpine` resolved to a digest that was newer than the local copy, so the tag moves. A rebuild a month later can change the image under a recorded proof. A person who runs `docker compose up` on a fresh checkout, before `up.sh`, makes Docker create root-owned directories at the password file paths, and `stack-prepare` then rejects them. `up.sh` also builds the helper image on every run, so the second run needs the network if the layer cache is gone.
- Fix: Pin both images by tag and digest in `compose.yaml` and in the Dockerfile `FROM` line, and take the digests from the observed proof. Pin the package versions of `curl` and `jq`, or use a base that already has them, and say that only `up.sh`'s first run builds. Write the mounts in the long form with `create_host_path: false`, so a direct `compose up` on a fresh checkout fails with a clear message. Give the health check numbers (interval, timeout, retries, start period) so `--wait-timeout 60` is justified.

### critic-r1 11. The launcher and the tests disagree with the existing stack checks and with each other on small points

- Severity: MINOR.
- Tied to: C13.
- Evidence: `deploy/test/checks.sh:356` requires `deploy/up.sh` to be executable and runs it directly, while `plan.md:221` calls it as `bash deploy/up.sh`. `plan.md:139` sets `disable_mlock = true`. `plan.md:149`, `plan.md:196`, `plan.md:217` and `plan.md:233` accept tests that report "the test names with `PASS`", with no output format.
- Failure scenario: OpenBao 2.7.3 prints `unknown or unsupported field disable_mlock found in configuration` and ignores the setting, as I saw in the logs of my runs. A builder who reads the warning removes the line or opens an issue for nothing. `checks.sh` reports "deploy/up.sh is missing or is not executable" if the file mode is not set. Test writers choose three different result formats, and the lead cannot compare them across items.
- Fix: Add "the file `deploy/up.sh` is executable" to item 6 and a check for the mode to `stack-vault.sh`. Delete `disable_mlock` from the plan and say that the container runs without the capability. Pick the output format of `checks.sh` (one Test Anything Protocol line for each test) and write it once in the plan, as the form of every acceptance line.

### critic-r1 12. Some paragraphs break the writing rules of `AGENTS.md`

- Severity: MINOR.
- Tied to: plan.
- Evidence: `plan.md:125`, `plan.md:137`, `plan.md:187`, `plan.md:204` and `plan.md:206` each hold 11 to 15 sentences and about 1,000 characters in one paragraph, with several claims in a sentence. `plan.md:179` uses "AppRole" with no explanation.
- Failure scenario: The reader of the plan is a person with ADHD on a black-and-white terminal. A paragraph that holds a whole state machine cannot be checked by that reader, and a reviewer cannot tie one sentence to one test.
- Fix: Turn the five paragraphs into numbered lists or tables with one claim in each line, and explain "AppRole" once at its first use. No model or harness is named, and the plan keeps the private-data rule, so nothing else needs to change.

## Answers to the attack questions

Lens file:

1. A test writer who never sees the code cannot write the tests of items 2, 5 and 6 as they stand, because five hooks are missing (finding 2). Item 5 also has no output format for "PASS" lines (finding 11). Items 1, 3 and 4 have enough names, inputs and expected results, with the exception of host-side file inspection (finding 4).
2. Every acceptance observation names a test or a command with a result. The weak spots are the bare phrase "reporting the test names with `PASS`" (finding 11), "both services become healthy" in the rollout order (`plan.md:46`), and "healthy" without health check numbers (finding 10).
3. Contradictions: "authenticated" probe against the trust setting of the image (finding 5); `bash deploy/up.sh` at `plan.md:221` against the executable check at `deploy/test/checks.sh:356` (finding 11); item 4 "alongside item 2" at `plan.md:291` against the shared library created by item 2 at `plan.md:115` (finding 8). The decisions log (rows 7 to 17) and the plan agree with each other. The plan keeps D4, D9, D10 and C13, and no row of the design is broken.
4. Every item names exact files. Item 1 appends to `scouting.md`, which the Stack team may not edit (finding 6). All other files are under `deploy/`. The owner and mode of `bao/` and the state root are missing (finding 4).
5. Item 5 is rated medium and should be split (finding 7). Order: item 1 is not needed by item 2 in fact, but it costs nothing. Items 2 and 4 may run together and touch the same test library (finding 8). Items 2, 3 and 5 each edit `deploy/compose.yaml`, but their `Depends on` column keeps them in sequence.
6. Gap states: a fresh checkout works through `up.sh` only, and a direct `compose up` creates directories (finding 10); no PostgreSQL is covered by `--wait` and a 60-second timeout with no message of its own (finding 3); a missing seal key has a diagnostic and a restore guide; a missing password file or credential file and a half-finished PostgreSQL initialization have none (finding 3); a second run has a defined result (see answer b); an interrupted vault initialization has a diagnostic and no way out (finding 3).
7. Gates: gate 1 passes, because rows 7 to 17 are `unanswered` with assumptions and none is open. Gate 2 passes with the format weakness of finding 11. Gate 3 is not triggered, because no screen changes. Gate 4 passes: 13 source items map to existing workstreams. Gate 5 passes, but the risk rows of WS-2 and WS-3 do not name the dead ends of finding 3. Gate 6 passes: one rollout order, and each Verify line names tests and says no device is needed. Gate 7 is pending: `attack/` holds this round only, and the attack log has one pending row. Gate 8: I spot-checked three anchors. `design/openpencil/compose.yml` exists and has `volumes.caddy-data` at line 24, `floorplan/plan.json` has `deploy/openbao.hcl` at line 1629 and `deploy/openbao-init.sh` at line 1640, and `scripts/check-private.mjs` exists. All three hold. The other targets are new files, and `deploy/` has only `deploy/test/checks.sh` and `deploy/test/README.md`. Gate 9 passes. Gate 10 passes on names, because the fold names no model, with the sizing weakness of finding 7.
8. The plan names no model and no harness (it says "vendor" only for seat counts). The private-data rule holds: no home path, no network address and no quotation. The writing rules are broken by five long paragraphs (finding 12). The acronyms TCP, TLS, HTTP, JSON and SQL are expanded at their first use.

Task file:

a. The plan's own `scouting.md` records no run: Docker access was denied (`scouting.md:31-51`), and the sequence is marked "no runtime output yet" (`scouting.md:55`). So the plan did not observe the two facts. I ran the sequence of `scouting.md:60-188` unchanged on 2026-11-26 in a temporary folder, with `bash -x`. It exited 0. The images were `postgres:16.16-alpine` (digest `sha256:f2f98726820d0342055351c6e9d36e705f5bfffb3dd53ff556325114b08afb6d`) and `openbao/openbao:2.7.3` (digest `sha256:ba4b133eb9586630024e71a617e66f37f29cc3534cdd46a6233e098ceded8dc9`). The output lines were `postgres_rows_before=28 postgres_rows_after=31`, `initialized=true sealed=false storage=postgresql value_restored=true`, `missing_key_blocks_unseal=true` and `cleanup=true`, and no container, volume or network of the run was left. Both facts hold: OpenBao keeps its data in PostgreSQL, and a replacement process opens it again from the key file with no call to unseal. The plan follows from the facts, except that the proof uses the superuser and the operator's own user, so it does not test the production roles (finding 1). Two further observations: a key file of 31 bytes makes OpenBao exit with "unknown encoding for AES-256 key", and a wrong 32-byte key leaves the process running and sealed with `bao status` exiting 2, as `plan.md:137` says.

b. A second run of the launcher is defined: the preparer keeps the key and the password files, the init job returns `vault already configured` with no write and no login, and the stack is brought up healthy (`plan.md:204`, `plan.md:223`). A run after a half-finished first run is defined for the vault's own phases (resume from the staged identifiers, `plan.md:204`) and left open for PostgreSQL's first initialization, a lost response of `sys/init`, and lost state files (finding 3). The credential file is `server/openbao.json` under the state folder, by default `deploy/.state/server/openbao.json`, mounted at `/run/lgtm/openbao.json`, owner and group 10001, mode 0400, one UTF-8 JSON object with exactly the string fields `role_id` and `secret_id` and a trailing newline (`plan.md:206`). The path, format and mode are exact. Its replacement is not defined (finding 9), and the files around it are not readable by the operator (finding 4).

c. Images are pinned by tag, not by digest, and the helper image installs unpinned packages (finding 10). No host port is published for either service, which is stricter than loopback only, and the web port comes in sprint 2. Health checks exist for PostgreSQL (`plan.md:125`, but not authenticated, finding 5) and for OpenBao (`bao status`, `plan.md:141`) and have no timing values. The order is set by `depends_on` with `service_healthy` for `openbao` on `postgres`, and by the launcher for the preparer and the init job (`plan.md:223`). PostgreSQL holds two databases and two roles apart, `lgtm_app` and `lgtm_bao`, and the plan leaves out who owns them (finding 1).

## Post-build checks

1. PostgreSQL stopped while OpenBao restarts. Scenario: stop `postgres`, restart `openbao` with `max_connect_retries=0`, wait, start `postgres`. Expected: the container exits or stays up with a clear message, the restart policy brings it back, and it is unsealed within 60 seconds of PostgreSQL's return. I saw OpenBao still running six seconds after a restart with PostgreSQL stopped and did not follow it further.
2. Wrong key, present and 32 bytes. Scenario: replace `bao/unseal.key` with other random bytes and run `up.sh`. Expected: a clear message from `up.sh`, not only a timeout of the health wait, and no change to the files. My run shows the process stays up sealed and retries.
3. `checks.sh` C13 against the built stack. Scenario: run `deploy/test/checks.sh --only C13` with only `postgres` and `openbao` running. Expected: the vault half passes. It reads `bao status` without an address, so `BAO_ADDR` must be set in the service.
4. Batch tokens and the policy. Scenario: log in with the AppRole, write `lgtm/data/server/link/signing-key` with `options.cas=0` twice. Expected: the first write succeeds, the second returns the conflict, and the token has no default policy so `auth/token/lookup-self` returns 403 for it.
5. Single-file mount after replacement. Scenario: replace `server/openbao.json` by atomic rename while a container has it mounted. Expected: the container still sees the old inode until it is recreated, as the plan says.
6. Operator teardown. Scenario: run two test projects as a non-root operator, tear them down, list the temporary directory. Expected: nothing left (finding 4).

## Grade

5 out of 10. Counts: 0 blockers, 4 majors, 8 minors.
