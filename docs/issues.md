# LGTM Studio: problems found

This file lists the problems that are still open.
A problem found while working is added here in the same change that finds it, even if it is not fixed.
Closed entries are kept, word for word, in [issues-closed.md](issues-closed.md).
No entry is ever deleted.

## How this file is kept

A new entry takes the next number after the highest number in this file and in `issues-closed.md` together.
The highest number in use on 2026-11-24 was 1630.
An entry stays in this file while it is open, while it waits for the owner, or while its fix is not verified.
An entry is moved to `issues-closed.md` in a cleanup pass, after its "State:" line says where it was fixed.
The numbered entries are in number order below the tables.

Each entry uses this shape:

```
<two number signs> <number>. <what is wrong, in one line>

Found: <date>, while <doing what>.
State: open | fixed in <commit or pull request>.

What happens: <the observed behaviour>.
What should happen: <the expected behaviour>.
Why it matters: <the cost, such as wasted turns, lost messages, or a security risk>.
```

## The triage of 2026-11-23

The owner asked for a triage of every issue and a cleanup of this file.
On that day the file held 1439 numbered entries.
A script read each entry's state line: 943 were clearly closed, and 496 were not.
Eight reviewers, each a Claude model, read those 496 entries and checked them against the code in proportion to their weight.
Their verdicts were: 254 open, 45 waiting for the owner, 77 fixed, 65 records that are not faults, 40 duplicates and 15 obsolete.
Every triaged entry carries a line that starts with "Triage (2026-11-23):".
A verdict can be wrong; correct the line when you find that it is.
The 43 entries that a reviewer called fixed without high confidence stay in this file until someone checks them.
This file therefore holds 342 entries, and `issues-closed.md` holds 1097.

The priorities mean this:

- P0: it breaks or silently corrupts daily use, or it breaks a binding rule of `SECURITY.md`.
- P1: a real fault that a user would hit, with a workaround.
- P2: minor, rare or cosmetic.
- P3: kept as a record and unlikely to be scheduled.

"Confidence" is the reviewer's own; "low" and "medium" mean the verdict rests mostly on the entry's text.

## P0: breaks daily use or a binding security rule

| Issue | What is wrong | Area | Confidence |
| --- | --- | --- | --- |
| 857 | An image file, including a text-based SVG, is sent to the page without the registered-secret check. | Security | high |
| 1575 | After one failed save the service silently refused every later save for two days while the website kept answering normally. | Engine | high |

## P1: real faults with a workaround

| Issue | What is wrong | Area | Confidence |
| --- | --- | --- | --- |
| 241 | A real agent's shell got owner access after removing its identity variables, where a refusal was required. | Security | medium |
| 251 | A disconnect right after a command is saved can skip its process work, leaving the record and the real process disagreeing. | Engine | medium |
| 252 | A lead that crashes and is resumed never receives its child's finished-work notice, so delegated work stalls. | Engine | medium |
| 253 | After a restart, one agent that stops reading input can block every other recovered agent from continuing. | Engine | medium |
| 255 | One unreadable stored receipt makes the service refuse every later command until it is restarted. | Engine | high |
| 257 | An agent asked to answer a permission request is not told the choices, so a team lead cannot answer it. | Board and roles | high |
| 265 | If an agent's adapter stops reading a large prompt, a stop command waits forever behind it. | Engine | medium |
| 267 | Cleanup from a cancelled agent start can run late and break a later successful stop or fresh resume of that agent. | Engine | high |
| 519 | Twenty confirmed review findings were still present, including a dropped permission request that leaves an agent waiting; none was rechecked since 2026-11-07. | Engine | medium |
| 939 | The browser tab crashed three times during a real project, probably from the chat screen redrawing too much; never reproduced. | Website | low |
| 1235 | After a failed first turn and a service restart, an agent's retried first message can be sent without its role instructions. | Harnesses and agent running | medium |
| 1461 | After a service restart one reloadable agent stayed interrupted and was neither retried nor restarted, with no failure shown. | Engine | low |
| 1464 | Define question cards show a whole proposal at once and bundle several decisions into one answer, so the owner cannot answer each separately. | Website | medium |
| 1472 | Review and file listing limits are nearly reached by a large repository, and listings past the limit drop entries without saying so. | Git | high |
| 1542 | The owner's first message to an agent is shown with the agent's whole role instructions in front of it. | Engine | medium |
| 1543 | Automatic reminders from the stall watch are stored and shown as ordinary posts, crowding the agents' messages. | Board and roles | medium |
| 1550 | The live Git tab refuses to abort a merge or update from a parent, although the feature list says both are supported. | Website | high |
| 1576 | A service started in the background throws away its log, so errors such as a failed save leave no trace. | Engine | high |

## Waiting for the owner's decision

| Issue | Priority | What is wrong | Area |
| --- | --- | --- | --- |
| 946 | P1 | An agent in a project reads and writes the owner's ordinary Claude Code memory and runs the owner's hooks. | Harnesses and agent running |
| 65 | P2 | Nothing announces that an owner alert has ended, so a client must work it out from the request's status. | Client API and command line |
| 113 | P2 | Questions asked by a crashed agent never close; the owner must decide on a withdrawn event and the blocked-task status. | Engine |
| 134 | P2 | When one agent hits a usage limit, idle agents on the same account still get work and each wastes a turn. | Engine |
| 152 | P2 | A paused agent that cannot reload after a restart can only be stopped and started anew, leaving its queued work stuck. | Engine |
| 194 | P2 | After a service kill, a prompt sent but not yet accepted is delivered to the agent twice; the owner must confirm this is acceptable. | Engine |
| 261 | P2 | Changing an account silently drops fields left out, and an account can be removed while agents still use it. | Engine |
| 326 | P2 | A board post cannot be deleted, so a secret posted by mistake stays in the log and receipts. | Board and roles |
| 347 | P2 | The recorded harness fixtures hold no tool call or permission request, so permission matching cannot be tested on real traffic. | Tests |
| 469 | P2 | The roles live check has never observed a real first stall; the owner must run it with a real account. | Tests |
| 470 | P2 | The repaired roles live observer still needs the owner's real-account rerun to close the proof gap. | Tests |
| 686 | P2 | The Mocks tab is hidden for a project with no mocks, and the owner has not said whether it should always show. | Website |
| 687 | P2 | The owner called Subscriptions and Service settings unfinished but has not yet said what is missing. | Website |
| 692 | P2 | The owner cannot manually open another channel named General or Leads because channel names must be unique per manager. | Board and roles |
| 728 | P2 | Workflow changes leave no event or receipt, and the owner must decide how they appear in the audit history. | Engine |
| 729 | P2 | Two tabs editing the same workflow silently overwrite each other; the last save wins. | Client API and command line |
| 750 | P2 | Any agent in a project can read other teams' builder worktrees through the Git read requests; the owner must rule on it. | Security |
| 752 | P2 | Merging a builder into its team holds the builder's lock, not the team's, so another team command could collide. | Git |
| 794 | P2 | The automatic Gemini installation has never been tried for real; the owner must try it before a release. | Harnesses and agent running |
| 1133 | P2 | The reviewer default settings are saved but nothing uses them; the owner must decide whether to hide the controls. | Board and roles |
| 1150 | P2 | The stop message promises Resume continues from the last turn, but a fresh resume loses memory and may repeat steps. | Website |
| 1151 | P2 | Restart confirmations still say running agents are stopped, although a restart now keeps them for recovery. | Client API and command line |
| 1171 | P2 | The owner must decide whether role instruction files inside a repository may be trusted to instruct agents. | Security |
| 1174 | P2 | Two documents hold private project references and owner quotations that need the owner's decision before publication. | Documents |
| 1264 | P2 | Hiding the first character of a possible secret delays some sentence endings and following tool calls; the owner must decide whether to keep this. | Security |
| 1546 | P2 | Two screens and the sample data tie each account to one machine, although accounts are shared by all machines. | Website |
| 1547 | P2 | The design document and the screens disagree in thirteen places, and each needs a decision on which side is right. | Website |
| 1548 | P2 | Many screens still carry explaining sentences, which the owner ruled out, and removing them from existing screens needs his approval. | Website |
| 1555 | P2 | Git and Files content is very narrow beside the conversation and their lists, and the layout needs the owner's choice. | Website |
| 1569 | P2 | A blind review of the bars above a project found fifteen faults; several were settled, and the rest need the owner's yes. | Website |
| 1572 | P2 | The owner's review of every screen left eleven questions awaiting his answers and eight blind review findings open in the new port. | Website |
| 38 | P3 | A normal Cursor greeting recording is still missing; the owner must record it once the account has usage. | Tests |
| 118 | P3 | When an account pauses, the provider's own error text is dropped, so odd limit messages are hard to diagnose. | Engine |
| 195 | P3 | The acceptance suite does not itself check several specification clauses; the owner decides whether lower-level tests are enough. | Tests |
| 336 | P3 | A board duty the engine keeps refusing is retried after each later batch, adding a refusal event each time. | Board and roles |
| 353 | P3 | Every file under a harness folder counts as settings, so edits in worktrees under such folders all wait for the owner. | Security |
| 670 | P3 | An answered question card shows the accepted answer inside the button row; the owner wants it on its own line. | Website |
| 730 | P3 | Renaming a workflow changes only its display name, not its identifier or filename; the owner must decide if that is wanted. | Client API and command line |
| 731 | P3 | The visual workflow editor shows a block's inner steps but cannot change them; only the API and terminal can. | Website |
| 775 | P3 | Automatic harness installs ignore proxy settings, so they can fail behind a required proxy. | Harnesses and agent running |
| 1161 | P3 | The thread's "requests below" indicator leaves out a held permission card that the owner can answer. | Website |
| 1162 | P3 | A collapsed team's request count includes a held permission that its expanded agent rows do not show. | Website |
| 1549 | P3 | Home lets only one request be open at a time, against the owner's newer rule that each item opens on its own. | Website |
| 1561 | P3 | Of nineteen sprint page faults, three remain: a tall phone header, faint button edges and a request not naming its blocked task. | Website |
| 1566 | P3 | In dark mode the attention colour and the link colour look alike, and changing the approved palette needs the owner. | Website |

## P2: minor, rare or cosmetic

### Engine (25)

| Issue | What is wrong | Confidence |
| --- | --- | --- |
| 1 | The event log keeps every event forever, so the database grows without limit and startup slows as it rebuilds. | high |
| 46 | If updating the in-memory views fails after a commit, the views silently fall behind the stored log until restart. | medium |
| 47 | Each agent's live thread is copied in full on every event and held in memory forever, so busy agents get slower. | high |
| 59 | One unreadable stored receipt or failed read during a command blocks every later command until the service restarts. | medium |
| 150 | An owner prompt carries no identifier, so after a restart it may be sent again and the agent repeats work. | high |
| 184 | The agent tool server writes no log at all, so a failed tool call leaves no record to debug. | high |
| 231 | The last output an agent wrote before it exited or was stopped can be missing from its thread, with no warning. | medium |
| 246 | Output from tools other than the terminal is stored whole, with no size limit. | high |
| 254 | A stale account return can mark an agent working after a newer sign-out, leaving it stranded when the account is cleared. | low |
| 256 | A failed save followed by a failed undo can escape the rule that the service refuses commands after a storage fault. | low |
| 259 | If saving a permission request fails, the agent stays blocked on a request that nobody can see or answer. | high |
| 268 | A cancelled start can wait without limit on the event writer, and cannot record its end after a real storage failure. | medium |
| 280 | A secret that matches a short internal word can make an agent's terminal output vanish from the thread. | medium |
| 332 | After a restart, an interrupted agent whose session cannot reload stays interrupted until the owner asks for a fresh session. | medium |
| 352 | All agents' runner events pass through one sequential worker, so one slow agent can delay the others. | medium |
| 624 | The engine reports no processor, memory, operating system, version or agent slot limit for its machine. | medium |
| 657 | An agent is told the owner's answer is data from another agent. | high |
| 723 | Workflows can be edited and saved, but a project cannot choose one and none is executed. | medium |
| 737 | The engine cannot generate commit or pull request text because no bounded model task with a chosen account and model exists. | high |
| 790 | The engine hashes the whole installed harness tree while blocking other work, on every status read and agent launch. | high |
| 818 | A stop signal sent just after a restart request can still let the service start again. | medium |
| 852 | Saving a file from the Files tab works only on Linux and is refused on macOS. | high |
| 1076 | The engine records no times for the Verify and Keep running phases or for skipped phases, so the Workflow tab shows blanks. | medium |
| 1083 | The launcher can report "started" and the service can then refuse to finish starting, with its output discarded. | high |
| 1286 | If a login store becomes unreadable, an agent's permission request is refused with no event, and the turn just fails. | medium |

### Client API and command line (16)

| Issue | What is wrong | Confidence |
| --- | --- | --- |
| 76 | Notices written by the engine name the owner as sender, so a client cannot tell them from the owner's own prompts. | high |
| 212 | A permission option that saves a rule does not say where it saves it, so the saved-permission security check cannot pass. | medium |
| 300 | The reason on an owner-needed alert is free text, so a wording change could silently break client alert handling. | high |
| 673 | The role settings read returns only stored overrides, so the website cannot show the limits actually in effect. | high |
| 791 | A harness installation can run for ten minutes and the owner cannot cancel it. | high |
| 849 | Text files over 512 kilobytes are not shown, though the feature list promises one megabyte. | high |
| 850 | A binary or large file cannot be downloaded because no request returns a file's bytes. | high |
| 878 | A project lead's subscription cannot be changed after it starts; only model and effort can. | high |
| 879 | A project cannot set the permission level for every agent in it. | medium |
| 880 | A project has no write-tests-first setting in the engine. | high |
| 881 | A project has no agent-review setting in the engine. | high |
| 883 | A project cannot set or approve which harness, model and effort each kind of work uses. | high |
| 888 | When a lead's model or effort change is refused, the owner sees only 'command refused' and not the reason. | high |
| 899 | The Files screen detects a changed file by matching the engine's English refusal sentence, which breaks if the wording changes. | high |
| 1172 | There are no client commands to list, read or save individual role instruction files, so they can only be edited on disk. | medium |
| 1537 | A repository cannot be removed from the registry, so it keeps constraining the workspace folder after its last project is deleted. | high |

### Git (7)

| Issue | What is wrong | Confidence |
| --- | --- | --- |
| 354 | One git clean command at the repository root would delete every worktree, including unmerged work. | medium |
| 738 | The owner cannot manually edit a hosted pull request's title or description from the page. | medium |
| 1016 | The engine cannot safely remove a worktree, so deleting a project leaves all its folders and branches behind. | high |
| 1135 | The engine stores the approval decision the page sends without checking that the current evidence supports it. | medium |
| 1253 | Stopped workers and deleted teams leave their checkouts and branches behind, and there is no safe command to remove them. | high |
| 1254 | A service crash between creating a checkout and registering it leaves an unregistered checkout that is never reconciled. | high |
| 1514 | When creating a worker fails, its worktree is left behind if it contains a submodule link, because Git refuses to remove it. | medium |

### Security (14)

| Issue | What is wrong | Confidence |
| --- | --- | --- |
| 91 | The settings cannot express the Claude adapter's only way to deny tools, so it is recorded as having none. | high |
| 104 | Codex, Gemini and Cursor have no verified launch rule stopping them from reading the state folder or writing harness settings. | high |
| 240 | A Claude Code agent's token appears in a process command line, readable by any process of the same user. | high |
| 269 | An account whose login folder equals a stored secret value fails to save and can block later account commands. | medium |
| 312 | A secret typed under an ordinary variable name in an account is written to the log and shown to clients. | medium |
| 321 | Events an agent reads through the subscribe command are not marked as data, so they could be taken as instructions. | medium |
| 323 | An agent in a Herdr pane could read the owner's web password from Herdr's environment. | medium |
| 329 | The permission ceiling is checked when an agent starts but not when it is prompted, unlike the design says. | medium |
| 783 | A hostile program running as the same user can swap the installed harness despite the installation check. | high |
| 817 | Restart runs whatever program is in the writable checkout, so an agent as the same user could change it first. | high |
| 834 | A registered secret inside a compressed design file is not caught by the save check. | medium |
| 1021 | A short registered secret such as "one" can alter a board post's fixed reply mode during redaction, so the post is refused. | medium |
| 1195 | An agent with shell access can write role override files that become the next agent's engine instructions. | medium |
| 1540 | Committing a single file no longer refuses a copied secret file that the repository's ignore rules expose, so it can reach agent reviews. | low |

### Board and roles (8)

| Issue | What is wrong | Confidence |
| --- | --- | --- |
| 331 | The service keeps every board post's text in memory, so memory grows with history. | medium |
| 346 | During a model-capacity outage the board can still mark a wake undeliverable after two returns. | high |
| 363 | An agent given the name 'owner' would share its board read records with the human owner. | high |
| 520 | The owner's ruling on replacement advice for a stalled lead is not built: no mark on the lead and no workers-done notice. | medium |
| 801 | The saved reviewer count and different-vendor rule do nothing, because no engine code picks reviewers. | medium |
| 1152 | A lead that already received a permission request is never told when the owner answers it first. | medium |
| 1173 | The engine does not enforce that a done report names a real tested branch head, or that merges exclude scratch files. | medium |
| 1364 | A task cannot exist without a team, so a lead must create a placeholder team before its first tasks. | high |

### Harnesses and agent running (14)

| Issue | What is wrong | Confidence |
| --- | --- | --- |
| 193 | Only four built-in harness names can be recognised as signed out or out of usage; any other harness is never paused. | high |
| 237 | Approval prompts from other tool servers are cancelled inside the adapter instead of reaching LGTM Studio's permission flow. | high |
| 272 | In ask mode, Codex can simply refuse to write a file instead of asking the owner for permission. | high |
| 273 | Terminal output can show a silent gap, grow with mixed status reports, and lag until the next update. | medium |
| 274 | Terminal output that contains a launch secret disappears entirely instead of appearing as a redacted line. | low |
| 516 | Grok cannot be run through Cursor's protocol mode without its Fast setting, which the owner forbids. | medium |
| 792 | Automatic harness installation is refused when the home folder is reached through a symbolic link. | high |
| 865 | A second Cursor account cannot be added because no known way isolates Cursor's login credentials. | medium |
| 906 | The owner cannot watch or take over an agent's terminal, because the engine has no way to attach to it. | high |
| 938 | A Cursor account cannot be given its own separate sign-in folder, so only one Cursor sign-in is possible. | high |
| 942 | The project lead's first reply feels slow, the delay has never been measured, and the page shows only dots while waiting. | medium |
| 1141 | A paused agent cannot be given a fresh session, because the engine refuses resume in the paused state. | medium |
| 1239 | A small Claude lead given long role instructions sometimes does not run the exact commands the owner asks for. | medium |
| 1457 | A review left five small faults in custom harness mode defaults, a session-record document sentence, one test assumption and a lost startup warning. | medium |

### Website (16)

| Issue | What is wrong | Confidence |
| --- | --- | --- |
| 517 | In a long project list the sidebar does not scroll to the selected agent, so the selection can be out of sight. | medium |
| 615 | The Git tab shows one centred empty message instead of keeping its columns when a project has no worktree. | high |
| 842 | Picking a checkout from the global menu can show old file rows because it skips the fresh Git read. | high |
| 890 | The Project settings screen has no control for the project's shell, though the engine supports it. | high |
| 891 | A goal change from another client silently replaces the owner's unsaved goal draft. | medium |
| 937 | The Add account dialog is confusing in five ways, and account rows show raw model identifiers and no usage figures. | high |
| 940 | A new project is still named from the first words of its goal instead of a name the owner chooses. | medium |
| 1034 | The chat shows a link to a local file as raw brackets and a full path instead of its label. | medium |
| 1146 | An agent that belongs to no project has no chat the owner can reach on the website. | medium |
| 1154 | A merge or sprint alert that names an agent opens that agent's chat instead of the merge or sprint. | medium |
| 1375 | The roster's 'Doing now' can show work from an earlier turn or the wrong task, and one journey check is fragile. | high |
| 1544 | In the draft design, open Define questions are counted in the sidebar but not on Home or the bell, so counts disagree. | low |
| 1552 | Removing the Chat tab from the design left no place for the agent details panel or the permission mode menu. | medium |
| 1560 | With the list hidden in Files or Git, there is no way to open another file without showing the list again. | medium |
| 1567 | The approved Questions view has many open points, is not on the live website, and agents are not told to write its sections. | medium |
| 1571 | Porting the screens found fourteen code faults, mostly in live website paths, and the website build cannot yet use the port. | medium |

### Tests (11)

| Issue | What is wrong | Confidence |
| --- | --- | --- |
| 264 | A test for a process group that survives a kill may time out before the sweep it waits for can finish. | medium |
| 524 | The live check's Codex tool step fails when the model simply does not call the tool, with no second prompt. | medium |
| 727 | The service-settings journey sometimes never sees the saved state after changing the heartbeat in a full run. | low |
| 732 | The Git browser journey sometimes cannot click Approve without evidence in a full run, and passes alone. | medium |
| 863 | The Mocks journey sometimes misses the short-lived Editor changes saved notice in a loaded full run. | medium |
| 875 | The retired older page's code and tests remain, because many journeys still test the engine through it. | high |
| 966 | Several browser journeys fail their fixed five-second waits or the six-minute limit when the computer is busy, and pass alone. | medium |
| 984 | The saved harness recordings contain only greetings, so no test uses real tool call, edit or question traffic. | medium |
| 1066 | Two engine Git concurrency tests miss a five-second startup marker when the computer is heavily loaded, and pass alone. | medium |
| 1397 | Some tests run with the real home folder, so readiness checks can look at real login paths. | high |
| 1445 | The hosted pull request browser journey sometimes runs past its waits, even when run alone, and the cause is unknown. | medium |

### Documents (3)

| Issue | What is wrong | Confidence |
| --- | --- | --- |
| 1459 | The roadmap and the mock's introduction still describe connected website actions as not connected or as future work. | medium |
| 1545 | The owner decided mocks are drawn after Define, but the build loop design and default workflow still place them inside Define. | medium |
| 1554 | The documented command for serving the mock lets a browser mix old and new scripts, which can leave a gray page. | high |

### Process (2)

| Issue | What is wrong | Confidence |
| --- | --- | --- |
| 521 | Finished review seats leave full repository copies in the temporary folder, which can exhaust its file limit and break every shell. | medium |
| 1474 | The change that lets the owner choose a project lead was merged without an independent code review or a whole test run. | medium |

## P3: records, unlikely to be scheduled

### Engine (26)

| Issue | What is wrong | Confidence |
| --- | --- | --- |
| 48 | The event log and the views modules still import each other, which works only while every use stays inside a function. | high |
| 60 | A retried list command with an old-format receipt would return today's data instead of the first reply; no command reaches this today. | medium |
| 114 | It is not stated or tested whether a parent blocked in a wait also gets a separate crash notice. | low |
| 120 | Leftover agent processes cannot be identified or cleaned up on macOS; recovery leaves them alone there. | high |
| 211 | The account reset timer is tested with one account only, and its immediate recheck has no minimum wait. | high |
| 314 | A phone alert that keeps failing adds one event to the log every ten minutes for as long as the outage lasts. | medium |
| 317 | The secret store reports a failed read the same way as a missing secret, hiding the real cause. | high |
| 318 | Internal projection hands pieces placeholder event details, which would mislead any piece that reads more than the event. | high |
| 340 | Nothing stops a future piece from forgetting its redaction table entries, which would redact its identifiers as text. | medium |
| 348 | Events from agents' runners are recorded as the owner's, so an automatic answer would look like the owner's own. | medium |
| 349 | The turn-started event is declared and read by views but nothing ever writes it. | high |
| 350 | A single tool call's input has no size limit, so one call can make an oversized event. | medium |
| 351 | Permission path checks use blocking file calls, so a slow file system stalls every agent. | high |
| 851 | Files and Git work only for worktrees on this computer, until machine pairing is built. | high |
| 1022 | A terminal can finish opening just after its project is deleted, leaving a shell in an unregistered folder. | medium |
| 1023 | The deletion filter matches field names, not ownership, so replay could drop a surviving record that merely mentions a deleted one. | medium |
| 1028 | Builder folder registrations carry no owning project, so project deletion leaves them behind under a shared worktree. | medium |
| 1029 | Deleting one project could remove a workspace marked for a surviving project if it sits below the removed root. | medium |
| 1030 | After a deletion, a kept workspace can still point to a removed parent, so a later update of it could fail. | medium |
| 1208 | Hung credential file reads no longer block later reads logically, but can still occupy all four system worker threads. | medium |
| 1209 | Every redaction does a blocking check of every account's credential file, including accounts never launched. | medium |
| 1271 | With a very large set of secrets, the first match still builds every spelling at once, costing time and memory. | medium |
| 1283 | Text that repeats the opening characters of many secrets makes the redaction scan about forty times slower than before. | medium |
| 1284 | Scanning a megabyte of text against many long secrets is four to five times slower than the original matcher. | medium |
| 1502 | Copying preparation files safely works only on Linux, so other operating systems refuse configured copying. | high |
| 1539 | The preparation message says the base branch was trusted for submodules even when the repository has no submodules. | high |

### Client API and command line (11)

| Issue | What is wrong | Confidence |
| --- | --- | --- |
| 51 | Harness settings are not held in the live views, so a client can only read them from a list reply. | medium |
| 66 | A request held by the owner still carries a delivery identifier that points at no delivery. | high |
| 164 | An agent following its own event stream may not see its refused commands or messages queued for it. | low |
| 177 | The agent tool server may report a failed exit when its input simply closes after successful calls. | low |
| 315 | Stopped agents accumulate in the client snapshot without a cap, so it could outgrow its size budget. | medium |
| 319 | A very large reply cannot be read over a very slow connection because heartbeats are missed. | medium |
| 359 | The contract states no largest size for an alert event or an alert-list page. | medium |
| 882 | A project cannot choose to remove agent credit lines when merging. | high |
| 884 | A team's need-to-know setting does not exist in the engine. | high |
| 885 | A project cannot set a prefix for its task labels. | high |
| 1099 | The request named board.read is treated as a plain read but also moves the caller's read marker. | medium |

### Git (5)

| Issue | What is wrong | Confidence |
| --- | --- | --- |
| 355 | A child's new worktree starts from the last commit, so a lead's uncommitted changes are missing. | medium |
| 600 | Common SSH and HTTPS sign-in refusals from a remote are reported as uncertain instead of as sign-in required. | medium |
| 1427 | GitHub cannot pin the reviewed base during a merge, so the base can move between the check and the merge. | medium |
| 1521 | Ignore rules added for copy patterns stay forever in every checkout, and no command removes them. | high |
| 1541 | A repository with thousands of tracked files matching a copy pattern could make agent file listings, reads and searches fail. | high |

### Security (8)

| Issue | What is wrong | Confidence |
| --- | --- | --- |
| 49 | The folder limit's duties are met, but SECURITY.md does not state the remaining non-Linux mount limit and worktree limit. | medium |
| 70 | Request identifiers accept any text; the engine refuses secret-bearing ones, but no bounded character set is defined. | high |
| 182 | The client checks the socket and then connects by path, so the same user could swap the socket in between. | high |
| 185 | The client reads the owner credential file without checking it is a regular, private file; a hardening note, not a break. | high |
| 243 | For Codex, the live check cannot prove the ancestry refusal because the shell fails earlier on a socket ownership check. | medium |
| 320 | While the web server is down, another local user could take its port and show a fake page. | medium |
| 793 | An npm configuration file or cache inside the agent-writable install folder can affect or break a harness installation. | medium |
| 1212 | A base64 secret written with escaped slashes or partial percent escapes is not recognised and hidden. | medium |

### Board and roles (4)

| Issue | What is wrong | Confidence |
| --- | --- | --- |
| 327 | Nothing forces every piece to keep its events under the network message size limit. | medium |
| 330 | The agents view forgets an agent's starting permission ceiling once its mode changes. | medium |
| 343 | A standing permission rule answered outside the engine still costs the holder agent a full wake. | medium |
| 1134 | When standing permission rules are built, the hidden website switch for them must be restored in the same change. | high |

### Harnesses and agent running (8)

| Issue | What is wrong | Confidence |
| --- | --- | --- |
| 16 | The protocol schema generator script ends with a formatter that is not installed, so it fails after writing its files. | high |
| 93 | The rule for whether a version 2 agent can be reloaded is built, but never confirmed against the real adapters. | medium |
| 128 | No real recording yet proves the form in which an adapter reports when a usage limit resets. | medium |
| 366 | An agent in a very long Claude project folder path gets no cache timing from the transcript fallback. | medium |
| 719 | Model discovery sessions may leave session history in the Claude, Codex and Gemini account folders, which is never cleaned. | medium |
| 761 | Cursor cannot be installed from the page; the owner must run its official installer by hand. | high |
| 1196 | A compaction started by the harness itself is not detected, so instructions are not restored after it. | medium |
| 1210 | A broken default Cursor credential file refuses a Cursor account whose own configured folder is healthy. | medium |

### Website (18)

| Issue | What is wrong | Confidence |
| --- | --- | --- |
| 613 | Sample data files and one engine test still use the invented agent names Rowan and Sage. | high |
| 614 | The design file has fewer colours and variables than the code and still shows an invented agent name. | medium |
| 616 | The Design page loads a full live copy of the mock for every screen frame, which is slow and heavy. | high |
| 631 | Message post action buttons may be hidden when the message panel is narrow. | low |
| 900 | A file beyond the 500-entry folder listing limit can open without its Commit control or change badge. | medium |
| 905 | A service settings field once came back empty after a reload; the likely cause was fixed but the failure was never reproduced. | low |
| 1026 | A chat tool title still shows the shell wrapper when the command is written as several joined quoted pieces. | medium |
| 1033 | The Git screen sends too many refresh requests; four attempts to rework the scheduling failed review and were not merged. | medium |
| 1098 | Five engine reads the website does not yet use are missing from its list of requests that wait for the connection. | high |
| 1106 | After a cancelled file open, the Files view can keep saying Reading although nothing is being read. | medium |
| 1107 | After a failed terminal settings read, the screen keeps saying Reading terminal settings although the read has ended. | medium |
| 1340 | A plan update rewrites a plan row that sits above later tool calls in the chat. | high |
| 1341 | The starting badge can show old recovery counts for one retry after the service stops answering. | high |
| 1551 | Nine small inconsistencies exist in the screens, such as differing warning thresholds, a stale label and missing search results. | medium |
| 1553 | A tooltip on a button in the row of project views is clipped to a thin dark line. | medium |
| 1556 | Four small gaps remain in the design: a missing paste button, limited unpinning, fixed pin order and a crowded session picker. | medium |
| 1558 | The conflict view in Git cannot have its list resized on its own, because two tests rebuild it from source text. | medium |
| 1570 | Space checks found large empty areas under the lead's conversation and an uneven margin on one document. | medium |

### Tests (31)

| Issue | What is wrong | Confidence |
| --- | --- | --- |
| 26 | Tests may not prove that a secret inside a malformed-frame or encoding failure stays out of the logs. | low |
| 37 | The fixture scrubber hides every environment value of eight or more characters, which can corrupt ordinary words in recordings. | high |
| 197 | The live check could not search harness session files for agent tokens because it never sees the tokens. | medium |
| 198 | The recorder script does not export two of its types, so the live check derives them indirectly. | high |
| 199 | Two separate copies of the 'is the service answering' probe exist and could drift apart. | high |
| 205 | No test proves the live check spares a process whose number was reused by a stranger. | medium |
| 206 | The live check's last-resort error handler, which hides possible secrets, is not exercised by any test. | high |
| 235 | Some engine tests fail when the machine is heavily loaded; two remain unexplained, including a possible real race in the orphan sweep. | medium |
| 247 | A live-check test waits a fixed 20 milliseconds after killing a process and can fail under load. | high |
| 258 | A spawn or log-file error during the live check crashes it without cleanup and can leave processes running. | high |
| 303 | The per-package test command runs nothing and passes, which gives false confidence. | medium |
| 356 | A recorder test once failed at the base commit in one environment and the cause was never found. | low |
| 360 | Starting two services at once timed out once in a merge gate and the cause was never found. | low |
| 593 | Git wrapper tests time out at five seconds when the machine is heavily loaded. | high |
| 611 | A browser test of the mock preview fails now and then in full parallel runs, with the cause unproved. | high |
| 782 | The older admin page's journey sometimes misses the token-cleared label and passes when rerun alone. | low |
| 839 | A browser journey sometimes clicks a permission answer that never reaches the test runner. | low |
| 901 | The Files screen's offline state is tested only by forcing the status, never by a real network interruption. | medium |
| 902 | No test checks that the Files screen releases an image's temporary address when the image changes or the view closes. | medium |
| 970 | In the retired admin page a background refresh can swallow a click on Clear token, making its journey fail about one run in four. | medium |
| 1114 | The test helper that tracks services started by tests misses several ways of launching a program, so those need manual cleanup. | medium |
| 1124 | The live check's reload step occasionally fails with a socket read error, cause unknown. | low |
| 1198 | The sign-in process cleanup test failed once in a full run and passed alone, attributed to load. | low |
| 1240 | Several browser journeys and one sign-in cleanup test fail when the whole suite runs on a busy computer and pass alone. | medium |
| 1241 | The test of two foreground launches racing for one lock failed once in a whole run under load. | medium |
| 1257 | The installer stops a timed-out recipe without waiting for every child process to exit, so one test sometimes sees a live process. | medium |
| 1295 | A service shutdown test sometimes exits with code 13 in a whole run, and four journeys fail under load. | medium |
| 1387 | A test of stopping the runner while a secret prefix is held failed once in a whole run and passed alone. | medium |
| 1415 | The large-output terminal test can take the echoed command for the finish marker and check too early. | high |
| 1416 | A Git overlap test sometimes fails before its held subprocess starts, also on the base commit. | medium |
| 1442 | The file-editor browser journey fails about one time in five, even on the base commit. | medium |

### Documents (7)

| Issue | What is wrong | Confidence |
| --- | --- | --- |
| 2 | The README says pnpm is needed but does not say Node.js 26 lacks it or how to install it. | high |
| 161 | The engine refuses the owner credential when ancestry headers are missing, but the decision record does not state that rule. | medium |
| 324 | The engine design document still describes device pairing and per-device credentials that were dropped. | high |
| 722 | The build-loop diagram still shows a forced exit after round six, though the owner removed that limit. | high |
| 795 | Install recipes cite vendor web pages with no saved copy, so their claims cannot be checked from the repository. | high |
| 1178 | The foundry guide still tells agents to use two commands that only the owner may run. | high |
| 1573 | The feature list holds two entries for the same usage page idea, and they should be merged into one. | high |

## Reported fixed, not verified

A reviewer found signs that these are fixed, without high confidence.

| Issue | What is wrong | Area | Confidence |
| --- | --- | --- | --- |
| 14 | Copied helpers fall back to the whole service environment, but both LGTM Studio callers now pass an explicit base environment. | Security | medium |
| 30 | Three follow-ups from the unit 8a review: the parse error export, absolute harness paths, and the owner reading the first recording. | Engine | medium |
| 45 | Production startup now turns on the database's write-ahead log before the live views first load. | Engine | medium |
| 234 | Registering very many secrets could make the output redactor crash, and it rebuilt its patterns on every call. | Security | medium |
| 515 | A design critique found fifteen misleading or blocking problems in the mock; those fifteen were fixed. | Website | medium |
| 522 | The mock could not start with no projects and offered no harness choice for a new lead. | Website | medium |
| 535 | An idle agent's configuration dispatch entered the delivery writer with no queued work and logged an extra failure line. | Engine | low |
| 540 | The reservation cancellation test could cancel before its fake agent had started the second turn. | Tests | medium |
| 554 | An empty project requests page could return a reply larger than the allowed size. | Client API and command line | medium |
| 566 | Reading review evidence while the owner approved could leave both calls stuck forever. | Git | medium |
| 588 | Building a combined diff rescanned the whole repository once per untracked file, making it very slow. | Git | medium |
| 601 | The engine could not find the editor image by its digest on the test machine. | Engine | medium |
| 627 | The live board guessed a sprint's column from its phase instead of using the engine's status. | Website | medium |
| 760 | The Git journey once failed to find the Use both button after a conflict and refresh. | Tests | medium |
| 769 | Managed harness programs were run without checking they were unchanged; an installation receipt now guards that. | Security | medium |
| 772 | One unreadable login file used to break every harness section of the status. | Harnesses and agent running | medium |
| 813 | The merged full check failed once while the Git journey waited for the Use both button. | Tests | medium |
| 944 | The first project lead skipped the Define step, went straight to sprints, lacked rename and delete tools, and had a persona name. | Board and roles | medium |
| 945 | The lead's chat showed duplicated text, lost lists, raw tool names and repeated file changes. | Website | medium |
| 1156 | The email notification browser fixture failed when run from packages/web because it looked for the mock relative to the working folder. | Tests | medium |
| 1330 | During agent recovery the website and its sign-in form were unavailable. | Engine | medium |
| 1331 | Reloading a browser tab lost unsent chat messages. | Website | medium |
| 1332 | An account that was merely available was shown as signed in without proof. | Engine | medium |
| 1333 | Each plan update appeared as a new chat row. | Website | medium |
| 1335 | Signing out left unsent drafts in the browser. | Website | medium |
| 1336 | Accounts with unknown sign-in status lost controls that used to work. | Website | medium |
| 1337 | Recovery progress counted agents that were not part of the recovery. | Engine | medium |
| 1338 | A very long saved draft could come back cut short after a reload. | Website | medium |
| 1339 | The first page during recovery looked like an empty installation. | Website | medium |
| 1342 | Recovery tests did not cover bad Host or Origin headers, and the feature list was stale. | Tests | medium |
| 1343 | A first fix let an account become available before ordinary reloads finished. | Engine | medium |
| 1344 | Opening a Setup bookmark skipped the loading guard during recovery. | Website | medium |
| 1345 | The too-long-draft notice stayed silent for the next long draft. | Website | medium |
| 1346 | A missing local credential was taken as proof of sign-out and disabled usable accounts. | Engine | medium |
| 1347 | Automatic account checks had no limit and repeated inconclusive reads. | Engine | medium |
| 1348 | Startup could reload a paused agent that already held a live token. | Engine | medium |
| 1349 | An unreadable stored event stopped ordinary agents from reloading at startup. | Engine | medium |
| 1350 | A refused first snapshot left the page loading forever. | Website | medium |
| 1351 | A sample account added without sign-in looked usable in the mock. | Website | medium |
| 1352 | The accounts header read zero signed in out of zero when every account was unknown. | Website | medium |
| 1377 | Older harness settings had no mapping for the automatic permission mode. | Harnesses and agent running | medium |
| 1378 | Changing HOME or XDG_CONFIG_HOME could put a harness login store inside a worktree. | Security | medium |
| 1394 | Folder documentation left out shared Git settings and clean subfolders. | Documents | medium |

## Open entries that live only in a screen that is being rebuilt

The owner is rebuilding the website's screens.
These 51 entries are about a present screen, so the rebuild decides them: 517, 611, 615, 616, 631, 670, 686, 687, 731, 782, 842, 890, 891, 900, 901, 902, 905, 937, 939, 966, 970, 1026, 1033, 1034, 1098, 1106, 1107, 1161, 1162, 1340, 1341, 1375, 1442, 1445, 1544, 1546, 1547, 1548, 1549, 1551, 1552, 1553, 1554, 1555, 1556, 1558, 1560, 1561, 1566, 1569, 1570.

## Behaviour of the retired page worth keeping in the rebuilt website

Issues 571, 573, 594 and 603 were closed as obsolete, because the fault sat in the retired `packages/web` page.
The reviewer judged the behaviour they ask for worth restating as requirements for the rebuilt website.
They are in `issues-closed.md`.

## 1. The engine's event log has no retention rule

Triage (2026-11-23): open, P2, engine, high confidence. The event log keeps every event forever, so the database grows without limit and startup slows as it rebuilds. Basis: No retention or archive code found under packages/engine/src; entry state is open.

Found: 2026-11-01, while writing the engine design
(`docs/specs/2026-11-01-engine-design.md`).
State: open.

What happens: the first version of the engine keeps every event forever,
including every streamed message and tool call of every agent.
What should happen: old events are summarised or archived by a rule the
owner can set, without breaking the rebuild of the views or a client's
catch-up from a sequence number.
Why it matters: the SQLite file grows without limit, and starting the
service gets slower as the log it rebuilds from gets longer.

## 2. Node.js 26 ships without pnpm or Corepack

Triage (2026-11-23): open, P3, documents, high confidence. The README says pnpm is needed but does not say Node.js 26 lacks it or how to install it. Basis: README.md line 73 says only "needs Node.js 26 and pnpm"; no installer step or Corepack note.

Found: 2026-11-01, while installing Node.js 26.13.0 with nvm.
State: worked around on the owner's host on 2026-11-01: pnpm 12.16.1 is
installed with pnpm's standalone installer, in `~/.local/share/pnpm/bin`.
The README's setup steps still need to say this.

What happens: Node.js 26 installs only `node`, `npm` and `npx`. Corepack,
which older Node.js versions used to provide pnpm, is no longer bundled, so
`pnpm` is not available under Node.js 26.
What should happen: the build plan installs pnpm for Node.js 26 at the
version named in the root `package.json` before unit 1 runs
`pnpm install`.
Why it matters: every build step and `pnpm check` need pnpm; without it,
unit 1 fails at its first command.

## 14. Copied environment helpers default to the whole process environment

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. Copied helpers fall back to the whole service environment, but both LGTM Studio callers now pass an explicit base environment. Basis: AcpRunner.ts:509-511 and leadDefaults.ts:53-54 pass buildBaseEnvironment output; spawn uses extendEnv false. makeClaudeEnvironment has no caller.

Found: 2026-11-01, in the unit 2 security read.
State: open; belongs to the unit that starts harness processes.

What happens: `mergeProviderInstanceEnvironment` (`instanceEnvironment.ts`) and
`makeClaudeEnvironment` (`ClaudeHome.ts`) use `process.env` when the caller
passes no base environment.
What should happen: LGTM Studio always passes an explicit base environment, so an
agent gets exactly `LGTM_SOCKET` and `LGTM_TOKEN` and what the harness
needs, as decision 57 says.
Why it matters: a call without the argument would hand every variable of the
service to a harness process.

## 16. The ACP schema generator cannot finish in LGTM Studio

Triage (2026-11-23): open, P3, harnesses and agent running, high confidence. The protocol schema generator script ends with a formatter that is not installed, so it fails after writing its files. Basis: third_party/t3code/effect-acp/scripts/generate.ts:490 still runs `pnpm exec vp fmt`.

Found: 2026-11-01, in the unit 2 security read.
State: open; low priority.

What happens: `effect-acp/scripts/generate.ts` downloads schema files from
GitHub and ends by running `pnpm exec vp fmt`. The `vp` program belongs to the
vite-plus tool, which LGTM Studio does not install, so the last step fails after
the files are written.
What should happen: if LGTM Studio regenerates the schemas, the formatting step is
replaced by an LGTM Studio tool, and the change is recorded in the README table.
Why it matters: someone running `generate` gets a confusing failure.

## 26. The unit 8a tests cannot reach every secret-handling path in issue 13

Triage (2026-11-23): open, P3, tests, low confidence. Tests may not prove that a secret inside a malformed-frame or encoding failure stays out of the logs. Basis: Entry state open; later tests mention malformed input (api/service-log.test.ts, identity/redactor-fix-1.test.ts) but I did not confirm they cover these two paths.

Found: 2026-11-01, while writing the unit 8a tests.
State: open.

What happened: issue 13 asks for planted secrets to be tested through the core,
notification, extension-defect and encoding-failure logging paths. The fake agent
tests drive a real session, so they reach the core and notification paths only
when a step makes the client log them, and the session test asserts that nothing
is logged with wire options off. The extension-defect and encoding-failure paths
need a peer that sends a malformed frame or a failing extension handler, which no
step in criterion 8.1 produces. The redacting service logger of units 5a and 7a
does not exist yet, so the tests install a logger that records every message and
cause, and they check that no planted secret appears in it or in the adapter's
standard error.
What should happen: units 6a, 6d and 7a add a malformed-frame step to the fake
agent, or their own tests, and run the planted-secret check through the real
redacting logger.
Why it matters: without them, a secret in a defect or encoding failure could pass
every unit 8a test and still reach a log.

A related trap: the callback that `it.layer(...)("name", (it) => ...)` passes in
`@effect/vitest` offers `it.effect` but not `it.live`, so a test that needs the
real clock must call the top-level `it.live` and provide its layer itself, as the
unit 8a tests do.

## 30. Later engine units and the live recording have three review seams

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. Three follow-ups from the unit 8a review: the parse error export, absolute harness paths, and the owner reading the first recording. Basis: packages/testing/src/replay.ts:8 exports ReplayParseError; contracts harnesses.ts:28 requires an absolute adapterPath; claude.ndjson is committed.

Found: 2026-11-01, in the focused review of unit 8a.
State: open for the named later work.

Unit 6a adds the named `ReplayParseError` and harness permission mapping exports
when it needs them. Unit 8a does not add unused exports.
Harness-path overrides introduced through settings must also be absolute paths.
The owner must read the first generated `claude.ndjson` before committing it.
The lead tells the manager this when requesting the owner-run recording.

## 37. The fixture scrubber redacts every environment value of eight or more characters

Triage (2026-11-23): open, P3, tests, high confidence. The fixture scrubber hides every environment value of eight or more characters, which can corrupt ordinary words in recordings. Basis: packages/engine/scripts/recorder.ts:27 still adds every value of length >= 8 without checking the variable name. Issue 192 covers the same cause.

Found: 2026-11-01, by the manager, in the owner-run re-record at 0318c78.
State: open.

What happens: `scrubText` replaces every value of the parent environment that
is eight or more characters long. Under pnpm, `npm_package_name` was
`lgtm` when this was found (the product is now called LGTM Studio, whose
seven-letter package name is below the limit, but the same over-scrubbing
applies to any longer value), so ordinary text such as a source note naming
`lgtm-engine` becomes `<redacted>-engine`. The re-recorded
`errors/cursor-out-of-usage.json` was over-scrubbed this way, so the earlier
committed copy was kept.
What should happen: the scrubber redacts the values of account variables and
of names that can hold secrets, and leaves well-known values that are not
secret, such as the package name, version and script name.
Why it matters: over-scrubbing is the safe direction, but it corrupts the
fixtures' text and can hide real differences between recordings.

Rechecked: 2026-11-02, against the live-check recordings from engine 71d13c1.
The refreshed first and second harness recordings both keep
`lgtm-recorder`, and neither has an over-redacted ordinary word.
The second recording's `msg_<redacted>` is a scrubbed message identifier.
The cause remains in `packages/engine/scripts/recorder.ts`, line 26:
`scrubText` still adds every environment value of eight or more characters
to its replacement list, without checking the variable name.
Calling it with `npm_package_name: "lgtm"` changes `lgtm-recorder`
to `<redacted>-recorder`; using `npm_package_name: "lgtm"` leaves
`lgtm-recorder` intact.
The symptom disappeared because the package name is shorter, so this issue
stays open, as issue 192 also records.

## 38. The third harness's greeting fixture records an out-of-usage reply

Triage (2026-11-23): waits for the owner's decision, P3, tests. A normal Cursor greeting recording is still missing; the owner must record it once the account has usage. Basis: packages/testing/fixtures has no cursor.ndjson; replay.test.ts:63 still skips "awaiting the owner's normal Cursor recording".

Owner ruling (2026-11-25): Leave it open until that account has usage again.

Found: 2026-11-01, by the lead, reading the `cursor.ndjson` recorded at e93bb00.
State: open. The fixture is committed as recorded, and the owner ruled on it.
The owner decided on 2026-11-01 to keep the out-of-usage sample as
`cursor-out-of-usage.ndjson`. The owner adds a normal Cursor recording after the
account resets. The file was renamed with
`git mv packages/testing/fixtures/cursor.ndjson packages/testing/fixtures/cursor-out-of-usage.ndjson`,
without changing a byte, in the commit "test(engine): name the Cursor fixture
for what it is (issue 38)" on branch `worker/engine-c38`. Tests replay it as
the out-of-usage case. `replay.test.ts` also has a case for a normal
`packages/testing/fixtures/cursor.ndjson`, where the recorder writes the new
recording; it is skipped, with the reason "awaiting the owner's normal Cursor
recording (issue 38)", until that file exists. The issue stays open until the
owner adds that recording.

What happened: `packages/testing/fixtures/cursor.ndjson` (now
`cursor-out-of-usage.ndjson`) is the
`read-only-greeting` scenario, but the owner's account for the third harness
had no usage left. The prompt turn's only agent text is "Upgrade your plan to
continue", and the turn still ends with `stopReason: "end_turn"`. The header
also records `versionSource: "cli"`, because the adapter reports no version.
What should happen: a test that replays this fixture treats it as an
out-of-usage turn that ends normally, not as a greeting. When the account has
usage again, the owner re-records it so a real greeting exists. Unit 6a's
mapping of this harness's out-of-usage reply should use this transcript
together with `errors/cursor-out-of-usage.json`.
Why it matters: the adapter reports running out of usage as ordinary agent
text with a normal stop reason, so LGTM Studio cannot tell it from a reply by the
stop reason alone.

## 45. The first view rebuild must not open a reader before WAL is enabled

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. Production startup now turns on the database's write-ahead log before the live views first load. Basis: packages/engine/src/service.ts:136 and views/Views.ts:39 set journal_mode WAL before the rebuild.

Found: 2026-11-01, while composing the unit 4c test layers at `4a5ad12`.
State: Verified fixed in unit f1-eventlog, with no change. Production startup sets WAL through a separate connection before the SQLite client and its reader worker open (`packages/engine/src/service.ts`, before `SqliteClient.layer` is provided), and `Views.makeLayer` sets WAL again before its first read.

What happened: `EventLog.layer` currently enables SQLite's write-ahead log
(WAL). The new live-view layer must load its initial state before the writer
that consumes it is constructed. The copied SQLite client's reader pool
permanently falls back to the writer when its first reader opens a database
that is not in WAL mode.
What should happen: enable WAL before constructing the live views or starting
their initial rebuild. The new `test/support/withViews.ts` fixture does this
after migrations. Unit 4c and the later service startup must preserve that order.
Why it matters: enabling WAL only after the first rebuild leaves that client's
reader pool disabled, so comparison can block behind an occupied writer even
though the database eventually reports WAL mode.

## 46. A failure while applying a committed batch would leave the live views behind

Triage (2026-11-23): open, P2, engine, medium confidence. If updating the in-memory views fails after a commit, the views silently fall behind the stored log until restart. Basis: No scheduled rebuild comparison or fatal handling found in packages/engine/src/log, views or service.ts; the one known trigger was fixed.

Found: 2026-11-01, while building unit 4c at `3fb1a96`.
State: Detected, not repaired, in unit f1-eventlog. A defect while applying a committed batch is now caught and logged, every later write is refused, and the version read reports "live views missed a committed batch" with `restartNeeded`.
The service does not rebuild its views by itself; the owner must restart it, and the restart rebuilds the views from the log.
The comparison on a schedule, the other remedy this entry names, is not built.

What happened: the live views are updated in memory after the database commit,
inside the writer permit. The update has no error channel, so it can only fail
by a defect. A defect there would leave the committed log ahead of the live
views, and nothing detects that until the next comparison with a rebuild.
What should happen: the service startup should treat such a defect as fatal
and rebuild the views from the log before serving again, or the comparison
should run on a schedule outside tests.
Why it matters: a client could read views that miss a committed event.
Round 1 of the unit 4c review also found a reachable defect of this kind: an
agent named `constructor`, `toString` or `__proto__` is a valid identity, and
the views, built as ordinary objects, threw after the append committed. Fixed
in round 1: every keyed view has no prototype and is read by own property only.

## 47. Live threads copy their event list on every append and keep full envelopes

Triage (2026-11-23): open, P2, engine, high confidence. Each agent's live thread is copied in full on every event and held in memory forever, so busy agents get slower. Basis: packages/engine/src/views/apply.ts:432 builds a new array per event; threads hold full envelopes (line 112).

Found: 2026-11-01, while building unit 4c at `3fb1a96`.
State: open.

What happened: `apply` stores each thread as an immutable array and builds a
new array for every event, so appending to a long thread is linear in its
length. The thread also holds the whole envelope of every event for the life
of the process.
What should happen: before long-running threads matter, store a bounded window
or event sequences with a read-through to the log, and use a persistent list.
Why it matters: memory and write time grow without bound for a busy agent.

## 48. The views module and the event log import each other

Triage (2026-11-23): open, P3, engine, high confidence. The event log and the views modules still import each other, which works only while every use stays inside a function. Basis: EventLog.ts:21 imports Views.ts; Views.ts:11 and rebuild.ts:8 import log/subscribe.ts, which imports EventLog.ts (line 10).

Found: 2026-11-01, while building unit 4c at `3fb1a96`.
State: open, harmless today.

What happened: `EventLog.ts` uses the `Views` service, and `Views.ts` reaches
`log/subscribe.ts`, which imports `EventLog.ts`, through `rebuild.ts`. The
module cycle works because every use sits inside a function body. Unit 4c also
exported `EventRow`, `decodeRow` and `PAGE_SIZE` from `subscribe.ts` so the
rebuild reuses the same decoding.
What should happen: move the row type, the decoder and the page size into a
small module that neither side imports back.
Why it matters: a later top-level use on either side would fail at load time.

## 49. The folder ceiling and the stream redactor leave duties to later units

Triage (2026-11-23): open, P3, security, medium confidence. The folder limit's duties are met, but SECURITY.md does not state the remaining non-Linux mount limit and worktree limit. Basis: recovery/reload.ts:46-50 rebuilds grants with checkFolder; SECURITY.md has no mention of mountinfo or the non-Linux mount gap.

Found: 2026-11-01, while building unit 5a.
State: open. Fix round 1 of unit 5a (2026-11-01) closed the mount and
back-link items on Linux; each remaining item names the unit that owns it.

What happens: unit 5a's ceiling and redactor meet criteria 5.11 and 5.12, but
these limits remain.

- A protected folder mounted beneath a requested folder. Fixed on Linux in
  fix round 1: the check reads `/proc/self/mountinfo` and compares every mount
  point at or beneath the folder, by device and inode, with each protected
  folder and each of its ancestors, and an unreadable mount table refuses an
  agent-started child. Other platforms have no such table, so there the check
  still sees only the folder's own ancestors, and a protected folder mounted
  below the requested folder is not found.
- Linked worktrees. Fixed in fix round 1: a `.git` file is accepted only when
  git's `gitdir` back-link exists, is a readable regular file, and names the
  folder's `.git` by device and inode. An agent that can write inside the
  parent repository's common directory can still register a new worktree
  entry, with its back-link, that passes. This is the limit decision row 35
  accepts by allowing worktrees at all.
- A folder grant lives in memory. Unit 6d's recovery reload (decision row 52,
  step 7) must rebuild it by calling `checkFolder` with the stored request and
  then compare the result with the stored real path before it calls
  `folderForSpawn`.
- The time between the check and its use. `folderForSpawn` returns a path,
  and Node.js cannot start a child process in a held directory handle, so a
  process running as the same user can redirect that path after the check.
  Unit 6a must call `folderForSpawn` immediately before each spawn. On Linux,
  unit 6a must also compare the child's `/proc/<pid>/cwd` with the granted
  device and inode right after the spawn and again after `session/new`, and
  stop the child on a mismatch.
- `redactChunk` keeps a carry for each item until that item's final chunk.
  The carry is bounded by the longest matched form, but unit 6a must send a
  final chunk for every item, including items cut off by a crash or an
  interruption, or the carries of abandoned items stay in memory.

Unit 6a now rechecks the grant at spawn, compares the Linux child working
folder by path, device and inode before initialization and after session
setup, and closes every text carry on completion, stop, crash and scope
closure. Recovery must still reconstruct grants, and the stated platform
and worktree limits remain.

What should happen: units 6a and 6d meet the grant, spawn-time and
final-chunk duties above, and SECURITY.md states the remaining platform and
worktree limits.
Why it matters: each limit is a way for an agent-started child to reach a
folder, or for the service to hold memory, beyond what the ceiling promises.

## 51. No view holds the harness settings, so a `harnesses set` cannot be read back from the views

Triage (2026-11-23): open, P3, client api and command line, medium confidence. Harness settings are not held in the live views, so a client can only read them from a list reply. Basis: No HarnessChanged handling in packages/engine/src/views; settings are read from the log in commands/internal.ts:82.

State: open. Read on 2026-11-15, not checked against the code. packages/engine/src/views has no harness table and no HarnessChanged handling, so settings are readable only from the list reply.

- **Found:** unit 5b tests, by the test writer.
- **What happens:** the reducer in `packages/engine/src/views/apply.ts` ignores `HarnessChanged`, and the snapshot has no harness table. The only way to see the settings a `harnesses set` wrote is the reply of `harnesses list`.
- **Why it matters:** criterion 5.4 says each lifecycle case asserts the event and the view change. For `harnesses set` there is no view change to assert, so the test reads the list reply, and a client that wants the settings has no live view to subscribe to.
- **What should change:** either unit 5b adds a harness table to the views, or the plan says that the list reply is the harness view.
- **State:** open. The test `5.4 harnesses set` reads the list reply.

## 59. A read failure inside a writer turn latches the whole writer

Triage (2026-11-23): open, P2, engine, medium confidence. One unreadable stored receipt or failed read during a command blocks every later command until the service restarts. Basis: EventLog.ts:281-282 latches every LogWriteFailed except InvalidEvent; CommandHandler.ts:361 and 431 raise read failures as LogWriteFailed.

Found: 2026-11-01, in the focused review of unit 5b.
State: open.

What happens: any `LogWriteFailed` raised inside an `EventLog.transact` callback latches the writer (`packages/engine/src/log/EventLog.ts`, the failure branch of `transact`). That includes read-type failures such as "database read failed" and "stored receipt is unreadable".
Why it matters: one corrupt receipt row then blocks every later command until the engine restarts, and the restart's first retry latches it again.
What should happen: latch only on failures of the write itself (the append, the commit or the side rows), and turn a read-type failure into a refusal of that one command.

## 60. Envelope-only receipts rebuild list replies from the current snapshot

Triage (2026-11-23): open, P3, engine, medium confidence. A retried list command with an old-format receipt would return today's data instead of the first reply; no command reaches this today. Basis: Entry verified against CommandHandler.ts on 2026-11-16; replyFor still builds from the snapshot (line 84, 487).

Found: 2026-11-01, in the focused review of unit 5b.
State: open, low impact; on 2026-11-16 `CommandHandler.ts` still rebuilds an envelope-only receipt's list reply from the current snapshot. Verified against the code on 2026-11-16.
Earlier state: open, low impact.

What happens: a receipt in the older envelope-only format has no stored reply. For `accounts` and `harnesses` list commands the handler rebuilds the reply from the current snapshot (`CommandHandler.ts`). That reply is not the first reply, and it does not pass `redactReceipt`.
Why it matters: today only direct callers of `append` write the older format, so no command reaches this path. If one did, a retry could return data that differs from the first reply.
What should happen: either remove the envelope-only path once no caller writes it, or refuse a list retry whose receipt holds no reply.

## 65. No event withdraws an owner alert, so a closed request's alert is visible only through its status

Triage (2026-11-23): waits for the owner's decision, P2, client api and command line. Nothing announces that an owner alert has ended, so a client must work it out from the request's status. Basis: No alert-withdrawn event in packages/contracts; entry waits for the owner of the client API or the lead to decide.

Owner ruling (2026-11-25): No event announces the end of an alert, and clients read the status. This is kept. The issue can be closed. Because the server and the client are being rewritten in Go, this ruling is an input to that design.

Found: 2026-11-01, while writing the unit 5c tests.
State: open; the owner of the client API (piece 2) or the lead decides.

What happens: decision rows 27 and 51 say that closing or moving a request withdraws its owner alert in the same transaction. The alert is the `OwnerNeeded` event, and the contracts (`packages/contracts/src/events.ts`) have no event that withdraws it. A request never moves down from the owner (row 51), so only closing ends an alert, and a client can see that only from the request's or question's status.
What should happen: either the contracts gain an "owner alert withdrawn" event, or the plan states that an alert ends when its subject closes, and clients derive it.
Why it matters: a phone alert for an answered request stays visible until the client checks the subject. The unit 5c tests cannot assert a withdrawal, so they assert only the closed status.

## 66. A request held by the owner still carries a delivery id that names no delivery

Triage (2026-11-23): open, P3, client api and command line, high confidence. A request held by the owner still carries a delivery identifier that points at no delivery. Basis: packages/contracts/src/events.ts:201 keeps deliveryId required on PermissionRequested.

Found: 2026-11-01, while writing the unit 5c tests.
State: open.

What happens: `PermissionRequested`, `PermissionMoved`, `QuestionAsked` and `QuestionMoved` require a `deliveryId`. When the holder is the owner, nothing is queued, because a delivery's target must be an agent. The id then refers to no delivery, and a later withdrawal of it changes no row.
What should happen: either the field becomes optional for an owner holder, or the plan states that the id is a placeholder there. The unit 5c tests assert only that no delivery is queued for an owner-held request.
Why it matters: a reader or a later unit that looks the id up finds nothing, and could mistake that for a lost delivery.

## 70. A command request identifier could store a live secret unchanged

Triage (2026-11-23): open, P3, security, high confidence. Request identifiers accept any text; the engine refuses secret-bearing ones, but no bounded character set is defined. Basis: packages/contracts/src/ids.ts:8 RequestId is still Schema.NonEmptyString.

Found: 2026-11-01, in the verified attack pass for unit 5c.
State: partly fixed; the engine refuses a secret-bearing request identifier since unit 5c fix round 1, and the bounded identifier alphabet in `packages/contracts/src/ids.ts` remains open. Verified against the code on 2026-11-16.
Earlier state: fixed in the engine by unit 5c fix round 1. A bounded identifier
alphabet remains open for the contracts scope.

What happened: the contracts accept any nonempty string as a request
identifier. The handler could store an agent token unchanged in event and
receipt keys, envelopes and thread views, because identifier metadata bypasses
event text redaction.
What changed: immediately after decoding a command's payload, the handler
passes the request identifier through the event log's `redactText` delegation
to its own `Redactor` service. If the identifier changes, the handler refuses
the command before receipt lookup and writes nothing. This also applies to
commands that would later be refused for rights or state.
What remains: the contracts scope must choose a bounded identifier alphabet.
Why it matters: redacting a reply alone leaves secrets in database keys and
subscription envelopes.

## 76. Engine notices name the owner as their sender

Triage (2026-11-23): open, P2, client api and command line, high confidence. Notices written by the engine name the owner as sender, so a client cannot tell them from the owner's own prompts. Basis: routing/waits.ts:39, recovery/crash.ts:76 and recovery/reload.ts:115 still set sender owner, although Sender now has a service form.

Found: 2026-11-01, while building unit 5d.
State: open for the contracts scope.

What happens: a wait's wake, a wait's crash notice and a returned-delivery notice are deliveries the engine writes. `Actor` has only the owner and agents, so these deliveries carry the owner as sender. Their text is still wrapped in the data envelope, and a wake carries its wait as `source`.
Why it matters: a client cannot tell an engine notice from the owner's own prompt by its sender. A notice that fails is returned to the owner, which ends there, so no notice loops.
What should happen: the contracts add an engine sender, or the plan states that the owner sender with a data envelope marks an engine notice.

## 91. The deny-rule contract cannot express the first harness's only deny mechanism

Triage (2026-11-23): open, P2, security, high confidence. The settings cannot express the Claude adapter's only way to deny tools, so it is recorded as having none. Basis: packages/contracts/src/harnesses.ts:16-22 DenyMechanism has only none and launch-arguments; denyRead.ts:5 comment confirms defaults stay none.

Found: 2026-11-01, while writing the unit 6a tests, reading `@agentclientprotocol/claude-agent-acp` 0.99.1 (`dist/acp-agent.js`, the session options near lines 400-440 and 9030-9095).
State: open for the contracts scope and the lead.

What happens: the adapter takes deny rules (`disallowedTools`, `settings`) only through the `_meta.claudeCode.options` field of `session/new`, not through launch arguments. The contracts' `DenyMechanism` (`packages/contracts/src/harnesses.ts`) has only `none` and `launch-arguments`.
Why it matters: rows 61 and 65 ask for a launch-time deny-read and deny-write rule where a harness supports one. With today's contract the first harness must be recorded as `none`, although its adapter has a mechanism.
What should happen: the contracts gain a session-option form of the mechanism, or the lead records the first harness as having no launch-time mechanism in SECURITY.md (row 72). The unit 6a tests cover only the `launch-arguments` and `none` forms.

## 93. Protocol version 2 advertises no session reload capability

Triage (2026-11-23): open, P3, harnesses and agent running, medium confidence. The rule for whether a version 2 agent can be reloaded is built, but never confirmed against the real adapters. Basis: Entry: rule implemented in unit 6a; post-build check PB-A12 on real adapters remains open, "No real adapter was started".

Found: 2026-11-01, while writing the unit 6a tests, reading `third_party/t3code/effect-acp/src/_generated/schema.gen.ts` (`SessionCapabilities`, line 4094) and the recorded fixtures.
State: the version 2 rule is implemented in unit 6a. Real-adapter confirmation remains open for post-build check PB-A12.

What happens: in version 2, `SessionCapabilities` has no `load` or `resume` field, and neither recorded version 2 adapter advertises one. Version 1 has `agentCapabilities.loadSession`, which the third harness's adapter sets.
Why it matters: the `reloadable` flag in "session opened" (issue 64) decides whether `resume` can reload an agent. For a version 2 adapter it cannot come from an advertised capability.
What should happen: the builder chooses the version 2 rule (for example, that `session/load` is part of the version 2 baseline) and records it with its source; post-build check PB-A12 confirms it on the real adapters. The unit 6a tests assert the flag only for a version 1 adapter that advertises `loadSession`.

Chosen rule: use the copied client's normalized `agentCapabilities.loadSession`
for both generations. `normalizeInitializeResponse` in
`third_party/t3code/effect-acp/src/client.ts` sets that flag for version 2
when `capabilities.session` exists, and exposes baseline resume support.
The runner records the normalized capabilities and negotiated version in its
process entry, and stores the derived reloadability with `SessionOpened`.
This uses the existing client compatibility rule rather than requiring a
version 2 capability field that does not exist. The frozen version 1 load
test and recorded replays pass locally. No real adapter was started.

## 104. Three harnesses have no verified deny-read or deny-write rule

Triage (2026-11-23): open, P2, security, high confidence. Codex, Gemini and Cursor have no verified launch rule stopping them from reading the state folder or writing harness settings. Basis: runner/denyRead.ts:5 keeps defaults at none; SECURITY.md lines 148-161 lists these harnesses as having no rule.

Found: 2026-11-01, by the lead, merging unit 6a for plan task L4.
State: open, for the live check.

What happens: `packages/engine/src/runner/denyRead.ts` gives every harness the
`none` deny mechanism by default. For `codex`, `gemini` and `cursor`, no
launch-argument rule that forbids reading the state folder or writing the
harness settings has been verified against the installed adapter.
What should happen: each of these harnesses gets a verified launch rule in its
default settings, or a recorded finding that its adapter has none.
Why it matters: without the rule, only the folder rule, the permission ceiling
and the approval routing guard the state folder and the settings files (rows
61, 65 and 72). `SECURITY.md` lists these harnesses.

## 113. Nothing closes the open questions of a crashed agent

Triage (2026-11-23): waits for the owner's decision, P2, engine. Questions asked by a crashed agent never close; the owner must decide on a withdrawn event and the blocked-task status. Basis: Entry: owner's decision still open; unit s18-smallfix only hides the questions. No QuestionWithdrawn event in packages/contracts.

Owner ruling (2026-11-25): The recommendation to show the questions as withdrawn is not taken. The open questions of a crashed agent stay open, and its transcripts stay available. The crash is reported to the project lead. The project lead decides whether to relaunch an agent and assign it those questions, or to keep them. The person may answer such a question at any time. Because the server and the client are being rewritten in Go, this ruling is an input to that design.

Found: 2026-11-01, by the unit 6c test writer.
State: the owner’s decision on a "withdrawn" event is still open. Unit s18-smallfix keeps stored questions open and filters unavailable askers from owner reads. Their questions return when they resume. A task whose only blocker is a hidden question still shows as blocked without a visible question explaining it. The owner’s decision must cover that status as well as withdrawal.

What happens: issue 74 says question closure must use an agreed contract and
not invent an answer. No event closes a question without an answer, so the
questions a crashed agent asked stay open after its crash. The unit 6c tests
only assert that a crash writes no invented answer.
What should happen: the owner decides whether a question needs a "withdrawn"
event, or whether an open question of a crashed agent stays open on purpose
until its asker is replaced.
Why it matters: the owner's inbox lists questions that nobody can receive an
answer for.

## 114. A blocking wait on a crashed agent has no stated parent notice

Triage (2026-11-23): open, P3, engine, low confidence. It is not stated or tested whether a parent blocked in a wait also gets a separate crash notice. Basis: Entry: "a reading is recorded and untested"; no code read.

Found: 2026-11-01, by the unit 6c test writer, while reading issue 89.
State: open, a reading is recorded and untested.

What happens: row 67 merges the crash notice with the parent notice "when the
waiter is also the parent" and a non-blocking wait stays open. A blocking wait
creates no delivery, so the rows do not say whether the parent, who is
blocked inside the wait call, should also get a notice.
What should happen: the builder follows the reading in the unit 6c result
(the wait reply is the notice, and no extra delivery is written), or the
owner states otherwise. The unit 6c tests do not cover this case.
Why it matters: a second notice to a blocked parent would be delivered after
its wait call returns and repeat what the call already told it.

## 118. A classified pause drops the provider's error text

Triage (2026-11-23): waits for the owner's decision, P3, engine. When an account pauses, the provider's own error text is dropped, so odd limit messages are hard to diagnose. Basis: Entry: "no change planned until the owner asks".

Owner ruling (2026-11-25): The recommendation to keep dropping the text is not taken. A provider's own error code is not dropped. The person who uses the software is told the error that the provider gave. Because the server and the client are being rewritten in Go, this ruling is an input to that design.

Found: 2026-11-01, by engine-u6bb, building unit 6b.
State: open; no change planned until the owner asks.

What happens: the `promptReply` pause outcome carries an `error` string, but the pause path writes only the fixed reasons ("account needs sign-in", "account is out of usage, no reset time known") so that no provider text reaches the log, a view or a reply.
What should happen: if a diagnostic is wanted, unit 6c classifies and redacts it before it is stored, and the owner decides where it is shown.
Why it matters: an owner sees that an account is paused but not what the provider said, which makes odd limit messages harder to diagnose.

## 120. The leftover-process reader does not support macOS yet

Triage (2026-11-23): open, P3, engine, high confidence. Leftover agent processes cannot be identified or cleaned up on macOS; recovery leaves them alone there. Basis: packages/engine/src/recovery/orphans.ts:40-43 macosProcessReader still throws "unsupported".

Found: 2026-11-01, while building unit 6c.
State: unsupported recovery is handled in unit 6d. A macOS implementation and host verification remain open.

What happens: `recovery/orphans.ts` provides the Linux reader and an explicit
unsupported macOS reader. The macOS reader throws before the sweep can send
a signal.
What should happen: a macOS implementation reads the boot identity, process
group, start time, user and executable, and passes criterion 6.8 on a macOS
host before recovery uses it.
Why it matters: a sweep must refuse unsupported process identity checks
rather than signal a group based only on its number. The lead authorized
this stub for unit 6c.

What changed in unit 6d: restart step 5 runs the sweep with the host's
reader. A reader that throws for a row is treated as "process identity
unavailable", so that group is left alone and never signalled. Recovery goes
on. If the sweep itself fails, recovery logs the fixed warning "leftover
process sweep failed" and goes on.

## 128. Classified pauses need trustworthy error sources and reset times

Triage (2026-11-23): open, P3, harnesses and agent running, medium confidence. No real recording yet proves the form in which an adapter reports when a usage limit resets. Basis: Entry verified 2026-11-16: classification and producer fixed; packages/testing/fixtures holds no new recording with a reset time.

Found: 2026-11-01, in unit 6c's focused review, finding F-4, and the unit 6b
review's ruling A3, relayed by engine-lead in board post 980.
State: partly fixed; error-source classification and the reset-time producer are fixed, and no recorded adapter fixture yet proves the real reset-time form. Verified against the code on 2026-11-16.
Earlier state: error-source classification is fixed in unit 6c fix round 1. The
reset-time producer is fixed in fix round 2 after merging unit 6b. Recorded
adapter evidence for a reset time remains open.

What happened: the runner classified a successful turn's text, so ordinary
agent prose containing "authentication required" could produce a signed-out
pause. Unit 6b would then apply that pause to the entire account.
What changed: a signed-out classification requires an actual error reply
in `turn.error`. Successful text can produce only an out-of-usage pause,
and only for cursor, whose recorded transcript uses that text form
(issue 38). All other harnesses classify only their error replies. The
same restrictions apply to an injected classifier.
The runner now reads `data.resetsAt` only from the error for its active
`session/prompt` request. This includes the terminal idle error in Agent
Client Protocol version 2, where the raw request reply is an acknowledgement.
Reading only the raw reply dropped valid deadlines in the frozen tests.
An out-of-usage pause carries a parsed date string
or a finite, non-negative integer in epoch milliseconds, within 366 days
of the current time in either direction. Invalid values are ignored, so
unit 6b keeps its "owner needed" path when no deadline is known. Successful
turn text and signed-out pauses never carry this field. The classifier's
signature is unchanged.
What remains: recorded fixtures must verify the real adapter's reset-time
form. The fake agent's `out_of_usage` step already supplies `data.resetsAt`,
but no recorded fixture holds a reset time yet, as recorded in the brief.
Why it matters: account recovery cannot schedule a usage reset from a
classified pause that lost its reset time.

## 134. An out-of-usage account still sends work to its idle siblings

Triage (2026-11-23): waits for the owner's decision, P2, engine. When one agent hits a usage limit, idle agents on the same account still get work and each wastes a turn. Basis: Entry: "open question for the owner".

Owner ruling (2026-11-25): When one agent hits a usage limit, the idle agents on the same account are paused too. Because the server and the client are being rewritten in Go, this ruling is an input to that design.

Found: 2026-11-01, in the focused review of unit 6b (finding C3).
State: open question for the owner.

What happens: rows 28 and 53 make only a sign-out account-wide. When one agent reports a usage limit, the account is marked out of usage and that agent pauses, but its idle siblings keep receiving work, and each may hit the limit once before it pauses itself.
What should happen: the owner decides whether idle siblings should pause (and their queued work wait) as soon as the account is known to be out of usage.
Why it matters: each sibling's first turn after the limit is wasted, and a turn that fails with the limit costs a delivery requeue.

## 150. An owner prompt carries no delivery identifier, so a replay cannot find it

Triage (2026-11-23): open, P2, engine, high confidence. An owner prompt carries no identifier, so after a restart it may be sent again and the agent repeats work. Basis: packages/engine/src/routing/envelope.ts:33-34 passes the owner's text unchanged with no delivery identifier.

Found: 2026-11-01, while building unit 6d, named in the lead's ruling #1042.
State: open; the lead decides later, because it changes owner prompt text (rows 38 and 58).

What happens: `routing/envelope.ts` (`agentText`) sends the owner's prompts
unchanged, with no envelope and no delivery identifier. After a restart, the
replay scan cannot find an owner prompt that was sent but never accepted.
What should happen: an owner prompt carries its identifier in a form the
replay scan can read, or recovery uses another proof that it was received.
Why it matters: such a prompt is sent again after a restart, so the agent may
do the same work twice.

## 152. A paused agent that cannot reload after a restart has no path back except stop

Triage (2026-11-23): waits for the owner's decision, P2, engine. A paused agent that cannot reload after a restart can only be stopped and started anew, leaving its queued work stuck. Basis: Entry: owner decides whether resume --fresh accepts a paused agent; unit s21-recover added only a website fallback.

Owner ruling (2026-11-25): A paused agent that cannot reload may be resumed fresh from the paused state. Because the server and the client are being rewritten in Go, this ruling is an input to that design.

Found: 2026-11-01, while building unit 6d.
State: open. Unit s21-recover adds the website's fresh fallback for states that accept resume, but the existing command contract refuses resume while paused.

What happens: when its account returns, a paused agent with a session whose
adapter cannot reload gets `AgentReloadFailed` with the fixed reason "agent
reload failed" and stays paused. `resume` refuses a paused agent, so only
`stop` and a new start remain.
What should happen: the owner decides whether `resume --fresh` should accept
a paused agent, or whether the return should open a fresh session by itself.
Why it matters: the agent's queued work stays stuck until the owner notices.

## 161. Row 64 does not say what an upgrade without an ancestor chain means

Triage (2026-11-23): open, P3, documents, medium confidence. The engine refuses the owner credential when ancestry headers are missing, but the decision record does not state that rule. Basis: Entry verified 2026-11-16: code fixed in commit 2b33da2, decision row 64 not updated.

Found: 2026-11-01, while writing the unit 7a tests.
State: partly fixed; since commit `2b33da2` the engine refuses the owner credential with 403 when the ancestry headers are missing or malformed, but decision row 64 still does not record that rule. Verified against the code on 2026-11-16.
Earlier state: open for the lead; the unit 7a tests take the reading below.

What happens: row 64 says the client sends its ancestor chain and process
group on upgrade, and the engine refuses the owner credential when one of
them matches a recorded adapter process. It does not say what happens when
the chain or the group is missing or malformed.
What should happen (the tests' reading): the engine refuses the owner
credential with HTTP 403 when either header is missing or malformed, so a
caller cannot skip the check by leaving the headers out. Agent tokens and
device credentials do not need the headers.
Why it matters: reading a missing chain as "outside every adapter" would make
the check optional for any caller.

## 164. An agent's subscribe never shows its own refusals or the deliveries queued to it

Triage (2026-11-23): open, P3, client api and command line, low confidence. An agent following its own event stream may not see its refused commands or messages queued for it. Basis: contracts events.ts:258 CommandRefused has no agentId; log/subscribe.ts:89 filters on agent_id. A thread_agent_id column now exists, effect on subscribe not checked.

Found: 2026-11-01, while writing the unit 7a tests.
State: open for the lead; criterion 7.3 is followed as written.

What happens: criterion 7.3 makes every agent-less event owner-only. Two
event types concern an agent without carrying a top-level `agentId`:
`CommandRefused` (no agent field) and `DeliveryQueued` (its agent is
`delivery.target`). The log stores no `agent_id` for them, so an agent's
`subscribe` never returns them, even for its own command or its own queue.
What should happen: either the criterion stays, or those events gain an
`agentId` (a contracts change) so they can be shown to the agent concerned.
Why it matters: an agent following its own thread cannot see that its
command was refused, or that a message is waiting for it.

## 177. Closing the MCP input can report an error after a successful tool call

Triage (2026-11-23): open, P3, client api and command line, low confidence. The agent tool server may report a failed exit when its input simply closes after successful calls. Basis: Entry state open; packages/mcp/src/server.ts:116-128 now waits on stdin end and close, which may already fix it. Not run.

Found: 2026-11-01, while writing the unit 7b fix-round tests against commit
`220efe3`, with uncommitted test helpers.
State: open shutdown usability note. This does not block the assigned fixes.

What happened: a test host ran the real `runMcp`, initialized the protocol,
listed the tools, and completed an `ask` against a fake socket peer. After
the parent closed standard input, `runMcp` returned exit code 1. The host
stayed alive to inspect the server layer's resource cleanup. The observed
return code was independent of the leaked connection asserted by X22.
What should happen: investigate whether the normal end of standard input
races the protocol layer's termination in `runMcp`. An orderly shutdown
should not report an error solely because the input ended.
Why it matters: a caller can see a failed server shutdown after its tools
completed successfully. X22 deliberately checks resource cleanup without
requiring a particular shutdown return code, because that behavior is
outside this fix round's assigned rulings.

## 182. The socket check and the connection are not one atomic step

Triage (2026-11-23): open, P3, security, high confidence. The client checks the socket and then connects by path, so the same user could swap the socket in between. Basis: No peer-credential check anywhere in packages (grep for SO_PEERCRED and peerCred finds nothing).

Found: 2026-11-01, while building the client's socket checks.
State: open.

What happens: the client checks every parent folder, the socket's folder and
the socket, and then opens the connection by path. A process running as the
same user, or a root process, could replace the socket between the check and
the connection. Only the state folder's owner can do this, so the engine's
trust boundary is not crossed, but the check is not a guarantee.
What should happen: if the platform allows it, check the connected
descriptor's peer credentials after connecting.
Why it matters: the check protects the owner credential from a swapped socket.

## 184. `lgtm mcp` discards its own logs

Triage (2026-11-23): open, P2, engine, high confidence. The agent tool server writes no log at all, so a failed tool call leaves no record to debug. Basis: packages/mcp/src/server.ts:112 still provides Logger.layer([]); no log file sink found.

Found: 2026-11-01, while building the tool server.
State: open.

What happens: the tool server's standard output carries the protocol, so the
server runs with `Logger.layer([])` and writes no log at all. A connection
failure reaches the agent as a tool error, and nothing is recorded.
What should happen: write the server's log to a file in the state folder,
without any credential.
Why it matters: an owner debugging an agent's failed tool call has no record.

## 185. A symlinked owner credential file is followed

Triage (2026-11-23): open, P3, security, high confidence. The client reads the owner credential file without checking it is a regular, private file; a hardening note, not a break. Basis: packages/client/src/identity.ts:48 uses plain readFileSync with no lstat or mode check.

Found: 2026-11-01, by the unit 7b attack pass (B2); the verifier showed no
break.
State: open, a hardening note.

What happens: `packages/client/src/identity.ts` reads `owner-credential` with
`readFileSync`, which follows a symbolic link.
Why it is not a break: replacing that file needs write access to the private
state folder, and a process with that access already holds the owner's
authority, so the link gives it no new data or authority.
What could be added later: check that the credential is a regular file, owned
by the user, with mode 0600, before reading it.

## 193. An error from the fake harness is never classified as signed out or out of usage

Triage (2026-11-23): open, P2, harnesses and agent running, high confidence. Only four built-in harness names can be recognised as signed out or out of usage; any other harness is never paused. Basis: packages/engine/src/runner/classify.ts:6-9,29-30 still keys patterns on claude, codex, cursor, gemini only.

Found: 2026-11-02, while writing the acceptance suite (unit 8b).
State: open.

What happens: `packages/engine/src/runner/classify.ts` decides whether a
failed turn means "signed out" or "out of usage" from the harness id, and only
the ids `claude`, `codex`, `cursor` and `gemini` have patterns. An agent on a
harness with any other id, including the fake harness the tests use, gets the
same message as a plain error and is never paused. The acceptance scenario for
account failures therefore registers the fake adapter under the harness id
`claude`.
What should happen: either the classification patterns are part of the
harness definition, so a test harness can carry its own, or the limitation is
written in the harness documentation.
Why it matters: a new harness added later is silently never paused when its
account runs out, and the failure shows only as a crashed turn.

## 194. A sent but unaccepted prompt reaches the agent twice after a restart

Triage (2026-11-23): waits for the owner's decision, P2, engine. After a service kill, a prompt sent but not yet accepted is delivered to the agent twice; the owner must confirm this is acceptable. Basis: Entry's own state: allowed by the specification, a decision to confirm; no later mention in docs/issues.md.

Owner ruling (2026-11-25): The recommendation to accept the repeat is not taken. A prompt that may be a repeat is marked as a possible repeat, and only when the transcript cannot prove that it was delivered. The agent must not answer twice. Because the server and the client are being rewritten in Go, this ruling is an input to that design.

Found: 2026-11-02, while writing the acceptance suite (unit 8b).
State: open. Allowed by the specification ("may be sent twice"), so it is a
decision to confirm, not a defect.

What happens: when the service is killed after a prompt was sent to an agent
and before the agent accepted it, recovery requeues the delivery and sends it
again. The agent's transcript then holds the same prompt twice. The log shows
one delivery, so a reader of the log sees nothing wrong.
What should happen: the owner confirms that a double delivery is acceptable,
or the sent event records enough for the engine to tell the agent that the
prompt may be a repeat.
Why it matters: an agent that acts twice on the same instruction can do
harm, for example by repeating a commit or a message to a third party.

## 195. The acceptance suite leaves some specification clauses to lower-level tests or to nothing

Triage (2026-11-23): waits for the owner's decision, P3, tests. The acceptance suite does not itself check several specification clauses; the owner decides whether lower-level tests are enough. Basis: Entry is a coverage map ending in an owner decision; several rows still say 'not covered'.

Owner ruling (2026-11-25): The acceptance suite may leave some clauses to lower-level tests. The issue can be closed. Because the server and the client are being rewritten in Go, this ruling is an input to that design.

Found: 2026-11-02, by the focused review of unit 8b (finding C1), after fix
round 1 of that unit.
State: open. Not a defect in the engine. It is a map of what the acceptance
suite does not itself check.

What happens: fix round 1 closed the review's findings A1 to A5. The flow now
follows all eight steps of "One request, start to finish". The disk test
observes the adapter. The nested wait, the automatic usage reset and the
overdue reset across a restart are exercised. The secret scan covers every
service process, every minted token, the accounts list and all replies. Many
clauses the review's table lists were not turned into acceptance scenarios. The
table below names, for each one, the lower-level test file that covers it, or
says "not covered". A file is named only when it exists in
`packages/engine/test` and its test titles speak about the clause. Its
assertions were matched by title, not audited line by line.

The foundation review also identified board-specific proof gaps in records
FR-047, FR-062, FR-063, FR-072, FR-076, FR-101, FR-112, and FR-122.
The collection and restart scenarios now correlate board manifests with
delivery identifiers and count through an independent completed prompt.
The acknowledgement matrix uses actual questions, returned deliveries,
capacity holds, and both wait modes.
The recovered withdrawal matrix invokes the dispatcher, reactor, and answer
command, and checks the order within each committed batch.
The error matrix sends authenticated current-incarnation events through the
runner handler and reopens the database after each error.
The deferral matrix covers both stages and runs a mutual-question control
with the stall watch active.
The alert matrix covers reverse resolution order, a fresh baseline from
rebuilt state, and an owner wait whose status and stream are offset.
The recovery matrices cover complete committed checkpoints, actual service
kills, repeated reactor runs, and later replies and lifecycle changes.
These checks live in `packages/engine/test/board/` and
`packages/engine/test/acceptance/board-*.test.ts`.
They do not close the other omissions listed below.

| Clause not checked by the acceptance suite | Covered at a lower level by |
|---|---|
| Protected folder relationships, forbidden home, state and install folders, worktree boundaries | `identity/ceiling.test.ts`, `identity/ceiling-fix-1.test.ts`, `commands/authorization.test.ts` |
| Adapter-reported mode elevation, extended ceiling with persistent permission options | `runner/mode-ceiling.test.ts`, `runner/hardening.test.ts` |
| Unrelated inherited environment excluded | `runner/baseEnvironment.test.ts` |
| Leading slash and whitespace-plus-slash prompts, closing markers, distinct markers per send | `routing/text-is-data.test.ts` |
| Open permission and question counts, complete paused, interrupted and crashed state sequences | `views/apply.test.ts`; the counts are not covered |
| Wait settled-state exclusions for an open request or question, held subject, outstanding delivery, compaction | `routing/waits.test.ts` (W01 to W05) |
| Wait resolution by stop, pause or nonreloadable interruption, held-request early return, member returning to work, crashed-waiter suspension and resume, disconnect cancellation | `routing/waits.test.ts` (W11 to W21) |
| Exact member-state payload of a wait | not covered |
| Plan entries of a stream | `runner/stream.test.ts` (6.2) |
| Split and encoded secrets met by the real redactor | `identity/redactor.test.ts`, `identity/redactor-fix-1.test.ts`, `runner/hardening.test.ts`; the acceptance scan itself is proven only on synthetic text by `acceptance/secret-scan.test.ts` |
| Parent reading its child's complete thread | not covered |
| Busy-holder delivery order of a routed permission request, persistent and settings options | `routing/permissions.test.ts`, `routing/permission-boundaries.test.ts` |
| `resume --fresh`, nonreloadable session refusal and fresh session | `commands/lifecycle.test.ts`, `recovery/restart.test.ts` |
| Picker distinguishing stopped, crashed and interrupted agents and other harnesses | `accounts/pick.test.ts` (P1 to P3) |
| Stored secret echoed by the adapter and redacted | `commands/request-id-secrets.test.ts`, `runner/hardening.test.ts` |
| Crash of a parent without an open wait, stderr truncation to a tail | `routing/waits.test.ts` (W10) and `recovery/crash.test.ts`; truncation is covered by `identity/redactor-fix-1.test.ts` |
| Nonreloadable agent, failed reload, idle reload with zero prompts, agent caught starting, blocking-wait cancellation, surviving question, paused-account recovery | `recovery/restart.test.ts`, `recovery/restart-regressions.test.ts`, `recovery/restart-accounts.test.ts`, `routing/waits.test.ts` (W20) |
| Automatic timeout deadline of a moved permission request, withdrawal of the old holder's copy, paused or interrupted holder, no move back down | `routing/move-withdrawal.test.ts`, `routing/moving-requests.test.ts`; a timeout deadline is not covered |
| Unknown usage reset time alerts the owner, mid-turn carry-on after acceptance | `accounts/pause-and-return.test.ts` (T4, T5), `accounts/carry-on-dispatch.test.ts` |
| Message to an agent that stays nonreloadably interrupted, ordinary adapter error reply returning the message with redacted error text | `routing/deliveries.test.ts`, `runner/classify-errors.test.ts` |
| Cleanup before serving, boot, user and executable mismatch controls, bounded record removal | `recovery/orphans.test.ts` (6.8 guards); the order "before serving" is not covered |
| File-mode protections of the state folder and owner credential | `api/state-folder.test.ts`, `identity/owner-credential.test.ts` |

What should happen: the owner decides whether the acceptance suite must show
any of the "not covered" rows itself, or whether the lower-level tests are
enough for the unit's sign-off. A row naming a file is a pointer, not a proof
that the file checks the whole clause.
Why it matters: a reader of the acceptance suite's titles could believe that
every clause of the specification's amendments is checked end to end. Only the
journeys above are.

## 197. The live check cannot look for agent tokens in the harnesses' own session files

Triage (2026-11-23): open, P3, tests, medium confidence. The live check could not search harness session files for agent tokens because it never sees the tokens. Basis: Entry state open; issue 212 later added a planted-token scan of session files and arguments, which may cover it. Not verified in code.

Found: 2026-11-02, while writing `pnpm live-check` (unit 8c).
State: already fixed before unit f3-security, by the token-scan probe that issue 212 added; unit f3-security changed nothing.
The live check now reads each agent's `LGTM_TOKEN` from `/proc/<pid>/environ` of the run's own adapter processes (`packages/engine/scripts/live/liveProbes.ts`, `snapshot`).
Its `token-scan` probe searches the session files that the run's sessions name, and the run's process arguments, for every captured token, and never prints the text (`scanTokens` and `sessionFiles` in `packages/engine/scripts/live/probeEvidence.ts`).
The tests in `packages/engine/test/live-check/probe-fix-round-1.test.ts` and `probe-fix-round-2.test.ts` cover it with planted tokens.

What happens: the plan's secret check (PB-A4) greps the session files that each
harness writes under its home for the agent's token. The 8b acceptance suite can
do this because the fake agent records its own environment. `pnpm live-check`
cannot: an agent's token is given to the agent when it starts and is never
visible over the socket, so the script has no token to search for. The live
check scans the events, replies, the accounts list, the service's output and the
thread text for the owner credential only.
What should happen: the owner can grep Claude Code's and Codex's session files
for the tokens of the run by hand, or a later unit can add a debug command that
returns a one-way hash of each live token for the scan to compare against.
Why it matters: a token written to a harness's session file would be readable
by anything that reads that home. Today only the fake-agent suite proves this
does not happen.

## 198. The recorder's option and result types are not exported

Triage (2026-11-23): open, P3, tests, high confidence. The recorder script does not export two of its types, so the live check derives them indirectly. Basis: packages/engine/scripts/recorder.ts:68,85 unexported; scripts/live/recording.ts:15-16 still derives them.

Found: 2026-11-02, while writing `pnpm live-check` (unit 8c).
State: open; the check works around it.

What happens: `scripts/recorder.ts` exports `recordFixtures`, `formatReport` and
`hasRecordingFailures`, but not the types `RecordOptions` and `RecordResult`
that those functions use. `scripts/live/recording.ts` derives them with
`Parameters<typeof recordFixtures>[0]` and `Effect.Success<ReturnType<...>>`.
What should happen: the recorder exports its two types, and the live check uses
them.
Why it matters: the derived types break silently if the recorder's signature is
wrapped or overloaded.

## 199. The startup-hook check keeps its own copy of the socket probe

Triage (2026-11-23): open, P3, tests, high confidence. Two separate copies of the 'is the service answering' probe exist and could drift apart. Basis: packages/cli/src/serve.ts:8 private answers; plugins/herdr/src/startupHook.ts:81 own socketAnswers (file moved from scripts/live).

Found: 2026-11-02, while writing `pnpm live-check` (unit 8c).
State: open.

What happens: `serve --detach` decides whether a service already runs with an
HTTP GET on `engine.sock`. That helper (`answers`) is not exported from
`packages/cli/src/serve.ts`,
so `scripts/live/startupHook.ts` carries a second copy (`socketAnswers`). The
two can drift: if the service ever stops answering a plain GET, `serve --detach`
and the live check would disagree.
What should happen: `packages/cli/src/serve.ts` exports the probe and the live check imports it.
Why it matters: the live check's claim that "the service answers" should be the
same claim that the startup hook relies on.

## 205. The live check has no test that a reused process number is spared

Triage (2026-11-23): open, P3, tests, medium confidence. No test proves the live check spares a process whose number was reused by a stranger. Basis: Entry state open; issue text at line 4861 still cites it as a missing reuse test.

Found: 2026-11-02, while fixing issue 204 at commit 0dda3e3.
State: open.

What happens: the live service's stop records each process's pid and start time and checks both again just before each signal, so a pid that the system gave to a stranger after the original exited is not signalled.
No test makes a pid change owner between the record and the signal, because a test cannot force the system to reuse a pid.
The path-boundary, descendant, ignored-signal and unconfirmed-stop cases are tested.
What should happen: the identity check is moved behind a small function that a test can feed a changed start time.
Why it matters: the one guard against signalling a stranger after pid reuse is read, not run, by the tests.

## 206. The live check's crash handler is not covered by a test

Triage (2026-11-23): open, P3, tests, high confidence. The live check's last-resort error handler, which hides possible secrets, is not exercised by any test. Basis: packages/engine/scripts/live-check.ts:8-12 still holds the handler inline beside the main call.

Found: 2026-11-02, while fixing issue 200 at commit f8715ed.
State: open.

What happens: `scripts/live-check.ts` prints only the kind of an unexpected error, because its message may repeat a credential.
Importing the file runs `main`, so no test can load the handler without starting a run, and the handler is verified by reading it.
What should happen: the handler moves into a function in `scripts/live/run.ts` that a test can call with an error holding a credential.
Why it matters: the last place a credential could reach the terminal is the one place no test looks.

## 211. The account reset timer's test has one account, and its immediate recheck has no floor

Triage (2026-11-23): open, P3, engine, high confidence. The account reset timer is tested with one account only, and its immediate recheck has no minimum wait. Basis: reset-timer.test.ts has three issue-210 cases only; accounts/timers.ts still uses Math.max(0, ...) for the wait.

Found: 2026-11-02, in the focused review of the issue 210 fix (commits
f4d79a6 and 5662a5f on branch `worker/engine-acfl`).

Status: open. Neither point is a defect on a clock that only moves forward.

The first point is a test gap.
The test file `packages/engine/test/accounts/reset-timer.test.ts` uses a
single account.
It has no case with two accounts and no case where a deadline is replaced
during a check.
The reviewer changed the timer to match a past release attempt on the account
alone, instead of on the account and its deadline.
All four tests still passed with that change.
In the running engine the change would be masked, because the event that
replaces a deadline also wakes the timer.
Fix: add one case with two accounts, one whose release fails and one whose
deadline is crossed during the check, and one case with a deadline replaced
during a check.

The second point is about robustness.
In `packages/engine/src/accounts/timers.ts`, a deadline that comes due during
a check is rechecked after a wait of zero milliseconds.
In Effect, a sleep of zero is a yield, so the wait has no floor.
A wall clock that moved backwards and then forwards across the deadline on
consecutive checks could make the timer loop quickly.
A single step of the clock, such as a time-server correction, does not cause
the loop.
Fix, if wanted: give the recheck a floor of one millisecond, or state in the
file's header comment that the timer assumes the clock does not move back.

## 212. The live check omitted post-build restart and security evidence

Triage (2026-11-23): open, P2, client api and command line, medium confidence. A permission option that saves a rule does not say where it saves it, so the saved-permission security check cannot pass. Basis: Entry's last paragraph: exposing the destination in the contract is the open work; no permission destination field in packages/contracts/src.

Found: 2026-11-02, in post-build review finding L-1.

Status: the probe implementation is written and covered locally with fake
adapters. Real-adapter evidence remains pending the lead's reviewed live run.

The original fourteen steps did not inspect the adapters' session files or
process arguments. They did not try a state-folder canary read, slash-command
escaping, saved permission scopes, removed identity variables, or a real
adapter reload. A passing journey therefore did not establish PB-A3, PB-A4,
PB-A7, PB-A11, PB-A12, or SEC-PB-1 through SEC-PB-8.

The live harness now records explicit outcomes for those criteria. It cites
existing fixture evidence for PB-A10 and exercises service reconnection for
PB-A8 during the restart probes. A missing attempt fails. The state-folder
probe uses only a non-secret canary in the run's temporary state folder. A
successful read is recorded as having no mechanism, as SECURITY.md requires.
The token scan reports paths and counts, and never reports matched text.
Persistent permission answers are restricted to the named command in the
run's work folder. Only the service child started by the run receives the
restart signal. No production permission or authentication code was changed.

The restart probe uses two kills. One interrupts working turns to observe
actual carry-on tool calls. The other interrupts sent envelope deliveries to
observe recovery accepting their replayed identifiers or re-sending them.
A receipt from an ordinary post-restart response is not counted as replay.
A delivery already accepted before the kill is not counted as a replay test.

Local checks cover both directions of received slash escaping, the real
command-line entry point's ancestry refusal with both variables removed,
old-token refusal after a real service restart with a fake adapter, planted
tokens in session files and arguments, and each probe's failure outcomes.
These checks do not establish real-adapter behavior or close the live gate.

Fix round 1 makes the evidence limits explicit. The two-agent tree proves
the upward slash receive path only for the first harness, and the downward
path only for the second. Its other rows are direction-specific and not
applicable. The core-inheritance check now requires a name-only listing from
the shell before explicit variable removal. Each token-scan row requires its
own agent token, nonempty session files, arguments and environment snapshots.

The saved-permission criterion remains unproven when a destination cannot be
verified before answering. The public permission-option contract carries an
identifier, name, kind, level and persistence, but no destination. The
installed first adapter's wire request omits its provider suggestions too.
The probe therefore rejects every persistent option without a verified
local-settings or session destination. It records "no safe persistent option
offered" and fails. It never infers the destination from a label and never
answers persistently before checking that the owner holds the request.
No production contract change was made for this limitation.

The replay probe asks a pending turn to sleep, then requests the second kill
as soon as the deliveries are sent. A content update may still accept a
delivery before the kill. The probe checks the whole interval again after
reload, and records that race as a failed attempt. It never counts the same
delivery being accepted early and then re-sent as successful replay evidence.

Fix round 2 addresses review findings m-7 and m-8. After the second restart,
the permission handler previously denied even the exact pending sleep
command. A replay that requested that permission could fail despite correct
recovery. The handler now permits that command at most once per agent, using
only a nonpersistent approval. It still denies a repeat or a different
command and fails the probe. The pending prompt explicitly forbids running
the command again after a restart, including when the prompt is re-sent.

The added regression tests isolate missing shell listings, a wrong harness
for an inapplicable outcome, missing environment snapshots with tokens and
arguments still present, and a stale token snapshot after the first restart.
They cover each guard separately without weakening the existing evidence
requirements. These remain local tests, not real-adapter evidence.

Fix round 4 distinguishes a command that ran without any permission request
from a request without a safe persistent destination. In live-check-2 run 2,
events 322-325 show the exact printf command completing, with no
PermissionRequested or PermissionAnswered for that call. No option or
destination was selected. The probe now reports
`no persistent permission request observed`, which fails. It does not claim
that an always-allow answer was sent or that a settings write failed after
one. It still requires the owner-held request, verified destination and
saved scope before it can pass SEC-PB-4.

Fix round 6 addresses the cause observed again in live-check-2 run 3 at
`b4154fa7c12ed47116493b7b209c63a355075c7b`, log lines 34-36. The command
`printf 'lcp-persistent-permission-probe\n'` was read-only, so Claude Code
completed it without a permission request. It could not exercise the
persistent-permission choices. The implementation commit is
`fix: exercise persistent permission with a local write`.

The new exact command uses the absolute Node.js executable to create
`lcp-persistent-permission-probe.txt` at an absolute path in the run's work
folder. The file contains one non-secret marker line. Exclusive creation
refuses to overwrite an existing file or follow an existing file symlink.
The installed Agent Software Development Kit, version 0.3.312,
sdk.d.ts:2033-2044 documents manual approval in default mode for dangerous
operations. The installed claude-agent-acp 0.99.1, dist/acp-agent.js:7962
and 8001-8050, forwards the kit's permission asks.
dist/permissions/options/shell.js:89-96 builds the Bash choices from the
kit's suggestions. Creating a file gives the probe a filesystem mutation
instead of a read-only print. Real permission behavior still needs an
observed request; a command that completes without one keeps the same
`no persistent permission request observed` failure.

The destination guard from m-3 is unchanged. An owner-held request can
receive always allow only for the exact command in the work folder and a
verified localSettings or session destination. An absent or unsafe
destination is denied and records `no safe persistent option offered`.
The existing settings.local.json containment and saved-scope checks remain.
The command itself writes no harness settings or user settings. Local
regressions run the marker command after a synthetic safe approval and
check its file, the sent prompt, the saved scope, rejection of printf and
other commands, the no-request failure and the unsafe-destination failures.

Live run 5 at `9751bda` gives the permission probe its first real result.
The round-6 write command made Claude Code ask for permission.
The request offered a persistent option, `allow-with-updates`.
The probe rejected the request before answering always, and it recorded
`no safe persistent option offered`, because the public permission contract does
not expose where a persistent option would save its rule.
Criterion SEC-PB-4 therefore stays failed: the option is routed to the owner,
but its scope cannot be recorded and no settings file was inspected after an answer.
Exposing the destination in the contract is the open work for this criterion.
The log is `.scratch/foundry-runs/engine-build-1/live-check-2/live-check-run5.log`.

## 231. Adapter updates still queued when the adapter exits are dropped without a trace

Triage (2026-11-23): open, P2, engine, medium confidence. The last output an agent wrote before it exited or was stopped can be missing from its thread, with no warning. Basis: AcpRunner.ts:631 still returns for a dead process; docs/issues.md lines 4263 and 17286 say the stop-path loss remains under 231.

Found: 2026-11-02, while fixing issue 223 on branch `worker/engine-shb`.
State: open. It is older than the issue 223 fix.

What happens: when the adapter process exits, the exit watcher in
`packages/engine/src/runner/AcpRunner.ts` sets `processEntry.dead = true`
at once (line 507 at commit `6882768`).
The session update handler returns without doing anything for a dead
process (line 396 at the same commit).
An update the adapter wrote before it exited, but which the handler had not
yet reached, is therefore dropped.
No warning is logged and no event is written for it.
The drop depends on timing.
A first version of the issue 223 fix made each update slower to handle, and
the refresh regression test then lost tool output written just before a
crash.
The final fix handles each update in one redaction pass, and that test
passed eight runs in a row, but the race remains.
What should happen: every update the adapter wrote before it exited is
handled before the process is treated as dead, or a dropped update is at
least logged.
Why it matters: the last output before a crash, often the most useful for
diagnosis, can be missing from the thread.

Additional evidence from the issue 242 focused review, finding E, on
2026-11-03: stopping an agent also loses the redactor's held tail for both
ordinary text and terminal output.
The two `E diagnostic` regressions in
`packages/engine/test/runner/terminal-lifecycle-round-7.test.ts` start a fake
adapter through the real command handler, wait for a notification behind
the held tail, and stop the agent through the owner command.
Both tails appear in the runner's delivery updates during stop.
Neither appears in a stored thread item.
This is a separate loss point from the queued-notification race above.
`packages/engine/src/commands/stop.ts:19-20` revokes the token in the stop
transaction before calling the runner.
`packages/engine/src/commands/CommandHandler.ts:142-147` ignores runner
events after the agent leaves a live state or its token is revoked.
The terminal path flushes its carry in `AcpRunner.ts` during close.
The issue 242 round-7 scope leaves this shared stop-state path unchanged.

## 234. Enough registered secrets make the chunk redactor throw

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. Registering very many secrets could make the output redactor crash, and it rebuilt its patterns on every call. Basis: redactor.ts:254-278 caches compiled forms; no spread Math.max left in identity/; commits 644c481d and 654c91f7 (issue 232).

Found: 2026-11-02, by the security review of the issue 223 fix (commit
b88b876, finding F7).
State: open, for the owner's hostile pass.

What happens: `redactChunk` in `packages/engine/src/identity/redactor.ts`
computes its window with one `Math.max` call that spreads the length of
every matched form of every registered secret (line 276).
Each secret adds several forms.
With enough secrets, the argument list passes the runtime's limit, and the
call throws `RangeError: Maximum call stack size exceeded`.
The review reached that with one valid login store of 672,012 bytes holding
24,000 distinct values.
Fix round 1 of issue 223 caps a login store at 256 values, so one store can
no longer do this.
The same limit can still be reached by many stores, many `secret:` values,
or both.
The redactor also rebuilds every form of every secret on each call.
What should happen: the redactor handles any number of registered secrets
without an exception, and does not rebuild unchanged forms on each call.
A possible fix is an iterative maximum, with the forms cached until the set
of secrets changes.
Why it matters: an exception in redaction stops that output path, and the
per-call cost grows with every secret.

## 235. Several acceptance and runner tests fail when `pnpm check` runs at high machine load

Triage (2026-11-23): open, P3, tests, medium confidence. Some engine tests fail when the machine is heavily loaded; two remain unexplained, including a possible real race in the orphan sweep. Basis: Entry state: partially addressed; failure-1-crash and recovery/orphans findings stay open.

Found: 2026-11-03, by the test runner (`pnpm check` at 8830d8b, posts #1319,
and at ebe060f, post #1321).
State: partially addressed by the test-only reliability fix. The local limits
in `classify-turn-text.test.ts` (6.9 A3) and `reset-time.test.ts` (6.10 F-4)
are now 120 seconds because each case starts several fake adapters in turn.
The 7.3 WebSocket open limit in the test helper is now 30 seconds (issue
248). The failure-1-crash and recovery/orphans findings stay open. The 6.9
and 6.10 findings are addressed only as far as their test limits go. Wider
limits reduce these failures but do not prove the absence of other load
failures.

What happens: two batches of `pnpm check` runs failed only on tests that the
branches under check did not touch, and the failing tests differed between
runs.

- At 8830d8b, runs 2 and 3 started at a load average of about 21, beside
  another branch's checks. Both failed the same test,
  `packages/engine/test/api/subscribe-visibility.test.ts`, "7.3 issue 41: a
  subscriber that stops acknowledging is disconnected while the writer runs
  on". Its client failed with `SocketOpenError: timeout waiting for "open"`.
- At ebe060f, three runs started at a load average of 92.9, above the test
  runner's limit of 24. Run 1 failed
  `packages/engine/test/acceptance/failure-1-crash.test.ts` ("1 crash: a
  parent that waits on the crashing child is woken once"), with the parent's
  echo not yet holding the crash notice. It also failed
  `packages/engine/test/runner/classify-turn-text.test.ts` (6.9 A3) and
  `packages/engine/test/runner/reset-time.test.ts` (6.10 F-4), each with
  "Test timed out in 30000ms". Run 2 failed
  `packages/engine/test/recovery/orphans.test.ts` (6.8 sweep, Linux), with an
  unexpected extra process group in the sweep result. Run 3 failed
  `reset-time.test.ts` (6.10 F-4) again, by the same timeout.

Each failing file passed when run alone: `subscribe-visibility.test.ts` three
times at load 47, and the four files from ebe060f together once at load 81.
Both heads then passed `pnpm check` when the runs were repeated one at a time
below the load limit (8830d8b in post #1325).
At 9340f71 (post #1329), run 2 of 3 failed `classify-turn-text.test.ts`
(6.9 A3) and `reset-time.test.ts` (6.10 F-4) again, each by the 30-second
timeout, although the load was below 24. Runs 1 and 3 passed. Run alone at
load 26, the slowest case in each file took 19.6 seconds (6.9 A3, "a
successful turn whose text says 'authentication required'") and 17.4 seconds
(6.10 F-4, the list of bad reset values). Both cases start a fake adapter for
each harness or each value in turn, so their fixed 30-second limit leaves
little margin when the full suite runs in parallel. These two failures are
therefore test sizing, not load alone, and they predate the B1 merge (they
also failed at ebe060f).
Why it matters: a test that fails only under load hides real failures behind
noise, and a check run at high load costs a full repeat. The orphan sweep
failure may be a real race, because the sweep found a process group it did
not expect; it has not been reproduced.
What should happen: the test runner keeps waiting for load below its limit
and runs one check at a time. If a test from this list fails again at normal
load, the wait or timeout in that test is examined, as in issue 196: the wait
should name the thing the next assertion is about, and fixed 30-second
timeouts should be checked against the work the test does.

## 237. Approval-gated tool servers need form elicitation routed through LGTM Studio

Triage (2026-11-23): open, P2, harnesses and agent running, high confidence. Approval prompts from other tool servers are cancelled inside the adapter instead of reaching LGTM Studio's permission flow. Basis: No 'elicit' match in packages/engine/src or packages/contracts/src; entry reserved as follow-up by the owner.

Found: 2026-11-03, while the owner decided issue 236.

Status: open. This is option (a), reserved as a follow-up by the owner.

The runner advertises no form elicitation capability. A Codex adapter can
therefore cancel a tool-approval elicitation before LGTM Studio sees a permission
request. Issue 236 configures LGTM Studio's own server to need no such approval,
but other tool servers retain their approval settings.

Advertise form elicitation and route a tool-approval elicitation through
LGTM Studio's permission flow. This is needed once agents use other tool servers
that require approval. The work must preserve the server's requested approval
and carry its answer through the adapter, rather than cancel it internally.

## 240. A Claude Code agent's token is visible in a process's command line

Triage (2026-11-23): open, P2, security, high confidence. A Claude Code agent's token appears in a process command line, readable by any process of the same user. Basis: AcpRunner.ts:848 still passes LGTM_TOKEN value in the mcpServers list; owner deferred it to the hostile pass on 2026-11-03.

Found: 2026-11-03, by the token scan in live-check-2 run 2 at 9e10ed6.

State: fixed for Claude Code in unit f3-security. The runner now gives Claude Code the tool server's token as the reference `${LGTM_TOKEN}`, which Claude Code expands from its own environment, so the token is no longer on the `claude` command line.

What each adapter accepts, checked in unit f3-security:
Claude Code 0.3.312, the binary the pinned Claude Agent SDK starts, both inherits its own environment into a stdio tool server and expands `${VAR}` in the server's `env` entries.
This was checked by running that binary with a probe server that wrote its environment to a file.
Codex receives only the variable names through `CODEX_CONFIG`, as issue 236 chose, so it was already safe.
Gemini and Cursor receive the list only over the Agent Client Protocol pipe, and the runner does not put it in any argument.
Whether those two programs pass it to a child process's arguments was not checked, because Gemini is not installed on this host and Cursor's program is closed.
They keep the literal token, because neither program's environment handling has been confirmed.
The test `issue 236: a Claude start retains the ACP server` in `packages/engine/test/runner/lgtm-server.test.ts` fails before the fix and passes after it.

Status: open, deferred by the owner to the hostile pass (2026-11-03).
The tokens are per spawn and short-lived, and on a single-user machine the
owner treats this as ordinary exposure.

The owner's decision, relayed through the manager on 2026-11-03:

> do not fix it now. The tokens are per spawn and short-lived, and on a single-user machine this is ordinary exposure, the same as any tool that puts a token on its command line. Leave issue 240 open and deferred to the hostile pass.

The runner gives every adapter except Codex the LGTM Studio tool server through
the Agent Client Protocol (ACP) `mcpServers` list, with the agent's token as
the value of the `LGTM_TOKEN` environment entry
(`packages/engine/src/runner/AcpRunner.ts:517-518`). The Claude Code adapter
hands that list to the Claude Agent SDK. SDK version 0.3.312 starts the
`claude` program with `--mcp-config` followed by the whole list as JSON, so
the token's value is part of that process's arguments. On Linux any process
of the same user can read them from `/proc/<pid>/cmdline`, and tools such as
`ps` show them. This breaks PB-A4 and SEC-PB-1, which require that no agent
token appears in process arguments.

Evidence: the live check found the Claude lead's own token, raw, in the
arguments of its run process 1501141 (an agent process whose working folder
is the run's work folder). The Codex row of the same step reports the same
process, because the scan checks every token of the run against every
argument snapshot. Codex itself receives only the variable names through
`CODEX_CONFIG` (issue 236, option (b)), so its token is not in any argument.
The run's folder is kept at `/tmp/lgtm-live-Twd6bJ`.

Possible fixes, not chosen: pass only the variable's name and let the tool
server inherit the value from the adapter's environment, which already holds
it (`AcpRunner.ts:307`), if the Claude Agent SDK and Claude Code keep the
parent environment for stdio tool servers; or use Claude Code's `${VAR}`
expansion in the server's `env` entry, if `--mcp-config` expands it. Either
must be checked against the pinned versions before it is relied on. The same
path is used for every other non-Codex harness, so each needs the same check.

## 241. A real adapter shell obtains owner access after removing both identity variables

Triage (2026-11-23): open, P1, security, medium confidence. A real agent's shell got owner access after removing its identity variables, where a refusal was required. Basis: Entry deferred to the hostile pass; cause never established; no later fix recorded. ancestry.ts trusts the caller's claimed ancestry chain.

Found: 2026-11-03, while diagnosing live-check-2 run 2 at 9e10ed6.

State: the observed cause is fixed in unit f3-security. The owner guard now identifies an adapter by its boot, process group and start time, and no longer by the executable recorded at spawn.

Cause, established in unit f3-security: the runner records an adapter's executable from `/proc/<pid>/exe` as soon as the spawn returns.
The Claude Code and Codex adapters are `node_modules/.bin` launchers, which are `/bin/sh` scripts that later `exec` Node.js.
On this host the recorded executable was `/usr/bin/dash`, and the same process read as `node` a moment later.
`packages/engine/src/api/ancestry.ts` skipped every row whose executable no longer matched, so it found no live adapter and accepted the owner credential.
The fake adapter used by the older tests is started as Node.js directly, so its executable never changed, and those tests passed.
The test `packages/engine/test/api/ancestry-exec.test.ts` fails before the fix and passes after it.
Other ways an agent's process can still act as the owner need a design change and are recorded in issue 1597.

Status: open, deferred to the hostile pass by the fix-round-5 addendum
(2026-11-03). This is a real security finding for SEC-PB-5. The round-5
terminal-output fix does not change the ancestry guard.

The Claude lead ran the exact removed-variable command. Events 157-158
record its input, event 159 requests permission, event 161 selects allow-once,
and event 162 marks the call completed. The call id is
toolu_01H5T2W8rk4qpNBxZNMddW4r. The command removes LGTM_SOCKET and
LGTM_TOKEN with env -u, sets LGTM_HOME to the run's state folder and
starts the real command-line interface's accounts list command. Its actual
tool result, line 102 of the lead's session file, contains the accounts list
and LCP_EXIT=0. The engine's thread lost that output, as recorded in issue 242.

This is owner access, rather than the required HTTP 403 ancestry refusal.
The guard in packages/engine/src/api/ancestry.ts did not prevent this real
adapter shell's request. The exact reason it missed the caller's ancestry
remains to be established. Passing fake-adapter tests do not refute this
observed real-adapter result.

Evidence: /tmp/lgtm-live-Twd6bJ/state/events.sqlite, opened read-only, and
the Claude Code session transcript of that run under `~/.claude/projects/`,
lines 101-102. The exact command is recorded in the worker's round-4 result.
It contains no token value.

## 243. The real Codex shell stops at socket ownership before the ancestry check

Triage (2026-11-23): open, P3, security, medium confidence. For Codex, the live check cannot prove the ancestry refusal because the shell fails earlier on a socket ownership check. Basis: Entry: fix round 5 reports 'not observable (issue 243)', which remains failing; SEC-PB-8 unproven.

Found: 2026-11-03, while diagnosing live-check-2 run 2 at 9e10ed6.

Status: open. The live harness path remains unproven for SEC-PB-8. No client,
engine or sandbox rule was changed.

Events 233-234 show the exact removed-variable command and its failed
completion for Codex. Its session tool output at line 47 contains a complete
LCP_ENV_BEGIN and LCP_ENV_END listing. Neither LGTM_SOCKET nor
LGTM_TOKEN appears in it. Core inheritance did remove both names. The
probe printed absent=false because issue 242 hid the output, rather than
because either name was observed. The probe now distinguishes an unobserved
listing from observed absent or present names, without weakening its gate.

The actual command-line interface printed `lgtm: the folder /tmp above
the socket is owned by another user`, followed by LCP_EXIT=1. This is the
local check in packages/client/src/socketCheck.ts, before authentication.
It is not the HTTP 403 ancestry refusal required by SEC-PB-8. The original
sandbox's ownership view was not captured, so its namespace or filesystem
cause has not been established. The guard must not be weakened to force a pass.

Evidence: the kept events.sqlite and lines 44-47 of
the Codex session transcript of that run under `~/.codex/sessions/2026/11/03/`.

Fix round 5 reports `not observable (issue 243)` in the Codex ancestry row
when this exact socket ownership rejection is observed in the tool output.
That outcome remains failing. It does not count as an HTTP 403 ancestry
refusal and does not change the socket ownership check.

## 246. Generic tool output has no runner retention bound

Triage (2026-11-23): open, P2, engine, high confidence. Output from tools other than the terminal is stored whole, with no size limit. Basis: packages/engine/src/runner/translate.ts:77 still does JSON.stringify(rawOutput) with no truncation; line 5177 says 246 remains open.

Found: 2026-11-03, while selecting the retention bound for issue 242.

Status: open. The round-5 scope adds a terminal-output bound and does not
change other tools' output behavior.

packages/engine/src/runner/translate.ts serializes rawOutput or the whole
content array without truncation. AcpRunner.ts then redacts and emits the
whole translated tool update. There is no generic retained-output bound in
those paths. The existing bounded diagnostic path is recovery/crash.ts,
which keeps 30 stderr lines of at most 16,384 characters each. The new
terminal-output path reuses that character bound for each terminal rather
than claiming a generic tool bound already existed. Broader retention policy
for other tools needs a separate decision and change.
Issue 270 records the separate, fixed amplification of bounded terminal
snapshots. Its fix does not bound generic tool output.

## 247. The live readiness regression can observe a killed child after its fixed wait

Triage (2026-11-23): open, P3, tests, high confidence. A live-check test waits a fixed 20 milliseconds after killing a process and can fail under load. Basis: packages/engine/test/live-check/probe-fix-round-3.test.ts still has setTimeout(resolve, 20) before the 'no live process' assertion.

Found: 2026-11-03, during fix round 5, at 138197a with the terminal-output
changes uncommitted.

Status: open test reliability finding. No readiness behavior or frozen test
was changed for this round.

The live-check folder failed once in probe-fix-round-3.test.ts:151. The test
kills its worker, waits 20 milliseconds and expects the readiness check to
report no live process. The check instead returned undefined. The same
folder had passed earlier in the round. The failed run overlapped a runner
folder run. A scheduling race is suspected, but the process state at the
failed assertion was not captured, so the cause is not established. Future
diagnosis should observe the child exit rather than assume the fixed wait
establishes it. This finding does not authorize weakening the readiness gate.

## 251. Interrupting a committed command can skip its runner work

Triage (2026-11-23): open, P1, engine, medium confidence. A disconnect right after a command is saved can skip its process work, leaving the record and the real process disagreeing. Basis: Entry deferred by owner 2026-11-03; no later fix recorded. CommandHandler.ts has since gained engineOwned uninterruptible steps, not verified to cover this.

Found: 2026-11-03, in the post-build review of commit `8ff4c9b`, finding A2.
State: fixed in unit f2-lifecycle. A start, stop or resume now runs in the engine's scope, and once it has committed its runner work runs to its end inside its lane even when the caller goes away, during the work or between the commit and the work; `packages/engine/test/commands/lifecycle-races.test.ts` proves it. This chooses that a cancelled start finishes in the background, which the owner can still reverse. A caller that went away gets no reply, and a retry with the same request id still returns the stored receipt, because the work already ran.

Evidence: the post-build A2 adjudication is in
`.scratch/foundry-runs/engine-build-1/review-post-build/adj/result-2.md` (not tracked).
It adjudicates flow1 F-1, s13 F-1 and flow5 F-3
by source inspection and hand reasoning against Effect 4.0.5's source.

What happens: `packages/engine/src/commands/CommandHandler.ts` restores
interruption around the runner step after the transaction commits.
A disconnect or cancellation can therefore skip or interrupt that step.
The stored receipt makes a same-identifier retry return without the step.
In `packages/engine/src/commands/start.ts`, cancellation during setup can
leave a committed agent starting, with its prompt queued and no process.
In `packages/engine/src/recovery/restart.ts`, that starting agent is revived
after a restart even though the caller cancelled its start.
In `packages/engine/src/commands/stop.ts`, interruption after the commit
can leave a live process whose agent is recorded as stopped and whose token
has been revoked.
In `packages/engine/src/commands/resume.ts`, the post-commit step has the
same shape, although its setup already has a time bound.

What should happen: committed runner work needs a durable completion or
repair policy, including receipt replay.
The owner has not chosen whether a cancelled start finishes in the background
or fails without being revived.
A change that prevents interruption must follow post-build A3's setup bound.

Score: MAJOR, severity 6 × fallout 3 × probability 5 = 90, band schedule.
The stop path separately scores severity 6 × fallout 4 × probability 2 = 48,
band record.
Why it matters: the recorded lifecycle can disagree with the process until
the owner intervenes.

## 252. Resuming a crashed waiter does not restore its unaccepted wait notice

Triage (2026-11-23): open, P1, engine, medium confidence. A lead that crashes and is resumed never receives its child's finished-work notice, so delegated work stalls. Basis: Entry deferred by owner; commands/resume.ts and recovery/reload.ts show no code restoring a withdrawn wait notice.

Found: 2026-11-03, in the post-build review of commit `8ff4c9b`, finding A5.
State: fixed in unit f2-lifecycle. A successful reload or resume now puts back each wait notice that the waiter's crash withdrew before the waiter read it, as long as its wait is not cancelled and the waiter has not been stopped since; `packages/engine/test/routing/crashed-waiter-notice.test.ts` proves it. This chooses restoring the saved completion, not reopening the wait.

Evidence: the post-build A5 adjudication is in
`.scratch/foundry-runs/engine-build-1/review-post-build/adj/result-4.md` (not tracked).
It adjudicates flow3 F-2 by source inspection,
hand reasoning and a dependency-free replay of the relevant source functions.
The replay did not execute a real resume or the Effect runtime.

What happens: `packages/engine/src/routing/waits.ts` can satisfy a wait and
queue its completion while the waiter is busy.
In `packages/engine/src/recovery/crash.ts`, a crash withdraws that notice
through `targetGone` in `packages/engine/src/routing/deliveries.ts`.
The wait remains satisfied in `packages/engine/src/views/apply.ts`.
Neither `packages/engine/src/commands/resume.ts` nor
`packages/engine/src/recovery/reload.ts` restores the withdrawn notice.
Wait checks revisit only open waits, so the resumed lead can look settled
without receiving its child's completion.
An ancestor wait opened after resume, or an all-wait kept open by another
busy member, can then report that lead settled before it handles the result.

What should happen: preserve enough durable association to restore an
unaccepted notice after a successful resume, without repeating accepted notices.
The owner has not chosen between restoring the saved completion and reopening
the wait to compute the members' current states.
Stopping the waiter must keep its cancellation behavior.

Score: MAJOR, severity 6 × fallout 3 × probability 4 = 72, band schedule.
Why it matters: delegated work can stop progressing until the owner prompts
the resumed lead.

## 253. A stalled recovery send prevents healthy recovered agents from continuing

Triage (2026-11-23): open, P1, engine, medium confidence. After a restart, one agent that stops reading input can block every other recovered agent from continuing. Basis: recovery/restart.ts:283 sendQueued still uses a plain Effect.forEach; docs/issues.md line 4924 still refers to 253 as the defect.

Found: 2026-11-03, in the post-build review of commit `8ff4c9b`, finding A6.
State: fixed in unit f2-lifecycle. Restart recovery's step 10 and an account return now send to each agent in a fiber of its own, each still under its own lane, reservation and incarnation; `packages/engine/test/recovery/recovery-send-independence.test.ts` proves it. The recovery call itself still returns only after every send, which issue 1605 records.

Evidence: the post-build A6 adjudication is in
`.scratch/foundry-runs/engine-build-1/review-post-build/adj/result-5.md` (not tracked).
It adjudicates flow4 F-1 and s21 F-2 by source
inspection and hand reasoning.
No stalled-transport test was executed in that adjudication.

What happens: the startup and account-return send loops in
`packages/engine/src/recovery/restart.ts` dispatch agents sequentially.
In `packages/engine/src/runner/AcpRunner.ts`, sending waits for the adapter's
input write acknowledgement without a transport deadline.
An adapter that stops reading can hold the first send indefinitely.
In `packages/engine/src/routing/deliveries.ts`, the ordinary follower cannot
rescue recovered agents that are already working, because that state requires
the recovery dispatch option.
The service can already be serving requests while these agents remain blocked.
The stalled agent's lane also keeps an ordinary stop waiting behind its send.

What should happen: give each recovery dispatch independent scoped progress
while preserving its lane, incarnation and delivery reservation.
Bound or cancel the transport acknowledgement with a policy for partial writes.
The send deadline and partial-write policy remain owner choices.
Issue 85's fix for the ordinary follower does not cover these recovery loops.

Score: MAJOR, severity 6 × fallout 3 × probability 4 = 72, band schedule.
Why it matters: one stalled adapter can strand healthy agents after a restart
or an account return.

## 254. An obsolete account return can overwrite a newer signed-out pause

Triage (2026-11-23): open, P2, engine, low confidence. A stale account return can mark an agent working after a newer sign-out, leaving it stranded when the account is cleared. Basis: Entry deferred by owner 2026-11-03; no later fix recorded; final-writer revalidation not confirmed in recovery/reload.ts.

Found: 2026-11-03, in the post-build review of commit `8ff4c9b`, finding A7.
State: fixed in unit f3-security. The return's final writer, and its failure append, now also require that the return still applies to the agent's pause and that the account is still available, so a superseded return is discarded and its new process is stopped.
The pause has no generation number, so the check uses the pause's reason and the account's status, which a sign-out always changes.

Evidence: the post-build A7 adjudication is in
`.scratch/foundry-runs/engine-build-1/review-post-build/adj/result-5.md` (not tracked).
It adjudicates flow4 F-2, s21 F-1 and flow5 F-2
by source inspection and hand reasoning.
The post-build A7 row in `.scratch/foundry-runs/engine-build-1/review-post-build/register.md` (not tracked)
retains this as a lifecycle finding, because no security property is shown to fail.

What happens: `packages/engine/src/recovery/restart.ts` checks account
availability before loading a paused agent's session.
While setup runs, a sibling on that account can report a sign-out through
`packages/engine/src/routing/deliveries.ts` and pause that agent again.
The final writer predicate in `packages/engine/src/recovery/reload.ts`
checks only whether the agent is still paused.
The obsolete return then opens the session and marks the agent working,
with a queued carry-on, despite the newer sign-out.
The signed-out send guard still prevents sending.
A later owner clearance selects paused agents, so it misses this working
agent and leaves its carry-on stranded.

What should happen: revalidate the account, its availability, the return
cause and the pause generation inside the final writer transaction.
Discard a superseded setup result and stop its replacement process.
The failure append also needs the same lifecycle validation.

Score: MAJOR, severity 6 × fallout 3 × probability 4 = 72, band schedule.
Why it matters: clearing the account does not restore progress without an
additional stop and resume, or a service restart.

## 255. Reading one unreadable receipt disables unrelated commands

Triage (2026-11-23): open, P1, engine, high confidence. One unreadable stored receipt makes the service refuse every later command until it is restarted. Basis: log/EventLog.ts:164 still raises LogWriteFailed 'stored receipt is unreadable'; line 281 latches every LogWriteFailed except InvalidEvent.

Found: 2026-11-03, in the post-build review of commit `8ff4c9b`, finding A11.
State: Fixed in unit f1-eventlog. All three receipt decoders (the event log, the command handler and the alert retry) now raise `ReceiptUnreadable`, which refuses only the request that reads it, reports "stored receipt is unreadable", and leaves the writer's health unchanged.

Evidence: the post-build A11 adjudication is in
`.scratch/foundry-runs/engine-build-1/review-post-build/adj/result-6.md` (not tracked).
It adjudicates flow7 F-1 and s05 F-1 by source
inspection and hand reasoning.
This entry records post-build A11 from
`.scratch/foundry-runs/engine-build-1/review-post-build/register.md` (not tracked).
It also supplements the broader receipt-read problem in issue 59.

What happens: receipt decoding in `packages/engine/src/log/EventLog.ts`
turns malformed stored data into `LogWriteFailed`.
The separate receipt decoder in `packages/engine/src/commands/CommandHandler.ts`
does the same inside the writer transaction.
The event log treats either read failure as a write failure and latches
command refusal for the service's lifetime.
Unrelated fresh commands then fail even though the database is writable.
The reason allow-list in `packages/engine/src/api/server.ts` reduces the
reported reason to "command refused".
Restarting clears the latch, but replaying the bad receipt sets it again.

What should happen: refuse an unreadable receipt locally at both decoding
sites, while keeping lifetime refusal for actual storage write failures.
Cover malformed text, invalid envelopes and invalid stored replies in both
the old and current receipt formats.

Score: MAJOR, severity 6 × fallout 3 × probability 4 = 72, band schedule.
Why it matters: one corrupt or incompatible receipt can stop every caller's
command flow until the owner restarts the service and uses a fresh identifier.

## 256. Failed transaction cleanup can bypass the lifetime database refusal

Triage (2026-11-23): open, P2, engine, low confidence. A failed save followed by a failed undo can escape the rule that the service refuses commands after a storage fault. Basis: Entry deferred by owner 2026-11-03; no later fix recorded; rare double fault, not re-verified in code.

Found: 2026-11-03, in the post-build review of commit `8ff4c9b`, finding A12.
State: fixed in unit f3-security. `EventLog.transact` now turns a storage defect from the transaction, such as a failed COMMIT whose cleanup ROLLBACK also failed, into a write failure that latches the lifetime refusal with its fixed reason; any other defect stays a defect.
Unit f1-eventlog then replaced the permanent latch: such a defect is now a write failure with the reason "database write failed: rollback failed", writes pause and are retried, and the next write reconciles the views with the log first.
The fault was confirmed in Effect 4.0.5's `makeWithTransaction`, which passes such a double fault through `failuresToDefects`, and which also makes a failed ROLLBACK after a failed body a defect.

Evidence: the post-build A12 adjudication is in
`.scratch/foundry-runs/engine-build-1/review-post-build/adj/result-6.md` (not tracked).
It adjudicates flow7 F-2 and s05 F-2 by source
inspection and hand reasoning against Effect 4.0.5's transaction source.
No double-fault transaction test was executed in that adjudication.

What happens: a failed database commit command (`COMMIT`), followed by a
failed database rollback command (`ROLLBACK`), can leave the transaction
failure in Effect's defect channel.
The connection is poisoned by
`third_party/t3code/node-sqlite/src/nodeSqliteClient.ts` until cleanup succeeds.
In `packages/engine/src/log/EventLog.ts`, typed-error mapping and observation
miss that defect, so the lifetime write-failure latch is not set.
If the next connection acquisition successfully rolls back, a new command
can succeed despite the earlier storage write failure.
The first failure also bypasses the promised fixed database reason.
The adjudicator corrected the flow report's claimed fixed web response:
`packages/engine/src/api/server.ts` can instead return a remote procedure
call (RPC) defect response on an upgraded connection.
This entry records the command-refusal defect, without claiming a reproduced
disclosure or permanent corruption.

What should happen: classify the complete transaction outcome at the database
boundary and retain lifetime refusal with a fixed safe reason.
Preserve connection cleanup before reuse and distinguish application defects
and receipt-read failures from storage failures.

Score: MAJOR, severity 6 × fallout 3 × probability 4 = 72, band schedule.
Why it matters: one transient storage fault can defeat the promised refusal
policy even after the physical connection becomes usable again.

## 257. A permission prompt omits the options its agent holder must choose

Triage (2026-11-23): open, P1, board and roles, high confidence. An agent asked to answer a permission request is not told the choices, so a team lead cannot answer it. Basis: routing/holders.ts:95 passes only description text; entry says partly fixed for project leads by commit 2616fb5 via project.requests.

Found: 2026-11-03, in the post-build review of commit `8ff4c9b`, finding A14.
State: partly fixed, because a project lead can read a request's options through the `project.requests` tool since commit `2616fb5`, while the holder's delivery text in `routing/holders.ts` still omits the options and a team lead holder still cannot read them. Verified against the code on 2026-11-16.
Earlier state: open, deferred by the owner on 2026-11-03.

Evidence: the post-build A14 adjudication is in
`.scratch/foundry-runs/engine-build-1/review-post-build/adj/result-1.md` (not tracked).
It adjudicates s14 F-1 by source inspection.
The related s14 F-3 ordering allegation is disproved on the success path.

What happens: `packages/engine/src/routing/permissions.ts` passes only the
description into the holder's delivery.
In `packages/engine/src/routing/holders.ts`, moved requests likewise omit
the options' identifiers, names and kinds.
The answer handler refuses an option identifier the request does not offer.
The tool list in `packages/mcp/src/tools.ts` offers no request-reading tool.
A direct parent can read the raw request event through a subscription, but
a holder farther up the hierarchy cannot see that child's event.
Decision row 55 also excludes subscriptions from the agent tools.

What should happen: include the options in both initial and moved deliveries,
with owner-only choices identified and adapter text enclosed and escaped.
A request-reading tool would instead require the owner's amendment to row 55.

Score: MAJOR, severity 4 × fallout 3 × probability 8 = 96, band schedule.
Why it matters: an ordinary agent-held permission request degrades into
owner intervention because the holder lacks the data needed to answer it.

## 258. Asynchronous live-check errors bypass the promised cleanup

Triage (2026-11-23): open, P3, tests, high confidence. A spawn or log-file error during the live check crashes it without cleanup and can leave processes running. Basis: packages/engine/scripts/live/service.ts has no on('error') listener on the child or the log stream.

Found: 2026-11-03, in the post-build review of commit `8ff4c9b`, finding A19.
State: open, deferred by the owner on 2026-11-03.

Evidence: the post-build A19 adjudication is in
`.scratch/foundry-runs/engine-build-1/review-post-build/adj/result-7.md` (not tracked).
It adjudicates s29 F-2 by source inspection,
hand reasoning and two dependency-free checks with synthetic error emitters.
The adjudicator's checks used no real service or adapter processes.

What happens: `packages/engine/scripts/live/service.ts` attaches neither
a child-process error listener nor a log-stream error listener.
An asynchronous spawn or log failure therefore throws outside the promise
awaited by `packages/engine/scripts/live/run.ts`.
The entry in `packages/engine/scripts/live-check.ts` catches only that
promise's rejection.
In the adjudicator's injected checks, both errors terminated the wrapper
with an unhandled error event before the finally or promise markers ran.
A log failure after spawn can leave the service and detached adapters running.
A failed spawn alone does not prove a surviving service.

What should happen: attach both error handlers immediately and propagate
one lifecycle failure into journey cancellation, bounded cleanup and the
named final failure summary.
Preserve diagnostics and handle errors during construction or shutdown too.

Score: MAJOR, severity 6 × fallout 3 × probability 4 = 72, band schedule.
Why it matters: the owner may need to stop leftover work by hand after a
local filesystem or process-creation fault.

## 259. A dropped permission event leaves an adapter hold with no answerable request

Triage (2026-11-23): open, P2, engine, high confidence. If saving a permission request fails, the agent stays blocked on a request that nobody can see or answer. Basis: commands/CommandHandler.ts:187-189 still only logs 'runner event dropped: processing failed'; no cancel of the adapter hold.

Found: 2026-11-03, in the post-build review of commit `8ff4c9b`, finding flow2 F-6.
State: open, deferred by the owner on 2026-11-03.

Evidence: finding F-6 is in
`.scratch/foundry-runs/engine-build-1/review-post-build/results/flow2.md` (not tracked).
It confirms the path by source inspection.

What happens: a full disk or lock timeout can fail the transaction that
records `permissionRequested`.
In `packages/engine/src/commands/CommandHandler.ts`, the runner-event catch
logs "runner event dropped: processing failed" and continues.
In `packages/engine/src/runner/AcpRunner.ts`, the permission hold stays pending.
There is no durable request row for an owner or agent to answer.
The agent remains working until someone stops it.
This differs from post-build A1, where a recorded request is stranded at its holder.

What should happen: cancel or reject the adapter hold when its permission
request cannot be recorded.

Score: MINOR, severity 6 × fallout 3 × probability 4 = 72, band schedule.
Why it matters: an agent can stay blocked on a request that no client can see.

## 261. Account replacement drops omitted fields, and removal ignores running agents

Triage (2026-11-23): waits for the owner's decision, P2, engine. Changing an account silently drops fields left out, and an account can be removed while agents still use it. Basis: commands/accounts.ts: set writes the supplied account whole; remove case has no running-agent check. Entry says owner has not chosen the policy.

Owner ruling (2026-11-25): The recommendation to refuse the removal is not taken. An account may be removed while agents use it. A dialog first states that the active agents on it will stop. The dialog links to a list of all agents, filtered to that account. Because the server and the client are being rewritten in Go, this ruling is an input to that design.

Found: 2026-11-03, in the post-build review of commit `8ff4c9b`, finding s13 F-3.
State: open, deferred by the owner on 2026-11-03.

Evidence: `.scratch/foundry-runs/engine-build-1/review-post-build/results/lgtm-engine-s13.md` (not tracked), F-3, confirms the path by source
inspection and hand reasoning.

What happens: `accounts set` in `packages/engine/src/commands/accounts.ts`
writes exactly the supplied account into `AccountChanged`.
In `packages/engine/src/views/apply.ts`, that event replaces the whole record.
Omitting `environment` or `home` therefore drops the old values and references.
Superseded secret values remain in `packages/engine/src/accounts/SecretStore.ts`
until restart collection.
Reusing a secret's name through `set-secret` can also leave the old value stored.
The remove handler checks no running agent's account usage.
Those agents can therefore reference an account that no longer exists.
The review did not establish the later resume outcome.

What should happen: document full replacement or define field merging.
Refuse removal while agents still use the account and collect superseded values.
The owner has not chosen the replacement policy.

Score: MINOR, severity 4 × fallout 3 × probability 6 = 72, band schedule.
Why it matters: an ordinary settings change can surprise the owner and
require setting the account fields or secrets again.

## 264. The orphan-sweep test for a surviving group times out before its own deadline

Triage (2026-11-23): open, P2, tests, medium confidence. A test for a process group that survives a kill may time out before the sweep it waits for can finish. Basis: test/recovery/orphans.test.ts F-3 still has no per-test timeout or killWaitMillis; orphans.ts default 5_000; entry verified 2026-11-16.

Found: 2026-11-03, by review-fix-2 while verifying the post-build A3 fix.
State: open, because the F-3 case still waits for the five-second default sweep bound with no per-test timeout, and the root 60-second test timeout likely does not reach the inline Vitest projects. Verified against the code on 2026-11-16.
Earlier state: open, recorded at the review lead's request in board post 8.

Evidence: the fixer's result is in
`.scratch/foundry-runs/engine-build-1/review-post-build/fix-round-1/result-2.md` (not tracked).
It reports a five-second test timeout in
`packages/engine/test/recovery/orphans.test.ts`, case F-3.
The case is named "a group that survives SIGKILL".
The full engine run on branch `worker/review-2`, commit `c9ff320`, had
1,627 passing tests, one failing test and two skipped tests.
Review-fix-2 reproduced the same failure on the untouched base commit
`8ff4c9b` in a temporary worktree, which was then removed.
I did not reproduce the runtime failure in this worktree.

What happens: the case keeps one fake process group alive after both signals.
It allows the sweep eight seconds to return through `Effect.timeoutOption`.
In `packages/engine/src/recovery/orphans.ts`, the default wait after the
kill signal is five seconds, after the case's 30-millisecond grace period.
The reported five-second test timeout therefore expires before that expected
sweep can complete.
The root `vitest.config.ts` declares a 60-second test timeout.
Its effective timeout for this workspace case still needs verification.
The report does not establish a production sweep failure or a post-build A3 regression.

What should happen: make the case's effective test deadline cover the
deliberate sweep bound, or inject a shorter sweep wait without weakening
the assertion that a surviving group is left alone and the next row is swept.
No test or production implementation is changed by this record.

Score: not assigned, because the new evidence establishes a test failure
rather than a scored production defect.
Why it matters: a failure already present on the base commit prevents a
clean full-suite result for unrelated engine changes.

## 265. A stalled prompt send holds the lane that a stop command needs

Triage (2026-11-23): open, P1, engine, medium confidence. If an agent's adapter stops reading a large prompt, a stop command waits forever behind it. Basis: No send deadline found in runner/AcpRunner.ts prompt path (only stderr and stop timeouts); entry says A3 fix does not cover it.

Found: 2026-11-03, in the post-build review of commit `8ff4c9b`, finding flow3 F-5.
State: fixed in unit f2-lifecycle. A send now registers in the agent's pre-emption record around its runner call, so a stop the caller could commit signals it in any stoppable state, and the send stops waiting and frees the lane while the runner call goes on without it; `packages/engine/test/commands/lifecycle-races.test.ts` proves it. No send deadline or partial-write policy was added, because those remain the owner's choices.

Evidence: finding F-5 is in
`.scratch/foundry-runs/engine-build-1/review-post-build/results/flow3.md` (not tracked).
It confirms the path by source inspection
and hand reasoning, without executing a stalled-send test.
The post-build A6 adjudication in
`.scratch/foundry-runs/engine-build-1/review-post-build/adj/result-5.md` (not tracked)
keeps this stop-control consequence separate from the cross-agent recovery
scheduling defect.
Issue 253 records that recovery scheduling defect.

What happens: an adapter can stop reading while a large prompt fills its
standard-input pipe.
In `packages/engine/src/routing/deliveries.ts`, dispatch holds the agent's
lane through the runner's prompt call.
In `packages/engine/src/runner/AcpRunner.ts`, that call waits for the write
acknowledgement without a send deadline.
In `packages/engine/src/commands/CommandHandler.ts`, a stop waits for the
same lane before it can stop the process.
The stop therefore waits for the adapter to exit or for an external kill.

The post-build A3 fix on `worker/review-2`, commit `c9ff320`, does not cover this path.
Its `packages/engine/src/routing/lanes.ts` cancellation registration is used
only by start setup, as the fixer's result confirms.
An ordinary stalled send registers no such cancellation signal.

What should happen: bound or cancel the send acknowledgement, or let an
authorized stop pre-empt the send before acquiring its lane.
The original flow score is severity 6 × fallout 3 × probability 2 = 36, band record.
The flow reviewer labelled it MINOR.
The post-build A6 adjudicator considers indefinitely failing stop control a MAJOR
consequence, while keeping this finding separate from post-build A6.
Why it matters: an ordinary stop can fail to end the affected agent until
the owner intervenes outside the command path.

## 267. A cancelled start's fallback cleanup races with a later stop or resume

Triage (2026-11-23): open, P1, engine, high confidence. Cleanup from a cancelled agent start can run late and break a later successful stop or fresh resume of that agent. Basis: commands/start.ts: after handle.settled, state is read outside the lane and failStart revokes all tokens by agentId with no incarnation check.

Found: 2026-11-03, in the re-review of the post-build A3 correction at `8596f63`.
State: fixed in unit f2-lifecycle. The cancelled start's fallback now takes the agent's lane again and acts only while the agent is still starting and the start's own token is unrevoked, checked again inside the failure transaction; `packages/engine/test/commands/lifecycle-races.test.ts` proves it.

Evidence: finding F-1 is in
`.scratch/foundry-runs/engine-build-1/review-post-build/fix-round-1/rresult-2b.md` (not tracked).
The re-reviewer confirmed the path by source inspection and supplemental
integration probes with a delayed writer.
The probes observed a duplicate terminal event after a successful stop and
revocation of the new token after a successful resume.
They did not establish that the fallback killed a real operating-system process.

What happens: in the merged A3 implementation, a cancelled start in
`packages/engine/src/commands/start.ts` releases its agent's lane before
waiting for the stops that cancelled it to settle.
It then checks the lifecycle outside the writer transaction and runs fallback cleanup.
The cleanup stops a runner by agent identifier and unconditionally appends
`AgentStartFailed` while revoking every token for that agent.
It does not recheck the current state or original token incarnation inside
the writer transaction.
The settlement helper in `packages/engine/src/routing/lanes.ts` waits only
for a snapshot of the stops already registered.
A later valid stop can therefore commit `AgentStopped` before the old
fallback appends `AgentStartFailed`.
A subsequent `resume --fresh` in `packages/engine/src/commands/resume.ts`
can open a new incarnation whose token the old fallback then revokes.
The view can stay idle while its new token is unusable.
The runner stop in `packages/engine/src/runner/AcpRunner.ts` also selects
only by agent identifier, so stale cleanup could target a later process.
The review directly proved the event and token consequences, rather than that kill.

What should happen: keep fallback cleanup inside the agent's lane and
validate the current state and the original token inside the failure transaction.
Cleanup from one incarnation must not stop or revoke a later incarnation.
This remains part of the post-build A3 follow-up linked from issue 103.
No implementation change is made by this record.

Score: not assigned, because the re-review supplied no numerical risk score.
The re-review labels the finding MAJOR.
Why it matters: overlapping commands or a delayed writer can let cleanup
from a cancelled start corrupt a later successful stop or resume.

## 268. A cancelled start can outwait its settlement deadline at the event writer

Triage (2026-11-23): open, P2, engine, medium confidence. A cancelled start can wait without limit on the event writer, and cannot record its end after a real storage failure. Basis: commands/start.ts: failStart runs after handle.settled(SETUP_TIMEOUT_MILLIS) with no own bound; issue 103 says 268 remains open.

Found: 2026-11-03, in the re-review of the post-build A3 correction at `8596f63`.
State: open, recorded at the review lead's request in board post 23.

Evidence: finding F-2 is in
`.scratch/foundry-runs/engine-build-1/review-post-build/fix-round-1/rresult-2b.md` (not tracked).
The re-reviewer checked the source and ran supplemental integration probes
through the real writer boundary.
One probe held the writer beyond the settlement deadline.
Another injected `LogWriteFailed` inside the real transaction to exercise
the production refusal latch without inducing a physical disk fault.
These are the re-reviewer's observations, not runtime tests run in this worktree.

What happens: in the merged A3 implementation, the two-minute timeout in
`packages/engine/src/routing/lanes.ts` bounds only the wait for stop settlement.
The fallback append in `packages/engine/src/commands/start.ts` runs afterward.
If a stop still holds the writer indefinitely, the cancelled start can wait
indefinitely for that writer even after its settlement deadline has expired.
The timed probe progressed only after the writer was released.

A real writer failure has a different remaining limit.
In `packages/engine/src/log/EventLog.ts`, `LogWriteFailed` latches refusal
for the lifetime of the writer.
The fallback uses that same writer, so it cannot append its terminal event
or revoke the original token after the latch is set.
The probe observed an agent still starting with an unrevoked token and
no `AgentStartFailed`, although the runner stop was called.
The new writer-failure case in
`packages/engine/test/commands/start-setup.test.ts` instead injects a
one-shot failure outside the real writer.
Its following fallback transaction can succeed because that synthetic
failure never exercises the refusal latch.
It does not prove terminal completion under the production latched failure.

What should happen: qualify the completion guarantee on a usable writer
or define a separate recovery policy for failed persistence.
Distinguish the recoverable synthetic failure from the real latched failure
in the test evidence.
Do not bypass the storage-failure latch to force a terminal append.
The settlement deadline must not be described as a bound on later writer work.
This remains part of the post-build A3 follow-up linked from issue 103.
No implementation change is made by this record.

Score: not assigned, because the re-review supplied no numerical risk score.
The re-review labels the finding MINOR.
Why it matters: the claimed terminal-outcome guarantee and its time bound
are stronger than the persistence path and the test establish.

## 269. An account home that equals a stored secret value is redacted, so the account event fails its schema and later account commands are refused

Triage (2026-11-23): open, P2, security, medium confidence. An account whose login folder equals a stored secret value fails to save and can block later account commands. Basis: Entry: residual after last fix round, PR #11 left it open; no home entry found in log/redactionStructure.ts. Not reproduced.

Found: 2026-11-03, in the re-review of the post-build A9 fix at `1f2602a`.
State: open, residual after the last fix round; the code reviewed at
`1f2602a` is merged in pull request #8.

Evidence: finding F-1 is in
`.scratch/foundry-runs/engine-build-1/review-post-build/fix-round-1/rresult-1b.md` (not tracked).
The re-reviewer reproduced the failure through the real service with a
Claude account and a planted login-folder value.
The same path applies to Codex by source inspection.
These are the re-reviewer's observations, not runtime tests run in this worktree.

What happens: add or set a Claude or Codex account whose home equals the
value stored under `CLAUDE_CONFIG_DIR` or `CODEX_HOME`.
The value can be stored through `accounts set-secret` or supplied through
a `secret:` reference.
The redactor registers that stored value in `packages/engine/src/service.ts`.
`packages/engine/src/log/EventLog.ts:250` redacts the account event before
the schema check at `packages/engine/src/log/EventLog.ts:182`.
The redacted home no longer meets the absolute-path requirement at
`packages/contracts/src/accounts.ts:22`.
The command fails with a `SchemaError`.
The secret file has already been written at
`packages/engine/src/commands/accounts.ts:78` and cannot be rolled back
with the database transaction.
Receipt redaction at `packages/engine/src/log/EventLog.ts:221` also replaces
the home in the stored account result.
Receipt decoding at `packages/engine/src/commands/CommandHandler.ts:252`
then refuses `accounts list` on a fresh connection.
These source line numbers are from the reviewed commit `1f2602a`, and the files have changed since.

The command tests use a pass-through redactor at
`packages/engine/test/support/handlerRig.ts:117`,
`packages/engine/test/commands/account-login.test.ts:120`, and
`packages/engine/test/commands/account-login.test.ts:142`.
They therefore do not exercise the real service's redaction boundary.
The fix now accepts the matching input instead of refusing it.
The matching-folder acceptance rule in row A9 of
`.scratch/foundry-runs/engine-build-1/review-post-build/owner-choices.md` (not tracked)
remains unmet through the real service.

What should happen: use an end-to-end representation that accepts the
matching setting while preserving secret removal.
Do not exempt secrets from redaction.
The registry in `packages/engine/src/service.ts` is also involved in
owner finding B1, recorded in issue 223.
Post-build A4 was fixed in pull request #11 and has no issue entry.
The A4 fix leaves this failure open: `home` is still free text, so it is still redacted
(`rresult-4.md`, question 4, in the same run folder).

Score: not assigned, because this is a residual after the last fix round.
The re-review labels the finding MAJOR.
Why it matters: a matching login-folder configuration can fail after
persisting its secret and leave later account commands refused.

## 272. Codex's ask mode can refuse a write without requesting permission

Triage (2026-11-23): open, P2, harnesses and agent running, high confidence. In ask mode, Codex can simply refuse to write a file instead of asking the owner for permission. Basis: runner/harnesses.ts:56 still maps ask to Codex read-only mode; entry says probe changes do not fix it.

Found: 2026-11-03, in live-check run 4 at commit
`f94dd9996a93cf658ddf3ee52cdbbf7fb5fc1ab9`.
Status: open, recorded; the engine is not changed in fix round 8.

`packages/engine/src/runner/harnesses.ts:60` maps LGTM Studio's `ask` level to
Codex's `read-only` mode.
In `@agentclientprotocol/codex-acp` version 2.5.0, the installed source at
`dist/index.js:40424-40449` gives that mode an `on-request` approval policy
and a read-only sandbox.
The adapter passes the selected mode's approval policy into each turn at
`dist/index.js:42079`.
The policy relies on the model choosing to request approval for a write.
It does not require a permission question before every attempted change.

The lead's event-log inspection reports that run 3 requested approval for
a Python write at event 81, and the live-view criterion passed.
In run 4, event 78 attempted `touch hello.txt`.
Event 80 reported a read-only file-system refusal and exit code 1.
The worker then said it could not create the file because the workspace
was read-only, and event 98 ended its turn without a PermissionRequested.
The person sees a worker that could not write and no permission question.
The failure direction is safe: the sandbox refuses the write.
The original live-view step then waited 300 seconds for a request that
could no longer arrive from that turn.
These observations come from the lead's round-8 task and the run-4 log at
`.scratch/foundry-runs/engine-build-1/live-check-2/live-check-run4.log`.
They are not a new real-adapter run by this worker.

One option to weigh is an approval policy that always asks before a write,
if the adapter can expose that behavior while preserving the sandbox.
The four session presets exposed by version 2.5.0 at
`dist/index.js:40424-40494` and `dist/index.js:40528-40547` offer read-only,
workspace-write, agent, and full-access modes.
Their approval policies are `on-request` or `never`.
None is an always-ask preset.
A future adapter integration change would need separate verification.
Widening the sandbox merely to obtain a passing probe would not establish
ask-before-write behavior.

Fix round 8 changes the live-check prompt to request approval explicitly,
including one retry after a read-only refusal, with an honest failure path.
It makes live-view fail as soon as a worker's turn ends without a request,
and continues collecting independent probe evidence after journey failures.
It still requires a real permission request and a preceding thread item
on the second connection to pass live-view.
These probe changes do not fix the engine's ask-mode semantics or close
this issue.

## 273. Three limits remain in the bounded terminal output

Triage (2026-11-23): open, P2, harnesses and agent running, medium confidence. Terminal output can show a silent gap, grow with mixed status reports, and lag until the next update. Basis: Entry: recorded, not fixed, three minor limits; no later fix text. Code not re-checked in detail.

Found: 2026-11-03, in the verification of the round-7 terminal fix.
Status: open, recorded, not fixed.
The review is `.scratch/foundry-runs/engine-build-1/reviews/lcp-242-verify.md`.
It rated each limit minor, and it found no redaction leak.

The first limit is a silent gap during a coverage loss.
Terminal updates that arrive while the login store cannot be read are discarded,
and the terminal map is cleared.
That interval leaves no marker in the thread.
If coverage returns before the turn ends, the rest of that command's output is
absent, and nothing says so.
The failure is in the safe direction, because nothing raw is stored.

The second limit is the bound on version 1 reports.
A version 1 delta report that carries any other field, such as `status`, is not
skipped as a data-only report.
In the review's run, 2,000 such reports stored 2,008 items and 559 KB.
That is 57 times smaller than before the fix, but the bound is the report count.
The regression in `terminal-output-round-7.test.ts` uses delta-only reports,
so it does not show this case.

The third limit is the lack of a trailing publish.
A burst of output inside the 500-millisecond window stays unpublished until the
next update, the command's exit or the end of the turn.
No output is lost, but the view can lag for as long as a command stays quiet.
One suggested fix is one delayed publish for each terminal with unpublished output.

## 274. A terminal chunk containing a secret from the launch is dropped without a marker

Triage (2026-11-23): open, P2, harnesses and agent running, low confidence. Terminal output that contains a launch secret disappears entirely instead of appearing as a redacted line. Basis: Entry: recorded, not fixed, cause only inferred; no later fix text found.

Found: 2026-11-03, in the verification of the round-7 terminal fix.
Status: open, recorded, not fixed. The fault is older than the fix.
The review saw the same result at `34af123`.
It ran 12 times, and the chunk never reached `packages/engine/src/runner/terminal.ts`.

A version 2 terminal chunk that contains a secret passed to the adapter at launch,
such as a login token, is lost whole.
The same text from a login store is redacted and shown.
A person who runs `env | grep TOKEN` therefore sees nothing, not a redacted line.

The review infers the cause, and it did not find the exact line.
The redactor matches base64 runs of a secret
(`packages/engine/src/identity/redactor.ts:25-58`), and a version 2 chunk's data
is base64, so a match on the whole frame may discard the chunk.
The failure is in the safe direction, because no secret is stored.
The gap is that the thread gives no sign that output was withheld.

## 280. A short explicit secret equal to a schema word makes a terminal snapshot fail to decode and drops the terminal item

Triage (2026-11-23): open, P2, engine, medium confidence. A secret that matches a short internal word can make an agent's terminal output vanish from the thread. Basis: AcpRunner.ts:375-378 still fails 'adapter terminal content is malformed' when decode fails while covered; only coverage loss yields a withheld item.

Found: 2026-11-03, in the post-build re-baseline at `ce50c50` (the merge of
pull request #13).
State: open, recorded, not fixed. The fix round was at its two-round cap.
The review lead scored it 36, which is the record band.
The re-reviewer's scoring was adopted: severity 6, because a core feature
stays wrong until the configuration is corrected by hand; fallout 3, because
the owner can change the conflicting configuration and restart the affected
work without support; and probability 2, because the conflicting credential
configuration is unusual but needs no privileged access.

Evidence: finding F-1 is in
`.scratch/foundry-runs/engine-build-1/review-post-build/fix-round-1/rresult-6.md` (not tracked).
The re-reviewer reproduced it with an in-memory probe that used the real
terminal output builder, the `cleanStructure` and `publishTerminals` bodies
copied unchanged from `ce50c50`, and the real `threadContent` schema.
The review lead read the same code and confirmed the path.
The re-reviewer also ran nine targeted test files, of which 38 tests passed and
35 failed.
The child-process tests were run, but adapter initialization failures prevented
validation of the full live path.

What happens: `publishTerminals` in `packages/engine/src/runner/AcpRunner.ts`
passes the generated terminal content through `cleanStructure`.
That helper redacts every object key and every string leaf, including the
`_tag` key and its value `tool-call`.
It therefore bypasses the structure protection that pull request #11 added to
the event log.
If a registered secret collides with part of a required key, discriminator or
enumerated value, the content no longer decodes as `threadContent`.
The redactor matches occurrences within strings, so the secret does not need
to equal the whole word.
The demonstrated examples are the explicit secrets `_tag`, `tool-call` and `t`.
The eight-character inferred values `ool-call` and `tool-cal` also corrupt the
`tool-call` discriminator and fail in the same way.
The decode failure becomes the rejection "adapter terminal content is
malformed".
The update handler catches that rejection and drops the terminal item.
A drain at the end of a turn can propagate the same rejection to its caller.
With an unrelated secret the same terminal publishes one item.
No secret leaks, because the failure closes the path.
The cost is lost terminal output, and possibly a turn that does not reply.

Why the probability is not higher: a registered secret has to collide with part
of the schema vocabulary.
An inferred value also needs an environment variable whose name matches the
service's secret-name pattern, and a value that passes the length and
scalar exclusions in `packages/engine/src/identity/inferredSecret.ts`.

The fix: redact the outer tool-call structure without rewriting its keys or
its discriminator.
Keep redacting the call identifier, the title, the signal and the whole
serialized output as text, including any `_tag` inside that string.
Keep the coverage-loss latch.
If redaction makes another required field invalid, publish a fixed withheld
item instead of dropping the terminal or failing the drain.
Add a runner-to-log test for the short explicit-secret collisions.
The existing terminal tests use a long secret, and the existing structure
tests exercise the event-log redactor without this publication step.

## 300. `OwnerNeeded.reason` is free text with three producers

Triage (2026-11-23): open, P2, client api and command line, high confidence. The reason on an owner-needed alert is free text, so a wording change could silently break client alert handling. Basis: contracts/src/events.ts:221-223: OwnerNeeded reason is still Schema.String; no literal list found.

Found: 2026-11-03, while planning the client API plan.
State: open.

What happens: `OwnerNeeded.reason` is free text, written in three places: `routing/holders.ts:49`, `recovery/crash.ts:69` and `routing/deliveries.ts:194`. A client that maps reasons to alert kinds breaks without a sign if a reason string changes.
What should happen: The client API plan pins the reasons with a contract test.
Why it matters: A changed reason string would silently stop an owner alert from being raised.

## 303. The root `pnpm -r test` runs no tests, and the engine package has no `test` script

Triage (2026-11-23): open, P3, tests, medium confidence. The per-package test command runs nothing and passes, which gives false confidence. Basis: Root package.json test is 'vitest run'; packages/engine/package.json has no test script. Real gate is pnpm check.

Found: 2026-11-03, while planning the client API plan.
State: open.

What happens: The root `pnpm -r test` passes without checking anything (client API row 111). The engine package has only a `typecheck` script, so `pnpm --filter @lgtm/engine test` also runs nothing. Tests run only through the root `vitest run`.
What should happen: Not decided. The real gate is the root `pnpm check`.
Why it matters: A passing test command that checks nothing gives false confidence.

## 312. The `accounts` read and `AccountChanged` carry environment values as entered

Triage (2026-11-23): open, P2, security, medium confidence. A secret typed under an ordinary variable name in an account is written to the log and shown to clients. Basis: Entry open, 'Not decided'; no later fix text. Inferred-secret handling exists but plain names are not covered per entry.

Found: 2026-11-03, while planning the client API plan.
State: open.

What happens: A plain value under a name that does not look like a secret reaches every client and the log (`contracts/src/accounts.ts:9-16`, client API row 141).
What should happen: Not decided.
Why it matters: A secret stored under an ordinary name would be written to the log and shown to clients.

## 314. A failing alert adds one event every 10 minutes

Triage (2026-11-23): open, P3, engine, medium confidence. A phone alert that keeps failing adds one event to the log every ten minutes for as long as the outage lasts. Basis: Entry open, 'Not decided'; no later fix text found.

Found: 2026-11-03, while planning the client API plan.
State: open.

What happens: An alert that keeps failing adds one small event every 10 minutes while the carrier is down (client API row 131).
What should happen: Not decided.
Why it matters: A long outage grows the event log without end.

## 315. The number of agents in `client.snapshot` is not capped

Triage (2026-11-23): open, P3, client api and command line, medium confidence. Stopped agents accumulate in the client snapshot without a cap, so it could outgrow its size budget. Basis: Entry open, 'Not decided'; no cap or pruning found under packages/engine/src/api.

Found: 2026-11-03, while planning the client API plan.
State: open.

What happens: Stopped agents can grow the snapshot, and only a fixture test bounds the whole (client API row 137).
What should happen: Not decided.
Why it matters: A long-lived installation could exceed the snapshot's size budget.

## 317. `SecretStore.resolve` cannot tell a missing value from a failed read

Triage (2026-11-23): open, P3, engine, high confidence. The secret store reports a failed read the same way as a missing secret, hiding the real cause. Basis: accounts/SecretStore.ts:75-81: resolve ends with catchCause returning undefined.

Found: 2026-11-03, while planning the client API plan.
State: open.

What happens: `accounts/SecretStore.ts:66-72`: `resolve` returns nothing both for a missing value and for a failed read. The carrier treats both as a failed attempt (client API row 168).
What should happen: Not decided.
Why it matters: A caller cannot report the real cause of a failure.

## 318. `project` gives a piece's reducer placeholder envelopes

Triage (2026-11-23): open, P3, engine, high confidence. Internal projection hands pieces placeholder event details, which would mislead any piece that reads more than the event. Basis: routing/waits.ts project still builds envelopes with eventId 'projected', occurredAt 0 and caller owner.

Found: 2026-11-03, while planning the client API plan.
State: open.

What happens: `routing/waits.ts:28-31`: `project` gives a piece's reducer placeholder envelopes, with the caller owner and time zero. A reducer now reads only the event and the sequence (client API row 161).
What should happen: Not decided.
Why it matters: A reducer that reads more than the event and the sequence would see false values.

## 319. A very large reply cannot reach a very slow link

Triage (2026-11-23): open, P3, client api and command line, medium confidence. A very large reply cannot be read over a very slow connection because heartbeats are missed. Basis: Entry open, 'Not decided'; no later fix text.

Found: 2026-11-03, while planning the client API plan.
State: open.

What happens: A reply of about 3 mebibytes cannot reach a link of about 500 kilobits per second, because the link misses two heartbeats on the read connection (client API row 160).
What should happen: Not decided.
Why it matters: A client on a very slow link cannot read a very large reply.

## 320. Another local user can hold the web server's loopback port while it is down

Triage (2026-11-23): open, P3, security, medium confidence. While the web server is down, another local user could take its port and show a fake page. Basis: Entry open; only the owner being told is decided. No later fix text.

Found: 2026-11-03, while planning the client API plan.
State: open.

What happens: While the web server is down, another local user can hold its loopback port and serve a long-cached page (client API row 167).
What should happen: The owner is told. Not otherwise decided.
Why it matters: A browser could be shown a page that did not come from LGTM Studio.

## 321. An agent that runs `lgtm client subscribe` reads events with no data envelope

Triage (2026-11-23): open, P2, security, medium confidence. Events an agent reads through the subscribe command are not marked as data, so they could be taken as instructions. Basis: Entry open, 'Not decided'; packages/cli subscribe parsing shows no envelope wrapping. Not traced fully.

Found: 2026-11-03, while planning the client API plan.
State: open.

What happens: This is how the engine's `subscribe` works today (client API row 166).
What should happen: Not decided.
Why it matters: Event text read by an agent is not marked as data, so it could be read as an instruction.

## 323. A Herdr pane starts from Herdr's environment, not the engine's

Triage (2026-11-23): open, P2, security, medium confidence. An agent in a Herdr pane could read the owner's web password from Herdr's environment. Basis: Only engine/src/service.ts and bin/lgtm-dev clear LGTM_WEB_PASSWORD; no clearing found in plugins/. Interactive mode is still being built.

Found: 2026-11-03, while planning the client API plan.
State: open.

What happens: A Herdr pane's command starts from Herdr's environment and not the engine's, so `LGTM_WEB_PASSWORD` could reach it (client API row 188, TM-PB-6).
What should happen: The interactive mode plan clears `LGTM_WEB_PASSWORD` in a pane's command.
Why it matters: An agent in a pane could read the owner's web password.

## 324. The engine design still names paired devices and device credentials

Triage (2026-11-23): open, P3, documents, high confidence. The engine design document still describes device pairing and per-device credentials that were dropped. Basis: docs/specs/2026-11-01-engine-design.md lines 32, 83, 226, 471, 479, 488-490 still name pairing and device credentials.

Found: 2026-11-03, while planning the client API plan.
State: open.

What happens: `docs/specs/2026-11-01-engine-design.md` still names paired devices and device credentials.
What should happen: Client API rows 184 and 189 replace those parts, so the design's text should be brought up to date.
Why it matters: A reader of the design would build a pairing that was dropped.

## 326. Board posts cannot be deleted or purged

Triage (2026-11-23): waits for the owner's decision, P2, board and roles. A board post cannot be deleted, so a secret posted by mistake stays in the log and receipts. Basis: Entry open: 'A purge path is not planned', 'Not decided'.

Owner ruling (2026-11-25): A purge of a board post is recorded as a feature and is not built now. The owner added a direction for secrets: a vault that the server holds. An agent's tool call goes through the client to the server, and the server adds the secret. An agent can use a tool that needs a key without ever reading the key. Because the server and the client are being rewritten in Go, this ruling is an input to that design.

Found: 2026-11-03, while planning the message board plan.
State: open.

What happens: A post's text stays in `board.PostWritten`, in every wake's `DeliveryQueued`, in the engine's `PromptSent`, in client caches and in harness session files. Command receipts copy every committed envelope, including post text and wake bodies. The log is append-only. A purge path is not planned (board review round 1, threat m3, and round 2, R2-TM-4).
What should happen: Not decided. Any purge path must also cover the receipts table.
Why it matters: A secret posted by mistake cannot be removed.

## 327. No rule makes every piece size its events against the listener's limit

Triage (2026-11-23): open, P3, board and roles, medium confidence. Nothing forces every piece to keep its events under the network message size limit. Basis: Entry open, 'Not decided'; no later fix text.

Found: 2026-11-03, while planning the message board plan.
State: open.

What happens: The network listener's 1 mebibyte message limit bounds piece events and read pages. The board caps a wake body at 256 kibibytes and a read page at 512 kibibytes, but nothing makes every piece do the same (board review round 1, threat M5).
What should happen: Not decided.
Why it matters: A piece event over the limit could not be sent.

## 329. The engine checks the permission ceiling at `start` but not at `prompt`

Triage (2026-11-23): open, P2, security, medium confidence. The permission ceiling is checked when an agent starts but not when it is prompted, unlike the design says. Basis: commands/prompt.ts has no ceiling or canUseMode reference; start.ts imports canUseMode from identity/ceiling.ts.

Found: 2026-11-03, while planning the message board plan.
State: open.

What happens: The engine's design states the rule for `prompt` too (`commands/prompt.ts`, `identity/ceiling.ts`) (board review round 2, logic m7).
What should happen: The ceiling is checked at `prompt` as the design states.
Why it matters: A prompt could ask for more than the ceiling allows.

## 330. The agents view keeps only an agent's current mode

Triage (2026-11-23): open, P3, board and roles, medium confidence. The agents view forgets an agent's starting permission ceiling once its mode changes. Basis: Entry open, 'Not decided'; no later fix text.

Found: 2026-11-03, while planning the message board plan.
State: open.

What happens: `ModeChanged` overwrites the start ceiling, which survives only in `AgentStarted`. A piece that needs the ceiling must keep it itself (board review round 2, threat S2).
What should happen: Not decided.
Why it matters: Each piece that needs the ceiling must repeat the bookkeeping.

## 331. The board's view keeps every post's text in memory

Triage (2026-11-23): open, P2, board and roles, medium confidence. The service keeps every board post's text in memory, so memory grows with history. Basis: Entry open, 'Not decided'; posts still cannot be deleted (issue 326).

Found: 2026-11-03, while planning the message board plan.
State: open.

What happens: Posts cannot be deleted, so the service's memory grows with board history (board review round 2, threat S1).
What should happen: Not decided.
Why it matters: A long-running service uses more memory without bound.

## 332. Recovery leaves an interrupted agent with a session that cannot reload, and writes no event

Triage (2026-11-23): open, P2, engine, medium confidence. After a restart, an interrupted agent whose session cannot reload stays interrupted until the owner asks for a fresh session. Basis: Entry state: partly fixed in unit s21-recover; automatic recovery and the board-disposition part remain open.

Found: 2026-11-03, while planning the message board plan.
State: partly fixed in unit s21-recover. The website can request a fresh session after a refused reload. Automatic recovery still leaves that agent interrupted, and the unrelated board-disposition description below remains open.

What happens: A follower of the board must apply the `waits.ts` test itself (board review round 2, R2-A3).
What should happen: Not decided.
Why it matters: Each follower may judge the same agent differently.

## 336. A refused duty is retried after every later batch that touches it

Triage (2026-11-23): waits for the owner's decision, P3, board and roles. A board duty the engine keeps refusing is retried after each later batch, adding a refusal event each time. Basis: Entry state: open and awaiting the owner's decision, verified against the code on 2026-11-16.

Owner ruling (2026-11-25): Three refusals are not enough as the only measure. The retries back off. After three refusals a background agent of a new kind, an investigation agent, is started. It is a short, quick job on the account with the lowest usage. It finds out what happened and delivers the result to the project lead, so that the project lead's own context is not filled. Because the server and the client are being rewritten in Go, this ruling is an input to that design.

Found: 2026-11-03, while planning the message board plan.
State: open and awaiting the owner's decision, because the board reactor still retries a refused duty after each later batch that touches it, and only the refusal events themselves are ignored. Verified against the code on 2026-11-16.
Earlier state: open.

What happens: With the board's reactor written as level rules, a duty the engine keeps refusing is retried after each later batch that touches it, and each retry writes one `CommandRefused`.
What should happen: Not decided.
Why it matters: A defect that makes a reactor batch always fail would grow the log by one event per touching batch.

## 340. A piece's ids are treated as text by the redactor unless the piece adds table entries

Triage (2026-11-23): open, P3, engine, medium confidence. Nothing stops a future piece from forgetting its redaction table entries, which would redact its identifiers as text. Basis: Entry state: partly fixed, board and project entries added, no guard for future pieces; verified 2026-11-16.

Found: 2026-11-03, while planning the message board plan.
State: partly fixed, because the board and project pieces added their redaction table entries, but no check stops a future piece from forgetting its entries. Verified against the code on 2026-11-16.
Earlier state: open.

What happens: The engine's redactor at `9e386b0` has fixed structure tables (`log/redactionStructure.ts`) that name each id field. The client API contract v12 has no edit or table for a piece's ids.
What should happen: The board adds its entries as B13. The roles and client API plans must do the same.
Why it matters: A piece that forgets gets its ids redacted as text.

The roles persistence proof was strengthened on 2026-11-04 for foundation review
finding FR-067. A nonempty collision batch now passes through the real redactor
and event writer. The test checks Structured Query Language (SQL) event rows,
receipt envelopes, agent and board joins, read marking, and rebuilt equality.
Free text remains redacted. A temporary roles-only writer mutation is rejected
by this test while the older isolated and empty-event checks still pass.
This closes the roles proof gap without claiming first-use execution.

## 343. A standing rule answered from outside the engine arrives after the request was delivered

Triage (2026-11-23): open, P3, board and roles, medium confidence. A standing permission rule answered outside the engine still costs the holder agent a full wake. Basis: Entry open; AGENTS.md says standing permission rules remain unbuilt pending the owner's harness recordings.

Found: 2026-11-03, while planning the foundry roles plan.
State: open.

What happens: The request has already been delivered to an idle holder, so the holder still pays a wake.
What should happen: Decision row 11, WS-7a, answers it inside the engine.
Why it matters: Each such request costs a full agent turn.

## 346. The board marks a capacity-returned wake undeliverable within seconds

Triage (2026-11-23): open, P2, board and roles, high confidence. During a model-capacity outage the board can still mark a wake undeliverable after two returns. Basis: board/reactor.ts:79 marks undeliverable at returns >= 2; entry partly fixed in f46d958, verified 2026-11-16.

Found: 2026-11-03, while planning the foundry roles plan.
State: partly fixed in commit f46d958, because the engine now holds capacity errors, but the board still marks a wake undeliverable after two returns once a hold is refused. Verified against the code on 2026-11-16.
Earlier state: open.

What happens: The board retries a returned wake once at once and then marks the addressee `undeliverable`, so a model-capacity outage ends a board addressee's part of a post within seconds.
What should happen: The engine holds capacity errors (roles WS-7b). The board's planner should consider skipping the immediate retry for a capacity return.
Why it matters: A short outage permanently fails board wakes.

## 347. The recorded fixtures hold no tool call and no permission request

Triage (2026-11-23): waits for the owner's decision, P2, tests. The recorded harness fixtures hold no tool call or permission request, so permission matching cannot be tested on real traffic. Basis: No tool-call or permission lines grep-matched in packages/testing/fixtures/*.ndjson; AGENTS.md: rules wait for the owner's harness recordings.

Owner ruling (2026-11-25): Leave it open. A tool call and a permission request are recorded when the owner next uses those harnesses.

Found: 2026-11-03, while planning the foundry roles plan.
State: open.

What happens: `packages/testing/fixtures/claude.ndjson` and `codex.ndjson` hold neither, so no test can pin a harness's shell-permission shape from them.
What should happen: The roles plan adds a recording task (2g).
Why it matters: A permission matcher cannot be tested against real traffic.

## 348. The runner-event batch is appended with the caller `owner`

Triage (2026-11-23): open, P3, engine, medium confidence. Events from agents' runners are recorded as the owner's, so an automatic answer would look like the owner's own. Basis: commands/CommandHandler.ts:160 still appends the runner batch with caller owner.

Found: 2026-11-03, while planning the foundry roles plan.
State: open.

What happens: `packages/engine/src/commands/CommandHandler.ts:148` appends it as `owner`, and any failure in it drops the whole batch (lines 174 to 176). Anything an automatic answer writes there would be recorded as the owner's.
What should happen: Not decided.
Why it matters: The log would show an automatic answer as the owner's own.

## 349. `TurnStarted` is declared but never written

Triage (2026-11-23): open, P3, engine, high confidence. The turn-started event is declared and read by views but nothing ever writes it. Basis: No '_tag: "TurnStarted"' producer in packages/*/src; only schema, state machine and view readers.

Found: 2026-11-03, while planning the foundry roles plan.
State: open.

What happens: `TurnStarted` is declared in `packages/contracts/src/events.ts` (line 116) and nothing writes it.
What should happen: Not decided.
Why it matters: A reader that expects the event gets nothing.

## 350. A `ThreadItem`'s tool input has no stated maximum

Triage (2026-11-23): open, P3, engine, medium confidence. A single tool call's input has no size limit, so one call can make an oversized event. Basis: Entry open, 'Not decided'; no later fix text.

Found: 2026-11-03, while planning the foundry roles plan.
State: open.

What happens: A single huge tool call makes a large event.
What should happen: Not decided.
Why it matters: One tool call can exceed a message budget.

## 351. The ceiling's path resolver is synchronous

Triage (2026-11-23): open, P3, engine, high confidence. Permission path checks use blocking file calls, so a slow file system stalls every agent. Basis: identity/ceiling.ts still uses statSync, realpathSync, lstatSync, openSync and readSync.

Found: 2026-11-03, while planning the foundry roles plan.
State: open.

What happens: `packages/engine/src/identity/ceiling.ts` uses `lstatSync`, so resolution on a slow or network file system blocks the event loop for every agent.
What should happen: Not decided.
Why it matters: One slow file system stalls all agents.

## 352. One shared stream fiber carries every agent's runner events, and `answerPermission` is awaited inside it

Triage (2026-11-23): open, P2, engine, medium confidence. All agents' runner events pass through one sequential worker, so one slow agent can delay the others. Basis: Entry state: partly fixed, permission answers moved out, shared fiber and mode reset remain; verified 2026-11-16.

Found: 2026-11-03, while planning the foundry roles plan.
State: partly fixed, because permission answers now run outside the shared runner-event fiber, but that one sequential fiber still carries every agent's events and still awaits a mode reset. Verified against the code on 2026-11-16.
Earlier state: open.

What happens: `packages/engine/src/commands/CommandHandler.ts:177` awaits `answerPermission` inside the shared fiber, so one slow adapter answer stalls the runner events of all agents.
What should happen: Not decided.
Why it matters: One slow agent delays every other agent.

## 353. `isHarnessSettings` treats every path under a harness folder as a settings path

Triage (2026-11-23): waits for the owner's decision, P3, security. Every file under a harness folder counts as settings, so edits in worktrees under such folders all wait for the owner. Basis: identity/ceiling.ts:417-422 still matches any path segment .claude/.codex/.gemini/.cursor/.lgtm; narrowing relaxes a protection, so owner's decision.

Owner ruling (2026-11-25): The rule is narrowed to the real settings files. Because many harnesses must be supported, the settings files are named for each harness. How to keep that list for about thirty harnesses is open. Because the server and the client are being rewritten in Go, this ruling is an input to that design.

Found: 2026-11-03, while planning the foundry roles plan.
State: open.

What happens: `isHarnessSettings` at `packages/engine/src/identity/ceiling.ts:317-322` treats every path under a `.claude/`, `.codex/`, `.gemini/` or `.cursor/` folder as a settings path, without regard to case. Every file edit in a worktree under `.claude/worktrees/` goes to the owner (roles row 85).
What should happen: Narrowing the test to the settings files and folders inside a harness folder would fix it, but it relaxes a protection, so it is the owner's decision. Foundry worktrees move to `<repo>/.worktrees/`, which avoids the problem.
Why it matters: Every file edit by an agent in a Foundry worktree waits for the owner.

## 354. `git clean -ffdX` at a repository root would remove all of `.worktrees/`

Triage (2026-11-23): open, P2, git, medium confidence. One git clean command at the repository root would delete every worktree, including unmerged work. Basis: Entry open, 'Not decided. For the worktree plan'; no clean guard found in packages/engine/src.

Found: 2026-11-03, while planning the foundry roles plan.
State: open.

What happens: A dry run showed that `git clean -ffdX` at a repository root would remove all of `.worktrees/`, unmerged work included. The ignore line does not protect against it.
What should happen: Not decided. For the worktree plan.
Why it matters: Unmerged work in worktrees could be lost with one command.

## 355. A new worktree starts from the last commit, so uncommitted changes do not reach a child

Triage (2026-11-23): open, P3, git, medium confidence. A child's new worktree starts from the last commit, so a lead's uncommitted changes are missing. Basis: Entry open, 'Not decided. For the worktree plan'; this is normal Git behaviour.

Found: 2026-11-03, while planning the foundry roles plan.
State: open.

What happens: A lead's uncommitted changes do not reach a child's new worktree (tested).
What should happen: Not decided. For the worktree plan.
Why it matters: A child starts from older files than the lead sees.

## 356. The recorder fake-agent test failed at the base commit in one environment

Triage (2026-11-23): open, P3, tests, low confidence. A recorder test once failed at the base commit in one environment and the cause was never found. Basis: Entry: current focused execution passes, historical cause unknown, this part of FR-158 remains open.

Found: 2026-11-04, by the builder or test writer of unit r3p, during the foundation build.
State: open, for the final review.

What happens: The existing recorder fake-agent test failed during initialization at the base commit in this environment. The new tests use a synchronous transport double and do not repair that separate failure.
Why it matters: The foundation build's reviews are held until its end, so this waits for the final review.

The foundation review investigation retained the exact failing status assertion
from `/tmp/r3p-recorder-existing.log`. The archived result describes persistent
initialization failures in a run with 107 failing files. It does not describe
an unchanged failing-then-passing recorder case. The archived assertion omits
the returned failure categories. Current focused execution passes, but does
not establish the historical cause. This part of FR-158 remains open.

## 359. The contract names no largest alert event or alert-list page

Triage (2026-11-23): open, P3, client api and command line, medium confidence. The contract states no largest size for an alert event or an alert-list page. Basis: Entry open for the final review; no later text giving a maximum.

Found: 2026-11-04, by the builder or test writer of unit a2, during the foundation build.
State: open, for the final review.

What happens: The contract does not state a concrete largest client alert event or alert-list page. The size tests cover the board’s encoded wake, its whole-event page, and the stated future board and roles event payload budgets. Complete maximum-page fixtures must be supplied when those pieces install their contracts.
Why it matters: The foundation build's reviews are held until its end, so this waits for the final review.

## 360. Concurrent service startup timed out once in the merge gate

Triage (2026-11-23): open, P3, tests, low confidence. Starting two services at once timed out once in a merge gate and the cause was never found. Basis: Entry: FR-158 investigation could not establish the cause; 'This issue remains open'.

Found: 2026-11-04, by the builder or test writer of unit r1, during the foundation build.
State: open, for the final review.

What happens: The first full merge gate observed an intermittent concurrent-service startup timeout. The unchanged focused rerun and complete gate rerun both passed. The cause has not been established.
Why it matters: The foundation build's reviews are held until its end, so this waits for the final review.

The FR-158 investigation located the original timeout while waiting for the
survivor's socket in `/tmp/foundation-r1-merge-check.log`. The fixture discards
the child's standard error, so that log cannot distinguish an exited survivor
from a delayed listener. Bounded focused startup contests preserve the existing
exclusive-lock and survivor assertions. Passing contests do not establish the
historical cause. This issue remains open.

## 363. An agent named `owner` shares its reader records with the human owner

Triage (2026-11-23): open, P2, board and roles, high confidence. An agent given the name 'owner' would share its board read records with the human owner. Basis: contracts/src/ids.ts:4: AgentId is any non-empty string, with no reserved 'owner' value.

Found: 2026-11-04, by the builder or test writer of unit b2a, during the foundation build.
State: open, for the final review.

What happens: AgentId accepts the literal owner, but the required raw-agent-id-or-owner convention aliases that agent with the human owner in readState, dispositions, takenAt, and other reader-keyed records. A synthetic owner read advanced the position stored for an agent with id owner. The lead should settle whether to reserve that id or change the shared reader-key convention before building the consumers. This unit follows the frozen tests' stated convention and does not change contracts outside its scope.
Why it matters: The foundation build's reviews are held until its end, so this waits for the final review.

## 366. The transcript fallback schedules nothing for a long Claude project folder

Triage (2026-11-23): open, P3, harnesses and agent running, medium confidence. An agent in a very long Claude project folder path gets no cache timing from the transcript fallback. Basis: Entry open for the final review; deliberate refusal for keys over 200 characters, no later change recorded.

Found: 2026-11-04, by the builder or test writer of unit r9, during the foundation build.
State: open, for the final review.

What happens: The transcript fallback refuses a Claude project-folder key longer than 200 characters, because Claude Code hashes such keys in its own way. LGTM Studio then schedules nothing rather than guess a path. An agent in such a folder gets no cache timing from this fallback.
Why it matters: The foundation build's reviews are held until its end, so this waits for the final review.

## 469. The roles live check omitted a positive first-stall observation

Triage (2026-11-23): waits for the owner's decision, P2, tests. The roles live check has never observed a real first stall; the owner must run it with a real account. Basis: Entry state: open for the owner's real-account execution of pnpm live-check:roles.

Owner ruling (2026-11-25): Leave it open until one run of the roles live check with a real account.

Found: 2026-11-05, while repairing foundation review record FR-064.
State: open for the owner's real-account execution.

What happened: The live workflow rejected a false stall during collection,
but never observed a genuine idle obligation through its first production-minute
stall. The in-process acceptance test did not supply the retained child-process
proof required by post-build check 7.

What changed: The owner-run workflow now gives one worker an unanswered post,
waits for its production stall interval without changing the clock or limits,
and requires one stall action, one nudge wake and the worker's later reply.
Its private report retains the event identities and elapsed milliseconds.
The owner must run `pnpm live-check:roles` and retain the authentic report.
No real account or owner recording was used for this repair.

## 470. The roles live observer could count evidence outside the selected wave

Triage (2026-11-23): waits for the owner's decision, P2, tests. The repaired roles live observer still needs the owner's real-account rerun to close the proof gap. Basis: Entry state: open for the owner's real-account execution after observer repair.

Owner ruling (2026-11-25): Leave it open until the same run as issue 469.

Found: 2026-11-05, while repairing foundation review record FR-125.
State: open for the owner's real-account execution after observer repair.

What happened: Orientation evidence was not bound to the selected post or
channel. Aggregate answered closure and acknowledgments could coexist with
a terminal non-reply disposition instead of a reply from each worker.

What changed: Each exact worker must supply an authenticated orientation reply
on that wave and channel. Its replied disposition must name that reply as its
deciding post. Manual positive and negative controls exercise the observer.
The owner must rerun the real-account workflow before this proof gap closes.
The unit 14 ruling against a new required automated suite remains in force.

## 515. A designer critique found 15 problems in the mock that mislead or block the owner

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. A design critique found fifteen misleading or blocking problems in the mock; those fifteen were fixed. Basis: Commit 6133a318 'apply the designer's Must fix findings to the mock' exists; remaining findings are the owner's pick.

Found: 2026-11-06, in a critique by a subagent acting as a senior Apple designer.
State: the 15 Must fix findings are fixed in the mock, in commit `6133a31` on the branch
`area/web-mock`. The owner picks from the other findings. The full list of 114 findings is
`design/UI-CRITIQUE-2026-11-06-DESIGNER.md` on that branch.

What happened: The sidebar hides attention that exists, and failed agents wear the
success color. The merge to main is approved from a dialog with no evidence. A cancelled
permission says that an answer was recorded. A status line says "No owner action needed"
above a waiting request. Neither side of machine pairing shows the code, although the
owner decided that the website generates it.

Two of the problems contradict claims made by the review in entry 514. `design/DESIGN.md`
says that threads open at the latest message, and the lead panel opens 34 pixels short of
its end and returns to its top when reopened. The split diff added in that review filters
each side separately, so the lines of the two sides do not correspond. The 71 checks in
`design/revision-checks.mjs` did not catch either problem.

## 516. LGTM Studio cannot run Grok without Fast through Cursor's Agent Client Protocol

Triage (2026-11-23): open, P2, harnesses and agent running, medium confidence. Grok cannot be run through Cursor's protocol mode without its Fast setting, which the owner forbids. Basis: Entry open, not fixed; an upstream Cursor limit, no Grok handling found in engine or contracts source.

Found: 2026-11-06, while setting up the advisory board on a private engine.
State: open. Not fixed.

Cursor's Agent Client Protocol (ACP) mode offers Grok 4.7 only as
`grok-4.7[context=256k,reasoning_effort=high,fast=true]`. It refused the same
value with `fast=false`, and it refused `grok-4.7-high`. Launch arguments and a
private `CURSOR_CONFIG_DIR` did not change the offered value. The interactive
`cursor-agent --model grok-4.7-high` runs without Fast. The owner's rule is no
fast modes, so a Grok agent cannot run under LGTM Studio's engine today. The board
used Herdr Synapse's team board and interactive panes instead.

## 517. Moving the mock onto data files exposed four faults in it

Triage (2026-11-23): open, P2, website, medium confidence. In a long project list the sidebar does not scroll to the selected agent, so the selection can be out of sight. Basis: Entry state verified 2026-11-16: three of four fixed, sidebar open. No sidebar scrollIntoView found in design/mock/js. It lives only in a screen that is being rebuilt.

Found: 2026-11-07, while moving the mock's fixtures into JSON files and stub functions.
State: three of the four faults are fixed, and the sidebar still does not scroll to the selected agent in a long project list. Verified against the code on 2026-11-16.
Earlier state: three are fixed on the branch `area/web-mock`. The sidebar fault is open in the mock.

- A thread opened from Home started 39 pixels short of its end. The fix for this fault in
  `6133a31` restored the position before the page finished changing its layout. It is fixed.
- A question in the heavy sample did not appear in its agent's thread, so its links opened a
  thread without it. Questions now come from the events. It is fixed.
- Leads in the heavy sample showed the first-run greeting although they had sprints, and their
  team agents showed the recovery team's text.
  It is fixed.
- When the page opens an agent in a long project list, the sidebar does not scroll to that
  agent, so the selection can be out of sight. It is open.

A real export cannot show boards yet, because the engine stores no projects, sprints, or tasks.
The website todo records those events as proposed.

## 519. The Claude and Grok review findings are checked against `master`, and 20 remain

Triage (2026-11-23): open, P1, engine, medium confidence. Twenty confirmed review findings were still present, including a dropped permission request that leaves an agent waiting; none was rechecked since 2026-11-07. Basis: Entry state: open, checked at 95fcb14 only. No later text records fixes for R-008, R-009, R-012, R-016, R-023, L-038, L-119, L-132.

Found: 2026-11-07, while reconciling `master` before the freeze.
State: open. The check ran on 2026-11-07 at `95fcb14`, and the remaining findings wait until the freeze lifts.

Three foundation reviews ran on the snapshot at the tag `foundation-pre-review`, and only the Codex review's fixes reached `master`. Six Codex seats (GPT-6.1 Sol, high) then checked every confirmed and unresolved finding of the other two reviews against `master`. That was 56 Claude findings and 22 Grok findings, leaving out the three Grok security items, which belong to the owner's attack list.

| Review | Fixed | Partly fixed | Still present |
|---|---|---|---|
| Claude | 25 | 17 | 14 |
| Grok | 14 | 2 | 6 |

These product findings are still present:

- `R-008`: a post to the owner that needs no reply still counts the owner as holding the asker.
- `R-009`: when a stopped asker's question notice is withdrawn, its holder is never told or settled.
- `R-012`, the same cause as Grok's `L-066`: a tool kind outside the known list drops the permission request, and the agent keeps waiting.
- `R-013`: the stall rule for a lead with a busy team. Issue 520 holds the owner's ruling.
- `R-016`: a long last turn loses its newest messages.
- `R-023`: a symbolic link anywhere above the configuration folder fails without saying why.
- `L-038`: an account's `enabled` flag can be left out of a snapshot that requires it.
- `L-047`: the Alerts and Settings pages stay up after the session ends.
- `L-119`: the owner's snapshot leaves out open alerts.
- `L-124`: saving the web settings reports a connection failure.
- `L-132`: a refused board post fails the notice loop, and the host restarts it. The advisory board left open whether the retry lasts forever.

The other still-present findings are missing tests: `R-026`, `R-032`, `R-035`, `R-043`, `R-046`, `R-047`, `R-049` and `R-054`. The 19 partly fixed findings name what remains in each result.

The check corrected one earlier reading. `FR-001` and the Claude review's `R-001` are fixed by commit `7078bd0`, because a public address must now use HTTPS, and plain HTTP is supported only on loopback.

The check did not cover the 526 Claude findings and 139 Grok findings that were never adjudicated. The advisory board found eight causes among those 526 that other panels confirmed, so they are worth a later pass. The task files and results are outside git, in `.scratch/foundry-runs/foundation-plans-1/findings-check/`, and the reviews' evidence is in `review-claude/`, `review-cursor/` and `review-codex/` beside it.

## 520. The stall rule nudges a lead while its own worker is busy, and the plan contradicts itself

Triage (2026-11-23): open, P2, board and roles, medium confidence. The owner's ruling on replacement advice for a stalled lead is not built: no mark on the lead and no workers-done notice. Basis: packages/engine/src/roles/view.ts only maps ReplaceAdvised to a notice kind; no replacement field or ready-for-replacement notice found by grep.

Found: 2026-11-07, recorded from review finding K-368.
State: open. The owner ruled on 2026-11-07, and the code change waits until the freeze lifts.

When an orchestrator posts a question that needs a reply to a lead, the lead goes idle while its workers run. After 20 minutes the lead receives `StallSeen`, and on the third episode the orchestrator receives `ReplaceAdvised`. The code is `packages/engine/src/roles/stall-watch/rules/stall.ts:145-150`. The plan's criterion 4.3, at `.plans/plans/lgtm-foundry-roles/plan.md:939`, says both "nor is the lead" and "still nudged". The plan's line 69 says that a worker waiting on a busy agent is not stalled. No test covers an orchestrator's post to a lead with a busy descendant.

The owner ruled on 2026-11-07 that the lead keeps both the nudges and the replacement advice in this case. The advice changes form, as follows:

- The stall watch stops nudging after the third time, as it does today.
- The replacement advice becomes a notice to the orchestrator and a mark on the lead. The lead's row in the roles view gains a field that says replacement is advised. Today the view counts nudges in `stallCount` but records nothing for `ReplaceAdvised`.
- When every worker under a marked lead has finished, the stall watch tells the orchestrator that the lead's workers are done and that the lead is ready for replacement.
- The orchestrator then saves the old lead's transcript and starts a new lead. The new lead receives the original order, the old transcript to summarise, and what to look through, so it continues where the old lead stopped. Because it is a lead, it reaches the team's workers directly.

The plan's criterion 4.3 should be corrected to say that the lead is nudged. The owner expects three unanswered nudges in a row to be rare. The owner decided to keep both the nudge and the replacement advice. The replacement advice can be a notice to the orchestrator and a mark on the lead.

## 521. Finished review seats filled the temporary folder's file limit

Triage (2026-11-23): open, P2, process, medium confidence. Finished review seats leave full repository copies in the temporary folder, which can exhaust its file limit and break every shell. Basis: Entry state: worked around by hand, cause remains. No cleanup of seat folders is recorded.

Found: 2026-11-07, when every shell command in the manager's session failed with "no space left on device".
State: worked around for now. The cause remains.

`/tmp` allows 1,048,576 files, and it reached that limit while only 21 percent of its bytes were in use. Each review seat kept a full copy of the repository, including `node_modules`, in its session's temporary folder under `/tmp`, and each adjudicator seat held about 28,000 files. Nothing removes those folders when a seat finishes. The failure affects every session on the machine, because each one needs a new file for each command's output.

The manager removed the `node_modules` folders from the 31 finished adjudicator seats, which brought the folder down to 56 percent of its file limit. The folders of the other finished seats remain. The Foundry should give seats a shared read-only copy, or remove a seat's temporary folder when the seat retires. The main disk is also at 96 percent.

## 522. Building the mock's first run exposed two faults in it

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. The mock could not start with no projects and offered no harness choice for a new lead. Basis: Entry state: both fixed on area/web-mock, which later became design/mock on master.

Found: 2026-11-07, while adding a first run to the mock on the branch `area/web-mock`.
State: both are fixed on that branch.

- The page could not start with no projects, because it read the first project while it loaded.
  A real service starts with none, so the mock had no way to show its first minutes.
- A new project's lead always ran on Claude Code, and the New project form offered no choice. A
  person signed in only to Codex would have got a lead on an account that does not work. The form
  now offers the signed-in harnesses, and it hides the choice when there is one.

## 524. The live check's Codex tool step can fail because the agent never calls the tool

Triage (2026-11-23): open, P2, tests, medium confidence. The live check's Codex tool step fails when the model simply does not call the tool, with no second prompt. Basis: Entry state: open. No retry or fix recorded; live-check probe tests still expect the 'not attempted' reason.

Found: 2026-11-08, while checking an earlier rename of the product.
State: open.

In the first `pnpm live-check` run on the rename branch, the `sandbox-mcp` step failed with "codex: not attempted; exact ask tool calls=0". The Codex agent, GPT-6 Luna at low effort, made no call to the `lgtm` ask tool, so the probe had nothing to judge. The Claude agent's call succeeded in the same run. A second run passed the step for both agents, with the same code, so the failure depends on whether the model chooses to call the tool. The step should give the agent a second prompt, or run on a model that follows the instruction reliably, before it reports a failure.

## 535. Idle configuration dispatch entered the writer without queued work

Triage (2026-11-23): reported fixed, not verified (low confidence); kept here until someone checks. An idle agent's configuration dispatch entered the delivery writer with no queued work and logged an extra failure line. Basis: Entry says correction written on unit/website-w05c with before and after proof; verification was pending, and I did not locate the recheck in code.

Found: 2026-11-08, during the website composer engine full check.
State: correction written on `unit/website-w05c`, pending verification.

The configuration dispatcher now admits an idle agent even when it has no queued delivery.
This lets a pending owner choice apply before another prompt arrives.
After configuration, the dispatcher entered the delivery writer without rechecking the queue.
The baseline returned before entering the writer when no delivery was ready.
A latched write failure rejects even an empty transaction.
An idle dispatch could therefore emit an extra fixed delivery failure diagnostic during the runner-event append-refusal test.
That test reported four captured lines where its exact assertion requires three.
Asynchronous scheduling exposes the extra diagnostic, but the empty transaction is a new configuration-path regression.

The correction rechecks the ready queue after configuration while retaining the same agent lane.
It returns before entering the delivery writer when there is no work to reserve.
The diagnostic count, fixed-message, redaction, defect-isolation, and recovery assertions remain exact.
The orient tests also had a local running-session fixture without an applied configuration report.
That fixture now includes the model and effort it claims the harness opened with.
The assertions that reads do not write events remain unchanged.
The read-only fixture audit found no other unreported direct test with a missing session configuration acknowledgement.
Historical projection fixtures continue to represent unknown applied configuration honestly.

A temporary diagnostic reproduced the pre-correction failure with the exact assertions retained.
Its captured order was three `runner event dropped: processing failed` messages followed by `delivery dispatch failed`.
The orient cases gained six `AgentConfigurationApplied` events each, changing sequences 21 to 27 and 15 to 21.
An isolated baseline-dispatcher control passed the same three assertions with only the three runner-event messages and unchanged sequences.
The corrected dispatcher and acknowledged orient fixtures also passed those three assertions.
Their captured messages and unchanged sequences matched the baseline control.
The temporary diagnostic instrumentation was removed after the comparison.

## 540. The reservation cancellation test could cancel before its fake peer started the turn

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. The reservation cancellation test could cancel before its fake agent had started the second turn. Basis: packages/engine/test/runner/cancel-reservation.test.ts lines 19 and 34 emit and wait for the plan marker 'reservation 42 is active'.

Found: 2026-11-08, during the first complete project lifecycle check.
State: the test synchronizes with the actual second child turn; independent review remains required.

The complete check exited 1 at 2026-11-09T05:16:12.611869+00:00.
The protocol 2 case in `packages/engine/test/runner/cancel-reservation.test.ts`
received only the first reservation's `promptReply` and `turnEnded` events.
It never received the stopped terminal event for reservation 42.
That failure remains preserved in `/tmp/w04b-full-check-1.log`.
An isolated pass of the unchanged test did not explain the failure.

The production runner returns from prompt after writing its frame to the child pipe.
That acknowledgement does not establish that the child has started its prompt handler.
The protocol transport queues prompt requests for its server handler.
It processes cancel notifications directly from its input reader.
The fake agent installs a cancel signal only when its prompt handler starts.
It discards a cancellation received before that signal exists.
The changed `withEngine` log-wrapper branch was not on this test's path.
The test uses the unchanged `withAcpRunner` helper.

A controlled real-runner probe paused the child while the second prompt and
current cancel reached its pipe, then resumed the child.
Observation-only instrumentation in a temporary copy of the fake agent recorded
cancel with no installed signal, followed by the second prompt handler installing it.
The wire still contained prompt 1, prompt 2, and exactly one cancel, in that order.
The original stale-cancel rejection and zero-frame assertions passed.
The terminal wait then failed with the same missing-event report as the full run.
The only terminal event belonged to sent sequence 17.
A control waited for the actual second handler before sending current cancel.
It passed the original assertions with sent sequence 42, `stopped: true`,
and exactly one cancel frame.
The full run did not record the child's internal scheduling.
The probe proves this failure mechanism without reconstructing that unrecorded schedule.

The permanent test now emits a unique plan marker from the second child turn.
It waits for that actual runner delivery update before exercising either cancellation.
The marker follows installation of the child's cancel signal.
It is not a timer, a fabricated runner event, or a relaxed terminal assertion.
The original reservation identities, stale-cancel failure, zero stale-cancel frames,
stopped terminal event, and exact one-current-cancel count remain unchanged.
The production runner, protocol transport, and shared fake agent are unchanged.
Temporary repository probes are removed after their evidence is retained in
`/tmp/w04b-cancel-order/`.

## 554. Empty project request pages skipped the complete reply budget

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. An empty project requests page could return a reply larger than the allowed size. Basis: Entry text: read now measures the final wrapped reply and refuses oversized ones; socket and handler regressions cover it.

Found: 2026-11-09, during independent project request review at `2616fb5`.

The `project.requests` read checked its encoded reply size only while adding rows.
An empty project, a filter with no matches, or an exhausted cursor skipped that check.
A valid request identifier containing 140,000 quotation marks produced a 280,027-byte owner success reply.
The same identifier produced an 840,165-byte lead success reply because the agent data envelope adds another encoding layer.
Both exceeded the 256 kibibyte complete success reply budget.

The read now measures the actual final wrapped success reply before returning it.
An oversized reply refuses without truncating an identifier or changing the shared request identifier schema.
The existing row budget, pagination, caller checks and owner answer authority remain in place.

Authenticated socket and direct-handler regressions cover the original owner and lead identifiers.
They also cover ordinary empty replies, escaped identifiers within budget, kind and status filters, and an exhausted cursor.
A separate control proves that an identifier can fit the owner reply while exceeding the lead envelope budget.
The inherited populated-page test continues to check every row, page progress and complete encoded sizes.
The reads preserve the durable log, and the real rebuild check remains enabled.
The original independent failed probes and logs are retained unchanged.

## 566. Reading review evidence could deadlock an owner approval

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. Reading review evidence while the owner approved could leave both calls stuck forever. Basis: Entry text: status reader releases the Git gate before the thread query, with a permanent regression; commit 75f48a8a names the Git recovery fix.

The independent review of issues 545 and 555 found a lock-order inversion.
The review status reader held the Git gate while validating a recorded thread through the shared Structured Query Language (SQL) connection.
A concurrent owner approval held that connection inside its writer transaction while waiting for the Git gate.
Both authenticated calls then remained pending after the active Git child finished.
The matched case without recorded evidence completed because it needed no thread query.

The status reader now completes its authorized project and checkout inspection under the Git gate and releases that gate before querying thread evidence.
It evaluates the evidence against the exact source head observed during that inspection and the same authorized snapshot.
The project, workspace, repository, branch, thread membership, redaction, and reply-budget checks remain in place.
Command transactions and active Git child ownership are unchanged.
The other Git read handlers do not query the SQL connection while holding the Git gate.

The permanent regression overlaps actual authenticated status and approval calls with recorded thread evidence and a matched no-evidence control.
It establishes an actual Git child barrier and observes entry after SQL begins its transaction.
Both calls must finish, and the approved destination must contain the reviewed commit and expected bytes.
The test also checks the durable decision and receipt, identical retry, and a rebuild from the event log.
Original failed probes and their historical invalid-barrier attempts remain preserved separately.
The builder report records actual red and green execution boundaries.

## 588. Aggregate untracked patches repeated full status scans

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. Building a combined diff rescanned the whole repository once per untracked file, making it very slow. Basis: Entry: aggregates reuse one status snapshot; nine real regressions and full remote check passed; commit 1f456e44 'bound aggregate reads'.

The original Git author isolated a recursive aggregate diff call that repeated HEAD and status reads for every untracked file.
The manager supplied that paused author's exact two-file patch for the shared correction in this unit.
Both owner and filtered agent aggregates now reuse one complete status snapshot and one HEAD observation, then use the same bounded direct no-index patch per admitted untracked file.
Literal paths, binary flags, full text statistics, byte and entry bounds, symlink checks and agent exclusions remain consequential in the adopted real process-count controls.
This is one canonical correction, shared with issue 598; its new proof and independent re-check remain pending.

The original Git author's sealed diagnostic recorded 71 recursive entries and 302 actual Git child calls.
Its original five-second assertion failed with 55 child calls unsettled.
The observed gate wait was 0.027 milliseconds, with no competing admission during that execution.
Call-to-spawn intervals and child wall intervals remain separate evidence, without a CPU, network or load cause claim.
The original narrower three-path preparation and all its proof remain archived in that author's retained worktree.
The author's later accepted-source join supersedes those engine paths with the canonical implementation and regressions from b974da0.
The original browser failures remain failed; the accepted flow proofs do not prove the subsequently joined Git page.

The joined page's later full12 passed its unit checks and failed three browser cases.
Its aggregate returned complete data near the original assertion boundary, without a proved allocation between gate waiting and child work.
The page also withheld received sync counts behind final working identity checks and invalidated coherent data after read-only review preparation.
Shared projection completeness restarted Git reads even while the connection remained live.
Seven consequential mounted controls failed on that original page; a lost preparation reply remained uncertain in the eighth passing compatibility control.
The scoped UI correction separates received counts from mutation readiness, preserves read continuity with explicit reconnect recovery, and retains review readiness only for an exact matching coherent generation.
The candidate's two mounted files passed all 50 controls, and web typing and credits passed.
The canonical aggregate engine implementation is unchanged.
The later full15 passed 7,675 unit tests and 36 browser cases, but the layout case again missed its five-second 71-file assertion.
Its second aggregate delivered complete data and the observer later counted 71 wrappers; the assertion deadline was not recorded, so that trace does not establish the cause.
The unchanged selected run16 passed and remains inconclusive about that failure.
The layout fixture now indexes its same 71 real files before opening the page, leaving file contents, layout assertions, commit count and deadlines unchanged.
The separate real aggregate regressions retain the untracked-file and process-count coverage.
The corrected layout case passed its eight viewport and theme combinations remotely with the original assertions and deadlines.
All nine selected real aggregate regressions also passed remotely, including the six 71-file owner and agent cases.
These results do not retrospectively establish the cause of the full15 failure.
The complete remote check then passed all 7,675 unit tests and all 37 browser cases, with two unit tests skipped.
The temporary browser observer, assertion reporter and explicit aggregate probe were subsequently removed from delivery under the owner's observability rule.
Their original source and artifacts remain in the retained investigation evidence.
The observer-free related browser run passed ten cases and failed the final push count assertion.
One diagnostic execution and three bounded repetitions passed, so those observations do not establish the cause of that failure.
The browser now requires the visible push acknowledgment separately before its existing fresh outgoing-count assertion.
Both assertions retain the default five-second timeout, and the case retains its original 120-second timeout.
All temporary observations were removed again before delivery.

## 593. Git process-wrapper tests are sensitive to host load

Triage (2026-11-23): open, P3, tests, high confidence. Git wrapper tests time out at five seconds when the machine is heavily loaded. Basis: Entry state open, checked 2026-11-15; packages/engine/test/project/main-review.test.ts still exists with no recorded change.

State: open. Checked against the code on 2026-11-15. Entry deliberately leaves timeouts unchanged and wrappers unrewritten; no later entry records a fix for host-load sensitivity. Kind: tests.

The independent Opus round-one review observed eight default five-second timeouts in main-review.test.ts at a load near 39 on 38 processors.
All 31 cases passed when the reviewer separately used a 60-second timeout.
That is attributed reviewer evidence, not a builder rerun or a passing result at the original timeout.
The wrappers start a Node.js process for each Git call.
This unit records the sensitivity without increasing timeouts, rewriting wrappers broadly or running a loaded local full.
Original failures, original timeouts and the lightly loaded remote full proof remain preserved.


## 600. Common remote authentication refusals conservatively remain uncertain

Triage (2026-11-23): open, P3, git, medium confidence. Common SSH and HTTPS sign-in refusals from a remote are reported as uncertain instead of as sign-in required. Basis: Entry state: open, accepted as a minor safe-side limitation; classifier deliberately left conservative.

Found: 2026-11-10, in the independent Opus check of the whole flow-fix unit.
State: open, accepted as a minor safe-side limitation.

A common SSH public-key refusal includes multiple diagnostic lines.
An HTTPS server can also prefix its authentication refusal with a server message.
The strict pre-effect grammar does not recognize these complete shapes, so they report terminal uncertainty rather than authentication_required.
The manager accepted the reviewed unit at Grade 8, with no blockers, majors or flags.
The classifier stays conservative in this change.
Any later widening must prove exact pre-effect refusal shapes without treating local partial effects as authentication failures.

## 601. The editor image config reference was unavailable in the containerd store

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. The engine could not find the editor image by its digest on the test machine. Basis: packages/engine/src/client/web/display/openPencil.ts:93 uses the fixed tag lgtm-openpencil:w10b-cb71737-build1; entry records the native run passing.

Found: 2026-11-10, during manager preparation for the W10B native display proof.
The manager allocated this number.
The approved image was present under its transferred name and tag on the remote personal computer.
The engine's config-digest reference returned `No such image`.
The original remote typing failure, later passing typing and Docker inspection records remain preserved separately.

The owner ruled that every host finds the editor by `lgtm-openpencil:w10b-cb71737-build1`.
The engine now inspects that local tag on every launch and requires Linux on AMD64.
No digest lookup or equality decides image presence.
A missing image is refused without building, pulling, retagging or choosing a fallback.
The existing `--pull=never`, non-root user, isolated network, folder mounts, resource limits, socket deadline and owned-name cleanup remain.
The native fixture uses the same tag and checks the actual generated container name.
Its original display, import, pixel, isolation and cleanup assertions remain.
Temporary static launcher diagnostics are retained separately and must not be committed.
Status: product and fixture corrected; independent remote typing and focused checks passed.
The native follow-up below records the later startup failures and their disposition.
The issue remains open pending unit review and integration.


### The later editor boot failure is separate from image presence

The source23 and direct remote native cases started the named editor image and opened its frame.
The registered document assertion failed before the capability assertion ran.
The approved temporary boot-notice read then measured a localStorage SecurityError in the opaque sandbox.
The original failures, cleanup outcomes and temporary observation remain preserved in website-1/w10b-document-import.
On 2026-11-10, the owner approved adding allow-same-origin to the editor iframe and its content security policy sandbox directive.
Recorded mocks keep their opaque sandbox.
The editor keeps its separate display hostname, credential-omitting document import, owner-cookie exclusion, frame ancestors, referrer policy, revocation and logout controls.
The temporary boot-notice read was removed before this correction's commit.
No OpenPencil source patch or image rebuild is applied.
At product commit e740c19, remote engine, web and native-input typing passed.
All 34 tests across the display boundaries, project review and actual engine review files passed.
The related native case first exposed an unhandled Playwright response-body read, then Docker's CAP_NET_BIND_SERVICE spelling.
The case now proves file bytes with a credential-omitting browser read of the exact issued URL, while retaining the original import status, component and canvas checks.
The capability assertion normalizes only the optional CAP_ prefix and still requires exactly the one approved capability.
The final native run passed in 10.6 seconds, including actual modules, WebAssembly, registered file import, screenshots, owner isolation, logout and cleanup.
Its outcome records completed=true and owned containers absent; a separate Docker listing was empty.
The screenshots still show OpenPencil's font-unavailable-or-substituted warning.
This proves the selected document renders, but does not establish exact font fidelity or whole-unit acceptance.
All original failed logs remain in website-1/w10b-document-import, alongside the passing logs and screenshots.

### The first ordinary full check found two missed repository guards

The full remote check of 720077d returned 1 after typechecking passed.
Vitest reported 7,650 passing tests, two skipped tests and two failing guards; browser and credit stages did not run.
The display input callback constructed an owner credential outside webCaller.
It now obtains and revalidates its caller through webCaller before each engine input read.
The pure display-boundary and markup tests also omitted their required rebuild declarations.
Both now register noLog=true because neither opens an event log.
The original full log and direct exit remain preserved in website-1/w10b-document-import.

## 611. The mock preview issuance observation failed in the W11 full

Triage (2026-11-23): open, P3, tests, high confidence. A browser test of the mock preview fails now and then in full parallel runs, with the cause unproved. Basis: Entry state open, checked 2026-11-15; packages/web/e2e/new-project.spec.ts still exists; four recurrences recorded. It lives only in a screen that is being rebuilt.

State: open. Checked against the code on 2026-11-15. packages/web/e2e/new-project.spec.ts still asserts data-loaded at lines 569, 626 and 640 and the entry records an unproved cause with recurrences. Kind: tests.

Open: on signed 5e12347, new-project.spec.ts:605 observed status 0 instead of 200 within five seconds in full4. The unchanged uninstrumented case and credits then passed. The cause remains unproved. The manager accepted that combined gate without changing the assertion or deadline.

The same registered component journey failed again in the final merged full on 4f9ae0a, at a different assertion.
At new-project.spec.ts:626, the preview frame's body never received data-loaded="yes" within five seconds.
The unchanged case passed on its own in 24.6 seconds.
Both failures are in the mock and component preview path under a full parallel browser run.
The cause remains unproved.

It recurred at new-project.spec.ts:605 in the full run of the rename branch on 2026-11-12, with status 0 instead of 200.
The unchanged case passed on its own in 27.3 seconds.

It recurred at the preview frame's data-loaded check in the full run of the earlier-names cleanup on 2026-11-12.
The unchanged case passed on its own in 24.5 seconds.

## 613. Agents carry invented personal names in the engine and the older mock

Triage (2026-11-23): open, P3, website, high confidence. Sample data files and one engine test still use the invented agent names Rowan and Sage. Basis: grep finds the names in design/data/heavy and sample events.json and reads.json, and leadName "Sage" in subscribe-visibility.test.ts:100.

State: open. Checked against the code on 2026-11-15. The engine name list in lifecycle.ts is gone, but design/data/heavy and sample events.json and reads.json still carry the invented names Rowan and Sage. Kind: internal.

Open: the owner rejected invented agent names on 2026-11-10. An agent is named by its role.
The engine still gives each project lead a name from a fixed list at
`packages/engine/src/project/lifecycle.ts:21`. The list starts with Sage and Rowan.
The test `packages/engine/test/api/subscribe-visibility.test.ts` uses those names.
The files under `design/data/` also use such names. The older `design/workbench.html` used
them too, until it was removed on 2026-11-12.
The rebuilt mock in `design/mock/` names every agent by its role.
The engine change is not made yet, because the lead's name is passed to the agent start
at `lifecycle.ts:251` and is written into stored events.

## 614. The design file lags the rebuilt mock's tokens and names

Triage (2026-11-23): open, P3, website, medium confidence. The design file has fewer colours and variables than the code and still shows an invented agent name. Basis: Entry state open, checked 2026-11-15 against design/mock/js/screens/design.js:87; design/lgtm.fig still present.

State: open. Checked against the code on 2026-11-15. design/mock/js/screens/design.js:87 FILE_COLORS still lists only 15 colours, so the file still lags the 38 code tokens; later issue 621 does not claim full parity. Kind: internal.

Open: found on 2026-11-10 while building the repository Design page in `design/mock/`.
The design file is `design/lgtm.fig`.
Its `Agent row` variants show the name "Rowan", which breaks the rule that agents are named
by role.
It has 15 colour variables, while `design/mock/css/tokens.css` has 38 colours.
Thirteen of the shared colours were retuned in code, and only `surface` and
`primary-contrast` still agree.
It has no type, space, density, shape or motion variables.
The Design page marks every such value "Only in code" or "Retuned in code" until the file
catches up.
The full list of changes is in `design/mock/feedback/2026-11-10-design-categories.md`,
under "What LGTM Studio's file must change to meet the standard".
The Design lead agent proposes those changes, and the owner approves them.

## 615. The mock's Git tab replaces its structure with a blank page when there is no worktree

Triage (2026-11-23): open, P2, website, high confidence. The Git tab shows one centred empty message instead of keeping its columns when a project has no worktree. Basis: design/mock/js/screens/git.js:722 still returns the single Empty 'No worktrees yet'. It lives only in a screen that is being rebuilt.

State: open. Checked against the code on 2026-11-15. design/mock/js/screens/git.js:722 still returns the single centred Empty state titled No worktrees yet when a project has no worktree. Kind: visible to the owner.

Open: found on 2026-11-10 during the second round of the mock.
The Git tab draws one centred "No worktrees yet" message at
`design/mock/js/screens/git.js:713` when a project has no worktree.
The rule since round 2 is that a screen with no data keeps its structure.
The Git tab should keep its Changes and History columns, with an empty row in each.
The round 2 brief did not list the Git tab's empty state, so it was left as it is.

## 616. The repository Design page draws every screen frame as a live copy of the mock

Triage (2026-11-23): open, P3, website, high confidence. The Design page loads a full live copy of the mock for every screen frame, which is slow and heavy. Basis: design/mock/js/screens/design.js:186 still renders an iframe per frame, lazy loaded. It lives only in a screen that is being rebuilt.

State: open. Checked against the code on 2026-11-15. design/mock/js/screens/design.js:186 still renders an iframe per screen frame, only lazy loaded, so every frame loads a live mock copy. Kind: internal.

Open: found on 2026-11-10 while building the Design page in `design/mock/`.
Each screen frame on the Screens sheets is a frame that loads the whole mock at that
screen's address.
A flow with eight frames starts eight copies of the mock, each with its own data.
The page takes about a second longer to settle on the larger flows, and it uses more memory
than a picture would.
A real build should draw the frames from the design file, or from stored pictures, and load
a live copy only for the frame the owner opens.

## 624. The live website's Machines page has no figures for the engine's machine

Triage (2026-11-23): open, P2, engine, medium confidence. The engine reports no processor, memory, operating system, version or agent slot limit for its machine. Basis: Entry state partly fixed, verified 2026-11-16; grep finds no cpus, totalmem or slot limit in engine or contracts source.

Found: 2026-11-13, while connecting the mock to the engine.
State: partly fixed, because the live page now shows the workspace, web port and harnesses, while processor, memory, operating system, version and the agent slot limit are still missing from the engine. Verified against the code on 2026-11-16.
Earlier state: open.

The engine sends no processor, memory, agent slot, address, operating system or harness version for any machine.
The live website therefore shows a dash for processor and memory, and "Scanning…" for the harnesses.
Its agent count reads "0 of 0", in the warning colour, because the slot limit is unknown and is stored as 0.
The engine calls its own computer `local` until machine pairing exists, so the page names the machine `local`.

## 627. The live board inferred sprint status from its phase.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. The live board guessed a sprint's column from its phase instead of using the engine's status. Basis: Entry text: 'This change maps those four engine states' and removes the mock defaults from live rendering.

The first connection put a sprint into a column from its phase and completion time.
The engine derives status from its completion text and task states.
This change maps those four engine states and leaves Backlog empty.
It also removes the mock's default priority, roadmap dates, and eight-phase progress from live board rendering.

## 631. The mock hides message post actions in a narrow panel.

Triage (2026-11-23): open, P3, website, low confidence. Message post action buttons may be hidden when the message panel is narrow. Basis: Entry state unclear on 2026-11-15; grep finds no 720 pixel rule hiding post actions in design/mock/css; needs a browser check. It lives only in a screen that is being rebuilt.

State: unclear. Checked against the code on 2026-11-15. No 720 pixel rule hiding post actions found in design/mock/css; could not locate the hiding rule, so it needs a browser check at narrow width. Kind: visible to the owner.

The existing message styles hide the post action buttons when the panel is narrower than 720 pixels.
This also happens on a desktop when the sidebar and channel list reduce the panel width.
The browser test uses a wide viewport to exercise those existing controls.
Changing this layout remains outside this connection change.

## 657. The engine labels the owner's answer as "Data from another agent"

Triage (2026-11-23): open, P2, engine, high confidence. An agent is told the owner's answer is data from another agent. Basis: packages/engine/src/routing/envelope.ts:18 still falls back to 'Data from another agent' with no owner case.

Found: 2026-11-13, in the same check as issue 656.
State: open.

The wrapper around an answer the owner gave begins "Data from another agent" (`packages/engine/src/routing/envelope.ts:18`).
The agent that asked is told the answer came from an agent, when it came from the owner.

## 670. An answered question shows its accepted answer beside the buttons.

Triage (2026-11-23): waits for the owner's decision, P3, website. An answered question card shows the accepted answer inside the button row; the owner wants it on its own line. Basis: Entry: owner will rework this look later, recorded and not changed. It lives only in a screen that is being rebuilt.

Owner ruling (2026-11-25): Leave it open. The rebuild of the website decides it, because it concerns a screen that is being redesigned.

State: open. Checked against the code on 2026-11-15. Entry says the owner will rework this look later and it is recorded unchanged; the Deny button row in design/mock service.js still holds the answer input. Kind: waits for a decision.

The owner reported this on 2026-11-13.
An answered question card puts the accepted answer in the button row, between "Deny" and "Details".
The owner finds that placement odd.
The owner wants a short form of the accepted answer on its own line above the buttons, as the card's second or third line.
The owner will work on this part of the look later, so it is recorded and not changed.

## 673. The settings contract does not return effective role defaults.

Triage (2026-11-23): open, P2, client api and command line, high confidence. The role settings read returns only stored overrides, so the website cannot show the limits actually in effect. Basis: packages/engine/src/roles/stall-watch/settings.ts:34 getSettings returns view.settings only; entry verified partly fixed 2026-11-16.

Found: 2026-11-13, while connecting Service settings.
State: partly fixed, because the lead and assigned stall timers now have settings since `87f9d2c`, while `roles.settings.get` still returns only stored overrides and not effective defaults. Verified against the code on 2026-11-16.
Earlier state: open.

The role settings request returns stored overrides.
It does not return the effective limits after the engine applies its defaults and role files.
The website shows an empty number for an unset limit.
An unset switch is labelled as not set by the service.
Worker stall time edits the service-wide stall limit.
The separate lead and assigned timers have no matching setting and remain empty.

## 686. The Mocks tab is absent on a project without mocks.

Triage (2026-11-23): waits for the owner's decision, P2, website. The Mocks tab is hidden for a project with no mocks, and the owner has not said whether it should always show. Basis: design/mock/js/app.js:260 still shows the tab only for the Design stage or recorded screens. It lives only in a screen that is being rebuilt.

Owner ruling (2026-11-25): Leave it open. The rebuild of the website decides it, because it concerns a screen that is being redesigned.

State: open. Checked against the code on 2026-11-15. Entry states whether the Mocks tab should always show is the owner decision, and no later entry records that decision. Kind: waits for a decision.

The owner reported this on 2026-11-13.
The live website shows the "Mocks" tab only when a project is in its design stage or has a recorded mock.
The project "Connect website engine" has neither, so the tab is not shown.
The mock behaves the same way with its sample data.
Whether the tab should always show, with an empty state, is the owner's decision.
This is not changed.

## 687. The owner judged Subscriptions and Service settings unfinished.

Triage (2026-11-23): waits for the owner's decision, P2, website. The owner called Subscriptions and Service settings unfinished but has not yet said what is missing. Basis: Entry text: the owner did not yet say what is missing; no later entry mentions 687. It lives only in a screen that is being rebuilt.

Owner ruling (2026-11-25): Leave it open. The rebuild of the website decides it, because it concerns a screen that is being redesigned.

State: open. Checked against the code on 2026-11-15. Entry records the owner has not yet said what is missing on Subscriptions or Service settings, and no later entry resolves it. Kind: waits for a decision.

The owner reported this on 2026-11-13.
The owner found that "Service settings" needs a lot of work and is acceptable for now.
The owner found that "Subscriptions" needs work before a release.
The owner did not yet say what is missing on either page.


## 692. Default project channel names conflict with the manager's unique-name rule.

Triage (2026-11-23): waits for the owner's decision, P2, board and roles. The owner cannot manually open another channel named General or Leads because channel names must be unique per manager. Basis: packages/engine/src/board/commands.ts:39-40 still refuses with 'channel name already belongs to this manager'; entry awaits owner decision.

Owner ruling (2026-11-25): A channel has an identifier and a label. The identifier is unique across the service. The label is unique within one project only, so two projects may both have a channel named General. Because the server and the client are being rewritten in Go, this ruling is an input to that design.

State: open. Checked against the code on 2026-11-15. packages/engine/src/board/commands.ts line 40 still refuses a channel name that already belongs to this manager; no later entry resolves the owner decision. Kind: waits for a decision.

Open after the default-channel review on 2026-11-13.
The owner manages a General channel and a Leads channel for each project.
These automatic channels can repeat those names across projects.
The board's manual open command still requires a unique name for each manager.
The owner therefore cannot manually open another General channel or another Leads channel.
The rule remains unchanged pending an owner decision.

## 719. Account homes can retain discovery session history.

Triage (2026-11-23): open, P3, harnesses and agent running, medium confidence. Model discovery sessions may leave session history in the Claude, Codex and Gemini account folders, which is never cleaned. Basis: Entry: cleaning arbitrary account-home history is not implemented; only the Cursor session directory is removed.

State: open. Checked against the code on 2026-11-15. Recorded limitation about account-home discovery history; no later entry or code found that cleans arbitrary account homes beyond the Cursor directory. Kind: internal.

The Agent Client Protocol (ACP) recordings show session identifiers and configuration replies.
They do not record filesystem writes in account homes.
A temporary working folder therefore does not prove that discovery leaves no persistent harness metadata.
No real harness was started for this investigation.

Claude Code documents transcripts under `~/.claude/projects/<encoded-working-directory>/<session-id>.jsonl`.
Its directory documentation also lists per-session debug, tool-result, and memory data.
The documentation establishes storage outside the working folder, but it does not establish whether a prompt-free ACP session always creates a transcript.
The sources are [session storage](https://code.claude.com/docs/en/sessions#export-and-locate-session-data) and [the account directory](https://code.claude.com/docs/en/claude-directory).

The Codex ACP adapter creates a Codex thread during `session/new`.
Codex has a persistent rollout recorder and state index under its configured account home.
The source distinguishes recorder creation from explicit persistence, so it does not prove that every prompt-free discovery produces a rollout file.
LGTM Studio's named-account shadow home also shares entries from its source home.
The sources are [the adapter's new-session implementation](https://raw.githubusercontent.com/zed-industries/codex-acp/main/src/codex_agent.rs), [the rollout recorder](https://raw.githubusercontent.com/openai/codex/main/codex-rs/rollout/src/recorder.rs), and `packages/engine/src/accounts/shadowHome.ts`.

Gemini's ACP new-session handler initializes a chat before returning its model report.
The chat initializes its recording service, which creates a chats directory and writes initial session metadata.
Its documented session location is `~/.gemini/tmp/<project_hash>/chats/`.
A distinct temporary project folder can therefore leave a distinct project history directory outside that folder.
The sources are [the ACP session manager](https://raw.githubusercontent.com/google-gemini/gemini-cli/main/packages/cli/src/acp/acpSessionManager.ts), [chat initialization](https://raw.githubusercontent.com/google-gemini/gemini-cli/main/packages/core/src/core/client.ts), [the recording service](https://raw.githubusercontent.com/google-gemini/gemini-cli/main/packages/core/src/services/chatRecordingService.ts), and [session management](https://geminicli.com/docs/cli/session-management/).

The installed Cursor runtime's `acp-storage.ts` stores its session database beneath `CURSOR_CONFIG_DIR/acp-sessions`.
The existing `cursorMcp.ts` integration redirects that location into the engine's per-agent session folder.
Discovery now removes both its disposable settings and its redirected session directory.
The runtime evidence and version are recorded in `packages/engine/src/runner/cursorMcp.ts`.
The recordings do not establish whether Cursor writes other account-wide session metadata.

Cleaning arbitrary account-home history is not implemented in this change.
Those stores can contain the owner's other sessions and use version-dependent layouts.
The bounded cache reduces repeated discovery without deleting account data or altering login settings.
A future harness-specific discovery mode could disable persistence where its supported interface permits it.

## 722. The build-loop diagram still contains a hard exit after round six.

Triage (2026-11-23): open, P3, documents, high confidence. The build-loop diagram still shows a forced exit after round six, though the owner removed that limit. Basis: docs/specs/2026-11-05-build-loop-chart.md:220 still has the step 'Plan cut, split or accepted'.

State: open. Checked against the code on 2026-11-15. docs/specs/2026-11-05-build-loop-chart.md line 220 still contains the step Plan cut, split or accepted; the historical specification was deliberately left unchanged. Kind: internal.

The owner removed the review limit on 2026-11-11.
The prose now asks for approval before every further round.
The steps under "Next round" still send round six to cutting, splitting, or accepting the plan.
The default workflow definition follows the newer decision and repeats the approval request.
The historical specification was left unchanged.

## 723. Workflow editing does not select or execute a project's workflow.

Triage (2026-11-23): open, P2, engine, medium confidence. Workflows can be edited and saved, but a project cannot choose one and none is executed. Basis: design/mock/js/screens/workflows.js:108 still says choosing a workflow for a project is not built yet.

State: open. Checked against the code on 2026-11-15. design/mock/js/screens/workflows.js line 107 still says projects run the default workflow because choosing a workflow for a project is not built yet. Kind: visible to the owner.

The engine had no stored workflow definition when this work began.
The new definition store and visual editor provide configuration operations.
Project selection and workflow execution remain the later build-loop work.
The Workflows page states that projects still run the default workflow.

## 727. The existing service-settings browser journey failed once while waiting for its saved state.

Triage (2026-11-23): open, P2, tests, low confidence. The service-settings journey sometimes never sees the saved state after changing the heartbeat in a full run. Basis: Recurs in issues 810 and 898 at studioService.ts:74-75; cause never established; passes alone.

State: open. Checked against the code on 2026-11-15. Later entries around lines 12993, 13840 and 13921 record the same save journey failing again in full runs, so the flakiness persists with no fix. Kind: tests.

The full check passed 8,080 unit tests and 50 browser tests.
The existing service-settings journey timed out at `studioService.ts:74` after changing the heartbeat from 25 to 31.
Its save bar did not show "No unsaved changes." within five seconds.
The unchanged journey passed when run alone.
The workflow browser journey passed in the full run.
The service-settings implementation was left to its assigned builder.
The full check therefore remains a failed run even though the isolated retry passed.


## 728. Workflow actions have no event or receipt.

Triage (2026-11-23): waits for the owner's decision, P2, engine. Workflow changes leave no event or receipt, and the owner must decide how they appear in the audit history. Basis: Entry: 'This is open for the owner'; no later entry resolves it.

State: open. Checked against the code on 2026-11-15. Entry waits for an owner decision on audit history of workflow actions; no later entry resolves it. Kind: waits for a decision.
Owner ruling (2026-11-23): no event and no receipt. A workflow change is recorded as a version beside the workflow file, as described in docs/features.md under "Versions of a workflow, kept beside its file". The owner said that a person who makes their own workflows keeps their own backups, and that the default is cloned, not modified.

This is open for the owner.
Workflow actions operate directly on configuration files.
They write neither an engine event nor an action receipt.
The owner must decide how workflow changes should appear in the audit history.

## 729. Two browser tabs can overwrite a workflow without a warning.

Triage (2026-11-23): waits for the owner's decision, P2, client api and command line. Two tabs editing the same workflow silently overwrite each other; the last save wins. Basis: No revision or etag found in packages/engine/src/workflows or contracts/src/workflows.ts; entry is open for the owner.

State: open. Checked against the code on 2026-11-15. No revision or etag check found in engine workflow code and no later entry resolves the last-save-wins owner question. Kind: waits for a decision.
Owner ruling (2026-11-23): the version number of a workflow file is the revision check. A save names the version it edited, and a stale save is refused. See docs/features.md under "Versions of a workflow, kept beside its file".

This is open for the owner.
The last successful save wins when two tabs edit the same workflow.
The editor has no revision check or conflict warning.

## 730. Renaming a workflow changes only its display name.

Triage (2026-11-23): waits for the owner's decision, P3, client api and command line. Renaming a workflow changes only its display name, not its identifier or filename; the owner must decide if that is wanted. Basis: Entry: 'This is open for the owner'; no later entry resolves it.

Owner ruling (2026-11-25): Renaming a workflow changes its display name only, and its identifier stays stable. This is kept. The issue can be closed. Because the server and the client are being rewritten in Go, this ruling is an input to that design.

State: open. Checked against the code on 2026-11-15. Owner decision on separate identifier rename is still pending; no later entry resolves it. Kind: waits for a decision.

This is open for the owner.
Saving a different name leaves the workflow identifier and filename unchanged.
The owner must decide whether a separate identifier rename is needed.

## 731. The visual editor cannot change inner steps.

Triage (2026-11-23): waits for the owner's decision, P3, website. The visual workflow editor shows a block's inner steps but cannot change them; only the API and terminal can. Basis: Entry: open for the owner; workflows.js shows only a count of inner moves. It lives only in a screen that is being rebuilt.

Owner ruling (2026-11-25): Leave it open. The rebuild of the website decides it, because it concerns a screen that is being redesigned.

State: open. Checked against the code on 2026-11-15. design/mock/js/screens/workflows.js line 120 only shows the count of inner moves per block with no step editing control. Kind: waits for a decision.

This is open for the owner.
The editor shows and preserves a block's inner steps.
Only the application programming interface (API) and terminal can edit these steps.


## 732. The Git browser journey could not approve without evidence.

Triage (2026-11-23): open, P2, tests, medium confidence. The Git browser journey sometimes cannot click Approve without evidence in a full run, and passes alone. Basis: Entry and issues 693, 704, 778 record the same timeout in packages/web/e2e/support/studioGit.ts; no cause or fix recorded.

State: open. Checked against the code on 2026-11-15. Intermittent Git journey approval timeout is a recorded flake; the click helper still exists at packages/web/e2e/support/studioGit.ts line 129 and no entry declares it fixed. Kind: tests.

The review-fix full check passed every unit test.
The existing Git browser journey timed out at `packages/web/e2e/support/studioGit.ts:100`.
The Approve without evidence button became disabled while Playwright waited to click it.
The unchanged journey passed alone in 2.2 minutes.
The full check remains a failed run.
The Git implementation and its journey were left unchanged for their assigned builder.
This intermittent failure remains open.

## 737. Git message generation has no configured execution boundary.

Triage (2026-11-23): open, P2, engine, high confidence. The engine cannot generate commit or pull request text because no bounded model task with a chosen account and model exists. Basis: No generate-message request in packages/contracts/src; packages/studio/src/git.ts:48 keeps gitModel as a placeholder.

State: open. Checked against the code on 2026-11-15. packages/studio/src/git.ts line 47 still sets gitModel to an empty placeholder, so Git message generation keeps its refusal. Kind: visible to the owner.

ProjectSettings has no Git text-model or account selection.
The live service mapping's gitModel value is an empty placeholder.
The runner has a temporary discovery operation, but it sends no prompt and returns only choices.
Ordinary prompts require a registered agent, an incarnation, a delivery, and durable thread routing.
There is no bounded short task that returns generated Git text with an owner-selected account and model.
Commit and pull request generation therefore retain their existing refusal.
This assignment adds no implicit account selection and sends no repository content to a model.

## 738. Hosted pull request text editing has no authenticated provider update action.

Triage (2026-11-23): open, P2, git, medium confidence. The owner cannot manually edit a hosted pull request's title or description from the page. Basis: docs/issues.md line 19478 says manual editing in issue 738 remains unavailable; gh pr edit runs only inside Prepare (git/index.ts:1104).

State: open for manual title and description editing.
Checked against the code on 2026-11-17 after the s27 change.
Kind: visible to the owner.

The review originally stored only a local approval record for a reviewed pair of heads.
The s27 change adds an optional GitHub delivery record with the pull request number, provider identity, title, description, and state.
Prepare now creates or finds that pull request with redacted text from the project and reported evidence.
Approval now merges that pull request when GitHub is available.
No authenticated provider text update operation exists in the engine.
The title and description controls therefore retain their refusal of manual live edits.
A bounded authenticated provider update action is still required to connect those editors.


## 750. Builder Git reads currently have project-wide access.

Triage (2026-11-23): waits for the owner's decision, P2, security. Any agent in a project can read other teams' builder worktrees through the Git read requests; the owner must rule on it. Basis: Entry: policy recorded for the owner to rule on, pinned by a wire regression; no later entry changes it.

Owner ruling (2026-11-25): Agents in one project may read the builder worktrees of other teams. This is accepted and is to be documented. Because the server and the client are being rewritten in Go, this ruling is an input to that design.

State: open. Checked against the code on 2026-11-15. Policy on project-wide builder Git read access is recorded for the owner to rule on and no later entry changes it. Kind: waits for a decision.

The workspace mapping added in df9e486 widened the existing project Git read scope to builder worktrees under registered teams.
Every authenticated agent assigned to that project can read those builder worktrees through changes, diff, history, commit.files, git.state, review, and conflict.
This includes a project agent in another team.
An outsider cannot read them by supplying the project identifier.
A project agent cannot use those requests to read the repository base checkout.
The owner can read registered local workspaces independently of project membership.
A wire regression pins these cases across all seven read requests.
The policy is recorded for the owner to rule on and is not changed by this review fix.

## 752. Builder integration holds the source workspace operation lane.

Triage (2026-11-23): waits for the owner's decision, P2, git. Merging a builder into its team holds the builder's lock, not the team's, so another team command could collide. Basis: Entry: source lane retained for the owner's ruling; no later entry changes it.

Owner ruling (2026-11-25): Merging a builder into its team holds both locks. Because the server and the client are being rewritten in Go, this ruling is an input to that design.

State: open. Checked against the code on 2026-11-15. No later entry changes the lane; the entry says it was retained for the owner's ruling, and packages/engine/src/project/README.md still describes the workspace operation lane. Kind: waits for a decision.

Builder-into-team integration mutates the team checkout while holding the builder's operation lane.
The shared Git gate bounds individual external operations but does not make the team lane its operation owner.
This can matter when another command targets the team during the integration.
The review explicitly requested recording this issue without changing the lane.
The source lane is retained for the owner's ruling.

## 760. The Git journey failed once at "Use both" under load.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. The Git journey once failed to find the Use both button after a conflict and refresh. Basis: Issue 836 found and fixed the cause: the conflict view showed Ready before the selected file's conflict parts were read.

State: open. Checked against the code on 2026-11-15. One-time load-sensitive Git journey failure with unknown cause; later issue 1044 records further load-sensitive failures of the same retired-page Git journeys. Kind: tests.

The full check on `85cf37e` failed at `packages/web/e2e/support/studioGit.ts:157`.
The "Use both" button did not appear within ten seconds after an external merge conflict and a refresh.
The same journey passed when it ran alone on the same commit.
The cause is not known.

## 761. Cursor has no compliant automatic recipe.

Triage (2026-11-23): open, P3, harnesses and agent running, high confidence. Cursor cannot be installed from the page; the owner must run its official installer by hand. Basis: packages/engine/src/harnesses/recipes.ts:37-40 sets automatic false for Cursor with a manual curl command.

State: open. Checked against the code on 2026-11-15. packages/engine/src/harnesses/recipes.ts:37 still sets automatic false for Cursor with a manual curl command and a reason text. Kind: visible to the owner.

Cursor's official installation page documents a download piped into a shell.
The assignment forbids that installation method inside LGTM Studio.
The recipe therefore exposes the official command for the owner to run manually.
The Install button stays disabled for Cursor.
The owner can run the command and use Check again.
An automatic recipe needs an official method that satisfies these limits.
The source is https://cursor.com/docs/cli/installation.

## 769. Managed executable files need an installation receipt.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. Managed harness programs were run without checking they were unchanged; an installation receipt now guards that. Basis: Entry says the installer now records hashes and refuses a changed tree; the remaining same-user limit is issue 783.

The first status read executed any file found in the managed harness folder.
The installer now records the real executable path and hashes every regular file and executable link in its dependency tree.
The receipt lives in the engine state folder, outside the installation prefix.
Status and runner entry points refuse a changed tree before executing it.
Version reads happen after installation or an explicit owner refresh.
Ordinary status reads use saved versions.
This check does not provide an operating-system boundary against another process with full access to the engine user's state and source files.
A hostile process running as the same user can rewrite the receipt and bless a tampered program.
The "changed since install" check guards against accidents and changes by install scripts.
It does not stop a hostile process running as the same user.

## 772. An unreadable login file must not break every harness section.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. One unreadable login file used to break every harness section of the status. Basis: Entry: status now handles a malformed or unreadable login store per harness and other sections remain available.

Status now handles a malformed, oversized or unreadable login store per harness.
It reports that the sign-in state cannot be read.
Other harness sections remain available.
A subprocess probe still requires readable credential sources so its output can be redacted safely.

## 775. Automatic installs do not support inherited proxy settings.

Triage (2026-11-23): waits for the owner's decision, P3, harnesses and agent running. Automatic harness installs ignore proxy settings, so they can fail behind a required proxy. Basis: No proxy handling in packages/engine/src/harnesses/programs.ts; entry asks for an owner-controlled design.

Owner ruling (2026-11-25): Closed as a record, until someone needs a proxy. The place in the code where proxy support would be added is marked with a note. Because the server and the client are being rewritten in Go, this ruling is an input to that design.

State: open. Checked against the code on 2026-11-15. installerEnvironment in packages/engine/src/harnesses/programs.ts:35 builds a fixed environment and no proxy setting exists; the entry itself says the matter is open. Kind: waits for a decision.

This matter is open.
The installer deliberately drops proxy variables and npm configuration variables.
An installation behind a required proxy can fail.
A future proxy setting needs an explicit owner-controlled design that preserves the fixed recipe and redaction rules.

## 782. The admin browser journey intermittently misses token clearing.

Triage (2026-11-23): open, P3, tests, low confidence. The older admin page's journey sometimes misses the token-cleared label and passes when rerun alone. Basis: Entry: isolated rerun passed; the journey drives the retired packages/web admin page. It lives only in a screen that is being rebuilt.

State: open. Checked against the code on 2026-11-15. Intermittent admin journey failure on the token-cleared label, passing on isolated rerun; no later entry records a fix. Kind: tests.

The full browser run passed 50 journeys and failed two journeys.
The admin journey failed while waiting for its token-cleared label.
An immediate isolated rerun passed without a source change.
This matter remains open because notifications are outside this builder's assignment.
The other failure was the permission-picker test location recorded above.

## 783. A same-user process can replace the installed harness despite verification.

Triage (2026-11-23): open, P2, security, high confidence. A hostile program running as the same user can swap the installed harness despite the installation check. Basis: Entry: the real fix is the planned agent sandbox; verification.ts only compares files the same user can write.

State: open. Checked against the code on 2026-11-15. Same-user replacement is a recorded limitation awaiting the planned agent sandbox; verification.ts only checks files and does not form a boundary against the same user. Kind: security.

This matter is open.
Every input the installation verification reads is in a folder that a process running as the same user can write.
Such a process can restore the staged files and rewrite npm's hidden lockfile before the check, or rewrite the program after extraction.
It can therefore still have a different program installed, run by the engine, recorded in the receipt, and proposed as the harness.
The check stops accidents, install scripts, and a swap that is not restored.
It is not a wall against a hostile process running as the same user.
The real fix is the planned sandbox for agents.
The check compares staged files with the committed files and installed package metadata with the pinned entries.
Only optional entries may be absent.
Every present entry must match its name, version, resolved location and integrity hash.
An extra entry or an executable outside the expected package folder is refused.

## 790. Whole-tree receipt checks can block the engine.

Triage (2026-11-23): open, P2, engine, high confidence. The engine hashes the whole installed harness tree while blocking other work, on every status read and agent launch. Basis: packages/engine/src/harnesses/programs.ts:90,108 still use readFileSync and readdirSync for the tree digest.

State: open. Read on 2026-11-15, not checked against the code. The entry says this is open; programs.ts still imports synchronous readFileSync, readdirSync and statSync for tree hashing.

This matter is open.
The engine hashes the whole managed tree synchronously on every status read and agent launch.
Large installations can therefore delay other work on the engine's event loop.

## 791. The page has no control to cancel an installation.

Triage (2026-11-23): open, P2, client api and command line, high confidence. A harness installation can run for ten minutes and the owner cannot cancel it. Basis: packages/contracts/src/client/commands.ts has harness.install but no cancel request.

State: open. Checked against the code on 2026-11-15. Grep of packages/engine/src, packages/studio/src and design/mock/js finds no cancel-install control or handler. Kind: visible to the owner.

This matter is open.
The installation can run for ten minutes.
The owner cannot cancel it from the page.
Engine shutdown and request interruption still stop its process group.

## 792. A symbolic link in the home path refuses automatic installation.

Triage (2026-11-23): open, P2, harnesses and agent running, high confidence. Automatic harness installation is refused when the home folder is reached through a symbolic link. Basis: packages/engine/src/harnesses/verification.ts:32 still requires realpathSync(folder) === folder.

State: open. Checked against the code on 2026-11-15. packages/engine/src/harnesses/verification.ts:32 still requires realpathSync(folder) to equal folder, so a symlinked home path is refused. Kind: visible to the owner.

This matter is open.
The managed root must equal its absolute real path.
A home folder reached through a symbolic link therefore refuses automatic installation.

## 793. npm can read configuration from the managed prefix.

Triage (2026-11-23): open, P3, security, medium confidence. An npm configuration file or cache inside the agent-writable install folder can affect or break a harness installation. Basis: Entry says open; post-install checks reject changed packages but give no same-user boundary; no later change found.

State: open. Checked against the code on 2026-11-15. Entry says open; the npm cache and project npmrc remain inside the managed root and no later entry reports a change. Kind: security.

This matter is open.
A project .npmrc placed in the managed prefix can affect npm or break an installation.
The npm cache is also inside the agent-writable managed root.
The post-install checks reject changed manifests or package metadata before executing the installed program.
Those checks do not create a boundary against another process running as the same user.

## 794. Gemini's real installation still needs an owner trial.

Triage (2026-11-23): waits for the owner's decision, P2, harnesses and agent running. The automatic Gemini installation has never been tried for real; the owner must try it before a release. Basis: Entry: owner must try a real installation manually; tests use fake package trees; no later entry reports the trial.

Owner ruling (2026-11-25): Leave it open until the owner installs Gemini once.

State: open. Checked against the code on 2026-11-15. Requires an owner trial of a real Gemini installation before release; automated tests use fake package trees and no later entry reports the trial. Kind: waits for a decision.

This matter is open.
No real vendor installation has run during this assignment.
Gemini CLI has not been verified with install scripts disabled.
It has not been checked for writes to its own package folder during normal operation.
The third review fixed the hidden-lock comparison to allow missing optional entries while still requiring all other entries.
The owner must try a real installation manually before release.
The automated tests use local fake package trees and never contact a registry.

## 795. Recipe citations cannot be verified from this repository alone.

Triage (2026-11-23): open, P3, documents, high confidence. Install recipes cite vendor web pages with no saved copy, so their claims cannot be checked from the repository. Basis: packages/engine/src/harnesses/recipes.ts holds only source addresses; no snapshots in the repository.

State: open. Checked against the code on 2026-11-15. Recipes in packages/engine/src/harnesses/recipes.ts hold only source addresses, and the repository has no page snapshots. Kind: internal.

This matter is open.
The repository records source addresses but does not include snapshots of those pages.
Their contents can change or become unavailable.
The recipe's documentation claims therefore cannot be confirmed from this repository alone.

## 801. Reviewer defaults have no automatic selector yet.

Triage (2026-11-23): open, P2, board and roles, medium confidence. The saved reviewer count and different-vendor rule do nothing, because no engine code picks reviewers. Basis: Grep of packages/engine/src finds reviewers only in README and role instruction files, no selector code.

State: open. Checked against the code on 2026-11-15. Grep of packages/engine/src finds no reviewer selector function; defaults are stored but nothing assigns reviewers. Kind: visible to the owner.

The engine stores the default reviewer count and the different-vendor rule in project settings.
The page says that these values are saved for future reviewer assignment.
No engine code currently selects reviewers.
The defaults therefore do not yet assign a reviewer or enforce a vendor choice.
The first reviewer selector must read these defaults.

## 813. The merged full check failed while waiting for Use both.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. The merged full check failed once while the Git journey waited for the Use both button. Basis: Issue 836 fixed the Use both failure: the conflict view now says Loading until the selection is read.

State: open. Checked against the code on 2026-11-15. Records an intermittent ten-second click timeout in the Git journey at studioGit.ts:157 that passed on retry; no fix or cause recorded. Kind: tests.

The merged full check passed all 554 unit files.
It passed 8,392 unit tests and skipped two.
The browser run passed 54 journeys and failed the Git journey.
The failure was a ten-second click timeout at `studioGit.ts:157` while waiting for Use both.
This is a different location from the task's known intermittent failure at line 100.
The unchanged Git journey passed alone in 3.1 minutes.
The credit check passed after that retry.
The service-settings, email, new settings, and standalone sample journeys passed in the full run.
The full check remains a failed run.
The settings worker changed no Git timeout or implementation beyond the lead's required integration merge.

## 817. Restart executes the writable checkout under the same user.

Triage (2026-11-23): open, P2, security, high confidence. Restart runs whatever program is in the writable checkout, so an agent as the same user could change it first. Basis: packages/engine/src/client/serviceControl.ts:14 uses process.argv[1]; issues.md line 13552 says it remains subject to issue 817; sandboxes planned.

State: open. Checked against the code on 2026-11-15. Restart still runs the writable checkout via process.argv[1] in serviceControl.ts; line 13084 of the list says it remains subject to issue 817, sandboxes planned. Kind: security.

Restart runs the installed Node.js executable and the script currently at the recorded path.
An agent running as the owner's user can change that checkout before an owner requests restart.
The same-user limit described in SECURITY.md applies to these launch files too.
Existence, regular-file, read, and execute checks prevent an accidental missing launch file from stopping every agent.
They do not establish that the program is trusted or unchanged.
Agent sandboxes remain planned.

## 818. A stop signal during the restart window can still reexecute the service.

Triage (2026-11-23): open, P2, engine, medium confidence. A stop signal sent just after a restart request can still let the service start again. Basis: serviceControl.ts:107-112 sets pending true and clears it only on error; entry says signal behaviour was left unchanged.

State: open. Checked against the code on 2026-11-15. packages/engine/src/client/serviceControl.ts keeps the pending flag set and resets it only in Effect.onError, so a stop signal still leaves the restart flag set. Kind: internal.

The restart acknowledgement schedules shutdown after a short transport drain.
A stop signal during this window interrupts the service but leaves the restart flag set.
The process can therefore start again after that signal.
This review records the behavior without changing signal semantics in this fix.

## 834. Compressed design exports are not inspected for registered secrets.

Triage (2026-11-23): open, P2, security, medium confidence. A registered secret inside a compressed design file is not caught by the save check. Basis: Entry: the redaction check scans decoded bytes as text only; no later entry or code change found for .fig contents.

State: open. Checked against the code on 2026-11-15. Recorded limitation for the owner that compressed fig exports are not scanned for registered secrets; no later entry or code change resolves it. Kind: security.

The editor-save redaction check scans decoded export bytes as text.
A secret inside a compressed `.fig` export is not detected by that check.
The review found this limitation and asked that it be recorded for the owner.
A file signature check does not inspect compressed document contents.

## 839. The baseline thread journey observed no permission answer once.

Triage (2026-11-23): open, P3, tests, low confidence. A browser journey sometimes clicks a permission answer that never reaches the test runner. Basis: Seen in thread-browser.spec.ts here and oversized-state.spec.ts:187 in issue 893; both passed on rerun; no cause found.

State: open. Checked against the code on 2026-11-15. One-off thread-browser permission answer failure that passed on rerun; no cause found or fix recorded in any later entry. Kind: tests.

The unmodified d34b903 full browser run failed in thread-browser.spec.ts after its permission clicks.
The fake runner had zero answerPermission calls instead of one within five seconds.
The snapshot still showed the open permission request.
The next diagnostic full browser run passed that case.
The saved failure snapshot and output are retained in the Git worker's handoff.
The Git page changes do not change this thread implementation.
This finding remains outside the current Git assignment.


## 842. The global inspection selector does not call the Git detail reader.

Triage (2026-11-23): open, P2, website, high confidence. Picking a checkout from the global menu can show old file rows because it skips the fresh Git read. Basis: design/mock/js/app.js:203 still sets st.inspecting directly; only screens/git.js:731 calls liveGit.selectGit. It lives only in a screen that is being rebuilt.

State: open. Checked against the code on 2026-11-15. design/mock/js/live/git.js line 344 still sets s.inspecting directly from the global menu path without calling the Git detail reader. Kind: visible to the owner.

The global inspection menu in the copied app updates the selected branch directly.
The Git worktree picker calls the Git selection action and refreshes its records.
Selecting an unread checkout through the global menu can therefore show cached file rows without loading their details.
This path was already present on the assigned baseline.
The current assignment keeps out of the app source and does not change that global menu.
The owner can use the Git worktree picker or Refresh Git for a fresh read.

## 849. The Files tab shows text files up to 512 kilobytes, not 1 megabyte

Triage (2026-11-23): open, P2, client api and command line, high confidence. Text files over 512 kilobytes are not shown, though the feature list promises one megabyte. Basis: packages/contracts/src/project/git.ts:40: fileTextLimit = 512 * 1024.

Found: 2026-11-14, while building the Files tab on the branch `area/files-settings`.
State: open.

`docs/features.md` says a file over 1 megabyte is not returned as text.
The service's WebSocket accepts messages of at most 1 mebibyte (`packages/engine/src/client/web/server.ts`).
A save sends the whole text in one message, and a read reply escapes the text as JSON.
The built limit is therefore 512 kilobytes, set by `fileTextLimit` in `packages/contracts/src/project/git.ts`.
A larger text file shows "Too large to show".
Raising the limit needs replies and saves that are split across messages, or a larger message limit.

## 850. The Files tab cannot download a binary or large file

Triage (2026-11-23): open, P2, client api and command line, high confidence. A binary or large file cannot be downloaded because no request returns a file's bytes. Basis: No download request in packages/contracts/src or packages/engine/src; readFile returns only kind and size for large or binary files.

Found: 2026-11-14, while building the Files tab.
State: open.

`docs/features.md` says the client offers a binary or large file as a download, which the service streams from the machine.
No client request returns a file's bytes, so the tab only names the file's kind and size.
A download needs a streamed request with the same path checks as `project.files.read`.

## 851. The Files tab reads only worktrees on this computer

Triage (2026-11-23): open, P3, engine, high confidence. Files and Git work only for worktrees on this computer, until machine pairing is built. Basis: packages/engine/src/project/gitApi.ts:49 still refuses any machine other than local.

Found: 2026-11-14, while building the Files tab.
State: open.

Every Git and file request refuses a workspace whose machine is not `local` (`packages/engine/src/project/gitApi.ts`).
The tab therefore says "Files on this machine are awaiting machine support" for a worktree on another machine.
This lifts when machine pairing lands, as it does for the Git tab.

## 852. Saving a file in the Files tab works only on Linux

Triage (2026-11-23): open, P2, engine, high confidence. Saving a file from the Files tab works only on Linux and is refused on macOS. Basis: packages/engine/src/project/git/replace.ts:40-43 still pins folders through /proc/self/fd.

Found: 2026-11-14, while building the Files tab.
State: open.

The save shares the checkout file replacement in `packages/engine/src/project/git/replace.ts` with the mock editor.
It pins each folder through `/proc/self/fd`, which exists only on Linux, and refuses on other systems.
A save on macOS is therefore refused, and the page shows the refusal.
A macOS version needs another way to pin the folders, such as `openat` through a native helper.

## 857. An image file can carry a registered secret to the page

Triage (2026-11-23): open, P0, security, high confidence. An image file, including a text-based SVG, is sent to the page without the registered-secret check. Basis: packages/engine/src/project/git/index.ts:834-835 returns base64 image bytes directly; entry cites SECURITY.md rule 6; no later fix found.

Found: 2026-11-14, in the review of pull request 47.
State: fixed in unit f3-security. `project.files.read` now checks an image's bytes against the registered secrets, shows a Scalable Vector Graphics file that holds one as its redacted source, and reports any other such image as binary.
The reply-wide redactor already replaced a long secret inside the base64 text, which broke the image, but it missed a secret too short to form an eight-character base64 run.
The check guards against accidents, not against an agent that hides a secret on purpose.
These known forms pass the check:
a Scalable Vector Graphics file encoded as UTF-16, which is not valid UTF-8 and is checked only as Latin-1 bytes;
a secret written with character entities, such as `&#x41;`;
a secret split by markup, such as `ab<tspan/>cd`;
and a secret inside compressed data, such as a PNG `zTXt` or `iTXt` chunk or a compressed image body.

`project.files.read` checks a text file against the service's registered secrets before it replies.
An image is sent as base64 bytes with no check, and a Scalable Vector Graphics image is text.
A registered secret inside such a file reaches the owner's page, which rule 6 of `SECURITY.md` forbids.
The fix is to check the decoded text of a Scalable Vector Graphics file and to report it as private data when it changes.

## 863. The baseline Mocks browser journey did not see its editor-save confirmation.

Triage (2026-11-23): open, P2, tests, medium confidence. The Mocks journey sometimes misses the short-lived Editor changes saved notice in a loaded full run. Basis: Entry: failed twice at studioMocks.ts:89, passed five times alone; cause not proven; recurs in issues 889 and 898.

State: open. Checked against the code on 2026-11-15. Entry states the cause is not proven and state open; no later entry in issues.md mentions 863 as resolved. Kind: tests.

The baseline check passed 577 test files with 8,720 tests passed and two skipped.
The browser stage passed 56 journeys and failed the existing Mocks journey.
The assertion in `packages/web/e2e/support/studioMocks.ts:89` did not find "Editor changes saved" within five seconds.
The check ran before the sign-in implementation and before the required merge from the base branch.
The chained credit check did not run in that attempt.

The same journey failed once more on 2026-11-14, in the full check of the merged sign-in branch, at the same line.
It then passed five times in a row when it ran alone.
Both failures happened while other builders' runs shared the test computer.
The save notice appears only after the engine saves and the page reads the mocks and the design again, and it stays for under four seconds.
The cause is not proven. The state is open.

## 865. Cursor has no established variable for isolating its login credentials.

Triage (2026-11-23): open, P2, harnesses and agent running, medium confidence. A second Cursor account cannot be added because no known way isolates Cursor's login credentials. Basis: Entry: account creation refuses a second Cursor account; no later entry establishes credential isolation.

State: open. Checked against the code on 2026-11-15. packages/engine/src/runner/AcpRunner.ts line 425 still overrides CURSOR_CONFIG_DIR with the tool directory; no later entry establishes credential isolation. Kind: visible to the owner.

Cursor documents `CURSOR_CONFIG_DIR` for its configuration.
Its authentication documentation does not establish that this moves login credentials.
The existing adapter also replaces that configuration directory with its own temporary tool configuration.
The account creation request therefore refuses a second Cursor account with a fixed reason.
A separate Cursor sign-in remains unavailable until the credential isolation can be established.

## 875. The older page's code and tests remain after the service stopped serving it.

Triage (2026-11-23): open, P2, tests, high confidence. The retired older page's code and tests remain, because many journeys still test the engine through it. Basis: packages/web with src and e2e still exists in the repository; AGENTS.md says it stays until coverage is moved.

Found: 2026-11-14, when the owner asked for one website and not two.
Status: open.

The service now serves only the website built from `design/mock/`, at the root address.
An old address under `/studio/` is sent to the root.
The package `packages/web` is no longer served to anyone.
Its source and its tests are still in the repository.
Many browser journeys still drive the engine through the older page.
Removing the package now would remove that coverage of the engine.
Each journey needs an equal journey on the website before the package is removed.
The Files tab and the Project settings screen existed only on the older page.
Both screens are now on the website, from the units `s8-files` and `s8-psettings`.

## 878. A project lead cannot change its subscription.

Triage (2026-11-23): open, P2, client api and command line, high confidence. A project lead's subscription cannot be changed after it starts; only model and effort can. Basis: packages/contracts/src/commands.ts:75-76: configure payload holds only model and effort.

State: open. Checked against the code on 2026-11-15. The configure command in packages/contracts/src/commands.ts still takes only model and effort; no subscription change command exists. Kind: visible to the owner.

The engine reports the lead's account, but `configure` accepts only a model and an effort.
The live subscription picker shows the account and is disabled.

## 879. A project cannot set the permission for every agent.

Triage (2026-11-23): open, P2, client api and command line, medium confidence. A project cannot set the permission level for every agent in it. Basis: project.permission.set (commit 8c3c234f) covers only the lead's planning and starting-work settings, not every agent's permission level.

State: open. Checked against the code on 2026-11-15. packages/contracts/src/project/commands.ts has no project permission command; only preferences.set for shell exists. Kind: visible to the owner.

The engine reports each agent's permission, but has no project permission command.
The live permission choices are disabled.

## 880. A project cannot set the write-tests-first switch.

Triage (2026-11-23): open, P2, client api and command line, high confidence. A project has no write-tests-first setting in the engine. Basis: No such field in ProjectSettings or packages/contracts/src/project; only a workflow field in contracts/src/workflows.ts.

State: open. Checked against the code on 2026-11-15. No project write-tests-first setting in project contracts; the tests flag in contracts/src/workflows.ts is a workflow field, not a project command. Kind: visible to the owner.

The engine has no project setting or command for this switch.
The live switch is disabled.

## 881. A project cannot set the agent-review switch.

Triage (2026-11-23): open, P2, client api and command line, high confidence. A project has no agent-review setting in the engine. Basis: agentReview appears only in packages/contracts/src/workflows.ts, not in project settings or commands.

State: open. Checked against the code on 2026-11-15. No project agent-review command in packages/contracts/src/project; agentReview appears only in workflows.ts. Kind: visible to the owner.

The engine has no project setting or command for this switch.
The live switch is disabled.

## 882. A project cannot choose to remove agent credits when merging.

Triage (2026-11-23): open, P3, client api and command line, high confidence. A project cannot choose to remove agent credit lines when merging. Basis: Grep of packages/contracts/src finds no project preference for credit removal.

State: open. Checked against the code on 2026-11-15. grep of packages/contracts/src finds no project preference for removing agent credit lines. Kind: visible to the owner.

The engine has no project preference for removing agent credit lines.
The live switch is disabled.

## 883. A project cannot configure or approve the assignment table.

Triage (2026-11-23): open, P2, client api and command line, high confidence. A project cannot set or approve which harness, model and effort each kind of work uses. Basis: No assignment record or apply/approve command in packages/contracts/src/project/commands.ts.

State: open. Checked against the code on 2026-11-15. No assignment table record or apply and approve commands exist in packages/contracts/src/project. Kind: visible to the owner.

The engine has no project work-kind assignment record or commands to apply or approve it.
The live harness, model, and effort choices are disabled for each work kind.
The Every row picker, Apply button, and Approve button are disabled.

## 884. A team cannot change its need-to-know setting.

Triage (2026-11-23): open, P3, client api and command line, high confidence. A team's need-to-know setting does not exist in the engine. Basis: Grep of packages/contracts/src finds no need-to-know record or command.

State: open. Checked against the code on 2026-11-15. grep of packages/contracts/src finds no team need-to-know record or command. Kind: visible to the owner.

The engine has no team need-to-know record or command.
The live team switches are disabled.

## 885. A project cannot configure its task label prefix.

Triage (2026-11-23): open, P3, client api and command line, high confidence. A project cannot set a prefix for its task labels. Basis: Grep of packages/contracts/src finds no task label prefix setting.

State: open. Checked against the code on 2026-11-15. grep of packages/contracts/src finds no project task label prefix setting. Kind: visible to the owner.

The engine accepts labels on tasks, but exposes no project prefix setting.
The live task label prefix field is disabled.

## 888. The engine hides the specific reason for a refused lead configuration.

Triage (2026-11-23): open, P2, client api and command line, high confidence. When a lead's model or effort change is refused, the owner sees only 'command refused' and not the reason. Basis: packages/engine/src/api/server.ts commandError passes specific reasons only for project., start and roles commands, not configure.

State: open. Checked against the code on 2026-11-15. commandError in packages/engine/src/api/server.ts line 57-63 whitelists reasons only for project., roles commands; the configure command still gets command refused. Kind: visible to the owner.

The `configure` planner can refuse a model or an effort that is no longer available.
The public command error filter reduces that reason to "command refused".
The live project settings notice displays the public reason that the engine returns.
The browser journey checks that reason and confirms that the refused value does not persist.
The filter remains unchanged in this migration.

## 890. The project shell command has no control on the live project settings screen.

Triage (2026-11-23): open, P2, website, high confidence. The Project settings screen has no control for the project's shell, though the engine supports it. Basis: No shell control in design/mock/js/live/project-settings.js; project.preferences.set exists at contracts/src/project/commands.ts:66. It lives only in a screen that is being rebuilt.

State: open. Checked against the code on 2026-11-15. design/mock/js/live/project-settings.js has no shell control while preferences.set with shell exists in packages/contracts/src/project/commands.ts line 64. Kind: visible to the owner.

The engine supports `project.preferences.set` for a project's preferred shell.
The connected mock's project settings screen has no shell control.
This migration does not add one.

## 891. A goal update from another client replaces an unsaved goal draft without a notice.

Triage (2026-11-23): open, P2, website, medium confidence. A goal change from another client silently replaces the owner's unsaved goal draft. Basis: Entry: behaviour left unchanged in Round 1; no later entry mentions a fix. It lives only in a screen that is being rebuilt.

State: open. Checked against the code on 2026-11-15. Entry records the draft-replacement behaviour as unchanged; no later issue entry mentions 891 as fixed. Kind: visible to the owner.

The live project settings screen resets its goal draft when the engine reports a changed goal.
An update from another client can therefore replace an unsaved local draft.
The screen does not warn the owner before replacing that draft.
This behaviour remains unchanged in Round 1.

## 899. File conflict detection depends on an English engine refusal.

Triage (2026-11-23): open, P2, client api and command line, high confidence. The Files screen detects a changed file by matching the engine's English refusal sentence, which breaks if the wording changes. Basis: design/mock/js/live/files.js:107 still tests e.message.startsWith('the file changed since you opened it').

Found: 2026-11-14, in the first review of the connected Files screen.
State: open.

The Files screen recognizes a stale version by matching the engine's English refusal sentence.
A change to that sentence could prevent the comparison controls from appearing.

## 900. A file absent from a loaded listing can lack change controls.

Triage (2026-11-23): open, P3, website, medium confidence. A file beyond the 500-entry folder listing limit can open without its Commit control or change badge. Basis: packages/engine/src/project/git/index.ts:826 still caps a listing at 500 items; no later entry reports a change. It lives only in a screen that is being rebuilt.

Found: 2026-11-14, in the first review of the connected Files screen.
State: open.

The Commit control and the file's change badge depend on the file appearing in a loaded folder listing.
The engine limits a folder listing to 500 entries.
A file beyond that limit may have no Commit control or change badge when opened through search or a direct address.

## 901. The Files offline state has only a forced-store browser check.

Triage (2026-11-23): open, P3, tests, medium confidence. The Files screen's offline state is tested only by forcing the status, never by a real network interruption. Basis: Entry states open; no later entry mentions issue 901. It lives only in a screen that is being rebuilt.

Found: 2026-11-14, in the first review of the connected Files screen.
State: open.

The browser journey checks the Files offline state by closing the connection and forcing the store's connection status.
It does not test the transition caused by an actual network interruption.

## 902. Image Blob address revocation has no test.

Triage (2026-11-23): open, P3, tests, medium confidence. No test checks that the Files screen releases an image's temporary address when the image changes or the view closes. Basis: Entry states open; no later entry mentions issue 902. It lives only in a screen that is being rebuilt.

Found: 2026-11-14, in the first review of the connected Files screen.
State: open.

The Files screen revokes an image's Blob address when the image changes or the view unmounts.
No test verifies that revocation.

## 905. A stall-watch timer field came back empty after a reload in one full run.

Triage (2026-11-23): open, P3, website, low confidence. A service settings field once came back empty after a reload; the likely cause was fixed but the failure was never reproduced. Basis: State line: open until several full runs pass; likely cause fixed under issue 918 (docs/issues.md line 14648). It lives only in a screen that is being rebuilt.

Found: 2026-11-14, in the full check of `area/studio-default` at `cfcc4de`.
State: open until several full runs pass, because a likely cause was fixed under issue 918 but the failure was never reproduced. Verified against the code on 2026-11-16.
Earlier state: a likely cause is fixed under issue 918; the failure was never reproduced, so it stays open until several full runs pass.

The full check ran alone on the test computer.
It passed 591 unit test files and 60 of 61 browser journeys.
The journey "live service settings: timers, nudges, workflow folder, review defaults and upkeep" failed at `studioSettings.ts:51`.
After a reload, the field `sd-a` held its saved value 7.
The field `sd-w` was empty, where 17 was expected.
The same journey then passed 6 of 6 times when run alone.
The cause is not known.
Issues 727, 863, 889, 893 and 898 record similar single failures of save journeys in full runs.
Load alone does not explain this one, because nothing else was running.
One possible cause is a save of one field that overwrites the save of the field before it.
That cause has not been tested.
An owner who types quickly across these fields might lose one value in the same way.

## 906. Agent terminal watching and takeover need engine support.

Triage (2026-11-23): open, P2, harnesses and agent running, high confidence. The owner cannot watch or take over an agent's terminal, because the engine has no way to attach to it. Basis: Entry and issue 1129 (checked 2026-11-16) both say unbuilt; terminalRequests.ts opens project owner shells only.

State: open. Checked by s20-controls on 2026-11-16. Watch agent session and Take over agent session remain disabled with Engine work needed. Interactive terminal attachment and keyboard handoff are unbuilt. Agent Client Protocol mode has no terminal to attach. Kind: visible to the owner.

The project terminal API opens private owner shells only.
It cannot attach to an agent's terminal or transfer that terminal's keyboard.
The live Session actions menu draws Watch agent session and Take over agent session disabled.
Both controls show “Engine work needed” on hover.

## 937. The owner's feedback on the "Add account" dialog.

Triage (2026-11-23): open, P2, website, high confidence. The Add account dialog is confusing in five ways, and account rows show raw model identifiers and no usage figures. Basis: Owner asked for these to be noted, not fixed; belongs to the harness setup overhaul in docs/features.md. It lives only in a screen that is being rebuilt.

Found: 2026-11-14, by the owner, while adding a Claude Code account on the real service.
Status: open. The owner asked for these to be noted and not fixed yet.

The owner used the dialog to add an existing sign-in and reported five problems.

1. The "Login folder" area is unclear. The owner was unsure about it.
2. The folder dropdown and the note "the service creates a private folder for each separate sign-in" do not appear linked. The note shows whatever the dropdown says.
3. With "my existing sign-in" chosen, the dialog still shows a command to run. An existing sign-in needs no command.
4. The other controls stay disabled until the first field, the name, is filled in. Nothing says that a name is needed.
5. With a name filled in and the existing sign-in chosen, two buttons become enabled: "Sign in here" and "Add account". "Sign in here" should not be offered for an existing sign-in. "Add account" worked.

The orchestrator saw two more things on the "Harnesses" page afterwards.

6. The account row's "Model for new agents" picker shows raw identifiers, such as `opus` and `claude-fable-5-2`. The pickers above it show names, such as "Opus 5.5".
7. Both accounts show "Not reported" for the five-hour and weekly usage.

These belong to the overhaul recorded in `docs/features.md` under "A simpler way to set up harnesses and subscriptions".

## 938. A separate sign-in cannot be chosen for a Cursor account.

Triage (2026-11-23): open, P2, harnesses and agent running, high confidence. A Cursor account cannot be given its own separate sign-in folder, so only one Cursor sign-in is possible. Basis: packages/engine/src/accounts/loginHome.ts: no homeVariable for cursor throws the unsupported reason; a second cursor account is refused.

Found: 2026-11-14, by the owner, while adding a Cursor account on the real service.
Status: open. The owner asked for it to be fixed later.

The "Add account" dialog disables "A separate sign-in" for Cursor.
The dialog allows it only for Claude Code, Codex and Gemini CLI, in `design/mock/js/screens/service.js`.
The engine refuses it too, with "Separate sign-in folders are not supported by this harness.", in `packages/engine/src/accounts/loginHome.ts`.
The cause is that those three harnesses have a variable that moves their login folder, and the engine knows none for Cursor.
The owner's view is that every harness only needs its home folder overridden.
Starting Cursor with a different `HOME` would give it a separate login folder.
One cost must be settled first.
A different `HOME` also hides the owner's Git settings, SSH keys and other tool settings from that agent, unless they are linked or copied in.
The dialog does not say why the choice is disabled.
This belongs to the overhaul in `docs/features.md` under "A simpler way to set up harnesses and subscriptions", and to "One agreed layout for the `~/.lgtm` folder".

## 939. The browser tab crashed with "error code 9" during the owner's first real project.

Triage (2026-11-23): open, P1, website, low confidence. The browser tab crashed three times during a real project, probably from the chat screen redrawing too much; never reproduced. Basis: Entry states cause unknown and not reproduced; no later entry mentions issue 939. It lives only in a screen that is being rebuilt.

Found: 2026-11-14, by the owner, on the real service, three times in a few minutes.
Status: open. The cause is not known, and it was not reproduced.

The owner started a new project with a Claude Code lead.
The browser tab crashed with "error code 9" while the lead was working.
It crashed a second time soon after.
It crashed a third time when the owner switched between two agents.
After the first crash, the lead showed only its working dots for a while, and then replied.
Chrome shows error code 9 when the tab's process is killed, which most often means that it used too much memory.
The orchestrator tried to reproduce the crash with a separate headless browser on the same service.
Every main screen stayed stable for 15 to 20 seconds, with no growth in memory or in page elements.
Switching between the two project leads' chats ten times did not crash the tab.
One thing was unusual on the chat screen.
The page's memory moved between about 20 and 66 megabytes within one or two seconds, while nothing visible changed.
That points to the chat screen rebuilding a large amount of content again and again.
A long reply or a long approval card that arrives piece by piece could multiply that work.
This has not been tested, because no agent was writing during the attempt.
The page asks for `accounts`, `client.agent.configuration` and `client.agent.context` again every five seconds on every project screen.

## 940. A new project took its name from the first words of its goal.

Triage (2026-11-23): open, P2, website, medium confidence. A new project is still named from the first words of its goal instead of a name the owner chooses. Basis: design/mock/js/live/board.js line 49 still calls nameFromGoal(input.goal); the lead instruction now only offers to suggest a rename.

Found: 2026-11-14, by the owner, on the real service.
Status: open.

The owner gave a new project a goal whose first sentence was about wiring up the whole LGTM Studio website.
The project was named "Goal here is fully", after the first words of that goal.
The owner called the name weird, and had expected the goal to be kept as the thing to build.
The name should be chosen by the owner, or be a short sensible title that the owner can change.

## 942. The project lead felt slow to reply.

Triage (2026-11-23): open, P2, harnesses and agent running, medium confidence. The project lead's first reply feels slow, the delay has never been measured, and the page shows only dots while waiting. Basis: Entry says nothing was measured; no later entry records a measurement.

Found: 2026-11-14, by the owner, on the real service.
Status: open. Nothing was measured.

The owner's first messages to a Claude Code lead took long enough that the owner thought the lead was stuck.
The lead did reply in the end.
The time from sending a message to the first visible sign of an answer should be measured, and the page should show more than dots while it waits.

## 944. The first real project lead went straight to sprints.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. The first project lead skipped the Define step, went straight to sprints, lacked rename and delete tools, and had a persona name. Basis: lifecycle.ts projectLeadInstruction and leadName "Project lead"; project.rename, team.delete, task.delete commands exist; PR 102 define-redesign. Points 3 and 4 not verified.

Found: 2026-11-14, by the owner, in the project "Goal here is fully" on the real service. The orchestrator then read the lead's whole chat.
Status: open.

The owner expected the first step to be: understand what the owner wants, gather context, and agree on it.
The lead's opening instruction is one line in `packages/engine/src/project/lifecycle.ts`: "You lead project <name>. Shape its first plan with the owner."
Nothing in it describes a Define phase, and nothing tells the lead to ask before it plans.
This is what the lead did, with the times the chat shows.

- 18:00 to 18:09: it read the code and the build-loop plan by itself. It posted one status line at 18:06. This silent reading is the wait that the owner felt as slowness (issue 942).
- 18:09: it asked one question that held its findings, a five-sprint proposal and a list of things left out. It asked the owner nothing about what the goal meant.
- 18:09, after "Accepted": within a minute it created five sprints, one team named "Build team", and four tasks.
- 18:10: it asked to build sprint 1 by itself. A lead is meant to give work to builders.
- 18:10: the owner told the lead to hold on and not to create teams yet. The lead stopped. It changed no code and made no commit.

Problems this shows:

1. The lead needs a Define step that the product enforces: restate the goal, ask what is unclear, and agree on an end result before any sprint, team or task exists.
2. Creating sprints, a team and tasks happened on one "Accepted" for a scope question. The owner wants the start of building to be its own approval.
3. The lead said its role advice forbids it to start agents. That sentence is the worker's advice in `packages/contracts/src/roles/role.ts`. The lead's advice says "Start your own workers". The project lead may be given the worker's advice. This is not confirmed.
4. A task cannot be created without a team, so the lead made a team only as a placeholder.
5. The lead has no tool to delete a team or a task, or to rename the project. The owner has to remove "Build team" and its four tasks by hand.
6. The lead was named "Rowan" from the old name list in `packages/engine/src/project/lifecycle.ts`. Agents are meant to be named by role.

## 945. The lead's chat showed noise and lost formatting.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. The lead's chat showed duplicated text, lost lists, raw tool names and repeated file changes. Basis: Status line: rendering defects fixed in the s12 chat change; session-start notices remain with issue 946.

Found: 2026-11-14, by reading the same chat.
Status: the rendering defects are fixed in the s12 chat change.
The session-start notices remain open with issue 946.

1. The question card showed the same long text twice, once as the summary and once under "THE QUESTION". The short version was not short.
2. The lead wrote lists, and the card showed them as one paragraph with dashes in it.
3. Tool calls appear as raw names, such as `mcp__lgtm__project_sprint_create`, five times in a row.
4. One file change was shown three times, each time as a full raw difference.
5. A reply stamped 18:06 appears after the question stamped 18:09.
6. The lead told the owner about two notices from its own session start that have nothing to do with the project.

## 946. An agent inside LGTM Studio shares this machine's Claude Code memory and hooks.

Triage (2026-11-23): waits for the owner's decision, P1, harnesses and agent running. An agent in a project reads and writes the owner's ordinary Claude Code memory and runs the owner's hooks. Basis: Status: needs an owner decision; issue text at docs/issues.md line 20700 still cites it; no isolation setting found in packages/engine/src/harnesses.

Owner ruling (2026-11-25): The recommendation of a separate settings home for each agent is not taken as written. All agents have one source for their memory files, possibly a database of memories. The aim is that a change of account, and a fleet of several machines, share the same memories. The owner noted that this may be hard to build. Because the server and the client are being rewritten in Go, this ruling is an input to that design.

Found: 2026-11-14, by reading the same chat.
Status: open. It needs a decision by the owner.

The lead ran with the owner's ordinary Claude Code settings.
It read the memory notes of the sessions that build LGTM Studio in a terminal.
It wrote a new note, "Lead creates no teams unasked", into that same memory folder and its index.
It also ran the owner's session-start hooks and reported their notices in the project chat.
So an agent in a project and a terminal session can change each other's standing rules without either one being told.
The design of the `~/.lgtm` folder should decide which settings, memory and hooks an agent in a project gets.

## 966. Five browser journeys still failed with their restored limits.

Triage (2026-11-23): open, P2, tests, medium confidence. Several browser journeys fail their fixed five-second waits or the six-minute limit when the computer is busy, and pass alone. Basis: Status: open for timing investigation; issues 1014, 1044, 1122, 1139, 1166 repeat it; issue 1033 says the Git journey still runs near its limit. It lives only in a screen that is being rebuilt.

Found during fix round 1 on 2026-11-14.
Status: open for further timing investigation.
The restored five-second assertion expired while two legacy Git merge dialogs still showed a pending submission.
The cases were an owner approval into main and an approval with stale reported evidence into master.
The broader projects and board journey also remained on All projects five seconds after pressing Create project.
The equivalent simple master merge and the dedicated creation journeys passed.
These failures do not establish which engine or page phase was slow.
The website Git journey reached its six-minute overall limit while a refresh was busy after merging.
The Files journey still showed its save as busy when the five-second assertion expired.
The full suite passed fifty-eight journeys and failed these five in 15.8 minutes with two workers.
The unchanged one-worker rerun passed both legacy Git cases, project creation and Files.
Their times were 17.7 seconds, 29.7 seconds, 50.7 seconds and 35.9 seconds respectively.
Website Git again reached its six-minute limit, with a refresh and diff still loading.
The rerun passed four journeys and failed one in 8.3 minutes.
The earlier one-worker website Git measurement passed in 4.5 minutes.
The later failures leave its timing unresolved.
No assertion or operation deadline is widened.
Issues 928 and 936 record the completed workspace and save optimizations.
The orchestrator supplied a baseline on 2026-11-14 from master before this unit.
The website Git journey also reached the 360-second limit on this computer with one browser worker.
That run used 303 seconds of system time and 116 seconds of user time.
The timeout therefore predates this unit.
A few unit tests ran at the same time, so this is one loaded sample.

## 970. A shell refresh can swallow Clear token in the retired admin page.

Triage (2026-11-23): open, P3, tests, medium confidence. In the retired admin page a background refresh can swallow a click on Clear token, making its journey fail about one run in four. Basis: Status: open in the retired page under packages/web; reproduced on master at 2518bfc; the page is retired but its journeys still run. It lives only in a screen that is being rebuilt.

Found during investigation round 3 on 2026-11-14.
Status: open in the retired page under packages/web.
The orchestrator measured five failures in thirteen runs on this branch.
The orchestrator measured no failures in nine runs on master at 2518bfc.
Those runs used this computer and one browser worker.
Temporary instrumentation reproduced one failure in five more branch runs.
The failing assertion still expected No token set within five seconds.
After removing the instrumentation, fifteen repetitions passed seven times and failed eight times in 2.3 minutes.
Five failures reached the No token set assertion at admin-pages.spec.ts:88.
Three failures stopped earlier at line 85 because saving on blur left the New token input populated.
Those three earlier failures were not instrumented, so their event sequence is not established by the Clear token trace.

The failing trace records the exact interleaving using milliseconds from the browser's performance clock.
At 731.1 milliseconds, settings committed busy false, offline false, uncertain false and tokenSet true.
At 746.8 milliseconds, the event stream delivered client.AlertDeliveryFailed at sequence 29.
At 747.3 milliseconds, the watcher requested client.snapshot.
That request succeeded at 750.9 milliseconds with sequence 29.
The shell still held collection reads from sequence 28, so complete became false.
At 754.7 milliseconds, settings committed offline true and locked true.
The busy and uncertain flags both remained false.
The connection stayed Live, and its generation stayed zero.
Here offline is the local name of useMutationDisabled, which also covers incomplete collection reads.
It does not establish a network outage.

At 762.3 milliseconds, the shell sent client.requests as request 32 on socket 18.
At 763.5 milliseconds, Clear token received pointerdown while that read was in flight.
The button's own disabled property was false.
Its ancestor fieldset's disabled property was true, and the button matched :disabled.
The same state held at pointerup at 763.7 milliseconds.
No mousedown, mouseup, click or clearToken handler entry followed.
Chromium suppressed the activation after the fieldset became disabled during the click.
No token-clear request was sent.
The client.requests read succeeded at 766.5 milliseconds with sequence 29.
The subsequent client.alerts.list and client.sequence reads succeeded.
At 772.9 milliseconds, settings was unlocked again, with Token set and Carrier token saved. still displayed.
Waiting longer for the assertion cannot recover the missing action.
A passing control trace records all five pointer and mouse events, the unlocked clearToken handler, its token-clear request and a successful reply.

The uninstrumented settings page, admin API helper, ShellDataProvider, watchSituation and journey are byte-identical to master at 2518bfc.
The alert delivery, reactor and token command files are also byte-identical to that revision.
The watcher refreshes its snapshot for every client.Alert event.
ShellDataProvider disables mutations until the new snapshot and collection reads agree.
That existing sequence permits the captured loss of activation on master too.
The finite run counts alone do not establish why the earlier branch sample failed more often.

The setup's ordinary ask succeeds with only text and its question appears in the browser.
The new approvalQuestion path immediately returns without additional approval metadata for that ordinary ask.
The optional impossible projectApprovalEffect field permits omission.
The journey never answers the question, so its approval answer planner is not invoked.
The captured failure involves a successful refresh after an alert event, rather than a refused setup command, a failed collection read or a reconnect.
No retired-page source, journey assertion or wait was changed for this investigation.
Temporary traces contained request names, event tags, sequence numbers and lock flags, without request payloads or token values.

The orchestrator then ran the same journey fifteen times in a row on `master` at `2518bfc`, with one worker.
Four of the fifteen runs failed.
Two failed at line 88, one at line 85 and one at line 96.
So `master` shows the same failures on this computer, and the earlier nine clean runs were too small a sample.
The lead definition unit did not introduce this race.

## 984. The committed harness recordings do not cover tool calls.

Triage (2026-11-23): open, P2, tests, medium confidence. The saved harness recordings contain only greetings, so no test uses real tool call, edit or question traffic. Basis: Entry states open and that tests use constructed protocol inputs; no later entry mentions issue 984.

Found: 2026-11-15, while fixing issue 945.
Status: open.

The Claude Code and Codex recordings in `packages/testing/fixtures/` contain read-only greeting exchanges.
Their prompts explicitly prohibit tools and file changes.
They cannot verify the titles or payloads that current real harnesses send for edits, reads, commands, searches, or questions.
The chat regression tests use the engine contracts and the tool shapes already exercised by the runner tests.
A recorded tool session from each harness remains needed.
For Claude Code in Agent Client Protocol mode, the recording must include an edit with several fragments for one path and its completing update.
It must show whether later updates repeat snapshots or append fragments.
It must also cover a diff-first file write and the raw tool titles and command arguments.
The repository still has no real recording of that multi-hunk shape.
The round 1 tests use explicitly constructed protocol inputs, rather than invented recordings.

Unit s15-chat2 inspected the installed `@agentclientprotocol/codex-acp` version 2.6.0.
Its `commandActionFacts` reports quoted read and list titles.
Its unknown command report retains the full command in `rawInput.command`.
Its `McpToolReporter` names a call `mcp.<server>.<tool>`.
Its renderer sends these fields as Agent Client Protocol tool updates.
The unit also inspected the installed `@agentclientprotocol/claude-agent-acp` version 0.99.1.
Its read reporter supplies a display path, line range and locations.
Its Bash reporter uses the command as the title.
Its renderer carries the tool input as `rawInput`.
The new tests reconstruct these shapes with synthetic paths and arguments.
Their comments distinguish adapter-derived reports from constructed command edge cases and question events.
They are not harness recordings.

A real Codex project-lead recording must include `ask`, its recommendation, the later reply and the owner's answer.
It must include the durable `QuestionAsked` and `ThreadItem` events, with their sequences and times.
The orchestrator supplied the observed three-paragraph message in s15 fix round 1.
It begins with the bold question title, continues with a recommendation, and ends with "Sent through LGTM Studio.".
That final sentence is the lead's own announcement.
No engine delivery code produces it.
The engine's ask planner records a question without writing a board post.
For an owner holder, routing emits only `OwnerNeeded`.
A prompt delivery is queued only when another agent holds the question.
The website maps agent messages only from thread text events.
It does not turn board posts into agent messages.
A board-originated tool result can therefore only appear as lead speech if the harness emits it again as agent text.
A real event export is still needed to confirm the message's tool correlation and turn boundaries.
A real Claude Code question recording is also still needed.

## 1016. The engine has no safe worktree removal operation.

Triage (2026-11-23): open, P2, git, high confidence. The engine cannot safely remove a worktree, so deleting a project leaves all its folders and branches behind. Basis: grep of packages/engine/src finds no worktree removal operation; entry says the requirement lives in docs/features.md.

State: open. Checked against the code on 2026-11-15. grep of packages/engine/src finds no worktree removal operation; the entry says it is open and the requirement lives in docs/features.md. Kind: internal.

Found while implementing project deletion in unit s14.
The Git service can create a worktree but has no removal path that verifies engine ownership and refuses dirty files.
The creation claims do not supply a completed-worktree removal interface.
Project deletion therefore preserves all folders and branches.
The optional folder removal checkbox is omitted, as the task permits.
The requirements for a future operation are recorded in `docs/features.md`.
This is open.


## 1021. A short registered secret can invalidate a board reply mode during redaction.

Triage (2026-11-23): open, P2, security, medium confidence. A short registered secret such as "one" can alter a board post's fixed reply mode during redaction, so the post is refused. Basis: packages/engine/src/log/redactionStructure.ts shows no explicit position for the board reply mode; no later entry mentions issue 1021.

State: open. Checked against the code on 2026-11-15. No later entry or code records an explicit redaction position for the board reply mode; the entry says it is open. Kind: internal.

Found while extending the deletion regression fixture in fix round 1.
Registering the synthetic secret `one` changes the fixed reply mode `none` in a board post event.
The real writer then refuses the event because its redacted reply mode fails the event schema.
The deletion fixture now registers complete identifiers to isolate the requested identifier exemptions.
The board event's fixed reply mode still needs its own explicit redaction position and regression test.
No board redaction rule is changed by this unit.
This is open.


## 1022. A terminal can finish opening after project deletion commits.

Triage (2026-11-23): open, P3, engine, medium confidence. A terminal can finish opening just after its project is deleted, leaving a shell in an unregistered folder. Basis: harnesses/terminalRequests.ts resolves the workspace before manager.start and does not reread it; recorded without a fix by brief.

State: open. Checked against the code on 2026-11-15. Recorded without a fix by brief; no later entry mentions issue 1022 and the entry states this is open. Kind: internal.

Recorded from minor 2 of review round 2.
The terminal request resolves its workspace before the session enters the in-memory session map.
Environment redaction and loading the terminal module leave a gap before that insertion.
Deletion can check for terminals and commit during that gap.
The terminal can then open in a folder whose registration was removed.
The folder remains on disk, and the terminal remains restricted to its authenticated owner connection.
This is a consistency gap rather than a new access gap.
A future fix should reread the current workspace after session start and close the session through the existing close path if its registration is gone.
The round 2 brief explicitly asks to record this issue without fixing it.
This is open.

## 1023. The tombstone gate matches metadata field names rather than ownership.

Triage (2026-11-23): open, P3, engine, medium confidence. The deletion filter matches field names, not ownership, so replay could drop a surviving record that merely mentions a deleted one. Basis: Recorded without a fix by the round 2 brief; current commands cannot write the case; no later entry mentions it.

State: open. Checked against the code on 2026-11-15. Recorded without a fix by brief; no later entry mentions issue 1023 and the entry states this is open. Kind: internal.

Recorded from minor 6 of review round 2.
The gate can drop a surviving agent's prompt event when its sender names a deleted agent.
It can also drop a surviving project's workspace event when its parent identifier names a deleted workspace.
The current authenticated commands cannot write those two cases, but raw replay can contain them.
An identifier under an unrecognised field or nested object can pass the same gate.
A future contracts test should check every event schema's identifier fields against the matcher.
Ownership rules also need to distinguish a record's owner from its references.
The round 2 brief explicitly asks to record this issue without fixing it.
This is open.

## 1026. A shell wrapper with a multi-part quoted argument still shows in a chat tool title.

Triage (2026-11-23): open, P3, website, medium confidence. A chat tool title still shows the shell wrapper when the command is written as several joined quoted pieces. Basis: packages/studio/src/chatTools.ts line 59 strips a wrapper with one quoted argument via unquote; no parser for adjacent quoted pieces. It lives only in a screen that is being rebuilt.

Found: 2026-11-15, on the real service after pull request 62 was merged.
Status: open.

The chat removes a shell wrapper from a tool title only when the wrapper has one quoted argument.
Codex wrote one command as `/usr/bin/zsh -lc` followed by an argument made of several quoted pieces joined together.
The pieces alternate between double quotes and single quotes, to carry a literal exclamation mark.
The title of that row still begins "Ran /usr/bin/zsh -lc".
The matcher of issue 1012 leaves this form alone on purpose, because it does not parse shell quoting.
A fix needs a small parser for adjacent quoted pieces, and tests for each quoting form.


## 1028. Default builder registrations below a kept team worktree have no exclusive owner.

Triage (2026-11-23): open, P3, engine, medium confidence. Builder folder registrations carry no owning project, so project deletion leaves them behind under a shared worktree. Basis: Entry records a documented and tested limitation; no later entry resolves it.

State: open. Checked against the code on 2026-11-15. Entry records a documented and tested limitation for unmarked builder registrations; no later entry resolves it. Kind: internal.

Recorded from minor 1 of review round 3.
A project's team can sit on a worktree shared with a surviving project.
The workspace registration command records the builder's parent but no owning project or team.
Deletion cannot distinguish that builder from another project's builder under the same root.
Those unmarked builder registrations remain, as a documented and tested limitation.
A row with an exclusive team identifier from the deleted project now starts its own removal walk.
The workspaces named by surviving projects and teams still form retention barriers.
The missing ownership on default registrations remains open.

## 1029. A surviving project identifier alone does not protect a workspace below a removed root.

Triage (2026-11-23): open, P3, engine, medium confidence. Deleting one project could remove a workspace marked for a surviving project if it sits below the removed root. Basis: Recorded without a fix by the round 3 brief; current commands cannot construct the case.

State: open. Checked against the code on 2026-11-15. Recorded without a fix by brief; no later entry mentions issue 1029 and current commands cannot build the case. Kind: internal.

Recorded from minor 2 of review round 3.
A raw workspace row can carry a surviving project's identifier below a root removed by another project's deletion.
The kept set protects worktrees named by surviving project and team rows, rather than every workspace carrying a surviving project identifier.
The descendant walk can therefore remove and tombstone that marked workspace.
The current commands cannot construct this case because a marked workspace is its project's own worktree.
The round 3 brief explicitly asks to record this issue without fixing it.
This is open.

## 1030. A kept workspace can retain a parent identifier naming a removed workspace.

Triage (2026-11-23): open, P3, engine, medium confidence. After a deletion, a kept workspace can still point to a removed parent, so a later update of it could fail. Basis: Recorded without a fix by the round 3 brief; current commands cannot construct the layout.

State: open. Checked against the code on 2026-11-15. Recorded without a fix by brief; no later entry mentions issue 1030 and current commands cannot build the case. Kind: internal.

Recorded from minor 5 of review round 3.
A surviving team's kept workspace can sit below another project's removal root in a raw registry row.
Deletion keeps that workspace and its parent identifier while removing the parent workspace.
The registry remains readable and replay agrees with the live view.
A later update can fail because its parent is no longer registered.
The current commands cannot construct this layout.
The round 3 brief explicitly asks to record this issue without fixing it.
This is open.

## 1033. The Git screen's refresh scheduling was reworked four times and is not merged.

Triage (2026-11-23): open, P3, website, medium confidence. The Git screen sends too many refresh requests; four attempts to rework the scheduling failed review and were not merged. Basis: Status: work kept on branch area/s13-journeys, not in master; issue text at line 16156 says the scheduler remains unchanged. It lives only in a screen that is being rebuilt.

Found: 2026-11-15, in review round 4 of unit s13-journeys.
Status: open. The work is kept on the branch `area/s13-journeys` and is not in `master`.

The unit s13-journeys tried to cut the requests that the website's Git screen sends to the engine.
The aim was to make the Git journey of issue 966 finish inside its 360-second limit.
Four fix rounds each closed their review findings.
Each round also introduced a new fault in the same refresh scheduling.
Round 4 added a cache of refused reads and a limit of 120 automatic requests per minute for each worktree.
The cache kept a refusal after its cause was gone, and "Refresh Git" could not clear it.
The limit left the diffs of a large change list blank for up to a minute while agents were active.
The reviewer judged the scheduler too tangled to maintain: thirteen module-level collections with overlapping lifetimes.
The engine's cheaper state read of issue 1031 and the retired settings fix of issue 1032 are sound and are merged on their own.
The Git screen in `master` keeps its earlier refresh behaviour.
The Git journey of issue 966 therefore still runs close to its limit.
A rebuild should start from a written design: one record per worktree with a named state, one function that decides the next read time, and file detail loading kept outside the scheduler.
The branch's test file `packages/studio/test/git-refresh.test.ts` is a usable acceptance suite for that rebuild.

## 1034. The chat shows a markdown link to a local file as raw text.

Triage (2026-11-23): open, P2, website, medium confidence. The chat shows a link to a local file as raw brackets and a full path instead of its label. Basis: Entry states open; grep of design/mock/js and packages/studio/src finds no local file link handling; no later entry mentions it. It lives only in a screen that is being rebuilt.

Found: 2026-11-15, on the real service after pull request 63 was merged.
Status: open.

The project lead wrote a link to a file in its worktree, with a line number after the path.
The chat shows the square brackets, the round brackets and the full path as plain text.
The message is hard to read, because each such link takes three lines.
The chat should show the link's label only, or the label with a short path.
A local file path must not become a clickable address in the browser.

## 1066. The unchanged engine concurrency tests missed their startup markers under load.

Triage (2026-11-23): open, P2, tests, medium confidence. Two engine Git concurrency tests miss a five-second startup marker when the computer is heavily loaded, and pass alone. Basis: Status: open with isolated rechecks passing; repeated in issue 1079 for packages/engine/test/project/main-review.test.ts.

Found: 2026-11-15, during the full fix-round test command.
Status: open, with isolated rechecks passing.

The issue 579 and issue 579/583 tests did not observe their controlled Git subprocess's startup marker within five seconds.
The concurrent browser journey recorded a starting one-minute load average of 44.60.
The full run passed 1,455 tests and failed those two startup assertions.
Both tests passed when rerun together without the full suite.
A later one-worker run also missed the issue 579/583 marker.
The cause of the missed marker has not been established.
No engine code or test deadline was changed.
The same startup-marker sensitivity was previously observed in issue s11-12.

## 1076. The engine does not record every phase shown by the Workflow tab.

Triage (2026-11-23): open, P2, engine, medium confidence. The engine records no times for the Verify and Keep running phases or for skipped phases, so the Workflow tab shows blanks. Basis: State: open for the missing phase records; no later entry mentions issue 1076.

Found: 2026-11-15, during unit s17-firstrun.
State: open for the missing phase records.

The event envelopes retain project creation and stage change times.
Unit s17 projects these times without changing stored events.
The engine has no project stages for Verify and Keep running.
A skipped phase has no timing record.
The website leaves those durations blank.
Only the latest visit to each recorded phase is shown.

## 1083. A startup refusal can follow the launcher's successful socket probe.

Triage (2026-11-23): open, P2, engine, high confidence. The launcher can report "started" and the service can then refuse to finish starting, with its output discarded. Basis: packages/cli/src/serve.ts still returns started once the child holds the lock and the socket answers; no readiness handshake.

Found: 2026-11-15, during review of unit s17-firstrun.
State: open beyond the early redacted-category fix in issues 260 and 626.

The service opens its Unix socket before all startup work has finished.
The launcher reports success when its child holds the lock and the socket answers.
It does not read a readiness handshake.
A later startup refusal can therefore follow a reported "started" result.
The service's standard output and standard error are still discarded on the detached path.
The startup channel carries a fixed redacted category when a refusal is observed before success.
It does not retain the service's own output.
This unit preserves the original successful startup timing.

## 1098. The shared website read list omits five unused engine reads.

Triage (2026-11-23): open, P3, website, high confidence. Five engine reads the website does not yet use are missing from its list of requests that wait for the connection. Basis: grep of packages/studio/src/readRequests.ts finds none of the five names, only board.channels, board.peek and board.read. It lives only in a screen that is being rebuilt.

State: open, recorded without a code change as required by s18-smallfix round 2.

The website does not currently call `project.checkouts`, `project.creations`, `project.repository.discover`, `project.mocks.screen.read`, or `board.status`.
Those engine read names are absent from `packages/studio/src/readRequests.ts`.
A future website caller using one of them would keep the immediate request behaviour during a connection attempt.
The type check rejects unknown names but does not prove that the list is complete.

## 1099. A request classified as a read can advance the board watermark.

Triage (2026-11-23): open, P3, client api and command line, medium confidence. The request named board.read is treated as a plain read but also moves the caller's read marker. Basis: Recorded without a policy change by s18-smallfix round 2; board.read is still in readRequests.ts line 10.

State: recorded without a policy change as required by s18-smallfix round 2.

The shared website read list includes `board.read`.
The engine handler in `packages/engine/src/board/commands.ts` advances the caller’s read watermark.
Waiting for the connection is compatible with that operation, but it is not a pure read.
The classification and the engine behaviour remain unchanged in this round.

## 1106. A cancelled file open could render an absent file.

Triage (2026-11-23): open, P3, website, medium confidence. After a cancelled file open, the Files view can keep saying Reading although nothing is being read. Basis: State: open after the wording restoration in s18-smallfix round 4; no retry or new wording was added. It lives only in a screen that is being rebuilt.

State: open after the wording restoration in s18-smallfix round 4.

Round 3 avoided an absent-file exception by showing an empty pane after a cancelled open.
That empty pane changed the existing website state without approval.
Round 4 restores the original Reading state when there is neither a file nor a visible error.
Navigation no longer causes this state because it no longer cancels reads.
A remaining session cancellation can settle a read while its view still says Reading.
The view keeps its previous wording until a later read or session transition replaces it.
The mounted regression verifies that it neither renders an empty pane nor accesses an absent file.
No new wording or retry mechanism was added.

## 1107. A settled terminal settings read still looked busy.

Triage (2026-11-23): open, P3, website, medium confidence. After a failed terminal settings read, the screen keeps saying Reading terminal settings although the read has ended. Basis: State: open after the wording restoration in s18-smallfix round 4. It lives only in a screen that is being rebuilt.

State: open after the wording restoration in s18-smallfix round 4.

Round 3 changed unknown terminal settings to show No sessions are open after a read settled.
That changed the existing website wording without approval.
Round 4 restores Reading terminal settings whenever the setting is unknown.
Navigation no longer cancels the settings read.
A genuine failure or session cancellation can still leave the setting unknown after its promise settles.
The existing connection effect can retry when the connection becomes live.
The regression verifies that the loading promise releases while the previous Reading wording remains.
No replacement wording was invented.

## 1114. The test ownership preload does not follow every launch form.

Triage (2026-11-23): open, P3, tests, medium confidence. The test helper that tracks services started by tests misses several ways of launching a program, so those need manual cleanup. Basis: Entry: scripts/service-test-owner.mjs wraps spawn and fork only; State open; no later entry mentions it.

State: open.

`scripts/service-test-owner.mjs` wraps spawn and fork only.
It does not inject a lease into exec, execFile, execSync, execFileSync or spawnSync calls.
Round 5 limits injection to the identified service entry, CLI entry or bin/lgtm launcher.
An arbitrary helper program receives no ownership metadata, even when it later starts a service.
Node launches using unrecognized prefix options are also outside the identified forms.
The hook follows only the argument-free Node flags named in its nodeFlags set.
All ordinary child environments remain exactly as supplied by their caller.
A child that removes the ownership variables or Node startup options can also escape the preload.
Those launch forms still need explicit fixture teardown and do not gain interruption cleanup from this hook.
The new real-runner test proves that serveDetach’s direct service spawn does receive a lease record.
No broader coverage is claimed.

## 1124. The live check's "reload" step failed once with a socket read error.

Triage (2026-11-23): open, P3, tests, low confidence. The live check's reload step occasionally fails with a socket read error, cause unknown. Basis: Seen again in a later run recorded at docs/issues.md line 18819; cause never established.

Found: 2026-11-16, in the orchestrator's live check of the unit `s18-smallfix`.
State: open.

The first `pnpm live-check` run passed 18 of 23 steps.
Its "reload" step failed with "SocketReadError: An error occurred during Read".
A second run on the same commit passed that step and gave the known result of 19 of 23.
The cause of the single failure is not known.

## 1133. The reviewer defaults have storage but no execution consumer.

Triage (2026-11-23): waits for the owner's decision, P2, board and roles. The reviewer default settings are saved but nothing uses them; the owner must decide whether to hide the controls. Basis: grep finds reviewDefaults only in packages/engine/src/project/commands.ts storage (lines 105, 115); no consumer.

Owner ruling (2026-11-25): Leave it open. The rebuild of the website decides it, because it concerns a screen that is being redesigned.

State: open. The owner must decide whether to hide these controls.

I read the Default reviewers and Reviewer from another vendor controls.
I searched the engine source for reviewDefaults.
I found its storage in the project settings command.
I found no execution consumer.
The controls remain visible because round 2 requests a record without a fix.

## 1134. Standing permission enforcement will need its website switch restored.

Triage (2026-11-23): open, P3, board and roles, high confidence. When standing permission rules are built, the hidden website switch for them must be restored in the same change. Basis: standingRulesEnabled is defined in roles/stall-watch/switch.ts with no caller; AGENTS.md says standing rules remain unbuilt.

State: open. Restore the control in the same change that builds enforcement.

I read standingRulesEnabled in the stall-watch switch module.
It treats an absent value as enabled.
I searched the engine source and found no caller today.
Hiding the website switch does not change permission execution today.
A stored true value or an absent value would enable the flag when a future consumer calls it.
The owner needs the website switch when enforcement is built.
The roles.settings.set request can still change the stored flag.
Round 2 does not change the default or build enforcement.

## 1135. The engine records the client's approval decision without checking evidence.

Triage (2026-11-23): open, P2, git, medium confidence. The engine stores the approval decision the page sends without checking that the current evidence supports it. Basis: State: open, round 2 records the behaviour without changing it; no later entry mentions issue 1135. Code not re-read.

State: open. Round 2 records this behavior without changing it.

I read admitMainReview and the approval planner.
The engine checks the owner, confirmation, review identity, and reviewed pair.
The Git operation checks the actual checkout heads before merging.
The planner stores the client's decision value.
It does not check whether the current evidence supports that decision.
A change of evidence alone does not cause an engine refusal.
The website's captured decision preserves what the pressed button showed.

## 1141. A paused agent still cannot request a fresh session

Triage (2026-11-23): open, P2, harnesses and agent running, medium confidence. A paused agent cannot be given a fresh session, because the engine refuses resume in the paused state. Basis: State: open as the remaining part of issue 152; no later entry mentions issue 1141.

Found: 2026-11-16, while reading the resume contract for unit s21-recover.
State: open, as the remaining part of issue 152.

The command contract refuses resume in the paused state.
The website's fresh fallback cannot override that refusal.
Changing paused-state admission would change the existing lifecycle contract.
This unit leaves that contract unchanged.

## 1146. An agent outside a project has no reachable website chat

Triage (2026-11-23): open, P2, website, medium confidence. An agent that belongs to no project has no chat the owner can reach on the website. Basis: State open; docs/issues.md line 18057 still cites issue 1146 as current.

Found: 2026-11-16, while checking notification destinations for unit s21-recover.
State: open.

The website reaches agent chats through a project.
An agent outside every project has no matching website chat route.
The notification sender omits that unusable link.
This unit does not add a screen or a route for an unassigned agent.

## 1150. A fresh resume can repeat steps without the previous session's memory

Triage (2026-11-23): waits for the owner's decision, P2, website. The stop message promises Resume continues from the last turn, but a fresh resume loses memory and may repeat steps. Basis: design/mock/js/live/chat.js line 229 and actions.js line 147 still say "Resume continues from its last turn.".

Owner ruling (2026-11-25): The honest wording that the entry proposes is used. The words must not be colloquial. Because the server and the client are being rewritten in Go, this ruling is an input to that design.

Found: 2026-11-16, while reading the round 2 review and fresh-resume path.
State: open for the owner's wording decision.

The composer and stop confirmation say that Resume continues from its last turn.
A fresh resume opens a new session without the old session's memory.
Replaying an unfinished delivery can repeat a step that was already performed.
Proposed wording: "Resume tries the saved session. If it cannot reload, a new session retries unfinished deliveries and may repeat steps."
The screen text was not changed in this unit.

## 1151. Restart messages still say that running agents are stopped

Triage (2026-11-23): waits for the owner's decision, P2, client api and command line. Restart confirmations still say running agents are stopped, although a restart now keeps them for recovery. Basis: packages/engine/src/api/server.ts line 51 and client/serviceControl.ts line 97 still hold the old sentence.

Owner ruling (2026-11-25): The wording that the entry proposes is used. Because the server and the client are being rewritten in Go, this ruling is an input to that design.

Found: 2026-11-16, while reading the round 2 review and the restart texts.
State: open for the owner's wording decision.

The service confirmation, terminal API confirmation, and website toast still describe agents as stopped.
A service restart preserves their recorded lifecycle for recovery.
The affected files are serviceControl.ts, api/server.ts, and design/mock/js/live/service-settings.js.
Proposed confirmation: "Restart LGTM Studio? Recoverable agents will reconnect after restart. Agents you stopped stay stopped."
Proposed toast: "LGTM Studio is restarting. Reconnect to see recovered agents."
The existing text was not changed in this unit.

## 1152. An already-delivered holder copy receives no answered-elsewhere notice

Triage (2026-11-23): open, P2, board and roles, medium confidence. A lead that already received a permission request is never told when the owner answers it first. Basis: State open; grep of packages/engine/src finds no answered-elsewhere notice.

Found: 2026-11-16, while running the round 2 idle-lead permission regression.
State: open.

An idle lead can receive a permission's routed copy before the owner answers it.
The owner's answer reaches the worker once.
The engine withdraws queued copies but leaves the already-sent copy unchanged.
No existing delivery tells the lead that another actor answered.
A later answer attempt receives the generic command refusal.
This unit records that gap without inventing a notice or a stored event.

## 1154. Merge and sprint alerts with an agent open that agent's chat

Triage (2026-11-23): open, P2, website, medium confidence. A merge or sprint alert that names an agent opens that agent's chat instead of the merge or sprint. Basis: State open; no later entry mentions issue 1154. Code not re-read.

Found: 2026-11-16, while reading the round 2 review and running the alert-kind journey.
State: open.

An alert with an agent uses that agent's chat unless its subject is a board post or a request.
A merge or sprint alert can therefore open chat instead of its merge or sprint.
This unit does not add destinations for those subjects.

## 1156. The email browser fixture assumes the repository root as its working directory

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. The email notification browser fixture failed when run from packages/web because it looked for the mock relative to the working folder. Basis: packages/web/e2e/support/studioNotifications.ts line 34 now resolves design/mock through import.meta.url, not the working directory.

Found: 2026-11-16, while running the round 2 browser journeys from packages/web.
State: open.

The email notification fixture copies design/mock through a working-directory-relative path.
Running the assigned journey from packages/web looks for packages/web/design/mock and fails before opening the page.
The failure is in the fixture's source-path lookup.
This unit leaves that unrelated fixture unchanged.

## 1161. The thread's requests-below indicator excludes an answerable held card

Triage (2026-11-23): waits for the owner's decision, P3, website. The thread's "requests below" indicator leaves out a held permission card that the owner can answer. Basis: State: open for the owner's presentation decision. It lives only in a screen that is being rebuilt.

Owner ruling (2026-11-25): Leave it open. The rebuild of the website decides it, because it concerns a screen that is being redesigned.

Found: 2026-11-16, while reviewing the round 3b visible-change list.
State: open for the owner's presentation decision.

The thread can display a permission held by its lead with owner answer controls.
Its requests-below indicator still uses the earlier audience that excludes held requests.
The indicator can therefore omit a card that the owner can answer below the current scroll position.
This round records the difference without changing that indicator.

## 1162. A collapsed team's request count differs from its expanded agent rows

Triage (2026-11-23): waits for the owner's decision, P3, website. A collapsed team's request count includes a held permission that its expanded agent rows do not show. Basis: State: open for the owner's presentation decision. It lives only in a screen that is being rebuilt.

Owner ruling (2026-11-25): Leave it open. The rebuild of the website decides it, because it concerns a screen that is being redesigned.

Found: 2026-11-16, while reviewing the round 3b visible-change list.
State: open for the owner's presentation decision.

The collapsed sidebar team count includes an answerable permission held by a lead.
The expanded agent rows keep their earlier status audience, which excludes that held permission.
The count can therefore show one request while the expanded rows show no matching request status.
This round records the difference without changing the expanded status words.

## 1171. Repository role overrides need an owner decision about trust.

Triage (2026-11-23): waits for the owner's decision, P2, security. The owner must decide whether role instruction files inside a repository may be trusted to instruct agents. Basis: State open; overrides load only from the home's .lgtm/roles folder until the owner decides.

Owner ruling (2026-11-25): Role files inside a repository may instruct agents only after the owner approves that repository's role files once, and again when they change. Anything inside a repository needs approval at least once. The repository agent is the part that would handle this. Because the server and the client are being rewritten in Go, this ruling is an input to that design.

State: open.

A repository could carry instructions written by a stranger.
This unit loads role overrides only from the configured home's `.lgtm/roles` folder.
The owner must decide whether repository files may instruct agents before repository overrides are built.

## 1172. Role instruction files have no editor or file-management API.

Triage (2026-11-23): open, P2, client api and command line, medium confidence. There are no client commands to list, read or save individual role instruction files, so they can only be edited on disk. Basis: State open; grep of packages/contracts/src finds no role instruction file list, read or save commands.

State: open.

The owner can replace instruction files on disk.
The existing authenticated role-file command can replace named advice.
There are no commands to list, read or save individual Markdown instruction files.
An editor and those application programming interface (API) actions remain future work.

## 1173. Done reports and product-only commits remain prose rules.

Triage (2026-11-23): open, P2, board and roles, medium confidence. The engine does not enforce that a done report names a real tested branch head, or that merges exclude scratch files. Basis: State open; these remain prose rules in the shipped instructions; no later entry mentions issue 1173.

State: open.

The engine does not require a completion report to name an existing branch head with finished checks.
It does not reject a merge that includes run notes or scratch files.
The shipped instructions require these checks from agents and leads.
Mechanical enforcement remains future work.

## 1174. Two existing documents contain details that need the owner's publication decision.

Triage (2026-11-23): waits for the owner's decision, P2, documents. Two documents hold private project references and owner quotations that need the owner's decision before publication. Basis: State open; names docs/specs/2026-11-05-build-loop-design.md and docs/foundry.md; passages left unchanged.

Owner ruling (2026-11-25): The private references and the quotations are removed. This was done in the clean-up of 2026-11-24.

State: open.

The review identified private project references and dated owner quotations in `docs/specs/2026-11-05-build-loop-design.md`.
It also identified a private source commit reference in `docs/foundry.md`.
This unit leaves those passages unchanged and does not repeat their contents here.

## 1178. The existing foundry guide still suggests owner-only reads to agents.

Triage (2026-11-23): open, P3, documents, high confidence. The foundry guide still tells agents to use two commands that only the owner may run. Basis: docs/foundry.md lines 62 and 112 still direct a run to roles.settings.get and roles.actions.list.

State: open.

The guide tells a run to read `roles.settings.get` and `roles.actions.list`.
Those commands are restricted to the owner.
The new default instructions do not send agents to these commands.
Rewriting the older foundry workflow guidance is outside this unit.

## 1195. Agents running as the engine's user can write future role instructions.

Triage (2026-11-23): open, P2, security, medium confidence. An agent with shell access can write role override files that become the next agent's engine instructions. Basis: State: open, pending the planned agent sandbox; exposure documented in docs/roles.md.

State: open, pending the planned agent sandbox.

An agent with shell access or full permission can write an override as the same operating-system user.
The next agent of that role receives the override as engine instructions.
This is the same file-access exposure as `~/.lgtm/workflows`.
Safe file opening does not isolate programs that run as the same user.
The exposure is now stated in `docs/roles.md`.

## 1196. Compaction repeats instruction costs and harness compaction is not detected.

Triage (2026-11-23): open, P3, harnesses and agent running, medium confidence. A compaction started by the harness itself is not detected, so instructions are not restored after it. Basis: State: documented limitation; harness compaction detection remains open.

State: documented limitation in s22 round 2; harness compaction detection remains open.

Every successful engine-requested compaction restores the full instructions on the next ordinary prompt.
The restored text costs roughly 10 kibibytes for a default lead and can reach the 48 kibibyte composition bound.
A compaction started by the harness itself is not detected.
These limits are now stated in `docs/roles.md`.

## 1198. The sign-in process-group check failed in a full run and passed alone.

Triage (2026-11-23): open, P3, tests, low confidence. The sign-in process cleanup test failed once in a full run and passed alone, attributed to load. Basis: State: open, load-sensitive failure in packages/engine/test/harnesses/signin.test.ts line 67; nothing changed.

State: open, load-sensitive test failure recorded in s22 round 2.

The final whole-suite run reported one failure in `packages/engine/test/harnesses/signin.test.ts`.
The process-group cleanup test saw a descendant still running at its immediate check on line 67.
The same file passed all eight tests when run alone after the suite finished.
It also passed in the earlier whole-suite run.
The task classifies a full-run failure that passes alone as load.
Neither the sign-in implementation nor its test changed in this unit.
No timeout or assertion was relaxed.

## 1208. Abandoned preload reads occupied every logical slot forever.

Triage (2026-11-23): open, P3, engine, medium confidence. Hung credential file reads no longer block later reads logically, but can still occupy all four system worker threads. Basis: State line: slot starvation fixed; uncancelled filesystem calls can still exhaust the libuv pool; line 17267 cites it.

Found: 2026-11-16, from independent review finding m2.
State: logical-slot starvation is fixed with deadline abandonment and deduplication of source path sets. Uncancelled filesystem calls can still exhaust the default four-thread libuv pool.

Read: the old counter fell only when the filesystem promise settled.
Ran: a later healthy batch failed to start after two planted never-settling reads.
Read: a timeout now releases the logical slot.
Read: an unfinished read keeps its source path list marked to prevent duplicate inspections of that same list.
Read: different source lists with an overlapping physical path are not deduplicated across batches.
Ran: the fake-clock tests retain the original two-start assertion for duplicate abandoned paths.
Ran: a later batch starts two other paths within its own 500 millisecond budget.
Ran: a healthy later batch completes after the abandoned reads.
Read: Node.js cannot cancel an already-running stat system call.
Read: such a call can continue occupying an operating-system worker after its logical slot is released.
Read: the default libuv thread pool has four worker threads.
Read: abandoned filesystem calls from distinct source path lists can occupy all four threads.
Read: releasing logical slots does not release those threads or guarantee later filesystem progress.

## 1209. Every redaction synchronously inspects preloaded account stores.

Triage (2026-11-23): open, P3, engine, medium confidence. Every redaction does a blocking check of every account's credential file, including accounts never launched. Basis: State: recorded without a fix, as assigned; no later entry mentions issue 1209.

Found: 2026-11-16, from independent review finding m3.
State: recorded without a fix, as assigned.

Read: the credential registry refreshes every registered source during `secrets()`.
Read: an unchanged file still requires a synchronous metadata inspection.
Read: this now includes enabled accounts that have never launched.
Read: a broken store for one of those accounts also makes `healthyAll()` false.
Read: initial preloading has a deadline, but these later metadata checks do not use that asynchronous path.

## 1210. Cursor's default credential file can refuse another configuration folder.

Triage (2026-11-23): open, P3, harnesses and agent running, medium confidence. A broken default Cursor credential file refuses a Cursor account whose own configured folder is healthy. Basis: State: recorded without a fix, as assigned; follows from the two-path coverage of issue 1200.

Found: 2026-11-16, from independent review finding m5.
State: recorded without a fix, as assigned.

Read: the Cursor source checks both the configured folder and the default-home candidate.
Read: an unreadable or malformed default file therefore refuses an account whose configured folder is healthy.
Read: this is stricter than inspecting only the account's own file.
Read: issue 1200 records why both candidate paths are covered.

## 1212. Escaped slashes and mixed percent escapes add unmatched base64 spellings.

Triage (2026-11-23): open, P3, security, medium confidence. A base64 secret written with escaped slashes or partial percent escapes is not recognised and hidden. Basis: State: recorded without a fix; the spelling limits are stated in rule 6.

Found: 2026-11-16, from independent review finding m7.
State: recorded without a fix, as assigned.

Read: `JSON.stringify` leaves base64's alphabet unchanged.
Read: its identity form does not register optional backslashes before slashes.
Read: the percent form encodes the complete base64 value with `encodeURIComponent`.
Read: partly escaping its reserved characters can therefore produce an unmatched spelling.
Read: rule 6 now states those spelling limits.

## 1235. A restart can forget an instruction obligation after a failed first turn.

Triage (2026-11-23): open, P1, harnesses and agent running, medium confidence. After a failed first turn and a service restart, an agent's retried first message can be sent without its role instructions. Basis: packages/engine/src/runner/AcpRunner.ts line 616 sets instructionsPending only when no session exists; the flag lives in process memory.

State: fixed in unit f2-lifecycle. The engine now reads the obligation from its log, where a new session or a completed compaction owes it and a turn that ended normally discharges it, and passes it to the runner when it loads a session; `packages/engine/test/roles/instruction-obligation.test.ts` and `packages/engine/test/recovery/instruction-obligation-restart.test.ts` prove it. The log cannot see a successful turn whose instruction restoration failed, so such a session is treated as served.

The runner keeps the instruction obligation in process memory.
A failed first turn can leave its initial delivery queued for retry.
A service restart can load the existing session without recovering the obligation.
That requeued delivery can then be sent without role instructions.
This round changes no persisted event shape and does not repair this recovery gap.

## 1239. In one live check of three, the small Claude lead did not attempt the exact commands it was asked to run.

Triage (2026-11-23): open, P2, harnesses and agent running, medium confidence. A small Claude lead given long role instructions sometimes does not run the exact commands the owner asks for. Basis: State: open; seen again in issue 1325's run of 11; no fix recorded.

The live check starts a Claude Code lead on the smallest model at low effort.
With the role instructions in place, the lead receives about 11,000 bytes of instructions ahead of its first prompt.
In the first of three runs on the role instruction branch, the steps "removed-variables", "state-read", "always-allow", "permission-kinds" and "reload" reported that Claude did not attempt the command.
That run passed 16 of 23 steps.
The second run passed 18 of 23, with the "reload" socket read error that issue 1124 records.
The third run passed 19 of 23, which is the known result.
One run in three does not prove a cause.
A small model that is given long role instructions may follow them over a direct request from the owner.

State: open. Recorded on 2026-11-16. Watch the next live checks for the same pattern.

## 1240. Browser journeys and one sign-in test failed under load in the full check of the role instruction unit.

Triage (2026-11-23): open, P3, tests, medium confidence. Several browser journeys and one sign-in cleanup test fail when the whole suite runs on a busy computer and pass alone. Basis: State: open; same journeys recur in 1367, 1379, 1398, 1428, 1444; no fix recorded.

The unit test "stops the process group on timeout, owner stop and disconnection after the grace period" in `packages/engine/test/harnesses/signin.test.ts` failed in the whole run and passed alone.
The journeys `git.spec.ts:183`, `git.spec.ts:253`, `studio.spec.ts:902` and `studio.spec.ts:1132` failed in the whole browser run.
Other builders were running their own test suites on the same computer.
The journey results after rerunning each alone are recorded in the pull request for this unit.

State: open. Recorded on 2026-11-16.

## 1241. The foreground lock race test failed once in a whole run under load.

Triage (2026-11-23): open, P3, tests, medium confidence. The test of two foreground launches racing for one lock failed once in a whole run under load. Basis: State: open; packages/cli/test/serve-foreground.test.ts still exists; no fix recorded.

The test "two foreground launches race for one lifetime lock" in `packages/cli/test/serve-foreground.test.ts` failed in a whole run of the unit tests on the role instruction branch after `master` was merged in.
Two builders were running their own test suites on the same computer.
The test passed when its file was run alone.
The role instruction unit does not change that code.

State: open. Recorded on 2026-11-16.


## 1253. Retained worker checkouts need an owner-directed cleanup path.

Triage (2026-11-23): open, P2, git, high confidence. Stopped workers and deleted teams leave their checkouts and branches behind, and there is no safe command to remove them. Basis: No cleanup command in packages/contracts or packages/cli; a later entry says the gap 'remains issue 1253'.

State: Open.

Stopping a worker does not remove its checkout or branch.
Deleting a team preserves its registered worktrees too.
This preserves uncommitted changes and commits that have not reached the parent.
An optional cleanup command should prove that the exact registered checkout is clean and merged before removal.
The internal rollback for a refused new allocation is not a general removal command.

## 1254. A hard service crash can leave an allocation without a registration.

Triage (2026-11-23): open, P2, git, high confidence. A service crash between creating a checkout and registering it leaves an unregistered checkout that is never reconciled. Basis: No allocation intent in packages/engine/src; only rollbackAllocation in project/git/index.ts; later entry says it 'remains issue 1254'.

State: Open.

Git allocation happens before the atomic worktree registration and agent start append.
Normal refusal, interruption, and writer failure run a guarded rollback.
A process kill or power loss in that interval cannot run the rollback.
The resulting Git checkout is preserved as recovery evidence.
The engine does not adopt or delete it on a later start.
A retry chooses another safe numeric name.
A durable allocation intent would allow this interval to be reconciled on startup.

## 1257. The installer process-tree test can observe a live descendant after timeout cleanup.

Triage (2026-11-23): open, P3, tests, medium confidence. The installer stops a timed-out recipe without waiting for every child process to exit, so one test sometimes sees a live process. Basis: State: open; packages/engine/test/client/harness-toolchain.test.ts still exists; passed on rerun.

State: Open, observed during the full unit s23-worktrees suite.

The full suite failed the existing timeout and cancellation test in `packages/engine/test/client/harness-toolchain.test.ts`.
Its immediate process-state assertion found a state other than absent or zombie after the recipe returned.
The same unchanged file passed when run alone with its original timing.
The installer sends the kill signal to the process group and resolves when the direct child closes.
It does not explicitly await every descendant's exit.
Neither the installer nor that test was changed in this unit.
The full-run result was 9,921 passed tests, one failed test, and two skipped tests.
A subsequent whole-suite run passed all 638 files, with 9,922 tests passed and two skipped.
That run used the unchanged installer test and its original timing.

## 1264. Genuine one-character secret prefixes delay ordinary sentence endings and tools.

Triage (2026-11-23): waits for the owner's decision, P2, security. Hiding the first character of a possible secret delays some sentence endings and following tool calls; the owner must decide whether to keep this. Basis: State: waits for the owner's decision; issue 1264 is cited as still standing by an earlier fixed entry.

Owner ruling (2026-11-25): Hiding the first character of a possible secret is kept, at the cost of a small delay. The issue can be closed. Because the server and the client are being rewritten in Go, this ruling is an input to that design.

Found: 2026-11-16, from round-two review finding M3.
State: waits for the owner's decision.

Read: a value-known secret's first character is private even when it ends ordinary prose.
Read: releasing it would violate the streaming privacy rule if the rest of that secret followed.
Read: a following tool remains queued to preserve event order.
Ran: a fixed fake-value corpus measures the following rates over 72 possible final characters.
Ran: each row also checks six ordinary sentences and those sentences with final full stops.

| Registered values | Final characters held | Ordinary sentences held | With full stops held | Maximum held characters |
| --- | --- | --- | --- | --- |
| 1 | 16 of 72 | 1 of 6 | 0 of 6 | 1 |
| 3 | 26 of 72 | 3 of 6 | 0 of 6 | 1 |
| 6 | 40 of 72 | 4 of 6 | 0 of 6 | 2 |
| 20 | 58 of 72 | 6 of 6 | 0 of 6 | 2 |

Read: these are fixture frequencies, not a probability estimate for all writing.
Ran: a runner regression shows the final private `s` and its following tool held until the planted continuation arrives.
Read: this task deliberately preserves that behavior.

## 1271. First-use matching allocated large automatons synchronously.

Triage (2026-11-23): open, P3, engine, medium confidence. With a very large set of secrets, the first match still builds every spelling at once, costing time and memory. Basis: State: 'Large-set first-use form construction remains open'; other parts bounded.

Found: 2026-11-16, from round-three review finding M2.
State: ordinary-set compilation and accelerator memory are bounded. Round five retains the newest form list even above the cache budget. Large-set first-use form construction remains open.

Read: the previous implementation built full-form and prefix automatons before the first result.
Read: round four uses one four-character candidate index for complete and prefix matching.
Read: short queries use direct lookups in that index.
Read: native expression compilation waits for a query longer than 256 characters.
Read: the index accepts at most 262144 form characters and 32768 head characters.
Read: its expression source is limited to 32768 characters.
Read: larger sets use literal searches over every form without an accelerator.
Read: no form is dropped to meet a resource budget.
Read: up to four states share a 1048576-character cache budget.
Read: round five always retains the newest state, even when it alone exceeds that budget.
Read: older states are evicted until only that state remains or the total fits.
Read: these are bounds on derived indexes and cached text, not a fixed bound on process memory.
Read: constructing every supported spelling still needs memory and time proportional to registered input.
Ran: the final time and memory table is in the round-four result.
Read: a state evicted by a different secret set must rebuild when selected again.
Read: the first long query can still pay for native expression compilation.

## 1283. Repeated candidate heads make adversarial whole-text scans slow.

Triage (2026-11-23): open, P3, engine, medium confidence. Text that repeats the opening characters of many secrets makes the redaction scan about forty times slower than before. Basis: State: recorded without a matcher redesign; reviewer measured 209 ms against 5 ms.

Found: 2026-11-16, from round-four review finding m3.
State: recorded without an unrequested matcher redesign.

Ran by the reviewer: twenty-five secrets beginning with s19- took 209 milliseconds on s19- repeated sixty thousand times.
Ran by the reviewer: the original baseline took five milliseconds on that input.
Read: those are the reviewer's measurements, not fresh round-five timings.
Read: every matching head still causes full-form verification for its bucket.

## 1284. Cached literal scans on a mebibyte cost more than the original baseline.

Triage (2026-11-23): open, P3, engine, medium confidence. Scanning a megabyte of text against many long secrets is four to five times slower than the original matcher. Basis: State: recorded without a matching redesign; reviewer measured 156 ms against 39 ms.

Found: 2026-11-16, from round-four review finding m4.
State: recorded without expanding this round's cache fix into a matching redesign.

Ran by the reviewer: sixty 134-character secrets took 156 milliseconds on one mebibyte of whole text.
Ran by the reviewer: the original baseline took 39 milliseconds.
Read: the reviewer observed a four-to-five-times slowdown beginning around forty such secrets.
Read: a mebibyte contains 1048576 bytes.
Read: retaining oversized states fixes repeated compilation rather than this literal scanning cost.

## 1286. An unreadable login store can refuse a permission request before its event is emitted.

Triage (2026-11-23): open, P2, engine, medium confidence. If a login store becomes unreadable, an agent's permission request is refused with no event, and the turn just fails. Basis: State: reproduced and recorded, fail-closed kept; no fix commit found by git log.

Found: 2026-11-16, while investigating the owner's round-five live-check observation.
State: reproduced and recorded without weakening fail-closed credential coverage.

Read: the owner's three live checks passed thirteen, thirteen, and nineteen of twenty-three steps.
Read: nineteen is the known normal result.
Read: the two shorter runs reported a worker turn ending without a permission request at step four.
Ran: corrupting a fake login store after launch made permission identifier checks return a refusal.
Ran: no permission event was emitted and the fake harness propagated the remote-procedure-call error into a failed turn.
Read: remote procedure call is abbreviated RPC.
Read: this is a possible path rather than a diagnosis of those live runs.
Ran: all four queue-cap tests preserve a healthy permission request until its answer.
Read: those tests cover count and byte caps with and without the redactor's explicit withholding method.
Read: neither those caps nor ordinary tool withholding generate a terminal update themselves.

## 1295. The full check of the secret hiding unit met two load-sensitive tests and four load-sensitive journeys.

Triage (2026-11-23): open, P3, tests, medium confidence. A service shutdown test sometimes exits with code 13 in a whole run, and four journeys fail under load. Basis: State: open; same shutdown failure repeats in 1334 and 1366; journeys are issue 1240's.

In one whole run of the unit tests, "stops cleanly without invoking startup redaction on an interruption-only cause" in `packages/cli/test/serve.test.ts` failed: the service exited with code 13 after the stop signal, where code 0 was expected.
The file passed three times in a row when run alone, and the unit does not change it or the code it tests.
In the whole browser run, `git.spec.ts:183`, `git.spec.ts:253`, `studio.spec.ts:361` and `studio.spec.ts:1189` failed, and each passed when run alone.
Three live checks before round five gave 13, 13 and 19 of 23 steps; in the two short runs the Codex worker ended its turn without asking for permission, which is issue 272.
Two live checks after round five gave 19 of 23, and three on `master` gave 19, 18 and 19.

State: open. Recorded on 2026-11-16.

## 1330. Recovery hid the website and its sign-in form.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. During agent recovery the website and its sign-in form were unavailable. Basis: Entry text: website now loads and login works during recovery; packages/web/e2e/studio-startup.spec.ts exists.

The web server returned an empty service-unavailable response for static files before checking the request policy or session.
Agent recovery can wait up to 120 seconds for each adapter setup.
The website now loads during recovery.
Login now checks the password during recovery.
Authenticated data requests and socket upgrades return a typed starting state with recovery counts until the full recovery gate opens.
The existing connection badge and menu show the progress.
Authenticated starting replies count as contact, so a long recovery does not become an offline warning.
Data waits because recovery changes agent credentials, lifecycle states, and request holders.
The total includes interrupted and paused agents.
A failed or unsupported reload does not count as a successful recovery.

## 1331. Reloading a tab discarded unsent chat messages.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. Reloading a browser tab lost unsent chat messages. Basis: Entry text: drafts now kept in per-tab session storage with limits.

Agent and board composers kept their text only in memory.
The shared mock and website draft store now uses per-tab session storage for composer keys.
The stored data is limited to 32 drafts, 8,192 characters per draft, and 65,536 serialized characters in total.
A successful send removes its draft from storage.
Password fields, secret fields, sign-in fields, and terminal input do not use this store.

## 1332. Availability was mistaken for sign-in evidence.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. An account that was merely available was shown as signed in without proof. Basis: Entry text: evidence now comes from session events; neutral 'Not reported' wording.

Both account adapters treated an available account as signed in without evidence.
The account view now derives positive evidence from successful session events.
The existing local credential check keeps its result in memory for the current account row.
The snapshot exposes only the resulting boolean.
A changed login configuration invalidates the evidence.
An account without evidence uses the existing neutral wording, “Not reported”.
An explicit sign-out remains signed out after the owner clears an availability pause.

## 1333. Plan updates accumulated as separate chat rows.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. Each plan update appeared as a new chat row. Basis: Entry text: the view now updates the first plan row of a turn.

The thread adapter appended every plan update as a new message.
The view now updates the first plan row of a turn with the latest entries.
The same fold handles live events and replay.
Stored plan events have no plan identifier.
Separate plans within one turn therefore share a row.
A turn or session boundary starts a separate row.

## 1335. Signing out left composer drafts in the browser.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. Signing out left unsent drafts in the browser. Basis: Entry text: sign-out, expiry and unconfirmed logout now clear memory and storage.

The first startup change preserved drafts across every session transition.
Sign-out, expiry, and an unconfirmed logout now clear both memory and per-tab storage.
An outage preserves the drafts.

## 1336. Unknown sign-in evidence disabled previously usable account controls.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. Accounts with unknown sign-in status lost controls that used to work. Basis: Entry text: unknown accounts now keep base actions and are counted separately.

The first startup change made unknown accounts look signed out to controls beyond their status label.
Unknown accounts now retain the base account actions, default badge, model choices, and composer selection.
The header counts confirmed sign-ins among known accounts and reports unknown accounts separately with “Not reported”.
Subscription loading now runs the existing sequential local check for accounts without evidence.
It repeats that check after an engine restart.
Successful session events and explicit sign-out events derive deterministic evidence on replay.
An owner-cleared availability event now returns the account to unknown.
Local-check evidence stays in memory for its current account row.
A replaced row loses that observation and can be checked again.

## 1337. Recovery progress counted successful reloads against unrelated pauses.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. Recovery progress counted agents that were not part of the recovery. Basis: Entry text: total now includes only agents selected for recovery; progress bounded.

The total now includes only the interrupted and returning paused agents selected for recovery attempts.
An attempt counts when it finishes, including a failed attempt.
Progress cannot exceed the total.
The browser clamps excess counts instead of rejecting the authenticated starting state.
The notice names completed recovery attempts.
The preflight phase reports zero attempts until its recovery work is selected.

## 1338. An overlong saved draft could return as shortened text.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. A very long saved draft could come back cut short after a reload. Basis: Entry text: overlong drafts are now left out of storage whole, with one notice.

A composer draft longer than 8,192 characters is now omitted from storage in full.
Its complete text remains in the current tab's memory.
The existing toast area reports once per composer that the draft is too long to keep after a reload.
The other draft count and total-size bounds remain in force.

## 1339. The first authenticated page could look like an empty installation during recovery.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. The first page during recovery looked like an empty installation. Basis: Entry text: main area shows Loading until the first snapshot; journey verifies it.

The main area now uses the existing empty-state component with “Loading…” until its first snapshot arrives.
The connection controls remain visible during that wait.
The held-recovery journey verifies the main area before recovery completes.

## 1340. A plan update can change text above later tool calls.

Triage (2026-11-23): open, P3, website, high confidence. A plan update rewrites a plan row that sits above later tool calls in the chat. Basis: State: open as the presentation tradeoff recorded by review finding m5. It lives only in a screen that is being rebuilt.

The latest plan replaces its first row in the turn.
An update after tool calls therefore changes a row above those calls.
This keeps one stable plan row without changing stored events.
State: open as the presentation tradeoff recorded by review finding m5.

## 1341. A starting badge can remain visible until the next session probe.

Triage (2026-11-23): open, P3, website, high confidence. The starting badge can show old recovery counts for one retry after the service stops answering. Basis: State: open as review finding m7. It lives only in a screen that is being rebuilt.

A failed socket attempt first preserves the last authenticated startup counts.
The subsequent session probe clears that starting state when the service stops answering.
The badge can therefore retain the old counts for one retry.
State: open as review finding m7.

## 1342. Recovery coverage omitted hostile request headers and the feature list was stale.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. Recovery tests did not cover bad Host or Origin headers, and the feature list was stale. Basis: Entry text: the recovery test now checks both; feature list updated.

The recovery test now checks that a bad Host or Origin receives the same forbidden response during and after recovery.
Those request-policy checks already ran before the recovery gate.
The test retains its 5.14 identifier.
The feature list now describes static loading and login during recovery.


## 1343. The first attempt-count fix moved timer reconciliation ahead of ordinary reloads.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. A first fix let an account become available before ordinary reloads finished. Basis: Entry text: final implementation previews returns without releasing early; regression caught it.

The existing recovery-order regression caught an account becoming available before an ordinary reload finished.
The final implementation previews overdue returns without releasing the account early.
Timer reconciliation still follows ordinary reloads.
The total is reconciled with the actual returning work because account deadlines can change while an adapter loads.
The completed count remains bounded by that total.


## 1344. A Setup bookmark bypassed the first-snapshot loading guard.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. Opening a Setup bookmark skipped the loading guard during recovery. Basis: Entry text: the Setup route now waits for the first snapshot.

The first loading guard covered the main views after the Setup route had already returned.
A held-recovery journey reproduced the gap through that bookmark.
The Setup route now waits for the first snapshot too.

## 1345. The long-draft notice initially stayed suppressed for the next draft.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. The too-long-draft notice stayed silent for the next long draft. Basis: Entry text: clearing or shortening the draft now resets the guard.

The first notice guard remembered a composer key after its text was cleared.
Clearing the draft or making it short enough now resets that guard.
A later overlong draft receives its own single notice.

## 1346. A missing local credential was treated as proof of sign-out.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. A missing local credential was taken as proof of sign-out and disabled usable accounts. Basis: Entry text: local checks now only add positive evidence.

The local check does not cover every harness or login method.
Its negative result disabled usable Cursor and custom accounts.
It also overrode successful session evidence and an owner's availability reset.
Local checks now only add positive evidence.
Stored event evidence takes precedence.
Only the stored AccountNeedsSignIn evidence confirms sign-out.

## 1347. Automatic account checks were unbounded and repeated inconclusive reads.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. Automatic account checks had no limit and repeated inconclusive reads. Basis: Entry text: at most 64 unchecked rows per load, outcomes remembered.

A subscription load now checks at most 64 previously unchecked account rows.
Every outcome is remembered on its account row.
An account with no harness configuration also consumes a slot and remembers its outcome.
Later loads advance through the remaining rows.
Replacing the row permits a new automatic check.
The explicit account check can still run again.

## 1348. The boot account-return step could reload a paused agent with a live token.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. Startup could reload a paused agent that already held a live token. Basis: Entry text: boot return selection now skips such agents; regression added.

Normal startup revokes the old tokens before recovery.
An intervening token issuance could still make a planned return stale.
The boot return selection now skips an agent that already holds a live token.
A regression issues a token during an ordinary reload and verifies the paused agent is skipped.

## 1349. A recovery progress preview moved a stored-event failure earlier.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. An unreadable stored event stopped ordinary agents from reloading at startup. Basis: Entry text: a failed preview now gives an empty estimate; regression added.

The preview decoded availability history before ordinary agents reloaded.
An unreadable availability event therefore prevented those reloads.
A failed preview now supplies an empty estimate.
The actual account-return step still reports the failure after ordinary reloads.
A regression restores the deliberately corrupted test event before checking replay consistency.

## 1350. A refused first snapshot left the main area loading indefinitely.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. A refused first snapshot left the page loading forever. Basis: Entry text: the main area now shows the snapshot notice.

The main area now shows the existing snapshot notice when it is set.
The startup count notice still leaves the existing Loading… state in the main area.
A later successful snapshot opens the usual screen.

## 1351. A sample account added without sign-in became unknown and usable.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. A sample account added without sign-in looked usable in the mock. Basis: Entry text: the mock now creates the row as signed out.

The mock now creates that sample row with known sign-out status.
Its row again shows Signed out and the Sign in button.
The live account still begins without sign-in evidence.

## 1352. The all-unknown header reported zero signed-in accounts out of zero.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. The accounts header read zero signed in out of zero when every account was unknown. Basis: Entry text: header now reads n Not reported.

The header now reads n Not reported when every account is unknown.
It appends the existing limit count only when that count is positive.
The empty list reads 0 Not reported.
The mixed list retains separate known and unknown counts.

## 1364. A task still requires a team in its stored contract.

Triage (2026-11-23): open, P2, board and roles, high confidence. A task cannot exist without a team, so a lead must create a placeholder team before its first tasks. Basis: packages/contracts/src/project/schema.ts line 136 still requires teamId; State: deferred.

Issue 944 reports that the lead created a placeholder team to create its first tasks.
The current `Task` schema in `packages/contracts/src/project/schema.ts` requires a team identifier.
The creation command and stored `project.TaskAdded` event both use those fields.
The planner also requires the team's worktree and checks the owner's team membership.
Allowing a task without a team therefore changes the stored event shape.
A follow-up would make the team optional in the command, event, views, and typed mappings while preserving replay of existing logs.
It would define the permitted project worktree and task owners before assignment to a team.
It would review team filtering, sprint team summaries, context selection, task creation, and default role guidance.
The board already has some missing-team fallbacks, but they do not remove those contract and authorization requirements.
State: deferred under the explicit stored-shape limit in unit s25-smallfix.

## 1375. The review of the small fix unit left three minor faults in "Doing now" and one weak journey check.

Triage (2026-11-23): open, P2, website, high confidence. The roster's 'Doing now' can show work from an earlier turn or the wrong task, and one journey check is fragile. Basis: packages/studio/src/board.ts workPreview still scans the whole thread and takes the first active task; studioGit.ts still compares toasts. It lives only in a screen that is being rebuilt.

With no active task, the roster's "Doing now" takes the latest tool or delivered message anywhere in the agent's thread, with no turn boundary (`packages/studio/src/board.ts`, near lines 22 to 27).
An agent woken by an answer can therefore show the last tool of its previous turn, and a fresh agent can show the start of its role prompt.
When an agent owns two active tasks, the roster always names the first in snapshot order (`board.ts`, near line 10).
The browser check that a repeated conflict choice shows no refusal compares the notices before and after the click (`packages/web/e2e/support/studioGit.ts`, near lines 149 to 152), so an expiring notice can fail it.
In the orchestrator's whole browser run, `git.spec.ts:183`, `git.spec.ts:253`, `studio.spec.ts:372` and `studio.spec.ts:1200` failed under load, and each passed when run alone.

State: open. Recorded on 2026-11-17.

## 1377. Older harness settings have no automatic permission mapping.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. Older harness settings had no mapping for the automatic permission mode. Basis: State: handled with an explicit edit fallback in unit s26.

State: Handled with an explicit edit fallback in unit s26.

The pinned Claude and Codex adapters expose automatic review.
Historical harness events have only the original four mappings.
Those events still decode without migration.
Round 2 upgrades the effective value of an exact previous factory setting when read.
An owner-edited historical mapping remains unchanged.
An `auto` request falls back to the stored edit mapping when no automatic mapping is configured.
Gemini and Cursor defaults also use the edit fallback because this unit verifies no separate automatic review mode for them.
The harness status reports the fallback.
Existing agents and owner-edited permission defaults are not changed.

## 1378. Alternate environment homes can place harness login stores inside a worktree.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. Changing HOME or XDG_CONFIG_HOME could put a harness login store inside a worktree. Basis: State: protected in unit s26; handler regressions cover both overrides.

State: Protected in unit s26.

The harness-specific home variable is not the only way to move a login store.
An account can override `HOME`.
Cursor can also override `XDG_CONFIG_HOME`.
The folder ceiling protects the existing login directories derived from these fallbacks.
Secret references are resolved without reading credential files.
Handler regressions cover both override paths under a registered project root.

## 1387. The whole round-two unit run failed a secret-carry stop check that passed alone.

Triage (2026-11-23): open, P3, tests, medium confidence. A test of stopping the runner while a secret prefix is held failed once in a whole run and passed alone. Basis: State: open; packages/engine/test/runner/secret-carry-s19.test.ts still exists; no fix recorded.

State: Open as a whole-run-only failure observed during round 2 of unit s26.

The whole unit run failed the existing stop case in `packages/engine/test/runner/secret-carry-s19.test.ts`.
The case is "s19 stop flushes a genuine secret prefix and its queued tool".
It observed no delivery updates immediately after stopping the runner.
It expected the message and queued tool updates.
The exact case passed alone with its existing assertions and timing limits.
This round changes no stop or redaction implementation.

## 1394. The folder documentation omitted shared Git settings and clean subfolders.

Triage (2026-11-23): reported fixed, not verified (medium confidence); kept here until someone checks. Folder documentation left out shared Git settings and clean subfolders. Basis: State: clarified in round 3 of unit s26.

State: Clarified in round 3 of unit s26.

Sibling worktrees share the Git common directory, configuration and hooks with the service checkout.
Folder admission does not isolate those settings.
The main checkout root containing the service is refused as a child folder.
A clean subfolder such as packages remains admitted when the parent is already in that main checkout.
The clean-subfolder regression passed before the clarification.


## 1397. Some test fixtures inherit the invoking home for readiness checks.

Triage (2026-11-23): open, P2, tests, high confidence. Some tests run with the real home folder, so readiness checks can look at real login paths. Basis: vitest setup files have no HOME override; State: repository-wide fix remains separate work.

State: Open as a test-isolation gap found in round 3 of unit s26.

The common unit setup clears provider variables but does not replace HOME.
Some Git fixtures also omit their own HOME override.
Lead readiness builds its environment from the invoking process before checking login stores.
Those fixtures can therefore probe login paths outside their synthetic rig.
The earlier round-three commands did not provide a synthetic outer home.
No real login store was inspected to determine whether such a file existed.
Subsequent validation uses a temporary outer home and clears provider variables before starting the test runner.
The wrapper keeps the installed browser binary cache available without using it as a login directory.
A repository-wide test-environment fix remains separate work.


## 1415. The large-output terminal test can mistake command echo for completion.

Triage (2026-11-23): open, P3, tests, high confidence. The large-output terminal test can take the echoed command for the finish marker and check too early. Basis: packages/engine/test/harnesses/terminal-server.test.ts lines 179-180 still accept any LARGE_DONE; State: open.

Observed during the s28-session round 2 whole unit run on 2026-11-17.
The test waits 200 milliseconds after sending `stty -echo`.
It then accepts any occurrence of `LARGE_DONE` as completion.
The failed run still contained that marker in the echoed command before all 400,000 lines arrived.
Its output-length assertion consequently ran after only a prefix of the output.
The original test passed when rerun alone.
The timeout and assertions remain unchanged.
State: open as a fixture synchronization problem.

## 1416. A Git overlap test failed before its held subprocess started.

Triage (2026-11-23): open, P3, tests, medium confidence. A Git overlap test sometimes fails before its held subprocess starts, also on the base commit. Basis: State: reproduced on the base as intermittent; test still at packages/engine/test/project/main-review.test.ts:523.

Observed during the s28-session round 2 whole unit run on 2026-11-17.
The test named "issue 579/583: queued registration and creation do not block a positive independent writer while real Git is held" did not observe its subprocess marker within five seconds.
It failed again when selected alone with the original assertion and timeout.
The failure occurs before the test launches its queued writers.
The Git implementation and this test are unchanged by round 2.
A control run with the changed production modules restored to signed Round 1 passed this case.
A following run with the exact round 2 source restored failed the five-second marker wait again.
The test uses the local socket and does not start a web listener.
These observations do not establish the cause of the difference.
State: open with the failure unresolved.

Round 3 selected this test alone five times on the unmodified base at d4fb1877.
Four base runs passed.
The failed base run reached the same five-second subprocess-marker assertion.
All five runs on signed Round 3 passed.
The complete 45-test main-review file also passed in the whole unit suite.
The Git implementation and this fixture have no diff from the base.
No assertion or timeout was changed.
The earlier single-control difference therefore does not establish a regression from this unit.
State: reproduced on the base as an intermittent setup failure.

## 1427. GitHub does not offer an atomic reviewed-base condition for its merge endpoint.

Triage (2026-11-23): open, P3, git, medium confidence. GitHub cannot pin the reviewed base during a merge, so the base can move between the check and the merge. Basis: State: open; a limit of GitHub's merge endpoint, not fixable in the engine alone.

State: open.
Found: 2026-11-17, while checking the GitHub merge endpoint documentation.
The engine checks the prepared remote base immediately before requesting the merge.
The merge request atomically pins the reviewed head.
The endpoint cannot atomically pin the prepared base as well.
The remote base can therefore move between preflight and merge.
GitHub still enforces its current merge and branch protection rules.
A remote merge and a local checkout update also cannot share one transaction.
The reply records a confirmed remote merge even when the later local update cannot complete.


## 1442. The file-editor browser failure also occurs on the unit's base commit.

Triage (2026-11-23): open, P3, tests, medium confidence. The file-editor browser journey fails about one time in five, even on the base commit. Basis: State: open; baseline failure established on d4fb1877, cause unknown; journey is in retired packages/web/e2e. It lives only in a screen that is being rebuilt.

State: open.
Found: 2026-11-17, during the round 3 baseline comparison required by the owner.
Five sequential one-test invocations on d4fb1877 passed four times and failed once.
The failure waited for the first external-edit conflict panel.
That is also a failure point recorded for round 2 in 1435.
This worker ran no other test job during those trials.
Host load observations were retained because other users' processes are outside this worker's control.
Five sequential one-test invocations on signed implementation 232b243b passed all five times.
The file-editor implementation and its assertions have not been changed by this unit.
These observations establish a baseline failure without establishing its cause or proving that the intermittent failure is gone.

## 1445. The hosted browser journey intermittently exceeds its existing waits.

Triage (2026-11-23): open, P2, tests, medium confidence. The hosted pull request browser journey sometimes runs past its waits, even when run alone, and the cause is unknown. Basis: Entry says cause unproven; a later entry (issues.md line 21886) records the same journey failing twice alone and names issue 1445. It lives only in a screen that is being rebuilt.

State: open.
Found: 2026-11-17, during the round 3 whole run and isolated rerun.
The clean-base hosted journey passed the earlier focused invocation.
It failed in the whole run while waiting ten seconds for its approval control.
Its first isolated rerun failed while waiting thirty seconds for Refresh Git to release its busy state.
An additional private diagnostic invocation of the unchanged helper passed.
Its trace showed preparation, refresh, approval, and final status reads completing.
The trace retained request names and timing without request data or credentials.
The dirty-base hosted journey passed both the focused invocation and the whole run.
The original journey passed unchanged in a second isolated invocation.
The cause of the intermittent clean-base failure remains unproven.

## 1457. The review of the second small fix unit left five minor findings.

Triage (2026-11-23): open, P2, harnesses and agent running, medium confidence. A review left five small faults in custom harness mode defaults, a session-record document sentence, one test assumption and a lost startup warning. Basis: harness-defaults.test.ts:57 still puts /usr/bin on PATH; SECURITY.md:230 still does not say the future-dated record is removed. No later fix text.

State: Open. Found by the independent review of unit s29-smallfix on 2026-11-18.

A custom harness row with an empty list of reported modes now keeps a stored default of "ask", although such a row never failed with "mode is not recorded".
A custom row that maps "auto" to a mode it does not report, while reporting its edit mode, still reads "ask" as "auto" and then fails to start; the check should use the mode that "auto" resolves to.
A session record dated more than five minutes in the future is removed from the session file, and `SECURITY.md` should say that it is removed and not only skipped.
The reset test in `packages/engine/test/api/harness-defaults.test.ts` assumes that no `gemini` program exists in `/usr/bin`.
An existing installation no longer logs the "harness adapters not found" warning at every start.

## 1459. The roadmap and mock introduction lag behind the connected website

Triage (2026-11-23): open, P2, documents, medium confidence. The roadmap and the mock's introduction still describe connected website actions as not connected or as future work. Basis: design/mock/README.md:53 still reads "Actions that change something are not connected to the engine yet.".

Found: 2026-11-19, while evaluating the current project status at `2d5a4568`.
State: Open as documentation drift.

The roadmap still describes the build-loop plan as awaiting approval and the website replacement as future work.
The mock README still says that live actions are not connected, before describing several connected actions later in the same file.
The current Studio entry point installs live Git, chat, project settings, Files, and Terminal mappings.
The website build passes in this checkout after the package manager restores its workspace dependency links.
These older status statements must not be treated as a current inventory of missing functionality.
Reconcile the documents against the implementation and the remaining disabled controls.

## 1461. Build lead 1 remained interrupted after the latest-code service restart

Triage (2026-11-23): open, P1, engine, low confidence. After a service restart one reloadable agent stayed interrupted and was neither retried nor restarted, with no failure shown. Basis: Entry says the cause has not been established; no later text or commit names a fix. Code not traced.

Found: 2026-11-19, while verifying the owner-requested service restart at `5e63ccab`.
State: Open. Unit f2-lifecycle traced the recovery code, and two silent paths fit the observation. A reload that fails or times out writes `AgentReloadFailed`, but that event changes no field of the agent's snapshot row, so the client sees only "interrupted, previous idle" with no start-failure code, which issue 1604 records. An agent whose project is archived or archiving is skipped by recovery with no event at all. The real log was not read, so which path happened is still unknown. Recovery never retries a failed reload by design; the owner resumes the agent.

The service completed its recovery gate and accepted authenticated data requests.
The snapshot retained three projects and all 15 roster records.
One agent was working, one was idle, 12 were stopped, and Build lead 1 remained interrupted with previous state idle.
That Codex agent has a session and reports that it is reloadable.
The client snapshot reports no start-failure code for it and no failed sections.
The session did not retry the agent or start a fresh conversation.
Inspect its recovery attempt before deciding how to resume it.

## 1464. Define question cards contain whole proposals and bundle separate decisions

Triage (2026-11-23): open, P1, website, medium confidence. Define question cards show a whole proposal at once and bundle several decisions into one answer, so the owner cannot answer each separately. Basis: Entry cites lifecycle.ts opening instruction and studio chat.ts mapping one answer per question; replacement is only a mock draft in features.md.

Found: 2026-11-19, by the owner in the two issue 940 projects, with the saved events and website source inspected in the same session.
State: Open. The owner wants a clearer Define interaction before implementation.

The Codex lead in "We're trying issue 940" put a numbered proposal and its requested execution path in one question.
Its question events are 71234 and 71364 in the inspected service log.
The Claude lead in "Issue 940 new project" put its proposal and two separate naming questions in event 1926.
The owner accepted that bundle as written in one answer and later clarified individual statements through chat.
The resulting confirmation card and the separate agent-choice card remained independent questions.
A chat correction did not itself answer either card.

The opening instruction in `packages/engine/src/project/lifecycle.ts` tells the lead to put the full proposal under Details.
`packages/studio/src/chat.ts` maps that proposal to the card body and supplies no separate summary.
`design/mock/js/ui.js` displays the full body immediately when no summary exists.
The built website contains the same fallback.
The promised Details treatment therefore does not hold for these live question records.

The live mapping also creates one proposed answer per question.
Several decisions written in its body do not become independently answerable controls.
The saved owner replies were delivered in the inspected examples.
The observed problem is the presentation and granularity of the decisions.
The proposed Define workspace is recorded in `docs/features.md`, with its layout still undecided.

On 2026-11-20 the owner described the same defect from memory.
The card "Agree on project ID and label" took about 70 percent of the space available to it.
The Details button changed the layout slightly and revealed nothing new.
The mock now holds a first draft of a replacement, recorded under "A better Define phase" in `docs/features.md`.

## 1472. Review and listing limits are close for a large monorepo

Triage (2026-11-23): open, P1, git, high confidence. Review and file listing limits are nearly reached by a large repository, and listings past the limit drop entries without saying so. Basis: packages/engine/src/project/git/index.ts:53-55 still default to 1 MiB output, 128 KiB diff and 200 entries.

Found on 2026-11-20 by reading the code and measuring a second repository.
A review is refused above 200 commits or files, above 128 KiB of diff, or with any binary file (`project/git/index.ts`).
The file listing has a 1 MiB output budget, and the measured repository's tracked file list is already about 0.97 MiB.
Past the budget, listings and file search drop entries without a message.
State: Open.

## 1474. Pull request 85 was merged without a code review or a whole test run

Triage (2026-11-23): open, P2, process, medium confidence. The change that lets the owner choose a project lead was merged without an independent code review or a whole test run. Basis: Merge 36318cbe (pull request 85); issues.md has no later record of a review, only the fix of its fault 1473.

Recorded on 2026-11-20. The owner asked for all open pull requests to be merged.
The pull request's own description says that no full code review ran and that the whole suite was not rerun.
Issue 1473 is one fault that the whole suite then showed.
State: Open until an independent review of that change has run.

## 1502. Safe preparation copying currently requires Linux directory descriptors

Triage (2026-11-23): open, P3, engine, high confidence. Copying preparation files safely works only on Linux, so other operating systems refuse configured copying. Basis: Entry state open for other operating systems; engine still relies on /proc/self/fd paths (e.g. roles/instructions.ts:39-46).

Found while building unit s31 on 2026-11-20.
State: open for support on other operating systems.

Node.js has no directory-relative file-open interface with the required symbolic-link protection.
The implementation uses Linux `/proc/self/fd` paths anchored to open directory descriptors.
A host without that facility refuses configured copying.
Submodule initialization and agent instructions do not require that copying facility.
A portable implementation needs an equally safe directory-relative file interface.

## 1514. Worker rollback cannot remove a worktree containing a gitlink

Triage (2026-11-23): open, P2, git, medium confidence. When creating a worker fails, its worktree is left behind if it contains a submodule link, because Git refuses to remove it. Basis: packages/engine/src/project/git/index.ts:376 runs `worktree remove` without force; no cleanup policy found; entry state open.

Found in review round 1 of unit s31 on 2026-11-20.
State: open.

Git refuses `git worktree remove` without force when a worktree contains a gitlink.
A worker allocation that fails after creation can therefore leave its worktree behind.
The rollback warning asks for inspection.
Automatic cleanup still needs a safe policy that protects nested work and never forces removal blindly.

## 1521. Copy-pattern exclusions are permanent and shared by every checkout

Triage (2026-11-23): open, P3, git, high confidence. Ignore rules added for copy patterns stay forever in every checkout, and no command removes them. Basis: README.md:229 "There is no command to remove these exclusions"; packages/engine/src/project/README.md:657 says the same.

Found in review round 2 of unit s31 on 2026-11-20.
State: open, documented limitation.

Saving copy patterns adds exclusions to the shared Git directory's `info/exclude` file.
The exclusions affect the main checkout and every linked worktree.
Clearing or replacing preparation settings does not remove old exclusions.
There is no engine command to remove them.

## 1537. A repository has no removal command

Triage (2026-11-23): open, P2, client api and command line, high confidence. A repository cannot be removed from the registry, so it keeps constraining the workspace folder after its last project is deleted. Basis: README.md:141 "There is currently no command to remove a repository from the registry"; no remove command found in contracts or engine.

Found in review round 1 of unit s32-workspacesetting on 2026-11-20.
State: Open.

Deleting the last project leaves its repository in the registry.
That repository and its retained registrations still constrain a changed workspace folder.
No command currently removes the repository registration.
The README now states this limit.

## 1539. The preparation detail names the base-branch fallback when it was not used

Triage (2026-11-23): open, P3, engine, high confidence. The preparation message says the base branch was trusted for submodules even when the repository has no submodules. Basis: packages/engine/src/project/preparation.ts:198 appends the sentence whenever startingHead is null.

Found on 2026-11-20 by the third review of unit s31-worktreeprep.
The fallback sentence is appended whenever no starting commit is recorded and submodules are on (`packages/engine/src/project/preparation.ts`).
That includes a repository with no `.gitmodules`, where no base-branch trust was used.
State: Open. Minor.

## 1540. The single-file commit no longer refuses a path that matches a copy pattern

Triage (2026-11-23): open, P2, security, low confidence. Committing a single file no longer refuses a copied secret file that the repository's ignore rules expose, so it can reach agent reviews. Basis: Entry state open, minor; index.ts guards copy paths for agent reads (lines 295, 853) but I found no guard on the commit path.

Found on 2026-11-20 by the third review of unit s31-worktreeprep.
If a repository's committed `.gitignore` un-ignores a copied secret file, that file shows as untracked.
`project.commit` with that path then commits it, and it appears in agent reviews.
An agent can do the same with plain Git, so this is a lost second line of defence.
State: fixed in unit f3-security. The engine refuses a path that matches a copy pattern and is not tracked in `HEAD` wherever it stages or commits: `project.stage` refuses it, `project.stage.all` skips it and names it in its reply, and both forms of `project.commit` refuse it, including when plain Git staged it first.
The refusal has its own sentence, which tells the owner to remove or narrow the copy pattern to commit a new file such as `.env.example`.
A path already tracked in `HEAD` remains ordinary content.
Commit `e087cc5c` had refused copy-pattern paths in the single-file commit, and commit `93c91f8b` narrowed that check to credential paths.
A committed `.gitignore` negation outranks the patterns in `info/exclude`, so plain `git add` can stage such a file.
The Git screen does not yet show the skipped names; that needs a screen change, which this unit could not make.

## 1541. Thousands of tracked files that match a copy pattern can fail agent file listings

Triage (2026-11-23): open, P3, git, high confidence. A repository with thousands of tracked files matching a copy pattern could make agent file listings, reads and searches fail. Basis: packages/engine/src/project/git/index.ts:658-659 passes every matching name as arguments to a second `git ls-files` call.

Found on 2026-11-20 by the third review of unit s31-worktreeprep.
The agent listing passes every tracked name that matches a copy pattern to a second `git ls-files` call (`packages/engine/src/project/git/index.ts`).
A very large match set could exceed the argument or output limit and fail the listing, file read and context search for agents.
State: Open. Minor. Low likelihood.

## 1542. The owner's first message to an agent is shown with the whole role text in front of it

Triage (2026-11-23): open, P1, engine, medium confidence. The owner's first message to an agent is shown with the agent's whole role instructions in front of it. Basis: instructionEcho.ts is used only by runner/AcpRunner.ts for the agent's side; entry says cause untraced; no later fix text.

Found on 2026-11-20. Reported by the owner from use of the website.
The engine sends an agent's role instructions as part of the first message of its session.
The owner's first message in the chat then appears with the whole role text prepended, so it is very long.
An echo filter exists for the agent's side (`packages/engine/src/roles/instructionEcho.ts`); the owner's own message is not covered by it.
The cause has not been traced: the text may be stored in the message record, or added when the thread is shown.
State: Open. The feature entry "How an agent receives its purpose, and which settings, hooks and memory it gets" covers the lasting answer.

## 1543. The Messages tab shows the stall watch's service notices as ordinary posts

Triage (2026-11-23): open, P1, board and roles, medium confidence. Automatic reminders from the stall watch are stored and shown as ordinary posts, crowding the agents' messages. Basis: Entry: 134 board.PostWritten events authored by roles.stallWatch; fix undecided; studio source has no filtering of that author.

Found: 2026-11-20, reported by the owner while describing the Define experience.
State: Open. The owner wants these notices kept out of the normal reading flow.

The owner saw posts whose author is the stall watch service among the agents' messages.
The owner said that their text is longer than the space available, and that one view listed about 1,900 posts one after another.
The owner said that a person who does not read code would think something had gone wrong.

A copy of the service's event store, read on 2026-11-20, holds 134 `board.PostWritten` events with the author `roles.stallWatch`.
Their text is a reminder to an agent, such as "Work is waiting. Check the board and carry on, or report what is blocking it."
The count of 1,900 and the exact screen were not reproduced in this session.
The fix is undecided; folding service notices into one quiet line per agent is one option.

## 1544. Define questions in the mock's Project view do not reach the bell or Home

Triage (2026-11-23): open, P2, website, low confidence. In the draft design, open Define questions are counted in the sidebar but not on Home or the bell, so counts disagree. Basis: Entry says it belongs to the unfinished design; issue 1567 repeats that Define questions are missing from the Questions list. It lives only in a screen that is being rebuilt.

Found: 2026-11-20, while building the first draft of the Project view in `design/mock/`.
State: Open. It belongs to the unfinished design, not to the live website.

The draft keeps a Define question in the project's Define record, not in the list of requests.
The bell and Home count only requests.
Since 2026-11-20 the sidebar counts open Define questions on the project's row, and the lead's row says "Needs answer".
The Home count, the Home list and the bell still leave them out.
The sidebar's project counts therefore add up to more than the count beside Home.


## 1545. "Design" names two different things in the build loop and in the project stages

Triage (2026-11-23): open, P2, documents, medium confidence. The owner decided mocks are drawn after Define, but the build loop design and default workflow still place them inside Define. Basis: Entry: build loop design and design/mock/workflows/build-loop.json still place mockups in Define; the phase name "Design and teams" is unsettled.

Found: 2026-11-20, when the owner asked whether screen design is set up in Define or in a Design phase.
State: Open. The owner decided the order on 2026-11-20; the documents and the default workflow still say otherwise.

The build loop design puts screen mockups inside phase 1, Define.
The mockups are made before the end-result page is written, and the owner reads both before answering "Build this?".
Phase 2 of the loop is "Design and teams", which means reading the code, splitting the work by domain, and setting sprints and teams.

The project stages in `design/DESIGN.md` and in the engine are Define, Design, Plan, Build, Review and Hand over.
There the Design stage is the time when a Design team draws the mocks, and "Approve mocks" moves the project to Plan.
A project in the Design stage is therefore still in the loop's Define phase.
The mock's sample project "Phone companion" shows this: its stage is Design and its phase is Define.

The owner decided on 2026-11-20: agree to the end result first, then design.
Agreement ends Define, and the mocks are drawn in the Design stage that follows.
The draft Project view in the mock now follows that order and shows the stages in its top line.
The build loop design and `design/mock/workflows/build-loop.json` still place the mockups inside Define.
The name "Design and teams" for the loop's second phase is still to be settled.

## 1546. The mock's Harnesses and Machines pages tie each account to one machine

Triage (2026-11-23): waits for the owner's decision, P2, website. Two screens and the sample data tie each account to one machine, although accounts are shared by all machines. Basis: Entry state: needs the owner's decision because it changes existing screens; engine accounts code names no machine. It lives only in a screen that is being rebuilt.

Owner ruling (2026-11-25): Leave it open. The rebuild of the website decides it, because it concerns a screen that is being redesigned.

Found: 2026-11-20, while revising the draft Usage page in `design/mock/`.
State: Open. It needs the owner's decision, because it changes existing screens.

The owner said on 2026-11-20 that accounts are signed in through the server, so every machine has the same accounts to choose from.
The mock's sample data gives every account one machine.
The mock's Harnesses page shows that machine on each account, and its Machines page lists accounts under a machine.
`docs/features.md` records a request of 2026-11-06 that says each account "has its own machine".
The engine's account code in `packages/engine/src/accounts/` names no machine.
The draft Usage page no longer shows a machine on an account.
The Harnesses page, the Machines page and the sample data are unchanged.

## 1547. `design/DESIGN.md` and the mock disagree in many places

Triage (2026-11-23): waits for the owner's decision, P2, website. The design document and the screens disagree in thirteen places, and each needs a decision on which side is right. Basis: Entry state: each item needs a decision; items were reported by researchers and not all re-checked. It lives only in a screen that is being rebuilt.

Owner ruling (2026-11-25): Leave it open. The rebuild of the website decides it, because it concerns a screen that is being redesigned.

Found: 2026-11-20, by the screen reference research in `docs/research/2026-11-20-screen-references/`.
State: Open. Each item needs a decision on which side is right.

The researchers compared `design/DESIGN.md` with the mock's code and reported the differences below.
The designer did not re-check every item.
Each research file lists its own items under "What is planned and not built".

- Sidebar: the record says the service pages are one row of icons, the selected project has a second line, and the rail keeps the owner's initials. The code has six text rows, one line for every project, and no owner in the rail.
- Top bar: the record says "Inspecting" has a search field and groups by project. The code has a flat menu.
- Home: the record says filters stay selected on return. The code forgets the search, the menus and the sort.
- First run: the record describes three pages (what it does, what was found, the New project form). The code's steps are Subscription, Repository and First project.
- New project: the record lists a "Project name" field. The code has none.
- Board: the record says a card shows a goal line and a progress bar. The code shows the goal on hover and progress as a count.
- Sprint page: the record says a sprint's status is one line under its title. The code shows a row of seven labelled facts.
- Git: the record describes a worktree rail, sync pinned at the top, a Conflicts section above Staged, "Open in Terminal", and an "All files" view. The code has a worktree menu, an integrate strip, a separate Conflicts view, and none of the other three.
- Terminal: the record says a session is renamed in place by a double click or F2. The code opens a dialog and has no F2 handler.
- Thread: the record describes "Delivery details", "Message not delivered" and "Refresh this request". The code renders none of them.
- Machines: the record has "New project here", "Over limit", Disconnect, and a second placement table. The code has none of them.
- Service settings: the record describes one scrolling page with no Save buttons. The code shows one section at a time, and several forms have Save buttons.
- Notifications: the record says recent notifications are a four-column table. The code has a list.

## 1548. Explaining sentences remain on many screens of the mock

Triage (2026-11-23): waits for the owner's decision, P2, website. Many screens still carry explaining sentences, which the owner ruled out, and removing them from existing screens needs his approval. Basis: Entry: e.g. the Git sentence in design/mock/js/screens/git.js; removal needs the owner's approval. It lives only in a screen that is being rebuilt.

Owner ruling (2026-11-25): Leave it open. The rebuild of the website decides it, because it concerns a screen that is being redesigned.

Found: 2026-11-20, by the screen reference research.
State: Open.

The owner said on 2026-11-20 that a screen gets no helper or instructional sentences.
The Project view and the Usage page follow that.
The researchers found explaining sentences on these screens: the "Inspecting" menu, the login field, Add repository, the first run, the Workflow tab's phase detail and settings rows, the Workflows page, the Git tab ("Stage the files that belong in the next commit."), the Terminal's watch bar, the Files tab, the Mocks tab, the repository Design page, and the roster's bar about the stall watch.
The designer checked the Git sentence, and it is in `design/mock/js/screens/git.js`.
These are existing screens, so removing the sentences needs the owner's approval.

## 1549. Home opens one request at a time, against the owner's newer rule

Triage (2026-11-23): waits for the owner's decision, P3, website. Home lets only one request be open at a time, against the owner's newer rule that each item opens on its own. Basis: Entry state: needs the owner's decision; design/DESIGN.md still records the one-at-a-time rule. It lives only in a screen that is being rebuilt.

Owner ruling (2026-11-25): Leave it open. The rebuild of the website decides it, because it concerns a screen that is being redesigned.

Found: 2026-11-20, by the screen reference research.
State: Open. It needs the owner's decision.

`design/DESIGN.md` records "Only one request expands at a time" for Home.
The owner said on 2026-11-20, about the Project view, that each collapsible thing opens and closes on its own.
Home still closes an open request when another one is opened.

## 1550. The live Git tab refuses two actions that the feature list calls supported

Triage (2026-11-23): open, P1, website, high confidence. The live Git tab refuses to abort a merge or update from a parent, although the feature list says both are supported. Basis: design/mock/js/screens/git.js:428 and :481 call refuse() when live.on is set.

Found: 2026-11-20, by the screen reference research. The designer checked the code.
State: Open.

`docs/features.md`, under "Git actions after the website connection", says the live website supports aborting a merge and updating from a parent.
In `design/mock/js/screens/git.js`, `abortMerge` and `updateFromBase` call `refuse()` when the screen is live.

## 1551. Small inconsistencies in the mock found by the screen research

Triage (2026-11-23): open, P3, website, medium confidence. Nine small inconsistencies exist in the screens, such as differing warning thresholds, a stale label and missing search results. Basis: Entry state open; all items are in design/mock screen code or sample data. It lives only in a screen that is being rebuilt.

Found: 2026-11-20, by the screen reference research. The designer checked the first four.
State: Open.

- The context meter turns amber at 70 percent on the roster (`js/screens/messages.js`) and at 80 percent in the message box (`js/ui.js`).
- The roster's total still reads "Subscriptions in use", although the Harnesses page replaced Subscriptions.
- The Workflow tab's settings stop at six review rounds. The build loop design removed the hard limit on 2026-11-11.
- A comment in `js/screens/git.js` still calls Files "a placeholder until the file view is chosen".
- The switcher cannot find the Usage and Workflows pages, and teams are not results.
- The usage meter turns amber at 80 percent in the code and at 90 percent in `design/DESIGN.md`.
- The end result appears in two places: the Project view and the Workflow tab's End result view.
- The mock's sample data has no General channel, although the feature list says each project starts with one.
- `design/DESIGN.md` says Control B does nothing while the terminal has focus. The researcher did not find that guard in the code.

## 1552. The mock has no place for "About this agent" since the Chat tab was removed

Triage (2026-11-23): open, P2, website, medium confidence. Removing the Chat tab from the design left no place for the agent details panel or the permission mode menu. Basis: Entry state: open, needs a design; the live website is unchanged and still has its Chat tab. It lives only in a screen that is being rebuilt.

Found: 2026-11-20, while moving the conversation to the left of every project view in `design/mock/`.
State: Open. It needs a design.

The owner decided on 2026-11-20 that the mock's Chat tab can go, because the conversation now stays on the left of every view.
The Chat tab also held a panel named "About this agent".
That panel showed the agent's task, harness, account, model, machine, context and run, and it held the menu that changes the agent's permission mode.
The panel is no longer reachable in the mock, and neither is that permission menu.
The Chat tab's count of open requests is also gone from the row of views; the count in the line of stages remains.
Another agent's thread in the conversation's column still shows the earlier request cards, not the new decision form.
The live website is unchanged and still has its Chat tab.

## 1553. A tooltip in the row of project views is cut off

Triage (2026-11-23): open, P3, website, medium confidence. A tooltip on a button in the row of project views is clipped to a thin dark line. Basis: Entry state open; a layout fault of the present mock screen only. It lives only in a screen that is being rebuilt.

Found: 2026-11-20, while checking the button that hides the conversation in `design/mock/`.
State: Open.

The row of project views scrolls sideways, so it clips anything that leaves its box.
A tooltip on a button in that row opens below the button and is cut to a thin dark line.
The button "Project lead" at the end of the row shows this.
The fault was there before the conversation moved to the left.

## 1554. The documented way to serve the mock lets a browser mix old and new scripts

Triage (2026-11-23): open, P2, documents, high confidence. The documented command for serving the mock lets a browser mix old and new scripts, which can leave a gray page. Basis: design/README.md:12 and design/mock/README.md:21 still give plain `python3 -m http.server`; neither mentions no-store. It lives only in a screen that is being rebuilt.

Found: 2026-11-20, when the owner got a gray screen after a change to `design/mock/`.
State: Open. The session's own server works around it.

`design/README.md` and `design/mock/README.md` say to serve the folder with `python3 -m http.server`.
That server sends no caching header, so a browser decides on its own how long to keep each script.
After a change that touched two scripts, the browser kept the old `js/app.js` and fetched the new `js/screens/mission.js`.
The old script asked for a name the new one no longer has, so the page failed to start and stayed gray.
A server that sends `Cache-Control: no-store` prevents this.
The two documents still name the plain command.

## 1555. Git and Files are too narrow beside the conversation

Triage (2026-11-23): waits for the owner's decision, P2, website. Git and Files content is very narrow beside the conversation and their lists, and the layout needs the owner's choice. Basis: Entry state: needs the owner's choice; the list was moved right on 2026-11-21 but the narrow content remains. It lives only in a screen that is being rebuilt.

Owner ruling (2026-11-25): Leave it open. The rebuild of the website decides it, because it concerns a screen that is being redesigned.

Found: 2026-11-20, by the owner, after the conversation moved to the left of every project view in `design/mock/`.
State: Open. It needs the owner's choice.

Git and Files each have their own list beside their content.
With the sidebar open, the page then has four columns: the sidebar, the conversation, the list, and the content.
On a 1440 pixel screen the diff in Git gets about 400 pixels.
On a 1280 pixel screen it gets about 295 pixels.
Both views already have a one-column layout for a phone: the list first, then one file with a back arrow.
That layout follows the width of the window today, not the width of the view.

On 2026-11-21 the owner said the width was not his main complaint.
The list sat in the middle, between the conversation and the content, and drew the eye.
The mock now puts the list of Git and Files on the far right.
The narrow content remains: the list and the conversation still take the same room.

## 1556. Smaller gaps left by the changes of 2026-11-20 in the mock

Triage (2026-11-23): open, P3, website, medium confidence. Four small gaps remain in the design: a missing paste button, limited unpinning, fixed pin order and a crowded session picker. Basis: Entry state open; all four are in design/mock screens. It lives only in a screen that is being rebuilt.

Found: 2026-11-20, while building the divider, the pins and "Paste in terminal" in `design/mock/`.
State: Open.

Another agent's thread in the conversation's column has no "Paste in terminal" button, because it still uses the older thread component.
A pinned project can only be unpinned from the open sidebar, not from its tile in the folded sidebar.
The pinned tiles cannot be put in a different order.
The session picker in the Terminal view shows a plain browser menu and crowds its row when the view is narrow.

## 1558. The conflict view in Git has no drag handle for its list

Triage (2026-11-23): open, P3, website, medium confidence. The conflict view in Git cannot have its list resized on its own, because two tests rebuild it from source text. Basis: Entry names packages/studio/test/git-refresh.test.ts and git-controller-runtime.test.ts as the constraint; state open. It lives only in a screen that is being rebuilt.

Found: 2026-11-21, while making the list of Git and Files resizable in `design/mock/`.
State: Open.

Two test files rebuild the conflict view from its source text and hand it a fixed list of names.
A new name inside that view fails those tests with "is not defined".
The conflict view therefore follows the width set in Changes, History or Files, and has no handle of its own.
The tests are `packages/studio/test/git-refresh.test.ts` and `packages/studio/test/git-controller-runtime.test.ts`.

## 1560. With its list hidden, Files has no way to open another file

Triage (2026-11-23): open, P2, website, medium confidence. With the list hidden in Files or Git, there is no way to open another file without showing the list again. Basis: Entry state open; the search box lives inside the hidden tree in design/mock. It lives only in a screen that is being rebuilt.

Found: 2026-11-21, while adding the button that hides a view's list in `design/mock/`.
State: Open.

The search box "Go to file" lives inside the tree of Files.
Hiding the list hides the search box too, so the owner must show the list again to open another file.
Git has the same gap for choosing another changed file.

## 1561. Faults the evaluation of the sprint page found

Triage (2026-11-23): waits for the owner's decision, P3, website. Of nineteen sprint page faults, three remain: a tall phone header, faint button edges and a request not naming its blocked task. Basis: Entry state: items 2, 9 and 19 are open and each needs the owner; the rest fixed on 2026-11-22, dates of item 12 excepted. It lives only in a screen that is being rebuilt.

Owner ruling (2026-11-25): Leave it open. The rebuild of the website decides it, because it concerns a screen that is being redesigned.

Found: 2026-11-21, by measuring the sprint page in `design/mock/` against the rules of the UI skills.
State: Open in part. Items 13 to 19 come from the blind review of the same day. The new layout of the sprint page, which the owner chose on 2026-11-22, fixed items 1, 3, 8, 10, 13, 14, 15, 17 and 18 in the mock. The owner asked on 2026-11-22 for all the issues to be addressed, and items 5, 6, 7, 11 and 16 were fixed that day. Item 4 was fixed the same day, after the owner said yes. Item 12 is fixed except for the dates. Items 2, 9 and 19 are open, and each needs the owner. The notes under the list say what was done.

The rules were the eight space checks, the scan and state rules of the screen-build skill, Vercel's Web Interface Guidelines, and the behaviour rules of Vercel's UI SDK.

1. **Open requests push the task list off the first screen.** With two requests the list starts at 71 percent of a 1440 by 900 window. On a phone it starts below the first screen, at 113 percent. The requests sit in the page's header, `design/mock/js/screens/project.js`, above the plan and the tasks.
2. **On a phone the header takes most of the first screen.** The task list starts at 61 to 69 percent. The facts strip alone is 143 pixels in three rows.
3. **A long sprint name is cut and cannot be read.** `design/mock/css/screens.css:216` cuts the name with an ellipsis. A name of 104 characters loses 357 pixels, and the heading has no tooltip. Wrap or cut is the owner's decision.
4. **The coloured left edge is still on other parts.** The owner had it removed from the question dock on 2026-11-20. It is still on a request block (`design/mock/css/base.css:540`), on a blocked task row (`design/mock/css/base.css:181`), on an expanded request row (`design/mock/css/screens.css:38`) and on a post of the message board (`design/mock/css/messages.css:49`).
5. **The menu says nothing to a screen reader while the arrow keys move.** `design/mock/js/ui.js:128` keeps focus on the button and marks the active item only with `data-active`. It sets no `aria-activedescendant`. The key listener at `design/mock/js/ui.js:144` is attached again after each move, so two fast presses can lose one.
6. **A dialog leaves the page behind it reachable.** `design/mock/js/ui.js:174` traps the Tab key by hand. The page behind is not marked `inert`, so a screen reader can still walk into it. The dialog sets no `overscroll-behavior`.
7. **Two requests give two buttons of the same name.** "Deny", "Details" and "Open thread" each appear twice on `/p/console/sprint/sp3`, from `design/mock/js/ui.js:458`. A screen reader user, or an agent that uses the page, cannot tell which request a button belongs to.
8. **The facts strip has uneven padding.** `design/mock/css/screens.css:221` puts the words 7 pixels from the top and 11 from the side. At some widths the last fact wraps alone onto a second row.
9. **A button's edge is too faint to count as an edge.** The border of `.btn` at `design/mock/css/base.css:46` has a contrast of 1.58 to 1 in light mode and 1.91 to 1 in dark mode. The rule for the edge of a control is 3 to 1. The label inside is readable.
10. **The plan pane takes half the page to say "No plan yet".** A sprint without a plan could open on its tasks alone.
11. **The sample contradicts the new rule.** SP-02, SP-03 and SP-04 are in progress or in review and have no plan, and In progress now needs an approved plan.
12. **Smaller gaps against the Web Interface Guidelines.** The mock sets no `touch-action: manipulation`, no tap highlight colour, no `text-wrap: balance` on a heading and no skip link. The choice of Plan or Tasks is kept in the browser and not in the address. The roadmap writes dates with the fixed locale `en-GB` at `design/mock/js/screens/project.js:146`.
13. **The plan and the tasks are two fixed columns, whichever has content.** On a sprint with a plan and no tasks, the plan reads at about 37 characters a line beside a blank column. The blind reviewer of 2026-11-21 found this.
14. **Tasks are two-line rows on a desktop.** They are 55 to 74 pixels tall beside the plan, and have no column headings. "Sprint and task pages" in `design/DESIGN.md` says tasks are one line. The number at the right end of a row has no label.
15. **The Plan and Tasks switch shows no state.** Both are on by default, and both look the same as a button that is off would.
16. **Button heights on a phone favour the wrong action.** A plain button is about 36 pixels tall and the small Accept and Deny of a request are about 28.
17. **A plan repeats the sprint's name as its own first heading.** It is the largest type on the page, about 215 pixels under the same name in the page's title.
18. **Two default values are blue.** They are the "TL;DR:" label and the "Active" pill of a task. Blue is neither the link colour nor the attention colour on this page.
19. **A request block does not name the task it blocks.** The owner matches it by reading "Blocked by a question" in the task list below.

What was done on 2026-11-22, by item:

- **Item 2 is open.** On a phone the task list starts at 40 to 53 percent. The owner said the phone is not the focus for now.
- **Item 4 is fixed.** The owner said yes on 2026-11-22. The coloured left edge is gone from a request block, from every table row, from an expanded request row and from a post of the message board. The rule for a blocked task row was the rule for every table, so the agents tables and the projects table lost the edge too. The edge is still on five parts the item did not name: a subscription row (`design/mock/css/service.css:112`), an agent row of the message board (`design/mock/css/messages.css:160`), a rail of the Usage page (`design/mock/css/usage.css:47`), a list on the Mocks page (`design/mock/css/mocks.css:82`) and a merged pull request on Workflow (`design/mock/css/workflow.css:143`). The request block's padding is 12 pixels on each side.
- **Item 5 is fixed.** The item under the arrow keys now holds the focus, so a screen reader says it. The key listener is attached once while the menu is open. Home and End work, and Tab closes the menu.
- **Item 6 is fixed.** The page behind a dialog is marked `inert`, and the dialog's body sets `overscroll-behavior`.
- **Item 7 is fixed.** Each button of a request block is described by its block's question, through `aria-describedby`. The names stay "Accept", "Deny" and "Details", because the browser journeys of the live website look for those exact names.
- **Item 9 is open.** The edge of a button comes from the approved palette in `design/lgtm.fig`. The change belongs in the design file.
- **Item 11 is fixed.** SP-01, SP-02, SP-03 and SP-04 each have an approved sample plan in `design/mock/plans/`.
- **Item 12 is fixed in part.** The mock now sets `touch-action: manipulation`, a tap highlight colour and `text-wrap: balance` on headings. The first Tab stop is "Skip to content". The open tab of a sprint is in the address. Dates still use the fixed locale `en-GB`. Following the browser's language would change how every date on every screen reads, so it waits for the owner.
- **Item 16 is fixed.** On a phone every button of a request block is 36 pixels tall.
- **Item 19 is open.** The design document says a brief shows the question, the wait and one line. Naming the blocked task adds to it, so it waits for the owner. The Questions view of issue 1567 names the blocked task and links it, so approving that view may close this item.

## 1566. Smaller faults the blind review of the new sprint page found

Triage (2026-11-23): waits for the owner's decision, P3, website. In dark mode the attention colour and the link colour look alike, and changing the approved palette needs the owner. Basis: Entry: only item 2 is open; colours come from the approved Dusk palette in design/lgtm.fig; other items fixed or closed. It lives only in a screen that is being rebuilt.

Owner ruling (2026-11-25): Leave it open. The rebuild of the website decides it, because it concerns a screen that is being redesigned.

Found: 2026-11-22, by a reviewer who saw only screenshots of the sprint page in `design/mock/`.
State: Open in part. Item 2 is open. Item 4 was closed by the owner. The others were fixed on 2026-11-22.

1. **Two controls moved a sprint.** Fixed. The Status fact is a plain value. The move button has an arrow beside it that opens the menu of all five lanes. A finished sprint shows "Move to…" with the same menu.
2. **In dark mode the attention colour and the link colour look alike.** Open. At 13 pixels "Blocked by an approval" and a link in the rail read as the same warm tone. The colours come from the approved Dusk palette in the design file `design/lgtm.fig`, so the change belongs there and needs the owner.
3. **A plan's headings were off the type scale.** Fixed. `design/mock/plans/plan.css` now uses 20, 15 and 13 pixels for headings and 13 for text.
4. **A plan opens with a Goal section that repeats the goal in the rail.** Closed, not a fault. The owner decided on 2026-11-22 that the goal in the rail is the summary of the plan and stays. His rule is that one view does not say a thing twice, and a short form beside its long form is allowed. The rule is in `design/DESIGN.md`.
5. **The two request cards had different shapes.** Fixed. On a card narrower than 470 pixels "Open thread" is its icon, with the name on hover, so a question and an approval keep their buttons on one line, on a desktop and on a phone.
6. **On a phone the facts line wrapped with a hanging dot.** Fixed in part. The dot now follows its word, so no line starts with one. Phase and Epic are still bare words without a label.
7. **On a phone the task rows were small touch targets.** Fixed. A stacked row is 54 pixels and opens the task, and the plan button is 40 pixels for a finger.

## 1567. The approved Questions view has open points

Triage (2026-11-23): open, P2, website, medium confidence. The approved Questions view has many open points, is not on the live website, and agents are not told to write its sections. Basis: Entry lists the open points: view not wired live, Define questions absent, no instruction for Background and Impact, several owner preferences.

Found: 2026-11-22, while building the owner's idea of a place for questions.
State: Open. The owner approved the view on 2026-11-22. The points below are still open.

The owner proposed a tab for one question on 2026-11-22. The first prototype put that tab on the sprint page. The owner answered the same day that the tab belongs beside Board, with a list of the questions and a full view of one question, and that a question then has no expanded view. The tab on the sprint page was taken out.

The mock now has a Questions view. It is the third view in a project's row of views, and its tab counts the questions that wait. The middle holds one question as a page to read. The page is a report to skim. The question stands alone at the top. A block of facts follows: who asked, what it blocks, how long it has waited, and "Open thread". Ruled sections follow: "Background" and "Impact" as short points. The answers come last, with the agent's recommended answer first and marked, each with a "Choose" button, and then "Your own answer". The owner asked for this order on 2026-11-22: no Accept button and no recommendation at the top. A first build ended with "Before it asked", the last messages of the asking agent. The owner had it removed the same day, because the list on the right already shows the questions over time. The list of the project's questions is on the far right, in the groups Waiting and Answered, and the list's button hides it. The addresses are `/p/console/questions` and `/p/console/questions/q-open`.

A question's card no longer opens a report in place. Its Details opens the question in the Questions view, and so does Deny, with "Your own answer" focused. This holds for a card in an agent's thread and on the sprint page. In the lead's conversation, a question above the message box has Details in place of "Why it matters" and "Open thread".

The owner approved the view on 2026-11-22, and `design/DESIGN.md` has its rules under "The Questions view". These points are open:

- Approvals are in the view for now, since the owner's yes of 2026-11-22. He said that an approval is an agent's action and not a question, and that it needs a better design of its own. The tab still reads "Questions" although it lists and counts approvals. The owner said on 2026-11-22 not to rename it, because approvals will get a home of their own later.
- An approval's page lists four decisions, each with "Choose". On a card the same four are two buttons, each with a menu. On the page the one-time Allow reads "Allow once", and on a card it reads "Allow".
- A row of Home that is a question or an approval now opens the Questions view. It can no longer be answered on Home. A merge and a failed start still open in place, so Home's rows behave in two ways.
- The questions of the Define phase are not in the list. A project in Define shows "No questions" in the view while the bar above says that questions are open.
- The page leaves out the agent's own wording of the question when the question has a report, because the report says it again. A question with no report shows that wording as its "Background".
- A long answer wraps in the list and is not cut. The owner may prefer it cut to two lines.
- The live website does not have the view. A question's card there still opens its details in place, until builders wire the view.
- In an answered row of the list, the answer stands beside the picture of the agent that asked. It reads as if the agent gave the answer. The owner's picture or a check mark would say it right.
- The two groups of the list run in opposite directions of time. Waiting starts with the longest wait. Answered starts with the newest answer.
- The question page shows "Background" and "Impact" as short points, one claim each. An agent must write them that way. Nothing tells an agent to yet, so a real question would arrive as paragraphs.
- Every answer on the page has an outlined "Choose" button, as the design document says of a request's buttons. The layer rules of the screen-build skill want one filled button in a row of actions.

The blind review of 2026-11-22 added these points. Its table is in `design/builds/questions-view.md`.

- The answers are the last thing on a tall page. With a long question and three answers, "Your own answer" is below the fold at 1440 by 900. The owner asked for the answers at the end.
- A row of the list is 54 pixels high. The design document allows 46 for a record. The asking agent's picture sets the height of the second line.
- An answered question shows an age in the list, such as "4 hr 56 min", and a clock time on its page, such as "16:33".
- Blue marks every "[1]". Green marks "Recommended" and "Your answer", which cannot be pressed, and green is also the colour of a link.
- A reference under "References" is plain text. It does not open the file.
- At 1920 each "Choose" stands far from its answer, and the lower part of both panes is empty.
- On a phone the question's page edge is 13 pixels and the list's is 17. The word "Questions" shows three times above the question.

## 1569. The blind review of the row of views and the stage line found fifteen faults

Triage (2026-11-23): waits for the owner's decision, P2, website. A blind review of the bars above a project found fifteen faults; several were settled, and the rest need the owner's yes. Basis: Entry: findings 1, 3, 4, 7, 8, 10, 11, 14, 15 open plus new points after the stage line removal; both bars are approved screens. It lives only in a screen that is being rebuilt.

Owner ruling (2026-11-25): Leave it open. The rebuild of the website decides it, because it concerns a screen that is being redesigned.

Found: 2026-11-22, by a blind review the owner asked for.
State: Open. Both bars are approved screens, so each change needs the owner's yes.

The owner asked on 2026-11-22 for a review of the two bars under the top bar of a project: the row of views, and the stage line under it. A fresh reviewer was sent nineteen pictures of them, across six views, the widths 1440, 1920, 1100 and 390, dark mode, and three projects in different stages. It read the design document. It did not see the source. Its verdict was "no", with no blocker, eight findings labelled major and seven labelled minor. Nothing was changed.

The findings labelled major:

1. Three amber counts sit in three stacked rows: 7 on the bell, 2 on the Questions tab, and "4 requests" in the stage line. They share one colour and one shape, and none names what it counts. The project's 4 is also in the sidebar, on the Project lead's row, and in "Waiting on 4 answers", in three wordings.
2. Nothing in the stage line shows what can be pressed. The done stages look like unselected tabs, so the line reads as a second row of tabs with "Build" chosen. "4 requests" is drawn like a label, although it opens the waiting decisions.
3. The stage line and the board's own line disagree. For the project "Phone companion" the stage line marks Design, and the board's line 40 pixels below reads "Define · Phase 1 of 8". The stage line has six stages, and the board's line counts eight phases.
4. The conversation button and the list button are tinted green while their panel shows, which is their normal state. The tint is the same as the current stage's. The current view, which the owner needs most, has only dark text and a thin underline.
5. Five ruled rows and about 198 pixels stand above the first line of a diff in Git. The stage line takes 36 of those pixels on every view for one stage and one count, and more than half of its width is empty.
6. At 1100 pixels the word "Board" shows three times in three stacked rows. "Current" is drawn four ways within 120 pixels: an underline, a green pill, and two white raised segments. The conversation's control changes its bar, its end and its shape between 1440 and 1100.
7. The tabs move between projects. Without a count on Questions, every later tab sits 26 pixels to the left. Without Mocks, Agents to Settings sit 64 pixels to the left. Answering the last question would move seven tabs under the pointer. The design document says the tab row stays still.
8. The tabs sit over the wrong column. They span 325 to 1003 pixels, and the pane they change starts at 758. The underlined "Board" stands above the conversation.

The findings labelled minor:

9. "Review" and "Hand over" in the stage line have a contrast of about 3.65 to 1 in light mode. The design document asks for 4.5 to 1.
10. A project with no Mocks tab shows Design with the check of a finished stage. It does not say whether the stage was done or skipped.
11. The left and right ends of the three bars share no line. On the left, six elements start at six places between 276 and 325 pixels. On the right, three end at 1419.5, 1424 and 1428.
12. A dash in the stage line sits 12.5 pixels after the stage before it and 8 to 9.5 pixels before the next.
13. At 1100 pixels "4 requests" starts at 838 on Board and at 856 on Git, because the switch beside it is as wide as the view's name.
14. On a phone "Mock" is cut at the screen's edge with no fade, and the stage line shows only "Build". The "Inspecting" line of the design document is absent.
15. The tab reads "Settings" and the design document's Names table says "Project settings". The first tab reads "End result" and the design document also calls it "the Project tab". The speech-bubble icon at the left end can read as "open chat".

The reviewer also found a gap in the pictures. The worst-case data changes nothing in the two bars, so a long project name, a long branch name and a count of three digits were not judged.

### The stage line was removed on 2026-11-22

The owner said yes on 2026-11-22 to removing the stage line as a row. The stage is now one word at the right end of the row of views, and it opens the list of the six stages. The view below starts 36 pixels higher.

- Findings 2, 5 and 12 are settled. Nothing in the row looks like a second row of tabs, the row is gone, and the dashes are gone.
- Finding 9 is settled. A later stage in the list has the muted colour, not the faint one.
- Finding 13 is settled above 1100 pixels. The stage sits at one place on every view.
- Finding 6 is settled. The owner's window is 1100 pixels or narrower, and he saw the stage on a third line there. That line is gone at every width. The conversation button at the left end of the row switches between the conversation and the view.
- Finding 1 is open. The count of requests still stands in the row, before the stage. Below 1280 pixels it leaves the row.
- Finding 3 is open. The stage has six values, and the board's own line still counts eight phases.
- Finding 10 is open. A skipped Design stage still has the check of a finished one, now in the list.
- Findings 4, 7, 8, 11, 14 and 15 are open and unchanged.

Two new points came with the change.

- The list of stages uses the menu part, with the list in the menu's head and no items. A screen reader hears a menu with nothing to choose. A builder should give it the part for a plain pop-up.
- Below 1280 pixels the count of requests is not in the row. With the conversation hidden, only the Questions tab and the bell then say that something waits.
- Below about 1140 pixels the tabs of the Agent console do not all fit beside the stage. They scroll under it. At 1000 pixels two tabs are out of sight until the row scrolls. The stage takes 77 of those pixels.
- On a narrow screen the switch between the conversation and the view is an icon with no word. The two worded buttons it replaced were easier to find.
- On a phone the stage is not shown at all.
- A tab that scrolls under the conversation button is cut with no fade.

## 1570. The space checks find three older faults in the lead's conversation

Triage (2026-11-23): open, P3, website, medium confidence. Space checks found large empty areas under the lead's conversation and an uneven margin on one document. Basis: Entry state open; three layout measurements at 1440 pixels on design/mock pages. It lives only in a screen that is being rebuilt.

Found: 2026-11-22, while measuring the waiting decisions above the message box.
State: Open.

The space checks were run on three pages at 1440 pixels. They found three faults. The same three show on the commit before the alignment change, so the change did not cause them.

1. On the Board of the Agent console, 308 pixels are empty under the last message of the lead's conversation, in a pane 653 pixels tall.
2. With worst-case data the same pane has 182 empty pixels, in a pane 568 pixels tall.
3. In the Standing permission rules project, the document beside the conversation starts 11.6 pixels from the top and 20 pixels from the left.

## 1571. The port to Preact 11 found fourteen faults in the mock's code

Triage (2026-11-23): open, P2, website, medium confidence. Porting the screens found fourteen code faults, mostly in live website paths, and the website build cannot yet use the port. Basis: Entry: the port in design/mock-vite carries each fault as it is; packages/studio/scripts replacements will not match the TypeScript files.

Found: 2026-11-22, by the builder that ported `design/mock/` to `design/mock-vite/`.
State: Open. The port carries each fault as it is, so the two mocks behave the same.

Typing the code showed these faults. None changes what a screen shows on sample data. Most sit in the code for the live website.

1. `js/screens/service.js` calls `board.changeApproval`, `changePermission` and `deleteTeam`. Those functions live in `js/live/project-settings.js`.
2. `draggable` and `spellcheck` receive the text "false". Preact reads that text as true, so spellcheck stays on.
3. `confirm()` drops `waitForReply` and `pendingLabel`.
4. Deleting the last workflow can open a workflow that does not exist.
5. `blank(p)` ignores its argument.
6. Five live code paths can throw: a frame whose screen is missing, `design?.components.length`, `x.window` in a sub-row, `target.id` in the Git refresh, and `old.selection.head` in the Git detail.
7. Selecting in Git can store the key "undefined".
8. `mapMainReview` sets `commits` and `files` to null where numbers are expected.
9. The terminal launcher's `value` should be optional.
10. The Vim screen has a `focused` prop it never uses.
11. `Menu` relies on its callers to remove false items.
12. The code editor ignores `busy`, so the live Files tab stays editable during a save.
13. Files has a fallback for a pair form that no data uses.
14. The table view gave an icon a `style` prop that did nothing. The port removed it.

Two more points concern the port itself.

- The text replacements under `packages/studio/scripts/` are written for the JavaScript files of `design/mock/`. They will not match the TypeScript files. The build of the live website must be rewritten before it can use the port.
- The port keeps loose types in about 60 places: 21 uses of `any`, 18 uses of a loose record type, and 21 double casts. Most are replies from the engine.


## 1572. The owner's review of the website on 2026-11-23

Triage (2026-11-23): waits for the owner's decision, P2, website. The owner's review of every screen left eleven questions awaiting his answers and eight blind review findings open in the new port. Basis: Entry: first list done in design/mock-vite; second list waits for the owner's answers; table in design/builds/review-2026-11-23.md.

Owner ruling (2026-11-25): Leave it open. The rebuild of the website decides it, because it concerns a screen that is being redesigned.

Found: 2026-11-23, by the owner, who read every screen of the mock.
State: Open. The first list is done in the port, `design/mock-vite/`. The second list waits for the owner's answers.

The owner said that nothing in the mock is frozen for this review. All changes are in the port only. The older mock in `design/mock/` and the live website do not have them.

Done in the port:

1. A project opens on the Board. The End result was the first view before.
2. The row of views no longer repeats the count of requests. The Questions tab carries the count.
3. The button that shows and hides the conversation is named "Project lead".
4. The End result is a document. Each statement is one line with its state as a word. The "Needs you" list became one button that opens Questions. A statement names the question that waits on it.
5. A waiting decision above the message box has two lines: the question, then its kind, its asker and its wait. A button beside the fold opens the question in Questions.
6. The separators between waiting decisions are brighter in dark mode.
7. A question's answers are radio buttons, or checkboxes when more than one may be chosen, with one "Send answer" button.
8. "Open thread" is named "Discuss", and it sits beside the asker's name.
9. A reference on a question's page is a link to its file.
10. An approval has two buttons, "Allow" and "Deny". Each has an arrow that offers the lasting answer. No sentence explains them.
11. The question page had three layouts behind a switch. The owner chose on 2026-11-23: the order of the report with the one-line header. The switch is removed.
12. A sprint's lane is one button at the top of its rail. Its menu moves the sprint. The words "Next lane" and "Force to" are gone.
13. A move that stops a working team asks first. The dialog offers "Safe shutdown" and "Terminate".
14. A fix sprint's label has its own column in Workflow, and the columns have space between them.
15. The agents under Waiting and Working keep one width for the name, so the activities start on one line.
16. A review round is a small bar chart. Each reviewer has a bar, a colour and its points. The round's total is under the bars, and everything is centred.
17. A round's findings are rows of two lines: what was found, then its number, its statement and its reviewer.
18. Workflow has no End result tab. The project's own End result view replaces it.
19. The workflow's settings are a page of the project's Settings.
20. A screen's picture in Mocks is whole, and it no longer touches the next one. Its note sits under its name.
21. The preview of the request block on the Components tab is as tall as the other previews.

Open, and each needs the owner's answer before it is drawn:

1. Home. The owner wants a talk about what "Needs attention" is for, and less clutter.
2. The three controls at the top right: "Live", the bell and Home. The owner says all three can be given another job.
3. The two top bars. The branch menu is out of place.
4. The stage. It is too far from the eye at the right end of the row of views. Drawn on 2026-11-23: the Status tab carries the stage, and the control at the right end is gone. It waits for the owner's look.
5. The sidebar. The plus beside "Repositories" adds a project, not a repository. A project needs its own button, and repositories need their own place.
6. The End result. The owner asked whether it should have a short form and a detailed form.
7. The Phases tab of Workflow. It cannot be skimmed within a second. The owner asked what brings a person to this view at all. The owner answered on 2026-11-23 that it is a status page about agents, not about tasks. Drawn the same day: the view is named Status, its first tab is Now, the phases are one strip, and the agents come first. It waits for the owner's look. The owner also suggested a new layout for the command center, and that is not drawn.
8. The Test runs tab of Workflow. The owner gave no change for it at first. The owner decided on 2026-11-23 to reuse the workflow elsewhere and to replace the tests with it. Drawn on 2026-11-23: it is a view of its own named Actions, after Status in the row of views. A run names its workflow file, and a Build action stands beside the test ones. It waits for the owner's look.
9. The Agents view. The owner set it aside. The owner decided on 2026-11-23 that the tab becomes the message board. Drawn the same day, with a page for one agent beside its conversation. Issue 1593 lists what the Agents view held that has no place now.
10. The Design view. A project may change the design too, so the view may belong in a project, where it could clash with Mocks.
11. Where approvals live. They are in Questions for now.

A blind review of the changed screens ran on 2026-11-23. Its table is in `design/builds/review-2026-11-23.md`. These findings stay open:

1. Test runs cuts its step names, its log lines and its run titles while the conversation is open. A "Jobs" column holds one item, and a run's state is an icon with no word. Fixed in part on 2026-11-23: the jobs are one row above the steps when the run is narrow, so no step is cut. A run's title in the list is still cut, and a run's state in the list is still an icon.
2. The deciding control of a question and of an approval is below the fold in the "Report" layout. At 1000 by 680 an approval's "Allow" and "Deny" are out of sight.
3. A state is a pill in some places and a word in others: "Approved" in Mocks, "Being scored" in Review rounds.
4. The sprint's tabs are underlined, and the tabs of Workflow, Mocks and Settings are filled.
5. Mocks says "Approved" in its status line and again on every row.
6. A long answer in the list of Questions is cut after two lines.
7. Two rows of the workflow's settings are facts that cannot be changed, and they look like settings.
8. "Stage" in the row of views and "Phase" in Workflow name one thing. Since 2026-11-23 the Status tab shows the stage, of six, and the strip under it shows the phase, of eight. The two lists are still two.

Three points are about the port's own code.

- The switch between the three layouts of a question was a prototype control. It was removed on 2026-11-23, when the owner chose.
- The colours of the three reviewers are three new tokens, `--color-series-1` to `--color-series-3`. The design document does not name them yet.
- The sample data gives each finding to one of three reviewers in turn. The engine does not record which reviewer made a finding.

## 1573. The feature list holds the usage page twice

Triage (2026-11-23): open, P3, documents, high confidence. The feature list holds two entries for the same usage page idea, and they should be merged into one. Basis: docs/features.md:1919 "A usage page for every connected harness and account" and :2098 "A Usage page for every harness account".

Found: 2026-11-23, while merging `master` into `area/define-redesign`.
State: Open.

`docs/features.md` has two entries for one idea, both from 2026-11-20.
"A usage page for every connected harness and account" came from `master`, and it is short.
"A Usage page for every harness account" came from the Define redesign branch, and it holds the owner's words and the open points.
They should become one entry.

## 1575. One failed write stopped every later write of the stable service for two days, with no sign

Triage (2026-11-23): open, P0, engine, high confidence. After one failed save the service silently refused every later save for two days while the website kept answering normally. Basis: packages/engine/src/log/EventLog.ts:140, :243, :282 keep the first failure in `latched` and fail every later write; no change since.

Found on 2026-11-23, when an owner command to the stable service was refused with "database write failed".
The event log keeps the first write failure for the life of the service and refuses every later write with it (`packages/engine/src/log/EventLog.ts`, the `latched` value).
The last stored event is from 2026-11-21 at about 10:40 local time; the refusal was found on 2026-11-23.
The database passed SQLite's quick check, the disk had 70 GB free, and no other process held the file.
A restart of the service cleared the fault, and the same command then succeeded.
The first failure's cause is unknown, because issue 1576 means nothing recorded it.
The website kept answering, so nothing told the owner that the service could no longer save anything.
State: Fixed in unit f1-eventlog. A failed write is now logged with its fixed reason and SQLite result code, reported by the owner's version read, refused from memory for a pause of two seconds that doubles up to one minute, and then retried against the database, after the live views are checked against the log.
The original cause is still not found, because nothing recorded it; the next one will be in the service log.

## 1576. A detached service discards its log

Triage (2026-11-23): open, P1, engine, high confidence. A service started in the background throws away its log, so errors such as a failed save leave no trace. Basis: packages/cli/src/serve.ts:24 starts the detached service with stdio ["ignore","ignore","ignore","ipc"].

Found on 2026-11-23 while diagnosing issue 1575.
`lgtm serve --detach` starts the service with its output sent to `/dev/null`.
An error that the engine logs, such as a failed write, therefore leaves no trace.
State: Fixed in unit f1-eventlog. The service now writes its redacted, filtered log to `service.log` in its state folder with mode 0600, renamed to `service.log.1` at 1 MiB, so the two files never exceed 2 MiB.

## 1609. Runner events that arrive while writes fail are dropped for good

Found on 2026-11-23 in unit f1-eventlog, while making failed writes recoverable for issue 1575.
The command handler appends each adapter event in its own writer turn (`packages/engine/src/commands/CommandHandler.ts`, `handleRunnerEvent`).
When that append fails, the event is logged as "runner event dropped: processing failed" and is not retried.
Before unit f1-eventlog this did not matter much, because every later write failed too until a restart.
Now writes can recover while agents keep running, so an agent's thread can miss the turns, tool calls or permission requests that arrived during the failure.
A permission request that was dropped this way leaves the agent waiting on an answer that nobody can give.
State: Fixed in unit f1-eventlog, review round 1. An agent event whose write fails is held in memory, in order per agent, and written when writes work again; nothing acts on it before it is written.
When one agent holds more than 256 events, the held events are dropped, every write is refused, and the version read reports `restartNeeded` with the reason "agent events were dropped while writes failed", because a restart's recovery repairs the agents' state.
Review round 2 narrowed it.
An event is held only when the event log refused it for a storage failure that any other write would also hit.
An event that fails for a reason of its own, such as a constraint, is dropped with the line "runner event dropped: write refused" and does not mark the database as failing.
A held event's retries reuse its first request id, body hash and generated item id, so a write that reported failure but was stored is found by its receipt and is not written twice.
A second bound of 32 MiB across all agents applies, and reaching either bound logs one line.
The version read counts the held events and their agents.

## 1610. A piece reactor's ordinary failure reaches the service log only as "service diagnostic"

Found on 2026-11-23 in unit f1-eventlog.
The service logs "piece reactor failed" with the reactor's namespace when a piece follower fails with a defect (`packages/engine/src/service.ts`, `pieceFollowers`).
That message is not in the log filter's fixed list (`packages/engine/src/api/serviceLog.ts`), so the line reads "service diagnostic" without the namespace.
The owner cannot tell from the log which follower is failing and restarting every second.
State: Fixed in unit f1-eventlog, review round 1. The log filter now passes "piece reactor failed" and "piece reactor paused: database write failed" with the reactor's namespace, when it is one of the fixed piece names.

## 1611. A retry that finds its receipt does not clear the failing-writes report

Found on 2026-11-23 in unit f1-eventlog, review round 2.
A write that reported failure but was stored is found by its receipt when it is retried (`packages/engine/src/log/EventLog.ts`, `commit`).
A receipt changes nothing in the database, so the event log does not count it as proof that writing works again (review round 1, item 5).
If every waiting write was such a retry, the version read keeps reporting `failing: true` until some other write changes the database.
On an idle service that can last a long time, although writing works.
State: Open. The write path could treat a transaction that took the write lock and committed, even an empty one, as proof, but that changes what review round 1, item 5 asked for.

## 1612. Minor findings of the third review of the event log recovery unit

Found on 2026-11-23 by the third review of unit f1-eventlog.
Two failure reasons that belong to one request still count as an outage: "receipt insert returned no result" and "stored harness settings are unreadable".
One corrupt settings row and a client that polls the harness list could therefore hold the write pause near 60 seconds for every command; this was read, not run.
The stale failing report of issue 1611 keeps its failure count, so the next real failure starts at a 4-second pause, not 2 seconds.
The test for a failed ROLLBACK covers the outer transaction only; a failed savepoint rollback is untested and would report "rollback failed" with no restart request.
The tests for the flush order and the byte bound replace the event log with a double, so they test the handler only, and one depends on finishing before the 2-second flush.
The held-event bound counts the UTF-8 length of the JSON, so real memory is about double, and every runner event is now serialised once even when writes are healthy.
If the log diverges for another reason while events are held, each held event is dropped with its own log line.
After the bound is reached, events are dropped and every write is refused until a restart, even if the database recovers; that is by design and is reported.

State: Open.

## 1602. A start that fails while its agent is paused writes an event the state machine rejects

Found: 2026-11-23, in unit f2-lifecycle, while fixing issue 267.
State: Open.

A sign-out can pause an agent while its first session is still being opened.
If that setup then fails, `packages/engine/src/commands/start.ts` writes `AgentStartFailed` and revokes the agent's token.
The state machine in `packages/contracts/src/stateMachine.ts` calls `AgentStartFailed` invalid in the paused state.
The writer does not check that table, so the event is stored and the agent stays paused.
The agent then has a recorded start failure, no process and no live token.
An account return later starts it again, so the work is not lost.
The stored history nevertheless contains an event that its own state machine rejects.

## 1603. A cancelled reply or a permission answer can skip its runner call when the caller goes away

Found: 2026-11-23, in unit f2-lifecycle, while fixing issue 251.
State: Open.

Issue 251's fix covers `start`, `stop` and `resume` only.
`cancel-reply` and a permission answer also commit first and then call the runner in `packages/engine/src/commands/CommandHandler.ts`.
A caller that goes away after the commit can still interrupt that call, or skip it at the boundary after the commit.
A committed permission answer whose runner call is skipped leaves the adapter waiting for an answer that the log records as given.

## 1604. A failed reload leaves no mark on the agent's snapshot row

Found: 2026-11-23, in unit f2-lifecycle, while tracing issue 1461.
State: Open.

`AgentReloadFailed` keeps the agent's state and sets no field in `packages/engine/src/views/apply.ts`.
A client that reads the snapshot therefore cannot tell a failed reload from one that was never attempted.
The failure is visible only in the agent's event history.
A fix needs a snapshot field, and showing it is a screen change for the owner.

## 1605. Restart recovery returns only after every agent's first send

Found: 2026-11-23, in unit f2-lifecycle, while fixing issue 253.
State: Open.

After issue 253, the step 10 sends run side by side, so a stalled adapter no longer holds the other agents.
`recover` in `packages/engine/src/recovery/restart.ts` still waits for all of them before it returns.
`packages/engine/src/service.ts` cleans up stale process rows and tells a foreground launcher that the service started only after that return.
An adapter that stops reading therefore delays both until a stop pre-empts its send.
A send deadline, or sends that outlive the recovery call, would remove the delay; the deadline is the owner's choice.

## 1606. A restart during an accepted first turn resends the role instructions with the carry-on

Found: 2026-11-23, in review round 1 of unit f2-lifecycle.
State: Open, for the owner to decide.

The engine owes the role instructions until a turn that carried them succeeds.
A turn the adapter accepted and that then failed, was cancelled or was cut off by a restart leaves them owed.
After such a restart, the carry-on therefore carries the full composed text again, which can be up to 48 KiB.
The loaded conversation usually holds that text already, so the repeat costs context.
The same repeat happens without a restart, when a carrying turn is cancelled after the adapter accepted it.
Letting acceptance discharge the obligation would remove the repeat.
That change would alter the s22 round 2 contract that `packages/engine/test/roles/instruction-sessions.test.ts` pins, so it is the owner's choice.

## 1577. Floorplan's normal local map does not show its export credit

Found on 2026-11-23 while implementing unit s33-codemap.
State: Fixed in unit s33-codemap.

Floorplan 0.11.0 creates its "Made with Floorplan" link only for exported pages.
The engine displays the ordinary local page without running the export script.
It therefore injects a visible footer credit when the local page has none.
When an exported page creates its own credit, the injected script keeps that link and removes the fallback.
Both forms open outside the frame with `noopener noreferrer`.
Tests cover an absent credit, an existing credit and a credit created during page boot.

## 1578. Floorplan's own script policy blocks an injected credit helper

Found on 2026-11-23 while reading Floorplan's page-safety helper for unit s33-codemap.
State: Fixed in unit s33-codemap.

Floorplan 0.11.0 writes a Content Security Policy meta element into its page.
That policy allows only scripts whose hashes were recorded when the map was built.
A script injected later cannot run unless its hash is also admitted there.
The engine adds only the credit helper's hash to Floorplan's marked script policy.
It retains the existing restrictions.
The response header independently blocks background resource requests and isolates the page from the owner.
Navigation remains possible, as recorded in issue 1580.
A test checks the injected script's actual hash against the delivered meta policy.


## 1579. The map policy blocked Floorplan's runtime styles and its reskin defeated theme switching

Found on 2026-11-23 in review round 1 of unit s33-codemap.
State: Fixed in review round 2.

Floorplan 0.11.0 creates nine style elements at runtime without nonces.
The response's style-element nonce policy blocked them.
The injected palette also overrode both themes with light colours.
The policy now allows inline styles without nonce handling.
The injection changes fonts and theme-specific accents while retaining Floorplan's palettes.
The credit helper's hash is admitted through both script policy directives.
A real map is generated offline from the reference project's own temporary shop fixture for the integration check.
A permanent synthetic fixture also creates a style at runtime and changes its theme.
The tests assert the complete response policy because jsdom cannot enforce it.

## 1580. An opened map can send its content away through navigation

Found on 2026-11-23 in review round 1 of unit s33-codemap.
State: Open limitation, with partial mitigation in review round 2.

The map's response policy isolates it from the owner's session and blocks background resource requests.
It does not block all navigation when the map is opened directly in a tab.
A page can navigate itself or open another window carrying its content and its own token address.
The engine strips automatic meta refresh and speculative fetch links from the served markup.
Scripts can still navigate and can insert new links, including speculative hints.
No response header makes top-level navigation a containment boundary.
The popup permission remains enabled so that the Floorplan credit link works.
The credit that the engine adds or keeps can also be hidden by the page.
The README, security document and feature entry now describe these limits.

## 1581. Local map access had removed the browser guard for every display kind

Found on 2026-11-23 in review round 1 of unit s33-codemap.
State: Fixed in review round 2.

Mock and editor previews must still require a signed-in browser.
Only a map opened through the authenticated local socket receives the browser-session exception.
The session records that exception and passes it only for map reads.
The browser revocation test checks that ordinary input is refused after the last browser signs out.
The map integration test still opens and reads through the local socket without a browser sign-in.

## 1582. A parent-folder swap could redirect a map read outside the checkout

Found on 2026-11-23 in review round 1 of unit s33-codemap.
State: Fixed in review round 2.

The no-follow flag applies only to the final path component.
A checked parent folder could be replaced by a symlink before the file was opened.
The map reader now resolves the actual Linux descriptor path before and after using the file.
The descriptor must remain inside the canonical checkout.
A host without descriptor-path verification refuses maps.
The graph reader uses the same verification.
A deterministic test swaps the parent immediately before open and confirms refusal before any content is read.

## 1583. Reloaded maps repeated expensive preparation and consumed preview slots

Found on 2026-11-23 in review round 1 of unit s33-codemap.
State: Fixed in review round 2.

Each request previously read, redacted and parsed a page of up to 32 mebibytes.
Map frames also consumed the same eight slots as mock and editor previews.
The page limit is now 16 mebibytes.
Four map slots are independent of the eight preview slots.
A token retains its prepared response while file identity, size and modification time are unchanged.
Registration and descriptor safety are checked on every request.
The redactor's opaque registry identity invalidates the secret check when live secrets or token hashes change.
A custom redactor without that identity still checks every response.
Tests check reuse, file edits, secret changes, unsafe replacement, independent quotas, revocation and expiry.
A reload loop can still issue cheap metadata checks and repeated response transfers.

## 1584. Map responses inherited permissive resource headers intended for previews

Found on 2026-11-23 in review round 1 of unit s33-codemap.
State: Fixed in review round 2.

Map responses inherited wildcard cross-origin resource sharing and a cross-origin resource policy from the preview headers.
The map response removes both permissive headers.
Its integration test checks their absence.

## 1585. Map tests and documents overstated guarantees and omitted a storage decision

Found on 2026-11-23 in review round 1 of unit s33-codemap.
State: Fixed in review round 2.

The agent refusal test used payloads with unrelated fields.
It now checks each command's valid payload against its schema before checking the identity refusal.
The response policy assertion now includes the exact script and style directives.
The documents now say that a page can hide the engine's added credit.
The feature entry again records the undecided storage location for maps across project worktrees and a future server.
It also states that opening a map requires the separate display hostname, which the default loopback-only setup lacks.

## 1586. The map status reads "no map" on a host without `/proc`

Found on 2026-11-23 by the second review of unit s33-codemap.
The descriptor check resolves `/proc/self/fd/<n>`, and on a host without `/proc` that fails with "not found" (`packages/engine/src/project/map.ts`).
`project.map` then reports that no map exists, where it should refuse.
Serving still refuses, so nothing unsafe is shown.
State: Open. Minor.

## 1587. Reading a map or its graph always allocates the full size limit

Found on 2026-11-23 by the second review of unit s33-codemap.
`packages/engine/src/project/mapFile.ts` allocates 16 mebibytes for every read, whatever the file's size.
The buffer should be sized from the file.
State: Open. Minor.

## 1588. Open map frames keep their pages in memory

Found on 2026-11-23 by the second review of unit s33-codemap.
Each open map frame holds the page's text and its prepared response for up to ten minutes.
Four frames of the largest allowed page hold about 130 mebibytes.
The amount is bounded and nothing is logged.
State: Open. Minor.

## 1589. The first request for a map repeats the secret check

Found on 2026-11-23 by the second review of unit s33-codemap.
The address token is registered as a secret after the page was prepared, so the prepared page is checked again on the first request.
This is safe and only wasteful.
State: Open. Minor.

## 1590. The map tests leave two gaps

Found on 2026-11-23 by the second review of unit s33-codemap.
The test against a real Floorplan page is skipped unless `FLOORPLAN_REFERENCE_CHECKOUT` is set.
No test proves that a browser caller cannot open a map frame without a signed-in session; only the guard itself is tested.
State: Open. Minor.

## 1591. The word "workflow" named two things

Found: 2026-11-23, in the design of the port.
State: fixed in 40a9e62a, by the owner's choice of the name "Actions".

The owner decided on 2026-11-23 that builds and tests run from workflow files of GitHub Actions.
The product already uses the word "workflow" for the build loop and its replacements.
`AGENTS.md` rule 6 calls the build loop "the default workflow", and the sidebar's "Workflows" page lists those definitions.

The owner decided to call them actions instead of workflows.
In the port, a project's view for these runs is named Actions, and its button reads "Run action".
"Workflow" means the build loop again, and the build loop's settings page keeps that name.

Two things remain.
A file's path still says `.github/workflows/`, because the files are used as they are.
GitHub itself calls a reusable step an "action", so a person who knows GitHub well may read the word differently.

Two more places follow from the decision and are unchanged.
The build loop's settings hold a "Full-run command" and a "Check command", and an action would replace both.
The sample runs are named "Full run", "Check", "Nightly" and "Build", and real files would bring their own names.

## 1592. The row of views was longer than a window of 1000 pixels

Found: 2026-11-23, in the design of the port.
State: fixed in 4dc13155.

The row held eleven names after Actions became a view, and at 1000 pixels it ended inside the seventh.
The owner said which views are opened often, and three left the row.
End result shows only while a project is in Define, and the board has its place after that.
The message board is a tab of Status.
The terminal is a panel under the view.
The row now holds eight names, and all eight fit a window of 1000 pixels.

## 1593. Facts of the old Agents view have no place in the port

Found: 2026-11-23, when the Agents tab became the message board.
State: open.

The Agents view listed every agent of a project in lanes or in a table.
The Status view lists the agents now, and an agent has its own page beside its conversation.
These facts and controls were only in the Agents view, and the port's mock shows them nowhere now:

1. The tokens the project has used so far, and its count of turns.
2. The subscriptions in use, each with its share used.
3. The count "May be stuck", for agents with no output for ten minutes, and the warning on such an agent's row.
4. The tokens of one agent. Its page shows context and turns, not tokens.
5. The buttons on an agent's row to message it and to stop it. The conversation of an agent still has "Stop agent" where the owner may write to it.
6. The filter by team, and the choice between lanes and a table.

The live mode of the port still has the Agents tab, because its message board is a separate tab there.

One more point is about the agent's page.
In a window under 1100 pixels only one half shows, and the page half does not name the agent. The top bar's path names it, and the path is cut at that width.

A second point is about the agent's page.
With the conversation open at 1440 pixels, the task table is wider than the page. Its Points column is out of sight, and the last owner's name is cut.
The sprint page stacks its task rows at that width, and this page does not.

A third point is about what a worker changed.
A commit on the agent's page opens the Git view, not that commit.
An agent that writes in its team's worktree shows only the changed files that its task names, so a file it changed outside its task is not shown.

## 1594. Open points after the row of views was shortened

Found: 2026-11-23, in the design of the port.
State: open. Item 1 is fixed in 8de408af.

1. The statements of the End result are not on the board. The owner expected them there. Answered on 2026-11-23: a sprint's page lists the statements it delivers, and the End result is a tab of Status after Define. Both are drawn. The owner will have the End result tab redesigned later. In the sample data only four sprints name their statements.
2. The terminal and a change of project. The owner said this needs a discussion. In the port the panel stays open and shows the sessions of the project that is now open.
3. The terminal outside a project. The sidebar's item switches the panel on, and nothing shows until a project is open.
4. The layout of Status. The owner liked the earlier version too: the phases as a vertical line on the left and the facts on the right. The port keeps the strip across the top. Both are not drawn side by side.
5. Mocks and Design. The owner said both are needed and that the mocks will be redone. Whether they become one view is not decided.
6. Files. The owner opens it rarely and still wants it. It stays in the row.
7. A window under 1100 pixels has no button for the conversation with a name. It is an icon, as before.

## 1595. The addresses of the port were checked, and some are still wrong

Found: 2026-11-23, when the owner reported that the back link of a task opened the sprint's Requests tab, and asked for a check of the addresses.
State: open. Seven faults are fixed in 89dbc84e and 3ab7f9da, and seven are open.
Where: `design/mock-vite/src/app.tsx` and `design/mock-vite/src/screens/project.tsx`. The live website has its own addresses and was not checked.
Fifty addresses were opened in a browser, and each one's marked view, marked tab, title and back link were read.

These faults are fixed in the port.

1. The back link of a task opened the sprint's Requests tab. A task now opens inside the Tasks tab of its sprint, so there is no back link.
2. Every page had the same title in the browser's tab and history, "LGTM Studio". A project's page now names itself.
3. Five pages had two addresses: Actions, Status, the End result after Define, the board with the terminal, and a task. Each old address now moves to the one address of its page.
4. A wrong address inside a project showed the board, and a wrong address outside a project showed Home. Both now say "This page does not exist".
5. The board's address in a project that is in Define showed an empty board with no tab marked. It now moves to the End result.
6. The back arrow of an agent's conversation always opened the End result. It now returns to the page the owner came from.
7. The words in the addresses were not the words on the screen. The owner agreed to a rename on 2026-11-23. Status is `status`, the End result is `end-result`, the Message board is `message-board`, and a conversation is `conversation`. The four older words move to the new ones. A check of 648 links on nineteen pages found no old word.

These faults are open.

1. The live website still uses the older words. Its addresses are the builders' work.
2. The End result has one address, and it means two places. In Define it is the first view of the row, and after Define it is a tab of Status.
3. A wrong tab name on a sprint's address opens the first tab, and the address stays wrong.
4. A conversation's address with an agent that does not exist shows the board with the lead's conversation, and says nothing.
5. The team page marks no view in the row.
6. Pages outside a project still have the title "LGTM Studio".
7. A list's filter, a Status phase that is open, and the terminal panel are not in the address, so a link cannot open them.

## 1596. The restart sweep leaves a launcher-started adapter running

Found on 2026-11-23 in unit f3-security, while finding the cause of issue 241.
The runner records an adapter's executable as soon as the spawn returns.
The Claude Code and Codex adapters are `/bin/sh` launchers that later `exec` Node.js, so the recorded executable is the shell.
`sweepOrphans` in `packages/engine/src/recovery/orphans.ts` leaves a group alone when its executable differs from the record.
After a service crash, a restarted service therefore leaves those adapters, and their agents' shells, running.
The boot, the group leader's pid and its start time already identify the process, and a start time survives `exec`.
Unit f3-security did not change the sweep, because it signals processes and is outside that unit.
State: Open.

## 1597. An agent's process can still act as the owner

Found on 2026-11-23 in unit f3-security, while investigating issue 241.
The command-line client acts as the owner whenever `LGTM_SOCKET` and `LGTM_TOKEN` are both unset.
It then reads `owner-credential` from the state folder, which it finds through `LGTM_HOME`, `XDG_STATE_HOME` or `HOME`.
Agents run as the owner's user and inherit `HOME`, so they can read that file.
The engine refuses the owner credential only when the caller's claimed ancestry names a live adapter, or its process group is an adapter's group.
An agent's shell can leave both, for example with `setsid -f`, `tmux new-session -d` or `systemd-run --user`, and the client then sends a chain that names no adapter.
A process can also write its own client that sends any ancestry headers it likes, because the engine trusts the claim.
`SECURITY.md` already says that agents can read the owner's credential until agents run in a sandbox.
A process left in an adapter's process group after the group's leader has exited also passes as the owner.
`outsideAdapters` in `packages/engine/src/api/ancestry.ts` skips a row whose leader cannot be read, so a surviving member of that group is no longer recognised.
Closing this needs a design change, such as the planned sandbox, a separate user account for agents, or the engine reading the caller's process from the socket's peer credentials.
State: Open. Needs an owner decision.

## 1598. A state-folder deny rule would not stop an agent's shell

Found on 2026-11-23 in unit f3-security, while investigating issue 241.
This entry first said that `SECURITY.md` promises a state-folder deny rule that no harness gets.
That was a misreading, corrected in review round 1 of the unit.
The section "Harnesses with no deny-read or deny-write rule" in `SECURITY.md` already says that no harness gets either rule today, and issues 91 and 104 track the missing rules.
What remains is one point that those entries do not make.
A deny rule given to a harness limits only that harness's own file tools.
A shell command that the agent runs, such as the command-line client reading `owner-credential`, is not covered by it.
So adding the rule for Claude Code would not have stopped the access in issue 241.
State: Open. It needs no change of its own; it is a note for whoever builds issues 91 and 104.

## 1599. A conflicted merge is refused when the other branch adds a file that matches a copy pattern

Found on 2026-11-23 by the second review of unit f3-security.
A clean merge commits a file that the other branch added, even when its name matches a copy pattern such as `.env.*`.
The same merge with a conflict elsewhere is refused when the owner finishes it, because the file shows as newly added in the index.
The refusal protects nothing there, because the file is already committed on the other branch.
Its sentence says the file "is not tracked", which is false in that case.
The owner can abort the merge or narrow the pattern.
A fix treats a path whose index entry equals the entry of `MERGE_HEAD` as ordinary content.

State: open.

## 1600. The copied-file refusal names no file

Found on 2026-11-23 by the second review of unit f3-security.
A whole commit that is refused with the reason `copied_file` does not say which staged file matched a copy pattern.
The sentence also does not offer unstaging the file as a way out.
The Git screen does not show the names that "stage all" skipped either, because that is a screen change.

State: open.

## 1601. The copied-file check lacks three tests and one bound

Found on 2026-11-23 by the second review of unit f3-security.
No test covers a merge that brings in a matching file, a rename onto a matching name, or a repository with no first commit.
The reviewer checked those three cases by hand and they behave correctly.
The `skipped` list in the reply of "stage all" has no length bound in the contract, unlike the neighbouring lists.

State: open.

## 1607. Three minor findings of the second review of the lifecycle unit

Found on 2026-11-23 by the second review of unit f2-lifecycle.
The fake runner in `packages/engine/test/support/fakeRunner.ts` accepts a send while a turn is unended, and the real runner refuses it with "agent already has a turn".
One test of a late send failure therefore ends in a state that the real runner cannot reach, although the check it guards is correct.
A wait notice that a reloaded session had already shown gets no event and stays withdrawn.
After a second crash followed by a fresh resume, that notice is delivered again, arbitrarily late.
It is repeated and never lost; writing an accepted event for a shown notice would close it.
The archive path in `packages/engine/src/project/lifecycle.ts` settles its pre-emption after it releases the lane, which is the window the unit closed for a stop.
That is harmless today, because the member is stopped by then.

State: open.

## 1608. The same nine browser journeys fail in every whole run and pass alone

Found on 2026-11-23 while checking units s33-codemap and f3-security.
In the whole `pnpm check` run of both units, the same nine journeys failed: `git.spec.ts` at lines 183, 222 and 253, `new-project.spec.ts` at lines 149 and 324, `studio-lead-selection.spec.ts` at line 21, and `studio.spec.ts` at lines 372, 1212 and 1220.
Neither unit touches what those journeys cover.
All nine pass when each is run alone, at a load average between 9 and 18.
The cause is therefore in the journeys or their shared test service under a whole run, not in the changes.
The whole run cannot be trusted as a gate until this is found.

State: open.

## 1613. Most of the website's records are not in the contract yet

Found: 2026-11-23, when the owner split the product into a website, a server and a client.
State: open.
Where: `src/frontend/`, `src/contracts/` and `src/mock-server/`.

What happens: the website reads only the session and the machines from a server. Every other record is sample data that the page makes itself and changes in the browser's memory.
What should happen: the website holds no sample data. Every record comes from the server through the contract, and every change is a call.
Why it matters: the owner cannot write the server in Go against the contract until the contract covers what the website does.

The records still inside the page, in the order they should move:

1. Harnesses and their accounts. They belong to a machine, so they follow the machines.
2. Repositories and projects, with creating, renaming and archiving a project.
3. Teams and agents, with stopping and resuming an agent.
4. Questions and approvals, with their answers. Home and the Questions view read them.
5. An agent's conversation, with sending a message. This is the largest stream.
6. The board: epics, sprints and tasks.
7. The End result and Define.
8. Status and review rounds, Actions, and the message board.
9. Git, Files and the terminal.
10. Mocks and Design, notifications, usage, and the settings of the service and of a project.

The port has 395 places that ask `live.on`. They belong to the older live website, for the engine in `packages/`. Each one leaves when its records move.

Open points for the owner:

1. An address with no scheme is `https`, unless it is this computer or port 80. A server on the local network over `http` must be typed with `http://`. The website connects to it, and then blocks the sign-in, because the owner's rule is "HTTPS or loopback". The rule covers both the page's address and the server's address. The owner had HTTPS turned on for the development server on 2026-11-23, with a certificate it signs itself. A browser on another computer signs in at `https://<address>:4179/` with the server address `/`. From that HTTPS page, the address `:4180` fails, because the mock server speaks `http` only.
2. The session is a cookie with `SameSite=Strict`. A website on another site than its server cannot send it. A website and a server on one host with two ports can. A website whose host passes the calls on to the server can too, and the address `/` does that since 2026-11-23. A hosted website for many servers needs another answer.
3. `ReconnectClient` asks the server to call an offline client. A client that dials the server cannot be called. The call may have to go.
4. The Machines page shows the roles "Master" and "Worker". The contract has no roles: one client runs on the server's machine, and the page shows that one as "Master".
5. A client's name may repeat the name of a client that waits for approval. The mock server allows it.

Open points in the code:

1. The website still reads the fonts, CodeMirror, the plans and the default workflow from `design/mock/`.
2. The user interface parts are not a package of their own. `src/frontend/src/ui.tsx` imports the store and the actions, so it cannot be used alone yet.
3. Removing a machine also removes its subscriptions and stops its agents. The page does that itself, because those records are not on the server yet.
4. The mock's menu of preview states sets the session in the page only. The server does not know.
5. No test covers the address rules, the mock server or the contract. The owner asked for no tests during design.
6. The older documents and the hand-over files in `design/builds/` still say `design/mock-vite/`.

## 1614. Open points of the Map view

Found: 2026-11-23, when the Map view was drawn in the website at the owner's request.
State: open.
Where: `src/frontend/src/screens/map.tsx`, `src/contracts/proto/lgtm/v1/map.proto` and `src/mock-server/`.

What happens: the Map view works against the mock server only.
What should happen: the view reads the map of the project's own checkout from the real server.
Why it matters: the owner sees a sample map, not the map of the project that is open.

Open points:

1. The sample map is the map of a private repository. The mock server reads it from that repository's folder by a path in `LGTM_MOCK_MAP`. It is not in this repository, because it describes another product's code. With no path, the view shows a placeholder page. This repository has no map of its own yet.
2. The live website hides the Map view. The engine already has `project.map`, `project.map.open` and `project.map.release`. Joining them to this view is builders' work.
3. The real server must serve the page from a separate hostname, as `SECURITY.md` says. The mock server serves it from its own address with a sandbox only.
4. Floorplan's page has its own "Ask" bar. It cannot work inside the frame, and it says "Asking is unavailable". The conversation with the project lead is beside the map. Hiding the bar would mean changing the page.
5. "Build map" and "Update map" leave no line in the project lead's conversation. In the real product an agent does the work, and the owner should see it there.
6. The line says when the map was built. It cannot say whether the map still matches the code.
7. The row of views now has nine entries on a project with mocks. Issue 1592 is about that row's length.
8. The website has no map icon. The empty state uses the network icon. The icon list is generated from `third_party/lucide/`.
9. The view was checked at 1440 and 1000 pixels in dark and light, in Chromium only. No blind review and no space checks were run.

## 1615. What the Actions view does not have yet, measured against GitHub

Found: 2026-11-23, when the Actions view got a rail of its actions, with GitHub's Actions tab as the yardstick.
State: open.
Where: `src/frontend/src/screens/workflow.tsx`, the part named `Runs`, and `src/frontend/src/actions-data.ts`.

What happens: the view lists the runs and the actions, and it opens an action's file. It leaves out several things GitHub has.
What should happen: the owner decides which of them this product needs.
Why it matters: the owner named GitHub as the yardstick.

The yardstick was GitHub's layout as I know it. Mobbin has no picture of GitHub's Actions tab, and no page of GitHub was read for this change.

Not drawn:

1. Filters by status, branch, event and who started the run. The view filters by action only.
2. Pages of runs. The list shows every run. A project with hundreds of runs needs pages or a "Load more".
3. Stopping a run, and running only the failed jobs again.
4. Turning an action off.
5. A summary of a run: the drawing of its jobs, its notes and the files it left.
6. GitHub's management pages: caches, runners and usage. Machines and Usage in the sidebar cover part of this.

Open points:

1. With the conversation open on a screen 1440 pixels wide, the list of runs is 208 pixels wide. A run's title takes two lines there, and a longer title is cut after the second line.
2. The sample has the same four actions for every repository.
3. The actions and their runs are still sample data inside the page. They are item 8 of issue 1613.
4. No blind review and no space checks were run on this view.

## 1616. Several views forget their choice when Back returns to them

Found: 2026-11-23, when a view's own header got a Back button.
State: open.
Where: `design/flows.md`; `src/frontend/src/lib.tsx`, the part named "where the owner came from"; `Back` in `src/frontend/src/ui.tsx`.

What happens: Back shows on every screen that `design/flows.md` marks for it, after the owner followed a link to get there. It returns to the address they came from. A view that keeps its choice in the page, not in the address, opens at its start.
What should happen: a view opens as the owner left it.
Why it matters: the owner asked that Back takes them to where they came from.

Views that open as they were left: Files (the open file), a sprint (its tab and its task), one question, Actions (the chosen action and the open run).

Views that forget, found by reading the code and not by trying each one:

1. Status: the chosen phase, and on Review rounds the chosen round and its filter.
2. The message board: the search text and its filters.
3. The board: the search text and the chosen epic.
4. Mocks: the open item and the filter of the data.
5. Git: the open file and the open commit. Back from Git to a file works. Back to Git opens its first file.
6. No view returns to where it was scrolled.

Open points of the button itself:

1. A task opened from its own sprint shows the arrow to the board, not Back. The task has a Close button that shows every task again.
2. On a screen 1000 pixels wide, Back in the Git bar leaves less room for the name of the worktree's owner, which is cut.
3. The Git view with no worktrees has no Back. The end result of a project in Define has none either.
4. Back on Review rounds and on one screen of the mocks is built and cannot show today, because no link from another view opens them.
5. A phone has no Back in the file header or above a question. That place holds the arrow to the list.
6. On a wide screen a file chosen in the tree takes the place of the open file in the browser's history. The browser's own Back no longer steps through the files.
7. A move counts as a link when the address changes within one second of a click outside the standing lists of places. A move that a screen makes later than that shows no Back.
8. A move from the search of projects and agents by the keyboard shows no Back.
9. The older layout in the same code, used when the live engine is on, was not checked.
10. All twenty-one flows in `design/flows.md` were walked by hand-written scripts, in Chromium only. No test in the repository covers any of them, and no blind review was run.

## 1617. What the project's Settings does not have yet, after its rework

Found: 2026-11-23, in the owner's review of the views of a project.
State: open. The rework is built in the website and the mock server. The points below remain.
Where: `ProjectSettings` in `src/frontend/src/screens/service.tsx`; `Settings` in `src/frontend/src/screens/workflow.tsx`; `PreviewSettings` in `src/frontend/src/screens/browser.tsx`; `design/builds/project-settings.md`.

What happened: the owner wanted the project settings page reworked, and asked for a critique and an overhaul. An independent reviewer scored the old page 19 of 40. Settings now has four short pages, and `design/builds/project-settings.md` records what changed and why.

What remains:

- An assignment that waits for the owner's approval shows only as a dot on the Settings tab and on its Agents page. The reviewer's advice was to raise it as a question in the Questions view. That is not built.
- "Delete team" is still in Settings, as an icon. The reviewer's advice was to move it to the team's own page.
- The Workflow page names the workflow, "Build loop", as a link to the Workflows page. A project cannot choose another workflow here.
- A project has no settings for its own notifications or for a limit on its usage. The reviewer looked for both.
- The previews come through the contract. Every other setting on these pages is still sample data in the page, as issue 1613 records.
- A preview's command runs on the owner's machine, and the Previews page lets the website set it. Issue 1618 records the rule the real server needs.
- A preview has no worktree of its own. It runs in the project's worktree.
- The older layout, used when the live engine is on, shows General and Agents as one page. It was not opened in a browser after this change.
- At 1000 by 680 three of the four pages scroll, by 40 to 80 pixels.
- The pages were not tried with worst-case data, and the space checks were not run.

Fixed in the same change: the sample data named the real owner's account in the address of the sample repository, in `src/frontend/src/data.ts` and `src/frontend/src/screens/design.tsx`. It names the sample owner now.

Reviewed on 2026-11-23, after the build: one blind reviewer, the space checks and the scripted access checks. `design/builds/project-settings.md` lists each finding and its answer. The reviewer's blocker was real: the port field of a preview was 22 pixels wide and showed half a digit. It is fixed. Left open from that review:

- A row's help covers the row under it while it is open.
- The select beside "Use for Plan, Build, Review and Test" has no visible name.
- "Need-to-know", "Remove agent credits" and "Stuck at" are the product's own words, explained only in the row's help.
- A control that is off, such as "Delete project", gives its reason only as hover text.
- The Previews page is mostly empty, and the General page is a third empty, at 1440 by 900.

## 1618. The Browser page shows a product only when the product works without an origin of its own

Found: 2026-11-23, when the Browser page of the Design view was built.
State: open. The page works on the mock server's sample page.
Where: `src/contracts/proto/lgtm/v1/preview.proto`; `src/mock-server/src/engine.ts` and `serve.ts`; `src/frontend/src/screens/browser.tsx`.

What happens: the product's pages are code that agents wrote, so the website shows them in a frame with `sandbox="allow-scripts allow-forms allow-popups"`. The frame has no origin. That keeps the product's code away from the owner's session. It also means a product cannot use cookies, `localStorage` or a service worker there, and many real products need one of them to start.

What a real server must do before this limit can go:

- Serve each preview from a hostname of its own, apart from the website's, as `SECURITY.md` requires for a codebase map. Only then may the frame have `allow-same-origin`.
- Pass on every path of the product, with its scripts, its styles and its pictures. The mock server answers one address with one page.
- Pass on the WebSocket that a development server uses to reload the page.
- End every open address of a preview when the preview stops. The mock server does this.

Other open points of the page:

- A preview's command runs on the owner's machine. Starting one from the website is the same power as running a command, so the real server needs a rule for which commands a preview may have.
- The sizes are Phone, Tablet and Full. A size cannot be turned on its side, and no size can be typed.
- The page was not tried with a real product, with a game, or with an Expo web build.
- Reload opens a new address and loads the product's first page. It does not reload the page the owner had moved to inside the product.
- The row of pages of Design has seven entries. At 1000 pixels they fit with 0 pixels to spare in the sample, so a longer count or one more page makes the row scroll.
- A link from Status into Design (flow F-25 in `design/flows.md`) was not walked in a browser.

Added on 2026-11-23: the Browser page has a "Pick an element" button. It turns on and off, and it picks nothing. The picker needs a browser extension, as `docs/research/2026-11-23-element-picker-and-browser.md` records. The owner also asked for the Browser page to load a plain address, with no command. That is not built, and the Previews page of Settings still holds a command.

## 1619. The shell fails five access checks on every page

Found: 2026-11-23, by the scripted access checks of the design plugin, run on seven pages of the website.
State: open. Three findings of the same run are fixed, and they are listed at the end.
Where: `src/frontend/src/app.tsx`, `src/frontend/src/ui.tsx`, `src/frontend/src/css/base.css`, `src/frontend/src/screens/files.tsx`.

What happened: the check runs axe-core, walks the page with the Tab key, and applies the text spacing of the Web Content Accessibility Guidelines (WCAG). These failures are in the parts around every view, so every page reports them.

- **The row of views is a list of tabs that holds other things (WCAG 1.3.1).** `.tabrow` has the role `tablist`, and it also holds the "Project lead" button. The views are links, so the row is navigation: a `nav` with `aria-current` on the open view.
- **Faint text is below the contrast bar (WCAG 1.4.3).** The faint colour on the raised surface measures 4.19 to 1 in the dark theme, and the bar is 4.5 to 1. It shows in the head of the chat column and in the context meter, at 11 and 12 pixels.
- **Three menus of the message box have a name that leaves out their words (WCAG 2.5.3).** The buttons are named "Subscription", "Model" and "Effort", and they show "Claude Code · Max", "Opus 5.5" and "High".
- **The file editor holds the Tab key (WCAG 2.1.2).** Tab indents inside the editor, so the keyboard cannot leave it with Tab. The editor's focus ring also changes too few pixels to be seen (WCAG 2.4.7).
- **Seven labels are cut off under the WCAG text spacing (WCAG 1.4.12).** They are in the sidebar and in the row of views.
- **The folder of a worktree is cut off in the side of Files.** The space check reports it. The end of the path shows, and the whole path is hover text only.

Not tested by the script, and not tested by hand: a screen reader, status messages (WCAG 4.1.3), content on hover (WCAG 1.4.13), forced colours, and the keyboard beyond the Tab walk.

Fixed in the same change: a sidebar link said "pressed" where a link is "current"; the "Add a task or a file" button had no name; the pickers of Browser and Files had a name that left out their words; the help mark in a table's header stayed invisible when the keyboard reached it; the "Skip to content" link slid in over 120 milliseconds and now shows at once.

## 1620. What the blind review of the Design view left open

Found: 2026-11-23, by a blind reviewer who read twenty pictures of the Design view, Files and Git.
State: open. Four findings are fixed, and they are listed at the end.
Where: `src/frontend/src/screens/mocks.tsx`, `src/frontend/src/screens/browser.tsx`, `src/frontend/src/screens/map.tsx`, `src/frontend/src/screens/design.tsx`.

What remains:

- **"Open in a new tab" does not show on sample data.** The Browser page and the Architecture page show it only against a server, because a sample page has no address that a new tab can open.
- **One kind of picker has three looks.** Git has a bordered select, Browser has bold text with no border, and Files has code text with its arrow at the far right.
- **"Approved" is a green pill on every row of Screens,** and once more in the line above the table. It is the usual state.
- **"Request changes" does not say on what,** on a Screens page where every screen is approved.
- **The Design file opens at 20 percent in a narrow pane.** The frames are too small to read until the owner zooms.
- **The name of the design file is cut off** in the bar, in a pane of 680 pixels.
- **The line beside Generate is empty while Commit is off.** The text "Write a message first" was removed at the owner's request on 2026-11-23, so this stays.
- **The icon button of the Architecture page has no word.** Its hover text says "Focus on the diagram".
- The reviewer named colour pairs to measure. Only the faint text was measured, in issue 1619.

Fixed in the same change: the Design file opened with its first column of frames cut off, because the framed sign-in screen took the focus and the browser scrolled to it; the row of pages cut "Architecture" when Back stood in it, and Back now gives up its word in a tight row; at 320 pixels the open page was out of sight in the row, and the row now scrolls to it; the worktree menu said "Assigned" where the other rows name a machine.

## 1621. What the command centers and the repository's view leave open

Found on 2026-11-23, while the command centers and the repository's view were drawn in `src/frontend/`.
The owner chose the grid for both on 2026-11-24, and the other layouts were removed.
Three points were closed in the same change: the scripted access checks were run, the worst-case data was tried, and a blind review was done.
What is left is not fixed.

1. **Some numbers are made up.** The tasks finished on each day, the tokens on each day, the sprints merged in each week and the full runs in each week come from the id of the project or the repository. They are not records. They are not in the contract, and neither are the plan of a repository and the repository agent. They join the list in issue 1613.
2. **The command center and Status overlap, and the owner keeps both.** "Needs you", "Working now", the phases and the checks are on Status as well. The owner decided on 2026-11-24 that Status keeps its Overview page. Nothing is left to do here.
3. **A chart's numbers were not heard with a screen reader.** Fixed in part on 2026-11-24: "Show the numbers" under a bar chart puts a table in its place, which a keyboard reaches. The small line of Usage has no table.
4. **A widget cannot be dragged, and the arrangement is not on the server.** Fixed in part on 2026-11-24: "Arrange the widgets" moves a widget, sets its width and hides it, and the browser remembers. Another browser shows the usual arrangement. There is one dashboard for all projects, and one for all repositories.
5. **The Git view of a repository says "Agents never edit it directly".** A repository agent that works in the main checkout does not fit that sentence. The owner has not said whether that agent may commit there.
6. **A repository had no architecture diagram on sample data.** Fixed on 2026-11-24: the sample starts with a diagram of the main checkout of `lgtm-studio`.
7. **The live website keeps the older repository page.** The engine knows no repository agent.
8. **The first address of a project changed.** A project past Define opened on `board`, and it now opens on `center`. The lead's own address shows the command center as well.
9. **The sidebar cuts a very long project name.** Kept on 2026-11-24: the row now says its whole name on hover, and the space check is told that the cut is meant.
10. **The shell around the command center still fails the access checks of issue 1619.** The row of views, the composer and the sidebar have the same findings as on every other page.
11. **The blind review was not repeated after its fixes.** `design/builds/command-center.md` lists its fifteen findings and what was done. The reviewer did not see the worst-case pictures, the empty states, the open menu or a width of 320 pixels.
12. **The contrast of the phase segments and of a meter's empty track was not measured.** The reviewer estimated the track at about 1.2 to 1 against its tile. The track is the website's own `.progress`, so a change reaches every meter.

## 1631. The workspace manifest test does not allow the buf entry

Found on 2026-11-24, while the full Vitest run was checked for the private-details sweep.
`packages/engine/test/smoke/workspace-manifest.test.ts` expects `allowBuilds` in `pnpm-workspace.yaml` to list only `@tsc-rs/typescript@0.8.0`.
The manifest also lists `@bufbuild/buf@1.75.0` with the value `false`, so the test fails on `master`.
Either the test learns the buf entry, or the entry moves elsewhere.
It is not fixed.
12. **The empty tracks are under 3 to 1.** Measured on 2026-11-24 in the dark theme, against a tile: an empty phase segment is 1.76 to 1, the empty track of a meter is 1.17 to 1, the filled part is 9.64 to 1, and a bar is 7.57 to 1. Every meter and the phases also say their value in words, so nothing was changed. The owner may want stronger tracks.

## 1622. What the lead menu, the arranged widgets and the workflow versions leave open

Found on 2026-11-24, while the three were drawn in `src/frontend/` on sample data. Nothing here is fixed.

1. **Workflow versions are not in the contract.** The Workflows page still asks the engine with its own request names, and the sample keeps the versions in the browser. The requests for the list of versions and for one version do not exist in the engine, so the live website shows no versions. This joins issue 1613.
2. **The owner has not settled three points of the versions.** The number kept is 20, as proposed. Whether the engine adds to an existing `.lgtm/.gitignore` is open. What else that file ignores is open.
3. **The menu of versions shows no difference.** A version must be put in the editor to be read. It does not say what changed.
4. **The lead menu lists only leads.** A builder, a reviewer or a test runner opens from the sidebar or from Status, and stays read only.
5. **The chosen lead is not in the address.** A reload goes back to the project lead.
6. **The arrow of the lead menu scrolls away in a narrow window.** The conversation's button stays at the left end of the row, and the arrow beside it does not.
7. **A repository has no lead menu.** Its conversation is with the repository agent only.
8. **A tile's content is dimmed while the owner arranges.** Its links are not pressed by a pointer, and a keyboard still reaches them.
9. **None of the three had a blind review, worst-case data or the scripted access checks.** The eight space checks pass on the command center while arranging, at 1440 by 900 and at 1000 by 680.
10. **A read-only review of the older open issues is in `docs/research/2026-11-24-website-leftovers-review.md`.** It says which points of issues 1572, 1593 to 1595 and 1613 to 1620 still hold, and which issue texts are out of date. The issue texts were not updated.

## 1623. What adding a repository and the terminal's place leave open, and what the same change fixed

Found on 2026-11-24, while both were drawn in `src/frontend/` on sample data, by the owner's word of that day.

### Open

1. **Nothing of "Add a repository" is in the contract.** The dialog, the tracked state and "Set up" change the sample in the browser. The live website keeps "New project" on the plus. The contract needs a tracked repository with no folder, a request to set one up on a client, and the progress of the clone.
2. **How the server reaches GitHub is not decided.** The owner is not sure. Claude Code in the cloud offers two ways: a GitHub App that is installed on each account or organization, and the token of the owner's own `gh` command, sent from a terminal. The sources are https://code.claude.com/docs/en/web-quickstart and https://code.claude.com/docs/en/claude-code-on-the-web. Either way the server would hold a credential for GitHub. `SECURITY.md` has no rule for that yet, so it needs one before this is built. The owner said on 2026-11-24 that a GitHub App is preferred and that OpenBao holds the secrets. The owner decided on the same day that pushes and pull requests appear as the app's bot by default, and that the owner can choose "Me" for the service or for one project. "Me" relies on a GitHub App's user access token, which acts for a person without a `gh` login on each client. That needs a check against GitHub's documentation before it is built. Whether a commit made on a client gets GitHub's "Verified" mark is a separate question, and it is open.
3. **A link alone does not prove access.** An SSH link works only when the chosen client machine has a key for it. The page does not check that before "Set up".
4. **The sample connects GitHub with one press.** No sign-in is drawn, and neither is the making of the GitHub App. Service settings has a "GitHub" section since 2026-11-24, with the account, the bot's name and "Disconnect". The live website does not show the section.
5. **Only GitHub is drawn.** The owner named other hosts as a later menu.
6. **The owner asked for variants from Mobbin, and one was built.** It follows v0's "Import from GitHub": the link above, the account's list under it. Two others were looked at and not drawn: Render's page with the hosts in a side column, and a three-step wizard as Greptile and Amazon Web Services Amplify have.
7. **The plus of the folded sidebar still adds a project.** Only the plus beside the "Repositories" label changed.
8. **The contract has no word for a machine that only runs the server.** The list of places for a terminal shows every machine. `hosts_server` says that a machine runs the server, and not that it runs nothing else.
9. **A repository on two machines offers one place.** The terminal opens in the first folder of the repository.
10. **The terminal of a machine is a sample shell.** It lists the folders of the repositories on that machine and knows little else.
11. **"Pick an element" stays as it is.** The owner said on 2026-11-24 that it is only drawn for now, and that how it works is still to be decided. This is point 11 of issue 1618.
12. **None of this had a blind review, worst-case data, the scripted access checks or a look in the light theme.** The new faint colour was measured by its numbers on the five surfaces of both themes. The space checks were run again on the command centers, the repository and the workflows page, and they found nothing new.
13. **The board's "empty-pane" findings and one "uneven-padding" finding on Workflows are older.** The space checks report them at 1440 by 900. This change did not cause them, and they are not fixed.

### Fixed in the same change

The read-only review in `docs/research/2026-11-24-website-leftovers-review.md` listed these as still open. The texts of the older issues were not edited.

- **Issue 1594, point 3.** "Terminal" outside a project now asks for a place and opens there.
- **Issue 1572, question O5.** The plus beside "Repositories" adds a repository.
- **Issue 1593, point c.** A commit on an agent's page and on the command center opens that commit in Git's History.
- **Issue 1572, finding B5, and issue 1620, point 3.** The Design table says "Approved" once.
- **Issue 1572, finding B1, and issue 1615, point O1.** A run says its state in a word, and its title wraps.
- **Issue 1593, point a, and issue 1594, point 7.** In a narrow window an agent's page names the agent, and the conversation's button says a word.
- **Issue 1616, points 1 to 3.** Status, the board and the message board keep their choice in the address.
- **Issue 1595, faults 3 to 5.** A wrong sprint tab moves to the tab that opens. An unknown agent says "This agent does not exist". A team's page marks Status.
- **Issue 1619, point 2.** Faint text is at least 4.57 to 1 on every surface.

## 1624. What the move of four pages into Settings leaves open

Found on 2026-11-24, while Machines, Harnesses, Notifications and Workflows moved into Settings in `src/frontend/`, as the owner asked that day. Nothing here is fixed.

1. **The Machines table is wider than its place at 1440 by 900.** The list of Settings takes 208 pixels, so the "Mem" column is cut and the table scrolls sideways.
2. **Two entries have nearly the same name.** "Workflows" is the page of workflow files. "Workflow" is the section of the service's settings for the build loop. They sit in one list now.
3. **Settings opens on "Appearance", the fifth entry.** The first four entries are the pages that moved in.
4. **The list has twelve entries.** In a narrow window it is one row that scrolls sideways.
5. **The live website is unchanged.** Its sidebar keeps the five entries in the foot.
6. **"Usage" is alone in the sidebar's foot.** The owner did not say where it goes.
7. **The move had no blind review, no space checks and no look in the light theme.** It was walked at 1440 by 900 and at 1000 by 680 in the dark theme, and no page was wider than the window.

## 1625. What the board of a repository leaves open

Found on 2026-11-24, while the board of a repository was drawn in `src/frontend/`, as the owner asked that day. Nothing here is fixed.

1. **Two cards could carry the same identifier.** This was closed the same day: every identifier now starts with the letters of its repository and its project.
2. **A drag orders cards across projects.** A card dropped between two cards of another project takes a rank among them. A project's own board then shows its cards in that order.
3. **There is no epic on the repository's board.** The card, the Table and the filter name the project in place of the epic.
4. **The Roadmap lists a project that has no sprint as an empty row.** "Phone companion" and "We're trying issue 940" are such rows in the sample data.
5. **The live website is unchanged.** A repository has no page of this kind there.
6. **Nothing new is in the contract.** The board reads the sprints the website already holds, and the contract does not yet say that a server sends the sprints of every project of a repository in one answer.
7. **The board had no blind review, no space checks and no look in the light theme.** It was walked at 1440 by 900 and at 1000 by 680 in the dark theme, with no page error and no page wider than the window.

## 1626. What the drawn screens of a repository leave open

Found on 2026-11-24, while the "Screens" page was added to a repository's Design view in `src/frontend/`, as the owner asked that day. Nothing here is fixed.

1. **Nothing says how a project's screens reach the main checkout.** The sample data holds the two lists apart. A merge does not move a screen from a project's list to the repository's list.
2. **"Changing in" matches a screen by its identifier.** The contract has no field that ties a project's screen to a screen of the main checkout.
3. **A project's list is not checked against the main checkout.** The owner said that a project shows only what it changed. The sample data of each project already holds only such screens, and no code enforces it.
4. **The owner cannot ask for a change from the repository's screen.** There is no comment box there. A change starts in a project.
5. **A repository has no "Components" and no "Data" page.** A project's Design view has both.
6. **The tools above an open screen wrap to two rows at 1000 by 680.** The arrow to the next screen drops under the name. A project's screen has the same row.
7. **The sample drawing of "Service settings" is the phone's notification screen.** The sample data has five drawings, and none is of the service's settings.
8. **The contract and the live website are unchanged.**
9. **The page had no blind review, no space checks and no look in the light theme.** It was walked at 1440 by 900 and at 1000 by 680 in the dark theme, with no page error and no page wider than the window.

## 1627. What the new identifiers of sprints and tasks leave open

Found on 2026-11-24, while the identifiers changed to the form "LS-AC-12" in `src/frontend/`, as the owner decided that day. Nothing here is fixed.

1. **The form is the designer's reading of the owner's words.** The owner named the repository's letters, the project's letters and a number. The hyphens between them, and "AC" for "Agent console" in place of "AA", are choices the owner has not confirmed. The owner answered the same day: a project has a code given in order, "A01" to "A99" and then "B01", so an identifier is "LS-A01-12". The hyphens are still the designer's choice.
2. **The letters are taken from the name.** The owner's example "AA" may mean letters given in order, "AA" then "AB". The website gives letters from the name and the next free pair on a clash. This was closed the same day for a project, which now has a code given in order. A repository's letters are still taken from its name.
3. **Nobody can change a repository's or a project's letters.** The project's settings only show them. A repository's letters are shown nowhere but in an identifier.
4. **A card no longer says that a sprint is a fix sprint.** "FX" said so. The sprint's own page still says "Fix sprint".
5. **A repository's own sprints and tasks are not drawn.** The owner said a repository makes them too. The form "LS-12" is written in the design and nothing shows it.
6. **The contract is unchanged.** It has no field for a repository's letters, and it does not say that the server gives the numbers. The server has to give them, because two clients can add a task at the same moment.
7. **A team's branch takes the identifier in small letters,** such as `feat/agent-console/ls-ac-3`.
8. **The live website keeps its old labels.**
9. **The change had no blind review, no space checks and no look in the light theme.** It was walked at 1440 by 900 and at 1000 by 680 in the dark theme, with no old identifier left on eleven pages and no page error.

## 1628. What the new "Add a repository" dialog and Connections leave open

Found on 2026-11-24, while the dialog was drawn again and "Connections" was added to Settings in `src/frontend/`, as the owner asked that day. Nothing here is fixed.

1. **Connections lists GitHub only.** The owner said the secrets are in OpenBao. Nothing shows whether the server reaches OpenBao, or at which address. The notification server and the subscriptions of the harnesses are on other pages.
2. **"Connect GitHub" connects at once.** The real step leaves the website for GitHub, where the owner installs the GitHub App and chooses its repositories. That step is not drawn.
3. **Nobody can change which repositories an account shares.** The table says "All repositories" or a count, and GitHub's own page is where it changes. There is no link to that page.
4. **Adding a repository closes the dialog.** The owner who wants three repositories opens the dialog three times.
5. **The search looks in one account at a time.** A word does not find a repository of the other account.
6. **A link to another host than GitHub is taken as it is.** Nothing says whether the server can reach that host.
7. **The address of the old section, `settings?section=github`, now opens the first section.**
8. **The contract and the live website are unchanged.**
9. **The three changes had no blind review, no space checks and no look in the light theme.** They were walked at 1440 by 900 and at 1000 by 680 in the dark theme, with no page error.

## 1629. What the audit of the wording leaves open

Found on 2026-11-24, while conversational headings and labels were renamed in `src/frontend/`, as the owner asked that day. The section "A heading is a noun label" of `design/DESIGN.md` lists every name that changed. Nothing here is fixed.

1. **The new names are the designer's choices.** The owner named one, "Breakdown". The owner has not read the others.
2. **Instructions on screens were removed the same day, except these.** The first-run pages lost the line under each step's heading the same day, when the owner said so. The dialog that adds a folder and the Workflows page lost their sentences the same day too. The Workflows page now does not say that choosing a workflow for a project is not built, so a saved workflow looks as if it applies. The owner has not read the removals.
3. **Sentences that report an event still say "you".** A toast says "You stopped this reply.", and a status says "Waiting for your answer".
4. **The drawn phone screens of the sample project keep the old notification names,** such as "An agent asks you a question". They are sample drawings of another project.
5. **The live website is unchanged.** It is still built from `design/mock/`, which holds the old names.
6. **A project's code cannot be changed, and the contract does not hold it.** The server has to give the code, because two clients can create a project at the same moment.
7. **Older sections of `design/DESIGN.md` and older entries of this file still quote some old names.** Five names were replaced across the documents, and the rest were left as history.
8. **Nothing here had a blind review, a light-theme look or a test.**
9. **Two harnesses of the first-run step show a line that starts with a dot.** "Gemini CLI" and "Cursor" have no models in the sample, so the line under each reads "· ACP mode". This was seen on 2026-11-24 and is older than the audit. It was closed the same day: the step no longer lists models or the run mode.

## 1630. What the first-run list of harnesses leaves open

Found on 2026-11-24, while the first step of the first-run setup was redrawn in `src/frontend/`, as the owner asked that day. Nothing here is fixed.

1. **"Add harness" installed at once.** This was closed the same day: a dialog now shows the commands and asks first. The rest of this point is still open. The step did not show the commands that run on the server, and it did not ask first. The dialog "Add account" in Settings shows the install recipe and its source. A program installed from a web page is a security matter, and `SECURITY.md` has not been read against this step.
2. **The sample installs Cursor like any other harness.** Its recipe in the sample is a script fetched from the internet, and Settings marks it as not automatic. The step should send the owner to the recipe for a harness the server may not install by itself.
3. **"Detected" means two things.** The engine ships its own copy of Claude Code and of Codex with their Agent Client Protocol (ACP) adapters, in `packages/engine/src/harnesses/programs.ts`. Gemini CLI and Cursor are found on the machine's `PATH`. The step shows one list and does not tell the two apart.
4. **Interactive mode will need the terminal program of Claude Code and of Codex.** The adapters do not open it. The step does not say whether that program is on the machine.
5. **A failed install is not drawn.** The row says "Installing…" and then shows a version.
6. **The live first-run page is unchanged.** `LiveFirstRun` in `auth.tsx` still has the heading "Connect a subscription". The contract is unchanged.
7. **The word "subscription" left this step.** Settings still calls a sign-in a subscription or an account.
8. **Nothing here had a blind review, a light-theme look or a test.**
9. **Seven of the nine offered harnesses have never been started by the engine.** They are OpenCode, GitHub Copilot, Pi, Kiro CLI, Grok Build, Google Antigravity and Amp. `docs/research/harnesses.md` says how they were chosen.
10. **The list of supported harnesses is written into the website.** It belongs to the server, which should read the ACP registry and say what it may install. The contract has no call for it.
11. **The install commands in the dialog are the designer's sketch.** The engine's own recipe for Gemini CLI uses `npm ci` with a lockfile. The dialog shows `npm install`, `curl` and `tar` lines that no code runs.
12. **Cursor's archive has no checksum in the registry.** The dialog says "Not published" and still lets the owner install it.
13. **Pi and Amp reach ACP through adapters by other authors.** The dialog does not say so.
14. **The versions and archive addresses are for Linux on x86-64 only.** A machine with another system would need another archive.
15. **An account row on the Harnesses page has a coloured left edge, and a helper sentence under a subscription at its limit.** Both are older than this change and go against the owner's standing rules.
16. **The sign-in drawn on the first-run step is a demonstration, and real harnesses differ.** The owner raised this on 2026-11-24. The step shows a code to compare, for every harness. Claude Code and others give a link to open and no code. How OpenCode signs in was not looked up, and a harness may have no proper sign-in at all. The step has to draw what each harness really does before it is built.

## 1632. What the design of the Go rewrite leaves open

Found on 2026-11-25, while `docs/specs/2026-11-25-go-rewrite-foundation-design.md` was written. Nothing here is fixed.

1. **`AGENTS.md` named the wrong language for new code.** This was closed the same day: the owner had the file rewritten short, and it now names Go. Its "Language" section says TypeScript with the Effect library on Node.js 26. The owner decided on 2026-11-24 that the server and the client are written in Go. A builder who reads the file will write the wrong language.
2. **Rule 4 of `SECURITY.md` kept web sessions in a file.** This was closed the same day: the owner removed `SECURITY.md`, because its rules described the old engine. The design keeps sessions in the database.
3. **`ClientService.ReconnectClient` cannot work as the contract describes it.** A machine dials the server, so the server cannot call a machine that is offline.
4. **`WatchMap` and `WatchPreviews` send the whole record or list on each change.** The design gives every `Watch` call one shape: everything first, then one change at a time.
5. **A plain database sequence does not give commit order.** A check against PostgreSQL 16 showed a reader missing a change that committed late. The change feed must take its positions under a lock, and a test must prove it.
6. **Cursor does not steer or queue over the Agent Client Protocol (ACP).** A second prompt cancels the running turn, starts a new one in the same conversation, stops that turn's subagents, and leaves the interrupted tool call open. Real steering exists only in Cursor's own terminal program. The owner decided on 2026-11-25 that version one uses ACP only.
7. **The Go library for ACP speaks protocol version 1.** The old engine asks for version 2 first. Claude and Codex answer a version 2 greeting with a different shape, which the library does not send.
8. **The terminal library `github.com/creack/pty` has no Windows support.** A client on Windows needs another library or a finished upstream change.
9. **A website hosted on another site than its server cannot sign in with a strict cookie.** The compose stack avoids this by serving both from one address.
10. **A client has no way yet to trust a certificate that no public authority signed.**
11. **The project has no security document.** The owner removed `SECURITY.md` on 2026-11-25. A new one is written when the design has settled. Until then, nothing tells a person how to report a vulnerability.
12. **Older documents still point at `SECURITY.md` and at numbered rules of the old `AGENTS.md`.** They are `docs/remote-access.md`, `docs/features.md`, two files in `docs/research/`, the older files in `docs/specs/`, the plans in `.plans/`, and comments in `packages/`. The three scope files in `.plans/scopes/engine/` name sections of `AGENTS.md` that no longer exist.
13. **Two tests of the old engine read `SECURITY.md`.** The test in `packages/engine/test/harnesses/signin-documentation.test.ts` was removed with the file. The test in `packages/contracts/test/permission-scale.test.ts` now reads only the decisions record.
14. **`README.md` still describes the old engine.**

## 1633. The design of the Go rewrite does not say where encryption ends for the link

Found on 2026-11-25, while the sprints of the Go rewrite were planned. Nothing here is fixed.

1. **The server is reachable only inside the compose stack.** A web server stands in front of it and holds the certificate. The design says the join token carries the fingerprint of the server's certificate. It does not say how the server learns the fingerprint of a certificate that another service holds.
2. **A certificate that a machine presents stops at the web server.** The design says the machine presents a self-signed certificate made from its own key. The server never sees it when the web server ends the encrypted connection.
3. **The challenge that both ends sign already proves the machine.** The design does not say whether the machine's certificate adds anything to it.
4. **The link is one long two-way stream.** The web server must pass it on without cutting it, and no check has shown that yet.

The plans `go-s2-stack-web`, `go-s3-server-machines` and `go-s3-client-link` settle this before sprint 3 is built.

## 1634. A Go test cannot import both programs, and the design asks for one that does

Found on 2026-11-25, while the sprints of the Go rewrite were planned. Nothing here is fixed.

The design's rule 10 says that tests of the link use a real client against a real server in one process.
The design also keeps each program's code under an `internal` folder, so that Go refuses an import from one program into the other.
A test package can therefore import neither `src/server/internal/` nor `src/client/internal/` from outside its own program.

Each program needs an entry point for tests that is not under `internal`, and a third folder needs to hold the tests that start both.
The design names neither.

## 1635. The build loop sizes work in points and gives each lead a cap, and no document defines either

Found on 2026-11-25, while the sprints of the Go rewrite were planned. Nothing here is fixed.

`docs/specs/2026-11-05-build-loop-design.md` says that work is sized in points, and that what is over a lead's cap is split.
No document says what a point is worth, or how many points a lead may hold.
The epic of the Go rewrite sizes each work item as low, medium or high instead, and it tries to split a high item first.

## 1636. The website and the design name the join command differently

Found on 2026-11-26, while the contract of the Go rewrite was planned. Nothing here is fixed.

The website shows the command as `lgtm client join <address> --token <code>`, in `src/frontend/src/screens/service.tsx`.
The design writes `lgtm-client join <address> <token>`.
The screen and the client must agree before check 6 of the design can pass.

## 1637. The contract says that the server reaches a machine at its address, and the server never dials a machine

Found on 2026-11-26, while the contract of the Go rewrite was planned. Nothing here is fixed.

The comment of `Client.address` in `src/contracts/proto/lgtm/v1/clients.proto` says that the address is where the server reaches the machine.
The design has the machine dial the server, always.
The field may still show the address that the connection came from. The comment needs a decision.

## 1638. Nothing on the link tells the server which machine runs the server

Found on 2026-11-26, while the contract of the Go rewrite was planned. Nothing here is fixed.

The website shows `Client.hosts_server` for the machine that also runs the server.
No message of the link carries that fact.
The plan `go-s3-server-machines` or the installer in `go-s3-stack-client` decides where it comes from.

## 1639. Wave 1 has no way to replace the server's signing key or the key of its certificate

Found on 2026-11-26, while the contract of the Go rewrite was planned. Nothing here is fixed.

A machine keeps the server's key and the fingerprint of the certificate's key from the moment it joins.
When either is replaced, every machine that joined before has to join again.
The plan `go-s3-server-machines` says what the owner sees then.

## 1640. A Codex worker in its sandbox cannot reach Docker or a local PostgreSQL

Found on 2026-11-26, while the plans of sprint 1 were written. Nothing here is fixed.

Two Codex planners reported that their sandbox refused the Docker socket, so neither ran a database or the vault.
The design tests against a real PostgreSQL, so a Codex builder needs the network of its own machine to run its tests.
The builders of this run are started with network access inside the workspace sandbox, and the run log records it.

## 1641. Wave 1 does not handle a database or a state folder that was restored to an earlier state

Found on 2026-11-26, in the review rounds of the plans of the Go rewrite. Nothing here is fixed.

The design does not promise that a restore works, and the plans of wave 1 name it as outside their scope.

1. **The server's database is restored to an earlier state.** A machine's event numbers then run ahead of the number the server remembers. The server refuses that machine with one code, and the client stops and reports. A machine that was removed after the restored state is known again.
2. **A client's state folder is restored to an earlier state.** Its event numbers and its command receipts then go back, so an event number can be used twice and a command can run twice.

Until a later wave handles it, the owner removes the machine and joins it again after a restore.

## 1642. Two live copies of one machine's state confuse the server

Found on 2026-11-26, in the third review round of the contract plan of the Go rewrite. Nothing here is fixed.

The contract states that one machine has one outbox.
If a machine's state folder is copied and both copies run, both send events under the same numbers.
The server's acknowledgement of one copy's report can then make the other copy forget a different report.
Wave 1 does not detect this, like the restore in issue 1641.
A later wave needs either an acknowledgement that names the report itself, or a way to detect a second copy.

## 1643. Findings of the review of the built contract that wait for the freeze of the link

Found on 2026-11-26, by the review after the build of sprint 0. The report is `.plans/plans/go-s0-contract/attack/post-build-review.md`. Nothing here is fixed.

The link is frozen when the plans `go-s3-server-machines` and `go-s3-client-link` have cleared review. The Contract team settles these findings before that.

1. **The tests hold the shape of the link and none of its rules** (major). A rule that lives in a comment can be wrong, and every contract test passes. The rules are proven by the tests of the server, of the client and of `src/linktest/` in sprint 3.
2. **The word "final" for a code does not say what a client does, and `event_gap` is called final although a later stream continues past it** (major).
3. `COMMAND_OUTCOME_EXPIRED` has two meanings, and the edge of the retention time can produce a false one.
4. A pending `SetState` has no end, and a removal names no end for pending commands.
5. The removal rule does not say whether a pending event of a removed machine is acknowledged.
6. The value rules do not list the numeric limits that the comments of the fields give.
7. The handshake answers about a machine before it checks the machine's signature.
8. A machine that is approved on a live stream never gets a connection token.
9. A lost reply to `Join` cannot be recovered after the token expired.
10. Nothing tells the client to give up a stream that went silent.

## 1644. Small findings of the reviews of the built client

Found on 2026-11-26, by the reviews after the build of the plan `go-s1-client-local`. The reports are `post-build-a.md` and `post-build-b.md` in `.plans/plans/go-s1-client-local/attack/`. The two major findings, both gaps in tests, went back to the test writer on the same day. These minor ones are open.

1. `Open` of the state folder fails when a parent folder can be searched but not read.
2. No test fails when the removal of temporary files runs before the lock is taken.
3. No test fixes what `Create` returns after a file was placed and not synced, and a placed file can be reported as a plain failure.
4. Three checks of `Load` and `Open` have no test, and `Load` accepts a key file that the plan calls invalid.
5. `Paths()` on a nil folder panics.
6. `WriteAtomic` can replace the lock file.
7. A store file that cannot be opened for permission reasons is reported as damaged.
8. The test that `Read` copies the payload passes even when the copy is removed, so it proves nothing yet.
9. No test checks that the store file is opened with sync at creation. The test needs a hook in the implementation.

## 1645. Small findings of the review of the built database layer

Found on 2026-11-26, by the review after the build of work items 1 to 3 of the plan `go-s1-server-store`. The report is `.plans/plans/go-s1-server-store/attack/post-build-a.md`. Its verdict was to ship. These minor findings are open.

1. The connection defaults, the isolation level of `Read`, the order of the migration parts and the copy of the configuration in `Open` have no test.
2. Two failures give the operator no useful sentence.
3. A test run without the database setting skips the database tests and looks green.
4. The module asks for Go 1.26.0, and the vulnerability scan reports faults of the standard library that later 1.26 releases fix. A `toolchain` line for the newest 1.26 release removes them.
5. `Close` can block forever, and no test covers it.
6. A nested error loses its outcome when it is sanitized a second time.
7. The test fixture does not check the connection limit of the server.

## 1646. Limits of the installer of the client that are outside wave 1

Found on 2026-11-26, by the second review round of the plan `go-s3-stack-client`. The plan accepts each of them for wave 1.

1. The installer cannot join a machine again after a removal, a changed port, a restore, or another stack on the same port. The owner does it by hand.
2. Two runs of the installer at the same time by one user can join twice, and both write the service file.
3. The installer checks no owner and no mode of the parent folders of the program, of the service unit and of the launch agent.

## 1647. Small findings of the review of the built vault stack

Found on 2026-11-26, by the review after the build of the plan `go-s1-stack-vault`. The report is `.plans/plans/go-s1-stack-vault/attack/post-build-all.md`. Its one major finding and three minor ones are gaps in tests, and went back to the test writer on the same day. These are open.

1. A failure after OpenBao is initialized and before the first start finishes loses the only copy of the root token. The install is then stuck, and the way out is to remove the state and start again.
2. A change of a pinned image gives a diagnostic that does not name the image as the cause.
3. The test of the proof trusts the text that the proof prints.

## 1648. Small findings of the review of the built reader of the vault

Found on 2026-11-26, by the review after the build of work item 8 of the plan `go-s2-server-signin`. The report is `.plans/plans/go-s2-server-signin/attack/post-build-b.md`. A reviewer of the builder's own vendor wrote it, because the reviewer of the other vendor was stopped by that vendor's content filter. Its major finding and two minor ones are gaps in tests, and went to a test writer on the same day. These are open.

1. A load with a pin reports "restore the key" for failures that are not a replaced key.
2. One sentence stands for five different failures at startup, and a database error is swallowed.
3. The code that clears the seed from memory has no effect.
4. The client of the vault has no close, and a token that the vault rejected stays cached.
5. The reader gives the application no way to log whether the key was created or adopted, which the plan asks of work item 9.

## 1649. Small findings of the review of the built transport of the client

Found on 2026-11-26, by the review after the build of work item 1 of the plan `go-s3-client-link`. The report is `.plans/plans/go-s3-client-link/attack/post-build-a.md`. Its verdict was to ship. These minor findings are open.

1. The wrapper of the server's address has no test that can fail, and the check that no proxy is taken from the environment cannot fail.
2. Two layers of defence and the case of a stolen certificate have no test.
3. Host names are loosely bounded, and a trailing dot gives a second spelling of one host.
4. The design says the join token carries the fingerprint of the server's certificate and that a machine presents its own certificate. The contract and the code pin the public key of the certificate, and the machine presents none. The design's sentence waits for the owner (issue 1633).

## 1650. Small findings of the review of the built installer of the client on Linux

Found on 2026-11-26, by the review after the build of work item 2 of the plan `go-s3-stack-client`. The report is `.plans/plans/go-s3-stack-client/attack/post-build-a.md`. Its one major finding is that the user-flow script `deploy/test/checks.sh` and its guide still use `XDG_STATE_HOME`, which went to the test writer of that script on the same day. These minor findings are open.

1. No test fails when the installer never waits for the service, or prints the client's output without removing the token.
2. No test fixes the lines of the service unit's template.
3. `XDG_CONFIG_HOME` is not checked, and a relative value writes the unit into the checkout.
4. The search path of the service keeps empty and relative entries.
5. The messages about the state folder and its comparison are too coarse.
6. A relative `COMPOSE_FILE` is read from another folder than the launcher uses.

## 1651. Small findings of the review of the built sign-in parts of the server

Found on 2026-11-26, by the review after the build of work items 1 to 5 of the plan `go-s2-server-signin`. The report is `.plans/plans/go-s2-server-signin/attack/post-build-a.md`. It found no fault in the code that a user can reach. Its one major finding is a gap in the tests: no test forbids a loopback alias as a browser origin when the public origin uses HTTPS. That gap and the other test gaps went to a test writer on the same day. These minor findings in the code are open.

1. The settings loader refuses `PGSERVICE` and lets other driver settings of the process environment reach the pool, such as `PGOPTIONS` and `PGAPPNAME`.
2. The password file of the standalone mode is read without a size limit.
3. The cookie reader accepts a value in double quotes, because the standard parser removes them.
4. The plan has no register with the heading "Not reviewed before the build". Its unreviewed places are marked in the plan itself.

## 1652. Small findings of the review of the built machine records and join tokens

Found on 2026-11-26, by the review after the build of work items 1 and 3 of the plan `go-s3-server-machines`. The report is `.plans/plans/go-s3-server-machines/attack/post-build-a.md`. Its verdict was to ship. Its one major finding is a gap in the tests: no test fails when the row lock of an approval or a rename is removed. That gap and the other test gaps went to a test writer on the same day. These minor findings are open.

1. A missing signing key of the server is a plain error, which a caller will report as a database outage. The key is better checked once, when the token service is made.
2. The registry accepts two names that look the same, one written with a precomposed letter and one with a combining accent.
3. The schema leaves three rules to the code: one machine per token, a name on every approved machine, and the three columns of a used token set together.

## 1653. Small findings of the review of the built installer of the client on macOS

Found on 2026-11-26, by the review after the build of work item 3 of the plan `go-s3-stack-client`. The report is `.plans/plans/go-s3-stack-client/attack/post-build-b.md`. Its verdict was to ship. The macOS branch ran only on Linux with stand-ins, so the check on a Mac is still owed. These minor findings are open.

1. The search path of the macOS agent is not tested for unsafe entries, and relative and empty entries are copied into the service.
2. The macOS branch is tested only with GNU tools, and the wait for the agent has no test of its timeout.
3. An agent file in another layout makes the installer refuse the right state folder.
4. The process type `Background` may slow the harnesses that the client starts.
5. The installer does not wait for the old agent to stop before it loads the new one.

## 1654. Small findings of the review of the built feed reader, follower and watch

Found on 2026-11-26, by the review after the build of work items 7 to 9 of the plan `go-s1-server-store`. The report is `.plans/plans/go-s1-server-store/attack/post-build-c.md`. Its two major findings are gaps in the tests: the production timer for a stale stream has no test, and no test proves that a notification wakes the reader. Those went to a test writer on the same day. These minor findings are open.

1. The feed reader swallows the errors of its listener. It sleeps and retries, and reports nothing.
2. One snapshot is admitted at a time for the whole server, and that limit has no load budget.
3. Every matching change takes one connection of the main pool for each watcher, so a saturated pool ends streams after two seconds.
4. A callback that ignores its context blocks the serving of a watch for good.

## 1655. Small findings of the review of the built web stack

Found on 2026-11-26, by the review after the build of work items 1, 4 and 5 of the plan `go-s2-stack-web`. The report is `.plans/plans/go-s2-stack-web/attack/post-build-b.md`. Its one major finding went to a test writer and a builder on the same day: the launcher checks the settings file quietly at the start, and a later step reads the file again and passes the error text of Compose through, so a settings file that is damaged while the launcher waits for its lock can print the password. This minor finding is open.

1. The suite of the server image does not check that the server program is the only file added to the base image.

## 1656. Small findings of the review of the repaired router

Found on 2026-11-26, by the review after the repair of two findings in work items 6 and 7 of the plan `go-s2-server-signin`. The report is `.plans/plans/go-s2-server-signin/attack/post-build-e.md`. Both repairs hold. These minor findings are open.

1. A call to a procedure without a registered handler still looks up the session without the time limit of the call. The call then answers `unimplemented` late instead of timing out.
2. No test fails when the router reads the body of a request before it checks the origin. The built router does not read it.

## 1657. Small findings of the review of the frozen link

Found on 2026-11-26, by the review after the build of the amendment that freezes the link at revision 1 (plan `go-s0-contract`, items F1 to F5). The report is `.plans/plans/go-s0-contract/attack/post-build-f.md`. Its verdict was to ship, and it confirmed that the wire did not change. These minor findings are open.

1. The test of the table of codes holds the columns of the table. It accepts a row whose prose says the opposite, such as "stop" in a row that means "dial again".
2. The check of the history trusts the commit it is given as the baseline. A branch that passes its own tip as the baseline can rewrite revision 1 unseen.
3. A new revision must be written on its own line with a leading comma. Neither the guide nor the error text says so.
4. Check 3 of the contract plan still says that a gap in the events stops the client, and the amendment says the client dials again.
5. The cases of an encrypted join with a self-signed certificate were moved out of wave 1, and no issue records them. They are: a join over `https` with a self-signed certificate, and the refusal of a wrong pin.

## 1658. Findings of the review of the built Join

Found on 2026-11-26, by the review after the build of work item 4 of the plan `go-s3-server-machines`. The report is `.plans/plans/go-s3-server-machines/attack/post-build-b.md`. The reviewer was of the same vendor as the builder, so the rule that another vendor reviews was not met. Its one major finding went to a test writer and a builder on the same day: a malformed `Join` body can carry the submitted secret or key into the error text, because the decoder puts the raw value into its error. These minor findings are open.

1. A `Join` that stops on its own time limit or is cancelled is reported with the sentence of a database outage.
2. The host name is decoded before the refusals and the replay are decided, although it is checked only after them.

## 1659. The website shows an empty page after sign-in while the watch of machines fails

Found on 2026-11-26, when the website of the built stack was opened in a browser after the sign-in checks of sprint 2 passed. The sign-in page loads, a wrong password is refused with a sentence, and the right password signs in. After that the page stays empty.

1. The Go server of sprint 2 answers `WatchClients` with `unimplemented`, because the calls of machines are wired in sprint 3. This part ends when work item 7 of the plan `go-s3-server-machines` is built.
2. The website asks again and again and shows nothing meanwhile. Its main element stays marked busy and holds no text. A person sees an empty page with no sentence that says what failed. The same would happen whenever the first watch fails for another reason. A change to what the page shows needs the owner's approval.

Update of 2026-11-26: part 1 has ended. Work item 7 was built in commit `aa633e16`, and the page renders after sign-in. Part 2 stays open.

## 1660. A machine reports the full path of a workspace that lies outside the home folder

Found on 2026-11-26 by the review of the built client reports (finding post-c 2 of the plan `go-s3-client-link`). The client shortens a workspace path that lies under the home folder. A workspace on another volume cannot be shortened, and the client then sends the full local path to the server as a fact of the machine. The contract permits the field, and the plan asks for the path, shortened when possible. The server belongs to the same single user, so this was ruled not a fault of the build. Whether such a path is left out or shown is a choice about what the Machines page shows.

## 1661. The dialog that adds a machine shows the commands of the old engine

Found on 2026-11-26, when the Machines page was opened in a browser against the Go server. The page renders, and "Add machine" gets a real join token from the Go server. The dialog then shows two commands that do not exist for the Go client.

1. Step 1 shows `npm install -g lgtm`. The Go client is installed by `deploy/install-client.sh`.
2. Step 2 shows `lgtm client join <address> --token <token>`. The Go client's command is `lgtm-client join`.

A person who follows the dialog cannot add a machine. A change to what the dialog shows needs the owner's approval.

Update of 2026-11-26: the owner approved the change, and commit `fce76a10` made it. Step 2 now shows `lgtm-client join <address> <token>` with the address that the server put in the token. Step 1 shows a `go build` command that needs a checkout of the repository, because the Go client has no installer for another machine yet. Step 1 stays open until such an installer exists.

## 1662. The website shows sample repositories and agents beside the real Machines page

Found on 2026-11-26 in the same look. After sign-in against the Go server, the Machines page shows real data, which is no machine at first. The sidebar and the Home page still show the sample repositories, teams, agents and seven waiting requests of the website's built-in sample data. Wave 1 of the Go server answers only the calls of sign-in and machines. A person cannot tell which parts of the page are real. Whether the website hides the parts that the server does not answer yet is a choice about what the page shows.

## 1663. The launcher has faults when a caller overrides Docker Compose through environment variables

Found on 2026-11-26 by the fourth review of the launcher `deploy/up.sh` (findings post-d 1 to post-d 5 of the plan `go-s2-stack-web`). The default start from the deployment folder passes every check, and so does the way the test scripts call the launcher. Each fault below needs a caller who sets `COMPOSE_FILE`, `COMPOSE_ENV_FILES` or `COMPOSE_PROFILES` in a particular way.

1. With both `COMPOSE_FILE` and `COMPOSE_ENV_FILES` set, and an override file that sets the project name, the launcher starts PostgreSQL in one project and prepares the database in another.
2. With an explicit settings list and an override whose service reads the original settings file as its `env_file`, a settings file damaged during the launch can print the password.
3. A `COMPOSE_PROFILES` value from the working folder is displaced by the launcher's own profile, and services that only that profile enables are missing.
4. The operator guide says to look up the port from the repository root. With `COMPOSE_FILE` set, the lookup must run from the folder of the launch.
5. A termination signal sent to the launcher alone, while it reads a settings source that blocks, leaves the private copy on disk until the read ends.

The review loop on the launcher was stopped here by the orchestrator after four reviews. A simpler rule may serve better than five more repairs: the launcher could refuse a caller's Compose overrides that it does not support, and the test scripts could use one supported way to make a private stack.

## 1664. A refused address of the token issuer is reported as a database outage

Found on 2026-11-26 by the review of the built record calls (finding post-d 3 of the plan `go-s3-server-machines`). The plan names two errors of the issuer of join tokens, an invalid address and a missing certificate pin, and gives neither a public code or sentence. The built handler answers both with the sentence for an unreachable database. The address comes from the server's own settings, so a person meets this only after a wrong setting. The sentence should name the setting.

## 1665. A Machines service without a shared follower starts one follower for each watch

Found on 2026-11-26 by the cross-vendor review of the Machines watch (finding post-c 4 of the plan `go-s3-server-machines`). The server program builds the Machines service with one shared follower of the feed. A service that is built without that option starts its own follower for every call of `WatchClients`, in `src/server/internal/machines/watch.go`. Each of those followers has no function to report an outage, so an outage of the feed is not logged. Each one also holds its own reader, its own listening connection and its own buffer, and all of them share the store's single page connection.

The running server is not affected, because it always passes the shared follower. A test harness or a later program that forgets the option gets a cost that grows with the number of open watches.

The next step is one of two changes. Either the constructor requires a follower, or the path without one is documented as a path for tests and gets a report function that reaches the log.

The same review's finding post-c 8 needs no issue. It said that the plan's test filter also ran a test of the stream plan, which failed then. That test passes since the stream plan's item 9 was built.

## 1666. Ten minor findings of the review of the server program

Found on 2026-11-26 by the review of the server program (findings post-f 1 to post-f 10 of the plan `go-s2-server-signin`, item 9). The review found no major fault and graded the build 8 out of 10. The report is `.plans/plans/go-s2-server-signin/attack/post-build-f.md`. None of the ten blocks the build.

Findings about the program:

1. After a silent loss of the instance lock, the old instance answers until the next probe fails. That is at most six seconds. During a graceful stop nothing probes the lock while the drain runs, for up to five seconds (post-f 4).
2. A failed unlock and a failed close of the store leave no line in the log (post-f 5).
3. An invalid value of `LGTM_ALLOW_PRIVATE_STACK` is reported with the sentence for invalid PostgreSQL settings (post-f 6).

Findings about the tests and the test server:

4. The test server ties a restarted application to the test that calls `Restart`, so a restart from a subtest ends with the subtest (post-f 1).
5. No test separates a busy database from another failure of the lock query (post-f 2).
6. No test fixes the error that `Wait` returns after a lost lock, although the exit code depends on it (post-f 3).
7. The test server reports a refused start with one generic sentence and drops the lines that name the cause (post-f 7).
8. The test server reserves its port, releases it, and binds it again later, so another process can take it in between (post-f 8).
9. Two fields of the test server are written and read by different goroutines without synchronisation (post-f 9).

The tenth finding is about the review's own task file, which named a heading that no file has (post-f 10). It needs no change of the code.

The next step is a small repair pass for points 2, 3, 6 and 9, which are cheap. Points 1 and 8 need a decision on whether the short windows matter.
