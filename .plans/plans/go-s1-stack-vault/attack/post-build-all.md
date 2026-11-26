# Post-build review of go-s1-stack-vault, all six work items

Task identifier: postbuild-go-s1-stack-vault-all. Nonce: 5f0f63.
Reviewer of built code, model claude-sonnet-5-5, effort high.
Code read: `git diff 0c332aac..6822f986 -- deploy` on branch `sprint-0`.

## Verdict

REVISE. The change is small, and nothing in it is a blocker.

## Top finding

The test of work item 4 accepts a seal step that fails open: if the database probe fails and the seal key is missing, a mutated `seal` creates a fresh key, and all 94 assertions of `deploy/test/seal.sh` still pass (post-all 1).

## What I ran

All runs used the checkout as it stands and scratch copies of `deploy/` made under `mktemp -d`.
Projects that I started by hand were named `lgtm-s1v-rv-*`, and their state folders came from `mktemp -d`.
Other teams ran their own `lgtm-s1v-test-*` containers at the same time, and I left those alone.

| Run | Result |
|---|---|
| `deploy/test/proof/proof.test.sh` | 23 passed, 0 failed when run alone. The first run, made while I ran other suites, passed 22 and failed one (see post-all 6). |
| `deploy/test/vault-fixtures.sh` | 38 passed, 0 failed. |
| `deploy/test/postgres.sh` | 70 passed, 0 failed. |
| `deploy/test/seal.sh` | 94 passed, 0 failed. |
| `deploy/test/vault-init.sh` | 117 passed, 0 failed. |
| `deploy/test/stack-vault.sh` | 66 passed, 0 failed. |

## Mutations tried in scratch copies

| Item | Mutation | Did a test fail? |
|---|---|---|
| 8 | The disposable database publishes its port on every address instead of loopback. | Yes, two assertions of `vault-fixtures.sh`. |
| 2 | The health check probes the application role only. | Yes, one assertion of `postgres.sh`. |
| 2 | The authentication environment settings are removed from `compose.yaml`. | Yes, five assertions of `postgres.sh`, through the `pg_hba` check in `postgres-init.sql`. |
| 3 | The key guard is bypassed, so a missing key never stops `seal`. | Yes, three assertions of `seal.sh`. |
| 3 | A failed database probe is treated as "no rows". | **No.** All 94 assertions passed (post-all 1). |
| 4 | The policy path is widened to `lgtm/data/server/link/*`. | Yes, two assertions of `vault-init.sh`. |
| 6 | `up.sh` reconciles the database only when PostgreSQL was not already running. | Yes, two assertions of `stack-vault.sh`. |

I did not run the two mutations that need a long rebuild: "publish an invalid secret identifier" and "in-memory storage for the proof".
For the second, the assertion on the row growth and the checks of `bao.hcl` in `proof.test.sh` would catch it by reading.
I did not run a mutation of the job that calls `jq`.
The test `the pinned image has no jq` (`vault-init.sh` line 145) shows that such a job would fail on the real image.

## The places marked UNREVIEWED FIX

- **logic-r4 1, root token without `jq`.** `read_root_token` (`deploy/openbao-init.sh` lines 63 to 85) checks all 15 lines of the response and extracts the token with `sed`. The tests for a truncated response, the repair afterwards, and the absence of `jq` in the image all pass. The check is correct, and it is brittle (post-all 5).
- **logic-r4 2, empty identifier list.** The real daemon was exercised by `vault-init.sh` through the first start, the repeat, an existing identifier without the final file, and the credential repair. All pass. I did not run the mutation that treats the pinned empty reply as an error.
- **logic-r4 3, web migration.** PB8 belongs to sprint 2 and cannot be run now. The diagnostic `server vault credential needs repair` exists in `openbao-init.sh`. The web files in the working tree (`deploy/Caddyfile`, `deploy/test/web-rules.sh`) are untracked and belong to another team.

## Scenarios of the post-build checks

- **PB1, clean Linux launch.** Covered by the proof output (platform and digests recorded) and by the configuration assertions of `vault-init.sh`. I did not clear the image cache first.
- **PB2, unfinished initialization.** The tests cover a missing response, a truncated response and the repair. I reproduced a stronger case by hand (post-all 2).
- **PB3, secret handling.** The assertions "the job output holds no secret", "the daemon log holds no secret" and "the output of neither launch holds the credential" pass. I did not inspect the rendered configuration by hand.
- **PB4, restart and concurrency.** Two launchers, one through a symbolic link, both end ready with one identifier. A plain `restart` of both services keeps the signing key. I could not run a real host restart.
- **PB5, rollout hooks.** The wrapper works and several teams ran it at once without clashes. I could not finish the scenario. The hook `check-feed-lock.sh` refuses a checkout with changes under `src`, which is its design, and the working tree holds uncommitted store edits of another team. In a clean clone of HEAD, `with-postgres.sh -- deploy/test/checks.sh --only C12` printed `not ok 12 - C12 go test ./... failed against the configured PostgreSQL`, and the store tests failed in `receipts_test.go`. Those tests belong to the Server team, so I do not count them against this plan.
- **PB6 and PB7.** Covered by `vault-init.sh` as listed above.
- **PB8.** Not buildable in sprint 1.

## Findings

### post-all 1, MAJOR: no test pins the failure of the database probe in `seal`

**Statement.** The branch that stops `seal` when the probe of OpenBao storage fails is covered by no assertion, and the guard it belongs to protects the one thing that cannot be rebuilt.
**Evidence.** `deploy/stack-prepare.sh` lines 129 to 147: `rows=$(...) || fail 'Could not check OpenBao storage in PostgreSQL.'`. In a scratch copy I changed line 144 to `) || rows=f`. `deploy/test/seal.sh` printed 94 passed and 0 failed. A search of `deploy/test` for the diagnostic text finds nothing.
**Failure scenario.** A later edit makes the probe fail open. PostgreSQL is down or its password is wrong while the key file is missing. The script then answers `f`, believes the vault is new, and writes a fresh key over the place of the lost one. The vault stays sealed with the wrong key and the launcher reports no clear reason.
**Fix.** Add one assertion to `seal.sh` for item 4. Stop PostgreSQL, remove the key, run `seal`, and expect a failure with `Could not check OpenBao storage in PostgreSQL.` and no new key file.
**Ties to.** Plan item 4 and the decision of 2026-11-25 that OpenBao keeps its data in PostgreSQL and unseals from a key file (the row of the decisions table that names OpenBao).

### post-all 2, MINOR: a failure after `operator init` destroys the only copy of the root token and leaves the install stuck

**Statement.** The job deletes its temporary response on every exit, including exits after OpenBao has already been initialized, and a later run then stops with a diagnostic that leads nowhere.
**Evidence.** `deploy/openbao-init.sh` lines 155 to 160 (the `cleanup` trap removes `response_temp`) and lines 168 to 173 (`timeout 60 bao operator init ... > "$response_temp"`, then `protect`, then `publish`). I reproduced it. In a scratch copy I changed the owner given to `protect` at line 171 to an unknown user. The first launch failed with `chown: unknown user/group`. OpenBao reported `initialized: true`. The state folder held four files and no `admin/init-response.json`. A second launch with the real script printed `vault is initialized but bootstrap state is incomplete` and stopped.
**Failure scenario.** The init client times out at 60 seconds on a slow disk after the server has finished, or the job is killed between init and `publish`. The recovery response is gone, and every later launch fails the same way. The only way out is the wipe in `deploy/README.md`, and the launcher does not say so.
**Fix.** Keep the temporary file when the job fails after init and let the next run adopt it, or name the wipe procedure in the diagnostic. Raising the 60-second limit would help too.
**Ties to.** Plan (the first-start item).

### post-all 3, MINOR: the check for a host listener in `seal.sh` depends on the whole host

**Statement.** The assertion that OpenBao has no host port looks at every listening socket of the machine, and it passes when `ss` is missing.
**Evidence.** `deploy/test/seal.sh` line 160: `! ss -ltnH 2>/dev/null | awk '{print $4}' | grep -Eq ":8200$"`.
**Failure scenario.** A developer who runs another OpenBao on its default port gets a false failure. A host without `ss` gets a false pass, because the empty output makes the negation true.
**Fix.** Check the published ports of the project's own container with `docker port` or `docker inspect`, and fail when `ss` is absent if it is kept.
**Ties to.** Plan item 4 (OpenBao reachable only inside the stack).

### post-all 4, MINOR: `seal.sh` prints `write /dev/stdout: broken pipe` ten times

**Statement.** The run prints a stray error line next to passing assertions.
**Evidence.** `seal.out` of my run, after the assertions for the missing key (lines 24, 29, 34, 47, 79, 86, 90 and later). I did not trace the pipeline that closes early.
**Failure scenario.** The noise teaches readers to ignore errors in the output, and it can hide a real one.
**Fix.** Find the command whose output is piped into a reader that exits early, such as `grep -q`, and read its whole output first.
**Ties to.** Plan item 4.

### post-all 5, MINOR: any change of the pinned image gives a misleading diagnostic

**Statement.** `read_root_token` accepts only the exact 15-line, fixed-width shape of the response of the pinned OpenBao version, and it reports every other shape as `vault is initialized but bootstrap state is incomplete`.
**Evidence.** `deploy/openbao-init.sh` lines 63 to 85, with fixed widths 46 and 66 at lines 72 and 75.
**Failure scenario.** The owner bumps the image digest. The response gains a field or changes a width. The next first start fails closed with a message that blames the state, and it leaves a response file that is fine.
**Fix.** Say in `deploy/README.md` and in the plan's image-bump notes that this function must be re-checked on every bump. Optionally name the shape in the diagnostic.
**Ties to.** Plan (logic-r4 1).

### post-all 6, MINOR: the leftover check of the proof test breaks when other teams run tests

**Statement.** `proof.test.sh` compares every resource whose name starts with `lgtm-s1v` or `lgtm-proof` before and after the proof, so any parallel run changes the result.
**Evidence.** `deploy/test/proof/proof.test.sh` lines 47 to 53. My first run, made while other suites were running, printed 22 passed and 1 failed (`no container, volume or network of the proof remains`). Alone, the same test passed 23 of 23. The wrapper `with-postgres.sh` of other teams creates `lgtm-s1v-test-*` containers that match the filter.
**Failure scenario.** Two teams run tests in the same minute. One of them sees a failure that belongs to nobody.
**Fix.** Filter by the label that the proof sets on its own resources (`lgtm.s1v.fixture=<project>`), not by a name prefix.
**Ties to.** R10 (tests run against a real PostgreSQL) and plan item 1.

### post-all 7, MINOR: the proof test trusts the text that the proof prints

**Statement.** Most checks of `proof.test.sh` read lines that `proof.sh` itself prints, so a script that only prints them would pass.
**Evidence.** `deploy/test/proof/proof.test.sh` lines 56 to 90 read `assertion ... pass`, `image ...` and `rows before=... after=...` from the output. Independent checks exist only for the leftover resources and for `bao.hcl`. I reasoned this from the file and did not run such a script.
**Failure scenario.** A later edit of `proof.sh` drops the restart step and keeps the printed line. The test stays green.
**Fix.** Let the test read one independent fact, such as the digests of the images in the local image store, and the row count from the fixture before it is removed.
**Ties to.** Plan item 1.

## Not found

- No file or folder of the state is world-readable. The modes and owners match the plan, and the tests assert them.
- No secret reached a log, a job output or an error text in the runs and the assertions above.
- The lock in `deploy/up.sh` is on the canonical state folder and serializes two launchers, one through a symbolic link. The plan's wording "one per Compose project" matches the code when each project has its own state folder.
- No builder result file for these items exists under `.foundry/go-rewrite/results/`. I found no difference between the code and the "Done when" rows, so nothing relied on one.

## Grade

8 out of 10. All suites pass, six of seven mutations fail a test, and the one that does not is a missing assertion rather than a wrong implementation. The one reproduced defect (post-all 2) costs only an empty first install.
