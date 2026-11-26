# Scouting — go-s3-server-machines

The planner read these files on 2026-11-26 and ran no program.
No Go code exists in the repository yet, so every server file of this plan is a target path.
For each target, the nearest pattern is a file of an earlier plan or a file of the old engine or the sample server.

## S1. The design and the epic

- `docs/specs/2026-11-25-go-rewrite-foundation-design.md`. The decisions table has 23 rows, D1 to D23, at lines 26 to 48. The rules R1 to R11 are at lines 362 to 372. The checks C1 to C13 are at lines 378 to 390.
- The sections this plan implements are "Joining: a token that authorizes an exchange of keys" (lines 243 to 256) and "The link between the server and a machine" (lines 258 to 325).
- `.plans/epics/go-rewrite/epic.md`, "The teams, and what each may edit": the Server team edits only `src/server/`.
- The brief is the first version of `.plans/plans/go-s3-server-machines/plan.md`, dated 2026-11-25, with nine work items and four points to settle.

## S2. The contract that this plan builds against

- `.plans/plans/go-s0-contract/plan.md`, items 3 to 6, and the drafts in `.plans/plans/go-s0-contract/draft/`.
- `draft/link.proto`: `service LinkService` with `rpc Join(JoinRequest) returns (JoinResponse)` and `rpc Open(stream OpenRequest) returns (stream OpenResponse)`. The messages are `JoinToken`, `JoinRequest`, `JoinResponse`, `OpenRequest`, `OpenResponse`, `Hello`, `Challenge`, `Proof` and `Welcome`.
- `draft/commands.proto`: `Command` with `id`, `attempt`, `expires_at` and the oneof `action` holding `scan_harnesses`, `set_state` and `restart`. `CommandResult` with `command_id`, `attempt`, `outcome` and `message`. The enums `CommandOutcome` and `Approval`, and `MachineState` with `approval`, `name` and `revision`.
- `draft/events.proto`: `Event` with `number`, `happened_at` and the oneof `report` holding `facts` and `harnesses_found`. `EventAck.through`, `Facts`, `HarnessesFound`, `Harness`, `Heartbeat` and `HeartbeatAck`.
- `draft/existing-files.diff`: `WatchClientsResponse.position` as field 4, of type `lgtm.v1.FeedPosition`, and the new comments of `ReconnectClient`, `RevokeClientToken`, `Client.hosts_server`, `Client.address` and `CreateClientTokenResponse`.
- The proof texts and their worked examples: the contract plan's item 6, `src/contracts/testdata/link-proof.json` (target). The token example: item 5, `src/contracts/testdata/join-token.json` (target).
- The orchestrator told this planner on 2026-11-26 that round 2 of the contract removes `Hello.acked_event_number`. The server alone decides what was applied. Restoring the server's database to an earlier state is outside wave 1.

## S3. The website's contract as it is today

- `src/contracts/proto/lgtm/v1/clients.proto`: `service ClientService` with `WatchClients`, `CreateClientToken`, `RevokeClientToken`, `ApproveClient`, `RenameClient`, `ScanClientHarnesses`, `ReconnectClient` and `RemoveClient`. `message Client` has fields 1 to 18, ending with `token_id`. `enum ClientState` has `CLIENT_STATE_WAITING_FOR_APPROVAL`, `CLIENT_STATE_JOINING`, `CLIENT_STATE_ONLINE` and `CLIENT_STATE_OFFLINE`.
- `src/frontend/src/server/sync.ts`, function `watch`: the website reads `all`, `saved` and `removedId`, reopens a closed stream with a growing wait, and shows a signed-out page on `unauthenticated`.
- `src/frontend/src/screens/service.tsx`, lines 349 to 351: "Scan harnesses again" for an online machine and "Try to reconnect" for an offline one. Lines 368 to 374: the pairing dialog revokes its token when it closes.
- The website sends no `Lgtm-Request-Key` header today. A search of `src/frontend/src` for that name found nothing.

## S4. The sample server, the nearest pattern for the Machines calls

- `src/mock-server/src/engine.ts`, `router.service(ClientService, ...)`, lines 162 to 245.
- `uniqueName` (line 136): a name is trimmed, must not be empty, and must differ without regard to case from the name of every machine that does not wait.
- `approveClient`: refuses a machine that does not wait with `failed_precondition`.
- `revokeClientToken`: removes a waiting machine that used the token.
- `removeClient`: refuses the machine that runs the server. This plan departs from that, see decision 21.
- `scanClientHarnesses`: refuses a machine that is not online.
- `need` (line 129): an unknown identifier gives `not_found`.

## S5. The database layer this plan stands on

- `.plans/plans/go-s1-server-store/plan.md`, as it stood on 2026-11-26.
- `store.Store`, `Store.Read`, `store.ReadTx`, `Store.Write`, `store.Tx` with `Exec`, `Query`, `QueryRow` and `Record`, `store.ChangeRef` with `Topic`, `Scope`, `Key` and `Change`, and `store.Saved` and `store.Removed`.
- `Store.Migrate` with `store.Migrations{Part, Files}`. Part names match `[a-z][a-z0-9_]*`, and files are `NNNN_description.sql` from 1.
- `Store.WriteOnce` with `store.Request{Key, ReceiptSafe, Caller, Procedure, FingerprintVersion, Fingerprints}` and `store.RequestKeyHeader`.
- `store.StartFollower`, `Follower.Subscribe`, `Subscription.Start`, `Next`, `Ack`, `Context` and `Close`, `store.ErrWatchLagged` and `store.ErrFeedUnavailable`.
- `store.NewID`, which returns a version 7 universally unique identifier.
- `watch.Serve`, `watch.Source[T]` with `Topic`, `Scope`, `Authorize`, `Snapshot` and `Load`, `watch.Message[T]` with `Position`, `Snapshot`, `Saved` and `RemovedKey`, and `watch.ErrWatchUnauthorized`.
- `storetest/migrated.NewStore(t, schema, parts...)` for part tests, and `storetest.NewDatabase`.
- The store reserves advisory lock keys `841941100001`, `841941100002` and the namespace `1279743053`. This plan uses row locks only.
- Both `*store.ReadTx` and `*store.Tx` have the methods `Query` and `QueryRow` with the same signatures, so a Go interface of the two methods accepts both.

## S6. Sign-in and the server program

- `.plans/plans/go-s2-server-signin/plan.md`, as it stood on 2026-11-26, at status `self-check`.
- `auth.Sessions.CheckInRead(ctx, tx *store.ReadTx, verifier auth.Verifier) error`, `auth.ErrNoSession`, and the constant principal `owner` for keyed calls of the website.
- `api.Options` with `Sessions`, `Origins`, `Session`, `Clients` and `Link`, `api.NewHandler`, and `api.SessionVerifier(ctx)`. The two link paths are mounted outside the session guard.
- `config.Config` with `ListenAddr`, `PublicURL`, `AllowedOrigins`, `Password`, `Postgres`, `AllowPrivateStack`, `OpenBaoAddr` and `OpenBaoCredentialFile`. It has no setting for the certificate.
- `app.Start`, `Server.SigningPublicKey`, and the in-memory signing key from `vault.Client.LinkSigningKey`.
- `servertest.Start(t, Options{Password})`, with `URL`, `HTTPClient`, `SigningPublicKey`, `Stop` and `Restart`. Its text says that the later machine work plugs its handlers and migrations into the same application.

## S7. The plans of the stack

- `.plans/plans/go-s2-stack-web/decisions.md`, rows 9 and 23 to 25: encryption ends at the web server, which passes calls to the server as cleartext HTTP/2. The server reads the public certificate from `LGTM_TLS_CERT_FILE`, which is `/run/lgtm-tls/tls.crt` in public mode and empty in loopback mode.
- `.plans/plans/go-s2-stack-web/plan.md`, the web server rules: `reverse_proxy h2c://server:8080 { flush_interval -1 }` for `/lgtm.link.v1.*`.
- `.plans/plans/go-s3-stack-client/plan.md`, item 1: `lgtm-server join-token --address <address> [--hosts-server]` in `src/server/cmd/lgtm-server/join_token.go`, with the function `joinTokenCommand`. Its result file asks this plan for a token function with an address and a mark of the server's machine.

## S8. The client plans

- `.plans/plans/go-s3-client-link/plan.md`, as it stood on 2026-11-26, at status `self-check`.
- The client reads the response header `Lgtm-Join-Refusal` on `permission_denied` from `Join`, and maps the values `token_used` and `token_expired`.
- The client sends an immediate heartbeat after `Welcome`, and every 10 seconds after that. While waiting, it sends heartbeats without load and no events. It sends its facts and all four harness entries after approval.
- An `unauthenticated` rejection after a verified `Challenge` stops the client as an unknown machine.

## S9. The user-flow script

- `deploy/test/checks.sh`, functions `check_c5` to `check_c11`, and `deploy/test/README.md`, "Names and observations the checks expect".
- `check_c5` updates `client_join_tokens.expires_at` by `id`, and expects a token lifetime of 885 to 905 seconds from `CreateClientTokenResponse.expires_at`.
- `watch_join` expects a `saved` record with the waiting state and `tokenId` equal to the new token.
- `check_c7` expects `os`, `version` and `workspace` not empty, `cores`, `memoryGigabytes` and `agentLimit` above 0, both load fields from 0 to 100, `harnessesScanned` true and the four harness names.
- `check_c9` expects a new `connectedAt` after a server restart.
- `server_sequences` reads `msg` `link_event_applied`, `machine_id` and numeric `sequence` from the server's log.

## S10. The issues

- `docs/issues.md`, issues 1632 to 1640, at lines 7501 to 7590.
- `.foundry/go-rewrite/owner-choices.md`, items 1 and 6 to 9: the defaults for a call that returns a credential, and the four points for the owner from the contract plan.

## S11. The blueprint

- `floorplan/plan.json`, entries `src/server/internal/machines/store.go`, `machines/tokens.go`, `machines/service.go`, `link/hub.go`, `link/outbox.go`, `link/inbox.go` and `link/service.go`.
- The blueprint has `machines/service.go` use `link/hub.go`, and `link/service.go` use `machines/tokens.go`. In Go that is an import cycle, so this plan inverts one direction, see decision 9.

## S12. Patterns for each target file

| Target | Nearest pattern |
|---|---|
| `src/server/internal/machines/store.go`, `Registry` | `src/server/internal/auth/sessions.go`, `Sessions`, of plan `go-s2-server-signin`, item 4. |
| `src/server/internal/machines/migrations/0001_machines.sql` | `src/server/internal/auth/migrations/0001_web_sessions.sql` of the same plan, item 3. |
| `src/server/internal/machines/tokens.go`, `Tokens.Create` | `src/mock-server/src/engine.ts`, `createClientToken`, and the token example of `go-s0-contract`, item 5. |
| `src/server/internal/machines/join.go`, `Tokens.Redeem` | `src/server/internal/auth/sessions.go`, `Sessions.Issue`, for one transaction that checks and writes. |
| `src/server/internal/machines/watch.go` | `src/server/internal/watch/watch.go`, `Serve`, of plan `go-s1-server-store`, item 9, and `src/mock-server/src/engine.ts`, `watchClients`. |
| `src/server/internal/machines/service.go` and `commands.go` | `src/mock-server/src/engine.ts`, `router.service(ClientService, ...)`. |
| `src/server/internal/link/handshake.go` | `src/contracts/open_test.go`, `TestProofExamplesHoldTogether`, of plan `go-s0-contract`, item 6. |
| `src/server/internal/link/presence.go`, `outbox.go`, `inbox.go` | `src/server/internal/store/receipts.go`, `WriteOnce`, of plan `go-s1-server-store`, item 5, for row locks inside `Store.Write`. |
| `src/server/internal/link/hub.go` | `src/server/internal/store/follow.go`, `Follower`, of plan `go-s1-server-store`, item 8. |
| `src/server/internal/transport/deadline.go` | The standard library's `net/http.ResponseController.SetWriteDeadline`. |
| `src/server/internal/link/machinetest/machine.go` | `src/server/servertest/server.go` of plan `go-s2-server-signin`, item 9. |
