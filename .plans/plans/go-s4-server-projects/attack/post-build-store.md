# SHIP

The frozen Store tests accept a regression that discards a retained replay callback error and commits the callback’s changes.

The test writer and builder were OpenAI workers, the same vendor as this reviewer. I wrote neither the implementation nor the frozen tests. I reviewed Projects item 9 and Link item 0 on `sprint-0` at `d71f69c2`, using `3548b0ff~1..d71f69c2` for the Store diff.

## post-store 1 · MINOR · Projects item 9 and foundation check 12

The frozen tests do not require a matching retained replay to surface its callback error and roll back any callback changes. The implementation currently propagates the error. The tests exercise successful callbacks, key conflicts, and injected failures in admission and retirement. [Replay implementation](../../../../src/server/internal/store/receipts.go#L111) · [Successful replay test](../../../../src/server/internal/store/held_receipts_test.go#L207) · [Lost commit reply cases](../../../../src/server/internal/store/held_receipts_test.go#L414) · [Projects transaction contract](../plan.md#L83) · [Foundation check 12](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L389)

**Failure scenario.** A retry finds its stored key. The callback records a change and then returns a domain error. In the mutation, Store discards the error, returns the saved operation identifier as a success and commits the callback’s change. The full Store package accepted this mutation with 60 top-level tests and 138 subtests passing. A separate PostgreSQL probe passed against the implementation. It failed against the mutation because the replay returned the original identifier with no error and the database contained two domain rows where only the original should remain.

**Fix.** Add the probe to the frozen receipt tests in a follow-up. Require the callback error, a nil reply, the original operation and receipt still present, and rollback of both a sentinel row and its feed reference.

## Store checks from the post-build plan

The plan and the latest Projects register mark the repository refresh as unreviewed under Projects items 6 and 7. The Link plan points to that same mechanism from its removal path. Neither assigned Store item has an unreviewed marker. The Projects post-build table assigns Store a share of the upgrade and uncertain-commit checks. [Projects register](round-1-findings.md#L10) · [Link removal plan](../../go-s4-server-link/plan.md#L238) · [Post-build checks](../plan.md#L279)

- **Sprint 3 Store schema upgraded in place.** Confirmed live. The frozen test applied the unchanged version 1 migration, seeded an ordinary Machines receipt and interrupted startup with a later part’s migration error. The ledger stayed at version 1 and the receipt was unchanged. Two successful migrations left the ledger at version 2, preserved every original receipt column and assigned `held = false`. Inserts using the old columns also defaulted to `held = false`. The same key replayed identical bytes without another write callback. It still replayed at the original cutoff, and pruning removed it one microsecond later. The version 1 manifest refused the upgraded ledger. In a supplemental probe I canceled a blocked startup after the Store migration ran, reopened the same database and retried. The ledger advanced from version 1 to version 2 on retry, and the saved reply was `00f0ff` both before and after the upgrade. [Upgrade assertions](../../../../src/server/internal/store/held_receipts_test.go#L41) · [Additive migration](../../../../src/server/internal/store/migrations/0002_held_receipts.sql#L1)
- **Uncertain commits and replay ordering.** Confirmed live. All four lost-commit-reply cases returned `OutcomeUnknown` and were checked after reopening the same database. Committed admission replayed the original operation identifier. Rolled-back admission permitted a fresh start. Committed retirement left neither row, while rolled-back retirement preserved both rows and could be retried. When replay won the lock it read the original operation before retirement. When retirement won, the same key admitted a replacement. [Lost commit reply assertions](../../../../src/server/internal/store/held_receipts_test.go#L544) · [Both lock orders](../../../../src/server/internal/store/held_receipts_test.go#L431)
- **Held receipt lifetime.** Confirmed live at the Store boundary. Ordinary pruning neither stamped a held receipt nor deleted it with an expired stamp. Retirement passed over mismatched bytes and ordinary receipts, and a callback could decline removal. When an eligible callback removed the operation, Store retired both rows in the same commit and freed the key. A fault rolled back both deletions. Projects item 3 owns the maturity cutoff and the recurring call to `RetireReceipt`. Its live scheduling, live-session survival, mounted discovery and the real client’s inspection count were outside this Store run. [Prune and retirement assertions](../../../../src/server/internal/store/held_receipts_test.go#L326) · [Projects retirement scheduling](../plan.md#L131)

## The built Store matches both items

- **Projects item 9.** By reading, ordinary and retained admission share a caller-and-key advisory lock, a receipt row lock and a fingerprint check. A successful new retained admission saves the operation identifier under the hold. Replay runs under the receipt lock. Retirement requires matching held bytes before it calls the removal callback, and both changes share Store’s transaction and commit classification. Confirmed live, both winning orders for ordinary and retained admission ran one fresh callback and returned a key conflict for the competing procedure. The test-writer baseline matches the reviewed commit for the frozen tests and version 1 migration. [Projects item 9](../plan.md#L81) · [Admission and replay](../../../../src/server/internal/store/receipts.go#L76) · [Retirement](../../../../src/server/internal/store/receipts.go#L157) · [Transaction cleanup and outcomes](../../../../src/server/internal/store/pool.go#L320)
- **Link item 0.** By reading, `RecordOnce` returns the sticky error first, compares the full change reference and delegates a new reference to `Record`. Feed positions still follow commit order under the existing lock. Confirmed live, the exact repeat consumed no slot at 256 references. A difference in any field rolled back the sentinel row and feed. A validation error stayed sticky, and a fresh transaction recorded the reference again. [Link item 0](../../go-s4-server-link/plan.md#L78) · [Recording and coalescing](../../../../src/server/internal/store/feed.go#L71) · [Feed lock](../../../../src/server/internal/store/feed.go#L143) · [Limit and rollback assertions](../../../../src/server/internal/store/feed_test.go#L448)

## All six plan mutations failed at their intended assertion

Each mutation ran independently against PostgreSQL in the same disposable checkout. Each command exited 1.

| Mutation | Observed failure |
|---|---|
| Check capacity before coalescing. | The exact repeat at capacity returned the overflow error. [Assertion](../../../../src/server/internal/store/feed_test.go#L469) |
| Ignore a sticky error for an exact repeat. | The repeat returned no error after the invalid reference. [Assertion](../../../../src/server/internal/store/feed_test.go#L532) |
| Bypass the caller-and-key lock on retained admission. | The competitor finished while the first callback held the key in both winning orders. [Assertion](../../../../src/server/internal/store/held_receipts_test.go#L269) |
| Let ordinary pruning stamp a held receipt. | The held receipt received an observation stamp. [Assertion](../../../../src/server/internal/store/held_receipts_test.go#L350) |
| Let ordinary pruning delete an expired held receipt. | Pruning deleted two receipts instead of one. [Assertion](../../../../src/server/internal/store/held_receipts_test.go#L380) |
| Commit operation removal separately from receipt deletion. | A receipt deletion fault left the receipt present and the operation missing. [Assertion](../../../../src/server/internal/store/held_receipts_test.go#L401) |

## PostgreSQL test output

This command ran the Store portion of the Projects plan’s test filter and both feed tests named by Link item 0. All 23 top-level tests and 66 subtests passed with zero skips.

```sh
bash deploy/test/with-postgres.sh -- go test -v -p 1 ./src/server/internal/store \
  -run '^Test(HeldReceipt|Migrate|Prune|WriteOnce|Record(Once)?ValidationAndLimit)' -count=1
```

```text
PASS
ok  github.com/qnlbnsl/lgtm-studio/src/server/internal/store  58.194s
```

The race detector run of the three `TestHeldReceipt` cases and both feed tests printed `PASS`, at `10.178s`, with zero skips or race reports. The full Store suite with the replay-error mutation also printed `PASS`, at `133.015s`. The supplemental replay-error and canceled-migration probes printed `PASS` on the original Store, at `0.962s`. The supplemental replay-error probe printed `FAIL` with the mutation, at `0.343s`.

## Grade

9 out of 10.
