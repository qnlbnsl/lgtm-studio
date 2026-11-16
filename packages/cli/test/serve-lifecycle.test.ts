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
