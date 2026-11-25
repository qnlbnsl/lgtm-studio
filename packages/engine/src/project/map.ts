import * as Effect from "effect/Effect"
import type { MapSelection } from "@lgtm/contracts/project/map"
import type { ReadHandler } from "../api/reads.ts"
import { refused, type Input } from "../commands/internal.ts"
import { withGit, workspacePath, safePath, repositoryPath, gitCall } from "./gitApi.ts"
import { withMapFile, readMapFile } from "./mapFile.ts"
import { own, projectView } from "./view.ts"
import { mapDisplayRuntime } from "../client/web/display/mapRuntime.ts"

export const mapByteLimit = 16 * 1024 * 1024
const graphByteLimit = 16 * 1024 * 1024

const atCheckout = <A>(input: Input, selection: MapSelection, read: (root: string, selection: MapSelection) => Effect.Effect<A, import("../commands/internal.ts").CommandRefused>) => Effect.gen(function* () {
  if (input.actor._tag !== "owner") return yield* refused("the codebase map requires the owner")
  const view = projectView(input.snapshot)
  const project = selection.projectId === undefined ? undefined : own(view.projects, selection.projectId)
  if (selection.projectId !== undefined && (project === undefined || project.archiving || project.archivedAt !== undefined)) return yield* refused("project is unavailable")
  const repoId = selection.repoId ?? project?.repoId
  if (project !== undefined && repoId !== project.repoId) return yield* refused("map checkout identities disagree")
  const worktreeId = selection.worktreeId ?? project?.worktreeId
    ?? Object.values(view.workspaces).find((row) => row.repoId === repoId && row.machineId === "local" && row.checkout === "main")?.worktreeId
  if (worktreeId === undefined && repoId !== undefined) return yield* withGit(input, (git) => Effect.gen(function* () {
    const { repository, location } = yield* repositoryPath(input, repoId)
    const checkout = yield* gitCall(git.register(repository.path))
    const local = yield* gitCall(git.register(location.path))
    if (checkout.commonDirectory !== local.commonDirectory) return yield* refused("checkout belongs to a different repository")
    return yield* read(checkout.path, { repoId })
  }))
  if (worktreeId === undefined) return yield* refused("select a registered repository, project or workspace")
  return yield* withGit(input, (git) => Effect.gen(function* () {
    const workspace = yield* workspacePath(input, { worktreeId, ...(selection.projectId === undefined ? {} : { projectId: selection.projectId }) }, git)
    if (repoId !== undefined && workspace.repoId !== repoId) return yield* refused("map checkout identities disagree")
    yield* safePath(input, "floorplan/index.html")
    return yield* read(workspace.path, { repoId: workspace.repoId, worktreeId: workspace.worktreeId, ...(selection.projectId === undefined ? {} : { projectId: selection.projectId }) })
  }))
})

const mapFile = async (root: string) => {
  try { return await withMapFile(root, "floorplan/index.html", mapByteLimit, async (_file, info) => ({ size: info.size, modifiedAt: info.mtimeMs })) }
  catch (error) { if ((error as NodeJS.ErrnoException).code === "ENOENT") return undefined; throw error }
}
const facts = async (root: string) => {
  try {
    const graph = JSON.parse(await withMapFile(root, "floorplan/graph.json", graphByteLimit, async (file) => (await readMapFile(file, graphByteLimit)).toString("utf8")))
    const counts = graph?.meta?.counts
    const count = (value: unknown): value is number => typeof value === "number" && Number.isSafeInteger(value) && value >= 0
    return {
      ...(typeof graph?.meta?.generatedAt === "string" && graph.meta.generatedAt.length <= 100 ? { generatedAt: graph.meta.generatedAt } : {}),
      ...(count(counts?.files) ? { files: counts.files } : {}), ...(count(counts?.loc) ? { loc: counts.loc } : {}),
      ...(Array.isArray(graph?.nodes) ? { nodes: graph.nodes.length } : {}),
      ...(Array.isArray(graph?.links) ? { links: graph.links.length } : {})
    }
  } catch { return undefined }
}
export type CachedMap = { readonly text: string; readonly version: string; readonly redactionKey: object | undefined }
export const mapDisplay = (input: Input, selection: MapSelection, cached?: CachedMap) => atCheckout(input, selection, (root, resolved) => Effect.gen(function* () {
  const current = yield* Effect.tryPromise({ try: () => withMapFile(root, "floorplan/index.html", mapByteLimit, async (file, _info, version) => ({
    text: cached?.version === version ? cached.text : new TextDecoder("utf-8", { fatal: true }).decode(await readMapFile(file, mapByteLimit)), version
  })), catch: () => refused("map is unavailable, unsafe or exceeds 16 mebibytes") })
  const redactionKey = yield* input.runtime.log.redactor?.cacheKey ?? Effect.succeed(undefined)
  // A custom redactor without a registry identity is always checked afresh.
  if ((cached?.version !== current.version || redactionKey === undefined || redactionKey !== cached.redactionKey)
    && (yield* input.runtime.log.redactText(current.text)) !== current.text) return yield* refused("map contains private data")
  return { ...current, redactionKey, selection: resolved }
}))

const status: ReadHandler = (input, raw) => atCheckout(input, raw as MapSelection, (root) => Effect.tryPromise({ try: async () => {
  const file = await mapFile(root)
  const metadata = file === undefined ? { exists: false } : { exists: true, size: file.size, modifiedAt: file.modifiedAt }
  const summary = await facts(root)
  const clean = summary === undefined || await Effect.runPromise(input.runtime.log.redactText(JSON.stringify(summary))) !== JSON.stringify(summary) ? undefined : summary
  return { reply: { requestId: raw.requestId, result: { ...metadata, ...(clean === undefined ? {} : { facts: clean }) } } }
}, catch: () => refused("map is unsafe, unreadable or exceeds 16 mebibytes") }))
const open: ReadHandler = (input, raw) => Effect.gen(function* () {
  if (input.actor._tag !== "owner") return yield* refused("the codebase map requires the owner")
  const runtime = mapDisplayRuntime(input.runtime.views)
  if (runtime === undefined) return yield* refused("configure the website's isolated display host before opening a map")
  const frame = yield* runtime.open(raw as MapSelection, input)
  return { reply: { requestId: raw.requestId, frame } }
})
const release: ReadHandler = (input, raw) => Effect.gen(function* () {
  if (input.actor._tag !== "owner") return yield* refused("the codebase map requires the owner")
  yield* mapDisplayRuntime(input.runtime.views)?.release(raw.id as string) ?? Effect.void
  return { reply: { requestId: raw.requestId } }
})
export const mapReads = { "project.map": status, "project.map.open": open, "project.map.release": release }
