# REVISE

When `COMPOSE_FILE` is set, the launcher can select a different project and port from the equivalent Docker Compose command.

## Findings

### post-c 1 — MAJOR — C1

By reading, the snapshot code selects the environment file beside the first Compose file whenever `COMPOSE_ENV_FILES` is unset. With `COMPOSE_FILE` set, the launcher calls Compose without `-f`, and that entry point gives the working folder's `.env` precedence. The new snapshot changes the project, port, and password that the launcher uses. [Launcher invocation](../../../../deploy/up.sh#L35), [snapshot selection](../../../../deploy/up.sh#L45), [plan item 5](../plan.md#L236), [design check C1](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L378).

I confirmed this live with a disposable working folder and deployment folder with different project names and ports in their settings files. With `COMPOSE_FILE=../deploy/compose.yaml`, the previous launcher's preflight and a direct Compose configuration lookup chose the working folder's project. The repaired launcher used the deployment folder's project and port in all 15 Compose calls. The probe rendered the models through Compose and stood in for container operations. An operator can therefore start one project and look up the port in another. Capture settings with Compose's working-folder and project-folder precedence for this entry point, and test that the launcher and the documented lookup select the same project. [Prior Compose precedence probe](../scouting.md#L269), [lookup specified by the plan](../plan.md#L275).

### post-c 2 — MAJOR — plan

I confirmed live that a termination signal sent to the launcher process alone leaves the password copy on disk while the state lock is held. Bash defers the signal trap while the foreground `flock` is blocked. Releasing the lock let the process exit 143 and remove the copy. Sending the same signal to the whole launcher process group removed it while the lock was still held. A second launch cancelled by its process identifier can retain the snapshot as long as the first launch holds the lock. Use an interruptible wait and have cancellation stop and reap the child that the launcher started. Test process-only cancellation with the lock still held. [Cleanup traps](../../../../deploy/up.sh#L69), [lock invocation](../../../../deploy/up.sh#L85), [builder's cleanup claim](../../../../.foundry/go-rewrite/results/s2-web-build-c.md#L7).

### post-c 3 — MINOR — plan

By reading, the frozen lock-wait test accepts an early exit after a silent reread of the changed settings. It checks for an exit status below 124 and for the absence of the marker. In scratch I added a second, silent Compose preflight against the original settings after the lock, with an exit status of 2 on error. The malformed edit aborted that launch, and the full launcher suite still printed `10 passed, 0 failed`. Require status 0, the vault-ready line, and the absence of the marker after the edit. [Frozen assertion](../../../../deploy/test/stack-up.sh#L431), [plan criterion 2b](../plan.md#L180).

## Grade

7 out of 10.

## Post-build checks

I reviewed commit `af4473dc` on `sprint-0` with Docker Engine 29.7.2 and Docker Compose 5.5.0. Live runs used disposable project names and ports.

| Check | Observation |
|---|---|
| PB1, clean launch and rollout | The driver printed `ok` for C1, C2, C3, C4 and C13 and exited 0. The run used fresh state, a shared Docker cache, and a random exported port. The cold-cache run remains pending. The plan places the file-only driver run after request D5. |
| PB8, malformed settings | Both launcher modes exited 2 with `the settings cannot be read; check the quoting in deploy/.env`. Each run printed zero phase lines and zero occurrences of the synthetic marker. |
| PB9, password removal | The real-server assertion passed. An exported password kept the session. Removing the effective password committed revocation and stopped the server. Restoring the same password left the old cookie invalid. With PostgreSQL unreachable from the server, the run reported unconfirmed revocation and stopped the server. A later run committed revocation. |
| PB10, file-only project and port | The real-server suite started the project from `deploy/.env` and the direct `-f` lookup returned its port despite conflicting settings in the checkout root. The test used a randomly selected port. Finding post-c 1 covers the `COMPOSE_FILE` invocation. |

The real vault launch reached readiness after I replaced `deploy/.env` with malformed settings while it waited for the lock. It exited 0, printed no marker, and removed the snapshot. In model probes, every launcher and preparer Compose call used the same snapshot after replacing an existing default file, creating an initially absent default file, or replacing a file in an explicit two-file list. Disabling the default environment file left explicit files active. By reading, the server and web failure paths also use the same Compose wrapper. The snapshot directory was mode `0700`, and both files in the signal probe were mode `0600` with the source at mode `0644`. The snapshot was removed on normal exit, on a malformed preflight, on a state-directory error, and on whole-group `HUP`, `INT`, and `TERM`. [Server and web failure paths](../../../../deploy/up.sh#L128).

I ran `stack-prepare.sh prepare-db`, `reconcile-db`, and `seal` directly. Each exited 0. The database volume had the project name from `deploy/.env` with a conflicting root file present. The helper's Compose wrapper is byte-for-byte the same as at the frozen-test commit. [Standalone Compose wrapper](../../../../deploy/stack-prepare.sh#L97).

## Frozen tests and scratch mutations

| Command | Output |
|---|---|
| `bash deploy/test/stack-web.sh` | Nine `PASS` lines, then `9 passed, 0 failed`. |
| `bash deploy/test/stack-up.sh` | Ten `PASS` lines, then `10 passed, 0 failed`. The added lock-wait test accounts for the tenth check. |
| `bash deploy/test/stack-vault.sh` | Both named assertions passed, followed by `66 passed, 0 failed`. |
| `deploy/test/checks.sh --only C1,C2,C3,C4,C13` | Five `ok` lines, with the website reached on loopback, the missing password refused, sign-in checked, the session ended on password change, and the signing key loaded again after restart. |

The plan's mutation that restores raw Compose output failed `up_keeps_the_password_out_of_its_output`. Restoring the launcher from `55ecf01f` failed `up_keeps_changed_settings_password_out_of_its_output` with `The launcher exposed the password from settings changed during the lock wait.` [Plan mutation](../plan.md#L259).
