---
topic: go-s3-server-machines
status: attack
tier: L
created: 2026-11-25
updated: 2026-11-26
epic: go-rewrite
---

# Machines and the server end of the link

- **Sprint:** 3. A machine joins
- **Team:** Server
- **Design:** [docs/specs/2026-11-25-go-rewrite-foundation-design.md](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md)
- **Epic:** [go-rewrite](../../epics/go-rewrite/epic.md)
- **Contract:** [go-s0-contract](../go-s0-contract/plan.md), with its drafts in `../go-s0-contract/draft/`

## Overview

This plan builds the server's half of sprint 3, in which a machine joins the Go server.
The client's half is the plan `go-s3-client-link`, and the installer is the plan `go-s3-stack-client`.

The owner makes a join token on the Machines page of the website.
A machine uses the token once to register its public key, and it appears on the page as waiting, without a reload.
After the owner approves it under a name, the machine connects by itself, proves who it is with its key on every connection, and shows as online with its facts, its load and its harnesses.
The owner can rename it, ask it to scan for harnesses again, restart its client, and remove it.

The server keeps all of this in PostgreSQL.
A machine counts as connected only while the database holds a row for its current connection, and it shows as offline within 30 seconds of its last heartbeat.
Only one server process runs against a database, because the sign-in plan holds a lock for it, so a server that starts clears the rows that the last process left.

A command to a machine is stored before it is sent, and only the answer to its newest attempt, on the current stream of its own machine, counts.
The machine's receipt keeps a command from running twice, so a command runs at most once: it can also expire unrun, or end interrupted with an unknown effect (architect-r1 12).
A machine removes a receipt 24 hours after the command's expiry by its own clock, so when that clock runs fast, a result lost late in the server's 24 hours is reported `expired` for a command that ran, and when it runs more than a day fast, a lost result can let the command run again (architect-r2 04).
A report from a machine is applied only as the next number, with its effect and its number in one commit, so none is lost or applied twice.
Every write that concerns one machine first locks that machine's record, which puts the writes of a connection, a removal and an approval in one line.

The plan has 14 work items in four workstreams, each rated low or medium, each with tests written before its code.
Twelve items are risky, and for each the plan names a change to the code that must turn its tests red.

The plans this one builds on changed in their review rounds on 2026-11-26, and this version follows them.
Wave 1 has no public encrypted address, so the plan reads no certificate, and the contract now carries the reason of every refusal inside the error.

## Key decisions

**A machine that waits for approval can connect and do nothing else.**
The server answers its handshake and its heartbeats, sends it no command except a change of its standing, and ignores its command results.
An event from it is consumed: its number counts, and its report changes nothing.
The client sends its facts again after approval, so nothing is lost.
If the owner's point 3 of the contract plan lets a waiting machine show its facts, one small table in the server changes.

**Wave 1 tokens are for the loopback address only.**
The stack of wave 1 has no certificate, so a token carries the server's configured `http` loopback address and no pin, and the server refuses an `https` address.
This settles issue 1633 for wave 1: the join secret and the stream travel in cleartext over the loopback and the stack's private network, inside one compose stack, and the signed challenge is the proof (threat-r1 01, decision 76).

**Keys are not replaced in wave 1.**
If the server's signing key is ever replaced by hand, every machine that joined before refuses the server, and the owner removes and joins each one again.
A hand-over in which the old key vouches for the new one needs a new message in the contract, so it waits for a later wave (issue 1639).

**The machine that runs the server is marked by its token.**
Only the server's own command, `lgtm-server join-token --hosts-server`, makes such a token (issue 1638).
The owner may remove that machine like any other.

**The address on the page is where the connection came from.**
It is for showing only, because the server never dials a machine (issue 1637).

**`ReconnectClient` restarts a client that is connected.**
The owner decided this on 2026-11-25.
Today's page offers "Try to reconnect" only for an offline machine, which the Go server refuses, and the owner's point 1 of the contract plan decides how the page follows.

Full log → `decisions.md`

## Workstreams

### The owner makes a join token, and a machine joins with it

The server keeps each machine's record and key, and only a verifier of each token, which works once and for 15 minutes.
A machine whose reply was lost asks again with the same key and gets the same answer.
The main risk is a token that works twice, and the tests race two uses.
When it is done, check C5 sees a used and an expired token refused, and check C6 sees the joined machine waiting.

### A machine proves itself and stays connected

Both ends sign a fresh challenge on every connection, and a new connection replaces the older one.
The main risk is an old connection that writes after a newer one was admitted, and the tests stage exactly that.
When it is done, checks C8, C9 and C10 see a machine go offline, come back after a server restart, and be refused after its removal.

### Commands go down once, and reports come up once

A command is stored before it is sent, and a report is saved together with its number.
The main risk is a confirmation that reaches the machine before the report is saved, and the tests break the save to see that none follows.
When it is done, check C11 sees reports kept during an outage arrive once and in order.

### The Machines page works against the Go server

The page's live list and its actions run against the Go server.
The page's join command and its restart action need a change on the page, which this plan cannot make (architect-r1 10).
The main risk is a list that keeps sending after the owner's session ended, and the tests revoke a session in the middle of a stream.
When it is done, check C7 sees an approved machine online with its facts, its load and its four harnesses.

## Rollout order

1. Build the machine records and the write deadline for long streams first. Nothing else of this plan can be tested without the records.
2. Build the tokens and the link's tables side by side, because they touch different tables.
3. Build joining, stored commands and incoming reports side by side.
4. Build the hub of open connections and the page's live list side by side.
5. Build the handshake, which needs the hub, and the page's actions side by side (architect-r2 01, logic-r2 04).
6. Build the loop that runs one connection after the handshake. It is the first point at which a scripted machine can stay connected.
7. Wire everything into the server program and its test entry point. From this step on, the orchestrator's tests in `src/linktest/` can run a real client against a real server.
8. At the end of the sprint, run `deploy/test/checks.sh` against the stack with the client and the installer of the other two plans. Checks C5 to C11 must pass. Check C11 runs only after the Stack team's revision of `deploy/test/checks.sh`, in which it reads the server's mark and compares the applied numbers as a sorted set, because two log lines can swap (architect-r2 07, logic-r2 07, threat-r2 07).

After each step, `deploy/test/with-postgres.sh -- go test -race -count=1 ./src/server/...` passes (architect-r1 02).
Each step adds its own numbered database change, and a later correction is a new change.
The first start of the sprint 3 server records the parts `machines` and `link` in the database's ledger of migrations.
After that start, the sprint 2 server refuses to open the database, so recovery goes forward with a corrected sprint 3 server (critic-r1 11).

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**How every item is built.** Phase 4 of the build loop applies to each item. A test writer of another vendor than the builder writes the named tests first, from this plan and the design, against stubs that return the empty value. Each test must fail on its assertion, not on a build error or a crash. The tests are then frozen. For a risky item, a reviewer applies the named mutation on a temporary branch and the named test must go red. Statements are cited as D1 to D23 (the design's decisions table), R1 to R11 (its rules) and C1 to C13 (its checks). Every item has the scope `server/backend` and the work area `backend`.

**Fixtures shared by every item.** A database test uses `migrated.NewStore` against the PostgreSQL that `LGTM_TEST_POSTGRES` names, with only the parts that exist when its item merges: `machines.Migrations()` alone for items 1 and 3, and `link.Migrations()` as well from item 5 on (architect-r1 02, critic-r1 02). Every acceptance command runs under `deploy/test/with-postgres.sh --`, so a missing database fails instead of skipping. A test of package `machines` that needs `link`, or a test of package `app` that needs `servertest`, sits in the external test package `machines_test` or `app_test`, because the other import would be a cycle. `link/machinetest` imports `machines` and the generated packages, never `link`. A test that must hold or fail a real commit uses the commit gate: it opens its store through `storetest.NewDatabase` and `store.Open` with a `pgx.QueryTracer` of the test, whose `TraceQueryStart` tells the test when the gated connection reaches `COMMIT`, after every statement and the feed's flush, and holds it until the test releases it or ends that backend with `pg_terminate_backend`, which rolls the transaction back (logic-r1 10, logic-r2 03). A trigger cannot hold the commit, because the store runs `SET CONSTRAINTS ALL IMMEDIATE` before its flush. A wait is at most 15 seconds, and a check that something does not happen waits 250 milliseconds. A transport test serves the handler through `net/http/httptest` with cleartext HTTP/2, the second version of the Hypertext Transfer Protocol (h2c). A signed-in website session comes from `auth.Sessions.Issue`. A test reads the server's log from a `log/slog` handler of JSON (JavaScript Object Notation) that writes into a buffer.

**The one rule of locking.** Every write that concerns one machine runs in one `Store.Write` and first calls `Registry.Lock`, which takes `SELECT ... FOR NO KEY UPDATE` on the machine's row. Only then does it touch a row of `link_machines`, `link_connections` or `link_commands`. A removal deletes the machine's row first, which waits for that lock, and then deletes the link rows. A token's row is locked before a machine's row, and no write locks them in the other order. `ClearConnections` and `SweepCommands` take the machines one at a time in order of identifier, one write each, under the same lock (logic-r1 12, architect-r2 01). This order puts the writes of one machine in one line and leaves no cycle of locks. `TestWritesOfOneMachineFormOneLine` proves it: item 5 writes it for `Heartbeat`, `Disconnect` and `Confirm`, and items 6 and 7 add `Claim`, `AcceptResult` and `ApplyEvent`, each held after its lock against `BecomeCurrent` in both orders (logic2-r1 4). (Decision 40.)

**Errors that cross the store.** A store write or read passes out only an error made with `store.NewDomainError` and returned unwrapped, and turns every other error into a `*store.Error` (`src/server/internal/store/pool.go`, `sanitizeError`). Every error of this plan that a caller tells apart is therefore such a sentinel, returned unwrapped from the callback: `ErrNoMachine`, `ErrNameTaken`, `ErrNotWaiting`, `ErrInvalidAddress`, `ErrNoCertificatePin`, `ErrInvalidHostName`, the five sentinels of the refusals of `Join`, `ErrNotCurrent`, `ErrUnknownMachine`, `ErrEventGap`, `ErrEventNumberZero` and `ErrInvalidMessage`. The handlers map them to codes outside the store. A page's call normalizes a name before it writes, so `ErrInvalidName` never crosses the store. A page's call gives `aborted` for a `*store.Error` whose `Retryable` is true, and `unavailable` for every other store error. `Join` and `Open` give `unavailable` for every store error, as the link's contract says. The store also passes out the context's own error, which gives `canceled` or `deadline_exceeded`, and `pgx.ErrNoRows` and `pgx.ErrTooManyRows`, which each read turns into its sentinel. Of the keyed calls, `store.ErrInvalidRequestKey` and `store.ErrInvalidRequest` give `invalid_argument`, `store.ErrRequestKeyConflict` gives `already_exists`, `store.ErrRequestFingerprintUnavailable` gives `failed_precondition`, each with the store's sentence, and any other error gives `unavailable`. (critic-r1 01, architect-r1 08, architect-r2 11, logic-r2 08, decision 54.)

**Public names, in the order of the items.** Each item declares the names of its row, and uses only names of the items it needs, which merge before it (architect-r2 01, logic-r2 04, threat-r2 01). A stub carries a name before its code exists.

| Item | Declares | Needs |
|---|---|---|
| 1, `machines` | `Topic = "machines"`, `MaxNameBytes = 64`, `Migrations()` of the part `machines`, `Approval` with `Waiting` and `Approved`, `Querier` (the `Query` and `QueryRow` that `*store.ReadTx` and `*store.Tx` share), `Record`, `NewMachine`, `Presence`, `Link`, `ErrNoMachine`, `ErrInvalidName`, `ErrNameTaken`, `ErrNotWaiting`, `NewRegistry`, `NormalizeName`, `Standing`, and the `Registry` methods `CreateWaiting`, `Get`, `List`, `Lock`, `ApplyFacts`, `ApplyHarnesses`, `NoteSeen`, `Touch`, `SetApproved`, `SetName`, `Delete` and `DeleteWaiting`. A write takes `(ctx, *store.Tx, id, ...)` and a read `(ctx, Querier, ...)`. | the store, the contract |
| 2, `transport` | `SendTimeout = 10 * time.Second`, `ErrNoDeadline`, `WithWriteDeadline(next http.Handler) http.Handler`, `Send(ctx context.Context, send func() error) error` | item 6 of `go-s2-server-signin` |
| 3, `machines` | `TokensTopic = "client_join_tokens"`, `TokenLifetime = 15 * time.Minute`, `Issuer`, `TokenOptions{HostsServer}`, `NewToken`, `ErrInvalidAddress`, `ErrNoCertificatePin`, `Tokens` with `Create`, `Revoke` and `Prune`, `NewTokens`, `EncodeToken`, `DecodeToken`, `NormalizeAddress` | 1 |
| 5, `link` | the first change of `Migrations()` of the part `link`, `State`, `NewState(s *store.Store, r *machines.Registry)`, `Stream{MachineID, StreamID, ConnectionToken, Generation, Standing, Mark, ServerTime}`, `ErrNotCurrent`, `ErrUnknownMachine`, and the `State` methods `ClearConnections`, `BecomeCurrent`, `Confirm`, `Heartbeat`, `Disconnect`, `Presence` and `Forget` | 1 |
| 4, `machines` and `link` | `Tokens.Redeem`, `Refusal` with its five values, `ErrTokenUnknown`, `ErrTokenExpired`, `ErrTokenRevoked`, `ErrTokenUsed`, `ErrMachineRemoved`, `RefusalOf(err error) (Refusal, bool)`, `ErrInvalidHostName`, `Joined`; `LinkRevision = 1`, `JoinRefusalKey = "Lgtm-Join-Refusal"`, `LinkRefusalKey = "Lgtm-Link-Refusal"`, `MaxMessageBytes = 64 << 10`, `Options{Store, Registry, Tokens, SigningKey, Log}`, `Service`, `NewService`, `HandlerOptions() []connect.HandlerOption` | 3, 5 |
| 6, `link` | the second change of `Migrations()`, `OutboxTopic = "machine_outbox"`, `CommandGrace = 24 * time.Hour`, `CommandRetention = 7 * 24 * time.Hour`, and the `State` methods `Enqueue`, `Claim`, `AcceptResult` and `SweepCommands`. From here `*State` satisfies `machines.Link`. | 5 |
| 7, `link` | `EventResult{Mark, Applied}`, `ErrEventGap`, `ErrEventNumberZero`, `ErrInvalidMessage`, `State.ApplyEvent` | 5 |
| 9, `link` | `Hub` with `Register`, `Unregister`, `Wake`, `End` and `Follow`, `NewHub`, `Handle` with `Woken` and `Ended`, `ErrSuperseded` | 1, 6 |
| 11, `machines` | `ServiceOptions{Store, Follower, Registry, Tokens, Link, Sessions, Log}`, `Service`, which embeds `lgtmv1connect.UnimplementedClientServiceHandler`, `NewService`, `Project(rec Record, p *Presence) *lgtmv1.Client` | 1, 2, 3, 5, 6 |
| 8, `link` and `link/machinetest` | `HandshakeTimeout = 10 * time.Second`, `ServerProofText`, `MachineProofText`, `Seams{AfterProof, AfterAdmission}`, the fields `Follower`, `State`, `Hub`, `BehindProxy` and `Seams` of `Options`, `Service.Open`; the fixture `Machine{ID, Key, ServerKey}`, `Conn`, `NewServer`, `Join`, `Machine.Dial`, `Machine.Connect`, `Conn.Send`, `Conn.Receive`, `Conn.Handshake` and `Conn.Close` | 2, 4, 5, 9 |
| 12, `machines` | `Service.CreateClientToken`, `RevokeClientToken` and `RemoveClient` | 3, 5, 6, 11 |
| 13, `machines` | `Service.ApproveClient`, `RenameClient`, `ScanClientHarnesses` and `ReconnectClient` | 6, 11 |
| 10, `link` | `HeartbeatTimeout = 30 * time.Second`, `Service.Run` | 2, 6, 7, 8, 9 |
| 14, `api`, `app` and `servertest` | `api.Options.LinkOptions []connect.HandlerOption`, `app.Hooks{LinkSeams link.Seams}`, `servertest.Options.Log`, `servertest.Options.LinkSeams`, `servertest.LinkSeams`, `Server.Clients`, `Server.Link` | 4, 10, 11, 12, 13 |

The names that other teams call have these exact signatures.

```go
// Package machines. Written by this plan; the command `lgtm-server join-token` of go-s3-stack-client calls these.
type Issuer struct{ PublicURL string; ServerKey ed25519.PublicKey }
type TokenOptions struct{ HostsServer bool } // no address (architect-r1 09, decision 53)
type NewToken struct{ ID, Text, Address string; ExpiresAt time.Time }
var ErrInvalidAddress, ErrNoCertificatePin error
func NewTokens(s *store.Store, r *Registry, issuer Issuer) *Tokens
func (t *Tokens) Create(ctx context.Context, o TokenOptions) (NewToken, error)

// Package servertest. The orchestrator's tests in src/linktest call these.
type LinkSeams struct{ AfterProof, AfterAdmission func(ctx context.Context, machineID string) }
// Options gains: Log io.Writer; LinkSeams LinkSeams.
func (s *Server) Clients(t testing.TB) lgtmv1connect.ClientServiceClient // signed in
func (s *Server) Link() linkv1connect.LinkServiceClient
```

**Tables and topics.** Part `machines` owns `machines` and `client_join_tokens`. Part `link` owns `link_machines`, `link_connections` and `link_commands`. Every table sits in the schema of the application's connection, which is `public` in the stack. The feed topics are `machines`, `machine_outbox` and `client_join_tokens`, each with an empty scope (decision 30).

**Log lines.** One JSON object per line through `log/slog`: `link_event_applied` with `machine_id` and numeric `sequence` after each commit that moved a mark; `link_stream_opened` with `machine_id` and `generation`; `link_stream_closed` with `machine_id` and `code`. No line holds a token, a secret, a nonce, a signature or a connection token (R9). A line is written after its commit, so across a replaced stream two lines of one machine can swap, and a process death after a commit can lose one. The mark and the stored effect are the record, and the log only corroborates them (architect-r1 07, threat-r1 07).

**Codes and sentences.** Each error is a Connect code with one fixed sentence (R8). A refusal's reason goes into the metadata of the `connect.Error` with `err.Meta().Set`, never into a header written before the error. Connect then carries it as a response header of `Join` and in the end-of-stream message of `Open`, and gRPC (Google Remote Procedure Call) as a trailer.

| Call | Condition | Code | Reason key and value | Sentence |
|---|---|---|---|---|
| page | Unknown machine identifier | `not_found` | none | No machine has that id. |
| page | Name empty, too long or with a control character | `invalid_argument` | none | A machine needs a name of 1 to 64 characters. |
| page | Name taken | `already_exists` | none | Another machine already has that name. |
| page | Approve a machine that does not wait | `failed_precondition` | none | This machine does not wait for approval. |
| page | Rename a machine that waits | `failed_precondition` | none | Approve this machine before you rename it. |
| page | Restart a machine that is not connected or not approved | `failed_precondition` | none | This machine is not connected. |
| page | Scan a machine that is not online | `failed_precondition` | none | This machine is not online. |
| page | A request key on `CreateClientToken` | `invalid_argument` | none | This call does not accept a request key. |
| page | The database is busy | `aborted` | none | The database was busy, retry the request. |
| both | The database is unavailable, and every store error of `Join` and `Open` | `unavailable` | none | The server cannot reach its database, try again. |
| `Join` | A refusal | `permission_denied` | `Lgtm-Join-Refusal`, the `Refusal` that `RefusalOf` gives | The sentence of the `Refusal`, below. |
| `Join`, `Open` | A value rule broken, or a message out of order before `Welcome` | `invalid_argument` | none | The link message broke a rule of the contract. |
| `Open` | Unknown or removed machine, after `Challenge` | `failed_precondition` | `Lgtm-Link-Refusal: unknown_machine` | The server does not know this machine. Join it again. |
| `Open` | Link revision other than 1, after `Challenge` | `failed_precondition` | `Lgtm-Link-Refusal: link_revision_mismatch` | The server speaks revision 1 of the link. |
| `Open` | An event above the mark plus one | `failed_precondition` | `Lgtm-Link-Refusal: event_gap` | The event leaves a gap after the server's mark. |
| `Open` | A bad signature | `unauthenticated` | none | The machine's signature did not verify. |
| `Open` | `Hello` or `Proof` late, or 30 seconds without a heartbeat, while the response can be written | `deadline_exceeded` | none | The machine did not answer in time. |
| `Open` | A newer stream of the same machine | `aborted` | none | A newer connection of this machine replaced this one. |
| `Open` | The server stops | `unavailable` | none | The server is stopping. |

The refusals of `Join` have these sentences: `token_unknown` "The server does not know this join token.", `token_expired` "This join token has expired. Make a new one.", `token_revoked` "This join token was revoked. Make a new one.", `token_used` "This join token was already used. Make a new one.", and `machine_removed` "The machine that used this join token was removed. Make a new token."

**WS-1: The owner makes a join token, and a machine joins with it**

This workstream gives the server its record of machines and its join tokens, and it answers `Join`. It implements the design's section "Joining: a token that authorizes an exchange of keys" and checks C5 and C6.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1 | A machine record keeps its key, standing, facts and harnesses, and its lock holds off a removal. | `TestCreateWaitingAndRead`, `TestApplyFactsAndHarnesses`, `TestApproveRenameAndNames`, `TestLockHoldsOffRemoval` and `TestDeleteWaitingSparesApproved` pass. |
| 2 | A token carries the contract's fields and a loopback address, lasts exactly 15 minutes, and is stored only as a verifier. | `TestCreateTokenText`, `TestTokenLifetime`, `TestTokenStoresOnlyVerifier` and `TestTokenAddressRules` pass. |
| 3 | `Join` makes one waiting machine, answers a lost reply with the same machine, and refuses every other request with the reason in the error's metadata. | `TestRedeemCreatesWaitingMachine`, `TestRedeemReplay`, `TestRedeemRefusals`, `TestRedeemConcurrentUses`, `TestJoinCall` and `TestJoinRefusalMetadata` pass. |
| 4 | On the stack, a used and an expired token are refused, and the joined machine appears as waiting. | `deploy/test/checks.sh --only C5,C6` prints two lines that start with `ok` and exits 0, with the client of `go-s3-client-link`. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 1 | Write the machines table and its registry. | Target `src/server/internal/machines/store.go`, `Registry`, and `machines/migrations/0001_machines.sql`. Pattern: `src/server/internal/auth/sessions.go`, `Sessions`, of `go-s2-server-signin`. Scouting S5 and S12. |
| 3 | Write token making, revoking and pruning. | Target `src/server/internal/machines/tokens.go`, `Tokens.Create`, and `machines/migrations/0002_client_join_tokens.sql`. Pattern: `src/mock-server/src/engine.ts`, `createClientToken`. Scouting S4. |
| 4 | Write the redemption and the `Join` call. | Targets `src/server/internal/machines/join.go`, `Tokens.Redeem`, `src/server/internal/link/join.go`, `Service.Join`, `link/service.go`, `NewService`, and `link/rules.go`. Pattern: `src/server/internal/auth/sessions.go`, `Sessions.Issue`. Scouting S2. |

**Item 1. Machine records.** Creates `src/server/internal/machines/store.go`, `machines/link.go` (the `Link` interface and `Presence`), `machines/migrations/0001_machines.sql` and `machines/store_test.go`. Needs `go-s1-server-store` and `go-s0-contract`. Runs beside item 2. Medium: one table and its functions on the pattern of the sessions table. Statements: D3, D5, D7, R1, R3, R4, R7, R10.

- **Risky:** it keeps the key that proves a machine and holds the lock of decision 40. Weakening `Lock` to `FOR SHARE` must fail `TestLockHoldsOffRemoval`. Dropping `approval = 'waiting'` from `DeleteWaiting` must fail `TestDeleteWaitingSparesApproved`.
- **The table.**

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

- **Behaviour.** `CreateWaiting` makes the identifier with `store.NewID`, inserts approval `waiting` and revision 1, and records `machines` `saved`. `Get` returns `ErrNoMachine` for an unknown identifier. `List` orders by `joined_at`, then `id`. `Lock` reads the row `FOR NO KEY UPDATE` and returns `ErrNoMachine` when it is gone. `ApplyFacts` writes the deterministic binary encoding of a new `Facts` built from its declared fields only. `ApplyHarnesses` does the same for `HarnessesFound` and each of its entries, and sets `harnesses_scanned`. A field that this server does not know is therefore never stored (threat-r2 05). The event's `happened_at` is not stored, because no field of `Client` shows it (logic-r2 06, architect-r2 10). Stored as bytes, every value the wire allows stays as sent, including a count above the signed range of PostgreSQL and a zero byte in a string (architect-r1 06, critic-r1 10, logic-r1 7, threat-r1 06). `NoteSeen` writes `last_seen_at` and `last_address` and records the change. `Touch` only records `machines` `saved`, so the link never writes this part's topic itself (R3). `Link` has `Presence(ctx, Querier, ids []string) (map[string]Presence, error)`, `Forget(ctx, *store.Tx, id string) error` and `Enqueue(ctx, *store.Tx, id string, c *linkv1.Command, ttl time.Duration) error`; item 5 writes the first two, item 6 the third, and items 11 to 13 call them. `SetApproved` refuses a row that is not waiting with `ErrNotWaiting` and sets the name, `approved_at` and the next revision. `SetName` returns `false` for the same name, and otherwise sets it and the next revision. A unique violation of `machines_approved_name` becomes `ErrNameTaken`. `Delete` deletes the row, records `removed`, and returns `ErrNoMachine` when nothing matched. `DeleteWaiting` deletes only `WHERE approval = 'waiting'`, so in read committed mode it re-checks the row after a concurrent approval commits. `NormalizeName` trims spaces and refuses an empty result, more than 64 bytes, invalid UTF-8 (Unicode Transformation Format, 8 bits) and a control character. `Standing` gives `APPROVAL_WAITING` or `APPROVAL_APPROVED`, the name (empty while waiting) and the revision. Every write that changes a row records a change of `machines`.
- **Tests written first,** in `store_test.go`. `TestCreateWaitingAndRead`: The record has a version 7 universally unique identifier (UUID), the key, host name, token identifier and `HostsServer` given, approval `waiting`, revision 1 and an empty name. The feed holds one `machines` `saved` row with that key. `Get` of a random identifier returns `ErrNoMachine`. A write that fails after `CreateWaiting` leaves no row and no feed row. `TestApplyFactsAndHarnesses`: `Get` returns the six facts unchanged, also with 4,294,967,295 cores, the largest `memory_bytes` and a zero byte in `workspace`. A marked text in an unknown field of `Facts` and of a harness entry, built with `protowire`, is in no stored byte and no feed row. `claude` at `2.4.1` and `codex` without a version come back in that order, `codex` without a version, and `HarnessesScanned` is true. Each call adds one feed row. `TestApproveRenameAndNames`: `SetApproved` gives revision 2, and a second one returns `ErrNotWaiting`. Approving another machine as the same name in other letter case returns `ErrNameTaken` and leaves it waiting. A waiting host name equal to an approved name is no conflict. `SetName` to the same name returns `false` and keeps the revision, and a new name gives revision 3. `NormalizeName` turns `" Box "` into `Box` and refuses the empty string, 65 bytes, a tab and invalid UTF-8. `TestLockHoldsOffRemoval`: Transaction A calls `Lock` and waits on a gate. `Delete` in transaction B has not returned after 250 milliseconds, and `pg_locks` shows B waiting for A. A second `Lock` in transaction C also waits for A. After A commits, B returns. `TestDeleteWaitingSparesApproved`: Transaction A approves a waiting machine and waits on a gate. `DeleteWaiting` in B waits, and after A commits it returns `false` and the machine remains. `DeleteWaiting` of a waiting machine returns `true` and records `removed`.

- **Acceptance:** `go test -count=1 -run 'TestCreateWaiting|TestApplyFacts|TestApproveRename|TestLockHolds|TestDeleteWaiting' ./src/server/internal/machines/` passes.

**Item 3. Join tokens: make, revoke and prune.** Creates `src/server/internal/machines/tokens.go`, `machines/migrations/0002_client_join_tokens.sql` and `machines/tokens_test.go`. Needs item 1. Runs beside item 5. Medium: one table and one encoding that the contract fixes. Statements: D7, R4, R7, R9, C5.

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
- **`Revoke`** locks the token's row `FOR UPDATE`. An unknown identifier is no error. It sets `revoked_at` when empty, records a change, and returns `used_by`. The caller then calls `Registry.DeleteWaiting` and, when that deleted the machine, `Link.Forget`, in the same transaction.
- **`Prune`** deletes tokens whose expiry passed more than 24 hours ago, except a token whose `used_by` machine still waits, at most 200 to a write.
- **Tests written first,** in `tokens_test.go`. `TestCreateTokenText`: With `PublicURL` `http://127.0.0.1:8787`, the text decodes to a token whose identifier equals `NewToken.ID`, with a 32-byte secret, the address `http://127.0.0.1:8787`, which also equals `NewToken.Address`, `server_key_sha256` equal to SHA-256 of the issuer's key, and an empty pin. `EncodeToken` of the message in `src/contracts/testdata/join-token.json` gives that file's text, and `DecodeToken` gives back its fields. `TestTokenLifetime`: `expires_at - created_at` is exactly 15 minutes, and `NewToken.ExpiresAt` equals the row's `expires_at`. `TestTokenStoresOnlyVerifier`: `secret_sha256` equals SHA-256 of the label, a zero byte and the secret. The secret, in raw bytes, hexadecimal, base64 or base64url, appears in no column read with `row_to_json` and in no feed row. `TestTokenAddressRules`: `HTTP://127.0.0.1:8787/` gives `http://127.0.0.1:8787`, and `http://[::1]:80` gives `http://[::1]`. `http://example.test:8787`, `ftp://127.0.0.1` and an address with a path return `ErrInvalidAddress`. `https://box.example` returns `ErrNoCertificatePin`. An issuer with each refused address makes `Create` fail and insert no row. `TestHostsServerMark`: A token made with `HostsServer` true has `hosts_server` true in its row, and one without has false. `TestRevokeAndPrune`: Revoking an unused token sets `revoked_at` and returns an empty string. An unknown identifier returns no error. A token used by a machine returns that machine. Prune deletes a token that expired 25 hours ago, keeps one that expired 23 hours ago, and keeps one that expired 25 hours ago whose machine still waits.

- **Acceptance:** `go test -count=1 -run 'TestCreateTokenText|TestTokenLifetime|TestTokenStoresOnlyVerifier|TestTokenAddressRules|TestHostsServerMark|TestRevokeAndPrune' ./src/server/internal/machines/` passes.

**Item 4. Joining: redeem a token, and the `Join` call.** Creates `src/server/internal/machines/join.go`, `src/server/internal/link/service.go`, `link/join.go`, `link/rules.go`, `machines/join_test.go` and `link/join_test.go`. Needs items 3 and 5. Runs beside items 6, 7 and 11. Medium: one transaction with a fixed order of checks, and one thin handler. Statements: D7, R4, R8, R9, R10, C5, C6.

- **Risky:** it decides who may register a key. Checking the expiry after the replay must fail `TestRedeemRefusals`. Comparing the digests with `bytes.Equal` must fail `TestSecretCompareIsConstantTime`. Writing the reason as a header before the error must fail `TestJoinRefusalMetadata`.
- **What `Redeem` does,** in one `Store.Write`. It locks the token's row `FOR UPDATE`, then reads `clock_timestamp()`. A second use that waited on the lock therefore decides against the state and the clock after the first use committed. It then decides in the contract's order:
  1. No row, or a verifier that `crypto/subtle.ConstantTimeCompare` finds different, gives `RefusalUnknown`.
  2. `expires_at` at or before the clock gives `RefusalExpired`.
  3. `revoked_at` set gives `RefusalRevoked`.
  4. `used_key` set to another key gives `RefusalUsed`.
  5. `used_key` equal to the key gives `Joined{used_by, Replay: true}` and writes nothing when `used_by` still exists, and `RefusalMachineRemoved` otherwise.
  6. Otherwise it calls `Registry.CreateWaiting` with the token's `hosts_server`, sets `used_at`, `used_key` and `used_by`, and records `client_join_tokens`.

  Step 6 alone checks the host name: valid UTF-8, at most 255 bytes, and no zero byte, which PostgreSQL text cannot hold. A broken one returns `ErrInvalidHostName`. A replay never reads the host name (threat-r1 10). Each refusal returns its own sentinel, unwrapped, so the store passes it out and the write commits nothing (critic-r1 01).
- **What `Service.Join` does.** `rules.go` checks the value rules of the contract: a canonical UUID, a 32-byte secret, and a 32-byte key. A broken rule, or `ErrInvalidHostName`, gives `invalid_argument` and touches no row. An error for which `RefusalOf` gives a reason gives `permission_denied` with its sentence and the metadata `Lgtm-Join-Refusal`. A store error gives `unavailable`. Success gives `JoinResponse{machine_id, server_public_key}`. Until item 8, `link.Service` embeds `linkv1connect.UnimplementedLinkServiceHandler`, so `Open` answers `unimplemented`. `HandlerOptions` sets `connect.WithReadMaxBytes(MaxMessageBytes)` for `Join` and `Open`.
- **Tests written first,** in `machines/join_test.go`. `TestRedeemCreatesWaitingMachine`: A fresh token, its secret, a key and the host name `box` give `Replay` false and a waiting machine with revision 1, host name `box`, the token's identifier and its `hosts_server`. The token row holds the key, the machine and `used_at`. `TestRedeemReplay`: The same token, secret and key with the host name `other` give the same machine and `Replay` true. One machine exists, still named `box`. `TestRedeemRefusals`: Each case starts from its own token and leaves the machines and the token row unchanged: an unknown identifier, a wrong secret, and a wrong secret on an expired token each give `token_unknown`; an expired token gives `token_expired`; a revoked token gives `token_revoked`; a revoked token that also expired gives `token_expired`; a used token with another key gives `token_used`; the first key after its machine was deleted gives `machine_removed`; a replay after expiry gives `token_expired`. `TestRedeemConcurrentUses`: Two first uses with two keys, held at a gate, give one machine and one `token_used`. Two identical first uses give the same machine to both. When the test sets the expiry one second ahead and holds the second use until it passed, the second gets `token_expired`. `TestRedeemRollsBack`: A test trigger that fails the insert into `machines` makes `Redeem` return an error for which `RefusalOf` gives no reason, leaves no machine and an empty `used_key`, and after the trigger is dropped the same request succeeds. `TestSecretCompareIsConstantTime`: Parsing `join.go` with `go/parser` finds `subtle.ConstantTimeCompare` on the two digests and no `bytes.Equal` or `==` between them.

- **Tests written first,** in `link/join_test.go`, through h2c. `TestJoinCall`: A fresh token answers a version 7 `machine_id` and a `server_public_key` whose SHA-256 equals the token's `server_key_sha256`. The same request again answers the same identifier. `TestJoinRefusalMetadata`: For each of the five reasons, the Connect client gets `permission_denied`, the reason's sentence, and `Lgtm-Join-Refusal` in `connect.Error.Meta()`. The same holds over the gRPC protocol, as a trailer. No header, trailer or message holds part of the token's text or secret. `TestJoinValueRules`: An identifier in upper case, a secret of 31 bytes, and a key of 33 bytes each answer `invalid_argument` and change no row. A host name of 256 bytes, and one with a zero byte, answer `invalid_argument` on a first use and the first machine on a replay. `TestJoinDatabaseDown`: With the store closed, `Join` answers `unavailable`. `TestJoinKeepsSecretsOutOfLogs`: After a success and a refusal, the log holds neither the token's text nor its secret in hexadecimal, base64 or base64url.

- **Acceptance:** `go test -count=1 -run 'TestRedeem|TestSecretCompare' ./src/server/internal/machines/` and `go test -count=1 -run 'TestJoin' ./src/server/internal/link/` pass.

**Design artifact:** not applicable. No screen changes.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| A token works twice, for two keys at once. | High | The token's row is locked for the whole redemption. `TestRedeemConcurrentUses`. |
| A refusal tells a caller without the secret whether a token exists. | Medium | A wrong secret gives `token_unknown` before any other check. `TestRedeemRefusals`. |
| A token's secret reaches the database, a log or the feed. | High | Only the verifier is stored. `TestTokenStoresOnlyVerifier` and `TestJoinKeepsSecretsOutOfLogs`. |

**Verify:** `deploy/test/with-postgres.sh -- go test -race -count=1 ./src/server/internal/machines/... ./src/server/internal/link/...`. Environment check: `deploy/test/checks.sh --only C5,C6` at the end of the sprint. No device is needed.

**Rollout detail:** rollout steps 1 to 3. `Store.Migrate` applies both changes of the part `machines` at start-up, once item 14 registers them.

**WS-2: A machine proves itself and stays connected**

This workstream gives the server its half of the stream that every machine keeps open. It implements the design's sections "One connection, dialled by the machine", "Both ends prove themselves on every connection" and "Knowing whether a machine is there", and checks C8, C9 and C10.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 5 | The proof texts are the contract's bytes, and the decisions after `Challenge` follow the contract's order. | `TestProofTextsMatchContractExamples` and `TestHandshakeDecisionOrder` pass. |
| 6 | One admission makes a stream current, and nothing an older stream sends changes the machine after it. | `TestWritesOfOneMachineFormOneLine`, `TestOldStreamCannotChangeRecord`, `TestOlderGenerationCannotReplaceNewer` and `TestNewStreamReplacesOld` pass. |
| 7 | A removal that commits before an admission gives no `Welcome`, and a silent or blocked stream ends. | `TestRemovalBeforeAdmission`, `TestSilenceEndsStream` and `TestBlockedMachineEndsStream` pass. |
| 8 | On the stack, a stopped client shows offline within 30 seconds, a server restart brings it back, and a removed machine is refused. | `deploy/test/checks.sh --only C8,C9,C10` prints three lines that start with `ok` and exits 0. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 2 | Write the write deadline for long streams, and wrap the two route families. | Target `src/server/internal/transport/deadline.go`, `WithWriteDeadline`. Changes `src/server/internal/api/server.go`, `NewHandler`, of `go-s2-server-signin`. Pattern: `net/http.ResponseController`. Scouting S6 and S12. |
| 5 | Write the link's tables, admission, heartbeats and disconnection. | Target `src/server/internal/link/presence.go`, `State.BecomeCurrent`, and `link/migrations/0001_link.sql`. Pattern: `src/server/internal/store/receipts.go`, `WriteOnce`. Scouting S5. |
| 8 | Write the handshake and the scripted machine. | Targets `src/server/internal/link/handshake.go`, `ServerProofText`, `link/stream.go`, `Service.Open`, and `link/machinetest/machine.go`. Pattern: `src/contracts/open_test.go` of `go-s0-contract`. Scouting S2. |
| 9 | Write the hub of open streams and its feed follower. | Target `src/server/internal/link/hub.go`, `Hub.Register`. Pattern: `src/server/internal/store/follow.go`, `Follower`. Scouting S5. |
| 10 | Write the loop of one stream after `Welcome`, and the service's background work. | `src/server/internal/link/stream.go`, `Service.Open`, and target `link/run.go`, `Service.Run`. Scouting S2 and S8. |

**Item 2. Write deadlines for long streams.** Creates `src/server/internal/transport/deadline.go` and `transport/deadline_test.go`. Changes `src/server/internal/api/server.go`, so that `NewHandler` wraps the handlers of `ClientService` and `LinkService` in `transport.WithWriteDeadline`, and adds `TestRouterWrapsLongStreams` to `api/server_test.go`. Needs item 6 of `go-s2-server-signin`. Runs beside item 1. Low: one middleware of the standard library. Statements: R5, R7, C8, and the store plan's rule that the Machines adapter ends a blocked response at the transport. Not risky.

- **Behaviour.** `WithWriteDeadline` puts a setter of `http.NewResponseController(w).SetWriteDeadline` into the request's context. `Send` sets the deadline to now plus `SendTimeout`, calls `send`, and clears the deadline, also on failure. While `send` runs, an end of `ctx` sets the deadline to now through `context.AfterFunc`, so a cancelled watch or an expired stream ends a blocked write at once (architect-r1 11). `Send` returns the context's error without calling `send` when `ctx` has already ended. After `send` it calls the stop function of `context.AfterFunc` and clears the deadline only when the callback never ran, so a late callback cannot expire a later send. An expired deadline resets the HTTP/2 stream, so the peer of a blocked send sees a transport error and no status (architect-r2 05, threat-r2 04). Without the setter, `Send` returns `ErrNoDeadline` and does not call `send`. A private `sendTimeout` lets tests shorten the 10 seconds.
- **Tests written first:** `TestDeadlineEndsBlockedStream` (a handler that sends 64 kibibytes at a time to a client that stops reading gets an error from `Send` within 2 seconds at a timeout of 200 milliseconds, and returns); `TestDeadlineClearedAfterSend` (two sends 400 milliseconds apart to a reading client both succeed); `TestSendNeedsTheMiddleware` (`ErrNoDeadline`, and the function is not called); `TestSendEndsOnCancel` (at a send timeout of 10 seconds, a send blocked on a client that stops reading returns within 300 milliseconds of the end of its context, and a later `Send` on that context returns its error without calling its function); `TestRouterWrapsLongStreams` (probe handlers at `/lgtm.v1.ClientService/WatchClients` and `/lgtm.link.v1.LinkService/Open` get no error from `Send`).
- **Acceptance:** `go test -count=1 ./src/server/internal/transport/` and `go test -count=1 -run TestRouterWrapsLongStreams ./src/server/internal/api/` pass.

**Item 5. The link's tables, admission and presence.** Creates `src/server/internal/link/presence.go`, `link/migrations/0001_link.sql` and `link/presence_test.go`. Needs item 1. Runs beside item 3. Medium: short writes under the lock of decision 40. Statements: D3, D7, R3, R4, R7, R10, C8, C9.

- **Risky:** it decides which stream counts. Dropping the condition on `stream_id` from `Heartbeat` must fail `TestOldStreamCannotChangeRecord`. Removing the call of `Registry.Lock` from `Heartbeat` must fail `TestWritesOfOneMachineFormOneLine`.
- **The tables.**

```sql
CREATE TABLE link_machines (machine_id text PRIMARY KEY,
    event_mark bigint NOT NULL DEFAULT 0 CHECK (event_mark >= 0), stream_generation bigint NOT NULL DEFAULT 0);
CREATE TABLE link_connections (machine_id text PRIMARY KEY, stream_id text NOT NULL UNIQUE, generation bigint NOT NULL,
    connected_at timestamptz NOT NULL, last_heard_at timestamptz NOT NULL, address text NOT NULL DEFAULT '',
    cpu_percent smallint CHECK (cpu_percent BETWEEN 0 AND 100), memory_percent smallint CHECK (memory_percent BETWEEN 0 AND 100),
    facts_received boolean NOT NULL DEFAULT false, harnesses_received boolean NOT NULL DEFAULT false);
```

- **Admission.** `BecomeCurrent` is the one moment at which a stream becomes current. In one `Store.Write` it calls `Registry.Lock`, where `ErrNoMachine` becomes `ErrUnknownMachine`. It inserts the `link_machines` row when it is missing and raises `stream_generation` by one. It makes a stream identifier, and for an approved machine a connection token of 32 random bytes in unpadded base64url, which it does not store, because no call of wave 1 presents it (decision 45). It upserts `link_connections` with the new stream, the generation, the address, both clock fields from `clock_timestamp()`, no load and both flags false. It then reads `event_mark` and `clock_timestamp()` for `Stream.Mark` and `Stream.ServerTime`. It calls `Registry.Touch`. Because every event of an older stream also runs under the machine's lock, the mark holds every event that the older stream committed, and no event of the older stream commits after it.
- **Writes of a stream.** `Heartbeat` and `Disconnect` call `Registry.Lock`, then work on the row `WHERE machine_id = $1 AND stream_id = $2`. `Heartbeat` writes the load and `last_heard_at` and calls `Touch`, and zero rows give `ErrNotCurrent`. `Disconnect` deletes the row with `RETURNING last_heard_at, address`, and when it deleted one it calls `NoteSeen` and returns `true`. It runs with its own context of 5 seconds, so it also runs after the stream's context ended. When the machine is gone, `Heartbeat` returns `ErrUnknownMachine` and `Disconnect` returns `false`.
- **`Confirm`** locks the machine and returns `ErrUnknownMachine` when it is gone, and `ErrNotCurrent` when `stream_generation` is no longer the stream's generation. It writes nothing (architect-r1 03, critic-r1 05, logic-r1 4, threat-r1 04).
- **After a restart.** `ClearConnections` deletes every `link_connections` row, one machine to a write in order of identifier, calling `NoteSeen` for each (architect-r1 05, architect-r2 01). The server calls it at start-up while it holds the instance lock of `go-s2-server-signin`, so no row of a dead process shows a machine online (decision 41).
- **Reads.** `Presence` returns the connection rows of the given machines. A row exists only for a current stream, and the stream's end deletes it, so no clock decides presence, and a fresh read and an open watch agree. UNREVIEWED MECHANISM: the wall-clock filter of round 1 is removed, because it disagreed with the monotonic watchdog and hid a row from a fresh read while an open watch still showed it (architect-r2 03, logic-r2 01, threat-r2 03, decision 65). `Forget` deletes the machine's rows of `link_machines` and `link_connections`, and item 6 adds `link_commands` (threat-r2 01). The caller has already deleted or locked the machine.
- **The address.** The private `peerAddress(remoteAddr string, h http.Header, behindProxy bool) string` takes the last entry of `X-Forwarded-For` when `behindProxy` is true, and the host of `remoteAddr` otherwise. A value that `net/netip.ParseAddr` refuses gives an empty string.
- **A test seam.** The private `afterLock func(ctx context.Context, op string)` of `State` runs right after `Registry.Lock` in each write, and is nil in production.
- **Tests written first,** in `presence_test.go`:

| Test | What it asserts |
|---|---|
| `TestBecomeCurrent` | An approved machine gets generation 1, a stream identifier, a connection token of 43 characters, mark 0, its standing and a `ServerTime` within one second of the database's clock. A waiting machine gets an empty token. A second and a third admission give generations 2 and 3 (logic2-r1 5). An unknown machine gives `ErrUnknownMachine` and writes no row. |
| `TestWritesOfOneMachineFormOneLine` | A table of writes, each run in two cases against stream A. Case one: A's write holds at `afterLock`, and B's `BecomeCurrent` has not returned after 250 milliseconds; after A is released, B becomes current. Case two: B's `BecomeCurrent` holds at `afterLock`, and A's write waits; after B commits, A's write changes nothing and gets its own stale result: `ErrNotCurrent` from `Heartbeat`, `Confirm`, `Claim` and `ApplyEvent`, and `false` with no error from `Disconnect` and `AcceptResult` (architect-r2 01, logic-r2 04). Item 5 writes the rows of `Heartbeat`, `Disconnect` and `Confirm`. Item 7 adds `ApplyEvent` of 41 at mark 40, where B's mark is 41 in case one and 40 in case two, which is the contract's post-build check 23. |
| `TestOldStreamCannotChangeRecord` | After B became current, `Heartbeat` with A returns `ErrNotCurrent` and leaves B's row unchanged, and `Disconnect` with A returns `false`. |
| `TestHeartbeatAndDisconnect` | A heartbeat with 7 and 28 stores both and a newer `last_heard_at`, and adds a `machines` feed row. One without load stores nulls, and one with `cpu_percent` 0 and no memory stores 0 and null (logic2-r1 6). `Disconnect` of the current stream removes the row, sets `LastSeenAt` and `LastAddress`, and `Presence` then has no entry. |
| `TestClearConnections` | Three rows are cleared in three writes, each machine's `LastSeenAt` is set, and one `machines` feed row per machine is added. |
| `TestConfirm` | After an admission `Confirm` returns nil. After a removal it returns `ErrUnknownMachine`. After a newer admission whose stream then disconnected, it returns `ErrNotCurrent`. |
| `TestForget` | After `Forget`, the machine has no row in `link_machines` or `link_connections`. Item 6 adds `link_commands` to this test. |
| `TestPeerAddress` | Behind the proxy, `203.0.113.9, 198.51.100.4` gives `198.51.100.4`. Otherwise the header is ignored and `127.0.0.1:5555` gives `127.0.0.1`. A value that is not an address gives an empty string. |

- **Acceptance:** `go test -count=1 -run 'TestBecomeCurrent|TestWritesOfOneMachine|TestOldStream|TestHeartbeatAndDisconnect|TestClearConnections|TestConfirm|TestForget|TestPeerAddress' ./src/server/internal/link/` passes.

**Item 8. The handshake.** Creates `src/server/internal/link/handshake.go`, `link/stream.go`, `link/machinetest/machine.go`, `link/handshake_test.go` and `link/machinetest/machine_test.go`. Changes `link/service.go` to hold the hub, the state and the seams. Needs items 2, 4, 5 and 9 (architect-r1 02). Runs alone, after item 9. Medium: bytes fixed by worked examples and a short state machine. Statements: D7, D17, R8, R10, C10.

- **Risky:** it proves both ends. Building the server's text with the two nonces swapped must fail `TestProofTextsMatchContractExamples`. Checking the revision before the machine must fail `TestHandshakeDecisionOrder`. Sending `Welcome` without `Confirm` must fail `TestRemovalBeforeAdmission`. Returning a fixed server nonce must fail `TestReplayedProofRefused`.
- **The service's lifetime.** `NewService` makes a context that only the end of `Run` cancels. `Open` derives its stream's context from the request and that context at its entry, so a stream held before or after its admission, and a seam that waits on its context, end with `unavailable` when the server stops, and a stream that registers after the stop has already ended (architect-r2 08, threat-r2 08).
- **The proof texts.** Each is the label, one zero byte, one byte with the length of the machine identifier, the identifier, the machine nonce and the server nonce. The labels are `lgtm-link-v1 server` and `lgtm-link-v1 machine`.
- **What `Open` does in this item.** A reader goroutine passes each received message, or its receive error, to a channel. An error from decoding a message ends the stream with `invalid_argument`.
  1. Within `HandshakeTimeout` of the start, the first message must be a `Hello` with a canonical identifier and a 32-byte nonce. Another kind, an empty message or a broken rule gives `invalid_argument`, and lateness gives `deadline_exceeded`.
  2. The server draws its 32-byte nonce and the stream identifier from `crypto/rand` through one private function (logic2-r1 1), signs `ServerProofText` and sends `Challenge` through `transport.Send`.
  3. It then decides in the contract's order, before or after `Proof` arrives: `Registry.Get` in `Store.Read` that fails gives `unavailable`; an unknown machine gives `unknown_machine`; `link_revision` other than 1 gives `link_revision_mismatch`; no `Proof` within `HandshakeTimeout` of the `Challenge` gives `deadline_exceeded`; a `Proof` that is not a 64-byte signature gives `invalid_argument`; a signature that `ed25519.Verify` refuses against the stored key gives `unauthenticated`.
  4. It runs `Seams.AfterProof`, then `State.BecomeCurrent`, where `ErrUnknownMachine` gives `unknown_machine` and a store error gives `unavailable`. That commit is the admission.
  5. It runs `Seams.AfterAdmission`, registers with the hub (`ErrSuperseded` gives `aborted`), and calls `State.Confirm`, where `ErrUnknownMachine` gives `unknown_machine`, `ErrNotCurrent` gives `aborted`, and a store error gives `unavailable`. The commit of `Confirm` is the contract's moment at which the server admits the stream to send `Welcome`, and the machine's lock orders it against a removal and a newer admission (decision 64). It then sends `Welcome{connection_token, state = machines.Standing(Stream.Standing), last_event_number = Mark, server_time = ServerTime, link_revision = 1}` and logs `link_stream_opened`. The read of step 3 decides only the refusals (logic2-r1 3). A removal or a newer admission that commits after `Confirm` reaches the registered handle through the hub, so this `Welcome` is the contract's case of a `Welcome` in transit, followed by `unknown_machine` or `aborted`, and nothing the stream sends later changes a row (architect-r2 02, logic-r2 02, threat-r2 02). Every exit after the admission, also a failed `Confirm` or first send, calls `Disconnect` and `Hub.Unregister`.

  In this item the stream then waits for its end and calls `Disconnect` and `Hub.Unregister`. Item 10 replaces that wait with the loop.
- **The scripted machine.** `NewServer` serves a handler over h2c inside `transport.WithWriteDeadline` (architect-r2 01). `Join` decodes a token's text, makes a key, calls `Join`, checks the server key against the token, and returns a `Machine`. `Dial` opens a raw stream. `Handshake` sends `Hello` and `Proof` and verifies the `Challenge`. `Connect` runs both and fails the test on an error.
- **Tests written first,** in `handshake_test.go`, against a real `link.Service` and PostgreSQL. `TestProofTextsMatchContractExamples`: With the values of `src/contracts/testdata/link-proof.json`, both texts equal the file's texts, and the signature with its seed equals the file's server signature. `TestHandshakeOpensStream`: An approved machine gets a `Challenge` that verifies with the server's key, then a `Welcome` with revision 1, `APPROVAL_APPROVED`, its name and revision, `last_event_number` 0, a `server_time` and a token of 43 characters. A waiting machine gets `APPROVAL_WAITING`, an empty name and an empty token. With `Seams.AfterProof` holding while the test approves a waiting machine, its `Welcome` is approved at revision 2 with a token of 43 characters. `TestServerNonceFreshPerStream`: Two streams that send the same `Hello` get different `server_nonce` values. `TestReplayedProofRefused`: The `Proof` of stream one, sent on stream two after the same `Hello`, ends with `unauthenticated` and admits nothing. `TestHandshakeDecisionOrder`: Each refusal comes after a `Challenge` that verifies, and the same request again gets the same answer: a removed machine with revision 2 gets `failed_precondition` and `unknown_machine`; a known machine with revision 2 and a bad signature gets `link_revision_mismatch`; a known machine that sends no `Proof` gets `deadline_exceeded` at a private timeout of 200 milliseconds; a `Proof` of 63 bytes gets `invalid_argument`; a signature by another key, and one over a text with swapped nonces, get `unauthenticated`. With the store closed after the `Challenge`, the stream gets `unavailable`. The reason sits in the metadata of the terminal error over Connect and over gRPC. `TestHandshakeValueRules`: A nonce of 31 bytes, an identifier in upper case, an empty first message and a `Heartbeat` as the first message each get `invalid_argument` and no `Challenge`. A stream that sends nothing gets `deadline_exceeded`. `TestRemovalBeforeAdmission`: With `Seams.AfterProof` holding, the test removes the machine, then releases. The machine gets no `Welcome`, and the stream ends with `unknown_machine`. With `Seams.AfterAdmission` holding instead, the test removes the machine, waits until the hub's follower has consumed the removal, and releases. The machine gets no `Welcome`, and the stream ends with `unknown_machine`. This is the contract's post-build check 10. With the private hook `afterConfirm` holding instead, a removal gives the machine `Welcome`, then `unknown_machine`, and a heartbeat it sends after the `Welcome` changes no row; a newer admission there gives the older stream `Welcome`, then `aborted`. `TestStaleAdmissionCannotRegister`: Stream A holds at `Seams.AfterAdmission`. Stream B is admitted, sends `Welcome` and disconnects. A is released, gets no `Welcome`, and ends with `aborted`.

- **Tests written first,** in `machinetest/machine_test.go`: `TestScriptedMachineJoinsAndConnects` makes a token, joins, approves through `Registry.SetApproved`, and gets a `Welcome` from `Connect`.
- **Acceptance:** `go test -count=1 -run 'TestProofTexts|TestHandshake|TestServerNonce|TestReplayedProof|TestRemovalBeforeAdmission|TestStaleAdmission' ./src/server/internal/link/` and `go test -count=1 ./src/server/internal/link/machinetest/` pass.

**Item 9. The hub of open streams.** Creates `src/server/internal/link/hub.go` and `link/hub_test.go`. Needs items 1 and 6. Runs beside item 11. Medium: a map under one lock and one subscriber of the store's follower. Statements: R5, R6, R7, C9, C10. The hub is only a list of the process's live streams, and the database stays the record (R7).

- **Risky:** it orders streams. Letting `Register` replace a handle of a higher generation must fail `TestOlderGenerationCannotReplaceNewer`.
- **Behaviour.** `Register` returns `ErrSuperseded` when the machine's handle has a higher generation, and otherwise keeps the new handle and ends the older one with `aborted`. `Unregister` removes a handle only while it is still the machine's handle. `Wake` signals a channel of size 1 without blocking. `End` sends an error to one handle. `Follow` subscribes to the follower from the head that `Store.Snapshot` reads and wakes every handle once. A `machine_outbox` row then wakes its key's handle, and a `machines` `removed` row ends its key's handle with `failed_precondition` and `unknown_machine`. It acknowledges every row. On `store.ErrWatchLagged` or `store.ErrFeedUnavailable` it waits one second, subscribes again and wakes every handle. A woken stream claims, and `Claim` returns `ErrUnknownMachine` for a removed machine, so a removal that a lag skipped still ends its stream.
- **Tests written first,** in `hub_test.go`: `TestRegisterReplacesLowerGeneration` (the first handle ends with `aborted`, the second does not end within 250 milliseconds); `TestOlderGenerationCannotReplaceNewer` (`ErrSuperseded`, and generation 2 is not ended); `TestUnregisterOnlyOwnHandle`; `TestWakeCoalesces` (two wakes give one signal, and a wake of an unknown machine returns at once); `TestFollowWakesAndEnds` (with a real store, a `machine_outbox` row wakes the handle within 3 seconds, a `machines` `removed` row ends it with `unknown_machine`, and another topic does neither); `TestFollowResubscribes` (after a forced `store.ErrWatchLagged`, every handle wakes once).
- **Acceptance:** `go test -count=1 -run 'TestRegister|TestOlderGeneration|TestUnregister|TestWake|TestFollow' ./src/server/internal/link/` passes.

**Item 10. The stream after `Welcome`.** Changes `src/server/internal/link/stream.go`. Creates `link/run.go`, `link/stream_test.go` and `link/run_test.go`. Needs items 2, 6, 7, 8 and 9. Medium: it joins tested functions in one select loop with one writer. Statements: D7, D16, R6, R7, R10, C7, C8, C9, C11.

- **Risky:** it orders events and commands on the wire. Sending `EventAck` before `ApplyEvent` returns must fail `TestEventsAckedAfterCommit`. Sending without `transport.Send` must fail `TestBlockedMachineEndsStream`.
- **The loop.** One goroutine reads into a channel. The handler's goroutine is the only writer and sends everything through `transport.Send`. Before each command it handles every message already received, then claims one command, sends it, and claims again until a claim returns none, so a heartbeat waits for at most one send. UNREVIEWED MECHANISM: one command to a claim replaces the batch of 64, because the store's cap of 256 counts feed references, not rows, and a claim now records none (architect-r2 06, logic-r2 05, threat-r2 06, decision 66). It first sends pending commands, then selects:
  - on a message: `heartbeat` calls `State.Heartbeat` and sends `HeartbeatAck`, and a load above 100 gives `invalid_argument`; `event` calls `State.ApplyEvent` and then sends `EventAck{through: Mark}`; `command_result` calls `State.AcceptResult`; `hello` or `proof` gives `invalid_argument`; a message with no known kind is ignored, because wave 2 adds kinds;
  - on the handle's wake: send pending commands;
  - on the handle's end: return that error;
  - on the end of the stream's context: return its cause, `deadline_exceeded` from the watchdog or `unavailable` from the end of `Run`. The machine reads that status only while the response can be written, and after a blocked send it sees the reset of item 2.

  The reader goroutine resets the watchdog, a timer of `HeartbeatTimeout` on the monotonic clock, each time a heartbeat arrives. When the timer fires it cancels the stream's context, which ends a blocked `transport.Send` at once, and a later heartbeat renews nothing (architect-r1 04, critic-r1 04, logic-r1 3, threat-r1 03).

  `ErrNotCurrent` from any call returns `aborted`. `ErrUnknownMachine` returns `failed_precondition` with `unknown_machine`. `ErrEventGap` returns `failed_precondition` with `event_gap`. `ErrEventNumberZero` and `ErrInvalidMessage` return `invalid_argument`. On every return it calls `Disconnect` and `Hub.Unregister`, and logs `link_stream_closed`. A `Disconnect` that fails goes to `Run` (logic2-r1 2).
- **What `Run` does.** It runs `Hub.Follow`. Every second it calls `Disconnect` again for each stream whose `Disconnect` failed, until it returns without an error, `false` included. While the database refuses the delete, the page shows the machine online, and it shows it offline within a second after the database accepts it. Check C8 stops the client, which closes its stream, so this path is outside C8 (architect-r2 03, logic-r2 01, threat-r2 03). Every hour it runs `State.SweepCommands` and `Tokens.Prune`. A failed sweep writes one log line and tries again at the next tick. When its context ends it cancels the service's context of item 8, which ends every stream with `unavailable`.
- **Tests written first,** in `stream_test.go`, with the scripted machine:

| Test | What it asserts |
|---|---|
| `TestHeartbeatIsAnswered` | 7 and 28 get `HeartbeatAck` and show in `Presence`. 101 ends the stream with `invalid_argument`. |
| `TestSilenceEndsStream` | At a private timeout of 300 milliseconds, a silent machine that still reads gets `deadline_exceeded` within one second, and `Presence` shows it gone. A silent machine that stopped reading, with a send blocked, sees its stream reset within one second, the handler returns, and `Presence` shows it gone (architect-r2 05). |
| `TestFailedDisconnectIsRetried` | A test trigger fails the first delete of the connection row. The stream ends, and within 2 seconds after the trigger is dropped `Presence` shows the machine gone. A replacement stream admitted meanwhile keeps its row. |
| `TestEventsAckedAfterCommit` | Events 1 to 3 get `EventAck` 1 to 3, and the log holds `link_event_applied` with sequences 1, 2 and 3 once each. A repeat of 2 gets `EventAck` 3. Event 5 ends the stream with `event_gap`. A new stream that sends 4 and 5 gets 4 and 5 applied. With the commit gate holding the commit of event 6, no `EventAck` 6 arrives. Rejected at the commit, event 6 ends the stream with `unavailable`, and no `EventAck` 6 arrives. |
| `TestCommandsDeliveredAndAnswered` | A stored `ScanHarnesses` arrives with attempt 1 within 3 seconds, and a `COMMAND_OUTCOME_DONE` for attempt 1 is stored. A command left unanswered arrives on the next stream with attempt 2, the same identifier and the same expiry. |
| `TestNewStreamReplacesOld` | A second stream gets `Welcome` and the first ends with `aborted`. A heartbeat and a result sent late on the first change nothing, and `Presence` names the second. |
| `TestRemovalEndsStream` | A removal of a connected machine ends its stream with `failed_precondition` and `unknown_machine` within 3 seconds, also when the hub's follower is forced into `store.ErrWatchLagged` during the removal. |
| `TestApprovalReachesWaitingStream` | A connected waiting machine receives a `SetState` stored at its approval within 3 seconds. Before the approval, a stored `ScanHarnesses` did not arrive, and a `CommandResult` it sent was not stored. |
| `TestBlockedMachineEndsStream` | At a `transport` timeout of 200 milliseconds, a machine that stops reading after `Welcome`, with an HTTP/2 window of 1 kibibyte, while 300 commands are stored, sees its stream reset within 3 seconds, the handler returns, and `Presence` shows it gone. |
| `TestHeartbeatAnsweredDuringBacklog` | With 300 pending commands, the first 64 of them scans 25 hours past their expiry, and a machine that reads one message every 100 milliseconds and sends a heartbeat every 300 milliseconds, each `HeartbeatAck` arrives before the second command after its heartbeat, no expired scan is sent, and the other commands arrive in the order of `seq` (architect-r2 06, logic-r2 05). |
| `TestUnknownKindIgnored` | A message with no kind changes nothing, and a later heartbeat still gets `HeartbeatAck`. |

- **Tests written first,** in `run_test.go`: `TestRunSweeps` (at private intervals of 100 milliseconds, a scan 25 hours past its expiry becomes `expired` within one second); `TestRunStopsStreams` (cancelling `Run` ends a connected stream, a stream held at `Seams.AfterProof` and one held at `Seams.AfterAdmission` with `unavailable` within one second, and no connection row or handle remains; architect-r2 08, threat-r2 08).
- **Acceptance:** `go test -count=1 -run 'TestHeartbeatIsAnswered|TestHeartbeatAnswered|TestSilence|TestFailedDisconnect|TestEventsAcked|TestCommandsDelivered|TestNewStream|TestRemovalEnds|TestApprovalReaches|TestBlockedMachine|TestUnknownKind|TestRun' ./src/server/internal/link/` passes.

**Design artifact:** not applicable. No screen changes.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| An old stream writes after a newer one was admitted. | High | Every stream write takes the machine's lock and names its stream. `TestWritesOfOneMachineFormOneLine` and `TestOldStreamCannotChangeRecord`. |
| A machine that stops reading holds a handler forever. | High | Every send has a deadline of 10 seconds, and the watchdog ends it sooner. `TestBlockedMachineEndsStream` and `TestSilenceEndsStream`. |
| A server that died leaves its machines online. | Medium | Start-up clears every connection row under the instance lock. `TestClearConnections` and `TestStartMarksNoMachineOnlineByMemory`. |
| The web server in front cuts a quiet stream. | Medium | The client sends a heartbeat every 10 seconds and the server answers each. `go-s2-stack-web` proves a long stream through the web server. |

**Verify:** `deploy/test/with-postgres.sh -- go test -race -count=1 ./src/server/internal/link/... ./src/server/internal/transport/... ./src/server/internal/api/...`. Environment check: `deploy/test/checks.sh --only C8,C9,C10`, and the orchestrator's tests in `src/linktest/` with the real client. No device is needed.

**Rollout detail:** rollout steps 1, 2, 4, 5 and 6.

**WS-3: Commands go down once, and reports come up once**

This workstream stores every command before it is sent, accepts only the answer to the newest attempt, and applies every report once and in order. It implements the design's sections "Commands go down, and none is lost or repeated" and "Events come up, and none is lost or applied twice", rule R6, and check C11.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 9 | A command is stored before it is sent, each attempt is stored before it leaves, only the newest attempt's answer counts, and `SetState` never expires. | `TestEnqueueStoresBeforeSend`, `TestClaimRaisesAttemptOncePerStream`, `TestClaimCommitsBeforeReturn`, `TestAcceptResultRules` and `TestSweepCommands` pass. |
| 10 | Each event is decided by the contract's table, with its effect and its number in one commit. | `TestEventTable` and `TestEffectAndMarkCommitTogether` pass. |
| 11 | On the stack, reports kept during an outage arrive once and in order. | `deploy/test/checks.sh --only C11` prints one line that starts with `ok` and exits 0. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 6 | Write stored commands. | Target `src/server/internal/link/outbox.go`, `State.Claim`, and `link/migrations/0002_link_commands.sql`. Pattern: `src/server/internal/store/receipts.go`, `WriteOnce`. Scouting S2 and S5. |
| 7 | Write incoming events. | Target `src/server/internal/link/inbox.go`, `State.ApplyEvent`. Pattern: the same. Scouting S2 and S9. |

**Item 6. Stored commands.** Creates `src/server/internal/link/outbox.go`, `link/migrations/0002_link_commands.sql` and `link/outbox_test.go`. Changes `State.Forget` to delete the machine's commands, and adds `link_commands` to `TestForget`. Needs item 5. Runs beside items 4 and 7. Medium: one table and four functions under the machine's lock. Statements: D16, R4, R6, R7, C7.

- **Risky:** it orders commands. Accepting a result for an attempt older than the stored one must fail `TestAcceptResultRules`. Returning the claimed commands before the commit must fail `TestClaimCommitsBeforeReturn`. Expiring a `SetState` must fail `TestSweepCommands`. Dropping `machine_id` and `sent_stream_id` together from the update of `AcceptResult` must fail `TestAcceptResultRules` (logic-r2 09).
- **The table.**

```sql
CREATE TABLE link_commands (id text PRIMARY KEY, machine_id text NOT NULL,
    action text NOT NULL CHECK (action IN ('scan_harnesses', 'set_state', 'restart')), payload bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(), expires_at timestamptz NOT NULL,
    attempt integer NOT NULL DEFAULT 0 CHECK (attempt >= 0), sent_stream_id text, finished_at timestamptz,
    seq bigint GENERATED ALWAYS AS IDENTITY,
    outcome text CHECK (outcome IN ('done', 'failed', 'expired', 'unsupported', 'interrupted')),
    CHECK ((outcome IS NULL) = (finished_at IS NULL)));
CREATE INDEX link_commands_pending ON link_commands (machine_id, seq) WHERE outcome IS NULL;
```

- **`Enqueue`** runs in the caller's transaction, which has already locked or written the machine. It reads only the action of the given `Command`, makes the identifier, stores the action's name and the binary `Command` with only its action set, sets `expires_at = clock_timestamp() + ttl` and attempt 0, and records `machine_outbox`, whose one reader is the hub. No other write of commands records anything. A `Command` without an action returns an error.
- **`Claim`,** in one write, calls `Registry.Lock`, where a missing machine gives `ErrUnknownMachine`, and checks that the stream is current, or returns `ErrNotCurrent`. It first marks `expired` every pending command other than `set_state` whose `expires_at + CommandGrace` is before `clock_timestamp()` (threat-r1 05). It then takes the one pending command of lowest `seq` whose `sent_stream_id IS DISTINCT FROM` this stream, only a `set_state` for a waiting machine, raises its attempt by one and sets `sent_stream_id`. After the commit it returns that `Command` with the identifier, the new attempt, the stored expiry and the stored action, or none. Every `Enqueue` runs under the machine's lock, so `seq` is the order of enqueueing for one machine whatever the clock does, and a `SetState` stored at approval leaves before any command stored after it (logic-r1 5).
- **`AcceptResult`** reads only `command_id`, `attempt` and `outcome`, and keeps no other field of the message. A non-canonical identifier or attempt 0 gives `ErrInvalidMessage`. In one write it calls `Registry.Lock` and checks that the stream is current, and returns `false` otherwise. A waiting machine's result returns `false`. It updates only `WHERE id = $1 AND machine_id = $2 AND sent_stream_id = $3 AND attempt = $4 AND outcome IS NULL`, with this stream's machine and identifier, so a result closes only a command that this stream received last (architect-r1 01, critic-r1 07, logic-r1 1, threat-r1 02). Zero rows return `false` and write nothing. Otherwise it stores the outcome and `finished_at` and returns `true`. `COMMAND_OUTCOME_UNSPECIFIED`, an unknown value, and `COMMAND_OUTCOME_EXPIRED` for a `set_state` are stored as `failed`.
- **A final outcome.** Every write of an outcome also sets `finished_at`, and the table's check refuses one without the other. A matching result wins while its row is pending, also after the cutoff, and the cutoff marks only a row that is still pending (logic-r2 10).
- **`SweepCommands`** works one machine to a write, under `Registry.Lock`. It marks `expired` by the rule of `Claim`, and deletes every command that finished more than `CommandRetention` ago. A `SetState` stays pending until it has a result.
- **A test seam.** The private `afterClaim func(context.Context, *store.Tx) error` runs before the claim's commit, and is nil in production.
- **Tests written first,** in `outbox_test.go`:

| Test | What it asserts |
|---|---|
| `TestEnqueueStoresBeforeSend` | A `ScanHarnesses` for 2 minutes has attempt 0, an expiry 2 minutes after the database's clock, and one `machine_outbox` feed row. When the caller's write fails, neither exists. |
| `TestClaimRaisesAttemptOncePerStream` | `Claim` on stream A returns attempt 1 and the stored expiry, and a second `Claim` on A returns nothing. On stream B it returns the same identifier with attempt 2 and the same expiry. |
| `TestClaimCommitsBeforeReturn` | With `afterClaim` failing, `Claim` returns the error and no command, and the attempt stays 0. With the commit gate holding the commit, `Claim` has not returned. Rejected at the commit, the attempt is unchanged. |
| `TestClaimOrder` | Two commands whose `created_at` the test sets in reverse order leave in the order of enqueueing, one to a claim, and a claim with none left returns none and adds no feed row. A scan whose expiry the test sets 25 hours back is not sent and becomes `expired` with `finished_at`, and a `SetState` 25 hours past its expiry is sent. |
| `TestClaimForWaitingMachine` | A waiting machine with a pending scan and a pending `SetState` gets only the `SetState`. An old stream gets `ErrNotCurrent`. |
| `TestAcceptResultRules` | For a command sent as attempt 2 on the current stream: attempt 2 returns `true` and stores `done`; a copy returns `false`; on a fresh command, attempts 1 and 3 store nothing; an old stream's result and an unknown identifier return `false`; a result for this command from the current stream of a second approved machine, and one on a new stream of the same machine before it claimed the command, store nothing, and the command's own stream then claims and closes it; `COMMAND_OUTCOME_UNSPECIFIED` and the value 99 store `failed`; a result carrying a field 4 with a marked text built with `protowire` stores the outcome, and the marked text is in no row. A matching result after the cutoff, before any claim or sweep, stores its outcome, and after the sweep marked the row it returns `false`. A stored outcome is never claimed again. |
| `TestSweepCommands` | A scan 25 hours past its expiry becomes `expired`, and one 23 hours past stays pending. A `SetState` 25 hours past its expiry stays pending. A command finished 8 days ago, also one that a claim marked `expired`, is deleted, and one finished 6 days ago is kept. |

- **Acceptance:** `go test -count=1 -run 'TestEnqueue|TestClaim|TestAcceptResult|TestSweepCommands' ./src/server/internal/link/` passes.

**Item 7. Incoming events.** Creates `src/server/internal/link/inbox.go` and `link/inbox_test.go`. Needs item 5. Runs beside items 4 and 6. Medium: one transaction with the contract's table of decisions. Statements: R3, R4, R7, R9, C7, C11.

- **Risky:** it orders events. Moving the mark's update into a separate transaction must fail `TestEffectAndMarkCommitTogether`. Checking the value rules before the number must fail `TestEventTable`. Removing the call of `Registry.Lock` from `ApplyEvent` must fail `TestWritesOfOneMachineFormOneLine` (architect-r2 01).
- **What `ApplyEvent` does,** in one `Store.Write`. It calls `Registry.Lock`, where a missing machine gives `ErrUnknownMachine`, and checks that the stream is current, or returns `ErrNotCurrent`. It reads the mark and decides by the number alone, as `events.proto` says:
  1. Number 0 returns `ErrEventNumberZero`. The number is compared with the mark as `uint64`. A machine that reached 2^63 events is outside wave 1.
  2. A number at or below the mark returns `EventResult{Mark: mark}` and writes nothing, whatever the report holds.
  3. A number above the mark plus one returns `ErrEventGap` and writes nothing. Nothing remembers the gap, so a later stream that sends the missing number goes on.
  4. At the mark plus one, the private `policy(approval, report)` decides the effect. A valid `Facts` of an approved machine calls `Registry.ApplyFacts` and sets `facts_received` on this stream's row. A valid `HarnessesFound` of an approved machine calls `Registry.ApplyHarnesses` and sets `harnesses_received`. Every other event is consumed: no report, a report this server does not know, a report that breaks a rule of `events.proto`, and every report of a waiting machine. A rule of `Facts` is `os` and `client_version` of at most 64 bytes, `workspace` of at most 256 bytes, valid UTF-8, and `cores` of at least 1. A rule of `HarnessesFound` is at most 16 entries, each name of 1 to 32 bytes appearing once, and each present version of 1 to 64 bytes of valid UTF-8.
  5. It sets the mark to the number and calls `Registry.Touch`.

  After the commit it logs `link_event_applied` and returns `EventResult{Mark: number, Applied: true}`.
- **Tests written first,** in `inbox_test.go`:

| Test | What it asserts |
|---|---|
| `TestEventTable` | At mark 42 on the current stream: event 0 gives `ErrEventNumberZero`; a repeat of 42 whose `Facts` breaks a limit gives mark 42 and `Applied` false; event 44 with a broken report gives `ErrEventGap` and mark 42; event 43 with a broken `Facts` is consumed, the mark becomes 43, the record holds no new facts and `facts_received` stays false; a valid event 44 is then applied. An event 45 with no report moves the mark and nothing else. A valid event 46 without `happened_at`, and one 47 whose `happened_at` has seconds of −10^12, are applied. This is the contract's post-build check 17. |
| `TestApplyNextEvent` | At mark 0, a valid `Facts` as event 1 is stored in the record, sets `facts_received`, and logs one `link_event_applied` with the machine and sequence 1. A repeat logs nothing. |
| `TestEffectAndMarkCommitTogether` | A test trigger that fails the update of `machines` makes event 2 return an error with the mark still 1, and after the trigger is dropped event 2 is applied. With the commit gate rejecting the commit of event 3, its facts, the flag `facts_received`, the mark and the feed are all unchanged. |
| `TestWaitingMachineEventsConsumed` | A waiting machine's `Facts` moves the mark and leaves the record without facts and the flag false. |
| `TestOldStreamEventIgnored` | An event on a stream that is no longer current returns `ErrNotCurrent`, and the mark stays. |
| `TestHundredEventsInOrder` | Events 1 to 100 give mark 100 and 100 log lines with sequences 1 to 100 in order. Event 2^63 and the largest `uint64` at mark 100 give `ErrEventGap` with no store error, because the number is compared as `uint64` before it becomes a query argument (logic2-r1 7). |

- **Acceptance:** `go test -count=1 -run 'TestEventTable|TestApplyNextEvent|TestEffectAndMark|TestWaitingMachineEvents|TestOldStreamEvent|TestHundredEvents' ./src/server/internal/link/` passes.

**Design artifact:** not applicable. No screen changes.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| A confirmation deletes a report on the machine that the server never saved. | High | `EventAck` is sent only after `ApplyEvent` returns from its commit. `TestEventsAckedAfterCommit` fails the commit and sees no confirmation. |
| A command runs twice, or an old answer closes a newer attempt. | High | Each attempt is stored before it is sent, and only the stored attempt's answer counts. `TestAcceptResultRules`. |
| A report from a machine fills the database. | Low | The rules bound every string and list, and a broken report has no effect. `TestEventTable`. |

**Verify:** `deploy/test/with-postgres.sh -- go test -race -count=1 ./src/server/internal/link/...`. Environment check: `deploy/test/checks.sh --only C11`. No device is needed.

**Rollout detail:** rollout step 3. The command table is the second database change of the part `link`.

**WS-4: The Machines page works against the Go server**

This workstream answers the eight calls of `ClientService` that the Machines page already makes. It implements the design's section "Serving the contract", rules R2, R5, R6 and R8, and checks C5 to C7.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 12 | The page's record shows the four disjoint cases, with load only while online, and every live-list message carries a position. | `TestProjectClient`, `TestWatchClientsFirstMessageAndChanges` and `TestWatchClientsPositionAlwaysPresent` pass. |
| 13 | The live list ends when the session ends or the reader stops. | `TestWatchClientsNeedsLiveSession` and `TestWatchClientsBlockedReaderEnds` pass. |
| 14 | Every action keeps its rules, an approval and its command are stored together, and a revocation never removes an approved machine. | `TestCreateClientToken`, `TestRevokeClientToken`, `TestRemoveClient`, `TestApproveClient`, `TestScanAndReconnect` and `TestKeyedCallsReplay` pass. |
| 15 | The server program serves both services, clears stale connections at start, and keeps machines across a restart. | `TestServerServesMachines`, `TestRestartKeepsMachines` and `TestStartMarksNoMachineOnlineByMemory` pass. |
| 16 | On the stack, an approved machine shows online with its facts, its load and its four harnesses. | `deploy/test/checks.sh --only C7` prints one line that starts with `ok` and exits 0. Once the page change that the owner chooses has landed, a person makes a token on the Machines page, pastes its command on a machine, approves it, and sees it online without a reload (architect-r1 10). |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 11 | Write the projection and `WatchClients`. | Targets `src/server/internal/machines/project.go`, `Project`, `machines/watch.go`, `Service.WatchClients`, and `machines/service.go`, `NewService`. Pattern: `src/mock-server/src/engine.ts`, `watchClients`, and `src/server/internal/watch/watch.go`, `Serve`. Scouting S3 and S5. |
| 12 | Write the calls that touch only records. | `src/server/internal/machines/service.go`, `Service.CreateClientToken`. Pattern: `src/mock-server/src/engine.ts`, `revokeClientToken`. Scouting S4. |
| 13 | Write the calls that send a command. | Target `src/server/internal/machines/commands.go`, `Service.ApproveClient`. Pattern: `src/mock-server/src/engine.ts`, `approveClient`. Scouting S4. |
| 14 | Wire both parts into the server program and its test entry point. | `src/server/internal/app/start.go`, `Start`, and `src/server/servertest/server.go`, `Start`, of `go-s2-server-signin`. Scouting S6. |

**Item 11. The record of the page, and `WatchClients`.** Creates `src/server/internal/machines/project.go`, `machines/watch.go`, `machines/service.go` (the type `Service`, `ServiceOptions` and `NewService`), `machines/project_test.go` and `machines/watch_test.go`. Needs items 1, 2, 3, 5 and 6 (architect-r2 01). Runs beside item 9. Medium: one pure mapping and one adapter of the store's helper. Statements: D5, R2, R5, R7, R8, C6 to C9.

- **Risky:** it orders changes and checks the session. Leaving `position` out of a `saved` message must fail `TestWatchClientsFirstMessageAndChanges`. An `Authorize` that returns nil must fail `TestWatchClientsNeedsLiveSession`.
- **What `Project` does.** The state is `CLIENT_STATE_WAITING_FOR_APPROVAL` while waiting, with or without a stream. An approved machine is `CLIENT_STATE_OFFLINE` without presence, `CLIENT_STATE_ONLINE` when its presence has both flags, and `CLIENT_STATE_JOINING` otherwise. The name is the host name while waiting, and `token_id` is set only while waiting. `address` and `last_seen` come from the presence when there is one, and from `LastAddress` and `LastSeenAt` otherwise. `connected_at` is set only with presence. The load is set only while online. The facts fill `os`, `version`, `workspace`, `agent_limit`, `cores` and `memory_gigabytes`, which is `memory_bytes` over 2^30 rounded to the nearest whole number and capped at the largest `uint32`. `harnesses_scanned`, `harnesses` and `hosts_server` come from the record.
- **What `WatchClients` does.** It reads the verifier with `api.SessionVerifier` and calls `watch.Serve` with the topic `machines` and an empty scope. `Authorize` calls `auth.Sessions.CheckInRead`, and `auth.ErrNoSession` becomes `watch.ErrWatchUnauthorized`. `Snapshot` and `Load` call `Registry.List` or `Get` and `Link.Presence`, and `ErrNoMachine` means the record is gone. Each message carries `position`, also 0. Each send goes through `transport.Send`. `watch.ErrWatchUnauthorized` gives `unauthenticated` with "Sign in again to reopen this watch.". `watch.ErrAuthorizationUnavailable` gives `unavailable` with "The watch authorization check is unavailable, try again.". `store.ErrWatchLagged` gives `aborted` with "The watch fell behind and must be reopened.". `store.ErrFeedUnavailable` gives `unavailable` with "The change feed is unavailable, try again.".
- **Tests written first:**

| Test | What it asserts |
|---|---|
| `TestProjectClient` | A table of the four cases: a waiting machine with presence shows waiting, its host name and token, and no load; an approved machine without presence shows offline with `last_seen` and `address` from the record and no `connected_at`; with presence and one flag it shows joining with no load; with both it shows online with load, `connected_at` and address. 1,610,612,736 bytes give 2 gigabytes, 429,496,729 bytes give 0, and the largest `uint64` gives 4,294,967,295. A heartbeat of 0 for the processor and none for memory shows 0 and no memory value. |
| `TestWatchClientsFirstMessageAndChanges` | With a cookie, the first message is `all` with a `position`. A new machine arrives as `saved` and a deleted one as `removed_id`, each with a higher position. |
| `TestWatchClientsPositionAlwaysPresent` | On an empty database the first message is an empty `all` with `position` 0 present. |
| `TestWatchClientsNeedsLiveSession` | Without a cookie the call gets `unauthenticated`. After a revocation in the middle of a stream, the stream ends with `unauthenticated` within 35 seconds, and no later `saved` arrives. |
| `TestWatchClientsErrorMapping` | The private mapping of errors, called with `store.ErrWatchLagged`, `store.ErrFeedUnavailable`, `watch.ErrWatchUnauthorized` and `watch.ErrAuthorizationUnavailable`, gives the four codes and sentences above, because the store's seams are private to the store (architect-r1 02). |
| `TestWatchClientsBlockedReaderEnds` | At a `transport` timeout of 200 milliseconds, a reader that stops while 500 changes are written sees the stream end within 3 seconds, and the handler returns. A send that is blocked when the session is revoked delivers nothing and ends at its deadline, and no `saved` written after the revocation arrives (architect-r1 11, threat-r2 04). |

- **Acceptance:** `go test -count=1 -run 'TestProjectClient|TestWatchClients' ./src/server/internal/machines/` passes.

**Item 12. The calls that touch only records.** Changes `src/server/internal/machines/service.go`. Creates `machines/service_test.go`. Needs items 3, 5, 6 and 11. Runs beside items 8 and 13. Medium: three handlers over tested functions. Its tests sit in `machines_test`. Statements: D3, R4, R8, R9, C5.

- **Risky:** it hands out tokens. Accepting a request key on `CreateClientToken` must fail `TestCreateClientToken`. Replacing `DeleteWaiting` with `Delete` in the revocation must fail `TestRevokeClientToken`.
- **Behaviour.**
  - `CreateClientToken` refuses a present `Lgtm-Request-Key` before it makes a token, calls `Tokens.Create` with no mark, and answers `token_id`, `token`, `expires_at` and `address` with the header `Cache-Control: no-store` (threat-r1 08).
  - `RevokeClientToken` calls `Tokens.Revoke`, then `Registry.DeleteWaiting` for the machine it returns, then `Link.Forget` when that deleted it, in one write. An approval that commits first leaves the machine (decision 48).
  - `RemoveClient` calls `Registry.Delete`, then `Link.Forget`, in one write. `ErrNoMachine` gives `not_found`. It removes a waiting machine and the machine that runs the server as well (decision 21). The hub ends a connected stream when the removal reaches the feed.
  - The two keyed calls run in `Store.WriteOnce` with the caller `owner`, their procedure, `ReceiptSafe` true and a version 1 fingerprint of the identifier, framed by an eight-byte big-endian length.
- **Tests written first,** in `service_test.go`, with the real `link.State`. `TestCreateClientToken`: The reply's `token` decodes, its identifier and address equal the reply's `token_id` and `address`, and `expires_at` is 15 minutes after the call within 2 seconds. The reply carries `Cache-Control: no-store`. A machine that joins with it has `hosts_server` false. With `Lgtm-Request-Key`, the call answers `invalid_argument` with "This call does not accept a request key." and no token row exists. `TestRevokeClientToken`: After a revocation of an unused token, `Redeem` gives `token_revoked`. A revocation of a token used by a waiting machine deletes the machine and its link rows, and an open `WatchClients` receives `removed_id`. With an approval held on the machine's lock, the revocation waits, and after the approval commits the machine remains approved. An unknown identifier answers success. `TestRemoveClient`: A connected machine's record and link rows are gone, and an open `WatchClients` receives `removed_id`. An unknown identifier answers `not_found`. One request key sent twice removes once and answers success twice. The machine that runs the server can be removed.

- **Acceptance:** `go test -count=1 -run 'TestCreateClientToken|TestRevokeClientToken|TestRemoveClient' ./src/server/internal/machines/` passes.

**Item 13. The calls that send a command.** Creates `src/server/internal/machines/commands.go` and `machines/commands_test.go`. Needs items 6 and 11. Runs beside items 8 and 12. Medium: four handlers on one pattern. Its tests sit in `machines_test`. Statements: D16, R4, R6, R8, C7.

- **Risky:** approval decides who may work. Storing the `SetState` in a second write must fail `TestApproveClient`. Accepting a scan for a machine that is joining must fail `TestScanAndReconnect`. Projecting the reply after the commit must fail `TestKeyedCallsReplay`.
- **Behaviour.** Each call runs in `Store.WriteOnce` with the caller `owner`, its procedure, `ReceiptSafe` true and a version 1 fingerprint of its fields in the order of the request.
  - `ApproveClient` normalizes the name before `WriteOnce` (architect-r2 11), calls `Registry.SetApproved`, and calls `Link.Enqueue` with `SetState{state: Standing(rec)}` and an expiry of 1 hour, which the machine ignores. Inside the same transaction it projects the machine through the `*store.Tx`, with `Link.Presence`, and returns the encoded reply to `WriteOnce`, so a replay gets the same bytes (architect-r1 08, critic-r1 08, logic-r1 8). `RenameClient` builds its reply the same way.
  - `RenameClient` refuses a waiting machine, calls `Registry.SetName`, and when the name changed enqueues the new `SetState` the same way.
  - `ScanClientHarnesses` and `ReconnectClient` call `Registry.Lock` and read the presence in the same write. A scan requires an online machine, with both flags on its presence. A restart requires an approved machine with presence. They enqueue `ScanHarnesses` or `Restart` for 2 minutes (logic-r1 5, threat-r1 09).
- **Tests written first,** in `commands_test.go`. `TestApproveClient`: Approving as `Box` answers a client of that name, revision 2, and one pending `set_state` holding `APPROVAL_APPROVED`, `Box` and revision 2. With a `Link` whose `Enqueue` fails, the call fails and the machine still waits at revision 1. An approved machine gets `failed_precondition`, an unknown one `not_found`, an empty name `invalid_argument`, and a taken name `already_exists`. `TestRenameClient`: A rename raises the revision and stores one `set_state`. The same name stores nothing. A waiting machine gets `failed_precondition`. `TestScanAndReconnect`: A connected approved machine with both reports gets one `scan_harnesses` and one `restart`, each expiring 2 minutes after the database's clock. An approved machine without presence and a connected waiting machine get `failed_precondition`, with "This machine is not online." for a scan and "This machine is not connected." for a restart, and store nothing (threat-r2 09). A joining machine's scan gets "This machine is not online.", and its restart is stored. `TestKeyedCallsReplay`: A scan sent twice with one request key stores one command. The same key with another identifier answers `already_exists`. An approval whose reply is dropped, then a rename and a removal, then the same key again, answer the first reply's bytes and leave one approval.

- **Acceptance:** `go test -count=1 -run 'TestApproveClient|TestRenameClient|TestScanAndReconnect|TestKeyedCalls' ./src/server/internal/machines/` passes.

**Item 14. Wiring into the server program and its test entry point.** Changes `src/server/internal/app/start.go`, `src/server/internal/api/server.go`, `src/server/cmd/lgtm-server/main.go`, `src/server/servertest/server.go` and `servertest/server_test.go`. Creates `src/server/internal/app/machines_test.go`, in package `app_test`. Needs items 4, 10, 11, 12 and 13. Medium: composition in one function and three test helpers. Statements: D1, D3, D7, R1, R7, R10, C5 to C11, and issue 1634.

- **Risky:** it places the page's calls behind the session guard and the link's calls outside it. Passing the `ClientService` handler as the link handler must fail `TestServerServesMachines`.
- **What changes.**
  - `app.Start` adds `machines.Migrations()` and `link.Migrations()` to `Store.Migrate`. After the migrations and under the instance lock, it calls `State.ClearConnections`. It builds the registry, the state, the tokens with `Issuer{PublicURL: cfg.PublicURL, ServerKey}` from `vault.LinkIdentity.SigningKey`, the hub, and both services. It passes them as `api.Options.Clients` and `api.Options.Link`, and `link.HandlerOptions()` as the new `api.Options.LinkOptions`, which `NewHandler` passes to `linkv1connect.NewLinkServiceHandler`; an empty list keeps the generated defaults. `link.Options.BehindProxy` is `cfg.AllowPrivateStack`. It starts `Service.Run` with the process's context and cancels it through `http.Server.RegisterOnShutdown`, so open streams end inside the five seconds of draining.
  - `servertest.Options` gains `Log io.Writer` and `LinkSeams LinkSeams`, where `type LinkSeams struct { AfterProof, AfterAdmission func(ctx context.Context, machineID string) }` is passed to `link.Options.Seams`. `app.Start` gains a fourth argument, `app.Hooks{LinkSeams link.Seams}`: `cmd/lgtm-server/main.go` passes `app.Hooks{}`, and `servertest.Start` and `Restart` pass the fixture's seams, so the orchestrator's tests reach the admission seams of the contract's post-build check 10 (architect-r2 09).
  - `servertest.Server` gains `Clients(t testing.TB) lgtmv1connect.ClientServiceClient`, signed in through the fixture's cookie jar, and `Link() linkv1connect.LinkServiceClient`. `Environment()` of the sign-in plan already gives a second program the server's settings.
- **Tests written first,** in `app/machines_test.go` and `servertest/server_test.go`. `TestServerServesMachines`: Through `servertest.Start`, `Clients(t).CreateClientToken` answers a token whose address is the server's URL. `Join` through `Link()` without a cookie answers a machine. `WatchClients` without a cookie answers `unauthenticated`. `Open` without a cookie answers a `Challenge`. A `Join` larger than 64 kibibytes through the router answers `resource_exhausted`. `TestRestartKeepsMachines`: A joined and approved machine is listed approved after `Restart`, and `SigningPublicKey` is unchanged. `TestShutdownEndsLinkStreams`: `Stop` returns within 5 seconds, the connected stream ends with `unavailable`, and after `Restart` the machine shows offline. `TestStartMarksNoMachineOnlineByMemory`: The test inserts a connection row for an approved machine while the server is stopped. After `Restart`, the first `WatchClients` message shows it offline. `TestLinkSeamsReachTheService`: A `LinkSeams.AfterAdmission` that records its argument sees the machine's identifier when a scripted machine connects, also after `Restart`.

- **Acceptance:** `go test -count=1 -run 'TestServerServesMachines|TestRestartKeepsMachines|TestShutdownEndsLinkStreams|TestStartMarksNoMachineOnlineByMemory' ./src/server/internal/app/` and `go test -count=1 -run TestLinkSeamsReachTheService ./src/server/servertest/` pass.

**Design artifact:** not applicable. No screen changes. The page's records and actions exist today, and the sample server shows how they behave.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| The live list keeps sending after the owner's session ended. | High | The store's helper authorizes every change again, and every 30 seconds while quiet. `TestWatchClientsNeedsLiveSession`. |
| An approval is stored without its command. | Medium | Both are written in one transaction. `TestApproveClient`. |
| A revocation removes a machine that was just approved. | Medium | The revocation deletes only a machine that still waits, after the machine's lock. `TestRevokeClientToken`. |
| The page shows the old join command and offers "Try to reconnect" only for an offline machine. | High | The owner chooses the page change, and criterion 16 waits for it. |
| A sprint 2 file changes in a way that breaks its own tests. | Medium | Item 14 runs every test of `src/server/` before it merges. |

**Verify:** `deploy/test/with-postgres.sh -- go test -race -count=1 ./src/server/...`. Environment check: `deploy/test/checks.sh --only C5,C6,C7,C8,C9,C10,C11` with the client and the installer of the other two plans, and the step a person performs in criterion 16.

**Rollout detail:** rollout steps 3, 4, 7 and 8. The stack runs `deploy/up.sh`, exports `LGTM_WEB_PASSWORD` and `LGTM_TEST_POSTGRES`, and runs `deploy/test/checks.sh`.

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

Source universe:

1. The brief's work items 1 to 9, its four observations and its two agreements, in `plan.md` of 2026-11-25.
2. The brief's points to settle 1 to 4.
3. The task files `plan-`, `reconcile-` and `fold-go-s3-server-machines-r1.md` and `-r2.md` in `.foundry/go-rewrite/tasks/`.
4. The requests to this plan in `.foundry/go-rewrite/requests-from-contract-r3.md` (1 to 10) and `requests-from-contract-r4.md` (1 to 9).
5. The issues 1633, 1636 to 1642 in `docs/issues.md`.
6. The design's statements that bear on this plan: D3, D5, D7, D16, D17, R1 to R10, C5 to C11.
7. `decisions.md`, rows 1 to 76.
8. The reports of review rounds 1 and 2 in `attack/`, folded in `attack/round-1-findings.md` and `attack/round-2-findings.md`.

| Source item | Workstream | Status |
|---|---|---|
| Brief items 1 to 9 | WS-1 items 1, 3, 4; WS-2 items 5, 8 to 10; WS-3 items 6, 7; WS-4 items 11 to 13 | covered, the high items split |
| Brief observations and agreements | Criteria 4, 8 and 16; public names; item 14 | covered |
| Brief points 1 to 3: issue 1633, a waiting machine, replacing the key | Decisions 39, 11, 44, 45 and 17 | settled for wave 1, with assumptions |
| Brief point 4, `link/service.go` cut into files | Items 4, 5, 8, 9 and 10 | settled: `service.go` keeps the type and its constructor, and `join.go`, `handshake.go`, `presence.go`, `hub.go`, `stream.go` and `run.go` hold the rest |
| Reconcile task: names of other plans, requests, no growth | This version; decisions 39 to 52; result file | covered |
| Contract rounds 3 and 4, every request: codes and reasons, the table of events, one admission and removal, revocation against approval, results and `SetState`, the address, the loser of two uses, four disjoint cases | The codes table; items 3 to 8, 10 to 12; decision 40 | covered |
| Issues 1637, 1638 and 1639 | Decisions 16, 15 and 17 | settled with assumptions |
| Issue 1640, a sandbox without PostgreSQL | Verify lines | every test needs `LGTM_TEST_POSTGRES` |
| Issues 1636, 1641 and 1642: the join command's spelling, a restore, two copies of one machine | Outside this plan, or outside wave 1 | the page's text belongs to the orchestrator and the owner, and no rule of this plan serves the other two |
| D3, D5, D7, D16, D17 | Items 1, 3 to 6, 8, 11 and 13 | covered |
| R1 to R10 | Every item; the codes table; the log lines; decisions 9, 40 and 41 | covered |
| C5, C6, C7 | Items 3, 4, 7, 10 to 12; criteria 4 and 16 | covered |
| C8, C9, C10, C11 | Items 5, 7, 8, 10 and 14; criteria 8 and 11 | covered |
| `decisions.md` rows 1 to 76 | Key decisions and the items | every row closed or unanswered with its assumption |
| Review rounds 1 and 2, every finding | `attack/round-1-findings.md`, `attack/round-2-findings.md`; the post-build checks | each with one outcome |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repo rule (CLAUDE.md etc.) | How this plan satisfies it |
|---|---|
| `AGENTS.md`: the three parts meet only through the contract in `src/contracts/`. | The server imports only the generated packages `lgtmv1` and `linkv1`, and asks the Contract team for every change of the wire. |
| `AGENTS.md`: new code does not go into `packages/`, `design/mock/` or `plugins/`. | Every file is under `src/server/`. |
| `AGENTS.md`, habit 1: credit copied code. | No code is copied. |
| `AGENTS.md`, habit 2: write down what you find. | This plan may not edit `docs/`. The result file lists the problems for the orchestrator. |
| `AGENTS.md`, habit 3: finished work reaches the base branch through a pull request. | The orchestrator merges the sprint. |
| `AGENTS.md`, writing style, and `scripts/check-private.mjs`. | Full sentences, expanded acronyms, no quotation of a person, and only loopback and documentation addresses. |
| The epic: a team edits only its own folders. | Every file is in `src/server/`. Three files of sprint 2 change, all in the Server team's folder (decision 28). |
| The epic: a builder adds a module together with its first import. | This plan needs no module beyond those that the sprint 0 to 2 plans add. |
| The design, rule 10: tests run against a real PostgreSQL. | Every database test uses `migrated.NewStore` with `LGTM_TEST_POSTGRES`. |
| The planning rules of this epic: name no model and no harness. | The plan names work areas and complexities only. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

Models come from the harness-sessions plugin's model sheet (its harness-session skill) at launch; this section names none (owner, 2026-10-16).
The scope `server/backend` exists in `.plans/scopes/server/backend/`.

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on |
|---|---|---|---|---|---|---|
| 1. Machine records | `backend` | `server/backend` | medium | One table and its functions on the pattern of the sessions table. | no | sprint 1 store, sprint 0 contract |
| 2. Write deadlines | `backend` | `server/backend` | low | One middleware of the standard library and a one-line router change. | no | sprint 2 router |
| 3. Tokens | `backend` | `server/backend` | medium | One table and an encoding the contract fixes. | no | 1 |
| 4. Joining | `backend` | `server/backend` | medium | One transaction with a fixed order of checks and one thin handler. | yes, a machine reaches `Join` without a session | 3, 5 |
| 5. Link tables and admission | `backend` | `server/backend` | medium | Short writes under one lock order. | no | 1 |
| 6. Stored commands | `backend` | `server/backend` | medium | One table and four functions under the machine's lock. | no | 5 |
| 7. Incoming events | `backend` | `server/backend` | medium | One transaction with the contract's table of decisions. | no | 5 |
| 8. The handshake | `backend` | `server/backend` | medium | Bytes fixed by worked examples and a short state machine. | yes, a machine reaches `Open` without a session | 2, 4, 5, 9 |
| 9. The hub | `backend` | `server/backend` | medium | A map under one lock and one feed subscriber. | no | 1, 6 |
| 10. The stream loop | `backend` | `server/backend` | medium | Tested functions in one select loop with one writer. | yes, the loop reads every message a machine sends | 2, 6, 7, 8, 9 |
| 11. `WatchClients` | `backend` | `server/backend` | medium | One pure mapping and one adapter of the store's helper. | yes, the website reaches it | 1, 2, 3, 5, 6 |
| 12. Record calls | `backend` | `server/backend` | medium | Three handlers over tested functions. | yes, the website reaches them | 3, 5, 6, 11 |
| 13. Command calls | `backend` | `server/backend` | medium | Four handlers on one pattern. | yes, the website reaches them | 6, 11 |
| 14. Wiring | `backend` | `server/backend` | medium | Composition in one function and three test helpers. | yes, it places the guard | 4, 10, 11, 12, 13 |

No item is high.

Load: each of the 14 items needs one test writer and one builder of the other vendor, which is 28 launches. The 12 risky items each need one reviewer who breaks the code on a temporary branch, which is 12 launches. Code review of the merged sprint needs about 6 launches in slices. That is about 46 launches, split about evenly between the first two vendors, with about 8 on the third for reading beside. Each plan review round adds about 6 launches. The orchestrator reads each vendor's remaining weekly allowance at the start of the sprint. If one vendor would run out, items 12 and 13 can wait, because only item 14 needs them.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (B/M/m) | Outcome |
|---|---|---|
| 1 | 4/15/7, counted once per group of duplicates, 88 points | Folded on 2026-11-26. The register is `attack/round-1-findings.md`. Three blockers are fixed, and the fourth, the cleartext path, is for the owner. |
| 2 | 1/12/5, counted once per group of duplicates, 50 points | Folded on 2026-11-26. The register is `attack/round-2-findings.md`. The blocker, the order of the items, is fixed by the table of public names. The presence filter, the batch of commands, `facts_at` and `Hub.CloseAll` are removed. |

</details>

<details><summary><b>Post-build checks</b></summary>

Each check needs the built server, and most need the real client, the web server in front, or both. The finding that asked for it is named in each row.

| # | Scenario | Expected result | From |
|---|---|---|---|
| P1 | Through the real web server, over Connect and over gRPC, replay a recorded `Proof` on a new stream, and refuse a removed machine after `Challenge`. | The replay ends with `unauthenticated` and admits nothing. The client reads `unknown_machine` from the metadata of the terminal error in both protocols. | architect-r1, critic-r1 PB1, logic-r1, logic2-r1 4, threat-r1 PB1 |
| P2 | Hold a stream right after `Confirm`, commit the removal, then release it. Let the server send a `Welcome`, hold its bytes in the web server, commit the removal, then release them. Separately, remove a connected machine while the hub's follower lags. | The machine receives `Welcome` and then `unknown_machine`, and nothing on the stream changes a row. The lagging case ends with `unknown_machine` within 3 seconds. | architect-r1 03, logic2-r1 6, contract check 16, architect-r2 02, logic-r2 02, threat-r2 02 |
| P3 | Through one open `WatchClients`, stop a client's traffic without a close while a send to it is blocked, fail the first cleanup, then reconnect it. | The blocked stream is reset, and the client reconnects. With every write succeeding, the page shows the machine offline within 30 seconds of its last heartbeat, plus one commit and one feed delivery. With the cleanup failing, it shows offline within a second after the database accepts the delete. The new stream shows joining, then online after both reports, and nothing from the old stream changes it. | architect-r1 04, critic-r1 PB2, logic2-r1 1 and 2, threat-r1 PB3, architect-r2 03 and 05, logic-r2 01, threat-r2 03 and 04 |
| P4 | Cut the link once before event 41 commits and once after its commit and before its `EventAck`. Restart the server, and in another run the client, with their stores kept. | Read from the mark and the stored facts, events 41 to 43 take effect once each and in order, and a later event goes on. The log lines corroborate this and are not the proof, and the revised C11 compares them as a sorted set. | architect-r1 07, critic-r1 PB3, logic2-r1 5, threat-r1 PB5, architect-r2 07, logic-r2 07, threat-r2 07 |
| P5 | Drop the result of a finished scan, restart the client, and reconnect. Lose the result of a `Restart` after the client saved it. Repeat with the machine's clock 5 minutes fast and the retry in the last 5 minutes of the 24 hours. | The next attempt closes each command once with the recorded outcome, and the restart happens once. With the fast clock the retry reports `expired`, as the overview states. | architect-r1 12, critic-r1 PB4, logic-r1, threat-r1 PB4, architect-r2 04, threat-r2 06 |
| P6 | Publish the website on port 8899 and join one machine with the installer and one with the page's command. | The token's address, the address the client dials and the page's command all name port 8899. | architect-r1 09 and 10 |
| P7 | Start the sprint 2 server against a database that the sprint 3 server migrated, then start a corrected sprint 3 server. | The sprint 2 server refuses to start with the store's sentence about the ledger, and the sprint 3 server starts with every machine kept. | critic-r1 11 |
| P8 | Start shutting the server down while one stream is held before and one after its admission. | Both end with `unavailable`, no handle survives, and the next server sees the machine reconnect with its earlier mark. | logic-r1, architect-r2 08, threat-r2 08 |
| P9 | Reconnect a waiting client 100 times while the test approves it at a random moment. | Every `Welcome` is waiting with an empty token, or approved with a token and a matching revision. | logic2-r1 3 |
| P10 | Join with a marked fake secret, then search the database, the feed, the logs of both programs, the process list and the reply's headers. | The secret appears only where the owner's choice on the token's input allows, and the reply carries `Cache-Control: no-store`. A marked unknown field of a report is in no stored byte, reply or log. | threat-r1 08 and PB1, threat-r2 05 |

</details>
