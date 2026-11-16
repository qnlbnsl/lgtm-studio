import { randomUUID } from 'node:crypto'
import { createServer } from 'node:net'
import * as Effect from 'effect/Effect'
import { connect } from '@lgtm/client'
import { spawn, type ChildProcess } from 'node:child_process'
import { chmodSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { request } from 'node:http'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { afterEach, expect, it, vi } from 'vitest'

const entry = fileURLToPath(new URL('../src/bin.ts', import.meta.url))
const roots: string[] = [], children: ChildProcess[] = []
const root = () => {
  const folder = mkdtempSync('/tmp/s18-foreground-'); roots.push(folder)
  mkdirSync(join(folder, 'home'), { mode: 0o700 })
  return { folder, state: join(folder, 'state'), socket: join(folder, 'state/engine.sock') }
}
type Root = ReturnType<typeof root>
const environment = (r: Root) => ({ PATH: process.env.PATH, HOME: join(r.folder, 'home'), LGTM_HOME: r.state,
  LGTM_TOKEN: 's18-token-never-log', S18_SECRET: 's18-secret-never-log' })
function launch(r: Root, args = ['serve'], extra: Record<string, string> = {}) {
  const child = spawn(process.execPath, [entry, ...args], { env: { ...environment(r), ...extra }, stdio: ['ignore', 'pipe', 'pipe'] })
  children.push(child)
  let output = '', error = ''
  child.stdout!.on('data', bytes => { output += bytes.toString() })
  child.stderr!.on('data', bytes => { error += bytes.toString() })
  const closed = new Promise<number | null>(resolve => child.once('close', resolve))
  return { child, closed, output: () => output, error: () => error }
}
const answers = (socketPath: string) => new Promise<boolean>(resolve => {
  const call = request({ socketPath, path: '/', timeout: 500 }, response => { response.resume(); resolve(response.statusCode !== 503) })
  call.once('error', () => resolve(false)); call.once('timeout', () => { call.destroy(); resolve(false) }); call.end()
})
async function ready(r: Root, p: ReturnType<typeof launch>) {
  const deadline = Date.now() + 30_000
  while (Date.now() < deadline) {
    expect(p.child.exitCode, p.error()).toBe(null)
    if (await answers(r.socket)) return
    await new Promise(resolve => setTimeout(resolve, 25))
  }
  throw new Error('The temporary foreground service did not answer.')
}
afterEach(async () => {
  for (const child of children.splice(0)) if (child.exitCode === null && child.signalCode === null) {
    const closed = new Promise(resolve => child.once('close', resolve)); child.kill('SIGTERM'); await closed
  }
  roots.splice(0).forEach(folder => rmSync(folder, { recursive: true, force: true }))
})

for (const signal of ['SIGINT', 'SIGTERM', 'SIGHUP'] as const) it(`foreground serve logs safely, releases its lock and exits zero on ${signal}`, async () => {
  const r = root()
  const p = launch(r)
  await ready(r, p)
  await vi.waitFor(() => expect(p.output()).toContain('"message":'))
  for (const secret of ['s18-password-never-log', 's18-token-never-log', 's18-secret-never-log']) expect(p.output() + p.error()).not.toContain(secret)
  p.child.kill(signal)
  expect(await p.closed).toBe(0)
  expect(await answers(r.socket)).toBe(false)
  const again = launch(r)
  await ready(r, again)
  again.child.kill('SIGTERM')
  expect(await again.closed).toBe(0)
}, 30_000)

it('foreground serve refuses the same state folder while preserving the first service', async () => {
  const r = root(), first = launch(r)
  await ready(r, first)
  const second = launch(r)
  expect(await second.closed).toBe(1)
  expect(second.output() + second.error()).toMatch(/already running/i)
  expect(await answers(r.socket)).toBe(true)
  expect(first.child.exitCode).toBe(null)
}, 30_000)

it('foreground startup prints only the existing fixed failure category', async () => {
  const r = root()
  mkdirSync(r.state, { mode: 0o755 }); chmodSync(r.state, 0o755)
  const p = launch(r)
  expect(await p.closed).toBe(1)
  expect(p.error()).toContain('state folder mode is not 0700')
  expect(p.error()).not.toContain(r.folder)
  for (const secret of ['s18-password-never-log', 's18-token-never-log', 's18-secret-never-log']) expect(p.output() + p.error()).not.toContain(secret)
}, 30_000)

it('foreground serves the authenticated web API on a temporary port without exposing the password', async () => {
  const r = root(), headless = launch(r)
  await ready(r, headless)
  const port = await new Promise<number>(resolve => {
    const server = createServer()
    server.listen(0, '127.0.0.1', () => {
      const port = (server.address() as import('node:net').AddressInfo).port
      server.close(() => resolve(port))
    })
  })
  await Effect.runPromise(Effect.scoped(Effect.gen(function* () {
    const client = yield* connect({ environment: { ...environment(r), LGTM_TOKEN: undefined } })
    yield* client['client.web.settings.set']({ requestId: randomUUID() as never, settings: { enabled: true, port, heartbeatSeconds: 25, proxyMode: false } })
  })))
  headless.child.kill('SIGTERM')
  expect(await headless.closed).toBe(0)
  const p = launch(r, ['serve'], { LGTM_WEB_PASSWORD: 's18-password-never-log' })
  await ready(r, p)
  const response = await fetch(`http://127.0.0.1:${port}/api/login`, { method: 'POST', headers: { 'Content-Type': 'application/json', Origin: `http://127.0.0.1:${port}` }, body: JSON.stringify({ password: 's18-password-never-log' }) })
  expect(response.status).toBe(204)
  p.child.kill('SIGTERM')
  expect(await p.closed).toBe(0)
  for (const secret of ['s18-password-never-log', 's18-token-never-log', 's18-secret-never-log']) expect(p.output() + p.error()).not.toContain(secret)
}, 30_000)

it('two foreground launches race for one lifetime lock', async () => {
  const r = root(), first = launch(r), second = launch(r)
  const loser = await Promise.race([first.closed.then(code => ({ code, process: first })), second.closed.then(code => ({ code, process: second }))])
  expect(loser.code).toBe(1)
  expect(loser.process.output() + loser.process.error()).toMatch(/already running/i)
  expect((loser.process.output() + loser.process.error()).trim().split('\n')).toEqual(['lgtm: The service is already running for this state folder.'])
  await ready(r, loser.process === first ? second : first)
}, 30_000)

it('a signal does not hide an unclean child shutdown', async () => {
  const r = root(), hook = join(r.folder, 'unclean-shutdown.mjs')
  writeFileSync(hook, "if (process.argv[1]?.endsWith('/engine/src/service.ts')) process.prependOnceListener('SIGTERM', () => process.exit(17));\n")
  const p = launch(r, ['serve'], { NODE_OPTIONS: `--import=${hook}` })
  await ready(r, p)
  p.child.kill('SIGTERM')
  expect(await p.closed).toBe(1)
}, 30_000)

for (const restarted of [false, true]) it.runIf(process.platform === 'linux')(`the service ends when its foreground launcher is killed${restarted ? ' after a service restart' : ''}`, async () => {
  const r = root(), boots = join(r.folder, 'service-boots'), hook = join(r.folder, 'record-boot.mjs')
  writeFileSync(hook, `import { appendFileSync } from 'node:fs'; if (process.argv[1]?.endsWith('/engine/src/service.ts')) appendFileSync(${JSON.stringify(boots)}, 'boot\\n');\n`)
  const p = launch(r, ['serve'], { NODE_OPTIONS: `--import=${hook}` })
  await ready(r, p)
  const ids = readFileSync(`/proc/${p.child.pid}/task/${p.child.pid}/children`, 'utf8').trim().split(/\s+/).map(Number)
  expect(ids).toHaveLength(1)
  const servicePid = ids[0]!
  try {
    if (restarted) {
      await Effect.runPromise(Effect.scoped(Effect.gen(function* () {
        const client = yield* connect({ environment: { ...environment(r), LGTM_TOKEN: undefined } })
        yield* client['client.service.restart']({ requestId: randomUUID() as never, confirmed: true })
      })))
      await vi.waitFor(() => expect(readFileSync(boots, 'utf8').trim().split('\n')).toHaveLength(2), { timeout: 5000 })
      await ready(r, p)
    }
    const exited = new Promise(resolve => p.child.once('exit', resolve))
    p.child.kill('SIGKILL')
    await exited
    await vi.waitFor(async () => expect(await answers(r.socket)).toBe(false), { timeout: 5000 })
  } finally {
    // This is the direct service child discovered while our launcher was alive.
    if (await answers(r.socket)) process.kill(servicePid, 'SIGTERM')
    await p.closed
  }
  const again = launch(r)
  await ready(r, again)
}, 30_000)

it('waits for shutdown after its terminal output pipe closes', async () => {
  const r = root(), hook = join(r.folder, 'shutdown-output.mjs')
  writeFileSync(hook, "if (process.argv[1]?.endsWith('/engine/src/service.ts')) process.prependOnceListener('SIGTERM', () => process.stdout.write('test shutdown forwarding\\n'));\n")
  const p = launch(r, ['serve'], { NODE_OPTIONS: `--import=${hook}` })
  await ready(r, p)
  // Closing the pipe reader reproduces a terminal that can no longer receive
  // output. The fixture forces a forwarded write during the real shutdown.
  p.child.stdout!.destroy(); p.child.kill('SIGHUP')
  expect(await p.closed).toBe(0)
  expect(p.error()).not.toMatch(/EPIPE|Unhandled|service failed to start/)
  expect(await answers(r.socket)).toBe(false)
}, 30000)
