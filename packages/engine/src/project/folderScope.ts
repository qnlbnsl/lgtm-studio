import { statSync } from "node:fs";
import { isAbsolute, join } from "node:path";
import type { AgentId, Workspace } from "@lgtm/contracts";
import * as Effect from "effect/Effect";
import * as Redacted from "effect/Redacted";
import { harnessLogin } from "../runner/harnesses.ts";
import type { Runtime, Snapshot } from "../commands/internal.ts";
import { allocatedWorkspaceInProject, own, projectView } from "./view.ts";

// Only project roots, team roots and engine allocations grant home admission.
// A manual descendant in the Git read scope is not a folder grant.
export const projectFolderScope = (runtime: Runtime, starterId: AgentId | undefined,
  pending?: { readonly workspace: Workspace; readonly sequence: Snapshot["sequence"] }) => ({
  projectWorktrees: runtime.views.snapshot.pipe(Effect.map((snapshot) => {
    const view = projectView(snapshot), id = starterId === undefined ? undefined : own(view.agentProjects, starterId);
    const project = id === undefined ? undefined : own(view.projects, id);
    const boundaries = Object.values(view.workspaces).filter(row => row.machineId === "local").map(row => row.path);
    if (!project || project.archiving || project.archivedAt !== undefined) return { allowed: [], boundaries };
    const paths = Object.values(view.workspaces).filter((row) => row.machineId === "local" && row.repoId === project.repoId
      && (row.worktreeId === project.worktreeId || Object.values(view.teams).some((team) => team.projectId === id && team.worktreeId === row.worktreeId)
        || allocatedWorkspaceInProject(view, row, id!))).map((row) => row.path);
    // The allocator's private, precommit registration is usable only at its
    // validated log head. After append, only the live registration grants it.
    if (pending && snapshot.sequence === pending.sequence && pending.workspace.projectId === id
      && pending.workspace.repoId === project.repoId && pending.workspace.machineId === "local") {
      paths.push(pending.workspace.path); boundaries.push(pending.workspace.path);
    }
    return { allowed: paths, boundaries };
  })),
  harnessHomes: Effect.gen(function* () {
    const snapshot = yield* runtime.views.snapshot;
    const homes = Object.values(snapshot.accounts).flatMap(account => account.home === undefined ? [] : [account.home]);
    for (const account of Object.values(snapshot.accounts)) {
      const variable = harnessLogin(account.harness).homeVariable;
      const resolved: Record<string, string> = {};
      for (const name of ["HOME", "XDG_CONFIG_HOME", ...(variable === undefined ? [] : [variable])]) {
        const raw = account.environment?.[name] ?? process.env[name];
        if (raw === undefined) continue;
        const secret = raw.startsWith("secret:") ? yield* runtime.secrets.resolve(raw) : undefined;
        const path = secret === undefined ? raw : Redacted.value(secret);
        if (!path.startsWith("secret:") && path.trim().length > 0) resolved[name] = path;
      }
      if (variable !== undefined && resolved[variable] !== undefined) homes.push(resolved[variable]);
      // Match the login-store fallbacks in accounts/credentialSource.ts.
      // Protect directories only; never inspect their credential files.
      const home = resolved.HOME ?? runtime.home;
      if (account.harness === "claude") homes.push(join(home, ".claude"));
      if (account.harness === "codex") homes.push(join(home, ".codex"));
      if (account.harness === "gemini") homes.push(join(home, ".gemini"));
      if (account.harness === "cursor") homes.push(join(home, ".config", "cursor"), join(resolved.XDG_CONFIG_HOME ?? join(home, ".config"), "cursor"));
    }
    return homes.filter(path => {
      if (!isAbsolute(path)) return false;
      try { return statSync(path).isDirectory(); }
      catch (error) { return !["ENOENT", "ENOTDIR"].includes((error as NodeJS.ErrnoException).code ?? ""); }
    });
  }),
});
