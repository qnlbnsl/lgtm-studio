import * as fs from 'node:fs'
import { createHmac, scryptSync } from 'node:crypto'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import * as Redacted from 'effect/Redacted'
import { afterEach, expect, it, vi } from 'vitest'
import { makeSession } from '../../src/client/web/session.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
registerRebuildCheck({ noLog: true })
vi.mock('node:fs', async original => { const actual = await original<typeof fs>(); return { ...actual, renameSync: vi.fn(actual.renameSync), fsyncSync: vi.fn(actual.fsyncSync), unlinkSync: vi.fn(actual.unlinkSync) } })
afterEach(async () => {
  const actual = await vi.importActual<typeof fs>('node:fs')
  vi.mocked(fs.renameSync).mockReset().mockImplementation(actual.renameSync)
  vi.mocked(fs.fsyncSync).mockReset().mockImplementation(actual.fsyncSync)
  vi.mocked(fs.unlinkSync).mockReset().mockImplementation(actual.unlinkSync)
})
const password = Redacted.make('synthetic-r2-password')
const fixture = (run: (folder: string) => void) => { const folder = fs.mkdtempSync(join(tmpdir(), 'lgtm-s28-store-')); try { run(folder) } finally { fs.rmSync(folder, { recursive: true, force: true }) } }
const open = (stateFolder: string, value = password) => makeSession(value, () => {}, { stateFolder })
const mint = (session: ReturnType<typeof open>) => { session.rotate(); return session.cookie(false).split(';')[0]! }
it('r2 eight independent cookies survive restart and signing out one preserves the others', () => fixture(folder => {
  const session = open(folder), cookies = Array.from({ length: 8 }, () => mint(session))
  expect(cookies.every(cookie => session.valid(cookie))).toBe(true)
  const ninth = mint(session)
  expect(session.valid(cookies[0])).toBe(false)
  expect(cookies.slice(1).every(cookie => session.valid(cookie))).toBe(true)
  const restored = open(folder)
  expect(restored.valid(ninth)).toBe(true)
  restored.revoke(cookies[1]!)
  expect(restored.valid(cookies[1])).toBe(false)
  expect(restored.valid(ninth)).toBe(true)
  expect(open(folder).valid(cookies[1])).toBe(false)
  expect(open(folder).valid(ninth)).toBe(true)
}))
it('r2 salted memory-hard bindings replace the public-key HMAC oracle', () => fixture(folder => {
  const session = open(folder), cookie = mint(session)
  const saved = fs.readFileSync(join(folder, 'web-session.json'), 'utf8'), snapshot = JSON.parse(saved)
  expect(snapshot.version).toBe(2)
  expect(snapshot.sessions[0].passwordSalt).toMatch(/^[a-f0-9]{32}$/)
  const record = snapshot.sessions[0]
  const key = scryptSync(Redacted.value(password), Buffer.from(record.passwordSalt, 'hex'), 32, { N: 65536, r: 8, p: 1, maxmem: 80 * 1024 * 1024 })
  expect(record.passwordBinding).toBe(createHmac('sha256', key).update('lgtm-web-password-v2\0').update(record.verifier).digest('hex'))
  mint(session)
  const next = JSON.parse(fs.readFileSync(join(folder, 'web-session.json'), 'utf8'))
  expect(next.sessions[1].passwordSalt).not.toBe(record.passwordSalt)
  expect(saved.includes(cookie.split('=')[1]!)).toBe(false)
  expect(saved.includes(Redacted.value(password))).toBe(false)
  open(folder, Redacted.make('changed-r2-password'))
  expect(open(folder).valid(cookie)).toBe(false)
}))
for (const fault of ['rename', 'sync', 'unlink'] as const) it(`r2 a ${fault} failure while revoking cannot revive a rejected cookie after restart`, () => fixture(folder => {
  const session = open(folder), cookie = mint(session)
  const operation = fault === 'rename' ? vi.mocked(fs.renameSync) : fault === 'sync' ? vi.mocked(fs.fsyncSync) : vi.mocked(fs.unlinkSync)
  // Let the file flush succeed, then fail the directory flush after rename.
  if (fault === 'sync') vi.mocked(fs.fsyncSync).mockImplementationOnce(vi.mocked(fs.fsyncSync).getMockImplementation()!)
  operation.mockImplementationOnce(() => { throw new Error('synthetic disk fault') })
  try { session.revoke(cookie) } catch {}
  expect(session.valid(cookie)).toBe(false)
  expect(open(folder).valid(cookie)).toBe(false)
}))

it('r2 persistent rename failure invalidates the existing inode before rejecting memory', () => fixture(folder => {
  const session = open(folder), cookie = mint(session), rename = vi.mocked(fs.renameSync).getMockImplementation()!
  vi.mocked(fs.renameSync).mockImplementation(() => { throw new Error('synthetic persistent rename fault') })
  try {
    expect(() => session.revoke(cookie)).toThrow('Web session storage is unavailable.')
    expect(session.valid(cookie)).toBe(false)
    expect(fs.statSync(join(folder, 'web-session.json')).mode & 0o777).toBe(0)
  } finally { vi.mocked(fs.renameSync).mockImplementation(rename) }
  expect(open(folder).valid(cookie)).toBe(false)
}))

it('r2 sessions expire independently at their own thirty-day boundary', () => fixture(folder => {
  const session = open(folder), start = Date.now(), first = mint(session)
  const clock = vi.spyOn(Date, 'now')
  try {
    clock.mockReturnValue(start + 86400000)
    const second = mint(session)
    clock.mockReturnValue(start + 30 * 86400000 + 1000)
    session.expire()
    expect(session.valid(first)).toBe(false)
    expect(session.valid(second)).toBe(true)
    const restored = open(folder)
    expect(restored.valid(first)).toBe(false)
    expect(restored.valid(second)).toBe(true)
  } finally { clock.mockRestore() }
}))
