import { it } from "@effect/vitest";
import * as Effect from "effect/Effect";
import { afterEach, beforeEach, expect, vi } from "vitest";

import { endTurn } from "../support/routingRig.ts";
import { accept, send, spawn } from "../support/deliveryRig.ts";
import { makeFakeRunner, type FakeRunner } from "../support/fakeRunner.ts";
import { makeRig, removeRig, seedAccount, seedHarness, type Rig } from "../support/handlerRig.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";
import { promptsAfter, provideRestart, recover } from "../support/restartRig.ts";

// Issue 1235: a restart loads an existing session. When the agent's first
// turn never succeeded, its retried first message must still carry the role
// instructions, so the engine tells the runner that the session owes them.

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

const recoverWith = (runner: FakeRunner) => provideRestart(rig, runner.layer, Effect.gen(function* () {
  yield* registerRebuildCheck();
  yield* runner.subscribed;
  yield* recover;
}));

live("1235 restart: catches a reload that forgets the instructions an interrupted first turn still owes", () => Effect.gen(function* () {
  const old = yield* history((runner) => Effect.gen(function* () {
    const target = yield* spawn(rig, runner);
    // The first turn is sent and never finishes before the restart.
    return { target, delivery: yield* send(runner, target, "First brief.") };
  }));
  const runner = yield* makeFakeRunner();
  yield* recoverWith(runner);
  const loads = runner.callsOf("load").filter((entry) => entry.input.agentId === old.target.agentId);
  expect(loads).toHaveLength(1);
  expect(loads[0]!.input.instructionsOwed, "the reload says the session still owes its instructions").toBe(true);
  expect(promptsAfter(runner, 0, old.target.agentId).map((entry) => entry.input.delivery.id)).toEqual([old.delivery]);
}));

live("1235 restart: catches a reload that sends the instructions again after a first turn that succeeded", () => Effect.gen(function* () {
  const old = yield* history((runner) => Effect.gen(function* () {
    const target = yield* spawn(rig, runner);
    const first = yield* send(runner, target, "First brief.");
    yield* accept(runner, target, first);
    yield* endTurn(runner, target);
    return { target, delivery: yield* send(runner, target, "Second brief.") };
  }));
  const runner = yield* makeFakeRunner();
  yield* recoverWith(runner);
  const loads = runner.callsOf("load").filter((entry) => entry.input.agentId === old.target.agentId);
  expect(loads).toHaveLength(1);
  expect(loads[0]!.input.instructionsOwed ?? false).toBe(false);
}));
