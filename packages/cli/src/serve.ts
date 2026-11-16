import { spawn } from "node:child_process";
import { request } from "node:http";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { stateFolder } from "@lgtm/engine/api/stateFolder";
import * as Effect from "effect/Effect";

const answers = (socketPath: string): Promise<boolean> => new Promise((resolve) => {
  const call = request({ socketPath, path: "/", method: "GET", timeout: 1_000 }, (response) => {
    response.resume();
    resolve(true);
  });
  call.once("error", () => resolve(false));
  call.once("timeout", () => { call.destroy(); resolve(false); });
  call.end();
});

// Probe the actual listener, including one still recovering. Neither an
// existing socket pathname nor opening engine.lock proves it is running.
export const serveDetach = (options: { readonly environment: Record<string, string | undefined>; readonly timeoutMs?: number }) => Effect.gen(function* () {
  const socket = join(stateFolder(options.environment), "engine.sock");
  if (yield* Effect.promise(() => answers(socket))) return { outcome: "already running" } as const;
  const child = spawn(process.execPath, [fileURLToPath(new URL("../../engine/src/service.ts", import.meta.url))], {
    env: options.environment, detached: true, stdio: ["ignore", "ignore", "ignore", "ipc"],
  });
  child.unref();
  let exited = false;
  let exitCode: number | null = null;
  let failed = false;
  let ownsLock = false;
  let closed = false;
  let reason: string | undefined;
  child.on("message", (message: unknown) => {
    if (typeof message !== "object" || message === null || !("_tag" in message)) return;
    if (message._tag === "LgtmLockAcquired") ownsLock = true;
    // Only this service-owned channel carries a bounded, already-redacted reason.
    if (message._tag === "LgtmStartupFailed" && "reason" in message && typeof message.reason === "string" && message.reason.length <= 512) reason = message.reason;
  });
  child.once("error", () => { failed = true; });
  child.once("exit", (code) => { exited = true; exitCode = code; });
  // Close follows channel shutdown, including a refusal delivered after exit.
  child.once("close", (code) => { closed = true; exitCode = code; });
  const deadline = Date.now() + (options.timeoutMs ?? 30_000);
  for (;;) {
    if (failed || (closed && exitCode !== 0)) return yield* Effect.fail(new Error(reason === undefined ? "service failed to start" : `service failed to start: ${reason}`));
    if (((ownsLock && !exited) || (exited && exitCode === 0)) && (yield* Effect.promise(() => answers(socket)))) {
      return ownsLock ? { outcome: "started", pid: child.pid! } as const : { outcome: "already running" } as const;
    }
    if ((!exited || exitCode === 0) && Date.now() >= deadline) {
      if (!exited && child.pid !== undefined) {
        // This launcher created the detached group. Stop its startup work
        // before reporting failure, including any descendants it spawned.
        try { process.kill(-child.pid, "SIGKILL"); } catch { child.kill("SIGKILL"); }
      }
      return yield* Effect.fail(new Error("service did not answer"));
    }
    yield* Effect.sleep(25);
  }
});
