---
topic: go-s0-contract
status: attack
tier: M
created: 2026-11-25
updated: 2026-11-26
epic: go-rewrite
---

# The contract and the Go module

- **Sprint:** 0. The contract
- **Team:** Contract
- **Design:** [docs/specs/2026-11-25-go-rewrite-foundation-design.md](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md)
- **Epic:** [go-rewrite](../../epics/go-rewrite/epic.md)
- **Draft contract files:** [draft/](draft/), checked with `buf lint`, generated in both languages and built on 2026-11-26

## Overview

LGTM Studio is being rewritten as three parts: a website, a server and a client on each machine.
The three parts meet only in the contract in `src/contracts/`, which is written in Protocol Buffers.
The server and the client are new programs in Go, and both are built against the contract.
This plan writes the parts of the contract that both Go teams need before they can start, so it is the first step of the whole rewrite.

Today the contract generates TypeScript for the website only, and it describes only what the website says to a server.
After this plan, the repository has one Go module at its root, and the contract generates Go code beside the TypeScript.
The plan also writes down two new things.
The first is the one shape that every live list of the website follows: everything first, then one change at a time, each with its place in the server's ordered record of changes.
The second is the link: what a machine and the server say to each other, from the moment a machine joins until it reports its load every ten seconds.

The link matters most, because the Server team builds one end of it and the Client team builds the other.
This plan gives every message, field name and field number of the link, so that the reviewers of both teams can attack it before it is frozen.
Draft files of the whole contract sit beside this plan in `draft/`.
They pass the contract's lint rules, they generate Go and TypeScript, and the Go builds.
Each building step then makes the real files match a description that the plan fixes in advance, and a test fails when anything differs.

The website builds and signs in as before.
The one field that this plan adds to a message the website reads is optional, so the website ignores it.
The revision number of the contract therefore stays at 1 in this sprint.
One action of the website does change its meaning, though (logic2-r1 1).
The owner decided that `ReconnectClient` restarts a connected client, and today's Machines page offers it only for an offline machine.
Against the Go server, that menu item would always be refused, so the page must follow before the Go server replaces the sample server.

Four points need the owner's attention, and none of them blocks sprint 0.
They are listed with their options in the section "Points for the owner" below.

1. How the Machines page follows the new meaning of `ReconnectClient`.
2. Whether the signed challenge is enough to protect the link after the web server in front of the server ends the encryption. The design says that a machine presents a certificate made from its own key, and that certificate would never reach the server.
3. What a machine may send while it waits for approval. The design says it may only connect, and today's page shows the facts of a waiting machine.
4. Whether the join token may be typed as an argument of a command, where the shell's history keeps it.

The design's list of what travels in wave 1 has no way to restart a client either.
This plan adds a restart command for that.

## Key decisions

- **The shared shape is a rule that a test checks.** Protocol Buffers cannot express "a list of anything", so each live list keeps its own message. Every such message uses the same field names and numbers, and a test fails when one does not. Two existing lists, the codebase map and the previews, do not follow the shape yet; the design moves them later.
- **The place in the change feed is its own small message.** It holds one growing number today, and it leaves room for a later field without breaking anything.
- **The Go code lives in `src/contracts/gen/go/`.** A trial run showed that the tool `buf` keeps the TypeScript there and adds the Go beside it.
- **The join token is a small message written as text.** It carries the token's id and secret, the server's address, and the fingerprints of the server's key and of its certificate's key. Both programs read it with generated code, so no two hand-written readers can disagree.
- **The token pins the certificate's key, not the whole certificate.** A renewed certificate that keeps its key then still works for every machine that joined.
- **A machine proves itself only by signing a challenge.** The design's certificate of the machine adds nothing once encryption ends at the web server. Whether that is enough is point 2 for the owner. (threat-r1 2)
- **A heartbeat is not kept on disk.** Facts and harness reports are numbered and kept until the server confirms them. A heartbeat only says "here now", so a stored one would report an old load. The server answers each heartbeat, so traffic moves both ways and the web server in front does not cut the connection.
- **Only the server decides which events it applied.** The machine numbers its events from 1 with no gaps. The server takes only the next number, saves the report and its count in one step, and only then confirms. A copy is confirmed again, and a gap ends the stream. Nothing the machine says moves the server's count. Round 2 removed the field by which it could, because two rounds found blockers there. (architect-r1 01, logic-r1 01, critic-r1 5, threat-r1 3, ruling 1 of round 2)
- **A restore or a copy of a machine's or the server's state is outside wave 1.** One machine id has one outbox. A machine whose numbers no longer fit the server's meets a gap, and a machine the server no longer knows is refused, both with one final code. The owner then removes the machine and joins it again. (ruling 2 of round 2, threat-r3 02)
- **This plan states facts of the wire and nothing else.** A fact of the wire is a message, a field, a number, a code, the bytes that are signed, and the order of messages that the other end can observe. What a program does inside, such as its exit status, how it keeps receipts, when it checks an expiry, which address an installer fills in or how the server orders its writes, belongs to the plan of that program. (ruling 2 of round 3)
- **A command sent twice runs once, and its answer still counts.** A machine that sees a command again sends its recorded answer under the new attempt number, so the server never ignores it as old. The server's clock decides when a command expires. A command starts only before its expiry, and a machine that cannot establish that declines it. A recorded answer comes before every rule of expiry. (UNREVIEWED FIX: architect-r4 05, logic-r4 05) A command that stopped halfway answers that its effect is unknown. A machine sends no text with an answer, so the website shows only text that the server writes. (architect-r1 07, logic-r1 02, logic-r1 03, architect-r2 07, threat-r2 10)
- **Removing a machine does not wait for the machine.** The server orders the commit against the moment it admits a stream: a stream admitted after the commit receives no `Welcome`, and a `Welcome` sent before it may still arrive, followed by the refusal, with no authority left. The server ends its stream with one final code, and refuses each later stream with the same code after it proved itself. A notice to the machine is a courtesy. (architect-r1 02, logic-r1 04, threat-r1 1, architect-r2 06, logic-r3 05; UNREVIEWED FIX: architect-r4 01, logic-r4 02)
- **Only the newest proven stream of a machine counts.** An older stream that ends late cannot mark the machine offline. How the server orders a new stream, a removal and the writes of a stream is the Server Machines team's design. (logic2-r1 4, logic-r1 06, ruling 3 of round 2)
- **The Machines page shows one state per machine.** A machine is waiting until approval, joining until the server applied its first two valid reports on its current stream, then online, and offline once approved without a stream. (architect-r2 08, logic-r2 09, architect-r3 07)
- **The reply that makes a token carries the token's address.** A page that shows a join command can take the address from there. (architect-r2 10, threat-r2 08)
- **Changes of a machine's standing carry a revision number.** An approval sent again after a rename cannot bring the old name back.
- **Restarting a client is a command.** The comment of `ReconnectClient` changes to match the owner's decision. The wire format changes only by added fields, so the revision stays at 1 in this sprint. The meaning of the call changes for the website, which is point 1 for the owner. (logic2-r1 1)
- **Room for wave 2 is left open, not reserved.** Wave 2 adds new kinds of messages and new calls beside the stream. A client that does not know a command answers that it cannot run it.

Full log → `decisions.md`

## Workstreams

### Both teams can build Go against one written contract

This is the plan's only workstream, in eight small steps.

After it, `go build ./...` passes at the repository root, and the contract generates Go for the server and the client.
The contract check fails when the generated Go is out of date, and it now also fails when a generated file was never committed.
The website and the sample server build and sign in as before.
The shared shape of live lists and the link are written as contract files, and their every name and number is fixed by a description that a test compares.

The main risk is that the link is wrong in a way that only shows when both ends are built in sprint 3.
Three things lower it.
The reviewers of both teams attack the drafts before the steps are built.
The exact bytes that each end signs come with worked examples, so the server and the client test against the same bytes without sharing code.
After the steps merge, a change to the contract goes back to planning, by the rule of the epic.

A second risk is that the generation step needs Go on every machine that regenerates the TypeScript.
The README says so, and Go is needed for the rest of the rewrite anyway.

## Rollout order

1. Create the Go module at the root (item 1). Nothing else can generate Go before it exists.
2. Turn on Go generation (item 2). From here on, every step regenerates all of `gen/`.
3. Write the Go check command (item 7) beside the rest of the chain. It needs the generated packages, because Go's vet and test steps fail on a module with no package.
4. Add the shared shape of live lists (item 3). It also brings the test helper that items 4 to 6 use.
5. Add the commands and events of the link (item 4).
6. Add joining (item 5).
7. Add the stream and its proof (item 6). The link is complete after this step.
8. Update the contract's README (item 8).
9. The orchestrator merges the sprint. The link is frozen later, when the plans `go-s3-server-machines` and `go-s3-client-link` have both cleared their reviews, which is before either team builds against it. Until then, a change to the link files is an ordinary work item of the Contract team. After the freeze, a change goes back to phase 3 of the build loop. (ruling 1 of round 3, logic-r3 02, architect-r3 09)

Items 3 to 6 run one after another because each one regenerates the whole of `gen/`, so two at once would always collide there.

## Points for the owner

Review round 1 raised four choices that the design does not settle.
The plan does not decide them, and items 1 to 5 of sprint 0 can be built while they wait.
Points 1 and 2 must be settled before the link is frozen, because option B of point 1 adds a call and options B and C of point 2 change the link or how it travels. (architect-r2 04)
Until the freeze, a choice that changes the link is an ordinary work item of the Contract team, as step 9 of the rollout says. (ruling 1 of round 3)
Points 3 and 4 change no field, and they must be settled before sprint 3, where the Server and Client teams build both ends of the link.

**1. How the Machines page follows the new meaning of `ReconnectClient`.**
Five reviewers found it (architect-r1 03, critic-r1 2, logic-r1 07, logic2-r1 1, threat-r1 12).
The page shows "Try to reconnect" only for an offline machine, and the sample server accepts the call only then.
The Go server refuses exactly that case, because it never dials a machine.

- **Option A:** in sprint 3, one pull request changes the page, the sample server and the Go server together. The page offers a restart for an online machine, and the contract revision becomes 2 in all three, because the change breaks a page of revision 1. The cost is a visible change of the Machines menu, and one pull request across three teams.
- **Option B:** add a new call `RestartClient` for a connected machine, and keep the old meaning of `ReconnectClient`, which the Go server always refuses. The revision stays at 1. The cost is a second call for one action, and the old menu item stays useless until the page changes.
- **Option C:** the Go server answers `ReconnectClient` for an offline machine by waiting for that machine's next stream, and restarts nothing. Restarting waits for a later wave. The cost is that it departs from the owner's decision of 2026-11-25 that the call restarts a connected client.

**2. Whether the signed challenge protects the link well enough.**
One reviewer rated it a blocker (threat-r1 2).
The web server in front of the server ends the encryption, and the hop behind it carries no encryption.
The signatures prove both ends at the start of a stream, but nothing protects the messages after that on that hop.
The design also says that a machine presents its own certificate, which never reaches the server in this layout.

- **Option A:** keep the layout. The hop behind the web server stays inside the compose stack of one machine, and the signed challenge is the proof of the machine. The cost is that a process on that machine which can read that hop can change commands and reports, and can send a status that makes a machine stop for good. It can also change an acknowledgement, so that a machine forgets a report the server never applied, or change `Welcome.server_time`, so that a machine refuses or keeps a command wrongly. Rounds 2 and 3 added these costs. (threat-r2 04, threat-r3 09)
- **Option B:** the web server passes the encrypted connection of the link through untouched, and the server ends it with the design's certificates on both sides. The cost is a second certificate at the server, and work for the Stack team in sprint 2.
- **Option C:** both ends agree on a session key during the challenge, and each message of the stream carries a code that proves it. The cost is new fields in `Hello` and `Challenge`, and code on both ends that no library gives.

**3. What a machine may send while it waits for approval.**
Two reviewers found it (architect-r1 08, threat-r1 7).
The design says that a waiting machine can connect and can do nothing else.
The draft lets it send heartbeats and all events, and the page shows the operating system, cores and memory of a waiting machine.

- **Option A:** follow the design. A waiting machine sends heartbeats only, and the server refuses an event from it. Its connection token is empty. The cost is that the approval row shows only the host name and the address.
- **Option B:** a waiting machine may also send `Facts`, which the server shows as self-reported. It sends no harness list and receives no work, and its connection token is empty. The cost is that a machine that was never approved writes a few short fields to the database.
- **Option C:** keep the draft, in which a waiting machine sends heartbeats and all events. The cost is the most data from a machine the owner has not approved.

**4. Whether the join token may be typed as an argument of a command.**
One reviewer found it (threat-r1 5).
The design writes `lgtm-client join <address> <token>`, and the shell's history keeps that line.

- **Option A:** keep the argument. The token works once and for 15 minutes, which limits the harm. The cost is a used secret left in the history file.
- **Option B:** the client reads the token from a prompt that does not echo, or from standard input. The page shows `lgtm-client join <address>`, and the owner pastes the token when asked. The cost is a second step on the new machine, and a change of the page's text.
- **Option C:** accept both, with `-` in place of the token to read it from standard input. The cost is two ways to do one thing.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

## Amendment: the freeze of the link

The orchestrator decided on 2026-11-26 that the link is frozen once plan `go-s3-client-link` has cleared review and the findings of issue 1643 and the requests of the other plans are settled. That plan cleared its fourth round on 2026-11-26. This amendment settles both lists and then freezes the link at revision 1. After the freeze, a change to a file of the link needs revision 2. One reviewer of the other vendor reads the amendment before it is built.

Almost every change is a comment. The wire keeps every message, field, number and code: `buf build --exclude-source-info` gives the same bytes before and after. Two value rules become stricter, and the "Changes to built code" below names what they cost.

The text of `link.proto`, `events.proto` and `commands.proto` after the amendment is in `draft/`. It passed `buf lint`, `buf generate` and every test of `src/contracts/`, including the new tests of items F4 and F5, in a scratch copy on 2026-11-26.

### Rulings and owner defaults that hold

- The admission of a stream is one commit of the database. It both replaces the older stream and lets the server send the new stream's first frame. (decision 75)
- The client trusts its own clock to drop its records of commands. A clock more than an hour fast is an accepted limit of wave 1. (decision 74)
- A restore of the server's database and two live copies of one machine's state are outside wave 1, as issues 1641 and 1642 record.
- The signed challenge is the proof of a machine in wave 1. The hop behind the web server stays inside one compose stack, which settles point 2 for the owner at option A and issue 1633 for wave 1. (decision 83)
- The server has one user with one password. This is why the handshake may answer about a machine id before the signature verified. (decision 77)
- Owner default: a waiting machine may send heartbeats and `Facts` only, which is option B of point 3. (decision 81)
- Owner default: the join token is accepted as an argument and on standard input, which is option C of point 4. The contract does not change for it. (decision 82)
- Point 1 for the owner, the Machines page and `ReconnectClient`, does not touch a file of the link and stays open.

### Every finding and request

| Source | What it asks | Outcome |
|---|---|---|
| Issue 1643, finding 1 | The tests hold the shape of the link and none of its rules. | New tests in `src/contracts/`, items F4 and F5. The rules that only built programs can prove are listed below with their owners. |
| Issue 1643, finding 2 | "Final" does not say what a machine does, and `event_gap` is called final. | Changed in a comment. The table of codes gains a column, "stop" or "dial again", and `event_gap` is "dial again". `testdata/link-codes.json` holds the table as data. |
| Issue 1643, finding 3 | `COMMAND_OUTCOME_EXPIRED` has two meanings, and the edge of the 24 hours has no margin. | Changed in a comment. The enumeration names both meanings, and the server sends a command at most until 23 hours after its expiry, an hour before the machine drops its record. |
| Issue 1643, finding 4 | A pending `SetState` has no end, and `APPROVAL_REMOVED` has no rule. | Changed in a comment. A removal ends every pending command. A machine treats `APPROVAL_REMOVED` as waiting and keeps its stream, and `Welcome.state` is never `APPROVAL_REMOVED`. |
| Issue 1643, finding 5 | An event of a removed machine may or may not be acknowledged. | Changed in a comment. The server sends no `EventAck` after the removal commits. |
| Issue 1643, finding 6 | The value rules do not list the numeric limits. | Changed in a comment, as a value rule: the limits of `Facts.cores`, the harnesses, the load and `Event.number`, a heartbeat that breaks one, and the value rules of `Join` before its refusals. |
| Issue 1643, finding 7 | The handshake answers about a machine before its signature. | Changed in a comment, which accepts the answer for wave 1 with its reasons. Moving the answer after the signature would leave a client unable to tell a removal from a bad key. |
| Issue 1643, finding 8 | A machine approved on a live stream never gets a connection token. | Changed in a comment. It gets one on its next stream, and no call of wave 1 presents it. A call of wave 2 that needs the token needs revision 2 or a new stream. |
| Issue 1643, finding 9 | A lost `Join` reply cannot be recovered after the token expired. | Changed in a comment. The order stays, and the comment names the recovery: a new token, and the owner removes the waiting machine. The owner's options on that record stay with decision 79 of `go-s1-client-local`. |
| Issue 1643, finding 10 | Nothing tells a machine to give up a silent stream. | Changed in a comment. A machine that hears nothing for 30 seconds ends the stream and dials again, as item 7b of `go-s3-client-link` already does. |
| `go-s3-server-machines` rounds 1 and 2: the host name holds no zero byte | PostgreSQL text holds no zero byte. | Changed on the wire, as a value rule for every string. |
| `go-s3-server-machines` rounds 1 to 3: `Event.happened_at` | The server does not keep it. | Changed in a comment: the server need not keep it. |
| `go-s3-server-machines` round 1: `CreateClientToken` and `RemoveClient` | Name the real command, and say that a waiting machine can be removed. | Changed in a comment of `clients.proto`, item F3. |
| `go-s3-server-machines` rounds 2 and 3: the moment of admission | Name the commit of admission, with the exact sentence of round 3. | Changed in a comment, with that sentence. |
| `go-s3-server-machines` rounds 2 and 3, `go-s3-client-link` round 1, `go-s1-client-local` round 4: the clock of the machine | State the limit of "a command runs once" for a fast clock. | Changed in a comment of `commands.proto`. |
| `go-s3-server-machines` rounds 2 and 3: the largest event number | 2^63 − 1, and a higher number is refused. | Changed on the wire, as a value rule, refused with `invalid_argument`. |
| `go-s3-server-machines` round 3: the 23 hours and the first outcome | The server sends a retry only until 23 hours after the expiry, and the first stored outcome wins. | Changed in a comment of `commands.proto`. |
| `go-s3-server-machines` round 3: the encrypted cases | Move them out of the combined acceptance of wave 1. | Outside wave 1: the `https` part of post-build check 2 moves out, and the result file asks for an issue in `docs/issues.md`. A server of wave 1 makes loopback tokens only. |
| `go-s3-client-link` round 1: the list of names to align | Keep the names of `Join`, `Open`, the reasons, the marks and `CommandResult`. | No change: the contract already holds every name the present plan reads. |
| `go-s3-client-link` round 2: bounds of the connection token and the name | Write the planned bounds into the contract. | Rejected: the plan withdrew the request in its round 3, and the combined test checks the server's real replies. |
| `go-s1-client-local` round 4: retention by count | Change the comment of retention. | Rejected: the plan withdrew it, because its ruling 1 keeps the 24 hours. |
| `go-s3-client-link` round 1, `go-s1-client-local` round 4: `SetState` and expiry | Apply `SetState` whatever its expiry, and keep it pending. | No change: the contract already says so. |
| `go-s3-stack-client` rounds 1 to 4 | None to the contract. | No change. |
| `go-s3-server-machines` round 4, request 1: a stream admitted before a newer admission | It receives no `Welcome` once the newer stream is admitted, and ends with `aborted`. | Changed in a comment of `Open`, with the plan's sentence, and in post-build check 23. |
| `go-s3-server-machines` round 4, request 2: the encrypted join | The combined acceptance uses the loopback route only, and the client's own pin tests stay. | Outside wave 1, with the plan's sentence in the introduction of the post-build checks and in check 2. |
| `go-s3-server-machines` round 4, request 3: the send window | Only commands other than `SetState` stop at 23 hours; a result is stored until the server records the expiry, no earlier than 24 hours. | Changed in a comment of `commands.proto`, and check 11 retries at 22 hours and 30 minutes. The check under the server's lock is the server's own, by ruling 2 of round 3. |
| `go-s3-server-machines` round 4, requests 4 to 6: the clock, the ceiling and `Event.happened_at` | The plan's exact sentences. | Changed in comments of `commands.proto` and `events.proto`, with those sentences. |

### Which rules have a test in `src/contracts/`

A test in `src/contracts/` can read the text of the rules, not run them. Item F4 tests the rules that a reader can mistake, and the two ends prove the behaviour.

| Rule | Proven by |
|---|---|
| The table of codes, its column "stop" or "dial again", and the key of each reason | `TestLinkCodesAreWritten` against `testdata/link-codes.json`. Plan `go-s3-client-link` reads the same file in `TestRetryCodeTable`, and plan `go-s3-server-stream` reads it in its tests of `Open`. |
| The order of the refusals of `Join`, and the six decisions after `Challenge` | `TestJoinRefusalOrderIsWritten` and `TestHandshakeOrderIsWritten`. |
| The numbers: 10 seconds, 30 seconds, 23 and 24 hours, 2^63 − 1, 0 to 100, 16 harnesses | `TestLinkNumbersAreWritten`. |
| The order of the proof text and its two labels | `TestProofCommentMatchesTheExample`, against the rule that `TestProofExamplesHoldTogether` builds. |
| The link has not changed since revision 1 | `TestLinkIsFrozenAtItsRevision` and `scripts/check-link-frozen.sh`, item F5. |
| The table of events, the acknowledgement after the commit, and a gap | Plans `go-s3-server-stream` and `go-s3-client-link`, item 4, and post-build checks 3, 17 and 24 in `src/linktest/`. |
| The admission, the current stream, and a removal against a `Welcome` in transit | Plan `go-s3-server-stream`, and post-build checks 10, 16 and 23. |
| The claim, the replay and the races of `Join` | Plans `go-s3-server-machines` and `go-s3-client-link`, item 2, and post-build checks 1, 12 and 25. |
| The life of a command, its expiry, `SetState` and the records of the machine | Plans `go-s3-server-commands` and `go-s3-client-link`, item 6, and post-build checks 6, 11, 19 and 20. |
| The four states of the website | Plans `go-s3-server-stream` and `go-s3-server-machines`, and post-build checks 4, 13 and 22. |
| The reasons in the metadata of the error, through the web server | Plans `go-s3-server-stream`, `go-s3-client-link`, item 3, and `go-s2-stack-web`, and post-build check 18. |

### Changes to built code

- `Event.number` is at most 2^63 − 1. In `src/client/internal/link/outbox.go`, `Append` returns `ErrSequenceExhausted` when the next number would pass `math.MaxInt64`, in place of `math.MaxUint64`. The tests at `outbox_test.go:121` and `outbox_test.go:174` take the new edge. The Client team makes this change, because the Contract team may not edit `src/client/`, and the result file asks for it.
- A string holds no zero byte. The client's reports and its host name must leave zero bytes out. No built client code sends a string yet.
- The Go and TypeScript code that `buf generate` makes changes in its comments only. Each of items F1 to F3 regenerates it and commits it.

### Work items

Every item regenerates `src/contracts/gen/` and passes `pnpm --filter ./src/contracts check` and `scripts/check-go.sh`. Each item runs after the one before it.

**Item F1. The comments of `link.proto`.**
- **Files:** `src/contracts/proto/lgtm/link/v1/link.proto` becomes `draft/link.proto`. `gen/` follows.
- **What changes:** the value rules, the table of codes with its column, a stream that a newer admission overtakes before its `Welcome`, the value rules of `Join` first, the lost reply, the accepted answer before the signature, the admission as one commit, the silence of 30 seconds, the token after approval, a waiting machine's `Facts`, the removal with the sentence of `go-s3-server-machines` round 3, and `Welcome.state`. (issue 1643, findings 2 and 4 to 10)
- **Done when:** `cmp draft/link.proto src/contracts/proto/lgtm/link/v1/link.proto` exits 0, and `buf build --exclude-source-info` gives the same md5 as before the item.

**Item F2. The comments of `commands.proto` and `events.proto`.**
- **Files:** both files become their copies in `draft/`. `gen/` follows.
- **What changes:** the 23 hours for commands other than `SetState`, the storing of a result until the server records the expiry, the meaning of an outcome the server records itself, the limit of the machine's clock, the ceiling of event numbers, the end of pending commands at a removal, the first outcome, the margin of the clocks, `APPROVAL_REMOVED`, `Event.happened_at` and the silence on `HeartbeatAck`. (findings 3, 4 and 10)
- **Done when:** `cmp` exits 0 for both files, and the md5 of `buf build --exclude-source-info` is unchanged.

**Item F3. Two comments of `clients.proto`.**
- **Files:** `src/contracts/proto/lgtm/v1/clients.proto`, and `gen/` for Go and TypeScript.
- **What changes:** "The server's own command `lgtm client add` makes the same kind of token." becomes "The server's own command `lgtm-server join-token` makes the same kind of token." The comment of `RemoveClient` becomes "RemoveClient disconnects a client and forgets it. It removes a client that waits for approval in the same way."
- **Done when:** `grep -c 'lgtm client add' src/contracts/proto/lgtm/v1/clients.proto` prints 0, and `pnpm --filter ./src/frontend typecheck` exits 0.

**Item F4. Tests of the rules as text.**
- **Files:** new `src/contracts/rules_test.go` and `src/contracts/testdata/link-codes.json`.
- **`link-codes.json`:** an array of nine objects in the order of the table, each with `call`, `code`, `action` ("stop" or "dial again"), and where it applies `key`, `reasons` and `after_verified_challenge`. Its rows: `Join permission_denied stop`, `Join invalid_argument stop`, `Open failed_precondition stop` with `unknown_machine` and `link_revision_mismatch`, `Open failed_precondition dial again` with `event_gap`, then `unauthenticated`, `aborted`, `invalid_argument` and `deadline_exceeded` on `Open`, and `both unavailable`, all "dial again".
- **Tests written first:** `TestLinkCodesAreWritten` and `TestLinkNumbersAreWritten` fail on the built files of sprint 0 and pass on the drafts. The other three hold already, and guard rules that the reviewers showed could change unseen.

| Test | What it checks |
|---|---|
| `TestLinkCodesAreWritten` | The table in the `LinkService` comment has one row per object, in order, with the same call, code and action, and each row's text names its key and its reasons. |
| `TestJoinRefusalOrderIsWritten` | The five reasons of `Join` appear as `//   - <reason>:` in their order. |
| `TestHandshakeOrderIsWritten` | The six decisions after `Challenge` appear in their order, as exact lines. |
| `TestLinkNumbersAreWritten` | Exact phrases: the 10 seconds of `Hello` and `Proof`, the 30 seconds of both ends, 2^63 − 1, 0 to 100, 16 harnesses, the 23 and the 24 hours, and both labels. |
| `TestProofCommentMatchesTheExample` | The comment of `Proof` gives the label, the zero byte, the length, the machine id, the machine nonce and the server nonce in that order, and names both labels. |

- **Done when:** `go test ./src/contracts/ -run 'Codes|Order|Numbers|ProofComment' -v` passes all five. Moving the line of `unauthenticated` above `unavailable` in the scratch copy makes `TestHandshakeOrderIsWritten` fail.

**Item F5. The freeze at revision 1.**
- **Files:** new `src/contracts/testdata/link-frozen.json`, `src/contracts/freeze_test.go` and `scripts/check-link-frozen.sh`. `scripts/check-go.sh` calls the script after `go test ./...`. `src/contracts/README.md` gains, above the line on revision 1: "The link `lgtm.link.v1` is frozen at revision 1. A change to `link.proto`, `commands.proto` or `events.proto` needs the next revision and a new entry in `testdata/link-frozen.json`."
- **`link-frozen.json`:** an array with one object per revision, each on one line: `revision` and `files`, the SHA-256 of each of the three files of the link. The first entry is revision 1, written after items F1 and F2.
- **`TestLinkIsFrozenAtItsRevision`:** the revisions run 1, 2 and so on. The last entry names every file of `proto/lgtm/link/v1/`, and each file's SHA-256 equals it. `link.proto` says "which is N" for the client's and the server's revision, and the README says "is frozen at revision N", where N is the last revision.
- **`scripts/check-link-frozen.sh`:** it reads the record at the base, `LGTM_LINK_BASE` or `git merge-base HEAD master`. Every line with `"revision"` there must be unchanged, in the same place, in the present record. A base without a record passes.
- **Done when:** `scripts/check-go.sh` exits 0. Adding one line to `events.proto` makes `TestLinkIsFrozenAtItsRevision` fail. Changing a hash of revision 1 in a scratch repository whose `master` holds the record makes the script exit 1.

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**WS-1: Both teams can build Go against one written contract**

This workstream turns the brief's seven work items into eight.
The brief's item on joining, rated high, is split into joining (item 5) and the stream (item 6), each rated medium.
The brief's item on the check command is split into the Go check (item 7) and the README (item 8).
The design's statements are cited as D1 to D23 (its decisions table), R1 to R11 (its rules) and C1 to C13 (its checks), counted in the order of the design file on `master`.
Every item has the scope `contract/contracts`, which `scope.py check . contract/contracts` accepted on 2026-11-26.

The facts that every item builds on were checked on 2026-11-26 in a scratch copy and are in `scouting.md`.
The installed Go 1.23.3 switches itself to the cached Go 1.26.0 when `go.mod` says `go 1.26.0`.
`buf` 1.75.0 is at `src/contracts/node_modules/.bin/buf`.

**Public names that other teams build against**

| Name | Where | What it is |
|---|---|---|
| `github.com/qnlbnsl/lgtm-studio` | `go.mod` | The module path. |
| `github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1`, package `lgtmv1` | generated | The messages of the website's contract. |
| `.../gen/go/lgtm/v1/lgtmv1connect` | generated | The Connect handlers and clients of `SessionService`, `ClientService`, `MapService` and `PreviewService`. |
| `.../gen/go/lgtm/link/v1`, package `linkv1` | generated | The messages of the link. |
| `.../gen/go/lgtm/link/v1/linkv1connect` | generated | `LinkServiceHandler`, `NewLinkServiceHandler`, `NewLinkServiceClient`, and the procedure names `/lgtm.link.v1.LinkService/Join` and `/lgtm.link.v1.LinkService/Open`. |
| `lgtm.v1.FeedPosition`, field `uint64 value = 1` | `common.proto` | A place in the change feed. |
| `WatchClientsResponse.position`, field 4 | `clients.proto` | The place of each message of `WatchClients`. |
| The shape of every `Watch` reply | `common.proto` comment | `oneof change { <Record>List all = 1; <Record> saved = 2; string removed_id = 3; }` and `FeedPosition position = 4`. |
| The code `aborted` on a `Watch` call | `common.proto` comment | The caller fell too far behind and opens the call again. |
| The text of a join token | `link.proto`, `JoinToken` | `lgtm1_` followed by the binary encoding of `JoinToken` in unpadded base64url. |
| The proof texts | `link.proto`, `Proof` comment | The label, one zero byte, one byte with the length of the machine id, the machine id in UTF-8 (Unicode Transformation Format, 8 bits), the 32-byte machine nonce and the 32-byte server nonce. The labels are `lgtm-link-v1 server` and `lgtm-link-v1 machine`. |
| The header `Lgtm-Link-Token` | `link.proto`, `LinkService` comment | How a later call beside the stream presents the connection token. |
| `link_revision` 1 | `Hello`, `Welcome` | The revision of the link that both programs speak. Each program names it, as the website names the contract revision. |
| `Welcome.last_event_number`, field 3, and `EventAck.through` | `link.proto`, `events.proto` | The last event number that the server applied from the machine, written in the transaction of that event's effect. The Server Link writes both, only after the commit. The Client Link reads them. A machine never changes the number of an event. (logic-r3 01) Nothing a client sends moves the server's number: the field `Hello.acked_event_number` of round 1 is gone. UNREVIEWED MECHANISM: ruling 1 of the orchestrator in round 2 replaced the mechanism of round 1 by the design's three lines. (architect-r2 01, architect-r2 02, logic-r2 01, logic-r2 02, logic2-r2 1, logic2-r2 2, threat-r2 01, threat-r2 05) |
| `COMMAND_OUTCOME_INTERRUPTED`, value 5 | `commands.proto` | The machine started the command and cannot tell how it ended. The Client Link writes it, and the Server Machines team shows it. (architect-r1 07, logic-r1 03, critic-r1 6) |
| `Heartbeat.cpu_percent` and `memory_percent`, now `optional` | `events.proto` | Absent means that the machine could not measure its load. The Client Link writes them, and the Server Link copies them into the website's optional load fields. (logic-r1 10) |
| `CommandResult.message`, field 4, removed and reserved | `commands.proto` | The machine sends no text with a result. The Client Link writes only the id, the attempt and the outcome. The Server Link reads only those and discards any unknown field. The Server Machines team writes the text that the website shows from the outcome. (threat-r2 10, architect-r3 08, logic-r3 12, threat-r3 03) |
| `CreateClientTokenResponse.address`, field 4 | `clients.proto` | The address inside the new token, which is the address the machine dials. The Server Machines team writes it, and a sample server may leave it empty. The website reads it. Which address a page or an installer shows is the website's and the Stack Client's own choice, by ruling 2 of round 3. (architect-r2 10, logic-r2 10, threat-r2 08, architect-r3 10) |
| The error metadata key `Lgtm-Join-Refusal` | `link.proto`, `LinkService` and `Join` comments | The reason of a refused `Join`, one word in the metadata of the error, which the Connect protocol carries as response headers of the unary error and gRPC as trailers: `token_unknown`, `token_expired`, `token_revoked`, `token_used` or `machine_removed`. The error's message is a sentence for people. The Server Machines team writes it in the metadata of the `connect` error, and the Client enrollment reads it from the error's metadata. Stack Web passes headers and trailers on unchanged. (architect-r3 03, logic-r3 06, threat-r3 05; UNREVIEWED FIX: architect-r4 03, logic-r4 03) |
| The error metadata key `Lgtm-Link-Refusal` | `link.proto`, `LinkService` comment | The reason of a final `failed_precondition` on `Open`: `unknown_machine`, `event_gap` or `link_revision_mismatch`. It is in the metadata of the stream's terminal error, which the Connect protocol carries in its end-of-stream message and gRPC in trailers, so it works after `Challenge` and long after `Welcome` alike. The Server Machines team and the Server Link write it in the metadata of the `connect` error that ends `Open`. The Client Link reads it from the terminal error, for check C10, which reads the reason `unknown_machine` from the client's log. Stack Web passes the stream's body and trailers on unchanged. (architect-r3 04; UNREVIEWED FIX: architect-r4 03, logic-r4 03) |
| `src/contracts/testdata/join-token.json` | item 5 | A worked example of a join token. |
| `src/contracts/testdata/link-proof.json` | item 6 | Worked examples of both proof texts and signatures. |
| The error codes of the link, and which are final | `link.proto`, `LinkService` comment | `permission_denied` for a refused join token; `failed_precondition` for an unknown or removed machine, an event that leaves a gap, and a link revision the server does not speak; `unauthenticated` for a bad signature; `invalid_argument` for a message out of order or one that breaks a value rule; `deadline_exceeded` when `Hello` or `Proof` is more than 10 seconds late; `aborted` for a stream that a newer one replaced; `unavailable` for a server that cannot read its database. The comment's table says which codes are final. A final code depends only on the request and on the server's state when the server decides it, so the same request against the same state gets the same answer. (UNREVIEWED FIX: logic-r4 07, architect-r4 06) What a client does on each code, and its exit status, belong to plan `go-s3-client-link`. The Server Link and the Server Machines team write the codes. The Client Link and the Client enrollment read them. (logic2-r1 6, logic-r1 09, architect-r2 06, logic-r2 06, logic-r2 11, ruling 2 of round 3) |
| The value rules of the link | `link.proto`, file comment | Canonical UUIDs, exact lengths of nonces, keys, signatures and hashes, counters from 1, the fields that must be present, and the length limits of strings. Both link teams validate against them. After `Welcome`, the table of events in `events.proto` comes before them for every event. (logic-r1 09, critic-r1 10, logic2-r1 10, threat-r1 11, logic-r3 04; UNREVIEWED FIX: architect-r4 02, logic-r4 06) |
| `ReconnectClient` | `clients.proto` comment | Restarts a connected client. A client that is not connected is refused with `failed_precondition`. How the website follows is point 1 for the owner. |
| `CreateClientTokenResponse.token` | `clients.proto` comment | From a Go server, the text of a `JoinToken`. The Server Machines team writes it, and the website and the installer only pass it on. A sample server may return other text, because no machine joins it. (logic2-r1 7, architect-r2 10) |
| `Lgtm-Request-Key` on `CreateClientToken` | `clients.proto` comment | Refused with `invalid_argument` before any token is made, as the Server Sign-in plan already refuses it for `SignIn`. (architect-r2 12, logic-r2 12, threat-r2 11) |

**The description format that the tests compare**

The tests of items 3 to 6 compare the generated descriptors with files in `src/contracts/testdata/golden/`.
The test writer of item 3 writes the helper `describe` in `src/contracts/describe_test.go`, with this behaviour:

1. `describe(t *testing.T, golden string) (got, want string)` reads the file `golden`. Its whole content is `want`.
2. Each line of `want` that starts with `rpc `, `message ` or `enum ` names one descriptor by its full name, which is the line's second word.
3. For each such name, in order, it looks the descriptor up with `protoregistry.GlobalFiles.FindDescriptorByName`. A name it cannot find adds the line `missing <name>` to `got`. It never stops the test.
4. A method adds one line: `rpc <full name> <input> <output>`, where each of `<input>` and `<output>` is the message's full name, preceded by `stream ` when that side streams.
5. A message adds the line `message <full name>`, then one line for each field in declaration order: two spaces, the number, the name, and the type. The type is the full name of a message or enum, or the scalar kind as `protoreflect.Kind.String()` writes it, such as `uint64`. Then, in this order and each preceded by a space: `repeated` for a list, `optional` for a field with the keyword `optional`, and `oneof=<name>` for a field of a oneof that is not synthetic.
6. An enum adds the line `enum <full name>`, then one line for each value: two spaces, the number and the name.
7. After its fields or values, a message or an enum adds one line for each reserved range, `  reserved <start>-<end>` with both ends included, and one line for each reserved name, `  reserved <name>`. Only `commands.txt` has such lines, for the removed `CommandResult.message`, so a builder who adds another reserved range or name fails the test. A field line without `repeated` or `optional` is a single field. (architect-r1 12)
8. `got` ends with a newline, like each golden file.

A test then fails with both texts when `got` differs from `want`.

**Item 1. The Go module**

The repository gets a Go module at its root, and the two code generators are pinned in it, so every machine generates the same Go.

- **Files:** creates `go.mod` and `go.sum`.
- **Needs:** nothing.
- **Work area and complexity:** `contracts`, low. It is one file and a mechanical change with exact check commands.
- **Scope:** `contract/contracts`.
- **Statements:** D2, and the design's section "Where the code lives" (the module path and Go 1.26).
- **Risky:** no.
- **Content of `go.mod`:** the line `module github.com/qnlbnsl/lgtm-studio`, the line `go 1.26.0`, a `tool` block with `connectrpc.com/connect/cmd/protoc-gen-connect-go` and `google.golang.org/protobuf/cmd/protoc-gen-go`, and requirements on `connectrpc.com/connect v1.21.1` and `google.golang.org/protobuf v1.36.12`. It is made with `go mod init github.com/qnlbnsl/lgtm-studio`, the `go` line set to `1.26.0`, `go get -tool` of the two generators at those versions, and `go mod tidy` last, which adds the hashes that the generators' own tests need to `go.sum`.

**Tests written first.** These commands run at the repository root. Each fails on the starting commit, because no `go.mod` exists.

| Command | Expected output |
|---|---|
| `go list -m` | `github.com/qnlbnsl/lgtm-studio` |
| `go version` | A line that starts with `go version go1.26.` |
| `go tool protoc-gen-go --version` | `protoc-gen-go v1.36.12` |
| `go tool protoc-gen-connect-go --version` | `1.21.1` |
| `go mod verify` | `all modules verified` |
| `go build ./...` | Exit status 0, with the warning that `./...` matched no packages. |
| `go mod tidy -diff` | Exit status 0. |

**Acceptance:** every command above gives its expected output.

**Item 2. Go generation**

The contract generates Go beside the TypeScript, and its check fails when either is out of date.

- **Files:** changes `src/contracts/buf.gen.yaml`, `src/contracts/package.json`, and the four files `src/contracts/proto/lgtm/v1/clients.proto`, `map.proto`, `preview.proto` and `session.proto`, which each gain only `option go_package`, after their imports. It regenerates `src/contracts/gen/`. It runs `go mod tidy`, which may change `go.mod` and `go.sum`.
- **Needs:** item 1.
- **Work area and complexity:** `contracts`, medium. It touches a few files on a known pattern, and the layout was tried on 2026-11-26.
- **Scope:** `contract/contracts`.
- **Statements:** D1, D2, and the design's section "Where the code lives" (`src/contracts/gen/` holds TypeScript and Go).
- **Risky:** no.
- **`buf.gen.yaml`:** as in `draft/buf.gen.yaml`. It keeps `clean: true` and the plugin `protoc-gen-es` with `out: gen`. It adds `local: ["go", "tool", "protoc-gen-go"]` and `local: ["go", "tool", "protoc-gen-connect-go"]`, each with `out: gen/go` and `opt: paths=source_relative`. It does not use managed mode.
- **`go_package`:** `github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1;lgtmv1` in every file of `lgtm/v1`. Items 1 and 2 merged in commit `28208cb4` and item 7 in commit `ac1ff06a`, both on 2026-11-26, so `draft/existing-files.diff` now holds only the change of item 3 to `clients.proto`, against that commit.
- **`package.json`:** `generate` stays `buf generate`. Every command below that calls `buf` runs it through `pnpm --filter ./src/contracts exec`, which puts the pinned `buf` and `protoc-gen-es` of `src/contracts/node_modules/.bin` on the path. (architect-r2 05) `check` becomes `buf lint && buf generate && go test ./... && git diff --exit-code -- gen && test -z "$(git status --porcelain --untracked-files=all -- gen)"`. The export map stays as it is.

**Tests written first.** Each runs at the repository root after item 1. The first fails on the starting commit.
The stale probe, the untracked probe and the guard probe run in a separate checkout of the item's commit, made with `git worktree add` in a new folder from `mktemp -d`, so that no probe can leave a changed or new file in the builder's checkout. (critic-r1 8)
In that checkout the builder first runs `pnpm install --frozen-lockfile`, and afterwards `git worktree remove --force` on the folder. (architect-r2 05)
Each probe first runs the check on the clean checkout, which must exit 0.

| Command | Expected result |
|---|---|
| `test -f src/contracts/gen/go/lgtm/v1/lgtmv1connect/clients.connect.go` | Exit status 0. |
| `pnpm --filter ./src/contracts check` | Exit status 0. |
| Stale probe: add the line `// probe` to the top comment of `src/contracts/proto/lgtm/v1/session.proto`, and run `pnpm --filter ./src/contracts check` | Exit status 1. `git diff --name-only -- src/contracts/gen` lists `session_pb.js`, so the tracked diff is the step that failed. |
| Untracked probe: add the file `src/contracts/proto/lgtm/v1/probe.proto` with the lines `syntax = "proto3";`, `package lgtm.v1;`, `option go_package = "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1;lgtmv1";` and `message Probe {}`, and run `pnpm --filter ./src/contracts check` | Exit status 1. Before it, `buf lint` and `buf generate` in `src/contracts` exit 0, and `git status --porcelain --untracked-files=all -- src/contracts/gen` lists `probe.pb.go`, `probe_pb.js` and `probe_pb.d.ts` as new. (architect-r1 10, critic-r1 8, logic-r1 12) |
| Guard probe: in the same checkout with `probe.proto`, run the check without its last step, `pnpm --filter ./src/contracts exec sh -c 'buf lint && buf generate && go test ./... && git diff --exit-code -- gen'` | Exit status 0. The check therefore fails on the untracked probe only because of its guard. A run on 2026-11-26 in a scratch copy gave 0 for the clean check, 1 for the probe with the guard, and 0 for the probe without it. |
| `go build ./...` and `go mod tidy -diff` | Exit status 0 for each. |
| `git diff -U0 master -- src/contracts/gen/lgtm \| grep '^[-+][^-+]' \| grep -vc 'fileDesc('` | `0`, because only the embedded descriptor lines changed. |
| `pnpm --filter ./src/frontend typecheck`, `pnpm --filter ./src/frontend build` and `pnpm --filter ./src/mock-server typecheck` | Exit status 0 for each. |

**Acceptance:** every command above gives its expected result.
The root `check` script calls neither the contract check nor the Go check. The Contract team's reviewer runs the contract check on the commit of each item from 2 to 8. The Go check `scripts/check-go.sh` exists only from item 7 on, so on items 2 to 6 the reviewer runs its commands directly: `test -z "$(gofmt -l src/contracts)"`, `go vet ./...`, `go test ./...` and `go mod tidy -diff`. From item 7 on, and on the sprint's merge commit, the reviewer and the orchestrator's gate run `scripts/check-go.sh`. (architect-r1 10, architect-r2 05, logic-r2 04)

**Item 3. The shared shape of live lists**

Every `Watch` call gets one written shape, `WatchClients` gains its feed position, and a test holds every later `Watch` call to the shape.

- **Files:** creates `src/contracts/proto/lgtm/v1/common.proto`. Changes `src/contracts/proto/lgtm/v1/clients.proto`: it imports `lgtm/v1/common.proto`, adds `FeedPosition position = 4;` to `WatchClientsResponse` after the oneof, adds `string address = 4;` to `CreateClientTokenResponse`, and replaces the comments of `ReconnectClient`, `CreateClientToken`, `RevokeClientToken`, `Client.hosts_server`, `Client.address`, `CreateClientTokenResponse.token` and `CreateClientTokenResponse.expires_at`. Only comments change in those, so no test sees them, and the description of each new comment is in "Rules that item 3 writes" below. It regenerates `src/contracts/gen/`. The test writer creates `src/contracts/describe_test.go`, `src/contracts/watchshape_test.go`, `src/contracts/common_test.go`, `src/contracts/testdata/golden/common.txt` and `src/contracts/testdata/golden/token-reply.txt`.
- **Needs:** item 2.
- **Work area and complexity:** `contracts`, medium. It adds one file and one field, and the new part is the shape test.
- **Scope:** `contract/contracts`.
- **Statements:** D5, R5, D16 (the comment of `ReconnectClient`), C5 (revoking a used token), C6 (a change reaches the page without a reload), C7 (the projected fields of a machine), R9 (no stored copy of a token), and the design's sections "How every `Watch` call works" and "Safe retries".
- **Risky:** yes, because the position orders changes. The mutation that must turn the tests red: remove `FeedPosition position = 4;` from `WatchClientsResponse` and generate again. A second mutation: change `FeedPosition.value` to `uint32`.
- **Text of the files:** as in `draft/common.proto` and the `clients.proto` part of `draft/existing-files.diff`.
- **Rules that item 3 writes:**
  - The first message of a `Watch` call carries the highest position its records include, or 0 when no change is visible yet. Each later message carries a higher position. A Go server sets the position on every message, and an absent position comes from an older server or the sample server. The Server Store writes the position, and no reader uses it yet. (logic-r1 11)
  - `CreateClientToken` refuses a call that carries `Lgtm-Request-Key` with `invalid_argument`, before any token is made, because the reply holds the token's secret and the server keeps no copy of it. Every other call makes a new token, and a token whose reply was lost ends at its expiry. The Server Machines team, the Server Store's request-key layer and the Server Sign-in plan follow it, and the Sign-in plan already refuses the key in the same way for `SignIn`. (threat-r1 4, architect-r2 12, logic-r2 12, threat-r2 11)
  - `RevokeClientToken` removes a machine that used the token and still waits for approval, and it leaves an approved machine alone, as the sample server already does. (logic-r1 05, threat-r1 6)
  - From a Go server, `CreateClientTokenResponse.token` holds the text of a `JoinToken`, and a sample server may return other text. `expires_at` is 15 minutes after the token was made. `address` is the address inside the token, and the join dialog writes it as the address of the command it shows. (logic2-r1 7, architect-r2 10, logic-r2 10, threat-r2 08)
  - `Client.hosts_server` is set by the server when the machine joined with a token from the server's own command on that machine, and never from anything a client says. The Server Machines team and the Stack team's installer follow it. (critic-r1 9)
  - `Client.address` is the network address that the machine's connection came from, for showing only, and empty when the server cannot tell. (architect-r1 11)
- **Not a comment-only change for the website:** the new meaning of `ReconnectClient` breaks the one use of it on today's Machines page. Sprint 0 builds no server, so nothing breaks in this sprint, and the revision stays at 1 here. Point 1 for the owner decides how the page follows. (logic2-r1 1)

**Tests written first.** The test files are package `contracts_test` and import `_ "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"`, which exists after item 2.

| Test | What it asserts |
|---|---|
| `TestEveryWatchCallHasTheSharedShape` in `watchshape_test.go` | For every method of every service in package `lgtm.v1` whose name starts with `Watch`, except the two in the waiting list: the server streams and the client does not; the reply has a oneof named `change`; field 1 is `all`, of a message type, in `change`; field 2 is `saved`, of a message type, in `change`; field 3 is `removed_id`, a `string`, in `change`; field 4 is `position`, of type `lgtm.v1.FeedPosition`, a single field that is neither repeated nor a map, in no oneof; the reply has no other field; the type of `all` has exactly one field, which is a list of the type of `saved`. It also asserts that `lgtm.v1.ClientService.WatchClients` was among the methods checked. On the starting code it fails, because `WatchClientsResponse` has no field 4. |
| `TestOnlyMapAndPreviewsWaitForTheShape` in `watchshape_test.go` | The waiting list is exactly `lgtm.v1.MapService.WatchMap` and `lgtm.v1.PreviewService.WatchPreviews`. Each exists in the registry. Neither has a oneof named `change`, so the list must shrink when either moves to the shape. |
| `TestCommonMatchesItsDescription` in `common_test.go` | `describe` of `testdata/golden/common.txt` equals the file. On the starting code it fails with the line `missing lgtm.v1.FeedPosition`. |
| `TestTokenReplyMatchesItsDescription` in `common_test.go` | `describe` of `testdata/golden/token-reply.txt` equals the file, whose lines are `message lgtm.v1.CreateClientTokenResponse`, `  1 token_id string`, `  2 token string`, `  3 expires_at google.protobuf.Timestamp` and `  4 address string`. On the starting code it fails, because field 4 is missing. (architect-r2 10) |

The content of `src/contracts/testdata/golden/common.txt`:

```text
message lgtm.v1.FeedPosition
  1 value uint64
```

**Acceptance:**

| Observation | Expected result |
|---|---|
| `go test ./src/contracts/ -run 'Watch\|Common\|TokenReply' -v` | Every named test passes. |
| `pnpm --filter ./src/contracts check` | Exit status 0. |
| `pnpm --filter ./src/frontend typecheck`, `pnpm --filter ./src/frontend build`, `pnpm --filter ./src/mock-server typecheck` | Exit status 0 for each. |
| `grep -c 'export const CONTRACT = 1;' src/mock-server/src/engine.ts src/frontend/src/server/api.ts` | `1` for each file, so the revision stays at 1. |

**Item 4. Commands and events of the link**

The two envelopes of the link are written: commands that go down and must run once, and events that come up and must arrive once and in order.

- **Files:** creates `src/contracts/proto/lgtm/link/v1/events.proto` and `src/contracts/proto/lgtm/link/v1/commands.proto`. It regenerates `src/contracts/gen/`. The test writer creates `src/contracts/envelopes_test.go`, `src/contracts/testdata/golden/events.txt` and `src/contracts/testdata/golden/commands.txt`, and the stub `src/contracts/gen/go/lgtm/link/v1/doc.go` that holds only the line `package linkv1`. Generation deletes the stub, because `clean: true` empties `gen/`.
- **Needs:** item 3, for the helper `describe`.
- **Work area and complexity:** `contracts`, medium. Two files of messages follow the pattern of `clients.proto`, and the descriptions fix every number.
- **Scope:** `contract/contracts`.
- **Statements:** R6, D16 (the command `Restart`), C7, C8, C11, and the design's sections "Commands go down, and none is lost or repeated", "Events come up, and none is lost or applied twice" and "Knowing whether a machine is there".
- **Risky:** yes, because the numbers, attempts and revisions order commands and events. The mutation that must turn the tests red: change `uint32 attempt = 2;` in `Command` to `uint64`. A second mutation: remove `uint64 revision = 3;` from `MachineState`.
- **Text of the files:** as in `draft/events.proto` and `draft/commands.proto`. Their comments state these rules.
- **Rules of commands** (architect-r1 07, critic-r1 6, logic-r1 02, logic-r1 03, logic2-r1 5, threat-r1 9):
  - The server stores a command before it sends it, with attempt 1 and an expiry that never changes. It stores each higher attempt before it sends it, and the attempt never wraps.
  - The server sends a pending command on each new current stream until it holds a final result. With no final result 24 hours after the expiry, the server records `COMMAND_OUTCOME_EXPIRED` itself. `SetState` is the exception: it stays pending until it has a result, whatever its expiry. UNREVIEWED FIX and UNREVIEWED MECHANISM: the reviewers offered a second `SetState` or an ended stream when the first was closed unanswered; keeping it pending makes that state impossible. (architect-r4 04, logic-r4 04)
  - Every outcome except `COMMAND_OUTCOME_UNSPECIFIED` is final, and the server never sends that command again.
  - A command that arrives twice runs once. A machine answers every attempt of a command it started with the result it recorded, also after the expiry, until 24 hours after the expiry, and does not run the command again.
  - The server's clock decides expiry, and `Welcome.server_time` gives it, read after `Proof` verified. A command starts only before its expiry. A machine that has not started a command answers `COMMAND_OUTCOME_EXPIRED` when it cannot establish that it starts before the expiry, which now is what that outcome means. The promise that no command starts late holds while the server's clock runs without a jump after `Welcome.server_time` was read. UNREVIEWED FIX. (architect-r4 05, logic-r4 05)
  - A recorded result comes before every rule of expiry: an attempt that finds one gets it. UNREVIEWED FIX. (architect-r4 05)
  - A machine answers every `SetState` with `COMMAND_OUTCOME_DONE`, whatever its expiry, once it has applied the state or found that its revision is not higher than the one it holds. An approval that reaches the machine late therefore still counts, and its two reports follow on the same stream. UNREVIEWED FIX. (logic-r3 10, architect-r4 04, logic-r4 04)
  - How the machine keeps its receipts, which clock it reads and when it checks an expiry belong to plan `go-s3-client-link`, by ruling 2 of round 3. The two facts above are what the server observes. UNREVIEWED MECHANISM: the result file of this fold asks that plan for a lower bound of the server's time when it drops a receipt and an upper bound when it starts a command, checked at the start of the work. (architect-r2 07, logic-r2 05, threat-r2 07, logic2-r2 7, architect-r3 06, logic-r3 08, logic-r3 09, threat-r3 07)
  - A command still running when a newer attempt arrives sends its result under the newest attempt it received.
  - The server accepts a result only from the current stream of the command's machine, and only for the attempt it sent last. It ignores a copy of a stored result, a result for an older attempt and a result for a later attempt. It stores `COMMAND_OUTCOME_UNSPECIFIED` and an outcome it does not know as `COMMAND_OUTCOME_FAILED`. (logic2-r1 10)
  - `COMMAND_OUTCOME_INTERRUPTED` means that the machine started the command and cannot tell how it ended. It is final, and a new action after it needs a new command.
  - A machine answers a command whose action it does not know with `COMMAND_OUTCOME_UNSUPPORTED`.
  - `ScanHarnesses` is done only after its `HarnessesFound` event is on disk, and it fails when the buffer refused the event. (critic-r1 6)
  - `Restart`: `COMMAND_OUTCOME_DONE` means that the machine accepted the restart and recorded that before it answered. The machine's next stream comes from the new program, which answers a repeat of the command from its record. How the program ends and what status it exits with belong to plans `go-s3-client-link` and `go-s3-stack-client`. (logic2-r1 3, threat-r1 8, logic-r2 11, ruling 2 of round 3)
  - `CommandResult` carries no text. Field 4, `message`, is removed and reserved. The Client Link writes only the id, the attempt and the outcome. The Server Link reads only those, and discards any unknown field. The Server Machines team writes the text that the website shows from the outcome. (threat-r1 11, threat-r2 10, architect-r3 08, logic-r3 12, threat-r3 03)
- **Rules of the standing** (logic-r1 04):
  - A machine applies a `MachineState` only when its `revision` is higher than the one it applied last. The same revision with the same content changes nothing. The same revision with other content is a fault of the server, and the machine keeps what it holds.
  - Revisions start at 1 for each machine id. The machine applies `Welcome.state` before any command of that stream.
  - A machine treats `APPROVAL_UNSPECIFIED` and an approval value it does not know as waiting. A `Welcome` without a state breaks a value rule. (logic2-r1 10, architect-r2 patch note 10)
  - `APPROVAL_REMOVED` is a courtesy that the server may send before it ends the stream, and nothing waits for its delivery.
- **Rules of events.** UNREVIEWED MECHANISM: ruling 1 of the orchestrator in round 2 replaced the mechanism of round 1, because two rounds found blockers in it. The rules are the design's three lines under "Events come up, and none is lost or applied twice", made exact. The server alone decides what was applied. (architect-r2 01, architect-r2 02, logic-r2 01, logic-r2 02, logic2-r2 1, logic2-r2 2, threat-r2 01, threat-r2 05)
  - A machine numbers its events from 1, with no gaps, and never reuses a number.
  - The server keeps the last event number it applied for each machine. It writes that number in the same transaction as the event's effect, and sends `EventAck` with it only after the commit.
  - `Welcome.last_event_number` carries that number. The machine forgets its events at or below it, and sends the rest from the next one, in order. A machine never changes the number of an event, so two reports never share a number. UNREVIEWED MECHANISM: logic-r3 01 asked for the three branches of the client-local `Reconcile` in the wire rule. That interface is gone from the client-local plan, whose `Ack` refuses a number it never handed out, and a rule with no renumbering makes the collision impossible. (logic-r3 01, threat-r3 08, architect-r3 05)
  - Only the current stream's events count, and the server ignores an event of another stream.
  - The server decides each event of the current stream by one table, which comes before the value rules of `link.proto`. A message that cannot be decoded, such as one with a string that is not valid UTF-8, ends the stream with `invalid_argument`, and the number stays. For a decoded event, the server reads only the number until the last step. A number of 0 ends the stream with `invalid_argument`. A number at or below the last applied is a repeat: the server applies nothing and sends `EventAck` with the last applied number, whatever the report holds. A number above the next one is a gap: the server ends this stream with `failed_precondition` and `event_gap`, whatever the report holds, and a later stream that sends the missing number goes on. UNREVIEWED FIX. (architect-r3 05, logic-r3 01, threat-r3 08, architect-r4 02, logic-r4 06, logic-r4 07)
  - At the next number, the server applies a valid report. It consumes an event that holds no report, a report it does not know, or a report that breaks a rule of `events.proto`. The number counts as applied, and the report has no effect: it sets no readiness of the projection. (logic-r2 07, logic-r3 04, threat-r3 08, architect-r3 05)
  - One machine id has one outbox. A server number at or above the machine's next number, or below one that the server acknowledged before, comes from a restore or a copy of the server's or the machine's state, which wave 1 does not cover. In the second case the machine's next event is a gap, so the server refuses it with the final code. UNREVIEWED MECHANISM: threat-r3 02 asked to bind acknowledgements to the reports or to forbid copies; the design promises neither, so the case is named outside wave 1, and the result file asks for an issue. (ruling 2 of round 2, threat-r3 02)
  - A machine checks a report against the rules of `events.proto` before the report takes a number.
  - When the machine becomes approved on a stream, by a `Welcome` whose state is approved or by a `SetState` that approves it, it sends its current `Facts` and `HarnessesFound` as new events. (logic2-r2 3, architect-r2 08, logic-r2 09)
- **Rules of heartbeats:**
  - A heartbeat is never numbered or kept. The machine sends one every 10 seconds. The server answers each heartbeat of the current stream with `HeartbeatAck`.
  - The server marks the machine offline and ends the stream after 30 seconds without a heartbeat on the current stream, measured by its monotonic clock. (logic-r1 06)
  - Both load fields are optional and run from 0 to 100. Absent means that the machine could not measure it, and a heartbeat without load still shows that the machine is there. (logic-r1 10)
- **Rules of facts:** the server shows `Facts` and trusts nothing in them. Strings keep the limits of their comments. The website's `memory_gigabytes` is `memory_bytes` divided by 2^30 and rounded to the nearest whole number. The server counts the harnesses as scanned once it holds one `HarnessesFound`. (architect-r1 11, threat-r1 11, logic2-r1 11)
- **What the website shows of a machine**, one projection that the Server Machines team writes (architect-r2 08, logic-r2 09, threat-r2 06):
  - Waiting, while its standing is waiting, whether or not it has a stream.
  - Offline, while it is approved and has no current stream.
  - Joining, while it is approved, has a current stream, and the server has not yet applied both a valid `Facts` and a valid `HarnessesFound` from that stream while the machine was approved. A new stream starts at joining. (architect-r3 07)
  - Online, while it is approved, has a current stream, and the server applied both from that stream. The four cases are disjoint. UNREVIEWED FIX. (architect-r4 07)
- Cases of `Command.action` and `Event.report` are numbered from 10, so the low numbers stay free for fields of the envelope.

**Tests written first.** The test file imports `_ "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1"`. It reaches generated types only through `protoregistry.GlobalTypes.FindMessageByName` and `protoreflect`, never by a Go type name, so it compiles with the stub in place. Every test then fails on an assertion: the description tests on their `missing` lines, and the other two because `FindMessageByName` finds no message, which the test reports with `t.Fatalf`.

| Test | What it asserts |
|---|---|
| `TestEventsMatchItsDescription` | `describe` of `testdata/golden/events.txt` equals the file. |
| `TestCommandsMatchItsDescription` | `describe` of `testdata/golden/commands.txt` equals the file. |
| `TestAnUnknownActionReadsAsNoAction` | Bytes built with `protowire` that hold field 1 of `Command` as the string `c1`, field 2 as the number 3, and an empty field 99 unmarshal into a new message of the type `lgtm.link.v1.Command` with field `id` equal to `c1`, field `attempt` equal to 3, and no field of the oneof `action` set (`WhichOneof` returns nil). This is how a machine recognizes a command from a newer server. |
| `TestAnUnknownReportReadsAsNoReport` | Bytes that hold field 1 of `Event` as the number 7 and an empty field 99 unmarshal into a new message of the type `lgtm.link.v1.Event` with field `number` equal to 7 and no field of the oneof `report` set. |

The content of `src/contracts/testdata/golden/events.txt`:

```text
message lgtm.link.v1.Event
  1 number uint64
  2 happened_at google.protobuf.Timestamp
  10 facts lgtm.link.v1.Facts oneof=report
  11 harnesses_found lgtm.link.v1.HarnessesFound oneof=report
message lgtm.link.v1.EventAck
  1 through uint64
message lgtm.link.v1.Facts
  1 os string
  2 cores uint32
  3 memory_bytes uint64
  4 client_version string
  5 workspace string
  6 agent_limit uint32
message lgtm.link.v1.HarnessesFound
  1 harnesses lgtm.link.v1.Harness repeated
message lgtm.link.v1.Harness
  1 name string
  2 version string optional
message lgtm.link.v1.Heartbeat
  1 cpu_percent uint32 optional
  2 memory_percent uint32 optional
message lgtm.link.v1.HeartbeatAck
```

The content of `src/contracts/testdata/golden/commands.txt`:

```text
message lgtm.link.v1.Command
  1 id string
  2 attempt uint32
  3 expires_at google.protobuf.Timestamp
  10 scan_harnesses lgtm.link.v1.ScanHarnesses oneof=action
  11 set_state lgtm.link.v1.SetState oneof=action
  12 restart lgtm.link.v1.Restart oneof=action
message lgtm.link.v1.CommandResult
  1 command_id string
  2 attempt uint32
  3 outcome lgtm.link.v1.CommandOutcome
  reserved 4-4
  reserved message
message lgtm.link.v1.ScanHarnesses
message lgtm.link.v1.Restart
message lgtm.link.v1.SetState
  1 state lgtm.link.v1.MachineState
message lgtm.link.v1.MachineState
  1 approval lgtm.link.v1.Approval
  2 name string
  3 revision uint64
enum lgtm.link.v1.CommandOutcome
  0 COMMAND_OUTCOME_UNSPECIFIED
  1 COMMAND_OUTCOME_DONE
  2 COMMAND_OUTCOME_FAILED
  3 COMMAND_OUTCOME_EXPIRED
  4 COMMAND_OUTCOME_UNSUPPORTED
  5 COMMAND_OUTCOME_INTERRUPTED
enum lgtm.link.v1.Approval
  0 APPROVAL_UNSPECIFIED
  1 APPROVAL_WAITING
  2 APPROVAL_APPROVED
  3 APPROVAL_REMOVED
```

**Acceptance:** `go test ./src/contracts/ -run 'Events|Commands|Unknown' -v` passes every named test, `test ! -e src/contracts/gen/go/lgtm/link/v1/doc.go` exits 0, and `pnpm --filter ./src/contracts check` exits 0.

**Item 5. Joining**

The call by which a new machine trades a join token for the server's key is written, together with the token's text.

- **Files:** creates `src/contracts/proto/lgtm/link/v1/link.proto` with the service `LinkService` holding only `Join`, and the messages `JoinToken`, `JoinRequest` and `JoinResponse`. Creates `src/contracts/testdata/join-token.json`. It regenerates `src/contracts/gen/`. The test writer creates `src/contracts/join_test.go` and `src/contracts/testdata/golden/join.txt`.
- **Needs:** item 4.
- **Work area and complexity:** `contracts`, medium. It is one file of messages, and a worked example pins the token's bytes.
- **Scope:** `contract/contracts`.
- **Statements:** D7, D17, C5, C10, R8, R9, and the design's section "Joining: a token that authorizes an exchange of keys".
- **Risky:** yes, because it carries tokens and keys. The mutation that must turn the tests red: renumber `server_key_sha256` in `JoinToken` from 4 to 6. A second mutation: write the example's text with the prefix `lgtm_` instead of `lgtm1_`.
- **Text of the file:** as the `Join` part of `draft/link.proto`: the file header, the package, the `go_package` option `github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/link/v1;linkv1`, the service with `Join` only, and the three messages with their comments. The comments state these rules:
  - The server checks the token's secret first. An unknown token id and a secret that does not match the token's verifier get the same refusal, `token_unknown`, so a caller without the secret learns nothing of the token's state. (architect-r3 01, logic-r3 11, threat-r3 05)
  - The server then refuses with `permission_denied`, in this order, against its state at the moment it decides: `token_expired` when the expiry has passed by the server's clock, `token_revoked`, `token_used` for a token bound to another key, and `machine_removed` when the machine that used the token with this key was removed. The reason is under the key `Lgtm-Join-Refusal` in the error's metadata, and the error's message is a sentence for people. These are the words and the key that the Server Machines plan already writes. (logic-r2 08, architect-r3 03, threat-r3 05; UNREVIEWED FIX: architect-r4 03, logic-r4 07)
  - A token that passes those checks and is not used yet is claimed. In the same transaction the server binds the token to the machine's key and creates the waiting machine. Of two first uses at once, exactly one wins. The loser is decided again by the same order, against the state at that moment: with the same key it is a replay, and with another key it gets `token_used`, unless an earlier refusal such as `token_expired` applies by then. UNREVIEWED FIX. (logic-r4 08) A failure before the commit changes nothing and returns `unavailable`. (architect-r1 06, critic-r1 3, logic-r1 05, threat-r1 6, logic2-r1 8, logic-r2 08, logic-r3 11)
  - A token that passes those checks and is used by the same key is a replay. It returns the first reply and creates nothing, and the host name of a replay is not read. This replay is the one exception to "used once" in check C5.
  - Revoking a used token removes its machine while the machine waits, and leaves an approved machine alone. How the server orders a revocation against an approval belongs to plan `go-s3-server-machines`, by ruling 2 of round 3. (logic-r3 11)
  - The token's text is a secret. No log, board or page other than the one that made it holds it.
  - `JoinToken.address` is the address at which the server answers the link: `https`, or `http` for a loopback address only, with a host and an optional port and no path. Every `https` token carries `certificate_key_sha256`, the SHA-256 of the certificate key the server presents there. A loopback token over `http` has none. (architect-r1 05, critic-r1 4, threat-r1 10)
  - The SHA-256 of `JoinResponse.server_public_key` equals the token's `server_key_sha256`, and the server signs every `Challenge` with that key.
  - `JoinRequest.host_name` is at most 255 bytes, and the server shows it as the machine's own claim. (threat-r1 11)
  - Which address the server puts in a token, how a client checks the address, the pin and the server's key, how often it retries `Join` and which exit status it gives belong to plans `go-s3-server-machines`, `go-s3-client-link`, `go-s3-stack-client` and `go-s2-stack-web`, by ruling 2 of round 3. The result file of this fold lists what each plan receives. UNREVIEWED MECHANISM: the reviewers asked the contract to settle those rules. (architect-r2 09, architect-r2 11, threat-r2 09, architect-r3 02, architect-r3 03, logic-r3 06, logic-r3 07, threat-r3 01, threat-r3 04)
- **`testdata/join-token.json`:** one object with these fields, bytes in lowercase hexadecimal:

| Field | Value |
|---|---|
| `token_id` | `0193a2b4-0000-7000-8000-000000000001` |
| `secret` | 32 bytes of `33` |
| `address` | `http://127.0.0.1:8787` |
| `server_public_key` | `8a88e3dd7409f195fd52db2d3cba5d72ca6709bf1d94121bf3748801b40f6f5c`, the Ed25519 public key of the seed of 32 bytes of `01` |
| `server_key_sha256` | `34750f98bd59fcfc946da45aaabe933be154a4b5094e1c4abf42866505f3c97e` |
| `certificate_key_sha256` | empty |
| `text` | `lgtm1_CiQwMTkzYTJiNC0wMDAwLTcwMDAtODAwMC0wMDAwMDAwMDAwMDESIDMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzMzGhVodHRwOi8vMTI3LjAuMC4xOjg3ODciIDR1D5i9Wfz8lG2kWqq-kzvhVKS1CU4cSr9ChmUF88l-` |

**Tests written first.**

| Test | What it asserts |
|---|---|
| `TestJoinMatchesItsDescription` | `describe` of `testdata/golden/join.txt` equals the file. |
| `TestJoinTokenExampleDecodes` | `testdata/join-token.json` exists and parses. Its `text` starts with `lgtm1_`. The rest decodes as unpadded base64url and unmarshals into a new message of the type `lgtm.link.v1.JoinToken`, which the test finds with `protoregistry.GlobalTypes.FindMessageByName`. The test names no generated Go type, so it compiles before the type exists. Each field of the message equals the field of the same name in the file, and each value equals the table above, which the test holds as constants. The hash SHA-256 (Secure Hash Algorithm, 256 bits) of `server_public_key` equals `server_key_sha256`. Marshalling the message again with `proto.MarshalOptions{Deterministic: true}` gives the same bytes. Before the file exists, the test fails on its first assertion. |

The content of `src/contracts/testdata/golden/join.txt`:

```text
rpc lgtm.link.v1.LinkService.Join lgtm.link.v1.JoinRequest lgtm.link.v1.JoinResponse
message lgtm.link.v1.JoinToken
  1 token_id string
  2 secret bytes
  3 address string
  4 server_key_sha256 bytes
  5 certificate_key_sha256 bytes
message lgtm.link.v1.JoinRequest
  1 token_id string
  2 token_secret bytes
  3 machine_public_key bytes
  4 host_name string
message lgtm.link.v1.JoinResponse
  1 machine_id string
  2 server_public_key bytes
```

**Acceptance:** `go test ./src/contracts/ -run 'Join' -v` passes both tests, and `pnpm --filter ./src/contracts check` exits 0.

**Item 6. The stream and its proof**

The one stream that a machine keeps open is written: who speaks first, the signed challenge on both sides, and what may follow.

- **Files:** changes `src/contracts/proto/lgtm/link/v1/link.proto`: it imports `commands.proto` and `events.proto`, adds `Open` to `LinkService`, and adds `OpenRequest`, `OpenResponse`, `Hello`, `Challenge`, `Proof` and `Welcome`. Creates `src/contracts/testdata/link-proof.json`. It regenerates `src/contracts/gen/`. The test writer creates `src/contracts/open_test.go` and `src/contracts/testdata/golden/open.txt`.
- **Needs:** item 5.
- **Work area and complexity:** `contracts`, medium. It adds messages to one file, and worked examples pin the signed bytes.
- **Scope:** `contract/contracts`.
- **Statements:** D7, D9 and D15 (room for the calls of wave 2), D17, R8, R10, C8, C9, C10, C11, and the design's sections "One connection, dialled by the machine" and "Both ends prove themselves on every connection".
- **Risky:** yes, because it carries the proof of both ends and the order of the stream. The mutation that must turn the tests red: in `testdata/link-proof.json`, compute `server_text` and `server_signature` with the server nonce before the machine nonce. A second mutation: remove `last_event_number` from `Welcome`. A third mutation: delete the comment line above `Restart restart = 12;` in `commands.proto`, which must make `TestEveryLinkRuleIsWritten` fail. (logic2-r1 12) A fourth mutation: replace the two comment lines above `COMMAND_OUTCOME_INTERRUPTED = 5;` by the line `//`, which must make the same test fail. (logic2-r2 8)
- **Text of the file:** as `draft/link.proto`. The comments state these rules.
- **The handshake** (logic-r1 09, critic-r1 10, architect-r1 09):
  - The machine sends `Hello` and then `Proof`. The server sends `Challenge` and then `Welcome`.
  - The server answers every well-formed `Hello` with a `Challenge` before it looks the machine up or reads its link revision. Then it decides in one order, and may do so before `Proof` arrives: `unavailable` when it cannot look the machine up; `failed_precondition` with `unknown_machine` for a machine id it does not know; `failed_precondition` with `link_revision_mismatch` for a revision it does not speak; `deadline_exceeded` when `Proof` is not there 10 seconds after `Challenge`; `invalid_argument` when `Proof` breaks a value rule; and `unauthenticated` when the machine's signature does not verify. UNREVIEWED FIX. (architect-r4 06) UNREVIEWED MECHANISM: ruling 2 of the orchestrator in round 2 asks for one named code for a machine the server does not know and for numbers that no longer fit the server's. `failed_precondition` is that code. (architect-r2 06, logic-r2 06, logic2-r2 5, threat-r2 04)
  - The link revision is read after the `Challenge`, so an unsupported revision is `failed_precondition` with the reason `link_revision_mismatch`, and never `invalid_argument`. (architect-r2 patch note, logic-r2 06, logic-r3 03)
  - Before `Welcome`, a message of another kind, an empty message or a message that breaks a value rule ends the stream with `invalid_argument`.
  - The server ends the stream with `deadline_exceeded` when `Hello` is not there 10 seconds after the stream opened. The time for `Proof` is measured apart, from `Challenge`.
  - The proof texts, as in the table of public names.
- **The value rules** in the file comment of `link.proto`: canonical UUIDs, exact lengths of nonces, keys, signatures and hashes, counters from 1, the fields that must be present, and the limits of strings. A message that breaks one is refused with `invalid_argument`. After `Welcome`, the table of events of item 4 comes before them for every event. (logic-r1 09, critic-r1 10, logic2-r1 10, threat-r1 11, logic-r3 04; UNREVIEWED FIX: architect-r4 02, logic-r4 06)
- **The codes**, in the table of the `LinkService` comment:
  - `permission_denied` and `invalid_argument` from `Join` are final. `failed_precondition` from `Open` is final when it follows a `Challenge` whose signature verified, and the key `Lgtm-Link-Refusal` in the metadata of the terminal error holds `unknown_machine`, `event_gap` or `link_revision_mismatch`. A final code depends only on the request and on the server's state when the server decides it: the same request against the same state gets the same answer. A gap ends only the stream on which it occurs. (architect-r3 04; UNREVIEWED FIX: architect-r4 03, architect-r4 06, logic-r3 03, logic-r4 07)
  - `unauthenticated` is a bad signature, `aborted` a stream that a newer one replaced, `deadline_exceeded` a late `Hello` or `Proof`, and `unavailable` a server that cannot read its database. A server never answers a final code because it cannot read its database.
  - What a client does on each code, how long it waits before it dials again, how long it waits for a `Challenge` or a `Welcome`, and which exit status it gives belong to plan `go-s3-client-link`, by ruling 2 of round 3. The 60-second wait after a second `aborted` is gone, so the design's bound of 15 seconds between tries holds. UNREVIEWED MECHANISM: the reviewers asked the contract to fix the exit statuses and the waits. (architect-r2 06, logic-r2 06, logic-r2 11, logic2-r2 6, architect-r3 03, architect-r3 11, logic-r3 06, threat-r3 01, threat-r3 11)
- **After `Welcome`:**
  - Either end may send any of its other messages, and events keep the order of item 4.
  - A receiver ignores a message of a kind it does not know. Wave 2 adds new kinds with numbers not used yet, and no number is reserved.
  - The server answers each heartbeat of the current stream with `HeartbeatAck`, so a machine can tell a silent stream. (logic2-r1 6, critic-r1 7)
- **The current stream** (architect-r1 09, logic-r1 06, logic2-r1 4, threat-r1 1):
  - A stream becomes its machine's current stream at the moment the server admits it to send `Welcome`, after its `Proof` verified. At that moment the older stream stops being current, even while its end is in transit, and nothing that arrives on it changes the machine. The server reads `Welcome.last_event_number` after that moment, so it holds every event that the older stream committed. A stream stops being current when it ends, when a newer stream becomes current, or when the removal of its machine commits. UNREVIEWED FIX. (logic-r4 01)
  - A newer stream of a machine whose `Proof` verified replaces the older one, which ends with `aborted`.
  - `Welcome.connection_token` ends when its stream ends or is replaced. Later calls beside the stream present it in the header `Lgtm-Link-Token`.
  - How the server orders the replacement of a stream, its removal and the writes of a stream belongs to plan `go-s3-server-machines`, by ruling 3 of round 2 and ruling 2 of round 3. The contract states only what the machine observes. UNREVIEWED MECHANISM: the reviewers asked for a fence in the contract. The request is in the result file of this fold, and post-build check 10 tests it. (architect-r2 03, logic-r2 03, threat-r2 02, logic2-r2 6, architect-r3 04, logic-r3 05, threat-r3 06)
- **A machine that waits for approval:**
  - Its `Welcome` holds an empty connection token. A machine gets a connection token in the `Welcome` of a stream that opens after its approval, which is what the Server Machines plan already does. (threat-r3 10)
  - The server sends it no command except `SetState`, and ignores its command results. (threat-r2 06)
  - It may send heartbeats and events until the owner settles point 3, which may narrow that. (architect-r1 08, threat-r1 7)
- **Removal** (architect-r1 02, logic-r1 04, threat-r1 1, critic-r1 7):
  - The server orders the commit of a removal against the moment it admits a stream. A stream that would be admitted after the commit receives no `Welcome`, and ends with `failed_precondition` and `unknown_machine`. A `Welcome` that the server sent before the commit may still arrive, followed by that refusal. After the commit, its connection token opens nothing, and nothing that arrives on that stream changes the machine. UNREVIEWED FIX: the rule of round 3 forbade the receipt of a `Welcome` that may already be in transit, which no server can keep. (logic-r3 05, architect-r3 04, architect-r4 01, logic-r4 02)
  - The server ends the machine's current stream with `failed_precondition` and the reason `unknown_machine`, and with it the connection token.
  - Every later `Open` of that machine gets a `Challenge` and then the same code and reason.
  - The server may send `SetState` with `APPROVAL_REMOVED` before it ends the stream, and nothing waits for that message.
  - A restore of the server's database that brings back a removed machine is outside wave 1, by ruling 2 of round 2. (threat-r2 03)
- **Event marks:** as the rules of events in item 4. `Hello` carries no event number. (architect-r2 01, logic2-r2 1, threat-r2 01)
- **`testdata/link-proof.json`:** one object with these fields, bytes in lowercase hexadecimal:

| Field | Value |
|---|---|
| `server_seed` | 32 bytes of `01` |
| `server_public_key` | `8a88e3dd7409f195fd52db2d3cba5d72ca6709bf1d94121bf3748801b40f6f5c` |
| `machine_seed` | 32 bytes of `02` |
| `machine_public_key` | `8139770ea87d175f56a35466c34c7ecccb8d8a91b4ee37a25df60f5b8fc9b394` |
| `machine_id` | `0193a2b4-5c6d-7e8f-9a0b-1c2d3e4f5a6b` |
| `machine_nonce` | 32 bytes of `11` |
| `server_nonce` | 32 bytes of `22` |
| `server_text` | `6c67746d2d6c696e6b2d763120736572766572` (the label), `00`, `24` (36, the length of the id), the id in UTF-8, then the machine nonce and the server nonce |
| `server_signature` | `f754334e7dd8404dca1420b2a5a17113b431cb0160e57ed2ff5665d0054474b265a01059a52647d78a4fb4d19bc29570daefe451c0344ae17aecd07a21a4b60a` |
| `machine_text` | `6c67746d2d6c696e6b2d7631206d616368696e65` (the label), `00`, `24`, the id in UTF-8, then the machine nonce and the server nonce |
| `machine_signature` | `977cea1412c854aa10e7408112dcf89eca4a508f2ea37714032f5c98bf343b0fffd5904eba1d5e89bb9b139d54daf983d76206000ee48b457e12a41cc7cc500e` |

**Tests written first.**

| Test | What it asserts |
|---|---|
| `TestOpenMatchesItsDescription` | `describe` of `testdata/golden/open.txt` equals the file. |
| `TestEveryLinkNameIsDescribed` | Every message, enum and method in package `lgtm.link.v1` is named by a header line in exactly one of `events.txt`, `commands.txt`, `join.txt` and `open.txt`. A builder therefore cannot add a message that no description fixes. |
| `TestEveryLinkRuleIsWritten` | It reads `proto/lgtm/link/v1/link.proto`, `commands.proto` and `events.proto` as text, because the generated Go keeps no comments. Every line that declares a service, method, message, enum, oneof, field or enum value has a comment block above it, which is the run of lines starting with `//` since the last other line that is not empty. That block holds at least three words of two letters or more, so a bare `//` does not count. (logic2-r2 8) The one exception is an enum value named `..._UNSPECIFIED = 0`. `link.proto` names each of `permission_denied`, `unauthenticated`, `failed_precondition`, `invalid_argument`, `deadline_exceeded`, `aborted`, `unavailable`, `Lgtm-Link-Token`, `Lgtm-Join-Refusal` and `Lgtm-Link-Refusal`, and `events.proto` names `failed_precondition`. (architect-r3 03, architect-r3 04) Before the files exist, the test fails on the first file it cannot read. A prototype passed on the drafts on 2026-11-26, and failed with `commands.proto:42 has no comment above it` when the comment above `Restart restart = 12;` was deleted. (logic2-r1 12) A second prototype with the three-word rule passed on the drafts of round 2, and failed on `COMMAND_OUTCOME_INTERRUPTED` when its comment was replaced by `//`. The test proves that a comment is there, not that it is right. (logic-r2 patch 12) |
| `TestProofExamplesHoldTogether` | `testdata/link-proof.json` exists and parses. The Ed25519 keys made from the two seeds equal the two public keys. The test builds both texts itself from the rule (label, zero byte, length byte, id, machine nonce, server nonce) and each equals the file's text. Both signatures equal the values in the table above, which the test holds as constants, and each verifies with `ed25519.Verify` against its own text and key. The server's signature does not verify against the machine's text with the server's key, and the machine's signature does not verify against the server's text with the machine's key. Before the file exists, the test fails on its first assertion. |

The content of `src/contracts/testdata/golden/open.txt`:

```text
rpc lgtm.link.v1.LinkService.Open stream lgtm.link.v1.OpenRequest stream lgtm.link.v1.OpenResponse
message lgtm.link.v1.OpenRequest
  1 hello lgtm.link.v1.Hello oneof=kind
  2 proof lgtm.link.v1.Proof oneof=kind
  3 heartbeat lgtm.link.v1.Heartbeat oneof=kind
  4 event lgtm.link.v1.Event oneof=kind
  5 command_result lgtm.link.v1.CommandResult oneof=kind
message lgtm.link.v1.OpenResponse
  1 challenge lgtm.link.v1.Challenge oneof=kind
  2 welcome lgtm.link.v1.Welcome oneof=kind
  3 heartbeat_ack lgtm.link.v1.HeartbeatAck oneof=kind
  4 event_ack lgtm.link.v1.EventAck oneof=kind
  5 command lgtm.link.v1.Command oneof=kind
message lgtm.link.v1.Hello
  1 machine_id string
  2 machine_nonce bytes
  3 link_revision uint32
message lgtm.link.v1.Challenge
  1 server_nonce bytes
  2 server_signature bytes
message lgtm.link.v1.Proof
  1 machine_signature bytes
message lgtm.link.v1.Welcome
  1 connection_token string
  2 state lgtm.link.v1.MachineState
  3 last_event_number uint64
  4 server_time google.protobuf.Timestamp
  5 link_revision uint32
```

**Acceptance:** `go test ./src/contracts/ -v` passes every test of items 3 to 6, `go build ./...` exits 0, and `pnpm --filter ./src/contracts check` exits 0.

**Item 7. The Go check command**

One command checks all Go code of the repository the same way on every machine.

- **Files:** creates `scripts/check-go.sh`, which is executable.
- **Needs:** item 2, because `go vet ./...` and `go test ./...` exit 1 while the module holds no package. It runs beside items 3 to 6.
- **Work area and complexity:** `infra`, low. It is one shell script of five steps.
- **Scope:** `contract/contracts`.
- **Statements:** C12 (`go test ./...` passes), R10 (the script passes `LGTM_TEST_POSTGRES` on as it finds it).
- **Risky:** no.
- **What it does, in order, from the repository root, stopping at the first failure:** it lists the Go files with `git ls-files --cached --others --exclude-standard -- '*.go'`; it fails and names each file that `gofmt -l` lists; it runs `go mod tidy -diff`; it runs `go vet ./...`; it runs `go test ./...`. It starts with `set -euo pipefail`, and it has a header comment that says what it checks.

**Tests written first.** Each runs at the repository root after item 2. The first fails before this item, because the script does not exist.

| Command | Expected result |
|---|---|
| `scripts/check-go.sh` | Exit status 0. |
| Write `package contracts_test\nfunc  x() {}\n` to `src/contracts/zz_probe_test.go`, run `scripts/check-go.sh`, then delete the file | Exit status 1, and the output names `src/contracts/zz_probe_test.go`. |
| Add the line `require golang.org/x/text v0.20.0` to `go.mod`, run `scripts/check-go.sh`, then undo the line | Exit status 1. |

**Acceptance:** every command above gives its expected result, on the branch of this item and again after item 6 merges.

**Item 8. The contract's README**

The README of the contract names the new files and the rules that come with them.

- **Files:** changes `src/contracts/README.md`.
- **Needs:** items 6 and 7.
- **Work area and complexity:** `contracts`, low. It is one document checked against files that exist by then.
- **Scope:** `contract/contracts`.
- **Statements:** the design's section "What must change outside this document before builders start", third point.
- **Risky:** no.
- **What changes:** the table of files gains `common.proto` and a second table for the package `lgtm.link.v1` with `link.proto`, `commands.proto` and `events.proto`. "What is here" names `gen/go/` and `testdata/`. The rules name the shared `Watch` shape, the feed position, the link revision, the header `Lgtm-Link-Token`, and that generating needs Go 1.26 or newer. The section "Not here yet" no longer says that the calls between the server and the client are not written. The sentence that sends a reader to `SECURITY.md`, which the owner removed on 2026-11-25, is removed. The rule that every call needs a session becomes a rule of the package `lgtm.v1`, and a new rule says that `LinkService.Join` needs a join token and `LinkService.Open` the machine's key, and that neither takes a website session. The rule "Answer a call without a session with the code `unauthenticated`" also becomes a rule of the package `lgtm.v1`, and the exceptions `SessionService.GetServer` and `SessionService.SignIn` stay. (critic-r1 11, architect-r2 patch note 11, logic-r2 patch 11)

**Tests written first.** Each absence check succeeds when the text is absent, because `grep -q` exits 1 when it finds nothing and `!` turns that into 0. (critic-r1 11, logic-r2 patch 11)

| Command | Expected result |
|---|---|
| `test -f src/contracts/README.md` | Exit status 0. |
| `grep -q 'common.proto' src/contracts/README.md`, and the same for `link.proto`, `commands.proto`, `events.proto`, `gen/go`, `Lgtm-Link-Token`, `Go 1.26`, `link revision 1` and `LinkService` | Exit status 0 for each. |
| `! grep -q 'The calls between the server and the client are not written' src/contracts/README.md` | Exit status 0. |
| `! grep -q 'SECURITY.md' src/contracts/README.md` | Exit status 0. |
| `! grep -q '^- Every call needs a session' src/contracts/README.md` | Exit status 0, because the session rule now names the package `lgtm.v1`. |
| `! grep -q '^- Answer a call without a session' src/contracts/README.md` | Exit status 0, because that rule now names the package `lgtm.v1` too. |
| `grep -q 'SessionService.GetServer' src/contracts/README.md` and `grep -q 'SessionService.SignIn' src/contracts/README.md` | Exit status 0 for each, because the exceptions stay. |
| `node scripts/check-private.mjs` | Exit status 0. |

**Acceptance:** every command above gives its expected result.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1 | `go build ./...` passes at the repository root. | `go build ./...` exits 0. |
| 2 | The contract check passes, and it fails when the generated Go is out of date or was never committed. | `pnpm --filter ./src/contracts check` exits 0; the stale probe and the untracked probe of item 2 make it exit 1, and the guard probe exits 0. (critic-r1 8) |
| 3 | The website still builds and signs in against the sample server. | `pnpm --filter ./src/frontend typecheck` and `pnpm --filter ./src/frontend build` exit 0. With `pnpm --filter ./src/mock-server start` running, `curl -s -X POST -H 'content-type: application/json' -d '{}' http://127.0.0.1:4180/lgtm.v1.SessionService/GetServer` prints JSON (JavaScript Object Notation) with `"contract":1`, and `curl -s -X POST -H 'content-type: application/json' -d '{"password":"sample"}' http://127.0.0.1:4180/lgtm.v1.SessionService/SignIn` prints `{"result":"SIGN_IN_RESULT_SIGNED_IN"}`. A person then runs `pnpm --filter ./src/frontend dev`, opens `https://127.0.0.1:4179/`, types `/` as the server's address, signs in with any password except `wrong`, and sees the Machines page list the sample machines. |
| 4 | Every `Watch` call that the shape covers has it. | `TestEveryWatchCallHasTheSharedShape` and `TestOnlyMapAndPreviewsWaitForTheShape` pass. |
| 5 | The contract revision stays at 1 in sprint 0. | The `grep` of item 3 prints `1` for each of the two files. Whether it becomes 2 in sprint 3 is point 1 for the owner. |
| 6 | Every name and number of the link is fixed. | `TestEventsMatchItsDescription`, `TestCommandsMatchItsDescription`, `TestJoinMatchesItsDescription`, `TestOpenMatchesItsDescription` and `TestEveryLinkNameIsDescribed` pass. `TestEveryLinkRuleIsWritten` passes, so every declaration has a comment of at least three words and the link names its codes. Whether each comment is right is the reviewer's reading, not the test's. (logic-r2 patch 12) |
| 7 | Both teams can test the proof and the token against the same bytes. | `TestProofExamplesHoldTogether` and `TestJoinTokenExampleDecodes` pass. |
| 8 | One command checks all Go code. | `scripts/check-go.sh` exits 0 after item 6. Items 2 to 6 ran its commands directly, because the script arrives with item 7. (architect-r2 05, logic-r2 04) |
| 9 | The README names the new files. | The commands of item 8 give their expected results. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 1 | Create the module and pin the generators. | Target `go.mod`; pattern: the scratch module in `scouting.md`, run 2. |
| 2 | Add the Go plugins, keeping `clean: true`. | `src/contracts/buf.gen.yaml`, `plugins`; target `draft/buf.gen.yaml`. |
| 3 | Add `option go_package` to the four existing files. | `src/contracts/proto/lgtm/v1/session.proto`, `package lgtm.v1`; the same in `clients.proto`, `map.proto` and `preview.proto`. |
| 4 | Make the check run the Go tests and refuse untracked generated files. | `src/contracts/package.json`, script `check`. |
| 5 | Write the shared shape and the feed position. | Target `src/contracts/proto/lgtm/v1/common.proto`; pattern `src/contracts/proto/lgtm/v1/session.proto`. |
| 6 | Add the position to `WatchClientsResponse`. | `src/contracts/proto/lgtm/v1/clients.proto`, `message WatchClientsResponse`, `oneof change`. |
| 7 | Rewrite the comment of `ReconnectClient`. | `src/contracts/proto/lgtm/v1/clients.proto`, `rpc ReconnectClient`. |
| 8 | Write the two envelopes. | Targets `src/contracts/proto/lgtm/link/v1/events.proto` and `commands.proto`; pattern `src/contracts/proto/lgtm/v1/clients.proto`, `enum ClientState`. |
| 9 | Write joining and its example. | Target `src/contracts/proto/lgtm/link/v1/link.proto`, `service LinkService`, `rpc Join`; target `src/contracts/testdata/join-token.json`. |
| 10 | Write the stream and its examples. | Target `src/contracts/proto/lgtm/link/v1/link.proto`, `rpc Open`; target `src/contracts/testdata/link-proof.json`. |
| 11 | Write the Go check. | Target `scripts/check-go.sh`; pattern `scripts/check-credits.sh`. |
| 12 | Update the README. | `src/contracts/README.md`, sections "What is here", "Rules" and "Not here yet". |

**Design artifact:** not applicable. The plan adds no screen and changes no screen.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| The link has a mistake that shows only when both ends are built in sprint 3. | high | The reviewers of the Server and Client teams attack `draft/` and the descriptions before items 4 to 6 are built. The worked examples let each team test its end alone. After the merge, a change goes back to phase 3. |
| Two builders regenerate `gen/` at the same time and collide. | medium | Items 2 to 6 run in a chain. Item 7 touches no generated file. |
| A machine without Go cannot regenerate the TypeScript. | low | The README says that generating needs Go 1.26 or newer. Go is needed for the rest of the rewrite. |
| A machine without the cached toolchain must download Go 1.26.0 the first time. | low | The `go` line names an existing release, `1.26.0`, which the download service offers. The development machine has it cached. |
| An old client meets a newer server that sends a command it does not know. | medium | The client answers `COMMAND_OUTCOME_UNSUPPORTED`, which `TestAnUnknownActionReadsAsNoAction` makes possible to detect. `link_revision` refuses a revision that does not match. |
| The clocks of a machine and the server differ, so a command expires early or late. | medium | The server's clock decides, and `Welcome.server_time` gives it. A command starts only before its expiry by that clock, while the server's clock runs without a jump after `Welcome.server_time`. A machine that cannot establish a timely start declines with `COMMAND_OUTCOME_EXPIRED`, and it answers every attempt of a command it started until 24 hours after the expiry. How it keeps that promise is plan `go-s3-client-link`'s, and post-build checks 6, 11 and 20 test the promise. (logic-r1 02, architect-r2 07, architect-r3 06, logic-r3 08, architect-r4 05, logic-r4 05) |
| An acknowledgement deletes an event that the server never applied. | high | The server takes only the next number, commits the effect and the mark together, and acknowledges only after the commit. A gap ends the stream. Post-build check 3 tests it through a crash. (architect-r1 01) |
| An old stream that ends late marks a connected machine offline. | medium | Only the current stream changes the machine's record, and a stream becomes current only after its proof. Post-build check 4 tests it. (logic2-r1 4) |
| A removed machine keeps trying to connect, or a broken connection makes a machine stop for good. | medium | Only `failed_precondition` after a verified `Challenge` is final, with a reason under the key `Lgtm-Link-Refusal` of the terminal error. Every other code may change on a later try. What the client does on each code is plan `go-s3-client-link`'s. A process on the hop behind the web server can still forge the final status, and also an acknowledgement or a `Welcome.server_time`, which is part of point 2 for the owner. (critic-r1 7, threat-r2 04, threat-r3 09) |
| The server's database is restored to an older copy, or a machine's state is copied and both copies run. | high | Outside wave 1, by ruling 2 of the orchestrator in round 2. One machine id has one outbox. A machine whose numbers no longer fit the server's meets a gap, and a machine the server no longer knows is refused, both with the final code. The owner removes the machine and joins it again. The result files ask for an issue in `docs/issues.md`. (architect-r2 01, logic-r2 02, logic2-r2 4, logic2-r2 5, threat-r2 03, threat-r3 02) |
| A removal and a new stream of the same machine race, so a removed machine writes after its removal. | high | The order belongs to plan `go-s3-server-machines`, by ruling 3. The contract fixes only the code and the refused later `Open`. Post-build check 10 tests the race. (architect-r2 03, logic-r2 03, threat-r2 02) |
| The owner leaves point 1 or 2 open when the link is due to freeze. | high | The link freezes only when both sprint-3 plans have cleared review, and points 1 and 2 must be settled by then. Until then, a choice that changes the link is an ordinary work item of the Contract team. (architect-r2 04, ruling 1 of round 3) |
| A renewed certificate with a new key cuts off every machine that joined. | medium | The token pins the key, so a renewal that keeps the key keeps working. Replacing the key is not in wave 1, and a machine then joins again with a new token. The result file reports it. (threat-r1 10) |
| The test writer copies a description file wrongly, and the builder must match a wrong file. | medium | The description files here were produced by a program from the drafts, which passed lint and generated. The builder reports a mismatch to the lead and never edits a frozen test. |
| A new field breaks a website that the plan did not consider. | low | Message fields are optional in the generated TypeScript (`position?: FeedPosition`), and items 2 and 3 run the website's type check and build. |

**Verify:** the gating tests are the commands of items 1, 2, 7 and 8, and the Go tests of items 3 to 6, all in `go test ./src/contracts/`. The environment check is the sign-in of row 3 of "Done when": two commands against the running sample server, and one person signing in on the website. No device is needed.

**Rollout detail:** step 1 of the rollout runs `go mod init github.com/qnlbnsl/lgtm-studio`, then sets the `go` line to `1.26.0`, then runs `go get -tool google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12 connectrpc.com/connect/cmd/protoc-gen-connect-go@v1.21.1`. Step 1 ends with `go mod tidy`. Steps 2 and 4 to 7 each end with `pnpm --filter ./src/contracts generate` and `go mod tidy`. Step 9 needs nothing more than the merge.

</details>

<details><summary><b>Agreements with other teams, from review round 1</b></summary>

The contract writes only facts of the wire, by ruling 2 of round 3. Each row names the team that writes the fact and the team that reads it. What each program does inside belongs to its own plan, and the result file of the fold of round 3 lists what each plan receives. The orchestrator copies each row into the brief of the team that writes it.

| Fact of the wire | Writes it | Reads it |
|---|---|---|
| Events are numbered from 1 with no gaps, and a number never changes. When the machine becomes approved on a stream, by `Welcome` or by `SetState`, it sends its current `Facts` and `HarnessesFound` as new events. (architect-r1 01, logic2-r2 3, logic-r3 01) | Client Link, with the outbox of `go-s1-client-local` | Server Link |
| The server applies only the next number and commits its effect and the number together. `EventAck.through` and `Welcome.last_event_number` are always the committed number, also for a repeat. A gap ends the stream with `failed_precondition` and `event_gap`. A report at the next number that the server cannot use is consumed and has no effect. (critic-r1 5, threat-r1 3, architect-r3 05, logic-r3 04, threat-r3 08) | Server Link, with the store of `go-s1-server-store` | Client Link |
| The server stores a command before it sends it, keeps one expiry, sends it on each new current stream until it holds a final result, accepts a result only for its last attempt on the current stream, and records `COMMAND_OUTCOME_EXPIRED` itself 24 hours after the expiry, except for `SetState`, which stays pending until it has a result. (logic-r1 03, threat-r1 9, architect-r4 04, logic-r4 04) | Server Link | Client Link |
| A machine answers every attempt of a command it started with its recorded result until 24 hours after the expiry, starts a command only when it can establish that it starts before the expiry and otherwise answers `COMMAND_OUTCOME_EXPIRED`, and answers every `SetState` with `COMMAND_OUTCOME_DONE` whatever its expiry. A recorded result comes first. `CommandResult` holds only the id, the attempt and the outcome. (architect-r1 07, logic-r3 10, architect-r3 08, architect-r4 04, architect-r4 05) | Client Link, with the receipts of `go-s1-client-local` | Server Link and Server Machines |
| A stream becomes current when the server admits it to send `Welcome`, and the older one stops being current at that moment. A newer proven stream replaces the older one with `aborted`. A stream admitted after a removal commits receives no `Welcome`, a `Welcome` sent before it has no authority after it, the current stream ends with `failed_precondition` and `unknown_machine`, and every later `Open` gets a `Challenge` and then the same. (logic-r1 06, architect-r2 03, logic-r3 05, architect-r3 04, threat-r3 06, logic-r4 01, logic-r4 02) | Server Machines and Server Link | Client Link |
| The table of codes in the `LinkService` comment, which codes are final, the order of decisions after `Challenge`, and the error metadata keys `Lgtm-Join-Refusal` and `Lgtm-Link-Refusal` with their reasons. (logic2-r1 6, logic-r1 09, architect-r3 03, architect-r3 04, architect-r4 03, architect-r4 06) | Server Machines and Server Link | Client Link and Client enrollment; Stack Web passes codes, headers, stream bodies and trailers on unchanged |
| One projection of a machine on the website, in four disjoint cases: waiting; offline when approved with no current stream; joining when approved with a current stream until the server applied a valid `Facts` and `HarnessesFound` from it while approved; and online after that. (architect-r2 08, logic-r2 09, architect-r3 07, architect-r4 07) | Server Machines | The website |
| A waiting machine's `Welcome` holds an empty connection token, and the server sends it no command except `SetState`. (threat-r2 06, threat-r3 10) | Server Link | Client Link; the later calls of wave 2 |
| `Join` checks the secret first and gives an unknown id and a wrong secret the same `token_unknown`. It then refuses `token_expired`, `token_revoked`, `token_used` and `machine_removed` in that order, claims a new token so that one of two first uses wins, and replays the first reply for the same key. Revoking a used token removes a machine that still waits. (architect-r1 06, logic-r1 05, logic-r2 08, architect-r3 01, logic-r3 11, threat-r3 05) | Server Machines | Client enrollment, the website's pairing dialog |
| `CreateClientTokenResponse.token` holds the `JoinToken` text from a Go server, `address` holds the token's address, and `CreateClientToken` refuses a present `Lgtm-Request-Key` with `invalid_argument` before it makes a token. A sample server may return other text and an empty address. (logic2-r1 7, threat-r1 4, architect-r2 12, architect-r3 10) | Server Machines, the request-key layer of `go-s1-server-store`, and `go-s2-server-signin` | The website, which only shows the text |
| `JoinToken.address` is `https`, or `http` for loopback only. Every `https` token carries the pin of the certificate key the server presents there. The server signs every `Challenge` with the key whose hash is `server_key_sha256`. (architect-r1 05, critic-r1 4, threat-r1 10) | Server Machines and the server's command, with Stack Web for the address and the certificate | Client enrollment and Client Link |
| `Client.hosts_server` comes from a token that the server's own command made on its machine. `Client.address` is the connection's address, for showing only. `memory_gigabytes` is `memory_bytes` over 2^30, rounded. `harnesses_scanned` turns true with the first applied `HarnessesFound`. (architect-r1 11, critic-r1 9) | Server Machines | The website |
| The link's two paths, `/lgtm.link.v1.LinkService/Join` and `/lgtm.link.v1.LinkService/Open`, are mounted outside the website's session gate, and the second passes as a long two-way HTTP/2 stream. A call of `Join` without a cookie gets a link code, never the session's `unauthenticated` for a missing cookie. (architect-r1 05, logic-r1 09) | Server Sign-in mounts them; Stack Web passes them on | Client Link and Client enrollment |

</details>

<details><summary><b>Post-build checks</b></summary>

These checks need built programs, so they run after sprint 3 in the orchestrator's combined link test, `src/linktest/`, against a real PostgreSQL. The combined acceptance of wave 1 joins and connects through the loopback route of the stack's web server only. The `https` join with a self-signed certificate and the refusal of a wrong pin move to the gate of the later proxy work, and the client's own test of a pinned listener and the pin fields stay. (`go-s3-server-machines` round 4) Each one names its scenario and the result that must be seen on the wire or in the database. A check of a program's exit status, its service or an installer belongs to the plan of that program, and the result file of the fold of round 3 hands each one over. (ruling 2 of round 3, architect-r3 12)

| # | Scenario | Expected result | From |
|---|---|---|---|
| 1 | Commit `Join`, drop its reply, and retry with the key the client saved. Then send `Join` with the same token and another key, with an expired token, with a revoked token, and after the machine was removed. | The retry gets the first machine id, and exactly one waiting machine exists. Each of the four other requests gets `permission_denied`. | architect-r1 P1, critic-r1 check 2, logic-r1, threat-r1 T2 |
| 2 | Join and connect through the real web server in front, over loopback. Keep the stream idle except for heartbeats for ten minutes. The `https` case with a pinned certificate is outside wave 1, by the amendment of the freeze. | The machine joins, and the stream stays open for the ten minutes. | architect-r1 P2, logic2-r1 check 8, threat-r1 T1 |
| 3 | Keep events 41 and 42 on the machine, sent on a stream that drops before either commits, and produce 43 during the reconnection while the server's number is 40. Stop the server once before and once after the commit of 41, before its acknowledgement. Send a repeat of 39 when the number is 42. Send an event with a gap, an event of an unknown kind, and an event whose report breaks a rule followed by a valid one. Restore the server's database to an older copy while a machine is connected. | Each committed event is applied once, in order, under its own number, and no two reports share a number. The repeat gets `EventAck` with 42. The gap ends the stream with `failed_precondition` and `event_gap`, and nothing is lost. The unknown event and the broken one are consumed, set no readiness, and the valid event behind them is applied on the same stream. After the restore, the machine's next event is a gap, and the server refuses it with the final code. | architect-r1 P3, critic-r1 check 3, logic-r1, logic2-r1 checks 3 and 4, threat-r1 T5, logic-r3 01, logic-r3 04, threat-r3 08 |
| 4 | Open stream B while the server still holds stream A, let B deliver a valid `Facts` and `HarnessesFound`, and let A end 50 milliseconds after B's `Welcome`, with a late heartbeat and a late result on A. Separately, hold B's reports after its `Welcome`. Then let a proxy stop passing bytes on B without closing it. | With both reports applied on B, the machine shows online on B, and nothing from A changes the record. With B's reports held, the machine shows joining. On the silent B, the server shows the machine offline within 30 seconds. | critic-r1 check 3, logic-r1, logic2-r1 checks 2 and 6, threat-r1 T4, architect-r3 07, logic-r3 10 |
| 5 | Remove a machine while it is offline, then let it dial. Remove another while it is connected, and drop the `SetState` notice on the way. Interrupt only the network of a third machine. | The first two get a `Challenge` whose signature verifies and then `failed_precondition` with `Lgtm-Link-Refusal: unknown_machine`, and neither appears in the list. The third machine connects again with its key and no new join. | architect-r1 P4, critic-r1 check 5, threat-r1 T4, logic-r3 03, threat-r3 06 |
| 6 | Keep a scan running across a reconnection, so that attempt 2 arrives while attempt 1 runs. Kill the client after a second scan recorded its start. Kill the client during `Restart`, once before and once after it recorded its acceptance. Send a command whose expiry has passed by the server's clock, with the machine's clock five minutes behind. Send a `SetState` whose expiry has passed. Remove a machine, join it again, and send it its first state. | The first scan's result closes attempt 2 once. The killed scan answers `COMMAND_OUTCOME_INTERRUPTED` and does not run again. The `Restart` is answered once with `COMMAND_OUTCOME_DONE`, and the next stream comes from a new program. The expired command answers `COMMAND_OUTCOME_EXPIRED` and has no effect. The late `SetState` is applied. The rejoined machine applies its state of revision 1. | architect-r1 P4, critic-r1 check 4, logic-r1, logic2-r1 checks 1, 5 and 7, threat-r1 T6, logic-r3 10 |
| 7 | On the real Machines page against the Go server, make a token, paste the page's command on a new machine, approve the machine, and use the page's restart action as point 1 for the owner settles it. | The waiting machine appears without a reload, then shows online with its facts, load and harnesses, and the restart happens once and the machine comes back. | architect-r1 P5, critic-r1 check 2, logic-r1, threat-r1 T7 |
| 8 | On a clean checkout of the sprint's merge commit, generate twice, then add the valid untracked probe of item 2 and run the check with and without its guard. Decode a `WatchClients` reply with and without `position` using the website's old generated code. | The second generation changes nothing. The check fails with the guard and passes without it. The old code reads the same saved machine from both replies. | architect-r1 P6, critic-r1 check 1 |
| 9 | Make and use join tokens with a marked fake secret, then search the database's request-key receipts, the server's and the client's logs, and the client's process list. Connect a waiting machine, try the actions that point 3 for the owner refuses it, and try an owner call with a machine's connection token. | The marked secret appears in none of those places, except the process list if point 4 keeps the argument. The waiting machine gets only what point 3 allows, and the connection token opens no owner call. | threat-r1 T2 and T3 |
| 10 | Through the test seams of `go-s3-server-machines`, hold stream B of a machine once after its `Proof` verified and before its admission commits, and once after the admission and before its `Welcome` is sent. Hold an event and a result of stream A before their commit. Remove the machine while each is held, then release them. Repeat with the revocation of a used token of a waiting machine, and with an approval racing that revocation. | B never receives `Welcome`, and gets `failed_precondition` with `unknown_machine`. Neither held write changes the database or a `Watch` reply. In the race of approval and revocation, B gets the refusal only when the revocation removed the machine; when the approval committed first, the machine stays and B may receive `Welcome`. Plan `go-s3-server-machines` owns the order that makes this true. UNREVIEWED FIX. | architect-r2 P2, logic-r2 L2-P1, threat-r2 T2, logic-r3 05, logic-r3 11, architect-r3 04, architect-r3 12, architect-r4 01 |
| 11 | Send a command that the machine started and recorded, drop its result, and send it again at its expiry plus 22 hours and 30 minutes by the server's clock, with `Welcome` delayed by 8 seconds. Then hold another pending command past 23 hours after its expiry. | The machine answers with the recorded result, not `COMMAND_OUTCOME_EXPIRED`, and the server stores it. The second command is never sent after 23 hours. Delivery at 23 hours and 59 minutes is a test of plan `go-s3-client-link`. | architect-r2 P4, logic-r2 L2-P3, threat-r2 T6, architect-r3 06, logic-r3 08, threat-r3 07, `go-s3-server-machines` round 4 |
| 12 | Join with a wrong secret on a known token id, with an unknown id, and with a used token and the first key but a wrong secret. Run two identical first uses at once, and two first uses with two keys at once. | The three refusals are the same `token_unknown` in `Lgtm-Join-Refusal`, and none returns a reply or changes a row. The identical pair both get the same machine id. Of the pair with two keys, one gets a machine and the other `token_used`. | architect-r3 01, logic-r3 11, threat-r3 05 |
| 13 | Connect a waiting machine, then approve it on that same stream. | The machine shows joining, then online with its facts and harnesses within 30 seconds, with no new stream. | architect-r2 P5, logic2-r2 check 3, threat-r2 T5 |
| 14 | Call `CreateClientToken` with a request key, and without one, against the Go server. | The keyed call is refused with `invalid_argument` and makes no token. The other reply carries `address` equal to the address inside its token. | architect-r2 P6, logic-r2 L2-P5, threat-r2 T7, architect-r3 10 |
| 15 | Send a `CommandResult` built with `protowire` that holds the outcome and a field 4 with a marked fake secret, and check the database, the logs and the website. | The outcome is stored, and the marked text appears nowhere, because the server discards the reserved field. | threat-r2 T8, architect-r3 12, logic-r3 12 |
| 16 | Removal and a `Welcome` in transit: let the server send B's `Welcome`, hold those bytes in the web server, commit the removal, then release them. In a second run, hold the server before it admits B, and commit the removal. | In the first run, the machine receives `Welcome` and then `failed_precondition` with `unknown_machine`, and its connection token and any later message on B change nothing. In the second run, it receives only `Challenge` and the refusal. UNREVIEWED FIX. | architect-r4 01, logic-r4 02 |
| 17 | At a mark of 42 on the current stream, send as separate fixtures: a message that cannot be decoded; event 0; a repeat of 42 whose `Facts` breaks a limit; event 44 whose report breaks a limit; event 43 with a report that breaks a rule, followed by a valid 44. | Respectively: `invalid_argument` with the mark at 42; `invalid_argument`; `EventAck` with 42 and the stream stays; `failed_precondition` with `event_gap` and the mark at 42; 43 consumed with no readiness, then 44 applied. UNREVIEWED FIX. | architect-r4 02, logic-r4 06 |
| 18 | Through the real web server, over Connect and over gRPC, refuse a removed machine right after `Challenge`, and end a welcomed stream on a gap. Refuse a `Join` with a used token. | The client reads `unknown_machine` and `event_gap` from the metadata of the terminal error of `Open`, and `token_used` from the metadata of the `Join` error, in each protocol. UNREVIEWED FIX. | architect-r4 03, logic-r4 03 |
| 19 | Approve a waiting machine on its stream and hold the first delivery of the `SetState` past its expiry, once by a few seconds and once by more than 24 hours. Send a `SetState` again whose revision the machine already holds. | Each `SetState` is answered `COMMAND_OUTCOME_DONE`. The machine applies the approval and sends its two reports on the same stream, and the server never records `COMMAND_OUTCOME_EXPIRED` for a `SetState`. The repeat changes nothing. UNREVIEWED FIX. | architect-r4 04, logic-r4 04 |
| 20 | Send an unstarted command whose expiry is one second after the server's time at the moment it arrives, with the hop delays before and after the sample of `Welcome.server_time` varied. Then send one whose start a recorded result already exists for. | No command starts at or after its expiry by the server's clock. A declined command answers `COMMAND_OUTCOME_EXPIRED` and never starts later. The recorded result is answered whatever the expiry. UNREVIEWED FIX. | architect-r4 05, logic-r4 05 |
| 21 | Send a `Hello` from a removed machine with an unsupported revision, from a known machine with an unsupported revision and a bad signature, and from a known machine that sends no `Proof`. Make the server's lookup fail once. | `unknown_machine`, then `link_revision_mismatch`, then `deadline_exceeded`, and `unavailable` for the failed lookup, each after `Challenge` and each the same on a second try. UNREVIEWED FIX. | architect-r4 06 |
| 22 | Approve a machine while it is offline and has never reported, open its stream, apply one valid report, then both, then close the stream. | `WatchClients` shows offline, joining, joining, online and offline, in that order. UNREVIEWED FIX. | architect-r4 07 |
| 23 | Hold stream A's event 41 against the admission of stream B with the mark at 40, once on each side of B's admission. Separately, admit A, hold its `Welcome` before it is sent, and admit B. | When A's event commits first, B's `Welcome.last_event_number` is 41. When B is admitted first, A's event changes nothing and the mark stays 40 until B sends 41. In the separate case, A receives no `Welcome` and ends with `aborted`. UNREVIEWED FIX. | logic-r4 01, `go-s3-server-machines` round 4 |
| 24 | End a stream on a gap at the mark 40 with event 42, then open a new stream that sends 41 and 42. | The new stream applies 41 and 42, and no refusal repeats. UNREVIEWED FIX. | logic-r4 07 |
| 25 | Let one first use of a token commit just before its expiry, and let a second request with the same key reach its decision just after it. Repeat with a revocation that commits between the two. | One machine exists. The second request gets `token_expired`, or `token_revoked`, by the order of refusals at its own decision. UNREVIEWED FIX. | logic-r4 08 |

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

Source universe:

1. The brief's work items 1 to 7 (`plan.md` of 2026-11-25).
2. The brief's points to settle 1 to 4.
3. The brief's three observations.
4. The brief's two agreements with other teams.
5. The five points of the task file `.foundry/go-rewrite/tasks/plan-go-s0-contract.md` under "Specific to this plan".
6. The design's statements that bear on the contract: D1, D2, D5, D7, D9, D15, D16, D17, R2, R5, R6, R8, R9, R10, C5 to C12.
7. The design's sections "How every `Watch` call works" and "What must change outside this document before builders start", third point.
8. `decisions.md`, rows 1 to 70.
9. The findings of review round 1, in `attack/round-1-findings.md`.
10. The findings of review round 2, in `attack/round-2-findings.md`, and the four rulings of the orchestrator for round 2.
11. The findings of review round 3, in `attack/round-3-findings.md`, and the five rulings of the orchestrator for round 3.
12. The findings of review round 4, in `attack/round-4-findings.md`, and the orchestrator's instruction that this is the last fold before sprint 0 builds the plan.

| Source item | Workstream | Status |
|---|---|---|
| Brief item 1, the Go module | WS-1, item 1 | covered |
| Brief item 2, Go generation | WS-1, item 2 | covered |
| Brief item 3, the shared `Watch` shape | WS-1, item 3 | covered |
| Brief item 4, joining and the proof of both ends | WS-1, items 5 and 6 | covered, split in two |
| Brief item 5, commands to a machine | WS-1, item 4 | covered |
| Brief item 6, events from a machine | WS-1, item 4 | covered |
| Brief item 7, the Go check and the README | WS-1, items 7 and 8 | covered, split in two |
| Point 1, what `common.proto` holds | WS-1, item 3; decision 8 | settled |
| Point 2, the feed position of `WatchClientsResponse` | WS-1, item 3; decision 9 | settled |
| Point 3, where the Go lives under `clean: true` | WS-1, item 2; decision 10 | settled |
| Point 4, the fields of the join token | WS-1, item 5; decisions 14 and 15 | settled |
| Observation: `go build ./...` passes | Done when, row 1 | covered |
| Observation: the contract check fails when the Go is stale | Done when, row 2 | covered |
| Observation: the website builds and signs in | Done when, row 3 | covered |
| Agreement: both teams review the link before it is frozen | Rollout order, steps 5 to 7 and 9; risk 1 | covered |
| Agreement: room for byte streams and for agent calls | WS-1, item 6; decision 27 | covered |
| Task: exact names and numbers of `common.proto` and the link files | WS-1, items 3 to 6, the description files | covered |
| Task: whether the revision stays at 1 | WS-1, item 3; Done when, row 5; decision 9 | covered |
| Task: `clean: true` and one plugin today | WS-1, item 2; decision 10 | covered |
| Task: Go 1.23.3 and connect 1.21.1 needing Go 1.26 | WS-1, item 1; decisions 12 and 13 | covered |
| Task: room for wave 2 without planning it | WS-1, items 4 and 6; decision 27 | covered |
| D1 | WS-1, item 2 | covered |
| D2 | WS-1, items 1 and 2 | covered |
| D5 | WS-1, item 3 | covered |
| D7 | WS-1, items 5 and 6 | covered |
| D9 | WS-1, item 6, room for agent calls | covered |
| D15 | WS-1, item 6, room for byte streams | covered |
| D16 | WS-1, items 3 and 4 | covered |
| D17 | WS-1, items 5 and 6 | covered |
| R2 | Rollout order; risk 1 | covered |
| R5 | WS-1, item 3 | covered |
| R6 | WS-1, item 4 | covered |
| R8 | WS-1, items 5 and 6, the error codes | covered |
| R9 | WS-1, item 5, the token text is a secret | covered |
| R10 | WS-1, items 6 and 7 | covered |
| C5 | WS-1, item 5 | covered |
| C6 | WS-1, item 3 | covered |
| C7 | WS-1, item 4 | covered |
| C8 | WS-1, item 4 | covered |
| C9 | WS-1, item 6 | covered |
| C10 | WS-1, items 5 and 6 | covered |
| C11 | WS-1, items 4 and 6 | covered |
| C12 | WS-1, item 7 | covered for `go test ./...`; the lock half belongs to `go-s1-server-store` |
| Section "How every `Watch` call works" | WS-1, item 3 | covered |
| Section "What must change outside", third point | WS-1, item 8 | covered |
| `decisions.md` rows 1 to 37 | Key decisions; WS-1 | every row closed or unanswered with its assumption |
| `decisions.md` rows 38 to 51 and 53 | Items 2 to 8; agreements with other teams | closed, or unanswered with its assumption (row 42); row 39 is closed, because row 54 replaces it |
| `decisions.md` row 52 | Points for the owner | unanswered with its assumption; points 1 and 2 wait for the owner until the link is frozen, points 3 and 4 until sprint 3 (ruling 1 of round 3) |
| `decisions.md` rows 54 to 62 | Items 2 to 6; agreements with other teams; post-build checks 10 to 15 | closed; rows 54, 55 and 56 carry the orchestrator's rulings |
| Review round 1, 59 findings | `attack/round-1-findings.md`; post-build checks | every finding has one outcome |
| Review round 2, 43 findings | `attack/round-2-findings.md`; post-build checks 10 to 15 | every finding has one outcome |
| Review round 3, 35 findings | `attack/round-3-findings.md`; post-build checks 3 to 6 and 10 to 15 | every finding has one outcome |
| `decisions.md` rows 63 to 68 | Rollout step 9; items 4 to 6; agreements; post-build checks | closed |
| Ruling 1 of round 3, when the link is frozen | Rollout step 9; points for the owner; risks | covered |
| Ruling 2 of round 3, facts of the wire only | Key decisions; items 4 to 6; agreements; post-build checks | covered; each removed rule is a request in the result file |
| Ruling 3 of round 3, a sprint-3 plan that disagrees fixes itself | Result file of the fold of round 3 | covered; each disagreement is listed there |
| Ruling 4 of round 3, the contract's own defects | Items 4 and 5 | covered: no two reports share a number, and `Join` checks the secret first |
| Review round 4, 15 findings | `attack/round-4-findings.md`; post-build checks 10 and 16 to 25 | every finding has one outcome; every changed place is marked UNREVIEWED FIX |
| Amendment: the freeze of the link, issue 1643 and the requests of plans `go-s3-client-link`, `go-s3-server-machines`, `go-s3-stack-client` and `go-s1-client-local` | Section "Amendment: the freeze of the link"; items F1 to F5; `decisions.md` rows 71 to 85 | every finding and request has one outcome |
| `decisions.md` rows 69 and 70 | Items 4 to 6; public names; post-build checks 16 to 25 | closed |
| Ruling 1 of round 2, the server alone decides what was applied | Item 4, rules of events; public names | covered; `Hello.acked_event_number` removed |
| Ruling 2 of round 2, a restore is outside wave 1 | Item 4, rules of events; item 6, the handshake and removal; risks | covered; the issue is requested in the result file |
| Ruling 3 of round 2, serialization belongs to `go-s3-server-machines` | Item 6, the current stream; agreements; post-build check 10 | covered; the request is in the result file |
| Ruling 4 of round 2, keep the plan from growing | `CommandResult.message` removed; the value rule on the revision removed; the current-stream rules cut to what the machine observes | covered |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repo rule (CLAUDE.md etc.) | How this plan satisfies it |
|---|---|
| `AGENTS.md`: the three parts meet only through the contract in `src/contracts/`. | The link is written in `src/contracts/proto/lgtm/link/v1/`, and the two programs share only its generated code. The worked examples are data, not code. |
| `AGENTS.md`: a change to what the parts exchange starts in `src/contracts/proto/`. | Every item starts from a `.proto` file or the generation set-up. |
| `AGENTS.md`, habit 1: credit copied code. | No code is copied. The worked examples were computed with the Go standard library. |
| `AGENTS.md`, habit 2: write down what you find. | This team may not edit `docs/`. The result file lists each finding for the orchestrator to record. |
| `AGENTS.md`, habit 3: finished work reaches the base branch through a pull request. | The orchestrator merges the sprint through a pull request. |
| `AGENTS.md`, writing style, and `scripts/check-private.mjs`. | The plan files use full sentences, expand acronyms, quote no person, and name no home folder or private address. Only loopback addresses appear. |
| `src/contracts/README.md`: a field number is never used again, and a field's type never changes. | Item 3 only adds field 4 to two messages. No existing field changes. `CommandResult.message` never reached `master`, and its number and name are reserved all the same. |
| `src/contracts/README.md`: a change that breaks an older website raises the contract revision. | Sprint 0 breaks no website, so the revision stays at 1, and item 3 checks it. The new meaning of `ReconnectClient` breaks the page once a Go server answers it, which point 1 for the owner settles before sprint 3. (logic2-r1 1) |
| `src/contracts/README.md`: every message, field and call has a comment. | The drafts of the link have them since round 1, and `TestEveryLinkRuleIsWritten` checks it for the link files. The files of `lgtm.v1` keep the comments they have, and this plan adds none to them except those item 3 names. (logic2-r1 12) |
| The epic: a team edits only its own folders. | Every file is in `src/contracts/`, `go.mod`, `go.sum` or `scripts/check-go.sh`, as `.plans/scopes/contract/contracts/scope.json` lists. |
| The design, rule 2: a contract file is written and reviewed before its code. | This plan is that step for the link and the shared shape. |
| The design, rule 10, and issue 1634. | The worked examples let the server and the client test the proof alone. The test that runs both lives in `src/linktest/`, which the orchestrator owns. |
| The planning rules of this epic: name no model and no harness. | The plan names work areas and complexities only. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

Models come from the harness-sessions plugin's model sheet (its harness-session skill) at launch; this section names none (owner, 2026-10-16).

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on |
|---|---|---|---|---|---|---|
| 1. The Go module | `contracts` | `contract/contracts` | low | One file made by two commands, with exact check commands. | no | nothing |
| 2. Go generation | `contracts` | `contract/contracts` | medium | A few files on a known pattern; the layout was tried on 2026-11-26. | no | 1 |
| 3. The shared shape | `contracts` | `contract/contracts` | medium | One new file, one new field, and a reflection test. | no | 2 |
| 4. Commands and events | `contracts` | `contract/contracts` | medium | Two files of messages on the pattern of `clients.proto`, with fixed descriptions. | no | 3 |
| 5. Joining | `contracts` | `contract/contracts` | medium | One file of messages, with a worked example of the token. | yes, because `Join` is reached by a machine without a session | 4 |
| 6. The stream and its proof | `contracts` | `contract/contracts` | medium | Messages added to one file, with worked examples of the signed bytes. The high-rated item of the brief was split into items 5 and 6. | yes, because `Open` is reached by a machine without a session | 5 |
| 7. The Go check | `infra` | `contract/contracts` | low | One shell script of five steps. | no | 2 |
| 8. The README | `contracts` | `contract/contracts` | low | One document checked against files that exist. | no | 6, 7 |

Load: each unit needs one test writer and one builder of the other vendor, which is 16 launches. Items 3 to 6 are risky, so each also needs one reviewer that breaks the code on a temporary branch, which is 4 launches. Code review of the merged sprint needs about 3 launches: one from each vendor that did not build a slice, and one that reads beside them. That is about 23 launches, split about evenly between the two vendors. The orchestrator reads each vendor's remaining weekly allowance at the start of the sprint, as the epic says, and the planner cannot read it.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (B/M/m) | Outcome |
|---|---|---|
| 1 | 6 / 40 / 13, from five reports, in 22 groups | 47 fixed, 3 fixed another way, 9 for the owner, none rejected. The event rule and the removal rule were rewritten, because each held a blocker. The register is `attack/round-1-findings.md`, and its list of patches names what round 2 attacks. |
| 2 | 13 / 26 / 4, from four reports, in 15 groups, 73 points | 27 fixed, 15 fixed another way, 1 for the owner, none rejected. By the orchestrator's rulings, the numbering, acknowledgement and resumption of events went back to the design's three lines, a restore left wave 1, and the order of removal and stream writes moved to `go-s3-server-machines`. No statement is stuck. The register is `attack/round-2-findings.md`, and its list of patches names what round 3 attacks. |
| 3 | 6 / 27 / 2, from three reports, in 17 groups, 85 points | 7 fixed, 27 fixed another way, 1 for the owner, none rejected. By the orchestrator's rulings, the link freezes when both sprint-3 plans clear review, and the plan keeps only facts of the wire: exit statuses, receipts, clocks, addresses and the server's write order moved to the plans that own them. The contract's own defects were fixed: no two reports can share a number, and `Join` checks the secret first. No statement is stuck: C11 has 21 points, which is 25% of the round. The register is `attack/round-3-findings.md`. |
| 4 | 0 / 15 / 0, from two reports, in 10 groups, 30 points | 13 fixed, 2 fixed another way, none for the owner or rejected. Every fix is marked UNREVIEWED FIX and has a post-build check, because no reviewer reads this fold before the build. Two rules no server could keep were weakened to what an end observes: a `Welcome` in transit at a removal, and a final code that never changes. Wire descriptors are unchanged. No statement is stuck. The register is `attack/round-4-findings.md`. |

</details>
