import { readFileSync } from "node:fs";
import { expect, it, vi } from "vitest";
import { gitActionsSource } from "../scripts/gitActions.ts";

const copied = (path: string) => gitActionsSource(path, readFileSync(`${process.cwd()}/design/mock/${path}`, "utf8"));
const gate = () => { let release!: () => void; const held = new Promise<void>((resolve) => { release = resolve; }); return { held, release }; };
function fixture(request: (tag: string, payload: any) => Promise<unknown>) {
  const project = { id: "project", repo: "repo", lead: "lead" };
  const state: any = { session: { status: "in" }, ui: {}, projects: [project], inspecting: {}, gitUi: { workspace: { tab: "conflicts" } },
    worktrees: [{ id: "workspace", project: project.id, repo: project.repo, branch: "project", machine: "local", kind: "project", files: [], merge: null,
      review: { id: "review", project: project.id, head: "head", baseHead: "base", agent: "lead" } }] };
  const dialogs: any[] = [], toasts: string[] = [];
  const act: any = { confirm: (d: any) => dialogs.push(d), closeDialog: vi.fn(), toast: (s: string) => toasts.push(s) };
  const source = copied("js/live/git.js").replace(/^import .*;$/gm, "").replace(/export /g, "");
  const actions = new Function("store", "live", "engineWrite", "notify", "engine", "act", "update", "addEventListener", "removeEventListener",
    `${source}\nreturn { installMappings, openScreen, refreshGit, busy, declineReview, discard, stage, integrate, selectGit, commit, syncGit, abortMerge, updateFromBase, answerReview };`)(
    { state }, { on: true }, (p: object) => Object.assign(state, p), () => {}, { request }, act, (f: (s: any) => void) => f(state), () => {}, () => {});
  actions.installMappings({ mapGitState: (s: any) => s, mapGitChanges: (items: any[]) => items,
    mapMainReview: (r: any) => r, mapConflict: (c: any) => ({ ...c, id: c.path, selection: c }) });
  return { state, actions, act, dialogs, toasts };
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

const narrowedActions = ["commit", "pull", "push", "update", "integrate", "abort", "approval"] as const;
for (const action of narrowedActions) it(`${action} refreshes affected sibling counts without reading an unaffected worktree`, async () => {
  const reads: string[] = [], mutations: string[] = [], reviews: string[] = []; let changed = false;
  const f = fixture(async (tag, payload) => {
    if (tag === "project.git.state") { reads.push(payload.worktreeId); return { result: { head: "fresh", mergeHead: null, behind: changed ? 7 : 0 } }; }
    if (tag === "project.changes") return { result: { items: [], truncated: false } };
    if (tag === "project.review.status") { reviews.push(payload.worktreeId); return { result: { review: null } }; }
    mutations.push(tag); changed = true;
    if (tag === "project.review.approve") return { result: { id: "review", outcome: "merged" } };
    return {};
  });
  const source = f.state.worktrees[0];
  Object.assign(source, { kind: action === "integrate" ? "team" : "project", parentId: "destination", mergeHead: "merge", merge: { from: "other", files: [] },
    review: { ...source.review, destinationId: "destination", evidence: { tests: { ok: false }, review: { ok: false } } } });
  const add = (id: string, parentId: string | null) => ({ ...source, id, branch: id, kind: "team", parentId, merge: null, mergeHead: null, review: null, behind: 0 });
  f.state.worktrees.push(add("destination", "base"), add("source-child-a", source.id), add("source-child-b", source.id),
    add("destination-sibling", "destination"), add("unaffected", "base"), add("grandchild", "source-child-a"));
  f.state.worktrees.find((w: any) => w.id === "source-child-a").kind = "project";
  const close = f.actions.openScreen("git", "project");
  await vi.waitFor(() => expect(f.actions.busy("git-read")).toBe(false));
  reads.length = 0; mutations.length = 0; reviews.length = 0;
  if (action === "abort") Object.assign(f.state.worktrees[0], { mergeHead: "merge", merge: { from: "other", files: [] } });
  try {
    if (action === "commit") await f.actions.commit(source.id, "A commit");
    if (action === "pull" || action === "push") await f.actions.syncGit(source.id, action);
    if (action === "update") await f.actions.updateFromBase(source.id);
    if (action === "integrate") await f.actions.integrate(source.id);
    if (action === "abort") { f.actions.abortMerge(source.id); await f.dialogs[0].onConfirm(); }
    if (action === "approval") await f.actions.answerReview("review", "merge");
    const hasDestination = ["update", "integrate", "approval"].includes(action);
    const expected = hasDestination
      ? [source.id, "destination", "source-child-a", "source-child-b", "destination-sibling"]
      : [source.id, "source-child-a", "source-child-b"];
    expect(reads).toEqual(expected);
    expect(reviews).toEqual(expected.filter((id) => f.state.worktrees.find((w: any) => w.id === id).kind === "project"));
    expect(mutations).toEqual([`project.${({ abort: "merge.abort", approval: "review.approve" } as Record<string, string>)[action] || action}`]);
    for (const id of expected) expect(f.state.worktrees.find((w: any) => w.id === id).behind).toBe(7);
    expect(f.state.worktrees.find((w: any) => w.id === "unaffected").behind).toBe(0);
    expect(f.state.worktrees.find((w: any) => w.id === "grandchild").behind).toBe(0);
    expect(f.toasts).toEqual([]);
  } finally { close(); }
});

it("Fetch and Refresh Git keep reading every worktree", async () => {
  const reads: string[] = [];
  const f = fixture(async (tag, payload) => {
    if (tag === "project.fetch") return {};
    if (tag === "project.git.state") { reads.push(payload.worktreeId); return { result: { mergeHead: null } }; }
    if (tag === "project.changes") return { result: { items: [], truncated: false } };
    if (tag === "project.review.status") return { result: { review: null } };
    throw new Error(tag);
  });
  f.state.worktrees.push({ ...f.state.worktrees[0], id: "unrelated", branch: "unrelated", kind: "team" });
  const close = f.actions.openScreen("git", "project");
  await vi.waitFor(() => expect(f.actions.busy("git-read")).toBe(false));
  reads.length = 0;
  try {
    await f.actions.syncGit("workspace", "fetch");
    expect(reads).toEqual(["workspace", "unrelated"]);
    reads.length = 0;
    await f.actions.refreshGit("project");
    expect(reads).toEqual(["workspace", "unrelated"]);
    expect(f.toasts).toEqual([]);
  } finally { close(); }
});

it("review status is read on first load, explicit refresh, head changes and review actions, but not index actions or repeated background reads", async () => {
  let reviews = 0;
  const f = fixture(async (tag) => {
    if (["project.stage.all", "project.commit", "project.discard", "project.conflict.reopen", "project.conflict.resolve"].includes(tag)) return {};
    if (tag === "project.discard.plan") return { result: { paths: [] } };
    if (tag === "project.git.state") return { result: { head: "head", mergeHead: null } };
    if (tag === "project.changes") return { result: { items: [], truncated: false } };
    if (tag === "project.review.status") { reviews++; return { result: { review: null } }; }
    if (tag === "project.review.decline") return { result: { id: "review", outcome: "sent_back" } };
    throw new Error(tag);
  });
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.waitFor(() => expect(f.actions.busy("git-read")).toBe(false));
    expect(reviews).toBe(1);
    await f.actions.refreshGit("project", true);
    await f.actions.stage("workspace", [], true, true);
    await f.actions.discard(f.state.worktrees[0], []); await f.dialogs[0].onConfirm();
    await f.actions.selectGit("workspace");
    expect(reviews).toBe(1);
    await f.actions.refreshGit("project"); expect(reviews).toBe(2);
    await f.actions.commit("workspace", "A commit"); expect(reviews).toBe(3);
    await f.actions.declineReview("review", "Please verify"); expect(reviews).toBe(4);
    expect(f.toasts).toEqual([]);
  } finally { close(); }
});

it("a refused initial review read is retried rather than marked loaded", async () => {
  let reviews = 0;
  const f = fixture(async (tag) => {
    if (tag === "project.git.state") return { result: { mergeHead: null } };
    if (tag === "project.changes") return { result: { items: [], truncated: false } };
    if (tag === "project.review.status") { if (++reviews === 1) throw new Error("Review temporarily unavailable"); return { result: { review: null } }; }
    throw new Error(tag);
  });
  expect(await f.actions.refreshGit("project", true)).toBe(false);
  expect(await f.actions.refreshGit("project", true)).toBe(true);
  expect(reviews).toBe(2);
});

it("a background refresh shows passing evidence reported by an agent for an open review", async () => {
  let passed = false, reviews = 0;
  const request = async (tag: string) => {
    if (tag === "project.review.report") { passed = true; return {}; }
    if (tag === "project.git.state") return { result: { head: "head", mergeHead: null } };
    if (tag === "project.changes") return { result: { items: [], truncated: false } };
    if (tag === "project.review.status") {
      reviews++;
      return { result: { review: { id: "review", status: "open", head: "head", baseHead: "base", files: [],
        evidence: { tests: { ok: passed }, review: { ok: passed } } } } };
    }
    throw new Error(tag);
  };
  const f = fixture(request), close = f.actions.openScreen("git", "project");
  try {
    await vi.waitFor(() => expect(f.actions.busy("git-read")).toBe(false));
    expect(f.state.worktrees[0].review.evidence).toMatchObject({ tests: { ok: false }, review: { ok: false } });
    await request("project.review.report");
    await f.actions.refreshGit("project", true);
    expect(f.state.worktrees[0].review.evidence).toMatchObject({ tests: { ok: true }, review: { ok: true } });
    expect(reviews).toBe(2);
    expect(f.toasts).toEqual([]);
  } finally { close(); }
});

for (const status of [null, "answered"]) it(`a background refresh skips an already loaded review with status ${status}`, async () => {
  let reviews = 0;
  const f = fixture(async (tag) => {
    if (tag === "project.git.state") return { result: { head: "head", mergeHead: null } };
    if (tag === "project.changes") return { result: { items: [], truncated: false } };
    if (tag === "project.review.status") {
      reviews++;
      return { result: { review: status ? { id: "review", status, evidence: { tests: { ok: true }, review: { ok: true } } } : null } };
    }
    throw new Error(tag);
  });
  if (status === null) f.state.worktrees[0].review = null;
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.waitFor(() => expect(f.actions.busy("git-read")).toBe(false));
    expect(reviews).toBe(1);
    await f.actions.refreshGit("project", true);
    expect(reviews).toBe(1);
    expect(f.toasts).toEqual([]);
  } finally { close(); }
});

it("opening the Git screen again reads review status once for the new visit", async () => {
  let reviews = 0;
  const f = fixture(async (tag) => {
    if (tag === "project.git.state") return { result: { mergeHead: null } };
    if (tag === "project.changes") return { result: { items: [], truncated: false } };
    if (tag === "project.review.status") { reviews++; return { result: { review: null } }; }
    throw new Error(tag);
  });
  let close = f.actions.openScreen("git", "project");
  try {
    await vi.waitFor(() => expect(f.actions.busy("git-read")).toBe(false));
    expect(reviews).toBe(1);
    close();
    close = f.actions.openScreen("git", "project");
    await vi.waitFor(() => expect(f.actions.busy("git-read")).toBe(false));
    expect(reviews).toBe(2);
    await f.actions.refreshGit("project", true);
    expect(reviews).toBe(2);
    expect(f.toasts).toEqual([]);
  } finally { close(); }
});

for (const operation of ["pull", "push"]) it(`${operation} ${operation === "pull" ? "refreshes" : "does not read"} the base row while refreshing the source and its children`, async () => {
  const reads: string[] = []; let changed = false;
  const f = fixture(async (tag, payload) => {
    if (tag === `project.${operation}`) { changed = true; return {}; }
    if (tag === "project.git.state") { reads.push(payload.worktreeId); return { result: { mergeHead: null, ahead: changed ? 0 : 1 } }; }
    if (tag === "project.changes") return { result: { items: [], truncated: false } };
    if (tag === "project.review.status") return { result: { review: null } };
    throw new Error(tag);
  });
  const source = f.state.worktrees[0];
  Object.assign(source, { parentId: "base" });
  f.state.worktrees.push({ ...source, id: "child", branch: "child", kind: "team", parentId: source.id },
    { ...source, id: "base", project: undefined, branch: "main", kind: "base", parentId: null },
    { ...source, id: "base-child", branch: "base-child", kind: "team", parentId: "base" });
  const close = f.actions.openScreen("git", "project");
  try {
    await vi.waitFor(() => expect(f.actions.busy("git-read")).toBe(false));
    expect(f.state.worktrees.find((w: any) => w.id === "base").ahead).toBe(1);
    reads.length = 0;
    await f.actions.syncGit(source.id, operation);
    expect(reads).toEqual(operation === "pull" ? [source.id, "child", "base"] : [source.id, "child"]);
    expect(f.state.worktrees.find((w: any) => w.id === "base").ahead).toBe(operation === "pull" ? 0 : 1);
    expect(f.state.worktrees.find((w: any) => w.id === "base-child").ahead).toBe(1);
    expect(f.toasts).toEqual([]);
  } finally { close(); }
});
