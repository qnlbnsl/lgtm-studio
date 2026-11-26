# Register of review round 2 of the plan `go-s3-server-machines`

The round had three reports: architect, logic and threat. They were folded on 2026-11-26 into `plan.md`, `decisions.md` rows 64 to 76, and `scouting.md`. The plan went from 732 to 723 lines, and its status stays `attack`.

## Every finding and its outcome

A group of duplicates has one letter. The first finding of a group carries its points, and the others name the finding they repeat.

| Finding | Severity | Statement | Group | Outcome | Reason |
|---|---|---|---|---|---|
| architect-r2 01 | blocker | R10 | A | fixed | One table of public names gives each item the names it declares and the items it needs. The stale result of each write is its own: `ErrNotCurrent`, or `false` for `Disconnect` and `AcceptResult`. The mutation of `ApplyEvent` moved to item 7, item 9 no longer runs beside item 8, item 11 needs items 3 and 6, and `machinetest.NewServer` wraps the deadline middleware (decision 74). |
| architect-r2 02 | major | C10 | B | fixed | The reviewer's second option: the commit of `State.Confirm` is the moment at which the server admits a stream to send `Welcome`, and a removal or newer admission after it gives a `Welcome` in transit. A private hook `afterConfirm` tests that gap, and every exit after the admission cleans up (decision 64). The result file asks the Contract team to confirm its checks 10 and 16. |
| architect-r2 03 | major | C8 | C | fixed another way | UNREVIEWED MECHANISM. `Presence` no longer filters by the wall clock, so a fresh read and an open watch agree, and `Run` retries a failed delete every second. Check C8 of the design stops the client, which closes the stream at once, so a delete that the database refuses is outside C8 (decision 65). |
| architect-r2 04 | major | R6 | D | fixed | The overview states the limit of the machine's clock, and P5 rehearses a clock 5 minutes fast. The result file asks the contract and the client plans to state the same limit (decision 75). |
| architect-r2 05 | major | R5 | E | fixed | `Send` stops the callback of `context.AfterFunc` before it clears the deadline, and returns the context's error without sending once the context ended. The tests expect a status only while the response can be written, and a reset after a blocked send. |
| architect-r2 06 | major | C8 | F | fixed another way | UNREVIEWED MECHANISM. A claim takes one command, and the writer handles every received message before each command, so a heartbeat waits for at most one send. An expired prefix is marked in the same claim and cannot hide later work. `CommandBatch` is removed, because the store's cap counts feed references, not rows (decision 66). `TestHeartbeatAnsweredDuringBacklog` is added. |
| architect-r2 07 | major | C11 | G | fixed | Rollout step 8 now runs C11 only after the Stack team's revision of `deploy/test/checks.sh`, which reads the server's mark and compares the applied numbers as a sorted set. The script is outside this plan, so the result file asks the orchestrator to assign that work. |
| architect-r2 08 | major | C9 | H | fixed | Every `Open` derives its context from a context of the service that only the end of `Run` cancels, from its entry. A stream held at a seam, or one that registers late, ends with `unavailable`. `Hub.CloseAll` is removed, and `TestRunStopsStreams` covers both seams (decision 67). |
| architect-r2 09 | major | R10 | I | fixed | `api.Options` gains `LinkOptions`, and `app.Start` gains `app.Hooks{LinkSeams}`. Item 14 writes them, the server command and `servertest` call them, and two tests cover an oversized `Join` and the seams after a restart (decision 73). |
| architect-r2 10 | minor | C11 | J | fixed | `facts_at` is removed, so the server stores no report time (decision 68). The result file asks the Contract team to drop the promise to show `happened_at`, and to publish one ceiling of event numbers. |
| architect-r2 11 | minor | R8 | L | fixed | The page's calls give `aborted` for a retryable store error, and `Join` and `Open` give `unavailable` for every store error. A name is normalized before the write, and the sanitizer's exceptions are named (decision 72). |
| logic-r2 01 | major | C8 | C | fixed another way | Repeats architect-r2 03. |
| logic-r2 02 | major | C10 | B | fixed | Repeats architect-r2 02. Its rule that every exit after the admission cleans up is applied. |
| logic-r2 03 | major | C11 | M | fixed | The commit gate is a `pgx.QueryTracer` of the test, which holds the gated `COMMIT` or ends its backend. A deferred trigger fired at `SET CONSTRAINTS ALL IMMEDIATE`, before the feed's flush (decision 70). |
| logic-r2 04 | major | R10 | A | fixed | Repeats architect-r2 01. Item 10's test no longer reads a real `WatchClients`. |
| logic-r2 05 | major | C8 | F | fixed another way | Repeats architect-r2 06. A claim with no command records no feed row. |
| logic-r2 06 | major | C11 | K | fixed | Fixed by removing `facts_at`, as the task's rule on growth prefers. An event without `happened_at` or with one out of range is applied like any other, and `TestEventTable` covers both (decision 68). |
| logic-r2 07 | major | C11 | G | fixed | Repeats architect-r2 07. |
| logic-r2 08 | minor | R8 | L | fixed | Repeats architect-r2 11. A store error of `Confirm` gives `unavailable`. |
| logic-r2 09 | minor | R6 | N | fixed | The mutation drops `machine_id` and `sent_stream_id` together. |
| logic-r2 10 | minor | R6 | O | fixed | Every write of an outcome also sets `finished_at`, a check of the table refuses one without the other, and a matching result wins while its row is pending (decision 71). |
| threat-r2 01 | blocker | R10 | A | fixed | Repeats architect-r2 01. `HeartbeatTimeout` is no longer read by item 5, and `Forget` of item 5 deletes two tables. |
| threat-r2 02 | major | C10 | B | fixed | Repeats architect-r2 02. |
| threat-r2 03 | major | C8 | C | fixed another way | Repeats architect-r2 03. |
| threat-r2 04 | major | R5 | E | fixed | Repeats architect-r2 05. A send that is blocked when a session is revoked delivers nothing, so the watch's test now asserts that no change written after the revocation arrives. |
| threat-r2 05 | major | R9 | P | fixed | The registry copies the declared fields of `Facts` and of `HarnessesFound` and its entries into a new message before encoding, and a test checks that a marked unknown field is stored nowhere (decision 69). |
| threat-r2 06 | major | R6 | Q | fixed another way | UNREVIEWED MECHANISM, the one of group F. A claim takes one command just before its send, so no command waits behind others past the cutoff. The window of one send is part of the clock limit of group D. |
| threat-r2 07 | major | C11 | G | fixed | Repeats architect-r2 07. |
| threat-r2 08 | major | C9 | H | fixed | Repeats architect-r2 08. |
| threat-r2 09 | minor | C7 | R | fixed | `TestScanAndReconnect` expects "This machine is not online." for a scan and "This machine is not connected." for a restart, and makes both reports explicit. |

## Points by statement

A minor finding is 1 point, a major is 3, and a blocker is 9. Each group counts once, at its highest severity.

| Statement | Groups | Points |
|---|---|---|
| R10 | A (9), I (3) | 12 |
| C11 | G (3), J (1), K (3), M (3) | 10 |
| R6 | D (3), N (1), O (1), Q (3) | 8 |
| C8 | C (3), F (3) | 6 |
| C10 | B (3) | 3 |
| R5 | E (3) | 3 |
| C9 | H (3) | 3 |
| R9 | P (3) | 3 |
| R8 | L (1) | 1 |
| C7 | R (1) | 1 |
| **Total** | 18 groups: 1 blocker, 12 majors, 5 minors | **50** |

## Stuck statements

A statement is stuck with at least 12 points and at least 30% of the round's points, which is 15 points. No statement is stuck. R10 has 12 points, which is 24% of the round.

## Mechanisms with several faults

Two mechanisms held several majors, and both were wrong, so both are replaced.

- **Presence by the wall clock.** Groups C and F, and part of E, sat in presence and its heartbeat. The filter of round 1 hid a row from a fresh read while an open watch still showed it, and it disagreed with the monotonic watchdog. Presence is now the row alone, and the stream's end deletes it.
- **The batch of commands.** Groups F and Q sat in the claim of 64 commands. The batch let a heartbeat wait behind many sends and let a command pass its cutoff in a queue. It also existed for a limit that does not apply, because the store's cap counts feed references. A claim now takes one command, and only `Enqueue` records `machine_outbox`.

The order of the first `Welcome` against a removal (group B) is one major. Its mechanism is right, and the fix names the moment that the lock already orders.

## Places changed in the plan

The next round attacks only these places.

1. Overview: the paragraph on commands.
2. Key decisions: "Wave 1 tokens are for the loopback address only", for the settled cleartext path.
3. Rollout order: steps 3 to 5 and 8.
4. Implementation details: "Fixtures shared by every item", "The one rule of locking", "Errors that cross the store", the new table "Public names, in the order of the items" and the Go listing after it, and two rows of "Codes and sentences".
5. WS-1: item 1.
6. WS-2: items 2, 5, 8, 9 and 10.
7. WS-3: items 6 and 7.
8. WS-4: items 11, 12, 13 and 14.
9. Traceability: the source universe and the merged rows.
10. Execution sizing: the row of item 11.
11. Attack log: row 2.
12. Post-build checks: rows P2, P3, P4, P5, P8 and P10.

## Top finding of the round

The top finding is that work items used names and tables that later items first write, so an item could not pass its own gate (group A). It was a blocker, and all three reviewers found it alone.

## Verdicts and grades

| Reviewer | Verdict | Grade |
|---|---|---|
| architect | REVISE | 5/10 |
| logic | REVISE | 5/10 |
| threat | REVISE | 5/10 |
