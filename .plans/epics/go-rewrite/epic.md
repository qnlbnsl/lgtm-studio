---
topic: go-rewrite
kind: epic
status: active
parked: false
ship_branch: master
updated: 2026-11-26
---

# LGTM Studio runs on a Go server and a Go client

## Outcome

The website in `src/frontend/` is served by a server written in Go.
The server keeps its state in PostgreSQL.
A client written in Go runs on each machine and runs the agents there.
The old TypeScript engine in `packages/` is removed.

The design is [docs/specs/2026-11-25-go-rewrite-foundation-design.md](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md).
The owner approved it on 2026-11-25.

## Done when

The epic is shipped when every screen of the website reads from the Go server, and `packages/` is gone.

The four sprints planned here reach a first stop on the way.
At that stop the 13 checks in the design's section "How wave 1 is proven done" pass on a clean machine.
In short:

- One command starts the stack, and the website opens at the loopback address.
- The owner signs in with the password.
- A machine that ran `lgtm-client join` appears on the Machines page, and it shows as online after approval.
- The stack, the server and the client each come back by themselves after a restart.

## How the build loop is applied

The owner decided on 2026-11-25 that this build follows the build loop in [docs/specs/2026-11-05-build-loop-design.md](../../../docs/specs/2026-11-05-build-loop-design.md).
LGTM Studio cannot run the loop yet, so the orchestrator runs it by hand.
The orchestrator is the planning session that wrote this file.

| Phase of the loop | What it is for this build | State |
|---|---|---|
| 1 Define | The design document is the end-result page. The map in `floorplan/` is its mockup. | Done. The owner approved on 2026-11-25. |
| 2 Design and teams | This file, and one brief for each plan. | Done. The owner confirmed the sprints and teams on 2026-11-26. |
| 3 Plan | The planner of each team turns its brief into a full plan. Reviewers attack it in rounds. The orchestrator then compares the plans. | Started on 2026-11-26. |
| 4 Build | For each work item, one agent writes the tests first and another writes the code. | Not started. |
| 5 Verify | The whole sprint is installed and every test runs on the merged work. | Not started. |
| 6 Review | Reviewers from another vendor read the merged code in slices. | Not started. |
| 7 Hand over | The sprint reaches `master` through a pull request. The owner tries the product. | Not started. |
| 8 Keep running | A full run each night. | Not set up for this build. |

Phases 2 to 7 repeat for each sprint.

### What phase 2 found at the starting commit

The loop starts a sprint only from a commit that passed a full run.
The starting commit is `e183b61f`.
These checks passed on it on 2026-11-25:

- `pnpm --filter ./src/contracts check`
- `pnpm --filter ./src/frontend typecheck`
- `pnpm --filter ./src/frontend build`
- `pnpm --filter ./src/mock-server typecheck`
- `node scripts/check-private.mjs`

The test suite of the old engine was not run, because the rewrite does not edit `packages/`.
No Go code exists yet, so no Go test exists.

### The rules of the loop that this build keeps

- **Tests first.** A test writer writes the tests of a work item from the design, before any code. The tests must fail on their assertion. They are then frozen, and the builder may not change them.
- **Another vendor writes the tests.** The test writer never runs on the model that builds.
- **Risky tests are proven.** For a work item that touches sign-in, keys or ordering, a reviewer breaks the code on a temporary branch. The tests must then fail.
- **One writer for each set of files.** Two builders never edit the same file at the same time.
- **A merge needs a passing report.** A lead merges a work item only when its checks passed on that exact commit.
- **A check that needs two teams' work is a gate of the sprint.** A work item is gated on its own tests. A check of the design that needs the server's program and the stack running together is run by the orchestrator once both teams' items are in the branch, and the sprint is not done before it passes.
- **Reviewers return findings, never edits.** No vendor reviews its own code.
- **Findings are tied to statements.** A statement is a row of the design's decisions table (D1 to D23), one of its rules (R1 to R11), or one of its checks (C1 to C13), counted in the order of commit `e183b61f`. A minor finding is 1 point, a major one is 3, and a blocker is 9.
- **A stuck statement goes to the owner.** A statement is stuck when it has at least 12 points and 30% of the points of one round, or when it drew a major or a blocker in three rounds.
- **Review rounds are limited.** Rounds 1 to 3 are full reviews. Round 4 runs only for blockers and majors. The owner approves each round after the fourth.
- **An evaluation round follows each phase step.** It grades the work and records which model did it. It never holds up the next step.

### Who does what

The owner decided on 2026-11-25 that builds and reviews use a mix of Codex and Claude, and that Cursor with Grok takes part as well.

| Seat | Who takes it |
|---|---|
| Orchestrator | The planning session, on Claude. |
| Planner of a plan | Claude and Codex in turn, plan by plan. |
| Reviewers of a plan | The vendor that did not write it, on the four lenses. One more seat on the logic lens goes to the vendor that wrote it. Grok joins the logic lens. |
| Test writer | The vendor that does not build the work item. |
| Builder | Claude and Codex in turn, work item by work item. |
| Reviewers of the code | The vendor that did not build it. Grok reads beside it. |

Whether Grok also builds is the orchestrator's reading, not a decision.
The reading is that Grok builds only work items of low size that have frozen tests.
The model sheet of the owner's toolkit picks the model and the effort of each seat at launch.

## The sprints

The design names four build waves.
Waves 0 and 1 are cut here into four sprints, so that each sprint ends with something a person can observe.
Cutting wave 1 into three sprints was the orchestrator's choice. The owner confirmed it on 2026-11-26.

| Sprint | What a person can observe at its end | Teams that work in it |
|---|---|---|
| 0. The contract | Go code is generated from the contract, and `go build ./...` passes. The link between a server and a machine is written down as a contract. | Contract |
| 1. Each part stands alone | The server's database layer passes its tests against a real PostgreSQL. The client makes its key and reads its machine. The stack starts PostgreSQL and the vault, and the vault opens again by itself after a restart. | Server, Client, Stack |
| 2. The owner signs in | One command starts the stack. The website opens, and the owner signs in against the Go server. | Server, Stack |
| 3. A machine joins | A machine joins with a token, the owner approves it, and it stays connected through restarts. All 13 checks pass. | Server, Client, Stack |

Waves 2 and 3 of the design get their sprints when sprint 3 is in review.

### The teams, and what each may edit

The server and the client are separate teams.
Neither edits the other's folder.
They meet only in the contract, which sprint 0 writes.

| Team | May edit | Reads only |
|---|---|---|
| Contract | `src/contracts/`, `go.mod`, `go.sum`, `scripts/` | Everything else |
| Server | `src/server/` | `src/contracts/`, the design |
| Client | `src/client/` | `src/contracts/`, the design |
| Stack | `deploy/` | `src/server/cmd/`, `src/client/cmd/`, `src/frontend/`, the design |

The link between a server and a machine is frozen when the plan `go-s3-client-link` has cleared its review and the findings of the review of the built contract (issue 1643) and the requests of the other plans are settled, which is before either team builds against it.
The plans `go-s3-server-stream` and `go-s3-server-commands` are written against the frozen link. A change after the freeze needs a new revision of the contract.
The link was frozen at revision 1 on 2026-11-26, in commit fcfa96c3. Its wire did not change.
(Changed on 2026-11-26: the plan `go-s3-server-machines` was cut in three after it did not clear its fourth review round with fifteen work items, and the freeze no longer waits for the server end of the stream.)
The orchestrator decided this on 2026-11-26, because the reviews of the contract kept finding disagreements with plans that were not yet reviewed.
Until then, a change to the link files is an ordinary work item of the Contract team. After it, a change goes back to phase 3.

From sprint 1 on, a team changes `go.mod` and `go.sum` only to add a module that its plan lists, with the version the plan states.
The builder adds the module in the same change as the first import of it, because the Go check refuses a module that nothing imports.
The orchestrator compares the change of `go.mod` with the plan's list before it commits.

The hook `deploy/test/mutate-feed-lock.sh`, which check 12 calls, is a wrapper that the Stack team owns.
The Server team writes the mutation that the wrapper calls, inside `src/server/`.

## Plans

Each plan is small on purpose: three to nine work items, each one sized low, medium or high.
The work items are listed in the brief of each plan.

| Order | Plan slug | What it delivers | State | Commit |
|---|---|---|---|---|
| 1 | go-s0-contract | The Go module, Go code generated from the contract, the shared `Watch` shape, and the link contract. | linked | 667ede94, and fcfa96c3 for the freeze of the link |
| 2 | go-s1-server-store | The server's database layer: transactions, numbered database changes, the change feed, safe retries and `Watch` streams. | linked | 89d3f656 |
| 3 | go-s1-client-local | What the client does on its own machine: its key, its facts, the harness scan, and the two records it keeps on disk. | linked | dd0c8cba |
| 4 | go-s1-stack-vault | The compose stack with PostgreSQL and OpenBao, where OpenBao opens by itself after a restart. | linked | f072231a |
| 5 | go-s2-server-signin | The server program: settings, request handling, sign-in, web sessions, and the vault client. | linked | f3d461e4 |
| 6 | go-s2-stack-web | The images of the server and the website, and one command that starts everything. | linked | 9a4dacf4 |
| 7 | go-s3-server-machines | The machine registry, join tokens, and the Machines page's calls that need no stream: a machine can join, wait, be approved and be listed. | linked | aa633e16 |
| 8 | go-s3-client-link | The `join` and `run` commands, and the client end of the link. | linked | ff36e948 |
| 9 | go-s3-stack-client | The installer of the client, and the server's own command that makes a join token. | linked | |
| 10 | go-s3-server-stream | The server end of the link: admission of a stream, presence, and the events a machine sends. Written on 2026-11-26 against the built code of plans 5 and 7 and the frozen link. | linked | 2b615a11 |
| 11 | go-s3-server-commands | Stored commands and the calls of the Machines page that send one. Written on 2026-11-26 against plan 10 and the frozen link. | linked | |

`planned` — named slice, no `.plans/plans/<slug>/plan.md` yet.
`linked` — that plan exists and its `epic:` is this slug.
Commit — the commit that landed the plan's build, filled in when the build merges.

Phase 3 turns each brief into a full plan. A plan with `status: attack` is in review.

## Tests that are written before any plan

The loop writes the user-flow tests in phase 2, from the end-result page, so that a wrong plan cannot shape them.
They stay red until phase 5 of the sprint that makes them pass.

| Test | Covers | Goes green in |
|---|---|---|
| A script that drives the running stack through the 13 checks of the design | C1 to C13 | Sprint 1 for C12 and the first half of C13. Sprint 2 for C1 to C4 and the rest of C13. Sprint 3 for C5 to C11. |
| Go tests that run a real client against a real server in one process | The link: C6 to C11 | Sprint 3 |

The script is `deploy/test/checks.sh`. It was written on 2026-11-26 from the design alone, and all 13 checks fail today, as intended.
The Go tests of the link wait until sprint 0 has frozen the contract, because they cannot compile before the generated code exists.

## What the teams must agree on

The orchestrator compares the plans on these points at the end of phase 3.

1. **The link contract.** Sprint 0 writes it. The server and the client both build against it in sprint 3.
2. **The Go interface of the database layer.** Sprint 1 writes it. Every later part of the server calls it.
3. **How the server gets its credential for the vault.** The Stack team writes it to a file in sprint 1. The Server team reads it in sprint 2.
4. **The layout of the client's state folder.** `go-s1-client-local` makes it, and `go-s3-client-link` uses it.
5. **The entry points that tests use to start each program.** See issue 1634.

## Problems found while planning

They are in [docs/issues.md](../../../docs/issues.md).

- **Issue 1633.** The design does not say where encryption ends for the link inside the compose stack.
- **Issue 1634.** A Go test cannot import both programs, and the design asks for one that does.
- **Issue 1635.** The build loop sizes work in points and gives each lead a cap. No document defines either, so this epic sizes work items as low, medium or high.

Two facts about OpenBao were read in its documentation and never run.
`go-s1-stack-vault` runs them first, before anything else builds on them.

## A first count of agent launches

This is an estimate from the 55 work items of the nine briefs.
It assumes two review rounds for each plan.

| What | Launches |
|---|---|
| Planners | 9 |
| Reviewers of plans | about 95 |
| Test writers and builders | about 110 |
| Reviewers of code | about 45 |

That is about 260 launches over four sprints: about 105 on Claude, about 115 on Codex and about 40 on Grok.
The orchestrator reads the usage meter of each vendor at the start of each sprint.

## Out of scope

- Running an agent. It is the first work of wave 2.
- Every service of the contract except `SessionService` and `ClientService`.
- Any import of the old engine's data.
- A client on Windows.

## Choices

- 2026-11-25: the owner approved the design of waves 0 and 1.
- 2026-11-25: the owner decided that builds and reviews use a mix of Codex and Claude, and that Cursor with Grok takes part as well.
- 2026-11-25: the owner asked for proper phases, separate plans for the server and the client, and smaller work items.
- 2026-11-25: the owner decided that this build follows the build loop.
- 2026-11-25: the orchestrator cut wave 1 into three sprints.
- 2026-11-26: the owner confirmed the four sprints and the teams, and told the orchestrator to start.
- Not decided: whether each builder works in a Git worktree of its own. On 2026-11-25 the owner told the planning session to start none. Phase 4 needs an answer, because builders that share one checkout collide.
