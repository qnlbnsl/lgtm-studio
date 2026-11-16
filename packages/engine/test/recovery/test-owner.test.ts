import { spawn, type ChildProcess } from 'node:child_process'
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, it, vi } from 'vitest'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
// The fixture owns external service processes, not an in-process event log.
registerRebuildCheck({ noLog: true })
const service = fileURLToPath(new URL('../../src/service.ts', import.meta.url))
const ownership = fileURLToPath(new URL('../../../../scripts/service-test-owner.mjs', import.meta.url))
const birth = (pid: number) => { try { const fields = readFileSync(`/proc/${pid}/stat`, 'utf8').split(') ').at(-1)!.split(' '); return fields[0] === 'Z' ? undefined : fields[19] } catch { return undefined } }
const alive = (pid: number) => birth(pid) !== undefined
for (const end of ['teardown', 'SIGKILL', 'blocked preload'] as const) it.runIf(process.platform === 'linux')(`a service test owner ends its detached service on ${end}`, async () => {
  const root = mkdtempSync('/tmp/s18-owner-'), worker = join(root, 'worker.mjs'), record = join(root, 'pid'), hook = join(root, 'blocked.mjs')
  writeFileSync(hook, `import { writeFileSync } from 'node:fs'; writeFileSync(${JSON.stringify(record + '-blocked')}, 'yes'); await new Promise(() => { setInterval(() => {}, 1000); });`)
  writeFileSync(worker, `import { installServiceTestOwner } from ${JSON.stringify(ownership)}; import { spawn } from 'node:child_process'; import { writeFileSync } from 'node:fs';
const owner = installServiceTestOwner();
const child = spawn(process.execPath, [${JSON.stringify(service)}], { detached: true, stdio: 'ignore', env: { PATH: process.env.PATH, HOME: ${JSON.stringify(root)}, LGTM_HOME: ${JSON.stringify(join(root, 'state'))}, ...(${end === 'blocked preload'} ? { NODE_OPTIONS: ${JSON.stringify('--import=' + hook)} } : {}) } });
writeFileSync(${JSON.stringify(record)}, String(child.pid)); child.unref();
process.on('message', async () => { await owner.close(); process.send('closed'); }); setInterval(() => {}, 1000);
`)
  let child: ChildProcess | undefined, pid: number | undefined, started: string | undefined
  try {
    child = spawn(process.execPath, [worker], { stdio: ['ignore', 'ignore', 'pipe', 'ipc'] })
    await vi.waitFor(() => { pid = Number(readFileSync(record, 'utf8')); expect(alive(pid)).toBe(true); started = birth(pid) })
    await vi.waitFor(() => expect(existsSync(end === 'blocked preload' ? record + '-blocked' : join(root, 'state/engine.sock'))).toBe(true), { timeout: 10000 })
    if (end !== 'teardown') child.kill('SIGKILL')
    else { const closed = new Promise(resolve => child!.once('message', resolve)); child.send('close'); await closed }
    await vi.waitFor(() => expect(alive(pid!)).toBe(false), { timeout: 8000 })
  } finally {
    child?.kill('SIGKILL')
    if (pid && started && birth(pid) === started) process.kill(pid, 'SIGKILL')
    rmSync(root, { recursive: true, force: true })
  }
}, 20000)
