import { execFileSync } from 'node:child_process'
import { mkdirSync, symlinkSync } from 'node:fs'
import { join } from 'node:path'
import { it } from '@effect/vitest'
import * as Effect from 'effect/Effect'
import { afterEach, beforeEach, expect, vi } from 'vitest'
import { provideBoard } from '../board/wakeFixtures.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
import { appendSetup, eventOf, harnessSettings, call, failureOf, makeRig, ownerCredential as owner, readLog, removeRig, seedHarness, success, type Rig } from '../support/handlerRig.ts'

let rig: Rig
beforeEach(() => {
  rig = makeRig(); vi.stubEnv('HOME', rig.home); vi.stubEnv('LGTM_HOME', rig.stateFolder)
  execFileSync('git', ['init', '-b', 'main', rig.work], { stdio: 'ignore' })
})
afterEach(() => { vi.unstubAllEnvs(); removeRig(rig) })
const setup = () => Effect.gen(function* () {
  yield* registerRebuildCheck(); yield* seedHarness()
  const settings = { workspaceFolder: rig.work, shell: 'sh', displayName: 'Maya Okafor',
    reviewDefaults: { reviewers: 3, differentVendor: false },
    lead: { harness: 'first', account: { _tag: 'any' }, permission: 'ask', permissionChosen: true } }
  success(yield* call(owner, 'project.settings.set', { settings }))
  success(yield* call(owner, 'project.repository.add', { name: 'First repository', path: rig.work, machineId: 'local' }))
  return settings
})

it.live('accepts an unchanged workspace, its canonical alias, and a wider workspace without losing settings', () => provideBoard(rig, Effect.gen(function* () {
  const settings = yield* setup()
  const alias = join(rig.root, 'workspace-alias'); symlinkSync(rig.work, alias, 'dir')
  for (const workspaceFolder of [rig.work, alias, rig.root]) {
    success(yield* call(owner, 'project.settings.set', { settings: { ...settings, workspaceFolder } }))
    expect(success(yield* call(owner, 'project.settings', {})).settings).toEqual({ ...settings, workspaceFolder: workspaceFolder === alias ? rig.work : workspaceFolder })
  }
})))

it.live('refuses the first excluded repository and leaves all saved settings unchanged', () => provideBoard(rig, Effect.gen(function* () {
  const settings = yield* setup()
  const sibling = rig.work + '-sibling'; mkdirSync(sibling)
  const child = join(rig.work, 'child'); mkdirSync(child)
  const before = (yield* readLog).filter(row => row.event._tag === 'project.SettingsChanged').length
  for (const workspaceFolder of [sibling, child, rig.home]) {
    const result = yield* call(owner, 'project.settings.set', { settings: { ...settings, workspaceFolder } })
    expect(failureOf(result)?.reason).toBe(`The workspace folder must contain the registered repository "First repository" (${rig.work}).`)
    expect(success(yield* call(owner, 'project.settings', {})).settings).toEqual(settings)
  }
  expect((yield* readLog).filter(row => row.event._tag === 'project.SettingsChanged')).toHaveLength(before)
})))

it.live('names the first excluded registration when an earlier repository is still contained', () => provideBoard(rig, Effect.gen(function* () {
  yield* setup()
  success(yield* call(owner, 'project.settings.set', { settings: { workspaceFolder: rig.root, shell: 'sh' } }))
  const second = join(rig.root, 'second'); mkdirSync(second)
  execFileSync('git', ['init', '-b', 'main', second], { stdio: 'ignore' })
  success(yield* call(owner, 'project.repository.add', { name: 'Second repository', path: second, machineId: 'local' }))
  const result = yield* call(owner, 'project.settings.set', { settings: { workspaceFolder: rig.work, shell: 'sh' } })
  expect(failureOf(result)?.reason).toBe(`The workspace folder must contain the registered repository "Second repository" (${second}).`)
  expect(success(yield* call(owner, 'project.settings', {})).settings?.workspaceFolder).toBe(rig.root)
})))

it.live('lets legacy installations save unrelated settings without changing their narrower workspace', () => provideBoard(rig, Effect.gen(function* () {
  const settings = yield* setup()
  yield* appendSetup([eventOf({ _tag: 'project.RepositoryAdded', repoId: 'legacy', name: 'Unused legacy repository', path: rig.outside })])
  const changed = { ...settings, reviewDefaults: { reviewers: 4, differentVendor: true }, lead: { ...settings.lead, permission: 'edit' } }
  success(yield* call(owner, 'project.settings.set', { settings: changed }))
  expect(success(yield* call(owner, 'project.settings', {})).settings).toEqual(changed)
  expect(failureOf(yield* call(owner, 'project.settings.set', { settings: { ...changed, workspaceFolder: rig.home } }))).toBeDefined()
})))

const commit = () => execFileSync('git', ['-C', rig.work, '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.test', '-c', 'commit.gpgsign=false', 'commit', '--allow-empty', '-m', 'Initial'], { stdio: 'ignore' })
for (const kind of ['location', 'workspace'] as const) it.live(`refuses a folder excluding a registered ${kind}`, () => provideBoard(rig, Effect.gen(function* () {
  yield* setup(); commit()
  success(yield* call(owner, 'project.settings.set', { settings: { workspaceFolder: rig.root, shell: 'sh' } }))
  const linked = join(rig.root, 'linked')
  execFileSync('git', ['-C', rig.work, 'worktree', 'add', '-b', 'linked', linked], { stdio: 'ignore' })
  const repoId = success(yield* call(owner, 'project.repositories', {})).items[0].repoId
  success(yield* call(owner, kind === 'location' ? 'project.repository.locate' : 'project.workspace.register', {
    repoId, path: linked, machineId: 'local', ...(kind === 'workspace' ? { branch: 'linked', checkout: 'worktree' } : {})
  }))
  const result = yield* call(owner, 'project.settings.set', { settings: { workspaceFolder: rig.work, shell: 'sh' } })
  expect(failureOf(result)?.reason).toBe(`The workspace folder must contain the registered repository "First repository" (${linked}).`)
})))

it.live('keeps the common Git directory inside the folder when the registered repository is a linked worktree', () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck(); commit()
  const linked = join(rig.root, 'linked')
  execFileSync('git', ['-C', rig.work, 'worktree', 'add', '-b', 'linked', linked], { stdio: 'ignore' })
  success(yield* call(owner, 'project.settings.set', { settings: { workspaceFolder: rig.root, shell: 'sh' } }))
  success(yield* call(owner, 'project.repository.add', { name: 'Linked repository', path: linked, machineId: 'local' }))
  const result = yield* call(owner, 'project.settings.set', { settings: { workspaceFolder: linked, shell: 'sh' } })
  expect(failureOf(result)?.reason).toBe(`The workspace folder must contain the registered repository "Linked repository" (${join(rig.work, '.git')}).`)
})))

it.live('redacts a repository refusal with the production redactor', () => provideBoard(rig, Effect.gen(function* () {
  const settings = yield* setup()
  success(yield* call(owner, 'project.settings.set', { settings: { ...settings, workspaceFolder: rig.root } }))
  yield* appendSetup([eventOf({ _tag: 'project.RepositoryAdded', repoId: 'private', name: 'synthetic-s32-secret', path: rig.outside })])
  const reason = failureOf(yield* call(owner, 'project.settings.set', { settings }))?.reason
  // This repository has no project, and it still keeps its registry boundary.
  expect(reason).toContain('[REDACTED]')
  expect(reason).not.toContain('synthetic-s32-secret')
  expect(JSON.stringify(yield* readLog)).not.toContain('synthetic-s32-secret')
}), ['synthetic-s32-secret']))

it.live('preserves the effective lead permission across a workspace save', () => provideBoard(rig, Effect.gen(function* () {
  const settings = yield* setup()
  success(yield* call(owner, 'harnesses', { operation: 'set', harness: harnessSettings('first', { reportedModes: [{ id: 'acceptEdits', level: 'edit' }] }) }))
  const { permissionChosen, ...legacyLead } = settings.lead
  success(yield* call(owner, 'project.settings.set', { settings: { ...settings, lead: legacyLead } }))
  const effective = success(yield* call(owner, 'project.settings', {})).settings
  expect(effective.lead.permission).toBe('auto')
  success(yield* call(owner, 'project.settings.set', { settings: { ...effective, workspaceFolder: rig.root } }))
  expect(success(yield* call(owner, 'project.settings', {})).settings.lead).toEqual(effective.lead)
})))
