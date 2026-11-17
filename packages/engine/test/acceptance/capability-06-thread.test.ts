// Capability 6: read any agent's thread live, from any client: every message,
// tool call, changed file and permission request. Criteria 8.3 to 8.5.
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { groupThreadItems } from "@lgtm/contracts/thread";
import { afterEach, expect } from "@effect/vitest";
import * as Effect from "effect/Effect";
import * as Fiber from "effect/Fiber";

import { eventsUntil, isTag, redacted, requestId, scenario, stopAllServices, tagged } from "../support/acceptance.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";

registerRebuildCheck({ noLog: true });
afterEach(async () => { await stopAllServices(); });

type Content = { _tag: string; text?: string; title?: string; status?: string; output?: string; path?: string; diff?: string };
const contents = (events: ReadonlyArray<{ sequence: number; agentId?: string; event: { _tag: string } }>, agentId: string): ReadonlyArray<Content> =>
  groupThreadItems(events).filter((envelope) => envelope.agentId === agentId && envelope.event._tag === "ThreadItem")
    .map((envelope) => (envelope.event as unknown as { content: Content }).content);

scenario("6 thread: the owner reads a message, a plan, a tool call, a changed file and a permission request as they happen (breaks if the runner drops any one of the five kinds or the stream withholds them until the turn ends)", (w) => Effect.gen(function* () {
  // The fixed owner credential keeps this ordering fixture reproducible.
  // Other registered credentials, including the random web-session token,
  // can still split a message into deltas sharing one item identifier.
  mkdirSync(w.root.state, { mode: 0o700 });
  writeFileSync(join(w.root.state, "owner-credential"), Buffer.alloc(32, 7), { mode: 0o600 });
  yield* w.boot();
  const gate = join(w.root.root, "gate-thread");
  const before = yield* w.watermark();
  const agent = yield* w.start(w.owner, {
    name: "Narrator", permission: "ask", prompt: "Show your work.",
    script: { turns: [[
      { step: "wait_for_file", path: gate },
      { step: "text", text: "I will read the file." },
      { step: "plan", entries: [{ content: "Read it", priority: "high", status: "in_progress" }] },
      { step: "tool_call", toolCallId: "call-1", title: "Read notes.md", kind: "read", status: "completed", output: "twelve lines" },
      { step: "file_change", toolCallId: "call-2", path: "/tmp/notes.md", operation: "update", patch: "--- a\n+++ b\n@@\n-old\n+new\n" },
      { step: "permission", toolCallId: "call-3", title: "Write notes.md", options: [
        { optionId: "allow-once", name: "Allow once", kind: "allow_once" }, { optionId: "reject-once", name: "Reject", kind: "reject_once" },
      ] },
    ]] },
  });
  // The reader is already subscribed when the first item is produced.
  const reader = yield* Effect.forkChild(eventsUntil(w.owner, before, "the permission request", isTag("PermissionRequested", agent.agentId)));
  writeFileSync(gate, "go");
  const seen = yield* Fiber.join(reader);
  expect(tagged(seen, "TurnEnded", agent.agentId), "the stream delivered the request while the turn was still open").toHaveLength(0);
  const items = contents(seen, agent.agentId);
  expect(items.map((item) => item._tag)).toEqual(["text", "plan", "tool-call", "file-change"]);
  expect(items[0]).toMatchObject({ text: "I will read the file." });
  expect(items[2]).toMatchObject({ title: "Read notes.md", status: "completed" });
  expect(items[2]!.output).toContain("twelve lines");
  expect(items[3]).toMatchObject({ path: "/tmp/notes.md" });
  expect(items[3]!.diff).toContain("+new");
  const request = tagged(seen, "PermissionRequested", agent.agentId)[0]!.event as unknown as { permissionRequestId: string; description: string; options: ReadonlyArray<{ id: string }> };
  expect(request.description).toContain("Write notes.md");
  expect(request.options.map((option) => option.id)).toEqual(["allow-once", "reject-once"]);

  // Let the agent finish: the answer is the last piece of the thread.
  yield* w.owner.answer({ requestId: requestId(), kind: "permission", id: request.permissionRequestId, optionId: "allow-once" } as never);
  const finished = yield* eventsUntil(w.owner, before, "the turn end", isTag("TurnEnded", agent.agentId));
  expect(contents(finished, agent.agentId).some((item) => item.text === "permission:selected:allow-once")).toBe(true);
}), { manual: true });

scenario("6 thread: a client that connects after the turn replays the same thread from the log, and an agent's environment printed into its thread never shows its token (breaks if replay differs from live, or a secret in agent output reaches an event)", (w) => Effect.gen(function* () {
  const before = yield* w.watermark();
  const agent = yield* w.start(w.owner, {
    name: "Leaky", permission: "ask", prompt: "Print everything.",
    script: { turns: [[{ step: "text", text: "start" }, { step: "print_env" }, { step: "text", text: "end" }]] },
  });
  const live = yield* eventsUntil(w.owner, before, "the turn end", isTag("TurnEnded", agent.agentId));
  // The token the engine minted for the agent is registered as a secret: the
  // closing check fails the scenario if it appears in any event.
  const { token } = yield* w.token(agent);
  expect(token.length).toBeGreaterThan(8);
  const replay = yield* eventsUntil(w.owner, before, "the replayed turn end", isTag("TurnEnded", agent.agentId));
  expect(contents(replay, agent.agentId)).toEqual(contents(live, agent.agentId));
  const printed = contents(live, agent.agentId).map((item) => item.text ?? "").join("");
  expect(printed, "the step did not print the environment, so the redaction was not exercised").toContain("LGTM_TOKEN=");
  expect(printed).toContain("start");
  expect(printed).toContain("end");
  expect(printed).not.toContain(token);
}));

scenario("r5 a held final character appends to the same message item at turn end, live and on replay", (w) => Effect.gen(function* () {
  const sentence = "A complete sentence", privateValue = "e-s19-fake-tail-private-value";
  const gate = join(w.root.root, "finish-held-character");
  const prepared = yield* w.prepare({ script: { turns: [[{ step: "text", text: sentence }, { step: "wait_for_file", path: gate }]] }, prompt: "Speak." });
  w.secrets.add(privateValue);
  yield* w.owner["accounts set-secret"]({ requestId: requestId(), accountId: prepared.accountId, name: "FAKE_API_KEY", nonce: "r5-tail", value: redacted(privateValue) } as never);
  const before = yield* w.watermark();
  const agent = yield* w.owner.start(prepared.request as never);
  const partial = yield* eventsUntil(w.owner, before, "the decided message prefix", isTag("ThreadItem", agent.agentId));
  const prefix = contents(partial, agent.agentId).filter(item => item._tag === "text").map(item => item.text).join("");
  expect(sentence.startsWith(prefix)).toBe(true);
  expect(prefix.length).toBeLessThan(sentence.length);
  expect(tagged(partial, "TurnEnded", agent.agentId)).toHaveLength(0);
  writeFileSync(gate, "go");
  const finished = yield* eventsUntil(w.owner, before, "the completed message", isTag("TurnEnded", agent.agentId));
  const deltas = finished.flatMap(row => row.agentId === agent.agentId && row.event._tag === "ThreadItem" && row.event.content._tag === "text" ? [row.event] : []);
  expect(deltas).toHaveLength(2);
  expect(new Set(deltas.map(event => event.itemId)).size).toBe(1);
  expect(deltas.map(event => event.content._tag === "text" ? event.content.text : "").join("")).toBe(sentence);
  expect(contents(finished, agent.agentId).filter(item => item._tag === "text")).toEqual([{ _tag: "text", text: sentence }]);
  const replay = yield* eventsUntil(yield* w.reopenOwner(), before, "the replayed completed message", isTag("TurnEnded", agent.agentId));
  expect(contents(replay, agent.agentId)).toEqual(contents(finished, agent.agentId));
}));
