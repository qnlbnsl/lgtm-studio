# REVISE

With both `COMPOSE_FILE` and `COMPOSE_ENV_FILES` set, the launcher can start PostgreSQL in one project and run database preparation in another.

The builder was an OpenAI worker from my own vendor. I reviewed commit `c4fca798` on `sprint-0`. [Builder's result](../../../../.foundry/go-rewrite/results/s2-web-build-d.md).

## Findings

### post-d 1 — MAJOR — C1 and plan

Confirmed live. An explicit `COMPOSE_ENV_FILES` list leaves the launcher's resolved model empty. The launcher follows the `COMPOSE_FILE` list, but its preparer falls back to the base file. With an override that sets the top-level project name and a settings file that supplies only a port, the live vault-only launch created the base project's data volume, started PostgreSQL in the override project, and exited 1 when the preparer could not reconcile the database. The port lookup uses the override project. If the base project already has a PostgreSQL container, reconciliation can target that container. Pass the same Compose file selection and settings to the preparer for every launcher invocation. Preserve the preparer's established base-file default when it is run directly. [Launcher selection and preflight](../../../../deploy/up.sh#L35), [preparer call and database phase](../../../../deploy/up.sh#L145), [preparer fallback](../../../../deploy/stack-prepare.sh#L97), [guide's lookup](../../../../deploy/README.md#L109), [plan item 5](../plan.md#L236), [design check C1](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L378).

### post-d 2 — MAJOR — R9

Confirmed live with the real Compose parser and a stand-in for container operations. The explicit-settings branch copies the interpolation files but keeps loading the original Compose model. A supported Compose override can point a service's `env_file` at the same original settings file. I changed that file to an unterminated password assignment after the launcher reached the state lock. The later database-phase Compose call reread the file and printed the synthetic password marker. The preflight had already succeeded, and its output suppression no longer applied. Resolve service environment files into the private model for this branch as well, and keep subsequent settings errors behind the fixed diagnostic. [File copy and preflight](../../../../deploy/up.sh#L47), [database-phase Compose call](../../../../deploy/up.sh#L149), [design rule R9](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L370).

### post-d 3 — MAJOR — C1 and plan

Confirmed with real Compose model loads. The first render forces `--profile setup`, which displaces the working-folder `COMPOSE_PROFILES` value. In a disposable override that assigned the server and website to an extra profile selected by the working folder's settings, direct Compose listed both services. The launcher's first render omitted both. Reloading that snapshot with `--profile '*'` could not restore the omitted services, and `build --print server web` reported `no such service: server`. Preserve the requested profiles while capturing the setup services, and check a working-folder profile that enables services named later by the launcher. [First render and reload](../../../../deploy/up.sh#L35), [preflight profile flag](../../../../deploy/up.sh#L72), [image phase](../../../../deploy/up.sh#L162), [design check C1](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L378).

### post-d 4 — MINOR — C1 and plan

Confirmed with real Compose model loads. The port guide tells the caller to change to the repository root and reuse `COMPOSE_FILE`. The frozen test launches from another folder and runs its lookup in that same folder. I used a relative file list and confirmed that the root lookup could not load it. I then used an absolute file list and distinct project names in the two folders' `.env` files. The root lookup selected the root project, while the launcher selected the launch-folder project. Tell users to run the `COMPOSE_FILE` lookup from the launch working directory with the same effective settings. [Port guide](../../../../deploy/README.md#L109), [frozen lookup](../../../../deploy/test/stack-up.sh#L484), [plan item 6](../plan.md#L275), [design check C1](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L378).

### post-d 5 — MINOR — plan

Confirmed live. The interruptible wait covers the state lock, while settings copies are read by a foreground child. I supplied a regular password file followed by a named pipe in `COMPOSE_ENV_FILES`. A termination signal sent to the launcher alone during the second read left it alive with the first settings copy still on disk at mode `0600`. After I unblocked the reader, the launcher exited 143 and removed its copy. A stalled settings source can leave the copy present indefinitely after cancellation. Use interruptible waits and reap the subprocesses that can block while the private settings exist. [Foreground copy](../../../../deploy/up.sh#L59), [exit trap and cleanup](../../../../deploy/up.sh#L86), [interruptible lock wait](../../../../deploy/up.sh#L116).

## Grade

6 out of 10.

## Post-build checks

I used Docker Engine 29.7.2 and Docker Compose 5.5.0. Container operations used disposable projects, state folders, and randomly selected host ports. The environment had a shared Docker cache.

| Check | Observation |
|---|---|
| PB1, the joint stack gate | The driver passed all five checks with a random exported port and again from fresh state with a random port set only in `deploy/.env`. A cold-cache run and the manual browser check remain unverified. |
| PB8, malformed settings | Both launcher modes exited 2 and printed only `the settings cannot be read; check the quoting in deploy/.env`. I checked the default file and the `COMPOSE_FILE` route without an explicit settings list. Each run printed zero phase lines, exposed no marker, and removed its temporary directory. |
| PB9, removal of the password | A live run confirmed that an exported password preserved the session after deletion from `deploy/.env`. Unsetting it and running the ordinary launcher committed revocation and stopped the server. Restoring the same password left the old cookie invalid. The full suite exercised the database-unreachable case and confirmed that revocation succeeded on a later run. |
| PB10, the file-only project and port | The full launcher suite checked the real website with conflicting root and deployment settings. The default launcher and its guide's `-f` lookup used the deployment project and port. With `COMPOSE_FILE` and no explicit settings list, a model probe observed the launcher, preparer, and same-directory lookup selecting the working-folder project. The two project-selection failures are above. |
| Traceability request E1 | The contract plan's second post-build scenario now specifies a loopback join and a ten-minute heartbeat stream. The encrypted connection and wrong-pin cases are assigned to the later proxy work. The combined link gate is a separate sprint 3 check. |

[Register of unreviewed changes](round-4-findings.md#L10), [post-build checks](../plan.md#L364), [contract plan's second check](../../go-s0-contract/plan.md#L874).

In the timed file-only run, the driver completed in 53 seconds. Docker events showed that the C2 server exited 0.36 seconds after it started. Its recovery recreation took 2.56 seconds from container creation to healthy. In C4, the original server answered through the website within 1.7 seconds of restart. The replacement and recovery containers reached healthy in 3.00 and 2.89 seconds from creation. In C13, 12.52 seconds elapsed from the first container creation to all four services reporting healthy.

A signal to the launcher alone during a genuinely blocked state-lock wait worked in both settings branches. Hangup, interrupt, and termination gave statuses 129, 130, and 143. Each reaped the lock child and removed the private copy while the fixture still held the lock. The files had mode `0600` and the containing directory had mode `0700`. The joint gate left no snapshot. A real vault-only launch also reached readiness after its working-folder settings became malformed while it waited for the lock. It printed no password and removed its copy. The healthy live launch and the three direct preparer commands emitted no occurrence of the fixture's password. [Lock cleanup](../../../../deploy/up.sh#L86), [lock wait](../../../../deploy/up.sh#L116).

Direct calls of `stack-prepare.sh prepare-db`, `reconcile-db`, and `seal` each exited 0 against a live disposable stack. A model probe confirmed that the standalone preparer still uses the base file when an unrelated `COMPOSE_FILE` list and an explicit settings list are in the shell. [Standalone Compose wrapper](../../../../deploy/stack-prepare.sh#L97).

## Frozen tests and scratch mutations

| Command | Output |
|---|---|
| `bash deploy/test/stack-web.sh` | Nine `PASS` lines, then `9 passed, 0 failed`. |
| `bash deploy/test/stack-up.sh` | Twelve `PASS` lines, then `12 passed, 0 failed`. |
| `bash deploy/test/stack-vault.sh` | Both named assertions passed, followed by `66 passed, 0 failed`. |
| `deploy/test/checks.sh --only C1,C2,C3,C4,C13` | The driver printed `ok 1`, `ok 2`, `ok 3`, `ok 4`, and `ok 13`. |

The frozen assertions allow the wrong implementation in post-d 1. Its ordinary fixtures export a project name that overrides the Compose files' names. The assertion that gives the working-folder file control of the project unsets `COMPOSE_ENV_FILES`. A split between the preparer and the launcher therefore escapes both sets of fixtures. The frozen lock-wait test now requires status 0 and the vault-ready line after the settings edit, closing the test gap recorded as post-c 3. [Fixture environment](../../../../deploy/test/web-testlib.sh#L17), [working-folder test](../../../../deploy/test/stack-up.sh#L406), [completion assertions](../../../../deploy/test/stack-up.sh#L550), [earlier review](post-build-c.md).

The plan's mutation that restores raw Compose diagnostics failed `up_keeps_the_password_out_of_its_output`. In independent scratch reversions, restoring foreground `flock` failed `up_cleans_up_settings_when_terminated_while_waiting_for_the_lock`, and restoring the previous settings selection failed `up_uses_the_working_folder_settings_with_compose_file`. Each selected assertion exited 1 and printed `0 passed, 1 failed`. [Plan mutation](../plan.md#L259), [frozen assertions](../../../../deploy/test/stack-up.sh#L386).

The preliminary archive lacked the Git metadata that the web and launcher suites read. Their reported results are from a clean Git checkout. I set Buildx's local configuration directory to a writable scratch path after the default path proved read-only. A separate vault probe needed the checked-in deployment file modes so that the unprivileged OpenBao process could read its mounted files. The corrected probe passed.
