# Post-build review B of plan go-s3-stack-client, item 3 (the installer on macOS)

- Task identifier: `postbuild-go-s3-stack-client-b`
- Nonce: `cb712e`
- Reviewer: claude-sonnet-5-5, high. I did not build this code and did not write its tests.
- Scope: `git diff fa1ec251..HEAD -- deploy/install-client.sh` (the plist and the macOS tests are in commit `fa1ec251`, before that range), `deploy/dev.lgtmstudio.client.plist`, and the macOS tests of `deploy/test/installer.sh`. This host is Linux, so the macOS branch ran only through the test's stand-ins. I never ran the installer against this host.

## Verdict

**SHIP.** The macOS branch does what the plan's item 3 says, the two places that no reviewer read before the build (PB6 and PB7, on macOS) behave as the plan expects, and all findings are minor test gaps or on-device doubts.

Top finding: the macOS tests run with GNU tools on Linux, so a macOS branch that uses a GNU-only flag such as `mv -fT` passes every test and would fail on a real Mac (post-b 2).

## Run of the tests

`bash deploy/test/installer.sh` prints `1..25`, then `ok 1` to `ok 25`, then `300 passed, 0 failed`, and exits 0. The four tests of item 3 are `ok 22` to `ok 25`.

## Check 1: the places no reviewer read before the build

Register: `attack/round-4-findings.md`, "Not reviewed before the build". Two of its places belong to item 3: the fourth and fifth runs of `rerun_uses_state_folder_of_installed_unit` on `Darwin` (PB6 and PB7), and step 11 of the behaviour, which item 3's `launch_agent_state_dir` feeds. I ran both scenarios in a scratch copy of the test's harness, with `SHIM_UNAME=Darwin` and `SHIM_UID=501`.

| Check | What I did | Result |
|---|---|---|
| PB6 on macOS | First run, then moved the state folder aside, then ran the installer again. | Exit 1 with `The installed service's folder <folder> holds no identity; ...`. The calls log holds no `go`, `exec` or `client join` line, and no folder was made. Holds. |
| PB7 on macOS | With the agent installed, ran with `LGTM_CLIENT_STATE_DIR` naming another folder. | Exit 1 with `... run install-client.sh --remove first.` Holds. |
| State folder read back from the agent | A folder named with `+ . @ -`, then a rerun without the variable. | Exit 0, and the folder was found. Holds. |
| Real launchd, real `plutil`, a real Mac | Not possible on this host. | Left to gate G3. |

## Check 2: the code against the plan

Each line of item 3's "Behaviour" is present:

- Steps 4 and 16 are skipped on `Darwin`, and the probe `launchctl print gui/<uid>` comes first, with the fixed sentence on failure (`install-client.sh`, the `Darwin` case of `main`).
- The agent file is rendered from the template and renamed into place, then `bootout` (a not-loaded agent is ignored), `bootstrap` and `enable` run in that order (`install-client.sh:161-183`). The wait reads `state = running` (`install-client.sh:204-213`).
- `--remove` runs `bootout`, deletes the agent file and the program, and exits 1 when `bootout` fails on a loaded agent (`install-client.sh:79-91`).
- The template has the nine keys the plan lists, with the four placeholders (`dev.lgtmstudio.client.plist`).
- The macOS branch changes no step that touches the token. The join still comes before the install, and `installed=absent` is asserted for the agent too.

I found no difference from the plan. I read the script for constructs that Bash 3.2 or the BSD tools of macOS reject, and found none in the macOS path: `replace_file` avoids `mv -T`, `mktemp` templates end in `X`, and `sed`, `awk` and `grep` use only POSIX options.

## Check 3: the tests

Mutations in scratch copies of `deploy/` (the repository was not touched). A mutation that a test catches is marked "caught".

| Mutation | Result |
|---|---|
| macOS branch drops `bootout` before `bootstrap` | caught by `installs_macos_launch_agent` |
| macOS branch drops `launchctl enable` | caught by `installs_macos_launch_agent` |
| `SuccessfulExit` true, `ThrottleInterval` 10, `ProcessType` Interactive in the template | each caught by `macos_agent_parses` |
| Raw `$PATH` written into the plist | **survives** (post-b 1) |
| `replace_file` uses GNU `mv -fT` on `Darwin` | **survives** (post-b 2) |
| The wait for the agent never fails on a timeout | **survives** (post-b 3) |

## Findings

### post-b 1 — MINOR — the macOS agent's search path is not tested for unsafe entries

- **Statement:** plan, item 3 ("steps 6, 7, 15 and 17 use launchd") and item 2 step 6 (the service's search path drops entries with an unsafe character).
- **Evidence:** `install-client.sh:169` writes `$(service_path)` into the plist. `unsafe_path_entries_dropped` runs on `Linux` only, and the macOS tests use a clean path (`installer.sh:935-948`). With the mutation `<string>${PATH}</string>`, all 300 checks pass.
- **Failure scenario:** a later edit that writes the raw path into the plist passes every test. A path entry such as `/x/R&D/bin` then makes the plist invalid, so `launchctl bootstrap` refuses it, and nothing in the tests notices.
- **Fix:** run `unsafe_path_entries_dropped` with `SHIM_UNAME=Darwin` and read the agent file's `PATH` string.

### post-b 2 — MINOR — the macOS branch is tested only with GNU tools

- **Statement:** plan, item 3 test table, and the plan's own note that "no machine of the checks runs macOS".
- **Evidence:** `installer.sh` links GNU `mv`, `sed`, `mktemp` and `grep` into every scenario. I changed `replace_file` to `mv -fT -- "$1" "$2"` for `Darwin`, and all 300 checks passed. BSD `mv` has no `-T`.
- **Failure scenario:** the builder first wrote `mv -fT` in the shared code, and `replace_file` exists because of that. Nothing but the person's run of gate G3 keeps it from coming back.
- **Fix:** for the `Darwin` scenarios, put a stand-in `mv` on the path that exits 64 for `-T`, and likewise for `sed -r`, `grep -P` and `mktemp --tmpdir`.

### post-b 3 — MINOR — the wait for the agent has no test of its timeout

- **Statement:** plan, item 2 step 17 and item 3 ("the wait of step 17 reads `launchctl print`").
- **Evidence:** the `launchctl` stand-in prints `state = running` for every call of the agent (`installer.sh:242-244`). With the mutation `... || break #`, which ends the wait at once, all 300 checks pass. The Linux wait has the same gap.
- **Failure scenario:** a wait that never reports a failure, or never waits, is accepted, and the installer then says that the client runs when it does not.
- **Fix:** add a variable `SHIM_AGENT_STATE` that prints `state = not running`, and assert exit 1 and the sentence `did not start`. The test takes 10 seconds, so let it shorten the wait through an environment variable.

### post-b 4 — MINOR — an agent file in another layout makes the installer refuse the right folder

- **Statement:** plan, item 3 ("Step 6 reads the folder from the agent's `EnvironmentVariables`") and D19.
- **Evidence:** `launch_agent_state_dir` (`install-client.sh:93-107`) reads only the layout in which the key and its `<string>` are on two lines. I rewrote the agent file so the key and the string share one line. A rerun without the variable then printed `LGTM_CLIENT_STATE_DIR must be an absolute path.`, and a rerun with the correct variable printed `LGTM_CLIENT_STATE_DIR names another folder than the installed service; run install-client.sh --remove first.`
- **Failure scenario:** a tool that rewrites the file (`plutil -convert`, `defaults write`, an editor) makes the installer refuse the folder the agent really uses, with a sentence that points at the wrong cause. It fails closed, so no second machine is joined.
- **Fix:** read the value with `plutil -extract EnvironmentVariables.LGTM_CLIENT_STATE_DIR raw` when `plutil` exists, and when the value is empty, say that the installed agent could not be read.

### post-b 5 — MINOR — relative and empty entries of the search path are copied into the service

- **Statement:** plan, item 2 step 6 and D19 (the harnesses run as the one user).
- **Evidence:** `service_path` keeps any entry made of letters, digits and `. _ / + @ -`, and keeps empty entries. A run on `Darwin` with the path `...:.:rel::/tail` wrote `.:rel::/tail` into the plist.
- **Failure scenario:** the service's folder is `/` under launchd and the home folder under a systemd user manager, so a harness name can resolve to a file in that folder. Security issues do not block the build, so this is recorded for `docs/issues.md`.
- **Fix:** keep only entries that start with `/`.

### post-b 6 — MINOR — `ProcessType` `Background` may slow the harnesses the client starts

- **Statement:** plan, item 3 (the template's `ProcessType`).
- **Evidence:** `dev.lgtmstudio.client.plist:32-33`. Apple's documentation for `ProcessType` says that `Background` lowers the priority of CPU and disk work of the job. I could not check this on a Mac, and a child process inherits the setting.
- **Failure scenario:** builds and tests that an agent runs under the client run slowly on a Mac, with no sign in the gate that the key is the cause.
- **Fix:** have the person at gate G3 time a build started through the client against one started in a terminal, and drop the key or use `Interactive` if the difference is large. This is a plan question, not a defect of the code.

### post-b 7 — MINOR — the stop of the old agent before `bootstrap` is not waited for (not checked on a Mac)

- **Statement:** D16 (the client comes back by itself) and plan, item 3 ("`bootout` ... ignoring an agent that is not loaded, `bootstrap`").
- **Evidence:** `install_launch_agent` runs `bootout` and straight after it `launchctl bootstrap` (`install-client.sh:176-178`). I know that a `bootstrap` right after a `bootout` of the same label can fail with an input/output error on some macOS versions, but I could not run it here.
- **Failure scenario:** on a rerun, the program and the agent file are already replaced when `bootstrap` fails, so the installer exits 1 with the old agent stopped. A second run then succeeds.
- **Fix:** after `bootout`, poll `launchctl print <target>` for up to 5 seconds until it fails, or retry `bootstrap` twice. Gate G3 should include a rerun on a Mac.

## Notes that are not findings

- The macOS agent's output file `~/Library/Logs/lgtm-client.log` is made by launchd and never rotated. The client's log content is covered by `TestAppLogRedactsSecrets` of plan `go-s3-client-link`.
- `--remove` on a Mac needs a login session, because the probe runs first. This is the plan's wording.

## Grade

**8 / 10.** The code follows the plan line by line, the unreviewed fixes hold, and the tests catch five of the eight mutations I made to the macOS branch. The three survivors are the same kind of gap: the macOS branch is proved on Linux tools only, and its timeout and its path filter are not asserted.
