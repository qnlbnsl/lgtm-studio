REVISE

A database error while the link runner reads the feed head can leave it without a subscription for the rest of the process, so later removals and commands stop waking open streams.

## Findings

### logic-r1 01 — MAJOR — R6

The feed retry policy skips database errors while reading the head. The runner subscribes, reads the head, and retries only on lag or feed unavailability. The store can instead return a database error directly from the snapshot read. Evidence by reading: `.plans/plans/go-s3-server-stream/plan.md:240`, `src/server/internal/store/read_feed.go:43`, and `src/server/internal/store/pool.go:320`.

Suppose the follower accepts a subscription and PostgreSQL times out on the head query. The runner can exit under the stated policy. After PostgreSQL recovers, an idle removed stream misses its removal notification, and a command recorded for a connected machine can sit until it reconnects. Part C uses the same wake path to deliver approval on an existing stream.

Put subscription acquisition, the head read, starting the subscription, and consuming rows in one recovery loop. Close the attempt's subscription on every failure. Retry transient database failures from the head read with a bounded attempt and the same backoff used for lag. Once the subscription starts again, wake the live streams. Add a test that fails the head read once after subscribing and observes a later removal and outbox wake.

Category is failure recovery. The plan gap is confirmed by reading. The server link runner builder owns it. Risk is 80 (severity 4 because delivery can recover on restart, fallout 5 because all current streams share the runner, and probability 4 because one transient read fault is enough).

### logic-r1 02 — MAJOR — C10

The first post-Welcome frame has different refusal precedence depending on its kind. An event checks identity before its number. The heartbeat write validates load before finding and locking the current row. The dispatcher explicitly checks identity for ignored kinds and results, and it maps a malformed decode to `invalid_argument`. The frozen contract gives the removed stream's next frame `unknown_machine`. Evidence by reading: `.plans/plans/go-s3-server-stream/plan.md:66`, `.plans/plans/go-s3-server-stream/plan.md:123`, `.plans/plans/go-s3-server-stream/plan.md:182`, `.plans/plans/go-s3-server-stream/plan.md:190`, and `src/contracts/proto/lgtm/link/v1/link.proto:164`.

Pause the feed before delivering removal. Commit removal, then deliver a decoded heartbeat with a load of 101. Validating the load first sets `invalid_argument` as the first cause. The client retries that code, although the machine is already removed. A read-only precheck also leaves a race if removal commits between that read and a mutating handler's validation.

Define one post-Welcome refusal order. Preserve any terminal cause already selected. For a new frame decision, check for a missing or displaced stream before applying its frame rules. For heartbeat and event writes, perform that check under the machine lock ahead of semantic validation. For non-writing frames and payload decode failures, use a consistent identity read before choosing a frame refusal. Define transport closure separately from payload decode failure. Test an invalid heartbeat, a wrong-phase frame, and a malformed payload with the follower held, and assert the terminal reason and absence of acknowledgement.

Category is ordering. The plan gap is confirmed by reading. The server stream dispatcher and presence builders own it. Risk is 24 (severity 4 because the client makes an unnecessary retry, fallout 3 because a later connection can learn the removal, and probability 2 because a frame fault must coincide with the removal window).

### logic-r1 03 — MAJOR — D7

The handshake sets Proof's cutoff from completion of Challenge and decides lookup and revision before Proof, but its result channel carries only the message or error. It gives no rule for deciding timeliness when Proof and the timer are both ready after a slow lookup. Evidence by reading: `.plans/plans/go-s3-server-stream/plan.md:137`, `.plans/plans/go-s3-server-stream/plan.md:139`, `.plans/plans/go-s3-server-stream/plan.md:141`, and `src/contracts/proto/lgtm/link/v1/link.proto:105`.

With the test duration shortened, let an unsupported revision or a known machine's lookup remain blocked across the Proof cutoff. For the known machine, put a valid Proof into the reader slot just before the cutoff, then release lookup after it. A timer check at dispatch can reject the timely Proof. A channel selection that accepts a queued Proof without its receive time can also admit one received after the cutoff. A timer that directly sets the terminal cause during lookup can preempt the earlier revision or lookup refusal.

Record the monotonic completion time of the Challenge send and of the first complete Proof receive or parse result. Fix the cutoff at Challenge completion, preserve the lookup and revision precedence, and decide Proof timeliness from its recorded receive time. Define the exact-cutoff case and apply the same rule to Hello at entry. Bound lookup separately so it returns its own database refusal. Stage the lookup across the cutoff with an early valid Proof, a late valid Proof, and a lookup or revision refusal.

Category is timing. The plan gap is confirmed by reading. The server handshake builder owns it. Risk is 48 (severity 4 because a valid machine loses a connection attempt, fallout 3 because reconnect can recover, and probability 4 because one slow lookup can expose the race).

### logic-r1 04 — MAJOR — C8

The disconnect queue has one worker and gives each write up to five seconds, with no progress rule for unrelated ready entries. Each attempted admission except a known missing machine also creates a cleanup obligation, including a write known to have rolled back. Evidence by reading: `.plans/plans/go-s3-server-stream/plan.md:139`, `.plans/plans/go-s3-server-stream/plan.md:239`, `.plans/plans/go-s3-server-stream/plan.md:241`, `src/server/internal/store/pool.go:301`, and `docs/specs/2026-11-25-go-rewrite-foundation-design.md:385`.

Stop a healthy machine behind six cleanup entries whose separate machine rows are locked. A serial worker can spend the entire 30-second offline window before reaching the healthy row. The database can accept that delete throughout. Repeated failed admissions can add more needless work as long as their lookups succeed.

Queue cleanup after a confirmed or possibly committed admission. Keep each cleanup keyed to the exact stream. Schedule eligible entries fairly and keep one contended row from occupying all progress, for example by trying the row lock without waiting and requeueing contention for the next retry. Bound attempts that must wait for shared database resources. Add a test that holds cleanup for one machine through several retries while a separate healthy stopped machine reaches offline.

Category is repair scheduling. The plan gap is confirmed by reading. The server link runner builder owns it. Risk is 24 (severity 4 because presence eventually repairs, fallout 3 because the effect clears automatically, and probability 2 because contention coincides with another machine stopping).

### logic-r1 05 — MINOR — R4

The stated approval test does not prove the approval reply is assembled in the keyed write. The test stages a snapshot read and verifies receipt replay. Its plan claims that those observations also fail when the first approval reply reads presence outside the write. Evidence by reading: `.plans/plans/go-s3-server-stream/plan.md:199`, `.plans/plans/go-s3-server-stream/plan.md:200`, `.plans/plans/go-s3-server-stream/plan.md:201`, and `src/server/internal/machines/records.go:78`.

A builder can read presence in a separate transaction before approval, build the reply from that read, and store those bytes with the keyed write. The scripted approval with no intervening writer returns plausible bytes and the replay returns them verbatim. A replacement between that read and the approval write can pair the newly approved record with the old stream's times and address.

In the approval test, make the presence loader assert it receives the approval transaction and can see the approval revision before an independent connection can see it. Also force the loader to fail and verify rollback of the approval, the feed reference, and the receipt. Keep the existing replay check for byte stability.

Category is verification. The plan gap is confirmed by reading. The server machine service test writer owns it. Risk is 2 (severity 1 and fallout 1 because this is a test gap, and probability 2 because the counterexample uses a staged race).

## Answers to the attack questions

1. The machine row orders report numbers and stream generations. Repeats return the durable mark, gaps end one stream, and the signed ceiling makes all valid numbers repeats at exhaustion. Restart keeps the mark and generation and clears connection rows. The feed supplies its own commit order. Database time supplies display timestamps and Welcome's clock sample. Monotonic time supplies handshake and silence deadlines. The plan needs an exact handshake deadline tie rule and has no explicit generation exhaustion policy beyond a database write failure.
2. A rejected report commit produces no acknowledgement. An uncertain admission ends with a retryable status and schedules exact-stream cleanup. A healthy retry or startup clears stale presence. Join keeps its existing token expiry and replay order. Client report timestamps never order writes. Proof time, feed setup failure, and cleanup contention need the policies in findings 01, 03 and 04.
3. For the first post-Welcome frame, current and valid proceeds, current and invalid follows its kind's refusal, displaced yields aborted, and removed yields unknown machine. The removed or displaced and invalid cell is empty for some kinds and contradictory for heartbeat validation. Admission before removal permits a first Welcome, removal before admission refuses it, and a newer admission suppresses an older unsent Welcome.
4. The durable mark starts at zero and event zero is refused. The first generation and standing revision are positive. A waiting token and name are empty. An absent load is distinct from present zero. An empty scan is valid and counts toward approved readiness. A missing next report advances only its mark. An empty first frame is invalid. The deferred command implementation owns the unspecified outcome mapping.
5. Admission, report writes, heartbeat, disconnect, approval, and removal serialize on the machine row, with the token locked first where needed. Admission publication and the start of Welcome share a per-machine gate. An old disconnect cannot delete the replacement. The feed and a frame handler can still choose different first causes for a committed removal in the case in finding 02.
6. For the five items marked for attack, item 6's write probe catches late fencing and its terminal probe catches an omitted deadline. Item 7's races catch early Welcome and missed uncertain-admission cleanup. Item 8's held commit and full queue catch early acknowledgement and writer starvation. Item 9's staged watch proves the watch snapshot boundary, but replay alone misses a separately read approval projection. Item 10's running fixture catches a missing shutdown signal or startup clear. Question 7 gives a surviving wrong implementation for each item.
7. Item 1 could use a shared row lock that still blocks deletion. Item 2 could let cancellation strand a goroutine waiting to enter the gate. Item 3 could read approval separately after checking identity. Item 4 could reject an inventory with two entries of the same valid name. Item 5 could end the feed runner on its first head read error. Item 6 could leak a gate callback armed before a send that returns without making a write. Item 7 could decide Proof timeliness at dispatch. Item 8 could validate a malformed heartbeat before refusing a removed stream. Item 9 could store a replayable approval reply built from an earlier presence read. Item 10 could log readiness before a failing startup clear if the test observes only the successful restart.
8. One ordered decision for each frame makes identity and validation precedence explicit. A typed admission outcome distinguishes work that needs cleanup from a known rollback. A feed attempt that owns its subscription until it closes makes the snapshot error path release it. The existing exact-stream predicates and atomic report transaction already protect durable state from a stale disconnect and an early acknowledgement.

## Post-build checks

- **logic-r1 P1.** Over both protocols, pause one response after a Welcome enters the writer, admit a replacement, and then release the write. The entered frame may arrive. A second run held before writer entry delivers no older Welcome. Observe the frames, terminal statuses, and each stream's identifier and generation in the database.
- **logic-r1 P2.** On the real stack, send valid heartbeats for longer than ten minutes on one stream. Observe regular acknowledgements and one unchanged connection generation across the proxy's configured ten-minute threshold. Repeat with silence and verify that the monotonic watchdog, cleanup, and feed move the machine offline.
- **logic-r1 P3.** In the combined test with the real client, stage a rollback before a report commits and a separate disconnection after its commit but before the acknowledgement reaches the client. Record the committed audit rows, the subsequent Welcome mark, and the final client outbox. The rolled-back event applies once after reconnect. The committed event has one effect, and the later reports retain their order.
- **logic-r1 P4.** At startup, force a stale-connection delete to fail before listening. Observe that the process refuses readiness. Release the fault and restart. The first snapshot is offline for the old stream, with its machine, mark and generation preserved.

## Grade

7 out of 10. There are 0 blockers, 4 majors, and 1 minor.
