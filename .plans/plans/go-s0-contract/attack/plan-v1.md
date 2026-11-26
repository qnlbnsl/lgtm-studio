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

The website keeps working without a change.
The one field that this plan adds to a message the website reads is optional, so the website ignores it.
The revision number of the contract therefore stays at 1, and no website refuses the server.

Two points need the owner's attention, though neither blocks the work.
The design says that a machine presents a certificate made from its own key, but the encrypted connection ends at the web server in front of the server, so that certificate would never reach the server.
This plan proves a machine by the signed challenge alone, which the design already requires on every connection.
The design's list of what travels in wave 1 also has no way to restart a client, although the owner decided that the Machines page can restart a connected client.
This plan adds a restart command for that.

## Key decisions

- **The shared shape is a rule that a test checks.** Protocol Buffers cannot express "a list of anything", so each live list keeps its own message. Every such message uses the same field names and numbers, and a test fails when one does not. Two existing lists, the codebase map and the previews, do not follow the shape yet; the design moves them later.
- **The place in the change feed is its own small message.** It holds one growing number today, and it leaves room for a later field without breaking anything.
- **The Go code lives in `src/contracts/gen/go/`.** A trial run showed that the tool `buf` keeps the TypeScript there and adds the Go beside it.
- **The join token is a small message written as text.** It carries the token's id and secret, the server's address, and the fingerprints of the server's key and of its certificate's key. Both programs read it with generated code, so no two hand-written readers can disagree.
- **The token pins the certificate's key, not the whole certificate.** A renewed certificate that keeps its key then still works for every machine that joined.
- **A machine proves itself only by signing a challenge.** The design's certificate of the machine adds nothing once encryption ends at the web server. The result file reports this to the orchestrator.
- **A heartbeat is not kept on disk.** Facts and harness reports are numbered and kept until the server confirms them. A heartbeat only says "here now", so a stored one would report an old load. The server answers each heartbeat, so traffic moves both ways and the web server in front does not cut the connection.
- **A command sent twice runs once, and its answer still counts.** A machine that sees a command again sends its recorded answer under the new attempt number, so the server never ignores it as old.
- **Changes of a machine's standing carry a revision number.** An approval sent again after a rename cannot bring the old name back.
- **Restarting a client is a command.** The comment of `ReconnectClient` changes to match the owner's decision. Only a comment changes, so the revision stays at 1.
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
| `src/contracts/testdata/join-token.json` | item 5 | A worked example of a join token. |
| `src/contracts/testdata/link-proof.json` | item 6 | Worked examples of both proof texts and signatures. |
| The error codes of the link | `link.proto` comments | `permission_denied` for a refused join token; `unauthenticated` for an unknown machine or a bad signature; `failed_precondition` for a link revision the server does not speak; `invalid_argument` for a message out of order; `deadline_exceeded` when `Hello` or `Proof` is more than 10 seconds late; `aborted` for a stream that a newer one replaced. |
| `ReconnectClient` | `clients.proto` comment | Restarts a connected client. A client that is not connected is refused with `failed_precondition`. |

**The description format that the tests compare**

The tests of items 3 to 6 compare the generated descriptors with files in `src/contracts/testdata/golden/`.
The test writer of item 3 writes the helper `describe` in `src/contracts/describe_test.go`, with this behaviour:

1. `describe(t *testing.T, golden string) (got, want string)` reads the file `golden`. Its whole content is `want`.
2. Each line of `want` that starts with `rpc `, `message ` or `enum ` names one descriptor by its full name, which is the line's second word.
3. For each such name, in order, it looks the descriptor up with `protoregistry.GlobalFiles.FindDescriptorByName`. A name it cannot find adds the line `missing <name>` to `got`. It never stops the test.
4. A method adds one line: `rpc <full name> <input> <output>`, where each of `<input>` and `<output>` is the message's full name, preceded by `stream ` when that side streams.
5. A message adds the line `message <full name>`, then one line for each field in declaration order: two spaces, the number, the name, and the type. The type is the full name of a message or enum, or the scalar kind as `protoreflect.Kind.String()` writes it, such as `uint64`. Then, in this order and each preceded by a space: `repeated` for a list, `optional` for a field with the keyword `optional`, and `oneof=<name>` for a field of a oneof that is not synthetic.
6. An enum adds the line `enum <full name>`, then one line for each value: two spaces, the number and the name.
7. `got` ends with a newline, like each golden file.

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

| Command | Expected result |
|---|---|
| `test -f src/contracts/gen/go/lgtm/v1/lgtmv1connect/clients.connect.go` | Exit status 0. |
| `pnpm --filter ./src/contracts check` | Exit status 0. |
| Add the line `// probe` to the top comment of `src/contracts/proto/lgtm/v1/session.proto`, run `pnpm --filter ./src/contracts check`, then undo the line | Exit status 1. |
| Add a file `src/contracts/proto/lgtm/v1/probe.proto` that declares `package lgtm.v1;` and `message Probe {}`, run `pnpm --filter ./src/contracts check`, then delete the file and every generated file named `probe*` | Exit status 1. |
| `go build ./...` and `go mod tidy -diff` | Exit status 0 for each. |
| `git diff -U0 master -- src/contracts/gen/lgtm \| grep '^[-+][^-+]' \| grep -vc 'fileDesc('` | `0`, because only the embedded descriptor lines changed. |
| `pnpm --filter ./src/frontend typecheck`, `pnpm --filter ./src/frontend build` and `pnpm --filter ./src/mock-server typecheck` | Exit status 0 for each. |

**Acceptance:** every command above gives its expected result.

**Item 3. The shared shape of live lists**

Every `Watch` call gets one written shape, `WatchClients` gains its feed position, and a test holds every later `Watch` call to the shape.

- **Files:** creates `src/contracts/proto/lgtm/v1/common.proto`. Changes `src/contracts/proto/lgtm/v1/clients.proto`: it imports `lgtm/v1/common.proto`, adds `FeedPosition position = 4;` to `WatchClientsResponse` after the oneof, and replaces the comment of `ReconnectClient`. It regenerates `src/contracts/gen/`. The test writer creates `src/contracts/describe_test.go`, `src/contracts/watchshape_test.go`, `src/contracts/common_test.go` and `src/contracts/testdata/golden/common.txt`.
- **Needs:** item 2.
- **Work area and complexity:** `contracts`, medium. It adds one file and one field, and the new part is the shape test.
- **Scope:** `contract/contracts`.
- **Statements:** D5, R5, D16 (the comment of `ReconnectClient`), C6 (a change reaches the page without a reload), and the design's section "How every `Watch` call works".
- **Risky:** yes, because the position orders changes. The mutation that must turn the tests red: remove `FeedPosition position = 4;` from `WatchClientsResponse` and generate again. A second mutation: change `FeedPosition.value` to `uint32`.
- **Text of the files:** as in `draft/common.proto` and the `clients.proto` part of `draft/existing-files.diff`.

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
- **Text of the files:** as in `draft/events.proto` and `draft/commands.proto`. Their comments state these rules:
  - The server stores a command before it sends it, and it sends the command again with a higher attempt number after a stream ends without its result.
  - A machine that receives a command it already ran sends the outcome it recorded, with the attempt number of the copy it just received. The server ignores a result for an older attempt.
  - A machine does not run a command after `expires_at` by its own clock, and answers `COMMAND_OUTCOME_EXPIRED`.
  - A machine answers a command whose action it does not know with `COMMAND_OUTCOME_UNSUPPORTED`.
  - `Restart` ends the client program, so that the service of its user starts it again. The machine sends its result before it ends.
  - A machine ignores a `MachineState` whose `revision` is lower than one it already applied.
  - Event numbers start at 1 and grow. A gap means the machine dropped events it could not keep. The server applies an event only when its number is higher than the last one it applied, and drops any other.
  - `EventAck.through` means every event up to and including that number is applied or dropped.
  - A heartbeat is never numbered or kept. The machine sends one every 10 seconds. The server marks the machine offline after 30 seconds without one, and it answers each heartbeat with `HeartbeatAck`.
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
  1 cpu_percent uint32
  2 memory_percent uint32
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
  - The same token with the same machine key returns the same reply until the token expires. The same token with another key is refused.
  - A used, expired, revoked or unknown token is refused with `permission_denied` and one sentence.
  - The token's text is a secret. No log, board or page other than the one that made it holds it.
  - A client refuses an address that is neither `https` nor a loopback address over `http`, and an `https` address without `certificate_key_sha256`.
  - The client refuses a server whose public key does not hash to `server_key_sha256`, and it keeps that key and trusts no other.
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
- **Statements:** D7, D9 and D15 (room for the calls of wave 2), D17, R8, R10, C9, C10, C11, and the design's sections "One connection, dialled by the machine" and "Both ends prove themselves on every connection".
- **Risky:** yes, because it carries the proof of both ends and the order of the stream. The mutation that must turn the tests red: in `testdata/link-proof.json`, compute `server_text` and `server_signature` with the server nonce before the machine nonce. A second mutation: remove `last_event_number` from `Welcome`.
- **Text of the file:** as `draft/link.proto`. The comments state these rules:
  - The machine sends `Hello` and then `Proof`. The server sends `Challenge` and then `Welcome`. After that either end sends any of its other messages, in any order.
  - A message out of this order ends the stream with `invalid_argument`. A `Hello` or `Proof` that has not arrived within 10 seconds ends it with `deadline_exceeded`.
  - An unknown machine id or a signature that does not verify ends the stream with `unauthenticated`.
  - A link revision the server does not speak ends the stream with `failed_precondition`.
  - A new stream of a machine replaces its older one, which ends with `aborted`.
  - The proof texts, as in the table of public names.
  - `Welcome.connection_token` ends when its stream ends. Later calls beside the stream present it in the header `Lgtm-Link-Token`.
  - `Welcome.last_event_number` is the highest event number the server has applied from this machine, or 0. The machine sends again every event it keeps with a higher number, in order.
  - A waiting machine may send heartbeats and events. The server sends it no command except `SetState`.
  - A receiver ignores a message of a kind it does not know. Wave 2 adds new kinds with numbers not used yet, and no number is reserved.
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
- **What changes:** the table of files gains `common.proto` and a second table for the package `lgtm.link.v1` with `link.proto`, `commands.proto` and `events.proto`. "What is here" names `gen/go/` and `testdata/`. The rules name the shared `Watch` shape, the feed position, the link revision, the header `Lgtm-Link-Token`, and that generating needs Go 1.26 or newer. The section "Not here yet" no longer says that the calls between the server and the client are not written. The sentence that sends a reader to `SECURITY.md`, which the owner removed on 2026-11-25, is removed.

**Tests written first.**

| Command | Expected result |
|---|---|
| `grep -c 'common.proto' src/contracts/README.md`, and the same for `link.proto`, `commands.proto`, `events.proto`, `gen/go` and `Lgtm-Link-Token` | At least `1` for each. |
| `grep -c 'The calls between the server and the client are not written' src/contracts/README.md` | `0` |
| `grep -c 'SECURITY.md' src/contracts/README.md` | `0` |
| `node scripts/check-private.mjs` | Exit status 0. |

**Acceptance:** every command above gives its expected result.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1 | `go build ./...` passes at the repository root. | `go build ./...` exits 0. |
| 2 | The contract check passes, and it fails when the generated Go is out of date. | `pnpm --filter ./src/contracts check` exits 0; the two probes of item 2 make it exit 1. |
| 3 | The website still builds and signs in against the sample server. | `pnpm --filter ./src/frontend typecheck` and `pnpm --filter ./src/frontend build` exit 0. With `pnpm --filter ./src/mock-server start` running, `curl -s -X POST -H 'content-type: application/json' -d '{}' http://127.0.0.1:4180/lgtm.v1.SessionService/GetServer` prints JSON (JavaScript Object Notation) with `"contract":1`, and `curl -s -X POST -H 'content-type: application/json' -d '{"password":"sample"}' http://127.0.0.1:4180/lgtm.v1.SessionService/SignIn` prints `{"result":"SIGN_IN_RESULT_SIGNED_IN"}`. A person then runs `pnpm --filter ./src/frontend dev`, opens `https://127.0.0.1:4179/`, types `/` as the server's address, signs in with any password except `wrong`, and sees the Machines page list the sample machines. |
| 4 | Every `Watch` call that the shape covers has it. | `TestEveryWatchCallHasTheSharedShape` and `TestOnlyMapAndPreviewsWaitForTheShape` pass. |
| 5 | The contract revision stays at 1. | The `grep` of item 3 prints `1` for each of the two files. |
| 6 | Every name and number of the link is fixed. | `TestEventsMatchItsDescription`, `TestCommandsMatchItsDescription`, `TestJoinMatchesItsDescription`, `TestOpenMatchesItsDescription` and `TestEveryLinkNameIsDescribed` pass. |
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
| The clocks of a machine and the server differ, so a command expires early or late. | medium | `Welcome.server_time` lets the client measure the difference. The client team decides what it does with it. |
| A renewed certificate with a new key cuts off every machine that joined. | medium | The token pins the key, so a renewal that keeps the key keeps working. Replacing the key is not in wave 1; the result file reports it. |
| The test writer copies a description file wrongly, and the builder must match a wrong file. | medium | The description files here were produced by a program from the drafts, which passed lint and generated. The builder reports a mismatch to the lead and never edits a frozen test. |
| A new field breaks a website that the plan did not consider. | low | Message fields are optional in the generated TypeScript (`position?: FeedPosition`), and items 2 and 3 run the website's type check and build. |

**Verify:** the gating tests are the commands of items 1, 2, 7 and 8, and the Go tests of items 3 to 6, all in `go test ./src/contracts/`. The environment check is the sign-in of row 3 of "Done when": two commands against the running sample server, and one person signing in on the website. No device is needed.

**Rollout detail:** step 1 of the rollout runs `go mod init github.com/qnlbnsl/lgtm-studio`, then sets the `go` line to `1.26.0`, then runs `go get -tool google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12 connectrpc.com/connect/cmd/protoc-gen-connect-go@v1.21.1`. Step 1 ends with `go mod tidy`. Steps 2 and 4 to 7 each end with `pnpm --filter ./src/contracts generate` and `go mod tidy`. Step 9 needs nothing more than the merge.

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
8. `decisions.md`, rows 1 to 37.

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
| `src/contracts/README.md`: a change that breaks an older website raises the contract revision. | Nothing breaks the website, so the revision stays at 1. Item 3 checks it. |
| `src/contracts/README.md`: every message, field and call has a comment. | The drafts have them, and item 8 keeps the README in step. |
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
| — | Not run yet. | The orchestrator runs the review rounds with other models. The planner stopped after the self-check. |

</details>
