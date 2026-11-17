import { it } from '@effect/vitest'
import * as Effect from 'effect/Effect'
import { afterEach, beforeEach, expect, vi } from 'vitest'
import * as translation from '../../src/runner/translate.ts'
import { instructionEnvelope } from '../../src/routing/envelope.ts'
import { accountFor, compactFor, deliveryFor, echoFile, harnessFor, makeRunnerRig, removeRunnerRig, spawnFor, withAcpRunner, writeFakeAdapter, type RunnerRig } from '../support/acpRunnerRig.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'

let rig: RunnerRig
beforeEach(() => { rig = makeRunnerRig(); vi.stubEnv('HOME', rig.home); vi.stubEnv('LGTM_HOME', rig.stateFolder) })
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllEnvs(); removeRunnerRig(rig) })
registerRebuildCheck({ noLog: true })
const catalogue = { step: 'raw_frame', frame: JSON.stringify({ jsonrpc: '2.0', method: 'session/update', params: { sessionId: '${session}', update: { sessionUpdate: 'available_commands_update', availableCommands: [{ name: 'compact', description: 'Compact' }] } } }) }

for (const leadingText of ['', '\n', '\uFEFF', '<user>\n']) it.live(`s22 r3: one turn filter protects every boundary between two user messages with prelude ${JSON.stringify(leadingText)}`, () => {
  // Live user chunks are currently ignored by the translator. Exercise the
  // runner boundary directly without changing that production behavior.
  const realTranslate = translation.translate
  vi.spyOn(translation, 'translate').mockImplementation((value, replay) => realTranslate(value, replay || (value as { sessionUpdate?: string }).sessionUpdate === 'user_message_chunk'))
  const instructions = 'PRIVATE ROLE RULES', caller = 'Caller brief.'
  const length = leadingText.length + instructionEnvelope(instructions).length + caller.length
  const turns = Array.from({ length: length + 1 }, (_, split) => [
    [...(split === 0 ? [catalogue] : []), { step: 'echo_prompt', chunkSize: 10000, messageSplit: split, leadingText }], [],
  ]).flat()
  return withAcpRunner(rig, {}, (context) => Effect.gen(function* () {
    const spawn = yield* spawnFor(rig, harnessFor('first', writeFakeAdapter(rig)), accountFor('first', { turns }, echoFile(rig)))
    yield* context.runner.start({ ...spawn.input, instructions })
    for (let split = 0; split <= length; split++) {
      const delivery = deliveryFor(spawn.agentId, caller), before = context.events.length
      yield* context.runner.prompt({ agentId: spawn.agentId, incarnation: spawn.incarnation, delivery, sentSequence: 1 })
      yield* context.waitFor((e) => e._tag === 'turnEnded' && e.deliveryId === delivery.id, before)
      const text = context.events.slice(before).filter((e) => e._tag === 'deliveryUpdate' && e.role === 'user').map((e) => 'text' in e ? e.text : '').join('')
      expect(text, `message boundary ${split}`).toBe(caller)
      if (split < length) {
        const compact = compactFor(spawn.agentId), compactBefore = context.events.length
        yield* context.runner.compact({ agentId: spawn.agentId, incarnation: spawn.incarnation, delivery: compact, sentSequence: 1 })
        yield* context.waitFor((e) => e._tag === 'turnEnded' && e.deliveryId === compact.id, compactBefore)
      }
    }
  }))
})
