// Failure rule 8: the disk or the database fails. The engine refuses new
// commands and says why. It never acts without writing first, so nothing
// happens that the log does not show. Criteria 8.3 to 8.5.
//
// The failure is a trigger planted in the event database that aborts the
// insert of one kind of event, which the engine meets as a failed write. A
// full disk and a busy database are reported by the same path with other
// reasons; their wording is checked by the log's own tests
// (test/log/disk-failure.test.ts), not here.
import { existsSync } from "node:fs";
import { join } from "node:path";
import { afterEach, expect } from "@effect/vitest";
import type { EventEnvelope } from "@lgtm/contracts";
import * as Effect from "effect/Effect";

import {
  eventsUntil, isTag, peekEcho, plantDatabaseFault, promptsOf, refusal, requestId, runSql, scenario, since, stopAllServices, tagged, threadText, waitFor,
} from "../support/acceptance.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";

registerRebuildCheck({ noLog: true });
afterEach(async () => { await stopAllServices(); });

const abortInserts = (name: string, eventType: string) =>
  `CREATE TRIGGER ${name} BEFORE INSERT ON events WHEN NEW.event_type = '${eventType}' BEGIN SELECT RAISE(ABORT, 'planted failure'); END`;

const options = [{ optionId: "allow-once", name: "Allow once", kind: "allow_once" }];

// Issue 1575: the refusal used to last for the life of the service, with no
// sign. A failed write is now reported by the owner's version read, refused
// from memory for a short pause, and retried against the database after it,
// so the service recovers once the fault is gone, without a restart.
scenario("8 disk: when a message cannot be written the call is refused with the database failure, nothing reaches the agent, the version read reports the failing writes while reading still works, and once the fault is gone the same request acts once without a restart (breaks if the engine sends the message before writing it, hides the reason or the failure, or never takes writes again)", (w) => Effect.gen(function* () {
  const agent = yield* w.start(w.owner, { name: "Listener", permission: "ask", prompt: "Hello." });
  yield* eventsUntil(w.owner, 0, "the first turn", isTag("TurnEnded", agent.agentId));
  const before = promptsOf(peekEcho(agent.echo)!).length;
  const mark = yield* w.watermark();
  const version = () => w.owner["client.service.version"]({ requestId: requestId() } as never) as unknown as Effect.Effect<{ writes?: Record<string, unknown> }, unknown>;
  expect((yield* version()).writes).toEqual({ failing: false });

  // The planted fault refuses every event, so the second command below is
  // refused whether it arrives during the writer's pause or after it. It is
  // a storage error (SQLite's "string or blob too big"), as a full disk
  // would give, not a constraint, which belongs to one write and does not
  // mark the database as failing (review round 2 of unit f1-eventlog, item 1).
  plantDatabaseFault(w.root, "CREATE TRIGGER planted_queue BEFORE INSERT ON events BEGIN SELECT zeroblob(2000000000); END", "SELECT 1");
  const lost = requestId();
  const failedAt = Date.now();
  const first = yield* refusal(w.owner.prompt({ requestId: lost, agentId: agent.agentId, text: "Lost." } as never));
  expect(first, "a message was accepted although it could not be written").toMatch(/^database write failed/);

  // A later command, whatever it is, gets the same answer and does not act.
  const second = yield* refusal(w.owner.group({ requestId: requestId(), agentId: agent.agentId, add: ["late"], remove: [] } as never));
  expect(second, "the engine kept taking commands after a failed write").toBe(first);

  // The owner can see that writes are failing, with the first failure's
  // fixed reason and time. Reading still works, and shows that nothing happened.
  const failing = (yield* version()).writes!;
  expect(failing).toMatchObject({ failing: true, reason: first, failures: 1 });
  expect(failing.since as number).toBeGreaterThanOrEqual(failedAt - 1_000);
  expect(failing.nextAttempt as number).toBeGreaterThan(failing.since as number);
  const quiet = yield* since(w.owner, mark);
  expect(tagged(quiet, "DeliveryQueued"), "a message that could not be written is in the log").toHaveLength(0);
  expect(tagged(quiet, "DeliverySent")).toHaveLength(0);
  expect(tagged(quiet, "AgentGrouped"), "a command refused for the database failure still acted").toHaveLength(0);
  expect(promptsOf(peekEcho(agent.echo)!), "the agent received a message the log does not show").toHaveLength(before);

  // The fault goes away while the service runs. After the writer's pause
  // the same request is taken, once, and the report is healthy again.
  runSql(w.root, "DROP TRIGGER planted_queue");
  const reply = yield* waitFor("the resent message to be taken after the pause", () =>
    Effect.runPromise(Effect.result(w.owner.prompt({ requestId: lost, agentId: agent.agentId, text: "Lost." } as never)))
      .then((result) => result._tag === "Success" ? result.success as { deliveryId: string } : undefined));
  yield* eventsUntil(w.owner, mark, "the resent message to be taken", (envelope) =>
    envelope.event._tag === "DeliveryAccepted" && envelope.agentId === agent.agentId
    && (envelope.event as unknown as { deliveryId: string }).deliveryId === reply.deliveryId, 30_000);
  expect((yield* version()).writes).toEqual({ failing: false });
  const settled = yield* since(w.owner, mark);
  expect(tagged(settled, "DeliveryQueued").filter((envelope) =>
    (envelope.event as unknown as { delivery: { id: string } }).delivery.id === reply.deliveryId), "the resent request queued more than once").toHaveLength(1);
  const received = yield* waitFor("the adapter's inbound prompts", () => {
    const texts = promptsOf(peekEcho(agent.echo)!);
    return texts.includes("Lost.") ? texts : undefined;
  });
  expect(received.filter((text) => text === "Lost.")).toHaveLength(1);

  // A restart after the fault keeps exactly what was written: the resent
  // request still acts once, and the refused command never acted.
  yield* w.kill("SIGKILL");
  yield* w.boot();
  const replayed = (yield* w.owner.prompt({ requestId: lost, agentId: agent.agentId, text: "Lost." } as never)) as { deliveryId: string };
  expect(replayed.deliveryId, "a restart forgot the request's one outcome").toBe(reply.deliveryId);
  const history = yield* since(w.owner, mark);
  expect(tagged(history, "DeliveryQueued").filter((envelope) =>
    (envelope.event as unknown as { delivery: { id: string } }).delivery.id === reply.deliveryId), "the request queued again after a restart").toHaveLength(1);
  expect(tagged(history, "AgentGrouped"), "a refused command acted after a restart").toHaveLength(0);
}));

// What the fake adapter writes when the script reaches an `exec` step: a file
// that only the adapter can create, so the scenario can see from outside the
// service that the adapter went on after an answer (the engine refuses writes
// after the failed write and cannot report anything itself).
const marker = (path: string) => ({
  step: "exec",
  argv: ["-e", "require('node:fs').writeFileSync(process.argv[1], 'x')", path],
});

// The protocol client numbers its own requests from 2 ** 30, so the answer to a
// permission request is recognised by its shape, a response whose result holds
// an outcome, not by a number.
const answersIn = (echoFile: string): number => (peekEcho(echoFile)?.inbound ?? []).filter((frame) => {
  const result = (frame as { result?: { outcome?: unknown } }).result;
  return result !== undefined && result !== null && typeof result === "object" && result.outcome !== undefined;
}).length;

scenario("8 disk: when the answer to a permission request cannot be written the answer does not reach the agent, and the request is still open in the log (breaks if the answer is sent to the agent before it is written, so the agent acts on a decision the log does not hold)", (w) => Effect.gen(function* () {
  const first = join(w.root.root, "after-first-answer");
  const second = join(w.root.root, "after-second-answer");
  const agent = yield* w.start(w.owner, {
    name: "Asker", permission: "ask", prompt: "Go.",
    script: { turns: [[
      { step: "permission", toolCallId: "call-one", title: "Write the first file", options },
      marker(first),
      { step: "permission", toolCallId: "call-two", title: "Write the second file", options },
      marker(second),
      { step: "text", text: "end of the turn" },
    ]] },
  });
  const idOf = (events: ReadonlyArray<EventEnvelope>, index: number) =>
    (tagged(events, "PermissionRequested", agent.agentId)[index]!.event as unknown as { permissionRequestId: string }).permissionRequestId;

  // Positive control: an answer that is written reaches the adapter, and the
  // matcher below sees it. Without this a matcher that never matches passes.
  const firstRequested = yield* eventsUntil(w.owner, 0, "the first request", isTag("PermissionRequested", agent.agentId));
  yield* w.owner.answer({ requestId: requestId(), kind: "permission", id: idOf(firstRequested, 0), optionId: "allow-once" } as never);
  yield* waitFor("the adapter to go on after the first answer", () => existsSync(first) ? true : undefined);
  expect(answersIn(agent.echo), "the matcher did not see the answer that was written and delivered").toBe(1);

  const requested = yield* eventsUntil(w.owner, 0, "the second request", (envelope) =>
    envelope.event._tag === "PermissionRequested" && envelope.agentId === agent.agentId && envelope.sequence > firstRequested[firstRequested.length - 1]!.sequence);
  const id = idOf(requested, 1);
  const mark = yield* w.watermark();

  plantDatabaseFault(w.root, abortInserts("planted_answer", "PermissionAnswered"), "SELECT 1");
  const failed = yield* refusal(w.owner.answer({ requestId: requestId(), kind: "permission", id, optionId: "allow-once" } as never));
  expect(failed, "an answer was accepted although it could not be written").toMatch(/^database write failed/);

  // The adapter-side barrier: a bounded quiet window in which an answer that
  // was sent before it was written would make the adapter write its marker and
  // record a second response frame. Neither may happen.
  yield* Effect.sleep(2_000);
  expect(existsSync(second), "the adapter went on after an answer the log does not hold").toBe(false);
  expect(answersIn(agent.echo), "a permission answer reached the adapter although it could not be written").toBe(1);

  // Reading still works and shows the request open and the agent still waiting.
  const quiet = yield* since(w.owner, mark);
  expect(tagged(quiet, "PermissionAnswered")).toHaveLength(0);
  expect(tagged(quiet, "TurnEnded", agent.agentId), "the agent went on after an answer the log does not hold").toHaveLength(0);
  expect(threadText(quiet, agent.agentId)).not.toContain("permission:selected");

  // Recovery is a restart with the fault removed (the final check needs a
  // service that takes commands). The log still holds exactly the one answer.
  yield* w.kill("SIGKILL");
  runSql(w.root, "DROP TRIGGER planted_answer");
  yield* w.boot();
  const history = yield* since(w.owner, 0);
  expect(tagged(history, "PermissionAnswered", agent.agentId), "the log holds an answer that was never written")
    .toHaveLength(1);
}));
