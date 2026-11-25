# Notifications: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

The screen comes from `design/mock/js/screens/service.js`, in the functions `Notifications`, `NtfyForm`, `EmailForm` and `QuietForm`. The delivery rules come from `docs/notifications.md`.

- The page head shows the title "Notifications" and a status dot with one line. The line reads "Sending to" with the topic's name, "Sending by email", or "Not set up". The sample data starts as "Not set up".
- The page has two columns. The left column holds four settings sections. The right column holds "Recent notifications".
- A notification reaches a phone in two ways. The first is ntfy: the owner subscribes to a topic in the ntfy app on the phone, and LGTM Studio posts to that topic. The second is email through the owner's own mail server. There is no list of devices and no push from LGTM Studio's own app.
- The "ntfy" section is a form with a server address, a topic, and an optional access token with a show and hide button. Its buttons are "Turn on" or "Save changes", "Send a test", "Clear token", and "Turn off". A switch for ntfy delivery appears after the form is saved.
- The "Email" section is a form with seven fields: host, port, connection security, user name, password or app key, sender address, and recipient address. The fields configure the Simple Mail Transfer Protocol (SMTP) server. Its buttons are "Save email", "Send a test", "Clear password", and "Remove email".
- A stored token or password is never shown. The field's placeholder says that one is set.
- "Notify me when" lists five kinds, each with a switch and a hint sentence: a question, an approval, a merge that is ready, an agent that failed to start, and a finished sprint. The live website can list nine more kinds, such as "An agent has stalled".
- Every enabled delivery kind receives every enabled alert kind. `docs/notifications.md` says so, and the page has no way to send one kind only by email.
- "Quiet hours" is a form with a switch, a start time, an end time, a time zone typed as text, and a second switch, "Let urgent alerts through". It has its own "Save quiet hours" button.
- "Recent notifications" is a list grouped under day labels. A row shows an unread mark, a kind tag, a title that opens the request or the thread, the agent and the project, a delivery state, and a time.
- The delivery state is one of five: Sent, Not sent, Failed, Held until quiet hours end, and Not sent · turned off. A failed row shows a "Retry" link. Hovering the state lists the result for each delivery kind.
- The list shows twelve rows, and then a "Show all" button. "Mark all read" sits in the section head. On the live website the same button reads "Dismiss all".
- A callout offers phone alerts while nothing is set up, and it can be dismissed.
- The sidebar shows a phone icon for this page, with a red count of failed notifications, as `design/DESIGN.md` records under "Sidebar rows".
- On a phone, a row of the recent list puts the name on its first line and the status first on its second line, as `design/DESIGN.md` records under "Lists and tables".
- A phone alert links to a thread through the "Public address" on Service settings, under Remote access.

## What is planned and not built

- `docs/features.md`, "Notifications with more than one kind of delivery": delivery kinds are added "starting with email and ntfy, and others may follow". No other kind is named. Email and ntfy are built.
- `docs/notifications.md`, "Client application programming interface actions": a new carrier registers its transport, validation and secret binding in one engine file. The engine is prepared for more delivery kinds.
- `design/DESIGN.md`, "Notifications": the recent list is recorded as a table with four columns, Notification, Time, About, and Delivery. The code in `service.js` renders a list of rows with no column headers.
- `design/DESIGN.md`, "Service settings": "Fields save on change. There are no Save buttons." The ntfy, Email and Quiet hours forms on this page each have a save button.
- `design/DESIGN.md`, "The first run, built from a private app's onboarding rules": notifications are offered on the first waiting request and not during setup. This is built as the callout.
- The owner has said that a screen gets no helper or instructional sentences. This rule was given to the researcher and was not found in `docs/features.md` or `design/DESIGN.md`. The "Notify me when" rows, the ntfy topic field and the Quiet hours rows each show a hint sentence.
- Nothing was found about a list of devices, a per-project choice, or a summary email.

## How other products do it

### Linear: delivery kinds as a list of four rows

[Mobbin screen](https://mobbin.com/screens/915c51c8-1e36-47d7-a8e9-a2c33bcc98f2?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The page starts with "Notification channels", a list of four rows: Desktop, Mobile, Email, and Slack. Each row holds an icon, the name, a dot with one line of state, and a chevron at the right. The state lines read "Disabled" or "Enabled for all notifications". A row opens that kind's own page. A sentence above the list says that notifications always go to the inbox as well.

Difference from LGTM Studio: Linear shows every delivery kind as one line with its state, and keeps the form on a second page. LGTM Studio shows every form open in one column.

### Linear: one delivery kind's page, with its own list of alert kinds

[Mobbin screen](https://mobbin.com/screens/27f3ec52-c8b6-4cd0-836e-a954a891c131?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A back link at the top reads "Notifications". The first card holds a master switch with the recipient address under it, and a "Notification format" menu set to "Digest". A second card holds two switches: one delays low-priority emails until the next work day, and one sends at once when an issue is urgent. A third card, "General notifications", lists the alert kinds, each with one line of description and a switch. When the master switch is off, every row below it is greyed, as a second screen shows: [Mobbin screen](https://mobbin.com/screens/fbb07294-5214-4769-97e0-f2bd9f8cb36a?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp).

Difference from LGTM Studio: Linear lets each delivery kind have its own set of alert kinds. LGTM Studio has one set that every delivery kind receives.

### GitHub: a menu of delivery kinds on each alert kind

[Mobbin screen](https://mobbin.com/screens/e8dcbfb1-b94a-4e42-8f6e-ee449693328b?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The page first sets a default email address, with a "Custom routing" button beside it. Under "Subscriptions", each alert kind is a block with a title, a sentence, and one small menu button. The button's label states the current choice, such as "Notify me: on GitHub, Email". Lower on the page, the same button reads "Notify me: on GitHub, Email. (Failed workflows only)", as a second screen shows: [Mobbin screen](https://mobbin.com/screens/6128d1d8-eab1-46cc-abb5-0ef77897ddb5?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp).

Difference from LGTM Studio: GitHub turns the layout around. The alert kind is the row, and the delivery kinds are the value. LGTM Studio gives each alert kind a single on or off switch.

### GitHub: a per-repository choice in a menu of four modes

[Mobbin screen](https://mobbin.com/screens/afad95d1-dfe9-4158-b5bb-43bc840185b2?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A button on the repository page opens a menu titled "Notifications". The menu lists four modes, each with a bold name and one or two lines under it: "Participating and @mentions", "All Activity", "Ignore", and "Custom". A tick marks the current mode. A last line links to the phone apps for push notifications.

Difference from LGTM Studio: GitHub lets one repository be louder or quieter than the rest. LGTM Studio has one setting for every project.

### GitHub: one saved address as one row

[Mobbin screen](https://mobbin.com/screens/c2bd9911-f477-4277-915f-22138339716e?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

After setup, the email notification page shows a single bordered row. The row holds a tick with "Active", the address, and two buttons, "Edit" and "Delete". The form with its fields appears only after "Edit", as a second screen shows: [Mobbin screen](https://mobbin.com/screens/ba0f1140-6d83-4b70-b65f-f91445608373?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp).

Difference from LGTM Studio: GitHub folds a configured delivery kind down to one line. LGTM Studio keeps the seven email fields on screen after they are saved.

### Slack: the hours in which notifications are allowed, on one line

[Mobbin screen](https://mobbin.com/screens/1b38a012-ea33-44dd-a9b4-4510605d6d74?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

"Notification schedule" is the first block of the notification preferences. One line holds three menus: "Every day", a start time, and an end time. The line states the hours in which notifications arrive, not the hours in which they are held. A "Show an Example" button further down sends a sample notification.

Difference from LGTM Studio: Slack sets the schedule in one line of three menus, with a choice of days. LGTM Studio uses five rows and a save button, and it has no choice of days.

### Slack: pause notifications from the account menu

[Mobbin screen](https://mobbin.com/screens/499aa83f-17e0-4b0f-bde1-60fa3777cb6d?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The account menu has a "Pause notifications" row that reads "On". Its submenu shows a banner, "Do Not Disturb", with "Notifications paused until 9:14 PM". Three actions follow: "Resume notifications", "Adjust time", and "Set a notification schedule".

Difference from LGTM Studio: Slack offers a pause that starts now and ends at a stated time. LGTM Studio has only the repeating quiet hours.

### Vapi: what raises an alert, as a table apart from where it goes

[Mobbin screen](https://mobbin.com/screens/1965682c-0c4e-4146-a0fb-864ad2eabf2d?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

"Monitors" is a table with seven columns: Status, Summary, Alert, Category, Assistants, Last Triggered, and Last Saved. The Summary cell holds a title and a line that says when the monitor fires. The Alert cell shows a dot and "Disabled". The sidebar lists "Notifiers" as a separate page under the same group.

Difference from LGTM Studio: Vapi shows when each alert kind last fired. LGTM Studio shows a switch for each kind, and the history sits in a separate list.

### Cursor: one switch and routing rules

[Mobbin screen](https://mobbin.com/screens/db14ab9e-0507-4a45-9483-163dec7f6cdd?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

"Notifications" is one section of the Cloud Agents settings page. It holds one row, "Slack Notifications", with a switch and one line that says an agent's finished task is reported in Slack. A block under it, "Repository routing", has an "Add Rule" button and an empty state. An "Unlink Slack" button sits at the top of the defaults section.

Difference from LGTM Studio: Cursor notifies about one event through one service. LGTM Studio has two delivery kinds, up to fourteen alert kinds, quiet hours and a delivery history.

## Options for LGTM Studio

1. Replace the two open forms with a list of delivery kinds, as Linear has. Each row would show the kind and one line of state, such as "ntfy", the topic's name and "On", or "Email" and "Not set up". A row would open its form under it, and each row would open and close on its own. This frees most of the left column. It costs one click to reach a field.
2. Fold a configured delivery kind to one line with "Edit" and "Remove", as GitHub does with a saved address. The form appears only when it is not set up or when "Edit" is pressed. This is a smaller change than option 1 and keeps the page's two columns.
3. Give each alert kind a menu of delivery kinds, as GitHub does with "Notify me: on GitHub, Email". A question could go to ntfy only, and a finished sprint to email only. The engine sends every kind to every carrier today, as `docs/notifications.md` records, so this needs engine work.
4. Give each delivery kind its own list of alert kinds on its own page, as Linear does. It reaches the same result as option 3 from the other side. It repeats the list of kinds once per delivery kind, which grows with every new carrier.
5. Rewrite quiet hours as one line of menus, as Slack does. The line would hold the days, the start and the end, and the time zone would become a menu. A choice of days needs a new field in the engine, which stores a start, an end and a time zone today.
6. Add "Pause notifications" with an end time, as Slack has in its account menu. It could sit in the bell's menu or in the sidebar, so it is reachable from a phone without opening this page. It is a new feature and needs engine support.
7. Add a per-project choice in each project's Settings tab, as GitHub has per repository. The choices could be the service's setting, only requests, and nothing. The risk is that a muted project hides an approval that blocks its agents.
8. Add a "Last sent" value to each row of "Notify me when", as Vapi shows "Last Triggered" on each monitor. The owner would see which kinds are noisy. It needs no engine change if the recent list already holds the data.

## Ideas that are not in the feature list

- A summary email that groups low-priority notifications, sent once a day or once a week (Linear, GitHub).
- A pause that starts now and ends at a chosen time (Slack).
- A choice of weekdays for quiet hours (Slack).
- A notification setting for one project that differs from the rest (GitHub).
- A different recipient address for one group of projects, which GitHub calls "Custom routing" (GitHub).
- Slack as a delivery kind (Linear, Cursor). `docs/features.md` says only that other kinds "may follow".
- Routing rules that send alerts about one repository to one place (Cursor).
- A list of the browsers and phones that are signed in, with the current one marked. This was seen on a Luma screen that the "connected accounts" search returned: [Mobbin screen](https://mobbin.com/screens/8634f778-63a3-472e-8d0a-357ea5e53e54?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp).

## Searches that were run

- "Linear notification settings": 8 results, all from Linear, and 3 were usable. The other five were other settings pages of Linear.
- "GitHub notification settings": 8 results, all from GitHub, and 5 were usable.
- "Slack notification schedule settings": 8 results, all from Slack. Three were usable. Three of the eight were the "Schedule message" dialog, which is a different feature.
- "Cursor settings models list" was run for the settings file. It returned the Cursor notification section that is cited above.
- Named products that returned screens: Linear, GitHub, Slack and Cursor. Sentry and Datadog were not searched by name for this screen, because the budget of 14 searches was spent.
- No result shows a history of sent notifications with a delivery state and a retry. The "Recent notifications" list has no reference.
- No result shows a form for a mail server or for ntfy.
