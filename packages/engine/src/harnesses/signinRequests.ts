import { createHash, randomUUID } from 'node:crypto'
import * as Effect from 'effect/Effect'
import * as Exit from 'effect/Exit'
import * as Stream from 'effect/Stream'
import * as Redacted from 'effect/Redacted'
import * as NodeServices from '@effect/platform-node/NodeServices'
import { terminalReasons } from './terminalReasons.ts'
import { Account } from '@lgtm/contracts'
import * as Schema from 'effect/Schema'
import { readHarnesses, refused, type Input } from '../commands/internal.ts'
import { accounts as planAccounts } from '../commands/accounts.ts'
import { prepareAccountHome, removeCreatedHomes, folderReasons } from '../accounts/loginHome.ts'
import { withCodexHome } from '../accounts/shadowHome.ts'
import { credentialSourceFor } from '../accounts/credentialSource.ts'
import { harnessLogin, homeConflict, loginRefusal } from '../runner/harnesses.ts'
import { buildBaseEnvironment } from '../runner/baseEnvironment.ts'
import { accountReadiness } from '../project/leadDefaults.ts'
import { embeddedHarnessPrograms } from './programs.ts'
import { SignInSessions } from './signinSessions.ts'
import { signInReasons } from './signinReasons.ts'
const instances = new WeakMap<object, SignInSessions>()
const known = new Set<string>([...Object.values(signInReasons), ...Object.values(folderReasons)])
const failure = (e: unknown) => refused(e instanceof Error && known.has(e.message) ? e.message : signInReasons.failed)
const owner = (input: Input) => { if (input.actor._tag !== 'owner') throw refused(signInReasons.owner); if (!input.connectionId) throw refused(signInReasons.connection); return input.connectionId }
export const sessions = (input: Input) => {
  let value = instances.get(input.runtime.log)
  if (!value) { value = new SignInSessions(); value.terminalEnabled = (input.snapshot.client as { webSettings?: { terminalEnabled?: boolean } } | undefined)?.webSettings?.terminalEnabled === true; instances.set(input.runtime.log, value) }
  return value
}
export const setTerminalSwitch = (log: object, enabled: boolean) => {
  let manager = instances.get(log)
  if (!manager) { manager = new SignInSessions(); instances.set(log, manager) }
  if (!enabled) manager.stopTerminals('The project terminal was turned off.')
  else manager.terminalEnabled = true
}
export const signOutTerminals = (log: object, ownerSessionId: string, remaining: ReadonlySet<string>) => { instances.get(log)?.signOut(ownerSessionId, remaining) }
export const closeSignIns = (log: object) => { instances.get(log)?.close(); instances.delete(log) }
const add = (input: Input, p: Record<string, unknown>) => Effect.gen(function* () {
  if (input.actor._tag !== 'owner') return yield* refused(signInReasons.owner)
  const created: string[] = []
  const bodyHash = createHash('sha256').update(JSON.stringify([p.harnessId, p.name, p.folder, p.home])).digest('hex')
  return yield* input.runtime.log.transact(append => Effect.gen(function* () {
    const receipts = yield* input.runtime.sql<{ body_hash: string; result_json: string }>`SELECT body_hash, result_json FROM receipts WHERE caller = 'owner' AND request_id = ${p.requestId as string}`
    const receipt = receipts[0]
    if (receipt) {
      if (receipt.body_hash !== bodyHash) return yield* refused('request id reused')
      return yield* Effect.try({ try: () => {
        const stored = JSON.parse(receipt.result_json).reply
        return { reply: { requestId: p.requestId, account: Schema.decodeUnknownSync(Account)(stored.account) } }
      }, catch: failure })
    }
    const snapshot = yield* input.runtime.views.snapshot
    const id = 'account-' + randomUUID(), harness = p.harnessId as string
    const home = yield* Effect.tryPromise({ try: () => prepareAccountHome({ home: input.runtime.home, harness, id, choice: p.folder as 'existing' | 'separate' | 'advanced', advanced: p.home as string | undefined, accounts: Object.values(snapshot.accounts), created }), catch: failure })
    const account = yield* Effect.try({ try: () => Schema.decodeUnknownSync(Account)({ id, harness, displayName: p.name, enabled: true, ...(home ? { home } : {}),
      ...(harness === 'codex' && home ? { environment: { CODEX_HOME: home } } : {}) }), catch: failure })
    const plan = yield* planAccounts({ ...input, snapshot }, { operation: 'add', account, requestId: p.requestId } as never)
    const written = yield* append({ _tag: 'owner' }, p.requestId as never, bodyHash, plan.events, {}, batch => {
      const event = batch[0]?.event
      return { requestId: p.requestId, account: event?._tag === 'AccountChanged' ? event.account : undefined }
    })
    const event = written[0]?.event
    if (event?._tag !== 'AccountChanged' || !event.account) return yield* refused(signInReasons.failed)
    return { reply: { requestId: p.requestId, account: event.account } }
  })).pipe(
    // Finish the short filesystem/commit boundary before interruption can run cleanup.
    Effect.uninterruptible,
    Effect.onExit(exit => Exit.isSuccess(exit) ? Effect.void : Effect.promise(() => removeCreatedHomes(created))),
    Effect.mapError(e => e._tag === 'CommandRefused' ? e : failure(e))
  )
})
const start = (input: Input, p: Record<string, unknown>) => Effect.gen(function* () {
  const connection = yield* Effect.try({ try: () => owner(input), catch: e => e as ReturnType<typeof refused> })
  const account = input.snapshot.accounts[p.accountId as string]
  if (!account) return yield* refused(signInReasons.missing)
  const programs = input.runtime.harnessPrograms ?? embeddedHarnessPrograms, recipe = programs.recipe(account.harness)
  if (!recipe || !recipe.signInCommand || !Array.isArray(recipe.signInArguments) || recipe.signInArguments.some(a => typeof a !== 'string' || a.includes('\0'))) return yield* refused(signInReasons.command)
  const harness = (yield* readHarnesses(input.runtime)).find(h => h.id === account.harness)
  const installation = yield* Effect.tryPromise({ try: signal => programs.inspect(account.harness, harness, false, signal), catch: () => refused(signInReasons.installed) })
  if (!installation.installed || !installation.path) return yield* refused(signInReasons.installed)
  if (loginRefusal(account.harness, account)) return yield* refused(folderReasons.unsupported)
  let variables: Record<string, string> = {}
  for (const [name, raw] of Object.entries(account.environment ?? {})) {
    if (name.startsWith('LGTM_') || name.startsWith('HERDR_')) continue
    if (raw.startsWith('secret:client.')) return yield* refused(signInReasons.failed)
    if (raw.startsWith('secret:')) { const secret = yield* input.runtime.secrets.resolve(raw); if (!secret) return yield* refused(signInReasons.failed); variables[name] = Redacted.value(secret) } else variables[name] = raw
  }
  const login = harnessLogin(account.harness)
  if (homeConflict(account.harness, account.home, login.homeVariable ? variables[login.homeVariable] : undefined)) return yield* refused(signInReasons.failed)
  if (account.home && login.homeVariable && account.harness !== 'codex') variables[login.homeVariable] = account.home
  if (account.harness === 'codex') variables = yield* withCodexHome({ home: input.runtime.home, stateFolder: input.runtime.stateFolder, accountId: account.id, variables, ...(account.home ? { homePath: account.home } : {}) }).pipe(Effect.provide(NodeServices.layer), Effect.mapError(() => refused(signInReasons.failed)))
  // Sign-in needs no API key or LGTM token. Match the runner's base allowlist.
  const environment = buildBaseEnvironment({ parent: { ...process.env, HOME: input.runtime.home }, accountVariables: variables })
  delete environment.LGTM_TOKEN; delete environment.LGTM_SOCKET
  if (account.harness === 'cursor') environment.NO_OPEN_BROWSER = '1'
  if (input.runtime.credentials) yield* input.runtime.credentials.open(credentialSourceFor({ harnessId: account.harness, home: input.runtime.home, variables: environment })).pipe(Effect.mapError(() => refused(signInReasons.failed)))
  const manager = sessions(input), generation = manager.terminalGeneration, disconnected = () => manager.disconnect(connection)
  const session = yield* Effect.tryPromise({ try: () => manager.start({ ownerSessionId: input.ownerSessionId, allowed: () => manager.terminalGeneration === generation && !input.connectionSignal?.aborted, accountId: account.id, connectionId: connection, executable: installation.path!, args: recipe.signInArguments, cwd: input.runtime.home, environment,
    onEnd: () => input.connectionSignal?.removeEventListener('abort', disconnected),
    check: () => Effect.runPromise(Effect.gen(function* () { const current = (yield* input.runtime.views.snapshot).accounts[account.id]; return { state: current && harness ? yield* accountReadiness(input, harness, current) : 'unavailable' } as const })) }), catch: failure })
  input.connectionSignal?.addEventListener('abort', disconnected, { once: true })
  if (input.connectionSignal?.aborted) manager.disconnect(connection)
  return { reply: { requestId: p.requestId, sessionId: session.id } }
})
const control = (method: 'write' | 'resize' | 'stop') => (input: Input, p: Record<string, unknown>) => Effect.gen(function* () {
  const connection = yield* Effect.try({ try: () => owner(input), catch: e => e as ReturnType<typeof refused> })
  yield* Effect.try({ try: () => { const manager = sessions(input), id = p.sessionId as string; if (method === 'write') manager.write(id, connection, p.text as string, undefined, input.ownerSessionId); else if (method === 'resize') manager.resize(id, connection, p.cols as number, p.rows as number, undefined, input.ownerSessionId); else manager.stop(id, connection, undefined, input.ownerSessionId) }, catch: failure })
  return { reply: { requestId: p.requestId } }
})
export const terminalOutput = (input: Input, p: Record<string, unknown>, kind?: 'project') => Stream.unwrap(Effect.gen(function* () {
  const connection = yield* Effect.try({ try: () => { if (kind && input.actor._tag !== 'owner') throw refused(terminalReasons.owner); return owner(input) }, catch: e => e as ReturnType<typeof refused> })
  // Owner-authorized PTY output is ephemeral and goes byte for byte to its
  // attached owner connection. It must never pass through logging or stored events.
  const manager = sessions(input), id = p.sessionId as string
  let attached: number | undefined
  const disconnected = () => { if (attached !== undefined) manager.detach(id, connection, attached) }
  input.connectionSignal?.addEventListener('abort', disconnected, { once: true })
  return Stream.fromAsyncIterable(manager.output(id, connection, kind, reader => { attached = reader }, input.ownerSessionId), kind ? e => refused(e instanceof Error && Object.values(terminalReasons).includes(e.message as never) ? e.message : terminalReasons.failed) : failure).pipe(
    // Async-generator return can wait behind a pending next(). Detach on
    // interruption too, but only if this subscription acquired the reader.
    Stream.ensuring(Effect.sync(() => { input.connectionSignal?.removeEventListener('abort', disconnected); if (attached !== undefined) manager.detach(id, connection, attached) }))
  )
}))
export const signInReads = {
  'client.harness.account.add': add,
  'client.harness.signin.start': start,
  'client.harness.signin.write': control('write'),
  'client.harness.signin.resize': control('resize'),
  'client.harness.signin.stop': control('stop'),
  'client.harness.signin.subscribe': (input: Input, p: Record<string, unknown>) => terminalOutput(input, p)
}
