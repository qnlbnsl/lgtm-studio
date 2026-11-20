import * as Effect from 'effect/Effect'
import type { RoleFile } from '@lgtm/contracts'
import { refused, type Runtime, type Snapshot } from '../commands/internal.ts'
import { escapeLeadingSlash } from '../routing/envelope.ts'
import { InstructionStore, instructionLimits } from './instructions.ts'
import { projectView } from '../project/view.ts'
import { inside } from '../project/git/paths.ts'
import type { RolesView } from './view.ts'

// Redact before escaping so a redaction cannot expose a new leading slash.
// Escape every line, including invisible leading whitespace, in engine prose.
export const composeInstructions = (runtime: Runtime, snapshot: Snapshot, role: string, file: RoleFile | null = (snapshot.roles as RolesView).file, folder?: string) => Effect.gen(function* () {
  const warnings: string[] = []
  const text = yield* Effect.tryPromise({
    try: () => new InstructionStore(runtime.home, (line) => warnings.push(line)).compose(role, file),
    catch: (error) => refused(error instanceof Error && 'reason' in error && error.reason === 'Role instructions exceed 48 KiB.'
      ? error.reason : 'Role instructions could not be read.'),
  }).pipe(Effect.ensuring(Effect.gen(function* () {
    for (const warning of warnings) yield* Effect.logWarning(yield* runtime.log.redactText(warning))
  })))
  const view = projectView(snapshot)
  const candidate = folder === undefined ? undefined : [
    ...Object.values(view.repositories).map(row => ({ path: row.path, repoId: row.repoId })),
    ...Object.values(view.workspaces).map(row => ({ path: row.path, repoId: row.repoId }))
  ].filter(row => inside(row.path, folder)).sort((a, b) => b.path.length - a.path.length)[0]
  const repository = candidate === undefined ? undefined : view.repositories[candidate.repoId]
  const preparation = repository?.preparation?.instructions
  const combined = preparation ? `${text}\n\nOwner's repository settings\n${preparation}` : text
  const clean = yield* runtime.log.redactText(combined)
  const escaped = clean.split(/\r\n|\n|\r|\u2028|\u2029/u).map(escapeLeadingSlash).join('\n')
  if (Buffer.byteLength(escaped) > instructionLimits.composedBytes) return yield* refused('Role instructions exceed 48 KiB.')
  return escaped
})
