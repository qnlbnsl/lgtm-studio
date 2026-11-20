import { execFileSync } from 'node:child_process'
import { randomBytes } from 'node:crypto'
import { readFile } from 'node:fs/promises'
import { createServer } from 'node:http'
import { extname, join, resolve, sep } from 'node:path'
import { fileURLToPath } from 'node:url'
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
import { call, ownerCredential, success, makeFakeSecrets, makeRig, removeRig } from '../../engine/test/support/handlerRig.ts'
import { registerRebuildCheck } from '../../engine/test/support/rebuildCheck.ts'
import { decisionSetup } from '../../engine/test/support/projectDecisionFixtures.ts'
import { buildStudio } from '../../studio/scripts/build.ts'
import { completedSetup } from './support/completedSetup.ts'

test('Service settings saves a wider workspace after reload and displays the registered repository refusal', async ({ page }) => {
  test.setTimeout(90_000)
  const rig = makeRig(), previous = process.env.LGTM_HOME, previousHome = process.env.HOME
  process.env.LGTM_HOME = rig.stateFolder; process.env.HOME = rig.home
  const password = randomBytes(32).toString('base64url'), errors: string[] = []
  page.on('pageerror', error => errors.push(error.message))
  try {
    execFileSync('git', ['init', '-b', 'main', rig.work], { stdio: 'ignore' })
    execFileSync('git', ['-C', rig.work, '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.test', '-c', 'commit.gpgsign=false', 'commit', '--allow-empty', '-m', 'Initial'], { stdio: 'ignore' })
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
        const settings = { workspaceFolder: rig.work, shell: '/bin/sh', displayName: 'Maya Okafor',
          reviewDefaults: { reviewers: 3, differentVendor: false },
          lead: { harness: 'first', account: { _tag: 'named', accountId: 'primary' }, permission: 'ask', permissionChosen: true } }
        success(yield* call(ownerCredential, 'project.settings.set', { settings }))
        yield* serve({ password: Redacted.make(password), staticRoot, lists: engineLists,
          settings: { enabled: true, port, heartbeatSeconds: 25, proxyMode: false } })
        yield* Effect.promise(async () => {
          const root = `http://127.0.0.1:${port}/`
          await page.goto(root)
          await page.locator('#login-password').fill(password)
          await page.getByRole('button', { name: 'Sign in', exact: true }).click()
          await expect(page.locator('.sidebar .tree-row.project')).toBeVisible()
          await page.goto(`${root}#/settings?section=defaults`)
          const field = page.getByLabel('Where is your workspace?', { exact: true })
          await expect(field).toHaveValue(rig.work)
          await expect(page.locator('.svc-pane')).toContainText("LGTM Studio's Git commands work in every repository under this folder.")
          await field.fill(rig.root); await field.press('Tab')
          await expect.poll(async () => page.evaluate(async () => {
            const engine = await import('/js/engine.js' as string)
            return (await engine.request('project.settings', {})).settings.workspaceFolder
          })).toBe(rig.root)
          await expect(page.getByText('Saving…', { exact: true })).toHaveCount(0)
          await page.reload()
          await expect(field).toHaveValue(rig.root)
          await field.fill(rig.home); await field.press('Tab')
          await expect(page.locator('.toasts')).toContainText(`The workspace folder must contain the registered repository "Repository" (${rig.work}).`)
          await expect(field).toHaveValue(rig.root)
          await page.reload()
          await expect(field).toHaveValue(rig.root)
          await field.fill(join(rig.root, 'missing')); await field.press('Tab')
          await expect(page.locator('.toasts')).toContainText('workspace folder must be an existing absolute directory')
          await expect(field).toHaveValue(rig.root)
          expect(errors).toEqual([])
        })
        expect(success(yield* call(ownerCredential, 'project.settings', {})).settings).toEqual({ ...settings, workspaceFolder: rig.root })
      }).pipe(Effect.provide(buildServices(rig, engineLists).pipe(Layer.provide(dependencies))), Effect.provide(runner.layer)))
    })).pipe(Effect.provide(SqliteClient.layer({ filename: rig.database, readerWorker: true }))))
  } finally {
    if (previous === undefined) delete process.env.LGTM_HOME; else process.env.LGTM_HOME = previous
    if (previousHome === undefined) delete process.env.HOME; else process.env.HOME = previousHome
    removeRig(rig)
  }
})

test('the sample workspace setting remains editable at desktop and narrow widths', async ({ page }) => {
  const design = resolve(fileURLToPath(new URL('../../../design/', import.meta.url)))
  const types: Record<string, string> = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.json': 'application/json', '.woff2': 'font/woff2' }
  const server = createServer(async (request, response) => {
    const path = resolve(design, '.' + new URL(request.url!, 'http://localhost').pathname)
    if (!path.startsWith(design + sep) && path !== design) { response.writeHead(403).end(); return }
    try { response.setHeader('Content-Type', types[extname(path)] || 'application/octet-stream'); response.end(await readFile(path)) }
    catch { response.writeHead(404).end() }
  })
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve))
  try {
    const address = server.address(); if (!address || typeof address === 'string') throw new Error('Missing sample server address')
    await page.goto(`http://127.0.0.1:${address.port}/mock/index.html#/settings?section=defaults`)
    const field = page.getByLabel('Where is your workspace?', { exact: true })
    await expect(field).toHaveValue('/home/maya/projects')
    await field.fill('/home/maya'); await field.press('Tab')
    await page.getByRole('link', { name: 'Appearance', exact: true }).click()
    await page.getByRole('link', { name: 'Defaults', exact: true }).click()
    await expect(field).toHaveValue('/home/maya')
    await field.fill('/home/maya/projects'); await field.press('Tab')
    for (const [name, width, height] of [['desktop', 1440, 1000], ['narrow', 390, 844]] as const) {
      await page.setViewportSize({ width, height })
      await expect(field).toBeVisible()
      const box = await field.boundingBox()
      expect(box!.x).toBeGreaterThanOrEqual(0)
      expect(box!.x + box!.width).toBeLessThanOrEqual(width)
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
      await page.screenshot({ path: fileURLToPath(new URL(`../../../design/builds/workspace-settings-${name}.png`, import.meta.url)), animations: 'disabled' })
    }
  } finally { await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve())) }
})
