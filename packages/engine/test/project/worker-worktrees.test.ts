import { existsSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { it } from "@effect/vitest";
import { afterEach, beforeEach, expect, vi } from "vitest";
import * as Effect from "effect/Effect";
import * as Schema from "effect/Schema";
import { Event } from "@lgtm/contracts";
import * as GitProcess from "../../src/project/git/process.ts";
import { provideRestart, recover, ready } from "../support/restartRig.ts";
import { provideBoard } from "../board/wakeFixtures.ts";
import { eventOf } from "../support/handlerRig.ts";
import { EventLog } from "../../src/log/EventLog.ts";
import { Views } from "../../src/views/Views.ts";
import { projectView } from "../../src/project/view.ts";
import { snapshotFor } from "../../src/project/snapshot.ts";
import { makeFakeRunner, type FakeRunner } from "../support/fakeRunner.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";
import { agentCredential, call, failureOf, freshRequest, makeFakeSecrets, makeRig, ownerCredential, provideRig, readLog, removeRig, success, type Rig } from "../support/handlerRig.ts";
import { workerCommit, workerGit as git, workerPayload, workerProject } from "../support/workerWorktrees.ts";

let rig: Rig;
beforeEach(() => { rig = makeRig(); vi.stubEnv("HOME", rig.home); vi.stubEnv("LGTM_HOME", rig.stateFolder); });
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllEnvs(); removeRig(rig); });
const snapshot = Effect.gen(function* () { return yield* (yield* Views).snapshot; });
const run = <A, E, R>(program: Effect.Effect<A, E, R> | ((runner: FakeRunner) => Effect.Effect<A, E, R>)) => Effect.gen(function* () {
  const runner = yield* makeFakeRunner();
  return yield* provideRig(rig, { runner, secrets: makeFakeSecrets() }, Effect.gen(function* () {
    yield* registerRebuildCheck();
    return yield* (typeof program === "function" ? program(runner) : program);
  }));
}).pipe(Effect.scoped);

// The table represents the Cartesian start dimensions. Every role with write
// access shares the builder policy; the reviewer rows prove read-only reuse.
export const startCases = ["owner", "project-lead", "team-lead"].flatMap((starter) =>
  ["builder:typescript", "reviewer", "test-runner", "evaluator", "lead:team"].flatMap((role) =>
    [false, true].flatMap((project) => [false, true].map((named) => ({ starter, role, project, named }))))).filter((row) => row.project || row.starter !== "team-lead");
for (const row of startCases) {
  it.live(`start table: ${row.starter} / ${row.role} / project=${row.project} / named=${row.named}`, () => run((runner) => Effect.gen(function* () {
    const p = yield* workerProject(rig);
    const actor = row.starter === "owner" ? ownerCredential : !row.project ? p.outsideCredential : row.starter === "team-lead" ? p.teamCredential : p.leadCredential;
    const basePath = !row.project ? rig.outside : row.starter === "team-lead" ? p.teamPath : p.path;
    const parentId = row.starter === "team-lead" ? p.teamWorktreeId : p.worktreeId;
    const namedId = row.starter === "team-lead" ? p.worktreeId : p.teamWorktreeId;
    const namedPath = row.starter === "team-lead" ? p.path : p.teamPath;
    const permission = row.role === "reviewer" || row.named && row.starter !== "owner" ? "read-only" : "ask";
    const before = yield* readLog;
    const result = yield* call(actor, "start", workerPayload(basePath, {
      role: row.role, permission,
      ...(row.project && row.starter === "owner" ? { projectId: p.projectId } : {}),
      ...(row.named ? { worktreeId: namedId } : {}),
    }));
    if (!row.project && row.named) {
      expect(failureOf(result)!.reason).toBe("worker-worktree:project_required");
      expect(runner.callsOf("start")).toHaveLength(0);
      expect((yield* readLog).slice(before.length).map((row) => row.event._tag)).toEqual(["CommandRefused"]);
      return;
    }
    const reply = success(result), state = yield* snapshot, agent = state.agents[reply.agentId as string]!;
    const allocated = row.project && !row.named && !row.role.startsWith("lead");
    const folder = allocated ? join(rig.work, ".worktrees", "worker-work-trees-builder") : row.named ? namedPath : basePath;
    expect(agent.folder).toBe(folder);
    expect(runner.callsOf("start")[0]!.input).toMatchObject({ folder, permission });
    expect(runner.callsOf("start")[0]!.input.grant.realPath).toBe(folder);
    const events = (yield* readLog).slice(before.length).map((row) => row.event);
    expect(events.filter((event) => event._tag === "project.WorktreeCreated")).toHaveLength(allocated ? 1 : 0);
    expect(events.filter((event) => event._tag === "AgentStarted")).toHaveLength(1);
    if (allocated) {
      const workspace = Object.values(projectView(state).workspaces).find((row) => row.path === folder)!;
      expect(workspace).toMatchObject({ branch: "worker/work-trees/builder", parentId, projectId: p.projectId, owner: agent.agentId });
      expect(snapshotFor({ _tag: "agent", agentId: p.lead.agentId }, state)!.workspaces.map((row) => row.worktreeId)).toContain(workspace.worktreeId);
      expect(git(folder, "rev-parse", "HEAD")).toBe(git(basePath, "rev-parse", "HEAD"));
    }
    expect(projectView(state).agentProjects[agent.agentId]).toBe(row.project ? p.projectId : undefined);
  })).pipe(Effect.orDie));
}

it.live("isolates sibling commits, integrates into the recorded project parent, reuses receipts and preserves removed workers", () => run(Effect.gen(function* () {
  const p = yield* workerProject(rig), requestId = freshRequest();
  const payload = workerPayload(p.path, { requestId });
  const first = success(yield* call(p.leadCredential, "start", payload));
  expect(success(yield* call(p.leadCredential, "start", payload))).toEqual(first);
  const second = success(yield* call(p.leadCredential, "start", workerPayload(p.path)));
  const state = yield* snapshot, one = state.agents[first.agentId as string]!, two = state.agents[second.agentId as string]!;
  expect(one.folder).not.toBe(two.folder);
  expect(git(two.folder, "branch", "--show-current")).toBe("worker/work-trees/builder-2");
  const head = workerCommit(one.folder, "isolated.txt");
  expect(existsSync(join(two.folder, "isolated.txt"))).toBe(false);
  expect(existsSync(join(p.path, "isolated.txt"))).toBe(false);
  const workspace = Object.values(projectView(state).workspaces).find((row) => row.path === one.folder)!;
  success(yield* call(ownerCredential, "project.integrate", { worktreeId: workspace.worktreeId, destinationId: p.worktreeId, expectedHead: head }));
  expect(readFileSync(join(p.path, "isolated.txt"), "utf8")).toBe("worker change\n");
  workerCommit(two.folder, "unmerged.txt"); writeFileSync(join(two.folder, "dirty.txt"), "keep\n");
  success(yield* call(ownerCredential, "stop", { agentId: second.agentId }));
  expect(readFileSync(join(two.folder, "dirty.txt"), "utf8")).toBe("keep\n");
  expect(git(two.folder, "log", "-1", "--format=%s")).toBe("unmerged.txt");
  const read = snapshotFor({ _tag: "agent", agentId: p.lead.agentId }, yield* snapshot);
  expect(read!.workspaces.map((row) => row.worktreeId)).toContain(workspace.worktreeId);
  success(yield* call(p.leadCredential, "project.changes", { projectId: p.projectId, worktreeId: workspace.worktreeId }));
})).pipe(Effect.orDie));

const refusals = {
  missing: "Start folder: folder is not an existing directory",
  "not-repository": "Git: not_repository",
  collision: "worker-worktree:collision",
  "foreign-project": "worker-worktree:project_unavailable",
  "unknown-worktree": "worker-worktree:workspace_unavailable",
  "outside-authority": "worker-worktree:project_unavailable",
} as const;
for (const kind of Object.keys(refusals) as (keyof typeof refusals)[]) {
  it.live(`failure table: ${kind} has no agent events, process, folder or branch`, () => run((runner) => Effect.gen(function* () {
    const p = yield* workerProject(rig);
    const original = git(rig.work, "for-each-ref", "--format=%(refname)", "refs/heads");
    if (kind === "missing") rmSync(p.path, { recursive: true });
    if (kind === "not-repository") rmSync(join(p.path, ".git"));
    if (kind === "collision") for (let n = 1; n <= 100; n++) git(rig.work, "branch", `worker/work-trees/builder${n === 1 ? "" : `-${n}`}`);
    const branches = kind === "collision" ? git(rig.work, "for-each-ref", "--format=%(refname)", "refs/heads") : original;
    const folders = readdirSync(join(rig.work, ".worktrees")).sort();
    const before = yield* readLog;
    const result = yield* call(kind === "outside-authority" ? p.outsideCredential : p.leadCredential, "start", workerPayload(p.path, {
      ...(kind === "foreign-project" || kind === "outside-authority" ? { projectId: kind === "outside-authority" ? p.projectId : "other" } : {}),
      ...(kind === "unknown-worktree" ? { worktreeId: "unknown" } : {}),
    }));
    expect(result._tag).toBe("Failure");
    expect(failureOf(result)).toEqual({ _tag: "CommandRefused", reason: refusals[kind] });
    expect(runner.callsOf("start")).toHaveLength(0);
    expect((yield* readLog).slice(before.length).map((row) => row.event._tag)).toEqual(["CommandRefused"]);
    expect(readdirSync(join(rig.work, ".worktrees")).sort()).toEqual(folders);
    expect(git(rig.work, "for-each-ref", "--format=%(refname)", "refs/heads")).toBe(branches);
  })).pipe(Effect.orDie));
}

it.live("restart, resume and fresh resume retain the folder; a deleted folder refuses before a process", () => Effect.gen(function* () {
  const runner = yield* makeFakeRunner();
  let workerId = "", folder = "";
  const fakes = { runner, secrets: makeFakeSecrets() };
  yield* provideRig(rig, fakes, Effect.gen(function* () {
    yield* registerRebuildCheck();
    const p = yield* workerProject(rig);
    workerId = success(yield* call(p.teamCredential, "start", workerPayload(p.teamPath))).agentId as string;
    folder = (yield* snapshot).agents[workerId]!.folder;
    success(yield* call(ownerCredential, "stop", { agentId: workerId }));
    success(yield* call(ownerCredential, "resume", { agentId: workerId }));
    expect(runner.callsOf("load").at(-1)!.input.folder).toBe(folder);
    success(yield* call(ownerCredential, "stop", { agentId: workerId }));
  }));
  // New command runtime and views rebuild from the same on-disk event log.
  yield* provideRig(rig, fakes, Effect.gen(function* () {
    yield* registerRebuildCheck();
    expect((yield* snapshot).agents[workerId]!.folder).toBe(folder);
    success(yield* call(ownerCredential, "resume", { agentId: workerId, fresh: true }));
    expect(runner.callsOf("start").at(-1)!.input.folder).toBe(folder);
    success(yield* call(ownerCredential, "stop", { agentId: workerId }));
    rmSync(folder, { recursive: true });
    const before = runner.callsOf("start").length;
    const result = yield* call(ownerCredential, "resume", { agentId: workerId, fresh: true });
    expect(failureOf(result)!.reason).toContain("Agent folder unavailable");
    expect(runner.callsOf("start")).toHaveLength(before);
    expect(existsSync(folder)).toBe(false);
  }));
}).pipe(Effect.scoped, Effect.orDie));

it.live("old AgentStarted logs omit projectId and keep decoding", () => run(Effect.gen(function* () {
  yield* workerProject(rig);
  const event = (yield* readLog).find((row) => row.event._tag === "AgentStarted")!.event;
  expect("projectId" in event).toBe(false);
  expect(Schema.decodeUnknownSync(Event)(event)).toEqual(event);
})).pipe(Effect.orDie));

for (const failure of ["before-checkout", "after-branch", "after-checkout"] as const) {
  it.live(`disk failure table: ${failure} rolls back the allocation`, () => run((runner) => Effect.gen(function* () {
    const p = yield* workerProject(rig), before = yield* readLog;
    const branches = git(rig.work, "for-each-ref", "--format=%(refname)", "refs/heads");
    const folders = readdirSync(join(rig.work, ".worktrees")).sort();
    const real = GitProcess.gitProcess;
    vi.spyOn(GitProcess, "gitProcess").mockImplementation(async (folder, args, limits, signal, truncate) => {
      if (args[0] === "worktree" && args[1] === "add") {
        if (failure === "after-branch") git(folder, "branch", args[args.indexOf("-b") + 1]!);
        if (failure === "after-checkout") await real(folder, args, limits, signal, truncate);
        return { code: 128, stdout: "", stderr: "synthetic disk failure", truncated: false };
      }
      return real(folder, args, limits, signal, truncate);
    });
    expect(failureOf(yield* call(p.leadCredential, "start", workerPayload(p.path)))!.reason).toBe("worker-worktree:git_failed");
    expect(runner.callsOf("start")).toHaveLength(0);
    expect((yield* readLog).slice(before.length).map((row) => row.event._tag)).toEqual(["CommandRefused"]);
    expect(readdirSync(join(rig.work, ".worktrees")).sort()).toEqual(folders);
    expect(git(rig.work, "for-each-ref", "--format=%(refname)", "refs/heads")).toBe(branches);
  })).pipe(Effect.orDie));
}

it.live("a project archived between allocation and append rolls back the new folder and branch", () => {
  let changed = false;
  let boundary: Effect.Effect<void> = Effect.void;
  return provideBoard(rig, (runner) => Effect.gen(function* () {
    yield* registerRebuildCheck();
    const p = yield* workerProject(rig), log = yield* EventLog;
    const branches = git(rig.work, "for-each-ref", "--format=%(refname)", "refs/heads");
    boundary = Effect.suspend(() => {
      if (changed || !existsSync(join(rig.work, ".worktrees", "worker-work-trees-builder"))) return Effect.void;
      changed = true;
      return log.append({ _tag: "owner" }, freshRequest(), "e".repeat(64),
        [eventOf({ _tag: "project.ArchiveStarted", projectId: p.projectId })], {}).pipe(Effect.asVoid, Effect.orDie);
    });
    expect(failureOf(yield* call(p.leadCredential, "start", workerPayload(p.path)))!.reason).toBe("worker-worktree:project_unavailable");
    expect(changed).toBe(true);
    expect(runner.callsOf("start")).toHaveLength(0);
    expect(existsSync(join(rig.work, ".worktrees", "worker-work-trees-builder"))).toBe(false);
    expect(git(rig.work, "for-each-ref", "--format=%(refname)", "refs/heads")).toBe(branches);
  }), [], {}, Effect.suspend(() => boundary)).pipe(Effect.orDie);
});

it.live("team deletion preserves unmerged commits, dirt and registered worker checkouts", () => run(Effect.gen(function* () {
  const p = yield* workerProject(rig);
  const reply = success(yield* call(p.teamCredential, "start", workerPayload(p.teamPath, { groups: [p.team.group] })));
  const agent = (yield* snapshot).agents[reply.agentId]!;
  workerCommit(agent.folder, "unmerged-team-worker.txt");
  writeFileSync(join(agent.folder, "dirty.txt"), "preserve\n");
  success(yield* call(ownerCredential, "stop", { agentId: reply.agentId }));
  success(yield* call(ownerCredential, "stop", { agentId: p.teamLead.agentId }));
  success(yield* call(ownerCredential, "project.team.delete", { projectId: p.projectId, teamId: p.team.teamId }));
  expect(readFileSync(join(agent.folder, "dirty.txt"), "utf8")).toBe("preserve\n");
  expect(git(agent.folder, "log", "-1", "--format=%s")).toBe("unmerged-team-worker.txt");
  expect(Object.values(projectView(yield* snapshot).workspaces).some((row) => row.path === agent.folder)).toBe(true);
})).pipe(Effect.orDie));

it.live("service recovery reloads an allocated worker from its original folder", () => Effect.gen(function* () {
  const original = yield* makeFakeRunner();
  let workerId = "", folder = "";
  yield* provideRig(rig, { runner: original, secrets: makeFakeSecrets() }, Effect.gen(function* () {
    yield* registerRebuildCheck();
    const p = yield* workerProject(rig);
    workerId = success(yield* call(p.leadCredential, "start", workerPayload(p.path))).agentId;
    folder = (yield* snapshot).agents[workerId]!.folder;
  }));
  const replacement = yield* makeFakeRunner();
  yield* provideRestart(rig, replacement.layer, Effect.gen(function* () {
    yield* registerRebuildCheck(); yield* replacement.subscribed;
    yield* recover;
    expect(yield* ready).toBe(true);
    expect(replacement.callsOf("load").find((call) => call.input.agentId === workerId)!.input.folder).toBe(folder);
    expect((yield* snapshot).agents[workerId]!.folder).toBe(folder);
    expect(git(folder, "branch", "--show-current")).toBe("worker/work-trees/builder");
  }));
}).pipe(Effect.scoped, Effect.orDie) as Effect.Effect<void>);

it.live("a read-only reviewer can select a sibling worker's registered checkout", () => run((runner) => Effect.gen(function* () {
  const p = yield* workerProject(rig);
  const builder = success(yield* call(p.teamCredential, "start", workerPayload(p.teamPath)));
  const state = yield* snapshot, folder = state.agents[builder.agentId]!.folder;
  const workspace = Object.values(projectView(state).workspaces).find((row) => row.path === folder)!;
  const reviewer = success(yield* call(p.teamCredential, "start", workerPayload(p.teamPath, {
    name: "Review", role: "reviewer", permission: "read-only", worktreeId: workspace.worktreeId,
  })));
  expect((yield* snapshot).agents[reviewer.agentId]!.folder).toBe(folder);
  expect(runner.callsOf("start").at(-1)!.input).toMatchObject({ folder, permission: "read-only" });
  expect(Object.values(projectView(yield* snapshot).workspaces).filter((row) => row.owner !== undefined)).toHaveLength(1);
})).pipe(Effect.orDie));

it.live("concurrent starts reserve distinct worktrees before either session opens", () => run((runner) => Effect.gen(function* () {
  const p = yield* workerProject(rig);
  const results = yield* Effect.all([
    call(p.teamCredential, "start", workerPayload(p.teamPath)),
    call(p.teamCredential, "start", workerPayload(p.teamPath)),
  ], { concurrency: "unbounded" });
  const state = yield* snapshot;
  const folders = results.map((result) => state.agents[success(result).agentId]!.folder);
  expect(new Set(folders).size).toBe(2);
  expect(new Set(folders.map((folder) => git(folder, "branch", "--show-current")))).toEqual(new Set(["worker/work-trees/builder", "worker/work-trees/builder-2"]));
  expect(runner.callsOf("start")).toHaveLength(2);
})).pipe(Effect.orDie));

it.live("r2 m8: the start table has no duplicate non-project starter rows", () => Effect.sync(() => {
  const keys = startCases.map((row) => JSON.stringify([row.project ? row.starter : row.starter === "owner" ? "owner" : "outside-lead", row.role, row.project, row.named]));
  expect(new Set(keys).size).toBe(startCases.length);
}));
