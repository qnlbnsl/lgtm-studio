import { randomBytes } from 'node:crypto'
import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, type Page } from '@playwright/test'
import * as Effect from 'effect/Effect'
import * as Layer from 'effect/Layer'
import * as Redacted from 'effect/Redacted'
import * as SqliteClient from '../../../../third_party/t3code/node-sqlite/src/nodeSqliteClient.ts'
import { buildServices } from '../../../engine/src/service.ts'
import { serve } from '../../../engine/src/client/web/server.ts'
import { engineLists } from '../../../engine/src/pieces/lists.ts'
import { refused, type Input } from '../../../engine/src/commands/internal.ts'
import { Redactor } from '../../../engine/src/log/Redactor.ts'
import { unusedPort } from '../../../engine/test/support/browserWeb.ts'
import { makeFakeRunner } from '../../../engine/test/support/fakeRunner.ts'
import { call, ownerCredential, success, makeFakeSecrets, makeRig, removeRig } from '../../../engine/test/support/handlerRig.ts'
import { registerRebuildCheck } from '../../../engine/test/support/rebuildCheck.ts'
import { decisionSetup } from '../../../engine/test/support/projectDecisionFixtures.ts'
import { buildStudio } from '../../../studio/scripts/build.ts'
import { completedSetup } from './completedSetup.ts'

export async function exerciseStudioProjectSettings(page: Page) {
  const rig = makeRig(), previous = process.env.LGTM_HOME
  process.env.LGTM_HOME = rig.stateFolder
  const password = randomBytes(32).toString('base64url')
  let refuseModel = false
  const lists = { ...engineLists, planners: { ...engineLists.planners,
    configure: (input: Input, payload: Record<string, unknown>) => refuseModel
      ? Effect.fail(refused()) : engineLists.planners.configure!(input, payload)
  } }
  const staticRoot = join(rig.root, 'static'); mkdirSync(staticRoot)
  writeFileSync(join(staticRoot, 'index.html'), '<title>Test service</title>')
  const errors: string[] = []; page.on('pageerror', (error) => errors.push(error.message))
  await page.addInitScript(() => { document.addEventListener('securitypolicyviolation', (event) => { (window as any).settingsPolicyViolations ||= []; (window as any).settingsPolicyViolations.push(event.violatedDirective) }) })
  try {
    const studioRoot = await buildStudio(join(rig.root, 'studio')), port = await unusedPort()
    await Effect.runPromise(Effect.scoped(Effect.gen(function* () {
      const runner = yield* makeFakeRunner()
      const dependencies = Layer.mergeAll(runner.layer, makeFakeSecrets().layer, Layer.succeed(Redactor, { redact: (event) => Effect.succeed(event), redactText: (text) => Effect.succeed(text) }))
      yield* Effect.scoped(Effect.gen(function* () {
        yield* registerRebuildCheck()
        const project = yield* decisionSetup(); yield* completedSetup(project.projectId)
        success(yield* call(ownerCredential, 'project.settings.set', { settings: { workspaceFolder: rig.work, shell: '/bin/sh' } }))
        runner.script('choices', () => Effect.promise(async () => {
          return { models: { source: 'negotiated' as const, options: [{ id: 'test-model', label: 'Engine model' }, { id: 'other-model', label: 'Other engine model' }] },
            efforts: { source: 'negotiated' as const, options: [{ id: 'high', label: 'High' }, { id: 'low', label: 'Low' }] } }
        }))
        yield* serve({ password: Redacted.make(password), staticRoot, studioRoot, lists, settings: { enabled: true, port, heartbeatSeconds: 25, proxyMode: false } })
        yield* Effect.promise(async () => {
          const root = `http://127.0.0.1:${port}/studio/`
          await page.goto(root)
          await page.locator('#login-password').fill(password)
          await page.getByRole('button', { name: 'Sign in', exact: true }).click()
          await expect(page.locator('.sidebar .tree-row.project')).toBeVisible()
          const settings = `${root}#/p/${project.projectId}/settings`
          await page.goto(settings)
          const screen = page.locator('.svc-project')
          await expect(screen).toContainText('Project settings')
          await expect(screen).toContainText('Repository')
          await expect(screen).toContainText('topic')
          await expect(screen).toContainText('Primary')
          await expect(screen.locator('.svc-teams tbody')).toContainText('Build')
          await expect(screen.locator('.svc-teams tbody a')).toHaveCount(2)
          for (const selector of ['#ps-base', '#ps-sub', '[name=ps-perm]', '[aria-label="Write tests first"]', '[aria-label="Agent review"]', '[aria-label="Remove agent credits when merging"]', '#ps-all', '#ps-prefix', '.svc-assign select', '.svc-teams [role=switch]']) {
            const controls = screen.locator(selector)
            expect(await controls.count()).toBeGreaterThan(0)
            for (const control of await controls.all()) {
              await expect(control).toBeDisabled()
              await expect(control.locator('xpath=ancestor-or-self::*[@title="Engine work needed"]').first()).toHaveAttribute('title', 'Engine work needed')
            }
          }
          for (const name of ['Approve', 'Apply', 'Delete']) {
            await expect(screen.getByRole('button', { name, exact: true })).toBeDisabled()
            await expect(screen.getByRole('button', { name, exact: true })).toHaveAttribute('title', 'Engine work needed')
          }
          await expect(screen.locator('#ps-machine')).toHaveCount(0)
          await screen.getByRole('button', { name: 'Rename', exact: true }).click()
          await page.locator('#rename-field').fill('Settings journey')
          await page.getByRole('dialog').getByRole('button', { name: 'Rename', exact: true }).click()
          await expect(page.getByRole('dialog')).toHaveCount(0)
          await expect(page.locator('.sidebar .tree-row.project')).toContainText('Settings journey')
          await page.locator('#ps-goal').fill('A goal saved through the live settings screen')
          await screen.getByRole('button', { name: 'Save goal', exact: true }).click()
          await expect(screen.getByRole('button', { name: 'Save goal', exact: true })).toHaveCount(0)
          const model = screen.getByRole('combobox', { name: 'Lead model', exact: true })
          const effort = screen.getByRole('combobox', { name: 'Lead effort', exact: true })
          await expect(model.locator('option')).toContainText(['Engine model', 'Other engine model'])
          await expect(model).toBeEnabled()
          await model.selectOption('other-model')
          await expect(model).toBeEnabled()
          await expect(model).toHaveValue('other-model')
          await effort.selectOption('low')
          await expect(effort).toBeEnabled()
          await expect(effort).toHaveValue('low')
          await page.reload()
          await expect(screen).toContainText('Settings journey')
          await expect(page.locator('#ps-goal')).toHaveValue('A goal saved through the live settings screen')
          await expect(model).toHaveValue('other-model')
          await expect(effort).toHaveValue('low')
          // Refuse the write while keeping the same choices available on readback.
          refuseModel = true
          await model.selectOption('test-model')
          await expect(page.locator('.toasts')).toContainText('command refused')
          await expect(model).toBeEnabled()
          await expect(model).toHaveValue('other-model')
          await expect(model.locator('option')).toContainText(['Engine model', 'Other engine model'])
          refuseModel = false
          await page.reload()
          await expect(model).toHaveValue('other-model')
          await screen.getByRole('button', { name: 'Archive', exact: true }).click()
          await page.getByRole('dialog').getByRole('button', { name: 'Archive project', exact: true }).click()
          await expect(page).toHaveURL(new RegExp('#/r/'))
          await page.goto(settings)
          await expect(screen).toContainText('This project is archived.')
          await screen.locator('.svc-danger').getByRole('button', { name: 'Restore', exact: true }).click()
          await expect(screen).not.toContainText('This project is archived.')
          await expect(page.locator('.sidebar .tree-row.project')).toContainText('Settings journey')
          await expect.poll(() => page.evaluate(() => (window as any).settingsPolicyViolations || [])).toEqual([])
          expect(errors).toEqual([])
        })
      }).pipe(Effect.provide(buildServices(rig, lists).pipe(Layer.provide(dependencies))), Effect.provide(runner.layer)))
    })).pipe(Effect.provide(SqliteClient.layer({ filename: rig.database, readerWorker: true }))))
  } finally { if (previous === undefined) delete process.env.LGTM_HOME; else process.env.LGTM_HOME = previous; removeRig(rig) }
}
