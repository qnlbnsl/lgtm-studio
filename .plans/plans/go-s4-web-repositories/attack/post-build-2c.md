# Post-build review 2c of item 2, server and session lifetime

Task identifier `postbuild-go-s4-web-repositories-2c`, nonce `e1fa84`. The reviewer is Claude Sonnet 5.5 at high effort. The reviewer did not build this code and did not write its tests.

The cross-vendor rule is met. Codex workers wrote the build and the tests, and Codex's content filter stopped its own reviewer, so this review comes from a Claude model.

## Verdict

**SHIP.**

The top finding is that the frozen tests accept an approval that writes the machine row while its own pairing dialog is still current, which is the mutation the plan names (post-2c 1).

The session boundary of item 2 holds. The four findings of the second review of item 1 are closed. Nothing a sample screen shows changed in any test I ran. The four findings below are minor and none of them blocks the merge of item 2.

## What I read and ran

I read the plan item 2, the builder's result file, `docs/issues.md` issue 1687, and the whole production diff of `git diff 2a827b68~1..a1607022 -- src/frontend`. I read the tests of `repository-session.spec.ts` and `repository-session-review.spec.ts`. I ran everything in one scratch copy made with `mktemp -d`, on the branch `sprint-0`.

The plan has no `UNREVIEWED FIX` mark for item 2. The only `UNREVIEWED` mark in the plan is on item 6. The attack folder has no register with a section "Not reviewed before the build" for item 2. The places to check are therefore the plan's post-build rows that fall on item 2, and the plan's named mutations, and I ran both.

## Findings

### post-2c 1 — MINOR — The tests accept an approval that writes the machine row while its dialog is current

**Statement.** The plan names the risky mutation "let approval write a machine row" and requires a red assertion. The only approval test releases the held approval after it has replaced the pairing dialog, so the approval is already stale when it completes. No test completes an approval whose own dialog is still current and then asks who owns the row. This breaks the plan's own mutation rule (`plan`, item 2, last bullet).

**Evidence and failure scenario.** Confirmed live in the scratch copy. I made the approval reply handler push a machine row and remove the joining row, placed after the `current()` guard. All 22 tests of `repository-session` passed on desktop. When I placed the same write before the guard, the test `the watch owns the row and approval cannot close a replacement dialog` failed in both orderings, so the builder's mutation was probably placed there. A wrong implementation that writes the row only for a current approval passes every test. Its effect is a machine row that appears before the watch confirms it, which violates "In real mode `WatchClients` owns both machine collections." [Approval handler](../../../../src/frontend/src/screens/service.tsx#L412) · [Only approval test](../../../../src/frontend/e2e/repository-session.spec.ts#L431)

**Fix.** Add a case where the approval is released while its own dialog is current, in both orders with the watch frame. Assert that `machines` and `joining` are exactly what the watch last sent until the frame arrives.

### post-2c 2 — MINOR — Whether a newly approved machine is expanded depends on which reply arrives first

**Statement.** After a current approval, the machine row is expanded only when the watch has already delivered the row. When the unary reply wins, the dialog closes with the toast "is paired" and the machine stays collapsed. The same user action gives two different screens.

**Evidence and failure scenario.** Confirmed live in the scratch copy with a probe that held the approval and sent the `saved` frame before or after it. With the watch first, the record `expanded['machine:waiting-worker']` was `true`. With the approval first it was `false`. The toast was "Newly approved is paired" in both runs. The cause is `const watched = machines.some(...)` read at reply time, with no later chance to expand. The frozen test asserts `expanded` only for a removed row, so neither order is checked for a current approval. [Expansion rule](../../../../src/frontend/src/screens/service.tsx#L417) · [Assertion on a removed row](../../../../src/frontend/e2e/repository-session.spec.ts#L463)

**Fix.** Remember the approved identifier and expand the row when the watch delivers it, or decide that approval never expands in repository mode. Add a test for a current approval in both orders.

### post-2c 3 — MINOR — A protected map or preview call that is refused as unauthenticated never ends the session

**Statement.** Only `machineCall` and the machine watch call `expireSession`. The map watch, the preview watch and the unscoped calls `buildMap`, `startPreview`, `stopPreview`, `savePreview` and `removePreview` report the refusal as an error text and retry. The plan limits the expiry rule of item 2 to a protected machine call or the machine watch, so this is not a plan violation. It is the place where items 3 and 6 must extend the same path, and nothing yet forces them to.

**Evidence and failure scenario.** By reading. On a repository server, suppose the cookie expires while the Map screen is open and the machine stream is still open. The map watch reconnects, receives `Code.Unauthenticated`, calls `onError`, waits, and tries again, as long as the screen is open. The session stays `in` and the user sees an error line instead of the Login screen. The unscoped calls use the global `api()`, which also throws "The website has no server yet." while a server change is connecting, because `forgetSelection` clears the selection first. [Map watch catch](../../../../src/frontend/src/server/sync.ts#L331) · [Preview watch catch](../../../../src/frontend/src/server/sync.ts#L376) · [Unscoped calls](../../../../src/frontend/src/server/sync.ts#L338)

**Fix.** Route the map and preview watches and calls through the captured scope and call `expireSession` on `Unauthenticated`, as `machineCall` does. Add the case to the item 3 and item 6 tests.

### post-2c 4 — MINOR — A failed sign-out is swallowed, so the page can show a signed-out state while the cookie is still valid

**Statement.** When the sign-out request fails or reaches its six-second deadline, the error is dropped, the page clears its records, and the user goes to Login with no notice. The server may still hold the session. The design says the page forgets the session either way, and the owner has decided that security issues do not block the build, so I rate this MINOR.

**Evidence and failure scenario.** By reading, and the test `a held signOut has a six-second network deadline` asserts the forgetting but nothing about a notice. The user presses Sign out on a bad network, sees Login, and closes the laptop. Discovery on the next load can report that the browser is still signed in, and the session resumes without a password. [Swallowed error](../../../../src/frontend/src/server/sync.ts#L265) · [Caller](../../../../src/frontend/src/actions.ts#L435)

**Fix.** Keep the immediate cleanup, and when the request fails show one toast "Sign-out did not reach the server." Add an assertion to the deadline test.

## The findings of the second review of item 1

All four are closed, and each has a mutation that turns a test red.

| Finding | Test | Mutation I tried | Result |
| --- | --- | --- | --- |
| post-1b 1, late discovery redirects the next sign-in | `post-1b 1` | Remove the generation check after `openServer` | 2 tests fail, including `post-1b 1` |
| post-1b 2, held sample workflow clone writes after a switch | `post-1b 2` | Remove the re-check after the fetch in `workflows.tsx` | `post-1b 2` fails |
| post-1b 3, late sample sign-in | `post-1b 3` | Apply the sign-in without the intent check | `post-1b 3` fails |
| post-1b 4, wrong harness version | `post-1b 4` | Overwrite the version on every later machine | `post-1b 4` fails |

## The plan's post-build scenarios

| Scenario or mutation | Result |
| --- | --- |
| Remove a generation guard, three places (after the cookie-writer wait, after `openServer`, in the unreachable catch) | One distinct test fails for each place. |
| Defer cleanup until after sign-out | Three tests fail, including `stalled sign-out drops session records before an obsolete action completes`. |
| Let approval write a machine row | Red when written before the guard, green when written after it (post-2c 1). |
| Do not serialize the cookie writers | `a queued sign-in follows sign-out and opens an authenticated watch` fails. |
| Removal done-callback ignores the scope | Four tests fail. |
| Removal applies sample edits in repository mode | `a current removal leaves sample edits alone and waits for the watch` fails. |
| Remove the abort check at the start of `machineCall` | All tests pass. I rate this as an equivalent mutation, because every call passes the scope's abort signal. |
| Remove the scope check before revoking a late pairing token | All tests pass. The revoke goes through the old clients to the old server, so the test that counts revokes at the next server cannot see it. The harm is small, and I did not rate it a finding. |

## Design ties

D1 and D11, rules 7 and 8, and checks C3 to C7 apply to item 2 according to the plan. Finding post-2c 3 belongs to the later items 3 and 6 and to `plan`. The other three findings tie to `plan`, item 2.

Known late writers left alone by the builder (the Map and Browser frame release, generic confirm and rename dialogs, the sign-in in `live/signin.tsx`, the Design canvas and the terminal clipboard) are recorded as issue 1687 and are not findings here. Issue 1682 is also known.

## Verification output

All commands ran in the scratch copy unless noted. Node comes from `~/.nvm/versions/node/v26.13.0/bin`.

| Command | Result |
| --- | --- |
| `playwright test repository-session` (desktop and phone) | 44 passed |
| `tsc --noEmit` in `src/frontend` | exit 0 |
| `vite build` in `src/frontend` | built, no errors |
| Full frontend suite in the scratch copy | 88 passed, 2 failed |
| The two failing tests, run again in the repository checkout | 2 passed |

The two failures in the scratch copy are `standalone mock keeps sign-in, sample work and simulation` on desktop and phone. The fixture starts the mock server with `pnpm --filter`, which pnpm refuses in a scratch copy. The same test passes in the real checkout, so the full suite is 90 passed. The builder's result file says 86 passed, which is a different count at an earlier moment and not a defect.

I received the range end `1c9e3f0e` from an earlier version of the task, and it does not exist in the repository. The current task names `a1607022`, and I used `2a827b68~1..a1607022`.

I changed no file of the repository. Playwright wrote only to the ignored `src/frontend/test-results` folder.

## Grade

**8 out of 10.** The session boundary is built carefully, the mutations I tried are caught by named tests, and the four findings of the second review of item 1 are closed with real assertions. Points are lost for the approval path, which has one test that cannot tell a current approval from a stale one, and for two places that item 3 and item 6 must still cover.
