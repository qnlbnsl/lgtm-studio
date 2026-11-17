import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { afterEach, expect } from '@effect/vitest'
import * as Effect from 'effect/Effect'
import { eventsUntil, isTag, peekEcho, promptsOf, refusal, requestId, scenario, stopAllServices, waitFor } from '../support/acceptance.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
registerRebuildCheck({ noLog: true })
afterEach(stopAllServices)
scenario('s22: real service reinstructs a nonreloadable builder on fresh resume and keeps owner advice, slash escapes and secrets out of stored prompts', (w) => Effect.gen(function* () {
  yield* w.owner['roles.settings.set']({ requestId: requestId(), settings: { stallWatch: false } } as never)
  const folder = join(w.root.home, '.lgtm/roles'); mkdirSync(folder, { recursive: true })
  writeFileSync(join(folder, 'worker-base.md'), '# Owner worker base\n /base-command\n')
  writeFileSync(join(folder, 'builder.md'), '# Replaced disk builder')
  yield* w.owner['roles.role-file.set']({ requestId: requestId(), content: JSON.stringify({ name: 'Owner', roles: {
    builder: { advice: '# Owner builder\n\u200b/role-command', limits: {} },
  } }) } as never)
  const worker = yield* w.start(w.owner, { role: 'worker:builder', prompt: 'Original brief.', script: { reloadable: false, turns: [] } })
  yield* eventsUntil(w.owner, 0, 'first builder turn', isTag('TurnEnded', worker.agentId))
  const first = promptsOf(peekEcho(worker.echo)!)[0]!
  expect(first).toContain('# Owner worker base')
  expect(first).toContain('# Owner builder')
  expect(first).not.toContain('# Replaced disk builder')
  expect(first).toContain(' \\/base-command')
  expect(first).toContain('\u200b\\/role-command')
  expect(first.indexOf('# Owner builder')).toBeLessThan(first.indexOf('Original brief.'))
  const client = yield* w.asAgent((yield* w.token(worker)).token)
  const oriented = (yield* client['roles.orient']({ requestId: requestId() } as never)) as any
  const advice = JSON.parse(oriented.data.slice(oriented.data.indexOf('\n') + 1, oriented.data.lastIndexOf('\n'))).advice
  expect(first).toContain(advice)
  const thread = yield* w.owner['client.thread']({ requestId: requestId(), agentId: worker.agentId, after: 0, limit: 500 } as never)
  expect(JSON.stringify(thread)).not.toContain('# Owner worker base')
  expect(JSON.stringify(thread)).toContain('Original brief.')
  yield* w.owner.stop({ requestId: requestId(), agentId: worker.agentId } as never)
  expect(yield* refusal(w.owner.resume({ requestId: requestId(), agentId: worker.agentId } as never))).toBeDefined()
  yield* w.owner.resume({ requestId: requestId(), agentId: worker.agentId, fresh: true } as never)
  const mark = yield* w.watermark()
  yield* w.owner.prompt({ requestId: requestId(), agentId: worker.agentId, text: 'Fresh brief.' } as never)
  yield* eventsUntil(w.owner, mark, 'fresh builder turn', isTag('TurnEnded', worker.agentId))
  yield* waitFor('the fresh prompt', () => promptsOf(peekEcho(worker.echo)!).some((p) => p.includes('Fresh brief.')))
  const fresh = promptsOf(peekEcho(worker.echo)!).at(-1)!
  expect(fresh).toContain(advice)
  expect(fresh).toContain('Fresh brief.')
}))
