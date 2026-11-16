import { readFileSync } from 'node:fs'
import { afterEach, expect, it, vi } from 'vitest'
import * as Effect from 'effect/Effect'
import * as Stream from 'effect/Stream'
import { engineConnection } from '../src/connection.ts'
import { engineSnapshot } from './fixtures.ts'
import { fixture, flush } from './cancelled-read-fixture.ts'
import * as chatMapping from '../src/chat.ts'

const deferred = <A,>() => { let resolve!: (value: A) => void; const promise = new Promise<A>(done => { resolve = done }); return { promise, resolve } }
const navigate = (hash: string) => { window.history.replaceState({}, '', hash); window.dispatchEvent(new HashChangeEvent('hashchange')) }
let f: Awaited<ReturnType<typeof fixture>>, c: ReturnType<typeof engineConnection>, stop: (() => void) | undefined, unmount: (() => void) | undefined
afterEach(() => { unmount?.(); unmount = undefined; stop?.(); stop = undefined; c?.close(); f?.close() })

async function setup(mode: 'waiting' | 'sent', held: string, response: any) {
  navigate('#/p/p/board'); f = await fixture()
  const opening = deferred<any>(), result = deferred<any>(), calls: string[] = []
  const client = new Proxy({}, { get: (_, tag: string) => {
    if (tag === 'then') return undefined
    if (tag === 'client.snapshot') return () => Effect.succeed(engineSnapshot)
    if (tag === 'client.subscribe') return () => Stream.never
    return (payload: any) => Effect.promise(async () => { calls.push(tag); return tag === held ? result.promise : f.reply(tag, payload) })
  } })
  let live = false
  c = engineConnection({ location: { protocol: 'http:', host: '127.0.0.1:12345' }, debug() {},
    fetch: (async () => new Response(null, { status: 204 })) as typeof fetch, connect: (() => Effect.promise(() => opening.promise)) as never })
  c.open(patch => { if (patch.conn?.status === 'live') live = true })
  f.engine.setConnection(c)
  if (mode === 'sent') { opening.resolve(client); await vi.waitFor(() => expect(live).toBe(true)) }
  return { calls, async change(hash = '#/p/p/files') {
    if (mode === 'sent') await vi.waitFor(() => expect(calls).toContain(held))
    else expect(calls).not.toContain(held)
    navigate(hash)
    opening.resolve(client); result.resolve(response)
    await flush()
  } }
}

for (const mode of ['waiting', 'sent'] as const) {
  it(`delivers and shows a docked lead thread across a project tab change while ${mode}`, async () => {
    const message = { sequence: 1, occurredAt: 1000, event: { _tag: 'ThreadItem', agentId: 'a', itemId: 'i', content: { _tag: 'text', text: 'The lead history arrived.' } } }
    const h = await setup(mode, 'client.thread', { items: [message] })
    f.chat.setMapping({ ...chatMapping, mapSubscriptionNames: () => [], mapAgentDetails: () => ({}), mapRosterDetails: () => ({}), mapFailedStart: () => null, mapHomeRequest: () => null })
    const kit = await import('../../../design/mock/vendor/htm-preact/standalone.module.js' as string)
    const source = readFileSync('design/mock/js/ui.js', 'utf8'), begin = source.indexOf('export function Thread('), end = source.indexOf('// Only the project lead', begin)
    const watch = vi.fn(f.chat.watchThread)
    const env = { ...kit, chatLive: { ...f.chat, watchThread: watch }, liveMode: { on: true }, agentOf: () => ({ id: 'a', name: 'Lead', project: 'p' }), threadOf: (s: any) => s.threads.a || [],
      useNow: () => 1000, cx: (...v: any[]) => v.filter(Boolean).join(' '), dayLabel: () => '', clock: () => '', Avatar: () => null,
      Rich: ({ text }: any) => kit.html`<p>${text}</p>`, Attachments: () => null }
    const Thread = Function(...Object.keys(env), source.slice(begin, end).replace('export ', '') + '\nreturn Thread;')(...Object.values(env))
    const host = document.createElement('div'); document.body.append(host)
    unmount = () => { kit.render(null, host); host.remove() }
    const render = () => kit.render(kit.h(Thread, { s: f.state, agentId: 'a' }), host)
    render(); await vi.waitFor(() => expect(watch).toHaveBeenCalledTimes(1))
    expect(host.textContent).not.toContain('The lead history arrived.')
    await h.change(); await vi.waitFor(() => expect(f.state.threads.a?.[0]?.text).toBe('The lead history arrived.'))
    render(); expect(host.textContent).toContain('The lead history arrived.'); expect(watch).toHaveBeenCalledTimes(1)
    expect(f.toasts).toEqual([])
  })

  it(`loads project mocks across a tab change while ${mode}`, async () => {
    const h = await setup(mode, 'project.mocks.read', { items: [] })
    f.connection.mocks.mapMocks = () => ({ revisionId: 'delivered' })
    const work = f.mocks.loadMocks('p'); await flush(); expect(f.board.busy('read-mocks:p')).toBe(true)
    await h.change(); await work
    expect(f.state.mocks.p.revisionId).toBe('delivered'); expect(f.board.busy('read-mocks:p')).toBe(false); expect(f.toasts).toEqual([])
  })

  it(`finishes mock approval across a tab change during its checks while ${mode}`, async () => {
    const h = await setup(mode, 'project.mocks.artifact', { result: { _tag: 'available', fingerprint: 'fingerprint' } })
    f.connection.mocks.mapMocks = () => ({ revisionId: 'revision', sources: [{ sourceId: 'source' }], artifacts: [{ sourceId: 'source', fingerprint: 'fingerprint' }] })
    f.mocks.approveMocks({ id: 'p' }); const work = f.dialogs[0].onConfirm(); await flush()
    await h.change(); await work
    expect(h.calls).toContain('project.mocks.approve'); expect(f.board.busy('approve-mocks:p')).toBe(false)

  })

  it(`delivers a Files read across a change of ?f= while ${mode}`, async () => {
    const h = await setup(mode, 'project.files.read', { result: { kind: 'text', path: 'file', text: 'Delivered file', version: 'v1', editable: true } })
    navigate('#/p/p/files?f=one'); const work = f.model.open('file'); await flush()
    expect(f.model.opened.loading).toBe(true)
    await h.change('#/p/p/files?f=two'); await work
    expect(f.model.opened.file.text).toBe('Delivered file')
  })
}

it('the Files model keeps B when A replies after B in the same model', async () => {
  f = await fixture()
  const first = deferred<any>(), second = deferred<any>(), model = f.model
  const request = f.engine.request
  f.engine.request = (tag: string, payload: any, options: any) => tag === 'project.files.read'
    ? (payload.path === 'A' ? first.promise : second.promise) : request(tag, payload, options)
  const openingA = model.open('A'); await flush()
  expect(model.selected).toBe('A'); expect(model.opened.loading).toBe(true)
  const openingB = model.open('B'); await flush()
  expect(model.selected).toBe('B'); expect(model.opened.loading).toBe(true)
  second.resolve({ result: { kind: 'text', path: 'B', text: 'File B', version: 'b', editable: true } }); await openingB
  expect(model.opened.file).toMatchObject({ path: 'B', text: 'File B' })
  first.resolve({ result: { kind: 'text', path: 'A', text: 'Late file A', version: 'a', editable: true } }); await openingA
  expect(model.selected).toBe('B'); expect(model.opened.file).toMatchObject({ path: 'B', text: 'File B' })
})

it('the Git reader ignores a late summary after its visit ends', async () => {
  f = await fixture(); stop = f.git.openScreen('git', 'p'); await f.cancel()
  const controller = f.git.status().gitController, record = controller.records.get('w'), reply = deferred<any>()
  const old = f.state.worktrees[0], work = f.git.readGitRecord(old, 'p', record, controller, false, false, () => reply.promise)
  await flush(); expect(record.phase).toBe('reading')
  stop!(); stop = undefined; navigate('#/p/p/files'); await flush()
  const before = JSON.stringify(f.state.worktrees)
  reply.resolve({ result: { head: 'late-old-head' } }); await work
  expect(JSON.stringify(f.state.worktrees)).toBe(before)
})
