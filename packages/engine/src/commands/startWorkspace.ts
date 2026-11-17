import { randomUUID } from "node:crypto";
import { ProjectId, WorkspaceId, type Workspace } from "@lgtm/contracts";
import * as Effect from "effect/Effect";
import * as Schema from "effect/Schema";
import { start } from "./start.ts";
import { refused, type Input, type Payload, type Planner } from "./internal.ts";
import { own, projectView, workspaceInProject } from "../project/view.ts";
import { gitCall, localMachine, safePath, withGit, workspacePath } from "../project/gitApi.ts";
import { slug } from "../project/git/paths.ts";

// Role names use the existing lead/orchestrator convention. New worker roles
// inherit isolation without having to be added to a hard-coded role inventory.
const leadRole = (role: string) => role.startsWith("lead") || role.startsWith("orchestrator");
const context = (input: Input, p: Payload<"start">) => Effect.gen(function* () {
  const view = projectView(input.snapshot);
  const parent = input.actor._tag === "agent" ? own(input.snapshot.agents, input.actor.agentId) : undefined;
  const inherited = parent === undefined ? undefined : own(view.agentProjects, parent.agentId);
  if (input.actor._tag === "agent" && p.projectId !== undefined && p.projectId !== inherited)
    return yield* refused("worker-worktree:project_unavailable");
  const supplied = Object.values(view.workspaces).find((row) => row.path === (parent?.folder ?? p.folder));
  const projectId = p.projectId ?? inherited ?? (input.actor._tag === "owner" && supplied !== undefined
    ? Object.values(view.projects).find((project) => workspaceInProject(view, supplied, project.projectId))?.projectId : undefined);
  if (projectId === undefined) {
    if (p.worktreeId !== undefined) return yield* refused("worker-worktree:project_required");
    return undefined;
  }
  const project = own(view.projects, projectId);
  if (!project || project.archiving || project.archivedAt !== undefined) return yield* refused("worker-worktree:project_unavailable");
  const base = supplied !== undefined && workspaceInProject(view, supplied, projectId) ? supplied : own(view.workspaces, project.worktreeId);
  if (base === undefined || parent !== undefined && base.path !== parent.folder) return yield* refused("worker-worktree:starter_unregistered");
  const selected = p.worktreeId === undefined ? undefined : own(view.workspaces, p.worktreeId);
  if (p.worktreeId !== undefined && (selected === undefined || !workspaceInProject(view, selected, projectId)))
    return yield* refused("worker-worktree:workspace_unavailable");
  return { project, base, selected, allocate: !leadRole(p.role) && selected === undefined };
});

// Admission and Git run before the writer. Only checked checkout facts cross
// back into the writer, where permission, account, project and parent checks
// run again. Manual registration remains owner-only.
export const prepareStartWorkspace = (input: Input, raw: Record<string, unknown>, allocated: (release: Effect.Effect<void>) => void) => Effect.gen(function* () {
  const p = raw as Payload<"start">;
  const selection = yield* context(input, p);
  const projectId = selection?.project.projectId;
  const admission = { ...p, ...(projectId === undefined ? {} : { projectId }), folder: selection?.selected?.path ?? (selection?.allocate ? selection.base.path : p.folder) };
  // Reuse the full start admission before touching Git. This plan is discarded.
  yield* start(input, admission);
  if (selection === undefined) return ((current, payload) => start(current, payload as Payload<"start">)) satisfies Planner;
  let workspace: Workspace | undefined = selection.selected;
  let created: Workspace | undefined;
  yield* withGit(input, (git) => Effect.gen(function* () {
    yield* localMachine(selection.base.machineId);
    yield* workspacePath(input, { projectId, worktreeId: selection.base.worktreeId }, git);
    if (workspace !== undefined) {
      yield* workspacePath(input, { projectId, worktreeId: workspace.worktreeId }, git);
    } else if (selection.allocate) {
      const changes = yield* gitCall(git.changes(selection.base.path));
      if (changes.items.length || changes.truncated) return yield* refused("worker-worktree:dirty_source");
      // Slugs are computed by the Git service too. Check their text before a
      // Git mutation so no secret-bearing path enters the event log.
      const prefix = yield* Effect.try({ try: () => `worker/${slug(selection.project.name).slice(0, 48).replace(/-$/, "")}`, catch: () => refused("worker-worktree:invalid_name") });
      yield* safePath(input, prefix); yield* safePath(input, p.name);
      const tree = yield* git.createWorktree(selection.base.path, p.name, "HEAD", prefix).pipe(
        Effect.mapError((error) => refused(`worker-worktree:${error.reason}`)),
      );
      created = { worktreeId: Schema.decodeUnknownSync(WorkspaceId)(randomUUID()), repoId: selection.base.repoId,
        machineId: selection.base.machineId, branch: tree.branch!, path: tree.path, checkout: "worktree",
        projectId: Schema.decodeUnknownSync(ProjectId)(projectId), parentId: selection.base.worktreeId };
      workspace = created;
      const id = created.worktreeId;
      // Register cleanup before leaving the uninterruptible Git gate. A
      // durable registration means the start owns its checkout thereafter,
      // including a harness rejection after the append.
      allocated(Effect.gen(function* () {
        const snapshot = yield* input.runtime.views.snapshot;
        if (own(projectView(snapshot).workspaces, id) !== undefined) return;
        yield* withGit({ ...input, snapshot }, () => gitCall(git.rollbackWorktree(selection.base.path, tree))).pipe(
          Effect.catchTag("CommandRefused", () => Effect.logWarning("Uncommitted worker allocation could not be removed; inspect Git worktrees")),
        );
      }));
      yield* safePath(input, tree.path); yield* safePath(input, tree.branch!);
    }
  }));
  const checked = workspace;
  const newWorkspace = created;
  return ((current, payload) => Effect.gen(function* () {
    const now = yield* context(current, payload as Payload<"start">);
    if (projectView(current.snapshot).settings?.workspaceFolder !== projectView(input.snapshot).settings?.workspaceFolder
      || now === undefined || now.project.projectId !== projectId || now.base.worktreeId !== selection.base.worktreeId
      || now.base.path !== selection.base.path || now.base.branch !== selection.base.branch
      || JSON.stringify(now.selected) !== JSON.stringify(selection.selected)) return yield* refused("worker-worktree:stale_state");
    const plan = yield* start(current, { ...payload as Payload<"start">, projectId, folder: checked?.path ?? p.folder });
    if (!newWorkspace) return plan;
    const agent = plan.events.find((event) => event._tag === "AgentStarted");
    return { ...plan, events: [{ _tag: "project.WorktreeCreated" as const, ...newWorkspace, owner: agent!.agentId }, ...plan.events] };
  })) satisfies Planner;
});
