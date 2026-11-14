import { expect } from 'vitest'
import * as Effect from 'effect/Effect'
import * as Logger from 'effect/Logger'
import * as Tracer from 'effect/Tracer'

// Wrap server creation too: child fibers must inherit these collectors, so
// server-side logging and RPC tracing are checked along with the client.
export const withPtyPrivacy = <A, E, R>(body: Effect.Effect<A, E, R>, privateText: readonly string[]) => Effect.gen(function* () {
  const logs: string[] = [], spans: Tracer.NativeSpan[] = []
  const tracer = Tracer.make({ span: options => { const span = new Tracer.NativeSpan(options); spans.push(span); return span } })
  const result = yield* body.pipe(
    Effect.provide(Logger.layer([Logger.make(options => { logs.push(Logger.formatJson.log(options)) })])),
    Effect.withTracer(tracer)
  )
  const diagnostics = JSON.stringify({ logs, spans: spans.map(span => ({ name: span.name, attributes: [...span.attributes], events: span.events, status: span.status })) }, (_key, value) => typeof value === 'bigint' ? value.toString() : value)
  for (const text of privateText) expect(diagnostics).not.toContain(text)
  return result
})
