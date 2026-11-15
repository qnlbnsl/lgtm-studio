import { execFileSync } from 'node:child_process'
import { mkdirSync, renameSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { it } from '@effect/vitest'
import * as Effect from 'effect/Effect'
import { afterEach, beforeEach, expect, vi } from 'vitest'
import { CommandHandler } from '../../src/commands/CommandHandler.ts'
import * as GitProcess from '../../src/project/git/process.ts'
import { withGit, workspacePath } from '../../src/project/gitApi.ts'
import { provideBoard } from '../board/wakeFixtures.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
import { appendSetup, call, eventOf, failureOf, makeRig, ownerCredential, removeRig, success, type Rig } from '../support/handlerRig.ts'
import { screenBundle, seedScreens } from './mockScreenFixtures.ts'

let rig: Rig
beforeEach(() => { rig = makeRig(); vi.stubEnv('HOME', rig.home); vi.stubEnv('LGTM_HOME', rig.stateFolder) })
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllEnvs(); removeRig(rig) })
const git = (folder: string, ...args: string[]) => execFileSync('/usr/bin/git', ['-C', folder, ...args], { stdio: 'ignore' })

it.live('one workspace check registers each distinct checkout once and uses five Git processes for one path', () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck(); const seeded = yield* seedScreens(rig)
  yield* appendSetup([eventOf({ _tag: "project.RepositoryLocated", repoId: seeded.repoId, machineId: "local", path: rig.work + "/." })])
  const handler = yield* CommandHandler, input = yield* handler.input({ _tag: 'owner' })
  const processes = vi.spyOn(GitProcess, 'gitProcess')
  yield* withGit(input, (service) => workspacePath(input, { projectId: 'project', worktreeId: 'workspace' }, service))
  expect(processes.mock.calls.length).toBe(5)
})))

it.live('one mock editor save shares its evidence gate and uses fifteen Git processes including durable preflight', () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  const f = yield* seedScreens(rig)
  writeFileSync(join(rig.work, 'mocks/layout.pen'), '{"version":"2.8","children":[]}')
  const designRevision = success(yield* call(ownerCredential, 'project.design.record', { projectId: 'project', design: { repoId: f.repoId, worktreeId: 'workspace', file: 'mocks/layout.pen', components: [] } })).revisionId
  const revisionId = success(yield* call(ownerCredential, 'project.mocks.record', { projectId: 'project', expectedRevision: f.revisionId, bundle: { ...screenBundle(), designRevision } })).revisionId
  const processes = vi.spyOn(GitProcess, 'gitProcess')
  success(yield* call(ownerCredential, 'project.mocks.editor.save', { projectId: 'project', worktreeId: 'workspace', revisionId, designRevision, confirmed: true, content: Buffer.from('{"version":"2.8","children":[],"name":"Edited"}').toString('base64') }))
  expect(processes.mock.calls.length).toBe(15)
})))

it.live('a workspace swapped between two requests is checked again and refused', () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  const f = yield* seedScreens(rig), folder = join(rig.root, 'linked')
  git(rig.work, '-c', 'user.name=Maya Okafor', '-c', 'user.email=maya@example.test', '-c', 'commit.gpgsign=false', 'commit', '--allow-empty', '-m', 'Initial')
  git(rig.work, 'worktree', 'add', '-b', 'linked', folder)
  const worktreeId = success(yield* call(ownerCredential, 'project.workspace.register', { repoId: f.repoId, path: folder, branch: 'linked', machineId: 'local', checkout: 'worktree' })).worktreeId
  yield* appendSetup([eventOf({ _tag: 'project.Placed', projectId: 'project', worktreeId, machineId: 'local' })])
  const handler = yield* CommandHandler, input = yield* handler.input({ _tag: 'owner' })
  const processes = vi.spyOn(GitProcess, 'gitProcess')
  yield* withGit(input, (service) => workspacePath(input, { projectId: 'project', worktreeId }, service))
  expect(processes.mock.calls.length).toBe(6)
  success(yield* call(ownerCredential, 'project.files.list', { projectId: 'project', worktreeId }))
  renameSync(folder, join(rig.root, 'before-swap')); mkdirSync(folder); git(folder, 'init', '-b', 'linked')
  expect(failureOf(yield* call(ownerCredential, 'project.files.list', { projectId: 'project', worktreeId }))?.reason).toBe('checkout belongs to a different repository')
})))


it.live('one state request reuses its own workspace observation and uses five Git processes', () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck(); yield* seedScreens(rig)
  const processes = vi.spyOn(GitProcess, 'gitProcess')
  success(yield* call(ownerCredential, 'project.git.state', { projectId: 'project', worktreeId: 'workspace' }))
  expect(processes.mock.calls.length).toBe(5)
})))

it.live('one linked state request adds target counts to its own check in nine Git processes', () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck(); const f = yield* seedScreens(rig)
  git(rig.work, '-c', 'user.name=Maya Okafor', '-c', 'user.email=maya@example.test', '-c', 'commit.gpgsign=false', 'commit', '--allow-empty', '-m', 'Initial')
  const folder = join(rig.root, 'state-linked'); git(rig.work, 'worktree', 'add', '-b', 'state-linked', folder)
  const worktreeId = success(yield* call(ownerCredential, 'project.workspace.register', { repoId: f.repoId, path: folder, branch: 'state-linked', machineId: 'local', checkout: 'worktree' })).worktreeId
  const processes = vi.spyOn(GitProcess, 'gitProcess')
  const reply = success(yield* call(ownerCredential, 'project.git.state', { worktreeId, target: 'state-linked' }))
  expect(processes.mock.calls.length).toBe(9)
  expect(reply.result).toMatchObject({ branch: 'state-linked', behindParent: 0, pending: 0 })
  renameSync(folder, join(rig.root, 'before-state-swap')); mkdirSync(folder); git(folder, 'init', '-b', 'state-linked')
  expect(failureOf(yield* call(ownerCredential, 'project.git.state', { worktreeId, target: 'state-linked' }))?.reason).toBe('checkout belongs to a different repository')
})))


it.live('a state request preserves the branch refusal before an invalid target refusal', () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck(); yield* seedScreens(rig)
  git(rig.work, 'checkout', '-b', 'changed')
  expect(failureOf(yield* call(ownerCredential, 'project.git.state', { projectId: 'project', worktreeId: 'workspace', target: '--invalid' }))?.reason).toBe('registered workspace branch changed; restore the registered branch')
})))
