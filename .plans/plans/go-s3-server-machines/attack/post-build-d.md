# Verdict — REVISE

The server stops the Machines page’s change feed while an approval can still commit during shutdown, and the watch reports a database outage.

The builder was a worker from my own vendor, so the requirement for a reviewer from another vendor was not met. The builder's handoff also records that the frozen test writer and builder shared a vendor. [Builder handoff](../../../../.foundry/go-rewrite/results/s3-machines-build-e.md)

I reviewed items 6 and 7 on `sprint-0` at `7294a077`, with `55241fa0` as the comparison base. The source of the reviewed components stayed the same as the branch advanced during the review. Test runs used committed snapshots so they excluded the later plan's uncommitted files. The task identifier was `postbuild-go-s3-server-machines-d` and the nonce was `d3ecc2`.

## Post-build checks for items 6 and 7

- **P1. Each build commit passed its own gate.** Confirmed live. On the frozen test commit `55241fa0`, item 6 compiled and failed assertions because the record calls answered `unimplemented`. On `ea4d7fb6`, item 6 passed and item 7 compiled and failed assertions. The oversized Join reached the placeholder, and the fixture had no signed-in Machines client. On `aa633e16`, item 7 passed. Both build commits passed `go vet ./src/server/...` with no output. All 19 frozen test hashes matched. [P1](../plan.md#L347) [Test manifest](../../../../.foundry/go-rewrite/frozen-s3machines-d.sha256)
- **P2. Join and Open through the deployed web server.** Confirmed live through Caddy. A Join larger than 64 kibibytes returned `resource_exhausted`. A normal Join without a website cookie returned a machine identifier. Open returned `unimplemented`. By reading, item 7 kept the `app.Start` signature and its callers. The test fixture used the running application's signing key and a separate client without a cookie jar for Link. [P2](../plan.md#L348) [Application wiring](../../../../src/server/internal/app/start.go#L229) [Fixture clients](../../../../src/server/servertest/server.go#L108) [Signature assertion](../../../../src/server/internal/app/machines_test.go#L30)
- **P3. Approval, rename, scan and reconnect on the deployed stack.** Confirmed live. The Machines page issued a token with `Cache-Control: no-store`. After a direct Join through Caddy, the browser showed the machine waiting in the approval dialog without a reload. I approved it, saw it offline under the chosen name and renamed it. The reconnect action displayed “This machine is not connected.” The page disables scan for an offline machine. A same-origin request from the browser returned “This machine is not online.” PostgreSQL reported zero command tables. The frozen test also checks that approval writes the machine and its feed change, and that scan and reconnect leave the tables unchanged. [P3](../plan.md#L349) [Approval test](../../../../src/server/internal/machines/records_test.go#L344) [Scan and reconnect test](../../../../src/server/internal/machines/records_test.go#L441)
- **P5. The combined client-command gate could not pass at the reviewed revision.** Confirmed live. `deploy/test/checks.sh --only C5,C6` printed `not ok 5 - C5 lgtm-client join refused a fresh token.` and `not ok 6 - C6 lgtm-client join refused a fresh token.` The committed `lgtm-client join` command still exits 1 as a stub. The direct Join and browser approval in P2 and P3 covered the server path. [Combined check](../../../../deploy/test/checks.sh#L237) [Client stub](../../../../src/client/cmd/lgtm-client/main.go#L19)

## Findings

### post-d 1 — MAJOR — Shutdown interrupts the Machines watch before the request drain is complete

- **Statement.** The sign-in plan's item 9 orders the Hypertext Transfer Protocol (HTTP) drain before closing the follower. Rule R8 requires the public error to describe the failure.
- **Evidence.** By reading, the follower inherits the application lifetime. Stopping the application cancels that lifetime before the HTTP drain. The follower passes the cancellation cause to its subscribers, and the Machines watch maps the application's stop cause to the generic database outage sentence. The later call to `follower.Close()` joins a reader that has already been told to stop. [Shutdown requirement](../../go-s2-server-signin/plan.md#L502) [Shared lifetime and wiring](../../../../src/server/internal/app/start.go#L218) [Stop signal](../../../../src/server/internal/app/start.go#L435) [Drain and release](../../../../src/server/internal/app/start.go#L284) [Subscription cancellation](../../../../src/server/internal/store/follow.go#L110) [Watch error mapping](../../../../src/server/internal/machines/watch.go#L86)
- **Failure scenario.** Confirmed live with a watch and an approval held in a deferred PostgreSQL commit trigger. The watch ended after 2 milliseconds with `unavailable` and “The server cannot reach its database, try again.” PostgreSQL still showed the instance lock and the pending approval commit. Releasing the test gate let the approval finish successfully within the drain. The watch had already ended and the owner saw a database failure during a normal stop.
- **Fix.** Give watches a dedicated drain signal and a fixed shutdown sentence. Keep the follower running through the request drain, then close it before releasing the instance lock and the Store. Pin the order in an application test with an open watch and a pending record write.

### post-d 2 — MINOR — The frozen tests allow keyed renames to lose replay protection

- **Statement.** Item 6 and the design's safe-retry section require a matching request key to replay the saved reply without reapplying the mutation.
- **Evidence.** Confirmed live. The current implementation passed a scratch probe that replayed a keyed rename after a later rename and after removal. Every scoped frozen test still passed after I cleared the key only in the RenameClient handler. The frozen test checks repeat calls with the same current name, and sends them unkeyed. [Safe retries](../../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L176) [Keyed rename](../../../../src/server/internal/machines/records.go#L100) [Frozen rename test](../../../../src/server/internal/machines/records_test.go#L387)
- **Failure scenario.** A caller retries a lost reply after the owner chooses a newer name. An implementation that silently discards rename keys overwrites the newer name, while the frozen tests pass.
- **Fix.** Freeze a wire test that retries the first name and key after an independent rename. Assert that the retry returns the original empty success, the later name and its revision survive, and a different request with the original key is refused.

### post-d 3 — MINOR — A refused token issuer address looks like a database outage

- **Statement.** Rule R8 requires a public sentence the website can show. The plan names the address and certificate sentinels, and the builder records that their codes and sentences are missing from the plan's table.
- **Evidence.** Confirmed live. I started the app with an alternate valid Internet Protocol version 4 (IPv4) loopback origin outside the token issuer's three accepted loopback hosts. Startup and sign-in succeeded. Token creation returned `unavailable` with “The server cannot reach its database, try again.” By reading, issuer validation runs before accessing the database, and the record handler maps both issuer sentinels to its fallback. [Plan error precedence](../plan.md#L76) [Issuer validation](../../../../src/server/internal/machines/tokens.go#L64) [Token address rules](../../../../src/server/internal/machines/tokens.go#L132) [Origin validation](../../../../src/server/internal/auth/origin.go#L129) [Record error mapping](../../../../src/server/internal/machines/records.go#L172)
- **Failure scenario.** An owner running the service at an accepted alternate origin sees a database outage when the configured address prevents token issuance. Retrying the same request leaves the configuration problem in place.
- **Fix.** Add a `failed_precondition` row for each issuer sentinel to the plan's error table. Map the sentinels to their fixed configuration sentences and pin them with a token-issuance test.

## Grade — 8 out of 10

## Session, replay, revocation and restart evidence

- **Every record call requires a session.** Confirmed live. All seven unsigned browser requests returned `unauthenticated`, and the fixture's database stayed unchanged. The frozen test also checks that submitted markers from a malformed signed-in request stay out of the reply. A separate wire probe observed “Sign in to continue.” and “The request body is invalid.” for each procedure. By reading, the router attaches the session gate to the entire website service before decoding. Link has its own handler outside that gate. [Router registration](../../../../src/server/internal/api/server.go#L129) [Session gate](../../../../src/server/internal/api/session_check.go#L39) [Seven record calls](../../../../src/server/internal/machines/records_test.go#L692)
- **Keyed success replies replay.** Confirmed live. The frozen approval test drops the original reply, renames and removes the machine, then compares the replay to the first response byte for byte. The frozen removal test replays the same key after deletion. A scratch probe confirmed keyed rename and revocation, including a conflicting second token identifier that left the second token untouched. By reading, the four keyed writes use `WriteOnce` with owner, procedure and the length-framed version 1 fingerprint. Scan and reconnect are read-only refusals in part A. [Approval replay](../../../../src/server/internal/machines/records_test.go#L553) [Keyed removal](../../../../src/server/internal/machines/records_test.go#L301) [Fingerprint and writes](../../../../src/server/internal/machines/records.go#L145)
- **Revocation spares the approved machine.** Confirmed live. The frozen test observed revocation wait for the approval's row lock, then checked the approved name and revision after the token was revoked. By reading, the token change and waiting-only deletion share a transaction. The conditional deletion checks the machine's standing after the lock wait. [Revocation handler](../../../../src/server/internal/machines/records.go#L40) [Conditional deletion](../../../../src/server/internal/machines/store.go#L187) [Concurrent revocation test](../../../../src/server/internal/machines/records_test.go#L251)
- **The database and key survive restart.** Confirmed live. The frozen app test preserved the approved offline record and the signing public key. A second scratch shutdown run held the approval through the full five-second drain. At lock release the deferred commit was no longer waiting. The machine remained waiting and no approval receipt was saved. By reading, the link has no worker of its own in part A. [Restart test](../../../../src/server/internal/app/machines_test.go#L100) [Link service](../../../../src/server/internal/link/service.go#L13)
- **The public errors in the item 6 table use the specified text.** By reading, the mapped domain and store errors match their fixed sentences. The frozen tests compare the request-key refusal, missing machine, invalid or taken name, invalid approval or rename state, busy database, and offline scan and reconnect text. The plan also specifies the context error codes, and leaves their text to the handler. The generic database sentence is correct for an unavailable database. Findings post-d 1 and post-d 3 demonstrate two other paths that reach that sentence. [Code and sentence table](../plan.md#L96) [Record mapping](../../../../src/server/internal/machines/records.go#L172) [Wire error assertion](../../../../src/server/internal/machines/records_test.go#L100)

## Mutation results

Each mutation ran against PostgreSQL in a scratch overlay. The frozen source files were unchanged.

| Change | Observed result |
| --- | --- |
| Allow a key on token creation. | `TestCreateClientToken` exited 1. The request succeeded where the test expected `invalid_argument`. |
| Use unconditional deletion during revocation. | `TestRevokeClientToken` exited 1. The machine that committed approval first was gone. |
| Project the approval after commit. | `TestKeyedCallsReplay` exited 1. The first approval reply included the address written during commit, and retry after removal returned 404. |
| Dispatch Link to the generated website handler. | `TestServerServesMachines` exited 1. Both Join cases reached a 404. The overlay changed dispatch because the two generated service types have distinct interfaces. |
| Clear the key on a rename. | All scoped frozen tests passed. The Machines, application and public fixture packages printed `ok` in 23.338, 7.976 and 3.223 seconds. |

## Acceptance output

The test commands ran under `deploy/test/with-postgres.sh --` at the reviewed revision unless the commit is named.

| Command after the wrapper | Output |
| --- | --- |
| `go test -count=1 -run 'TestCreateClientToken\|TestRevokeClientToken\|TestRemoveClient\|TestApproveClient\|TestRenameClient\|TestScanAndReconnectRefused\|TestKeyedCalls' ./src/server/internal/machines/` | `ok` in 20.930 seconds, exit 0. The same command printed `ok` in 11.945 seconds at `ea4d7fb6`. |
| `go test -count=1 -run 'TestServerServesMachines\|TestRestartKeepsMachines' ./src/server/internal/app/` | `ok` in 6.043 seconds, exit 0. The same command printed `ok` in 8.761 seconds at `aa633e16`. |
| `go test -count=1 -v -run TestRecordCallsNeedSessionAndRedactMalformedJSON ./src/server/internal/machines/` | All seven subtests printed `PASS`. The package printed `ok` in 1.949 seconds, exit 0. |
| `go test -count=1 -run TestMachinesFixtureClientsAndLink ./src/server/servertest/` | `ok` in 2.289 seconds, exit 0. |

The nine tests named by sign-in item 9 also passed. The application, store, server command and public fixture packages printed `ok` in 28.391, 9.489, 25.427 and 3.315 seconds. [Sign-in item 9 tests](../../go-s2-server-signin/plan.md#L547)

The first full `go test -race -count=1 ./src/server/...` run exited 1 in the unchanged Store package. The application, Machines and public fixture packages printed `ok`. The full rerun with `GOFLAGS=-p=2 GOMAXPROCS=2` exited 0. All 13 packages with tests printed `ok`. The application took 55.713 seconds, Machines took 99.642 seconds, and the public fixture took 18.602 seconds.
