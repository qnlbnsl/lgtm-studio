import { mkdtempSync, rmSync } from "node:fs";
import { join } from "node:path";

import { it } from "@effect/vitest";
import { AgentId, RequestId, type Actor, type Event } from "@lgtm/contracts";
import * as SqliteClient from "@lgtm/t3-node-sqlite/nodeSqliteClient";
import * as Effect from "effect/Effect";
import * as Exit from "effect/Exit";
import * as Layer from "effect/Layer";
import * as Schema from "effect/Schema";
import * as SqlClient from "effect/sql/SqlClient";
import { ConnectionError, SqlError } from "effect/sql/SqlError";
import { afterEach, beforeEach, expect, vi } from "vitest";

import * as EventLog from "../../src/log/EventLog.ts";
import * as Views from "../../src/views/Views.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";
import { migrate } from "../../src/log/migrations.ts";
import * as Redactor from "../../src/log/Redactor.ts";

let home: string;
beforeEach(() => {
  home = mkdtempSync("/tmp/lgtm-commit-failure-");
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

const withLog = <A, E>(program: Effect.Effect<A, E, EventLog.EventLog | SqlClient.SqlClient>) =>
  Effect.gen(function* () {
    yield* migrate;
    return yield* Effect.scoped(Effect.gen(function* () {
      yield* registerRebuildCheck();
      return yield* program;
    })).pipe(
      Effect.provide(EventLog.layer.pipe(Layer.provideMerge(Views.layer), Layer.provide(Redactor.layer))),
    );
  }).pipe(
    Effect.provide(SqliteClient.layer({ filename: join(home, "events.sqlite") })),
    Effect.scoped,
  );

it.effect("D2 failed commit: catches leaving event and receipt writes in an open transaction, or refusing later writes after one write's own constraint failure", () =>
  withLog(Effect.gen(function* () {
    const log = yield* EventLog.EventLog;
    const sql = yield* SqlClient.SqlClient;
    yield* sql`PRAGMA foreign_keys = ON`;
    expect(yield* sql`PRAGMA foreign_keys`).toEqual([{ foreign_keys: 1 }]);
    yield* sql`CREATE TABLE commit_test_parents (id INTEGER PRIMARY KEY)`;
    yield* sql`CREATE TABLE commit_test_children (
      parent_id INTEGER REFERENCES commit_test_parents(id) DEFERRABLE INITIALLY DEFERRED
    )`;
    // The event INSERT and receipt INSERT both succeed. Only COMMIT checks
    // this deferred constraint, so statement-failure rollback cannot pass.
    yield* sql`CREATE TRIGGER fail_event_commit AFTER INSERT ON events
      BEGIN INSERT INTO commit_test_children (parent_id) VALUES (719); END`;
    const failed = yield* log.append(owner, request("commit-fails"), "7".repeat(64), [event], {})
      .pipe(Effect.result);

    expect(failed._tag).toBe("Failure");
    if (failed._tag !== "Failure") throw new Error("The deferred foreign-key violation unexpectedly committed");
    expect(failed.failure).toBeInstanceOf(EventLog.LogWriteFailed);
    // Soft assertions let all three required consequences run against a
    // broken client, including BEGIN on this same shared connection.
    expect.soft(yield* sql`SELECT COUNT(*) AS n FROM events`).toEqual([{ n: 0 }]);
    expect.soft(yield* sql`SELECT COUNT(*) AS n FROM receipts`).toEqual([{ n: 0 }]);
    const following = yield* sql.withTransaction(sql`SELECT 1`).pipe(Effect.result);
    expect.soft(following._tag).toBe("Success");

    yield* sql`DROP TRIGGER fail_event_commit`;
    // Review round 2 of unit f1-eventlog, item 1: a constraint of the
    // schema, checked at COMMIT here, belongs to that write. It leaves no
    // open transaction and no stored row, and the next write is taken.
    expect(yield* log.health!).toEqual({ failing: false });
    const later = yield* log.append(owner, request("after-commit-failure"), "8".repeat(64), [event], {})
      .pipe(Effect.result);
    expect(later._tag, "the log refused a write after another write's own constraint failure").toBe("Success");
    expect.soft(yield* sql`SELECT COUNT(*) AS n FROM events`).toEqual([{ n: 1 }]);
    expect.soft(yield* sql`SELECT COUNT(*) AS n FROM receipts`).toEqual([{ n: 1 }]);
  })),
);

// Issue 256. When COMMIT fails and the cleanup ROLLBACK fails too, Effect
// 4.0.5's `makeWithTransaction` reports both as defects, and a failed
// ROLLBACK after a failed body is a defect too. This client rolls back for
// real and then reports that same shape once.
const doubleFault = Layer.effect(SqlClient.SqlClient, Effect.gen(function* () {
  const real = yield* SqlClient.SqlClient;
  let armed = true;
  const marker = Symbol("double fault");
  const withTransaction = <A, E, R>(effect: Effect.Effect<A, E, R>) => {
    if (!armed) return real.withTransaction(effect);
    armed = false;
    return real.withTransaction(Effect.flatMap(effect, () => Effect.fail(marker))).pipe(
      Effect.catchIf((error): error is typeof marker => error === marker, () => Effect.die(new SqlError({
        reason: new ConnectionError({ message: "SQLite connection cannot be reused after failed COMMIT cleanup", operation: "rollback", cause: new Error("disk I/O error") }),
      }))),
    ) as unknown as Effect.Effect<A, E | SqlError, R>;
  };
  return new Proxy(real, { get: (target, key) => key === "withTransaction" ? withTransaction : Reflect.get(target, key) });
}));

it.effect("issue 256: a failed COMMIT whose cleanup also fails is a write failure with its fixed reason, and writes pause", () =>
  Effect.gen(function* () {
    yield* migrate;
    return yield* Effect.scoped(Effect.gen(function* () {
      yield* registerRebuildCheck();
      const log = yield* EventLog.EventLog;
      const sql = yield* SqlClient.SqlClient;
      const failed = yield* log.append(owner, request("double-fault"), "7".repeat(64), [event], {}).pipe(Effect.exit);
      expect(Exit.isFailure(failed) && failed.cause.reasons.map((reason) => reason._tag)).toEqual(["Fail"]);
      const error = Exit.isFailure(failed) ? failed.cause.reasons[0] : undefined;
      expect(error?._tag === "Fail" && error.error).toBeInstanceOf(EventLog.LogWriteFailed);
      expect(error?._tag === "Fail" && (error.error as EventLog.LogWriteFailed).reason).toBe("database write failed: rollback failed");
      // The connection works again, but writes are refused from memory during
      // the pause that follows a failure (issue 1575 replaced the permanent latch).
      const later = yield* log.append(owner, request("after-double-fault"), "8".repeat(64), [event], {}).pipe(Effect.exit);
      expect(Exit.isFailure(later) && later.cause.reasons.map((reason) => reason._tag === "Fail" && reason.error instanceof EventLog.LogWriteFailed)).toEqual([true]);
      expect(yield* sql`SELECT COUNT(*) AS n FROM events`).toEqual([{ n: 0 }]);
    })).pipe(
      Effect.provide(EventLog.layer.pipe(Layer.provideMerge(Views.layer), Layer.provide(Redactor.layer), Layer.provide(doubleFault))),
    );
  }).pipe(
    Effect.provide(SqliteClient.layer({ filename: join(home, "events.sqlite") })),
    Effect.scoped,
  ),
);
