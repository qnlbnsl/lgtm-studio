---
topic: go-s3-client-link
status: done
tier: L
created: 2026-11-25
updated: 2026-11-26
epic: go-rewrite
---

# Joining a server and keeping a machine connected

## Overview

The owner runs the join command from the Machines page. The machine appears on the open page, then shows its facts, load and harnesses after approval.

The client authenticates the server and reconnects by itself after a server restart. It replays saved reports from the server's last applied number.

Durable scan and restart receipts let the client answer retries from its saved outcomes.

## Key decisions

The draft checks the certificate's public key and the server's separate signing key. The owner must settle where encryption ends and whether a machine presents a certificate before freezing the link or transport tests (critic-r1 6, logic2-r1 08, threat-r1 1). A certificate renewal can keep the same key. Replacing either pin requires fresh enrollment.

`lgtm-client join <address> <token>` and `lgtm-client join <address> -` both use the token's origin. The second form reads one token line from standard input (UNREVIEWED FIX, decision 64).

Wave 1 accepts early receipt pruning by a fast machine clock. A retry after expiry can report expired despite earlier completion. A retry can repeat work only if pruning happens before expiry and the retry meets the start bound (logic2-r1 01, architect-r2 5, logic-r2 4). [Decision log](decisions.md).

## Workstreams

### A machine joins the server it was given

The client protects enrollment by checking the token's origin and server keys. Reusing its saved key after a lost reply preserves one machine record.

### A machine proves itself and sends its reports in order

An approved machine sends facts, load and harness inventory. The server receives outage reports in their saved order.

### Command receipts survive disconnections

The client applies new standing and records a restart before it stops. For removal it follows the `APPROVAL_REMOVED` comment in the frozen [commands contract](../go-s0-contract/draft/commands.proto) (UNREVIEWED FIX, decision 65).

## Rollout order

1. Settle trust, match the Sprint 0 contract and freeze after both link plans clear review (logic2-r1 02). Item 6 owns its receipt extension (architect-r2 1, logic-r2 1).
2. Build item 1b before item 2. Build through item 6 in dependency order, then reconnect supervision (7a), sessions (7b), the application (8), and entry points (9) (UNREVIEWED FIX, critic-r1 9).
3. Require Stack Client to pass the absolute fixture directory as `LGTM_CLIENT_STATE_DIR` in `client_join` and `start_client`. Check C10 edits `identity.json` directly in that directory. The installer and shim use the same filename (critic-r1 1, logic2-r1 05, architect-r1 08).
4. Run combined tests on disposable PostgreSQL and require 13 passes from `deploy/test/checks.sh` on the Stack runner. Test the pasted join command on the Machines page.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**Shared build context**

Use Go 1.26 or newer. `src/contracts/gen/go/lgtm/link/v1` supplies Sprint 0 Protocol Buffers types and `linkv1connect`. D, R and C refer to design decisions, rules and wave 1 checks in source order.

`config.StateDir` accepts an absolute, nonempty `LGTM_CLIENT_STATE_DIR` override and defaults to `$HOME/.local/state/lgtm-client`. Hold the `state.Open` lock until `link.Store` closes. Sprint 1 item 6 adds `go.etcd.io/bbolt` v1.4.3 for reports and receipts in `client.db`.

Freeze tests before building. Require failure on each stub's named assertion and risky mutation before merge. Client item tests need no PostgreSQL. The orchestrator owns `TestLink*` in `src/linktest/` and skips explicitly without `LGTM_TEST_POSTGRES`. Use `servertest.Start(t, servertest.Options{Password: ...})` with disposable PostgreSQL and OpenBao (architect-r1 04).

**WS-1: A machine joins the server it was given**

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 1a | A token for another origin, a plain connection outside loopback, a redirect and a wrong certificate key leave the token unsent to the refused endpoint. | `TestOriginAndDialPolicy`, `TestPinnedTransport`, `TestJoinRejectsAddressAndRedirect`. |
| 1b | A lost reply followed by a retry leaves one waiting machine and one saved enrollment. A bad signing key leaves the enrollment file absent. | `TestJoinReusesDurableKey`, `TestJoinCommitVerifiedEnrollment`, `TestLinkJoinReplay` in the combined suite. |
| 1c | A pasted command adds a waiting row to an open Machines page. Used and expired tokens are refused. | After rollout step 3, `deploy/test/checks.sh --only C5,C6` prints two `ok` lines and exits 0. Paste on a disposable machine and observe the new row without reload (critic-r1 7). |
| 1d | The join command accepts one token line on standard input and still accepts a positional token. Empty or oversized input yields a usage error. Output never includes the token (UNREVIEWED FIX). | `TestRunJoinTokenInput`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 1 | Require the permitted origin and pin the encrypted endpoint. | `src/client/internal/transport/transport.go`, `ParseOrigin`, `NewHTTPClient`. |
| 1b | Stop handing out event numbers at the wire limit (UNREVIEWED FIX). | `src/client/internal/link/outbox.go:28`, `Store.Append`. |
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

**Item 1b. Outbox sequence limit (UNREVIEWED FIX)**

- **Files.** Edit Sprint 1 Client Local's `src/client/internal/link/outbox.go:28` and `outbox_test.go:121` and `:174` in the same package.
- **Needs and parallel work.** Needs the built Sprint 1 store and the frozen `Event.number` comment in [events.proto](../go-s0-contract/draft/events.proto). Merge before item 2.
- **Area, complexity and scope.** `backend`, low, `client/backend`. This changes one append guard and its boundary tests.
- Use `math.MaxInt64` as the last number that `Store.Append` hands out. Return `ErrSequenceExhausted` when the next number exceeds it.
- **Risky.** Reject the last legal append or admit the first number above the limit.
- **Tests written first in `outbox_test.go`.** Update `TestAckRefusesAhead` to exercise the new edge. Update `TestAppendRefusesWhenFull` to cover the last legal append and the first refused append. Assert that the refusal leaves the outbox unchanged. Reopen the store and verify that the last event can still be read and acknowledged and that appends remain exhausted.
- **Acceptance.** `go test -race ./src/client/internal/link -run 'TestAckRefusesAhead|TestAppendRefusesWhenFull'` passes.

**Item 2. Enrollment**

- **Files.** Create `src/client/internal/enroll/join.go` and `src/client/internal/enroll/join_test.go`.
- **Needs and parallel work.** Needs items 1 and 1b, the trust ruling, `go-s1-client-local` items 2 and 3 and the Sprint 0 enrollment draft (UNREVIEWED FIX). It can run beside item 5. Item 3 consumes the saved enrollment.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. One unary call uses the local plan's key and atomic write primitives.
- **Statements.** D7, D17, R8, R9 and C5, C6 and C10. The design section is "Joining: a token that authorizes an exchange of keys".
- **Public names.** `type Server struct { Address string; MachineID string; ClientPublicKey ed25519.PublicKey; PublicKey ed25519.PublicKey; CertificateKeySHA256 []byte }`, `func LoadServer(f *state.Folder) (Server, error)` and `func Join(ctx context.Context, f *state.Folder, address, token, hostName string, onRetry func()) (Server, error)`. The enrollment writer calls the app callback once on the first retry. `JoinError{Code string}` exposes fixed diagnostics. `ErrEnrollmentExists`, `ErrEnrollmentMissing` and `ErrEnrollmentInvalid` distinguish local conditions. `func SaveServer(f *state.Folder, server Server) error` commits a verified enrollment (critic-r1 8).
- Decode at most 4 kibibytes as `lgtm1_` and unpadded base64url with `token_id`, `secret`, `address`, `server_key_sha256` and `certificate_key_sha256`. Check canonical lowercase universally unique identifiers (UUIDs), lengths and matching origins. Dial the token origin. Clip 10-second attempts and waits of 1, 2, 4, 8 and at most 15 seconds to 15 minutes. Items 2 and 3 use `connect.WithReadMaxBytes(64 << 10)`. The planned 161-byte wrapped welcome fits with ample headroom. Treat that size as a test fixture (UNREVIEWED MECHANISM, architect-r3 4, architect-r2 6, threat-r1 2).
- First refuse an existing `f.Paths().Identity`, including an invalid file. Load the key with `enroll.Load`. Create it only on `ErrKeyMissing`. On `state.ErrPlacedNotSynced` from creation, load the same key and require a successful `state.SyncDir` before sending its public half. A new process with a previously pending key also confirms the directory sync before the first attempt. A key read error stops enrollment.
- Send `JoinRequest` with `token_id`, `token_secret`, `machine_public_key` and `host_name`. Prepare the host name before sending. Omit any zero bytes and validate it as Unicode Transformation Format, 8-bit (UTF-8). Truncate on a character boundary to the limit in the `JoinRequest.host_name` comment. Use an empty name if it is unavailable. See also the package value rules in [link.proto](../go-s0-contract/draft/link.proto) (UNREVIEWED FIX).
- Retry transient failures with the same token and key. Classify both `Join` stops using the `LinkService` code table in [link.proto](../go-s0-contract/draft/link.proto). Stop on pin mismatch or a bad reply. Read `Lgtm-Join-Refusal` from `(*connect.Error).Meta()` for the allowlisted diagnostic. Use the fixed diagnostic `token_refused` for an absent or unrecognized permission denial reason (UNREVIEWED FIX, architect-r1 07, logic2-r1 09).
- `Join` validates `JoinResponse.machine_id` and compares the 32-byte `server_public_key` digest in constant time. A key mismatch returns `JoinError{Code: "server_key_mismatch"}`. Item 8 commits the verified `Server` through `SaveServer`, using `state.WriteAtomic` for mode `0600` `identity.json`. Its JavaScript Object Notation (JSON) fields are `version` equal to 1, `address`, `machineId`, `clientPublicKey`, `serverPublicKey` and optional `certificateKeySHA256`. Byte strings use padded standard base64. `LoadServer` validates the fields, origin and key lengths and reads one bounded, regular, private file owned by the current user without following links. After `state.ErrPlacedNotSynced`, success requires a validated reread and `state.SyncDir`. A failed sync preserves the file and key.
- **Risky.** Yes, for credentials and crash ordering. Skip the reply-key comparison or generate a fresh key after an unavailable response.
- **Tests written first in `join_test.go`.** `TestJoinCommitVerifiedEnrollment` checks that a verified reply returns the enrollment without publishing it, that `SaveServer` writes it and that a bad identifier or key yields no enrollment. `TestJoinReusesDurableKey` drops a reply, verifies the key is on disk before the first request and checks both requests use it. A committed identity prevents a second join. `TestJoinRejectsAddressAndRedirect` observes zero requests to a mismatched origin or redirect target. `TestJoinRejectsExpiredAndRedactsRefusal` sends the five reasons through Connect error metadata and repeats the used-token case over gRPC (gRPC Remote Procedure Calls). It checks fallback, the fixed total budget, redaction and expiry after a lost reply. `TestJoinRejectsAddressAndRedirect` also checks that a loopback wrong signer or oversized reply yields no identity and a fixed diagnostic. `TestLoadServerRejectsUnsafeFile` covers a symlink, wrong mode, bad version, bad key length and an absent file. `TestJoinSyncFailureKeepsKey` observes no request before key sync.
- **Freeze tests in `join_test.go` (UNREVIEWED FIX).** Extend `TestJoinRejectsExpiredAndRedactsRefusal` to assert that `invalid_argument` and `permission_denied` each stop without metadata and that an unrecognized permission denial reason still stops. Add `TestJoinHostNameFollowsWireRules`. Observe the outgoing name from input containing a zero byte and multibyte text across the 255-byte boundary. Assert that it is valid UTF-8, that it contains no zero byte and that truncation keeps only whole characters within the limit.
- **Acceptance.** `go test ./src/client/internal/enroll` passes the enrollment and Sprint 1 credential tests (UNREVIEWED FIX).

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A join token is disclosed. | High | The encrypted transport checks the leaf before sending. Plain loopback authenticates the reply, so a wrong signer prompts token revocation (threat-r1 3). |
| A join succeeds on the server but the reply is lost. | High | Save the private key first. Replay with the same key and check that the server makes only one waiting record. |
| An owner copies a token into a shell that keeps its history. | Medium | Item 9 also accepts a token on standard input. Test both forms for redaction (UNREVIEWED FIX). |

**Verify.** Run the tests for items 1 and 2 and the combined `TestLinkJoinReplay`. Run C5 and C6 through the loopback stack and paste the real Machines page command in a disposable environment. A separate loopback HTTPS listener exercises pinning with a self-signed leaf.

**WS-2: A machine proves itself and sends its reports in order**

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 2a | Both ends prove themselves. A terminal refusal requires a verified challenge. A gap ends the stream and the client retries from the server's committed prefix. | `TestOpenProofFixture`, `TestRetryStopsForVerifiedRemoval`, `TestRetryResumesAfterEventGap`, C10. |
| 2b | Approval sends facts and all four scan entries. Load updates, a stop and a server restart are seen through the machine watch. | `TestReportInventoryAndLoad`, `deploy/test/checks.sh --only C7,C8,C9` prints three `ok` lines and exits 0. |
| 2c | Three reports saved during an outage are applied once and in order, and a later report also arrives. | `TestLinkOutboxReplayAfterServerFailure` and `deploy/test/checks.sh --only C11` prints one `ok` line and exits 0. |
| 2d | The outbox hands out its final wire number. That event stays readable and acknowledgeable after reopening the store (UNREVIEWED FIX). | `TestAckRefusesAhead` and `TestAppendRefusesWhenFull`. |

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
- Cap each handshake exchange at 10 seconds within item 7a's budget. Cancel blocked work and reject unexpected frames. A bad signature closes the stream with `errServerKeyMismatch` and sends no proof. Classify `Open` errors using the `LinkService` code table and its note on absent or unfamiliar `failed_precondition` reasons in [link.proto](../go-s0-contract/draft/link.proto). Read `Lgtm-Link-Refusal` from `(*connect.Error).Meta()`. An incompatible welcome revision also stops the process (UNREVIEWED FIX, architect-r1 03, critic-r1 5).
- **Risky.** Yes, for key proof and message order. Swap the two nonces, accept a bad signature or read the refusal from the stream's leading headers.
- **Tests written first in `conn_test.go`.** `TestOpenProofFixture` checks both fixture proofs, frame order and fresh nonces. `TestOpenRejectsWrongProofAndOrder` checks bad and stale signatures, wrong kind, incomplete welcome and unknown approval. A 70-kibibyte challenge closes the stream without proof. The maximal planned welcome fits (architect-r2 6, threat-r1 2). `TestOpenExpiryAndRefusal` checks deadlines, retryable early refusal and `unauthenticated`, and terminal revision mismatch. Connect and gRPC deliver `unknown_machine` after a verified challenge with no reason in leading headers.
- **Acceptance.** The three tests pass in `go test ./src/client/internal/link -run 'TestOpen'`.

**Item 4. Durable event replay**

- **Files.** Create `src/client/internal/link/replay.go` and `src/client/internal/link/replay_test.go`.
- **Needs and parallel work.** Needs item 1b, the Sprint 1 store and the Sprint 0 event contract (UNREVIEWED FIX). Runs beside items 3 and 5. Item 7b binds it to a session (architect-r2 7).
- **Area, complexity and scope.** `backend`, medium, `client/backend`. It reads one existing queue through an ordered cursor and validates the acknowledgements.
- **Statements.** R6, R9, R10, C9 and C11, and the design section "Events come up, and none is lost or applied twice".
- **Names shared inside the package.** Item 4 writes `newReplay(store *Store, through uint64) (*replay, error)`, `next() (*linkv1.Event, error)` and `ack(through uint64) error` for item 7b. Item 4 writes `encodeReport(*linkv1.Event) ([]byte, error)` for item 5 and uses `decodeReport` internally (architect-r2 8, logic-r3 2).
- `Store.Append` takes version byte 1 followed by a binary `Event` with number unset, `happened_at` set at capture and one `facts` or `harnesses_found` report. Stored payloads stay immutable. `next` assigns `Event.Seq` to `Event.number` on the outgoing copy. An invalid stored payload returns `ErrStoreDamaged`.
- `newReplay` acknowledges the verified welcome mark, sets the ceiling to that mark and starts at the next number. `next` returns `(nil, nil)` when empty. Hand one event directly to the sole sender and advance the cursor and ceiling at that handoff. A failed send ends the stream. Discard the cursor and replay from the next welcome. Restores and live copies are outside wave 1 (logic-r3 2).
- `ack` refuses a mark above the handoff ceiling with `ErrAckAhead` before calling `Store.Ack`. The store returns that error for a welcome mark at or above `Next()`. A lower repeat mark is harmless (logic-r3 2).
- **Risky.** Let a mark above the ceiling reach `Store.Ack`, advance the ceiling after send returns, or resume from the last handed-out event.
- **Tests written first in `replay_test.go`.** `TestReplayPreservesSequenceAndCaptureTime` checks a committed event is removed on welcome and the uncommitted and unsent events keep their numbers and timestamps. `TestReplayRejectsImpossibleMarks` accepts the handed-out event at once, rejects a mark for the next queued event and rejects welcome at `Next()`. `TestReplayRejectsDamagedPayload` checks a bad version or body stops delivery and preserves the file (logic-r3 2).
- **Acceptance.** `go test -race ./src/client/internal/link -run 'TestReplay'` passes.

**Item 5. Reports and heartbeat contents**

- **Files.** Create `src/client/internal/link/report.go` and `src/client/internal/link/report_test.go`.
- **Needs and parallel work.** Needs Sprint 1 `config.Settings`, `sysinfo.ReadFacts`, `sysinfo.ReadLoad`, `harness.Scan` and `Store.Append`. Runs beside items 1 to 4 using the payload signature specified in item 4. A test writer can stub that interface on its branch until item 4 merges. Integrates after item 4.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. It maps existing facts and inventory into two event types and a heartbeat.
- **Statements.** D8, D16, D19, R9, C7 and C11, and the design sections "What travels in wave 1" and "The client in wave 1".
- **Names shared inside the package.** `reporter` takes settings, version, scan options, outbox and logger. Item 5 provides `captureFacts`, `heartbeat` and `scanHarnesses(ctx context.Context, requested bool) (committed bool, err error)` to items 6 and 7b. It timestamps and validates reports before append (architect-r2 7, logic-r2 2).
- Send `Facts.os`, `cores`, `memory_bytes`, `client_version`, `workspace` and `agent_limit`. Use the resolved settings, with the user's home portion abbreviated to `~` when possible. Leave the workspace field empty when it exceeds 256 bytes after abbreviation, as Sprint 1 local state specifies. Keep the other strings within their wire limits at Unicode character boundaries. Keep core count positive and preserve bytes for the server's units conversion. The inventory has all four local scan entries in the order defined by `harness.Definitions()`. Send `HarnessesFound.harnesses` with each `Harness.name` and optional `version` when known. Use the existing scan and its trust checks. The report contains no raw process output, executable path or diagnostic detail.
- Prepare each report string before append to meet the package string rule in [link.proto](../go-s0-contract/draft/link.proto) (UNREVIEWED FIX).
- `Heartbeat.cpu_percent` and `memory_percent` are optional integers between 0 and 100. Prefer absence over a made-up measurement. When the local probe returns an error, send a heartbeat with both fields absent. The supervisor sends an immediate heartbeat after `Welcome` and every 10 seconds after that. While waiting, it sends the heartbeat with both fields absent. A heartbeat never enters the outbox.
- One publisher orders captures at start. A newer commit displaces older unnumbered reports of its kind. On full, retain only the newest automatic snapshot per kind. Drop refused requested reports. `scanHarnesses` returns `(false, nil)` for full, discard or cancellation before commit and `(true, nil)` after commit. Propagate store errors. Log `msg=link_event_buffered` and numeric `sequence` in commit order (logic2-r1 06, logic-r2 2, logic-r3 3).
- **Risky.** Yes, for ordering and disclosure. Move the log before the append returns, or use `Found.Path` as a version.
- **Tests written first in `report_test.go`.** `TestReportInventoryAndLoad` checks six facts fields, four names, optional versions, path redaction and zero, nonzero and absent load without append. `TestReportLogFollowsDurableAppend` holds the commit return and checks no early log. `TestReportBackpressure` fills a one-entry queue and observes no extra sequence or log. It frees room and commits newer inventory before a late requested scan. The late scan gets no number and fails. Only the newest refused facts append (logic-r2 2). `TestReportUncertainAppendIsNotRepeated` injects ambiguous commit failure, checks no success log or subsequent append, and reopens to verify any committed payload keeps its number.
- Extend `TestReportInventoryAndLoad` with input strings containing zero bytes. Assert that every string in the encoded facts and inventory is free of zero bytes (UNREVIEWED FIX).
- **Acceptance.** The four tests pass in `go test ./src/client/internal/link -run 'TestReport'`.

**Item 7a. Reconnect cadence**
- **Files.** Create `src/client/internal/link/retry.go` and `src/client/internal/link/retry_test.go`.
- **Needs and parallel work.** Needs item 3's classified errors and the freeze fixture `src/contracts/testdata/link-codes.json` (UNREVIEWED FIX). Can run beside item 6. Item 7b supplies real sessions.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. The supervisor owns deadlines and teardown.
- **Statements.** D16, R8, C9 and C10. `supervise` runs one attempt callback at a time and receives its welcome and stop or retry outcome.
- Measure the targets of 1, 2, 4, 8 and 15 seconds from each disconnected attempt's start. Cancel unfinished dial or proof at 14 seconds and leave one second for network teardown. Retry after teardown if the target has passed. Stop that deadline on welcome and reset backoff after 30 seconds of a valid stream. A broken established stream starts a new attempt promptly. Cancellation interrupts attempts and waits (logic-r1 3, logic2-r1 04, critic-r1 4).
- **Risky.** Add the whole wait after failure or start a second attempt before teardown.
- **Tests written first in `retry_test.go`.** `TestRetryBackoffAndSilence` checks 1, 2, 4, 8 and 15-second targets, the 30-second reset and at most 15 seconds between starts for a blocked dial and silent handshake. Settled streams get 30 seconds of silence before cancellation. Observe no overlap. `TestRetryCodeTable` reads `src/contracts/testdata/link-codes.json` and uses its expected actions to exercise every row, including the verified challenge condition. Add cases for `out_of_range` and absent or unknown reasons on `Open failed_precondition` (UNREVIEWED FIX).
- **Acceptance.** `go test -race ./src/client/internal/link -run 'TestRetry'` passes.

**Item 7b. Session and reporting**
- **Files.** Create `src/client/internal/link/session.go` and `src/client/internal/link/session_test.go`.
- **Needs and parallel work.** Needs items 3 to 6 and 7a. Item 8 can prepare tests against the exit mapping below.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. Wires the sender, receiver, publisher and command handler.
- **Statements.** D16, R6, R8, R9, R10 and C7 to C11.
- **Public names.** `type RunOptions struct { Server enroll.Server; Credential *enroll.Credential; Folder *state.Folder; Settings config.Settings; ScanOptions harness.ScanOptions; Version string; Logger *slog.Logger }` and `func Run(ctx context.Context, opts RunOptions) (RunOutcome, error)`. Item 7b writes the pair and item 8 reads it. `RunStopped=1`, `RunRestart=2` and zero is invalid. Stack reads item 8's process status. The app owns the folder (architect-r2 4, architect-r3 3).
- One owner watches `Store.Changed()`, admits `OpenRequest.event` only when its sender is ready and reads acknowledgements only from the current stream. Give ready heartbeats and results priority between events. Join workers before retry and settle the scan before closing the store. Ignore unknown frames. Close on invalid known frames or 30 seconds of silence. If sending closes first, collect the receive error within the deadline. Discard the token on refusal (architect-r2 8, logic-r3 2).
- Log one JSON diagnostic per unchanged reason until recovery. C10 reads item 7b's `msg=link_refused` with `code=server_key_mismatch` or `code=unknown_machine` in `deploy/test/checks.sh`. Retry mismatches without proof or token. Verified removal directs enrollment, revision refusal an upgrade, and a pin mismatch enrollment after a key change. Warn on `aborted` about a second process (architect-r1 02, critic-r1 2, logic2-r1 05).
- Open the store before dialing. Preserve files. For `ErrSequenceExhausted` and `ErrAckAhead`, item 7b logs one error with `msg=link_stopped` and `code=sequence_exhausted` or `code=impossible_ack`, respectively. The `message` is “An owner must decide how to recover this machine's saved events before restarting.” Item 8's test checks the line, and Stack Client checks the journal code. These two faults, verified final refusals and cancellation return `(RunStopped, nil)`. Cancellation adds no diagnostic. Other store faults log once and return `(RunStopped, err)`. Open failure names `client.db`. Accepted restart returns `(RunRestart, nil)` (UNREVIEWED MECHANISM, logic-r3 1, architect-r3 5, UNREVIEWED FIX, logic-r4 1).
- Initialize standing and replay. Each approved welcome or transition queues fresh facts and inventory. Waiting retains the queue and sends bare heartbeats. A process that saw approval captures facts at outage detection and every 10 seconds while disconnected.
- **Risky.** Stop on a removal notice without the refusal in the `LinkService.Open` removal comment. Stop on gaps, report while waiting or omit `link_stopped` (UNREVIEWED FIX, logic-r4 1).
- **Tests written first in `session_test.go`.** `TestRetryStopsForVerifiedRemoval` checks C10 fields, retry without proof or token and removal before or after welcome. `TestRetryResumesAfterEventGap` checks both protocols. `TestRetryReportsOnApprovalAndOutage` checks heartbeat cadence, approval reports and three outage facts. `TestRetryFailureExitsAndKeepsStore` checks retryable open, read and commit errors, preserved files and zero dials on open failure. Exhaustion and impossible welcome or live marks each yield `(RunStopped, nil)` and one exact diagnostic. `TestSessionPrioritizesReadyFrames` queues a result and heartbeat mid-send and sees both before the next event. `TestSessionAcceptsImmediateAck` holds send after writing, accepts the immediate reply, ignores retired replies and replays a failed unacknowledged send (logic-r3 1, logic-r3 2, architect-r3 5, UNREVIEWED FIX, logic-r4 1).
- Extend `TestRetryStopsForVerifiedRemoval` to see a done reply and a waiting heartbeat after the notice alone. Observe a fresh dial after the notice followed by an ordinary break. In the removal case, require `unknown_machine` after a verified `Challenge` before asserting `RunStopped`. Extend `TestRetryResumesAfterEventGap` to observe the first replay at `Welcome.last_event_number + 1` on the fresh stream (UNREVIEWED FIX).
- **Acceptance.** `go test -race ./src/client/internal/link` passes.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A machine discloses its proof to an endpoint that holds a different signing key. | High | Check the golden signed texts and the negative assertions before sending any proof. |
| A guessed acknowledgement removes unapplied reports. | High | The authenticated welcome sets the initial mark. Live acknowledgements are bounded by handoffs to that stream's sender (UNREVIEWED FIX). |
| A stalled stream prevents a process from reconnecting or observing removal. | High | Bound proof exchanges and post-proof silence. Cancel blocked network work and distinguish terminal removal from transient errors. |
| An outage fills the local store. | Medium | The store refuses before numbering a report. Retain the newest refused snapshots and let acknowledgements free capacity. |

**Verify.** Run link tests and rollout step 4. For the long proxy observation, run Stack Web's `LGTM_STREAM_SOAK=1 bash deploy/test/link-stream.sh` and require seven `ok` lines with none skipped (critic-r1 7).

**WS-3: Commands survive disconnections without running again**

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 3a | A retry and a scan completion may arrive in either order across reconnect. One scan runs and the current attempt receives its result. | `TestCommandLatestAttemptAndReplay`, `TestCommandReconnectWhileScanRuns` and `TestLinkCommandAttemptsAcrossReconnect` in the combined suite. |
| 3b | A new scan or restart skips work at expiry. An interrupted action reports that outcome on replay. A restart saves its result before exit and replays the result after relaunch. | `TestCommandExpiryUsesServerTime`, `TestCommandRestartAndRecoveredReceipt`, `TestLinkRestartReplaysAfterDroppedResult` and the installer check `live_reconnect_restarts_service`. |
| 3c | An unenrolled run directs enrollment without dialing. A stopped client releases its lock. The service sees 75 for restart, 0 for permanent stops and 1 for retryable local faults. | `TestAppRunRequiresEnrollment`, `TestAppRunClosesState`, `TestAppRunStoreStatus`, `TestRunCommandExitStatus`. |
| 3d | The client handles a removal notice and a later refusal according to the `APPROVAL_REMOVED` comment in [commands.proto](../go-s0-contract/draft/commands.proto) (UNREVIEWED FIX). | `TestCommandStateAndUnknownAction`, `TestRetryStopsForVerifiedRemoval` and contract post-build check 30. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 6 | Handle standing and durable commands. | `src/client/internal/link/commands.go`, `commandHandler`, `serverClock`. In `receipts.go`, extend `Store.Begin` and `Store.Finish` with `Store.Decline`. |
| 8 | Start and stop the process from shared application entry points. | `src/client/internal/app/client.go`, `Join` and `Run`. |
| 9 | Connect the command line and the in-process test entry. | `src/client/cmd/lgtm-client/main.go`, `runContext`; `src/client/clienttest/clienttest.go`, `Options`, `Join` and `Run`. |

**Item 6. Standing and command receipts**

- **Files.** In `src/client/internal/link/`, create `commands.go` and `commands_test.go`, and edit `receipts.go` and `receipts_test.go` (architect-r2 1, logic-r2 1).
- **Needs and parallel work.** Needs items 3 and 5, the built Sprint 1 store and Sprint 0 commands. Can run beside item 7a.
- **Area, complexity, scope and statements.** `backend`, medium, `client/backend`. One new store transaction uses the existing receipt format. The handler covers D16, R6, R9, R10 and C7 to C9.
- **Names shared inside the package.** `commandHandler` owns standing, the scan slot and completions. `serverClock` bounds expiry with `Welcome.server_time` plus sleep-inclusive time since proof send, assuming no server-clock jump. Check at admission and immediately before work. A failed or backward clock read cannot admit new work.
- Validate identifier, positive attempt and expiry. Use the `MachineState` and `APPROVAL_REMOVED` comments and the opening `SetState` result rule in [commands.proto](../go-s0-contract/draft/commands.proto). Use the waiting rule in the `LinkService.Open` comment in [link.proto](../go-s0-contract/draft/link.proto). Use the normal result send path for the removal notice. Skip the receipt store for `SetState` (UNREVIEWED FIX).
- Reserve a free scan slot before normal `Begin`. Keep it across reconnect and release it on completion or when admission starts no scan. While busy, call `Begin(c, c.ExpiresAt)` for a scan. At equality, a missing receipt yields `DecisionExpired` without writing. Existing receipts replay, wait or answer interrupted on the same stream. For an absent receipt, decline if the real admission bound forbids starting. Otherwise close the stream for retry, with no receipt for that scan. Other actions bypass the scan slot (UNREVIEWED MECHANISM, architect-r2 2, architect-r3 2, logic-r3 4).
- `Store.Begin(Command{ID, ExpiresAt}, now)` uses the admission bound or expiry if the bound is unavailable. Replay before checking expiry. Wait on running and answer interrupted on recovery. On `DecisionExpired`, call `Store.Decline(c Command, result []byte) (Decision, []byte, error)` from `commands.go`. Item 6 writes it in `receipts.go`. Over an absent receipt, commit the expiry and result as finished and return `DecisionExpired` without a running entry. A finished receipt returns `DecisionReplay` and its bytes. A valid start still running returns `DecisionRunning`. A recovered start or invalid receipt returns `DecisionInterrupted`. Existing receipts cause no write (architect-r1 01, logic-r1 1, logic2-r1 03, architect-r2 1, logic-r2 1).
- Share `Begin`'s identifier and expiry checks. Return `ErrResultTooLarge` on an oversized new result. Prune in the same decline write, as in `Finish`, using the machine clock and the 24-hour rule. A failed commit fences the store, sends no reply and returns `ErrStoreFailed` to item 7b for teardown (logic-r2 5). If expiry strikes after a normal start reservation, record expired with `Finish`. Recovery before that commit reports interruption.
- One owner orders deliveries, finishes and send admission. Reply on the latest attempt's supplying session if current. A later delivery replays the saved result. Persist one outcome byte before reply. Unknown actions finish unsupported. An accepted restart cancels the active scan and settles its receipt before committing restart done. Allow up to two seconds to send the restart result, then return `RunRestart` even on send failure. Write only `CommandResult.command_id`, `attempt` and `outcome` (UNREVIEWED MECHANISM, logic-r3 5).
- A requested scan finishes done if its own report commits. Full buffer, cancellation before commit or discard behind newer inventory finishes failed. An ambiguous store commit tears down the run (UNREVIEWED MECHANISM, logic-r2 2).
- A clock two hours fast can prune a finished receipt when the server is 23 hours past expiry. That retry records expired. With a clock 30 hours fast and an hour left, pruning can let the retry run again (architect-r2 5, logic-r2 4).
- **Risky.** Start work before receipt commit, admit a second scan before it has a slot, check expiry before replay, omit the decline write or stamp a discarded scan done.
- **Tests written first in `receipts_test.go`.** `TestDeclineDecisions` asserts all receipt cases, validates input errors and checks no writes on replay or refusal, pruning and replay after reopening. `TestDeclineSurvivesKillBeforeFinish` kills before commit and between commit and reply. Expect no entry before, expired replay after, and `ErrStoreFailed` with later writes fenced on a failed commit. `TestFinishPrunesByAge` exercises both offsets with explicit `Begin` time (architect-r2 1, architect-r2 5, logic-r2 1, logic-r2 4, logic-r2 5).
- **Tests written first in `commands_test.go`.** `TestCommandLatestAttemptAndReplay` and `TestCommandReconnectWhileScanRuns` cover one scan and both delivery orders. `TestCommandBusySlotWritesNoReceipt` keeps the stream open for finished, interrupted and saved expiry receipts and a state update. A runnable distinct scan closes without writing and runs once after reconnect. `TestCommandExpiryUsesServerTime` covers delay, offset, sleep, failed reads and replay without a bound. A new expired scan saves expiry even when busy. Expiry during reservation saves expired or recovers interrupted after a kill. `TestCommandWaitsForStartCommit` checks zero effects or replies on held or failed commit (logic2-r1 07). `TestCommandRestartAndRecoveredReceipt` checks scan failed before restart commits and both outcomes replay. `TestCommandStateAndUnknownAction` checks late, older, equal and conflicting state, removal reply and unsupported actions. `TestCommandScanRequiresDurableReport` checks four outcomes and three fields (architect-r3 2, logic-r3 4, logic-r3 5, UNREVIEWED FIX).
- In `TestCommandStateAndUnknownAction`, give an approved machine a valid higher-revision `SetState` with `APPROVAL_REMOVED`. Assert waiting and `COMMAND_OUTCOME_DONE`. Hold the reply send and verify that the notice schedules no timer and returns no stop outcome (UNREVIEWED FIX).
- **Acceptance.** `go test -race ./src/client/internal/link -run 'TestCommand|TestDecline|TestFinishPrunesByAge'` passes.

**Item 8. Shared application lifecycle**

- **Files.** Create `src/client/internal/app/client.go` and `src/client/internal/app/client_test.go`.
- **Needs and parallel work.** Needs items 2 and 7b, Sprint 1 configuration and state, and the installer exit-code agreement. Tests can be prepared beside item 7b using `link.Run`'s documented signature.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. It sequences the private folder, credential, settings and lifecycle at the two entry points.
- **Statements.** D2, D7, D16, D19, R1, R9, R10, C6, C8 and C10.
- **Public names.** `type Options struct { StateDir string; Version string; Getenv func(string) string; Stderr io.Writer }`, `func Join(ctx context.Context, opts Options, address, token string) error` and `func Run(ctx context.Context, opts Options) int` in package `app`. The supplied state directory must be absolute. Empty uses `config.StateDir`. Nil `Getenv` uses `os.Getenv`. Empty version uses `0.0.0-dev`. Nil `Stderr` discards logs. The app writes only fixed diagnostics and local configuration values deliberately shown at startup. An upstream error is never written raw.
- Lock the folder, read the host name and call `enroll.Join`. On the first retry log `msg=join_retrying` and the fixed sentence The server has not answered. Retrying enrollment. For `ErrEnrollmentExists`, say This folder is already joined. Peer refusal logs `msg=join_refused` with an allowlisted `code` and fixed `message`. On expiry after a pending join, say the server may know the saved key. Match the waiting or approved row by host name and join time, remove it if present and use a new token and fresh folder. A loopback signing-key mismatch tells the owner to revoke the token. After verified success, `link.CreateStore(f, link.StoreOptions{})` must close successfully before `enroll.SaveServer` publishes `identity.json`. Preserve existing files (architect-r1 07, logic2-r1 09, critic-r1 8, threat-r1 3).
- `Run` locks the folder, loads `identity.json` with `enroll.LoadServer`, syncs the directory and matches the key. Bad or missing enrollment directs join and exits 1. Load `config.LoadSettings` and `harness.DefaultScanOptions`. Log resolved folders. Map stopped and restart with nil error to 0 and 75, respectively. Map errors and invalid outcomes to 1. Log invalid outcomes and `msg=link_restart_requested` for foreground restart. Close the folder after the store and workers (architect-r3 3).
- **Risky.** Yes, for trusting saved keys and lifecycle ordering. Dial before verifying the identity file, or release the folder on exit before the link workers stop.
- **Tests written first in `client_test.go`.** `TestAppJoinCreatesStoreBeforeIdentity` checks store close before publication, retry reuse and no publication on failure. `TestAppRunRequiresEnrollment` checks exit 1 and zero dials for bad enrollment or key mismatch. `TestAppRunClosesState` checks locking through cancellation. `TestAppLogRedactsSecrets` checks fixed refusal, progress and recovery lines, both standings and loopback revocation. `TestAppPreservesStateOnRemoval` checks exit 0, preserved files and lock release. `TestAppRefusesUnsyncedIdentity` checks zero dials. `TestAppRunStoreStatus` expects status 1 on retryable faults or logged invalid zero. Exhaustion and impossible marks exit 0 with item 7b's exact diagnostic (architect-r3 3, logic-r3 1, UNREVIEWED FIX, logic-r4 1).
- **Acceptance.** `go test -race ./src/client/internal/app` passes.

**Item 9. Command line and in-process test entry**

- **Files.** Change `src/client/cmd/lgtm-client/main.go` and `src/client/cmd/lgtm-client/main_test.go`. Create `src/client/clienttest/clienttest.go` and `src/client/clienttest/clienttest_test.go`.
- **Needs and parallel work.** Needs item 8 and the Sprint 1 command stub. Contract decisions 82 and 95 settle token input (UNREVIEWED FIX). It is the last build item. The orchestrator can write combined scenarios against the signatures below while the earlier items build.
- **Area, complexity and scope.** `backend`, low, `client/backend`. It delegates to existing application operations and maps command exits.
- **Statements.** D2, D16, R10 and C5 to C11. The design section is "The client in wave 1".
- Preserve `var version`. Give `run` and `runContext` an `stdin io.Reader` beside their two writers. Pass `os.Stdin` from `main`. Use a context canceled by interrupt or termination and release its signal registration before calling `os.Exit`. `version` still prints the injected version and exits 0. `run` has no positional arguments and uses the app's exit status. Missing, unknown or extra arguments print a fixed usage line and exit 2 (UNREVIEWED FIX).
- Keep `join <address> <token>` for the installer and Machines page. When the token argument is `-`, read one token line with a bounded reader and trim its line ending. Empty input or a token line longer than 4096 bytes after trimming produces the fixed usage line and exits 2. Both forms pass the token to the same `app.Join` call. Enrollment returns 0 on success and 1 on error. Usage and diagnostics never print the token (UNREVIEWED FIX).
- **Exactly three exports in `src/client/clienttest`.** `type Options struct { StateDir string; Version string; Getenv func(string) string; Stderr io.Writer }`, `func Join(ctx context.Context, opts Options, address, token string) error` and `func Run(ctx context.Context, opts Options) int`. The package imports only its own program's application entry and standard-library types. It requires a nonempty absolute disposable `StateDir`. `Join` returns an error on invalid state-directory input. `Run` returns 1 on invalid input. `Getenv` may be supplied to isolate the home and executable search path without changing process-wide environment. `Run` blocks until it closes the folder and returns the same status as the command. An external test starts the server first, joins through the actual endpoint, starts `Run` on a goroutine and cancels and joins it before cleanup. It can observe state through the real server and fixed logs. The package exports no credential, mutable store, fake transport or private process hook.
- **Risky.** Yes, because the command receives a secret and maps restart exits. Echo the supplied arguments on usage error or replace status 75 with 0.
- **Tests written first.** `TestRunCommandExitStatus` in `main_test.go` checks version, bad arity without echoing the secret, failed join with a refusal or exhausted retries, and run status 0, 1 or 75 for cancellation or permanent stop, a failed commit and accepted restart, respectively. `TestClienttestSharesApp` in external package `clienttest_test` checks the same enrollment and diagnostics as the binary, an explicit absolute state path and lock release on cancellation. A stub listener exercises the entry point. The combined suite uses the real server and database.
- Update the existing `run` calls in `main_test.go` to pass a reader. Add `TestRunJoinTokenInput` for a valid line, empty input, oversized input and a positional token. Assert both valid forms reach enrollment with the same token. Assert the positional form does not read standard input and that invalid input exits 2 before enrollment. Check both output streams for token redaction (UNREVIEWED FIX).
- **Acceptance.** `go test -race ./src/client/cmd/lgtm-client ./src/client/clienttest` passes. A binary built with `go build -ldflags '-X main.version=0.0.0-test' ./src/client/cmd/lgtm-client` prints `0.0.0-test` for `version`.

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
| `TestLinkCommandAttemptsAcrossReconnect` and `TestLinkRestartReplaysAfterDroppedResult` | Hold a scan across reconnect until the new inventory is applied. Release it, observe failed on the latest attempt, then observe done for a subsequent scan. Drop a committed restart result, relaunch and replay done once (logic-r3 3). |
| Busy scan expiry | Replay a saved expiry while busy. Expect no write. A new expired scan saves expiry without running. The stream stays open (UNREVIEWED FIX). |
| Removal notice and refusal | Run [contract post-build check 30](../go-s0-contract/plan.md). Observe a continuing heartbeat after the notice alone, a fresh dial after ordinary disconnection and a stop after verified removal (UNREVIEWED FIX). |
| `go test -race -count=1 -v ./src/linktest/...` | Run the scenarios with disposable `LGTM_TEST_POSTGRES`. Without it expect explicit skips. |
| C10 after Stack Client lands | `deploy/test/checks.sh --only C10` uses the absolute fixture override. Check `identity.json` remains private after replacing its saved key. Observe both refusal codes within 20 seconds each, then require 13 passes in the full run (logic-r2 6). |
| Installed service with a store fault | Make `client.db` unreadable to the service user. Expect exit 1, five-second retries and recovery on restored access. Damage exits 1 and preserves the file. Inject an impossible mark on an intact fixture. Expect exit 0, no restart after 15 seconds and one `link_stopped` with `impossible_ack`. Removal yields `link_refused` with `unknown_machine` (architect-r3 1, logic-r3 1, UNREVIEWED FIX, logic-r4 1). |
| Sleep, blocked sends and immediate replies | Sleep past expiry on macOS. Expect expired without work. Cancel a blocked send. Both workers exit and a new stream opens. Run `TestSessionAcceptsImmediateAck` with `-race -count=200`. The reply for the handed-off event is accepted before send returns (UNREVIEWED FIX). |

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

Sources are the client-link assignments, the epic, the approved foundation design, issues 1632 through 1642, phase 4 of the build loop, the current prerequisite plans, their fold requests, and the wave 1 user-flow script. The [link freeze amendment](../go-s0-contract/plan.md#amendment-the-freeze-of-the-link) adds five requests (UNREVIEWED FIX).

| Source item | Workstream | Status |
|---|---|---|
| Bounded task and brief, enrollment, proof, retry, reporting and commands | WS-1 to WS-3 | Items 1 to 9 and the combined acceptance table. |
| Task, issue 1634 and epic, in-process tests | WS-3 | Item 9 supplies the client entry. The Server team and orchestrator own the server entry and combined suite. |
| D1 and D2, the product boundary and Go rewrite | WS-1, WS-2, WS-3 | All network work uses generated types. Items 8 and 9 wire the Go program. |
| D3 to D5 and D9 to D11 | Outside client scope | The Server and Stack plans own store, feed, vault and sign-in. |
| D7 and D17, the key exchange and trust | WS-1, WS-2 | Items 1 to 3 check the certificate, public key and proof. |
| D8, the agent protocol | WS-2 | Item 5 reads harness versions. Driving an agent is later work. |
| D16, outbound connection, retry and restart | WS-2, WS-3 | Items 3, 6, 7a, 7b and 9 (architect-r2 7). |
| D6, D12 to D15, D18 and D20 to D23 | Later wave | The foundation design defers these. The token stays session-local. |
| D19, the machine user's client state and repositories | WS-2, WS-3 | Items 5 and 8 read the resolved user settings. |
| R1 and R2, ownership and frozen contract | WS-1, WS-2, WS-3 | Rollout 1 and the guardrails. |
| R3 to R5 and R7, database ownership, writes and watch | Outside client scope | The client calls neither these packages nor their tables. |
| R6, durable commands | WS-2, WS-3 | Items 6 and 7b consume commands received on the link. |
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
| Sprint 0 public messages, proof and results | WS-1, WS-2, WS-3 | Items 1 to 6 and 7b use the wire contract. Rollout step 1 names the shared freeze gate. |
| Link freeze amendment | WS-1, WS-2, WS-3 | Decisions 64 to 68 trace the five requests to items 1b, 2, 3, 5, 6, 7a, 7b and 9 (UNREVIEWED FIX). |
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
| 1b. Outbox sequence limit | backend | client/backend | low | One append guard and two existing boundary tests (UNREVIEWED FIX). | Yes | Built Sprint 1 store and the frozen event limit. |
| 2. Enrollment | backend | client/backend | medium | One generated unary call and one existing durable write boundary. | Yes | 1, 1b, Sprint 0, Sprint 1 items 2 and 3 (UNREVIEWED FIX). |
| 3. Stream proof | backend | client/backend | medium | The proof fixture covers one handshake, independent of retry and reports. | Yes | 1, 2 and the Sprint 0 link contract. |
| 4. Event replay | backend | client/backend | medium | One send cursor over the existing transactional store and the server's committed mark. | Yes | 1b, Sprint 1 item 6 and the Sprint 0 mark rules (UNREVIEWED FIX). |
| 5. Reports | backend | client/backend | medium | Existing local probes produce two durable report kinds and one heartbeat. | Yes | Sprint 1 probes and store. Uses 4's agreed encoding. |
| 6. Commands | backend | client/backend | medium | Three fixed actions reuse the receipt store with one atomic decline write. | Yes | 3 and 5, Sprint 1 store and Sprint 0 commands. Owns `Store.Decline`. |
| 7a. Retry | backend | client/backend | medium | Owns reconnect scheduling and teardown. | Yes | 3 and the freeze fixture `src/contracts/testdata/link-codes.json` (UNREVIEWED FIX). |
| 7b. Session | backend | client/backend | medium | Composes independently tested link pieces. | Yes | 3 to 6 and 7a. |
| 8. Application lifecycle | backend | client/backend | medium | Opens the local resources and orders their teardown through one call. | Yes | 2, 7b, Sprint 1 state and config. |
| 9. Command and test entry | backend | client/backend | low | Thin entry points reuse the application lifecycle. | Yes | 8 and the Sprint 1 command stub. |

Load for build and code checks. Eleven units need eleven test writers, eleven builders and twenty-two reviewers. Check the vendor allocation and remaining allowances at dispatch (UNREVIEWED FIX).

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (B/M/m) | Outcome |
|---|---|---|
| 1 | 2 / 21 / 19 | Folded in `attack/round-1-findings.md`. |
| 2 | 0 / 6 / 8 | Folded in `attack/round-2-findings.md`. |
| 3 | 0 / 4 / 6 | Folded in `attack/round-3-findings.md`. |
| 4 | 0 / 1 / 0 | `attack/round-4-findings.md` lists each `UNREVIEWED FIX`. |

</details>
