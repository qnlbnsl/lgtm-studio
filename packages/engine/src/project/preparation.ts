import { constants } from "node:fs";
import { mkdir, open, opendir, lstat, realpath, type FileHandle } from "node:fs/promises";
import { isAbsolute, join } from "node:path";
import { validCopyPattern, WorktreePreparation } from "@lgtm/contracts";
import * as Schema from "effect/Schema";
import * as Effect from "effect/Effect";
import { refused } from "../commands/internal.ts";
import { gitProcess } from "./git/process.ts";
import { cleanText, filePath, smallFile } from "./git/paths.ts";

class CopyRefusal extends Error {
  constructor(pattern: string) { super(`Worktree preparation refused pattern ${JSON.stringify(pattern)}: unsafe path or copy limit.`); }
}
export const isPreparationRefusal = (reason: string) => {
  const match = /^Worktree preparation refused pattern (.+): unsafe path or copy limit\.$/.exec(reason);
  if (!match) return false;
  try { const pattern: unknown = JSON.parse(match[1]!); return typeof pattern === "string" && validCopyPattern(pattern); }
  catch { return false; }
};

const limits = { files: 128, fileBytes: 1024 * 1024, totalBytes: 8 * 1024 * 1024, entries: 512 };
const noDirectory = constants.O_RDONLY | constants.O_DIRECTORY | constants.O_NOFOLLOW;
const at = (dir: FileHandle, name: string) => `/proc/self/fd/${dir.fd}/${name}`;
const missing = (error: unknown) => (error as NodeJS.ErrnoException).code === "ENOENT";
// Node has no openat. Linux descriptor paths keep each traversal anchored to
// an opened directory. Every child is opened with NOFOLLOW, including roots.
// An unsupported host refuses copying rather than falling back to path races.
const directory = async (path: string): Promise<FileHandle> => {
  if (!isAbsolute(path)) throw new Error();
  let handle = await open("/", noDirectory);
  try {
    for (const part of path.split("/").filter(Boolean)) {
      if (part === "." || part === "..") throw new Error();
      const next = await open(at(handle, part), noDirectory);
      await handle.close(); handle = next;
    }
    return handle;
  } catch (error) { await handle.close(); throw error; }
};
const childDirectory = async (parent: FileHandle, name: string, create: boolean) => {
  if (create) await mkdir(at(parent, name), { mode: 0o700 }).catch(error => { if (error.code !== "EEXIST") throw error; });
  return open(at(parent, name), noDirectory);
};

export const copyPreparationFiles = async (source: string, destination: string, patterns: ReadonlyArray<string>) => {
  let pattern = patterns.find(pattern => !validCopyPattern(pattern)) ?? patterns[0] ?? "";
  try {
    if (process.platform !== "linux" && patterns.length > 0) throw new Error();
    if (patterns.length > 16 || patterns.some(pattern => !validCopyPattern(pattern))) throw new Error();
    if (patterns.length === 0) return { copied: 0, skipped: 0 };
    const from = await directory(source);
    let to: FileHandle | undefined;
    let count = 0, bytes = 0, copied = 0, skipped = 0;
    const seen = new Set<string>();
    try {
      to = await directory(destination);
      const copy = async (src: FileHandle, parts: string[], prefix: string[], dest: FileHandle, matched = false): Promise<void> => {
        // Detect an opened directory moved away from its recorded location.
        if (await realpath(at(src, ".")) !== join(source, ...prefix)
          || await realpath(at(dest, ".")) !== join(destination, ...prefix)) throw new Error();
        const part = parts[0]!;
        if (part.includes("*")) {
          const [before, after] = part.split("*") as [string, string];
          const listing = await opendir(at(src, "."));
          let listed = 0;
          for await (const entry of listing) {
            if (++listed > limits.entries) throw new Error();
            if (entry.name.length >= before.length + after.length && entry.name.startsWith(before) && entry.name.endsWith(after)) {
              const target = await lstat(at(dest, entry.name)).catch(error => { if (missing(error)) return undefined; throw error; });
              if (entry.isSymbolicLink() || target?.isSymbolicLink() || entry.name.includes("*")
                || !validCopyPattern([...prefix, entry.name, ...parts.slice(1)].join("/"))) { skipped++; continue; }
              await copy(src, [entry.name, ...parts.slice(1)], prefix, dest, true);
            }
          }
          return;
        }
        let info;
        try { info = await lstat(at(src, part)); } catch (error) { if (missing(error)) return; throw error; }
        if (info.isSymbolicLink()) { if (matched) { skipped++; return; } throw new Error(); }
        if (parts.length > 1) {
          if (!info.isDirectory()) return;
          const existing = await lstat(at(dest, part)).catch(error => { if (missing(error)) return undefined; throw error; });
          if (existing && (!existing.isDirectory() || existing.isSymbolicLink())) { if (matched) { skipped++; return; } throw new Error(); }
          const child = await childDirectory(src, part, false);
          let target: FileHandle | undefined;
          try { target = await childDirectory(dest, part, true); await copy(child, parts.slice(1), [...prefix, part], target, matched); }
          finally { await child.close(); await target?.close(); }
          return;
        }
        if (!info.isFile()) return;
        const key = [...prefix, part].join("/");
        if (seen.has(key)) return;
        seen.add(key);
        if (++count > limits.files || info.size > limits.fileBytes) throw new Error();
        const input = await open(at(src, part), constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
        try {
          const checked = await input.stat();
          if (!checked.isFile() || checked.size > limits.fileBytes) throw new Error();
          const data = Buffer.alloc(limits.fileBytes + 1);
          let size = 0;
          while (size < data.length) {
            const next = await input.read(data, size, data.length - size, size);
            if (next.bytesRead === 0) break;
            size += next.bytesRead;
          }
          bytes += size;
          if (size > limits.fileBytes || bytes > limits.totalBytes) throw new Error();
          if (await realpath(at(dest, ".")) !== join(destination, ...prefix)) throw new Error();
          let output: FileHandle;
          try { output = await open(at(dest, part), constants.O_WRONLY | constants.O_CREAT | constants.O_EXCL | constants.O_NOFOLLOW, checked.mode & 0o700); }
          catch (error) {
            if ((error as NodeJS.ErrnoException).code !== "EEXIST") throw error;
            const existing = await lstat(at(dest, part));
            if (!existing.isFile() || existing.isSymbolicLink()) { if (matched) { skipped++; return; } throw new Error(); }
            skipped++; return;
          }
          try { await output.chmod(checked.mode & 0o700); await output.writeFile(data.subarray(0, size)); copied++; }
          finally { await output.close(); }
        } finally { await input.close(); }
      };
      for (const current of patterns) { pattern = current; await copy(from, current.split("/"), [], to); }
      return { copied, skipped };
    } finally { await from.close(); await to?.close(); }
  } catch { throw new CopyRefusal(pattern); }
};

// Read name/path/URL tuples from the recorded starting commit and new checkout.
// Git parses its own config syntax; includes are explicitly disabled.
const initializeSubmodules = async (source: string, destination: string, baseBranch: string | undefined, startingHead: string | null | undefined, signal: AbortSignal) => {
  const limits = { bytes: 65_536, timeoutMs: 20_000 };
  const run = async (cwd: string, args: string[]) => gitProcess(cwd, ["--no-replace-objects", ...args], limits, signal);
  const modules = async (cwd: string, location: string[]) => {
    const result = await run(cwd, ["config", "--no-includes", ...location, "--null", "--get-regexp", "^submodule\\..*\\.(path|url)$"]);
    if (result.code !== 0 && result.code !== 1) throw new Error();
    const entries = new Map<string, { name: string; path?: string; url?: string }>();
    for (const row of result.stdout.split("\0").filter(Boolean)) {
      const split = row.indexOf("\n"), key = row.slice(0, split), value = row.slice(split + 1);
      const match = /^submodule\.(.+)\.(path|url)$/.exec(key);
      if (split < 0 || !match || !cleanText(match[1]!, 1024) || !cleanText(value)) throw new Error();
      const entry = entries.get(match[1]!) ?? { name: match[1]! };
      const field = match[2] as "path" | "url";
      if (entry[field] !== undefined) throw new Error();
      entry[field] = value; entries.set(entry.name, entry);
      if (entries.size > 128) throw new Error();
    }
    return [...entries.values()];
  };
  const file = join(destination, ".gitmodules");
  if (!await lstat(file).catch(error => { if (missing(error)) return undefined; throw error; })) return { skipped: false };
  await smallFile(file, limits.bytes);
  const candidates = await modules(destination, ["--file", ".gitmodules"]);
  if (candidates.length === 0) return { skipped: false };
  if (startingHead == null && (baseBranch === undefined || baseBranch.startsWith("-") || (await run(source, ["check-ref-format", "--branch", baseBranch])).code !== 0)) return { skipped: true };
  const base = startingHead == null ? await run(source, ["rev-parse", "--verify", "--end-of-options", `refs/heads/${baseBranch}^{commit}`]) : { code: 0, stdout: startingHead };
  if (base.code !== 0 || !/^[a-f0-9]{40,64}\n?$/.test(base.stdout)) return { skipped: true };
  const tree = await run(source, ["ls-tree", "-z", base.stdout.trim(), "--", ".gitmodules"]);
  if (tree.code !== 0) throw new Error();
  if (!/^100(?:644|755) blob [a-f0-9]{40,64}\t\.gitmodules\0$/.test(tree.stdout)) return { skipped: true };
  const trusted = await modules(source, ["--blob", `${base.stdout.trim()}:.gitmodules`]);
  const paths = candidates.flatMap(entry => entry.path === undefined ? [] : [entry.path]);
  if (paths.length === 0) return { skipped: true };
  const index = await run(destination, ["ls-files", "--stage", "-z", "--", ...paths]);
  if (index.code !== 0) throw new Error();
  const gitlinks = new Set(index.stdout.split("\0").flatMap(row => /^160000 [a-f0-9]{40,64} 0\t([\s\S]+)$/.exec(row)?.slice(1) ?? []));
  const approved = candidates.filter(entry => entry.path !== undefined && entry.url !== undefined
    && !/[=\r\n]/.test(entry.name)
    && (isAbsolute(entry.url) || /^[a-z][a-z0-9+.-]*:/i.test(entry.url) || /^[^/\\:\s]+:/.test(entry.url))
    && gitlinks.has(entry.path)
    && candidates.filter(other => other.path === entry.path).length === 1
    && !paths.some(path => path !== entry.path && (path.startsWith(`${entry.path}/`) || entry.path!.startsWith(`${path}/`)))
    && trusted.some(base => base.name === entry.name && base.path === entry.path && base.url === entry.url));
  for (const entry of approved) await filePath(destination, entry.path!, true);
  if (approved.length > 0) {
    const result = await run(destination,
      ["--no-literal-pathspecs", "-c", "protocol.allow=never", "-c", "protocol.https.allow=always", "-c", "protocol.ssh.allow=always",
        "-c", "protocol.file.allow=never", "-c", "protocol.ext.allow=never",
        "-c", "core.askPass=", "-c", "http.followRedirects=false", "-c", "submodule.recurse=false",
        "-c", "fetch.recurseSubmodules=false", "-c", "remote.origin.uploadpack=git-upload-pack",
        ...approved.flatMap(entry => ["-c", `submodule.${entry.name}.url=${entry.url}`]),
        // Literal magic protects names. The exact mode-160000 index check
        // and rejected overlaps prevent these selectors expanding to children.
        "submodule", "update", "--init", "--checkout", "--", ...approved.map(entry => `:(top,literal)${entry.path}`)]).catch(() => undefined);
    return { skipped: approved.length !== candidates.length, failed: result?.code !== 0 };
  }
  return { skipped: approved.length !== candidates.length };
};

export const prepareWorktree = (source: string, destination: string, settings?: WorktreePreparation, baseBranch?: string, startingHead?: string | null) => Effect.gen(function* () {
  if (settings === undefined) return "";
  const checked = Schema.decodeUnknownSync(WorktreePreparation)(settings);
  const copied = yield* Effect.tryPromise({ try: () => copyPreparationFiles(source, destination, checked.copyPaths ?? []),
    catch: (error) => refused(error instanceof CopyRefusal ? error.message : "Worktree preparation refused an unsafe path or exceeded a copy limit.") });
  let detail = `Worktree preparation copied ${copied.copied} files and kept ${copied.skipped} existing files.`;
  if (checked.submodules) {
    const result = yield* Effect.tryPromise({ try: (signal) => initializeSubmodules(source, destination, baseBranch, startingHead, AbortSignal.any([signal, AbortSignal.timeout(20_000)])), catch: () => "failed" as const }).pipe(Effect.result);
    detail += result._tag === "Success" && !result.success.failed ? " Submodules initialized." : " Submodule initialization failed; creation continued.";
    if (result._tag === "Success" && result.success.skipped) detail += " Unapproved submodules were skipped.";
    if (startingHead == null) detail += " Submodule trust used the base branch because no starting commit was recorded.";
  }
  return detail;
});
