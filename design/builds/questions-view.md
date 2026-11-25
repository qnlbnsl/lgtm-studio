# Questions view: one question in full, and the list of a project's questions

This screen is in `design/mock-vite/` only since 2026-11-23. It was built in `design/mock/`, and that folder went back to the state of `master`, because the changes reached the live website and broke one of its journeys. That is issue 1574. Where the text below says `design/mock/`, read the file of the same name under `design/mock-vite/src/`.

The question page was changed in the port on 2026-11-23. Answers are radio buttons or checkboxes with one "Send answer" button, "Open thread" is "Discuss", and an approval has "Allow" and "Deny" with a lasting answer behind an arrow. The text below describes the page of 2026-11-22. The rules are in `design/DESIGN.md`, and the changes are items 7 to 11 of issue 1572.

Built on 2026-11-22 in `design/mock/`, on sample data. The owner approved it the same day,
and its rules are in `design/DESIGN.md` under "The Questions view". It is not wired to the
engine.

surface: app screen. The owner opens it whenever an agent asks something.

reference: the mock's own Git, Files and message board views, which put the content beside
the conversation and their list on the far right. No other product was searched.

## What the owner asked for

The owner said on 2026-11-22: a tab next to Board, a thing that lists the questions, and a
full view of one question. A click on Details in the chat opens that view. A question then
has no expanded view. Approvals are left for later.

## Plan

| # | Region | Job | Container | Why this container | Rows visible in the first viewport | Empty state |
|---|---|---|---|---|---|---|
| 1 | The question, in the middle | Read one question in full, then answer it | none | One document, read from the top: the question, its details, the answers | not a list | "No questions", when the project has none |
| 2 | The list, on the far right | See which questions wait and pick one | timeline | Events in time order, in a side column, each with a sentence, a picture and a time | 13 rows of two lines in 782 pixels | The view's empty state replaces it |

main offset: 0.19. The row of views and the row of stages sit above the list.

Navigation: Questions is one more destination in the project's row of views, after Board.
Its tab counts the questions that wait.

Parts: the list's drag handle is the mock's own `ListResizer`, shared with Git, Files and the
message board. The question is its own component, `QuestionDoc`, in
`design/mock/js/screens/questions.js`. No library was added.

Motion: none.

- #1 scan: edges the pane's left padding for every text, and on a wide pane a second line
  for every value, point and answer; "Open thread", "Choose" and "Send" on the right;
  groups by the four ruled sections; visuals the asking agent's picture, the sparkles icon
  on the recommended answer and a check on the chosen one; coloured a wait of an hour or
  more, "Recommended", "Your answer" and the link to the blocked task.
- #2 scan: edges the list's left padding, the time on the right; groups Waiting and
  Answered; visuals the asking agent's picture; coloured the selected row and a wait of an
  hour or more.

layer: none. Nothing opens over the view, and no action shows only on hover.

first run: same as returning.

feedback: an answer changes the question in place. The buttons become the answer, the row
moves to Answered, and the count on the tab drops.

## What it does

- With no question in the address, the question that has waited longest shows.
- A waiting row shows the question, who asked, and how long it has waited. An answered row
  shows the answer in place of who asked, and how long ago it was answered.
- The list's button at the right end of the row of views hides the list. The list drags
  from its left edge.
- The page is a report to skim, not prose. The owner asked for this on 2026-11-22, because
  a page to skim is easier to read than an essay.
- The question stands alone at the top, in the page's one large text. The owner asked for no
  Accept button and no recommendation at the top.
- A block of facts follows, each a label beside its value: "Asked by", "Blocks" as a link to
  the task, and "Waiting" or "Answered". "Open thread" sits at the right end of the first
  row and opens the asking agent's thread in the conversation's column.
- Ruled sections follow: "Background", "Impact", "Answers" and "References". A rule above
  each section separates it from the one before. The headings are terms of a written report.
  They were "What it is about" and "Why it matters".
- "Background" and "Impact" are short points, one claim each, in the product's body size.
  A point's mark hangs left of the text, so every text keeps one left line. A section with
  one point has no mark.
- On a pane of 560 pixels or more, the headings move into a column of their own, in line
  with the labels of the facts. Every value, point and answer then starts on one second
  line. On a narrower pane a heading stands above its section.
- The answers come last, so the owner answers after reading. The agent's recommended answer
  is first, marked with the sparkles icon and "Recommended", with its reason under it. Each
  answer has "Choose". "Your own answer" ends the list.
- An answered question marks the chosen answer with "Your answer" and mutes the others. An
  answer in the owner's own words is the first row.
- A question with no report shows the agent's wording as its "Background".
- A first build ended with "Before it asked", the last messages of the asking agent. The
  owner had it removed on 2026-11-22, because the list on the right already shows the
  questions over time.
- Deny on a question's card opens the view with the recommended answer struck out, marked
  "Declined", and "Your own answer" focused.
- Every link to a question opens the view, since the owner's yes of 2026-11-22. The shared
  link `routeForRequest` in `design/mock/js/data.js` chooses the place. The bell, the message
  board, the roster of agents, the lead's list of requests and the notifications use it. An
  approval's link, and "Open thread" on a card, lead to the asking agent's thread through
  `threadRouteForRequest`.
- A question's card in a thread and on the sprint page has Details, which opens the view.
  In the lead's conversation, a question above the message box has Details in place of "Why
  it matters" and "Open thread".
- On a phone the view shows the list first, then one question with a way back.

## Captured

At 1440: the view from its tab, a second question, the list hidden, an answered question, a
declined recommendation, the lead's conversation with Details, the sprint page's cards, the
view after an answer, worst-case data, the empty state, and dark mode. At 390: the list and
one question.

## Worst case

The switch is `?data=worst`. It adds a question whose title has a word that cannot break, a
question with a one-letter title and answer, a long typed answer, a cancelled question, and
24 more answered questions.

- The long answer was cut in the list while its time was whole. It now wraps and shows
  whole. Whether to cut it at two lines is the owner's decision.
- The long title wraps in the middle and stops at two lines in the list.

## Gate

The space checks find nothing at 1440, with demo data and with worst-case data. At 390 they
find only the older fault of the sidebar's padding.

Two waivers of `empty-pane`. Five questions cannot fill a list that is 782 pixels tall, and
the list is as tall as the view. A short question cannot fill the middle either: it is one
document, and an answered one with no report is a few lines.

The build passes and the 795 tests pass. A crawl of the new addresses at 1440 and 390 finds
no page error.

## Checked against the scan and layer rules

Done on 2026-11-22, at the owner's request, against the newest screen-build skill. That is
a newer version of the skill than the one this machine had installed, read from its
repository. The lines were
measured in the browser at 1440 pixels, not judged by eye.

What holds:

- In the question, every text starts on one left line, 20 pixels from the pane's edge. The
  last letter of "Open thread", "Choose", "Send", "Your answer" and every rule end on one
  right line. This was measured again after the page became a report.
- A fact's label and its value share a baseline. On a wide pane a heading shares the
  baseline of its section's first line. In the list, who asked and the time share one.
- In the list, the headings, the titles and the pictures start on one left line. The times
  end on one right line, with tabular figures.
- The report stops at 620 pixels when the headings stand above, and at 760 pixels when
  they stand beside.

What was off, and is fixed:

- The "Open thread" icon stood 7 pixels inside the right line, because the button has no
  border. Its icon now ends on the line.
- "Details" in the lead's conversation stood 9 pixels right of the answers' left line, for
  the same reason. Its word now starts on the line.
- "Answered at" and its time were split by a column gap. They are one phrase now.
- A cancelled question looked like a live one in the list. Its title is muted now.
- A title cut at two lines in the list had no way to be read whole. It has a tooltip now.

What the rules question, and waits for the owner in issue 1567:

- The rules want one filled button in a row of actions. Every "Choose" is outlined, as
  `design/DESIGN.md` says of a request's buttons, so the design document wins.
- The report's sections are points with no indent, so the open point about a paragraph's
  indent is gone from this page. A request card's details still indent their paragraphs.
- In an answered row the answer stands beside the picture of the agent that asked.
- Waiting runs from the longest wait down. Answered runs from the newest down. The two
  groups run in opposite directions of time.

## Not checked

- No second blind review was run after the fixes of the first.
- The live website does not have the view. Its browser journeys were not run.
- The phone was checked only for breakage, as the owner asked on 2026-11-22.
- Approvals and the questions of the Define phase are not in the view.

## What a builder needs for the report

The report shows "Background" and "Impact" as short points. The mock's sample questions
store each as a list of short sentences. An agent that asks a question must write them that
way: one claim in each point, and no paragraphs. That is a rule for the agent's instructions
and for the question's data, and it is not built.

## Blind review

A fresh reviewer was sent eleven pictures on 2026-11-22: six states at 1440, one at 1920,
dark mode, the lead's conversation with its waiting questions, and two at 390. It was also
sent a picture of the Git view as the reference, and the design document. It did not see the
plan, the source or the gate's output. Its verdict was "no", with no blocker, two findings
labelled major and twelve labelled minor. It said the first major is the only thing that
holds the view back.

| # | Label | Plan row | What the reviewer saw | Answer |
|---|---|---|---|---|
| 1 | major | 2 | An answered row grows without limit. A long answer ran to six lines, the height of three rows | Fixed. An answer keeps to two lines in the list, and the question's page shows it whole |
| 2 | major | 1 | The answers are the last thing on a tall page. With worst-case data "Your own answer" is below the fold | Not fixed. The owner asked on 2026-11-22 for the answers at the end and for no button at the top. It is in issue 1567 |
| 3 | minor | 2 | A two-line row is 54 pixels, against 46 for a record in the design document. The picture sets the second line's height | Not fixed. It is in issue 1567 |
| 4 | minor | 1, 2 | An answered question shows an age in the list and a clock time on the page | Not fixed. It is in issue 1567 |
| 5 | minor | 2 | In an answered row the asking agent's picture stands before the owner's answer | Not fixed. It was already in issue 1567 |
| 6 | minor | 1 | At 1440 with every panel open the pane is 401 pixels, so the headings never stand beside | Not fixed. The owner saw both forms on 2026-11-22 and said they are fine |
| 7 | minor | 1 | "Send" is 24 pixels high and its field is 28 | Fixed. Both are 28 |
| 8 | minor | 1, 2 | With no questions, the list's button is still tinted although no list shows | Fixed. The button shows only when the project has questions. The empty state's own type is shared with every view and is unchanged |
| 9 | minor | 1 | Blue on every "[1]", and green on "Recommended" and "Your answer", which cannot be pressed | Not fixed. The colours are the approved ones of a request card. It is in issue 1567 |
| 10 | minor | 1 | "Open thread" looks like a label. "[1]" and the reference's path may or may not be pressable | Fixed in part. "Open thread" is an outlined button. The references are not links |
| 11 | minor | 1 | Answers that were not chosen have a contrast of 3.65 to 1 | Fixed. They use the muted colour at regular weight |
| 12 | minor | 1 | The asking agent's name starts 26 pixels right of the other values, because of its picture | Fixed. The fact has no picture, so the three values start on one line |
| 13 | minor | 1 | At 1920 each "Choose" is far from its answer, and both panes are largely empty | Not fixed. It is in issue 1567 |
| 14 | minor | 1 | On a phone the question's edge is 13 pixels and the list's is 17. "Questions" shows three times | Not fixed. The phone is not the focus for now. It is in issue 1567 |

After the fixes the journey, the space checks, the build and the 795 tests were run again.
At 1440 the space checks find nothing, with demo data and with worst-case data. No second
blind review was run.

## Approvals joined the view

The owner said on 2026-11-22 that approvals join the view for now, so that no card expands
in place anywhere. He also said that an approval is an agent's action and not a question,
and that it needs a better design of its own.

- The list holds questions and approvals in one run. A waiting approval says "Approval"
  before the asking agent. An answered one shows "Allowed" or "Denied".
- An approval's page uses the question's form. Its facts add "Tool". "Command" comes first,
  with the command and the folder in the card's own code boxes. "Decision" comes last, with
  "Allow once", "Always allow", "Deny" and "Always deny".
- An approval whose input the harness hides says so under "Command", and its one-time
  answer keeps the harness's wording, "Allow without seeing input".
- Details on an approval's card opens its page. Its Allow and Deny stay on the card.
- In the waiting decisions above the message box, an approval has Details in place of the
  folded "Impact" line and of "Open thread".
- A row of Home that is a question or an approval opens its page. A merge and a failed
  start still open in place.
- `inQuestionsView` in `design/mock/js/data.js` says which requests have a page.

It was checked by clicking each path: a row of Home, the bell, a card in a thread, the
waiting decisions, and "Allow once" on the page. The space checks find nothing in the view
for the three approval states at 1440. No blind review was run on the approval's page.

## Alignment run on the waiting decisions and the cards

The owner asked on 2026-11-22 for an alignment run on the question's component. Two parts
were measured at 1440: a waiting decision above the message box, and a request card in a
thread and on a sprint page. Each part's box edges, ink edges and true baselines were read
from the page.

The card had no fault. Its title, summary, recommendation and buttons start on one edge,
its title and age share a baseline, and its buttons share a baseline.

A waiting decision had five faults. All five are fixed in `design/mock/css/mission.css`.

| Fault | Before | After |
|---|---|---|
| The decision's content and the message box's placeholder had two left edges | 14 and 12 pixels | 12 and 12 |
| A title after "Approval" started later than a title after "Question" | 86.4 and 85.4 | 86 and 86 |
| The kind stood above the title's baseline | 1.6 pixels | 0 |
| On a title of two lines the kind, the asker and the time sat between the lines | centred | on the first line |
| "Recommended" stood above its answer's baseline | 1.9 pixels | 0 |

The note under a marked answer also lost its coloured bar, and it starts on the content's
edge. The owner had the coloured edge removed from these decisions on 2026-11-20.

A row is still 36 pixels tall and an answer is still 38. The worst-case title of six lines,
a marked answer, the lead's own question in another project and 390 pixels wide were
captured, and nothing breaks.

The owner then said yes to the asker's name in full on an open decision. The title wraps
beside the name. A title of more than 48 characters takes the name's room back. The limit
is `LONG_TITLE` in `design/mock/js/screens/mission.js`. This closes issue 1568.

