# Review round 2 — register of findings

This register folds the four reports of round 2: `round-2-architect.md`, `round-2-logic.md`, `round-2-logic2.md` and `round-2-threat.md`.
The reviewed revision is the frozen copy `attack/plan-v2.md`, with the drafts as they stood after round 1.

The orchestrator gave four rulings for this round, and this register applies them.

1. The server is the only authority on what was applied. `Hello.acked_event_number` is removed, and the events return to the design's three lines under "Events come up, and none is lost or applied twice".
2. A restore of the server's database is outside wave 1. One named code covers a machine whose numbers run ahead of the server's and a removed machine that is known again. On it the client stops and reports.
3. The order of removal, stream admission and writes belongs to plan `go-s3-server-machines`. The contract states only what the other end observes.
4. A rule or field that no wave-1 check and no design statement needs is removed rather than patched.

A finding that sits in a mechanism of a ruling has the outcome "fixed another way", and the changed place in the plan is marked UNREVIEWED MECHANISM with the ruling's number.

## 1. Every finding

The severities are B for a blocker, M for a major and m for a minor.
The group column names the group of duplicates, and the "Repeats" column names the first finding of that group.

| Finding | Severity | Statement | Group | Outcome | Reason | Repeats |
|---|---|---|---|---|---|---|
| architect-r2 01 | B | C11 | G1 | fixed another way | Ruling 1 removes `Hello.acked_event_number`, so no claim raises the mark past absent effects. Ruling 2 puts the restore outside wave 1, and the machine stops when the server's number is below one it acknowledged. | — |
| architect-r2 02 | B | C11 | G1 | fixed another way | Ruling 1. The wrong scouting claim about `Ack` beyond `Next()` is corrected, and the contract now matches the client-local plan's `Reconcile`, which renumbers unsent events after a higher mark and refuses a lower one. | architect-r2 01 |
| architect-r2 03 | B | D7 | G2 | fixed another way | Ruling 3. The contract keeps the code of removal and the refused later `Open`. The fence is a request to `go-s3-server-machines` in the result file, and post-build check 10. | — |
| architect-r2 04 | B | D17 | G3 | fixed | Points 1 and 2 for the owner must be settled before item 6 merges. If they are open, the link does not freeze, and a choice that changes it returns to phase 3 before sprint 2. | — |
| architect-r2 05 | B | plan | G4 | fixed | Items 2 to 6 run the Go check's commands directly. The script is required from item 7 on. `buf` runs through `pnpm --filter ./src/contracts exec`, and the separate checkout is installed and removed. | — |
| architect-r2 06 | M | C10 | G5 | fixed | `unauthenticated` now means a bad signature and retries. Only `failed_precondition` after a verified `Challenge` stops, with exit status 0, which the Stack Client service does not restart. `aborted` has a bounded wait. A failed send still reads the server's last code. | — |
| architect-r2 07 | M | R6 | G6 | fixed | The guarantee is now "never late": a machine may refuse a command up to 10 seconds early. The receipts take the same count for `Begin` and pruning, and prune nothing before the first `Welcome`. | — |
| architect-r2 08 | M | C7 | G7 | fixed | One projection replaces the two rules: waiting, joining, online, offline. `Welcome` no longer marks a machine online. Approval by `SetState` sends the two reports. | — |
| architect-r2 09 | M | D17 | G8 | fixed | Two issuance contexts: the public address with its pin, and loopback with no pin for the server's own command, which the installer uses. Stack Web names the pin's setting. The client's persisted fields have a writer and a reader. | architect-r2 10 |
| architect-r2 10 | M | C6 | G8 | fixed | `CreateClientTokenResponse.address`, field 4, gives the dialog the token's address. A sample server may return other token text, so no coordinated handoff with the sample server is needed. | — |
| architect-r2 11 | M | C5 | G9 | fixed | The client retries `Join` for at most 15 minutes after its first try, which is the token's life. The server stays the authority on expiry. | — |
| architect-r2 12 | M | R9 | G10 | fixed | `CreateClientToken` refuses a present `Lgtm-Request-Key` with `invalid_argument` before it makes a token, as the Sign-in plan does for `SignIn`. | — |
| logic-r2 01 | B | C11 | G1 | fixed another way | Ruling 1. An empty outbox renumbers after the server's number through `Reconcile`, as the client-local plan's test `TestReconcileNewStoreRenumbers` already shows. | architect-r2 02 |
| logic-r2 02 | B | C11 | G1 | fixed another way | Ruling 1 removes the claim, and ruling 2 puts the restore outside wave 1. | architect-r2 01 |
| logic-r2 03 | B | D7 | G2 | fixed another way | Ruling 3. | architect-r2 03 |
| logic-r2 04 | B | plan | G4 | fixed | As architect-r2 05. | architect-r2 05 |
| logic-r2 05 | M | R6 | G6 | fixed | As architect-r2 07. The arithmetic of the finding is right, and the plan now promises only "never late". | architect-r2 07 |
| logic-r2 06 | M | C10 | G5 | fixed | As architect-r2 06. The link revision is checked after the `Challenge`, so an unsupported revision is `failed_precondition`, and the value rules no longer name it. | architect-r2 06 |
| logic-r2 07 | M | C11 | G11 | fixed another way | Ruling 1 rewrote the rules of events. The machine checks a report before it numbers it, and the server consumes and logs a report it cannot use at the next number. | — |
| logic-r2 08 | M | C5 | G9 | fixed | Revocation and expiry are checked before a replay. A new token is claimed by a conditional update, so one of two first uses wins. The replay is named as the one exception to C5. | architect-r2 11 |
| logic-r2 09 | M | C7 | G7 | fixed | As architect-r2 08. | architect-r2 08 |
| logic-r2 10 | M | C6 | G8 | fixed | As architect-r2 09 and 10. | architect-r2 10 |
| logic-r2 11 | M | D16 | G12 | fixed | After its receipt is on disk, `Restart` tries to send for at most 5 seconds and exits with 75 whether or not the send worked. The new process answers from the receipt. | — |
| logic-r2 12 | M | R9 | G10 | fixed | As architect-r2 12. | architect-r2 12 |
| logic2-r2 1 | B | C11 | G1 | fixed another way | Ruling 1 removes the unsigned claim, so there is nothing to sign. | architect-r2 01 |
| logic2-r2 2 | M | C11 | G1 | fixed another way | Ruling 1. No epoch is added: the server's number alone decides, and `Reconcile` renumbers a new outbox. | architect-r2 02 |
| logic2-r2 3 | M | C7 | G7 | fixed | The machine sends its two reports when it becomes approved on a stream, by `Welcome` or by `SetState`. Post-build check 13. | architect-r2 08 |
| logic2-r2 4 | M | plan | G13 | fixed another way | Ruling 2: a restore that lowers the standing's revision is outside wave 1. The owner removes and joins the machine again. | threat-r2 03 |
| logic2-r2 5 | M | C10 | G13 | fixed another way | Ruling 2: the machine stops and reports on `failed_precondition`, and the owner joins it again. | threat-r2 03 |
| logic2-r2 6 | m | D7 | G2 | fixed | A second `aborted` within one minute waits 60 seconds. The stream counter and its lock are part of the request to `go-s3-server-machines` under ruling 3. | architect-r2 03 |
| logic2-r2 7 | m | R6 | G6 | fixed | The agreement names the Client Link as the source of the receipts' clock, and `go-s1-client-local` as its reader. | architect-r2 07 |
| logic2-r2 8 | m | plan | G14 | fixed | The comment test needs three words of two letters or more above each declaration. A fourth mutation replaces a comment by `//`. A prototype failed on it. | — |
| threat-r2 01 | B | C11 | G1 | fixed another way | Ruling 1. The ordinary acknowledgement is already bounded by `sent` in the client-local plan's `Ack`. | architect-r2 01 |
| threat-r2 02 | B | C10 | G2 | fixed another way | Ruling 3. Token checks of a waiting machine are refused in the contract, and the rest of the fence is in the request. | architect-r2 03 |
| threat-r2 03 | B | C10 | G13 | fixed another way | Ruling 2. A removed client stopped with status 0 and does not dial again, so a restored record admits nothing by itself. The issue is requested in the result file. | — |
| threat-r2 04 | M | C9 | G5 | for the owner | A bad proof and a bare status now retry. A forged `failed_precondition` on the hop behind the web server can still stop a machine, which only point 2 for the owner can settle. That point now names this cost. | architect-r2 06 |
| threat-r2 05 | M | C11 | G1 | fixed another way | Ruling 1. | architect-r2 02 |
| threat-r2 06 | M | D7 | G7 | fixed | A waiting machine gets no command but `SetState`, its results are ignored, and its connection token opens no later call until approval. | architect-r2 08 |
| threat-r2 07 | M | R6 | G6 | fixed | As architect-r2 07. | architect-r2 07 |
| threat-r2 08 | M | C6 | G8 | fixed | As architect-r2 09 and 10. | architect-r2 10 |
| threat-r2 09 | M | C5 | G9 | fixed | As architect-r2 11. `Join` refuses with one word, and the words include `token_used` and `token_expired`, which check C5 reads. | architect-r2 11 |
| threat-r2 10 | M | R9 | G15 | fixed | `CommandResult.message` is removed and reserved, and the server writes the website's text from the outcome. Ruling 4: no wave-1 check reads text from a machine. | — |
| threat-r2 11 | m | R9 | G10 | fixed | As architect-r2 12. | architect-r2 12 |

The reports also carry notes that are not numbered findings.
The plan folds the cheap ones: the singular cardinality of `position`, a `Welcome` without a state, the README's second session rule and its exceptions, the summary of `Join invalid_argument`, and the description of `grep -q`.

### Counts

The four reports hold 43 findings: 13 blockers, 26 majors and 4 minors.

| Outcome | Findings |
|---|---|
| fixed | 27 |
| fixed another way | 15 |
| rejected | 0 |
| post-build check | 0 |
| for the owner | 1 |
| not folded | 0 |

### Mechanisms with two or more blockers or majors

- **G1, the numbering of events** (6 blockers, 2 majors). The mechanism was wrong, not the patch. Ruling 1 replaces it with the design's three lines.
- **G2, the order of removal and stream writes** (3 blockers). The mechanism belongs to the server's design. Ruling 3 moves it to `go-s3-server-machines`.
- **G13, the restore of the database** (1 blocker, 2 majors). Ruling 2 puts it outside wave 1.
- **G5, the codes that make a client stop** (3 majors). The mechanism of round 1 was wrong in one place: `unauthenticated` mixed a removal with a bad proof. It now has one stop code. The rest of the risk is the transport, which is point 2 for the owner.
- **G7, approval and presence** (4 majors). Two rules contradicted each other. One projection replaces both.
- **G8, the token's address** (4 majors). The one-address rule was wrong for the installer. Two issuance contexts and a reply field replace it.
- **G6, the clock of a command** (2 majors, 2 minors). The mechanism stays, because a server-only expiry cannot stop a late arrival on a slow stream. Its promise is restated as "never late", and the receipts share it.
- **G9 and G10** have two majors each, in one rule each. The rules are corrected in place.

## 2. Points per statement

A group counts once, at its highest severity: a minor is 1 point, a major 3 and a blocker 9.

| Statement | Groups | Points |
|---|---|---|
| C11 | G1 (9), G11 (3) | 12 |
| C10 | G5 (3), G13 (9) | 12 |
| plan | G4 (9), G14 (1) | 10 |
| D7 | G2 (9) | 9 |
| D17 | G3 (9) | 9 |
| R9 | G10 (3), G15 (3) | 6 |
| R6 | G6 (3) | 3 |
| C7 | G7 (3) | 3 |
| C6 | G8 (3) | 3 |
| C5 | G9 (3) | 3 |
| D16 | G12 (3) | 3 |

The round's total is 73 points.

## 3. Stuck statements

None.
A statement is stuck at 12 points and 30% of the round, which is 21.9 points here.
C11 and C10 each have 12 points, which is 16% of the round.

## 4. Patches by heading

Round 3 attacks only these places.

1. `plan.md`, "Key decisions": the events, the restore, commands, removal, the current stream, the projection, the dialog's address and restart.
2. `plan.md`, "Points for the owner": the deadline of points 1 and 2, and the added cost of point 2, option A.
3. `plan.md`, "Public names that other teams build against": the rows of the event number, `CommandResult.message`, `CreateClientTokenResponse.address`, the codes, the value rules, the token text and the request key.
4. `plan.md`, "The description format", rule 7: the reserved lines of `commands.txt`.
5. `plan.md`, item 2: `pnpm exec` for `buf`, the install and removal of the checkout, and the gate before item 7.
6. `plan.md`, item 3: the field `address`, the rules of `CreateClientToken` and its reply, the single `position`, and `TestTokenReplyMatchesItsDescription`.
7. `plan.md`, item 4: the clock of commands and receipts, `Restart`, `CommandResult` without text, the standing, the rules of events, and the projection.
8. `plan.md`, item 5: the order of refusal, the claim, the refusal words, the retry limit and the two issuance contexts.
9. `plan.md`, item 6: the fourth mutation, the handshake, the codes, a waiting machine, the current stream, removal, the event marks, the golden `Hello` and `TestEveryLinkRuleIsWritten`.
10. `plan.md`, item 8 and "Done when", rows 6 and 8.
11. `plan.md`, "Risks": the clock, the stop, the restore, the race and the owner's deadline.
12. `plan.md`, "Agreements with other teams": every row changed or added in round 2.
13. `plan.md`, "Post-build checks": checks 3 and 5, and the new checks 10 to 15.
14. `plan.md`, "Traceability" and "Attack log".
15. `draft/link.proto`, `draft/events.proto`, `draft/commands.proto` and `draft/existing-files.diff`.
16. `decisions.md`, rows 39, 42 and 44 amended, and rows 54 to 62.
17. `scouting.md`, the corrected anchor of the client-local plan, and runs 22 to 26.

## 5. The top finding

`Hello.acked_event_number` let a server move its mark to a number that the machine claimed, so it could acknowledge reports it never applied, and the machine then deleted its only copies.
It was a blocker, and all four reviewers found it on their own (architect-r2 01, logic-r2 02, logic2-r2 1, threat-r2 01).
By ruling 1, the field is gone, and only the server's own committed number decides.

## 6. Each reviewer's verdict and grade

| Reviewer | Verdict | Grade | Counts |
|---|---|---|---|
| architect | REVISE | 4 | 5 blockers, 7 majors, 0 minors |
| logic | REVISE | 4 | 4 blockers, 8 majors, 0 minors |
| logic2 | REVISE | 5 | 1 blocker, 4 majors, 3 minors |
| threat | REVISE | 4 | 3 blockers, 7 majors, 1 minor |
