import { dirname } from "node:path"
import { validArtifactPath } from "./artifacts.ts"
import { replaceFile, ReplaceFileError, type FileChange } from "./git/replace.ts"

export class MockEditorFileError extends Error {
  readonly uncertain: boolean
  constructor(uncertain: boolean) { super("mock editor file save failed"); this.uncertain = uncertain }
}

// The destination is an existing registered design file, never a caller-chosen
// path, and never a file at the checkout root. The replacement itself is the
// shared checkout file replacement in git/replace.ts.
export type MockEditorFileChange = FileChange
export function saveMockEditorFile(root: string, recordedPath: string, expected: string, bytes: Uint8Array, retain: true): Promise<MockEditorFileChange>
export function saveMockEditorFile(root: string, recordedPath: string, expected: string, bytes: Uint8Array): Promise<void>
export async function saveMockEditorFile(root: string, recordedPath: string, expected: string, bytes: Uint8Array, retain = false): Promise<MockEditorFileChange | void> {
  if (!validArtifactPath(recordedPath) || dirname(recordedPath) === ".") throw new MockEditorFileError(false)
  try {
    return retain ? await replaceFile(root, recordedPath, expected, bytes, 1024 * 1024, true) : await replaceFile(root, recordedPath, expected, bytes, 1024 * 1024)
  } catch (error) {
    throw new MockEditorFileError(error instanceof ReplaceFileError && error.uncertain)
  }
}
