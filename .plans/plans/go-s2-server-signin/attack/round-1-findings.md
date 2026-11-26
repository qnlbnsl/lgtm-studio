# Sign-in plan findings from attack round 1

Round 1 assessed the frozen plan. The plan stays in attack after these changes.

## Findings and outcomes

| Finding | Severity | Statement | Outcome | Reason |
|---|---|---|---|---|
| architect-r1 1 | blocker | C1 | fixed | Item 9 implements the binary health command, exact readiness status and dispatch table with TestHealthCommand. Stack's Compose probe reads that command. |
| architect-r1 2 | major | D10 | fixed another way | Item 1 keeps the five PostgreSQL fields already produced by Stack sprint 1 and uses the Stack listener name. Warnings name only retired server settings so shared environment variables remain usable. |
| architect-r1 3 | major | C3 | fixed another way | Items 1 and 5 make Stack supply its actual public origin and admit the three bundled loopback aliases at that exact port. An HTTPS public address remains explicit. |
| architect-r1 4 | major | C4 | fixed another way | Item 9 holds an exclusive PostgreSQL session lock before migrations or reconciliation and shuts down on lock connection failure. This prevents a losing server from starting schema work. |
| architect-r1 5 | major | R9 | fixed | Item 1 accepts the Stack file's group-readable protected mode. Tests check the mounted-file shape and the rejection of unsafe modes and file types. |
| architect-r1 6 | major | R10 | fixed | Item 9 adds servertest.Server.Environment for the sprint 3 command test and names app.Start as the later handler and migration edit point. |
| architect-r1 7 | minor | C4 | fixed | Repeats logic2-r1 2. Items 1 and 9 distinguish committed revocation from a failed attempt, and PB3 exercises completion after recovery. Logic2-r1 2 records the remaining risk decision. |
| architect-r1 8 | minor | D3, R4 | fixed | Item 9 gives one order. Item 3 records every removed session row. Listener, origin and vault validation follow reconciliation. |
| architect-r1 9 | minor | plan | fixed | Repeats critic-r1 3. The module handoff pins both dependencies and names their first importers. |
| architect-r1 10 | minor | C13 | fixed another way | Item 8 performs one canonical read after any create outcome, including permission denial or loss of the reply. This handles the pinned permission boundary without decoding conflict text. |
| architect-r1 11 | minor | C13 | fixed | Item 9 sets the logger to omit the caller field and freezes the typed source, fingerprint and vault version in the event test. |
| critic-r1 1 | major | plan | fixed another way | Repeats architect-r1 2. Item 1 defines the common map against the existing Stack and Store settings and reserves removed names for fixed warnings. |
| critic-r1 2 | major | C3 | fixed | Repeats architect-r1 3. Configuration and Stack checks use a nondefault port and the exact derived numeric origin. The browser probe sends a real Origin header. |
| critic-r1 3 | major | plan | fixed another way | The module handoff uses the published Go-1.26-compatible dependency pair. The network module requires the selected crypto version. Official tag pages and cached metadata cover the proxy check that sandboxing refused. |
| critic-r1 4 | major | C2 | fixed | Items 1 and 9 define fixed, redacted, ordered diagnostics with a stream and exit status and a case-by-case command test. |
| critic-r1 5 | minor | plan | for the owner | Repeats logic2-r1 6. The retry exception and its lost-response behavior are carried to the owner before tests freeze. |
| critic-r1 6 | minor | C4 | fixed another way | Repeats logic2-r1 2. The operator procedure confirms a transition after database recovery. The separate exclusive-start and current-row binding changes enforce the routine serving boundary. |
| critic-r1 7 | minor | D11 | fixed | Item 5 documents the unpartitioned cross-site cookie. PB2 covers both browser policies and the bundled same-site fallback. |
| critic-r1 8 | minor | plan | fixed another way | Item 4 tests an exact microsecond boundary against an injected timestamp. This removes wall-clock tolerance from the boundary assertion. |
| critic-r1 9 | minor | R8 | fixed another way | Item 6 specifies the library's measured-size sentence, the request-key refusal and the empty early HTTP rejections. The pinned transport enforces the read limit before a unary interceptor can run. |
| critic-r1 10 | minor | R9 | fixed | Item 2 names the derivation and tag operations and item 4 proves derivation completes before table-lock admission. |
| critic-r1 11 | minor | plan | fixed | The execution sizing rates router and application work high. Each separates its independently frozen test groups within its ownership boundary. |
| logic-r1 1 | major | C4 | fixed another way | Repeats architect-r1 4. Item 9 blocks simultaneous serving. Item 4 independently checks the current row's password binding and proves altered rows fail, as recorded in threat-r1 3. |
| logic-r1 2 | minor | D5 | post-build check | PB7 observes a lost reply or unconfirmed commit at the eight-row cap. A second explicit sign-in can consume another slot. A compensating write could neither prove the initial outcome nor restore its evicted row. |
| logic-r1 3 | minor | D4 | fixed | Item 4 refuses and prunes a row more than one minute ahead of database time. Logic2-r1 3 covers expiry across an arbitrary clock rollback. |
| logic-r1 4 | minor | plan | for the owner | Repeats logic2-r1 6. The owner decides the retry exception. PB2 confirms what header the existing browser sends. |
| logic-r1 5 | minor | D4, R3, R7 | fixed | TestSessionIssueTieAndBackwardClock freezes a timestamp tie and verifies the eviction and removal feed order. |
| logic-r1 6 | minor | C2, C4 | fixed | Repeats logic2-r1 2. Items 1 and 9 specify a fixed rollback or completed-purge diagnostic and PB3 tests an intentional transition after recovery. |
| logic-r1 7 | minor | C13 | fixed | Repeats architect-r1 10. Item 8 uses one read after an uncertain create and makes no additional create. PB4 uses the real vault. |
| logic-r1 8 | minor | C13 | fixed another way | Repeats logic2-r1 4. Item 8 verifies the public key against a PostgreSQL pin and logs the returned vault version for the Stack check. |
| logic2-r1 1 | blocker | C1 | fixed | Repeats architect-r1 1. Items 1, 5 and 9 provide the server binary probe, rendered Compose settings and the real browser origin. |
| logic2-r1 2 | major | C4 | for the owner | The guarantee covers committed revocation. A durable intent that outlives a failed transition and an arbitrary reversal of settings needs an owner decision. PB3 checks the recovery path. |
| logic2-r1 3 | major | C4 | for the owner | An expired row can become visible after a large backward clock change across restarts. Decision 23 records the provisional supported clock behavior. Durable one-way expiry needs an owner choice. |
| logic2-r1 4 | major | C13 | fixed another way | Item 8 introduces a single public fingerprint row in PostgreSQL and compares it before serving. Established missing or changed key values cause refusal. The fixture and sprint 3 command consume the shared loader. |
| logic2-r1 5 | major | D11 | fixed | Items 5 through 7 keep separate host-only names for the two HTTPS session modes. One mode's issue or clear leaves the other mode's cookie untouched. |
| logic2-r1 6 | major | plan | for the owner | The general retry rule and credential response need an explicit owner decision. Item 7 and decision 7 retain a provisional unkeyed policy with the effects spelled out. |
| logic2-r1 7 | major | C3 | fixed | Items 4, 6 and 7 use Store's domain errors and inspect the commit outcome before retryability. Tests observe failure status and no cookie write. |
| logic2-r1 8 | major | R1 | fixed | Item 9 allows Store its full five-minute migration budget and Stack gets a compatible readiness wait. TestStartupMigrationBudget observes the deadlines received at the phase boundaries. |
| logic2-r1 9 | minor | C4 | fixed | Item 4 synchronizes on actual lock admission and uses the exact microsecond expiry boundary for deterministic checks. |
| logic2-r1 10 | minor | plan | fixed | Repeats critic-r1 3. The module handoff records versions, source modules and their first consumers. |
| logic2-r1 11 | minor | C4 | post-build check | PB1 repeats C4 with a limit-sized fixture and an independently generated valid replacement. The Stack handoff requests the driver change under its file ownership. |
| threat-r1 1 | major | C4 | fixed another way | Repeats architect-r1 4. Item 9 pairs pre-migration exclusion with loss detection on the lock connection. Item 4 also refuses sessions that no longer match the configured password. |
| threat-r1 2 | major | plan | fixed another way | Repeats architect-r1 2. Stack reads one settled server settings table. Item 9 supplies the health command covered separately under architect-r1 1. |
| threat-r1 3 | minor | C4 | fixed | Item 4 checks each current row's binding before authentication and covers altered rows at runtime. Successful full-row matches are memoized only for the active password. |
| threat-r1 4 | minor | C4 | fixed | Repeats logic2-r1 2. Item 9 emits a fixed removal failure diagnostic and specifies the recovery invocation documented by Stack. |
| threat-r1 5 | minor | D11 | fixed | Item 5 uses host-prefixed cookie names on HTTPS and retains the plaintext loopback name. PB2 exercises the sibling-site cookie boundary. |
| threat-r1 6 | minor | D11 | fixed another way | Item 5 selects the cross-site cookie only with a listed explicit Origin. Native Origin-less callers select the normal cookie and do not need an artificial browser header. |
| threat-r1 7 | minor | R9 | fixed | Repeats critic-r1 3. The plan pins and documents the reviewed scrypt and public-suffix imports. |
| threat-r1 8 | minor | D7 | fixed | Item 8 requests read and create permissions and denies updates to the signing-key path. PB4 checks the permissions and a stable version. Logic2-r1 4 supplies the independent identity comparison. |

## Points and stuck statements

The 49 reports of findings form 32 groups. Each group contributes the weight of its highest severity once, using its primary statement. Minor findings count for one point, major findings count for three and blockers count for nine.

| Statement | Points | Share of round |
|---|---|---|
| C4 | 12 | 17.6% |
| C1 | 9 | 13.2% |
| plan | 8 | 11.8% |
| C3 | 6 | 8.8% |
| D11 | 6 | 8.8% |
| C13 | 5 | 7.4% |
| R9 | 4 | 5.9% |
| C2 | 3 | 4.4% |
| D10 | 3 | 4.4% |
| R1 | 3 | 4.4% |
| R10 | 3 | 4.4% |
| D4 | 2 | 2.9% |
| D3 | 1 | 1.5% |
| D5 | 1 | 1.5% |
| D7 | 1 | 1.5% |
| R8 | 1 | 1.5% |
| Total | 68 | 100.0% |

No statement is stuck. C4 reaches 12 points and accounts for 17.6% of the round, below the required 30%.

## Plan headings for the next round

- **Key decisions, Workstreams and Rollout order.** Describe independent browser sessions, signing-key continuity, committed revocation, the provisional retry policy and rollout of the additive schemas.
- **Modules and first consumers.** Pin the reviewed Go dependencies and assign the import handoff.
- **Item 1. Server settings.** Settle the Compose environment, protected database credential file and fixed startup diagnostics. Verify against the Stack plan before freezing tests.
- **Item 2. Password verification and binding.** Name the derivation and tag functions used at the session lock boundary.
- **Item 3. Session schema and password reconciliation.** Specify recorded removal for expired as well as revoked rows and distinguish failed from committed reconciliation.
- **Item 4. Session issuance, verification and sign-out.** Verify stored bindings, fix Store error handling and pin lock, expiry and eviction tests.
- **Item 5. Browser origins and cookies.** Admit the bundled loopback aliases, distinguish the two HTTPS cookies and require explicit cross-site Origin selection.
- **Item 6. Request handling and the session guard.** Finish wire errors and test the browser's explicit Origin on the configured port.
- **Item 7. The three Session calls.** Test commit outcomes, credential retries, the transport read limit and cookie separation.
- **Item 8. OpenBao and the signing key.** Verify persisted creation, pin the public identity, specify vault permissions and exercise exact log fields.
- **Item 9. The runnable application and the in-process test entry point.** Own binary dispatch, pre-migration exclusion, phase deadlines and the shared test fixture settings.
- **Traceability, Execution sizing and Attack log.** Add the release startup check to the trace, size the larger boundaries and record this round.
- **Post-build checks from round 1.** Check Compose, browser policy, recovery, the real vault, stream revocation and lost sign-in replies.

## Mechanisms with several major findings

- **Stack startup.** The missing command and inconsistent environment share the same entry point. The binary now owns readiness, and Stack feeds it one server settings contract.
- **Session validity.** Startup reconciliation alone cannot protect a running server from concurrent ownership or an altered row. Exclusive startup and runtime row verification enforce the serving boundary. The choices about failed transitions and clock rollback are recorded in decisions 22 and 23.
- **Browser cookie selection.** One name let the two HTTPS modes overwrite each other. Cookie selection now chooses an independent name using the admitted Origin.

## Top finding and reviewer grades

The top finding was blocker architect-r1 1, also reported by logic2-r1 1. Compose had no executable readiness command in the distroless Server image. The missing command stopped the first release check and all dependent checks.

| Reviewer | Verdict | Grade | Blockers | Majors | Minors |
|---|---|---|---|---|---|
| Architect | REVISE | 6/10 | 1 | 5 | 5 |
| Critic | REVISE | 6/10 | 0 | 4 | 7 |
| Logic | REVISE | 6/10 | 0 | 1 | 7 |
| Logic 2 | REVISE | 5/10 | 1 | 7 | 3 |
| Threat | REVISE | 6/10 | 0 | 2 | 6 |
