import { existsSync, mkdirSync, renameSync, symlinkSync } from "node:fs";
import { join } from "node:path";
import { it } from "@effect/vitest";
import { afterEach, beforeEach, expect, vi } from "vitest";
import * as Effect from "effect/Effect";
import * as Redacted from "effect/Redacted";
import { SecretStore } from "../../src/commands/SecretStore.ts";
import { Views } from "../../src/views/Views.ts";
import { projectView } from "../../src/project/view.ts";
import { provideRestart, recover, ready } from "../support/restartRig.ts";
import { makeFakeRunner, type FakeRunner } from "../support/fakeRunner.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";
import { agentCredential, appendSetup, call, eventOf, failureOf, makeFakeSecrets, makeRig, ownerCredential, provideRig, readLog, removeRig, seedAgent, success, type Rig } from "../support/handlerRig.ts";
import { workerPayload, workerProject } from "../support/workerWorktrees.ts";

let rig: Rig;
beforeEach(() => {
  rig = makeRig();
  const work = join(rig.home, "repository"); renameSync(rig.work, work); rig = { ...rig, work };
  vi.stubEnv("HOME", rig.home); vi.stubEnv("LGTM_HOME", rig.stateFolder);
});
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllEnvs(); removeRig(rig); });
const snapshot = Effect.gen(function* () { return yield* (yield* Views).snapshot; });
const run = <A, E, R>(program: (runner: FakeRunner) => Effect.Effect<A, E, R>) => Effect.gen(function* () {
  const runner = yield* makeFakeRunner(), secrets = makeFakeSecrets();
  return yield* provideRig(rig, { runner, secrets }, Effect.gen(function* () {
    yield* registerRebuildCheck(); return yield* program(runner);
  }).pipe(Effect.provide(secrets.layer)));
}).pipe(Effect.scoped, Effect.orDie);

for (const permission of ["read-only", "ask", "edit", "auto", "full"] as const) {
  it.live(`s26: an omitted worker permission respects a ${permission} starter and leaves its level unchanged`, () => run(runner => Effect.gen(function* () {
    const p = yield* workerProject(rig);
    const lead = yield* seedAgent({ id: "level-lead", role: "lead:team", folder: p.path, parent: p.lead.agentId, permission });
    const payload = workerPayload(p.path); delete (payload as Partial<typeof payload>).permission;
    const child = success(yield* call(agentCredential(lead.token), "start", payload));
    const state = yield* snapshot;
    const expected = permission === "full" ? "auto" : permission;
    expect(state.agents[child.agentId]!.permission).toBe(expected);
    expect(state.agents[lead.agentId]!.permission).toBe(permission);
    expect(runner.callsOf("start").at(-1)!.input.permission).toBe(expected);
  })));
}

it.live("s26: explicit full stays refused under an auto lead before allocation", () => run(runner => Effect.gen(function* () {
  const p = yield* workerProject(rig);
  const lead = yield* seedAgent({ id: "auto-lead", role: "lead:team", folder: p.path, parent: p.lead.agentId, permission: "auto" });
  const count = Object.keys(projectView(yield* snapshot).workspaces).length;
  expect(failureOf(yield* call(agentCredential(lead.token), "start", workerPayload(p.path, { permission: "full" })))).toBeDefined();
  expect(runner.callsOf("start")).toHaveLength(0);
  expect(Object.keys(projectView(yield* snapshot).workspaces)).toHaveLength(count);
})));

it.live("s26: team and allocated roots grant home admission and foreign checkout writes stay refused", () => run(runner => Effect.gen(function* () {
  const p = yield* workerProject(rig);
  const first = success(yield* call(p.teamCredential, "start", workerPayload(p.teamPath)));
  const firstAgent = (yield* snapshot).agents[first.agentId]!;
  const worker = yield* seedAgent({ id: "allocated-starter", role: "worker:builder", folder: firstAgent.folder, parent: p.teamLead.agentId });
  const tree = Object.values(projectView(yield* snapshot).workspaces).find(row => row.path === firstAgent.folder)!;
  yield* appendSetup([eventOf({ _tag: "project.WorktreeCreated", ...tree, owner: worker.agentId })]);
  success(yield* call(agentCredential(worker.token), "start", workerPayload(firstAgent.folder, { name: "Grandchild" })));
  expect(failureOf(yield* call(p.teamCredential, "start", workerPayload(p.teamPath, { permission: "edit", worktreeId: tree.worktreeId })))!.reason)
    .toBe("Starting in another agent's worktree requires read-only permission.");
  const reader = success(yield* call(p.teamCredential, "start", workerPayload(p.teamPath, { permission: "read-only", worktreeId: tree.worktreeId })));
  expect((yield* snapshot).agents[reader.agentId]!.folder).toBe(firstAgent.folder);
  expect(runner.callsOf("start")).toHaveLength(3);
})));

it.live("s26: manual descendants and unrelated repositories do not grant home admission", () => run(runner => Effect.gen(function* () {
  const p = yield* workerProject(rig);
  const other = join(rig.home, "unrelated"); mkdirSync(other);
  yield* appendSetup([eventOf({ _tag: "project.WorktreeCreated", worktreeId: "manual", repoId: p.repoId, machineId: "local", branch: "manual", path: other, checkout: "worktree", parentId: p.worktreeId })]);
  for (const folder of [other, rig.work, rig.home]) {
    expect(failureOf(yield* call(p.leadCredential, "start", workerPayload(folder, { role: "lead:team", permission: "read-only" })))!.reason).toMatch(/Start folder:/);
  }
  expect(runner.callsOf("start")).toHaveLength(0);
})));

it.live("s26: a hand-registered project root containing a home dotfolder refuses before allocation", () => run(runner => Effect.gen(function* () {
  const p = yield* workerProject(rig);
  const hidden = join(p.path, "private"); mkdirSync(hidden); symlinkSync(hidden, join(rig.home, ".ssh"));
  expect(failureOf(yield* call(p.leadCredential, "start", workerPayload(p.path)))!.reason).toBe("Start folder: folder contains a protected folder");
  expect(runner.callsOf("start")).toHaveLength(0);
})));

for (const fresh of [false, true]) it.live(`s26: ${fresh ? "fresh" : "session"} resume refuses after its worktree registration is replaced`, () => run(runner => Effect.gen(function* () {
  const p = yield* workerProject(rig);
  const reply = success(yield* call(p.leadCredential, "start", workerPayload(p.path)));
  const folder = (yield* snapshot).agents[reply.agentId]!.folder;
  const tree = Object.values(projectView(yield* snapshot).workspaces).find(row => row.path === folder)!;
  success(yield* call(ownerCredential, "stop", { agentId: reply.agentId }));
  // There is no unregister API. Replacing this row with another path removes
  // the old folder from the engine's records without inventing an event type.
  yield* appendSetup([eventOf({ _tag: "project.WorktreeCreated", ...tree, path: rig.outside })]);
  const calls = runner.callsOf("start").length + runner.callsOf("load").length;
  expect(failureOf(yield* call(ownerCredential, "resume", { agentId: reply.agentId, fresh }))!.reason)
    .toBe("Agent folder unavailable: This folder inside home is not a registered worktree of the starter's project. Ask the owner to restore its registration before retrying.");
  expect(runner.callsOf("start").length + runner.callsOf("load").length).toBe(calls);
  expect(existsSync(folder)).toBe(true);
})));

it.live("s26: recovery refuses a home worker whose registration was removed from that folder", () => Effect.gen(function* () {
  const runner = yield* makeFakeRunner();
  let workerId = "";
  yield* provideRig(rig, { runner, secrets: makeFakeSecrets() }, Effect.gen(function* () {
    yield* registerRebuildCheck();
    const p = yield* workerProject(rig);
    workerId = success(yield* call(p.leadCredential, "start", workerPayload(p.path))).agentId;
    const agent = (yield* snapshot).agents[workerId]!;
    const tree = Object.values(projectView(yield* snapshot).workspaces).find(row => row.path === agent.folder)!;
    yield* appendSetup([eventOf({ _tag: "project.WorktreeCreated", ...tree, path: rig.outside })]);
  }));
  const replacement = yield* makeFakeRunner();
  yield* provideRestart(rig, replacement.layer, Effect.gen(function* () {
    yield* registerRebuildCheck(); yield* replacement.subscribed; yield* recover;
    expect(yield* ready).toBe(true);
    expect(replacement.callsOf("load").some(call => call.input.agentId === workerId)).toBe(false);
    expect((yield* readLog).some(row => row.event._tag === "AgentReloadFailed" && row.event.agentId === workerId)).toBe(true);
  }));
}).pipe(Effect.scoped, Effect.orDie) as Effect.Effect<void>);

it.live("s26: a project whose repository is the synthetic home cannot grant a lead start", () => {
  rig = { ...rig, work: rig.home };
  return run(runner => Effect.gen(function* () {
    const p = yield* workerProject(rig);
    expect(failureOf(yield* call(p.leadCredential, "start", workerPayload(p.path)))!.reason).toMatch(/Start folder:/);
    expect(runner.callsOf("start")).toHaveLength(0);
  }));
});

it.live("s26: a configured harness login inside a registered root refuses", () => run(runner => Effect.gen(function* () {
  const p = yield* workerProject(rig);
  const login = join(p.path, "custom-login"); mkdirSync(login);
  yield* appendSetup([eventOf({ _tag: "AccountChanged", accountId: "login-account", account: { id: "login-account", harness: "claude", environment: { CLAUDE_CONFIG_DIR: login } } })]);
  expect(failureOf(yield* call(p.leadCredential, "start", workerPayload(p.path)))!.reason).toBe("Start folder: folder contains a protected folder");
  expect(runner.callsOf("start")).toHaveLength(0);
})));

it.live("s26: a secret-referenced harness login folder also stays protected", () => run(runner => Effect.gen(function* () {
  const p = yield* workerProject(rig);
  const login = join(p.path, "private-login"); mkdirSync(login);
  yield* (yield* SecretStore).put("secret:login-folder", Redacted.make(login));
  yield* appendSetup([eventOf({ _tag: "AccountChanged", accountId: "login-account", account: { id: "login-account", harness: "claude", environment: { CLAUDE_CONFIG_DIR: "secret:login-folder" } } })]);
  expect(failureOf(yield* call(p.leadCredential, "start", workerPayload(p.path)))!.reason).toBe("Start folder: folder contains a protected folder");
  expect(runner.callsOf("start")).toHaveLength(0);
})));

for (const variant of ["account-home", "cursor-config"] as const) it.live(`s26: login fallbacks protect ${variant} overrides`, () => run(runner => Effect.gen(function* () {
  const p = yield* workerProject(rig);
  const base = join(p.path, "login-base");
  mkdirSync(join(base, variant === "account-home" ? ".claude" : "cursor"), { recursive: true });
  yield* (yield* SecretStore).put("secret:login-base", Redacted.make(base));
  yield* appendSetup([eventOf({ _tag: "AccountChanged", accountId: "override-account", account: { id: "override-account",
    harness: variant === "account-home" ? "claude" : "cursor",
    environment: { [variant === "account-home" ? "HOME" : "XDG_CONFIG_HOME"]: "secret:login-base" } } })]);
  expect(failureOf(yield* call(p.leadCredential, "start", workerPayload(p.path)))!.reason).toBe("Start folder: folder contains a protected folder");
  expect(runner.callsOf("start")).toHaveLength(0);
})));
