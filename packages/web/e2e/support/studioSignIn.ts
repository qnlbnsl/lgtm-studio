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
import { Redactor } from '../../../engine/src/log/Redactor.ts'
import { fixtureHarnessPrograms } from '../../../engine/test/support/fixtureHarnessPrograms.ts'
import { unusedPort } from '../../../engine/test/support/browserWeb.ts'
import { makeFakeRunner } from '../../../engine/test/support/fakeRunner.ts'
import { appendSetup, eventOf, harnessSettings, makeFakeSecrets, makeRig, readLog, removeRig, seedAgent } from '../../../engine/test/support/handlerRig.ts'
import { registerRebuildCheck } from '../../../engine/test/support/rebuildCheck.ts'
import { buildStudio } from '../../../studio/scripts/build.ts'
import { completedSetup } from './completedSetup.ts'
async function withPage(page: Page, journey: (root: string) => Promise<void>) {
  const errors: string[] = []; page.on('pageerror', e => errors.push(e.message))
  const rig = makeRig(), password = randomBytes(32).toString('base64url'), port = await unusedPort()
  const script = join(rig.root, 'login.cjs')
  writeFileSync(script, `if(!process.stdin.isTTY||!process.stdout.isTTY)process.exit(88);process.stdout.write('Fake sign-in code https://example.test/signin\\r\\nPress y to finish');process.stdin.setRawMode(true);process.stdin.once('data',()=>{const fs=require('node:fs'),p=require('node:path');fs.mkdirSync(process.env.CLAUDE_CONFIG_DIR,{recursive:true});fs.writeFileSync(p.join(process.env.CLAUDE_CONFIG_DIR,'.credentials.json'),JSON.stringify({claudeAiOauth:{accessToken:'fake-credential'}}));process.exit(0)});`)
  const base = fixtureHarnessPrograms({ home: rig.home, executable: process.execPath, script, id: 'claude', name: 'Claude Code' })
  const harnessPrograms = { ...base, recipe: (id: string) => { const r = base.recipe(id); return r ? { ...r, signInArguments: [script] } : undefined } }
  try {
    const studioRoot = await buildStudio(join(rig.root, 'studio')), staticRoot = join(rig.root, 'web')
    mkdirSync(staticRoot); writeFileSync(join(staticRoot, 'index.html'), '<title>Test</title>')
    await Effect.runPromise(Effect.scoped(Effect.gen(function* () {
      const runner = yield* makeFakeRunner()
      const dependencies = Layer.mergeAll(runner.layer, makeFakeSecrets().layer, Layer.succeed(Redactor, { redact: e => Effect.succeed(e), redactText: s => Effect.succeed(s) }))
      yield* Effect.scoped(Effect.gen(function* () {
        yield* registerRebuildCheck()
        const lead = yield* seedAgent({ id: 'setup-history-lead', role: 'project-lead', name: 'Project lead', state: 'idle', folder: rig.work })
        yield* appendSetup([
          eventOf({ _tag: 'project.RepositoryAdded', repoId: 'setup-history-repo', name: 'Setup history', path: rig.work }),
          eventOf({ _tag: 'project.RepositoryLocated', repoId: 'setup-history-repo', machineId: 'local', path: rig.work }),
          eventOf({ _tag: 'project.WorktreeCreated', repoId: 'setup-history-repo', worktreeId: 'setup-history-worktree', branch: 'main', path: rig.work, machineId: 'local' }),
          eventOf({ _tag: 'project.Created', projectId: 'setup-history-project', repoId: 'setup-history-repo', worktreeId: 'setup-history-worktree', leadAgentId: lead.agentId, name: 'Setup history', goal: 'Test project', machineId: 'local' })
        ])
        yield* completedSetup('setup-history-project', rig.work)
        yield* appendSetup([eventOf({ _tag: 'HarnessChanged', harness: harnessSettings('claude', { adapterPath: process.execPath, models: [], efforts: [] }) })])
        yield* serve({ password: Redacted.make(password), staticRoot, studioRoot, lists: engineLists, settings: { enabled: true, port, heartbeatSeconds: 25, proxyMode: false } })
        yield* Effect.promise(async () => {
          const root = `http://127.0.0.1:${port}/studio/`
          await page.setViewportSize({ width: 1600, height: 1000 }); await page.goto(root)
          await page.locator('#login-password').fill(password)
          await page.getByRole('button', { name: 'Sign in', exact: true }).click()
          await page.locator('.sidebar').getByRole('link', { name: 'Harnesses', exact: true }).click()
          await expect(page.getByRole('heading', { name: 'Harnesses', exact: true })).toBeVisible()
          await expect(page.locator('#harness-claude')).toBeVisible()
          await journey(root)
          expect(errors).toEqual([])
        })
        expect(JSON.stringify(yield* readLog).includes('Fake sign-in code')).toBe(false)
      })).pipe(Effect.provide(buildServices({ ...rig, harnessPrograms }, engineLists).pipe(Layer.provide(dependencies))), Effect.provide(runner.layer))
    })).pipe(Effect.provide(SqliteClient.layer({ filename: rig.database, readerWorker: true })), Effect.scoped) as Effect.Effect<void, unknown, never>)
  } finally { removeRig(rig) }
}
export async function exerciseStudioSignIn(page: Page) {
  await withPage(page, async () => {
    await page.locator('#harness-claude').getByRole('button', { name: 'Add account', exact: true }).click()
    const dialog = page.getByRole('dialog')
    await dialog.locator('#as-name').fill('Owner separate sign-in')
    await dialog.getByLabel('A separate sign-in', { exact: true }).check()
    await dialog.getByRole('button', { name: 'Sign in here', exact: true }).click()
    await expect(dialog.getByRole('log', { name: 'Sign-in output' })).toContainText('Press y to finish')
    await expect(dialog.getByRole('link', { name: 'https://example.test/signin', exact: true })).toHaveAttribute('target', '_blank')
    await dialog.getByRole('textbox', { name: 'Sign-in terminal keys' }).press('y')
    await expect(dialog.getByRole('status')).toContainText('Local sign-in found')
    await dialog.getByRole('button', { name: 'Done', exact: true }).click()
    await expect(page.locator('#harness-claude .svc-sub')).toContainText('Owner separate sign-in')
  })
  await exerciseSampleSignIn(page)
}
export async function exerciseHarnessesThenChat(page: Page) {
  await withPage(page, async root => {
    await page.goto(root + '#/p/setup-history-project/chat')
    await expect(page.getByRole('textbox', { name: /message|prompt/i }).first()).toBeVisible()
  })
}

async function exerciseSampleSignIn(page: Page) {
  const { createServer } = await import('node:http')
  const { readFile } = await import('node:fs/promises')
  const { extname } = await import('node:path')
  const { fileURLToPath } = await import('node:url')
  const design = fileURLToPath(new URL('../../../../design/', import.meta.url))
  const types: Record<string, string> = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.woff2': 'font/woff2', '.svg': 'image/svg+xml' }
  const server = createServer(async (request, response) => {
    const path = decodeURIComponent((request.url ?? '/').split('?')[0]!), file = path.endsWith('/') ? path + 'index.html' : path
    try {
      if (path.includes('..')) throw new Error()
      const body = await readFile(join(design, file))
      response.setHeader('Content-Type', types[extname(file)] ?? 'application/octet-stream'); response.end(body)
    } catch { response.statusCode = 404; response.end() }
  })
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve))
  try {
    await page.goto(`http://127.0.0.1:${(server.address() as import('node:net').AddressInfo).port}/mock/#/harnesses`)
    await page.locator('#harness-claude').getByRole('button', { name: 'Add account', exact: true }).click()
    const dialog = page.getByRole('dialog')
    await dialog.locator('#as-name').fill('Sample separate sign-in')
    await expect(dialog.getByLabel('A separate sign-in', { exact: true })).toBeChecked()
    await dialog.getByRole('button', { name: 'Sign in here', exact: true }).click()
    await expect(dialog.getByRole('log', { name: 'Sign-in output' })).toContainText('Sample sign-in code')
    await dialog.getByRole('textbox', { name: 'Sign-in terminal keys' }).press('y')
    await expect(dialog.getByRole('status')).toContainText('Local sign-in found')
    await dialog.getByRole('button', { name: 'Done', exact: true }).click()
  } finally { server.closeAllConnections(); await new Promise<void>(resolve => server.close(() => resolve())) }
}
