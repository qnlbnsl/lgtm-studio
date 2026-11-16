import { existsSync, mkdtempSync, readFileSync, readdirSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { request } from 'node:http'
import * as Effect from 'effect/Effect'
import { afterAll, beforeAll, expect, it, vi } from 'vitest'
import { serveDetach } from '../src/serve.ts'

const birth = (pid: number) => { try { const fields = readFileSync(`/proc/${pid}/stat`, 'utf8').split(') ').at(-1)!.split(' '); return fields[0] === 'Z' ? undefined : fields[19] } catch { return undefined } }
const root = mkdtempSync(join(tmpdir(), 's18-file-lease-')), socket = join(root, 'state/engine.sock')
let pid: number, started: string | undefined, record: string
const answers = () => new Promise<boolean>(resolve => {
  const call = request({ socketPath: socket, path: '/', timeout: 500 }, response => { response.resume(); resolve(true) })
  call.once('error', () => resolve(false)); call.once('timeout', () => { call.destroy(); resolve(false) }); call.end()
})
beforeAll(async () => {
  const result = await Effect.runPromise(serveDetach({ environment: { PATH: process.env.PATH, HOME: root, LGTM_HOME: join(root, 'state') } }))
  expect(result.outcome).toBe('started')
  if (result.outcome !== 'started') throw new Error('Expected a new temporary service.')
  pid = result.pid; started = birth(pid)
  const leases = readdirSync(tmpdir()).filter(name => name.startsWith('lgtm-test-owner-'))
  const records = leases.map(name => join(tmpdir(), name, `${pid}-${started}`)).filter(existsSync)
  expect(records).toHaveLength(1); record = records[0]!
})
afterAll(async () => {
  if (started && birth(pid) === started) process.kill(pid, 'SIGTERM')
  if (pid) await vi.waitFor(() => expect(birth(pid)).not.toBe(started), { timeout: 8000 })
  rmSync(root, { recursive: true, force: true })
})
it.runIf(process.platform === 'linux')('serveDetach inside the real test runner records its service under the setup lease', async () => {
  expect(existsSync(record)).toBe(true)
  expect(await answers()).toBe(true)
})
it.runIf(process.platform === 'linux')('a beforeAll service retains its lease and answers in the second test', async () => {
  expect(existsSync(join(record, '..', 'active'))).toBe(true)
  expect(birth(pid)).toBe(started)
  expect(await answers()).toBe(true)
})
