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
async function fixture(members: number[], options: { reused?: boolean; absent?: boolean; fork?: boolean } = {}) {
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
  const session = await manager.start({ kind: 'project', accountId: '', connectionId: 'owner', executable: '/bin/sh', args: [], cwd: '/tmp', environment: {}, check: async () => ({ state: 'unavailable' }) })
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
