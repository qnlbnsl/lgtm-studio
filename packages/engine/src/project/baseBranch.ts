import { checkoutHead } from "./git/paths.ts"
import { randomUUID } from "node:crypto"
import * as Effect from "effect/Effect"
import { freshRequest, refused, type Input, type Runtime } from "../commands/internal.ts"
import { gitCall, safePath, withGit } from "./gitApi.ts"
import { own, projectView } from "./view.ts"

export const checkoutBaseReason = (base: string) => `check out ${JSON.stringify(base)} in the main repository checkout before creating a project`
export const isCheckoutBaseReason = (reason: string) => /^check out "(?:[^"\\\x00-\x20\x7f]|\\")+" in the main repository checkout before creating a project$/.test(reason)
export const readonlyBaseReason = (base: string) => `the base branch ${JSON.stringify(base)} is read-only; agents never edit it directly`
export const integrationBaseReason = (base: string) => `the base branch ${JSON.stringify(base)} requires the owner's review`
export const isBaseRefusal = (reason: string) => isCheckoutBaseReason(reason)
  || /^the base branch "(?:[^"\\\x00-\x20\x7f]|\\")+" (?:is read-only; agents never edit it directly|requires the owner's review)$/.test(reason)

// Unresolved legacy rows do not make every owner snapshot launch Git again.
// An explicit repository operation retries; an owner base.set always repairs.
const unresolved = new WeakMap<Runtime, Map<string, string>>()

// Called after typed authentication, outside the event writer. Resolve only
// repositories available to this caller. Append a new event; never rewrite history.
export const prepareRepositoryBases = (input: Input, name: string, raw: Record<string, unknown>) => Effect.gen(function* () {
  if (input.actor._tag !== "owner") return input
  // Pin legacy rows before the opening sequence of a coherent client read.
  // Otherwise the later repository read would invalidate that same read.
  const collection = ["client.sequence", "client.snapshot", "project.repositories", "project.snapshot"].includes(name)
  if (!name.startsWith("project.") && !collection) return input
  const view = projectView(input.snapshot)
  const project = own(view.projects, raw.projectId as string)
  const workspace = own(view.workspaces, raw.worktreeId as string)
  if (raw.projectId !== undefined && project === undefined
    || project !== undefined && (raw.repoId !== undefined && raw.repoId !== project.repoId
      || workspace !== undefined && workspace.repoId !== project.repoId)) return input
  const repoId = project?.repoId ?? workspace?.repoId ?? raw.repoId ?? (raw.repository as { repoId?: string } | undefined)?.repoId
  const candidates = Object.values(view.repositories).filter((repo) => repo.baseBranch === undefined
    && (repo.repoId === repoId || collection && repoId === undefined))
  let misses = unresolved.get(input.runtime)
  if (misses === undefined) { misses = new Map(); unresolved.set(input.runtime, misses) }
  for (const repo of candidates) {
    if (collection && (view.settings === undefined || !Object.values(view.locations).some((row) => row.repoId === repo.repoId && row.machineId === "local"))) continue
    const key = JSON.stringify([repo.repoId, repo.path, view.settings?.workspaceFolder])
    const head = yield* Effect.tryPromise({ try: () => checkoutHead(view.settings!.workspaceFolder, repo.path), catch: () => undefined }).pipe(Effect.catch(() => Effect.succeed("")))
    if (collection && misses.get(key) === head) continue
    misses.delete(key)
    if (name === "project.repository.base.set") continue
    const inferred = yield* Effect.gen(function* () {
      yield* safePath(input, repo.path)
      const baseBranch = yield* withGit(input, (git) => gitCall(git.detectBaseBranch(repo.path)))
      if ((yield* input.runtime.log.redactText(baseBranch)) !== baseBranch) return yield* refused("name is invalid after redaction")
      yield* input.runtime.log.transact((append) => Effect.gen(function* () {
        const current = own(projectView(yield* input.runtime.views.snapshot).repositories, repo.repoId)
        if (current?.baseBranch !== undefined) return
        if (current?.path !== repo.path) return yield* refused("project state changed; refresh and retry")
        yield* append(input.actor, freshRequest(), randomUUID(), [{ _tag: "project.RepositoryBaseChanged", repoId: repo.repoId, baseBranch }], {}, false)
      }))
    }).pipe(Effect.result)
    if (inferred._tag === "Failure") {
      misses.set(key, head)
    } else misses.delete(key)
  }
  return candidates.length === 0 ? input : { ...input, snapshot: yield* input.runtime.views.snapshot }
})
