# Sign-in plan findings from attack round 3

Round 3 assessed the round 2 patches. The plan remains in attack.

## Findings and outcomes

| Finding | Severity | Statement | Outcome | Reason |
|---|---|---|---|---|
| architect-r3 1 | minor | C4, R10 | fixed | The app test gates real replies on the dedicated socket. It covers both a recoverable delay and a stalled probe that closes readiness and releases the lock. |
| architect-r3 2 | minor | D7, C13 | fixed another way | The vault wire fixture rejects any writing method or POST outside the allowed initial creation. The result requests the Stack-owned permission issue under the bounded write scope. |
| architect-r3 3 | minor | R8 | fixed | Items 6 and 7 test the resource_exhausted code and the maximum fragment shared by both pinned Connect variants. |
| architect-r3 4 | minor | R10 | fixed another way | Item 4 names the shared fixture and Store setup. It installs the shadow function after migration using the existing fixture schema. |
| architect-r3 5 | minor | R8 | fixed | Repeats logic-r3 1. Item 4 declares the sentinel, and the API test proves its exact code, sentence and unchanged state. |
| architect-r3 6 | minor | D7 | fixed | Item 8 distinguishes the observation of adoption from diagnosis of a restore. One load event is sufficient for wave 1, and issue 1641 tracks restore recovery. |
| architect-r3 7 | minor | D7, C13 | fixed another way | The diagnostic tells the operator to restore the matching OpenBao key, and the test proves successful recovery. A new identity would require rotation covered by issue 1639. |
| architect-r3 8 | minor | R7 | fixed another way | Repeats logic-r3 2. Issuance leaves the cache untouched. Independent cancellable slots avoid blocking cache hits or issuance behind a cold verification. |
| architect-r3 9 | minor | plan | fixed | Repeats logic-r3 7. The old ownership decision explicitly points to row 32 and is closed. The existing pins carry forward. |
| logic-r3 1 | major | plan | fixed | Item 4 exports the directly returned domain sentinel and a test fixture shared with item 7. A row at the exact bound drives the API test through the real Store. |
| logic-r3 2 | major | R7 | fixed another way | A short cache mutex and separate cancellable derivation slots preserve bounded work and allow issuance during a held verification. The test asserts actual cache length, eviction and admission behavior. |
| logic-r3 3 | minor | plan | fixed another way | Repeats architect-r3 4. The helper migrates before shadowing and reopens the same schema with the explicit path. The setup checks the clock, and the stored defaults remain bound to the built-in. |
| logic-r3 4 | minor | D7 | fixed | Repeats architect-r3 6. The revised rationale cites issue 1641 and distinguishes first creation from adoption. The interruption case checks retained identity without adding a log field. |
| logic-r3 5 | minor | C4 | fixed another way | Store owns the admission deadline and closes failed connections. Parent cancellation and driver failure stay distinct from contention. Migration and prerequisite deadlines descend from the lifetime context. |
| logic-r3 6 | minor | R10 | fixed another way | Repeats architect-r3 1. The dedicated application name selects a transport relay rather than a query tracer. The injected deadline drives the actual stalled-reply path. |
| logic-r3 7 | minor | R9 | fixed | The module handoff records either merge order and the second tidy run. Older decision rows point to the authoritative ownership and browser rules. |

## Points and stuck statements

The 16 findings form ten groups. Each group contributes its highest-severity weight once under its primary statement. A minor counts for one point, a major for three and a blocker for nine.

| Statement | Points | Share of round |
|---|---|---|
| D7 | 3 | 21.4% |
| R7 | 3 | 21.4% |
| plan | 3 | 21.4% |
| C4 | 2 | 14.3% |
| R10 | 1 | 7.1% |
| R8 | 1 | 7.1% |
| R9 | 1 | 7.1% |
| Total | 14 | 100.0% |

No statement is stuck. The largest group of points under any statement is three.

## Plan headings for the next round

- **Overview, Key decisions and Workstreams.** Shorten the summary of the current scope and goals.
- **Modules and first consumers.** Record both first-import orders and the second merge's tidy check.
- **Item 4. Session issuance, verification and sign-out.** Declare the sentinel and share the real clock fixture. Separate issuance, verification and cache access.
- **Item 6. Request handling and the session guard.** Map auth domain errors and recognize both Connect overrun messages.
- **Item 7. The three Session calls.** Drive the API clock refusal from the exact database boundary and exercise both overrun paths.
- **Item 8. OpenBao and the signing key.** Assert permitted writes, test recovery using the matching key and record why adoption is not a restore diagnosis.
- **Item 9. The runnable application and the in-process test entry point.** Distinguish admission outcomes and exercise stalled probes through the actual transport.
- **Attack log and Post-build checks.** Record this round and preserve identity across interruption before pin commit.

## Session verification and its two major findings

The clock refusal and the verification cache both belong to the session layer, but they fail at separate boundaries. The refusal lacked a shared database fixture. The cache let a blocking derivation hold state that other requests needed. The revised plan shares only the clock setup and keeps both database state and derivation admission explicit.

## Top finding and reviewer grades

The top finding was major logic-r3 1, also reported by architect-r3 5 at minor severity. The API had no deterministic way to exercise its session clock error. The fold connects a shared database fixture to the normal SignIn route.

| Reviewer | Verdict | Grade | Blockers | Majors | Minors |
|---|---|---|---|---|---|
| Architect | SHIP | 8/10 | 0 | 0 | 9 |
| Logic | REVISE | 7/10 | 0 | 2 | 5 |
