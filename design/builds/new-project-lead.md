# New project lead selection

The owner requested editable harness, subscription, model and effort choices on 2026-11-19.
He then rejected the wide paired-field layout and explicitly chose the chat composer's single row of dropdowns.
The reference is the existing chat composer, including its shared Menu component and pick button style.
The Field and Dusk palettes remain the design contract.

## Screen plan

| Region | Container and density | Interactive parts |
| --- | --- | --- |
| Goal | Existing form field, three textarea rows. | Native textarea. |
| Placement | Two compact rows, with a shared label edge. | Native machine and repository selects. |
| Project lead | One compact toolbar row, with a hairline above the section. | Four shared Menu dropdowns, using the chat composer's pick style. |
| Submission | Existing footer, able to wrap a longer refusal sentence. | Native Cancel and Create project buttons. |

The dialog is at most 520 pixels wide, reduced from 880 pixels.
There is no main-page offset because this is a dialog.
The lead row keeps the harness, subscription, model and effort together at both shipping widths.
Only the subscription takes the remaining row width, so a long account name cannot squeeze an ordinary model name to a single letter.
Long trigger values can shorten, as in the chat composer.
Their titles retain the complete value.
The subscription menu wraps its full names and shows the account identifiers.
No new component library, navigation, route or motion was introduced.

## Behavior and validation

The saved lead supplies the initial selections.
Changing the harness selects an available account for that harness.
Changing the account clears the previous model and effort.
Changing the model clears the previous effort and reads its actual choices.
Creation waits for the current report and uses a named account.
The shared Menu component now accepts an optional disabled flag.
Existing callers remain enabled by default.
Model and effort dropdowns are disabled while loading or unavailable, and explanatory text appears beneath the row.
The authenticated engine validates the lead before allocating a project.
The saved global defaults stay unchanged.

## Captures and checks

The populated dialog and its open subscription menu were captured at 1440 by 1000 pixels and 390 by 844 pixels.
The fixture includes a long subscription name, multiple harnesses, two accounts for one harness, and a model without reported effort choices.
The real-engine browser journey selects through the shared menus and verifies the launched runner input.
It also verifies defaults, account-specific options, clearing stale choices, disabled effort, bounds and center hit testing.
The existing standalone sample journey exercises the same four dropdowns and passes.
The Studio and root TypeScript checks pass after the interaction changes.
The website build and whitespace check pass.
The earlier engine regressions are unchanged by this layout revision.
The whole repository suite was not rerun for the layout change.

The space gate was manual, using the screen-build space rules and all four captures.
No repeated cards, covered closed controls or horizontal overflow were found.
An open menu can temporarily cover footer actions, as in the existing shared component.
List-row and main-list-offset checks do not apply to this form.
No automated eight-check space report was produced.

## Independent visual review

A fresh reviewer inspected the final captures without the source or screen plan and returned a ship verdict with no findings.
The earlier stacked-field proposal was superseded by the owner's explicit instruction to use one chat-style toolbar row.
The final review checked both the closed row and the full-name subscription menu.

## Evidence and limits

The closed dialog captures are [desktop](new-project-lead-desktop.png) and [narrow](new-project-lead-narrow.png).
The open subscription captures are [desktop](new-project-subscription-desktop.png) and [narrow](new-project-subscription-narrow.png).
The fixture uses the real engine, authenticated API and durable project creation, with a fake harness runner and synthetic credentials.
Physical mobile devices and real provider launches were not exercised by the fixture.
Loading and unavailable-effort states were exercised, but only the final populated and open-menu states were captured.
There was no separate full code-review round for this layout revision.
