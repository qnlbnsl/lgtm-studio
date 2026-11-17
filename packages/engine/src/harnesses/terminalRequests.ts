import { readFile } from 'node:fs/promises'
import { isAbsolute } from 'node:path'
import * as Effect from 'effect/Effect'
import { refused, type Input } from '../commands/internal.ts'
import { projectView, own } from '../project/view.ts'
import { sessions, terminalOutput } from './signinRequests.ts'
import { terminalReasons as reasons } from './terminalReasons.ts'
const failure = (error: unknown) => refused(error instanceof Error && Object.values(reasons).includes(error.message as never) ? error.message : reasons.failed)
const owner = (input: Input) => { if (input.actor._tag !== 'owner') throw refused(reasons.owner); if (!input.connectionId) throw refused(reasons.connection); return input.connectionId }
// The executable and environment are resolved by the service. No request data
// is used as a path, command, shell argument, or environment entry.
const open = (input: Input, p: Record<string, unknown>) => Effect.gen(function* () {
  const connection = yield* Effect.try({ try: () => owner(input), catch: e => e as ReturnType<typeof refused> })
  if ((input.snapshot.client as { webSettings?: { terminalEnabled?: boolean } } | undefined)?.webSettings?.terminalEnabled !== true) return yield* refused(reasons.off)
  const manager = sessions(input), generation = manager.terminalGeneration
  const candidate = own(projectView(input.snapshot).workspaces, p.worktreeId as string)
  if (candidate && candidate.machineId !== 'local') return yield* refused(reasons.remote)
  const { workspaceReference } = yield* Effect.promise(() => import('../project/gitApi.ts'))
  const workspace = yield* workspaceReference(input, p, { ownerMain: true }).pipe(Effect.mapError(() => refused(reasons.workspace)))
  // Give a direct reason even when a remote record is not attached to this project.
  if (workspace.machineId !== 'local') return yield* refused(reasons.remote)
  const shell = yield* Effect.promise(async () => {
    const candidate = process.env.SHELL
    if (candidate && isAbsolute(candidate)) {
      try { if ((await readFile('/etc/shells', 'utf8')).split('\n').some(line => line.trim() === candidate)) return candidate } catch {}
    }
    return '/bin/sh'
  })
  const environment: Record<string, string> = {}
  for (const [name, value] of Object.entries(process.env)) {
    if (name.startsWith('LGTM_') || value === undefined) continue
    // The log redactor registry removes every known secret value,
    // including a value embedded in an otherwise ordinary environment entry.
    if ((yield* input.runtime.log.redactText(value)) === value) environment[name] = value
  }
  const disconnected = () => manager.disconnect(connection)
  const session = yield* Effect.tryPromise({ try: () => manager.start({ ownerSessionId: input.ownerSessionId, kind: 'project', projectId: p.projectId as string, worktreeId: workspace.worktreeId, accountId: '', connectionId: connection, executable: shell, args: ['-l'], cwd: workspace.path, environment,
    cols: p.cols as number, rows: p.rows as number, allowed: () => manager.terminalGeneration === generation && !input.connectionSignal?.aborted,
    check: async () => ({ state: 'unavailable' }), onEnd: () => input.connectionSignal?.removeEventListener('abort', disconnected) }), catch: failure })
  input.connectionSignal?.addEventListener('abort', disconnected, { once: true })
  if (input.connectionSignal?.aborted) manager.disconnect(connection)
  return { reply: { requestId: p.requestId, sessionId: session.id } }
})
const control = (method: 'write' | 'resize' | 'close') => (input: Input, p: Record<string, unknown>) => Effect.gen(function* () {
  const connection = yield* Effect.try({ try: () => owner(input), catch: e => e as ReturnType<typeof refused> })
  yield* Effect.try({ try: () => {
    const manager = sessions(input), id = p.sessionId as string
    if (method === 'write') manager.write(id, connection, p.text as string, 'project', input.ownerSessionId)
    else if (method === 'resize') manager.resize(id, connection, p.cols as number, p.rows as number, 'project', input.ownerSessionId)
    else manager.stop(id, connection, 'project', input.ownerSessionId)
  }, catch: failure })
  return { reply: { requestId: p.requestId } }
})
export const terminalReads = {
  'client.terminal.list': (input: Input, p: Record<string, unknown>) => Effect.gen(function* () {
    const connection = yield* Effect.try({ try: () => owner(input), catch: e => e as ReturnType<typeof refused> })
    return { reply: { requestId: p.requestId, sessions: sessions(input).list(input.ownerSessionId, connection) } }
  }),
  'client.terminal.open': open,
  'client.terminal.write': control('write'),
  'client.terminal.resize': control('resize'),
  'client.terminal.close': control('close'),
  'client.terminal.subscribe': (input: Input, p: Record<string, unknown>) => terminalOutput(input, p, 'project')
}
