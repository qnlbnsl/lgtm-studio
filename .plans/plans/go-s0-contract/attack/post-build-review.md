# Post-build review of sprint 0, the contract

- Task identifier: s0-postbuild-review, nonce pr-18d3.
- Reviewer: a reviewer of built code that did not build the contract and did not write its tests.
- Branch `sprint-0`, commits `28208cb4`, `ac1ff06a`, `8d6c6e12` and `702e05e3`, with the tests of `88dfae99` and `20d5c4d9`.
- Read-only review. No file other than this report was changed.

## Verdict

**REVISE.** The contract is built as the plan says and nothing blocks the next sprint, but a short list of text changes and two test additions should land before the link is frozen.

## Top finding

The tests hold the shape of the link (names, numbers, types) but none of its rules, so a contract with a wrong rule in a comment passes every test (post-s0 1).

## What was run

- `go test -count=1 ./src/contracts/...` exits 0, and every test in `src/contracts` passes.
- `scripts/check-go.sh` exits 0. It checks the formatting, runs `go mod tidy -diff` and `go vet ./...`, and runs `go test ./...`.
- The working tree also holds unrelated changes of other teams in `src/server`, `deploy` and `.plans`, and one change to `src/contracts/checks/item2-go-generation.sh` that is not committed. I did not judge those.
- The post-build checks 10 and 16 to 25 need the programs of `src/linktest/`, which do not exist yet, so I could not run any of them. For each place I judged the text of the contract, and the verdict of each place is in the table below.

## The built files against the drafts

- `link.proto`, `events.proto`, `commands.proto` and `common.proto` equal the files in `draft/` byte for byte. This is a clean result, and it is not a finding.
- The five description files of `testdata/golden/` equal the golden contents that the plan prints.
- The changes to the four existing website files equal `draft/existing-files.diff`, except for the `go_package` lines. Items 1 and 2 of the plan, merged in `28208cb4`, added those lines, and the plan says so in the section on item 2. This is explained, and it is not a finding.

## The worked examples

- `testdata/join-token.json` is verified by `TestJoinTokenExampleDecodes` (`join_test.go:43`). The test decodes the text, unmarshals it as a `JoinToken`, compares each field, recomputes the SHA-256 of the server key, and marshals the message again to the same bytes.
- `testdata/link-proof.json` is verified by `TestProofExamplesHoldTogether` (`open_test.go:139`). The test derives both keys from the seeds, rebuilds both proof texts from the rule, and verifies both signatures. It also checks that neither signature verifies against the other end's text.
- Both examples are checked by computation and not only read. One weakness remains: the test builds the proof text from its own copy of the rule, so nothing ties that rule to the comment in `link.proto` (post-s0 1).

## The ten places that no reviewer read before the build

| # | Place | Judgement of the text |
|---|---|---|
| 1 | Removal and a `Welcome` in transit | Holds together. `link.proto` lines 129 to 139 order the removal against admission, allow a late `Welcome`, and strip its authority. Check 16 cannot run. See post-s0 5 for one gap next to it. |
| 2 | The event decision table and the value rules | Holds together, with one gap in the value rules (post-s0 6). `link.proto` lines 9 to 11 hand precedence to the table, and `events.proto` lines 14 to 26 are complete for numbers. Checks 17 and 24 cannot run. |
| 3 | Refusal reasons in metadata | The text is written and consistent in `link.proto` lines 36 to 57. Check 18 cannot run, and no test reads the reasons (post-s0 1). |
| 4 | `SetState` stays pending | Contradicts a rule next to it (post-s0 4). |
| 5 | Command expiry | Holds together for starting a command. The outcome `COMMAND_OUTCOME_EXPIRED` means two things (post-s0 3). |
| 6 | Handshake order | Self-consistent. It lets an unauthenticated caller learn a machine's standing (post-s0 7). |
| 7 | The website's projection | Four disjoint cases, as the plan says. No finding. |
| 8 | The current stream and `Welcome.last_event_number` | Holds together. One gap about the connection token (post-s0 8). |
| 9 | Final codes and gaps | The word "final" is defined as determinism only, and the text beside it uses it for a gap that the machine must continue past (post-s0 2). |
| 10 | The loser of two first uses | Holds together with the order of refusals. One consequence for a lost reply is in post-s0 9. |

## Findings

### post-s0 1 — MAJOR — The tests hold no rule that lives in a comment

- **Statement.** The substance of the link is its rules, and every rule is a comment. `TestEveryLinkRuleIsWritten` checks that each declaration has a comment of three words, and that ten code and header names occur somewhere in `link.proto`. It reads no rule.
- **Evidence.** `open_test.go:101` to `137`. The golden files fix names, numbers, types and reserved ranges. Nothing reads the refusal reasons `token_unknown`, `event_gap` or `link_revision_mismatch`, the numbers 10 seconds, 30 seconds and 24 hours, or the order of the handshake. The proof text rule is rebuilt inside the test at `open_test.go:152` to `160` from the test's own copy.
- **Failure scenario.** Each of these changes passes `go test ./src/contracts/...` and `scripts/check-go.sh`:
  - the text "10 seconds" in the handshake becomes "10 minutes";
  - the order after `Challenge` in `link.proto` lines 89 to 94 is reversed, so that `unauthenticated` comes first;
  - the reason `event_gap` is deleted from the table, because the test only looks for the code name `failed_precondition` in `events.proto`;
  - the row for `aborted` is deleted, since the name occurs again in the `Open` comment;
  - the comment of `Proof` names the machine nonce before the machine id, which contradicts the example file, and the test never reads that comment.
  A server team and a client team then build against different readings with no red test.
- **Fix.** Add a test that extracts a small set of exact strings from the three files: the nine reason words, the numbers of seconds and hours, the two proof labels, and the order of the six handshake decisions as one ordered match. Add a test that parses the `Proof` comment and compares the order of its fields with the rule that the proof test builds. Plan items: `plan`, post-build checks 18, 21 and 20. Design: C10.

### post-s0 2 — MAJOR — "Final" does not say what a client does, and `event_gap` is final yet must be continued past

- **Statement.** `link.proto` defines a final code only by determinism: the same request against the same state gets the same answer (lines 36 to 38). The same table marks `event_gap` as final (lines 46 to 51), while `events.proto` says that a later stream that sends the missing number goes on (lines 21 to 23). A client reads "final" as "stop trying", which for `event_gap` destroys the machine's link.
- **Evidence.** `link.proto:36-38`, `link.proto:46-51`, `events.proto:21-23`. The plan hands the client's reaction to `go-s3-client-link`, which cannot read it from the contract.
- **Failure scenario.** The sprint-3 client team reads the table and stops redialling on any final code. A single gap, for example after a restore of the server's database or a lost buffer, then ends the machine's link for good. This breaks C11, which wants buffered events to arrive once and in order, and C9, which wants a machine to come back by itself. If the client team reads it the other way, a removed machine redials forever, which breaks C10.
- **Fix.** In the table, say per row what the machine does: stop and tell the owner (`unknown_machine`, `link_revision_mismatch`, `permission_denied`), or dial again at once and resend from the mark (`event_gap`). Rename the idea of "final" to "deterministic" in the table, or keep the word and add the column. Design: C9, C10, C11. Plan: the agreement with `go-s3-client-link`.

### post-s0 3 — MINOR — `COMMAND_OUTCOME_EXPIRED` has two meanings, and the retention edge can produce a false one

- **Statement.** The enum says the machine did not start the command (`commands.proto:78-80`). The server also records the outcome itself 24 hours after `expires_at` when it holds no final result (`commands.proto:7-9`). A command that ran, whose result never arrived, is then recorded as one that did not start.
- **Evidence.** `commands.proto:6-9`, `12-13`, `78-80`. The server stops at 24 hours and the machine keeps receipts "until 24 hours after expires_at". The two ends measure that time with different clocks and have no margin between them.
- **Failure scenario.** A `Restart` runs, and its result is lost with the connection. The machine stays offline for 25 hours. The server records `COMMAND_OUTCOME_EXPIRED`, and the website says the restart did not happen. The owner asks again. In the second case, a machine whose clock estimate runs ahead drops the receipt a little early, a late attempt arrives, and the machine answers `COMMAND_OUTCOME_EXPIRED` for a command it ran. Design: R6.
- **Fix.** Add a separate value for the server's own timeout, or say in the enum that the server may record this outcome for a command whose effect is unknown. Add a margin: the machine keeps receipts longer than the server sends, by a stated amount.

### post-s0 4 — MINOR — A pending `SetState` has no end, and removal names no end for pending commands

- **Statement.** `SetState` stays pending until it has a result, whatever its expiry (`commands.proto:8-9`). A removed machine is refused for good (`link.proto:136-137`), and the server "may send SetState with APPROVAL_REMOVED ... nothing waits for its delivery" (`link.proto:138-139`). The text never says that a removal ends the machine's pending commands.
- **Evidence.** `commands.proto:6-9`, `link.proto:129-139`, `commands.proto:126-128`. The text also names no action of the client on `APPROVAL_REMOVED`, and does not say whether `Welcome.state` may hold it, although a `Welcome` sent before the commit can still arrive (`link.proto:133-135`).
- **Failure scenario.** The owner removes an offline machine with a `SetState` to `APPROVAL_REMOVED` pending. The machine never connects again, so the command never has a result, and the rule keeps it pending without end. A server that implements "send pending commands on each new stream" and "stay pending" literally keeps the row for ever. A client that does receive `APPROVAL_REMOVED` has no rule to follow.
- **Fix.** Add one sentence: a removal ends every pending command of that machine. Add one sentence to `MachineState`: what a machine does on `APPROVAL_REMOVED`, and that `Welcome.state` is never `APPROVAL_REMOVED`, or that it may be and what follows. Plan: places 1 and 4. Design: R6, D7.

### post-s0 5 — MINOR — The removal rule does not say whether a removed machine's pending event is acknowledged

- **Statement.** `link.proto:133-135` says that nothing arriving after the commit changes the machine, and `events.proto:7-13` says that `EventAck` follows only a commit. The text does not say what the server sends for an event that arrives after the removal committed but before the stream ends. A server may send `EventAck` for an event that it did not apply, or nothing.
- **Evidence.** `link.proto:129-139`, `events.proto:7-13`, `events.proto:19-20`.
- **Failure scenario.** After a removal, stream B (still open for a moment) sends event 44. One server sends `EventAck` with 43 as for a repeat, another sends nothing and ends the stream. A client that treats an ack as permission to forget its buffer can drop events for a machine whose removal the owner later undoes by joining again, and the two servers leave it in different states.
- **Fix.** State that a removed machine's stream gets no `EventAck` after the commit, or that an ack never lets the machine forget more than the mark it already held. Plan: place 1, check 16.

### post-s0 6 — MINOR — The value rules do not list the numeric limits of the field comments

- **Statement.** The list of value rules in `link.proto:9-18` names ids, nonces, keys, signatures, hashes, three numbers and strings. The limits "0 to 100" (`events.proto:102-106`), "at least 1" for `Facts.cores` (`events.proto:69-70`) and "at most 16" harnesses (`events.proto:85`) live only in field comments. The order between value rules and refusals on `Join` is not given either.
- **Evidence.** `link.proto:9-18`, `events.proto:69-70`, `85`, `102-106`. `Heartbeat` is not an event and has no table, so a heartbeat with a load of 250 has no stated outcome. A `Join` with a wrong secret and a host name of 300 bytes may get `token_unknown` or `invalid_argument`.
- **Failure scenario.** Two servers answer a heartbeat of 250: one ends the stream with `invalid_argument` and the machine is offline until the next stream, the other stores 250 and the website shows a load above 100. Check 12 and check 17 see different codes from two correct servers.
- **Fix.** Add the numeric limits to the list of value rules, and say in one sentence what happens to a heartbeat that breaks one. Say whether the value rules of `Join` come before or after the refusals.

### post-s0 7 — MINOR — The handshake answers about a machine before its signature is checked

- **Statement.** The server answers every well-formed `Hello` with a `Challenge`, then decides `unavailable`, `unknown_machine` and `link_revision_mismatch` before `Proof` arrives (`link.proto:86-94`). Any caller can learn whether a machine id exists, was removed, or speaks another revision, with no key.
- **Evidence.** `link.proto:86-94` and `46-51`. Machine ids are version 7 UUIDs, which carry a time and are not secret.
- **Failure scenario.** A caller on the loopback address or behind the proxy sends `Hello` with a guessed id and reads `unknown_machine` against a silent `deadline_exceeded`. Under the owner's one-user decision the damage is small, and design D7 asks for a proof by both ends, which this does give for a verified machine. The oracle is still new information to an unauthenticated caller.
- **Fix.** Either accept the oracle and write one sentence that says so with the reason, or move `unknown_machine` after the signature check and answer a wrong machine id with `unauthenticated` only.

### post-s0 8 — MINOR — A machine approved on a live stream never gets a connection token

- **Statement.** A waiting machine's `Welcome` holds an empty connection token, and the machine gets a token "in the Welcome of a stream that opens after its approval" (`link.proto:125-126`). Check 13 approves a machine on the same stream with no new stream. No message carries a token to that machine afterwards.
- **Evidence.** `link.proto:125-126`, `254-255`, and the header of `Lgtm-Link-Token` at `link.proto:31-34`.
- **Failure scenario.** In wave 2 the machine, approved on a live stream, tries a call beside the stream, such as a byte stream, and has no token. The contract offers no way to get one except to drop the stream, so wave 2 must either break the rule or add a message to a frozen link.
- **Fix.** Say that an approved machine reconnects to obtain its token, and that the server ends the stream for it after approval, or add the field to `SetState` now while room remains. Plan: place 8.

### post-s0 9 — MINOR — A lost `Join` reply is unrecoverable after the token's expiry

- **Statement.** The refusals are checked in order, and `token_expired` comes before the replay rule (`link.proto:61-76`). A machine whose reply was lost and which retries after 15 minutes gets `permission_denied` with `token_expired`, though its machine exists.
- **Evidence.** `link.proto:61-76`, `clients.proto` (the expiry of 15 minutes), and the joining steps of the design, which give a token 15 minutes.
- **Failure scenario.** The client crashes after the commit, and is restarted after 20 minutes. It holds no machine id, so it cannot send `Hello`. The server holds a waiting machine under the host name that nobody can connect. The owner sees a machine that never comes online and must remove it and make a new token.
- **Fix.** Either let a replay with the same key succeed after expiry and say so in the order, or say that the client must treat `token_expired` after a lost reply as the owner's case and name the owner's action in the Machines page text.

### post-s0 10 — MINOR — Nothing tells the client to give up a silent stream

- **Statement.** The server marks a machine offline after 30 seconds without a heartbeat (`events.proto:98-100`). `HeartbeatAck` exists so that traffic moves both ways, but no rule tells the machine what to do when acknowledgements stop.
- **Evidence.** `events.proto:98-111`. Design C8 and C9 name the server's view only.
- **Failure scenario.** A proxy stops passing bytes without closing the connection. The server drops the machine after 30 seconds. The client keeps sending heartbeats into the dead connection, receives no error, and does not redial, so the machine stays offline until the owner restarts it.
- **Fix.** Add one sentence to `Heartbeat` or `HeartbeatAck`: a machine that receives no `HeartbeatAck` for 30 seconds ends the stream and dials again. Design: C9.

## Grade

**7 out of 10.** The files equal the plan, the shape tests are strong, the worked examples are checked by computation, and both required commands pass. The grade falls because the rules are held by no test (post-s0 1) and because the one definition that clients will build on, the word "final", is ambiguous (post-s0 2).
