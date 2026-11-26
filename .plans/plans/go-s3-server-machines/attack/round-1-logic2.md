# Round 1, seat logic2: review of `go-s3-server-machines`

Task identifier: review-go-s3-server-machines-r1-logic2. Nonce: 756b16. Lens: logic. Reviewed text: `.plans/plans/go-s3-server-machines/plan.md`, as it is now.

## Verdict

REVISE

## Top finding

The plan never says where the server's challenge nonce comes from and no test shows that it is fresh, so a constant or predictable nonce passes every listed test while a replayed `Proof` would admit an impersonator (logic2-r1 1).

## Findings

### logic2-r1 1. The nonce that defeats a replayed challenge has no source and no test

- **Severity:** MAJOR. A wrong build lets a recorded `Hello` and `Proof` pair open a stream for a machine that does not hold the key.
- **Tied to:** C10, and D7 (the two-way proof).
- **Evidence:** `.plans/plans/go-s3-server-machines/plan.md:362` says only that the server "draws a 32-byte nonce". Only the token secret names `crypto/rand` (`plan.md:252`). The connection token is stated as 32 random bytes (`plan.md:336`), but the stream identifier has no stated source. The handshake tests at `plan.md:369` check that the `Challenge` verifies with the server's key, that nonces swapped inside one text fail, and that a signature by another key fails. No test compares the nonces of two streams.
- **Failure scenario:** A builder writes `serverNonce := make([]byte, 32)` and leaves it zero, or takes it from `math/rand`. `TestHandshakeOpensStream`, `TestHandshakeDecisionOrder` and `TestProofTextsMatchContractExamples` all pass. An eavesdropper on the loopback or the proxy hop records the machine's `Hello` and `Proof`. The same bytes sent on a new stream verify against the same nonces, so the attacker gets a `Welcome`, a current stream, and the machine's presence and commands.
- **Fix:** In item 8, state that the server nonce and the stream identifier come from `crypto/rand`, through one private function. Add `TestServerNonceFreshPerStream`: two streams that send the same `Hello` get different `server_nonce` values. Add `TestReplayedProofRefused`: the `Proof` signed on stream one, sent on stream two with the same `machine_nonce`, ends with `unauthenticated`. Add a mutation to the risky line of item 8: returning a fixed nonce must fail `TestReplayedProofRefused`.

### logic2-r1 2. A failed `Disconnect` leaves a machine online until the machine returns or the server restarts

- **Severity:** MAJOR. Check C8 fails, and the wrong value does not heal by itself.
- **Tied to:** C8, and D7 (presence).
- **Evidence:** `plan.md:29-34` and decision 41 say that presence is a database row and that there is no lease and no sweep. `plan.md:337` makes `Disconnect` the only deleter of a `link_connections` row while the process runs, with a 5-second context. `plan.md:338` and `plan.md:417` repair only at server start-up. `plan.md:367` calls `Disconnect` once when the stream ends.
- **Failure scenario:** PostgreSQL restarts for ten seconds while the server runs. Every stream ends with `unavailable`, and every `Disconnect` fails because the database is down. The database returns. Machine A's client redials and is admitted, which replaces its row. Machine B's client was stopped during the outage and never returns. Its row stays, so the page shows B online with its last load, with no heartbeat for hours. The same happens when `Disconnect` waits longer than 5 seconds on the machine's lock, and when the server stops while the pool is already closed.
- **Fix:** Keep the table and the rule that only a stream's own end deletes its row. Add a retry that cannot be missed. When `Disconnect` fails, put the pair of machine and stream identifiers in a process-local set. `Service.Run` retries every set member every 5 seconds until `Disconnect` returns without error, which includes the answer `false`. Add `TestFailedDisconnectIsRetried`: a trigger fails the delete once, the stream ends, and within 15 seconds `Presence` shows the machine gone. Add the same case to the post-build checks below.

### logic2-r1 3. `Welcome.state` comes from a read taken before the admission

- **Severity:** MINOR. A wrong `Welcome` is possible, and it heals on the next stream.
- **Tied to:** D7.
- **Evidence:** `plan.md:363` reads the machine in `Store.Read` right after the `Challenge`, before `Proof` arrives. `plan.md:365` builds `Welcome{state = machines.Standing(rec)}` from that read. `plan.md:336` and the table row at `plan.md:346` show that `BecomeCurrent` already returns `Stream.Standing` and decides the connection token under the machine's lock.
- **Failure scenario:** The machine is waiting and reconnects. The server reads it as waiting at revision 1, then waits for `Proof`, which can take up to the 10-second `HandshakeTimeout`. The owner approves in that window. `BecomeCurrent` sees the approved machine and makes a connection token. `Welcome` says `APPROVAL_WAITING`, revision 1, with a non-empty token. The client plan requires an empty token for a waiting `Welcome` (`.plans/plans/go-s3-client-link/plan.md:153`), so the client either rejects the stream or keeps a stale standing until the `SetState` arrives. The page shows the wrong state for one retry.
- **Fix:** In step 5 of item 8, build `Welcome.state` from `Stream.Standing`, and never from the record of step 3. Say in the plan that step 3's read decides only the refusals. Add to `TestHandshakeOpensStream` a case in which `Seams.AfterProof` holds, the test approves the machine, and then releases. The `Welcome` must be approved, at revision 2, with a non-empty token.

### logic2-r1 4. Only `ApplyEvent` is tested for the machine lock

- **Severity:** MINOR. A missing lock in four other writes passes every listed test.
- **Tied to:** R4, and the plan's rule at `plan.md:120`.
- **Evidence:** `plan.md:324` names two mutations, and the second is only "removing the call of `Registry.Lock` from `ApplyEvent`". `plan.md:341` puts the `afterLock` seam "in each write", but `TestWritesOfOneMachineFormOneLine` (`plan.md:347`) uses it for `ApplyEvent` and `BecomeCurrent` only. `Heartbeat`, `Disconnect`, `Claim` and `AcceptResult` have no race test.
- **Failure scenario:** A builder leaves `Registry.Lock` out of `AcceptResult` and conditions the update only on `id` and `outcome IS NULL`. Stream A reads that it is current and that the command has attempt 1. Stream B is admitted and claims attempt 2. A's late result then closes the command under attempt 1, although the contract says only the newest attempt's answer counts. All listed tests pass.
- **Fix:** Turn `TestWritesOfOneMachineFormOneLine` into a table over every write of `State`: `Heartbeat`, `Disconnect`, `Claim`, `AcceptResult` and `ApplyEvent`, each held at `afterLock` against `BecomeCurrent` in both orders. Extend the risky lines of items 5 and 6 so that each omission of `Registry.Lock` must fail that table.

### logic2-r1 5. The generation counter can be constant and every test still passes

- **Severity:** MINOR. Two streams of one machine in a narrow window can leave the hub with the wrong stream.
- **Tied to:** D7.
- **Evidence:** `plan.md:336` raises `stream_generation` at each admission. `TestBecomeCurrent` (`plan.md:346`) asserts generation 1 for a first admission only. `plan.md:377` makes `Register` replace a handle unless the existing one has a higher generation. `TestOlderGenerationCannotReplaceNewer` (`plan.md:378`) uses hand-made generations in the hub alone.
- **Failure scenario:** `BecomeCurrent` always returns generation 1. Stream A is admitted and held at `Seams.AfterAdmission`. Stream B is admitted, registered, and sends `Welcome`. A is released and registers, and equal generations replace the older handle, so B is ended with `aborted` although the database names B as current. B's client redials, which ends A. The machine's presence flaps once per redial. No listed test notices.
- **Fix:** Add to `TestBecomeCurrent` a second and third admission that return generations 2 and 3. Add `TestStaleAdmissionCannotRegister` to item 8 or 10: hold A at `Seams.AfterAdmission`, run B to `Welcome`, then release A. A must end with `aborted` and no `Welcome`, and B must stay current.

### logic2-r1 6. A heartbeat with a load of 0 is never tested

- **Severity:** MINOR. The page can show a measured zero as an unmeasured value.
- **Tied to:** D7, and C7 (load shown for an online machine).
- **Evidence:** The contract makes both load fields optional with a range of 0 to 100 (`src/contracts/proto/lgtm/link/v1/events.proto`, message `Heartbeat`). `TestHeartbeatAndDisconnect` (`plan.md:349`) covers 7 and 28, and covers no load. `TestHeartbeatIsAnswered` (`plan.md:396`) covers 7, 28 and 101.
- **Failure scenario:** The handler treats 0 as absent, for example with `if cpu > 0`. Both tests pass. An idle machine reports 0 percent of processor load, and the page shows no load while online. The client plan has its own test for a present zero (`.plans/plans/go-s3-client-link/plan.md:184`), so the server's half is the one that is missing.
- **Fix:** Add a case with `cpu_percent` 0 and `memory_percent` absent to `TestHeartbeatAndDisconnect`. The row must hold 0 and null. Add the same case to `TestProjectClient`, where the online machine must show 0 for the processor and no memory value.

### logic2-r1 7. Event numbers are `uint64` on the wire and `bigint` in the table

- **Severity:** MINOR. A corrupt or hostile client gets the wrong answer, and an honest one never reaches it.
- **Tied to:** R6, and C11.
- **Evidence:** `Event.number` is `uint64` (`src/contracts/proto/lgtm/link/v1/events.proto`). `plan.md:329` stores `event_mark bigint`. `plan.md:481-483` compares the number with the mark and with the mark plus one. `TestEventTable` (`plan.md:491`) tops out at 45.
- **Failure scenario:** A number of 2^63 is cast to a signed 64-bit value and becomes negative. Step 2 then reads it as "at or below the mark" and answers `EventAck` with the mark, where the contract demands `event_gap`. With a driver that refuses the cast, the same event gives a store error that ends the stream with `unavailable`, which the client retries forever.
- **Fix:** In item 7, compare the number as `uint64` before it becomes a query argument, and say so. Add 2^63 and the largest `uint64` to `TestEventTable`. Both must give `ErrEventGap`, a mark that did not move, and no store error.

## Answers to the lens questions

1. **Sequences, counters, expiries.** Event numbers are ordered by the machine's single outbox and decided by number alone, with a tie, repeat or gap handled by the table at `plan.md:480-483`. The wrap case at 2^63 is finding 7. Attempts rise once per stream in `Claim` and survive a restart because they are stored. The generation is a counter per machine and is finding 5. Token expiry and command expiry read the database clock after the lock (`plan.md:263`, `plan.md:457`), so clocks that disagree do not matter. The heartbeat timer is a monotonic Go timer, as the contract wants.
2. **Failures.** A crash in the middle of an event leaves nothing, because the effect and the mark share one commit. A crash between the `Claim` commit and the send costs one attempt number, and the next stream sends the next one. A database that is down ends streams with `unavailable`, and `Join` answers `unavailable`. The repair of the presence row after a failed `Disconnect` is missing (finding 2). An expired token is refused before any write.
3. **Truth table of standing, presence and flags.** Waiting with or without a stream shows waiting. Approved without presence shows offline. Approved with presence and both flags shows online, and with fewer flags shows joining. Waiting with flags cannot occur, because a waiting machine's events are consumed. The one contradictory cell is an approved machine whose stream is dead and whose row survived (finding 2), which shows online with a frozen load. I found no empty cell.
4. **Zero values.** The first approval value is waiting, and the generation, mark and attempt all start at 0 with a first real value of 1. Hello's `link_revision` of 0 gives `link_revision_mismatch` in the contract's order. A present load of 0 is untested (finding 6). A number of 0 gives `ErrEventNumberZero`. An empty `Welcome` token is correct for a waiting machine.
5. **Two actors on one row.** Approval and revocation: decision 48 and `DeleteWaiting` settle it, and `TestRevokeClientToken` holds the approval on the lock. Two first uses of one token: the token row lock settles it. Two streams of one machine: the machine lock plus the generation settle it, with the gaps in findings 4 and 5. Approval and a reconnecting machine: finding 3. Removal against any stream write: the removal waits for the lock, and a waiting write then finds no row and gets `ErrNoMachine`.
6. **Risky items and their mutations.** The mutations of items 4, 6, 7 and 10 do make their named tests fail: I traced the repeat with a broken report in `TestEventTable`, the trigger in `TestEffectAndMarkCommitTogether`, the older attempt in `TestAcceptResultRules`, and the failing mark trigger in `TestEventsAckedAfterCommit`. Wrong changes the tests miss: a missing lock in four writes (finding 4), a constant generation (finding 5), a constant nonce (finding 1).
7. **Cheapest wrong implementations.** Item 5: a constant generation, and a heartbeat that treats 0 as absent. Item 8: a constant server nonce. Item 7: a signed cast of the number. Item 10: none found beyond finding 2. Item 9: `TestFollowResubscribes` checks only that handles wake. That is enough, because a wake runs `Claim`, which locks the machine and ends a removed machine's stream with `unknown_machine`.
8. **A different shape.** A foreign key from `link_connections` and `link_commands` to `machines` with `ON DELETE CASCADE` would make an orphaned row impossible, where today `Forget` must be called by hand in every removal path. The parts migrate separately, so this is a choice for the planner and not a finding. A presence row that outlives its stream (finding 2) would be impossible if presence were read from the hub and the row held only history, but that reopens decision 41.

## Answers to the task's questions

- **a. Join and handshake.** A used or expired token cannot get further: `Redeem` locks the token row, reads the database clock after the lock, and follows the contract's order of refusals (`plan.md:263-271`), and `TestRedeemRefusals` and `TestRedeemConcurrentUses` cover each cell. An unknown key gets `unauthenticated` after a verified `Challenge`, and an unknown machine gets `unknown_machine`, both in the contract's order. A replayed challenge is only safe if the nonce is fresh, and the plan does not say so (finding 1). A second connection of the same machine replaces the first under the machine lock, with the test gaps in findings 4 and 5.
- **b. Commands and events.** A command cannot run twice, because the attempt is stored and committed before the send and only the stored attempt's answer counts. It cannot be lost, because it stays pending until a result and each new stream claims it again. An event cannot be applied twice or out of order across a restart of the server, because the mark and the effect share one commit and `Welcome.last_event_number` tells the client what to resend. A restart of the client replays from its own outbox, and the client plan resends fresh facts and harnesses on every approved `Welcome` (`.plans/plans/go-s3-client-link/plan.md:206`), which the readiness flags need. The server commits before it acknowledges (`plan.md:398`, `plan.md:507`). The wrap case is finding 7.
- **c. Presence and the contract.** Heartbeats every 10 seconds with a 30-second timer reset at each heartbeat show a stopped client offline at 30 seconds after its last heartbeat plus the time of the write and the feed. A client that closes its stream shows offline at once. A killed client with no closing frame can show offline slightly after 30 seconds from the moment of the kill, which is a post-build check below. A server restart clears every row, and the client's redial plus its fresh facts and harnesses bring it back online (C9). Two streams of one machine leave one current stream, and the older ends with `aborted`. The plan builds against the exact messages as they stand: the files `src/contracts/proto/lgtm/link/v1/{link,events,commands}.proto` equal the drafts in `.plans/plans/go-s0-contract/draft/`. One departure is harmless: the table stores attempt 0 and the first send is attempt 1, where the contract text says a command is stored with attempt 1. Each attempt is still stored before it leaves.

## Post-build checks

1. **Killed client, C8.** Kill the client's process with no closing frame, just after a heartbeat. Expected: `WatchClients` shows the machine offline within 30 seconds plus a few seconds of margin, and the check's own window should be read with that margin in mind.
2. **Database blip.** Restart PostgreSQL while a machine is connected, and stop that machine's client during the outage. Expected after the database returns: the machine shows offline within 10 seconds (finding 2).
3. **Approval during reconnect.** Loop 100 times: the client reconnects while the test approves the machine at a random moment. Expected: every `Welcome` has either waiting with an empty token, or approved with a token and a matching revision (finding 3).
4. **Replayed proof.** Record a `Hello` and `Proof` on the loopback and send them again on a new stream. Expected: `unauthenticated`.
5. **Server killed during a backlog.** Buffer 100 events at the client, start the server, and kill the server with `SIGKILL` during replay. Expected after restart: the log holds each sequence of the machine at most once and in order, the final mark is 100, and the machine shows online.
6. **Removal during a feed lag.** Remove a connected machine while the hub's follower is forced into `ErrWatchLagged`. Expected: the machine's stream ends with `unknown_machine` within 3 seconds.
7. **Start-up order.** Dial the server in a tight loop during start. Expected: no stream is admitted before `ClearConnections` has run, and no machine that connects during start-up stays offline.
8. **Two clients on one machine record.** Run two clients that hold the same machine key. Expected: each redial replaces the other, the page shows at most one online connection, and neither client's events are applied out of order. The contract excludes copies of state in wave 1, so this check only bounds the damage.
9. **Readiness after a consumed report.** Send a `Facts` with `cores` 0 from an approved machine. Expected: the mark moves, the machine stays joining, and the next stream with valid facts brings it online.

## Grade

7 out of 10. The plan is careful about ordering and transactions, and I found no blocker. Counts: 0 blockers, 2 majors, 5 minors.
