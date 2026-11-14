import * as Pty from '@lydell/node-pty'
import { vi } from 'vitest'
import { engineLists } from '../../src/pieces/lists.ts'
import { expect, it } from '@effect/vitest'
import * as Effect from 'effect/Effect'
import * as Stream from 'effect/Stream'
import * as Redacted from 'effect/Redacted'
import * as Logger from 'effect/Logger'
import { chmodSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { randomBytes } from 'node:crypto'
import * as Server from '../../src/api/server.ts'
import { serve } from '../../src/client/web/server.ts'
import { CommandHandler } from '../../src/commands/CommandHandler.ts'
import { makeRedactor } from '../../src/identity/redactor.ts'
import { makeFakeRunner } from '../support/fakeRunner.ts'
import { appendSetup, eventOf, makeFakeSecrets, makeRig, provideRig, removeRig, seedAgent } from '../support/handlerRig.ts'
import { connect, upgrade, ancestryHeaders, authorization, requestFrame } from '../support/socketClient.ts'
import { alive, waitFor } from '../support/serviceProcess.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
import { unusedPort } from '../support/browserWeb.ts'
vi.mock('@lydell/node-pty', async original => { const actual = await original<typeof import('@lydell/node-pty')>(); return { ...actual, spawn: vi.fn(actual.spawn) } })
const known = 'registered-terminal-secret', marker = 'PRIVATE_TERMINAL_OUTPUT', keys = 'PRIVATE_TERMINAL_INPUT'
const live = (name: string, body: () => Effect.Effect<any, any, any>) => it.live(name, () => body() as Effect.Effect<void, unknown, never>, 45000)
const req = (client: any, operation: string, payload = {}) => client['client.terminal.' + operation]({ requestId: randomBytes(8).toString('hex'), ...payload }) as Effect.Effect<any, any>
const output = (client: any, sessionId: string) => client['client.terminal.subscribe']({ requestId: randomBytes(8).toString('hex'), sessionId }) as Stream.Stream<any, any>
const open = (client: any, extra = {}) => req(client, 'open', { projectId: 'terminal-project', worktreeId: 'terminal-worktree', cols: 80, rows: 24, ...extra })
const failure = (effect: Effect.Effect<any, any>, reason: string) => effect.pipe(Effect.result, Effect.map(r => expect(r).toMatchObject({ _tag: 'Failure', failure: { reason } })))
const enabled = (client: any, value: boolean) => client['client.web.settings.set']({ requestId: randomBytes(8).toString('hex'), settings: { enabled: true, port: 8787, proxyMode: false, heartbeatSeconds: 25, terminalEnabled: value } }) as Effect.Effect<any, any>
const withServer = (body: (ctx: { client: any; socket: string; bearer: string; rig: ReturnType<typeof makeRig> }) => Effect.Effect<any, any, any>) => Effect.gen(function* () {
  const rig = makeRig(), bearer = randomBytes(32).toString('base64url'), socket = join(rig.stateFolder, 'terminal.sock')
  chmodSync(rig.stateFolder, 0o700)
  const previous = { SHELL: process.env.SHELL, LGTM_TEST_TERMINAL: process.env.LGTM_TEST_TERMINAL, NORMAL_PRIVATE: process.env.NORMAL_PRIVATE }
  process.env.SHELL = '/bin/sh'; process.env.LGTM_TEST_TERMINAL = 'must-not-inherit'; process.env.NORMAL_PRIVATE = known
  const redactor = makeRedactor({ secrets: () => [Redacted.make(known), Redacted.make(bearer)], tokenHashes: () => new Set() })
  try {
    return yield* provideRig(rig, { runner: yield* makeFakeRunner(), secrets: makeFakeSecrets(), redactor: redactor.layer }, Effect.gen(function* () {
      yield* registerRebuildCheck()
      yield* appendSetup([
        eventOf({ _tag: 'project.WorktreeCreated', repoId: 'terminal-repo', worktreeId: 'terminal-worktree', branch: 'main', path: rig.work, machineId: 'local', checkout: 'main' }),
        eventOf({ _tag: 'project.Created', projectId: 'terminal-project', name: 'Terminal', goal: 'Test', repoId: 'terminal-repo', worktreeId: 'terminal-worktree', machineId: 'local', leadAgentId: 'lead' })
      ])
      yield* Server.serve({ socketPath: socket, ownerCredential: Redacted.make(bearer), debug: false, lists: engineLists })
      return yield* body({ client: yield* connect(socket, bearer), socket, bearer, rig })
    }))
  } finally { for (const [name, value] of Object.entries(previous)) { if (value === undefined) delete process.env[name]; else process.env[name] = value }; removeRig(rig) }
})
live('terminal is off initially, refuses extra open fields and refuses remote worktrees', () => withServer(({ client, socket, bearer }) => Effect.gen(function* () {
  const processes = vi.mocked(Pty.spawn).mock.calls.length
  yield* failure(open(client), 'The project terminal is turned off.')
  expect(vi.mocked(Pty.spawn).mock.calls.length).toBe(processes)
  yield* enabled(client, true)
  for (const field of ['command', 'path', 'executable', 'args', 'environment']) {
    const upgraded = yield* Effect.promise(() => upgrade(socket, { ...authorization(bearer), ...ancestryHeaders() }))
    if (upgraded.status !== 101) throw new Error('owner socket refused')
    const ws = upgraded.socket!
    const reply = yield* Effect.promise(() => new Promise<string>(resolve => { ws.once('message', frame => resolve(frame.toString())); ws.send(requestFrame('1', 'client.terminal.open', { requestId: 'extra', projectId: 'terminal-project', worktreeId: 'terminal-worktree', cols: 80, rows: 24, [field]: 'untrusted' })) }))
    ws.close()
    expect(reply, field).toContain('Failure')
    expect(reply, field).not.toContain('sessionId')
  }
  yield* appendSetup([eventOf({ _tag: 'project.WorktreeCreated', repoId: 'terminal-repo', worktreeId: 'remote', branch: 'remote', path: '/unavailable', machineId: 'remote', checkout: 'worktree' })])
  yield* failure(open(client, { worktreeId: 'remote' }), 'Project terminals are available only on the local machine.')
})))
live('terminal access is owner-only and isolated to the opening socket', () => withServer(({ client, socket, bearer }) => Effect.gen(function* () {
  yield* enabled(client, true)
  const handler = yield* CommandHandler
  for (const operation of ['open', 'write', 'resize', 'close', 'subscribe']) {
    expect((yield* handler.execute({ _tag: 'service', service: 'roles.stallWatch' }, 'client.terminal.' + operation, { requestId: 'service-' + operation }).pipe(Effect.result))._tag).toBe('Failure')
  }
  const { sessionId } = yield* open(client), second = yield* connect(socket, bearer)
  const agent = yield* seedAgent({ id: 'worker' }), asAgent = yield* connect(socket, agent.token)
  yield* failure(open(asAgent), 'Only the owner can run a project terminal.')
  // Opening a fresh shell is allowed for a second owner; claiming an existing ID is not.
  // A new open cannot name a session in the schema; controls below use its identifier.
  for (const other of [asAgent, second]) {
    const reason = other === asAgent ? 'Only the owner can run a project terminal.' : 'This terminal belongs to another connection.'
    yield* failure(req(other, 'write', { sessionId, text: 'exit\r' }), reason)
    yield* failure(req(other, 'resize', { sessionId, cols: 100, rows: 30 }), reason)
    yield* failure(req(other, 'close', { sessionId }), reason)
    yield* failure(output(other, sessionId).pipe(Stream.runDrain), reason)
  }
  yield* req(client, 'close', { sessionId })
})))
live('real sh redacts split secrets, strips private environment and leaves no durable or protocol logs', () => withServer(({ client }) => Effect.gen(function* () {
  const diagnostics: string[] = []
  yield* Effect.gen(function* () {
    yield* enabled(client, true)
    const { sessionId } = yield* open(client)
    let text = ''
    yield* output(client, sessionId).pipe(Stream.runForEach(item => Effect.sync(() => { if (item._tag === 'output') text += item.text })), Effect.forkScoped)
    // Disable terminal echo so the assertion checks the child output as well as input privacy.
    yield* req(client, 'write', { sessionId, text: 'stty -echo\r' }); yield* Effect.sleep(100)
    yield* req(client, 'write', { sessionId, text: `printf '${marker} '; printf 'registered-terminal-'; sleep 0.1; printf 'secret\\n'; printf '${keys}\\n'; env; printf 'ENV_DONE\\n'\r` })
    yield* waitFor('shell output', () => text.includes('ENV_DONE'))
    expect(text).toContain(marker); expect(text).toContain('[REDACTED]'); expect(text).not.toContain(known)
    expect(text).not.toMatch(/\bLGTM_[A-Za-z0-9_]*=/); expect(text).not.toContain('LGTM_TEST_TERMINAL='); expect(text).not.toContain('NORMAL_PRIVATE=')
    yield* req(client, 'resize', { sessionId, cols: 100, rows: 30 })
    yield* req(client, 'close', { sessionId })
    const input = yield* (yield* CommandHandler).input({ _tag: 'owner' })
    const tables = yield* input.runtime.sql<{ name: string }>`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`
    for (const { name } of tables) {
      const data = JSON.stringify(yield* input.runtime.sql.unsafe('SELECT * FROM "' + name.replaceAll('"', '""') + '"'))
      expect(data).not.toContain(marker); expect(data).not.toContain(keys)
    }
    const snapshot = JSON.stringify(yield* input.runtime.views.snapshot)
    expect(snapshot).not.toContain(marker); expect(snapshot).not.toContain(keys)
  }).pipe(Effect.provide(Logger.layer([Logger.make(options => { diagnostics.push(Logger.formatJson.log(options)) })])))
  expect(diagnostics.join('')).not.toContain(marker); expect(diagnostics.join('')).not.toContain(keys)
})))
live('terminal refuses a ninth live shell', () => withServer(({ client }) => Effect.gen(function* () {
  yield* enabled(client, true)
  for (let i = 0; i < 8; i++) expect((yield* open(client)).sessionId).toBeTypeOf('string')
  yield* failure(open(client), 'Eight project terminal sessions are already open.')
})))
for (const mode of ['close', 'disable', 'disconnect', 'logout', 'shutdown'] as const) live(`terminal ${mode} kills the real shell process group`, () => Effect.gen(function* () {
  let pids: number[] = []
  yield* withServer(({ socket, bearer, client, rig }) => Effect.gen(function* () {
    yield* enabled(client, true)
    yield* Effect.scoped(Effect.gen(function* () {
      const one = yield* connect(socket, bearer), { sessionId } = yield* open(one)
      yield* output(one, sessionId).pipe(Stream.runDrain, Effect.forkScoped)
      yield* req(one, 'write', { sessionId, text: `sleep 120 & printf '%s %s' "$$" "$!" > '${join(rig.work, 'pids')}'\r` })
      yield* waitFor('shell and descendant', () => { try { pids = readFileSync(join(rig.work, 'pids'), 'utf8').split(' ').map(Number); return pids.length === 2 && pids.every(alive) } catch { return false } })
      if (mode === 'close') yield* req(one, 'close', { sessionId })
      if (mode === 'disable') yield* enabled(client, false)
      if (mode === 'logout') {
        const port = yield* Effect.promise(unusedPort), password = 'terminal-test-password'
        yield* serve({ password: Redacted.make(password), staticRoot: rig.work, settings: { enabled: true, port, proxyMode: false, heartbeatSeconds: 25 } })
        const response = yield* Effect.promise(() => fetch(`http://127.0.0.1:${port}/api/login`, { method: 'POST', headers: { Origin: `http://127.0.0.1:${port}` }, body: JSON.stringify({ password }) }))
        const cookie = response.headers.get('set-cookie')!.split(';')[0]!
        const logout = yield* Effect.promise(() => fetch(`http://127.0.0.1:${port}/api/logout`, { method: 'POST', headers: { Cookie: cookie, Origin: `http://127.0.0.1:${port}` } }))
        expect(logout.status).toBe(204)
      }
      if (mode === 'close' || mode === 'disable' || mode === 'logout') yield* waitFor(mode, () => pids.every(pid => !alive(pid)))
    }))
    if (mode === 'disconnect') yield* waitFor('connection grace', () => pids.every(pid => !alive(pid)), 10000)
  }))
  yield* waitFor('shutdown', () => pids.every(pid => !alive(pid)))
}))

live('a project terminal can open its repository main checkout but not a different repository', () => withServer(({ client, rig }) => Effect.gen(function* () {
  yield* enabled(client, true)
  yield* appendSetup([
    eventOf({ _tag: 'project.WorktreeCreated', repoId: 'terminal-repo', worktreeId: 'base-worktree', branch: 'main', path: rig.work, machineId: 'local', checkout: 'main' }),
    eventOf({ _tag: 'project.WorktreeCreated', repoId: 'other-repo', worktreeId: 'other-worktree', branch: 'main', path: rig.work, machineId: 'local', checkout: 'main' })
  ])
  const { sessionId } = yield* open(client, { worktreeId: 'base-worktree' })
  yield* req(client, 'close', { sessionId })
  yield* failure(open(client, { worktreeId: 'other-worktree' }), 'The project worktree is unavailable.')
})))

live('a refused output subscription cannot reset the owner stream redaction carry', () => withServer(({ client, socket, bearer }) => Effect.gen(function* () {
  yield* enabled(client, true)
  const { sessionId } = yield* open(client)
  let text = ''
  yield* output(client, sessionId).pipe(Stream.runForEach(item => Effect.sync(() => { if (item._tag === 'output') text += item.text })), Effect.forkScoped)
  yield* req(client, 'write', { sessionId, text: 'stty -echo\r' }); yield* Effect.sleep(100)
  yield* req(client, 'write', { sessionId, text: "printf 'CARRY_READY registered-terminal-'; sleep 0.6; printf 'secret\\nCARRY_DONE\\n'\r" })
  yield* waitFor('first secret chunk', () => text.includes('CARRY_READY '))
  const second = yield* connect(socket, bearer)
  yield* failure(output(second, sessionId).pipe(Stream.runDrain), 'This terminal belongs to another connection.')
  yield* failure(output(client, sessionId).pipe(Stream.runDrain), 'This terminal already has an output stream.')
  yield* waitFor('second secret chunk', () => text.includes('CARRY_DONE'))
  expect(text).toContain('[REDACTED]'); expect(text).not.toContain(known)
  expect(text).not.toContain('secret')
  yield* req(client, 'close', { sessionId })
})))
