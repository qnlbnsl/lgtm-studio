import { afterEach, expect, it } from '@effect/vitest'
import * as Effect from 'effect/Effect'
import * as Stream from 'effect/Stream'
import * as Layer from 'effect/Layer'
import * as Logger from 'effect/Logger'
import { writeFileSync, readFileSync, existsSync } from 'node:fs'
import { join } from 'node:path'
import { CommandHandler } from '../../src/commands/CommandHandler.ts'
import { Redactor } from '../../src/log/Redactor.ts'
import { signInReads } from '../../src/harnesses/signinRequests.ts'
import { makeRig, removeRig, provideRig, makeFakeSecrets, appendSetup, eventOf, harnessSettings, readLog } from '../support/handlerRig.ts'
import { makeFakeRunner } from '../support/fakeRunner.ts'
import { fixtureHarnessPrograms } from '../support/fixtureHarnessPrograms.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
let rig: ReturnType<typeof makeRig>
afterEach(() => { if (rig) removeRig(rig) })
it.live('refuses agents for every sign-in request and sends fake output only to the initiating connection without durable leaks', () => Effect.gen(function* () {
  rig = makeRig(); const script = join(rig.root, 'signin.cjs')
  writeFileSync(script, `if(!process.stdin.isTTY)process.exit(88);process.stdout.write('PRIVATE_SIGNIN_MARKER known-secret Press y');process.stdin.setRawMode(true);process.stdin.once('data',()=>{require('node:fs').writeFileSync(require('node:path').join(process.env.CLAUDE_CONFIG_DIR,'.credentials.json'),JSON.stringify({claudeAiOauth:{accessToken:'new-credential'}}));process.exit(0)});`)
  const testRedactor = { redact: (e: any) => Effect.succeed(e), redactText: (t: string) => Effect.succeed(t.replaceAll('known-secret', '[REDACTED]')) }
  const base = fixtureHarnessPrograms({ home: rig.home, executable: process.execPath, script, id: 'claude' })
  let mode = 'normal'
  const diagnostics: string[] = []
  const logger = Logger.make(options => { diagnostics.push(Logger.formatJson.log(options)) })
  const programs = { ...base, inspect: async (...args: Parameters<typeof base.inspect>) => mode === 'missing' ? { harnessId: 'claude' as never, installed: false } : base.inspect(...args), recipe: (id: string) => { const r = base.recipe(id); return r ? { ...r, signInCommand: mode === 'no-command' ? '' : r.signInCommand, signInArguments: [script] } : undefined } }
  yield* provideRig(rig, { runner: yield* makeFakeRunner(), secrets: makeFakeSecrets(), harnessPrograms: programs,
    redactor: Layer.succeed(Redactor, testRedactor) }, Effect.gen(function* () {
    yield* registerRebuildCheck()
    yield* appendSetup([eventOf({ _tag: 'HarnessChanged', harness: harnessSettings('claude', { adapterPath: process.execPath }) })])
    const handler = yield* CommandHandler
    const input = { ...yield* handler.input({ _tag: 'owner' }), connectionId: 'one' }
    const added = yield* signInReads['client.harness.account.add'](input, { requestId: 'add', harnessId: 'claude', name: '../../hostile; $(touch bad)', folder: 'separate' })
    const account = added.reply.account as any
    expect(account.home.startsWith(join(rig.home, '.lgtm/accounts/claude/'))).toBe(true)
    const current = { ...yield* handler.input({ _tag: 'owner' }), connectionId: 'one' }
    for (const [next, reason] of [['no-command', 'No usable sign-in command is shipped for this harness.'], ['missing', 'The harness program is not installed.']]) {
      mode = next!
      expect(yield* signInReads['client.harness.signin.start'](current, { requestId: 'refused', accountId: account.id }).pipe(Effect.result)).toMatchObject({ _tag: 'Failure', failure: { reason } })
    }
    mode = 'normal'
    for (const tag of ['start', 'write', 'resize', 'stop', 'subscribe']) {
      const result = signInReads['client.harness.signin.' + tag as keyof typeof signInReads]({ ...current, actor: { _tag: 'agent', agentId: 'worker' } as any }, { requestId: 'agent', accountId: account.id, sessionId: 'x', text: 'y', cols: 80, rows: 24 })
      const checked = Stream.isStream(result) ? Stream.runDrain(result) : result
      expect(yield* checked.pipe(Effect.provideService(Redactor, testRedactor), Effect.result)).toMatchObject({ _tag: 'Failure', failure: { reason: 'Only the owner can run a sign-in session.' } })
    }
    const started = yield* signInReads['client.harness.signin.start'](current, { requestId: 'start', accountId: account.id })
    const sessionId = started.reply.sessionId
    expect(yield* signInReads['client.harness.signin.write']({ ...current, connectionId: 'two' }, { requestId: 'wrong', sessionId, text: 'y' }).pipe(Effect.result)).toMatchObject({ _tag: 'Failure', failure: { reason: 'This sign-in belongs to another connection.' } })
    let text = '', state = '', sent = false
    yield* signInReads['client.harness.signin.subscribe'](current, { requestId: 'stream', sessionId }).pipe(Stream.provideService(Redactor, testRedactor), Stream.runForEach(item => Effect.gen(function* () {
      if (item._tag === 'output') { text += item.text; if (!sent && text.includes('Press y')) { sent = true; yield* signInReads['client.harness.signin.write'](current, { requestId: 'write', sessionId, text: 'PRIVATE_KEY_INPUT_MARKER' }) } }
      else state = item.check.state
    })))
    expect(text.includes('PRIVATE_SIGNIN_MARKER')).toBe(true)
    expect(text.includes('known-secret')).toBe(false)
    expect(state).toBe('local-credential')
    expect(existsSync(join(rig.home, '.claude/.credentials.json'))).toBe(false)
    const rows = yield* readLog
    expect(JSON.stringify(rows).includes('PRIVATE_SIGNIN_MARKER')).toBe(false)
    expect(JSON.stringify(yield* input.runtime.views.snapshot).includes('PRIVATE_SIGNIN_MARKER')).toBe(false)
    const tables = yield* input.runtime.sql<{ name: string }>`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`
    for (const { name } of tables) {
      const content = yield* input.runtime.sql.unsafe('SELECT * FROM "' + name.replaceAll('"','""') + '"')
      expect(JSON.stringify(content).includes('PRIVATE_SIGNIN_MARKER')).toBe(false)
      expect(JSON.stringify(content).includes('PRIVATE_KEY_INPUT_MARKER')).toBe(false)
    }
  })).pipe(Effect.provide(Logger.layer([logger])))
  expect(diagnostics.join('\n').includes('PRIVATE_SIGNIN_MARKER')).toBe(false)
  expect(diagnostics.join('\n').includes('PRIVATE_KEY_INPUT_MARKER')).toBe(false)
}))
