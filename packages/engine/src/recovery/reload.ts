// Pattern: T3 Code packages/effect-acp/src/client.ts (`loadSession`, line 91;
// MIT licence; the copy here is third_party/t3code/effect-acp/src/client.ts):
// a session is reloaded through whichever method the peer advertised. The
// shape was reused, no code.
import { composeInstructions } from "../roles/compose.ts";
import { randomUUID } from "node:crypto";
import { PermissionLevel, type Caller, type AgentId, type DeliveryId, type Event } from "@lgtm/contracts";
import type * as Context from "effect/Context";
import * as Effect from "effect/Effect";
import * as Option from "effect/Option";
import type * as Redacted from "effect/Redacted";
import * as Schema from "effect/Schema";

import { projectFolderScope } from "../project/folderScope.ts";
import { checkFolder, folderForSpawn } from "../identity/ceiling.ts";
import type { mintAgentToken } from "../identity/tokens.ts";
import type { LogWriteFailed } from "../log/EventLog.ts";
import { freshDelivery } from "../routing/holders.ts";
import { withWaitChecks } from "../routing/waits.ts";
import { RunnerRejected, type Runner, type SessionInfo } from "../runner/Runner.ts";
import {
  freshRequest, readHarnesses, refused, type Agent, type CommandRefused, type Runtime, type Snapshot,
} from "../commands/internal.ts";
import { projectPreventsRun } from "../project/access.ts";
import { carryOnText } from "../accounts/available.ts";
import { replayedDelivery, userRuns } from "./replay.ts";
import { owedAfter } from "../roles/instructionObligation.ts";

type Spawn = Parameters<Context.Service.Shape<typeof Runner>["start"]>[0];
type Token = ReturnType<typeof mintAgentToken>;

// Issue 103: no adapter setup may hold recovery, a return or a resume for
// ever. A setup that has not finished in time is stopped through the runner.
export const SETUP_TIMEOUT_MILLIS = 120_000;

const RESTART_CARRY_ON = "The engine restarted. Carry on with what you were doing.";

// What the runner needs to start or reload an agent, checked again as for a
// spawn (rows 56 and 65). Any refusal is the fixed "command refused".
export const spawnInput = (
  runtime: Runtime, snapshot: Snapshot, agent: Agent, token: Redacted.Redacted<string>, operation: "resume" | "reload",
  mode: "start" | "load" = agent.session === undefined ? "start" : "load",
): Effect.Effect<Spawn, CommandRefused | LogWriteFailed> => Effect.gen(function* () {
  if (projectPreventsRun(snapshot, agent.agentId)) return yield* refused("project archive is unfinished because an agent could not stop");
  const parent = agent.parent._tag === "agent" ? snapshot.agents[agent.parent.agentId] : undefined;
  if (agent.parent._tag === "agent" && parent === undefined) return yield* refused();
  const grant = yield* checkFolder({ ...runtime, ...projectFolderScope(runtime, parent?.agentId), caller: agent.parent,
    parentFolder: parent?.folder ?? agent.folder, folder: agent.folder,
  }).pipe(Effect.mapError((error) => refused(`Agent folder unavailable: ${error.reason}`)));
  if (grant.realPath !== agent.folder) return yield* refused();
  const folder = yield* folderForSpawn(grant, operation).pipe(Effect.mapError(() => refused()));
  const harness = (yield* readHarnesses(runtime)).find((settings) => settings.id === agent.harness);
  const account = snapshot.accounts[agent.account];
  if (harness === undefined || account === undefined) return yield* refused();
  // An owner choice can use that session's advertised values, which can be
  // wider than the optional configured launch lists. Open the
  // replacement session first and report its actual settings. The existing
  // configuration/send lane then revalidates and acknowledges the durable
  // desired choice before releasing work. Unchanged initial settings retain
  // the runner's recorded-list admission and older-protocol start calls.
  const launchChoice = agent.configuration?.revision === undefined ? {
    ...(agent.model === undefined ? {} : { model: agent.model }), ...(agent.effort === undefined ? {} : { effort: agent.effort }),
  } : {};
  const instructions = mode === "start" ? yield* composeInstructions(runtime, snapshot, agent.role, undefined, agent.folder) : undefined;
  return { agentId: agent.agentId, token, folder, grant, harness, account,
    ...(instructions === undefined ? { restoreInstructions: () => runtime.views.snapshot.pipe(
      Effect.flatMap((current) => composeInstructions(runtime, current, agent.role, undefined, agent.folder)),
      Effect.mapError(() => new RunnerRejected({ reason: "Role instructions could not be read." })),
    ) } : { instructions }),
    ...(mode === "load" && instructionsOwed(snapshot, agent.agentId) ? { instructionsOwed: true } : {}),
    permission: Schema.decodeUnknownSync(PermissionLevel)(agent.permission), ...launchChoice,
  };
});

// Issue 1235: whether the agent's current conversation still owes its role
// instructions, by the rule the runner uses (roles/instructionObligation.ts),
// read from the log. A session reference that changes opens a new
// conversation; a load keeps the reference, so it keeps the obligation.
//
// The log has no "turn succeeded" event, so this reads one from what the
// engine wrote for the turn. A turn ended without being stopped succeeded,
// unless its delivery was put back, held for capacity or returned after it
// was sent: the runner counts those turns as failed, while the engine can
// still write their turn end. The log does not record whether a turn carried
// the instructions either. A turn sent while they were owed carried them,
// except when the runner could not restore them after a compaction, which
// the log cannot see; such a session is treated as served.
export const instructionsOwed = (snapshot: Snapshot, agentId: AgentId): boolean => {
  let owed = owedAfter(false, { _tag: "conversationOpened" });
  let session: string | undefined;
  let turn: { readonly deliveryId: string; failed: boolean } | undefined;
  for (const { event } of snapshot.threads[agentId] ?? []) {
    switch (event._tag) {
      case "SessionOpened": {
        const opened = JSON.stringify(event.session);
        if (opened !== session) owed = owedAfter(owed, { _tag: "conversationOpened" });
        session = opened;
        break;
      }
      case "DeliverySent": turn = { deliveryId: event.deliveryId, failed: false }; break;
      case "DeliveryRequeued":
      case "DeliveryHeld":
      case "DeliveryReturned": if (turn?.deliveryId === event.deliveryId) turn.failed = true; break;
      case "TurnEnded":
        if (event.stopped !== true && turn?.failed !== true) owed = owedAfter(owed, { _tag: "turnSucceeded", compact: event.compact, carried: owed });
        turn = undefined;
        break;
    }
  }
  return owed;
};

// Bounded setup with cleanup (issue 103). The caller holds the agent's send
// lane, which serializes setups for one agent. A timeout stops the process
// through the runner and fails with `RunnerRejected`.
export const bounded = (runtime: Runtime, agentId: AgentId, timeoutMillis: number, setup: Effect.Effect<SessionInfo, RunnerRejected>) =>
  setup.pipe(
    Effect.timeoutOption(timeoutMillis),
    Effect.flatMap((result) => Option.isSome(result) ? Effect.succeed(result.value)
      : runtime.runner.stop({ agentId }).pipe(
        Effect.catchTag("RunnerRejected", () => Effect.void),
        Effect.andThen(Effect.fail(new RunnerRejected({ reason: "adapter setup timed out", startFailure: "timed-out" }))),
      )),
  );

// Issue 252: a wait notice that a crash withdrew before the waiter read it.
// The notice keeps its wait as its source, and the crash's withdrawal names
// the crash, so a successful reload puts it back in the queue. Only notices
// withdrawn since the agent last stopped count, so a stop still cancels what
// the waiter had not read, and a notice that is put back and then read is no
// longer withdrawn, so it is never repeated.
//
// Review round 1 of unit f2-lifecycle: a crash also withdraws a notice that
// was already written to the adapter. `sent` says so; `reloadedEvents` puts
// such a notice back only when the loaded conversation does not show it.
export interface WithdrawnNotice {
  readonly deliveryId: DeliveryId;
  readonly sent: boolean;
}
export const crashWithdrawnNotices = (snapshot: Snapshot, agentId: AgentId): ReadonlyArray<WithdrawnNotice> => {
  const withdrawn = new Map<DeliveryId, boolean>();
  const written = new Set<string>();
  for (const { event } of snapshot.threads[agentId] ?? []) {
    if (event._tag === "AgentStopped") withdrawn.clear();
    else if (event._tag === "DeliverySent") written.add(event.deliveryId);
    else if (event._tag === "DeliveryRequeued") written.delete(event.deliveryId);
    else if (event._tag === "DeliveryWithdrawn" && event.reason === "agent crashed") withdrawn.set(event.deliveryId, written.has(event.deliveryId));
  }
  return [...withdrawn].map(([deliveryId, sent]) => ({ deliveryId, sent })).filter(({ deliveryId: id }) => {
    const row = snapshot.deliveries[id];
    if (row?.target !== agentId || row.status !== "withdrawn" || row.source?._tag !== "wait") return false;
    const wait = snapshot.waits[row.source.id];
    return wait?.agentId === agentId && wait.status !== "cancelled";
  });
};

// Row 66: what one successful reload commits, in one transaction with the
// session. Each delivery that was sent but not accepted is either accepted,
// when the replayed user content names it, or put back at the head of the
// queue to be sent again. A carry-on goes to an agent that was mid-turn, or
// whose replay showed its delivery accepted, and has nothing left to send
// again; an earlier run's queued carry-on is kept rather than repeated. The
// agent is working when it has something to send again or carry on, so it
// does not count as settled at step 9; otherwise it is idle.
export const reloadedEvents = (
  snapshot: Snapshot, agentId: AgentId, session: SessionInfo, midTurn: boolean, carryText: string,
): ReadonlyArray<Event> => {
  const runs = userRuns(session.replay ?? []);
  const agent = snapshot.agents[agentId]!;
  const rows = Object.values(snapshot.deliveries).filter((row) => row.target === agentId);
  const sent = rows.filter((row) => row.status === "sent");
  const found = sent.filter((row) => replayedDelivery(runs, row.id));
  const unaccepted = sent.filter((row) => !found.includes(row));
  const requeued = rows.filter((row) => row.status === "queued" && row.reason !== undefined && row.content._tag !== "carry-on");
  const carried = rows.some((row) => row.status === "queued" && row.content._tag === "carry-on");
  // A withdrawn notice the loaded conversation already shows is not sent
  // again; like a found delivery, it earns a carry-on instead.
  const notices = crashWithdrawnNotices(snapshot, agentId);
  const shown = notices.filter((notice) => notice.sent && replayedDelivery(runs, notice.deliveryId));
  const restored = notices.filter((notice) => !shown.includes(notice)).map((notice) => notice.deliveryId);
  const retry = !agent.capacityHeld && (unaccepted.length > 0 || requeued.length > 0 || restored.length > 0);
  const carry = !agent.capacityHeld && !retry && !carried && (found.length > 0 || shown.length > 0 || midTurn);
  const id = freshDelivery();
  return [
    ...(agent.state._tag === "starting" ? [] : [{ _tag: "StateChanged" as const, agentId, state: { _tag: "starting" as const } }]),
    // Named fields only: the replayed text never reaches the log (row 51).
    { _tag: "SessionOpened", agentId, session: session.session, protocolVersion: session.protocolVersion, reloadable: session.reloadable, ...(session.configuration === undefined ? {} : { configuration: session.configuration }) },
    ...found.map((row): Event => ({ _tag: "DeliveryAccepted", agentId, deliveryId: row.id })),
    ...unaccepted.map((row): Event => ({ _tag: "DeliveryRequeued", agentId, deliveryId: row.id, reason: "agent interrupted" })),
    ...restored.map((deliveryId): Event => ({ _tag: "DeliveryRequeued", agentId, deliveryId, reason: "agent resumed" })),
    ...(carry ? [{ _tag: "DeliveryQueued" as const, delivery: {
      id, target: agentId, sender: { _tag: "owner" as const }, content: { _tag: "carry-on" as const, text: carryOnText(id, carryText) },
    } }] : []),
    ...(!agent.capacityHeld && (retry || carry || carried) ? [{ _tag: "StateChanged" as const, agentId, state: { _tag: "working" as const } }] : []),
  ];
};

export interface Reload {
  readonly runtime: Runtime;
  readonly actor: Caller;
  readonly agent: Agent;
  readonly spawn: Spawn;
  readonly token: Token;
  readonly mode: "load" | "start";
  readonly midTurn: boolean;
  readonly timeoutMillis: number;
  // Whether the success transaction stores the new token. A resume stored it
  // with its command; recovery and a return store it only on success.
  readonly storeToken: boolean;
  readonly carryText?: string;
  // The state the agent must still be in when either result is written.
  readonly still: (agent: Agent | undefined, snapshot: Snapshot) => boolean;
  readonly failed: (snapshot: Snapshot) => ReadonlyArray<Event>;
}

// Starts or reloads one agent and writes the result. Nothing is committed
// before the setup returns (row 66). A failure writes the caller's fixed
// events and revokes the agent's tokens, and never carries the runner's
// reason. Returns whether the session opened and was recorded.
export const reload = (input: Reload): Effect.Effect<boolean, LogWriteFailed> => Effect.gen(function* () {
  const { runtime, agent, spawn, token } = input;
  const agentId = agent.agentId;
  const setup = Effect.suspend(() => input.mode === "start" || agent.session === undefined
    ? runtime.runner.start(spawn) : runtime.runner.load({ ...spawn, session: agent.session }));
  const outcome = yield* bounded(runtime, agentId, input.timeoutMillis, setup).pipe(Effect.result);
  const recorded = yield* runtime.log.transact((append) => Effect.gen(function* () {
    const snapshot = yield* runtime.views.snapshot;
    if (!input.still(snapshot.agents[agentId], snapshot)) return false;
    if (outcome._tag === "Failure") {
      yield* append(input.actor, freshRequest(), randomUUID(), withWaitChecks(snapshot, input.failed(snapshot), runtime.lists), { revokeAgentTokens: [agentId] });
      return false;
    }
    const events = reloadedEvents(snapshot, agentId, outcome.success, input.midTurn, input.carryText ?? RESTART_CARRY_ON);
    yield* append(input.actor, freshRequest(), randomUUID(), withWaitChecks(snapshot, events, runtime.lists), input.storeToken
      ? { revokeAgentTokens: [agentId], agentTokens: [{ agentId, tokenHash: token.tokenHash, revokedAt: null }] } : {});
    return true;
  })).pipe(Effect.catchTag("RequestIdReused", () => Effect.succeed(false)));
  // A stop or crash won while the session opened: the new process has no
  // record and must not outlive this call.
  if (!recorded && outcome._tag === "Success") yield* runtime.runner.stop({ agentId }).pipe(Effect.catchTag("RunnerRejected", () => Effect.void));
  return recorded;
});
