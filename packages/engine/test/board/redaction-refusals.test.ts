import { it } from "@effect/vitest";
import * as Effect from "effect/Effect";
import { afterEach, beforeEach, expect, vi } from "vitest";
import { call, failureOf, headSequence, logAfter, makeRig, ownerCredential, removeRig, success, type Rig } from "../support/handlerRig.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";
import { provideBoard, seedBoard } from "./wakeFixtures.ts";

let rig: Rig;
beforeEach(() => { rig = makeRig(); vi.stubEnv("HOME", rig.home); vi.stubEnv("LGTM_HOME", rig.stateFolder); });
afterEach(() => { vi.unstubAllEnvs(); removeRig(rig); });

// This is the frozen tests' public synthetic collision marker.
const marker = "beef";
for (const kind of ["name", "post"] as const) {
  it.live(`decision 161 refuses a redacted ${kind} that fails its stored schema without a board event or crash`, () => provideBoard(rig, Effect.gen(function* () {
    yield* registerRebuildCheck();
    yield* seedBoard();
    const before = yield* headSequence;
    // A random UUID can contain the registered collision marker and be
    // refused before this test reaches the intended board validation.
    const requestId = `refusal-${kind}`;
    const result = kind === "name"
      ? yield* call(ownerCredential, "board.open", { requestId, name: `Room-${marker}` })
      : yield* call(ownerCredential, "board.post", { requestId, direct: { _tag: "agent", agentId: "worker" }, text: marker.repeat(4096), reply: "none", to: [] });
    const reason = kind === "name" ? "name holds a secret" : "post too long once secrets are hidden";
    expect(failureOf(result)).toEqual({ _tag: "CommandRefused", reason });
    expect((yield* logAfter(before)).map((row) => row.event)).toEqual([
      { _tag: "CommandRefused", command: kind === "name" ? "board.open" : "board.post", reason },
    ]);
    // A subsequent real command proves that the refusal left the handler usable.
    expect(success(yield* call(ownerCredential, "board.open", { requestId: `after-${kind}`, name: "AfterRefusal" })).channelId).toBeTypeOf("string");
  }), [marker]).pipe(Effect.orDie));
}
