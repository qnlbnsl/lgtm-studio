import { it } from "@effect/vitest";
import * as Effect from "effect/Effect";
import * as Redacted from "effect/Redacted";
import { afterEach, beforeEach, expect, vi } from "vitest";

import { incarnationOf, sessionFor } from "../support/fakeRunner.ts";
import { call, makeRig, ownerCredential, removeRig, success, type Rig } from "../support/handlerRig.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";
import { deliveryRow, endTurn, eventually, snapshot } from "../support/routingRig.ts";
import { accept, crash, deliveryRig, oneWake, send, spawn, waitFor, type Spawned } from "../support/deliveryRig.ts";

// Issue 252: a lead whose wait was satisfied while it was busy has its notice
// queued. A crash withdraws that unread notice. A successful resume must put
// it back, without repeating a notice the lead has read and without undoing
// the cancellation that a stop applies.

let rig: Rig;
beforeEach(() => { rig = makeRig(); vi.stubEnv("HOME", rig.home); vi.stubEnv("LGTM_HOME", rig.stateFolder); });
afterEach(() => { vi.unstubAllEnvs(); removeRig(rig); });
const run = <A, E, R>(body: Parameters<typeof deliveryRig<A, E, R>>[1]) =>
  deliveryRig(rig, (runner) => Effect.gen(function* () { yield* registerRebuildCheck(); return yield* body(runner); })).pipe(Effect.orDie);
const settled = (member: Spawned) => ({ agentId: member.agentId, reason: "settled" });

// A busy lead, a settled child, and the lead's queued notice, withdrawn by its crash.
const withdrawnNotice = (runner: Parameters<Parameters<typeof run>[0]>[0]) => Effect.gen(function* () {
  const lead = yield* spawn(rig, runner);
  const child = yield* spawn(rig, runner, lead);
  yield* send(runner, lead, "Keep this turn open.");
  const waitId = success(yield* waitFor(lead, [child])).waitId as string;
  const wake = yield* oneWake(waitId, lead, [settled(child)]);
  expect((yield* deliveryRow(wake.id))?.status).toBe("queued");
  yield* crash(runner, lead);
  expect((yield* eventually(deliveryRow(wake.id), (row) => row?.status === "withdrawn"))?.status).toBe("withdrawn");
  return { lead, wakeId: wake.id };
});

const sentTo = (runner: Parameters<Parameters<typeof run>[0]>[0], lead: Spawned, deliveryId: string) =>
  runner.callsOf("prompt").filter((entry) => entry.input.agentId === lead.agentId && entry.input.delivery.id === deliveryId);

// Review round 1 of unit f2-lifecycle, item 8: the end point for "nothing was
// put back". A restored notice ranks ahead of new work and the lead takes one
// turn at a time, so an owner probe queued after the resume reaches the lead
// only when nothing was restored ahead of it.
const probeReaches = (lead: Spawned) => Effect.gen(function* () {
  const id = success(yield* call(ownerCredential, "prompt", { agentId: lead.agentId, text: "Probe after the resume." })).deliveryId as string;
  expect((yield* eventually(deliveryRow(id), (row) => row?.status === "sent"))?.status, "the probe reached the lead first").toBe("sent");
});

// Review round 1, item 5: the same, but the notice was already written to the
// lead's adapter when the crash withdrew it.
const sentNoticeWithdrawn = (runner: Parameters<Parameters<typeof run>[0]>[0]) => Effect.gen(function* () {
  const lead = yield* spawn(rig, runner);
  const child = yield* spawn(rig, runner, lead);
  yield* send(runner, lead, "Keep this turn open.");
  const waitId = success(yield* waitFor(lead, [child])).waitId as string;
  const wake = yield* oneWake(waitId, lead, [settled(child)]);
  yield* endTurn(runner, lead);
  expect((yield* eventually(deliveryRow(wake.id), (row) => row?.status === "sent"))?.status).toBe("sent");
  yield* crash(runner, lead);
  expect((yield* eventually(deliveryRow(wake.id), (row) => row?.status === "withdrawn"))?.status).toBe("withdrawn");
  return { lead, wakeId: wake.id };
});

it.live("252: catches a resumed lead that never receives the wait notice its crash withdrew", () => run((runner) => Effect.gen(function* () {
  const { lead, wakeId } = yield* withdrawnNotice(runner);
  success(yield* call(ownerCredential, "resume", { agentId: lead.agentId }));
  const row = yield* eventually(deliveryRow(wakeId), (row) => row?.status === "sent");
  expect(row?.status, "the withdrawn notice is sent again after the resume").toBe("sent");
  expect(sentTo(runner, lead, wakeId)).toHaveLength(1);
})));

it.live("252: catches a notice repeated by a second crash and resume after the lead read it", () => run((runner) => Effect.gen(function* () {
  const { lead, wakeId } = yield* withdrawnNotice(runner);
  success(yield* call(ownerCredential, "resume", { agentId: lead.agentId }));
  yield* eventually(deliveryRow(wakeId), (row) => row?.status === "sent");
  // The resumed lead reads the notice and ends its turn.
  const loaded = runner.callsOf("load").at(-1)!;
  const live: Spawned = { agentId: lead.agentId, token: Redacted.value(loaded.input.token) };
  yield* accept(runner, live, wakeId);
  yield* endTurn(runner, live);
  expect((yield* deliveryRow(wakeId))?.status).toBe("accepted");
  yield* crash(runner, live);
  success(yield* call(ownerCredential, "resume", { agentId: lead.agentId }));
  yield* probeReaches(lead);
  expect((yield* deliveryRow(wakeId))?.status).toBe("accepted");
  expect(sentTo(runner, lead, wakeId)).toHaveLength(1);
})));

it.live("252: catches a stop's cancellation undone when a stopped crashed lead is resumed", () => run((runner) => Effect.gen(function* () {
  const { lead, wakeId } = yield* withdrawnNotice(runner);
  success(yield* call(ownerCredential, "stop", { agentId: lead.agentId }));
  success(yield* call(ownerCredential, "resume", { agentId: lead.agentId }));
  yield* probeReaches(lead);
  expect((yield* deliveryRow(wakeId))?.status).toBe("withdrawn");
  expect(sentTo(runner, lead, wakeId)).toEqual([]);
})));

it.live("252 review: catches a sent notice that the loaded conversation shows being delivered a second time", () => run((runner) => Effect.gen(function* () {
  const { lead, wakeId } = yield* sentNoticeWithdrawn(runner);
  runner.script("load", (input) => Effect.gen(function* () {
    yield* runner.emitRaw({ _tag: "deliveryUpdate", agentId: input.agentId, incarnation: incarnationOf(Redacted.value(input.token)),
      deliveryId: "replay-only", kind: "message", replay: true, role: "user", text: `(delivery ${wakeId})` });
    return { ...sessionFor(input.agentId), session: input.session };
  }));
  success(yield* call(ownerCredential, "resume", { agentId: lead.agentId }));
  // The lead is told to carry on instead, and that is its next turn.
  const carry = yield* eventually(snapshot, (state) => Object.values(state.deliveries)
    .some((row) => row.target === lead.agentId && row.content._tag === "carry-on" && row.status === "sent"));
  expect(Object.values(carry.deliveries).filter((row) => row.target === lead.agentId && row.content._tag === "carry-on")).toHaveLength(1);
  expect((yield* deliveryRow(wakeId))?.status).toBe("withdrawn");
  expect(sentTo(runner, lead, wakeId), "the notice reached the loaded conversation twice").toHaveLength(1);
})));

it.live("252 review: catches a sent notice that the loaded conversation lacks being lost", () => run((runner) => Effect.gen(function* () {
  const { lead, wakeId } = yield* sentNoticeWithdrawn(runner);
  success(yield* call(ownerCredential, "resume", { agentId: lead.agentId }));
  expect((yield* eventually(deliveryRow(wakeId), (row) => row?.status === "sent"))?.status).toBe("sent");
  expect(sentTo(runner, lead, wakeId)).toHaveLength(2);
})));
