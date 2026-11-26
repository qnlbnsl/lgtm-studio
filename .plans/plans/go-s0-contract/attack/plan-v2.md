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
- **Events arrive with no gap, and a confirmation covers only what the server saved.** The server takes only the next number, saves the report and its count in one step, and only then confirms. A copy is confirmed again, and a gap ends the stream so the machine sends again from the server's count. (architect-r1 01, logic-r1 01, critic-r1 5, threat-r1 3)
- **A command sent twice runs once, and its answer still counts.** A machine that sees a command again sends its recorded answer under the new attempt number, so the server never ignores it as old. The server's clock decides when a command expires, and a command that stopped halfway answers that its effect is unknown. (architect-r1 07, logic-r1 02, logic-r1 03)
- **Removing a machine does not wait for the machine.** The server deletes the machine's key and ends its stream. A notice to the machine is a courtesy. A machine that the server no longer knows stops trying once the server has proved itself. (architect-r1 02, logic-r1 04, threat-r1 1)
- **Only the newest proven stream of a machine counts.** An older stream that ends late cannot mark the machine offline. (logic2-r1 4, logic-r1 06)
- **Changes of a machine's standing carry a revision number.** An approval sent again after a rename cannot bring the old name back.
- **Restarting a client is a command.** The comment of `ReconnectClient` changes to match the owner's decision. The wire format changes only by an added field, so the revision stays at 1 in this sprint. The meaning of the call changes for the website, which is point 1 for the owner. (logic2-r1 1)
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
9. The orchestrator merges the sprint. From then on the contract is frozen, and a change to it goes back to phase 3 of the build loop.

Items 3 to 6 run one after another because each one regenerates the whole of `gen/`, so two at once would always collide there.

## Points for the owner

Review round 1 raised four choices that the design does not settle.
The plan does not decide them, and sprint 0 can be built while they wait.
Each one must be settled before sprint 3, where the Server and Client teams build both ends of the link.

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

- **Option A:** keep the layout. The hop behind the web server stays inside the compose stack of one machine, and the signed challenge is the proof of the machine. The cost is that a process on that machine which can read that hop can change commands and reports.
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
| `Hello.acked_event_number`, field 4 | `link.proto` | The highest event number a server acknowledged to the machine. The Client Link writes it from its outbox's mark, and the Server Link reads it to raise a mark that a restored database lost. UNREVIEWED MECHANISM: no reviewer proposed this field. It answers the case of threat-r1 3 in which the server's mark is behind the machine's. |
| `COMMAND_OUTCOME_INTERRUPTED`, value 5 | `commands.proto` | A command that started and stopped before its end was recorded. The Client Link writes it for a receipt that the client-local plan recovers as `interrupted`, and the Server Machines team shows it. (architect-r1 07, logic-r1 03, critic-r1 6) |
| `Heartbeat.cpu_percent` and `memory_percent`, now `optional` | `events.proto` | Absent means that the machine could not measure its load. The Client Link writes them, and the Server Link copies them into the website's optional load fields. (logic-r1 10) |
| `src/contracts/testdata/join-token.json` | item 5 | A worked example of a join token. |
| `src/contracts/testdata/link-proof.json` | item 6 | Worked examples of both proof texts and signatures. |
| The error codes of the link, and what a client does with each | `link.proto`, `LinkService` comment | `permission_denied` for a refused join token; `unauthenticated` for an unknown or removed machine or a bad signature; `failed_precondition` for a link revision the server does not speak; `invalid_argument` for a message out of order or one that breaks a value rule; `deadline_exceeded` when `Hello` or `Proof` is more than 10 seconds late; `aborted` for a stream that a newer one replaced; `out_of_range` for an event that leaves a gap; `unavailable` for a server that cannot read its database. The comment's table says which codes make a client stop and which make it retry within 15 seconds. The Server Link writes the codes, and the Client Link and the Client enrollment read them. (logic2-r1 6, logic-r1 09) |
| The value rules of the link | `link.proto`, file comment | Canonical UUIDs, exact lengths of nonces, keys, signatures and hashes, link revision 1, counters from 1, the fields that must be present, and the length limits of strings. Both link teams validate against them. (logic-r1 09, critic-r1 10, logic2-r1 10, threat-r1 11) |
| `ReconnectClient` | `clients.proto` comment | Restarts a connected client. A client that is not connected is refused with `failed_precondition`. How the website follows is point 1 for the owner. |
| `CreateClientTokenResponse.token` | `clients.proto` comment | The text of a `JoinToken`. The Server Machines team writes it, and the website and the installer only pass it on. (logic2-r1 7) |

**The description format that the tests compare**

The tests of items 3 to 6 compare the generated descriptors with files in `src/contracts/testdata/golden/`.
The test writer of item 3 writes the helper `describe` in `src/contracts/describe_test.go`, with this behaviour:

1. `describe(t *testing.T, golden string) (got, want string)` reads the file `golden`. Its whole content is `want`.
2. Each line of `want` that starts with `rpc `, `message ` or `enum ` names one descriptor by its full name, which is the line's second word.
3. For each such name, in order, it looks the descriptor up with `protoregistry.GlobalFiles.FindDescriptorByName`. A name it cannot find adds the line `missing <name>` to `got`. It never stops the test.
4. A method adds one line: `rpc <full name> <input> <output>`, where each of `<input>` and `<output>` is the message's full name, preceded by `stream ` when that side streams.
5. A message adds the line `message <full name>`, then one line for each field in declaration order: two spaces, the number, the name, and the type. The type is the full name of a message or enum, or the scalar kind as `protoreflect.Kind.String()` writes it, such as `uint64`. Then, in this order and each preceded by a space: `repeated` for a list, `optional` for a field with the keyword `optional`, and `oneof=<name>` for a field of a oneof that is not synthetic.
6. An enum adds the line `enum <full name>`, then one line for each value: two spaces, the number and the name.
7. After its fields or values, a message or an enum adds one line for each reserved range, `  reserved <start>-<end>`, and one line for each reserved name, `  reserved <name>`. No golden file of this plan has such a line, so a builder who adds a reserved range or name fails the test. A field line without `repeated` or `optional` is a single field. (architect-r1 12)
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
- **`go_package`:** `github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1;lgtmv1` in every file of `lgtm/v1`, as in `draft/existing-files.diff`.
- **`package.json`:** `generate` stays `buf generate`. `check` becomes `buf lint && buf generate && go test ./... && git diff --exit-code -- gen && test -z "$(git status --porcelain --untracked-files=all -- gen)"`. The export map stays as it is.

**Tests written first.** Each runs at the repository root after item 1. The first fails on the starting commit.
The stale probe, the untracked probe and the guard probe run in a separate checkout of the item's commit, made with `git worktree add` in a new folder from `mktemp -d`, so that no probe can leave a changed or new file in the builder's checkout. (critic-r1 8)
Each probe first runs the check on the clean checkout, which must exit 0.

| Command | Expected result |
|---|---|
| `test -f src/contracts/gen/go/lgtm/v1/lgtmv1connect/clients.connect.go` | Exit status 0. |
| `pnpm --filter ./src/contracts check` | Exit status 0. |
| Stale probe: add the line `// probe` to the top comment of `src/contracts/proto/lgtm/v1/session.proto`, and run `pnpm --filter ./src/contracts check` | Exit status 1. `git diff --name-only -- src/contracts/gen` lists `session_pb.js`, so the tracked diff is the step that failed. |
| Untracked probe: add the file `src/contracts/proto/lgtm/v1/probe.proto` with the lines `syntax = "proto3";`, `package lgtm.v1;`, `option go_package = "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1;lgtmv1";` and `message Probe {}`, and run `pnpm --filter ./src/contracts check` | Exit status 1. Before it, `buf lint` and `buf generate` in `src/contracts` exit 0, and `git status --porcelain --untracked-files=all -- src/contracts/gen` lists `probe.pb.go`, `probe_pb.js` and `probe_pb.d.ts` as new. (architect-r1 10, critic-r1 8, logic-r1 12) |
| Guard probe: in the same checkout with `probe.proto`, run the check without its last step, `cd src/contracts && buf lint && buf generate && go test ./... && git diff --exit-code -- gen` | Exit status 0. The check therefore fails on the untracked probe only because of its guard. A run on 2026-11-26 in a scratch copy gave 0 for the clean check, 1 for the probe with the guard, and 0 for the probe without it. |
| `go build ./...` and `go mod tidy -diff` | Exit status 0 for each. |
| `git diff -U0 master -- src/contracts/gen/lgtm \| grep '^[-+][^-+]' \| grep -vc 'fileDesc('` | `0`, because only the embedded descriptor lines changed. |
| `pnpm --filter ./src/frontend typecheck`, `pnpm --filter ./src/frontend build` and `pnpm --filter ./src/mock-server typecheck` | Exit status 0 for each. |

**Acceptance:** every command above gives its expected result.
The root `check` script calls neither the contract check nor the Go check. The Contract team's reviewer runs both on the commit of each item from 2 to 8, and the orchestrator's gate on the sprint's merge commit runs them again. (architect-r1 10)

**Item 3. The shared shape of live lists**

Every `Watch` call gets one written shape, `WatchClients` gains its feed position, and a test holds every later `Watch` call to the shape.

- **Files:** creates `src/contracts/proto/lgtm/v1/common.proto`. Changes `src/contracts/proto/lgtm/v1/clients.proto`: it imports `lgtm/v1/common.proto`, adds `FeedPosition position = 4;` to `WatchClientsResponse` after the oneof, and replaces the comments of `ReconnectClient`, `CreateClientToken`, `RevokeClientToken`, `Client.hosts_server`, `Client.address`, `CreateClientTokenResponse.token` and `CreateClientTokenResponse.expires_at`. Only comments change in those, so no test sees them, and the description of each new comment is in "Rules that item 3 writes" below. It regenerates `src/contracts/gen/`. The test writer creates `src/contracts/describe_test.go`, `src/contracts/watchshape_test.go`, `src/contracts/common_test.go` and `src/contracts/testdata/golden/common.txt`.
- **Needs:** item 2.
- **Work area and complexity:** `contracts`, medium. It adds one file and one field, and the new part is the shape test.
- **Scope:** `contract/contracts`.
- **Statements:** D5, R5, D16 (the comment of `ReconnectClient`), C5 (revoking a used token), C6 (a change reaches the page without a reload), C7 (the projected fields of a machine), R9 (no stored copy of a token), and the design's sections "How every `Watch` call works" and "Safe retries".
- **Risky:** yes, because the position orders changes. The mutation that must turn the tests red: remove `FeedPosition position = 4;` from `WatchClientsResponse` and generate again. A second mutation: change `FeedPosition.value` to `uint32`.
- **Text of the files:** as in `draft/common.proto` and the `clients.proto` part of `draft/existing-files.diff`.
- **Rules that item 3 writes:**
  - The first message of a `Watch` call carries the highest position its records include, or 0 when no change is visible yet. Each later message carries a higher position. A Go server sets the position on every message, and an absent position comes from an older server or the sample server. The Server Store writes the position, and no reader uses it yet. (logic-r1 11)
  - `CreateClientToken` is never replayed from a stored reply for `Lgtm-Request-Key`, because the reply holds the token's secret. Each call makes a new token. The Server Machines team and the Server Store's request-key layer follow it. (threat-r1 4)
  - `RevokeClientToken` removes a machine that used the token and still waits for approval, and it leaves an approved machine alone, as the sample server already does. (logic-r1 05, threat-r1 6)
  - `CreateClientTokenResponse.token` holds the text of a `JoinToken`, and `expires_at` is 15 minutes after the token was made. (logic2-r1 7)
  - `Client.hosts_server` is set by the server when the machine joined with a token from the server's own command on that machine, and never from anything a client says. The Server Machines team and the Stack team's installer follow it. (critic-r1 9)
  - `Client.address` is the network address that the machine's connection came from, for showing only, and empty when the server cannot tell. (architect-r1 11)
- **Not a comment-only change for the website:** the new meaning of `ReconnectClient` breaks the one use of it on today's Machines page. Sprint 0 builds no server, so nothing breaks in this sprint, and the revision stays at 1 here. Point 1 for the owner decides how the page follows. (logic2-r1 1)

**Tests written first.** The test files are package `contracts_test` and import `_ "github.com/qnlbnsl/lgtm-studio/src/contracts/gen/go/lgtm/v1"`, which exists after item 2.

| Test | What it asserts |
|---|---|
| `TestEveryWatchCallHasTheSharedShape` in `watchshape_test.go` | For every method of every service in package `lgtm.v1` whose name starts with `Watch`, except the two in the waiting list: the server streams and the client does not; the reply has a oneof named `change`; field 1 is `all`, of a message type, in `change`; field 2 is `saved`, of a message type, in `change`; field 3 is `removed_id`, a `string`, in `change`; field 4 is `position`, of type `lgtm.v1.FeedPosition`, in no oneof; the reply has no other field; the type of `all` has exactly one field, which is a list of the type of `saved`. It also asserts that `lgtm.v1.ClientService.WatchClients` was among the methods checked. On the starting code it fails, because `WatchClientsResponse` has no field 4. |
| `TestOnlyMapAndPreviewsWaitForTheShape` in `watchshape_test.go` | The waiting list is exactly `lgtm.v1.MapService.WatchMap` and `lgtm.v1.PreviewService.WatchPreviews`. Each exists in the registry. Neither has a oneof named `change`, so the list must shrink when either moves to the shape. |
| `TestCommonMatchesItsDescription` in `common_test.go` | `describe` of `testdata/golden/common.txt` equals the file. On the starting code it fails with the line `missing lgtm.v1.FeedPosition`. |

The content of `src/contracts/testdata/golden/common.txt`:

```text
message lgtm.v1.FeedPosition
  1 value uint64
```

**Acceptance:**

| Observation | Expected result |
|---|---|
| `go test ./src/contracts/ -run 'Watch\|Common' -v` | Every named test passes. |
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
  - The server sends a pending command on each new current stream until it holds a final result. An attempt after the expiry only collects a recorded result. With no final result 24 hours after the expiry, the server records `COMMAND_OUTCOME_EXPIRED` itself.
  - Every outcome except `COMMAND_OUTCOME_UNSPECIFIED` is final, and the server never sends that command again.
  - The server's clock decides expiry. The server reads `Welcome.server_time` after `Proof` verified, and the machine counts the server's time as that value plus the time its monotonic clock measured since it sent `Proof`. That count is never behind the server's clock, so a machine never starts a command that the server counts as expired.
  - The machine writes a receipt to disk before it starts the work, and it keeps the receipt at least 24 hours after the expiry. The client-local plan's default of 24 hours already gives this.
  - A machine that holds a final receipt sends the recorded result under the attempt it just received, also after the expiry, and does not run the command again. A machine with no receipt whose count is at or past the expiry answers `COMMAND_OUTCOME_EXPIRED`.
  - A command still running when a newer attempt arrives sends its result under the newest attempt it received.
  - The server accepts a result only from the current stream of the command's machine, and only for the attempt it sent last. It ignores a copy of a stored result, a result for an older attempt and a result for a later attempt. It stores `COMMAND_OUTCOME_UNSPECIFIED` and an outcome it does not know as `COMMAND_OUTCOME_FAILED`. (logic2-r1 10)
  - A receipt that the client-local plan recovers as `interrupted` is sent as `COMMAND_OUTCOME_INTERRUPTED`, which is final. A new action after it needs a new command.
  - A machine answers a command whose action it does not know with `COMMAND_OUTCOME_UNSUPPORTED`.
  - `ScanHarnesses` is done only after its `HarnessesFound` event is on disk, and it fails when the buffer refused the event. (critic-r1 6)
  - `Restart`: the machine writes its receipt with `COMMAND_OUTCOME_DONE` to disk, then sends the result, and then ends. The outcome means that the machine accepted the restart, and the new process shows itself with a new stream. A `Restart` that already has a receipt is answered from it and not run again. (logic2-r1 3, threat-r1 8)
  - `CommandResult.message` is at most 200 bytes, and the client writes it from its own fixed sentences, never from the output of another program. (threat-r1 11)
- **Rules of the standing** (logic-r1 04):
  - A machine applies a `MachineState` only when its `revision` is higher than the one it applied last. The same revision with the same content changes nothing. The same revision with other content is a fault of the server, and the machine keeps what it holds.
  - Revisions start at 1 for each machine id. The machine applies `Welcome.state` before any command of that stream.
  - A machine treats `APPROVAL_UNSPECIFIED`, an approval value it does not know and a missing state as waiting. (logic2-r1 10)
  - `APPROVAL_REMOVED` is a courtesy that the server may send before it ends the stream, and nothing waits for its delivery.
- **Rules of events** (architect-r1 01, critic-r1 5, logic-r1 01, threat-r1 3, logic2-r1 9). The rule of the draft that let numbers have gaps and let an acknowledgement cover dropped events is gone, because it let an acknowledgement delete an event that the server never applied.
  - One sender on the machine sends events on the current stream in ascending number: first every kept event above `Welcome.last_event_number`, then each new one.
  - A machine never skips or reuses a number. It keeps its highest number also when it keeps no event, and its next number is higher than that and higher than `Welcome.last_event_number`. (logic2-r1 2)
  - A full buffer refuses an event before the event takes a number. The machine then sends its newest `Facts` or `HarnessesFound` again once there is room.
  - The server applies an event only when its number is its mark plus one. It commits the event's effect and the new mark in one transaction, and sends `EventAck` only after that commit.
  - An event at or below the mark is a copy, and the server sends its mark again. An event above the mark plus one ends the stream with `out_of_range`, and the mark stays.
  - An event with no report, or with a report that this server does not know, is consumed: the mark moves past it and nothing else changes. A newer client therefore never sends the same event forever.
  - An event whose report breaks a value rule ends the stream with `invalid_argument`, and the mark stays.
  - `EventAck.through` and `Welcome.last_event_number` are the same committed mark.
  - After every `Welcome` in which it may send events, the machine sends its current `Facts` and `HarnessesFound` as new events, so a server that lost rows gets them back. (logic2-r1 2, logic2-r1 11)
- **Rules of heartbeats:**
  - A heartbeat is never numbered or kept. The machine sends one every 10 seconds. The server answers each heartbeat of the current stream with `HeartbeatAck`.
  - The server marks the machine offline and ends the stream after 30 seconds without a heartbeat on the current stream, measured by its monotonic clock. (logic-r1 06)
  - Both load fields are optional and run from 0 to 100. Absent means that the machine could not measure it, and a heartbeat without load still shows that the machine is there. (logic-r1 10)
- **Rules of facts:** the server shows `Facts` and trusts nothing in them. Strings keep the limits of their comments. The website's `memory_gigabytes` is `memory_bytes` divided by 2^30 and rounded to the nearest whole number. The server counts the harnesses as scanned once it holds one `HarnessesFound`. An approved machine shows as joining until the server holds a `Facts` and a `HarnessesFound` sent on its current stream, and as online after that. (architect-r1 11, threat-r1 11, logic2-r1 11)
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
  4 message string
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
  - One transaction checks the secret against the verifier, uses up the token, binds it to the machine's key and creates the waiting machine. A failure before the commit changes nothing and returns `unavailable`. (architect-r1 06, critic-r1 3, logic-r1 05, threat-r1 6, logic2-r1 8)
  - A replay returns the first reply and creates nothing. A replay has the same token id, a secret that verifies and the same machine key, arrives before the token expires, and finds the machine that the token created still there. The host name of a replay is not read.
  - Every other request is refused with `permission_denied` and one sentence. The server checks, in this order, for an unknown token, an expired token, a revoked token, a token bound to another key, and a token whose machine was removed.
  - Revoking a used token removes its machine while the machine waits, and leaves an approved machine alone.
  - The client stores its key before its first `Join`, sends the same key on every retry, and retries any code other than `permission_denied` and `invalid_argument` until the token expires.
  - The token's text is a secret. No log, board or page other than the one that made it holds it.
  - `JoinToken.address` is the only address the client dials. A client refuses an address on its command line that differs from it, after both are written in lowercase without the default port. It follows no redirect. (architect-r1 04, critic-r1 1, logic-r1 08, logic2-r1 7)
  - A client refuses an address that is neither `https` nor a loopback address over `http`, and an `https` address without `certificate_key_sha256`.
  - The client checks `certificate_key_sha256` before it sends the secret, and again on every stream. The server makes every token from one configured address and pin, whether the Machines page or its own command asks, and it refuses to make an `https` token without a pin. (architect-r1 05, critic-r1 4, threat-r1 10)
  - The client refuses a server whose public key does not hash to `server_key_sha256`, and it keeps that key and trusts no other.
  - `JoinRequest.host_name` is at most 255 bytes, and the server shows it as the machine's own claim. (threat-r1 11)
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
- **Risky:** yes, because it carries the proof of both ends and the order of the stream. The mutation that must turn the tests red: in `testdata/link-proof.json`, compute `server_text` and `server_signature` with the server nonce before the machine nonce. A second mutation: remove `last_event_number` from `Welcome`. A third mutation: delete the comment line above `Restart restart = 12;` in `commands.proto`, which must make `TestEveryLinkRuleIsWritten` fail. (logic2-r1 12)
- **Text of the file:** as `draft/link.proto`. The comments state these rules.
- **The handshake** (logic-r1 09, critic-r1 10, architect-r1 09):
  - The machine sends `Hello` and then `Proof`. The server sends `Challenge` and then `Welcome`.
  - The server answers every well-formed `Hello` with a `Challenge` before it looks the machine up. It then ends the stream with `unauthenticated` when the machine id is unknown or removed, and when the machine's signature does not verify. UNREVIEWED MECHANISM: the reviewers asked for a removal refusal that a client can trust, and logic-r1 04 proposed a distinct code backed by a record of the removed id. This plan lets the verified `Challenge` make the refusal trustworthy instead, so the server needs no record of removed ids.
  - Before `Welcome`, a message of another kind, an empty message or a message that breaks a value rule ends the stream with `invalid_argument`.
  - The server ends the stream with `deadline_exceeded` when `Hello` is not there 10 seconds after the stream opened, or `Proof` 10 seconds after it sent `Challenge`. The two times are measured apart.
  - The machine ends the stream and retries when `Challenge` is not there 10 seconds after it sent `Hello`, when `Welcome` is not there 10 seconds after it sent `Proof`, and when the server's signature does not verify.
  - A link revision the server does not speak ends the stream with `failed_precondition`.
  - The proof texts, as in the table of public names.
- **The value rules** in the file comment of `link.proto`: canonical UUIDs, exact lengths of nonces, keys, signatures and hashes, link revision 1, counters from 1, the fields that must be present, and the limits of strings. A message that breaks one is refused with `invalid_argument`. (logic-r1 09, critic-r1 10, logic2-r1 10, threat-r1 11)
- **What a client does with each code**, in the table of the `LinkService` comment. `permission_denied` from `Join`, `failed_precondition` from `Open`, and `unauthenticated` from `Open` after a verified `Challenge` make the client stop with one sentence for the owner. Every other code makes it retry within 15 seconds with its key and files kept. A server that cannot read its database answers `unavailable`, never `unauthenticated`. UNREVIEWED MECHANISM: logic2-r1 6 asked that `unauthenticated` retry and that `aborted` reconnect at once. This plan stops on a verified `unauthenticated`, as architect-r1 02, critic-r1 7 and threat-r1 1 ask, and retries `aborted` after the normal wait, so two processes with one key do not replace each other without pause.
- **After `Welcome`:**
  - Either end may send any of its other messages, and events keep the order of item 4.
  - A receiver ignores a message of a kind it does not know. Wave 2 adds new kinds with numbers not used yet, and no number is reserved.
  - The machine ends the stream and dials again when it hears nothing for 30 seconds. `HeartbeatAck` makes that silence a signal. (logic2-r1 6, critic-r1 7)
- **The current stream** (architect-r1 09, logic-r1 06, logic2-r1 4, threat-r1 1):
  - A stream becomes its machine's current stream only after its `Proof` verified and the server read the machine's record again. The older stream then ends with `aborted`.
  - Only the current stream changes what the server stores about its machine: its presence, its events, its command results and its connection token. A stream that is no longer current changes nothing, also when it ends.
  - `Welcome` marks the machine online. The end of the current stream, and 30 seconds without a heartbeat on it, mark the machine offline.
  - `Welcome.connection_token` ends when its stream ends or stops being current. Later calls beside the stream present it in the header `Lgtm-Link-Token`.
- **Removal** (architect-r1 02, logic-r1 04, threat-r1 1, critic-r1 7):
  - When the removal of a machine commits, its key is gone, so no new stream of it verifies.
  - The server then ends the machine's current stream, and with it the connection token. It may send `SetState` with `APPROVAL_REMOVED` first, and nothing waits for that message.
  - The next stream of a removed machine gets a verified `Challenge` and then `unauthenticated`, so the client stops and tells the owner to join it again. A broken connection never makes the client stop, and it keeps its key and files.
- **Event marks** (architect-r1 01, critic-r1 5, logic-r1 01, threat-r1 3):
  - `Welcome.last_event_number` is the server's committed mark, the same value that `EventAck.through` gives. The machine sends again every event it keeps above it, in order.
  - When the mark is higher than the machine's highest number, the machine forgets its kept events up to it and numbers its next event one higher. The client-local plan's `Outbox.Ack` beyond `Next()` already does this. (logic2-r1 2)
  - `Hello.acked_event_number` carries the machine's own mark. A server whose mark is lower, because its database was restored, raises its mark to that number before it sends `Welcome`. A machine that claims a higher number can only skip its own reports. UNREVIEWED MECHANISM, as in the table of public names.
- **A waiting machine** may send heartbeats and events, and the server sends it no command except `SetState`. This rule of the draft stays until the owner settles point 3, which may narrow it. (architect-r1 08, threat-r1 7)
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
| `TestEveryLinkRuleIsWritten` | It reads `proto/lgtm/link/v1/link.proto`, `commands.proto` and `events.proto` as text, because the generated Go keeps no comments. Every line that declares a service, method, message, enum, oneof, field or enum value has a line starting with `//` as the nearest line above it that is not empty. The one exception is an enum value named `..._UNSPECIFIED = 0`. `link.proto` names each of `permission_denied`, `unauthenticated`, `failed_precondition`, `invalid_argument`, `deadline_exceeded`, `aborted`, `unavailable` and `Lgtm-Link-Token`, and `events.proto` names `out_of_range`. Before the files exist, the test fails on the first file it cannot read. A prototype passed on the drafts on 2026-11-26, and failed with `commands.proto:42 has no comment above it` when the comment above `Restart restart = 12;` was deleted. (logic2-r1 12) |
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
  4 acked_event_number uint64
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
- **What changes:** the table of files gains `common.proto` and a second table for the package `lgtm.link.v1` with `link.proto`, `commands.proto` and `events.proto`. "What is here" names `gen/go/` and `testdata/`. The rules name the shared `Watch` shape, the feed position, the link revision, the header `Lgtm-Link-Token`, and that generating needs Go 1.26 or newer. The section "Not here yet" no longer says that the calls between the server and the client are not written. The sentence that sends a reader to `SECURITY.md`, which the owner removed on 2026-11-25, is removed. The rule that every call needs a session becomes a rule of the package `lgtm.v1`, and a new rule says that `LinkService.Join` needs a join token and `LinkService.Open` the machine's key, and that neither takes a website session. (critic-r1 11)

**Tests written first.** Each absence check succeeds when the text is absent, because `grep -c` exits 1 when it finds nothing. (critic-r1 11)

| Command | Expected result |
|---|---|
| `test -f src/contracts/README.md` | Exit status 0. |
| `grep -q 'common.proto' src/contracts/README.md`, and the same for `link.proto`, `commands.proto`, `events.proto`, `gen/go`, `Lgtm-Link-Token`, `Go 1.26`, `link revision 1` and `LinkService` | Exit status 0 for each. |
| `! grep -q 'The calls between the server and the client are not written' src/contracts/README.md` | Exit status 0. |
| `! grep -q 'SECURITY.md' src/contracts/README.md` | Exit status 0. |
| `! grep -q '^- Every call needs a session' src/contracts/README.md` | Exit status 0, because the session rule now names the package `lgtm.v1`. |
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
| 6 | Every name and number of the link is fixed. | `TestEventsMatchItsDescription`, `TestCommandsMatchItsDescription`, `TestJoinMatchesItsDescription`, `TestOpenMatchesItsDescription` and `TestEveryLinkNameIsDescribed` pass. `TestEveryLinkRuleIsWritten` passes, so every rule has its written comment. |
| 7 | Both teams can test the proof and the token against the same bytes. | `TestProofExamplesHoldTogether` and `TestJoinTokenExampleDecodes` pass. |
| 8 | One command checks all Go code. | `scripts/check-go.sh` exits 0 after item 6. |
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
| The clocks of a machine and the server differ, so a command expires early or late. | medium | The server's clock decides. The machine counts the server's time from `Welcome.server_time` and its monotonic clock, a count that is never behind the server's clock, so it never starts an expired command. A recorded result is sent also after the expiry. (logic-r1 02) |
| An acknowledgement deletes an event that the server never applied. | high | The server takes only the next number, commits the effect and the mark together, and acknowledges only after the commit. A gap ends the stream. Post-build check 3 tests it through a crash. (architect-r1 01) |
| An old stream that ends late marks a connected machine offline. | medium | Only the current stream changes the machine's record, and a stream becomes current only after its proof. Post-build check 4 tests it. (logic2-r1 4) |
| A removed machine keeps trying to connect, or a broken connection makes a machine stop for good. | medium | The client stops only on a refusal that follows a verified `Challenge`. Every other failure keeps the key and retries within 15 seconds. Post-build check 5 tests both. (critic-r1 7) |
| The owner leaves one of the four points open past sprint 2. | medium | Sprint 0 does not depend on them. The orchestrator raises them before the Server and Client teams plan sprint 3. |
| A renewed certificate with a new key cuts off every machine that joined. | medium | The token pins the key, so a renewal that keeps the key keeps working. Replacing the key is not in wave 1, and a machine then joins again with a new token. The result file reports it. (threat-r1 10) |
| The test writer copies a description file wrongly, and the builder must match a wrong file. | medium | The description files here were produced by a program from the drafts, which passed lint and generated. The builder reports a mismatch to the lead and never edits a frozen test. |
| A new field breaks a website that the plan did not consider. | low | Message fields are optional in the generated TypeScript (`position?: FeedPosition`), and items 2 and 3 run the website's type check and build. |

**Verify:** the gating tests are the commands of items 1, 2, 7 and 8, and the Go tests of items 3 to 6, all in `go test ./src/contracts/`. The environment check is the sign-in of row 3 of "Done when": two commands against the running sample server, and one person signing in on the website. No device is needed.

**Rollout detail:** step 1 of the rollout runs `go mod init github.com/qnlbnsl/lgtm-studio`, then sets the `go` line to `1.26.0`, then runs `go get -tool google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12 connectrpc.com/connect/cmd/protoc-gen-connect-go@v1.21.1`. Step 1 ends with `go mod tidy`. Steps 2 and 4 to 7 each end with `pnpm --filter ./src/contracts generate` and `go mod tidy`. Step 9 needs nothing more than the merge.

</details>

<details><summary><b>Agreements with other teams, from review round 1</b></summary>

The contract only writes these rules down. Each row names the team that writes the behaviour and the team that reads it. The orchestrator copies each row into the brief of the team that writes it.

| Rule | Writes it | Reads it |
|---|---|---|
| One ordered sender of events on the current stream. The newest `Facts` or `HarnessesFound` is sent again once a full buffer has room. After every `Welcome`, the current `Facts` and `HarnessesFound` are sent as new events. (architect-r1 01, logic2-r1 2) | Client Link, with the outbox of `go-s1-client-local` | Server Link |
| The server applies only the next event number, commits its effect and the mark in one transaction, and acknowledges after the commit. It raises its mark to `Hello.acked_event_number` when that is higher. (critic-r1 5, threat-r1 3) | Server Link, with the store of `go-s1-server-store` | Client Link |
| A receipt recovered as `interrupted` is sent as `COMMAND_OUTCOME_INTERRUPTED`. A running command answers under the newest attempt. A `Restart` receipt is on disk before its result is sent. (architect-r1 07, logic2-r1 3) | Client Link, with the receipts of `go-s1-client-local` | Server Link and Server Machines |
| The server keeps one expiry per command, accepts a result only for its current attempt on the current stream, and records `COMMAND_OUTCOME_EXPIRED` itself 24 hours after the expiry. (logic-r1 03, threat-r1 9) | Server Link | Client Link |
| A stream becomes current only after its proof and a fresh read of the machine's record. Only the current stream changes presence, events, results and the connection token. (logic-r1 06, logic2-r1 4) | Server Link | Server Machines, which shows presence |
| Removal deletes the key when it commits, then ends the current stream. The client stops only on `unauthenticated` after a verified `Challenge`. (architect-r1 02, threat-r1 1) | Server Machines and Server Link | Client Link |
| The table of codes in the `LinkService` comment, with the client's 10-second handshake limits, its 30-second silence limit and its retry within 15 seconds. (logic2-r1 6, logic-r1 09) | Server Link writes the codes | Client Link and Client enrollment |
| `Join` uses up the token, binds the key and creates the waiting machine in one transaction, and replays the first reply for the same token and key. Revoking a used token removes a machine that still waits. (architect-r1 06, logic-r1 05) | Server Machines | Client enrollment, the website's pairing dialog |
| `CreateClientTokenResponse.token` holds the `JoinToken` text, and `CreateClientToken` is never replayed from a stored reply. (logic2-r1 7, threat-r1 4) | Server Machines, and the request-key layer of `go-s1-server-store` | The website and the installer, which only pass the text on |
| One configured public address and one certificate pin feed every join token, from the Machines page and from the server's own command. The server refuses to make an `https` token without a pin. (architect-r1 05, critic-r1 4, threat-r1 10) | Stack Web gives the address and the pin; Server Machines and the server's command make the token | Client enrollment, which checks the pin before it sends the secret and on every stream |
| The token's address is the only address the client dials, and a different address on the command line is refused. (architect-r1 04, logic-r1 08) | Client enrollment | Stack Client's installer, the website's pairing dialog |
| The website's pairing dialog shows the design's command, `lgtm-client join <address> <token>`, and the install step of the Go client, before the Go server replaces the sample server. Point 4 for the owner may change the command's form. (critic-r1 1, logic-r1 08) | The orchestrator, with the owner's approval of the visible change | The owner, on the Machines page |
| `Client.hosts_server` comes from a token that the server's own command made on its machine. `Client.address` is the connection's address, for showing only. `memory_gigabytes` is `memory_bytes` over 2^30, rounded. `harnesses_scanned` turns true with the first `HarnessesFound`. (architect-r1 11, critic-r1 9) | Server Machines, with Stack Client's installer for the token | The website |
| The link's two paths, `/lgtm.link.v1.LinkService/Join` and `/lgtm.link.v1.LinkService/Open`, are mounted outside the website's session gate, and the second passes as a long two-way HTTP/2 stream. A route test calls `Join` without a cookie and gets a link code, never the session's `unauthenticated` for a missing cookie. (architect-r1 05, logic-r1 09) | Server Sign-in mounts them; Stack Web passes them on | Client Link and Client enrollment |

</details>

<details><summary><b>Post-build checks</b></summary>

These checks need built programs, so they run after sprint 3 in the orchestrator's combined link test, `src/linktest/`, against a real PostgreSQL. Each one names its scenario and the result that must be seen.

| # | Scenario | Expected result | From |
|---|---|---|---|
| 1 | Commit `Join`, drop its reply, and retry with the key the client saved. Then send `Join` with the same token and another key, with an expired token, with a revoked token, and after the machine was removed. | The retry gets the first machine id, and exactly one waiting machine exists. Each of the four other requests gets `permission_denied`. | architect-r1 P1, critic-r1 check 2, logic-r1, threat-r1 T2 |
| 2 | Join and connect through the real web server in front, over loopback and over an `https` address with a self-signed certificate. Keep the stream idle except for heartbeats for ten minutes. Then try a token whose pin does not match. | The stored pin works after a client restart, and the stream stays open for the ten minutes. The wrong pin fails before the client sends the secret. | architect-r1 P2, logic2-r1 check 8, threat-r1 T1 |
| 3 | Keep events 41 and 42 on the machine, produce 43 during the reconnection, and stop the server once before and once after the commit of 41, before its acknowledgement. Also send an event with a gap, an event of an unknown kind, and start a machine whose event file is empty while the server's mark is 40. Restore the server's database to an older copy while a machine is connected. | Each committed event is applied once, in order, and the machine deletes only the acknowledged prefix. The gap ends the stream with `out_of_range` and nothing is lost. The unknown event is consumed. The empty machine's next event is 41 or higher and is applied. After the restore, the machine's next event is not a gap, and its `Facts` and `HarnessesFound` arrive again. | architect-r1 P3, critic-r1 check 3, logic-r1, logic2-r1 checks 3 and 4, threat-r1 T5 |
| 4 | Open stream B while the server still holds stream A, and let A end 50 milliseconds after B's `Welcome`, and also send a late heartbeat and a late result on A. Then let a proxy stop passing bytes on B without closing it. | The machine stays online on B, and nothing from A changes the record. On the silent B, the server shows the machine offline within 30 seconds, and the client dials again within 30 seconds plus 15. | critic-r1 check 3, logic-r1, logic2-r1 checks 2 and 6, threat-r1 T4 |
| 5 | Remove a machine while it is offline, then start it. Remove another while it is connected, and drop the `SetState` notice on the way. Interrupt only the network of a third machine. | The first two machines log one sentence telling the owner to join them again and stop trying, and neither appears in the list. The third machine keeps its key and connects again within 15 seconds of the failure, with no new join. | architect-r1 P4, critic-r1 check 5, threat-r1 T4 |
| 6 | Keep a scan running across a reconnection, so that attempt 2 arrives while attempt 1 runs. Kill the client after a second scan recorded its start. Kill the client during `Restart`, once before and once after its receipt is on disk. Run a command with the machine's clock five minutes ahead and once five minutes behind. Remove a machine, join it again, and send it its first state. | The first scan's result closes attempt 2 once. The killed scan answers `COMMAND_OUTCOME_INTERRUPTED` and does not run again. The machine restarts once and then settles. No command starts after its expiry by the server's clock, and a command within its expiry runs whatever the machine's clock says. The rejoined machine applies its state of revision 1. | architect-r1 P4, critic-r1 check 4, logic-r1, logic2-r1 checks 1, 5 and 7, threat-r1 T6 |
| 7 | On the real Machines page against the Go server, make a token, paste the page's command on a new machine, approve the machine, and use the page's restart action as point 1 for the owner settles it. | The waiting machine appears without a reload, then shows online with its facts, load and harnesses, and the restart happens once and the machine comes back. | architect-r1 P5, critic-r1 check 2, logic-r1, threat-r1 T7 |
| 8 | On a clean checkout of the sprint's merge commit, generate twice, then add the valid untracked probe of item 2 and run the check with and without its guard. Decode a `WatchClients` reply with and without `position` using the website's old generated code. | The second generation changes nothing. The check fails with the guard and passes without it. The old code reads the same saved machine from both replies. | architect-r1 P6, critic-r1 check 1 |
| 9 | Make and use join tokens with a marked fake secret, then search the database's request-key receipts, the server's and the client's logs, and the client's process list. Connect a waiting machine, try the actions that point 3 for the owner refuses it, and try an owner call with a machine's connection token. | The marked secret appears in none of those places, except the process list if point 4 keeps the argument. The waiting machine gets only what point 3 allows, and the connection token opens no owner call. | threat-r1 T2 and T3 |

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
8. `decisions.md`, rows 1 to 53.
9. The findings of review round 1, in `attack/round-1-findings.md`.

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
| `decisions.md` rows 38 to 51 and 53 | Items 2 to 8; agreements with other teams | closed, or unanswered with its assumption (rows 39 and 42) |
| `decisions.md` row 52 | Points for the owner | unanswered with its assumption, waiting for the owner before sprint 3 |
| Review round 1, 59 findings | `attack/round-1-findings.md`; post-build checks | every finding has one outcome |

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
| `src/contracts/README.md`: a field number is never used again, and a field's type never changes. | Item 3 only adds field 4. No existing field changes. |
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

</details>
