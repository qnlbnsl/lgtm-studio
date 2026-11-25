# Review findings and test runs: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

All of this comes from `design/mock/js/screens/workflow.js` and `design/mock/css/workflow.css`. The scoring rules come from `docs/specs/2026-11-05-build-loop-design.md`, section "When a review stops and pings the owner".

- Review rounds and Test runs are two views of a project's Workflow tab, at `/p/<project>/workflow/review` and `/p/<project>/workflow/runs`.
- A review has a subject (a sprint or a branch), a kind (Plan review or Code review), the team that runs it, and a state badge: Being scored, Round running, Stuck, or Clean. A project with more than one review gets a switch between them.
- The rounds are a row of six boxes. A finished round's box holds a stacked bar, the round's name, and its points. The bar's segments are blockers, majors and minors, sized by points. A round with no findings shows a check mark and "Clean".
- A round that has not run is a dashed box with a word: "Next", "Running now", "Only if needed", "Not needed", or "Needs your approval" for rounds 5 and 6.
- A legend under the rounds gives the scale: a blocker is 9 points, a major is 3 points, and a minor is 1 point.
- Selecting a round fills the findings table under it. Its columns are Finding, Statement, Severity, What the reviewer found, Points, and State.
- Each finding has an identifier such as F-03 and is tied to a numbered statement of the end-result page. The statement number is a link to the End result view, and hovering it shows the statement's text.
- A finding's state is Open or Fixed. A switch above the table filters to All, Open or Fixed, with a count on each. An open major or blocker gets a warning edge on its row.
- A statement that keeps drawing findings is "stuck". A stuck card above the rounds quotes the statement, gives its points and share of the round, and offers "Change it", "Keep it" and "Drop it". The statement's rows in the table carry a Stuck badge.
- The Test runs view follows GitHub Actions, as the code comment says. A bar holds a switch (All, Full runs, Checks, Nightly) with counts, and a "Start full run" button.
- The left column, 300 pixels wide, lists the runs. Each run has a state icon, the reason it ran, its kind, its commit, and its age.
- A run has one of five states: Running, Passed, Failed, Failed as expected, and Skipped. "Failed as expected" is for tests written before the code, which must fail first.
- The right side shows the selected run. Its head has the kind, the reason and the commit. A line under it gives the state word, the counts of passed, failed and skipped tests, the branch, the duration, the age, and the agent that ran it.
- Under that are the run's jobs on the left (Set up, Tests, User flows) and the selected job's steps on the right. A step is a row that opens to its log. The first failing or running job and step are opened for the reader. Failing log lines are marked.
- A flaky test has no state of its own. It appears as a sentence in the failing test's note, such as "Flaky: it failed 3 of 40 runs. FX-01 fixes it.", at the end of the log.

## What is planned and not built

- "Scored stops on stuck statements" and "A limit on review rounds" under "8. The build loop" in `docs/features.md`. Both are agreed. The mock shows them on sample data, and the live website gives every project an empty record until the engine holds one (`emptyWorkflow` in `workflow.js`).
- "How many review rounds can run" in `docs/specs/2026-11-05-build-loop-design.md`. Since 2026-11-11 there is no hard limit of six rounds; each further round asks the owner first. The mock draws exactly six boxes and its settings stop at 6.
- "How findings are scored" in the same note. Plan review uses the labels Minor, Major and Blocker. The Review phase uses a risk score (under 150, 150 to 399, 400 or more) mapped to the same points. The findings table shows only the label and the points, and never the risk score.
- "Evaluation rounds that update the model ledger" in `docs/features.md`. Not built. A round would grade each task from its reviews and later corrections. Nothing in the Review rounds view shows a grade for an agent or a model.
- "A testing skill with fixed rules" in `docs/features.md`. State: Idea. The aim is that agents do not spend compute on redundant test runs.
- "Test tools for each language, added by pull request" in `docs/features.md`. State: Idea. The same extension point covers simulator testing and testing on a physical phone.
- "A way to bisect" under "8. The build loop" in `docs/features.md`. Requested and not designed. A command finds the commit that broke something. It would start from a failed run.
- "Pushing a finished project and opening its pull request" in `docs/features.md`. The project is to show the state of the pull request's checks and its review state.
- "A better build loop" in `docs/features.md`. No replacement is designed, so the shape of rounds and runs may change.

## How other products do it

### GitHub Actions: the summary of one failed run

[Mobbin screen](https://mobbin.com/screens/0690bb8b-3bbb-45be-9dfa-8cef91e2956f?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The title carries a red failure icon, the run's name and its number, with a "Re-run jobs" menu at the right. A left menu lists Summary, then Jobs with each job and its state icon, then "Run details" with Usage and "Workflow file". The first card states what triggered the run, who pushed which commit on which branch, the Status, the "Total duration", the "Billable time" and Artifacts. A second card draws the jobs as boxes with a state icon and a duration. A third card, Annotations, is headed "2 errors and 1 warning" and lists each one with an icon, the job's name, and the message, with a "Show more" link on a long one.

Difference from LGTM Studio: GitHub gathers every error of a run into one Annotations list on the summary, before any log is opened. LGTM Studio shows the counts in one line and puts the failing tests inside the log of one step.

### GitHub Actions: the list of runs with filters

[Mobbin screen](https://mobbin.com/screens/d46e347a-3067-4877-9d7d-f682084e52ca?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A left menu lists "All workflows" and each workflow by name. The list's head counts the runs and has four filter menus: Event, Status, Branch and Actor. A run is one row with a state icon, a bold title, a second line ("Deploy static content to Pages #1: Commit 85356de pushed by jdoemobbin"), a branch chip, the date, and the duration. A menu on the row offers "View workflow file" and "Delete workflow run".

Difference from LGTM Studio: GitHub's runs are a full-width list, and a click leaves the list for the run's page. LGTM Studio keeps the list and the selected run side by side. GitHub filters by four fields, and LGTM Studio filters by kind only.

### GitHub Actions: a queued run in the list

[Mobbin screen](https://mobbin.com/screens/103932a1-fba2-4b8c-b210-8473adbe9576?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The same row before the run starts shows a small amber dot in place of the state icon. The duration's place holds the word "Queued" in italics.

Difference from LGTM Studio: GitHub has a queued state for a whole run. LGTM Studio has a queued icon only for a step inside a running run, and a run appears in the list only once it runs.

### Vercel: a build log with its warnings and errors marked

[Mobbin screen](https://mobbin.com/screens/50a79ff4-37c0-4a9b-8df3-29599652a9fb?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The Build Logs row is open. Its head shows the count of lines, a copy button, a count of errors and a count of warnings with their icons, and a "Find in logs" field. Each log line has a time and its text. A warning line has an amber background, and the two error lines have a red background. The stages that did not run sit under the log as grey rows with a clock.

Difference from LGTM Studio: Vercel gives the log a search field, a copy button, and counts that work as a summary of the log. LGTM Studio's log has line numbers and marked failure lines, and no search or copy.

### Vercel: deployments as one dense list

[Mobbin screen](https://mobbin.com/screens/b95a75d7-02f8-4b45-9147-b8251441ca75?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

Each row holds a short identifier and its environment, a coloured dot with "Ready" or "Error" and the duration under it, the project, the branch with the commit's message under it, and the date and author. Badges mark special rows: "Current" and "Rolled Back". Five filters sit above the list: a date range, authors, environments, repositories and branches, and a status menu that reads "Status 5/6".

Difference from LGTM Studio: Vercel marks which run is the current one with a badge. LGTM Studio does not mark which full run is the one a merge relies on.

### GitHub: a review comment tied to a line, with its resolved state

[Mobbin screen](https://mobbin.com/screens/4f98de0b-6dad-4867-8f10-b7521d1974fd?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A review comment sits in the pull request's conversation as a box. The box shows the file's name, the few changed lines the comment is about, the comment, and a reply field. Under it is an "Unresolve conversation" button and the line "marked this conversation as resolved". A "Hide resolved" link folds the box. An older comment above it is folded to one line, "This comment was marked as outdated", with a "Show comment" link.

Difference from LGTM Studio: a GitHub finding carries its evidence, the lines of code, and a thread for replies. An LGTM Studio finding is one sentence tied to a statement, with no code and no thread. GitHub has a third state, outdated, beside open and resolved.

### GitHub: a review in progress on the changed files

[Mobbin screen](https://mobbin.com/screens/e9bad011-d5c5-4e8d-b56f-4fba2ffde691?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The "Files changed" tab has a file tree on the left and the changes on the right. A comment sits under the line it is about and carries a "Pending" badge until the review is sent. The top right has a counter, "0 / 2 files viewed", and a green button, "Finish your review", with the count of pending comments. Each file has a Viewed checkbox.

Difference from LGTM Studio: GitHub shows a review as it is being written, with a count of what has been looked at. LGTM Studio shows a round only after it is scored, and "Being scored" is a badge with no progress.

### Sentry: a list of issues with a trend, counts and a priority on each row

[Mobbin screen](https://mobbin.com/screens/92525937-e703-4e62-bd8a-2b6bcb7ca627?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The list is a saved view named "Medium-high". Its search field holds two filter chips, "is unresolved" and "issue.priority is high or medium". The columns are Issue, Last Seen, Age, Trend, Events, Users, Priority and Assignee. An issue's cell has three lines: a bold name, the message, and a line of small facts such as the project's short identifier and an "Unhandled" tag. The Trend cell is a small chart with a word under it, "New" or "Ongoing". The Priority cell is a small menu with a bars icon. The left menu lists saved views with a count on each.

Difference from LGTM Studio: Sentry shows how long a problem has lasted and whether it is new or ongoing, on the row. LGTM Studio shows a finding inside one round, and the owner compares rounds by eye to see that statement 7 drew a blocker three times.

## Options for LGTM Studio

1. Gather a run's failures into one list above the jobs. The selected run would show "2 failed" as a list of test names with their file and message, as GitHub's Annotations card does. The log would stay under it for detail. A flaky test would be a row with its own mark and its rate, such as "3 of 40". The cost is that the failures appear twice, in the list and in the log.
2. Give flaky its own state. A run whose only failures are known flaky tests would get a sixth icon and the word "Flaky", beside Passed and Failed. The switch above the list would gain a filter. No reference above shows a flaky state; this follows from the sample data, where the flaky note is a sentence in a log. It needs the engine to record a failure rate.
3. Mark the run that a merge relies on. The latest passing full run for the project branch's head commit would carry a badge, as Vercel marks "Current". The End result view's evidence line would link to that run. The cost is small.
4. Show findings across rounds, grouped by statement. A second layout of the findings table would have one row for each statement, with a cell for each round holding its points, and a word such as "New" or "Ongoing", as Sentry's Trend cell does. This makes a stuck statement visible before the stuck rule fires. It contradicts the current layout, which is one round at a time.
5. Let a finding open to its evidence. A row would open in place to show the lines of the plan or the code it is about, and the builder's reply, as GitHub's review comment does. Rows open and close on their own. This needs reviewers to record a file and a line with each finding, which the sample data does not have.
6. Add a third finding state for a finding that no longer applies. A finding on a statement that was changed or dropped would read "Outdated" and fold to one line, as GitHub's outdated comment does. Today such a finding stays Open or Fixed.
7. Replace the six round boxes with a row that grows. The rounds would be a list of rows, one for each round that ran, each with its bar, its points and its counts by severity, and one row for the next round with its condition. This follows the design note's rule that there is no hard limit of six, and the owner's preference for dense rows. No reference above shows this.

## Ideas that are not in the feature list

- A search field and a copy button on a log. Reference: Vercel build logs.
- A "Re-run jobs" menu on a finished run. Reference: GitHub Actions run summary. LGTM Studio has only "Start full run", which is not tied to a run.
- Filters on the list of runs by branch and by the agent that ran it. Reference: GitHub Actions runs list.
- A queued state for a whole run, shown in the list before it starts. Reference: GitHub Actions.
- Saved views of findings, such as "open majors and blockers", with a count on each. Reference: Sentry's saved views.
- A count of what a reviewer has looked at while a round runs, such as files viewed. Reference: GitHub "Files changed".
- An "Undo" on a state change, in a short notice at the bottom of the window. Reference: Sentry, where a notice read "Resolved JAVASCRIPT-NEXTJS-1" with an Undo button ([Mobbin screen](https://mobbin.com/screens/90011886-5504-453c-9383-2a20335a4707?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).

## Searches that were run

- "GitHub Actions workflow run with jobs list and step logs". Eight results, all from GitHub. Three were usable. No result showed a job's page with its steps and log open, which is the layout LGTM Studio's Test runs view copies.
- "GitHub pull request page with review comments and merge checks". Eight results, all from GitHub. Two were usable for review findings. No result showed a list of checks with passing and failing rows.
- "Sentry issues list with priority levels and status". Five results, all from Sentry. Two were usable. The rest repeated the same list.
- "Vercel deployment detail page with build steps and status". Eight results, all from Vercel. Two were used here, for the log and for the list of deployments.
- "CircleCI pipeline page with workflow jobs and their status". No results. CircleCI was not found on Mobbin.
- No search found a screen that shows graded review rounds or a test marked flaky. The budget of 14 searches was spent.
