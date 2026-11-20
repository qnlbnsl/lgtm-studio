import { execFileSync } from 'node:child_process'
import { randomBytes } from 'node:crypto'
import { join } from 'node:path'
import { expect, test } from '@playwright/test'
import * as Effect from 'effect/Effect'
import * as Layer from 'effect/Layer'
import * as Redacted from 'effect/Redacted'
import * as SqliteClient from '../../../third_party/t3code/node-sqlite/src/nodeSqliteClient.ts'
import { buildServices } from '../../engine/src/service.ts'
import { serve } from '../../engine/src/client/web/server.ts'
import { engineLists } from '../../engine/src/pieces/lists.ts'
import { Redactor } from '../../engine/src/log/Redactor.ts'
import { unusedPort } from '../../engine/test/support/browserWeb.ts'
import { makeFakeRunner } from '../../engine/test/support/fakeRunner.ts'
import { appendSetup, call, eventOf, ownerCredential, success, makeFakeSecrets, makeRig, removeRig } from '../../engine/test/support/handlerRig.ts'
import { registerRebuildCheck } from '../../engine/test/support/rebuildCheck.ts'
import { decisionSetup } from '../../engine/test/support/projectDecisionFixtures.ts'
import { buildStudio } from '../../studio/scripts/build.ts'
import { completedSetup } from './support/completedSetup.ts'

test('the website shows the recorded repository base and follows an owner command without adding a control', async ({ page }) => {
  test.setTimeout(90_000)
  const rig = makeRig(), previous = process.env.LGTM_HOME
  process.env.LGTM_HOME = rig.stateFolder
  const password = randomBytes(32).toString('base64url')
  const git = (...args: string[]) => execFileSync('/usr/bin/git', ['-C', rig.work, ...args], { stdio: 'ignore' })
  const errors: string[] = []
  page.on('pageerror', error => errors.push(error.message))
  try {
    git('init', '-b', 'topic')
    git('-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid', '-c', 'commit.gpgsign=false', 'commit', '--allow-empty', '-m', 'Initial')
    git('branch', 'dev'); git('branch', 'release/next')
    const staticRoot = await buildStudio(join(rig.root, 'studio')), port = await unusedPort()
    await Effect.runPromise(Effect.scoped(Effect.gen(function* () {
      const runner = yield* makeFakeRunner()
      const dependencies = Layer.mergeAll(runner.layer, makeFakeSecrets().layer, Layer.succeed(Redactor, {
        redact: event => Effect.succeed(event), redactText: text => Effect.succeed(text)
      }))
      yield* Effect.scoped(Effect.gen(function* () {
        yield* registerRebuildCheck()
        const project = yield* decisionSetup(rig.work)
        yield* completedSetup(project.projectId)
        success(yield* call(ownerCredential, 'project.settings.set', { settings: { workspaceFolder: rig.root, shell: 'sh' } }))
        yield* appendSetup([eventOf({ _tag: 'project.RepositoryBaseChanged', repoId: project.repoId, baseBranch: 'dev' })])
        yield* serve({ password: Redacted.make(password), staticRoot, lists: engineLists, settings: { enabled: true, port, heartbeatSeconds: 25, proxyMode: false } })
        yield* Effect.promise(async () => {
          await page.goto(`http://127.0.0.1:${port}/`)
          await page.locator('#login-password').fill(password)
          await page.getByRole('button', { name: 'Sign in', exact: true }).click()
          await page.goto(`http://127.0.0.1:${port}/#/r/${project.repoId}`)
          await expect(page.locator('.repo-facts dt').filter({ hasText: 'Base branch' }).locator('+ dd')).toHaveText('dev')
        })
        success(yield* call(ownerCredential, 'project.repository.base.set', { repoId: project.repoId, baseBranch: 'release/next' }))
        yield* Effect.promise(async () => {
          await expect(page.locator('.repo-facts dt').filter({ hasText: 'Base branch' }).locator('+ dd')).toHaveText('release/next')
          await page.reload()
          await expect(page.locator('.repo-facts dt').filter({ hasText: 'Base branch' }).locator('+ dd')).toHaveText('release/next')
          await expect(page.locator('#ps-base')).toHaveCount(0)
          expect(errors).toEqual([])
        })
      }).pipe(Effect.provide(buildServices(rig, engineLists).pipe(Layer.provide(dependencies))), Effect.provide(runner.layer)))
    })).pipe(Effect.provide(SqliteClient.layer({ filename: rig.database, readerWorker: true }))))
  } finally {
    if (previous === undefined) delete process.env.LGTM_HOME; else process.env.LGTM_HOME = previous
    removeRig(rig)
  }
})
