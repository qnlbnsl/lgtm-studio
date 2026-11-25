# Harnesses and accounts: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

Everything in this section comes from `design/mock/js/screens/service.js`, in the functions `Harnesses`, `SubRow`, `HarnessDefaults`, `AddSubscription` and `HarnessSignIn`.

- The page head shows the title "Harnesses", a "Saved" mark, and a "Check again" button. A line under it sums up the accounts, such as "4 of 5 signed in, 1 at its limit".
- A jump row lists every harness with a dot, its name, and "2 accounts", "No account" or "Not installed". Clicking an item scrolls to that harness.
- A first card, "Defaults for new project leads", holds three controls: Harness, Model, and Effort.
- Each harness then has its own card: Claude Code, Codex, Gemini CLI, and Cursor. The card head shows the name, a badge such as "Installed · 2.4.1" or "Not installed", and an "Add account" button.
- Each harness card has three titled parts: "Program", "Defaults for this harness", and "Accounts".
- "Defaults for this harness" holds Run mode, Default model, and Default effort. Run mode offers "Agent Client Protocol (ACP)" and "Interactive".
- "Accounts" is a grid with seven columns: Account, Status, 5 hours, This week, Agents, Model for new agents, and actions.
- The Account cell shows the account's name, a "Default" badge, and a second line with the account address and the machine name.
- The Status cell is a dot and one of seven words: Signed in, Signed out, Turned off, Limit reached, Weekly limit reached, Near its limit, or Not reported.
- The "5 hours" and "This week" cells each show a bar, a percentage, and a reset line such as "Resets in 2 h" or "Resets Thursday". The bar takes the warning colour at 80 percent and the error colour at 100 percent. A harness that reports no figure shows "Not reported".
- The Agents cell shows a count and how many are running. Opening the row lists those agents with their project, model, effort and state. Each row opens and closes on its own.
- A signed-out account shows a primary "Sign in" button in its row. The row menu holds the default choice, Turn off, Rename, Check sign-in again, Sign out, and Remove.
- An account at its limit shows a sentence under its row. The sentence says that its agents wait and try again until the reset time.
- "Add account" opens a dialog. For a harness that is not installed, the dialog first shows the install commands, the package names, a link to the official instructions, and an "Install" button that works only for a harness with an automatic recipe. The account step then asks for a Name, a Machine, and a Login folder, and it offers "Sign in here", which opens a terminal inside the dialog.
- The old address `/subscriptions` opens this same page.

## What is planned and not built

- `docs/issues.md`, issue 1546, "The mock's Harnesses and Machines pages tie each account to one machine": the owner stated on 2026-11-20 that accounts are signed in through the server, so every machine has the same accounts. This page still shows a machine on each account, words its default as "Use for new agents on" a machine, and asks for a Machine when adding an account. The issue is open and needs the owner's decision.
- `docs/features.md`, "A simpler way to set up harnesses and subscriptions": the owner decided on 2026-11-14 that adding a harness account must become simpler. The page and its "Add account" dialog need an overhaul.
- `docs/features.md`, "Offer a sign-in that is already on the machine": the page should notice an existing sign-in and offer it in one step. It must not add the account by itself.
- `docs/features.md`, "Sign in to a harness from the page, with a login folder for each account": "Add account" runs the harness's sign-in in a terminal beside the dialog, and each account gets its own login folder under `~/.lgtm`. A builder is building both.
- `docs/features.md`, "A harness installer": Gemini CLI installs from the page. Claude Code and Cursor show their official commands for the owner to run by hand.
- `docs/features.md`, "A choice between the bundled harness program and the one installed on the machine": a setting for each harness could offer "Bundled with LGTM Studio" and "Installed on this machine". The owner has not ruled on it.
- `docs/features.md`, "A website control for a harness's default permission": the page has no control for a harness's default permission. It is recorded as an idea.
- `docs/features.md`, "Custom harnesses": the owner wants a simple way to declare a custom harness. It is an idea.
- `docs/features.md`, "A Usage page for every harness account", under "What is undecided": which limits each harness can report is undecided, and so is the form for a harness that reports neither a five-hour nor a weekly limit. This page has two fixed columns for those two limits.
- `design/DESIGN.md`, "Machines and subscriptions": usage of 90 percent or more is amber. The code in `service.js` turns amber at 80 percent.

## How other products do it

### Claude: plan usage limits in Settings

[Mobbin screen](https://mobbin.com/screens/ea02c392-d84c-4ce0-81bb-66d7bda183b3?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

"Usage" is one entry in a settings list. The section "Plan usage limits" shows the plan name, "Pro", at its right edge. Each limit is one row: a label and a reset line at the left, a long bar in the middle, and "21% used" at the right. The two limits are "Current session" and "Weekly limits · All models", and the weekly one reads "Resets Tue 5:00 AM". A line under them reads "Last updated: just now" with a refresh icon.

Difference from LGTM Studio: Claude lists limits as rows, so a third limit is one more row. LGTM Studio gives the five-hour and weekly limits fixed columns, and it has no time of last report.

### Braintrust: model providers as one table

[Mobbin screen](https://mobbin.com/screens/df7dc902-f414-4799-80e0-6c0b833d0fe0?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The page "AI providers" is one table with three columns: Name, Status, and Last updated. Each row holds a logo, the provider's name, and the name of its secret in small monospace text. Status reads "Configured" with a tick or "Not configured" in muted text. A pencil icon at the end of each row edits it, and a bin icon appears only on configured rows. About thirteen providers fit on one screen.

Difference from LGTM Studio: Braintrust gives a provider one line of a table. LGTM Studio gives a harness a card with three titled parts, so four harnesses fill several screens.

### Retool: a built-in provider set apart from the ones the owner sets up

[Mobbin screen](https://mobbin.com/screens/fb50d9b9-96bc-4f79-ad46-c19672f1552c?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The page has three stacked blocks. "Retool managed" shows one provider with an "Enabled" badge and a "Disable" button. "Default model" is one select. "Use your API keys" lists at least five more providers, each with a "Disabled" badge and a "Set up" button. API means application programming interface. Two tabs at the top split "API keys" from "Usage (2)".

Difference from LGTM Studio: Retool separates what arrives with the product from what the owner must set up. LGTM Studio lists bundled and not-installed harnesses as equal cards, and tells them apart with a badge.

### Coda: connected accounts with "Sign in again"

[Mobbin screen](https://mobbin.com/screens/8c9b4a22-2d88-4a27-b78c-51892a141f97?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

Under "Accounts connected to packs", each service is a heading with its account address as a row beneath. An opened account shows what uses it, with a "Disconnect" link for each item. The opened row also holds a "Sign in again" button at the left and a red "Remove this account" link at the right. Services with no account sit in a second list, "Other integrations", each with a "Connect" link.

Difference from LGTM Studio: Coda shows repair and removal as visible buttons inside the opened account. LGTM Studio keeps Check sign-in again, Sign out and Remove in a menu of three dots.

### Amie: a list of accounts that ends in "Add account"

[Mobbin screen](https://mobbin.com/screens/e9a97565-52fe-4bf5-9061-cded8fae8eb7?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The list holds one row per account with an avatar, a name, and an address. A menu of three dots sits at the end of a row. The last row of the list is a full-width "+ Add account" button. A toast at the bottom right reads "Account added" and offers the next step.

Difference from LGTM Studio: Amie puts the add action where the next account will appear. LGTM Studio puts "Add account" in the head of the harness card, away from the list.

### Vercel: sign-in methods with their state in the row

[Mobbin screen](https://mobbin.com/screens/1817e0ea-db41-4bd9-861e-3cc867b97fa9?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

"Sign-in Methods" is one list in which every method is a row. A connected method shows the account name under its title and "Last used Apr 21" with a menu at the right. A method that is not connected shows a filled "Connect" or "Add" button at the right. The less common methods fold under a "More options" row that shows only their icons.

Difference from LGTM Studio: Vercel shows a method that is available and unused as a row with one button. LGTM Studio lists an account only after someone adds it, which is the gap that "Offer a sign-in that is already on the machine" describes.

### Cursor: included usage with a total that opens into its parts

[Mobbin screen](https://mobbin.com/screens/77cb9e3b-d6d4-4780-ae58-d1cb1ffcab4f?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The "Spending" page starts with a plan card that reads "Pro", "$20/mo" and "Resets on Apr 24 (29 days)". Under "Included in Pro", a "Total" row shows a thin bar and "3%". A line under it, "0% Auto and 11% API used", opens two sub-rows that each have their own bar and percentage. A last block, "On-Demand Usage", shows "$0.00 / $5" and a monthly limit with a select, a number field and "Save".

Difference from LGTM Studio: Cursor shows one total bar first and the separate pools on request. LGTM Studio shows every limit of every account at once.

### Langdock: default models and the keys that serve them

[Mobbin screen](https://mobbin.com/screens/3255e6bd-ff69-477f-9762-ef14f56b35cc?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The "Models" page starts with two rows, "Default chat model" and "Image generation model", each with a select at the right. A tabbed list of models follows, with "Edit" and a bin on each row. A last block, "Keys", lists each key with its provider, the line "Used by 1 model", and the same two actions. "Add key" sits at the right of the block's heading.

Difference from LGTM Studio: Langdock puts the defaults first and the credentials last, on one page with no cards. LGTM Studio repeats a block of defaults inside every harness card, above the accounts.

## Options for LGTM Studio

1. Make each harness one table row, as Braintrust does with providers. The row would hold the name, "Installed · 2.4.1" or "Not installed", and the number of accounts. Opening the row would show its accounts and its defaults. This fits the owner's wish for dense rows. It hides the usage bars until a row is opened.
2. Split the page into "Bundled" and "Install", as Retool does. Claude Code and Codex would sit in the first block with their accounts. Gemini CLI and Cursor would be one-line rows with an "Install" button. This removes two near-empty cards. It needs a rule for where a harness moves after it is installed.
3. Show an account's limits as a list of rows, as Claude does, in place of the two fixed columns. Each limit would have a label, a reset line, a bar and a percentage. This answers the undecided question about harnesses that report other limits. It makes an account taller than one line.
4. Remove the machine from the account line, from the default's wording, and from the "Add account" dialog. This follows issue 1546 and the draft Usage page, which already dropped the machine. It needs the owner's decision, and the sample data in `design/mock/js/data.js` gives every account one machine.
5. Show a sign-in that was found and not yet added as a row with one button, as Vercel shows an unconnected method. The button would read "Use this sign-in". This follows "Offer a sign-in that is already on the machine". The risk is that a row looks like an account before the owner has agreed to it.
6. Put "Add account" as the last row of each accounts list, as Amie does, and put "Sign in again" and "Remove" inside the opened account, as Coda does. This costs little. It adds a visible destructive link to every opened account.
7. Take the usage bars off this page and show them only on the Usage page, as Cursor separates its settings pages from Usage and Spending. An account row would keep its state word, such as "Near its limit". The risk is that the owner chooses a default account here and can no longer see how full each account is.
8. Move "Defaults for this harness" out of each card into one block at the top, as Langdock does. The block would be a short table with one row per harness and three columns: Run mode, Default model, Default effort. This removes one titled part from each card.

## Ideas that are not in the feature list

- A "Last updated" time on the limits, with a refresh action (Claude). LGTM Studio has "Check again" for sign-in and shows no time for the last limit report.
- A "Last used" date on an account (Vercel).
- A spending cap on paid extra use, set as a fixed amount (Cursor).
- A checklist of which models a harness offers in pickers. This was seen on the Twenty screen that the "AI model provider settings" search returned: [Mobbin screen](https://mobbin.com/screens/ecb8c8e1-26f5-431e-9360-8839dbf6d48e?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp).
- A fallback model that is used when the first one fails because of its provider. This was seen on a Relevance AI screen: [Mobbin screen](https://mobbin.com/screens/c2a24da7-c90a-447c-a418-52feaad57343?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp). For LGTM Studio it would be a second account that takes over when the first reaches its limit. The sections of `docs/features.md` that were read for this file do not mention it.

## Searches that were run

- "Claude settings usage limits with progress bars and reset times": 1 result, from Claude, and it was usable. It is the closest match to the five-hour and weekly meters.
- "settings page listing connected accounts per provider with sign-in status": 8 results from Coda, Luma, Twingate, Amie, Sentry, Vercel, Wrangle and WorkOS. Three were usable. Most results were about signing in to the product itself, which is a different problem.
- "AI model provider settings with API key fields and a model list": 8 results from Braintrust, Retool, Adaline, Twenty, Langdock, Grok, Cofounder and AirOps. Five were usable.
- "Cursor settings models list": 8 results, all from Cursor. None shows a list of harnesses or accounts. The results were used for the settings file and the machines file.
- "Cursor dashboard usage table with tokens and cost per model" was run for the Usage file. It returned the Cursor "Spending" screen that is cited above.
- Named products that returned screens: Claude and Cursor. ChatGPT, Codex, Zed and Warp were not searched by name, because the budget of 14 searches was spent.
- No result shows a page that installs a command-line program from the browser. The install step has no reference.
