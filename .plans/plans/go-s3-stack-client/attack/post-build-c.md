# REVISE

Killing the live test during join can leave its installer running, and that installer can put the program and unit back after cleanup reports success.

This is review `postbuild-go-s3-stack-client-c`, nonce `369131`. I reviewed work item 4 and the standard-input handoff on `sprint-0` in `84fa8bbe..a82e6e16`. The four files in the assigned diff had the same content at the review checkout's head. I wrote neither the implementation nor the frozen tests.

## Findings

### post-c 1 — MAJOR — A surviving installer can undo successful cleanup

**Statement.** The plan's post-build check PB4 requires cleanup of the interrupted default part to leave no program or unit, with a harmless replay.

**Evidence and failure scenario.** The live test runs the installer as a child at `deploy/test/installer-live.sh:268`. Cleanup can remove the fixture and return success at lines 506 through 514. At lines 464 through 466, a replay with no fixture returns a skip. I paused the installer just after the stand-in client saved its identity and killed only the live-test process. Cleanup printed `ok 8 # SKIP no running service`, exited 0 and removed the fixture. When the surviving installer resumed, it wrote the final executable and unit and requested a service restart. A second cleanup exited 0 and left both files. The service manager was a stand-in, and the executable and unit were actual files under the disposable home.

**Fix.** Give the fixture installer and cleanup a shared lock that stays held until the installer exits even if the live-test process dies. Cleanup should wait for the invocation to end or stop the recorded, fixture-owned invocation before it removes state and installed files.

### post-c 2 — MAJOR — An interrupted join leaves an executable after cleanup

**Statement.** The plan's post-build check PB4 requires cleanup to leave no fixture folder, unit or program after an interrupted install.

**Evidence and failure scenario.** The installer creates and builds the temporary executable beside the final path at `deploy/install-client.sh:320` through line 327. The exit trap removes it at line 50 during ordinary exits. A kill signal that cannot be caught bypasses that trap. Cleanup calls `--remove` at `deploy/test/installer-live.sh:506`, and that command deletes only the final binary and unit at `deploy/install-client.sh:73`. I killed the whole fixture process group just after the stand-in client saved its key, and again just after it saved its identity. The real installer and cleanup scripts ran in the disposable fixture. Both runs left an executable named `lgtm-client.<suffix>`. The identity case reported success, and both replays reported a skip while the executable remained.

**Fix.** Record the exact installer-owned temporary paths before creating the files. After the installer has stopped, have fixture cleanup remove those paths before it removes the fixture record. Exercise the two kill points with the actual build and install sequence in the fixture.

### post-c 3 — MAJOR — Local cleanup depends on the stack before an identity exists

**Statement.** The plan's item 4 and PB4 require cleanup after interruption once the record exists, both before and after the key is saved.

**Evidence and failure scenario.** At `deploy/test/installer-live.sh:469` through line 472, a record triggers a Compose port lookup before the script checks for an identity or a key. I repeated the record-only and key-only cases with the Compose stand-in reporting no published port. Both cleanup runs exited 1 with `The fixture record or its published address could not be verified.` Both left the fixture in place, and the key-only case omitted the instruction to remove the waiting machine. Neither case has a machine identifier that cleanup can send to the server.

**Fix.** Validate the published address immediately before making a server request. In the record-only and key-only cases, finish the owned local cleanup and retain the key-only instruction even if the stack is unavailable.

### post-c 4 — MINOR — A fixture without a record can delete an unrelated executable

**Statement.** The plan's item 4 limits cleanup to the fixture. Its early-stop case removes the fixture folder when both the record and the state folder are absent.

**Evidence and failure scenario.** The matching-unit check at `deploy/test/installer-live.sh:52` succeeds when the unit file and loaded unit are both absent. The early-stop branch then reaches the unconditional `--remove` at line 506. I killed the fixture before it wrote a record, placed a stand-in executable at the normal installation path, and ran cleanup. It removed that executable and reported a skipped service check. A user who installs or copies a standalone client after an interrupted fixture can lose it during later fixture cleanup.

**Fix.** In the no-record and no-state case, remove the fixture folder alone. Require evidence of fixture ownership before invoking the full uninstall in later cases.

### post-c 5 — MINOR — An interrupted sign-in can outlive fixture cleanup

**Statement.** Item 4 of the plan requires sign-out for parts that signed in. Rule R9 covers handling secrets.

**Evidence and failure scenario.** Sign-in creates a new cookie jar at `deploy/test/installer-live.sh:101`. The exit handler signs out using only the current invocation's `jar` at line 132. Cleanup creates a fresh session when there is an identity at line 480, signs that session out at line 507, and deletes the entire fixture at line 508. In the key-only interruption, the stand-in recorded zero sign-outs. In the identity interruption, it recorded a sign-out for the newly created jar. The earlier session's jar was discarded in both runs. On the real server, sign-out revokes the session associated with the supplied cookie. An interruption can therefore leave a session valid until expiry. See `src/server/internal/api/session.go:94`.

**Fix.** When the recorded stack is reachable, attempt sign-out for the private cookie jars left by earlier fixture invocations before discarding them.

### post-c 6 — MINOR — The frozen tests miss two regressions in token transport

**Statement.** The standard-input assignment requires the token to stay out of arguments, child environments, files, traces and output. Rule R9 covers secret handling.

**Evidence and failure scenario.** The stand-in checks the last argument and reads standard input at `deploy/test/installer.sh:180` through line 189. The tests check output and surviving files at line 483. The live gate's source guard at `deploy/test/installer-live.sh:261` matches one particular here-string call. It also matches that call if the installer exports the token or removes the pipe check. Both changes passed all 301 frozen assertions. An observation from inside the child confirmed that exporting the token exposes it in the environment. Forcing the unchecked here-string put the token on a regular file descriptor under Bash 5.0 compatibility. The production handoff used a pipe in both observed shell modes.

**Fix.** With a synthetic token, assert the child's argument list and environment and the type of its standard-input descriptor. Exercise both Bash modes and replace the live source match with a behavioral check that can also accept an always-pipe implementation.

## Unreviewed scenarios and test output

The post-build checks below come from `attack/round-4-findings.md` and `plan.md:321`. Results are confirmed live using the environment noted for each check. I verified the source assertions by reading the cited files. The interrupted-cleanup probes used the real scripts with stand-ins for the service manager, Docker, the network calls and the client build. The probes synchronized each kill with the relevant fixture write or operation.

| Check | Observed result |
|---|---|
| PB4 before the record and after the record but before the key | Both cleanups printed `ok 8 # SKIP no running service`, exited 0 and removed the fixture. Replays printed the same skip and exited 0. |
| PB4 after the key but before the identity | Cleanup printed `not ok 8 - live_removed_machine_stays_stopped Remove the waiting machine on the Machines page.` and exited 1. It removed the fixture and left the temporary executable. A replay printed the skip and exited 0. |
| PB4 after the identity but before the unit | Cleanup sent one `RemoveClient` for the recorded machine, removed the fixture, printed the skip and exited 0. A `not_found` response gave the same result. The temporary executable survived the first cleanup and its replay. |
| PB4 after the identity, killing the live-test process alone | The surviving installer wrote the program and unit after cleanup printed the skip and exited 0. A replay printed the skip and left both files. |
| PB4 recorded Compose settings | Cleanup restored the complete two-file `COMPOSE_FILE` value recorded by the first run, overriding a different value supplied by the cleanup caller. |
| PB6 and PB7 | Test 18 passed with both Linux and macOS stand-ins. Moving the installed state gave `holds no identity`. A conflicting override gave `--remove first`. Both cases kept the files unchanged and made no join. This session could not reach a real systemd user manager and the host runs Linux. |
| PB8 and gate G1 | I rebuilt the server in an isolated Compose project with PostgreSQL and OpenBao. The real token command exited 0, wrote one correctly shaped line and left standard error empty. I then added a diagnostic to the successful command in the scratch copy and rebuilt it. The command still exited 0 and wrote one correctly shaped line. The gate reported that standard error was non-empty and suppressed the diagnostic text. An invalid argument exited 2 with empty standard output and non-empty standard error. The probe captured both streams in private files and removed them. |
| PB8 installer | The `SHIM_EXEC_FAIL` case in test 8 exited 1, printed `returned no token`, suppressed the marker written to both streams, and attempted no join. |
| PB2 token and trace | A synthetic join ran with Bash tracing requested in the native shell and Bash 5.0 compatibility mode. The child saw a dash, pipe-backed standard input and no token in its arguments or environment. A traced, opted-in live script with stand-ins reached the key write after sign-in and printed only `+ set +x` before interruption. The cross-user process-list observation during a real join remains unrun. |

The guide covers installation, the service and state paths, platform-specific logs and startup, updates, and removal and rejoin. Its description of the token supersedes the positional-token wording in the plan's guide and PB2. The standard-input builder's result records that change. See `deploy/README.md:85` and `.foundry/go-rewrite/results/s3-installer-build-c.md`.

The acceptance command for item 4, `deploy/test/installer-live.sh` with `LGTM_INSTALL_LIVE` unset, printed exactly `1..8` and exited 2. With tracing requested and a synthetic password, all three entry points printed only `+ set +x` on standard error and `1..8` on standard output, then exited 2. The disposable home was unchanged. Syntax and executable-mode checks passed.

The frozen installer suite, `bash deploy/test/installer.sh`, exited 0. It printed `1..25`, all 25 `ok` lines, and `301 passed, 0 failed`. The check for the old state variable in criterion 2e also passed.

I ran both mutations prescribed for the risky installer at `plan.md:192` and the positional-token regression in the same scratch checkout. The scratch control also printed `301 passed, 0 failed`.

| Change | Frozen suite result |
|---|---|
| Print the token before join. | Exit 1 with `297 passed, 4 failed`. Test 4 reported `the output does not hold the token` as a failed assertion. |
| Install the program before join. | Exit 1 with `290 passed, 11 failed`. Test 3 reported `the join line ends with installed=absent` as a failed assertion. |
| Restore the positional token. | Exit 1 with `203 passed, 98 failed`. Test 4 reported `The token was in the argument list.` |
| Export the token before passing it on standard input. | Exit 0 with `301 passed, 0 failed`. |
| Force the here-string without checking that it uses a pipe. | Exit 0 with `301 passed, 0 failed`. |

Gate G2 remains open. An opted-in preflight exited 1 with `The systemd user manager is unavailable.` The active-service, reboot and before-login checks require the disposable Linux runner and the observer named by the plan. The macOS device gate requires a Mac. Gate G4 and PB5 belong to the separate user-flow test, outside the assigned diff.

## Grade

**6 / 10.** The nominal token handoff and the installer suite passed. Three major findings concern cleanup after interruption. The live service and reboot gate remains unverified.
