import { readFileSync } from 'node:fs'
import { expect, it } from 'vitest'
import * as Effect from 'effect/Effect'
import { engineConnection } from '../src/connection.ts'

async function builtEngine() {
  const source = readFileSync('design/mock/js/engine.js', 'utf8')
  return Function(source.replaceAll('export ', '') + '\nreturn { request, setConnection };')()
}

it('settles a cancelled read at the shared website request boundary', async () => {
  const engine = await builtEngine()
  const connection = engineConnection({ location: { protocol: 'http:', host: '127.0.0.1:12345' }, debug() {},
    fetch: (async () => new Response(null, { status: 204 })) as typeof fetch, connect: (() => Effect.never) as never })
  connection.open(() => {})
  engine.setConnection(connection)
  let settled = false, error: unknown
  engine.request('client.web.settings', {}).then(() => { settled = true }, (e: unknown) => { settled = true; error = e })
  connection.close()
  for (let i = 0; i < 10; i++) await Promise.resolve()
  expect(error).toMatchObject({ name: 'AbortError', cancelledRead: true })
  expect(settled).toBe(true)
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
