import { execFileSync } from "node:child_process";
import { writeFileSync } from "node:fs";
import { join } from "node:path";
import * as Effect from "effect/Effect";
import { agentCredential, appendSetup, call, eventOf, ownerCredential, seedAccount, seedAgent, seedHarness, success, type Rig } from "./handlerRig.ts";

export const workerGit = (folder: string, ...args: string[]) => execFileSync("/usr/bin/git", ["-C", folder, ...args], { encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] }).trim();
export const workerCommit = (folder: string, filename: string, content = "worker change\n") => {
  writeFileSync(join(folder, filename), content);
  workerGit(folder, "add", filename); workerGit(folder, "commit", "-m", filename);
  return workerGit(folder, "rev-parse", "HEAD");
};
export const workerProject = (rig: Rig) => Effect.gen(function* () {
  workerGit(rig.work, "init", "-b", "main");
  workerGit(rig.work, "config", "user.name", "Test owner"); workerGit(rig.work, "config", "user.email", "test@example.invalid");
  workerGit(rig.work, "config", "commit.gpgsign", "false");
  workerCommit(rig.work, "base.txt", "base\n");
  const path = join(rig.work, ".worktrees", "project");
  workerGit(rig.work, "worktree", "add", "-b", "project", path);
  // Match the service's managed worktree exclusion before checking dirt.
  writeFileSync(join(rig.work, ".git", "info", "exclude"), "/.worktrees/\n");
  const teamPath = join(rig.work, ".worktrees", "team");
  workerGit(path, "worktree", "add", "-b", "team", teamPath);
  yield* seedAccount(); yield* seedHarness();
  success(yield* call(ownerCredential, "project.settings.set", { settings: { workspaceFolder: rig.root, shell: "sh" } }));
  const repoId = success(yield* call(ownerCredential, "project.repository.add", { name: "Repository", path: rig.work, machineId: "local" })).repoId;
  const register = (path: string, branch: string, parentId?: unknown) => call(ownerCredential, "project.workspace.register", { repoId, machineId: "local", path, branch, checkout: parentId === undefined ? "main" : "worktree", ...(parentId === undefined ? {} : { parentId }) });
  const mainId = success(yield* register(rig.work, "main")).worktreeId;
  const worktreeId = success(yield* register(path, "project", mainId)).worktreeId;
  const lead = yield* seedAgent({ id: "project-lead", role: "lead:project", folder: path });
  const projectId = "worker-project";
  yield* appendSetup([eventOf({ _tag: "project.Created", projectId, repoId, worktreeId, leadAgentId: lead.agentId, machineId: "local", name: "Work trees", goal: "Isolate workers" })]);
  const teamWorktreeId = success(yield* register(teamPath, "team", worktreeId)).worktreeId;
  const team = success(yield* call(ownerCredential, "project.team.create", { projectId, name: "Build", worktreeId: teamWorktreeId }));
  const teamLead = yield* seedAgent({ id: "team-lead", role: "lead:team", folder: teamPath, parent: lead.agentId, groups: [team.group as string] });
  const outside = yield* seedAgent({ id: "outside", role: "lead", folder: rig.outside });
  return { projectId, repoId, worktreeId, mainId, path, lead, teamPath, teamWorktreeId, team, teamLead, outside,
    leadCredential: agentCredential(lead.token), teamCredential: agentCredential(teamLead.token), outsideCredential: agentCredential(outside.token) };
});
export const workerPayload = (folder: string, extra: Record<string, unknown> = {}) => ({ name: "Builder", role: "builder:typescript", harness: "first", account: { _tag: "any" }, permission: "ask", folder, ...extra });
