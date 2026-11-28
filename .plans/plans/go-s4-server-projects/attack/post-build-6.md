# SHIP

The removal commits its refresh with the database changes, but the frozen tests also pass when the refresh commits separately and survives a failed removal.

OpenAI supplied the test writer, the builder and this reviewer. I wrote neither the built code nor the frozen tests. I reviewed Projects item 6 on `sprint-0` at `8aedeaaf`, using the range `018ba0b6~1..8aedeaaf`. The task identifier is `postbuild-go-s4-server-projects-6` and the nonce is `41b211`.

## Findings

### post-6 1 · MINOR · The frozen tests accept a separately committed refresh

**Statement.** The plan and foundation rule 4 require the repository cleanup and its refresh to commit in the same Store write. [Plan](../plan.md#L202) · [Foundation rule 4](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L365)

**Evidence.** By reading, the built branch records the marker on the removal transaction. The budget cases observe the feed after successful removal, the rollback case faults orphan deletion before the refresh is recorded, and the watch fixture writes a modeled refresh. Confirmed live, replacing the large-removal branch's `tx.Record` with a separate `s.store.Write` passed all 12 frozen attachment, removal and watch groups in 45.676 seconds. A separate probe then faulted the deletion of the Link connection after the Projects callback. The mutated removal returned unavailable and left the domain rows intact, but its refresh remained in the feed. The pinned implementation passed the same probe. [Built refresh write](../../../../src/server/internal/projects/removal.go#L43) · [Budget assertions](../../../../src/server/internal/projects/removal_test.go#L107) · [Early rollback fault](../../../../src/server/internal/projects/removal_test.go#L142) · [Modeled watch fixture](../../../../src/server/internal/projects/watch_changes_test.go#L115) · [Later Link deletion](../../../../src/server/internal/link/presence.go#L320)

**Failure scenario.** Link cleanup fails after Projects records the large-removal marker. With independent publication, a live watch consumes the marker and closes even though the machine and its folders survive. [Refresh consumption](../../../../src/server/internal/projects/repositories.go#L47)

**Fix.** In the combined startup case, also fault a write after the Projects callback records the marker. Require the domain rows and feed position to remain intact. Deliver an independent save on the original watch, then retry the removal and require exactly one committed refresh and a complete replacement snapshot. [Combined case](../plan.md#L252)

### post-6 2 · MINOR · The no-command case covers an open Add but omits an open Inspect

**Statement.** Item 6 requires the removal callback to fail every open Projects operation on the removed machine, even when Link has no pending command. [Plan](../plan.md#L201)

**Evidence.** By reading, the callback updates both operation kinds. The no-command test creates the Inspect before removal, completes it, and leaves only an Add open on the removed machine. Confirmed live, adding `AND kind = 'add'` to the callback passed all 12 frozen attachment, removal and watch groups in 54.253 seconds. A separate probe created an open Inspect and deleted its Link command before removal. The pinned code returned `not_found` with “The machine was removed.” The mutant left the Inspect open with an empty failure. [Callback predicate](../../../../src/server/internal/projects/removal.go#L13) · [Frozen setup](../../../../src/server/internal/projects/removal_test.go#L47) · [Operation-kind helper](../../../../src/server/internal/projects/reports_test.go#L170)

**Failure scenario.** An open Inspect without a pending Link command remains open after removal. A keyed retry waits for the operation until the folder wait limit instead of returning the fixed refusal. [Keyed replay](../../../../src/server/internal/projects/operations.go#L98) · [Wait loop](../../../../src/server/internal/projects/operations.go#L185)

**Fix.** Keep both an Inspect and an Add open after deleting their Link commands. Require both rows to fail with `machine_removed` and both keyed retries to return the fixed removal sentence. Retain the existing terminal replay checks. [Current replay assertions](../../../../src/server/internal/projects/removal_test.go#L85)

## Grade

9/10.

## The large removal kept cleanup and publication together

The round-one register leaves logic-r1 02 and its three duplicates open for post-build verification. Item 6 owns the lock, folder deletion, orphan decision and marker write. By reading, it acquires the affected repository locks in identifier order, deletes the machine's folders, and checks the locked repositories for survivors in the next statement of the read-committed write. Through 254 affected repositories, it records one reference per repository. Above 254, it records one saved reference for topic `repositories`, empty scope and key `refresh`. Ordinary repository identifiers come from the version 7 universally unique identifier generator. [Round-one register](round-1-findings.md#L10) · [Plan](../plan.md#L199) · [Repository locking](../../../../src/server/internal/projects/removal.go#L20) · [Orphan decision](../../../../src/server/internal/projects/removal.go#L64) · [Store transaction](../../../../src/server/internal/store/feed.go#L114) · [Identifier generator](../../../../src/server/internal/store/ids.go#L11)

Confirmed live, the 254-repository control committed 254 repository references and one machine reference. The 255 and 256 controls each committed one refresh and one machine reference. The fixtures included an affected repository with two folders on the departing machine and one surviving folder, so the threshold measured distinct affected repositories. [Budget fixture](../../../../src/server/internal/projects/removal_test.go#L107) · [Feed assertions](../../../../src/server/internal/projects/removal_helpers_test.go#L99)

Confirmed live, a scratch probe combined the real Machines removal, Link and Projects callbacks, PostgreSQL, a signed Link stream and the real Projects watch handler. It started with 257 affected repositories and 258 folders, including a shared repository. A signed Add supplied the earlier stored reply, and a second folder command was staged above the durable Link mark. Three separate attempts each enabled one fault, at orphan deletion, insertion of the refresh into the feed, or Link connection deletion. Each preserved the machine, connection, staged command, operation, receipt, membership and feed at position 20. The same watch then delivered an independent save at 24. [Removal callback wiring in the fixture](../../../../src/server/internal/projects/removal_helpers_test.go#L22) · [Signed Link fixture](../../../../src/server/internal/projects/reports_test.go#L91) · [Watch fixture](../../../../src/server/internal/projects/watch_test.go#L95)

Confirmed live, retrying the same removal key committed one machine removal followed by one repository refresh at position 26. The command settled as `removed` and lost its staged outcome. The open Inspect stored `machine_removed`, and retrying the earlier Add returned its original stored reply bytes. The old watch produced no refresh data frame and ended with `aborted` and “The watch fell behind and must be reopened.” A new watch opened at position 26 with the shared survivor and the unrelated control, each with its one surviving folder. A second replay of the removal key emitted no reference. [Fixed removal refusal](../../../../src/server/internal/projects/errors.go#L91) · [Fixed watch refusal assertion](../../../../src/server/internal/projects/watch_changes_test.go#L103)

By reading the pinned startup and paired test list, the production startup registration, the real client process, the recovery-warning case and the browser tree remained outside this fixture. The startup registration and combined case belong to item 8. The pinned revision has no Projects startup test in its committed app tree. Issues 1704 through 1706 were already recorded. [Pinned startup](../../../../src/server/internal/app/start.go#L266) · [Combined case](../plan.md#L252) · [Post-build scenarios](../plan.md#L279) · [Known issues](../../../../docs/issues.md#L8151)

## Ordinary removal and both lock orders

By reading, Machines deletes its registry row and invokes Link in one Store write. Link settles pending commands and dispatches the Projects removal callback before it deletes connection and Link state. Projects updates only its own open operations and repository tables. Store flushes the references in the same write, and the Machines handler maps storage failures to fixed Connect messages. [Machines write](../../../../src/server/internal/machines/records.go#L59) · [Link removal](../../../../src/server/internal/link/presence.go#L308) · [Projects update](../../../../src/server/internal/projects/removal.go#L13) · [Feed flush](../../../../src/server/internal/store/feed.go#L120) · [Error mapping](../../../../src/server/internal/machines/records.go#L182)

Confirmed live, disconnect preserved the open operation and all folders. Removal then returned the fixed refusal, kept prior terminal Add and Inspect replies and the other machine's operation, saved the shared survivor and removed the orphan. When the second final remover waited for the first, the two commits produced one saved reference followed by one orphan removal and left no repository or folder. A probe that held the higher identifier observed the lower identifier already locked. [Ordinary removal](../../../../src/server/internal/projects/removal_test.go#L37) · [Concurrent final removals](../../../../src/server/internal/projects/removal_test.go#L201) · [Ascending-lock probe](../../../../src/server/internal/projects/removal_test.go#L226)

Confirmed live, attachment first saved the new folder and kept its full earlier reply after removal. Removal first left zero folders, advanced the reporting machine's Link mark and returned `not_found` with “The repository was not found.” It emitted no attachment save. A report from the removed machine was separately fenced. [Attachment and removal orders](../../../../src/server/internal/projects/removal_test.go#L269) · [Losing report](../../../../src/server/internal/projects/removal_test.go#L171)

## Frozen tests and scratch mutations

By reading the commits, `018ba0b6` added the fixtures and a stub, and `8aedeaaf` replaced the stub without changing either frozen test file. The middle commit is the item 7 report and issue update. Confirmed live, the plan command passed all six named groups with zero skips. The same selection passed with Go's race detector. These focused runs did not select the deliberately red router and Compose checks, the long Link package or the listed intermittent client and paired tests. [Plan command](../plan.md#L215) · [Shared test limits](../../../../.foundry/go-rewrite/common-build.md#L41)

```sh
bash deploy/test/with-postgres.sh -- go test -v ./src/server/internal/projects -run '^TestProject(Attachment|MachineRemoval|ConcurrentOrphanRemoval|AttachAgainstRemoval)' -count=1
bash deploy/test/with-postgres.sh -- go test -race -v ./src/server/internal/projects -run '^TestProject(Attachment|MachineRemoval|ConcurrentOrphanRemoval|AttachAgainstRemoval)' -count=1
```

```text
--- PASS: TestProjectMachineRemoval (10.14s)
--- PASS: TestProjectConcurrentOrphanRemoval (1.14s)
--- PASS: TestProjectAttachAgainstRemoval (1.73s)
--- PASS: TestProjectAttachmentAliases (2.45s)
--- PASS: TestProjectAttachmentIdentity (8.57s)
--- PASS: TestProjectAttachmentRollback (0.86s)
PASS
ok  github.com/qnlbnsl/lgtm-studio/src/server/internal/projects  24.906s
ok  github.com/qnlbnsl/lgtm-studio/src/server/internal/projects  49.993s  (race run)
```

Confirmed live, each mutation ran as a Go overlay in the same disposable archive. Each result below was an assertion failure. The two supplemental probes passed against the pinned code in 4.605 seconds, and each rejected its corresponding surviving mutation above.

| Scratch change | Confirmed result |
|---|---|
| Skip the explicit target row lock when attaching a new path, while keeping it for saved paths. | Exit 1. Both halves of `TestProjectAttachAgainstRemoval` failed in 2.150 seconds. Attachment first failed because the remover returned before waiting. Removal first reached the competing write and then failed because the report did not commit an attachment decision or advance the mark. This supplies the coverage missing in post-5 1. [Prior finding](post-build-5.md#post-5-1--minor--the-target-lock-test-covers-a-refusal-and-leaves-new-attachments-unguarded-by-the-suite) · [Attachment query](../../../../src/server/internal/projects/reports.go#L172) · [Wait assertion](../../../../src/server/internal/projects/removal_helpers_test.go#L167) · [Report assertion](../../../../src/server/internal/projects/removal_test.go#L299) |
| Omit the removal callback's work. | Exit 1. The open Add without a pending command did not store the fixed removal failure. [Assertion](../../../../src/server/internal/projects/removal_test.go#L78) |
| Drop the removal repository row lock. | Exit 1. The second final remover returned before waiting on the first. [Assertion](../../../../src/server/internal/projects/removal_helpers_test.go#L167) |
| Choose orphan candidates before taking the locks. | Exit 1. The two final removals left one repository and zero folders. [Assertion](../../../../src/server/internal/projects/removal_test.go#L221) |
| Take repository locks in descending order. | Exit 1. The remover reached the higher identifier before locking the lower one. [Assertion](../../../../src/server/internal/projects/removal_test.go#L253) |
| Keep publishing individual references at 255 and 256. | Exit 1. At 255 the feed assertion saw an unexpected reference. At 256 the removal returned unavailable. [Assertions](../../../../src/server/internal/projects/removal_test.go#L120) |
