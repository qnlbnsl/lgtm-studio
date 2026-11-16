import { readFileSync } from 'node:fs'
import { expect, it } from 'vitest'
import * as Effect from 'effect/Effect'
import { readRequestsSource } from '../scripts/readRequestsSource.ts'
import { engineConnection } from '../src/connection.ts'

const readers = () => import('../../../design/mock/js/live/reads.js' as string)
const navigate = (hash: string) => window.history.replaceState({}, '', hash)
async function builtEngine() {
  const { readOptions, silenceCancelledRead } = await readers()
  const source = readRequestsSource(readFileSync('design/mock/js/engine.js', 'utf8'))
  return Function('readOptions', 'silenceCancelledRead', source.replace(/^import[^\n]*\n/, '').replaceAll('export ', '') + '\nreturn { request, setConnection };')(readOptions, silenceCancelledRead)
}

it('retains a waiting read across a query change on the same screen', async () => {
  const { readOptions } = await readers()
  navigate('#/p/project-a/files?f=one')
  const first = readOptions({}).readSignal
  navigate('#/p/project-a/files?f=two')
  window.dispatchEvent(new HashChangeEvent('hashchange'))
  expect(first.aborted).toBe(false)
  expect(readOptions({}).readSignal).toBe(first)
})

it('does not cancel a new screen read when its navigation event arrives later', async () => {
  const { readOptions } = await readers()
  navigate('#/p/project-a/files')
  const first = readOptions({}).readSignal
  navigate('#/p/project-a/git')
  const next = readOptions({}).readSignal
  expect(first.aborted).toBe(true)
  window.dispatchEvent(new HashChangeEvent('hashchange'))
  expect(next.aborted).toBe(false)
  expect(readOptions({}).readSignal).toBe(next)
})

it('silently drops a cancelled read at the shared website request boundary', async () => {
  const engine = await builtEngine()
  const connection = engineConnection({ location: { protocol: 'http:', host: '127.0.0.1:12345' }, debug() {},
    fetch: (async () => new Response(null, { status: 204 })) as typeof fetch, connect: (() => Effect.never) as never })
  connection.open(() => {})
  engine.setConnection(connection)
  let settled = false, error: unknown
  engine.request('client.web.settings', {}).then(() => { settled = true }, (e: unknown) => { settled = true; error = e })
  connection.close()
  for (let i = 0; i < 10; i++) await Promise.resolve()
  expect(error).toBeUndefined()
  expect(settled).toBe(false)
})

it('preserves actual failures and cancelled writes at the shared boundary', async () => {
  const engine = await builtEngine()
  const failure = new Error('The service is not connected.')
  engine.setConnection({ request: () => Promise.reject(failure) })
  await expect(engine.request('client.web.settings', {})).rejects.toBe(failure)
  const cancelledWrite = new DOMException('The owner action was cancelled.', 'AbortError')
  engine.setConnection({ request: () => Promise.reject(cancelledWrite) })
  await expect(engine.request('project.settings.set', {})).rejects.toBe(cancelledWrite)
})

it('cancels the screen lifetime when the page closes', async () => {
  const { readOptions } = await readers()
  const first = readOptions({}).readSignal
  window.dispatchEvent(new Event('pagehide'))
  expect(first.aborted).toBe(true)
})
