import { expect, it } from '@effect/vitest'
import * as Effect from 'effect/Effect'
import * as Stream from 'effect/Stream'
import * as Redacted from 'effect/Redacted'
import { withPtyPrivacy } from '../support/ptyPrivacy.ts'
import { chmodSync, existsSync, mkdirSync, readFileSync, symlinkSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import * as Server from '../../src/api/server.ts'
import { CommandHandler } from '../../src/commands/CommandHandler.ts'
import { makeRedactor } from '../../src/identity/redactor.ts'
import { engineLists } from '../../src/pieces/lists.ts'
import { makeFakeRunner } from '../support/fakeRunner.ts'
import { appendSetup, eventOf, harnessSettings, makeFakeSecrets, makeRig, provideRig, removeRig, seedAccount, seedAgent, seedHarness } from '../support/handlerRig.ts'
import { connect } from '../support/socketClient.ts'
import { alive, waitFor } from '../support/serviceProcess.ts'
import { fixtureHarnessPrograms } from '../support/fixtureHarnessPrograms.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'

const marker = 'PRIVATE_RPC_SIGNIN_MARKER', keys = 'PRIVATE_RPC_KEYS_MARKER', known = 'registered-signin-secret'
const live = (name: string, body: () => Effect.Effect<unknown, unknown, any>) => it.live(name, () => body() as Effect.Effect<void, unknown, never>, 45000)
const withServer = <A, E>(body: (context: { socket: string; bearer: string; rig: ReturnType<typeof makeRig>; invoked: string[] }) => Effect.Effect<A, E, any>) => Effect.gen(function* () {
  // Keep the synthetic bearer fixed so PTY assertions have no random secret forms.
  const rig = makeRig(), bearer = 'fixed-owner-signin-bearer-0123456789', socket = join(rig.stateFolder, 'engine.sock'), invoked: string[] = []
  chmodSync(rig.stateFolder, 0o700)
  const script = join(rig.root, 'signin.cjs')
  writeFileSync(script, `if(!process.stdin.isTTY||!process.stdout.isTTY)process.exit(88);const fs=require('node:fs'),path=require('node:path');const mode=process.env.MODE;if(mode==='overflow'){process.stdout.write('x'.repeat(70000));setInterval(()=>{},1000)}else{const child=require('node:child_process').spawn(process.execPath,['-e','setInterval(()=>{},1000)'],{stdio:'ignore'});fs.writeFileSync(path.join(process.env.CLAUDE_CONFIG_DIR,'pids'),JSON.stringify([process.pid,child.pid]));process.stdout.write('${marker} '+(process.env.REGISTERED||'')+' Press y: ');process.stdin.setRawMode(true);process.stdin.once('data',()=>{fs.writeFileSync(path.join(process.env.CLAUDE_CONFIG_DIR,'.credentials.json'),JSON.stringify({claudeAiOauth:{accessToken:'new-credential'}}));child.kill('SIGKILL');process.exit(0)});setInterval(()=>{},1000)}`)
  const base = fixtureHarnessPrograms({ home: rig.home, executable: process.execPath, script, id: 'claude' })
  const programs = { ...base, recipe: (id: string) => { const r = base.recipe(id); return r && { ...r, signInArguments: [script] } } }
  // Count handler entry so the agent checks prove the transport gate itself refused.
  const lists = { ...engineLists, reads: Object.fromEntries(Object.entries(engineLists.reads).map(([tag, read]) => [tag, (input: Parameters<typeof read>[0], payload: Parameters<typeof read>[1]) => { invoked.push(input.actor._tag + ':' + tag); return read(input, payload) }])) }
  const redactor = makeRedactor({ secrets: () => [Redacted.make(known), Redacted.make(bearer)], tokenHashes: () => new Set() })
  return yield* provideRig(rig, { runner: yield* makeFakeRunner(), secrets: makeFakeSecrets(), harnessPrograms: programs, redactor: redactor.layer, lists }, Effect.gen(function* () {
    yield* registerRebuildCheck()
    yield* seedHarness(); yield* seedAccount()
    yield* appendSetup([eventOf({ _tag: 'HarnessChanged', harness: harnessSettings('claude') })])
    yield* Server.serve({ socketPath: socket, ownerCredential: Redacted.make(bearer), debug: false, lists })
    return yield* body({ socket, bearer, rig, invoked })
  })).pipe(Effect.ensuring(Effect.sync(() => removeRig(rig))))
})

live('review 5: real access gate, owner connection isolation, unchanged output and ephemeral marker privacy', () => withPtyPrivacy(withServer(({ socket, bearer, rig, invoked }) => Effect.gen(function* () {
  const agent = yield* seedAgent({ id: 'worker' }), asAgent = yield* connect(socket, agent.token)
  const refusal = { _tag: 'Failure', failure: { reason: 'Only the owner can run a sign-in session.' } }
  expect(yield* asAgent['client.harness.signin.start']({ requestId: 'agent-start', accountId: 'none' } as never).pipe(Effect.result)).toMatchObject(refusal)
  expect(yield* asAgent['client.harness.signin.write']({ requestId: 'agent-write', sessionId: 'none', text: 'y' } as never).pipe(Effect.result)).toMatchObject(refusal)
  expect(yield* asAgent['client.harness.signin.resize']({ requestId: 'agent-resize', sessionId: 'none', cols: 80, rows: 24 } as never).pipe(Effect.result)).toMatchObject(refusal)
  expect(yield* asAgent['client.harness.signin.stop']({ requestId: 'agent-stop', sessionId: 'none' } as never).pipe(Effect.result)).toMatchObject(refusal)
  expect(yield* asAgent['client.harness.signin.subscribe']({ requestId: 'agent-stream', sessionId: 'none' } as never).pipe(Stream.runDrain, Effect.result)).toMatchObject(refusal)
  expect(invoked.some(tag => tag.startsWith('agent:client.harness.signin.'))).toBe(false)
  const one = yield* connect(socket, bearer), two = yield* connect(socket, bearer)
  const added = yield* one['client.harness.account.add']({ requestId: 'add', harnessId: 'claude', name: 'Test', folder: 'separate' } as never)
  const handler = yield* CommandHandler, input = yield* handler.input({ _tag: 'owner' })
  yield* input.runtime.secrets.put('secret:test', Redacted.make(known))
  yield* appendSetup([eventOf({ _tag: 'AccountChanged', accountId: added.account.id, account: { ...added.account, environment: { REGISTERED: 'secret:test' } } })])
  const { sessionId } = yield* one['client.harness.signin.start']({ requestId: 'start', accountId: added.account.id } as never)
  const wrong = { _tag: 'Failure', failure: { reason: 'This sign-in belongs to another connection.' } }
  expect(yield* two['client.harness.signin.write']({ requestId: 'wrong-write', sessionId, text: 'y' } as never).pipe(Effect.result)).toMatchObject(wrong)
  expect(yield* two['client.harness.signin.resize']({ requestId: 'wrong-resize', sessionId, cols: 80, rows: 24 } as never).pipe(Effect.result)).toMatchObject(wrong)
  expect(yield* two['client.harness.signin.stop']({ requestId: 'wrong-stop', sessionId } as never).pipe(Effect.result)).toMatchObject(wrong)
  expect(yield* two['client.harness.signin.subscribe']({ requestId: 'wrong-read', sessionId } as never).pipe(Stream.runDrain, Effect.result)).toMatchObject(wrong)
  let output = '', sent = false, state = ''
  yield* one['client.harness.signin.subscribe']({ requestId: 'output', sessionId } as never).pipe(Stream.runForEach(item => Effect.gen(function* () {
    if (item._tag === 'output') {
      output += item.text
      if (!sent && output.includes('Press y')) { sent = true; yield* one['client.harness.signin.write']({ requestId: 'keys', sessionId, text: keys } as never) }
    } else state = item.check.state
  })))
  expect(output.includes(marker)).toBe(true)
  expect(output.includes(known)).toBe(true)
  // This also proves the resolved secret reached the child rather than being dropped.
  expect(output.includes('[REDACTED]')).toBe(false)
  expect(state).toBe('local-credential')
  expect(existsSync(join(rig.home, '.claude'))).toBe(false)
  const tables = yield* input.runtime.sql<{ name: string }>`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`
  for (const { name } of tables) {
    const content = JSON.stringify(yield* input.runtime.sql.unsafe('SELECT * FROM "' + name.replaceAll('"', '""') + '"'))
    expect(content.includes(marker) || content.includes(keys) || content.includes(known)).toBe(false)
  }
  const snapshot = JSON.stringify(yield* input.runtime.views.snapshot)
  expect(snapshot.includes(marker) || snapshot.includes(keys) || snapshot.includes(known)).toBe(false)
})), [marker, keys, known]))

for (const mode of ['disconnect', 'shutdown'] as const) live(`review 5: ${mode} kills the real PTY group`, () => Effect.gen(function* () {
  let pids: number[] = []
  yield* withServer(({ socket, bearer }) => Effect.gen(function* () {
    yield* Effect.scoped(Effect.gen(function* () {
      const client = yield* connect(socket, bearer)
      const { account } = yield* client['client.harness.account.add']({ requestId: 'add', harnessId: 'claude', name: 'Wait', folder: 'separate' } as never)
      const { sessionId } = yield* client['client.harness.signin.start']({ requestId: 'start', accountId: account.id } as never)
      let attached = false
      yield* client['client.harness.signin.subscribe']({ requestId: 'attach', sessionId } as never).pipe(Stream.runForEach(item => Effect.sync(() => { if (item._tag === 'output' && item.text.includes(marker)) attached = true })), Effect.forkScoped)
      yield* waitFor('owner stream attached', () => attached)
      yield* waitFor('fake process group', () => { try { pids = JSON.parse(readFileSync(join(account.home!, 'pids'), 'utf8')); return pids.length === 2 } catch { return false } })
      expect(pids.every(alive)).toBe(true)
    })) // Close the actual owner socket.
    if (mode === 'disconnect') yield* waitFor('disconnect grace to kill the group', () => pids.every(pid => !alive(pid)), 10000)
  })) // Close the engine's scope with a session still alive in shutdown mode.
  yield* waitFor('shutdown to kill the group', () => pids.every(pid => !alive(pid)), 5000)
}))

live('review 5: real application enforces the cap, unread output bound and final symlink refusal', () => withServer(({ socket, bearer, rig }) => Effect.gen(function* () {
  const client = yield* connect(socket, bearer), sessions: string[] = []
  const accountIds: string[] = []
  for (let i = 0; i < 17; i++) {
    const { account } = yield* client['client.harness.account.add']({ requestId: 'add-' + i, harnessId: 'claude', name: 'Wait', folder: 'separate' } as never)
    accountIds.push(account.id)
    const result = yield* client['client.harness.signin.start']({ requestId: 'start-' + i, accountId: account.id } as never).pipe(Effect.result)
    if (i < 16) { expect(result._tag).toBe('Success'); if (result._tag === 'Success') sessions.push(result.success.sessionId) }
    else expect(result).toMatchObject({ _tag: 'Failure', failure: { reason: 'A sign-in is already running for this account.' } })
  }
  for (const sessionId of sessions) {
    yield* client['client.harness.signin.stop']({ requestId: 'stop-' + sessionId, sessionId } as never)
    yield* client['client.harness.signin.subscribe']({ requestId: 'drain-' + sessionId, sessionId } as never).pipe(Stream.runDrain)
  }
  const handler = yield* CommandHandler, input = yield* handler.input({ _tag: 'owner' })
  const account = (yield* input.runtime.views.snapshot).accounts[accountIds[0]!]!
  yield* appendSetup([eventOf({ _tag: 'AccountChanged', accountId: account.id, account: { ...account, environment: { MODE: 'overflow' } } })])
  const { sessionId } = yield* client['client.harness.signin.start']({ requestId: 'overflow', accountId: account.id } as never)
  yield* Effect.sleep(500)
  let reason = ''
  yield* client['client.harness.signin.subscribe']({ requestId: 'overflow-read', sessionId } as never).pipe(Stream.runForEach(item => Effect.sync(() => { if (item._tag === 'ended') reason = item.reason })))
  expect(reason).toBe('The sign-in produced too much unread output.')
  const target = join(rig.home, 'target'), link = join(rig.home, 'link')
  mkdirSync(target); symlinkSync(target, link)
  expect(yield* client['client.harness.account.add']({ requestId: 'link', harnessId: 'claude', name: 'Link', folder: 'advanced', home: link } as never).pipe(Effect.result)).toMatchObject({ _tag: 'Failure', failure: { reason: 'The login folder must be owned by you and have no symbolic links.' } })
})))

live('a refused duplicate sign-in subscription leaves the real reader alive past its grace period', () => withServer(({ socket, bearer }) => Effect.gen(function* () {
  const client = yield* connect(socket, bearer)
  const { account } = yield* client['client.harness.account.add']({ requestId: 'add-duplicate', harnessId: 'claude', name: 'Wait', folder: 'separate' } as never)
  const { sessionId } = yield* client['client.harness.signin.start']({ requestId: 'start-duplicate', accountId: account.id } as never)
  let text = '', ended = false
  yield* client['client.harness.signin.subscribe']({ requestId: 'original', sessionId } as never).pipe(Stream.runForEach(item => Effect.sync(() => { if (item._tag === 'output') text += item.text; else ended = true })), Effect.forkScoped)
  yield* waitFor('sign-in prompt', () => text.includes('Press y'))
  expect(yield* client['client.harness.signin.subscribe']({ requestId: 'duplicate', sessionId } as never).pipe(Stream.runDrain, Effect.result)).toMatchObject({ _tag: 'Failure', failure: { reason: 'This sign-in already has an output stream.' } })
  yield* Effect.sleep(5500)
  expect(ended).toBe(false)
  yield* client['client.harness.signin.write']({ requestId: 'finish-original', sessionId, text: 'y' } as never)
  yield* waitFor('original sign-in completes', () => ended)
})))
