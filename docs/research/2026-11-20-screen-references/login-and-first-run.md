# Login and first run: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

All of this is read from `design/mock/js/screens/auth.js`.

- The login page is at `/login`. It is one centred card with the "LG" mark, the name "LGTM Studio" and the line "Sign in to the service on Studio", with the machine's name.
- The card has one field, "Password", and one button, "Sign in". There is no user name, no account and no pairing, because the service has one user and one password.
- The field has a show and hide button. A line under the field reads "The one password set on the service as `LGTM_WEB_PASSWORD`." The same line becomes "Caps Lock is on." or an error when needed.
- A wrong password clears the field and says "That password is wrong. Nothing was signed in." An empty field says "Enter the password."
- An expired session shows a notice above the field that says the owner returns to where they were after signing in.
- When the service does not answer, an error notice names the address and the command `lgtm serve --detach`. The button becomes "Try again".
- On a connection that is neither HTTPS (Hypertext Transfer Protocol Secure) nor the service's own computer, a warning notice explains the rule and the field is disabled.
- The foot of the card shows a coloured dot with "Service reachable", "Service unreachable" or "Sign-in is blocked on this connection", and the service's address.
- The first run is at `/setup`. It opens after login on a service with no finished setup. Its head has the brand and a "Skip setup" button.
- The first run is one card of 600 pixels with three numbered steps across its top: "Subscription", "Repository" and "First project". A step that was reached can be clicked. A line under the card reads "Step 1 of 3 · Everything here can be changed later in Service settings."
- Step 1, "Connect a subscription", lists each harness as a radio card with its models and run mode. The chosen card shows a "Sign in with Claude Code" button, then a sign-in code and "Waiting for you to approve…", then "Signed in · Max". "Skip for now" moves on without a sign-in.
- Step 2, "Add a repository", has one path field that starts with the workspace folder. The folder is checked half a second after typing stops. The result line reads "Git repository found." with the branch, the number of commits and the remote. A list headed "Found in" offers the repositories of the workspace folder as buttons that fill the field.
- Step 3, "Start the first project", has a "Goal" text box, a "Model for commit and pull request messages" select, and a three-line summary: Repository, Project lead and Machine. The button reads "Create project".
- Each step has a heading. Steps 1 and 2 also have a paragraph that explains the step, and the sign-in cards hold further sentences.
- On the live website step 1 differs. It lists the harnesses the service discovered, with "Installed" and "Configured" words and their accounts. It asks for "Workspace folder", "Shell" and "Your name", and has "Refresh discovery". "Skip setup" and the harness sign-in buttons are not connected there. Finishing opens the new project's board.

## What is planned and not built

- `design/DESIGN.md`, "The first run, built from a private app's onboarding rules", describes three pages that differ from the three steps in the code. The first page states what the product does and asks for nothing. The second page shows what was found and asks for nothing. The third page is the New project form with its answers filled in. The code's steps are a sign-in, a path field and a goal.
- The same section says: "Look around first is a link, not a step." The code has "Skip setup" and no link to a sample.
- The same section says: "Setup hands off to the lead." Creating the project should open the lead's conversation with the message box focused. The live first run opens the board.
- The same section says: "Notifications are offered on the first waiting request, not in setup." This part is built. The offer is on Home (`design/mock/js/screens/home.js`).
- The same section says the first run does not cover a computer that joins as a worker.
- `design/DESIGN.md`, "Login", says the heading is "Sign in to LGTM Studio". The code shows the product's name with the line "Sign in to the service on Studio".
- `docs/features.md`, "The next piece: installation and configuration", entry "Standalone installer and startup at login". An installer and starting the service at login are agreed and not built.
- `docs/features.md`, "A "Where is your workspace?" setting in the Service settings". The state is To build. The entry says the owner can choose the workspace folder only in the first-run steps today.
- `docs/features.md`, "Offer a sign-in that is already on the machine". The Harnesses page should notice an existing sign-in and offer it in one step. The first run's step 1 would use the same check.
- `docs/features.md`, "Sign in to a harness from the page, with a login folder for each account". "Add account" should run the harness's sign-in in a terminal shown beside the dialog.
- `docs/features.md`, "A simpler way to set up harnesses and subscriptions". The owner decided that adding a harness account must become simpler.
- `docs/features.md`, "A repository agent that works in the main checkout and prepares a repository for projects". Its first job would run when the first repository is added, which is step 2 of the first run.
- The owner's rule of 2026-11-20 says a screen carries no helper or instructional sentences. The login hint and the paragraphs in the first run are such sentences. Whether the rule covers these two pages is not recorded.

## How other products do it

### Tally: a page with a heading, one password field and one button

[Mobbin screen](https://mobbin.com/screens/0020f73b-2046-4c90-9abf-2b011fb7dbdd?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The page is white with one small block in the middle. The heading reads "Authentication required" and a grey line under it reads "This page is protected." A "Password" label, the field and a full-width "Continue" button follow. There is no logo, no link and no foot.

Difference from LGTM Studio: Tally says nothing about where the password comes from or what is behind it. The login page names the machine, the environment variable and the service's address.

### Maze: the password field and its button on one line

[Mobbin screen](https://mobbin.com/screens/f48a15a6-51ac-4841-8798-1c562d5c02fa?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A heading says the page is password protected, and one line under it says where the password came from. The field and a "Start" button share one line, with the button joined to the field's right end. Nothing else is on the page.

Difference from LGTM Studio: Maze puts the action beside the field. The login page stacks a labelled field, a hint line and a full-width button inside a card.

### Family: a password page with a second way in

[Mobbin screen](https://mobbin.com/screens/ef8659d7-87a2-4409-bbad-280540360f95?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A round lock icon sits above the heading "Enter Password". The field has no label and no button, only a placeholder. A line under it reads "Forgot your password?" with the link "Use a Recovery Code". A back arrow is at the top left, and three small links are at the foot.

Difference from LGTM Studio: Family offers a way back in when the password is lost. The login page has none, because the password is set where the service runs.

### Luma: a password card with a link beside the label

[Mobbin screen](https://mobbin.com/screens/92cf9dcc-c5b8-46f0-a12f-c57df32a2c58?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A small card holds a back arrow, the heading "Password", one sentence, the field and a dark "Continue" button. The label "Password" is on the left above the field, and the link "Send Me a Code" is on the same line at the right. The top bar of the page shows the local time.

Difference from LGTM Studio: Luma uses the label's line for the second action. The login page uses the space under the field for its hint and its errors.

### Linear: a setup step with three ticked lines, one button and a way to put it off

[Mobbin screen](https://mobbin.com/screens/e0f6ce66-0d22-4858-b8a7-2d2aa3431b23?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The step fills a blank page. A GitHub mark, the heading "Install GitHub integration" and one grey line are centred. A box holds three lines, each with a tick. The third line says what Linear will not ask for. One filled button reads "Authenticate with GitHub", and a faint link under it reads "I'll do this later". Seven dots at the foot mark the step.

Difference from LGTM Studio: Linear gives each step a whole page with one action and unnamed dots. The first run keeps three named steps in one card and shows "Back", "Skip for now" and "Continue" together.

### Linear: a last page that points at three next things

[Mobbin screen](https://mobbin.com/screens/287d9435-e8b9-497f-a632-9bdbd827414c?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The last step is headed "You're good to go". Three tiles sit side by side: "Tell your team", "Integrate GitHub & Slack" and "Keyboard shortcuts". Each tile has an icon, a bold title and one or two lines, and the third names the key that lists the shortcuts. One filled button reads "Open Linear".

Difference from LGTM Studio: Linear ends setup with pointers to what was skipped. The first run ends by creating the project and leaving setup.

### Cursor: setup done by an agent, with a checklist that fills in

[Mobbin screen](https://mobbin.com/screens/44239877-86ce-46c8-a615-273ec82046f5?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The setup is inside the product, not before it. The agent's conversation is on the left and shows the files it searches. A pane on the right is headed "Agent is setting things up" and lists five steps with a round mark each. The first step is open and holds a "Turn On Notifications" button. A card at the foot of the pane links to documentation and a video.

Difference from LGTM Studio: Cursor lets the person watch setup happen and act on one step at a time. The first run is a form the owner fills in before the product opens.

### Devin: a setup checklist that stays on the first page of the product

[Mobbin screen](https://mobbin.com/screens/cb506f20-a571-438c-9897-b3cc43b20345?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

Under the prompt box is a list headed "Get started with Devin". Its head shows "4 of 6", a progress bar and a close button. Four lines are ticked and blue: "Connect to Git", "Select repositories", "Make your first session" and "Validate in Devin Review". Two lines are still open, and a last row reads "Show advanced tips".

Difference from LGTM Studio: Devin lets the person use the product while setup is unfinished and keeps the count in view. The first run is either finished or skipped, and nothing afterwards shows what was skipped.

## Options for LGTM Studio

1. Reduce the login card to the label, the field and the button, and keep the service's address in the foot. This follows Tally. The hint about `LGTM_WEB_PASSWORD` moves to the README. The cost is that an owner who forgot where the password is set gets no pointer on the page.
2. Put "Sign in" on the same line as the field. This follows Maze. The cost is a narrow button on a phone, and less room for the show and hide button.
3. Replace the first-run card with a checklist that sits on Home until it is done or closed: connect a harness, add a repository, start a project. This follows Devin. The cost is that Home and the sidebar must work with no harness and no repository, and the record that setup is finished is still needed.
4. Let an agent do the repository step and show its steps as a checklist beside its conversation. This follows Cursor and fits the planned repository agent. It needs a signed-in harness first, so the harness step stays a form.
5. Give each step its own page with one action and one way to put it off. This follows Linear. It matches `design/DESIGN.md`, "The first run, built from a private app's onboarding rules", which also asks for one job per page. The risk is that Linear's pages lean on explanatory lines, which the owner has ruled out on screens.
6. End the first run with three pointers to what was skipped, such as Notifications, a second harness and the shortcuts. This follows Linear's last page. It contradicts "Setup hands off to the lead" in `design/DESIGN.md`, which says there is no tour.
7. In step 2, drop the path field as the first thing and show the found repositories as rows with one "Add" button each. This follows the Vercel import list described in `new-project.md`. The cost is nothing new to build, because the first run already reads that list.

## Ideas that are not in the feature list

- A second way in when the password is lost, offered on the login page ([Family](https://mobbin.com/screens/ef8659d7-87a2-4409-bbad-280540360f95?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A setup checklist with a count, such as "4 of 6", that stays in the product after the first run ([Devin](https://mobbin.com/screens/cb506f20-a571-438c-9897-b3cc43b20345?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A per-project checklist of what is still to set, shown on the project's first page ([Vercel](https://mobbin.com/screens/6410a8f7-a1dc-4e36-8db4-ef1ffc95049d?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A last setup page that points at what was skipped ([Linear](https://mobbin.com/screens/287d9435-e8b9-497f-a632-9bdbd827414c?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- Ready first tasks offered when setup ends, such as "Find PR bugs" and "Fix flaky CI" ([Cursor](https://mobbin.com/screens/4b4f4f61-1c30-496c-abec-fde4d35d61bd?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).
- A setup step that has the owner press the switcher's keys ([Linear](https://mobbin.com/screens/1e783aca-8b8e-4ae5-994d-284a8ddae491?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)).

## Searches that were run

- "login screen with only a password field". It returned 8 screens from Tally, Family, Uber Eats, Luma, Calendly, Heidi, Uber and Maze. Four were usable. None is a developer tool, and the other four are the second step of a login that already asked for an email address.
- "Linear onboarding step connecting GitHub". It returned 3 screens, all from Linear. Two were usable.
- The Cursor and Devin screens came from searches recorded in `sidebar.md` and `new-project.md`. The Vercel checklist came from the search recorded in `top-bar.md`. The Linear switcher step came from the search recorded in `command-switcher.md`.
- No query named GitHub, Vercel, Sentry, Notion or Slack for a login page, because the search budget was spent. Nothing is said about their login pages here.
