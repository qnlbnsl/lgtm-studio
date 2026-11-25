import * as Effect from "effect/Effect"
import * as Redacted from "effect/Redacted"
import { expect, it } from "vitest"
import { makeRedactor } from "../../src/identity/redactor.ts"
import { Redactor } from "../../src/log/Redactor.ts"
import { registerRebuildCheck } from "../support/rebuildCheck.ts"

registerRebuildCheck({ noLog: true })
it("invalidates cached page checks when either live registry changes, despite fresh registry wrappers", async () => {
  const secrets: string[] = [], hashes = new Set<string>()
  const redactor = makeRedactor({ secrets: () => secrets.map(value => Redacted.make(value)), tokenHashes: () => new Set(hashes) })
  await Effect.runPromise(Effect.gen(function* () {
    const service = yield* Redactor
    const key = yield* service.cacheKey!
    expect(yield* service.cacheKey!).toBe(key)
    secrets.push("map-private-value")
    const withSecret = yield* service.cacheKey!
    expect(withSecret).not.toBe(key)
    expect(yield* service.redactText("map-private-value")).toBe("[REDACTED]")
    expect(yield* service.cacheKey!).toBe(withSecret)
    hashes.add("fake-registry-hash")
    const withToken = yield* service.cacheKey!
    expect(withToken).not.toBe(withSecret)
    expect(yield* service.cacheKey!).toBe(withToken)
    hashes.clear()
    expect(yield* service.cacheKey!).not.toBe(withToken)
  }).pipe(Effect.provide(redactor.layer)))
})
