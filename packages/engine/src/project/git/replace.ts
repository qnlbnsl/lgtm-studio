import { constants } from "node:fs";
import { lstat, open, realpath, rename, unlink, type FileHandle } from "node:fs/promises";
import { createHash, randomUUID } from "node:crypto";
import { join } from "node:path";
import { isRepositoryArtifactPath } from "@lgtm/contracts";
import { filePath, smallFile } from "./paths.ts";

export const sha256 = (bytes: Uint8Array) => createHash("sha256").update(bytes).digest("hex");

export class ReplaceFileError extends Error {
  readonly uncertain: boolean;
  constructor(uncertain: boolean) { super("file replacement failed"); this.uncertain = uncertain; }
}
export interface FileChange { readonly restore: () => Promise<void>; readonly close: () => Promise<void> }

// Replaces an existing regular file inside a checkout, keeping its mode.
// Linux directory descriptors pin every directory, so a swapped path component
// cannot redirect the temporary file or the rename through a symbolic link.
// The file must still hold the expected bytes immediately before the rename.
// On a failure after the rename, the original bytes are put back; when that
// also fails, the error is uncertain.
export function replaceFile(root: string, recordedPath: string, expected: string, bytes: Uint8Array, maximum: number, retain: true): Promise<FileChange>;
export function replaceFile(root: string, recordedPath: string, expected: string, bytes: Uint8Array, maximum: number): Promise<void>;
export async function replaceFile(root: string, recordedPath: string, expected: string, bytes: Uint8Array, maximum: number, retain = false): Promise<FileChange | void> {
  const handles: FileHandle[] = [];
  let temporary: string | undefined, changed = false, retained = false, restore: (() => Promise<void>) | undefined;
  const close = async () => {
    if (temporary !== undefined) await unlink(temporary).catch(() => {});
    temporary = undefined;
    for (const directory of handles.splice(0).reverse()) await directory.close().catch(() => {});
  };
  try {
    if (process.platform !== "linux" || !isRepositoryArtifactPath(recordedPath)) throw new Error();
    if (await realpath(root) !== root) throw new Error();
    await filePath(root, recordedPath);
    const parts = recordedPath.split("/");
    let directory = await open(root, constants.O_RDONLY | constants.O_DIRECTORY | constants.O_NOFOLLOW);
    handles.push(directory);
    for (const part of parts.slice(0, -1)) {
      directory = await open(`/proc/self/fd/${directory.fd}/${part}`, constants.O_RDONLY | constants.O_DIRECTORY | constants.O_NOFOLLOW);
      handles.push(directory);
    }
    const parent = `/proc/self/fd/${directory.fd}`, destination = `${parent}/${parts.at(-1)!}`;
    const originalParent = parts.length === 1 ? root : join(root, ...parts.slice(0, -1));
    const identity = await directory.stat();
    const stillRecorded = async (wanted: string) => {
      await filePath(root, recordedPath);
      const current = await lstat(originalParent);
      if (current.dev !== identity.dev || current.ino !== identity.ino || await realpath(parent) !== originalParent) throw new Error();
      if (sha256(await smallFile(destination, maximum)) !== wanted) throw new Error();
    };
    await stillRecorded(expected);
    const original = await smallFile(destination, maximum);
    if (sha256(original) !== expected) throw new Error();
    const mode = (await lstat(destination)).mode & 0o7777;
    const replace = async (content: Uint8Array, wanted: string) => {
      await stillRecorded(wanted);
      temporary = `${parent}/.lgtm-editor-${randomUUID()}`;
      const output = await open(temporary, constants.O_WRONLY | constants.O_CREAT | constants.O_EXCL | constants.O_NOFOLLOW, 0o600);
      try { await output.writeFile(content); await output.chmod(mode); await output.sync(); } finally { await output.close(); }
      await stillRecorded(wanted);
      await rename(temporary, destination);
      changed = true;
      temporary = undefined;
      await directory.sync();
    };
    restore = () => replace(original, sha256(bytes));
    await replace(bytes, expected);
    if (retain) { retained = true; return { restore, close }; }
  } catch {
    if (changed) {
      try { await restore!(); } catch { throw new ReplaceFileError(true); }
    }
    throw new ReplaceFileError(false);
  } finally { if (!retained) await close(); }
}
