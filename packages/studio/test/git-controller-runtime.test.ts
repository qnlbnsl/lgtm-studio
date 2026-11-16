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
  const dialogs: any[] = [], toasts: string[] = [], listeners = new Map<string, () => void>();
  const act: any = { confirm: (d: any) => dialogs.push(d), closeDialog: vi.fn(), toast: (s: string) => toasts.push(s) };
  const source = copied("js/live/git.js").replace(/^import .*;$/gm, "").replace(/export /g, "");
  const actions = new Function("store", "live", "engineWrite", "notify", "engine", "act", "update", "addEventListener", "removeEventListener",
    `${source}\nreturn { installMappings, openScreen, refreshGit, observe, busy, declineReview, discard, stage, integrate, selectGit, commit, abortMerge, resolve };`)(
    { state }, { on: true }, (p: object) => Object.assign(state, p), () => {}, { request }, act, (f: (s: any) => void) => f(state), (name: string, fn: () => void) => listeners.set(name, fn), (name: string) => listeners.delete(name));
  actions.installMappings({ ...refreshPolicy, mapGitState: (s: any) => s, mapGitChanges: (items: any[]) => items,
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
