import { randomBytes } from 'node:crypto'
import { execFileSync } from 'node:child_process'
import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { it } from '@effect/vitest'
import * as Effect from 'effect/Effect'
import * as Redacted from 'effect/Redacted'
import { afterEach, beforeEach, expect, vi } from 'vitest'
import * as Server from '../../src/api/server.ts'
import { projectView } from '../../src/project/view.ts'
import { projectLeadInstruction } from '../../src/project/lifecycle.ts'
import { agentCredential, call, failureOf, freshRequest, ownerCredential, readLog, success } from '../support/handlerRig.ts'
import { snapshot, eventually } from '../support/routingRig.ts'
import { connect } from '../support/socketClient.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
import { accountFor, echoFile, harnessFor, makeRunnerRig, promptTexts, readEcho, removeRunnerRig, startPayloadFor, withEngine, writeFakeAdapter, type RunnerRig } from '../support/acpRunnerRig.ts'

let rig: RunnerRig
beforeEach(() => {
  rig = makeRunnerRig(); vi.stubEnv('HOME', rig.home); vi.stubEnv('LGTM_HOME', rig.stateFolder)
  const git = (...args: string[]) => execFileSync('git', ['-C', rig.work, ...args], { stdio: 'pipe' })
  git('init', '-b', 'main')
  git('-c', 'commit.gpgsign=false', '-c', 'user.name=Test', '-c', 'user.email=test@example.test', 'commit', '--allow-empty', '-m', 'Initial')
})
afterEach(() => { vi.unstubAllEnvs(); removeRunnerRig(rig) })

for (const overrideLead of [false, true]) it.live(`s22: authenticated project creation delivers bases, project role and goal, and its lead starts builders with per-file overrides without changing stored prompts (lead override ${overrideLead})`, () => {
  const harness = harnessFor('cursor', writeFakeAdapter(rig))
  const leadEcho = echoFile(rig), builderEcho = echoFile(rig), overrideEcho = echoFile(rig)
  const accounts = [leadEcho, builderEcho, overrideEcho].map((echo, n) => accountFor(harness.id, { turns: [[{ step: 'echo_prompt', chunkSize: 137 }]] }, echo, { CURSOR_API_KEY: 'secret:role-test' }, `account-${n}`))
  return withEngine(rig, { harness, accounts, secrets: { 'secret:role-test': 'synthetic-role-test-credential-1234' } }, () => Effect.gen(function* () {
    yield* registerRebuildCheck()
    const bearer = randomBytes(32).toString('base64url')
    yield* Server.serve({ socketPath: rig.socket, ownerCredential: Redacted.make(bearer), debug: false })
    const api = yield* connect(rig.socket, bearer)
    yield* api['project.settings.set']({ requestId: freshRequest(), settings: { workspaceFolder: rig.work, shell: 'sh', lead: {
      harness: harness.id, account: { _tag: 'named', accountId: accounts[0]!.id }, permission: 'edit',
    } } })
    const { repoId } = yield* api['project.repository.add']({ requestId: freshRequest(), name: 'Product', path: rig.work, machineId: 'local' })
    if (overrideLead) yield* api['roles.role-file.set']({ requestId: freshRequest(), content: JSON.stringify({ name: 'Owner', roles: { 'project-lead': { advice: '# Owner project role', limits: {} } } }) })
    const goal = 'Deliver the requested product slice.'
    const created = yield* api['project.create']({ requestId: freshRequest(), repository: { _tag: 'existing', repoId }, name: 'Slice', goal, machineId: 'local' })
    const state = projectView(yield* snapshot), project = state.projects[created.projectId]!, workspace = state.workspaces[project.worktreeId]!
    const leadPeer = yield* readEcho(leadEcho, (e) => promptTexts(e).length > 0)
    const text = promptTexts(leadPeer)[0]!
    expect(text.indexOf('# Rules for every lead')).toBeGreaterThanOrEqual(0)
    const roleHeading = overrideLead ? '# Owner project role' : '# Project lead'
    expect(text.indexOf(roleHeading)).toBeGreaterThan(text.indexOf('# Rules for every lead'))
    expect(text.indexOf(projectLeadInstruction('Slice', goal))).toBeGreaterThan(text.indexOf(roleHeading))
    expect(text.trimEnd().endsWith(goal)).toBe(true)
    yield* eventually(snapshot, (s) => s.agents[project.leadAgentId]?.state._tag === 'idle')
    const thread = yield* api['client.thread']({ requestId: freshRequest(), agentId: project.leadAgentId, after: 0 as never, limit: 500 })
    expect(JSON.stringify(thread)).not.toContain('# Rules for every lead')
    expect(JSON.stringify(thread)).toContain(goal)
    const stored = (yield* readLog).find(({ event }) => event._tag === 'AgentStarted')!.event
    expect(stored._tag === 'AgentStarted' && stored.firstPromptDelivery?.content).toEqual({ _tag: 'prompt', text: projectLeadInstruction('Slice', goal) })
    const lead = yield* connect(rig.socket, leadPeer.env.LGTM_TOKEN!)
    for (const permission of ['planning', 'starting-work'] as const) yield* api['project.permission.set']({ requestId: freshRequest(), projectId: created.projectId, permission, setting: 'allowed' })
    const brief = 'Implement this assigned slice.'
    const launch = (n: number) => lead.start({ requestId: freshRequest(), name: `Builder ${n}`, role: 'worker:builder', harness: harness.id,
      account: { _tag: 'named', accountId: accounts[n]!.id }, permission: 'edit', folder: workspace.path, prompt: brief })
    const builder = yield* launch(1)
    const workerText = promptTexts(yield* readEcho(builderEcho, (e) => promptTexts(e).length > 0))[0]!
    expect(workerText.indexOf('# Rules for every worker')).toBeGreaterThanOrEqual(0)
    expect(workerText.indexOf('# Builder')).toBeGreaterThan(workerText.indexOf('# Rules for every worker'))
    expect(workerText.indexOf(brief)).toBeGreaterThan(workerText.indexOf('# Builder'))
    mkdirSync(join(rig.home, '.lgtm/roles'), { recursive: true })
    writeFileSync(join(rig.home, '.lgtm/roles/builder.md'), 'Custom builder instruction.\nsynthetic-role-test-credential-1234\n/hidden-command')
    yield* launch(2)
    const overrideText = promptTexts(yield* readEcho(overrideEcho, (e) => promptTexts(e).length > 0))[0]!
    expect(overrideText).toContain('# Rules for every worker')
    expect(overrideText).toContain('Custom builder instruction.')
    expect(overrideText).not.toContain('# Builder')
    expect(overrideText).not.toContain('synthetic-role-test-credential-1234')
    expect(overrideText).toContain('\\/hidden-command')
    yield* eventually(snapshot, (s) => s.agents[builder.agentId]?.state._tag === 'idle')
    const builderThread = yield* api['client.thread']({ requestId: freshRequest(), agentId: builder.agentId, after: 0 as never, limit: 500 })
    expect(JSON.stringify(builderThread)).not.toContain('# Rules for every worker')
    expect(JSON.stringify(builderThread)).toContain(brief)
    yield* eventually(snapshot, (s) => Object.values(s.agents).every((a) => a.state._tag === 'idle'))
  }))
})

it.live('s22: oversized composition refuses start before AgentStarted or a process, and oversized orient replies are refused', () => {
  const harness = harnessFor('first', writeFakeAdapter(rig))
  const echo = echoFile(rig), account = accountFor(harness.id, { turns: [] }, echo)
  return withEngine(rig, { harness, accounts: [account] }, () => Effect.gen(function* () {
    yield* registerRebuildCheck()
    const bearer = randomBytes(32).toString('base64url')
    yield* Server.serve({ socketPath: rig.socket, ownerCredential: Redacted.make(bearer), debug: false })
    const api = yield* connect(rig.socket, bearer)
    success(yield* call(ownerCredential, 'roles.role-file.set', { content: JSON.stringify({ name: 'Large', roles: { builder: { advice: 'x'.repeat(48 * 1024), limits: {} } } }) }))
    expect(failureOf(yield* call(ownerCredential, 'start', startPayloadFor(harness, account, rig.work, { role: 'worker:builder' })))).toMatchObject({ _tag: 'CommandRefused', reason: 'Role instructions exceed 48 KiB.' })
    const publicRefusal = yield* api.start({ requestId: freshRequest(), name: 'Large builder', role: 'worker:builder', harness: harness.id, account: { _tag: 'named', accountId: account.id }, folder: rig.work, permission: 'ask' }).pipe(Effect.result)
    expect(publicRefusal._tag === 'Failure' && publicRefusal.failure).toMatchObject({ _tag: 'CommandError', reason: 'Role instructions exceed 48 KiB.' })
    expect((yield* readLog).filter(({ event }) => event._tag === 'AgentStarted')).toHaveLength(0)
    expect((yield* snapshot).agents).toEqual({})
    success(yield* call(ownerCredential, 'roles.role-file.set', { content: JSON.stringify({ name: 'Empty', roles: {} }) }))
    const folder = join(rig.home, '.lgtm/roles'); mkdirSync(folder, { recursive: true })
    writeFileSync(join(folder, 'worker-base.md'), '\u0000'.repeat(23 * 1024))
    writeFileSync(join(folder, 'builder.md'), '\u0000'.repeat(23 * 1024))
    const started = success(yield* call(ownerCredential, 'start', startPayloadFor(harness, account, rig.work, { role: 'worker:builder' })))
    const peer = yield* readEcho(echo)
    expect(failureOf(yield* call(agentCredential(peer.env.LGTM_TOKEN!), 'roles.orient', {}))).toMatchObject({ _tag: 'CommandRefused', reason: 'Role orientation exceeds 256 KiB.' })
    const agent = yield* connect(rig.socket, peer.env.LGTM_TOKEN!)
    const publicRead = yield* agent['roles.orient']({ requestId: freshRequest() }).pipe(Effect.result)
    expect(publicRead._tag === 'Failure' && publicRead.failure).toMatchObject({ _tag: 'CommandError', reason: 'Role orientation exceeds 256 KiB.' })
    expect((yield* snapshot).agents[started.agentId as string]).toBeDefined()
  }))
})
