import { afterEach, expect, it } from 'vitest'
import { chmodSync, existsSync, mkdtempSync, mkdirSync, readFileSync, rmSync, statSync, symlinkSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { prepareAccountHome } from '../../src/accounts/loginHome.ts'
import { SignInSessions } from '../../src/harnesses/signinSessions.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
registerRebuildCheck({ noLog: true })
let roots: string[] = []
const root = () => { const p = mkdtempSync(join(tmpdir(), 'lgtm-signin-')); roots.push(p); return p }
afterEach(() => { for (const p of roots) rmSync(p, { recursive: true, force: true }); roots = [] })
it('creates private generated homes independently of hostile display names and refuses an occupied managed home', async () => {
  const home = root()
  const folder = await prepareAccountHome({ home, harness: 'claude', id: 'account-123', choice: 'separate', accounts: [] })
  expect(folder).toBe(join(home, '.lgtm/accounts/claude/account-123'))
  expect(statSync(folder!).mode & 0o777).toBe(0o700)
  await expect(prepareAccountHome({ home, harness: 'claude', id: '../escape', choice: 'separate', accounts: [] })).rejects.toThrow('The account identifier is invalid.')
  await expect(prepareAccountHome({ home, harness: 'claude', id: 'account-123', choice: 'separate', accounts: [] })).rejects.toThrow('The separate login folder already exists.')
})
it('refuses shared homes, symlinks in parent paths, and a folder owned by another user', async () => {
  const home = root(), elsewhere = root()
  symlinkSync(elsewhere, join(home, '.lgtm'))
  await expect(prepareAccountHome({ home, harness: 'claude', id: 'a', choice: 'separate', accounts: [] })).rejects.toThrow('The login folder must be owned by you and have no symbolic links.')
  rmSync(join(home, '.lgtm'))
  await expect(prepareAccountHome({ home, harness: 'claude', id: 'a', choice: 'existing', accounts: [{ id: 'other', harness: 'claude' }] })).rejects.toThrow('Two accounts cannot share one login folder.')
  const foreign = join(home, 'foreign'); mkdirSync(foreign)
  // Inject only the owner lookup; the real directory walk and identity checks still run.
  await expect(prepareAccountHome({ home, harness: 'claude', id: 'a', choice: 'advanced', advanced: foreign, accounts: [], uid: () => -1 })).rejects.toThrow('The login folder must be owned by you and have no symbolic links.')
})
it('runs a strict PTY fake, accepts only its connection, writes only the private credential, and checks on exit', async () => {
  const home = root(), folder = join(home, 'private'); mkdirSync(folder)
  const script = join(home, 'signin.cjs')
  writeFileSync(script, `if(!process.stdin.isTTY||!process.stdout.isTTY)process.exit(88);process.stdout.write('SIGNIN_MARKER_CODE https://example.test/login\\r\\nPress y');process.stdin.setRawMode(true);process.stdin.once('data',()=>{require('node:fs').writeFileSync(require('node:path').join(process.env.CLAUDE_CONFIG_DIR,'.credentials.json'),'credential-marker');process.exit(0)});`)
  const sessions = new SignInSessions({ graceMs: 30, limitMs: 3000 })
  const s = await sessions.start({ accountId: 'a', connectionId: 'owner-one', executable: process.execPath, args: [script], cwd: home, environment: { HOME: home, CLAUDE_CONFIG_DIR: folder }, check: async () => ({ state: 'local-credential' }) })
  await expect(sessions.output(s.id, 'owner-two').next()).rejects.toThrow('This sign-in belongs to another connection.')
  expect(() => sessions.resize(s.id, 'owner-two', 80, 24)).toThrow('This sign-in belongs to another connection.')
  expect(() => sessions.stop(s.id, 'owner-two')).toThrow('This sign-in belongs to another connection.')
  expect(() => sessions.write(s.id, 'owner-two', 'y')).toThrow('This sign-in belongs to another connection.')
  await expect(sessions.start({ accountId: 'a', connectionId: 'owner-one', executable: process.execPath, args: [script], cwd: home, environment: {}, check: async () => ({ state: 'signed-out' }) })).rejects.toThrow('A sign-in is already running for this account.')
  let text = '', result: unknown
  for await (const item of sessions.output(s.id, 'owner-one')) {
    if (item._tag === 'output') { text += item.text; if (text.includes('Press y')) sessions.write(s.id, 'owner-one', 'y') }
    if (item._tag === 'ended') result = item.check
  }
  expect(text.includes('SIGNIN_MARKER_CODE')).toBe(true)
  expect(result).toEqual({ state: 'local-credential' })
  expect(readFileSync(join(folder, '.credentials.json'), 'utf8')).toBe('credential-marker')
  expect(() => readFileSync(join(home, '.claude/.credentials.json'))).toThrow()
  sessions.close()
})
it('stops the process group on timeout, owner stop and disconnection after the grace period', async () => {
  for (const mode of ['limit', 'stop', 'disconnect']) {
    const home = root(), pidfile = join(home, 'child'), script = join(home, 'wait.cjs')
    writeFileSync(script, `const c=require('node:child_process').spawn(process.execPath,['-e','setInterval(()=>{},1000)'],{stdio:'ignore'});require('node:fs').writeFileSync(${JSON.stringify(pidfile)},String(c.pid));process.stdout.write('ready');setInterval(()=>{},1000)`)
    const sessions = new SignInSessions({ graceMs: 30, limitMs: 200 })
    const s = await sessions.start({ accountId: 'a', connectionId: 'one', executable: process.execPath, args: [script], cwd: home, environment: {}, check: async () => ({ state: 'signed-out' }) })
    for await (const item of sessions.output(s.id, 'one')) {
      if (item._tag === 'output' && item.text.includes('ready')) { if (mode === 'stop') sessions.stop(s.id, 'one'); if (mode === 'disconnect') sessions.disconnect('one') }
    }
    let alive = false
    try { process.kill(s.pid, 0); alive = true } catch {}
    expect(alive).toBe(false)
    const descendant = Number(readFileSync(pidfile, 'utf8'))
    let descendantRunning = false
    try { descendantRunning = !readFileSync(`/proc/${descendant}/stat`, 'utf8').split(') ')[1]!.startsWith('Z') } catch {}
    expect(descendantRunning).toBe(false)
    sessions.close()
  }
})
it('redacts split registered tokens while releasing ordinary terminal prompts immediately', async () => {
  const { makeRedactor } = await import('../../src/identity/redactor.ts')
  const Redacted = await import('effect/Redacted')
  const redactor = makeRedactor({ secrets: () => [Redacted.make('known-secret-token')], tokenHashes: () => new Set() })
  expect(redactor.redactChunk('terminal', 'Press y: ', false, true)).toBe('Press y: ')
  const first = redactor.redactChunk('terminal', 'known-secret-', false, true)
  const last = redactor.redactChunk('terminal', 'token done', true, true)
  expect(first + last).toBe('[REDACTED] done')
})
it('redacts registered credentials at every split in the responsive terminal stream', async () => {
  const { makeRedactor } = await import('../../src/identity/redactor.ts')
  const { mintCredential, sha256Hex } = await import('../../src/identity/credential.ts')
  const Redacted = await import('effect/Redacted')
  const token = Redacted.value(mintCredential('shp_a_').token)
  const encoded = [token, Buffer.from(token).toString('hex').toUpperCase(), Buffer.from(token).toString('base64'), Buffer.from('x' + token + 'zz').toString('base64'), Buffer.from('xx' + token + 'z').toString('base64url'), [...token].map((ch, i) => i % 2 ? ch : '%' + ch.charCodeAt(0).toString(16).toUpperCase()).join('')]
  for (const hashOnly of [false, true]) for (const form of encoded) for (let at = 1; at < form.length; at++) {
    const r = makeRedactor({ secrets: () => hashOnly ? [] : [Redacted.make(token)], tokenHashes: () => new Set(hashOnly ? [sha256Hex(token)] : []) })
    const actual = r.redactChunk('s', form.slice(0, at), false, true) + r.redactChunk('s', form.slice(at), true, true)
    expect(actual === r.redactText(form), `${hashOnly ? 'hash' : 'exact'} form ${encoded.indexOf(form)} split ${at}`).toBe(true)
  }
})

it('review 3: existing sign-in stores no home and creates no default folder', async () => {
  const home = root()
  expect(await prepareAccountHome({ home, harness: 'claude', id: 'a', choice: 'existing', accounts: [] })).toBeUndefined()
  expect(existsSync(join(home, '.claude'))).toBe(false)
})
it('review 2: custom harnesses can keep multiple default accounts while Cursor cannot', async () => {
  const home = root()
  expect(await prepareAccountHome({ home, harness: 'first', id: 'b', choice: 'existing', accounts: [{ id: 'a', harness: 'first' }] })).toBeUndefined()
  await expect(prepareAccountHome({ home, harness: 'cursor', id: 'b', choice: 'existing', accounts: [{ id: 'a', harness: 'cursor' }] })).rejects.toThrow('Separate sign-in folders are not supported')
})
it('review 8: advanced folders refuse writable permissions and missing parents', async () => {
  const home = root(), folder = join(home, 'writable')
  mkdirSync(folder); chmodSync(folder, 0o770)
  await expect(prepareAccountHome({ home, harness: 'claude', id: 'a', choice: 'advanced', advanced: folder, accounts: [] })).rejects.toThrow('The login folder must be owned by you')
  chmodSync(folder, 0o707)
  await expect(prepareAccountHome({ home, harness: 'claude', id: 'a', choice: 'advanced', advanced: folder, accounts: [] })).rejects.toThrow('The login folder must be owned by you')
  await expect(prepareAccountHome({ home, harness: 'claude', id: 'a', choice: 'advanced', advanced: join(home, 'missing', 'child'), accounts: [] })).rejects.toThrow('The login folder must be owned by you')
  expect(existsSync(join(home, 'missing'))).toBe(false)
  await expect(prepareAccountHome({ home, harness: 'claude', id: 'a', choice: 'advanced', advanced: join(home, 'foreign-parent'), accounts: [], uid: () => -1 })).rejects.toThrow('The login folder must be owned by you')
  expect(await prepareAccountHome({ home, harness: 'claude', id: 'a', choice: 'advanced', advanced: join(home, 'allowed'), accounts: [] })).toBe(join(home, 'allowed'))
})
it('review 5: refuses a real symbolic link at the final advanced component', async () => {
  const home = root(), target = root(), link = join(home, 'link')
  symlinkSync(target, link)
  await expect(prepareAccountHome({ home, harness: 'claude', id: 'a', choice: 'advanced', advanced: link, accounts: [] })).rejects.toThrow('The login folder must be owned by you')
})
