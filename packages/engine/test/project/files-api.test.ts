import { randomBytes, createHash } from "node:crypto"
import { execFileSync } from "node:child_process"
import { mkdirSync, readFileSync, symlinkSync, writeFileSync } from "node:fs"
import { join } from "node:path"
import * as NodeSocket from "@effect/platform-node/NodeSocket"
import { makeNodeSocket } from "@lgtm/client/nodeSocket"
import { connectWith, type Client } from "../../../client/src/core.ts"
import { Event, ProjectId } from "@lgtm/contracts"
import { it } from "@effect/vitest"
import * as Effect from "effect/Effect"
import * as Redacted from "effect/Redacted"
import * as Schema from "effect/Schema"
import { afterEach, beforeEach, expect, vi } from "vitest"
import * as Server from "../../src/api/server.ts"
import { EventLog } from "../../src/log/EventLog.ts"
import { Views } from "../../src/views/Views.ts"
import { engineLists } from "../../src/pieces/lists.ts"
import { projectView } from "../../src/project/view.ts"
import { ancestryHeaders, authorization, socketUrl } from "../support/socketClient.ts"
import { provideBoard } from "../board/wakeFixtures.ts"
import { registerRebuildCheck } from "../support/rebuildCheck.ts"
import { call, success, seedAccount, seedHarness, seedAgent, ownerCredential as owner, freshRequest, makeRig, removeRig, type Rig } from "../support/handlerRig.ts"

let rig: Rig
const git = (folder: string, ...args: string[]) => execFileSync("/usr/bin/git", ["-C", folder, ...args], { encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] }).trim()
const save = (folder: string, path: string, content: string | Buffer) => { mkdirSync(join(folder, path, ".."), { recursive: true }); writeFileSync(join(folder, path), content) }
const sha = (text: string | Buffer) => createHash("sha256").update(text).digest("hex")
beforeEach(() => {
  rig = makeRig(); vi.stubEnv("HOME", rig.home); vi.stubEnv("LGTM_HOME", rig.stateFolder)
  git(rig.work, "init", "-b", "main")
  git(rig.work, "config", "user.name", "Test Author")
  git(rig.work, "config", "user.email", "test@example.test")
  git(rig.work, "config", "commit.gpgsign", "false")
  save(rig.work, "README.md", "# Product\n")
  save(rig.work, "src/app.ts", "export const app = 1\n")
  save(rig.work, "src/deep/note.txt", "deep\n")
  save(rig.work, "logo.png", Buffer.from([0x89, 0x50, 0x4e, 0x47, 0, 1, 2]))
  save(rig.work, "data.bin", Buffer.from([1, 0, 2, 0]))
  save(rig.work, ".gitignore", "ignored.txt\n")
  git(rig.work, "add", "--all"); git(rig.work, "commit", "-m", "Initial")
})
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllEnvs(); removeRig(rig) })
const snapshot = Effect.gen(function* () { return yield* (yield* Views).snapshot })
const wire = () => Effect.gen(function* () {
  const bearer = randomBytes(32).toString("base64url"), socket = join(rig.stateFolder, "files.sock")
  yield* Server.serve({ socketPath: socket, ownerCredential: Redacted.make(bearer), debug: false })
  return yield* connectWith({ attempts: 1, makeSocket: () => makeNodeSocket(socketUrl(socket), {
    headers: { ...authorization(bearer), ...ancestryHeaders() }
  }).pipe(Effect.provide(NodeSocket.layerWebSocketConstructorWS)) })
})
const historicProject = (fields: Record<string, unknown>) => Effect.gen(function* () {
  const log = yield* EventLog, requestId = freshRequest()
  const projectId = Schema.decodeUnknownSync(ProjectId)(`project-${requestId}`)
  yield* log.append({ _tag: "owner" }, requestId, "b".repeat(64), [Schema.decodeUnknownSync(Event)({ _tag: "project.Created", ...fields, projectId })], {}, () => ({ requestId, projectId }))
  return projectId
})
// The project works on a feature worktree. The main checkout is the base.
const setup = (working = false) => Effect.gen(function* () {
  yield* registerRebuildCheck(); yield* seedAccount(); yield* seedHarness()
  const feature = join(rig.root, "feature")
  git(rig.work, "worktree", "add", "-b", "feature", feature)
  const lead = yield* seedAgent({ id: "lead", folder: feature, ...(working ? { state: "working" as const } : {}) })
  success(yield* call(owner, "project.settings.set", { settings: { workspaceFolder: rig.root, shell: "zsh" } }))
  const repoId = success(yield* call(owner, "project.repository.add", { name: "Product", path: rig.work, machineId: "local" })).repoId
  const baseId = success(yield* call(owner, "project.workspace.register", { repoId, machineId: "local", branch: "main", path: rig.work, checkout: "main" })).worktreeId
  const worktreeId = success(yield* call(owner, "project.workspace.register", { repoId, machineId: "local", branch: "feature", path: feature, checkout: "worktree" })).worktreeId
  const projectId = yield* historicProject({ name: "Files", goal: "Edit files", leadAgentId: lead.agentId, repoId, worktreeId, machineId: "local" })
  return { feature, baseId, worktreeId, projectId, lead }
})
const run = (api: Client, name: string, payload: Record<string, unknown> = {}) =>
  (api as unknown as Record<string, (p: any) => Effect.Effect<any, any>>)[name]!({ requestId: freshRequest(), ...payload })
const result = (effect: Effect.Effect<any, any>) => effect.pipe(Effect.map((reply) => { expect(reply.result).toBeDefined(); return reply.result }))
const refusal = (effect: Effect.Effect<any, any>) => effect.pipe(Effect.result, Effect.map((outcome) => {
  expect(outcome._tag).toBe("Failure"); if (outcome._tag === "Failure") return outcome.failure.reason as string
  throw new Error("expected refusal")
}))

it.live("registers the three file operations as owner reads and an owner save", () => Effect.sync(() => {
  for (const name of ["project.files.list", "project.files.read"]) { expect(Object.hasOwn(engineLists.reads, name)).toBe(true); expect(engineLists.access[name]).toEqual(["owner"]) }
  expect(Object.hasOwn(engineLists.planners, "project.files.write")).toBe(true)
  expect(engineLists.access["project.files.write"]).toEqual(["owner"])
}))

it.live("lists one folder at a time with sizes and changes, and hides ignored, credential and linked entries", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(); const api = yield* wire(); const p = { worktreeId: f.worktreeId }
  save(f.feature, "ignored.txt", "hidden\n"); save(f.feature, ".env", "TOKEN=1\n"); save(f.feature, "new.md", "new\n")
  save(f.feature, "src/app.ts", "export const app = 2\n")
  symlinkSync(join(rig.outside), join(f.feature, "outside-link"))
  const root = yield* result(run(api, "project.files.list", p))
  expect(root.folder).toBe("")
  expect(root.items.map((row: any) => `${row.kind}:${row.name}:${row.change}`)).toEqual([
    "folder:src:modified", "file:.gitignore:null", "file:data.bin:null", "file:logo.png:null", "file:new.md:untracked", "file:README.md:null"])
  expect(root.items.find((row: any) => row.name === "README.md").size).toBe(10)
  const src = yield* result(run(api, "project.files.list", { ...p, folder: "src" }))
  expect(src.items.map((row: any) => `${row.kind}:${row.path}:${row.change}`)).toEqual(["folder:src/deep:null", "file:src/app.ts:modified"])
  for (const folder of ["../outside", ".git", "outside-link", "src/app.ts", "/etc"]) expect(yield* refusal(run(api, "project.files.list", { ...p, folder })), folder).toBe("Git: invalid_path")
})).pipe(Effect.orDie))

it.live("reads text, images and binary files, and refuses paths outside the listed files", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(); const api = yield* wire(); const p = { worktreeId: f.worktreeId }
  expect(yield* result(run(api, "project.files.read", { ...p, path: "README.md" }))).toEqual({ kind: "text", path: "README.md", size: 10, version: sha("# Product\n"), text: "# Product\n", editable: true })
  expect(yield* result(run(api, "project.files.read", { ...p, path: "logo.png" }))).toMatchObject({ kind: "image", mediaType: "image/png", data: Buffer.from([0x89, 0x50, 0x4e, 0x47, 0, 1, 2]).toString("base64") })
  expect(yield* result(run(api, "project.files.read", { ...p, path: "data.bin" }))).toEqual({ kind: "binary", path: "data.bin", size: 4 })
  save(f.feature, "big.txt", "x".repeat(600 * 1024))
  expect(yield* result(run(api, "project.files.read", { ...p, path: "big.txt" }))).toEqual({ kind: "large", path: "big.txt", size: 600 * 1024 })
  // The base checkout is shown read-only.
  expect(yield* result(run(api, "project.files.read", { worktreeId: f.baseId, path: "README.md" }))).toMatchObject({ kind: "text", editable: false })
  save(f.feature, "ignored.txt", "hidden\n"); save(f.feature, ".env", "TOKEN=1\n")
  symlinkSync(join(f.feature, "README.md"), join(f.feature, "linked.md"))
  for (const path of ["ignored.txt", ".env", "linked.md", ".git/config", "../outside/x", "missing.md", "src"]) expect(yield* refusal(run(api, "project.files.read", { ...p, path })), path).toBe("Git: invalid_path")
})).pipe(Effect.orDie))

it.live("saves a file against the opened version, refuses a stale version and the base branch, and commits only that file", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(); const api = yield* wire(); const p = { worktreeId: f.worktreeId, projectId: f.projectId }
  const opened = yield* result(run(api, "project.files.read", { worktreeId: f.worktreeId, path: "README.md" }))
  const saved = yield* result(run(api, "project.files.write", { ...p, path: "README.md", version: opened.version, text: "# Product\nEdited\n" }))
  expect(saved).toEqual({ version: sha("# Product\nEdited\n"), told: 0 })
  expect(readFileSync(join(f.feature, "README.md"), "utf8")).toBe("# Product\nEdited\n")
  expect(yield* refusal(run(api, "project.files.write", { ...p, path: "README.md", version: opened.version, text: "lost\n" }))).toBe("the file changed since you opened it; reload it and save again")
  expect(readFileSync(join(f.feature, "README.md"), "utf8")).toBe("# Product\nEdited\n")
  expect(yield* refusal(run(api, "project.files.write", { ...p, worktreeId: f.baseId, path: "README.md", version: sha("# Product\n"), text: "base\n" }))).toBe("project is unavailable to this caller")
  expect(readFileSync(join(rig.work, "README.md"), "utf8")).toBe("# Product\n")
  const baseProject = yield* historicProject({ name: "On main", goal: "Base", leadAgentId: f.lead.agentId, repoId: projectView(yield* snapshot).workspaces[f.baseId]!.repoId, worktreeId: f.baseId, machineId: "local" })
  expect(yield* refusal(run(api, "project.files.write", { worktreeId: f.baseId, projectId: baseProject, path: "README.md", version: sha("# Product\n"), text: "base\n" }))).toBe("the base branch is read-only; agents never edit it directly")
  expect(readFileSync(join(rig.work, "README.md"), "utf8")).toBe("# Product\n")
  expect(yield* refusal(run(api, "project.files.write", { ...p, path: ".git/config", version: "a", text: "x" }))).toBe("Git: invalid_path")
  // Another staged file stays staged and out of the one-file commit.
  save(f.feature, "src/app.ts", "export const app = 3\n"); git(f.feature, "add", "src/app.ts")
  const head = (yield* result(run(api, "project.commit", { worktreeId: f.worktreeId, message: "Edit the readme", path: "README.md" }))).head
  expect(git(f.feature, "show", "--name-only", "--format=", head)).toBe("README.md")
  expect(git(f.feature, "diff", "--cached", "--name-only")).toBe("src/app.ts")
  expect(yield* refusal(run(api, "project.commit", { worktreeId: f.worktreeId, message: "Nothing", path: "README.md" }))).toBe("Git: nothing_staged")
})).pipe(Effect.orDie))

it.live("tells an agent working in the worktree which file the owner saved", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(true); const api = yield* wire(); const p = { worktreeId: f.worktreeId, projectId: f.projectId }
  const saved = yield* result(run(api, "project.files.write", { ...p, path: "src/app.ts", version: sha("export const app = 1\n"), text: "export const app = 4\n" }))
  expect(saved.told).toBe(1)
  const state = yield* snapshot
  const notices = Object.values(projectView(state).fileSaveNotices)
  expect(notices).toMatchObject([{ projectId: f.projectId, worktreeId: f.worktreeId, path: "src/app.ts", targetAgentId: f.lead.agentId }])
  const delivery = Object.values(state.deliveries).find((row) => row.source?._tag === "piece" && row.source.id === `file-save:${notices[0]!.noticeId}`)
  expect(delivery?.target).toBe(f.lead.agentId)
  expect(delivery?.content._tag === "prompt" && delivery.content.text).toContain('The owner saved "src/app.ts" in your worktree')
})).pipe(Effect.orDie))
