// Issue 241. The runner records an adapter's identity right after spawn. The
// real Claude Code and Codex adapters are `/bin/sh` launchers that later
// `exec` Node.js, so the executable recorded at spawn is the shell. The owner
// guard must still recognise that adapter's process tree after the exec.
import { spawn } from "node:child_process";
import { mkdtempSync, readlinkSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import * as SqliteClient from "@lgtm/t3-node-sqlite/nodeSqliteClient";
import { it } from "@effect/vitest";
import * as Effect from "effect/Effect";
import * as SqlClient from "effect/sql/SqlClient";
import { expect } from "vitest";
import { outsideAdapters } from "../../src/api/ancestry.ts";
import { linuxProcessReader } from "../../src/recovery/orphans.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";

registerRebuildCheck({ noLog: true });

it.live.skipIf(process.platform !== "linux")("refuses the owner inside an adapter whose launcher has since exec'd another program", () => Effect.suspend(() => {
  const folder = mkdtempSync(join(tmpdir(), "lgtm-ancestry-"));
  return Effect.gen(function* () {
  const child = spawn("/bin/sh", ["-c", "sleep 0.3; exec sleep 30"], { detached: true, stdio: "ignore" });
  yield* Effect.promise(() => new Promise<void>((resolve) => child.once("spawn", () => resolve())));
  const pid = Number(child.pid);
  try {
    // The facts the runner records at spawn, while the launcher is still the shell.
    const recorded = linuxProcessReader.facts(pid)!;
    expect(recorded).toBeDefined();
    const sql = yield* SqlClient.SqlClient;
    yield* sql`CREATE TABLE processes (id INTEGER PRIMARY KEY AUTOINCREMENT, agent_id TEXT NOT NULL, group_id INTEGER NOT NULL,
      boot_id TEXT NOT NULL, start_time TEXT NOT NULL, user_id INTEGER NOT NULL, executable TEXT NOT NULL)`;
    yield* sql`INSERT INTO processes (agent_id, group_id, boot_id, start_time, user_id, executable)
      VALUES ('lead', ${pid}, ${linuxProcessReader.bootId()}, ${recorded.startTime}, ${recorded.userId}, ${recorded.executable})`;
    const exe = () => { try { return readlinkSync(`/proc/${pid}/exe`); } catch { return undefined; } };
    for (let i = 0; i < 200 && exe() === recorded.executable; i++) yield* Effect.sleep(20);
    expect(exe()).not.toBe(recorded.executable);
    const ownGroup = String(linuxProcessReader.ownGroupId());
    // A descendant of the adapter names it in its chain.
    expect(yield* outsideAdapters(`${process.pid}:1,${pid}:${recorded.startTime}`, ownGroup)).toBe(false);
    // A process left in the adapter's group is refused too.
    expect(yield* outsideAdapters(`${process.pid}:1`, String(pid))).toBe(false);
    // A chain that names the same pid with another start time is a different process.
    expect(yield* outsideAdapters(`${process.pid}:1,${pid}:${Number(recorded.startTime) + 1}`, ownGroup)).toBe(true);
  } finally {
    try { process.kill(-pid, "SIGKILL"); } catch { /* gone */ }
  }
  }).pipe(Effect.provide(SqliteClient.layer({ filename: join(folder, "events.sqlite") })), Effect.ensuring(Effect.sync(() => rmSync(folder, { recursive: true, force: true }))));
}), 20_000);
