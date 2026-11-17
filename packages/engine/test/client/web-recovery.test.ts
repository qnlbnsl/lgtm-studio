import * as Deferred from "effect/Deferred";
import * as Effect from "effect/Effect";
import * as Layer from "effect/Layer";
import { expect, it } from "vitest";
import { Restart } from "../../src/recovery/service.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";
import { http, login, post, upgrade, withWeb } from "../support/webRig.ts";

registerRebuildCheck({ noLog: true });

it("serves static files and login during recovery, authenticating before typed progress", async () => {
  let ready = false;
  let recovered = 0;
  const gate = Deferred.makeUnsafe<void>();
  const recovery = Layer.succeed(Restart, {
    ready: Effect.sync(() => ready), whenReady: Deferred.await(gate), recover: Effect.void, returnAccount: () => Effect.void,
    progress: Effect.sync(() => ({ _tag: 'ServiceStarting' as const, recovered, total: 2 })),
  });
  await withWeb((ctx) => Effect.gen(function* () {
    expect((yield* Effect.promise(() => http(ctx, "/"))).status).toBe(200);
    expect((yield* Effect.promise(() => post(ctx, "/api/login", { password: 'wrong' }))).status).toBe(401);
    const { cookie } = yield* login(ctx);
    for (const path of ["/api", "/api/session", "/api/logout"]) {
      expect((yield* Effect.promise(() => post(ctx, path))).status).toBe(401);
      const reply = yield* Effect.promise(() => post(ctx, path, undefined, { Cookie: cookie }));
      expect(reply.status).toBe(503);
      expect(JSON.parse(reply.body)).toEqual({ _tag: 'ServiceStarting', recovered: 0, total: 2 });
    }
    expect((yield* upgrade(ctx)).status).toBe(401);
    const socketReply = yield* upgrade(ctx, { Cookie: cookie });
    expect(socketReply.status).toBe(503);
    expect(JSON.parse(socketReply.body!)).toEqual({ _tag: 'ServiceStarting', recovered: 0, total: 2 });
    recovered = 1;
    expect(JSON.parse((yield* Effect.promise(() => post(ctx, "/api/session", undefined, { Cookie: cookie }))).body)).toMatchObject({ recovered: 1, total: 2 });
    ready = true; yield* Deferred.succeed(gate, undefined);
    expect((yield* Effect.promise(() => http(ctx, "/api/session", { method: "POST", headers: { Cookie: cookie } }))).status).toBe(204);
    expect((yield* upgrade(ctx, { Cookie: cookie })).status).toBe(101);
  }), { recovery });
});
