import { constants } from "node:fs"
import { open, realpath, type FileHandle } from "node:fs/promises"
import type { Stats } from "node:fs"
import { filePath, inside } from "./git/paths.ts"

// Check the descriptor, not only its pathname: NOFOLLOW covers the leaf but
// cannot prevent a parent directory being replaced before open. Fail closed
// on hosts without descriptor paths. The engine's supported host is Linux.
export const withMapFile = async <A>(root: string, relative: string, maximum: number,
  use: (file: FileHandle, info: Stats, version: string) => Promise<A>): Promise<A> => {
  const canonical = await realpath(root)
  const path = await filePath(canonical, relative)
  const file = await open(path, constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK)
  try {
    const check = async () => {
      if (!inside(canonical, await realpath(`/proc/self/fd/${file.fd}`))) throw new Error("map descriptor escaped checkout")
    }
    await check()
    const info = await file.stat()
    if (!info.isFile() || info.size > maximum) throw new Error("invalid map file")
    const version = JSON.stringify([canonical, info.dev, info.ino, info.size, info.mtimeMs, info.ctimeMs])
    const result = await use(file, info, version)
    await check()
    const after = await file.stat()
    if (after.size !== info.size || after.mtimeMs !== info.mtimeMs || after.ctimeMs !== info.ctimeMs) throw new Error("map changed during read")
    return result
  } finally { await file.close() }
}

export const readMapFile = async (file: FileHandle, maximum: number) => {
  const buffer = Buffer.alloc(maximum + 1)
  let count = 0
  while (count <= maximum) {
    const next = await file.read(buffer, count, maximum + 1 - count, count)
    if (next.bytesRead === 0) break
    count += next.bytesRead
  }
  if (count > maximum) throw new Error("map exceeds limit")
  return buffer.subarray(0, count)
}
