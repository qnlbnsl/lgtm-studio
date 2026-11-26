# Post-build review b of the sign-in plan, item 8: the reader of the vault and the pin of the link's public key

- Task identifier: postbuild-go-s2-server-signin-b
- Nonce: ad5237
- Reviewer model: claude-sonnet-5-5, high effort.
- Branch and range: `sprint-0`, `git diff 8c5d6034..fb922e6d -- src/server/internal/vault`.

**This review is by a different model of the builder's vendor. Claude Opus built item 8, the reviewer of the other vendor was stopped by that vendor's content filter, and so the cross-vendor rule was not met.**

## Verdict

REVISE.

## Top finding

The plan's explicit hardening of the vault client, which is verified Transport Layer Security (TLS) and no proxy discovery, has no test at all, so a build with `InsecureSkipVerify: true` or with `http.ProxyFromEnvironment` passes every frozen test (post-b 1).

The production code itself follows the plan. I found no blocker. The revision asks for added tests and a few small code changes, and it does not ask for a different design.

## What I ran

- `deploy/test/with-postgres.sh -- go test -race -count=1 ./src/server/internal/vault/...` printed `ok  github.com/qnlbnsl/lgtm-studio/src/server/internal/vault  45.291s`, and `vaulttest` has no test files. The race detector reported nothing.
- `gofmt -l src/server/internal/vault` printed nothing, and `go vet ./src/server/internal/vault/...` exited 0.
- The vault folder in the working tree is byte-identical to commit `fb922e6d`, so the review covers the committed code.
- I did not run Post-build check PB4 (real OpenBao). The command `src/server/cmd` does not exist yet, the stack scripts belong to other teams, and containers of other agents were running, which I did not touch. I covered the same states with the wire fixture and a real PostgreSQL, as listed below.
- I did not find a file in `attack/` that holds the heading "Not reviewed before the build". I used the plan's own `UNREVIEWED MECHANISM` marks in item 8 instead.

## Check 1. The places no reviewer read before the build

The plan marks three places in item 8: the public pin that detects replacement even with the stack's permission (architect-r2 1, threat-r2 1), the pin table with the order of reading and writing (logic2-r1 4, logic-r1 8), and recovery under the existing pin (architect-r3 7).

| Place | Scenario | Result |
|---|---|---|
| Pin detects replacement. | PB5, a different valid seed (version 8) and an absent entry, both after an established pin. | Both fail the load, the pin and the feed stay unchanged, and no create request is sent. `TestLinkIdentityRefusesReplacement/replacement` and `/absence` pass. |
| Pin order and first write. | An interrupted first pin commit, with a trigger that refuses the feed row. | The load fails, no pin row and no feed row remain, and a restart adopts the stored seed with one create request in total. `/interrupted_first_pin` passes. |
| Recovery. | Restore the matching seed after a failure. | The load succeeds with the original fingerprint and the pin is unchanged. Passes. |
| Versioned deletion and sealing (PB4). | Wire fixture only: 404 with version metadata, destroyed, 503. | Every case fails the load with no create request. Passes, but the fixture has no pin in these cases (see post-b 4). |
| Real OpenBao (PB4). | Not run. | Open for the sprint's joint gate. |

## Check 2. The code against the plan

Every row of item 8's text holds in the code, and I list the ones I checked against lines.

- The credential is read from an opened descriptor with `O_NOFOLLOW`, as a regular file owned by the effective user with mode `0400` or `0600`, up to 4 KiB, with a token walk that refuses duplicate, extra, differently cased, empty and non-string fields (`vault.go:139-198`).
- The address policy admits HTTPS, plaintext on a literal loopback address, and plaintext on exactly `openbao:8200` after opt-in (`vault.go:112-131`).
- One client has no proxy, no redirect, a five-second request budget and a 64 KiB reply bound (`vault.go:79-101`, `:359-387`).
- Login uses `auth.client_token` and a positive `auth.lease_duration`, counts the lease from before the request, and refreshes under a mutex when one minute or less remains (`vault.go:202-247`).
- A 404 without `data` is absence, and a 404 with version metadata is refused (`vault.go:295-306`). The seed must be canonical standard base64 of 32 bytes, and the version must be positive (`vault.go:313-340`).
- Creation is one POST with `options.cas=0`, and its reply is ignored. The load always reads once more and uses only the stored seed (`identity.go:96-109`).
- The pin is read first. Without a pin and without `allowInitialize` the load fails before any OpenBao request (`identity.go:45-51`). With a pin the load never writes (`identity.go:58-71`). The first pin and its feed reference are written in one `Store.Write`, and the identity is returned after commit (`identity.go:89-93`, `:138-169`).

The one difference from the plan is post-b 8, and the builder's result file does not explain it.

A note on design references: the foundation design has no decisions table with the rows D1 to D23. I tied findings to the numbered rules ("Rules that every later part follows", R1 to R11) and to the numbered checks ("How wave 1 is proven done", C1 to C13) in that file, and otherwise to `plan`.

## Check 3. The tests

The plan names three mutations for item 8. I ran each in a scratch copy under a folder from `mktemp -d`, with the named test only, against disposable PostgreSQL.

| Mutation | Result |
|---|---|
| Remove compare-and-set (`options` field deleted from the create body). | `TestLinkSigningKeyCreateRace` fails at `vault_test.go:104`, `:417` and `:434`. |
| Use the local losing candidate after the follow-up read. | `TestLinkSigningKeyCreateRace` fails at `vault_test.go:434` and `:435`. |
| Skip the pin comparison. | `TestLinkIdentityRefusesReplacement` fails at `identity_test.go:61`. |

I then tried six wrong implementations that the plan's text rules out. The whole package test suite accepted all of them.

| Wrong implementation | Where | Suite result |
|---|---|---|
| Accept an existing pin that differs from the new fingerprint (`if false && existing != fingerprint`). | `identity.go:151` | `ok` |
| Use `http.ProxyFromEnvironment` instead of no proxy. | `vault.go:80` | `ok` |
| Set `InsecureSkipVerify: true` in the TLS configuration. | `vault.go:82` | `ok` |
| Open the credential without `O_NOFOLLOW`. | `vault.go:139` | `ok` |
| Remove the owner check (`stat.Uid != uint32(os.Geteuid())`). | `vault.go:153` | `ok` |
| Return `ErrReplaced` instead of `ErrUnavailable` for a pinned load when the vault is unreachable or sealed. | `identity.go:66-67` | `ok` |

## Findings

### post-b 1. MAJOR. Verified TLS and the absence of proxy discovery are untested

- Statement: Plan item 8 says "Verify TLS except for literal loopback and the opted-in private `openbao:8200` service" and "Use one bounded client with redirects and proxy discovery disabled". Rule R9 says secrets never reach a log, and the AppRole `secret_id` travels in the login body.
- Evidence: `vault.go:79-89` sets `Proxy: nil` and a TLS configuration. No test starts a TLS server or sets a proxy variable. `testOpenValidation` only checks that `Open` accepts `https://vault.example` (`vault_test.go:580`). Every wire test uses `httptest.NewServer` on `127.0.0.1`, and Go's `ProxyFromEnvironment` never proxies loopback requests, so even a proxy test through the loopback fixture would not see the mutation. The two mutations in the table above both print `ok`.
- Failure scenario: A later edit sets `InsecureSkipVerify` to ease a development stack, or switches to `ProxyFromEnvironment`. The suite stays green. In a deployment with an HTTPS vault address and an `HTTPS_PROXY` variable, the `secret_id` goes to the proxy, or to any host that presents a certificate.
- Fix: Add a test with `httptest.NewTLSServer` that expects a certificate failure with no login request received, and a second case with the fixture's own authority pool that cannot be injected, so the test shows verification is on. For the proxy, use a non-loopback listener address on the test host together with `HTTPS_PROXY` set to a counting listener, and require zero connections to it.

### post-b 2. MINOR. The credential file's symbolic-link and owner checks are untested

- Statement: Plan item 8: "On the opened descriptor require process ownership and mode `0400` or `0600`", and a regular file. R9.
- Evidence: `vault.go:139` (`O_NOFOLLOW`) and `vault.go:152-155` (owner). `testOpenValidation` covers modes `0644`, `0440`, `0700` and a directory, and nothing else (`vault_test.go:534-559`). Both mutations print `ok`.
- Failure scenario: A refactor removes the flag or the owner check and nothing fails. A symbolic link in the credential path, or a file that another user owns, is then accepted.
- Fix: Add a symbolic-link case and a named pipe case (which `O_NONBLOCK` and the regular-file check exist for). The owner check can be tested by `Chown` when the test runs as root, or skipped with a stated reason otherwise.

### post-b 3. MINOR. The pin write's conflict branch has no test

- Statement: Plan item 8: "Skipping the pin comparison must fail `TestLinkIdentityRefusesReplacement`". The builder's `writePin` has a second comparison, for a pin that another writer committed first.
- Evidence: `identity.go:146-155`. `TestLinkSigningKeyCreateRace` gives each competing load its own database (`vault_test.go:344`), so two writers never reach the same pin table. The mutation `if false && existing != fingerprint` prints `ok`.
- Failure scenario: Two loads that share a database, which the exclusive startup lock should prevent, but which a lock failure in sprint 3 or a restore could allow, each pin their own key and both return an identity. The code is right today, and nothing keeps it right.
- Fix: Add a case where a second store on the same database holds a different pin when `writePin` runs. A trigger that inserts the other pin before the first insert gives this deterministically. Require `ErrReplaced`, one pin row and no identity.

### post-b 4. MINOR. A pinned load reports "restore the key" for failures that are not a replacement

- Statement: R8, an error is one sentence the person can act on. Plan PB4: "Deletion and sealing close readiness."
- Evidence: `identity.go:66-70` maps `keyUnavailable` to `ErrUnavailable` and every other state to `ErrReplaced`. `keyInvalid` includes a malformed 200 reply and a non-JSON 404 body (`vault.go:299-301`), such as the page of a proxy or a missing mount that answers 404 with an error body. The operator then reads "Restore the established OpenBao signing key" while the key is intact. No test pins the pinned load against 503, 403 or a malformed reply, and the mutation that returns `ErrReplaced` for the unavailable case prints `ok`.
- Failure scenario: After a mount is misconfigured in the stack, a pinned server refuses to start and tells the operator to restore a key. The operator may write a second key into the wrong place or lose time.
- Fix: Map a malformed reply under a pin to `ErrUnavailable`, and reserve `ErrReplaced` for a 404 without data and for a canonical seed that differs. Add pinned cases for 403, 503 and malformed replies next to `replacement` and `absence`.

### post-b 5. MINOR. One sentence stands for five different startup failures, and a database error is swallowed

- Statement: Plan item 8: "Sanitize credential, login and key errors." Check C13 and R8 ask for a sentence the operator can act on.
- Evidence: `ErrEstablish` is returned for a failed pin read (`identity.go:46-48`), a malformed or deleted first read (`:86`), a create that left the entry absent (`:81`), a failed pin write (`:166`) and a random-source failure (`:102-103`). The underlying error is dropped each time.
- Failure scenario: The first start fails and prints "The server could not establish its OpenBao signing key." The operator cannot tell whether the migration is missing, PostgreSQL refused the write, the policy lacks `create`, or the entry was deleted.
- Fix: Add fixed sentences for "the public pin could not be read or written" and for "the vault refused the creation" and keep the existing ones. Keep them free of values, as the tests require.

### post-b 6. MINOR. The seed zeroization does nothing

- Statement: R9, secrets never reach a log. The builder's text says the seed is cleared.
- Evidence: `identity.go:111-113` takes `storedKey` by value, so `clear(key.seed[:])` clears a copy. The caller's `key` in `LoadLinkIdentity` (`:57`, `:76`) and the value returned by `parseKey` keep the seed until they leave scope. `LinkIdentity.SigningKey` also holds the seed in its last 32 bytes, as Ed25519 keys do.
- Failure scenario: There is no direct exploit. The code gives a reader the impression that the stack holds no seed after loading, and it does.
- Fix: Either take a pointer and clear the caller's value, or remove the `clear` and the claim. The second is honest, because the signing key must stay in memory for the life of the server.

### post-b 7. MINOR. The client has no close, and a rejected token stays cached

- Statement: Review check 4, a file or connection left open. R7, a part keeps nothing in memory that it cannot read again.
- Evidence: `vault.go:79-101` builds a private `http.Transport` with idle connections and a keep-alive dialer, and `Client` has no `Close` method, so `CloseIdleConnections` is unreachable. After a 403 on the key read, `currentToken` still returns the same token while its lease lasts (`vault.go:202-206`, `:282-310`).
- Failure scenario: Item 9's `TestStartAndRestartServer` starts and stops the application several times in one process. Each start leaves an idle connection and its goroutines for up to 30 seconds, which can trip a goroutine-leak check. After the stack rotates the role, a second load inside the lease reuses a token that the vault already refuses.
- Fix: Add `func (c *Client) Close()` that calls `CloseIdleConnections` and clears the token, and let item 9 call it after the load. Clear the cached token when a key request returns 403.

### post-b 8. MINOR. Item 9 cannot log the adoption label that the plan requires

- Statement: Plan item 8: "Keep one success event for creation and adoption ... A log label records adoption without identifying its cause."
- Evidence: `LinkIdentity` has the three fields the plan gives (`identity.go:16-20`), and `LoadLinkIdentity` returns no sign of whether it created the key, adopted a stored one, or confirmed an existing pin. Item 9, which writes the log line (plan line 502), cannot know.
- Failure scenario: After an interrupted first pin commit, the restart adopts the key and logs the same line as a first creation. The operator the plan means to help cannot see the adoption.
- Fix: Settle in the plan which side owns the label. The smallest change is a fourth field `Adopted bool` on `LinkIdentity`, set when the load found a stored key without a pin. The signature in the plan then changes, and item 9's builder needs to know.

## What I checked and found sound

- The wire requests are exactly one login, one read, at most one create and one follow-up read per load, and the fixture rejects a query string, a PUT, a PATCH, a DELETE and a versioned POST by failing on any unexpected method and path.
- The `Store.Write` closure sets a flag for a mismatch, so the retried closure cannot leave a stale value, and the error text is fixed.
- The race detector found no data race in the concurrent refresh test.
- The server policy file grants `read`, `create` and `update` (`deploy/openbao-server-policy.hcl`). The code uses only read and a create-only write. The plan accepts `update` and relies on the pin, so I report no finding.

## Grade

7 out of 10. The code does what the plan says and the named mutations are caught, but the hardening that protects the AppRole credential is unguarded by tests, and several small defects remain.
