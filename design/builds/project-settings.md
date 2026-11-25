# The Settings of a project

surface:    app screen
reference:  Mobbin, searched on 2026-11-23. Taken from them: short pages reached from a standing list instead of one long scroll ([Devin](https://mobbin.com/screens/8195d766-e909-46b9-8c6b-b6e113aff8c1), [Navattic](https://mobbin.com/screens/9a8cf272-67e8-4782-8792-f2711dcb09d9)); a name at the left and one control beside it, in a capped column ([Graphite](https://mobbin.com/screens/102736c8-9a2a-4185-a155-025f1c0b3217)); the destructive actions as the last section, each a row with its button ([Campsite](https://mobbin.com/screens/0d82287a-6895-48d6-86fe-23fe075299df), [Neon](https://mobbin.com/screens/bc404d49-1da8-4a3c-83f4-888b5777ff96)). No artwork, wording or brand was copied.

## The critique of the old page

An independent reviewer read seven pictures of the old page and its source, without my notes.
It scored Nielsen's ten heuristics at 19 of 40. The lowest two were consistency and minimal design, at 1 each.
The detector of the design plugin found nothing in the two source files.

| Finding | Severity | What was done |
|---|---|---|
| One setting had two names on two pages: "Write tests first" and "Run tests". "Agent review" was on both. | major | Both are on Workflow only. |
| The permission was five radio buttons with a sentence each, 290 pixels tall. | major | One select. The sentences are in the row's help. "Full access" asks first. |
| The Teams table was cut off at "Delete tea", with three red buttons. | major | The Agents column is gone. Delete is an icon at the end of the row. |
| An assignment waiting for approval sat 1,000 pixels down. | major | A dot shows on the Settings tab and on its Agents page. The button reads "Approve assignment". Raising it in Questions is not built. |
| A third of the page was facts, repeats and a title that said "Project" a third time. | minor | The title is gone. The facts are one line. The lead's model is a row of the Models table. |
| The two pages had two layouts. | minor | Both use the same row, with the control in one column. |
| There were three ways of saving. | minor | Two: a field saves on leaving it, and the goal keeps its buttons. |
| "Apply" and "Approve" did not name their outcome. | minor | "Use for Plan, Build, Review and Test" and "Approve assignment". |

## The plan

| # | Region | Job | Container | Why this container | Rows in the first viewport | Empty state |
|---|---|---|---|---|---|---|
| 1 | Row of pages | Switch among General, Agents, Workflow, Previews | none | The product's sub-tabs, as in Status and Design | 1 | never empty |
| 2 | General | Name, goal, base branch, machine, archive, delete | form | Six unlike settings | 6 | never empty |
| 3 | Agents: Models | Harness, model and effort for the lead and four kinds of work | table | Five like rows with three like columns | 5 | never empty |
| 4 | Agents: Permissions | What agents and the lead may do | form | Three selects | 3 | never empty |
| 5 | Agents: Teams | Need-to-know and delete, for each team | table | Like rows | 3 | "None yet" beside the heading |
| 6 | Workflow | Tests, review, merging | form | Nine unlike settings | 9 | never empty |
| 7 | Previews | Name, command, port and machine of each preview | table | Like rows that are edited in place | 3 | "None yet", and the empty last row |

main offset: 0.08. Only the row of pages and one heading stand above the first row.
navigation: four destinations, switched often, so a row of sub-tabs.
parts: native selects and inputs; the product's own switch, dialog and help mark. No new library.
motion: none, beyond the help mark's fade on hover.
feedback: "Saved" at the end of the row of pages after a change; a toast after a rename, a goal and a preview.

captured:   1440 by 900 dark and 1000 by 680 light, each of the four pages, populated. Also: a hovered row with its help, focus on a field and on a tab, the "Full access" dialog and focus inside it, a hovered bin, an edited goal, a refused preview, and three pages at 320 pixels.
worst case: not done.
gate:       the space checks at both sizes: no finding on Agents and Workflow. General is 34 percent empty under its last row at 1440 by 900, and Previews is 73 percent empty. Neither is waived, and neither is fixed: a short page of settings is short.
access:     the scripted checks at 1440 by 900. 2.4.7 and 2.4.11 pass after the fixes. 1.3.1, 1.4.3, 1.4.12 and 2.5.3 fail in the shell around the page, as issue 1619 records. 4.1.3, 1.4.13 and 2.1.4 are not tested. No criterion was tested by hand beyond the Tab walk.

## The blind review of the new pages, 2026-11-23

One reviewer read 22 pictures, the design contract and the rubric, and never the plan above. Its grades: layout weak, hierarchy holds, spacing weak, parts fails, colour holds, access fails. It would not ship the pages as they were.

| # | Finding | Label | Rule | Plan row | Answer |
|---|---|---|---|---|---|
| 1 | The port field of a preview was 22 pixels wide and showed half a digit. | blocker | H1 | 7 | Fixed. The table has fixed columns, and the port field is 62 pixels wide. My note above, "No table is cut off", was wrong for a pane of 660 pixels. |
| 2 | At 320 pixels the names kept their column, and the controls showed one letter. | major | 1.4.10 | 2, 4, 6 | Fixed. A row stacks below 720 pixels. |
| 3 | A refused preview showed a toast far from the row. No field was marked. | major | 3.3.1, 3.3.3, H9 | 7 | Fixed. The field is marked, and a line under the row says what was refused and what to do. |
| 4 | An open help covers the next row's value. | minor | H8 | 2 | Not fixed. A help that opens over the page covers something. |
| 5 | "Workstation" was cut to "Worksta". | minor | none | 7 | Fixed with the columns of finding 1. |
| 6 | The select beside "Use for Plan, Build, Review and Test" has no name in sight. | minor | H6 | 3 | Not fixed. Issue 1617. |
| 7 | One waiting state was shown three ways, and its button was as quiet as its neighbour. | minor | H4 | 3 | Fixed. The tab carries a dot, and "Approve assignment" is the filled button. |
| 8 | "Need-to-know", "Remove agent credits" and "Stuck at" are the product's words. | minor | H2 | 5, 6 | Not fixed. Each is explained in its help. Issue 1617. |
| 9 | "Running" reads as a state, not a link. A control that is off gives no reason in sight. | minor | H4, H1 | 7, 2 | Not fixed. The owner's rule is no helper sentences on a screen. |
| 10 | Workflow opens on a line of facts with no heading. | minor | H4 | 6 | Not fixed. |

review:     one blind reviewer, after the build. The findings and answers are above.
not checked: worst-case data; a phone as a platform; the older layout used when the live engine is on; an archived project; the "Saved" mark, the empty Teams table and the light theme's dialog, which the reviewer was not shown; a screen reader.
