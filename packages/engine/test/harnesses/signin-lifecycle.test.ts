import { expect, it, vi } from 'vitest'
import { SignInSessions } from '../../src/harnesses/signinSessions.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
registerRebuildCheck({ noLog: true })
const fake = vi.hoisted(() => ({ onExit: undefined as undefined | (() => void), kill: vi.fn() }))
vi.mock('@lydell/node-pty', () => ({ spawn: () => ({ pid: 987654, onData: () => {}, onExit: (f: () => void) => { fake.onExit = f }, kill: fake.kill }) }))
it('review 11: normal exit never signals a possibly recycled process group', async () => {
  const kill = vi.spyOn(process, 'kill').mockImplementation(() => true)
  const sessions = new SignInSessions()
  try {
    const s = await sessions.start({ accountId: 'a', connectionId: 'one', executable: 'fake', args: [], cwd: '/tmp', environment: {}, check: async () => ({ state: 'signed-out' }) })
    fake.onExit!()
    for await (const item of sessions.output(s.id, 'one')) { if (item._tag === 'ended') break }
    expect(kill).not.toHaveBeenCalled()
    expect(fake.kill).not.toHaveBeenCalled()
  } finally { sessions.close(); kill.mockRestore() }
})
