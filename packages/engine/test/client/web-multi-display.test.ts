import * as Effect from 'effect/Effect'
import { expect, it, vi } from 'vitest'
import { serveDisplay } from '../../src/client/web/display/service.ts'
import { freePort, login, post, setSettings, withWeb } from '../support/webRig.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
registerRebuildCheck({ noLog: true })
vi.mock('../../src/client/web/display/service.ts', () => ({ serveDisplay: vi.fn() }))
it('r2 ending one browser revokes only its isolated frames and keeps the other browser authenticated', async () => {
  const closed: string[] = []
  let next = 0, input: (() => Promise<unknown>) | undefined
  vi.mocked(serveDisplay).mockImplementation(async options => {
    input = options.input
    return { open: async () => ({ id: 'frame-' + ++next, url: 'https://display.example/frame', kind: 'editor' }),
      closeSession: async (id: string) => { closed.push(id) }, close: async () => { closed.push('all') } } as any
  })
  await withWeb(ctx => Effect.gen(function* () {
    yield* setSettings({ ...ctx.settings, display: { publicAddress: 'https://display.example', port: yield* Effect.promise(freePort) } } as any)
    const desktop = yield* login(ctx), phone = yield* login(ctx)
    const selection = { _tag: 'editor', repoId: 'synthetic-repository' }
    const one = yield* Effect.promise(() => post(ctx, '/api/display/open', selection, { Cookie: desktop.cookie }))
    const two = yield* Effect.promise(() => post(ctx, '/api/display/open', selection, { Cookie: phone.cookie }))
    expect(one.status).toBe(200); expect(two.status).toBe(200)
    expect((yield* Effect.promise(() => post(ctx, '/api/logout', undefined, { Cookie: desktop.cookie }))).status).toBe(204)
    expect(closed).toEqual([JSON.parse(one.body).id])
    yield* Effect.promise(() => input!())
    expect((yield* Effect.promise(() => post(ctx, '/api/session', undefined, { Cookie: phone.cookie }))).status).toBe(204)
    expect((yield* Effect.promise(() => post(ctx, '/api/logout', undefined, { Cookie: phone.cookie }))).status).toBe(204)
    expect(closed).toContain(JSON.parse(two.body).id)
    expect(closed).toContain('all')
  }), { registerSecret: () => {} })
})
