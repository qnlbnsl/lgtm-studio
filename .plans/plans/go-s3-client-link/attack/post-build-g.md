# SHIP

The code of the three repairs is correct as far as I could test it, and the one weakness is in the tests: nothing fixes the size of the new bound on events handed over without an acknowledgement.

**Top finding.** Post-g 1: the window of 32 unacknowledged events can become 33, 48 or 63 and every test still passes.

**Cross-vendor note.** The builder and the test writer were workers of OpenAI. I am a Claude reviewer, so the rule that another vendor reviews is met.

Task `postbuild-go-s3-client-link-g`, nonce `6447d7`. I reviewed `0817d39f..f5ba7347` on `sprint-0` on 2026-11-27, restricted to `src/client`: `session.go`, `main.go` and the four `review_f_*` test files. I changed no repository file except this report. I did not read the other commits in that range, which belong to other teams.

## Findings

### post-g 1 | MINOR | The size of the window of unacknowledged events is held by no test

The repair stops handing over events once `handedThrough - ackedThrough` reaches `maxUnackedEvents`, which is `maxPendingFrames / 2`, so 32. The comment says it leaves room in the 64-frame receive queue for heartbeats and commands. [Constant](../../../../src/client/internal/link/session.go#L47) · [Gate](../../../../src/client/internal/link/session.go#L539) · [Mark update](../../../../src/client/internal/link/session.go#L564)

I changed the constant in a scratch copy and ran the new test, `TestReviewFImmediateAcksSurviveOwnerStall`, and then the whole link package.

| Scratch mutation | Result |
|---|---|
| `maxUnackedEvents = 1 << 30` | The new test failed with `receive_queue_full`. |
| `maxUnackedEvents = maxPendingFrames * 3 / 4` (48) | `TestReviewF*` passed. |
| `maxUnackedEvents = maxPendingFrames - 1` (63) | `TestReviewF*` and the whole link package passed. |
| Gate `<=` instead of `<` (33) | `TestReviewF*` passed. |
| The acknowledgement never advances the mark | The new test failed, because only 32 events were acknowledged. |

The test catches an unbounded window and a window that never reopens. It does not catch a window that is too large to leave room. With 63 outstanding events, one heartbeat acknowledgement fills the queue to 64, and one command from the server overflows it. That is the case the constant's own comment names.

Failure scenario: a later edit raises the constant to the queue size minus one, the suite passes, and a long replay during a command from the server ends the stream with `receive_queue_full`. The client loses no data and reconnects, but it pays a backoff, which is the fault of post-f 3 again.

Fix: add a test with a silent peer that counts the events handed over before any acknowledgement and requires exactly `maxUnackedEvents`. Add a second step in which a command and a heartbeat acknowledgement arrive while the window is full, and require that the stream stays open.

### post-g 2 | MINOR | The changed branch of the token reader is held by no test

The repair for issue 1669 changed two places. The end-of-input case trims a final carriage return, and the full-buffer case now accepts a full-length token followed by a carriage return and then end of input. [Reader](../../../../src/client/cmd/lgtm-client/main.go#L82) · [Full-length case](../../../../src/client/cmd/lgtm-client/main.go#L91)

I removed `err == io.EOF ||` from line 91 in a scratch copy. Every test in `src/client/cmd/lgtm-client` still passed. The new `TestReadJoinTokenTrimsCarriageReturnAtEndOfInput` uses a 36-byte fixture and so never reaches the full-buffer case. `TestRunJoinAcceptsFullLengthToken` covers a line feed, a carriage return plus line feed, and end of input with no line ending, but not a carriage return at the very end. Removing the other change (the trim on line 83) fails the new test, as it should.

The code is right: I traced a token of 4,096 bytes with each of the four endings, and a token of 4,097 bytes, through the 4,097-byte buffer, and each gives the expected result.

Failure scenario: someone simplifies the full-buffer case back to `next == '\n'`. An owner who pipes a full-length token from a file saved with a carriage return and no line feed gets the usage line, and no test says why.

Fix: add a fourth case to `TestRunJoinAcceptsFullLengthToken`, with the ending `"\r"` and the pipe closed, or a unit case of `readJoinToken` with a 4,096-byte token and a final carriage return.

### post-g 3 | MINOR | The plan does not describe the receive queue, the window or the new stop

Item 7b says the session should “close on invalid known frames or 30 seconds of silence”. The code also stops on a 64-frame queue overflow with `receive_queue_full`, and since this repair it holds back events at 32 unacknowledged. The plan has no word for either, and its risk table and its test list for item 7b do not mention them. The builder result explains them, but a later reader of the plan will not find them. [Plan text](../plan.md#L212) · [Builder result](../../../../.foundry/go-rewrite/results/s3-link-build-k.md) · statement: `plan`, D16, C9.

Failure scenario: a later team that changes the server to acknowledge in batches reads the plan, sees no rule about the window, and stalls every client (see post-g 4).

Fix: add one sentence to item 7b, one row to its tests (`TestReviewFImmediateAcksSurviveOwnerStall`), and one row to the workstream 2 risks.

### post-g 4 | MINOR | The window depends on an acknowledgement for every event, which the contract implies but does not state

The client now waits for an acknowledgement before it hands over event 33. The server sends one for every decoded event, and the contract's event table lists an acknowledgement for a repeat, for an applied report and for a consumed report. It does not say that the server must acknowledge each event separately, and `EventAck.through` is defined as “the last event number the server applied”. [Server](../../../../src/server/internal/link/stream.go#L149) · [Contract table](../../../../src/contracts/proto/lgtm/link/v1/events.proto#L21)

A server that coalesced acknowledgements, for instance one for every 50 events, would be conforming to the second sentence, and the client would then hand over 32 events and wait. Heartbeats keep the stream alive, so the 30-second silence rule never fires, and the events stay unsent until the next reconnect. I did not build such a server. The scenario follows from reading `canSendEvent` and the dispatch loop. The deployed server does not do this, so there is no fault today. Statement: D16, C9.

Fix: state in `events.proto` that the server sends one `EventAck` for each event of the current stream, or a rule that its acknowledgements never lag by more than 32 events. Alternatively let the client end the stream when the window stays full for longer than the heartbeat interval.

### post-g 5 | MINOR | The test of the acknowledgement ceiling finds its wait point by parsing goroutine stacks

`waitForReviewFReplayRead` calls `runtime.Stack(buf, true)` with a 64 KiB buffer every millisecond and looks for the text `(*replay).next(` and `(*Store).Read(` in one goroutine's stack. [Helper](../../../../src/client/internal/link/review_f_ack_test.go#L160)

Two things can break it. The buffer truncates the output when the process has many goroutines, so the wanted stack can be cut off and the test then waits until its five-second context ends. A rename of either function silently changes what it waits for. I could not break it: `TestReviewFAckCeilingUsesReceipt` passed 240 runs under `-race` while six copies ran at once. The risk is latent, and it is the reason this test is listed.

Fix: let the store expose a hook that fires when a read starts, as the plan's test list does for other fixtures, or at least fail with a clear message when the output is truncated.

## Order of the assigned checks

**1. Places no reviewer read before the build.** The register's five places are the stop codes (item 7b), the app status (item 8), the workstream 2 risk wording, the busy scan expiry (item 6) and the combined post-build checks. The repairs of this round touch only the receive loop of item 7b and the token reader of item 9. For the combined checks, the plan's scenario for immediate acknowledgements is `TestSessionAcceptsImmediateAck` with `-race -count=200`, which I ran and which passed in 19.4 seconds. I did not run the installed-service scenario, C10 or the macOS sleep scenario, because they need a stack or a host that is not mine.

**2. Code against the plan.**

- A handoff is allowed only while fewer than 32 events are outstanding. The mark `ackedThrough` starts at the welcome mark and is raised only after the ceiling check and the successful `l.receive`, and by `max`, so a repeated lower acknowledgement cannot move it back. It cannot exceed `handedThrough`, because the ceiling check rejects a larger value first, so the unsigned subtraction cannot wrap.
- The owner is the only goroutine that writes or reads `ackedThrough`, and `handedThrough` is written by the owner under the timing lock, so `-race` found nothing.
- Priority is kept. A due heartbeat or a command result is dispatched before an event, whatever the window says, and a full window does not block them. `TestSessionPrioritizesReadyFrames` passed.
- Queue arithmetic. At most 32 event acknowledgements can be queued. A behaving server adds at most one heartbeat acknowledgement per heartbeat that the owner sent, so 64 is enough, with one slot of reserve for each pending command.
- `readJoinToken` handles all four endings at the full length, and an oversized token is refused. The slice `line` is copied to a string before the next read, so the later `ReadByte` cannot overwrite it. The token reaches no log and no error text. The new code adds no output.
- The plan does not describe the window (post-g 3).

**3. Mutations.** See the tables in post-g 1 and post-g 2. The mutation that the builder used, reversing the whole patch, fails the new test. The mutations that keep the idea but change the number do not.

**4. Defects of the code.** I found no race, leak, swallowed error, secret in a log, or open file. The only gap in the repair is the stall in post-g 4, which depends on a server that does not exist today.

**5. Design.** Findings are tied to D16 and C9 in post-g 3 and post-g 4, and to `plan` in post-g 3.

**6. Tests that I ran.** All were run with `-race`, as a regular user, on the host.

| Command | Output |
|---|---|
| `go test -race -count=1 ./src/client/...` | All ten packages printed `ok`. The link package took 21.7 seconds. |
| `go test -race -count=200 -run '^TestSessionAcceptsImmediateAck$' ./src/client/internal/link` | `ok` in 19.4 seconds. |
| `go test -race -count=20 -run '^TestReviewF' ./src/client/internal/link` | `ok` in 26.0 seconds. |
| `go test -race -count=20 -run 'TestReadJoinToken\|TestRunJoin\|TestRunCommandExitStatus' ./src/client/cmd/lgtm-client` | `ok` in 24.6 seconds. |
| Six copies at once of `TestReviewFAckCeilingUsesReceipt` and `TestReviewFRestartHalfClosesBeforeCancel`, 40 runs each | All six printed `PASS`. |
| `go vet ./src/client/...` and `gofmt -l src/client` | No output. |

## Grade

**8 out of 10.** The window and the token reader behave as intended, the ceiling and the restart tests of the earlier round are stable, and the new tests fail for the faults that they were written for. The grade is not higher because the size of the window and one changed branch of the token reader can change without a failing test, and the plan does not mention the new rule.
