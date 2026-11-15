import { randomBytes } from "node:crypto";
import { join } from "node:path";
import * as NodeSocket from "@effect/platform-node/NodeSocket";
import { it } from "@effect/vitest";
import { makeNodeSocket } from "@lgtm/client/nodeSocket";
import { connectWith } from "../../../client/src/core.ts";
import * as Effect from "effect/Effect";
import * as Redacted from "effect/Redacted";
import { afterEach, expect, vi } from "vitest";

import { engineConnection, type StorePatch } from "../../../studio/src/connection.ts";
import { serve } from "../../src/api/server.ts";
import { provideBoard } from "../board/wakeFixtures.ts";
import { makeRig, readLog, removeRig } from "../support/handlerRig.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";
import { ancestryHeaders, authorization, socketUrl } from "../support/socketClient.ts";
import { seedScreens } from "./mockScreenFixtures.ts";

afterEach(() => vi.unstubAllEnvs());

it.live("the real connection matches a real stage reply to its encoded Git completion", () => {
  const rig = makeRig();
  vi.stubEnv("HOME", rig.home); vi.stubEnv("LGTM_HOME", rig.stateFolder);
  return provideBoard(rig, Effect.gen(function* () {
    yield* Effect.addFinalizer(() => Effect.sync(() => removeRig(rig)));
    yield* registerRebuildCheck(); yield* seedScreens(rig);
    const bearer = randomBytes(32).toString("base64url"), socket = join(rig.stateFolder, "git-identity.sock");
    yield* serve({ socketPath: socket, ownerCredential: Redacted.make(bearer), debug: false });
    const state: StorePatch = {};
    const connection = engineConnection({
      location: { protocol: "http:", host: "127.0.0.1:1" }, debug() {},
      // Session probing is browser-only. The real socket authenticates with the
      // throwaway owner's bearer and uses the real encoder and client decoder.
      fetch: async () => new Response(null, { status: 204 }),
      connect: () => connectWith({ attempts: 1, makeSocket: () => makeNodeSocket(socketUrl(socket), {
        headers: { ...authorization(bearer), ...ancestryHeaders() }
      }).pipe(Effect.provide(NodeSocket.layerWebSocketConstructorWS)) })
    });
    yield* Effect.addFinalizer(() => Effect.sync(() => connection.close()));
    const reply = yield* Effect.promise(async () => {
      connection.open(patch => Object.assign(state, patch));
      await expect.poll(() => state.conn?.status).toBe("live");
      const reply = await connection.request("project.stage.all", { projectId: "project", worktreeId: "workspace" }) as { requestId: string };
      expect(reply.requestId).toMatch(/^[0-9a-f-]{36}$/);
      await expect.poll(() => state.gitChange?.requestId).toBe(reply.requestId);
      return reply;
    });
    const completion = (yield* readLog).find(item => item.event._tag === "project.GitFinished");
    expect(completion?.event).toMatchObject({ _tag: "project.GitFinished", operationRequestId: reply.requestId });
  }));
});
