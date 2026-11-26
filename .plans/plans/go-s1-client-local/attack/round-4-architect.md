REVISE

An enrolled client can silently recreate a missing `client.db` and run a command whose receipt it lost, because a server that has applied no events sends the same welcome number for a new store and for the missing history.

## Findings

The review compares `b68adba0` with `809ab00b` for the client-local plan. The plan, decisions and scouting files matched the review target when checked at `ac1ff06a`. References to the link contract use the working Sprint 0 drafts, including changes that have yet to be frozen.

### architect-r4 01 — BLOCKER — R6 — An enrolled machine can recreate a missing command history

**By reading.** `OpenStore` creates `client.db` whenever it is absent. `join` writes `identity.json` after server confirmation, and `run` only needs a valid identity before it uses the store. The first `Welcome` checks the server's event number against `Next()`. Commands and events have separate records and the server can send `SetState` before it has applied any event. See `.plans/plans/go-s1-client-local/plan.md:414`, `.plans/plans/go-s1-client-local/plan.md:499`, `.plans/plans/go-s1-client-local/plan.md:509`, `.plans/plans/go-s3-client-link/plan.md:198`, `.plans/plans/go-s3-client-link/plan.md:201` and `docs/specs/2026-11-25-go-rewrite-foundation-design.md:283`.

**Failure.** A machine finishes a command before its first event reaches the server. Its terminal result is lost on disconnect, and `client.db` is subsequently removed or lost while the enrollment and key survive. The service makes a fresh store with `Next() == 1`. The genuine `Welcome` reports 0, so the freshly initialized store accepts it and a retry of a still-valid command returns `DecisionRun`. The command history is gone even if the action's effect remains. No rollback to an earlier copy is needed.

**Fix.** Make store creation part of enrollment, under the folder lock, and sync the new store's name before publishing `identity.json`. A retry of partial enrollment reuses its already initialized store. A normal run with an identity and no store returns a terminal local error before dialing and leaves the files in place. Name the creation and existing-only open entry points for item 6 and the Sprint 3 enrollment and runtime callers. Test the missing-store case with a pending server command and a server event mark of zero.

### architect-r4 02 — BLOCKER — C11 — The new acknowledgement rule can delete an unsent report

**By reading.** Rule 2 tells the sender to forward a current-stream `EventAck.through` straight to `Ack`. The store checks only `n < Next()`. The Server Machines plan writes an acknowledgement after application commits, and the Client Link plan already specifies a separate in-memory ceiling and a test to reject an impossible acknowledgement. The local handoff omits what that ceiling must become when durable `MarkSent` is removed. See `.plans/plans/go-s1-client-local/plan.md:419`, `.plans/plans/go-s1-client-local/plan.md:499`, `.plans/plans/go-s3-client-link/plan.md:172`, `.plans/plans/go-s3-client-link/plan.md:175` and `.plans/plans/go-s3-server-machines/plan.md:795`.

**Failure.** Events 1 to 3 are queued and `Next()` is 4. The client writes only event 1. If an authenticated server emits `through = 3` because of a regression, a sender following the local handoff deletes all three. Reports 2 and 3 are permanently lost. `TestAckRefusesAhead` exercises only numbers at or above `Next()`, so it misses this case.

**Fix.** Keep the store's cumulative acknowledgement and remove the durable sent mark. Require the Sprint 3 stream owner to maintain the current connection's highest successfully offered number in memory, initialized from the authenticated welcome. Serialize send completion with acknowledgement handling. Reject an acknowledgement above that ceiling before calling `Ack` and use the next authenticated welcome to resolve an uncertain send. Keep both the impossible acknowledgement and the immediate valid acknowledgement cases in the Client Link test.

### architect-r4 03 — BLOCKER — R6 — Pruning by count can forget a result the server is still allowed to request

**By reading.** `Finish` evicts the finished receipt with the lowest start order when the count exceeds its limit. `Begin` treats an absent receipt as a new command until expiry and reports it as expired afterward. The draft still lets Server Machines send a higher attempt until it holds a final result, and requires the client to replay a recorded result for 24 hours after expiry. Neither the plan nor the draft enforces the claimed bound on commands that can finish during that interval. See `.plans/plans/go-s1-client-local/plan.md:421`, `.plans/plans/go-s1-client-local/plan.md:484`, `.plans/plans/go-s1-client-local/decisions.md:81`, `.plans/plans/go-s0-contract/draft/commands.proto:6` and `.plans/plans/go-s3-server-machines/plan.md:892`.

**Failure.** A command finishes and the server never durably records its result. When 1,000 later completions evict that receipt, a subsequent attempt before expiry can run its effect again. A subsequent attempt after expiry and inside the retry window instead returns `EXPIRED`, permanently replacing the actual result on the server. The same transition is reproducible with one later completion and `MaxReceipts = 1`, which the public options allow.

**Fix.** Take this back to the owner before freezing either sprint's tests. Give the Contract, Server and Client plans one condition that proves the server cannot request a receipt again. One option preserves count-based cleanup for confirmed receipts and retains an unconfirmed receipt until the server has durably accepted the result or otherwise guaranteed that it will never redeliver that identifier. The count may bound the former, and any hard total cap needs admission backpressure that preserves the latter. If the existing wire contract is kept, preserve its replay window. Freeze tests for a lost terminal result with more completions than the count cap, with the replay both before and after expiry.

### architect-r4 04 — MAJOR — C11 — The event walk accepts a missing tail and cannot distinguish two empty histories

**By reading.** Consecutive keys and `last < next` are the new local invariants. `Ack` removes a prefix, and the server rejects an event above its next expected number. The new kill test also removes the old explicit acknowledged-number oracle while still asking whether every printed number was acknowledged. See `.plans/plans/go-s1-client-local/plan.md:412`, `.plans/plans/go-s1-client-local/plan.md:415`, `.plans/plans/go-s1-client-local/plan.md:431`, `.plans/plans/go-s1-client-local/plan.md:450` and `.plans/plans/go-s0-contract/draft/events.proto:13`.

**Confirmed live in a model of the written rules.** With `next = 6`, the walk accepts keys `3, 4` and an empty bucket. In the first case, a server at 2 can apply 3 and 4 but then refuses 6 because 5 is missing. In the empty case the same server refuses 6 immediately. An empty bucket with a server at 5 is valid, so simply refusing empty buckets would break normal restarts.

**Fix.** Require positive event numbers and `last == next - 1` whenever the queue is nonempty. Check the metadata encoding and prevent sequence wrap before any append. Keep the validation read-only. At the authenticated welcome, compute the local prefix from the first pending key minus one or from `next - 1` if empty. Refuse a lower server mark with a local gap diagnostic before appending or sending on that session. This needs no durable sent mark or restore procedure. Add fixtures for a missing tail and an empty queue with a lower server mark. Give the crash tests an independent record of the server's last applied number, for example by making the parent own application and signalling the child to acknowledge that prefix. Deriving the test's acknowledged prefix from missing keys would hide the defect.

### architect-r4 05 — MAJOR — C11 — A writable library open can modify a nonempty store before the read-only check

**By reading.** Rejecting zero bytes closes the specific initialization case. Both candidate bbolt versions also commit during a default writable `Open` when the file lacks a persisted list of free pages. The plan opens it in that mode before it checks the schema and promises to leave all refused data untouched. See `.plans/plans/go-s1-client-local/plan.md:414`, `.plans/plans/go-s1-client-local/plan.md:416` and `.plans/plans/go-s1-client-local/plan.md:432`. [bbolt v1.5.0 `Open`](https://github.com/etcd-io/bbolt/blob/v1.5.0/db.go#L280-L299), [bbolt v1.4.3 `Open`](https://github.com/etcd-io/bbolt/blob/v1.4.3/db.go#L275-L294).

**Failure.** A nonempty bbolt fixture uses `NoFreelistSync` and has application version `2`. `bolt.Open` with the plan's default writable options writes the free-page list and commits metadata. Only then does the application's read transaction refuse the store for its version. The bytes change on a refusal, before the owner can inspect the original.

**Fix.** Open an existing file in the library's read-only mode for validation, under the state folder claim. Enter writable mode only through an open configuration that performs no implicit commit. Explicitly scope any library format conversion to a separate action. Add a wrong-schema fixture with an unpersisted free-page list, and check the unchanged bytes and release of the folder claim.

### architect-r4 06 — MAJOR — R6 — Decodable but invalid receipts can masquerade as successful results

**By reading.** The patched JavaScript Object Notation (JSON) receipt has a version, start order, state and conditional result. `Begin` only gives the damaged result when a value does not decode. The test covers `{`, which tests syntax only. The patch removed the explicit schema checks from `b68adba0` and the existing write path prunes by interpreting the state and order of the same values. See `.plans/plans/go-s1-client-local/plan.md:412`, `.plans/plans/go-s1-client-local/plan.md:421`, `.plans/plans/go-s1-client-local/plan.md:443` and `.plans/plans/go-s1-client-local/decisions.md:90`.

**Failure.** A receipt under a valid command identifier contains `version = 2`, `order = 0`, `state = finished` and a decodable result. A reader implementing only the written decode and state checks either sends the incompatible result as final or treats the receipt as an ordinary candidate for eviction. After eviction, a still-valid retry can start the action again. A receipt with an unknown state has no defined branch at all.

**Fix.** Define one receipt validator shared by `Begin` and the pruning scan. Require schema version 1, a valid positive order, one of the two declared states, and the expected result field and bound for that state. On invalid stored data, keep the bytes, report `DecisionInterrupted` for the identifier and exclude the value from pruning. Retain the two written states. Add parseable invalid-version, invalid-state and impossible-field fixtures, including a finish that would otherwise evict the damaged record.

### architect-r4 07 — MAJOR — plan — The new store interface has an incompatible Sprint 3 caller

**By reading.** The item 6 writer removes `Reconcile`, `MarkSent`, `Prune`, `Command.Attempt`, typed receipt outcomes and `LatestAttempt`. It changes `Begin` and `Finish` to take explicit admission time and opaque result bytes, respectively. The Client Link replay and command tasks and their frozen-test specifications still use the old members. The updated wire draft also exempts `SetState` from expiry while the new `Begin` denies every missing receipt at or after expiry. See `.plans/plans/go-s1-client-local/plan.md:405`, `.plans/plans/go-s1-client-local/plan.md:503`, `.plans/plans/go-s1-client-local/decisions.md:91`, `.plans/plans/go-s3-client-link/plan.md:172`, `.plans/plans/go-s3-client-link/plan.md:248` and `.plans/plans/go-s0-contract/draft/commands.proto:14`.

**Failure.** The Sprint 3 test writer freezes the current `TestReplayReconcilesToServerPrefix` and command tests. Their methods and fields do not exist in the Sprint 1 store. If the caller only adapts those signatures, a delayed `SetState` with an unseen identifier can be reported as expired and skipped even though the authoritative command revision is newer. An opaque result encoded with the original wire attempt can also be replayed with the wrong attempt and ignored by the server.

**Fix.** Make alignment of the Sprint 1 store, Client Link plan and wire contract a dependency of freezing the Sprint 3 tests. Replace the moved-number and pruning-clock test cases with agreed cases for the new interface. Give valid `SetState` a defined admission path consistent with the reviewed wire contract. Define the local finished result as an attempt-independent encoding of outcome and message, written by the Client Link command handler and decoded by its replay path, with the server reading a new `CommandResult` carrying the current eligible attempt. Keep the attempt and clock handoff explicit alongside the exact signatures.

### architect-r4 08 — MAJOR — D19 — The state-folder handoff still leaves the installer and fixtures on older rules

**By reading.** The new resolver uses `LGTM_CLIENT_STATE_DIR` or the home default. The fold's result names Stack's needed changes, and the installer already passes one chosen directory to its enrollment and user service. The Stack plan still checks and fixtures `server.json`, gives its own resolution rule precedence to `XDG_STATE_HOME`, and assumes the stack checks use that variable. The live checks set it for both entry points and read public keys from `identity.json`. See `.plans/plans/go-s1-client-local/plan.md:129`, `.plans/plans/go-s1-client-local/plan.md:134`, `.plans/plans/go-s3-stack-client/plan.md:224`, `.plans/plans/go-s3-stack-client/plan.md:253`, `.plans/plans/go-s3-stack-client/decisions.md:23`, `deploy/test/checks.sh:219`, `deploy/test/checks.sh:233` and `deploy/test/checks.sh:520`.

**Failure.** Reinstalling an already enrolled machine with `identity.json` makes the installer request another token and call `join`. The existing identity makes `join` refuse. Separately, a fixture that sets only `XDG_STATE_HOME` falls through to the real home directory, where it can hit an existing machine or leave state outside its fixture.

**Fix.** Before freezing the Stack plan, make its installer, both service templates, fixture writer and reinstall tests agree on the chosen absolute directory and `identity.json`. The installer either uses the same override and home default or documents how a foreground invocation selects its explicitly managed directory. Have Stack pass `LGTM_CLIENT_STATE_DIR` to both fixture invocations and align the test README. Exercise conflicting shell and service environments and a second install with an existing identity.

### architect-r4 09 — MAJOR — plan — The proposed dependency pre-commit cannot pass the Go gate

**By reading.** The merged item 6 waits for bbolt in `go.mod` and the fold asks the Contract team to add it separately. The epic gives post-Sprint 0 module edits to the orchestrator, while `scripts/check-go.sh` runs `go mod tidy -diff`. There is no earlier importing work item. See `.plans/plans/go-s1-client-local/plan.md:79`, `.plans/plans/go-s1-client-local/plan.md:401`, `.plans/plans/go-s1-client-local/plan.md:578`, `.plans/epics/go-rewrite/epic.md:129` and `scripts/check-go.sh:16`.

**Confirmed live with Go 1.26.0.** A scratch module with an unused direct requirement gave a removal diff and exit 1. Adding its first import to that same snapshot gave exit 0. Pre-pinning bbolt on a standalone base commit leaves the gate red, and tidying away the pin leaves item 6 unable to use it from that base.

**Fix.** State the exact bbolt version and put the orchestrator's `go.mod` and `go.sum` edits in the same reviewed merge unit as item 6's first import. Check the combined snapshot with `scripts/check-go.sh` before merging. Keep item 2's independent merge and item 3's parallel build.

## Answers to the round-four questions

### 1. What the registered patches fix

| Register patch | Result |
|---|---|
| 1. Overview, rollout and owner choices | The exclusions make rollback of an older copy out of scope. A missing store under an existing identity remains a live case. The dependency order has the exact-commit problem in findings 01 and 09. Decision 79 explicitly leaves the expired-token choice to the owner. |
| 2. State-folder agreement and layout | The client rule is clear, and the layout matches the Client Link writer and the user-flow key check. The installer and fixture readers still disagree with it, as finding 08 records. |
| 3. Item 1 | `TestStateDirRule` proves the new precedence, including an ignored `XDG_STATE_HOME`. It cannot settle which state directory the Stack callers actually pass. |
| 4. Items 2 and 3 | The merged key and write tests retain the crash, placement and key refusal cases they claim. The state hooks remain callable from item 6, and the credential still depends only on item 2. |
| 5. Item 5 | The change removes the scanner's separate trust policy and the diagnostics that depended on it. The prior link-mode and filtered-directory failures from groups C and L no longer follow from the scanner rules. The scanner stays inside the client. |
| 6. Item 6 | Merging both parts closes group D's dependency cycle. The new mutex closes group J's admission gap if it remains held through the failure record. A zero-size refusal and a full walk close the exact original empty-file and interior-gap tests. Missing history, invalid acknowledgement, premature receipt eviction and incomplete validation remain in findings 01–07. |
| 7. Client Link handoff | Groups E and F correctly move report creation and freshness out of this store. Client Link already proposes numbered `Facts` during outages and lets sequence, rather than producer time, determine order. The new store and receipt signatures still need its tests and caller aligned, as findings 02 and 07 record. |
| 8. Post-build checks | The physical power-loss and failed-disk tests belong after the implementation. The outage test still calls the existing check 11, which expects at least three buffered numbers with matching ordered applications. Fix the design gaps in findings 01–07 before those checks run. |
| 9. Traceability, guardrails, sizing and attack log | The six-item map removes the obsolete item 7 dependency. The module handoff remains inconsistent with the exact-commit gate, as finding 09 records. |
| 10. Decision rows 72 to 86 | The rows record the intended changes. The removal of a lower-welcome check leaves no early diagnosis of a queue missing a prefix, and the retention and caller clocks still disagree with the neighboring plans and wire draft. Findings 03, 04 and 07 give the consequences. |
| 11. Scouting | The empty-file and ordinary open probes support their specific cases. The tags show the extra implicit-open write in finding 05. Go 1.26.0 and the root module now exist, and `src/client` targets remain greenfield. See `.plans/plans/go-s1-client-local/scouting.md:178`, `go.mod:3` and `scripts/check-go.sh:16`. |

### 2. What the unreviewed mechanisms do

The open walk in decision 81 misses the queue tail, and its proposed library entry is not uniformly read-only. The computed interruption in decision 83 handles a valid `started` receipt left by a crash and the two possible outcomes of an ambiguous finish. It needs a complete schema check and retention that keeps pending retry history. The earlier tagged decisions on temporary creation, the embedded store and uncertain commits still apply where the patch carries them forward. Temporary creation needs an initial-enrollment owner, and reopening after an uncertain commit must preserve the database's actual state. The earlier epoch, launcher trust and renumbering mechanisms were superseded.

### 3. Whether a rejection lacks evidence

Nothing found. The outcome table records zero rejections. The 17 alternate fixes are rulings and were checked against the preceding failure scenarios. See `.plans/plans/go-s1-client-local/attack/round-3-findings.md:114`.

### 4. Who writes and reads the changed names

| Changed name or format | Writer and readers |
|---|---|
| `LGTM_CLIENT_STATE_DIR`, `config.StateDir`, `settings.json` and `identity.json` | Item 1 defines the resolver. The installer, user-service managers, foreground operator and Stack test supply the variable. Client Link `join` and `run` read it. The owner writes settings and the runtime loads them. Client Link `join` writes the identity. Runtime, the installer's marker check and the key-refusal user-flow test read it. The layout omits the installer as a reader. |
| `state.Folder`, file constants and test hooks | Item 2 writes the folder interface, atomic writes, release function and test hooks. Items 3 and 6 consume the file and claim interfaces. Item 6's tests consume `SetSyncFolderForTest`. Sprint 3 enrollment and runtime consume the folder interface. The installer consumes the marker name and the `main.version` build setting. |
| Removed trust functions and `ProblemUntrusted` | Item 5 owned them. Their listed direct readers were Item 5 tests. The Sprint 3 harness consumer takes `Found.Harness` and `Found.Version` and keeps `Path` and `Problem` local. No neighboring use of the removed constant was found. |
| `client.db`, `meta.next`, the three buckets and count snapshots | Item 6 owns all writes, validation and private format readers. Client Link producers call `Append`, and its sender calls `Read`, `Ack`, `Next` and `Changed`. Its runtime owns open and close. Client Link must retire `MarkSent`, `Reconcile` and their old errors from the replay cases. Initial creation also needs the enrollment caller from finding 01. |
| `Welcome.last_event_number` at field 3, `EventAck.through` at field 1 and `Welcome.server_time` at field 4 | The Contract team writes definitions. Server Machines writes values from committed state and its authenticated clock sample. Client Link proof and replay code reads them and passes acknowledgements and admission time to item 6. The Stack test reads committed event numbers through each program's diagnostic output. |
| `Command`, `Decision`, `Begin`, `Finish`, the receipt encoding and removed attempt fields | Item 6 writes the local interface and receipt format. Client Link's command handler supplies the command identifier, expiry and authenticated time and writes result bytes. Its replay code reads those bytes. Server Machines writes wire attempts and reads outcomes against the current stream. The opaque result encoding and the owner of safe eviction are missing agreements, as findings 03 and 07 record. |
| `go.mod` and `go.sum` | The orchestrator writes the post-Sprint 0 pins and hashes. Item 6 is the first importer, and root build tooling and the Stack build consume the locked module graph. The reviewed plan still names the Contract team as the writer. |

### 5. How the changed rules interact

| State or input | Result prescribed by the patch | Consequence |
|---|---|---|
| New enrollment, no store and server event mark 0 | The client creates the store and accepts the mark. | This is valid for first enrollment. |
| Existing identity, no store and server event mark 0 | The same creation and acceptance occur. | A past command can run again. |
| Store with `Next() == 6`, no pending keys and welcome mark 5 | `Ack(5)` succeeds without a deletion. | This is valid after acknowledging the entire prefix. |
| Same empty queue and welcome mark 2 | `Ack(2)` also succeeds without a deletion. | The next report is refused as a gap. |
| Current-stream acknowledgement below or at the successful-send ceiling and below `Next()` | `Ack` forgets through the supplied number. | The valid prefix is removed. |
| Current-stream acknowledgement above the successful-send ceiling and below `Next()` | The local handoff still calls `Ack`, which succeeds. | The offered prefix can be confused with the larger queued prefix. |
| Authenticated mark at or above `Next()` | `ErrAckAhead` preserves the file and the sender stops. | Client Link and Stack need a terminal exit outcome that the service manager will not restart automatically. |
| Valid finished receipt, before or after expiry | `DecisionReplay`. | The saved result remains authoritative. |
| Valid started receipt in the active process, from an earlier process, or unreadable JSON | Running for the first case, interrupted for the other two. | No action is repeated while the identifier remains in the store. |
| Missing receipt before expiry, or at or after expiry | `DecisionRun` before expiry and `DecisionExpired` afterward. | An evicted receipt changes the answer. An expired `SetState` also exposes the current draft's exception. |
| `Finish` commits and returns an error, or fails before committing | The store is failed until close. Reopen replays the first case and returns interrupted in the second. | The Sprint 3 caller must send no speculative terminal result before it reopens. |

### 6. Which round-three findings remain open

Groups A and B remain open for a missing database under an existing identity and for a live acknowledgement above what this stream sent. Group G loses the old clock bug but retains the earlier failure to replay throughout the server's retry window. Group H remains open in the installer and fixture consumers. Group I closes the literal zero-byte and interior-hole cases but the broad nonmutating and gap checks still fail. Group K closes the valid ambiguous finish case. Decision 79 is already marked for the owner.

### 7. Which architect questions the patches touch

All eight questions apply to the patches. The next section records their answers.

## Answers to the architect questions

1. **Code and tools.** Confirmed live that `go version` returns `go1.26.0 linux/amd64`, the root module declares `go 1.26.0`, and no client Go source is tracked. By reading, the fixture calls and their `identity.json` fields are at `deploy/test/checks.sh:219`, `deploy/test/checks.sh:233` and `deploy/test/checks.sh:520`. The existing anchors resolve at `packages/cli/src/bin.ts:1`, `packages/engine/src/harnesses/programs.ts:124`, `packages/engine/src/harnesses/programs.ts:199`, `packages/engine/src/harnesses/recipes.ts:24`, `packages/engine/src/workflows/store.ts:17`, `floorplan/plan.json:1239` and `floorplan/plan.json:1267`. The installed toolchain has `os/exec.Cmd.WaitDelay` and the Darwin sync and `syscall` functions the plan cites.
2. **Writers and readers.** The preceding inventory follows the changed names across the Client, Stack, Contract and Server boundaries. The missing writers or readers are store creation during enrollment, Stack's marker check, the finished-result encoding and safe retirement condition, and the orchestrator's module edit.
3. **Placement.** The state folder, scan, credentials and both records belong under `src/client/`. The server owns application of reports and generation of attempts. The Contract team owns the wire definitions. The Stack team owns installation and fixture changes. The epic assigns root module changes after Sprint 0 to the orchestrator. See `.plans/epics/go-rewrite/epic.md:120`.
4. **Rules.** R1 is kept by the client package layout. R2 applies when the wire behavior is frozen for the Sprint 3 consumers and still needs alignment. R3, R4 and R5 do not apply to this local database and its lack of server calls. R6 is broken by findings 01 and 03. R7 applies to the server's durable state. The client-local running set has a durable `started` receipt and correctly treats a prior process as interrupted. R8 does not apply to local errors. R9 is kept by the explicit local-key and bounded version output rules under the owner-approved single-user setup. R10 leaves the real client-server and PostgreSQL tests to Sprint 3, and uses real local files now. R11 is kept by using the existing engine only as reference material. See `docs/specs/2026-11-25-go-rewrite-foundation-design.md:358`.
5. **Neighboring plans.** Epic agreement points 1 and 4 are affected. The latest `events.proto` draft already stops renumbering, while the older Client Link tasks still expect it. The draft command replay window and expiry exception, the Server Machines retry loop, the changed Go store interface, and the Stack folder and marker names require joint updates. Client Link already owns the numbered `Facts` during an outage. The planned in-process harness for linked tests remains in Sprint 3 under epic agreement point 5 and issue 1634. See `.plans/epics/go-rewrite/epic.md:172`, `.plans/plans/go-s3-client-link/plan.md:66` and `.plans/plans/go-s3-client-link/plan.md:201`.
6. **Merge order.** Items 1, 2, 4 and 5 have independent target paths. Items 3 and 6 can use the already merged item 2. The merged item 6 owns both the outbox and the receipts in the same package and removes the old intermediate dependency. The separate dependency-only commit fails the module check in finding 09.
7. **Wave 2.** No new harness execution, memories, vault calls or repository clones are built. The repository path and byte payload boundaries leave room for those later. Same-identity rollback stays outside wave 1. Initialization and validation of the current store are needed for wave 1 itself.
8. **Dependencies.** By reading the published tags, bbolt v1.5.0 requires Go 1.25.0 and lists `golang.org/x/sys` v0.45.0. The v1.4.3 fallback requires Go 1.23 and lists `golang.org/x/sys` v0.29.0. The Go 1.26 floor satisfies both declared toolchain minima. The local fetch of v1.5.0 was blocked by sandbox networking, so this review did not rerun the bbolt build matrix. See `.plans/plans/go-s1-client-local/decisions.md:64` and `go.mod:3`. [bbolt v1.5.0 module file](https://github.com/etcd-io/bbolt/blob/v1.5.0/go.mod), [bbolt v1.4.3 module file](https://github.com/etcd-io/bbolt/blob/v1.4.3/go.mod).

## Post-build checks

| Check | Scenario and expected result | Evidence and responsible area |
|---|---|---|
| architect-r4-P1 | Cut power while an append or finish is committing on each supported file system. The next run sees a whole transaction or the prior state. Every event that returned a number remains applied or pending, and every command effect remains unique. | Before-and-after database read and independent server-prefix and effect logs. Client Local item 6 and Sprint 3 verification. |
| architect-r4-P2 | Simulate a disk error during commit, then restore disk access. The waiting writer gets the failed-store result, the service reopens once disk access returns, and its final state follows the committed data. | Failed-sync trace, exact exit status, green reopen and event or receipt comparison. Client Local item 6 and Client Link runtime. |
| architect-r4-P3 | On Linux and macOS, run the actual installer twice with an existing enrollment and a different environment between the shell and the service. Both starts resolve the same directory, and the reinstall uses the same identity without asking for another token. | Installer and service logs with fixture-only paths and a stable machine identifier. Stack with Client Link. |
| architect-r4-P4 | Scan the four supported harnesses from actual user-service environments, including ordinary linked installations. Each reports its intended name and version, and no raw child output appears in diagnostics. | Machine inventory and log inspection. Client Local item 5 and Sprint 3 verification. |

## Grade

4 of 10. Three blockers, six majors and zero minors.
