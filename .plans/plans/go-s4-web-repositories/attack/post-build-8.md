# REVISE

An online host marked Out of sync is included in the Machines count and can be selected in both dialogs.

The test writers, builders, and reviewer are workers of the same vendor, OpenAI. I did not build the website or write the frozen tests. I reviewed the website changes in `5fb5fb16..dd5c4fa1` on `sprint-0`. The task identifier is `postbuild-go-s4-web-repositories-8`. The nonce is `7305bb`.

## Findings

### post-8 1. MAJOR. An online machine marked Out of sync still counts as online and can be selected for folder work

**Statement.** The assignment summary excludes recovering machines from the count and choices. Item 8 of the plan says to keep watched standing for counts and actions, and explicitly excludes flagged waiting records. The frozen browser case encodes the plan’s narrower reading for a flagged host. [Assignment](../../../../.foundry/go-rewrite/tasks/postbuild-go-s4-web-repositories-8.md#L12) · [Plan item 8](../plan.md#L264) · [Frozen expected count and action](../../../../src/frontend/e2e/machine-recovery.spec.ts#L129)

**Evidence and failure scenario.** Confirmed live at both widths. With one ordinary online worker, one flagged online host, two offline workers, and one flagged waiting machine, Machines displayed “2 of 4 online”. Add repository opened from the flagged host with that host selected. Generic New project also selected it because it was the first online machine. Both choices retained the flagged offline worker as a disabled option. The flagged waiting machine was absent from both choices and from the count. By reading, the server projects recovery independently of online standing, and the website calculates eligibility from standing alone. [Machines count](../../../../src/frontend/src/screens/service.tsx#L313) · [Default machine](../../../../src/frontend/src/server/folders.ts#L5) · [Add repository choice](../../../../src/frontend/src/screens/service.tsx#L798) · [New project choice](../../../../src/frontend/src/dialogs.tsx#L207) · [Server projection](../../../../src/server/internal/machines/project.go#L36)

**Fix.** Reconcile the task and plan on whether a flagged registered machine should remain usable. If the assignment’s exclusion applies to every flagged record, use the recovery flag in the count, both choice lists and defaults, and the folder action on the machine row. Update the frozen host expectation to assert that policy.

### post-8 2. MINOR. An open removal confirmation keeps its pairing instruction after the watch flags the machine

**Statement.** The flagged removal confirmation uses the two approved facts. The plan omits the pairing instruction when the recovery flag is true. [Approved confirmation copy](../plan.md#L233) · [Owner decision](../../../../docs/specs/2026-11-27-running-an-agent-design.md#L50)

**Evidence and failure scenario.** Confirmed live at both widths. Open Remove machine for an unflagged offline worker, leave the confirmation open, and deliver a saved frame for that worker with recovery true. The row changes to “Out of sync” while the confirmation still ends with “Pair it again to bring it back.” The action stores the body as a string when it opens, and the confirmation renders that stored body after the watch updates the row. [Removal action](../../../../src/frontend/src/screens/service.tsx#L318) · [Stored confirmation](../../../../src/frontend/src/actions.ts#L58) · [Rendered body](../../../../src/frontend/src/ui.tsx#L279)

**Fix.** Derive the body from the current watched record by machine identifier, or dismiss the confirmation when its target becomes flagged. Exercise the saved frame while the confirmation is open.

### post-8 3. MINOR. The frozen tests accept a recovery warning that sticks on a still-waiting machine

**Statement.** Item 8 requires saved frames to reflect a later ready admission. A later admission can clear recovery while the machine still waits for approval. The server’s waiting replay case expects precisely that saved frame. [Plan item 8](../plan.md#L256) · [Contract field](../../../../src/contracts/proto/lgtm/v1/clients.proto#L73) · [Waiting replay case](../../../../src/server/internal/link/recovery_v2_test.go#L246)

**Evidence and failure scenario.** Confirmed live in scratch. I changed the waiting saved-frame projection to keep the old true flag when a new waiting record has a false flag. All 292 frozen browser cases passed. An independent probe sent that frame and found one Out of sync row after the saved update, where it expected zero. The same probe passed on the reviewed code at both widths. The frozen waiting case sends a second true flag before removal. Its clearing cases take a registered machine online. [Saved-frame projection](../../../../src/frontend/src/server/sync.ts#L157) · [Waiting save and removal](../../../../src/frontend/e2e/machine-recovery.spec.ts#L173) · [Registered clearing](../../../../src/frontend/e2e/machine-recovery.spec.ts#L249)

**Fix.** Send a saved frame carrying false for an already flagged waiting machine. Assert that its diagnostic row disappears and its waiting record remains.

**Grade.** 7 out of 10.

## Previously unreviewed mechanism and item 8 behavior

By reading, the plan’s sole unreviewed marker belongs to item 6. I ran its browser scenario at both widths because the deadline repair touches the same dialog. Confirmed live, a known save with a lost watch ignored the retained row and refreshed only the repository watch. A silent replacement released Add at the six-second join deadline. A subsequent click rechecked the watch within the same bound and sent no second save. A later matching snapshot returned once with one saved identity. [Plan marker](../plan.md#L190) · [Review register](round-1-findings.md#L24) · [Browser case](../../../../src/frontend/e2e/add-repository.spec.ts#L616)

Confirmed live, flagged offline, waiting, and host rows show the exact status and one approved reason in their opened detail. A record that omits the flag keeps Offline and its reported facts. An unrelated save and a replacement snapshot preserve the diagnosis. The later ready save and a replacement snapshot with a false flag clear it. A late approval neither clears the flag nor restores a removed row. Remove machine is reachable for all three flagged standings. The success reply leaves each row in place until the watched removal arrives. The waiting record remains outside counts and choices. Findings post-8 1 and post-8 2 describe the two exceptions in adjoining behavior. [Done-when row](../plan.md#L252) · [Watch projection](../../../../src/frontend/src/server/sync.ts#L153) · [Snapshot and removal tests](../../../../src/frontend/e2e/machine-recovery.spec.ts#L129) · [Approval and reconnect tests](../../../../src/frontend/e2e/machine-recovery.spec.ts#L227)

## Verdict on each repair after review 7

- **Post-7 1 closes.** Confirmed live at both widths. A higher-ranked watched insert pushes the selected repository past the 14-result limit. The selected repository stays visible and active, and Enter opens it. Removing that repository activates a remaining visible result. Searching only the visible slice again failed both the visibility and Enter assertions. [Switcher implementation](../../../../src/frontend/src/dialogs.tsx#L76) · [Frozen cutoff case](../../../../src/frontend/e2e/repository-entry-review.spec.ts#L67)
- **Post-7 2 closes.** Confirmed live at both widths. Removing the last repository selects “New repository…”, preserves the goal, and allows the existing unavailable response after the owner supplies a name. By reading, the first surviving repository is the fallback when the list still has entries. Retaining the stale selection failed the selected-option assertion. [Fallback implementation](../../../../src/frontend/src/dialogs.tsx#L163) · [Frozen removal case](../../../../src/frontend/e2e/repository-entry-review.spec.ts#L103)
- **Post-7 4 closes.** Confirmed live at both widths. The frozen checks observe each silent attempt at 29,950 and 30,050 milliseconds. They cover the first inspection, its automatic retry, the first save, and its manual replay. The retry and replay retain their request keys. Mutating the shared deadline to 59 seconds failed all four browser cases at their retry or release assertions. A 29-second deadline failed all four at the before-deadline assertions. [Shared deadline](../../../../src/frontend/src/server/folders.ts#L15) · [Clock boundary](../../../../src/frontend/e2e/add-repository-review.spec.ts#L186) · [Silent attempts](../../../../src/frontend/e2e/add-repository-review.spec.ts#L255)

## Sample screens and approved text

Confirmed live, 72 captures matched between the base and reviewed revisions across in-page sample, standalone mock, and the older-server fixture, at both widths. The comparison covered the Machines list, expanded host and offline worker, action menus, remove confirmation, Add repository, New project, and the Switcher. It compared visible text, titles, controls, options, selected states, and dimensions. I normalized only the mock’s clock text in four captures because its sample timestamp is derived from process start time. [Compatibility fixture](../../../../src/frontend/e2e/repository-discovery.spec.ts#L229) · [Mock sample clock](../../../../src/mock-server/src/engine.ts#L61)

By reading the production diff, “Out of sync” and “This machine's saved history is older than the server's.” are the new displayed wording. New flagged removal confirmations reuse the approved existing wording. The patch adds zero instructional sentences. The stale open confirmation in post-8 2 retains an existing instruction. [Approved wording](../plan.md#L232) · [Rendered wording](../../../../src/frontend/src/screens/service.tsx#L177) · [Confirmation branch](../../../../src/frontend/src/screens/service.tsx#L322)

## Browser runs and recovery mutations

The pinned scratch copy ran Chromium at 1440 by 900 and 390 by 844 pixels with Node.js `v26.13.0` and pnpm `12.16.1`. The commands below used `pnpm --config.verify-deps-before-run=false exec playwright test --config src/frontend/playwright.config.ts`. An initial scratch run could not launch the standalone mock through a symbolic link at the workspace dependency directory. A real directory with links to the installed packages resolved the setup issue.

| Filters | Exit and final output |
|---|---|
| `machine-recovery` | Exit 0. `4 passed (9.3s)` after restoring the scratch mutations. |
| `machine-recovery folder-calls add-repository repository-entry` | Exit 0. `130 passed (1.6m)`. This includes all four item 8 cases and all frozen repair cases. |
| `repository-discovery repository-session repository-watch repository-routes repository-navigation` | Exit 0. `162 passed (1.9m)`. |
| Independent recovery eligibility and open confirmation probe | Exit 1. `2 failed`. Both widths reproduced post-8 1 and post-8 2. |
| Independent waiting clear and long-hostname removal probes | Exit 0. `4 passed (5.6s)`. The waiting clear probe rejected the sticky-flag mutation with `1 failed`. |
| Sample capture probe at each revision | Exit 0. `6 passed` on each revision. |

Each named recovery mutation exited 1 with `1 failed` in its targeted desktop browser case. Dropping the flag on registered saves, clearing it on the old approval reply, and clearing it on an unrelated save each failed “Name from watch shows the watched recovery status.” Dropping it in the waiting projection failed “Lagging waiting has exactly one watched row.” [Registered transition assertions](../../../../src/frontend/e2e/machine-recovery.spec.ts#L236) · [Waiting assertion](../../../../src/frontend/e2e/machine-recovery.spec.ts#L98)

## Post-build checks awaiting the joint server and website step

- **Stack and Website, sprint check R13.** Run the frozen one-build compatibility check against the advertising candidate server alongside the two sample transports. On the compatibility paths, observe sign-in and sample work with zero repository calls. On the advertising path, observe sign-in and the real workspace after both snapshots, including with the worst-case sample query. The older-server case, compatibility procedure traces, and the delayed sample action across a server switch passed with the browser fixtures. [Website post-build row](../plan.md#L285) · [Stack evidence](../../go-s4-stack-checks/plan.md#L190)
- **Stack, Projects, and Website, sprint check R14.** Use the deployed same-origin candidate at both widths and save one real checkout from the sidebar and another from Machines. Compare Base branch, Remote, and Status with the checkout. Hold a real add response and correlate the request key, returned repository, folder, and machine identifiers, watched identifiers, and resolved paths. After reload open both from the tree and the Switcher. Expect one entry for each identity and zero synthetic activity or terminal. [Website post-build row](../plan.md#L286) · [Deployed gate](../plan.md#L246)
- **Website and Stack, sprint check R12.** Inspect a real worker before its first harness scan and check that its detail shows only reported facts. Expect zero invented version comparisons, installations, or account statuses. On the deployed stack remove a machine with an open operation and observe the orphan disappear, the surviving folder remain, and the operation fail. Check both orderings with a delayed watch and observe that a removed-machine action never resumes. The browser fixtures covered the watched facts and the two orderings. [Website post-build row](../plan.md#L288) · [Deployed removal check](../../go-s4-stack-checks/plan.md#L191)
- **Orchestrator, Link, and Website, history recovery.** Run the merged `TestSprintFourHistoryRecovery` with the client, server, and PostgreSQL. Observe the flag on an open Machines page at both widths. Reconnect and send an unrelated save while flagged, then observe clearing on later ready admission and removal of a flagged row. Exercise Remove machine for waiting and host records. The named merged case was absent from the reviewed revision. [Website post-build row](../plan.md#L289) · [Merged case requirements](../../go-s4-stack-checks/plan.md#L195)

The Website-owned cookie-ordering row passed in Chromium with browser cookies. Discovery on a second port on the same hostname waited for the first port’s cookie writer and sent its resulting cookie. A late completion on another hostname left the selected address, route, and authenticated watches intact. [Website post-build row](../plan.md#L287) · [Cookie cases](../../../../src/frontend/e2e/repository-session.spec.ts#L292)
