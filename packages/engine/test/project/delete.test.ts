import { createHash } from 'node:crypto'
import { designWorkspaceReference } from '../../src/project/gitApi.ts'
import * as Server from '../../src/api/server.ts'
import * as Redacted from 'effect/Redacted'
import { connect } from '../support/socketClient.ts'
import { CommandHandler } from '../../src/commands/CommandHandler.ts'
import { terminalReads } from '../../src/harnesses/terminalRequests.ts'
import { closeSignIns, sessions } from '../../src/harnesses/signinRequests.ts'
import { clientView } from '../../src/client/alerts/view.ts'
import { engineSource } from '../../src/client/alerts/sources.ts'
import { execFileSync } from 'node:child_process'
import { existsSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { it } from '@effect/vitest'
import * as Effect from 'effect/Effect'
import { afterEach, beforeEach, expect, vi } from 'vitest'
import { ProjectId, type BoardView } from '@lgtm/contracts'
import * as Schema from 'effect/Schema'
import { cleanupUnit7b } from '../../../testing/test/support/unit7b.ts'
import { provideBoard, serviceCall, snapshot } from '../board/wakeFixtures.ts'
import { decisionProject, decisionSetup } from '../support/projectDecisionFixtures.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
import { appendSetup, call, eventOf, failureOf, freshRequest, makeRig, ownerCredential as owner, readLog, removeRig, success, type Rig } from '../support/handlerRig.ts'
import { projectView } from '../../src/project/view.ts'
import { deletedAgent, deletedEvent } from '../../src/project/deleted.ts'
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
  for (const credential of [p.leadCredential, p.workerCredential, other.leadCredential]) expect(failureOf(yield* call(credential, 'project.delete', payload))?.reason).toBe('command refused')
  expect(failureOf(yield* serviceCall('project.delete', payload))?.reason).toBe('command refused')
  success(yield* call(owner, 'project.archive', { projectId: p.projectId }))
  for (const credential of [p.leadCredential, p.workerCredential, other.leadCredential]) {
    expect(failureOf(yield* call(credential, 'project.delete', payload))?.reason).toBe(credential === p.leadCredential || credential === p.workerCredential ? 'invalid credential' : 'command refused')
  }
  expect(failureOf(yield* serviceCall('project.delete', payload))?.reason).toBe('command refused')
  success(yield* call(other.leadCredential, 'project.read', { projectId: other.projectId, collection: 'projects' }))
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
  yield* appendSetup([eventOf({ _tag: 'project.WorktreeCreated', repoId: p.repoId, worktreeId: p.worktreeId, path: rig.work, branch: 'topic', machineId: 'local' })])
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

it.live('identifier secrets do not cause a partial deletion in the real writer', () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  const p = yield* decisionSetup()
  yield* appendSetup([
    eventOf({ _tag: 'project.WorktreeCreated', repoId: p.repoId, worktreeId: p.worktreeId, branch: 'topic', path: rig.work, machineId: 'local' }),
    eventOf({ _tag: 'board.PostWritten', postId: 'one-post', channelId: p.team.channelId, author: { _tag: 'agent', agentId: p.leadId }, text: 'History', to: [], reply: 'none' }),
    eventOf({ _tag: 'client.AlertRaised', alertId: 'one-alert', alertKey: 'one:alert', kind: 'question', agentId: p.leadId, title: 'Question', sourceSequence: 1 })
  ])
  success(yield* call(owner, 'project.archive', { projectId: p.projectId }))
  success(yield* call(owner, 'project.delete', { projectId: p.projectId, confirmed: true }))
  const stored = (yield* readLog).find(row => row.event._tag === 'project.Deleted')!.event
  expect(stored).toMatchObject({ projectId: p.projectId, agentIds: expect.arrayContaining([p.leadId, p.workerId]), channelIds: expect.arrayContaining([p.team.channelId]), postIds: ['one-post'], alertIds: ['one-alert'], worktreeIds: [p.worktreeId] })
  const now = yield* snapshot
  expect(projectView(now).projects[p.projectId]).toBeUndefined()
  expect(now.agents[p.leadId]).toBeUndefined()
  expect(now.agents[p.workerId]).toBeUndefined()
}), ['one-project', 'one-lead', 'one-worker', 'one-workspace', 'one-post', 'one-alert', 'project-default:']).pipe(Effect.orDie))

it.live('descendant workspaces without projectId disappear and every unscoped read refuses them', () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  const p = yield* decisionSetup()
  yield* appendSetup([eventOf({ _tag: 'project.WorktreeCreated', repoId: p.repoId, worktreeId: p.worktreeId, path: join(rig.root, 'parent'), branch: 'topic', machineId: 'local', checkout: 'worktree', projectId: p.projectId })])
  for (const [id, parent] of [['child', p.worktreeId], ['grandchild', 'child']]) yield* appendSetup([eventOf({ _tag: 'project.WorktreeCreated', repoId: p.repoId, worktreeId: id, parentId: parent, path: rig.work, branch: 'topic', machineId: 'local' })])
  success(yield* call(owner, 'project.settings.set', { settings: { workspaceFolder: rig.root, shell: 'zsh' } }))
  const creation = (yield* readLog).find(row => row.event._tag === 'project.WorktreeCreated' && row.event.worktreeId === 'child')!
  success(yield* call(owner, 'project.archive', { projectId: p.projectId }))
  success(yield* call(owner, 'project.delete', { projectId: p.projectId, confirmed: true }))
  const items = success(yield* call(owner, 'project.workspaces', {})).items
  expect(items.map((row: any) => row.worktreeId)).not.toContain('child')
  for (const worktreeId of ['child', 'grandchild']) {
    expect(projectView(yield* snapshot).workspaces[worktreeId]).toBeUndefined()
    for (const [name, extra] of [['project.git.state', {}], ['project.changes', {}], ['project.files.list', {}], ['project.files.read', { path: 'file.txt' }], ['project.files.write', { projectId: p.projectId, path: 'file.txt', text: 'keep', version: 'version' }]] as const)
      expect(failureOf(yield* call(owner, name, { worktreeId, ...extra }))?.reason, name).toBe('registered workspace not found')
  }
  expect(failureOf(yield* call(owner, 'client.event', { sequence: creation.sequence }))?.reason).toBe('command refused')
  const handler = yield* CommandHandler
  const input = { ...(yield* handler.input({ _tag: 'owner' })), connectionId: 'owner', snapshot: { ...(yield* snapshot), client: { ...(yield* snapshot).client as object, webSettings: { terminalEnabled: true } } } }
  for (const worktreeId of ['child', 'grandchild']) expect(yield* terminalReads['client.terminal.open'](input, { worktreeId, cols: 80, rows: 24 }).pipe(Effect.result)).toMatchObject({ _tag: 'Failure', failure: { reason: 'The project worktree is unavailable.' } })
  closeSignIns(input.runtime.log)
})).pipe(Effect.orDie))

it.live('late project, agent, board and alert delivery events cannot resurrect deleted views', () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  const p = yield* decisionSetup()
  yield* appendSetup([eventOf({ _tag: 'client.AlertRaised', alertId: 'late-alert', alertKey: 'late:alert', kind: 'question', agentId: p.leadId, title: 'Late', sourceSequence: 1 })])
  const old = (yield* readLog).filter(row => ['project.Created', 'AgentStarted'].includes(row.event._tag)).map(row => row.event)
  success(yield* call(owner, 'project.archive', { projectId: p.projectId }))
  success(yield* call(owner, 'project.delete', { projectId: p.projectId, confirmed: true }))
  yield* appendSetup([...old, eventOf({ _tag: 'client.AlertRaised', alertId: 'late-new-alert', alertKey: 'late:new', kind: 'question', agentId: p.leadId, title: 'Late new alert', sourceSequence: 1 }),
    eventOf({ _tag: 'client.AlertDelivered', alertId: 'late-new-alert', carrier: 'ntfy', generation: 0, attempt: 1 }),
    eventOf({ _tag: 'client.AlertDelivered', alertId: 'late-alert', carrier: 'ntfy', generation: 0, attempt: 1 }),
    eventOf({ _tag: 'board.PostWritten', postId: 'late-post', channelId: p.team.channelId, author: { _tag: 'owner' }, text: 'Late', to: [], reply: 'none' })])
  const now = yield* snapshot
  expect(projectView(now).projects[p.projectId]).toBeUndefined()
  expect(now.agents[p.leadId]).toBeUndefined()
  expect(now.threads[p.leadId]).toBeUndefined()
  expect(clientView(now.client).delivery['late-alert']).toBeUndefined()
  expect(clientView(now.client).alerts['late-new-alert']).toBeUndefined()
  expect(clientView(now.client).delivery['late-new-alert']).toBeUndefined()
  expect((now.board as BoardView).posts['late-post']).toBeUndefined()
})).pipe(Effect.orDie))

it.live('shared-channel replies and repository design registrations survive deletion', () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  const p = yield* decisionSetup(), other = yield* decisionProject('other')
  execFileSync('git', ['init', '-b', 'main', rig.work], { stdio: 'ignore' })
  execFileSync('git', ['-C', rig.work, '-c', 'user.name=Test Author', '-c', 'user.email=test@example.test', '-c', 'commit.gpgsign=false', 'commit', '--allow-empty', '-m', 'Initial'], { stdio: 'ignore' })
  const projectFolder = join(rig.root, 'design-project'), survivorFolder = join(rig.root, 'design-survivor')
  execFileSync('git', ['-C', rig.work, 'worktree', 'add', '-b', 'topic', projectFolder], { stdio: 'ignore' })
  execFileSync('git', ['-C', rig.work, 'worktree', 'add', '-b', 'survivor', survivorFolder], { stdio: 'ignore' })
  writeFileSync(join(projectFolder, 'design.fig'), 'Design remains on disk')
  success(yield* call(owner, 'project.settings.set', { settings: { workspaceFolder: rig.root, shell: 'zsh' } }))
  yield* appendSetup([
    eventOf({ _tag: 'project.RepositoryAdded', repoId: p.repoId, name: 'Design repository', path: rig.work }),
    eventOf({ _tag: 'project.RepositoryLocated', repoId: p.repoId, path: rig.work, machineId: 'local' }),
    eventOf({ _tag: 'project.WorktreeCreated', repoId: p.repoId, worktreeId: other.worktreeId, path: survivorFolder, branch: 'survivor', machineId: 'local', checkout: 'worktree', projectId: other.projectId }),
    eventOf({ _tag: 'project.Created', ...projectView(yield* snapshot).projects[other.projectId]!, repoId: p.repoId }),
    eventOf({ _tag: 'project.WorktreeCreated', repoId: p.repoId, worktreeId: p.worktreeId, path: projectFolder, branch: 'topic', machineId: 'local', checkout: 'worktree', projectId: p.projectId })
  ])
  yield* appendSetup([eventOf({ _tag: 'board.ChannelOpened', channelId: 'shared', name: 'Shared', kind: 'plain', opener: { _tag: 'owner' }, manager: { _tag: 'owner' }, members: [{ _tag: 'owner' }, { _tag: 'agent', agentId: p.leadId }, { _tag: 'agent', agentId: other.leadId }] }),
    eventOf({ _tag: 'project.DesignRecorded', projectId: p.projectId, revisionId: 'shared-design', design: { repoId: p.repoId, worktreeId: p.worktreeId, file: 'design.fig', components: [] }, author: { _tag: 'owner' }, fileFingerprint: createHash('sha256').update('Design remains on disk').digest('hex') })])
  expect(success(yield* call(owner, 'project.design.read', { repoId: p.repoId })).result.file._tag).toBe('available')
  expect(unwrap(success(yield* call(other.leadCredential, 'project.design.read', { repoId: p.repoId, projectId: other.projectId }))).result.file._tag).toBe('available')
  const postId = success(yield* call(p.leadCredential, 'board.post', { channelId: 'shared', text: 'Deleted author', to: [], reply: 'none' })).postId
  yield* appendSetup([eventOf({ _tag: 'board.PostWritten', postId: 'owner-reply', channelId: 'shared', author: { _tag: 'owner' }, text: 'Owner reply', to: [], reply: 'none', replyToPostId: postId, context: [{ _tag: 'file', projectId: p.projectId, repoId: p.repoId, worktreeId: p.worktreeId, path: 'file.txt' }] })])
  yield* appendSetup([eventOf({ _tag: 'board.PostWritten', postId: 'other-reply', channelId: 'shared', author: { _tag: 'agent', agentId: other.leadId }, text: 'Other reply', to: [], reply: 'none', replyToPostId: postId })])
  const before = projectView(yield* snapshot)
  success(yield* call(owner, 'project.archive', { projectId: p.projectId }))
  success(yield* call(owner, 'project.delete', { projectId: p.projectId, confirmed: true }))
  const now = yield* snapshot
  expect(Object.values((now.board as BoardView).posts).map(row => row.text)).toEqual(['Owner reply', 'Other reply'])
  expect(projectView(now).designs).toEqual(before.designs)
  expect(projectView(now).designHeads).toEqual(before.designHeads)
  expect(projectView(now).workspaces[p.worktreeId]).toBeUndefined()
  expect(success(yield* call(owner, 'project.design.read', { repoId: p.repoId })).result).toMatchObject({ _tag: 'registered', record: { revisionId: 'shared-design' }, file: { _tag: 'unavailable', reason: 'artifact unavailable' } })
  const input = yield* (yield* CommandHandler).input({ _tag: 'owner' })
  expect(yield* designWorkspaceReference(input, { repoId: p.repoId, worktreeId: p.worktreeId }).pipe(Effect.result)).toMatchObject({ _tag: 'Failure', failure: { reason: 'registered design workspace is unavailable' } })
  expect(unwrap(success(yield* call(other.leadCredential, 'project.design.read', { repoId: p.repoId, projectId: other.projectId }))).result.file).toEqual({ _tag: 'unavailable', reason: 'artifact unavailable' })
  expect(existsSync(join(projectFolder, 'design.fig'))).toBe(true)
  // The context-bearing row stays stored, while board.read cannot open it.
  const shared = success(yield* call(owner, 'board.read', { channelId: 'shared' }))
  expect(JSON.stringify(shared)).not.toContain('Owner reply')
  expect(JSON.stringify(shared)).toContain('Other reply')
})).pipe(Effect.orDie))

it.live('deletion refuses an open terminal until the existing close path finishes', () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  const p = yield* decisionSetup()
  yield* appendSetup([eventOf({ _tag: 'project.WorktreeCreated', repoId: p.repoId, worktreeId: p.worktreeId, path: rig.work, branch: 'topic', machineId: 'local' })])
  const handler = yield* CommandHandler
  const input = { ...(yield* handler.input({ _tag: 'owner' })), connectionId: 'terminal-owner', snapshot: { ...(yield* snapshot), client: { ...(yield* snapshot).client as object, webSettings: { terminalEnabled: true } } } }
  const manager = sessions(input); manager.terminalEnabled = true
  try {
    const terminal = yield* terminalReads['client.terminal.open'](input, { worktreeId: p.worktreeId, cols: 80, rows: 24 })
    success(yield* call(owner, 'project.archive', { projectId: p.projectId }))
    expect(failureOf(yield* call(owner, 'project.delete', { projectId: p.projectId, confirmed: true }))?.reason).toBe('Close the project terminals before deleting the project.')
    const socket = join(rig.stateFolder, 'delete.sock'), token = 'temporary-owner-delete-fixture'
    yield* Server.serve({ socketPath: socket, ownerCredential: Redacted.make(token), debug: false })
    const api = yield* connect(socket, token)
    const reply = yield* api['project.delete']({ requestId: freshRequest(), projectId: Schema.decodeUnknownSync(ProjectId)(p.projectId), confirmed: true }).pipe(Effect.result)
    expect(reply).toMatchObject({ _tag: 'Failure', failure: { reason: 'Close the project terminals before deleting the project.' } })
    yield* terminalReads['client.terminal.close'](input, { sessionId: terminal.reply.sessionId })
    success(yield* call(owner, 'project.delete', { projectId: p.projectId, confirmed: true }))
  } finally { closeSignIns(input.runtime.log) }
})).pipe(Effect.orDie))

it.live('a surviving project keeps the descendants of its shared workspace', () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  const p = yield* decisionSetup(), other = yield* decisionProject('other')
  yield* appendSetup([
    eventOf({ _tag: 'project.WorktreeCreated', repoId: p.repoId, worktreeId: p.worktreeId, path: rig.work, branch: 'topic', machineId: 'local' }),
    eventOf({ _tag: 'project.WorktreeCreated', repoId: p.repoId, worktreeId: other.worktreeId, parentId: p.worktreeId, path: join(rig.work, 'other'), branch: 'other', machineId: 'local' }),
    eventOf({ _tag: 'project.WorktreeCreated', repoId: p.repoId, worktreeId: 'other-child', parentId: other.worktreeId, path: join(rig.work, 'other-child'), branch: 'other-child', machineId: 'local' })
  ])
  success(yield* call(owner, 'project.archive', { projectId: p.projectId }))
  success(yield* call(owner, 'project.delete', { projectId: p.projectId, confirmed: true }))
  const view = projectView(yield* snapshot)
  expect(view.workspaces[p.worktreeId]).toBeUndefined()
  expect(view.workspaces[other.worktreeId]).toBeDefined()
  expect(view.workspaces['other-child']).toBeDefined()
})).pipe(Effect.orDie))


it.live('the tombstone index is reused across project views sharing the deletion map', () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  const p = yield* decisionSetup()
  success(yield* call(owner, 'project.archive', { projectId: p.projectId }))
  success(yield* call(owner, 'project.delete', { projectId: p.projectId, confirmed: true }))
  const now = yield* snapshot, view = projectView(now), deletion = view.deletions![p.projectId]!
  let visits = 0
  const deletions = Object.defineProperty({}, p.projectId, { enumerable: true, get: () => { visits++; return deletion } })
  for (let i = 0; i < 3; i++) expect(deletedAgent({ ...now, project: { ...view, deletions } }, p.leadId)).toBe(true)
  expect(visits).toBe(1)
  expect(deletedAgent({ ...now, project: { ...view, deletions: {} } }, p.leadId)).toBe(false)
})).pipe(Effect.orDie))
