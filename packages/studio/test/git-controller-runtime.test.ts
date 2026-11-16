import * as refreshPolicy from '../src/gitRefresh.ts';
import { readFileSync } from "node:fs";
import { expect, it, vi } from "vitest";
import { gitActionsSource } from "../scripts/gitActions.ts";

const copied = (path: string) => gitActionsSource(path, readFileSync(`${process.cwd()}/design/mock/${path}`, "utf8"));
const gate = () => { let release!: () => void; const held = new Promise<void>((resolve) => { release = resolve; }); return { held, release }; };
function fixture(request: (tag: string, payload: any) => Promise<unknown>) {
  const project = { id: "project", repo: "repo", lead: "lead" };
  const state: any = { session: { status: "in" }, ui: {}, projects: [project], inspecting: {}, gitUi: { workspace: { tab: "conflicts" } },
    worktrees: [{ id: "workspace", project: project.id, repo: project.repo, branch: "project", machine: "local", kind: "project", files: [], commits: [], merge: null,
      review: { id: "review", project: project.id, head: "head", baseHead: "base", agent: "lead" } }] };
  const calls: any[] = [], active = new Set<unknown>(); let sequence = 0;
  const send = async (tag: string, payload: any, options: any = {}) => {
    options.onRequestId?.("own-" + ++sequence);
    const call = { tag, payload, signal: options.signal }; calls.push(call); active.add(call);
    let stop = () => {};
    const abort = new Promise<never>((_, reject) => { stop = () => reject(new DOMException("cancelled", "AbortError")); if (options.signal?.aborted) stop(); else options.signal?.addEventListener("abort", stop, { once: true }); });
    try { return await Promise.race([request(tag, payload), abort]); } finally { options.signal?.removeEventListener("abort", stop); active.delete(call); }
  };
  const dialogs: any[] = [], toasts: string[] = [], listeners = new Map<string, () => void>();
  const act: any = { confirm: (d: any) => dialogs.push(d), closeDialog: vi.fn(), openDialog: vi.fn(), toast: (s: string) => toasts.push(s) };
  const source = copied("js/live/git.js").replace(/^import .*;$/gm, "").replace(/export /g, "");
  const actions = new Function("store", "live", "engineWrite", "notify", "engine", "act", "update", "addEventListener", "removeEventListener",
    `${source}\nreturn { installMappings, openScreen, refreshGit, observe, busy, declineReview, discard, stage, integrate, selectGit, commit, abortMerge, resolve, gitDetail, openReview, choose };`)(
    { state }, { on: true }, (p: object) => Object.assign(state, p), () => {}, { request: send }, act, (f: (s: any) => void) => f(state), (name: string, fn: () => void) => listeners.set(name, fn), (name: string) => listeners.delete(name));
  actions.installMappings({ ...refreshPolicy, mapGitState: (s: any) => s, mapGitChanges: (items: any[]) => items,
    mapGitCommit: (c: any) => c, mapCommitFiles: (items: any[]) => items, mapGitDiff: (r: any) => r,
    mapMainReview: (r: any) => r, mapConflict: (c: any) => ({ ...c, id: c.path, selection: c }) });
  return { state, actions, act, dialogs, toasts, listeners, calls, active };
}

async function gitTabView(f: ReturnType<typeof fixture>, panels: Record<string, any> = {}) {
  const kit = await import("../../../design/mock/vendor/htm-preact/standalone.module.js" as string);
  const source = copied("js/screens/git.js");
  const component = source.slice(source.indexOf("export function GitTab(")).replace("export ", "");
  const GitTab = new Function("html", "useEffect", "useMedia", "useState", "persist", "remember", "live", "liveGit", "agentOf", "machineOf", "Menu", "Icon", "AheadBehind", "Integrate", "Changes", "History", "Conflicts", "KIND_WORD", "ownerText", "plural", `${component}\nreturn GitTab;`)(
    kit.html, () => {}, () => false, () => ["unified", () => {}], () => "unified", () => {}, { on: true }, f.actions, () => null, () => ({ name: "local" }),
    () => null, () => null, () => null, () => null, panels.Changes || (() => null), panels.History || (() => null), panels.Conflicts || (() => null), {}, () => "Project lead", (n: number, word: string) => `${n} ${word}`);
  const element = document.createElement("div"); document.body.append(element);
  return { element, draw: () => kit.render(kit.h(GitTab, { s: f.state, project: f.state.projects[0] }), element), close: () => { kit.render(null, element); element.remove(); } };
}

it("stale sentence persists after the toast expires and clears on a successful forced read", async () => {
  vi.useFakeTimers(); let fail = true;
  const f = fixture(async (tag) => { if (tag === 'project.git.state' && fail) throw new Error('private path'); return emptyGitReply(tag); });
  const view = await gitTabView(f), close = f.actions.openScreen('git', 'project');
  try {
    await vi.advanceTimersByTimeAsync(0); f.toasts.length = 0; view.draw();
    expect(view.element.querySelector('[role="alert"]')?.textContent).toBe('This worktree could not be refreshed.');
    expect(view.element.textContent).not.toContain('private path');
    await vi.advanceTimersByTimeAsync(61000); view.draw();
    expect(f.state.worktrees[0].stale).toBe(true);
    expect(view.element.textContent).toContain('This worktree could not be refreshed.');
    fail = false; await f.actions.refreshGit('project'); view.draw();
    expect(f.state.worktrees[0].stale).toBe(false);
    expect(view.element.textContent).not.toContain('This worktree could not be refreshed.');
  } finally { close(); view.close(); vi.useRealTimers(); }
});
const emptyGitReply = (tag: string) => {
  if (tag === "project.git.state") return { result: { mergeHead: null } };
  if (["project.changes", "project.history", "project.branches", "project.commit.files", "project.conflict.resolved"].includes(tag)) return { result: { items: [], truncated: false } };
  if (tag === "project.review.status") return { result: { review: null } };
  throw new Error(tag);
};
it("a superseded background state read stops before requesting changes", async () => {
  const held = gate(); let states = 0, changes = 0;
  const f = fixture(async (tag) => {
    if (tag === "project.git.state") { if (++states === 1) await held.held; return { result: { mergeHead: null } }; }
    if (tag === "project.changes") { changes++; return { result: { items: [], truncated: false } }; }
    if (tag === "project.review.status") return { result: { review: null } };
    throw new Error(tag);
  });
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.waitFor(() => expect(states).toBe(1));
    await f.actions.refreshGit("project");
    expect(changes).toBe(1);
    held.release();
    await vi.waitFor(() => expect(f.actions.busy("git-read")).toBe(false));
    expect(changes).toBe(1);
  } finally { held.release(); close(); }
});


it("retries the initial full read after a view selection supersedes it", async () => {
  vi.useFakeTimers();
  const first = gate(), reads: string[] = [];
  const f = fixture(async (tag, payload) => {
    if (tag === "project.git.state") { reads.push(payload.worktreeId); if (reads.length === 1) await first.held; return { result: { mergeHead: null } }; }
    if (tag === "project.changes") return { result: { items: [], truncated: false } };
    if (tag === "project.review.status") return { result: { review: null } };
    throw new Error(tag);
  });
  f.state.worktrees.push({ ...f.state.worktrees[0], id: "other", branch: "other", kind: "team" });
  const close = f.actions.openScreen("git", "project");
  try {
    expect(reads).toEqual(["workspace"]);
    await f.actions.selectGit("workspace", "changes");
    first.release();
    await vi.advanceTimersByTimeAsync(1000);
    expect(reads).toContain("other");
    expect(f.actions.busy("git-read")).toBe(false);
    expect(f.toasts).toEqual([]);
  } finally { first.release(); close(); vi.useRealTimers(); }
});


for (const tab of ["history", "conflicts", "changes"]) it(`recovers an initial read superseded by ${tab}`, async () => {
  vi.useFakeTimers();
  const held = gate(), reads: string[] = [];
  const f = fixture(async (tag, payload) => {
    if (tag === "project.git.state") { reads.push(payload.worktreeId); if (reads.length === 1) await held.held; }
    return emptyGitReply(tag);
  });
  f.state.worktrees.push({ ...f.state.worktrees[0], id: "other", branch: "other", kind: "team" });
  const close = f.actions.openScreen("git", "project");
  try {
    await f.actions.selectGit("workspace", tab, tab === "history" ? "selected-commit" : undefined);
    held.release(); await vi.advanceTimersByTimeAsync(1000);
    expect(reads).toContain("other");
    expect(f.toasts).toEqual([]);
  } finally { held.release(); close(); vi.useRealTimers(); }
});

it("treats a missing document as visible and skips screen listeners", async () => {
  vi.useFakeTimers();
  const f = fixture(async tag => emptyGitReply(tag));
  let close: (() => void) | undefined;
  vi.stubGlobal("document", undefined);
  try {
    close = f.actions.openScreen("git", "project");
    await vi.advanceTimersByTimeAsync(0);
    expect(f.state.worktrees[0].ready).toBe(true);
    expect(f.listeners.size).toBe(0);
    expect(f.toasts).toEqual([]);
    close!(); close = undefined;
  } finally { close?.(); vi.unstubAllGlobals(); vi.useRealTimers(); }
});

it("round 4: focus queues a background read and an immediate owner click is sent", async () => {
  vi.useFakeTimers(); const held = gate(); let hold = false, stages = 0;
  const f = fixture(async tag => {
    if (tag === "project.stage.all") { stages++; return { requestId: "stage" }; }
    if (hold && tag === "project.git.state") await held.held;
    return emptyGitReply(tag);
  });
  const view = await gitTabView(f);
  const close = f.actions.openScreen("git", "project"); let action: Promise<unknown> | undefined;
  try {
    await vi.advanceTimersByTimeAsync(0); hold = true;
    f.listeners.get("focus")!();
    expect(f.actions.busy("git")).toBe(false);
    view.draw(); expect(view.element.querySelector<HTMLFieldSetElement>("fieldset")!.disabled).toBe(false);
    expect(view.element.querySelector("button:disabled")).toBeNull();
    action = f.actions.stage("workspace", [], true, true);
    expect(stages).toBe(1);
    held.release(); await action;
  } finally { held.release(); await action; close(); view.close(); vi.useRealTimers(); }
});

it("round 4: focus and visibility share one background refresh", async () => {
  vi.useFakeTimers(); let reads = 0;
  const f = fixture(async tag => { if (tag === "project.git.state") reads++; return emptyGitReply(tag); });
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.advanceTimersByTimeAsync(0); reads = 0;
    f.listeners.get("focus")!(); document.dispatchEvent(new Event("visibilitychange"));
    await vi.advanceTimersByTimeAsync(200);
    expect(reads).toBe(1); expect(f.actions.busy("git")).toBe(false);
  } finally { close(); vi.useRealTimers(); }
});

it("round 4: a focus read preserves Mark resolved while refreshing the same merge", async () => {
  vi.useFakeTimers(); const held = gate(); let hold = false, resolves = 0;
  const f = fixture(async tag => {
    if (tag === "project.git.state") return { result: { head: "head", mergeHead: "merge" } };
    if (tag === "project.changes") return { result: { items: [{ path: "text.txt", conflict: true }], truncated: false } };
    if (tag === "project.conflict") { if (hold) await held.held; return { result: { path: "text.txt", head: "head", fingerprint: "fingerprint", parts: [{ id: "hunk", ours: ["ours"], theirs: ["theirs"], choice: "ours" }], choices: ["ours"] } }; }
    if (tag === "project.conflict.resolve") { resolves++; return {}; }
    return emptyGitReply(tag);
  });
  const kit = await import("../../../design/mock/vendor/htm-preact/standalone.module.js" as string);
  const source = copied("js/screens/git.js");
  const component = source.slice(source.indexOf("const CHOICES"), source.indexOf("// ---- integrate strip"));
  const Conflicts = new Function("html", "live", "update", "Icon", "cx", "baseName", "dirName", "plural", "markResolved", `${component}\nreturn Conflicts;`)(
    kit.html, { on: true }, () => {}, () => null, (...p: unknown[]) => p.filter(Boolean).join(" "), (p: string) => p, () => "", (n: number, word: string) => `${n} ${word}`, (id: string, file: string) => f.actions.resolve(id, file, true));
  const view = await gitTabView(f, { Conflicts });
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.advanceTimersByTimeAsync(0); hold = true;
    f.listeners.get("focus")!(); await vi.advanceTimersByTimeAsync(100);
    expect(f.actions.busy("git-read")).toBe(true); expect(f.actions.busy("git")).toBe(false);
    view.draw();
    const button = [...view.element.querySelectorAll<HTMLButtonElement>("button")].find(b => b.textContent === "Mark resolved")!;
    expect(button.disabled).toBe(false); button.click(); expect(resolves).toBe(1);
    held.release(); await vi.advanceTimersByTimeAsync(0);
    expect(f.actions.busy("git")).toBe(false);
  } finally { held.release(); close(); view.close(); vi.useRealTimers(); }
});

for (const known of [null, 'answered']) it(`issue 1042: an event discovers a review after known ${known}`, async () => {
  vi.useFakeTimers(); let review: any = known ? { id: 'old', status: known } : null, reviews = 0;
  const f = fixture(async tag => { if (tag === 'project.review.status') { reviews++; return { result: { review } }; } return emptyGitReply(tag); });
  const close = f.actions.openScreen('git', 'project');
  try {
    await vi.advanceTimersByTimeAsync(0); expect(reviews).toBe(1);
    review = { id: 'remote', status: 'open' };
    f.actions.observe({ git: { group: 'review', worktreeIds: ['workspace'] } });
    await vi.advanceTimersByTimeAsync(100);
    expect(reviews).toBe(2); expect(f.state.worktrees[0].review.id).toBe('remote');
    expect(f.calls.filter(c => c.tag === 'project.git.state')).toHaveLength(1);
  } finally { close(); vi.useRealTimers(); }
});

it('issue 1043: an old visit cannot block or clear the new visit first load', async () => {
  vi.useFakeTimers(); const old = gate(), fresh = gate(); let states = 0;
  const f = fixture(async tag => { if (tag === 'project.git.state') { if (++states === 1) await old.held; else await fresh.held; } return emptyGitReply(tag); });
  const closeOld = f.actions.openScreen('git', 'project'); closeOld();
  const close = f.actions.openScreen('git', 'project');
  try {
    expect(states).toBe(2); old.release(); await vi.advanceTimersByTimeAsync(0);
    expect(f.actions.busy('git-read')).toBe(true);
    fresh.release(); await vi.advanceTimersByTimeAsync(0);
    expect(f.state.worktrees[0]).toMatchObject({ ready: true, reviewLoaded: true });
    expect(f.actions.busy('git-read')).toBe(false);
  } finally { old.release(); fresh.release(); close(); vi.useRealTimers(); }
});

it('issue 1036: the original integrate refusal survives a failing confirmation', async () => {
  const f = fixture(async tag => { if (tag === 'project.integrate') throw new Error('The destination is dirty.'); if (tag === 'project.git.state') throw new Error('second failure'); return emptyGitReply(tag); });
  Object.assign(f.state.worktrees[0], { kind: 'team', parentId: 'parent' });
  f.state.worktrees.push({ ...f.state.worktrees[0], id: 'parent', kind: 'project' });
  const close = f.actions.openScreen('git', 'project');
  try {
    await vi.waitFor(() => expect(f.actions.busy('git-read')).toBe(false)); f.toasts.length = 0;
    expect(await f.actions.integrate('workspace')).toBe(false);
    expect(f.toasts).toContain('The destination is dirty.');
    expect(f.state.worktrees.every((w: any) => w.stale)).toBe(true);
    expect(f.calls.filter(c => c.tag === 'project.git.state').map(c => c.payload.worktreeId)).toEqual(['workspace', 'parent', 'workspace', 'parent']);
  } finally { close(); }
});

for (const lifecycle of ['close', 'hide', 'remove', 'replace', 'logout']) for (const detail of [false, true]) it(`${lifecycle} aborts a held ${detail ? 'detail' : 'summary'} with no late publication`, async () => {
  vi.useFakeTimers(); const held = gate(); let hold = false;
  const f = fixture(async tag => {
    if (hold && tag === (detail ? 'project.diff' : 'project.git.state')) await held.held;
    if (tag === 'project.changes') return { result: { items: [{ id: 'file', path: 'file', staged: false }] } };
    if (tag === 'project.diff') return { result: { hunks: ['fresh'] } };
    return emptyGitReply(tag);
  });
  f.state.gitUi.workspace.tab = 'changes';
  const close = f.actions.openScreen('git', 'project');
  try {
    await vi.advanceTimersByTimeAsync(0); hold = true;
    f.listeners.get('focus')!(); await vi.advanceTimersByTimeAsync(100);
    expect(f.active.size).toBe(1); const pendingCall = f.calls.at(-1);
    if (lifecycle === 'close') close();
    if (lifecycle === 'hide') { vi.spyOn(document, 'hidden', 'get').mockReturnValue(true); document.dispatchEvent(new Event('visibilitychange')); }
    if (lifecycle === 'remove' || lifecycle === 'replace') { const patch = { worktrees: lifecycle === 'remove' ? [] : [{ ...f.state.worktrees[0], folder: '/replacement' }] }; f.actions.observe(patch); Object.assign(f.state, patch); }
    if (lifecycle === 'logout') { f.actions.observe({ session: { status: 'out' } }); f.state.session.status = 'out'; }
    await vi.advanceTimersByTimeAsync(0);
    expect(f.active.size).toBe(0);
    expect(pendingCall.signal.aborted).toBe(true);
    const snapshot = JSON.stringify(f.state.worktrees), count = f.calls.length;
    held.release(); await vi.advanceTimersByTimeAsync(lifecycle === 'replace' ? 0 : 10000);
    expect(JSON.stringify(f.state.worktrees)).toBe(snapshot);
    expect(f.calls).toHaveLength(count);
    if (lifecycle !== 'replace') expect(vi.getTimerCount()).toBe(0);
  } finally { held.release(); close(); vi.restoreAllMocks(); vi.useRealTimers(); }
});

it('unchanged snapshots and live heartbeats do not read, and focus/reconnect preserve backoff', async () => {
  vi.useFakeTimers(); let fail = true;
  const f = fixture(async tag => { if (tag === 'project.git.state' && fail) throw new Error('failed'); return emptyGitReply(tag); });
  f.state.conn = { status: 'live' };
  const close = f.actions.openScreen('git', 'project');
  try {
    await vi.advanceTimersByTimeAsync(0); expect(f.calls).toHaveLength(1);
    for (let n = 0; n < 10; n++) {
      f.actions.observe({ worktrees: f.state.worktrees, projects: f.state.projects, conn: { status: 'live' } });
      f.listeners.get('focus')!(); f.state.conn.status = 'reconnecting'; f.actions.observe({ conn: { status: 'live' } }); f.state.conn.status = 'live';
      await vi.advanceTimersByTimeAsync(90);
    }
    expect(f.calls).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(100); expect(f.calls).toHaveLength(2);
    fail = false; await f.actions.refreshGit('project');
    const count = f.calls.length;
    for (let n = 0; n < 10; n++) { f.actions.observe({ worktrees: f.state.worktrees, projects: f.state.projects, conn: { status: 'live' } }); await vi.advanceTimersByTimeAsync(100); }
    expect(f.calls).toHaveLength(count);
  } finally { close(); vi.useRealTimers(); }
});

it('a foreign dirty counter during owner readback survives while own completions cause no duplicate', async () => {
  vi.useFakeTimers(); const held = gate(); let hold = false;
  const f = fixture(async tag => { if (tag === 'project.stage.all') return {}; if (hold && tag === 'project.changes') await held.held; return emptyGitReply(tag); });
  const close = f.actions.openScreen('git', 'project');
  try {
    await vi.advanceTimersByTimeAsync(0); hold = true;
    const action = f.actions.stage('workspace', [], true, true);
    await vi.advanceTimersByTimeAsync(0);
    const commandIndex = f.calls.findIndex(c => c.tag === 'project.stage.all'), own = 'own-' + (commandIndex + 1);
    f.actions.observe({ git: { group: 'summary', worktreeIds: [], requestId: own } });
    f.actions.observe({ git: { group: 'summary', worktreeIds: ['workspace'], requestId: 'foreign' } });
    held.release(); await action; hold = false;
    const count = f.calls.filter(c => c.tag === 'project.git.state').length;
    await vi.advanceTimersByTimeAsync(100);
    expect(f.calls.filter(c => c.tag === 'project.git.state')).toHaveLength(count + 1);
    f.actions.observe({ git: { group: 'summary', worktreeIds: [], requestId: own, completed: true } });
    await vi.advanceTimersByTimeAsync(4000);
    expect(f.calls.filter(c => c.tag === 'project.git.state')).toHaveLength(count + 1);
  } finally { held.release(); close(); vi.useRealTimers(); }
});

it('a failed affected sibling stays due after a source-only selection recovers', async () => {
  vi.useFakeTimers(); let failing = false;
  const f = fixture(async (tag, payload) => { if (tag === 'project.integrate') return {}; if (tag === 'project.git.state' && payload.worktreeId === 'sibling' && failing) throw new Error('sibling'); return emptyGitReply(tag); });
  Object.assign(f.state.worktrees[0], { kind: 'team', parentId: 'parent' });
  f.state.worktrees.push({ ...f.state.worktrees[0], id: 'parent', kind: 'project' }, { ...f.state.worktrees[0], id: 'sibling' });
  const close = f.actions.openScreen('git', 'project');
  try {
    await vi.advanceTimersByTimeAsync(0); failing = true;
    expect(await f.actions.integrate('workspace')).toBe(false);
    await f.actions.selectGit('workspace'); failing = false;
    const count = f.calls.filter(c => c.tag === 'project.git.state' && c.payload.worktreeId === 'sibling').length;
    await vi.advanceTimersByTimeAsync(1100);
    expect(f.calls.filter(c => c.tag === 'project.git.state' && c.payload.worktreeId === 'sibling')).toHaveLength(count + 1);
    expect(f.state.worktrees.find((w: any) => w.id === 'sibling').stale).toBe(false);
  } finally { close(); vi.useRealTimers(); }
});
it("round 4: a real binary conflict keeps the usable text file and aborts through the built module", async () => {
  const { execFileSync } = await import("node:child_process");
  const { mkdtemp, writeFile, rm } = await import("node:fs/promises");
  const { tmpdir } = await import("node:os");
  const { join } = await import("node:path");
  const Effect = await import("effect/Effect");
  const Stream = await import("effect/Stream");
  const { makeGitService } = await import("../../engine/src/project/git/index.ts");
  const { engineConnection } = await import("../src/connection.ts");
  const { engineSnapshot } = await import("./fixtures.ts");
  const mappings = await import("../src/git.ts");
  const root = await mkdtemp(join(tmpdir(), "s13-binary-conflict-"));
  const service = makeGitService({ workspaceFolder: root });
  const run = Effect.runPromise;
  const git = (path: string, ...args: string[]) => execFileSync("/usr/bin/git", ["-C", path, ...args], { encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] }).trim();
  const commit = (path: string) => { git(path, "add", "--all"); git(path, "commit", "-m", "Fixture"); };
  let close: (() => void) | undefined, view: Awaited<ReturnType<typeof gitTabView>> | undefined, connection: ReturnType<typeof engineConnection> | undefined;
  const element = document.createElement("div"); document.body.append(element);
  const kit = await import("../../../design/mock/vendor/htm-preact/standalone.module.js" as string);
  try {
    const repo = (await run(service.createRepository("repository"))).path;
    git(repo, "config", "user.name", "Fixture"); git(repo, "config", "user.email", "fixture@example.invalid"); git(repo, "config", "commit.gpgsign", "false");
    await writeFile(join(repo, "text.txt"), "base\n"); await writeFile(join(repo, "binary.bin"), "\0base"); commit(repo);
    const ours = (await run(service.createWorktree(repo, "ours"))).path;
    const theirs = (await run(service.createWorktree(repo, "theirs"))).path;
    await writeFile(join(ours, "text.txt"), "ours\n"); await writeFile(join(ours, "binary.bin"), "\0ours"); commit(ours);
    await writeFile(join(theirs, "text.txt"), "theirs\n"); await writeFile(join(theirs, "binary.bin"), "\0theirs"); commit(theirs);
    const merged = await run(Effect.result(service.mergeTeam(theirs, ours, git(theirs, "rev-parse", "HEAD"))));
    expect(merged).toMatchObject({ _tag: "Failure", failure: { reason: "conflicts" } });
    const counts = new Map<string, number>(); let aborts = 0;
    const invoke = (tag: string, input: any) => {
      const key = tag + ":" + (input.path || ""); counts.set(key, (counts.get(key) || 0) + 1);
      let operation;
      switch (tag) {
        case "project.git.state": operation = service.state(ours); break;
        case "project.changes": operation = service.changes(ours); break;
        case "project.conflict.resolved": operation = service.resolvedConflicts(ours); break;
        case "project.conflict": operation = service.conflict(ours, input.path); break;
        case "project.merge.abort": aborts++; operation = service.abortMerge(ours, input.expectedHead, input.expectedMergeHead, input.confirmed); break;
        default: throw new Error(tag);
      }
      return operation.pipe(Effect.map(result => ({ requestId: input.requestId, result })),
        Effect.mapError(error => ({ _tag: "CommandError", reason: `Git: ${error.reason}`, requestId: input.requestId })));
    };
    const client = Object.fromEntries(["project.git.state", "project.changes", "project.conflict.resolved", "project.conflict", "project.merge.abort"].map(tag => [tag, (input: any) => invoke(tag, input)]));
    client["project.settings"] = () => Effect.succeed({}) as any;
    client["client.snapshot"] = () => Effect.succeed(engineSnapshot) as any;
    client["client.subscribe"] = () => Stream.never as any;
    const connected: any = {};
    connection = engineConnection({ location: { protocol: "http:", host: "127.0.0.1:1" }, debug() {}, fetch: async () => new Response(null, { status: 204 }), connect: (() => Effect.succeed(client)) as any });
    connection.open(patch => Object.assign(connected, patch));
    await vi.waitFor(() => expect(connected.conn?.status).toBe("live"));
    const f = fixture((tag, input) => connection!.request(tag, input));
    Object.assign(f.state.worktrees[0], { folder: ours, kind: "team", commits: [], parent: null });
    f.actions.installMappings(mappings);
    close = f.actions.openScreen("git", "project");
    await vi.waitFor(() => expect(f.actions.busy("git-read")).toBe(false));
    await vi.waitFor(() => expect(f.state.worktrees[0].merge.files.find((file: any) => file.path === 'text.txt').loaded).toBe(true));
    let w = f.state.worktrees[0];
    expect(w.gitError).toBeNull(); expect(w.ready).toBe(true);
    expect(w.merge.files.find((file: any) => file.path === "text.txt")).toMatchObject({ loaded: true, parts: [{ ours: ["ours\n"], theirs: ["theirs\n"] }] });
    for (let i = 1; i <= 3; i++) { await f.actions.refreshGit('project'); expect(counts.get('project.git.state:')).toBe(i + 1); }
    expect(counts.get('project.conflict:binary.bin')).toBe(4);
    await f.actions.selectGit('workspace', 'conflicts');
    expect(counts.get('project.conflict:binary.bin')).toBe(5);
    const source = copied("js/screens/git.js");
    const component = source.slice(source.indexOf("const CHOICES"), source.indexOf("// ---- integrate strip"));
    const Conflicts = new Function("html", "live", "update", "Icon", "cx", "baseName", "dirName", "plural", "abortMerge", "continueMerge", "markResolved", "choose", `${component}\nreturn Conflicts;`)(
      kit.html, { on: true }, () => {}, () => null, (...parts: unknown[]) => parts.filter(Boolean).join(" "), (p: string) => p.split("/").at(-1), () => "", (n: number, noun: string) => `${n} ${noun}`, (_: unknown, selected: any) => f.actions.abortMerge(selected.id), () => {}, () => {}, () => {});
    w = f.state.worktrees[0];
    view = await gitTabView(f, { Conflicts }); view.draw();
    const panel = view.element;
    expect(panel.textContent).toContain("binary.bin");
    expect(panel.querySelector<HTMLFieldSetElement>("fieldset")!.disabled).toBe(false);
    const buttons = [...panel.querySelectorAll<HTMLButtonElement>("button")];
    expect(buttons.find(b => b.textContent === "Continue merge")?.disabled).toBe(true);
    const abort = buttons.find(b => b.textContent === "Abort")!;
    expect(abort.disabled).toBe(false); abort.click();
    expect(f.dialogs).toHaveLength(1); expect(aborts).toBe(0);
    await f.dialogs[0].onConfirm();
    expect(aborts).toBe(1); expect((await run(service.state(ours))).mergeHead).toBeNull();
    expect(f.state.worktrees[0].merge).toBeNull();
    expect(f.toasts).toHaveLength(1);
  } finally { close?.(); view?.close(); connection?.close(); kit.render(null, element); element.remove(); await rm(root, { recursive: true, force: true }); }
});


it('two hundred paths on both sides publish progressively without a duplicate restart when events arrive', async () => {
  vi.useFakeTimers(); const held = gate(); const bodies: string[] = [];
  const files = Array.from({ length: 200 }, (_, i) => ({ id: `${i}`, path: `file-${i >> 1}`, staged: !!(i % 2) }));
  const f = fixture(async (tag, payload) => {
    if (tag === 'project.changes') return { result: { items: files } };
    if (tag === 'project.diff') { const key = `${payload.path}:${payload.staged}`; bodies.push(key); if (bodies.length === 2) await held.held; if (payload.path === 'file-7') throw new Error('one file'); return { result: { hunks: [key] } }; }
    return emptyGitReply(tag);
  });
  f.state.gitUi.workspace.tab = 'changes';
  const close = f.actions.openScreen('git', 'project');
  try {
    await vi.advanceTimersByTimeAsync(0);
    expect(f.state.worktrees[0].files[0]).toMatchObject({ loaded: true, hunks: ['file-0:false'] });
    expect(f.actions.busy('git-read')).toBe(false);
    expect(bodies).toHaveLength(2);
    for (let n = 0; n < 3; n++) { f.actions.observe({ git: { group: 'review', worktreeIds: ['workspace'] } }); await vi.advanceTimersByTimeAsync(10000); }
    expect(bodies).toHaveLength(2);
    expect(f.state.worktrees[0].files[0].hunks).toEqual(['file-0:false']);
    held.release(); await vi.advanceTimersByTimeAsync(0);
    expect(bodies).toHaveLength(200); expect(new Set(bodies).size).toBe(200);
    expect(f.state.worktrees[0].files.every((file: any) => file.loaded)).toBe(true);
    expect(f.state.worktrees[0].ready).toBe(true);
    expect(f.state.worktrees[0].files[14].diffError).toBeTruthy();
  } finally { held.release(); close(); vi.useRealTimers(); }
});

for (const failure of ['project.review.status', 'project.history', 'project.branches', 'project.conflict.resolved', 'project.diff']) it(`${failure} cannot erase the summary or the worktree panels`, async () => {
  vi.useFakeTimers(); let broken = false;
  const f = fixture(async tag => {
    if (broken && tag === failure) throw new Error('detail only');
    if (tag === 'project.git.state') return { result: { head: 'head', mergeHead: failure === 'project.conflict.resolved' ? 'merge' : null } };
    if (tag === 'project.changes') return { result: { items: [{ id: 'file', path: 'file', staged: false }] } };
    if (tag === 'project.history') return { result: { items: [{ hash: 'retained', subject: 'Retained', files: [], loaded: true }] } };
    if (tag === 'project.diff') return { result: { hunks: ['retained'] } };
    return emptyGitReply(tag);
  });
  f.state.gitUi.workspace.tab = failure.includes('history') || failure.includes('branches') ? 'history' : 'changes';
  const close = f.actions.openScreen('git', 'project');
  try {
    await vi.advanceTimersByTimeAsync(0); broken = true;
    await f.actions.refreshGit('project');
    expect(f.state.worktrees[0].ready).toBe(true);
    if (failure === 'project.diff') expect(f.state.worktrees[0].files[0].hunks).toEqual(['retained']);
    if (failure.includes('history') || failure.includes('branches')) expect(f.state.worktrees[0].commits[0].hash).toBe('retained');
    const view = await gitTabView(f); view.draw(); expect(view.element.querySelector('fieldset')?.disabled).toBe(false); view.close();
  } finally { close(); vi.useRealTimers(); }
});

it('Refresh Git retries repaired summary and detail refusals without a head change', async () => {
  vi.useFakeTimers(); let phase = 'state';
  const f = fixture(async tag => {
    if (tag === 'project.git.state' && phase === 'state' || tag === 'project.changes' && phase === 'changes' || tag === 'project.diff' && phase === 'diff') throw new Error('refused');
    if (tag === 'project.changes') return { result: { items: [{ id: 'file', path: 'file', staged: false }] } };
    if (tag === 'project.diff') return { result: { hunks: ['repaired'] } };
    return emptyGitReply(tag);
  });
  f.state.gitUi.workspace.tab = 'changes';
  const close = f.actions.openScreen('git', 'project');
  try {
    await vi.advanceTimersByTimeAsync(0); expect(f.state.worktrees[0].stale).toBe(true);
    phase = 'changes'; expect(await f.actions.refreshGit('project')).toBe(false);
    phase = 'diff'; await f.actions.refreshGit('project'); expect(f.state.worktrees[0].files[0].diffError).toBeTruthy();
    phase = 'repaired'; await f.actions.refreshGit('project');
    expect(f.state.worktrees[0]).toMatchObject({ ready: true, stale: false, files: [{ loaded: true, hunks: ['repaired'] }] });
  } finally { close(); vi.useRealTimers(); }
});

it('the sixty-second fallback sees a terminal commit and then a same-status edit despite a failing sibling', async () => {
  const { execFileSync } = await import('node:child_process');
  const { mkdtempSync, writeFileSync, readFileSync, rmSync } = await import('node:fs');
  const { tmpdir } = await import('node:os'); const { join } = await import('node:path');
  const root = mkdtempSync(join(tmpdir(), 's16-fallback-'));
  const git = (...args: string[]) => execFileSync('/usr/bin/git', ['-C', root, ...args], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim();
  git('init', '-b', 'main'); git('config', 'user.name', 'Fixture'); git('config', 'user.email', 'fixture@example.invalid'); git('config', 'commit.gpgsign', 'false');
  writeFileSync(join(root, 'file'), 'base\n'); git('add', '.'); git('commit', '-m', 'Base');
  writeFileSync(join(root, 'file'), 'first\n');
  vi.useFakeTimers(); vi.setSystemTime(100000);
  const f = fixture(async (tag, payload) => {
    if (tag === 'project.git.state') { if (payload.worktreeId === 'bad') throw new Error('sibling'); return { result: { head: git('rev-parse', 'HEAD'), mergeHead: null } }; }
    if (tag === 'project.changes') return { result: { items: git('diff', '--name-only') ? [{ id: 'file', path: 'file', staged: false, status: 'M' }] : [] } };
    if (tag === 'project.diff') return { result: { hunks: [readFileSync(join(root, 'file'), 'utf8')] } };
    return emptyGitReply(tag);
  });
  f.state.gitUi.workspace.tab = 'changes';
  f.state.worktrees.push({ ...f.state.worktrees[0], id: 'bad', kind: 'team', branch: 'bad' });
  const close = f.actions.openScreen('git', 'project');
  try {
    await vi.advanceTimersByTimeAsync(0); const before = f.state.worktrees[0].head;
    git('add', '.'); git('commit', '-m', 'Terminal commit'); writeFileSync(join(root, 'file'), 'second\n');
    await vi.advanceTimersByTimeAsync(63000);
    expect(f.state.worktrees[0].head).not.toBe(before);
    expect(f.state.worktrees[0].files[0].hunks).toEqual(['second\n']);
    writeFileSync(join(root, 'file'), 'same status, new bytes\n');
    await vi.advanceTimersByTimeAsync(63000);
    expect(f.state.worktrees[0].files[0].hunks).toEqual(['same status, new bytes\n']);
    expect(f.state.worktrees[1].stale).toBe(true);
  } finally { close(); vi.useRealTimers(); rmSync(root, { recursive: true, force: true }); }
});

for (const action of ['stage', 'commit', 'resolve', 'refresh']) it(`a held automatic state cannot block an accepted ${action}`, async () => {
  vi.useFakeTimers(); const held = gate(); let hold = false; const sent: string[] = [];
  const f = fixture(async tag => {
    if (['project.stage.all', 'project.commit', 'project.conflict.resolve'].includes(tag)) { sent.push(tag); return {}; }
    if (tag === 'project.git.state') { if (hold) await held.held; return { result: { head: 'head', mergeHead: 'merge' } }; }
    if (tag === 'project.changes') return { result: { items: [{ id: 'file', path: 'file', conflict: true }] } };
    if (tag === 'project.conflict') return { result: { path: 'file', parts: [], head: 'head', fingerprint: 'fingerprint' } };
    return emptyGitReply(tag);
  });
  const close = f.actions.openScreen('git', 'project');
  try {
    await vi.advanceTimersByTimeAsync(0); hold = true;
    f.listeners.get('focus')!(); await vi.advanceTimersByTimeAsync(100);
    expect(f.actions.busy('git-read')).toBe(true); expect(f.actions.busy('git')).toBe(false);
    const before = f.calls.length;
    const done = action === 'stage' ? f.actions.stage('workspace', [], true, true) : action === 'commit' ? f.actions.commit('workspace', 'Commit') : action === 'resolve' ? f.actions.resolve('workspace', 'file', true) : f.actions.refreshGit('project');
    expect(f.calls.length).toBeGreaterThan(before);
    if (action !== 'refresh') expect(sent).toHaveLength(1);
    hold = false; held.release(); await done;
    expect(f.actions.busy('git')).toBe(false); expect(f.state.worktrees[0].stale).toBe(false);
  } finally { held.release(); close(); vi.useRealTimers(); }
});

it('hiding an in-flight mutation cancels its observation and revalidates on return without replay', async () => {
  vi.useFakeTimers(); const held = gate(); let stages = 0;
  const f = fixture(async tag => { if (tag === 'project.stage.all') { stages++; await held.held; return {}; } return emptyGitReply(tag); });
  const close = f.actions.openScreen('git', 'project');
  try {
    await vi.advanceTimersByTimeAsync(0);
    const action = f.actions.stage('workspace', [], true, true);
    expect(stages).toBe(1);
    const hidden = vi.spyOn(document, 'hidden', 'get').mockReturnValue(true);
    document.dispatchEvent(new Event('visibilitychange')); await vi.advanceTimersByTimeAsync(0);
    expect(await action).toBe(false); expect(f.active.size).toBe(0); expect(vi.getTimerCount()).toBe(0);
    const before = f.calls.filter(c => c.tag === 'project.git.state').length;
    hidden.mockReturnValue(false); document.dispatchEvent(new Event('visibilitychange'));
    await vi.advanceTimersByTimeAsync(100);
    expect(f.calls.filter(c => c.tag === 'project.git.state')).toHaveLength(before + 1);
    expect(stages).toBe(1); expect(f.toasts).toEqual([]);
  } finally { held.release(); close(); vi.restoreAllMocks(); vi.useRealTimers(); }
});

it('a later admitted identity gets its first load without a waiting agent collection', async () => {
  vi.useFakeTimers(); const f = fixture(async tag => emptyGitReply(tag));
  const close = f.actions.openScreen('git', 'project');
  try {
    await vi.advanceTimersByTimeAsync(0);
    const patch = { worktrees: [...f.state.worktrees, { ...f.state.worktrees[0], id: 'placed', folder: '/placed', branch: 'placed', kind: 'agent', ready: false }] };
    f.actions.observe(patch); Object.assign(f.state, patch); await vi.advanceTimersByTimeAsync(0);
    expect(f.calls.filter(c => c.tag === 'project.git.state').map(c => c.payload.worktreeId)).toEqual(['workspace', 'placed']);
    expect(f.state.worktrees[1].ready).toBe(true);
  } finally { close(); vi.useRealTimers(); }
});

it('a newer state head publishes even if its change list fails, and invalidates old mutable bytes', async () => {
  vi.useFakeTimers(); let broken = false;
  const f = fixture(async tag => {
    if (tag === 'project.git.state') return { result: { head: broken ? 'new' : 'old', mergeHead: null } };
    if (tag === 'project.changes') { if (broken) throw new Error('changes'); return { result: { items: [{ id: 'file', path: 'file' }] } }; }
    if (tag === 'project.diff') return { result: { hunks: ['old bytes'] } };
    return emptyGitReply(tag);
  });
  f.state.gitUi.workspace.tab = 'changes'; const close = f.actions.openScreen('git', 'project');
  try {
    await vi.advanceTimersByTimeAsync(0); broken = true;
    await f.actions.refreshGit('project');
    expect(f.state.worktrees[0]).toMatchObject({ head: 'new', stale: true, files: [{ loaded: false, hunks: [] }] });
  } finally { close(); vi.useRealTimers(); }
});

it('an explicit history demand retries a failed body while retaining completed immutable bodies', async () => {
  vi.useFakeTimers(); let broken = true; const paths: string[] = [];
  const f = fixture(async (tag, payload) => {
    if (tag === 'project.history') return { result: { items: [{ hash: 'commit' }] } };
    if (tag === 'project.commit.files') return { result: { items: [{ id: 'one', path: 'one' }, { id: 'two', path: 'two' }] } };
    if (tag === 'project.diff') { paths.push(payload.path); if (payload.path === 'two' && broken) throw new Error('failed body'); return { result: { hunks: [payload.path] } }; }
    return emptyGitReply(tag);
  });
  f.state.gitUi.workspace.tab = 'history'; const close = f.actions.openScreen('git', 'project');
  try {
    await vi.advanceTimersByTimeAsync(0); expect(paths).toEqual(['one', 'two']);
    expect(f.state.worktrees[0].commits[0].files[0].loaded).toBe(true);
    broken = false; await f.actions.refreshGit('project');
    expect(paths).toEqual(['one', 'two', 'two']);
    expect(f.state.worktrees[0].commits[0].files.every((f: any) => f.loaded)).toBe(true);
  } finally { close(); vi.useRealTimers(); }
});

it('preparing a review and choosing a preview publish direct replies without a duplicate automatic pass', async () => {
  vi.useFakeTimers();
  const f = fixture(async tag => {
    if (tag === 'project.review.prepare') return { result: { review: { id: 'prepared' }, comparison: {} } };
    if (tag === 'project.conflict.choose' || tag === 'project.conflict') return { result: { path: 'file', parts: [{ id: 'hunk', hunk: 0 }], head: 'head', fingerprint: 'fingerprint' } };
    if (tag === 'project.git.state') return { result: { head: 'head', mergeHead: 'merge' } };
    if (tag === 'project.changes') return { result: { items: [{ id: 'file', path: 'file', conflict: true }] } };
    return emptyGitReply(tag);
  });
  f.state.worktrees[0].parentId = 'base'; f.state.worktrees.push({ ...f.state.worktrees[0], id: 'base', kind: 'base', branch: 'main', project: null });
  const close = f.actions.openScreen('git', 'project');
  try {
    await vi.advanceTimersByTimeAsync(0); const before = f.calls.length;
    await f.actions.openReview(f.state.worktrees[0], f.state.projects[0]);
    const prepare = f.calls.findIndex(c => c.tag === 'project.review.prepare');
    f.actions.observe({ git: { group: 'review', worktreeIds: ['workspace'], requestId: 'own-' + (prepare + 1), completed: true } });
    await f.actions.choose('workspace', 'file', 'hunk', 'ours');
    const choose = f.calls.findIndex(c => c.tag === 'project.conflict.choose');
    f.actions.observe({ git: { group: 'summary', worktreeIds: [], requestId: 'own-' + (choose + 1), completed: true } });
    await vi.advanceTimersByTimeAsync(10000);
    expect(f.calls.slice(before).map(c => c.tag)).toEqual(['project.review.prepare', 'project.conflict.choose']);
  } finally { close(); vi.useRealTimers(); }
});
