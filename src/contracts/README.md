# The contract between the website, the server and the client

This folder holds the contract of LGTM Studio, written in Protocol Buffers.
The owner decided on 2026-11-23 to split the product into a website, a server and a client.
The three parts meet only here, so that one part can be rewritten without the others.
The owner chose Protocol Buffers and gRPC on 2026-11-12.

## What is here

- `proto/lgtm/v1/` holds the messages and the calls. This is the source.
- `gen/` holds the TypeScript generated from it. It is committed, and nobody edits it.
- `buf.yaml` and `buf.gen.yaml` set up the tool `buf`.

| File | Service | What it covers |
|---|---|---|
| `session.proto` | `SessionService` | What answers at an address, signing in and signing out |
| `clients.proto` | `ClientService` | The clients of a server, and the tokens that let a client join |
| `map.proto` | `MapService` | The codebase map of a project, or of a repository's main checkout: its state, a request to build it, and a short-lived address of its page |
| `preview.proto` | `PreviewService` | The running product of a project, or of a repository's main checkout: its previews, a request to start or stop one, a short-lived address of its page, and a request to add, change or remove a preview |

A browser cannot speak gRPC itself. The website uses the Connect protocol over `fetch`.
A server that answers Connect can answer gRPC from the same files.

## Rules

- A change to what a part sends or reads starts in a `.proto` file. Run `pnpm --filter @lgtm/api generate` after it.
- `pnpm --filter @lgtm/api check` lints the files and fails when `gen/` is out of date.
- A field number is never used again, and a field's type never changes. Add a new field instead.
- A change that breaks an older website raises the `contract` number that `GetServer` returns. The website and the mock server each name the revision they speak.
- Every call needs a session, except `SessionService.GetServer` and `SessionService.SignIn`.
- The session travels in a cookie that the server sets. No message carries it.
- A list that changes is read with a `Watch` call. Its first message holds everything, and each later message holds one change.
- Every message, field and call has a comment that says what it is for.

## What a server must do beyond the messages

- Set the session cookie on `SignIn` with `HttpOnly` and `SameSite=Strict`, and clear it on `SignOut`.
- Answer a website on another origin only when that origin is allowed, and allow credentials for it. `src/mock-server/src/serve.ts` shows one rule.
- Answer a call without a session with the code `unauthenticated`.
- Serve the page of a codebase map as untrusted content. The page is an agent's output. The engine in `packages/` serves it from a separate hostname with a response sandbox, and `SECURITY.md` has the rules. The mock server serves it from its own address with a sandbox only, because its page is sample data.
- Serve a running product as untrusted content too. Its pages are code that agents wrote. A frame's address is its own key and carries no session. The website shows it in a frame with a sandbox and no origin, so a product that needs its own storage or its own sign-in does not work there yet. A real server gives each preview a hostname of its own before the frame may have an origin. It also passes on every path, every asset and every WebSocket of the product, which the mock server does not.

## Not here yet

The calls between the server and the client are not written.
Most of the website's records are not in the contract yet. Issue 1613 lists them.
