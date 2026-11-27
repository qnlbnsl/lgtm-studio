# REVISE

The code of all five repairs is correct as far as I could test it, but the new acknowledgement ceiling and the restart delivery are held by no deterministic test.

**Cross-vendor note.** The builder was a worker of my own vendor, so the rule that another vendor reviews was not met. I am a Claude reviewer of code that Codex workers built and tested, so for this review the cross-vendor rule is met. This is the last review of this plan's loop. Every finding below is a fault of these repairs unless it says it is older.

Task `postbuild-go-s3-client-link-f`, nonce `3699b7`. I reviewed `520f8ffb..ff36e948` on `sprint-0` on 2026-11-27, which is commits 975d5a64, 1badd2fe, 4f2669e9 and ff36e948. I changed no repository file except this report.

## Findings

### post-f 1 | MAJOR | The acknowledgement ceiling is held by no test

Plan item 7b and its risk table (“A guessed acknowledgement removes unapplied reports”, High) bound a live acknowledgement by the events handed to the sender on this stream. The repair stamps each queued reply with `handedThrough` at receipt and refuses a larger acknowledgement with `ErrAckAhead`. [Ceiling check](../../../../src/client/internal/link/session.go#L546) · [Stamp at receipt](../../../../src/client/internal/link/session.go#L458) · [Update at handoff](../../../../src/client/internal/link/session.go#L645) · [Plan risk](../plan.md#L226)

I read the logic for a wrong event and found none. The stamp is taken and the handoff is written under one lock. The receiver must take that lock after it has read a frame. A legitimate acknowledgement therefore always sees the number of the event that it acknowledges.

I removed the check in a scratch copy (`false && ack.GetThrough() > reply.ackCeiling`). The entire link package still passed, in 10.6 seconds. The old cursor check in `replay.ack` still stops an acknowledgement above the cursor. It does not stop a queued acknowledgement that was ahead of the handoffs when it arrived but is below the cursor when the owner reads it. That case exists only because of the new queue, and nothing exercises it. A wrong implementation that compares with the live cursor, or that stamps the ceiling when the owner reads the reply, passes every test. The path that sets `l.ended = reply.receivedAt` for an acknowledgement that is ahead has no test either.

Failure scenario: a peer sends a heartbeat acknowledgement and then an acknowledgement through 2 while event 1 is the last one handed. The owner hands event 2 and then reads the second frame. The mutated build accepts it, and the store drops event 2 before the server has it.

Fix: add a test that queues an acknowledgement ahead of the ceiling behind another frame, lets the owner hand the next event before it reads that frame, and expects `impossible_ack` and an end time at receipt.

### post-f 2 | MINOR | The restart delivery repair is held only by a probabilistic test

Plan item 7b requires up to two seconds to send the restart result before returning `RunRestart`. Commit 975d5a64 closes the request side, lets the peer finish, and cancels afterwards. [Close request](../../../../src/client/internal/link/session.go#L570) · [Plan](../plan.md#L261)

I replaced that block with an immediate `return true, nil` in a scratch copy. The link package passed. The command package, whose `TestRunCommandExitStatus` checks that the peer receives the result, failed in 2 of 6 runs with the mutation. The unmutated code passed 12 of 12 runs with `-race`. The repair works, but a regression is found by chance. The builder result for `s3-link-tests-j` records the same intermittence.

Failure scenario: a later edit restores the early cancel, the suite passes once, and a restart result is lost. The saved receipt replays it after the relaunch, so the loss costs a delay and not a repeated restart.

Fix: add a deterministic link-level test. The peer keeps its response open, sees the result frame, and sees the end of the request before the stream is canceled.

### post-f 3 | MINOR | A full queue ends a stream whose peer behaves

The repair queues 64 frames, and overflow cancels the stream with the new code `receive_queue_full`. The plan says only “close on invalid known frames or 30 seconds of silence”. The builder result documents the addition. [Queue](../../../../src/client/internal/link/session.go#L420) · [Overflow](../../../../src/client/internal/link/session.go#L460) · [Plan text](../plan.md#L212) · [Builder result](../../../../.foundry/go-rewrite/results/s3-link-build-j.md#L11)

The server answers every event with its own acknowledgement. [Server acknowledgement](../../../../src/server/internal/link/stream.go#L149) The client can hand over events without waiting for them, because the sender gate opens when the transport accepts the write.

Confirmed live in a scratch copy with the in-memory peer and 300 stored events:

- With no owner stall, the peer acknowledged all 300 events back to back, and also at 0.2, 1 and 3 milliseconds apart. The stream stayed open each time.
- With a 400 millisecond local stall (a slow load read before a heartbeat) while the peer acknowledged at 1 millisecond, the stream closed after about 250 acknowledgements and the client logged `link_disconnected` with `receive_queue_full`. I shortened the heartbeat interval in the scratch copy to trigger the stall.

The old design blocked the receiver and relied on transport flow control, so this reconnect is new. It loses no data, because the client resumes at the welcome mark. A slow local operation during a long replay can still cost a reconnect and a backoff. The design rule is D16 and check C9.

Failure scenario: after an outage the client replays several hundred buffered events, and a slow local read stalls the owner for longer than 64 acknowledgements take to arrive.

Fix: bound the unacknowledged events the owner hands over, for example to 32, so that a behaving peer cannot fill a queue of 64. Keep overflow for a peer that sends more than it should.

### post-f 4 | MINOR, older | A trailing carriage return at end of input is kept

At end of input without a line feed, the reader returns the line with a final carriage return untrimmed. The line then fails decoding as a token. It is older than these repairs, and it never reaches a log. [Reader](../../../../src/client/cmd/lgtm-client/main.go#L72)

Fix: trim a final carriage return in the end-of-input case, or document that it is refused.

## Order of the assigned checks

**1. Places no reviewer read before the build.** I ran `TestSessionAcceptsImmediateAck` with `-race -count=200`, and it passed. The reply for the handed-off event was accepted before send returned. I did not run the deployed check C10, the installed-service store-fault scenario, the macOS sleep scenario or the combined suite with a real database, because they need a stack or a host that is not mine. The reconnect scenarios of the plan's post-build checks run in the link fixtures, and they passed.

**2. Code against the plan.**

- A busy session keeps reading and measures silence at receipt. The deadline is updated under the lock before the frame is queued, and the two-frame test passes.
- A rejected frame is timed at its receipt. The end time is taken from the receipt of the frame for protocol and acknowledgement rejections. Removing those lines fails `TestRetryProtocolRejectionUsesFrameReceiptTime`.
- A signal ends a join that waits for its token. The read runs in a goroutine, and the command returns 1 on the signal. The result channel has room for one value, so the goroutine cannot block. Both signals pass `TestRunJoinStopsOnSignalWhileReadingToken`.
- An oversized token is refused at once. The reader has 4,097 bytes of buffer. It refuses byte 4,097 unless that byte is a carriage return followed by a line feed. A token of 4,096 bytes with a line feed, a carriage return plus line feed, or the end of input is accepted.
- The restart result is delivered before the cancel (post-f 2).

**Receive queue.** The reader is a single goroutine, and the channel is first in, first out, so no frame is reordered. A frame is lost only when the stream is already ending (after the deadline, after cancellation, or on overflow). A terminal transport error is queued behind the pending frames with a blocking send and no lock held, and it is abandoned only on cancellation. The owner checks the context before it handles a reply, so no frame is processed after the cancel. The stamp and the handoff share one lock (post-f 1). I found no race, and `-race` was clean.

**Join token.** The token reaches only the enrollment call. The arguments hold `-`, no process title is set, and the usage text is fixed. The join tests check both output streams for the fixture token, including the full-length token on all three line endings. I found no path to a log, an error text, the process title or the argument list.

**3. Mutations in a scratch copy.**

| Mutation | Result |
|---|---|
| Remove the acknowledgement ceiling. | Whole link package passed (post-f 1). |
| Close nothing before restart return. | Link package passed, command package failed 2 of 6 (post-f 2). |
| Drop the receipt time of a rejected frame. | `TestRetryProtocolRejectionUsesFrameReceiptTime` failed. |
| Queue of one frame. | `TestSessionReceivesHeartbeatsDuringApprovalSetup` failed. |
| Buffer of 4,098 bytes. | `TestRunJoinRejectsOversizedTokenBeforeLineEnds` failed. |
| Accept any byte after a full token. | The oversized open-pipe test and the oversized case of `TestRunJoinTokenInput` failed. |

The lowered-limit mutation that escaped in post-e 4 is now caught by the full-length token test.

**6. Tests that I ran.** All were run with `-race`, as a regular user, on the host.

| Command | Output |
|---|---|
| `go test -race -count=1 ./src/client/...` | All ten packages printed `ok`. The link package took 20.0 seconds. |
| `go test -race -count=200 -run '^TestSessionAcceptsImmediateAck$' ./src/client/internal/link` | `ok` in 18.0 seconds. |
| `go test -race -count=12 -run TestRunCommandExitStatus ./src/client/cmd/lgtm-client` | `ok`. |
| `go vet ./src/client/...` | No output. |

## Older than these repairs

Post-f 4 is the only older item. The server and deployed-stack gaps that post-e recorded (an unimplemented `Open` on the pinned server, C10) are outside these repairs, and I did not repeat them.

## Grade

**8 out of 10.** The five repairs behave as the plan says, the join token stays out of every channel that I could inspect, and the queue keeps order. Two of the repairs, the ceiling and the restart delivery, can be undone without a failing test, and the queue can end a behaving stream under a long local stall.
