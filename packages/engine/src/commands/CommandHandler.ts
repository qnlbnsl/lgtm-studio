import { prepareStartWorkspace } from "./startWorkspace.ts";
import { withProjectProgress } from '../project/progress.ts';
import { closeChangedPermissionQuestions } from '../project/permissionQuestions.ts';
import { closeSignIns, setTerminalSwitch } from "../harnesses/signinRequests.ts";
import { serviceRestartPending, completeServiceRestart } from "../client/serviceControl.ts"
import { recoverInstalls } from "../harnesses/lifecycle.ts";
import { acceptCache, type CacheTarget } from "./cache.ts";
import { randomUUID } from "node:crypto";
import {
  EventEnvelope, commandOutcome, publicAddressError,
  type Caller, type Event, type HarnessSettings, type RequestId,
} from "@lgtm/contracts";
import * as Cause from "effect/Cause";
import * as Context from "effect/Context";
import * as Effect from "effect/Effect";
import * as Exit from "effect/Exit";
import * as Fiber from "effect/Fiber";
import * as Layer from "effect/Layer";
import * as Option from "effect/Option";
import * as Redacted from "effect/Redacted";
import * as Schema from "effect/Schema";
import * as Semaphore from "effect/Semaphore";
import * as Stream from "effect/Stream";
import * as RpcSchema from "effect/rpc/RpcSchema";
import * as SqlClient from "effect/sql/SqlClient";

import { preloadAccountCredentials } from "../accounts/preloadCredentials.ts";
import { followAvailability } from "../accounts/available.ts";
import { sha256Hex } from "../identity/credential.ts";
import { callerKey } from "../identity/caller.ts";
import { verifyAgentToken } from "../identity/tokens.ts";
import { EventLog, firstRetryMillis, InvalidEvent, LogWriteFailed, ReceiptUnreadable, type SideRows } from "../log/EventLog.ts";
import { crashEvents } from "../recovery/crash.ts";
import { Restart } from "../recovery/service.ts";
import { Runner, RunnerEvent } from "../runner/Runner.ts";
import { dispatchCommitted, deliveryUpdate, promptReply, turnEnded } from "../routing/deliveries.ts";
import { settingsRefusal } from "../roles/stall-watch/settings.ts";
import { holderAbove } from "../routing/holders.ts";
import { preempt, withLane } from "../routing/lanes.ts";
import { permissionRequested } from "../routing/permissions.ts";
import { waitReply, withWaitChecks } from "../routing/waits.ts";
import { Views } from "../views/Views.ts";
import { apply } from "../views/apply.ts";
import { engineLists, type EngineLists } from "../pieces/lists.ts";
import { checkPieceBatch, pieceNamespace } from "./pieceBatch.ts";
import { SecretStore } from "./SecretStore.ts";
import { loadFirstDefaults } from "./harnesses.ts";
import { webSettingsCommitted } from "../client/settings.ts";
import { gitPlanners } from "../project/gitCommands.ts";
import { planGitOperation, refreshGitState } from "./gitOperation.ts";
import { prepareRegistration, registrationPlanners } from "../project/commands.ts";
import { prepareDesignRecord } from "../project/design.ts";
import { prepareMock } from "../project/mocks.ts";
import { finalizeMockEditorSave } from "../project/mockEditor.ts";
import { settleMockEditorSave, mockEditorUncertainReason } from "../project/mockEditorRecovery.ts";
import { prepareContextFiles } from "../project/context.ts";
import { promptPlanner } from "./planners.ts";
import { postPlanner } from "../board/commands.ts";
import { own, projectKey, projectView } from "../project/view.ts";
import { isCheckoutBaseReason, prepareRepositoryBases } from "../project/baseBranch.ts";
import { prepareProjectLifecycle, projectLifecycleCommand } from "../project/lifecycle.ts";
import { CommandRefused, CommandUncertain, type Plan, type Planner, freshRequest, readHarnesses, refused, type Input, type Runtime, type Snapshot } from "./internal.ts";

export { CommandRefused } from "./internal.ts";

type Credential =
  | { readonly _tag: "owner" }
  | Extract<Caller, { _tag: "service" }>
  | { readonly _tag: "agent"; readonly token: Redacted.Redacted<string> };

export class CommandHandler extends Context.Service<CommandHandler, {
  readonly cache: (target: CacheTarget) => Effect.Effect<"flush" | "compact" | undefined, CommandRefused | LogWriteFailed>;
  readonly input: (actor: Caller) => Effect.Effect<Input>;
  readonly execute: (credential: Credential, name: string, payload: unknown) => Effect.Effect<Record<string, unknown>, CommandRefused | LogWriteFailed>;
}>()("lgtm/engine/commands/CommandHandler") {}

const canonical = (value: unknown): string => {
  if (Array.isArray(value)) return `[${value.map(canonical).join(",")}]`;
  if (value !== null && typeof value === "object") {
    const entries = Object.entries(value).filter(([, item]) => item !== undefined).sort(([a], [b]) => a < b ? -1 : a > b ? 1 : 0);
    return `{${entries.map(([key, item]) => `${JSON.stringify(key)}:${canonical(item)}`).join(",")}}`;
  }
  return JSON.stringify(value);
};

const replyFor = (snapshot: Snapshot, name: string, requestId: RequestId, envelopes: ReadonlyArray<EventEnvelope>, settings: ReadonlyArray<HarnessSettings>) => {
  const events = envelopes.map((envelope) => envelope.event);
  if (events.some((event) => event._tag === "project.GitFinished" && event.outcome === "uncertain")) {
    return { failure: new CommandUncertain({ reason: name === "project.mocks.editor.save" ? mockEditorUncertainReason : "Git outcome is uncertain; refresh Git state" }) };
  }
  const gitRefusal = events.find((event) => event._tag === "project.GitFinished" && event.refusal !== undefined);
  if (gitRefusal?._tag === "project.GitFinished" && gitRefusal.refusal !== undefined) return { failure: refused(`Git: ${gitRefusal.refusal}`) };
  const failure = events.find((event) => event._tag === "CommandRefused");
  if (failure !== undefined) return { failure: refused(failure.reason) };
  const reply: Record<string, unknown> = { requestId };
  const started = events.find((event) => event._tag === "AgentStarted");
  if (started !== undefined) reply.agentId = started.agentId;
  const asked = events.find((event) => event._tag === "QuestionAsked");
  if (asked !== undefined) reply.questionId = asked.questionId;
  const queued = events.find((event) => event._tag === "DeliveryQueued");
  if (queued !== undefined && name !== "wait") reply.deliveryId = queued.delivery.id;
  if (name === "wait") Object.assign(reply, waitReply(events));
  if (name === "accounts") reply.accounts = Object.values(envelopes.reduce((state, envelope) => apply(state, envelope), snapshot).accounts);
  if (name === "harnesses") {
    const harnesses = new Map(settings.map((harness) => [harness.id, harness]));
    for (const event of events) if (event._tag === "HarnessChanged") harnesses.set(event.harness.id, event.harness);
    reply.harnesses = [...harnesses.values()];
  }
  return { reply };
};

// The most agent events held for one agent while writes fail (review round 1
// of unit f1-eventlog, item 4).
export const heldRunnerEventLimit = 256;
// The most bytes of held agent events across all agents (review round 2,
// item 4), counted as each event's whole JSON payload. 32 MiB lets every
// agent of a large team of 40 hold its full count of typical events of about
// 3 KiB, which covers many minutes of a failing disk, while a long outage
// with large replies cannot grow the service's memory without bound.
export const heldRunnerEventBytesLimit = 32 * 1024 * 1024;

// The owner chose "finish" on 2026-11-23 (issue 251; review round 1 of unit
// f2-lifecycle).
// What a start does when its caller goes away after the start has committed.
//
// "finish" is the current behaviour. The start's setup runs to its end in the
// background, as a committed stop's or resume's process work does. The agent
// opens its session or gets its start failure, and the owner reads the
// outcome from the log.
//
// "fail" would end the setup when the caller goes away and record the start
// as failed. Choosing it takes two edits. First, set this to "fail", so the
// start's committed work runs under the caller's interruption again. Second,
// in start.ts, make an interruption of the setup run the same cleanup as a
// cancelled setup: stop the runner and call `failStart`, under the lane and
// only while `current` holds. Without the second edit, an interrupted start
// is left "starting" for ever, which is issue 251.
export const startWithoutCaller: "finish" | "fail" = "finish";

export const layer = (configuration: Pick<Runtime, "home" | "stateFolder" | "pluginRoot" | "harnessDefaults" | "harnessPrograms" | "credentials"> & { readonly capacityHoldCondition?: import("../routing/deliveries.ts").CapacityHoldCondition }, lists: EngineLists = engineLists) =>
  Layer.effect(CommandHandler, Effect.gen(function* () {
    const sql = yield* SqlClient.SqlClient;
    const log = yield* EventLog;
    const views = yield* Views;
    const runner = yield* Runner;
    const secrets = yield* SecretStore;
    const engineScope = yield* Effect.scope;
    // Project creation and archive own real side effects beyond an HTTP/RPC
    // request's lifetime. Register their complete, serialized operation in the
    // engine scope before allowing caller cancellation. Their normal bounded
    // runner setup and stop pre-emption remain interruptible. Engine shutdown
    // still closes them, and durable intents/receipts cover the next boot.
    const engineOwned = <A, E, R>(operation: Effect.Effect<A, E, R>) => Effect.uninterruptibleMask((restore) => Effect.gen(function* () {
      const fiber = yield* Effect.forkIn(operation, engineScope);
      return yield* restore(Fiber.join(fiber));
    }));
    const runtime: Runtime = { ...configuration, lists, sql, log, runner, secrets, views, scope: engineScope };
    yield* preloadAccountCredentials(runtime, Object.values((yield* views.snapshot).accounts));
    setTerminalSwitch(log, ((yield* views.snapshot).client as { webSettings?: { terminalEnabled?: boolean } } | undefined)?.webSettings?.terminalEnabled === true);
    yield* Effect.addFinalizer(() => Effect.sync(() => closeSignIns(log)));
    // Fix round 1, N-1: the first start loads the harness defaults.
    yield* loadFirstDefaults(runtime);
    yield* recoverInstalls(runtime);
    if (configuration.harnessPrograms?.close) yield* Effect.addFinalizer(() => Effect.promise(() => configuration.harnessPrograms!.close!()));
    // Issue 88 and row 52: while restart recovery is present and has not
    // finished step 9, no command runs.
    const recovery = yield* Effect.serviceOption(Restart);

    // An agent event as it arrived, with the identity of its one write.
    // Review round 2, item 2: the request id, the body hash and every id the
    // write generates are chosen once, here, and reused on every attempt. A
    // write that reported failure but was stored is then found by its
    // receipt, which returns the stored result instead of writing it twice.
    interface Arrival {
      readonly event: RunnerEvent;
      readonly requestId: RequestId;
      readonly bodyHash: string;
      readonly itemId: string;
      readonly bytes: number;
    }

    // Writes one runner event and takes any action that follows its write.
    // Nothing acts on an event before it is written (failure rule 8).
    const consumeRunnerEvent = ({ event, requestId, bodyHash, itemId }: Arrival) => {
      const consume = Effect.gen(function* () {
        let resetMode = false;
        yield* log.transact((append) => Effect.gen(function* () {
          const snapshot = yield* views.snapshot;
          const agent = snapshot.agents[event.agentId];
          if (agent === undefined || !["starting", "working", "idle", "paused"].includes(agent.state._tag)) return;
          const tokens = yield* sql<{ agent_id: string }>`
            SELECT agent_id FROM agent_tokens WHERE token_hash = ${event.incarnation}
            AND agent_id = ${event.agentId} AND revoked_at IS NULL`;
          if (tokens.length === 0) return;
          // Every batch carries the wait checks it causes (rows 25 and 50).
          const write = (events: ReadonlyArray<Event>, sideRows: SideRows = {}) => events.length === 0 ? Effect.void
            : append({ _tag: "owner" }, requestId, bodyHash, withWaitChecks(snapshot, events, lists), sideRows).pipe(Effect.asVoid);
          switch (event._tag) {
            case "configurationReported": return yield* write([{ _tag: "AgentConfigurationObserved", agentId: event.agentId, applied: event.applied }]);
            case "permissionRequested": return yield* write(yield* permissionRequested(snapshot, event));
            case "turnEnded": return yield* write(turnEnded(snapshot, event));
            case "deliveryUpdate": return yield* write(deliveryUpdate(snapshot, event, itemId));
            case "modeReported": {
              resetMode = event.level === undefined;
              return yield* write([event.level === undefined
                ? { _tag: "ModeRefused", agentId: event.agentId, mode: event.mode, ceiling: event.ceiling, reason: "mode exceeds permission ceiling" }
                : { _tag: "ModeChanged", agentId: event.agentId, mode: event.mode, level: event.level }]);
            }
            case "promptReply": return yield* write(yield* promptReply(snapshot, event, log.redactText, configuration.capacityHoldCondition));
            case "exited": {
              return yield* write(crashEvents(snapshot, event, lists),
                { revokeAgentTokens: [event.agentId] });
            }
          }
        }));
        if (event._tag === "modeReported" && resetMode) {
          yield* runner.setMode({ agentId: event.agentId, permission: event.ceiling }).pipe(
            Effect.catchTag("RunnerRejected", () => Effect.logWarning("mode set-back failed")),
          );
        }
      });
      return event._tag === "modeReported" ? withLane(log, event.agentId)(consume) : consume;
    };

    // Review round 1 of unit f1-eventlog, item 4 (issue 1609), and review
    // round 2. Writes can fail and later recover while agents keep running.
    // An agent event whose write is refused because the event log is failing
    // for a storage reason is held in memory, in order, behind any earlier
    // held event of the same agent, and written when writes work again.
    // Nothing acts on a held event until it is written. An event refused for
    // a reason of its own, such as a constraint, is dropped with its fixed
    // reason in the log, as before this unit, so it cannot block the others.
    //
    // Two bounds keep memory finite: `heldRunnerEventLimit` events for one
    // agent, and `heldRunnerEventBytesLimit` bytes of held events across all
    // agents. When either is reached, every held event is dropped, the event
    // log refuses every write until a restart, whose recovery repairs the
    // agents' state, and one line is logged. Later events are dropped
    // without a line.
    const held = new Map<string, Array<Arrival>>();
    let heldEvents = 0;
    let heldBytes = 0;
    let heldLost = false;
    let flushing = false;
    const report = () => log.noteHeld?.({ heldRunnerEvents: heldEvents, heldAgents: held.size });
    const runnerWrites = yield* Semaphore.make(1);
    const outage = (cause: Cause.Cause<unknown>) => cause.reasons.some((reason) => reason._tag === "Fail" && (log.isOutage === undefined
      ? reason.error instanceof LogWriteFailed && !(reason.error instanceof InvalidEvent) && !(reason.error instanceof ReceiptUnreadable)
      : log.isOutage(reason.error)));
    const dropped = Effect.logWarning("runner event dropped: processing failed");
    // Writes one runner event, or reports that the event log refused it
    // because writes are failing. Any other failure drops the event.
    const tryRunnerEvent = (arrival: Arrival) => Effect.gen(function* () {
      const exit = yield* Effect.exit(consumeRunnerEvent(arrival));
      if (Exit.isSuccess(exit)) return true;
      if (Cause.hasInterrupts(exit.cause)) return yield* Effect.failCause(exit.cause);
      if (outage(exit.cause)) return false;
      const refusal = exit.cause.reasons.find((reason) => reason._tag === "Fail" && reason.error instanceof LogWriteFailed);
      yield* refusal?._tag === "Fail"
        ? Effect.logWarning("runner event dropped: write refused").pipe(Effect.annotateLogs({ reason: (refusal.error as LogWriteFailed).reason }))
        : dropped;
      return true;
    });
    // Writes one agent's held events in order, and stops at the first one
    // that still cannot be written. Runs under `runnerWrites`.
    const flushAgent = (agentId: string) => Effect.gen(function* () {
      const queue = held.get(agentId);
      while (queue !== undefined && queue.length > 0) {
        if (!(yield* tryRunnerEvent(queue[0]!))) return;
        const written = queue.shift()!;
        heldEvents -= 1;
        heldBytes -= written.bytes;
        report();
      }
      held.delete(agentId);
      report();
    });
    const flushLoop = Effect.gen(function* () {
      while (true) {
        yield* Effect.sleep(firstRetryMillis);
        const more = yield* runnerWrites.withPermits(1)(Effect.gen(function* () {
          for (const agentId of [...held.keys()]) yield* flushAgent(agentId);
          if (held.size === 0 || heldLost) { flushing = false; return false; }
          return true;
        }));
        if (!more) return;
      }
    }).pipe(Effect.catchCause((cause) => Cause.hasInterrupts(cause) ? Effect.failCause(cause)
      : Effect.sync(() => { flushing = false; }).pipe(Effect.andThen(dropped))));
    // Runs under `runnerWrites`.
    const hold = (arrival: Arrival) => Effect.gen(function* () {
      if (heldLost) return;
      const queue = held.get(arrival.event.agentId) ?? [];
      if (queue.length >= heldRunnerEventLimit || heldBytes + arrival.bytes > heldRunnerEventBytesLimit) {
        heldLost = true;
        held.clear();
        heldEvents = 0;
        heldBytes = 0;
        report();
        yield* log.requireRestart?.("agent events were dropped while writes failed") ?? Effect.void;
        return yield* Effect.logError("runner events dropped: writes failed too long");
      }
      // One line when an agent's first event is held, not one per event.
      if (queue.length === 0) yield* Effect.logWarning("runner event held: database write failed");
      queue.push(arrival);
      held.set(arrival.event.agentId, queue);
      heldEvents += 1;
      heldBytes += arrival.bytes;
      report();
      if (!flushing) {
        flushing = true;
        yield* Effect.forkIn(flushLoop, engineScope);
      }
    });

    const handleRunnerEvent = (rawEvent: unknown) => Effect.gen(function* () {
      // A bad adapter event cannot reach the views or the writer, and its
      // validation error must not expose any of the input it describes.
      const decoded = yield* Schema.decodeUnknownEffect(RunnerEvent)(rawEvent).pipe(Effect.result);
      if (decoded._tag === "Failure") return yield* Effect.logWarning("runner event dropped: malformed");
      const event = decoded.success;
      // Temporary discovery sessions have no roster entry or registered token.
      // Drop their notices before acquiring either the writer or an agent lane.
      if ((yield* views.snapshot).agents[event.agentId] === undefined) return;
      // After the bound was reached, writes are refused until a restart, and
      // the one line about it has been logged.
      if (heldLost) return;
      const arrival: Arrival = {
        event, requestId: freshRequest(), bodyHash: randomUUID(), itemId: randomUUID(),
        bytes: Buffer.byteLength(JSON.stringify(event)),
      };
      yield* runnerWrites.withPermits(1)(Effect.gen(function* () {
        // An event never overtakes an earlier held event of the same agent.
        // The earlier ones are tried first (review round 2, item 3), so an
        // agent at the bound whose next event arrives after writes recovered
        // loses nothing.
        if (held.has(event.agentId)) {
          yield* flushAgent(event.agentId);
          if (held.has(event.agentId)) return yield* hold(arrival);
        }
        if (!(yield* tryRunnerEvent(arrival))) yield* hold(arrival);
      }));
    // A failed event is isolated, but scope closure must still stop the stream.
    }).pipe(Effect.catchCause((cause) => Cause.hasInterrupts(cause)
      ? Effect.failCause(cause)
      : dropped));
    yield* runner.events.pipe(Stream.runForEach(handleRunnerEvent), Effect.forkScoped);
    yield* dispatchCommitted;
    yield* followAvailability;

    // Fix round 1, F3: a command that changes a target's process holds the
    // target's send lane from before its writer transaction through its
    // runner call. The lane always comes before the writer (lanes.ts). The
    // payload is not decoded yet, so only a string id picks a lane; a lane
    // for a bad id is harmless.
    const laneFor = (name: string, rawPayload: unknown): string | undefined => {
      if (projectLifecycleCommand(name) || name === "project.creation.abandon") return "project-lifecycle";
      if ((name !== "stop" && name !== "resume" && name !== "cancel-reply" && name !== "configure") || typeof rawPayload !== "object" || rawPayload === null) return undefined;
      const agentId = (rawPayload as { readonly agentId?: unknown }).agentId;
      return typeof agentId === "string" ? agentId : undefined;
    };
    // Fix round 1, A3: before `stop` queues for the lane, it signals a start of
    // that agent whose setup is still running, or (issue 265) a send waiting
    // for the adapter to read it (lanes.ts), so the lane frees at once. The signal is sent only for a stop this caller could commit: the
    // credential, the payload, the agent's state and the holder are checked
    // here as the transaction checks them. The checks cannot see every refusal
    // (a reused request id, a writer failure), so the signal is not trusted
    // to mean the stop commits: the result is the step that tells the start
    // how the stop ended, and the start finishes the agent itself when the
    // stop did not (lanes.ts, round 2 F-1).
    const resolveCredential = (credential: Credential, name: string): Effect.Effect<Exclude<Caller, { _tag: "piece" }>, CommandRefused> => Effect.gen(function* () {
      if (credential._tag === "owner") return { _tag: "owner" } as const;
      if (credential._tag === "service") {
        if (!Object.hasOwn(lists.services, credential.service) || !lists.services[credential.service]?.includes(name)
          || (pieceNamespace(name) === undefined && !["prompt", "compact", "resume", "release", "subscribe"].includes(name))) return yield* refused();
        return credential;
      }
      const agentId = yield* verifyAgentToken(Redacted.value(credential.token)).pipe(Effect.mapError(() => refused("invalid credential")));
      const current = (yield* views.snapshot).agents[agentId];
      if (current === undefined || current.state._tag === "stopped" || current.state._tag === "crashed") return yield* refused("invalid credential");
      return { _tag: "agent", agentId } as const;
    }).pipe(Effect.provideService(SqlClient.SqlClient, sql));

    const preemptStart = (credential: Credential, name: string, rawPayload: unknown) => Effect.gen(function* () {
      const agentId = name === "stop" ? laneFor(name, rawPayload) : undefined;
      const rpc = lists.commands.requests.get("stop");
      if (agentId === undefined || rpc === undefined) return Effect.void;
      yield* Effect.try({
        try: () => Schema.decodeUnknownSync(Schema.toType(rpc.payloadSchema))(rawPayload),
        catch: () => refused(),
      });
      const caller = yield* resolveCredential(credential, name);
      const snapshot = yield* views.snapshot;
      const target = snapshot.agents[agentId];
      // Issue 265: a send that holds the lane is pre-empted too, in any
      // state a stop can end, not only a start's setup.
      if (target === undefined || commandOutcome(target.state._tag, "stop") === "refused") return Effect.void;
      if (caller._tag === "agent") {
        const current = snapshot.agents[caller.agentId];
        const holder = holderAbove(snapshot, agentId);
        if (current === undefined || current.state._tag === "stopped" || current.state._tag === "crashed"
          || holder._tag !== "agent" || holder.agentId !== caller.agentId) return Effect.void;
      }
      return yield* preempt(log, agentId);
    }).pipe(Effect.orElseSucceed((): Effect.Effect<void> => Effect.void), Effect.provideService(SqlClient.SqlClient, sql));
    // Issue 77: a blocking wait commits its opening and registers its
    // cancellation in one uninterruptible step (waits.ts, `block`). Every
    // other command stays interruptible throughout, as before.
    const isBlockingWait = (name: string, rawPayload: unknown) => name === "wait" && typeof rawPayload === "object"
      && rawPayload !== null && (rawPayload as { readonly block?: unknown }).block === true;

    // Direct callers use the same read table as the transport. A read always
    // rechecks current credentials and access. A Git state refresh may record
    // terminal uncertainty receipts for existing pending mutations.
    const executeRead = (credential: Credential, name: string, rawPayload: unknown) => Effect.gen(function* () {
      if (Option.isSome(recovery) && !(yield* recovery.value.ready)) return yield* refused("recovering");
      const actor = yield* resolveCredential(credential, name);
      const rpc = lists.commands.requests.get(name);
      if (rpc === undefined || RpcSchema.isStreamSchema(rpc.successSchema) || !lists.access[name]?.includes(actor._tag)) return yield* refused(name.startsWith("client.harness.signin.") ? "Only the owner can run a sign-in session." : "command refused");
      const payload = yield* Effect.try({
        try: () => Schema.decodeUnknownSync(Schema.toType(rpc.payloadSchema))(rawPayload) as Record<string, unknown>,
        catch: () => refused(),
      });
      const snapshot = yield* views.snapshot;
      const input = yield* prepareRepositoryBases({ runtime, actor, snapshot }, name, payload);
      const invoke = (current: Input) => lists.reads[name]!(current, payload);
      const read = name === "project.git.state" ? refreshGitState(input, payload, (current) =>
        invoke(current) as Effect.Effect<import("../api/reads.ts").ReadReply, unknown, EventLog | Views | SqlClient.SqlClient>) : invoke(input);
      if (!Effect.isEffect(read)) return yield* refused();
      const result = yield* (read as Effect.Effect<import("../api/reads.ts").ReadReply, unknown, EventLog | Views | SqlClient.SqlClient>);
      const marks = result.marks ?? [];
      if (marks.length > 0 || (result.deliveries?.length ?? 0) > 0 || Object.keys(result.sideRows ?? {}).length > 0) {
        const namespace = pieceNamespace(name);
        if (namespace === undefined) return yield* refused();
        yield* log.transact((append) => Effect.gen(function* () {
          const current = yield* views.snapshot;
          const events = yield* checkPieceBatch(namespace, current, { events: marks, deliveries: result.deliveries, sideRows: result.sideRows }, actor, lists);
          yield* append(actor, freshRequest(), "read marks", events, result.sideRows ?? {}, false);
        }));
      }
      return result.reply;
    }).pipe(
      Effect.provideService(EventLog, log), Effect.provideService(Views, views), Effect.provideService(SqlClient.SqlClient, sql),
      Effect.mapError((error) => error instanceof CommandRefused || error instanceof LogWriteFailed ? error : refused()),
    );

    const requestLane = <A, E, R>(credential: Credential, name: string, raw: unknown, operation: Effect.Effect<A, E, R>) => Effect.gen(function* () {
      const actor = yield* resolveCredential(credential, name);
      const id = typeof raw === "object" && raw !== null ? (raw as { requestId?: unknown }).requestId : undefined;
      if (typeof id !== "string") return yield* operation;
      return yield* withLane(log, JSON.stringify(["request", callerKey(actor), id]))(Effect.gen(function* () {
        const pending = own(projectView(yield* views.snapshot).gitAttempts, projectKey(callerKey(actor), id));
        if (pending !== undefined) {
          const rpc = lists.commands.requests.get(name);
          if (rpc === undefined || pending.command !== name) return yield* refused("request id reused");
          const payload = yield* Effect.try({
            try: () => Schema.decodeUnknownSync(Schema.toType(rpc.payloadSchema))(raw) as Record<string, unknown>,
            catch: () => refused("request id reused"),
          });
          const { requestId: _requestId, ...body } = payload;
          if (pending.bodyHash !== sha256Hex(canonical({ command: name, payload: body }))) return yield* refused("request id reused");
        }
        return yield* operation;
      }));
    }).pipe(Effect.provideService(SqlClient.SqlClient, sql));

    type Step = <A, E, R>(effect: Effect.Effect<A, E, R>) => Effect.Effect<A, E, R>;
    const executeMutation: CommandHandler["Service"]["execute"] = (credential, name, rawPayload) => Effect.suspend(() => {
      let releaseStartWorkspace: Effect.Effect<void> = Effect.void;
      // Issue 251: a committed start, stop or resume owes the process its
      // runner work. The whole command runs in the engine's scope, and the
      // caller only waits for it, so a caller that goes away passes its
      // interruption on and returns at once. Before the commit that cancels
      // the command. After it, the work is recorded with its commit and runs
      // to its end, inside the command's lane, whether the interruption came
      // during the work or between the commit and the work. Its own setup
      // bound and a stop's pre-emption still end it; engine shutdown
      // interrupts it. A start's or resume's caller that left gets no reply,
      // and the owner reads the outcome from the log. Whether a start does
      // this is the owner's pending choice, `startWithoutCaller` above.
      const ownsProcessWork = name === "stop" || name === "resume" || (name === "start" && startWithoutCaller === "finish");
      let committedWork: Plan["afterCommit"];
      const runCommitted = (work: NonNullable<Plan["afterCommit"]>) =>
        Effect.uninterruptible(Effect.forkIn(work, engineScope).pipe(Effect.flatMap(Fiber.join)));
      const leftover = Effect.suspend(() => {
        const work = committedWork;
        committedWork = undefined;
        return work === undefined ? Effect.void : runCommitted(work).pipe(Effect.catchCause((cause) => Cause.hasInterruptsOnly(cause)
          ? Effect.void : Effect.logWarning("committed process work failed")));
      });
      const prepare = (step: Step) => Effect.gen(function* () {
        if (serviceRestartPending() && !["stop", "client.service.restart"].includes(name)) return yield* refused("service restarting");
        if (Option.isSome(recovery) && !(yield* recovery.value.ready)) return yield* refused("recovering");
        let external: { readonly plan?: Plan; readonly finalize?: Planner; readonly failure?: CommandRefused | LogWriteFailed; readonly finished?: Event } | undefined;
        if (name === "start" || Object.hasOwn(gitPlanners, name) || Object.hasOwn(registrationPlanners, name)
          || ["project.design.record", "project.mocks.record", "project.mocks.approve"].includes(name)
          || (name === "prompt" || name === "board.post") && typeof rawPayload === "object" && rawPayload !== null
            && "context" in rawPayload && Array.isArray(rawPayload.context)
            && rawPayload.context.some((ref: unknown) => typeof ref === "object" && ref !== null && "_tag" in ref && ref._tag === "file")) {
          const prepareGit = Effect.gen(function* () {
            const admitted = yield* log.transact(() => Effect.gen(function* () {
              const actor = yield* resolveCredential(credential, name);
              const rpc = lists.commands.requests.get(name);
              if (rpc === undefined) return yield* refused();
              const payload = yield* Effect.try({ try: () => Schema.decodeUnknownSync(Schema.toType(rpc.payloadSchema))(rawPayload) as Record<string, unknown>, catch: () => refused() });
              const { requestId, ...body } = payload;
              if (typeof requestId !== "string" || (yield* log.redactText(requestId)) !== requestId) return yield* refused();
              const bodyHash = sha256Hex(canonical({ command: name, payload: body }));
              const caller = callerKey(actor);
              const receipts = yield* sql<{ body_hash: string }>`SELECT body_hash FROM receipts WHERE caller = ${caller} AND request_id = ${requestId}`;
              if (receipts[0] !== undefined) {
                if (receipts[0].body_hash !== bodyHash) return yield* refused("request id reused");
                return undefined;
              }
              if (!lists.access[name]?.includes(actor._tag)) return yield* refused();
              if (name.startsWith("board.") && lists.boardEnabled === false) return yield* refused("board disabled");
              return { actor, payload, bodyHash, snapshot: yield* views.snapshot };
            }));
            if (admitted === undefined) return undefined;
            const resolved = yield* prepareRepositoryBases({ runtime, actor: admitted.actor, snapshot: admitted.snapshot }, name, admitted.payload);
            admitted.snapshot = resolved.snapshot;
            if (name === "start") return { finalize: yield* prepareStartWorkspace(
              { runtime, actor: admitted.actor, snapshot: admitted.snapshot }, admitted.payload,
              (release) => { releaseStartWorkspace = release; }) };
            if (Object.hasOwn(registrationPlanners, name)) return { finalize: yield* prepareRegistration(
              { runtime, actor: admitted.actor, snapshot: admitted.snapshot }, name as keyof typeof registrationPlanners, admitted.payload) };
            const input = { runtime, actor: admitted.actor, snapshot: admitted.snapshot };
            if (name === "project.design.record") return { finalize: yield* prepareDesignRecord(input, admitted.payload) };
            if (name === "project.mocks.record" || name === "project.mocks.approve") return { finalize: yield* prepareMock(input, name, admitted.payload) };
            if (name === "prompt" || name === "board.post") {
              const files = yield* prepareContextFiles(input, admitted.payload.context as import("@lgtm/contracts").ContextReference[] | undefined);
              return { finalize: name === "prompt" ? promptPlanner(files) : postPlanner(files) };
            }
            const operation = yield* planGitOperation({ runtime, actor: admitted.actor, snapshot: admitted.snapshot }, name, admitted.payload,
              admitted.bodyHash, gitPlanners[name as keyof typeof gitPlanners]);
            return name === "project.mocks.editor.save" && operation.plan !== undefined
              ? { finalize: finalizeMockEditorSave(input, admitted.payload, operation.plan) } : operation;
          }).pipe(
            Effect.catchTag("RequestIdReused", () => Effect.fail(refused("request id reused"))),
            Effect.catchTag("SqlError", () => Effect.fail(new LogWriteFailed({ reason: "database read failed" }))),
            Effect.result,
          );
          const result = yield* step(prepareGit);
          if (result._tag === "Failure") {
            if (result.failure instanceof CommandUncertain || result.failure._tag === "LogWriteFailed") return yield* result.failure;
            external = { failure: result.failure };
          } else external = result.success;
        }
        let finalizeLifecycle: Planner | undefined;
        if (projectLifecycleCommand(name)) finalizeLifecycle = yield* step(Effect.gen(function* () {
          const actor = yield* resolveCredential(credential, name);
          const rpc = lists.commands.requests.get(name);
          if (actor._tag !== "owner" || rpc === undefined || !lists.access[name]?.includes(actor._tag)) return yield* refused();
          const payload = yield* Effect.try({ try: () => Schema.decodeUnknownSync(Schema.toType(rpc.payloadSchema))(rawPayload) as Record<string, unknown>, catch: () => refused() });
          const { requestId, ...body } = payload;
          if (typeof requestId !== "string" || (yield* log.redactText(requestId)) !== requestId) return yield* refused();
          const bodyHash = sha256Hex(canonical({ command: name, payload: body }));
          const receipts = yield* sql<{ body_hash: string }>`SELECT body_hash FROM receipts WHERE caller = 'owner' AND request_id = ${requestId}`;
          if (receipts[0] !== undefined) {
            if (receipts[0].body_hash !== bodyHash) return yield* refused("request id reused");
            return;
          }
          return yield* prepareProjectLifecycle(yield* prepareRepositoryBases({ runtime, actor, snapshot: yield* views.snapshot }, name, payload), name, payload, bodyHash);
        }));
        return yield* step(log.transact((append) => Effect.gen(function* () {
          if (serviceRestartPending() && !["stop", "client.service.restart"].includes(name)) return yield* refused("service restarting");
          // Invalid credentials never write an event or retrieve a receipt.
          const actor = yield* resolveCredential(credential, name);
          const rpc = lists.commands.requests.get(name);
          if (rpc === undefined || !Object.hasOwn(lists.planners, name)) return yield* refused();
          const payload = yield* Effect.try({
            try: () => {
              // The transport supplies decoded values. Normalize the wrapper's
              // label before validating it; the label is not secret data.
              let value = rawPayload;
              if (lists.secretCommands.includes(name) && typeof value === "object" && value !== null
                && "value" in value && Redacted.isRedacted(value.value)) {
                value = { ...value, value: Redacted.make(Redacted.value(value.value), { label: "secret value" }) };
              }
              return Schema.decodeUnknownSync(Schema.toType(rpc.payloadSchema))(value) as Record<string, unknown>;
            },
            catch: (error) => name === "roles.settings.set" ? settingsRefusal(error)
              : name === "client.web.settings.set" ? refused(`Invalid web settings. ${publicAddressError}`) : refused(),
          });
          const { requestId: id, ...body } = payload;
          const requestId = id as RequestId;
          // The rollback switch also refuses requests with an old receipt.
          if (name.startsWith("board.") && lists.boardEnabled === false) return yield* refused("board disabled");
          // Identifiers bypass event-text redaction. Reject a secret-bearing
          // id before looking up a receipt or planning even a refusal event.
          if ((yield* log.redactText(requestId)) !== requestId) return yield* refused();
          const bodyHash = sha256Hex(canonical(lists.secretCommands.includes(name)
            ? { command: name, payload: Object.fromEntries(Object.entries(body).filter(([key]) => key !== "value")) }
            : { command: name, payload: body }));
          const caller = callerKey(actor);
          const receipts = yield* sql<{ body_hash: string; result_json: string }>`
            SELECT body_hash, result_json FROM receipts WHERE caller = ${caller} AND request_id = ${requestId}`;
          const snapshot = yield* views.snapshot;
          const settings = name === "harnesses" ? yield* readHarnesses(runtime) : [];
          const decodeReceipt = (resultJson: string) => Effect.try({
            try: () => {
              const data: unknown = JSON.parse(resultJson);
              if (Array.isArray(data)) {
                const envelopes = Schema.decodeUnknownSync(Schema.Array(lists.envelope))(data);
                return replyFor(snapshot, name, requestId, envelopes as ReadonlyArray<EventEnvelope>, settings);
              }
              const result = Schema.decodeUnknownSync(Schema.Struct({ envelopes: Schema.Array(lists.envelope), reply: Schema.Unknown }))(data);
              return { reply: Schema.decodeUnknownSync(Schema.toType(rpc.successSchema))(result.reply) as Record<string, unknown> };
            },
            catch: () => new ReceiptUnreadable(),
            });
          const receipt = receipts[0];
          if (receipt !== undefined) {
            if (receipt.body_hash !== bodyHash) return yield* refused("request id reused");
            const stored = yield* decodeReceipt(receipt.result_json);
            return { ...stored, afterCommit: undefined };
          }
          const planned = yield* Effect.gen(function* () {
            if (!lists.access[name]?.includes(actor._tag)) return yield* refused();
            if (external?.failure !== undefined) return yield* external.failure;
            const rawPlan = external?.plan ?? (yield* (external?.finalize ?? finalizeLifecycle ?? lists.planners[name]!)({ runtime, actor, snapshot }, payload));
            const plan = { ...rawPlan, events: withProjectProgress(snapshot, rawPlan.events) };
            if (name === "project.review.decline" || name === "project.mocks.comment") {
              // The named composition appends exactly one project decision after
              // the complete board plan, including unprefixed withdrawals.
              if (plan.afterCommit !== undefined) return yield* refused();
              const decision = plan.events.at(-1);
              if (decision?._tag !== (name === "project.review.decline" ? "project.MainReviewDecided" : "project.MockScreenCommented")) return yield* refused();
              const board = yield* checkPieceBatch("board", snapshot, { events: plan.events.slice(0, -1), deliveries: plan.deliveries, sideRows: plan.sideRows }, actor, lists);
              const project = yield* checkPieceBatch("project", snapshot, { events: [decision] }, actor, lists);
              return { ...plan, events: [...board, ...project] };
            }
            const namespace = pieceNamespace(name);
            if (namespace !== undefined && !projectLifecycleCommand(name)) {
              if (plan.afterCommit !== undefined) return yield* refused();
              const events = yield* checkPieceBatch(namespace, snapshot, plan, actor, lists);
              if (name === "project.permission.set" || name === "project.approval.withdraw") {
                const closed = yield* closeChangedPermissionQuestions({ runtime, actor, snapshot }, plan.events, requestId);
                return { ...plan, events: withWaitChecks(snapshot, [...events, ...closed], lists) };
              }
              return { ...plan, events };
            }
            return plan;
          }).pipe(Effect.result);
          if (planned._tag === "Failure") {
            if (planned.failure._tag === "LogWriteFailed") return yield* planned.failure;
            const failure = planned.failure;
            if (external?.plan !== undefined || failure instanceof CommandUncertain) return yield* new CommandUncertain({ reason: name === "project.mocks.editor.save" ? mockEditorUncertainReason : "Git outcome is uncertain; refresh Git state" });
            const initialMainRefusal = name === "project.create" && isCheckoutBaseReason(failure.reason)
              && own(projectView(snapshot).creations, requestId) === undefined;
            yield* append(actor, requestId, bodyHash, [...(external?.finished === undefined ? [] : [external.finished]), { _tag: "CommandRefused", command: name, reason: failure.reason }], {},
              failure.reason === "no usable account" || projectLifecycleCommand(name) && !initialMainRefusal ? false : undefined);
            return { failure, afterCommit: undefined };
          }
          if (serviceRestartPending() && !["stop", "client.service.restart"].includes(name)) return yield* refused("service restarting");
          const plan = planned.success;
          if (plan.events.some(event => event._tag === "AccountChanged")) {
            const accounts: Record<string, import("@lgtm/contracts").Account> = { ...snapshot.accounts };
            for (const event of plan.events) if (event._tag === "AccountChanged") {
              if (event.account === undefined) delete accounts[event.accountId];
              else accounts[event.accountId] = event.account;
            }
            yield* preloadAccountCredentials(runtime, Object.values(accounts));
          }
          yield* append(actor, requestId, bodyHash, (pieceNamespace(name) === undefined || projectLifecycleCommand(name)) ? withWaitChecks(snapshot, plan.events, lists) : plan.events, plan.sideRows ?? {}, (batch) =>
            plan.reply === undefined ? replyFor(snapshot, name, requestId, batch, settings).reply : plan.reply(batch));
          if (ownsProcessWork) committedWork = plan.afterCommit;
          // Return precisely the redacted result that a later retry reads.
          const written = yield* sql<{ result_json: string }>`
            SELECT result_json FROM receipts WHERE caller = ${caller} AND request_id = ${requestId}`;
          if (written[0] === undefined) return yield* new LogWriteFailed({ reason: "receipt insert returned no result" });
          return { ...(yield* decodeReceipt(written[0].result_json)), afterCommit: plan.afterCommit, restartAccepted: name === "client.service.restart" };
        })).pipe(Effect.onExit((exit) => Exit.isFailure(exit) && !Cause.hasInterruptsOnly(exit.cause)
          ? Effect.sync(() => { committedWork = undefined; }) : Effect.void)));
      });
      const finish = (outcome: Effect.Success<ReturnType<typeof prepare>>, step: Step) => Effect.gen(function* () {
        if (outcome.failure !== undefined) return yield* outcome.failure;
        if (name === "client.web.settings.set") { setTerminalSwitch(log, ((yield* views.snapshot).client as { webSettings?: { terminalEnabled?: boolean } } | undefined)?.webSettings?.terminalEnabled === true); yield* webSettingsCommitted(views); }
        if ("restartAccepted" in outcome && outcome.restartAccepted) yield* completeServiceRestart();
        const late = outcome.afterCommit === undefined ? undefined : ownsProcessWork
          ? yield* Effect.suspend(() => { committedWork = undefined; return runCommitted(outcome.afterCommit!); })
          : yield* step(outcome.afterCommit);
        return late ?? outcome.reply!;
      });
      // Process commands keep request -> process ownership through their
      // runner continuation. A blocking wait also retains its original atomic
      // opening/cancellation registration. Other commands release the request
      // after the durable outcome, before a start takes its new agent lane.
      const complete = Effect.uninterruptibleMask((restore) => Effect.gen(function* () {
        const step: Step = isBlockingWait(name, rawPayload) ? (effect) => effect : restore;
        const outcome = yield* prepare(step);
        return yield* finish(outcome, step);
      })).pipe(Effect.onExit(() => leftover));
      const lane = laneFor(name, rawPayload);
      const lifecycleCommand = projectLifecycleCommand(name) || name === "project.creation.abandon";
      const processLane = lifecycleCommand ? undefined : lane;
      // A stop ends its pre-emption before it leaves the lane, so no command
      // that takes the lane next can see the stop's signal still raised.
      let settleStop: Effect.Effect<void> = Effect.void;
      const operation = processLane !== undefined
        ? requestLane(credential, name, rawPayload, withLane(log, processLane)(complete.pipe(Effect.ensuring(Effect.suspend(() => settleStop)))))
        : isBlockingWait(name, rawPayload)
          ? requestLane(credential, name, rawPayload, complete)
          : Effect.uninterruptibleMask((restore) => Effect.gen(function* () {
            const outcome = yield* restore(requestLane(credential, name, rawPayload, Effect.uninterruptibleMask(prepare)));
            return yield* finish(outcome, restore);
          })).pipe(Effect.onExit(() => leftover));
      const settled = Effect.acquireUseRelease(preemptStart(credential, name, rawPayload).pipe(Effect.tap((settle) => Effect.sync(() => { settleStop = settle; }))), () => operation.pipe(
        Effect.provideService(SqlClient.SqlClient, sql),
        Effect.catchTag("RequestIdReused", () => Effect.fail(refused("request id reused"))),
        Effect.catchTag("LogWriteFailed", (error) => Effect.fail(error instanceof InvalidEvent ? refused(error.reason) : error)),
        Effect.catchTag("SqlError", () => Effect.fail(new LogWriteFailed({ reason: "database read failed" }))),
      ), (settle) => settle);
      const cleaned = name === "start" ? settled.pipe(Effect.ensuring(Effect.suspend(() => releaseStartWorkspace))) : settled;
      if (ownsProcessWork) return Effect.uninterruptibleMask((restore) => Effect.gen(function* () {
        const fiber = yield* Effect.forkIn(cleaned, engineScope);
        return yield* restore(Fiber.join(fiber)).pipe(Effect.onInterrupt(() => Effect.sync(() => fiber.interruptUnsafe())));
      }));
      const recoverable = name === "project.mocks.editor.save" && credential._tag === "owner"
        ? cleaned.pipe(Effect.onExit(() => settleMockEditorSave(runtime, rawPayload))) : cleaned;
      // Lifecycle ownership precedes requests, and requests precede process
      // lanes (including archive members). Runner/recovery paths take only
      // process lanes and short writer transactions, never request lanes.
      if (name === "client.service.restart") return engineOwned(withLane(log, "service-restart")(settled));
      if (lifecycleCommand) {
        const lifecycle = withLane(log, "project-lifecycle")(recoverable);
        return projectLifecycleCommand(name) ? engineOwned(lifecycle) : lifecycle;
      }
      return Object.hasOwn(gitPlanners, name) || name === "project.review.decline"
        ? withLane(log, `git-operation:${typeof rawPayload === "object" && rawPayload !== null ? String((rawPayload as { worktreeId?: unknown }).worktreeId) : "invalid"}`)(recoverable)
        : recoverable;
    });

    return CommandHandler.of({
      cache: (target) => Effect.gen(function* () {
        if (Option.isSome(recovery) && !(yield* recovery.value.ready)) return yield* refused("recovering");
        return yield* acceptCache(runtime, lists, target);
      }),
      input: (actor) => views.snapshot.pipe(Effect.map((snapshot) => ({ runtime, actor, snapshot }))),
      execute: (credential, name, rawPayload) => Object.hasOwn(lists.reads, name) ? executeRead(credential, name, rawPayload)
        : executeMutation(credential, name, rawPayload),
    });
  }));
