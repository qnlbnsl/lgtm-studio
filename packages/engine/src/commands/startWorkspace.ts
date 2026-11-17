import { randomUUID } from "node:crypto";
import { realpathSync } from "node:fs";
import { ProjectId, WorkspaceId, parseRole, defaultChildPermission, type PermissionLevel, type Workspace } from "@lgtm/contracts";
import * as Effect from "effect/Effect";
import * as Schema from "effect/Schema";
import { start } from "./start.ts";
import { refused, type Input, type Payload, type Planner } from "./internal.ts";
import { own, projectView, workspaceInProject, allocatedWorkspaceInProject } from "../project/view.ts";
import { gitCall, localMachine, safePath, withGit, workspacePath } from "../project/gitApi.ts";
import { inside, slug } from "../project/git/paths.ts";

// The owner may choose a manually registered project descendant. This is a
// start placement choice, never a grant to project agents or project reads.
const ownerPlacement = (input: Input, workspace: Workspace, projectId: string) => {
  const view = projectView(input.snapshot), project = own(view.projects, projectId), seen = new Set<string>();
  let row: Workspace | undefined = workspace;
  while (row && row.repoId === project?.repoId && !seen.has(row.worktreeId)) {
    if (workspaceInProject(view, row, projectId)) return true;
    seen.add(row.worktreeId); row = row.parentId === undefined ? undefined : own(view.workspaces, row.parentId);
  }
  return false;
};
const context = (input: Input, p: Payload<"start">) => Effect.gen(function* () {
  const view = projectView(input.snapshot);
  const worker = parseRole(p.role).kind === "worker";
  const parent = input.actor._tag === "agent" ? own(input.snapshot.agents, input.actor.agentId) : undefined;
  const inherited = parent === undefined ? undefined : own(view.agentProjects, parent.agentId);
  if (input.actor._tag === "agent" && p.projectId !== undefined && p.projectId !== inherited)
    return yield* refused("worker-worktree:project_unavailable");
  const folder = parent?.folder ?? p.folder;
  const containing = Object.values(view.workspaces).filter((row) => inside(row.path, folder)).sort((a, b) => b.path.length - a.path.length);
  // Implicit owner placement retains its original exact-folder scope. A
  // broad historical registration must not adopt unrelated starts beneath it.
  const supplied = containing.find((row) => row.path === folder);
  const projectId = p.projectId ?? inherited ?? (input.actor._tag === "owner"
    && worker && supplied !== undefined ? Object.values(view.projects).find((project) => ownerPlacement(input, supplied, project.projectId))?.projectId : undefined);
  if (projectId === undefined) {
    if (p.worktreeId !== undefined) return yield* refused("worker-worktree:project_required");
    return undefined;
  }
  const project = own(view.projects, projectId);
  const belongs = (row: Workspace) => workspaceInProject(view, row, projectId)
    || input.actor._tag === "owner" && ownerPlacement(input, row, projectId);
  // Leads retain their requested folder. Resolve aliases before comparing
  // that effective checkout with the caller's own registered checkout.
  if (parent !== undefined && !worker && p.worktreeId === undefined && p.permission !== "read-only") {
    let requested = p.folder;
    try { requested = realpathSync(requested); } catch { /* The normal folder admission supplies the refusal. */ }
    const target = Object.values(view.workspaces).filter((row) => belongs(row) && inside(row.path, requested))
      .sort((a, b) => b.path.length - a.path.length)[0];
    if (target !== undefined && target.worktreeId !== containing.find(belongs)?.worktreeId)
      return yield* refused("Starting in another agent's worktree requires read-only permission.");
  }
  // Preserve ordinary start admission, including its archived-project reason,
  // when this command neither allocates nor selects a checkout.
  if (!worker && p.worktreeId === undefined) return undefined;
  if (!project || project.archiving || project.archivedAt !== undefined) return yield* refused("worker-worktree:project_unavailable");
  const base = containing.find(belongs) ?? (parent === undefined ? own(view.workspaces, project.worktreeId) : undefined);
  const selected = p.worktreeId === undefined ? undefined : own(view.workspaces, p.worktreeId);
  if (p.worktreeId !== undefined && (selected === undefined || !belongs(selected)))
    return yield* refused("worker-worktree:workspace_unavailable");
  if (input.actor._tag === "agent" && selected !== undefined && selected.worktreeId !== base?.worktreeId && p.permission !== "read-only")
    return yield* refused("Starting in another agent's worktree requires read-only permission.");
  if (worker && selected === undefined && parent !== undefined && parent.agentId !== project.leadAgentId && parseRole(parent.role).kind !== "lead"
    && base !== undefined && base.owner !== parent.agentId)
    return yield* refused("Only a lead or the agent that owns this checkout can allocate a worker from it. Ask your lead to start the worker.");
  return { project, base, selected, allocate: worker && selected === undefined };
});
const safeSlug = (name: string, identifier: string) => { try { return slug(name); } catch { return slug(identifier); } };
const same = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b);
const attempts = (input: Input, path: string) => Object.values(input.snapshot.agents).filter((agent) => agent.folder === path);
const failedAttempt = (input: Input, p: Payload<"start">, agent: Input["snapshot"]["agents"][string]) => agent.name === p.name
  && same(agent.parent, input.actor) && agent.state._tag === "stopped" && agent.startFailure !== undefined && agent.session === undefined;

export const prepareStartWorkspace = (input: Input, raw: Record<string, unknown>, allocated: (release: Effect.Effect<void>) => void) => Effect.gen(function* () {
  const p = { ...raw, permission: raw.permission ?? defaultChildPermission(input.actor._tag === "agent" ? input.snapshot.agents[input.actor.agentId]?.permission as PermissionLevel : undefined) } as Payload<"start">;
  const selection = yield* context(input, p);
  const projectId = selection?.project.projectId;
  const starterFolder = input.actor._tag === "agent" ? input.snapshot.agents[input.actor.agentId]?.folder : selection?.base?.path;
  const folder = selection?.selected?.path ?? (selection?.allocate ? starterFolder ?? p.folder : p.folder);
  const admission = { ...p, ...(projectId === undefined ? {} : { projectId }), folder };
  // Full permission, protected-folder, account and instruction checks precede
  // Git. Protected starts cannot create an orphan.
  yield* start(input, admission);
  if (selection === undefined || !selection.allocate && selection.selected === undefined)
    return ((current, payload) => Effect.gen(function* () {
      const normalized = { ...payload, permission: payload.permission ?? p.permission } as Payload<"start">;
      const now = yield* context(current, normalized);
      return yield* start(current, { ...normalized, ...(now === undefined ? {} : { projectId: now.project.projectId }) });
    })) satisfies Planner;
  const base = selection.base;
  if (selection.allocate && base === undefined) return yield* refused("The starter's checkout is not registered for this project. Ask your parent or the owner to register it before starting a worker.");
  if (selection.allocate && (base!.machineId !== "local" || selection.project.machineId !== "local"))
    return yield* refused("Worker worktrees can only be allocated on the local machine before machine pairing.");
  let workspace: Workspace | undefined = selection.selected;
  let created: Workspace | undefined;
  let reused: Workspace | undefined;
  yield* withGit(input, (git) => Effect.gen(function* () {
    if (workspace !== undefined) {
      yield* localMachine(workspace.machineId);
      // Owner placement may include a private manual descendant, without
      // broadening the project-scoped read interface.
      yield* workspacePath(input, { ...(input.actor._tag === "owner" ? {} : { projectId }), worktreeId: workspace.worktreeId }, git);
      return;
    }
    // Check the original text before slugging can disguise a registered
    // secret through case folding or punctuation removal.
    yield* safePath(input, p.name);
    yield* workspacePath(input, { ...(input.actor._tag === "owner" ? {} : { projectId }), worktreeId: base!.worktreeId }, git);
    const source = yield* gitCall(git.state(base!.path));
    const view = projectView(input.snapshot);
    for (const candidate of Object.values(view.workspaces)) {
      const previous = candidate.owner === undefined ? undefined : own(input.snapshot.agents, candidate.owner);
      if (!allocatedWorkspaceInProject(view, candidate, projectId!) || candidate.parentId !== base!.worktreeId
        || candidate.workerSourceHead === undefined || candidate.workerSourceHead !== source.head
        || previous === undefined || !failedAttempt(input, p, previous)
        || !attempts(input, candidate.path).every((agent) => failedAttempt(input, p, agent))) continue;
      // Missing, changed, dirty or switched candidates are retained. Allocation
      // can use another safe name without overwriting recovery evidence.
      const eligible = yield* Effect.gen(function* () {
        yield* workspacePath(input, { ...(input.actor._tag === "owner" ? {} : { projectId }), worktreeId: candidate.worktreeId }, git);
        const state = yield* gitCall(git.state(candidate.path)), changes = yield* gitCall(git.changes(candidate.path));
        return state.head === candidate.workerSourceHead && !changes.truncated && changes.items.length === 0;
      }).pipe(Effect.catchTag("CommandRefused", () => Effect.succeed(false)));
      if (eligible) { workspace = reused = candidate; return; }
    }
    const id = Schema.decodeUnknownSync(WorkspaceId)(randomUUID());
    const prefix = `worker/${safeSlug(selection.project.name, selection.project.projectId).slice(0, 48).replace(/-$/, "")}`;
    const name = safeSlug(p.name, id);
    yield* safePath(input, prefix); yield* safePath(input, name);
    const tree = yield* git.createWorktree(base!.path, name, "HEAD", prefix).pipe(
      Effect.mapError((error) => refused(`worker-worktree:${error.reason}`)),
    );
    created = { worktreeId: id, repoId: base!.repoId, machineId: base!.machineId, branch: tree.branch!, path: tree.path, checkout: "worktree",
      projectId: Schema.decodeUnknownSync(ProjectId)(projectId), parentId: base!.worktreeId, workerSourceHead: tree.head };
    workspace = created;
    allocated(Effect.gen(function* () {
      const snapshot = yield* input.runtime.views.snapshot;
      if (own(projectView(snapshot).workspaces, id) !== undefined) return;
      yield* withGit({ ...input, snapshot }, () => gitCall(git.rollbackWorktree(base!.path, tree))).pipe(
        Effect.catchTag("CommandRefused", () => Effect.logWarning("Uncommitted worker allocation could not be removed; inspect Git worktrees")),
      );
    }));
    yield* safePath(input, tree.path); yield* safePath(input, tree.branch!);
  }));
  const checked = workspace, newWorkspace = created, retryWorkspace = reused;
  return ((current, payload) => Effect.gen(function* () {
    const normalized = { ...payload, permission: payload.permission ?? p.permission } as Payload<"start">;
    const now = yield* context(current, normalized);
    if (projectView(current.snapshot).settings?.workspaceFolder !== projectView(input.snapshot).settings?.workspaceFolder
      || now === undefined || now.project.projectId !== projectId || !same(now.base, base) || !same(now.selected, selection.selected)
      || retryWorkspace !== undefined && (!same(own(projectView(current.snapshot).workspaces, retryWorkspace.worktreeId), retryWorkspace)
        || !same(attempts(current, retryWorkspace.path), attempts(input, retryWorkspace.path))))
      return yield* refused("worker-worktree:stale_state");
    const plan = yield* start(current, { ...normalized, projectId, folder: checked!.path }, newWorkspace);
    const registration = newWorkspace ?? retryWorkspace;
    if (!registration) return plan;
    const agent = plan.events.find((event) => event._tag === "AgentStarted");
    // Reusing a failed allocation keeps its id and source, and refreshes its
    // owner in the same append. Snapshot discovery must name the new agent.
    return { ...plan, events: [{ _tag: "project.WorktreeCreated" as const, ...registration, owner: agent!.agentId }, ...plan.events] };
  })) satisfies Planner;
});
