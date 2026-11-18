import { randomBytes } from 'node:crypto'
import { execFileSync } from 'node:child_process'
import { readFileSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect as baseExpect, type Page } from '@playwright/test'
import * as Effect from 'effect/Effect'
import * as Layer from 'effect/Layer'
import * as Redacted from 'effect/Redacted'
import * as SqliteClient from '../../../../third_party/t3code/node-sqlite/src/nodeSqliteClient.ts'
import { buildServices } from '../../../engine/src/service.ts'
import { serve } from '../../../engine/src/client/web/server.ts'
import { engineLists } from '../../../engine/src/pieces/lists.ts'
import { Views } from '../../../engine/src/views/Views.ts'
import { projectView } from '../../../engine/src/project/view.ts'
import { Redactor } from '../../../engine/src/log/Redactor.ts'
import { unusedPort } from '../../../engine/test/support/browserWeb.ts'
import { makeFakeRunner } from '../../../engine/test/support/fakeRunner.ts'
import { call, harnessSettings, makeFakeSecrets, makeRig, ownerCredential, removeRig, seedAccount, success } from '../../../engine/test/support/handlerRig.ts'
import { fakeGithub } from '../../../engine/test/support/fakeGithub.ts'
import { registerRebuildCheck } from '../../../engine/test/support/rebuildCheck.ts'
import { buildStudio } from '../../../studio/scripts/build.ts'
import { completedSetup } from './completedSetup.ts'

const expect = baseExpect.configure({ timeout: 30_000 })

export async function exerciseStudioGithub(page: Page, dirtyBase = false, moveBaseBeforeApproval = false) {
  const rig = makeRig(), previousHome = process.env.LGTM_HOME, previousPath = process.env.PATH
  process.env.LGTM_HOME = rig.stateFolder
  const git = (folder: string, ...args: string[]) => execFileSync('/usr/bin/git', ['-C', folder, ...args], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim()
  const commit = (folder: string, message: string) => { git(folder, 'add', '--all'); git(folder, 'commit', '-m', message) }
  const password = randomBytes(32).toString('base64url'), errors: string[] = []
  page.on('pageerror', error => errors.push(error.message))
  try {
    git(rig.work, 'init', '-b', 'main'); git(rig.work, 'config', 'user.name', 'Fixture'); git(rig.work, 'config', 'user.email', 'fixture@example.invalid'); git(rig.work, 'config', 'commit.gpgsign', 'false')
    writeFileSync(join(rig.work, 'base.txt'), 'Base\n'); commit(rig.work, 'Initial')
    const gh = fakeGithub(rig.root, rig.work)
    process.env.PATH = `${gh.bin}:${previousPath}`
    const studioRoot = await buildStudio(join(rig.root, 'studio')), port = await unusedPort()
    await Effect.runPromise(Effect.scoped(Effect.gen(function* () {
      const runner = yield* makeFakeRunner()
      const dependencies = Layer.mergeAll(runner.layer, makeFakeSecrets().layer, Layer.succeed(Redactor, { redact: e => Effect.succeed(e), redactText: s => Effect.succeed(s) }))
      yield* Effect.scoped(Effect.gen(function* () {
        yield* registerRebuildCheck(); yield* seedAccount('launch', 'cursor')
        success(yield* call(ownerCredential, 'harnesses', { operation: 'set', harness: harnessSettings('cursor', { adapterPath: process.execPath, reportedModes: [{ id: 'default', level: 'ask' }] }) }))
        success(yield* call(ownerCredential, 'accounts set-secret', { accountId: 'launch', name: 'CURSOR_API_KEY', nonce: 's27-browser', value: Redacted.make('synthetic-s27-browser-key') }))
        success(yield* call(ownerCredential, 'project.settings.set', { settings: { workspaceFolder: rig.root, shell: 'sh' } }))
        const repoId = success(yield* call(ownerCredential, 'project.repository.add', { name: 'GitHub fixture', path: rig.work, machineId: 'local' })).repoId
        success(yield* call(ownerCredential, 'project.workspace.register', { repoId, path: rig.work, branch: 'main', machineId: 'local', checkout: 'main' }))
        const created = success(yield* call(ownerCredential, 'project.create', { name: 'GitHub project', goal: 'Deliver through a pull request', machineId: 'local', repository: { _tag: 'existing', repoId }, useWorktree: true }))
        const view = projectView(yield* (yield* Views).snapshot), project = view.projects[created.projectId]!, source = view.workspaces[project.worktreeId]!
        yield* completedSetup(project.projectId)
        writeFileSync(join(source.path, 'feature.txt'), 'Feature\n'); commit(source.path, 'Feature')
        const original = git(rig.work, 'rev-parse', 'HEAD')
        const remoteBase = dirtyBase ? git(gh.bare, '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid', 'commit-tree', `${original}^{tree}`, '-p', original, '-m', 'Another hosted merge') : original
        if (dirtyBase) {
          git(gh.bare, 'update-ref', 'refs/heads/main', remoteBase)
          writeFileSync(join(rig.work, 'owner-notes.txt'), 'Owner notes must remain.\n')
        }
        yield* serve({ password: Redacted.make(password), staticRoot: studioRoot, lists: engineLists, settings: { enabled: true, port, heartbeatSeconds: 25, proxyMode: false } })
        yield* Effect.promise(async () => {
          const url = `http://127.0.0.1:${port}/#/p/${project.projectId}/git`
          await page.goto(url); await page.locator('#login-password').fill(password); await page.getByRole('button', { name: 'Sign in', exact: true }).click()
          await expect(page.locator('.tree-row.project')).toBeVisible(); await page.goto(url)
          await page.getByRole('button', { name: 'Open merge review', exact: true }).click()
          const dialog = page.getByRole('dialog')
          await expect(dialog).toContainText('GitHub pull request')
          await expect(dialog.getByRole('link', { name: '#17', exact: true })).toHaveAttribute('href', 'https://github.com/fixture/product/pull/17')
          await expect(page.locator('#pr-title')).toHaveValue('GitHub project')
          await expect(page.locator('#pr-body')).toHaveValue(/Deliver through a pull request/)
          await expect(page.locator('#pr-body')).toHaveValue(new RegExp(`Reviewed base: ${remoteBase}`))
          expect(git(rig.work, 'rev-parse', 'HEAD')).toBe(original)
          await dialog.getByRole('button', { name: 'Cancel', exact: true }).click()
          const unchangedCalls = readFileSync(gh.calls, 'utf8')
          await page.getByRole('button', { name: 'Refresh Git', exact: true }).click()
          await expect(page.locator('.git')).toHaveAttribute('aria-busy', 'false')
          expect(readFileSync(gh.calls, 'utf8')).toBe(unchangedCalls)
          writeFileSync(join(source.path, 'later.txt'), 'Later commit\n'); commit(source.path, 'After Prepare')
          await page.getByRole('button', { name: 'Refresh Git', exact: true }).click()
          await expect(page.locator('.git')).toHaveAttribute('aria-busy', 'false')
          expect(git(gh.bare, 'rev-parse', `refs/heads/${source.branch}`)).toBe(git(source.path, 'rev-parse', 'HEAD'))
          await page.getByRole('button', { name: 'Open merge review', exact: true }).click()
          if (moveBaseBeforeApproval) {
            await expect(page.locator('#pr-body')).toHaveValue(new RegExp(`Reviewed base: ${remoteBase}`))
            const moved = git(gh.bare, '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid', 'commit-tree', `${remoteBase}^{tree}`, '-p', remoteBase, '-m', 'Another project approved')
            git(gh.bare, 'update-ref', 'refs/heads/main', moved)
            await dialog.getByRole('button', { name: 'Approve without evidence', exact: true }).click()
            await expect(page.locator('.toasts')).toContainText('The GitHub base moved since this review was prepared. Open the merge review again.')
            expect(git(rig.work, 'rev-parse', 'HEAD')).toBe(original)
            expect(gh.read().state).toBe('OPEN')
            await dialog.getByRole('button', { name: 'Cancel', exact: true }).click()
            await page.getByRole('button', { name: 'Open merge review', exact: true }).click()
            await expect(page.locator('#pr-body')).toHaveValue(new RegExp(`Reviewed base: ${moved}`))
          }
          await dialog.getByRole('button', { name: 'Approve without evidence', exact: true }).click()
          await expect(dialog).toHaveCount(0); await expect(page.locator('.git')).toHaveAttribute('aria-busy', 'false')
          await expect(page.locator('.git-integrate')).toContainText('MERGED')
          if (dirtyBase) {
            await expect(page.locator('.git-integrate')).toContainText('left alone')
            expect(git(rig.work, 'rev-parse', 'HEAD')).toBe(original)
            expect(readFileSync(join(rig.work, 'owner-notes.txt'), 'utf8')).toBe('Owner notes must remain.\n')
            expect(git(gh.bare, 'rev-list', '--parents', '-n', '1', 'refs/heads/main').split(' ')).toHaveLength(3)
          } else {
            await expect(page.locator('.git-integrate')).toContainText('fast-forwarded')
            expect(readFileSync(join(rig.work, 'later.txt'), 'utf8')).toBe('Later commit\n')
            expect(git(rig.work, 'rev-list', '--parents', '-n', '1', 'HEAD').split(' ')).toHaveLength(3)
            expect(git(rig.work, 'rev-parse', 'HEAD')).toBe(git(gh.bare, 'rev-parse', 'refs/heads/main'))
          }
          expect(git(rig.work, 'rev-parse', 'refs/remotes/origin/main')).toBe(git(gh.bare, 'rev-parse', 'refs/heads/main'))
          expect(errors).toEqual([])
        })
      })).pipe(Effect.provide(buildServices(rig, engineLists).pipe(Layer.provide(dependencies))), Effect.provide(runner.layer))
    })).pipe(Effect.provide(SqliteClient.layer({ filename: rig.database, readerWorker: true })), Effect.scoped) as Effect.Effect<void, unknown>)
  } finally {
    if (previousHome === undefined) delete process.env.LGTM_HOME; else process.env.LGTM_HOME = previousHome
    if (previousPath === undefined) delete process.env.PATH; else process.env.PATH = previousPath
    removeRig(rig)
  }
}
