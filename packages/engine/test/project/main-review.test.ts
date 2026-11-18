import { githubCommands } from "../../src/project/git/github.ts"
import { fakeGithub } from "../support/fakeGithub.ts"
import { randomBytes } from "node:crypto"
import { execFileSync } from "node:child_process"
import { chmodSync, existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs"
import { join } from "node:path"
import { it } from "@effect/vitest"
import * as NodeSocket from "@effect/platform-node/NodeSocket"
import { makeNodeSocket } from "@lgtm/client/nodeSocket"
import { connectWith, type Client } from "../../../client/src/core.ts"
import * as Effect from "effect/Effect"
import * as Fiber from "effect/Fiber"
import * as Redacted from "effect/Redacted"
import * as SqlClient from "effect/sql/SqlClient"
import { afterEach, beforeEach, expect, vi } from "vitest"
import * as Server from "../../src/api/server.ts"
import { CommandHandler } from "../../src/commands/CommandHandler.ts"
import * as GitApi from "../../src/project/gitApi.ts"
import * as GitProcesses from "../../src/project/git/process.ts"
import { engineLists } from "../../src/pieces/lists.ts"
import { type PiecePlanner, type PiecePlan } from "../../src/commands/internal.ts"
import { reviewPlanners } from "../../src/project/gitReview.ts"
import { makeGitService } from "../../src/project/git/index.ts"
import { GitFailure } from "../../src/project/git/types.ts"
import { makeFakeRunner } from "../support/fakeRunner.ts"
import { projectView } from "../../src/project/view.ts"
import { Views } from "../../src/views/Views.ts"
import { ancestryHeaders, authorization, socketUrl, upgradeStatus } from "../support/socketClient.ts"
import { provideBoard } from "../board/wakeFixtures.ts"
import { registerRebuildCheck } from "../support/rebuildCheck.ts"
import { appendSetup, call, eventOf, freshRequest, harnessSettings, makeRig, ownerCredential as owner, readLog,
  removeRig, seedAccount, seedAgent, success, provideRig, makeFakeSecrets, type Fakes, type Rig } from "../support/handlerRig.ts"

let rig: Rig
const git = (path: string, ...args: string[]) => execFileSync("/usr/bin/git", ["-C", path, ...args], { encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] }).trim()
// Observe only the process boundary. Every operation still executes real Git.
const observeMerges = () => {
  const bin = join(rig.root, "observed-git"), calls = join(rig.root, "observed-merges")
  mkdirSync(bin)
  writeFileSync(join(bin, "git"), `#!${process.execPath}\nimport { appendFileSync } from 'node:fs';\nimport { spawnSync } from 'node:child_process';\nconst args=process.argv.slice(2);\nif(args.includes('merge')) appendFileSync(${JSON.stringify(calls)}, 'merge\\n');\nconst result=spawnSync('/usr/bin/git',args,{stdio:'inherit'});process.exit(result.status??1);\n`, { mode: 0o700 })
  vi.stubEnv("PATH", `${bin}:${process.env.PATH}`)
  return () => existsSync(calls) ? readFileSync(calls, "utf8").trim().split("\n").length : 0
}
const noGitReceipt = (requestId: string) => Effect.gen(function* () {
  const sql = yield* SqlClient.SqlClient
  expect(yield* sql`SELECT request_id FROM receipts WHERE request_id = ${requestId}`).toHaveLength(0)
  expect(Object.values(projectView(yield* snapshot).gitAttempts).some((attempt) => attempt.requestId === requestId)).toBe(true)
  expect((yield* readLog).filter((row) => row.requestId === requestId && ["CommandRefused", "project.MainReviewDecided", "project.GitFinished"].includes(row.event._tag))).toEqual([])
})
const save = (path: string, name: string, text: string) => writeFileSync(join(path, name), text)
const commit = (path: string, title: string) => { git(path, "add", "--all"); git(path, "commit", "-m", title); return git(path, "rev-parse", "HEAD") }
const snapshot = Effect.gen(function* () { return yield* (yield* Views).snapshot })
const client = (socket: string, token: string) => connectWith({ attempts: 1, makeSocket: () => makeNodeSocket(socketUrl(socket), {
  headers: { ...authorization(token), ...ancestryHeaders() }
}).pipe(Effect.provide(NodeSocket.layerWebSocketConstructorWS)) })
const run = (api: Client, name: string, payload: Record<string, unknown> = {}) =>
  (api as unknown as Record<string, (p: any) => Effect.Effect<any, any>>)[name]!({ requestId: freshRequest(), ...payload })
const refusal = (effect: Effect.Effect<any, any>) => effect.pipe(Effect.result, Effect.map((r) => {
  expect(r._tag).toBe("Failure"); if (r._tag !== "Failure") throw new Error("expected refusal"); return r.failure.reason as string
}))
beforeEach(() => {
  rig = makeRig(); vi.stubEnv("HOME", rig.home); vi.stubEnv("LGTM_HOME", rig.stateFolder)
  git(rig.work, "init", "-b", "main"); git(rig.work, "config", "user.name", "Review fixture")
  git(rig.work, "config", "user.email", "review@example.invalid"); git(rig.work, "config", "commit.gpgsign", "false")
  save(rig.work, "file.txt", "base\n"); commit(rig.work, "Initial")
})
afterEach(() => { vi.unstubAllEnvs(); removeRig(rig) })
const setup = (base = "main") => Effect.gen(function* () {
  yield* registerRebuildCheck()
  if (base !== "main") git(rig.work, "branch", "-m", base)
  yield* seedAccount("launch", "cursor")
  success(yield* call(owner, "harnesses", { operation: "set", harness: harnessSettings("cursor", { adapterPath: process.execPath, reportedModes: [{ id: "default", level: "ask" }] }) }))
  success(yield* call(owner, "accounts set-secret", { accountId: "launch", name: "CURSOR_API_KEY", nonce: "review-fixture", value: Redacted.make("synthetic-review-fixture-key") }))
  const socket = join(rig.stateFolder, "review.sock"), token = randomBytes(32).toString("base64url")
  yield* Server.serve({ socketPath: socket, ownerCredential: Redacted.make(token), debug: false })
  expect((yield* Effect.promise(() => upgradeStatus(socket, ancestryHeaders()))).status).toBe(401)
  const api = yield* client(socket, token)
  yield* run(api, "project.settings.set", { settings: { workspaceFolder: rig.root, shell: "zsh" } })
  const { repoId } = yield* run(api, "project.repository.add", { name: "Product", path: rig.work, machineId: "local" })
  const { worktreeId: destinationId } = yield* run(api, "project.workspace.register", { repoId, path: rig.work, branch: base, checkout: "main", machineId: "local" })
  // The real lifecycle command creates and records the project/worktree. Only
  // its external harness is fake; Git, SQL, redactor, handler and wire are real.
  const { projectId } = yield* run(api, "project.create", { name: "Product work", goal: "Review actual changes", machineId: "local", repository: { _tag: "existing", repoId }, useWorktree: true })
  const view = projectView(yield* snapshot), project = view.projects[projectId]!, workspace = view.workspaces[project.worktreeId]!
  save(workspace.path, "feature.txt", "actual feature\n"); const head = commit(workspace.path, "Feature /tool <script>literal</script>")
  const member = yield* seedAgent({ id: "reviewer", parent: project.leadAgentId, folder: workspace.path })
  // A durable item from the fake harness supplies a genuine thread location.
  const [thread] = yield* appendSetup([eventOf({ _tag: "ThreadItem", agentId: member.agentId, itemId: "report-item", content: { _tag: "text", text: "Actual reported review, not engine-run tests" }, replay: false })])
  return { api, socket, repoId, projectId, worktreeId: workspace.worktreeId, destinationId, source: workspace.path,
    base, head, member, threadSequence: thread!.sequence, baseHead: git(rig.work, "rev-parse", "HEAD") }
})
const prepare = (f: Effect.Success<ReturnType<typeof setup>>) => run(f.api, "project.review.prepare", { projectId: f.projectId, worktreeId: f.worktreeId, destinationId: f.destinationId })
const approve = (f: Effect.Success<ReturnType<typeof setup>>, review: any, extra: Record<string, unknown> = {}) => run(f.api, "project.review.approve", {
  projectId: f.projectId, worktreeId: f.worktreeId, reviewId: review.reviewId, expectedHead: review.head, expectedBaseHead: review.baseHead,
  confirmed: true, decision: "approve_without_evidence", ...extra,
})
const report = (f: Effect.Success<ReturnType<typeof setup>>, api = f.api, extra: Record<string, unknown> = {}) => run(api, "project.review.report", {
  projectId: f.projectId, worktreeId: f.worktreeId, head: git(f.source, "rev-parse", "HEAD"), kind: "review", status: "failed",
  summary: "Reported failure /tool <script>literal</script>", threadAgentId: f.member.agentId, threadSequence: f.threadSequence, ...extra,
})

for (const base of ["main", "master"]) it.live(`owner explicitly reviews and merges ${base} with missing or failed evidence; identical retry merges once`, () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(base)
  if (base === "master") yield* report(f)
  const { result: opened } = yield* prepare(f), review = opened.review
  expect(opened.comparison).toMatchObject({ head: f.head, baseHead: f.baseHead, base, files: { items: [{ path: "feature.txt" }], truncated: false } })
  expect(opened.comparison.diff.patch).toContain("+actual feature")
  expect(opened.comparison.commits.items[0].title).toContain("<script>literal</script>")
  expect(review.evidence.review).toMatchObject(base === "main" ? { state: "missing", report: null } : { state: "current", report: { status: "failed", provenance: "reported", threadAgentId: f.member.agentId, threadSequence: f.threadSequence } })
  expect(git(rig.work, "rev-parse", "HEAD")).toBe(f.baseHead)
  expect(existsSync(join(rig.work, "feature.txt"))).toBe(false)
  expect(yield* refusal(approve(f, review, { confirmed: false }))).toBe("Git: confirmation_required")
  const requestId = freshRequest(), payload = { requestId, projectId: f.projectId, worktreeId: f.worktreeId,
    reviewId: review.reviewId, expectedHead: f.head, expectedBaseHead: f.baseHead, confirmed: true, decision: "approve_without_evidence" }
  const merged = yield* run(f.api, "project.review.approve", payload)
  expect(merged.result).toMatchObject({ outcome: "merged", head: f.head, baseHead: f.baseHead, mergeHead: f.head, decision: "approve_without_evidence" })
  expect(git(rig.work, "rev-parse", "HEAD")).toBe(f.head)
  expect(readFileSync(join(rig.work, "feature.txt"), "utf8")).toBe("actual feature\n")
  expect(yield* run(f.api, "project.review.approve", payload)).toEqual(merged)
  expect(yield* refusal(run(f.api, "project.review.approve", { ...payload, decision: "approve" }))).toBe("request id reused")
  expect((yield* readLog).filter(({ event }) => event._tag === "project.MainReviewDecided")).toHaveLength(1)
  expect(yield* refusal(approve(f, review))).toBe("main review is unavailable; prepare a new review")
  expect((yield* run(f.api, "project.review.status", f)).result.review).toMatchObject({ outcome: "merged", head: f.head, baseHead: f.baseHead })
})).pipe(Effect.orDie))

it.live("reports actual head-bound provenance, rejects foreign or missing threads, and marks old reports stale", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), member = yield* client(f.socket, f.member.token)
  yield* report(f, member, { status: "passed", summary: "Reported reviewer success" })
  yield* report(f, f.api, { kind: "tests", status: "passed", summary: "Reported tests succeeded" })
  const before = (yield* prepare(f)).result
  expect(before.review.evidence.tests.report).toMatchObject({ reportedBy: { _tag: "owner" }, provenance: "reported", status: "passed" })
  expect(before.review.evidence.review.report).toMatchObject({ reportedBy: { _tag: "agent", agentId: f.member.agentId }, status: "passed" })
  expect((yield* run(member, "project.review.status", f)).data).toContain("Reported reviewer success")
  expect(yield* refusal(report(f, member, { threadSequence: 999999 }))).toBe("review report requires an existing project thread item")
  const stranger = yield* seedAgent({ id: "stranger" })
  expect(yield* refusal(report(f, member, { threadAgentId: stranger.agentId }))).toBe("review report requires the caller's project thread")
  expect(yield* refusal(report(f, f.api, { threadAgentId: stranger.agentId }))).toBe("review report requires an existing project thread item")
  expect(yield* refusal(run(yield* client(f.socket, stranger.token), "project.review.status", f))).toBe("project is unavailable to this caller")
  save(f.source, "later", "new head\n"); commit(f.source, "After evidence")
  const after = (yield* prepare(f)).result.review
  expect(after.evidence.tests.state).toBe("stale"); expect(after.evidence.review.state).toBe("stale")
  expect(yield* refusal(report(f, member, { head: f.head }))).toBe("Git: stale_state")
  expect(yield* refusal(approve(f, before.review))).toBe("main review is unavailable; prepare a new review")
  expect((yield* approve(f, after, { decision: "approve" })).result).toMatchObject({ outcome: "merged", decision: "approve" })
})).pipe(Effect.orDie))

for (const side of ["source", "destination"] as const) it.live(`refuses changed reviewed ${side} head and requires a new owner review`, () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), { review } = (yield* prepare(f)).result
  const folder = side === "source" ? f.source : rig.work
  save(folder, "later", "later\n"); const later = commit(folder, "Head changed")
  const result = (yield* approve(f, review)).result
  expect(result).toMatchObject({ outcome: "refused", reason: "stale_state", head: f.head, baseHead: f.baseHead, mergeHead: null })
  expect(git(rig.work, "rev-parse", "HEAD")).toBe(side === "source" ? f.baseHead : later)
  expect(yield* refusal(approve(f, review))).toBe("main review is unavailable; prepare a new review")
  const fresh = (yield* prepare(f)).result.review
  expect(fresh.head).toBe(git(f.source, "rev-parse", "HEAD")); expect(fresh.baseHead).toBe(git(rig.work, "rev-parse", "HEAD"))
  expect((yield* approve(f, fresh)).result.outcome).toBe("merged")
})).pipe(Effect.orDie))

for (const side of ["source", "destination"] as const) it.live(`preserves a dirty ${side} and persists an honest refused approval outcome`, () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), { review } = (yield* prepare(f)).result
  const folder = side === "source" ? f.source : rig.work
  save(folder, "dirty", "keep these exact bytes\n")
  const requestId = freshRequest()
  const attempt = yield* approve(f, review, { requestId })
  expect(attempt.result).toMatchObject({ outcome: "refused", reason: `dirty_${side}`, mergeHead: null })
  expect(readFileSync(join(folder, "dirty"), "utf8")).toBe("keep these exact bytes\n")
  expect(git(rig.work, "rev-parse", "HEAD")).toBe(f.baseHead)
  rmSync(join(folder, "dirty"))
  expect(yield* approve(f, review, { requestId })).toEqual(attempt)
  expect((yield* approve(f, (yield* prepare(f)).result.review)).result.outcome).toBe("merged")
})).pipe(Effect.orDie))

it.live("denies agents, service/caller-less execution, missing approval and unrelated registered destinations", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), member = yield* client(f.socket, f.member.token), { review } = (yield* prepare(f)).result
  expect(yield* refusal(run(member, "project.review.prepare", f))).toBe("command refused")
  const payload = { projectId: f.projectId, worktreeId: f.worktreeId, reviewId: review.reviewId, expectedHead: f.head, expectedBaseHead: f.baseHead, confirmed: true, decision: "approve_without_evidence", actor: { _tag: "owner" } }
  expect(yield* refusal(run(member, "project.review.approve", payload))).toBe("command refused")
  expect(yield* refusal(run(f.api, "project.integrate", { worktreeId: f.worktreeId, destinationId: f.destinationId, expectedHead: f.head }))).toBe("integration requires a registered team source and its project workspace")
  const service = makeGitService({ workspaceFolder: rig.root })
  const reviewed = (yield* service.reviewMain(f.source, rig.work))
  for (const caller of [undefined, { _tag: "agent", agentId: f.member.agentId } as const, { _tag: "piece", namespace: "project" } as const]) {
    expect(yield* refusal(service.approveMain(f.source, rig.work, reviewed, caller, true))).toBe("owner_required")
  }
  expect(yield* refusal(service.mergeTeam(f.source, rig.work, f.head))).toBe("base_requires_review")
  const handler = yield* CommandHandler
  // The typed client rejects a missing field before transport. Exercise the
  // same malformed input at the actual production command admission boundary.
  expect(yield* refusal(handler.execute(owner, "project.review.approve", { requestId: freshRequest(), ...payload, confirmed: undefined }))).toBe("command refused")
  expect(yield* refusal(handler.execute({ _tag: "service", service: "test.board" }, "project.review.approve", { requestId: freshRequest(), ...payload }))).toBe("command refused")
  const pieceInput = yield* handler.input({ _tag: "piece", namespace: "project" })
  expect(yield* refusal(reviewPlanners["project.review.approve"](pieceInput, { requestId: freshRequest(), ...payload }))).toBe("Git: owner_required")
  const other = join(rig.root, "other"); mkdirSync(other); git(other, "init", "-b", "main")
  git(other, "config", "user.name", "Other"); git(other, "config", "user.email", "other@example.invalid"); git(other, "config", "commit.gpgsign", "false")
  save(other, "own", "other\n"); commit(other, "Other initial")
  const { repoId } = yield* run(f.api, "project.repository.add", { name: "Other", machineId: "local", path: other })
  const { worktreeId: otherId } = yield* run(f.api, "project.workspace.register", { repoId, machineId: "local", path: other, branch: "main", checkout: "main" })
  expect(yield* refusal(prepare({ ...f, destinationId: otherId }))).toBe("main review requires the registered project and its repository base")
  expect(yield* refusal(prepare({ ...f, worktreeId: f.destinationId }))).toBe("project is unavailable to this caller")
  const invalidId = freshRequest(), invalid = { ...f, projectId: "different-project" }
  expect(yield* refusal(approve(invalid, review, { requestId: invalidId }))).toBe("main review is unavailable; prepare a new review")
  expect(yield* refusal(approve(invalid, review, { requestId: invalidId }))).toBe("main review is unavailable; prepare a new review")
  const sql = yield* SqlClient.SqlClient
  expect(yield* sql`SELECT request_id FROM receipts WHERE request_id = ${invalidId}`).toHaveLength(1)
  expect((yield* readLog).filter((row) => row.requestId === invalidId).map((row) => row.event)).toEqual([
    { _tag: "CommandRefused", command: "project.review.approve", reason: "main review is unavailable; prepare a new review" }
  ])
  expect(Object.values(projectView(yield* snapshot).gitAttempts)).toEqual([])
  expect(git(rig.work, "rev-parse", "HEAD")).toBe(f.baseHead)
})).pipe(Effect.orDie))

it.live("issue 590: records a proven conflicting owner merge as refused, replays it and permits a fresh review after resolution", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup()
  save(f.source, "file.txt", "theirs\n"); commit(f.source, "Project edit")
  save(rig.work, "file.txt", "ours\n"); commit(rig.work, "Base edit")
  const { review } = (yield* prepare(f)).result
  const mergeCalls = observeMerges(), requestId = freshRequest()
  const rejected = yield* approve(f, review, { requestId })
  expect(rejected.result).toMatchObject({ outcome: "refused", reason: "conflicts" })
  expect(git(rig.work, "rev-parse", "MERGE_HEAD")).toBe(review.head)
  const unresolved = readFileSync(join(rig.work, "file.txt"), "utf8")
  expect(unresolved).toContain("<<<<<<<"); expect(unresolved).toContain("ours\n"); expect(unresolved).toContain("theirs\n")
  expect(projectView(yield* snapshot).gitReviews[f.worktreeId]).toMatchObject({ outcome: "refused", reason: "conflicts" })
  expect(Object.values(projectView(yield* snapshot).gitAttempts)).toHaveLength(0)
  expect(yield* approve(f, review, { requestId })).toEqual(rejected)
  expect(mergeCalls()).toBe(1)
  expect(readFileSync(join(rig.work, "file.txt"), "utf8")).toBe(unresolved)
  expect(Object.values(projectView(yield* snapshot).gitAttempts)).toHaveLength(0)
  // The ordinary service admission still refuses a new merge before effects.
  const service = makeGitService({ workspaceFolder: rig.root })
  expect(yield* refusal(service.approveMain(f.source, rig.work, review, { _tag: "owner" }, true))).toBe("dirty_destination")
  expect(mergeCalls()).toBe(1)
  const p = { worktreeId: f.destinationId, path: "file.txt" }
  const conflict = (yield* run(f.api, "project.conflict", p)).result
  expect(conflict).toMatchObject({ ours: "ours\n", base: "base\n", theirs: "theirs\n" })
  const chosen = (yield* run(f.api, "project.conflict.choose", { worktreeId: f.destinationId, selection: conflict, hunk: 0, choice: "both" })).result
  yield* run(f.api, "project.conflict.resolve", { worktreeId: f.destinationId, selection: chosen })
  expect(readFileSync(join(rig.work, "file.txt"), "utf8")).toBe("ours\ntheirs\n")
  yield* run(f.api, "project.stage.all", { worktreeId: f.destinationId })
  yield* run(f.api, "project.commit", { worktreeId: f.destinationId, message: "Resolve the reviewed merge" })
  const actual = git(rig.work, "rev-parse", "HEAD")
  expect(git(rig.work, "merge-base", "--is-ancestor", review.head, actual)).toBe("")
  expect(git(rig.work, "merge-base", "--is-ancestor", review.baseHead, actual)).toBe("")
  expect((yield* run(f.api, "project.git.state", { worktreeId: f.destinationId })).result.mergeHead).toBe(null)
  expect(yield* approve(f, review, { requestId })).toEqual(rejected)
  const freshReview = (yield* prepare(f)).result.review
  const reconciled = yield* approve(f, freshReview)
  expect(reconciled.result).toMatchObject({ outcome: "merged", mergeHead: actual })
  expect(yield* approve(f, review, { requestId })).toEqual(rejected)
  expect(mergeCalls()).toBe(1)
  expect((yield* readLog).filter((row) => row.requestId === requestId && row.event._tag === "project.MainReviewDecided")).toHaveLength(1)
})).pipe(Effect.orDie))

for (const hosted of [false, true]) it.live(`refuses partial or binary review data instead of issuing an all-files approval ticket (${hosted})`, () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup()
  const gh = hosted ? fakeGithub(rig.root, rig.work) : undefined
  if (gh) vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  save(f.source, "large", "line\n".repeat(35000)); commit(f.source, "Large patch")
  expect(yield* refusal(prepare(f))).toBe("Git: unsupported_review")
  expect(Object.values(projectView(yield* snapshot).gitAttempts)).toHaveLength(0)
  expect((yield* run(f.api, "project.review.status", f)).result.review).toBe(null)
  git(f.source, "reset", "--hard", f.head)
  save(f.source, "binary", "\0binary"); commit(f.source, "Binary change")
  expect(yield* refusal(prepare(f))).toBe("Git: unsupported_review")
  expect(Object.values(projectView(yield* snapshot).gitAttempts)).toHaveLength(0)
  if (gh) expect(readFileSync(gh.gitCalls, "utf8")).not.toContain('"push"')
  git(f.source, "reset", "--hard", f.head)
  for (let i = 0; i < 201; i++) save(f.source, `file-${i}`, "bounded file list\n")
  commit(f.source, "Too many files for complete review")
  expect(yield* refusal(prepare(f))).toBe("Git: unsupported_review")
  expect((yield* run(f.api, "project.review.status", f)).result.review).toBe(null)
  git(f.source, "reset", "--hard", f.head)
  expect((yield* prepare(f)).result.review.outcome).toBe("open")
})).pipe(Effect.orDie))

it.live("redacts reports, comparison prose and durable outcomes through the production redactor while preserving protocol enums", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup()
  save(f.source, "secret.txt", "git-review-secret\n"); commit(f.source, "Literal git-review-secret /tool")
  yield* report(f, f.api, { summary: "failed current reported git-review-secret", status: "failed" })
  const opened = yield* prepare(f)
  expect(JSON.stringify(opened)).not.toContain("git-review-secret")
  expect(opened.result.comparison.diff.patch).toContain("[REDACTED]")
  expect(opened.result.review.evidence.review).toMatchObject({ state: "current", report: { status: "failed", provenance: "reported", summary: "[REDACTED] [REDACTED] [REDACTED] [REDACTED]" } })
  const merged = yield* approve(f, opened.result.review)
  expect(merged.result.outcome).toBe("merged")
  const sql = yield* SqlClient.SqlClient
  const events = yield* sql<{ payload_json: string }>`SELECT payload_json FROM events`
  const receipts = yield* sql<{ result_json: string }>`SELECT result_json FROM receipts`
  expect(JSON.stringify(events)).not.toContain("git-review-secret"); expect(JSON.stringify(receipts)).not.toContain("git-review-secret")
  expect(readFileSync(join(rig.work, "secret.txt"), "utf8")).toBe("git-review-secret\n")
}), ["git-review-secret", "failed", "current", "reported"]).pipe(Effect.orDie))

for (const base of ["main", "master"]) for (const [options, divergent] of [
  ["--squash", false],
  ["--no-ff --no-commit", true],
  ["--squash --no-commit --ff-only -s ours", true],
] as const) it.live(`owner merge into ${base} ignores merge defaults ${options} and records actual ancestry`, () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(base)
  if (divergent) { save(rig.work, "base-only.txt", "independent base work\n"); commit(rig.work, "Base work") }
  git(rig.work, "config", `branch.${base}.mergeOptions`, options)
  git(rig.work, "config", "merge.ff", "only")
  git(rig.work, "config", "merge.twohead", "ours")
  git(rig.work, "config", "merge.autoStash", "true")
  const { review } = (yield* prepare(f)).result
  const member = yield* client(f.socket, f.member.token)
  expect(yield* refusal(run(member, "project.review.approve", { ...f, reviewId: review.reviewId,
    expectedHead: review.head, expectedBaseHead: review.baseHead, confirmed: true,
    decision: "approve_without_evidence", actor: { _tag: "owner" } }))).toBe("command refused")
  const requestId = freshRequest(), merged = yield* approve(f, review, { requestId })
  const actual = git(rig.work, "rev-parse", `refs/heads/${base}`)
  expect(merged.result).toMatchObject({ outcome: "merged", mergeHead: actual, head: f.head, baseHead: review.baseHead })
  expect(git(rig.work, "merge-base", "--is-ancestor", f.head, actual)).toBe("")
  expect(git(rig.work, "status", "--porcelain=v1")).toBe("")
  expect(existsSync(join(rig.work, ".git", "MERGE_HEAD"))).toBe(false)
  expect(readFileSync(join(rig.work, "feature.txt"), "utf8")).toBe("actual feature\n")
  if (divergent) {
    expect(git(rig.work, "rev-list", "--parents", "-n", "1", actual).split(" ").slice(1)).toEqual([review.baseHead, f.head])
    expect(readFileSync(join(rig.work, "base-only.txt"), "utf8")).toBe("independent base work\n")
  } else expect(actual).toBe(f.head)
  expect(yield* approve(f, review, { requestId })).toEqual(merged)
  expect(git(rig.work, "rev-parse", "HEAD")).toBe(actual)
  expect((yield* readLog).filter(({ event }) => event._tag === "project.MainReviewDecided")).toHaveLength(1)
  // Only this invocation overrides defaults; the user's settings are preserved.
  expect(git(rig.work, "config", `branch.${base}.mergeOptions`)).toBe(options)
})).pipe(Effect.orDie))

for (const base of ["main", "master"]) it.live(`already-contained reviewed commit in ${base} is a verified no-op`, () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(base)
  git(rig.work, "merge", "--ff-only", f.head)
  save(rig.work, "later-base.txt", "later base work\n"); const contained = commit(rig.work, "After integration")
  const { review } = (yield* prepare(f)).result
  expect(review.head).toBe(f.head); expect(review.baseHead).toBe(contained)
  const requestId = freshRequest(), result = yield* approve(f, review, { requestId })
  expect(result.result).toMatchObject({ outcome: "merged", mergeHead: contained })
  expect(git(rig.work, "rev-parse", "HEAD")).toBe(contained)
  expect(git(rig.work, "merge-base", "--is-ancestor", f.head, contained)).toBe("")
  expect(git(rig.work, "status", "--porcelain=v1")).toBe("")
  expect(readFileSync(join(rig.work, "feature.txt"), "utf8")).toBe("actual feature\n")
  expect(readFileSync(join(rig.work, "later-base.txt"), "utf8")).toBe("later base work\n")
  expect(yield* approve(f, review, { requestId })).toEqual(result)
  expect((yield* readLog).filter(({ event }) => event._tag === "project.MainReviewDecided")).toHaveLength(1)
})).pipe(Effect.orDie))

for (const mode of ["noop", "squash", "changed-head"]) it.live(`zero-exit ${mode} without reviewed ancestry retains uncertainty and actual state`, () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), { review } = (yield* prepare(f)).result
  const bin = join(rig.root, "merge-probe-bin"), calls = join(rig.root, "merge-calls")
  mkdirSync(bin)
  // Inject the process boundary only for merge. Other commands, including
  // postcondition reads, use actual Git. Squash leaves genuine staged bytes;
  // the changed-head case creates a real commit without the reviewed source.
  writeFileSync(join(bin, "git"), `#!${process.execPath}\nimport { appendFileSync } from 'node:fs';\nimport { spawnSync } from 'node:child_process';\nconst args=process.argv.slice(2);\nif(args.includes('merge')) {\n appendFileSync(${JSON.stringify(calls)}, JSON.stringify(args)+'\\n');\n const mode=${JSON.stringify(mode)};\n if(mode==='noop') process.exit(0);\n const changed=mode==='squash' ? ['merge','--squash',args.at(-1)] : ['commit','--allow-empty','-m','Unrelated destination change'];\n const r=spawnSync('/usr/bin/git',['-c','core.hooksPath=/dev/null',...changed],{stdio:'inherit'}); process.exit(r.status ?? 1);\n}\nconst r=spawnSync('/usr/bin/git',args,{stdio:'inherit'}); process.exit(r.status ?? 1);\n`, { mode: 0o700 })
  vi.stubEnv("PATH", `${bin}:${process.env.PATH}`)
  const requestId = freshRequest(), result = yield* refusal(approve(f, review, { requestId }))
  const actual = git(rig.work, "rev-parse", "HEAD"), status = git(rig.work, "status", "--porcelain=v1")
  expect(result).toBe("Git outcome is uncertain; refresh Git state")
  expect(git(rig.work, "rev-list", actual).split("\n")).not.toContain(f.head)
  expect(actual === f.baseHead).toBe(mode !== "changed-head")
  if (mode === "squash") {
    expect(status).toBe("A  feature.txt")
    expect(readFileSync(join(rig.work, "feature.txt"), "utf8")).toBe("actual feature\n")
  } else { expect(status).toBe(""); expect(existsSync(join(rig.work, "feature.txt"))).toBe(false) }
  expect(yield* refusal(approve(f, review, { requestId }))).toBe(result)
  expect(readFileSync(calls, "utf8").trim().split("\n")).toHaveLength(1)
  expect(git(rig.work, "rev-parse", "HEAD")).toBe(actual)
  expect(git(rig.work, "status", "--porcelain=v1")).toBe(status)
  const decided = (yield* readLog).filter(({ event }) => event._tag === "project.MainReviewDecided")
  expect(decided).toHaveLength(0)
  expect(Object.values(projectView(yield* snapshot).gitAttempts)).toHaveLength(1)
  yield* noGitReceipt(requestId)
  const sql = yield* SqlClient.SqlClient
  const receipts = yield* sql<{ result_json: string }>`SELECT result_json FROM receipts WHERE request_id = ${requestId}`
  expect(receipts).toHaveLength(0)
  const wrongBody = yield* refusal(approve(f, review, { requestId, confirmed: false }))
  expect(wrongBody).toBe("request id reused")
  expect(yield* refusal(run(f.api, "project.preferences.set", { requestId, projectId: f.projectId, shell: "bash" }))).toBe("request id reused")
  expect(readFileSync(calls, "utf8").trim().split("\n")).toHaveLength(1)
  // A real state refresh clears the bulky attempt but reserves its identity as
  // terminal uncertainty. It neither decides the open review nor runs Git again.
  const openReview = projectView(yield* snapshot).gitReviews[f.worktreeId]
  yield* run(f.api, "project.git.state", { worktreeId: f.worktreeId })
  expect(Object.values(projectView(yield* snapshot).gitAttempts)).toHaveLength(0)
  expect(projectView(yield* snapshot).gitReviews[f.worktreeId]).toEqual(openReview)
  expect(yield* refusal(approve(f, review, { requestId }))).toBe(result)
  expect(yield* refusal(approve(f, review, { requestId, confirmed: false }))).toBe("request id reused")
  expect(yield* refusal(run(f.api, "project.preferences.set", { requestId, projectId: f.projectId, shell: "bash" }))).toBe("request id reused")
  const finished = (yield* readLog).filter((row) => row.requestId === requestId).map((row) => row.event)
  expect(finished.filter((event) => event._tag === "project.GitFinished")).toMatchObject([{ outcome: "uncertain" }])
  expect(finished.filter((event) => event._tag === "CommandRefused")).toEqual([])
  expect(yield* sql`SELECT result_json FROM receipts WHERE caller = 'owner' AND request_id = ${requestId}`).toHaveLength(1)
  expect(readFileSync(calls, "utf8").trim().split("\n")).toHaveLength(1)
})).pipe(Effect.orDie))

for (const withEvidence of [false, true]) it.live(`review status and owner approval complete while overlapped, recorded evidence=${withEvidence}`, () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup()
  if (withEvidence) yield* report(f)
  const statusPayload = { projectId: f.projectId, worktreeId: f.worktreeId }
  const expectedEvidence = withEvidence ? "current" : "missing"
  expect((yield* run(f.api, "project.review.status", statusPayload)).result.evidence.review.state).toBe(expectedEvidence)
  const { review } = (yield* prepare(f)).result
  const sql = yield* SqlClient.SqlClient
  const bin = join(rig.root, "overlap-bin"), hold = join(rig.root, "hold-git")
  const entered = join(rig.root, "git-entered"), writer = join(rig.root, "writer-entered")
  mkdirSync(bin); writeFileSync(hold, "hold")
  // The real child blocks inside status's Git gate. All commands still run
  // actual Git; the wrapper only establishes the otherwise racy ordering.
  writeFileSync(join(bin, "git"), `#!${process.execPath}\nimport { existsSync, writeFileSync } from 'node:fs';\nimport { spawnSync } from 'node:child_process';\nconst args = process.argv.slice(2);\nif (args.includes('rev-parse') && existsSync(${JSON.stringify(hold)})) {\n writeFileSync(${JSON.stringify(entered)}, JSON.stringify({ pid: process.pid, args }));\n while (existsSync(${JSON.stringify(hold)})) await new Promise(resolve => setTimeout(resolve, 10));\n}\nconst result = spawnSync('/usr/bin/git', args, { stdio: 'inherit' });\nprocess.exit(result.status ?? 1);\n`, { mode: 0o700 })
  vi.stubEnv("PATH", `${bin}:${process.env.PATH}`)
  let armed = false, statusDone = false, approvalDone = false
  let statusReply: any, approvalReply: any
  const transaction = sql.withTransaction.bind(sql)
  // SqlClient runs the body only after acquiring the actual connection and
  // executing BEGIN. This observes ownership; it does not supply a fake lock.
  const observer = vi.spyOn(sql, "withTransaction").mockImplementation((effect) => transaction(Effect.gen(function* () {
    if (armed) {
      const active = yield* Effect.serviceOption(sql.transactionService)
      expect(active._tag).toBe("Some")
      writeFileSync(writer, "SQL connection owned after BEGIN")
    }
    return yield* effect
  })))
  const waitFor = (ready: () => boolean) => Effect.gen(function* () {
    for (let attempt = 0; attempt < 1000 && !ready(); attempt++) yield* Effect.sleep(10)
    expect(ready()).toBe(true)
  })
  try {
    yield* run(f.api, "project.review.status", statusPayload).pipe(
      Effect.tap((reply) => Effect.sync(() => { statusReply = reply; statusDone = true })), Effect.forkChild)
    yield* waitFor(() => existsSync(entered))
    expect(statusDone).toBe(false)
    const child = JSON.parse(readFileSync(entered, "utf8"))
    expect(child.args).toContain("rev-parse"); expect(child.pid).toBeGreaterThan(0)
    armed = true
    const requestId = freshRequest()
    yield* approve(f, review, { requestId }).pipe(
      Effect.tap((reply) => Effect.sync(() => { approvalReply = reply; approvalDone = true })), Effect.forkChild)
    yield* waitFor(() => existsSync(writer))
    expect(readFileSync(writer, "utf8")).toBe("SQL connection owned after BEGIN")
    expect(approvalDone).toBe(false); expect(statusDone).toBe(false)
    rmSync(hold)
    yield* waitFor(() => statusDone && approvalDone)
    expect(statusReply.result.evidence.review.state).toBe(expectedEvidence)
    if (withEvidence) expect(statusReply.result.evidence.review.report).toMatchObject({
      head: f.head, provenance: "reported", status: "failed", threadAgentId: f.member.agentId, threadSequence: f.threadSequence,
    })
    const actual = git(rig.work, "rev-parse", "HEAD")
    expect(approvalReply.result).toMatchObject({ outcome: "merged", head: f.head, baseHead: f.baseHead, mergeHead: actual })
    expect(git(rig.work, "merge-base", "--is-ancestor", review.head, actual)).toBe("")
    expect(readFileSync(join(rig.work, "feature.txt"), "utf8")).toBe("actual feature\n")
    expect(git(rig.work, "status", "--porcelain=v1")).toBe("")
    expect(yield* approve(f, review, { requestId })).toEqual(approvalReply)
    const events = (yield* readLog).filter(({ event }) => event._tag === "project.MainReviewDecided")
    expect(events).toHaveLength(1)
    expect(events[0]!.event).toMatchObject({ review: approvalReply.result })
    const receipts = yield* sql<{ result_json: string }>`SELECT result_json FROM receipts WHERE request_id = ${requestId}`
    expect(receipts).toHaveLength(1)
    expect(JSON.parse(receipts[0]!.result_json).reply).toEqual(approvalReply)
    expect((yield* run(f.api, "project.review.status", statusPayload)).result.review).toEqual(approvalReply.result)
    // setup's finalizer also rebuilds the live projections from durable events.
  } finally {
    rmSync(hold, { force: true })
    observer.mockRestore()
  }
})).pipe(Effect.orDie))


it.live("issue 575: an unrecorded real merge reconciles the reviewed ancestry before rejecting the changed base", () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  const f = yield* setup(), { review } = (yield* prepare(f)).result
  // This is the exact durable state a crash after Git and before append leaves:
  // a real merge, an open persisted review, and no approval receipt.
  git(rig.work, "merge", "--no-edit", review.head)
  const requestId = freshRequest()
  const merged = yield* approve(f, review, { requestId })
  expect(merged.result).toMatchObject({ outcome: "merged", head: review.head, baseHead: review.baseHead, mergeHead: git(rig.work, "rev-parse", "HEAD") })
  expect(yield* approve(f, review, { requestId })).toEqual(merged)
  const sql = yield* SqlClient.SqlClient
  expect(yield* sql`SELECT request_id FROM receipts WHERE request_id = ${requestId}`).toHaveLength(1)
})).pipe(Effect.orDie))

it.live("issue 579: an independent real writer completes while an approval Git subprocess is pending", () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  const f = yield* setup(), { review } = (yield* prepare(f)).result
  const bin = join(rig.root, "controlled-bin"), hold = join(rig.root, "hold"), started = join(rig.root, "started")
  mkdirSync(bin); writeFileSync(hold, "hold")
  // Only scheduling is controlled. All subprocess replies come from real Git.
  writeFileSync(join(bin, "git"), `#!${process.execPath}\nimport { existsSync, writeFileSync } from 'node:fs';\nimport { spawnSync } from 'node:child_process';\nconst args=process.argv.slice(2);\nif(args.includes('merge')) { writeFileSync(${JSON.stringify(started)},'started'); while(existsSync(${JSON.stringify(hold)})) await new Promise(r=>setTimeout(r,10)); }\nconst result=spawnSync('/usr/bin/git',args,{stdio:'inherit'}); process.exit(result.status ?? 1);\n`, { mode: 0o700 })
  vi.stubEnv("PATH", `${bin}:${process.env.PATH}`)
  const merge = yield* approve(f, review).pipe(Effect.forkChild)
  yield* Effect.addFinalizer(() => Effect.sync(() => { rmSync(hold, { force: true }) }))
  const startedBy = Date.now() + 5000
  while (!existsSync(started) && Date.now() < startedBy) yield* Effect.sleep("10 millis")
  expect(existsSync(started)).toBe(true)
  let completed = false
  let writerAttempted = false
  const writer = yield* Effect.sync(() => { writerAttempted = true }).pipe(Effect.andThen(run(f.api, "project.preferences.set", { projectId: f.projectId, shell: "bash" }))).pipe(Effect.tap(() => Effect.sync(() => { completed = true })), Effect.forkChild)
  const writerBy = Date.now() + 5000
  while (!completed && Date.now() < writerBy) yield* Effect.sleep("10 millis")
  expect(writerAttempted).toBe(true)
  const completedWhilePending = completed && existsSync(hold)
  rmSync(hold)
  yield* Fiber.join(merge); yield* Fiber.join(writer)
  expect(completedWhilePending).toBe(true)
})).pipe(Effect.orDie))

it.live("issue 579/583: queued registration and creation do not block a positive independent writer while real Git is held", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), { review } = (yield* prepare(f)).result
  git(rig.outside, "init", "-b", "main")
  const bin = join(rig.root, "registration-git"), hold = join(rig.root, "registration-hold"), started = join(rig.root, "registration-started")
  mkdirSync(bin); writeFileSync(hold, "owned hold")
  writeFileSync(join(bin, "git"), `#!${process.execPath}\nimport { existsSync, writeFileSync } from 'node:fs';\nimport { spawnSync } from 'node:child_process';\nconst args=process.argv.slice(2);\nif(args.includes('merge')) { writeFileSync(${JSON.stringify(started)},'started'); while(existsSync(${JSON.stringify(hold)})) await new Promise(r=>setTimeout(r,10)); }\nconst result=spawnSync('/usr/bin/git',args,{stdio:'inherit'}); process.exit(result.status ?? 1);\n`, { mode: 0o700 })
  vi.stubEnv("PATH", `${bin}:${process.env.PATH}`)
  const merge = yield* approve(f, review).pipe(Effect.forkChild)
  yield* Effect.addFinalizer(() => Effect.sync(() => { rmSync(hold, { force: true }); vi.restoreAllMocks() }))
  const startedBy = Date.now() + 5000
  while (!existsSync(started) && Date.now() < startedBy) yield* Effect.sleep("10 millis")
  expect(existsSync(started)).toBe(true)
  let gateAttempts = 0
  const realWithGit = GitApi.withGit
  const observed: typeof GitApi.withGit = (input, body) => Effect.sync(() => { gateAttempts++ }).pipe(Effect.andThen(realWithGit(input, body)))
  vi.spyOn(GitApi, "withGit").mockImplementation(observed)
  const registerRequest = { requestId: freshRequest(), name: "Queued repository", path: rig.outside, machineId: "local" }
  const createRequest = { requestId: freshRequest(), name: "Queued project", goal: "Observe without holding the writer", machineId: "local", repository: { _tag: "existing", repoId: f.repoId }, useWorktree: true }
  const registration = yield* call(owner, "project.repository.add", registerRequest).pipe(Effect.forkChild)
  const creation = yield* call(owner, "project.create", createRequest).pipe(Effect.forkChild)
  const queuedBy = Date.now() + 5000
  while (gateAttempts < 2 && Date.now() < queuedBy) yield* Effect.sleep("10 millis")
  expect(gateAttempts).toBeGreaterThanOrEqual(2)
  // Positive completion is observed after both physical paths reach the gate.
  // A delay or a merely forked request is not the scheduling oracle.
  let completed = false
  const writer = yield* run(f.api, "project.preferences.set", { projectId: f.projectId, shell: "bash" }).pipe(
    Effect.tap(() => Effect.sync(() => { completed = true })), Effect.forkChild)
  const writerBy = Date.now() + 5000
  while (!completed && Date.now() < writerBy) yield* Effect.sleep("10 millis")
  const completedWhileHeld = completed && existsSync(hold)
  expect(projectView(yield* snapshot).projects[f.projectId]?.shell).toBe("bash")
  rmSync(hold)
  yield* Fiber.join(merge); yield* Fiber.join(writer)
  const registered = success(yield* Fiber.join(registration)), created = success(yield* Fiber.join(creation))
  expect(completedWhileHeld).toBe(true)
  expect(success(yield* call(owner, "project.repository.add", registerRequest))).toEqual(registered)
  expect(success(yield* call(owner, "project.create", createRequest))).toEqual(created)
})).pipe(Effect.orDie))


for (const divergent of [false, true]) it.live(`issue 575: actual SQL failure after ${divergent ? "divergent" : "fast-forward"} Git merge retains the restart latch and terminal uncertainty after rebuild`, () => Effect.gen(function* () {
  const observedProcesses = vi.spyOn(GitProcesses, "gitProcess")
  yield* Effect.addFinalizer(() => Effect.sync(() => observedProcesses.mockRestore()))
  const runner = yield* makeFakeRunner(), secrets = makeFakeSecrets()
  let armed = true
  let breakWriter: Effect.Effect<void> = Effect.die(new Error("uninitialized SQL fault"))
  const wrapLog: NonNullable<Fakes["wrapLog"]> = (real) => ({ ...real, transact: (use) => real.transact((append) => use((actor, id, hash, events, side, receipt) => {
    if (armed && events.some((event) => event._tag === "project.MainReviewDecided")) {
      armed = false
      return breakWriter.pipe(Effect.andThen(append(actor, id, hash, events, side, receipt)))
    }
    return append(actor, id, hash, events, side, receipt)
  })) })
  const saved = yield* provideRig(rig, { runner, secrets, wrapLog }, Effect.gen(function* () {
    yield* registerRebuildCheck()
    const sql = yield* SqlClient.SqlClient
    breakWriter = sql`PRAGMA query_only = ON`.pipe(Effect.orDie, Effect.asVoid)
    const f = yield* setup()
    if (divergent) { save(rig.work, "base-only.txt", "base work\n"); commit(rig.work, "Base divergence") }
    const { review } = (yield* prepare(f)).result
    const request = { requestId: freshRequest(), projectId: f.projectId, worktreeId: f.worktreeId, reviewId: review.reviewId,
      expectedHead: review.head, expectedBaseHead: review.baseHead, confirmed: true, decision: "approve_without_evidence" }
    const failed = yield* call(owner, "project.review.approve", request)
    expect(failed._tag).toBe("Failure")
    if (failed._tag === "Failure") expect(failed.failure._tag).toBe("LogWriteFailed")
    const actual = git(rig.work, "rev-parse", "HEAD")
    expect(git(rig.work, "merge-base", "--is-ancestor", review.head, actual)).toBe("")
    expect(git(rig.work, "merge-base", "--is-ancestor", review.baseHead, actual)).toBe("")
    expect(projectView(yield* snapshot).gitReviews[f.worktreeId]!.outcome).toBe("open")
    expect(yield* sql`SELECT request_id FROM receipts WHERE request_id = ${request.requestId}`).toHaveLength(0)
    expect((yield* call(owner, "project.review.approve", request))._tag).toBe("Failure")
    expect((yield* readLog).filter((row) => row.event._tag === "project.MainReviewDecided")).toHaveLength(0)
    return { request, actual }
  }))
  yield* provideRig(rig, { runner, secrets }, Effect.gen(function* () {
    yield* registerRebuildCheck()
    const prior = yield* call(owner, "project.review.approve", saved.request)
    expect(prior._tag).toBe("Failure")
    if (prior._tag === "Failure") expect(prior.failure.reason).toBe("Git outcome is uncertain; refresh Git state")
    expect(projectView(yield* snapshot).gitReviews[saved.request.worktreeId]?.outcome).toBe("open")
    success(yield* call(owner, "project.git.state", { worktreeId: saved.request.worktreeId, projectId: saved.request.projectId }))
    expect(Object.values(projectView(yield* snapshot).gitAttempts)).toHaveLength(0)
    const terminal = yield* call(owner, "project.review.approve", saved.request)
    expect(terminal._tag).toBe("Failure")
    if (terminal._tag === "Failure") expect(terminal.failure.reason).toBe("Git outcome is uncertain; refresh Git state")
    // A new owner request uses the existing containment proof, without another
    // merge or retroactive success on the old uncertain identity.
    const fresh = { ...saved.request, requestId: freshRequest() }
    const result = success(yield* call(owner, "project.review.approve", fresh))
    expect(result.result).toMatchObject({ outcome: "merged", mergeHead: saved.actual })
    expect(success(yield* call(owner, "project.review.approve", fresh))).toEqual(result)
    expect(git(rig.work, "rev-parse", "HEAD")).toBe(saved.actual)
    expect((yield* readLog).filter((row) => row.event._tag === "project.MainReviewDecided")).toHaveLength(1)
  }))
  yield* provideRig(rig, { runner, secrets }, Effect.gen(function* () {
    yield* registerRebuildCheck()
    const old = yield* call(owner, "project.review.approve", saved.request)
    expect(old._tag).toBe("Failure")
    if (old._tag === "Failure") expect(old.failure.reason).toBe("Git outcome is uncertain; refresh Git state")
    expect(Object.values(projectView(yield* snapshot).gitAttempts)).toHaveLength(0)
  }))
  expect(observedProcesses.mock.calls.filter(([, args]) => args.includes("merge"))).toHaveLength(1)
}).pipe(Effect.orDie))

for (const merging of [false, true]) it.live(`issue 575: commit uncertainty after actual SQL failure preserves merge parent=${merging}`, () => Effect.gen(function* () {
  const observedProcesses = vi.spyOn(GitProcesses, "gitProcess")
  yield* Effect.addFinalizer(() => Effect.sync(() => observedProcesses.mockRestore()))
  const runner = yield* makeFakeRunner(), secrets = makeFakeSecrets()
  let requestId = "", armed = true
  let breakWriter: Effect.Effect<void> = Effect.die(new Error("The writer fault was not initialized"))
  const wrapLog: NonNullable<Fakes["wrapLog"]> = (real) => ({ ...real, transact: (use) => real.transact((append) => use((actor, id, hash, events, rows, receipt) => {
    if (armed && id === requestId && events.some((event) => event._tag === "project.GitFinished")) {
      armed = false
      return breakWriter.pipe(Effect.andThen(append(actor, id, hash, events, rows, receipt)))
    }
    return append(actor, id, hash, events, rows, receipt)
  })) })
  const saved = yield* provideRig(rig, { runner, secrets, wrapLog }, Effect.gen(function* () {
    yield* registerRebuildCheck()
    const sql = yield* SqlClient.SqlClient
    breakWriter = sql`PRAGMA query_only = ON`.pipe(Effect.orDie, Effect.asVoid)
    const f = yield* setup()
    const before = git(f.source, "rev-parse", "HEAD")
    let mergeHead: string | undefined
    const branch = git(f.source, "branch", "--show-current")
    if (merging) {
      git(f.source, "checkout", "-b", "side")
      save(f.source, "side.txt", "Side parent\n"); mergeHead = commit(f.source, "Side work")
      git(f.source, "checkout", branch)
      save(f.source, "primary.txt", "Primary parent\n"); commit(f.source, "Primary work")
      git(f.source, "merge", "--no-ff", "--no-commit", "side")
    } else {
      save(f.source, "intended.txt", "Frozen index content\n")
      success(yield* call(owner, "project.stage", { projectId: f.projectId, worktreeId: f.worktreeId, path: "intended.txt" }))
    }
    const parent = git(f.source, "rev-parse", "HEAD"), tree = git(f.source, "write-tree")
    requestId = freshRequest()
    const request = { requestId, projectId: f.projectId, worktreeId: f.worktreeId, message: "Intended frozen message" }
    const failed = yield* call(owner, "project.commit", request)
    expect(failed._tag).toBe("Failure")
    if (failed._tag === "Failure") expect(failed.failure._tag).toBe("LogWriteFailed")
    const actual = git(f.source, "rev-parse", "HEAD")
    expect(actual).not.toBe(before)
    expect(git(f.source, "show", "-s", "--format=%P", actual).split(" ")).toEqual(mergeHead === undefined ? [parent] : [parent, mergeHead])
    expect(git(f.source, "show", "-s", "--format=%T", actual)).toBe(tree)
    expect(yield* sql`SELECT request_id FROM receipts WHERE request_id = ${requestId}`).toHaveLength(0)
    expect(Object.values(projectView(yield* snapshot).gitAttempts)).toHaveLength(1)
    expect((yield* call(owner, "project.commit", request))._tag).toBe("Failure")
    return { request, actual, source: f.source }
  }))
  yield* provideRig(rig, { runner, secrets }, Effect.gen(function* () {
    yield* registerRebuildCheck()
    expect((yield* call(owner, "project.commit", { ...saved.request, message: "Different body" }))._tag).toBe("Failure")
    const prior = yield* call(owner, "project.commit", saved.request)
    expect(prior._tag).toBe("Failure")
    if (prior._tag === "Failure") expect(prior.failure.reason).toBe("Git outcome is uncertain; refresh Git state")
    success(yield* call(owner, "project.git.state", { projectId: saved.request.projectId, worktreeId: saved.request.worktreeId }))
    const terminal = yield* call(owner, "project.commit", saved.request)
    expect(terminal._tag).toBe("Failure")
    if (terminal._tag === "Failure") expect(terminal.failure.reason).toBe("Git outcome is uncertain; refresh Git state")
    expect((yield* call(owner, "project.commit", { ...saved.request, message: "Different body" }))._tag).toBe("Failure")
    expect(git(saved.source, "rev-parse", "HEAD")).toBe(saved.actual)
    expect(Object.values(projectView(yield* snapshot).gitAttempts)).toHaveLength(0)
    const sql = yield* SqlClient.SqlClient
    expect(yield* sql`SELECT request_id FROM receipts WHERE request_id = ${saved.request.requestId}`).toHaveLength(1)
    const events = (yield* readLog).filter((row) => row.requestId === saved.request.requestId).map((row) => row.event)
    expect(events).toMatchObject([{ _tag: "project.GitFinished", outcome: "uncertain" }])
  }))
  yield* provideRig(rig, { runner, secrets }, Effect.gen(function* () {
    yield* registerRebuildCheck()
    const old = yield* call(owner, "project.commit", saved.request)
    expect(old._tag).toBe("Failure")
    if (old._tag === "Failure") expect(old.failure.reason).toBe("Git outcome is uncertain; refresh Git state")
    expect(Object.values(projectView(yield* snapshot).gitAttempts)).toHaveLength(0)
  }))
  expect(observedProcesses.mock.calls.filter(([, args]) => args.includes("commit"))).toHaveLength(1)
}).pipe(Effect.orDie))

it.live("issue 595: partial main checkout permission failure leaves the review open and its request uncertain", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup()
  mkdirSync(join(rig.work, "dir")); save(rig.work, "dir/existing", "tracked directory\n")
  save(rig.work, "a.txt", "before a\n"); save(rig.work, "z.txt", "before z\n"); commit(rig.work, "Permission base")
  git(f.source, "merge", "--no-edit", "main")
  save(f.source, "a.txt", "after a\n"); save(f.source, "z.txt", "after z\n"); save(f.source, "dir/new", "cannot create\n"); commit(f.source, "Incoming files")
  const { review } = (yield* prepare(f)).result, before = git(rig.work, "rev-parse", "HEAD"), mergeCalls = observeMerges()
  const request = { requestId: freshRequest(), projectId: f.projectId, worktreeId: f.worktreeId, reviewId: review.reviewId,
    expectedHead: review.head, expectedBaseHead: review.baseHead, confirmed: true, decision: "approve_without_evidence" }
  chmodSync(join(rig.work, "dir"), 0o555)
  yield* Effect.addFinalizer(() => Effect.sync(() => chmodSync(join(rig.work, "dir"), 0o755)))
  const failed = yield* call(owner, "project.review.approve", request)
  expect(failed._tag).toBe("Failure")
  if (failed._tag === "Failure") expect(failed.failure).toMatchObject({ reason: "Git outcome is uncertain; refresh Git state" })
  expect(readFileSync(join(rig.work, "a.txt"), "utf8")).toBe("after a\n")
  expect(readFileSync(join(rig.work, "z.txt"), "utf8")).toBe("after z\n")
  expect(git(rig.work, "rev-parse", "HEAD")).toBe(before)
  expect(projectView(yield* snapshot).gitReviews[f.worktreeId]?.outcome).toBe("open")
  const sql = yield* SqlClient.SqlClient
  expect(yield* sql`SELECT request_id FROM receipts WHERE request_id = ${request.requestId}`).toHaveLength(0)
  expect((yield* readLog).filter((row) => row.event._tag === "project.MainReviewDecided")).toHaveLength(0)
  const retry = yield* call(owner, "project.review.approve", request)
  expect(retry._tag).toBe("Failure")
  if (retry._tag === "Failure") expect(retry.failure.reason).toBe("Git outcome is uncertain; refresh Git state")
  expect(mergeCalls()).toBe(1)
  yield* run(f.api, "project.git.state", { projectId: f.projectId, worktreeId: f.worktreeId })
  expect(Object.values(projectView(yield* snapshot).gitAttempts)).toHaveLength(0)
  const afterRefresh = yield* call(owner, "project.review.approve", request)
  expect(afterRefresh._tag).toBe("Failure")
  if (afterRefresh._tag === "Failure") expect(afterRefresh.failure.reason).toBe("Git outcome is uncertain; refresh Git state")
  expect(mergeCalls()).toBe(1)
  expect(projectView(yield* snapshot).gitReviews[f.worktreeId]?.outcome).toBe("open")
})).pipe(Effect.orDie))


it.live("s4-git2: sends a main review back atomically with its reason on the lead board and refuses old or agent decisions", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), review = (yield* prepare(f)).result.review;
  expect(typeof (f.api as any)["project.review.decline"]).toBe("function");
  const payload = { requestId: freshRequest(), projectId: f.projectId, worktreeId: f.worktreeId, reviewId: review.reviewId, expectedHead: review.head, expectedBaseHead: review.baseHead, reason: "Please add coverage <script>literal</script>." };
  const member = yield* client(f.socket, f.member.token);
  expect(yield* refusal(run(member, "project.review.decline", payload))).toBe("command refused");
  expect(yield* refusal(run(f.api, "project.review.decline", { ...payload, requestId: freshRequest(), reason: "   " }))).toBe("give a reason for sending the review back");
  const reply = yield* run(f.api, "project.review.decline", payload);
  expect(reply.result).toMatchObject({ outcome: "sent_back", reason: payload.reason, decision: null, mergeHead: null });
  expect(yield* run(f.api, "project.review.decline", payload)).toEqual(reply);
  expect((yield* run(f.api, "project.review.status", f)).result.review.outcome).toBe("sent_back");
  const posts = (yield* readLog).filter((row) => row.event._tag === "board.PostWritten" && row.event.text.includes(payload.reason));
  expect(posts).toHaveLength(1);
  expect(posts[0]!.event).toMatchObject({ to: [{ _tag: "agent", agentId: projectView(yield* snapshot).projects[f.projectId]!.leadAgentId }] });
  expect(yield* refusal(approve(f, review))).toBe("main review is unavailable; prepare a new review");
  expect(yield* refusal(run(f.api, "project.review.decline", { ...payload, requestId: freshRequest() }))).toBe("main review is unavailable; prepare a new review");
  expect(git(rig.work, "rev-parse", "HEAD")).toBe(f.baseHead);
})).pipe(Effect.orDie));

it.live("s4-git2: updates only from the registered parent and integrates a registered builder into its team", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup();
  expect(typeof (f.api as any)["project.update"]).toBe("function");
  save(rig.work, "parent.txt", "parent change\n"); const parentHead = commit(rig.work, "Parent update");
  const update = { requestId: freshRequest(), projectId: f.projectId, worktreeId: f.worktreeId, destinationId: f.destinationId, expectedHead: parentHead };
  yield* run(f.api, "project.update", update);
  expect(git(f.source, "merge-base", "--is-ancestor", parentHead, "HEAD")).toBe("");
  const teamPath = join(rig.root, "team"), builderPath = join(rig.root, "builder");
  git(f.source, "worktree", "add", "-b", "team-build", teamPath);
  const teamId = (yield* run(f.api, "project.workspace.register", { repoId: f.repoId, machineId: "local", path: teamPath, branch: "team-build", checkout: "worktree", parentId: f.worktreeId })).worktreeId;
  const team = yield* run(f.api, "project.team.create", { projectId: f.projectId, name: "Build", worktreeId: teamId });
  git(teamPath, "worktree", "add", "-b", "builder-work", builderPath);
  const builderId = (yield* run(f.api, "project.workspace.register", { repoId: f.repoId, machineId: "local", path: builderPath, branch: "builder-work", checkout: "worktree", parentId: teamId })).worktreeId;
  save(builderPath, "builder.txt", "builder contribution\n"); const head = commit(builderPath, "Builder contribution");
  yield* run(f.api, "project.integrate", { worktreeId: builderId, destinationId: teamId, expectedHead: head });
  expect(readFileSync(join(teamPath, "builder.txt"), "utf8")).toBe("builder contribution\n");
  expect(existsSync(join(f.source, "builder.txt"))).toBe(false);
  expect(yield* refusal(run(f.api, "project.integrate", { worktreeId: builderId, destinationId: f.worktreeId, expectedHead: head }))).toBe("integration requires a registered team source and its project workspace");
  expect(yield* refusal(run(f.api, "project.update", { ...update, requestId: freshRequest(), destinationId: teamId }))).toBe("update requires the registered parent workspace");
  save(rig.work, "file.txt", "parent conflict\n"); const nextParent = commit(rig.work, "Parent conflict");
  save(f.source, "file.txt", "project conflict\n"); commit(f.source, "Project conflict");
  expect(yield* refusal(run(f.api, "project.update", { ...update, requestId: freshRequest(), expectedHead: nextParent }))).toBe("Git: conflicts");
  const state = (yield* run(f.api, "project.git.state", { worktreeId: f.worktreeId })).result;
  expect(state.mergeHead).toBe(nextParent);
  const conflict = (yield* run(f.api, "project.conflict", { worktreeId: f.worktreeId, path: "file.txt" })).result;
  const chosen = (yield* run(f.api, "project.conflict.choose", { worktreeId: f.worktreeId, selection: conflict, hunk: 0, choice: "ours" })).result;
  yield* run(f.api, "project.conflict.resolve", { worktreeId: f.worktreeId, selection: chosen });
  const seen = (yield* run(f.api, "project.conflict.resolved", { worktreeId: f.worktreeId })).result.items[0];
  expect(seen.path).toBe("file.txt");
  yield* run(f.api, "project.conflict.reopen", { worktreeId: f.worktreeId, path: "file.txt", expectedHead: state.head, expectedMergeHead: state.mergeHead, fingerprint: seen.fingerprint, confirmed: true });
  expect((yield* run(f.api, "project.conflict", { worktreeId: f.worktreeId, path: "file.txt" })).result.choices).toEqual([null]);
  yield* run(f.api, "project.merge.abort", { worktreeId: f.worktreeId, expectedHead: state.head, expectedMergeHead: state.mergeHead, confirmed: true });
  expect((yield* run(f.api, "project.git.state", { worktreeId: f.worktreeId })).result.mergeHead).toBeNull();
})).pipe(Effect.orDie));


it.live("s4-git2: a send-back decision waits for the same workspace's in-flight owner approval", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), review = (yield* prepare(f)).result.review;
  const original = GitProcesses.gitProcess;
  let release!: () => void, entered!: () => void;
  const held = new Promise<void>((resolve) => { release = resolve; });
  const started = new Promise<void>((resolve) => { entered = resolve; });
  const spy = vi.spyOn(GitProcesses, "gitProcess").mockImplementation(async (...args) => {
    if (args[0] === rig.work && args[1].includes("merge") && !args[1].includes("--is-ancestor")) { entered(); await held; }
    return original(...args);
  });
  yield* Effect.gen(function* () {
    const merging = yield* approve(f, review).pipe(Effect.forkChild);
    yield* Effect.promise(() => started);
    const declining = yield* run(f.api, "project.review.decline", { worktreeId: f.worktreeId, projectId: f.projectId,
      reviewId: review.reviewId, expectedHead: review.head, expectedBaseHead: review.baseHead, reason: "Please check the tests." }).pipe(Effect.result, Effect.forkChild);
    yield* Effect.sleep(100);
    expect(projectView(yield* snapshot).gitReviews[f.worktreeId]!.outcome).toBe("open");
    release();
    expect((yield* Fiber.join(merging)).result.outcome).toBe("merged");
    expect((yield* Fiber.join(declining))._tag).toBe("Failure");
  }).pipe(Effect.ensuring(Effect.sync(() => { release(); spy.mockRestore(); })));
})).pipe(Effect.orDie));

const decline = (f: Effect.Success<ReturnType<typeof setup>>, review: any, extra: Record<string, unknown> = {}) => run(f.api, "project.review.decline", {
  projectId: f.projectId, worktreeId: f.worktreeId, reviewId: review.reviewId, expectedHead: review.head, expectedBaseHead: review.baseHead, reason: "Please check the tests.", ...extra,
});
it.live("review 2: a send-back wakes the recorded lead and replaces its queued board wake without dropping withdrawal", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), review = (yield* prepare(f)).result.review;
  const lead = projectView(yield* snapshot).projects[f.projectId]!.leadAgentId;
  yield* run(f.api, "board.post", { direct: { _tag: "agent", agentId: lead }, to: [{ _tag: "agent", agentId: lead }], reply: "all", text: "Earlier queued lead work." });
  const earlier = Object.values((yield* snapshot).deliveries).find((d) => d.target === lead && d.source?._tag === "piece" && d.source.namespace === "board" && d.status === "queued")!;
  expect(earlier).toBeDefined();
  yield* decline(f, review);
  const rows = yield* readLog, decided = rows.find((r) => r.event._tag === "project.MainReviewDecided")!;
  const ownRows = rows.filter((r) => r.requestId === decided.requestId);
  expect(ownRows.some((r) => r.event._tag === "DeliveryWithdrawn" && r.event.deliveryId === earlier.id)).toBe(true);
  expect((yield* snapshot).deliveries[earlier.id]!.status).toBe("withdrawn");
  const queued = Object.values((yield* snapshot).deliveries).filter((d) => d.target === lead && d.source?._tag === "piece" && d.source.namespace === "board" && d.status === "queued");
  expect(queued).toHaveLength(1);
  expect(queued[0]!.content).toMatchObject({ _tag: "prompt", text: expect.stringContaining("Please check the tests.") });
})).pipe(Effect.orDie));
for (const absent of ["stopped", "removed"] as const) it.live(`review 2: refuses a send-back when its lead is ${absent}`, () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), review = (yield* prepare(f)).result.review, lead = projectView(yield* snapshot).projects[f.projectId]!.leadAgentId;
  if (absent === "stopped") yield* run(f.api, "stop", { agentId: lead });
  else { const views = yield* Views; const original = views.snapshot; Object.defineProperty(views, "snapshot", { configurable: true, get: () => original.pipe(Effect.map((s) => ({ ...s, agents: Object.fromEntries(Object.entries(s.agents).filter(([id]) => id !== lead)) }))) }); yield* Effect.addFinalizer(() => Effect.sync(() => Object.defineProperty(views, "snapshot", { configurable: true, value: original }))); }
  expect(yield* refusal(decline(f, review))).toBe("project lead is unavailable; the review was not sent back");
  expect(projectView(yield* snapshot).gitReviews[f.worktreeId]!.outcome).toBe("open");
})).pipe(Effect.orDie));
for (const flaw of ["afterCommit", "sideRows"] as const) it.live(`review 1: decline rejects a planner's ${flaw} boundary violation`, () => {
  const original = engineLists.planners["project.review.decline"]!;
  const planner: PiecePlanner = (input, raw) => original(input, raw).pipe(Effect.map((plan) => ({ ...plan, ...(flaw === "afterCommit" ? { afterCommit: Effect.void } : { sideRows: { foreign_rows: [] } }) } as unknown as PiecePlan)));
  return provideBoard(rig, Effect.gen(function* () {
    const f = yield* setup(), review = (yield* prepare(f)).result.review;
    expect(yield* refusal(decline(f, review))).toBe("command refused");
    expect(projectView(yield* snapshot).gitReviews[f.worktreeId]!.outcome).toBe("open");
  }), [], { lists: { ...engineLists, planners: { ...engineLists.planners, "project.review.decline": planner } } }).pipe(Effect.orDie);
});
it.live("review 3: update reads a dirty parent's commit but refuses a dirty child, stale head and foreign repository", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(); save(rig.work, "parent.txt", "committed parent\n"); const parent = commit(rig.work, "Parent update");
  const payload = { worktreeId: f.worktreeId, destinationId: f.destinationId, expectedHead: parent };
  save(rig.work, "untracked.txt", "Leave this parent file alone.\n"); save(rig.work, "file.txt", "uncommitted parent edit\n");
  yield* run(f.api, "project.update", payload);
  expect(readFileSync(join(f.source, "parent.txt"), "utf8")).toBe("committed parent\n");
  expect(readFileSync(join(rig.work, "file.txt"), "utf8")).toBe("uncommitted parent edit\n");
  save(f.source, "dirty.txt", "child edit\n");
  expect(yield* refusal(run(f.api, "project.update", payload))).toBe("Git: dirty_destination"); rmSync(join(f.source, "dirty.txt"));
  expect(yield* refusal(run(f.api, "project.update", { ...payload, expectedHead: f.baseHead }))).toBe("Git: stale_state");
  const other = join(rig.root, "other-repository"); mkdirSync(other); git(other, "init", "-b", "main"); git(other, "config", "user.name", "Builder"); git(other, "config", "user.email", "builder@example.invalid"); git(other, "config", "commit.gpgsign", "false"); save(other, "foreign.txt", "foreign\n"); const foreignHead = commit(other, "Foreign");
  const repository = yield* run(f.api, "project.repository.add", { name: "Other", path: other, machineId: "local" });
  const workspace = yield* run(f.api, "project.workspace.register", { repoId: repository.repoId, path: other, branch: "main", machineId: "local", checkout: "main" });
  expect(yield* refusal(run(f.api, "project.update", { ...payload, destinationId: workspace.worktreeId, expectedHead: foreignHead }))).toBe("update requires the registered parent workspace");
})).pipe(Effect.orDie));
it.live("review 9: every new owner-only Git action refuses an authenticated agent with the exact reason", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), member = yield* client(f.socket, f.member.token);
  for (const [name, extra] of [["project.merge.abort", { expectedHead: f.head, expectedMergeHead: f.baseHead, confirmed: true }], ["project.conflict.reopen", { path: "file.txt", expectedHead: f.head, expectedMergeHead: f.baseHead, fingerprint: "abc", confirmed: true }], ["project.conflict.resolved", {}], ["project.update", { destinationId: f.destinationId, expectedHead: f.baseHead }]] as const) {
    expect(yield* refusal(run(member, name, { projectId: f.projectId, worktreeId: f.worktreeId, ...extra }))).toBe("command refused");
  }
})).pipe(Effect.orDie));

it.live("review 2: sending back an idle lead's review queues its reason without any earlier board duty", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), review = (yield* prepare(f)).result.review;
  const lead = projectView(yield* snapshot).projects[f.projectId]!.leadAgentId;
  expect(Object.values((yield* snapshot).deliveries).filter((d) => d.target === lead && d.source?._tag === "piece" && d.source.namespace === "board")).toEqual([]);
  yield* decline(f, review);
  const deliveries = Object.values((yield* snapshot).deliveries).filter((d) => d.target === lead && d.source?._tag === "piece" && d.source.namespace === "board" && d.status === "queued");
  expect(deliveries).toHaveLength(1);
  expect(deliveries[0]!.content).toMatchObject({ _tag: "prompt", text: expect.stringContaining("Please check the tests.") });
})).pipe(Effect.orDie));
it.live("review 7: abort's overlapping-edit refusal is fixed on the wire and replayable", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup();
  expect(yield* refusal(run(f.api, "project.merge.abort", { worktreeId: f.worktreeId, expectedHead: f.head, expectedMergeHead: f.baseHead, confirmed: true }))).toBe("Git: no_merge");
  save(rig.work, "file.txt", "parent conflict\n"); const parent = commit(rig.work, "Parent conflict");
  save(f.source, "file.txt", "project conflict\n"); commit(f.source, "Project conflict");
  expect(yield* refusal(run(f.api, "project.update", { worktreeId: f.worktreeId, destinationId: f.destinationId, expectedHead: parent }))).toBe("Git: conflicts");
  const c = (yield* run(f.api, "project.conflict", { worktreeId: f.worktreeId, path: "file.txt" })).result;
  const chosen = (yield* run(f.api, "project.conflict.choose", { worktreeId: f.worktreeId, selection: c, hunk: 0, choice: "both" })).result;
  yield* run(f.api, "project.conflict.resolve", { worktreeId: f.worktreeId, selection: chosen });
  git(f.source, "add", "--", "file.txt"); save(f.source, "file.txt", "owner edit\n");
  const p = { requestId: freshRequest(), worktreeId: f.worktreeId, expectedHead: git(f.source, "rev-parse", "HEAD"), expectedMergeHead: parent, confirmed: true };
  expect(yield* refusal(run(f.api, "project.merge.abort", p))).toBe("Git: abort_blocked");
  expect(yield* refusal(run(f.api, "project.merge.abort", p))).toBe("Git: abort_blocked");
  expect(Object.values(projectView(yield* snapshot).gitAttempts)).toEqual([]);
  expect(readFileSync(join(f.source, "file.txt"), "utf8")).toBe("owner edit\n");
})).pipe(Effect.orDie));
it.live("review 12: project-wide Git readers include other team builders, but exclude outsiders and the base checkout", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), parent = projectView(yield* snapshot).projects[f.projectId]!.leadAgentId;
  const teamPath = join(rig.root, "access-team"), builderPath = join(rig.root, "access-builder");
  git(f.source, "worktree", "add", "-b", "access-team", teamPath);
  const team = yield* run(f.api, "project.workspace.register", { repoId: f.repoId, path: teamPath, branch: "access-team", machineId: "local", checkout: "worktree", parentId: f.worktreeId });
  yield* run(f.api, "project.team.create", { projectId: f.projectId, name: "Access", worktreeId: team.worktreeId });
  git(teamPath, "worktree", "add", "-b", "access-builder", builderPath);
  const builder = yield* run(f.api, "project.workspace.register", { repoId: f.repoId, path: builderPath, branch: "access-builder", machineId: "local", checkout: "worktree", parentId: team.worktreeId });
  save(builderPath, "file.txt", "builder\n"); const head = commit(builderPath, "Builder");
  save(teamPath, "file.txt", "team\n"); commit(teamPath, "Team"); expect(() => git(builderPath, "merge", "access-team")).toThrow();
  const otherPath = join(rig.root, "other-team"); git(f.source, "worktree", "add", "-b", "other-team", otherPath);
  const otherWorkspace = yield* run(f.api, "project.workspace.register", { repoId: f.repoId, path: otherPath, branch: "other-team", machineId: "local", checkout: "worktree", parentId: f.worktreeId });
  const otherTeamRecord = yield* run(f.api, "project.team.create", { projectId: f.projectId, name: "Other", worktreeId: otherWorkspace.worktreeId });
  const sibling = yield* seedAgent({ id: "other-team-reader", parent, folder: otherPath, groups: [otherTeamRecord.group] });
  const outsider = yield* seedAgent({ id: "outside-project" });
  const member = yield* client(f.socket, f.member.token), otherTeam = yield* client(f.socket, sibling.token), foreign = yield* client(f.socket, outsider.token);
  for (const [name, extra] of [["project.changes", {}], ["project.diff", { path: "file.txt" }], ["project.history", {}], ["project.commit.files", { commit: head }], ["project.git.state", {}], ["project.review", { base: "access-team" }], ["project.conflict", { path: "file.txt" }]] as const) {
    const p = { projectId: f.projectId, worktreeId: builder.worktreeId, ...extra };
    expect((yield* run(f.api, name, { worktreeId: builder.worktreeId, ...extra })).result).toBeDefined();
    expect((yield* run(member, name, p)).data).toBeTypeOf("string");
    expect((yield* run(otherTeam, name, p)).data).toBeTypeOf("string");
    expect(yield* refusal(run(foreign, name, p))).toBe("project is unavailable to this caller");
    expect(yield* refusal(run(member, name, { ...p, projectId: "another-project" }))).toBe("project is unavailable to this caller");
    expect(yield* refusal(run(member, name, { ...p, worktreeId: f.destinationId }))).toBe("project is unavailable to this caller");
  }
})).pipe(Effect.orDie));

it.live("GitHub prepare pushes only the project branch, creates one redacted pull request, and owner approval merges remotely", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  const first = (yield* prepare(f)).result
  expect(first.review.delivery).toMatchObject({ mode: "github", number: 17, state: "OPEN", repository: "fixture/product" })
  expect(gh.read().title).toContain("Product work")
  expect(gh.read().body).toContain("Review actual changes")
  expect(git(gh.bare, "rev-parse", `refs/heads/${first.review.branch}`)).toBe(f.head)
  expect((yield* prepare(f)).result.review.delivery.number).toBe(17)
  const review = (yield* prepare(f)).result.review
  const result = (yield* approve(f, review)).result
  expect(result).toMatchObject({ outcome: "merged", delivery: { mode: "github", state: "MERGED" }, localSync: "The local base checkout was fast-forwarded." })
  expect(git(rig.work, "rev-parse", "HEAD")).toBe(result.mergeHead)
  expect(git(rig.work, "rev-list", "--parents", "-n", "1", result.mergeHead).split(" ")).toHaveLength(3)
})).pipe(Effect.orDie))

for (const status of ["DIRTY", "BLOCKED", "UNKNOWN"]) it.live(`GitHub ${status} refuses approval without changing local files`, () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  const { review } = (yield* prepare(f)).result
  gh.set({ mergeStateStatus: status })
  expect((yield* approve(f, review)).result).toMatchObject({ outcome: "refused", reason: expect.stringContaining("GitHub refused") })
  expect(git(rig.work, "rev-parse", "HEAD")).toBe(f.baseHead)
  expect(readFileSync(gh.calls, "utf8")).not.toContain('"PUT"')
})).pipe(Effect.orDie))

it.live("GitHub closed pull requests and stale source or remote base heads cannot merge", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  let review = (yield* prepare(f)).result.review
  gh.set({ state: "CLOSED" })
  expect((yield* approve(f, review)).result.outcome).toBe("refused")
  gh.set({ state: "OPEN" })
  review = (yield* prepare(f)).result.review
  save(f.source, "later.txt", "later\n"); const later = commit(f.source, "Later commit")
  expect((yield* approve(f, review)).result).toMatchObject({ outcome: "refused", reason: "stale_state" })
  review = (yield* prepare(f)).result.review
  expect(git(gh.bare, "rev-parse", `refs/heads/${review.branch}`)).toBe(later)
  git(gh.bare, "update-ref", "refs/heads/main", later)
  expect(yield* refusal(approve(f, review))).toBe("The GitHub base moved since this review was prepared. Open the merge review again.")
  expect(projectView(yield* snapshot).gitReviews[f.worktreeId]?.outcome).toBe("open")
  expect(git(rig.work, "rev-parse", "HEAD")).toBe(f.baseHead)
})).pipe(Effect.orDie))

for (const dirty of [true, false]) it.live(`GitHub approval preserves the local base when ${dirty ? "dirty" : "on another branch"}`, () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  const { review } = (yield* prepare(f)).result
  if (dirty) save(rig.work, "file.txt", "owner edits\n")
  else git(rig.work, "checkout", "-b", "owner-unrelated")
  const result = (yield* approve(f, review)).result
  expect(result).toMatchObject({ outcome: "merged", localSync: expect.stringContaining("left alone") })
  expect(git(rig.work, "rev-parse", "HEAD")).toBe(f.baseHead)
  if (dirty) expect(readFileSync(join(rig.work, "file.txt"), "utf8")).toBe("owner edits\n")
})).pipe(Effect.orDie))

it.live("GitHub fallback explains no remote, non-GitHub, signed out and missing CLI", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup()
  expect((yield* prepare(f)).result.review.delivery).toMatchObject({ mode: "local", reason: expect.stringContaining("remote") })
  const gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  gh.set({ signedOut: true })
  expect((yield* prepare(f)).result.review.delivery).toMatchObject({ mode: "local", reason: expect.stringContaining("not signed in") })
  rmSync(join(gh.bin, "gh")); writeFileSync(join(gh.bin, "git"), '#!/bin/sh\nexec /usr/bin/git "$@"\n', { mode: 0o700 })
  vi.stubEnv("PATH", gh.bin)
  expect((yield* prepare(f)).result.review.delivery).toMatchObject({ mode: "local", reason: expect.stringContaining("not installed") })
  git(rig.work, "remote", "set-url", "origin", gh.bare)
  expect((yield* prepare(f)).result.review.delivery).toMatchObject({ mode: "local", reason: expect.stringContaining("not on GitHub") })
})).pipe(Effect.orDie))

it.live("GitHub refuses forbidden remotes, including rewritten push URLs, before pushing", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  for (const url of ["https://github.com/vitalysim/herdr-synapse.git", "git@github.com:vitalysim/herdr-synapse.git", "ssh://git@github.com/vitalysim/herdr-synapse", "https://github.com/vitalysim/herdr-synapse.git///", "https://github.com/%76italysim/herdr-synapse.git"]) {
    git(rig.work, "config", "remote.origin.pushurl", url)
    expect(yield* refusal(prepare(f))).toBe("Git: forbidden_remote")
  }
  expect(git(gh.bare, "branch", "--list", git(f.source, "branch", "--show-current"))).toBe("")
})).pipe(Effect.orDie))

it.live("GitHub never executes hostile title or branch data, and redacts reported secrets before publishing", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  const secret = "synthetic-review-secret-987654"
  gh.set({ secret })
  yield* report(f, f.api, { summary: `Evidence ${secret} $(touch SHOULD_NOT_EXIST)` })
  const { review } = (yield* prepare(f)).result
  expect(gh.read().body).not.toContain(secret)
  expect(gh.read().body).toContain("[REDACTED]")
  expect(JSON.stringify(yield* readLog)).not.toContain(secret)
  expect(gh.read().body).toContain("$(touch SHOULD_NOT_EXIST)")
  expect(existsSync(join(f.source, "SHOULD_NOT_EXIST"))).toBe(false)
  gh.set({ title: secret, body: secret })
  expect(JSON.stringify((yield* prepare(f)).result)).not.toContain(secret)
  const member = yield* client(f.socket, f.member.token), before = readFileSync(gh.calls, "utf8")
  expect(yield* refusal(run(member, "project.review.prepare", { ...f }))).toBe("command refused")
  expect(yield* refusal(approve({ ...f, api: member }, review))).toBe("command refused")
  expect(readFileSync(gh.calls, "utf8")).toBe(before)
}), ["synthetic-review-secret-987654"]).pipe(Effect.orDie))

for (const failure of ["hang", "overflow"] as const) it.live(`GitHub ${failure} is bounded without a local or remote merge`, () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  gh.set({ [failure]: true })
  const service = makeGitService({ workspaceFolder: rig.root, networkTimeoutMs: failure === "hang" ? 100 : 5000 })
  const comparison = yield* service.reviewMain(f.source, rig.work)
  const result = yield* service.prepareDelivery(f.source, comparison, { title: "Bounded", body: "Data" }, { _tag: "owner" }, rig.work).pipe(Effect.result)
  expect(result).toMatchObject({ _tag: "Failure", failure: { reason: failure === "hang" ? "timeout" : "output_limit" } })
  expect(git(rig.work, "rev-parse", "HEAD")).toBe(f.baseHead)
  expect(git(gh.bare, "rev-parse", "refs/heads/main")).toBe(f.baseHead)
})).pipe(Effect.orDie))

it.live("GitHub hostile branch and title remain argument data, and direct service calls require the owner", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  const branch = "project/$(id)"
  git(f.source, "branch", "-m", branch)
  yield* appendSetup([eventOf({ _tag: "project.WorktreeCreated", repoId: f.repoId, worktreeId: f.worktreeId, branch,
    path: f.source, machineId: "local", checkout: "worktree" })])
  const service = makeGitService({ workspaceFolder: rig.root }), comparison = yield* service.reviewMain(f.source, rig.work)
  const title = "--head=evil $(touch TITLE_EXECUTED) `id`"
  expect(yield* service.reviewMain(f.source, rig.work, { _tag: "agent", agentId: f.member.agentId }).pipe(Effect.result)).toMatchObject({ _tag: "Failure", failure: { reason: "owner_required" } })
  const denied = yield* service.prepareDelivery(f.source, comparison, { title, body: "Data" }, { _tag: "agent", agentId: f.member.agentId }, rig.work).pipe(Effect.result)
  expect(denied).toMatchObject({ _tag: "Failure", failure: { reason: "owner_required" } })
  const { delivery } = yield* service.prepareDelivery(f.source, comparison, { title, body: "Data" }, { _tag: "owner" }, rig.work)
  expect(delivery).toMatchObject({ mode: "github", title })
  expect(git(gh.bare, "rev-parse", `refs/heads/${branch}`)).toBe(f.head)
  expect(existsSync(join(f.source, "TITLE_EXECUTED"))).toBe(false)
  expect(gh.read().branch).toBe(branch)
  const deniedMerge = yield* service.approveDelivery(f.source, rig.work, comparison, delivery as any, { _tag: "agent", agentId: f.member.agentId }, true).pipe(Effect.result)
  expect(deniedMerge).toMatchObject({ _tag: "Failure", failure: { reason: "owner_required" } })
})).pipe(Effect.orDie))

it.live("GitHub push never forces a remote branch that has advanced independently", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  const { review } = (yield* prepare(f)).result
  expect(git(gh.bare, "for-each-ref", "--format=%(refname)", "refs/heads/").split("\n").sort()).toEqual(["refs/heads/main", `refs/heads/${review.branch}`].sort())
  const tree = git(gh.bare, "rev-parse", `${f.head}^{tree}`)
  const advanced = git(gh.bare, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit-tree", tree, "-p", f.head, "-m", "Remote work")
  git(gh.bare, "update-ref", `refs/heads/${review.branch}`, advanced, f.head)
  expect(yield* refusal(prepare(f))).toBe("Git: push_rejected")
  expect(git(gh.bare, "rev-parse", `refs/heads/${review.branch}`)).toBe(advanced)
  expect(git(rig.work, "rev-parse", "HEAD")).toBe(f.baseHead)
  expect(git(f.source, "rev-parse", "HEAD")).toBe(f.head)
})).pipe(Effect.orDie))

it.live("s27 R2 M1: live base movement refuses even when the pull request base snapshot is unchanged", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  const { review } = (yield* prepare(f)).result
  git(gh.bare, "update-ref", "refs/heads/main", f.head)
  expect(gh.read().creationBase).toBe(f.baseHead)
  expect(yield* refusal(approve(f, review))).toBe("The GitHub base moved since this review was prepared. Open the merge review again.")
  expect(projectView(yield* snapshot).gitReviews[f.worktreeId]?.outcome).toBe("open")
  expect(readFileSync(gh.calls, "utf8")).not.toContain('"PUT"')
  const refreshed = (yield* prepare(f)).result.review
  expect(refreshed.baseHead).toBe(f.head)
  expect((yield* approve(f, refreshed)).result.outcome).toBe("merged")
})).pipe(Effect.orDie))

it.live("s27 R2 M1: a reused request can be prepared and approved after another base merge", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  yield* prepare(f)
  save(rig.work, "other-project.txt", "Other project\n"); const updated = commit(rig.work, "Other project merged")
  git(gh.bare, "fetch", rig.work, "main:main")
  const { review } = (yield* prepare(f)).result
  expect(gh.read().creationBase).toBe(f.baseHead)
  expect(review.delivery.remoteBaseHead).toBe(updated)
  expect((yield* approve(f, review)).result.outcome).toBe("merged")
})).pipe(Effect.orDie))

for (const localUpdated of [false, true]) it.live(`s27 R2 M2/m8: a lost successful GitHub merge receipt reconciles with local update ${localUpdated}`, () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  const { review } = (yield* prepare(f)).result
  const original = GitProcesses.ghProcess
  const boundary = vi.spyOn(GitProcesses, "ghProcess").mockImplementation((folder, args, limits, signal) => original(folder, args, { ...limits, ...(args.includes("PUT") ? { timeoutMs: 1500 } : {}) }, signal))
  yield* Effect.addFinalizer(() => Effect.sync(() => boundary.mockRestore()))
  const requestId = freshRequest()
  gh.set({ hangAt: "merge" })
  expect(yield* refusal(approve(f, review, { requestId }))).toBe("Git outcome is uncertain; refresh Git state")
  yield* noGitReceipt(requestId)
  const merged = gh.read().mergeCommit
  expect(gh.read().state).toBe("MERGED")
  gh.set({ hangAt: null }); boundary.mockRestore()
  expect(yield* refusal(approve(f, review, { requestId }))).toBe("Git outcome is uncertain; refresh Git state")
  yield* run(f.api, "project.git.state", { projectId: f.projectId, worktreeId: f.worktreeId })
  expect(Object.values(projectView(yield* snapshot).gitAttempts)).toHaveLength(0)
  expect(yield* refusal(approve(f, review, { requestId }))).toBe("Git outcome is uncertain; refresh Git state")
  if (localUpdated) { git(rig.work, "fetch", gh.bare, "main"); git(rig.work, "merge", "--ff-only", merged) }
  const result = (yield* approve(f, review)).result
  expect(result).toMatchObject({ outcome: "merged", mergeHead: merged, delivery: { state: "MERGED" } })
  expect(git(rig.work, "rev-parse", "HEAD")).toBe(merged)
  expect(readFileSync(gh.calls, "utf8").split('\n').filter(line => line.includes('"PUT"'))).toHaveLength(1)
})).pipe(Effect.orDie))

it.live("s27 R2 m2: a non-GitHub push URL refuses before branch publication", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  for (const rewrite of [false, true]) {
    if (rewrite) {
      git(rig.work, "config", "--unset", "remote.origin.pushurl")
      git(rig.work, "config", "url.https://elsewhere.invalid/fixture/product.git.insteadOf", "https://github.com/fixture/product.git")
    } else git(rig.work, "config", "remote.origin.pushurl", "https://elsewhere.invalid/fixture/product.git")
    expect(yield* refusal(prepare(f))).toContain("GitHub")
    expect(git(gh.bare, "branch", "--list", git(f.source, "branch", "--show-current"))).toBe("")
  }
  expect(readFileSync(gh.gitCalls, "utf8").split('\n').filter(Boolean).map(line => JSON.parse(line) as string[]).some(args => args.includes("push"))).toBe(false)
})).pipe(Effect.orDie))

for (const fork of [false, true]) it.live(`s27 R2 m3/m4/m8: numeric project branches ignore fork requests and never become pull request numbers (${fork})`, () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work), branch = "123"
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  git(f.source, "branch", "-m", branch)
  yield* appendSetup([eventOf({ _tag: "project.WorktreeCreated", repoId: f.repoId, worktreeId: f.worktreeId, branch, path: f.source, machineId: "local", checkout: "worktree" })])
  gh.set({ fork })
  const { review } = (yield* prepare(f)).result
  expect(review.delivery).toMatchObject({ mode: "github", number: 17 })
  const refreshed = (yield* prepare(f)).result.review
  expect(refreshed.delivery.number).toBe(17)
  expect((yield* approve(f, refreshed)).result.outcome).toBe("merged")
  expect(readFileSync(gh.calls, "utf8")).not.toContain('["pr","view","123"')
})).pipe(Effect.orDie))

it.live("s27 R2 m5: unreachable GitHub refuses with a sentence instead of offering local approval", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  gh.set({ offline: true, secret: "synthetic-network-secret-987654" })
  expect(yield* refusal(prepare(f))).toMatch(/GitHub.*[.]$/)
  expect(projectView(yield* snapshot).gitReviews[f.worktreeId]).toBeUndefined()
  expect(JSON.stringify(yield* readLog)).not.toContain("synthetic-network-secret-987654")
})).pipe(Effect.orDie))

it.live("s27 R2 m6: forbidden SCP paths are normalized before any push", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  for (const url of ["git@github.com:vitalysim//herdr-synapse.git", "git@github.com:./vitalysim/./herdr-synapse.git", "git@github.com:any/../vitalysim/herdr-synapse.git"]) {
    git(rig.work, "config", "remote.origin.pushurl", url)
    expect(yield* refusal(prepare(f))).toBe("Git: forbidden_remote")
  }
})).pipe(Effect.orDie))

it.live("s27 R2 m1: delivery disables configured signing and remote programs while retaining a real push and fetch", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work), marker = join(rig.root, "program-ran"), program = join(rig.root, "unwanted-program")
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  writeFileSync(program, `#!/bin/sh\ntouch '${marker}'\nexit 1\n`, { mode: 0o700 })
  git(rig.work, "config", "remote.origin.mirror", "true"); git(rig.work, "config", "push.followTags", "true")
  git(f.source, "tag", "-a", "unpublished-tag", "-m", "Do not publish")
  git(rig.work, "config", "push.gpgSign", "true"); git(rig.work, "config", "gpg.program", program)
  git(rig.work, "config", "remote.origin.receivepack", program); git(rig.work, "config", "remote.origin.uploadpack", program)
  const { review } = (yield* prepare(f)).result
  expect((yield* approve(f, review)).result.outcome).toBe("merged")
  expect(existsSync(marker)).toBe(false)
  expect(git(gh.bare, "tag", "--list")).toBe("")
  const calls = readFileSync(gh.gitCalls, "utf8").split('\n').filter(Boolean).map(line => JSON.parse(line) as string[]).filter(args => args.includes("push") || args.includes("fetch"))
  expect(calls).toHaveLength(3)
  for (const args of calls) {
    for (const option of ["protocol.allow=never", "protocol.https.allow=always", "protocol.ssh.allow=always", "protocol.ext.allow=never", "push.gpgSign=false", "core.askPass=", "http.followRedirects=false"]) expect(args).toContain(option)
    if (args.includes("push")) for (const option of ["remote.origin.mirror=false", "push.followTags=false", "--no-mirror", "--no-force", "--receive-pack=git-receive-pack", "--no-follow-tags", "--recurse-submodules=no", "https://github.com/fixture/product.git"]) expect(args).toContain(option)
    else for (const option of ["--upload-pack=git-upload-pack", "--no-tags", "--no-prune", "--no-recurse-submodules", "--refmap=", "+refs/heads/main:refs/remotes/origin/main"]) expect(args).toContain(option)
  }
})).pipe(Effect.orDie))

it.live("s27 R2 m8: a stalled create is bounded after the real branch push", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`); gh.set({ hangAt: "create" })
  const service = makeGitService({ workspaceFolder: rig.root, networkTimeoutMs: 1000 }), review = yield* service.reviewMain(f.source, rig.work)
  const failed = yield* service.prepareDelivery(f.source, review, { title: "Bounded create", body: "Data" }, { _tag: "owner" }, rig.work).pipe(Effect.result)
  expect(failed).toMatchObject({ _tag: "Failure", failure: { reason: "timeout", uncertain: true } })
  expect(git(gh.bare, "rev-parse", `refs/heads/${review.branch}`)).toBe(f.head)
  expect(gh.read().branch).toBeUndefined()
})).pipe(Effect.orDie))

for (const phase of ['prepare', 'approve'] as const) it.live(`s27 R2 m5: an unreachable GitHub timeout has a sentence during ${phase}`, () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv('PATH', `${gh.bin}:${process.env.PATH}`)
  const review = phase === 'approve' ? (yield* prepare(f)).result.review : undefined
  const original = GitProcesses.ghProcess
  const boundary = vi.spyOn(GitProcesses, 'ghProcess').mockImplementation((folder, args, limits, signal) => original(folder, args, { ...limits, timeoutMs: 500 }, signal))
  yield* Effect.addFinalizer(() => Effect.sync(() => boundary.mockRestore()))
  gh.set({ hang: true })
  const reason = phase === 'prepare' ? yield* refusal(prepare(f)) : yield* refusal(approve(f, review))
  expect(reason).toMatch(/GitHub.*[.]$/)
  expect(git(rig.work, 'rev-parse', 'HEAD')).toBe(f.baseHead)
  expect(Object.values(projectView(yield* snapshot).gitAttempts)).toHaveLength(0)
})).pipe(Effect.orDie))

it.live('s27 R2 m5: unreachable approval reports availability instead of claiming a failed pull request condition', () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv('PATH', `${gh.bin}:${process.env.PATH}`)
  const { review } = (yield* prepare(f)).result
  gh.set({ offline: true })
  expect(yield* refusal(approve(f, review))).toContain('availability')
  expect(projectView(yield* snapshot).gitReviews[f.worktreeId]?.outcome).toBe('open')
  expect(git(rig.work, 'rev-parse', 'HEAD')).toBe(f.baseHead)
  expect(readFileSync(gh.calls, 'utf8')).not.toContain('"PUT"')
  gh.set({ offline: false })
  expect((yield* approve(f, review)).result.outcome).toBe('merged')
})).pipe(Effect.orDie))


it.live("s27 R3 M-A: a dirty local base can stay behind while successive hosted reviews use the live base", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  save(rig.work, "owner-notes.txt", "Owner untracked notes\n")
  const first = (yield* prepare(f)).result.review
  const merged = (yield* approve(f, first)).result
  expect(merged.outcome).toBe("merged")
  expect(git(rig.work, "rev-parse", "HEAD")).toBe(f.baseHead)
  save(f.source, "second-project.txt", "Second contribution\n")
  const next = commit(f.source, "Second contribution")
  const prepared = (yield* prepare(f)).result
  expect(prepared.review).toMatchObject({ head: next, baseHead: merged.mergeHead, delivery: { remoteBaseHead: merged.mergeHead } })
  expect(prepared.comparison.commits.items.map((row: any) => row.title)).toEqual(["Second contribution"])
  expect(prepared.comparison.files.items.map((row: any) => row.path)).toEqual(["second-project.txt"])
  expect(prepared.comparison.diff.patch).not.toContain("actual feature")
  expect((yield* approve(f, prepared.review)).result.outcome).toBe("merged")
  expect(git(rig.work, "rev-parse", "HEAD")).toBe(f.baseHead)
  expect(readFileSync(join(rig.work, "owner-notes.txt"), "utf8")).toBe("Owner untracked notes\n")
})).pipe(Effect.orDie))

it.live("s27 R3 m-1: Prepare records an externally merged request after a prior verified refusal", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  const { review } = (yield* prepare(f)).result
  gh.set({ mergeStateStatus: "BLOCKED" })
  expect((yield* approve(f, review)).result.outcome).toBe("refused")
  gh.set({ mergeStateStatus: "CLEAN" })
  const merged = JSON.parse(execFileSync(join(gh.bin, "gh"), ["api", "--method", "PUT", "repos/fixture/product/pulls/17/merge", "-f", `sha=${review.head}`], { cwd: f.source, encoding: "utf8" }))
  git(gh.bare, "update-ref", "-d", `refs/heads/${review.branch}`)
  const recovered = (yield* prepare(f)).result.review
  expect(git(gh.bare, "branch", "--list", review.branch)).toBe("")
  expect(recovered).toMatchObject({ outcome: "merged", mergeHead: merged.sha, decision: null, delivery: { state: "MERGED" } })
  expect(projectView(yield* snapshot).gitReviews[f.worktreeId]?.outcome).toBe("merged")
  expect(git(rig.work, "rev-parse", "HEAD")).toBe(merged.sha)
  const calls = readFileSync(gh.calls, "utf8").split("\n").filter(Boolean).map(line => JSON.parse(line) as string[])
  expect(calls.filter(args => args.includes("create"))).toHaveLength(1)
  expect(calls.filter(args => args.includes("PUT"))).toHaveLength(1)
})).pipe(Effect.orDie))

it.live("s27 R3 m-2: hosted synchronization updates only the explicit remote tracking base", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  git(rig.work, "config", "branch.main.remote", "origin"); git(rig.work, "config", "branch.main.merge", "refs/heads/main")
  git(rig.work, "config", "remote.origin.fetch", "+refs/heads/*:refs/heads/unsafe/*")
  const { review } = (yield* prepare(f)).result
  const result = (yield* approve(f, review)).result
  expect(git(rig.work, "rev-parse", "refs/remotes/origin/main")).toBe(result.mergeHead)
  expect(yield* makeGitService({ workspaceFolder: rig.root }).state(rig.work)).toMatchObject({ ahead: 0, behind: 0 })
  expect(git(rig.work, "for-each-ref", "--format=%(refname)", "refs/heads/unsafe/")).toBe("")
})).pipe(Effect.orDie))

for (const authState of ["error", "timeout"]) it.live(`s27 R3 m-3: zero-exit authentication ${authState} refuses with no fallback`, () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`); gh.set({ authState })
  expect(yield* refusal(prepare(f))).toContain("availability")
  expect(projectView(yield* snapshot).gitReviews[f.worktreeId]).toBeUndefined()
  expect(readFileSync(gh.gitCalls, "utf8")).not.toContain('"push"')
})).pipe(Effect.orDie))

it.live("s27 R3 m-3: a merged request with another head never reconciles", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  const { review } = (yield* prepare(f)).result
  gh.set({ state: "MERGED", mergeCommit: f.head, headOverride: f.baseHead })
  expect((yield* approve(f, review)).result).toMatchObject({ outcome: "refused", reason: "stale_state" })
  expect(git(rig.work, "rev-parse", "HEAD")).toBe(f.baseHead)
  expect(readFileSync(gh.calls, "utf8")).not.toContain('"PUT"')
})).pipe(Effect.orDie))

it.live("s27 R3 m-3: Prepare reconciles a same-head squash merged by the owner elsewhere", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  const { review } = (yield* prepare(f)).result
  const tree = git(gh.bare, "rev-parse", `${f.head}^{tree}`)
  const squash = git(gh.bare, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit-tree", tree, "-p", f.baseHead, "-m", "Owner squash")
  git(gh.bare, "update-ref", "refs/heads/main", squash)
  gh.set({ state: "MERGED", mergeCommit: squash, mergedHead: review.head })
  const recovered = (yield* prepare(f)).result.review
  expect(recovered).toMatchObject({ outcome: "merged", mergeHead: squash })
  expect(git(rig.work, "rev-list", "--parents", "-n", "1", "HEAD").split(" ")).toHaveLength(2)
})).pipe(Effect.orDie))

it.live("s27 R3 m-3: the live reference endpoint encodes a slash in the base name", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  git(gh.bare, "update-ref", "refs/heads/release/next", f.baseHead)
  expect(yield* Effect.promise(() => githubCommands(f.source, new AbortController().signal, 5000).baseHead("fixture/product", "release/next"))).toBe(f.baseHead)
  expect(readFileSync(gh.calls, "utf8")).toContain("git/ref/heads/release%2Fnext")
})).pipe(Effect.orDie))

it.live("s27 R3 m-4: reusing a pull request refreshes its redacted description through stdin", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  yield* prepare(f)
  save(f.source, "later-description.txt", "Later\n"); const next = commit(f.source, "Later description")
  const prepared = (yield* prepare(f)).result.review
  expect(gh.read().body).toContain(`Reviewed head: ${next}`)
  expect(gh.read().body).not.toContain(`Reviewed head: ${f.head}`)
  expect(prepared.delivery.body).toBe(gh.read().body)
  const args = readFileSync(gh.calls, "utf8").split("\n").filter(Boolean).map(line => JSON.parse(line) as string[]).find(args => args.includes("edit"))
  expect(args).toEqual(["pr", "edit", "17", "--repo", "fixture/product", "--body-file", "-"])
})).pipe(Effect.orDie))

it.live("s27 R3 m-5: a canonical repository name change tells the owner to update the remote URL", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`); gh.set({ repositoryName: "fixture/renamed" })
  expect(yield* refusal(prepare(f))).toContain("remote URL")
})).pipe(Effect.orDie))

it.live("s27 R3 m-6: an explicit create refusal is determinate after a successful branch push", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`); gh.set({ createRefused: true, secret: "synthetic-create-secret-987654" })
  expect(yield* refusal(prepare(f))).toMatch(/GitHub.*creat.*[.]$/)
  expect(Object.values(projectView(yield* snapshot).gitAttempts)).toHaveLength(0)
  expect(git(gh.bare, "rev-parse", `refs/heads/${git(f.source, "branch", "--show-current")}`)).toBe(f.head)
  expect(JSON.stringify(yield* readLog)).not.toContain("synthetic-create-secret-987654")
})).pipe(Effect.orDie))


it.live("s27 R3 M-A: sign-out between comparison and delivery cannot label a remote-base comparison as local", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  const remoteBase = git(gh.bare, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit-tree", `${f.baseHead}^{tree}`, "-p", f.baseHead, "-m", "Remote advance")
  git(gh.bare, "update-ref", "refs/heads/main", remoteBase)
  const original = GitProcesses.ghProcess
  let authQueries = 0
  const boundary = vi.spyOn(GitProcesses, "ghProcess").mockImplementation((folder, args, limits, signal, input) => {
    if (args[0] === "auth" && ++authQueries === 2) gh.set({ signedOut: true })
    return original(folder, args, limits, signal, input)
  })
  yield* Effect.addFinalizer(() => Effect.sync(() => boundary.mockRestore()))
  expect(yield* refusal(prepare(f))).toBe("Git: stale_state")
  expect(projectView(yield* snapshot).gitReviews[f.worktreeId]).toBeUndefined()
})).pipe(Effect.orDie))

it.live("s27 R4: Prepare mirrors a rewritten base into only its selected tracking ref", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  const unrelated = git(rig.work, "commit-tree", `${f.baseHead}^{tree}`, "-m", "Diverged tracking ref")
  git(rig.work, "update-ref", "refs/remotes/origin/main", unrelated)
  expect((yield* prepare(f)).result.review.baseHead).toBe(f.baseHead)
  expect(git(rig.work, "rev-parse", "refs/remotes/origin/main")).toBe(f.baseHead)
  const fetch = readFileSync(gh.gitCalls, "utf8").split("\n").filter(Boolean).map(line => JSON.parse(line) as string[]).find(args => args.includes("fetch"))!
  expect(fetch).toContain("+refs/heads/main:refs/remotes/origin/main")
  expect(git(rig.work, "rev-parse", "HEAD")).toBe(f.baseHead)
})).pipe(Effect.orDie))

it.live("s27 R4: a successful URL push updates only its project tracking ref without following a symbolic ref", () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  const branch = git(f.source, "branch", "--show-current"), tracking = `refs/remotes/origin/${branch}`
  git(rig.work, "config", `branch.${branch}.remote`, "origin")
  git(rig.work, "config", `branch.${branch}.merge`, `refs/heads/${branch}`)
  git(rig.work, "symbolic-ref", tracking, "refs/heads/main")
  yield* prepare(f)
  expect(git(f.source, "rev-parse", tracking)).toBe(f.head)
  expect(git(f.source, "for-each-ref", "--format=%(symref)", tracking)).toBe("")
  expect(yield* makeGitService({ workspaceFolder: rig.root }).state(f.source)).toMatchObject({ upstream: `origin/${branch}`, ahead: 0, behind: 0 })
  expect(git(rig.work, "rev-parse", "HEAD")).toBe(f.baseHead)
})).pipe(Effect.orDie))

for (const mode of ["local", "signed-out", "github"]) it.live(`s27 R4: comparison timeout reports the selected delivery context (${mode})`, () => provideBoard(rig, Effect.gen(function* () {
  const f = yield* setup(), gh = mode === "local" ? undefined : fakeGithub(rig.root, rig.work)
  if (gh) vi.stubEnv("PATH", `${gh.bin}:${process.env.PATH}`)
  if (mode === "signed-out") gh!.set({ signedOut: true })
  const original = GitProcesses.gitProcess
  const boundary = vi.spyOn(GitProcesses, "gitProcess").mockImplementation((folder, args, limits, signal, truncate) => {
    if (args.includes("diff")) return Promise.reject(new GitFailure({ reason: "timeout" }))
    return original(folder, args, limits, signal, truncate)
  })
  yield* Effect.addFinalizer(() => Effect.sync(() => boundary.mockRestore()))
  expect(yield* refusal(prepare(f))).toBe(mode === "github"
    ? "GitHub could not complete or verify the request. Check its availability, authentication, and pull request status, then refresh."
    : "Git: timeout")
  expect(Object.values(projectView(yield* snapshot).gitAttempts)).toHaveLength(0)
  boundary.mockRestore()
  expect((yield* prepare(f)).result.review.outcome).toBe("open")
})).pipe(Effect.orDie))
