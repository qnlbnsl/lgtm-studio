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

The client records which commands it has started or finished, including a scan or restart requested from the site. That record lets it answer retries without repeating completed work. Before the build, the teams need to agree on what a waiting connection may send and how the server reports token refusal. The plan carries those requests with the handoff.

## Key decisions

A machine checks two identities. It checks the public key of the certificate presented by the stack before sending an enrollment token over an encrypted connection. It also checks the server's separate signing key when it joins and every time it connects. A certificate renewal can keep the same key. Replacing either pinned key requires a new enrollment.

The Machines page and the installer use `lgtm-client join <address> <token>`. The address must agree with the address inside the token. The positional token can appear in shell history and process arguments. A successful join writes the enrollment before returning and leaves the machine waiting for the owner.

An approved machine saves reports before sending them and keeps them until the server acknowledges them. A stored command result survives a dropped connection. If the process dies during a command, the next process reports the interruption and keeps that action from running a second time under the same identifier.

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

1. Freeze the enrollment and link names with the Contract and Server teams, and merge the Sprint 1 local-state interfaces.
2. Build origin checks, enrollment and stream proof. Build the report and receipt paths against the agreed types.
3. Connect the report and command paths to the client process. Then connect the command line and the in-process test entry to that same process.
4. Run the combined tests with a real server and database. Run the wave 1 stack checks and paste the Machines page's command once on a disposable machine.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**Shared build context**

The root Go module is `github.com/qnlbnsl/lgtm-studio`, with Go 1.26 or newer. The wire types are generated from Protocol Buffers in `src/contracts/gen/go/lgtm/link/v1` as `linkv1), with the Connect bindings in `linkv1connect`. The service methods are `LinkService.Join` and `LinkService.Open`. Their paths are `/lgtm.link.v1.LinkService/Join` and `/lgtm.link.v1.LinkService/Open`.

The wire names below come from the Sprint 0 plan and the second-round draft. D numbers refer to the design decisions, R numbers to the design rules and C numbers to the wave 1 checks, in each case in source order. Requested additions are called out as dependencies. In particular, the stable refusal metadata and the pruning clock for the receipt store must be settled before their consumers build. The orchestrator compares the plan texts before freezing tests.

The command uses `config.StateDir`, whose precedence is `LGTM_CLIENT_STATE_DIR`, then the X Desktop Group (XDG) state-directory environment variable `XDG_STATE_HOME` with child `lgtm-client`, then the default in the user's local state folder. It uses `state.Open` on that resolved folder. It keeps the file lock until after it closes the embedded store. The store is the Sprint 1 `link.Store` backed by `go.etcd.io/bbolt` in `client.db`. It holds both the outbox and the command receipts. The design supplies the scope of the work. It covers the wave 1 machine link through approval, reporting, the three command actions and removal.

The test writer freezes each named test against the design and this plan before the builder starts. If a symbol is greenfield, a compiling stub for that item returns zero, false or an empty value. The first run must fail on the assertion described here. Risky tests must also fail after the named code mutation on a temporary branch. The lead checks the exact immutable commit and checks the temporary merge again. The combined tests are owned by the orchestrator under `src/linktest/`; isolated listeners in client tests check framing and failure boundaries, and the combined tests use the real programs and a real PostgreSQL.

**WS-1: A machine joins the server it was given**

The first two items establish which endpoint can receive the token and when the machine can regard itself as enrolled.

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
| 2 | Decode, send and commit enrollment. | `src/client/internal/enroll/join.go`, `Join`, `LoadServer`. |

**Item 1. Origin and transport**

- **Files.** Create `src/client/internal/transport/transport.go` and `src/client/internal/transport/transport_test.go`.
- **Needs and parallel work.** Needs the frozen Sprint 0 origin and certificate-pin rules. It can run beside item 5.
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
- **Needs and parallel work.** Needs item 1, `go-s1-client-local` items 2 and 3, and the frozen Sprint 0 enrollment contract. It can run beside item 5. Item 3 consumes the saved enrollment.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. One unary call uses the local plan's key and atomic write primitives.
- **Statements.** D7, D17, R8, R9 and C5, C6 and C10. The design section is "Joining: a token that authorizes an exchange of keys".
- **Public names.** `type Server struct { Address string; MachineID string; ClientPublicKey ed25519.PublicKey; PublicKey ed25519.PublicKey; CertificateKeySHA256 []byte }`, `func LoadServer(f *state.Folder) (Server, error)` and `func Join(ctx context.Context, f *state.Folder, address, token, hostName string) (Server, error)`. `ErrEnrollmentExists`, `ErrEnrollmentMissing` and `ErrEnrollmentInvalid` distinguish local conditions. `type JoinError struct { Code string }` implements `error` with fixed messages. It never embeds an upstream message or the token.
- Decode the `lgtm1_` prefix and unpadded base64url text to `JoinToken.token_id`, `secret`, `address`, `server_key_sha256` and `certificate_key_sha256`. Check the canonical lowercase universally unique identifier (UUID), the lengths of hashes and secret, and the destination policy. Normalize and compare both origins before any network activity, then use the token origin. Cap the encoded input at 4 kibibytes and each attempt at 10 seconds. Stop retrying 15 minutes after the first attempt, measured by the monotonic clock. The server alone judges expiry and can refuse an expired token on the first attempt.
- First refuse an existing `f.Paths().Identity`, including an invalid file. Load the key with `enroll.Load`. Create it only on `ErrKeyMissing`. On `state.ErrPlacedNotSynced` from creation, load the same key and require a successful `state.SyncDir` before sending its public half. A new process with a previously pending key also confirms the directory sync before the first attempt. A key read error stops enrollment.
- Use `linkv1connect.NewLinkServiceClient` to send `JoinRequest.token_id`, `token_secret`, `machine_public_key` and `host_name`. Validate the host name as UTF-8 (Unicode Transformation Format, 8-bit) and bound it to 255 bytes at a character boundary. The join can use an empty host name if the local name was unavailable. A lost reply retries the same key and token. Retry transient codes at 1, 2, 4, 8 and at most 15 seconds while the context and the overall attempt window allow it. Stop on `invalid_argument`, `permission_denied`, a transport pin mismatch or an invalid successful reply. Map only the proposed reason values `token_used` and `token_expired` from `Lgtm-Join-Refusal` on `permission_denied`. Other refused tokens use `token_refused`. A malformed request, wrong pin and bad signing key use `invalid_argument`, `certificate_key_mismatch` and `server_key_mismatch` respectively.
- Validate `JoinResponse.machine_id` and its exact 32-byte `server_public_key`. Check the SHA-256 digest of the raw key against the token using constant-time comparison. Write `identity.json` last with `state.WriteAtomic` in the locked folder. Its JavaScript Object Notation (JSON) fields are `version` equal to 1, `address`, `machineId`, `clientPublicKey`, `serverPublicKey` and `certificateKeySHA256` when applicable. The byte strings use padded standard base64, and the file mode is `0600`. `LoadServer` reads a bounded regular file owned by the current user, without following links, and validates the fields, version, permissions, origin and key lengths. The client public key equals the credential the join loaded. The run path verifies that public half against the private key before dialing. It treats unknown fields and trailing content as invalid. It never reads a secret from this file. A placed but unsynced `identity.json` is only reported as successful after a validated reread and successful directory sync. An error after placement keeps the file and key for inspection.
- **Risky.** Yes, for credentials and crash ordering. Move the reply-key comparison after the write, or generate a fresh key after an unavailable response.
- **Tests written first in `join_test.go`.** `TestJoinCommitVerifiedEnrollment` asserts that a valid response writes mode `0600` with exactly the named fields, and that a malformed identifier or wrong key leaves it absent and keeps the machine key. `TestJoinReusesDurableKey` drops one reply, verifies the credential can be loaded from disk before the first request arrives, verifies both request public keys match, and verifies a second join with a committed file makes no request. `TestJoinRejectsAddressAndRedirect` observes no post to a mismatched origin or redirect target. `TestJoinRejectsExpiredAndRedactsRefusal` checks that the server's expiry refusal stops the attempt loop, that an unavailable server gets no attempts after the 15-minute window, and that the two stable refusal codes survive and excludes marked token text, secret bytes and injected remote text from returned errors. `TestLoadServerRejectsUnsafeFile` covers a symlink, incorrect mode, unsupported version, invalid key length and a missing file. `TestJoinSyncFailureKeepsKey` injects the Sprint 1 directory-sync failure and asserts that no request leaves until the key placement has been synced.
- **Acceptance.** `go test ./src/client/internal/enroll` passes all six tests and the credential tests from Sprint 1.

**Design artifact.** The approved foundation design, "Joining: a token that authorizes an exchange of keys". The client plan changes no screen hierarchy.

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| The client discloses a usable join token to another endpoint. | High | Validate the origin and leaf pin before writing the request. Reject redirects and log only fixed reasons. |
| A join succeeds on the server but the reply is lost. | High | Save the private key first. Replay with the same key and check that the server makes only one waiting record. |
| An owner copies a token into a shell that keeps its history. | Medium | Keep the existing positional agreement for wave 1. The handoff records command arguments and shell history as exposure surfaces for the orchestrator's pending contract decision. |

**Verify.** Run the tests for items 1 and 2 and the combined `TestJoinReplay`. Environment check is C5 and C6 through the local stack plus a pasted command from the real Machines page on a disposable machine. The stack also checks an HTTPS enrollment through its public web endpoint with a self-signed leaf.

**Rollout detail.** Steps 1, 2 and 4 freeze names, build the command and run the observations. Build the command from `./src/client/cmd/lgtm-client`. Run `lgtm-client join <address> <token>` with the address issued for that disposable stack.

**WS-2: A machine proves itself and sends its reports in order**

The client proves the server before accepting anything on the stream. It sends reports saved on previous connections before sending newer reports. The run loop keeps transport interruptions from becoming a new enrollment.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 2a | A fresh challenge proves both ends. An invalid server proof sends no machine proof. Only a provable unknown-machine refusal stops the retry. | `TestOpenProofFixture`, `TestOpenRejectsWrongProofAndOrder`, `TestRetryStopsForVerifiedRemoval`, C10. |
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

- **Files.** Create `src/client/internal/link/conn.go` and `src/client/internal/link/conn_test.go`.
- **Needs and parallel work.** Needs items 1 and 2, Sprint 1 `Credential` and the frozen `LinkService.Open`. It can run beside item 5.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. It handles three proof messages on a generated stream and verifies the golden fixture.
- **Statements.** D7, D16, D17, R9, R10 and C8 to C11. The design sections are "One connection, dialled by the machine" and "Both ends prove themselves on every connection".
- **Names shared inside the package.** `openSession` takes a context, the generated `LinkServiceClient`, `enroll.Server` and `*enroll.Credential`. It returns a `session` or a classified error. A `session` owns one bidirectional stream, cancellation, the validated `*linkv1.Welcome`, `proofSent` and `welcomeReceived` monotonic timestamps. It exposes `send`, `receive` and `close` within the package. `proofText` builds the two signed inputs. Internal errors distinguish `errServerKeyMismatch` and a terminal link refusal carrying one fixed reason.
- Start each attempt with a fresh cryptographically random 32-byte nonce. Send `OpenRequest.hello` with `Hello.machine_id`, `machine_nonce`, and `link_revision` equal to 1. Under the second-round ruling, the client sends no event mark or epoch in `Hello`. Accept `OpenResponse.challenge` with exact 32-byte `server_nonce` and 64-byte `server_signature`. Verify Ed25519 using the key returned by `LoadServer`. The signed bytes use the literal label `lgtm-link-v1 server` or `lgtm-link-v1 machine`, followed by a zero byte, one byte for the identifier length, the identifier bytes, the machine nonce and the server nonce. When the server signature verifies, send `OpenRequest.proof` with `Proof.machine_signature` from `Credential.Sign`. Take the monotonic timestamp just before sending it.
- Wait for `OpenResponse.welcome`. Check `Welcome.link_revision`, `state` and `last_event_number` according to the frozen contract. Require `Welcome.server_time`, sampled after verification of the proof, for the existing receipt-clock rule. The current draft assigns it field 4 and assigns `link_revision` field 5. Apply the standing before the session is available to the command and report loop. Treat a missing or unknown approval as waiting under the contract's fail-closed rule. Keep `Welcome.connection_token` in the session only. A missing token for waiting is accepted under the requested waiting rule. The client makes no side calls in wave 1.
- Give each exchange 10 seconds, including a blocked send. Cancel the attempt to unblock the stream on timeout. Accept only the prescribed frame for each handshake step. An `unauthenticated` status is retryable. A `failed_precondition` status before a verified challenge is retryable. After a verified challenge it becomes a terminal refusal. The proposed `Lgtm-Link-Refusal` values `unknown_machine`, `link_revision_mismatch` and `event_gap` allow the client to choose a fixed diagnostic. An unrecognized or missing value selects `link_precondition_failed`. An incompatible authenticated `Welcome.link_revision` has the same terminal effect as a revision refusal. A bad signature becomes `errServerKeyMismatch` and sends no proof. A broken stream and `aborted` are retryable.
- **Risky.** Yes, for key proof and message order. Swap the two nonces in one proof, accept a bad server signature, or treat an early unauthenticated status as a terminal refusal.
- **Tests written first in `conn_test.go`.** `TestOpenProofFixture` fixes both proof byte strings and signatures against `src/contracts/testdata/link-proof.json` with its specified keys and nonces, then checks the exact observed `Hello`, `Proof` and validated `Welcome`. A separate attempt has a different machine nonce. `TestOpenRejectsWrongProofAndOrder` sends an invalid signature, a signature from the last connection and a wrong message kind, and observes no machine proof. `TestOpenExpiryAndRefusal` leaves each handshake stage unanswered and asserts cancellation at the 10-second bound using a test clock attached to the same step deadline. It asserts a retry for unauthenticated and for failed precondition before proof. It asserts a terminal refusal for failed precondition after the verified challenge and for an authenticated revision mismatch.
- **Acceptance.** The three tests pass in `go test ./src/client/internal/link -run 'TestOpen'`.

**Item 4. Durable event replay**

- **Files.** Create `src/client/internal/link/replay.go` and `src/client/internal/link/replay_test.go`.
- **Needs and parallel work.** Needs the Sprint 1 `Store` and the frozen server mark rules. Runs beside items 3 and 5 after it has the report payload agreement below. Item 7 binds it to a session.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. It reads one existing queue through an ordered cursor and validates the acknowledgements.
- **Statements.** R6, R9, R10, C9 and C11, and the design section "Events come up, and none is lost or applied twice".
- **Names shared inside the package.** `replay` has `reconcile(serverLast uint64) error`, `next() (*linkv1.Event, error)`, `markSending(number uint64) error`, `sent(number uint64)` and `ack(through uint64) error`. `encodeReport(*linkv1.Event) ([]byte, error)` and `decodeReport([]byte) (*linkv1.Event, error)` are used by item 5 and by this item.
- The payload passed to `Store.Append` starts with byte 1. The rest is the current binary `Event` with number unset, `happened_at` set at capture and one `facts` or `harnesses_found` report. The writer never patches a stored record. The replay path gives `Event.number` the value of the row's `Event.Seq` and wraps it in `OpenRequest.event`. A bad payload stops delivery with a local diagnostic and keeps the store file.
- On a verified `Welcome`, call `Store.Reconcile(Welcome.last_event_number)` once before offering events. That mark is the server's committed prefix for this machine. Only the server decides it. Below the local acknowledged mark, `ErrServerBehind` ends the run as a terminal link refusal with `event_mark_regressed` and leaves the store intact. Restoring the server database to an older state is outside wave 1. From the acknowledged through the durably recorded sent mark, `Reconcile` discards only the confirmed prefix. Above the sent mark, it preserves unsent payloads and moves them to follow the server mark, as the Sprint 1 store specifies for a new or restored local store. Log the number moved when nonzero. On a ceiling or store error, leave the remaining reports on disk.
- Use one stream writer. Call `Store.MarkSent(number)` just before attempting each event send. The persistent mark is an upper bound on events the server could have read across a crash. A successful send raises the per-stream sent ceiling, initially the welcome mark. Serialize that update with `EventAck` handling so a fast acknowledgement cannot beat its send bookkeeping. Repeated lower marks are harmless. A mark above the ceiling is a protocol fault and cannot reach `Store.Ack`, even if it names a locally queued event. Advance the send cursor after a successful send. If the connection ends, discard the cursor and rebuild it from the next authenticated `Welcome`. Use the outbox's bounded reads and `Changed()` to wake the sender. Give waiting heartbeat and command-result frames priority between event sends.
- **Risky.** Yes, for sequence and data retention. Allow an acknowledgement above the sent ceiling to reach `Store.Ack`, or start at the last sent number after reconnect.
- **Tests written first in `replay_test.go`.** `TestReplayPreservesSequenceAndCaptureTime` queues three reports with fixed timestamps, sends two, acknowledges one, reopens at the first mark, and asserts that the remaining wire numbers and timestamps match the saved reports. `TestReplayRejectsImpossibleMarks` asserts that a mark for an unsent row and a welcome behind the local mark change nothing. It checks that an immediate valid acknowledgement after send is accepted. `TestReplayReconcilesToServerPrefix` starts with sent events 1 and 2 and unsent events 3 and 4. A welcome at 7 discards the confirmed prefix, retains the unsent payloads as 8 and 9 in order, logs a move count of 2, and sends 8 first. During an ordinary reconnect, it verifies that a committed but unacknowledged event is removed and a sent but uncommitted event is replayed under its original number. `TestReplayRejectsDamagedPayload` asserts that a bad version and bad body each leave the original file pending and permit no later send.
- **Acceptance.** All four pass in `go test ./src/client/internal/link -run 'TestReplay'`.

**Item 5. Reports and heartbeat contents**

- **Files.** Create `src/client/internal/link/report.go` and `src/client/internal/link/report_test.go`.
- **Needs and parallel work.** Needs Sprint 1 `config.Settings`, `sysinfo.ReadFacts`, `sysinfo.ReadLoad`, `harness.Scan` and `Store.Append`. Runs beside items 1 to 4 using the payload signature specified in item 4. A test writer can stub that interface on its branch until item 4 merges. Integrates after item 4.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. It maps existing facts and inventory into two event types and a heartbeat.
- **Statements.** D8, D16, D19, R9, C7 and C11, and the design sections "What travels in wave 1" and "The client in wave 1".
- **Names shared inside the package.** `reporter` takes the Sprint 1 settings, client version, scan options, outbox and a JSON logger. It supplies `captureFacts`, `scanHarnesses`, `heartbeat` and a durable append operation to items 6 and 7. Collection functions accept context for scan cancellation. The producer timestamps and validates the report against the event contract before it offers it to the outbox.
- Send `Facts.os`, `cores`, `memory_bytes`, `client_version`, `workspace` and `agent_limit`. Use the resolved settings, with the user's home portion abbreviated to `~` when possible. Leave the workspace field empty when it exceeds 256 bytes after abbreviation, as the local-state plan specifies. Keep the other strings within their wire limits at Unicode character boundaries. Keep core count positive and preserve bytes for the server's units conversion. The inventory has all four local scan entries in the order defined by `harness.Definitions()`. Send `HarnessesFound.harnesses` with each `Harness.name` and optional `version` when known. Use the existing scan and its trust checks. The report contains no raw process output, executable path or diagnostic detail.
- `Heartbeat.cpu_percent` and `memory_percent` are optional integers between 0 and 100. Prefer absence over a made-up measurement. When the local probe returns an error, send a heartbeat with both fields absent. The supervisor sends an immediate heartbeat after `Welcome` and every 10 seconds after that. While waiting, it sends the heartbeat with both fields absent. A heartbeat never enters the outbox.
- When `Append` succeeds, log `msg=link_event_buffered` and `sequence` as that positive numeric sequence. On `ErrOutboxFull`, no number was taken. Keep the newest refused facts and automatic inventory snapshots of each kind and retry when an acknowledgement frees room. A requested scan returns a failure when its report cannot be durably appended. Serialize append and the associated log write so their observations keep commit order. Any error wrapping `ErrStoreFailed` cancels this run with status 1. The next process opens the store afresh and appends new observations. It does not guess whether the failed commit saved a report.
- **Risky.** Yes, for ordering and disclosure. Move the log before the append returns, or use `Found.Path` as a version.
- **Tests written first in `report_test.go`.** `TestReportInventoryAndLoad` supplies facts, settings and four scan entries, including unknown versions and marked paths. It checks the six facts fields, four names, optional versions and no marked strings. It checks present zero, measured nonzero and absent load values without issuing an outbox append. `TestReportLogFollowsDurableAppend` pauses the Sprint 1 transaction commit return using the store test hook and observes no log until the append returns. `TestReportBackpressure` fills a one-entry outbox, confirms no extra sequence or log, frees the entry and checks that only the most recent refused facts append. `TestReportUncertainAppendIsNotRepeated` injects an ambiguous commit failure, asserts no success log and no further append in that run, and reopens to verify that any committed payload keeps its sequence.
- **Acceptance.** The four tests pass in `go test ./src/client/internal/link -run 'TestReport'`.

**Item 7. Connection supervision and report timing**

- **Files.** Create `src/client/internal/link/retry.go`, `src/client/internal/link/session.go` and `src/client/internal/link/retry_test.go`.
- **Needs and parallel work.** Needs items 3 to 6. Item 8 can prepare its tests alongside this item using the public names below.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. It composes the proof, queue, reporter and command handler that the earlier items test independently.
- **Statements.** D16, R6, R9, R10 and C7 to C11. The design sections are "Knowing whether a machine is there", "What travels in wave 1" and "The client in wave 1".
- **Public names.** `type RunOptions struct { Server enroll.Server; Credential *enroll.Credential; Folder *state.Folder; Settings config.Settings; ScanOptions harness.ScanOptions; Version string; Logger *slog.Logger }` and `func Run(ctx context.Context, opts RunOptions) (RunOutcome, error)`. `type RunOutcome uint8` with `RunStopped` equal to 0 and `RunRestart` equal to 1. A non-nil error takes precedence over the outcome. The caller owns the open folder. `Run` opens and closes `Store` with that folder and constructs the pinned client. It dials the configured server and never opens a listening port. It returns after all workers have stopped.
- Retry on a broken connection, an unverified refusal, a signing-key mismatch, a certificate mismatch, stream replacement or server unavailability. After a failed or lost connection, wait 1, 2, 4, 8 and then at most 15 seconds between attempts. Reset after 30 seconds on a valid stream. A user cancellation interrupts an attempt or wait. Each attempt closes its sender and receiver before the next starts. If `Send` reports a closed stream before its status is known, collect the terminal status from the receive side with the same deadline before classifying the result. A `failed_precondition` after the verified challenge ends with `RunStopped` and one `link_refused` line with the classified code. This applies during the handshake or after the stream opens. A removed or unknown machine uses `code=unknown_machine`. The sentence directs the owner to remove the old entry if present and enroll again from a fresh folder. A bad machine signature given as `unauthenticated` retries. A signature mismatch emits `link_refused` with `code=server_key_mismatch`. Log an unchanged failure reason once until it changes or the link recovers. A verified revision refusal stops with the fixed revision diagnostic. For a replaced stream, log that another process may be using the key and cap the retry wait at 15 seconds. The result asks Sprint 0 to remove its draft exception for repeated replacements to keep the assignment's 15-second bound. A state of removed on the current proved session also stops with a re-enrollment message. Broken or superseded sessions do not delete local state.
- Keep one sender and one receiver per session. Route incoming frames through a bounded queue to the session owner, which orders command acceptance, event acknowledgements and the active standing. Unknown post-welcome frame kinds are ignored. Known invalid frames or contradictory state for the same revision close the session. Cancel the session after 30 seconds without a valid response. An independent timer can cancel a blocked send or receive. Only the current session advances the replay cursor or submits commands. The process owns the asynchronous scan so it can finish across an ordinary reconnect. Session cancellation closes the network work, and process cancellation also cancels and settles the local scan. Wait for the current workers and close the store on controlled exit.
- Before any work on a session, apply its `Welcome.state`, reconcile the report queue and set the server clock. While waiting, send only heartbeats without load and the result of a `SetState` command. It can take approval on the same stream. After an approved `Welcome` or a transition to approved, enqueue current `Facts` and one freshly collected `HarnessesFound`. The initial inventory scan runs independently of the heartbeat timer. Start one facts capture on a detected outage for a process that was approved, then one every 10 seconds until a new session establishes its standing. The same producer runs across all intervening retries. A process that has never seen approval sends and queues no reports while disconnected. After reconnection, send the kept prefix before newly numbered reports. If the server still says waiting, keep the queue on disk until approval. Use `Event.happened_at` as the capture time and preserve it on replay. The wire sequence governs application order even if the machine wall clock changes.
- A corrupt or unsafe `client.db` terminates with a local error that names the file. A failed transaction also terminates with a local error. Neither path moves or replaces the file. One runtime supervises the single store. Pending transient transport failures leave both stored reports and receipts intact.
- **Risky.** Yes, for connection and producer ordering. Remove the 15-second cap, treat an unsigned refusal as removal or start the report producer while waiting.
- **Tests written first in `retry_test.go`.** `TestRetryBackoffAndSilence` uses virtual time to assert 1, 2, 4, 8, 15 and 15-second waits, cancellation at 30 seconds of silence, the stable-session reset and no overlapping stream after cancellation. `TestRetryStopsForVerifiedRemoval` asserts a retry for a pre-proof refusal, a retry and one mismatch log for a wrong signature, and a stopped outcome for a verified unknown machine, a regressed server mark or `APPROVAL_REMOVED`. It repeats stream replacement within one minute and asserts that each wait stays within 15 seconds. `TestRetryReportsOnApprovalAndOutage` starts waiting, sees only bare heartbeats, then observes the six facts fields and four harness entries on approval, three new durable facts during an outage, and replay before any later report. `TestRetryFailureExitsAndKeepsStore` uses a corrupt store and an injected failed commit in separate cases. It checks preservation of the file and absence of a new send from the failed run.
- **Acceptance.** `go test -race ./src/client/internal/link -run 'TestRetry'` passes all four tests.

**Design artifact.** The approved foundation design, "The link between the server and a machine".

**Risks**

| Risk | Severity | Mitigation |
|---|---|---|
| A machine discloses its proof to an endpoint that holds a different signing key. | High | Check the golden signed texts and the negative assertions before sending any proof. |
| A guessed acknowledgement removes reports that the server never applied. | High | Update the local attempted-send watermark just before writing. Use the current session's successful-send ceiling for live acknowledgements. The verified server mark and store transaction govern reconciliation. |
| A stalled stream prevents a process from reconnecting or observing removal. | High | Bound proof exchanges and post-proof silence. Cancel blocked network work and distinguish terminal removal from transient errors. |
| An outage fills the local store. | Medium | The store refuses before numbering a report. Retain the newest refused snapshots and let acknowledgements free capacity. |

**Verify.** Run `go test -race ./src/client/internal/link`. The environment checks are C7 to C11 and the combined tests `TestPresenceAcrossRestart`, `TestKeysAndRemoval` and `TestOutboxReplayAfterServerFailure`. Verify a long open link through the stack web server using each supported transport, with ten minutes of heartbeats and no unplanned disconnect.

**Rollout detail.** Steps 2 and 3 build and join the pieces. In step 4 run `deploy/test/checks.sh --only C7,C8,C9,C10,C11` through the Stack team's disposable environment.

**WS-3: Commands survive disconnections without running again**

The command handler uses the receipt store for ordering. The shell entry and test entry use the same lifecycle so that a test can exercise the behavior of the deployed program.

**Done when**

| # | Criterion | Observed by |
|---|---|---|
| 3a | A newer attempt during a scan receives one result under the newer attempt, and a duplicate reads back the stored outcome. | `TestCommandLatestAttemptAndReplay` and `TestCommandAttemptsAcrossReconnect` in the combined suite. |
| 3b | Expired commands never start. A killed action comes back as interrupted. A requested restart saves its result before process exit and runs once on replay. | `TestCommandExpiryUsesServerTime`, `TestCommandRestartAndRecoveredReceipt`, `TestRestartReplaysAfterDroppedResult` and the installer check `live_reconnect_restarts_service`. |
| 3c | A fresh run reports how to join and sends no network request. A stopped client releases its lock. The service sees status 75 on a requested restart and status 0 on removal. | `TestAppRunRequiresEnrollment`, `TestAppRunClosesState`, `TestRunCommandExitStatus`, `TestClienttestSharesApp`. |

**Tasks**

| # | Task | Anchor |
|---|---|---|
| 6 | Handle approval state and bounded commands through receipts. | `src/client/internal/link/commands.go`, `commandHandler`, `serverClock`; Sprint 1 `Store.Begin` and `Store.Finish`. |
| 8 | Start and stop the process from shared application entry points. | `src/client/internal/app/client.go`, `Join` and `Run`. |
| 9 | Connect the command line and the in-process test entry. | `src/client/cmd/lgtm-client/main.go`, `runContext`; `src/client/clienttest/clienttest.go`, `Options`, `Join` and `Run`. |

**Item 6. Standing and command receipts**

- **Files.** Create `src/client/internal/link/commands.go` and `src/client/internal/link/commands_test.go`.
- **Needs and parallel work.** Needs Sprint 1 `Store.Begin`, `Store.Finish`, `Store.Get` and `Store.Prune`, the Sprint 0 commands and clock rules, item 3's welcome and timestamps, and the report interface of item 5. Runs alongside item 4 if it uses the agreed reporter interface.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. It maps six existing receipt states to the outcomes of three fixed actions, using store transactions for replay and expiry.
- **Statements.** D16, R6, R9, R10, C7 to C9. The design sections are "Commands go down, and none is lost or repeated" and "What travels in wave 1".
- **Names shared inside the package.** `commandHandler` owns the most recent `linkv1.MachineState`, the receipt store and at most one active requested scan. It accepts a validated `*linkv1.Command` tagged with the current session, applies a welcome before command handling and returns wire `*linkv1.CommandResult` messages to the session writer. `serverClock` publishes the latest authenticated sample taken from `Welcome.server_time`, `proofSent` and `welcomeReceived`. Its `now` function is the server time plus monotonic elapsed since proof send. Its `pruneNow` function is the server time plus monotonic elapsed since welcome receipt. The first is an upper estimate for expiry and the second a lower estimate for retention. The requested `StoreOptions.PruneNow` keeps those uses separate. Serialize publication and clock reads. Keep the last validated sample for finishing a scan after its stream goes down. Start the hourly prune only after a validated welcome, with valid server time. A backward machine wall clock never supplies either clock.
- Validate `Command.id`, `attempt` and `expires_at` before calling the store. Map these to Sprint 1 `Command{ID, Attempt, ExpiresAt}`. Apply `MachineState.approval`, `name` and `revision` only on an authenticated welcome or accepted `SetState.state`. A higher revision updates it. An identical same revision is a no-op. Different content for the same revision closes the session and keeps the last accepted state. A lower command revision does not change standing. An older welcome than one already applied in this run closes the session with a diagnostic. A fresh process gets its authority from a fresh welcome. Missing or unknown approval grants waiting privileges only. An authenticated `APPROVAL_REMOVED` stops further work and leads to permanent stop.
- Validate a new state and check its revision for conflicts before starting a `SetState` receipt. While waiting, accept only `SetState` and close the session on an inadmissible action. While approved, dispatch `Command.scan_harnesses`, `set_state` or `restart`. An unknown action in the approved state maps to `ReceiptUnsupported`. Call `Store.Begin` before doing any action. `DecisionReplay` sends the stored result using the arriving attempt. `DecisionRunning` sends nothing yet and uses the updated `LatestAttempt` from `Store.Finish` for the eventual result. `DecisionRun` allows one action. A begin error allows no action. The receipt's expiry remains the original server timestamp. Hold pending scan admission until a scan slot is ready, or end the stream and let the server deliver a distinct pending scan later if a second scan would exceed that bound. A newer attempt of the active identifier can join it immediately.
- For a scan, run the existing inventory scan under the process context. Write `ReceiptDone` only after its `HarnessesFound` event commits. A full buffer or a canceled scan writes `ReceiptFailed` with a fixed sentence. An automatic or later newly requested scan can produce a fresh report. For new standing, apply the state once and finish before replying. For restart, finish with `ReceiptDone` before offering the result. Give the result send a bounded attempt of at most two seconds. Then cancel the network, settle any running action, close the store and return `RunRestart`, whether or not the send arrived. A reply from a finished command goes only to a current stream that supplied the attempt it names. Otherwise the next delivery replays from the durable receipt. On controlled shutdown, cancel and finish active work before teardown. A failed store commit aborts the run and leaves recovery of an ambiguous receipt to `OpenStore`.
- Map `ReceiptDone`, `ReceiptFailed`, `ReceiptExpired`, `ReceiptUnsupported` and `ReceiptInterrupted` to `COMMAND_OUTCOME_DONE`, `COMMAND_OUTCOME_FAILED`, `COMMAND_OUTCOME_EXPIRED`, `COMMAND_OUTCOME_UNSUPPORTED` and `COMMAND_OUTCOME_INTERRUPTED`. A `Store.Finish` that returns `ErrResultTooLarge` with a failed receipt sends the returned failure. Set `CommandResult.command_id`, `attempt`, `outcome` and `message`. The message is empty or one fixed sentence up to 200 bytes. It carries no receipt diagnostic text or command output. Recovery of a started receipt yields interrupted and asks for a new command identifier if the owner needs the work done again.
- **Risky.** Yes, for ordering and expiry. Run a scan before `Begin` commits, use the machine's wall clock for expiry, or terminate a restart before `Finish` commits.
- **Tests written first in `commands_test.go`.** `TestCommandLatestAttemptAndReplay` holds scan attempt 1 open, delivers attempt 2 and a lower duplicate, and observes one scan, then a result for 2. A later attempt 3 returns the same outcome with 3 and starts no scan. It also breaks the stream before delivering 2 to check that the result cannot go to a stream with no matching attempt. `TestCommandExpiryUsesServerTime` varies the machine wall clock five minutes both ways, with server time and monotonic time fixed. A command strictly before expiry runs, one at expiry does not, and an expired replay of a finished command returns the saved result. A delayed welcome does not advance the pruning clock by the proof round trip. `TestCommandRestartAndRecoveredReceipt` holds the finish commit and asserts no send or restart yet, then releases it, drops the result and asserts restart followed by replay with no second restart. In a separate process, kill a scan after its start was committed and check that replay says interrupted and does no work. `TestCommandStateAndUnknownAction` covers waiting, approve, rename, old and conflicting revisions, removal and an unknown action. `TestCommandScanRequiresDurableReport` fills the store, then tests a successful scan after space is freed under a new identifier.
- **Acceptance.** `go test -race ./src/client/internal/link -run 'TestCommand'` passes the five tests.

**Item 8. Shared application lifecycle**

- **Files.** Create `src/client/internal/app/client.go` and `src/client/internal/app/client_test.go`.
- **Needs and parallel work.** Needs items 2 and 7, Sprint 1 configuration and state, and the installer exit-code agreement. Tests can be prepared beside item 7 using `link.Run`'s documented signature.
- **Area, complexity and scope.** `backend`, medium, `client/backend`. It sequences the private folder, credential, settings and lifecycle at the two entry points.
- **Statements.** D2, D7, D16, D19, R1, R9, R10, C6, C8 and C10.
- **Public names.** `type Options struct { StateDir string; Version string; Getenv func(string) string; Stderr io.Writer }`, `func Join(ctx context.Context, opts Options, address, token string) error` and `func Run(ctx context.Context, opts Options) int` in package `app`. The supplied state directory must be absolute. Empty uses `config.StateDir`. Nil `Getenv` uses `os.Getenv`. Empty version uses `0.0.0-dev`. Nil `Stderr` discards logs. The app writes only fixed diagnostics and local configuration values deliberately shown at startup. An upstream error is never written raw.
- `Join` resolves the folder, opens it under a lock, reads the host name and calls `enroll.Join`. It closes the folder before returning. On a refusal it emits one JSON object on standard error with `msg=join_refused`, `code` and a fixed sentence in `message`. On an expired or unusable token for a partial enrollment, the sentence tells the owner to remove the waiting record if present and use a new token with a fresh folder. A saved identity stays in its folder. The app never deletes the folder or its files.
- `Run` opens the locked folder, loads `identity.json`, syncs the directory before trusting it, then loads the key and checks the public half. A missing or invalid identity exits 1 with one instruction to join. It loads `settings.json` through `config.LoadSettings` and prepares the environment for `harness.DefaultScanOptions`. It logs the resolved state folder and repository folder once locally and calls `link.Run`. It maps canceled or removed to 0, requested restart to 75, and other errors to 1. It emits one restart sentence, including how to relaunch the foreground invocation, with `msg=link_restart_requested`. All paths close the folder after the store and workers finish. Use a synchronous JSON logger so a returned exit status implies its diagnostic was written.
- **Risky.** Yes, for trusting saved keys and lifecycle ordering. Dial before verifying the identity file, or release the folder on exit before the link workers stop.
- **Tests written first in `client_test.go`.** `TestAppRunRequiresEnrollment` checks that missing or malformed identity and a mismatched client public key each exit 1 before any dial. `TestAppRunClosesState` blocks the link, cancels the context, confirms the link settles before another process can open the state, and confirms the folder is usable after return. `TestAppLogRedactsSecrets` gives a remote refusal with marked text and checks a single stable JSON diagnostic with no marked token or message. `TestAppPreservesStateOnRemoval` checks a verified removal returns 0, leaves both keys and the store and releases the lock. `TestAppRefusesUnsyncedIdentity` forces a directory-sync failure and observes no dial.
- **Acceptance.** All five tests pass in `go test -race ./src/client/internal/app`.

**Item 9. Command line and in-process test entry**

- **Files.** Change `src/client/cmd/lgtm-client/main.go` and `src/client/cmd/lgtm-client/main_test.go`. Create `src/client/clienttest/clienttest.go` and `src/client/clienttest/clienttest_test.go`.
- **Needs and parallel work.** Needs item 8 and the Sprint 1 command stub. It is the last build item. The orchestrator can write combined scenarios against the signatures below while the earlier items build.
- **Area, complexity and scope.** `backend`, low, `client/backend`. It delegates to existing application operations and maps command exits.
- **Statements.** D2, D16, R10 and C5 to C11. The design section is "The client in wave 1".
- Preserve `var version` and `run(args []string, stdout, stderr io.Writer) int` for Sprint 1 and the installer. Add `runContext(ctx context.Context, args []string, stdout, stderr io.Writer) int`. In `main`, use a context canceled by interrupt or termination and release its signal registration before calling `os.Exit`. `version` still prints the injected version and exits 0. `join <address> <token>` delegates to `app.Join`, returns 0 on success and 1 on error. `run` has no positional arguments and uses the app's exit status. Missing, unknown or extra arguments print a fixed usage line and exit 2. Usage never echoes arguments.
- **Exactly three exports in `src/client/clienttest`.** `type Options struct { StateDir string; Version string; Getenv func(string) string; Stderr io.Writer }`, `func Join(ctx context.Context, opts Options, address, token string) error` and `func Run(ctx context.Context, opts Options) int`. The package imports only its own program's application entry and standard-library types. It requires a nonempty absolute disposable `StateDir`. `Join` returns an error on invalid state-directory input. `Run` returns 1 on invalid input. `Getenv` may be supplied to isolate the home and executable search path without changing process-wide environment. `Run` blocks until it closes the folder and returns the same status as the command. An external test starts the server first, joins through the actual endpoint, starts `Run` on a goroutine and cancels and joins it before cleanup. It can observe state through the real server and fixed logs. The package exports no credential, mutable store, fake transport, private type or process signal hook.
- **Risky.** Yes, because the command receives a secret and maps restart exits. Echo the supplied arguments on usage error or replace status 75 with 0.
- **Tests written first.** `TestRunCommandExitStatus` in `main_test.go` asserts version output, status 2 on bad arity without echoing the marked secret, status 1 for an unjoined run, and exact propagation of 0 and 75 from the app through the command dispatch. `TestClienttestSharesApp` in `clienttest_test.go`, in external package `clienttest_test`, enrolls into a disposable folder through the same endpoint as the command, sees the same diagnostics, verifies that the nonempty absolute path is required and confirms context cancellation releases the state lock. The stub test listener exercises the entry point. The combined tests exercise it with a real server and database.
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

The source universe is the client-link task, the initial client-link brief, the client scope and Sprint 3 checks in the Go rewrite epic, the foundation design and issues 1632 through 1640, the build loop's phase 4, the two prerequisite plans, the contract result and second-round ruling, and the client-facing expectations in the wave 1 test. Where a numbered design statement belongs to another team or wave, the mapping below names it.

| Source item | Workstream | Status |
|---|---|---|
| Task and brief, enrollment and issue 1636 | WS-1, WS-3 | Items 1, 2 and 9 carry the command. The result requests the corresponding website copy and install step. |
| Task and brief, proof, retry, reports and removal | WS-2 | Items 3, 4, 5 and 7 cover the two keys, the replay sender, the report producer and bounded reconnect. |
| Task and brief, wave 1 commands and foreground restart | WS-3 | Items 6, 8 and 9 fix the receipt rules and exit status. |
| Task, issue 1634 and epic, in-process tests | WS-3 | Item 9 supplies the client entry. The Server team and orchestrator own the server entry and combined suite. |
| Task, wave 1 test names and requests from the contract result | WS-1, WS-2, WS-3 | The result lists each client-facing name and the resolution of the four Client team requests. |
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
| R9, secrets and agent text | WS-1, WS-2, WS-3 | Transport, sanitized reports, fixed result strings and redaction tests. |
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
| Sprint 0 public messages, proofs and results, including the second-round ruling | WS-1, WS-2, WS-3 | Items 1 to 7 use the agreed messages. The handoff names requested additions and points for comparison. |
| Sprint 1 state, key, reports, outbox and receipt callers | WS-1, WS-2, WS-3 | Items 2 to 8 reuse the state, scan and transactional store. The handoff requests the one clock option and the updated event-mark prose. |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repository or assignment rule | How this plan satisfies it |
|---|---|
| `AGENTS.md`, "Where things are" and the epic's Client scope | Build changes are under `src/client/`. The generated contract supplies the network boundary. Changes to the floorplan, server, stack, test script and website go to their owners through the result. |
| `AGENTS.md`, habit 1, credit copied code | The plan uses existing in-repository primitives and Go interfaces. Any implementation copied from another project needs its third-party directory, license and notice entry through the owning team. |
| `AGENTS.md`, habit 2, write down what was found | The result names issues 1632 to 1640 and records new compatibility problems for the orchestrator. |
| `AGENTS.md`, habits 3 and 4, branch and upstream rules | Plan authorship stops before repository integration. The orchestrator owns commits and the base-branch pull request. |
| `AGENTS.md`, "Writing style for documents" | Use dated decisions and repository-relative anchors. Run `scripts/check-private.mjs` and read the new files against the writing rules. |
| The bounded assignment and `common-planner.md` | The four assignment artifacts are the only planned writes. The orchestrator runs the adversarial review and supplies the launch allocation. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on |
|---|---|---|---|---|---|---|
| 1. Origin and transport | backend | client/backend | medium | One parser and one transport policy, with independent listener tests. | Yes | Sprint 0. |
| 2. Enrollment | backend | client/backend | medium | One generated unary call and one existing durable write boundary. | Yes | 1, Sprint 0, Sprint 1 items 2 and 3. |
| 3. Stream proof | backend | client/backend | medium | The frozen proof fixture covers one handshake, independent of retry and reports. | Yes | 1, 2 and the Sprint 0 link contract. |
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
| Plan ready for round 1 | Pending | The bounded assignment ends at the pre-review handoff. The orchestrator creates the review rounds and folds their findings. |

</details>
