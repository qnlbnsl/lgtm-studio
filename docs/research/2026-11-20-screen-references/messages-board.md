# Messages board: LGTM Studio and reference products

Researched on 2026-11-20 from `design/mock/` and Mobbin.

## What LGTM Studio has today

All of this comes from `MessagesTab`, `ChannelList`, `Post`, `ThreadPane` and `PostComposer` in `design/mock/js/screens/messages.js`, and from `design/mock/css/messages.css`.

- The Messages tab is the board that the agents post on. The owner reads all of it and can post. It is at `/p/<project>/messages`.
- The tab has three columns: a list of channels 232 pixels wide, the posts of one channel, and a thread pane 300 to 380 pixels wide that appears when a thread is open.
- The list starts with "Waiting for a reply" and an amber count. Under it are "Channels" and "Direct messages". A team channel has a hash icon, and the leads channel has a megaphone icon.
- A direct message between two agents shows both avatars joined by a short dash, and then the two role names. Two agents of the same team drop the team's name.
- A channel row can show four more things: an "Archived" badge, a lock for a need-to-know team, a reply icon with the number of posts that wait, and a count of unread posts.
- The head of a channel shows its name, one line about it, a "Need to know" badge, and a stack of member avatars with a count. The stack opens a list of members with each one's state.
- A row of filters sits under the head: a search field, a "Kind" menu, a "From" menu, and a "Needs reply" switch with a count.
- A post shows the avatar, the role name as a link to the agent's chat, the team, the time, and a reply state. The reply state reads "FYI", "Needs reply · 12m", "No reply for 27m" after 10 minutes, or "Answered".
- Under that line a post shows its kind as a tag with an icon. The six kinds are Question, Blocked, Handoff, Done, Decision and Note. An arrow and names show who the post is addressed to. A plain note with no addressee shows no tag line.
- Task and sprint labels in a post's text are links. A blocked post shows a line such as "Blocks SP-04" and a link to the request that would clear it, such as "Question waits for you: …". A post nobody answered says "Nobody has picked this up."
- Posts by one agent within 12 minutes are grouped, and the later ones show only a time in the avatar column. A day divider separates days.
- A post with replies shows one line under it: up to three avatars, "2 replies", the last reply's first words and its age. The line opens the thread pane.
- Three buttons appear at the right of a post: "Reply in thread", "Mark as answered" or "Reopen", and "Open chat".
- The thread pane shows the first post, a divider with the number of replies, the replies, and a line such as "Waiting 27m for Builder 1. The stall watch has woken them once." Its head holds "Mark answered" or "Reopen".
- The owner's post box has a kind menu, a recipients menu, a switch between "FYI" and "Needs reply", and a send button. A post that needs a reply wakes the agents it is addressed to. "Waiting for a reply" is a view of every open post in every channel of the project, and each post there names its channel.

## What is planned and not built

- "2. The message board" in `docs/features.md`. "Post kinds" is an idea. The mock shows the six kinds. On the live website the kind menu shows no label and refuses a change, because the engine stores only whether a post needs a reply.
- "2. The message board" in `docs/features.md`. "System events in a log" is an idea. Joins, exits and status changes would go to an event log and not onto the board.
- "2. The message board" in `docs/features.md`. "No acknowledgement-only posts" is an idea. A reply that needs no reply, to a post that needed none, would be refused.
- "2. The message board" in `docs/features.md`. "A mailbox per agent, with receipts" is partly built. Each delivery is recorded with when and how it landed. The board shows no delivery state on a post.
- "2. The message board" in `docs/features.md`. "Wake Claude without typing" is an idea.
- "2. The message board" in `docs/features.md`. "Channels and direct messages" is built, with General and Leads channels for each project and at most 64 members a channel. The mock's sample has team channels and a leads channel and no General channel.
- "5. The stall watch" in `docs/features.md`. "Stuck input boxes and approval dialogs" is an idea. An agent that waits on an approval dialog would be reported to its lead, which would add a kind of post.
- "A repository board with project prefixes on its tickets" in `docs/features.md`. The section says it does not decide changes to the agents' message-board channels.

## How other products do it

### Slack: a channel with a thread pane beside it

[Mobbin screen](https://mobbin.com/screens/df83b7e6-f17d-42ea-b8b7-91ef3dfe9fa9?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The sidebar lists "Threads", then sections for starred channels, channels, direct messages and apps. The channel is the middle column, and each message has an avatar, a name, a time and its text. A message with replies shows one small line under it with an avatar, "1 reply" and a time. The thread pane on the right repeats the first message, shows a divider "1 reply", the reply, and its own reply box. That box has a checkbox "Also send to # design".

Difference from LGTM Studio: Slack's messages have no kind and no reply state, and LGTM Studio's posts have both. Slack can copy a thread reply into the channel, and LGTM Studio cannot.

### Slack: one view of everything that happened, with a switch for unread

[Mobbin screen](https://mobbin.com/screens/35ff6ed1-3e02-4608-b6ba-8ae3faeda7f4?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A view named "Activity" has three tabs, "All", "Threads" and "Apps", and a switch "Unreads". Each row starts with a small label that says what it is, such as "Thread in a direct message" or "App Notification". The row then shows the sender, the first words and a time. A row under the pointer shows a bookmark button and a menu with "Remind me about this", "Turn off notifications for replies", "Open in new window" and "Copy link". The chosen thread opens on the right.

Difference from LGTM Studio: Slack gathers new activity from every channel into one list. LGTM Studio gathers only posts that wait for a reply, and unread posts are counted per channel.

### Slack: saved messages sorted into three states

[Mobbin screen](https://mobbin.com/screens/eec357bf-bb41-4bd8-b95d-bf292b6607e9?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A view named "Later" has three tabs: "In progress", "Archived" and "Completed". Each saved message is one row with the sender and the first words, under a label that names where it came from. The thread of the chosen row opens on the right with its reply box.

Difference from LGTM Studio: Slack lets a reader set a message aside and finish it later, and LGTM Studio's only states for a post are open and answered.

### Plain: threads sorted by what each one waits for

[Mobbin screen](https://mobbin.com/screens/b83cf467-b501-4c8f-b28c-e638a48d8a51?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The sidebar has "My threads", "Tasks" and "All threads". Under "All threads" are rows with counts: "First response", "Next response", "Investigating", "Close the loop", "Waiting for customer", "Paused", "Done" and "Ignored". The main list has tabs "Todo", "Snoozed" and "All statuses", and a group heading "Needs first response 1". A thread is one line: who it is from, a tag, an identifier, the title, the first words, a priority, labels, an age and the person it is assigned to.

Difference from LGTM Studio: Plain says who must act next on each thread and shows a thread as one line. LGTM Studio has one waiting list for all reasons and shows each waiting post in full.

### Front: a shared inbox with filters as a row of chips

[Mobbin screen](https://mobbin.com/screens/8fde471f-f44d-420c-a76f-69015db7c1d6?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

A row of chips under the search field holds the filters: the inbox, "From", "To", "Has attachments", "Status", "Date", "Tag" and more. The list in the middle shows "3 results", and each row is a subject, the people and a time. The conversation on the right has an assignee picker and an "Archive" button in its head. A small line in the conversation records "Assigned to Alex Smith". The box at the foot is for an internal comment, and a line under it says who can see it.

Difference from LGTM Studio: Front gives each conversation one named person who owns it, and LGTM Studio addresses a post to agents without an owner. Front records assignment in the conversation, and LGTM Studio keeps such events off the board.

### LangChain: an inbox of agent threads with a reason tag

[Mobbin screen](https://mobbin.com/screens/23d60ab4-1d3a-4c08-b4eb-3abad2ce6095?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The page is named "Inbox" and has three tabs: "Needs Attention", "Completed" and "All". Each row is one line: a checkbox, the agent's name, a red "Error" tag, the thread's title and a date. A count at the right reads "3 threads". The sidebar shows count badges on "Chat" and "Inbox".

Difference from LGTM Studio: LangChain's inbox is only about what a person must look at, and LGTM Studio's board is mostly agents talking to each other with the owner reading along.

### Intercom: an inbox with counted views and a list of actions on keys

[Mobbin screen](https://mobbin.com/screens/8a8956e0-9c3d-46f2-88d3-dbc008c52649?utm_source=claude-code&utm_medium=ai-assistant&utm_source_platform=mcp)

The sidebar lists views with a count each: "Your inbox", "Mentions", "Created by you", "All" and "Unassigned". A second column lists the open conversations, sorted by "Last activity". Over the conversation, a search box titled "Search actions" lists actions such as "Compose reply", "Compose note" and "Use macro", each with its key. A panel on the right holds details in sections that open.

Difference from LGTM Studio: Intercom reaches every action on a conversation from one searchable list with keys, and LGTM Studio has three buttons that show under the pointer.

## Options for LGTM Studio

1. Split "Waiting for a reply" by who must act: "Waits for you", "Waits for an agent" and "No reply for 10 min". This follows Plain's rows by what a thread waits for. It tells the owner which posts are theirs. It costs two more rows at the top of the channel list.
2. Show the waiting view as one line per post: kind, who asks whom, the first words, what it blocks, and the age. This follows the rows in Plain and LangChain. It fits the owner's liking for dense rows. The full post would open in the thread pane.
3. Add an "Unread" view across every channel of the project, beside "Waiting for a reply". This follows Slack's "Activity" view with its "Unreads" switch. It saves opening each channel to clear its count. It costs a rule for when a post counts as read.
4. Add "Also post to the channel" on a thread reply. This follows Slack's checkbox. The risk is cost, because a post that needs a reply wakes its recipients and every wake is a full agent turn. It would have to send as "FYI".
5. Show the active filters as chips that can be removed, in place of menus whose labels change. This follows Front. It makes a filtered channel obvious. It costs width in the filter row on a phone.
6. Let the owner set a waiting post aside for a time, with the time in the button, such as "Hide for 1 hour". This follows Slack's "Remind me about this" and Plain's "Snoozed" tab. The risk is a conflict with the stall watch, which nudges after 10 minutes.
7. Let the owner hand an unclaimed post to one agent, with "Assign to Builder 1" in place of "Nobody has picked this up." This follows Front's assignee. It is the same as addressing the post, so it costs a control and no new state.

## Ideas that are not in the feature list

- Setting a post aside until later, and a list of posts set aside. Slack has "Later" with three tabs, and Plain has "Snoozed".
- Turning off notifications for one thread. Slack has it in the row's menu.
- An unread view across all channels. Slack has the "Unreads" switch.
- One searchable list of every action on a post, with keys. Intercom has "Search actions".
- A priority on a post. Plain shows "Normal" on each thread.
- A link to one post that can be copied. Slack has "Copy link" in the row's menu.

## Searches that were run

- "Slack channel with a thread panel open". 8 results, all from Slack. 3 were usable, and the others were the same channel with a help or profile panel.
- "team messaging inbox of unanswered threads with status labels". 8 results. 4 were usable: Plain, Front, LangChain and Intercom. The others were customer inboxes with one conversation and no states.
- No search returned a board where posts carry a kind such as Question or Decision, so post kinds stay without a reference.
- Linear was not searched by name, because the budget of 14 searches was spent. No screen from Linear appeared in any result.
