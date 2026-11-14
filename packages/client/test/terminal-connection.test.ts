import './support/clientFixtureGroup.ts'
import * as Effect from 'effect/Effect'
import * as Stream from 'effect/Stream'
import { afterEach, expect, it } from 'vitest'
import { cleanupUnit7b } from '../../testing/test/support/unit7b.ts'
import { chunk, clientApiPeer, closeClientApiPeers, succeed } from './support/clientApiPeer.ts'
afterEach(async () => { await closeClientApiPeers(); await cleanupUnit7b() })
for (const transport of ['node', 'browser'] as const) {
  it(`${transport} keeps terminal output and controls on the opening socket`, async () => {
    const peer = await clientApiPeer(transport, (frame, socket) => {
      if (frame.tag === 'client.terminal.subscribe') chunk(socket, frame, [{ _tag: 'output', text: 'private-output' }])
      else succeed(socket, frame, { requestId: frame.payload.requestId, ...(frame.tag === 'client.terminal.open' ? { sessionId: 'private-session' } : {}) })
    })
    const connect = await peer.connect()
    await Effect.runPromise(Effect.scoped(Effect.gen(function* () {
      const client = yield* connect
      const session = yield* client['client.terminal.open']!({ requestId: 'open', projectId: 'p', worktreeId: 'w', cols: 80, rows: 24 })
      const output = yield* client['client.terminal.subscribe']!({ requestId: 'stream', sessionId: session.sessionId }).pipe(Stream.take(1), Stream.runCollect)
      expect(output).toEqual([{ _tag: 'output', text: 'private-output' }])
      yield* client['client.terminal.write']!({ requestId: 'keys', sessionId: session.sessionId, text: 'exit\r' })
    })).pipe(Effect.timeout(5000)))
    expect(peer.connections).toHaveLength(1)
    expect(peer.requests.map(r => r.connection)).toEqual([1, 1, 1])
  })
  it(`${transport} never replays terminal opens after a transport failure`, async () => {
    const peer = await clientApiPeer(transport, (_frame, socket) => socket.close(1012, 'connection lost'))
    const connect = await peer.connect()
    await Effect.runPromise(Effect.scoped(Effect.gen(function* () {
      const client = yield* connect
      const result = yield* client['client.terminal.open']!({ requestId: 'open', projectId: 'p', worktreeId: 'w', cols: 80, rows: 24 }).pipe(Effect.result)
      expect(result._tag).toBe('Failure')
    })).pipe(Effect.timeout(5000)))
    expect(peer.requests).toHaveLength(1)
  })
}
