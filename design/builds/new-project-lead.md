# New project lead selection

The owner requested four editable lead choices in the existing New project dialog on 2026-11-19.
The existing dialog is the reference for the placement, native fields, spacing and footer.
The Field and Dusk palettes remain the design contract.

## Screen plan

| Region | Container and density | Interactive parts |
| --- | --- | --- |
| Goal | Existing form field, three textarea rows. | Native textarea. |
| Placement | Existing two-column form row. | Native machine and repository selects. |
| Project lead | Two form rows, two columns, with no additional card. | Native harness, subscription, model and effort selects. |
| Submission | Existing dialog footer. | Native Cancel and Create project buttons. |

There is no main-page offset because this is a dialog above the existing page.
The lead fields use the existing form styles, with zero minimum grid-column width so long subscription names cannot enlarge the dialog.
No new motion, component library, navigation or route is introduced.

## Behavior and validation

The saved lead supplies the initial selections.
Changing the harness selects an available account for that harness.
Changing the account clears the old model and effort.
Changing the model clears the old effort and reads its actual choices.
Creation waits for the current report and uses a named account.
An unavailable report offers the harness default and explains the missing choices.
The authenticated engine validates the lead before allocating a project.

## Captured states and checks

The populated dialog was captured at 1440 by 1000 pixels and 390 by 844 pixels.
The browser fixture includes a long subscription name, multiple harnesses, multiple accounts for one harness, and a model with no reported effort choices.
The journey checks initial defaults, account-specific model options, clearing stale choices, disabled unavailable effort, successful creation and the actual runner input.
Every select fits its viewport and passes center hit-testing at both widths.
The captures disable resize transitions after the browser checks actual visibility.

The space gate was manual, using the screen-build space rules and the two captures.
There are no repeated cards, covered controls, excessive row gaps or horizontal overflow in the dialog.
List-row and main-list-offset checks do not apply to this form.
No automated eight-check space report was produced.
The optional detector reported no findings on the changed form files.

## Independent visual review

The reviewer inspected the captures without the plan or source and returned a ship verdict.
Three minor findings concern clipped native-select labels: the long subscription at both widths, and the existing machine label on the narrow layout.
Those are retained because the native selector opens its full option label and the controls remain operable.
No blocker or major finding was reported.

## Verification and limits

The full TypeScript checks pass.
The creation-recovery suite passes all 26 tests.
The Studio suite had 793 passing tests and one outdated exact mapping expectation, which was corrected; the two affected mapping suites then passed all 33 tests.
An independent agent reran the two new engine regressions and the browser journey successfully.
The existing standalone mock smoke journey also passes.
The credits check and whitespace check pass.

The browser test uses the real engine, authenticated API and durable project creation, with a fake harness runner and synthetic credentials.
Physical mobile devices and real provider launches were not exercised by that test.
The review was behavioral and visual; a separate full code-review round was not run.
Loading and unavailable-effort states were exercised, but only the final populated state was captured.

The captures are [desktop](new-project-lead-desktop.png) and [narrow](new-project-lead-narrow.png).

