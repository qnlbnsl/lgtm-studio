import { promises as fs } from 'node:fs'
import { it } from '@effect/vitest'
import * as Effect from 'effect/Effect'
import { afterEach, beforeEach, expect, vi } from 'vitest'
import { makeFakeRunner } from '../support/fakeRunner.ts'
import { agentCredential, call, failureOf, makeRig, ownerCredential, readLog, removeRig, seedAccount, seedAgent, seedHarness, success, type Rig } from '../support/handlerRig.ts'
import { appendEvents, provideRestart, recover } from '../support/restartRig.ts'
import { snapshot } from '../support/routingRig.ts'
import { unwrap } from '../client/support/readRig.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'

const live = (name: string, body: () => Effect.Effect<unknown, unknown, any>) => it.live(name, () => body() as Effect.Effect<unknown, unknown, never>)
let rig: Rig
beforeEach(() => { rig = makeRig(); vi.stubEnv('HOME', rig.home); vi.stubEnv('LGTM_HOME', rig.stateFolder) })
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllEnvs(); removeRig(rig) })
for (const fault of ['legacy oversized advice', 'shipped file read error'] as const) live(`s22 r2: restart loads a healthy session despite ${fault}`, () => Effect.gen(function* () {
  const runner = yield* makeFakeRunner()
  const agent = yield* provideRestart(rig, runner.layer, Effect.gen(function* () {
    yield* registerRebuildCheck(); yield* seedAccount(); yield* seedHarness()
    const agent = yield* seedAgent({ id: 'builder', role: 'worker:builder', folder: rig.work })
    // Old committed files remain valid history even when new admission rejects them.
    if (fault === 'legacy oversized advice') yield* appendEvents({ _tag: 'roles.RoleFileSet', roleFileHash: 'a'.repeat(64), content: JSON.stringify({ name: 'Legacy', roles: { builder: { advice: 'x'.repeat(45 * 1024), limits: {} } } }) })
    return agent
  }), { recovering: false })
  if (fault === 'shipped file read error') {
    const read = fs.readFile.bind(fs)
    vi.spyOn(fs, 'readFile').mockImplementation(((...args: Parameters<typeof fs.readFile>) => String(args[0]).includes('/roles/instructions/') ? Promise.reject(new Error('private read detail')) : read(...args)) as typeof fs.readFile)
  }
  yield* provideRestart(rig, runner.layer, Effect.gen(function* () {
    yield* registerRebuildCheck(); yield* runner.subscribed; yield* recover
    expect(runner.callsOf('load').map((row) => row.input.agentId)).toEqual([agent.agentId])
    expect(runner.callsOf('load')[0]!.input.instructions).toBeUndefined()
    expect((yield* snapshot).agents[agent.agentId]!.state._tag).toBe('idle')
    expect((yield* readLog).filter((row) => row.event._tag === 'AgentReloadFailed')).toEqual([])
  }))
}))
live('s22 r2: role-file set refuses advice whose base makes the composition oversized', () => Effect.gen(function* () {
  const runner = yield* makeFakeRunner()
  yield* provideRestart(rig, runner.layer, Effect.gen(function* () {
    yield* registerRebuildCheck()
    expect(failureOf(yield* call(ownerCredential, 'roles.role-file.set', { content: JSON.stringify({ name: 'Large', roles: { builder: { advice: 'x'.repeat(45 * 1024), limits: {} } } }) }))).toMatchObject({ _tag: 'CommandRefused', reason: 'Role instructions exceed 48 KiB.' })
    expect((yield* readLog).filter((row) => row.event._tag === 'roles.RoleFileSet')).toEqual([])
  }), { recovering: false })
}))
live('s22 r2: orient retains identity and roster when a shipped file cannot be read', () => Effect.gen(function* () {
  const runner = yield* makeFakeRunner()
  yield* provideRestart(rig, runner.layer, Effect.gen(function* () {
    yield* registerRebuildCheck(); yield* seedAccount(); yield* seedHarness()
    const agent = yield* seedAgent({ id: 'builder', role: 'worker:builder', folder: rig.work })
    const read = fs.readFile.bind(fs)
    vi.spyOn(fs, 'readFile').mockImplementation(((...args: Parameters<typeof fs.readFile>) => String(args[0]).includes('/roles/instructions/') ? Promise.reject(new Error('private read detail')) : read(...args)) as typeof fs.readFile)
    const reply = success(yield* call(agentCredential(agent.token), 'roles.orient', {}))
    const text = unwrap(reply)
    expect(text.agentId).toBe(agent.agentId)
    expect(text.agents.map((row: {agentId: string}) => row.agentId)).toContain(agent.agentId)
    expect(text.advice).toBe('Role instructions are unavailable. Ask your lead for the current brief.')
    expect(JSON.stringify(reply)).not.toContain('private read detail')
  }), { recovering: false })
}))
for (const fault of ['oversized composition', 'shipped file read error'] as const) live(`s22 r3: a fresh restart logs the fixed composition reason for ${fault}`, () => Effect.gen(function* () {
  const runner = yield* makeFakeRunner(), lines: string[] = []
  yield* provideRestart(rig, runner.layer, Effect.gen(function* () {
    yield* registerRebuildCheck(); yield* seedAccount(); yield* seedHarness()
    yield* seedAgent({ id: 'starting-builder', role: 'worker:builder', folder: rig.work, state: 'starting' })
    if (fault === 'oversized composition') yield* appendEvents({ _tag: 'roles.RoleFileSet', roleFileHash: 'a'.repeat(64), content: JSON.stringify({ name: 'Legacy', roles: { builder: { advice: 'x'.repeat(45 * 1024), limits: {} } } }) })
  }), { recovering: false })
  if (fault === 'shipped file read error') {
    const read = fs.readFile.bind(fs)
    vi.spyOn(fs, 'readFile').mockImplementation(((...args: Parameters<typeof fs.readFile>) => String(args[0]).includes('/roles/instructions/') ? Promise.reject(new Error('private restart detail')) : read(...args)) as typeof fs.readFile)
  }
  yield* provideRestart(rig, runner.layer, Effect.gen(function* () {
    yield* registerRebuildCheck(); yield* runner.subscribed; yield* recover
    expect(runner.callsOf('start')).toHaveLength(0)
    expect((yield* readLog).filter((row) => row.event._tag === 'AgentReloadFailed')).toMatchObject([{ event: { reason: 'agent start failed' } }])
    const reason = fault === 'oversized composition' ? 'Role instructions exceed 48 KiB.' : 'Role instructions could not be read.'
    expect(lines.filter((line) => line.includes('Role instruction composition failed during restart: ' + reason))).toHaveLength(1)
    expect(lines.join('\n')).not.toContain('private restart detail')
  }), { logLines: lines })
}))
live('s22 r3: orientation refuses when roster and metadata alone exceed 256 KiB', () => Effect.gen(function* () {
  const runner = yield* makeFakeRunner()
  yield* provideRestart(rig, runner.layer, Effect.gen(function* () {
    yield* registerRebuildCheck(); yield* seedAccount(); yield* seedHarness()
    const lead = yield* seedAgent({ id: 'lead', role: 'lead:build-lead', folder: rig.work })
    yield* appendEvents(...Array.from({ length: 800 }, (_, index) => ({
      _tag: 'AgentStarted', agentId: `child-${index}`, name: 'n'.repeat(200), role: 'worker:builder', groups: [],
      parent: { _tag: 'agent', agentId: lead.agentId }, harness: 'first', account: 'primary', permission: 'ask', folder: rig.work, runMode: 'acp',
    })))
    expect(failureOf(yield* call(agentCredential(lead.token), 'roles.orient', {}))).toMatchObject({ _tag: 'CommandRefused', reason: 'Role orientation exceeds 256 KiB.' })
  }), { recovering: false })
}))
