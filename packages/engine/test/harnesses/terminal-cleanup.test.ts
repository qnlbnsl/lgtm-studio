import * as fs from 'node:fs'
import { afterEach, expect, it, vi } from 'vitest'
import * as Pty from '@lydell/node-pty'
import { SignInSessions } from '../../src/harnesses/signinSessions.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
registerRebuildCheck({ noLog: true })
vi.mock('@lydell/node-pty', () => ({ spawn: vi.fn() }))
vi.mock('node:fs', async original => { const actual = await original<typeof import('node:fs')>(); return { ...actual, readdirSync: vi.fn(actual.readdirSync), readFileSync: vi.fn(actual.readFileSync), existsSync: vi.fn(actual.existsSync) } })
afterEach(() => vi.restoreAllMocks())
const leader = 2000000000
async function fixture(members: number[], options: { reused?: boolean; absent?: boolean; fork?: boolean; web?: string } = {}) {
  vi.clearAllMocks()
  const actual = await vi.importActual<typeof import('node:fs')>('node:fs')
  let exited!: () => void, data!: (text: string) => void
  const child = { pid: leader, _boundClose: false, onData: (f: typeof data) => { data = f }, onExit: (f: typeof exited) => { exited = f }, pause: vi.fn(), resume: vi.fn(), kill: vi.fn(), write: vi.fn(), resize: vi.fn() }
  vi.mocked(Pty.spawn).mockReturnValue(child as unknown as Pty.IPty)
  const scans = vi.mocked(fs.readdirSync).mockImplementation(((path: string) => { if (path !== '/proc') return actual.readdirSync(path); if (options.absent) throw new Error('no proc'); return members.map(String) }) as never)
  vi.mocked(fs.existsSync).mockImplementation(path => String(path).startsWith('/proc/') ? String(path) === '/proc/' + leader && options.reused === true : actual.existsSync(path))
  vi.mocked(fs.readFileSync).mockImplementation(((path: string, encoding: any) => {
    if (!String(path).startsWith('/proc/')) return actual.readFileSync(path, encoding)
    const pid = Number(String(path).split('/')[2]); if (!members.includes(pid)) throw new Error('gone')
    return `${pid} (name with ) spaces) S 1 2 ${leader} 0`
  }) as never)
  const signals: number[] = []
  vi.spyOn(process, 'kill').mockImplementation((pid => {
    signals.push(pid); members.splice(members.indexOf(pid), 1)
    if (options.fork) members.push(pid + 1)
    return true
  }) as typeof process.kill)
  const manager = new SignInSessions(); manager.terminalEnabled = true
  const session = await manager.start({ ownerSessionId: options.web, kind: 'project', accountId: '', connectionId: 'owner', executable: '/bin/sh', args: [], cwd: '/tmp', environment: {}, check: async () => ({ state: 'unavailable' }) })
  return { manager, session, child, signals, scans, exit: () => { child._boundClose = true; exited() }, data: (text: string) => data(text) }
}
it('a natural exit with an empty session scans once and signals nothing', async () => {
  const f = await fixture([])
  try { f.exit(); expect(f.scans).toHaveBeenCalledTimes(1); expect(f.signals).toEqual([]) } finally { f.manager.close() }
})
it('cleanup catches forks between scanning and signalling and stops after five passes', async () => {
  const f = await fixture([leader + 1], { fork: true })
  try { f.exit(); expect(f.signals).toEqual([1, 2, 3, 4, 5].map(n => leader + n)); expect(f.scans).toHaveBeenCalledTimes(5) } finally { f.manager.close() }
})
it('a reused former leader identifier prevents all post-exit signalling', async () => {
  const f = await fixture([leader + 1], { reused: true })
  try { f.exit(); expect(f.signals).toEqual([]); expect(f.scans).not.toHaveBeenCalled() } finally { f.manager.close() }
})
it('a host without proc does no descendant signalling', async () => {
  const f = await fixture([], { absent: true })
  try { f.exit(); expect(f.signals).toEqual([]); expect(f.scans).toHaveBeenCalledTimes(1) } finally { f.manager.close() }
})
it('a paused PTY resumes on native exit before its public exit closes the socket', async () => {
  const f = await fixture([])
  try {
    f.data('x'.repeat(128 * 1024)); expect(f.child.pause).toHaveBeenCalledOnce()
    // node-pty sets this flag when waitpid fires, then destroys an unread
    // socket after 200 ms. Its public onExit arrives only after that close.
    f.child._boundClose = true
    await vi.waitFor(() => expect(f.child.resume).toHaveBeenCalledOnce(), { timeout: 150, interval: 5 })
    f.data('END-MARK\r\n'); f.exit()
    let text = ''
    for await (const item of f.manager.output(f.session.id, 'owner', 'project')) if (item._tag === 'output') text += item.text
    expect(text).toContain('END-MARK\r\n')
  } finally { f.exit(); f.manager.close() }
})
it('s28 a stale reader cleanup cannot detach its replacement on the same connection', async () => {
  const f = await fixture([])
  let reader: number | undefined
  const original = f.manager.output(f.session.id, 'owner', 'project', value => { reader = value })
  try {
    const first = original.next(); f.data('FIRST'); await first
    f.manager.detach(f.session.id, 'owner', reader)
    const replacement = f.manager.output(f.session.id, 'owner', 'project')
    expect((await replacement.next()).value).toMatchObject({ _tag: 'output', text: 'FIRST' })
    f.manager.detach(f.session.id, 'owner', reader)
    await original.return(undefined)
    const next = replacement.next(); f.data('SECOND')
    expect((await next).value).toMatchObject({ _tag: 'output', text: 'SECOND' })
    await replacement.return(undefined)
  } finally { f.exit(); f.manager.close() }
})

it('r2 a half-open web reader is replaced by another owner session and both can type', async () => {
  const f = await fixture([], { web: 'desktop' })
  const old = f.manager.output(f.session.id, 'owner', 'project', undefined, 'desktop')
  try {
    const first = old.next(); f.data('BEFORE'); expect((await first).value).toMatchObject({ text: 'BEFORE' })
    const waiting = old.next()
    const next = f.manager.output(f.session.id, 'phone-socket', 'project', undefined, 'phone')
    expect((await next.next()).value).toMatchObject({ _tag: 'output', text: 'BEFORE' })
    expect((await waiting).value).toMatchObject({ _tag: 'ended', reason: expect.stringContaining('another') })
    expect((await old.next()).done).toBe(true)
    f.manager.write(f.session.id, 'owner', 'desktop keys', 'project', 'desktop')
    f.manager.write(f.session.id, 'phone-socket', 'phone keys', 'project', 'phone')
    expect(f.child.write.mock.calls).toEqual([['desktop keys'], ['phone keys']])
    const live = next.next(); f.data('AFTER'); expect((await live).value).toMatchObject({ text: 'AFTER' })
    await next.return(undefined)
  } finally { f.exit(); f.manager.close() }
})
it('r2 a web logout does not touch local terminals', async () => {
  const f = await fixture([])
  try { f.manager.signOut('web'); expect(f.child.kill).not.toHaveBeenCalled(); f.manager.write(f.session.id, 'owner', 'still local', 'project') }
  finally { f.exit(); f.manager.close() }
})
it('r2 an attached phone inherits a desktop shell on desktop logout and last-session logout closes it', async () => {
  const f = await fixture([], { web: 'desktop' })
  try {
    const phone = f.manager.output(f.session.id, 'phone-socket', 'project', undefined, 'phone')
    const first = phone.next(); f.data('PHONE'); await first
    f.manager.signOut('desktop', new Set(['phone']))
    expect(f.child.kill).not.toHaveBeenCalled()
    f.manager.write(f.session.id, 'phone-socket', 'still here', 'project', 'phone')
    f.manager.signOut('phone', new Set())
    expect(f.child.kill).toHaveBeenCalledOnce()
  } finally { f.exit(); f.manager.close() }
})
it('r2 a local socket cannot reach a live web terminal even with its exact identifier', async () => {
  const f = await fixture([], { web: 'desktop' })
  try {
    expect(() => f.manager.write(f.session.id, 'local-socket', 'forbidden', 'project')).toThrow()
    expect(f.manager.list('phone')).toHaveLength(1)
    expect(f.manager.list(undefined, 'local-socket')).toEqual([])
  } finally { f.exit(); f.manager.close() }
})

it('r3 an empty escape-safe replay resumes a PTY paused by the displaced reader', async () => {
  const f = await fixture([], { web: 'desktop' })
  const old = f.manager.output(f.session.id, 'owner', 'project', undefined, 'desktop')
  const next = f.manager.output(f.session.id, 'phone', 'project', undefined, 'phone')
  try {
    const first = old.next(); f.data('FIRST'); await first
    f.data('\x1b]' + 'x'.repeat(256 * 1024))
    expect(f.child.pause).toHaveBeenCalledOnce()
    const waiting = next.next()
    expect(f.child.resume).toHaveBeenCalledOnce()
    f.data('\x07R3_RESUMED')
    expect((await waiting).value).toMatchObject({ _tag: 'output', text: expect.stringContaining('R3_RESUMED') })
    await next.return(undefined); await old.return(undefined)
  } finally { f.exit(); f.manager.close() }
})
it('r3 a detached shell pauses after its bounded history turns over and resumes on attach', async () => {
  const f = await fixture([], { web: 'desktop' })
  try {
    f.manager.disconnect('owner')
    for (let n = 0; n < 16; n++) f.data('x'.repeat(4096))
    expect(f.child.pause).not.toHaveBeenCalled()
    f.data('TURNED_OVER')
    expect(f.child.pause).toHaveBeenCalledOnce()
    const phone = f.manager.output(f.session.id, 'phone', 'project', undefined, 'phone')
    expect((await phone.next()).value).toMatchObject({ text: expect.stringContaining('TURNED_OVER') })
    expect(f.child.resume).toHaveBeenCalledOnce()
    await phone.return(undefined)
  } finally { f.exit(); f.manager.close() }
})
it('r3 a typing-only or reloading browser preserves the shell when its creator signs out', async () => {
  const f = await fixture([], { web: 'desktop' })
  try {
    f.manager.disconnect('owner')
    f.manager.write(f.session.id, 'phone', 'typing without reading', 'project', 'phone')
    f.manager.signOut('desktop', new Set(['phone']))
    expect(f.child.kill).not.toHaveBeenCalled()
    expect(f.manager.list('phone')).toHaveLength(1)
    f.manager.write(f.session.id, 'new-phone-connection', 'after reload', 'project', 'phone')
    f.manager.signOut('phone', new Set())
    expect(f.child.kill).toHaveBeenCalledOnce()
  } finally { f.exit(); f.manager.close() }
})
for (const operation of ['get', 'list', 'start'] as const) it('r3 revoked owner cannot ' + operation + ' while another owner keeps the shell live', async () => {
  const f = await fixture([], { web: 'desktop' })
  try {
    f.manager.signOut('stale', new Set(['desktop']))
    expect(f.manager.list('desktop')).toHaveLength(1)
    f.manager.write(f.session.id, 'owner', 'still live', 'project', 'desktop')
    if (operation === 'get') expect(() => f.manager.write(f.session.id, 'stale-socket', 'forbidden', 'project', 'stale')).toThrow()
    if (operation === 'list') expect(f.manager.list('stale')).toEqual([])
    if (operation === 'start') {
      await expect(f.manager.start({ ownerSessionId: 'stale', kind: 'project', accountId: '', connectionId: 'stale', executable: '/bin/sh', args: [], cwd: '/tmp', environment: {}, check: async () => ({ state: 'unavailable' }) })).rejects.toThrow()
      expect(Pty.spawn).toHaveBeenCalledOnce()
    }
  } finally { f.exit(); f.manager.close() }
})

it('r3 native exit resumes a detached paused shell to drain its finite tail', async () => {
  const f = await fixture([], { web: 'desktop' })
  try {
    f.manager.disconnect('owner'); f.data('x'.repeat(128 * 1024))
    expect(f.child.pause).toHaveBeenCalledOnce()
    f.child._boundClose = true
    await vi.waitFor(() => expect(f.child.resume).toHaveBeenCalledOnce(), { timeout: 150, interval: 5 })
    f.data('FINAL_TAIL'); f.exit()
    expect(f.child.pause).toHaveBeenCalledOnce()
  } finally { f.exit(); f.manager.close() }
})
