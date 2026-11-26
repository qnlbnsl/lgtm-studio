# Sign-in plan findings from attack round 2

Round 2 assessed the patches from round 1. The plan remains in attack.

## Findings and outcomes

| Finding | Severity | Statement | Outcome | Reason |
|---|---|---|---|---|
| architect-r2 1 | major | D7 | fixed another way | Item 8 and PB4 adopt Stack's built policy. The Server writes only with initial compare-and-set, and the public pin detects replacement. A Stack-owned permission change is requested separately. |
| architect-r2 2 | major | R10 | fixed another way | Item 9 declares private phase budgets and their caller. Item 4 uses one clock function in the real PostgreSQL test schema to cover both exact time and production clock sourcing. |
| architect-r2 3 | minor | C4 | fixed another way | PostgreSQL waits under the admission context. The monitor begins before migrations and checks before listening. The tests cover released contention, deadline and loss during startup. |
| architect-r2 4 | minor | D11 | fixed | Item 5 ties the bundled aliases to the plaintext public origin and its port. TestOriginPolicies names the refused HTTPS cases. Decision 27 supersedes the older rule. |
| architect-r2 5 | minor | R9 | fixed another way | The module handoff lists the selected indirect graph and verifies the Store suite. Only imports that require a root entry cause one to be recorded. |
| architect-r2 6 | minor | R7 | fixed another way | Item 4 specifies least-recently-used eviction and one mutex for successful verification and new derivation. The parallel cache test observes one derivation for an identical tuple. |
| logic-r2 1 | major | plan | fixed another way | Repeats architect-r2 3. Use the database's bounded advisory wait and explicitly unlock the owning connection during shutdown. The lock test observes wait and immediate reacquisition. |
| logic-r2 2 | major | plan | fixed another way | Use a five-second probe on the existing connection. A real delayed query for 1.5 seconds must succeed without replacing that session. Loss still cancels startup and serving. |
| logic-r2 3 | major | plan | fixed | Repeats architect-r2 4. An HTTPS public origin never admits a plaintext alias. Item 5 tests HTTPS public addresses on loopback and elsewhere. |
| logic-r2 4 | major | plan | fixed another way | The fixture shadows the real database time function and tests Issue, Check and Revoke. Clock overflow is a dedicated domain error with an aborted response and no falsely classified Store outcome. |
| logic-r2 5 | minor | plan | fixed | Item 9 emits one revocation outcome even when a purge never starts. Database or admission details precede it, and the password line follows it as Stack's handoff requires. |
| logic-r2 6 | minor | plan | fixed | Item 9 reads the already reserved serverInstanceLockID from locks.go. The new instance_test.go asserts distinct identifiers. |
| logic-r2 7 | minor | plan | fixed another way | Revoke verifies outside the lock. It compares the full verified row and the current expiry under the lock to avoid deleting a row that changed in the interim. |
| threat-r2 1 | major | C13 | fixed another way | Repeats architect-r2 1. The plan uses the built vault role and proves the Server never updates the seed. The result names the Stack policy, initializer and tests for a future least-privilege change. |
| threat-r2 2 | minor | D11 | fixed | Repeats architect-r2 4. A plaintext alias is bundled only under a plaintext loopback public origin at the same configured port. |
| threat-r2 3 | minor | D11 | fixed | The browser risk table and PB1 state the existing plaintext cookie's port boundary and verify that a replacement causes only refusal. |
| threat-r2 4 | minor | R9 | fixed another way | Repeats architect-r2 5. The first-import builders check the tagged graph, Go checksum verification and the Store suite. These checks cover the dependency and checksum changes under the epic's existing build rules. |
| threat-r2 5 | minor | R7 | fixed | Repeats logic-r2 7. Item 4 verifies before locking, revalidates the tuple and uses item 2's constant-time helper. A gated derivation proves that sign-in can proceed during verification. |
| threat-r2 6 | minor | D7 | rejected | The proposed field addresses an older database restore. The frozen attack/plan-v2.md:441 already places that case outside wave 1. The first-start path cannot distinguish restore from initialization. The result asks to track recovery separately. |
| threat-r2 7 | minor | R9 | fixed | Item 1 identifies the trusted parent path and checks protected owner, mode and file type on the opened descriptor. |

## Points and stuck statements

The 20 findings form 14 groups. Each counts once at its highest severity under its primary statement. Minor findings count for one point, majors for three and blockers for nine.

| Statement | Points | Share of round |
|---|---|---|
| plan | 9 | 34.6% |
| D11 | 4 | 15.4% |
| D7 | 4 | 15.4% |
| C4 | 3 | 11.5% |
| R10 | 3 | 11.5% |
| R9 | 2 | 7.7% |
| R7 | 1 | 3.8% |
| Total | 26 | 100.0% |

No statement is stuck. The largest bucket is the plan, with 9 points, below the 12-point minimum.

## Plan headings for the next round

- **Overview, Key decisions, Workstreams and Rollout order.** Set the joint gates and the wave 1 loopback browser boundary.
- **Build loop and Modules and first consumers.** Set first-import ownership and the module graph checks.
- **Item 2. Password verification and binding.** Tie the existing constant-time helper to the runtime checks that read it.
- **Item 3. Session schema and password reconciliation.** Use the current Store change-reference type and the binding comparison helper.
- **Item 4. Session issuance, verification and sign-out.** Control and prove the database clock, freeze cache bounds, and move revocation derivation out of the write lock.
- **Item 5. Browser origins and cookies.** Define the loopback origin condition and test the encrypted public-origin cases.
- **Items 6 and 7. Request handling and the session guard, and The three Session calls.** Map the clock refusal and drive the rollback and uncertain cases through the real Store.
- **Item 1. Server settings.** Match the eleven Stack keys and make the file-descriptor and parent-directory boundary explicit.
- **Item 8. OpenBao and the signing key.** Use the existing policy and place competing creates in the scripted fixture.
- **Item 9. The runnable application and the in-process test entry point.** Define the startup budgets, locking lifecycle and ordered removal diagnostics. Reuse the existing Store fixture support.
- **Traceability, Execution sizing, Attack log and Post-build checks.** Record the joint gate, move the later browser checks, and cover current vault behavior and the local cookie risk.

## Mechanisms with several major findings

- **Vault policy and key continuity.** Both reviewers found the same unsupported permissions guarantee. The Server relies on compare-and-set and the stored public pin. The result requests a Stack issue for least privilege and notes the remaining risk to key availability.
- **Serving lock.** A one-shot admission probe can refuse a normal restart, and a one-second fatal check can mistake scheduling delay for connection loss. The plan uses the database's bounded wait and monitors the owning connection during startup and serving.
- **Time and test seams.** Freezing an input value alone does not prove production uses database time. The session tests replace the clock within the real database, and the startup tests pass private budgets through the actual app path.

## Top finding and reviewer grades

The top finding is architect-r2 1, a major repeated by threat-r2 1. The plan's OpenBao permission expectation conflicted with the built policy and test. The current Stack role allows updates, while the Server uses only initial creation and fails readiness on a changed established key.

| Reviewer | Verdict | Grade | Blockers | Majors | Minors |
|---|---|---|---|---|---|
| Architect | REVISE | 7/10 | 0 | 2 | 4 |
| Logic | REVISE | 6/10 | 0 | 4 | 3 |
| Threat | REVISE | 7/10 | 0 | 1 | 6 |
