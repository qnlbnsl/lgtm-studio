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
export const endingRows = [
  { name: 'success', steps: [], next: false, outcome: { _tag: 'success' } },
  { name: 'capacity', steps: [{ step: 'rpc_error', message: 'capacity fixture' }], next: true, outcome: { _tag: 'capacity' } },
  { name: 'out-of-usage error', steps: [{ step: 'out_of_usage', message: 'usage fixture', resetsAt: '2099-01-01T00:00:00Z' }], next: true, outcome: { _tag: 'pause', reason: 'out-of-usage' } },
  { name: 'out-of-usage text', steps: [{ step: 'text', text: 'usage fixture' }], next: true, outcome: { _tag: 'pause', reason: 'out-of-usage' } },
  { name: 'signed-out', steps: [{ step: 'rpc_error', message: 'sign-in fixture' }], next: true, outcome: { _tag: 'pause', reason: 'signed-out' } },
  { name: 'prompt error', steps: [{ step: 'rpc_error', message: 'ordinary fixture' }], next: true, outcome: { _tag: 'error' } },
  { name: 'cancel v2', steps: [{ step: 'plan', entries: [] }, { step: 'slow', ms: 60000 }], next: true, outcome: { _tag: 'success' }, cancel: true },
  { name: 'cancel v1', steps: [{ step: 'plan', entries: [] }, { step: 'slow', ms: 60000 }], next: true, outcome: { _tag: 'success' }, cancel: true, protocolVersion: 1 },
] as const
for (const row of endingRows) it.live(`s22 r2: ${row.name} leaves instructions owed=${row.next} for the retry`, () =>
  withAcpRunner(rig, { classifyError: ({ text }) => text.includes('capacity fixture') ? 'capacity' : text.includes('usage fixture') ? 'out-of-usage' : text.includes('sign-in fixture') ? 'signed-out' : undefined }, (context) => Effect.gen(function* () {
    const echo = echoFile(rig)
    const harness = harnessFor('cursor', writeFakeAdapter(rig))
    const spawn = yield* spawnFor(rig, harness, accountFor(harness.id, { protocolVersion: 'protocolVersion' in row ? row.protocolVersion : 2, turns: [row.steps, [], []] }, echo))
    const instructions = 'PRIVATE ROLE RULES'
    yield* context.runner.start({ ...spawn.input, instructions })
    const delivery = deliveryFor(spawn.agentId, 'Retry the original brief.')
    const input = { agentId: spawn.agentId, incarnation: spawn.incarnation, delivery }
    yield* context.runner.prompt({ ...input, sentSequence: 1 })
    if ('cancel' in row) {
      yield* context.waitFor((e) => e._tag === 'deliveryUpdate' && e.kind === 'plan')
      yield* context.runner.cancel({ agentId: spawn.agentId, deliveryId: delivery.id, sentSequence: 1 })
    }
    const first = yield* context.waitFor((e) => e._tag === 'turnEnded' && e.sentSequence === 1)
    expect(first).toMatchObject('cancel' in row ? { stopped: true } : { _tag: 'turnEnded' })
    expect(context.events.find((e) => e._tag === 'promptReply')).toMatchObject({ outcome: row.outcome })
    yield* context.runner.prompt({ ...input, sentSequence: 2 })
    yield* context.waitFor((e) => e._tag === 'turnEnded' && e.sentSequence === 2)
    yield* context.runner.prompt({ ...input, sentSequence: 3 })
    yield* context.waitFor((e) => e._tag === 'turnEnded' && e.sentSequence === 3)
    const texts = promptTexts(yield* readEcho(echo))
    expect(texts[0]).toContain(instructions)
    expect(texts[1]!.includes(instructions)).toBe(row.next)
    expect(texts[2]).toBe('Retry the original brief.')
  })))
for (const ending of ['stop', 'crash'] as const) it.live(`s22 r2: ${ending} during the carrying turn restores instructions in a fresh session`, () =>
  withAcpRunner(rig, {}, (context) => Effect.gen(function* () {
    const echo = echoFile(rig), harness = harnessFor('first', writeFakeAdapter(rig))
    const steps = ending === 'stop' ? [{ step: 'plan', entries: [] }, { step: 'slow', ms: 60000 }]
      : [{ step: 'plan', entries: [] }, { step: 'slow', ms: 50 }, { step: 'crash', exitCode: 1 }]
    const spawn = yield* spawnFor(rig, harness, accountFor(harness.id, { turns: [steps] }, echo))
    const instructions = 'PRIVATE ROLE RULES', delivery = deliveryFor(spawn.agentId, 'Retry the original brief.')
    const input = { agentId: spawn.agentId, incarnation: spawn.incarnation, delivery }
    yield* context.runner.start({ ...spawn.input, instructions })
    yield* context.runner.prompt({ ...input, sentSequence: 1 })
    yield* context.waitFor((e) => e._tag === 'deliveryUpdate' && e.kind === 'plan')
    expect(promptTexts(yield* readEcho(echo))[0]).toContain(instructions)
    if (ending === 'stop') yield* context.runner.stop({ agentId: spawn.agentId })
    else yield* context.waitFor((e) => e._tag === 'exited')
    yield* context.runner.start({ ...spawn.input, account: accountFor(harness.id, { turns: [[], []] }, echo), instructions })
    yield* context.runner.prompt({ ...input, sentSequence: 2 })
    yield* context.waitFor((e) => e._tag === 'turnEnded' && e.sentSequence === 2)
    yield* context.runner.prompt({ ...input, sentSequence: 3 })
    yield* context.waitFor((e) => e._tag === 'turnEnded' && e.sentSequence === 3)
    const texts = promptTexts(yield* readEcho(echo))
    expect(texts[0]).toContain(instructions)
    expect(texts[1]).toBe('Retry the original brief.')
  })))
it.live('s22 r2: engine rules have their own label and do not add replay acceptance proof', () =>
  withAcpRunner(rig, {}, (context) => Effect.gen(function* () {
    const echo = echoFile(rig)
    const spawn = yield* spawnFor(rig, harnessFor('first', writeFakeAdapter(rig)), accountFor('first', { turns: [[]] }, echo))
    yield* context.runner.start({ ...spawn.input, instructions: 'Engine rules.' })
    const delivery = deliveryFor(spawn.agentId, 'Owner brief.')
    yield* context.runner.prompt({ agentId: spawn.agentId, incarnation: spawn.incarnation, delivery, sentSequence: 1 })
    yield* context.waitFor((e) => e._tag === 'turnEnded')
    const text = promptTexts(yield* readEcho(echo))[0]!
    expect(text).toMatch(/^Engine role instructions \[[A-Za-z0-9_-]{24}\]:\n/)
    expect(text).not.toContain('(delivery ')
    expect(text).not.toContain('Data from another agent')
    expect(text.endsWith('\n\nOwner brief.')).toBe(true)
  })))
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
    let reads = 0
    yield* context.runner.load({ ...spawn.input, restoreInstructions: () => Effect.sync(() => { reads++; return instructions }), session: opened.session })
    yield* send('Loaded brief.')
    expect(promptTexts(yield* readEcho(echo, (e) => promptTexts(e).length === 1))[0]).toBe('Loaded brief.')
    expect(framesOf(yield* readEcho(echo), 'session/resume')).toHaveLength(1)
    expect(reads).toBe(0)
    const loadedCompact = compactFor(spawn.agentId), loadedBefore = context.events.length
    yield* context.runner.compact({ agentId: spawn.agentId, incarnation: spawn.incarnation, delivery: loadedCompact, sentSequence: 1 })
    yield* context.waitFor((e) => e._tag === 'turnEnded' && e.deliveryId === loadedCompact.id, loadedBefore)
    yield* send('After loaded compaction.')
    expect(reads).toBe(1)
    expect(promptTexts(yield* readEcho(echo)).at(-1)).toContain(instructions)
    yield* context.runner.stop({ agentId: spawn.agentId })
    yield* context.runner.start(input)
    yield* send('Fresh after crash.')
    expect(promptTexts(yield* readEcho(echo, (e) => promptTexts(e).length === 1))[0]).toContain(instructions)
  })))
