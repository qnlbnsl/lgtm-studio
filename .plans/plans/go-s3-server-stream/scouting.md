# Scouting for the server stream

## Assignment, design and review inputs

Read the bounded task, the common planner rules, the foundation design, the epic, the build loop's Phase 4, and issues 1632 to 1635. Read the planning template and its tier M probes and the work-area and complexity table. The foundation statement numbers used in the plan count the owner's decision rows, the later-part rules and the wave 1 checks in order.

Read [the Part B handoff](../go-s3-server-machines/handoff-stream-and-commands.md), the matching [reviewed plan](../go-s3-server-machines/attack/plan-v4.md) items 5, 7, 8, 9 and 10, and its shared locking, fixture, status and post-build sections. Read each of the four finding registers under the predecessor's `attack/` and the handoff's referenced [decision rows](../go-s3-server-machines/decisions.md). Read both round 4 reports' post-build sections. Row 88 replaces the skeleton. The four older registers explain why compile dependencies and stale registrations need their own observable checks.

Read [the owner defaults](../../../.foundry/go-rewrite/owner-choices.md) items 7, 8, 11 and 12. The frozen contract supersedes older language about waiting facts, very large report numbers and sending a displaced Welcome. The commands plan inherits the command-only findings.

## Frozen and built anchors

The following observations come from this session's checkout. The Part A builder was still landing work during scouting. The plan takes the settled tests and plan text as the interface of that work.

| Source and anchor | What it supplies |
|---|---|
| `src/contracts/proto/lgtm/link/v1/link.proto`: `LinkService.Open`, `Hello`, `Challenge`, `Proof`, `Welcome` | The proof order, byte format, first-frame fence, waiting behavior and durable replay mark. |
| `src/contracts/proto/lgtm/link/v1/events.proto`: `Event`, `EventAck`, `Facts`, `HarnessesFound`, `Heartbeat`, `HeartbeatAck` | The number-first decision table, report value rules, transaction boundary and monotonic timeout. |
| `src/contracts/proto/lgtm/link/v1/commands.proto`: `MachineState`, `Command`, `CommandResult`, `SetState` | The state fields for Welcome and seams for the later plan. |
| `src/contracts/testdata/link-codes.json` and `link-proof.json` | The retry and refusal table, reason keys and fixed proof examples. |
| `src/contracts/proto/lgtm/v1/clients.proto`: `Client`, `ClientState`, `WatchClientsResponse` | The four displayed states and optional load, timestamps and watch frame shape. |
| `src/server/internal/store/pool.go`: `Store.Read`, `withWrite`, `sanitizeError`, `sanitizeCommitError`, `Error.Outcome` | Existing repeatable reads, bounded write transactions, unwrapped domain errors and classification of an uncertain commit. |
| `src/server/internal/store/feed.go`: `Store.Write`, `Tx.Record`, `flushFeed`. `receipts.go`: `Store.WriteOnce` | The commit-order feed and keyed write boundary. The store checks constraints before flushing and committing. |
| `src/server/internal/store/read_feed.go`: `Store.Snapshot`. `follow.go`: `Follower.Subscribe`, `Subscription.Start`, `Next`, `Ack`, `Close` | Use an acquired subscriber before taking its head. Acknowledge each delivery. Reopen after lag or feed unavailability. |
| `src/server/internal/store/instance.go`: `AcquireServerInstance`, `InstanceLock.Check`, `Close` | One live server owns the database. Stop work before releasing the lock. |
| `src/server/internal/store/storetest/postgres.go`: `NewDatabase`. `storetest/migrated/store.go`: `NewStore`. `storetest/observe.go`: `WaitOnRow`, `Poll`, `ReceiveFrom` | Real disposable PostgreSQL fixtures and checks for staged races. A commit gate uses a test-specific connection configuration and tracer. |
| `src/server/internal/watch/watch.go`: `Serve`, `readChange`, `reauthorize` | A shared snapshot and current-record load path with session checks and a supplied send context. |
| `src/server/internal/transport/deadline.go`: `WithTimeout`, `WithWriteDeadline`, `Send` | A context-aware write bound. Send waits for a cancellation callback already in progress before returning. |
| `src/server/internal/api/server.go`: `NewHandler`, `Options.Link`, `linkReadLimit`, `linkProcedures` | Generated Open and Join bypass the web-session guard. The router wraps the link with deadline middleware and a 64-kibibyte read cap. |
| `src/server/internal/machines/store.go`: `Registry.Get`, `List`, `SetApproved`, `Delete`, `DeleteWaiting`, `recordMachineChange`. `migrate.go`: `Migrations` | The registry owns its rows and feed references. The approval update already takes the intended row lock. |
| `src/server/internal/machines/join.go`: `Tokens.Redeem` | A fixed-order transaction that owns the machine write and stable result. |
| `src/server/internal/machines/migrations/0001_machines.sql` | Facts, harness inventory, scan status, last seen and last address already have columns. |
| `src/server/internal/machines/records.go`: `ApproveClient`, `RemoveClient`, `RevokeClientToken`, `recordRequest`. `records_test.go`: `TestKeyedCallsReplay`, `TestRevokeClientToken`, `TestRemoveClient` | Preserve the keyed write around approval projection and both removals. Revocation already decides deletion of waiting machines within the token transaction. |
| `src/server/internal/machines/service.go`: `NewService`, `WithFollower`. `project.go`: `Project`. `watch.go`: `WatchClients`, `watchResponse` | Project a machine inside the shared watch reads after obtaining its presence in the same transaction. Preserve the pure Part A projection entry point for existing callers. |
| `src/server/internal/link/service.go`: `NewService`. `join.go`: `Join`, `errInvalidLinkMessage`, `errDatabaseUnavailable`. `rules.go`: `canonicalUUID` | Join currently takes the server's public key and has stable refusal text. Open needs the private signing key separately. |
| `src/server/internal/vault/identity.go`: `LinkIdentity.SigningKey`, `LoadLinkIdentity`. `src/server/internal/app/start.go`: `Start`, `startup.run`, `serve`, `release` | The app already loads the signing key, holds the instance lock, starts the follower and joins its background workers on release. |
| `src/server/servertest/server.go`: `Start`, `Options`, `Clients`, `Link`, `launch`, `Restart`, `Environment` | The public fixture keeps its database, vault and cookie jar across restarts. Its standalone machine-link client has no cookies and currently uses a short whole-request timeout. |
| `src/server/internal/app/machines_test.go`: `TestServerServesMachines`, `TestRestartKeepsMachines`. `src/server/servertest/machines_test.go`: `TestMachinesFixtureClientsAndLink` | Keep the website, Join, restart and fixture assertions. The Open subtest explicitly expects the Part A stub. It needs a test-baseline transition on activation. |
| Installed Connect dependency: `connectStreamingHandlerConn.Send`, `grpcHandlerConn.Send`, `flushResponseWriter`, `envelopeReader.Unmarshal` | Both streaming senders write a framed response and flush. The flush path checks `http.Flusher`. Framed decoding has a separate error path from a decoded message with invalid values. |
| `src/client/internal/link/conn.go`: `openSession`, `validateWelcome`, `streamError`, `proofText`. `replay.go`: `newReplay`, `next`, `ack`, `isUnnumberedReport`. `report.go`: `captureFacts`, `heartbeat` | The client verifies Challenge before terminal removal and uses the Welcome mark to resume. Its validator uses named harnesses and string limits. |
| `src/client/internal/link/store.go`: `eventsBucket`, `OpenStore`. `outbox.go`: `Store.Ack`, `Len` | The durable outbox is the events bucket of the client database. The combined fixture can inspect that bucket read only after the client exits. |
| [Client link plan](../go-s3-client-link/plan.md) items 3, 4, 5, 7b and 9 | The client sends reports when approved and retains the outage queue. The public `clienttest.Options`, `Join` and `Run` entry points are planned. `src/linktest/` is the orchestrator's combined suite. |
| `deploy/test/checks.sh`: `check_c5` through `check_c11`. `server_sequences` | C7 starts the client after approval. C8 measures a stopped client. C9 observes a new connected time. C10 covers both keys. C11 currently uses application logs as its effect assertion. |
| `floorplan/plan.json` | The proposed link file names include `hub.go` and `inbox.go`. The reviewed handoff and live anchors supply ownership and signatures. |

## New files and neighboring patterns

| Target | Existing pattern |
|---|---|
| `src/server/internal/machines/link.go`, `stream_registry_test.go`, `stream_projection_test.go`, `stream_records_test.go` | `machines/store.go` defines `Querier`. `machines/registry_locks_test.go`, `project_test.go`, `watch_test.go` and `records_test.go` establish the registry, projection and page-call fixture patterns. |
| `src/server/internal/link/hub.go`, `hub_test.go`, `run.go`, `run_test.go` | `link/service.go` is the service entry point. `store/follow.go` owns subscriptions and coalesced signals. `app/start.go` owns process lifetime. |
| `src/server/internal/link/presence.go`, `migrate.go`, `migrations/0001_link.sql`, `presence_test.go`, `oneline_test.go` | `machines/migrate.go` embeds migrations. `store/feed.go` writes the feed. `machines/registry_locks_test.go` stages a PostgreSQL row-lock race. |
| `src/server/internal/link/inbox.go`, `inbox_test.go` | `machines/join.go` owns a fixed-order transaction. `machines/join_rollback_test.go` demonstrates rollback observation. |
| `src/server/internal/transport/link_stream.go`, `link_stream_test.go` | `transport/deadline.go` and `deadline_test.go` wrap streaming requests and exercise cancellation. `api/long_stream_test.go` exercises generated streams over cleartext Hypertext Transfer Protocol version 2. |
| `src/server/internal/link/handshake.go`, `handshake_test.go`, `machinetest/machine.go`, `machine_key.go` and `machine_key_test.go` | `link/join_test.go` exercises generated calls with a real database. `src/client/internal/link/conn_test.go` exercises the peer handshake and protocol metadata. |
| `src/server/internal/link/stream.go`, `stream_test.go` | `transport/deadline_test.go` and `machines/watch_test.go` exercise blocked response writes and stream shutdown. |
| `src/server/internal/app/link_stream_test.go`, `src/server/servertest/link_stream_test.go` | `app/machines_test.go` and `servertest/machines_test.go` start the real app in process and test the generated clients. |

## Planning checks

| Gate | Observation at the planning handoff |
|---|---|
| 1. Decisions | All 24 rows have a rationale and are closed or carry a dated assumption. Scope rows name the dated sprint split and assignment. |
| 2. Acceptance | Each workstream names observable tests or checks. Each item names a red assertion and mutations. |
| 3. Design artifacts | The plan changes no screens and links to the frozen data and state design. |
| 4. Traceability | The traceability table accounts for the bounded inputs and connects them to the workstreams. |
| 5. Risks | Each workstream names its risks, severity and mitigations. |
| 6. Verification | Each workstream has tests and an environment check. One rollout sequence covers the plan. |
| 7. Review | The plan names the inherited review evidence. The orchestrator performs this plan's review after the self-check. |
| 8. Anchors | Existing task anchors were checked in source. Every new file has a neighboring pattern. |
| 9. Guardrails | The plan accounts for the repository rules and the Server team's editable folder. |
| 10. Sizing | Each unit has one area, scope and size. The scope check passed. The coordinator compares the one-vendor launch count with its allowance at dispatch. |


- Compared the foundation design with `master` and the frozen link files and code table with the freeze commit. Both comparisons are empty.
- Ran the read-only scope checker. It returned `ok: server/backend`.
- Re-read the four deliverables against the writing rules. Checked the ten planning gates at their pre-review boundary. The structural checks covered the template, all three workstreams, 10 work items, the decision statuses, acceptance observations, the file and symbol anchors, dependency order, mutations and local links.
- The planning review belongs to the orchestrator. The epic row is still marked planned and its link transition belongs to the same handoff. The bounded planner cannot read the vendor allowance.
- The bare Node.js 26.13.0 privacy check hit the sandbox's `spawnSync git EPERM`. The same script and Node.js version exited 0 with a process-only preload setting the Git child's unused input to ignore. Ran the same privacy rules against the four new files without staging them. Runtime acceptance commands belong to the build.

## Round 1 verification against source

Read the four round 1 reports and compared the frozen plan with commit `abdc8de1`. The source findings below use the frozen protocol, the approved foundation design and the checkout. The table distinguishes tracked code from the in-progress machine baseline.

| Finding group | Evidence checked |
|---|---|
| Replay barriers, wire mark and rollout | The foundation's C11 is at `docs/specs/2026-11-25-go-rewrite-foundation-design.md:388`. `src/contracts/proto/lgtm/link/v1/link.proto:125` ties Welcome to admission. `src/client/internal/link/replay.go:23` drops entries through that Welcome mark. `src/server/servertest/server.go:144` starts the real app and exposes no report boundary in the Part A fixture. Client item 9 defines a runner with a disposable state directory and no transport injection. |
| Stack and baseline | `deploy/test/checks.sh:471` runs the token check and `:586` defines `server_sequences`. `deploy/test/README.md:88` requires ordered log records. Stack item 2.3 owns the script and its documentation, and G4 names its inputs. The epic at `.plans/epics/go-rewrite/epic.md:75` puts cross-team tests at the sprint gate. `git ls-tree` at `c4fca798` showed the fixture stub and no `records.go`. The records, router and fixture edits were visible only in the working tree during the check. `src/client/cmd/lgtm-client/main.go:19` still refused join on that commit. |
| Proof cutoff, versioned identifier and independent test | `src/contracts/proto/lgtm/link/v1/link.proto:108` sets lookup and revision precedence before the Proof deadline. `src/server/internal/store/pool.go:301` shows a context-bounded write and the read path uses its caller's context. `src/server/internal/store/ids.go:11` writes a version 7 universally unique identifier (UUID). `src/server/internal/link/service.go:14` embeds the unimplemented generated handler. The reachable item 7 test therefore goes in the same package as the private Open continuation. |
| First-write status, receive size and credential headers | `src/server/internal/transport/deadline.go:37` arms the ordinary `Send` callback before entering its function. The pinned Go response writer `http2responseWriter.SetWriteDeadline` can reset a stream on an expired deadline. In Connect 1.21.1, `envelopeReader.Read` reads a five-byte prefix then uses `io.CopyN` to drain a declared oversize. `Unmarshal` separately bounds decompression. `connectStreamingHandlerConn.Send` and `grpcHandlerConn.Send` defer a flush even when a write fails. `src/server/internal/api/server.go:164` applies the size limit to the shared handler. `src/server/internal/link/join.go:43` sets a no-store response precedent. |
| Current identity and cleanup | `src/contracts/proto/lgtm/link/v1/link.proto:164` prescribes the removal refusal for the next frame. `src/server/internal/store/pool.go:20` sets the pool minimum and reserves three slots from the write budget. Its `Error.Retryable` includes the PostgreSQL lock-unavailable result. The older cleanup queue could delay other machines before making any write. The foundation imposes the offline window at line 385. |
| Subscription recovery | `src/server/internal/store/read_feed.go:45` returns snapshot read errors. `src/server/internal/store/follow.go:127` makes subscription lifetime inherit its context, and `:246` starts it at the supplied head. The recovery loop bounds the snapshot without applying that bound to the subscription lifetime. |
| Presence and startup | `src/server/internal/machines/records.go:72` reads the approval inside `WriteOnce`. `src/contracts/proto/lgtm/v1/clients.proto:64` defines last contact. `src/server/internal/app/start.go:51` sets the prerequisite budget, `:245` opens listening and `:315` releases resources. The ledger refuses unknown migrations in `src/server/internal/store/migrate.go:101`. |
| Registered key | `src/server/internal/link/rules.go:13` and `src/server/internal/machines/join.go:124` lead to storing a public key whose length comes from enrollment validation. `src/server/internal/machines/migrations/0001_machines.sql:3` holds it. The pin for the added validator supports Go 1.24 or later. Its point parsing accepts noncanonical encodings, so use a canonical round trip together with the subgroup check. The target `link/machine_key.go` follows `link/rules.go` and the standard signature verification pattern in `src/client/internal/link/conn.go`. [The pinned library source](https://github.com/FiloSottile/edwards25519/tree/v1.2.0) and [its package documentation](https://pkg.go.dev/filippo.io/edwards25519@v1.2.0). |
| Forwarding and stored metadata | `src/server/internal/app/start.go:399` validates a private-stack listener. `deploy/compose.yaml:166` and `:194` place the server on multiple private networks. `deploy/Caddyfile:23` supplies the proxy hop. `src/contracts/proto/lgtm/v1/clients.proto:48` permits missing address provenance. The named database volume at `deploy/compose.yaml:249` has no documented at-rest requirement. The foundation at lines 177 through 182 and 215 through 228 retains keyed replies for one day and protects signing material in OpenBao. |

Round 1 target anchors include `link/machine_key_test.go` beside the proof tests, `transport/link_stream.go` beside `transport/deadline.go`, and the additional application startup and fixture cases beside `app/machines_test.go` and `servertest/machines_test.go`. The shared `Service` anchor is in `src/server/internal/link/service.go`. The future `src/linktest/` suite and Client fixture are handoffs to their recorded owners.

The scope checker returned `ok: server/backend`. The frozen review plan matches its committed copy, and the revision 1 link and refusal table match the freeze. The foundation text matches the reviewed commit. No runtime acceptance was claimed for the planned changes.

The final tier M pass checked 40 dated decision rows, nine workstream acceptance criteria, no screen change, the traceability universe, three risk registers, one rollout and the named environment checks. It also checked all major and blocker markers, task anchors, repository guardrails, nine sized units, the 30-launch count, frontmatter, section order and epic linkage. The new plan has fewer words than the frozen copy. The finding probe matched all 29 report entries, counted 58 points after grouping and found no stuck statement. The repository privacy checker passed on the tracked files and both new deliverables with the Git child's unused input set to ignore.
