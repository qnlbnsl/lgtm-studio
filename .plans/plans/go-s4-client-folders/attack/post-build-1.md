# REVISE

The frozen tests accept a replay-window regression that stalls a waiting client when it reconnects after the server has committed 32 events.

OpenAI supplied the test writer, builder and reviewer. I reviewed item 1 on `sprint-0` across `722bdf41..81fa5f28`. The task identifier is `postbuild-go-s4-client-folders-1` and its nonce is `310c13`.

## Findings

### post-1 1 — MAJOR — The replay window is untested at a full-window Welcome mark

**Statement.** The plan measures the 32-event window from the greater of the Welcome mark and the last acknowledgement. Foundation check C11 requires buffered events to arrive in order after an outage. [Plan](../plan.md#L77) · [Foundation check C11](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L388)

**Evidence.** The code initializes both counters from Welcome. The frozen tests fill the window from mark zero and reconnect at mark two with one retained event remaining. I initialized only `ackedThrough` to zero in scratch. The existing race tests in all ten in-scope client packages exited 0. [Window initialization](../../../../src/client/internal/link/session.go#L369) · [Capacity case](../../../../src/client/internal/link/revision_two_session_test.go#L145) · [Reconnect case](../../../../src/client/internal/link/revision_two_session_test.go#L203)

**Failure scenario.** I seeded 65 retained events and received 1 through 32 on a waiting stream. I dropped the acknowledgement and reconnected with Welcome mark 32. The reviewed code sent events 33 through 64. I received event 65 after sending acknowledgement 33. The mutation sent a heartbeat and stalled before event 33. A server waiting for the retained prefix would keep approval gated. [Window check](../../../../src/client/internal/link/session.go#L541) · [Run readiness](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L140)

**Fix.** Extend the waiting reconnect test to use a Welcome mark of 32 and at least 33 retained events above it. Require exactly 32 resumed event handoffs before a new acknowledgement, then require the next event after one slot opens.

### post-1 2 — MINOR — The waiting tests omit a queued event at the run boundary

**Statement.** The plan lets a waiting client send retained events strictly below its run boundary and holds fresh events until approval. [Plan](../plan.md#L77) · [Waiting stream contract](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L186)

**Evidence.** The reviewed comparison uses `<`. The frozen waiting cases leave the boundary event absent until approval. Replacing `<` with `<=` in scratch still passed the existing race tests in all ten in-scope client packages. [Comparison](../../../../src/client/internal/link/session.go#L541) · [Waiting cases](../../../../src/client/internal/link/revision_two_session_test.go#L145) · [First fresh reports](../../../../src/client/internal/link/revision_two_session_test.go#L315)

**Failure scenario.** A synthetic sender probe retained event 1, captured boundary 2 and then queued event 2. After the waiting peer acknowledged event 1, the reviewed code withheld event 2. The mutation sent event 2 before approval.

**Fix.** Add a sender test with an event queued exactly at the announced boundary. Assert that it stays queued while the peer is waiting and is eligible once the peer approves.

## Post-build scenarios and source review

The plan and the newest findings register contain no `UNREVIEWED FIX` entry for item 1. The fold records zero findings assigned to post-build checks. The paired cutover still applies. The reviewed server declares revision 1. The focused paired command reported `testing: warning: no tests to run` for `TestSprintFourWaitingReplay` and `TestSprintFourPendingRun`. Those cases and the PostgreSQL cutover remain integration gates under foundation rule R10. [Post-build checks](../plan.md#post-build-checks) · [Review register](round-1-findings.md) · [Fold result](../../../../.foundry/go-rewrite/results/fold-go-s4-client-folders-r1.md#L9) · [Server revision](../../../../src/server/internal/link/handshake.go#L24) · [Foundation rule R10](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L371)

By reading the code, `Run` captures the durable next number and a version 7 universally unique identifier (UUID) generated with cryptographic randomness before reporting or dialing. Each attempt sends that same pair with a new nonce and retains both signed proof labels. The sender keeps one handoff path and the receiver records the acknowledgement ceiling under the same lock as the handoff update. Retry handling preserves the revision refusal after a verified Challenge and keeps the event-gap and invalid-argument retries. I found no additional discrepancy in the item 1 code or new race, leak, swallowed error, credential disclosure or permission change. [Run startup](../../../../src/client/internal/link/session.go#L84) · [Identifier](../../../../src/client/internal/link/run_identity.go#L16) · [Hello and proof](../../../../src/client/internal/link/conn.go#L105) · [Refusal](../../../../src/client/internal/link/conn.go#L207) · [Retry](../../../../src/client/internal/link/retry.go#L22) · [Acknowledgement ceiling](../../../../src/client/internal/link/session.go#L464) · [Handoff](../../../../src/client/internal/link/session.go#L648)

## Linux verification

I used Go 1.26.0 on Linux in a disposable copy of the pinned commit. The container kept the state-directory checks active and provided loopback networking. The frozen-file checksums matched. A native macOS host was unavailable.

```sh
go test -v -race ./src/client/internal/link -run 'TestRevisionTwoRunAnnouncement|TestRevisionTwoWaitingPrefix|TestReviewGUnacknowledgedEventWindow|TestOpenProofFixture' -count=1
```

All four named groups printed `PASS` with zero skips. The package printed `ok github.com/qnlbnsl/lgtm-studio/src/client/internal/link 4.478s` and exited 0. The wider client race run excluded the item 4 Folders package as directed. It exited 0 in all ten packages and Link printed `ok github.com/qnlbnsl/lgtm-studio/src/client/internal/link 82.021s`. The subprocess entry helper skipped as expected. The file ownership test skipped for the unprivileged run and printed `PASS` in a separate root run.

Each planned mutation ran independently and exited 1 at a frozen assertion.

| Mutation | Frozen assertion output in `revision_two_session_test.go` |
|---|---|
| Generate the pair in `attempt`. | Line 55 printed `Hello announced first event 7, want durable next 4`. |
| Capture the retained floor. | Line 33 printed `Hello announced first event 1, want durable next 4`. |
| Change the server proof label. | Line 33 printed `the client stream closed before sending the expected frame`. |
| Change the machine proof label. | Line 33 printed `the machine did not send a valid proof`. |
| Suppress waiting replay. | Line 162 printed `the waiting stream did not replay the next retained event`. |
| Set the window to 31. | Line 162 printed `the waiting stream did not replay the next retained event`. |
| Set the window to 33. | Line 164 printed `the waiting stream handed off 33 events, want 32`. |

**Grade.** 8 out of 10.
