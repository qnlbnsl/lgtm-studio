# Client link round 4 findings

## Not reviewed before the build

- **Item 7b. Session and reporting.** The client writes the two local stop codes. Its tests require one exact diagnostic for exhaustion and for an impossible welcome or live acknowledgement.
- **Item 8. Shared application lifecycle.** The app test pairs each diagnostic with status 0.
- **Workstream 2 risks.** The acknowledgement limit now says handoff, as item 4 already specifies.
- **Item 6. Standing and command receipts.** The busy scan tests distinguish replaying saved expiry from saving a new expiry.
- **Combined acceptance and post-build checks.** Check the installed journal, busy scan expiry and immediate acknowledgement. No further plan review precedes the build.

## Findings and outcomes

Decision 16 (D16) covers the outbound link, reconnect and restart.

| Finding | Severity | Statement | Outcome | Evidence and reason |
|---|---|---|---|---|
| logic-r4 1 | Major | D16 | fixed | The built store distinguishes `ErrSequenceExhausted` and `ErrAckAhead` in `src/client/internal/link/outbox.go:29` and `:84`. Item 7b logs one exact local stop record. Item 8 and the installed check read it. |

There are no duplicate findings. This resolves the client side of architect-r3 1, which round 3 deferred to installed verification. There is one major in the stop path. The defect was the unnamed diagnostic, so the existing exit mapping and log path suffice. The Stack Client installer text and live journal assertion are requested in the result file under that team's ownership.

## Points and stuck statements

| Statement | Distinct findings | Points |
|---|---|---:|
| D16 | One major | 3 |
| Total | One finding | 3 |

No statement is stuck. D16 has all three points, below the threshold of 12.

## Patch headings

The five headings in “Not reviewed before the build” identify the fixes and their checks. The two uncounted review notes are the handoff wording and the busy scan expiry test. Shared build context was tightened to make room, and the Attack log records the fold.

## Top finding and verdict

The top finding is logic-r4 1, a major. A permanent local fault could exit with status 0 and leave no identifiable diagnostic.

| Review | Verdict | Grade |
|---|---|---|
| Logic | Revise | 7.5 out of 10 |
