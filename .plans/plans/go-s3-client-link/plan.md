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

The client records which commands it has started or finished, including a scan or restart requested from the site. That record lets it answer retries without repeating completed work.

## Key decisions

A machine checks two identities. It checks the public key of the certificate presented by the stack before sending an enrollment token over an encrypted connection. It also checks the server's separate signing key when it joins and every time it connects. A certificate renewal can keep the same key. Replacing either pinned key requires a new enrollment.

The Machines page and the installer use `lgtm-client join <address> <token>`. The address must agree with the address inside the token. The positional token can appear in shell history and process arguments. A successful join writes the enrollment before returning and leaves the machine waiting for the owner.

An approved machine saves reports before sending them and keeps them until the server acknowledges them. A stored command result survives a dropped connection. If the process dies during a command, the next process reports the interruption and keeps that action from running a second time under the same identifier. Receipt retention uses the machine's clock, so a clock set ahead can remove a receipt before the server's retry window ends.

A restart request ends the client after saving its receipt. A service manager starts it again. A person running the program in a terminal starts it again from the same state folder.

Full log → [decisions.md](decisions.md).

## Workstreams

### A machine joins the server it was given

The owner copies a command from the Machines page and sees a new entry waiting for approval. The client keeps the same key when a join reply is lost. Checks of the token's origin and the server's keys protect the enrollment from a wrong endpoint.

### A machine proves itself and sends its reports in order

The machine reports its facts, load and harness inventory after approval. It dials again by itself after the server restarts. It keeps reports made during an outage, and the server receives them in the order in which the machine saved them.

### Commands survive disconnections without running again

An approved machine can scan its harnesses on request. It can take a new name or standing from the server. A restart request leaves a durable receipt before the program stops, and removal stops the machine from trying again.

## Rollout order

1. Integrate the Sprint 0 drafts and Sprint 1 local-state interfaces. Freeze the link when this plan and `go-s3-server-machines` both clear review.
2. Build origin checks, enrollment, proof, reporting and receipt handling.
3. Connect the report and command paths to the client process. Then connect the command line and the in-process test entry to that same process.
4. Run the combined tests with a real server and database. Run the wave 1 stack checks and paste the Machines page's command once on a disposable machine.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**Shared build context**

The root Go module is `github.com/qnlbnsl/lgtm-studio`, with Go 1.26 or newer. The wire types are generated from Protocol Buffers in `src/contracts/gen/go/lgtm/link/v1` as `linkv1`, with the Connect bindings in `linkv1connect`. The service methods are `LinkService.Join` and `LinkService.Open`. Their paths are `/lgtm.link.v1.LinkService/Join` and `/lgtm.link.v1.LinkService/Open`.

The wire names come from the Sprint 0 plan and its current drafts. D numbers refer to design decisions, R numbers to design rules and C numbers to wave 1 checks, in source order.

The command uses `config.StateDir` with an absolute, nonempty `LGTM_CLIENT_STATE_DIR` override and a default of `$HOME/.local/state/lgtm-client`. The caller holds `state.Open` until after the embedded `link.Store` closes. The transactional database `client.db` holds reports and receipts. Sprint 1 local-state item 6 adds `go.etcd.io/bbolt` v1.4.3 with its first import.

The test writer freezes each named test against the design and this plan before the builder starts. If a symbol is greenfield, a compiling stub for that item returns zero, false or an empty value. The first run must fail on the assertion described here. Risky tests must also fail after the named code mutation on a temporary branch. The lead checks the exact immutable commit and checks the temporary merge again. The combined tests are owned by the orchestrator under `src/linktest/`; isolated listeners check framing and failures. Combined tests start the real server with `servertest.Start(t, servertest.Options{Password: ...})` from Sprint 2 sign-in, using its `URL`, `HTTPClient`, `SigningPublicKey`, `Stop(t)` and `Restart(t)` as needed. The fixture supplies a disposable PostgreSQL database and the OpenBao wire fixture.

**WS-1: A machine joins the server it was given**

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 1a | A token for another origin, a plain connection outside loopback, a redirect and a wrong certificate key leave the token unsent to the refused endpoint. | `TestOriginAndDialPolicy`, `TestPinnedTransport`, `TestJoinRejectsAddressAndRedirect`. |
| 1b | A lost reply followed by a retry leaves one waiting machine and one saved enrollment. A bad signing key leaves the enrollment file absent. | `TestJoinReusesDurableKey`, `TestJoinCommitVerifiedEnrollment`, `TestJoinReplay` in the combined suite. |
| 1c | The Go client joins with the pasted command and the open machine watch announces it. A used token with another key and an expired token are refused. | `deploy/test/checks.sh --only C5,C6` prints two `ok` lines and exits 0. In a real Machines page session, paste its command and observe the waiting row without a reload. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 1 | Require the permitted origin and pin the encrypted endpoint. | `src/client/internal/transport/transport.go`, `ParseOrigin`, `NewHTTPClient`. |
| 2 | Decode and verify enrollment, then expose its commit. | `src/client/internal/enroll/join.go`, `Join`, `SaveServer`, `LoadServer`. |

**Item 1. Origin and transport**

- **Files.** Create `src/client/internal/transport/transport.go` and `src/client/internal/transport/transport_test.go`.
- **Needs and parallel work.** Needs the Sprint 0 draft origin and certificate-pin rules. It can run beside item 5.
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
- **Needs and parallel work.** Needs item 1, `go-s1-client-local` items 2 and 3, and the Sprint 0 enrollment draft. It can run beside item 5. Item 3 consumes the saved enrollment.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. One unary call uses the local plan's key and atomic write primitives.
- **Statements.** D7, D17, R8, R9 and C5, C6 and C10. The design section is "Joining: a token that authorizes an exchange of keys".
- **Public names.** `type Server struct { Address string; MachineID string; ClientPublicKey ed25519.PublicKey; PublicKey ed25519.PublicKey; CertificateKeySHA256 []byte }`, `func LoadServer(f *state.Folder) (Server, error)` and `func Join(ctx context.Context, f *state.Folder, address, token, hostName string) (Server, error)`. `ErrEnrollmentExists`, `ErrEnrollmentMissing` and `ErrEnrollmentInvalid` distinguish local conditions. `type JoinError struct { Code string }` implements `error` with fixed messages. `func SaveServer(f *state.Folder, server Server) error` commits a verified enrollment. It never embeds an upstream message or the token.
- Decode the `lgtm1_` prefix and unpadded base64url text to `JoinToken.token_id`, `secret`, `address`, `server_key_sha256` and `certificate_key_sha256`. Check the canonical lowercase universally unique identifier (UUID), the lengths of hashes and secret, and the destination policy. Normalize and compare both origins before any network activity, then use the token origin. Cap the encoded input at 4 kibibytes and each attempt at 10 seconds. Stop retrying 15 minutes after the first attempt, measured by the monotonic clock. The server alone judges expiry and can refuse an expired token on the first attempt.
- First refuse an existing `f.Paths().Identity`, including an invalid file. Load the key with `enroll.Load`. Create it only on `ErrKeyMissing`. On `state.ErrPlacedNotSynced` from creation, load the same key and require a successful `state.SyncDir` before sending its public half. A new process with a previously pending key also confirms the directory sync before the first attempt. A key read error stops enrollment.
- Send the generated `JoinRequest` with `token_id`, `token_secret`, `machine_public_key` and `host_name`. An unavailable host name is empty. Validate the name as UTF-8 (Unicode Transformation Format, 8-bit) and cap it at 255 bytes on a character boundary. Retry a transient failure or lost reply with the same token and key after 1, 2, 4, 8 and then 15 seconds. Clip requests and waits to the remaining budget. Stop on `invalid_argument`, `permission_denied`, a pin mismatch or a bad successful reply. For `permission_denied`, accept only `token_unknown`, `token_expired`, `token_revoked`, `token_used` and `machine_removed` from `Lgtm-Join-Refusal`. Use `token_refused` for an absent or unrecognized value. Use fixed diagnostics for malformed input and pin or signing-key mismatches.
- `Join` validates `JoinResponse.machine_id` and the 32-byte `server_public_key` against the token digest in constant time, then returns the verified `Server`. At item 8's commit step, `SaveServer` uses `state.WriteAtomic` to write mode `0600` `identity.json`. Its JavaScript Object Notation (JSON) fields are `version` equal to 1, `address`, `machineId`, `clientPublicKey`, `serverPublicKey` and `certificateKeySHA256` when applicable. Byte strings use padded standard base64. `LoadServer` validates those fields, the origin and key lengths. It rejects extra content and reads only a bounded, regular, private file owned by the current user without following links. After `state.ErrPlacedNotSynced`, success requires a validated reread and `state.SyncDir`. A failed sync leaves the file and key in place.
- **Risky.** Yes, for credentials and crash ordering. Skip the reply-key comparison or generate a fresh key after an unavailable response.
- **Tests written first in `join_test.go`.** `TestJoinCommitVerifiedEnrollment` checks that a verified reply returns the enrollment without publishing it, that `SaveServer` writes the specified file and that a bad identifier or key never yields an enrollment to save. `TestJoinReusesDurableKey` drops a reply, reads the durable key before the first request and checks both requests use it. A committed identity prevents a second join. `TestJoinRejectsAddressAndRedirect` observes zero requests to a mismatched origin or redirect target. `TestJoinRejectsExpiredAndRedactsRefusal` checks all five reason codes, fallback, exhaustion and exclusion of remote text, token and secret from errors. `TestLoadServerRejectsUnsafeFile` covers a symlink, wrong mode, bad version, bad key length and a missing file. `TestJoinSyncFailureKeepsKey` observes no request until the key placement has synced.
- **Acceptance.** `go test ./src/client/internal/enroll` passes all six tests and the credential tests from Sprint 1.

**Design artifact.** The approved foundation design, "Joining: a token that authorizes an exchange of keys". The client plan changes no screen hierarchy.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| The client discloses a usable join token to another endpoint. | High | Validate the origin and leaf pin before writing the request. Reject redirects and log only fixed reasons. |
| A join succeeds on the server but the reply is lost. | High | Save the private key first. Replay with the same key and check that the server makes only one waiting record. |
| An owner copies a token into a shell that keeps its history. | Medium | Keep the existing positional agreement for wave 1. The handoff records command arguments and shell history as exposure surfaces for the orchestrator's pending contract decision. |

**Verify.** Run the tests for items 1 and 2 and the combined `TestJoinReplay`. Run C5 and C6 through the loopback stack and paste the real Machines page command in a disposable environment. A separate loopback HTTPS listener exercises pinning with a self-signed leaf.

**Rollout detail.** Steps 1, 2 and 4 freeze names, build the command and run the observations. Build the command from `./src/client/cmd/lgtm-client`. Run `lgtm-client join <address> <token>` with the address issued for that disposable stack.

**WS-2: A machine proves itself and sends its reports in order**

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 2a | A fresh challenge proves both ends. An invalid server proof sends no machine proof, and a terminal peer refusal requires a verified challenge. | `TestOpenProofFixture`, `TestOpenRejectsWrongProofAndOrder`, `TestRetryStopsForVerifiedRemoval`, C10. |
| 2b | Approval sends facts and all four scan entries. Load updates, a stop and a server restart are seen through the machine watch. | `TestReportInventoryAndLoad`, `deploy/test/checks.sh --only C7,C8,C9` prints three `ok` lines and exits 0. |
| 2c | Three reports saved during an outage are applied once and in order, and a later report also arrives. | `TestOutboxReplayAfterServerFailure` and `deploy/test/checks.sh --only C11` prints one `ok` line and exits 0. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 3 | Prove both ends of a fresh stream. | `src/client/internal/link/conn.go`, `openSession`, `proofText`. |
| 4 | Send and acknowledge durable reports in sequence. | `src/client/internal/link/replay.go`, `replay`; Sprint 1 `Store`. |
| 5 | Collect facts, inventory and load, and append reports. | `src/client/internal/link/report.go`, `reporter`, `captureFacts`, `scanHarnesses`. |
| 7 | Supervise connections, lifecycle and reporting. | `src/client/internal/link/retry.go`, `Run`, `RunOptions`. |

**Item 3. Opening and proving a stream**

- **Files.** Create `src/client/internal/link/conn.go`, `clock_linux.go`, `clock_darwin.go` and `conn_test.go` in that package.
- **Needs and parallel work.** Needs items 1 and 2, Sprint 1 `Credential` and the Sprint 0 `LinkService.Open` draft. It can run beside item 5.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. It handles three proof messages on a generated stream and verifies the golden fixture.
- **Statements.** D7, D16, D17, R9, R10 and C8 to C11. The design sections are "One connection, dialled by the machine" and "Both ends prove themselves on every connection".
- **Names shared inside the package.** `openSession` takes a context, a generated `LinkServiceClient`, `enroll.Server` and `*enroll.Credential`. Its `session` owns a stream, cancellation, a validated `*linkv1.Welcome` and the elapsed-clock reading at proof send. It supplies `send`, `receive` and `close`. `proofText` builds the signed inputs. Errors distinguish a server-key mismatch and a terminal refusal with a fixed code.
- Start each attempt with a fresh cryptographically random 32-byte nonce. Send `OpenRequest.hello` with `Hello.machine_id`, `machine_nonce`, and `link_revision` equal to 1. Accept `OpenResponse.challenge` with exact 32-byte `server_nonce` and 64-byte `server_signature`. Verify Ed25519 using the key returned by `LoadServer`. The signed bytes use the literal label `lgtm-link-v1 server` or `lgtm-link-v1 machine`, followed by a zero byte, one byte for the identifier length, the identifier bytes, the machine nonce and the server nonce. When the server signature verifies, send `OpenRequest.proof` with `Proof.machine_signature` from `Credential.Sign`. Read the sleep-inclusive monotonic clock just before sending it. Use `unix.ClockGettime` with Linux `CLOCK_BOOTTIME` and macOS `CLOCK_MONOTONIC_RAW`. Item 3 lists `golang.org/x/sys` v0.50.0, already selected by the Sprint 2 sign-in dependency. Its builder makes the dependency direct with the first direct import.
- Require `OpenResponse.welcome` with valid `Welcome.state` and `server_time`. Check `link_revision` and keep `last_event_number` for item 4. Require an empty `connection_token` for `APPROVAL_WAITING`. Treat unspecified or unknown approval as waiting and discard any token in that case. A usable token stays in the session, and wave 1 has no side calls.
- Give each exchange 10 seconds, including a blocked send. Cancel the attempt to unblock the stream on timeout. Accept only the prescribed frame for each handshake step. An `unauthenticated` status is retryable. A `failed_precondition` status before a verified challenge is retryable. After a verified challenge it becomes a terminal refusal. The `Lgtm-Link-Refusal` values `unknown_machine`, `link_revision_mismatch` and `event_gap` allow the client to choose a fixed diagnostic. An unrecognized or missing value selects `link_precondition_failed`. An incompatible authenticated `Welcome.link_revision` has the same terminal effect as a revision refusal. A bad signature becomes `errServerKeyMismatch` and sends no proof. A broken stream and `aborted` are retryable.
- **Risky.** Yes, for key proof and message order. Swap the two nonces in one proof, accept a bad server signature, or treat an early unauthenticated status as a terminal refusal.
- **Tests written first in `conn_test.go`.** `TestOpenProofFixture` fixes both proof texts and signatures against `src/contracts/testdata/link-proof.json` and checks the exact `Hello`, `Proof` and validated `Welcome`. A separate attempt has a different nonce. `TestOpenRejectsWrongProofAndOrder` observes no proof after a bad or stale signature or wrong frame kind. It refuses an incomplete welcome and grants waiting privileges for unknown approval. `TestOpenExpiryAndRefusal` exercises each 10-second deadline, retries `unauthenticated` and an early `failed_precondition`, and stops on `failed_precondition` after a verified challenge or a revision mismatch in the authenticated welcome.
- **Acceptance.** The three tests pass in `go test ./src/client/internal/link -run 'TestOpen'`.

**Item 4. Durable event replay**

- **Files.** Create `src/client/internal/link/replay.go` and `src/client/internal/link/replay_test.go`.
- **Needs and parallel work.** Needs the Sprint 1 `Store` and the Sprint 0 event draft. Runs beside items 3 and 5 using the payload format below. Item 7 binds it to a session.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. It reads one existing queue through an ordered cursor and validates the acknowledgements.
- **Statements.** R6, R9, R10, C9 and C11, and the design section "Events come up, and none is lost or applied twice".
- **Names shared inside the package.** `replay` owns the send cursor and the current stream's successful-send ceiling. `encodeReport(*linkv1.Event) ([]byte, error)` and `decodeReport([]byte) (*linkv1.Event, error)` share the payload format with item 5.
- The payload passed to `Store.Append` starts with byte 1. The rest is the current binary `Event` with number unset, `happened_at` set at capture and one `facts` or `harnesses_found` report. The writer never patches a stored record. The replay path gives `Event.number` the value of the row's `Event.Seq` and wraps it in `OpenRequest.event`. A bad payload stops delivery with a local diagnostic and keeps the store file.
- Call `Store.Ack(Welcome.last_event_number)` on a verified welcome, before offering any event. The server alone decides the committed prefix. The mark initializes the successful-send ceiling, and the first bounded `Store.Read` starts at the next number. Events keep their original numbers. Issue 1641 excludes restored state and issue 1642 excludes two live copies of one machine.
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
- When `Append` succeeds, log `msg=link_event_buffered` and `sequence` as that positive numeric sequence. Serialize append and the associated log write so their observations keep commit order. On `ErrOutboxFull`, keep the newest refused facts and automatic inventory snapshots of each kind and retry when an acknowledgement frees room. Send store failures to the supervisor and a requested scan's append failure to the command handler.
- **Risky.** Yes, for ordering and disclosure. Move the log before the append returns, or use `Found.Path` as a version.
- **Tests written first in `report_test.go`.** `TestReportInventoryAndLoad` supplies facts, settings and four scan entries, including unknown versions and marked paths. It checks the six facts fields, four names, optional versions and no marked strings. It checks present zero, measured nonzero and absent load values without issuing an outbox append. `TestReportLogFollowsDurableAppend` pauses the Sprint 1 transaction commit return using the store test hook and observes no log until the append returns. `TestReportBackpressure` fills a one-entry outbox, confirms no extra sequence or log, frees the entry and checks that only the most recent refused facts append. `TestReportUncertainAppendIsNotRepeated` injects an ambiguous commit failure, asserts no success log and no further append in that run, and reopens to verify that any committed payload keeps its sequence.
- **Acceptance.** The four tests pass in `go test ./src/client/internal/link -run 'TestReport'`.

**Item 7. Connection supervision and report timing**

- **Files.** Create `src/client/internal/link/retry.go`, `src/client/internal/link/session.go` and `src/client/internal/link/retry_test.go`.
- **Needs and parallel work.** Needs items 3 to 6. Item 8 can prepare its tests alongside this item using the public names below.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. It composes the proof, queue, reporter and command handler that the earlier items test independently.
- **Statements.** D16, R6, R9, R10 and C7 to C11. The design sections are "Knowing whether a machine is there", "What travels in wave 1" and "The client in wave 1".
- **Public names.** `type RunOptions struct { Server enroll.Server; Credential *enroll.Credential; Folder *state.Folder; Settings config.Settings; ScanOptions harness.ScanOptions; Version string; Logger *slog.Logger }` and `func Run(ctx context.Context, opts RunOptions) (RunOutcome, error)`. `type RunOutcome uint8` with `RunStopped` equal to 0 and `RunRestart` equal to 1. A non-nil error takes precedence over the outcome. The caller owns the open folder. `Run` opens and closes `Store` with that folder and constructs the pinned client. It dials the configured server and never opens a listening port. It returns after all workers have stopped.
- Retry a broken link, unverified refusal, signing-key or certificate mismatch, superseded stream or server unavailability. Wait 1, 2, 4, 8 and then at most 15 seconds between tries, including after `aborted`. Reset after 30 seconds on a valid stream. On `aborted`, say another process may be using this machine key. Log the same failure once until it changes or the link recovers. Cancellation interrupts a try or wait. If a send closes before status is known, collect it on the receiver within the deadline. A final `failed_precondition` after a verified challenge stops the run with one `link_refused` diagnostic. For `unknown_machine` or verified removal, tell the owner to remove the old entry if present and join with a new token and fresh state folder. For `link_revision_mismatch`, tell the owner to upgrade. For `event_gap`, tell the owner to re-enroll. A bad server signature logs `server_key_mismatch`.
- Keep one sender and receiver per session and one owner for incoming frames, command completion and the active standing. Ignore unknown frame kinds after welcome. Close the session on a known invalid frame or 30 seconds without a valid response. A timer can cancel blocked network work. Only the current session advances replay or delivers commands. A process-owned scan can finish across reconnects. Controlled process exit cancels and settles that work. Shut down network work before retry, and close the store after all workers settle.
- Initialize replay and standing before work on the session. An approved welcome or new approval queues current `Facts` and one fresh `HarnessesFound`. The initial scan leaves the heartbeat timer free. A process that has seen approval captures facts at outage detection and every 10 seconds until a fresh standing is established, across retries. A process that has never seen approval queues no reports during an outage. While waiting, retain the queue and send bare heartbeats. The server ignores a waiting machine's command results. An approved session sends queued events in number order. The machine's capture timestamp is for display.
- `Run` calls `OpenStore(opts.Folder, StoreOptions{})` before dialing. Treat `ErrStoreMissing`, `ErrStoreUnsafe`, `ErrStoreDamaged`, `ErrSequenceExhausted` and impossible acknowledgements as permanent stops with a fixed diagnostic naming `client.db`. Preserve the file if present. A failed commit wrapping `ErrStoreFailed` fails the run so the next process can reopen the store. The process stops writing after that failure.
- **Risky.** Yes, for connection and producer ordering. Remove the 15-second cap, treat an unsigned refusal as removal or start the report producer while waiting.
- **Tests written first in `retry_test.go`.** `TestRetryBackoffAndSilence` checks the capped waits, stable-session reset, 30-second silence limit and nonoverlapping streams. `TestRetryStopsForVerifiedRemoval` checks unverified refusal, key mismatch, `aborted` with its warning and retry cap, and terminal diagnostics for verified refusal and removal. `TestRetryReportsOnApprovalAndOutage` observes bare heartbeats while waiting, facts and inventory on approval and three durable facts made during an outage that precede later reports in replay. `TestRetryFailureExitsAndKeepsStore` covers missing, unsafe and corrupt stores before dial, sequence exhaustion and a failed commit without further send or rebuild.
- **Acceptance.** `go test -race ./src/client/internal/link -run 'TestRetry'` passes all four tests.

**Design artifact.** The approved foundation design, "The link between the server and a machine".

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A machine discloses its proof to an endpoint that holds a different signing key. | High | Check the golden signed texts and the negative assertions before sending any proof. |
| A guessed acknowledgement removes reports that the server never applied. | High | Only the authenticated welcome sets the initial mark. Live acknowledgements are bounded by the successful writes on that stream. |
| A stalled stream prevents a process from reconnecting or observing removal. | High | Bound proof exchanges and post-proof silence. Cancel blocked network work and distinguish terminal removal from transient errors. |
| An outage fills the local store. | Medium | The store refuses before numbering a report. Retain the newest refused snapshots and let acknowledgements free capacity. |

**Verify.** Run `go test -race ./src/client/internal/link`. The environment checks are C7 to C11 and the combined tests `TestPresenceAcrossRestart`, `TestKeysAndRemoval` and `TestOutboxReplayAfterServerFailure`. Verify the link stays open through the loopback stack web server for ten minutes of heartbeats.

**Rollout detail.** In step 4, run `deploy/test/checks.sh --only C7,C8,C9,C10,C11` in the disposable stack.

**WS-3: Commands survive disconnections without running again**

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 3a | A retry and a scan completion may arrive in either order across reconnect. One scan runs and the current attempt receives its result. | `TestCommandLatestAttemptAndReplay`, `TestCommandReconnectWhileScanRuns` and `TestCommandAttemptsAcrossReconnect` in the combined suite. |
| 3b | A new scan or restart skips work at expiry. An interrupted action reports that outcome on replay. A restart saves its result before exit and replays the result after relaunch. | `TestCommandExpiryUsesServerTime`, `TestCommandRestartAndRecoveredReceipt`, `TestRestartReplaysAfterDroppedResult` and the installer check `live_reconnect_restarts_service`. |
| 3c | A fresh run reports how to join and sends no network request. A stopped client releases its lock. The service sees status 75 on a requested restart and status 0 on removal. | `TestAppRunRequiresEnrollment`, `TestAppRunClosesState`, `TestRunCommandExitStatus`, `TestClienttestSharesApp`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 6 | Handle approval state and bounded commands through receipts. | `src/client/internal/link/commands.go`, `commandHandler`, `serverClock`; Sprint 1 `Store.Begin` and `Store.Finish`. |
| 8 | Start and stop the process from shared application entry points. | `src/client/internal/app/client.go`, `Join` and `Run`. |
| 9 | Connect the command line and the in-process test entry. | `src/client/cmd/lgtm-client/main.go`, `runContext`; `src/client/clienttest/clienttest.go`, `Options`, `Join` and `Run`. |

**Item 6. Standing and command receipts**

- **Files.** Create `src/client/internal/link/commands.go` and `src/client/internal/link/commands_test.go`.
- **Needs and parallel work.** Needs Sprint 1 `Store.Begin` and `Store.Finish`, the Sprint 0 commands draft, item 3's authenticated welcome and clock and item 5's reporter. It can run beside item 4.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. It handles the fixed actions through standing revisions and the receipt store.
- **Statements.** D16, R6, R9, R10, C7 to C9. The design sections are "Commands go down, and none is lost or repeated" and "What travels in wave 1".
- **Names shared inside the package.** `commandHandler` handles the current `MachineState`, one active requested scan and command completion events. Its `serverClock` uses `Welcome.server_time` plus the elapsed time since proof send as an upper estimate, assuming the server clock does not step forward. The sample was taken after proof send, and the elapsed clock advances through sleep. Recompute for `Begin` and immediately at the start of work. An unavailable or backward elapsed reading prevents new work. If expiry passes after the start receipt commits but before work starts, finish with `COMMAND_OUTCOME_EXPIRED` and skip the action.
- Verify the command identifier, positive attempt and timestamp shape. Apply `Welcome.state` before commands. A higher valid revision updates standing. Identical content at the same revision changes nothing. Different content at the same revision logs a fixed diagnostic and keeps the accepted state. A lower revision changes nothing. An unspecified or unknown approval grants waiting privileges only. A verified removal stops the process.
- Handle `SetState` by revision without calling `Begin` or checking expiry. Reply with done for an applied or redundant state and failed for a conflicting revision. Removal can stop before a reply. It is the only command the client accepts while waiting. Use `Store.Begin(Command{ID, ExpiresAt}, now)` when an approved session admits a scan, restart or unknown action. `DecisionRun` permits work, `DecisionRunning` waits, `DecisionReplay` uses the recorded outcome and the delivering attempt, `DecisionInterrupted` answers interrupted without changing the receipt, and `DecisionExpired` answers expired. Finish an unknown action as unsupported. A begin error permits no action. Close the stream if a distinct requested scan cannot enter the single slot, so the server can redeliver it.
- One process owner orders delivery and finish across reconnects. While a command runs, keep the highest attempt and its supplying session in memory. If delivery precedes completion, send the result only to that supplying session if it is still current. If finish comes first, commit the result and replay the outcome under the later delivery's attempt. A result queued for an old session stays off a replacement session. Select the attempt at send admission under the same ordering.
- A scan yields `COMMAND_OUTCOME_DONE` only after the report commits. A full buffer or canceled scan yields `COMMAND_OUTCOME_FAILED`. Call `Finish(id, result)` before replying for every command admitted to run, including on controlled shutdown. Store one byte containing the defined outcome. If `ErrResultTooLarge` leaves the command running, retry with the single-byte failed outcome. `Finish` also removes valid finished receipts whose stored expiry precedes the local wall clock by more than 24 hours. A fast machine clock can shorten retention under the Sprint 1 ruling. Finish an accepted restart before spending up to two seconds on its result, then return `RunRestart` even if delivery failed. Write only `CommandResult.command_id`, `attempt` and `outcome`.
- **Risky.** Yes, for ordering and expiry. Run a scan before `Begin` commits, ignore elapsed sleep, send a running scan's result under its first attempt, or terminate a restart before `Finish` commits.
- **Tests written first in `commands_test.go`.** `TestCommandLatestAttemptAndReplay` observes one scan, a reply for the highest pending attempt and a replay under a later attempt. `TestCommandReconnectWhileScanRuns` covers delivery before finish and finish before delivery, with one scan and no result on a session that did not deliver that attempt. `TestCommandExpiryUsesServerTime` tests both wall-clock offsets, the expiry boundary, sleep through expiry, clock failure, expiry during the start commit and replay of an expired receipt. The store's `TestFinishPrunesByAge` checks retention by stored expiry. `TestCommandRestartAndRecoveredReceipt` holds the finish commit and observes no early reply or exit, then drops the reply and checks replay without a second restart. A restarted scan process answers interrupted. `TestCommandStateAndUnknownAction` covers an expired `SetState` while waiting, revision conflicts, removal and unsupported commands. `TestCommandScanRequiresDurableReport` covers a full buffer and a committed scan report. Inspect result frames for the three allowed fields.
- **Acceptance.** `go test -race ./src/client/internal/link -run 'TestCommand'` passes.

**Item 8. Shared application lifecycle**

- **Files.** Create `src/client/internal/app/client.go` and `src/client/internal/app/client_test.go`.
- **Needs and parallel work.** Needs items 2 and 7, Sprint 1 configuration and state, and the installer exit-code agreement. Tests can be prepared beside item 7 using `link.Run`'s documented signature.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. It sequences the private folder, credential, settings and lifecycle at the two entry points.
- **Statements.** D2, D7, D16, D19, R1, R9, R10, C6, C8 and C10.
- **Public names.** `type Options struct { StateDir string; Version string; Getenv func(string) string; Stderr io.Writer }`, `func Join(ctx context.Context, opts Options, address, token string) error` and `func Run(ctx context.Context, opts Options) int` in package `app`. The supplied state directory must be absolute. Empty uses `config.StateDir`. Nil `Getenv` uses `os.Getenv`. Empty version uses `0.0.0-dev`. Nil `Stderr` discards logs. The app writes only fixed diagnostics and local configuration values deliberately shown at startup. An upstream error is never written raw.
- `Join` resolves and locks the folder, reads the host name and calls `enroll.Join`. After the server confirms the join, call `link.CreateStore(f, link.StoreOptions{})` and close the resulting store before `enroll.SaveServer` publishes `identity.json`. A pending retry opens and reuses the store. A store failure leaves the identity unpublished. Emit one JSON diagnostic on a peer refusal with `msg=join_refused`, the allowlisted `code` and a fixed sentence in `message`. For an expired or unusable token with a pending enrollment, direct the owner to remove the waiting record if present and use a new token with a fresh folder. Close the locked folder before returning and leave existing state files in place.
- `Run` opens the locked folder, validates `identity.json` with `enroll.LoadServer`, syncs the directory and verifies the loaded key matches the saved public half. Missing or invalid enrollment exits 1 with an instruction to join. Load `config.LoadSettings` with the settings path, home and state directory and prepare `harness.DefaultScanOptions`. Log the resolved state and repository folders once and call `link.Run`. Return 0 on cancellation or permanent stop, 75 for an accepted restart, and 1 for other failures, including failed store commits. Use `msg=link_restart_requested` to tell a foreground user to run the command again. Log synchronously and close the folder after the store and workers finish.
- **Risky.** Yes, for trusting saved keys and lifecycle ordering. Dial before verifying the identity file, or release the folder on exit before the link workers stop.
- **Tests written first in `client_test.go`.** `TestAppJoinCreatesStoreBeforeIdentity` holds up store creation after a valid reply, observes no identity until the store closes, and checks a pending retry reuses a private store and key. An unsafe or failed creation leaves the identity unpublished. `TestAppRunRequiresEnrollment` checks exit 1 and zero dials for missing or invalid identity and a mismatched key. `TestAppRunClosesState` observes the lock held until the canceled run settles. `TestAppLogRedactsSecrets` checks the stable refusal code without remote text or secrets. `TestAppPreservesStateOnRemoval` checks exit 0, preservation and lock release. `TestAppRefusesUnsyncedIdentity` observes zero dials on a directory-sync failure.
- **Acceptance.** `go test -race ./src/client/internal/app` passes.

**Item 9. Command line and in-process test entry**

- **Files.** Change `src/client/cmd/lgtm-client/main.go` and `src/client/cmd/lgtm-client/main_test.go`. Create `src/client/clienttest/clienttest.go` and `src/client/clienttest/clienttest_test.go`.
- **Needs and parallel work.** Needs item 8 and the Sprint 1 command stub. It is the last build item. The orchestrator can write combined scenarios against the signatures below while the earlier items build.
- **Area, complexity and scope.** `backend`, low, `client/backend`. It delegates to existing application operations and maps command exits.
- **Statements.** D2, D16, R10 and C5 to C11. The design section is "The client in wave 1".
- Preserve `var version` and `run(args []string, stdout, stderr io.Writer) int` for Sprint 1 and the installer. Add `runContext(ctx context.Context, args []string, stdout, stderr io.Writer) int`. In `main`, use a context canceled by interrupt or termination and release its signal registration before calling `os.Exit`. `version` still prints the injected version and exits 0. `join <address> <token>` delegates to `app.Join`, returns 0 on success and 1 on error. `run` has no positional arguments and uses the app's exit status. Missing, unknown or extra arguments print a fixed usage line and exit 2. Usage never echoes arguments.
- **Exactly three exports in `src/client/clienttest`.** `type Options struct { StateDir string; Version string; Getenv func(string) string; Stderr io.Writer }`, `func Join(ctx context.Context, opts Options, address, token string) error` and `func Run(ctx context.Context, opts Options) int`. The package imports only its own program's application entry and standard-library types. It requires a nonempty absolute disposable `StateDir`. `Join` returns an error on invalid state-directory input. `Run` returns 1 on invalid input. `Getenv` may be supplied to isolate the home and executable search path without changing process-wide environment. `Run` blocks until it closes the folder and returns the same status as the command. An external test starts the server first, joins through the actual endpoint, starts `Run` on a goroutine and cancels and joins it before cleanup. It can observe state through the real server and fixed logs. The package exports no credential, mutable store, fake transport, private type or process signal hook.
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

**Rollout detail.** Step 3 delivers the executable and test entry. In step 4 run `go test -race -count=1 ./src/linktest/...` with `LGTM_TEST_POSTGRES` pointing at real disposable PostgreSQL. Then run `deploy/test/checks.sh` and require 13 `ok` lines and status 0. Run on a host with Docker and local network access, as issue 1640 specifies. A runner without them cannot supply the stack evidence.

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

Sources are the client-link assignments, the epic, the approved foundation design, issues 1632 through 1642, phase 4 of the build loop, the current prerequisite plans, their fold requests, and the wave 1 user-flow script.

| Source item | Workstream | Status |
|---|---|---|
| Task and brief, enrollment and issue 1636 | WS-1, WS-3 | Items 1, 2 and 9 carry the command. The result requests the corresponding website copy and install step. |
| Task and brief, proof, retry, reports and removal | WS-2 | Items 3, 4, 5 and 7 cover the two keys, the replay sender, the report producer and bounded reconnect. |
| Task and brief, wave 1 commands and foreground restart | WS-3 | Items 6, 8 and 9 fix the receipt rules and exit status. |
| Task, issue 1634 and epic, in-process tests | WS-3 | Item 9 supplies the client entry. The Server team and orchestrator own the server entry and combined suite. |
| Task, wave 1 tests and reconciliation requests | WS-1, WS-2, WS-3 | The result maps the requested changes and the unresolved retention assumption. |
| Epic, file ownership, test timing and prerequisite interfaces | WS-1, WS-2, WS-3 | Scope rows keep edits under `src/client/`. Rollout 1 obtains agreement before tests and implementation. |
| D1 and D2, the product boundary and Go rewrite | WS-1, WS-2, WS-3 | All network work uses generated types. Items 8 and 9 wire the Go program. |
| D3 to D5, the server database and feed | Outside client scope | The server plans own these. The client consumes the contract only. |
| D6, steering running agents | Later wave | Agent execution is outside wave 1. |
| D7 and D17, the key exchange and trust | WS-1, WS-2 | Items 1 to 3 check the certificate, public key and proof. |
| D8, the agent protocol | WS-2 | Item 5 reads harness versions. Driving an agent is later work. |
| D9, D10 and D11, tool secrets, vault and web sign-in | Outside client scope | The client uses its proof. The server and stack own the vault and website sign-in. |
| D12 to D15, memory sync, lost machines, harness accounts and product routing | Later wave | The connection token stays session-local. |
| D16, outbound connection, retry and restart | WS-2, WS-3 | Items 3, 6, 7 and 9. |
| D18, D20 to D23, later agent behavior and boundaries | Later wave | The foundation design schedules these after the link. |
| D19, the machine user's client state and repositories | WS-2, WS-3 | Items 5 and 8 read the resolved user settings. |
| R1 and R2, ownership and frozen contract | WS-1, WS-2, WS-3 | Rollout 1 and the guardrails. |
| R3 to R5 and R7, database ownership, writes and watch | Outside client scope | The client calls neither these packages nor their tables. |
| R6, durable commands | WS-2, WS-3 | Items 6 and 7 consume only commands received on the link. |
| R8, stable codes | WS-1, WS-2 | Items 2 and 3 interpret the agreed Connect codes and map fixed diagnostics. |
| R9, secrets and agent text | WS-1, WS-2, WS-3 | Transport, sanitized reports, outcome-only results and redaction tests. |
| R10, real server and PostgreSQL for integration | WS-1, WS-2, WS-3 | Combined tests run both real programs in one process. |
| R11, third-party credit | WS-1, WS-2, WS-3 | No copied implementation is planned. The repository credit rule still applies. |
| C1 to C4 and C13, stack, sign-in and vault | Outside client scope | The combined sprint still runs all 13 wave 1 checks. |
| C5 and C6, token and pending approval | WS-1, WS-3 | `TestJoinReplay` and the C5 and C6 user-flow steps. |
| C7 to C9, reports, presence and server restart | WS-2, WS-3 | `TestPresenceAcrossRestart` and the C7 through C9 user-flow steps. |
| C10 and C11, wrong keys and replay | WS-1, WS-2, WS-3 | `TestKeysAndRemoval`, `TestOutboxReplayAfterServerFailure` and the C10 and C11 user-flow steps. |
| C12, all packages and feed-order probe | WS-3 | Run repository Go tests as part of phase 5. The feed mutation belongs to the Server team. |
| Issue 1632, open foundation questions | WS-1, WS-2, WS-3 | Decisions 7 to 24 resolve the client slice or request agreement. |
| Issue 1633, encryption boundary | WS-1, WS-2 | Both server identities, the leaf-key digest, and the private hop are recorded in decision 8. |
| Issue 1635, sizing | WS-1, WS-2, WS-3 | Nine low or medium items with first tests and dependencies. |
| Issues 1637 and 1638, machine address and host role | WS-1 | Decision 23 and the Server and Stack team requests. |
| Issue 1639, key replacement | WS-1, WS-2 | Decision 18 and the mismatched-key acceptance observations. |
| Issue 1640, runner access | WS-3 | Phase 5 uses a development runner with Docker and local network access. |
| Build loop phase 4, tests first, mutation and exact-commit merge | WS-1, WS-2, WS-3 | The shared build context and each item's tests, mutation and acceptance. |
| Sprint 0 public messages, proof and results | WS-1, WS-2, WS-3 | Items 1 to 7 use the drafted messages. Rollout step 1 names the shared freeze gate. |
| Sprint 1 local state | WS-1, WS-2, WS-3 | Items 2 to 8 use the state, scan and transactional store interfaces. |
| Sprint 1 server store and vault, Sprint 2 sign-in and web | WS-1, WS-2 | The generated link is the client boundary. The shared build context names the server test entry. |
| Issues 1641 and 1642 | Outside wave 1 | Item 4 uses the wire sequence and its local bounds. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repository or assignment rule | How this plan satisfies it |
|---|---|
| `AGENTS.md`, "Where things are" and the epic's Client scope | Build changes are under `src/client/`. The generated contract supplies the network boundary. Changes to the floorplan, server, stack, test script and website go to their owners through the result. |
| `AGENTS.md`, habit 1, credit copied code | The plan uses existing repository primitives and Go interfaces, including the pinned dependency for elapsed time. Any implementation copied from another project needs its third-party directory, license and notice entry through the owning team. |
| `AGENTS.md`, habit 2, write down what was found | The traceability table names the scoped issues. The result records compatibility problems for the orchestrator. |
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
| 7. Connection supervision | backend | client/backend | medium | Composes four independently testable pieces and owns their lifetimes. | Yes | 3, 4, 5 and 6. |
| 8. Application lifecycle | backend | client/backend | medium | Opens the local resources and orders their teardown through one call. | Yes | 2, 7, Sprint 1 state and config. |
| 9. Command and test entry | backend | client/backend | low | Thin entry points reuse the application lifecycle. | Yes | 8 and the Sprint 1 command stub. |

Load for build and code checks. The lead needs nine test-writer starts, nine builder starts and eighteen review starts if each item gets its two code reviews. The epic estimates a separate 95 plan-review starts across nine plans. With alternating builders and an independent test writer from the other primary vendor, each primary vendor gets nine build or test starts. With one review from the other primary vendor and one from the third vendor per item, the primary vendors get five and four code review starts and the third gets nine. The build and code-review allocation is therefore 14, 13 and 9. For plan review, the epic allocates four seats to the other primary vendor, one to the writing vendor and one to the third vendor per full round. The orchestrator checks these estimates against the remaining allowances at launch. This bounded assignment has no live allowance readings.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (B/M/m) | Outcome |
|---|---|---|
| Plan ready for round 1 | Pending | The prerequisite reconciliation is ready for review. The orchestrator creates the review rounds and folds their findings. |

</details>
