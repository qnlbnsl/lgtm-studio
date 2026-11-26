# Client-link round 2 findings

The two reports contain six majors and eight minors. Ten distinct findings carry 18 points. D, R and C refer to design decisions, design rules and wave 1 checks, in source order.

## Findings and outcomes

| Finding | Severity | Statement | Outcome | Reason |
|---|---|---|---|---|
| architect-r2 1 | major | R6 | fixed | Item 6 owns the store and handler changes, defines the receipt cases and freezes its own tests. Client Local row 84 already supersedes its historical clock field. |
| logic-r2 1 | major | R6 | fixed | Repeats architect-r2 1. The tests cover all receipt decisions, inputs and replay after reopening. |
| architect-r2 2 | major | R6 | fixed | Reserve the scan slot before writing a receipt. A busy distinct scan retries on a new stream with its receipt untouched. |
| logic-r2 5 | minor | R6 | fixed | Decline prunes in the same write and propagates a failed commit without sending a result. |
| architect-r2 5 | minor | R6 | fixed | Document when pruning changes the outcome and when it permits repeat work. Test both offsets with the existing clocks. |
| logic-r2 4 | minor | R6 | fixed | Repeats architect-r2 5. The accepted clock limit is stated in the overview, item 6 and decision 49. |
| architect-r2 3 | major | D16 | fixed another way | Retry missing and unsafe files under the existing service policy. Stop on irreparable faults. Stack must explain both classes. |
| logic-r2 3 | major | D16 | fixed another way | Repeats architect-r2 3. Items 7b and 8 use the existing return pair and status codes. The checks cover both stop and recovery. |
| architect-r2 4 | minor | D16 | fixed | Repeats architect-r2 3. The link tests assert each return pair and the application tests assert the status. |
| architect-r2 8 | minor | D16 | fixed | Item 4 owns the replay methods. Item 7b owns and tests priority between event sends. |
| logic-r2 2 | major | C7 | fixed another way | Record failed when a newer inventory supersedes a requested scan. `src/contracts/proto/lgtm/link/v1/commands.proto:88` requires that scan's own event for done. |
| logic-r2 6 | minor | C10 | post-build check | Keep the Stack rollout gate. Run C10 after Stack lands and check the private identity and both timed refusal diagnostics. |
| architect-r2 6 | minor | R9 | fixed | Budget receive size from the largest planned server reply and ask Contract to state the producer's field bounds. |
| architect-r2 7 | minor | plan | fixed | References name 7a for reconnect scheduling and 7b for work on the stream. Item 6 and 7a run beside each other. |

## Points and stuck statements

Duplicate groups count once, at the highest reported severity. A finding with several effects contributes once to its primary statement.

| Statement | Points |
|---|---:|
| R6 | 8 |
| D16 | 4 |
| C7 | 3 |
| C10 | 1 |
| R9 | 1 |
| plan | 1 |
| Total | 18 |

No statement is stuck. R6 accounts for about 44 percent, with eight points against the 12-point minimum.

## Patches for round 3

- **Overview, Key decisions and Rollout order** explain the clock limits and place the decline change inside the client-link build.
- **Items 2 and 3** justify the independent receive cap from the planned response bounds and name its tests.
- **Items 4 and 7b** specify the replay interface and give the live session ownership of send completion, acknowledgement ordering and frame priority.
- **Items 5 and 6** give a superseded requested scan a durable failed result. The two receipt-admission majors expose a flaw in leaving admission to two independent checks. One process owner reserves the slot before the start write. The same build item adds the atomic decline write to the store and tests the failure boundary.
- **Items 7b and 8 and Combined acceptance and post-build checks** settle the return pair, status mapping, installed service behavior and C10 fixture check. The earlier single policy for all local faults could suppress recovery. The new mapping retries faults that can clear when access returns.
- **Traceability, Execution sizing and Attack log** name the split session items, the receipt writer and this round. The decision log and scouting point to the new boundaries.

## Top finding and verdicts

The top finding is major architect-r2 1, independently reported as logic-r2 1. Item 6 depended on a store change without assigning any builder to it. Item 6 now owns the source and tests required for that change.

| Reviewer | Verdict | Grade out of 10 |
|---|---|---:|
| Architect | REVISE | 6 |
| Logic | REVISE | 6 |
