# The website of LGTM Studio

This folder holds the website, in Preact 11, Vite and TypeScript (TSX).
It is one of three parts. The other two are a server and a client, and the owner will write them in Go.
The three parts meet only through the contract in `src/contracts/`.
The owner decided this on 2026-11-23.

The folder began as `design/mock-vite/`, a port of `design/mock/` at commit `885021d7`.
Design work happens here. A screen's rules are in `design/DESIGN.md`.

## How it reaches a server

The website talks to a server at an address.
A build fixes the address with the environment variable `VITE_LGTM_SERVER`.
When that is not set, the first screen asks for the address and keeps the answer in the browser.

An address is a name or an Internet Protocol (IP) address, with a port when it has one.
`localhost:4180` is reached over `http`. Port 80 is `http`. Every other address is `https`.
`:4180` means that port on the page's own host. A full address such as `http://192.0.2.5:4180` is used as written.
`sample` means the sample server inside the page, with no network at all.
`/` means the page's own address. The host that serves the website then passes the calls on to a server.
The development server does that. It passes them to `http://127.0.0.1:4180`, or to the address in `LGTM_SERVER_BEHIND`.

The website is not rendered on a server. Every call starts in the browser.
A password therefore travels from the browser to the website's host, whichever address the server has.

Only `src/server/` knows how a call reaches the server.
`src/server/api.ts` makes the typed clients, and `src/server/sync.ts` keeps the store in step with the server.
A screen never builds a request itself.

## What has moved to the contract

The session, the machines, a project's codebase map and a project's running product are read from the server and changed through it.
Every other record is still sample data inside the page. Issue 1613 lists them and the order they move in.
A record moves in four steps.

1. Its messages and calls are written in `src/contracts/proto/`, and the code is generated.
2. The mock server holds its sample data and answers its calls.
3. `src/server/sync.ts` reads it into the store, and the screens call the server for each change.
4. Its sample data and its `live.on` branches leave this folder.

## How to run it

Run `pnpm install` from the repository root once.
Run `pnpm --filter @lgtm/mock-server dev` for the mock server on `http://127.0.0.1:4180`.
Run `pnpm --filter @lgtm/frontend dev`, and open `https://127.0.0.1:4179/`.
Type `/` on the first screen, or choose "Use sample data".

The development server speaks HTTPS with a certificate it signs itself, so the browser warns once.
It needs `openssl`, and it keeps the certificate in `.cert/`, which git ignores.
From another computer, open `https://<this computer's address>:4179/` and type `/`.
The mock server accepts any password except `wrong`.
The routes after the `#` are such as `#/home` or `#/p/console/board`.
The worst-case sample loads with `?data=worst` before the `#`.

Other scripts:

- `pnpm --filter @lgtm/frontend build` writes the finished page to `dist/`.
- `pnpm --filter @lgtm/frontend typecheck` checks the types with the repository's pinned compiler.

## What it reads from `design/mock/`

The website has its own stylesheets, in `src/css/`.
It still reads four things from `design/mock/`: the fonts, the CodeMirror bundle, the generated `workflow-contracts.js`, and the default workflow in `design/mock/workflows/`.
It serves the sprint plans from `design/mock/plans/`.
The Vite set-up in `vite.config.ts` allows those folders and names `design/mock/` as `@mock`.
These files must move here before the older mock can be removed.

## Rules for the code

- A screen's look follows `design/DESIGN.md`. A change to a screen is checked at 1440 and at 1000 pixels, in dark and in light.
- Write `class`, not `className`. Use `onInput` for typing and `onChange` for the committed value.
- Import from `preact` and `preact/hooks` only. Do not use `preact/compat`, Signals or `defaultProps`.
- Write the unit in every style value, such as `'12px'`, because Preact 11 no longer adds `px`.
- The data model and the store's state are typed in `src/types.ts`. Component props are typed.
- The modules in `src/live/` hold the older live website's behaviour, for the engine in `packages/`. They stay switched off, and each one leaves when its records move to the contract.
- The website names no person. The foot of the sidebar shows the server's address. A sample that needs a person's name, such as an account, uses Maya Okafor.
- Preact 11 writes a data attribute whose value is `false` as `"false"`. The mock's older Preact left it out, and the styles depend on that, so `src/lib.tsx` keeps the old rule.

## Moves between the screens

`design/flows.md` lists every move between the screens and where a Back button belongs.
A new link between two screens is added to that document in the same change.
