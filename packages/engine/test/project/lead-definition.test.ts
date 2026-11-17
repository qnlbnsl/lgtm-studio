import { execFileSync } from 'node:child_process'
import { it } from '@effect/vitest'
import * as Effect from 'effect/Effect'
import * as Redacted from 'effect/Redacted'
import { type FakeRunner } from '../support/fakeRunner.ts'
import { afterEach, beforeEach, expect, vi } from 'vitest'
import { unwrap } from '../client/support/readRig.ts'
import type { BoardView } from '@lgtm/contracts'
import { cleanupUnit7b } from '../../../testing/test/support/unit7b.ts'
import { Views } from '../../src/views/Views.ts'
import { projectKey, projectView } from '../../src/project/view.ts'
import { provideBoard } from '../board/wakeFixtures.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
import { agentCredential, appendSetup, call, eventOf, failureOf, makeRig, ownerCredential as owner, readLog, removeRig, seedAgent, harnessSettings, freshRequest, success, type Rig } from '../support/handlerRig.ts'

let rig: Rig
beforeEach(() => {
  rig = makeRig(); vi.stubEnv('HOME', rig.home); vi.stubEnv('LGTM_HOME', rig.stateFolder)
  execFileSync('git', ['init', '-b', 'main', rig.work], { stdio: 'ignore' })
  execFileSync('git', ['-C', rig.work, '-c', 'user.name=Maya Okafor', '-c', 'user.email=maya@example.test', '-c', 'commit.gpgsign=false', 'commit', '--allow-empty', '-m', 'Initial'], { stdio: 'ignore' })
})
afterEach(() => { cleanupUnit7b(); vi.unstubAllEnvs(); removeRig(rig) })
const snapshot = Effect.gen(function* () { return yield* (yield* Views).snapshot })
const setup = (runner: FakeRunner, settings: 'ask' | 'default' = 'ask') => Effect.gen(function* () {
  yield* registerRebuildCheck()
  success(yield* call(owner, 'harnesses', { operation: 'set', harness: harnessSettings('cursor', { adapterPath: process.execPath, reportedModes: [{ id: 'default', level: 'ask' }, { id: 'acceptEdits', level: 'edit' }] }) }))
  success(yield* call(owner, 'accounts', { operation: 'add', account: { id: 'primary', harness: 'cursor', enabled: true } }))
  success(yield* call(owner, 'accounts set-secret', { accountId: 'primary', name: 'CURSOR_API_KEY', nonce: freshRequest(), value: Redacted.make('synthetic-lead-test-readiness') }))
  success(yield* call(owner, 'project.settings.set', { settings: { workspaceFolder: rig.root, shell: 'sh' } }))
  const repoId = success(yield* call(owner, 'project.repository.add', { name: 'Product', path: rig.work, machineId: 'local' })).repoId
  const projectId = success(yield* call(owner, 'project.create', { name: 'Product', goal: 'Make a useful product', machineId: 'local', repository: { _tag: 'existing', repoId } })).projectId
  const project = projectView(yield* snapshot).projects[projectId]!
  const lead = { token: Redacted.value(runner.callsOf('start')[0]!.input.token) }
  const worker = yield* seedAgent({ id: 'builder', name: 'Builder', parent: project.leadAgentId, state: 'idle' })
  const other = yield* seedAgent({ id: 'other-lead', name: 'Project lead', role: 'lead:project-lead', state: 'working' })
  yield* appendSetup([eventOf({ _tag: 'project.Created', ...project, projectId: 'other-project', leadAgentId: 'other-lead', name: 'Other' })])
  if (settings === 'ask') for (const permission of ['planning', 'starting-work']) success(yield* call(owner, 'project.permission.set', { projectId, permission, setting: 'ask' }))
  return { project, lead: agentCredential(lead.token), worker: agentCredential(worker.token), other: agentCredential(other.token) }
})
const approve = (credential: ReturnType<typeof agentCredential>, projectId: string, projectApproval: string, accepted = true) => Effect.gen(function* () {
  const questionId = success(yield* call(credential, 'ask', { projectId, projectApproval, title: 'Approve the next step?', text: 'The full proposal.', proposedAnswer: 'Accepted' })).questionId
  success(yield* call(owner, 'answer', { kind: 'question', id: questionId, text: accepted ? 'Accepted' : 'Denied' }))
  return questionId
})

it.live('s10 item 1: opens with Define, short context updates, lead permissions and builders', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  yield* setup(runner)
  const started = (yield* readLog).find(({ event }) => event._tag === 'AgentStarted' && event.firstPromptDelivery)!.event
  expect(started._tag === 'AgentStarted' && started.firstPromptDelivery?.content).toMatchObject({ text: expect.stringContaining('Within the first minute') })
  const text = JSON.stringify(started)
  for (const phrase of ['Define', 'one or two sentences', 'about a minute', 'few at a time', 'end result', 'before building', 'builders', 'rename', 'Small work', 'Nitpick work', 'without sprints']) expect(text).toContain(phrase)
})).pipe(Effect.orDie))

it.live('s10 item 3: a created project lead receives lead orientation and a role name', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner)
  const orientation = unwrap(success(yield* call(f.lead, 'roles.orient', {})))
  expect(orientation).toMatchObject({ kind: 'lead', name: 'Project lead', advice: expect.stringContaining('Start your own workers') })
})).pipe(Effect.orDie))

it.live('s10 item 2: ask settings require explicit owner decisions for planning and starting work, with durable events', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner), p = f.project
  expect(projectView(yield* snapshot).projects[p.projectId]?.stage).toBe('define')
  const sprint = { projectId: p.projectId, label: 'SP-1', name: 'First', goal: 'Deliver', phase: 'Build', owner: p.leadAgentId, needs: [] }
  const team = { projectId: p.projectId, name: 'Builders', worktreeId: p.worktreeId }
  expect(failureOf(yield* call(f.lead, 'project.sprint.create', sprint))?.reason).toBe('Project planning requires permission; use ask with projectApproval set to "end-result" or "skip-planning".')
  expect(failureOf(yield* call(f.lead, 'project.team.create', team))?.reason).toBe('Project planning requires permission; use ask with projectApproval set to "end-result" or "skip-planning".')
  expect(failureOf(yield* call(f.lead, 'project.stage.set', { projectId: p.projectId, stage: 'build' }))).toBeDefined()
  success(yield* call(owner, 'project.stage.set', { projectId: p.projectId, stage: 'build' }))
  expect(failureOf(yield* call(f.lead, 'project.sprint.create', sprint))?.reason).toBe('Project planning requires permission; use ask with projectApproval set to "end-result" or "skip-planning".')
  success(yield* call(owner, 'project.stage.set', { projectId: p.projectId, stage: 'define' }))
  yield* approve(f.lead, p.projectId, 'end-result', false)
  expect(projectView(yield* snapshot).projects[p.projectId]?.stage).toBe('define')
  for (const actor of [owner, f.worker, f.other, { _tag: 'service', service: 'test.board' } as never]) expect(failureOf(yield* call(actor, 'ask', { projectId: p.projectId, projectApproval: 'end-result', text: 'Approve another project?' }))).toBeDefined()
  const ordinary = success(yield* call(f.lead, 'ask', { text: 'An ordinary question' })).questionId
  success(yield* call(owner, 'answer', { kind: 'question', id: ordinary, text: 'Accepted' }))
  expect(projectView(yield* snapshot).projects[p.projectId]?.endResultApproval).toBeUndefined()
  const questionId = success(yield* call(f.lead, 'ask', { projectId: p.projectId, projectApproval: 'end-result', title: 'Approve the end result?', text: 'A product, without deployment.' })).questionId
  for (const actor of [f.lead, f.worker, f.other, { _tag: 'service', service: 'test.board' } as never]) expect(failureOf(yield* call(actor, 'answer', { kind: 'question', id: questionId, text: 'Accepted' }))).toBeDefined()
  success(yield* call(owner, 'answer', { kind: 'question', id: questionId, text: 'Accepted' }))
  const sprintId = success(yield* call(f.lead, 'project.sprint.create', sprint)).sprintId
  const teamId = success(yield* call(f.lead, 'project.team.create', team)).teamId
  const task = { projectId: p.projectId, sprintId, teamId, owner: p.leadAgentId, worktreeId: p.worktreeId, title: 'Deliver', state: 'queue' }
  const taskId = success(yield* call(f.lead, 'project.task.create', task)).taskId
  expect(failureOf(yield* call(f.lead, 'project.task.update', { projectId: p.projectId, taskId, state: 'active' }))?.reason).toBe('Starting work requires permission; use ask with projectApproval set to "build".')
  const start = { name: 'Builder', role: 'worker:builder', harness: 'cursor', account: { _tag: 'any' }, permission: 'ask', folder: projectView(yield* snapshot).workspaces[p.worktreeId]!.path }
  expect(failureOf(yield* call(f.lead, 'start', start))?.reason).toBe('Starting work requires permission; use ask with projectApproval set to "build".')
  yield* approve(f.lead, p.projectId, 'build', false)
  expect(projectView(yield* snapshot).projects[p.projectId]?.stage).toBe('plan')
  yield* approve(f.lead, p.projectId, 'build')
  success(yield* call(f.lead, 'start', start))
  success(yield* call(f.lead, 'project.task.update', { projectId: p.projectId, taskId, state: 'active' }))
  success(yield* call(f.lead, 'project.task.update', { projectId: p.projectId, taskId, state: 'queue' }))
  expect(failureOf(yield* call(owner, 'project.task.delete', { projectId: p.projectId, taskId }))?.reason).toBe('Only a task that was never started can be deleted.')
  expect(projectView(yield* snapshot).projects[p.projectId]?.stage).toBe('build')
  expect((yield* readLog).filter(({ event }) => event._tag === 'project.PermissionDecided')).toHaveLength(4)
})).pipe(Effect.orDie))

it.live('s10 item 4: rename is available to the owner and own lead only and preserves placement', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner), p = f.project
  for (const actor of [f.other, f.worker, { _tag: 'service', service: 'test.board' } as never]) expect(failureOf(yield* call(actor, 'project.rename', { projectId: p.projectId, name: 'Wrong' }))).toBeDefined()
  for (const actor of [owner, f.lead]) success(yield* call(actor, 'project.rename', { projectId: p.projectId, name: 'Useful product' }))
  expect(projectView(yield* snapshot).projects[p.projectId]).toMatchObject({ name: 'Useful product', worktreeId: p.worktreeId })
  expect(failureOf(yield* call(f.lead, 'project.rename', { projectId: p.projectId, name: '  ' }))).toBeDefined()
})).pipe(Effect.orDie))

it.live('s10 item 5: delete only never-started tasks, refuse occupied teams and archive their board history', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner), p = f.project
  yield* approve(f.lead, p.projectId, 'end-result')
  const team = success(yield* call(f.lead, 'project.team.create', { projectId: p.projectId, name: 'Builders', worktreeId: p.worktreeId }))
  const sprintId = success(yield* call(f.lead, 'project.sprint.create', { projectId: p.projectId, label: 'SP-1', name: 'First', goal: 'Deliver', phase: 'Build', owner: p.leadAgentId, needs: [] })).sprintId
  const taskId = success(yield* call(f.lead, 'project.task.create', { projectId: p.projectId, teamId: team.teamId, sprintId, worktreeId: p.worktreeId, owner: p.leadAgentId, title: 'First', state: 'queue' })).taskId
  const teamPayload = { projectId: p.projectId, teamId: team.teamId }, taskPayload = { projectId: p.projectId, taskId }
  for (const actor of [f.other, f.worker, { _tag: 'service', service: 'test.board' } as never]) for (const [command, payload] of [['project.team.delete', teamPayload], ['project.task.delete', taskPayload]] as const) expect(failureOf(yield* call(actor, command, payload))).toBeDefined()
  expect(failureOf(yield* call(owner, 'project.team.delete', teamPayload))?.reason).toBe('Delete or move the team’s tasks before deleting the team.')
  success(yield* call(f.lead, 'project.task.delete', taskPayload))
  const ownerTaskId = success(yield* call(owner, 'project.task.create', { projectId: p.projectId, teamId: team.teamId, sprintId, worktreeId: p.worktreeId, owner: p.leadAgentId, title: 'Owner correction', state: 'queue' })).taskId
  success(yield* call(owner, 'project.task.delete', { projectId: p.projectId, taskId: ownerTaskId }))
  success(yield* call(owner, 'group', { agentId: 'builder', add: [team.group], remove: [] }))
  expect(failureOf(yield* call(f.lead, 'project.team.delete', teamPayload))?.reason).toBe('Stop the team’s agents before deleting the team.')
  const postId = success(yield* call(f.lead, 'board.post', { channelId: team.channelId, text: 'A preserved question', to: [{ _tag: 'owner' }], reply: 'all' })).postId
  success(yield* call(owner, 'stop', { agentId: 'builder' }))
  success(yield* call(f.lead, 'project.team.delete', teamPayload))
  expect(projectView(yield* snapshot).teams[projectKey(p.projectId, team.teamId)]).toBeUndefined()
  expect(failureOf(yield* call(owner, 'board.post', { channelId: team.channelId, text: 'Too late', to: [], reply: 'none' }))).toBeDefined()
  const events = (yield* readLog).map(({ event }) => event._tag)
  expect(((yield* snapshot).board as BoardView).posts[postId]).toMatchObject({ status: 'closed', outcome: 'withdrawn', text: 'A preserved question' })
  expect(success(yield* call(owner, 'board.channels', {})).items.find((row: any) => row.channelId === team.channelId)).toMatchObject({ archived: true })
  expect(events).toContain('project.TaskDeleted'); expect(events).toContain('project.TeamDeleted'); expect(events).toContain('board.ChannelArchived')
})).pipe(Effect.orDie))

for (const projectApproval of ['skip-planning', 'skip-planning-and-build']) it.live(`s10 addition: owner approves ${projectApproval} without sprints`, () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner), p = f.project
  yield* approve(f.lead, p.projectId, projectApproval, false)
  expect(projectView(yield* snapshot).projects[p.projectId]?.stage).toBe('define')
  yield* approve(f.lead, p.projectId, projectApproval)
  expect(projectView(yield* snapshot).projects[p.projectId]?.stage).toBe(projectApproval === 'skip-planning' ? 'plan' : 'build')
  success(yield* call(f.lead, 'project.team.create', { projectId: p.projectId, name: 'Build team', worktreeId: p.worktreeId }))
  const start = { name: 'Build lead', role: 'lead:builder', harness: 'cursor', account: { _tag: 'any' }, permission: 'ask', folder: projectView(yield* snapshot).workspaces[p.worktreeId]!.path }
  if (projectApproval === 'skip-planning') {
    expect(failureOf(yield* call(f.lead, 'start', start))?.reason).toBe('Starting work requires permission; use ask with projectApproval set to "build".')
    yield* approve(f.lead, p.projectId, 'build')
  }
  success(yield* call(f.lead, 'start', start))
  expect(projectView(yield* snapshot).projects[p.projectId]?.buildApproval).toBeDefined()
  expect(Object.values(projectView(yield* snapshot).sprints)).toHaveLength(0)
})).pipe(Effect.orDie))

it.live('s10 item 3: CLI project creation reaches the same lifecycle and its real MCP orientation is lead advice', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  yield* setup(runner)
  const { serve } = yield* Effect.promise(() => import('../../src/api/server.ts'))
  const { prepareRoot, startCli, finishCli, mcpPeer, agentEnvironment } = yield* Effect.promise(() => import('../../../testing/test/support/unit7b.ts'))
  const socket = `${rig.root}/engine.sock`, bearer = 'synthetic-owner-transport-credential'
  yield* serve({ socketPath: socket, ownerCredential: Redacted.make(bearer), debug: false })
  const root = prepareRoot()
  const child = startCli(root, ['project', 'create', '--input', JSON.stringify({ name: 'CLI product', goal: 'Agree on a useful product', machineId: 'local', repository: { _tag: 'new', name: 'CLI repository' } })], { ...agentEnvironment(root), LGTM_SOCKET: socket, LGTM_TOKEN: bearer })
  expect(yield* Effect.promise(() => finishCli(child)), child.stderr()).toEqual({ code: 0, signal: null })
  const projectId = JSON.parse(child.stdout()).projectId
  const spawned = runner.callsOf('start').at(-1)!.input
  expect(projectView(yield* snapshot).projects[projectId]?.leadAgentId).toBe(spawned.agentId)
  const mcp = startCli(root, ['mcp'], { ...agentEnvironment(root), LGTM_SOCKET: socket, LGTM_TOKEN: Redacted.value(spawned.token) })
  const peer = mcpPeer(mcp)
  yield* Effect.promise(() => peer.initialize())
  const response = yield* Effect.promise(() => peer.request('tools/call', { name: 'roles.orient', arguments: {} }))
  expect(response.result.isError).not.toBe(true)
  const orientation = unwrap({ data: response.result.content.map((part: any) => part.text ?? '').join('') })
  expect(orientation).toMatchObject({ name: 'Project lead', kind: 'lead', advice: expect.stringContaining('Start your own workers') })
})).pipe(Effect.orDie))

it.live('r1 item 1: the owner grants and withdraws project approvals directly, including historical projects', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner), projectId = f.project.projectId
  success(yield* call(owner, 'project.stage.set', { projectId, stage: 'review' }))
  for (const command of ['project.approval.grant', 'project.approval.withdraw']) {
    for (const actor of [f.lead, f.other, f.worker, { _tag: 'service', service: 'test.board' } as never]) {
      expect(failureOf(yield* call(actor, command, { projectId, approval: 'end-result' }))).toBeDefined()
    }
  }
  for (const approval of ['end-result', 'build']) {
    success(yield* call(owner, 'project.approval.grant', { projectId, approval }))
    expect(projectView(yield* snapshot).projects[projectId]?.stage).toBe('review')
  }
  expect(projectView(yield* snapshot).projects[projectId]?.buildApproval).toBeDefined()
  success(yield* call(owner, 'project.approval.withdraw', { projectId, approval: 'build' }))
  expect(projectView(yield* snapshot).projects[projectId]?.buildApproval).toBeUndefined()
  expect(projectView(yield* snapshot).projects[projectId]?.endResultApproval).toBeDefined()
  success(yield* call(owner, 'project.approval.grant', { projectId, approval: 'build' }))
  success(yield* call(owner, 'project.approval.withdraw', { projectId, approval: 'end-result' }))
  const project = projectView(yield* snapshot).projects[projectId]!
  expect(project.endResultApproval).toBeUndefined(); expect(project.skipPlanningApproval).toBeUndefined(); expect(project.buildApproval).toBeDefined()
  expect(project.stage).toBe('review')
  expect((yield* readLog).filter(({ event }) => event._tag === 'project.ApprovalGranted')).toHaveLength(3)
  expect((yield* readLog).filter(({ event }) => event._tag === 'project.PermissionWithdrawn')).toHaveLength(2)
})).pipe(Effect.orDie))

it.live('r1 item 1: ask settings without grants allow existing work to finish but refuse planning edits and new work', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner), p = f.project, projectId = p.projectId
  const sprintId = success(yield* call(owner, 'project.sprint.create', { projectId, label: 'SP-1', name: 'Historical', goal: 'Deliver', phase: 'Build', owner: p.leadAgentId, needs: [] })).sprintId
  const team = success(yield* call(owner, 'project.team.create', { projectId, name: 'Historical team', worktreeId: p.worktreeId }))
  success(yield* call(owner, 'group', { agentId: 'builder', add: [team.group], remove: [] }))
  const fields = { projectId, teamId: team.teamId, sprintId, worktreeId: p.worktreeId, owner: 'builder', title: 'Existing task' }
  const taskId = success(yield* call(owner, 'project.task.create', { ...fields, state: 'active' })).taskId
  for (const state of ['review', 'done']) success(yield* call(f.worker, 'project.task.update', { projectId, taskId, state, note: `Ready for ${state}; see the commit.` }))
  const edits = [
    ['project.task.update', { projectId, taskId, title: 'A title' }],
    ['project.task.note', { projectId, taskId, title: 'A note', detail: 'Some context' }],
    ['project.sprint.update', { projectId, sprintId, phase: 'Review' }]
  ] as const
  for (const [command, payload] of edits) expect(failureOf(yield* call(f.lead, command, payload))).toBeDefined()
  expect(failureOf(yield* call(f.lead, 'prompt', { agentId: 'builder', text: 'Build now' }))).toBeDefined()
  const queued = success(yield* call(owner, 'project.task.create', { ...fields, title: 'Queued task', state: 'queue' })).taskId
  const questionId = success(yield* call(f.worker, 'ask', { taskId: queued, text: 'Can I finish?' })).questionId
  expect(failureOf(yield* call(f.lead, 'answer', { kind: 'question', id: questionId, text: 'Yes' }))).toBeDefined()
  expect(projectView(yield* snapshot).tasks[projectKey(projectId, queued)]?.state).toBe('queue')
  expect(failureOf(yield* call(f.lead, 'project.task.create', { ...fields, title: 'New queued task', state: 'queue' }))).toBeDefined()
  yield* approve(f.lead, projectId, 'end-result')
  for (const [command, payload] of edits) success(yield* call(f.lead, command, payload))
  expect(failureOf(yield* call(f.lead, 'prompt', { agentId: 'builder', text: 'Build now' }))).toBeDefined()
  yield* approve(f.lead, projectId, 'build')
  success(yield* call(f.lead, 'prompt', { agentId: 'builder', text: 'Build now' }))
  success(yield* call(f.lead, 'answer', { kind: 'question', id: questionId, text: 'Yes' }))
  expect(projectView(yield* snapshot).tasks[projectKey(projectId, queued)]?.state).toBe('done')
})).pipe(Effect.orDie))

it.live('r2 item 1: a worker may note its own active task without approvals, while planning notes stay gated', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner), p = f.project, projectId = p.projectId
  const sprintId = success(yield* call(owner, 'project.sprint.create', { projectId, label: 'SP-1', name: 'Historical', goal: 'Deliver', phase: 'Build', owner: p.leadAgentId, needs: [] })).sprintId
  const team = success(yield* call(owner, 'project.team.create', { projectId, name: 'Historical team', worktreeId: p.worktreeId }))
  success(yield* call(owner, 'group', { agentId: 'builder', add: [team.group], remove: [] }))
  const fields = { projectId, teamId: team.teamId, sprintId, worktreeId: p.worktreeId, owner: 'builder', title: 'Existing task' }
  const taskId = success(yield* call(owner, 'project.task.create', { ...fields, state: 'active' })).taskId
  const note = { projectId, taskId, title: 'Ready', detail: 'See the commit.' }
  success(yield* call(f.worker, 'project.task.note', note))
  expect(projectView(yield* snapshot).tasks[projectKey(projectId, taskId)]?.history.at(-1)).toMatchObject({ title: note.title, detail: note.detail })
  expect(failureOf(yield* call(f.lead, 'project.task.note', note))).toBeDefined()
  expect(failureOf(yield* call(f.other, 'project.task.note', note))).toBeDefined()
  const queued = success(yield* call(owner, 'project.task.create', { ...fields, state: 'queue' })).taskId
  expect(failureOf(yield* call(f.worker, 'project.task.note', { ...note, taskId: queued }))).toBeDefined()
  expect(failureOf(yield* call(f.worker, 'project.task.update', { projectId, taskId: queued, state: 'active', note: 'Starting' }))?.reason).toBe('Starting work requires permission; use ask with projectApproval set to "build".')
})).pipe(Effect.orDie))

it.live('r1 item 1: answering a historical approval never moves Review backwards', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner), projectId = f.project.projectId
  success(yield* call(owner, 'project.stage.set', { projectId, stage: 'review' }))
  yield* approve(f.lead, projectId, 'end-result')
  expect(projectView(yield* snapshot).projects[projectId]?.stage).toBe('review')
  yield* approve(f.lead, projectId, 'build')
  expect(projectView(yield* snapshot).projects[projectId]?.stage).toBe('review')
})).pipe(Effect.orDie))

it.live('r1 item 2: approval effects and answer outcomes are written by the engine, including misleading prose', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner), projectId = f.project.projectId
  const cases = [
    ['end-result', 'Accepting grants Project planning. The lead can create sprints, teams and tasks.'],
    ['skip-planning', 'Accepting grants Project planning. The lead can create sprints, teams and tasks.'],
    ['skip-planning-and-build', 'Accepting grants Project planning and Starting work. The lead can create sprints, teams and tasks. The lead can start workers and move tasks out of the queue.']
  ] as const
  for (const [projectApproval, effect] of cases) {
    const id = success(yield* call(f.lead, 'ask', { projectId, projectApproval, proposedAnswer: 'Harmless', title: 'Quick check on naming', text: 'A harmless recommendation.' })).questionId
    const asked = (yield* readLog).map((row) => row.event).find((event) => event._tag === 'QuestionAsked' && event.questionId === id)!
    expect(asked).toMatchObject({ projectApprovalEffect: effect, proposedAnswer: 'Accepted' })
    success(yield* call(owner, 'answer', { kind: 'question', id, text: 'Yes, accepted. Go ahead.' }))
    const answer = (yield* readLog).map((row) => row.event).filter((event) => event._tag === 'DeliveryQueued' && event.delivery.target === f.project.leadAgentId).at(-1)!
    expect(JSON.stringify(answer)).toContain('Approval not recorded. Ask again if you still need it.')
    expect(projectView(yield* snapshot).projects[projectId]?.buildApproval).toBeUndefined()
  }
  yield* approve(f.lead, projectId, 'end-result')
  const id = success(yield* call(f.lead, 'ask', { projectId, projectApproval: 'build', text: 'Build the agreed result.' })).questionId
  expect((yield* readLog).map((row) => row.event).find((event) => event._tag === 'QuestionAsked' && event.questionId === id)).toMatchObject({ projectApprovalEffect: 'Accepting grants Starting work. The lead can start workers and move tasks out of the queue.' })
  success(yield* call(owner, 'answer', { kind: 'question', id, text: 'Accepted' }))
  const answer = (yield* readLog).map((row) => row.event).filter((event) => event._tag === 'DeliveryQueued' && event.delivery.target === f.project.leadAgentId).at(-1)!
  expect(JSON.stringify(answer)).toContain('Approval recorded.')
})).pipe(Effect.orDie))

it.live('r1 item 5: resume of an existing worker needs building approval after scope approval', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner), projectId = f.project.projectId
  yield* approve(f.lead, projectId, 'end-result')
  success(yield* call(owner, 'project.approval.grant', { projectId, approval: 'build' }))
  const folder = projectView(yield* snapshot).workspaces[f.project.worktreeId]!.path
  const agentId = success(yield* call(f.lead, 'start', { name: 'Existing builder', role: 'worker:builder', harness: 'cursor', account: { _tag: 'any' }, permission: 'ask', folder })).agentId
  success(yield* call(owner, 'stop', { agentId }))
  success(yield* call(owner, 'project.approval.withdraw', { projectId, approval: 'build' }))
  expect(failureOf(yield* call(f.lead, 'resume', { agentId }))?.reason).toBe('Starting work requires permission; use ask with projectApproval set to "build".')
  yield* approve(f.lead, projectId, 'build')
  success(yield* call(f.lead, 'resume', { agentId }))
})).pipe(Effect.orDie))

it.live('r1 item 5: a nonqueue task needs building approval after scope approval', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner), projectId = f.project.projectId
  yield* approve(f.lead, projectId, 'end-result')
  const sprintId = success(yield* call(f.lead, 'project.sprint.create', { projectId, label: 'SP-1', name: 'First', goal: 'Deliver', phase: 'Build', owner: f.project.leadAgentId, needs: [] })).sprintId
  const teamId = success(yield* call(f.lead, 'project.team.create', { projectId, name: 'Builders', worktreeId: f.project.worktreeId })).teamId
  const payload = { projectId, sprintId, teamId, worktreeId: f.project.worktreeId, title: 'Start immediately', owner: f.project.leadAgentId, state: 'active' }
  expect(failureOf(yield* call(f.lead, 'project.task.create', payload))?.reason).toBe('Starting work requires permission; use ask with projectApproval set to "build".')
  yield* approve(f.lead, projectId, 'build')
  success(yield* call(f.lead, 'project.task.create', payload))
})).pipe(Effect.orDie))

it.live('r1 item 5: setting Build needs building approval after scope approval', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner), projectId = f.project.projectId
  yield* approve(f.lead, projectId, 'end-result')
  expect(failureOf(yield* call(f.lead, 'project.stage.set', { projectId, stage: 'build' }))?.reason).toBe('Starting work requires permission; use ask with projectApproval set to "build".')
  expect(projectView(yield* snapshot).projects[projectId]?.stage).toBe('plan')
  yield* approve(f.lead, projectId, 'build')
  success(yield* call(f.lead, 'project.stage.set', { projectId, stage: 'build' }))
})).pipe(Effect.orDie))

it.live('r1 item 5: historical project-lead spelling gets lead orientation', () => provideBoard(rig, () => Effect.gen(function* () {
  yield* registerRebuildCheck()
  const legacy = yield* seedAgent({ id: 'historical-lead', name: 'Project lead', role: 'project-lead', state: 'idle' })
  expect(unwrap(success(yield* call(agentCredential(legacy.token), 'roles.orient', {})))).toMatchObject({ kind: 'lead', roleName: 'project-lead', advice: expect.stringContaining('Start your own workers') })
})).pipe(Effect.orDie))

it.live('r1 item 6: the opening distinguishes fields, controls, denial and building authority', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  yield* setup(runner)
  const started = (yield* readLog).find(({ event }) => event._tag === 'AgentStarted' && event.firstPromptDelivery)!.event
  const text = JSON.stringify(started)
  for (const phrase of ['During Define, gather context and size the work', 'in the project chat', 'text field', 'Details control', 'If the owner denies', 'ask what to change', 'unchanged', 'without projectApproval', 'They do not replace', 'do not attempt that action again', 'project.stage.set']) expect(text).toContain(phrase)
  expect(text).not.toContain('Size the work first')
})).pipe(Effect.orDie))

it.live('r1 item 8: deleted queued tasks keep readable, marked history and remain unavailable for writes', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner), projectId = f.project.projectId
  yield* approve(f.lead, projectId, 'end-result')
  const sprintId = success(yield* call(f.lead, 'project.sprint.create', { projectId, label: 'SP-1', name: 'First', goal: 'Deliver', phase: 'Build', owner: f.project.leadAgentId, needs: [] })).sprintId
  const team = success(yield* call(f.lead, 'project.team.create', { projectId, name: 'Builders', worktreeId: f.project.worktreeId }))
  const taskId = success(yield* call(f.lead, 'project.task.create', { projectId, sprintId, teamId: team.teamId, worktreeId: f.project.worktreeId, title: 'Remember this task', owner: f.project.leadAgentId, state: 'queue' })).taskId
  success(yield* call(f.lead, 'project.task.note', { projectId, taskId, title: 'Context', detail: 'Keep these owner decisions.' }))
  success(yield* call(f.lead, 'project.task.delete', { projectId, taskId }))
  success(yield* call(f.lead, 'project.team.delete', { projectId, teamId: team.teamId }))
  const history = success(yield* call(owner, 'project.task.history', { projectId, taskId })).items
  expect(history).toEqual(expect.arrayContaining([expect.objectContaining({ detail: 'Keep these owner decisions.' }), expect.objectContaining({ title: 'Task deleted', deleted: true })]))
  expect(unwrap(success(yield* call(f.lead, 'project.task.history', { projectId, taskId })))).toMatchObject({ items: expect.arrayContaining([expect.objectContaining({ deleted: true })]) })
  expect(success(yield* call(owner, 'project.read', { projectId, collection: 'tasks' })).items).toHaveLength(0)
  expect(failureOf(yield* call(f.lead, 'project.task.update', { projectId, taskId, title: 'Resurrect' }))).toBeDefined()
  expect(failureOf(yield* call(f.other, 'project.task.history', { projectId, taskId }))).toBeDefined()
})).pipe(Effect.orDie))

it.live('r1 item 8: deleting a stopped team removes only its group from the agent views', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner), projectId = f.project.projectId
  yield* approve(f.lead, projectId, 'end-result')
  const team = success(yield* call(f.lead, 'project.team.create', { projectId, name: 'Builders', worktreeId: f.project.worktreeId }))
  success(yield* call(owner, 'group', { agentId: 'builder', add: [team.group, 'unrelated'], remove: [] }))
  success(yield* call(owner, 'stop', { agentId: 'builder' }))
  success(yield* call(f.lead, 'project.team.delete', { projectId, teamId: team.teamId }))
  expect((yield* snapshot).agents.builder!.groups).toEqual(['unrelated'])
  expect(((yield* snapshot).board as BoardView).labels.builder).toEqual(['unrelated'])
  const roster = success(yield* call(owner, 'client.snapshot', {})).agents.find((a: any) => a.agentId === 'builder')
  expect(roster.groups).toEqual(['unrelated'])
})).pipe(Effect.orDie))

for (const text of ['No', 'Accepted']) it.live(`r2 item 2: an open starting-work request is independent of planning withdrawal with ${text}`, () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner), projectId = f.project.projectId
  yield* approve(f.lead, projectId, 'end-result')
  const id = success(yield* call(f.lead, 'ask', { projectId, projectApproval: 'build', text: 'Start building?' })).questionId
  success(yield* call(owner, 'project.approval.withdraw', { projectId, approval: 'end-result' }))
  success(yield* call(owner, 'answer', { kind: 'question', id, text }))
  const expected = text === 'Accepted'
    ? 'Approval recorded.'
    : 'Approval not recorded. Ask again if you still need it.'
  expect((yield* snapshot).questions[id]).toMatchObject({ status: 'answered', projectApprovalOutcome: expected })
  expect(projectView(yield* snapshot).projects[projectId]?.buildApproval !== undefined).toBe(text === 'Accepted')
  const rows = yield* readLog
  expect(rows.filter(({ event }) => event._tag === 'project.PermissionDecided' && event.questionId === id && event.granted.includes('starting-work'))).toHaveLength(text === 'Accepted' ? 1 : 0)
  const delivery = rows.map(({ event }) => event).filter((event) => event._tag === 'DeliveryQueued' && event.delivery.target === f.project.leadAgentId).at(-1)!
  expect(JSON.stringify(delivery)).toContain(expected)
  expect(success(yield* call(owner, 'client.requests', { after: 0, limit: 200 })).items.find((row: any) => row.id === id)).toMatchObject({ projectApprovalOutcome: expected })
})).pipe(Effect.orDie))

for (const projectApproval of ['end-result', 'build', 'skip-planning-and-build']) it.live(`r2 item 5: a direct grant stays intact but a declined ${projectApproval} card records no grant`, () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner), projectId = f.project.projectId
  if (projectApproval === 'build') success(yield* call(owner, 'project.approval.grant', { projectId, approval: 'end-result' }))
  const id = success(yield* call(f.lead, 'ask', { projectId, projectApproval, text: 'Approve the next step?' })).questionId
  for (const approval of projectApproval === 'skip-planning-and-build' ? ['end-result', 'build'] : [projectApproval]) success(yield* call(owner, 'project.approval.grant', { projectId, approval }))
  success(yield* call(owner, 'answer', { kind: 'question', id, text: 'Handled in settings.' }))
  expect((yield* snapshot).questions[id]).toMatchObject({ status: 'answered', projectApprovalOutcome: 'Approval not recorded. Ask again if you still need it.' })
  expect(projectView(yield* snapshot).projects[projectId]?.[projectApproval === 'build' ? 'buildApproval' : 'endResultApproval']).toBeDefined()
  const delivery = (yield* readLog).map(({ event }) => event).filter((event) => event._tag === 'DeliveryQueued' && event.delivery.target === f.project.leadAgentId).at(-1)!
  expect(JSON.stringify(delivery)).toContain('Approval not recorded. Ask again if you still need it.')
  expect(JSON.stringify(delivery)).not.toContain('Approval recorded.')
  expect(success(yield* call(owner, 'client.requests', { after: 0, limit: 200 })).items.find((row: any) => row.id === id)).toMatchObject({ projectApprovalOutcome: 'Approval not recorded. Ask again if you still need it.' })
})).pipe(Effect.orDie))

it.live('r2 item 5: ask refuses the engine-owned approval effect field', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner)
  expect(failureOf(yield* call(f.lead, 'ask', { projectId: f.project.projectId, projectApproval: 'end-result', text: 'Approve?', projectApprovalEffect: 'Only renames a file.' }))).toBeDefined()
  expect(Object.values((yield* snapshot).questions)).toHaveLength(0)
})).pipe(Effect.orDie))

const setPermission = (projectId: string, permission: string, setting: string) => call(owner, 'project.permission.set', { projectId, permission, setting })
const sprintInput = (p: any) => ({ projectId: p.projectId, label: 'SP-1', name: 'First', goal: 'Deliver', phase: 'Build', owner: p.leadAgentId, needs: [] })
const workerInput = (folder: string) => ({ name: 'Builder', role: 'worker:builder', harness: 'cursor', account: { _tag: 'any' }, permission: 'ask', folder })
for (const historical of [false, true]) it.live(`s11: ${historical ? 'existing' : 'new'} project defaults to allowed without a card`, () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner, 'default'), p = f.project
  if (historical) { const { stage: _stage, ...legacy } = p; yield* appendSetup([eventOf({ _tag: 'project.Created', ...legacy })]) }
  expect(projectView(yield* snapshot).projects[p.projectId]?.stage).toBe(historical ? 'plan' : 'define')
  success(yield* call(f.lead, 'project.sprint.create', sprintInput(p)))
  success(yield* call(f.lead, 'start', workerInput(rig.work)))
  expect((yield* readLog).filter(({ event }) => event._tag === 'QuestionAsked')).toHaveLength(0)
})).pipe(Effect.orDie))

it.live('s11: only the owner can change either permission setting', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner, 'default'), projectId = f.project.projectId
  for (const permission of ['planning', 'starting-work']) {
    for (const actor of [f.lead, f.worker, f.other, { _tag: 'service', service: 'test.board' } as never]) expect(failureOf(yield* call(actor, 'project.permission.set', { projectId, permission, setting: 'denied' }))).toBeDefined()
    success(yield* setPermission(projectId, permission, 'denied'))
  }
  expect((yield* readLog).filter(({ event }) => event._tag === 'project.PermissionChanged')).toHaveLength(2)
})).pipe(Effect.orDie))

it.live('s11: not allowed refuses the same actions and approval requests with the same reason', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner, 'default'), p = f.project, projectId = p.projectId
  const sprintId = success(yield* call(owner, 'project.sprint.create', sprintInput(p))).sprintId
  const teamId = success(yield* call(owner, 'project.team.create', { projectId, name: 'Build team', worktreeId: p.worktreeId })).teamId
  const task = { projectId, sprintId, teamId, worktreeId: p.worktreeId, owner: p.leadAgentId, title: 'Build', state: 'queue' }
  const taskId = success(yield* call(owner, 'project.task.create', task)).taskId
  const pausedAgentId = success(yield* call(f.lead, 'start', workerInput(rig.work))).agentId
  success(yield* call(owner, 'stop', { agentId: pausedAgentId }))
  success(yield* call(owner, 'group', { agentId: 'builder', add: [projectView(yield* snapshot).teams[projectKey(projectId, teamId)]!.group], remove: [] }))
  const queued = success(yield* call(owner, 'project.task.create', { ...task, owner: 'builder' })).taskId
  const questionId = success(yield* call(f.worker, 'ask', { taskId: queued, text: 'Finished?' })).questionId
  for (const permission of ['planning', 'starting-work']) success(yield* setPermission(projectId, permission, 'denied'))
  const planning = 'The owner has turned off Project planning. This permission cannot be requested.'
  const starting = 'The owner has turned off Starting work. This permission cannot be requested.'
  for (const [command, payload] of [
    ['project.sprint.create', sprintInput(p)], ['project.sprint.update', { projectId, sprintId, name: 'Changed' }],
    ['project.sprint.complete', { projectId, sprintId, completion: 'Complete', evidence: 'Checked' }],
    ['project.team.create', { projectId, name: 'Another team', worktreeId: p.worktreeId }], ['project.task.create', task],
    ['project.task.update', { projectId, taskId, title: 'Changed' }], ['project.task.note', { projectId, taskId, title: 'Plan', detail: 'Changed' }],
    ['project.stage.set', { projectId, stage: 'plan' }], ['ask', { projectId, projectApproval: 'end-result', text: 'Allow planning?' }],
    ['ask', { projectId, projectApproval: 'skip-planning-and-build', text: 'Allow both?' }]
  ] as const) expect(failureOf(yield* call(f.lead, command, payload))?.reason, command).toBe(planning)
  // With planning allowed, all starting gates still refuse independently.
  success(yield* setPermission(projectId, 'planning', 'allowed'))
  for (const [command, payload] of [
    ['start', workerInput(rig.work)], ['resume', { agentId: pausedAgentId }], ['prompt', { agentId: 'builder', text: 'Build' }],
    ['answer', { kind: 'question', id: questionId, text: 'Yes' }],
    ['ask', { projectId, projectApproval: 'skip-planning-and-build', text: 'Allow both?' }],
    ['project.task.create', { ...task, state: 'active' }], ['project.task.update', { projectId, taskId, state: 'active' }],
    ['project.stage.set', { projectId, stage: 'build' }], ['ask', { projectId, projectApproval: 'build', text: 'Allow work?' }]
  ] as const) expect(failureOf(yield* call(f.lead, command, payload))?.reason, command).toBe(starting)
  success(yield* call(owner, 'project.task.update', { projectId, taskId, state: 'active' }))
  success(yield* call(owner, 'project.sprint.create', { ...sprintInput(p), label: 'SP-2' }))
})).pipe(Effect.orDie))

it.live('s11: starting work can be granted first and survives planning withdrawal or setting changes', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner), p = f.project, projectId = p.projectId
  yield* approve(f.lead, projectId, 'build')
  success(yield* call(f.lead, 'start', workerInput(rig.work)))
  expect(failureOf(yield* call(f.lead, 'project.sprint.create', sprintInput(p)))).toBeDefined()
  yield* approve(f.lead, projectId, 'end-result')
  success(yield* call(owner, 'project.approval.withdraw', { projectId, approval: 'end-result' }))
  expect(projectView(yield* snapshot).projects[projectId]?.buildApproval).toBeDefined()
  success(yield* setPermission(projectId, 'planning', 'denied'))
  expect(projectView(yield* snapshot).projects[projectId]?.buildApproval).toBeDefined()
  success(yield* setPermission(projectId, 'starting-work', 'allowed'))
  success(yield* setPermission(projectId, 'starting-work', 'ask'))
  expect(projectView(yield* snapshot).projects[projectId]?.buildApproval).toBeUndefined()
  expect(failureOf(yield* call(f.lead, 'start', workerInput(rig.work)))).toBeDefined()
  expect(projectView(yield* snapshot).projects[projectId]?.stage).toBe('build')
})).pipe(Effect.orDie))

it.live('s11: replay retains all three legacy approval event types', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner, 'default'), projectId = f.project.projectId
  yield* appendSetup([eventOf({ _tag: 'project.ApprovalGranted', projectId, approval: 'end-result', requestId: 'old-grant' }),
    eventOf({ _tag: 'project.ApprovalDecided', projectId, approval: 'build', questionId: 'old-question', approved: true })])
  expect(projectView(yield* snapshot).projects[projectId]).toMatchObject({ endResultApproval: 'old-grant', buildApproval: 'old-question' })
  yield* appendSetup([eventOf({ _tag: 'project.ApprovalWithdrawn', projectId, approval: 'build' })])
  expect(projectView(yield* snapshot).projects[projectId]?.buildApproval).toBeUndefined()
  expect(projectView(yield* snapshot).projects[projectId]?.endResultApproval).toBe('old-grant')
})).pipe(Effect.orDie))

for (const permission of ['planning', 'starting-work']) it.live(`s11: changing or withdrawing ${permission} invalidates an earlier card`, () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner), projectId = f.project.projectId
  const projectApproval = permission === 'planning' ? 'end-result' : 'build'
  const id = success(yield* call(f.lead, 'ask', { projectId, projectApproval, text: 'Allow the next step?' })).questionId
  success(yield* setPermission(projectId, permission, 'denied'))
  success(yield* setPermission(projectId, permission, 'ask'))
  expect(failureOf(yield* call(owner, 'answer', { kind: 'question', id, text: 'Accepted' }))).toBeDefined()
  expect((yield* snapshot).questions[id]?.projectApprovalOutcome).toBe('Approval not recorded. Ask again if you still need it.')
  const fresh = yield* approve(f.lead, projectId, projectApproval)
  expect((yield* snapshot).questions[fresh]?.projectApprovalOutcome).toBe('Approval recorded.')
  success(yield* call(owner, 'project.approval.withdraw', { projectId, approval: projectApproval }))
  const next = success(yield* call(f.lead, 'ask', { projectId, projectApproval, text: 'Ask again?' })).questionId
  success(yield* call(owner, 'project.approval.withdraw', { projectId, approval: projectApproval }))
  expect(failureOf(yield* call(owner, 'answer', { kind: 'question', id: next, text: 'Accepted' }))).toBeDefined()
  expect((yield* snapshot).questions[next]?.projectApprovalOutcome).toBe('Approval not recorded. Ask again if you still need it.')
})).pipe(Effect.orDie))

it.live('s11: off permissions still let a worker annotate and finish its own started task', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner, 'default'), p = f.project, projectId = p.projectId
  const sprintId = success(yield* call(owner, 'project.sprint.create', sprintInput(p))).sprintId
  const team = success(yield* call(owner, 'project.team.create', { projectId, name: 'Build team', worktreeId: p.worktreeId }))
  success(yield* call(owner, 'group', { agentId: 'builder', add: [team.group], remove: [] }))
  const taskId = success(yield* call(owner, 'project.task.create', { projectId, sprintId, teamId: team.teamId, owner: 'builder', worktreeId: p.worktreeId, title: 'Started', state: 'active' })).taskId
  for (const permission of ['planning', 'starting-work']) success(yield* setPermission(projectId, permission, 'denied'))
  success(yield* call(f.worker, 'project.task.note', { projectId, taskId, title: 'Progress', detail: 'Checked the change.' }))
  success(yield* call(f.worker, 'project.task.update', { projectId, taskId, state: 'done', note: 'Ready.' }))
})).pipe(Effect.orDie))

for (const changed of ['planning', 'starting-work']) it.live(`s11: a setting change closes a combined card when ${changed} changes`, () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner), projectId = f.project.projectId
  const id = success(yield* call(f.lead, 'ask', { projectId, projectApproval: 'skip-planning-and-build', text: 'Allow both?' })).questionId
  success(yield* setPermission(projectId, changed, 'denied'))
  expect(failureOf(yield* call(owner, 'answer', { kind: 'question', id, text: 'Accepted' }))).toBeDefined()
  const project = projectView(yield* snapshot).projects[projectId]!
  expect(project.endResultApproval).toBeUndefined()
  expect(project.buildApproval).toBeUndefined()
  expect((yield* snapshot).questions[id]).toMatchObject({ status: 'answered', answer: 'Permission request closed because the owner changed its setting.' })
  yield* approve(f.lead, projectId, changed === 'planning' ? 'build' : 'end-result')
  expect(projectView(yield* snapshot).projects[projectId]?.[changed === 'planning' ? 'buildApproval' : 'endResultApproval']).toBeDefined()
})).pipe(Effect.orDie))

it.live('s11 r1: default planning and worker start advance stages in their command batches', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner, 'default'), p = f.project, projectId = p.projectId
  const planned = success(yield* call(f.lead, 'project.sprint.create', sprintInput(p)))
  expect(projectView(yield* snapshot).projects[projectId]?.stage).toBe('plan')
  expect((yield* readLog).filter((row) => row.requestId === planned.requestId).map((row) => row.event)).toContainEqual({ _tag: 'project.StageChanged', projectId, stage: 'plan' })
  const built = success(yield* call(f.lead, 'start', workerInput(rig.work)))
  expect(projectView(yield* snapshot).projects[projectId]?.stage).toBe('build')
  expect((yield* readLog).filter((row) => row.requestId === built.requestId).map((row) => row.event)).toContainEqual({ _tag: 'project.StageChanged', projectId, stage: 'build' })
  success(yield* call(f.lead, 'project.team.create', { projectId, name: 'Another team', worktreeId: p.worktreeId }))
  expect(projectView(yield* snapshot).projects[projectId]?.stage).toBe('build')
  success(yield* call(owner, 'project.stage.set', { projectId, stage: 'review' }))
  success(yield* call(f.lead, 'start', workerInput(rig.work)))
  success(yield* call(f.lead, 'project.sprint.create', { ...sprintInput(p), label: 'SP-2' }))
  expect(projectView(yield* snapshot).projects[projectId]?.stage).toBe('review')
})).pipe(Effect.orDie))

it.live('s11 r1: tasks, teams and resumed workers advance stages without rewriting old events', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner, 'default'), p = f.project, projectId = p.projectId
  const teamId = success(yield* call(f.lead, 'project.team.create', { projectId, name: 'Builders', worktreeId: p.worktreeId })).teamId
  expect(projectView(yield* snapshot).projects[projectId]?.stage).toBe('plan')
  const sprintId = success(yield* call(f.lead, 'project.sprint.create', sprintInput(p))).sprintId
  const task = { projectId, sprintId, teamId, worktreeId: p.worktreeId, owner: p.leadAgentId, title: 'Build', state: 'queue' }
  success(yield* call(owner, 'project.stage.set', { projectId, stage: 'define' }))
  const taskId = success(yield* call(f.lead, 'project.task.create', task)).taskId
  expect(projectView(yield* snapshot).projects[projectId]?.stage).toBe('plan')
  success(yield* call(f.lead, 'project.task.update', { projectId, taskId, state: 'active' }))
  expect(projectView(yield* snapshot).projects[projectId]?.stage).toBe('build')
  const agentId = success(yield* call(f.lead, 'start', workerInput(rig.work))).agentId
  success(yield* call(owner, 'stop', { agentId }))
  success(yield* call(owner, 'project.stage.set', { projectId, stage: 'define' }))
  success(yield* call(f.lead, 'resume', { agentId }))
  expect(projectView(yield* snapshot).projects[projectId]?.stage).toBe('build')
  success(yield* call(owner, 'project.stage.set', { projectId, stage: 'define' }))
  success(yield* call(f.lead, 'project.task.create', { ...task, state: 'active' }))
  expect(projectView(yield* snapshot).projects[projectId]?.stage).toBe('build')
  // Stored planning events keep their prior projection during replay.
  success(yield* call(owner, 'project.stage.set', { projectId, stage: 'define' }))
  yield* appendSetup([eventOf({ _tag: 'project.SprintAdded', ...sprintInput(p), sprintId: 'legacy-sprint', label: 'OLD' })])
  expect(projectView(yield* snapshot).projects[projectId]?.stage).toBe('define')
})).pipe(Effect.orDie))

for (const allowed of ['planning', 'starting-work']) it.live(`s11 r1: a combined card only names and grants the permission beside Allowed ${allowed}`, () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner), projectId = f.project.projectId
  success(yield* setPermission(projectId, allowed, 'allowed'))
  const id = yield* approve(f.lead, projectId, 'skip-planning-and-build')
  const row = (yield* snapshot).questions[id]!
  const expected = allowed === 'planning' ? 'Starting work' : 'Project planning'
  expect(row.projectApprovalEffect).toContain('Accepting grants ' + expected + '.')
  expect(row.projectApprovalEffect).not.toContain(allowed === 'planning' ? 'Project planning' : 'Starting work')
  expect(row.projectApprovalOutcome).toBe('Approval recorded.')
  expect(projectView(yield* snapshot).projects[projectId]?.[allowed === 'planning' ? 'buildApproval' : 'endResultApproval']).toBeDefined()
})).pipe(Effect.orDie))

it.live('s11 r1: settings close affected cards with a fixed note and refuse later answers', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner), projectId = f.project.projectId
  const ordinary = success(yield* call(f.lead, 'ask', { text: 'An ordinary question' })).questionId
  const planning = success(yield* call(f.lead, 'ask', { projectId, projectApproval: 'end-result', text: 'Plan?' })).questionId
  const starting = success(yield* call(f.lead, 'ask', { projectId, projectApproval: 'build', text: 'Start?' })).questionId
  const changed = success(yield* setPermission(projectId, 'planning', 'allowed'))
  const row = (yield* snapshot).questions[planning]!
  expect(row).toMatchObject({ status: 'answered', answer: 'Permission request closed because the owner changed its setting.', projectApprovalOutcome: 'Approval not recorded. Ask again if you still need it.' })
  expect((yield* readLog).filter((r) => r.requestId === changed.requestId).some(({ event }) => event._tag === 'QuestionAnswered' && event.questionId === planning)).toBe(true)
  for (const text of ['Accepted', 'No, do not plan yet.']) expect(failureOf(yield* call(owner, 'answer', { kind: 'question', id: planning, text }))).toBeDefined()
  expect((yield* snapshot).questions[starting]?.status).toBe('open')
  expect((yield* snapshot).questions[ordinary]?.status).toBe('open')
  success(yield* setPermission(projectId, 'starting-work', 'denied'))
  expect((yield* snapshot).questions[starting]?.status).toBe('answered')
  success(yield* setPermission(projectId, 'starting-work', 'ask'))
  yield* approve(f.lead, projectId, 'build')
})).pipe(Effect.orDie))

it.live('s11 r1: legacy end-result withdrawal retains its cascade while new withdrawals stay independent', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner), projectId = f.project.projectId
  yield* appendSetup([eventOf({ _tag: 'project.ApprovalGranted', projectId, approval: 'end-result', requestId: 'old-scope' }),
    eventOf({ _tag: 'project.ApprovalGranted', projectId, approval: 'build', requestId: 'old-work' }),
    eventOf({ _tag: 'project.ApprovalWithdrawn', projectId, approval: 'end-result' })])
  expect(projectView(yield* snapshot).projects[projectId]?.endResultApproval).toBeUndefined()
  expect(projectView(yield* snapshot).projects[projectId]?.buildApproval).toBeUndefined()
  yield* approve(f.lead, projectId, 'skip-planning-and-build')
  success(yield* call(owner, 'project.approval.withdraw', { projectId, approval: 'end-result' }))
  expect(projectView(yield* snapshot).projects[projectId]?.buildApproval).toBeDefined()
})).pipe(Effect.orDie))

it.live('s11 r1: declining a historical card after Allowed records no grant by the card', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner), projectId = f.project.projectId
  const id = success(yield* call(f.lead, 'ask', { projectId, projectApproval: 'end-result', text: 'Plan?' })).questionId
  // Pre-r1 stored setting events did not carry question closures.
  yield* appendSetup([eventOf({ _tag: 'project.PermissionChanged', projectId, permission: 'planning', setting: 'allowed' })])
  success(yield* call(owner, 'answer', { kind: 'question', id, text: 'No, do not plan yet.' }))
  expect((yield* snapshot).questions[id]?.projectApprovalOutcome).toBe('Approval not recorded. Ask again if you still need it.')
})).pipe(Effect.orDie))

it.live('s11 r1: a historical combined card reports a grant of its unchanged permission', () => provideBoard(rig, (runner) => Effect.gen(function* () {
  const f = yield* setup(runner), projectId = f.project.projectId
  const id = success(yield* call(f.lead, 'ask', { projectId, projectApproval: 'skip-planning-and-build', text: 'Allow both?' })).questionId
  yield* appendSetup([eventOf({ _tag: 'project.PermissionChanged', projectId, permission: 'planning', setting: 'denied' })])
  success(yield* call(owner, 'answer', { kind: 'question', id, text: 'Accepted' }))
  expect((yield* snapshot).questions[id]?.projectApprovalOutcome).toBe('Approval recorded.')
  expect(projectView(yield* snapshot).projects[projectId]?.endResultApproval).toBeUndefined()
  expect(projectView(yield* snapshot).projects[projectId]?.buildApproval).toBeDefined()
})).pipe(Effect.orDie))
