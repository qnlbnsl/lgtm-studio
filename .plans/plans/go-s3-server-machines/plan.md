---
topic: go-s3-server-machines
status: done
tier: L
created: 2026-11-25
updated: 2026-11-26
epic: go-rewrite
---

# Machines and the server's join, without the stream

- **Sprint:** 3. A machine joins
- **Team:** Server
- **Design:** [docs/specs/2026-11-25-go-rewrite-foundation-design.md](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md)
- **Epic:** [go-rewrite](../../epics/go-rewrite/epic.md)
- **Contract:** [go-s0-contract](../go-s0-contract/plan.md), with its drafts in `../go-s0-contract/draft/`

## Overview

This plan builds part A of the server's half of sprint 3: the calls that need no stream from a machine. The client's half is the plan `go-s3-client-link`, and the installer is the plan `go-s3-stack-client`.

The owner makes a join token on the Machines page of the website. A machine uses the token once to register its public key, and it appears on the page as waiting, without a reload. The owner can approve it under a name, rename it, revoke its token while it waits, and remove it.

UNREVIEWED FIX: on 2026-11-26 the orchestrator cut the plan in three, because its 15 items could not each be given a passing gate on paper (architect-r4 01, logic-r4 01, decision 87). Part B, the stream, becomes the plan `go-s3-server-stream`, and part C, the commands, becomes `go-s3-server-commands`. Both are planned after this part and sign-in item 9 are built, against the frozen contract of the link. What this plan knew about them is in [handoff-stream-and-commands.md](handoff-stream-and-commands.md), and the reviewed text is in `attack/plan-v4.md`.

Until part B is built, no machine is ever connected. An approved machine therefore shows as offline, and a scan or a restart is refused as not online or not connected (decision 90).

The plan has 7 work items in two workstreams, each rated low or medium, each with tests written before its code. Six items are risky, and for each the plan names a change to the code that must turn its tests red.

## Key decisions

**Wave 1 tokens are for the loopback address only.** The stack of wave 1 has no certificate, so a token carries the server's configured `http` loopback address and no pin, and the server refuses an `https` address. This settles issue 1633 for wave 1: the join secret travels in cleartext over the loopback and the stack's private network, inside one compose stack, and the signed challenge of part B is the proof (threat-r1 01, decision 76). UNREVIEWED FIX: the combined check of an encrypted join waits for the later work on the proxy, and the result file asks the Contract team to move it out of wave 1 (architect-r3 09, architect-r4 06).

**Keys are not replaced in wave 1.** If the server's signing key is ever replaced by hand, every machine that joined before refuses the server, and the owner removes and joins each one again (issue 1639).

**The machine that runs the server is marked by its token.** Only the server's own command, `lgtm-server join-token --hosts-server`, makes such a token (issue 1638). The owner may remove that machine like any other.

**An approval stores no command in part A.** Part C adds the change of standing that the machine receives, in the same write as the approval (decision 89).

**Each item declares its own names.** No item changes a signature of built code, so no caller needs adapting (decision 88).

Full log → `decisions.md`

## Workstreams

### The owner makes a join token, and a machine joins with it

The server keeps each machine's record and key, and only a verifier of each token, which works once and for 15 minutes. A machine whose reply was lost asks again with the same key and gets the same answer. The main risk is a token that works twice, and the tests race two uses. When it is done, check C5 sees a used and an expired token refused, and check C6 sees the joined machine waiting.

### The Machines page works against the Go server

The page's live list and its record calls run against the Go server. The page's join command needs a change on the page, which this plan cannot make (architect-r1 10). The main risk is a list that keeps sending after the owner's session ended, and the tests revoke a session in the middle of a stream.

## Rollout order

1. Build the machine records, and the write deadline once sign-in item 6 is built, side by side.
2. Build the tokens.
3. Build joining and the page's live list side by side.
4. Build the page's record calls.
5. Wire both into the server program and its test entry point, once sign-in item 9 is built.
6. At the end of part A, run `deploy/test/checks.sh --only C5,C6` against the stack with the client and the installer of the other two plans. Checks C7 to C11 wait for parts B and C.

After each step, `deploy/test/with-postgres.sh -- go test -race -count=1 ./src/server/...` passes (architect-r1 02). Each item that adds a table adds its own numbered database change of the part `machines`, and a later correction is a new change. After the first start of this server, the sprint 2 server refuses to open the database, so recovery goes forward with a corrected server (critic-r1 11).

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**How every item is built.** Phase 4 of the build loop applies to each item. A test writer of another vendor than the builder writes the named tests first, from this plan and the design, against stubs that the test writer adds in the item's own files and that return the zero value. Each test must fail on its assertion, not on a build error or a crash, at a commit where every item of its row in "Execution sizing" is merged. The tests are then frozen. For a risky item, a reviewer applies the named mutation on a temporary branch and the named test must go red. Statements are cited as D1 to D23 (the design's decisions table), R1 to R11 (its rules) and C1 to C13 (its checks). Every item has the scope `server/backend` and the work area `backend`. UNREVIEWED FIX: the skeleton of round 3 is removed, and each item declares the names it writes (architect-r4 01, logic-r4 01, decision 88).

**Fixtures shared by every item.** A database test uses `migrated.NewStore` with `machines.Migrations()` against the PostgreSQL that `LGTM_TEST_POSTGRES` names. Every acceptance command runs under `deploy/test/with-postgres.sh --`, so a missing database fails instead of skipping. A test of package `machines` that needs `link` sits in `machines_test`. A wait is at most 15 seconds, and a check that something does not happen waits 250 milliseconds. A transport test serves the handler through `net/http/httptest` with cleartext HTTP/2, the second version of the Hypertext Transfer Protocol (h2c). A signed-in website session comes from `auth.Sessions.Issue`. A test reads the server's log from a `log/slog` handler of JSON (JavaScript Object Notation) that writes into a buffer.

**The order of locks.** Every write runs in one `Store.Write`. A token's row is locked before a machine's row, and no write locks them in the other order. An approval and a revocation of the same waiting machine therefore form one line, decided by the machine's row (decision 48).

**Errors that cross the store.** A store write or read passes out only an error made with `store.NewDomainError` and returned unwrapped, and turns every other error into a `*store.Error` (`src/server/internal/store/pool.go`, `sanitizeError`). Every error that a caller of this plan tells apart is therefore such a sentinel: `ErrNoMachine`, `ErrNameTaken`, `ErrNotWaiting`, `ErrInvalidAddress`, `ErrNoCertificatePin`, `ErrInvalidHostName` and the five sentinels of the refusals of `Join`. A handler maps an error in one order (decision 85). First, a sentinel of this plan gives its code below, and `store.ErrInvalidRequestKey` and `store.ErrInvalidRequest` give `invalid_argument`, `store.ErrRequestKeyConflict` gives `already_exists`, and `store.ErrRequestFingerprintUnavailable` gives `failed_precondition`, each with the store's sentence. Second, the context's own error gives `canceled` or `deadline_exceeded`. Third, on a page's call, a `*store.Error` whose `Retryable` is true gives `aborted`. Every other error gives `unavailable`, and `Join` skips the third step, as the link's contract says. A page's call normalizes a name before `WriteOnce`, so `ErrInvalidName` never crosses the store.

The names that other teams call have these exact signatures.

```go
// Package machines. The command `lgtm-server join-token` of go-s3-stack-client calls these.
type Issuer struct{ PublicURL string; ServerKey ed25519.PublicKey }
type TokenOptions struct{ HostsServer bool } // no address (architect-r1 09, decision 53)
type NewToken struct{ ID, Text, Address string; ExpiresAt time.Time }
var ErrInvalidAddress, ErrNoCertificatePin error
func NewTokens(s *store.Store, r *Registry, issuer Issuer) *Tokens
func (t *Tokens) Create(ctx context.Context, o TokenOptions) (NewToken, error)

// Package servertest, added by item 7 to the fixture of sign-in item 9. The orchestrator's tests call these.
func (s *Server) Clients(t testing.TB) lgtmv1connect.ClientServiceClient // signed in
func (s *Server) Link() linkv1connect.LinkServiceClient
```

**Tables and topics.** Part `machines` owns `machines` and `client_join_tokens`, in the schema of the application's connection, which is `public` in the stack. The feed topics are `machines` and `client_join_tokens`, each with an empty scope (decision 30). No log line holds a token or a secret (R9).

**Codes and sentences.** Each error is a Connect code with one fixed sentence (R8). A refusal's reason goes into the metadata of the `connect.Error` with `err.Meta().Set`. Connect carries it as a response header of `Join`, and gRPC (Google Remote Procedure Call) as a trailer.

| Call | Condition | Code | Reason key and value | Sentence |
|---|---|---|---|---|
| page | Unknown machine identifier | `not_found` | none | No machine has that id. |
| page | Name empty, too long or with a control character | `invalid_argument` | none | A machine needs a name of 1 to 64 characters. |
| page | Name taken | `already_exists` | none | Another machine already has that name. |
| page | Approve a machine that does not wait | `failed_precondition` | none | This machine does not wait for approval. |
| page | Rename a machine that waits | `failed_precondition` | none | Approve this machine before you rename it. |
| page | Restart a known machine, in part A | `failed_precondition` | none | This machine is not connected. |
| page | Scan a known machine, in part A | `failed_precondition` | none | This machine is not online. |
| page | A request key on `CreateClientToken` | `invalid_argument` | none | This call does not accept a request key. |
| page | The database is busy | `aborted` | none | The database was busy, retry the request. |
| both | The database is unavailable, and every store error of `Join` | `unavailable` | none | The server cannot reach its database, try again. |
| `Join` | A refusal | `permission_denied` | `Lgtm-Join-Refusal`, the `Refusal` that `RefusalOf` gives | The sentence of the `Refusal`, below. |
| `Join` | A value rule broken | `invalid_argument` | none | The link message broke a rule of the contract. |

The refusals of `Join` have these sentences: `token_unknown` "The server does not know this join token.", `token_expired` "This join token has expired. Make a new one.", `token_revoked` "This join token was revoked. Make a new one.", `token_used` "This join token was already used. Make a new one.", and `machine_removed` "The machine that used this join token was removed. Make a new token."

**WS-1: The owner makes a join token, and a machine joins with it**

This workstream gives the server its record of machines and its join tokens, and it answers `Join`. It implements the design's section "Joining: a token that authorizes an exchange of keys" and checks C5 and C6.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1 | A machine record keeps its key and standing, and a removal of a waiting machine spares one just approved. | `TestCreateWaitingAndRead`, `TestApproveRenameAndNames` and `TestDeleteWaitingSparesApproved` pass. |
| 2 | A token carries the contract's fields and a loopback address, lasts exactly 15 minutes, and is stored only as a verifier. | `TestCreateTokenText`, `TestTokenLifetime`, `TestTokenStoresOnlyVerifier` and `TestTokenAddressRules` pass. |
| 3 | `Join` makes one waiting machine, answers a lost reply with the same machine, and refuses every other request with the reason in the error's metadata. | `TestRedeemCreatesWaitingMachine`, `TestRedeemReplay`, `TestRedeemRefusals`, `TestRedeemConcurrentUses`, `TestJoinCall` and `TestJoinRefusalMetadata` pass. |
| 4 | On the stack, a used and an expired token are refused, and the joined machine appears as waiting. | `deploy/test/checks.sh --only C5,C6` prints two lines that start with `ok` and exits 0, with the client of `go-s3-client-link`. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 1 | Write the machines table and its registry. | Target `src/server/internal/machines/store.go`, `Registry`. Pattern: `src/server/internal/auth/sessions.go`, `Sessions`. Scouting S5 and S12. |
| 3 | Write token making and revoking. | Target `src/server/internal/machines/tokens.go`, `Tokens.Create`. Pattern: `src/mock-server/src/engine.ts`, `createClientToken`. Scouting S4. |
| 4 | Write the redemption and the `Join` call. | Targets `src/server/internal/machines/join.go`, `Tokens.Redeem`, and `src/server/internal/link/service.go`, `NewService`. Pattern: `src/server/internal/auth/sessions.go`, `Sessions.Issue`. Scouting S2. |

**Item 1. Machine records.** Writes `src/server/internal/machines/store.go`, `machines/migrate.go` with `Migrations()`, `machines/migrations/0001_machines.sql` and `machines/store_test.go`. Medium: one table and its functions on the pattern of the sessions table. Statements: D3, D5, D7, R1, R3, R4, R7, R10.

- **Risky:** it keeps the key that will prove a machine. Dropping `approval = 'waiting'` from `DeleteWaiting` must fail `TestDeleteWaitingSparesApproved`.
- **The table.** The columns that only the stream of part B writes are part of the record already, so part B adds no table change for them.

```sql
CREATE TABLE machines (
    id text PRIMARY KEY CHECK (id ~ '^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
    public_key bytea NOT NULL CHECK (octet_length(public_key) = 32), host_name text NOT NULL CHECK (octet_length(host_name) <= 255),
    name text NOT NULL DEFAULT '' CHECK (octet_length(name) <= 64), approval text NOT NULL CHECK (approval IN ('waiting', 'approved')),
    state_revision bigint NOT NULL CHECK (state_revision >= 1), hosts_server boolean NOT NULL DEFAULT false, token_id text NOT NULL,
    joined_at timestamptz NOT NULL DEFAULT clock_timestamp(), approved_at timestamptz,
    facts bytea, harnesses bytea, harnesses_scanned boolean NOT NULL DEFAULT false,
    last_seen_at timestamptz, last_address text NOT NULL DEFAULT '');
CREATE UNIQUE INDEX machines_approved_name ON machines (lower(name)) WHERE approval = 'approved';
```

- **Behaviour.** `CreateWaiting` makes the identifier with `store.NewID`, inserts approval `waiting` and revision 1, and records `machines` `saved`. `Get` returns `ErrNoMachine` for an unknown identifier. `List` orders by `joined_at`, then `id`. `SetApproved` refuses a row that is not waiting with `ErrNotWaiting` and sets the name, `approved_at` and the next revision. `SetName` returns `false` for the same name, and otherwise sets it and the next revision. A unique violation of `machines_approved_name` becomes `ErrNameTaken`. `Delete` deletes the row, records `removed`, and returns `ErrNoMachine` when nothing matched. `DeleteWaiting` deletes only `WHERE approval = 'waiting'`, so in read committed mode it re-checks the row after a concurrent approval commits. `NormalizeName` trims spaces and refuses an empty result, more than 64 bytes, invalid UTF-8 (Unicode Transformation Format, 8 bits) and a control character. Every write that changes a row records a change of `machines`.
- **Tests written first,** in `store_test.go`. `TestCreateWaitingAndRead`: The record has a version 7 universally unique identifier (UUID), the key, host name, token identifier and `HostsServer` given, approval `waiting`, revision 1 and an empty name. The feed holds one `machines` `saved` row with that key. `Get` of a random identifier returns `ErrNoMachine`. A write that fails after `CreateWaiting` leaves no row and no feed row. `TestApproveRenameAndNames`: `SetApproved` gives revision 2, and a second one returns `ErrNotWaiting`. Approving another machine as the same name in other letter case returns `ErrNameTaken` and leaves it waiting. A waiting host name equal to an approved name is no conflict. `SetName` to the same name returns `false` and keeps the revision, and a new name gives revision 3. `NormalizeName` turns `" Box "` into `Box` and refuses the empty string, 65 bytes, a tab and invalid UTF-8. `TestDeleteWaitingSparesApproved`: Transaction A approves a waiting machine and waits on a gate. `DeleteWaiting` in B waits, and after A commits it returns `false` and the machine remains. `DeleteWaiting` of a waiting machine returns `true` and records `removed`.
- **Acceptance:** `go test -count=1 -run 'TestCreateWaiting|TestApproveRename|TestDeleteWaiting' ./src/server/internal/machines/` passes.

**Item 3. Join tokens: make and revoke.** Writes `src/server/internal/machines/tokens.go`, `machines/migrations/0002_client_join_tokens.sql` and `machines/tokens_test.go`. Medium: one table and one encoding that the contract fixes. Statements: D7, R4, R7, R9, C5.

- **Risky:** it makes secrets. Storing the secret instead of its verifier must fail `TestTokenStoresOnlyVerifier`. A lifetime of 16 minutes must fail `TestTokenLifetime`.
- **The table.**

```sql
CREATE TABLE client_join_tokens (
    id text PRIMARY KEY, secret_sha256 bytea NOT NULL CHECK (octet_length(secret_sha256) = 32),
    created_at timestamptz NOT NULL, expires_at timestamptz NOT NULL,
    hosts_server boolean NOT NULL DEFAULT false, revoked_at timestamptz, used_at timestamptz,
    used_key bytea CHECK (used_key IS NULL OR octet_length(used_key) = 32), used_by text);
```

- **Which address a token carries.** Always `Issuer.PublicURL`, which the sign-in plan sets to `http://127.0.0.1:<published port>` in the stack. No caller of wave 1 passes another address, so `TokenOptions` has none (architect-r1 09). `Create` passes the address through `NormalizeAddress` before it writes a row. `NormalizeAddress` writes the scheme and host in lowercase, drops `:80` and a trailing slash, and refuses a path, a query, a fragment, a user, another scheme and a host other than `127.0.0.1`, `[::1]` or `localhost` with `ErrInvalidAddress`. An `https` address gives `ErrNoCertificatePin`, because wave 1 has no certificate to pin.
- **What `Create` does.** It draws 32 secret bytes from `crypto/rand` and an identifier from `store.NewID`. It inserts the row with `secret_sha256` equal to SHA-256 (Secure Hash Algorithm, 256 bits) of `lgtm-join-token-v1`, one zero byte and the secret, and one reading of `clock_timestamp()` as `created_at`, with that reading plus 15 minutes as `expires_at` (critic-r1 12, logic-r1 11). It records `client_join_tokens`. It returns the address, the `expires_at` read back from the row, and the text of `JoinToken{token_id, secret, address, server_key_sha256 = SHA-256(Issuer.ServerKey)}` with an empty `certificate_key_sha256`.
- **`EncodeToken` and `DecodeToken`.** The text is `lgtm1_` and the deterministic binary encoding in unpadded base64url. `DecodeToken` refuses another prefix, padding and trailing bytes.
- **`Revoke`** locks the token's row `FOR UPDATE`. An unknown identifier is no error. It sets `revoked_at` when empty, records a change, and returns `used_by`. The caller then calls `Registry.DeleteWaiting` in the same transaction.
- **No pruning.** UNREVIEWED FIX: `Prune` is removed, because no caller was ever named and no check of wave 1 needs it. Expired tokens stay in the table, which is outside wave 1, and the result file asks for an issue (decision 92).
- **Tests written first,** in `tokens_test.go`. `TestCreateTokenText`: With `PublicURL` `http://127.0.0.1:8787`, the text decodes to a token whose identifier equals `NewToken.ID`, with a 32-byte secret, the address `http://127.0.0.1:8787`, which also equals `NewToken.Address`, `server_key_sha256` equal to SHA-256 of the issuer's key, and an empty pin. `EncodeToken` of the message in `src/contracts/testdata/join-token.json` gives that file's text, and `DecodeToken` gives back its fields. `TestTokenLifetime`: `expires_at - created_at` is exactly 15 minutes, and `NewToken.ExpiresAt` equals the row's `expires_at`. `TestTokenStoresOnlyVerifier`: `secret_sha256` equals SHA-256 of the label, a zero byte and the secret. The secret, in raw bytes, hexadecimal, base64 or base64url, appears in no column read with `row_to_json` and in no feed row. `TestTokenAddressRules`: `HTTP://127.0.0.1:8787/` gives `http://127.0.0.1:8787`, and `http://[::1]:80` gives `http://[::1]`. `http://example.test:8787`, `ftp://127.0.0.1` and an address with a path return `ErrInvalidAddress`. `https://box.example` returns `ErrNoCertificatePin`. An issuer with each refused address makes `Create` fail and insert no row. `TestHostsServerMark`: A token made with `HostsServer` true has `hosts_server` true in its row, and one without has false. `TestRevoke`: Revoking an unused token sets `revoked_at` and returns an empty string. An unknown identifier returns no error. A token whose `used_by` the test sets returns that machine.
- **Acceptance:** `go test -count=1 -run 'TestCreateTokenText|TestTokenLifetime|TestTokenStoresOnlyVerifier|TestTokenAddressRules|TestHostsServerMark|TestRevoke' ./src/server/internal/machines/` passes.

**Item 4. Joining: redeem a token, and the `Join` call.** Writes `src/server/internal/machines/join.go`, `src/server/internal/link/service.go`, `link/join.go`, `link/rules.go`, `machines/join_test.go` and `link/join_test.go`. Medium: one transaction with a fixed order of checks, and one thin handler. Statements: D7, R4, R8, R9, R10, C5, C6.

- **Risky:** it decides who may register a key. Checking the expiry after the replay must fail `TestRedeemRefusals`. Comparing the digests with `bytes.Equal` must fail `TestSecretCompareIsConstantTime`. Writing the reason as a header before the error must fail `TestJoinRefusalMetadata`.
- **What `Redeem` does,** in one `Store.Write`. It locks the token's row `FOR UPDATE`, then reads `clock_timestamp()`. A second use that waited on the lock therefore decides against the state and the clock after the first use committed. It then decides in the contract's order. (1) No row, or a verifier that `crypto/subtle.ConstantTimeCompare` finds different, gives `RefusalUnknown`. (2) `expires_at` at or before the clock gives `RefusalExpired`. (3) `revoked_at` set gives `RefusalRevoked`. (4) `used_key` set to another key gives `RefusalUsed`. (5) `used_key` equal to the key gives `Joined{used_by, Replay: true}` and writes nothing when `used_by` still exists, and `RefusalMachineRemoved` otherwise. (6) Otherwise it calls `Registry.CreateWaiting` with the token's `hosts_server`, sets `used_at`, `used_key` and `used_by`, and records `client_join_tokens`. Step 6 alone checks the host name: valid UTF-8, at most 255 bytes, and no zero byte, which PostgreSQL text cannot hold. A broken one returns `ErrInvalidHostName`. A replay never reads the host name (threat-r1 10). Each refusal returns its own sentinel, unwrapped, so the store passes it out and the write commits nothing (critic-r1 01).
- **What `link.Service` is in part A.** `NewService(s *store.Store, t *machines.Tokens, serverKey ed25519.PublicKey) *Service` keeps its arguments. `Service` embeds `linkv1connect.UnimplementedLinkServiceHandler`, so `Open` answers `unimplemented` until part B (decision 90). Part B changes this file.
- **What `Service.Join` does.** `rules.go` checks the value rules of the contract: a canonical UUID, a 32-byte secret, and a 32-byte key. A broken rule, or `ErrInvalidHostName`, gives `invalid_argument` and touches no row. An error for which `RefusalOf` gives a reason gives `permission_denied` with its sentence and the metadata `Lgtm-Join-Refusal`. A store error gives `unavailable`. Success gives `JoinResponse{machine_id, server_public_key}`.
- **Tests written first,** in `machines/join_test.go`. `TestRedeemCreatesWaitingMachine`: A fresh token, its secret, a key and the host name `box` give `Replay` false and a waiting machine with revision 1, host name `box`, the token's identifier and its `hosts_server`. The token row holds the key, the machine and `used_at`. `TestRedeemReplay`: The same token, secret and key with the host name `other` give the same machine and `Replay` true. One machine exists, still named `box`. `TestRedeemRefusals`: Each case starts from its own token and leaves the machines and the token row unchanged: an unknown identifier, a wrong secret, and a wrong secret on an expired token each give `token_unknown`; an expired token gives `token_expired`; a revoked token gives `token_revoked`; a revoked token that also expired gives `token_expired`; a used token with another key gives `token_used`; the first key after its machine was deleted gives `machine_removed`; a replay after expiry gives `token_expired`. `TestRedeemConcurrentUses`: Two first uses with two keys, held at a gate, give one machine and one `token_used`. Two identical first uses give the same machine to both. When the test sets the expiry one second ahead and holds the second use until it passed, the second gets `token_expired`. `TestRedeemRollsBack`: A test trigger that fails the insert into `machines` makes `Redeem` return an error for which `RefusalOf` gives no reason, leaves no machine and an empty `used_key`, and after the trigger is dropped the same request succeeds. `TestSecretCompareIsConstantTime`: Parsing `join.go` with `go/parser` finds `subtle.ConstantTimeCompare` on the two digests and no `bytes.Equal` or `==` between them.
- **Tests written first,** in `link/join_test.go`, through h2c. `TestJoinCall`: A fresh token answers a version 7 `machine_id` and a `server_public_key` whose SHA-256 equals the token's `server_key_sha256`. The same request again answers the same identifier. `TestJoinRefusalMetadata`: For each of the five reasons, the Connect client gets `permission_denied`, the reason's sentence, and `Lgtm-Join-Refusal` in `connect.Error.Meta()`. The same holds over the gRPC protocol, as a trailer. No header, trailer or message holds part of the token's text or secret. `TestJoinValueRules`: An identifier in upper case, a secret of 31 bytes, and a key of 33 bytes each answer `invalid_argument` and change no row. A host name of 256 bytes, and one with a zero byte, answer `invalid_argument` on a first use and the first machine on a replay. `TestJoinDatabaseDown`: With the store closed, `Join` answers `unavailable`. `TestJoinKeepsSecretsOutOfLogs`: After a success and a refusal, the log holds neither the token's text nor its secret in hexadecimal, base64 or base64url. `TestOpenUnimplemented`: `Open` answers `unimplemented`.
- **Acceptance:** `go test -count=1 -run 'TestRedeem|TestSecretCompare' ./src/server/internal/machines/` and `go test -count=1 -run 'TestJoin|TestOpenUnimplemented' ./src/server/internal/link/` pass.

**Design artifact:** not applicable. No screen changes.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| A token works twice, for two keys at once. | High | The token's row is locked for the whole redemption. `TestRedeemConcurrentUses`. |
| A refusal tells a caller without the secret whether a token exists. | Medium | A wrong secret gives `token_unknown` before any other check. `TestRedeemRefusals`. |
| A token's secret reaches the database, a log or the feed. | High | Only the verifier is stored. `TestTokenStoresOnlyVerifier` and `TestJoinKeepsSecretsOutOfLogs`. |

**Verify:** `deploy/test/with-postgres.sh -- go test -race -count=1 ./src/server/internal/machines/... ./src/server/internal/link/...`. Environment check: `deploy/test/checks.sh --only C5,C6` at the end of part A. No device is needed.

**Rollout detail:** rollout steps 1 to 3. `Store.Migrate` applies both changes of the part `machines` at start-up, once item 7 registers them.

**WS-2: The Machines page works against the Go server**

This workstream answers the calls of `ClientService` that the Machines page already makes. It implements the design's section "Serving the contract", rules R2, R5 and R8, and checks C5 and C6.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 5 | A blocked send ends at its deadline or at the end of its context. | `TestDeadlineEndsBlockedStream`, `TestSendEndsOnCancel` and `TestSendWaitsForCallback` pass. |
| 6 | The page's record shows waiting or offline, and every live-list message carries a position. | `TestProjectClient`, `TestWatchClientsFirstMessageAndChanges` and `TestWatchClientsPositionAlwaysPresent` pass. |
| 7 | The live list ends when the session ends or the reader stops. | `TestWatchClientsNeedsLiveSession` and `TestWatchClientsBlockedReaderEnds` pass. |
| 8 | Every record call keeps its rules, and a revocation never removes an approved machine. | `TestCreateClientToken`, `TestRevokeClientToken`, `TestRemoveClient`, `TestApproveClient`, `TestRenameClient`, `TestScanAndReconnectRefused` and `TestKeyedCallsReplay` pass. |
| 9 | The server program serves both services and keeps machines across a restart. | `TestServerServesMachines` and `TestRestartKeepsMachines` pass. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 2 | Write the write deadline for long streams, and wrap the two route families. | Target `src/server/internal/transport/deadline.go`, `WithWriteDeadline`. Changes `src/server/internal/api/server.go`, `NewHandler`, of `go-s2-server-signin`. Pattern: `net/http.ResponseController`. Scouting S6 and S12. |
| 5 | Write the projection and `WatchClients`. | Targets `src/server/internal/machines/service.go`, `NewService`, `machines/project.go`, `Project`, and `machines/watch.go`, `Service.WatchClients`. Pattern: `src/mock-server/src/engine.ts`, `watchClients`, and `src/server/internal/watch/watch.go`, `Serve`. Scouting S3 and S5. |
| 6 | Write the record calls. | Target `src/server/internal/machines/records.go`, `Service.CreateClientToken`. Pattern: `src/mock-server/src/engine.ts`, `revokeClientToken`. Scouting S4. |
| 7 | Wire both parts into the server program and its test entry point. | `src/server/internal/app/start.go`, `Start`, and `src/server/servertest/server.go`, of sign-in item 9. Scouting S6. |

**Item 2. Write deadlines for long streams.** Writes `src/server/internal/transport/deadline.go` and `transport/deadline_test.go`. Once sign-in item 6 has built `src/server/internal/api/server.go`, it changes `NewHandler` in the shape it has then: the handlers of `ClientService` and `LinkService` are wrapped in `transport.WithWriteDeadline`, and the link's handler gets `connect.WithReadMaxBytes(linkReadLimit)`, a private constant of 64 kibibytes. It adds `TestRouterWrapsLongStreams` to `api/server_test.go`. Low: one middleware of the standard library. Statements: R5, R7, and the store plan's rule that the Machines adapter ends a blocked response at the transport. Not risky.

- **Behaviour.** `WithWriteDeadline` puts a setter of `http.NewResponseController(w).SetWriteDeadline` into the request's context. `Send` sets the deadline to now plus `SendTimeout`, which is 10 seconds, calls `send`, and clears the deadline, also on failure. While `send` runs, an end of `ctx` sets the deadline to now through `context.AfterFunc`, so a cancelled watch ends a blocked write at once (architect-r1 11). `Send` returns the context's error without calling `send` when `ctx` has already ended. After `send` it calls the stop function of `context.AfterFunc`. When the callback never ran it clears the deadline, and when the callback has started, `Send` waits for it to finish and leaves the deadline expired, so the callback never touches a released response (decision 84). An expired deadline resets the HTTP/2 stream, so the peer of a blocked send sees a transport error and no status (architect-r2 05). Without the setter, `Send` returns `ErrNoDeadline` and does not call `send`. `WithWriteDeadline` is `WithTimeout(next, SendTimeout)`, and a test of another package shortens the 10 seconds with `WithTimeout`.
- **UNREVIEWED FIX: no new option of the router.** `api.Options.LinkOptions` and `app.Hooks` are removed, so the signatures of sign-in items 6 and 9 stay as those items build them (architect-r4 01, logic-r4 01, decision 91).
- **Tests written first:** `TestDeadlineEndsBlockedStream` (a handler that sends 64 kibibytes at a time to a client that stops reading gets an error from `Send` within 2 seconds at a timeout of 200 milliseconds, and returns); `TestDeadlineClearedAfterSend` (two sends 400 milliseconds apart to a reading client both succeed); `TestSendNeedsTheMiddleware` (`ErrNoDeadline`, and the function is not called); `TestSendWaitsForCallback` (under `-race`, a private hook pauses a started callback, and `Send` has not returned until the hook releases it); `TestSendEndsOnCancel` (at a send timeout of 10 seconds, a send blocked on a client that stops reading returns within 300 milliseconds of the end of its context, and a later `Send` on that context returns its error without calling its function); `TestRouterWrapsLongStreams` (probe handlers at `/lgtm.v1.ClientService/WatchClients` and `/lgtm.link.v1.LinkService/Open` get no error from `Send`, and a probe `Join` larger than 64 kibibytes answers `resource_exhausted`).
- **Acceptance:** `go test -count=1 ./src/server/internal/transport/` and `go test -count=1 -run TestRouterWrapsLongStreams ./src/server/internal/api/` pass.

**Item 5. The record of the page, and `WatchClients`.** Writes `src/server/internal/machines/service.go`, `machines/project.go`, `machines/watch.go`, `machines/project_test.go` and `machines/watch_test.go`. Medium: one pure mapping and one adapter of the store's helper. Statements: D5, R2, R5, R7, R8, C6.

- **Risky:** it orders changes and checks the session. Leaving `position` out of a `saved` message must fail `TestWatchClientsFirstMessageAndChanges`. An `Authorize` that returns nil must fail `TestWatchClientsNeedsLiveSession`.
- **The service.** `NewService(s *store.Store, sessions *auth.Sessions, r *Registry, t *Tokens) *Service` keeps its arguments. `Service` embeds `lgtmv1connect.UnimplementedClientServiceHandler`, so a call that item 6 has not built answers `unimplemented`. Item 6 adds its methods in its own file.
- **What `Project` does.** UNREVIEWED FIX: until part B, no machine is connected (decision 90). The state is `CLIENT_STATE_WAITING_FOR_APPROVAL` while waiting, and `CLIENT_STATE_OFFLINE` once approved. The load and `connected_at` are never set. The name is the host name while waiting, and `token_id` is set only while waiting. `address` and `last_seen` come from `LastAddress` and `LastSeenAt`, which stay empty until part B writes them. The facts fill `os`, `version`, `workspace`, `agent_limit`, `cores` and `memory_gigabytes`, which is `memory_bytes` over 2^30 rounded to the nearest whole number and capped at the largest `uint32`. `harnesses_scanned`, `harnesses` and `hosts_server` come from the record.
- **What `WatchClients` does.** It reads the verifier with `api.SessionVerifier` and calls `watch.Serve` with the topic `machines` and an empty scope. `Authorize` calls `auth.Sessions.CheckInRead`, and `auth.ErrNoSession` becomes `watch.ErrWatchUnauthorized`. `Snapshot` and `Load` call `Registry.List` or `Get`, and `ErrNoMachine` means the record is gone. Each message carries `position`, also 0. Each send goes through `transport.Send` with the context that `watch.Serve` passes to `send` (threat-r3 07). `watch.ErrWatchUnauthorized` gives `unauthenticated` with "Sign in again to reopen this watch.". `watch.ErrAuthorizationUnavailable` gives `unavailable` with "The watch authorization check is unavailable, try again.". `store.ErrWatchLagged` gives `aborted` with "The watch fell behind and must be reopened.". `store.ErrFeedUnavailable` gives `unavailable` with "The change feed is unavailable, try again.".
- **Tests written first.** `TestProjectClient`: A waiting machine shows waiting, its host name and token, and no load. An approved machine shows offline with its name, no load and no `connected_at`. Facts and harnesses that the test writes into the row with SQL come out mapped: 1,610,612,736 bytes give 2 gigabytes, 429,496,729 bytes give 0, and the largest `uint64` gives 4,294,967,295. `TestWatchClientsFirstMessageAndChanges`: With a cookie, the first message is `all` with a `position`. A new machine arrives as `saved` and a deleted one as `removed_id`, each with a higher position. `TestWatchClientsPositionAlwaysPresent`: On an empty database the first message is an empty `all` with `position` 0 present. `TestWatchClientsNeedsLiveSession`: Without a cookie the call gets `unauthenticated`. After a revocation in the middle of a stream, the stream ends with `unauthenticated` within 35 seconds, and no later `saved` arrives. `TestWatchClientsErrorMapping`: The private mapping of errors, called with `store.ErrWatchLagged`, `store.ErrFeedUnavailable`, `watch.ErrWatchUnauthorized` and `watch.ErrAuthorizationUnavailable`, gives the four codes and sentences above, because the store's seams are private to the store (architect-r1 02). `TestWatchClientsBlockedReaderEnds`: At a `transport.WithTimeout` of 200 milliseconds, a reader that stops while 500 changes are written sees the stream end within 3 seconds, and the handler returns. UNREVIEWED FIX: a send that is blocked when the session is revoked ends at its deadline at the latest, and no `saved` written after the revocation arrives. The test does not require the revocation to end the send sooner, because the shared watch checks the session only on a change and on its 30-second tick (architect-r4 07, logic-r4 08).
- **Acceptance:** `go test -count=1 -run 'TestProjectClient|TestWatchClients' ./src/server/internal/machines/` passes.

**Item 6. The record calls.** Writes `src/server/internal/machines/records.go` and `machines/records_test.go`. Medium: seven handlers over tested functions. Its tests sit in `machines_test`. Statements: D3, R4, R8, R9, C5.

- **Risky:** it hands out tokens and decides who may work. Accepting a request key on `CreateClientToken` must fail `TestCreateClientToken`. Replacing `DeleteWaiting` with `Delete` in the revocation must fail `TestRevokeClientToken`. Projecting the reply after the commit must fail `TestKeyedCallsReplay`.
- **Behaviour.** Each keyed call runs in `Store.WriteOnce` with the caller `owner`, its procedure, `ReceiptSafe` true and a version 1 fingerprint of its fields in the order of the request, each framed by an eight-byte big-endian length.
  - `CreateClientToken` refuses a present `Lgtm-Request-Key` before it makes a token, calls `Tokens.Create` with no mark, and answers `token_id`, `token`, `expires_at` and `address` with the header `Cache-Control: no-store` (threat-r1 08).
  - `RevokeClientToken` calls `Tokens.Revoke`, then `Registry.DeleteWaiting` for the machine it returns, in one write. An approval that commits first leaves the machine (decision 48).
  - `RemoveClient` calls `Registry.Delete` in one write. `ErrNoMachine` gives `not_found`. It removes a waiting machine and the machine that runs the server as well (decision 21).
  - `ApproveClient` normalizes the name before `WriteOnce`, calls `Registry.SetApproved`, projects the machine through the `*store.Tx`, and returns the encoded reply to `WriteOnce`, so a replay gets the same bytes (architect-r1 08, critic-r1 08, logic-r1 8). UNREVIEWED FIX: it stores no command in part A (decision 89).
  - `RenameClient` normalizes the name before `WriteOnce`, refuses a waiting machine, calls `Registry.SetName`, and builds its reply the same way.
  - UNREVIEWED FIX: `ScanClientHarnesses` and `ReconnectClient` read the record and answer `not_found` for an unknown machine. For a known machine they answer "This machine is not online." and "This machine is not connected." and store nothing, because no machine is connected before part B (decision 90).
- **Tests written first,** in `records_test.go`. `TestCreateClientToken`: The reply's `token` decodes, its identifier and address equal the reply's `token_id` and `address`, and `expires_at` is 15 minutes after the call within 2 seconds. The reply carries `Cache-Control: no-store`. A machine that joins with it has `hosts_server` false. With `Lgtm-Request-Key`, the call answers `invalid_argument` with "This call does not accept a request key." and no token row exists. `TestRevokeClientToken`: After a revocation of an unused token, `Redeem` gives `token_revoked`. A revocation of a token used by a waiting machine deletes the machine, and an open `WatchClients` receives `removed_id`. With an approval held on the machine's row, the revocation waits, and after the approval commits the machine remains approved. An unknown identifier answers success. `TestRemoveClient`: The record is gone, and an open `WatchClients` receives `removed_id`. An unknown identifier answers `not_found`. One request key sent twice removes once and answers success twice. The machine that runs the server can be removed. `TestApproveClient`: Approving as `Box` answers a client of that name, revision 2 and state offline. An approved machine gets `failed_precondition`, an unknown one `not_found`, an empty name `invalid_argument`, and a taken name `already_exists`. `TestRenameClient`: A rename raises the revision. The same name keeps it. A waiting machine gets `failed_precondition`. `TestScanAndReconnectRefused`: A waiting and an approved machine each get the two refusals, and no row of any table changes. `TestKeyedCallsReplay`: A taken name in a keyed approval answers `already_exists`, and a lock timeout in a keyed approval answers `aborted` (logic-r3 10). The same key with another identifier answers `already_exists`. An approval whose reply is dropped, then a rename and a removal, then the same key again, answer the first reply's bytes and leave one approval.
- **Acceptance:** `go test -count=1 -run 'TestCreateClientToken|TestRevokeClientToken|TestRemoveClient|TestApproveClient|TestRenameClient|TestScanAndReconnectRefused|TestKeyedCalls' ./src/server/internal/machines/` passes.

**Item 7. Wiring into the server program and its test entry point.** UNREVIEWED FIX: once sign-in item 9 is built, this item changes `src/server/internal/app/start.go` and `src/server/servertest/server.go` in the shape they have then, and writes `app/machines_test.go` in package `app_test` and new tests in `servertest/server_test.go`. It changes no signature of sign-in item 9, so `cmd/lgtm-server/main.go` and every caller stay as they are (architect-r4 01, logic-r4 01, decision 93). Medium: composition in one function and two test helpers. Statements: D1, D3, D7, R1, R7, R10, C5, C6, and issue 1634.

- **Risky:** it places the page's calls behind the session guard and `Join` outside it. Passing the `ClientService` handler as the link handler must fail `TestServerServesMachines`.
- **What changes.** The application's start adds `machines.Migrations()` to `Store.Migrate`. It builds the registry, the tokens with `Issuer{PublicURL: cfg.PublicURL, ServerKey}` from the public half of `vault.LinkIdentity.SigningKey`, and both services, and passes them as `api.Options.Clients` and `api.Options.Link`. `servertest.Server` gains `Clients(t testing.TB) lgtmv1connect.ClientServiceClient`, signed in through the fixture's cookie jar, and `Link() linkv1connect.LinkServiceClient`.
- **Tests written first.** `TestServerServesMachines`: Through `servertest.Start`, `Clients(t).CreateClientToken` answers a token whose address is the server's URL. `Join` through `Link()` without a cookie answers a machine. `WatchClients` without a cookie answers `unauthenticated`. `Open` answers `unimplemented`. A `Join` larger than 64 kibibytes answers `resource_exhausted`. `TestRestartKeepsMachines`: A joined and approved machine is listed approved after `Restart`, and `SigningPublicKey` is unchanged.
- **Acceptance:** `go test -count=1 -run 'TestServerServesMachines|TestRestartKeepsMachines' ./src/server/internal/app/` passes, and every test of sign-in item 9 still passes.

**Design artifact:** not applicable. No screen changes. The page's records and actions exist today, and the sample server shows how they behave.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| The live list keeps sending after the owner's session ended. | High | The store's helper authorizes every change again, and every 30 seconds while quiet. `TestWatchClientsNeedsLiveSession`. |
| A revocation removes a machine that was just approved. | Medium | The revocation deletes only a machine that still waits. `TestRevokeClientToken`. |
| The page shows the old join command. | High | The owner chooses the page change (result file). |
| A sprint 2 file changes in a way that breaks its own tests. | Medium | Items 2 and 7 run every test of `src/server/` before they merge. |

**Verify:** `deploy/test/with-postgres.sh -- go test -race -count=1 ./src/server/...`. Environment check: `deploy/test/checks.sh --only C5,C6` with the client and the installer of the other two plans.

**Rollout detail:** rollout steps 1, 3, 4 and 5. The stack runs `deploy/up.sh`, exports `LGTM_WEB_PASSWORD` and `LGTM_TEST_POSTGRES`, and runs `deploy/test/checks.sh`.

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

The sources are the brief of 2026-11-25 with its items 1 to 9, observations, agreements and points 1 to 4; the task files of this plan in `.foundry/go-rewrite/tasks/`, with the orchestrator's ruling on the split; the requests in `.foundry/go-rewrite/requests-from-contract-r3.md` and `-r4.md`; the issues 1633 and 1636 to 1642; the design's statements D3, D5, D7, D16, D17, R1 to R10 and C5 to C11; `decisions.md` rows 1 to 94; and review rounds 1 to 4 with their four registers in `attack/`.

| Source item | Where | Status |
|---|---|---|
| Brief items 1 to 9, observations and agreements | Items 1 to 7, criteria 4 and 9, and the names other teams call; the stream and the commands in `handoff-stream-and-commands.md` | part A covered, parts B and C handed off |
| Brief points 1 to 4 | Decisions 39, 11, 44, 45 and 17; item 4 writes `link/service.go`, `join.go` and `rules.go` | settled for wave 1, with assumptions |
| Contract rounds 3 and 4 | The codes table; items 3, 4 and 6; decision 48; the stream's requests in the handoff | covered for part A |
| Issues 1633 and 1636 to 1642 | Decisions 16, 15, 17 and 39; the verify lines; the rest outside wave 1 | unchanged |
| D3, D5, D7, R1 to R10, C5, C6 | Every item, the codes table and criterion 4 | covered |
| D16, D17, C7 to C11 | `handoff-stream-and-commands.md` | handed off |
| `decisions.md` rows 1 to 94, review rounds 1 to 4 | Key decisions, the items, the registers, the post-build checks and the handoff | each with one outcome |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repo rule (CLAUDE.md etc.) | How this plan satisfies it |
|---|---|
| `AGENTS.md`: the three parts meet only through the contract in `src/contracts/`. | The server imports only the generated packages `lgtmv1` and `linkv1`, and asks the Contract team for every change of the wire. |
| `AGENTS.md`: new code does not go into `packages/`, `design/mock/` or `plugins/`, and habit 1 credits copied code. | Every file is under `src/server/`, and no code is copied. |
| `AGENTS.md`, habits 2 and 3: write down what you find, and merge through a pull request. | The result file lists the problems for the orchestrator, who merges the sprint. |
| `AGENTS.md`, writing style, and `scripts/check-private.mjs`. | Full sentences, expanded acronyms, no quotation of a person, and only loopback and documentation addresses. |
| The epic: a team edits only its own folders. | Every file is in `src/server/`. Three files of sprint 2 change, all in the Server team's folder (decision 28). |
| The design, rule 10: tests run against a real PostgreSQL. | Every database test uses `migrated.NewStore` with `LGTM_TEST_POSTGRES`. |
| The planning rules of this epic: name no model and no harness. | The plan names work areas and complexities only. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

Models come from the harness-sessions plugin's model sheet (its harness-session skill) at launch; this section names none (owner, 2026-10-16).
The scope `server/backend` exists in `.plans/scopes/server/backend/`.
The last column lists only the items whose behaviour an item's tests call, and the built code they need.

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on |
|---|---|---|---|---|---|---|
| 1. Machine records | `backend` | `server/backend` | medium | One table and its functions on the pattern of the sessions table. | no | sprint 1 store, sprint 0 contract |
| 2. Write deadlines | `backend` | `server/backend` | low | One middleware of the standard library and a router change. | no | sign-in item 6 |
| 3. Tokens | `backend` | `server/backend` | medium | One table and an encoding the contract fixes. | no | 1 |
| 4. Joining | `backend` | `server/backend` | medium | One transaction with a fixed order of checks and one thin handler. | yes, a machine reaches `Join` without a session | 1, 3 |
| 5. `WatchClients` | `backend` | `server/backend` | medium | One pure mapping and one adapter of the store's helper. | yes, the website reaches it | 1, 2, sign-in sessions |
| 6. Record calls | `backend` | `server/backend` | medium | Seven handlers over tested functions. | yes, the website reaches them | 1, 3, 4, 5 |
| 7. Wiring | `backend` | `server/backend` | medium | Composition in one function and two test helpers. | yes, it places the guard | 1 to 6, sign-in item 9 |

No item is high.

Load: each of the 7 items needs one test writer and one builder of the other vendor, which is 14 launches. The 6 risky items each need one reviewer who breaks the code on a temporary branch, which is 6 launches. Code review of the merged part needs about 3 launches. That is about 23 launches, split about evenly between the first two vendors.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (B/M/m) | Outcome |
|---|---|---|
| 1 | 4/15/7, counted once per group of duplicates, 88 points | Folded on 2026-11-26. The register is `attack/round-1-findings.md`. |
| 2 | 1/12/5, counted once per group of duplicates, 50 points | Folded on 2026-11-26. The register is `attack/round-2-findings.md`. |
| 3 | 1/9/4, counted once per group of duplicates, 40 points | Folded on 2026-11-26 under the orchestrator's three rulings. The register is `attack/round-3-findings.md`. |
| 4 | 1/9/0, counted once per group of duplicates, 36 points | Folded on 2026-11-26 under the orchestrator's ruling to split the plan. The register is `attack/round-4-findings.md`. Part A stays, and parts B and C are in `handoff-stream-and-commands.md`. No reviewer reads this fold before the build. |

</details>

<details><summary><b>Post-build checks</b></summary>

Each check needs the built server, and some need the real client or the web server in front. The finding that asked for it is named in each row. Rows P1 to P6 check the places marked `UNREVIEWED FIX`.

| # | Scenario | Expected result | From |
|---|---|---|---|
| P1 | Check out the merge commit of each item in turn, with the items after it absent, and run its acceptance command and `go vet ./src/server/...`. | Each commit builds, and its acceptance passes. At the commit before each item, its frozen tests fail on an assertion and not on a build error. | architect-r4 01, logic-r4 01 |
| P2 | Through the real web server, send a `Join` larger than 64 kibibytes, then a normal one, and call `Open`. | The large one answers `resource_exhausted`, the normal one answers a machine, and `Open` answers `unimplemented`. No caller of `app.Start` changed. | architect-r4 01, logic-r4 01 |
| P3 | On the stack, join a machine, approve it on the page, rename it, then press scan and restart. | The page shows it waiting, then offline under its name. Scan answers "This machine is not online." and restart answers "This machine is not connected.", and no command row exists. | decisions 89 and 90 |
| P4 | Open a `WatchClients` and stop reading while a send is blocked. In one run, revoke the session and use the store's controlled authorization tick, once the Store team offers it, to run a check right after the revocation. In another run, revoke and make no check before the 10-second deadline. | In the first run the handler ends before the deadline. In the second it ends at the deadline. In both, no change written after the revocation arrives. | architect-r4 07, logic-r4 08 |
| P5 | Run the combined join of the Contract plan through the loopback route of the stack's web server. | The join, the waiting state and the approval pass. No encrypted case runs in wave 1. | architect-r4 06 |
| P6 | Make 1,000 tokens and let them expire. | Every row stays, and `CreateClientToken` still answers within one second. | decision 92 |
| P7 | Publish the website on port 8899 and join one machine with the installer and one with the page's command. | The token's address, the address the client dials and the page's command all name port 8899. | architect-r1 09 and 10 |
| P8 | Start the sprint 2 server against a database that this server migrated, then start a corrected server. | The sprint 2 server refuses to start with the store's sentence about the ledger, and the corrected server starts with every machine kept. | critic-r1 11 |
| P9 | Join with a marked fake secret, then search the database, the feed, the logs of both programs, the process list and the reply's headers. | The secret appears only where the owner's choice on the token's input allows, and the reply carries `Cache-Control: no-store`. | threat-r1 08 and PB1 |

</details>
