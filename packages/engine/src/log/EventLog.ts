import { randomUUID } from "node:crypto";
import { callerKey } from "../identity/caller.ts";

import {
  EventEnvelope, type Caller, type AgentId, type BootId, type Event, type RequestId,
} from "@lgtm/contracts";
import * as Clock from "effect/Clock";
import * as Context from "effect/Context";
import * as Data from "effect/Data";
import * as Effect from "effect/Effect";
import * as Layer from "effect/Layer";
import * as PubSub from "effect/PubSub";
import * as Schema from "effect/Schema";
import type * as Scope from "effect/Scope";
import * as Semaphore from "effect/Semaphore";
import * as SqlClient from "effect/sql/SqlClient";
import { isSqlError, type SqlError } from "effect/sql/SqlError";

import { engineLists, type EngineLists } from "../pieces/lists.ts";
import { threadAgent } from "../views/apply.ts";
import * as Views from "../views/Views.ts";
import * as Redactor from "./Redactor.ts";
import { fieldStructure, receiptStructure, type RedactionStructure } from "./redactionStructure.ts";
import { loadRoleFile } from "../roles/roleFile.ts";
import { decodeRowWith, type EventRow } from "./subscribe.ts";

// The reason names the failure and never carries data. It is built from a
// fixed category and nothing else, because SQLite's own message can quote a
// stored value.
export class LogWriteFailed extends Data.TaggedError("LogWriteFailed")<{
  readonly reason: string;
}> {}

// A refused event is a failed write, but does not mean SQLite is unusable.
// The command boundary translates it into an ordinary input refusal.
export class InvalidEvent extends LogWriteFailed {}

// Issue 255: a stored receipt that no longer decodes is bad data in one row,
// not a failing database. It refuses only the request that reads it, and the
// writer's health is unchanged.
export class ReceiptUnreadable extends LogWriteFailed {
  constructor() { super({ reason: "stored receipt is unreadable" }); }
}

// Issue 1575: what a status read reports about the writer. `since` and
// `reason` describe the first failure of the current run of failures, and
// `sqliteCode` is SQLite's extended result code for it, when SQLite gave one.
// None of these fields carries stored data.
export type WriteHealth =
  | ({ readonly failing: false } & HeldRunnerEvents)
  | {
    readonly failing: true;
    readonly reason: string;
    readonly since: number;
    readonly sqliteCode?: number;
    readonly failures: number;
    // When the next write goes to the database again. It is absent, and
    // `restartNeeded` is set, when the live views disagree with the log in a
    // way only a restart repairs: views ahead of the log, or a committed
    // batch the views failed to apply (issue 46).
    readonly nextAttempt?: number;
    readonly restartNeeded?: true;
    // Why only a restart clears the failure, when `restartNeeded` is set.
    readonly restartReason?: string;
  } & HeldRunnerEvents;

// Review round 2, item 5: agent events waiting in memory for writes to work
// again, and how many agents they belong to. Both are absent when none wait.
export interface HeldRunnerEvents {
  readonly heldRunnerEvents?: number;
  readonly heldAgents?: number;
}

// A failed write is retried against the database after this pause, which
// doubles with each further failure up to the cap. Until then a write is
// refused from memory, so a broken disk is not asked again at once.
export const firstRetryMillis = 2_000;
export const maxRetryMillis = 60_000;

export class RequestIdReused extends Data.TaggedError("RequestIdReused")<{
  readonly reason: "request id reused";
}> {}

export interface AgentTokenRow {
  readonly agentId: AgentId;
  readonly tokenHash: string;
  readonly revokedAt: number | null;
}

export interface ProcessRow {
  readonly agentId: AgentId;
  readonly groupId: number;
  readonly bootId: BootId;
  readonly startTime: string;
  readonly userId: number;
  readonly executable: string;
}

// Rows written in the same transaction as the batch's events (decision row
// 32, issue 33). Revocations are applied before inserts, so a rotation
// revokes an agent's earlier tokens and then adds its new one. `"all"`
// revokes every agent token, for start-up recovery (decision row 52, step 4).
// A revoked row's `revoked_at` is the batch's `occurredAt`.
export interface SideRows {
  readonly [table: `${"client" | "board" | "roles"}_${string}`]: ReadonlyArray<Readonly<Record<string, unknown>>> | undefined;
  readonly revokeAgentTokens?: ReadonlyArray<AgentId> | "all";
  readonly agentTokens?: ReadonlyArray<AgentTokenRow>;
  readonly processes?: ReadonlyArray<ProcessRow>;
}

type Append = (
  caller: Caller, requestId: RequestId, bodyHash: string,
  events: ReadonlyArray<Event>, sideRows: SideRows,
  receipt?: false | ((envelopes: ReadonlyArray<EventEnvelope>) => unknown),
) => Effect.Effect<ReadonlyArray<EventEnvelope>, LogWriteFailed | RequestIdReused>;

export class EventLog extends Context.Service<
  EventLog,
  {
    readonly lists?: EngineLists;
    readonly append: Append;
    // Check metadata with the same redactor instance that protects appends.
    readonly redactor?: Redactor.Redactor["Service"];
    readonly redactText: (text: string) => Effect.Effect<string>;
    readonly subscribeCommitted: Effect.Effect<PubSub.Subscription<EventEnvelope>, never, Scope.Scope>;
    // Runs an effect while holding the writer's permit, so no batch commits
    // or updates the live views meanwhile. For `compareViews` only.
    readonly withWriterPermit: <A, E, R>(effect: Effect.Effect<A, E, R>) => Effect.Effect<A, E, R>;
    // Whether writes are failing. Optional only so test doubles may omit it.
    readonly health?: Effect.Effect<WriteHealth>;
    // Refuses every later write until a restart, with a fixed reason that the
    // health report gives. For state a restart's recovery repairs, such as
    // agent events that had to be dropped (review round 1, item 4).
    readonly requireRestart?: (reason: string) => Effect.Effect<void>;
    // Review round 2, item 1: whether a failed write was refused because the
    // log is failing for a storage reason that any other write would also
    // hit, rather than for a reason of its own. Only such a write is worth
    // holding and retrying.
    readonly isOutage?: (error: unknown) => boolean;
    // Review round 2, item 5: how many agent events wait in memory, for the
    // health report.
    readonly noteHeld?: (held: HeldRunnerEvents) => void;
    // Checks and append share the permit and the SQLite transaction. Only
    // use the supplied append inside the callback; publication follows COMMIT.
    readonly transact: <A, E, R>(
      use: (append: Append) => Effect.Effect<A, E, R>,
    ) => Effect.Effect<A, E | LogWriteFailed, R>;
  }
>()("lgtm/engine/log/EventLog") {}



// SQLite's primary result code for a full disk.
const SQLITE_FULL = 13;

const sqliteCode = (error: SqlError): number | undefined => {
  const cause = error.reason.cause;
  const errno: unknown = typeof cause === "object" && cause !== null && "errno" in cause ? cause.errno : undefined;
  return typeof errno === "number" && Number.isInteger(errno) ? errno : undefined;
};

// A write failure keeps its SQLite result code, a number that names the
// fault and never quotes stored data, for the health report and the log.
class StorageFailed extends LogWriteFailed {
  readonly sqliteCode: number | undefined;
  // Set when the failure is the database's, so another write would hit it
  // too. A constraint failure belongs to the one write that broke it.
  readonly outage: boolean;
  constructor(error: SqlError, reason = failureReason(error), options: { readonly sqliteCode?: number | undefined; readonly outage?: boolean } = {}) {
    super({ reason });
    this.sqliteCode = "sqliteCode" in options ? options.sqliteCode : sqliteCode(error);
    this.outage = options.outage ?? (!ownFailureReasons.has(reason) || this.sqliteCode === SQLITE_CONSTRAINT_TRIGGER);
  }
}

// SQLite's extended result code for an abort raised by a trigger. The
// engine's schema defines no triggers, so such an abort comes from outside
// the engine, from a damaged or altered database, and the writer cannot tell
// which other writes it hits. It counts as a storage failure, unlike a
// constraint of the engine's own schema, which belongs to one write.
const SQLITE_CONSTRAINT_TRIGGER = 1811;

// Review round 2, item 1: failures that belong to one write and say nothing
// about the database, except a trigger's abort (see above). They refuse that write, are logged, and leave the
// writer's health alone, as before unit f1-eventlog.
const ownFailureReasons: ReadonlySet<string> = new Set([
  "database write failed: constraint",
  "event insert returned no sequence",
  "invalid piece side rows",
  "invalid piece side row",
  "writer transaction is closed",
]);
const countsAsOutage = (error: LogWriteFailed) =>
  !(error instanceof InvalidEvent) && !(error instanceof ReceiptUnreadable)
  && (error instanceof StorageFailed ? error.outage : !ownFailureReasons.has(error.reason));

// Adapted from isNoTransactionError in effect/sql/SqlClient (Effect 4.0.5,
// Copyright (c) 2023 Effectful Technologies Inc, MIT License; see
// NOTICE), as the copied SQLite client also does
// (third_party/t3code/node-sqlite, it is not exported): SQLite's answer to a
// ROLLBACK when no transaction is open. The property check is written out,
// where Effect uses its hasProperty helper.
const isNoTransactionError = (error: SqlError): boolean => {
  const cause: unknown = error.reason.cause;
  const message = typeof cause === "string" ? cause
    : typeof cause === "object" && cause !== null && "message" in cause && typeof cause.message === "string" ? cause.message : "";
  return /no transaction is active/i.test(message);
};

const failureReason = (error: SqlError): string => {
  const reason = error.reason;
  const errno = sqliteCode(error);
  if (errno !== undefined && (errno & 0xff) === SQLITE_FULL) return "database write failed: disk full";
  switch (reason._tag) {
    case "UniqueViolation":
    case "ConstraintError": return "database write failed: constraint";
    case "LockTimeoutError": return "database write failed: busy";
    default: return "database write failed";
  }
};

// Pattern: T3 Code apps/server/src/orchestration-v2/EventSink.ts
// (`sql.withTransaction`, lines 716 and 781; MIT licence). The shape was
// reused, no code.
export const makeLayer = (lists: EngineLists = engineLists) => Layer.effect(EventLog, Effect.gen(function* () {
  const Envelopes = Schema.Array(lists.envelope);
  const decodeEnvelopes = (value: unknown) => Schema.decodeUnknownSync(Envelopes)(value) as ReadonlyArray<EventEnvelope>;
  const sql = yield* SqlClient.SqlClient;
  const redactor = yield* Redactor.Redactor;
  const views = yield* Views.Views;
  // WAL lets readers run beside this single writer. The mode is stored in the
  // file; setting it again on an already-WAL file changes nothing.
  yield* sql`PRAGMA journal_mode = WAL`;
  const writer = yield* Semaphore.make(1);
  const committedEvents = yield* Effect.acquireRelease(
    PubSub.unbounded<EventEnvelope>(),
    PubSub.shutdown,
  );
  // Set when the live views and the log disagree in a way this service cannot
  // repair. Every later write is refused until a restart rebuilds the views.
  let diverged: { readonly error: LogWriteFailed; readonly since: number } | undefined;
  const diverge = (reason: string) => Effect.gen(function* () {
    if (diverged !== undefined) return;
    diverged = { error: new LogWriteFailed({ reason }), since: yield* Clock.currentTimeMillis };
    yield* Effect.logError("event log write failed").pipe(Effect.annotateLogs({ reason }));
  });
  // Called only after COMMIT, while the writer permit still orders batches.
  // The live views are updated first, so a subscriber that sees an event and
  // then reads the views finds the event applied. Neither step can fail.
  //
  // Issue 46: a defect in either step would leave the live views behind a
  // committed batch. The writer then refuses every write and reports that a
  // restart is needed, because a restart rebuilds the views from the log.
  const publishCommitted = (envelopes: ReadonlyArray<EventEnvelope>) =>
    views.applyCommitted(envelopes).pipe(
      Effect.andThen(PubSub.publishAll(committedEvents, envelopes)),
      Effect.asVoid,
      Effect.tapDefect(() => diverge("live views missed a committed batch")),
    );
  // Issue 1575. The writer used to keep its first failure for the life of the
  // service and refuse every later write with it, which hid a transient fault
  // for days. Now a failure is logged, reported by `health`, and refused from
  // memory only until `retryAt`; the next write after that goes to the
  // database again. These values are read and set only under the writer
  // permit.
  //
  // The latch protected one property: the live views never run ahead of or
  // behind the committed log. Views change only after a successful COMMIT, so
  // a failure cannot put them ahead. A COMMIT that reports failure might still
  // have stored its batch, which would put them behind. So before the first
  // write after a failure, `reconcile` compares the log's head with the views
  // and applies any committed events they lack, as a restart's rebuild would.
  // Views ahead of the log cannot be repaired here, and that state, like a
  // defect while applying a batch (issue 46), refuses every write until a
  // restart.
  let failure: {
    readonly first: LogWriteFailed;
    readonly since: number;
    readonly sqliteCode: number | undefined;
    latest: LogWriteFailed;
    failures: number;
    retryAt: number;
    reconciled: boolean;
  } | undefined;

  const noteFailure = (error: LogWriteFailed) => Effect.gen(function* () {
    outages.add(error);
    const now = yield* Clock.currentTimeMillis;
    if (failure === undefined) {
      failure = {
        first: error, since: now, sqliteCode: error instanceof StorageFailed ? error.sqliteCode : undefined,
        latest: error, failures: 1, retryAt: now + firstRetryMillis, reconciled: false,
      };
    } else {
      failure.latest = error;
      failure.failures += 1;
      failure.retryAt = now + Math.min(maxRetryMillis, firstRetryMillis * 2 ** Math.min(failure.failures - 1, 16));
      failure.reconciled = false;
    }
    const code = error instanceof StorageFailed ? error.sqliteCode : undefined;
    yield* Effect.logError("event log write failed").pipe(Effect.annotateLogs({
      reason: error.reason, failures: failure.failures, ...(code === undefined ? {} : { sqliteCode: code }),
    }));
  });

  const noteRecovered = Effect.suspend(() => {
    if (failure === undefined) return Effect.void;
    const failures = failure.failures;
    failure = undefined;
    return Effect.logWarning("event log writes recovered").pipe(Effect.annotateLogs({ failures }));
  });

  const reconcile = Effect.gen(function* () {
    const rows = yield* sql<{ head: number }>`SELECT COALESCE(MAX(sequence), 0) AS head FROM events`;
    const head = rows[0]?.head ?? 0;
    const applied: number = (yield* views.snapshot).sequence;
    if (head < applied) {
      yield* diverge("live views are ahead of the log");
      return yield* Effect.fail(diverged!.error);
    }
    if (head === applied) return;
    const missing = yield* sql<EventRow>`
      SELECT sequence, event_id, caller, caller_rule, request_id, agent_id, occurred_at, payload_json
      FROM events WHERE sequence > ${applied} ORDER BY sequence`;
    const envelopes = yield* Effect.forEach(missing, (row) => decodeRowWith(row, lists.envelope));
    yield* publishCommitted(envelopes);
  }).pipe(Effect.mapError((error) => isSqlError(error) ? new StorageFailed(error)
    : error instanceof LogWriteFailed ? error : new LogWriteFailed({ reason: error.reason })));

  // Errors that `admit` returns without asking the database. They were
  // recorded when they first happened and are not counted again.
  const refusedFromMemory = new WeakSet<LogWriteFailed>();
  // Every error recorded as a storage failure. A refusal from memory during
  // the pause is the recorded error itself, so it is here too.
  const outages = new WeakSet<LogWriteFailed>();
  const isOutage = (error: unknown) => diverged === undefined && error instanceof LogWriteFailed && outages.has(error);
  let held: HeldRunnerEvents = {};
  const noteHeld = (next: HeldRunnerEvents) => {
    held = (next.heldRunnerEvents ?? 0) > 0 ? { heldRunnerEvents: next.heldRunnerEvents!, heldAgents: next.heldAgents ?? 0 } : {};
  };
  const refuse = (error: LogWriteFailed) => { refusedFromMemory.add(error); return error; };

  // Runs under the writer permit before a transaction. A write inside the
  // pause is refused with the latest failure; the first one after it
  // reconciles the views with the log and then goes to the database.
  const admit = Effect.gen(function* () {
    if (diverged !== undefined) return yield* Effect.fail(refuse(diverged.error));
    if (failure === undefined) return;
    if ((yield* Clock.currentTimeMillis) < failure.retryAt) return yield* Effect.fail(refuse(failure.latest));
    if (!failure.reconciled) {
      // A failure here is recorded by `transact`, like a failed write.
      yield* reconcile.pipe(Effect.mapError((error) => error === diverged?.error ? refuse(error) : error));
      failure.reconciled = true;
    }
  });

  const health = Effect.sync((): WriteHealth => {
    if (failure === undefined && diverged === undefined) return { failing: false, ...held };
    if (failure === undefined) {
      return { failing: true, reason: diverged!.error.reason, since: diverged!.since, failures: 1, restartNeeded: true, restartReason: diverged!.error.reason, ...held };
    }
    return {
      failing: true, reason: failure.first.reason, since: failure.since,
      ...(failure.sqliteCode === undefined ? {} : { sqliteCode: failure.sqliteCode }),
      failures: failure.failures,
      ...(diverged === undefined ? { nextAttempt: failure.retryAt } : { restartNeeded: true as const, restartReason: diverged.error.reason }),
      ...held,
    };
  });

  const commit = (
    caller: string, requestId: RequestId, bodyHash: string, events: ReadonlyArray<Event>,
    sideRows: SideRows, actor: Caller, occurredAt: number,
    receiptResult?: false | ((envelopes: ReadonlyArray<EventEnvelope>) => unknown),
  ): Effect.Effect<{
    readonly envelopes: ReadonlyArray<EventEnvelope>;
    readonly isNew: boolean;
    readonly changed: boolean;
  }, RequestIdReused | LogWriteFailed | SqlError> =>
    sql.withTransaction(Effect.gen(function* () {
      const existing = yield* sql<{ body_hash: string; result_json: string }>`
        SELECT body_hash, result_json FROM receipts WHERE caller = ${caller} AND request_id = ${requestId}`;
      const receipt = existing[0];
      if (receipt !== undefined) {
        if (receipt.body_hash !== bodyHash) {
          return yield* Effect.fail(new RequestIdReused({ reason: "request id reused" }));
        }
        const envelopes = yield* Effect.try({
          try: () => {
            const result: unknown = JSON.parse(receipt.result_json);
            return decodeEnvelopes(Array.isArray(result) ? result
              : Schema.decodeUnknownSync(Schema.Struct({ envelopes: Envelopes }))(result).envelopes);
          },
          catch: () => new ReceiptUnreadable(),
        });
        return { envelopes, isNew: false, changed: false };
      }

      const envelopes: Array<EventEnvelope> = [];
      for (const event of events) {
        const eventId = randomUUID();
        const agentId = "agentId" in event ? event.agentId : undefined;
        const rows = yield* sql<{ sequence: number }>`
          INSERT INTO events (event_id, caller, caller_rule, request_id, agent_id, thread_agent_id, event_type, occurred_at, payload_json)
          VALUES (${eventId}, ${caller}, ${actor._tag === "service" ? actor.rule ?? null : null}, ${requestId}, ${agentId ?? null}, ${threadAgent(event) ?? null}, ${event._tag}, ${occurredAt}, ${JSON.stringify(event)})
          RETURNING sequence`;
        const row = rows[0];
        if (row === undefined) return yield* Effect.fail(new LogWriteFailed({ reason: "event insert returned no sequence" }));
        envelopes.push(Schema.decodeUnknownSync(lists.envelope)({
          sequence: row.sequence, eventId, occurredAt, caller: actor, requestId,
          ...(agentId === undefined ? {} : { agentId }), event,
        }) as EventEnvelope);
      }
      if (sideRows.revokeAgentTokens === "all") {
        yield* sql`UPDATE agent_tokens SET revoked_at = ${occurredAt} WHERE revoked_at IS NULL`;
      } else {
        for (const agentId of sideRows.revokeAgentTokens ?? []) {
          yield* sql`UPDATE agent_tokens SET revoked_at = ${occurredAt}
            WHERE agent_id = ${agentId} AND revoked_at IS NULL`;
        }
      }
      for (const token of sideRows.agentTokens ?? []) {
        yield* sql`INSERT INTO agent_tokens (token_hash, agent_id, revoked_at)
          VALUES (${token.tokenHash}, ${token.agentId}, ${token.revokedAt})`;
      }
      for (const row of sideRows.processes ?? []) {
        yield* sql`INSERT INTO processes (agent_id, group_id, boot_id, start_time, user_id, executable)
          VALUES (${row.agentId}, ${row.groupId}, ${row.bootId}, ${row.startTime}, ${row.userId}, ${row.executable})`;
      }
      // Names are fixed by migrations. Values stay bound parameters, and every
      // side row shares this transaction with its events and receipt.
      for (const [table, rows] of Object.entries(sideRows)) {
        if (!/^(client|board|roles)_[A-Za-z0-9_]+$/.test(table)) continue;
        if (rows === undefined) continue;
        if (!Array.isArray(rows)) return yield* new LogWriteFailed({ reason: "invalid piece side rows" });
        for (const row of rows) {
          if (row === null || typeof row !== "object") return yield* new LogWriteFailed({ reason: "invalid piece side row" });
          yield* sql`INSERT INTO ${sql(table)} ${sql.insert(row as Record<string, unknown>)}`;
        }
      }
      if (receiptResult !== false) {
        const result = receiptResult === undefined ? envelopes
          : { envelopes, reply: yield* redactReceipt(receiptResult(envelopes), receiptStructure) };
        yield* sql`INSERT INTO receipts (caller, request_id, body_hash, result_json, first_sequence)
          VALUES (${caller}, ${requestId}, ${bodyHash}, ${JSON.stringify(result)}, ${envelopes[0]?.sequence ?? null})`;
      }
      // Every statement above that ran was a write that committed with the batch.
      const pieceRows = Object.entries(sideRows).filter(([table]) => /^(client|board|roles)_[A-Za-z0-9_]+$/.test(table))
        .reduce((total, [, rows]) => total + (Array.isArray(rows) ? rows.length : 0), 0);
      const statements = events.length + (receiptResult === false ? 0 : 1)
        + (sideRows.revokeAgentTokens === "all" ? 1 : sideRows.revokeAgentTokens?.length ?? 0)
        + (sideRows.agentTokens?.length ?? 0) + (sideRows.processes?.length ?? 0) + pieceRows;
      return { envelopes, isNew: true, changed: statements > 0 };
    }));

  // Replies share the event rule: preserve keys, tags and typed identities,
  // and redact all other string values, including values in free-text records.
  const redactReceipt = (value: unknown, structure: RedactionStructure | undefined): Effect.Effect<unknown> => Effect.gen(function* () {
    if (typeof value === "string") return structure?.identity === true ? value : yield* redactor.redactText(value);
    if (Array.isArray(value)) return yield* Effect.forEach(value, (item) => redactReceipt(item, structure?.elements));
    if (typeof value === "object" && value !== null) {
      const entries = yield* Effect.forEach(Object.entries(value), ([key, item]) => Effect.gen(function* () {
        return [key, yield* redactReceipt(item, fieldStructure(structure, key, "_tag" in value ? value._tag : undefined))] as const;
      }));
      return Object.fromEntries(entries);
    }
    return value;
  });

  // Identifiers and token hashes are not free text and are left alone.
  const redactSideRows = (sideRows: SideRows): Effect.Effect<SideRows> => Effect.gen(function* () {
    const processes = yield* Effect.forEach(sideRows.processes ?? [], (row) =>
      Effect.map(redactor.redactText(row.executable), (executable): ProcessRow => ({ ...row, executable })));
    return { ...sideRows, processes };
  });

  // Item 2 of review round 1 and item 6 of review round 2. Effect's
  // `makeWithTransaction` (effect/src/sql/SqlClient.ts, lines 448 to 472 of
  // version 4.0.5) turns two paths into a defect:
  //
  // - A COMMIT fails and the cleanup ROLLBACK fails too. The copied SQLite
  //   client then marks the connection poisoned
  //   (third_party/t3code/node-sqlite/src/nodeSqliteClient.ts, lines 1141
  //   to 1173), and every later hand-out of the connection retries the
  //   ROLLBACK first and fails until it succeeds (`usable`, called at line
  //   1283). The connection heals by itself, but the batch may or may not be
  //   stored, so this is a storage failure, "rollback failed", and the next
  //   write after the pause reconciles first.
  // - The body fails and the ROLLBACK after it fails. Effect raises the
  //   ROLLBACK's error with `Effect.orDie` and drops the body's failure. The
  //   copied client does not poison the connection on this path, because
  //   its `onCommitFailure` hook runs only after a COMMIT. If the ROLLBACK
  //   failed because SQLite had already rolled the transaction back (a
  //   statement error such as a full disk can do that), the connection is
  //   clean and the body's failure is the answer. Otherwise the transaction
  //   may still be open, every later BEGIN IMMEDIATE on the writer
  //   connection fails, and only a restart opens a fresh connection.
  //
  // Any other defect stays a defect.
  const failedRollback = (defect: SqlError, bodyFailure: { readonly error: unknown } | undefined) => Effect.gen(function* () {
    if (bodyFailure === undefined) {
      return yield* Effect.fail(new StorageFailed(defect, "database write failed: rollback failed", { outage: true }));
    }
    const first = bodyFailure.error;
    // SQLite had already rolled back: the body's failure stands as it was.
    if (isNoTransactionError(defect)) return yield* Effect.fail(first as never);
    yield* diverge("database write failed: transaction left open");
    return yield* Effect.fail(first instanceof LogWriteFailed && !(first instanceof InvalidEvent) && !(first instanceof ReceiptUnreadable)
      ? new StorageFailed(defect, first.reason, { sqliteCode: first instanceof StorageFailed ? first.sqliteCode : undefined, outage: true })
      : new StorageFailed(defect, "database write failed: rollback failed", { outage: true }));
  });

  const transact: Context.Service.Shape<typeof EventLog>["transact"] = (use) =>
    // Waiting for the writer permit remains interruptible. Everything after it
    // is not (review round 1, item 1): a caller's cancellation must not split
    // a successful COMMIT from its publication, must not split the views'
    // update from the publication during `reconcile`, and must not skip
    // recording a failure, or the next write would skip `reconcile`.
    writer.withPermits(1)(Effect.uninterruptible(Effect.gen(function* () {
      yield* admit;
      const published: Array<EventEnvelope> = [];
      let active = true;
      // Set by any append that committed a change: events, a receipt, or side
      // rows. Only such a write proves that writing works again (item 5).
      let changed = false;
      // Item 6: the body's own failure, kept because Effect replaces it with
      // a defect when the ROLLBACK after it fails too.
      let bodyFailure: { readonly error: unknown } | undefined;
      const append: Context.Service.Shape<typeof EventLog>["append"] = (actor, requestId, bodyHash, events, sideRows, receiptResult) =>
        Effect.gen(function* () {
          if (!active) return yield* new LogWriteFailed({ reason: "writer transaction is closed" });
          const redacted = yield* Effect.forEach(events, (event) => redactor.redact(event));
          // Nothing transforms an event after this check. A registry refresh
          // or expanding redaction must not store an invalid role document or
          // cross either of its admission budgets. This is an input refusal,
          // not a database fault, and must not latch the writer.
          for (const event of redacted) {
            if (event._tag === "roles.RoleFileSet") yield* loadRoleFile(event.content).pipe(
              Effect.mapError(() => new InvalidEvent({ reason: "invalid redacted role file" })),
            );
          }
          const redactedSideRows = yield* redactSideRows(sideRows);
          const occurredAt = yield* Clock.currentTimeMillis;
          const batch = yield* commit(callerKey(actor), requestId, bodyHash, redacted, redactedSideRows, actor, occurredAt, receiptResult).pipe(
            Effect.mapError((error) => error._tag === "SqlError" ? new StorageFailed(error) : error),
          );
          if (batch.isNew) published.push(...batch.envelopes);
          if (batch.changed) changed = true;
          return batch.envelopes;
        });
      const body = Effect.suspend(() => use(append)).pipe(
        Effect.tapError((error) => Effect.sync(() => { bodyFailure ??= { error }; })),
      );
      const result = yield* sql.withTransaction(body).pipe(
        Effect.ensuring(Effect.sync(() => { active = false; })),
        Effect.catchDefect((defect) => isSqlError(defect) ? failedRollback(defect, bodyFailure) : Effect.die(defect)),
        Effect.mapError((error) => isSqlError(error) ? new StorageFailed(error) : error),
      );
      if (published.length > 0) yield* publishCommitted(published);
      if (changed) yield* noteRecovered;
      return result;
    }).pipe(
      // Refused requests, invalid events and unreadable receipts are not an
      // infrastructure fault and leave the writer's health alone. A refusal
      // from memory by `admit` is not a new failure either.
      //
      // Review round 2, item 1: a failure of the write's own, such as a
      // constraint, does not mark the database as failing either. Before
      // this, one agent event that could never be written was retried for
      // ever and drove the pause to its cap for every command.
      Effect.tapError((error) => error instanceof LogWriteFailed && countsAsOutage(error) && !refusedFromMemory.has(error)
        ? noteFailure(error)
        : Effect.void),
    )));

  const append: Context.Service.Shape<typeof EventLog>["append"] = (...args) =>
    transact((write) => write(...args));

  return EventLog.of({
    lists,
    append,
    redactor,
    redactText: redactor.redactText,
    transact,
    subscribeCommitted: PubSub.subscribe(committedEvents),
    withWriterPermit: writer.withPermits(1),
    health,
    requireRestart: diverge,
    isOutage,
    noteHeld,
  });
}));

export const layer = Layer.unwrap(Effect.sync(() => makeLayer()));
