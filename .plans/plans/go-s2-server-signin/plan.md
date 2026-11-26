---
topic: go-s2-server-signin
status: discover
tier: M
created: 2026-11-25
updated: 2026-11-25
epic: go-rewrite
---

# The server program and sign-in

This file is a brief.
The orchestrator wrote it on 2026-11-25, in phase 2 of the build loop, which is named "Design and teams".
The planner of the Server team turns it into the full plan in phase 3.
The full plan follows the template of the planning skill, with its gates.

- **Sprint:** 2. The owner signs in
- **Team:** Server
- **Design:** [docs/specs/2026-11-25-go-rewrite-foundation-design.md](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md)
- **Epic:** [go-rewrite](../../epics/go-rewrite/epic.md)

## What this plan delivers

- The server program starts, brings the database up to date, and opens its port.
- The website's calls are answered over the Connect protocol.
- The owner signs in with the password from `LGTM_WEB_PASSWORD`, and the session survives a restart.
- The server keeps its own secrets in OpenBao, starting with its signing key.

## What a person can observe when it is done

- The server refuses to start without `LGTM_WEB_PASSWORD`.
- The website's sign-in page works against the Go server with no change to a screen.
- A wrong password is refused, and a call without a session gets the code `unauthenticated`.
- Changing the password ends every session.

## Work items

Each work item is built by the loop's steps: tests first, then code, then a check on that exact commit.
A size is low, medium or high.
The column "Needs" names the items that must be merged first.

| # | Work item | Files | Size | Needs |
|---|---|---|---|---|
| 1 | Settings and start-up: the server reads its settings from the environment, applies the database changes, and then opens its port. | `src/server/internal/config/config.go`, `src/server/cmd/lgtm-server/main.go` | medium | sprint 1 |
| 2 | Request handling: every service is mounted on one port. A service that is not built answers `unimplemented`. An error is a Connect code with one sentence. | `src/server/internal/api/server.go` | medium | 1 |
| 3 | The session check: every call needs a session, except `GetServer` and `SignIn`. | `src/server/internal/api/session_check.go` | high | 2, 5 |
| 4 | The password check, in constant time. | `src/server/internal/auth/password.go` | medium | nothing |
| 5 | Web sessions as rows: a verifier only, eight at most, 30 days each, bound to the password. A changed password ends them before the port opens. | `src/server/internal/auth/sessions.go`, `src/server/internal/auth/migrations/` | high | sprint 1 |
| 6 | The cookie and the origin rules: strict on the same site, not strict for a listed website on another site, and a refusal of a page that is neither HTTPS nor loopback. | `src/server/internal/auth/cookie.go`, `src/server/internal/auth/origin.go` | high | 5 |
| 7 | The session calls: `GetServer`, `SignIn` and `SignOut`. | `src/server/internal/api/session.go` | medium | 3, 4, 5, 6 |
| 8 | The vault client: the only code that speaks to OpenBao. The server makes its signing key on the first start and keeps it there. | `src/server/internal/vault/vault.go` | high | 1 |

A file named here that the blueprint in `floorplan/` does not name is a suggestion.
The planner may cut the files another way.

## What this team may edit and read

- **Edits:** `src/server/`
- **Reads only:** `src/contracts/`, `deploy/`, `packages/engine/` as a reference for sign-in, the design

## What it must agree on with other teams

- The credential file of the vault, with the Stack team.
- The environment variables of the server, with the Stack team, which writes them into the compose file.

## Checks of the design that it makes pass

C2, C3 and C4. The second half of C13.
The checks are the numbered lines C1 to C13 in the design's section "How wave 1 is proven done".

## Points the planner must settle

1. How the retry key of a call is checked beside the session.
2. How the old engine's sign-in logic is carried over, and whether any file of it needs a credit.
3. What the server does when the vault is not reachable at start-up.
4. The blueprint names three files in `auth` and two in `api`. This brief adds `cookie.go` and `session_check.go`.

## How this plan is reviewed

The plan is reviewed by the rules of the build loop, not by a fixed number of rounds: rounds 1 to 3 are full reviews, round 4 runs only for blockers and majors, and the owner approves each round after the fourth.
A finding is tied to a statement of the design: a row of its decisions table, one of its rules, or one of its checks.
