---
topic: go-s3-client-link
status: attack
tier: L
created: 2026-11-25
updated: 2026-11-26
epic: go-rewrite
---

# Joining a server and keeping a machine connected

## Overview

Sprint 3 gives a machine the two commands it needs to join LGTM Studio and stay connected. The owner creates a token on the Machines page and runs the displayed join command on that machine. The pending machine appears in the open page. Once the owner approves it, the page shows its facts, load and installed harnesses.

The client dials the server and checks who answered. It reconnects on its own when the server restarts. During an outage, it saves reports locally. After reconnecting, it starts from the last report the server confirms it applied.

The client records which commands it has started or finished, including a scan or restart requested from the site. It answers retries from retained receipts without repeating completed work.

## Key decisions

The draft checks the certificate's public key and the server's separate signing key. The owner must settle where encryption ends and whether a machine presents a certificate before freezing the link or transport tests (critic-r1 6, logic2-r1 08, threat-r1 1). A certificate renewal can keep the same key. Replacing either pin requires fresh enrollment.

`lgtm-client join <address> <token>` uses the token's origin. The owner must settle whether to accept tokens on standard input before item 9 freezes tests. The displayed form exposes the token in shell history and process arguments (threat-r1 4).

An approved machine saves reports until the server acknowledges them. A fast machine clock can prune a receipt early, so a retry can lose its outcome or repeat work (UNREVIEWED MECHANISM, logic2-r1 01, critic-r1 3, logic-r1 4, threat-r1 5).

Full log → [decisions.md](decisions.md).

## Workstreams

### A machine joins the server it was given

The owner copies a command from the Machines page and sees a new entry waiting for approval. The client keeps the same key when a join reply is lost. Checks of the token's origin and the server's keys protect the enrollment from a wrong endpoint.

### A machine proves itself and sends its reports in order

The machine reports its facts, load and harness inventory after approval. It dials again by itself after the server restarts. It keeps reports made during an outage, and the server receives them in the order in which the machine saved them.

### Command receipts survive disconnections

An approved machine can scan its harnesses on request. It can take a new name or standing from the server. A restart request leaves a durable receipt before the program stops, and removal stops the machine from trying again.

## Rollout order

1. Settle trust, match the Sprint 0 contract and freeze only after this plan and Server Machines clear review (UNREVIEWED MECHANISM, logic2-r1 02). Confirm the Sprint 1 receipt interface for item 6.
2. Build items 1 to 6 in dependency order, then reconnect supervision (7a), sessions (7b), the application (8), and entry points (9) (critic-r1 9).
3. Require Stack Client to pass the absolute fixture directory as `LGTM_CLIENT_STATE_DIR` in `client_join` and `start_client`. Check C10 edits `identity.json` directly in that directory. The installer and shim use the same filename (critic-r1 1, logic2-r1 05, architect-r1 08).
4. Run combined tests on disposable PostgreSQL and require 13 passes from `deploy/test/checks.sh` on the Stack runner. Test the pasted join command on the Machines page.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**Shared build context**

The root Go module is `github.com/qnlbnsl/lgtm-studio`, with Go 1.26 or newer. The wire types are generated from Protocol Buffers in `src/contracts/gen/go/lgtm/link/v1` as `linkv1`, with the Connect bindings in `linkv1connect`. The service methods are `LinkService.Join` and `LinkService.Open`. Their paths are `/lgtm.link.v1.LinkService/Join` and `/lgtm.link.v1.LinkService/Open`.

The wire rules come from the Sprint 0 round 4 drafts. A final code describes the request and server state at the time of the decision. D numbers refer to design decisions, R numbers to design rules and C numbers to wave 1 checks, in source order.

The command uses `config.StateDir` with an absolute, nonempty `LGTM_CLIENT_STATE_DIR` override and a default of `$HOME/.local/state/lgtm-client`. The caller holds `state.Open` until after the embedded `link.Store` closes. The transactional database `client.db` holds reports and receipts. Sprint 1 local-state item 6 adds `go.etcd.io/bbolt` v1.4.3 with its first import.

Freeze each test before coding. A stub must fail on its named assertion and each risky mutation must fail before the lead merges the tested commit. Client item tests use no PostgreSQL. The orchestrator owns `src/linktest/`, uses `TestLink*` names and skips explicitly without `LGTM_TEST_POSTGRES`. Start the real server with `servertest.Start(t, servertest.Options{Password: ...})` and a disposable database and OpenBao fixture (architect-r1 04).

**WS-1: A machine joins the server it was given**

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 1a | A token for another origin, a plain connection outside loopback, a redirect and a wrong certificate key leave the token unsent to the refused endpoint. | `TestOriginAndDialPolicy`, `TestPinnedTransport`, `TestJoinRejectsAddressAndRedirect`. |
| 1b | A lost reply followed by a retry leaves one waiting machine and one saved enrollment. A bad signing key leaves the enrollment file absent. | `TestJoinReusesDurableKey`, `TestJoinCommitVerifiedEnrollment`, `TestLinkJoinReplay` in the combined suite. |
| 1c | A pasted command adds a waiting row to an open Machines page. Used and expired tokens are refused. | After rollout step 3, `deploy/test/checks.sh --only C5,C6` prints two `ok` lines and exits 0. Paste on a disposable machine and observe the new row without reload (critic-r1 7). |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 1 | Require the permitted origin and pin the encrypted endpoint. | `src/client/internal/transport/transport.go`, `ParseOrigin`, `NewHTTPClient`. |
| 2 | Decode and verify enrollment, then expose its commit. | `src/client/internal/enroll/join.go`, `Join`, `SaveServer`, `LoadServer`. |

**Item 1. Origin and transport**

- **Files.** Create `src/client/internal/transport/transport.go` and `src/client/internal/transport/transport_test.go`.
- **Needs and parallel work.** Needs the owner decision on issue 1633 and Sprint 0 origin and pin rules. Can run beside item 5.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. It uses the Go Hypertext Transfer Protocol (HTTP) client with one origin validator and a certificate check.
- **Statements.** D1, D7, D17, R9 and C10. The design section is "Encryption on the link".
- **Public names.** `func ParseOrigin(raw string) (string, error)` and `func NewHTTPClient(origin string, certificateKeySHA256 []byte) (*http.Client, error)`. It exposes `ErrCertificateKeyMismatch` for a failed pin.
- `ParseOrigin` accepts an origin with an explicit scheme and a host, plus an optional valid numeric port. It lowercases the scheme and host and removes the scheme's default port. It rejects credentials, paths, queries, fragments, whitespace and scoped addresses. It accepts plain HTTP only for a literal loopback address or `localhost`. The latter dials only explicit numeric loopback candidates. The transport uses no environment proxy and follows no redirects. Loopback checks use the parsed Internet Protocol (IP) address, including IPv6, with the actual dial restricted to loopback.
- Plain loopback uses HTTP/2 with cleartext prior knowledge. HTTPS negotiates HTTP/2 and verifies the leaf before sending an HTTP request. Its trust anchor is the expected 32-byte SHA-256 (Secure Hash Algorithm, 256 bits) digest of the leaf's DER (Distinguished Encoding Rules) Subject Public Key Info bytes. The transport also verifies the certificate's validity dates and the origin host against the certificate. It needs no public certificate authority. A matching key in a different certificate chain position does not satisfy the check. Plain loopback requires an empty certificate pin, and HTTPS requires the pin. The caller closes idle connections when it finishes with the client.
- **Risky.** Yes, for token routing and certificate trust. Remove the leaf pin comparison or permit plain non-loopback HTTP.
- **Tests written first in `transport_test.go`.** `TestOriginAndDialPolicy` asserts that explicit default ports compare equal to omitted ports, IPv4 and IPv6 loopback are accepted, deceptive hostnames and non-loopback HTTP are rejected, and a configured HTTP proxy cannot receive a loopback request. A local listener observes HTTP/2. `TestPinnedTransport` uses a loopback HTTPS origin, a self-signed leaf for that host, and a second key. The correct leaf reaches the handler over HTTP/2. The wrong pin, expired leaf, wrong hostname and missing pin yield zero handler calls. A renewed valid leaf with the original key reaches the handler. A handler returning a redirect cannot make the second handler run.
- **Acceptance.** Both tests pass with `go test ./src/client/internal/transport`.

**Item 2. Enrollment**

- **Files.** Create `src/client/internal/enroll/join.go` and `src/client/internal/enroll/join_test.go`.
- **Needs and parallel work.** Needs item 1, the trust ruling, `go-s1-client-local` items 2 and 3 and the Sprint 0 enrollment draft. It can run beside item 5. Item 3 consumes the saved enrollment.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. One unary call uses the local plan's key and atomic write primitives.
- **Statements.** D7, D17, R8, R9 and C5, C6 and C10. The design section is "Joining: a token that authorizes an exchange of keys".
- **Public names.** `type Server struct { Address string; MachineID string; ClientPublicKey ed25519.PublicKey; PublicKey ed25519.PublicKey; CertificateKeySHA256 []byte }`, `func LoadServer(f *state.Folder) (Server, error)` and `func Join(ctx context.Context, f *state.Folder, address, token, hostName string, onRetry func()) (Server, error)`. The enrollment writer calls the app callback once on the first retry. `JoinError{Code string}` exposes fixed diagnostics. `ErrEnrollmentExists`, `ErrEnrollmentMissing` and `ErrEnrollmentInvalid` distinguish local conditions. `func SaveServer(f *state.Folder, server Server) error` commits a verified enrollment (critic-r1 8).
- Decode at most 4 kibibytes as `lgtm1_` and unpadded base64url with `token_id`, `secret`, `address`, `server_key_sha256` and `certificate_key_sha256`. Verify canonical lowercase universally unique identifiers (UUIDs), exact byte lengths and matching normalized origins before dialing the token origin. Clip each 10-second attempt and waits of 1, 2, 4, 8 and at most 15 seconds to the 15-minute budget. Use `connect.WithReadMaxBytes(64 << 10)` for unary responses and stream messages, matching the server cap (UNREVIEWED MECHANISM, threat-r1 2).
- First refuse an existing `f.Paths().Identity`, including an invalid file. Load the key with `enroll.Load`. Create it only on `ErrKeyMissing`. On `state.ErrPlacedNotSynced` from creation, load the same key and require a successful `state.SyncDir` before sending its public half. A new process with a previously pending key also confirms the directory sync before the first attempt. A key read error stops enrollment.
- Send `JoinRequest` with `token_id`, `token_secret`, `machine_public_key` and `host_name`. Validate the name as UTF-8 (Unicode Transformation Format, 8-bit), cap at 255 bytes on a character boundary and use empty if unavailable. Retry transient failures with the same token and key. Same-key replay requires a token that still passes the server checks. Stop on `invalid_argument`, `permission_denied`, pin mismatch or a bad reply. Read `Lgtm-Join-Refusal` from `(*connect.Error).Meta()`. Allow `token_unknown`, `token_expired`, `token_revoked`, `token_used` and `machine_removed`, with fallback `token_refused` (architect-r1 07, logic2-r1 09).
- `Join` validates `JoinResponse.machine_id` and compares the 32-byte `server_public_key` digest in constant time. A key mismatch returns `JoinError{Code: "server_key_mismatch"}`. Item 8 commits the verified `Server` through `SaveServer`, using `state.WriteAtomic` for mode `0600` `identity.json`. Its JavaScript Object Notation (JSON) fields are `version` equal to 1, `address`, `machineId`, `clientPublicKey`, `serverPublicKey` and optional `certificateKeySHA256`. Byte strings use padded standard base64. `LoadServer` validates the fields, origin and key lengths and reads one bounded, regular, private file owned by the current user without following links. After `state.ErrPlacedNotSynced`, success requires a validated reread and `state.SyncDir`. A failed sync preserves the file and key.
- **Risky.** Yes, for credentials and crash ordering. Skip the reply-key comparison or generate a fresh key after an unavailable response.
- **Tests written first in `join_test.go`.** `TestJoinCommitVerifiedEnrollment` checks that a verified reply returns the enrollment without publishing it, that `SaveServer` writes it and that a bad identifier or key yields no enrollment. `TestJoinReusesDurableKey` drops a reply, verifies the key is on disk before the first request and checks both requests use it. A committed identity prevents a second join. `TestJoinRejectsAddressAndRedirect` observes zero requests to a mismatched origin or redirect target. `TestJoinRejectsExpiredAndRedactsRefusal` sends the five reasons through Connect error metadata and repeats the used-token case over gRPC (gRPC Remote Procedure Calls). It checks fallback, the fixed total budget, redaction and expiry after a lost reply. `TestJoinRejectsAddressAndRedirect` also checks that a loopback wrong signer or oversized reply yields no identity and a fixed diagnostic. `TestLoadServerRejectsUnsafeFile` covers a symlink, wrong mode, bad version, bad key length and an absent file. `TestJoinSyncFailureKeepsKey` observes no request before key sync.
- **Acceptance.** `go test ./src/client/internal/enroll` passes all six tests and the credential tests from Sprint 1.

**Design artifact.** The approved foundation design, "Joining: a token that authorizes an exchange of keys". The client plan changes no screen hierarchy.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A join token is disclosed. | High | The encrypted transport checks the leaf before sending. Plain loopback authenticates the reply, so a wrong signer prompts token revocation (threat-r1 3). |
| A join succeeds on the server but the reply is lost. | High | Save the private key first. Replay with the same key and check that the server makes only one waiting record. |
| An owner copies a token into a shell that keeps its history. | Medium | Keep the existing positional agreement for wave 1. The handoff records command arguments and shell history as exposure surfaces for the orchestrator's pending contract decision. |

**Verify.** Run the tests for items 1 and 2 and the combined `TestLinkJoinReplay`. Run C5 and C6 through the loopback stack and paste the real Machines page command in a disposable environment. A separate loopback HTTPS listener exercises pinning with a self-signed leaf.

**WS-2: A machine proves itself and sends its reports in order**

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 2a | Both ends prove themselves. A terminal refusal requires a verified challenge. A gap ends the stream and the client retries from the server's committed prefix. | `TestOpenProofFixture`, `TestRetryStopsForVerifiedRemoval`, `TestRetryResumesAfterEventGap`, C10. |
| 2b | Approval sends facts and all four scan entries. Load updates, a stop and a server restart are seen through the machine watch. | `TestReportInventoryAndLoad`, `deploy/test/checks.sh --only C7,C8,C9` prints three `ok` lines and exits 0. |
| 2c | Three reports saved during an outage are applied once and in order, and a later report also arrives. | `TestLinkOutboxReplayAfterServerFailure` and `deploy/test/checks.sh --only C11` prints one `ok` line and exits 0. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 3 | Prove both ends of a fresh stream. | `src/client/internal/link/conn.go`, `openSession`, `proofText`. |
| 4 | Send and acknowledge durable reports in sequence. | `src/client/internal/link/replay.go`, `replay`; Sprint 1 `Store`. |
| 5 | Collect facts, inventory and load, and append reports. | `src/client/internal/link/report.go`, `reporter`, `captureFacts`, `scanHarnesses`. |
| 7a | Bound reconnect attempts. | `src/client/internal/link/retry.go`, `supervise`. |
| 7b | Own the live session and reporting. | `src/client/internal/link/session.go`, `Run`. |

**Item 3. Opening and proving a stream**

- **Files.** Create `src/client/internal/link/conn.go`, `clock_linux.go`, `clock_darwin.go` and `conn_test.go` in that package.
- **Needs and parallel work.** Needs items 1 and 2, Sprint 1 `Credential` and the Sprint 0 `LinkService.Open` draft. It can run beside item 5.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. It handles three proof messages on a generated stream and verifies the golden fixture.
- **Statements.** D7, D16, D17, R9, R10 and C8 to C11. The design sections are "One connection, dialled by the machine" and "Both ends prove themselves on every connection".
- **Names shared inside the package.** `openSession` takes a context, a generated `LinkServiceClient`, `enroll.Server` and `*enroll.Credential`. Its `session` owns a stream, cancellation, a validated `*linkv1.Welcome` and the elapsed-clock reading at proof send. It supplies `send`, `receive` and `close`. `proofText` builds the signed inputs. Errors distinguish a server-key mismatch and a terminal refusal with a fixed code.
- Send `Hello` with `machine_id`, fresh 32-byte random `machine_nonce` and `link_revision` 1. Verify the 32-byte server nonce and 64-byte Ed25519 signature with the saved key. The signed text is `lgtm-link-v1 server` or `lgtm-link-v1 machine`, a zero byte, the identifier length in one byte, the identifier and nonces, machine first. Send `Proof.machine_signature` from `Credential.Sign` after verification. Sample `unix.ClockGettime` immediately before sending. A failed reading leaves no expiry bound. Linux uses `CLOCK_BOOTTIME` and macOS uses `CLOCK_MONOTONIC_RAW`. Item 3 runs `go get golang.org/x/sys@v0.50.0` with the first import in the two clock files. The version matches `golang.org/x/net` v0.63.0. Require clean `go mod tidy -diff` (architect-r1 06, critic-r1 11).
- Require `OpenResponse.welcome` with valid `Welcome.state` and `server_time`. Check `link_revision` and keep `last_event_number` for item 4. Require an empty `connection_token` for `APPROVAL_WAITING`. Treat unspecified or unknown approval as waiting and discard any token in that case. A usable token stays in the session, and wave 1 has no side calls.
- Cap each handshake exchange at 10 seconds within item 7a's budget. Cancel blocked work and reject unexpected frames. A bad signature closes the stream with `errServerKeyMismatch` and sends no proof. For `failed_precondition` after a verified challenge, read `Lgtm-Link-Refusal` from `(*connect.Error).Meta()`. A verified `unknown_machine`, `link_revision_mismatch` or incompatible welcome revision stops the process. Default to retry for other statuses and absent or unfamiliar reasons (architect-r1 03, critic-r1 5).
- **Risky.** Yes, for key proof and message order. Swap the two nonces, accept a bad signature or read the refusal from the stream's leading headers.
- **Tests written first in `conn_test.go`.** `TestOpenProofFixture` checks both proofs against `src/contracts/testdata/link-proof.json`, the expected frames and fresh nonces. `TestOpenRejectsWrongProofAndOrder` checks bad and stale signatures, wrong frame kind, incomplete welcome and unknown approval. An oversized challenge sends no proof (threat-r1 2). `TestOpenExpiryAndRefusal` checks deadlines, retryable early refusal and `unauthenticated`, and terminal revision mismatch. Over Connect and gRPC, it reads `unknown_machine` from an error immediately after a verified challenge with no reason in the leading headers.
- **Acceptance.** The three tests pass in `go test ./src/client/internal/link -run 'TestOpen'`.

**Item 4. Durable event replay**

- **Files.** Create `src/client/internal/link/replay.go` and `src/client/internal/link/replay_test.go`.
- **Needs and parallel work.** Needs the Sprint 1 `Store` and the Sprint 0 event draft. Runs beside items 3 and 5 using the payload format below. Item 7 binds it to a session.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. It reads one existing queue through an ordered cursor and validates the acknowledgements.
- **Statements.** R6, R9, R10, C9 and C11, and the design section "Events come up, and none is lost or applied twice".
- **Names shared inside the package.** `replay` owns the send cursor and the current stream's successful-send ceiling. `encodeReport(*linkv1.Event) ([]byte, error)` and `decodeReport([]byte) (*linkv1.Event, error)` share the payload format with item 5.
- The payload passed to `Store.Append` starts with byte 1. The rest is the current binary `Event` with number unset, `happened_at` set at capture and one `facts` or `harnesses_found` report. The writer never patches a stored record. The replay path gives `Event.number` the value of the row's `Event.Seq` and wraps it in `OpenRequest.event`. A bad payload stops delivery with a local diagnostic and keeps the store file.
- Call `Store.Ack(Welcome.last_event_number)` on each verified welcome before sending events. Initialize the successful-send ceiling to that mark and start `Store.Read` at the next number. A gap by itself leaves the outbox intact. The next welcome decides the committed prefix again. Events keep their original numbers. Issues 1641 and 1642 exclude restored state and live copies.
- One writer uses `Store.Changed()` and bounded reads. After a successful send, it advances the cursor and raises the ceiling. Serialize that update with acknowledgements so a fast response cannot outrun the send bookkeeping. Only the current stream can acknowledge. A repeated lower mark is harmless. A live mark above the ceiling is rejected before calling `Store.Ack`. The store itself rejects a mark at or above `Next()` with `ErrAckAhead`. Either impossible mark stops the run with one diagnostic naming `client.db` and requires a person before restart. On disconnect, discard the cursor and ceiling. Give heartbeat and result frames priority between events.
- **Risky.** Yes, for sequence and data retention. Allow an acknowledgement above the sent ceiling to reach `Store.Ack`, or start at the last sent number after reconnect.
- **Tests written first in `replay_test.go`.** `TestReplayPreservesSequenceAndCaptureTime` reconnects with a committed but unacknowledged event and a sent but uncommitted event. It checks that the former is removed before the next send and that the latter and unsent reports keep their numbers and timestamps. `TestReplayRejectsImpossibleMarks` checks a welcome at `Next()`, a queued but unsent live mark, an old stream acknowledgement and an immediate valid acknowledgement racing send completion. `TestReplayRejectsDamagedPayload` observes no later send with either a bad version or a bad body, and checks that the file is kept.
- **Acceptance.** `go test -race ./src/client/internal/link -run 'TestReplay'` passes.

**Item 5. Reports and heartbeat contents**

- **Files.** Create `src/client/internal/link/report.go` and `src/client/internal/link/report_test.go`.
- **Needs and parallel work.** Needs Sprint 1 `config.Settings`, `sysinfo.ReadFacts`, `sysinfo.ReadLoad`, `harness.Scan` and `Store.Append`. Runs beside items 1 to 4 using the payload signature specified in item 4. A test writer can stub that interface on its branch until item 4 merges. Integrates after item 4.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. It maps existing facts and inventory into two event types and a heartbeat.
- **Statements.** D8, D16, D19, R9, C7 and C11, and the design sections "What travels in wave 1" and "The client in wave 1".
- **Names shared inside the package.** `reporter` takes the Sprint 1 settings, client version, scan options, outbox and a JSON logger. It supplies `captureFacts`, `scanHarnesses`, `heartbeat` and a durable append operation to items 6 and 7. Collection functions accept context for scan cancellation. The producer timestamps and validates the report against the event contract before it offers it to the outbox.
- Send `Facts.os`, `cores`, `memory_bytes`, `client_version`, `workspace` and `agent_limit`. Use the resolved settings, with the user's home portion abbreviated to `~` when possible. Leave the workspace field empty when it exceeds 256 bytes after abbreviation, as Sprint 1 local state specifies. Keep the other strings within their wire limits at Unicode character boundaries. Keep core count positive and preserve bytes for the server's units conversion. The inventory has all four local scan entries in the order defined by `harness.Definitions()`. Send `HarnessesFound.harnesses` with each `Harness.name` and optional `version` when known. Use the existing scan and its trust checks. The report contains no raw process output, executable path or diagnostic detail.
- `Heartbeat.cpu_percent` and `memory_percent` are optional integers between 0 and 100. Prefer absence over a made-up measurement. When the local probe returns an error, send a heartbeat with both fields absent. The supervisor sends an immediate heartbeat after `Welcome` and every 10 seconds after that. While waiting, it sends the heartbeat with both fields absent. A heartbeat never enters the outbox.
- One publisher owns capture order, pending replacement and append. Mark capture order at start. On `ErrOutboxFull`, keep the newest unnumbered automatic snapshot of each kind. A newer commit discards older unnumbered snapshots of that kind, including late scan completions. Numbered events keep their place. A requested scan whose report cannot commit fails. After successful `Append`, log `msg=link_event_buffered` and numeric `sequence` in commit order. Return store failures to the supervisor (logic2-r1 06).
- **Risky.** Yes, for ordering and disclosure. Move the log before the append returns, or use `Found.Path` as a version.
- **Tests written first in `report_test.go`.** `TestReportInventoryAndLoad` supplies facts, settings and four scan entries, including unknown versions and marked paths. It checks the six facts fields, four names, optional versions and no marked strings. It checks present zero, measured nonzero and absent load values without issuing an outbox append. `TestReportLogFollowsDurableAppend` pauses the Sprint 1 transaction commit return using the store test hook and observes no log until the append returns. `TestReportBackpressure` fills a one-entry outbox, confirms no extra sequence or log, frees room, publishes newer inventory, and verifies stale pending inventory and late scan completions get no number. Only the newest refused facts append. `TestReportUncertainAppendIsNotRepeated` injects an ambiguous commit failure, asserts no success log and no further append in that run, and reopens to verify that any committed payload keeps its sequence.
- **Acceptance.** The four tests pass in `go test ./src/client/internal/link -run 'TestReport'`.

**Item 7a. Reconnect cadence**
- **Files.** Create `src/client/internal/link/retry.go` and `src/client/internal/link/retry_test.go`.
- **Needs and parallel work.** Needs item 3's classified errors. Can run beside item 6. Item 7b supplies real sessions.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. The supervisor owns deadlines and teardown.
- **Statements.** D16, R8, C9 and C10. `supervise` runs one attempt callback at a time and receives its welcome and stop or retry outcome.
- Measure the targets of 1, 2, 4, 8 and 15 seconds from each disconnected attempt's start. Cancel unfinished dial or proof at 14 seconds and leave one second for network teardown. Retry after teardown if the target has passed. Stop that deadline on welcome and reset backoff after 30 seconds of a valid stream. A broken established stream starts a new attempt promptly. Cancellation interrupts attempts and waits (logic-r1 3, logic2-r1 04, critic-r1 4).
- **Risky.** Add the whole wait after failure or start a second attempt before teardown.
- **Tests written first in `retry_test.go`.** `TestRetryBackoffAndSilence` checks 1, 2, 4, 8 and 15-second targets, the 30-second reset and at most 15 seconds between starts for a blocked dial and silent handshake. Settled streams get 30 seconds of silence before cancellation. Observe no overlap. `TestRetryCodeTable` exercises every contract status, `out_of_range` and missing or unknown refusal metadata. Only item 3's verified stops end supervision.
- **Acceptance.** `go test -race ./src/client/internal/link -run 'TestRetry'` passes.

**Item 7b. Session and reporting**
- **Files.** Create `src/client/internal/link/session.go` and `src/client/internal/link/session_test.go`.
- **Needs and parallel work.** Needs items 3 to 6 and 7a. Item 8 can prepare tests against its entry points.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. Wires the sender, receiver, publisher and command handler.
- **Statements.** D16, R6, R8, R9, R10 and C7 to C11.
- **Public names.** `type RunOptions struct { Server enroll.Server; Credential *enroll.Credential; Folder *state.Folder; Settings config.Settings; ScanOptions harness.ScanOptions; Version string; Logger *slog.Logger }` and `func Run(ctx context.Context, opts RunOptions) (RunOutcome, error)`. `RunStopped=0` and `RunRestart=1` are `RunOutcome` values. The app owns the folder. A nonnil error takes precedence.
- One owner orders frames and work. Join network workers before retry and settle the process-owned scan before closing the store. Ignore unknown frames, close on invalid known frames or after 30 seconds without a valid response. If sending closes first, collect the receive error within the deadline. A welcome in transit can precede removal. On refusal discard its token. A gap ends only its stream and uses ordinary reconnect.
- Emit one JSON diagnostic per unchanged reason until recovery. C10 reads `msg=link_refused` and `code=server_key_mismatch` on a bad challenge and `code=unknown_machine` on verified removal (writer: item 7b. Reader: `deploy/test/checks.sh`). A mismatch retries with no proof or token. Removal stops and directs fresh enrollment. A revision refusal directs an upgrade. A certificate-pin mismatch explains fresh enrollment after a key change. Warn on `aborted` that another process may use the key (architect-r1 02, critic-r1 2, logic2-r1 05).
- Call `OpenStore(opts.Folder, StoreOptions{})` before dialing. `ErrStoreMissing`, `ErrStoreUnsafe`, `ErrStoreDamaged`, `ErrSequenceExhausted` and impossible acknowledgements log at error level with the cause and `client.db`, then exit 0 to stop the service loop until repair. `ErrStoreFailed` exits 1 for service retry. Preserve files (UNREVIEWED MECHANISM, architect-r1 05, critic-r1 5).
- Initialize standing and replay before work. Every approved welcome or approval transition queues fresh facts and inventory. While waiting, retain the queue and send bare heartbeats. A process that saw approval captures facts at outage detection and every 10 seconds while disconnected. A new process waits to learn standing.
- **Risky.** Stop on a gap, honor an unsigned removal or produce reports while waiting.
- **Tests written first in `session_test.go`.** `TestRetryStopsForVerifiedRemoval` checks both C10 fields, no proof or token on mismatch, continued retry and terminal removal before or after welcome. `TestRetryResumesAfterEventGap` checks terminal metadata on Connect and gRPC and replay from the next welcome. `TestRetryReportsOnApprovalAndOutage` checks bare heartbeats, approval reports and three outage facts. Assert the immediate heartbeat and the 10-second cadence. `TestRetryFailureExitsAndKeepsStore` checks one error line, status 0 and file preservation for each local permanent stop, zero dials for a bad or missing store and status 1 on failed commit.
- **Acceptance.** `go test -race ./src/client/internal/link` passes.

**Design artifact.** The approved foundation design, "The link between the server and a machine".

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A machine discloses its proof to an endpoint that holds a different signing key. | High | Check the golden signed texts and the negative assertions before sending any proof. |
| A guessed acknowledgement removes reports that the server never applied. | High | Only the authenticated welcome sets the initial mark. Live acknowledgements are bounded by the successful writes on that stream. |
| A stalled stream prevents a process from reconnecting or observing removal. | High | Bound proof exchanges and post-proof silence. Cancel blocked network work and distinguish terminal removal from transient errors. |
| An outage fills the local store. | Medium | The store refuses before numbering a report. Retain the newest refused snapshots and let acknowledgements free capacity. |

**Verify.** Run link tests and rollout step 4. For the long proxy observation, run Stack Web's `LGTM_STREAM_SOAK=1 bash deploy/test/link-stream.sh` and require seven `ok` lines with none skipped (critic-r1 7).

**WS-3: Commands survive disconnections without running again**

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 3a | A retry and a scan completion may arrive in either order across reconnect. One scan runs and the current attempt receives its result. | `TestCommandLatestAttemptAndReplay`, `TestCommandReconnectWhileScanRuns` and `TestLinkCommandAttemptsAcrossReconnect` in the combined suite. |
| 3b | A new scan or restart skips work at expiry. An interrupted action reports that outcome on replay. A restart saves its result before exit and replays the result after relaunch. | `TestCommandExpiryUsesServerTime`, `TestCommandRestartAndRecoveredReceipt`, `TestLinkRestartReplaysAfterDroppedResult` and the installer check `live_reconnect_restarts_service`. |
| 3c | A fresh run reports how to join and sends no network request. A stopped client releases its lock. The service sees status 75 on a requested restart and status 0 on removal. | `TestAppRunRequiresEnrollment`, `TestAppRunClosesState`, `TestRunCommandExitStatus`, `TestClienttestSharesApp`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 6 | Handle approval state and bounded commands through receipts. | `src/client/internal/link/commands.go`, `commandHandler`, `serverClock`; Sprint 1 `Store.Begin` and `Store.Finish`. |
| 8 | Start and stop the process from shared application entry points. | `src/client/internal/app/client.go`, `Join` and `Run`. |
| 9 | Connect the command line and the in-process test entry. | `src/client/cmd/lgtm-client/main.go`, `runContext`; `src/client/clienttest/clienttest.go`, `Options`, `Join` and `Run`. |

**Item 6. Standing and command receipts**

- **Files.** Create `src/client/internal/link/commands.go` and `src/client/internal/link/commands_test.go`.
- **Needs and parallel work.** Needs items 3 and 5 and the Sprint 0 commands draft. Freeze the tests after Client Local agrees to `Decline`. Can run beside item 7a.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. It handles the fixed actions through standing revisions and the receipt store.
- **Statements.** D16, R6, R9, R10, C7 to C9. The design sections are "Commands go down, and none is lost or repeated" and "What travels in wave 1".
- **Names shared inside the package.** `commandHandler` owns standing, one requested scan and completions. Its `serverClock` estimates the upper bound as `Welcome.server_time` plus time since proof send, including sleep. The bound assumes the server clock does not jump after sampling. Evaluate it at admission and again immediately before work. Only a valid bound strictly before expiry permits work. A missing or backward elapsed reading, or a bound at or after expiry, declines new work with `COMMAND_OUTCOME_EXPIRED`.
- Verify the command identifier, positive attempt and valid nonzero expiry timestamp. Apply `Welcome.state` before commands. Only a higher valid revision changes standing. A same-revision conflict logs a fixed diagnostic and keeps the accepted state. `SetState` is the only command accepted while waiting. It bypasses receipts and expiry. After applying it or finding its revision at or below the current one, always answer `COMMAND_OUTCOME_DONE`. On removal, attempt that send before stopping, with the same bound as restart.
- Call `Store.Begin(Command{ID, ExpiresAt}, now)` with the admission bound or expiry when no bound is available. `DecisionReplay` answers the stored result first, `DecisionRunning` waits and `DecisionInterrupted` answers interrupted. On `DecisionExpired`, call proposed `Store.Decline(c Command, result []byte) (Decision, []byte, error)` (writer: Client Local. Reader: item 6). Apply the existing command and result bounds. One transaction writes a finished expired receipt for an absent identifier and returns `DecisionExpired`, or returns the existing receipt decision and bytes. Reply after commit. A failed last expiry check after a normal reservation records expired with `Finish`. Recovery before that commit reports interruption. Store errors prevent work. Unknown actions finish unsupported. Close the stream if a distinct scan cannot enter the slot (architect-r1 01, logic-r1 1, logic2-r1 03).
- One process owner orders delivery and finish across reconnects. While a command runs, keep the highest attempt and its supplying session in memory. If delivery precedes completion, send the result only to that supplying session if it is still current. If finish comes first, commit the result and replay the outcome under the later delivery's attempt. A result queued for an old session stays off a replacement session. Select the attempt at send admission under the same ordering.
- A scan yields `COMMAND_OUTCOME_DONE` only after the report commits. A full buffer or canceled scan yields `COMMAND_OUTCOME_FAILED`. Call `Finish(id, result)` before replying for every command admitted to run, including on controlled shutdown. Store one byte containing the defined outcome. If `ErrResultTooLarge` leaves the command running, retry with the single-byte failed outcome. `Finish` also removes valid finished receipts whose stored expiry precedes the local wall clock by more than 24 hours. A fast machine clock can shorten retention under the Sprint 1 ruling. Finish an accepted restart before spending up to two seconds on its result, then return `RunRestart` even if delivery failed. Write only `CommandResult.command_id`, `attempt` and `outcome`.
- **Risky.** Yes, for ordering and expiry. Start work before the receipt commits, check expiry before replay, skip recording a decline or answer a conflicting `SetState` with failed.
- **Tests written first in `commands_test.go`.** `TestCommandLatestAttemptAndReplay` checks one scan and the latest attempt. `TestCommandReconnectWhileScanRuns` covers both orders of delivery and finish. `TestCommandExpiryUsesServerTime` checks delays, clock offsets, sleep, failed reads and expiry during commit. Drop the expired reply and replay after reopening with an earlier bound. `TestDeclineSurvivesKillBeforeFinish` kills before and after a decline commit. Expect no receipt before an initial decline commit and expired after it. For expiry during a normal reservation, expect interruption before the final commit and expired after it. Neither case executes work. `TestCommandWaitsForStartCommit` holds and fails the start commit. Observe no scan or restart effect until commit (logic2-r1 07). `TestFinishPrunesByAge` checks store retention. `TestCommandRestartAndRecoveredReceipt` checks commit before restart, replay and interruption. `TestCommandStateAndUnknownAction` checks done for approval more than 24 hours late, older, identical and conflicting revisions and removal before exit. It checks unchanged state on conflict and unsupported for unknown actions. `TestCommandScanRequiresDurableReport` checks full and committed buffers and the three result fields.
- **Acceptance.** `go test -race ./src/client/internal/link -run 'TestCommand|TestDecline'` passes.

**Item 8. Shared application lifecycle**

- **Files.** Create `src/client/internal/app/client.go` and `src/client/internal/app/client_test.go`.
- **Needs and parallel work.** Needs items 2 and 7b, Sprint 1 configuration and state, and the installer exit-code agreement. Tests can be prepared beside item 7b using `link.Run`'s documented signature.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. It sequences the private folder, credential, settings and lifecycle at the two entry points.
- **Statements.** D2, D7, D16, D19, R1, R9, R10, C6, C8 and C10.
- **Public names.** `type Options struct { StateDir string; Version string; Getenv func(string) string; Stderr io.Writer }`, `func Join(ctx context.Context, opts Options, address, token string) error` and `func Run(ctx context.Context, opts Options) int` in package `app`. The supplied state directory must be absolute. Empty uses `config.StateDir`. Nil `Getenv` uses `os.Getenv`. Empty version uses `0.0.0-dev`. Nil `Stderr` discards logs. The app writes only fixed diagnostics and local configuration values deliberately shown at startup. An upstream error is never written raw.
- Lock the folder, read the host name and call `enroll.Join`. On the first retry log `msg=join_retrying` and the fixed sentence The server has not answered. Retrying enrollment. For `ErrEnrollmentExists`, say This folder is already joined. Peer refusal logs `msg=join_refused` with an allowlisted `code` and fixed `message`. On expiry after a pending join, say the server may know the saved key. Match the waiting or approved row by host name and join time, remove it if present and use a new token and fresh folder. A loopback signing-key mismatch tells the owner to revoke the token. After verified success, `link.CreateStore(f, link.StoreOptions{})` must close successfully before `enroll.SaveServer` publishes `identity.json`. Preserve existing files (architect-r1 07, logic2-r1 09, critic-r1 8, threat-r1 3).
- `Run` opens the locked folder, validates `identity.json` with `enroll.LoadServer`, syncs the directory and verifies the loaded key matches the saved public half. Missing or invalid enrollment exits 1 with an instruction to join. Load `config.LoadSettings` with the settings path, home and state directory and prepare `harness.DefaultScanOptions`. Log the resolved state and repository folders once and call `link.Run`. Return 0 on cancellation or permanent stop, 75 for an accepted restart, and 1 for other failures, including failed store commits. Use `msg=link_restart_requested` to tell a foreground user to run the command again. Log synchronously and close the folder after the store and workers finish.
- **Risky.** Yes, for trusting saved keys and lifecycle ordering. Dial before verifying the identity file, or release the folder on exit before the link workers stop.
- **Tests written first in `client_test.go`.** `TestAppJoinCreatesStoreBeforeIdentity` holds up store creation after a valid reply, observes no identity until the store closes, and checks a pending retry reuses a private store and key. An unsafe or failed creation leaves the identity unpublished. `TestAppRunRequiresEnrollment` checks exit 1 and zero dials for missing or invalid identity and a mismatched key. `TestAppRunClosesState` observes the lock held until the canceled run settles. `TestAppLogRedactsSecrets` checks refusal fields, recovery for either standing, loopback revocation, one progress line, the already-joined diagnostic and redaction. `TestAppPreservesStateOnRemoval` checks exit 0, preservation and lock release. `TestAppRefusesUnsyncedIdentity` observes zero dials on a directory-sync failure.
- **Acceptance.** `go test -race ./src/client/internal/app` passes.

**Item 9. Command line and in-process test entry**

- **Files.** Change `src/client/cmd/lgtm-client/main.go` and `src/client/cmd/lgtm-client/main_test.go`. Create `src/client/clienttest/clienttest.go` and `src/client/clienttest/clienttest_test.go`.
- **Needs and parallel work.** Needs item 8, the token input decision and the Sprint 1 command stub. It is the last build item. The orchestrator can write combined scenarios against the signatures below while the earlier items build.
- **Area, complexity and scope.** `backend`, low, `client/backend`. It delegates to existing application operations and maps command exits.
- **Statements.** D2, D16, R10 and C5 to C11. The design section is "The client in wave 1".
- Preserve `var version` and `run(args []string, stdout, stderr io.Writer) int` for Sprint 1 and the installer. Add `runContext(ctx context.Context, args []string, stdout, stderr io.Writer) int`. In `main`, use a context canceled by interrupt or termination and release its signal registration before calling `os.Exit`. `version` still prints the injected version and exits 0. `join <address> <token>` delegates to `app.Join`, returns 0 on success and 1 on error. `run` has no positional arguments and uses the app's exit status. Missing, unknown or extra arguments print a fixed usage line and exit 2. Usage never echoes arguments.
- **Exactly three exports in `src/client/clienttest`.** `type Options struct { StateDir string; Version string; Getenv func(string) string; Stderr io.Writer }`, `func Join(ctx context.Context, opts Options, address, token string) error` and `func Run(ctx context.Context, opts Options) int`. The package imports only its own program's application entry and standard-library types. It requires a nonempty absolute disposable `StateDir`. `Join` returns an error on invalid state-directory input. `Run` returns 1 on invalid input. `Getenv` may be supplied to isolate the home and executable search path without changing process-wide environment. `Run` blocks until it closes the folder and returns the same status as the command. An external test starts the server first, joins through the actual endpoint, starts `Run` on a goroutine and cancels and joins it before cleanup. It can observe state through the real server and fixed logs. The package exports no credential, mutable store, fake transport or private process hook.
- **Risky.** Yes, because the command receives a secret and maps restart exits. Echo the supplied arguments on usage error or replace status 75 with 0.
- **Tests written first.** `TestRunCommandExitStatus` in `main_test.go` checks version, bad arity without echoing the secret, failed join with a refusal or exhausted retries, and run status 0, 1 or 75 for cancellation or permanent stop, a failed commit and accepted restart, respectively. `TestClienttestSharesApp` in external package `clienttest_test` checks the same enrollment and diagnostics as the binary, an explicit absolute state path and lock release on cancellation. A stub listener exercises the entry point. The combined suite uses the real server and database.
- **Acceptance.** `go test -race ./src/client/cmd/lgtm-client ./src/client/clienttest` passes. A binary built with `go build -ldflags '-X main.version=0.0.0-test' ./src/client/cmd/lgtm-client` prints `0.0.0-test` for `version`.

**Design artifact.** The approved foundation design, "The client in wave 1".

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A command repeats when a receipt or result is lost. | High | Commit the start before work and its outcome before the reply. Exercise interruption and retries using the same identifier. |
| A late command runs because clocks differ. | High | Use the authenticated estimate of server time for expiry. Exercise a machine clock ahead and behind in the same test. |
| Removal causes a service to restart the client in a loop. | Medium | Use status 0 for a verified removal and require the installer restart policy to respect it. |
| Tests exercise a different process path than the binary. | Medium | Both call the application entry. The test package requires explicit disposable configuration. |

**Verify.** Run the tests of items 6, 8 and 9 and `go test -race ./src/client/...`. The environment checks are the real server and database in the combined suite, plus `live_reconnect_restarts_service` through the installer on Linux. Run the client unit tests and a direct program start and stop on macOS before accepting the macOS installer.

**Combined acceptance and post-build checks** (critic-r1 7, architect-r1 04)

| Test or command | Pass observation |
|---|---|
| `TestLinkJoinReplay` | Drop the committed reply and retry with the same key. The open watch shows one waiting row and the folder has one identity. |
| `TestLinkPresenceAcrossRestart` and `TestLinkKeysAndRemoval` | Stop a client and see offline within 30 seconds. Restart the server and obtain a new current stream for the same identity. Verify both C10 refusal records and zero proof from a wrong-key client attempt. |
| `TestLinkOutboxReplayAfterServerFailure` | Stop the server with the approved client up. Observe three buffered facts and a later report after recovery. The real database applies each sequence once in order. |
| `TestLinkCommandAttemptsAcrossReconnect` and `TestLinkRestartReplaysAfterDroppedResult` | Hold a scan across reconnect and accept its result on the latest attempt. Drop a committed restart result, relaunch and replay done with one restart. |
| `go test -race -count=1 -v ./src/linktest/...` | Run the scenarios with disposable `LGTM_TEST_POSTGRES`. Without it expect explicit skips. |
| macOS sleep and blocked sends | Sleep across command expiry and observe expired without work. Cancel a blocked event send and observe both workers exit, a racing acknowledgement stay valid and a new stream open. |

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

Sources are the client-link assignments, the epic, the approved foundation design, issues 1632 through 1642, phase 4 of the build loop, the current prerequisite plans, their fold requests, and the wave 1 user-flow script.

| Source item | Workstream | Status |
|---|---|---|
| Bounded task and brief, enrollment, proof, retry, reporting and commands | WS-1 to WS-3 | Items 1 to 9 and the combined acceptance table. |
| Task, issue 1634 and epic, in-process tests | WS-3 | Item 9 supplies the client entry. The Server team and orchestrator own the server entry and combined suite. |
| D1 and D2, the product boundary and Go rewrite | WS-1, WS-2, WS-3 | All network work uses generated types. Items 8 and 9 wire the Go program. |
| D3 to D5 and D9 to D11 | Outside client scope | The Server and Stack plans own store, feed, vault and sign-in. |
| D7 and D17, the key exchange and trust | WS-1, WS-2 | Items 1 to 3 check the certificate, public key and proof. |
| D8, the agent protocol | WS-2 | Item 5 reads harness versions. Driving an agent is later work. |
| D16, outbound connection, retry and restart | WS-2, WS-3 | Items 3, 6, 7 and 9. |
| D6, D12 to D15, D18 and D20 to D23 | Later wave | The foundation design defers these. The token stays session-local. |
| D19, the machine user's client state and repositories | WS-2, WS-3 | Items 5 and 8 read the resolved user settings. |
| R1 and R2, ownership and frozen contract | WS-1, WS-2, WS-3 | Rollout 1 and the guardrails. |
| R3 to R5 and R7, database ownership, writes and watch | Outside client scope | The client calls neither these packages nor their tables. |
| R6, durable commands | WS-2, WS-3 | Items 6 and 7 consume only commands received on the link. |
| R8, stable codes | WS-1, WS-2 | Items 2 and 3 interpret the agreed Connect codes and map fixed diagnostics. |
| R9, secrets and agent text | WS-1, WS-2, WS-3 | Transport, sanitized reports, outcome-only results and redaction tests. |
| R10, real server and PostgreSQL for integration | WS-1, WS-2, WS-3 | Combined tests run both real programs in one process. |
| R11, third-party credit | WS-1, WS-2, WS-3 | No copied implementation is planned. The repository credit rule still applies. |
| C1 to C4 and C13, stack, sign-in and vault | Outside client scope | The combined sprint still runs all 13 wave 1 checks. |
| C5 to C11, client acceptance | WS-1 to WS-3 | Rollout steps 3 and 4 and the combined acceptance table. |
| C12, all packages and feed-order probe | WS-3 | Run repository Go tests as part of phase 5. The feed mutation belongs to the Server team. |
| Issues 1632 and 1635, foundation and sizing | WS-1 to WS-3 | Superseded decisions point to current rows. See item tests and execution sizing (logic-r1 7, critic-r1 10, threat-r1 6). |
| Issue 1633, encryption boundary | WS-1, WS-2 | Rollout step 1 and owner decision 41. |
| Issues 1637 and 1638, machine address and host role | WS-1 | Decision 23 and the Server and Stack team requests. |
| Issue 1639, key replacement | WS-1, WS-2 | Decision 18 and the mismatched-key acceptance observations. |
| Issue 1640, runner access | WS-3 | Phase 5 uses a development runner with Docker and local network access. |
| Sprint 0 public messages, proof and results | WS-1, WS-2, WS-3 | Items 1 to 7 use the drafted messages. Rollout step 1 names the shared freeze gate. |
| Issues 1641 and 1642 | Outside wave 1 | Item 4 uses the wire sequence and its local bounds. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repository or assignment rule | How this plan satisfies it |
|---|---|
| `AGENTS.md`, Client scope and the epic | Client owns `src/client/` and the item 3 direct module change. The orchestrator owns `src/linktest/` and checks the module pin. Stack owns `deploy/`. Requests go to other owners through the result (critic-r1 11). |
| `AGENTS.md`, habit 1, credit copied code | The plan uses existing repository primitives and Go interfaces, including the pinned dependency for elapsed time. Any implementation copied from another project needs its third-party directory, license and notice entry through the owning team. |
| `AGENTS.md`, habits 3 and 4, branch and upstream rules | Plan authorship stops before repository integration. The orchestrator owns commits and the base-branch pull request. |
| `AGENTS.md`, "Writing style for documents" | Use dated decisions and repository-relative anchors. Run `scripts/check-private.mjs` and read the new files against the writing rules. |
| The bounded assignment and `common-planner.md` | The reconciliation writes only this plan folder and its result. The orchestrator runs the adversarial review and supplies the launch allocation. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on |
|---|---|---|---|---|---|---|
| 1. Origin and transport | backend | client/backend | medium | One parser and one transport policy, with independent listener tests. | Yes | Sprint 0. |
| 2. Enrollment | backend | client/backend | medium | One generated unary call and one existing durable write boundary. | Yes | 1, Sprint 0, Sprint 1 items 2 and 3. |
| 3. Stream proof | backend | client/backend | medium | The proof fixture covers one handshake, independent of retry and reports. | Yes | 1, 2 and the Sprint 0 link contract. |
| 4. Event replay | backend | client/backend | medium | One send cursor over the existing transactional store and the server's committed mark. | Yes | Sprint 1 item 6 and the Sprint 0 mark rules. |
| 5. Reports | backend | client/backend | medium | Existing local probes produce two durable report kinds and one heartbeat. | Yes | Sprint 1 probes and store. Uses 4's agreed encoding. |
| 6. Commands | backend | client/backend | medium | A receipt-driven handler with three fixed actions. The supervisor is separate. | Yes | 3 and 5, Sprint 1 receipt methods and Sprint 0 commands. |
| 7a. Retry | backend | client/backend | medium | Owns reconnect scheduling and teardown. | Yes | 3. |
| 7b. Session | backend | client/backend | medium | Composes independently tested link pieces. | Yes | 3 to 6 and 7a. |
| 8. Application lifecycle | backend | client/backend | medium | Opens the local resources and orders their teardown through one call. | Yes | 2, 7b, Sprint 1 state and config. |
| 9. Command and test entry | backend | client/backend | low | Thin entry points reuse the application lifecycle. | Yes | 8 and the Sprint 1 command stub. |

Load for build and code checks. Ten units need ten test writers, ten builders and twenty reviewers. With alternating primary vendors and one third-vendor review per unit, the allocation is 15, 15 and 10. Check remaining allowances at dispatch.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (B/M/m) | Outcome |
|---|---|---|
| 1 | 2 / 21 / 19 | Folded in `attack/round-1-findings.md`. The next round reviews the named patches. |

</details>
