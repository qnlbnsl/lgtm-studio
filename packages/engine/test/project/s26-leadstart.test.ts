import { existsSync, mkdirSync, readFileSync, readdirSync, renameSync, rmSync, symlinkSync, writeFileSync } from "node:fs";
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
import * as GitProcess from "../../src/project/git/process.ts";
import { agentCredential, appendSetup, call, eventOf, failureOf, harnessSettings, makeFakeSecrets, makeRig, ownerCredential, provideRig, readLog, removeRig, seedAgent, success, type Rig } from "../support/handlerRig.ts";
import { workerGit, workerPayload, workerProject } from "../support/workerWorktrees.ts";

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
    success(yield* call(ownerCredential, "harnesses", { operation: "set", harness: harnessSettings("first", { defaultPermission: "auto" }) }));
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
  expect(failureOf(yield* call(p.leadCredential, "start", workerPayload(p.path, { role: "lead:team" })))!.reason).toBe("Start folder: folder contains a protected folder");
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
  expect(failureOf(yield* call(p.leadCredential, "start", workerPayload(p.path, { role: "lead:team" })))!.reason).toBe("Start folder: folder contains a protected folder");
  expect(runner.callsOf("start")).toHaveLength(0);
})));

it.live("s26: a secret-referenced harness login folder also stays protected", () => run(runner => Effect.gen(function* () {
  const p = yield* workerProject(rig);
  const login = join(p.path, "private-login"); mkdirSync(login);
  yield* (yield* SecretStore).put("secret:login-folder", Redacted.make(login));
  yield* appendSetup([eventOf({ _tag: "AccountChanged", accountId: "login-account", account: { id: "login-account", harness: "claude", environment: { CLAUDE_CONFIG_DIR: "secret:login-folder" } } })]);
  expect(failureOf(yield* call(p.leadCredential, "start", workerPayload(p.path, { role: "lead:team" })))!.reason).toBe("Start folder: folder contains a protected folder");
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
  expect(failureOf(yield* call(p.leadCredential, "start", workerPayload(p.path, { role: "lead:team" })))!.reason).toBe("Start folder: folder contains a protected folder");
  expect(runner.callsOf("start")).toHaveLength(0);
})));

const mainLead = (p: { projectId: string; mainId: string; lead: { agentId: string } }) => Effect.gen(function* () {
  yield* appendSetup([eventOf({ _tag: 'project.Placed', projectId: p.projectId, worktreeId: p.mainId, machineId: 'local' })]);
  return yield* seedAgent({ id: 'main-lead', role: 'lead:team', folder: rig.work, parent: p.lead.agentId, permission: 'full' });
});

for (const permission of ['read-only', 'edit'] as const) it.live(`s26 r2 M3: a broad project root cannot grant another project's nested checkout with ${permission}`, () => run(runner => Effect.gen(function* () {
  const p = yield* workerProject(rig), lead = yield* mainLead(p);
  const other = yield* seedAgent({ id: 'other-project-lead', role: 'lead:project', folder: p.path });
  yield* appendSetup([eventOf({ _tag: 'project.Created', projectId: 'other-project', repoId: p.repoId, worktreeId: p.worktreeId,
    leadAgentId: other.agentId, machineId: 'local', name: 'Other project', goal: 'Stay separate' })]);
  const folder = join(p.path, 'src'); mkdirSync(folder);
  const process = vi.spyOn(GitProcess, 'gitProcess');
  expect(failureOf(yield* call(agentCredential(lead.token), 'start', workerPayload(folder, { role: 'lead:team', permission })))!.reason)
    .toBe("Start folder: This folder belongs to a nested worktree outside the starter's project.");
  expect(process).not.toHaveBeenCalled(); expect(runner.callsOf('start')).toHaveLength(0);
})));

it.live('s26 r2 M3: an unregistered linked checkout under a broad root cannot inherit its grant', () => run(runner => Effect.gen(function* () {
  const p = yield* workerProject(rig), lead = yield* mainLead(p);
  const folder = join(rig.work, '.worktrees', 'hand-made');
  workerGit(rig.work, 'worktree', 'add', '-b', 'hand-made', folder);
  const nested = join(folder, 'src'); mkdirSync(nested);
  const process = vi.spyOn(GitProcess, 'gitProcess');
  expect(failureOf(yield* call(agentCredential(lead.token), 'start', workerPayload(nested, { role: 'lead:team', permission: 'read-only' })))!.reason)
    .toBe("Start folder: This nested checkout is not registered to the starter's project.");
  expect(process).not.toHaveBeenCalled(); expect(runner.callsOf('start')).toHaveLength(0);
})));

it.live('s26 r2 m1: replacing an allocated registration with a symlink cannot follow another valid own checkout', () => run(runner => Effect.gen(function* () {
  const p = yield* workerProject(rig);
  const first = success(yield* call(p.leadCredential, 'start', workerPayload(p.path)));
  const folder = (yield* snapshot).agents[first.agentId]!.folder;
  renameSync(folder, folder + '-original'); symlinkSync(p.teamPath, folder);
  const count = runner.callsOf('start').length;
  expect(failureOf(yield* call(p.leadCredential, 'start', workerPayload(folder, { name: 'Alias child', role: 'lead:team', permission: 'read-only' })))!.reason)
    .toBe("Start folder: The registered worktree path is now a symbolic link. Ask the owner to restore its directory before retrying.");
  expect(runner.callsOf('start')).toHaveLength(count);
})));

for (const location of ['home', 'outside-home'] as const) for (const kind of ['relative', 'file'] as const) it.live(`s26 r2 m3: an unrelated ${kind} login configuration does not block starts or resume ${location}`, () => {
  if (location === 'outside-home') { const work = join(rig.root, 'repository-outside'); renameSync(rig.work, work); rig = { ...rig, work }; }
  return run(runner => Effect.gen(function* () {
  const p = yield* workerProject(rig);
  const bad = kind === 'relative' ? '.' : join(rig.root, 'not-a-login-directory');
  if (kind === 'file') writeFileSync(bad, 'a regular file');
  yield* appendSetup([eventOf({ _tag: 'AccountChanged', accountId: 'bad-home', account: { id: 'bad-home', harness: 'codex', environment: { CODEX_HOME: bad } } })]);
  const first = success(yield* call(p.leadCredential, 'start', workerPayload(p.path)));
  success(yield* call(ownerCredential, 'stop', { agentId: first.agentId }));
  success(yield* call(ownerCredential, 'resume', { agentId: first.agentId, fresh: true }));
  expect(runner.callsOf('start')).toHaveLength(2);
  }));
});

it.live('s26 r2 m4: omitted worker permission honors a non-ask saved harness default', () => run(runner => Effect.gen(function* () {
  const p = yield* workerProject(rig);
  success(yield* call(ownerCredential, 'harnesses', { operation: 'set', harness: harnessSettings('first', { defaultPermission: 'edit' }) }));
  const lead = yield* seedAgent({ id: 'full-lead', role: 'lead:team', folder: p.path, parent: p.lead.agentId, permission: 'full' });
  const payload = workerPayload(p.path); delete (payload as Partial<typeof payload>).permission;
  const child = success(yield* call(agentCredential(lead.token), 'start', payload));
  expect((yield* snapshot).agents[child.agentId]!.permission).toBe('edit');
  expect(runner.callsOf('start').at(-1)!.input.permission).toBe('edit');
})));

it.live('s26 r2: a main-checkout starter allocates beside the nested service but cannot start a child in main', () => {
  const service = join(rig.work, '.worktrees', 'service'); mkdirSync(service, { recursive: true });
  rig = { ...rig, pluginRoot: service };
  return run(runner => Effect.gen(function* () {
    const p = yield* workerProject(rig), lead = yield* mainLead(p);
    const before = { folders: readdirSync(join(rig.work, '.worktrees')).sort(), branches: workerGit(rig.work, 'for-each-ref', '--format=%(refname)', 'refs/heads') };
    const process = vi.spyOn(GitProcess, 'gitProcess');
    expect(failureOf(yield* call(agentCredential(lead.token), 'start', workerPayload(rig.work, { role: 'lead:team' })))!.reason).toMatch(/protected folder/);
    expect(process).not.toHaveBeenCalled(); expect(runner.callsOf('start')).toHaveLength(0);
    expect({ folders: readdirSync(join(rig.work, '.worktrees')).sort(), branches: workerGit(rig.work, 'for-each-ref', '--format=%(refname)', 'refs/heads') }).toEqual(before);
    const child = success(yield* call(agentCredential(lead.token), 'start', workerPayload(rig.work)));
    const folder = (yield* snapshot).agents[child.agentId]!.folder;
    expect(folder).not.toBe(service); expect(folder).not.toBe(rig.work);
    expect(existsSync(join(folder, '.git'))).toBe(true);
    expect(runner.callsOf('start').at(-1)!.input.folder).toBe(folder);
    success(yield* call(ownerCredential, 'stop', { agentId: child.agentId }));
    success(yield* call(ownerCredential, 'resume', { agentId: child.agentId, fresh: true }));
  }));
});

it.live('s26 r2 m5: a protected source refuses before Git and preserves its inventory', () => {
  rig = { ...rig, pluginRoot: rig.work };
  return run(runner => Effect.gen(function* () {
    const p = yield* workerProject(rig);
    const before = { folders: readdirSync(join(rig.work, '.worktrees')).sort(), branches: workerGit(rig.work, 'for-each-ref', '--format=%(refname)', 'refs/heads'), workspaces: projectView(yield* snapshot).workspaces };
    const process = vi.spyOn(GitProcess, 'gitProcess');
    expect(failureOf(yield* call(p.leadCredential, 'start', workerPayload(p.path)))!.reason).toBe('Start folder: folder is, or is inside, a protected folder');
    expect(process).not.toHaveBeenCalled(); expect(runner.callsOf('start')).toHaveLength(0);
    expect({ folders: readdirSync(join(rig.work, '.worktrees')).sort(), branches: workerGit(rig.work, 'for-each-ref', '--format=%(refname)', 'refs/heads'), workspaces: projectView(yield* snapshot).workspaces }).toEqual(before);
  }));
});

it.live('s26 r2 m3: unrelated invalid login paths do not prevent service recovery', () => Effect.gen(function* () {
  const runner = yield* makeFakeRunner(); let workerId = '';
  yield* provideRig(rig, { runner, secrets: makeFakeSecrets() }, Effect.gen(function* () {
    yield* registerRebuildCheck();
    const p = yield* workerProject(rig);
    workerId = success(yield* call(p.leadCredential, 'start', workerPayload(p.path))).agentId;
    const file = join(rig.root, 'not-a-login-folder'); writeFileSync(file, 'file');
    yield* appendSetup([
      eventOf({ _tag: 'AccountChanged', accountId: 'bad-relative', account: { id: 'bad-relative', harness: 'codex', environment: { CODEX_HOME: '.' } } }),
      eventOf({ _tag: 'AccountChanged', accountId: 'bad-file', account: { id: 'bad-file', harness: 'cursor', home: file } }),
    ]);
  }));
  const replacement = yield* makeFakeRunner();
  yield* provideRestart(rig, replacement.layer, Effect.gen(function* () {
    yield* registerRebuildCheck(); yield* replacement.subscribed; yield* recover;
    expect(yield* ready).toBe(true);
    expect(replacement.callsOf('load').some(call => call.input.agentId === workerId)).toBe(true);
  }));
}).pipe(Effect.scoped, Effect.orDie) as Effect.Effect<void>);

it.live('s26 r2: an owner-started main-checkout lead allocates beside the protected service', () => {
  const service = join(rig.work, '.worktrees', 'service'); mkdirSync(service, { recursive: true });
  rig = { ...rig, pluginRoot: service };
  return run(runner => Effect.gen(function* () {
    const p = yield* workerProject(rig);
    yield* appendSetup([eventOf({ _tag: 'project.Placed', projectId: p.projectId, worktreeId: p.mainId, machineId: 'local' })]);
    const lead = success(yield* call(ownerCredential, 'start', workerPayload(rig.work, { name: 'Owner main lead', role: 'lead:team', projectId: p.projectId, permission: 'full' })));
    expect((yield* snapshot).agents[lead.agentId]!.parent).toEqual({ _tag: 'owner' });
    const token = runner.callsOf('start').at(-1)!.input.token;
    const child = success(yield* call(agentCredential(Redacted.value(token)), 'start', workerPayload(rig.work)));
    const folder = (yield* snapshot).agents[child.agentId]!.folder;
    expect(folder.startsWith(join(rig.work, '.worktrees') + '/')).toBe(true);
    expect(folder).not.toBe(service);
    const workspace = Object.values(projectView(yield* snapshot).workspaces).find(row => row.path === folder)!;
    expect(workspace.parentId).toBe(p.mainId);
    expect(workspace.owner).toBe(child.agentId);
    expect(runner.callsOf('start')).toHaveLength(2);
    expect((yield* snapshot).agents[lead.agentId]!.permission).toBe('full');
  }));
});

for (const permission of ['read-only', 'edit'] as const) it.live(`s26 r3 m2: a foreign registered checkout outside home refuses ${permission} without an enclosing own root`, () => {
  const work = join(rig.root, 'outside-repository'); renameSync(rig.work, work); rig = { ...rig, work };
  return run(runner => Effect.gen(function* () {
    const p = yield* workerProject(rig), path = join(rig.work, '.worktrees', 'other-project');
    workerGit(rig.work, 'worktree', 'add', '-b', 'other-project', path);
    const row = success(yield* call(ownerCredential, 'project.workspace.register', { repoId: p.repoId, path, branch: 'other-project', machineId: 'local', checkout: 'worktree', parentId: p.mainId }));
    const other = yield* seedAgent({ id: 'other-lead', role: 'lead:project', folder: path });
    yield* appendSetup([eventOf({ _tag: 'project.Created', projectId: 'other-project', repoId: p.repoId, worktreeId: row.worktreeId,
      leadAgentId: other.agentId, machineId: 'local', name: 'Other project', goal: 'Keep checkouts separate' })]);
    const starter = yield* seedAgent({ id: 'outside-full-lead', role: 'lead:team', folder: p.path, parent: p.lead.agentId, permission: 'full' });
    expect(failureOf(yield* call(agentCredential(starter.token), 'start', workerPayload(path, { role: 'lead:team', permission })))?.reason)
      .toBe("Start folder: This folder belongs to a worktree outside the starter's project.");
    expect(runner.callsOf('start')).toHaveLength(0);
  }));
});

it.live('s26 r3 m1: a clean linked source cannot allocate into a protected main checkout before Git mutates it', () => {
  rig = { ...rig, pluginRoot: rig.work };
  return run(runner => Effect.gen(function* () {
    const p = yield* workerProject(rig), source = join(rig.home, 'separate-checkout');
    workerGit(rig.work, 'worktree', 'remove', p.teamPath);
    workerGit(rig.work, 'worktree', 'move', p.path, source);
    rmSync(join(rig.work, '.worktrees'), { recursive: true });
    const exclude = join(rig.work, '.git', 'info', 'exclude'); writeFileSync(exclude, 'owner exclusion\n');
    const view = projectView(yield* snapshot);
    yield* appendSetup([eventOf({ _tag: 'project.WorktreeCreated', ...view.workspaces[p.worktreeId]!, path: source })]);
    const lead = yield* seedAgent({ id: 'clean-source-lead', role: 'lead:team', folder: source, parent: p.lead.agentId });
    const before = { branches: workerGit(rig.work, 'for-each-ref', '--format=%(refname)', 'refs/heads'), workspaces: projectView(yield* snapshot).workspaces };
    const process = vi.spyOn(GitProcess, 'gitProcess');
    expect(failureOf(yield* call(agentCredential(lead.token), 'start', workerPayload(source)))?.reason)
      .toBe('Worker allocation folder: folder is, or is inside, a protected folder');
    expect(process.mock.calls.some(([, args]) => args.includes('worktree') && args.includes('add'))).toBe(false);
    expect(existsSync(join(rig.work, '.worktrees'))).toBe(false);
    expect(readFileSync(exclude, 'utf8')).toBe('owner exclusion\n');
    expect({ branches: workerGit(rig.work, 'for-each-ref', '--format=%(refname)', 'refs/heads'), workspaces: projectView(yield* snapshot).workspaces }).toEqual(before);
    expect(runner.callsOf('start')).toHaveLength(0);
  }));
});

it.live('s26 r3 m7: a main-checkout lead may start a child in a clean subfolder beside the service', () => {
  const service = join(rig.work, '.worktrees', 'service'); mkdirSync(service, { recursive: true });
  rig = { ...rig, pluginRoot: service };
  return run(runner => Effect.gen(function* () {
    const p = yield* workerProject(rig), lead = yield* mainLead(p), folder = join(rig.work, 'packages'); mkdirSync(folder);
    success(yield* call(agentCredential(lead.token), 'start', workerPayload(folder, { role: 'lead:team' })));
    expect(runner.callsOf('start').at(-1)!.input.folder).toBe(folder);
  }));
});
