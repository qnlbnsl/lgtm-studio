import { EventEmitter } from 'node:events'
import { afterEach, expect, it, vi } from 'vitest'
import * as Effect from 'effect/Effect'

const rig = vi.hoisted(() => ({ listening: false, spawn: vi.fn() }))
vi.mock('node:child_process', () => ({ spawn: rig.spawn }))
vi.mock('node:http', () => ({ request: (_options: unknown, receive: (response: unknown) => void) => {
  const call = new EventEmitter() as EventEmitter & { end(): void }
  call.end = () => queueMicrotask(() => rig.listening ? receive({ resume() {} }) : call.emit('error', new Error('not listening')))
  return call
} }))
import { serveDetach } from '../src/serve.ts'

afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks(); rig.listening = false; rig.spawn.mockReset() })

for (const order of ['message-first', 'exit-first', 'exit-at-deadline']) it(`retains the refusal until the child's close event (${order})`, async () => {
  vi.useFakeTimers()
  const child = Object.assign(new EventEmitter(), { pid: 12345, unref() {} })
  rig.spawn.mockReturnValue(child)
  const result = Effect.runPromise(serveDetach({ environment: { LGTM_HOME: '/tmp/s17-ordered-startup' } }).pipe(Effect.result))
  await vi.advanceTimersByTimeAsync(0)
  if (order === 'exit-at-deadline') await vi.advanceTimersByTimeAsync(29_975)
  const message = () => child.emit('message', { _tag: 'LgtmStartupFailed', reason: 'state folder mode is not 0700' })
  const exit = () => child.emit('exit', 1)
  if (order === 'message-first') message(); else exit()
  await vi.advanceTimersByTimeAsync(25)
  if (order === 'message-first') exit(); else message()
  child.emit('close', 1)
  await vi.advanceTimersByTimeAsync(25)
  expect(await result).toMatchObject({ _tag: 'Failure', failure: { message: 'service failed to start: state folder mode is not 0700' } })
})

it('fails at the deadline when a child exits zero without an answering service', async () => {
  vi.useFakeTimers()
  const child = Object.assign(new EventEmitter(), { pid: 12345, unref() {}, kill: vi.fn() })
  rig.spawn.mockReturnValue(child)
  let finished = false
  const result = Effect.runPromise(serveDetach({ environment: { LGTM_HOME: '/tmp/s17-ordered-startup' } }).pipe(Effect.result))
    .then(value => { finished = true; return value })
  await vi.advanceTimersByTimeAsync(0)
  child.emit('exit', 0)
  child.emit('close', 0)
  await vi.advanceTimersByTimeAsync(29_975)
  expect(finished).toBe(false)
  await vi.advanceTimersByTimeAsync(25)
  expect(finished).toBe(true)
  expect(await result).toMatchObject({ _tag: 'Failure', failure: { message: 'service did not answer' } })
  expect(child.kill).not.toHaveBeenCalled()
})

it('returns before short recovery outlasts an injected launcher deadline', async () => {
  vi.useFakeTimers()
  const child = Object.assign(new EventEmitter(), { pid: 12345, unref() {}, kill: vi.fn() })
  const killGroup = vi.spyOn(process, 'kill').mockReturnValue(true)
  rig.spawn.mockReturnValue(child)
  let recovered = false
  const result = Effect.runPromise(serveDetach({ environment: { LGTM_HOME: '/tmp/s17-ordered-startup' }, timeoutMs: 100 }))
  await vi.advanceTimersByTimeAsync(0)
  child.emit('message', { _tag: 'LgtmLockAcquired' })
  rig.listening = true
  setTimeout(() => { recovered = true }, 200)
  await vi.advanceTimersByTimeAsync(25)
  expect(await result).toEqual({ outcome: 'started', pid: 12345 })
  expect(recovered).toBe(false)
  await vi.advanceTimersByTimeAsync(200)
  expect(recovered).toBe(true)
  expect(rig.listening).toBe(true)
  expect(child.kill).not.toHaveBeenCalled()
  expect(killGroup).not.toHaveBeenCalled()
})
