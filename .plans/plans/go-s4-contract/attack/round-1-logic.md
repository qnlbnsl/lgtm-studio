REVISE

The planner, test writers, builders, and reviewer are all OpenAI workers in separate sessions.

**Top finding.** The server can detect lost event history before Welcome, but its planned refusal makes the client retry indefinitely without telling the owner how to recover.

## Findings

### logic-r1 01 — MAJOR — C11

The client has no recovery path when the server alone can detect the lost history. The plan stops the client on an incompatible Welcome, but refuses a contradictory run before Welcome with the existing retryable `invalid_argument`. If the server has applied through event 5 and staged a result through event 8, an otherwise readable backup with next event 7 passes the client's local checks. It never receives the Welcome needed to run the other checks. The supervisor redials with the same boundary and logs a generic disconnect. The result remains pending and the owner sees no recovery instruction.

In item 2, distinguish a malformed announcement, which can keep the existing retry rule, from a valid announcement that contradicts the server's saved history. Give the latter a fixed terminal reason after verified proof. Add it to the link code table, its test, and both program handoffs. Require the client to stop with one recovery sentence and leave the enrollment and server state intact. Test the server-only rejection, a successful replay after restoring the correct prefix, and removal when that prefix cannot be restored.

Evidence by reading. `.plans/plans/go-s4-contract/plan.md:149–153`, `src/client/internal/link/store.go:320–351`, `src/client/internal/link/conn.go:206–242`, `src/client/internal/link/retry.go:39–68`, `src/client/internal/link/session.go:257–273`, `src/contracts/testdata/link-codes.json:4–9`.

Design, Contract and both Link ends. Confirmed plan gap. Risk 120 (6 for a machine that cannot recover by itself, 5 for recovery intervention, 4 for a restored but incomplete buffer).

### logic-r1 02 — MAJOR — R7

The admission rule protects the boundary of a ready run and leaves a pending run's boundary unprotected. If the mark is 5, an authenticated run announces first event 10, and the stream drops before reaching 9, the same run can reconnect announcing 6. With no staged result and no recorded ready pair for that run, the stated validations permit immediate readiness. Work can reach the new process before events 6 through 9 are applied or consumed. A server restart makes a check kept only on the stream insufficient.

In item 2, require admission to preserve the announced pair durably while it drains. Under the machine lock, reject a changed boundary for the same pending or ready run. Specify how a newer process replaces a pending announcement. Allow it to repeat the boundary if the previous process appended nothing, and forbid it to move that boundary backward. Keep the ready-run callback on the single transition that reaches the fence. Test the two admissions across a server restart and require no permit, claim, or callback on the changed boundary.

Evidence by reading. `.plans/plans/go-s4-contract/plan.md:143`, `.plans/plans/go-s4-contract/plan.md:151`, `docs/specs/2026-11-27-running-an-agent-design.md:115–124`, `src/server/internal/link/presence.go:43–84`.

Design, Contract and Server Link. Confirmed plan gap. Risk 60 (6 for early command execution, 5 for ledger reconciliation, 2 for a reconnect with an inconsistent announcement).

### logic-r1 03 — MAJOR — R7

The plan says a command outcome changes only an open operation and leaves the converse implicit. A folder report can arrive for the first time after the sweep has closed the operation. For example, the client persists a successful `FolderSeen`, loses its connection before delivering the report or result, and returns after the command is swept. A handler that only checks report identity and whether it has seen the report before can create the repository on replay. The same keyed request can then return success after it already returned a final unknown outcome.

Require both callbacks to transition only an open operation under the machine lock. A late report for a terminal operation still advances the event mark and receives its acknowledgement, with no Projects change. A report applied before the result in the same write still wins. Add the orderings report then settlement, settlement then first report, and both orderings against removal to the handoffs and tests.

Evidence by reading. `.plans/plans/go-s4-contract/plan.md:145`, `.plans/plans/go-s4-contract/plan.md:155`, `.plans/plans/go-s4-contract/plan.md:223`, `docs/specs/2026-11-27-running-an-agent-design.md:217–232`, `src/server/internal/link/outbox_sweep.go:117–143`, `src/server/internal/link/inbox.go:27–72`.

Design, Contract and Projects. Confirmed plan gap. Risk 60 (6 for a stored repository after a final failure, 5 for reconciliation, 2 for a long outage after a report was buffered).

### logic-r1 04 — MAJOR — R4

The request-key fallback does not preserve the existing caller-wide key scope after pruning. The store locks and looks up by caller and key, with the procedure included in the fingerprint. After an open inspection's generic receipt is pruned, a call to add with the same key can miss a fallback indexed by caller, procedure, and key. An unrelated keyed procedure can also claim the released key and make the original inspection retry fail before the Projects callback runs.

In item 3 and the Projects handoff, keep one caller-and-key reservation while the folder operation remains retrievable. Compare procedure and request fingerprint within that reservation. Require the ordinary store path and its pruner to respect the same reservation, through the Store interface. Pin three cases after the ordinary retention cutoff. The same inspect returns the original operation, a changed inspect is refused, and a different keyed procedure is refused without a second write.

Evidence by reading. `.plans/plans/go-s4-contract/plan.md:199`, `.plans/plans/go-s4-contract/plan.md:223`, `.foundry/go-rewrite/results/plan-go-s4-contract.md:42`, `src/server/internal/store/receipts.go:41–43`, `src/server/internal/store/receipts.go:69–100`, `src/server/internal/store/receipts.go:128–137`, `src/server/internal/store/prune.go:267–283`.

Design, Contract and Server Projects/Store. Confirmed plan gap. Risk 60 (6 for a lost old result or a second write, 5 for reconciliation, 2 for reuse after pruning).

### logic-r1 05 — MAJOR — R7

The stable-watermark promise covers every result, but `SetState` has no receipt in the built client. It applies state and replies before reaching the receipt path. A builder can add the current high-water number to the shared reply function, lose the first `SetState` reply, and then return a larger number on its next attempt after a periodic report was buffered. The repeated result acquires an unrelated dependency and can be held for an event the first result never depended on.

In item 2 and the Client handoff, say explicitly how `SetState` obtains a saved watermark. Store the first reply's watermark by command identifier before handing it off and replay it under the latest attempt. Preserve the existing approval, revision and expiry behavior for standing state. Require a crash or reconnect test that drops the first reply, appends another event, and sees the first number on replay. Separate a positively identified revision 1 receipt, which uses zero, from damaged revision 2 receipt metadata.

Evidence by reading. `.plans/plans/go-s4-contract/plan.md:139`, `.plans/plans/go-s4-contract/plan.md:145`, `.foundry/go-rewrite/results/plan-go-s4-contract.md:43`, `docs/specs/2026-11-27-running-an-agent-design.md:88–94`, `src/client/internal/link/commands.go:132–149`, `src/client/internal/link/commands.go:179–199`, `src/client/internal/link/commands.go:299–312`, `src/client/internal/link/receipts.go:116–135`.

Design, Contract and Client Link. Confirmed plan gap. Risk 48 (6 for a standing command that can remain unresolved, 4 for recovery of one machine, 2 for a lost reply followed by a new buffered event).

### logic-r1 06 — MAJOR — D5

The existing-folder return and repository comparison have no stated precedence, and the normal form of a remote is unspecified. An owner can add a folder, change its origin, and submit a new request for the same resolved folder and repository. One reading returns the stored folder and another refuses it because the origins differ. A new attachment also has no fixed answer when an origin differs only by a trailing slash or suffix.

In item 3, decide existing ownership under the machine lock first. Return the stored folder for the same target and refuse another target. For a new association, define one exact normalization rule for sanitized remotes, including the treatment of transport, case, port, path, suffix, and trailing slash or a rule of conservative exact equality. Give the Projects plan examples of equivalent and distinct remotes, a repeat after an origin change, and the known-absent and withheld-origin combinations.

Evidence by reading. `.plans/plans/go-s4-contract/plan.md:225`, `docs/specs/2026-11-27-running-an-agent-design.md:202–211`, `docs/specs/2026-11-27-running-an-agent-design.md:289–294`.

Design, Contract and Projects. Confirmed plan gap. Risk 108 (6 for a valid add that can be refused, 3 for one owner who can repair the input, 6 for an ordinary repository operation with two origin spellings).

### logic-r1 07 — MINOR — R8

Several status codes in the dependent names table have no corresponding exact assertion. Item 3's source test targets authentication and key use. Its descriptor and round-trip tests do not read the refusal mappings. A builder could document a missing repository as `failed_precondition`, disagree with the table's `not_found`, and pass the listed contract tests. A dependent website could then send the owner to the wrong recovery action.

Put all fixed condition-to-code and sentence rules in one contract fixture or a source table checked by a contract test. Require one mutation of a code and a downstream endpoint check against the same entries. Bind the full discovery name to the generated service descriptor and keep the existing tests for the two sample routes.

Evidence by reading. `.foundry/go-rewrite/results/plan-go-s4-contract.md:115–124`, `.plans/plans/go-s4-contract/plan.md:197–201`, `.plans/plans/go-s4-contract/plan.md:227–247`, `.plans/plans/go-s4-contract/plan.md:251–254`.

Design, Contract. Confirmed verification gap. Risk 6 (1 for the missing direct check, 1 for effect during development, 6 because a handoff reader can reasonably rely on the table).

## Answers to the attack questions

1. The client counter orders events. Repeats receive the durable cumulative acknowledgement, gaps close the stream, and `2^63` is a drain boundary, with no event wrap. The machine lock orders command writes, the attempt increases without wrapping, the latest dispatched attempt wins, and commit order governs watch positions. Server time governs the 20-second expiry and the inherited 23-hour offer and 24-hour receipt and sweep rules. Changed pending boundaries and the `SetState` replay case are findings 02 and 05. Evidence is in `.plans/plans/go-s4-contract/plan.md:139–145`, `.plans/plans/go-s4-contract/plan.md:221–223`, `src/server/internal/link/outbox_claim.go:54–79`, and `src/contracts/proto/lgtm/v1/common.proto:25–36`.
2. A failed database write leaves report, mark and outcome unchanged and produces no event acknowledgement. A lost reply gets a durable replay. The caller's 25-second timeout does not write a failure. Existing join-token precedence, session expiry and clock bounds carry forward. The recovery failure before Welcome and the request-key failure after pruning are findings 01 and 04. Evidence is in `.plans/plans/go-s4-contract/plan.md:138–153`, `.plans/plans/go-s4-contract/plan.md:221–247`, and `src/contracts/proto/lgtm/link/v1/link.proto:80–101`.
3. A current matching result with watermark at or below the mark settles now. Above the mark it stages and waits. A duplicate staged result has no effect, and removal ends it as removed. An obsolete stream or wrong attempt cannot settle it. A report that reaches the fence in the same write runs first. The uncovered combinations are a changed pending run, a first report after a terminal operation, a cross-procedure reuse after pruning, and an existing folder with changed identity facts. See findings 02–04 and 06.
4. Zero first event and an empty run identifier fail revision 2 admission. Zero result watermark means no prior event or a revision 1 receipt. An unset report arm or zero or unknown refusal is consumed without effect. An empty typed path fails validation. An absent repository identifier selects create-or-reuse, and a present empty identifier is invalid. False clean means dirty. A missing service list is empty. The new watch sets a position even when its initial value is zero, which means no visible changes. Test explicit required strings and damaged revision 2 receipts in the respective dependent runtime cases. Evidence is in `.plans/plans/go-s4-contract/plan.md:143–157`, `.plans/plans/go-s4-contract/plan.md:221–225`, `.plans/plans/go-s4-contract/plan.md:251`, and `src/contracts/proto/lgtm/v1/common.proto:30–36`.
5. Admission, event application, result receipt, claim, sweep, and removal contend on one machine lock. The old stream loses its write right at replacement. On the client the store serializes appends with receipt changes. On the server the receipt lock serializes contacts with the same key. The late report and pruning paths need explicit guards as in findings 03 and 04. Evidence is in `src/server/internal/link/inbox.go:27–57`, `src/server/internal/link/outbox_result.go:40–53`, `src/server/internal/link/outbox_sweep.go:117–143`, `src/client/internal/link/outbox.go:23–54`, and `src/server/internal/store/receipts.go:71–100`.
6. By reading, item 1's line-comparison mutation changes the outcome of the scratch comparator. Item 2's tag, presence, size, comment and hash mutations have distinct assertions. Item 3's stream shape, position and required header mutations reach the described tests. Item 4's tag and cardinality mutations reach descriptor assertions and its short name mutation must compare with the exact string. The result callback could still settle before applying its report, or a refusal could map to the wrong status, while the contract tests pass. Evidence is in `.plans/plans/go-s4-contract/plan.md:74–77`, `.plans/plans/go-s4-contract/plan.md:109–115`, `.plans/plans/go-s4-contract/plan.md:199–201`, and `.plans/plans/go-s4-contract/plan.md:249–254`.
7. Item 1's build gate catches an edited or displaced old line. A documentation assertion can still accept the right phrases beside conflicting instructions. Item 2 can document the fence and replay a fresh watermark in the program. Item 3 can satisfy every descriptor and use the wrong refusal mapping or overwrite a terminal result. Item 4 can add the correct discovery descriptor while the future website fails to enforce the sample flag. Those runtime mutations belong to the named dependent gates. The explicit refusal assertions are missing as in finding 07.
8. Store the pending run's pair as one durable admission record. Make folder operations conditional transitions out of open and enforce one canonical resolved folder per machine. Keep one reservation per caller and key while its operation is retrievable. Persist an outcome and its watermark together, with a version that distinguishes legacy receipts. See findings 02–05. The two result arms already share a union.

- **a.** Item 2 covers the additive revision 2 fields, proof, acknowledgement and budgets. Items 1, 3 and 4 cover freeze, repository calls, watch and discovery. Program constants and installed-client upgrade are named handoffs. Sprint 5's agent, thread and repository-creation declarations stay outside the contract. The recovery, settlement and retry gaps in findings 01–06 prevent a complete sprint 4 handoff. Evidence is in `docs/specs/2026-11-27-running-an-agent-design.md:168–302`, `docs/specs/2026-11-27-running-an-agent-design.md:317–320`, and `.foundry/go-rewrite/results/plan-go-s4-contract.md:39–45`.
- **b.** Nothing found. Confirmed live at `19873537`. The working record equals the pinned revision 1 blob, its three hashes match the source, and the existing Go contract suite passed. In a scratch probe using the script's comparison program, the leading-comma append passed against revision 1 and revision 2. A trailing comma on the old line, a moved old line, and a revision 2 hash rewrite failed under the respective baselines. The full gate for the built revision 2 candidate remains a build check. Evidence is in `src/contracts/freeze_test.go:31–107`, `src/contracts/testdata/link-frozen.json:2`, `scripts/check-go.sh:20–49`, and `.plans/plans/go-s4-contract/plan.md:74–87`.
- **c.** Nothing found. By reading, current action tags 10–12 and report tags 10–11 leave room for revision 2 tags 13 and 12. Six more action tags fit at 14–19 and seven more report tags fit at 13–19 if the next plan chooses them. This leaves every existing tag and the result reservation intact. Evidence is in `src/contracts/proto/lgtm/link/v1/commands.proto:41–80`, `src/contracts/proto/lgtm/link/v1/events.proto:49–58`, and `.plans/plans/go-s4-contract/plan.md:117–134`.
- **d.** The built storage and transaction boundaries can support local history validation, staging, real replay, and removal, after the two ends implement revision 2. The server-only contradiction cannot enter the client's current local fault path with the planned response. The built receipt path also requires the changes in finding 05. See findings 01 and 02 and `src/server/internal/link/inbox.go:27–72`, `src/server/internal/link/outbox.go:21–46`, `src/client/internal/link/store.go:320–351`, and `src/client/internal/link/receipts.go:116–178`.
- **e.** Nothing found in the additive discovery change. Confirmed live in a temporary copy with tag 6 generated. Both unchanged TypeScript consumers typechecked. The shared engine's in-page and cookie-authenticated mock route sets returned revision 1, an empty services list, and a first Machines snapshot. The website built with a stub for development certificate interface discovery, which the sandbox blocked. Browser and mock listener checks remain post-build. Evidence is in `src/frontend/src/server/api.ts:14–47`, `src/mock-server/src/engine.ts:58–73`, `src/mock-server/src/engine.ts:144–166`, and `src/mock-server/src/serve.ts:53–54`.
- **f.** The wire layout, procedures, exact limits, and service identity in the handoff table map to items 2–4 and their descriptor, fixture or discovery tests. The website status rows lack direct assertions, and the existing link refusal proof misses the new recovery case. See findings 01 and 07 and `.foundry/go-rewrite/results/plan-go-s4-contract.md:51–124`.
- **g.** The planner, test writers, builders, and reviewer are all OpenAI workers. The plan records separate sessions in `.plans/plans/go-s4-contract/plan.md:327`.

## Post-build checks

- **logic-r1 P1.** The Stack and Link teams should use the paired-program fixture and real PostgreSQL. Interrupt the database write after the folder report callback but before the mark and result commit. The first attempt leaves no effect or acknowledgement. Replay saves the repository once, advances the mark, and settles the saved result in the same commit. Compare the stored operation, command, mark, and frames.
- **logic-r1 P2.** The Stack and Client teams should kill the client after it buffers the folder report and before it finishes the command receipt. Restart twice and append unrelated events in the later processes. The replayed receipt still names the original process's high-water number and the report wins. Include a revision 1 started and completed receipt with watermark zero.
- **logic-r1 P3.** The Stack and Link teams should hold all three send permits and replay the old prefix, including a duplicate event and a consumed next event. Each eligible receipt receives one cumulative acknowledgement and no command is claimed while the gate is closed. On return of a permit, the queued commands arrive in order. A gap receives the fixed refusal without an acknowledgement. Count handoffs to enforce the 32-event window.
- **logic-r1 P4.** The Stack and Website teams should use one built website with the in-page sample, the mock server over its regular listener, an older real server, and the sprint 4 real server. The first three retain their existing workspace. The sprint 4 real server starts the repository watch. Expire a session or switch servers while the watch and an inspection are active, and verify that the old records and late reply are discarded.

## Grade

6 out of 10. 0 blockers, 6 majors, and 1 minor.
