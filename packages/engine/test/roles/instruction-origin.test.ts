import { it } from '@effect/vitest'
import * as Effect from 'effect/Effect'
import { afterEach, beforeEach, expect, vi } from 'vitest'
import { accountFor, echoFile, harnessFor, makeRunnerRig, promptTexts, readEcho, removeRunnerRig, startPayloadFor, withEngine, writeFakeAdapter, type RunnerRig } from '../support/acpRunnerRig.ts'
import { agentCredential, appendSetup, call, eventOf, ownerCredential, readLog, success } from '../support/handlerRig.ts'
import { eventually, snapshot } from '../support/routingRig.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
import { dispatchQueued } from '../../src/routing/deliveries.ts'

let rig: RunnerRig
beforeEach(() => { rig = makeRunnerRig(); vi.stubEnv('HOME', rig.home); vi.stubEnv('LGTM_HOME', rig.stateFolder) })
afterEach(() => { vi.unstubAllEnvs(); removeRunnerRig(rig) })
const catalogue = { step: 'raw_frame', frame: JSON.stringify({ jsonrpc: '2.0', method: 'session/update', params: { sessionId: '${session}', update: { sessionUpdate: 'available_commands_update', availableCommands: [{ name: 'compact', description: 'Compact' }] } } }) }

it.live('s22 r3: a routed child question after compaction is not the lead brief, and owner prompts name no agent', () => {
  const harness = harnessFor('first', writeFakeAdapter(rig)), leadEcho = echoFile(rig), workerEcho = echoFile(rig)
  const leadAccount = accountFor(harness.id, { turns: [[catalogue], [], []] }, leadEcho, {}, 'lead-account')
  const workerAccount = accountFor(harness.id, { turns: [[]] }, workerEcho, {}, 'worker-account')
  return withEngine(rig, { harness, accounts: [leadAccount, workerAccount] }, (context) => Effect.gen(function* () {
    yield* registerRebuildCheck()
    const lead = success(yield* call(ownerCredential, 'start', startPayloadFor(harness, leadAccount, rig.work, { role: 'lead:build-lead', prompt: 'Lead this work.' })))
    yield* context.waitFor((e) => e._tag === 'turnEnded' && e.agentId === lead.agentId)
    yield* eventually(snapshot, (s) => s.agents[lead.agentId]?.state._tag === 'idle')
    const peer = yield* readEcho(leadEcho, (e) => promptTexts(e).length >= 1)
    expect(promptTexts(peer)[0]).not.toContain('The following data block is the brief from agent')
    const compact = success(yield* call(ownerCredential, 'compact', { agentId: lead.agentId }))
    yield* context.waitFor((e) => e._tag === 'turnEnded' && e.deliveryId === compact.deliveryId)
    yield* eventually(snapshot, (s) => s.agents[lead.agentId]?.state._tag === 'idle')
    const worker = success(yield* call(agentCredential(peer.env.LGTM_TOKEN!), 'start', startPayloadFor(harness, workerAccount, rig.work, { role: 'worker:builder', prompt: 'Use the agreed fixture.' })))
    const workerPeer = yield* readEcho(workerEcho, (e) => promptTexts(e).length >= 1)
    success(yield* call(agentCredential(workerPeer.env.LGTM_TOKEN!), 'ask', { text: 'Which fixture should I use?' }))
    const routed = promptTexts(yield* readEcho(leadEcho, (e) => promptTexts(e).length === 3))[2]!
    expect(routed).toContain('# Rules for every lead')
    expect(routed).toContain('Which fixture should I use?')
    expect(routed).not.toContain('The following data block is the brief from agent')
    expect(Object.values((yield* snapshot).deliveries).find((row) => row.target === lead.agentId && row.sender._tag === 'agent' && row.sender.agentId === worker.agentId))
      .toMatchObject({ sender: { _tag: 'agent', agentId: worker.agentId }, source: { _tag: 'question' } })
  }))
})
it.live('s22 r3: a retry of a real parent start brief retains its brief label after a failed first turn', () => {
  const harness = harnessFor('first', writeFakeAdapter(rig)), leadEcho = echoFile(rig), workerEcho = echoFile(rig)
  const leadAccount = accountFor(harness.id, { turns: [[]] }, leadEcho, {}, 'lead-account')
  const workerAccount = accountFor(harness.id, { turns: [[{ step: 'rpc_error', message: 'capacity fixture' }], []] }, workerEcho, {}, 'worker-account')
  return withEngine(rig, { harness, accounts: [leadAccount, workerAccount], capacityHoldCondition: () => true,
    classifyError: ({ text }) => text.includes('capacity fixture') ? 'capacity' : undefined,
  }, (context) => Effect.gen(function* () {
    yield* registerRebuildCheck()
    const lead = success(yield* call(ownerCredential, 'start', startPayloadFor(harness, leadAccount, rig.work, { role: 'lead:build-lead', prompt: 'Lead this work.' })))
    yield* eventually(snapshot, (s) => s.agents[lead.agentId]?.state._tag === 'idle')
    const peer = yield* readEcho(leadEcho, (e) => promptTexts(e).length >= 1)
    const worker = success(yield* call(agentCredential(peer.env.LGTM_TOKEN!), 'start', startPayloadFor(harness, workerAccount, rig.work, { role: 'worker:builder', prompt: 'Implement this exact brief.' })))
    yield* context.waitFor((e) => e._tag === 'turnEnded' && e.agentId === worker.agentId)
    yield* eventually(snapshot, (s) => s.agents[worker.agentId]?.capacityHeld === true && s.agents[worker.agentId]?.state._tag === 'idle')
    const started = (yield* readLog).find((row) => row.event._tag === 'AgentStarted' && row.event.agentId === worker.agentId)!.event
    if (started._tag !== 'AgentStarted') throw new Error('Missing started event')
    const brief = started.firstPromptDelivery!
    // Release the real held delivery without waiting for the capacity timer.
    yield* appendSetup([eventOf({ _tag: 'DeliveryReleased', agentId: worker.agentId })])
    yield* dispatchQueued(worker.agentId)
    const texts = promptTexts(yield* readEcho(workerEcho, (e) => promptTexts(e).length === 2))
    for (const text of texts) {
      expect(text).toContain(`The following data block is the brief from agent ${lead.agentId}.`)
      expect(text.endsWith(brief.content._tag === 'prompt' ? brief.content.text : '')).toBe(true)
    }
    expect((yield* readLog).filter((row) => row.event._tag === 'DeliverySent' && row.event.deliveryId === brief.id)).toHaveLength(2)
  }))
})
