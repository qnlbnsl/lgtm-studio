# Post-build review A of plan go-s3-stack-client, item 2 (the installer on Linux)

- Task identifier: `postbuild-go-s3-stack-client-a`
- Nonce: `aff523`
- Reviewer: claude-sonnet-5-5, high. I did not build this code and did not write its tests.
- Code read: `git diff fb3f7f68..HEAD -- deploy/install-client.sh deploy/lgtm-client.service deploy/test/installer.sh`. Only `deploy/install-client.sh` changed in that range. The unit template and the frozen test were already in `fb3f7f68`, and the test's hash equals `frozen-s3installer-a.sha256`.
- I never ran the installer against this host. Every run below went through the frozen test's stand-ins, or through a copy of the test and the installer under a folder made by `mktemp -d`.

## Verdict: REVISE

The installer matches steps 1 to 18 of the plan, and all 21 tests pass, but criterion 2e and its task 2.3 are done by no one, and the user-flow script still points the clients at a variable that the client ignores.

## Top finding

`deploy/test/checks.sh` lines 219 and 233 still set `XDG_STATE_HOME`, which the client ignores, so a run of the user-flow script joins its clients into the real `$HOME/.local/state/lgtm-client` of the person who runs it (post-a 1).

## What the tests print

`bash deploy/test/installer.sh` exits 0 and ends with `222 passed, 0 failed`. All 21 tests print `ok`. Test 18 prints `SKIP` for its Darwin part, because `deploy/dev.lgtmstudio.client.plist` belongs to item 3. The run takes about 19 seconds with no Docker, no network and no service manager. I did not run the user-flow script or any other team's tests.

## Check 1: the places no reviewer read before the build

| Place | Scenario | Result |
|---|---|---|
| Step 11, a unit whose folder holds no identity (logic-r4 03) | PB6 | Run through the stand-ins by the frozen test, in its fifth run. It exits 1 with `holds no identity`, runs no `go`, `exec` or `client join`, and leaves everything unchanged. I added one variant: a unit exists and its folder holds `machine.key` only. It also gives `holds no identity`, not `did not finish`. That is allowed by the plan's wording, and it is noted in post-a 7. PB6 on a Mac and on a real user manager was not run, because the macOS branch is item 3 and the host is off limits. |
| Rerun test, fourth and fifth runs (logic-r4 03 and 04) | PB7 | Run through the stand-ins by the frozen test: exit 1 with `--remove first`, and unit, program and both folders unchanged. A real user manager was not used. |
| `SHIM_EXEC_FAIL` and `refuses_docker_failure` (logic-r4 05) | PB8, installer part | Passes. A token command that exits 1 after writing the marker on both streams gives `returned no token`, and the marker is not in the output. The `compose` function discards Compose's standard error (`install-client.sh:12`). Gate G1 needs the real server command and is not mine. |
| PB1, PB2 (process list) and PB3 | Need the real client, the real stack or another user | Not run. The real `lgtm-client join` is still a stub that prints "not built yet" (`src/client/cmd/lgtm-client/main.go:19`). For PB2 the frozen test runs `bash -x` with a made-up token and the trace stays clean. For PB3 the installer checks no ancestor of `$HOME/.local/bin` or of the unit's folder. That is as the plan states. The client checks the ancestors of its own state folder (`src/client/internal/state/folder.go:166`). |

Work item 4's places (the `--cleanup` rules, PB4, and the gate rows G1, G4 and PB5) do not belong to item 2 and I did not review them.

## Check 2: the code against the plan

Steps 1 to 18 and the unit template match the plan, with these differences:

- Step 3: `Darwin` gives `The macOS installer is not available yet.` This is the stub for item 3 and is correct for now.
- Step 6 and step 7: the code runs `--remove` before it reads the state folder (`install-client.sh:170`). The plan says the same in effect, because `--remove` ignores the variable.
- Criterion 2e and task 2.3 are not met (post-a 1). The builder's result file says they belong to the test writer, but the plan lists `checks.sh` and the README row among item 2's files.

## Check 3: the tests

I copied the installer, the template, `compose.yaml` and the frozen test into six trees under one `mktemp -d` folder and changed one thing in each.

| Mutation | Suite result |
|---|---|
| Control, no change | 222 passed, 0 failed. |
| Print the token before the join (the plan's mutation) | 3 failed, all in `token_never_printed`. |
| Install the program before the join (the plan's second mutation, as run by the builder, not repeated by me) | The builder reports 8 failed in `joins_before_service_starts`. |
| Remove the call to `wait_for_service` | 222 passed, 0 failed (post-a 2). |
| Remove the redaction of the token from the client's output (line 97) | 222 passed, 0 failed (post-a 3). |
| `WantedBy=multi-user.target` in the template | 222 passed, 0 failed (post-a 4). |
| `Type=simple` in the template | 222 passed, 0 failed (post-a 4). |

## Findings

### post-a 1, MAJOR: the user-flow script and its README row still use `XDG_STATE_HOME`

- Statement: `plan`, criterion 2e, task 2.3 and work item 2, "The change of the user-flow script"; D19.
- Evidence: `! grep -q XDG_STATE_HOME deploy/test/checks.sh` exits 1. `deploy/test/checks.sh:219` runs `XDG_STATE_HOME=$1 "$client_bin" join`, and `deploy/test/checks.sh:233` runs `XDG_STATE_HOME=$state_home "$client_bin" run`. `deploy/test/README.md:81` still describes `XDG_STATE_HOME`. `src/client/internal/config/config.go:27` takes only `LGTM_CLIENT_STATE_DIR` or `$HOME/.local/state/lgtm-client`, and its test `TestStateDirRule` shows that `XDG_STATE_HOME` is ignored. The builder's result file says it "belongs to the user-flow test writer".
- Failure scenario: a person runs `deploy/test/checks.sh` (gate G4) on a machine. Its clients join the disposable test server into the real default state folder. C10 then reads `$state_home/lgtm-client/identity.json`, which does not exist, and fails. The default folder now holds an `identity.json` for a server that is gone, so the next `install-client.sh` takes the `already joined` path and installs a client that can never connect.
- Fix: change the two lines to `LGTM_CLIENT_STATE_DIR=$1/lgtm-client` and `LGTM_CLIENT_STATE_DIR=$state_home/lgtm-client`, change the README row, and name an owner for the change in the foundry's task list.

### post-a 2, MINOR: no test fails when the installer never waits for the service

- Statement: `plan`, step 17 and the rule "A successful run means that the service manager started the client".
- Evidence: the `systemctl` stand-in answers every `is-active` with `active` (`deploy/test/installer.sh:214`), and no knob makes it answer anything else. Replacing the call at `install-client.sh:241` with `:` gives 222 passed.
- Failure scenario: an installer that prints its success sentence when the unit failed to start is accepted by every test. Only the live test, which does not exist yet, would see it.
- Fix: add a stand-in variable that makes `is-active` print `failed`, and a test that expects exit 1 and `did not start` after about 10 seconds.

### post-a 3, MINOR: no test fails when the installer prints the client's output without redaction

- Statement: R9 and criterion 2b.
- Evidence: the stand-in client's failure output is the fixed line `{"msg":"join_refused","code":"token_used"}` and never contains the token (`deploy/test/installer.sh:193`). Deleting `install-client.sh:97` (`join_output=${join_output//"$token"/[redacted]}`) gives 222 passed.
- Failure scenario: a client that repeats its arguments in an error (a usage line, a Go error with the address and token) reaches the owner's terminal and the owner's shell history or log with the token in it, and the suite stays green.
- Fix: let the stand-in echo its arguments when a variable is set, and assert that `token_never_printed` still holds. Redaction by exact match also misses a token printed in pieces. Say so in the guide, or print only a fixed sentence plus the client's exit status.

### post-a 4, MINOR: the unit template's own lines are not pinned by any test

- Statement: D16 and the plan's "The unit template" paragraph; risk "The service runs after a login only".
- Evidence: `unit_text_equals_template` (`deploy/test/installer.sh:351`) reads the template from the tree under test, so a changed template always equals itself. Only `Restart=`, `RestartSec=` and `StartLimitIntervalSec=` are checked on their own (lines 373 to 375). `WantedBy=multi-user.target` and `Type=simple` both give 222 passed. `ExecStart=`, `TimeoutStopSec=` and the `[Install]` section are not asserted either. The build flags `CGO_ENABLED=0` and `-trimpath` are not asserted, because the stand-in `go` only logs its arguments.
- Failure scenario: a later edit that breaks `WantedBy=default.target` makes `enable` do nothing at boot, and the client no longer comes back after a restart. The hermetic suite stays green, and only `live_service_enabled_with_linger` would catch it, once it runs.
- Fix: assert the exact line set of the template, or `WantedBy=default.target` and `Type=exec` by name, in `installs_linux_user_service`.

### post-a 5, MINOR: `XDG_CONFIG_HOME` is not checked and a relative value writes the unit into the checkout

- Statement: step 5 and the plan's "Each refusal ... changes no file"; D16.
- Evidence: `install-client.sh:169` sets `unit_file=${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user/lgtm-client.service`, and the script changed to the repository root at line 153. In a scratch copy, `XDG_CONFIG_HOME=relcfg` exited 0 and wrote `<repository>/relcfg/systemd/user/lgtm-client.service`.
- Failure scenario: a relative `XDG_CONFIG_HOME` is ignored by a real systemd user manager, so `enable` fails after the join and after the program is installed. The owner has a joined, waiting machine, a program, no service, and a stray `relcfg/` folder in the checkout. A rerun recovers, because the identity exists, but the message is the generic `systemctl --user enable lgtm-client.service failed.`
- Fix: apply `safe_absolute_path` to the folder, or ignore a value that is not absolute the way systemd does, before step 6.

### post-a 6, MINOR: the service's search path keeps empty and relative entries

- Statement: plan step 6 (the filter names characters only); D19.
- Evidence: the pattern at `install-client.sh:42` is `^[a-zA-Z0-9._/+@-]*$`. It accepts the empty entry, `.` and `bin`. With `PATH=<shims>::.:<tools>:` the unit got `Environment=PATH=<shims>::.:<tools>:`.
- Failure scenario: a user manager runs the client with its working folder at the user's home. An empty or `.` entry then makes the client's lookup of a harness program find a file in the home folder before the real one.
- Fix: drop empty entries and entries that do not start with `/`, and state it in step 6.

### post-a 7, MINOR: the state-folder messages and the comparison are too coarse

- Statement: plan step 6, and R8 (one useful sentence).
- Evidence: `install-client.sh:179` compares the requested folder with the unit's folder as plain text. A scratch run with the unit at `<T>/st` and `LGTM_CLIENT_STATE_DIR=<T>/st/` gave `names another folder than the installed service; run install-client.sh --remove first.` A unit with its state line deleted gave `LGTM_CLIENT_STATE_DIR must be an absolute path.`, although the variable was not set.
- Failure scenario: the owner removes and reinstalls for a trailing slash, which is the case the plan says should keep the identity. A hand-edited unit gets a sentence about a variable the owner did not set.
- Fix: strip trailing slashes before the comparison, and give the unreadable-unit case its own sentence.

### post-a 8, MINOR: a relative `COMPOSE_FILE` is read from another folder than the launcher's

- Statement: the plan's rule "The installer calls Compose as the launcher does", architect-r2 04.
- Evidence: `install-client.sh:153` runs `cd` to the repository root before any Compose call, and `deploy/up.sh` makes no `cd` (its `compose` is at line 19 of that script). I did not run Docker, so this comes from reading the two scripts.
- Failure scenario: an owner who started the stack from `deploy/` with `COMPOSE_FILE=compose.yaml` runs the installer and gets `docker compose could not read the stack`, because the file is now looked up in the repository root. The reverse can read a different file with the same name.
- Fix: resolve a relative `COMPOSE_FILE` before the `cd`, or do not `cd` and use `$repo` paths, as `deploy/up.sh` does.

## Check 4: defects of the code itself

I found no race, no leaked temporary file and no secret in a log or an error text. The traps remove the temporary program and unit file on every exit, `build_failure_stops_installer` proves it, and `bash -x` shows no token. The token is an argument of `lgtm-client join`, which the plan accepts. The temporary files are mode 0600, the program is 0755 as the plan says, and the unit file is 0600. The installer creates no secret file. Two things are left to others: the client's own state folder is created with mode 0700 by the client (`folder.go:182`), and two runs at the same time are outside wave 1.

## Check 5: the design

The findings above are tied to `plan` or to D16 and D19. R9 is post-a 3, and R8 is post-a 7. C6 (a client joined with a token the server's command makes) cannot be run until the server's command, the client's `join` and the stack exist, and it belongs to gate G4.

## Grade: 7 of 10

The installer is careful and its failure paths are real. It loses points for the missing change of the user-flow script, which can put a wrong identity into a real home folder, and for a suite that accepts three plausible wrong installers.
