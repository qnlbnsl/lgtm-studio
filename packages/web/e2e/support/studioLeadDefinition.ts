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
          await page.goto(`${root}#/projects`)
          await expect(page.locator('.stage-dots[title="Stage: Define"] i.now')).toHaveCount(1)
          await expect(page.locator('.stage-dots[title="Stage: Define"] i:first-child')).toHaveClass('now')
          const sprint = { projectId, label: 'SP-1', name: 'First sprint', goal: 'Deliver the agreed scope', phase: 'Build', owner: spawn.agentId, needs: [] }
          // Both permissions allow a new lead to work without an approval card.
          expect((await execute(lead, 'project.sprint.create', sprint))._tag).toBe('Success')
          await expect(tree).toContainText('Plan')
          await page.goto(`${root}#/p/${projectId}/settings`)
          const screen = page.locator('.svc-project')
          const planning = screen.getByRole('combobox', { name: 'Project planning permission', exact: true })
          const starting = screen.getByRole('combobox', { name: 'Starting work permission', exact: true })
          await expect(screen).toContainText('Lead permissions')
          await expect(screen).toContainText('The lead can create sprints, teams and tasks.')
          await expect(screen).toContainText('The lead can start workers and move tasks out of the queue.')
          for (const control of [planning, starting]) {
            await expect(control).toHaveValue('allowed')
            await expect(control.locator('option')).toHaveText(['Allowed', 'Not allowed', 'Always ask for permission'])
          }
          await planning.selectOption('ask')
          await expect(screen.getByRole('status', { name: 'Project planning grant', exact: true })).toHaveText('Not granted')
          await starting.selectOption('denied')
          await expect(starting).toHaveValue('denied')
          const denied = await execute(lead, 'ask', { projectId, projectApproval: 'build', text: 'Allow work?' })
          expect(denied._tag === 'Failure' && denied.failure.reason).toBe('The owner has turned off Starting work. This permission cannot be requested.')
          const refused = await execute(lead, 'project.sprint.create', { ...sprint, label: 'SP-2' })
          expect(refused._tag === 'Failure' && refused.failure.reason).toBe('Project planning requires permission; use ask with projectApproval set to "end-result" or "skip-planning".')
          expect((await execute(lead, 'ask', { projectId, projectApproval: 'end-result', title: 'Allow Project planning?', recommendation: 'Build the agreed product without deployment.', text: 'The product will provide the agreed page. Deployment is left out.' }))._tag).toBe('Success')
          await page.goto(`${root}#/home`)
          await page.getByRole('button', { name: 'Show Allow Project planning?', exact: true }).click()
          await expect(page.getByText('Accepting grants Project planning. The lead can create sprints, teams and tasks.', { exact: true }).last()).toBeVisible()
          await page.getByRole('textbox', { name: 'Another answer', exact: true }).fill('Yes, accepted. Go ahead.')
          await expect(page.getByText('This answer does not approve. Use Accept to grant approval.', { exact: true })).toBeVisible()
          await page.getByRole('button', { name: 'Accept', exact: true }).click()
          await expect(tree).toContainText('Plan')
          expect((await execute(lead, 'project.sprint.create', { ...sprint, label: 'SP-2' }))._tag).toBe('Success')
          await page.goto(`${root}#/p/${projectId}/settings`)
          await expect(screen).toContainText('The branch and the worktree folder keep their names.')
          await page.locator('#ps-name').fill('Agreed product')
          await screen.getByRole('button', { name: 'Save name', exact: true }).click()
          await expect(page.locator('.sidebar .tree-row.project').filter({ hasText: 'Agreed product' })).toBeVisible()
          await page.reload()
          await expect(page.locator('#ps-name')).toHaveValue('Agreed product')
          await expect(planning).toHaveValue('ask')
          await expect(starting).toHaveValue('denied')
          await expect(screen.getByRole('status', { name: 'Project planning grant', exact: true })).toHaveText('Granted')
          await starting.selectOption('ask')
          await expect(screen.getByRole('status', { name: 'Starting work grant', exact: true })).toHaveText('Not granted')
          expect((await execute(lead, 'ask', { projectId, projectApproval: 'build', title: 'Allow Starting work?', text: 'Start the agreed work.' }))._tag).toBe('Success')
          await page.goto(`${root}#/p/${projectId}/chat/${spawn.agentId}`)
          const granted = page.locator('.req').filter({ hasText: 'Allow Starting work?' }).first()
          await expect(granted).toContainText('Accepting grants Starting work. The lead can start workers and move tasks out of the queue.')
          await granted.getByRole('button', { name: 'Accept', exact: true }).click()
          await expect(granted.getByRole('status')).toHaveText('Approval recorded.')
          await page.reload()
          await expect(granted.getByRole('status')).toHaveText('Approval recorded.')
          await page.goto(`${root}#/p/${projectId}/settings`)
          await screen.getByRole('button', { name: 'Withdraw Project planning', exact: true }).click()
          await expect(screen.getByRole('status', { name: 'Project planning grant', exact: true })).toHaveText('Not granted')
          await expect(screen.getByRole('status', { name: 'Starting work grant', exact: true })).toHaveText('Granted')
          await screen.getByRole('button', { name: 'Withdraw Starting work', exact: true }).click()
          await expect(screen.getByRole('status', { name: 'Starting work grant', exact: true })).toHaveText('Not granted')
          const independent = await execute(lead, 'ask', { projectId, projectApproval: 'build', title: 'Starting work without planning?', text: 'Start the agreed work.' })
          if (independent._tag !== 'Success') throw new Error('Permission request unavailable')
          expect((await execute(ownerCredential, 'answer', { kind: 'question', id: (independent.success as any).questionId, text: 'Accepted' }))._tag).toBe('Success')
          await expect(screen.getByRole('status', { name: 'Starting work grant', exact: true })).toHaveText('Granted')
          const obsolete = await execute(lead, 'ask', { projectId, projectApproval: 'end-result', title: 'Planning request closed in settings?', text: 'Allow planning?' })
          if (obsolete._tag !== 'Success') throw new Error('Permission request unavailable')
          await planning.selectOption('allowed')
          await expect(planning).toHaveValue('allowed')
          await expect(screen.getByRole('status', { name: 'Project planning grant', exact: true })).toHaveCount(0)
          expect((await execute(ownerCredential, 'answer', { kind: 'question', id: (obsolete.success as any).questionId, text: 'Accepted' }))._tag).toBe('Failure')
          await page.goto(`${root}#/p/${projectId}/chat/${spawn.agentId}`)
          const closed = page.locator('.req').filter({ hasText: 'Planning request closed in settings?' }).first()
          await expect(closed).toContainText('Permission request closed because the owner changed its setting.')
          await expect(closed.getByRole('button', { name: 'Accept', exact: true })).toBeDisabled()
          await page.goto(`${root}#/p/${projectId}/settings`)
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
          await expect(page.locator('.mb-stream h2')).toContainText('Build team')
          await expect(page.locator('.ch-row[aria-current="page"]')).toContainText('Archived')
          expect(errors).toEqual([])
        })
      }).pipe(Effect.provide(buildServices(rig).pipe(Layer.provide(dependencies))), Effect.provide(runner.layer)))
    })).pipe(Effect.provide(SqliteClient.layer({ filename: rig.database, readerWorker: true }))))
  } finally { if (previous === undefined) delete process.env.LGTM_HOME; else process.env.LGTM_HOME = previous; removeRig(rig) }
}
