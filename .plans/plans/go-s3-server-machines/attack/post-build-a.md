# Post-build review A: machine records and join tokens

- Task identifier: `postbuild-go-s3-server-machines-a`. Nonce: `0b18e1`.
- Reviewer of built code. Requested model and effort: claude-sonnet-5-5, high. I did not build this code and did not write its tests.
- Reviewed: items 1 and 3 of `plan.md`, which are `src/server/internal/machines/store.go`, `tokens.go`, `migrate.go` and the two migration files, at commit `d5ed52bb` on `sprint-0`. The frozen tests are `store_test.go` and `tokens_test.go`, and their hashes equal `.foundry/go-rewrite/frozen-s3machines-a.sha256`.
- The untracked files of item 4 (`join.go` tests and similar) are not part of this review.

## Verdict: SHIP

The code does what items 1 and 3 say, and every frozen test passes under `-race`. The top finding is that the frozen tests do not fail when the row locks of `SetApproved` and `SetName` are removed, so the lock order that decision 48 rests on has no test (post-a 1). Nothing here blocks the build. Grade: 8 out of 10.

## What I ran

1. **P1, each commit builds and fails on assertions before it passes.** I extracted `9f2cfa55` (machine records) and `d5ed52bb` (tokens) with `git archive` into a scratch folder. At `9f2cfa55`, `go vet` is clean, the store tests pass, and every token test fails on an assertion (the text does not start with `lgtm1_`, `NormalizeAddress` returns an empty string and no error), not on a build error. At `d5ed52bb` the whole package passes.
2. **The plan's acceptance and the full package under `-race`.** `deploy/test/with-postgres.sh -- go test -race -count=1 -v ./src/server/internal/machines/` printed `PASS` for all twelve tests of items 1 and 3 (`TestCreateWaitingAndRead`, `TestCreateWaitingListOrder`, `TestApproveRenameAndNames`, `TestDeleteWaitingSparesApproved`, `TestDeleteWaitingAndDeleteRecordRemovals`, `TestCreateTokenText`, `TestTokenLifetime`, `TestTokenStoresOnlyVerifier`, `TestTokenAddressRules`, `TestHostsServerMark`, `TestRevoke`, `TestTokenCreateKeepsExpiredRows`). `gofmt -l` prints nothing and `go vet` is clean.
3. **P6, a thousand expired tokens.** My own probe made 1,000 tokens, moved them one day back, and made one more. `Create` took 14 milliseconds, and all 1,001 rows stayed. This matches decision 92 and the builder's own probe, and no probe file remains in the tree.
4. **P5, the loopback address.** Only the part that belongs to item 3 can run now. `NormalizeAddress` returns `ErrNoCertificatePin` for every `https` address and refuses every host other than `127.0.0.1`, `[::1]` and `localhost`. The combined join needs item 4 and the stack, so I did not run it.
5. **The named mutations.** Dropping `AND approval = 'waiting'` from `DeleteWaiting`, a 16-minute lifetime, and storing the secret instead of the verifier each fail the test the plan names. I did not repeat them, because the builder's result file shows each failure and the first two are plain one-line changes. I ran 21 further mutants in a scratch copy, listed under the findings.
6. **Scratch probes.** Two concurrent approvals of one waiting machine gave `This machine does not wait for approval.` to one and success to the other, so the real code is correct. Scratch folder: the one `mktemp -d` made for this review, at the path in the scratchpad file `scratch-dir`. I left it there.

The places that no reviewer read before the build, for these items, are the paragraph on the loopback address (P5), and "No pruning" in item 3 (P6). Both behave as the plan says. The other rows of the register (P2 to P4 and the sprint 2 files) belong to items 2 and 4 to 7.

## Findings

### post-a 1 — MAJOR — Removing the row lock of `SetApproved` or `SetName` fails no test

- **Statement:** `plan` ("The order of locks", decision 48), and design rule R4 (every write goes through the store's transaction).
- **Evidence:** `store.go:131` and `store.go:158` read the row `FOR NO KEY UPDATE`. In a scratch copy I removed the lock clause from each, one at a time, and `go test ./src/server/internal/machines/` printed `ok` both times. `TestApproveRenameAndNames` is sequential, and `TestDeleteWaitingSparesApproved` holds a gate only for `DeleteWaiting`.
- **Failure scenario:** A later edit drops the lock, for example while changing the query. Two owner tabs approve one waiting machine under two names at once. Both reads see `waiting`, the second `UPDATE` waits and then runs, and the machine ends at revision 3 under the second name with no error for either caller. The plan's claim that approval and revocation form "one line, decided by the machine's row" then rests on nothing that a test checks.
- **Fix:** Add a test that races two `SetApproved` calls on one waiting machine, and one that races `SetName` against `SetApproved`. Each must show exactly one success and `ErrNotWaiting` for the other. The probe I ran in the scratch copy (`TestProbeConcurrentApprove`) is a starting point.

### post-a 2 — MINOR — Removing the row lock of `Revoke` fails no test

- **Statement:** `plan` ("The order of locks": a token's row is locked before a machine's row), D7.
- **Evidence:** `tokens.go:113` has `FOR UPDATE`. Removing it left every test green. `TestRevoke` is sequential.
- **Failure scenario:** Without the lock, a revocation races a redemption and can mark the token revoked after the machine was made, or leave `used_by` unread. Items 4 and 6 name tests for the race (`TestRedeemConcurrentUses`, `TestRevokeClientToken`), so the gap is closed only if those tests race `Revoke` itself, not just the page call.
- **Fix:** When item 4 is built, check that one of its tests holds a `Redeem` on the token's row and shows that `Revoke` waits. Otherwise add a test for it to `tokens_test.go`.

### post-a 3 — MINOR — `DecodeToken` keeps three guards that no test checks

- **Statement:** `plan` item 3 ("`DecodeToken` refuses another prefix, padding and trailing bytes"), D7.
- **Evidence:** `tokens.go:194` (the re-encoding must equal the payload), `tokens.go:185` (`Strict()`) and `tokens.go:182` (the check for `\r` and `\n`). Making each one inert left all tests green. The test's "trailing byte" case appends a zero byte, which the protobuf parser refuses by itself, so it never reaches the canonical check.
- **Failure scenario:** A refactor removes the canonical check. A token text with a duplicated field, or with non-zero trailing bits in the base64, or with a newline in it, decodes and gives a second text for one token. The client of the other plan then accepts a text that the server would never have made.
- **Fix:** Add cases to the `refused` map of `TestCreateTokenText`: the payload followed by a second copy of its `token_id` field, a last character whose unused bits are not zero, and a text with `\n` in the middle.

### post-a 4 — MINOR — Three guards of `NormalizeAddress` have no test

- **Statement:** `plan` item 3 ("refuses a path, a query, a fragment, a user, another scheme and a host other than..."), R9.
- **Evidence:** `tokens.go:134` (the `%` and white space refusal), `tokens.go:139` (a host that ends in a colon) and `tokens.go:145` (port 0). Each guard made inert left all tests green. For port 0, `http://127.0.0.1:0` would then be accepted and normalized to `http://127.0.0.1`, an address on port 80 that the owner did not give.
- **Failure scenario:** A configured `PublicURL` of `http://127.0.0.1:0` gives tokens that point at port 80, and a client of wave 1 fails to join with no clue why.
- **Fix:** Add `http://127.0.0.1:0`, `http://127.0.0.1:`, `http://127.0.0.1%3a8787` and `http://127.0.0.1:8787 ` to the `invalid` list of `TestTokenAddressRules`.

### post-a 5 — MINOR — A missing server key is a plain Go error that a caller will report as a database outage

- **Statement:** R8 (an error is a Connect code with one sentence the website can show) and `plan` ("Errors that cross the store").
- **Evidence:** `tokens.go:29` makes `errInvalidServerKey` with `errors.New`, not `store.NewDomainError`, and `Create` returns it at `tokens.go:70` before any store call. In my probe, `Create` with an empty `ServerKey` returned `The server does not have a valid signing public key.` and `errors.As` for `*store.Error` was false. The plan's mapping ends with "every other error gives `unavailable`", and its sentence for `unavailable` is "The server cannot reach its database, try again." Both error texts also start with a capital letter and end with a full stop, which Go tools flag.
- **Failure scenario:** The vault returns no signing key at start-up, the owner presses "make a token", and the page says the database is unreachable, so the owner looks at the wrong service.
- **Fix:** Decide where the server key is checked. The best place is `NewTokens`, which can refuse a key of the wrong size at start-up, so the call never has the case. Otherwise give the error its own code in item 6's mapping.

### post-a 6 — MINOR — The registry accepts two names that look the same

- **Statement:** `plan` item 1 ("A unique violation of `machines_approved_name` becomes `ErrNameTaken`"), R8.
- **Evidence:** `store.go:174` trims and checks bytes only, and the index is `lower(name)`. In my probe, `café` written precomposed and `café` written with a combining accent were both approved on two machines, while `CAFÉ` was refused. The plan does not name Unicode normalization.
- **Failure scenario:** Two machines show the same name on the page, and the owner cannot tell which agent runs where. This is small for one user, and security issues do not block the build, so I rate it minor.
- **Fix:** Either normalize the name to NFC in `NormalizeName`, or write the choice down in `decisions.md` as outside wave 1.

### post-a 7 — MINOR — The schema leaves two invariants to the code of item 4

- **Statement:** `plan` item 1 and item 3 (the tables are copied byte for byte), D7 ("used once"), R7.
- **Evidence:** `0001_machines.sql` has no unique constraint on `token_id` and no check that an approved machine has a name. `0002_client_join_tokens.sql` has no check that `used_key`, `used_at` and `used_by` are set together. The builder followed the plan exactly, so this is a property of the plan, not a defect of the builder.
- **Failure scenario:** If item 4's lock is ever wrong, two machines could share one token, and the database would accept it. A later correction is a new numbered change, so it is cheap now and costs a migration later.
- **Fix:** When item 4 is merged, add `0003` with `UNIQUE (token_id)`, `CHECK (approval = 'waiting' OR name <> '')` and a check that the three `used_*` columns are all null or all set. Or record in `decisions.md` that the code alone guards them.

### post-a 8 — MINOR — `SetApproved` and `SetName` normalize the name again, which no test checks

- **Statement:** `plan` item 1 and its page-call rule ("A page's call normalizes a name before `WriteOnce`, so `ErrInvalidName` never crosses the store"), and the builder's result file, which does not explain it.
- **Evidence:** `store.go:126` and `store.go:153` call `NormalizeName` inside the transaction. Removing the call at `store.go:126` left every test green, and my probe showed `ErrInvalidName` returned for an unknown machine identifier and an invalid name, so the sentence "never crosses the store" is no longer true of the registry.
- **Failure scenario:** None today. `ErrInvalidName` is a domain error, so it passes the store unwrapped. The risk is only that item 6 relies on the plan's sentence and maps it wrongly, or that the defence silently disappears.
- **Fix:** Either remove the second call and keep the plan's rule, or keep it and add one test that calls `SetApproved` with `" Box "` and reads back `Box`, and update the plan's sentence.

## Checks with no finding

- **A secret in a log, an error or the feed.** `Create` writes nothing to a log. Its errors name no secret or verifier. `recordTokenChange` records only the token's identifier. `TestTokenStoresOnlyVerifier` covers the database columns and the feed.
- **A race in the code.** `Registry` and `Tokens` keep no mutable state. `NewTokens` clones the server key. `-race` is clean.
- **A leak, an open file or a permission.** The code opens no file and no connection of its own, and the embedded migrations are read-only.
- **The lifetime.** `expires_at` is one reading of `clock_timestamp()` plus 15 minutes, read back from the row, as critic-r1 12 and logic-r1 11 ask.
- **The lock order of `DeleteWaiting` against `SetApproved`.** `DELETE ... WHERE approval = 'waiting'` re-checks the row after a concurrent approval commits, and `TestDeleteWaitingSparesApproved` fails when the condition is removed.

## Mutants that the tests caught

Making each of these changes in a scratch copy turned at least one frozen test red: dropping the `approval = 'waiting'` condition; a 16-minute lifetime; storing the secret; skipping the user-information check; allowing any host; not recording the token's creation, its revocation, an approval or a rename; ordering the list by `id`; treating a same-name rename as a change; `Delete` of an unknown machine returning no error.

## Grade

8 out of 10. The code matches the plan and the design for both items, and no secret reaches a place it should not. The marks are lost for locks and decoder guards that no test checks, and for one error that will be shown as a database outage.
