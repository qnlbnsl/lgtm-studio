REVISE

# Round 1 review of the plan go-s3-client-link, lens critic

- Task identifier: review-go-s3-client-link-r1-critic
- Nonce: 36ec05
- Plan read: `.plans/plans/go-s3-client-link/plan.md`, 379 lines, as it stood in round 1.
- Finding identifier prefix: `critic-r1`.

## Top finding

The stack script and the client disagree on the state folder and on the log record of a key mismatch, so checks C5, C6 and C10, which the done criteria rely on, cannot pass as the plan is written (critic-r1 1 and critic-r1 2).

## Findings

### critic-r1 1: The acceptance script and the client use different state folders

- Severity: MAJOR.
- Tied to: C5, C6, C10, and the plan's done criteria 1c, 2b and 2c.
- Evidence:
  - `deploy/test/checks.sh:217` to `deploy/test/checks.sh:235` start `join` and `run` with the variable `XDG_STATE_HOME`.
  - `deploy/test/checks.sh:520` edits the file `$state_home/lgtm-client/identity.json`.
  - `src/client/internal/config/config.go:28` to `src/client/internal/config/config.go:41` read only `LGTM_CLIENT_STATE_DIR` or the home folder default. They never read `XDG_STATE_HOME`.
  - `.plans/plans/go-s3-client-link/scouting.md:74` says that the script "still export[s] the superseded state setting" and that "the result carries both requests".
  - `plan.md:133` to `plan.md:135` use `deploy/test/checks.sh --only` as acceptance. No item or gate of the plan owns the script change. The epic (`.plans/epics/go-rewrite/epic.md:126`) lets only the Stack team edit `deploy/`.
- Failure scenario: The runner calls `lgtm-client join`. The client writes `identity.json` under the runner's real home folder, not under the script's temporary folder. Check C10 then looks for `$state_home/lgtm-client/identity.json`, finds no file, and fails. In the worse case, the check pollutes the real state folder of the person who runs it, and a second run is refused with `ErrEnrollmentExists`.
- Prescribed fix: Name the Stack team's change to `client_join`, `start_client` and `check_c10` as a dependency of this plan, with the exact variable (`LGTM_CLIENT_STATE_DIR`) and the exact path of `identity.json`. Make that change a gate of the plan, so that no criterion that runs the script is claimed before it lands.

### critic-r1 2: A server key mismatch has no specified log record, and the plan retries it

- Severity: MAJOR.
- Tied to: C10, D16.
- Evidence:
  - `deploy/test/checks.sh:343` to `deploy/test/checks.sh:350` (`wait_refusal`) wait 20 seconds for a JSON log line with `.msg` equal to `link_refused` and the expected `.code`.
  - `deploy/test/checks.sh:529` expects the code `server_key_mismatch`.
  - `plan.md:195` lists a "key or certificate mismatch" among the failures that are retried, and says only that a bad server signature logs `server_key_mismatch`. It states no `msg`. A retried failure is not a terminal refusal, and the plan elsewhere ties `link_refused` to terminal refusals.
  - No item states the field names `msg` and `code` for the diagnostics of the stream side.
- Failure scenario: The second half of C10 swaps the saved server key. The client logs a retry line with another `msg`, or one `server_key_mismatch` line the script does not match. The script waits 20 seconds and fails, although the client is behaving as the plan describes.
- Prescribed fix: State the record exactly (`msg` equal to `link_refused`, `code` equal to `server_key_mismatch`) and state whether a mismatch is retried or terminal. Add a test in item 7 that reads the log output and asserts the record. State the same for `unknown_machine`, which C10 also reads.

### critic-r1 3: Receipt retention on the machine's clock can run a command twice, and the decline path misuses `Begin`

- Severity: MAJOR.
- Tied to: R6 and the design's rule that a command that arrives twice runs once (`docs/specs/2026-11-25-go-rewrite-foundation-design.md:284`).
- Evidence:
  - `plan.md:26` accepts that "a clock set ahead can remove a receipt before the server's" expiry.
  - `plan.md:246` says `Finish` also removes finished receipts. The Sprint 1 plan (`.plans/plans/go-s1-client-local/plan.md:424`) prunes with the machine's wall clock, and there is no `Reconcile`, no `StoreOptions.Now` and no server-time input.
  - `commands.proto:12` to `commands.proto:13` promise that a machine answers with the recorded result "until 24 hours after expires_at".
  - `plan.md:244` calls `Begin` with a counterfeit pre-expiry `now` to write a `started` receipt on `DecisionExpired`. A crash between that `Begin` and `Finish` leaves a receipt that answers INTERRUPTED, but the command was never started and should answer EXPIRED. Sprint 1's tests do not describe this use of `Begin`.
- Failure scenario: The machine clock runs 25 hours ahead. A later `Finish` prunes a finished receipt of command C. The server, which never saw the result, sends C again before its server-time expiry. The receipt is gone, so the machine runs C a second time. The same plan states the opposite promise to the owner.
- Prescribed fix: Ask Sprint 1 for a pruning input that comes from server time, or state the accepted bound in an acceptance test (`TestCommandExpiryUsesServerTime` with a clock 25 hours ahead, and the expected result). Add a test for the crash between `Begin` and `Finish` on the decline path, with the expected outcome. Fix the wording of `plan.md:26` so that it does not contradict the contract without a decision row.

### critic-r1 4: "Retry at least every 15 seconds" bounds the wait, not the period

- Severity: MAJOR.
- Tied to: D16, C9.
- Evidence:
  - `plan.md:195` says to wait 1, 2, 4, 8 and then "at most 15 seconds". No dial timeout is stated.
  - The handshake allows 10 seconds for each of three steps (`link.proto:55`, `link.proto:92` and `link.proto:97`) and 30 seconds of silence.
  - The design says: "A client that lost its connection retries at least every 15 seconds" (`docs/specs/2026-11-25-go-rewrite-foundation-design.md:312`). C9 expects a new `connectedAt` within 35 seconds of a server restart.
- Failure scenario: The server address is black-holed, so each dial blocks for the operating system's default timeout, which can be more than a minute. Or a server answers the stream and then stays silent, so each attempt takes up to 30 seconds. The period between starts of attempts is then far above 15 seconds, and C9's 35-second window fails.
- Prescribed fix: State a dial timeout, and define the period as the time between the starts of two attempts. Extend `TestRetryBackoffAndSilence` with a listener that never accepts and a listener that accepts and stays silent, and assert the largest gap between attempt starts.

### critic-r1 5: Permanent stops exit with status 0, so nothing restarts the client and nothing shows the person

- Severity: MAJOR.
- Tied to: D16, and the gap states of the lens.
- Evidence:
  - `plan.md:195`, `plan.md:198` and `plan.md:259` stop for a damaged or missing `client.db` (`ErrStoreMissing`, `ErrStoreDamaged`, `ErrStoreUnsafe`), an impossible mark, and a final refusal.
  - `.plans/plans/go-s3-client-link/decisions.md:19` (row 12) says a refusal exits 0, and row 32 (line 39) says otherwise for some cases. Both are `unanswered`.
  - The Stack team's service policy restarts a nonzero exit only (`scouting.md:74`), so an exit of 0 is not restarted.
  - A refusal without the `Lgtm-Link-Refusal` key becomes the terminal `link_precondition_failed`, so a proxy that removes the end-of-stream metadata turns a transient `event_gap` into a permanent stop.
- Failure scenario: A person deletes `client.db` by accident, or a proxy strips the metadata. The client exits 0, the service manager treats that as a clean stop, and the machine stays offline. The person sees an offline machine and has no line to read.
- Prescribed fix: Give each permanent stop a distinct, documented exit status and one visible line. Retry, with the same backoff, a refusal whose reason is missing or unknown. Add one test per stop that asserts the status and the line.

### critic-r1 6: The trust model stands on an unanswered decision row

- Severity: MAJOR.
- Tied to: D7, D17, and the plan as a whole.
- Evidence:
  - `plan.md:22` presents the two-key trust model as the plan's choice.
  - `decisions.md:15` (row 8, the trust model) is `unanswered`. Rows 9, 10, 11, 12, 14, 16, 17, 18 to 23, 27, 28, 30, 31, 32 and 39 are also `unanswered`.
  - The epic records issue 1633 (where encryption ends inside the stack) as open (`.plans/epics/go-rewrite/epic.md:189`).
- Failure scenario: Issue 1633 is decided with another pin, or with no certificate pin inside the stack. The signature `NewHTTPClient(origin, certificateKeySHA256)`, the key in `identity.json` and the join-token pin all change, after the tests for items 1 and 2 were written and frozen.
- Prescribed fix: Have the orchestrator close row 8 before the tests are frozen, or mark items 1 and 2 blocked on it. List in the overview which unanswered rows each item depends on.

### critic-r1 7: Combined tests and the live check have no steps or expected values

- Severity: MAJOR.
- Tied to: the lens questions 1 and 2, and C8, C9, C11.
- Evidence:
  - `plan.md:74` to `plan.md:76` and `plan.md:133` to `plan.md:135` name `TestJoinReplay`, `TestCommandAttemptsAcrossReconnect`, `TestPresenceAcrossRestart`, `TestKeysAndRemoval` and `TestOutboxReplayAfterServerFailure` with no steps and no expected values.
  - `TestRestartReplaysAfterDroppedResult` appears only in the criteria (`plan.md:225`), under no item.
  - `plan.md:200` lists subjects of `TestRetryBackoffAndSilence` and `TestCommandExpiryUsesServerTime` but no numbers: the waits 1, 2, 4, 8 and 15, the reset after 30 seconds, and the silence cancel at 30 seconds are never asserted.
  - `plan.md:214` asks to "verify the link stays open ... for ten minutes of heartbeats" with no command, no log line and no expected result. Criterion 1c's "observe the waiting row" has the same fault.
- Failure scenario: A test writer from another vendor must invent the expected values. The frozen tests then drift from the design, and the builder cannot change them.
- Prescribed fix: Add one table of the combined tests with steps and expected results, put the numbers into the unit tests, and give the ten-minute check a command, a count of heartbeats and a pass condition.

### critic-r1 8: Two gap states have no stated message, and one instruction contradicts the code

- Severity: MINOR.
- Tied to: D16, and the gap states of the lens.
- Evidence: `plan.md:106` to `plan.md:107` retry `Join` for the budget of 15 minutes against an unavailable server with no stated progress line. `plan.md:258` handles a half-finished earlier join (a pending key and no `identity.json`) with "use a new token with a fresh folder", yet `plan.md:106` reuses the pending key when a new token is used in the same folder.
- Failure scenario: PostgreSQL is down, so `Join` returns `unavailable`. The person waits up to 15 minutes in silence and cannot tell a hang from a retry. After a lost reply, the person follows the instruction, uses the same folder, and the client reuses the key, so the server may hold two waiting machines.
- Prescribed fix: State the progress line and the exact message, fix the instruction so that it matches the code, and test both.

### critic-r1 9: Item 7 is too large for medium, and the build order is wrong in the workstream tables

- Severity: MINOR.
- Tied to: the plan (lens question 5).
- Evidence:
  - `plan.md:188` to `plan.md:201` make item 7 compose four pieces, own their lifetimes, and carry five tests and many failure paths, yet it is sized medium.
  - The WS-2 table (`plan.md:139` to `plan.md:144`) lists items 3, 4, 5 and 7, but item 7 needs item 6, which is in WS-3. Item 5 says both "Runs beside items 1 to 4" and "Integrates after item 4" (`plan.md:174` to `plan.md:186`).
- Failure scenario: A builder who follows the workstream tables builds item 7 before item 6 exists, and two builders edit the same package at once.
- Prescribed fix: Give one global build order, and split item 7 into the supervisor (retry, backoff, silence) and the session (handshake, reports, commands).

### critic-r1 10: Stale and contradictory decision rows

- Severity: MINOR.
- Tied to: the plan.
- Evidence:
  - Row 33 (`decisions.md:40`, closed) says an event gap means re-enrollment. Rows 25 and 38 (`decisions.md:32` and `decisions.md:45`) and `plan.md:155` and `plan.md:195` retry the gap.
  - Row 30 (`decisions.md:37`, conflict answers failed) and row 40 (`decisions.md:47`, done supersedes) disagree, and row 30 stays `unanswered`.
  - Row 12 (`decisions.md:19`) and row 32 (`decisions.md:39`) disagree about exit 0.
  - Rows 13 and 15 (`decisions.md:20` and `decisions.md:22`) name `Store.Reconcile` and `StoreOptions.Now`, which the Sprint 1 plan does not have.
  - Row 5 (`decisions.md:12`) is closed with the text "Not yet".
  - `scouting.md:53` says "The 24 rows" and `plan.md:327` says "Decisions 7 to 24", against 40 rows.
- Failure scenario: A test writer reads row 33 and writes a re-enrollment test that contradicts `TestRetryResumesAfterEventGap`.
- Prescribed fix: Mark the superseded rows, remove the names that do not exist, and correct the counts.

### critic-r1 11: Edits outside `src/client/` and an unverified module pin

- Severity: MINOR.
- Tied to: R1, and the lens question 4.
- Evidence:
  - The epic lets the Client team edit `src/client/` only (`.plans/epics/go-rewrite/epic.md:125`), and lets a team change `go.mod` and `go.sum` only for a module its plan lists (`.plans/epics/go-rewrite/epic.md:132`). The plan needs `go.mod` and `go.sum` (item 3, `plan.md:153`), `src/linktest/` (the orchestrator) and `deploy/test/checks.sh` (the Stack team).
  - The plan uses `golang.org/x/sys` v0.50.0 "with the Sprint 2 sign-in pin", but the current `go.mod` has no `golang.org/x/sys` and no `go.etcd.io/bbolt`. The sign-in plan's pin was not checked.
- Failure scenario: The sign-in plan pins another version. Item 3 then either does not build or forces a change in a file that another team owns.
- Prescribed fix: Name the owner of each file outside `src/client/`, and say that the builder uses the plan's own version if the sign-in pin differs.

### critic-r1 12: Every approved welcome queues new durable reports and runs a scan

- Severity: MINOR.
- Tied to: D16.
- Evidence: `plan.md:197` queues fresh `Facts` and `HarnessesFound` on every approved welcome and runs a scan.
- Failure scenario: A flapping link reconnects 180 times in an hour. The machine adds 360 durable events and runs 180 scans, until `ErrOutboxFull` stops all reports.
- Prescribed fix: Queue them when the standing changes or on the first approved connection of a process, and test that a second reconnect adds no event.

## Answers to the attack questions

Lens questions:

1. Test writers: partly. Items 1 to 6 give test names and subjects, but the combined tests and the numbers of the retry rules are missing (critic-r1 7). The decline path and the crash between `Begin` and `Finish` have no test (critic-r1 3).
2. Acceptance observations: most name a command and an expected result. The bare adjectives are the ten-minute heartbeat check (`plan.md:214`) and "observe the waiting row" in criterion 1c (critic-r1 7).
3. Contradictions: rows 33, 30, 12, 13, 15 and 5 of the decisions log, the counts "24 rows" and "Decisions 7 to 24", the wording of `plan.md:26` against `commands.proto:12` to `commands.proto:13`, and item 2's key reuse against `plan.md:258` (critic-r1 3, 8, 10).
4. Files: every item names exact files. Edits outside `src/client/` are `go.mod`, `go.sum`, `src/linktest/` and `deploy/test/checks.sh` (critic-r1 1, 11).
5. Splitting and order: item 7 is splittable, and the workstream tables put item 7 before item 6 (critic-r1 9). No two parallel items write the same file; nothing found, after comparing the file lists of the nine items.
6. Gap states:
   - Fresh checkout: `run` before `join` exits 1 with an instruction to join.
   - No PostgreSQL: the server answers `unavailable`, and the client retries without a progress line (critic-r1 8).
   - Missing setting: defaults apply, and an invalid settings file exits 1.
   - Second run of `join`: `ErrEnrollmentExists` with no stated message (critic-r1 8).
   - Half-finished earlier run: the pending key and the store are reused, but the instruction text contradicts that (critic-r1 8).
   - Deleted `client.db`: exit 0 and no restart (critic-r1 5).
7. Self-check: gates 1 and 2 are weak, because many rows are `unanswered` yet the plan describes them as settled (critic-r1 6), and the combined tests have no steps (critic-r1 7). Gate 4 holds in form. The other gates pass as written.
8. Writing rules and models: no model or harness name appears in the plan, and acronyms are mostly expanded at first use. Items 6 and 7 pack several claims into one sentence (`plan.md:195` and `plan.md:246`), which breaks the one-claim rule. Nothing else found.

Task questions:

- a. Joining and the proof: the client keeps `identity.json`, `machine.key` and `client.db`. It saves the key before it sends the first request, and it retries a lost reply with the same token and key for up to 15 minutes, so the server holds one waiting machine. It checks the saved server key on every stream, and a bad server signature sends no proof. The weak points are the log record that C10 reads (critic-r1 2) and the half-finished-join message (critic-r1 8).
- b. Reconnecting: events are replayed from the server's mark plus one, in order, once, and a trusted refusal stops the client. The period of retries is not bounded by 15 seconds in the worst case (critic-r1 4), and a stop exits 0 (critic-r1 5). A gap that never closes loops forever with one unchanged log line. That case is outside wave 1 under issues 1641 and 1642.
- c. Running a command: a repeated command runs once through the receipts, except when the machine clock runs fast (critic-r1 3). An expired command answers EXPIRED, and a failed clock read makes every new command except SetState answer EXPIRED, with no log line specified. The plan's use of the contract matches the messages in `link.proto`, `commands.proto` and `events.proto`. Against Sprint 1 the store names match (`CreateStore`, `OpenStore`, `Read`, `Ack`, `Next`, `Begin`, `Finish` and the errors). `Store.Reconcile` and `StoreOptions.Now` in the decisions log do not exist (critic-r1 10). The names `sysinfo.ReadFacts`, `sysinfo.ReadLoad`, `harness.Scan`, `harness.Definitions` and `harness.DefaultScanOptions` could not be confirmed in the current tree: `src/client/internal/harness/` holds no Go file, and `src/client/internal/sysinfo/` is untracked.

## Post-build checks

1. Scenario: the join reply is lost (a proxy drops it once), then the client retries. Expected: one waiting machine on the Machines page, and one `identity.json`.
2. Scenario: `serverPublicKey` in `identity.json` is replaced by another key while the client runs. Expected: within 20 seconds one log line with `msg` equal to `link_refused` and `code` equal to `server_key_mismatch`, and no proof sent.
3. Scenario: the server address is black-holed. Expected: a new attempt starts at least every 15 seconds plus the stated dial timeout.
4. Scenario: the client is killed during a command, and the server sends the command again. Expected: one effect, and the answer INTERRUPTED.
5. Scenario: the machine clock is 25 hours ahead, a command finishes, and the server sends it again before its expiry. Expected: the recorded result is answered and the command does not run twice.
6. Scenario: the server is stopped for 60 seconds while three or more events are buffered. Expected: all events are applied once and in order, as C11 checks.
7. Scenario: `client.db` is deleted. Expected: a documented nonzero exit status and one visible line.
8. Scenario: the link flaps 20 times in a minute. Expected: the number of durable events stays bounded, and at most one scan runs for each change of standing.

## Grade

- Grade: 5 out of 10.
- Blockers: 0.
- Majors: 7.
- Minors: 5.
