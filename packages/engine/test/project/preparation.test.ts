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
import * as Preparation from "../../src/project/preparation.ts";
import * as Processes from "../../src/project/git/process.ts";
import { makeGitService } from "../../src/project/git/index.ts";
import { Views } from "../../src/views/Views.ts";
import { projectView } from "../../src/project/view.ts";
import { snapshotFor } from "../../src/project/snapshot.ts";
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

for (const pattern of ["", "*", "*/file", ".*", ".*/file", "/etc/passwd", "../secret", "env/../secret", "env/./a", "env//a", ".git/config", "env/.GIT/config", "C:/secret", "env\\a", "env/a\0b", "env/a\nb", "env/??", "env/[ab]", "env/{a}", "env/**", "env/*/*.env", "a".repeat(257)]) {
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
it("skips unsafe wildcard matches and refuses an explicit destination directory leaf with its pattern", async () => {
  writeFileSync(join(root, "outside"), "keep"); symlinkSync(join(root, "outside"), join(source, "local.env"));
  expect(await copyPreparationFiles(source, destination, ["*.env"])).toEqual({ copied: 0, skipped: 1 });
  rmSync(join(source, "local.env")); writeFileSync(join(source, "local.env"), "copy"); mkdirSync(join(destination, "local.env"));
  await expect(copyPreparationFiles(source, destination, ["local.env"])).rejects.toThrow('pattern "local.env"');
});
it("skips invalid wildcard names and does not overlap a wildcard's prefix and suffix", async () => {
  mkdirSync(join(source, "env")); mkdirSync(join(source, "env", ".git"));
  writeFileSync(join(source, "env", "[bad]"), "skip"); writeFileSync(join(source, "env", "good"), "copy");
  writeFileSync(join(source, "env", "star*name"), "skip");
  symlinkSync(join(root, "outside"), join(source, "env", "link"));
  expect(await copyPreparationFiles(source, destination, ["env/*"])).toEqual({ copied: 1, skipped: 4 });
  writeFileSync(join(source, "a"), "skip"); writeFileSync(join(source, "aa"), "copy");
  expect(await copyPreparationFiles(source, destination, ["a*a"])).toEqual({ copied: 1, skipped: 0 });
  expect(existsSync(join(destination, "a"))).toBe(false);
});
it("skips symbolic links below an intermediate wildcard and at matched destination files", async () => {
  for (const name of ["env-good", "env-source-link", "env-destination-link"]) {
    mkdirSync(join(source, name)); writeFileSync(join(source, name, "local.env"), "copy");
  }
  rmSync(join(source, "env-source-link", "local.env")); symlinkSync(join(root, "outside"), join(source, "env-source-link", "local.env"));
  mkdirSync(join(destination, "env-destination-link")); symlinkSync(join(root, "outside"), join(destination, "env-destination-link", "local.env"));
  expect(await copyPreparationFiles(source, destination, ["env-*/local.env"])).toEqual({ copied: 1, skipped: 2 });
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
  const spy = vi.spyOn(Processes, "gitProcess").mockImplementation((cwd, args, ...rest) => real(cwd, args.includes("submodule") ? [...args.slice(0, args.indexOf("submodule")), "-c", "protocol.file.allow=always", ...args.slice(args.indexOf("submodule"))] : args, ...rest));
  const detail = await Effect.runPromise(prepareWorktree(destination, worktree, { submodules: true }, "main"));
  expect(detail).toContain("Submodules initialized."); expect(readFileSync(join(worktree, "tf", "initial.txt"), "utf8")).toBe("initial\n");
  const args = spy.mock.calls.find(call => call[1].includes("submodule"))![1];
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
  vi.spyOn(Processes, "gitProcess").mockImplementation((cwd, args, ...rest) => real(cwd, args.includes("submodule") ? [...args.slice(0, args.indexOf("submodule")), "-c", "protocol.file.allow=always", ...args.slice(args.indexOf("submodule"))] : args, ...rest));
  expect(await Effect.runPromise(prepareWorktree(destination, worktree, { submodules: true }, "main"))).toContain("Submodules initialized.");
  expect(existsSync(marker)).toBe(false);
  vi.mocked(Processes.gitProcess).mockRejectedValue(new Error("private timeout diagnostic"));
  const detail = await Effect.runPromise(prepareWorktree(destination, worktree, { submodules: true }, "main"));
  expect(detail).toContain("Submodule initialization failed"); expect(detail).not.toContain("private timeout");
});
for (const change of ["url", "path", "added", "duplicate"]) it(`skips unapproved ${change} entries committed by a worker`, async () => {
  const { first } = localSubmodule();
  const project = join(root, "untrusted-project"), worker = join(root, "untrusted-worker");
  git(destination, "worktree", "add", "-b", "project", project);
  if (change === "url") git(project, "config", "--file", ".gitmodules", "submodule.tf.url", "ssh://invalid.example/agent-chosen");
  if (change === "path") git(project, "config", "--file", ".gitmodules", "submodule.tf.path", "elsewhere");
  if (change === "added" || change === "duplicate") {
    git(project, "config", "--file", ".gitmodules", "submodule.extra.path", change === "added" ? "extra" : "tf");
    git(project, "config", "--file", ".gitmodules", "submodule.extra.url", change === "added" ? source : "ssh://invalid.example/duplicate");
    if (change === "added") git(project, "update-index", "--add", "--cacheinfo", `160000,${first},extra`);
  }
  git(project, "add", ".gitmodules"); git(project, "commit", "-m", "Agent changes modules");
  git(project, "worktree", "add", "-b", "worker", worker);
  // A dirty main-checkout file is not authority for the agent's new URL.
  if (change === "url") writeFileSync(join(destination, ".gitmodules"), readFileSync(join(project, ".gitmodules")));
  const real = Processes.gitProcess;
  const spy = vi.spyOn(Processes, "gitProcess").mockImplementation((cwd, args, ...rest) => args.includes("submodule")
    ? Promise.resolve({ code: 0, stdout: "", stderr: "", truncated: false }) : real(cwd, args, ...rest));
  const detail = await Effect.runPromise(prepareWorktree(destination, worker, { submodules: true }, "main"));
  expect(detail).toContain("Unapproved submodules were skipped.");
  const updates = spy.mock.calls.filter(call => call[1].includes("submodule"));
  if (change === "added") expect(updates[0]![1].slice(-2)).toEqual(["--", ":(top,literal)tf"]);
  else expect(updates).toHaveLength(0);
});
it("never clones an agent's nested gitlink through an approved directory prefix with production options", async () => {
  const first = initialize(source); initialize(destination);
  const evil = join(root, "evil"); mkdirSync(evil); const evilHead = initialize(evil);
  const home = join(root, "transport-home"), shim = join(root, "ssh-fixture"), contacts = join(root, "contacts");
  mkdirSync(home); vi.stubEnv("HOME", home);
  // Git's actual SSH transport runs a fixture upload-pack, with no network or
  // change to production options. Refuse every path outside these fixtures.
  writeFileSync(shim, `#!${process.execPath}\nconst fs = require('node:fs');\nconst { spawnSync } = require('node:child_process');\nconst match = /^git-upload-pack '([^']+)'$/.exec(process.argv.at(-1));\nif (!match || !${JSON.stringify([source, evil])}.includes(match[1])) process.exit(97);\nfs.appendFileSync(${JSON.stringify(contacts)}, match[1] + '\\n');\nconst result = spawnSync('git-upload-pack', [match[1]], { stdio: 'inherit' });\nprocess.exit(result.status ?? 98);\n`, { mode: 0o700 });
  writeFileSync(join(home, ".gitconfig"), `[core]\n\tsshCommand = ${shim}\n[ssh]\n\tvariant = simple\n`);
  git(destination, "config", "--file", ".gitmodules", "submodule.A.path", "libs");
  git(destination, "config", "--file", ".gitmodules", "submodule.A.url", `ssh://fixture.invalid${source}`);
  git(destination, "add", ".gitmodules"); git(destination, "update-index", "--add", "--cacheinfo", `160000,${first},libs`);
  git(destination, "commit", "-m", "Trusted module"); const base = git(destination, "rev-parse", "HEAD");
  const safe = join(root, "safe"), project = join(root, "project"), worker = join(root, "worker");
  git(destination, "worktree", "add", "-b", "safe", safe);
  expect(await Effect.runPromise(prepareWorktree(destination, safe, { submodules: true }, "main", base))).not.toContain("failed");
  expect(existsSync(join(safe, "libs", ".git"))).toBe(true);
  expect(readFileSync(contacts, "utf8")).toContain(source);
  git(destination, "worktree", "add", "-b", "project", project);
  git(project, "config", "--file", ".gitmodules", "submodule.B.path", "libs/evil");
  git(project, "config", "--file", ".gitmodules", "submodule.B.url", `ssh://fixture.invalid${evil}`);
  git(project, "add", ".gitmodules"); git(project, "commit", "-m", "Overlapping module configuration");
  const overlapping = join(root, "overlapping"); git(project, "worktree", "add", "-b", "overlapping", overlapping);
  expect(await Effect.runPromise(prepareWorktree(destination, overlapping, { submodules: true }, "main", base))).toContain("Unapproved submodules were skipped.");
  expect(existsSync(join(overlapping, "libs", ".git"))).toBe(false);
  git(project, "rm", "--cached", "libs"); mkdirSync(join(project, "libs"), { recursive: true });
  writeFileSync(join(project, "libs", "ordinary.txt"), "plain directory\n");
  git(project, "add", ".gitmodules", "libs/ordinary.txt");
  git(project, "update-index", "--add", "--cacheinfo", `160000,${evilHead},libs/evil`); git(project, "commit", "-m", "Nested attacker module");
  git(project, "worktree", "add", "-b", "worker", worker);
  const detail = await Effect.runPromise(prepareWorktree(destination, worker, { submodules: true }, "main", base));
  expect(detail).toContain("Unapproved submodules were skipped.");
  expect(existsSync(join(worker, "libs", "evil", ".git"))).toBe(false);
  expect(readFileSync(contacts, "utf8")).not.toContain(evil);
});
it("pins submodule trust to the recorded commit even when an agent moves the shared base ref", async () => {
  localSubmodule(); const recorded = git(destination, "rev-parse", "HEAD");
  const project = join(root, "project"), worker = join(root, "worker");
  git(destination, "worktree", "add", "-b", "project", project);
  git(project, "config", "--file", ".gitmodules", "submodule.tf.url", "ssh://fixture.invalid/agent-chosen");
  git(project, "add", ".gitmodules"); git(project, "commit", "-m", "Agent module URL");
  git(project, "update-ref", "refs/heads/main", git(project, "rev-parse", "HEAD"));
  git(project, "worktree", "add", "-b", "worker", worker);
  const real = Processes.gitProcess;
  const spy = vi.spyOn(Processes, "gitProcess").mockImplementation((cwd, args, ...rest) => args.includes("submodule")
    ? Promise.resolve({ code: 0, stdout: "", stderr: "", truncated: false }) : real(cwd, args, ...rest));
  expect(await Effect.runPromise(prepareWorktree(destination, worker, { submodules: true }, "main", recorded))).toContain("Unapproved submodules were skipped.");
  expect(spy.mock.calls.filter(call => call[1].includes("submodule"))).toHaveLength(0);
  // Historical projects without a recorded commit explicitly disclose fallback.
  expect(await Effect.runPromise(prepareWorktree(destination, worker, { submodules: true }, "main"))).toContain("Submodule trust used the base branch because no starting commit was recorded.");
  expect(spy.mock.calls.filter(call => call[1].includes("submodule"))).toHaveLength(1);
});
for (const name of ["renamed", "tf=wrong", "tf\nwrong"]) it(`refuses changed or unsafe submodule name ${JSON.stringify(name)}`, async () => {
  localSubmodule();
  // Unsafe names are present even in the recorded commit, so equality alone
  // cannot hide a missing validation check.
  if (name !== "renamed") {
    git(destination, "config", "--file", ".gitmodules", "--rename-section", "submodule.tf", `submodule.${name}`);
    git(destination, "add", ".gitmodules"); git(destination, "commit", "-m", "Unsafe module name");
  }
  const base = git(destination, "rev-parse", "HEAD"), worker = join(root, "worker");
  git(destination, "worktree", "add", "-b", "worker", worker);
  if (name === "renamed") git(worker, "config", "--file", ".gitmodules", "--rename-section", "submodule.tf", "submodule.renamed");
  const spy = vi.spyOn(Processes, "gitProcess");
  const detail = await Effect.runPromise(prepareWorktree(destination, worker, { submodules: true }, "main", base));
  expect(detail).toMatch(/Unapproved submodules were skipped|Submodule initialization failed/);
  expect(spy.mock.calls.filter(call => call[1].includes("submodule"))).toHaveLength(0);
});
it("skips a relative submodule URL even when the recorded commit contains it", async () => {
  localSubmodule(); git(destination, "config", "--file", ".gitmodules", "submodule.tf.url", "../source");
  git(destination, "add", ".gitmodules"); git(destination, "commit", "-m", "Relative module URL");
  const base = git(destination, "rev-parse", "HEAD"), worker = join(root, "worker");
  git(destination, "worktree", "add", "-b", "worker", worker); const spy = vi.spyOn(Processes, "gitProcess");
  expect(await Effect.runPromise(prepareWorktree(destination, worker, { submodules: true }, "main", base))).toContain("Unapproved submodules were skipped.");
  expect(spy.mock.calls.filter(call => call[1].includes("submodule"))).toHaveLength(0);
});
it("refuses a file URL with the unchanged production transport options", async () => {
  localSubmodule(); git(destination, "config", "protocol.file.allow", "always");
  git(destination, "config", "--file", ".gitmodules", "submodule.tf.url", `file://${source}`);
  git(destination, "add", ".gitmodules"); git(destination, "commit", "-m", "Local URL");
  const worktree = join(root, "file-url-worktree"); git(destination, "worktree", "add", "-b", "file-url", worktree);
  const spy = vi.spyOn(Processes, "gitProcess");
  const detail = await Effect.runPromise(prepareWorktree(destination, worktree, { submodules: true }, "main"));
  expect(detail).toContain("Submodule initialization failed; creation continued.");
  expect(existsSync(join(worktree, "tf", ".git"))).toBe(false);
  const options = spy.mock.calls.find(call => call[1].includes("submodule"))![1];
  expect(options).toContain("protocol.allow=never"); expect(options).toContain("protocol.file.allow=never"); expect(options).not.toContain("protocol.file.allow=always");
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
it("handles an embedded .git directory without hiding other changes and safely discards its pointer", async () => {
  initialize(destination);
  const embedded = join(destination, "embedded"); mkdirSync(embedded); const first = initialize(embedded);
  git(destination, "add", "embedded"); git(destination, "commit", "-m", "Embedded gitlink");
  const second = workerCommit(embedded, "next.txt", "next\n"); writeFileSync(join(destination, "initial.txt"), "other change\n");
  const service = makeGitService({ workspaceFolder: root }), run = Effect.runPromise;
  const changes = (await run(service.changes(destination))).items;
  expect(changes.find(row => row.path === "embedded")).toMatchObject({ submodule: { oldHead: first, newHead: second } });
  expect(changes.some(row => row.path === "initial.txt")).toBe(true);
  await run(service.discard(destination, await run(service.planDiscard(destination, ["embedded"])), true));
  expect(git(embedded, "rev-parse", "HEAD")).toBe(first);
  expect((await run(service.diff(destination, { path: "initial.txt" }))).patch).toContain("other change");
});
it("keeps ordinary status and file reads available when a gitlink has malformed metadata", async () => {
  localSubmodule(); writeFileSync(join(destination, "tf", ".git"), "not a gitdir marker\n");
  writeFileSync(join(destination, "initial.txt"), "ordinary change\n");
  const service = makeGitService({ workspaceFolder: root }), run = Effect.runPromise;
  expect((await run(service.changes(destination))).items.some(row => row.path === "initial.txt")).toBe(true);
  expect(await run(service.readFile(destination, "initial.txt"))).toMatchObject({ text: "ordinary change\n" });
  expect((await run(service.listFolder(destination))).items.some(row => row.path === "initial.txt")).toBe(true);
  expect((await run(service.diff(destination, { path: "initial.txt" }))).patch).toContain("ordinary change");
  await expect(run(service.planDiscard(destination, ["tf"]))).rejects.toMatchObject({ reason: "dirty_source" });
});
it("reports an uninitialized changed pointer and a missing submodule directory without reading either as a file", async () => {
  const { first, second } = localSubmodule();
  git(join(destination, "tf"), "checkout", second); git(destination, "add", "tf");
  git(destination, "submodule", "deinit", "-f", "--", "tf");
  const service = makeGitService({ workspaceFolder: root }), run = Effect.runPromise;
  expect((await run(service.changes(destination))).items).toMatchObject([{ path: "tf", submodule: { oldHead: first, indexHead: second, newHead: second } }]);
  expect((await run(service.diff(destination))).patch).toContain(second);
  expect((await run(service.planDiscard(destination, ["tf"]))).paths).toEqual([]);
  rmSync(join(destination, "tf"), { recursive: true });
  expect((await run(service.changes(destination))).items).toMatchObject([{ path: "tf", submodule: { newHead: null } }]);
});
it("batches gitlink metadata for ordinary deleted rows and preserves renamed gitlinks in review", async () => {
  const { first } = localSubmodule();
  for (let i = 0; i < 12; i++) workerCommit(destination, `delete-${i}`, "delete\n");
  for (let i = 0; i < 12; i++) rmSync(join(destination, `delete-${i}`));
  const spy = vi.spyOn(Processes, "gitProcess"), service = makeGitService({ workspaceFolder: root });
  expect((await Effect.runPromise(service.changes(destination))).items).toHaveLength(12);
  expect(spy.mock.calls.filter(call => call[1][0] === "ls-files")).toHaveLength(1);
  expect(spy.mock.calls.filter(call => call[1][0] === "ls-tree")).toHaveLength(1);
  git(destination, "restore", "."); git(destination, "checkout", "-b", "renamed");
  git(destination, "mv", "tf", "renamed-tf"); git(destination, "commit", "-m", "Rename module");
  const review = await Effect.runPromise(service.review(destination, "main", true));
  expect(review.files.items.find(row => row.path === "renamed-tf")).toMatchObject({ originalPath: "tf", submodule: { oldHead: first, newHead: first } });
});

const integrated = (test: (rig: Rig, runner: FakeRunner) => Effect.Effect<void, unknown, any>) => Effect.gen(function* () {
  const rig = makeRig(); vi.stubEnv("HOME", rig.home); vi.stubEnv("LGTM_HOME", rig.stateFolder);
  const runner = yield* makeFakeRunner();
  try { yield* provideRig(rig, { runner, secrets: makeFakeSecrets() }, Effect.gen(function* () { yield* registerRebuildCheck(); yield* test(rig, runner); })); }
  finally { removeRig(rig); }
}).pipe(Effect.scoped, Effect.orDie);
effectIt.live("owner preparation reaches project and worker worktrees and their started prompts; submodule failure is nonfatal", () => integrated((rig, runner) => Effect.gen(function* () {
  const p = yield* workerProject(rig);
  initialize(source);
  git(rig.work, "-c", "protocol.file.allow=always", "submodule", "add", "--", source, "tf");
  git(rig.work, "add", ".gitmodules", "tf"); git(rig.work, "commit", "-m", "Base submodule");
  git(p.path, "merge", "--ff-only", "main");
  mkdirSync(join(rig.work, "env")); writeFileSync(join(rig.work, "env", "local.env"), "synthetic-env");
  const preparation = { copyPaths: ["env/*.env"], submodules: true, instructions: "Run pnpm install before building." };
  expect(failureOf(yield* call(p.leadCredential, "project.repository.preparation.set", { repoId: p.repoId, preparation }))).toBeDefined();
  success(yield* call(ownerCredential, "project.repository.preparation.set", { repoId: p.repoId, preparation }));
  expect(success(yield* call(ownerCredential, "project.repositories", {})).items).toMatchObject([{ preparation }]);
  expect(snapshotFor({ _tag: "agent", agentId: p.lead.agentId }, yield* (yield* Views).snapshot)?.repositories).toMatchObject([{ preparation: { copyPaths: ["env/*.env"] } }]);
  expect(readFileSync(join(rig.work, ".git", "info", "exclude"), "utf8")).toContain("/env/*.env\n");
  expect(git(rig.work, "status", "--porcelain")).toBe("");
  const real = Processes.gitProcess;
  const prepared = vi.spyOn(Preparation, "prepareWorktree");
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
  for (const start of runner.callsOf("start")) {
    expect(start.input.instructions).toContain("--- BEGIN OWNER REPOSITORY SETTINGS ---\nOwner's repository settings\nRun pnpm install before building.\n--- END OWNER REPOSITORY SETTINGS ---");
  }
  const log = yield* readLog;
  expect(log.filter(row => row.event._tag === "project.CreationFinished").at(-1)?.event).toMatchObject({ detail: expect.stringContaining("Submodule initialization failed") });
  expect(JSON.stringify(log)).not.toContain("private diagnostic"); expect(JSON.stringify(log)).not.toContain("synthetic-env");
  const projectPath = view.workspaces[project.worktreeId]!.path;
  expect(project.startingHead).toBe(git(projectPath, "rev-parse", "HEAD"));
  expect(prepared.mock.calls.find(args => args[1] === projectPath)?.[4]).toBe(project.startingHead);
  // Advance the project branch before allocating a worker. Its trust anchor
  // must remain the project's recorded start, not the branch's current head.
  const advanced = workerCommit(projectPath, "advanced.txt", "later project commit\n");
  expect(advanced).not.toBe(project.startingHead);
  const pinnedWorker = success(yield* call(ownerCredential, "start", workerPayload(projectPath, { name: "Pinned worker" })));
  const pinnedState = yield* (yield* Views).snapshot;
  expect(prepared.mock.calls.find(args => args[1] === pinnedState.agents[pinnedWorker.agentId as string]!.folder)?.[4]).toBe(project.startingHead);
})));
effectIt.live("copy patterns preserve owner file access and tracked context for every actor", () => integrated((rig) => Effect.gen(function* () {
  const p = yield* workerProject(rig);
  workerCommit(p.path, "private-tracked", "tracked configuration\n");
  workerCommit(p.path, ".gitignore", "!/private-untracked\n");
  success(yield* call(ownerCredential, "project.repository.preparation.set", { repoId: p.repoId, preparation: { copyPaths: ["private*"] } }));
  writeFileSync(join(p.path, "private-untracked"), "untracked configuration\n");
  const selection = { projectId: p.projectId, worktreeId: p.worktreeId };
  for (const path of ["private-tracked", "private-untracked"]) {
    expect(success(yield* call(ownerCredential, "project.files.read", { ...selection, path })).result).toMatchObject({ kind: "text", text: expect.stringContaining("configuration") });
  }
  const owner = success(yield* call(ownerCredential, "project.context.search", { ...selection, scope: "files", query: "private" }));
  expect(owner.result.items.map((row: any) => row.reference.path).sort()).toEqual(["private-tracked", "private-untracked"]);
  const agent = success(yield* call(p.leadCredential, "project.context.search", { ...selection, scope: "files", query: "private" }));
  expect(agent.data).toContain("private-tracked");
  expect(agent.data).not.toContain("private-untracked");
})));
effectIt.live("refuses excessive composed repository instructions when the owner sets them", () => integrated((rig) => Effect.gen(function* () {
  const p = yield* workerProject(rig);
  const overrides = join(rig.home, ".lgtm", "roles"); mkdirSync(overrides, { recursive: true });
  writeFileSync(join(overrides, "lead-base.md"), "a".repeat(24_500));
  writeFileSync(join(overrides, "project-lead.md"), "b".repeat(24_500));
  const response = yield* call(ownerCredential, "project.repository.preparation.set", { repoId: p.repoId, preparation: { instructions: "x".repeat(1024) } });
  expect(failureOf(response)).toMatchObject({ reason: "Role instructions exceed 48 KiB." });
  expect(projectView(yield* (yield* Views).snapshot).repositories[p.repoId as string]!.preparation).toBeUndefined();
})));
effectIt.live("surfaces the offending copy pattern through project creation and worker start", () => integrated((rig, runner) => Effect.gen(function* () {
  const p = yield* workerProject(rig);
  writeFileSync(join(rig.work, "private-config"), ""); truncateSync(join(rig.work, "private-config"), 1024 * 1024 + 1);
  success(yield* call(ownerCredential, "project.repository.preparation.set", { repoId: p.repoId, preparation: { copyPaths: ["private-config"] } }));
  const worker = yield* call(p.leadCredential, "start", workerPayload(p.path));
  expect(failureOf(worker)).toMatchObject({ reason: 'Worktree preparation refused pattern "private-config": unsafe path or copy limit.' });
  yield* seedAccount("lead-account", "cursor"); yield* seedHarness("cursor");
  success(yield* call(ownerCredential, "harnesses", { operation: "set", harness: harnessSettings("cursor", { adapterPath: process.execPath, reportedModes: [{ id: "default", level: "ask" }, { id: "acceptEdits", level: "edit" }] }) }));
  success(yield* call(ownerCredential, "accounts set-secret", { accountId: "lead-account", name: "CURSOR_API_KEY", nonce: "refusal-key", value: Redacted.make("synthetic-preparation-key-012345") }));
  const created = yield* call(ownerCredential, "project.create", { requestId: freshRequest(), name: "Refused copy", goal: "Build", machineId: "local", repository: { _tag: "existing", repoId: p.repoId }, lead: { harness: "cursor", account: { _tag: "any" }, permission: "ask" } });
  expect(failureOf(created)).toMatchObject({ reason: 'Worktree preparation refused pattern "private-config": unsafe path or copy limit.' });
  expect(runner.callsOf("start")).toHaveLength(0);
})));
it("excludes copy patterns in every checkout while keeping tracked matches visible to owners and agents", async () => {
  initialize(destination); mkdirSync(join(destination, "config"));
  workerCommit(destination, "config/private token", "synthetic secret\n");
  git(destination, "checkout", "-b", "project"); workerCommit(destination, "config/private token", "changed secret\n");
  const patterns = ["config/private*", "other private"];
  const service = makeGitService({ workspaceFolder: root, copyPaths: () => ({ [destination]: patterns }) }), run = Effect.runPromise;
  await run(service.excludePreparationFiles(destination, patterns)); await run(service.excludePreparationFiles(destination, patterns));
  const exclude = readFileSync(join(destination, ".git", "info", "exclude"), "utf8");
  expect(exclude.match(/\/config\/private\*/g)).toHaveLength(1); expect(exclude).toContain("/other\\ private\n");
  writeFileSync(join(destination, "other private"), "untracked secret");
  const tree = await run(service.createWorktree(destination, "worker"));
  writeFileSync(join(tree.path, "other private"), "worker secret");
  expect(git(destination, "status", "--porcelain")).toBe(""); expect(git(tree.path, "status", "--porcelain")).toBe("");
  git(destination, "add", "--all"); expect(git(destination, "diff", "--cached", "--name-only")).toBe("");
  for (const agentRead of [false, true]) {
    expect((await run(service.listFolder(destination, "config", agentRead))).items).toMatchObject([{ path: "config/private token" }]);
    expect(await run(service.readFile(destination, "config/private token", agentRead))).toMatchObject({ text: "changed secret\n" });
    await run(service.contextFile(destination, "config/private token", agentRead));
    expect((await run(service.contextFiles(destination, "private", agentRead))).items).toEqual(["config/private token"]);
    expect((await run(service.commitFiles(destination, "HEAD", agentRead))).items).toMatchObject([{ path: "config/private token" }]);
    expect((await run(service.review(destination, "main", agentRead))).files.items).toMatchObject([{ path: "config/private token" }]);
    expect((await run(service.diff(destination, { commit: "HEAD", agentRead }))).patch).toContain("changed secret");
    expect((await run(service.diff(destination, { path: "config/private token", commit: "HEAD", agentRead }))).patch).toContain("changed secret");
  }
  writeFileSync(join(destination, "config/private token"), "unstaged secret\n");
  expect((await run(service.changes(destination, true))).items).toMatchObject([{ path: "config/private token" }]);
  expect((await run(service.diff(destination, { agentRead: true }))).patch).toContain("unstaged secret");
  await run(service.contextFile(destination, "other private"));
  await expect(run(service.contextFile(destination, "other private", true))).rejects.toMatchObject({ reason: "invalid_path" });
  // A repository ignore override exposes the untracked copy to owner reads.
  // Copy-pattern protection still applies to agent reads only.
  workerCommit(destination, ".gitignore", "!/other private\n");
  expect((await run(service.listFolder(destination))).items.map(row => row.path)).toContain("other private");
  expect(await run(service.readFile(destination, "other private"))).toMatchObject({ text: "untracked secret" });
  expect((await run(service.contextFiles(destination, "private"))).items).toContain("other private");
  expect((await run(service.changes(destination))).items.map(row => row.path)).toContain("other private");
  expect((await run(service.diff(destination))).patch).toContain("untracked secret");
  expect((await run(service.listFolder(destination, undefined, true))).items.map(row => row.path)).not.toContain("other private");
  expect((await run(service.contextFiles(destination, "private", true))).items).not.toContain("other private");
  await expect(run(service.readFile(destination, "other private", true))).rejects.toMatchObject({ reason: "invalid_path" });
  await expect(run(service.diff(destination, { path: "other private", agentRead: true }))).rejects.toMatchObject({ reason: "invalid_path" });
  expect((await run(service.changes(destination, true))).items.map(row => row.path)).not.toContain("other private");
  expect((await run(service.diff(destination, { agentRead: true }))).patch).not.toContain("untracked secret");
});
