**SHIP.**

The frozen tests accept a machine watch that applies a queued frame after the browser has switched to another signed-in session.

The builders, frozen test writers, and reviewer are OpenAI workers. I did not build the code or write the frozen tests. The task identifier is `postbuild-go-s4-web-repositories-3`, and the nonce is `14b63d`. I reviewed items 3 and 5 and points 1 through 3 of issue 1689 on `sprint-0` at `8c63848b`, against `git diff cee27753~1..8c63848b -- src/frontend`.

### post-3 1 — MINOR — The machine watch lacks a queued-frame regression test

**Statement.** The plan requires both watches to check their session before committing a queued frame. The fixture can hold a decoded repository frame. For the machine stream, the frozen boundary tests observe cancellation without releasing a decoded frame into the next session. [Plan item 2](../plan.md#L87) · [Plan item 3](../plan.md#L96) · [Frame fixture](../../../../src/frontend/e2e/support/repository-server.ts#L70) · [Frozen boundary tests](../../../../src/frontend/e2e/repository-watch.spec.ts#L373)

**Evidence and failure scenario.** Confirmed live in scratch. I bypassed the frame guard only for the machine watch, leaving the repository and stream-error guards in place. The frozen browser selection still printed `126 passed`. I then held a decoded save from the first server, signed in to another server with both watches live, and released the frame. The mutated build put the old machine in the new workspace at both widths. The same probe passed against `8c63848b`. [Production frame guard](../../../../src/frontend/src/server/sync.ts#L180) · [Machine row update](../../../../src/frontend/src/server/sync.ts#L147)

**Fix.** Extend the frozen fixture to hold a decoded machine frame. Release it after another server has opened both watches and assert that the address, authentication, records, and open dialog belong to the new session.

### post-3 2 — MINOR — The folder tests accept a request dispatched after its intent expires

**Statement.** Item 5 binds dispatch and settlement to the current attempt. The frozen cases make the guard false only after the generated call has started. [Plan item 5](../plan.md#L178) · [Frozen settlement cases](../../../../src/frontend/e2e/folder-calls.spec.ts#L131)

**Evidence and failure scenario.** Confirmed live in scratch. Removing the first `isCurrent()` check while keeping both settlement checks left all 16 folder cases green. If a queued callback invokes Add after its dialog has been replaced while the session remains signed in, that implementation can send a save with the old path and key. It then discards the reply. The built guard returns before dispatch. [Dispatch and settlement checks](../../../../src/frontend/src/server/folders.ts#L21) · [Add wrapper](../../../../src/frontend/src/server/folders.ts#L42)

**Fix.** Start each wrapper with an already-obsolete guard and a live signal. Assert that it returns `undefined` and that the client received zero requests.

**Grade.** 9 out of 10 for the reviewed build.

## Post-build checks and scope

- **Unreviewed changes.** By reading, the plan's only unreviewed mechanism is the later dialog retry in item 6. The round-one findings register assigns the compatibility, cookie, and removal checks to the relevant workstreams. [Plan mark](../plan.md#L190) · [Findings register](round-1-findings.md#L41)
- **Repository watch and approval.** Confirmed live in Chromium. Both snapshot orders, including an empty snapshot, kept the deep link busy until both feeds initialized. A lost watch retained the rows, dropped live status, and retried from 500 milliseconds up to eight seconds. A replacement snapshot removed missed rows. Removal converged with the machine frame or repository frames first. The orphan disappeared and the other machine kept its folder. The issue 1689 cases showed that a current approval in either order expands the row while the watch supplies its facts. The inherited tests also kept a removed row absent and a replacement dialog open in both approval orders. [Watch cases](../../../../src/frontend/e2e/repository-watch.spec.ts#L188) · [Approval cases](../../../../src/frontend/e2e/repository-watch.spec.ts#L498) · [Removal and late approval](../../../../src/frontend/e2e/repository-session.spec.ts#L431)
- **Sessions and folder requests.** Confirmed live in Chromium. Expiry on either watch, sign-out, and server change aborted both watches and cleared the records and dialog. Same-host discovery waited for the held cookie writer and received the expected cookie. A different hostname progressed while the old writer was pending. In the independent probe, a queued save or authentication failure from either watch left the replacement session intact. A held inspection success and add authentication failure both resolved to `undefined` after the captured scope was aborted. A fresh sign-in at the same address also rejected an old repository frame and both folder replies. The frozen folder tests exercised success and expiry for each procedure, stable keys and bodies on retry, fresh attempt signals, the exact path, and both forms of the optional add target. [Cookie tests](../../../../src/frontend/e2e/repository-session.spec.ts#L291) · [Watch boundary tests](../../../../src/frontend/e2e/repository-watch.spec.ts#L373) · [Session invalidation](../../../../src/frontend/src/server/sync.ts#L86) · [Folder tests](../../../../src/frontend/e2e/folder-calls.spec.ts#L95)
- **Sample screens.** Confirmed live in Chromium. I compared normalized visible text from the pre-build commit `f193b64c` and the reviewed commit. Login, Home, Agent console, Machines, expanded machine detail, Command center, and Track repository matched in the in-page sample, standalone mock, and older-server fixture, at both widths. The frozen discovery tests also showed zero repository calls in those modes, and no delayed sample writes or simulation interval surviving the switch to the advertising fixture. [Compatibility tests](../../../../src/frontend/e2e/repository-discovery.spec.ts#L229) · [Pending sample action test](../../../../src/frontend/e2e/repository-discovery.spec.ts#L293)
- **Deployed checks.** The browser portions of sprint checks R12 and R13 were exercised with fixtures. The exact deployed R12 and R13 commands and the end-to-end R14 observation remain pending. The pinned server composition supplies sessions, clients, and the machine link. The project-service implementation available in the review was a test probe, and item 6 owns the dialog join used in R14. [Post-build rows](../plan.md#L281) · [Server composition](../../../../src/server/internal/app/start.go#L268) · [Test probe](../../../../src/server/internal/api/projects_test.go#L92)

## Commands and mutation results

I used `export PATH=~/.nvm/versions/node/v26.13.0/bin:$PATH`. Node.js printed `v26.13.0`, and pnpm printed `12.16.1`. Playwright ran desktop and 390 by 844 pixel projects in one scratch copy of the reviewed commit.

| Check | Last output and exit status |
|---|---|
| `pnpm exec playwright test --config src/frontend/playwright.config.ts --reporter=line --output browser-results repository-watch folder-calls repository-session repository-discovery` | `126 passed (1.2m)`, exit 0. This includes 24 repository-watch cases, 16 folder-call cases, and 86 discovery and session cases. |
| The independent boundary and removal probes. | `14 passed (10.2s)`, exit 0. |
| The independent before-and-after sample comparison. | `6 passed (11.2s)`, exit 0. |
| `pnpm --filter @lgtm/frontend typecheck` and `pnpm --filter @lgtm/frontend build` | `$ tsc --noEmit` and `✓ built in 962ms`, both exit 0. Vite printed its chunk-size warning. |
| `git diff --check cee27753~1..8c63848b -- src/frontend` and `sha256sum -c` for both frozen test manifests. | Exit 0 for both checks. Both frozen test files printed `OK`. |

Each named mutation ran independently. The watch suite rejected opening after one snapshot, merging a replacement, appending a saved record, applying a replaced repository frame, and declaring live status from one watch. Each printed `1 failed` and exited 1. The folder suite rejected a constant key and a trimmed path with `1 failed` each. It rejected returning obsolete success and propagating obsolete expiry with `2 failed` each. All four exited 1. The two surviving mutations are recorded above.

By reading, the assigned code uses the generated Protocol Buffers services and has no log writes or file operations. Current call errors reach their caller and stale errors are dropped. The repository watch builder records the position-only fixture used in place of a heartbeat, which has no message in the contract. [Folder request implementation](../../../../src/frontend/src/server/folders.ts#L21) · [Watch failure path](../../../../src/frontend/src/server/sync.ts#L198) · [Builder result](../../../../.foundry/go-rewrite/results/s4-web-3-build.md)

The skills checkout is one commit behind its remote. Four skill feedback rows are open.
