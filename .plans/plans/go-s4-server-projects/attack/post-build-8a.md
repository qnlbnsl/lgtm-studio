# SHIP

The frozen router tests still pass when both unary Projects procedures stop requiring the Connect protocol-version header.

OpenAI supplied the test writer, the builder and this reviewer. I wrote neither the router code nor the frozen tests. This review covers the first two bullets of Projects item 8 at commit `7ed4ffbd` on `sprint-0`. The task identifier is `postbuild-go-s4-server-projects-8a` and the nonce is `52e352`.

## Findings

### post-8a 1 · MINOR · The frozen suite accepts a missing protocol guard

The plan requires the Projects procedures to reuse the existing protocol guard. By reading, the built handler passes the required-version option, and every direct request in the frozen fixture supplies the version header. The option requires that header on unary Connect calls. [Plan](../plan.md#L245) · [Guard configuration](../../../../src/server/internal/api/server.go#L126) · [Projects mount](../../../../src/server/internal/api/server.go#L159) · [Request fixture](../../../../src/server/internal/api/projects_test.go#L389)

Confirmed live, all four frozen groups passed after I removed only the protocol option from the Projects mount. With that change, a signed-in Inspect or Add request with no version header could reach the service. Add an assertion for each unary procedure that omits the header, expects `invalid_argument` and verifies that the handler was never called.

### post-8a 2 · MINOR · The frozen suite accepts a one-hour write deadline

The plan requires the shared write deadline and observation through an actual response writer. By reading, the router uses the shared ten-second timeout. The observer forwards each deadline to the real writer and counts set and clear calls. It does not check when the deadline will expire. [Plan](../plan.md#L263) · [Projects mount](../../../../src/server/internal/api/server.go#L162) · [Transport timeout](../../../../src/server/internal/transport/deadline.go#L11) · [Writer assertions](../../../../src/server/internal/api/projects_test.go#L221)

Confirmed live, all four frozen groups passed when the Projects mount supplied a one-hour timeout. A blocked watch write could then hold the stream much longer than intended. Assert that the observed deadline is within the shared send budget for a request with no earlier deadline.

### post-8a 3 · MINOR · Oversized messages keep the transport reading after the cap

The plan sets a 32-kibibyte read limit before decoding. By reading, the pinned Connect version enforces the cap on messages admitted to the decoder. On oversize it drains a unary body to its end or reads the remaining length of a framed message before returning `resource_exhausted`. It also decompresses and discards the remainder of an oversized gzip message. The relevant dependency lines are `protocol_connect.go:1166`, `envelope.go:346` and `compression.go:101`. [Plan](../plan.md#L245) · [Projects read option](../../../../src/server/internal/api/server.go#L159) · [Connect version](../../../../go.mod#L11)

A signed-in sender that pauses an upload after crossing the limit can delay the size refusal while the transport waits for the rest. The frozen cases send complete bodies and assert the eventual refusal. Keep the decoder cap and use a bounded transport read and decompression path that stops on oversize. Check that the refusal can be returned when the sender has yet to finish the body. [Size assertions](../../../../src/server/internal/api/projects_test.go#L317) · [Request fixture](../../../../src/server/internal/api/projects_test.go#L389)

**Grade 8/10.**

## Post-build scenarios

By reading, the newest register places the unreviewed refresh mechanism in items 5 through 7. There is no separately marked unreviewed fix in the router bullets. The mechanism reaches the combined item 8 startup case through removal and watch reopen. Confirmed live, the available watch tests passed for an explicit refresh, seeded removal and rollback, and the lag boundary at references 2,047 and 2,048. The combined 257-repository startup case and the browser race with an earlier Add reply remain assigned to their integration owners. [Review register](round-1-findings.md#L10) · [Unreviewed watch mechanism](../plan.md#L239) · [Combined startup case](../plan.md#L252) · [Post-build scenarios](../plan.md#L279)

Confirmed live, the router refused an unlisted origin and missing or expired credentials before dispatch on all three procedures. The repository-watch fixture produced “Sign in again to reopen this watch.” as its next response when a change followed expiry or direct session revocation. This ordering leaves the known race in issue 1705 untested here. The router also checked the exact entry refusal on an actual request, as called for in issue 1706. The partially written frame case from issue 1706 and the removal controls from issue 1707 remain known. Issues 1702 and 1704 are also recorded already. [Router guard assertions](../../../../src/server/internal/api/projects_test.go#L161) · [Watch lifetime fixture](../../../../src/server/internal/projects/watch_test.go#L380) · [Known issues](../../../../docs/issues.md#L8130)

## The two router bullets

By reading, the mount uses the existing origin check before dispatch and the existing Connect request gate before reading the message. The three generated procedure constants appear in both the dispatch registration and the preflight table. GetServer derives the mounted flag from that registration. Confirmed live, the frozen cases observed the expected preflight and dispatch replies, exactly one Projects service in public and signed-in discovery, contract revision 1, and the authenticated `unimplemented` reply when Projects was absent. This preserves foundation check 3. [Router wiring](../../../../src/server/internal/api/server.go#L159) · [Preflight lookup](../../../../src/server/internal/api/server.go#L265) · [Discovery lookup](../../../../src/server/internal/api/server.go#L330) · [Discovery response](../../../../src/server/internal/api/session.go#L39) · [Foundation check 3](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L380)

Confirmed live, both unary calls and the first framed watch request returned `invalid_argument` and “The request body is invalid.” for malformed JavaScript Object Notation (JSON). No submitted marker appeared in their response bodies or metadata. Missing credentials took precedence. A valid binary Protocol Buffers watch request carrying the same unknown field reached the stub and returned a snapshot. Plain and gzip messages at 32,768 bytes reached the stub, 32,769-byte messages received `resource_exhausted` before dispatch, and the escaped 4,096-byte path reached the stub unchanged. The actual Hypertext Transfer Protocol (HTTP) response writer observed a deadline set and clear. The fixed error matches foundation rules 8 and 9. [Malformed test](../../../../src/server/internal/api/projects_test.go#L182) · [Size test](../../../../src/server/internal/api/projects_test.go#L302) · [Foundation rules](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L369)

## Frozen assertions that responded to omissions

I archived `7ed4ffbd` into one temporary directory. The frozen router file came from `58d3bd97`, was already present in the reviewed commit, and matched the freeze checksum before and after the experiments. I restored each source change before running the next mutation. Every test command ran with a disposable PostgreSQL database. The two mutations that passed ran all four frozen router groups.

| Omission or mutation | Frozen result |
|---|---|
| Remove Add from the preflight table. | Failed at `projects_test.go:147`. The listed origin received HTTP 403 instead of 204. |
| Remove the Projects session request gate. | Failed at `projects_test.go:171`. A request that should have been refused reached the handler. |
| Skip the origin refusal for Add. | Failed at `projects_test.go:169` and `projects_test.go:171`. The unlisted origin received HTTP 200 and reached the handler. |
| Remove the first-watch redaction adapter. | Failed at `projects_test.go:198`. The reply was missing the single terminal error frame. |
| Remove the shared JSON redaction option. | Failed at `projects_test.go:198`. InspectFolder did not return the exact public error. |
| Remove the Projects read cap. | Failed at `projects_test.go:328`. Plain and gzip oversized Add calls received HTTP 200. |
| Remove the Projects write-deadline wrapper. | Failed at `projects_test.go:225`. The response writer saw neither a set nor a clear. |
| Remove the Projects protocol-version option. | All four groups passed in 3.365 seconds. |
| Replace the Projects write deadline with one hour. | All four groups passed in 3.406 seconds. |
| Remove Projects from discovery. | Failed at `projects_test.go:284`. Public and signed-in discovery advertised Projects zero times. |

## PostgreSQL test output

```sh
bash deploy/test/with-postgres.sh -- go test -v ./src/server/internal/api -run '^TestProjectRouter' -count=1
bash deploy/test/with-postgres.sh -- go test -v ./src/server/internal/projects -run '^TestProjectWatch' -count=1
bash deploy/test/with-postgres.sh -- go test -race -timeout 25m -count=1 ./src/server/internal/api
```

The router gate printed `PASS` for `TestProjectRouterPreflight`, `TestProjectRouterMalformedJSON`, `TestProjectRouterDiscovery` and `TestProjectRouterReadLimit`, with zero skips. The package line ended with `4.154s`. After restoration the same four groups passed with zero skips and the package line ended with `3.396s`.

The watch gate printed `PASS` for `TestProjectWatchAttachment`, `TestProjectWatchExplicitRefresh`, `TestProjectWatchSeededRemoval`, `TestProjectWatchLagAndReopen`, `TestProjectWatchSnapshotAndChanges` and `TestProjectWatchSessionExpiry`, with zero skips. The package line ended with `10.938s`.

The full router package passed under the race detector after restoration. Its `ok` line ended with `79.215s`.
