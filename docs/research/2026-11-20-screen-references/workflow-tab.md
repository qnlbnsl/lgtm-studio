# Workflow tab: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

All of this comes from `design/mock/js/screens/workflow.js`, `design/mock/css/workflow.css` and the phase list in `design/mock/js/data.js`. The review rounds and the test runs have their own file, `review-and-test-results.md`.

- The Workflow tab of a project has five views in a second tab row: Phases, Review rounds, Test runs, End result, and Settings. Each view has its own address under `/p/<project>/workflow`.
- A view's tab shows a dot when something waits. Review rounds gets a warning dot for a stuck statement. Test runs gets an error dot when the latest full run failed.
- The Phases view starts with a status line: "Phase 4 of 8 · Build", the count of sprints done and under way, and a link "2 requests waiting" that goes to Home.
- The build loop has eight phases: Define, Design and teams, Plan, Build, Verify, Review, Hand over, and Keep running. The build loop design note calls these the steps of the default workflow.
- The phases are a vertical rail on the left. A finished phase has a check mark and the time it took. The current phase has a filled dot and the word "Now". A later phase has its number.
- Selecting a phase fills a detail panel on the right. The panel has a heading, a badge (Done, Now, or Not started), and a time such as "Started 3 h ago".
- The detail panel is a list of five labelled parts: "What happens", "Run by", "Your part", "Ends", and "Produced". "Run by" is a row of role badges. "Produced" is a list of rows, each with an icon, a name, a short note, and a link to where the thing lives.
- For a phase that has not started, "Produced" reads "Will produce", and its rows are not links.
- A stuck statement shows above the phases as a request card. It quotes the statement, gives the two reasons it counts as stuck, shows the agent's recommendation, and offers three choices: "Change it", "Keep it", and "Drop it". Each choice has a sentence under its name.
- Under the phases, the project's agents are listed in three groups: Waiting, Working, and Idle. Each row is a link to the agent's thread and shows its role and what it is doing.
- Under the agents is a "Fix sprints" table with the columns Sprint, Why it was added, Merge, and State. The Merge column says "On its own" or "Needs your approval".
- The End result view starts with a status line: a Draft, Ready or Merged badge, "9 of 14 statements hold", and a bar with one segment for each statement.
- When a pull request exists, a band shows its title and number, the two branches, the count of commits and files, and a "Review and merge" button or a Merged badge.
- In the last two phases, two columns appear: "What was built" lists the sprints and their goals, and "Evidence" lists the last full run, the review's state, the statement count, and "Open items".
- The end-result page itself is a numbered list of statements. Each statement has a state badge: Holds, Being built, In review, Stuck, Dropped, or Not started. A note can sit under a statement. A "Request change" button opens the project lead's thread.
- The Settings view has three groups: Tests, Review, and Merging. Each row has a name, a sentence of explanation, and one control: a switch, a command field, or a stepper. The status line says the values are saved to `.lgtm/config` on the project's branch.

## What is planned and not built

- "A better build loop" in `docs/features.md`. The owner said on 2026-11-14 that the current build loop is not good enough. No replacement has been designed.
- "A Workflows tab with a visual editor" in `docs/features.md`. Choosing a workflow for a project remains planned. Every project runs the default build loop today, so the Phases view always shows the same eight phases.
- "Paths by the size of work." in `docs/features.md`. The lead may propose skipping planning for small work. Automatic choice of a path by the workflow is not built. The Phases view has no way to show a skipped phase.
- "A hotfix lane", "A way to bisect" and "A way to freeze interfaces" under "8. The build loop" in `docs/features.md`. All three are requested and not designed.
- "Evaluation rounds that update the model ledger" in `docs/features.md`. Not built. An evaluation round would run at the end of each key task and at the end of a project. It has no place in the Workflow tab yet.
- "Project settings controls awaiting engine support" in `docs/features.md`. "Write tests first" and "Agent review" each need a stored project policy and enforcement before the live website can offer them.
- "Pushing a finished project and opening its pull request" in `docs/features.md`. Its "Watch" step says the project shows the pull request's number, a link, the state of its checks, and its review state. The mock's pull request band shows the number, the branches, and the counts of commits and files. It shows no state of checks. The section says automatic push triggers and editing of the pull request's text remain planned.
- "The Project view, decided on 2026-11-20" in `design/DESIGN.md`. The end result is a document on the right of the Project view, and the top line shows six stages. The same statements therefore appear in two places: the Project view and the Workflow tab's End result view.
- "The view carries no instructions" in the same section of `design/DESIGN.md`. The Phases detail ("What happens", "Your part"), the three choices on the stuck card, and every Settings row carry explaining sentences today.
- The build loop design note says there is no hard limit on review rounds since 2026-11-11, and each round after the fifth asks the owner. The Settings view still limits "Most review rounds" to 6. The note and the mock differ.

## How other products do it

### Vercel: a deployment's stages as foldable rows

[Mobbin screen](https://mobbin.com/screens/bcd88c7b-6a42-4e01-b9ea-01fac8247bd2?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The top card is "Deployment Details". It holds a preview picture and labelled facts: Created, Status, Duration, Environment, Domains and Source, with the branch and the commit. Share, Logs and Visit buttons sit in the card's head. Below it, the stages are single rows that fold: Deployment Settings, Build Logs, Deployment Summary, Deployment Checks, and Assigning Custom Domains. Each row ends with its duration and a state mark, which is a tick when done and a clock when it has not run.

Difference from LGTM Studio: Vercel's stages are rows that open in place, and several can be open at once. LGTM Studio's phases are a rail that shows one phase's detail at a time.

### Vercel: the same page when the build failed

[Mobbin screen](https://mobbin.com/screens/ff81f1e9-25b1-46f9-8448-31fa40a77e4b?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The preview picture is replaced by a red-edged box titled "Build Failed" with the one line that failed. The Status fact reads "Error". The head button changes from Visit to Redeploy. The Build Logs row is already open and shows the log with a count of errors and warnings and a search field.

Difference from LGTM Studio: Vercel lifts the one failing line to the top of the page and opens the failed stage for the reader. LGTM Studio marks the tab with a dot, and the owner opens the Test runs view to find the failure.

### Laravel Cloud: a deployment in progress, as two groups of steps

[Mobbin screen](https://mobbin.com/screens/f9a999ee-f498-4a02-b17f-be383d866950?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The head line holds the deployment's short commit, the word "Building", the commit message, and the elapsed time. Cancel and Visit buttons sit at the right. The body has two groups, "Build logs" and "Deployment logs". Each group has a head row with a spinner and the current activity in words, and under it three or four step rows that each end with "Pending" and an arrow. A small bar floats at the bottom of the window with the deployment's name and a Cancel button.

Difference from LGTM Studio: each group's head row says what is happening now in words. LGTM Studio's rail says "Now" on the current phase, and what is happening is further down in the agents list.

### Amazon Web Services: one progress bar over a foldable list of steps

[Mobbin screen](https://mobbin.com/screens/810e5974-b4c8-4979-837c-449abfb6f7fc?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The page shows one line with a spinner and the current step, a progress bar, and "14%". Under it, a "Details" twist opens a plain list of about twenty steps. Finished steps are green with a tick, the current step has a progress icon, and later steps are black with a clock icon. Finished steps that made something end with a link to it.

Difference from LGTM Studio: this is the densest form of the same idea, with one line per step and no detail panel. It has no place for "your part" or for a decision.

### Mercury: a checklist with the current step open, and a small time line beside it

[Mobbin screen](https://mobbin.com/screens/00b75359-beff-466d-98f5-6b893ee918ea?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

Four finished steps are single rows with green ticks. The fifth step has a number and is open in place, with its text, its content and a Next button. A small card on the right, "Application timeline", lists three points on a vertical line: Apply with its date, In review with "Approx. 1 day", and Account ready.

Difference from LGTM Studio: Mercury separates what the user must do (the checklist) from where the whole process stands (the time line). LGTM Studio mixes the two in one detail panel, where "Your part" is one of five parts.

### Wix: setup steps as rows with one button each

[Mobbin screen](https://mobbin.com/screens/ba505e36-28c4-4b67-a1f5-d59e715d1e37?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

Six steps are rows in one card. The finished step is struck through. Each other row has its name on the left and one button on the right that names the action, such as "Design App" or "Connect Accounts". The current row is tinted and its button is filled. A counter at the top right reads "1/6" with a small bar.

Difference from LGTM Studio: every row's button names its outcome, which matches the owner's rule for buttons. LGTM Studio's phase rows carry no action; the owner's actions are request cards elsewhere.

### GitHub: the foot of a merged pull request

[Mobbin screen](https://mobbin.com/screens/4f98de0b-6dad-4867-8f10-b7521d1974fd?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A band sticks to the top with a Merged badge, the title, and one line saying who merged how many commits into which branch. The conversation ends in a box with a merge icon that reads "Pull request successfully merged and closed", with a "Delete branch" button. Above it, a resolved review thread shows the file, the changed lines, the comment, and "marked this conversation as resolved".

Difference from LGTM Studio: GitHub's merge box is the last item of a history, so the evidence is whatever came above it. LGTM Studio's End result view puts the pull request first and lists the evidence as three named lines under it.

## Options for LGTM Studio

1. Turn the phases into rows that open in place. Each phase would be one row with its mark, name, duration and state, and opening it would show what it produced, as Vercel's stage rows do. Rows open and close on their own. The current phase starts open. This removes the rail and the separate detail panel, and it fits a phone without a second layout.
2. Lift what waits to the top of the Phases view. A failed full run or a stuck statement would appear as one box with the one line that matters and one button, as Vercel's "Build Failed" box does. The stuck card already does this for statements. A failed run does not.
3. Say what is happening now in the phase's own row. The current phase's row would carry the lead's current activity in words, as Laravel Cloud's group head does. The cost is that an activity line changes often, so the row would move while the owner reads.
4. Split "your part" from the phase list. The owner's open decisions would sit in one list at the top, and the phases would become a small time line at the side, as on Mercury. This fits the Project view's rule that waiting decisions sit together. The risk is a third place for decisions, after Home and the Project view.
5. Cut the detail panel to labels and values. "What happens", "Your part" and "Ends" are explaining sentences and would go. "Run by" and "Produced" would stay. This follows the owner's rule against instructional copy and no reference. The cost is that a new user loses the only description of the loop; the Workflows page could hold it.
6. Collapse the eight phases to one progress line with a foldable list. The view would show "Phase 4 of 8 · Build" with a bar, and the list of phases would fold under it, as on the Amazon Web Services page. The agents and the fix sprints would move up. This suits an owner who rarely needs the phase detail.
7. Remove the End result view from the Workflow tab. The Project view now holds the end result as a document. The pull request band and the evidence lines would move there or to the Hand over phase's row. The risk is losing the statement-by-statement state badges if the Project view's document does not show them.

## Ideas that are not in the feature list

- A duration on every finished phase and step, always in the same column. Reference: Vercel deployment details.
- A rough estimate on a phase that has not finished, such as "Approx. 1 day". Reference: Mercury.
- A small floating bar that follows the owner across pages while a run is in progress, with a Cancel button. Reference: Laravel Cloud.
- A count of recommendations on the settings row, such as "3 Recommendations". Reference: Vercel "Deployment Settings".
- A link from each finished step to the thing it made, on the step's own line. Reference: Amazon Web Services. LGTM Studio has this only inside the detail panel.

## Searches that were run

- "Vercel deployment detail page with build steps and status". Eight results, all from Vercel. Three were usable for phases. The rest showed the project overview and the deployments list.
- "CircleCI pipeline page with workflow jobs and their status". No results. CircleCI was not found on Mobbin.
- "progress page with a vertical list of stages and details of the current stage". Eight results, from Flodesk, Deputy, Amazon Web Services, Laravel Cloud, Gamma, Mercury, Wix and Hims. Four were usable. The others were sign-up forms and import dialogs.
- "GitHub pull request page with review comments and merge checks". Eight results, all from GitHub. One was used here for the End result view. No result showed a list of checks on an open pull request.
- No search was run for the workflow's settings. The budget of 14 searches was spent.
