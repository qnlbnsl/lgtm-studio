import { execFileSync } from "node:child_process"
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs"
import { join } from "node:path"
import { it } from "@effect/vitest"
import * as Effect from "effect/Effect"
import { afterEach, beforeEach, expect, vi } from "vitest"
import { CommandHandler } from "../../src/commands/CommandHandler.ts"
import { prepareRepositoryBases } from "../../src/project/baseBranch.ts"
import { makeGitService, GitFailure } from "../../src/project/git/index.ts"
import { githubCommands } from "../../src/project/git/github.ts"
import * as Processes from "../../src/project/git/process.ts"
import { projectView } from "../../src/project/view.ts"
import { Views } from "../../src/views/Views.ts"
import { provideBoard } from "../board/wakeFixtures.ts"
import { fakeGithub } from "../support/fakeGithub.ts"
import { registerRebuildCheck } from "../support/rebuildCheck.ts"
import { agentCredential, appendSetup, call, eventOf, failureOf, makeRig, ownerCredential as owner,
  readLog, removeRig, seedAgent, success, type Rig } from "../support/handlerRig.ts"

let rig: Rig
const git = (folder: string, ...args: string[]) => execFileSync("/usr/bin/git", ["-C", folder, ...args], { encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] }).trim()
const snapshot = Effect.gen(function* () { return yield* (yield* Views).snapshot })
const service = () => makeGitService({ workspaceFolder: rig.root })
const conventional = "without a usable local remote default, registration requires main or master to be the current branch, or the current branch to be unborn"
beforeEach(() => {
  rig = makeRig(); vi.stubEnv("HOME", rig.home); vi.stubEnv("LGTM_HOME", rig.stateFolder)
  git(rig.work, "init", "-b", "main")
  git(rig.work, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "Initial")
})
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllEnvs(); removeRig(rig) })
const configure = () => call(owner, "project.settings.set", { settings: { workspaceFolder: rig.root, shell: "sh" } }).pipe(Effect.map(success))
const legacy = () => appendSetup([
  eventOf({ _tag: "project.RepositoryAdded", repoId: "legacy", name: "Legacy", path: rig.work }),
  eventOf({ _tag: "project.RepositoryLocated", repoId: "legacy", machineId: "local", path: rig.work })
])

it.live("round 2: an old row on plain main pins only on an owner call without rewriting history", () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  yield* configure(); const [original] = yield* legacy()
  expect(projectView(yield* snapshot).repositories.legacy?.baseBranch).toBeUndefined()
  const opening = success(yield* call(owner, "client.sequence", {}))
  expect(success(yield* call(owner, "project.repositories", {})).items).toMatchObject([{ baseBranch: "main" }])
  expect(success(yield* call(owner, "client.sequence", {})).sequence).toBe(opening.sequence)
  const log = yield* readLog
  expect(log.find((row) => row.sequence === original!.sequence)?.event).toEqual(original!.event)
  expect(log.filter((row) => row.event._tag === "project.RepositoryBaseChanged")).toMatchObject([{ caller: { _tag: "owner" } }])
})).pipe(Effect.orDie))

for (const checkout of ["feature-x", "detached"]) it.live(`round 2: unresolved ${checkout} legacy rows keep snapshots readable and cache the failed inference`, () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  yield* configure(); yield* legacy()
  git(rig.work, "checkout", ...(checkout === "detached" ? ["--detach"] : ["-b", checkout]))
  const observed = vi.spyOn(Processes, "gitProcess")
  success(yield* call(owner, "client.snapshot", {}))
  const first = observed.mock.calls.length
  expect(first).toBeGreaterThan(0)
  success(yield* call(owner, "client.snapshot", {}))
  success(yield* call(owner, "project.repositories", {}))
  expect(observed.mock.calls).toHaveLength(first)
  expect(projectView(yield* snapshot).repositories.legacy?.baseBranch).toBeUndefined()
  expect(yield* service().repositoryBase(rig.work)).toBe("main")
})).pipe(Effect.orDie))

it.live("round 2: an inference refusal cannot fail an otherwise readable owner snapshot", () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  yield* configure(); yield* legacy()
  vi.spyOn(Processes, "gitProcess").mockImplementation(() => Promise.reject(new GitFailure({ reason: "git_failed" })))
  success(yield* call(owner, "client.snapshot", {}))
  expect(projectView(yield* snapshot).repositories.legacy?.baseBranch).toBeUndefined()
})).pipe(Effect.orDie))

it.live("round 2: an agent read cannot pin a legacy base or trust a rewritten origin HEAD", () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  yield* configure(); yield* legacy()
  const worker = yield* seedAgent({ id: "legacy-worker", folder: rig.work })
  yield* appendSetup([
    eventOf({ _tag: "project.WorktreeCreated", worktreeId: "legacy-work", repoId: "legacy", path: rig.work, branch: "main", machineId: "local", checkout: "main" }),
    eventOf({ _tag: "project.Created", projectId: "legacy-project", repoId: "legacy", worktreeId: "legacy-work", name: "Legacy", goal: "Read", leadAgentId: worker.agentId, machineId: "local" })
  ])
  git(rig.work, "branch", "feature-x")
  git(rig.work, "update-ref", "refs/remotes/origin/feature-x", git(rig.work, "rev-parse", "HEAD"))
  git(rig.work, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/feature-x")
  success(yield* call(agentCredential(worker.token), "project.changes", { projectId: "legacy-project", worktreeId: "legacy-work" }))
  expect(projectView(yield* snapshot).repositories.legacy?.baseBranch).toBeUndefined()
  expect((yield* readLog).filter((row) => row.event._tag === "project.RepositoryBaseChanged")).toEqual([])
  expect(yield* service().repositoryBase(rig.work)).toBe("main")
  success(yield* call(owner, "client.snapshot", {}))
  expect(projectView(yield* snapshot).repositories.legacy?.baseBranch).toBe("main")
  expect((yield* readLog).filter((row) => row.event._tag === "project.RepositoryBaseChanged")).toMatchObject([{ caller: { _tag: "owner" }, event: { baseBranch: "main" } }])
})).pipe(Effect.orDie))

it.live("round 2: a conflicting raw repoId cannot infer a different repository through a project call", () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  yield* configure(); yield* legacy()
  const worker = yield* seedAgent({ id: "scoped-worker", folder: rig.work })
  yield* appendSetup([
    eventOf({ _tag: "project.RepositoryAdded", repoId: "other", name: "Other", path: rig.work }),
    eventOf({ _tag: "project.RepositoryLocated", repoId: "other", machineId: "local", path: rig.work }),
    eventOf({ _tag: "project.Created", projectId: "legacy-project", repoId: "legacy", worktreeId: "legacy-work", name: "Legacy", goal: "Read", leadAgentId: worker.agentId, machineId: "local" })
  ])
  const handler = yield* CommandHandler, observed = vi.spyOn(Processes, "gitProcess")
  yield* prepareRepositoryBases(yield* handler.input({ _tag: "owner" }), "project.changes", { projectId: "legacy-project", repoId: "other" })
  expect(observed).not.toHaveBeenCalled()
  expect((yield* readLog).filter((row) => row.event._tag === "project.RepositoryBaseChanged")).toEqual([])
  yield* prepareRepositoryBases(yield* handler.input({ _tag: "owner" }), "project.snapshot", { projectId: "legacy-project" })
  expect(projectView(yield* snapshot).repositories.legacy?.baseBranch).toBe("main")
  expect(projectView(yield* snapshot).repositories.other?.baseBranch).toBeUndefined()
})).pipe(Effect.orDie))

for (const current of ["main", "feature-x"]) it.live(`round 2: a missing local origin base ${current === "main" ? "falls back to main" : "refuses feature registration"}`, () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  yield* configure()
  git(rig.work, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/missing")
  if (current !== "main") git(rig.work, "checkout", "-b", current)
  const reply = yield* call(owner, "project.repository.add", { name: "Product", path: rig.work, machineId: "local" })
  if (current === "main") expect(projectView(yield* snapshot).repositories[success(reply).repoId as string]?.baseBranch).toBe("main")
  else expect(failureOf(reply)?.reason).toBe(conventional)
})).pipe(Effect.orDie))

it.live("round 2: registration explains an overlong inferred base before writing its event", () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  yield* configure()
  const name = "b".repeat(129)
  git(rig.work, "branch", name)
  git(rig.work, "symbolic-ref", "refs/remotes/origin/HEAD", `refs/remotes/origin/${name}`)
  expect(failureOf(yield* call(owner, "project.repository.add", { name: "Product", path: rig.work, machineId: "local" }))?.reason).toBe("the repository base branch name must contain at most 128 characters")
  expect(Object.values(projectView(yield* snapshot).repositories)).toEqual([])
})).pipe(Effect.orDie))

it.live("round 2: locating and registering a workspace preserves an explicitly selected unconventional base", () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  yield* configure()
  const { repoId } = success(yield* call(owner, "project.repository.add", { name: "Product", path: rig.work, machineId: "local" }))
  git(rig.work, "branch", "-m", "dev")
  success(yield* call(owner, "project.repository.base.set", { repoId, baseBranch: "dev" }))
  success(yield* call(owner, "project.repository.locate", { repoId, path: rig.work, machineId: "local" }))
  const { worktreeId } = success(yield* call(owner, "project.workspace.register", { repoId, path: rig.work, branch: "dev", checkout: "main", machineId: "local" }))
  expect(projectView(yield* snapshot).workspaces[worktreeId as string]?.branch).toBe("dev")
  expect(projectView(yield* snapshot).repositories[repoId as string]?.baseBranch).toBe("dev")
})).pipe(Effect.orDie))

it.live("round 2: valid at-sign branches resolve while reflog syntax refuses", () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  git(rig.work, "branch", "dev@next")
  const gitService = service()
  yield* gitService.validateBranch(rig.work, "dev@next")
  expect((yield* gitService.review(rig.work, "dev@next")).baseHead).toBe(git(rig.work, "rev-parse", "HEAD"))
  expect((yield* gitService.review(rig.work, "main@{0}").pipe(Effect.result))._tag).toBe("Failure")
})).pipe(Effect.orDie))

it.live("round 2: GitHub base endpoints retain branch path separators", () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  git(rig.work, "branch", "release/next")
  const hosted = fakeGithub(rig.root, rig.work)
  git(hosted.bare, "update-ref", "refs/heads/release/next", git(rig.work, "rev-parse", "HEAD"))
  vi.stubEnv("PATH", `${hosted.bin}:${process.env.PATH}`)
  expect(yield* Effect.promise(() => githubCommands(rig.work, new AbortController().signal, 1000).baseHead("fixture/product", "release/next"))).toBe(git(rig.work, "rev-parse", "HEAD"))
  expect(readFileSync(hosted.calls, "utf8")).toContain('repos/fixture/product/git/ref/heads/release/next')
  expect(readFileSync(hosted.calls, "utf8")).not.toContain('release%2Fnext')
})).pipe(Effect.orDie))

for (const failing of ["fetch", "config", "merge-base"] as const) it.live(`round 2: a nonfatal ${failing} failure records a local starting fallback`, () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  const hosted = fakeGithub(rig.root, rig.work)
  vi.stubEnv("PATH", `${hosted.bin}:${process.env.PATH}`)
  const original = Processes.gitProcess
  vi.spyOn(Processes, "gitProcess").mockImplementation((folder, args, ...rest) => args.includes(failing)
    ? Promise.reject(new GitFailure({ reason: "timeout" })) : original(folder, args, ...rest))
  const start = yield* service().projectStart(rig.work, "main")
  expect(start.head).toBe(git(rig.work, "rev-parse", "HEAD"))
  expect(start.detail).toMatch(/Started from local base main; .*failed/)
})).pipe(Effect.orDie))

it.live("round 2: the creation fetch deadline releases the Git operation with a local fallback", () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  git(rig.work, "remote", "add", "origin", "https://github.com/fixture/product.git")
  const deadline = new AbortController()
  const timeout = AbortSignal.timeout.bind(AbortSignal)
  const deadlines = vi.spyOn(AbortSignal, "timeout").mockImplementation((ms) => ms === 20_000 ? deadline.signal : timeout(ms))
  const original = Processes.gitProcess
  vi.spyOn(Processes, "gitProcess").mockImplementation((folder, args, limits, signal, truncate) => args.includes("fetch")
    ? new Promise((_, reject) => {
      const stop = () => reject(new GitFailure({ reason: "timeout" }))
      if (signal.aborted) stop(); else signal.addEventListener("abort", stop, { once: true })
      deadline.abort()
    }) : original(folder, args, limits, signal, truncate))
  const start = yield* service().projectStart(rig.work, "main")
  expect(deadlines).toHaveBeenCalledWith(20_000)
  expect(start.head).toBe(git(rig.work, "rev-parse", "HEAD"))
  expect(start.detail).toContain("fetching its remote base failed")
})).pipe(Effect.orDie))

for (const branch of ["main", "dev"]) it.live(`round 3: registration accepts unborn ${branch} and returns its recorded base`, () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  yield* configure()
  const folder = join(rig.root, "unborn")
  mkdirSync(folder); git(folder, "init", "-b", branch)
  const reply = success(yield* call(owner, "project.repository.add", { name: "Unborn", path: folder, machineId: "local" }))
  expect(reply.baseBranch).toBe(branch)
  expect(projectView(yield* snapshot).repositories[reply.repoId as string]?.baseBranch).toBe(branch)
  expect((yield* service().state(folder)).head).toBeNull()
})).pipe(Effect.orDie))

for (const value of ["ref: refs/heads/feature-x\n", "ref: refs/remotes/origin/bad..name\n", "malformed\n"]) it.live(`round 3: malformed origin HEAD ${JSON.stringify(value)} falls back at registration`, () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  yield* configure()
  mkdirSync(join(rig.work, ".git/refs/remotes/origin"), { recursive: true })
  writeFileSync(join(rig.work, ".git/refs/remotes/origin/HEAD"), value)
  const reply = success(yield* call(owner, "project.repository.add", { name: "Product", path: rig.work, machineId: "local" }))
  expect(reply.baseBranch).toBe("main")
  expect(projectView(yield* snapshot).repositories[reply.repoId as string]?.baseBranch).toBe("main")
})).pipe(Effect.orDie))

it.live("round 3: a changed checkout invalidates an inference miss on the next collection read", () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  yield* configure(); yield* legacy()
  git(rig.work, "checkout", "--detach")
  success(yield* call(owner, "client.snapshot", {}))
  expect(projectView(yield* snapshot).repositories.legacy?.baseBranch).toBeUndefined()
  git(rig.work, "checkout", "main")
  success(yield* call(owner, "client.snapshot", {}))
  expect(projectView(yield* snapshot).repositories.legacy?.baseBranch).toBe("main")
})).pipe(Effect.orDie))

it.live("round 3: any owner command scoped to a repository retries a cached inference miss", () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  yield* configure(); yield* legacy()
  git(rig.work, "checkout", "--detach")
  const observed = vi.spyOn(Processes, "gitProcess")
  success(yield* call(owner, "client.snapshot", {}))
  const first = observed.mock.calls.length
  success(yield* call(owner, "project.workspaces", { repoId: "legacy" }))
  expect(observed.mock.calls.length).toBeGreaterThan(first)
})).pipe(Effect.orDie))

it.live("round 3: both conventional branches retain legacy integration and review guards", () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  git(rig.work, "branch", "master")
  const source = join(rig.root, "feature"), master = join(rig.root, "master")
  git(rig.work, "worktree", "add", "-b", "feature", source)
  git(rig.work, "worktree", "add", master, "master")
  const gitService = service(), head = git(source, "rev-parse", "HEAD")
  for (const path of [rig.work, master]) {
    const name = git(path, "branch", "--show-current")
    expect(yield* gitService.isBaseBranch(path, name)).toBe(true)
    const integrated = yield* gitService.mergeTeam(source, path, head).pipe(Effect.result)
    expect(integrated).toMatchObject({ _tag: "Failure", failure: { reason: "base_requires_review" } })
    expect((yield* gitService.reviewMain(source, path)).base).toBe(name)
  }
})).pipe(Effect.orDie))

for (const name of ["main", "master"]) it.live(`round 3: Files remains read-only on unrecorded ${name} with the main checkout on a feature branch`, () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  yield* configure(); yield* legacy()
  git(rig.work, "branch", "master")
  git(rig.work, "checkout", "-b", "feature-x")
  const path = join(rig.root, name)
  git(rig.work, "worktree", "add", path, name)
  writeFileSync(join(path, "file.txt"), "Conventional base\n")
  const worker = yield* seedAgent({ id: "legacy-files-worker", folder: path })
  yield* appendSetup([
    eventOf({ _tag: "project.WorktreeCreated", worktreeId: "legacy-work", repoId: "legacy", path, branch: name, machineId: "local", checkout: "worktree" }),
    eventOf({ _tag: "project.Created", projectId: "legacy-project", repoId: "legacy", worktreeId: "legacy-work", name: "Legacy", goal: "Read", leadAgentId: worker.agentId, machineId: "local" })
  ])
  const reply = success(yield* call(owner, "project.files.read", { projectId: "legacy-project", worktreeId: "legacy-work", path: "file.txt" }))
  expect(reply.result).toMatchObject({ kind: "text", editable: false })
  expect(projectView(yield* snapshot).repositories.legacy?.baseBranch).toBeUndefined()
})).pipe(Effect.orDie))

for (const divergent of [false, true]) it.live(`round 4: one merge-base process keeps all project work visible with ${divergent ? "divergent inherited history" : "a later fetched start"}`, () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  const commit = (folder: string, title: string) => {
    git(folder, "add", "--all")
    git(folder, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "commit", "-m", title)
    return git(folder, "rev-parse", "HEAD")
  }
  writeFileSync(join(rig.work, "conflict.txt"), "Original base\n")
  const original = commit(rig.work, "Original base")
  const source = join(rig.root, "project")
  git(rig.work, "worktree", "add", "-b", "project", source)
  writeFileSync(join(source, "upstream.txt"), "Fetched upstream\n")
  writeFileSync(join(source, "conflict.txt"), "Fetched side\n")
  const start = commit(source, "Fetched upstream")
  writeFileSync(join(source, "feature.txt"), "Own new file\n")
  writeFileSync(join(source, "upstream.txt"), "Fetched upstream\nOwn edit\n")
  commit(source, "Project work")
  if (divergent) {
    writeFileSync(join(rig.work, "parent.txt"), "Later local parent update\n")
    writeFileSync(join(rig.work, "conflict.txt"), "Local parent side\n")
    commit(rig.work, "Parent update")
    expect(() => git(source, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "merge", "--no-edit", "main")).toThrow()
    writeFileSync(join(source, "conflict.txt"), "Own conflict resolution\n")
    commit(source, "Resolve inherited conflict")
  }
  const current = git(source, "rev-parse", "HEAD"), target = git(rig.work, "rev-parse", "main")
  // A clean filter must not run when the owner merely opens a comparison.
  const marker = join(rig.root, "filter-ran"), filter = join(rig.root, "clean-filter")
  writeFileSync(filter, `#!${process.execPath}\nrequire('node:fs').writeFileSync(${JSON.stringify(marker)}, 'ran')\n`, { mode: 0o700 })
  git(source, "config", "filter.probe.clean", filter)
  git(source, "config", "merge.renormalize", "true")
  writeFileSync(join(source, ".gitattributes"), "*.txt filter=probe\n")
  const objects = git(rig.work, "count-objects", "-v")
  const observed = vi.spyOn(Processes, "gitProcess")
  const gitService = makeGitService({ workspaceFolder: rig.root, startingHeads: () => ({ [source]: start }) })
  const comparison = yield* gitService.reviewMain(source, rig.work)
  const calls = observed.mock.calls.map(([, args]) => args)
  expect(calls.filter((args) => args[0] === "merge-base")).toEqual([["merge-base", current, target, start]])
  expect(calls.some((args) => args.includes("merge-tree") || args.includes("--get-regexp"))).toBe(false)
  const baseline = calls.find((args) => args[0] === "diff")![1]!
  expect(divergent ? [target, start] : [start]).toContain(baseline)
  if (!divergent) {
    // --octopus would choose the old local base and expose fetched changes.
    expect(git(source, "merge-base", "--octopus", current, target, start)).toBe(original)
    expect(comparison.files.items.map((row) => row.path)).toEqual(["feature.txt", "upstream.txt"])
  }
  expect(comparison.diff.patch).toContain("Own new file")
  expect(comparison.diff.patch).toContain("Own edit")
  if (divergent) expect(comparison.diff.patch).toContain("Own conflict resolution")
  expect(existsSync(marker)).toBe(false)
  expect(git(rig.work, "count-objects", "-v")).toBe(objects)
  expect(comparison.commits.items.map((row) => row.title)).not.toContain("Fetched upstream")
  expect(comparison.commits.items.map((row) => row.title)).not.toContain("Parent update")
  expect(yield* gitService.state(source, "main")).toMatchObject({ pending: divergent ? 2 : 1, behindParent: 0 })
})).pipe(Effect.orDie))

it.live("round 4: registration without origin explains that a conventional branch must be current", () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  yield* configure()
  git(rig.work, "checkout", "-b", "feature-x")
  expect(git(rig.work, "remote")).toBe("")
  const reply = yield* call(owner, "project.repository.add", { name: "Product", path: rig.work, machineId: "local" })
  expect(failureOf(reply)?.reason).toBe(conventional)
  expect(failureOf(reply)?.reason).not.toContain("origin")
})).pipe(Effect.orDie))
