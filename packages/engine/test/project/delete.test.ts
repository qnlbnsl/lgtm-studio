import { clientView } from '../../src/client/alerts/view.ts'
import { engineSource } from '../../src/client/alerts/sources.ts'
import { execFileSync } from 'node:child_process'
import { existsSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { it } from '@effect/vitest'
import * as Effect from 'effect/Effect'
import { afterEach, beforeEach, expect, vi } from 'vitest'
import type { BoardView } from '@lgtm/contracts'
import { cleanupUnit7b } from '../../../testing/test/support/unit7b.ts'
import { provideBoard, snapshot } from '../board/wakeFixtures.ts'
import { decisionProject, decisionSetup } from '../support/projectDecisionFixtures.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
import { appendSetup, call, eventOf, failureOf, freshRequest, makeRig, ownerCredential as owner, readLog, removeRig, success, type Rig } from '../support/handlerRig.ts'
import { projectView } from '../../src/project/view.ts'
import { deletedEvent } from '../../src/project/deleted.ts'
import { visibleTo } from '../../src/api/visibility.ts'
import { unwrap } from '../client/support/readRig.ts'

let rig: Rig
beforeEach(() => { rig = makeRig(); vi.stubEnv('HOME', rig.home); vi.stubEnv('LGTM_HOME', rig.stateFolder) })
afterEach(() => { cleanupUnit7b(); vi.unstubAllEnvs(); removeRig(rig) })

it.live('only the owner can delete, after a completed archive and an explicit confirmation', () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  const p = yield* decisionSetup(), other = yield* decisionProject('other')
  const payload = { projectId: p.projectId, confirmed: true }
  expect(failureOf(yield* call(owner, 'project.delete', payload))?.reason).toBe('Archive the project before deleting it.')
  for (const credential of [p.leadCredential, p.workerCredential, other.leadCredential, { _tag: 'service', service: 'test.board' } as never]) expect(failureOf(yield* call(credential, 'project.delete', payload))).toBeDefined()
  success(yield* call(owner, 'project.archive', { projectId: p.projectId }))
  for (const credential of [p.leadCredential, p.workerCredential, other.leadCredential, { _tag: 'service', service: 'test.board' } as never]) {
    expect(failureOf(yield* call(credential, 'project.delete', payload))).toBeDefined()
  }
  expect(failureOf(yield* call(owner, 'project.delete', { projectId: p.projectId }))).toBeDefined()
  expect(failureOf(yield* call(owner, 'project.delete', { ...payload, confirmed: false }))).toBeDefined()
  const current = projectView(yield* snapshot).projects[p.projectId]!
  yield* appendSetup([eventOf({ _tag: 'project.Created', ...current, archiving: true })])
  expect(failureOf(yield* call(owner, 'project.delete', payload))?.reason).toBe('Archive the project before deleting it.')
  yield* appendSetup([eventOf({ _tag: 'project.Created', ...current })])
  success(yield* call(owner, 'project.delete', payload))
  expect(projectView(yield* snapshot).projects[p.projectId]).toBeUndefined()
  expect(projectView(yield* snapshot).projects[other.projectId]).toBeDefined()
})).pipe(Effect.orDie))

it.live('refuses deletion if an archived project has an agent that is not stopped', () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  const p = yield* decisionSetup()
  yield* appendSetup([eventOf({ _tag: 'project.Archived', projectId: p.projectId })])
  expect(failureOf(yield* call(owner, 'project.delete', { projectId: p.projectId, confirmed: true }))?.reason).toBe('Stop every project agent before deleting the project.')
})).pipe(Effect.orDie))

it.live('deletion removes every project view and read, preserves events, branches and dirty folders, and replays identically', () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  const p = yield* decisionSetup(), other = yield* decisionProject('other')
  execFileSync('git', ['init', '-b', 'topic', rig.work], { stdio: 'ignore' })
  execFileSync('git', ['-C', rig.work, '-c', 'user.name=Maya Okafor', '-c', 'user.email=maya@example.test', '-c', 'commit.gpgsign=false', 'commit', '--allow-empty', '-m', 'Initial'], { stdio: 'ignore' })
  writeFileSync(join(rig.work, 'uncommitted.txt'), 'Keep this file')
  yield* appendSetup([eventOf({ _tag: 'project.WorktreeCreated', repoId: p.repoId, worktreeId: p.worktreeId, machineId: 'local', branch: 'topic', path: rig.work })])
  const taskId = success(yield* call(p.leadCredential, 'project.task.create', p.task)).taskId
  const questionId = success(yield* call(p.workerCredential, 'ask', { projectId: p.projectId, text: 'A private project question' })).questionId
  success(yield* call(p.leadCredential, 'board.post', { channelId: p.team.channelId, text: 'Project board history', to: [], reply: 'none' }))
  // A direct chat and a shared channel test ownership and preservation separately.
  yield* appendSetup([
    eventOf({ _tag: 'board.ChannelOpened', channelId: 'project-dm', name: 'project-chat', kind: 'direct', opener: { _tag: 'owner' }, members: [{ _tag: 'owner' }, { _tag: 'agent', agentId: p.leadId }] }),
    eventOf({ _tag: 'board.ChannelOpened', channelId: 'shared', name: 'Shared', kind: 'plain', opener: { _tag: 'owner' }, manager: { _tag: 'owner' }, members: [{ _tag: 'owner' }, { _tag: 'agent', agentId: p.workerId }, { _tag: 'agent', agentId: other.leadId }] })
  ])
  success(yield* call(owner, 'board.post', { channelId: 'project-dm', text: 'Owner chat history', to: [], reply: 'none' }))
  success(yield* call(owner, 'board.post', { channelId: 'shared', text: 'Shared history', to: [], reply: 'none' }))
  yield* appendSetup([
    eventOf({ _tag: 'project.SprintCompleted', projectId: p.projectId, sprintId: p.sprintId, completion: 'Done', evidence: 'Tests passed' }),
    eventOf({ _tag: 'client.AlertRaised', alertId: 'project-sprint-alert', alertKey: 'sprint:complete', kind: 'sprint', subject: { _tag: 'piece', namespace: 'project', id: p.sprintId }, title: 'Project sprint finished', sourceSequence: 1 }),
    eventOf({ _tag: 'client.AlertRaised', alertId: 'project-agent-alert', alertKey: 'agent:question', kind: 'question', agentId: p.workerId, title: 'Project needs you', sourceSequence: 1 }),
    eventOf({ _tag: 'client.AlertRaised', alertId: 'shared-alert', alertKey: 'shared:account', kind: 'account', title: 'Shared account needs you', sourceSequence: 1 }),
    eventOf({ _tag: 'client.AlertDelivered', alertId: 'project-sprint-alert', carrier: 'ntfy', generation: 0, attempt: 1 })
  ])
  success(yield* call(owner, 'project.archive', { projectId: p.projectId }))
  const before = yield* snapshot, prior = yield* readLog
  const requestId = freshRequest()
  success(yield* call(owner, 'project.delete', { projectId: p.projectId, confirmed: true, requestId }))
  success(yield* call(owner, 'project.delete', { projectId: p.projectId, confirmed: true, requestId }))
  const now = yield* snapshot, view = projectView(now), board = now.board as BoardView
  for (const id of [p.leadId, p.workerId]) { expect(now.agents[id]).toBeUndefined(); expect(now.threads[id]).toBeUndefined() }
  expect(now.questions[questionId]).toBeUndefined()
  for (const collection of [view.projects, view.teams, view.sprints, view.tasks, view.deletedTasks ?? {}, view.gitReviews, view.mockRevisions])
    expect(Object.values(collection).some((row) => row.projectId === p.projectId)).toBe(false)
  expect(view.workspaces[p.worktreeId]).toBeUndefined()
  expect(board.channels[p.team.channelId]).toBeUndefined()
  expect(board.channels['project-dm']).toBeUndefined()
  expect(board.channels.shared?.members).toEqual([{ _tag: 'owner' }, { _tag: 'agent', agentId: other.leadId }])
  expect(Object.values(board.posts).map((post) => post.text)).toEqual(['Shared history'])
  for (const [command, payload] of [
    ['project.read', { projectId: p.projectId, collection: 'projects' }],
    ['project.task.history', { projectId: p.projectId, taskId }],
    ['project.requests', { projectId: p.projectId }],
    ['client.thread', { agentId: p.leadId, after: 0, limit: 100 }],
    ['client.agent.configuration', { agentId: p.leadId }],
    ['project.restore', { projectId: p.projectId }],
    ['board.read', { channelId: p.team.channelId }],
    ['board.read', { channelId: 'project-dm' }],
  ] as const) expect(failureOf(yield* call(owner, command, payload)), command).toBeDefined()
  const projects = success(yield* call(owner, 'project.read', { collection: 'projects' }))
  expect(projects.items.map((row: any) => row.projectId)).toEqual([other.projectId])
  const client = success(yield* call(owner, 'client.snapshot', {}))
  expect(client.agents.map((row: any) => row.agentId)).not.toContain(p.leadId)
  expect(client.project.projects.map((row: any) => row.projectId)).toEqual([other.projectId])
  expect(Object.keys(clientView(now.client).alerts)).toEqual(['shared-alert'])
  expect(clientView(now.client).delivery['project-sprint-alert']).toBeUndefined()
  const alertRead = success(yield* call(owner, 'client.alerts.list', { after: 0, limit: 100 }))
  expect(alertRead.items.map((alert: any) => alert.alertId)).toEqual(['shared-alert'])
  const sprintEvent = prior.find(({ event }) => event._tag === 'project.SprintCompleted')!
  const sprintEnvelope = { ...before.threads[p.leadId]![0]!, event: sprintEvent.event }
  expect(engineSource(sprintEnvelope, now)).toBeUndefined()
  const alertEvent = prior.find(({ event }) => event._tag === 'client.AlertRaised' && event.alertId === 'project-sprint-alert')!
  expect(failureOf(yield* call(owner, 'client.event', { sequence: alertEvent.sequence }))).toBeDefined()
  // A tool's ordinary data cannot hide an unrelated event by naming a deleted id.
  expect(deletedEvent(now, eventOf({ _tag: 'PermissionRequested', agentId: other.leadId, permissionRequestId: 'unrelated-request', deliveryId: 'unrelated-delivery', toolCallId: 'call', description: 'Read a record', options: [{ id: 'allow', name: 'Allow once', kind: 'allow_once', level: 'read-only', persistent: false }], holder: { _tag: 'owner' }, settingsWrite: false, toolInput: { projectId: p.projectId } }))).toBe(false)
  const deletedThread = before.threads[p.leadId]![0]!
  expect(failureOf(yield* call(owner, 'client.event', { sequence: deletedThread.sequence }))).toBeDefined()
  expect(visibleTo({ _tag: 'owner' }, deletedThread, now)).toBe(false)
  for (const row of prior.filter(({ event }) => 'agentId' in event && [p.leadId, p.workerId].includes(event.agentId) || 'projectId' in event && event.projectId === p.projectId))
    expect(deletedEvent(now, row.event), row.event._tag).toBe(true)
  const log = yield* readLog
  expect(log.filter((row) => row.event._tag === 'project.Deleted')).toHaveLength(1)
  expect(log.slice(0, prior.length)).toEqual(prior)
  expect(existsSync(join(rig.work, 'uncommitted.txt'))).toBe(true)
  expect(execFileSync('git', ['-C', rig.work, 'branch', '--list', 'topic'], { encoding: 'utf8' })).toContain('topic')
  expect(unwrap(success(yield* call(other.leadCredential, 'project.read', { projectId: other.projectId, collection: 'projects' }))).items).toHaveLength(1)
})).pipe(Effect.orDie))

for (const pending of ['review', 'integration'] as const) it.live(pending === 'review' ? 'deletes an archived project with its open review' : 'refuses deletion during an unfinished integration', () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  const p = yield* decisionSetup()
  success(yield* call(owner, 'project.archive', { projectId: p.projectId }))
  yield* appendSetup([eventOf(pending === 'review' ? { _tag: 'project.MainReviewOpened', review: {
    reviewId: 'open-review', projectId: p.projectId, worktreeId: p.worktreeId, destinationId: 'main', head: 'a', baseHead: 'b', branch: 'topic', base: 'main',
    evidence: { tests: { state: 'missing', report: null }, review: { state: 'missing', report: null } }, openedAt: 1,
    decision: null, outcome: 'open', mergeHead: null, reason: null, decidedAt: null
  } } : { _tag: 'project.GitAttempted', attempt: { requestId: 'open-integration', bodyHash: 'hash', caller: { _tag: 'owner' }, command: 'project.integrate', worktreeId: p.worktreeId,
    before: { head: 'a', branch: 'topic', tree: 'b', mergeHead: null } } })])
  const result = yield* call(owner, 'project.delete', { projectId: p.projectId, confirmed: true })
  if (pending === 'review') {
    success(result)
    expect(projectView(yield* snapshot).projects[p.projectId]).toBeUndefined()
    expect(projectView(yield* snapshot).gitReviews[p.worktreeId]).toBeUndefined()
  } else expect(failureOf(result)?.reason).toBe("Finish the project's Git operation before deleting it.")
})).pipe(Effect.orDie))
