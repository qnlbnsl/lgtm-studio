import { randomBytes } from 'node:crypto'
import { join } from 'node:path'
import { expect, type Page } from '@playwright/test'
import * as Effect from 'effect/Effect'
import * as Layer from 'effect/Layer'
import * as Redacted from 'effect/Redacted'
import * as SqliteClient from '../../../../third_party/t3code/node-sqlite/src/nodeSqliteClient.ts'
import { buildServices } from '../../../engine/src/service.ts'
import { serve } from '../../../engine/src/client/web/server.ts'
import { CommandHandler } from '../../../engine/src/commands/CommandHandler.ts'
import { Redactor } from '../../../engine/src/log/Redactor.ts'
import { unusedPort } from '../../../engine/test/support/browserWeb.ts'
import { makeFakeRunner } from '../../../engine/test/support/fakeRunner.ts'
import { call, freshRequest, harnessSettings, ownerCredential, success, makeFakeSecrets, makeRig, removeRig } from '../../../engine/test/support/handlerRig.ts'
import { registerRebuildCheck } from '../../../engine/test/support/rebuildCheck.ts'
import { buildStudio } from '../../../studio/scripts/build.ts'
import { unwrap } from '../../../engine/test/client/support/readRig.ts'
import { completedSetup } from './completedSetup.ts'

export async function exerciseStudioLeadDefinition(page: Page) {
  const rig = makeRig(), previous = process.env.LGTM_HOME
  process.env.LGTM_HOME = rig.stateFolder
  const password = randomBytes(32).toString('base64url'), errors: string[] = []
  page.on('pageerror', (error) => errors.push(error.message))
  try {
    const staticRoot = await buildStudio(join(rig.root, 'studio')), port = await unusedPort()
    await Effect.runPromise(Effect.scoped(Effect.gen(function* () {
      const runner = yield* makeFakeRunner()
      const dependencies = Layer.mergeAll(runner.layer, makeFakeSecrets().layer, Layer.succeed(Redactor, { redact: (event) => Effect.succeed(event), redactText: (text) => Effect.succeed(text) }))
      yield* Effect.scoped(Effect.gen(function* () {
        yield* registerRebuildCheck()
        success(yield* call(ownerCredential, 'harnesses', { operation: 'set', harness: harnessSettings('cursor', { adapterPath: process.execPath, reportedModes: [{ id: 'default', level: 'ask' }] }) }))
        success(yield* call(ownerCredential, 'accounts', { operation: 'add', account: { id: 'ready', harness: 'cursor', enabled: true } }))
        success(yield* call(ownerCredential, 'accounts set-secret', { accountId: 'ready', name: 'CURSOR_API_KEY', nonce: freshRequest(), value: Redacted.make('synthetic-browser-readiness-key') }))
        success(yield* call(ownerCredential, 'project.settings.set', { settings: { workspaceFolder: rig.work, shell: '/bin/sh', displayName: 'Maya Okafor' } }))
        yield* completedSetup()
        const handler = yield* CommandHandler
        const execute = (credential: { _tag: 'agent', token: Redacted.Redacted<string> } | typeof ownerCredential, command: string, payload: Record<string, unknown>) => Effect.runPromise(handler.execute(credential, command, { requestId: freshRequest(), ...payload }).pipe(Effect.result))
        yield* serve({ password: Redacted.make(password), staticRoot, settings: { enabled: true, port, heartbeatSeconds: 25, proxyMode: false } })
        yield* Effect.promise(async () => {
          const root = `http://127.0.0.1:${port}/`
          await page.goto(root)
          await page.locator('#login-password').fill(password)
          await page.getByRole('button', { name: 'Sign in', exact: true }).click()
          await expect(page.locator('.sidebar')).toContainText('Maya Okafor')
          await page.locator('.sidebar').getByRole('button', { name: 'New project', exact: true }).click()
          await page.locator('#np-repo').selectOption('__new')
          await page.locator('#np-newrepo').fill('Repository')
          await page.locator('#np-goal').fill('Build first goal')
          await page.getByRole('button', { name: 'Create project', exact: true }).click()
          const tree = page.locator('.sidebar .tree-row.project').filter({ hasText: 'First goal' })
          await expect(tree).toBeVisible()
          const created = await execute(ownerCredential, 'client.snapshot', {})
          if (created._tag !== 'Success') throw new Error('Snapshot unavailable')
          const projectId = (created.success as any).project.projects.find((p: any) => p.name === 'First goal').projectId
          const spawn = runner.callsOf('start').at(-1)!.input
          const lead = { _tag: 'agent' as const, token: spawn.token }
          const orientation = await execute(lead, 'roles.orient', {})
          if (orientation._tag !== 'Success') throw new Error('Orientation unavailable')
          expect(unwrap(orientation.success as { data: string })).toMatchObject({ kind: 'lead', name: 'Project lead' })
          await expect(tree).toContainText('First goal')
          await expect(tree).toContainText('Define')
          const sprint = { projectId, label: 'SP-1', name: 'First sprint', goal: 'Deliver the agreed scope', phase: 'Build', owner: spawn.agentId, needs: [] }
          const refused = await execute(lead, 'project.sprint.create', sprint)
          expect(refused._tag === 'Failure' && refused.failure.reason).toBe('The owner must approve the end result or skipping planning before work is planned.')
          expect((await execute(lead, 'ask', { projectId, projectApproval: 'end-result', title: 'Approve the end result?', recommendation: 'Build the agreed product without deployment.', text: 'The product will provide the agreed page. Deployment is left out.' }))._tag).toBe('Success')
          await page.goto(`${root}#/home`)
          await page.getByRole('button', { name: 'Show Approve the end result?', exact: true }).click()
          await page.getByRole('button', { name: 'Accept recommendation', exact: true }).click()
          await expect(tree).toContainText('Plan')
          expect((await execute(lead, 'project.sprint.create', sprint))._tag).toBe('Success')
          await page.goto(`${root}#/p/${projectId}/settings`)
          const screen = page.locator('.svc-project')
          await expect(screen).toContainText('The branch and the worktree folder keep their names.')
          await page.locator('#ps-name').fill('Agreed product')
          await screen.getByRole('button', { name: 'Save name', exact: true }).click()
          await expect(page.locator('.sidebar .tree-row.project').filter({ hasText: 'Agreed product' })).toBeVisible()
          await page.reload()
          await expect(page.locator('#ps-name')).toHaveValue('Agreed product')
          const snapshot = await execute(ownerCredential, 'client.snapshot', {})
          if (snapshot._tag !== 'Success') throw new Error('Snapshot unavailable')
          const project = (snapshot.success as any).project.projects.find((p: any) => p.projectId === projectId)
          const team = await execute(lead, 'project.team.create', { projectId, name: 'Build team', worktreeId: project.worktreeId })
          expect(team._tag).toBe('Success')
          await expect(screen.locator('.svc-teams tbody')).toContainText('Build team')
          await screen.getByRole('button', { name: 'Delete team Build team', exact: true }).click()
          await expect(page.getByRole('dialog')).toContainText('read-only history')
          await page.getByRole('dialog').getByRole('button', { name: 'Delete team', exact: true }).click()
          await expect(screen.locator('.svc-teams tbody')).toContainText('No teams')
          if (team._tag !== 'Success') throw new Error('Team unavailable')
          await page.goto(`${root}#/p/${projectId}/messages/${team.success.channelId}`)
          await expect(page.getByPlaceholder('Archived channel. Its history is kept.')).toBeDisabled()
          expect(errors).toEqual([])
        })
      }).pipe(Effect.provide(buildServices(rig).pipe(Layer.provide(dependencies))), Effect.provide(runner.layer)))
    })).pipe(Effect.provide(SqliteClient.layer({ filename: rig.database, readerWorker: true }))))
  } finally { if (previous === undefined) delete process.env.LGTM_HOME; else process.env.LGTM_HOME = previous; removeRig(rig) }
}
