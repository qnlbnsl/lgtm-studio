import { constants } from "node:fs";
import { mkdir, open, opendir, lstat, realpath, type FileHandle } from "node:fs/promises";
import { isAbsolute, join } from "node:path";
import { validCopyPattern, WorktreePreparation } from "@lgtm/contracts";
import * as Schema from "effect/Schema";
import * as Effect from "effect/Effect";
import { refused } from "../commands/internal.ts";
import { gitProcess } from "./git/process.ts";

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
  if (process.platform !== "linux" && patterns.length > 0) throw new Error();
  if (patterns.length > 16 || patterns.some(pattern => !validCopyPattern(pattern))) throw new Error();
  if (patterns.length === 0) return { copied: 0, skipped: 0 };
  const from = await directory(source);
  let to: FileHandle | undefined;
  let count = 0, bytes = 0, copied = 0, skipped = 0;
  const seen = new Set<string>();
  try {
    to = await directory(destination);
    const copy = async (src: FileHandle, parts: string[], prefix: string[], dest: FileHandle): Promise<void> => {
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
          if (entry.name.startsWith(before) && entry.name.endsWith(after)) {
            if (!validCopyPattern([...prefix, entry.name, ...parts.slice(1)].join("/"))) throw new Error();
            await copy(src, [entry.name, ...parts.slice(1)], prefix, dest);
          }
        }
        return;
      }
      let info;
      try { info = await lstat(at(src, part)); } catch (error) { if (missing(error)) return; throw error; }
      if (info.isSymbolicLink()) throw new Error();
      if (parts.length > 1) {
        if (!info.isDirectory()) return;
        const child = await childDirectory(src, part, false);
        let target: FileHandle | undefined;
        try { target = await childDirectory(dest, part, true); await copy(child, parts.slice(1), [...prefix, part], target); }
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
          if (!existing.isFile() || existing.isSymbolicLink()) throw new Error();
          skipped++; return;
        }
        try { await output.chmod(checked.mode & 0o700); await output.writeFile(data.subarray(0, size)); copied++; }
        finally { await output.close(); }
      } finally { await input.close(); }
    };
    for (const pattern of patterns) await copy(from, pattern.split("/"), [], to);
    return { copied, skipped };
  } finally { await from.close(); await to?.close(); }
};

export const prepareWorktree = (source: string, destination: string, settings?: WorktreePreparation) => Effect.gen(function* () {
  if (settings === undefined) return "";
  const checked = Schema.decodeUnknownSync(WorktreePreparation)(settings);
  const copied = yield* Effect.tryPromise({ try: () => copyPreparationFiles(source, destination, checked.copyPaths ?? []),
    catch: () => refused("Worktree preparation refused an unsafe path or exceeded a copy limit.") });
  let detail = `Worktree preparation copied ${copied.copied} files and kept ${copied.skipped} existing files.`;
  if (checked.submodules) {
    const result = yield* Effect.tryPromise({ try: (signal) => gitProcess(destination,
      ["-c", "protocol.allow=never", "-c", "protocol.https.allow=always", "-c", "protocol.ssh.allow=always",
        "-c", "core.askPass=", "-c", "http.followRedirects=false", "-c", "submodule.recurse=false",
        "-c", "fetch.recurseSubmodules=false", "-c", "remote.origin.uploadpack=git-upload-pack",
        "submodule", "update", "--init", "--checkout"], { bytes: 65_536, timeoutMs: 20_000 }, signal), catch: () => "failed" as const }).pipe(Effect.result);
    detail += result._tag === "Success" && result.success.code === 0 ? " Submodules initialized." : " Submodule initialization failed; creation continued.";
  }
  return detail;
});
