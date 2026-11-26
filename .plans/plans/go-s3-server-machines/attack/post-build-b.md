# Verdict — REVISE

A malformed Join body can put submitted credential text in the error response before the handler applies its fixed sentences.

The builder was a worker of my own vendor, so the requirement for another vendor to review the code was not met.

I reviewed item 4 of `go-s3-server-machines` at `f7efc767` on `sprint-0`, using `85363e3e` as the diff base. The task identifier is `postbuild-go-s3-server-machines-b` and the nonce is `88def9`.

## Post-build checks

- **P1, the item builds on its own.** At the pre-build commit `bf1b4f54`, both acceptance commands compiled and exited 1 on assertions. The redemption stub returned an empty machine identifier. The Join stub returned `unimplemented`. `TestOpenUnimplemented` already passed. At the item build commit `28ef61ae`, the link package printed `ok` in 14.308 seconds and `go vet ./src/server/...` exited 0 without output. Redemption acceptance exited 1 after `TestRedeemConcurrentUses/two_keys` timed out observing the token lock. The builder's result explains the activity snapshot retained by the test's transaction. The later commit clears that snapshot before each poll, and the current acceptance and race runs passed. [Activity snapshot refresh](../../../../src/server/internal/machines/join_test.go#L132)
- **P2, Join through the web proxy.** I ran the pinned Caddy executable with the stack's proxy configuration adapted to disposable loopback ports, the application router, and a fixture-backed Join service. A request larger than 64 kibibytes returned `invalid_argument` and created no machine. The expected `resource_exhausted` depends on the size bound owned by item 2. A normal request without a website session created a waiting machine and returned `Cache-Control: no-store`. The replay returned the same machine, and Open returned `unimplemented`. App startup wiring is item 7. [P2](../plan.md#L348) [Router](../../../../src/server/internal/api/server.go#L145)
- **P5 and checks 5 and 6, the combined loopback join.** The proxy run covered the loopback Join call and the waiting row in PostgreSQL. The full loopback join, the page's waiting update and approval, and the stack command `deploy/test/checks.sh --only C5,C6` could not run against the reviewed revision. Its application startup and client command were stubs at `f7efc767:src/server/internal/app/start.go:25` and `f7efc767:src/client/cmd/lgtm-client/main.go:19`.
- **P9, where the credentials travel.** The normal server-side paths ran in `TestJoinKeepsSecretsOutOfLogs`, `TestJoinRefusalMetadata` and `TestTokenStoresOnlyVerifier`. By reading, redemption hashes the secret locally, the feed carries record identifiers, the store discards raw database errors, and Join has no log call. The normal response carries the public signing key as specified. The decode path has the separate error-text issue in post-b 1. The full client log and process checks require the combined stack. [Verifier](../../../../src/server/internal/machines/tokens.go#L211) [Store error handling](../../../../src/server/internal/store/pool.go#L99) [Join response](../../../../src/server/internal/link/join.go#L40)

## Findings

### post-b 1 — MAJOR — The decoder can echo submitted credentials in an error

- **Statement.** Rules R8 and R9 require a fixed public sentence and protection of secrets. The plan requires the fixed `invalid_argument` sentence for Join.
- **Evidence.** By reading, the router supplies no sanitizing codec for LinkService. The generated handler accepts JavaScript Object Notation (JSON) and decodes the body before calling `Join`. In the pinned Protocol Buffers dependency, `encoding/protojson/decode.go:335`–`349` includes the raw field value when a bytes field fails to decode. Connect propagates the decoder text in `codec.go:170`–`184` and `protocol_connect.go:1182`–`1183`. Join's fixed message at `link/join.go:23` applies only after a successful decode. [Router](../../../../src/server/internal/api/server.go#L145) [Generated handler](../../../../src/contracts/gen/go/lgtm/link/v1/linkv1connect/link.connect.go#L326) [Existing protection for session JSON](../../../../src/server/internal/api/session_decode.go#L20)
- **Failure scenario.** A client misencodes its join credential or key in a JSON bytes field. The resulting error can carry the submitted text into client diagnostics.
- **Fix.** Sanitize LinkService decoder errors before they are serialized. Use a fixed sentence and add a wire test that checks the response with synthetic credential markers.

### post-b 2 — MINOR — Join reports stopped requests as database outages

- **Statement.** The plan's error precedence and decision 85 place context errors after domain refusals. Rule R8 calls for a sentence that describes the error.
- **Evidence.** The store can return the request context's error. The handler maps all remaining errors to `unavailable`. A scratch probe called the handler with canceled and expired contexts. Both returned `unavailable`, and neither created a machine. The plan's code table and the frozen link's pre-commit failure sentence also use the broad word `unavailable`. [Plan error precedence](../plan.md#L76) [Plan code table](../plan.md#L109) [Store context return](../../../../src/server/internal/store/pool.go#L301) [Join error return](../../../../src/server/internal/link/join.go#L40) [Frozen pre-commit rule](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L96)
- **Failure scenario.** When a Join stops on its request deadline while the database is healthy, the handler presents the database outage sentence. The transport can mask the mismatch when the client deadline fires locally.
- **Fix.** Bring the plan, contract and handler into agreement on context errors. Under decision 85, map cancellation and deadlines explicitly while keeping database failures as `unavailable`. Pin both cases with a handler test.

### post-b 3 — MINOR — Protocol Buffers decodes the host name before the refusal and replay decisions

- **Statement.** The plan and the frozen link place host-name validation after the refusals and replay.
- **Evidence.** By reading, the frozen request defines `host_name` as a Protocol Buffers `string`. The pinned Go decoder validates Unicode Transformation Format, 8-bit (UTF-8), while decoding that field in `internal/impl/codec_gen.go:5052`–`5064`. Connect invokes that decoder before the handler. Redemption correctly defers the zero byte and length checks, which can be represented in a decoded request. [Frozen Join order](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L75) [Host-name field](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L210) [Deferred check](../../../../src/server/internal/machines/join.go#L121) [Wire test cases](../../../../src/server/internal/link/join_test.go#L357)
- **Failure scenario.** A retry whose host-name field contains invalid UTF-8 fails decoding with `invalid_argument` before Join can return the original reply or apply a token refusal.
- **Fix.** Clarify that the two-phase rule applies to successfully decoded requests, with malformed wire messages refused by the decoder. If the contract requires late validation of invalid UTF-8 on the wire, specify a representation that can carry those bytes.

### post-b 4 — MINOR — The link tests accept a Join success without the no-store header

- **Statement.** The plan's P9 requires `Cache-Control: no-store`.
- **Evidence.** The handler sets the required header today. `TestJoinCall` only rejects a header containing `public`. I removed the header assignment in a scratch copy, and all link tests still printed `ok` in 21.810 seconds. [Header assignment](../../../../src/server/internal/link/join.go#L43) [Header assertion](../../../../src/server/internal/link/join_test.go#L208) [P9](../plan.md#L355)
- **Failure scenario.** A later response refactor drops the no-store directive and the package tests still pass.
- **Fix.** Require the no-store directive on both the first success and a replay through the client interface.

### post-b 5 — MINOR — The frozen rollback test stops before the first insert succeeds

- **Statement.** Rule R4 and the frozen link require the machine creation, token claim and their changes to commit together.
- **Evidence.** `TestRedeemRollsBack` rejects the initial machine insert, so it does not exercise failure after that insert succeeds. In a scratch probe, PostgreSQL observed the new machine inside the transaction and then refused the token update. The current code returned a sanitized rolled-back database error, left the machine, token and feed unchanged, and created one machine on retry. [Frozen rollback test](../../../../src/server/internal/machines/join_test.go#L488) [Join writes in order](../../../../src/server/internal/machines/join.go#L124) [Frozen transaction rule](../../../../src/contracts/proto/lgtm/link/v1/link.proto#L89)
- **Failure scenario.** A future implementation commits the machine separately before claiming the token. The frozen early-insert failure still leaves no machine and passes even though a later failure would leave a waiting machine behind.
- **Fix.** Add the late token-update failure case to the regression suite. Compare the token, registry and feed before and after the failed call, then check that the retry creates one machine.

## Grade — 7 out of 10

## Refusal order, replay and mutation results

By reading, Join checks the token identifier and credential lengths before redemption. Redemption locks the token row, compares its 32-byte Secure Hash Algorithm (SHA-256) verifier with `crypto/subtle.ConstantTimeCompare`, and then samples the database clock. The code orders unknown, expired, revoked, used by a different key, and removed before returning a replay or creating the waiting machine. The constant-time call covers the two digests. The database lookup and lock remain separate operations. Both successful writes and their feed entries run in one transaction. The replay adds no changes. Current tests cover simultaneous different keys, simultaneous same-key replay, and an expiry passed while waiting on the row lock. [Input checks](../../../../src/server/internal/link/rules.go#L11) [Redemption](../../../../src/server/internal/machines/join.go#L71) [Concurrency tests](../../../../src/server/internal/machines/join_test.go#L411)

I ran the plan's three mutations on scratch copies of the reviewed revision under the PostgreSQL wrapper.

| Mutation | Observed result |
| --- | --- |
| Check expiry after replay. | `TestRedeemRefusals` exited 1. Its replay-after-expiry case reported that redemption succeeded when it expected `token_expired`. |
| Compare the token digests with `bytes.Equal`. | `TestSecretCompareIsConstantTime` exited 1. It reported the non-constant-time call and the missing constant-time verifier comparison. |
| Put the refusal reason in the initial Hypertext Transfer Protocol (HTTP) header. | `TestJoinRefusalMetadata` exited 0 in 11.809 seconds. The raw Google Remote Procedure Call (gRPC) test `TestJoinRefusalIsATrailerOverGRPC` exited 1 and observed the reason in the initial header and an empty trailer. The newer raw test now guards the required placement. |

## Acceptance output at the reviewed revision

Every test command ran through `deploy/test/with-postgres.sh --`. Both item 4 package acceptance commands and the workstream race verification exited 0. All seven test hashes matched the provided manifest. [Test manifest](../../../../.foundry/go-rewrite/frozen-s3machines-b.sha256)

| Command after the wrapper | Output |
| --- | --- |
| `go test -count=1 -run 'TestRedeem\|TestSecretCompare' ./src/server/internal/machines/` | `ok` for `src/server/internal/machines` in 22.542 seconds. |
| `go test -count=1 -run 'TestJoin\|TestOpenUnimplemented' ./src/server/internal/link/` | `ok` for `src/server/internal/link` in 18.026 seconds. |
| `go test -race -count=1 ./src/server/internal/machines/... ./src/server/internal/link/...` | `ok` for machines in 62.881 seconds and `ok` for link in 30.209 seconds. |
