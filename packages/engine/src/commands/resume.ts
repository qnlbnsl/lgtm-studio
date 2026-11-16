import { requireBuilding } from "../project/approvals.ts";
import type { Event, LifecycleState } from "@lgtm/contracts";
import * as Effect from "effect/Effect";
import * as SqlClient from "effect/sql/SqlClient";

import { mintAgentToken } from "../identity/tokens.ts";
import { EventLog } from "../log/EventLog.ts";
import { reload, SETUP_TIMEOUT_MILLIS, spawnInput } from "../recovery/reload.ts";
import { dispatchQueued } from "../routing/deliveries.ts";
import { Runner } from "../runner/Runner.ts";
import { Views } from "../views/Views.ts";
import { nextState } from "../views/apply.ts";
import { refused, type Agent, type Input, type Payload, type Plan } from "./internal.ts";
import { own, projectView } from "../project/view.ts";

// Rows 52 and 66. `resume` reloads the agent's session and then runs
// recovery's step 10 for this one agent: a delivery sent but not accepted is
// sent again, or a carry-on goes to an agent that was mid-turn. An agent
// that cannot reload is refused with "cannot reload". `resume --fresh`
// opens a new session and sends again every delivery accepted since the
// last turn end, and each one sent but never accepted, with no carry-on.
export const resume = ({ runtime, snapshot, actor }: Input, agent: Agent, payload: Payload<"resume">) =>
  Effect.gen(function* () {
    const view = projectView(snapshot), id = own(view.agentProjects, agent.agentId);
    const project = id === undefined ? undefined : own(view.projects, id);
    if (project?.archiving || project?.archivedAt !== undefined) return yield* refused("project is archived or archiving");
    if (project !== undefined && agent.agentId !== project.leadAgentId) yield* requireBuilding({ runtime, snapshot, actor }, project);
    const fresh = payload.fresh === true;
    const reloads = runtime.runner.capabilities.loadSession || runtime.runner.capabilities.resumeSession;
    if (!fresh && (agent.reloadable !== true || agent.session === undefined || !reloads)) return yield* refused("cannot reload");
    const token = mintAgentToken();
    const spawn = yield* spawnInput(runtime, snapshot, agent, token.token, "resume");
    const events: Array<Event> = [{ _tag: "StateChanged", agentId: agent.agentId, state: { _tag: "starting" } }];
    const thread = snapshot.threads[agent.agentId] ?? [];
    let state: LifecycleState = { _tag: "starting" };
    let currentDelivery: string | undefined;
    let turnEnd = 0, crashedMidTurn = false, capacityHeld = false;
    for (const row of thread) {
      // Opening the replacement session resolves the prior interruption.
      // An owner stop also ends that reload continuation. Failed setup does
      // neither, so a crash during another setup must retain the value.
      if (row.event._tag === "SessionOpened" || row.event._tag === "AgentStopped") {
        crashedMidTurn = false;
        currentDelivery = undefined;
      }
      if (row.event._tag === "DeliverySent") currentDelivery = row.event.deliveryId;
      // Reload must carry accepted work through a pause or capacity hold.
      // A second crash during setup preserves the earlier interruption.
      if (row.event._tag === "AgentCrashed" && state._tag !== "starting") {
        const accepted = currentDelivery !== undefined && snapshot.deliveries[currentDelivery]?.status === "accepted";
        crashedMidTurn = state._tag === "working" && accepted || state._tag === "paused" && state.midTurn || capacityHeld;
      }
      if (row.event._tag === "DeliveryHeld") capacityHeld = snapshot.deliveries[row.event.deliveryId]?.status === "accepted";
      if (row.event._tag === "DeliveryReleased" || row.event._tag === "AgentStopped" || row.event._tag === "AgentCrashed") capacityHeld = false;
      state = nextState(state, row.event);
      // Fresh sessions replay accepted work since the last actual turn end.
      // Stop and hold events can cut a turn; returned deliveries stay excluded
      // by their status below, even if an error did not write TurnEnded.
      if (row.event._tag === "TurnEnded") turnEnd = row.sequence;
    }
    if (fresh) {
      const accepted = new Set(thread.flatMap((row) => row.sequence > turnEnd && row.event._tag === "DeliveryAccepted"
        ? [row.event.deliveryId] : []));
      for (const deliveryId of accepted) {
        if (snapshot.deliveries[deliveryId]?.status === "accepted") events.push({
          _tag: "DeliveryRequeued", agentId: agent.agentId, deliveryId, reason: "fresh session",
        });
      }
    }
    // A failed reload keeps what the interruption recorded, so a later
    // resume still knows the agent was mid-turn.
    const previous = agent.state._tag === "interrupted" ? agent.state.previous : agent.state._tag === "crashed" && crashedMidTurn ? "working" : "starting";
    const afterCrash = agent.state._tag === "crashed" || agent.state._tag === "interrupted" && thread.findLast((row) => row.event._tag === "AgentCrashed" || row.event._tag === "AgentInterrupted")?.event._tag === "AgentCrashed";
    return {
      events, sideRows: { revokeAgentTokens: [agent.agentId], agentTokens: [{ agentId: agent.agentId, tokenHash: token.tokenHash, revokedAt: null }] },
      // Issue 78: the failure carries the wait checks it causes. The
      // target's send lane is held by CommandHandler around this command.
      afterCommit: Effect.gen(function* () {
        const opened = yield* reload({
          runtime, actor, agent, spawn, token, mode: fresh ? "start" : "load",
          midTurn: !fresh && previous === "working",
          ...(afterCrash ? { carryText: "The agent crashed. Carry on with what you were doing." } : {}),
          timeoutMillis: SETUP_TIMEOUT_MILLIS, storeToken: false,
          still: (current) => current?.state._tag === "starting",
          failed: () => [
            { _tag: "StateChanged", agentId: agent.agentId, state: { _tag: "interrupted", previous } },
            { _tag: "AgentReloadFailed", agentId: agent.agentId, reason: "agent reload failed" },
          ],
        });
        if (opened) yield* dispatchQueued(agent.agentId, { recovered: true, laneHeld: true }).pipe(Effect.catchTag("RequestIdReused", () => Effect.void));
      }).pipe(
        Effect.provideService(EventLog, runtime.log), Effect.provideService(Views, runtime.views),
        Effect.provideService(Runner, runtime.runner), Effect.provideService(SqlClient.SqlClient, runtime.sql),
      ),
    } satisfies Plan;
  });
