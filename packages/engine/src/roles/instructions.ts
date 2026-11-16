import { parseRole, type RoleFile } from '@lgtm/contracts'
import * as Data from 'effect/Data'
import { constants as C, promises as fs } from 'node:fs'
import type { FileHandle } from 'node:fs/promises'

export const instructionLimits = { fileBytes: 24 * 1024, composedBytes: 48 * 1024, replyBytes: 256 * 1024 } as const
const mapping = {
  'project-lead': ['lead-base'], orchestrator: ['lead-base'], 'planning-lead': ['lead-base'],
  'design-lead': ['lead-base'], 'build-lead': ['lead-base'],
  planner: ['worker-base'], designer: ['worker-base'], 'build-init': ['worker-base'], builder: ['worker-base'],
  reviewer: ['reviewer-base'], 'test-runner': [], evaluator: [],
} satisfies Record<string, string[]>
type InstructionName = keyof typeof mapping

// This selects prose only. parseRole remains the sole authority parser.
export const instructionName = (role: string): InstructionName | undefined => {
  const name = role.includes(':') ? role.slice(role.indexOf(':') + 1) : role
  return Object.hasOwn(mapping, name) ? name as InstructionName : undefined
}
export const instructionFiles = (role: string): string[] => {
  const name = instructionName(role)
  return name === undefined ? [parseRole(role).kind === 'worker' ? 'worker-base' : 'lead-base'] : [...mapping[name], name]
}
export class InstructionError extends Data.TaggedError('InstructionError')<{ readonly reason: string }> {}

export class InstructionStore {
  readonly home: string
  readonly warn: (line: string) => void
  constructor(home: string, warn: (line: string) => void = () => {}) { this.home = home; this.warn = warn }
  private async read(name: string): Promise<string> {
    const handles: FileHandle[] = []
    try {
      // As in WorkflowStore, pin every directory to prevent pathname swaps.
      handles.push(await fs.open(this.home, C.O_RDONLY | C.O_DIRECTORY | C.O_NOFOLLOW))
      for (const part of ['.lgtm', 'roles']) handles.push(await fs.open(`/proc/self/fd/${handles.at(-1)!.fd}/${part}`, C.O_RDONLY | C.O_DIRECTORY | C.O_NOFOLLOW))
      const handle = await fs.open(`/proc/self/fd/${handles.at(-1)!.fd}/${name}.md`, C.O_RDONLY | C.O_NOFOLLOW | C.O_NONBLOCK)
      handles.push(handle)
      const info = await handle.stat()
      if (!info.isFile() || info.nlink !== 1 || info.size > instructionLimits.fileBytes) throw new Error('unsafe')
      const bytes = Buffer.alloc(instructionLimits.fileBytes + 1)
      let length = 0
      while (length < bytes.length) {
        const { bytesRead } = await handle.read(bytes, length, bytes.length - length, null)
        if (bytesRead === 0) break
        length += bytesRead
      }
      if (length > instructionLimits.fileBytes) throw new Error('oversized')
      return bytes.toString('utf8', 0, length)
    } catch {
      // name is drawn only from the fixed mapping, never from a supplied path.
      this.warn(`Role instruction override ${name} unavailable; using the built-in file.`)
    } finally { for (const handle of handles.reverse()) await handle.close() }
    return fs.readFile(new URL(`./instructions/${name}.md`, import.meta.url), 'utf8')
  }
  async compose(role: string, file: RoleFile | null = null): Promise<string> {
    const names = instructionFiles(role), selected = instructionName(role), parsed = parseRole(role)
    const adviceName = selected ?? (parsed.flags.length === 0 ? parsed.name : undefined)
    const advice = adviceName !== undefined && file !== null && Object.hasOwn(file.roles, adviceName) ? file.roles[adviceName]!.advice : undefined
    const parts: string[] = []
    for (const name of names) {
      if (name === selected && advice !== undefined) continue
      parts.push(await this.read(name))
    }
    if (advice !== undefined) parts.push(advice)
    const text = parts.join('\n\n')
    if (Buffer.byteLength(text) > instructionLimits.composedBytes) throw new InstructionError({ reason: 'Role instructions exceed 48 KiB.' })
    return text
  }
}
