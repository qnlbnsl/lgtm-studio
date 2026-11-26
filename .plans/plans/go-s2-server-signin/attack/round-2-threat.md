# Round 2 threat review of go-s2-server-signin

- Task identifier: review-go-s2-server-signin-r2-threat
- Nonce: d5202e
- Lens: threat
- Scope: the patches between commits 32c6c345 and f1dcbb7d in `plan.md` and `decisions.md`, and what follows from them.

## Verdict

REVISE

## Top finding

The plan asks the finished vault plan to restrict the signing-key policy to read and create, but the vault plan and its built test still require update, and no owner, file or test is named for the change (threat-r2 1).

## Findings

### threat-r2 1 — MAJOR — The signing-key policy change has no owner and contradicts a built test

- Tied statement: C13 and the Server item 8 sentence that asks Stack to restrict the key path to read and create (`plan.md:439`).
- Evidence:
  - `.plans/plans/go-s1-stack-vault/plan.md:209` grants `read, create and update` at `lgtm/data/server/link/signing-key`, and the plan is marked done.
  - `deploy/test/vault-init.sh:73` asserts that the policy grants exactly `create read update`.
  - `deploy/test/vault-init.sh:15` defines `UPDATE_KEY`, and a check named "the server may update the key with the right version" requires an update with `cas=1` to succeed.
  - `.plans/plans/go-s1-stack-vault/plan.md:225` says the first write uses `options.cas=0`, which needs only create.
  - Round 1 asked for the narrower policy, so the patch is right in intent. The handoff is what fails.
- Failure scenario:
  - The Server builder follows `plan.md:439` and assumes the policy has already been narrowed.
  - No task changes `deploy/openbao-init.sh` or `deploy/test/vault-init.sh`.
  - PB4 expects that no update is authorized by the role, and it fails because the role still holds update.
  - If someone narrows only the policy file, the built check at `deploy/test/vault-init.sh:73` fails instead.
  - If neither happens, a compromised server process can overwrite the signing key with a new version.
  - Every client trust pin and every link token then follows that key, so the fix is a security property and not tidiness.
- Prescribed fix:
  - Name Stack as the writer in the plan, with the files `deploy/openbao-init.sh`, `deploy/test/vault-init.sh` and the vault plan line 209.
  - In the vault test, change the line 73 assertion to exactly `create read`.
  - Replace the `write_update` success check with a check that an update is denied with 403.
  - Add the same change to the task list that the orchestrator hands to the builder, so it does not depend on a sentence inside item 8.

### threat-r2 2 — MINOR — Loopback aliases are admitted without a condition on the public origin

- Tied statement: D-series cookie and origin rules, item 5 (`plan.md:244`, `plan.md:246`, `plan.md:248`).
- Evidence: line 244 admits `127.0.0.1`, `localhost` and `[::1]` as plaintext origins "on the configured loopback port" and counts them as same-site. It does not say that the public origin must itself be plaintext loopback.
- Truth table, with the listener on a loopback port:

| Public origin | Request `Origin` | Plan text today | Expected |
| --- | --- | --- | --- |
| plaintext loopback at port P | alias at port P | admitted, unprefixed cookie | admitted |
| plaintext loopback at port P | alias at another port | refused | refused |
| HTTPS | alias at the listener port | admitted, unprefixed cookie without Secure | refused |
| HTTPS | the HTTPS origin | admitted, `__Host-` cookie | admitted |

- Failure scenario: an operator serves the website over HTTPS and also forwards the listener to a loopback port for debugging. A page at `http://localhost:<port>` then gets a credentialed answer and a plain `lgtm_session` cookie that is not Secure, in a deployment that promised HTTPS only.
- Prescribed fix: admit the aliases only when the public origin is plaintext loopback, and only at that port. Add the third row as a case in `TestOriginPolicies`.

### threat-r2 3 — MINOR — The unprefixed loopback cookie can be shadowed from another port

- Tied statement: the cookie-name rule at `plan.md:246`, with the unprefixed `lgtm_session` for plaintext loopback.
- Evidence: cookies are not scoped by port, and the unprefixed name has no `__Host-` protection.
- Failure scenario: an agent-run preview server on `localhost:5173` sets `lgtm_session=junk; Path=/`. The browser then sends the junk value to the LGTM server on another port. `ReadCookie` reads only the selected name and finds a value that does not verify, so the owner is signed out repeatedly until the cookie is cleared. The impact is lockout, not access.
- Prescribed fix: record this as a residual risk in the risks table, and add a post-build check that a shadow cookie only signs the browser out. Do not add code for it, because the owner decided there is one user and a local setup.

### threat-r2 4 — MINOR — The module pins have no verification gate, and the indirect bumps are not listed

- Tied statement: plan part for module pins (pgx v5.13.0, x/crypto v0.59.0, x/net v0.63.0, connect v1.21.1, protobuf v1.36.12).
- Evidence:
  - I checked the module proxy myself. All five versions exist, and each is a tagged commit.
  - `go.mod` today has no `golang.org/x/crypto` and no `golang.org/x/net`, so item 2 adds them.
  - The proxy shows that x/crypto v0.59.0 and x/net v0.63.0 require `golang.org/x/sys v0.50.0`, `golang.org/x/term v0.48.0` and `golang.org/x/text v0.44.0`. `go.mod` has sys v0.29.0 and text v0.41.0.
  - The plan lists no step that runs `go mod verify` or a vulnerability scan.
- Failure scenario: adding the two modules silently raises three indirect modules that the plan never names. A reviewer who compares the changed `go.sum` to the plan sees unexplained lines, and nobody checks the downloaded content against the checksum database.
- Prescribed fix: list the three indirect bumps in the plan. Add `go mod verify` and `govulncheck ./...` to the build gate, and fail on a `go.sum` difference that the plan did not list.

### threat-r2 5 — MINOR — Scrypt derivation happens near the table lock, and the compare is not named constant time

- Tied statement: item 4 `Check` (`plan.md:171`) and `Revoke` (`plan.md:173`), and the clock sample taken under the table lock (`plan.md:169`).
- Evidence: `Check` on a cache miss and `Revoke` both derive a binding with scrypt, which uses about 64 MiB and a noticeable time, and the clock sample is taken under a `SHARE ROW EXCLUSIVE` lock. The plan names no rule that the derivation runs outside the lock. It names no constant-time comparison for the binding.
- Failure scenario: an attacker with only database write access, or a stale row after a password change, causes repeated cache misses. Each miss derives while a sign-in waits for the lock, so sign-ins stall for the derivation time multiplied by the number of rows, which is at most eight. A byte-by-byte comparison of the binding would also leak the position of the first difference, although only a local process could time it.
- Prescribed fix: read the row, derive outside the lock, then re-validate the row under the lock. Require `subtle.ConstantTimeCompare` for the binding and have `TestCheckRejectsForeignBinding` fail on a plain `bytes.Equal`.

### threat-r2 6 — MINOR — A restored database re-pins whatever the vault holds, with no distinct log

- Tied statement: item 8 `LoadLinkIdentity` with `allowInitialize` (`plan.md:441`).
- Evidence: a missing pin with `allowInitialize` allows create or adopt, and the first pin is written through `Store.Write`. The log event `link_signing_key_loaded` has the fields `source`, `fingerprint` and `vault_version`, with no field that says the pin was new.
- Failure scenario: the database is restored from an older copy that lacks the pin table row, while OpenBao holds a different, newer key. The server adopts the vault key and pins it, and the owner and clients see only the ordinary load event. A swapped vault is indistinguishable from a first start.
- Prescribed fix: add a `pin` field with the values `existing`, `created` and `adopted` to the event. Check it in PB4.

### threat-r2 7 — MINOR — The database credential file check covers only the last path component

- Tied statement: item 1 credential-file paragraph (`plan.md:402`).
- Evidence: the file is opened without following symlinks, and the mode and owner are checked, with Compose giving owner 70, group 10001 and mode 0440.
- Failure scenario: `O_NOFOLLOW` guards only the final component. A parent directory that another local user can replace lets that user swap the whole path. Mode 0440 also gives group 10001 read access to the database password.
- Prescribed fix: state in the plan that the parent directory is trusted and owned by the stack user. Check the mode and owner on the opened descriptor, as the OpenBao file check does.

## Answers to the lens questions

1. Trust boundaries: the patches touch the cookie and origin boundary, and finding 2 covers the loopback alias gap.
2. Inventory of secrets: the pin table and the link signing key are covered. Findings 1 and 6 cover the key's write rights and its adoption.
3. How each credential ends: nothing new found. A password change ends sessions through the binding check, and the lock-loss window is closed by the runtime check on each call.
4. Encryption in transit and at rest: nothing found beyond finding 2. The cookie modes follow the foundation design at lines 195 to 245.
5. Pinned dependencies and trust: finding 4. I checked all five pinned versions at the module proxy.
6. Integrity against untrusted parties: findings 1, 2 and 3 cover a compromised server, a plaintext page and a sibling local page.
7. Post-build checks: listed below.

## Answers to the task questions

1. Patches against their findings: each patch removes its failure as written, except the signing-key policy patch, whose handoff is incomplete (finding 1). I found no other patch that breaks another statement.
2. `UNREVIEWED MECHANISM` places, which are the password-change reconcile at item 3, the risk row 193 and the pin migration at line 441:
   - The reconcile is sound with the instance lock and the runtime binding check.
   - The pin is attacked in findings 1 and 6.
3. Rejected findings: the register lists no finding with the outcome "rejected". The only open outcomes are "for the owner" (logic2-r1 2, 3 and 6, critic-r1 5 and logic-r1 4). Nothing found.
4. Names others build against: the lock ID `841941100003` is already in `src/server/internal/store/locks.go`. `NewDomainError` and `Error.Outcome()` appear in the Store plan. The retired setting names and the 11 Compose keys match the stack-web plan. The one incomplete list is the policy path (finding 1).
5. Contradictions: the cookie names against the foundation design are consistent. The only interaction that needed a truth table is origin by mode (finding 2). Nothing else found.
6. Round-1 blockers and majors still open although marked fixed: the signing-key restriction (finding 1) is marked fixed, but it is open as a handoff. Nothing else found.
7. Lens questions touched: answered above.

## Post-build checks

1. A write to an existing signing key by the server role returns 403, and a follow-up read returns the winner. Expected: no update right in the policy.
2. With an HTTPS public origin, a request from an alias origin at the listener port is refused. Expected: no credentialed answer and no unprefixed cookie.
3. A `lgtm_session` cookie set from another loopback port only signs the browser out. Expected: no access and no server error.
4. `go mod verify` and `govulncheck ./...` pass at the pinned versions. Expected: no output and exit code 0.
5. A forged-row `Revoke` runs while a sign-in waits. Expected: the sign-in finishes within one derivation time.
6. A restore with a missing pin and a populated vault logs `pin: adopted`. Expected: the event differs from the ordinary start.
7. PB1 to PB7 of the plan still pass after the changes above.

## Grade

7 out of 10. Counts: 0 blockers, 1 major, 6 minors.
