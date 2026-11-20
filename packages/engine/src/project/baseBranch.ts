import { randomUUID } from "node:crypto"
import * as Effect from "effect/Effect"
import { freshRequest, refused, type Input } from "../commands/internal.ts"
import { canReadProject } from "./access.ts"
import { gitCall, safePath, withGit } from "./gitApi.ts"
import { own, projectView } from "./view.ts"

export const checkoutBaseReason = (base: string) => `check out ${JSON.stringify(base)} in the main repository checkout before creating a project`
export const isCheckoutBaseReason = (reason: string) => /^check out "(?:[^"\\\x00-\x20\x7f]|\\")+" in the main repository checkout before creating a project$/.test(reason)
export const readonlyBaseReason = (base: string) => `the base branch ${JSON.stringify(base)} is read-only; agents never edit it directly`
export const integrationBaseReason = (base: string) => `the base branch ${JSON.stringify(base)} requires the owner's review`
export const isBaseRefusal = (reason: string) => isCheckoutBaseReason(reason)
  || /^the base branch "(?:[^"\\\x00-\x20\x7f]|\\")+" (?:is read-only; agents never edit it directly|requires the owner's review)$/.test(reason)

// Called after typed authentication, outside the event writer. Resolve only
// repositories available to this caller. Append a new event; never rewrite history.
export const prepareRepositoryBases = (input: Input, name: string, raw: Record<string, unknown>) => Effect.gen(function* () {
  // Pin legacy rows before the opening sequence of a coherent client read.
  // Otherwise the later repository read would invalidate that same read.
  const collection = ["client.sequence", "client.snapshot", "project.repositories", "project.snapshot"].includes(name)
  if ((!name.startsWith("project.") && !collection) || name === "project.repository.base.set") return input
  const relevant = collection || name === "project.create"
    || name === "project.placement.preview" || name.startsWith("project.repository.") || name.startsWith("project.files.")
    || name.startsWith("project.review") || name.startsWith("project.git.") || ["project.branches", "project.checkouts",
      "project.integrate", "project.update", "project.stage", "project.unstage", "project.stage.all", "project.unstage.all",
      "project.commit", "project.fetch", "project.pull", "project.push", "project.changes", "project.diff", "project.history"].includes(name)
  if (!relevant) return input
  const view = projectView(input.snapshot)
  const project = own(view.projects, raw.projectId as string)
  if (input.actor._tag !== "owner" && (project === undefined || !canReadProject(input.snapshot, input.actor, project.projectId))) return input
  const workspace = own(view.workspaces, raw.worktreeId as string)
  const repoId = raw.repoId ?? (raw.repository as { repoId?: string } | undefined)?.repoId ?? project?.repoId ?? workspace?.repoId
  const candidates = Object.values(view.repositories).filter((repo) => repo.baseBranch === undefined
    && (repo.repoId === repoId || input.actor._tag === "owner" && collection))
  for (const repo of candidates) {
    if (collection && (view.settings === undefined || !Object.values(view.locations).some((row) => row.repoId === repo.repoId && row.machineId === "local"))) continue
    yield* safePath(input, repo.path)
    const inferred = yield* withGit(input, (git) => gitCall(git.detectBaseBranch(repo.path))).pipe(Effect.result)
    if (inferred._tag === "Failure") {
      if (collection) continue
      return yield* inferred.failure
    }
    const baseBranch = inferred.success
    if ((yield* input.runtime.log.redactText(baseBranch)) !== baseBranch) return yield* refused("name is invalid after redaction")
    yield* input.runtime.log.transact((append) => Effect.gen(function* () {
      const current = own(projectView(yield* input.runtime.views.snapshot).repositories, repo.repoId)
      if (current?.baseBranch !== undefined) return
      if (current?.path !== repo.path) return yield* refused("project state changed; refresh and retry")
      yield* append(input.actor, freshRequest(), randomUUID(), [{ _tag: "project.RepositoryBaseChanged", repoId: repo.repoId, baseBranch }], {}, false)
    }))
  }
  return candidates.length === 0 ? input : { ...input, snapshot: yield* input.runtime.views.snapshot }
})
