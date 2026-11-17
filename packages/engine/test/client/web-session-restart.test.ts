import { chmodSync, readFileSync, symlinkSync, unlinkSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import * as Effect from 'effect/Effect'
import { expect, it } from 'vitest'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
import { login, post, withWeb } from '../support/webRig.ts'
registerRebuildCheck({ noLog: true })

for (const change of ['none', 'password', 'removed', 'expired', 'tampered', 'mode', 'unreadable', 'symlink', 'logout'] as const) {
  it(`s28 real listener restart: ${change}`, async () => withWeb(ctx => Effect.gen(function* () {
    const { cookie, setCookie } = yield* login(ctx)
    expect(setCookie).toContain('Max-Age=2592000')
    const file = join(ctx.rig.stateFolder, 'web-session.json')
    const stored = readFileSync(file, 'utf8')
    for (const value of [JSON.parse(stored).verifier, JSON.parse(stored).passwordBinding]) expect((yield* Effect.promise(() => post(ctx, '/api/session', undefined, { Cookie: 'lgtm_session=' + value }))).status).toBe(401)
    expect(stored.includes(cookie.split('=')[1]!)).toBe(false)
    expect(stored.includes(ctx.password)).toBe(false)
    if (change === 'logout') expect((yield* Effect.promise(() => post(ctx, '/api/logout', undefined, { Cookie: cookie }))).status).toBe(204)
    yield* ctx.close
    if (change === 'expired') { const record = JSON.parse(stored); record.createdAt = Date.now() - 31 * 86400000; writeFileSync(file, JSON.stringify(record), { mode: 0o600 }) }
    if (change === 'tampered') writeFileSync(file, '{broken')
    if (change === 'mode') chmodSync(file, 0o644)
    if (change === 'unreadable') chmodSync(file, 0o000)
    if (change === 'symlink') { const target = join(ctx.rig.root, 'linked-session'); writeFileSync(target, stored, { mode: 0o600 }); unlinkSync(file); symlinkSync(target, file) }
    const next = yield* ctx.open(change === 'password' ? 'a-different-password' : change === 'removed' ? undefined : ctx.password)
    if (change !== 'removed') expect((yield* Effect.promise(() => post(ctx, '/api/session', undefined, { Cookie: cookie }))).status).toBe(change === 'none' ? 204 : 401)
    yield* next.close
    // Removal invalidates the record even though there is no HTTP listener.
    if (change === 'removed' || change === 'password') {
      const restored = yield* ctx.open(ctx.password)
      expect((yield* Effect.promise(() => post(ctx, '/api/session', undefined, { Cookie: cookie }))).status).toBe(401)
      yield* restored.close
    }
  })))
}

// Repeat the revocation cases across real operating-system service processes.
for (const change of ['none', 'password', 'removed', 'expired', 'tampered', 'logout'] as const) {
  it(`s28 real engine process restart: ${change}`, async () => {
    const { processWeb } = await import('../support/processWeb.ts')
    await Effect.runPromise(Effect.scoped(Effect.gen(function* () {
      const process = yield* processWeb(), { cookie } = yield* login(process.ctx)
      const file = join(process.root.state, 'web-session.json')
      if (change === 'logout') expect((yield* Effect.promise(() => post(process.ctx, '/api/logout', undefined, { Cookie: cookie }))).status).toBe(204)
      yield* process.service.stop
      if (change === 'expired') { const record = JSON.parse(readFileSync(file, 'utf8')); record.createdAt = Date.now() - 31 * 86400000; writeFileSync(file, JSON.stringify(record)) }
      if (change === 'tampered') writeFileSync(file, '{broken')
      if (change === 'password') yield* process.restart('changed-s28-password')
      else if (change === 'removed') yield* process.restart(undefined)
      else yield* process.restart()
      if (change !== 'removed') expect((yield* Effect.promise(() => post(process.ctx, '/api/session', undefined, { Cookie: cookie }))).status).toBe(change === 'none' ? 204 : 401)
      if (change === 'password' || change === 'removed') {
        yield* process.restart()
        expect((yield* Effect.promise(() => post(process.ctx, '/api/session', undefined, { Cookie: cookie }))).status).toBe(401)
      }
    })))
  })
}
