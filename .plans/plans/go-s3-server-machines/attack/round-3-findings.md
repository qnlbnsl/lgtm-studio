# Register of review round 3 of the plan `go-s3-server-machines`

The round had three reports: architect, logic and threat. They were folded on 2026-11-26 into `plan.md`, `decisions.md` rows 77 to 86, and `scouting.md`, under the orchestrator's three rulings on R10, C10 and C8. The plan went from 723 to 711 lines, and its status stays `attack`.

## Every finding and its outcome

A group of duplicates has one letter. The first finding of a group carries its points, and the others name the finding they repeat.

| Finding | Severity | Statement | Group | Outcome | Reason |
|---|---|---|---|---|---|
| architect-r3 01 | blocker | R10 | A | fixed another way | UNREVIEWED MECHANISM, by ruling 1. Item 0, the skeleton, declares every name, seam and schema file, with bodies that answer not built. Every later item fills bodies only, no two items write one file, and each dependency row lists only the items its tests call. The table of public names is removed (decision 77). |
| architect-r3 02 | major | C10 | B | fixed | By ruling 2, one of the reviewers' options. The commit of `BecomeCurrent` is the one admission, `State.Confirm` is removed, and `Welcome` follows the commit outside the transaction. The result file gives the Contract team the exact sentence (decision 78). |
| architect-r3 03 | major | C8 | C | fixed another way | UNREVIEWED MECHANISM, by ruling 3. A scan or restart needs the row and this process's hold on the stream, `Presence.Held` from `Hub.Holds`. A refused delete is a named degraded case of wave 1, retried until it succeeds and cleared at the next start, and the one-second promise is removed (decision 79). |
| architect-r3 04 | major | C8 | E | fixed | Each writer turn handles at most one message and then at most one command, every call takes the stream's context, and the reader's channel holds 16 frames (decision 82). `TestHeartbeatAnsweredDuringBacklog` covers continuous events and a held claim. |
| architect-r3 05 | major | C11 | F | fixed | Rollout step 8 and P4 make the combined test the proof of order and once-only application, with the server's mark read after each released commit and after a replay. The log is diagnosis only. The script is outside this plan, so the result file asks the Stack team and the orchestrator for it. |
| architect-r3 06 | major | R6 | G | fixed | A claim sends an action only before `expires_at + CommandGrace` less one hour, marks nothing, and the sweep marks expiry at 24 hours. The first stored outcome wins (decision 83). |
| architect-r3 07 | major | R5 | I | fixed | `Send` waits for a started callback of `context.AfterFunc` and leaves the deadline expired. `TestSendWaitsForCallback` forces the interleaving under `-race` (decision 84). |
| architect-r3 08 | major | C9 | K | fixed | Every stream's context carries a cause, the first cause wins, and every exit reports `context.Cause` once the context has ended. `Hub.Register` takes the stream's `CancelCauseFunc`, and `Run` cancels streams before its retry stops (decision 81). |
| architect-r3 09 | major | R2 | L | fixed | The key decision says that the combined check of an encrypted join waits for the later work on the proxy. The result file asks the Contract team to move the encrypted cases out of the combined acceptance of wave 1. |
| architect-r3 10 | minor | C11 | M | fixed | An event number above 2^63 − 1 gives `ErrInvalidMessage`, tested at mark 2^63 − 1 (decision 86). The result file asks the Contract team for the ceiling and to drop the promise to show `happened_at`. |
| architect-r3 11 | minor | R8 | N | fixed | The errors paragraph gives one order of mapping, and rename and approval normalize the name before `WriteOnce`. `TestKeyedCallsReplay` covers a taken name and a lock timeout in a keyed call (decision 85). |
| logic-r3 01 | major | C8 | D | fixed | `Open` makes the stream identifier before `BecomeCurrent`, and after any error other than `ErrUnknownMachine`, also an unknown outcome, it hands a `Disconnect` of that identifier to the retry (decision 80). |
| logic-r3 02 | major | C10 | B | fixed | Repeats architect-r3 02. |
| logic-r3 03 | major | C8 | C | fixed another way | Repeats architect-r3 03. |
| logic-r3 04 | major | C8 | E | fixed | Repeats architect-r3 04. |
| logic-r3 05 | major | R6 | G | fixed | Repeats architect-r3 06. The cutoff is now taken with a margin of one hour for the commit and the send. |
| logic-r3 06 | major | C11 | F | fixed | Repeats architect-r3 05. Duplicates in the log are kept for diagnosis and prove nothing. |
| logic-r3 07 | major | C9 | K | fixed | Repeats architect-r3 08. `TestRunStopsStreams` adds a held claim at shutdown and a watchdog during a held heartbeat. |
| logic-r3 08 | major | R10 | A | fixed another way | Repeats architect-r3 01. The shared lock-line test is now the helper `checkOneLine` in item 5's own file, and items 6 and 7 call it from theirs. |
| logic-r3 09 | minor | R6 | H | fixed | P5 runs the fast clock twice: with the receipt intact it replays `done`, and after a pruning write it reports `expired` with no new effect. |
| logic-r3 10 | minor | R8 | N | fixed | Repeats architect-r3 11. |
| threat-r3 01 | blocker | R10 | A | fixed another way | Repeats architect-r3 01. `ErrInvalidMessage` is a skeleton sentinel, so item 6 no longer needs item 7. |
| threat-r3 02 | major | C10 | B | fixed | Repeats architect-r3 02. |
| threat-r3 03 | major | C8 | C | fixed another way | Repeats architect-r3 03, and its part on a lost admission reply repeats logic-r3 01. |
| threat-r3 04 | major | C8 | E | fixed | Repeats architect-r3 04. |
| threat-r3 05 | major | R6 | G | fixed | Repeats architect-r3 06. |
| threat-r3 06 | major | C11 | F | fixed | Repeats architect-r3 05. |
| threat-r3 07 | minor | R5 | J | post-build check | `watch.serve` is still a stub (`src/server/internal/watch/watch.go:50`), so the revocation of a blocked send cannot be tested in this plan. Item 11 now hands `transport.Send` the context that `watch.Serve` passes, and P11 checks revocation at the ordinary 10-second deadline. |

## Points by statement

A minor finding is 1 point, a major is 3, and a blocker is 9. Each group counts once, at its highest severity.

| Statement | Groups | Points |
|---|---|---|
| R10 | A (9) | 9 |
| C8 | C (3), D (3), E (3) | 9 |
| C11 | F (3), M (1) | 4 |
| R6 | G (3), H (1) | 4 |
| R5 | I (3), J (1) | 4 |
| C10 | B (3) | 3 |
| C9 | K (3) | 3 |
| R2 | L (3) | 3 |
| R8 | N (1) | 1 |
| **Total** | 14 groups: 1 blocker, 9 majors, 4 minors | **40** |

## Stuck statements

A statement is stuck with at least 12 points and at least 30% of the round's points, which is 12 points. No statement is stuck. R10 and C8 each have 9 points, which is 23% of the round.

## Mechanisms with several faults

- **The order of items.** R10 drew a blocker in rounds 2 and 3. A table that names who declares each name kept missing names, so ruling 1 replaces it with a skeleton that declares everything first.
- **Presence and the writer.** C8 holds three majors in groups C, D and E. A refused delete, a lost commit reply and a busy writer each left the page or the stream wrong. Ruling 3 names the first as a degraded case, and the other two are fixed in item 8 and item 10.

## Places changed in the plan

Round 4 reads only blockers and majors, at these places.

1. Overview: the paragraph on commands, and the key decision on cleartext.
2. Rollout order: steps 1 to 5 and 8, and the sentence on database changes.
3. Implementation details: "How every item is built", the fixtures, "The one rule of locking", "Errors that cross the store", and the new item 0 in place of the table of public names.
4. Every item header, which now names only the files the item writes.
5. WS-1: item 1's `Presence`, and the migration targets of items 1 and 3.
6. WS-2: items 2, 5, 8, 9 and 10.
7. WS-3: items 6 and 7.
8. WS-4: items 11, 12, 13 and 14.
9. Traceability, execution sizing with the dependency column, the load, and attack log row 3.
10. Post-build checks: rows P2, P3, P4, P5 and P8, and the new row P11.

## Top finding of the round

The top finding is that the order of items still let an item call a name that a sibling item builds, so it could not pass its own gate (group A). It was the one blocker, and all three reviewers found it.

## Verdicts and grades

| Reviewer | Verdict | Grade |
|---|---|---|
| architect | REVISE | 5/10 |
| logic | REVISE | 5/10 |
| threat | REVISE | 5/10 |
