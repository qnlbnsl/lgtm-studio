# Scouting — go-s0-contract

This file records what the planner read and ran on 2026-11-26, with file and symbol anchors.
Every path is relative to the repository root.
The experiments ran in throwaway copies outside the repository, and nothing in the repository was changed by them.

## What was read

| Source | What it gave this plan |
|---|---|
| `docs/specs/2026-11-25-go-rewrite-foundation-design.md` | The module path `github.com/qnlbnsl/lgtm-studio` and Go 1.26 ("Where the code lives"). The shared `Watch` shape ("How every `Watch` call works"). Joining, the link, the proof on every stream, commands, events, heartbeats and encryption ("Joining" and "The link between the server and a machine"). Decisions D1 to D23, rules R1 to R11 and checks C1 to C13. |
| `.plans/epics/go-rewrite/epic.md` | The Contract team may edit `src/contracts/`, `go.mod`, `go.sum` and `scripts/`. Sprint 0 ends when `go build ./...` passes and the link is written down. |
| `.plans/plans/go-s0-contract/plan.md` (the brief) and `decisions.md` rows 1 to 5 | The seven work items and the four points to settle. |
| `docs/specs/2026-11-05-build-loop-design.md`, section "4 Build" | A test must fail on its assertion, never on a crash or a bad import. Stubs carry names only. Risky tests are proven by breaking the code on a temporary branch. |
| `docs/issues.md`, issues 1632 to 1635 | Issue 1632 item 3 (`ReconnectClient`), item 4 (the `Watch` shape) and item 10 (a certificate without a public authority). Issue 1633 (where encryption ends). Issue 1634 (test entry points). Issue 1635 (sizes are low, medium and high). |
| `.foundry/go-rewrite/common-planner.md` | The rules of this assignment, and the orchestrator's readings of issues 1633 to 1635. It also states that the owner confirmed the sprints and teams. |
| The planning skill `world-class-plan`, its `templates.md` and `self-check.md` | The template and the ten gates. |
| The model sheet of the harness-sessions plugin, its sections "Work areas" and "Complexity" | The work areas `contracts` and `infra`, and the meaning of low, medium and high. |
| `floorplan/plan.json`, key `files` | The planned contract files: `common.proto`, `link.proto`, `commands.proto`, `events.proto`, and `agent_calls.proto` for wave 2. |
| The briefs of `go-s3-server-machines`, `go-s3-client-link` and `go-s1-client-local` | What the two teams build against the link. The client brief already plans a restart command, and it leaves open what a full event buffer drops. |
| `.plans/scopes/contract/contracts/scope.json` | The scope exists. `scope.py check . contract/contracts` printed `ok: contract/contracts` on 2026-11-26. |

## Anchors in the existing code

| File | Symbol or line | What it shows |
|---|---|---|
| `src/contracts/buf.gen.yaml` | `clean: true`, plugin `protoc-gen-es`, `out: gen` | One plugin today. `clean` empties every output folder before each run. |
| `src/contracts/buf.yaml` | `lint.use: STANDARD`, `breaking.use: FILE` | Lint names. The rule `RPC_REQUEST_STANDARD_NAME` is why the stream's messages are named `OpenRequest` and `OpenResponse`. |
| `src/contracts/package.json` | scripts `generate`, `lint`, `check` | `check` is `buf lint && buf generate && git diff --exit-code -- gen`. It misses untracked generated files. The export map `./*` points at `./gen/lgtm/v1/*_pb.js` only. |
| `src/contracts/README.md` | section "Not here yet" | It says the calls between the server and the client are not written. |
| `src/contracts/proto/lgtm/v1/clients.proto` | `message WatchClientsResponse`, `oneof change` with `all = 1`, `saved = 2`, `removed_id = 3` | The only `Watch` reply that already has the shared shape, without a position. |
| `src/contracts/proto/lgtm/v1/clients.proto` | `rpc ReconnectClient`, comment "asks the server to try an offline client again" | The comment that decision D16 makes wrong. |
| `src/contracts/proto/lgtm/v1/clients.proto` | `message Client`, fields 5 `address`, 6 `workspace`, 7 `os`, 10 `cores`, 11 `memory_gigabytes` | What the server fills from the link's facts. |
| `src/contracts/proto/lgtm/v1/map.proto` | `message WatchMapResponse { CodebaseMap map = 1; }` | Not the shared shape yet. The design says it changes later and raises the revision. |
| `src/contracts/proto/lgtm/v1/preview.proto` | `message WatchPreviewsResponse { repeated Preview previews = 1; }` | Not the shared shape yet, for the same reason. |
| `src/mock-server/src/engine.ts` | `export const CONTRACT = 1;` (line 47), `watchClients` (line 164) yields `{ change }` | The mock server builds replies as init shapes, in which every message field is optional. |
| `src/frontend/src/server/api.ts` | `export const CONTRACT = 1;` (line 15), `WrongServer` (line 46) | The website refuses a server of another revision. |
| `src/frontend/src/server/sync.ts` | `watch()`, `toJoining` (line 25) | The website reads `change.case` only, and it reconnects on any error except `unauthenticated`. A waiting machine shows `os`, `cores` and memory. |
| `src/frontend/src/screens/service.tsx` | line 410, the `Command` text | The website shows `lgtm client join <address> --token <code>`. |
| `pnpm-workspace.yaml` | `catalog: '@bufbuild/buf': 1.75.0` | The `buf` version. It is at `src/contracts/node_modules/.bin/buf`. |

## Files that do not exist yet, and the nearest pattern

| Target path | Nearest existing pattern |
|---|---|
| `go.mod`, `go.sum` | None in the repository. The scratch module of 2026-11-26 below. |
| `src/contracts/proto/lgtm/v1/common.proto` | `src/contracts/proto/lgtm/v1/session.proto` for the header and comment style. |
| `src/contracts/proto/lgtm/link/v1/link.proto`, `commands.proto`, `events.proto` | `src/contracts/proto/lgtm/v1/clients.proto` for a service, enums with a prefix and a `oneof`. |
| `src/contracts/gen/go/` | `src/contracts/gen/lgtm/v1/`, the generated TypeScript. |
| `src/contracts/*_test.go`, `src/contracts/testdata/` | None in Go. `scripts/check-private.mjs` is the nearest check that reads files. |
| `scripts/check-go.sh` | `scripts/check-credits.sh`, a shell check that the root `check` script calls. |

## What was run on 2026-11-26

All runs used the installed Go 1.23.3, which switched itself to the cached Go 1.26.0, with `GOPROXY=off`.

1. `go version` printed `go1.23.3`. `go env GOTOOLCHAIN` printed `auto`. The module cache holds the toolchains go1.26.0, go1.26.9, go1.26.10 and go1.26.11, and the modules `connectrpc.com/connect@v1.21.1` and `google.golang.org/protobuf@v1.36.12`.
2. A scratch module with the line `go 1.26.0` and `go get -tool` of both generators wrote two `tool` lines and two `require` lines. `go version` inside it printed `go1.26.0`.
3. `go tool protoc-gen-go --version` printed `protoc-gen-go v1.36.12`. `go tool protoc-gen-connect-go --version` printed `1.21.1`. `go mod verify` printed `all modules verified`.
4. `buf generate` with `clean: true` and the outputs `gen` and `gen/go` wrote `gen/lgtm/v1/*_pb.{js,d.ts}` and `gen/go/lgtm/v1/*.pb.go` and `gen/go/lgtm/v1/lgtmv1connect/*.connect.go`. The TypeScript files stayed.
5. With managed mode on and only a `go_package_prefix`, every TypeScript file changed: the embedded descriptor gained the options `java_package`, `java_multiple_files`, `java_outer_classname`, `objc_class_prefix`, `csharp_namespace`, `php_namespace`, `php_metadata_namespace` and `ruby_package`. With an explicit `option go_package` in each file instead, the descriptor gains only that option.
6. Draft files of `common.proto` and the three link files, as this plan gives them, passed `buf lint`, generated in both languages, and passed `go build ./...`, `go vet ./...` and `go mod tidy -diff`.
7. A draft of `src/contracts/watchshape_test.go` passed. With the line `FeedPosition position = 4;` removed from `clients.proto` and the code generated again, it failed with `lgtm.v1.WatchClientsResponse field 4: want position of kind message`.
8. A folder that holds only test files did not stop `go build ./...` or `go vet ./...`, and `go test .` inside it ran.
9. A throwaway program computed the proof examples and the join token example from fixed seeds. The values are in the plan, item 5 and item 6. Ed25519 signatures are deterministic, so any correct program computes the same bytes.
10. A throwaway program printed the description format of item 3 for each draft file. Its output is the content of the description files in the plan.
11. `find . -name '*.go'` outside `node_modules/.pnpm` found no Go file in the repository, so `go build ./...` at the root reads only the new code.
12. In the generated TypeScript, the new field reads `position?: FeedPosition | undefined;`, so it is optional.
13. A comment added above `syntax` in `session.proto` changed `session_pb.js`, `session_pb.d.ts`, `session.pb.go` and `session.connect.go` after generation, so the first probe of item 2 makes the check fail.
14. The sample server, started with `LGTM_MOCK_PORT=4199` from `src/mock-server`, answered `GetServer` with `{"product":"lgtm-studio","version":"0.9.0-sample","contract":1,"sample":true}` and `SignIn` with the password `sample` with `{"result":"SIGN_IN_RESULT_SIGNED_IN"}`. The plan's check uses the default port 4180.
15. `src/frontend/README.md`, lines 45 to 52: the website runs at `https://127.0.0.1:4179/`, the address `/` passes calls on to `http://127.0.0.1:4180`, and the sample server accepts any password except `wrong`.
16. In a module with the two `tool` lines and no package, `go build ./...` exited 0 with a warning, `go vet ./...` and `go test ./...` exited 1 with "no packages to vet" and "no packages to test", and `go mod tidy -diff` exited 1 because `go.sum` lacked the hashes of `github.com/google/go-cmp v0.7.0`.
