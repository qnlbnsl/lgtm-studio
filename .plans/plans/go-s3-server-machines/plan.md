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

Sprint 3 is the sprint in which a machine joins the Go server.
This plan builds the server's half of that.
The client's half is the plan `go-s3-client-link`, and the installer is the plan `go-s3-stack-client`.

When this plan is built, the owner opens the Machines page of the website against the Go server and makes a join token.
A machine uses the token once to register its public key, and it then appears on the page as waiting for approval, without a reload.
The owner approves the machine under a name.
From then on the machine connects by itself, proves who it is with its key on every connection, and shows as online with its facts, its load and the harnesses it found.
The owner can rename it, ask it to look for harnesses again, restart its client, and remove it.

The server keeps all of this in PostgreSQL and nothing in memory that it could not read again.
A machine counts as online only while the database holds a recent heartbeat of its current connection.
A machine that stops sending heartbeats shows as offline within 30 seconds.
A server that restarts therefore needs no memory of who was connected, and a machine that was connected comes back by itself within 15 seconds.

Two promises of the design need the most care.
A command to a machine must never be lost or run twice, so the server stores each command before it sends it, numbers each attempt, and accepts an answer only for the attempt it sent last.
A report from a machine must be applied once and in order, so the server applies only the next number, commits the report and its number together, and confirms only after that commit.

The plan has 14 work items in four workstreams.
The brief had nine items, five of them rated high.
Every item is now rated low or medium, and each has tests that are written before its code.
Twelve items are risky, because they touch tokens, keys or ordering, and for each of them the plan names a change to the code that must turn its tests red.

The plan needs several names from other plans, and it writes each as a request in its result file.
The most important ones are a response header that tells the client why a token was refused, which check C5 needs, and a setting that names the certificate of a public address.
Four choices of the owner from the contract plan are still open, and none of them blocks this plan.
The plan builds on the defaults the orchestrator recommended, and each choice changes one small part if the owner decides otherwise.

## Key decisions

**A machine that waits for approval can connect and do nothing else.**
The design says this.
The server answers its handshake and its heartbeats, sends it no command except a change of its standing, and gives it no connection token.
An event from it is accepted and set aside, so a client that sends one anyway does not loop.
The client plan sends its facts after approval, so nothing is lost.
If the owner chooses to see the facts of a waiting machine, which the orchestrator recommended as option B, one table in the server changes.

**The token pins the certificate's key, and encryption ends at the web server.**
This answers issue 1633 the way the stack plan does.
In public mode the server reads the public certificate of the stack and puts the hash of its key into each token.
On the loopback address nothing is encrypted, and the pin is empty.
A machine proves itself only by signing a challenge.

**Keys are not replaced in wave 1.**
This answers issue 1639.
If the server's signing key or the certificate's key is ever replaced by hand, every machine that joined before refuses the server and shows as offline.
The owner then removes each such machine and joins it again.
A hand-over in which the old key vouches for the new one needs a new message in the contract, so it waits for a later wave.

**The machine that runs the server is marked by its token.**
This answers issue 1638.
Only the server's own command, `lgtm-server join-token --hosts-server`, makes such a token.
The owner may remove that machine like any other, which issue 1639 needs.

**The address on the page is where the connection came from.**
This answers issue 1637.
It is for showing only, because the server never dials a machine.

**A removal stops a connected machine at once.**
The server ends the machine's stream with the code that tells the client the server no longer knows it, and the client stops trying.

**`ReconnectClient` restarts a client that is connected.**
The owner decided this on 2026-11-25.
Today's Machines page offers "Try to reconnect" only for an offline machine, which the Go server refuses.
How the page follows is the owner's point 1 of the contract plan, and this plan does not change the page.

Full log → `decisions.md`

## Workstreams

### The owner makes a join token, and a machine joins with it

The server keeps a record of each machine and of the key it registered, and it keeps a verifier of each token, never the token itself.
A token works once, expires after 15 minutes, and can be revoked.
A machine whose reply was lost can ask again with the same key and gets the same answer.
The main risk is a token that works twice, or a refusal that says more than it should, and the tests try both.
When it is done, check C5 sees a used and an expired token refused, and check C6 sees the joined machine waiting.

### A machine proves itself and stays connected

Both ends sign a fresh challenge on every connection.
A new connection of a machine replaces its older one, and the older one can no longer change anything.
The server knows a machine is there from its heartbeats, and it ends a connection that falls silent for 30 seconds.
The main risk is an old connection that ends late and marks a connected machine as offline, and the tests stage exactly that.
When it is done, checks C8, C9 and C10 see a machine go offline, come back after a server restart, and be refused when its key is unknown.

### Commands go down once, and reports come up once

A command is stored before it is sent, and each attempt has its own number.
A report is applied only when its number follows the last one, and its effect and its number are saved together.
The main risk is a confirmation that reaches the machine before the report is saved, and the tests break the save to see that no confirmation follows.
When it is done, check C11 sees reports that a machine kept during an outage arrive once and in order.

### The Machines page works against the Go server

The page's live list sends everything first and then one change at a time, each with its place in the server's order of changes.
The page's actions make tokens, approve, rename, scan, restart and remove.
The main risk is a list that keeps sending after the owner's session ended, and the tests revoke a session in the middle of a stream.
When it is done, check C7 sees an approved machine online with its facts, its load and its four harnesses, and the page needs no change.

## Rollout order

1. Build the machine records and the write deadline for long streams first. Nothing else of this plan can be tested without the records.
2. Build the tokens and the link's tables. The two run side by side, because they touch different tables.
3. Build joining, stored commands, incoming reports and the page's live list side by side. Each needs only the steps before.
4. Build the handshake, the hub that tracks open connections, and the page's actions side by side.
5. Build the loop that runs one connection after the handshake. It is the first point at which a scripted machine can stay connected.
6. Wire everything into the server program and its test entry point. From this step on, the orchestrator's tests in `src/linktest/` can run a real client against a real server.
7. At the end of the sprint, run `deploy/test/checks.sh` against the stack with the client and the installer of the other two plans. Checks C5 to C11 must pass.

No step is irreversible.
Each step adds its own numbered database change, and a later correction is a new change.

---
<!-- ⛔ implementation details and gate evidence below; owner overview is complete -->

<details><summary><b>Implementation details (gates 2, 3, 5, 6, 8)</b></summary>

**How every item is built.** Phase 4 of the build loop applies to each item. A test writer of another vendor than the builder writes the named tests first, from this plan and the design, against stubs that return the empty value. Each test must fail on its assertion, not on a build error or a crash. The tests are then frozen. The builder writes the code. The lead checks the exact commit. For a risky item, a reviewer applies the named mutation on a temporary branch and the named test must go red. The design's statements are cited as D1 to D23 (its decisions table), R1 to R11 (its rules) and C1 to C13 (its checks), counted in the order of the design file on `master`. Every item has the scope `server/backend`.

**Test fixtures shared by every item.** A test that touches the database uses `migrated.NewStore(t, schema, machines.Migrations(), link.Migrations())` from `src/server/internal/store/storetest/migrated`, against the real PostgreSQL that `LGTM_TEST_POSTGRES` names. A fixture wait is at most 15 seconds, and a check that something does not happen waits 250 milliseconds. A test that needs the transport serves the handler through `net/http/httptest` with cleartext HTTP/2 (h2c), set with `http.Server.Protocols`. A test that needs a signed-in website session calls `auth.Sessions.Issue` of plan `go-s2-server-signin` and sends the cookie. A test reads the server's log from a `log/slog` JSON handler that writes into a buffer.

**Public names that other items and other teams build against**

The Go names below are exact. A stub carries them before its code exists.

```go
// Package machines, src/server/internal/machines. Items 1, 3, 4, 11, 12 and 13.
const Topic = "machines"                   // feed topic of the page's record, key = machine id
const TokensTopic = "client_join_tokens"   // feed topic of a token, key = token id
const TokenLifetime = 15 * time.Minute
const MaxNameBytes = 64

func Migrations() store.Migrations          // Part "machines", files in machines/migrations/

type Approval string
const (
    Waiting  Approval = "waiting"
    Approved Approval = "approved"
)

type Querier interface {                    // *store.ReadTx and *store.Tx both satisfy it
    Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
    QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Record struct {
    ID               string
    PublicKey        ed25519.PublicKey
    HostName         string
    Name             string                 // empty while waiting
    Approval         Approval
    StateRevision    int64
    HostsServer      bool
    TokenID          string
    JoinedAt         time.Time
    Facts            *linkv1.Facts          // nil until the first applied Facts
    Harnesses        []*linkv1.Harness
    HarnessesScanned bool
    LastSeenAt       time.Time              // zero until the first connection ended
    LastAddress      string
}

type NewMachine struct {
    PublicKey   ed25519.PublicKey
    HostName    string
    HostsServer bool
    TokenID     string
}

var ErrNoMachine, ErrInvalidName, ErrNameTaken, ErrNotWaiting error

type Registry struct{ /* private */ }
func NewRegistry() *Registry
func NormalizeName(name string) (string, error)
func (r *Registry) CreateWaiting(ctx context.Context, tx *store.Tx, m NewMachine) (Record, error)
func (r *Registry) Get(ctx context.Context, q Querier, id string) (Record, error)
func (r *Registry) List(ctx context.Context, q Querier) ([]Record, error)
func (r *Registry) LockForStream(ctx context.Context, tx *store.Tx, id string) (Record, error)
func (r *Registry) ApplyFacts(ctx context.Context, tx *store.Tx, id string, f *linkv1.Facts) error
func (r *Registry) ApplyHarnesses(ctx context.Context, tx *store.Tx, id string, h *linkv1.HarnessesFound) error
func (r *Registry) NoteSeen(ctx context.Context, tx *store.Tx, id string, at time.Time, address string) error
func (r *Registry) Touch(ctx context.Context, tx *store.Tx, id string) error
func (r *Registry) SetApproved(ctx context.Context, tx *store.Tx, id, name string) (Record, error)
func (r *Registry) SetName(ctx context.Context, tx *store.Tx, id, name string) (Record, bool, error)
func (r *Registry) Delete(ctx context.Context, tx *store.Tx, id string) error
func Standing(rec Record) *linkv1.MachineState

type Presence struct {
    StreamID          string
    ConnectedAt       time.Time
    LastHeardAt       time.Time
    Address           string
    CPUPercent        *uint32
    MemoryPercent     *uint32
    FactsReceived     bool
    HarnessesReceived bool
}

type Link interface {                       // *link.State satisfies it
    Presence(ctx context.Context, q Querier, ids []string) (map[string]Presence, error)
    Enqueue(ctx context.Context, tx *store.Tx, machineID string, action *linkv1.Command, ttl time.Duration) (string, error)
    Forget(ctx context.Context, tx *store.Tx, machineID string) error
}

type Issuer struct {
    PublicURL       string
    ServerKey       ed25519.PublicKey
    CertificateFile string                  // required when PublicURL is https
}
type TokenOptions struct {
    Address     string                      // empty means Issuer.PublicURL
    HostsServer bool
}
type NewToken struct {
    ID        string
    Text      string
    ExpiresAt time.Time
}
var ErrAddressMismatch, ErrNoCertificatePin error

type Tokens struct{ /* private */ }
func NewTokens(s *store.Store, r *Registry, issuer Issuer) *Tokens
func (t *Tokens) Create(ctx context.Context, o TokenOptions) (NewToken, error)
func (t *Tokens) Revoke(ctx context.Context, tx *store.Tx, tokenID string) (waitingMachineID string, err error)
func (t *Tokens) Prune(ctx context.Context) (int64, error)
func (t *Tokens) Redeem(ctx context.Context, tokenID string, secret []byte, key ed25519.PublicKey, hostName string) (Joined, error)
func EncodeToken(tok *linkv1.JoinToken) (string, error)
func DecodeToken(text string) (*linkv1.JoinToken, error)
func NormalizeAddress(address string) (string, error)
func CertificateKeySHA256(certPEM []byte) ([]byte, error)

type Refusal string
const (
    RefusalUnknown        Refusal = "token_unknown"
    RefusalExpired        Refusal = "token_expired"
    RefusalRevoked        Refusal = "token_revoked"
    RefusalUsed           Refusal = "token_used"
    RefusalMachineRemoved Refusal = "machine_removed"
)
type RefusedError struct{ Reason Refusal }
func (e *RefusedError) Error() string        // the fixed sentence of the reason
type Joined struct {
    MachineID string
    Replay    bool
}

type ServiceOptions struct {
    Store    *store.Store
    Follower *store.Follower
    Registry *Registry
    Tokens   *Tokens
    Link     Link
    Sessions *auth.Sessions
    Log      *slog.Logger
}
type Service struct {
    lgtmv1connect.UnimplementedClientServiceHandler
    /* private */
}
func NewService(o ServiceOptions) *Service
func Project(rec Record, p *Presence) *lgtmv1.Client
```

```go
// Package link, src/server/internal/link. Items 4 to 10.
const LinkRevision = 1
const RefusalHeader = "Lgtm-Join-Refusal"
const MaxMessageBytes = 64 << 10
const OutboxTopic = "machine_outbox"        // feed topic, key = machine id
const HandshakeTimeout = 10 * time.Second
const HeartbeatTimeout = 30 * time.Second
const LeaseLength = 35 * time.Second
const SweepInterval = 5 * time.Second
const CommandGrace = 24 * time.Hour
const CommandRetention = 7 * 24 * time.Hour

func Migrations() store.Migrations          // Part "link", files in link/migrations/
func ServerProofText(machineID string, machineNonce, serverNonce []byte) []byte
func MachineProofText(machineID string, machineNonce, serverNonce []byte) []byte

type State struct{ /* private */ }
func NewState(s *store.Store, r *machines.Registry) *State
type Stream struct {
    MachineID       string
    StreamID        string
    Generation      int64
    Standing        machines.Record
    Mark            uint64
    ServerTime      time.Time
    ConnectionToken string                  // empty while waiting
}
var ErrNotCurrent, ErrUnknownMachine, ErrEventGap, ErrInvalidReport, ErrSuperseded error
func (st *State) BecomeCurrent(ctx context.Context, machineID, address string) (Stream, error)
func (st *State) Heartbeat(ctx context.Context, s Stream, cpu, memory *uint32) error
func (st *State) Disconnect(ctx context.Context, s Stream) (bool, error)
func (st *State) SweepPresence(ctx context.Context) (int, error)
func (st *State) Presence(ctx context.Context, q machines.Querier, ids []string) (map[string]machines.Presence, error)
func (st *State) Forget(ctx context.Context, tx *store.Tx, machineID string) error
func (st *State) CheckConnectionToken(ctx context.Context, token string) (string, error)
func (st *State) Enqueue(ctx context.Context, tx *store.Tx, machineID string, action *linkv1.Command, ttl time.Duration) (string, error)
func (st *State) Claim(ctx context.Context, s Stream) ([]*linkv1.Command, error)
func (st *State) AcceptResult(ctx context.Context, s Stream, r *linkv1.CommandResult) (bool, error)
func (st *State) SweepCommands(ctx context.Context) (expired, deleted int64, err error)
type EventResult struct {
    Mark    uint64
    Applied bool
}
func (st *State) ApplyEvent(ctx context.Context, s Stream, ev *linkv1.Event) (EventResult, error)

type Hub struct{ /* private */ }
type Handle struct{ /* private */ }
func NewHub() *Hub
func (h *Hub) Register(machineID string, generation int64) (*Handle, error)
func (h *Hub) Unregister(hd *Handle)
func (h *Hub) Wake(machineID string)
func (h *Hub) WakeAll()
func (h *Hub) End(machineID string, err error)
func (h *Hub) CloseAll(err error)
func (h *Hub) Follow(ctx context.Context, s *store.Store, f *store.Follower) error
func (hd *Handle) Woken() <-chan struct{}
func (hd *Handle) Ended() <-chan error

type Options struct {
    Store       *store.Store
    Follower    *store.Follower
    Registry    *machines.Registry
    Tokens      *machines.Tokens
    State       *State
    Hub         *Hub
    SigningKey  ed25519.PrivateKey
    Log         *slog.Logger
    BehindProxy bool                        // config.Config.AllowPrivateStack
}
type Service struct{ /* private */ }
func NewService(o Options) *Service          // satisfies linkv1connect.LinkServiceHandler
func (s *Service) Join(ctx context.Context, req *connect.Request[linkv1.JoinRequest]) (*connect.Response[linkv1.JoinResponse], error)
func (s *Service) Open(ctx context.Context, stream *connect.BidiStream[linkv1.OpenRequest, linkv1.OpenResponse]) error
func (s *Service) Run(ctx context.Context) error
func HandlerOptions() []connect.HandlerOption
```

```go
// Package transport, src/server/internal/transport. Item 2.
const SendTimeout = 10 * time.Second
var ErrNoDeadline error
func WithWriteDeadline(next http.Handler) http.Handler
func Send(ctx context.Context, send func() error) error
```

```go
// Package machinetest, src/server/internal/link/machinetest. Item 8, test fixture only.
type Machine struct {
    ID        string
    Key       ed25519.PrivateKey
    ServerKey ed25519.PublicKey
}
type Conn struct{ /* private */ }
func NewServer(t testing.TB, h http.Handler) (url string, client linkv1connect.LinkServiceClient)
func Join(t testing.TB, c linkv1connect.LinkServiceClient, tokenText string) *Machine
func (m *Machine) Dial(t testing.TB, c linkv1connect.LinkServiceClient) *Conn
func (m *Machine) Connect(t testing.TB, c linkv1connect.LinkServiceClient) (*Conn, *linkv1.Welcome)
func (c *Conn) Send(t testing.TB, msg *linkv1.OpenRequest)
func (c *Conn) Receive(t testing.TB) (*linkv1.OpenResponse, error)
func (c *Conn) Handshake(t testing.TB) (*linkv1.Challenge, *linkv1.Welcome, error)
func (c *Conn) Close()
```

**Tables.** Part `machines` owns `machines` and `client_join_tokens`. Part `link` owns `link_machines`, `link_connections` and `link_commands`. Every table sits in the schema of the application's connection, which is `public` in the stack. The exact columns are in items 1, 3 and 5.

**Feed topics.** `machines`, `machine_outbox` and `client_join_tokens`, each with an empty scope. Decision 30 gives the reasons.

**Log lines.** One JSON object per line through `log/slog`. `link_event_applied` with `machine_id` and numeric `sequence` after each commit that moved a mark. `link_stream_opened` with `machine_id` and `generation`. `link_stream_closed` with `machine_id` and `code`. `link_refused` with `reason`, one of `bad_hello`, `unknown_machine`, `bad_proof`, `link_revision` and `timeout`. No line holds a token, a secret, a nonce, a signature or a connection token.

**Errors on the website's calls.** Each is a Connect code with one fixed sentence (R8).

| Condition | Code | Sentence |
|---|---|---|
| Unknown machine identifier | `not_found` | No machine has that id. |
| Name empty, too long or with a control character | `invalid_argument` | A machine needs a name of 1 to 64 characters. |
| Name taken | `already_exists` | Another machine already has that name. |
| Approve a machine that does not wait | `failed_precondition` | This machine does not wait for approval. |
| Rename a machine that waits | `failed_precondition` | Approve this machine before you rename it. |
| Scan or restart a machine that is not connected or not approved | `failed_precondition` | This machine is not connected. |
| A request key on `CreateClientToken` | `invalid_argument` | This call does not accept a request key. |
| A token for `https` without a readable certificate | `failed_precondition` | The server cannot read its certificate, so it cannot make a join token. |
| The database is busy | `aborted` | The database was busy, retry the request. |
| The database is unavailable | `unavailable` | The server cannot reach its database, try again. |

**Errors on the link.** These follow the table of codes in the contract's `LinkService` comment.

| Condition | Code | Sentence |
|---|---|---|
| `Join` refused | `permission_denied` | The sentence of the `Refusal`, below, and the header `Lgtm-Join-Refusal`. |
| A value rule broken, or a message out of order before `Welcome` | `invalid_argument` | The link message broke a rule of the contract. |
| Link revision other than 1 | `failed_precondition` | The server speaks revision 1 of the link. |
| Unknown or removed machine, or a bad proof, after `Challenge` | `unauthenticated` | The server does not know this machine. Join it again. |
| `Hello` or `Proof` late | `deadline_exceeded` | The machine did not answer in time. |
| 30 seconds without a heartbeat | `deadline_exceeded` | The machine sent no heartbeat for 30 seconds. |
| A newer stream of the same machine | `aborted` | A newer connection of this machine replaced this one. |
| An event above the mark plus one | `out_of_range` | The event leaves a gap after the server's mark. |
| The server stops | `unavailable` | The server is stopping. |
| The database is unavailable | `unavailable` | The server cannot reach its database, try again. |

The sentences of the refusals are fixed: `token_unknown` gives "The server does not know this join token.", `token_expired` gives "This join token has expired. Make a new one.", `token_revoked` gives "This join token was revoked. Make a new one.", `token_used` gives "This join token was already used. Make a new one.", and `machine_removed` gives "The machine that used this join token was removed. Make a new token."

**WS-1: The owner makes a join token, and a machine joins with it**

This workstream gives the server its record of machines and its join tokens, and it answers `Join`.
A token is a secret that the owner copies to a new machine, so the server keeps only a verifier of it.
`Join` checks the token, uses it up and makes the waiting machine in one transaction.
The workstream implements the design's section "Joining: a token that authorizes an exchange of keys" and checks C5 and C6.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 1 | A machine record keeps its key, standing, facts and harnesses, and a removal waits for a handshake that holds the record. | `TestCreateWaitingAndRead`, `TestApplyFactsAndHarnesses`, `TestApproveRenameAndNames`, `TestLockForStreamHoldsOffRemoval` and `TestDeleteRecordsRemoval` pass. |
| 2 | A token carries the contract's fields, lasts exactly 15 minutes, and is stored only as a verifier. | `TestCreateTokenText`, `TestTokenLifetime`, `TestTokenStoresOnlyVerifier`, `TestTokenPinsCertificateKey` and `TestTokenAddressMustMatch` pass. |
| 3 | `Join` makes one waiting machine, answers a lost reply with the same machine, and refuses every other request with the reason in a header. | `TestRedeemCreatesWaitingMachine`, `TestRedeemReplay`, `TestRedeemRefusals`, `TestRedeemConcurrentKeys`, `TestJoinCall` and `TestJoinRefusalCodes` pass. |
| 4 | On the stack, a used and an expired token are refused, and the joined machine appears as waiting. | `deploy/test/checks.sh --only C5,C6` prints two lines that start with `ok` and exits 0, with the client of `go-s3-client-link`. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 1 | Write the machines table and its registry. | Target `src/server/internal/machines/store.go`, `Registry`, and `src/server/internal/machines/migrations/0001_machines.sql`. Pattern: `src/server/internal/auth/sessions.go`, `Sessions`, of `go-s2-server-signin`. Scouting S5 and S12. |
| 3 | Write token making, revoking and pruning. | Target `src/server/internal/machines/tokens.go`, `Tokens.Create`, and `machines/migrations/0002_client_join_tokens.sql`. Pattern: `src/mock-server/src/engine.ts`, `createClientToken`. Scouting S4. |
| 4 | Write the redemption and the `Join` call. | Targets `src/server/internal/machines/join.go`, `Tokens.Redeem`, `src/server/internal/link/join.go`, `Service.Join`, `src/server/internal/link/service.go`, `NewService`, and `src/server/internal/link/rules.go`. Pattern: `src/server/internal/auth/sessions.go`, `Sessions.Issue`. Scouting S2. |

**Item 1. Machine records**

The server gets one table of machines and the functions that read and write it.
Every later item reads a machine only through these functions (R3).

- **Files:** creates `src/server/internal/machines/store.go`, `src/server/internal/machines/link.go` (the `Link` interface and `Presence`), `src/server/internal/machines/migrations/0001_machines.sql`, and the test file `src/server/internal/machines/store_test.go`.
- **Needs first:** `go-s1-server-store` (the store, the feed and `migrated.NewStore`) and `go-s0-contract` (package `linkv1`). Runs at the same time as item 2.
- **Work area and complexity:** `backend`, medium. One table and its functions on the pattern of the sessions table.
- **Scope:** `server/backend`.
- **Statements:** D3, D5, D7, R1, R3, R4, R7, R10, and the design's sentence that the server stores the public key of each machine and no secret of any machine.
- **Risky:** yes, because it keeps the key that proves a machine. Removing `FOR SHARE` from `LockForStream` must fail `TestLockForStreamHoldsOffRemoval`. Dropping the index `machines_approved_name` must fail `TestApproveRenameAndNames`.
- **The table**, in `0001_machines.sql`:

```sql
CREATE TABLE machines (
    id text PRIMARY KEY CHECK (id ~ '^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'),
    public_key bytea NOT NULL CHECK (octet_length(public_key) = 32),
    host_name text NOT NULL CHECK (octet_length(host_name) <= 255),
    name text NOT NULL DEFAULT '' CHECK (octet_length(name) <= 64),
    approval text NOT NULL CHECK (approval IN ('waiting', 'approved')),
    state_revision bigint NOT NULL CHECK (state_revision >= 1),
    hosts_server boolean NOT NULL DEFAULT false,
    token_id text NOT NULL,
    joined_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    approved_at timestamptz,
    os text, cores integer, memory_bytes bigint, client_version text,
    workspace text, agent_limit integer, facts_at timestamptz,
    harnesses jsonb NOT NULL DEFAULT '[]'::jsonb,
    harnesses_scanned boolean NOT NULL DEFAULT false,
    last_seen_at timestamptz,
    last_address text NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX machines_approved_name ON machines (lower(name)) WHERE approval = 'approved';
```

- **What the functions do:**
  - `CreateWaiting` makes the identifier with `store.NewID`, inserts a row with approval `waiting` and revision 1, and records `store.ChangeRef{Topic: "machines", Key: id, Change: store.Saved}`.
  - `Get` and `List` read rows. `List` orders by `joined_at` and then `id`. `Get` returns `ErrNoMachine` for an unknown identifier.
  - `LockForStream` reads one row with `SELECT ... FOR SHARE`, so a removal waits until a handshake that holds the row has committed.
  - `ApplyFacts` writes the six fields of `Facts` and `facts_at = clock_timestamp()`. `ApplyHarnesses` writes `harnesses` as a JSON array of `{"name": ..., "version": ...}`, leaving out `version` when it is absent, keeps the order of the message, and sets `harnesses_scanned`. Both record a change of `machines`.
  - `NoteSeen` writes `last_seen_at` and `last_address`. `Touch` records a change of `machines` and writes nothing else. The link calls `Touch` when presence changes, so the page's topic stays owned by this part.
  - `SetApproved` refuses a row that is not waiting with `ErrNotWaiting`, sets approval, name, `approved_at` and the next revision, and records a change. `SetName` returns `false` and changes nothing when the name is the same, and otherwise sets it and the next revision. A unique violation of `machines_approved_name` becomes `ErrNameTaken`.
  - `Delete` deletes the row and records `store.Removed`. It returns `ErrNoMachine` when no row matched.
  - `NormalizeName` trims spaces at both ends and refuses an empty result, more than 64 bytes, invalid UTF-8 (Unicode Transformation Format, 8 bits) or a control character with `ErrInvalidName`.
  - `Standing` maps a record to `linkv1.MachineState` with `APPROVAL_WAITING` or `APPROVAL_APPROVED`, the name (empty while waiting) and the revision.
- **Tests written first,** in `store_test.go`, package `machines_test`:

| Test | What it asserts |
|---|---|
| `TestCreateWaitingAndRead` | `CreateWaiting` inside `Store.Write` returns a record whose identifier is a version 7 UUID (universally unique identifier). `Get` returns the same key, host name, token identifier and `HostsServer`, approval `waiting`, revision 1 and an empty name. The feed holds exactly one row with topic `machines`, scope empty, that key and `saved`. `Get` of a random identifier returns `ErrNoMachine`. A write that returns an error after `CreateWaiting` leaves no row and no feed row. |
| `TestApplyFactsAndHarnesses` | After `ApplyFacts`, `Get` returns the six fields unchanged. After `ApplyHarnesses` with `claude` at `2.4.1` and `codex` without a version, `Get` returns the two in that order, `codex` without a version, and `HarnessesScanned` true. Each call adds one `machines` feed row. |
| `TestApproveRenameAndNames` | `SetApproved` on a waiting machine gives approval `approved`, the name and revision 2. A second `SetApproved` on it returns `ErrNotWaiting`. Approving a second machine as the same name in other letter case returns `ErrNameTaken` and leaves that machine waiting. A waiting machine whose host name equals an approved name is no conflict. `SetName` to the same name returns `false` and keeps the revision. `SetName` to a new name returns `true` and revision 3. `NormalizeName` accepts `" Box "` as `Box` and refuses the empty string, 65 bytes, a tab and invalid UTF-8. |
| `TestLockForStreamHoldsOffRemoval` | Transaction A calls `LockForStream` and then waits on a gate of the test. Transaction B calls `Delete` on the same machine. While A waits, B has not returned after 250 milliseconds, and `pg_locks` shows B waiting for A. After the gate opens and A commits, B returns without an error. |
| `TestDeleteRecordsRemoval` | `Delete` removes the row and adds one feed row with `removed`. A second `Delete` returns `ErrNoMachine`. |

- **Acceptance:** `go test -count=1 -run 'TestCreateWaiting|TestApplyFacts|TestApproveRename|TestLockForStream|TestDeleteRecords' ./src/server/internal/machines/` passes against `LGTM_TEST_POSTGRES`.

**Item 3. Join tokens: make, revoke and prune**

The owner gets tokens from the Machines page and from the server's own command.
Each token works once, for 15 minutes, and the server keeps only a verifier.

- **Files:** creates `src/server/internal/machines/tokens.go`, `src/server/internal/machines/migrations/0002_client_join_tokens.sql` and the test file `src/server/internal/machines/tokens_test.go`.
- **Needs first:** item 1. Runs at the same time as item 5.
- **Work area and complexity:** `backend`, medium. One table, one encoding that the contract fixes, and one certificate read.
- **Scope:** `server/backend`.
- **Statements:** D7, D17, R4, R7, R9, C5, and the design's steps 1 and 2 of joining.
- **Risky:** yes, because it makes secrets. Storing the secret itself instead of its verifier must fail `TestTokenStoresOnlyVerifier`. A lifetime of 16 minutes must fail `TestTokenLifetime`.
- **The table**, in `0002_client_join_tokens.sql`:

```sql
CREATE TABLE client_join_tokens (
    id text PRIMARY KEY,
    secret_sha256 bytea NOT NULL CHECK (octet_length(secret_sha256) = 32),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    expires_at timestamptz NOT NULL,
    hosts_server boolean NOT NULL DEFAULT false,
    revoked_at timestamptz,
    used_at timestamptz,
    used_key bytea CHECK (used_key IS NULL OR octet_length(used_key) = 32),
    used_by text
);
```

- **What `Create` does:** it normalizes the address with `NormalizeAddress`, which writes the scheme and host in lowercase, removes `:80` for `http` and `:443` for `https`, removes a trailing slash, and refuses a path, a query, a user or another scheme. An empty `TokenOptions.Address` takes `Issuer.PublicURL`. Any other address that normalizes differently returns `ErrAddressMismatch`. For an `https` address it reads `Issuer.CertificateFile` on every call, decodes the first `CERTIFICATE` block, and hashes `RawSubjectPublicKeyInfo` with SHA-256 (Secure Hash Algorithm, 256 bits). A missing setting, an unreadable file or a file without a certificate returns `ErrNoCertificatePin`. It draws 32 secret bytes from `crypto/rand`, makes the identifier with `store.NewID`, and inserts the row with `secret_sha256` equal to SHA-256 of `lgtm-join-token-v1`, one zero byte and the secret, and `expires_at = clock_timestamp() + interval '15 minutes'`. It records `client_join_tokens` with the token's identifier. It returns the text that `EncodeToken` makes from `JoinToken{token_id, secret, address, server_key_sha256 = SHA-256(Issuer.ServerKey), certificate_key_sha256}`, and the `expires_at` read back from the row.
- **`EncodeToken` and `DecodeToken`:** the text is `lgtm1_` followed by `proto.MarshalOptions{Deterministic: true}` of the message in unpadded base64url, as the contract's item 5 fixes. `DecodeToken` refuses another prefix, padding and trailing bytes.
- **What `Revoke` does:** it locks the row with `FOR UPDATE`. An unknown identifier is no error and returns an empty string. It sets `revoked_at` when it is empty and records a change. When `used_by` names a machine that is still waiting, it returns that machine's identifier, and the caller removes the machine in the same transaction.
- **What `Prune` does:** it deletes tokens whose `expires_at` is more than 24 hours past, except those whose `used_by` machine still waits. It returns the count.
- **Tests written first,** in `tokens_test.go`, package `machines_test`:

| Test | What it asserts |
|---|---|
| `TestCreateTokenText` | With `PublicURL` `http://127.0.0.1:8787`, the text starts with `lgtm1_` and `DecodeToken` gives a token whose `token_id` equals `NewToken.ID` and is a version 7 UUID, whose secret has 32 bytes, whose address is `http://127.0.0.1:8787`, whose `server_key_sha256` is SHA-256 of the issuer's key, and whose `certificate_key_sha256` is empty. `EncodeToken` of the message in `src/contracts/testdata/join-token.json` gives that file's `text`, and `DecodeToken` of the text gives the file's fields. |
| `TestTokenLifetime` | `expires_at - created_at` in the row is exactly 15 minutes, and `NewToken.ExpiresAt` equals the row's `expires_at`. |
| `TestTokenStoresOnlyVerifier` | The row's `secret_sha256` equals SHA-256 of `lgtm-join-token-v1`, a zero byte and the secret. Neither the secret bytes nor their hexadecimal or base64 form appear in any column of the row, read as text with `row_to_json`, nor in the feed. |
| `TestTokenPinsCertificateKey` | With an `https` public address and a self-signed certificate that the test writes in Privacy-Enhanced Mail (PEM) form, `certificate_key_sha256` equals SHA-256 of the certificate's Subject Public Key Info. A second certificate with the same key gives the same pin. A missing file, an empty `CertificateFile` and a file with no certificate each return `ErrNoCertificatePin` and insert no row. |
| `TestTokenAddressMustMatch` | `HTTP://127.0.0.1:8787/` and an empty address both give `http://127.0.0.1:8787`. `https://Box.example:443` normalizes to `https://box.example`. `http://127.0.0.1:8899` against a public address with port 8787 returns `ErrAddressMismatch`. An address with a path is refused. |
| `TestHostsServerMark` | A token made with `HostsServer` true has `hosts_server` true in its row, and one made without it has false. |
| `TestRevokeToken` | Revoking an unused token sets `revoked_at` and returns an empty string. Revoking an unknown identifier returns no error. A token used by a waiting machine returns that machine's identifier. A token used by an approved machine returns an empty string. |
| `TestPruneTokens` | A token that expired 25 hours ago is deleted, one that expired 23 hours ago is kept, and one that expired 25 hours ago whose machine still waits is kept. |

- **Acceptance:** `go test -count=1 -run 'TestCreateTokenText|TestTokenLifetime|TestTokenStoresOnlyVerifier|TestTokenPinsCertificateKey|TestTokenAddressMustMatch|TestHostsServerMark|TestRevokeToken|TestPruneTokens' ./src/server/internal/machines/` passes.

**Item 4. Joining: redeem a token, and the `Join` call**

A new machine trades its token for the server's key and becomes a waiting machine.
The call is reached without a session, so it checks everything itself.

- **Files:** creates `src/server/internal/machines/join.go`, `src/server/internal/link/service.go`, `src/server/internal/link/join.go`, `src/server/internal/link/rules.go`, and the test files `src/server/internal/machines/join_test.go` and `src/server/internal/link/join_test.go`.
- **Needs first:** items 3 and 5 (for the type `State` in `link.Options`). Runs at the same time as items 6, 7 and 11.
- **Work area and complexity:** `backend`, medium. One transaction with a fixed order of checks, and one thin handler.
- **Scope:** `server/backend`.
- **Statements:** D7, D17, R4, R8, R9, R10, C5, C6, and the design's steps 3 to 6 of joining.
- **Risky:** yes, because it decides who may register a key. Checking the expiry after the replay must fail `TestRedeemRefusals`. Accepting a replay with another key must fail `TestRedeemRefusals`. Comparing the digests with `bytes.Equal` must fail `TestSecretCompareIsConstantTime`.
- **What `Redeem` does,** in one `Store.Write`:
  1. Lock the token row with `FOR UPDATE`. No row gives `RefusalUnknown`.
  2. Compare the verifier of the given secret with `secret_sha256` through `crypto/subtle.ConstantTimeCompare`. A mismatch gives `RefusalUnknown`.
  3. `expires_at <= clock_timestamp()` gives `RefusalExpired`.
  4. `revoked_at` set gives `RefusalRevoked`.
  5. `used_key` set and different from the given key gives `RefusalUsed`.
  6. `used_key` equal to the given key: when `used_by` still exists, return `Joined{MachineID: used_by, Replay: true}` and write nothing. Otherwise give `RefusalMachineRemoved`.
  7. Otherwise call `Registry.CreateWaiting` with the key, the host name, the token's `hosts_server` and the token's identifier, set `used_at`, `used_key` and `used_by`, and record `client_join_tokens`.

  A refusal is returned as `*RefusedError` and commits nothing. Any other error is returned as it is.
- **What `Service.Join` does:** it checks the value rules of the contract through `rules.go`: `token_id` a canonical UUID, `token_secret` 32 bytes, `machine_public_key` 32 bytes, `host_name` valid UTF-8 of at most 255 bytes with no control character. A broken rule gives `invalid_argument` and touches no row. It calls `Redeem`. A `*RefusedError` gives `permission_denied` with the sentence of its reason and the response header `Lgtm-Join-Refusal` set to the reason. A store error gives `unavailable`. Success gives `JoinResponse{machine_id, server_public_key}`, with the public half of `Options.SigningKey`. It reads no `Lgtm-Request-Key` and logs no field of the request except the reason of a refusal.
- **`rules.go`:** the private helpers `validUUID(s string) bool` and `validText(s string, max int) bool`, used by items 4, 7 and 8.
- **`HandlerOptions`:** `connect.WithReadMaxBytes(MaxMessageBytes)`, for `Join` and `Open`.
- **Tests written first,** in `machines/join_test.go`, package `machines_test`:

| Test | What it asserts |
|---|---|
| `TestRedeemCreatesWaitingMachine` | A fresh token, its secret, a new key and the host name `box` give `Replay` false and a machine that waits, with revision 1, host name `box`, the token's identifier and `HostsServer` copied from the token. The token row has `used_key`, `used_by` and `used_at` set. The feed has one `machines` row for the new machine. |
| `TestRedeemReplay` | A second `Redeem` with the same token, secret and key, and the host name `other`, gives the same machine identifier and `Replay` true. One machine exists, and its host name is still `box`. |
| `TestRedeemRefusals` | Each case starts from its own token and gives exactly the named reason, and afterwards the count of machines and the token row are unchanged: an unknown identifier gives `token_unknown`; a wrong secret gives `token_unknown`; a wrong secret on an expired token gives `token_unknown`; a token whose `expires_at` the test set one second into the past gives `token_expired`; a revoked token gives `token_revoked`; a revoked token that also expired gives `token_expired`; a used token with another key gives `token_used`; a used token whose machine was deleted, with the first key, gives `machine_removed`; a replay of a used token after its expiry gives `token_expired`. |
| `TestRedeemConcurrentKeys` | Two goroutines redeem one token at the same moment with two keys, held at a gate so both have started. Exactly one returns a machine, the other returns `token_used`, and one machine exists. |
| `TestRedeemRollsBack` | A trigger of the test raises an error on the insert into `machines`. `Redeem` returns an error that is not a `*RefusedError`, no machine exists, and the token row's `used_key` is still empty. After the trigger is dropped, the same request succeeds. |
| `TestSecretCompareIsConstantTime` | It parses `join.go` with `go/parser` and asserts that the comparison of the verifier calls `subtle.ConstantTimeCompare`, and that no `bytes.Equal` or `==` compares the two digests. |

- **Tests written first,** in `link/join_test.go`, package `link_test`, against a `link.Service` served through h2c:

| Test | What it asserts |
|---|---|
| `TestJoinCall` | `Join` with a fresh token answers a `machine_id` that is a version 7 UUID and a `server_public_key` whose SHA-256 equals the token's `server_key_sha256`. The same request again answers the same `machine_id`. |
| `TestJoinRefusalCodes` | For each of the five reasons, `Join` answers `permission_denied`, the error's message is that reason's sentence, and the response metadata holds `Lgtm-Join-Refusal` with the reason. No header and no message holds any part of the token's text or secret. |
| `TestJoinValueRules` | A token identifier in upper case, a secret of 31 bytes, a key of 33 bytes, a host name of 256 bytes and a host name with invalid UTF-8 each answer `invalid_argument`, and the token row is unchanged. |
| `TestJoinDatabaseDown` | After the test closes the store, `Join` answers `unavailable` and never `permission_denied`. |
| `TestJoinKeepsSecretsOutOfLogs` | After a success and one refusal, the log buffer holds neither the token's text, nor its secret in hexadecimal, base64 or base64url. |

- **Acceptance:** `go test -count=1 -run 'TestRedeem|TestSecretCompare' ./src/server/internal/machines/` and `go test -count=1 -run 'TestJoin' ./src/server/internal/link/` pass.

**Design artifact:** not applicable. No screen changes. The page's pairing dialog already shows a token and waits for the machine.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| A token works twice, for two keys at once. | High | The token row is locked for the whole redemption. `TestRedeemConcurrentKeys` races two keys. |
| A refusal tells a caller without the secret whether a token exists or was used. | Medium | A wrong secret gives `token_unknown` before any other check, also for an expired token. `TestRedeemRefusals` covers it. |
| A token's secret reaches the database, a log or the feed. | High | Only the verifier is stored. `TestTokenStoresOnlyVerifier` and `TestJoinKeepsSecretsOutOfLogs` search for the secret in every encoding. |
| A token carries an address or a pin that the machine cannot use. | Medium | One normalized public address feeds every token, and `https` needs a readable certificate. `TestTokenAddressMustMatch` and `TestTokenPinsCertificateKey`. |

**Verify:** `deploy/test/with-postgres.sh -- go test -race -count=1 ./src/server/internal/machines/... ./src/server/internal/link/...`. Environment check: `deploy/test/checks.sh --only C5,C6` on the stack at the end of the sprint. No device is needed.

**Rollout detail:** rollout steps 1 to 3. The two database changes of the part `machines` are applied by `Store.Migrate` at start-up, after item 14 registers `machines.Migrations()`.

**WS-2: A machine proves itself and stays connected**

This workstream gives the server its half of the stream that every machine keeps open.
It checks the proof of both ends, decides which stream of a machine is current, tracks presence by heartbeats, and ends a stream that falls silent or blocks.
It implements the design's sections "One connection, dialled by the machine", "Both ends prove themselves on every connection" and "Knowing whether a machine is there", and checks C8, C9 and C10.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 5 | The proof texts are the contract's bytes, and a challenge comes before any lookup. | `TestProofTextsMatchContractExamples` and `TestChallengeComesBeforeLookup` pass. |
| 6 | A newer stream replaces an older one, and the older one changes no row. | `TestGenerationsFollowCommitOrder`, `TestOldStreamCannotChangeRecord`, `TestOlderGenerationCannotReplaceNewer` and `TestNewStreamReplacesOld` pass. |
| 7 | A silent or blocked stream ends, and the machine shows offline. A passed lease shows offline after a crash. | `TestSilenceEndsStream`, `TestBlockedMachineEndsStream`, `TestSweepExpiredLease` and `TestPresenceIgnoresPassedLease` pass. |
| 8 | On the stack, a stopped client shows offline within 30 seconds, a server restart brings it back, and an unknown key is refused. | `deploy/test/checks.sh --only C8,C9,C10` prints three lines that start with `ok` and exits 0. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 2 | Write the write deadline for long streams, and wrap the two route families. | Target `src/server/internal/transport/deadline.go`, `WithWriteDeadline` and `Send`. Changes `src/server/internal/api/server.go`, `NewHandler`, of `go-s2-server-signin`. Pattern: `net/http.ResponseController`. Scouting S6. |
| 5 | Write the link's tables, becoming current, heartbeats, disconnection and the presence sweep. | Target `src/server/internal/link/presence.go`, `State.BecomeCurrent`, and `src/server/internal/link/migrations/0001_link.sql`. Pattern: `src/server/internal/store/receipts.go`, `WriteOnce`, of `go-s1-server-store`. Scouting S5. |
| 8 | Write the handshake and the scripted machine. | Targets `src/server/internal/link/handshake.go`, `ServerProofText`, `src/server/internal/link/stream.go`, `Service.Open`, and `src/server/internal/link/machinetest/machine.go`, `Machine.Connect`. Pattern: `src/contracts/open_test.go`, `TestProofExamplesHoldTogether`, of `go-s0-contract`. Scouting S2. |
| 9 | Write the hub of open streams and its feed follower. | Target `src/server/internal/link/hub.go`, `Hub.Register` and `Hub.Follow`. Pattern: `src/server/internal/store/follow.go`, `Follower`, of `go-s1-server-store`. Scouting S5. |
| 10 | Write the loop of one stream after `Welcome`, and the service's background work. | `src/server/internal/link/stream.go`, `Service.Open`, and target `src/server/internal/link/run.go`, `Service.Run`. Scouting S2 and S8. |

**Item 2. Write deadlines for long streams**

A send on a long stream must end when the other side stops reading, or its handler never returns.
This item gives every long stream one way to set a deadline on its next write.

- **Files:** creates `src/server/internal/transport/deadline.go` and its test file `src/server/internal/transport/deadline_test.go`. Changes `src/server/internal/api/server.go`: `NewHandler` wraps the handlers of `ClientService` and of `LinkService` in `transport.WithWriteDeadline`. Adds `TestRouterWrapsLongStreams` to `src/server/internal/api/server_test.go`.
- **Needs first:** `go-s2-server-signin`, item 6. Runs at the same time as item 1.
- **Work area and complexity:** `backend`, low. One middleware of the standard library and a one-line change in the router.
- **Scope:** `server/backend`.
- **Statements:** R5, R7, C8, and the store plan's rule that a blocked response ends at the transport.
- **Risky:** no.
- **What it does:** `WithWriteDeadline` makes `http.NewResponseController(w)` and puts a setter of its write deadline into the request's context. `Send` sets the deadline to now plus `SendTimeout`, calls `send`, and clears the deadline with the zero time, also when `send` fails. Without the setter in the context, `Send` returns `ErrNoDeadline` and does not call `send`. A private variable `sendTimeout` lets a test shorten the 10 seconds.
- **Tests written first:**

| Test | What it asserts |
|---|---|
| `TestDeadlineEndsBlockedStream` | In `deadline_test.go`. An h2c test server's handler, wrapped, writes 64 kibibytes at a time through `Send` and flushes, with `sendTimeout` at 200 milliseconds. The client reads the headers and then nothing. The handler's `Send` returns an error within 2 seconds, and the handler returns. |
| `TestDeadlineClearedAfterSend` | A handler sends once, waits 400 milliseconds with `sendTimeout` at 200 milliseconds, and sends again while the client reads. Both sends succeed. |
| `TestSendNeedsTheMiddleware` | Without `WithWriteDeadline`, `Send` returns `ErrNoDeadline` and the function it was given is not called. |
| `TestRouterWrapsLongStreams` | In `api/server_test.go`. Probe handlers for `/lgtm.v1.ClientService/WatchClients` and `/lgtm.link.v1.LinkService/Open` call `transport.Send` with a function that returns nil, and both get no error. |

- **Acceptance:** `go test -count=1 ./src/server/internal/transport/` and `go test -count=1 -run TestRouterWrapsLongStreams ./src/server/internal/api/` pass.

**Item 5. The link's tables, the current stream and presence**

The server records, for each machine, which stream is current and when it last heard from it.
Nothing about presence lives only in memory, so a restarted server reads it again.

- **Files:** creates `src/server/internal/link/presence.go`, `src/server/internal/link/migrations/0001_link.sql` and the test file `src/server/internal/link/presence_test.go`.
- **Needs first:** item 1. Runs at the same time as item 3.
- **Work area and complexity:** `backend`, medium. Row locks inside one write per operation, on the pattern of the store's receipts.
- **Scope:** `server/backend`.
- **Statements:** D3, D7, R3, R4, R7, R10, C8, C9, and the design's sentences that a new connection replaces the older one, that the server sends a short-lived token for the connection, and that a machine is offline when its stream closes or after 30 seconds without a heartbeat.
- **Risky:** yes, because it decides which stream counts. Computing the next generation in Go from a plain read must fail `TestGenerationsFollowCommitOrder`. Dropping the condition on `stream_id` from `Disconnect` must fail `TestOldStreamCannotChangeRecord`.
- **The tables**, in `0001_link.sql`:

```sql
CREATE TABLE link_machines (
    machine_id text PRIMARY KEY,
    event_mark bigint NOT NULL DEFAULT 0 CHECK (event_mark >= 0),
    stream_generation bigint NOT NULL DEFAULT 0 CHECK (stream_generation >= 0)
);
CREATE TABLE link_connections (
    machine_id text PRIMARY KEY,
    stream_id text NOT NULL UNIQUE,
    generation bigint NOT NULL,
    connected_at timestamptz NOT NULL,
    last_heard_at timestamptz NOT NULL,
    lease_until timestamptz NOT NULL,
    address text NOT NULL DEFAULT '',
    cpu_percent smallint CHECK (cpu_percent BETWEEN 0 AND 100),
    memory_percent smallint CHECK (memory_percent BETWEEN 0 AND 100),
    facts_received boolean NOT NULL DEFAULT false,
    harnesses_received boolean NOT NULL DEFAULT false,
    token_sha256 bytea CHECK (token_sha256 IS NULL OR octet_length(token_sha256) = 32)
);
CREATE INDEX link_connections_lease ON link_connections (lease_until);
```

- **What `BecomeCurrent` does,** in one `Store.Write`: it calls `Registry.LockForStream`, and `ErrNoMachine` becomes `ErrUnknownMachine`. It inserts the machine's `link_machines` row when it is missing, and raises `stream_generation` by one with `UPDATE ... RETURNING`, under that row's lock. It makes a stream identifier with `store.NewID`. For an approved machine it makes a connection token of 32 random bytes in unpadded base64url and stores only its SHA-256. It upserts `link_connections` with the new stream, the generation, `connected_at = last_heard_at = clock_timestamp()`, `lease_until = clock_timestamp() + interval '35 seconds'`, the address, no load and both received flags false. It reads `event_mark` and `clock_timestamp()` for `Stream.Mark` and `Stream.ServerTime`, and calls `Registry.Touch`.
- **What `Heartbeat` does:** it updates the row `WHERE machine_id = $1 AND stream_id = $2` with the load, `last_heard_at` and a lease 35 seconds from now, and calls `Touch`. No row gives `ErrNotCurrent`.
- **What `Disconnect` does:** it deletes the row `WHERE machine_id = $1 AND stream_id = $2 RETURNING last_heard_at, address`. When it deleted one, it calls `Registry.NoteSeen` with those values and `Touch`, and returns `true`. Otherwise it returns `false` and writes nothing. It runs with its own context of 5 seconds, so it also runs after the stream's context ended.
- **What `SweepPresence` does:** it deletes every row whose `lease_until` has passed, and for each calls `NoteSeen` and `Touch` in the same transaction.
- **What `Presence` does:** it reads the rows of the given machines whose lease has not passed, so a row of a dead process shows nobody even before a sweep.
- **What `Forget` does:** it deletes the machine's rows in `link_machines`, `link_connections` and `link_commands`. Item 6 adds the last table, so `Forget` deletes from it once that table exists.
- **What `CheckConnectionToken` does:** it hashes the token, finds the connection row with that hash and an unpassed lease, and returns its machine. It is for the calls of wave 2 beside the stream.
- **The address:** the private `peerAddress(remoteAddr string, h http.Header, behindProxy bool) string` takes the last entry of `X-Forwarded-For` when `behindProxy` is true, and the host of `remoteAddr` otherwise. A value that `net/netip.ParseAddr` refuses gives an empty string.
- **A test seam:** a private field `afterGenerationLock func(context.Context)` of `State`, nil in production, runs right after the generation update.
- **Tests written first,** in `presence_test.go`, package `link`:

| Test | What it asserts |
|---|---|
| `TestBecomeCurrent` | For an approved machine, `BecomeCurrent` returns generation 1, a stream identifier, a connection token of 43 characters, `Mark` 0, the standing and a `ServerTime` within one second of the database's clock. The row holds the token's SHA-256 and never the token. For a waiting machine the token is empty and `token_sha256` is null. For an unknown machine it returns `ErrUnknownMachine` and writes no row. |
| `TestGenerationsFollowCommitOrder` | Stream A's `BecomeCurrent` stops at `afterGenerationLock`. Stream B's `BecomeCurrent` for the same machine starts and has not returned after 250 milliseconds. After A is released, A has generation 1 and B generation 2, and the row names B's stream. |
| `TestOldStreamCannotChangeRecord` | After B became current, `Heartbeat` with A returns `ErrNotCurrent` and leaves B's row unchanged, and `Disconnect` with A returns `false` and leaves the row. |
| `TestHeartbeatRenewsLease` | After `Heartbeat` with 7 and 28, the row holds those loads, `last_heard_at` is newer, `lease_until` is 35 seconds after it, and the feed has one more `machines` row. A heartbeat without load stores nulls. |
| `TestDisconnectMarksOffline` | `Disconnect` of the current stream returns `true`, removes the row, sets the record's `LastSeenAt` and `LastAddress`, and adds a `machines` feed row. `Presence` then has no entry for the machine. |
| `TestSweepExpiredLease` | A row whose `lease_until` the test set into the past is deleted by `SweepPresence`, with `NoteSeen` and a feed row. A row with a future lease is kept. |
| `TestPresenceIgnoresPassedLease` | A row whose lease passed and that no sweep has deleted yet gives no entry in `Presence`. |
| `TestForget` | After `Forget`, the machine has no row in `link_machines` or `link_connections`. |
| `TestCheckConnectionToken` | The token of the current stream gives its machine. After `Disconnect` it gives `ErrNotCurrent`. A random token gives `ErrNotCurrent`. |
| `TestPeerAddress` | Behind the proxy, `X-Forwarded-For: 203.0.113.9, 198.51.100.4` gives `198.51.100.4`. Not behind it, the header is ignored and `remoteAddr` `127.0.0.1:5555` gives `127.0.0.1`. A value that is not an address gives an empty string. |

- **Acceptance:** `go test -count=1 -run 'TestBecomeCurrent|TestGenerations|TestOldStream|TestHeartbeatRenews|TestDisconnect|TestSweepExpired|TestPresenceIgnores|TestForget|TestCheckConnectionToken|TestPeerAddress' ./src/server/internal/link/` passes.

**Item 8. The handshake**

Both ends prove themselves before anything else happens on a stream.
The server signs first, and it looks the machine up only after the machine's proof, so a stranger learns nothing about which identifiers exist.

- **Files:** creates `src/server/internal/link/handshake.go`, `src/server/internal/link/stream.go`, `src/server/internal/link/machinetest/machine.go`, and the test files `src/server/internal/link/handshake_test.go` and `src/server/internal/link/machinetest/machine_test.go`. Changes `src/server/internal/link/service.go` to keep the hub and the state.
- **Needs first:** items 4 and 5. Runs at the same time as items 9, 12 and 13.
- **Work area and complexity:** `backend`, medium. The bytes are fixed by worked examples, and the order is a short state machine.
- **Scope:** `server/backend`.
- **Statements:** D7, D17, R8, R10, C10, and the design's section "Both ends prove themselves on every connection".
- **Risky:** yes, because it proves both ends. Building the server's text with the server nonce before the machine nonce must fail `TestProofTextsMatchContractExamples`. Answering an unknown identifier before sending `Challenge` must fail `TestChallengeComesBeforeLookup`.
- **The proof texts:** each is the label, one zero byte, one byte with the length of the machine identifier, the identifier in UTF-8, the machine nonce and the server nonce. The labels are `lgtm-link-v1 server` and `lgtm-link-v1 machine`.
- **What `Open` does in this item:** a reader goroutine passes each received message to a channel. Within `HandshakeTimeout` of the stream's start, the first message must be a `Hello` with a canonical identifier, a 32-byte nonce and `link_revision` 1. A revision other than 1 gives `failed_precondition`, a broken value rule or another kind gives `invalid_argument`, and lateness gives `deadline_exceeded`. The server draws a 32-byte nonce, signs `ServerProofText` with `Options.SigningKey`, and sends `Challenge` through `transport.Send`. Within `HandshakeTimeout` after that, the next message must be a `Proof` with a 64-byte signature. Only then does the server read the machine with `Registry.Get`. An unknown machine or a signature that `ed25519.Verify` refuses against the stored key gives `unauthenticated`. It then calls `State.BecomeCurrent`, where `ErrUnknownMachine` also gives `unauthenticated`. A store error at any step gives `unavailable`. It registers the stream in the hub. `ErrSuperseded` gives `aborted`. It sends `Welcome{connection_token, state = machines.Standing(rec), last_event_number = Mark, server_time = ServerTime, link_revision = 1}` and logs `link_stream_opened`. In this item the stream then waits for its end, and calls `Disconnect` and `Hub.Unregister` when it ends. Item 10 replaces that wait with the loop.
- **The scripted machine:** `machinetest.NewServer` serves a handler over h2c with `httptest` and returns its address and a generated client. `Join` decodes a token's text, makes a key with `crypto/ed25519`, calls `Join`, checks the server key against the token, and returns a `Machine`. `Dial` opens a raw stream on which the test sends what it wants. `Handshake` sends `Hello` and `Proof` and verifies the `Challenge` against `ServerKey`. `Connect` runs `Dial` and `Handshake` and fails the test on any error.
- **Tests written first,** in `handshake_test.go`, package `link_test`, against a real `link.Service` and PostgreSQL:

| Test | What it asserts |
|---|---|
| `TestProofTextsMatchContractExamples` | With the values of `src/contracts/testdata/link-proof.json`, `ServerProofText` and `MachineProofText` equal the file's `server_text` and `machine_text`, and `ed25519.Sign` with the seed of 32 bytes of `01` over the server's text equals `server_signature`. |
| `TestHandshakeOpensStream` | An approved scripted machine gets a `Challenge` whose signature verifies with the server's key, then a `Welcome` with `link_revision` 1, approval `APPROVAL_APPROVED`, its name and revision, `last_event_number` 0, a `server_time` and a connection token of 43 characters. `Presence` then shows it connected. |
| `TestChallengeComesBeforeLookup` | A `Hello` with a random identifier gets a `Challenge` that verifies. A `Proof` signed by any key then gets `unauthenticated` with the sentence "The server does not know this machine. Join it again.", and no connection row exists. |
| `TestBadProofRefused` | A machine that signs its text with another key, and one that signs a text with the two nonces swapped, each get `unauthenticated` and no connection row. |
| `TestHandshakeValueRules` | A nonce of 31 bytes, an identifier in upper case and an empty first message each get `invalid_argument`. A `Heartbeat` as the first message gets `invalid_argument`. `link_revision` 2 gets `failed_precondition`. |
| `TestHandshakeTimeouts` | With the private timeout at 200 milliseconds, a stream that sends nothing ends with `deadline_exceeded`, and a stream that sends `Hello` and no `Proof` ends with `deadline_exceeded`. |
| `TestRemovedDuringHandshake` | The test deletes the machine after the `Challenge` and before the `Proof`. The stream ends with `unauthenticated`. |
| `TestWaitingMachineWelcome` | A waiting machine gets `APPROVAL_WAITING`, an empty name and an empty connection token. |
| `TestHandshakeDatabaseDown` | The test closes the store after the `Challenge`. The stream ends with `unavailable`. |

- **Tests written first,** in `machinetest/machine_test.go`: `TestScriptedMachineJoinsAndConnects` makes a token through `Tokens.Create`, joins with `machinetest.Join`, approves the machine through `Registry.SetApproved`, and asserts that `Connect` returns a `Welcome`.
- **Acceptance:** `go test -count=1 -run 'TestProofTexts|TestHandshake|TestChallenge|TestBadProof|TestRemovedDuring|TestWaitingMachineWelcome' ./src/server/internal/link/` and `go test -count=1 ./src/server/internal/link/machinetest/` pass.

**Item 9. The hub of open streams**

Each server process keeps a list of the streams it holds, so it can wake a stream when its machine has a new command and end it when the machine is removed or replaced.
The list is only a cache of live connections, and the database stays the record (R7).

- **Files:** creates `src/server/internal/link/hub.go` and the test file `src/server/internal/link/hub_test.go`.
- **Needs first:** items 1 and 6 (for `OutboxTopic`). Runs at the same time as items 8, 12 and 13.
- **Work area and complexity:** `backend`, medium. A map under one lock, and one subscriber of the store's follower.
- **Scope:** `server/backend`.
- **Statements:** R5, R6, R7, C9, C10, and the design's sentence that a new connection from a machine replaces its older one.
- **Risky:** yes, because it orders streams. Letting `Register` replace a handle of a higher generation must fail `TestOlderGenerationCannotReplaceNewer`.
- **What it does:** `Register` returns `ErrSuperseded` when the machine's handle has a higher generation. Otherwise it keeps the new handle and sends the older one `connect.NewError(connect.CodeAborted, ...)` on its end channel. `Unregister` removes a handle only when it is still the machine's handle. `Wake` sends on a channel of size 1 without blocking. `End` sends an error to the machine's handle. `CloseAll` ends every handle. `Follow` subscribes to the follower, starts at the head that `Store.Snapshot` reads, wakes every handle once, and then for each feed row: `machine_outbox` wakes the key's handle, and `machines` with `removed` ends the key's handle with `unauthenticated` and the sentence "The server does not know this machine. Join it again." It acknowledges every row. On `store.ErrWatchLagged` or `store.ErrFeedUnavailable` it waits one second, subscribes again and wakes every handle. It returns when its context ends.
- **Tests written first,** in `hub_test.go`, package `link`:

| Test | What it asserts |
|---|---|
| `TestRegisterReplacesLowerGeneration` | After `Register(m, 1)` and `Register(m, 2)`, the first handle's `Ended` yields an error with the code `aborted`, and the second handle's `Ended` yields nothing within 250 milliseconds. |
| `TestOlderGenerationCannotReplaceNewer` | After `Register(m, 2)`, `Register(m, 1)` returns `ErrSuperseded`, and the handle of generation 2 is not ended. |
| `TestUnregisterOnlyOwnHandle` | After generation 2 replaced generation 1, `Unregister` of the first handle leaves the second, and `Wake(m)` still reaches the second. |
| `TestWakeCoalesces` | Two `Wake` calls before a read give one signal on `Woken`. `Wake` of an unknown machine returns at once. |
| `TestFollowWakesAndEnds` | With a real store and follower, a write that records `machine_outbox` for the machine wakes its handle within 3 seconds. A write that records `machines` `removed` for it ends the handle with the code `unauthenticated`. A row of another topic does neither. |
| `TestFollowResubscribes` | A private seam makes the first subscription fail with `store.ErrWatchLagged`. `Follow` subscribes again and wakes every handle once. |
| `TestCloseAllEndsEveryHandle` | `CloseAll` with an `unavailable` error ends two handles of two machines with that code. |

- **Acceptance:** `go test -count=1 -run 'TestRegister|TestOlderGeneration|TestUnregister|TestWake|TestFollow|TestCloseAll' ./src/server/internal/link/` passes.

**Item 10. The stream after `Welcome`**

One loop runs each connection after its handshake.
It answers heartbeats, applies reports, delivers commands, takes results, and ends the stream when it falls silent, blocks, is replaced or is removed.

- **Files:** changes `src/server/internal/link/stream.go`. Creates `src/server/internal/link/run.go` and the test files `src/server/internal/link/stream_test.go` and `src/server/internal/link/run_test.go`.
- **Needs first:** items 2, 6, 7, 8 and 9.
- **Work area and complexity:** `backend`, medium. It joins functions that exist and are tested, in one select loop with one writer.
- **Scope:** `server/backend`.
- **Statements:** D7, D16, R6, R7, R10, C7, C8, C9, C11, and the design's sections "Commands go down, and none is lost or repeated", "Events come up, and none is lost or applied twice" and "Knowing whether a machine is there".
- **Risky:** yes, because it orders events and commands on the wire. Sending `EventAck` before `ApplyEvent` returns must fail `TestEventsAckedAfterCommit`. Sending without `transport.Send` must fail `TestBlockedMachineEndsStream`.
- **The loop:** one goroutine reads, and the handler's goroutine is the only writer, which sends every message through `transport.Send`. It starts by claiming and sending pending commands. It then selects on:
  - a message: `heartbeat` calls `State.Heartbeat` and sends `HeartbeatAck`. A load above 100 ends the stream with `invalid_argument`. `event` calls `State.ApplyEvent`, sends `EventAck{through: Mark}` after it returns, and maps `ErrEventGap` to `out_of_range` and `ErrInvalidReport` to `invalid_argument`. `command_result` calls `State.AcceptResult`, and `ErrInvalidReport` gives `invalid_argument`. A `hello` or a `proof` gives `invalid_argument`. A message with no kind set is ignored, because a newer client may send a kind this server does not know.
  - the handle's wake: claim and send pending commands.
  - the handle's end: return that error.
  - a timer of `HeartbeatTimeout`, reset after each heartbeat: return `deadline_exceeded`.
  - `ErrNotCurrent` from any call: return `aborted`.

  When it returns for any reason, it calls `Disconnect` and `Hub.Unregister`, and logs `link_stream_closed` with the code.
- **What `Run` does:** it runs `Hub.Follow`, `State.SweepPresence` every `SweepInterval`, and `State.SweepCommands` and `Tokens.Prune` every hour, until its context ends. It reports an error of a sweep in one log line and tries again at the next tick. When its context ends it calls `Hub.CloseAll` with `unavailable` and the sentence "The server is stopping."
- **Tests written first,** in `stream_test.go`, package `link_test`, with the scripted machine:

| Test | What it asserts |
|---|---|
| `TestHeartbeatIsAnswered` | A heartbeat with 7 and 28 gets `HeartbeatAck`, and `Presence` shows those loads. A heartbeat with 101 ends the stream with `invalid_argument`. |
| `TestSilenceEndsStream` | With the private timeout at 300 milliseconds, a machine that sends nothing after `Welcome` sees the stream end with `deadline_exceeded` within one second, and `Presence` shows it gone. |
| `TestEventsAckedAfterCommit` | Events 1, 2 and 3 get `EventAck` with 1, 2 and 3. Event 2 again gets `EventAck` with 3. Event 5 ends the stream with `out_of_range`. On a new stream, a trigger of the test that fails the mark's update makes event 4 end the stream with `unavailable`, and no `EventAck` with 4 arrives. The log holds `link_event_applied` with sequences 1, 2 and 3, once each and in that order. |
| `TestCommandsDeliveredAndAnswered` | A `ScanHarnesses` command stored for a connected machine arrives with attempt 1 within 3 seconds. A result `COMMAND_OUTCOME_DONE` for attempt 1 is stored. A second command left without a result arrives again with attempt 2 on the next stream, with the same identifier and expiry. |
| `TestNewStreamReplacesOld` | A second stream of the same machine gets `Welcome`, and the first ends with `aborted`. A heartbeat that the test sent on the first just before it ended changes nothing, and `Presence` names the second stream. |
| `TestRemovalEndsStream` | A write that calls `Registry.Delete` and `State.Forget` for a connected machine ends its stream with `unauthenticated` within 3 seconds. |
| `TestApprovalReachesWaitingStream` | A connected waiting machine receives a `SetState` command stored for it within 3 seconds. A stored `ScanHarnesses` command does not arrive within 500 milliseconds. |
| `TestBlockedMachineEndsStream` | With `transport`'s timeout at 200 milliseconds, a scripted machine stops reading after `Welcome`, and the test stores 50 commands for it. The stream ends within 3 seconds, and `Presence` shows it gone. |
| `TestUnknownKindIgnored` | After `Welcome`, an `OpenRequest` with no kind set changes nothing, and a later heartbeat still gets `HeartbeatAck`. |

- **Tests written first,** in `run_test.go`:

| Test | What it asserts |
|---|---|
| `TestRunSweeps` | With private intervals of 100 milliseconds, `Run` deletes a connection row whose lease passed and finishes a command 25 hours past its expiry within one second. |
| `TestRunStopsStreams` | Cancelling the context of `Run` ends a connected stream with `unavailable` within one second, and its connection row is gone. |

- **Acceptance:** `go test -count=1 -run 'TestHeartbeatIsAnswered|TestSilence|TestEventsAcked|TestCommandsDelivered|TestNewStream|TestRemovalEnds|TestApprovalReaches|TestBlockedMachine|TestUnknownKind|TestRun' ./src/server/internal/link/` passes.

**Design artifact:** not applicable. No screen changes.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| An old stream that ends late marks a connected machine offline. | High | Every write names its stream, and the generation is raised under a row lock. `TestOldStreamCannotChangeRecord` and `TestNewStreamReplacesOld`. |
| A machine that stops reading holds a handler forever. | High | Every send has a deadline of 10 seconds. `TestBlockedMachineEndsStream`. |
| A server that died leaves its machines online. | Medium | Presence needs an unpassed lease, and a sweep deletes passed ones. `TestPresenceIgnoresPassedLease` and `TestRunSweeps`. |
| The handshake tells a stranger which identifiers exist. | Medium | The server signs before any lookup, and every refusal after the proof looks the same. `TestChallengeComesBeforeLookup`. |
| The web server in front cuts a quiet stream. | Medium | The client sends a heartbeat every 10 seconds, and the server answers each. `go-s2-stack-web` proves a silent stream of 6 minutes through the web server. |

**Verify:** `deploy/test/with-postgres.sh -- go test -race -count=1 ./src/server/internal/link/... ./src/server/internal/transport/... ./src/server/internal/api/...`. Environment check: `deploy/test/checks.sh --only C8,C9,C10` on the stack, and the orchestrator's tests in `src/linktest/` with the real client. No device is needed.

**Rollout detail:** rollout steps 1, 2, 4 and 5. The database change of the part `link` is applied at start-up after item 14 registers `link.Migrations()`.

**WS-3: Commands go down once, and reports come up once**

This workstream stores every command before it is sent and accepts only the answer to the newest attempt.
It applies every report once, in order, with its effect and its number in one transaction.
It implements the design's sections "Commands go down, and none is lost or repeated" and "Events come up, and none is lost or applied twice", rule R6, and check C11.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 9 | A command is stored before it is sent, each attempt is stored before it leaves, and only the newest attempt's answer counts. | `TestEnqueueStoresBeforeSend`, `TestClaimRaisesAttemptOncePerStream`, `TestClaimCommitsBeforeReturn` and `TestAcceptResultRules` pass. |
| 10 | A report is applied only as the next number, with its effect and its number together. | `TestApplyNextEvent`, `TestDropLowerNumber`, `TestGapEndsStream` and `TestEffectAndMarkCommitTogether` pass. |
| 11 | On the stack, reports kept during an outage arrive once and in order. | `deploy/test/checks.sh --only C11` prints one line that starts with `ok` and exits 0. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 6 | Write stored commands. | Target `src/server/internal/link/outbox.go`, `State.Enqueue`, `State.Claim` and `State.AcceptResult`, and `src/server/internal/link/migrations/0002_link_commands.sql`. Pattern: `src/server/internal/store/receipts.go`, `WriteOnce`. Scouting S2 and S5. |
| 7 | Write incoming events. | Target `src/server/internal/link/inbox.go`, `State.ApplyEvent`. Pattern: the same. Scouting S2 and S9. |

**Item 6. Stored commands**

- **Files:** creates `src/server/internal/link/outbox.go`, `src/server/internal/link/migrations/0002_link_commands.sql` and the test file `src/server/internal/link/outbox_test.go`. Changes `State.Forget` in `presence.go` to also delete the machine's commands.
- **Needs first:** item 5. Runs at the same time as items 4, 7 and 11.
- **Work area and complexity:** `backend`, medium. One table and four functions with row locks.
- **Scope:** `server/backend`.
- **Statements:** D16, R4, R6, R7, C7, and the design's section "Commands go down, and none is lost or repeated".
- **Risky:** yes, because it orders commands. Accepting a result whose attempt is older than the stored one must fail `TestAcceptResultRules`. Returning the claimed commands before the commit must fail `TestClaimCommitsBeforeReturn`.
- **The table**, in `0002_link_commands.sql`:

```sql
CREATE TABLE link_commands (
    id text PRIMARY KEY,
    machine_id text NOT NULL,
    action text NOT NULL CHECK (action IN ('scan_harnesses', 'set_state', 'restart')),
    payload bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    expires_at timestamptz NOT NULL,
    attempt integer NOT NULL DEFAULT 0 CHECK (attempt >= 0),
    sent_stream_id text,
    sent_at timestamptz,
    outcome text CHECK (outcome IN ('done', 'failed', 'expired', 'unsupported', 'interrupted')),
    message text NOT NULL DEFAULT '' CHECK (octet_length(message) <= 200),
    finished_at timestamptz
);
CREATE INDEX link_commands_pending ON link_commands (machine_id, created_at) WHERE outcome IS NULL;
```

- **What `Enqueue` does:** inside the caller's transaction, it takes the action of the given `Command` (only the oneof is read), makes the identifier with `store.NewID`, stores the action's name and `proto.Marshal` of the whole `Command` with the oneof alone, sets `expires_at = clock_timestamp() + ttl` and attempt 0, and records `machine_outbox` for the machine. A `Command` with no action returns an error.
- **What `Claim` does,** in one `Store.Write`: it locks the stream's connection row with `FOR UPDATE`, and no row gives `ErrNotCurrent`. For a waiting machine it takes only `set_state`. It locks, in order of `created_at`, every pending command whose `sent_stream_id` is not this stream, raises its attempt by one, sets `sent_stream_id` and `sent_at`, and records `machine_outbox`. After the commit it returns one `Command` for each, with the identifier, the new attempt, the stored `expires_at` and the stored action. A command past its expiry is still claimed, because the machine may hold a recorded result for it.
- **What `AcceptResult` does:** it refuses a non-canonical identifier, attempt 0, a message over 200 bytes or invalid UTF-8 with `ErrInvalidReport`. In one write it locks the stream's connection row, and no row gives `false`. It locks the command of this machine. It returns `false` and writes nothing when the command is missing, already has an outcome, or has an attempt other than the result's. Otherwise it stores the outcome, the message and `finished_at`, records `machine_outbox`, and returns `true`. `COMMAND_OUTCOME_UNSPECIFIED` and a value it does not know are stored as `failed`.
- **What `SweepCommands` does:** it sets `expired` on every command without an outcome whose `expires_at` is more than `CommandGrace` past, and deletes every command whose `finished_at` is more than `CommandRetention` past.
- **A test seam:** the private `afterClaim func(context.Context, *store.Tx) error`, nil in production, runs after the claim's updates and before the commit.
- **Tests written first,** in `outbox_test.go`, package `link`:

| Test | What it asserts |
|---|---|
| `TestEnqueueStoresBeforeSend` | `Enqueue` of a `ScanHarnesses` with 2 minutes stores a row with attempt 0, `expires_at` 2 minutes after the database's clock, and one `machine_outbox` feed row keyed by the machine. When the caller's write returns an error, no row and no feed row exist. |
| `TestClaimRaisesAttemptOncePerStream` | `Claim` on stream A returns the command with attempt 1 and the stored expiry, and the row holds attempt 1 and stream A. A second `Claim` on A returns nothing. After stream B became current, `Claim` on B returns the same identifier with attempt 2 and the same expiry. |
| `TestClaimCommitsBeforeReturn` | With `afterClaim` returning an error, `Claim` returns that error and no command, and the row keeps attempt 0. |
| `TestClaimForWaitingMachine` | For a waiting machine with a pending `ScanHarnesses` and a pending `SetState`, `Claim` returns only the `SetState`. |
| `TestClaimNotCurrent` | `Claim` on a stream that is no longer current returns `ErrNotCurrent` and changes no attempt. |
| `TestAcceptResultRules` | For a command sent as attempt 2 on the current stream: a result for attempt 2 returns `true` and stores `done`; a copy of it returns `false`; on a fresh command, results for attempts 1 and 3 return `false` and store nothing; a result on an old stream returns `false`; a result for an unknown identifier returns `false`; `COMMAND_OUTCOME_UNSPECIFIED` and the value 99 store `failed`; a message of 201 bytes returns `ErrInvalidReport`. |
| `TestFinalCommandNotClaimedAgain` | After a stored outcome, `Claim` on a new stream returns nothing. |
| `TestSweepCommands` | A command without an outcome 25 hours past its expiry becomes `expired`. One 23 hours past stays pending. A command finished 8 days ago is deleted, and one finished 6 days ago is kept. |

- **Acceptance:** `go test -count=1 -run 'TestEnqueue|TestClaim|TestAcceptResult|TestFinalCommand|TestSweepCommands' ./src/server/internal/link/` passes.

**Item 7. Incoming events**

- **Files:** creates `src/server/internal/link/inbox.go` and the test file `src/server/internal/link/inbox_test.go`.
- **Needs first:** item 5. Runs at the same time as items 4, 6 and 11.
- **Work area and complexity:** `backend`, medium. One transaction with a fixed order of checks, and value rules from the contract.
- **Scope:** `server/backend`.
- **Statements:** R3, R4, R7, R9, C7, C11, and the design's section "Events come up, and none is lost or applied twice".
- **Risky:** yes, because it orders events. Moving the mark's update into a separate transaction must fail `TestEffectAndMarkCommitTogether`. Applying an event above the mark plus one must fail `TestGapEndsStream`.
- **What `ApplyEvent` does,** in one `Store.Write`, as the orchestrator's note of 2026-11-26 on round 2 of the contract fixes:
  1. Lock the stream's connection row with `FOR UPDATE`. No row gives `ErrNotCurrent`. This lock puts the events of one machine in a line.
  2. Lock the machine's `link_machines` row and read the mark.
  3. A number at or below the mark returns `EventResult{Mark: mark, Applied: false}` and writes nothing.
  4. A number above the mark plus one returns `ErrEventGap` and writes nothing.
  5. A report that breaks a value rule returns `ErrInvalidReport` and writes nothing. For `Facts`: `os` and `client_version` at most 64 bytes, `workspace` at most 256 bytes, all valid UTF-8, and `cores` at least 1. For `HarnessesFound`: at most 16 entries, each name matching `^[a-z][a-z0-9-]{0,31}$` and appearing once, and each version, when present, valid UTF-8 of 1 to 64 bytes.
  6. The private `policy(approval, report)` decides the effect. For an approved machine, `Facts` calls `Registry.ApplyFacts` and sets `facts_received`, and `HarnessesFound` calls `Registry.ApplyHarnesses` and sets `harnesses_received`. For a waiting machine, every report is consumed without an effect (decision 11). An event with no report, or with a report this server does not know, is consumed.
  7. Set the mark to the number, and call `Registry.Touch`.

  After the commit, it logs `link_event_applied` with `machine_id` and `sequence` equal to the number, and returns `EventResult{Mark: number, Applied: true}`.
- **Tests written first,** in `inbox_test.go`, package `link`:

| Test | What it asserts |
|---|---|
| `TestApplyNextEvent` | With mark 0, event 1 with `Facts` returns `Applied` true and mark 1. The record holds the facts, the connection row has `facts_received` true, and the log has one `link_event_applied` with the machine's identifier and sequence 1. |
| `TestDropLowerNumber` | Event 1 again returns `Applied` false and mark 1, writes no row, and logs nothing. |
| `TestGapEndsStream` | With mark 1, event 3 returns `ErrEventGap`, and the mark and the record are unchanged. |
| `TestEffectAndMarkCommitTogether` | A trigger of the test raises an error on the update of `machines`. Event 2 returns an error, and the mark stays 1. After the trigger is dropped, event 2 is applied. |
| `TestUnknownReportConsumed` | An event with no report moves the mark and changes nothing else. |
| `TestInvalidReport` | An `os` of 65 bytes, `cores` 0, 17 harnesses and a name given twice each return `ErrInvalidReport`, and the mark stays. |
| `TestWaitingMachineEventsConsumed` | For a waiting machine, an event with `Facts` moves the mark, and the record holds no facts. |
| `TestOldStreamEventIgnored` | An event on a stream that is no longer current returns `ErrNotCurrent`, and the mark stays. |
| `TestHundredEventsInOrder` | Events 1 to 100, sent one after another, give mark 100 and 100 log lines with sequences 1 to 100 in order. |

- **Acceptance:** `go test -count=1 -run 'TestApplyNextEvent|TestDropLower|TestGapEnds|TestEffectAndMark|TestUnknownReport|TestInvalidReport|TestWaitingMachineEvents|TestOldStreamEvent|TestHundredEvents' ./src/server/internal/link/` passes.

**Design artifact:** not applicable. No screen changes.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| A confirmation deletes a report on the machine that the server never saved. | High | `EventAck` is sent only after `ApplyEvent` returns from its commit. `TestEventsAckedAfterCommit` in item 10 fails the commit and sees no confirmation. |
| A command runs twice, or an old answer closes a newer attempt. | High | Each attempt is stored before it is sent, and only the stored attempt's answer counts. `TestAcceptResultRules`. |
| A report from a machine fills the database. | Low | The value rules bound every string and list. `TestInvalidReport`. |

**Verify:** `deploy/test/with-postgres.sh -- go test -race -count=1 ./src/server/internal/link/...`. Environment check: `deploy/test/checks.sh --only C11` on the stack. No device is needed.

**Rollout detail:** rollout step 3. The command table is the second database change of the part `link`.

**WS-4: The Machines page works against the Go server**

This workstream answers the eight calls of `ClientService` that the Machines page already makes.
The page needs no change, except the one the owner's point 1 of the contract plan may bring.
It implements the design's section "Serving the contract", rules R2, R5, R6 and R8, and checks C5 to C7.

**Done when:**

| # | Criterion | Observed by (test id / command / on-device step) |
|---|---|---|
| 12 | The page's record shows each standing, with load only while online, and the live list sends a position on every message. | `TestProjectClient`, `TestWatchClientsFirstMessageAndChanges` and `TestWatchClientsPositionAlwaysPresent` pass. |
| 13 | The live list ends when the session ends or the reader stops. | `TestWatchClientsNeedsLiveSession` and `TestWatchClientsBlockedReaderEnds` pass. |
| 14 | Every action keeps its rules, and an approval and its command are stored together. | `TestCreateClientToken`, `TestRevokeClientToken`, `TestRemoveClient`, `TestApproveClient`, `TestApproveRollsBackWithItsCommand`, `TestScanClientHarnesses` and `TestReconnectClient` pass. |
| 15 | The server program serves both services and keeps machines across a restart. | `TestServerServesMachines` and `TestRestartKeepsMachines` pass. |
| 16 | On the stack, an approved machine shows online with its facts, its load and its four harnesses. | `deploy/test/checks.sh --only C7` prints one line that starts with `ok` and exits 0. A person makes a token on the Machines page, pastes its command on a machine, approves it, and sees it online without a reload. |

**Tasks:**

| # | Task | Anchor (file + symbol; line optional) |
|---|---|---|
| 11 | Write the projection and `WatchClients`. | Targets `src/server/internal/machines/project.go`, `Project`, `src/server/internal/machines/watch.go`, `Service.WatchClients`, and `src/server/internal/machines/service.go`, `NewService`. Pattern: `src/mock-server/src/engine.ts`, `watchClients`, and `src/server/internal/watch/watch.go`, `Serve`. Scouting S3 and S5. |
| 12 | Write the calls that touch only records. | `src/server/internal/machines/service.go`, `Service.CreateClientToken`, `Service.RevokeClientToken`, `Service.RemoveClient`. Pattern: `src/mock-server/src/engine.ts`, `revokeClientToken`. Scouting S4. |
| 13 | Write the calls that send a command. | Target `src/server/internal/machines/commands.go`, `Service.ApproveClient`, `Service.RenameClient`, `Service.ScanClientHarnesses`, `Service.ReconnectClient`. Pattern: `src/mock-server/src/engine.ts`, `approveClient`. Scouting S4. |
| 14 | Wire both parts into the server program and its test entry point. | `src/server/internal/app/start.go`, `Start`, `src/server/internal/config/config.go`, `Load`, and `src/server/servertest/server.go`, `Start`, of `go-s2-server-signin`. Scouting S6. |

**Item 11. The record of the page, and `WatchClients`**

- **Files:** creates `src/server/internal/machines/project.go`, `src/server/internal/machines/watch.go`, `src/server/internal/machines/service.go` (the type `Service`, `ServiceOptions` and `NewService`), and the test files `src/server/internal/machines/project_test.go` and `src/server/internal/machines/watch_test.go`.
- **Needs first:** items 1, 2 and 5. Runs at the same time as items 4, 6 and 7.
- **Work area and complexity:** `backend`, medium. One pure mapping and one adapter of the store's `Watch` helper.
- **Scope:** `server/backend`.
- **Statements:** D5, R2, R5, R7, R8, C6, C7, C8, C9, and the design's section "How every `Watch` call works".
- **Risky:** yes, because it orders changes and checks the session. Leaving `position` out of a `saved` message must fail `TestWatchClientsFirstMessageAndChanges`. An `Authorize` that returns nil must fail `TestWatchClientsNeedsLiveSession`.
- **What `Project` does:** it maps a record and its presence, or nil, to `lgtmv1.Client`:
  - `state` is `CLIENT_STATE_WAITING_FOR_APPROVAL` while waiting, whether connected or not. For an approved machine it is `CLIENT_STATE_OFFLINE` without presence, `CLIENT_STATE_ONLINE` when `FactsReceived` and `HarnessesReceived` are both true, and `CLIENT_STATE_JOINING` otherwise.
  - `name` is the host name while waiting and the given name after approval. `token_id` is set only while waiting.
  - `address` and `last_seen` come from the presence when there is one, and from `LastAddress` and `LastSeenAt` otherwise. `connected_at` is set only with presence.
  - `cpu_percent` and `memory_percent` are set only while online, from the presence.
  - The facts fill `os`, `version`, `workspace`, `agent_limit`, `cores` and `memory_gigabytes`, which is `memory_bytes` divided by 2^30 and rounded to the nearest whole number.
  - `harnesses_scanned` and `harnesses` come from the record, and `hosts_server` from the token.
- **What `WatchClients` does:** it reads the session's verifier with `api.SessionVerifier`, and calls `watch.Serve` with a `watch.Source` of topic `machines` and an empty scope. `Authorize` calls `auth.Sessions.CheckInRead`, and `auth.ErrNoSession` becomes `watch.ErrWatchUnauthorized`. `Snapshot` calls `Registry.List` and `Link.Presence` and projects each. `Load` calls `Registry.Get` and `Link.Presence`, and `ErrNoMachine` means the record is gone. Each message becomes a `WatchClientsResponse` with `all`, `saved` or `removed_id`, and `position` set to the message's position on every message, also 0. Each send goes through `transport.Send`. `watch.ErrWatchUnauthorized` gives `unauthenticated` with "Sign in again to reopen this watch.", `store.ErrWatchLagged` gives `aborted` with "The watch fell behind and must be reopened.", and `store.ErrFeedUnavailable` gives `unavailable` with "The change feed is unavailable, try again."
- **Tests written first,** in `project_test.go` and `watch_test.go`:

| Test | What it asserts |
|---|---|
| `TestProjectClient` | A table of cases: a waiting machine gives the waiting state, its host name, its token's identifier and no load, also with presence. An approved machine without presence gives `CLIENT_STATE_OFFLINE`, `last_seen` and `address` from the record, and no `connected_at` and no load. With presence and only facts received it gives `CLIENT_STATE_JOINING` and no load. With both received it gives `CLIENT_STATE_ONLINE` with the presence's load, `connected_at` and address. 1,610,612,736 bytes of memory give 2 gigabytes, and 429,496,729 bytes give 0. A harness without a version has no version. `hosts_server` follows the record. |
| `TestWatchClientsFirstMessageAndChanges` | With a session cookie, the first message is `all` with every machine and a `position`. A machine created afterwards arrives as `saved` with a higher position. A deleted machine arrives as `removed_id` with a higher position still. |
| `TestWatchClientsPositionAlwaysPresent` | On an empty database the first message is `all` with no machine and a present `position` of 0. |
| `TestWatchClientsNeedsLiveSession` | Without a cookie the call gets `unauthenticated`. With a cookie, after the first message the test revokes the session, and the stream ends with `unauthenticated` and the sentence "Sign in again to reopen this watch." within 35 seconds, with no later `saved` message. |
| `TestWatchClientsLagIsAborted` | When a private seam makes the follower report `store.ErrWatchLagged`, the stream ends with `aborted`. |
| `TestWatchClientsBlockedReaderEnds` | With `transport`'s timeout at 200 milliseconds, a reader that stops reading while the test writes 500 changes sees the stream end within 3 seconds, and the handler returns. |

- **Acceptance:** `go test -count=1 -run 'TestProjectClient|TestWatchClients' ./src/server/internal/machines/` passes.

**Item 12. The calls that touch only records**

- **Files:** changes `src/server/internal/machines/service.go`. Creates the test file `src/server/internal/machines/service_test.go`.
- **Needs first:** items 3, 5 and 11. Runs at the same time as items 8, 9 and 13.
- **Work area and complexity:** `backend`, medium. Three handlers over tested functions, with request keys.
- **Scope:** `server/backend`.
- **Statements:** D3, R4, R8, R9, C5, and the contract's comments of `CreateClientToken`, `RevokeClientToken` and `RemoveClient`.
- **Risky:** yes, because it hands out tokens. Accepting a request key on `CreateClientToken` must fail `TestCreateClientTokenRefusesRequestKey`. A `RevokeClientToken` that leaves a waiting machine must fail `TestRevokeClientToken`.
- **What each call does:**
  - `CreateClientToken` refuses a present `Lgtm-Request-Key`, calls `Tokens.Create` with no address and no mark, and answers `token_id`, `token` and `expires_at`. `ErrNoCertificatePin` gives `failed_precondition`.
  - `RevokeClientToken` runs in `Store.WriteOnce` with the caller `owner`, the procedure `/lgtm.v1.ClientService/RevokeClientToken`, `ReceiptSafe` true and a version 1 fingerprint of the token identifier. It calls `Tokens.Revoke`, and when that returns a waiting machine, it calls `Link.Forget` and `Registry.Delete` in the same write.
  - `RemoveClient` runs in `Store.WriteOnce` the same way, with a fingerprint of the machine identifier. It calls `Link.Forget` and `Registry.Delete`, and `ErrNoMachine` gives `not_found`. The hub of item 9 ends a connected stream when the removal reaches the feed.
  - A version 1 fingerprint is the field's bytes framed by an eight-byte big-endian length, as the store plan's receipts describe.
- **Tests written first,** in `service_test.go`, package `machines_test`, with the real `link.State`:

| Test | What it asserts |
|---|---|
| `TestCreateClientToken` | The answer's `token` decodes with `DecodeToken`, its `token_id` equals the answer's, its `expires_at` is 15 minutes after the call within 2 seconds, and a machine that joins with it has `hosts_server` false. |
| `TestCreateClientTokenRefusesRequestKey` | With `Lgtm-Request-Key` present, the call answers `invalid_argument` with "This call does not accept a request key.", and no token row exists. |
| `TestCreateClientTokenNeedsPin` | With an `https` public address and no certificate file, the call answers `failed_precondition`. |
| `TestRevokeClientToken` | After revoking an unused token, `Redeem` with it gives `token_revoked`. Revoking a token used by a waiting machine deletes that machine and its link rows, and an open `WatchClients` receives its `removed_id`. Revoking a token used by an approved machine leaves the machine. Revoking an unknown identifier answers success. |
| `TestRemoveClient` | Removing a connected machine deletes its record and its link rows, and an open `WatchClients` receives `removed_id`. An unknown identifier answers `not_found`. The same request key sent twice answers success twice and removes once. The machine that runs the server can be removed. |

- **Acceptance:** `go test -count=1 -run 'TestCreateClientToken|TestRevokeClientToken|TestRemoveClient' ./src/server/internal/machines/` passes.

**Item 13. The calls that send a command**

- **Files:** creates `src/server/internal/machines/commands.go` and the test file `src/server/internal/machines/commands_test.go`.
- **Needs first:** items 6 and 11. Runs at the same time as items 8, 9 and 12.
- **Work area and complexity:** `backend`, medium. Four handlers on one pattern: check, change the record, store a command, in one write.
- **Scope:** `server/backend`.
- **Statements:** D16, R4, R6, R8, C7, and the contract's comments of `ApproveClient`, `RenameClient`, `ScanClientHarnesses` and `ReconnectClient`.
- **Risky:** yes, because approval decides who may work. Storing the `SetState` command in a second write after the approval must fail `TestApproveRollsBackWithItsCommand`. Accepting a scan for a machine that is not connected must fail `TestScanClientHarnesses`.
- **What each call does,** each in `Store.WriteOnce` with the caller `owner`, its own procedure, `ReceiptSafe` true and a version 1 fingerprint of its fields in the order of the request message:
  - `ApproveClient` normalizes the name, calls `Registry.SetApproved`, and calls `Link.Enqueue` with `SetState{state: Standing(rec)}` for 1 hour. It answers the projected client, read after the commit. `ErrNotWaiting` gives `failed_precondition`, `ErrNameTaken` gives `already_exists`, `ErrInvalidName` gives `invalid_argument` and `ErrNoMachine` gives `not_found`.
  - `RenameClient` refuses a waiting machine with `failed_precondition`. It calls `Registry.SetName`, and when the name changed, `Link.Enqueue` with the new `SetState` for 1 hour.
  - `ScanClientHarnesses` requires an approved machine with presence, read in the same write, and otherwise answers `failed_precondition`. It calls `Link.Enqueue` with `ScanHarnesses` for 2 minutes.
  - `ReconnectClient` requires the same, and calls `Link.Enqueue` with `Restart` for 2 minutes.
- **Tests written first,** in `commands_test.go`, package `machines_test`:

| Test | What it asserts |
|---|---|
| `TestApproveClient` | Approving a waiting machine as `Box` answers a client with that name, revision 2 in the record, and one pending `set_state` command whose stored state is `APPROVAL_APPROVED`, `Box` and revision 2. |
| `TestApproveRollsBackWithItsCommand` | With a `Link` whose `Enqueue` returns an error, the call fails, and the machine still waits with revision 1. |
| `TestApproveRefusals` | An approved machine answers `failed_precondition`, an unknown identifier `not_found`, an empty name `invalid_argument`, and a name taken in other letter case `already_exists`. |
| `TestRenameClient` | Renaming an approved machine raises its revision and stores one `set_state`. The same name stores nothing. A waiting machine answers `failed_precondition`. A taken name answers `already_exists`. |
| `TestScanClientHarnesses` | A connected approved machine gets one `scan_harnesses` command that expires 2 minutes after the database's clock. A machine with no presence, and a connected waiting machine, answer `failed_precondition` with "This machine is not connected." and store nothing. |
| `TestReconnectClient` | A connected approved machine gets one `restart` command. A machine with no presence answers `failed_precondition`. |
| `TestKeyedCallsReplay` | `ScanClientHarnesses` with one request key sent twice stores one command and answers success twice. The same key with another machine identifier answers `already_exists`. |

- **Acceptance:** `go test -count=1 -run 'TestApprove|TestRenameClient|TestScanClientHarnesses|TestReconnectClient|TestKeyedCalls' ./src/server/internal/machines/` passes.

**Item 14. Wiring into the server program and its test entry point**

The server program of sprint 2 starts with these two parts in it, and the test entry point lets the orchestrator's tests drive them.

- **Files:** changes `src/server/internal/app/start.go`, `src/server/internal/config/config.go`, `src/server/internal/config/config_test.go`, `src/server/servertest/server.go` and `src/server/servertest/server_test.go`. Creates the test file `src/server/internal/app/machines_test.go`.
- **Needs first:** items 4, 10, 11, 12 and 13.
- **Work area and complexity:** `backend`, medium. Composition in one function of a known pattern, one setting, and four test helpers.
- **Scope:** `server/backend`.
- **Statements:** D1, D3, D7, R1, R7, R10, C5 to C11, and issue 1634.
- **Risky:** yes, because it places the machines' calls behind the session guard and the link's calls outside it. Passing the `ClientService` handler as a link handler, outside the guard, must fail `TestServerServesMachines`.
- **What changes:**
  - `config.Config` gains `TLSCertFile`, read from `LGTM_TLS_CERT_FILE`. `Load` refuses an `https` public address without it, and names the setting.
  - `app.Start` adds `machines.Migrations()` and `link.Migrations()` to `Store.Migrate`. It builds `machines.NewRegistry()`, `link.NewState`, `machines.NewTokens` with `Issuer{PublicURL, ServerKey, CertificateFile}`, `link.NewHub()`, `machines.NewService` and `link.NewService`. It passes them as `api.Options.Clients` and `api.Options.Link`, with `link.HandlerOptions()`. It starts `link.Service.Run` with the process's context, and registers its cancellation with `http.Server.RegisterOnShutdown`, so open streams end before the five seconds of draining.
  - `servertest.Options` gains `Log io.Writer`. `servertest.Server` gains `Clients(t testing.TB) lgtmv1connect.ClientServiceClient`, signed in through the fixture's cookie jar, `Link() linkv1connect.LinkServiceClient`, `ExpireJoinToken(t testing.TB, tokenID string)`, which sets that token's `expires_at` one second into the past, and `Settings() func(string) string`, which returns the settings of the started server for a second program, as `go-s3-stack-client` asked.
- **Tests written first,** in `app/machines_test.go` and `servertest/server_test.go`:

| Test | What it asserts |
|---|---|
| `TestServerServesMachines` | Through `servertest.Start`: `Clients(t).CreateClientToken` answers a token. A `Join` through `Link()` with no cookie answers a machine. `WatchClients` with a fresh client without a cookie answers `unauthenticated`. `Open` without a cookie answers a `Challenge` to a `Hello`. |
| `TestConfigReadsCertificateFile` | In `config_test.go`. `LGTM_TLS_CERT_FILE` fills `TLSCertFile`. An `https` public address without it fails `Load` with a message that names `LGTM_TLS_CERT_FILE`. |
| `TestRestartKeepsMachines` | A machine joins and is approved. After `Restart`, `WatchClients` lists it approved, and `SigningPublicKey` is unchanged. |
| `TestShutdownEndsLinkStreams` | A scripted machine is connected. `Stop` returns within 5 seconds, the stream ends with `unavailable`, and after `Restart` the machine shows `CLIENT_STATE_OFFLINE`. |
| `TestStartMarksNoMachineOnlineByMemory` | A machine is connected, and the test stops the server and sets the machine's lease 40 seconds into the past through the database. After `Restart`, the first `WatchClients` message shows it offline. |
| `TestExpireJoinTokenHelper` | After `ExpireJoinToken`, a `Join` with that token answers `permission_denied` with `Lgtm-Join-Refusal: token_expired`. |

- **Acceptance:** `go test -count=1 -run 'TestServerServesMachines|TestRestartKeepsMachines|TestShutdownEndsLinkStreams|TestStartMarksNoMachineOnlineByMemory' ./src/server/internal/app/`, `go test -count=1 -run TestConfigReadsCertificateFile ./src/server/internal/config/` and `go test -count=1 -run TestExpireJoinTokenHelper ./src/server/servertest/` pass.

**Design artifact:** not applicable. No screen changes. The page's records and actions exist today, and the sample server shows how they behave.

**Risks:**

| Risk | Severity | Mitigation |
|---|---|---|
| The live list keeps sending after the owner's session ended. | High | Every change is authorized again, and a quiet stream is checked every 30 seconds by the store's helper. `TestWatchClientsNeedsLiveSession`. |
| An approval is stored without its command, so a connected machine keeps working as waiting until it reconnects. | Medium | Both are written in one transaction. `TestApproveRollsBackWithItsCommand`. |
| A retried page action runs twice. | Medium | Each action takes a request key into the store's receipts, and the token call refuses one. `TestKeyedCallsReplay` and `TestRemoveClient`. |
| The page offers "Try to reconnect" for an offline machine, and the Go server refuses it. | Medium | The refusal is one sentence the page shows. The owner's point 1 of the contract plan decides how the page follows. |
| A sprint 2 file changes in a way that breaks its own tests. | Medium | Item 14 runs every test of `src/server/` before it merges. |

**Verify:** `deploy/test/with-postgres.sh -- go test -race -count=1 ./src/server/...`. Environment check: `deploy/test/checks.sh --only C5,C6,C7,C8,C9,C10,C11` on the stack with the client and the installer of the other two plans, and the step a person performs in criterion 16. No device is needed beyond the machine that runs the client.

**Rollout detail:** rollout steps 6 and 7. The stack runs `deploy/up.sh`, exports `LGTM_WEB_PASSWORD` and `LGTM_TEST_POSTGRES`, and runs `deploy/test/checks.sh`.

</details>

<details><summary><b>Requests to other plans</b></summary>

Each request names the plan that owns the name. The result file repeats them.

1. **To `go-s0-contract`: the refusal header of `Join`.** The comment of `Join` names the response header `Lgtm-Join-Refusal` and its values `token_unknown`, `token_expired`, `token_revoked`, `token_used` and `machine_removed`. Check C5 needs the client to tell a used token from an expired one, and every refusal shares one code.
2. **To `go-s0-contract`: the token of a waiting machine.** The comment of `Welcome.connection_token` says that it is empty while the machine waits for approval.
3. **To `go-s0-contract`: the code of a removal.** The comment of `Open`, section "Removal", says that the server ends the current stream with `unauthenticated`, and may skip the `SetState` with `APPROVAL_REMOVED`.
4. **To `go-s0-contract`: `CreateClientToken` refuses a request key**, rather than ignoring it, and its comment names `lgtm-server join-token` instead of `lgtm client add`.
5. **To `go-s0-contract`, for the client plan's proposals:** this plan fills `JoinToken.expires_at` from the token row if the contract adds it, and reads no `Hello.outbox_epoch`. A field the server does not know is ignored.
6. **To `go-s2-server-signin`: four files change in sprint 3.** Item 2 wraps two route families in `api/server.go`. Item 14 adds `Config.TLSCertFile` from `LGTM_TLS_CERT_FILE`, composes both parts in `app.Start`, and adds `Options.Log`, `Clients`, `Link`, `ExpireJoinToken` and `Settings` to `servertest`. The sign-in plan should keep `app.Start` as one composition point and accept these changes.
7. **To `go-s2-server-signin`: the session for a live list.** `auth.Sessions.CheckInRead`, `auth.ErrNoSession`, `api.SessionVerifier` and the principal `owner` stay as they are written on 2026-11-26.
8. **To `go-s2-stack-web`: the public address in loopback mode.** A join token carries `LGTM_PUBLIC_URL`, so in loopback mode the server must receive the real published port, such as `http://127.0.0.1:8899` when `LGTM_WEB_PORT` is 8899. Today the compose file passes an empty value and the sign-in plan's default is port 8787.
9. **To `go-s2-stack-web`: the forwarded address.** The web server keeps its default of setting `X-Forwarded-For` to the address of its own peer, so the page shows where a machine connected from.
10. **To `go-s3-client-link`: after approval on an open stream**, the client sends its current `Facts` and `HarnessesFound` as new events, because the server sets aside any report it receives while the machine waits. The plan of 2026-11-26 already does this.
11. **To `go-s3-client-link`: a removal on the current stream.** An `unauthenticated` end of a stream whose `Challenge` verified stops the client with `link_refused` and `unknown_machine`, also when it comes after `Welcome`.
12. **To `go-s3-stack-client`: the token function.** The command calls `machines.NewTokens(store, registry, machines.Issuer{...})` and `Tokens.Create(ctx, machines.TokenOptions{Address, HostsServer: true})`. `ErrAddressMismatch` and `ErrNoCertificatePin` give exit 1 with one sentence. The table is `client_join_tokens` with `id` and `expires_at`, in the schema of the application's connection.
13. **To the orchestrator, for `src/linktest/`:** tests with the real client against `servertest`: join and the waiting record; approval and online with facts, load and four harnesses; stop and offline within 30 seconds; server restart and a new `connected_at`; removal and the client's stop; a changed server key and the client's refusal; events kept during an outage, applied once and in order; a scan command answered once across a reconnection; and two client processes with one key, where the newer stream wins.
14. **To the Client team, for `src/client/clienttest`:** a way to join with a token text into a state folder, to run a client in its own goroutine, to stop it, to pause it and to resume it, and to read its log lines.

</details>

<details><summary><b>Traceability (gate 4)</b></summary>

Source universe:

1. The brief's work items 1 to 9, its four observations and its two agreements, in `plan.md` of 2026-11-25.
2. The brief's points to settle 1 to 4.
3. The task file `.foundry/go-rewrite/tasks/plan-go-s3-server-machines.md`, section "Specific to this plan", and its demand to keep the names of `deploy/test/README.md`.
4. The issues 1633, 1636, 1637, 1638, 1639 and 1640 in `docs/issues.md`.
5. The orchestrator's note of 2026-11-26 on round 2 of the contract.
6. The design's statements that bear on this plan: D3, D5, D7, D16, D17, R1 to R10, C5 to C11.
7. `decisions.md`, rows 1 to 38.

| Source item | Workstream | Status |
|---|---|---|
| Brief item 1, machine records | WS-1, item 1 | covered |
| Brief item 2, join tokens | WS-1, items 3 and 4 | covered, split in two |
| Brief item 3, the page's calls that touch records | WS-4, items 11, 12 and 13 | covered; approve and rename move to item 13, because they store a command |
| Brief item 4, `Join` | WS-1, item 4 | covered |
| Brief item 5, the proof and the replacement of streams | WS-2, items 5, 8 and 9 | covered, split in three |
| Brief item 6, stored commands | WS-3, item 6, and item 10 for the wire | covered, split |
| Brief item 7, incoming events | WS-3, item 7, and item 10 for the wire | covered, split |
| Brief item 8, presence | WS-2, items 5 and 10 | covered |
| Brief item 9, scan and reconnect | WS-4, item 13 | covered |
| Brief observation, the page works with no change of a screen | WS-4, criterion 16 | covered |
| Brief observation, a used or expired token is refused | WS-1, criterion 4 | covered |
| Brief observation, an unknown key is refused | WS-2, criterion 8 | covered |
| Brief observation, a restart brings a machine back | WS-2, criterion 8; item 14, `TestStartMarksNoMachineOnlineByMemory` | covered |
| Brief agreement, every message of the link with `go-s3-client-link` | Requests 10 and 11; public names | covered |
| Brief agreement, the test entry point | Item 14; requests 13 and 14 | covered |
| Point 1, issue 1633 | Decision 10; item 3 | settled with an assumption |
| Point 2, what a waiting machine may do | Decision 11; items 7, 8 and 6 | settled with an assumption |
| Point 3, replacing the signing key | Decision 17 | settled for wave 1; a hand-over waits |
| Point 4, `link/service.go` cut into files | Items 4, 5, 8, 9 and 10 | settled: `service.go` keeps the type and its constructor, and `join.go`, `handshake.go`, `presence.go`, `hub.go`, `stream.go` and `run.go` hold the rest |
| Task: split high items | Decision 8; execution sizing | covered |
| Task: issues 1633, 1637, 1638, 1639 | Decisions 10, 16, 15 and 17 | settled with assumptions |
| Task: what `src/linktest/` and `clienttest` need | Requests 13 and 14 | covered |
| Task: names of `deploy/test/README.md` | Result file, section on the script | kept: `client_join_tokens`, `id`, `expires_at`, `link_event_applied`, `machine_id`, `sequence`, the website's calls; C5's client codes need request 1 |
| Issue 1636, the join command's spelling | Outside this plan | the page's text belongs to the orchestrator and the owner |
| Issue 1640, a sandbox without PostgreSQL | Verify lines | every test needs `LGTM_TEST_POSTGRES`; builders run with network access, as the run log records |
| The orchestrator's note of 2026-11-26 | Decision 20; item 7 | covered |
| D3, D5 | Items 1, 5, 11 | covered |
| D7 | Items 1, 3, 4, 5, 8 | covered |
| D16 | Items 6, 13 | covered |
| D17 | Items 3, 8; decision 10 | covered |
| R1 | Items 1, 3, 5, 6 | covered |
| R2 | Items 11 to 13 build against the frozen contract | covered |
| R3 | Decision 9; item 1 | covered |
| R4 | Items 1, 3 to 7, 12, 13 | covered |
| R5 | Item 11 | covered |
| R6 | Items 6, 13; decision 25 | covered |
| R7 | Items 5, 9; decision 18 | covered |
| R8 | The error tables; items 4, 8, 11 to 13 | covered |
| R9 | Items 3, 4; log lines | covered |
| R10 | Every item; requests 13 and 14 | covered |
| C5 | Items 3, 4, 12; criterion 4 | covered |
| C6 | Items 4, 11; criterion 4 | covered |
| C7 | Items 7, 10, 11, 13; criterion 16 | covered |
| C8 | Items 5, 10; criterion 8 | covered |
| C9 | Items 5, 9, 10, 14; criterion 8 | covered |
| C10 | Items 4, 8; criterion 8 | covered |
| C11 | Items 7, 10; criterion 11 | covered |
| `decisions.md` rows 1 to 38 | Key decisions and the items | every row closed or unanswered with its assumption |

</details>

<details><summary><b>Guardrails triggered (gate 9)</b></summary>

| Repo rule (CLAUDE.md etc.) | How this plan satisfies it |
|---|---|
| `AGENTS.md`: the three parts meet only through the contract in `src/contracts/`. | The server imports only the generated packages `lgtmv1` and `linkv1`. Requests 1 to 5 go to the Contract team instead of changing a message here. |
| `AGENTS.md`: new code does not go into `packages/`, `design/mock/` or `plugins/`. | Every file is under `src/server/`. The sample server is read as a pattern. |
| `AGENTS.md`, habit 1: credit copied code. | No code is copied. The patterns are named in the scouting notes. |
| `AGENTS.md`, habit 2: write down what you find. | This plan may not edit `docs/`. The result file lists the problems for the orchestrator. |
| `AGENTS.md`, habit 3: finished work reaches the base branch through a pull request. | The orchestrator merges the sprint. |
| `AGENTS.md`, writing style, and `scripts/check-private.mjs`. | The plan uses full sentences, expands acronyms, quotes no person, and names only loopback and documentation addresses. |
| The epic: a team edits only its own folders. | Every file is in `src/server/`. Four files of sprint 2 change, all in the Server team's folder, as decision 28 records. |
| The design, rule 2: a contract file is reviewed before its code. | The plan builds against the contract of sprint 0 and asks for changes as requests. |
| The design, rule 10: tests run against a real PostgreSQL. | Every database test uses `migrated.NewStore` with `LGTM_TEST_POSTGRES`. |
| The planning rules of this epic: name no model and no harness. | The plan names work areas and complexities only. |

</details>

<details><summary><b>Execution sizing (gate 10)</b></summary>

Models come from the harness-sessions plugin's model sheet (its harness-session skill) at launch; this section names none (owner, 2026-10-16).
The scope `server/backend` exists in `.plans/scopes/server/backend/`. The orchestrator writes and checks the scope files.

| Unit | Work area | Scope | Complexity | Why this complexity | Attack pass | Depends on |
|---|---|---|---|---|---|---|
| 1. Machine records | `backend` | `server/backend` | medium | One table and its functions on the pattern of the sessions table. | no | sprint 1 store, sprint 0 contract |
| 2. Write deadlines | `backend` | `server/backend` | low | One middleware of the standard library and a one-line router change. | no | sprint 2 router |
| 3. Tokens: make and revoke | `backend` | `server/backend` | medium | One table, an encoding the contract fixes, and one certificate read. | no | 1 |
| 4. Joining | `backend` | `server/backend` | medium | One transaction with a fixed order of checks and one thin handler. | yes, a machine reaches `Join` without a session | 3, 5 |
| 5. Link tables and presence | `backend` | `server/backend` | medium | Row locks inside one write per operation, on the receipts pattern. | no | 1 |
| 6. Stored commands | `backend` | `server/backend` | medium | One table and four functions with row locks. | no | 5 |
| 7. Incoming events | `backend` | `server/backend` | medium | One transaction with a fixed order of checks and value rules. | no | 5 |
| 8. The handshake | `backend` | `server/backend` | medium | Bytes fixed by worked examples and a short state machine. | yes, a machine reaches `Open` without a session | 4, 5 |
| 9. The hub | `backend` | `server/backend` | medium | A map under one lock and one feed subscriber. | no | 1, 6 |
| 10. The stream loop | `backend` | `server/backend` | medium | Joins tested functions in one select loop with one writer. | yes, the loop reads every message a machine sends | 2, 6, 7, 8, 9 |
| 11. `WatchClients` | `backend` | `server/backend` | medium | One pure mapping and one adapter of the store's helper. | yes, the website reaches it | 1, 2, 5 |
| 12. Record calls | `backend` | `server/backend` | medium | Three handlers over tested functions, with request keys. | yes, the website reaches them | 3, 5, 11 |
| 13. Command calls | `backend` | `server/backend` | medium | Four handlers on one pattern in one write each. | yes, the website reaches them | 6, 11 |
| 14. Wiring | `backend` | `server/backend` | medium | Composition in one function, one setting and four test helpers. | yes, it places the guard | 4, 10, 11, 12, 13 |

Items 4, 8 and 10 were the brief's high items 4, 5, 6 and 7, cut so that each part has its own tests.
No item is left high.

Load: each of the 14 items needs one test writer and one builder of the other vendor, which is 28 launches. The 12 risky items each need one reviewer who breaks the code on a temporary branch, which is 12 launches. Code review of the merged sprint needs about 6 launches in slices: one per workstream from the vendor that did not build it, and two that read beside them. That is about 46 launches, split about evenly between the first two vendors, with about 8 on the third for reading beside. Each plan review round adds about 6 launches. The orchestrator reads each vendor's remaining weekly allowance at the start of the sprint, as the epic says, and the planner cannot read it. If one vendor would run out, items 12 and 13 can wait, because only item 14 needs them.

</details>

<details><summary><b>Attack log (gate 7 — M/L only)</b></summary>

| Round | New findings (B/M/m) | Outcome |
|---|---|---|
| 1 | not run yet | The orchestrator runs the review rounds with other models, by the rules of the build loop. This plan stops at its self-check. |

</details>
