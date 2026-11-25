import { lstatSync, readdirSync, readFileSync } from "node:fs";
import { hasValidChecksum } from "../../src/identity/credential.ts";
import { join } from "node:path";
import * as Effect from "effect/Effect";
import { expect, it, vi } from "vitest";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";
import { connected, eventually, login, post, withWeb } from "../support/webRig.ts";

registerRebuildCheck({ noLog: true });

const tokenFrom = (cookie: string): Buffer => {
  const text = decodeURIComponent(cookie.slice(cookie.indexOf("=") + 1));
  expect(hasValidChecksum(text, "shp_w_")).toBe(true);
  return Buffer.from(text.slice(6, 49), "base64url");
};
const filesIn = (root: string): string[] => readdirSync(root).flatMap((name) => {
  const path = join(root, name); const stat = lstatSync(path);
  return stat.isDirectory() ? filesIn(path) : stat.isFile() ? [path] : [];
});

it("r2 each login adds a separate 256-bit token without persisting either token", async () => withWeb((ctx) => Effect.gen(function* () {
  const first = yield* login(ctx);
  const second = yield* login(ctx);
  expect(first.cookie === second.cookie, "login must rotate the session").toBe(false);
  expect((yield* Effect.promise(() => post(ctx, "/api/session", undefined, { Cookie: first.cookie }))).status).toBe(204);
  expect((yield* Effect.promise(() => post(ctx, "/api/session", undefined, { Cookie: second.cookie }))).status).toBe(204);
  const token = tokenFrom(first.cookie);
  expect(token.byteLength).toBe(32);
  const text = first.cookie.slice(first.cookie.indexOf("=") + 1);
  for (const path of filesIn(ctx.rig.root)) {
    const data = readFileSync(path);
    expect(data.includes(token) || data.includes(text), "session token reached a persistent file").toBe(false);
  }
})));

it("5.6 logout: catches leaving either connection open or allowing an old cookie to reconnect", async () => withWeb((ctx) => Effect.gen(function* () {
  const { cookie } = yield* login(ctx);
  const one = yield* connected(ctx, cookie);
  const two = yield* connected(ctx, cookie);
  expect((yield* Effect.promise(() => post(ctx, "/api/logout", undefined, { Cookie: cookie }))).status).toBe(204);
  yield* eventually(() => [one.readyState, two.readyState], (states) => states.every((s) => s === 3), "logout left a web connection open");
  expect((yield* Effect.promise(() => post(ctx, "/api/session", undefined, { Cookie: cookie }))).status).toBe(401);
  const next = yield* login(ctx);
  expect(next.cookie === cookie, "logout reused the old token").toBe(false);
  expect((yield* Effect.promise(() => post(ctx, "/api/session", undefined, { Cookie: next.cookie }))).status).toBe(204);
})));

it("s28 fixed expiry refuses a cookie after thirty days", async () => withWeb((ctx) => Effect.gen(function* () {
  const { cookie } = yield* login(ctx);
  // Move wall time to the exact boundary without advancing the listener's
  // periodic cleanup or replacing real network timers.
  const future = Date.now() + 30 * 24 * 60 * 60 * 1_000;
  yield* Effect.acquireRelease(Effect.sync(() => vi.spyOn(Date, "now").mockReturnValue(future)),
    (clock) => Effect.sync(() => clock.mockRestore()));
  expect((yield* Effect.promise(() => post(ctx, "/api/session", undefined, { Cookie: cookie }))).status).toBe(401);
})));

it('r3 a ninth HTTP login closes only the oldest browser sockets and preserves the others', async () => withWeb(ctx => Effect.gen(function* () {
  const cookies: string[] = []
  for (let n = 0; n < 8; n++) cookies.push((yield* login(ctx)).cookie)
  const oldest = yield* connected(ctx, cookies[0]!), second = yield* connected(ctx, cookies[1]!)
  const ninth = yield* login(ctx)
  yield* eventually(() => oldest.readyState, state => state === 3, 'evicted browser kept its socket')
  expect(second.readyState).toBe(1)
  expect((yield* Effect.promise(() => post(ctx, '/api/session', undefined, { Cookie: cookies[0]! }))).status).toBe(401)
  for (const cookie of [...cookies.slice(1), ninth.cookie]) expect((yield* Effect.promise(() => post(ctx, '/api/session', undefined, { Cookie: cookie }))).status).toBe(204)
  expect((yield* Effect.promise(() => post(ctx, '/api/logout', undefined, { Cookie: ninth.cookie }))).status).toBe(204)
  expect(second.readyState).toBe(1)
})))
