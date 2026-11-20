import { checkedPullRequest, deliveryMode, forbiddenRemote, githubCommands, type GithubDelivery } from "./github.ts";
import { createHash, randomUUID } from "node:crypto";
import { constants } from "node:fs";
import { lstat, mkdir, mkdtemp, open, opendir, realpath, rename, rm, rmdir, stat, writeFile } from "node:fs/promises";
import { basename, dirname, join, resolve } from "node:path";
import { tmpdir } from "node:os";
import * as Effect from "effect/Effect";
import { fileTextLimit, isRepositoryCredentialPath, type Caller } from "@lgtm/contracts";
import { checkedMetadata, cleanText, directory, filePath, inside, slug, smallFile } from "./paths.ts";
import { parseChanges, parseConflict, parseFiles, parseHistory, parseNumstat } from "./parse.ts";
import { replaceFile, ReplaceFileError, sha256 } from "./replace.ts";
import { boundedUtf8, gitProcess, processFailure, type ProcessReply } from "./process.ts";
import { GitFailure, type Bounded, type Change, type FileChange, type FileEntry, type FileImageType, type Conflict, type ConflictChoice, type Diff, type DiscardPlan,
  type GitFailureReason, type ResolvedConflict, type GitService, type Repository, type Review, type Worktree } from "./types.ts";
export * from "./types.ts";

export interface GitConfiguration {
  /** Trusted engine setting, never an agent-supplied root. */
  readonly workspaceFolder: string;
  readonly baseBranches?: () => Readonly<Record<string, string>>;
  readonly startingHeads?: () => Readonly<Record<string, string>>;
  readonly maxOutputBytes?: number;
  readonly maxDiffBytes?: number;
  readonly maxEntries?: number;
  readonly timeoutMs?: number;
  readonly operationTimeoutMs?: number;
  readonly networkTimeoutMs?: number;
}
const deliveryOptions = ["-c", "protocol.allow=never", "-c", "protocol.https.allow=always", "-c", "protocol.ssh.allow=always",
  "-c", "protocol.ext.allow=never", "-c", "push.gpgSign=false", "-c", "core.askPass=", "-c", "http.followRedirects=false"];
const fail = (reason: GitFailureReason): never => { throw new GitFailure({ reason }); };
const determinate = (reason: GitFailureReason): never => { throw new GitFailure({ reason, determinate: true }); };
const hash = (...parts: ReadonlyArray<string | Buffer>) => {
  const value = createHash("sha256");
  for (const part of parts) { value.update(String(Buffer.byteLength(part))); value.update(":"); value.update(part); }
  return value.digest("hex");
};
const exists = async (path: string) => {
  try { await lstat(path); return true; }
  catch (error) { if ((error as NodeJS.ErrnoException).code === "ENOENT") return false; throw error; }
};
const bound = (requested: number | undefined, fallback: number, maximum: number) =>
  requested === undefined ? fallback : Number.isSafeInteger(requested) && requested > 0 && requested <= maximum ? requested : fail("output_limit");

/** Raw, bounded local data. W04 must authorize the caller/project and redact
 * every returned string through the production redactor before API delivery.
 * This service neither registers commands nor writes project events. */
export const makeGitService = (configuration: GitConfiguration): GitService => {
  const outputBytes = bound(configuration.maxOutputBytes, 1024 * 1024, 2 * 1024 * 1024);
  const diffBytes = bound(configuration.maxDiffBytes, 128 * 1024, 256 * 1024);
  const entries = bound(configuration.maxEntries, 200, 500);
  const timeoutMs = bound(configuration.timeoutMs, 10_000, 60_000);
  const operationMs = bound(configuration.operationTimeoutMs, 60_000, 120_000);
  const networkMs = bound(configuration.networkTimeoutMs, 90_000, 120_000);
  let rootIdentity: { path: string; dev: number; ino: number } | undefined;
  const root = async () => {
    const path = await directory(resolve(configuration.workspaceFolder), configuration.workspaceFolder);
    const info = await stat(path);
    if (rootIdentity === undefined) rootIdentity = { path, dev: info.dev, ino: info.ino };
    else if (rootIdentity.path !== path || rootIdentity.dev !== info.dev || rootIdentity.ino !== info.ino) fail("invalid_path");
    return path;
  };
  // The runtime owns the single bounded queue. A service operation owns only
  // its cancellation budget and whether an external mutation may have started.
  const mutations = new WeakSet<AbortSignal>();
  // Authentication failures belong to GitHub. A proven local fallback clears
  // this context before the comparison, without changing stored event shapes.
  const githubOperations = new WeakSet<AbortSignal>();
  const operation = <A>(body: (signal: AbortSignal) => Promise<A>, network = false) => Effect.tryPromise({
    try: async (interrupted) => {
      const signal = AbortSignal.any([interrupted, AbortSignal.timeout(network ? 120_000 : operationMs)]);
      if (signal.aborted) return fail("cancelled");
      try { return await body(signal); }
      catch (error) {
        const failure = error instanceof GitFailure ? error : new GitFailure({ reason: "invalid_path" });
        throw new GitFailure({ reason: failure.reason, ...(githubOperations.has(signal) ? { githubDelivery: true } : {}), ...(failure.pullRequestState === undefined ? {} : { pullRequestState: failure.pullRequestState }), ...(failure.uncertain || mutations.has(signal) && !failure.determinate ? { uncertain: true } : {}) });
      }
    },
    catch: (error) => error instanceof GitFailure ? error : new GitFailure({ reason: "invalid_path" }),
  });
  const run = async (folder: string, args: ReadonlyArray<string>, signal: AbortSignal, truncate = false, bytes = outputBytes): Promise<ProcessReply> => {
    await directory(await root(), folder);
    return gitProcess(folder, args, { bytes, timeoutMs: ["fetch", "pull", "push"].some((command) => args.includes(command)) ? networkMs : timeoutMs }, signal, truncate);
  };
  const mutateRun = async (folder: string, args: ReadonlyArray<string>, signal: AbortSignal) => {
    mutations.add(signal);
    return run(folder, args, signal);
  };
  const mutateGit = async (folder: string, args: ReadonlyArray<string>, signal: AbortSignal, network?: "fetch" | "push") => {
    const reply = await mutateRun(folder, args, signal);
    if (reply.code !== 0) throw processFailure(reply, network);
    return reply.stdout;
  };
  const git = async (folder: string, args: ReadonlyArray<string>, signal: AbortSignal) => {
    const reply = await run(folder, args, signal);
    if (reply.code !== 0) throw processFailure(reply);
    return reply.stdout;
  };
  const repository = async (folder: string, signal: AbortSignal): Promise<Repository> => {
    const base = await root(), path = await directory(base, folder);
    if (!await exists(join(path, ".git"))) return fail("not_repository");
    const commonDirectory = await checkedMetadata(base, path);
    const identity = (await git(path, ["rev-parse", "--path-format=absolute", "--git-common-dir", "--show-toplevel"], signal)).trimEnd().split("\n");
    if (identity.length !== 2) return fail("invalid_path");
    if (await directory(base, identity[0]!) !== commonDirectory) return fail("invalid_path");
    if (await realpath(identity[1]!) !== path) return fail("not_repository");
    return { path, name: basename(path), commonDirectory };
  };
  const list = <A>(items: ReadonlyArray<A>, maximum = entries): Bounded<A> => ({ items: items.slice(0, maximum), truncated: items.length > maximum });
  const completeChanges = async (folder: string, signal: AbortSignal): Promise<Change[]> => {
    const changes = parseChanges(await git(folder, ["status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignore-submodules=none"], signal));
    if (changes.length > entries) return fail("output_limit");
    return changes;
  };
  const head = async (folder: string, signal: AbortSignal): Promise<string | null> => {
    const reply = await run(folder, ["rev-parse", "--verify", "--quiet", "HEAD"], signal);
    if (reply.code === 1) return null;
    if (reply.code !== 0) throw processFailure(reply);
    return reply.stdout.trim();
  };
  const branch = async (folder: string, signal: AbortSignal): Promise<string | null> => {
    const reply = await run(folder, ["symbolic-ref", "--quiet", "--short", "HEAD"], signal);
    if (reply.code === 1) return null;
    if (reply.code !== 0) throw processFailure(reply);
    return reply.stdout.trim();
  };
  const ref = async (folder: string, value: string, signal: AbortSignal): Promise<string> => {
    // Deliberately accepts branch names and object IDs, not revision syntax,
    // reflog selectors, option prefixes or paths.
    if (!cleanText(value, 1024) || value === "@" || value.startsWith("-") || /[~^:{}\s\\]/.test(value)) return fail("invalid_ref");
    const reply = await run(folder, ["rev-parse", "--verify", "--end-of-options", `${value}^{commit}`], signal);
    if (reply.code !== 0) return fail("invalid_ref");
    return reply.stdout.trim();
  };
  const localBranch = async (folder: string, name: string, signal: AbortSignal) => {
    if (!cleanText(name, 1024) || name.startsWith("-")) return fail("invalid_ref");
    if ((await run(folder, ["check-ref-format", "--branch", name], signal)).code !== 0) return fail("invalid_ref");
    return ref(folder, `refs/heads/${name}`, signal);
  };
  const history = async (folder: string, revision: string, signal: AbortSignal, maximum = entries, exclusions: string[] = []) =>
    list(parseHistory(await git(folder, ["log", `--max-count=${maximum + 1}`, "-z", "--format=%H%x00%P%x00%an%x00%cI%x00%s", revision, ...exclusions, "--"], signal)), maximum);
  const startingHead = async (folder: string, signal: AbortSignal) => {
    const start = configuration.startingHeads?.()[folder];
    return start === undefined ? undefined : ref(folder, start, signal);
  };
  const comparisonBase = async (folder: string, current: string, target: string, start: string | undefined, signal: AbortSignal) => {
    const base = (await git(folder, ["merge-base", current, target], signal)).trim();
    if (start === undefined) return base;
    const initial = (await git(folder, ["merge-base", current, start], signal)).trim();
    const ancestor = await run(folder, ["merge-base", "--is-ancestor", base, initial], signal);
    if (ancestor.code !== 0 && ancestor.code !== 1) throw processFailure(ancestor);
    if (ancestor.code === 0) return initial;
    const reverse = await run(folder, ["merge-base", "--is-ancestor", initial, base], signal);
    if (reverse.code === 0) return base;
    if (reverse.code !== 1) throw processFailure(reverse);
    // A project may have merged a divergent local-base update after its
    // fetched start. Combine those two inherited trees without a checkout or
    // reference write, so neither side's upstream files become project work.
    const drivers = await run(folder, ["config", "--get-regexp", "^merge\\..*\\.driver$"], signal);
    if (drivers.code === 0) return fail("unsupported_review");
    if (drivers.code !== 1) throw processFailure(drivers);
    const inherited = await run(folder, ["merge-tree", "--write-tree", "--no-messages", base, initial], signal);
    if (inherited.code === 1) return fail("unsupported_review");
    if (inherited.code !== 0) throw processFailure(inherited);
    const tree = inherited.stdout.trim();
    if (!/^(?:[0-9a-f]{40}|[0-9a-f]{64})$/.test(tree)) return fail("invalid_ref");
    return tree;
  };
  const readable = (row: { path: string; originalPath: string | null }) => !isRepositoryCredentialPath(row.path)
    && (row.originalPath === null || !isRepositoryCredentialPath(row.originalPath));
  const filesBetween = async (folder: string, args: string[], signal: AbortSignal, agentRead = false) => {
    const files = parseFiles(await git(folder, [...args, "--no-ext-diff", "--no-textconv", "--name-status", "-z", "--find-renames", "--"], signal));
    return list(agentRead ? files.filter(readable) : files);
  };
  const patch = async (folder: string, args: string[], signal: AbortSignal, paths: string[] = []): Promise<Diff> => {
    const reply = await run(folder, [...args, "--no-ext-diff", "--no-textconv", "--no-color", "--patch", "--", ...paths], signal, true, diffBytes);
    if (reply.code !== 0 && !reply.truncated && !(args.includes("--no-index") && reply.code === 1)) throw processFailure(reply);
    const stats = await run(folder, [...args, "--no-ext-diff", "--no-textconv", "--numstat", "-z", "--", ...paths], signal);
    if (stats.code !== 0 && !(args.includes("--no-index") && stats.code === 1)) throw processFailure(stats);
    return { patch: reply.stdout, ...parseNumstat(stats.stdout), truncated: reply.truncated };
  };
  const diff = async (folder: string, options: { path?: string; staged?: boolean; commit?: string; agentRead?: boolean }, signal: AbortSignal): Promise<Diff> => {
    const path = options.path;
    // One complete status snapshot serves path admission and both aggregates.
    const workingChanges = options.commit === undefined ? await completeChanges(folder, signal) : undefined;
    let aggregatePaths: string[] | undefined;
    if (options.agentRead) {
      if (path !== undefined && isRepositoryCredentialPath(path)) return fail("invalid_path");
      const changes = workingChanges ?? parseFiles(await git(folder, ["diff-tree", "--root", "--no-commit-id", "-r", "-m", "--first-parent", await ref(folder, options.commit!, signal), "--name-status", "-z", "--find-renames", "--"], signal));
      if (path !== undefined && changes.some((row) => row.path === path && !readable(row))) return fail("invalid_path");
      if (path === undefined) {
        aggregatePaths = changes.filter(readable).flatMap((row) => row.originalPath === null ? [row.path] : [row.originalPath, row.path]);
        // Never pass an empty path list to Git: that would mean all files.
        if (aggregatePaths.length === 0) return { patch: "", additions: 0, deletions: 0, binary: false, truncated: false };
        for (const name of aggregatePaths) await filePath(folder, name);
      }
    }
    if (path !== undefined) await filePath(folder, path);
    else if (aggregatePaths !== undefined) { /* The selected paths were checked above. */ }
    else if (options.commit !== undefined) {
      const names = await git(folder, ["show", "--format=", "--first-parent", "--name-only", "-z", await ref(folder, options.commit, signal), "--"], signal);
      for (const name of names.split("\0").filter(Boolean)) await filePath(folder, name);
    } else {
      for (const change of workingChanges!) {
        await filePath(folder, change.path);
        if (change.originalPath !== null) await filePath(folder, change.originalPath);
      }
    }
    if (options.commit !== undefined) {
      const commit = await ref(folder, options.commit, signal);
      return patch(folder, ["show", "--format=", "--first-parent", commit], signal, path === undefined ? aggregatePaths ?? [] : [path]);
    }
    const currentHead = options.staged === undefined ? await head(folder, signal) : null;
    const diffArgs = ["diff", ...(options.staged === true ? ["--cached"] : currentHead === null ? [] : [currentHead])];
    if (path !== undefined) {
      const change = workingChanges!.find((item) => item.path === path);
      if ((change?.untracked || options.staged === undefined && currentHead === null && change?.index === "A") && options.staged !== true) return patch(folder, ["diff", "--no-index"], signal, ["/dev/null", await filePath(folder, path)]);
      if (change?.originalPath !== null && change?.originalPath !== undefined) await filePath(folder, change.originalPath);
      return patch(folder, diffArgs, signal, change?.originalPath ? [change.originalPath, path] : [path]);
    }
    const tracked = options.staged === undefined && currentHead === null
      ? { patch: "", additions: 0, deletions: 0, binary: false, truncated: false }
      : await patch(folder, diffArgs, signal, aggregatePaths ?? []);
    if (options.staged === true) return tracked;
    let result = tracked;
    for (const change of workingChanges!) {
      if (options.agentRead && !readable(change)) continue;
      if ((!change.untracked && !(options.staged === undefined && currentHead === null && change.index === "A")) || result.truncated) continue;
      // Issue 588: the aggregate already resolved HEAD and status. Use the
      // original Git author's bounded direct patch, after agent filtering.
      const next = await patch(folder, ["diff", "--no-index"], signal, ["/dev/null", await filePath(folder, change.path)]);
      const joined = Buffer.from(result.patch + next.patch);
      result = { patch: boundedUtf8(joined, diffBytes), additions: result.additions + next.additions,
        deletions: result.deletions + next.deletions, binary: result.binary || next.binary,
        truncated: next.truncated || joined.length > diffBytes };
    }
    return result;
  };
  const worktrees = async (folder: string, signal: AbortSignal): Promise<Bounded<Worktree>> => {
    const fields = (await git(folder, ["worktree", "list", "--porcelain", "-z"], signal)).split("\0"), items: Worktree[] = [];
    let current: { path: string; head: string | null; branch: string | null; locked: boolean; available: boolean } | undefined;
    for (const field of fields) {
      if (field.startsWith("worktree ")) {
        if (current !== undefined) items.push(current);
        const path = field.slice(9);
        let available = false;
        try { await repository(path, signal); available = true; } catch (error) { if (!(error instanceof GitFailure) && (error as NodeJS.ErrnoException).code !== "ENOENT") throw error; }
        // Do not disclose paths of linked worktrees outside the configured root.
        current = inside(await root(), path) ? { path, head: null, branch: null, locked: false, available } : undefined;
      } else if (current !== undefined) {
        if (field.startsWith("HEAD ")) current.head = /^0+$/.test(field.slice(5)) ? null : field.slice(5);
        if (field.startsWith("branch refs/heads/")) current.branch = field.slice(18);
        if (field.startsWith("locked")) current.locked = true;
      }
    }
    if (current !== undefined) items.push(current);
    return list(items);
  };
  const rollbackAllocation = async (path: string, allocation: Worktree, signal: AbortSignal) => {
    const rows = await worktrees(path, signal);
    if (rows.truncated) return fail("output_limit");
    const row = rows.items.find((row) => row.path === allocation.path);
    if (row) {
      if (row.branch !== allocation.branch || row.head !== allocation.head) return fail("stale_state");
      if ((await git(row.path, ["status", "--porcelain=v1", "--untracked-files=all"], signal)).trim()) return fail("dirty_source");
      await git(path, ["worktree", "remove", "--", row.path], signal);
    }
    // An unregistered failed checkout can still have created its branch.
    // The expected old value refuses deletion if somebody has committed on it.
    if (allocation.branch && allocation.head && !rows.items.some((other) => other.path !== allocation.path && other.branch === allocation.branch)) {
      const present = await run(path, ["show-ref", "--verify", "--quiet", `refs/heads/${allocation.branch}`], signal);
      if (present.code === 0) await git(path, ["update-ref", "-d", `refs/heads/${allocation.branch}`, allocation.head], signal);
      else if (present.code !== 1) throw processFailure(present);
    }
  };
  const resetPaths = async (folder: string, paths: string[], signal: AbortSignal) => {
    if (paths.length === 0) return;
    for (const path of paths) await filePath(folder, path);
    if (await head(folder, signal) === null) await mutateGit(folder, ["rm", "--cached", "-f", "--", ...paths], signal);
    else await mutateGit(folder, ["reset", "--quiet", "HEAD", "--", ...paths], signal);
  };
  const changePaths = (change: Change) => change.originalPath === null ? [change.path] : [change.originalPath, change.path];
  // A staged rename has already removed the original path from the index.
  // add must update its destination only; reset still needs both paths.
  const stagePaths = (change: Change) => /[RC]/.test(change.index) ? [change.path] : changePaths(change);
  const planDiscard = async (folder: string, requested: ReadonlyArray<string> | undefined, signal: AbortSignal): Promise<DiscardPlan> => {
    const changes = await completeChanges(folder, signal);
    if (requested !== undefined && (requested.length > entries || requested.some((path) => !changes.some((change) => change.path === path && !change.conflict)))) return fail("invalid_path");
    const paths = changes.filter((change) => !change.conflict && change.unstaged && (requested === undefined || requested.includes(change.path))).map((change) => change.path);
    const parts: (string | Buffer)[] = [folder, await head(folder, signal) ?? "", JSON.stringify(changes),
      await git(folder, ["diff", "--cached", "--no-ext-diff", "--no-textconv", "--raw", "--no-abbrev", "-z", "--"], signal)];
    for (const path of paths) {
      const file = await filePath(folder, path);
      parts.push(path, await exists(file) ? await smallFile(file, outputBytes) : "<deleted>");
    }
    return { paths, fingerprint: hash(...parts) };
  };
  const conflict = async (folder: string, path: string, signal: AbortSignal): Promise<Conflict> => {
    const file = await filePath(folder, path);
    if (!(await completeChanges(folder, signal)).some((change) => change.path === path && change.conflict)) return fail("conflicts");
    const index = await git(folder, ["ls-files", "--unmerged", "-z", "--", path], signal);
    const stages = new Map<string, string>();
    for (const row of index.split("\0").filter(Boolean)) {
      const match = /^(\d+) ([0-9a-f]+) ([123])\t/.exec(row);
      if (match === null || match[1] !== "100644" && match[1] !== "100755") return fail("unsupported_conflict");
      stages.set(match[3]!, match[2]!);
    }
    if (stages.size !== 3) return fail("unsupported_conflict");
    const blobs: string[] = [];
    for (const stage of ["2", "1", "3"]) {
      const blob = await git(folder, ["cat-file", "blob", stages.get(stage)!], signal);
      if (blob.includes("\0") || blob.includes("\ufffd")) return fail("unsupported_conflict");
      blobs.push(blob);
    }
    const current = await smallFile(file, outputBytes), currentHead = await head(folder, signal);
    if (currentHead === null) return fail("unsupported_conflict");
    const temporary = await mkdtemp(join(tmpdir(), "lgtm-git-conflict-"));
    let merged: ProcessReply;
    // Existing marker-like source text cannot become a synthetic hunk.
    let markerSize = 7;
    for (const blob of blobs) {
      for (const match of blob.matchAll(/^(<+|\|+|=+|>+)(?: |\r?$)/gm)) markerSize = Math.max(markerSize, match[1]!.length + 1);
    }
    try {
      const paths = ["ours", "base", "theirs"].map((name) => join(temporary, name));
      for (let i = 0; i < paths.length; i++) await writeFile(paths[i]!, blobs[i]!, { mode: 0o600 });
      merged = await run(folder, ["merge-file", "--stdout", "--diff3", `--marker-size=${markerSize}`, "-L", "ours", "-L", "base", "-L", "theirs", "--", ...paths], signal);
      if (merged.code < 0 || merged.code > 127) return fail("unsupported_conflict");
    } finally { await rm(temporary, { recursive: true, force: true }); }
    const segments = parseConflict(merged.stdout, markerSize);
    // merge-file adds a separator newline before a closing marker when a
    // source has no final newline. It is not part of that source side.
    const last = segments.at(-1);
    if (last?.kind === "conflict") {
      for (const [side, blob] of [[last.ours, blobs[0]!], [last.base, blobs[1]!], [last.theirs, blobs[2]!]] as const) {
        if (!blob.endsWith("\n") && side.length > 0) (side as string[])[side.length - 1] = side.at(-1)!.replace(/\n$/, "");
      }
    }
    if (!segments.some((segment) => segment.kind === "conflict")) return fail("unsupported_conflict");
    return { path, fingerprint: hash(folder, path, currentHead, index, current), head: currentHead,
      ours: blobs[0]!, base: blobs[1]!, theirs: blobs[2]!, segments,
      choices: segments.filter((segment) => segment.kind === "conflict").map(() => null) };
  };
  const checkedSelection = async (folder: string, selection: Conflict, signal: AbortSignal) => {
    const actual = await conflict(folder, selection.path, signal);
    if (actual.fingerprint !== selection.fingerprint || actual.head !== selection.head) return fail("stale_state");
    if (selection.choices.length !== actual.choices.length || selection.choices.some((choice) => choice !== null && !["ours", "theirs", "both"].includes(choice))) return fail("incomplete_choices");
    return actual;
  };
  const review = async (folder: string, base: string, signal: AbortSignal, agentRead = false, pinnedBase?: string): Promise<Review> => {
    const currentBranch = await branch(folder, signal), currentHead = await head(folder, signal);
    if (currentBranch === null || currentHead === null) return fail("invalid_ref");
    const baseHead = pinnedBase ?? await localBranch(folder, base, signal);
    const start = await startingHead(folder, signal);
    const baseline = await comparisonBase(folder, currentHead, baseHead, start, signal);
    const files = await filesBetween(folder, ["diff", baseline, currentHead], signal, agentRead);
    const paths = files.items.flatMap((row) => row.originalPath === null ? [row.path] : [row.originalPath, row.path]);
    if (agentRead) for (const path of paths) await filePath(folder, path);
    const diff = agentRead && paths.length === 0 ? { patch: "", additions: 0, deletions: 0, binary: false, truncated: false }
      : await patch(folder, ["diff", baseline, currentHead], signal, agentRead ? paths : []);
    return { head: currentHead, baseHead, branch: currentBranch, base,
      commits: await history(folder, `${baseHead}..${currentHead}`, signal, entries, start === undefined ? [] : [`^${start}`]), files,
      diff: { ...diff, truncated: diff.truncated || agentRead && files.truncated } };
  };
  const remote = async (folder: string, signal: AbortSignal) => {
    if (!(await git(folder, ["remote"], signal)).split("\n").includes("origin")) return fail("missing_remote");
    for (const args of [["config", "--get", "remote.origin.url"], ["remote", "get-url", "--all", "origin"], ["remote", "get-url", "--push", "--all", "origin"]]) {
      if ((await git(folder, args, signal)).trim().split("\n").some(forbiddenRemote)) return fail("forbidden_remote");
    }
  };
  const upstream = async (folder: string, signal: AbortSignal) => {
    const reply = await run(folder, ["rev-parse", "--verify", "--quiet", "@{upstream}"], signal);
    if (reply.code !== 0) return fail("missing_upstream");
    return reply.stdout.trim();
  };
  const mergeHead = async (folder: string, signal: AbortSignal): Promise<string | null> => {
    const reply = await run(folder, ["rev-parse", "--verify", "--quiet", "MERGE_HEAD"], signal);
    if (reply.code === 1) return null;
    if (reply.code !== 0) throw processFailure(reply);
    return reply.stdout.trim();
  };
  // Git retains the original unmerged stages in its resolve-undo index extension.
  // Only the current merge can use them; never reconstruct from client content.
  const resolvedConflicts = async (folder: string, signal: AbortSignal) => {
    if (await mergeHead(folder, signal) === null) return list<ResolvedConflict>([]);
    const records = (await git(folder, ["ls-files", "--resolve-undo", "-z"], signal)).split("\0").filter(Boolean);
    const paths = new Map<string, Set<string>>();
    for (const record of records) {
      const match = /^(100644|100755) ([0-9a-f]+) ([123])\t(.+)$/.exec(record);
      if (match === null) continue;
      const stages = paths.get(match[4]!) ?? new Set<string>();
      stages.add(match[3]!); paths.set(match[4]!, stages);
    }
    const unresolved = new Set((await completeChanges(folder, signal)).filter((row) => row.conflict).map((row) => row.path));
    const result: ResolvedConflict[] = [];
    for (const [path, stages] of paths) {
      if (stages.size !== 3 || unresolved.has(path)) continue;
      try {
        const full = await filePath(folder, path);
        result.push({ path, fingerprint: hash(path, await smallFile(full, diffBytes)) });
      } catch (error) {
        // One no-longer-readable resolved leaf cannot hide the remaining merge.
        if (error instanceof GitFailure && ["invalid_path", "output_limit"].includes(error.reason)
          || ["ENOENT", "EACCES", "ELOOP", "ENOTDIR"].includes((error as NodeJS.ErrnoException).code ?? "")) continue;
        throw error;
      }
    }
    return list(result);
  };
  const expectedMerge = async (folder: string, expectedHead: string, expectedMergeHead: string, signal: AbortSignal) => {
    const merging = await mergeHead(folder, signal);
    if (merging === null) return fail("no_merge");
    if (merging !== expectedMergeHead || await head(folder, signal) !== expectedHead) return fail("stale_state");
  };
  const fetchDeliveryBase = async (folder: string, remote: string, base: string, signal: AbortSignal) => {
    const tracking = `refs/remotes/${remote}/${base}`;
    if ((await run(folder, ["check-ref-format", tracking], signal)).code !== 0
      || (await run(folder, ["check-ref-format", `refs/heads/${base}`], signal)).code !== 0) return fail("invalid_ref");
    const symbolic = await run(folder, ["symbolic-ref", "--quiet", tracking], signal);
    if (symbolic.code === 0) return fail("invalid_ref");
    if (symbolic.code !== 1) throw processFailure(symbolic);
    // This repeatable bookkeeping fetch publishes nothing and changes no
    // checkout. Later comparison refusals must remain determinate.
    const reply = await run(folder, [...deliveryOptions, "fetch", "--upload-pack=git-upload-pack", "--no-tags", "--no-prune", "--no-recurse-submodules", "--refmap=", "--", remote,
      `+refs/heads/${base}:${tracking}`], signal);
    if (reply.code !== 0) throw processFailure(reply, "fetch");
    return ref(folder, tracking, signal);
  };
  const syncHostedBase = async (from: Repository, to: Repository, base: string, delivery: GithubDelivery, mergedHead: string, signal: AbortSignal) => {
    const result = { head: mergedHead, delivery: { ...delivery, state: "MERGED" as const }, localSync: "The local base checkout was left alone because it is dirty, on another branch, or cannot fast-forward." };
    try {
      await fetchDeliveryBase(from.path, delivery.remote, base, signal);
      await ref(from.path, mergedHead, signal);
      if (await branch(to.path, signal) !== base || (await completeChanges(to.path, signal)).length > 0 || await mergeHead(to.path, signal) !== null) return result;
      if ((await run(to.path, ["merge-base", "--is-ancestor", (await head(to.path, signal))!, mergedHead], signal)).code !== 0) return result;
      if (await branch(to.path, signal) !== base || (await completeChanges(to.path, signal)).length > 0 || await mergeHead(to.path, signal) !== null) return result;
      await mutateGit(to.path, ["-c", `branch.${base}.mergeOptions=`, "merge", "--ff-only", "--no-squash", "--commit", "--no-edit", "--no-autostash", mergedHead], signal);
      if (await head(to.path, signal) !== mergedHead) return result;
      return { ...result, localSync: "The local base checkout was fast-forwarded." };
    } catch { return { ...result, localSync: "GitHub merged the pull request. The local base checkout could not be updated; refresh it before continuing." }; }
  };
  const detectBase = async (folder: string, signal: AbortSignal, registration: boolean) => {
    const repo = await repository(folder, signal), main = dirname(repo.commonDirectory);
    const symbolic = registration ? await run(main, ["symbolic-ref", "--quiet", "refs/remotes/origin/HEAD"], signal) : undefined;
    if (symbolic?.code === 0) {
      const value = symbolic.stdout.trim();
      if (value.startsWith("refs/remotes/origin/")) {
        const name = value.slice("refs/remotes/origin/".length);
        if ((await run(main, ["check-ref-format", `refs/heads/${name}`], signal)).code === 0) {
          const local = await run(main, ["show-ref", "--verify", "--quiet", `refs/heads/${name}`], signal);
          if (local.code === 0) {
            if ([...name].length > 128) return fail("base_branch_too_long");
            return name;
          }
          if (local.code !== 1) throw processFailure(local);
        }
      }
    }
    else if (symbolic !== undefined && symbolic.code !== 1 && symbolic.code !== 128) throw processFailure(symbolic);
    const current = await branch(main, signal);
    if (registration && current !== null && await head(main, signal) === null) {
      if ([...current].length > 128) return fail("base_branch_too_long");
      return current;
    }
    if (current !== "main" && current !== "master") return fail("base_branch_required");
    const existing = await run(main, ["show-ref", "--verify", "--quiet", `refs/heads/${current}`], signal);
    if (existing.code === 1) return fail("base_branch_required");
    if (existing.code !== 0) throw processFailure(existing);
    return current;
  };
  const recordedBase = async (folder: string, signal: AbortSignal) => {
    const repo = await repository(folder, signal);
    const bases = configuration.baseBranches?.() ?? {}, recorded = bases[dirname(repo.commonDirectory)];
    if (recorded !== undefined) return recorded;
    for (const [path, base] of Object.entries(bases)) {
      try { if ((await repository(path, signal)).commonDirectory === repo.commonDirectory) return base; }
      catch { /* An unavailable unrelated repository cannot block this checkout. */ }
    }
    return undefined;
  };
  const isBaseBranch = async (folder: string, name: string, signal: AbortSignal) => {
    const recorded = await recordedBase(folder, signal);
    return recorded === undefined ? name === "main" || name === "master" : name === recorded;
  };
  const repositoryBase = async (folder: string, signal: AbortSignal) => {
    const recorded = await recordedBase(folder, signal);
    if (recorded !== undefined) return recorded;
    const repo = await repository(folder, signal);
    // Legacy reads never consult agent-writable origin/HEAD.
    const main = dirname(repo.commonDirectory), current = await branch(main, signal);
    if (current === "main" || current === "master") return current;
    const master = await run(main, ["show-ref", "--verify", "--quiet", "refs/heads/master"], signal);
    const primary = await run(main, ["show-ref", "--verify", "--quiet", "refs/heads/main"], signal);
    return primary.code !== 0 && master.code === 0 ? "master" : "main";
  };
  const mainComparison = async (source: string, destination: string, signal: AbortSignal, caller?: Caller) => {
    const from = await repository(source, signal), to = await repository(destination, signal);
    if (from.commonDirectory !== to.commonDirectory || from.path === to.path) return fail("invalid_path");
    const base = await branch(to.path, signal), sourceBranch = await branch(from.path, signal);
    if (base === null || !await isBaseBranch(to.path, base, signal) || sourceBranch === null || sourceBranch === base) return fail("invalid_ref");
    let liveBase: string | undefined;
    if (caller !== undefined) {
      if (caller._tag !== "owner") return fail("owner_required");
      const gh = githubCommands(from.path, signal, networkMs);
      const mode = await deliveryMode((args) => run(from.path, args, signal), (args) => {
        githubOperations.add(signal);
        return gh.run(args);
      });
      if (mode.mode === "local") githubOperations.delete(signal);
      if (mode.mode === "github") {
        liveBase = await gh.baseHead(mode.repository, base);
        if (await fetchDeliveryBase(from.path, mode.remote, base, signal) !== liveBase) return determinate("stale_state");
      }
    }
    const comparison = await review(from.path, base, signal, false, liveBase);
    if (liveBase === undefined && comparison.baseHead !== await head(to.path, signal)) return fail("stale_state");
    // A binary or bounded partial patch is not an all-files review. This path
    // refuses honestly instead of letting an approval imply unseen contents.
    if (comparison.commits.truncated || comparison.files.truncated || comparison.diff.truncated || comparison.diff.binary) return fail("unsupported_review");
    return { from, to, comparison };
  };
  // Tracked and nonignored untracked names, as Git lists them. The Files tab
  // never shows, reads or writes an ignored file, a credential path, or a
  // file reached through a symbolic link.
  const listedFiles = async (folder: string, signal: AbortSignal, only?: string) => {
    const reply = await run(folder, ["ls-files", "--cached", "--others", "--exclude-standard", "-z", ...(only === undefined ? [] : ["--", only])], signal, true);
    if (reply.code !== 0 && !reply.truncated) throw processFailure(reply);
    const names = reply.stdout.split("\0"); names.pop();
    return { names: [...new Set(names)].filter((name) => !isRepositoryCredentialPath(name)), truncated: reply.truncated };
  };
  const listedFile = async (folder: string, requested: string, signal: AbortSignal) => {
    if (!cleanText(requested) || isRepositoryCredentialPath(requested)) return fail("invalid_path");
    const full = await filePath(folder, requested);
    if (!(await listedFiles(folder, signal, requested)).names.includes(requested)) return fail("invalid_path");
    const info = await lstat(full);
    if (!info.isFile() || await realpath(full) !== full) return fail("invalid_path");
    return { full, size: info.size };
  };
  const folderPath = async (folder: string, requested: string | undefined) => {
    if (requested === undefined) return "";
    if (!cleanText(requested) || requested.startsWith("/") || requested.includes("\\") || isRepositoryCredentialPath(requested)) return fail("invalid_path");
    const parts = requested.split("/");
    if (parts.some((part) => part === "" || part === "." || part === ".." || part.toLowerCase() === ".git")) return fail("invalid_path");
    let current = folder;
    for (const part of parts) {
      current = join(current, part);
      const info = await lstat(current).catch(() => fail("invalid_path"));
      if (info.isSymbolicLink() || !info.isDirectory() || await exists(join(current, ".git"))) return fail("invalid_path");
    }
    return requested;
  };
  const changeOf = (change: Change): FileChange => change.conflict ? "conflict" : change.untracked ? "untracked"
    : /R/.test(change.index + change.worktree) ? "renamed" : /D/.test(change.index + change.worktree) ? "deleted"
    : /A/.test(change.index + change.worktree) ? "added" : "modified";
  const imageTypes: Readonly<Record<string, FileImageType>> = { png: "image/png", jpg: "image/jpeg", jpeg: "image/jpeg", gif: "image/gif", webp: "image/webp", svg: "image/svg+xml" };
  const withRepo = <A>(folder: string, body: (path: string, signal: AbortSignal) => Promise<A>, network = false, registration?: Repository) => operation(async (signal) => body((registration?.path === folder ? registration : await repository(folder, signal)).path, signal), network);
  const excludeManagedWorktrees = async (common: string) => {
    const info = join(common, "info");
    if (!await exists(info)) await mkdir(info, { mode: 0o700 });
    if (await directory(await root(), info) !== info) return fail("invalid_path");
    const path = join(info, "exclude");
    const previous = await exists(path) ? (await smallFile(path, 65_536)).toString("utf8") : "";
    if (previous.split(/\r?\n/).some((line) => line === "/.worktrees/" || line === ".worktrees/")) return;
    // Keep generated checkouts out of the parent's untracked changes without
    // changing a tracked .gitignore or the user's existing exclude rules.
    const file = await open(path, constants.O_WRONLY | constants.O_APPEND | constants.O_CREAT | constants.O_NOFOLLOW | constants.O_NONBLOCK, 0o600);
    try {
      if (!(await file.stat()).isFile()) return fail("invalid_path");
      await file.write(`${previous.endsWith("\n") || previous === "" ? "" : "\n"}/.worktrees/\n`);
    } finally { await file.close(); }
  };
  const integrate = (source: string, destination: string, expectedHead: string, requireCleanSource: boolean) => operation(async (signal) => {
      const from = await repository(source, signal), to = await repository(destination, signal);
      if (from.commonDirectory !== to.commonDirectory || from.path === to.path) return fail("invalid_path");
      const destinationBranch = await branch(to.path, signal);
      if (destinationBranch === null) return fail("invalid_ref");
      if (await isBaseBranch(to.path, destinationBranch, signal)) return fail("base_requires_review");
      if ((await completeChanges(to.path, signal)).length > 0 || await mergeHead(to.path, signal) !== null) return fail("dirty_destination");
      if (requireCleanSource && ((await completeChanges(from.path, signal)).length > 0 || await mergeHead(from.path, signal) !== null)) return fail("dirty_source");
      const sourceHead = await head(from.path, signal);
      if (sourceHead !== expectedHead) return fail("stale_state");
      const reply = await mutateRun(to.path, ["-c", "merge.conflictStyle=diff3", "merge", "--no-edit", "--no-autostash", sourceHead!], signal);
      if (reply.code !== 0) {
        if (await mergeHead(to.path, signal) !== null && (await completeChanges(to.path, signal)).some((change) => change.conflict)) return determinate("conflicts");
        throw processFailure(reply);
      }
      return (await head(to.path, signal))!;
    });
  return {
    detectBaseBranch: (folder, registration = false) => operation((signal) => detectBase(folder, signal, registration)),
    isBaseBranch: (folder, name) => operation((signal) => isBaseBranch(folder, name, signal)),
    repositoryBase: (folder) => operation((signal) => repositoryBase(folder, signal)),
    validateBranch: (folder, name) => withRepo(folder, async (path, signal) => {
      if ([...name].length > 128) return fail("base_branch_too_long");
      if ((await run(path, ["check-ref-format", `refs/heads/${name}`], signal)).code !== 0) return fail("invalid_ref");
      await ref(path, `refs/heads/${name}`, signal);
    }),
    projectStart: (folder, base) => withRepo(folder, async (path, signal) => {
      if ((await run(path, ["check-ref-format", `refs/heads/${base}`], signal)).code !== 0) return fail("invalid_ref");
      const local = await head(path, signal);
      if (await branch(path, signal) !== base) return fail("stale_state");
      if (local === null) return { head: null, detail: `Started from the empty local base ${base}; there is no commit to fetch from.` };
      // Optional discovery and network work share a short budget. Its failure
      // never invalidates the already observed local starting commit.
      const decisionSignal = AbortSignal.any([signal, AbortSignal.timeout(20_000)]);
      try {
        const configured = await run(path, ["config", "--get", `branch.${base}.remote`], decisionSignal);
        if (configured.code !== 0 && configured.code !== 1) throw processFailure(configured);
        const remote = configured.code === 0 ? configured.stdout.trim() : "origin";
        const remotes = (await git(path, ["remote"], decisionSignal)).trim().split("\n");
        if (remote === "." || !remotes.includes(remote)) return { head: local, detail: `Started from local base ${base}; no remote is configured for it.` };
        const merge = await run(path, ["config", "--get", `branch.${base}.merge`], decisionSignal);
        if (merge.code !== 0 && merge.code !== 1) throw processFailure(merge);
        const remoteBranch = merge.code === 0 && merge.stdout.trim().startsWith("refs/heads/") ? merge.stdout.trim().slice(11) : base;
        let fetched: string;
        try { fetched = await fetchDeliveryBase(path, remote, remoteBranch, decisionSignal); }
        catch { return { head: local, detail: `Started from local base ${base}; fetching its remote base failed.` }; }
        const ancestor = await run(path, ["merge-base", "--is-ancestor", local, fetched], decisionSignal);
        if (ancestor.code === 1) return { head: local, detail: `Started from local base ${base}; it is not an ancestor of the fetched remote base.` };
        if (ancestor.code !== 0) throw processFailure(ancestor);
        const gh = githubCommands(path, decisionSignal, 20_000);
        const mode = await deliveryMode((args) => run(path, args, decisionSignal), (args) => gh.run(args));
        // Hosted review fetches this same branch and uses the live remote base.
        // Local review and its pending counts still compare with the local base.
        if (mode.mode !== "github" || mode.remote !== remote || remoteBranch !== base) return { head: local,
          detail: `Started from local base ${base}; local delivery compares against the local base.` };
        return { head: fetched, detail: `Started from the fetched remote base for ${base}. The local base checkout was preserved.` };
      } catch { return { head: local, detail: `Started from local base ${base}; remote start discovery failed or timed out.` }; }
    }, true),
    mutationIdentity: (folder) => withRepo(folder, async (path, signal) => {
      const tree = await run(path, ["write-tree"], signal);
      return { head: await head(path, signal), branch: await branch(path, signal),
        tree: tree.code === 0 ? tree.stdout.trim() : null, mergeHead: await mergeHead(path, signal) };
    }),
    discover: () => operation(async (signal) => {
      const base = await root(), pending = [{ path: base, depth: 0 }], items: Repository[] = [];
      let visited = 0, truncated = false;
      while (pending.length > 0) {
        const next = pending.shift()!;
        if (++visited > 500 || signal.aborted) { truncated = true; break; }
        if (await exists(join(next.path, ".git"))) {
          try { items.push(await repository(next.path, signal)); } catch (error) { if (error instanceof GitFailure && ["timeout", "cancelled", "output_limit", "git_unavailable"].includes(error.reason)) throw error; }
          if (items.length > entries) { truncated = true; break; }
          continue;
        }
        if (next.depth >= 3) { truncated = true; continue; }
        const directoryHandle = await opendir(next.path);
        for await (const entry of directoryHandle) {
          if (!entry.isDirectory() || entry.name.startsWith(".") || entry.name === "node_modules") continue;
          if (pending.length + visited >= 500) { truncated = true; break; }
          pending.push({ path: join(next.path, entry.name), depth: next.depth + 1 });
        }
      }
      return { items: items.slice(0, entries), truncated };
    }),
    register: (folder) => operation((signal) => repository(folder, signal)),
    listFolder: (folder, requested) => withRepo(folder, async (path, signal) => {
      const at = await folderPath(path, requested), prefix = at === "" ? "" : `${at}/`;
      const listed = await listedFiles(path, signal);
      const status = await run(path, ["status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignore-submodules=none"], signal);
      if (status.code !== 0) throw processFailure(status);
      const changes = new Map(parseChanges(status.stdout).map((change) => [change.path, changeOf(change)] as const));
      const folders = new Map<string, FileEntry>(), files: FileEntry[] = [];
      for (const name of listed.names) {
        if (!name.startsWith(prefix)) continue;
        const rest = name.slice(prefix.length), slash = rest.indexOf("/");
        if (signal.aborted) return fail("cancelled");
        if (slash >= 0) {
          const child = rest.slice(0, slash), childPath = prefix + child;
          const changed = changes.get(name) !== undefined;
          const known = folders.get(child);
          if (known === undefined) folders.set(child, { name: child, path: childPath, kind: "folder", size: null, change: changed ? "modified" : null });
          else if (changed && known.change === null) folders.set(child, { ...known, change: "modified" });
          continue;
        }
        const info = await lstat(join(path, name)).catch(() => undefined);
        if (info === undefined || !info.isFile()) continue;
        files.push({ name: rest, path: name, kind: "file", size: info.size, change: changes.get(name) ?? null });
      }
      // A folder that holds only symbolic links or nested checkouts is left out.
      const kept: FileEntry[] = [];
      for (const entry of [...folders.values()].sort((a, b) => a.name.localeCompare(b.name))) {
        const info = await lstat(join(path, entry.path)).catch(() => undefined);
        if (info?.isDirectory() && !info.isSymbolicLink() && !await exists(join(path, entry.path, ".git"))) kept.push(entry);
      }
      const items = [...kept, ...files.sort((a, b) => a.name.localeCompare(b.name))];
      return { folder: at, items: items.slice(0, 500), truncated: listed.truncated || items.length > 500 };
    }),
    readFile: (folder, requested) => withRepo(folder, async (path, signal) => {
      const { full, size } = await listedFile(path, requested, signal);
      if (size > fileTextLimit) return { kind: "large", path: requested, size };
      let bytes: Buffer;
      try { bytes = await smallFile(full, fileTextLimit); }
      catch (error) { if (error instanceof GitFailure && error.reason === "output_limit") return { kind: "large", path: requested, size }; throw error; }
      const version = sha256(bytes), mediaType = imageTypes[(/\.([a-z0-9]+)$/i.exec(requested)?.[1] ?? "").toLowerCase()];
      if (mediaType !== undefined) return { kind: "image", path: requested, size: bytes.length, version, mediaType, data: bytes.toString("base64") };
      if (bytes.includes(0)) return { kind: "binary", path: requested, size: bytes.length };
      try { return { kind: "text", path: requested, size: bytes.length, version, text: new TextDecoder("utf-8", { fatal: true }).decode(bytes) }; }
      catch { return { kind: "binary", path: requested, size: bytes.length }; }
    }),
    writeFile: (folder, requested, version, bytes) => withRepo(folder, async (path, signal) => {
      if (bytes.length > fileTextLimit) return fail("output_limit");
      const { full, size } = await listedFile(path, requested, signal);
      if (size > fileTextLimit || sha256(await smallFile(full, fileTextLimit)) !== version) return determinate("stale_state");
      mutations.add(signal);
      try { await replaceFile(path, requested, version, bytes, fileTextLimit); }
      catch (error) {
        if (error instanceof ReplaceFileError && error.uncertain) throw new GitFailure({ reason: "git_failed", uncertain: true });
        return determinate("stale_state");
      }
      return sha256(bytes);
    }),
    contextFile: (folder, requested) => withRepo(folder, async (path) => {
      if (isRepositoryCredentialPath(requested)) return fail("invalid_path");
      const full = await filePath(path, requested);
      if (!(await lstat(full)).isFile() || await realpath(full) !== full) return fail("invalid_path");
    }),
    contextFiles: (folder, query) => withRepo(folder, async (path, signal) => {
      // Git supplies tracked and nonignored untracked names, never contents.
      // Discard an incomplete final name when the subprocess budget is hit.
      const reply = await run(path, ["ls-files", "--cached", "--others", "--exclude-standard", "-z"], signal, true);
      if (reply.code !== 0 && !reply.truncated) throw processFailure(reply);
      const names = reply.stdout.split("\0"); names.pop();
      const items: string[] = [];
      let truncated = reply.truncated;
      for (const name of new Set(names)) {
        if (isRepositoryCredentialPath(name)) continue;
        if (!name.toLowerCase().includes(query.toLowerCase())) continue;
        if (signal.aborted) return fail("cancelled");
        try {
          const full = await filePath(path, name);
          if (!(await lstat(full)).isFile() || await realpath(full) !== full) continue;
        } catch (error) {
          if (error instanceof GitFailure && error.reason === "invalid_path" || ["ENOENT", "ENOTDIR"].includes((error as NodeJS.ErrnoException).code ?? "")) continue;
          throw error;
        }
        if (items.length === entries) { truncated = true; break; }
        items.push(name);
      }
      return { items, truncated };
    }),
    createRepository: (name) => operation(async (signal) => {
      const base = await root(), path = join(base, slug(name));
      if (await exists(path)) return fail("collision");
      await mkdir(path, { mode: 0o700 });
      await git(path, ["init", "--initial-branch=main", "--", path], signal);
      return repository(path, signal);
    }),
    branches: (folder) => withRepo(folder, async (path, signal) => {
      const current = await branch(path, signal);
      const text = await git(path, ["for-each-ref", `--count=${entries + 1}`, "--format=%(refname:short)%00%(objectname)%00%(upstream:short)", "refs/heads/"], signal);
      return list(text.split("\n").filter(Boolean).map((row) => {
        const [name, head, upstream] = row.split("\0");
        return { name: name!, head: head!, current: name === current, upstream: upstream || null };
      }));
    }),
    worktrees: (folder) => withRepo(folder, worktrees),
    createWorktree: (folder, name, from = "HEAD", branchPrefix) => withRepo(folder, async (path, signal) => {
      const repo = await repository(path, signal);
      const currentHead = await head(path, signal);
      const start = currentHead === null && (from === "HEAD" || from === await branch(path, signal)) ? null : await ref(path, from, signal);
      const stem = slug(name);
      const prefix = branchPrefix === undefined ? "" : branchPrefix.split("/").map(slug).join("/") + "/";
      const main = dirname(repo.commonDirectory), directoryPath = join(main, ".worktrees");
      if (await exists(directoryPath)) {
        if ((await lstat(directoryPath)).isSymbolicLink() || await directory(await root(), directoryPath) !== directoryPath) return fail("invalid_path");
      } else await mkdir(directoryPath, { mode: 0o700 });
      await excludeManagedWorktrees(repo.commonDirectory);
      for (let i = 1; i <= 100; i++) {
        const leaf = i === 1 ? stem : `${stem}-${i}`, name = prefix + leaf;
        const target = join(directoryPath, prefix.replaceAll("/", "-") + leaf);
        if (await exists(target) || (await run(path, ["show-ref", "--verify", "--quiet", `refs/heads/${name}`], signal)).code === 0) continue;
        // Reserve an empty directory exclusively. On refusal we can remove
        // that exact empty directory without touching a competing allocation.
        try { await mkdir(target, { mode: 0o700 }); }
        catch (error) { if ((error as NodeJS.ErrnoException).code === "EEXIST") continue; throw error; }
        const identity = await lstat(target);
        const clearEmpty = async () => {
          if (!await exists(target)) return;
          const now = await lstat(target);
          if (now.dev === identity.dev && now.ino === identity.ino) await rmdir(target);
        };
        let reply: ProcessReply;
        try {
          reply = await run(path, ["worktree", "add", ...(start === null ? ["--orphan"] : []), "-b", name, "--", target, ...(start === null ? [] : [start])], signal);
        } catch (error) {
          // A cancelled/timed-out child has already exited. Cleanup has its own
          // finite budget, rather than inheriting its exhausted signal.
          // Cleanup is best effort. A changed partial checkout is evidence,
          // and neither that refusal nor ENOTEMPTY may replace this failure.
          try { await rollbackAllocation(path, { path: target, branch: name, head: start, locked: false, available: true }, AbortSignal.timeout(10_000)); } catch { /* preserve external work */ }
          try { await clearEmpty(); } catch { /* keep a changed reservation */ }
          throw error;
        }
        if (reply.code !== 0) {
          if (/already exists|already registered/i.test(reply.stderr)) { await clearEmpty(); continue; }
          try { await rollbackAllocation(path, { path: target, branch: name, head: start, locked: false, available: true }, AbortSignal.timeout(10_000)); } catch { /* preserve external work */ }
          try { await clearEmpty(); } catch { /* keep a changed reservation */ }
          throw processFailure(reply);
        }
        return { path: target, branch: name, head: start, locked: false, available: true };
      }
      return fail("collision");
    }),
    // Only an unchanged, clean checkout allocated by a failed start is eligible.
    // Never force removal: external edits or commits keep their recovery evidence.
    rollbackWorktree: (folder, worktree) => withRepo(folder, (path, signal) => rollbackAllocation(path, worktree, signal)),
    changes: (folder, agentRead = false) => withRepo(folder, async (path, signal) => {
      const changes = parseChanges(await git(path, ["status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignore-submodules=none"], signal));
      return list(agentRead ? changes.filter(readable) : changes);
    }),
    diff: (folder, options = {}) => withRepo(folder, (path, signal) => diff(path, options, signal)),
    history: (folder, limit) => withRepo(folder, async (path, signal) => await head(path, signal) === null ? list([]) : history(path, "HEAD", signal, bound(limit, entries, entries))),
    commitFiles: (folder, commit, agentRead = false) => withRepo(folder, async (path, signal) => filesBetween(path, ["diff-tree", "--root", "--no-commit-id", "-r", "-m", "--first-parent", await ref(path, commit, signal)], signal, agentRead)),
    state: (folder, target, registration, checkedState) => withRepo(folder, async (path, signal) => {
      const currentHead = checkedState ? checkedState.head : await head(path, signal);
      const currentBranch = checkedState ? checkedState.branch : await branch(path, signal);
      const upstreamReply = checkedState ? undefined : await run(path, ["rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"], signal);
      const upstreamName = checkedState ? checkedState.upstream : upstreamReply?.code === 0 ? upstreamReply.stdout.trim() : null;
      let ahead = checkedState?.ahead ?? 0, behind = checkedState?.behind ?? 0, behindParent = 0, pending = 0, pendingSince: string | null = null;
      if (!checkedState && currentHead !== null && upstreamName !== null) {
        const counts = (await git(path, ["rev-list", "--left-right", "--count", `${await ref(path, upstreamName, signal)}...${currentHead}`, "--"], signal)).trim().split(/\s+/);
        behind = Number(counts[0]); ahead = Number(counts[1]);
      }
      if (currentHead !== null && target !== undefined) {
        const targetHead = await ref(path, target, signal);
        const start = await startingHead(path, signal), exclusions = start === undefined ? [] : [`^${start}`];
        behindParent = Number((await git(path, ["rev-list", "--count", `${currentHead}..${targetHead}`, ...exclusions, "--"], signal)).trim());
        const range = `${targetHead}..${currentHead}`;
        pending = Number((await git(path, ["rev-list", "--count", range, ...exclusions, "--"], signal)).trim());
        if (pending > 0) {
          const dates = (await git(path, ["log", "--format=%cI", range, ...exclusions, "--"], signal)).trim().split("\n");
          pendingSince = dates.reduce((oldest, date) => Date.parse(date) < Date.parse(oldest) ? date : oldest);
        }
      }
      return { head: currentHead, branch: currentBranch, upstream: upstreamName, mergeHead: checkedState ? checkedState.mergeHead : await mergeHead(path, signal), ahead, behind, behindParent, pending, pendingSince };
    }, false, registration),
    stage: (folder, requested) => withRepo(folder, async (path, signal) => {
      const change = (await completeChanges(path, signal)).find((change) => change.path === requested);
      if (change === undefined) return fail("invalid_path");
      if (change.conflict) return fail("conflicts");
      const paths = stagePaths(change);
      for (const file of paths) await filePath(path, file);
      await mutateGit(path, ["add", "--", ...paths], signal);
    }),
    unstage: (folder, requested) => withRepo(folder, async (path, signal) => {
      const change = (await completeChanges(path, signal)).find((change) => change.path === requested && change.staged);
      if (change === undefined) return fail("invalid_path");
      await resetPaths(path, changePaths(change), signal);
    }),
    stageAll: (folder) => withRepo(folder, async (path, signal) => {
      const paths = (await completeChanges(path, signal)).filter((change) => !change.conflict).flatMap(stagePaths);
      for (const file of paths) await filePath(path, file);
      if (paths.length > 0) await mutateGit(path, ["add", "--", ...new Set(paths)], signal);
    }),
    unstageAll: (folder) => withRepo(folder, async (path, signal) => resetPaths(path, (await completeChanges(path, signal)).filter((change) => change.staged && !change.conflict).flatMap(changePaths), signal)),
    planDiscard: (folder, paths) => withRepo(folder, (path, signal) => planDiscard(path, paths, signal)),
    discard: (folder, plan, confirmed) => withRepo(folder, async (path, signal) => {
      if (confirmed !== true) return fail("confirmation_required");
      const actual = await planDiscard(path, plan.paths, signal);
      if (actual.fingerprint !== plan.fingerprint || JSON.stringify(actual.paths) !== JSON.stringify(plan.paths)) return fail("stale_state");
      const changes = await completeChanges(path, signal);
      for (const file of actual.paths) {
        const change = changes.find((change) => change.path === file)!;
        const full = await filePath(path, file);
        if (change.untracked) { mutations.add(signal); await rm(full); }
        else await mutateGit(path, ["restore", "--worktree", "--", file], signal);
      }
    }),
    commit: (folder, message, file) => withRepo(folder, async (path, signal) => {
      if (typeof message !== "string" || message.trim() === "" || message.includes("\0") || Buffer.byteLength(message) > 16_384) return fail("invalid_message");
      const changes = await completeChanges(path, signal);
      if (changes.some((change) => change.conflict)) return fail("conflicts");
      if (file !== undefined) {
        // One file only. Its change is added, then committed with --only, so
        // other staged files stay staged and out of this commit.
        const change = changes.find((row) => row.path === file);
        if (change === undefined || isRepositoryCredentialPath(change.path)) return fail("nothing_staged");
        if (await mergeHead(path, signal) !== null) return fail("conflicts");
        const paths = changePaths(change);
        for (const name of paths) await filePath(path, name);
        await mutateGit(path, ["add", "--all", "--", ...paths], signal);
        await mutateGit(path, ["commit", "-m", message, "--only", "--", ...paths], signal);
        return (await head(path, signal))!;
      }
      if (!changes.some((change) => change.staged) && await mergeHead(path, signal) === null) return fail("nothing_staged");
      await mutateGit(path, ["commit", "-m", message], signal);
      return (await head(path, signal))!;
    }),
    fetch: (folder) => withRepo(folder, async (path, signal) => { await remote(path, signal); await mutateGit(path, ["fetch", "--prune", "origin"], signal, "fetch"); }, true),
    pull: (folder) => withRepo(folder, async (path, signal) => {
      if ((await completeChanges(path, signal)).length > 0) return fail("dirty_destination");
      await remote(path, signal); await upstream(path, signal);
      const reply = await mutateRun(path, ["pull", "--ff-only", "--no-rebase"], signal);
      if (reply.code !== 0) { if (!reply.truncated && reply.code > 0 && /^fatal: Not possible to fast-forward, aborting\.\r?$/m.test(reply.stderr)) return determinate("pull_diverged"); throw processFailure(reply, "pull"); }
    }, true),
    push: (folder) => withRepo(folder, async (path, signal) => {
      await remote(path, signal);
      const name = await branch(path, signal);
      if (name === null) return fail("invalid_ref");
      await localBranch(path, name, signal);
      await mutateGit(path, ["push", "--set-upstream", "origin", `refs/heads/${name}:refs/heads/${name}`], signal, "push");
    }, true),
    mergeTeam: (source, destination, expectedHead) => integrate(source, destination, expectedHead, true),
    updateFromParent: (source, destination, expectedHead) => integrate(source, destination, expectedHead, false),
    review: (folder, base, agentRead = false) => withRepo(folder, (path, signal) => review(path, base, signal, agentRead)),
    prepareDelivery: (source, reviewed, text, caller, destination) => operation(async (signal) => {
      if (caller?._tag !== "owner") return fail("owner_required");
      const from = await repository(source, signal), to = await repository(destination, signal);
      if (from.commonDirectory !== to.commonDirectory || from.path === to.path) return fail("invalid_path");
      if (await branch(from.path, signal) !== reviewed.branch || await localBranch(from.path, reviewed.branch, signal) !== reviewed.head) return fail("stale_state");
      const gh = githubCommands(from.path, signal, networkMs);
      const mode = await deliveryMode((args) => run(from.path, args, signal), gh.run);
      if (mode.mode === "local") {
        if (await localBranch(from.path, reviewed.base, signal) !== reviewed.baseHead) return fail("stale_state");
        return { delivery: mode };
      }
      if (!cleanText(text.title, 1024) || Buffer.byteLength(text.body) > 32 * 1024 || text.body.includes("\0")) return fail("invalid_message");
      const args = ["--repo", mode.repository];
      const list = async (state: "open" | "merged") => {
        const values = await gh.json(["pr", "list", ...args, "--head", reviewed.branch, "--base", reviewed.base, "--state", state, "--json", gh.fields]);
        if (!Array.isArray(values)) return fail("github_refused");
        const own = values.filter((value) => value?.isCrossRepository === false && (state !== "merged" || value.headRefOid === reviewed.head));
        if (own.length > 1) return fail("github_refused");
        return own;
      };
      if ((await list("open")).length === 0) {
        const merged = (await list("merged"))[0];
        if (merged !== undefined) {
          const delivery = checkedPullRequest(merged, mode.repository, mode.remote, reviewed.branch, reviewed.base, reviewed.head);
          if (delivery.state !== "MERGED" || typeof merged.mergeCommit?.oid !== "string" || !/^[a-f0-9]{40,64}$/.test(merged.mergeCommit.oid)) return fail("github_refused");
          const synced = await syncHostedBase(from, to, reviewed.base, { ...delivery, remoteBaseHead: reviewed.baseHead }, merged.mergeCommit.oid, signal);
          return { delivery: synced.delivery, merged: { head: synced.head, localSync: synced.localSync } };
        }
      }
      if (await gh.baseHead(mode.repository, reviewed.base) !== reviewed.baseHead) return fail("stale_state");
      await mutateGit(from.path, [...deliveryOptions, "-c", `remote.${mode.remote}.mirror=false`, "-c", "push.followTags=false", "push", "--no-mirror", "--no-force", "--receive-pack=git-receive-pack", "--no-follow-tags", "--recurse-submodules=no", "--", mode.pushUrl,
        `${reviewed.head}:refs/heads/${reviewed.branch}`], signal, "push");
      // URL pushes skip named-remote bookkeeping. Replace only this tracking
      // ref, never a symbolic target that could name the owner's local branch.
      // The branch is already published. A stale conflicting local ref must
      // not prevent creation of its pull request or make publication uncertain.
      try { await git(from.path, ["update-ref", "--no-deref", `refs/remotes/${mode.remote}/${reviewed.branch}`, reviewed.head], signal); }
      catch { /* local tracking bookkeeping is best effort */ }
      let found = await list("open");
      if (found.length === 0) {
        const created = await gh.run(["pr", "create", ...args, "--head", reviewed.branch, "--base", reviewed.base, "--title", text.title, "--body", text.body]);
        if (created.code !== 0) {
          if (created.code > 0 && !created.truncated) return determinate("github_create_refused");
          return fail("github_refused");
        }
        // Branch strings can be numeric; never pass one as a PR selector.
        found = await list("open");
      } else {
        const current = checkedPullRequest(found[0], mode.repository, mode.remote, reviewed.branch, reviewed.base, reviewed.head);
        const edited = await gh.run(["pr", "edit", String(current.number), ...args, "--body-file", "-"], text.body);
        if (edited.code !== 0) return fail("github_refused");
        found = await list("open");
      }
      const delivery = checkedPullRequest(found[0], mode.repository, mode.remote, reviewed.branch, reviewed.base, reviewed.head);
      if (await gh.baseHead(mode.repository, reviewed.base) !== reviewed.baseHead) return fail("stale_state");
      return { delivery: { ...delivery, remoteBaseHead: reviewed.baseHead } };
    }, true),
    approveDelivery: (source, destination, reviewed, delivery, caller, confirmed) => operation(async (signal) => {
      if (caller?._tag !== "owner") return fail("owner_required");
      if (!confirmed) return fail("confirmation_required");
      const from = await repository(source, signal), to = await repository(destination, signal);
      if (from.commonDirectory !== to.commonDirectory || from.path === to.path) return fail("invalid_path");
      if (await branch(from.path, signal) !== reviewed.branch || await localBranch(from.path, reviewed.branch, signal) !== reviewed.head) return fail("stale_state");
      const gh = githubCommands(from.path, signal, networkMs);
      const mode = await deliveryMode((args) => run(from.path, args, signal), gh.run);
      if (mode.mode !== "github" || mode.repository !== delivery.repository || mode.remote !== delivery.remote) return fail("stale_state");
      const pr = await gh.json(["pr", "view", String(delivery.number), "--repo", mode.repository, "--json", gh.fields]);
      const fresh = checkedPullRequest(pr, mode.repository, mode.remote, reviewed.branch, reviewed.base, reviewed.head);
      let mergedHead: string;
      if (fresh.state === "MERGED") {
        // A new owner request reconciles a confirmed remote effect without
        // replaying the original uncertain command or issuing another merge.
        if (typeof pr.mergeCommit?.oid !== "string" || !/^[a-f0-9]{40,64}$/.test(pr.mergeCommit.oid)) return fail("github_refused");
        mergedHead = pr.mergeCommit.oid;
      } else {
        if (fresh.state !== "OPEN" || pr.mergeable !== "MERGEABLE" || pr.mergeStateStatus !== "CLEAN") throw new GitFailure({ reason: "github_refused", determinate: true, pullRequestState: fresh.state });
        const liveBase = await gh.baseHead(mode.repository, reviewed.base);
        if (liveBase !== delivery.remoteBaseHead || liveBase !== reviewed.baseHead) return fail("github_base_moved");
        mutations.add(signal);
        const merged = await gh.json(["api", "--method", "PUT", `repos/${mode.repository}/pulls/${delivery.number}/merge`,
          "-f", "merge_method=merge", "-f", `sha=${reviewed.head}`]);
        if (merged.merged !== true || typeof merged.sha !== "string" || !/^[a-f0-9]{40,64}$/.test(merged.sha)) return fail("github_refused");
        mergedHead = merged.sha;
      }
      return syncHostedBase(from, to, reviewed.base, { ...fresh, remoteBaseHead: delivery.remoteBaseHead }, mergedHead, signal);
    }, true),
    reviewMain: (source, destination, caller) => operation(async (signal) => (await mainComparison(source, destination, signal, caller)).comparison, caller !== undefined),
    approveMain: (source, destination, reviewed, caller, confirmed) => operation(async (signal) => {
      // Check trusted authority again at execution, including caller-less and
      // piece callers. No report or payload boolean supplies this credential.
      if (caller?._tag !== "owner") return fail("owner_required");
      if (confirmed !== true) return fail("confirmation_required");
      const from = await repository(source, signal), to = await repository(destination, signal);
      if (from.commonDirectory !== to.commonDirectory || from.path === to.path) return fail("invalid_path");
      if (await branch(to.path, signal) !== reviewed.base || !await isBaseBranch(to.path, reviewed.base, signal)
        || await branch(from.path, signal) !== reviewed.branch) return fail("stale_state");
      const actual = await head(to.path, signal);
      if (actual !== null) {
        const contains = async (value: string) => {
          const result = await run(to.path, ["merge-base", "--is-ancestor", value, actual], signal);
          if (result.code !== 0 && result.code !== 1) throw processFailure(result);
          return result.code === 0;
        };
        if (await contains(reviewed.baseHead) && await contains(reviewed.head)) return actual;
      }
      const { comparison } = await mainComparison(source, destination, signal);
      if (comparison.head !== reviewed.head || comparison.baseHead !== reviewed.baseHead
        || comparison.branch !== reviewed.branch || comparison.base !== reviewed.base) return fail("stale_state");
      if ((await completeChanges(to.path, signal)).length > 0 || await mergeHead(to.path, signal) !== null) return fail("dirty_destination");
      if ((await completeChanges(from.path, signal)).length > 0 || await mergeHead(from.path, signal) !== null) return fail("dirty_source");
      // Terminal edits are outside the engine queue. Recheck both heads and
      // branch identities immediately before launching Git, then merge the
      // reviewed object ID rather than resolving a mutable source branch.
      if (await head(from.path, signal) !== reviewed.head || await head(to.path, signal) !== reviewed.baseHead
        || await branch(from.path, signal) !== reviewed.branch || await branch(to.path, signal) !== reviewed.base) return fail("stale_state");
      // This approval promises a committed merge of this object. Repository
      // merge defaults must not turn it into a squash, an uncommitted merge,
      // a different strategy, or a fast-forward-only refusal on divergence.
      const reply = await mutateRun(to.path, ["-c", `branch.${reviewed.base}.mergeOptions=`, "-c", "merge.conflictStyle=diff3",
        "merge", "--strategy=ort", "--ff", "--no-squash", "--commit", "--no-edit", "--no-autostash", reviewed.head], signal);
      if (reply.code !== 0) {
        if (await mergeHead(to.path, signal) !== null && (await completeChanges(to.path, signal)).some((change) => change.conflict)) return determinate("conflicts");
        throw processFailure(reply);
      }
      // A zero exit alone is insufficient. Read back the actual destination
      // and prove it contains the reviewed object before any durable success.
      // An already-contained object is a valid no-op; HEAD need not move.
      const merged = await head(to.path, signal);
      if (merged === null || await branch(to.path, signal) !== reviewed.base) return fail("stale_state");
      const ancestry = await run(to.path, ["merge-base", "--is-ancestor", reviewed.head, merged], signal);
      if (ancestry.code === 1) return fail("git_failed");
      if (ancestry.code !== 0) throw processFailure(ancestry);
      return merged;
    }),
    abortMerge: (folder, expectedHead, expectedMergeHead, confirmed) => withRepo(folder, async (path, signal) => {
      if (confirmed !== true) return fail("confirmation_required");
      await expectedMerge(path, expectedHead, expectedMergeHead, signal);
      const beforeIndex = await git(path, ["ls-files", "--stage", "-z"], signal);
      const reply = await mutateRun(path, ["merge", "--abort"], signal);
      if (reply.code !== 0) {
        const overlap = /^error: Entry '[^\r\n]+' not uptodate\. Cannot merge\.\r?\n(?:fatal: )?Could not reset index file to revision 'HEAD'\.\r?\n?$/.test(reply.stderr);
        if (reply.code > 0 && !reply.truncated && overlap && await mergeHead(path, signal) === expectedMergeHead
          && await head(path, signal) === expectedHead && await git(path, ["ls-files", "--stage", "-z"], signal) === beforeIndex) return determinate("abort_blocked");
        throw processFailure(reply);
      }
      if (await mergeHead(path, signal) !== null || await head(path, signal) !== expectedHead) return fail("stale_state");
    }),
    resolvedConflicts: (folder) => withRepo(folder, resolvedConflicts),
    reopen: (folder, requested, expectedHead, expectedMergeHead, fingerprint, confirmed) => withRepo(folder, async (path, signal) => {
      if (confirmed !== true) return fail("confirmation_required");
      await filePath(path, requested);
      await expectedMerge(path, expectedHead, expectedMergeHead, signal);
      const resolved = await resolvedConflicts(path, signal);
      if (resolved.truncated) return fail("output_limit");
      const seen = resolved.items.find((row) => row.path === requested);
      if (seen === undefined) return fail("unsupported_conflict");
      if (seen.fingerprint !== fingerprint) return fail("stale_state");
      await mutateGit(path, ["checkout", "--conflict=diff3", "-m", "--", requested], signal);
      await conflict(path, requested, signal);
    }),
    conflict: (folder, path, agentRead = false) => withRepo(folder, (base, signal) => {
      if (agentRead && isRepositoryCredentialPath(path)) return fail("invalid_path");
      return conflict(base, path, signal);
    }),
    choose: (folder, selection, hunk, choice) => withRepo(folder, async (path, signal) => {
      const actual = await checkedSelection(path, selection, signal);
      if (!Number.isInteger(hunk) || hunk < 0 || hunk >= actual.choices.length || !["ours", "theirs", "both"].includes(choice)) return fail("incomplete_choices");
      const choices = [...selection.choices]; choices[hunk] = choice;
      return { ...actual, choices };
    }),
    resolve: (folder, selection) => withRepo(folder, async (path, signal) => {
      const actual = await checkedSelection(path, selection, signal);
      if (selection.choices.some((choice) => choice === null)) return fail("incomplete_choices");
      let index = 0;
      const content = actual.segments.map((segment) => {
        if (segment.kind === "context") return segment.context.join("");
        const choice: ConflictChoice = selection.choices[index++]!;
        if (choice === "ours") return segment.ours.join("");
        if (choice === "theirs") return segment.theirs.join("");
        const ours = segment.ours.join(""), theirs = segment.theirs.join("");
        return ours + (ours !== "" && theirs !== "" && !ours.endsWith("\n") ? "\n" : "") + theirs;
      }).join("");
      const full = await filePath(path, actual.path), info = await stat(full);
      const temporary = join(dirname(full), `.lgtm-resolve-${randomUUID()}.tmp`);
      let created = false;
      try {
        await writeFile(temporary, content, { flag: "wx", mode: info.mode & 0o777 });
        created = true;
        await checkedSelection(path, selection, signal);
        mutations.add(signal);
        await rename(temporary, full);
      } finally { if (created) await rm(temporary, { force: true }); }
      // Clear unmerged stages back to HEAD, preserving the resolved working
      // file for the mock's separate review-and-stage step.
      await resetPaths(path, [actual.path], signal);
    }),
  };
};
