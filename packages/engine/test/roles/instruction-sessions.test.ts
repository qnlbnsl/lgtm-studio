import { it } from '@effect/vitest'
import * as Effect from 'effect/Effect'
import { afterEach, beforeEach, expect, vi } from 'vitest'
import { accountFor, compactFor, deliveryFor, echoFile, framesOf, harnessFor, makeRunnerRig, promptTexts, readEcho, removeRunnerRig, spawnFor, withAcpRunner, writeFakeAdapter, type RunnerRig } from '../support/acpRunnerRig.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
let rig: RunnerRig
beforeEach(() => { rig = makeRunnerRig(); vi.stubEnv('HOME', rig.home); vi.stubEnv('LGTM_HOME', rig.stateFolder) })
afterEach(() => { vi.unstubAllEnvs(); removeRunnerRig(rig) })
registerRebuildCheck({ noLog: true })
const catalogue = { step: 'raw_frame', frame: JSON.stringify({ jsonrpc: '2.0', method: 'session/update', params: { sessionId: '${session}', update: { sessionUpdate: 'available_commands_update', availableCommands: [{ name: 'compact', description: 'Compact' }] } } }) }
it.live('s22: fresh sessions receive instructions once, loaded sessions retain memory, compaction restores instructions on the next prompt', () =>
  withAcpRunner(rig, {}, (context) => Effect.gen(function* () {
    const echo = echoFile(rig)
    const spawn = yield* spawnFor(rig, harnessFor('first', writeFakeAdapter(rig)), accountFor('first', { turns: [[catalogue], [], [], [], []] }, echo))
    const instructions = '# Worker base\n\n# Builder'
    const input = { ...spawn.input, instructions }
    const opened = yield* context.runner.start(input)
    const send = (text: string) => Effect.gen(function* () {
      const delivery = deliveryFor(spawn.agentId, text), before = context.events.length
      yield* context.runner.prompt({ agentId: spawn.agentId, incarnation: spawn.incarnation, delivery, sentSequence: 1 })
      yield* context.waitFor((e) => e._tag === 'turnEnded' && e.deliveryId === delivery.id, before)
    })
    yield* send('First brief.')
    yield* send('Second brief.')
    let prompts = promptTexts(yield* readEcho(echo, (e) => promptTexts(e).length === 2))
    expect(prompts[0]).toContain(instructions)
    expect(prompts[0]!.indexOf(instructions)).toBeLessThan(prompts[0]!.indexOf('First brief.'))
    expect(prompts[1]).toBe('Second brief.')
    const compact = compactFor(spawn.agentId), before = context.events.length
    yield* context.runner.compact({ agentId: spawn.agentId, incarnation: spawn.incarnation, delivery: compact, sentSequence: 1 })
    yield* context.waitFor((e) => e._tag === 'turnEnded' && e.deliveryId === compact.id, before)
    yield* send('After compaction.')
    prompts = promptTexts(yield* readEcho(echo, (e) => promptTexts(e).length === 4))
    expect(prompts[2]).toBe('/compact')
    expect(prompts[3]).toContain(instructions)
    yield* context.runner.stop({ agentId: spawn.agentId })
    yield* context.runner.load({ ...input, session: opened.session })
    yield* send('Loaded brief.')
    expect(promptTexts(yield* readEcho(echo, (e) => promptTexts(e).length === 1))[0]).toBe('Loaded brief.')
    expect(framesOf(yield* readEcho(echo), 'session/resume')).toHaveLength(1)
    yield* context.runner.stop({ agentId: spawn.agentId })
    yield* context.runner.start(input)
    yield* send('Fresh after crash.')
    expect(promptTexts(yield* readEcho(echo, (e) => promptTexts(e).length === 1))[0]).toContain(instructions)
  })))
