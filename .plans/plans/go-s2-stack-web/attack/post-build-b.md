# REVISE

The launcher can print the web password if the settings file changes after its preflight while the launcher waits for the state lock.

## Findings

### post-b 1 — MAJOR — R9

Rule 9 of the foundation design keeps secrets out of logs. The launcher discards Compose's diagnostics at preflight, then acquires the lock and calls the database helper, which rereads the same settings file and passes Compose's diagnostics through. [Rule 9](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L370), [launcher preflight and lock](../../../../deploy/up.sh#L43-L55), [database phase](../../../../deploy/up.sh#L77-L81), [database helper](../../../../deploy/stack-prepare.sh#L97-L103).

In a disposable checkout, I held the state lock and started `deploy/up.sh --until vault` with valid settings. After preflight reached the lock, I saved a password with a missing closing quote and released the lock. The launcher printed `phase database start`, then Compose's error included the synthetic password, and the launcher exited 1. The existing test writes the malformed file before starting the launcher, so it exercises the preflight path. [Malformed-settings test](../../../../deploy/test/stack-up.sh#L381-L393).

Use a private settings snapshot for preflight and every later Compose call, including the database helper. Add a regression that changes the original file while the launcher waits for the lock and checks the captured output for the marker.

### post-b 2 — MINOR — plan

Work item 1 specifies that the runtime stage adds one static server program to the base image. The frozen suite checks for the planted secret markers, the entry point, the user, the absence of a shell and the missing-password error. A scratch build that also copied `/src/go.mod` into `/usr/local/share/go.mod` passed all six assertions with the same stand-in used for the baseline. The extra file therefore ships undetected by this gate. [Server image plan](../plan.md#L89-L107), [runtime stage](../../../../deploy/server.Dockerfile#L16-L20), [image assertions](../../../../deploy/test/server-image.sh#L69-L117).

Check the paths added to the final pinned base and require that the server program is the only added file.

## Grade

8 out of 10.

## Post-build checks from the final review round

The review used the deployment code at `12b12a47`, Docker Engine 29.7.2 and Docker Compose 5.5.0. The live checks used disposable projects.

| Check | Observation |
|---|---|
| PB1, the clean launch and joint sign-in gate. | The cold-cache driver run and its deadlines remain pending on the real server from Server item 9. The reviewed snapshot lacked `src/server/cmd/lgtm-server/main.go`. The alternate-port driver run also depends on request D5 in the plan. |
| PB8, a malformed settings file. | Both launcher modes exited 2 and printed `the settings cannot be read; check the quoting in deploy/.env`. Each captured output had zero phase lines and zero occurrences of the synthetic marker. The lock-wait variant is finding post-b 1. |
| PB9, password removal. | Compose resolved an empty password after file removal and kept the password when exported in the shell. Against a stand-in, the exported value kept the same server container and the launch exited 0. After unsetting it, the launch recreated the server and exited 1 with the missing-password message, and the container was stopped. Restoring the password let the launcher exit 0 again. Committed revocation, old-cookie rejection and the database-failure scenario require the real server. |
| PB10, project and port selection. | With conflicting project and port settings in the root file, the full launcher with a stand-in selected the disposable project from `deploy/.env` and served the website at port 18787. The guide's Compose lookup printed `127.0.0.1:18787`. The root file's decoy project had zero containers. |

## Frozen suites and scratch mutations

| Command | Observed output |
|---|---|
| `bash deploy/test/stack-web.sh` | Nine `PASS` lines and `9 passed, 0 failed`. |
| `bash deploy/test/stack-up.sh` | Six `PASS` lines for the checks runnable without the real server, including the slow start, failed-server stop, taken port and malformed settings checks. |
| `bash deploy/test/stack-vault.sh` | `66 passed, 0 failed`. |
| `bash deploy/test/server-image.sh` | Both static assertions printed `PASS`. With a tracked stand-in in the scratch build context, the suite printed `6 passed, 0 failed`. |
| `bash deploy/test/link-stream.sh --rules-only` and `bash deploy/test/link-stream.sh` | Each printed `6 passed, 0 failed, 1 skipped`. |
| `LGTM_STREAM_SOAK=1 bash deploy/test/link-stream.sh` | `PASS [stream_survives_soak]` and `7 passed, 0 failed, 0 skipped`. |

All three Compose mutations tripped their named assertions. A wildcard publication failed `web_publishes_on_loopback_only`, adding the edge network to the server failed `server_has_no_route_out`, and removing the private-stack setting failed `server_settings_are_named`.

Moving the image phase first and removing the fixed vault port each failed `up_until_vault_needs_no_web_settings`. Shortening the server wait to 120 seconds failed `up_waits_for_a_slow_server`. Removing the stop failed `up_stops_on_a_failed_server`. Showing the preflight error failed `up_keeps_the_password_out_of_its_output`.

Removing the leading ignore pattern failed `server_image_leaves_out_state` and the exact allow-list assertion. Removing `USER` failed `server_image_runs_unprivileged`. Both image mutations ran with the same stand-in.

The stream repair passed a scratch test that forces a response write to fail while the request body still has a third line for the scanner to send. The reader goroutine exited with the repair and remained blocked with the previous version. The launcher syntax check and the stream probe format, vet and package checks passed. [Stream cancellation](../../../../deploy/test/streamprobe/main.go#L172-L193).
