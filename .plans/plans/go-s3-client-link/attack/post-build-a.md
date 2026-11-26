# Post-build review A of the client link plan, item 1: the server address and the transport

- Task identifier: postbuild-go-s3-client-link-a
- Nonce: 8913fa
- Reviewer: claude-sonnet-5-5, high. I did not build this code and did not write its tests.
- Code read: `git diff d0f300e8..HEAD -- src/client/internal/transport` (commit 3f27e345 on top of the frozen tests of commit d0f300e8).

## Verdict

**SHIP.** Item 1 does what its text says, the frozen tests pass, and I found no blocker and no major defect.

The top finding is post-a 1: the wrapper that keeps the client on its own origin has no test that can fail, so a later edit could remove it without any test noticing.

## What I checked

### 1. Places no reviewer read before the build

The plan marks no `UNREVIEWED FIX` or `UNREVIEWED MECHANISM` inside item 1, and round 4's list "Not reviewed before the build" names only items 6, 7b and 8, the workstream 2 risks and the combined checks. Nothing in this review belongs to those places, so there was no scenario to run.

### 2. The code against the plan

Each sentence of item 1 maps to code, and I found no difference that the builder's result file does not already explain.

- `ParseOrigin` requires an explicit scheme and a host, lowercases both and drops the scheme's default port (`transport.go`, `parseOrigin`). It rejects credentials, paths, queries, fragments, whitespace, `%` and scoped addresses.
- Plain HTTP is accepted only for `localhost` or a parsed loopback address, and the dialer refuses any other destination (`loopbackDialer`). `localhost` is dialled only as `127.0.0.1` and then `::1`.
- The transport has no `Proxy` function and the client returns redirects unfollowed (`http.ErrUseLastResponse`).
- HTTPS pins the SHA-256 of the leaf's `RawSubjectPublicKeyInfo` in `VerifyConnection`, then checks validity dates, host name and the `h2` protocol. The check runs inside the handshake, so no HTTP byte leaves before it passes.
- I confirmed by experiment that `InsecureSkipVerify` does not skip the proof of the private key. A server that presents the genuine certificate with another private key fails with `tls: invalid signature by the server certificate: ECDSA verification failure`.

### 3. The tests, and the mutations in scratch copies

The folder was made with `mktemp -d` and is left at `/tmp/tmp.UQGigISTea`.

| Mutation of `transport.go` | Result |
|---|---|
| Remove the leaf pin comparison (the plan's named mutation) | The builder's own run failed `wrong pin sends nothing`. |
| Permit plain non-loopback HTTP (the plan's named mutation) | The builder's own run failed `rejected origins`. |
| Remove the validity-date check | Fails: expired and not-yet-valid leaf tests. |
| Remove the pin length check | Fails by a panic in the test run, which is a failure but not a clean assertion. |
| Add `Proxy: http.ProxyFromEnvironment` | **All tests pass.** |
| Remove the `h2` check | **All tests pass.** |
| Replace `loopbackDialer` with the plain dialer | **All tests pass.** |
| Remove `CheckRedirect` | **All tests pass.** |
| Remove the origin and the host-header comparison in `originTransport` | **All tests pass.** |

### 4. Defects of the code itself

`go vet`, `gofmt -l` and `go test -race -count=5` are clean. I found no race, leak, swallowed error or secret in an error text. `ParseOrigin` returns fixed errors and never echoes its input.

### 6. The tests of the plan

`go test -race -count=1 -v ./src/client/internal/transport` printed `PASS` for `TestOriginAndDialPolicy` (nine subtests) and `TestPinnedTransport` (thirteen subtests) and `ok` for the package.

## Findings

### post-a 1 — MINOR — The origin wrapper has no test that can fail

- **Statement.** Plan item 1: "a client that talks only to the given origin"; D17; R9.
- **Evidence.** `transport.go`, `originTransport.RoundTrip` and `matches`. In a scratch copy I replaced the whole condition by `true`, and again removed the host-header comparison. Every frozen test still passed. The frozen tests never send a request to a second origin or with a foreign `Host` header. The redirect test passes without the wrapper because `CheckRedirect` alone stops it, and it passes without `CheckRedirect` because the wrapper alone stops it. Each layer hides the other's removal.
- **Failure scenario.** A later edit removes the wrapper while tidying. A caller that builds a request from a URL it did not validate, such as the enrollment's token origin, then sends the join token to another endpoint, and no test of this package notices.
- **Fix.** Add a subtest that sends a request to a second live origin and one with `req.Host` set to another origin through a client built for the first, and asserts zero calls on the second handler. Add a subtest that stops the redirect with `CheckRedirect` alone by redirecting to the same origin.

### post-a 2 — MINOR — The "no environment proxy" check cannot fail

- **Statement.** Plan item 1: "a configured HTTP proxy cannot receive a loopback request"; "the transport uses no environment proxy".
- **Evidence.** `transport_test.go:361` to `:404`. Go's `ProxyFromEnvironment` never proxies a loopback destination, and every test origin is loopback. The mutation that adds `Proxy: http.ProxyFromEnvironment` passed all tests. The extra assertion at `transport_test.go:401` does `c.Transport.(*http.Transport)`, but the built client's transport is `*originTransport`, so the assertion is skipped silently. The builder's result file states this.
- **Failure scenario.** A later edit adds the environment proxy. A machine with `HTTPS_PROXY` set then sends the connection for an external server through that proxy, and the proxy sees the address, timing and size of the join. The pin still protects the token itself, so the harm is exposure of metadata, not of the secret.
- **Fix.** Use a non-loopback destination that the test controls, for example an HTTPS origin on a reserved name with a dial hook, or test an unexported constructor that returns the `*http.Transport` and assert `Proxy == nil`. Make the type assertion fail the test when it does not match.

### post-a 3 — MINOR — Two defence layers and the stolen-certificate case have no test

- **Statement.** Plan item 1 ("Risky. Yes, for token routing and certificate trust"); D17.
- **Evidence.** Replacing `loopbackDialer` by the plain dialer and removing the `h2` check each left every test passing. Every test server negotiates `h2`. Standard Go servers without HTTP/2 refuse the handshake themselves, so the check matters only for a peer that ignores the offered protocols. No test presents the genuine certificate with a different private key. I verified by experiment that the implementation refuses it, because `crypto/tls` checks the handshake signature even with `InsecureSkipVerify`, but the property rests on a library behaviour that no test of this package states.
- **Failure scenario.** A refactor replaces the `tls.Config` callback by a manual dial that reads the peer certificate without finishing the handshake. A party that has only the public certificate then passes the pin and receives the token.
- **Fix.** Add a subtest that serves the pinned certificate with another private key and asserts zero calls. Add a plain-HTTP subtest that calls the dialer function with a non-loopback address.

### post-a 4 — MINOR — Host names are loosely bounded and a trailing dot makes a second spelling

- **Statement.** Plan item 1 ("one origin validator"), and plan item 2, which compares the token's origin to the given address.
- **Evidence.** `transport.go`, `validHostName`. I ran `ParseOrigin` on these inputs. `https://example.test.` returns `https://example.test.`, which is not equal to `https://example.test`. A 300-character name, a 70-character label, `-.test` and `a_b.test` are all accepted. So are `https://2130706433` and `https://0x7f.1`, which a system resolver can read as `127.0.0.1`. The builder's result file states the trailing dot.
- **Failure scenario.** The Machines page shows the address without the dot and the owner types it with one. Item 2's equality check then refuses a token for the same server with a misleading "another origin" refusal. The numeric names cannot reach a loopback server with a token, because HTTPS needs the pin, so I found no security effect.
- **Fix.** Strip one trailing dot in the canonical form, limit the whole name to 253 bytes and each label to 63, and reject a name whose last label is all digits or starts with `0x`.

### post-a 5 — MINOR — The build follows the plan, but the design text and the owner's decision say something else

- **Statement.** D7 and D17 ("Encryption on the link" in `docs/specs/2026-11-25-go-rewrite-foundation-design.md`, line 316); `plan` decision rows 8 and 41.
- **Evidence.** The design says the join token carries "the fingerprint of the server's certificate" and that "the machine presents a self-signed certificate made from its own key". The code pins the leaf's public key and presents no client certificate. `link.proto` line 159 already names `certificate_key_sha256`, so the contract agrees with the code. The plan's decision rows 8 and 41 are still `unanswered`, and the plan says the owner must settle the point before freezing the transport tests. The builder's result file records the open boundary.
- **Failure scenario.** The owner later reads the design, expects a whole-certificate fingerprint, and finds that the client accepts any renewed certificate with the same key. The two readings differ in what a renewal needs: a fingerprint forces re-enrollment, a key pin does not.
- **Fix.** The owner records the decision on issue 1633, and the design sentence is changed to "the SHA-256 of the server certificate's public key". Nothing in the code needs to change under the current reading.

## Grade

**8 out of 10.** The code is small, matches the plan sentence by sentence, refuses everything the tests ask it to refuse, and passes `-race` repeatedly. The grade is held down by three layers (the origin wrapper, the proxy rule and the dial guard) that a later edit could delete without a failing test, and by the owner's open decision on the encryption boundary.
