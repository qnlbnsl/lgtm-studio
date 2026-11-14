// The Files tab's engine side: list one folder, read one file, and save one
// file of a registered worktree. Every path passes the same checks as Git
// reads (see git/index.ts), and nothing under .git, no credential path and no
// ignored file is listed, read or written. Saves are owner-only external
// mutations, so they run under the Git attempt and workspace lane.
import * as Effect from "effect/Effect"
import type { Event, FileSaveNotice, Workspace } from "@lgtm/contracts"
import { CommandUncertain, refused, type Input, type PiecePlanner } from "../commands/internal.ts"
import type { ReadHandler } from "../api/reads.ts"
import type { ReachInput } from "../routing/reach.ts"
import { gitCall, gitReply, safePath, withGit, workspacePath } from "./gitApi.ts"
import type { FileRead, GitFailure } from "./git/index.ts"
import { own, projectView } from "./view.ts"

// A file read may carry up to 512 KiB of text. The WebSocket frame limit is
// 1 MiB, so the reply keeps a margin for JSON escaping.
const fileReplyBudget = 960 * 1024
export const baseBranch = (workspace: Workspace) => ["main", "master"].includes(workspace.branch)
export const fileSaveSource = (noticeId: string) => `file-save:${noticeId}`

// Valid at admission and at later dispatch, where the caller is absent. The
// durable notice, its recorded agent and the exact bound delivery are the proof.
export const fileSaveReach = ({ snapshot, source, target, deliveryId }: ReachInput): boolean => {
  if (source.namespace !== "project" || !source.id.startsWith("file-save:")) return false
  const view = projectView(snapshot)
  const notice = own(view.fileSaveNotices, source.id.slice("file-save:".length))
  if (notice === undefined || fileSaveSource(notice.noticeId) !== source.id || notice.targetAgentId !== target
    || notice.deliveryId !== undefined && notice.deliveryId !== deliveryId) return false
  const project = own(view.projects, notice.projectId)
  if (project === undefined || project.archiving || project.archivedAt !== undefined) return false
  const delivery = own(snapshot.deliveries, deliveryId)
  return delivery === undefined || delivery.target === target && delivery.source?._tag === "piece"
    && delivery.source.namespace === "project" && delivery.source.id === source.id
}

const listFolder: ReadHandler = (input, raw) => withGit(input, (git) => Effect.gen(function* () {
  const workspace = yield* workspacePath(input, raw, git)
  if (raw.folder !== undefined) yield* safePath(input, raw.folder as string)
  const listing = yield* gitCall(git.listFolder(workspace.path, raw.folder as string | undefined))
  // A name that the redactor would change cannot be opened again by its
  // redacted form, so it is left out instead of shown with a placeholder.
  const items = []
  for (const item of listing.items) if ((yield* input.runtime.log.redactText(item.path)) === item.path) items.push(item)
  return { reply: yield* gitReply(input, raw.requestId, { ...listing, items, truncated: listing.truncated || items.length !== listing.items.length }) }
}))

const readFile: ReadHandler = (input, raw) => withGit(input, (git) => Effect.gen(function* () {
  const workspace = yield* workspacePath(input, raw, git)
  const path = raw.path as string
  yield* safePath(input, path)
  const file: FileRead = yield* gitCall(git.readFile(workspace.path, path))
  let result: unknown = file
  if (file.kind === "text") {
    // Text holding a registered secret is shown redacted and cannot be saved,
    // because a save would write the placeholder over the secret.
    const clean = yield* input.runtime.log.redactText(file.text)
    result = { ...file, text: clean, editable: clean === file.text && !baseBranch(workspace) }
  }
  if (Buffer.byteLength(JSON.stringify({ requestId: raw.requestId, result })) > fileReplyBudget - 4096) result = { kind: "large", path, size: file.size }
  return { reply: yield* gitReply(input, raw.requestId, result, false, fileReplyBudget) }
}))

const workingAgents = (input: Input, workspace: Workspace) => Object.values(input.snapshot.agents)
  .filter((agent) => agent.folder === workspace.path && agent.state._tag === "working")

export const writeFile: PiecePlanner = (input, raw) => withGit(input, (git) => Effect.gen(function* () {
  if (input.actor._tag !== "owner") return yield* refused()
  const workspace = yield* workspacePath(input, raw, git)
  const projectId = raw.projectId as string, path = raw.path as string, text = raw.text as string
  const project = own(projectView(input.snapshot).projects, projectId)
  if (project === undefined || project.archiving || project.archivedAt !== undefined) return yield* refused("project is unavailable to this caller")
  if (baseBranch(workspace)) return yield* refused("the base branch is read-only; agents never edit it directly")
  yield* safePath(input, path)
  if ((yield* input.runtime.log.redactText(text)) !== text) return yield* refused("the file contains private data and cannot be saved here")
  const version = yield* git.writeFile(workspace.path, path, raw.version as string, Buffer.from(text, "utf8")).pipe(Effect.mapError((error: GitFailure) =>
    error.uncertain ? new CommandUncertain({ reason: "Git outcome is uncertain; refresh Git state" })
      : refused(error.reason === "stale_state" ? "the file changed since you opened it; reload it and save again" : `Git: ${error.reason}`)))
  // An agent working in this worktree is told which file the owner changed.
  // An idle agent is not woken; it reads the file the next time it needs it.
  const working = workingAgents(input, workspace)
  const notices: FileSaveNotice[] = working.map((agent) => ({ noticeId: `${raw.requestId as string}:${agent.agentId}` as FileSaveNotice["noticeId"],
    projectId: project.projectId, worktreeId: workspace.worktreeId, path, targetAgentId: agent.agentId as FileSaveNotice["targetAgentId"] }))
  const events: Event[] = notices.map((notice) => ({ _tag: "project.FileSaveNoticePlanned", ...notice }))
  return {
    events,
    deliveries: notices.map((notice) => ({ target: notice.targetAgentId, sourceId: fileSaveSource(notice.noticeId),
      body: `The owner saved ${JSON.stringify(path)} in your worktree from the Files tab. Read the file again before you change it.` })),
    reply: () => ({ requestId: raw.requestId as string, result: { version, told: notices.length } })
  }
}))

export const fileReads = { "project.files.list": listFolder, "project.files.read": readFile } satisfies Record<string, ReadHandler>
