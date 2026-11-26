REVISE

# Round 1 review of go-s3-client-link, architect lens

- Task identifier: review-go-s3-client-link-r1-architect
- Nonce: eb4a7b
- Reviewer model: claude-sonnet-5-5, high effort
- Plan reviewed: `.plans/plans/go-s3-client-link/plan.md`, as it stands today.

## Top finding

The command item reserves an expired command by calling `Begin` with a made-up time, so a crash before `Finish` records the command as INTERRUPTED although it never started (architect-r1 01).

## Findings

### architect-r1 01 — MAJOR, a command that never started is reported as interrupted after a crash

- Statement: R6, D16, and the plan's item 6.
- Evidence: `.plans/plans/go-s3-client-link/plan.md:244` and `:246` describe the reservation trick for an expired command. `.plans/plans/go-s1-client-local/plan.md:411` and `:423-424` define `Begin`, which writes a `started` receipt and returns DecisionRun only after the commit. `.plans/plans/go-s1-client-local/plan.md:448` fixes that `now` equal to `ExpiresAt` gives DecisionExpired without a write. `.plans/plans/go-s1-client-local/plan.md:507-509` states the caller's rules.
- Failure scenario: An expired command arrives. Because DecisionExpired writes no receipt, the plan reserves the command by calling `Begin` again with a substitute time that makes it runnable, then calls `Finish` with EXPIRED. The process is killed between the two calls. The `started` receipt is now on disk with no result. After the restart, the store reports the command as interrupted, and the client sends INTERRUPTED. The server and the website then show a command that never ran as one that started and stopped. The plan's own rule says INTERRUPTED means started and stopped before its end was recorded, so the result is false.
- Prescribed fix: Ask the Client Local plan, in its fold, for one new store method, for example `Decline(c Command, outcome []byte) error`, that writes a finished receipt directly without a `started` state. Name it in item 6 and in the Sprint 1 request list. Add the test `TestDeclineSurvivesKillBeforeFinish`. The test kills the process after the decline call and expects EXPIRED, never INTERRUPTED, after the restart. Delete the substitute-time reservation from item 6.

### architect-r1 02 — MAJOR, check C10 needs an exact log record that no frozen test asserts

- Statement: C10, D7, D17, R8.
- Evidence: `deploy/test/checks.sh:329` (`log_has` matches both `.msg` and `.code`), `:343` (`wait_refusal` waits up to 20 seconds), `:505-530` (`check_c10` replaces `serverPublicKey` in `identity.json` with the client key and then expects `link_refused` with code `server_key_mismatch`). The plan's item 3 at `.plans/plans/go-s3-client-link/plan.md:146-158` refuses a bad server signature but never states the log message or the code string.
- Failure scenario: The builder refuses the mismatched server by closing the stream, and logs it as `link_failed` with another code. The in-process test passes, because it asserts only that no Proof frame was sent. The installer-level check C10 waits 20 seconds, finds no `link_refused` record with `server_key_mismatch`, and fails in sprint 3.
- Prescribed fix: State in item 3 that a Challenge whose signature fails against the saved server key ends the stream, sends no Proof, and writes the log record with message `link_refused` and code `server_key_mismatch`. Add to the frozen tests one that reads the log and asserts both fields. Add a second assertion that the mismatched peer received no Proof frame and no connection token.

### architect-r1 03 — MAJOR, the server plan answers the stream with codes that contradict the contract, and the client has no default for codes it does not list

- Statement: C9, C10, C11, R8, D16.
- Evidence: `src/contracts/proto/lgtm/link/v1/link.proto:46-53` makes `failed_precondition` with `unknown_machine` the final code for an unknown or removed machine, and an event gap `failed_precondition` with `event_gap`. `.plans/plans/go-s3-server-machines/plan.md:417` answers the same case with `unauthenticated`, and `:421` answers an event gap with `out_of_range`. The client's retry set is at `.plans/plans/go-s3-client-link/plan.md:195` and the permanent stops at `:198`.
- Failure scenario: A removed machine reconnects. The server answers `unauthenticated`. The client's plan treats `unauthenticated` as a bad proof that may change, so it retries every 15 seconds forever and never stops, which contradicts the removal rule. In the other direction, an event gap arrives as `out_of_range`. The client's table lists no such code, so it either stops permanently or falls into an unspecified branch, and the buffered events of C11 are never replayed.
- Prescribed fix: Add one sentence to item 7 that every code not in the plan's two lists is treated as retryable with the normal waits, and that only a `failed_precondition` after a verified Challenge, with reason `unknown_machine` or `link_revision_mismatch`, is final. Add a test that feeds a stand-in server each code of the contract table plus `out_of_range` and `unauthenticated`, and expects the stop or retry named in the table. Put a request in the result file for the Server team to use exactly the contract codes at lines 417 and 421.

### architect-r1 04 — MAJOR, the combined tests have no owner who can edit them

- Statement: R10, C6 to C11, the epic's file ownership, and the plan's done criteria 1b, 3a and 3b.
- Evidence: `.plans/plans/go-s3-client-link/plan.md:60-66` says the combined tests are owned by the orchestrator, and `:289` names a run in `src/linktest`. The epic's edit table at `.plans/epics/go-rewrite/epic.md:121-126` lists no team for `src/linktest/`. `.plans/plans/go-s3-server-machines/plan.md:1126` also asks the orchestrator for it. `scripts/check-go.sh` runs `go test ./...`.
- Failure scenario: Neither team may create `src/linktest/`, so done criteria 1b, 3a and 3b have no file to run. If the orchestrator later adds it, `go test ./...` in `scripts/check-go.sh` runs the combined tests on every machine, including those with no PostgreSQL, and the shared check fails for an unrelated team. The test names there also repeat the names in item 6, so two files could define the same test.
- Prescribed fix: Add a row to the plan's request list that names the folder, the owner, and a build tag or an environment guard, for example the combined tests skip with an explicit message unless a PostgreSQL address is set. State that the tests of items 1 to 9 inside `src/client/` never need PostgreSQL. Rename the combined tests with a `Link` prefix so they cannot clash with the item tests.

### architect-r1 05 — MINOR (consequence: a broken machine stops looking broken), permanent stops exit with code 0

- Statement: D16, C8.
- Evidence: `.plans/plans/go-s3-client-link/plan.md:198` (permanent stops) and `:259` (exit codes 0, 75 and 1). `.plans/plans/go-s3-stack-client/plan.md:30` and `:101` say exit 0 stops the service for good, 75 is a restart the server asked for, and any other code is restarted after 5 seconds.
- Failure scenario: A damaged `client.db` makes `run` stop permanently. If the plan maps that to exit 0, the service manager sees a clean stop, never restarts the client, and raises no alert. The owner finds the machine offline after 30 seconds and has no sign of why. A removed machine legitimately stops for good, but a damaged store does not.
- Prescribed fix: Split the permanent stops in item 7 and item 8 into two sets. A refusal the server gave (`unknown_machine`, `link_revision_mismatch`, a verified `server_key_mismatch`) exits 0 after one clear log record. Everything local and broken, such as a damaged store or an unreadable identity, exits 1 so the service restarts it and the log shows the cause. Add a test for each exit code.

### architect-r1 06 — MINOR (consequence: a build fails or a module drifts), the x/sys pin names the wrong source

- Statement: plan, and the epic's `go.mod` rule.
- Evidence: `.plans/plans/go-s3-client-link/plan.md:153` pins `golang.org/x/sys` and attributes the version to the sign-in list. `.plans/plans/go-s2-server-signin/plan.md:66-68` lists x/crypto v0.59.0, x/net v0.63.0 and connect v1.21.1 but no x/sys. `.plans/epics/go-rewrite/epic.md:132-133` lets a team add only a module its plan lists, together with its first import. In the module cache, `golang.org/x/net@v0.63.0.mod` requires x/sys v0.50.0, and that version has `CLOCK_BOOTTIME` and `ClockGettime` on Linux and `CLOCK_MONOTONIC_RAW` on macOS.
- Failure scenario: The builder adds x/sys at another version, because the named source lists none. `go mod tidy -diff` in `scripts/check-go.sh` then differs from the orchestrator's comparison with the plan, or the module graph resolves one version when the plan says another.
- Prescribed fix: Replace the attribution with: "x/sys v0.50.0, the version that x/net v0.63.0 requires." Add that the builder runs `go get golang.org/x/sys@v0.50.0` in the same change as the first import in `clock_linux.go` and `clock_darwin.go`, and that `go mod tidy -diff` must be clean afterwards.

### architect-r1 07 — MINOR (consequence: an unrecoverable join after a long outage), the lost-reply recovery has a hidden time limit

- Statement: D7, C5.
- Evidence: `.plans/plans/go-s3-client-link/plan.md:106-108` and `:258`. `src/contracts/proto/lgtm/link/v1/link.proto:61-76` checks `token_expired` before the replay rule, so an expired token is refused even for the same key.
- Failure scenario: The join succeeds on the server, the reply is lost, and the machine is offline for longer than the 15-minute token. The client has a durable key but no `identity.json`, and the server now answers `token_expired`. The client stops with no way to learn the machine id. The plan notes the deferral but gives the person no step to follow.
- Prescribed fix: Make the log message on `token_expired` after a retry say that the folder holds a key that the server may already know, and that the person must remove the machine on the Machines page and then join again with a new token in a fresh state folder. Add a test for that message.

### architect-r1 08 — MINOR (consequence: a check script silently measures nothing), the stack scripts still use the old state-folder name

- Statement: C5, C10, and the plan's request list.
- Evidence: `deploy/test/checks.sh:217-233` sets `XDG_STATE_HOME`. `src/client/internal/config/config.go:14` and `:28-39` read only `LGTM_CLIENT_STATE_DIR` and the default under the home folder. `deploy/test/README.md:81` still names the old variable. `.plans/plans/go-s3-stack-client/plan.md:224` still reads `server.json`.
- Failure scenario: The check script starts `lgtm-client join` with `XDG_STATE_HOME` set. The client ignores it and writes the key and identity into the person's real state folder. C10 then edits the wrong `identity.json`, or a second run finds a machine that already joined.
- Prescribed fix: Keep the request in the result file, and name it in the plan's rollout step 1 so it blocks the first sprint 3 run: the check script and README use `LGTM_CLIENT_STATE_DIR` with an absolute path, and the installer plan reads `identity.json` instead of `server.json`.

## Lens questions

1. Existing code claims: The plan's claims about `go-s1-client-local` names (`CreateStore`, `OpenStore`, `Read`, `Ack`, `Begin`, `Finish`, `Credential`, `WriteAtomic`) and about `config.StateDir` match the plans and the source in `src/client/internal/config/config.go`. The only mismatch is finding 01, where the plan uses `Begin` for a purpose it does not have.
2. Public names, writers and readers: Every name the plan introduces (`ParseOrigin`, `NewHTTPClient`, `Join`, `SaveServer`, `LoadServer`, `openSession`, `proofText`, `replay`, `reporter`, `commandHandler`, `serverClock`, `Run`, `RunOptions`) has one writer and one reader in the later items. `identity.json` has one writer, the join, and readers in the run command. The log record in finding 02 has no named writer, which is the gap.
3. Placement: All paths are under `src/client/`, which the epic allows. The only misplaced item is `src/linktest/` (finding 04).
4. Rules R1 to R11: R1 and R2 hold, because only generated types cross the boundary. R6 is weakened by finding 01. R8 is weakened by findings 02 and 03. R9 holds, because the plan redacts the token and secrets in logs. R10 holds for the item tests and is open for the combined tests (finding 04). R11 holds, because no code is copied. Nothing found on R3, R4, R5 and R7, which belong to the server.
5. Cross-plan mismatches: Finding 03 (codes), finding 08 (state variable and `server.json`) and finding 06 (the dependency source). The stack plan's exit codes match the plan's three codes. The server answers a waiting machine's heartbeats with `HeartbeatAck`, as the plan expects.
6. Each item merges alone: Items 1, 2, 3, 4, 5, 6, 7, 8 and 9 each name their dependencies, and each dependency is an earlier item or a Sprint 1 interface. The `link` package gets code in items 3 to 7, and `scripts/check-go.sh` has no unused-code linter, so a partly built package passes the check. Nothing found.
7. Wave 2 room: The `Open` receiver ignores unknown message kinds, so wave 2 commands and events fit. The command handler has three fixed actions and a table, so a fourth is one more row. Nothing found.
8. Dependencies and versions: Finding 06 for x/sys. The plan adds no other module. The cache holds bbolt v1.4.3, and the plan uses the Sprint 1 store through its interface only.

## Plan questions

- a. Joining and the proof: The client keeps `identity.json` (version, address, machine id, client public key, server public key, certificate key digest) and the private key in `machine.key`, and writes `identity.json` last through `WriteAtomic`, so a crash before it leaves no half identity. It refuses a server whose key does not match by never sending a Proof, but the log record that check C10 reads is not stated (finding 02). A lost reply reuses the durable key and replays the join inside the token's life, which returns the first reply. After the token expires the join cannot be recovered, and the plan gives no instruction for it (finding 07).
- b. Reconnecting: The waits are 1, 2, 4, 8 and then 15 seconds, so the client retries at least every 15 seconds, and the wait resets after 30 seconds of a valid stream. A 30-second silence cancels the stream. After a verified final refusal it stops, but the server plan's codes do not match the contract (finding 03). Replay starts from `Welcome.last_event_number` plus one, in number order and once, and facts are captured every 10 seconds during an outage, so C11 sees at least three buffered events. Nothing found against C9 and C11 apart from finding 03.
- c. Running a command: A command that arrives twice runs once, because `Begin` returns the stored result for a known id. The one exception is the crash window in finding 01, which turns a declined command into INTERRUPTED. An expired command is answered EXPIRED, judged by `serverClock`, which is `Welcome.server_time` plus the elapsed time since the Proof was sent; it can only run late by the one-way delay, so it errs toward an upper bound. SetState is always answered DONE and is exempt from expiry, as the contract says. The plan builds against the exact messages of `go-s0-contract` and the exact names of `go-s1-client-local` as they are now. I checked the generated names against the three proto files and the store names against the plan's items.

## Post-build checks

1. Kill the client between the decline of an expired command and `Finish`, then restart it. Expected: the server receives EXPIRED, never INTERRUPTED (finding 01).
2. Run check C10. Expected: the log holds `link_refused` with code `server_key_mismatch` within 20 seconds, and the stand-in server saw no Proof frame (finding 02).
3. Feed a stand-in server every code of the contract table, plus `unauthenticated` and `out_of_range`. Expected: only the verified `unknown_machine` and `link_revision_mismatch` refusals stop the client, and all others retry within 15 seconds (finding 03).
4. Stop the server for at least 35 seconds while the client captures facts. Expected: at least three buffered events, and after the server returns the server's log lists the same numbers in order, each once (C9, C11).
5. Run the combined tests on a machine with no PostgreSQL. Expected: they skip with a clear message, and `scripts/check-go.sh` stays green (finding 04).
6. Damage `client.db`, then start the service. Expected: exit 1, a log record with the cause, and a restart after 5 seconds (finding 05).
7. Run `go mod tidy -diff` after the first import of x/sys. Expected: no difference, and x/sys is at v0.50.0 (finding 06).
8. On macOS, put the machine to sleep for a minute during a stream and then send a command that expires during the sleep. Expected: `serverClock` counts the sleep, and the command is answered EXPIRED (the `clock_darwin.go` build is also checked).
9. Join, drop the reply, and rejoin within the token's life, and again after it. Expected: the first returns the same machine id, the second gives the message from finding 07.
10. Run all link tests with `-race`. Expected: no data race between the reporter, the replay sender and the command handler.

## Grade

6 out of 10. The plan is well grounded in the contract and the Sprint 1 interfaces, and its retry, replay and key handling are sound. Four majors, each with a short fix, keep it from shipping as written. Counts: 0 blockers, 4 majors, 4 minors.
