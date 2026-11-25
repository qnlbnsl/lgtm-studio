# User flows of the website

This document lists how the owner moves between the screens of the website.
It has three uses.
It says where a Back button belongs.
It is the index of the flows that the browser tests will cover.
It tells a builder which control opens which address.

It describes the website in `src/frontend/` as it was at the end of 2026-11-23.
It was made by reading every link in the code.
The flows marked "walked" were also tried in a browser.

## The four kinds of move

Every move between two addresses is one of four kinds.
The kind decides whether the screen the owner lands on shows Back.

1. **Switch.** The owner picks a place from a standing list of places.
   The row of views, the sidebar, the path in the top bar and a row of
   sub-tabs are such lists.
   The list is still there after the move, so one click returns.
   A switch shows no Back.
2. **Choice.** The owner picks one item inside a view, and the view shows it
   beside its list.
   A file in the tree, a run in the list of runs and a question in the list of
   questions are choices.
   A choice shows no Back.
   A choice takes the place of the last choice in the browser's history.
3. **Open inside.** An item of a view opens on a page of its own, and the list
   it came from is gone from sight.
   A sprint opened from the board is the example.
   The page shows Back.
4. **Link.** A reference inside one view, or inside the conversation, opens
   another view.
   A file name in a question is the example.
   The view the owner lands on shows Back.

## What Back does

- Back returns to the address the owner came from. It never goes to a parent.
- Back names the view it returns to, such as "Actions".
- Back is the first thing in the header of the view the owner landed on.
- Back shows only after a move of kind 3 or 4.
- The view it returns to opens on the same choice as before, when that view
  keeps its choice in the address.
- Back holds after a reload. A new browser tab opened on an address has no Back.
- An address typed by hand is not a link, so it shows no Back.
- Status, the message board and the end result are pages of one view. A move
  among them shows no Back, because their sub-tabs return.

## The screens and their addresses

Every address starts after the `#` of the website's address.

Inside a project, `/p/<project>/` and then:

- `board`: the board. Its sub-tabs are `board/<view>`.
- `sprint/<sprint>`: one sprint. Its tabs are `sprint/<sprint>/<tab>`.
- `sprint/<sprint>/tasks/<task>`: one task, inside its sprint.
- `questions`: the list of questions.
- `questions/<question>`: one question, beside the list.
- `status`: the Status view, on its Overview page. `status/review` is its Reviews page.
- `message-board/<channel>`: one channel. `?t=<post>` opens one thread.
- `end-result`: the end result.
- `actions`: the Actions view. `?action=<file>&run=<run>` is its choice.
- `design`: the Design view, on its list of drawn screens. It was named Mocks,
  and the old address `mocks` opens it.
- `design/changes`, `design/components` and `design/data`: three more pages of
  what was drawn.
- `design/screen/<screen>`: one drawn screen.
- `design/file/<category>/<item>`: the design file on its canvas.
- `design/browser`: the running product. `?show=<preview>&size=<size>` is its
  choice.
- `design/architecture`: the codebase map. The old address `map` opens it.
- `files?f=<path>`: the Files view with one file open.
- `git`: the Git view.
- `settings`, `settings/agents`, `settings/workflow` and `settings/previews`:
  the four pages of Settings.
- `conversation/<agent>`: another agent's thread, in the conversation's column.
- `team/<team>`: one team's page.

Outside a project:

- `home`, `projects`, `machines`, `harnesses`, `usage`, `notifications`,
  `workflows` and `settings`.
- `r/<repository>` and `r/<repository>/design/...`.

## Switches, which show no Back

- The row of views of a project.
- The sidebar: its pages, its repositories, its projects, its teams and its agents.
- The path in the top bar.
- The sub-tabs of Status: Overview, Reviews, Message board, End result.
- The sub-tabs of Design: Screens, Changes, Components, Data, Design file,
  Browser, Architecture.
- The sub-tabs of the board and of Settings.
- The tabs of a sprint. A tab also takes the place of the last one in the
  browser's history.

## How each screen is reached

Each entry names the screen, then every control that opens it.
"Back: built" means the screen shows Back today.

### A file in the Files view

Back: built.

- Link: the file name of an action, in the rail of the Actions view.
- Link: a file name in the references of a question.
- Choice: a file in the tree of the Files view.
- Switch: "Files" in the row of views.

### The Git view

Back: built, at the start of the bar that names the worktree.

- Link: "Open in Git" in the header of a changed file, in the Files view.
- Link: "Open in Git" on a task.
- Link: "Git" on a team's page.
- Link: "Open its worktree in Git" in the menu of the conversation.
- Switch: "Git" in the row of views.

### A sprint

Back: built.

- Open inside: a sprint's card or row on the board.
- Link: a sprint named in the conversation, such as "LS-A01-3".
- Link: a sprint in the list of sprints of the end result.
- Link: a sprint named under "After" in another sprint's header.

### A task

Back: built, in the header of its sprint.

- Open inside: a row of the Tasks tab of its sprint. The task has its own
  Close button, which shows every task again.
- Link: a task named in the conversation, such as "LS-A01-22".
- Link: the task that a question blocks, in the question.
- Link: the task an agent works on, on the agent's page, a team's page and the
  message board.

### One question

Back: built, on a line of its own above the question's title.

- Link: "Open in Questions" on a question in the conversation.
- Link: "Details" on a question's card in the conversation.
- Link: "Open requests" above the conversation, which opens the list.
- Link: a row of "Needs attention" on Home.
- Link: a row of the bell's menu in the top bar.
- Link: a row of the Notifications page.
- Link: the question named in a post of the message board.
- Choice: a row of the list of questions.

### The end result

Back: built, at the start of the sub-tabs of Status. A project in Define has
no such sub-tabs, so its end result has no Back.

- Link: the statement a question is about, in the question.
- Link: "Statement 3" in a finding of a review round. No Back: the same view.
- Link: the statements on the Status view. No Back: the same view.
- Link: the statement on a task.
- Switch: "End result" in the sub-tabs of Status.

### The Actions view

Back: built.

- Link: the last run on the Status view.
- Link: "Runs" on the Status view.
- Switch: "Actions" in the row of views.
- Choice: an action in the rail, and a run in the list.

### The Reviews page of Status

Back: built, at the start of the sub-tabs of Status. No link from another
view opens this page today, so Back does not show there yet.

- Link: the open round on the Status view. No Back: the same view.
- Link: "Rounds" on the Status view. No Back: the same view.
- Switch: "Reviews" in the sub-tabs of Status.

### Another agent's conversation

Back: built before this work. The thread has its own arrow, which returns to
where the owner came from.

- Link: an agent's name on the Status view, a team's page, the message board,
  the Usage page, the Machines page and the Harnesses page.
- Link: the owner of a task, on the task.
- Link: "Open thread" on a request.
- Switch: an agent in the sidebar.

### A team's page

Back: built, on a line of its own above the team's name.

- Link: the team named on a sprint, on a task and in a review round.
- Link: the team named in the header of its channel on the message board.
- Link: a team in the project's Settings.
- Switch: a team in the sidebar.

### A channel of the message board

Back: built, at the start of the sub-tabs of Status.

- Link: "Team channel" on a team's page and on an agent's page.
- Link: a post shown in the conversation, which opens its thread.
- Choice: a channel in the list of channels.
- Switch: "Message board" in the sub-tabs of Status.

### The Design view

Back: built, at the start of the sub-tabs of Design.

- Link: "Open the design" on the board of a project that is in its Design
  stage.
- Link: "Design" among the things a phase made, on the Status view.
- Switch: "Design" in the row of views.
- Switch: each of its seven sub-tabs.
- Choice: a preview and a size on the Browser page.
- Choice: a category and an item on the Design file page.

### One drawn screen of the Design view

Back: built. It takes the place of the arrow named "Screens" after a link
from another view. No such link exists today, so the arrow to the list of
screens is what shows.

- Open inside: a row of the list of screens.
- Link: a screen named in a review of the design. No Back: the same view.

### The Browser page, from a preview in Settings

Back: built, at the start of the sub-tabs of Design.

- Link: the state of a preview, on the Previews page of Settings.

### A worktree in Files and in Git

No address: the choice is kept for the project, not in the address.

- Choice: the branch at the top of the side column of Files.
- Choice: the worktree at the start of the bar of Git.

### The board, the list of questions, Status and Settings

These need no Back.
Each is reached by a switch.
A project opened from Home, from "All projects" or from the Machines page
lands on its first view, and the sidebar returns from there.

## The index of flows

Each flow has a start, the moves, and the place Back returns to.
"Walked" means it was tried in a browser on 2026-11-23, in Chromium.

- **F-01. From an action to its file, and back.** Start on Actions. Choose an
  action and a run. Follow the file name in the rail. Back returns to Actions
  with the same action and run. Walked.
- **F-02. From a question to a file it refers to, and back.** Start on one
  question. Follow a file name in its references. Back returns to the
  question. Walked.
- **F-03. From the conversation to a question, and back.** Start on any view.
  Follow a question in the conversation. Back returns to the view. Walked
  from Git.
- **F-04. From Home to a question, and back.** Start on Home. Follow a row of
  "Needs attention". Back returns to Home. Walked.
- **F-05. From the board into a sprint, and back.** Start on the board. Open a
  sprint. Back returns to the board. Walked.
- **F-06. From a sprint into a task, and out.** Start on a sprint. Open a
  task. The task's Close button shows every task again. Walked.
- **F-07. From a name in the conversation to a sprint, and back.** Start on
  any view. Follow a sprint named in the conversation. Back returns to the
  view. Walked from Actions.
- **F-08. From a question to the task it blocks, and back.** Start on one
  question. Follow the task. Back returns to the question. Walked.
- **F-09. From a file to Git, and back.** Start on a changed file. Follow
  "Open in Git". Back returns to the file. Walked.
- **F-10. From a task to Git, and back.** Start on a task. Follow "Open in
  Git". Back returns to the task. Walked.
- **F-11. From Status to the last run, and back.** Start on Status. Follow the
  last run. Back returns to Status. Walked.
- **F-12. From a review round to a statement.** Start on Reviews.
  Follow "Statement 3". The end result shows no Back, and the sub-tab
  "Reviews" returns. Walked.
- **F-13. From an agent's name to its thread, and back.** Start on any view.
  Follow an agent's name. The thread's arrow returns to the view. Walked
  from Status.
- **F-14. From the message board to a team, and back.** Start on a team's
  channel. Follow the team's name. Back returns to the channel. Walked.
- **F-15. From a team to its channel, and back.** Start on a team's page.
  Follow "Team channel". Back returns to the team. Walked.
- **F-16. From Design into a drawn screen, and out.** Start on Design. Open a
  screen. The arrow "Screens" returns to the list. Walked.
- **F-17. A switch shows no Back.** Move through the row of views and the
  sub-tabs of Status. No view shows Back. Walked.
- **F-18. A choice keeps Back.** After F-01, choose another file in the tree.
  Back still returns to Actions. Walked.
- **F-19. A reload keeps Back.** After F-01, reload the page. Back still
  returns to Actions. Walked.
- **F-20. A new tab has no Back.** Open a file's address in a new tab. The
  header shows no Back. Walked.
- **F-21. A typed address has no Back.** Click something that opens nothing,
  wait, then type a file's address. The header shows no Back. Walked.
- **F-22. An old address opens the new page.** Type the address `mocks`, then
  the address `map`. The first opens Design. The second opens Architecture.
  Walked.
- **F-23. The running product, stopped and run again.** Start on Browser.
  Choose Phone. Press Stop, then Run. The product shows again, and the address
  that was open before Stop no longer answers. Walked, against the mock server.
- **F-24. A choice in Design keeps the browser's history short.** On Design
  file, pick another category. On Browser, pick another size. The browser's
  history does not grow. Walked.
- **F-26. A preview is added, changed and removed.** Start on the Previews page
  of Settings. Add a preview. Change the command of one that runs: it stops.
  Give one the name of another: the name is put back. Remove the new one.
  Browser lists the same previews. Walked, on sample data and against the
  mock server.
- **F-27. Full access is confirmed.** Start on the Agents page of Settings.
  Choose "Full access". A dialog asks, and Escape leaves the choice as it was.
  Walked.
- **F-25. From Status into Design, and back.** Start on Status. Follow
  "Design" among the things a phase made. Back returns to Status. Not walked.
- **F-28. A project opens on its command center.** Follow a project in the
  sidebar. The command center shows. Follow a row of "Open requests": its question
  opens. Go back, and follow "Active agents" to Status. Walked.
- **F-29. A widget is hidden.** On the command center open "Widgets" and take
  "Usage" off. Reload the page. "Usage" is still hidden. Walked.
- **F-30. A repository and its agent.** Follow a repository in the sidebar.
  Its command center shows, with the repository agent on the left. Open
  Design, then Browser: the product of the main checkout shows. Open Files and
  Git: both show the base branch. Walked, on sample data.
- **F-32. A message to a build lead.** On the board, open the arrow beside
  "Project lead" and choose "Build lead · Shell". Its thread takes the
  conversation's place. Send a message: the lead answers. Open Git: the same
  lead is still beside it. Choose "Project lead" to go back. Walked.
- **F-33. The widgets are arranged.** On the command center open "Widgets" and
  choose "Arrange the widgets". Move "Checks" earlier, make "Progress" two
  columns wide, hide "Usage", and press "Done". Reload: the arrangement is
  kept. "Reset layout" restores it. Walked.
- **F-34. An older version of a workflow comes back.** On Workflows, clone the
  default. Change a phase name and save, twice. Open "Version 3" and choose
  "Version 1". The editor says that version 1 is in it, not saved. Save: it
  becomes version 4. Walked, on sample data.
- **F-35. A repository is added and set up.** Press the plus beside
  "Repositories". Paste `git@github.com:acme/storefront.git` and press "Add".
  The repository's page says it is tracked and on no machine. Press "Set up on
  Workstation". The repository agent's conversation opens beside the command
  center. Walked, on sample data.
- **F-36. A repository is added from GitHub.** In "Add a repository" press
  "Connect GitHub", choose an account, and press "Add" on a row. Walked, on
  sample data.
- **F-37. A terminal outside a project.** On Home press "Terminal". Choose
  `lgtm-studio` under "Open a terminal in". The terminal opens under Home, in
  the main checkout on Workstation. On a repository's page, Control and the
  backtick key open its terminal with no question. Walked.
- **F-38. The board of a repository.** Open `lgtm-studio` and press "Board".
  The sprints of all its projects are in the lanes, and each card names its
  project. Choose "Agent console" under "Project". Open a card, and press
  Back. The board returns with the filter kept. Walked.
- **F-39. The drawn screens of a repository.** Open `lgtm-studio` and press
  "Design". The six screens of the main checkout are listed. "Request" names
  "Phone companion" under "Changing in". Open "Request", and follow "Phone
  companion" to that project's drawing of it. Walked.
- **F-31. A goal becomes a project.** On a repository's command center, press
  "New project" on a goal that has none. The new project dialog opens with
  the goal filled in. Walked as far as the dialog.

## What this document does not cover

- The pages outside a project were read for their links into a project only.
  The moves among them are not listed.
- A repository's design file is listed only as far as F-30 opens it.
- The sign-in, the first run and the Connect screen are not listed.
- The dialogs and the menus that change data are not listed. They do not move
  the owner to another address.
- The older layout in the same code, used when the live engine is on, is not
  listed.
- A phone is not covered. On a phone a list and its item are two pages, so a
  choice there is an "open inside".
