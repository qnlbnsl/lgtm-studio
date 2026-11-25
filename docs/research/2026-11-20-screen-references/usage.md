# Usage: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

Everything in this section comes from `design/mock/js/screens/usage.js`. The page is a draft on sample data, and it exists in the mock only. `design/mock/js/app.js` leaves the Usage row out of the sidebar on the live website.

- The page head shows the title "Usage" and a period switch with three choices: Today, This week, and 30 days.
- One line under the head gives two totals for the chosen period: the tokens, and the dollars "at API prices". API means application programming interface. The prices in the file are invented.
- The page then has three sections: "Accounts", "Where it went", and "Findings".
- "Accounts" is a table with five columns: Account, Limits, Tokens, Share, and At API prices. Each harness opens a darker band row with its own totals, and its accounts follow.
- An account row shows the account's name and its address, and no machine.
- The Limits cell lists every limit the account reports, one per line: a label, a bar, a percentage, and "Resets in". The sample has "5 hours", "This week", and a third limit, "This week · Opus", on one account. The bar takes the warning colour at 80 percent and the error colour at 100 percent.
- "Where it went" is one table that opens level by level. A switch groups it by Project, by Stage, or by Account. The levels are project, stage and agent, in the order the switch sets.
- Each row of that table shows a name, a model on agent rows, tokens, a "Share of the row above" bar, and the cost. Rows sort by tokens. A row opens and closes on its own, and the first row starts open.
- An agent row names the agent by role, links to the agent, and shows its team.
- A row that a waiting finding is about carries a small "1 finding" button. The button opens that finding and scrolls to it.
- "Findings" is a table with four columns: Finding, Tokens, Share of this week, and At API prices. Its heading shows when the usage review ran, such as "Usage review · 2 h ago".
- A finding row shows a title and a line with the project, the stage and who it is about. An open finding shows "Cause" at the left with a "Show where it went" button, and "Guardrail" in a box at the right.
- The guardrail box shows the proposed rule, where it would go, such as "Build loop · Plan review", and a two-way switch between the repository's name and "Every repository". Its buttons are "Add guardrail" and "Dismiss".
- A finding that was added or dismissed moves under a folded "Handled" row with a count. "Reopen" puts it back.
- The page has no chart, no trend over time, and no export.

## What is planned and not built

- `docs/features.md`, "A Usage page for every harness account", under "What the owner said on 2026-11-20": the page must not assume one shape of limit, because harnesses report limits differently. The breakdown should support a report such as "planning took 30% of your tokens, and 25% of that 30% came from Planner 1".
- The same section, under "What the owner said about the draft on 2026-11-20": the owner judged the page acceptable and open to improvement. The owner judged that the findings had the right details and a very poor layout. The draft changed in answer, and the changed draft awaits the owner's approval.
- The same section, under "What is undecided": where the dollar prices come from is undecided. Which limits each harness can report is undecided. Which agent writes the findings, and when it runs, is undecided. Whether a guardrail needs the owner's approval before it takes effect is undecided.
- The same list says that the engine's stored turn usage holds input and cache token counts, no price, no stage, and no output token count in the record that was read. The engine and the client API have no usage read for the website yet.
- The same section, under "Where a guardrail goes": a guardrail for one repository would go into that repository's `.lgtm` folder, and a guardrail for every repository would go into `~/.lgtm`. The designer recommends that a guardrail attach to one step or one role of the workflow. This awaits the owner's decision.
- `docs/features.md`, "Short instruction files for agents": a size figure for instruction files on the Usage page, and a finding when one grows, are possible follow-ups and are not designed.

## How other products do it

### Vercel: usage for a billing period, grouped by product

[Mobbin screen](https://mobbin.com/screens/7a6a80ba-6db0-426c-9813-60ef6e50554b?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A row of five controls sits under the title: a period menu, a date range, a product filter, a project filter, and a "By Product" grouping menu. An "Included Credit" bar reads "$1.25 / $20.00". A chart named "Consumption Breakdown" shows stacked bars per day, with a Daily, Weekly and Monthly switch and a "Cumulative" tick box. The table below has four columns: Product, an unlabelled trend line, Usage, and Charge. Rows sit under plain group headings such as "Vercel Functions", and a usage value with a quota reads "89 / 10M" beside a small ring.

Difference from LGTM Studio: Vercel shows change over time, in a chart and in a trend line on every row. LGTM Studio shows one figure per row for the chosen period.

### Vercel: the end of the usage table, with its totals

[Mobbin screen](https://mobbin.com/screens/e828f30f-3564-432d-8a2f-33b3116dd465?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The table continues through five group headings, and every row keeps the same four columns. Each row starts with a coloured dot that matches its colour in the chart. The table ends with four right-aligned total lines: two subtotals, "Credits Applied", and "Total". A muted line under the table reads "Usage data can be up to 1 hour old."

Difference from LGTM Studio: Vercel puts the total at the end of the table and says how fresh the figures are. LGTM Studio puts the total above the first table and gives no freshness.

### Vercel: the period menu

[Mobbin screen](https://mobbin.com/screens/8e385d03-bd42-4376-b1de-62263908900a?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The period menu lists seven presets, from "Current Billing Cycle" to "Last 12 Months". A second column in the same menu shows examples of typed periods, such as "45m", "last month" and "1/1 - 1/2". A separate control beside the menu shows the exact start and end.

Difference from LGTM Studio: Vercel offers the current cycle, the previous cycle, and typed ranges. LGTM Studio offers three fixed periods. `usage.js` does not say whether "This week" follows an account's weekly reset.

### OpenAI Platform: spend, a budget, and a breakdown in one view

[Mobbin screen](https://mobbin.com/screens/fe36ec31-b25c-4cf1-9932-85bf66cd89fc?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The head holds a project filter chip, a date range, and "Export". The main area shows "Total Spend" as a figure over a bar chart, with a "Group by" menu and a range slider under the chart. A right rail stacks three small blocks: a monthly budget bar with "Resets in 28 days" and an "Edit budget" link, "Total tokens" with a trend line, and "Total requests" with small bars. Under the rail, three tabs named Users, Services and API Keys switch a short ranked list. Under the chart, two tabs switch a grid of small cards, one per kind of request, each with its counts and its own small chart.

Difference from LGTM Studio: OpenAI Platform keeps the limit, the totals and a ranked list in a rail beside the chart. LGTM Studio stacks its three sections in one column.

### OpenAI Platform: input and output tokens on hover

[Mobbin screen](https://mobbin.com/screens/b781c7ac-618d-4946-9ef2-808f6f933d71?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

Hovering a point on the "Total tokens" trend opens a small box. The box shows the date, a total, and two lines: "Input 10.354K" and "Output 5.888K". The rest of the page does not change.

Difference from LGTM Studio: OpenAI Platform splits tokens into input and output. LGTM Studio shows one blended token figure, and `docs/features.md` records that the engine has no output count yet.

### OpenAI Platform: a project's budget with an alert

[Mobbin screen](https://mobbin.com/screens/8cf8d3eb-515b-4ff9-b14c-dddcfb207adf?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

"Project budget" is one card with "$0.09 / $5.00", a bar, and "Resets in 29 days". Two buttons, "Add alert" and "Edit budget", sit in the card's head. Each alert is one row inside the card, such as "100% usage alert", with a bin and a pencil. A second block, "Model usage", has two rows that each end in a "Select models" button: one to allow or block models, and one for rate limits.

Difference from LGTM Studio: OpenAI Platform lets the owner set a threshold and be told when it is crossed. LGTM Studio's guardrails come from an agent's finding after the tokens are spent.

### Cursor: usage by model, with a log of single events

[Mobbin screen](https://mobbin.com/screens/3a84294a-2371-44ba-8a09-fdfd163fb453?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A card named "Your Usage" holds a cumulative area chart and two menus, "By Model" and "Spend". Hovering a day opens a "Daily Breakdown" box with each model's cost and its share in percent, a daily total, and a cumulative total. Under the card, a date range sits beside three short buttons, "1d", "7d" and "30d", and "Export CSV" sits at the right. CSV means comma-separated values. The table below is a flat log with five columns: Date, Type, Model, Tokens, and Cost. Cost reads "Included" or "Free" for use that the plan covers.

Difference from LGTM Studio: Cursor lists single events in time order. LGTM Studio shows sums by project, stage and agent, and it has no event log.

### Cursor: included usage as a two-level table

[Mobbin screen](https://mobbin.com/screens/320d82ae-c555-4345-ad3e-eba3903115fb?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

"Included Usage" is a table with three columns: Item, Tokens, and Usage. A bold parent row reads "API", "8.2M tokens" and "4.8%". Two indented rows under it name single models with their tokens and their percentage. The percentage is a share of what the plan includes. A second block, "On-Demand Usage", shows "$0.00 / $5.00" for paid extra use.

Difference from LGTM Studio: Cursor's percentage says how much of the plan's allowance a model used. LGTM Studio's Share column says how much of all tokens a row used, and it does not tie a project to an account's limit.

### Claude: two limits as two rows

[Mobbin screen](https://mobbin.com/screens/ea02c392-d84c-4ce0-81bb-66d7bda183b3?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

"Plan usage limits" shows the plan name at its right edge. "Current session" is one row with a full-width bar and "0% used". "Weekly limits · All models" is a second row with "Resets Tue 5:00 AM" and "21% used". A line reads "Last updated: just now" with a refresh icon. A switch named "Extra usage" sits under a divider.

Difference from LGTM Studio: Claude shows only limits, with no tokens and no cost. LGTM Studio fits the same limits into one cell of the Accounts table.

## Options for LGTM Studio

1. Add a trend line to every row of "Where it went", as Vercel does in its product table. The row keeps one line, and the owner sees whether a project's use is rising. This needs use stored per day, and the engine has no usage read for the website yet.
2. Add one chart of use per day above "Where it went", stacked by the chosen grouping, as Vercel and OpenAI Platform do. A chart shows a spike that a sum hides. It costs a screen of height, and it needs a colour for each group.
3. Move the limits out of the Accounts table into rows of their own at the top, as Claude does, with a "Last updated" time. Each limit gets a full-width bar, which reads better on a phone. The Accounts table then loses its widest column. The cost is that limits and tokens for one account are no longer on one line.
4. Add an "Of the weekly limit" column to "Where it went", following Cursor's included-usage table. A row would then say what share of an account's allowance a project or agent consumed. The risk is that a harness reports a limit only as a percentage, so the share may have to be estimated.
5. Add a flat log under "Where it went", as Cursor has: one row per turn or per agent session with the time, the agent, the model, tokens and cost. It answers "what just used the tokens". It adds a fourth section to a page that already has three.
6. Add an alert on a limit, as OpenAI Platform does with "Add alert". The owner would set, for example, 80 percent of the five-hour limit, and LGTM Studio would send a notification. This is a guardrail the owner sets before the tokens are spent. It needs a new alert kind next to "An account needs your attention" on the Notifications page.
7. Put the totals, the limits and the findings count in a rail at the right, as OpenAI Platform does. The breakdown table would get the full height of the screen. A rail does not fit a phone, so it would have to move above the table there.

## Ideas that are not in the feature list

- An export of the usage table to a file (Cursor, Vercel, OpenAI Platform).
- A period that follows the account's own reset, and a "previous period" choice to compare with (Vercel).
- A typed or picked custom date range (Vercel, OpenAI Platform).
- A line that says how old the figures are (Vercel, Claude).
- Input and output tokens shown apart (OpenAI Platform).
- An alert when a limit passes a chosen percentage (OpenAI Platform).
- A list of models that one project may or may not use (OpenAI Platform).
- A "Cumulative" switch on a chart of use (Vercel, Cursor).
- A "Type" column that separates use the subscription covers from use that is billed (Cursor).

## Searches that were run

- "Vercel usage dashboard": 8 results, all from Vercel, and 5 were usable.
- "OpenAI platform usage page with token spend by project": 8 results, all from OpenAI Platform, and 6 were usable.
- "Claude settings usage limits with progress bars and reset times": 1 result, from Claude, and it was usable.
- "Cursor dashboard usage table with tokens and cost per model": 8 results, all from Cursor, and 6 were usable.
- Named products that returned screens: Vercel, OpenAI Platform, Claude and Cursor. ChatGPT, Datadog, Sentry, Render and Railway were not searched by name, because the budget of 14 searches was spent.
- No search was spent on the Findings section, so it has no direct reference. No result from the four searches shows a review that names a cause and proposes a rule. No result shows a table that opens level by level with a share of the row above. The closest is Cursor's two-level included-usage table.
