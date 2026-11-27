# REVISE

A temporary read failure on an intact `client.db` still makes the client say to restore history or enroll again, and the command maps that outcome to exit status 0.

The test writers and builders were OpenAI Codex workers, the same vendor as this reviewer. The review covers item 8 and the repair for post-3c 1 on `sprint-0` at `c0716557`. The review diff is `a292c8df~2..c0716557` for `src/client`.

## Checks before the findings

- By reading, the plan and the round 1 register contain no places marked `UNREVIEWED FIX` for this assignment. The register records the startup, refresh and shared filter decisions under logic-r1 07 and threat-r1 02. [Plan item 8](../plan.md#L223) · [Round 1 register](round-1-findings.md#L7)
- Confirmed live, the executable met the local part of item 8's “Done when” row. The signed-peer fixture observed a masked startup path, omitted workspace, masked inventory after sign-in replacement on the same stream, and matching numbered reports on disk and on the wire. The result handoff fixture refused an identifier learned after the receipt was written. Early state failures in both `join` and `run` emitted one fixed diagnostic and omitted the planted path text. [Done when](../plan.md#L202) · [Executable fixture](../../../../src/client/internal/app/outgoing_test.go#L146) · [Early diagnostics](../../../../src/client/internal/app/outgoing_test.go#L231) · [Result handoff](../../../../src/client/internal/app/outgoing_test.go#L626)
- By reading, the application uses one filter for the logger and Link. The enrollment loader reads the signing key from a private file descriptor, verifies its public half against the saved enrollment and registers the key forms. The sign-in reader bounds the file read and closes the descriptor on every path after open. [Application startup](../../../../src/client/internal/app/client.go#L80) · [Enrollment loader](../../../../src/client/internal/enroll/credential.go#L84) · [Sign-in reader](../../../../src/client/internal/app/outgoing.go#L22)
- Confirmed live, the repaired permission cases behave as specified. Link made zero transport initializations, the unreadable-store executable exited 1 with the plain diagnostic, and both permission cases preserved the bytes. Native macOS and the paired capture remain release-wave checks. Issues 1696 and 1697 were already recorded. [Link permission fixture](../../../../src/client/internal/link/revision_two_store_open_review_test.go#L32) · [Executable permission fixture](../../../../src/client/internal/app/revision_two_process_test.go#L150) · [Release checks](../plan.md#L265) · [Known issues](../../../../docs/issues.md#L8087)

## Findings

### post-8 1

**MAJOR. Design statement `plan`, item 3 and the post-3c 1 repair.**

The database opener still treats an ambiguous metadata read error as proof that saved history is damaged.

Confirmed live, a Linux subprocess injected an input/output error (`EIO`) into `pread64` while opening a valid fixture database. A control read confirmed that the read syscall received `EIO`. `OpenStore` returned `ErrStoreDamaged`. Link returned `RunRecoveryStop` with a nil error and logged the restore instruction. The same database opened after the fault cleared, with identical bytes. The application maps that Link outcome to exit status 0. [Open classifier](../../../../src/client/internal/link/store.go#L371) · [Recovery diagnostic](../../../../src/client/internal/link/session.go#L370) · [Exit mapping](../../../../src/client/internal/app/client.go#L125)

The builder documented that bbolt can lose the underlying read error. In pinned bbolt 1.4.3, `db.go:327–411` returns `ErrInvalid` when neither metadata page could be read. The application assigns that sentinel to damage. The frozen classifier test injects `EIO` after the dependency boundary, so the dependency still collapses this real path while the test passes. [Pinned bbolt version](../../../../go.mod#L14) · [Builder's note](../../../../.foundry/go-rewrite/results/s4-client-3r-build.md#L17) · [Frozen classifier test](../../../../src/client/internal/link/revision_two_store_open_review_test.go#L18)

A temporary disk read failure at startup can leave a machine offline under a restart-on-failure supervisor. The owner also receives advice to replace valid state.

Preserve metadata read errors through the database opener, or perform an independent read that establishes damage before assigning the recovery outcome. Route ambiguous failures through the plain store error. Add a regression that injects the fault during metadata reads and checks the diagnostic, exit status and preserved file.

### post-8 2

**MINOR. Design statement `plan`, item 8 report admission, and foundation rule R9.**

The frozen client suite accepts an implementation that screens a new report with cached credentials and skips the file refresh at durable append.

Confirmed live, changing only `appendFilteredReport` to take the filter's read lock without refreshing left all 11 client packages green. The executable fixture replaces the file before it sends its scan command. Command admission already refreshes the shared filter, so by the time the report is appended that test has already registered the replacement. [Append gate](../../../../src/client/internal/link/outbox.go#L24) · [Fixture order](../../../../src/client/internal/app/outgoing_test.go#L120) · [Command admission](../../../../src/client/internal/link/commands.go#L151) · [Foundation rule R9](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L370)

A replacement after command admission but before report append would let that mutant write the new secret to the buffer. A later wire check could stop the send, and the secret would already be on disk. The scratch probe replaced the file immediately before append and inspected the stored payload. It passed on the reviewed code and found the replacement value on disk under the mutation.

Move the fixture replacement to the interval after command admission and before the report append. Check the durable payload for masking independently of the next wire handoff.

### post-8 3

**MINOR. Design statement `plan`, item 8 replay refresh and item 3 validation before acknowledgement.**

The frozen client suite also accepts an implementation that refreshes at event handoff but skips the file refresh before applying the Welcome acknowledgement.

Confirmed live, changing only `newReplayWithFilter` to take the read lock without refreshing left all 11 client packages green. The frozen replacement fixture offers no positive acknowledgement. Its single retained event can still be stopped by the later handoff gate, with the database bytes unchanged. [Replay preflight](../../../../src/client/internal/link/replay.go#L33) · [Handoff gate](../../../../src/client/internal/link/session.go#L617) · [Fixture Welcome](../../../../src/client/internal/app/outgoing_test.go#L421) · [Frozen byte check](../../../../src/client/internal/app/outgoing_test.go#L216)

If Welcome acknowledges event 1 and required event 2 holds the replacement secret, the mutant removes event 1 before detecting the unsafe suffix. The scratch probe observed both rows preserved in the reviewed code. With the mutation, preflight returned nil and only one row remained.

Give the replacement replay fixture an acknowledged safe row before the unsafe required row. Assert that both rows survive the stop.

## Linux acceptance and mutations

The direct item 8 command stopped several fixtures in setup with `state: a parent folder can be changed by others: /`. The sandbox presents the root and temporary parent directories as owned by user identifier 65534. In one scratch copy of the reviewed commit, I treated that identifier as root for those two parent directories only, with the mode checks intact. The runs used Go 1.26.0 on Linux as the ordinary user. All outcome and mutation checks below used that scratch copy.

```sh
go test -v -race ./src/client/internal/app ./src/client/internal/link -run 'TestOutgoingFilterExecutable|TestOutgoingFilterStructuredLog|TestAppLogRedactsSecrets|TestOpenFrameDebugDoesNotLogConnectionToken' -count=1
go test -v -race ./src/client/internal/link ./src/client/internal/app -run 'TestRevisionTwoTerminalHistory|TestRevisionTwoExecutableStop|TestRetryFailureExitsAndKeepsStore|TestAppRunStoreStatus' -count=1
go test -v -race -count=1 ./src/client/internal/link -run '^TestRevisionTwoStoreOpen(ErrorClassification|PermissionFailure)$'
go test -v -race -count=1 ./src/client/...
```

| Run | Output |
|---|---|
| Item 8 | Exit 0. The four named tests printed `PASS`, with zero skips. App printed `ok` at 6.119s and Link printed `ok` at 2.118s. |
| Item 3 | Exit 0. The four named tests printed `PASS`, including the unreadable-store and requested-restart executable cases. Link printed `ok` at 1.900s and App printed `ok` at 2.566s. |
| Store-open repair | Exit 0. The classifier and both permission cases printed `PASS`. Link printed `ok` at 1.047s. |
| Full client | Exit 0. All 11 packages printed `ok`. Four inherited entries skipped, comprising a subprocess entry, two resolved-path length cases and the superuser-only ownership case. |

Reverting the item 3 repair made the classifier, both Link permission cases and the unreadable-store executable case fail. The executable printed the recovery instruction and exited 0 where the test requires 1. [Executable assertion](../../../../src/client/internal/app/revision_two_process_test.go#L168)

Each of the five planned item 8 mutations independently exited 1 on a frozen assertion. The two narrower mutations are documented in post-8 2 and post-8 3. [Planned mutations](../plan.md#L230)

| Mutation | Observed assertion |
|---|---|
| Delay sign-in loading past startup. | The executable output contained a planted value and the startup path failed its masking assertion. [Test](../../../../src/client/internal/app/outgoing_test.go#L146) |
| Screen only the message. | The structured log exposed planted secret material. [Test](../../../../src/client/internal/app/outgoing_test.go#L314) |
| Bypass screening of attached attributes. | Attached keys, groups, values or lazy attributes escaped masking after replacement. [Test](../../../../src/client/internal/app/outgoing_test.go#L335) |
| Load the sign-in once and omit refresh. | Inventory and durable reports exposed the replacement, and the unsafe replay case failed to stop before handing off a report. [Test](../../../../src/client/internal/app/outgoing_test.go#L163) |
| Give the logger and Link separate filters. | The structured log did not mask the peer token registered through Link. [Test](../../../../src/client/internal/app/outgoing_test.go#L294) |

## Grade

8 out of 10. The local executable and the planned filter mutations establish that the shared filter works through the exercised output paths. A read fault on valid history still needs a plain failure outcome, and the two independent refresh points need their own observations.
