# Post-build review A of go-s2-server-signin, items 1 to 5

Reviewer role: reviewer of built code, model claude-sonnet-5-5 at high effort. Task identifier: postbuild-go-s2-server-signin-a, nonce e511cf.
The reviewed range is `8f8eaa58..a6bff367` on the branch `sprint-0`. The reviewed folders are `src/server/internal/config`, `src/server/internal/auth` and the `go.mod` change.
The code has not changed between `a6bff367` and the current head.

## Verdict

REVISE.
The code does what the plan says, and every defect I looked for in the code turned out to be absent.
The frozen tests leave one security boundary unpinned, and that is the reason for the verdict.

## Top finding

No frozen test stops an HTTPS public origin from also admitting `https://localhost` and `https://127.0.0.1` at its own port as the same origin (post-a 1).

## What I ran

- `deploy/test/with-postgres.sh -- go test -race -count=1 ./src/server/internal/auth` printed `ok` after 244 seconds.
- `go test -count=1 ./src/server/internal/config` printed `ok`, and `go vet` on both packages printed nothing.
- I copied the two packages into a scratch folder without the untracked `api` folder and ran about 75 mutants there, through `deploy/test/with-postgres.sh`.
  Twelve are the mutations that the plan names. About sixty are my own.
- Of the plan's twelve mutants, eleven were stopped by a named test. The twelfth, "reconcile swallows the delete error", is equivalent: PostgreSQL aborts the transaction when the delete fails, so the commit fails anyway, and `TestReconcileRollsBack` still passes for the right reason.
- Of my own mutants, all but the ones in the findings below were stopped. Three more survivors are equivalent and are named in the last section.
- A handful of probe tests in the scratch copy exercised origin and cookie edge cases and the settings loader against process environment variables. I changed nothing in the repository and wrote nothing but this report.
- I could not run the Post-build checks PB1 to PB7. PB1 needs the deployed stack and a browser, PB2 needs the later proxy, PB3 to PB5 need the vault and the startup command of items 8 and 9, and PB6 and PB7 need later plans.
  The parts of PB1 that the packages can show alone are covered by tests I ran: the bundled loopback aliases, the 4096-byte password (`TestPasswordMatchesExactBytes`) and the refusal of a duplicated or malformed cookie (`TestReadsOneCanonicalCookie`).
- The task names a register "Not reviewed before the build" in `attack/`. No file there holds that heading (post-a 8). I used the plan's own `UNREVIEWED MECHANISM` marks instead.

## Findings

### post-a 1. MAJOR. The tests do not forbid loopback aliases under an HTTPS public origin

Statement: plan item 5, and the design section "Sign-in and web sessions", say that the server allows a website on another origin only when that origin is listed. The loopback aliases are meant for a plaintext loopback public origin alone.

Evidence: `src/server/internal/auth/origin.go` lines 56 to 61 add the aliases `127.0.0.1`, `localhost` and `::1` only when `public.scheme == "http"`.
I changed that condition to `true` in the scratch copy. Then `OriginPolicy.Check` admits `https://localhost` and `https://127.0.0.1` as the public mode for an HTTPS public origin.
`TestOriginPolicies`, `TestOriginPolicyConfiguration`, `TestCookieAttributes` and `TestReadsOneCanonicalCookie` all passed against that mutant.

Failure scenario: a later edit that widens the alias loop, for example while adding the second bundled address, would let a page served from `https://localhost` pass the browser origin gate of a server whose public origin is a real HTTPS name. No test would fail.

Fix: add cases to `TestOriginPolicies` that expect a refusal for `https://localhost`, `https://127.0.0.1` and `https://[::1]` when the public origin is HTTPS, at the same port and at the default port.

### post-a 2. MINOR. The settings tests accept a loader that refuses every remote database host

Statement: plan item 1 says to "check verified TLS for a non-loopback host".

Evidence: `src/server/internal/config/config_test.go` line 210 skips a host that fails to load with the comment that a refusal is as safe as verified Transport Layer Security (TLS).
Lines 184 and 193 do the same for the host `postgres` without the opt-in. No test loads a non-loopback host successfully and checks the verified TLS settings.
I added a refusal for every host that is not literal loopback, not a Unix socket and not the opted-in `postgres` (mutant `config-nonloop-refused`), and I also changed `verify-full` to `require` (mutant `config-verify`). `TestReadServerSettings` and `TestSettingsErrorsRedactValues` passed both times.

Failure scenario: a change that makes `Load` fail for every real database host, or that weakens its checks into a refusal, ships green and breaks every deployment with a database on another machine. The same test hole would hide a wrong TLS server name only if the loader still loaded.

Fix: add one positive case that loads a hostname such as `db.example.test`, then asserts `tls.ServerName` equals that host and `InsecureSkipVerify` is false. My probe shows the current code passes it.

### post-a 3. MINOR. Five behaviours of the settings loader have no test

Statement: plan item 1, which lists literal loopback only, the credential file checks, and the derived public origin.

Evidence, each confirmed by a mutant that every config test accepted:
- `config.go` line 101 treats only literal loopback addresses as plaintext. Adding `host == "localhost"` to that condition passed, so a database named `localhost` could silently skip TLS.
- `config.go` line 167 rejects `PGSERVICE`. Removing the check passed.
- `config.go` lines 52 to 55 derive the public origin only from a loopback listener. Dropping the loopback guard passed, so a wildcard listener `0.0.0.0:8080` would yield the public origin `http://0.0.0.0:8080`.
- `config.go` line 133 requires the owner or the effective group of the process. Replacing the check with `false` passed, because the fixtures are always owned by the test user.
- `config.go` line 122 opens the file with `O_NONBLOCK`. Removing it passed, so a named pipe at the path would block the start instead of being refused.

Failure scenario: each one is a silent regression of a safety rule that the plan states.

Fix: add a `localhost` case beside the other verified hosts, a test that sets `PGSERVICE` with `t.Setenv`, a wildcard-listener case with no public URL, and a named-pipe case. The ownership check can stay untested if the owner accepts that a normal test user cannot create a foreign-owned file.

### post-a 4. MINOR. Driver settings from the process environment other than `PGSERVICE` reach the pool

Statement: plan item 1 says to reject driver settings that substitute an endpoint or weaken verification. The code comment at `config.go` line 165 says the driver reads its process environment too.

Evidence: in a scratch probe, with `PGOPTIONS=-c search_path=evil` or `PGAPPNAME=x` set, `Load` succeeds and `Postgres.ConnConfig.RuntimeParams` holds `options` and `application_name`. `PGHOST`, `PGSSLMODE` and `PGTARGETSESSIONATTRS` change nothing, because the explicit settings win, and `PGSSLROOTCERT` pointing at a missing file is refused. Only `PGSERVICE` has an explicit check.

Failure scenario: an environment that sets `PGOPTIONS` changes the server's session defaults, for example the search path, without any line in the settings contract. The Store sets its own timeouts and isolation per transaction, so the effect is small today.

Fix: either refuse a non-empty `PGOPTIONS` as well, or state in the plan that the other `PG` variables are accepted, and add a test for the choice.

### post-a 5. MINOR. The standalone password file is read without a size limit

Statement: plan item 1 says standalone accepts one nonempty password line. The private-stack path is bounded to 66 bytes.

Evidence: `config.go` lines 136 to 140 wrap the reader in `io.LimitReader` only when `privateStack` is true. In a probe, a 200 MiB regular file owned by the user with mode 0600 was read whole by `io.ReadAll` before the loader refused it.

Failure scenario: a mistaken path, for example a database dump, is read entirely into memory at start. The file lies behind the deployment trust boundary, so this is a robustness defect and not an attack.

Fix: apply a limit in both modes, such as 4097 bytes, and refuse a longer file with the existing message.

### post-a 6. MINOR. No test pins that reconciliation ignores expired rows when it compares bindings

Statement: plan item 3, and the design text, say a changed binding on a live row purges all rows.

Evidence: `src/server/internal/auth/reconcile.go` line 62 skips expired rows in the binding loop. Removing that skip (mutant `reconcile-expired-derive`) passed `TestReconcilePassword`.
With the skip removed, an expired row that carries an old binding would purge the live rows, and the loop also pays a derivation for each expired row.

Failure scenario: a stale expired row left from an earlier password makes every restart log out the current sessions, once a later edit removes the skip.

Fix: add to `TestReconcilePassword` an expired row with a different binding next to live rows with the current binding, and expect that the live rows stay and the expired row is removed.

### post-a 7. MINOR. Two non-canonical inputs are accepted and no test mentions them

Statement: plan item 5 names the cookie reader `TestReadsOneCanonicalCookie` and requires exact origin tuples.

Evidence: `cookie.go` line 28 takes the value from the Go cookie parser, which removes surrounding double quotes. A probe showed `lgtm_session="<token>"` is read as the valid token.
`origin.go` line 148 parses the port with `ParseUint`, so `http://localhost:08787` matches the origin `http://127.0.0.1:8787` and `https://app.example.test:0443` is accepted as a public origin. A probe showed both.
Browsers send neither form, so no access is gained, but each admits a second spelling of one value.

Failure scenario: none that gives access. The risk is a later reader that compares the raw cookie text or the raw origin text and disagrees with this one.

Fix: either refuse both forms and add refusal cases to `TestReadsOneCanonicalCookie` and `TestOriginPolicyConfiguration`, or add accepting cases that record the choice.

### post-a 8. MINOR. The register "Not reviewed before the build" does not exist

Statement: `plan`, the assignment of this review.

Evidence: a search of `.plans/plans/go-s2-server-signin/attack/` for that heading finds nothing. Round 4 lists "Unreviewed mechanisms" in `round-4-logic.md` line 39, and the plan carries `UNREVIEWED MECHANISM` marks. The other post-build report made the same observation.

Failure scenario: a reviewer who relies on the register cannot tell whether a place was left out by mistake.

Fix: write the register from the plan's marks, or change the task template to name the marks.

## Code defects that I looked for and did not find

- No data race. The cache uses its own mutex, the two derivation slots are single-slot channels, and the full suite passed under `-race`.
- No swallowed error that matters. The one mutant that swallows a delete error is equivalent, as described above.
- No secret in an error or a log. Every settings error is a fixed sentence that names the variable, and the driver's parse error is replaced by a fixed sentence.
- The password file is opened with `O_NOFOLLOW`, checked on the open descriptor for type, ownership and a `0o027` mask, and closed on every path. The tests stop the `O_NOFOLLOW` and mode mutants.
- A stale cache entry after `Revoke` cannot grant access, because a cached tuple is consulted only after the row was just read from the database, and the row must match the tuple.
- The binding derivation, the token form, the three cookie modes, the Strict, `Secure` and `None` attributes, the 30-day lifetime, the eight-session cap, the database clock and the lock all have a test that stops its mutant.

## Equivalent survivors

- `password-empty-candidate`: removing the empty-candidate check changes nothing, because a configured password can never be empty.
- `config-fallback`: removing the check for fallback hosts changes nothing, because an earlier check already refuses a comma in the host.
- `plan-reconcile-swallow`: described above.

## Grade

8 out of 10.
The code is careful and matches the plan, and the tests stopped almost every mutant I tried, including all of the plan's risky ones except one equivalent mutant.
One point is lost for the unpinned origin boundary, and one for the settings tests that treat a refusal as a pass.
