# Client-link round 1 findings

The five reports contain 2 blockers, 21 majors and 19 minors. The 22 distinct findings carry 58 points after duplicates are counted once. D, R and C refer to design decisions, design rules and wave 1 checks, in source order.

## Findings and outcomes

| Finding | Severity | Statement | Outcome | Reason |
|---|---|---|---|---|
| logic2-r1 01 | blocker | R6 | fixed another way | A fast machine clock can prune a live receipt. Ruling 1 accepts this limit. |
| critic-r1 3 | major | R6 | fixed another way | Repeats logic2-r1 01. Also reports the false start covered under architect-r1 01. |
| logic-r1 4 | minor | R6 | fixed another way | Repeats logic2-r1 01. |
| threat-r1 5 | minor | R6 | fixed another way | Repeats logic2-r1 01. |
| logic2-r1 02 | blocker | R2 | fixed another way | Ruling 2 puts contract parity with the server owner. The client defaults to retry for unspecified codes. |
| architect-r1 03 | major | R2 | fixed another way | Repeats logic2-r1 02. Also adds the explicit retry default and test. |
| architect-r1 01 | major | R6 | fixed | Item 6 uses one atomic finished write for a known decline, pending the Client Local interface agreement. |
| logic-r1 1 | major | R6 | fixed | Repeats architect-r1 01. |
| logic2-r1 03 | major | R6 | fixed | Repeats architect-r1 01. A previously committed normal start still recovers conservatively. |
| critic-r1 2 | major | C10 | fixed | Item 7b names and tests both log fields. It retries a mismatch without sending proof. |
| architect-r1 02 | major | C10 | fixed | Repeats critic-r1 2. |
| critic-r1 1 | major | C10 | fixed | Rollout step 3 gates stack acceptance on the explicit state directory and identity name. |
| logic2-r1 05 | major | C10 | fixed | Repeats critic-r1 1. The exact log fields are covered under critic-r1 2. |
| architect-r1 08 | minor | C10 | fixed | Repeats critic-r1 1. |
| logic2-r1 04 | major | D16 | fixed | Item 7a caps elapsed time between disconnected attempt starts and tests blocked dialing and proof. |
| logic-r1 3 | major | D16 | fixed | Repeats logic2-r1 04. |
| critic-r1 4 | major | D16 | fixed | Repeats logic2-r1 04. |
| architect-r1 04 | major | R10 | fixed | The orchestrator owns the guarded combined suite. Named scenarios have expected results and the Stack soak has a command. |
| critic-r1 7 | major | R10 | fixed | Repeats architect-r1 04. |
| critic-r1 5 | major | D16 | fixed another way | Item 7b keeps status 0 for irreparable local stops under the service policy and requires a visible error. Missing metadata retries. |
| architect-r1 05 | minor | D16 | fixed another way | Repeats critic-r1 5. |
| architect-r1 06 | minor | plan | fixed | Item 3 names the network module requirement and its own direct pin. The guardrails name all outside owners. |
| critic-r1 11 | minor | plan | fixed | Repeats architect-r1 06. |
| logic2-r1 09 | minor | C6 | fixed | Join explains the first retry, existing identity and expired replay recovery for waiting or approved machines. |
| architect-r1 07 | minor | C6 | fixed | Repeats logic2-r1 09. |
| critic-r1 8 | minor | C6 | fixed | Repeats logic2-r1 09. |
| logic-r1 2 | major | C11 | rejected | C11 stops only the server and suspends then resumes the same client process in deploy/test/checks.sh:550. Its producer is covered. |
| logic-r1 5 | minor | D16 | rejected | `src/contracts/proto/lgtm/link/v1/events.proto:21` ends only one stream on a gap. Issues 1641 and 1642 exclude restored state and copies. |
| logic-r1 6 | minor | C11 | rejected | `src/contracts/proto/lgtm/link/v1/events.proto:8` preserves numbered reports. The design bounds the queue, and `deploy/test/checks.sh:560` needs snapshots even if facts are unchanged. |
| critic-r1 10 | minor | plan | fixed | The stale rows point to their successors. Scouting describes the current decision log. |
| logic-r1 7 | minor | plan | fixed | Repeats critic-r1 10. |
| threat-r1 6 | minor | plan | fixed | Repeats critic-r1 10. The owner gate is also tracked under logic2-r1 08. |
| logic2-r1 08 | major | D17 | for the owner | Issue 1633 remains open. The result gives the private-hop, pass-through and authenticated-message options and gates the freeze. |
| critic-r1 6 | major | D17 | for the owner | Repeats logic2-r1 08. |
| threat-r1 1 | major | D17 | for the owner | Repeats logic2-r1 08. The new pin-mismatch diagnostic gives fresh-enrollment recovery. |
| logic2-r1 06 | major | C7 | fixed | Item 5 orders captures before scan completion and drops older unnumbered inventory after a newer commit. |
| logic2-r1 07 | major | R6 | fixed | Item 6 tests the start commit in both the delayed and failed cases for scans and restart. |
| critic-r1 9 | minor | plan | fixed | Split retry scheduling and session integration, and give the global build order. |
| critic-r1 12 | minor | D16 | rejected | `src/contracts/proto/lgtm/link/v1/events.proto:30` requires fresh facts and inventory on an approved welcome. Server readiness belongs to the current stream. |
| threat-r1 2 | minor | R9 | fixed another way | One 64-kibibyte client cap matches the peer and applies before and after proof. Each path has a refusal test. |
| threat-r1 3 | minor | R9 | fixed | A loopback signer mismatch prompts token revocation and a fixed diagnostic test. |
| threat-r1 4 | minor | D7 | for the owner | The positional form is approved. The result gives the standard-input options and their costs. |

## Points and stuck statements

The statement column names the primary statement used for scoring. A finding with several effects contributes once to that primary statement.

| Statement | Points |
|---|---|
| R6 | 15 |
| R2 | 9 |
| D16 | 8 |
| C10 | 6 |
| C11 | 4 |
| C7 | 3 |
| D17 | 3 |
| plan | 3 |
| R10 | 3 |
| R9 | 2 |
| C6 | 1 |
| D7 | 1 |
| Total | 58 |

No statement is stuck. R6 has 15 points, about 26 percent of the round. The threshold is at least 12 points and 30 percent.

## Patches for round 2

- **Key decisions and Rollout order** name the owner gates, the retention limit, the current server contract and the Stack fixture prerequisite.
- **Shared build context and Combined acceptance and post-build checks** give the test owner, database guard, scenarios and expected results.
- **Items 1 to 3** gate trust-dependent tests, test bounded replies, log loopback token exposure, state the direct clock-module pin and classify unknown refusals as retryable.
- **Items 4 and 5** keep numbered events fixed and serialize snapshot freshness before append.
- **Item 6** replaces the false-start decline with an atomic finished receipt and tests the start-commit boundary. The repeated receipt findings identify two independent defects. Fixing the decline path does not change the accepted retention clock.
- **Items 7a and 7b** divide retry scheduling from session wiring, bound stalled attempts, and specify and test stop diagnostics. The cadence findings identify a limit on the wrong interval, which the attempt deadline replaces.
- **Items 8 and 9 and the decision log** give join-recovery diagnostics, the command-test gate and pointers from superseded rows.

## Top finding and verdicts

The top finding is blocker logic2-r1 01. A fast wall clock can erase a completed restart receipt while the server may still retry the command. The round's ruling accepts that limit. Three other reports corroborate the issue.

| Reviewer | Verdict | Grade out of 10 |
|---|---|---|
| Architect | REVISE | 6 |
| Critic | REVISE | 5 |
| Logic | REVISE | 6 |
| Second logic | REVISE | 5 |
| Threat | REVISE | 7 |
