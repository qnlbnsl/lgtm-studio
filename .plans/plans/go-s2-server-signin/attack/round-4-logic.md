# Round 4 report of the logic reviewer for the sign-in plan

- Task identifier: review-go-s2-server-signin-r4-logic
- Nonce: ea41eb
- Lens: logic and failure policy
- Scope: the diff between commits 9d87abd0 and 7d5db4c6 in `.plans/plans/go-s2-server-signin/plan.md` and `.plans/plans/go-s2-server-signin/decisions.md`, and what follows from it.

## Verdict

SHIP

## Top finding

I found no blocker and no major in the round 3 patches, so there is no top finding.

## Findings

There are none at blocker or major severity. Round 4 does not report minors.

## What I checked

- I read the diff in full, the register in `attack/round-3-findings.md`, the round 3 logic report, and items 2 to 9 of the plan.
- I read `src/server/internal/store/pool.go` for `sanitizeError`, `Open`, `configureConnection`, `withWrite` and `withTransaction`.
- I read `src/server/internal/store/storetest/postgres.go` and `src/server/internal/store/storetest/migrated/store.go` for the schema and search path of the shared fixture.
- I read `src/server/internal/store/prune.go` for its uses of `clock_timestamp()`.
- I checked the pinned Connect library at version 1.21.1. Both overrun paths in `envelope.go` and `compression.go` contain the text "larger than configured max" followed by the number, so the fragment `configured max 32768` is real for the complete and the interrupted overrun.
- I checked that `docs/issues.md` holds issue 1641, which the plan cites for the restore case.
- I checked that `store.NewDomainError` exists at `src/server/internal/store/pool.go:80` and that `sanitizeError` passes a directly returned domain error through unchanged. A sentinel wrapped by another error would be lost, and the plan returns both sentinels directly from the write callback, so the plan and the Store agree.

## Answers to the attack questions

1. Patch effect. Every patch in the register removes the failure it describes as written, and none breaks another statement.
   - logic-r3 1: `ErrSessionClock` is declared in item 4, is mapped to `aborted` in the item 6 table, and is driven through the real Store by `TestSessionClockRefusal`. The refusal fires only when the newest surviving row equals the sample plus 60 seconds, so the test is deterministic.
   - logic-r3 2: Issue never touches the cache, and the derivation slots are independent, so no wait cycle exists between the cache mutex and a slot.
   - logic-r3 3: `ClockStore` migrates on the original search path and then reopens with `clockfix, pg_catalog`, which is required because `storetest.NewDatabase` sets only the quoted schema and leaves `pg_catalog` implicitly first.
   - logic-r3 5: The admission deadline, the parent cancellation and the driver failure are three distinct outcomes, and the socket is closed on each failure.
   - logic-r3 6: The relay selected by `application_name` exercises the real transport.
   - logic-r3 4 and logic-r3 7: Both are documentation patches and are consistent with the unchanged text.
2. Unreviewed mechanisms.
   - The admission lock with its deadline split is sound. Its small gaps are minors: a deadline that expires during the dial and a lock loss that prints the startup canceled sentence.
   - The single longer probe is sound. It does not retry a timed-out driver connection, so it cannot double a lock holder.
   - The `ClockStore` shadow clock is sound, with one post-build note below.
   - The rule that rejects a cross-site entry unless both origins use HTTPS, the plaintext loopback aliases, and the cookie mode names are unchanged text and were not reopened.
   - The public pin recovery by restoring the matching seed is sound. The pin is never rewritten on a failure, so recovery cannot change machine trust.
3. Rejected findings. The register of round 3 has no finding with the outcome rejected, so there is no evidence to check.
4. Writers and readers. The names that changed are `ErrSessionClock`, `authtest.ClockStore`, `AcquireServerInstance`, `ErrServerInstanceBusy`, `InstanceLock.Check`, `InstanceLock.Close`, `phaseBudgets` and `start`. A repository search found them only in this plan, its history and two earlier plans that mention the method without a conflicting signature. Item 4 writes the first two, item 9 writes the rest, and items 6, 7 and 9 read them. The list inside the fix is complete.
5. Contradictions and truth tables.
   - The startup diagnostic table has no empty cell. The order is detail line, reconciliation line when applicable, one outcome line, then the password line.
   - The error table in item 6 has no overlap. The two `aborted` rows differ by sentinel, and the plan says auth's direct sentinels are mapped before Store errors.
   - The three outcomes of `AcquireServerInstance` (acquired, busy, other error with the parent context error taking precedence) are exclusive and each has one exit.
6. Open blockers and majors. None. Both round 3 majors, logic-r3 1 and logic-r3 2, are closed by their patches.
7. Lens questions touched by the patched text.

| Lens question | Answer |
|---|---|
| 1. Ordering and ties | Session order is `created_at DESC, verifier DESC`, and a tie is broken by the verifier. The exact skew boundary has a sentinel and a test. Restart keeps the pin and the rows. |
| 2. Failure permits or denies | Clock refusal denies and leaves state unchanged. Lock loss cancels serving. A failed lock probe closes readiness. A failed first pin commit permits adoption of the same seed on restart. |
| 3. Truth tables | Built for the diagnostics, the error mapping and the lock outcomes. No empty or contradictory cell. |
| 4. Zero values | A missing password is retained as absent and reconciled. A zero wait is not given a meaning by the plan, which passes the admission budget from `phaseBudgets`. |
| 5. Two actors on one state | `LOCK TABLE web_sessions IN SHARE ROW EXCLUSIVE MODE` serializes writes. The advisory lock serializes serving instances. The cache mutex is short and never held across derivation. |
| 6. Risky item mutations | The named mutations fail the named tests. The clock mutation fails `TestSessionsUseDatabaseClock` and `TestSessionClockRefusal`. |
| 7. Cheapest wrong implementation | Nothing found that passes every listed test while being wrong. The nearest is a Store that ignores the unknown-outcome case, and `TestSessionContract` kills it with a terminated backend. |
| 8. A shape that makes the wrong state impossible | The one-row `server_link_identity` table with a boolean primary key makes a second pin impossible. The sentinel as a directly returned domain error is the only shape that survives `sanitizeError`, and the plan uses it. |

## Post-build checks

- Confirm that the interrupted overrun test still receives a response when the client aborts its request. If the client closes first, the test may see a transport error instead of the Connect code, and the shared fragment would not appear.
- Confirm that `ClockStore`'s shadow `clock_timestamp()` does not reach the pruner. `src/server/internal/store/prune.go` calls the unqualified function in three places, so any test that starts the pruner on a shadowed Store would see the fixture clock.
- Confirm that the `TestStartMonitorsLockProbeBudget` relay propagates a client close to PostgreSQL, so that the advisory lock is released when the app closes the socket.
- Confirm that `CheckInRead` does not hold the caller's read transaction across a cold derivation for longer than the 30-second idle-in-transaction limit of pooled connections.
- Confirm that the module row for `golang.org/x/crypto v0.59.0` is consistent with the `golang.org/x/net` version that item 5 adds, using the second tidy run the plan already requires.

## Grade

8 out of 10. Counts: 0 blockers, 0 majors, 0 minors reported in this round.
