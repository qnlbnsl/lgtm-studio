import { expect, it, vi } from 'vitest'
import * as Pty from '@lydell/node-pty'
import { SignInSessions } from '../../src/harnesses/signinSessions.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
registerRebuildCheck({ noLog: true })
vi.mock('@lydell/node-pty', async original => { const actual = await original<typeof import('@lydell/node-pty')>(); return { ...actual, spawn: vi.fn(actual.spawn) } })
it('a slow reader pauses a real sh and receives its complete tail before exit', async () => {
  const manager = new SignInSessions(); manager.terminalEnabled = true
  const s = await manager.start({ kind: 'project', accountId: '', connectionId: 'owner', executable: '/bin/sh', args: [], cwd: '/tmp', environment: { PATH: '/usr/bin:/bin' }, check: async () => ({ state: 'unavailable' }) })
  const child = vi.mocked(Pty.spawn).mock.results.at(-1)!.value as Pty.IPty, paused = vi.spyOn(child, 'pause')
  try {
    manager.write(s.id, 'owner', 'seq 1 200000; echo END-MARK; exit\r', 'project')
    await new Promise(resolve => setTimeout(resolve, 100))
    let text = '', reason = '', chunks = 0
    for await (const item of manager.output(s.id, 'owner', 'project')) {
      if (item._tag === 'output') { text += item.text; if (++chunks % 32 === 0) await new Promise(resolve => setTimeout(resolve, 20)) }
      else reason = item.reason
    }
    expect(paused).toHaveBeenCalled()
    expect(text).toContain('200000\r\nEND-MARK\r\n')
    expect(reason).toBe('The shell exited.')
  } finally { manager.close(); paused.mockRestore() }
}, 45000)
