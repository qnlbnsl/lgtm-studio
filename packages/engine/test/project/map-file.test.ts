import { mkdtemp, mkdir, writeFile, rename, symlink, rm, open } from "node:fs/promises"
import { join } from "node:path"
import { tmpdir } from "node:os"
import { expect, it, vi } from "vitest"
import { withMapFile, readMapFile } from "../../src/project/mapFile.ts"
import { registerRebuildCheck } from "../support/rebuildCheck.ts"

registerRebuildCheck({ noLog: true })
vi.mock("node:fs/promises", async (original) => ({ ...await original<typeof import("node:fs/promises")>(), open: vi.fn() }))

it("refuses an outside descriptor when a parent is swapped after path checks, before reading any bytes", async () => {
  const root = await mkdtemp(join(tmpdir(), "lgtm-map-race-"))
  const actual = await vi.importActual<typeof import("node:fs/promises")>("node:fs/promises")
  try {
    const checkout = join(root, "checkout"), outside = join(root, "outside")
    await mkdir(join(checkout, "floorplan"), { recursive: true }); await mkdir(outside)
    await writeFile(join(checkout, "floorplan/index.html"), "safe map")
    await writeFile(join(outside, "index.html"), "outside private bytes")
    let read: ReturnType<typeof vi.spyOn> | undefined, close: ReturnType<typeof vi.spyOn> | undefined
    vi.mocked(open).mockImplementationOnce(async (...args) => {
      await rename(join(checkout, "floorplan"), join(checkout, "original"))
      await symlink(outside, join(checkout, "floorplan"))
      const descriptor = await actual.open(...args)
      read = vi.spyOn(descriptor, "read"); close = vi.spyOn(descriptor, "close")
      return descriptor
    })
    await expect(withMapFile(checkout, "floorplan/index.html", 1024, file => readMapFile(file, 1024)))
      .rejects.toThrow("map descriptor escaped checkout")
    expect(read).not.toHaveBeenCalled()
    expect(close).toHaveBeenCalledOnce()
  } finally { vi.mocked(open).mockReset(); await rm(root, { recursive: true, force: true }) }
})
