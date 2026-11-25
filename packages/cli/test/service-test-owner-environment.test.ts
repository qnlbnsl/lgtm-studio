import { spawn, fork } from 'node:child_process'
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { fileURLToPath } from 'node:url'
import { afterAll, expect, it } from 'vitest'

const root = mkdtempSync(join(tmpdir(), 's18-owner-environment-')), probe = join(root, 'probe.mjs')
writeFileSync(probe, "process.stdout.write(JSON.stringify(process.env));\n")
afterAll(() => rmSync(root, { recursive: true, force: true }))
for (const method of ['spawn', 'fork'] as const) {
  for (const socket of [false, true]) it(`${method} preserves an ordinary child's explicit environment with socket ${socket}`, async () => {
    const env = { PATH: process.env.PATH!, ACCOUNT_VALUE: 'account-value', NODE_OPTIONS: '--no-warnings', ...(socket ? { LGTM_SOCKET: '/tmp/owned-test-socket' } : {}) }
    const child = method === 'spawn' ? spawn(process.execPath, [probe], { env, stdio: ['ignore', 'pipe', 'pipe'] })
      : fork(probe, [], { env, execArgv: [], stdio: ['ignore', 'pipe', 'pipe', 'ipc'] })
    let text = ''
    child.stdout!.on('data', bytes => { text += bytes })
    try {
      const code = await new Promise(resolve => child.once('close', resolve))
      expect(code).toBe(0); expect(JSON.parse(text)).toEqual(env)
    } finally { if (child.exitCode === null && child.signalCode === null) child.kill('SIGTERM') }
  })
}
it('does not treat a service path passed as ordinary script data as the child entry', async () => {
  const env = { PATH: process.env.PATH! }
  const child = spawn(process.execPath, [probe, fileURLToPath(new URL('../../engine/src/service.ts', import.meta.url))], { env, stdio: ['ignore', 'pipe', 'pipe'] })
  let text = ''; child.stdout.on('data', bytes => { text += bytes })
  try {
    const code = await new Promise(resolve => child.once('close', resolve))
    expect(code).toBe(0); expect(JSON.parse(text)).toEqual(env)
  } finally { if (child.exitCode === null && child.signalCode === null) child.kill('SIGTERM') }
})
