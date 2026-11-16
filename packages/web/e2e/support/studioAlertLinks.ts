import { randomBytes } from 'node:crypto'
import { join } from 'node:path'
import { expect, type Page } from '@playwright/test'
import { alertKindNames, alertTitles } from '@lgtm/contracts'
import * as Effect from 'effect/Effect'
import * as Layer from 'effect/Layer'
import * as Redacted from 'effect/Redacted'
import * as SqliteClient from '../../../../third_party/t3code/node-sqlite/src/nodeSqliteClient.ts'
import { buildServices } from '../../../engine/src/service.ts'
import { serve } from '../../../engine/src/client/web/server.ts'
import { engineLists } from '../../../engine/src/pieces/lists.ts'
import { Redactor } from '../../../engine/src/log/Redactor.ts'
import { alertPayload } from '../../../engine/src/client/alerts/send.ts'
import { clientView } from '../../../engine/src/client/alerts/view.ts'
import { Views } from '../../../engine/src/views/Views.ts'
import { unusedPort } from '../../../engine/test/support/browserWeb.ts'
import { makeFakeRunner } from '../../../engine/test/support/fakeRunner.ts'
import { appendSetup, call, eventOf, makeFakeSecrets, makeRig, ownerCredential, removeRig, success } from '../../../engine/test/support/handlerRig.ts'
import { registerRebuildCheck } from '../../../engine/test/support/rebuildCheck.ts'
import { decisionProject, decisionSetup } from '../../../engine/test/support/projectDecisionFixtures.ts'
import { buildStudio } from '../../../studio/scripts/build.ts'
import { completedSetup } from './completedSetup.ts'

export async function exerciseStudioAlertLinks(page: Page) {
  const rig = makeRig(), previous = process.env.LGTM_HOME
  process.env.LGTM_HOME = rig.stateFolder
  const password = randomBytes(32).toString('base64url'), port = await unusedPort()
  try {
    const staticRoot = await buildStudio(join(rig.root, 'studio'))
    await Effect.runPromise(Effect.scoped(Effect.gen(function* () {
      const runner = yield* makeFakeRunner()
      const dependencies = Layer.mergeAll(runner.layer, makeFakeSecrets().layer, Layer.succeed(Redactor, {
        redact: (event) => Effect.succeed(event), redactText: (text) => Effect.succeed(text),
      }))
      yield* Effect.scoped(Effect.gen(function* () {
        yield* registerRebuildCheck()
        const project = yield* decisionSetup(); yield* completedSetup(project.projectId)
        const channelId = success(yield* call(ownerCredential, 'board.open', { name: 'alert-subjects' })).channelId
        success(yield* call(ownerCredential, 'board.members', { channelId, add: [project.leadId], remove: [] }))
        const postId = success(yield* call(project.leadCredential, 'board.post', { channelId, text: 'Read and answer this post.', to: [{ _tag: 'owner' }], reply: 'any' })).postId
        const replyId = success(yield* call(project.leadCredential, 'board.post', { channelId, text: 'First reply subject.', to: [{ _tag: 'owner' }], reply: 'any', replyTo: postId })).postId
        const nestedId = success(yield* call(project.leadCredential, 'board.post', { channelId, text: 'Nested reply subject.', to: [{ _tag: 'owner' }], reply: 'any', replyTo: replyId })).postId
        const foreign = yield* decisionProject('foreign-alert')
        const foreignPostId = success(yield* call(foreign.leadCredential, 'board.post', { channelId: foreign.team.channelId, text: 'Subject in another project.', to: [{ _tag: 'owner' }], reply: 'any' })).postId
        const questionId = success(yield* call(project.leadCredential, 'ask', { title: 'Read this question', text: 'Choose the next task.', proposedAnswer: 'Continue' })).questionId
        yield* appendSetup([
          eventOf({ _tag: 'client.WebSettingsChanged', settings: { enabled: true, port, heartbeatSeconds: 25, proxyMode: true, publicAddress: 'https://alerts.example' } }),
          eventOf({ _tag: 'PermissionRequested', agentId: project.leadId, permissionRequestId: 'link-permission', holder: { _tag: 'owner' }, deliveryId: 'link-permission-delivery', toolCallId: 'link-tool', description: 'Read this permission', options: [{ id: 'link-once', name: 'Allow once', kind: 'allow_once', level: 'ask', persistent: false }], settingsWrite: false }),
          ...alertKindNames.map((kind) => eventOf({ _tag: 'client.AlertRaised', alertId: `link-${kind}`, alertKey: `link:${kind}`, kind,
            agentId: project.leadId, title: alertTitles[kind], sourceSequence: 1,
            subject: kind.startsWith('board.') ? { _tag: 'piece', namespace: 'board', id: postId }
              : kind === 'approval' ? { _tag: 'permission', id: 'link-permission' }
              : kind === 'question' ? { _tag: 'question', id: questionId } : { _tag: 'agent', id: project.leadId },
          })),
        ])
        yield* appendSetup(['nested', 'foreign'].map((key) => eventOf({ _tag: 'client.AlertRaised', alertId: `link-${key}`, alertKey: `link:${key}`, kind: 'board.post', agentId: project.leadId, title: alertTitles['board.post'], sourceSequence: 1, subject: { _tag: 'piece', namespace: 'board', id: key === 'nested' ? nestedId : foreignPostId } })))
        const snapshot = yield* (yield* Views).snapshot
        const payloads = [...alertKindNames, 'nested', 'foreign'].map((kind) => alertPayload(clientView(snapshot.client).alerts[`link-${kind}`]!, snapshot))
        expect(payloads.every((p) => p.link !== undefined)).toBe(true)
        yield* serve({ password: Redacted.make(password), staticRoot, lists: engineLists, settings: { enabled: true, port, heartbeatSeconds: 25, proxyMode: false } })
        yield* Effect.promise(async () => {
          const origin = `http://127.0.0.1:${port}`
          // Follow the emitted bookmark through login, then check every kind.
          const question = payloads.find((p) => p.kind === 'question')!
          await page.goto(origin + new URL(question.link!).hash)
          await page.locator('#login-password').fill(password)
          await page.getByRole('button', { name: 'Sign in', exact: true }).click()
          await expect(page.locator(`#req-${questionId}`)).toContainText('Choose the next task.')
          for (const payload of payloads) {
            const url = new URL(payload.link!)
            expect(url.origin).toBe('https://alerts.example')
            expect(url.username + url.password + url.search).toBe('')
            expect(payload.link).not.toContain('Choose the next task.')
            await page.goto(origin + url.hash)
            if (payload.kind.startsWith('board.')) {
              const subject = payload.alertId === 'link-nested' ? 'Nested reply subject.' : payload.alertId === 'link-foreign' ? 'Subject in another project.' : 'Read and answer this post.'
              await expect(page.getByRole('complementary', { name: 'Thread', exact: true })).toContainText(subject)
              if (payload.alertId === 'link-foreign') expect(url.hash).toContain(`/p/${foreign.projectId}/messages/`)
              if (payload.alertId === 'link-nested') expect(url.hash).toContain(`?t=${replyId}`)
            } else {
              await expect(page.locator('.chat-main h1')).toHaveText('Project lead')
              if (payload.kind === 'approval') await expect(page.locator('#req-link-permission').getByRole('button', { name: 'Allow', exact: true })).toBeVisible()
              if (payload.kind === 'question') await expect(page.locator(`#req-${questionId}`).getByRole('button', { name: 'Accept', exact: true })).toBeVisible()
            }
          }
        })
      })).pipe(Effect.provide(buildServices(rig, engineLists).pipe(Layer.provide(dependencies))), Effect.provide(runner.layer))
    })).pipe(Effect.provide(SqliteClient.layer({ filename: rig.database, readerWorker: true })), Effect.scoped) as Effect.Effect<void, unknown, never>)
  } finally {
    if (previous === undefined) delete process.env.LGTM_HOME; else process.env.LGTM_HOME = previous
    removeRig(rig)
  }
}
