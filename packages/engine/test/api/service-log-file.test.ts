// Issue 1576: a service started with `lgtm serve --detach` used to send its
// output to /dev/null, so a failed write left no trace. The service now keeps
// a bounded, owner-only log file in its state folder, fed by the same
// redacted and filtered lines as its terminal output.
//
// Issue 1575: the write-failure line keeps its fixed reason and numbers, and
// nothing else, through the log filter.
import { execFileSync } from "node:child_process";
import { closeSync, constants, existsSync, lstatSync, mkdtempSync, openSync, readFileSync, readSync, rmSync, statSync, symlinkSync, writeFileSync } from "node:fs";
import { join } from "node:path";

import { afterEach, beforeEach, expect, test } from "vitest";

import { serviceLog } from "../../src/api/serviceLog.ts";
import { serviceLogFile, serviceLogFileName, serviceLogLimitBytes } from "../../src/api/serviceLogFile.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";

registerRebuildCheck({ noLog: true });

let folder: string;
beforeEach(() => { folder = mkdtempSync("/tmp/lgtm-service-log-"); });
afterEach(() => { rmSync(folder, { recursive: true, force: true }); });

const line = (message: string, annotations: Record<string, unknown> = {}) =>
  JSON.stringify({ timestamp: "2026-11-23T00:00:00.000Z", level: "ERROR", fiberId: "#1", message, annotations, spans: {} });

test("1576 log file: lines reach service.log in the state folder, readable by the owner only", () => {
  const file = serviceLogFile(folder);
  const sink = serviceLog(false, file.write);
  sink(line("event log write failed", { reason: "database write failed: busy", failures: 1, sqliteCode: 5 }));
  file.close();
  const path = join(folder, serviceLogFileName);
  expect(file.path).toBe(path);
  expect(statSync(path).mode & 0o777).toBe(0o600);
  expect(readFileSync(path, "utf8").trim().split("\n").map((entry) => JSON.parse(entry))).toEqual([{
    timestamp: "2026-11-23T00:00:00.000Z", level: "ERROR", message: "event log write failed",
    annotations: { reason: "database write failed: busy", failures: 1, sqliteCode: 5 },
  }]);
});

test("1576 log file: an existing file is narrowed to the owner, and a symbolic link is not followed", () => {
  const path = join(folder, serviceLogFileName);
  writeFileSync(path, "", { mode: 0o644 });
  const file = serviceLogFile(folder);
  file.write("first");
  file.close();
  expect(statSync(path).mode & 0o777).toBe(0o600);
  rmSync(path);
  const target = join(folder, "elsewhere");
  writeFileSync(target, "untouched");
  symlinkSync(target, path);
  const linked = serviceLogFile(folder);
  linked.write("must not reach the target");
  linked.close();
  expect(readFileSync(target, "utf8")).toBe("untouched");
  expect(lstatSync(path).isSymbolicLink()).toBe(true);
});

test("1576 log file: the log rotates at its limit, so the two files never exceed twice the limit, and an oversized line is dropped", () => {
  expect(serviceLogLimitBytes).toBe(1024 * 1024);
  const limit = 100;
  const file = serviceLogFile(folder, limit);
  const path = join(folder, serviceLogFileName);
  for (let index = 0; index < 50; index++) file.write(`line ${String(index).padStart(2, "0")} ${"x".repeat(20)}`);
  file.write("y".repeat(limit + 1));
  file.close();
  expect(statSync(path).size).toBeLessThanOrEqual(limit);
  expect(statSync(`${path}.1`).size).toBeLessThanOrEqual(limit);
  expect(statSync(`${path}.1`).mode & 0o777).toBe(0o600);
  expect(existsSync(`${path}.2`)).toBe(false);
  const newest = readFileSync(path, "utf8");
  expect(newest).toContain("line 49");
  expect(newest).not.toContain("yyyy");
});

test("1575 log filter: a write-failure line keeps only its fixed reason and numbers", () => {
  const lines: Array<string> = [];
  const sink = serviceLog(false, (entry) => { lines.push(entry); });
  sink(line("event log write failed", { reason: "database write failed: disk full", failures: 3, sqliteCode: 13, row: "private text" }));
  sink(line("event log write failed", { reason: "a reason quoting private text", failures: -1, sqliteCode: "13" }));
  sink(line("event log writes recovered", { failures: 3, reason: "ignored" }));
  sink(line("piece reactor paused: database write failed"));
  expect(lines.map((entry) => JSON.parse(entry))).toEqual([
    expect.objectContaining({ message: "event log write failed", annotations: { reason: "database write failed: disk full", failures: 3, sqliteCode: 13 } }),
    expect.objectContaining({ message: "event log write failed", annotations: { reason: "other" } }),
    expect.objectContaining({ message: "event log writes recovered", annotations: { failures: 3 } }),
    { timestamp: "2026-11-23T00:00:00.000Z", level: "ERROR", message: "piece reactor paused: database write failed" },
  ]);
});

// Review round 1, item 7: a FIFO planted at the log's path must not block the
// service, and only a regular file is written.
test("item 7 log file: a FIFO at the log's path neither blocks the service nor receives lines", () => {
  const path = join(folder, serviceLogFileName);
  execFileSync("mkfifo", ["-m", "600", path]);
  const file = serviceLogFile(folder);
  const started = Date.now();
  file.write("must not block");
  file.close();
  expect(Date.now() - started).toBeLessThan(1_000);
  expect(lstatSync(path).isFIFO()).toBe(true);
  // With a reader waiting, the open succeeds, and the FIFO is still refused.
  const reader = openSync(path, constants.O_RDONLY | constants.O_NONBLOCK);
  try {
    const again = serviceLogFile(folder);
    again.write("must not reach the reader");
    again.close();
    const buffer = Buffer.alloc(64);
    let read = 0;
    try { read = readSync(reader, buffer); } catch { read = 0; }
    expect(read).toBe(0);
  } finally { closeSync(reader); }
});

// Review round 1, item 8 (issue 1610): the supervisor's reactor lines keep
// the reactor's namespace, and only a fixed piece name.
test("item 8 log filter: reactor lines keep a fixed namespace and nothing else", () => {
  const lines: Array<string> = [];
  const sink = serviceLog(false, (entry) => { lines.push(entry); });
  sink(line("piece reactor failed", { namespace: "board", detail: "private text" }));
  sink(line("piece reactor paused: database write failed", { namespace: "client" }));
  sink(line("piece reactor failed", { namespace: "private text" }));
  expect(lines.map((entry) => JSON.parse(entry))).toEqual([
    expect.objectContaining({ message: "piece reactor failed", annotations: { namespace: "board" } }),
    expect.objectContaining({ message: "piece reactor paused: database write failed", annotations: { namespace: "client" } }),
    { timestamp: "2026-11-23T00:00:00.000Z", level: "ERROR", message: "piece reactor failed" },
  ]);
});
