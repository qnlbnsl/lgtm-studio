# The mock server

This folder holds a server with sample data. It answers the contract in `src/contracts/`.
The website is built against it, so the real server can be written later, in Go, against the same contract.
Nothing here is meant to be copied into the real server.

## How to run it

Run `pnpm --filter @lgtm/mock-server dev`. It answers at `http://127.0.0.1:4180` and starts again when a file changes.
Its data lives in memory, so a restart brings the sample back.

| Variable | Meaning | When not set |
|---|---|---|
| `LGTM_MOCK_PORT` | The port | 4180 |
| `LGTM_MOCK_HOST` | The address to listen on | `127.0.0.1` |
| `LGTM_WEB_PASSWORD` | The password to sign in with | Any password except `wrong` |
| `LGTM_WEBSITE_ORIGIN` | More origins a website may call from, separated by commas | None |
| `LGTM_MOCK_MAP` | A `floorplan` folder whose map the Architecture page shows | A placeholder page |

## What is here

- `src/engine.ts` holds the data and answers the calls. It uses no file, no network and no Node API.
- `src/sample.ts` holds the sample records.
- `src/serve.ts` puts the engine behind a network port.

The website can also run the engine inside its own page. The address `sample` does that.

## What the sample makes up

- A token made on the Machines page is used about three seconds later by a machine named `mini.lan`.
- An approved machine is online about three seconds after that.
- A machine that is asked to reconnect is online after a moment.
- The project `console` has a codebase map from the start. Another project gets one about five seconds after "Draw the architecture".
- Every project has two previews: "Website", which runs, and "Phone app", which is stopped. A stopped preview runs about three seconds after "Run". No command runs: every preview shows one small sample page. A preview that is added or changed is kept until the server stops.

## The sample map

The Architecture page of the Design view shows a page that [Floorplan](https://github.com/haydenschmitty/floorplan) built.
No such page is in this repository, and no Floorplan code is copied here.
Name a `floorplan` folder of any checkout with `LGTM_MOCK_MAP`, and the mock server reads its `index.html` and `graph.json`.
With no folder, the Architecture page shows a small placeholder page.
