import { existsSync, mkdirSync, readdirSync, readFileSync, renameSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { it } from "@effect/vitest";
import { afterEach, beforeEach, expect, vi } from "vitest";
import * as Effect from "effect/Effect";
import * as Redacted from "effect/Redacted";
import { makeRedactor } from "../../src/identity/redactor.ts";
import * as GitProcess from "../../src/project/git/process.ts";
import { GitFailure } from "../../src/project/git/types.ts";
import { Views } from "../../src/views/Views.ts";
import { projectView } from "../../src/project/view.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";
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

it.live("r2 M1: a repository inside home refuses the lead before any Git work", () => {
  const work = join(rig.home, "repository"); renameSync(rig.work, work); rig = { ...rig, work };
  return run((runner) => Effect.gen(function* () {
    const p = yield* workerProject(rig), before = inventory(), events = (yield* readLog).length;
    const process = vi.spyOn(GitProcess, "gitProcess");
    expect(failureOf(yield* call(p.leadCredential, "start", workerPayload(p.path)))).toEqual({ _tag: "CommandRefused", reason: "Start folder: folder is, or is inside, a protected folder" });
    expect(process).not.toHaveBeenCalled(); expect(runner.callsOf("start")).toHaveLength(0);
    expect(inventory()).toEqual(before);
    expect((yield* readLog).slice(events).map((row) => row.event._tag)).toEqual(["CommandRefused"]);
  }));
});

for (const target of ["lead", "sibling", "other-team"] as const) for (const permission of ["ask", "edit", "full", "read-only"] as const) {
  it.live(`r2 M3: a builder names its ${target} checkout with ${permission}`, () => run((runner) => Effect.gen(function* () {
    const p = yield* workerProject(rig);
    const first = success(yield* call(ownerCredential, "start", workerPayload(p.path, { projectId: p.projectId, permission: "full" })));
    const firstFolder = (yield* snapshot).agents[first.agentId]!.folder;
    const builder = yield* seedAgent({ id: "builder-caller", role: "worker:builder", folder: firstFolder, parent: p.lead.agentId, permission: "full" });
    const sibling = success(yield* call(p.leadCredential, "start", workerPayload(p.path, { name: "Sibling" })));
    const siblingFolder = (yield* snapshot).agents[sibling.agentId]!.folder;
    const selected = target === "lead" ? p.worktreeId : target === "other-team" ? p.teamWorktreeId
      : Object.values(projectView(yield* snapshot).workspaces).find((row) => row.path === siblingFolder)!.worktreeId;
    const before = inventory(), calls = runner.callsOf("start").length, events = (yield* readLog).length;
    const result = yield* call(agentCredential(builder.token), "start", workerPayload(firstFolder, { name: "Child", permission, worktreeId: selected }));
    if (permission !== "read-only") {
      expect(failureOf(result)).toEqual({ _tag: "CommandRefused", reason: "Starting in another agent's worktree requires read-only permission." });
      expect(runner.callsOf("start")).toHaveLength(calls);
      expect((yield* readLog).slice(events).map((row) => row.event._tag)).toEqual(["CommandRefused"]);
    } else {
      const reply = success(result), selectedPath = projectView(yield* snapshot).workspaces[selected]!.path;
      expect((yield* snapshot).agents[reply.agentId]!.folder).toBe(selectedPath);
      expect(runner.callsOf("start").at(-1)!.input).toMatchObject({ folder: selectedPath, permission });
    }
    expect(inventory()).toEqual(before);
  })));
}

for (const kind of ["subfolder", "changed-branch", "unset-settings", "non-local"] as const) {
  it.live(`r2 M4: a lead keeping its folder bypasses Git with ${kind}`, () => run((runner) => Effect.gen(function* () {
    const p = yield* workerProject(rig, { settings: kind !== "unset-settings" });
    let folder = p.path, actor = p.leadCredential;
    if (kind === "subfolder") {
      folder = join(p.path, "source"); mkdirSync(folder);
      const lead = yield* seedAgent({ id: "subfolder-lead", role: "lead:team", folder, parent: p.lead.agentId }); actor = agentCredential(lead.token);
    }
    if (kind === "changed-branch") git(p.path, "switch", "-c", "other-branch");
    if (kind === "non-local") yield* appendSetup([eventOf({ _tag: "project.WorktreeCreated", ...projectView(yield* snapshot).workspaces[p.worktreeId]!, machineId: "buildbox" }), eventOf({ _tag: "project.Placed", projectId: p.projectId, worktreeId: p.worktreeId, machineId: "buildbox" })]);
    const before = inventory(), process = vi.spyOn(GitProcess, "gitProcess");
    const reply = success(yield* call(actor, "start", workerPayload(folder, { role: "lead:builder" })));
    expect((yield* snapshot).agents[reply.agentId]!.folder).toBe(folder);
    expect(process).not.toHaveBeenCalled(); expect(runner.callsOf("start").at(-1)!.input.folder).toBe(folder);
    expect(inventory()).toEqual(before);
  })));
}

it.live("r2 M4: a subfolder starter allocates from its registered checkout", () => run((runner) => Effect.gen(function* () {
  const p = yield* workerProject(rig), folder = join(p.teamPath, "source"); mkdirSync(folder);
  const head = workerCommit(p.teamPath, "committed.txt");
  const lead = yield* seedAgent({ id: "subfolder-lead", role: "lead:team", folder, parent: p.lead.agentId });
  const reply = success(yield* call(agentCredential(lead.token), "start", workerPayload(folder)));
  const agent = (yield* snapshot).agents[reply.agentId]!;
  const tree = Object.values(projectView(yield* snapshot).workspaces).find((row) => row.path === agent.folder)!;
  expect(tree.parentId).toBe(p.teamWorktreeId); expect(git(agent.folder, "rev-parse", "HEAD")).toBe(head);
  expect(runner.callsOf("start").at(-1)!.input.grant.realPath).toBe(tree.path);
})));

it.live("r2 M4: remote worker allocation refuses clearly before Git", () => run((runner) => Effect.gen(function* () {
  const p = yield* workerProject(rig);
  yield* appendSetup([eventOf({ _tag: "project.WorktreeCreated", ...projectView(yield* snapshot).workspaces[p.worktreeId]!, machineId: "buildbox" }), eventOf({ _tag: "project.Placed", projectId: p.projectId, worktreeId: p.worktreeId, machineId: "buildbox" })]);
  const before = inventory(), process = vi.spyOn(GitProcess, "gitProcess");
  expect(failureOf(yield* call(p.leadCredential, "start", workerPayload(p.path)))).toEqual({ _tag: "CommandRefused", reason: "Worker worktrees can only be allocated on the local machine before machine pairing." });
  expect(process).not.toHaveBeenCalled(); expect(runner.callsOf("start")).toHaveLength(0); expect(inventory()).toEqual(before);
})));

for (const [role, allocate] of [["leadership-builder", true], ["orchestratorx", true], ["project-lead", false], ["lead:builder", false]] as const) {
  it.live(`r2 m1: role ${role} follows parseRole`, () => run(() => Effect.gen(function* () {
    const p = yield* workerProject(rig);
    const reply = success(yield* call(p.leadCredential, "start", workerPayload(p.path, { role })));
    expect((yield* snapshot).agents[reply.agentId]!.folder === p.path).toBe(!allocate);
  })));
}

it.live("r2 m2: a dirty starter allocates from HEAD without copying uncommitted files", () => run(() => Effect.gen(function* () {
  const p = yield* workerProject(rig); writeFileSync(join(p.path, "dirty.txt"), "pending\n");
  writeFileSync(join(p.path, "base.txt"), "uncommitted edit\n");
  const reply = success(yield* call(p.leadCredential, "start", workerPayload(p.path)));
  const folder = (yield* snapshot).agents[reply.agentId]!.folder;
  expect(git(folder, "rev-parse", "HEAD")).toBe(git(p.path, "rev-parse", "HEAD"));
  expect(existsSync(join(folder, "dirty.txt"))).toBe(false); expect(readFileSync(join(folder, "base.txt"), "utf8")).toBe("base\n");
  expect(readFileSync(join(p.path, "dirty.txt"), "utf8")).toBe("pending\n");
})));

for (const changed of ["none", "dirty", "committed", "session", "different-starter", "different-name", "source-moved", "legacy-allocation"] as const) {
  it.live(`r2 m4: failed harness allocation retry with ${changed}`, () => run((runner) => Effect.gen(function* () {
    const p = yield* workerProject(rig); runner.rejectNext("start", "synthetic harness failure");
    const first = success(yield* call(p.leadCredential, "start", workerPayload(p.path)));
    const agent = (yield* snapshot).agents[first.agentId]!;
    expect(agent.startFailure).toBeDefined(); expect(agent.session).toBeUndefined();
    const tree = Object.values(projectView(yield* snapshot).workspaces).find((row) => row.path === agent.folder)!;
    if (changed === "legacy-allocation") {
      const { workerSourceHead: _source, ...legacy } = tree;
      yield* appendSetup([eventOf({ _tag: "project.WorktreeCreated", ...legacy })]);
    }
    if (changed === "dirty") writeFileSync(join(agent.folder, "dirty.txt"), "keep\n");
    if (changed === "committed") workerCommit(agent.folder, "unmerged.txt");
    if (changed === "source-moved") workerCommit(p.path, "new-source.txt");
    if (changed === "session") { success(yield* call(ownerCredential, "resume", { agentId: first.agentId, fresh: true })); success(yield* call(ownerCredential, "stop", { agentId: first.agentId })); }
    let actor = p.leadCredential;
    if (changed === "different-starter") { const other = yield* seedAgent({ id: "another-lead", folder: p.path, role: "lead", parent: p.lead.agentId }); actor = agentCredential(other.token); }
    const before = (yield* readLog).length;
    const reply = success(yield* call(actor, "start", workerPayload(p.path, changed === "different-name" ? { name: "Other" } : {})));
    const folder = (yield* snapshot).agents[reply.agentId]!.folder;
    const reuse = changed === "none";
    expect(folder === agent.folder).toBe(reuse);
    const registrations = (yield* readLog).slice(before).filter((row) => row.event._tag === "project.WorktreeCreated").map((row) => row.event);
    expect(registrations).toHaveLength(1);
    if (reuse) {
      expect(registrations[0]).toMatchObject({ worktreeId: tree.worktreeId, owner: reply.agentId });
      const visible = success(yield* call(p.leadCredential, "client.snapshot", {}));
      expect(visible.project.workspaces.find((row: { owner?: string }) => row.owner === reply.agentId).worktreeId).toBe(tree.worktreeId);
    }
    expect(existsSync(tree.path)).toBe(true);
    if (changed === "dirty") expect(readFileSync(join(tree.path, "dirty.txt"), "utf8")).toBe("keep\n");
  })));
}

for (const kind of ["project", "agent", "both", "punctuation-agent"] as const) {
  it.live(`r2 m5: non-ASCII ${kind} names fall back to identifiers`, () => run(() => Effect.gen(function* () {
    const p = yield* workerProject(rig);
    if (kind === "project" || kind === "both") success(yield* call(ownerCredential, "project.rename", { projectId: p.projectId, name: "项目" }));
    const reply = success(yield* call(p.leadCredential, "start", workerPayload(p.path, kind !== "project" ? { name: kind === "punctuation-agent" ? "!!!" : "建设者" } : {})));
    const folder = (yield* snapshot).agents[reply.agentId]!.folder, branch = git(folder, "branch", "--show-current");
    expect(branch).toMatch(/^worker\/[a-z0-9-]+\/[a-z0-9-]+$/);
    if (kind === "project" || kind === "both") expect(branch).toContain("worker-project");
    expect(git(folder, "rev-parse", "HEAD")).toBe(git(p.path, "rev-parse", "HEAD"));
  })));
}

for (const kind of ["reply", "throw"] as const) {
  it.live(`r2 m6: dirty partial checkout preserves the original ${kind} failure`, () => run((runner) => Effect.gen(function* () {
    const p = yield* workerProject(rig), real = GitProcess.gitProcess;
    vi.spyOn(GitProcess, "gitProcess").mockImplementation(async (folder, args, limits, signal, truncate) => {
      if (args[0] === "worktree" && args[1] === "add") {
        await real(folder, args, limits, signal, truncate);
        writeFileSync(join(args[args.indexOf("--") + 1]!, "keep.txt"), "external work\n");
        if (kind === "throw") throw new GitFailure({ reason: "timeout" });
        return { code: 128, stdout: "", stderr: "synthetic disk error", truncated: false };
      }
      return real(folder, args, limits, signal, truncate);
    });
    expect(failureOf(yield* call(p.leadCredential, "start", workerPayload(p.path)))).toEqual({ _tag: "CommandRefused", reason: `worker-worktree:${kind === "reply" ? "git_failed" : "timeout"}` });
    expect(readFileSync(join(rig.work, ".worktrees", "worker-work-trees-builder", "keep.txt"), "utf8")).toBe("external work\n");
    expect(runner.callsOf("start")).toHaveLength(0);
  })));
}

it.live("r2 m7: a manual child of the project stays private to the owner", () => run(() => Effect.gen(function* () {
  const p = yield* workerProject(rig), path = join(rig.work, ".worktrees", "private");
  git(p.path, "worktree", "add", "-b", "private", path);
  const id = success(yield* call(ownerCredential, "project.workspace.register", { repoId: p.repoId, machineId: "local", branch: "private", path, checkout: "worktree", parentId: p.worktreeId })).worktreeId;
  const visible = success(yield* call(p.leadCredential, "client.snapshot", {}));
  expect(visible.project.workspaces.map((row: { worktreeId: string }) => row.worktreeId)).not.toContain(id);
  expect(failureOf(yield* call(p.leadCredential, "project.changes", { projectId: p.projectId, worktreeId: id }))!.reason).toBe("project is unavailable to this caller");
  expect(failureOf(yield* call(p.leadCredential, "start", workerPayload(p.path, { worktreeId: id, permission: "read-only" })))!.reason).toBe("worker-worktree:workspace_unavailable");
  const owner = success(yield* call(ownerCredential, "start", workerPayload(p.path, { projectId: p.projectId, worktreeId: id, permission: "full" })));
  expect((yield* snapshot).agents[owner.agentId]!.folder).toBe(path);
})));

it.live("r2 m4: repeated sessionless failures keep reusing one allocation", () => run((runner) => Effect.gen(function* () {
  const p = yield* workerProject(rig);
  runner.rejectNext("start", "synthetic harness failure", 3);
  const folders: string[] = [];
  for (let n = 0; n < 4; n++) {
    const reply = success(yield* call(p.leadCredential, "start", workerPayload(p.path)));
    folders.push((yield* snapshot).agents[reply.agentId]!.folder);
  }
  expect(new Set(folders).size).toBe(1);
  const registrations = (yield* readLog).filter((row) => row.event._tag === "project.WorktreeCreated" && row.event.owner !== undefined).map((row) => row.event);
  expect(registrations).toHaveLength(4);
  expect(new Set(registrations.map((event) => event._tag === "project.WorktreeCreated" ? event.worktreeId : "unexpected" )).size).toBe(1);
})));


it.live("r2 M3: an agent may name its own checkout with edit permission", () => run((runner) => Effect.gen(function* () {
  const p = yield* workerProject(rig);
  const parent = yield* seedAgent({ id: "edit-lead", folder: p.path, role: "lead", parent: p.lead.agentId, permission: "full" });
  const reply = success(yield* call(agentCredential(parent.token), "start", workerPayload(p.path, { worktreeId: p.worktreeId, permission: "edit" })));
  expect((yield* snapshot).agents[reply.agentId]!.folder).toBe(p.path);
  expect(runner.callsOf("start").at(-1)!.input.permission).toBe("edit");
})));

it.live("r2 M3: the owner can name a project checkout with every permission", () => run((runner) => Effect.gen(function* () {
  const p = yield* workerProject(rig), before = inventory();
  for (const permission of ["read-only", "ask", "edit", "full"]) {
    const reply = success(yield* call(ownerCredential, "start", workerPayload(p.path, { projectId: p.projectId, worktreeId: p.teamWorktreeId, permission })));
    expect((yield* snapshot).agents[reply.agentId]!.folder).toBe(p.teamPath);
    expect(runner.callsOf("start").at(-1)!.input.permission).toBe(permission);
  }
  expect(inventory()).toEqual(before);
})));

it.live("r2 m7: original direct team-child Git access remains narrow", () => run(() => Effect.gen(function* () {
  const p = yield* workerProject(rig), path = join(rig.work, ".worktrees", "manual-team-child");
  git(p.teamPath, "worktree", "add", "-b", "manual-team-child", path);
  const id = success(yield* call(ownerCredential, "project.workspace.register", { repoId: p.repoId, machineId: "local", branch: "manual-team-child", path, checkout: "worktree", parentId: p.teamWorktreeId })).worktreeId;
  success(yield* call(p.leadCredential, "project.changes", { projectId: p.projectId, worktreeId: id }));
  const visible = success(yield* call(p.leadCredential, "client.snapshot", {}));
  expect(visible.project.workspaces.map((row: { worktreeId: string }) => row.worktreeId)).not.toContain(id);
  const lead = yield* seedAgent({ id: "legacy-checkout-lead", folder: path, role: "lead", parent: p.lead.agentId });
  const reply = success(yield* call(agentCredential(lead.token), "start", workerPayload(path)));
  const state = yield* snapshot, folder = state.agents[reply.agentId]!.folder;
  const allocated = Object.values(projectView(state).workspaces).find((row) => row.path === folder)!;
  const after = success(yield* call(p.leadCredential, "client.snapshot", {}));
  expect(after.project.workspaces.map((row: { worktreeId: string }) => row.worktreeId)).toContain(allocated.worktreeId);
  success(yield* call(p.leadCredential, "project.changes", { projectId: p.projectId, worktreeId: allocated.worktreeId }));
})));

it.live("r2 M4: owner project inference stays limited to the exact registered folder", () => run((runner) => Effect.gen(function* () {
  const p = yield* workerProject(rig, { settings: false });
  // Historical setup can register a broad folder. An unrelated owner start
  // below it must retain the original non-project start behaviour.
  yield* appendSetup([eventOf({ _tag: "project.WorktreeCreated", ...projectView(yield* snapshot).workspaces[p.worktreeId]!, path: rig.root })]);
  const before = inventory(), process = vi.spyOn(GitProcess, "gitProcess");
  const reply = success(yield* call(ownerCredential, "start", workerPayload(rig.outside)));
  const state = yield* snapshot;
  expect(state.agents[reply.agentId]!.folder).toBe(rig.outside);
  expect(projectView(state).agentProjects[reply.agentId]).toBeUndefined();
  expect(runner.callsOf("start").at(-1)!.input.folder).toBe(rig.outside);
  expect(process).not.toHaveBeenCalled(); expect(inventory()).toEqual(before);
})));


it.live("r2: secret-bearing agent names are refused before slug conversion can expose them", () => Effect.gen(function* () {
  const runner = yield* makeFakeRunner();
  const secret = "MixedCasePrivateValue";
  const redactor = makeRedactor({ secrets: () => [Redacted.make(secret)], tokenHashes: () => new Set() });
  yield* provideRig(rig, { runner, secrets: makeFakeSecrets(), redactor: redactor.layer }, Effect.gen(function* () {
    yield* registerRebuildCheck();
    const p = yield* workerProject(rig), before = inventory();
    const process = vi.spyOn(GitProcess, "gitProcess");
    expect(failureOf(yield* call(p.leadCredential, "start", workerPayload(p.path, { name: secret })))).toEqual({ _tag: "CommandRefused", reason: "a path cannot contain a registered secret" });
    expect(process.mock.calls.some(([, args]) => args[0] === "worktree" && args[1] === "add")).toBe(false);
    expect(runner.callsOf("start")).toHaveLength(0); expect(inventory()).toEqual(before);
  }));
}).pipe(Effect.scoped, Effect.orDie));
