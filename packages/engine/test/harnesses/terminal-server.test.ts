import * as Pty from '@lydell/node-pty'
import { vi } from 'vitest'
import { engineLists } from '../../src/pieces/lists.ts'
import { expect, it } from '@effect/vitest'
import * as Effect from 'effect/Effect'
import * as Stream from 'effect/Stream'
import * as Redacted from 'effect/Redacted'
import { withPtyPrivacy } from '../support/ptyPrivacy.ts'
import { chmodSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { randomBytes } from 'node:crypto'
import * as Server from '../../src/api/server.ts'
import { serve } from '../../src/client/web/server.ts'
import { CommandHandler } from '../../src/commands/CommandHandler.ts'
import { mintCredential } from '../../src/identity/credential.ts'
import { makeRedactor } from '../../src/identity/redactor.ts'
import { makeFakeRunner } from '../support/fakeRunner.ts'
import { appendSetup, eventOf, makeFakeSecrets, makeRig, provideRig, removeRig, seedAgent } from '../support/handlerRig.ts'
import { connect, upgrade, ancestryHeaders, authorization, requestFrame } from '../support/socketClient.ts'
import { alive, waitFor } from '../support/serviceProcess.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
import { unusedPort } from '../support/browserWeb.ts'
vi.mock('@lydell/node-pty', async original => { const actual = await original<typeof import('@lydell/node-pty')>(); return { ...actual, spawn: vi.fn(actual.spawn) } })
const token = mintCredential('shp_a_')
const known = 'sk-registered-terminal-secret', marker = 'PRIVATE_TERMINAL_OUTPUT', keys = 'PRIVATE_TERMINAL_INPUT'
const live = (name: string, body: () => Effect.Effect<any, any, any>) => it.live(name, () => body() as Effect.Effect<void, unknown, never>, 45000)
const req = (client: any, operation: string, payload = {}) => client['client.terminal.' + operation]({ requestId: randomBytes(8).toString('hex'), ...payload }) as Effect.Effect<any, any>
const output = (client: any, sessionId: string) => client['client.terminal.subscribe']({ requestId: randomBytes(8).toString('hex'), sessionId }) as Stream.Stream<any, any>
const open = (client: any, extra = {}) => req(client, 'open', { projectId: 'terminal-project', worktreeId: 'terminal-worktree', cols: 80, rows: 24, ...extra })
const failure = (effect: Effect.Effect<any, any>, reason: string) => effect.pipe(Effect.result, Effect.map(r => expect(r).toMatchObject({ _tag: 'Failure', failure: { reason } })))
const enabled = (client: any, value: boolean) => client['client.web.settings.set']({ requestId: randomBytes(8).toString('hex'), settings: { enabled: true, port: 8787, proxyMode: false, heartbeatSeconds: 25, terminalEnabled: value } }) as Effect.Effect<any, any>
const withServer = (body: (ctx: { client: any; socket: string; bearer: string; rig: ReturnType<typeof makeRig> }) => Effect.Effect<any, any, any>) => Effect.gen(function* () {
  // Fixed synthetic credentials make output expectations independent of randomness.
  // In particular the registered sk- secret must not delay the final s in ls.
  const rig = makeRig(), bearer = 'fixed-owner-terminal-bearer-0123456789', socket = join(rig.stateFolder, 'terminal.sock')
  chmodSync(rig.stateFolder, 0o700)
  const previous = { SHELL: process.env.SHELL, LGTM_TEST_TERMINAL: process.env.LGTM_TEST_TERMINAL, NORMAL_PRIVATE: process.env.NORMAL_PRIVATE }
  process.env.SHELL = '/bin/sh'; process.env.LGTM_TEST_TERMINAL = 'must-not-inherit'; process.env.NORMAL_PRIVATE = known
  const redactor = makeRedactor({ secrets: () => [Redacted.make(known), Redacted.make(bearer)], tokenHashes: () => new Set([token.tokenHash]) })
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
  const { sessionId } = yield* open(client), second = yield* connect(socket, bearer)
  const agent = yield* seedAgent({ id: 'worker' }), asAgent = yield* connect(socket, agent.token)
  yield* failure(open(asAgent), 'Only the owner can run a project terminal.')
  const handler = yield* CommandHandler
  for (const credential of [{ _tag: 'service', service: 'roles.stallWatch' } as const, { _tag: 'agent', agentId: 'worker' } as const]) {
    const input = { ...(yield* handler.input(credential as never)), connectionId: 'non-owner' }
    for (const operation of ['list', 'open', 'write', 'resize', 'close', 'subscribe']) {
      const reader = engineLists.reads['client.terminal.' + operation]!
      const result = reader(input, { requestId: 'authority-' + operation, projectId: 'terminal-project', worktreeId: 'terminal-worktree', sessionId, text: 'exit\r', cols: 80, rows: 24 })
      yield* failure((operation === 'subscribe' ? Stream.runDrain(result as Stream.Stream<any, any>) : result) as Effect.Effect<any, any>, 'Only the owner can run a project terminal.')
    }
  }
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
live('real sh delivers split secrets unchanged only to its opener, strips private environment and leaves no durable or protocol logs', () => withPtyPrivacy(withServer(({ client, socket, bearer }) => Effect.gen(function* () {
  yield* enabled(client, true)
  const { sessionId } = yield* open(client)
  let text = ''
  yield* output(client, sessionId).pipe(Stream.runForEach(item => Effect.sync(() => { if (item._tag === 'output') text += item.text })), Effect.forkScoped)
  // Disable terminal echo so the assertion checks the child output as well as input privacy.
  yield* req(client, 'write', { sessionId, text: 'stty -echo\r' }); yield* Effect.sleep(100)
  yield* req(client, 'write', { sessionId, text: `printf '${marker} '; printf 'sk-registered-terminal-'; sleep 0.1; printf 'secret\\n'; printf '${keys}\\n'; env; printf 'ENV_DONE\\n'\r` })
  yield* waitFor('shell output', () => text.includes('ENV_DONE'))
  expect(text).toContain(marker + ' ' + known); expect(text).not.toContain('[REDACTED]')
  const other = yield* connect(socket, bearer)
  yield* failure(output(other, sessionId).pipe(Stream.runDrain), 'This terminal belongs to another connection.')
  expect(text).not.toMatch(/\bLGTM_[A-Za-z0-9_]*=/); expect(text).not.toContain('LGTM_TEST_TERMINAL='); expect(text).not.toContain('NORMAL_PRIVATE=')
  yield* req(client, 'resize', { sessionId, cols: 100, rows: 30 })
  yield* req(client, 'close', { sessionId })
  const input = yield* (yield* CommandHandler).input({ _tag: 'owner' })
  const tables = yield* input.runtime.sql<{ name: string }>`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`
  for (const { name } of tables) {
    const data = JSON.stringify(yield* input.runtime.sql.unsafe('SELECT * FROM "' + name.replaceAll('"', '""') + '"'))
    expect(data).not.toContain(marker); expect(data).not.toContain(keys); expect(data).not.toContain(known)
  }
  const snapshot = JSON.stringify(yield* input.runtime.views.snapshot)
  expect(snapshot).not.toContain(marker); expect(snapshot).not.toContain(keys); expect(snapshot).not.toContain(known)
})), [marker, keys, known]))
live('terminal refuses a ninth live shell', () => withServer(({ client }) => Effect.gen(function* () {
  yield* enabled(client, true)
  const ids: string[] = []
  for (let i = 0; i < 8; i++) ids.push((yield* open(client)).sessionId)
  yield* failure(open(client), 'Eight project terminal sessions are already open.')
  yield* req(client, 'close', { sessionId: ids[0] }); expect((yield* open(client)).sessionId).toBeTypeOf('string')
})))
for (const mode of ['close', 'disable', 'disconnect', 'shutdown'] as const) live(`terminal ${mode} kills the real shell process group`, () => Effect.gen(function* () {
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
      if (mode === 'close' || mode === 'disable') yield* waitFor(mode, () => pids.every(pid => !alive(pid)))
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

live('a refused duplicate output subscription does not disconnect the real reader', () => withServer(({ client, socket, bearer }) => Effect.gen(function* () {
  yield* enabled(client, true)
  const { sessionId } = yield* open(client)
  let text = ''
  yield* output(client, sessionId).pipe(Stream.runForEach(item => Effect.sync(() => { if (item._tag === 'output') text += item.text })), Effect.forkScoped)
  yield* req(client, 'write', { sessionId, text: 'stty -echo\r' }); yield* Effect.sleep(100)
  yield* req(client, 'write', { sessionId, text: "printf 'CARRY_READY sk-registered-terminal-'; sleep 0.6; printf 'secret\\nCARRY_DONE\\n'\r" })
  yield* waitFor('first secret chunk', () => text.includes('CARRY_READY '))
  const second = yield* connect(socket, bearer)
  yield* failure(output(second, sessionId).pipe(Stream.runDrain), 'This terminal belongs to another connection.')
  yield* failure(output(client, sessionId).pipe(Stream.runDrain), 'This terminal already has an output stream.')
  yield* waitFor('second secret chunk', () => text.includes('CARRY_DONE'))
  yield* Effect.sleep(5500)
  yield* req(client, 'write', { sessionId, text: "printf 'AFTER_DUPLICATE\\n'\r" })
  yield* waitFor('reader survives duplicate beyond the grace period', () => text.includes('AFTER_DUPLICATE'))
  yield* req(client, 'close', { sessionId })
})))

live('several megabytes drain without ending the project shell', () => withServer(({ client }) => Effect.gen(function* () {
  yield* enabled(client, true)
  const { sessionId } = yield* open(client)
  let text = '', ended = false
  yield* output(client, sessionId).pipe(Stream.runForEach(item => Effect.sync(() => { if (item._tag === 'output') text += item.text; else ended = true })), Effect.forkScoped)
  yield* req(client, 'write', { sessionId, text: 'stty -echo\r' }); yield* Effect.sleep(200)
  text = ''
  yield* req(client, 'write', { sessionId, text: "seq 1 400000; printf 'LARGE_DONE\\n'\r" })
  yield* waitFor('large output completion', () => ended || text.includes('LARGE_DONE'), 30000)
  expect(ended).toBe(false)
  expect(text).toContain('400000\r\n'); expect(text.length).toBeGreaterThan(2_500_000)
  yield* req(client, 'write', { sessionId, text: "printf 'AFTER_LARGE\\n'\r" })
  yield* waitFor('same shell remains usable', () => text.includes('AFTER_LARGE'))
  yield* req(client, 'close', { sessionId })
})))
live('typed characters echo with an agent token registered before Enter', () => withServer(({ client }) => Effect.gen(function* () {
  yield* enabled(client, true)
  const { sessionId } = yield* open(client)
  let text = ''
  yield* output(client, sessionId).pipe(Stream.runForEach(item => Effect.sync(() => { if (item._tag === 'output') text += item.text })), Effect.forkScoped)
  yield* Effect.sleep(300); text = ''
  for (const typed of ['ls', 'chmod 777']) {
    text = ''
    yield* req(client, 'write', { sessionId, text: typed })
    yield* waitFor('typed characters echoed before Enter', () => text.includes(typed), 500)
    yield* req(client, 'write', { sessionId, text: '\x15' })
  }
  yield* req(client, 'close', { sessionId })
})))
live('raw and encoded registered tokens and control sequences reach the opener unchanged', () => withServer(({ client }) => Effect.gen(function* () {
  yield* enabled(client, true)
  const { sessionId } = yield* open(client)
  let text = ''
  yield* output(client, sessionId).pipe(Stream.runForEach(item => Effect.sync(() => { if (item._tag === 'output') text += item.text })), Effect.forkScoped)
  yield* req(client, 'write', { sessionId, text: 'stty -echo\r' }); yield* Effect.sleep(200)
  const encoded = Buffer.from('xy' + Redacted.value(token.token)).toString('base64')
  text = ''
  // A shell-side handshake proves the first bytes are delivered without a hold.
  yield* req(client, 'write', { sessionId, text: `printf 'ENCODED_BEGIN ${encoded.slice(0, 3)}'; read terminal_continue; printf '${encoded.slice(3)}\\nENCODED_DONE\\n'\r` })
  yield* waitFor('encoded lead arrives without further output', () => text.includes('ENCODED_BEGIN ' + encoded.slice(0, 3)), 500)
  yield* req(client, 'write', { sessionId, text: '\r' })
  yield* waitFor('encoded token completion', () => text.includes('ENCODED_DONE'))
  expect(text).toContain(encoded)
  const raw = Redacted.value(token.token)
  yield* req(client, 'write', { sessionId, text: `printf '${raw}\\000é🤖\\033[c'; read terminal_continue\r` })
  yield* waitFor('raw token and complete device query', () => text.includes(raw + '\x00é🤖\x1b[c'), 500)
  yield* req(client, 'close', { sessionId })
})))
live('a background job is killed when the shell exits itself', () => withServer(({ client, rig }) => Effect.gen(function* () {
  yield* enabled(client, true)
  const { sessionId } = yield* open(client)
  let ended = false, pid = 0
  yield* output(client, sessionId).pipe(Stream.runForEach(item => Effect.sync(() => { if (item._tag === 'ended') ended = true })), Effect.forkScoped)
  try {
    yield* req(client, 'write', { sessionId, text: `sleep 300 & printf '%s' "$!" > '${join(rig.work, 'exit-job')}'; exit\r` })
    yield* waitFor('shell exit', () => ended)
    pid = Number(readFileSync(join(rig.work, 'exit-job'), 'utf8'))
    yield* waitFor('background job exit', () => !alive(pid), 1000)
  } finally { if (pid && alive(pid)) try { process.kill(pid, 'SIGKILL') } catch {} }
})))
live('a settings save that omits the terminal switch preserves its stored value and shell', () => withServer(({ client }) => Effect.gen(function* () {
  yield* enabled(client, true)
  const { sessionId } = yield* open(client)
  yield* output(client, sessionId).pipe(Stream.runDrain, Effect.forkScoped)
  yield* client['client.web.settings.set']({ requestId: 'old-client-save', settings: { enabled: true, port: 8787, proxyMode: false, heartbeatSeconds: 30 } })
  const settings = yield* client['client.web.settings']({ requestId: 'read-preserved-switch' })
  expect(settings.terminalEnabled).toBe(true)
  yield* req(client, 'write', { sessionId, text: 'echo STILL_OPEN\r' })
  yield* req(client, 'close', { sessionId })
})))

live('s28 web owner session lists and reattaches a terminal with memory scrollback across connections', () => withPtyPrivacy(withServer(({ client, rig }) => Effect.gen(function* () {
  yield* enabled(client, true)
  const port = yield* Effect.promise(unusedPort), password = 'synthetic-s28-web-password'
  const endpoint = { password, settings: { enabled: true, port, proxyMode: false, heartbeatSeconds: 25 }, origin: `http://127.0.0.1:${port}`, host: `127.0.0.1:${port}` }
  const web = yield* importWeb(endpoint, rig.stateFolder)
  const { cookie } = yield* web.login(endpoint)
  const one = yield* web.connected(endpoint, cookie)
  const result = yield* web.rpc(one, 'client.terminal.open', { requestId: 's28-open', projectId: 'terminal-project', worktreeId: 'terminal-worktree', cols: 80, rows: 24 })
  expect(result._tag).toBe('Success')
  const sessionId = result.value.sessionId
  yield* web.rpc(one, 'client.terminal.write', { requestId: 's28-print', sessionId, text: "printf 'S28_REATTACH_MARKER\\n'\r" })
  yield* Effect.sleep(100)
  one.terminate()
  const two = yield* web.connected(endpoint, cookie)
  const listed = yield* web.rpc(two, 'client.terminal.list', { requestId: 's28-list' })
  expect(listed).toMatchObject({ _tag: 'Success', value: { sessions: [{ sessionId, worktreeId: 'terminal-worktree' }] } })
  let text = ''
  two.on('message', raw => { for (const line of String(raw).split('\n').filter(Boolean)) { const m = JSON.parse(line); if (m._tag === 'Chunk' && m.requestId === 's28-output') { for (const item of m.values) if (item._tag === 'output') text += item.text; two.send(JSON.stringify({ _tag: 'Ack', requestId: m.requestId })) } } })
  two.send(requestFrame('s28-output', 'client.terminal.subscribe', { requestId: 's28-output', sessionId }))
  yield* waitFor('replayed output', () => text.includes('S28_REATTACH_MARKER'))
  expect((yield* web.rpc(two, 'client.terminal.write', { requestId: 's28-print-again', sessionId, text: "echo S28_STILL_SAME_SHELL\r" }))._tag).toBe('Success')
  yield* waitFor('new output after reattach', () => text.includes('S28_STILL_SAME_SHELL'))
  // A second browser is the same owner and leaves the desktop session alive.
  const next = yield* web.login(endpoint), other = yield* web.connected(endpoint, next.cookie)
  const foreign = yield* web.rpc(other, 'client.terminal.write', { requestId: 's28-phone', sessionId, text: 'echo S28_PHONE_KEYS\r' })
  expect(foreign._tag).toBe('Success')
  expect((yield* web.rpc(other, 'client.terminal.list', { requestId: 's28-new-list' }))).toMatchObject({ _tag: 'Success', value: { sessions: [{ sessionId, projectId: 'terminal-project', worktreeId: 'terminal-worktree' }] } })
})), ['S28_REATTACH_MARKER', 'S28_STILL_SAME_SHELL']))

const importWeb = (endpoint: { password: string; settings: { enabled: boolean; port: number; proxyMode: boolean; heartbeatSeconds: number } }, stateFolder: string) => Effect.gen(function* () {
  yield* serve({ stateFolder, password: Redacted.make(endpoint.password), staticRoot: '/does-not-exist', settings: endpoint.settings, lists: engineLists })
  return yield* Effect.promise(() => import('../support/webRig.ts'))
})

const withWebTerminal = (body: (ctx: { web: typeof import('../support/webRig.ts'); endpoint: import('../support/webRig.ts').WebEndpoint; cookie: string; rig: ReturnType<typeof makeRig> }) => Effect.Effect<any, any, any>) => withServer(({ client, rig }) => Effect.gen(function* () {
  yield* enabled(client, true)
  const port = yield* Effect.promise(unusedPort), password = 'synthetic-s28-bounded-password'
  const endpoint = { password, settings: { enabled: true, port, proxyMode: false, heartbeatSeconds: 25 }, origin: `http://127.0.0.1:${port}`, host: `127.0.0.1:${port}` }
  const web = yield* importWeb(endpoint, rig.stateFolder), { cookie } = yield* web.login(endpoint)
  return yield* body({ web, endpoint, cookie, rig })
}))
const webOpen = (web: typeof import('../support/webRig.ts'), socket: import('ws').default) => web.rpc(socket, 'client.terminal.open', { requestId: randomBytes(8).toString('hex'), projectId: 'terminal-project', worktreeId: 'terminal-worktree', cols: 80, rows: 24 })
live('s28 detached web terminals expire after the bounded grace and still count towards the eight-shell limit', () => withWebTerminal(({ web, endpoint, cookie }) => Effect.gen(function* () {
  const handler = yield* CommandHandler
  const { sessions } = yield* Effect.promise(() => import('../../src/harnesses/signinRequests.ts'))
  const manager = sessions(yield* handler.input({ _tag: 'owner' }))
  const grace = vi.spyOn(manager as any, 'graceMs')
  const one = yield* web.connected(endpoint, cookie), ids: string[] = [], pids: number[] = []
  try {
    for (let i = 0; i < 8; i++) { const result = yield* webOpen(web, one); expect(result._tag).toBe('Success'); ids.push(result.value.sessionId); pids.push(vi.mocked(Pty.spawn).mock.results.at(-1)!.value.pid) }
    expect(grace.mock.results.every(r => r.value === 300000)).toBe(true)
    // Only the grace clock is shortened. The server, shell and socket remain real.
    grace.mockReturnValue(600)
    one.terminate()
    const two = yield* web.connected(endpoint, cookie)
    expect((yield* web.rpc(two, 'client.terminal.list', { requestId: 'detached-list' }))).toMatchObject({ _tag: 'Success', value: { sessions: expect.arrayContaining(ids.map(sessionId => ({ sessionId, projectId: 'terminal-project', worktreeId: 'terminal-worktree' }))) } })
    expect((yield* webOpen(web, two))._tag).toBe('Failure')
    yield* waitFor('detached grace closes process groups', () => pids.every(pid => !alive(pid)))
    expect((yield* web.rpc(two, 'client.terminal.list', { requestId: 'expired-list' }))).toMatchObject({ _tag: 'Success', value: { sessions: [] } })
  } finally { grace.mockRestore() }
})))

live('s28 scrollback stays below 64 KiB and sign-out kills the reattached shell', () => withPtyPrivacy(withWebTerminal(({ web, endpoint, cookie }) => Effect.gen(function* () {
  const one = yield* web.connected(endpoint, cookie), opened = yield* webOpen(web, one)
  const sessionId = opened.value.sessionId, pid = vi.mocked(Pty.spawn).mock.results.at(-1)!.value.pid
  one.terminate()
  const two = yield* web.connected(endpoint, cookie)
  yield* web.rpc(two, 'client.terminal.write', { requestId: 'scrollback-print', sessionId, text: "printf 'S28_OLD_OUTPUT'; head -c 200000 /dev/zero | tr '\\000' x; printf '\\nS28_RECENT_OUTPUT\\n'\r" })
  yield* Effect.sleep(500)
  let text = '', firstBytes = -1
  two.on('message', raw => { for (const line of String(raw).split('\n').filter(Boolean)) { const m = JSON.parse(line); if (m._tag === 'Chunk' && m.requestId === 'bounded-output') { for (const item of m.values) if (item._tag === 'output') { if (firstBytes < 0) firstBytes = Buffer.byteLength(item.text); text += item.text } two.send(JSON.stringify({ _tag: 'Ack', requestId: m.requestId })) } } })
  two.send(requestFrame('bounded-output', 'client.terminal.subscribe', { requestId: 'bounded-output', sessionId }))
  yield* waitFor('recent output in bounded scrollback', () => text.includes('S28_RECENT_OUTPUT'))
  expect(firstBytes).toBeLessThanOrEqual(64 * 1024)
  expect(text.includes('S28_OLD_OUTPUT')).toBe(false)
  const third = yield* web.connected(endpoint, cookie)
  yield* webOpen(web, third)
  const detachedPid = vi.mocked(Pty.spawn).mock.results.at(-1)!.value.pid
  third.terminate()
  expect((yield* Effect.promise(() => web.post(endpoint, '/api/logout', undefined, { Cookie: cookie }))).status).toBe(204)
  yield* waitFor('sign-out kills attached and detached shells', () => !alive(pid) && !alive(detachedPid))
  expect((yield* web.upgrade(endpoint, { Cookie: cookie })).status).toBe(401)
})), ['S28_OLD_OUTPUT', 'S28_RECENT_OUTPUT']))

live('r2 a phone takes over a half-open desktop stream, both can type, and logout preserves the attached handoff', () => withPtyPrivacy(withServer(({ client, rig }) => Effect.gen(function* () {
  yield* enabled(client, true)
  const local = yield* open(client)
  const localPid = vi.mocked(Pty.spawn).mock.results.at(-1)!.value.pid
  const port = yield* Effect.promise(unusedPort)
  const endpoint = { password: 'synthetic-r2-devices', settings: { enabled: true, port, proxyMode: false, heartbeatSeconds: 25 }, origin: `http://127.0.0.1:${port}`, host: `127.0.0.1:${port}` }
  const web = yield* importWeb(endpoint, rig.stateFolder), desktop = yield* web.login(endpoint)
  const one = yield* web.connected(endpoint, desktop.cookie, false)
  const opened = yield* webOpen(web, one), sessionId = opened.value.sessionId
  const shellPid = vi.mocked(Pty.spawn).mock.results.at(-1)!.value.pid
  let firstText = '', replay = '', oldEnded = false, oldExited = false
  one.on('message', raw => { for (const line of String(raw).split('\n').filter(Boolean)) {
    const frame = JSON.parse(line)
    if (frame.requestId !== 'desktop-stream') continue
    if (frame._tag === 'Chunk') { for (const item of frame.values) { if (item._tag === 'output') firstText += item.text; else if (item.reason.includes('another')) oldEnded = true } one.send(JSON.stringify({ _tag: 'Ack', requestId: frame.requestId })) }
    if (frame._tag === 'Exit') { expect(frame.exit._tag).toBe('Success'); oldExited = true }
  } })
  one.send(requestFrame('desktop-stream', 'client.terminal.subscribe', { requestId: 'desktop-stream', sessionId }))
  yield* web.rpc(one, 'client.terminal.write', { requestId: 'before-handoff', sessionId, text: "sleep 120 & printf '%s' \"$!\" > 'r2-descendant'; printf 'R2_HANDOFF_%s\\n' BEFORE\r" })
  yield* waitFor('desktop output', () => firstText.includes('R2_HANDOFF_BEFORE'))
  const descendant = Number(readFileSync(join(rig.work, 'r2-descendant'), 'utf8'))
  const transport = (one as unknown as { _socket: { pause(): void; resume(): void } })._socket
  transport.pause() // The server has not observed a close or a missed heartbeat.
  const phone = yield* web.login(endpoint), two = yield* web.connected(endpoint, phone.cookie)
  expect((yield* web.rpc(two, 'client.terminal.list', { requestId: 'phone-list' })).value.sessions).toContainEqual({ sessionId, projectId: 'terminal-project', worktreeId: 'terminal-worktree' })
  yield* failure(req(client, 'write', { sessionId, text: 'must not enter shell' }), 'This terminal belongs to another connection.')
  expect((yield* req(client, 'list')).sessions.some((row: any) => row.sessionId === sessionId)).toBe(false)
  expect((yield* web.upgrade(endpoint)).status).toBe(401)
  two.on('message', raw => { for (const line of String(raw).split('\n').filter(Boolean)) { const frame = JSON.parse(line); if (frame._tag === 'Chunk' && frame.requestId === 'phone-stream') { for (const item of frame.values) if (item._tag === 'output') replay += item.text; two.send(JSON.stringify({ _tag: 'Ack', requestId: frame.requestId })) } } })
  two.send(requestFrame('phone-stream', 'client.terminal.subscribe', { requestId: 'phone-stream', sessionId }))
  yield* waitFor('phone replay', () => replay.includes('R2_HANDOFF_BEFORE'))
  transport.resume()
  yield* waitFor('old reader ends cleanly', () => oldEnded && oldExited)
  expect((yield* web.rpc(one, 'client.terminal.write', { requestId: 'desktop-keys', sessionId, text: "printf 'R2_DESKTOP_%s\\n' KEYS\r" }))._tag).toBe('Success')
  expect((yield* web.rpc(two, 'client.terminal.write', { requestId: 'phone-keys', sessionId, text: "printf 'R2_PHONE_%s\\n' KEYS\r" }))._tag).toBe('Success')
  yield* waitFor('both browsers can type', () => replay.includes('R2_DESKTOP_KEYS') && replay.includes('R2_PHONE_KEYS'))
  expect((yield* Effect.promise(() => web.post(endpoint, '/api/logout', undefined, { Cookie: desktop.cookie }))).status).toBe(204)
  expect((yield* Effect.promise(() => web.post(endpoint, '/api/session', undefined, { Cookie: desktop.cookie }))).status).toBe(401)
  expect((yield* Effect.promise(() => web.post(endpoint, '/api/session', undefined, { Cookie: phone.cookie }))).status).toBe(204)
  expect(alive(shellPid) && alive(descendant) && alive(localPid)).toBe(true)
  expect((yield* web.rpc(two, 'client.terminal.write', { requestId: 'inherited', sessionId, text: "printf 'R2_%s\\n' INHERITED\r" }))._tag).toBe('Success')
  yield* waitFor('inherited shell output', () => replay.includes('R2_INHERITED'))
  expect((yield* Effect.promise(() => web.post(endpoint, '/api/logout', undefined, { Cookie: phone.cookie }))).status).toBe(204)
  yield* waitFor('last browser closes inherited shell and descendant', () => !alive(shellPid) && !alive(descendant))
  expect(alive(localPid)).toBe(true)
  yield* req(client, 'close', { sessionId: local.sessionId })
})), ['R2_HANDOFF_BEFORE', 'R2_DESKTOP_KEYS', 'R2_PHONE_KEYS', 'R2_INHERITED']))

live('r3 a typing-only phone keeps a detached desktop shell through logout and reload', () => withWebTerminal(({ web, endpoint, cookie }) => Effect.gen(function* () {
  const desktop = yield* web.connected(endpoint, cookie), opened = yield* webOpen(web, desktop)
  const sessionId = opened.value.sessionId, pid = vi.mocked(Pty.spawn).mock.results.at(-1)!.value.pid
  const phone = yield* web.login(endpoint), typing = yield* web.connected(endpoint, phone.cookie)
  expect((yield* web.rpc(typing, 'client.terminal.write', { requestId: 'typing-only', sessionId, text: "printf 'R3_%s\\n' TYPED\r" }))._tag).toBe('Success')
  desktop.terminate(); typing.terminate()
  expect((yield* Effect.promise(() => web.post(endpoint, '/api/logout', undefined, { Cookie: cookie }))).status).toBe(204)
  expect(alive(pid)).toBe(true)
  const reloaded = yield* web.connected(endpoint, phone.cookie)
  expect((yield* web.rpc(reloaded, 'client.terminal.list', { requestId: 'reload-list' })).value.sessions).toContainEqual({ sessionId, projectId: 'terminal-project', worktreeId: 'terminal-worktree' })
  expect((yield* web.rpc(reloaded, 'client.terminal.write', { requestId: 'reload-write', sessionId, text: "printf 'R3_%s\\n' RELOADED\r" }))._tag).toBe('Success')
  expect((yield* Effect.promise(() => web.post(endpoint, '/api/logout', undefined, { Cookie: phone.cookie }))).status).toBe(204)
  yield* waitFor('last session ends a shell with no reader', () => !alive(pid))
})))
