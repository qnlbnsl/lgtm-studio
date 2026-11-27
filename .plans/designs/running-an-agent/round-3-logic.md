REVISE

Grade 6 out of 10.

This is a cross-vendor review. A Codex worker reviewed the Claude-authored third draft at commit `5cbb308e` on `sprint-0`. The findings are based on reading the draft, the approved foundation, the two scouts and the source. The scenarios concern the proposed design.

## What the third draft does with the round 2 findings

The references in this table point to sections of the [third draft](../../../docs/specs/2026-11-27-running-an-agent-design.md).

| Round 2 finding | Disposition |
|---|---|
| logic 1 | Closes in part. “A folder holds one lead until the lead is proven gone” and checks A9 and A10 keep the reservation through the visible deadline. Some listed proofs can still release it early. |
| logic 2 | Closes in part. “How the runner keeps order for one agent” and A2 put `AgentStarted` before the first answer and turn end. “The states of an agent” assigns a replayed start to the stream that brings it. |
| logic 3 | Closes. “Link revision 2” and “Link revision 3” give the separate releases separate revisions, keep the earlier freeze, and require a replay check in R10. |
| logic 4 | Closes. “Nothing is sent to a client process before its past is known” checks admission and each event, holds queued commands in place, and proves same-stream release in R9. |
| logic 5 | Closes. “A folder operation outlives the call that began it” and R7 let the call time out and observe failure after the machine answers `expired`. |
| logic 6 | Closes in part. “Changes of the link's Go interface on the server” and “A folder operation outlives the call that began it” order the report before the result and save the operation identifier. The outcome table still permits a later failure to replace a committed success. |
| logic 7 | Closes. The `CancelTurn` table, “How the runner keeps order for one agent”, and A4 cover cancellation before the write, after submission, and after the turn ended. |
| logic 8 | Closes in part. “Commands of sprint 5” makes partial prompt writes fatal and defines `interrupted` and `timed out`. A timed-out prompt can still leave its turn active indefinitely. |
| logic 9 | Closes. “What sprint 5 guarantees about output, and what it does not” lets the process stop independently, flushes accepted text before the end event, and states the unread-output loss boundary. |
| logic 10 | Closes in part. “The runner and Claude Code” gives an unrecorded child a pipe guard. The reservation rule still needs proof of completed cleanup before a new run opens the link. |
| logic 11 | Closes. “Changes of the link's Go interface on the server” orders command allocation under the machine lock. `CreateProject` records the returned command identifier in the same write. |
| logic 12 | Closes. “The website in sprint 5” gates each project on its repository and lead, including after create and on reload. |
| logic 13 | Closes. “How output travels” ignores byte-identical repeats and starts each rollover at byte zero under a fresh item identifier. |
| fit 1 | Closes in part. “A folder holds one lead until the lead is proven gone” separates visible failure from checkout ownership. Expiry and run changes still need stronger evidence. |
| fit 2 | Closes in part. “What leaves a machine passes one filter” covers the readable sign-in store, paths, remotes and output. It records the keychain exception and defers the sandbox check under the owner's security decision. |
| fit 3 | Closes. “The sprints” and “The website in sprint 5” put name-based creation in sprint 5. The sprint 4 sidebar opens real folder addition. |
| fit 4 | Closes in part. The sprint 4 operation section distinguishes timeout from settlement and inspection from storage. The sprint 5 `MakeRepository` row admits an unknown effect and offers manual recovery. The original operation still needs a single durable outcome. |
| fit 5 | Closes in part. “The website in sprint 4” selects a contract revision. Both website sections define readiness and clear the watches on session or server change. Sprint 5 has no corresponding capability gate. |
| fit 6 | Closes in part. “New project” and “The runner and Claude Code” define the machine-scoped account, sign-in readiness, model-specific efforts, defaults and refresh. The design leaves the saved permission source and stale-choice invalidation open. |
| fit 7 | Closes in part. `ProjectService` and “What the client does with a folder” define canonical identity and branch facts. The website ignores answers for a different input, and A15 covers a link out of the repository root. A matching first commit can still override distinct remotes. |
| fit 8 | Closes in part. “The lead's column” and the state and item tables define most controls, fields and notes. The existing bare composer can still hide the stop action under the instruction to show it where it appears today. |
| fit 9 | Closes in part. The command tables cover the main client results, offline stops, cancellation and start ordering. They leave the effective stop retry path and the sweep clock unresolved. |
| fit 10 | Closes. Both revision sections name the freeze boundaries. The shared Link interface, R8 through R10 and the requirement to observe report effects cover command order, old buffers and replay. |
| fit 11 | Closes in part. The output guarantee separates local kill, accepted output and eventual reporting. The guard and A8 cover both running and starting children. A14 still exercises draining and then stopping, so it leaves a stop under pressure to the plan. |
| fit 12 | Closes in part. “The sprints” names crash loss, sign-in renewal, relocation with a report and the last pushed commit, and later repository tracking. The sprint 9 visible check still leaves receipt of the renewed sign-in unobserved. |
| fit 13 | Closes in part. “Every message of the link has a size budget” bounds individual fields, and `WatchThread` signals older history. An aggregate harness description can still exceed the event budget with valid fields. |
| fit 14 | Closes in part. R1 through R11, A1 through A16 and the inherited fixtures add most of the distinct observations. The crash between creation and its receipt and the ordering seen by the browser still need coverage. |

## Findings

### logic3-1 | BLOCKER | An expired start reply can free a checkout while its lead is alive

**Draft.** The `Expired, unsupported` row for `StartAgent` says that the start never ran and releases the reservation. A9 uses `expired` as proof. [Outcome rules](../../../docs/specs/2026-11-27-running-an-agent-design.md#L90) · [Start rows](../../../docs/specs/2026-11-27-running-an-agent-design.md#L414) · [A9 and A10](../../../docs/specs/2026-11-27-running-an-agent-design.md#L675)

**Scenario.** The client starts Claude Code and stores `AgentStarted` and a `done` receipt. The stream ends before the server receives the result. The client's clock runs far enough ahead that finishing another command prunes the start receipt. The client reconnects in the same run inside the server's 23-hour retry window. The repeated start finds no receipt and returns `expired`. Even if the result waits for the old `AgentStarted` event and the eight-minute sweep has already asked for a stop, the expiry row releases the checkout before that stop has proved that the process exited. A second project can reserve it while the first lead still runs.

**Evidence.** By reading, the frozen link explicitly allows a client with enough clock skew to return `expired` for a command it ran. `Store.Begin` checks the receipt before testing expiry, and `writeFinishedReceipt` prunes completed receipts using the client's wall clock. [Frozen command rules](../../../src/contracts/proto/lgtm/link/v1/commands.proto#L4) · [`Store.Begin`](../../../src/client/internal/link/receipts.go#L64) · [Receipt pruning](../../../src/client/internal/link/receipts.go#L159)

**Smallest repair.** Treat a bare `expired` as inconclusive for a start that may have been taken. Release only on durable proof that the start never ran and cannot run, or on confirmed process exit and a fence against a later start. Keep A9's release by proving that its command was never offered, or add an explicit non-execution proof to the receipt contract. Apply the same distinction to website outcomes that say a folder command did nothing.

### logic3-2 | BLOCKER | A replacement client can inherit an old start and free an undelivered one

**Draft.** The run changes after old reports are applied. `AgentStarted` belongs to the run of the stream that brought it. The run-change row fails agents belonging to an older run or to none, while the following paragraph keeps an undelivered start in `starting`. The reservation rule lists a run change as proof. [Run barrier](../../../docs/specs/2026-11-27-running-an-agent-design.md#L103) · [Reservation proofs](../../../docs/specs/2026-11-27-running-an-agent-design.md#L337) · [State and run rules](../../../docs/specs/2026-11-27-running-an-agent-design.md#L379)

**Scenario.** Run A writes the goal and buffers `AgentStarted`, then dies before finishing the receipt. Run B replays that event on its own stream. Literal stream attribution binds the lead to B. When the run changes to B, the agent stays `working` with no harness, and the later `interrupted` result assumes that the run change already failed it. There is a second failure for a different project whose start has never been offered. The state row can fail it and release its folder, while the prose still permits the pending start to be dispatched to B. A second project can then start in the same checkout.

**Evidence.** By reading, client replay reconstructs reports from their saved payload and transmits them on the new stream. The server inbox checks the current stream and applies the event by number. The stream ID alone identifies the transport. The command receipt identifies the command and whether it was started or finished. A recorded start with no finished receipt is `interrupted` on a new process. [Client replay](../../../src/client/internal/link/replay.go#L23) · [`ApplyEvent`](../../../src/server/internal/link/inbox.go#L27) · [Receipt lookup](../../../src/client/internal/link/receipts.go#L116)

**Smallest repair.** Carry or derive the original run for every numbered agent report, including events replayed by a replacement. At the barrier, distinguish a start still eligible to be offered from an abandoned start that can never run again. Keep the former queued and reserved. Fail the latter and release only in the transaction that prevents any further start and holds cleanup proof. Require the new run to wait for verified process-group exit before it advertises that proof. The guard's asynchronous reaction to pipe closure alone gives the new client no completion signal. Exercise A8 on both sides of the `AgentStarted` buffer commit and add an undelivered start to the same restart case. [Guard](../../../docs/specs/2026-11-27-running-an-agent-design.md#L532)

### logic3-3 | MAJOR | A usable stream can leave a lead working or stopping forever

**Draft.** A timed-out `Prompt` leaves the turn active until the machine returns or is removed. A stop is reissued only when a stream is admitted while a stop is pending. The outcome table says the server waits two days past expiry. [Outcome rules](../../../docs/specs/2026-11-27-running-an-agent-design.md#L90) · [Prompt and stop rows](../../../docs/specs/2026-11-27-running-an-agent-design.md#L427)

**Scenario.** A message is accepted just before the link drops, with its prompt still in the outbox. The same client run reconnects after the retry cutoff. The prompt cannot be offered, and a later sweep records `timed_out`. The idle harness has no turn to end, no run change occurs, and the website keeps refusing messages. Independently, a stop reissued on a same-run admission can spend its two-minute lifetime waiting behind earlier work. For example, hundreds of machines can each keep a shared send permit nearly two seconds before releasing it to the next waiter. When the stop reaches the client it comes back `expired`. The still-open stream has no second admission to trigger another stop, so the process can continue and the control stays disabled.

**Evidence.** By reading, `claimNext` stops offering ordinary commands 23 hours after their expiry. `SweepCommands` starts checking for settlement after 24 hours and the sweep runs at startup and hourly. A send permit covers claim and dispatch. It supplies neither a receipt nor a prompt end. [Claim cutoff](../../../src/server/internal/link/outbox_claim.go#L17) · [Settlement grace](../../../src/server/internal/link/outbox.go#L15) · [Sweep timing](../../../src/server/internal/link/outbox_sweep.go#L49) · [Three send permits](../../../src/server/internal/link/outbox_send.go#L17)

**Smallest repair.** State the actual cutoff and sweep clocks, or explicitly change both. Return a lead to idle when the server can prove its prompt was never offered. In uncertain cases on a same-run return, recover durable proof of delivery and turn status, or fail the lead and ask for a confirmed stop. Keep a pending stop eligible for a fresh command after the gate opens or an earlier stop fails to take effect on a healthy stream. Fence attempts and apply bounded retry spacing. Cover a reconnect beyond the retry cutoff and expiry of a stop on an already admitted stream.

### logic3-4 | MAJOR | A staged result can lose to a retransmission or a sweep

**Draft.** A result that arrives before its event number is stored and settled when that event is applied. The result bears the last number the client had buffered. [Result barrier](../../../docs/specs/2026-11-27-running-an-agent-design.md#L78)

**Scenario.** The server accepts the result of attempt 1 as pending behind a numbered event and restarts before applying that event. If the command still looks unsettled to the normal claim path, the replacement stream claims attempt 2. The old pending result can now either settle against a different attempt or be discarded. Replaying the receipt may attach a later number if the client samples the current tail again. Near the retry cutoff there may be no attempt 2 at all. The sweep can settle `timed_out` before the numbered event releases a result the server already received.

**Evidence.** By reading, `AcceptResult` checks the current dispatched attempt and currently finishes it in one write. Both the retry and the sweep select commands with a null outcome. The client's current receipt stores an outcome byte, and `liveSession.serve` sends queued results before numbered events. [Result match](../../../src/server/internal/link/outbox_result.go#L11) · [Claim and attempt assignment](../../../src/server/internal/link/outbox_claim.go#L91) · [Sweep query](../../../src/server/internal/link/outbox_sweep.go#L35) · [Receipt and reply](../../../src/client/internal/link/commands.go#L299) · [Send priority](../../../src/client/internal/link/session.go#L623)

**Smallest repair.** Store the completion watermark in the finished receipt and replay it unchanged under the newest attempt. Define a validated, durably staged result as ineligible for ordinary resend or timeout while its prerequisite events are in flight. Match duplicates against the staged record and settle once under the machine lock. Provide an explicit default for revision 1 receipts and an explicit outcome if the prerequisite can never arrive. For `interrupted`, use the replacement run's fixed old-event boundary. Keep all waits outside the three send permits.

### logic3-5 | MAJOR | One folder operation can report success and later report failure

**Draft.** `FolderSeen` finishes the operation and stores the folder in the same write. The `Interrupted, timed out` row makes the operation failed with an unknown outcome. `MakeRepository` uses that table. [Folder operations](../../../docs/specs/2026-11-27-running-an-agent-design.md#L194) · [`MakeRepository`](../../../docs/specs/2026-11-27-running-an-agent-design.md#L460)

**Scenario.** The client creates a Git repository and durably appends `RepositoryMade`, then dies before finishing the command receipt. The replacement replays the report and the server registers the repository. A waiting call can return that repository immediately. The same replacement answers the pending command `interrupted`. The outcome table then changes the operation to unknown, so a retry using the same key sees failure for a repository the server already registered. `AddRepository` has the same race after `FolderSeen`. Restarting the server between these two transactions preserves both effects.

**Evidence.** By reading, the client has separate transactions for the append and receipt. Unfinished receipts become `interrupted` after process restart. `WriteOnce` returns the saved reply bytes, which the draft correctly limits to the operation identifier. The operation row is therefore what a retry will read. [Event append](../../../src/client/internal/link/outbox.go#L23) · [Receipt lookup and finish](../../../src/client/internal/link/receipts.go#L116) · [`WriteOnce` replay](../../../src/server/internal/store/receipts.go#L71)

**Smallest repair.** Let a correlated report finish an operation exactly once. Apply command outcomes only to operations that still await a report. For repository creation, define whether an existing directory is refused or can be recovered, and require proof of ownership by the original operation before treating an existing directory as that operation's success. Preserve the stated manual add path for an effect that cannot be proved. Keep the website's wait bounded and outside the transaction. Exercise a crash between the saved report and receipt on both folder writes, and retry the same key after the server restarts.

### logic3-6 | MAJOR | A separate agent watch can show idle before the last answer text

**Draft.** The lead's column shows a growing answer and then idle. `WatchThread` streams items, while `WatchAgents` separately streams agent state. A1 asks a watch of the thread to observe the lead becoming idle. [Lead's first answer](../../../docs/specs/2026-11-27-running-an-agent-design.md#L272) · [Agent calls](../../../docs/specs/2026-11-27-running-an-agent-design.md#L315) · [A1](../../../docs/specs/2026-11-27-running-an-agent-design.md#L666)

**Scenario.** The server commits two growth steps and then `TurnEnded` in the promised order. The agent stream is quick and the thread stream is delayed. The website sees idle first and can offer the next message while the final text is still in transit. Since the shared watch reloads the current row at each feed reference, two updates to one item may also both carry its final value if the watcher is slow. The creation screen's project-and-lead gate does not order thread text against state.

**Evidence.** By reading, each `watch.Serve` takes its own snapshot and filters the feed by topic and scope. `readChange` loads the saved object by key as it stands when read. Commit order is visible in feed positions, but the separate watches do not together form a display barrier. [Watch delivery](../../../src/server/internal/watch/watch.go#L89) · [Current-row reads](../../../src/server/internal/watch/watch.go#L142) · [Feed commit order](../../../src/server/internal/store/feed.go#L131)

**Smallest repair.** Give the lead column a persisted thread position to meet before it shows a turn's terminal state, or send lead and thread changes in one ordered watch for that column. In A1, pause the stand-in until the first observed growth, and deliberately delay the thread stream at turn end. Assert that the column shows both growth steps and the final text before idle. Allow catch-up reads to combine intermediate growth.

### logic3-7 | MAJOR | Sprint 4 can leave repository folders bound to a removed machine

**Draft.** Sprint 4 stores folders under a machine and streams them to the website. The removal section starts folder and operation cleanup in sprint 5, alongside agent cleanup. [Sprint 4 service](../../../docs/specs/2026-11-27-running-an-agent-design.md#L178) · [Machine removal](../../../docs/specs/2026-11-27-running-an-agent-design.md#L621)

**Scenario.** The owner adds a repository in sprint 4 and then removes the machine on the Machines page. The machine watch drops the machine. Its repository folder has no stated removal path in that sprint, so it can survive a reload and refer to a machine that no longer exists. A direct database cascade would also need a repository change in the feed, or open pages will keep the obsolete folder.

**Evidence.** By reading, `RemoveClient` deletes the machine and calls Link's `Forget` in one write. `Forget` settles link commands and removes link state. There is no Projects part yet. Foundation rules 3 and 4 require an owning Go interface and a recorded change in the write that changes the repository. [Machine removal handler](../../../src/server/internal/machines/records.go#L59) · [`Forget`](../../../src/server/internal/link/presence.go#L245) · [Foundation rules](../../../docs/specs/2026-11-25-go-rewrite-foundation-design.md#L358)

**Smallest repair.** Put folder and operation cleanup in sprint 4. Invoke Projects through its own Go interface in the removal transaction and publish the repository changes there. Extend the same transaction with Agents cleanup in sprint 5. Preserve the already committed request receipt, and define what a retry returns when the folder later goes away.

### logic3-8 | MAJOR | A shared first commit can attach the wrong checkout

**Draft.** An explicit “Add to a machine” succeeds if either the normalized remotes match or both checkouts have the same first commit. R4 says a folder of another repository is refused. [Repository matching](../../../docs/specs/2026-11-27-running-an-agent-design.md#L184) · [R4](../../../docs/specs/2026-11-27-running-an-agent-design.md#L260)

**Scenario.** Two independent repositories grew from the same copied history and now have different origins. The owner selects the other checkout for a known repository. The first-commit test succeeds despite the two known origins. `CreateProject` later starts the lead in the other checkout while the website displays the intended repository.

**Evidence.** The predicate in the draft treats shared ancestry as enough by itself. The same draft defines the persisted folder as the actual working directory of `CreateProject`. [Folder matching](../../../docs/specs/2026-11-27-running-an-agent-design.md#L188) · [Project checkout](../../../docs/specs/2026-11-27-running-an-agent-design.md#L328)

**Smallest repair.** Require equality when both repositories have known, nonempty origins. For a missing origin, make same-root attachment explicit and confirmable, or refuse the attachment. Test two distinct origins with a shared first commit in R4.

### logic3-9 | MAJOR | A sprint 5 website can accept a server with no agent service

**Draft.** Sprint 4 raises the website contract revision when it adds `ProjectService`. Sprint 5 adds `AgentService` and waits for its initial agent list before showing the workspace. Its revision section changes the machine link. [Sprint 4 website](../../../docs/specs/2026-11-27-running-an-agent-design.md#L232) · [Sprint 5 link revision](../../../docs/specs/2026-11-27-running-an-agent-design.md#L283) · [Sprint 5 website](../../../docs/specs/2026-11-27-running-an-agent-design.md#L570)

**Scenario.** The sprint 5 website opens a sprint 4 server during a separate rollout or when the owner picks its address. If the website revision remains the same, the connection check accepts the server. `WatchAgents` then returns `unimplemented`. The promised readiness condition can never be met, even though the repository and machine watches are ready.

**Evidence.** By reading, `openServer` compares the value returned by `GetServer` to one website constant. That value is separate from the machine link revision. The router returns `unimplemented` for a service it does not handle. [Website handshake](../../../src/frontend/src/server/api.ts#L33) · [`GetServer`](../../../src/server/internal/api/session.go#L19) · [Unhandled service](../../../src/server/internal/api/server.go#L115)

**Smallest repair.** Define the new website capability in sprint 5 before changing the workspace gate. Use a separate website revision for the incompatible pairing, or keep the repository workspace available when the server advertises that it lacks the agent service. Test that the wrong pairing returns a visible result without waiting on a watch that cannot start.

### logic3-10 | MINOR | A valid harness description can exceed the whole-message budget

**Draft.** The client rejects any encoded event larger than 48 kibibytes before numbering it. It can send 100 models, with titles or labels as long as 512 bytes, and each model's offered efforts. `DescribeHarness` leaves a previous description in place on any outcome other than done. [Size budget](../../../docs/specs/2026-11-27-running-an-agent-design.md#L114) · [Description command and report](../../../docs/specs/2026-11-27-running-an-agent-design.md#L458)

**Scenario.** A description has 100 models with valid 512-byte labels. The labels alone take 51,200 bytes, above 48 kibibytes before model identifiers, efforts or encoding overhead. The client correctly refuses to number it. The design specifies no bounded description to send instead. A machine with no previous description remains unusable in the lead picker, or one with a previous description appears usable with stale choices.

**Evidence.** The draft establishes the per-field and aggregate limits independently. By reading, the built transport caps each incoming link message at 64 kibibytes on both sides. The client assigns a sequence only after its size and buffer checks. [Server receive limit](../../../src/server/internal/api/server.go#L28) · [Client receive limit](../../../src/client/internal/link/session.go#L61) · [Client append](../../../src/client/internal/link/outbox.go#L23)

**Smallest repair.** Trim display metadata and complete option records by encoded size, mark the list incomplete, and preserve identifiers exactly. Give any unrepresentable required fact a small correlated refusal. Define what a picker shows after that refusal and test a description at the cap and one above it.

## Checks that hold

- The `CreateProject` write can include the reservation, project, first item, lead, queued start, stored start identifier and request receipt. `WriteOnce` supports one caller transaction and flushes the change feed before commit. Projects and Agents can own their respective tables through methods taking that transaction. A disconnect or server crash before commit leaves none of this write. After commit a repeated request can recover both identifiers. [Project write](../../../docs/specs/2026-11-27-running-an-agent-design.md#L326) · [Store receipt](../../../src/server/internal/store/receipts.go#L41) · [Store transaction](../../../src/server/internal/store/feed.go#L102)
- While the client lives, a link outage and server restart preserve the client run. A numbered event committed just before an acknowledgement was lost is ignored on replay. Appending below the current text end ignores a fully covered byte-identical repeat with a fresh number. A complete character that crosses the item cap goes into a new item at byte zero. The highest tool revision wins. These rules preserve output already stored when the link or server restarts. [Inbox mark](../../../src/server/internal/link/inbox.go#L49) · [Text and tool rules](../../../docs/specs/2026-11-27-running-an-agent-design.md#L493)
- The state table has a normal path from `starting` to `working` to `idle`, accepts another prompt only from idle, and scopes settlement and cancellation to the active turn. `AgentEnded` and removal have terminal rows, and a replayed outcome cannot revive a final state. A same-run reconnect has no lifecycle effect by itself. [State table and transition rules](../../../docs/specs/2026-11-27-running-an-agent-design.md#L379)
- The link freeze can add the folder-only revision 2 and then revision 3 without rewriting an earlier record. By reading, the Go freeze test hashes every file of the link package and checks both revision comments and the readme. The shell check preserves earlier record lines at their original positions. R10 observes a real rejection and old on-disk data. The existing revision 1 digest test passed during this review. [Revision 2](../../../docs/specs/2026-11-27-running-an-agent-design.md#L153) · [Revision 3](../../../docs/specs/2026-11-27-running-an-agent-design.md#L283) · [Digest check](../../../src/contracts/freeze_test.go#L57) · [History check](../../../scripts/check-go.sh#L20)
- Sprint 4's request receipt stores the folder operation identity before the website waits. R1 through R3 distinguish an inspection from an add and concurrent aliases from duplicate rows. R7 distinguishes the website deadline from later link settlement. The stated output limits also distinguish loss on client death from replay on server restart and keep process kill independent of a full buffer. [Folder calls](../../../docs/specs/2026-11-27-running-an-agent-design.md#L178) · [Repository checks](../../../docs/specs/2026-11-27-running-an-agent-design.md#L252) · [Output guarantees](../../../docs/specs/2026-11-27-running-an-agent-design.md#L507)

## For the plans

- For R8 and R9, occupy all three send permits, apply old events on the gated stream, and check that claiming starts after the gate opens without advancing the cursor early.
- For A7, compare exact ordered text across a 32-kibibyte rollover, with a multibyte character at the boundary and both kinds of replay.
- For A14, deliver a stop just before the link drops, observe process exit while the buffer is full, and then compare accepted text and the stop note after reconnect.
- Replace browser snapshots on reconnect, ignore replies from superseded inspection intents, and check where the bare lead composer exposes both stop actions. [Existing composer](../../../src/frontend/src/ui.tsx#L1118)
- Exercise a lost `CreateProject` reply and a rollback after the lead and first item are written. Check project, lead, thread, reservation and command together.
- Use the configured repository folder for A15 and inspect the Git repository and its unborn `main` rather than only the returned database row. [Client configuration](../../../src/client/internal/config/config.go#L42)

The design needs revision before it can go to the owner for approval.
