import { it } from "@effect/vitest";
import * as Effect from "effect/Effect";
import { afterEach, beforeEach, expect, vi } from "vitest";

import { accountFor, deliveryFor, echoFile, harnessFor, makeRunnerRig, promptTexts, readEcho, removeRunnerRig, spawnFor, withAcpRunner, writeFakeAdapter, type RunnerRig } from "../support/acpRunnerRig.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";

// Issue 1235: the obligation to send role instructions lives in the runner's
// memory. A loaded session whose earlier first turn never succeeded still
// owes them, and the engine tells the runner so when it loads the session.

let rig: RunnerRig;
beforeEach(() => { rig = makeRunnerRig(); vi.stubEnv("HOME", rig.home); vi.stubEnv("LGTM_HOME", rig.stateFolder); });
afterEach(() => { vi.unstubAllEnvs(); removeRunnerRig(rig); });
registerRebuildCheck({ noLog: true });

it.live("1235 runner: catches a loaded session that still owes its role instructions sending the first prompt without them", () =>
  withAcpRunner(rig, {}, (context) => Effect.gen(function* () {
    const echo = echoFile(rig);
    const spawn = yield* spawnFor(rig, harnessFor("first", writeFakeAdapter(rig)), accountFor("first", { turns: [[], []] }, echo));
    const opened = yield* context.runner.start({ ...spawn.input, instructions: "unused" });
    yield* context.runner.stop({ agentId: spawn.agentId });
    let reads = 0;
    yield* context.runner.load({
      ...spawn.input, session: opened.session, instructionsOwed: true,
      restoreInstructions: () => Effect.sync(() => { reads++; return "# Worker base\n\nOWED ROLE RULES"; }),
    });
    const send = (text: string) => Effect.gen(function* () {
      const delivery = deliveryFor(spawn.agentId, text), before = context.events.length;
      yield* context.runner.prompt({ agentId: spawn.agentId, incarnation: spawn.incarnation, delivery, sentSequence: 1 });
      yield* context.waitFor((e) => e._tag === "turnEnded" && e.deliveryId === delivery.id, before);
    });
    yield* send("Retried first brief.");
    yield* send("Second brief.");
    const prompts = promptTexts(yield* readEcho(echo, (e) => promptTexts(e).length === 2));
    expect(prompts[0], "the retried first prompt carries the owed instructions").toContain("OWED ROLE RULES");
    expect(prompts[0]!.indexOf("OWED ROLE RULES")).toBeLessThan(prompts[0]!.indexOf("Retried first brief."));
    expect(prompts[1]).toBe("Second brief.");
    expect(reads).toBe(1);
  })));
