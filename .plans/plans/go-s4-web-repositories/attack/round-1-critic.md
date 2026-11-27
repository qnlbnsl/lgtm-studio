REVISE

The planner, test writers, builders and reviewer are workers of one vendor, OpenAI.

The discovery rule can start the sample workspace and simulation on a real server that advertises Projects alongside an unfamiliar service.

## Findings

The evidence comes from reading the plans and source at commit `5d451ffd`. The browser scenarios describe the consequences of implementing the plan as written.

### critic-r1 1 · MAJOR · R13

Item 1 says that an unknown entry selects compatibility behavior. The reviewed contract says clients ignore unknown entries. The Projects plan preserves other entries when it advertises Projects. Evidence is `.plans/plans/go-s4-web-repositories/plan.md:83–85`, `.plans/plans/go-s4-contract/plan.md:261–266` and `.plans/plans/go-s4-server-projects/plan.md:208`.

A real revision 1 server advertises `lgtm.v1.ProjectService` and another service. Following the fallback sentence seeds sample projects and leaves the real repository watch unopened. The same rule could break Sprint 5 discovery when the service list grows.

Specify that the case-sensitive membership test alone determines whether the required service is advertised. Freeze cases for the exact entry alongside an unknown entry in both orders, an unknown entry alone, a lookalike alone, an empty or omitted list, and the exact entry with the sample flag set. Require the mixed real list to open both watches and schedule no simulation.

### critic-r1 2 · MAJOR · R13

Item 1's proposed timer fence has a file list that leaves out two sample writers. The store updater invokes its callback against whichever state is current when the callback fires. Evidence is `.plans/plans/go-s4-web-repositories/plan.md:80–85`, `src/frontend/src/lib.tsx:46–55`, `src/frontend/src/screens/messages.tsx:132–165` and `src/frontend/src/screens/harness-catalog.tsx:47–63`.

Send a sample message, switch servers before the canned reply fires, and advance the clock. The reply appends a sample post to the real workspace. A delayed sample harness installation can instead update the first real machine's harness list and the shared catalog.

Add both files and those writers to item 1's fence. Audit the remaining delayed store mutations against the item file list. Freeze a clock case for the canned post and a sample installation where the real machine has a different harness inventory. Assert that advancing the clock changes neither the real store nor catalog facts.

### critic-r1 3 · MAJOR · R13

The route table gives `/setup` an unavailable view for a signed-in workspace and relies on the existing authentication gate. That gate expressly exempts signed-out default mounts on `/setup`. The next branch mounts First run, which reads the first machine's workspace immediately and can create sample work. Evidence is `.plans/plans/go-s4-web-repositories/plan.md:82,101–106,125`, `src/frontend/src/app.tsx:629–636` and `src/frontend/src/screens/auth.tsx:168–175,204–219`.

Reload that hash against an advertising server with an expired or absent cookie. A literal implementation can open the simulated setup with an empty machine list and throw before the owner can sign in.

Specify the repository-mode route precedence in item 4. Signed-out and expired direct setup links show Login. Signed-in setup waits for both snapshots and shows the existing unavailable view. Freeze both cases with zero machines and keep the compatibility setup outcomes in the mode fixture.

### critic-r1 4 · MAJOR · R14

The two-second timer starts one repository refresh. The plan gives that refresh no deadline for its first snapshot and lists no way to release the add operation when a replacement watch stalls without closing. Evidence is `.plans/plans/go-s4-web-repositories/plan.md:97–99,181–186`.

The server commits an add and returns its identities. The repository response stream stalls before the matching frame, and the replacement request never delivers its first snapshot. The Machines stream keeps working. The form stays in progress until the owner closes it or the session ends, and the owner cannot retry the same intent from the form.

Give the refresh wait a stated bound. On expiry keep the form and the same add key, release the submit lock, and use the existing network feedback. Treat a current replacement snapshot as evidence of absence only when it actually arrives. Freeze a withheld-first-frame case followed by a same-key retry that converges after the watch recovers.

### critic-r1 5 · MAJOR · R12

The dialog resets its intent and facts when the watched machine disappears. The same reset can make the fixed removal refusal obsolete. The plan tests removal after a successful add, but it does not order an in-flight removal refusal both ways. Evidence is `.plans/plans/go-s4-web-repositories/plan.md:180–186`, `docs/specs/2026-11-27-running-an-agent-design.md:253–260,301` and `.plans/plans/go-s4-contract/plan.md:256–259`.

Submit on machine A while machine B is online. Remove A in another tab while the operation is open. If the machine watch wins the race, the default moves to B, A's refusal is discarded, and the owner gets no explanation of why the pending action ended.

Define the visible result for both arrival orders. If the current intent loses its machine, settle that intent with the fixed removal sentence, preserve the typed path, clear the old facts, and allow the live chooser to select a current machine. Show the sentence once and suppress it after a user edit, replacement dialog or session change. Freeze both arrival orders and assert that selecting a new default sends no add to that machine.

### critic-r1 6 · MAJOR · plan

The approved New project response uses the existing Files and Terminal unavailable treatment. The plan and its decisions log explicitly choose a refusal toast. The current reference is an unavailable view in the body. Evidence is `docs/specs/2026-11-27-running-an-agent-design.md:277–282`, `.plans/plans/go-s4-web-repositories/plan.md:190–193,199–208`, `.plans/plans/go-s4-web-repositories/decisions.md:16` and `src/frontend/src/app.tsx:517,598–600`.

Click Create project without a lead on an online machine. A toast-only implementation passes the planned test, despite replacing the requested treatment with a transient notification.

Make the existing unavailable treatment the required result and pin its place and lifetime inside the open dialog. Freeze the exact sentence and treatment for button and Enter, preservation of the form, and the absence of project requests. Record any proposed presentation change as an owner decision before baking it into the test.

### critic-r1 7 · MAJOR · plan

The recovery row says to show the reason in the existing expanded container. The current offline detail already tells the owner to run a command and promises that agents restart. Item 8 only checks that the new reason is present. Evidence is `.plans/plans/go-s4-web-repositories/plan.md:245–249`, `src/frontend/src/screens/service.tsx:159–181` and `docs/specs/2026-11-27-running-an-agent-design.md:49–50`.

Expand a flagged offline machine. A builder who appends the recovery reason to the ordinary detail leaves the command beside it. The fixture still passes, although the approved opened row gives no instruction and the old sentence describes agents that Sprint 4 cannot observe.

Specify that the recovery reason replaces the old offline or pairing callout while the flag is set. Assert exactly one reason in the opened row, no recovery instruction or simulated agent claim there, and an available Remove machine action. Exercise flagged offline, flagged waiting and flagged host records, plus the transition back to the normal display.

### critic-r1 8 · MAJOR · R14

Three files describe different deployment gates. The Website result names the earlier `deploy/test/checks.sh` or an unspecified Website entry. The Stack plan requires two machine-backed browser adds through `deploy/test/checks-sprint4.sh`. The available Sprint 4 driver explicitly leaves R14 to the owner. Its guide closes the sidebar dialog before performing the only save from Machines. Evidence is `.foundry/go-rewrite/results/plan-go-s4-web-repositories.md:32–36,47`, `.plans/plans/go-s4-stack-checks/plan.md:162,186–198`, `deploy/test/checks-s4.sh:11–16` and `deploy/test/README-s4.md:47–54`.

An implementation with a sidebar dialog that correctly inspects but never commits can pass the current manual instructions. The Stack plan requires a passing R14 result from a different entry point, and the Website item has no exact gate to invoke.

Have the orchestrator name the canonical check entry point, owner, required exit or report evidence, and two-path fixture variables before dispatch. Preserve existing frozen assertions and add the missing sidebar commit and reload assertion alongside them. Pin the listener and locators in the handoff. Require both watched identities and paths after reload, and keep the controlled timing assertions in the Website suite.

### critic-r1 9 · MINOR · plan

Item 5 captures a generated project client from the selected session and claims it can finish concurrently with item 3. Item 3 first populates the shared `Api.projects` client, and item 5's own test uses an isolated typed transport. Evidence is `.plans/plans/go-s4-web-repositories/plan.md:89,95–98,172–176,323` and `src/frontend/src/server/api.ts:14–16,41–43`.

Item 5 can pass an isolated transport test while the actual session returned by item 2 lacks the field it consumes. A builder who works around that by adding the field creates a second writer for `api.ts`.

Specify item 5's injected client signature and say that item 6 obtains it from the session after item 3. Alternatively move the production client seam into an earlier item and make it a dependency. Split item 6 along inspection versus add and watch reconciliation if both sets of assertions cannot be completed in one session. The assignment allows a ninth item.

### critic-r1 10 · MINOR · plan

The shared browser fixture promises host-separated authentication checks but does not name which fixture serves the page for each server or what origin and cookie behavior it emulates. The delayed-cookie test uses only a hostname description. Evidence is `.plans/plans/go-s4-web-repositories/plan.md:78,89–93`, `src/frontend/src/server/api.ts:39–44` and `docs/specs/2026-11-25-go-rewrite-foundation-design.md:197–209`.

A test writer serves the page on one origin and points it at a fixture on a second origin. With no credentialed preflight or valid cookie behavior, the browser rejects requests before the test reaches the stalled sign-out or queued-reply assertions. Path prefixes on one origin also cannot prove independent-host behavior.

Pin the same-host and different-host fixture layouts. Specify the credentialed responses and a positive cookie assertion before either race. Freeze the observed request order, current address, hash and surviving authenticated watch after releasing each obsolete completion.

### Risk estimates

The scores multiply severity, fallout and probability. Evidence confirms the plan gaps by reading. The scores estimate what would happen after dispatch or release.

| Finding | Category and responsible area | Severity, fallout and probability | Score |
|---|---|---|---|
| 1 | Design, Website discovery | 6 because the real workspace is replaced, 3 because deploying the corrected bundle restores it, and 10 for servers advertising the two entries. | 180 |
| 3 | Design, Website routing | 6 because the requested route cannot reach sign-in, 3 because navigating away recovers, and 6 for a direct setup link with an expired session. | 108 |
| 4 | Bug, Website add and watch | 6 because the active save form is stranded, 3 because the owner can close it and recover, and 4 for a stalled stream. | 72 |
| 2 | Bug, Website startup | 4 because sample state appears until reload, 3 because a reload clears it, and 4 for a switch within a short timer window. | 48 |
| 6 | Design, Website dialogs | 2 because the reply is still shown, 3 because the user can submit again, and 6 for submitting from an online machine. | 36 |
| 7 | Design, Website Machines | 4 because following the stale instruction delays recovery, 3 because removal still works, and 3 for the uncommon history fault. | 36 |
| 5 | Bug, Website folder dialog | 4 because the form loses its current explanation, 3 because the owner can continue, and 2 for the overlapping submit and removal. | 24 |
| 8 | Design, orchestrator and Stack | The required release check has no agreed entry point. | Required gate |
| 9 | Design, Website work sequencing | 1 because the defect concerns an intermediate build, 1 because only development is affected, and 6 if builders follow the parallel schedule. | 6 |
| 10 | Design, Website browser fixtures | 1 because the defect affects test setup, 1 because it is confined to development, and 6 if the fixture uses real browser cookies across origins. | 6 |

## Answers to the attack questions

1. All eight items name a first test. Item 1 lacks the mixed service list and delayed message and installation inputs. Item 2 lacks the host and cookie fixture details. Item 3 has writable inputs and results for initial gating, replacement and removal. Item 4 lacks a signed-out and expired setup result. Item 5 lacks the concrete injected-client seam for an independent item run. Item 6 lacks the stalled replacement and both pending-removal outcomes. Item 7 leaves the approved feedback placement untested. Item 8 asserts the reason's presence without asserting the absence of the old instruction.
2. All eight acceptance observations at `plan.md:64–67,160–162,237` name tests or numbered stack observations. No bare adjective such as “works”, “correct” or “fast” substitutes for a result. The R14 observation lacks one agreed executable or documented step and release result across the handoffs.
3. Item 1's “an unknown entry” conflicts with the contract's “Unknown entries are ignored by clients”. The decisions log's “action-refusal toast” conflicts with the approved “in the way the views Files and Terminal already answer”. The epic names four changes, and the later recovery decision adds the fifth. The epic's usual cross-vendor staffing also differs from the explicit one-vendor assignment. The newer instructions govern those last two discrepancies. See findings 1 and 6, `.plans/epics/go-rewrite/epic.md:69–94,179` and `docs/specs/2026-11-27-running-an-agent-design.md:49–50`.
4. Every item lists exact production and test paths under `src/frontend/`. The two missing sample writer files are within the same boundary. The root Playwright configuration and legacy tests are named as read-only patterns. No planned Website production edit lies outside the ownership table at `.plans/epics/go-rewrite/epic.md:135–141`.
5. All eight items are rated medium. Item 6 has separate inspection and save lifecycles and is the first candidate to split. Item 5's concurrent start depends on making its signature independent of item 3. No two items declared parallel name the same file for a write. See `plan.md:95,101,172,178,314–325`.
6. A fresh unconfigured website uses the existing Connect chooser. A fixed address probes the configured service. Missing `LGTM_WEB_PASSWORD` prevents the server starting under foundation C2. With PostgreSQL unavailable before startup, the real stack cannot become ready and the browser falls to its connection gate if the static site is reachable. The plan leaves the browser state open when discovery succeeded earlier but the first watch snapshot never does. A second signed-in load waits for both snapshots and renders saved records. A lost add reply can be recovered with the same intent key in the open dialog. A reload discovers the committed add through the watch. A stalled replacement snapshot and a signed-out setup link remain gaps.
7. Gates 1–10 were probed. 1 fails for the unresolved approved feedback divergence and scope rows that cite dated approval by paraphrase rather than meeting the literal quotation-or-widened probe. The repository's no-private-quotation rule calls for a dated decision citation and an explicit exception to that probe, rather than inserting a quotation. 2 passes the named-observation and adjective probes, with the R14 handoff gap above. 3 passes with the existing design, bounded view inventory and recovery note. 4 passes the named source-to-workstream structure, with findings above on the adequacy of mapped coverage. 5 has full risk rows for all three workstreams. 6 has one rollout order and all three Verify lines name commands and their environments. 7 is pending because the plan's row is still pending and adjudication has yet to occur. 8 passes the anchor probe. `mount`, `watch`, `NewProject` and `AddRepository` match the scouting entries and source. 9 has a populated table citing real repository rules. 10 has eight sized rows with a valid work area and a counted load, but lacks the remaining allowance comparison. Epic linkage also remains pending outside the ten gates. The frontmatter, divider and folds are present.
8. The plan names no execution model or harness as a seat assignment. It names the assigned vendor. The first workstream uses the `WS` abbreviation before spelling out the mapping. Write “workstream” on first prose use and keep exact identifiers. The generic “Overview” heading can name the owner-visible change it describes. Nothing found for private home paths or network addresses in the plan.
a. The plan maps both watches, folder actions, defaults, cleanup, compatibility and the recovery display. Findings 1–7 identify missing or conflicting Website behavior. Sprint 5 writers and services remain out of the work items. Preserve service membership and the shared authenticated scope and the planned extension can add project, agent and lead-thread readiness.
b. The generated service, procedure, fields, header, watch cases and returned identifiers match. The exact service membership rule needs finding 1's repair. Items 2, 3, 5, 6 and 8 otherwise supply mechanisms and named tests for replacement, both reply orders, replay identity, stale approval, sign-out and switch, and flag persistence, clearing and removal. Name expiry and abort handling in the same acceptance matrix.
c. Contract's package export and generated names agree. Projects' three procedure constants, mounted-only discovery and browser preflight agree. Link's `ProjectWithPresence` signature and durable projection agree, and the combined test belongs to the orchestrator. Client Folders supplies the reviewed facts shape and real inspection behavior. The disposable deployed checkout belongs to Stack's fixture, which already plans the dirty A checkout and named B checkout. Stack's file and runner disagree with the Website request and the available driver. Findings above require extra verification without editing any frozen assertion. The orchestrator can materialize the requested scope and update the epic link before dispatch.
d. Most items provide a bounded implementation and a first failing assertion. Item 6 combines replay, debounced inspection, identity reconciliation, recovery and narrow layout. Item 4's route matrix is broad but uses existing views. Resolve the item 5 seam and the expected results listed under question 1 before freezing. Split item 6 if the test and implementation cannot fit one session.
e. The real route table, tree and search can be backed entirely by watched records. The mixed discovery list, setup bypass and late sample writes still expose or introduce sample behavior. The intended post-discovery behavior for both actual samples and an older real server is preserved. The samples now wait for discovery before initial seeding, so the tests must demonstrate that the chooser, sign-in, Track repository and simulation remain usable on all compatibility paths.
f. The dirty sentence, direct sidebar entry, empty selection and diagnostic word and reason have explicit coverage. The toast choice changes the approved New project presentation. The flagged offline detail can retain the instruction to run a command. The waiting diagnostic row is the plan's way to make the fifth visible change observable for waiting machines. Keep it diagnostic-only with the existing removal action. The active machine path label and placeholder are unspecified even though the current labels interpolate a machine workspace. Record their exact behavior in the copy table. Existing empty and unavailable bodies are the authorized route fallback.
g. The one-vendor statement is at the top of this report.

## Post-build checks

- Deploy one unchanged website bundle with both sample transports, the older real server and the advertising server. Confirm the first three reach the sample workflow. The advertising server should send both authenticated snapshots before the shell opens and should show no sample project, including after a reload with the worst-case query.
- On a narrow and a desktop viewport, use both deployed entry points with distinct checkout paths on the selected machine. Compare the three rows to actual checkout facts and both saved paths after reload. Verify that neither the repository center nor search introduces invented activity or a terminal.
- Use the paired history recovery fixture and an open Machines page. Confirm that an unrelated save and reconnect preserve the opened-row reason. A later ready admission clears it. Removing a flagged waiting or host machine updates the page on the watched removal.
- Run the cross-server browser session tests with real browser cookie handling. Observe that a same-host sign-in waits for the earlier cookie writer and remains authenticated. An obsolete completion should leave the current address, hash and watched records alone.

## Grade

5 out of 10. 0 blockers, 8 majors and 2 minors.
