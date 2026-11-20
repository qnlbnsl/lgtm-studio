# The workspace folder in Service settings

The owner approved one workspace control on 2026-11-20.
It appears first in Service settings, under Defaults.
It uses the existing setting row, input, monospace path style and helper text.
No stylesheet or design token changed.

The label is "Where is your workspace?".
The helper says, "LGTM Studio's Git commands work in every repository under this folder."
The field saves when the owner leaves the changed input.
A focused draft survives background settings refreshes.
The saved lead defaults and their explicit permission marker are preserved.
The engine refuses a folder that excludes a registered repository.
The existing error toast displays the engine's refusal and the field restores the saved path.

The [desktop capture](workspace-settings-desktop.png) is 1440 by 1000 pixels.
The [narrow capture](workspace-settings-narrow.png) is 390 by 844 pixels.
Both captures show the standalone mock with Maya Okafor's sample path.
The narrow layout stacks the existing row without horizontal page overflow.
The workspace control remains inside the viewport at both widths.

The focused browser journey opens the root address of a temporary test engine.
It verifies a wider folder after reload, both refusal messages and the complete saved settings record.
A separate sample journey changes the path and returns to the section before taking these captures.
The engine uses a fake harness runner and temporary state for the live journey.
No physical phone or real harness account was used.
