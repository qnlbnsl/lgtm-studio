import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { pathToFileURL } from "node:url";
import { it } from "@effect/vitest";
import * as Effect from "effect/Effect";
import { afterEach, beforeEach, expect, vi } from "vitest";
import { accountFor, deliveryFor, echoFile, fakeAgentPath, harnessFor, makeRunnerRig, removeRunnerRig, spawnFor, withAcpRunner, writeFakeAdapter, type RunnerRig } from "../support/acpRunnerRig.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";
registerRebuildCheck({ noLog: true });
let rig: RunnerRig;
beforeEach(() => { rig = makeRunnerRig(); vi.stubEnv("HOME", rig.home); vi.stubEnv("LGTM_HOME", rig.stateFolder); });
afterEach(() => { vi.unstubAllEnvs(); removeRunnerRig(rig); });
const permission = { step: "permission", toolCallId: "r5-permission", title: "Safe permission", options: [{ optionId: "allow-once", name: "Allow once", kind: "allow_once" }] };
const privateValue = "e-s19-fake-long-value";
for (const withoutWithholdParts of [false, true]) for (const cap of ["count", "bytes"] as const) {
  it.live(`r5 ${cap} overflow cannot swallow permission control or end its pending turn (withoutWithholdParts=${withoutWithholdParts})`, () =>
    withAcpRunner(rig, { withoutWithholdParts, secrets: { "secret:r5": privateValue } }, context => Effect.gen(function* () {
      const reports = Array.from({ length: cap === "count" ? 1025 : 1 }, (_, i) => ({ step: "tool_call", toolCallId: `r5-tool-${i}`, title: "Safe tool", kind: "read", status: "completed", output: cap === "bytes" ? "x".repeat(1024 * 1024 + 1) + "." : "Safe output." }));
      const script = join(rig.root, "permission-script.json");
      writeFileSync(script, JSON.stringify({ turns: [[{ step: "text", text: "safe: e" }, ...reports, permission, { step: "text", text: privateValue.slice(1) + "." }]] }));
      const adapter = writeFakeAdapter(rig);
      writeFileSync(adapter, `#!${process.execPath}\nimport { readFileSync } from "node:fs";\nprocess.env.FAKE_AGENT_SCRIPT = JSON.stringify({ ...JSON.parse(process.env.FAKE_AGENT_SCRIPT), ...JSON.parse(readFileSync(${JSON.stringify(script)}, "utf8")) });\nawait import(${JSON.stringify(pathToFileURL(fakeAgentPath).href)});\n`);
      const spawn = yield* spawnFor(rig, harnessFor("first", adapter), accountFor("first", {}, echoFile(rig)));
      yield* context.runner.start(spawn.input);
      yield* context.runner.prompt({ agentId: spawn.agentId, incarnation: spawn.incarnation, delivery: deliveryFor(spawn.agentId, "go"), sentSequence: 1 });
      const request = yield* context.waitFor(event => event._tag === "permissionRequested");
      expect(request._tag).toBe("permissionRequested");
      expect(context.events.some(event => event._tag === "turnEnded")).toBe(false);
      expect(context.events.filter(event => event._tag === "deliveryUpdate" && event.kind === "tool-call")).toHaveLength(reports.length);
      expect(JSON.stringify(context.events).includes(privateValue)).toBe(false);
      if (request._tag === "permissionRequested") yield* context.runner.answerPermission({ agentId: spawn.agentId, requestId: request.permissionRequestId, optionId: "allow-once" });
      yield* context.waitFor(event => event._tag === "turnEnded");
      expect(context.events.filter(event => event._tag === "permissionRequested")).toHaveLength(1);
      expect(JSON.stringify(context.events).includes(privateValue.slice(1))).toBe(false);
    })), 60_000);
}

it.live("r5 an unreadable login store refuses permission identifiers and the adapter's RPC failure ends its turn", () => {
  const config = join(rig.home, ".claude"); mkdirSync(config, { recursive: true });
  const store = join(config, ".credentials.json"); writeFileSync(store, JSON.stringify({ token: "s19-fake-login-value" }));
  return withAcpRunner(rig, {}, context => Effect.gen(function* () {
    const gate = join(rig.root, "request-after-loss"), echo = echoFile(rig);
    const spawn = yield* spawnFor(rig, harnessFor("claude", writeFakeAdapter(rig)), accountFor("claude", { turns: [[{ step: "mode_change", configId: "mode", value: "read-only" }, { step: "wait_for_file", path: gate }, permission]] }, echo));
    yield* context.runner.start(spawn.input);
    yield* context.runner.prompt({ agentId: spawn.agentId, incarnation: spawn.incarnation, delivery: deliveryFor(spawn.agentId, "go"), sentSequence: 1 });
    yield* context.waitFor(event => event._tag === "modeReported");
    writeFileSync(store, "{broken"); writeFileSync(gate, "go");
    yield* context.waitFor(event => event._tag === "turnEnded");
    expect(context.events.filter(event => event._tag === "permissionRequested")).toHaveLength(0);
    const inbound = (JSON.parse(readFileSync(echo, "utf8")) as { inbound: Array<{ error?: { code: number } }> }).inbound;
    expect(inbound.some(frame => frame.error?.code === -32602)).toBe(true);
    expect(context.events.some(event => event._tag === "promptReply" && event.outcome._tag === "error")).toBe(true);
  }));
}, 60_000);
