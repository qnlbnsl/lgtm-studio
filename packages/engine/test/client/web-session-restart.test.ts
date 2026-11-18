import { createHmac, scryptSync } from 'node:crypto'
import { chmodSync, linkSync, readFileSync, symlinkSync, unlinkSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import * as Effect from 'effect/Effect'
import { expect, it } from 'vitest'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
import { login, post, withWeb } from '../support/webRig.ts'
registerRebuildCheck({ noLog: true })

// Synthetic fixture records must stay authentic when advancing their age.
// Otherwise expiry would accidentally test tampering and discard other devices.
const ageRecord = (stored: string, password: string) => {
  const snapshot = JSON.parse(stored), record = snapshot.sessions[0]
  record.createdAt = Date.now() - 31 * 86400000
  const key = scryptSync(password, Buffer.from(record.passwordSalt, 'hex'), 32, { N: 65536, r: 8, p: 1, maxmem: 80 * 1024 * 1024 })
  try { record.passwordBinding = createHmac('sha256', key).update('lgtm-web-password-v3\0').update(record.verifier).update('\0').update(String(record.createdAt)).digest('hex') }
  finally { key.fill(0) }
  return JSON.stringify(snapshot)
}

for (const change of ['none', 'password', 'removed', 'expired', 'tampered', 'mode', 'unreadable', 'symlink', 'hardlink', 'oversized', 'future', 'logout'] as const) {
  it(`s28 real listener restart: ${change}`, async () => withWeb(ctx => Effect.gen(function* () {
    const { cookie, setCookie } = yield* login(ctx)
    expect(setCookie).toContain('Max-Age=2592000')
    const file = join(ctx.rig.stateFolder, 'web-session.json')
    const stored = readFileSync(file, 'utf8')
    for (const value of [JSON.parse(stored).sessions[0].verifier, JSON.parse(stored).sessions[0].passwordBinding]) expect((yield* Effect.promise(() => post(ctx, '/api/session', undefined, { Cookie: 'lgtm_session=' + value }))).status).toBe(401)
    expect(stored.includes(cookie.split('=')[1]!)).toBe(false)
    expect(stored.includes(ctx.password)).toBe(false)
    if (change === 'logout') expect((yield* Effect.promise(() => post(ctx, '/api/logout', undefined, { Cookie: cookie }))).status).toBe(204)
    yield* ctx.close
    if (change === 'expired') writeFileSync(file, ageRecord(stored, ctx.password), { mode: 0o600 })
    if (change === 'tampered') writeFileSync(file, '{broken')
    if (change === 'mode') chmodSync(file, 0o644)
    if (change === 'unreadable') chmodSync(file, 0o000)
    if (change === 'symlink') { const target = join(ctx.rig.root, 'linked-session'); writeFileSync(target, stored, { mode: 0o600 }); unlinkSync(file); symlinkSync(target, file) }
    if (change === 'hardlink') linkSync(file, join(ctx.rig.root, 'linked-record'))
    if (change === 'oversized') writeFileSync(file, 'x'.repeat(4097))
    if (change === 'future') { const record = JSON.parse(stored); record.sessions[0].createdAt = Date.now() + 60000; writeFileSync(file, JSON.stringify(record)) }
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
      const process = yield* processWeb(), { cookie } = yield* login(process.ctx), phone = yield* login(process.ctx)
      expect((yield* Effect.promise(() => post(process.ctx, '/api/session', undefined, { Cookie: cookie }))).status).toBe(204)
      const file = join(process.root.state, 'web-session.json')
      if (change === 'logout') expect((yield* Effect.promise(() => post(process.ctx, '/api/logout', undefined, { Cookie: cookie }))).status).toBe(204)
      yield* process.service.stop
      if (change === 'expired') writeFileSync(file, ageRecord(readFileSync(file, 'utf8'), process.ctx.password))
      if (change === 'tampered') writeFileSync(file, '{broken')
      if (change === 'password') yield* process.restart('changed-s28-password')
      else if (change === 'removed') yield* process.restart(undefined)
      else yield* process.restart()
      if (change !== 'removed') expect((yield* Effect.promise(() => post(process.ctx, '/api/session', undefined, { Cookie: cookie }))).status).toBe(change === 'none' ? 204 : 401)
      if (change !== 'removed') expect((yield* Effect.promise(() => post(process.ctx, '/api/session', undefined, { Cookie: phone.cookie }))).status).toBe(['none', 'expired', 'logout'].includes(change) ? 204 : 401)
      if (change === 'password' || change === 'removed') {
        yield* process.restart()
        expect((yield* Effect.promise(() => post(process.ctx, '/api/session', undefined, { Cookie: cookie }))).status).toBe(401)
        expect((yield* Effect.promise(() => post(process.ctx, '/api/session', undefined, { Cookie: phone.cookie }))).status).toBe(401)
      }
    })))
  })
}
