import { randomBytes } from 'node:crypto'
import { cpSync, readFileSync, writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'
import { expect, type Page } from '@playwright/test'
import * as Deferred from 'effect/Deferred'
import * as Effect from 'effect/Effect'
import * as Layer from 'effect/Layer'
import * as Redacted from 'effect/Redacted'
import * as SqliteClient from '../../../../third_party/t3code/node-sqlite/src/nodeSqliteClient.ts'
import { buildServices } from '../../../engine/src/service.ts'
import { serve } from '../../../engine/src/client/web/server.ts'
import { Restart } from '../../../engine/src/recovery/service.ts'
import { engineLists } from '../../../engine/src/pieces/lists.ts'
import { Redactor } from '../../../engine/src/log/Redactor.ts'
import { unusedPort } from '../../../engine/test/support/browserWeb.ts'
import { makeFakeRunner } from '../../../engine/test/support/fakeRunner.ts'
import { appendSetup, eventOf, makeFakeSecrets, makeRig, removeRig, call, success, ownerCredential } from '../../../engine/test/support/handlerRig.ts'
import { registerRebuildCheck } from '../../../engine/test/support/rebuildCheck.ts'
import { decisionSetup } from '../../../engine/test/support/projectDecisionFixtures.ts'
import { buildStudio } from '../../../studio/scripts/build.ts'
import { completedSetup } from './completedSetup.ts'

export async function exerciseStartup(page: Page, kind: 'startup' | 'drafts' | 'plans' | 'mock-drafts') {
  const rig = makeRig(), previous = process.env.LGTM_HOME
  process.env.LGTM_HOME = rig.stateFolder
  const password = randomBytes(32).toString('base64url'), port = await unusedPort()
  const errors: string[] = []
  page.on('pageerror', error => errors.push(error.message))
  try {
    const staticRoot = await buildStudio(join(rig.root, 'studio'))
    if (kind === 'mock-drafts') {
      // Serve an isolated copy of the sample through the test engine's root.
      // The sample entry point never opens the engine connection.
      cpSync(fileURLToPath(new URL('../../../../design/mock/js/', import.meta.url)), join(staticRoot, 'js'), { recursive: true })
      const index = join(staticRoot, 'index.html')
      writeFileSync(index, readFileSync(index, 'utf8').replace('src="live.js"', 'src="js/main.js"'))
    }
    await Effect.runPromise(Effect.scoped(Effect.gen(function* () {
      const runner = yield* makeFakeRunner()
      const dependencies = Layer.mergeAll(runner.layer, makeFakeSecrets().layer, Layer.succeed(Redactor, { redact: event => Effect.succeed(event), redactText: text => Effect.succeed(text) }))
      yield* Effect.scoped(Effect.gen(function* () {
        yield* registerRebuildCheck()
        const project = yield* decisionSetup(rig.root); yield* completedSetup(project.projectId)
        const channel = success(yield* call(ownerCredential, 'board.open', { name: 'draft-check' }))
        success(yield* call(ownerCredential, 'board.members', { channelId: channel.channelId, add: [project.leadId, project.workerId], remove: [] }))
        const context = yield* Effect.context<Effect.Services<ReturnType<typeof appendSetup>>>()
        const append = (events: Parameters<typeof appendSetup>[0]) => Effect.runPromise(appendSetup(events).pipe(Effect.provide(context)))
        const plan = (status: string) => eventOf({ _tag: 'ThreadItem', agentId: project.leadId, itemId: 'plan-' + status, replay: false, content: { _tag: 'plan', entries: [{ content: 'Check startup', status, priority: 'high' }] } })
        let ready = kind !== 'startup', recovered = 0
        const gate = Deferred.makeUnsafe<void>()
        const recovery = Restart.of({ ready: Effect.sync(() => ready), whenReady: Deferred.await(gate), recover: Effect.void, returnAccount: () => Effect.void,
          progress: Effect.sync(() => ({ _tag: 'ServiceStarting' as const, recovered, total: 2 })) })
        yield* serve({ password: Redacted.make(password), staticRoot, lists: engineLists, settings: { enabled: true, port, heartbeatSeconds: 25, proxyMode: false } }).pipe(Effect.provideService(Restart, recovery))
        yield* Effect.promise(async () => {
          const root = `http://127.0.0.1:${port}/`
          await page.goto(kind === 'mock-drafts' ? root + '#/p/console/chat/console-lead' : root)
          if (kind === 'mock-drafts') {
            const chatRoute = `${root}#/p/console/chat/console-lead`
            const composer = page.locator('.chat-main .composer textarea')
            await composer.fill('Sample agent draft')
            await page.reload(); await expect(composer).toHaveValue('Sample agent draft')
            await page.goto(`${root}#/p/console/messages/ch-console-shell`)
            const board = page.locator('.board-composer textarea').first()
            await board.fill('Sample channel draft')
            await page.reload(); await expect(board).toHaveValue('Sample channel draft')
            await board.press('Enter'); await expect(board).toHaveValue('')
            await page.reload(); await expect(board).toHaveValue('')
            await page.goto(chatRoute); await expect(composer).toHaveValue('Sample agent draft')
            await composer.press('Enter'); await expect(composer).toHaveValue('')
            await page.reload(); await expect(composer).toHaveValue('')
            return
          }
          await page.locator('#login-password').fill(password)
          await page.getByRole('button', { name: 'Sign in', exact: true }).click()
          await expect(page.locator('#login-password')).toHaveCount(0)
          expect(await page.evaluate(() => JSON.stringify(sessionStorage))).not.toContain(password)
          if (kind === 'startup') {
            await expect(page.getByRole('button', { name: 'Connection: Starting · 0/2', exact: true })).toBeVisible()
            await page.reload()
            await expect(page.getByRole('button', { name: 'Connection: Starting · 0/2', exact: true })).toBeVisible()
            recovered = 1
            const connection = page.getByRole('button', { name: 'Connection: Starting · 1/2', exact: true })
            await expect(connection).toBeVisible(); await connection.click()
            await expect(page.getByText('The service is starting. 1 of 2 agents recovered.', { exact: true })).toBeVisible()
            ready = true; await Effect.runPromise(Deferred.succeed(gate, undefined))
            await expect(page.getByRole('button', { name: 'Connection: Live', exact: true })).toBeVisible()
            await expect(page.getByText('The service is starting.', { exact: false })).toHaveCount(0)
            await page.goto(`${root}#/p/${project.projectId}/chat/${project.leadId}`)
            await expect(page.locator('.chat-main .composer textarea')).toBeVisible()
            return
          }
          const chatRoute = `${root}#/p/${project.projectId}/chat/${project.leadId}`
          await page.goto(chatRoute)
          const thread = page.locator('.chat-main .thread'), composer = page.locator('.chat-main .composer textarea')
          await expect(page.getByRole('button', { name: 'Connection: Live', exact: true })).toBeVisible()
          if (kind === 'plans') {
            await append([plan('pending')])
            const row = thread.locator('.msg').filter({ hasText: 'Check startup' })
            await expect(row).toHaveCount(1); await expect(row).toContainText('pending: Check startup')
            const original = await row.elementHandle()
            await append([eventOf({ _tag: 'ThreadItem', agentId: project.leadId, itemId: 'between', replay: false, content: { _tag: 'text', text: 'Between plan updates.' } }), plan('completed')])
            await expect(row).toHaveCount(1); await expect(row).toContainText('completed: Check startup'); await expect(row).not.toContainText('pending:')
            expect(await row.evaluate((node, previous) => node === previous, original)).toBe(true)
            await expect(thread.locator('.msg').filter({ hasText: 'Between plan updates.' })).toHaveCount(1)
            await append([eventOf({ _tag: 'TurnEnded', agentId: project.leadId, compact: false }), plan('in_progress')])
            await expect(row).toHaveCount(2)
            await expect(row.nth(0)).toContainText('completed: Check startup'); await expect(row.nth(1)).toContainText('in_progress: Check startup')
            await page.reload(); await expect(row).toHaveCount(2)
            await expect(row.nth(0)).toContainText('completed: Check startup'); await expect(row.nth(1)).toContainText('in_progress: Check startup')
          } else {
            await composer.fill('Unsent agent draft')
            const boardRoute = `${root}#/p/${project.projectId}/messages/${channel.channelId}`
            await page.goto(boardRoute)
            const board = page.locator('.board-composer textarea').first()
            await board.fill('Unsent channel draft')
            await page.reload(); await expect(board).toHaveValue('Unsent channel draft')
            await page.goto(chatRoute); await expect(composer).toHaveValue('Unsent agent draft')
            await page.reload(); await expect(composer).toHaveValue('Unsent agent draft')
            const otherTab = await page.context().newPage()
            await otherTab.goto(chatRoute); await expect(otherTab.locator('.chat-main .composer textarea')).toHaveValue(''); await otherTab.close()
            await composer.press('Enter'); await expect(composer).toHaveValue('')
            await page.reload(); await expect(composer).toHaveValue('')
            await page.goto(boardRoute); await expect(board).toHaveValue('Unsent channel draft')
            await board.press('Enter'); await expect(board).toHaveValue('')
            await page.reload(); await expect(board).toHaveValue('')
          }
        })
      })).pipe(Effect.provide(buildServices(rig, engineLists).pipe(Layer.provide(dependencies))), Effect.provide(runner.layer))
    })).pipe(Effect.provide(SqliteClient.layer({ filename: rig.database, readerWorker: true })), Effect.scoped) as Effect.Effect<void, unknown, never>)
    expect(errors).toEqual([])
  } finally {
    if (previous === undefined) delete process.env.LGTM_HOME; else process.env.LGTM_HOME = previous
    removeRig(rig)
  }
}
