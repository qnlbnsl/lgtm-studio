import { execFileSync } from 'node:child_process'
import { mkdirSync, symlinkSync } from 'node:fs'
import { join } from 'node:path'
import { it } from '@effect/vitest'
import * as Effect from 'effect/Effect'
import { afterEach, beforeEach, expect, vi } from 'vitest'
import { provideBoard } from '../board/wakeFixtures.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
import { call, failureOf, makeRig, ownerCredential as owner, readLog, removeRig, seedHarness, success, type Rig } from '../support/handlerRig.ts'

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
