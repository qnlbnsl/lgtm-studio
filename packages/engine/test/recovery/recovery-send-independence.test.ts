import { it } from "@effect/vitest";
import * as Effect from "effect/Effect";
import * as Fiber from "effect/Fiber";
import { afterEach, beforeEach, expect, vi } from "vitest";

import { send, spawn } from "../support/deliveryRig.ts";
import { makeFakeRunner, type FakeRunner } from "../support/fakeRunner.ts";
import { makeRig, removeRig, seedAccount, seedHarness, type Rig } from "../support/handlerRig.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";
import { provideRestart, recover } from "../support/restartRig.ts";
import { deliveryRow, eventually } from "../support/routingRig.ts";

// Issue 253: after a restart, recovery sends each reloaded agent the head of
// its queue. An adapter that stops reading holds its own send; it must not
// hold the sends to the other recovered agents.

let rig: Rig;
beforeEach(() => {
  rig = makeRig();
  vi.stubEnv("LGTM_HOME", rig.stateFolder);
  vi.stubEnv("HOME", rig.home);
});
afterEach(() => { vi.unstubAllEnvs(); removeRig(rig); });

// The rigs provide every dependency, as in the other restart tests.
const live = (name: string, body: () => Effect.Effect<unknown, unknown, any>) =>
  it.live(name, () => body() as Effect.Effect<unknown, unknown, never>, 60_000);

const history = <A, E, R>(body: (runner: FakeRunner) => Effect.Effect<A, E, R>) => Effect.gen(function* () {
  const runner = yield* makeFakeRunner();
  return yield* provideRestart(rig, runner.layer, Effect.gen(function* () {
    yield* registerRebuildCheck();
    yield* seedAccount();
    yield* seedHarness();
    yield* runner.subscribed;
    return yield* body(runner);
  }), { recovering: false });
});

live("253: catches one stalled recovery send holding the sends to every other recovered agent", () => Effect.gen(function* () {
  const old = yield* history((runner) => Effect.gen(function* () {
    const first = yield* spawn(rig, runner);
    const second = yield* spawn(rig, runner);
    // Each has a sent, unaccepted delivery when the service stops.
    return [
      { agentId: first.agentId as string, delivery: yield* send(runner, first, "First agent's work.") },
      { agentId: second.agentId as string, delivery: yield* send(runner, second, "Second agent's work.") },
    ];
  }));
  const runner = yield* makeFakeRunner();
  yield* provideRestart(rig, runner.layer, Effect.gen(function* () {
    yield* registerRebuildCheck();
    yield* runner.subscribed;
    // Whichever agent recovery sends to first, its adapter stops reading.
    const held = runner.holdNext("prompt");
    yield* Effect.addFinalizer(() => held.release);
    const recovering = yield* recover.pipe(Effect.forkChild);
    yield* held.entered;
    const reached = yield* eventually(Effect.sync(() => runner.callsOf("prompt").map((entry) => entry.input.agentId as string)), (ids) => ids.length === 1);
    expect(reached, "the other agent's send reached its process while the first was stalled").toHaveLength(1);
    const healthy = old.find((agent) => agent.agentId === reached[0])!;
    expect((yield* deliveryRow(healthy.delivery))?.status).toBe("sent");
    yield* held.release;
    yield* Fiber.join(recovering);
    expect(runner.callsOf("prompt").map((entry) => entry.input.agentId as string).sort()).toEqual(old.map((agent) => agent.agentId).sort());
  }));
}));
