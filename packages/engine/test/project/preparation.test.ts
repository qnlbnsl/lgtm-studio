import { chmodSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, statSync, symlinkSync, truncateSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { it as effectIt } from "@effect/vitest";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import * as Effect from "effect/Effect";
import * as Schema from "effect/Schema";
import * as Redacted from "effect/Redacted";
import { harnessSettings } from "../support/handlerRig.ts";
import { WorktreePreparation } from "@lgtm/contracts";
import { copyPreparationFiles, prepareWorktree } from "../../src/project/preparation.ts";
import * as Processes from "../../src/project/git/process.ts";
import { makeGitService } from "../../src/project/git/index.ts";
import { Views } from "../../src/views/Views.ts";
import { projectView } from "../../src/project/view.ts";
import { makeFakeRunner, type FakeRunner } from "../support/fakeRunner.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";
import { call, failureOf, freshRequest, makeFakeSecrets, makeRig, ownerCredential, provideRig, readLog, removeRig, seedAccount, seedHarness, success, type Rig } from "../support/handlerRig.ts";
import { workerCommit, workerGit as git, workerPayload, workerProject } from "../support/workerWorktrees.ts";

let root: string, source: string, destination: string;
beforeEach(() => {
  root = mkdtempSync(join(tmpdir(), "lgtm-preparation-")); source = join(root, "source"); destination = join(root, "destination");
  mkdirSync(source); mkdirSync(destination);
});
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllEnvs(); rmSync(root, { recursive: true, force: true }); });
registerRebuildCheck({ noLog: true });

for (const pattern of ["", "/etc/passwd", "../secret", "env/../secret", "env/./a", "env//a", ".git/config", "env/.GIT/config", "C:/secret", "env\\a", "env/a\0b", "env/a\nb", "env/??", "env/[ab]", "env/{a}", "env/**", "env/*/*.env", "a".repeat(257)]) {
  it(`refuses unsafe preparation pattern ${JSON.stringify(pattern)}`, () => {
    expect(() => Schema.decodeUnknownSync(WorktreePreparation)({ copyPaths: [pattern] })).toThrow();
  });
}
it("bounds settings and accepts plain text, a plain path and a wildcard in one segment", () => {
  const decode = Schema.decodeUnknownSync(WorktreePreparation);
  expect(decode({ copyPaths: ["env/*.env", "env/local.env"], instructions: "run pnpm install\nbefore building", submodules: true })).toBeDefined();
  for (const settings of [{ copyPaths: Array(17).fill("a") }, { instructions: "a".repeat(8193) }, { instructions: "💡".repeat(2049) }, { instructions: "a\0b" }, { instructions: "\x1b[31m" }]) expect(() => decode(settings)).toThrow();
});
it("copies wildcard matches privately, preserves owner modes, skips missing and nonfiles, and never overwrites", async () => {
  mkdirSync(join(source, "env")); writeFileSync(join(source, "env", "local.env"), "synthetic-env\n"); chmodSync(join(source, "env", "local.env"), 0o750);
  writeFileSync(join(source, "env", "readme.txt"), "skip"); mkdirSync(join(source, "env", "folder.env"));
  expect(await copyPreparationFiles(source, destination, ["env/*.env", "env/local.env", "missing/x"])).toEqual({ copied: 1, skipped: 0 });
  const file = join(destination, "env", "local.env");
  expect(readFileSync(file, "utf8")).toBe("synthetic-env\n"); expect(statSync(file).mode & 0o777).toBe(0o700);
  expect(existsSync(join(destination, "env", "readme.txt"))).toBe(false);
  writeFileSync(file, "keep");
  expect(await copyPreparationFiles(source, destination, ["env/*.env"])).toEqual({ copied: 0, skipped: 1 });
  expect(readFileSync(file, "utf8")).toBe("keep");
});
it("supports a wildcard in an intermediate segment", async () => {
  mkdirSync(join(source, "env-one")); writeFileSync(join(source, "env-one", "local.env"), "one");
  expect(await copyPreparationFiles(source, destination, ["env-*/local.env"])).toEqual({ copied: 1, skipped: 0 });
});
for (const end of ["source", "destination"]) for (const position of ["leaf", "parent", "root"]) {
  it(`refuses a symbolic link at the ${end} ${position}`, async () => {
    const outside = join(root, "outside"); mkdirSync(outside); writeFileSync(join(outside, "local.env"), "keep");
    const target = end === "source" ? source : destination;
    if (position === "root") { rmSync(target, { recursive: true }); symlinkSync(outside, target); }
    else if (position === "parent") symlinkSync(outside, join(target, "env"));
    else { mkdirSync(join(target, "env")); symlinkSync(join(outside, "local.env"), join(target, "env", "local.env")); }
    const other = end === "source" ? destination : source;
    mkdirSync(join(other, "env")); writeFileSync(join(other, "env", "local.env"), "synthetic-env");
    await expect(copyPreparationFiles(source, destination, ["env/local.env"])).rejects.toThrow();
    expect(readFileSync(join(outside, "local.env"), "utf8")).toBe("keep");
  });
}
it("refuses wildcard matches through a source symbolic link and a destination directory leaf", async () => {
  writeFileSync(join(root, "outside"), "keep"); symlinkSync(join(root, "outside"), join(source, "local.env"));
  await expect(copyPreparationFiles(source, destination, ["*.env"])).rejects.toThrow();
  rmSync(join(source, "local.env")); writeFileSync(join(source, "local.env"), "copy"); mkdirSync(join(destination, "local.env"));
  await expect(copyPreparationFiles(source, destination, ["local.env"])).rejects.toThrow();
});
it("bounds file count, file size, aggregate bytes, and wildcard enumeration", async () => {
  const file = join(source, "big"); writeFileSync(file, ""); truncateSync(file, 1024 * 1024 + 1);
  await expect(copyPreparationFiles(source, destination, ["big"])).rejects.toThrow(); rmSync(file);
  for (let i = 0; i < 129; i++) writeFileSync(join(source, `${i}.env`), "");
  await expect(copyPreparationFiles(source, destination, ["*.env"])).rejects.toThrow();
  rmSync(source, { recursive: true }); mkdirSync(source);
  for (let i = 0; i < 9; i++) { writeFileSync(join(source, `${i}.bin`), ""); truncateSync(join(source, `${i}.bin`), 1024 * 1024); }
  await expect(copyPreparationFiles(source, destination, ["*.bin"])).rejects.toThrow();
  for (let i = 0; i < 513; i++) writeFileSync(join(source, `${i}.txt`), "");
  await expect(copyPreparationFiles(source, destination, ["*.absent"])).rejects.toThrow();
});

const initialize = (path: string) => {
  git(path, "init", "-b", "main"); git(path, "config", "user.name", "Test"); git(path, "config", "user.email", "test@example.invalid"); git(path, "config", "commit.gpgsign", "false");
  return workerCommit(path, "initial.txt", "initial\n");
};
const localSubmodule = () => {
  const first = initialize(source); const second = workerCommit(source, "second.txt", "second\n");
  initialize(destination);
  git(destination, "-c", "protocol.file.allow=always", "submodule", "add", "--", source, "tf");
  git(join(destination, "tf"), "checkout", first); git(destination, "add", "."); git(destination, "commit", "-m", "Submodule");
  return { first, second };
};
it("initializes a submodule from a local repository using the fixed production arguments", async () => {
  const { first, second } = localSubmodule();
  const worktree = join(root, "worktree"); git(destination, "worktree", "add", "-b", "project", worktree);
  const real = Processes.gitProcess;
  const spy = vi.spyOn(Processes, "gitProcess").mockImplementation((cwd, args, ...rest) => real(cwd, [...args.slice(0, -4), "-c", "protocol.file.allow=always", ...args.slice(-4)], ...rest));
  const detail = await Effect.runPromise(prepareWorktree(destination, worktree, { submodules: true }));
  expect(detail).toContain("Submodules initialized."); expect(readFileSync(join(worktree, "tf", "initial.txt"), "utf8")).toBe("initial\n");
  const args = spy.mock.calls[0]![1];
  spy.mockRestore();
  const service = makeGitService({ workspaceFolder: root });
  git(join(worktree, "tf"), "checkout", second);
  expect((await Effect.runPromise(service.changes(worktree))).items[0]).toMatchObject({ path: "tf", submodule: { oldHead: first, newHead: second } });
  await Effect.runPromise(service.stage(worktree, "tf")); await Effect.runPromise(service.unstage(worktree, "tf"));
  const plan = await Effect.runPromise(service.planDiscard(worktree, ["tf"]));
  await Effect.runPromise(service.discard(worktree, plan, true));
  expect(git(join(worktree, "tf"), "rev-parse", "HEAD")).toBe(first);
  expect(args).toContain("--init"); expect(args).toContain("--checkout"); expect(args).not.toContain("--recursive"); expect(args).not.toContain("--remote");
});
it("ignores repository-authored custom update commands and records timeout failures without their diagnostics", async () => {
  localSubmodule();
  const worktree = join(root, "custom-worktree"); git(destination, "worktree", "add", "-b", "custom", worktree);
  const marker = join(root, "executed");
  git(destination, "config", "submodule.tf.update", `!touch ${marker}`);
  const real = Processes.gitProcess;
  vi.spyOn(Processes, "gitProcess").mockImplementation((cwd, args, ...rest) => real(cwd, [...args.slice(0, -4), "-c", "protocol.file.allow=always", ...args.slice(-4)], ...rest));
  expect(await Effect.runPromise(prepareWorktree(destination, worktree, { submodules: true }))).toContain("Submodules initialized.");
  expect(existsSync(marker)).toBe(false);
  vi.mocked(Processes.gitProcess).mockRejectedValue(new Error("private timeout diagnostic"));
  const detail = await Effect.runPromise(prepareWorktree(destination, worktree, { submodules: true }));
  expect(detail).toContain("Submodule initialization failed"); expect(detail).not.toContain("private timeout");
});
it("handles status, aggregate and path diffs, stage, review, and safe discard for a changed gitlink", async () => {
  const { first, second } = localSubmodule(); git(destination, "checkout", "-b", "project"); git(join(destination, "tf"), "checkout", second);
  writeFileSync(join(destination, "initial.txt"), "changed\n");
  const service = makeGitService({ workspaceFolder: root });
  const run = Effect.runPromise;
  expect((await run(service.changes(destination))).items.find(row => row.path === "tf")).toMatchObject({ submodule: { oldHead: first, newHead: second } });
  const diff = await run(service.diff(destination)); expect(diff.patch).toContain(`+Subproject commit ${second}`); expect(diff.patch).toContain("+changed");
  expect((await run(service.diff(destination, { path: "tf", agentRead: true }))).patch).toContain(`-Subproject commit ${first}`);
  expect((await run(service.listFolder(destination))).items.map(row => row.path)).not.toContain("tf");
  await run(service.stage(destination, "tf")); expect((await run(service.diff(destination, { staged: true }))).patch).toContain(second);
  await run(service.unstage(destination, "tf")); await run(service.stageAll(destination)); await run(service.unstageAll(destination));
  let plan = await run(service.planDiscard(destination, ["tf"]));
  writeFileSync(join(destination, "tf", "untracked.txt"), "keep");
  await expect(run(service.discard(destination, plan, true))).rejects.toMatchObject({ reason: "dirty_source" });
  rmSync(join(destination, "tf", "untracked.txt"));
  plan = await run(service.planDiscard(destination, ["tf"])); await run(service.discard(destination, plan, true));
  expect(git(join(destination, "tf"), "rev-parse", "HEAD")).toBe(first); expect(readFileSync(join(destination, "initial.txt"), "utf8")).toBe("changed\n");
  git(join(destination, "tf"), "checkout", second); await run(service.stage(destination, "tf")); git(destination, "commit", "-m", "Move pointer");
  const review = await run(service.review(destination, "main", true));
  expect(review.files.items).toMatchObject([{ path: "tf", submodule: { oldHead: first, newHead: second } }]); expect(review.diff.patch).toContain(second);
  expect((await run(service.diff(destination, { commit: "HEAD", agentRead: true }))).patch).toContain(second);
});

it("refuses to overwrite an ignored local file while discarding a submodule pointer", async () => {
  const { first, second } = localSubmodule(); const module = join(destination, "tf");
  git(module, "checkout", second); git(destination, "add", "tf"); git(module, "checkout", first);
  const ignore = join(root, "module-ignore"); writeFileSync(ignore, "second.txt\n"); git(module, "config", "core.excludesFile", ignore);
  writeFileSync(join(module, "second.txt"), "keep ignored local data\n");
  const service = makeGitService({ workspaceFolder: root });
  const plan = await Effect.runPromise(service.planDiscard(destination, ["tf"]));
  await expect(Effect.runPromise(service.discard(destination, plan, true))).rejects.toMatchObject({ reason: "git_failed" });
  expect(readFileSync(join(module, "second.txt"), "utf8")).toBe("keep ignored local data\n");
  expect(git(module, "rev-parse", "HEAD")).toBe(first);
});
it("diffs an added gitlink before the parent repository has its first commit", async () => {
  const first = initialize(source); git(destination, "init", "-b", "main");
  git(destination, "-c", "protocol.file.allow=always", "submodule", "add", "--", source, "tf");
  const service = makeGitService({ workspaceFolder: root });
  expect((await Effect.runPromise(service.diff(destination, { agentRead: true }))).patch).toContain(`+Subproject commit ${first}`);
  expect((await Effect.runPromise(service.diff(destination, { path: "tf" }))).patch).toContain(first);
  expect((await Effect.runPromise(service.changes(destination))).items.find(row => row.path === "tf")).toMatchObject({ submodule: { oldHead: null, newHead: first } });
});
it("keeps removed and uninitialized gitlinks out of ordinary file reads", async () => {
  const { first } = localSubmodule();
  const service = makeGitService({ workspaceFolder: root }); const run = Effect.runPromise;
  git(destination, "checkout", "-b", "removed");
  git(destination, "rm", "--cached", "tf");
  // The nested checkout can remain after staging its removal.
  expect((await run(service.changes(destination))).items).toMatchObject([{ path: "tf", submodule: { oldHead: first, newHead: null } }]);
  expect((await run(service.diff(destination, { agentRead: true }))).patch).toContain(`-Subproject commit ${first}`);
  git(destination, "commit", "-m", "Remove pointer");
  expect((await run(service.diff(destination, { commit: "HEAD", agentRead: true }))).patch).toContain(first);
  const review = await run(service.review(destination, "main", true));
  expect(review.files.items).toMatchObject([{ path: "tf", submodule: { oldHead: first, newHead: null } }]);
  git(destination, "checkout", "main"); git(destination, "submodule", "deinit", "-f", "--", "tf");
  expect((await run(service.diff(destination))).patch).toBe("");
  expect((await run(service.listFolder(destination))).items.map(row => row.path)).not.toContain("tf");
});

const integrated = (test: (rig: Rig, runner: FakeRunner) => Effect.Effect<void, unknown, any>) => Effect.gen(function* () {
  const rig = makeRig(); vi.stubEnv("HOME", rig.home); vi.stubEnv("LGTM_HOME", rig.stateFolder);
  const runner = yield* makeFakeRunner();
  try { yield* provideRig(rig, { runner, secrets: makeFakeSecrets() }, Effect.gen(function* () { yield* registerRebuildCheck(); yield* test(rig, runner); })); }
  finally { removeRig(rig); }
}).pipe(Effect.scoped, Effect.orDie);
effectIt.live("owner preparation reaches project and worker worktrees and their started prompts; submodule failure is nonfatal", () => integrated((rig, runner) => Effect.gen(function* () {
  const p = yield* workerProject(rig);
  mkdirSync(join(rig.work, "env")); writeFileSync(join(rig.work, "env", "local.env"), "synthetic-env");
  // Ignore the build files so the base checkout remains eligible for creation.
  writeFileSync(join(rig.work, ".git", "info", "exclude"), "/.worktrees/\n/env/\n");
  const preparation = { copyPaths: ["env/*.env"], submodules: true, instructions: "Run pnpm install before building." };
  expect(failureOf(yield* call(p.leadCredential, "project.repository.preparation.set", { repoId: p.repoId, preparation }))).toBeDefined();
  success(yield* call(ownerCredential, "project.repository.preparation.set", { repoId: p.repoId, preparation }));
  expect(success(yield* call(ownerCredential, "project.repositories", {})).items).toMatchObject([{ preparation }]);
  const real = Processes.gitProcess;
  vi.spyOn(Processes, "gitProcess").mockImplementation((cwd, args, ...rest) => args.includes("submodule") ? Promise.resolve({ code: 1, stdout: "", stderr: "private diagnostic", truncated: false }) : real(cwd, args, ...rest));
  const worker = success(yield* call(p.leadCredential, "start", workerPayload(p.path, { prompt: "Build it." })));
  expect(worker.detail).toContain("Submodule initialization failed");
  const state = yield* (yield* Views).snapshot;
  expect(readFileSync(join(state.agents[worker.agentId as string]!.folder, "env", "local.env"), "utf8")).toBe("synthetic-env");
  yield* seedAccount("lead-account", "cursor"); yield* seedHarness("cursor");
  success(yield* call(ownerCredential, "harnesses", { operation: "set", harness: harnessSettings("cursor", { adapterPath: process.execPath, reportedModes: [{ id: "default", level: "ask" }, { id: "acceptEdits", level: "edit" }] }) }));
  success(yield* call(ownerCredential, "accounts set-secret", { accountId: "lead-account", name: "CURSOR_API_KEY", nonce: "preparation-key", value: Redacted.make("synthetic-preparation-key-012345") }));
  const request = { requestId: freshRequest(), name: "Prepared project", goal: "Build", machineId: "local", repository: { _tag: "existing", repoId: p.repoId }, lead: { harness: "cursor", account: { _tag: "any" }, permission: "ask" } };
  const created = success(yield* call(ownerCredential, "project.create", request));
  const view = projectView(yield* (yield* Views).snapshot), project = view.projects[created.projectId as string]!;
  expect(readFileSync(join(view.workspaces[project.worktreeId]!.path, "env", "local.env"), "utf8")).toBe("synthetic-env");
  expect(runner.callsOf("start")).toHaveLength(2);
  for (const start of runner.callsOf("start")) expect(start.input.instructions).toContain("Owner's repository settings\nRun pnpm install before building.");
  const log = yield* readLog;
  expect(log.filter(row => row.event._tag === "project.CreationFinished").at(-1)?.event).toMatchObject({ detail: expect.stringContaining("Submodule initialization failed") });
  expect(JSON.stringify(log)).not.toContain("private diagnostic"); expect(JSON.stringify(log)).not.toContain("synthetic-env");
})));
