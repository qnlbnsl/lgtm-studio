// Review round 1 of unit f1-eventlog, item 4 (issue 1609), and review round
// 2. Writes can fail and recover while agents keep running. An agent event
// refused because the event log is failing for a storage reason is held in
// memory, in order, and written when writes work again, so a permission
// request still reaches the owner and an exit still revokes the agent's
// token. Nothing acts on a held event before it is written. An event that
// fails for a reason of its own is dropped and does not stop other writes.
// Bounded memory makes the service report that a restart is needed.
import { randomUUID } from "node:crypto";

import { it } from "@effect/vitest";
import * as Effect from "effect/Effect";
import * as Layer from "effect/Layer";
import { MinimumLogLevel } from "effect/References";
import * as SqlClient from "effect/sql/SqlClient";
import { ConnectionError, SqlError } from "effect/sql/SqlError";
import { afterEach, beforeEach, expect, vi } from "vitest";

import { heldRunnerEventBytesLimit, heldRunnerEventLimit } from "../../src/commands/CommandHandler.ts";
import { redactingLoggerLayer } from "../../src/identity/logger.ts";
import { verifyAgentToken } from "../../src/identity/tokens.ts";
import { EventLog, LogWriteFailed } from "../../src/log/EventLog.ts";
import { incarnationOf, makeFakeRunner } from "../support/fakeRunner.ts";
import {
  agentIdOf, headSequence, logAfter, makeFakeSecrets, makeRig, provideRig, removeRig, seedAccount, seedAgent, seedHarness, stateOf,
  type Fakes, type Rig,
} from "../support/handlerRig.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";
import { pause, requestRow, standardOptions } from "../support/routingRig.ts";

let rig: Rig;
beforeEach(() => {
  rig = makeRig();
  vi.stubEnv("HOME", rig.home);
  vi.stubEnv("LGTM_HOME", rig.stateFolder);
});
afterEach(() => {
  vi.unstubAllEnvs();
  removeRig(rig);
});

type Extra = Omit<Fakes, "runner" | "secrets">;
const withRig = <A, E, R>(body: (fakes: Fakes) => Effect.Effect<A, E, R>, options: { readonly rebuildCheck?: boolean; readonly extra?: Extra } = {}) =>
  Effect.gen(function* () {
    const fakes: Fakes = { runner: yield* makeFakeRunner(), secrets: makeFakeSecrets(), ...options.extra };
    return yield* provideRig(rig, fakes, Effect.gen(function* () {
      if (options.rebuildCheck !== false) yield* registerRebuildCheck();
      yield* seedAccount();
      yield* seedHarness();
      yield* fakes.runner.subscribed;
      return yield* body(fakes);
    }));
  });

const capturing = (lines: Array<string>) => Layer.mergeAll(
  redactingLoggerLayer({ redactText: (text: string) => text }, (line) => { lines.push(line); }),
  Layer.succeed(MinimumLogLevel, "All"),
);
const messages = (lines: ReadonlyArray<string>) => lines.map((line) => (JSON.parse(line) as { message: unknown }).message);

// A storage fault that every event insert hits: the trigger names a table
// that does not exist, so SQLite fails the insert with its generic error,
// not a constraint.
const failWrites = Effect.gen(function* () {
  const sql = yield* SqlClient.SqlClient;
  yield* sql.unsafe(`CREATE TRIGGER planted_failure BEFORE INSERT ON events BEGIN INSERT INTO planted_missing_table VALUES (1); END`);
});
const restoreWrites = Effect.gen(function* () {
  const sql = yield* SqlClient.SqlClient;
  yield* sql.unsafe(`DROP TRIGGER planted_failure`);
});
const health = Effect.flatMap(EventLog, (log) => log.health!);

// Waits up to ten seconds of real time, which covers the writer's pause and
// the held events' next attempt.
const within = <A, E, R>(effect: Effect.Effect<A, E, R>, done: (value: A) => boolean) => Effect.gen(function* () {
  let value = yield* effect;
  for (let tries = 0; tries < 200 && !done(value); tries++) {
    yield* pause(50);
    value = yield* effect;
  }
  return value;
});

type Who = { readonly agentId: string; readonly token: string };
const permission = (who: Who, id: string) => ({
  _tag: "permissionRequested" as const, agentId: agentIdOf(who.agentId), incarnation: incarnationOf(who.token),
  permissionRequestId: id, toolCallId: "tool-held", description: "Run the tests.", options: standardOptions, settingsWrite: false,
});
const turnEnded = (who: Who) => ({ _tag: "turnEnded" as const, agentId: agentIdOf(who.agentId), incarnation: incarnationOf(who.token) });
const update = (who: Who, text: string) => ({
  _tag: "deliveryUpdate" as const, agentId: agentIdOf(who.agentId), incarnation: incarnationOf(who.token),
  deliveryId: randomUUID(), kind: "message" as const, replay: false, text, content: { _tag: "text" as const, text },
});

it.live("item 4 permission request: an agent's request that arrives while writes fail is written, in order, once writes work again", () =>
  withRig((fakes) => Effect.gen(function* () {
    yield* seedAgent({ id: "lead" });
    const worker = yield* seedAgent({ id: "held-worker", parent: "lead", state: "working" });
    const before = yield* headSequence;
    yield* failWrites;
    yield* fakes.runner.emit(permission(worker, "perm-held") as never);
    yield* fakes.runner.emit(turnEnded(worker) as never);
    yield* pause(300);
    // Nothing acts on an event that is not written.
    expect(yield* requestRow("perm-held")).toBeUndefined();
    expect(yield* logAfter(before)).toEqual([]);
    // Review round 2, item 5: the report counts the held events and agents.
    expect(yield* health).toMatchObject({ failing: true, heldRunnerEvents: 2, heldAgents: 1 });

    yield* restoreWrites;
    const rows = yield* within(logAfter(before), (found) => found.some((row) => row.event._tag === "TurnEnded"));
    expect(rows.map((row) => row.event._tag).filter((tag) => tag === "PermissionRequested" || tag === "TurnEnded"))
      .toEqual(["PermissionRequested", "TurnEnded"]);
    expect(yield* requestRow("perm-held")).toMatchObject({ agentId: "held-worker", status: "open" });
    expect(yield* health).toEqual({ failing: false });
  })).pipe(Effect.orDie));

it.live("item 4 exit: an exit that arrives while writes fail still crashes the agent and revokes its token once writes work again", () =>
  withRig((fakes) => Effect.gen(function* () {
    const solo = yield* seedAgent({ id: "held-solo", state: "working" });
    expect((yield* Effect.result(verifyAgentToken(solo.token)))._tag).toBe("Success");
    yield* failWrites;
    yield* fakes.runner.emit({ _tag: "exited", agentId: agentIdOf(solo.agentId), incarnation: incarnationOf(solo.token), exitCode: 1, signal: null, stderr: "" } as never);
    yield* pause(300);
    expect(yield* stateOf(solo.agentId)).toBe("working");

    yield* restoreWrites;
    const state = yield* within(stateOf(solo.agentId), (found) => found !== "working");
    expect(state).toBe("crashed");
    expect((yield* Effect.result(verifyAgentToken(solo.token)))._tag, "a crashed agent kept a live token").toBe("Failure");
  })).pipe(Effect.orDie));

it.live("item 4 bound: more held events than the limit make the service report that a restart is needed instead of dropping them silently", () =>
  withRig((fakes) => Effect.gen(function* () {
    const worker = yield* seedAgent({ id: "held-many", state: "working" });
    yield* failWrites;
    for (let index = 0; index <= heldRunnerEventLimit; index++) yield* fakes.runner.emit(turnEnded(worker) as never);
    const found = yield* within(health, (value) => value.failing && value.restartNeeded === true);
    expect(found).toMatchObject({ failing: true, restartNeeded: true, restartReason: "agent events were dropped while writes failed" });
    expect(found).not.toHaveProperty("heldRunnerEvents");
    // A restart is the only way out: writes stay refused after the fault goes.
    yield* restoreWrites;
    yield* pause(2_500);
    expect(yield* health).toMatchObject({ restartNeeded: true });
  }), { rebuildCheck: false }).pipe(Effect.orDie));

// Review round 2, item 1. One event that fails for a reason of its own, here
// a unique constraint that allows one permission request only, is dropped
// with its fixed reason.
// It is not held, it does not mark the database as failing, and the events
// before and after it are written at once.
it.live("item 1 poisoned event: an event that can never be written is dropped, and the events around it and later commands still write", () => {
  const lines: Array<string> = [];
  return withRig((fakes) => Effect.gen(function* () {
    const worker = yield* seedAgent({ id: "poison-worker", state: "working" });
    const sql = yield* SqlClient.SqlClient;
    yield* sql.unsafe(`CREATE UNIQUE INDEX planted_poison ON events(event_type) WHERE event_type = 'PermissionRequested'`);
    const before = yield* headSequence;
    yield* fakes.runner.emit(permission(worker, "perm-first") as never);
    yield* within(logAfter(before), (found) => found.some((row) => row.event._tag === "PermissionRequested"));
    const mark = lines.length;
    yield* fakes.runner.emit(update(worker, "First.") as never);
    yield* fakes.runner.emit(permission(worker, "perm-poisoned") as never);
    yield* fakes.runner.emit(update(worker, "Second.") as never);
    const rows = yield* within(logAfter(before), (found) => found.filter((row) => row.event._tag === "ThreadItem").length >= 2);
    expect(rows.filter((row) => row.event._tag === "ThreadItem").map((row) => (row.event as unknown as { content: { text: string } }).content.text))
      .toEqual(["First.", "Second."]);
    expect(rows.filter((row) => row.event._tag === "PermissionRequested")).toHaveLength(1);
    expect(yield* requestRow("perm-poisoned")).toBeUndefined();
    expect(yield* health).toEqual({ failing: false });
    expect(messages(lines.slice(mark))).toContain("runner event dropped: write refused");
    expect(messages(lines.slice(mark))).not.toContain("event log write failed");
    expect(messages(lines.slice(mark))).not.toContain("runner event held: database write failed");
    // It is not retried: after the held events' interval, nothing failed.
    yield* pause(2_500);
    expect(yield* health).toEqual({ failing: false });
    yield* fakes.runner.emit(update(worker, "Third.") as never);
    const later = yield* within(logAfter(before), (found) => found.filter((row) => row.event._tag === "ThreadItem").length >= 3);
    expect(later.filter((row) => row.event._tag === "ThreadItem")).toHaveLength(3);
  })).pipe(Effect.provide(capturing(lines)), Effect.orDie);
});

// Review round 2, item 2. A held event's retry reuses the identity of its
// first attempt. The first COMMIT stores the batch and then reports a
// failure, as when the ROLLBACK after a failed COMMIT fails too. The event
// is held, the next write reconciles the views with the stored batch, and
// the retry finds its receipt instead of writing a second item.
it.live("item 2 stored retry: a held update whose first write was stored is not written twice", () => {
  const armed = { now: false };
  const wrapLogSql = (sql: SqlClient.SqlClient): SqlClient.SqlClient => new Proxy(sql, {
    get: (target, property, receiver) => property === "withTransaction" && armed.now
      ? <A, E, R>(effect: Effect.Effect<A, E, R>) => {
        armed.now = false;
        return target.withTransaction(effect).pipe(Effect.andThen(Effect.die(new SqlError({
          reason: new ConnectionError({ message: "planted ROLLBACK failure after a failed COMMIT", operation: "rollback", cause: undefined }),
        }))));
      }
      : Reflect.get(target, property, receiver),
  });
  return withRig((fakes) => Effect.gen(function* () {
    const worker = yield* seedAgent({ id: "stored-worker", state: "working" });
    const before = yield* headSequence;
    armed.now = true;
    yield* fakes.runner.emit(update(worker, "Stored once.") as never);
    expect(yield* within(health, (value) => value.failing)).toMatchObject({ failing: true, reason: "database write failed: rollback failed" });
    // The retry found its receipt. A receipt changes nothing in the
    // database, so it does not prove that writing works again (review round
    // 1, item 5), and the report clears with the next real write.
    const drained = yield* within(health, (value) => value.heldRunnerEvents === undefined);
    expect(drained).toMatchObject({ failing: true, reason: "database write failed: rollback failed" });
    expect(drained).not.toHaveProperty("heldRunnerEvents");
    const sql = yield* SqlClient.SqlClient;
    const stored = yield* sql<{ count: number }>`SELECT COUNT(*) AS count FROM events WHERE event_type = 'ThreadItem' AND sequence > ${before}`;
    expect(stored[0]!.count, "the retry wrote the update a second time").toBe(1);
    expect((yield* logAfter(before)).filter((row) => row.event._tag === "ThreadItem")).toHaveLength(1);
    yield* fakes.runner.emit(update(worker, "A later update.") as never);
    expect(yield* within(health, (value) => !value.failing)).toEqual({ failing: false });
    expect((yield* logAfter(before)).filter((row) => row.event._tag === "ThreadItem")).toHaveLength(2);
  }), { extra: { wrapLogSql } }).pipe(Effect.orDie);
});

// Review round 2, items 1 and 3. The event log refuses writes with an error
// it counts as an outage, without a pause, so the test decides exactly when
// writes recover. An agent holds exactly the limit; its next event arrives
// after writes work again and must write everything, not drop it.
const plantedOutage = new LogWriteFailed({ reason: "database write failed" });
const switchable = (failing: { now: boolean }): Extra => ({
  wrapLog: (real) => ({
    ...real,
    transact: (use) => failing.now ? Effect.fail(plantedOutage) : real.transact(use),
    isOutage: (error) => error === plantedOutage || (real.isOutage?.(error) ?? false),
  }),
});

it.live("item 3 flush first: an agent at the limit whose next event arrives after writes recover loses nothing", () => {
  const failing = { now: false };
  return withRig((fakes) => Effect.gen(function* () {
    const worker = yield* seedAgent({ id: "limit-worker", state: "working" });
    const before = yield* headSequence;
    failing.now = true;
    for (let index = 0; index < heldRunnerEventLimit; index++) yield* fakes.runner.emit(update(worker, `Held ${index}.`) as never);
    const full = yield* within(health, (value) => value.heldRunnerEvents === heldRunnerEventLimit);
    // The held events' next attempt has not run yet, so the next event is the
    // first to find writes working.
    expect(full.heldRunnerEvents).toBe(heldRunnerEventLimit);
    failing.now = false;
    yield* fakes.runner.emit(update(worker, "After recovery.") as never);
    const rows = yield* within(logAfter(before), (found) => found.filter((row) => row.event._tag === "ThreadItem").length > heldRunnerEventLimit);
    const texts = rows.filter((row) => row.event._tag === "ThreadItem").map((row) => (row.event as unknown as { content: { text: string } }).content.text);
    expect(texts).toEqual([...Array.from({ length: heldRunnerEventLimit }, (_, index) => `Held ${index}.`), "After recovery."]);
    expect(yield* health).toEqual({ failing: false });
  }), { extra: switchable(failing) }).pipe(Effect.orDie);
});

// Review round 2, item 4. A byte bound across all agents: few, large events
// from several agents, each agent far below its count, reach it. The service
// reports that a restart is needed once and logs one line, however many
// events arrive after.
it.live("item 4 byte bound: large held events across agents reach the byte bound, report a restart once, and log one line", () => {
  const failing = { now: false };
  const lines: Array<string> = [];
  return withRig((fakes) => Effect.gen(function* () {
    const workers = [];
    for (let index = 0; index < 4; index++) workers.push(yield* seedAgent({ id: `bytes-worker-${index}`, state: "working" }));
    const large = "x".repeat(4 * 1024 * 1024);
    const needed = Math.ceil(heldRunnerEventBytesLimit / large.length) + 1;
    const mark = lines.length;
    failing.now = true;
    for (let index = 0; index < needed + 4; index++) yield* fakes.runner.emit(update(workers[index % workers.length]!, large) as never);
    const found = yield* within(health, (value) => value.failing && value.restartNeeded === true);
    expect(found).toMatchObject({ restartNeeded: true, restartReason: "agent events were dropped while writes failed" });
    yield* pause(300);
    const logged = messages(lines.slice(mark));
    expect(logged.filter((message) => message === "runner events dropped: writes failed too long")).toHaveLength(1);
    expect(logged.filter((message) => typeof message === "string" && message.startsWith("runner event dropped"))).toEqual([]);
  }), { rebuildCheck: false, extra: switchable(failing) }).pipe(Effect.provide(capturing(lines)), Effect.orDie);
});
