import { readFileSync } from 'node:fs'
import { readRequestsSource } from '../scripts/readRequestsSource.ts'
import { isReadRequest } from '../src/readRequests.ts'
import { afterEach, expect, it, vi } from 'vitest'
import * as Effect from 'effect/Effect'
import * as Stream from 'effect/Stream'
import { ClientError, type Client, type connect } from '@lgtm/client/browser'
import { engineConnection, type StorePatch } from '../src/connection.ts'
import { engineSnapshot } from './fixtures.ts'

const connections: ReturnType<typeof engineConnection>[] = []
afterEach(() => { connections.splice(0).forEach(c => c.close()); vi.useRealTimers() })
function deferred<A>() { let resolve!: (value: A) => void; const promise = new Promise<A>(done => { resolve = done }); return { promise, resolve } }
function rig(observe: (patch: StorePatch) => void = () => {}) {
  const opening = deferred<Client>(), recovery = deferred<Client>(), lost = deferred<void>()
  let attempts = 0, dispatched = 0, streams = 0
  const state: StorePatch = {}
  const client = {
    'client.snapshot': () => Effect.succeed(engineSnapshot),
    'project.settings': () => Effect.succeed({}),
    'client.subscribe': () => ++streams > 1 ? Stream.never : Stream.fromEffect(Effect.promise(() => lost.promise).pipe(Effect.andThen(Effect.fail(new ClientError({ message: 'lost' }))))),
    'client.web.settings': () => Effect.sync(() => { dispatched++; return { terminalEnabled: true } }),
  } as unknown as Client
  const c = engineConnection({ location: { protocol: 'http:', host: '127.0.0.1:12345' }, debug() {},
    fetch: (async () => new Response(null, { status: 204 })) as typeof fetch,
    connect: (() => { attempts++; return Effect.promise(() => attempts === 1 ? opening.promise : recovery.promise) }) as typeof connect })
  connections.push(c); c.open(p => { Object.assign(state, p); observe(p) })
  return { c, state, client, opening, recovery, lost, dispatched: () => dispatched, attempts: () => attempts }
}

it('waits through the initial connection and snapshot before dispatching a read', async () => {
  const h = rig()
  let settled = false
  const result = h.c.request('client.web.settings', {}).then(v => { settled = true; return v }, e => { settled = true; throw e })
  void result.catch(() => {})
  await vi.waitFor(() => expect(h.attempts()).toBe(1))
  expect(settled).toBe(false)
  expect(h.dispatched()).toBe(0)
  h.opening.resolve(h.client)
  expect(await result).toEqual({ terminalEnabled: true })
  expect(h.dispatched()).toBe(1)
})

for (const cancel of ['page', 'request', 'close', 'sign out'] as const) it(`drops a waiting read on ${cancel}`, async () => {
  const h = rig(), abort = new AbortController()
  const result = h.c.request('client.web.settings', {}, cancel === 'page' ? { readSignal: abort.signal } : { signal: abort.signal }).catch(e => e)
  await vi.waitFor(() => expect(h.attempts()).toBe(1))
  if (cancel === 'close') h.c.close()
  else if (cancel === 'sign out') await h.c.signOut()
  else abort.abort()
  expect(((await result) as Error).name).toBe('AbortError')
  h.opening.resolve(h.client)
  await Promise.resolve()
  expect(h.dispatched()).toBe(0)
})

it('keeps writes immediate while a connection is being made', async () => {
  const h = rig()
  await expect(h.c.request('project.settings.set', { settings: {} })).rejects.toThrow('The service is not connected.')
  await expect(h.c.request('accounts', { operation: 'set' })).rejects.toThrow('The service is not connected.')
  await expect(h.c.request('harnesses', { operation: 'set' })).rejects.toThrow('The service is not connected.')
  h.opening.resolve(h.client)
})

for (const defect of [false, true]) it(`reports the existing sentence after a ${defect ? 'defect' : 'failed connection attempt'}`, async () => {
  const state: StorePatch = {}
  const c = engineConnection({ location: { protocol: 'http:', host: '127.0.0.1:12345' }, debug() {},
    fetch: (async () => new Response(null, { status: 204 })) as typeof fetch,
    connect: (() => defect ? Effect.die(new Error('unreachable')) : Effect.fail(new ClientError({ message: 'unreachable' }))) as unknown as typeof connect })
  connections.push(c)
  c.open(p => Object.assign(state, p))
  const pending = c.request('client.web.settings', {}).catch(e => e)
  expect(((await pending) as Error).message).toBe('The service is not connected.')
  await expect(c.request('client.web.settings', {})).rejects.toThrow('The service is not connected.')
})

it('reports loss, then waits during the next connection attempt', async () => {
  vi.useFakeTimers()
  const h = rig()
  h.opening.resolve(h.client)
  await vi.advanceTimersByTimeAsync(0)
  expect(h.state.conn?.status).toBe('live')
  h.lost.resolve()
  await vi.advanceTimersByTimeAsync(0)
  await expect(h.c.request('client.web.settings', {})).rejects.toThrow('The service is not connected.')
  await vi.advanceTimersByTimeAsync(250)
  expect(h.attempts()).toBe(2)
  let settled = false
  const result = h.c.request('client.web.settings', {}).finally(() => { settled = true })
  void result.catch(() => {})
  await vi.advanceTimersByTimeAsync(0)
  expect(settled).toBe(false)
  h.recovery.resolve(h.client)
  await vi.advanceTimersByTimeAsync(0)
  expect(await result).toEqual({ terminalEnabled: true })
  expect(h.dispatched()).toBe(1)
})

it('attaches a page signal in the live build and refuses a stale replacement', async () => {
  const source = readFileSync('design/mock/js/engine.js', 'utf8')
  expect(readRequestsSource(source)).toContain('readOptions(options)')
  expect(() => readRequestsSource(source.replace('return current.request(tag, payload, options);', ''))).toThrow('Read lifecycle integration')
  const { readOptions } = await import('../../../design/mock/js/live/reads.js' as string)
  const first = readOptions({}).readSignal
  window.dispatchEvent(new HashChangeEvent('hashchange'))
  expect(first.aborted).toBe(true)
  expect(readOptions({}).readSignal.aborted).toBe(false)
})

it('classifies the live readers and keeps owner actions immediate', () => {
  for (const tag of [
    'client.web.settings', 'client.service.version', 'client.workflows.list', 'client.workflows.read',
    'client.requests', 'client.alerts.list', 'client.harness.choices', 'project.settings', 'roles.role-file.get',
    'board.channels', 'board.peek', 'board.read', 'project.task.history', 'project.setup.discover',
    'project.mocks.read', 'project.mocks.reviews.read', 'project.mocks.artifact', 'project.design.read',
    'project.files.list', 'project.files.read', 'project.context.search', 'client.agent.configuration',
    'project.git.state', 'project.history', 'project.commit.files', 'project.conflict', 'project.conflict.resolved', 'project.review.status'
  ]) expect(isReadRequest(tag, {}), tag).toBe(true)
  expect(isReadRequest('accounts', { operation: 'list' })).toBe(true)
  expect(isReadRequest('harnesses', { operation: 'list' })).toBe(true)
  for (const tag of ['project.settings.set', 'client.service.restart', 'client.harness.install', 'client.harness.account.check',
    'client.terminal.open', 'display.open', 'project.stage', 'project.review.prepare', 'answer', 'configure', 'unknown']) {
    expect(isReadRequest(tag, {}), tag).toBe(false)
  }
})

it('drops a queued read if navigation wins just as the connection becomes live', async () => {
  const page = new AbortController()
  const h = rig(patch => { if (patch.conn?.status === 'live') page.abort() })
  const reply = h.c.request('client.web.settings', {}, { readSignal: page.signal }).catch(e => e)
  h.opening.resolve(h.client)
  expect(((await reply) as Error).name).toBe('AbortError')
  expect(h.dispatched()).toBe(0)
})
