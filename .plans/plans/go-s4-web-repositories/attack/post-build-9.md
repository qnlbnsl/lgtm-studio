# REVISE

A repository watch removal can make the Switcher open a different repository from the one the reader selected with the keyboard.

The builders and the frozen test writers were OpenAI workers, as is this reviewer. I carried out the post-build review independently of their implementation and test writing. The review covers `23b26fe3..fe8f4f3d` on `sprint-0`, for task `postbuild-go-s4-web-repositories-9` and nonce `a8dad6`.

## Findings

### post-9 1 — MAJOR — The Switcher can open the wrong repository when a result disappears

**Statement.** Plan item 9 requires the keyboard Switcher to open the watched repository selected by the reader. [Plan item 9](../plan.md#L109)

**Evidence and failure scenario.** Confirmed live in Chromium at both widths. Select the middle of three repositories with Arrow Down. Remove the first through the watch while the dialog is open. The third repository becomes selected, and Enter opens it even though the intended repository is still present. With only two repositories, the same update leaves the sole remaining result unselected and Enter keeps the Switcher open. The Switcher stores a numeric index and resets it on query changes. It uses that index directly when the watch changes the results. The frozen tests exercise keyboard selection and watch removal in separate sequences. [Selection and Enter handler](../../../../src/frontend/src/dialogs.tsx#L72) · [Frozen keyboard case](../../../../src/frontend/e2e/repository-navigation.spec.ts#L266)

**Fix.** Keep the active result by its destination so it survives an insertion, removal or reorder elsewhere in the list. Choose a valid visible result when the active result itself disappears. Test both removal cases while the search input has focus.

### post-9 2 — MINOR — The Switcher hides the end of a long watched origin

**Statement.** Plan item 9 requires the Switcher and Checkouts to display a present origin as text. [Plan item 9](../plan.md#L109) · [Origin copy](../plan.md#L213)

**Evidence and failure scenario.** Confirmed live in Chromium at both widths. A grouped Git origin needed 1,367 pixels on one line. The Switcher allocated 379 pixels and hid the rest with an ellipsis. The option exposed no full-value tooltip or disclosure. Two origins that share a long group prefix can therefore look identical in search. The same origin wrapped in Checkouts. The frozen Switcher assertion uses a short origin and checks element visibility. [Switcher option](../../../../src/frontend/src/dialogs.tsx#L95) · [Truncation style](../../../../src/frontend/src/css/base.css#L35) · [Frozen origin assertion](../../../../src/frontend/e2e/repository-navigation.spec.ts#L279)

**Fix.** Let watched origins wrap or offer a full-value disclosure usable by keyboard and touch. Check visibility of the distinguishing suffix with two origins that share a long prefix.

### post-9 3 — MINOR — The watched tree presents a filter that has no effect

**Statement.** Plan item 9 requires the tree to show watched repositories even when the sample filter was selected. [Plan item 9](../plan.md#L109)

**Evidence and failure scenario.** Confirmed live in Chromium at both widths. The reader can still activate “Only projects needing you” in repository mode. The button changes to pressed, and the watched tree shows the same rows. The tree ignores the preference in this mode, while the sidebar still offers the enabled toggle. [Tree filter condition](../../../../src/frontend/src/app.tsx#L80) · [Sidebar toggle](../../../../src/frontend/src/app.tsx#L258)

**Fix.** Show the sample filter only where it applies. Keep the sample preference available when the reader returns to a compatibility workspace.

### post-9 4 — MINOR — Unknown descendants of leaf repository pages open the parent view

**Statement.** The route table assigns unknown pages the existing missing-page view. It lists descendant addresses for Design, Files and Git, and leaf addresses for Command center, Board and Terminal. [Route table](../plan.md#L116)

**Evidence and failure scenario.** Confirmed live in Chromium at both widths. On a watched repository with a folder, `/r/<id>/center/unknown` renders Command center, and `/r/<id>/board/unknown` renders the unavailable Board. The ordinary unknown child `/r/<id>/unknown` displays the correct missing-page body. The route check validates only the first component after the repository identifier. [Repository route dispatch](../../../../src/frontend/src/screens/repo.tsx#L33) · [Frozen deep-link cases](../../../../src/frontend/e2e/repository-navigation.spec.ts#L368)

**Fix.** Check the complete repository address against the listed route families. Keep the listed descendants and older Design links, together with the missing-record and folderless gates.

### post-9 5 — MINOR — A malformed sample address changes the browser title and visible controls

**Statement.** The assignment requires compatibility screens to keep their visible behavior. Plan item 9 applies the new route dispatch in repository mode. [Plan item 9](../plan.md#L109)

**Evidence and failure scenario.** Confirmed live in all three compatibility paths at both widths. Opening `/r/lgtm/constructor` showed Command center in both revisions. The baseline had no active tab and its browser title started with `function Object() { [native code] }`. The reviewed build uses “Command center” in the title, selects that tab and shows its Widgets menu on desktop. The previous title lookup accepted a name inherited from the JavaScript object. The new explicit tab lookup also applies in compatibility mode. This fixes an old malformed-address display incidentally, and the builder's visible text inventory covers repository mode. [Shared route lookup](../../../../src/frontend/src/screens/repo.tsx#L23) · [Compatibility tab rendering](../../../../src/frontend/src/screens/repo.tsx#L70) · [Builder's inventory](../../../../.foundry/go-rewrite/results/s4-web-9-build.md#visible-text-in-repository-mode)

**Fix.** Keep the compatibility route dispatch separate from the watched route changes. Record the malformed-address correction as a deliberate compatibility change if it is adopted.

### post-9 6 — MINOR — The frozen tests accept sample text beside a watched checkout path

**Statement.** The repository Command center must show the watched checkout facts. The assignment requires a repository view to keep sample output out of watched rows. [Plan route table](../plan.md#L133) · [Checkouts implementation](../../../../src/frontend/src/screens/center.tsx#L414)

**Evidence and failure scenario.** Confirmed live in scratch. Adding the sample-style suffix “ · Offline” beside every watched checkout path labeled an online machine's checkout as offline. The frozen tests check the exact path inside its own span and overlook the extra sibling text. The frozen browser run outside the excluded Add repository spec printed `160 passed (1.9m)`. An independent check of the whole value cell failed at both widths. [Frozen checkout assertions](../../../../src/frontend/e2e/repository-navigation.spec.ts#L306)

**Fix.** Assert the complete machine and path text for each checkout row, including two paths on the same machine. Verify that the online row contains only the reported facts.

**Grade.** 7 out of 10.

## Post-build scenarios and visible text

- **Unreviewed mechanism.** By reading the plan and the latest register, the sole marker is the item 6 Add repository retry after a successful reply. The register has advisory post-build checks for the compatibility modes, browser cookies, watched removal, and later deployed work. None is an unreviewed mechanism in these two assigned changes. [Plan marker](../plan.md#L190) · [Review register](round-1-findings.md#L42) · [Post-build scenarios](../plan.md#L281)
- **Compatibility and watch readiness.** Confirmed live. The same built frontend exercised the in-page sample, standalone mock, older-server fixture and advertising fixture. The compatibility paths signed in, used sample projects and the simulation, and opened no repository watch. With a worst-case data query the advertising fixture waited for both initial snapshots and showed watched records only. The delayed sample action, canned post and harness installation stayed out of the watched workspace after the switch. The same-host cookie writer held discovery until completion, and the independent hostname proceeded while the older request was held. [Compatibility cases](../../../../src/frontend/e2e/repository-discovery.spec.ts#L229) · [Readiness cases](../../../../src/frontend/e2e/repository-watch.spec.ts#L188) · [Session cases](../../../../src/frontend/e2e/repository-session.spec.ts#L291)
- **Sample render comparison.** Confirmed live. Against `23b26fe3`, the pinned build matched 120 normalized render captures across the three compatibility paths and two viewport widths. I compared visible text, headings, control and link state, and browser titles with a controlled browser clock. I normalized the standalone mock’s startup-dependent offline time. The captures covered Home, the repository center and tabs, older and missing addresses, the expanded and filtered tree, Switcher searches, and the Machines table, detail and action menu. Six additional captures of the malformed sample address differed as described in post-9 5. The settled in-page runs reported two browser errors at each width in both revisions.
- **Watched repositories.** Confirmed live. Controlled saves opened the watched identifiers from the tree and Switcher after reload. Checkouts showed three folders, including two on the same machine, and preserved the surviving folder when that machine was removed. The missing, folderless and removed routes used their listed bodies. The tested repository pages mounted no sample board, design, editor, agent or terminal output. The Command center showed its empty and unavailable widgets alongside watched checkout facts. Findings above record additional route and Switcher cases. [Navigation cases](../../../../src/frontend/e2e/repository-navigation.spec.ts#L218) · [Checkouts and removal](../../../../src/frontend/e2e/repository-navigation.spec.ts#L306)
- **Machines repairs.** Confirmed live. Online and offline watched machines offered zero reconnect or restart entries and issued zero reconnect calls. Measured zero stayed `0%`, missing loads used dashes, and a missing contact time left the cell blank. A watched save that omitted readings replaced previously reported readings. The expanded offline row displayed no timestamp sentence when contact time was absent. The sample reconnect still triggered its call and returned the machine to Online. The older-server fixture retained its call. [Repair cases](../../../../src/frontend/e2e/repository-routes-review.spec.ts#L59) · [Watch projection](../../../../src/frontend/src/server/sync.ts#L149)
- **Copy and data.** By reading the complete frontend source diff and the builders' inventories, the new display conditions use the existing action labels and empty-state wording from the plan. “Go to Home” is the existing action. I found no newly authored sentence directing the reader. Watched paths and origins render as text. The contract describes the origin as credential-free, and neither renderer turns it into a link or sends it to a log. [Item 9 copy inventory](../../../../.foundry/go-rewrite/results/s4-web-9-build.md#visible-text-in-repository-mode) · [Repair copy inventory](../../../../.foundry/go-rewrite/results/s4-web-4r-build.md#visible-text-in-repository-mode) · [Copy table](../plan.md#L205) · [Repository contract](../../../../src/contracts/proto/lgtm/v1/projects.proto#L104) · [Foundation rule 9](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L370)
- **Integrated observations.** Confirmed live, Compose had no running web service. By reading the pinned server composition, the project service is awaiting registration. The deployed removal and compatibility checks, the real worker before its first harness scan, and the two real folder saves with a held reply remain unverified. The paired history-recovery observation belongs to item 8 and its later dependencies. The browser observations above used controlled watch records. [Pinned server composition](../../../../src/server/internal/app/start.go#L266) · [Deployed and recovery scenarios](../plan.md#L281)

## Commands and mutations

I used Node.js `v26.13.0` and pnpm `12.16.1`. Chromium ran at 1440 by 900 and 390 by 844 pixels. The runs used one scratch copy of the pinned source with `export PATH=~/.nvm/versions/node/v26.13.0/bin:$PATH` and the dependency verification override. The initial scratch command linked a pnpm task-state directory and prevented standalone mock cases from starting. A local task-state directory resolved the fixture error for the reported runs.

| Check | Last output |
|---|---|
| `pnpm exec playwright test --config src/frontend/playwright.config.ts repository-discovery repository-session repository-watch repository-routes repository-navigation` | `144 passed (1.9m)`, exit 0. |
| `pnpm exec playwright test --config src/frontend/playwright.config.ts repository-routes-review repository-discovery-review repository-session-review repository-routes` | `36 passed (47.8s)`, exit 0. |
| `pnpm --filter @lgtm/frontend typecheck` and `pnpm --filter @lgtm/mock-server typecheck` | Each printed `$ tsc --noEmit` and exited 0. |
| `pnpm --filter @lgtm/frontend build` | `✓ built in 1.00s`, exit 0. Vite warned about a generated chunk larger than 500 kilobytes. |
| `deploy/test/checks-s4.sh --only R12,R13` | Exit 1. `not ok 12 - R12 Compose could not report the published website port.` `not ok 13 - R13 Compose could not report the published website port.` |
| `sha256sum -c .foundry/go-rewrite/frozen-s4-web-9.sha256` and `git diff --check 23b26fe3..fe8f4f3d -- src/frontend` | `src/frontend/e2e/repository-navigation.spec.ts: OK`. The diff check exited 0 with no output. |
| Force the watched origin omission flag to false, restore the offline reconnect item, and default omitted readings and timestamps to zero. Run the four affected frozen cases. | `8 failed`, exit 1. Both widths failed the two origin cases, the reconnect case, and the missing-readings case. |
| Add the sample-style offline suffix to watched Checkouts and run all frozen browser specs outside Add repository. | `160 passed (1.9m)`, exit 0. The independent exact row check printed `2 failed`, exit 1. |

The excluded Add repository spec contains 84 cases, including the 78 expected failures. The connected-machine restart decision remains with the owner.

The session startup reported that the workflow toolkit was one commit behind upstream and had four open skill feedback rows.

Suggested first project journal entry for approval. On November 27, the repository review matched 120 sample renders and found that removing a watched row could move the Switcher's keyboard selection to a different repository.
