# Client-link round 3 findings

The two reports contain four majors and six minors. Nine distinct findings carry 17 points. D, R and C refer to design decisions, design rules and wave 1 checks, in source order.

## Findings and outcomes

| Finding | Severity | Statement | Outcome | Reason |
|---|---|---|---|---|
| logic-r3 1 | major | D16 | fixed another way | Retry all store-open failures. `storeOpenError` groups damage and access faults, so the service needs no new error type or classifier. |
| architect-r3 1 | major | D16 | post-build check | The installer already points to the journal. Verify that its existing local fault and verified-removal diagnostics identify which stop occurred. |
| architect-r3 3 | minor | D16 | fixed | Zero is invalid. Item 8 logs it and exits 1. |
| architect-r3 5 | minor | D16 | fixed | Any otherwise unclassified store failure returns an error. The test includes a lock conflict and a read failure. |
| architect-r3 2 | major | R6 | fixed another way | Read existing receipts through `Begin` at expiry while the slot is busy. Close only for a distinct scan that would start. This uses the existing retry path without a queue. |
| logic-r3 4 | minor | R6 | fixed | Repeats architect-r3 2. The equality boundary provides the requested read-only lookup, with finished, interrupted and expired cases tested on an open stream. |
| logic-r3 5 | minor | R6 | fixed another way | Restart cancels and settles the scan using its current cancellation rule, then completes the restart. |
| logic-r3 2 | major | C11 | fixed | Advance the ceiling at handoff to the sole sender. A frozen session test holds the send return and delivers the acknowledgement. |
| logic-r3 3 | minor | C7 | fixed | The held-scan test waits for newer inventory, expects failed on the latest attempt and checks that a later scan gets done. Refused requested reports leave no pending snapshot. |
| architect-r3 4 | minor | R9 | fixed another way | Keep the receive cap and planned test fixture without a dependency on an unwritten comment or a shared server constant. |

## Points and stuck statements

Duplicate groups count once at their highest severity. Each group scores against its primary statement.

| Statement | Points |
|---|---:|
| D16 | 8 |
| R6 | 4 |
| C11 | 3 |
| C7 | 1 |
| R9 | 1 |
| Total | 17 |

No statement is stuck. D16 has about 47 percent of the points but falls below the 12-point minimum.

## Patches for round 4

- **Item 2** keeps the existing receive cap and sizes its fixture from the current planned producer. This is a minor fix.
- **Items 4 and 7b** move the acknowledgement ceiling to handoff, remove send-completion bookkeeping and move the racing acknowledgement check into the frozen session tests.
- **Items 5 and 6** answer receipt-only cases while a scan holds the slot, use the existing retry for a distinct runnable scan and settle an active scan on restart. The duplicate slot findings identify an admission check that was too broad. The plan reuses `Begin`'s existing read-only path.
- **Items 7b and 8 and Combined acceptance and post-build checks** replace the open-error split and test the exit pairs, installed recovery and journal guidance. Store-open retry and stop guidance are distinct. The built classification cannot support a permanent stop on an open error, and the service log already provides an operator-facing source. The same checks name the expected scan outcome across reconnect.
- **Key decisions and Attack log** retain the decision log link and record the round. The decision log and scouting reflect the changed boundaries.

Round 4 reads blockers and majors in the changed text.

## Top finding and verdicts

The top finding is major logic-r3 1. An unreadable store can return the same error as structural damage, so the former exit policy could suppress recovery after file access returned. The architect's top major concerns how the operator identifies a local stop from the installed service.

| Reviewer | Verdict | Grade out of 10 |
|---|---|---:|
| Architect | REVISE | 6 |
| Logic | REVISE | 6.5 |
