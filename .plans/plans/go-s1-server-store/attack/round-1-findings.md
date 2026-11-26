# Round 1 findings folded into the server store plan

Folded on 2026-11-26 against the frozen [plan reviewed in round 1](plan-v1.md). The 49 findings have 19 majors and 30 minors. No reviewer reported a blocker. The Store is planned and its Go code is still absent.

The statement identifiers refer to the approved design's decisions (D), rules (R) and checks (C). A finding can cite several statements. The points column assigns each duplicate group to one primary statement so its highest severity contributes once. A repeated finding names the primary identifier. A finding with a separately testable failure stays distinct even if it touches the same code.

## Findings and outcomes

| Finding | Severity | Statements cited | Points statement or repeated finding | Outcome | Reason |
|---|---|---|---|---|---|
| architect-r1 1 | major | C12, R10, plan | C12 | fixed another way | Schedule the driver with its first use. The Stack fixture supplies the role, and the hook has a Server body and Stack wrapper. |
| architect-r1 2 | major | R8, R2, C6 to C9 | R8 | fixed | Use the existing draft's aborted status and define the application sentence. |
| architect-r1 3 | major | R4, D5, C12 | R4 | fixed | Add the transaction-local flush flag, insert trigger and direct-insert test. |
| architect-r1 4 | minor | R2, D1 | R2 | fixed | Name the variant mapping and checked conversion, including a present zero position. |
| architect-r1 5 | minor | D5, R5, C6 to C9 | critic-r1 4 | fixed | Repeat critic-r1 4. Freeze the observed-row formula and test a full page crossing it. |
| architect-r1 6 | minor | R7, D3 | R7 | fixed | Detect a lower head, close old subscriptions and reopen on the restored head. |
| architect-r1 7 | minor | D4, D5, R4 | logic-r1 2 | fixed another way | Repeat logic-r1 2 for the wait bound. Retryable errors go to the caller and deferred checks move ahead of the lock. |
| architect-r1 8 | minor | R1, D3 | R1 | fixed | Write the bootstrap ledger in Go and remove it from the SQL file's contents. |
| architect-r1 9 | minor | R10, plan | R10 | fixed another way | Centralize the lock identifiers and put the migrated fixture in a subpackage to avoid an import cycle. |
| critic-r1 1 | major | D3 | logic-r1 2 | fixed another way | Repeat logic-r1 2 for lock liveness. Retain ordered single-row inserts in one driver batch and run deferred checks before locking. |
| critic-r1 2 | major | D3, C12 | C12 | fixed | Add a gated cold-start test and the ledger-before-lock mutation. |
| critic-r1 3 | major | C12 | C12 | fixed | Freeze the gate helper, SQL trigger, fixture clocks and schemas before implementation. |
| critic-r1 4 | major | D5, R5 | R5 | fixed | Define the threshold once. Exercise no in-flight row, a held row, filtered rows and large committed fixtures. |
| critic-r1 5 | major | R2, R8, plan | architect-r1 1 | fixed another way | Repeat architect-r1 1 for the driver and architect-r1 2 and 4 for the contract. Land the root changes with their first use. |
| critic-r1 6 | minor | plan | plan | fixed | Add tests for validation, subscription state, timed pruning, an empty retained head and one response variant. |
| critic-r1 7 | minor | D5, R9 | D5 | fixed | Test exact key locking and cutoff behavior. Bound reply bytes and separate receipt cleanup from the feed floor lock. |
| critic-r1 8 | minor | plan | plan | fixed another way | Give specific setup errors and stale-feed reporting. Use a disposable database instance for crash cleanup instead of sweeping another test's database. |
| critic-r1 9 | minor | plan | architect-r1 8 | fixed | Repeat architect-r1 8. Only Migrate owns the bootstrap ledger. |
| critic-r1 10 | minor | plan | plan | fixed | Assign Tx and the shared gates to item 4. Pruning depends only on the pool and schema. |
| critic-r1 11 | minor | D5, R5 | D5 | fixed another way | Keep the snapshot lag limit and report repeated immediate lag. Head regression and stale-prefix reporting use the amended reader and sweep. |
| critic-r1 12 | minor | plan | plan | fixed another way | Close the superseded status, document assumptions and retain the vendor counts required by the sizing probe. |
| logic-r1 1 | major | C12 | C12 | fixed | Observe the sequence allocation while B waits. Require both lock removal and lock-after-insert to fail. |
| logic-r1 2 | major | D3 | D3 | fixed | Bound lock waits and writes. Reserve reader capacity and test a saturated writer pool. |
| logic-r1 3 | major | C12 | C12 | fixed | Exercise cancellation, panic and commit failure on a different backend for both advisory lock paths. |
| logic-r1 4 | minor | plan | plan | fixed | Cancel stale subscribers and report once per outage after the read deadline. |
| logic-r1 5 | minor | D5 | critic-r1 4 | fixed | Repeat critic-r1 4. The in-flight row counts once, across all topics and scopes. |
| logic-r1 6 | minor | plan | plan | fixed | Reject invalid request metadata and incomplete Watch sources before using either path. |
| logic-r1 7 | minor | C12 | C12 | fixed another way | Test an ambiguous method and payload boundary within one principal. Terminate the reader after its first applied page. |
| logic-r1 8 | minor | D5 | D5 | fixed another way | Base pruning on committed observation and report a stalled prefix. Keep young rows through clock skew. |
| logic-r1 9 | minor | D5 | D5 | fixed | Use one cursor-owning goroutine and test simultaneous poll and notification. |
| logic-r1 10 | minor | C12 | critic-r1 2 | fixed | Repeat critic-r1 2. Freeze a cold-start test before bootstrap. |
| logic-r1 11 | minor | D5 | D5 | fixed | Record the old removal and new save together and test both scopes. |
| logic2-r1 1 | major | R4 | R4 | fixed | Use the prescribed durable first observation of committed rows for both retention periods. |
| logic2-r1 2 | major | R5 | R5 | fixed | Time out real stalled page queries and listener registration without resetting progress. |
| logic2-r1 3 | major | R4 | R4 | fixed | Store the fingerprint version and require stable semantic encoders across upgrades. |
| logic2-r1 4 | major | R4 | R4 | fixed | Force domain checks before taking the lock and keep a test-only commit gate. |
| logic2-r1 5 | major | R5 | R5 | fixed | Freeze both snapshot split mutations with load barriers on either side of the record read. |
| logic2-r1 6 | minor | R2 | architect-r1 2 | fixed | Repeat architect-r1 2 and the mapping in architect-r1 4. Include explicit zero presence. |
| logic2-r1 7 | minor | R5 | R5 | fixed | Reject a second Next with an outstanding row. Exercise state transitions and cleanup. |
| threat-r1 1 | major | R5 | R5 | fixed | Require per-message and periodic session authorization. Cancel a blocked sender when revoked. |
| threat-r1 2 | major | R9 | R9 | for the owner | Choose how to replay credential-bearing calls without persisting credentials or a password-derived digest. |
| threat-r1 3 | major | R4 | logic-r1 2 | fixed | Repeat logic-r1 2. Cap writer admission and verify reader progress while writers wait. |
| threat-r1 4 | minor | R5 | R5 | fixed another way | Cap at 256 and check one ordered driver batch. Observe driver tracing rather than assume protocol round-trip counts. |
| threat-r1 5 | minor | R4 | architect-r1 3 | fixed | Repeat architect-r1 3. The guard prevents accidental direct insertion. Parts leave transaction control to the store. |
| threat-r1 6 | minor | D3 | D3 | fixed | Namespace the receipt primary key and the advisory lock by the authorized principal. |
| threat-r1 7 | minor | D3 | logic2-r1 3 | fixed another way | Repeat logic2-r1 3. Semantic versions keep equivalent requests stable across supported transport encodings. |
| threat-r1 8 | minor | R9 | R9 | fixed another way | Discard database detail and raw causes. Test safe formatting and error inspection. |
| threat-r1 9 | minor | D4 | D4 | fixed another way | Check every endpoint and fallback. Allow the private stack exception only when the Server bootstrap opts in. |
| threat-r1 10 | minor | R4 | logic2-r1 4 | fixed another way | Repeat logic2-r1 4. Run deferred checks early and classify residual deadlocks without replaying callbacks. |

## Points and stuck statements

A minor is worth one point. A major is worth three points. A blocker is worth nine points. 37 distinct groups contribute 69 points.

| Statement | Points |
|---|---|
| C12 | 16 |
| D3 | 4 |
| D4 | 1 |
| D5 | 5 |
| plan | 6 |
| R1 | 1 |
| R2 | 1 |
| R4 | 12 |
| R5 | 14 |
| R7 | 1 |
| R8 | 3 |
| R9 | 4 |
| R10 | 1 |
| Total | 69 |

No statement is stuck in round 1. C12 has the most points, at 16 of 69. R4 and R5 also reach 12 points, but no statement reaches 30 percent of the total.

## Patches for the next round

| Plan heading | Changes to review |
|---|---|
| Overview, Key decisions and Rollout order | Driver ownership and integration with tidy, Stack fixture, mutation hook interface and the still-open credential policy. |
| 1. Connection pool, query handles and test isolation | Writer admission, deadlines, sanitized database errors, connection policy, fixture messages and cleanup. |
| 3. Numbered database changes and store tables | Cold bootstrap, lock registry, migrated fixture, trigger, principal-specific receipt keys, fingerprint version and durable retention age. |
| 4. Feed writes and commit order | Deferred checks, capped ordered batch, flush guard, named fixture gates, mutation hook, lock timing and release proofs. |
| 5. Request receipts | Principal namespaces, semantic fingerprints, reply cap, retention window, validation and restart fixture. |
| 6. Feed and receipt retention | First committed observation, separate replay cleanup transaction, stalled prefix reporting and lifecycle checks. |
| 7. Read committed pages with notifications and polling | One cursor owner, stalled-query and listener deadlines, history detection and retry seams. |
| 8. Bounded broadcast and subscriber lifetime | Lag count, invalid transitions, reset and outage handling. |
| 9. Typed Watch streams | Authorization, exact contract mapping, both split-read mutations and blocked sender behavior. |
| Traceability, Execution sizing, Attack log and Post-build checks from round 1 | Credential-policy boundary, revised dependency order, test ownership and merged evidence. |

The feed writer had several major findings in one mechanism. The underlying transaction-level lock still fits the design. The problems were where it started, what ran under it and how the tests observed it. Keep the lock and re-review the batch, guard and deferred constraints together. The receipt mechanism also had multiple majors. Its atomic write remains the right boundary. Its timestamp and fingerprint designs changed. The owner still has the credential-handling decision below. The Watch helper still uses a single snapshot transaction and a shared follower. Re-review the new end conditions with its session and transport adapters.

## Top finding

Major, logic2-r1 1. A receipt could expire less than a day after commit, letting a retry execute an already committed change. The first-observation clock protects the full retention window when the database clock advances normally.

## Choice for the owner

Major, threat-r1 2. The design requires stable retries and prohibits storing secrets in the server's tables. Choose the behavior for keyed calls that accept a secret or return a newly created credential:

1. Reject the request key for credential-bearing calls. The scope is small, but the caller may have to recover manually from a lost reply.
2. Store a completion receipt with a keyed fingerprint from the vault. Return a fixed recovery response on replay. This avoids saving the credential and avoids a second mutation. The original secret reply is available only on the first successful delivery.
3. Add vault-backed replay for the original reply. This preserves identical retries. It adds a vault dependency and requires handling partial failures between the vault and PostgreSQL.

The store plan leaves that choice to the owner. The existing prohibition on credential storage applies while later adapters are being planned.

## Reviewer verdicts and grades

| Reviewer | Verdict | Grade | Findings before deduplication |
|---|---|---|---|
| Architect | Revise. | 7 out of 10. | 3 majors and 6 minors. |
| Critic | Revise. | 6 out of 10. | 5 majors and 7 minors. |
| Logic | Revise. | 6 out of 10. | 3 majors and 8 minors. |
| Second logic | Revise. | 6 out of 10. | 5 majors and 2 minors. |
| Threat | Revise. | 6 out of 10. | 3 majors and 7 minors. |
