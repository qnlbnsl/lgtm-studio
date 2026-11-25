import { mkdirSync, readFileSync, readdirSync, symlinkSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { it } from "@effect/vitest";
import { afterEach, beforeEach, expect, vi } from "vitest";
import * as Effect from "effect/Effect";
import { projectView } from "../../src/project/view.ts";
import { Views } from "../../src/views/Views.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";
import { decisionSetup } from "../support/projectDecisionFixtures.ts";
import { makeFakeRunner, type FakeRunner } from "../support/fakeRunner.ts";
import { agentCredential, appendSetup, call, eventOf, failureOf, makeFakeSecrets, makeRig, ownerCredential, provideRig, readLog, removeRig, seedAgent, success, type Rig } from "../support/handlerRig.ts";
import { workerCommit, workerGit as git, workerPayload, workerProject } from "../support/workerWorktrees.ts";

let rig: Rig;
beforeEach(() => { rig = makeRig(); vi.stubEnv("HOME", rig.home); vi.stubEnv("LGTM_HOME", rig.stateFolder); });
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllEnvs(); removeRig(rig); });
const snapshot = Effect.gen(function* () { return yield* (yield* Views).snapshot; });
const run = <A, E, R>(program: (runner: FakeRunner) => Effect.Effect<A, E, R>) => Effect.gen(function* () {
  const runner = yield* makeFakeRunner();
  return yield* provideRig(rig, { runner, secrets: makeFakeSecrets() }, Effect.gen(function* () {
    yield* registerRebuildCheck(); return yield* program(runner);
  }));
}).pipe(Effect.scoped, Effect.orDie);
const inventory = () => ({ folders: readdirSync(join(rig.work, ".worktrees")).sort(), branches: git(rig.work, "for-each-ref", "--format=%(refname)", "refs/heads") });

for (const role of ["lead:x", "orchestrator"] as const) for (const explicit of [false, true]) {
  it.live(`r3 m1: owner starts ${role} with explicit project ${explicit}`, () => run(() => Effect.gen(function* () {
    const p = yield* workerProject(rig), before = inventory();
    const reply = success(yield* call(ownerCredential, "start", workerPayload(p.path, { role, ...(explicit ? { projectId: p.projectId } : {}) })));
    const state = yield* snapshot;
    expect(projectView(state).agentProjects[reply.agentId]).toBe(explicit ? p.projectId : undefined);
    expect(state.agents[reply.agentId]!.folder).toBe(p.path);
    expect(inventory()).toEqual(before);
  })));
}

for (const target of ["sibling", "subfolder", "alias", "own"] as const) for (const permission of ["edit", "read-only"] as const) {
  it.live(`r3 m3: builder starts lead:x in ${target} folder with ${permission}`, () => run((runner) => Effect.gen(function* () {
    const p = yield* workerProject(rig);
    const first = success(yield* call(ownerCredential, "start", workerPayload(p.path, { projectId: p.projectId, permission: "full" })));
    const firstFolder = (yield* snapshot).agents[first.agentId]!.folder;
    const builder = yield* seedAgent({ id: "builder-caller", role: "worker:builder", folder: firstFolder, parent: p.lead.agentId, permission: "full" });
    const sibling = success(yield* call(p.leadCredential, "start", workerPayload(p.path, { name: "Sibling" })));
    const siblingFolder = (yield* snapshot).agents[sibling.agentId]!.folder;
    let folder = target === "own" ? firstFolder : siblingFolder;
    if (target === "subfolder") { folder = join(siblingFolder, "source"); mkdirSync(folder); }
    if (target === "alias") { folder = join(firstFolder, "sibling-link"); symlinkSync(siblingFolder, folder); }
    const before = inventory(), count = runner.callsOf("start").length, events = (yield* readLog).length;
    const reply = yield* call(agentCredential(builder.token), "start", workerPayload(folder, { name: "Child", role: "lead:x", permission }));
    if (target !== "own" && permission !== "read-only" || target === "subfolder") {
      expect(failureOf(reply)).toEqual({ _tag: "CommandRefused", reason: permission === "read-only"
        ? "Start folder: folder is outside the parent's folder and its repository"
        : "Starting in another agent's worktree requires read-only permission." });
      expect(runner.callsOf("start")).toHaveLength(count);
      expect((yield* readLog).slice(events).map((row) => row.event._tag)).toEqual(["CommandRefused"]);
    } else {
      const id = success(reply).agentId;
      expect((yield* snapshot).agents[id]!.folder).toBe(target === "alias" ? siblingFolder : folder);
      expect(runner.callsOf("start").at(-1)!.input.permission).toBe(permission);
    }
    expect(inventory()).toEqual(before);
  })));
}

for (const tag of ["project.ArchiveStarted", "project.Archived"] as const) {
  it.live(`r3 m4: a non-allocating start keeps the base refusal with ${tag}`, () => run((runner) => Effect.gen(function* () {
    const p = yield* workerProject(rig), before = inventory();
    yield* appendSetup([eventOf({ _tag: tag, projectId: p.projectId })]);
    expect(failureOf(yield* call(p.leadCredential, "start", workerPayload(p.path, { role: "lead:x" })))).toEqual({ _tag: "CommandRefused", reason: "project is archived or archiving" });
    expect(runner.callsOf("start")).toHaveLength(0); expect(inventory()).toEqual(before);
  })));
}

it.live("r3 m5: a failed agent cannot fresh-resume after its checkout is reassigned", () => run((runner) => Effect.gen(function* () {
  const p = yield* workerProject(rig); runner.rejectNext("start", "synthetic failure");
  const first = success(yield* call(p.leadCredential, "start", workerPayload(p.path)));
  const second = success(yield* call(p.leadCredential, "start", workerPayload(p.path)));
  const state = yield* snapshot, folder = state.agents[first.agentId]!.folder;
  expect(state.agents[second.agentId]!.folder).toBe(folder);
  expect(Object.values(projectView(state).workspaces).find((row) => row.path === folder)!.owner).toBe(second.agentId);
  const count = runner.callsOf("start").length, events = (yield* readLog).length;
  expect(failureOf(yield* call(ownerCredential, "resume", { agentId: first.agentId, fresh: true }))).toEqual({ _tag: "CommandRefused", reason: "This worktree now belongs to another agent. Start a new agent in its own worktree." });
  expect(runner.callsOf("start")).toHaveLength(count);
  expect((yield* readLog).slice(events).map((row) => row.event._tag)).toEqual(["CommandRefused"]);
  expect((yield* snapshot).agents[first.agentId]!.state._tag).toBe("stopped");
})));

it.live("r3 m6: a read-only visitor cannot allocate from a builder's checkout", () => run((runner) => Effect.gen(function* () {
  const p = yield* workerProject(rig);
  const builder = success(yield* call(p.leadCredential, "start", workerPayload(p.path)));
  const state = yield* snapshot, folder = state.agents[builder.agentId]!.folder;
  const worktreeId = Object.values(projectView(state).workspaces).find((row) => row.path === folder)!.worktreeId;
  const visitor = yield* seedAgent({ id: "reviewer", role: "worker:reviewer", parent: p.lead.agentId, folder, permission: "read-only" });
  // Selection remains available; this request instead asks for a new allocation.
  expect(worktreeId).toBeDefined();
  const before = inventory(), count = runner.callsOf("start").length, events = (yield* readLog).length;
  expect(failureOf(yield* call(agentCredential(visitor.token), "start", workerPayload(folder, { permission: "read-only" })))).toEqual({ _tag: "CommandRefused", reason: "Only a lead or the agent that owns this checkout can allocate a worker from it. Ask your lead to start the worker." });
  expect(runner.callsOf("start")).toHaveLength(count); expect(inventory()).toEqual(before);
  expect((yield* readLog).slice(events).map((row) => row.event._tag)).toEqual(["CommandRefused"]);
})));

it.live("r3 m6: an owning worker can allocate from its own checkout", () => run((runner) => Effect.gen(function* () {
  const p = yield* workerProject(rig);
  const worker = yield* seedAgent({ id: "owning-builder", role: "worker:builder", parent: p.lead.agentId, folder: p.teamPath });
  const view = projectView(yield* snapshot), tree = view.workspaces[p.teamWorktreeId]!;
  yield* appendSetup([eventOf({ _tag: "project.WorktreeCreated", ...tree, owner: worker.agentId })]);
  const reply = success(yield* call(agentCredential(worker.token), "start", workerPayload(p.teamPath)));
  const state = yield* snapshot, folder = state.agents[reply.agentId]!.folder;
  expect(folder).not.toBe(p.teamPath);
  expect(Object.values(projectView(state).workspaces).find((row) => row.path === folder)!.parentId).toBe(p.teamWorktreeId);
  expect(runner.callsOf("start").at(-1)!.input.folder).toBe(folder);
})));

it.live("r3 m6: the recorded project lead can allocate with a legacy worker role", () => run((runner) => Effect.gen(function* () {
  git(rig.work, "init", "-b", "topic");
  git(rig.work, "config", "user.name", "Test owner"); git(rig.work, "config", "user.email", "test@example.invalid");
  git(rig.work, "config", "commit.gpgsign", "false"); workerCommit(rig.work, "base.txt");
  const p = yield* decisionSetup(rig.work);
  success(yield* call(ownerCredential, "project.settings.set", { settings: { workspaceFolder: rig.root, shell: "sh" } }));
  expect((yield* snapshot).agents[p.leadId]!.role).toBe("worker");
  const reply = success(yield* call(p.leadCredential, "start", workerPayload(rig.work)));
  const folder = (yield* snapshot).agents[reply.agentId]!.folder;
  expect(folder).not.toBe(rig.work);
  expect(Object.values(projectView(yield* snapshot).workspaces).find((row) => row.path === folder)!.parentId).toBe(p.worktreeId);
  expect(runner.callsOf("start").at(-1)!.input.folder).toBe(folder);
})));

for (const kind of ["unregistered", "branch-changed"] as const) {
  it.live(`r3 m7: ${kind} gives a sentence with a recovery action`, () => run((runner) => Effect.gen(function* () {
    const p = yield* workerProject(rig);
    if (kind === "branch-changed") git(p.path, "switch", "-c", "changed");
    const lead = kind === "unregistered" ? yield* seedAgent({ id: "unregistered-lead", role: "lead:x", parent: p.lead.agentId, folder: rig.work }) : undefined;
    const before = inventory();
    expect(failureOf(yield* call(lead ? agentCredential(lead.token) : p.leadCredential, "start", workerPayload(lead ? rig.work : p.path)))).toEqual({ _tag: "CommandRefused", reason: kind === "unregistered"
      ? "The starter's checkout is not registered for this project. Ask your parent or the owner to register it before starting a worker."
      : "The registered workspace branch has changed. Restore the registered branch before retrying." });
    expect(runner.callsOf("start")).toHaveLength(0); expect(inventory()).toEqual(before);
  })));
}

it.live("r3 m2: a test runner gets the builder's merged commit and writable logs in its own checkout", () => run((runner) => Effect.gen(function* () {
  const p = yield* workerProject(rig);
  const builder = success(yield* call(p.leadCredential, "start", workerPayload(p.path)));
  const builderFolder = (yield* snapshot).agents[builder.agentId]!.folder;
  const head = workerCommit(builderFolder, "built.txt", "built\n");
  git(p.path, "merge", "--ff-only", head);
  const lead = yield* seedAgent({ id: "writable-lead", role: "lead:x", folder: p.path, parent: p.lead.agentId, permission: "full" });
  const reply = success(yield* call(agentCredential(lead.token), "start", workerPayload(p.path, { role: "worker:test-runner", name: "Tests", permission: "edit" })));
  const folder = (yield* snapshot).agents[reply.agentId]!.folder;
  expect(folder).not.toBe(builderFolder); expect(folder).not.toBe(p.path);
  expect(git(folder, "rev-parse", "HEAD")).toBe(head);
  expect(readFileSync(join(folder, "built.txt"), "utf8")).toBe("built\n");
  mkdirSync(join(folder, "test-logs")); writeFileSync(join(folder, "test-logs", "run.log"), "passed\n");
  expect(readFileSync(join(folder, "test-logs", "run.log"), "utf8")).toBe("passed\n");
  expect(git(builderFolder, "status", "--porcelain")).toBe("");
  expect(runner.callsOf("start").at(-1)!.input).toMatchObject({ folder, permission: "edit" });
})));
