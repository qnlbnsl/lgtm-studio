# Post-build review f of the amendment that freezes the link

- Task identifier: postbuild-go-s0-contract-f, nonce 76618c.
- Reviewer: a reviewer of built code that did not build this code and did not write its tests. Requested model claude-sonnet-5-5, high.
- Range read: `git diff 9e45479c..HEAD -- src/contracts scripts/check-go.sh`, on branch `sprint-0`, head `fcfa96c3`.
- Read-only review. Only this report was written. Scratch work was done in folders made by `mktemp -d`.

## Verdict

**SHIP.** Items F1 to F5 do what the plan says, the wire is unchanged, the history check fails closed, and no finding below blocks the next sprint.

## Top finding

The tests of the table of codes hold only the call, the code and the action of each row, so a prose table that says a machine stops on `event_gap`, or that lists `event_gap` under the stop row, still passes (post-f 1).

## What I ran

- **The wire is unchanged.** `buf build --exclude-source-info` at `9e45479c` and at `HEAD` both give md5 `ed99e5cf12616f6979a19bf24e2654e2`.
- **The generated code is current.** In a scratch copy of `HEAD`, `buf lint` and `buf generate` ran with no output, and `diff -r` of `gen/` against the committed `gen/` printed nothing.
- **The proto files are the drafts.** `cmp` of `draft/link.proto`, `draft/commands.proto` and `draft/events.proto` against the built files exits 0 for all three.
- **The tests.** `go test ./src/contracts/` prints `ok`. The six tests of item F4 and `TestLinkIsFrozenAtItsRevision` pass with `-v`. `TestFreezeCheckFailsClosed` passes as part of the package.
- **The two mutations the plan names.** Setting `after_verified_challenge` to false on the stop row of `Open` makes `TestLinkCodesAreWritten` fail at `rules_test.go:130` and `:152`. Appending one comment line to `events.proto` makes `TestLinkIsFrozenAtItsRevision` fail. The reordering of `unauthenticated` is covered by the builder's own run, and the test compares exact consecutive lines, so I accepted it.
- **The history check, in a clone at the freeze commit** (the tail of `scripts/check-go.sh` run alone, because `go test ./...` fails today in client stubs):

| Scenario | Result |
|---|---|
| The freeze commit itself, no `LGTM_LINK_BASE` | exit 0 |
| A later commit, no `LGTM_LINK_BASE` | exit 1, prints "The link is frozen, so set LGTM_LINK_BASE to the commit of the freeze." |
| A later commit, base is the freeze | exit 0 |
| Base is `nonsense` | exit 1 |
| Base is `9e45479c`, before the freeze | exit 1 |
| The record deleted | exit 1 |
| Revision 1 hash rewritten and committed, base is the freeze | exit 1 |
| Revision 2 added with the comma at the end of the revision 1 line | exit 1 |
| Revision 2 added with the comma at the start of its own line | exit 0 |

## Places marked UNREVIEWED FIX that belong to these items

- **logic-r5 04, the report whose only fault is a zero byte or a number above its limit.** The sentence is in `events.proto` (lines 28 to 31 of the table) and in item F2. It reads consistently with the rule that an `Event.number` of 0 or above 2^63 − 1 ends the stream. The ceiling is not yet in the client: `src/client/internal/link/outbox.go:28` still stops at `math.MaxUint64`. The request is written in `.foundry/go-rewrite/requests-from-contract-r5.md`, item 5, and another team's uncommitted `outbox_test.go` already holds the new test. See post-f 4.
- **logic-r5 05, the fallback and the condition of the table.** Built as planned. Both directions of the condition are tested, and every stop row of `Open` must hold it. The mutation fails as the plan says. See post-f 1 for what the test still accepts.
- **logic-r5 08, the history check.** Built, and it differs from the plan's text in two reasons that the builder's result file gives: it searches every parent with `HEAD^@` instead of `HEAD~`, so a merge cannot hide the freeze, and it also fails in a shallow clone. Both are stricter than the plan and I found no case where they reject a good run. See post-f 2 and post-f 3.

## Findings

### post-f 1 — MINOR — The table test accepts a wrong table written in prose

- **Statement:** item F4, `TestLinkCodesAreWritten`, and issue 1643 finding 2, which asked that `event_gap` stop being called final. Plan.
- **Evidence:** `rules_test.go` lines 140 to 152 compare only the call, the code and the action of each row, then check that the text contains the key, contains each listed reason, and holds the sentence "After a Challenge whose signature verified." exactly when the data says so. Nothing forbids an extra reason or a contradicting sentence. In a scratch copy of `link.proto`, each of these three edits left `go test -run 'Codes|Fallback|Order|Numbers|ProofComment'` at `ok`: adding "or event_gap" to the stop row of `Open failed_precondition` (lines 54 to 58); changing "It never answers a final code" to "It may answer a final code" in the `unavailable` row; and adding "and the machine stops" to the `event_gap` row.
- **Failure scenario:** a later edit reintroduces the exact fault of finding 2 in the text of the rows, and the client team reads "stop" for `event_gap`, while `link-codes.json` and the tests still say "dial again".
- **Fix:** in the test, fail when a reason listed in another row of the same code appears in this row's text, and require the stop rows not to contain "dial again" and the dial-again rows not to contain "stop". Alternatively, say in the plan that the test holds the columns only.

### post-f 2 — MINOR — The baseline of the history check is not tied to the history

- **Statement:** item F5, "The check of the history". Plan.
- **Evidence:** `scripts/check-go.sh` lines 36 to 49 use `LGTM_LINK_BASE` as given. In a clone, I committed a rewritten revision 1 hash and ran with `LGTM_LINK_BASE=HEAD`: exit 0. With `LGTM_LINK_BASE=fcfa96c3`: exit 1.
- **Failure scenario:** a dependent branch, or an agent that exports the variable from its own tip, passes the gate after rewriting revision 1, and the freeze is lost with no sign in the run.
- **Fix:** require `git merge-base --is-ancestor "$link_commit" HEAD` and refuse a base whose record holds fewer revision lines than the one in `.foundry/go-rewrite/link-base.txt`, or let the gate read its base from a file that a branch cannot change. The plan names the orchestrator as the one who passes the base, so a note in the README is the least change.

### post-f 3 — MINOR — A new revision must put its comma at the start of its line, and nothing says so

- **Statement:** item F5, the README line, and `TestLinkIsFrozenAtItsRevision`. Plan.
- **Evidence:** the line of revision 1 in `testdata/link-frozen.json` has no trailing comma, and the check compares that line byte for byte. Adding revision 2 the usual way, with a comma at the end of line 2, exits 1 with "The frozen link record must keep every revision line from LGTM_LINK_BASE in place." Starting the new line with a comma exits 0. The builder's result file records the second form; the README, the test and the error text do not.
- **Failure scenario:** the first person to make revision 2 follows the README, edits the line the check protects, and reads a message that does not say what to change. The record's tests pass, so the failure shows only at the merge gate.
- **Fix:** add the form to the README line, for example "write the new entry on its own line, with its leading comma", and name it in the error text.

### post-f 4 — MINOR — The client still hands out numbers above the ceiling the contract now states

- **Statement:** item 4 "Changes to built code", and `link.proto` value rules (an `Event.number` above 2^63 − 1 is refused with `invalid_argument`). Plan.
- **Evidence:** `src/client/internal/link/outbox.go:28` returns `ErrSequenceExhausted` only at `math.MaxUint64`. The request for the change is `requests-from-contract-r5.md` item 5. It is not in the committed client.
- **Failure scenario:** a client that reaches number 2^63 sends it, the server ends the stream with `invalid_argument`, which the table calls "dial again", and the client resends the same number forever. Reaching the number takes about 9.2 × 10^18 events, so this is a risk of the contract's wording only.
- **Fix:** none in the contract. The Client team makes the change, and the freeze should not be called settled in the plan's status until it lands.

### post-f 5 — MINOR — Two text items of the amendment are stale or not recorded

- **Statement:** "Which rules have a test", the rows for the encrypted join, and AGENTS.md habit 2 (a problem goes into `docs/issues.md`). Plan.
- **Evidence:** the builder's result file says that post-build check 3 "still calls the refusal after a database restore a final code" while the amended contract makes a gap "dial again". Its note on the issue for the deferred encrypted join and wrong-pin cases asks the orchestrator to record it, and `grep -i` for `wrong pin`, `wrong-pin`, `certificate_key` and `loopback token` in `docs/issues.md` finds nothing. `requests-from-contract-r5.md` says "None new" for that file.
- **Failure scenario:** the combined acceptance runs check 3 against the amended contract and fails for a reason that is only the plan's wording. The encrypted cases are dropped from wave 1 and appear nowhere.
- **Fix:** change check 3 to say "dial again" with the missing events available, and add one issue for the deferred encrypted join cases.

## Points the task asked for, with no finding

- **The wire is unchanged.** Same descriptor md5 before and after, and the generated code differs from a fresh `buf generate` in nothing.
- **The history check fails closed without `LGTM_LINK_BASE`.** It prints the plan's sentence at exit 1 on a later commit, passes on the freeze commit, and fails in a shallow clone and on a merge whose second parent holds the record.
- **Defects of the code itself.** The shell script quotes its variables, reads no secret, and leaves no open file. The test writes to `t.TempDir()` only, strips `GIT_*` and `LGTM_LINK_BASE` from the child environment, and disables git hooks and signing. `git rev-list` on a root commit would print a usage error and exit 129, which also fails closed, and this repository has no such case.
- **The record is only as strong as `go test ./...` reaching it.** In this checkout the script stops at the client tests that belong to another team, so the history check never ran in the real tree. I ran the same lines in a clone, as listed above.

## Grade

8 out of 10. The built code does what the plan says, and the two failing mutations and the history scenarios behave as written. The grade falls for the tests that accept a wrong table in prose, for a baseline that nothing ties to the history, and for a record format that the first change of revision will trip over.
