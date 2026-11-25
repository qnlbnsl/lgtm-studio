// Issue 1575: one failed write used to refuse every later write for the life
// of the service, with no sign. A failure is now logged, reported by the
// log's health, refused from memory only for a bounded pause, and retried
// against the database after it. The live views still never run ahead of or
// behind the committed log.
//
// Issue 255: an unreadable stored receipt refuses only the request that
// reads it, and the writer stays healthy.
import { mkdtempSync, rmSync } from "node:fs";
import { join } from "node:path";

import { it } from "@effect/vitest";
import { AgentId, RequestId, type Actor, type EventEnvelope, type Event } from "@lgtm/contracts";
import * as SqliteClient from "@lgtm/t3-node-sqlite/nodeSqliteClient";
import * as Effect from "effect/Effect";
import * as Exit from "effect/Exit";
import * as Fiber from "effect/Fiber";
import * as Layer from "effect/Layer";
import * as Clock from "effect/Clock";
import * as PubSub from "effect/PubSub";
import * as Schema from "effect/Schema";
import type * as Scope from "effect/Scope";
import * as SqlClient from "effect/sql/SqlClient";
import { ConnectionError, SqlError } from "effect/sql/SqlError";
import * as TestClock from "effect/testing/TestClock";
import { afterEach, beforeEach, expect, vi } from "vitest";

import { serviceLog } from "../../src/api/serviceLog.ts";
import { callerKey } from "../../src/identity/caller.ts";
import { redactingLoggerLayer } from "../../src/identity/logger.ts";
import * as EventLog from "../../src/log/EventLog.ts";
import { migrate } from "../../src/log/migrations.ts";
import * as Redactor from "../../src/log/Redactor.ts";
import * as Views from "../../src/views/Views.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";

let home: string;
beforeEach(() => {
  home = mkdtempSync("/tmp/lgtm-write-recovery-");
  vi.stubEnv("LGTM_HOME", home);
});
afterEach(() => {
  vi.unstubAllEnvs();
  rmSync(home, { recursive: true, force: true });
});

const owner: Actor = { _tag: "owner" };
const agentId = Schema.decodeUnknownSync(AgentId)("agent-a");
const request = (id: string) => Schema.decodeUnknownSync(RequestId)(id);
const event: Event = { _tag: "TurnStarted", agentId };
const hash = (digit: string) => digit.repeat(64);
const start = 1_800_000_000_000;
// SQLite's result code for a write to a read-only connection.
const SQLITE_READONLY = 8;

const withLog = <A, E>(
  program: Effect.Effect<A, E, EventLog.EventLog | Views.Views | SqlClient.SqlClient | Scope.Scope>,
  options: {
    readonly rebuildCheck?: boolean; readonly lines?: Array<string>; readonly views?: Layer.Layer<Views.Views, unknown, SqlClient.SqlClient>;
    // Replaces the SQL client that the event log alone sees.
    readonly wrapSql?: (sql: SqlClient.SqlClient) => SqlClient.SqlClient;
  } = {},
) =>
  Effect.gen(function* () {
    yield* TestClock.setTime(start);
    yield* migrate;
    return yield* Effect.scoped(Effect.gen(function* () {
      if (options.rebuildCheck !== false) yield* registerRebuildCheck();
      return yield* program;
    })).pipe(
      Effect.provide(EventLog.layer.pipe(
        options.wrapSql === undefined ? (layer) => layer
          : Layer.provide(Layer.effect(SqlClient.SqlClient, Effect.map(SqlClient.SqlClient, options.wrapSql))),
        Layer.provideMerge(options.views ?? Views.layer), Layer.provide(Redactor.layer))),
    );
  }).pipe(
    // The service's own pipeline: the redacting logger, then the log filter.
    Effect.provide(redactingLoggerLayer({ redactText: (text) => text }, serviceLog(false, (line) => { options.lines?.push(line); }))),
    Effect.provide(SqliteClient.layer({ filename: join(home, "events.sqlite") })),
    Effect.scoped,
  );

const failureOf = <A, E>(effect: Effect.Effect<A, E>) => Effect.gen(function* () {
  const result = yield* Effect.result(effect);
  if (result._tag !== "Failure") throw new Error("the write unexpectedly succeeded");
  return result.failure;
});

const count = (table: "events" | "receipts") => Effect.gen(function* () {
  const sql = yield* SqlClient.SqlClient;
  return (yield* sql<{ n: number }>`SELECT COUNT(*) AS n FROM ${sql(table)}`)[0]!.n;
});

const health = Effect.gen(function* () {
  const log = yield* EventLog.EventLog;
  return yield* log.health!;
});

it.effect("1575 recovery: a failed write is reported and logged, refused from memory during the pause, and retried against the database after it", () => {
  const lines: Array<string> = [];
  return withLog(Effect.gen(function* () {
    const log = yield* EventLog.EventLog;
    const sql = yield* SqlClient.SqlClient;
    yield* log.append(owner, request("before"), hash("1"), [event], {});
    expect(yield* health).toEqual({ failing: false });

    yield* sql`PRAGMA query_only = ON`;
    const failed = yield* failureOf(log.append(owner, request("cannot-write"), hash("2"), [event], {}));
    expect(failed).toBeInstanceOf(EventLog.LogWriteFailed);
    expect(failed.reason).toBe("database write failed");
    expect(yield* health).toEqual({
      failing: true, reason: "database write failed", since: start, sqliteCode: SQLITE_READONLY,
      failures: 1, nextAttempt: start + EventLog.firstRetryMillis,
    });

    // The fault is gone, but the pause has not ended: the writer refuses from
    // memory, without asking the database, and the failure count stays.
    yield* sql`PRAGMA query_only = OFF`;
    const early = yield* failureOf(log.append(owner, request("too-early"), hash("3"), [event], {}));
    expect(early.reason).toBe(failed.reason);
    expect(yield* health).toMatchObject({ failing: true, failures: 1 });
    expect(yield* count("events")).toBe(1);

    // After the pause the next write goes to the database and succeeds.
    yield* TestClock.adjust(EventLog.firstRetryMillis);
    const written = yield* log.append(owner, request("after-pause"), hash("4"), [event], {});
    expect(written).toHaveLength(1);
    expect(yield* health).toEqual({ failing: false });
    expect(yield* count("events")).toBe(2);
    expect((yield* (yield* Views.Views).snapshot).sequence).toBe(written[0]!.sequence);

    const logged = lines.map((line) => JSON.parse(line) as { message: string; annotations?: Record<string, unknown> });
    expect(logged.filter((line) => line.message === "event log write failed")).toEqual([expect.objectContaining({
      level: "ERROR", annotations: { reason: "database write failed", failures: 1, sqliteCode: SQLITE_READONLY },
    })]);
    expect(logged.filter((line) => line.message === "event log writes recovered")).toEqual([expect.objectContaining({
      annotations: { failures: 1 },
    })]);
  }), { lines });
});

it.effect("1575 back-off: each further failure doubles the pause up to its cap, and the report keeps the first failure", () =>
  withLog(Effect.gen(function* () {
    const log = yield* EventLog.EventLog;
    const sql = yield* SqlClient.SqlClient;
    yield* sql`PRAGMA query_only = ON`;
    yield* failureOf(log.append(owner, request("first"), hash("1"), [event], {}));
    const pauses: Array<number> = [];
    let pause = EventLog.firstRetryMillis;
    for (let attempt = 2; attempt <= 8; attempt++) {
      // One millisecond short of the pause, the writer still refuses from memory.
      yield* TestClock.adjust(pause - 1);
      yield* failureOf(log.append(owner, request(`early-${attempt}`), hash("2"), [event], {}));
      expect(yield* health).toMatchObject({ failures: attempt - 1 });
      yield* TestClock.adjust(1);
      yield* failureOf(log.append(owner, request(`retry-${attempt}`), hash("3"), [event], {}));
      const report = yield* health;
      if (!report.failing) throw new Error("the writer reported health during a failure");
      expect(report).toMatchObject({ reason: "database write failed", since: start, failures: attempt });
      const now = yield* Clock.currentTimeMillis;
      pause = report.nextAttempt! - now;
      pauses.push(pause);
    }
    expect(pauses).toEqual([4_000, 8_000, 16_000, 32_000, 60_000, 60_000, 60_000]);
    expect(EventLog.maxRetryMillis).toBe(60_000);
    yield* sql`PRAGMA query_only = OFF`;
    yield* TestClock.adjust(pause);
    yield* log.append(owner, request("recovered"), hash("4"), [event], {});
    expect(yield* health).toEqual({ failing: false });
  })));

it.effect("1575 behind: a batch stored by a write that reported failure reaches the live views and subscribers before the next write", () =>
  withLog(Effect.gen(function* () {
    const log = yield* EventLog.EventLog;
    const views = yield* Views.Views;
    const sql = yield* SqlClient.SqlClient;
    const committed = yield* log.subscribeCommitted;
    yield* log.append(owner, request("before"), hash("1"), [event], {});
    yield* sql`PRAGMA query_only = ON`;
    yield* failureOf(log.append(owner, request("cannot-write"), hash("2"), [event], {}));
    yield* sql`PRAGMA query_only = OFF`;
    // Stand-in for a COMMIT that reported failure but stored its batch: the
    // row is in the log, and the live views have not seen it.
    yield* sql`INSERT INTO events (event_id, caller, caller_rule, request_id, agent_id, thread_agent_id, event_type, occurred_at, payload_json)
      SELECT 'stored-despite-failure', caller, caller_rule, 'cannot-write', agent_id, thread_agent_id, event_type, occurred_at, payload_json
      FROM events WHERE sequence = 1`;
    expect((yield* views.snapshot).sequence).toBe(1);

    yield* TestClock.adjust(EventLog.firstRetryMillis);
    const written = yield* log.append(owner, request("after-pause"), hash("3"), [event], {});
    expect(written[0]!.sequence).toBe(3);
    expect((yield* views.snapshot).sequence).toBe(3);
    const seen: Array<EventEnvelope> = [];
    for (let index = 0; index < 3; index++) seen.push(yield* PubSub.take(committed));
    expect(seen.map((envelope) => [envelope.sequence, envelope.eventId === "stored-despite-failure"])).toEqual([
      [1, false], [2, true], [3, false],
    ]);
    expect(yield* health).toEqual({ failing: false });
  })));

it.effect("1575 ahead: live views ahead of the log refuse every write until a restart, and the report says so", () =>
  withLog(Effect.gen(function* () {
    const log = yield* EventLog.EventLog;
    const sql = yield* SqlClient.SqlClient;
    yield* log.append(owner, request("before"), hash("1"), [event], {});
    yield* sql`PRAGMA query_only = ON`;
    yield* failureOf(log.append(owner, request("cannot-write"), hash("2"), [event], {}));
    yield* sql`PRAGMA query_only = OFF`;
    yield* sql`DELETE FROM events WHERE sequence = 1`;

    yield* TestClock.adjust(EventLog.firstRetryMillis);
    const refused = yield* failureOf(log.append(owner, request("after-pause"), hash("3"), [event], {}));
    expect(refused.reason).toBe("live views are ahead of the log");
    expect(yield* count("events")).toBe(0);
    expect(yield* health).toEqual({
      failing: true, reason: "database write failed", since: start, sqliteCode: SQLITE_READONLY,
      failures: 1, restartNeeded: true, restartReason: "live views are ahead of the log",
    });
    yield* TestClock.adjust(10 * EventLog.maxRetryMillis);
    expect((yield* failureOf(log.append(owner, request("much-later"), hash("4"), [event], {}))).reason)
      .toBe("live views are ahead of the log");
    expect(yield* count("events")).toBe(0);
  }), { rebuildCheck: false }));

// Each shape a stored receipt can take, old (a bare list of envelopes) and
// current (envelopes with a reply), broken so that it no longer decodes.
const unreadableReceipts = {
  "malformed text": "{not json",
  "old format with an invalid envelope": JSON.stringify([{ sequence: "one" }]),
  "current format with an invalid envelope": JSON.stringify({ envelopes: [{ sequence: "one" }], reply: {} }),
  "current format without envelopes": JSON.stringify({ reply: {} }),
};

for (const [name, resultJson] of Object.entries(unreadableReceipts)) {
  it.effect(`255 unreadable receipt (${name}): refuses only the request that reads it and leaves the writer healthy`, () =>
    withLog(Effect.gen(function* () {
      const log = yield* EventLog.EventLog;
      const sql = yield* SqlClient.SqlClient;
      yield* sql`INSERT INTO receipts (caller, request_id, body_hash, result_json, first_sequence)
        VALUES (${callerKey(owner)}, 'damaged', ${hash("1")}, ${resultJson}, NULL)`;
      const refused = yield* failureOf(log.append(owner, request("damaged"), hash("1"), [event], {}));
      expect(refused).toBeInstanceOf(EventLog.ReceiptUnreadable);
      expect(refused.reason).toBe("stored receipt is unreadable");
      expect(yield* health).toEqual({ failing: false });
      // An unrelated write right after it is taken.
      expect(yield* log.append(owner, request("unrelated"), hash("2"), [event], {})).toHaveLength(1);
      // The damaged request is still refused the same way, and only it.
      expect((yield* failureOf(log.append(owner, request("damaged"), hash("1"), [event], {}))).reason)
        .toBe("stored receipt is unreadable");
      expect(yield* count("events")).toBe(1);
    })));
}

// Issue 46: the live views change in memory after COMMIT. A defect there
// would leave them behind the committed log, and later batches would then be
// applied on top of a state that misses one. The writer now refuses every
// write and reports that a restart, which rebuilds the views, is needed.
const poisonedViews = Layer.effect(Views.Views, Effect.gen(function* () {
  const real = yield* Views.Views;
  return Views.Views.of({ ...real, applyCommitted: (batch) => batch.some((envelope) => envelope.requestId === "poison")
    ? Effect.die(new Error("planted defect")) : real.applyCommitted(batch) });
})).pipe(Layer.provide(Views.layer));

it.effect("46 defect after commit: a batch the views could not apply stops every later write and reports that a restart is needed", () => {
  const lines: Array<string> = [];
  return withLog(Effect.gen(function* () {
    const log = yield* EventLog.EventLog;
    yield* log.append(owner, request("before"), hash("1"), [event], {});
    const exit = yield* Effect.exit(log.append(owner, request("poison"), hash("2"), [event], {}));
    expect(exit._tag).toBe("Failure");
    // The batch is committed; the views missed it.
    expect(yield* count("events")).toBe(2);
    expect((yield* (yield* Views.Views).snapshot).sequence).toBe(1);
    expect(yield* health).toEqual({ failing: true, reason: "live views missed a committed batch", since: start, failures: 1, restartNeeded: true, restartReason: "live views missed a committed batch" });
    yield* TestClock.adjust(10 * EventLog.maxRetryMillis);
    expect((yield* failureOf(log.append(owner, request("later"), hash("3"), [event], {}))).reason).toBe("live views missed a committed batch");
    expect(yield* count("events")).toBe(2);
    expect(lines.map((line) => JSON.parse(line) as { message: string; annotations?: unknown })
      .filter((line) => line.message === "event log write failed")).toEqual([
      expect.objectContaining({ annotations: { reason: "live views missed a committed batch" } }),
    ]);
  }), { rebuildCheck: false, lines, views: poisonedViews });
});

// Review round 1, item 1. A caller's interruption must not skip recording a
// failure, or the next write would skip reconciling the views with the log.
it.effect("item 1 interrupted failure: a write that fails while its caller is interrupted is still recorded", () =>
  withLog(Effect.gen(function* () {
    const log = yield* EventLog.EventLog;
    const sql = yield* SqlClient.SqlClient;
    yield* sql`PRAGMA query_only = ON`;
    // The caller asks for its own interruption inside the writer's turn,
    // just before the append that fails.
    const fiber = yield* Effect.forkChild(log.transact((append) => Effect.withFiber((self) => {
      self.interruptUnsafe();
      return append(owner, request("interrupted"), hash("1"), [event], {});
    })));
    const exit = yield* Fiber.await(fiber);
    expect(Exit.isFailure(exit)).toBe(true);
    expect(yield* health).toMatchObject({ failing: true, reason: "database write failed", failures: 1 });
  })));

// Review round 1, item 1. The views' update and the publication of the same
// events, during `reconcile`, must not be split by an interruption.
const interruptingViews = Layer.effect(Views.Views, Effect.gen(function* () {
  const real = yield* Views.Views;
  return Views.Views.of({ ...real, applyCommitted: (batch) => batch.some((envelope) => envelope.eventId === "stored-despite-failure")
    ? Effect.withFiber((self) => { self.interruptUnsafe(); return real.applyCommitted(batch); }).pipe(Effect.andThen(Effect.yieldNow))
    : real.applyCommitted(batch) });
})).pipe(Layer.provide(Views.layer));

it.effect("item 1 interrupted reconcile: subscribers receive every event the views applied, even when the caller is interrupted between the two", () =>
  withLog(Effect.gen(function* () {
    const log = yield* EventLog.EventLog;
    const views = yield* Views.Views;
    const sql = yield* SqlClient.SqlClient;
    const committed = yield* log.subscribeCommitted;
    yield* log.append(owner, request("before"), hash("1"), [event], {});
    yield* sql`PRAGMA query_only = ON`;
    yield* failureOf(log.append(owner, request("cannot-write"), hash("2"), [event], {}));
    yield* sql`PRAGMA query_only = OFF`;
    yield* sql`INSERT INTO events (event_id, caller, caller_rule, request_id, agent_id, thread_agent_id, event_type, occurred_at, payload_json)
      SELECT 'stored-despite-failure', caller, caller_rule, 'cannot-write', agent_id, thread_agent_id, event_type, occurred_at, payload_json
      FROM events WHERE sequence = 1`;
    yield* TestClock.adjust(EventLog.firstRetryMillis);
    yield* Fiber.await(yield* Effect.forkChild(log.append(owner, request("interrupted"), hash("3"), [event], {})));
    yield* log.append(owner, request("after"), hash("4"), [event], {});
    const head = (yield* views.snapshot).sequence;
    const seen: Array<number> = [];
    while (seen.length < head) seen.push((yield* PubSub.take(committed)).sequence);
    expect(seen).toEqual(Array.from({ length: head }, (_, index) => index + 1));
  }), { views: interruptingViews }));

// Review round 1, item 2. When a COMMIT fails and its ROLLBACK fails too,
// Effect reports both as a defect. The batch may be stored. This stands in
// for that pair: the transaction commits, and the client then reports the
// defect that Effect raises for a failed COMMIT cleanup.
const failingRollback = (armed: { now: boolean }) => (sql: SqlClient.SqlClient): SqlClient.SqlClient =>
  new Proxy(sql, {
    get: (target, property, receiver) => property === "withTransaction" && armed.now
      ? <A, E, R>(effect: Effect.Effect<A, E, R>) => {
        armed.now = false;
        return target.withTransaction(effect).pipe(Effect.andThen(Effect.die(new SqlError({
          reason: new ConnectionError({ message: "planted ROLLBACK failure after a failed COMMIT", operation: "rollback", cause: undefined }),
        }))));
      }
      : Reflect.get(target, property, receiver),
  });

it.effect("item 2 failed rollback: a COMMIT and ROLLBACK that both fail are recorded as a write failure, and the next write reconciles", () => {
  const armed = { now: false };
  return withLog(Effect.gen(function* () {
    const log = yield* EventLog.EventLog;
    const views = yield* Views.Views;
    yield* log.append(owner, request("before"), hash("1"), [event], {});
    armed.now = true;
    const failed = yield* failureOf(log.append(owner, request("unknown-outcome"), hash("2"), [event], {}));
    expect(failed).toBeInstanceOf(EventLog.LogWriteFailed);
    expect(failed.reason).toBe("database write failed: rollback failed");
    expect(yield* health).toMatchObject({ failing: true, reason: "database write failed: rollback failed", failures: 1 });
    expect(yield* count("events")).toBe(2);
    expect((yield* views.snapshot).sequence).toBe(1);
    yield* TestClock.adjust(EventLog.firstRetryMillis);
    const written = yield* log.append(owner, request("after"), hash("3"), [event], {});
    expect(written[0]!.sequence).toBe(3);
    expect((yield* views.snapshot).sequence).toBe(3);
    expect(yield* health).toEqual({ failing: false });
  }), { wrapSql: failingRollback(armed) });
});

// Review round 1, item 5. A committed write with no events, such as side
// rows alone, also proves that writing works again.
it.effect("item 5 side rows only: a committed write without events clears the failure", () =>
  withLog(Effect.gen(function* () {
    const log = yield* EventLog.EventLog;
    const sql = yield* SqlClient.SqlClient;
    yield* sql`PRAGMA query_only = ON`;
    yield* failureOf(log.append(owner, request("cannot-write"), hash("1"), [event], {}));
    yield* sql`PRAGMA query_only = OFF`;
    yield* TestClock.adjust(EventLog.firstRetryMillis);
    yield* log.append(owner, request("tokens-only"), hash("2"), [], {
      agentTokens: [{ agentId, tokenHash: "e".repeat(64), revokedAt: null }],
    }, false);
    expect(yield* count("events")).toBe(0);
    expect(yield* health).toEqual({ failing: false });
    // A transaction that writes nothing proves nothing and leaves a failure alone.
    yield* sql`PRAGMA query_only = ON`;
    yield* TestClock.adjust(EventLog.firstRetryMillis);
    yield* failureOf(log.append(owner, request("cannot-write-2"), hash("3"), [event], {}));
    yield* sql`PRAGMA query_only = OFF`;
    yield* TestClock.adjust(EventLog.firstRetryMillis);
    yield* log.transact(() => Effect.void);
    expect(yield* health).toMatchObject({ failing: true });
  })));

// Review round 2, item 6. When a statement fails and the ROLLBACK after it
// fails too, Effect raises the ROLLBACK's error as a defect and drops the
// statement's failure. This stands in for that pair: the real transaction
// fails and rolls back, and the client then reports the ROLLBACK's defect.
const failingRollbackAfterFailure = (armed: { now: boolean }, message: string) => (sql: SqlClient.SqlClient): SqlClient.SqlClient =>
  new Proxy(sql, {
    get: (target, property, receiver) => property === "withTransaction" && armed.now
      ? <A, E, R>(effect: Effect.Effect<A, E, R>) => {
        armed.now = false;
        return target.withTransaction(effect).pipe(Effect.catch(() => Effect.die(new SqlError({
          reason: new ConnectionError({ message: "planted ROLLBACK failure", operation: "rollback", cause: { message } }),
        }))));
      }
      : Reflect.get(target, property, receiver),
  });

// A storage fault inside the body: every event insert names a missing table.
const missingTable = "CREATE TRIGGER planted_storage BEFORE INSERT ON events BEGIN INSERT INTO planted_missing_table VALUES (1); END";

it.effect("item 6 already rolled back: a statement failure keeps its own reason and code when SQLite had already rolled back", () => {
  const armed = { now: false };
  return withLog(Effect.gen(function* () {
    const log = yield* EventLog.EventLog;
    const sql = yield* SqlClient.SqlClient;
    yield* sql.unsafe(missingTable);
    armed.now = true;
    const failed = yield* failureOf(log.append(owner, request("readonly"), hash("1"), [event], {}));
    expect(failed.reason).toBe("database write failed");
    expect(yield* health).toEqual({
      failing: true, reason: "database write failed", since: start, sqliteCode: 1, failures: 1, nextAttempt: start + EventLog.firstRetryMillis,
    });
  }), { wrapSql: failingRollbackAfterFailure(armed, "cannot rollback - no transaction is active") });
});

it.effect("item 6 own failure, already rolled back: a constraint whose ROLLBACK found no transaction leaves the writer healthy", () => {
  const armed = { now: false };
  return withLog(Effect.gen(function* () {
    const log = yield* EventLog.EventLog;
    const sql = yield* SqlClient.SqlClient;
    yield* log.append(owner, request("first"), hash("0"), [event], {});
    // A constraint of the schema: one event of this type only.
    yield* sql.unsafe("CREATE UNIQUE INDEX planted_unique ON events(event_type)");
    armed.now = true;
    const failed = yield* failureOf(log.append(owner, request("constraint"), hash("1"), [event], {}));
    expect(failed.reason).toBe("database write failed: constraint");
    expect(yield* health).toEqual({ failing: false });
  }), { wrapSql: failingRollbackAfterFailure(armed, "cannot rollback - no transaction is active") });
});

it.effect("item 6 transaction left open: a statement failure whose ROLLBACK really failed keeps its reason and code and asks for a restart", () => {
  const armed = { now: false };
  return withLog(Effect.gen(function* () {
    const log = yield* EventLog.EventLog;
    const sql = yield* SqlClient.SqlClient;
    yield* sql.unsafe(missingTable);
    armed.now = true;
    const failed = yield* failureOf(log.append(owner, request("left-open"), hash("1"), [event], {}));
    expect(failed.reason).toBe("database write failed");
    expect(yield* health).toEqual({
      failing: true, reason: "database write failed", since: start, sqliteCode: 1, failures: 1,
      restartNeeded: true, restartReason: "database write failed: transaction left open",
    });
    yield* sql.unsafe("DROP TRIGGER planted_storage");
    yield* TestClock.adjust(EventLog.maxRetryMillis);
    yield* failureOf(log.append(owner, request("after"), hash("2"), [event], {}));
    expect(yield* count("events")).toBe(0);
  }), { wrapSql: failingRollbackAfterFailure(armed, "cannot rollback - disk I/O error") });
});
