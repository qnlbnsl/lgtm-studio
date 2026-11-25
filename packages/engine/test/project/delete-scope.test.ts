import { mkdirSync } from 'node:fs'
import { join } from 'node:path'
import { it } from '@effect/vitest'
import * as Effect from 'effect/Effect'
import { afterEach, beforeEach, expect, vi } from 'vitest'
import { CommandHandler } from '../../src/commands/CommandHandler.ts'
import { closeSignIns, sessions } from '../../src/harnesses/signinRequests.ts'
import { projectKey, projectView } from '../../src/project/view.ts'
import { cleanupUnit7b } from '../../../testing/test/support/unit7b.ts'
import { provideBoard, snapshot } from '../board/wakeFixtures.ts'
import { decisionProject, decisionSetup } from '../support/projectDecisionFixtures.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
import { appendSetup, call, eventOf, failureOf, makeRig, ownerCredential as owner, readLog, removeRig, success, type Rig } from '../support/handlerRig.ts'

type WorkspaceCase = { id: string; parent?: string; project?: boolean; team?: boolean; checkout?: 'main' | 'worktree'; repositoryPath?: boolean }
type ScopeCase = { id: string; label: string; project: string; teams?: readonly string[]; survivor?: string; survivorTeam?: string;
  workspaces: readonly WorkspaceCase[]; removed: readonly string[]; kept: readonly string[]; terminal?: string; git?: string; gitAgent?: boolean; missingRepository?: boolean }
const main: WorkspaceCase = { id: 'main-ws', checkout: 'main', repositoryPath: true }
const free = [main, { id: 'free-ws', parent: 'main-ws' }, { id: 'free-child', parent: 'free-ws' }]
const shared = [{ id: 'shared-project-ws' }, { id: 'p-team-ws', parent: 'shared-project-ws' }, { id: 'p-builder-ws', parent: 'p-team-ws' }]
// Each row corresponds to exactly one row in .foundry/delete-scope.md.
const cases: readonly ScopeCase[] = [
  { id: 'A', label: 'a legacy main project keeps unowned descendants', project: 'main-ws', workspaces: free, removed: [], kept: ['main-ws', 'free-ws', 'free-child'] },
  { id: 'B', label: 'a shared project root does not protect the deleted team and builder roots', project: 'shared-project-ws', survivor: 'shared-project-ws', teams: ['p-team-ws'], workspaces: shared, removed: ['p-team-ws', 'p-builder-ws'], kept: ['shared-project-ws'] },
  { id: 'C', label: 'an exclusive project removes its project, team and builder workspaces', project: 'p-ws', teams: ['p-team-ws'], workspaces: [main, { id: 'p-ws', parent: 'main-ws' }, { id: 'p-team-ws', parent: 'p-ws' }, { id: 'p-builder-ws', parent: 'p-team-ws' }], removed: ['p-ws', 'p-team-ws', 'p-builder-ws'], kept: ['main-ws'] },
  { id: 'D', label: 'a sibling under the same checkout keeps its descendants', project: 'p-ws', survivor: 'sibling-ws', workspaces: [main, { id: 'p-ws', parent: 'main-ws' }, { id: 'sibling-ws', parent: 'main-ws' }, { id: 'sibling-child', parent: 'sibling-ws' }], removed: ['p-ws'], kept: ['main-ws', 'sibling-ws', 'sibling-child'] },
  { id: 'E', label: 'a reachable parent cycle terminates and removes each identifier once', project: 'p-ws', workspaces: [main, { id: 'p-ws', parent: 'cycle-ws' }, { id: 'cycle-ws', parent: 'p-ws' }], removed: ['p-ws', 'cycle-ws'], kept: ['main-ws'] },
  { id: 'F', label: 'an unowned parent cycle outside the walk remains registered', project: 'p-ws', workspaces: [main, { id: 'p-ws' }, { id: 'free-ws', parent: 'free-child' }, { id: 'free-child', parent: 'free-ws' }], removed: ['p-ws'], kept: ['main-ws', 'free-ws', 'free-child'] },
  { id: 'G', label: 'an explicit project workspace seeds a walk below a kept root', project: 'main-ws', workspaces: [main, { id: 'explicit-ws', parent: 'main-ws', project: true }, { id: 'explicit-child', parent: 'explicit-ws' }], removed: ['explicit-ws', 'explicit-child'], kept: ['main-ws'] },
  { id: 'H', label: 'the walk stops at a surviving team workspace', project: 'p-ws', survivorTeam: 'kept-team-ws', workspaces: [main, { id: 'p-ws' }, { id: 'kept-team-ws', parent: 'p-ws' }, { id: 'kept-child', parent: 'kept-team-ws' }], removed: ['p-ws'], kept: ['main-ws', 'kept-team-ws', 'kept-child'] },
  { id: 'I', label: 'a terminal in an unowned retained workspace does not block deletion', project: 'main-ws', workspaces: free, terminal: 'free-ws', removed: [], kept: ['main-ws', 'free-ws', 'free-child'] },
  { id: 'J', label: 'an unfinished Git operation in a retained shared root does not block deletion', project: 'shared-project-ws', survivor: 'shared-project-ws', teams: ['p-team-ws'], workspaces: shared, git: 'shared-project-ws', removed: ['p-team-ws', 'p-builder-ws'], kept: ['shared-project-ws'] },
  { id: 'K', label: 'a legacy checkout without a checkout type keeps unowned descendants', project: 'main-ws', workspaces: [{ id: 'main-ws', repositoryPath: true }, { id: 'free-ws', parent: 'main-ws' }], removed: [], kept: ['main-ws', 'free-ws'] },
  { id: 'L', label: 'a terminal in another project shared root does not block deletion', project: 'shared-project-ws', survivor: 'shared-project-ws', teams: ['p-team-ws'], workspaces: shared, terminal: 'shared-project-ws', removed: ['p-team-ws', 'p-builder-ws'], kept: ['shared-project-ws'] },
  { id: 'M', label: 'a deleted agent must finish its Git attempt in a kept root before either project is deleted', project: 'shared-project-ws', survivor: 'shared-project-ws', teams: ['p-team-ws'], workspaces: shared, git: 'shared-project-ws', gitAgent: true, removed: ['p-team-ws', 'p-builder-ws'], kept: ['shared-project-ws'] },
  { id: 'N', label: 'default builders below a kept team root lack exclusive ownership and remain registered', project: 'shared-project-ws', survivor: 'shared-project-ws', workspaces: [{ id: 'shared-project-ws' }, { id: 'default-builder', parent: 'shared-project-ws', checkout: 'worktree' }], removed: [], kept: ['shared-project-ws', 'default-builder'] },
  { id: 'O', label: 'an explicitly team-owned builder below a kept team root seeds a removal walk', project: 'shared-project-ws', survivor: 'shared-project-ws', workspaces: [{ id: 'shared-project-ws' }, { id: 'team-builder', parent: 'shared-project-ws', checkout: 'worktree', team: true }, { id: 'builder-child', parent: 'team-builder' }], removed: ['team-builder', 'builder-child'], kept: ['shared-project-ws'] },
  { id: 'P', label: 'a parentless legacy checkout without a repository row and its child stay registered', project: 'orphan-main', missingRepository: true, workspaces: [{ id: 'orphan-main', repositoryPath: true }, { id: 'orphan-child', parent: 'orphan-main' }], removed: [], kept: ['orphan-main', 'orphan-child'] }
]
let rig: Rig
beforeEach(() => { rig = makeRig(); vi.stubEnv('HOME', rig.home); vi.stubEnv('LGTM_HOME', rig.stateFolder) })
afterEach(() => { cleanupUnit7b(); vi.unstubAllEnvs(); removeRig(rig) })
for (const example of cases) it.live(`scope ${example.id}: ${example.label}`, () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  const p = yield* decisionSetup(rig.work), other = yield* decisionProject('other', rig.work)
  success(yield* call(owner, 'project.settings.set', { settings: { workspaceFolder: rig.root, shell: 'zsh' } }))
  const before = projectView(yield* snapshot)
  const repoId = example.missingRepository ? 'missing-repository' : p.repoId
  const path = (workspace: WorkspaceCase) => workspace.repositoryPath ? rig.work : join(rig.root, workspace.id)
  for (const workspace of example.workspaces) mkdirSync(path(workspace), { recursive: true })
  yield* appendSetup([
    eventOf({ _tag: 'project.RepositoryAdded', repoId: p.repoId, name: 'Scope repository', path: rig.work }),
    ...example.workspaces.map(workspace => eventOf({ _tag: 'project.WorktreeCreated', repoId, worktreeId: workspace.id, path: path(workspace), branch: workspace.repositoryPath ? 'main' : workspace.id, machineId: 'local',
      ...(workspace.parent === undefined ? {} : { parentId: workspace.parent }), ...(workspace.project ? { projectId: p.projectId } : {}), ...(workspace.team ? { teamId: p.team.teamId } : {}), ...(workspace.checkout === undefined ? {} : { checkout: workspace.checkout }) })),
    eventOf({ _tag: 'project.Created', ...before.projects[p.projectId]!, repoId, worktreeId: example.project }),
    eventOf({ _tag: 'project.TeamAdded', ...Object.values(before.teams).find(team => team.teamId === p.team.teamId)!, worktreeId: example.teams?.[0] ?? example.project }),
    ...(example.survivor === undefined ? [] : [eventOf({ _tag: 'project.Created', ...before.projects[other.projectId]!, repoId: p.repoId, worktreeId: example.survivor }), eventOf({ _tag: 'project.TeamAdded', ...Object.values(before.teams).find(team => team.teamId === other.team.teamId)!, worktreeId: example.survivor })]),
    ...(example.survivorTeam === undefined ? [] : [eventOf({ _tag: 'project.Created', ...before.projects[other.projectId]!, repoId: p.repoId, worktreeId: 'main-ws' }), eventOf({ _tag: 'project.TeamAdded', ...Object.values(before.teams).find(team => team.teamId === other.team.teamId)!, worktreeId: example.survivorTeam })])
  ])
  const handler = yield* CommandHandler, input = yield* handler.input({ _tag: 'owner' })
  try {
    if (example.terminal !== undefined) {
      const manager = sessions(input); manager.terminalEnabled = true
      yield* Effect.promise(() => manager.start({ kind: 'project', worktreeId: example.terminal!, accountId: '', connectionId: 'scope-owner', executable: '/bin/sh', args: [], cwd: path(example.workspaces.find(w => w.id === example.terminal)!), environment: {}, check: async () => ({ state: 'unavailable' }) }))
    }
    const caller = example.gitAgent ? { _tag: 'agent' as const, agentId: p.leadId } : { _tag: 'owner' as const }
    const attemptKey = projectKey(caller._tag === 'owner' ? 'owner' : `agent:${caller.agentId}`, 'retained-attempt')
    if (example.git !== undefined) yield* appendSetup([eventOf({ _tag: 'project.GitAttempted', attempt: { requestId: 'retained-attempt', bodyHash: 'hash', caller, command: 'project.update', worktreeId: example.git, before: { head: 'a', branch: 'topic', tree: 'b', mergeHead: null } } })])
    success(yield* call(owner, 'project.archive', { projectId: p.projectId }))
    if (example.gitAgent) {
      expect(failureOf(yield* call(owner, 'project.delete', { projectId: p.projectId, confirmed: true }))?.reason).toBe("Finish the project's Git operation before deleting it.")
      const refusedView = projectView(yield* snapshot)
      expect(refusedView.projects[p.projectId]).toMatchObject({ projectId: p.projectId })
      expect(refusedView.deletions?.[p.projectId]).toBeUndefined()
      expect(refusedView.gitAttempts[attemptKey]).toMatchObject({ caller, worktreeId: example.git })
      yield* appendSetup([eventOf({ _tag: 'project.GitFinished', caller, operationRequestId: 'retained-attempt' })])
      expect(projectView(yield* snapshot).gitAttempts[attemptKey]).toBeUndefined()
    }
    success(yield* call(owner, 'project.delete', { projectId: p.projectId, confirmed: true }))
    const view = projectView(yield* snapshot), deletion = view.deletions![p.projectId]!
    expect([...deletion.worktreeIds].sort()).toEqual([...example.removed].sort())
    for (const id of example.removed) {
      expect(view.workspaces[id], id).toBeUndefined()
      expect(failureOf(yield* call(owner, 'project.files.list', { worktreeId: id }))?.reason, id).toBe('registered workspace not found')
    }
    for (const id of example.kept) {
      expect(view.workspaces[id], id).toMatchObject({ worktreeId: id, path: path(example.workspaces.find(workspace => workspace.id === id)!) })
      expect(deletion.worktreeIds).not.toContain(id)
    }
    if (example.id === 'A') for (const id of ['free-ws', 'free-child']) {
      const created = (yield* readLog).find(row => row.event._tag === 'project.WorktreeCreated' && row.event.worktreeId === id)!
      success(yield* call(owner, 'client.event', { sequence: created.sequence }))
    }
    if (example.git !== undefined && !example.gitAgent) expect(view.gitAttempts[attemptKey]).toMatchObject({ requestId: 'retained-attempt', worktreeId: example.git })
    if (example.gitAgent) {
      success(yield* call(owner, 'project.archive', { projectId: other.projectId }))
      success(yield* call(owner, 'project.delete', { projectId: other.projectId, confirmed: true }))
      const finalView = projectView(yield* snapshot)
      expect(finalView.projects[p.projectId]).toBeUndefined()
      expect(finalView.projects[other.projectId]).toBeUndefined()
      expect(finalView.gitAttempts[attemptKey]).toBeUndefined()
      expect(finalView.deletions![other.projectId]!.worktreeIds).toEqual(['shared-project-ws'])
      expect(finalView.workspaces['shared-project-ws']).toBeUndefined()
    }
    if (example.terminal !== undefined) expect(sessions(input).hasProjectTerminal(new Set([example.terminal]))).toBe(true)
  } finally { closeSignIns(input.runtime.log) }
})).pipe(Effect.orDie))
