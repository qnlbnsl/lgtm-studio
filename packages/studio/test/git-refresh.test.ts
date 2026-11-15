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
  const dialogs: any[] = [], toasts: string[] = [], listeners = new Map<string, () => void>();
  const act: any = { confirm: (d: any) => dialogs.push(d), closeDialog: vi.fn(), toast: (s: string) => toasts.push(s) };
  const source = copied("js/live/git.js").replace(/^import .*;$/gm, "").replace(/export /g, "");
  const actions = new Function("store", "live", "engineWrite", "notify", "engine", "act", "update", "addEventListener", "removeEventListener",
    `${source}\nreturn { installMappings, openScreen, refreshGit, observe, busy, declineReview, discard, stage, integrate, selectGit, commit, abortMerge, resolve };`)(
    { state }, { on: true }, (p: object) => Object.assign(state, p), () => {}, { request }, act, (f: (s: any) => void) => f(state), (name: string, fn: () => void) => listeners.set(name, fn), (name: string) => listeners.delete(name));
  actions.installMappings({ mapGitState: (s: any) => s, mapGitChanges: (items: any[]) => items,
    mapGitCommit: (c: any) => c, mapCommitFiles: (items: any[]) => items, mapGitDiff: (r: any) => r,
    mapMainReview: (r: any) => r, mapConflict: (c: any) => ({ ...c, id: c.path, selection: c }) });
  return { state, actions, act, dialogs, toasts, listeners };
}

async function gitTabView(f: ReturnType<typeof fixture>, panels: Record<string, any> = {}) {
  const kit = await import("../../../design/mock/vendor/htm-preact/standalone.module.js" as string);
  const source = copied("js/screens/git.js");
  const component = source.slice(source.indexOf("export function GitTab(")).replace("export ", "");
  const GitTab = new Function("html", "useEffect", "useMedia", "useState", "persist", "remember", "live", "liveGit", "agentOf", "machineOf", "Menu", "Icon", "AheadBehind", "Integrate", "Changes", "History", "Conflicts", "KIND_WORD", "ownerText", `${component}\nreturn GitTab;`)(
    kit.html, () => {}, () => false, () => ["unified", () => {}], () => "unified", () => {}, { on: true }, f.actions, () => null, () => ({ name: "local" }),
    () => null, () => null, () => null, () => null, panels.Changes || (() => null), panels.History || (() => null), panels.Conflicts || (() => null), {}, () => "Project lead");
  const element = document.createElement("div"); document.body.append(element);
  return { element, draw: () => kit.render(kit.h(GitTab, { s: f.state, project: f.state.projects[0] }), element), close: () => { kit.render(null, element); element.remove(); } };
}

it("an explicit refresh survives a held older background read and stays busy until the conflict details arrive", async () => {
  const older = gate(), detail = gate(); let stateReads = 0, conflictReads = 0;
  const f = fixture(async (tag) => {
    if (tag === "project.git.state") { if (++stateReads === 1) { await older.held; return { result: { mergeHead: null } }; } return { result: { mergeHead: "merge" } }; }
    if (tag === "project.changes") return { result: { items: [{ path: "file.txt", conflict: true }], truncated: false } };
    if (tag === "project.conflict.resolved") return { result: { items: [], truncated: false } };
    if (tag === "project.review.status") return { result: { review: null } };
    if (tag === "project.conflict") { conflictReads++; await detail.held; return { result: { path: "file.txt", parts: [{ id: "hunk", ours: ["ours"], theirs: ["theirs"] }], fingerprint: "new" } }; }
    throw new Error(tag);
  });
  const close = f.actions.openScreen("git", "project");
  let refresh: Promise<unknown> | undefined;
  try {
    await vi.waitFor(() => expect(stateReads).toBe(1));
    refresh = f.actions.refreshGit("project");
    expect(f.actions.busy("git")).toBe(true);
    await vi.waitFor(() => expect(conflictReads).toBe(1));
    expect(f.state.worktrees[0].merge.files[0].loaded).toBe(false);
    expect(f.actions.busy("git")).toBe(true);
    detail.release(); await refresh;
    expect(f.state.worktrees[0].merge.files[0]).toMatchObject({ loaded: true, selection: { fingerprint: "new" } });
    expect(f.actions.busy("git")).toBe(false);
    older.release(); await vi.waitFor(() => expect(f.actions.busy("git-read")).toBe(false));
    expect(f.state.worktrees[0].merge.files[0].selection.fingerprint).toBe("new");
    expect(f.toasts).toEqual([]);
  } finally { older.release(); detail.release(); await refresh; close(); }
});

it("Send back keeps its dialog open until its complete readback and pending state have finished", async () => {
  const readback = gate(); let reads = 0;
  const f = fixture(async (tag) => {
    if (tag === "project.review.decline") return { result: { id: "review", outcome: "sent_back" } };
    if (tag === "project.git.state") { reads++; await readback.held; return { result: { mergeHead: null } }; }
    if (tag === "project.changes") return { result: { items: [], truncated: false } };
    if (tag === "project.review.status") return { result: { review: null } };
    throw new Error(tag);
  });
  const decision = f.actions.declineReview("review", "Please check it");
  try {
    await vi.waitFor(() => expect(reads).toBe(1));
    expect(f.actions.busy("git")).toBe(true);
    expect(f.act.closeDialog).not.toHaveBeenCalled();
    f.act.closeDialog.mockImplementation(() => expect(f.actions.busy("git")).toBe(false));
    readback.release(); await decision;
    expect(f.act.closeDialog).toHaveBeenCalledOnce();
    expect(f.toasts).toEqual([]);
  } finally { readback.release(); await decision; }
});

it("discard passes the existing confirmation a visible pending state through the complete readback", async () => {
  const readback = gate(); let reads = 0, dialog: any, hold = false;
  const f = fixture(async (tag) => {
    if (tag === "project.discard.plan") return { result: { paths: ["file.txt"] } };
    if (tag === "project.discard") return {};
    if (tag === "project.git.state") { if (hold) { reads++; await readback.held; } return { result: { mergeHead: null } }; }
    if (tag === "project.changes") return { result: { items: [], truncated: false } };
    if (tag === "project.review.status") return { result: { review: null } };
    throw new Error(tag);
  });
  const close = f.actions.openScreen("git", "project");
  await vi.waitFor(() => expect(f.actions.busy("git-read")).toBe(false));
  hold = true;
  // Exercise the copied action wrapper too; it must preserve the dialog options.
  const source = copied("js/actions.js");
  const start = source.indexOf("export function confirm("), end = source.indexOf("\n}", start) + 2;
  f.act.confirm = new Function("openDialog", `${source.slice(start, end).replace("export ", "")}\nreturn confirm;`)((d: unknown) => { dialog = d; });
  await f.actions.discard(f.state.worktrees[0], [{ path: "file.txt" }]);
  expect(dialog).toMatchObject({ waitForReply: true, pendingLabel: "Discarding…" });
  const kit = await import("../../../design/mock/vendor/htm-preact/standalone.module.js" as string);
  const ui = copied("js/ui.js"), a = ui.indexOf("function ConfirmDialog("), b = ui.indexOf("function RenameDialog(", a);
  const Confirm = new Function("html", "useState", "act", "Dialog", "cx", "Icon", `${ui.slice(a, b)}\nreturn ConfirmDialog;`)(kit.html, kit.useState, f.act, ({ foot }: any) => foot, () => "", () => null);
  const element = document.createElement("div"); document.body.append(element);
  try {
    kit.render(kit.h(Confirm, { d: dialog }), element);
    [...element.querySelectorAll("button")].find((b) => b.textContent === "Discard")!.click();
    await vi.waitFor(() => expect(reads).toBe(1));
    await vi.waitFor(() => expect(element.textContent).toContain("Discarding…"));
    expect(element.querySelector('[aria-busy="true"]')).not.toBeNull();
    expect(f.act.closeDialog).not.toHaveBeenCalled();
    readback.release();
    await vi.waitFor(() => expect(f.act.closeDialog).toHaveBeenCalledOnce());
    expect(f.actions.busy("git")).toBe(false);
  } finally { readback.release(); close(); kit.render(null, element); element.remove(); }
});

it("the conflict view shows loading while its selection is still being read", async () => {
  const source = copied("js/screens/git.js");
  const start = source.indexOf("function Conflicts("), end = source.indexOf("// ---- integrate strip", start);
  const kit = await import("../../../design/mock/vendor/htm-preact/standalone.module.js" as string);
  const Conflicts = new Function("html", "live", "Icon", "cx", "baseName", "dirName", `${source.slice(start, end)}\nreturn Conflicts;`)(kit.html, { on: true }, () => null, () => "", (s: string) => s, () => "");
  const element = document.createElement("div"); document.body.append(element);
  try {
    kit.render(kit.h(Conflicts, { s: { gitUi: {} }, w: { id: "workspace", branch: "project", merge: { from: "other", files: [{ id: "file.txt", path: "file.txt", parts: [], loaded: false }] } } }), element);
    expect(element.querySelector('[role="status"]')?.textContent).toBe("Loading conflicts…");
    expect([...element.querySelectorAll("button")].find((b) => b.textContent === "Mark resolved")?.disabled).toBe(true);
  } finally { kit.render(null, element); element.remove(); }
});


it("a stage readback finishes without rereading an unrelated workspace", async () => {
  const unrelated = gate(), reads: string[] = []; let finished = false, hold = false;
  const f = fixture(async (tag, payload) => {
    if (tag === "project.stage.all") return {};
    if (tag === "project.git.state") { reads.push(payload.worktreeId); if (hold && payload.worktreeId === "other") await unrelated.held; return { result: { mergeHead: null } }; }
    if (tag === "project.changes") return { result: { items: [], truncated: false } };
    if (tag === "project.review.status") return { result: { review: null } };
    throw new Error(tag);
  });
  f.state.worktrees.push({ ...f.state.worktrees[0], id: "other", kind: "team", branch: "other" });
  const close = f.actions.openScreen("git", "project");
  // Finish the initial all-workspace read before holding the unrelated checkout.
  await vi.waitFor(() => expect(f.actions.busy("git-read")).toBe(false));
  reads.length = 0; hold = true;
  const action = f.actions.stage("workspace", [], true, true).then(() => { finished = true; });
  try {
    await vi.waitFor(() => expect(finished).toBe(true));
    expect(reads).toEqual(["workspace"]);
  } finally { unrelated.release(); await action; close(); }
});

it("integration also refreshes sibling counts when its destination head advances", async () => {
  const reads: string[] = [];
  const f = fixture(async (tag, payload) => {
    if (tag === "project.integrate") return {};
    if (tag === "project.git.state") { reads.push(payload.worktreeId); return { result: { mergeHead: null } }; }
    if (tag === "project.changes") return { result: { items: [], truncated: false } };
    if (tag === "project.review.status") return { result: { review: null } };
    throw new Error(tag);
  });
  Object.assign(f.state.worktrees[0], { kind: "agent", parentId: "team" });
  f.state.worktrees.push({ ...f.state.worktrees[0], id: "team", branch: "team", kind: "team" }, { ...f.state.worktrees[0], id: "other", branch: "other" });
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.waitFor(() => expect(f.actions.busy("git-read")).toBe(false)); reads.length = 0;
    await f.actions.integrate("workspace");
    expect(reads).toEqual(["workspace", "team", "other"]);
    expect(f.toasts).toEqual([]);
  } finally { close(); }
});

it("picking another worktree refreshes its cached Git state before the owner can act", async () => {
  const reads: string[] = [];
  const f = fixture(async (tag, payload) => {
    if (tag === "project.git.state") { reads.push(payload.worktreeId); return { result: { head: "new-head", mergeHead: null } }; }
    if (tag === "project.changes") return { result: { items: [], truncated: false } };
    throw new Error(tag);
  });
  f.state.worktrees.push({ ...f.state.worktrees[0], id: "other", branch: "other", kind: "team", head: "old-head" });
  await f.actions.selectGit("other");
  expect(reads).toEqual(["other"]);
  expect(f.state.worktrees.find((w: any) => w.id === "other").head).toBe("new-head");
  expect(f.toasts).toEqual([]);
});


it("an explicit project refresh still reads every worktree", async () => {
  const reads: string[] = [];
  const f = fixture(async (tag, payload) => {
    if (tag === "project.git.state") { reads.push(payload.worktreeId); return { result: { mergeHead: null } }; }
    if (tag === "project.changes") return { result: { items: [], truncated: false } };
    if (tag === "project.review.status") return { result: { review: null } };
    throw new Error(tag);
  });
  f.state.worktrees.push({ ...f.state.worktrees[0], id: "other", kind: "team", branch: "other" });
  await f.actions.refreshGit("project");
  expect(reads).toEqual(["workspace", "other"]);
  expect(f.toasts).toEqual([]);
});


it("heartbeats and unrelated project snapshots do not repeat Git reads, while a new Git revision does", async () => {
  let reads = 0;
  const f = fixture(async (tag) => {
    if (tag === "project.git.state") { reads++; return { result: { mergeHead: null } }; }
    if (tag === "project.changes") return { result: { items: [], truncated: false } };
    if (tag === "project.review.status") return { result: { review: null } };
    throw new Error(tag);
  });
  f.state.conn = { status: "live" }; f.state.gitRevision = 0;
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.waitFor(() => expect(f.actions.busy("git-read")).toBe(false));
    reads = 0; vi.useFakeTimers();
    for (let i = 0; i < 3; i++) {
      f.actions.observe({ conn: { status: "live", lastSeen: i } });
      f.actions.observe({ projects: f.state.projects, worktrees: f.state.worktrees, gitRevision: 0 });
      await vi.advanceTimersByTimeAsync(200);
    }
    expect(reads).toBe(0);
    f.actions.observe({ gitRevision: 1 }); f.state.gitRevision = 1;
    await vi.advanceTimersByTimeAsync(100);
    expect(reads).toBe(1);
    expect(f.actions.busy("git-read")).toBe(false);
    f.state.conn = { status: "reconnecting" };
    f.actions.observe({ conn: { status: "live" } }); f.state.conn = { status: "live" };
    await vi.advanceTimersByTimeAsync(100);
    expect(reads).toBe(2);
  } finally { close(); vi.useRealTimers(); }
});

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


it("a scoped action readback cannot consume an invalidation for another worktree", async () => {
  const reads: string[] = [];
  const f = fixture(async (tag, payload) => {
    if (tag === "project.stage.all") return {};
    if (tag === "project.git.state") { reads.push(payload.worktreeId); return { result: { mergeHead: null } }; }
    if (tag === "project.changes") return { result: { items: [], truncated: false } };
    if (tag === "project.review.status") return { result: { review: null } };
    throw new Error(tag);
  });
  f.state.worktrees.push({ ...f.state.worktrees[0], id: "other", branch: "other", kind: "team" });
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.waitFor(() => expect(f.actions.busy("git-read")).toBe(false));
    reads.length = 0; vi.useFakeTimers();
    f.actions.observe({ gitRevision: 1 }); f.state.gitRevision = 1;
    await f.actions.stage("workspace", [], true, true);
    expect(reads).toEqual(["workspace"]);
    await vi.advanceTimersByTimeAsync(100);
    expect(reads).toEqual(["workspace", "workspace", "other"]);
  } finally { close(); vi.useRealTimers(); }
});


it("picking a worktree also reads its parent's head before enabling an update", async () => {
  const parent = gate(), reads: string[] = [];
  const f = fixture(async (tag, payload) => {
    if (tag === "project.git.state") {
      reads.push(payload.worktreeId);
      if (payload.worktreeId === "parent") await parent.held;
      return { result: { head: payload.worktreeId + "-fresh", mergeHead: null } };
    }
    if (tag === "project.changes") return { result: { items: [], truncated: false } };
    throw new Error(tag);
  });
  Object.assign(f.state.worktrees[0], { kind: "agent", parentId: "parent" });
  f.state.worktrees.push({ ...f.state.worktrees[0], id: "parent", parentId: undefined, branch: "parent", kind: "team", head: undefined });
  const selection = f.actions.selectGit("workspace");
  try {
    await vi.waitFor(() => expect(reads).toContain("parent"));
    expect(f.actions.busy("git")).toBe(true);
    parent.release(); await selection;
    expect(f.state.worktrees.find((w: any) => w.id === "parent").head).toBe("parent-fresh");
    expect(reads).toEqual(["workspace", "parent"]);
    expect(f.toasts).toEqual([]);
  } finally { parent.release(); await selection; }
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


const emptyGitReply = (tag: string) => {
  if (tag === "project.git.state") return { result: { mergeHead: null } };
  if (["project.changes", "project.history", "project.branches", "project.commit.files"].includes(tag)) return { result: { items: [], truncated: false } };
  if (tag === "project.review.status") return { result: { review: null } };
  throw new Error(tag);
};

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

it("uses a visible heartbeat only after sixty seconds without a full read", async () => {
  vi.useFakeTimers(); vi.setSystemTime(100_000);
  let reads = 0;
  const f = fixture(async (tag) => { if (tag === "project.git.state") reads++; return emptyGitReply(tag); });
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.advanceTimersByTimeAsync(0); expect(reads).toBe(1);
    vi.setSystemTime(159_999); f.actions.observe({ gitHeartbeat: 159_999 });
    await vi.advanceTimersByTimeAsync(0); expect(reads).toBe(1);
    vi.setSystemTime(160_000); f.actions.observe({ gitHeartbeat: 160_000 });
    await vi.advanceTimersByTimeAsync(100); expect(reads).toBe(2);
    f.actions.observe({ gitHeartbeat: 160_100 });
    await vi.advanceTimersByTimeAsync(1000); expect(reads).toBe(2);
  } finally { close(); vi.useRealTimers(); }
});

it("refreshes for an agent in a shown worktree and ignores agents elsewhere", async () => {
  vi.useFakeTimers();
  let reads = 0;
  const f = fixture(async (tag) => { if (tag === "project.git.state") reads++; return emptyGitReply(tag); });
  f.state.worktrees[0].folder = "/shown";
  f.state.agents = [{ id: "worker", folder: "/shown" }, { id: "elsewhere", folder: "/elsewhere" }];
  const close = f.actions.openScreen("git", "project");
  const activity = async (revision: number, agentId: string) => {
    f.actions.observe({ gitRevision: revision, gitChange: { agentId } }); f.state.gitRevision = revision;
    await vi.advanceTimersByTimeAsync(100);
  };
  try {
    await vi.advanceTimersByTimeAsync(0); expect(reads).toBe(1);
    await activity(1, "elsewhere"); expect(reads).toBe(1);
    await activity(2, "worker"); expect(reads).toBe(2);
  } finally { close(); vi.useRealTimers(); }
});

it("defers all automatic Git refreshes while hidden and catches up when visible", async () => {
  vi.useFakeTimers(); vi.setSystemTime(100_000);
  const visibility = vi.spyOn(document, "visibilityState", "get").mockReturnValue("visible");
  let reads = 0;
  const f = fixture(async (tag) => { if (tag === "project.git.state") reads++; return emptyGitReply(tag); });
  f.state.worktrees[0].folder = "/shown"; f.state.agents = [{ id: "worker", folder: "/shown" }];
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.advanceTimersByTimeAsync(0); expect(reads).toBe(1);
    visibility.mockReturnValue("hidden"); vi.setSystemTime(170_000);
    f.actions.observe({ gitHeartbeat: 170_000 });
    f.actions.observe({ gitRevision: 1, gitChange: { agentId: "worker" } }); f.state.gitRevision = 1;
    f.actions.observe({ gitRevision: 2, gitChange: { requestId: "another-browser" } }); f.state.gitRevision = 2;
    await vi.advanceTimersByTimeAsync(5000); expect(reads).toBe(1);
    visibility.mockReturnValue("visible"); document.dispatchEvent(new Event("visibilitychange"));
    await vi.advanceTimersByTimeAsync(100); expect(reads).toBe(2);
  } finally { close(); visibility.mockRestore(); vi.useRealTimers(); }
});

for (const action of ["stage", "commit"]) for (const delivery of ["before-reply", "during-readback", "after-readback"]) {
  it(`${action} covers its own completion ${delivery} without a second read`, async () => {
    vi.useFakeTimers();
    const reads: string[] = []; let mutation = false, notified = false;
    const changed = () => { notified = true; f.actions.observe({ gitRevision: 1, gitChange: { requestId: "mine" } }); f.state.gitRevision = 1; };
    const f = fixture(async (tag, payload) => {
      if (["project.stage.all", "project.commit"].includes(tag)) { mutation = true; if (delivery === "before-reply") changed(); return { requestId: "mine" }; }
      if (tag === "project.git.state") { reads.push(payload.worktreeId); if (mutation && !notified && delivery === "during-readback") changed(); }
      return emptyGitReply(tag);
    });
    f.state.worktrees.push({ ...f.state.worktrees[0], id: "other", branch: "other", kind: "team" });
    const close = f.actions.openScreen("git", "project");
    try {
      await vi.advanceTimersByTimeAsync(0); reads.length = 0;
      if (action === "stage") await f.actions.stage("workspace", [], true, true); else await f.actions.commit("workspace", "A commit");
      if (delivery === "after-readback") changed();
      await vi.advanceTimersByTimeAsync(1000);
      expect(reads).toEqual(action === "stage" ? ["workspace"] : ["workspace", "other"]);
      expect(f.toasts).toEqual([]);
    } finally { close(); vi.useRealTimers(); }
  });
}

for (const action of ["stage", "commit"]) it(`${action} preserves another caller's completion during its readback`, async () => {
  vi.useFakeTimers();
  const reads: string[] = []; let mutation = false, notified = false;
  const f = fixture(async (tag, payload) => {
    if (["project.stage.all", "project.commit"].includes(tag)) { mutation = true; return { requestId: "mine" }; }
    if (tag === "project.git.state") {
      reads.push(payload.worktreeId);
      if (mutation && !notified) {
        notified = true;
        f.actions.observe({ gitRevision: 1, gitChange: { requestId: "mine" } }); f.state.gitRevision = 1;
        f.actions.observe({ gitRevision: 2, gitChange: { requestId: "another-browser" } }); f.state.gitRevision = 2;
      }
    }
    return emptyGitReply(tag);
  });
  f.state.worktrees.push({ ...f.state.worktrees[0], id: "other", branch: "other", kind: "team" });
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.advanceTimersByTimeAsync(0); reads.length = 0;
    if (action === "stage") await f.actions.stage("workspace", [], true, true); else await f.actions.commit("workspace", "A commit");
    await vi.advanceTimersByTimeAsync(1000);
    expect(reads).toEqual([...(action === "stage" ? ["workspace"] : ["workspace", "other"]), "workspace", "other"]);
    expect(f.toasts).toEqual([]);
  } finally { close(); vi.useRealTimers(); }
});


it("stops a pending read before its next request when the tab is hidden", async () => {
  vi.useFakeTimers();
  const visibility = vi.spyOn(document, "visibilityState", "get").mockReturnValue("visible"), held = gate();
  let states = 0, changes = 0;
  const f = fixture(async (tag) => {
    if (tag === "project.git.state" && ++states === 1) await held.held;
    if (tag === "project.changes") changes++;
    return emptyGitReply(tag);
  });
  const close = f.actions.openScreen("git", "project");
  try {
    visibility.mockReturnValue("hidden"); document.dispatchEvent(new Event("visibilitychange"));
    held.release(); await vi.advanceTimersByTimeAsync(1000);
    expect(states).toBe(1); expect(changes).toBe(0);
    visibility.mockReturnValue("visible"); document.dispatchEvent(new Event("visibilitychange"));
    await vi.advanceTimersByTimeAsync(100);
    expect(states).toBe(2); expect(changes).toBe(1);
  } finally { held.release(); close(); visibility.mockRestore(); vi.useRealTimers(); }
});

it("retries an own completion whose action readback failed", async () => {
  vi.useFakeTimers();
  let fail = false, reads = 0;
  const f = fixture(async (tag) => {
    if (tag === "project.stage.all") return { requestId: "mine" };
    if (tag === "project.git.state") {
      reads++;
      if (fail) {
        fail = false;
        f.actions.observe({ gitRevision: 1, gitChange: { requestId: "mine" } }); f.state.gitRevision = 1;
        throw new Error("Readback failed");
      }
    }
    return emptyGitReply(tag);
  });
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.advanceTimersByTimeAsync(0); reads = 0; fail = true;
    expect(await f.actions.stage("workspace", [], true, true)).toBe(false);
    await vi.advanceTimersByTimeAsync(1000);
    expect(reads).toBe(2);
    expect(f.toasts).toEqual(["Readback failed"]);
    f.actions.observe({ gitRevision: 2, gitChange: { requestId: "mine" } }); f.state.gitRevision = 2;
    await vi.advanceTimersByTimeAsync(1000); expect(reads).toBe(2);
  } finally { close(); vi.useRealTimers(); }
});

it("defers an action readback whose mutation reply arrives while hidden", async () => {
  vi.useFakeTimers();
  const visibility = vi.spyOn(document, "visibilityState", "get").mockReturnValue("visible"), mutation = gate();
  let reads = 0;
  const f = fixture(async (tag) => {
    if (tag === "project.stage.all") { await mutation.held; return { requestId: "mine" }; }
    if (tag === "project.git.state") reads++;
    return emptyGitReply(tag);
  });
  const close = f.actions.openScreen("git", "project");
  let action: Promise<unknown> | undefined;
  try {
    await vi.advanceTimersByTimeAsync(0); reads = 0;
    action = f.actions.stage("workspace", [], true, true);
    visibility.mockReturnValue("hidden"); document.dispatchEvent(new Event("visibilitychange"));
    mutation.release(); await action;
    expect(reads).toBe(0);
    visibility.mockReturnValue("visible"); document.dispatchEvent(new Event("visibilitychange"));
    await vi.advanceTimersByTimeAsync(100); expect(reads).toBe(1);
  } finally { mutation.release(); await action; close(); visibility.mockRestore(); vi.useRealTimers(); }
});

it("backs off persistent read failures to six attempts in sixty seconds and keeps healthy worktrees loaded", async () => {
  vi.useFakeTimers();
  const attempts: number[] = [], healthy: string[] = [];
  const f = fixture(async (tag, payload) => {
    if (tag === "project.git.state") {
      if (payload.worktreeId === "workspace") { attempts.push(Date.now()); throw new Error("checkout belongs to a different repository"); }
      healthy.push(payload.worktreeId);
    }
    return emptyGitReply(tag);
  });
  f.state.worktrees.push({ ...f.state.worktrees[0], id: "healthy", branch: "healthy", kind: "team" });
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.advanceTimersByTimeAsync(60_000);
    expect(attempts.map(t => t - attempts[0]!)).toEqual([0, 1000, 3000, 7000, 15000, 31000]);
    expect(f.toasts).toEqual(["checkout belongs to a different repository"]);
    expect(f.state.worktrees[0]).toMatchObject({ ready: false, gitError: "checkout belongs to a different repository" });
    expect(f.state.worktrees[1].ready).toBe(true);
    expect(healthy).toEqual(["healthy"]);
  } finally { close(); vi.useRealTimers(); }
});

it("caps retries at sixty seconds and resets the delay after recovery while retaining the toast clock", async () => {
  vi.useFakeTimers();
  let fail = true, reads = 0;
  const f = fixture(async tag => {
    if (tag === "project.git.state") { reads++; if (fail) throw new Error("missing checkout"); }
    return emptyGitReply(tag);
  });
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.advanceTimersByTimeAsync(123_000); expect(reads).toBe(8);
    await vi.advanceTimersByTimeAsync(59_999); expect(reads).toBe(8);
    fail = false; await vi.advanceTimersByTimeAsync(1); expect(reads).toBe(9);
    expect(f.state.worktrees[0].gitError).toBeNull();
    fail = true;
    f.actions.observe({ gitRevision: 1 }); f.state.gitRevision = 1;
    await vi.advanceTimersByTimeAsync(100); expect(reads).toBe(10);
    await vi.advanceTimersByTimeAsync(999); expect(reads).toBe(10);
    await vi.advanceTimersByTimeAsync(1); expect(reads).toBe(11);
    expect(f.toasts).toEqual(Array(4).fill("missing checkout"));
  } finally { close(); vi.useRealTimers(); }
});

it("an explicit refresh bypasses and resets read backoff", async () => {
  vi.useFakeTimers();
  let reads = 0;
  const f = fixture(async tag => { if (tag === "project.git.state") { reads++; throw new Error("missing checkout"); } return emptyGitReply(tag); });
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.advanceTimersByTimeAsync(7000); expect(reads).toBe(4);
    expect(await f.actions.refreshGit("project")).toBe(false); expect(reads).toBe(5);
    await vi.advanceTimersByTimeAsync(999); expect(reads).toBe(5);
    await vi.advanceTimersByTimeAsync(1); expect(reads).toBe(6);
    expect(f.toasts).toEqual(["missing checkout"]);
  } finally { close(); vi.useRealTimers(); }
});

it("skips a remote worktree without a request, toast or retry", async () => {
  vi.useFakeTimers();
  const requests: string[] = [];
  const f = fixture(async (tag, payload) => { requests.push(payload.worktreeId); return emptyGitReply(tag); });
  f.state.worktrees[0].machine = "remote";
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.advanceTimersByTimeAsync(60_000);
    expect(requests).toEqual([]); expect(f.toasts).toEqual([]);
    expect(f.state.worktrees[0]).toMatchObject({ gitUnsupported: true, ready: false });
    expect(vi.getTimerCount()).toBe(0);
  } finally { close(); vi.useRealTimers(); }
});

it("a scoped selection cannot cover integration after its full readback failed on a sibling", async () => {
  vi.useFakeTimers();
  const reads: string[] = []; let fail = false;
  const f = fixture(async (tag, payload) => {
    if (tag === "project.integrate") return { requestId: "integration" };
    if (tag === "project.git.state") { reads.push(payload.worktreeId); if (fail && payload.worktreeId === "sibling") throw new Error("missing sibling"); }
    return emptyGitReply(tag);
  });
  Object.assign(f.state.worktrees[0], { kind: "agent", parentId: "parent" });
  f.state.worktrees.push({ ...f.state.worktrees[0], id: "parent", branch: "parent", kind: "team" }, { ...f.state.worktrees[0], id: "sibling", branch: "sibling", kind: "team" });
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.advanceTimersByTimeAsync(0); fail = true;
    expect(await f.actions.integrate("workspace")).toBe(false);
    await f.actions.selectGit("workspace");
    fail = false; await vi.advanceTimersByTimeAsync(1000);
    expect(vi.getTimerCount()).toBe(0); reads.length = 0;
    f.actions.observe({ gitRevision: 1, gitChange: { requestId: "integration" } }); f.state.gitRevision = 1;
    await vi.advanceTimersByTimeAsync(1000);
    expect(reads).toContain("sibling");
    reads.length = 0;
    f.actions.observe({ gitRevision: 2, gitChange: { requestId: "integration" } }); f.state.gitRevision = 2;
    await vi.advanceTimersByTimeAsync(1000); expect(reads).toEqual([]);
  } finally { close(); vi.useRealTimers(); }
});

it("scopes working transitions to their agent worktree and catches up an agent that appears later", async () => {
  vi.useFakeTimers();
  const reads: string[] = [];
  const f = fixture(async (tag, payload) => { if (tag === "project.git.state") reads.push(payload.worktreeId); return emptyGitReply(tag); });
  f.state.worktrees[0].folder = "/shown";
  f.state.worktrees.push({ ...f.state.worktrees[0], id: "other", branch: "other", kind: "team", folder: "/other" });
  f.state.agents = [{ id: "worker", folder: "/shown" }];
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.advanceTimersByTimeAsync(0); reads.length = 0;
    f.actions.observe({ gitRevision: 1, gitChange: { agentId: "worker", scoped: true } }); f.state.gitRevision = 1;
    await vi.advanceTimersByTimeAsync(100); expect(reads).toEqual(["workspace"]);
    reads.length = 0;
    f.actions.observe({ gitRevision: 2, gitChange: { agentId: "later", scoped: true } }); f.state.gitRevision = 2;
    await vi.advanceTimersByTimeAsync(100); expect(reads).toEqual([]);
    const agents = [...f.state.agents, { id: "later", folder: "/other" }];
    f.actions.observe({ agents }); f.state.agents = agents;
    await vi.advanceTimersByTimeAsync(100); expect(reads).toEqual(["other"]);
    reads.length = 0; f.actions.observe({ agents }); await vi.advanceTimersByTimeAsync(1000);
    expect(reads).toEqual([]);
  } finally { close(); vi.useRealTimers(); }
});

it("waits for a busy action to finish without polling a timer", async () => {
  vi.useFakeTimers();
  const mutation = gate(); let reads = 0;
  const f = fixture(async tag => {
    if (tag === "project.stage.all") { await mutation.held; return { requestId: "mine" }; }
    if (tag === "project.git.state") reads++;
    return emptyGitReply(tag);
  });
  const close = f.actions.openScreen("git", "project");
  let action: Promise<unknown> | undefined;
  try {
    await vi.advanceTimersByTimeAsync(0); reads = 0;
    action = f.actions.stage("workspace", [], true, true);
    f.actions.observe({ gitRevision: 1, gitChange: { requestId: "foreign" } }); f.state.gitRevision = 1;
    await vi.advanceTimersByTimeAsync(1000);
    expect(reads).toBe(0); expect(vi.getTimerCount()).toBe(0);
    mutation.release(); await action; await vi.advanceTimersByTimeAsync(100);
    expect(reads).toBe(2);
  } finally { mutation.release(); await action; close(); vi.useRealTimers(); }
});

it("keeps a failed selected worktree visibly marked after the toast disappears", async () => {
  const kit = await import("../../../design/mock/vendor/htm-preact/standalone.module.js" as string);
  const source = copied("js/screens/git.js");
  const component = source.slice(source.indexOf("export function GitTab(")).replace("export ", "");
  const GitTab = new Function("html", "useEffect", "useMedia", "useState", "persist", "remember", "live", "liveGit", "agentOf", "machineOf", "Menu", "Icon", "AheadBehind", "Integrate", "Changes", "History", "Conflicts", "KIND_WORD", "ownerText", `${component}\nreturn GitTab;`)(
    kit.html, () => {}, () => false, () => ["unified", () => {}], () => "unified", () => {}, { on: true }, { busy: () => false }, () => null, () => ({ name: "local" }),
    () => null, () => null, () => null, () => null, () => null, () => null, () => null, {}, () => "Project lead");
  const element = document.createElement("div"); document.body.append(element);
  const project = { id: "project", branch: "project", repo: "repo" };
  const w = { id: "workspace", project: "project", branch: "project", kind: "project", files: [], gitError: "missing checkout" };
  try {
    kit.render(kit.h(GitTab, { s: { worktrees: [w], inspecting: {}, gitUi: {} }, project }), element);
    expect(element.querySelector('[role="alert"]')?.textContent).toBe("Git state could not be read: missing checkout");
    kit.render(kit.h(GitTab, { s: { worktrees: [{ ...w, gitError: null, gitUnsupported: true }], inspecting: {}, gitUi: {} }, project }), element);
    expect(element.querySelector('[role="status"]')?.textContent).toBe("Git is available only for a workspace on local.");
    expect(element.querySelector('[role="alert"]')).toBeNull();
  } finally { kit.render(null, element); element.remove(); }
});

for (const trigger of ["focus", "action"]) it(`${trigger} retries without waiting for read backoff and resets the delay`, async () => {
  vi.useFakeTimers(); let reads = 0;
  const f = fixture(async tag => {
    if (tag === "project.stage.all") return { requestId: "mine" };
    if (tag === "project.git.state") { reads++; throw new Error("missing checkout"); }
    return emptyGitReply(tag);
  });
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.advanceTimersByTimeAsync(7000); expect(reads).toBe(4);
    if (trigger === "focus") f.listeners.get("focus")!();
    if (trigger === "action") await f.actions.stage("workspace", [], true, true);
    await vi.advanceTimersByTimeAsync(trigger === "focus" ? 100 : 0); expect(reads).toBe(5);
    await vi.advanceTimersByTimeAsync(999); expect(reads).toBe(5);
    await vi.advanceTimersByTimeAsync(1); expect(reads).toBe(6);
    expect(f.toasts).toEqual(["missing checkout"]);
  } finally { close(); vi.useRealTimers(); }
});

it("a recovered scoped agent read resets its failure sequence", async () => {
  vi.useFakeTimers(); let fail = false, reads = 0;
  const f = fixture(async tag => { if (tag === "project.git.state") { reads++; if (fail) throw new Error("missing checkout"); } return emptyGitReply(tag); });
  f.state.worktrees[0].folder = "/shown"; f.state.agents = [{ id: "worker", folder: "/shown" }];
  const close = f.actions.openScreen("git", "project");
  const invalidate = (revision: number) => { f.actions.observe({ gitRevision: revision, gitChange: { agentId: "worker", scoped: true } }); f.state.gitRevision = revision; };
  try {
    await vi.advanceTimersByTimeAsync(0); fail = true; invalidate(1);
    await vi.advanceTimersByTimeAsync(100); expect(reads).toBe(2);
    fail = false; await vi.advanceTimersByTimeAsync(1000); expect(reads).toBe(3);
    expect(f.state.worktrees[0].gitError).toBeNull();
    fail = true; invalidate(2); await vi.advanceTimersByTimeAsync(100); expect(reads).toBe(4);
    await vi.advanceTimersByTimeAsync(999); expect(reads).toBe(4);
    await vi.advanceTimersByTimeAsync(1); expect(reads).toBe(5);
    expect(f.toasts).toEqual(["missing checkout"]);
  } finally { close(); vi.useRealTimers(); }
});

it("retires a queued agent refresh when its worktree leaves the roster", async () => {
  vi.useFakeTimers(); const reads: string[] = [];
  const f = fixture(async (tag, payload) => { if (tag === "project.git.state") reads.push(payload.worktreeId); return emptyGitReply(tag); });
  f.state.worktrees[0].folder = "/shown"; f.state.agents = [{ id: "worker", folder: "/shown" }];
  f.state.worktrees.push({ ...f.state.worktrees[0], id: "other", branch: "other", kind: "team", folder: "/other" });
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.advanceTimersByTimeAsync(0); reads.length = 0;
    f.actions.observe({ gitRevision: 1, gitChange: { agentId: "worker", scoped: true } }); f.state.gitRevision = 1;
    const patch = { worktrees: [f.state.worktrees[1]] };
    f.actions.observe(patch); Object.assign(f.state, patch);
    await vi.advanceTimersByTimeAsync(1000);
    expect(reads).toEqual(["other"]); expect(vi.getTimerCount()).toBe(0);
  } finally { close(); vi.useRealTimers(); }
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

for (const tag of ["project.diff", "project.history", "project.branches"]) it(`round 4: ${tag} failures stay in their own panel and deterministic refusals are not retried`, async () => {
  vi.useFakeTimers(); let failures = 0;
  const f = fixture(async request => {
    if (request === tag) { failures++; throw Object.assign(new Error("This detail is unavailable."), { refused: true, retryable: false, reason: "Git: invalid_path", gitCode: "invalid_path" }); }
    if (request === "project.changes") return { result: { items: [{ id: "file", path: "file.txt", staged: false }], truncated: false } };
    return emptyGitReply(request);
  });
  f.state.gitUi.workspace.tab = tag === "project.diff" ? "changes" : "history";
  const kit = await import("../../../design/mock/vendor/htm-preact/standalone.module.js" as string);
  const source = copied("js/screens/git.js");
  const Diff = new Function("html", "useState", "useMemo", "useEffect", "live", source.slice(source.indexOf("function Diff("), source.indexOf("// ---- changes")) + "\nreturn Diff;")(kit.html, () => [false, () => {}], (fn: () => unknown) => fn(), () => {}, { on: true });
  const History = new Function("html", "useState", "useNow", "live", "liveGit", "Empty", "cx", "Author", "Icon", "Ref", "ago", "plural", source.slice(source.indexOf("function History("), source.indexOf("// ---- conflicts")) + "\nreturn History;")(
    kit.html, kit.useState, () => Date.now(), { on: true }, f.actions, () => null, (...p: unknown[]) => p.filter(Boolean).join(" "), () => null, () => null, () => null, () => "now", (n: number, word: string) => `${n} ${word}`);
  if (tag !== "project.diff") f.state.worktrees[0].commits = [{ hash: "retained", subject: "Retained commit", refs: [], files: [], loaded: true }];
  const view = await gitTabView(f, { Changes: ({ w }: any) => kit.h(Diff, { f: w.files[0] }), History });
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.advanceTimersByTimeAsync(0);
    expect(f.state.worktrees[0].gitError).toBeNull(); expect(f.state.worktrees[0].ready).toBe(true);
    if (tag === "project.diff") expect(f.state.worktrees[0].files[0].diffError).toBeTruthy();
    else expect(f.state.worktrees[0].historyError).toBeTruthy();
    view.draw();
    expect(view.element.textContent).toContain("This detail is unavailable.");
    expect(view.element.querySelector<HTMLFieldSetElement>("fieldset")!.disabled).toBe(false);
    if (tag !== "project.diff") expect(view.element.querySelector(".git-commit-row")?.textContent).toContain("Retained commit");
    for (let i = 1; i <= 5; i++) { f.actions.observe({ gitRevision: i }); f.state.gitRevision = i; await vi.advanceTimersByTimeAsync(100); }
    await vi.advanceTimersByTimeAsync(120_000);
    expect(failures).toBe(1); expect(f.toasts).toHaveLength(1);
  } finally { close(); view.close(); vi.useRealTimers(); }
});

it("round 4: a failing scope does not delay healthy events or the sixty-second full fallback", async () => {
  vi.useFakeTimers(); vi.setSystemTime(100_000); let fail = false;
  const reads: { id: string; at: number }[] = [];
  const f = fixture(async (tag, payload) => {
    if (tag === "project.git.state") { reads.push({ id: payload.worktreeId, at: Date.now() }); if (fail && payload.worktreeId === "workspace") throw new Error("temporarily unavailable"); }
    return emptyGitReply(tag);
  });
  f.state.worktrees[0].folder = "/failing";
  f.state.worktrees.push({ ...f.state.worktrees[0], id: "healthy", branch: "healthy", kind: "team", folder: "/healthy" });
  f.state.agents = [{ id: "a", folder: "/failing" }, { id: "b", folder: "/healthy" }];
  const changed = (n: number, agentId: string) => { f.actions.observe({ gitRevision: n, gitChange: { agentId, scoped: true } }); f.state.gitRevision = n; };
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.advanceTimersByTimeAsync(0); fail = true; changed(1, "a");
    await vi.advanceTimersByTimeAsync(7100); reads.length = 0;
    changed(2, "b"); await vi.advanceTimersByTimeAsync(100);
    expect(reads).toEqual([{ id: "healthy", at: 107_200 }]);
    reads.length = 0; await vi.advanceTimersByTimeAsync(52_800);
    f.actions.observe({ gitHeartbeat: Date.now() }); await vi.advanceTimersByTimeAsync(100);
    expect(reads.some(r => r.id === "healthy")).toBe(true);
  } finally { close(); vi.useRealTimers(); }
});

it("round 4: a reconnect only resets failure delay after ten continuous seconds live", async () => {
  vi.useFakeTimers(); vi.setSystemTime(100_000); const times: number[] = [];
  const f = fixture(async tag => { if (tag === "project.git.state") { times.push(Date.now()); throw new Error("offline checkout"); } return emptyGitReply(tag); });
  const patchConn = (status: string) => { const patch = { conn: { status } }; f.actions.observe(patch); Object.assign(f.state, patch); };
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.advanceTimersByTimeAsync(7000); expect(times).toHaveLength(4);
    patchConn("reconnecting"); patchConn("live");
    await vi.advanceTimersByTimeAsync(7999); expect(times).toHaveLength(4);
    await vi.advanceTimersByTimeAsync(1); expect(times.at(-1)).toBe(115_000);
    await vi.advanceTimersByTimeAsync(2001); expect(times.at(-1)).toBe(117_001);
    await vi.advanceTimersByTimeAsync(1000); expect(times.at(-1)).toBe(118_001);
  } finally { close(); vi.useRealTimers(); }
});

for (const trigger of ["flapping", "alternating", "focus"]) it(`round 4: ${trigger} obeys 120 automatic requests in every sliding minute per worktree`, async () => {
  vi.useFakeTimers(); vi.setSystemTime(100_000); const calls: number[] = []; let stateReads = 0;
  const f = fixture(async tag => {
    calls.push(Date.now());
    if (tag === "project.git.state" && ++stateReads % 2 && trigger === "alternating") throw new Error(`varying failure ${stateReads}`);
    return emptyGitReply(tag);
  });
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.advanceTimersByTimeAsync(0);
    for (let i = 1; i <= 1300; i++) {
      if (trigger === "flapping") { f.state.conn = { status: "reconnecting" }; f.actions.observe({ conn: { status: "live" } }); f.state.conn = { status: "live" }; }
      else if (trigger === "focus") f.listeners.get("focus")!();
      else { f.actions.observe({ gitRevision: i }); f.state.gitRevision = i; }
      await vi.advanceTimersByTimeAsync(100);
    }
    for (const at of calls) expect(calls.filter(t => t > at - 60_000 && t <= at).length).toBeLessThanOrEqual(120);
    expect(calls.length).toBeGreaterThan(120);
    if (trigger === "alternating") expect(f.toasts.length).toBeLessThanOrEqual(3);
    expect(f.actions.busy("git")).toBe(false);
  } finally { close(); vi.useRealTimers(); }
});

it("round 4: a varying error alternating with success toasts at most once per minute per worktree", async () => {
  vi.useFakeTimers(); vi.setSystemTime(100_000); let fail = false, sequence = 0;
  const toastTimes: number[] = [];
  const f = fixture(async tag => { if (tag === "project.git.state" && fail) throw new Error(`error ${++sequence}`); return emptyGitReply(tag); });
  f.act.toast = () => toastTimes.push(Date.now());
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.advanceTimersByTimeAsync(0);
    for (let i = 1; i <= 30; i++) { fail = !!(i % 2); f.actions.observe({ gitRevision: i }); f.state.gitRevision = i; await vi.advanceTimersByTimeAsync(1100); }
    expect(toastTimes).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(60_000); fail = true; f.listeners.get("focus")!(); await vi.advanceTimersByTimeAsync(100);
    expect(toastTimes[1]! - toastTimes[0]!).toBeGreaterThanOrEqual(60_000);
  } finally { close(); vi.useRealTimers(); }
});

it("round 4: a waiting full agent event cannot be downgraded and closing clears waiting agents", async () => {
  vi.useFakeTimers(); const reads: string[] = [];
  const f = fixture(async (tag, p) => { if (tag === "project.git.state") reads.push(p.worktreeId); return emptyGitReply(tag); });
  f.state.worktrees[0].folder = "/shown";
  f.state.worktrees.push({ ...f.state.worktrees[0], id: "sibling", branch: "sibling", kind: "team", folder: "/sibling" });
  let close = f.actions.openScreen("git", "project");
  const changed = (n: number, scoped: boolean) => { f.actions.observe({ gitRevision: n, gitChange: { agentId: "later", scoped } }); f.state.gitRevision = n; };
  try {
    await vi.advanceTimersByTimeAsync(0); reads.length = 0; changed(1, false); changed(2, true);
    f.actions.observe({ agents: [{ id: "later", folder: "/shown" }] });
    await vi.advanceTimersByTimeAsync(100); expect(reads).toEqual(["workspace", "sibling"]);
    changed(3, false); close(); close = f.actions.openScreen("git", "project");
    await vi.advanceTimersByTimeAsync(0); reads.length = 0;
    f.actions.observe({ agents: [{ id: "later", folder: "/shown" }] });
    await vi.advanceTimersByTimeAsync(100); expect(reads).toEqual([]);
  } finally { close(); vi.useRealTimers(); }
});

it("round 4: a real binary conflict keeps the text file, explains the binary row, and aborts through the built module", async () => {
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
    let w = f.state.worktrees[0];
    expect(w.gitError).toBeNull(); expect(w.ready).toBe(true);
    expect(w.merge.files.find((file: any) => file.path === "text.txt")).toMatchObject({ loaded: true, parts: [{ ours: ["ours\n"], theirs: ["theirs\n"] }] });
    expect(w.merge.files.find((file: any) => file.path === "binary.bin").conflictError).toContain("Binary files");
    for (let i = 1; i <= 3; i++) { f.actions.observe({ gitRevision: i }); f.state.gitRevision = i; await vi.waitFor(() => expect(counts.get("project.git.state:")).toBe(i + 1)); await vi.waitFor(() => expect(f.actions.busy("git-read")).toBe(false)); }
    expect(counts.get("project.conflict:binary.bin")).toBe(1);
    await f.actions.selectGit("workspace", "conflicts");
    expect(counts.get("project.conflict:binary.bin")).toBe(1);
    const source = copied("js/screens/git.js");
    const component = source.slice(source.indexOf("const CHOICES"), source.indexOf("// ---- integrate strip"));
    const Conflicts = new Function("html", "live", "update", "Icon", "cx", "baseName", "dirName", "plural", "abortMerge", "continueMerge", "markResolved", "choose", `${component}\nreturn Conflicts;`)(
      kit.html, { on: true }, () => {}, () => null, (...parts: unknown[]) => parts.filter(Boolean).join(" "), (p: string) => p.split("/").at(-1), () => "", (n: number, noun: string) => `${n} ${noun}`, (_: unknown, selected: any) => f.actions.abortMerge(selected.id), () => {}, () => {}, () => {});
    w = f.state.worktrees[0];
    view = await gitTabView(f, { Conflicts }); view.draw();
    const panel = view.element;
    expect(panel.textContent).toContain("binary.bin"); expect(panel.textContent).toContain("Binary files");
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

it("round 4: an exhausted automatic budget does not refuse an owner action", async () => {
  vi.useFakeTimers(); let calls = 0, stages = 0;
  const f = fixture(async tag => { calls++; if (tag === "project.stage.all") { stages++; return { requestId: "mine" }; } return emptyGitReply(tag); });
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.advanceTimersByTimeAsync(0);
    for (let i = 1; i <= 100; i++) { f.listeners.get("focus")!(); await vi.advanceTimersByTimeAsync(100); }
    expect(calls).toBe(120); expect(f.actions.busy("git")).toBe(false);
    expect(await f.actions.stage("workspace", [], true, true)).toBe(true);
    expect(stages).toBe(1); expect(calls).toBeGreaterThan(120);
  } finally { close(); vi.useRealTimers(); }
});

it("round 4: a deterministic state refusal has no timer loop and roster changes permit a fresh input", async () => {
  vi.useFakeTimers(); let reads = 0;
  const f = fixture(async tag => { if (tag === "project.git.state") { reads++; throw Object.assign(new Error("Unavailable workspace"), { retryable: false }); } return emptyGitReply(tag); });
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.advanceTimersByTimeAsync(0); expect(reads).toBe(1); expect(vi.getTimerCount()).toBe(0);
    for (let i = 1; i <= 3; i++) { f.actions.observe({ gitRevision: i }); f.state.gitRevision = i; await vi.advanceTimersByTimeAsync(60_000); }
    expect(reads).toBe(1); expect(vi.getTimerCount()).toBe(0);
    const patch = { worktrees: [{ ...f.state.worktrees[0], folder: "/new-input" }] };
    f.actions.observe(patch); Object.assign(f.state, patch); await vi.advanceTimersByTimeAsync(100);
    expect(reads).toBe(2); expect(vi.getTimerCount()).toBe(0);
  } finally { close(); vi.useRealTimers(); }
});

it("round 4: a detail list larger than one automatic budget finishes without rereading completed files", async () => {
  vi.useFakeTimers(); const diffs: string[] = [];
  const f = fixture(async (tag, payload) => {
    if (tag === "project.changes") return { result: { items: Array.from({ length: 130 }, (_, i) => ({ id: `file-${i}`, path: `file-${i}`, staged: false })), truncated: false } };
    if (tag === "project.diff") { diffs.push(payload.path); return { result: { hunks: [] } }; }
    return emptyGitReply(tag);
  });
  f.state.gitUi.workspace.tab = "changes";
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.advanceTimersByTimeAsync(0); expect(diffs).toHaveLength(117); expect(f.state.worktrees[0].gitError).toBeNull();
    await vi.advanceTimersByTimeAsync(60_000); expect(diffs).toHaveLength(130); expect(new Set(diffs).size).toBe(130);
    expect(f.state.worktrees[0].files.every((file: any) => file.loaded)).toBe(true);
    expect(vi.getTimerCount()).toBe(0); expect(f.toasts).toEqual([]);
  } finally { close(); vi.useRealTimers(); }
});

it("round 4: a changed HEAD permits a fresh change-list input without repeating its old deterministic refusal", async () => {
  vi.useFakeTimers(); let head = "old", changes = 0;
  const f = fixture(async tag => {
    if (tag === "project.git.state") return { result: { head, mergeHead: null } };
    if (tag === "project.changes") { changes++; if (head === "old") throw Object.assign(new Error("Change list unavailable"), { retryable: false }); }
    return emptyGitReply(tag);
  });
  f.state.worktrees[0].head = "old";
  const close = f.actions.openScreen("git", "project");
  const changed = (n: number) => { f.actions.observe({ gitRevision: n }); f.state.gitRevision = n; };
  try {
    await vi.advanceTimersByTimeAsync(0); expect(changes).toBe(1);
    changed(1); await vi.advanceTimersByTimeAsync(100); expect(changes).toBe(1);
    head = "new"; changed(2); await vi.advanceTimersByTimeAsync(100);
    expect(changes).toBe(2); expect(f.state.worktrees[0]).toMatchObject({ head: "new", ready: true, gitError: null });
    expect(vi.getTimerCount()).toBe(0);
  } finally { close(); vi.useRealTimers(); }
});

it("round 4: a refused change list retains its latest observed HEAD while the displayed record is older", async () => {
  vi.useFakeTimers(); let head = "old", changes = 0;
  const f = fixture(async tag => {
    if (tag === "project.git.state") return { result: { head, mergeHead: null } };
    if (tag === "project.changes") { changes++; throw Object.assign(new Error("Change list unavailable"), { retryable: false }); }
    return emptyGitReply(tag);
  });
  f.state.worktrees[0].head = "old";
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.advanceTimersByTimeAsync(0); expect(changes).toBe(1);
    head = "new";
    for (let i = 1; i <= 3; i++) { f.actions.observe({ gitRevision: i }); f.state.gitRevision = i; await vi.advanceTimersByTimeAsync(100); }
    expect(changes).toBe(2); expect(f.state.worktrees[0].head).toBe("old");
    expect(vi.getTimerCount()).toBe(0);
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
