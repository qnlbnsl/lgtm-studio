# Post-build review F of go-s2-server-signin, item 9

Reviewer role: reviewer of built code, model claude-sonnet-5-5 at high effort. Task identifier: postbuild-go-s2-server-signin-f, nonce 42b364.
The builder of this item was a worker of my own vendor on another model, so the rule that another vendor reviews was not met.
The reviewed code is the committed state of `src/server/internal/app`, `src/server/cmd/lgtm-server`, `src/server/servertest`, `src/server/internal/store/instance.go` and `src/server/internal/vault` on the branch `sprint-0`.
I exported that committed state into a scratch folder, because other teams have uncommitted changes in the working tree. Line numbers below refer to that exported state.

## Verdict

SHIP, with ten minor findings that the owner can fold into a later pass.
The code does what the plan's item 9 says. The port stays closed until every prerequisite holds, a second instance cannot serve, and no password, token or key reaches a log or an error text.
Every mutation that the plan names is stopped by a frozen test. Two of my own mutations survive, and each of them hides a behaviour that is visible to an operator or to a later test.

## Top finding

The fixture `servertest.Server.Restart` ties the restarted application to the test that calls it, so a restart from inside a subtest silently stops the server when the subtest ends (post-f 1).

## What I ran

- I ran `deploy/test/with-postgres.sh -- go test -race -count=1` on the packages `app`, `cmd/lgtm-server`, `servertest`, `vault` and `store`, filtered to the item 9 tests and their neighbours.
  Every item 9 test passed: `TestStartRevokesBeforeListen`, `TestStartWithUnavailableVault`, `TestStartRefusesSecondInstance`, `TestStartupMigrationBudget`, `TestStartMonitorsLockProbeBudget`, `TestStartupDiagnostics`, `TestHealthCommand`, `TestStartAndRestartServer`, `TestServerInstanceLock` and `TestLinkIdentityReportsKeyAdoption`.
  The packages `cmd/lgtm-server`, `vault` and `store` printed `ok`.
  The packages `app` and `servertest` printed `FAIL`, and only because of tests that belong to the link and machines plans of other teams: `TestUnsignedOpenProvesServer`, `TestStreamStartClearsStalePresence`, `TestPartialLinkCleanupRefusesReadiness`, `TestShutdownEndsLinkStreams`, `TestLinkStartupPreservesPartAReceipt`, `TestServerServesMachines` and `TestLinkSeamsReachTheService`. I do not report them.
- I ran the plan's six mutations for item 9 in separate scratch copies, each through `deploy/test/with-postgres.sh`.
  - M1, the real listener opens right after the lock and serves later: stopped by `TestStartRevokesBeforeListen` at `start_test.go:323` and `:359`. My first variant, a second throwaway listener, failed for the wrong reason, a port collision, so I discarded it. The builder reports the same slip.
  - M2, admission bypassed (the `pg_advisory_lock` query becomes `SELECT 1`): stopped by `TestServerInstanceLock`, at `instance_test.go:46`, in three subtests.
  - M3, the health bound grows to one hour: stopped by `TestHealthCommand/stalled_reply_has_a_two_second_bound` at `main_test.go:504`.
  - M4, the probe loses its deadline: stopped by `TestStartMonitorsLockProbeBudget/short_budget_closes_the_socket_and_releases_the_lock` at `start_test.go:709`.
  - M5, a failed OpenBao load continues with a transient generated key: stopped by `TestStartWithUnavailableVault` at `start_test.go:412`.
  - M6, `Restart` starts a new OpenBao fixture with a new seed: stopped by `TestStartAndRestartServer` at `server_test.go:97`.
- I also ran three mutations of my own on the places that the plan marks `UNREVIEWED MECHANISM`.
  - X2, a lost lock drains for five seconds instead of closing at once (the builder's unclear place 3): stopped by `TestStartMonitorsLockProbeBudget` at `start_test.go:711`. So that choice is tested.
  - X5, any non-deadline failure of the lock query is reported as "another server is already using this database": **survived**. Both `TestServerInstanceLock` and `TestStartupDiagnostics` printed `ok` (post-f 2).
  - Y1, `Wait` returns nil after a lost lock: **survived** in the whole `app` package (post-f 3).
- I wrote one probe test in a scratch copy of `servertest`: `Start(t)`, then `Restart` inside `t.Run`, then a request after the subtest ended. It failed with "connection refused" (post-f 1).
- I read, but did not run, the builder's U1c result: the explicit socket close on a failed acquisition survives its mutation.
- Post-build checks:
  - PB3 and PB5 are covered by the frozen tests that I ran (`TestStartRefusesSecondInstance`, `TestStartWithUnavailableVault`, `TestStartAndRestartServer` and the adoption tests). I did not run `deploy/up.sh --remove-password`.
  - PB1, PB2, PB4, PB6 and PB7 I could not run. PB1 needs the deployed stack and a browser, PB2 needs the later proxy, PB4 needs a real OpenBao that is sealed and unsealed by hand, and PB6 and PB7 need the Machines and sign-in plans of later items.
  - The builder reports that `deploy/test/server-image.sh` and `deploy/test/stack-up.sh` pass. I did not repeat them, because the stack suites start containers that other teams may also use.
- The task names a register "Not reviewed before the build" in `attack/`. No file there holds that heading. As the earlier reviews did, I used the plan's own `UNREVIEWED MECHANISM` marks.
- The scratch folders are in the system temporary folder with the prefixes `postf-` and `postf-mut-`. I left them in place.

## The three checks that the task names

- **No password, token or key reaches a log or an error text.** The only log lines of the application are the readiness event, with the public key's fingerprint, the vault version and the adoption flag, and the store report, with an error that the store already reduces to a fixed sentence. Every `StartupError` line is a fixed sentence. The command prints a retired setting by name only. The fixture writes its password file with mode `0400` and discards its log. I found no path that carries a secret.
- **The port stays closed until every prerequisite holds.** The order in `start.go` is lock, probe, migration, reconciliation, password check, settings and vault checks, key load, follower, last probe, listen, a recheck of the lifetime, and only then the readiness line and `Serve`. M1 and M5 are both stopped by tests.
- **A second instance cannot serve.** The lock is a session lock on an unpooled connection, the admission wait is bounded, and a failed probe closes the raw socket first. M2 and M4 are stopped. The remaining windows are bounded and are named in post-f 4.

## The builder's seven unclear places

1. **Added fixed sentences for the lock, the follower, the listener and three invalid settings.** Acceptable. The sentences carry no values. The plan's table should gain these rows, so that a later reviewer does not have to read the result file to find them.
2. **The interruption order is applied in every phase.** Acceptable, and `TestStartupDiagnostics` requires it for admission and for an expired start context.
3. **A lost lock closes the server at once instead of draining.** Acceptable, and tested: mutation X2 is stopped.
4. **`Wait` returns nil after `Close` or the end of the start context, and `Close` always returns nil.** The choice is reasonable, but the error that `Wait` returns after a lost lock is not pinned by any test (post-f 3), and a failed unlock is invisible (post-f 5).
5. **The adoption label is the boolean field `adopted`.** Acceptable. The value is false when an established pin returns early, which is the intended meaning "this load did not adopt".
6. **A literal loopback address unless the private-stack opt-in.** Matches the plan. With the opt-in the listener also accepts an empty host, an unspecified address and a private address, as the Compose value `:8080` needs.
7. **The validated `LinkIdentity` is kept in a private field.** Acceptable.

## Findings

### post-f 1. MINOR. `servertest` ties a restarted application to the test that calls `Restart`.

- **Statement.** `launch` starts the application with `app.Start(t.Context(), ...)` and a lifetime that derives from that context. A `Restart(t)` from a subtest therefore ends the server when the subtest ends.
- **Evidence.** `src/server/servertest/server.go:147` passes `t.Context()` of the caller. The builder's own result file says that ending the start context stops a running server (place 4). My probe test, `Start(t)` then `Restart` inside `t.Run` and then a request, failed with "connection refused". Every current caller passes the top-level test, so the frozen tests do not see it.
- **Failure scenario.** A later plan, such as Machines or link, restarts the server inside a table-driven subtest, then makes a request in the parent. The server is gone, and the failure looks like a defect of the code under test.
- **Fix.** Start the application under a context that the fixture owns and cancels in `stop`, as `Start` already does for the first launch, or document and test that `Restart` must receive the test that called `Start`. (Design: plan, item 9, "public test server".)

### post-f 2. MINOR. No test separates a busy database from any other failure of the lock query.

- **Statement.** A non-deadline failure of the `pg_advisory_lock` query, and of the preceding `SET statement_timeout`, is classified by the `default` branch as the fixed acquisition sentence. The mutation "report busy on every such failure" survives.
- **Evidence.** `src/server/internal/store/instance.go:61-68`. With the `default` branch returning `ErrServerInstanceBusy`, `TestServerInstanceLock` and `TestStartupDiagnostics` both printed `ok`. The builder reports the related survivor U1c, the explicit close at `instance.go:60`, for the same branch.
- **Failure scenario.** A later change makes any server error of the query report "Another server is already using this database." An operator then looks for a second instance when the cause is a terminated backend or a permission error.
- **Fix.** Add one subtest that terminates the dedicated backend during the wait, or fails the `SET` with a server error, and requires the acquisition sentence and a closed socket. (Design: plan, "UNREVIEWED MECHANISM" of the instance lock.)

### post-f 3. MINOR. No test pins the error that `Wait` returns after a lost lock.

- **Statement.** `main` exits with code 1 only if `Wait` returns an error after a lost lock. The mutation that returns nil there survives in the whole `app` package.
- **Evidence.** `src/server/internal/app/start.go:296-299` sets `failure = errLockLost`, and `:310` stores it. With `failure = errLockLost` removed, `TestStartRefusesSecondInstance`, `TestStartMonitorsLockProbeBudget`, `TestStartupDiagnostics` and `TestStartAndRestartServer` printed `ok`. The helper `requireWaitBy` ignores the value of `Wait`. `main.go:79-84` turns a non-nil value into exit code 1.
- **Failure scenario.** After the mutation, a server that lost its lock exits with code 0 and no line on standard error. Compose's `restart: unless-stopped` restarts it anyway, but a standalone run, a supervisor with `on-failure`, and the operator reading the exit code all see a clean stop.
- **Fix.** Make `requireWaitBy` or one test assert that `Wait` returns an error that carries the fixed sentence "The server lost its PostgreSQL instance lock." after the backend is terminated. (Design: plan, item 9.)

### post-f 4. MINOR. Two bounded windows let a stale instance answer after it has lost the lock.

- **Statement.** After a silent loss, the old instance answers until the next probe fails, which is at most the one-second interval plus the five-second probe budget. During a graceful stop, nothing probes while the drain runs for up to five seconds.
- **Evidence.** `start.go:346-361` (`watchLock` exits when the lifetime ends) and `:300-304` (the drain happens before `release`, which stops the monitor at `:321` and unlocks at `:324`). The plan accepts the first window (plan line 498, one second after each completion, one longer probe tolerates a pause). The second is not mentioned.
- **Failure scenario.** The instance receives the termination signal and starts its drain. While it drains, its lock is lost, so another server starts and migrates. For up to five seconds both answer requests that touch the same database.
- **Fix.** Keep the probe running during the drain, or shorten the drain, or state the bound in the plan's item 9 text next to the first window. The owner decision about two live copies of one machine does not cover two servers of one database. (Design: plan, item 9.)

### post-f 5. MINOR. A failed unlock or a failed store close leaves no trace.

- **Statement.** `release` discards the error of `lock.Close`, and `Server.Close` always returns nil.
- **Evidence.** `start.go:324` (`_ = st.lock.Close(ctx)`) and `:437-441`. `instance.go:134-135` returns `errInstanceRelease` for exactly this case.
- **Failure scenario.** PostgreSQL does not answer the unlock within five seconds at shutdown. The socket closes, so the lock is released anyway, but the operator never learns that the clean path failed, and a stuck shutdown cannot be diagnosed from the log.
- **Fix.** Log the fixed sentence with the same `Warn` call that the store report uses, and let `Close` return it. (Design: plan, item 9.)

### post-f 6. MINOR. `main` reports a wrong setting when the private-stack value is invalid.

- **Statement.** Any error of `config.Load`, including the invalid value of `LGTM_ALLOW_PRIVATE_STACK`, ends as "The PostgreSQL settings are invalid."
- **Evidence.** `src/server/cmd/lgtm-server/main.go:68-72` sets `cfg.Postgres = nil` on every error, and `src/server/internal/config/config.go:63-68` returns `LGTM_ALLOW_PRIVATE_STACK must be true or false` early. `start.go:29` and `:140` then print the PostgreSQL line.
- **Failure scenario.** An operator writes `LGTM_ALLOW_PRIVATE_STACK=True`. The server answers that the PostgreSQL settings are invalid, and the operator checks the host, user and password file in vain.
- **Fix.** Give this setting its own fixed sentence, as the builder did for the listen address, the public URL and the origins. (Design: plan, diagnostic table.)

### post-f 7. MINOR. The fixture hides the fixed startup sentences behind a generic failure.

- **Statement.** When `app.Start` refuses, `launch` fails the test with "The fixture application did not start." and drops the `StartupError` lines that name the cause.
- **Evidence.** `src/server/servertest/server.go:147-150`. The builder's M6a run shows it: the test fails at `server_test.go:97` with the generic message, while the real cause is the replaced-key sentence.
- **Failure scenario.** A later team's change makes the restart fail with the vault or the migration line. Its developer sees only the generic sentence and must rerun under a debugger.
- **Fix.** Call `t.Fatal` with `err.Error()`, which carries fixed sentences only. (Design: plan, item 9; the sentences hold no secrets.)

### post-f 8. MINOR. The fixture reserves its port and releases it before the application binds it.

- **Statement.** `loopbackAddress` closes its listener, and the application binds the same port later, at launch and again at every restart.
- **Evidence.** `src/server/servertest/server.go:173-183`, used at `:65`, and the restart at `:133-137`.
- **Failure scenario.** Another test process, or another team's fixture on the same machine, takes the port between the release and the bind. The start fails with the listener sentence, and only under load, so the failure is flaky and the first two items of this list make it hard to read.
- **Fix.** Retry the start on a fresh port when the failure is the listener sentence, or let the first launch keep its listener. (Design: plan item 9.)

### post-f 9. MINOR. The fixture's shared fields have no synchronisation.

- **Statement.** `s.app` and `s.SigningPublicKey` are written by `launch` and `stop` and read by other goroutines of a test.
- **Evidence.** `src/server/servertest/server.go:151-152` and `:155-161`, with the cleanup registered at `:103`. A test that restarts from one goroutine and reads `SigningPublicKey` from another is a data race under `-race`.
- **Failure scenario.** A parallel subtest reads the key while the parent restarts the server. `go test -race` reports a race in the fixture, not in the code under test.
- **Fix.** Guard the two fields with a mutex, or state in the package comment that a fixture is for one goroutine. (Design: plan item 9.)

### post-f 10. MINOR. The task's register heading does not exist.

- **Statement.** The task says that the newest register in `attack/` lists the unreviewed places under "Not reviewed before the build". No file has that heading.
- **Evidence.** `grep` over `attack/` finds no such heading. The plan's `UNREVIEWED MECHANISM` marks are the only list, and I used them.
- **Failure scenario.** A reviewer who trusts the task text believes that the list is complete and does not check the plan's marks.
- **Fix.** Either add the register, or change the task template to name the plan's marks. (Design: plan.)

## Grade

I grade this build 8 out of 10.
The startup order, the lock, the probe, the health command and the adoption flag all do what the plan says, and every mutation that the plan names is stopped by a test that fails for the right reason.
I take two points off because the error that `Wait` returns after a lost lock is not tested, and because the fixture has three weaknesses that will show up when other teams use it in subtests and in parallel.
I found no blocker and no major defect.
