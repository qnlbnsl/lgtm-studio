# REVISE

A repository watch update can move the selected result below the Switcher's 14-result limit and make Enter open a different repository.

The builders and frozen test writers were OpenAI workers, as is this reviewer. I neither built the website nor wrote the frozen tests. This review covers `a2b09fd1..c73250c1` on `sprint-0`, for task `postbuild-go-s4-web-repositories-7` and nonce `e392ef`. Source lines refer to the pinned revision.

## Findings

### post-7 1. MAJOR. The Switcher loses a selected destination when it falls below the visible limit

**Statement.** The plan and the repair of post-9 1 require the Switcher to preserve a selected repository by its destination when other results change. [Plan item 9](../plan.md#L109) · [Prior finding](post-build-9.md#post-9-1--major--the-switcher-can-open-the-wrong-repository-when-a-result-disappears)

**Evidence and failure scenario.** Confirmed live in Chromium at both widths. With 14 matches, select Cobalt at the bottom with the arrow keys. Send a save for a new repository whose name matches more of the existing query. Cobalt remains watched and still matches the query. Its rank becomes 15 and Enter opens the newly inserted Priority repository. The lookup runs after the results are sliced, and the layout effect saves the first visible result as the new selection. The frozen insertion test starts with three results. [Cutoff and selection](../../../../src/frontend/src/dialogs.tsx#L76) · [Enter handler](../../../../src/frontend/src/dialogs.tsx#L96) · [Frozen insertion case](../../../../src/frontend/e2e/repository-navigation-review.spec.ts#L75)

**Fix.** Resolve the active destination against the full matching results before limiting the display. Keep that destination in the rendered window so the highlighted result and Enter still agree. Test a ranked insertion at the cutoff and a later removal of the selected repository.

### post-7 2. MINOR. Removing the selected repository leaves New project at a silent blank choice

**Statement.** Item 7 requires New project to derive its options from the current store and validate the selected repository. It already has a named New repository branch for an empty inventory. [Plan item 7](../plan.md#L196)

**Evidence and failure scenario.** Confirmed live at both widths. Open generic New project with one attached repository and an online machine, then enter a goal. Remove that repository through the watch. The Repository selector goes blank even though it contains “New repository…”. The name field and attachment action are absent, the footer gives zero validation feedback, and the button and Enter both stop at the stale choice. The repository state is initialized on mount, while the options continue to follow the watch. Choosing “New repository…” manually recovers the form and preserves the goal. [Repository state and validation](../../../../src/frontend/src/dialogs.tsx#L159) · [Options and name field](../../../../src/frontend/src/dialogs.tsx#L210) · [Watch removal](../../../../src/frontend/src/server/repositories.ts#L31)

**Fix.** Reconcile a removed selection with a current option or a deliberate empty state while preserving the draft. Reuse “New repository…” when the last repository disappears and exercise the removal with the dialog open.

### post-7 3. MINOR. An older server without Studio shows a different sample New project screen

**Statement.** The plan keeps the sample workspace for an older server. The sprint design keeps what these screens show in all three compatibility modes. [Compatibility plan](../plan.md#L82) · [Sprint design](../../../../docs/specs/2026-11-27-running-an-agent-design.md#L269)

**Evidence and failure scenario.** Confirmed live at both widths with an older-server fixture that reported Buildbox and Laptop. I opened New project from the sample repository row. That action produced a page error and zero dialogs at the base. The reviewed revision opens the dialog and disables Create project because Studio, its compatibility default, is missing. The new missing-machine handling applies in the shared component. The builder's inventory describes the new display conditions for repository mode. [Repository entry](../../../../src/frontend/src/app.tsx#L91) · [Machine selection and guard](../../../../src/frontend/src/dialogs.tsx#L159) · [Previous lookup helper](../../../../src/frontend/src/data.ts#L523) · [Builder's copy inventory](../../../../.foundry/go-rewrite/results/s4-web-7-build.md#visible-text-and-its-authority)

**Fix.** Record the compatibility recovery as a visible exception for approval. Keep the compatibility behavior pinned separately if the old display must be preserved.

### post-7 4. MINOR. The frozen deadline assertions allow a 59-second folder request

**Statement.** The post-6 2 repair requires each inspection or save attempt to end after 30 seconds. The production deadline is 30 seconds. The frozen assertions advance up to 60 seconds. [Prior finding](post-build-6.md#post-6-2-major-a-silent-connection-can-leave-add-disabled-indefinitely) · [Production deadline](../../../../src/frontend/src/server/folders.ts#L15) · [Frozen deadline bound](../../../../src/frontend/e2e/add-repository-review.spec.ts#L31)

**Evidence and failure scenario.** In scratch, changing the shared deadline to 59 seconds leaves a silent request busy for nearly twice the allowed interval. All 284 frozen browser cases passed with that mutation. The independent inspection and save cases at both widths rejected the changed deadline header. The frozen cases wait until 60 seconds before checking release or retry. [Frozen clock helper](../../../../src/frontend/e2e/add-repository-review.spec.ts#L188) · [Frozen silence cases](../../../../src/frontend/e2e/add-repository-review.spec.ts#L252)

**Fix.** Assert that each silent attempt is still pending just before 30 seconds and that its specific retry or release occurs as the clock crosses that deadline. Check the same boundary on the inspection retry and the manual save replay.

**Grade.** 7 out of 10.

## Previously unreviewed mechanism and repair verdicts

- **Known save with a lost watch.** Confirmed live at both widths. This is the plan's sole unreviewed mechanism. With the saved row retained after disconnect, I ran the case that fails one replacement watch and leaves the next one silent. The form stayed open, restarted only the repository watch, and released Add at six seconds. Pressing Add again rechecked the watch with the same bound. When a later snapshot supplied the matching identities, there was one return and one saved identity. The browser sent one save in total. [Plan marker](../plan.md#L190) · [Review register](round-1-findings.md#L24) · [Browser scenario](../../../../src/frontend/e2e/add-repository.spec.ts#L616)
- **Post-9 1 partially closes.** Confirmed live. Destination-based selection survives the original earlier-row removal, reordering, and insertion when the selected result stays in the displayed set. Removal of the selected row activates the remaining visible result. The cutoff case remains open as post-7 1. [Frozen cases](../../../../src/frontend/e2e/repository-navigation-review.spec.ts#L60)
- **Post-9 4 closes.** Confirmed live. Unknown descendants of Command center, Board and Terminal show the missing-page body with a Home link and zero repository tabs. The listed Design, Files and Git routes, older Design links, and the missing and folderless record gates hold. Extra probes of unknown descendants below Design Screens, an individual screen, Browser and Architecture also reached the missing page. [Route guard](../../../../src/frontend/src/screens/repo.tsx#L33) · [Frozen route cases](../../../../src/frontend/e2e/repository-navigation-review.spec.ts#L116)
- **Post-6 1 closes.** Confirmed live. A decoded inspection or save answer released after the wrapper check and after a path or machine edit did not change the current input, inspection rows, dialog, or success feedback. The same watch still owns any record actually saved. Both dialog continuations now repeat the attempt check before using the answer. [Inspection guard](../../../../src/frontend/src/screens/service.tsx#L619) · [Save guard](../../../../src/frontend/src/screens/service.tsx#L714) · [Frozen race cases](../../../../src/frontend/e2e/add-repository-review.spec.ts#L210)
- **Post-6 2 closes in the code.** Confirmed live at both widths. An independent browser probe saw the 30,000-millisecond Connect deadline on the initial inspection, its automatic retry, the save and its manual replay. Each request was pending 50 milliseconds before the deadline and retried or settled when the clock crossed it. The retry and replay reused their request keys and bodies. The frozen test gap is post-7 4. [Shared call options](../../../../src/frontend/src/server/folders.ts#L68) · [Inspection retry](../../../../src/frontend/src/screens/service.tsx#L619) · [Save replay](../../../../src/frontend/src/screens/service.tsx#L697)

## Item 7, compatibility and deployed checks

- **Dialog entry and return.** Confirmed live. The advertising fixture's sidebar plus keeps its accessible name and tooltip and selects the first online machine. Both Machines actions preserve their opener. The two entry points saved distinct identities, and the reloaded tree and Switcher opened both resolved paths. The checkout row checks compare the whole displayed value. With zero machines or only offline machines, the Add repository folder field stays editable and submit stays disabled. New project also has a blank machine choice and no usable attachment action. An arriving online machine becomes the default. An unrelated save preserves the manual machine. [Sidebar](../../../../src/frontend/src/app.tsx#L267) · [Entry and reload](../../../../src/frontend/e2e/repository-entry.spec.ts#L271) · [Whole-row assertion](../../../../src/frontend/e2e/repository-navigation.spec.ts#L306) · [Empty choices](../../../../src/frontend/e2e/repository-entry.spec.ts#L418)
- **Project validation and wording.** Confirmed live. Back and a matching watched attachment restore the goal, machine and repository. I also checked Back with a hidden draft repository name. Missing goal, missing new name and an unattached repository prevent submission by both button and Enter. A valid submission keeps the fields and empty lead controls mounted, puts the message in the body after the form, and sends zero project or named-repository writes. The response persists until edit or close. By reading the production diff, the only new sentence is the approved “Projects are not connected to this server yet.” The new missing-page branch reuses existing copy. I found zero new instruction sentences. [Item 7 and copy approval](../plan.md#L196) · [Exact approved sentence](../plan.md#L222) · [Dialog response](../../../../src/frontend/src/dialogs.tsx#L175) · [Validation and return cases](../../../../src/frontend/e2e/repository-entry.spec.ts#L313)
- **Sample comparison.** Confirmed live. Against `a2b09fd1`, 78 render captures matched across in-page sample, standalone mock and an older server with Studio, at both widths. Captures compared visible text, browser titles, control labels, options, values and state. They covered Track repository, three New project states, three Switcher states and six repository addresses. The extra older-server fixture without Studio differs as described in post-7 3. Issue 1698's malformed-address behavior was already present at the base. [Compatibility discovery](../../../../src/frontend/src/server/sync.ts#L255) · [Three-mode tests](../../../../src/frontend/e2e/repository-discovery.spec.ts#L229)
- **Post-build browser and stack checks.** Confirmed live in the browser fixtures. The advertising path waited for both snapshots under worst-case sample parameters. Held sample work stayed out of the watched workspace. Same-host discovery waited for its cookie writer, and a late completion on another hostname left the selected server and watches alone. The fixtures exercised the two entry points, three inspection rows, a held save response, keys, watched identities and reload. The isolated Compose check could not resolve a published website port. By reading the pinned Go composition, it registers the session, machine and link services and discovery has no project service advertisement. The real-machine facts before the first harness scan, deployed orphan and survivor check, and the two real saves in sprint check R14 remain unverified here. The history-recovery check belongs to the later recovery item. [Post-build scenarios](../plan.md#L281) · [Deployment gate](../plan.md#L246) · [Go registration](../../../../src/server/internal/app/start.go#L266) · [Discovery reply](../../../../src/server/internal/api/session.go#L38)

## Commands and scratch mutations

I used Node.js `v26.13.0` and pnpm `12.16.1`. Chromium ran at 1440 by 900 and 390 by 844 pixels. The browser runs and mutations used one disposable copy of the pinned source.

| Check | Last output |
|---|---|
| `pnpm exec playwright test --config src/frontend/playwright.config.ts add-repository -g 'a lost watch ignores retained rows'` | `2 passed (5.0s)`, exit 0. |
| `pnpm exec playwright test --config src/frontend/playwright.config.ts folder-calls add-repository repository-entry repository-discovery repository-session repository-watch repository-routes repository-navigation` | `284 passed (3.1m)`, exit 0. This combines both plan selectors and includes the frozen repair cases. |
| Compare the base and reviewed sample captures. | `8 passed (9.7s)` at the base and `8 passed (9.8s)` at the reviewed revision. The planned captures matched. The extra no-Studio fixture exposed the compatibility change. |
| Repeat the older-server comparison using the sample repository row's New project action. | `2 passed (4.6s)` at the base and `2 passed (4.8s)` at the reviewed revision. The captured dialog and page error outcomes differed at both widths. |
| Probe exact folder deadlines, cutoff selection, repository removal, draft preservation, and Design leaf routes. | `16 passed` and `4 failed (22.0s)`. Cutoff selection and the unhandled repository removal produced the four failures. |
| `pnpm --filter @lgtm/frontend typecheck` and `pnpm --filter @lgtm/mock-server typecheck` | Each printed `$ tsc --noEmit` and exited 0. |
| `pnpm --filter @lgtm/frontend build` | `✓ built in 1.08s`, exit 0. Vite warned about a chunk over 500 kilobytes. |
| `deploy/test/checks-s4.sh --only R12,R13` | Exit 1. `not ok 12 - R12 Compose could not report the published website port.` `not ok 13 - R13 Compose could not report the published website port.` |
| Check the five frozen test manifests and run `git diff --check a2b09fd1..c73250c1 -- src/frontend`. | All five manifests printed `OK`. The diff check exited 0 with no output. |
| Prefer the first online machine over a registered opener. | `1 failed`. The machine detail opened Add repository on the wrong machine. [Assertion](../../../../src/frontend/e2e/repository-entry.spec.ts#L284) |
| Clear the restored goal on mount. | `1 failed`. The Back action returned with an empty goal. [Assertion](../../../../src/frontend/e2e/repository-entry.spec.ts#L370) |
| Reset the manual machine to the default whenever the watch changes the list. | `1 failed`. The unrelated save replaced the manual machine. [Assertion](../../../../src/frontend/e2e/repository-entry.spec.ts#L471) |
| Restore the lead prerequisite. | `1 failed`. The Create project button stayed disabled on a valid form with no lead. [Assertion](../../../../src/frontend/e2e/repository-entry.spec.ts#L389) |
| Extend each folder deadline to 59 seconds and run the independent 30-second probe. | `4 failed`. Both procedures sent `59000` in place of `30000` at both widths. |
| Run the full frozen browser selection with the same 59-second mutation. | `284 passed (1.8m)`, exit 0. |

The session startup reported that the session tooling was one commit behind upstream and that four skill feedback rows were open.

Suggested first project journal entry for approval. The sprint 4 website review passed 284 frozen browser cases and reproduced a Switcher result limit that could still send keyboard users to a different repository after a watch update.
