import { EventEmitter } from 'node:events'
import { afterEach, expect, it, vi } from 'vitest'
import * as Effect from 'effect/Effect'

const rig = vi.hoisted(() => ({ spawn: vi.fn() }))
vi.mock('node:child_process', () => ({ spawn: rig.spawn }))
vi.mock('node:http', () => ({ request: () => {
  const call = new EventEmitter() as EventEmitter & { end(): void }
  call.end = () => queueMicrotask(() => call.emit('error', new Error('not listening')))
  return call
} }))
import { serveForeground } from '../src/serve.ts'

afterEach(() => { vi.restoreAllMocks(); rig.spawn.mockReset() })
function launch() {
  const child = Object.assign(new EventEmitter(), { pid: 12345, exitCode: null as number | null, signalCode: null, stdout: new EventEmitter(), kill: vi.fn() })
  rig.spawn.mockReturnValue(child)
  const signal = new AbortController()
  const result = Effect.runPromise(serveForeground({ environment: { LGTM_HOME: '/tmp/s18-foreground-lifecycle' }, signal: signal.signal } as Parameters<typeof serveForeground>[0]).pipe(Effect.result))
  const close = (code: number) => { child.exitCode = code; child.emit('close', code) }
  return { child, signal, result, close }
}

it('reports the lock race refusal with only the already-running sentence', async () => {
  const h = launch()
  await vi.waitFor(() => expect(rig.spawn).toHaveBeenCalledOnce())
  h.child.emit('message', { _tag: 'LgtmAlreadyRunning' })
  h.close(1)
  expect(await h.result).toMatchObject({ _tag: 'Failure', failure: { message: 'The service is already running for this state folder.' } })
})

for (const code of [0, 1]) it(`reports an unexpected stop after startup even with child exit ${code}`, async () => {
  const h = launch()
  await vi.waitFor(() => expect(rig.spawn).toHaveBeenCalledOnce())
  h.child.emit('message', { _tag: 'LgtmServiceStarted' })
  h.close(code)
  expect(await h.result).toMatchObject({ _tag: 'Failure', failure: { message: 'The service stopped unexpectedly.' } })
})

for (const code of [0, 1]) it(`returns success after a signal only when shutdown exits zero (child exit ${code})`, async () => {
  const h = launch()
  await vi.waitFor(() => expect(rig.spawn).toHaveBeenCalledOnce())
  h.child.emit('message', { _tag: 'LgtmServiceStarted' })
  h.signal.abort()
  const killed = h.child.kill.mock.calls.length
  h.close(code)
  expect(await h.result).toMatchObject(code === 0 ? { _tag: 'Success', success: 0 } : { _tag: 'Failure', failure: { message: 'The service stopped unexpectedly.' } })
  expect(killed).toBe(1)
})

it('preserves a startup failure category when a signal arrives during startup', async () => {
  const h = launch()
  await vi.waitFor(() => expect(rig.spawn).toHaveBeenCalledOnce())
  h.child.emit('message', { _tag: 'LgtmStartupFailed', reason: 'state folder mode is not 0700' })
  h.signal.abort()
  h.close(1)
  expect(await h.result).toMatchObject({ _tag: 'Failure', failure: { message: 'service failed to start: state folder mode is not 0700' } })
  expect(h.child.kill).toHaveBeenCalledOnce()
})

for (const code of [0, 1]) it(`keeps an early owner stop quiet and returns child outcome ${code}`, async () => {
  const h = launch()
  await vi.waitFor(() => expect(rig.spawn).toHaveBeenCalledOnce())
  h.signal.abort(); h.close(code)
  expect(await h.result).toMatchObject({ _tag: 'Success', success: code })
})

it('ignores a failed forwarding write and waits for the service to close', async () => {
  const h = launch()
  await vi.waitFor(() => expect(rig.spawn).toHaveBeenCalledOnce())
  h.child.emit('message', { _tag: 'LgtmServiceStarted' })
  // A closed pipe stands in for a terminal whose output has been hung up.
  const { Writable } = await import('node:stream')
  const output = new Writable({ write(_bytes, _encoding, done) { done(Object.assign(new Error('closed'), { code: 'EPIPE' })) } })
  const original = process.stdout.write
  vi.spyOn(process.stdout, 'write').mockImplementation(function (this: typeof process.stdout, ...args: any[]) {
    if (args[0]?.toString() === 'service shutdown\n') return (output.write as any)(...args)
    return (original as any).apply(this, args)
  })
  const errors: unknown[] = []
  // Observe only to keep the red test from ending the test runner itself.
  output.on('error', e => { errors.push(e); process.stdout.emit('error', e) })
  let done = false; void h.result.then(() => { done = true })
  h.child.stdout.emit('data', Buffer.from('service shutdown\n'))
  await new Promise(resolve => setImmediate(resolve))
  expect(errors).toHaveLength(1); expect(done).toBe(false)
  h.signal.abort(); h.close(0)
  expect(await h.result).toMatchObject({ _tag: 'Success', success: 0 })
})
