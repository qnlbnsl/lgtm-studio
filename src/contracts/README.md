# The contract between the website, the server and the client

This folder holds the contract of LGTM Studio, written in Protocol Buffers.
The owner decided on 2026-11-23 to split the product into a website, a server and a client.
The three parts meet only here, so that one part can be rewritten without the others.
The owner chose Protocol Buffers and gRPC on 2026-11-12.

## What is here

- `proto/lgtm/v1/` holds the website's messages and calls.
- `proto/lgtm/link/v1/` holds the messages and calls between a server and a client.
- `gen/lgtm/` holds the generated JavaScript and TypeScript declarations. `gen/go/` holds the generated Go messages and Connect services. Commit the generated files and make changes in the Protocol Buffers source.
- `testdata/` holds the contract descriptions and worked examples of a join token and the signed proofs.
- `buf.yaml` and `buf.gen.yaml` set up the tool `buf`.

### The website contract in `lgtm.v1`

| File | Service | What it covers |
|---|---|---|
| `session.proto` | `SessionService` | What answers at an address, signing in and signing out |
| `clients.proto` | `ClientService` | The clients of a server, and the tokens that let a client join |
| `common.proto` | | The shape of a `Watch` reply and its position in the change feed |
| `map.proto` | `MapService` | The codebase map of a project, or of a repository's main checkout: its state, a request to build it, and a short-lived address of its page |
| `preview.proto` | `PreviewService` | The running product of a project, or of a repository's main checkout: its previews, a request to start or stop one, a short-lived address of its page, and a request to add, change or remove a preview |

A browser cannot speak gRPC itself. The website uses the Connect protocol over `fetch`.
A server that answers Connect can answer gRPC from the same files.

### The server and client contract in `lgtm.link.v1`

| File | Service | What it covers |
|---|---|---|
| `link.proto` | `LinkService` | Joining and opening the stream, including the proof of both ends |
| `commands.proto` | | Commands the server sends, machine standing, and the results the client sends back |
| `events.proto` | | Ordered reports from the client, their acknowledgements, and heartbeats |

## Rules

- A change to what a part sends or reads starts in a `.proto` file. Run `pnpm --filter @lgtm/api generate` after it. Generation needs Go 1.26 or newer.
- `pnpm --filter @lgtm/api check` lints the files, regenerates the contract, and runs the Go contract tests. It fails on changed or untracked files in `gen/`.
- A field number is never used again, and a field's type never changes. Add a new field instead.
- A change that breaks an older website raises the `contract` number that `GetServer` returns. The website and the mock server each name the revision they speak.
- Calls in `lgtm.v1` need a website session, except `SessionService.GetServer` and `SessionService.SignIn`. The session travels in a cookie that the server sets. No message carries it.
- The shared `Watch` shape in `common.proto` starts with `all`, then sends a `saved` record or a `removed_id` for each change. The first `position` is the highest position the records include, or zero when no change is visible. Each later `position` is higher. `WatchMap` and `WatchPreviews` will adopt this shape when their services are built.
- `LinkService.Join` needs a join token, and `LinkService.Open` needs the machine's key. Both operate outside the website session.
- The link `lgtm.link.v1` is frozen at revision 1. A change to `link.proto`, `commands.proto` or `events.proto` needs the next revision and a new entry in `testdata/link-frozen.json`.
- The server and client speak link revision 1. A later call beside the stream presents the current stream's connection token in the `Lgtm-Link-Token` header.
- Every message, field and call has a comment that says what it is for.

## What a server must do beyond the messages

- Set the session cookie on `SignIn` with `HttpOnly` and `SameSite=Strict`, and clear it on `SignOut`.
- Answer a website on another origin only when that origin is allowed, and allow credentials for it. `src/mock-server/src/serve.ts` shows one rule.
- For a call in `lgtm.v1` that requires a session, return `unauthenticated` when the session is absent.
- Serve the page of a codebase map as untrusted content. The page is an agent's output. The engine in `packages/` serves it from a separate hostname with a response sandbox. The mock server serves it from its own address with a sandbox only, because its page is sample data.
- Serve a running product as untrusted content too. Its pages are code that agents wrote. A frame's address is its own key and carries no session. The website shows it in a frame with a sandbox and no origin, so a product that needs its own storage or its own sign-in does not work there yet. A real server gives each preview a hostname of its own before the frame may have an origin. It also passes on every path, every asset and every WebSocket of the product, which the mock server does not.

## Not here yet

Most of the website's records are not in the contract yet. Issue 1613 lists them.
