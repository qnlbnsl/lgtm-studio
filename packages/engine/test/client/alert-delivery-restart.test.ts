import { spawn } from "node:child_process";
import { once } from "node:events";
import { DatabaseSync } from "node:sqlite";
import { fileURLToPath } from "node:url";
import * as Effect from "effect/Effect";
import * as TestClock from "effect/testing/TestClock";
import { expect, it } from "vitest";
import { engineLists } from "../../src/pieces/lists.ts";
import { makeRig, removeRig } from "../support/handlerRig.ts";
import { raised } from "../support/alertsRig.ts";
import { delivered, ntfyServer, posts, withCarrier } from "../support/carrierRig.ts";
import { pause } from "../support/routingRig.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";

interface Boundary {
  readonly phase: string;
  readonly alertId: string;
  readonly writerLatched?: boolean;
  readonly reactorStopped?: boolean;
  readonly supervisorStopped?: boolean;
}

const deliveredRows = (path: string): number => {
  const database = new DatabaseSync(path, { readOnly: true });
  try { return (database.prepare("SELECT COUNT(*) AS n FROM events WHERE event_type='client.AlertDelivered'").get() as { n: number }).n; }
  finally { database.close(); }
};

it.each(["before-send", "after-acceptance", "backoff"])("6.11 SIGKILL %s keeps one alert and delivers after reopening", async (phase) => {
  registerRebuildCheck({ noLog: true });
  expect(engineLists.planners["client.alerts.carrier.set"], "6b must install carrier commands before crash boundaries can be exercised").toBeTypeOf("function");
  const rig = makeRig(); const server = await ntfyServer();
  if (phase === "backoff") server.respond(({ response }) => { response.writeHead(503); response.end(); });
  const child = spawn(process.execPath, [fileURLToPath(new URL("../support/carrierCrashChild.ts", import.meta.url)), JSON.stringify({ rig, origin: server.origin, phase })], {
    env: { PATH: process.env.PATH, HOME: rig.home, LGTM_HOME: rig.stateFolder }, stdio: ["ignore", "ignore", "pipe", "ipc"],
  });
  const exit = once(child, "exit");
  try {
    const message = await new Promise<Boundary>((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error("delivery child did not reach its boundary")), 15_000);
      child.once("message", (value) => { clearTimeout(timer); resolve(value as Boundary); });
      child.once("error", () => { clearTimeout(timer); reject(new Error("delivery child failed to start")); });
      child.once("exit", () => { clearTimeout(timer); reject(new Error("delivery child exited before the boundary")); });
    });
    expect(message.phase).toBe(phase);
    if (phase === "after-acceptance") {
      expect(message.writerLatched, "the failed writer must refuse from memory during its pause after removing the trigger").toBe(true);
      expect(message.reactorStopped, "the composed client delivery reactor must terminate with LogWriteFailed").toBe(true);
      expect(message.supervisorStopped, "the service supervisor must pause the failed reactor").toBe(true);
      const deadline = Date.now() + 4_000;
      while (posts(server).length === 0 && Date.now() < deadline) await new Promise<void>((resolve) => setTimeout(resolve, 10));
      expect(posts(server)).toHaveLength(1);
      const reply = (expected: string) => {
        const answer = new Promise<unknown>((resolve, reject) => {
          const timer = setTimeout(() => reject(new Error(`child did not reach ${expected}`)), 8_000);
          child.once("message", (value) => { clearTimeout(timer); resolve(value); });
        });
        child.send({ phase: "advance" });
        return answer;
      };
      // Review round 1, item 3: while the record keeps failing, the
      // supervisor restarts the worker every minute, and the accepted alert
      // must not be sent again.
      expect(await reply("advanced-failing")).toEqual({ phase: "advanced-failing" });
      expect(posts(server), "the live worker must not send an accepted alert again").toHaveLength(1);
      expect(deliveredRows(rig.database)).toBe(0);
      // Once writes work again (issue 1575), the live service records the
      // delivery it remembered, without sending it again and without a restart.
      expect(await reply("advanced")).toEqual({ phase: "advanced" });
      const recorded = Date.now() + 10_000;
      while (Date.now() < recorded && deliveredRows(rig.database) === 0) await new Promise<void>((resolve) => setTimeout(resolve, 20));
      expect(child.exitCode).toBeNull(); expect(child.signalCode).toBeNull();
      expect(deliveredRows(rig.database), "the live service did not record the remembered delivery").toBe(1);
      expect(posts(server), "the live worker must not send an accepted alert again").toHaveLength(1);
    }
    child.kill("SIGKILL"); expect((await exit)[1]).toBe("SIGKILL");
    const database = new DatabaseSync(rig.database);
    try {
      expect((database.prepare("SELECT COUNT(*) AS n FROM events WHERE event_type='client.AlertDelivered'").get() as { n: number }).n).toBe(phase === "after-acceptance" ? 1 : 0);
      database.exec("DROP TRIGGER IF EXISTS reject_delivery");
    } finally { database.close(); }
    server.respond(({ response }) => { response.writeHead(200); response.end(); });
    await withCarrier(() => Effect.gen(function* () {
      yield* registerRebuildCheck();
      if (phase === "backoff") {
        yield* TestClock.adjust(4_999); yield* pause(20); expect(posts(server)).toHaveLength(1);
        yield* TestClock.adjust(1);
      }
      const result = yield* delivered(message.alertId); expect(result).toHaveLength(1);
      expect(result[0]!.event.attempt).toBe(phase === "backoff" ? 2 : 1);
      expect(yield* raised()).toHaveLength(1);
      expect(posts(server)).toHaveLength(phase === "backoff" ? 2 : 1);
    }), { rig, initialize: false, clockAt: 1_800_000_000_000 });
  } finally {
    if (child.exitCode === null && child.signalCode === null) child.kill("SIGKILL");
    await exit; await server.close(); removeRig(rig);
  }
});
