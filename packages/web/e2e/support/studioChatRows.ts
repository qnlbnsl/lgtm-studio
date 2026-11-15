import { randomBytes } from 'node:crypto'
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
import { unusedPort } from '../../../engine/test/support/browserWeb.ts'
import { makeFakeRunner } from '../../../engine/test/support/fakeRunner.ts'
import { appendSetup, eventOf, makeFakeSecrets, makeRig, removeRig } from '../../../engine/test/support/handlerRig.ts'
import { registerRebuildCheck } from '../../../engine/test/support/rebuildCheck.ts'
import { decisionSetup } from '../../../engine/test/support/projectDecisionFixtures.ts'
import { translate } from '../../../engine/src/runner/translate.ts'
import { buildStudio } from '../../../studio/scripts/build.ts'
import { completedSetup } from './completedSetup.ts'

export async function exerciseStudioChatRows(page: Page) {
  const rig = makeRig(), previous = process.env.LGTM_HOME
  process.env.LGTM_HOME = rig.stateFolder
  const password = randomBytes(32).toString('base64url'), port = await unusedPort()
  const errors: string[] = []
  page.on('pageerror', (error) => errors.push(error.message))
  try {
    const studioRoot = await buildStudio(join(rig.root, 'studio'))
    await Effect.runPromise(Effect.scoped(Effect.gen(function* () {
      const runner = yield* makeFakeRunner()
      const dependencies = Layer.mergeAll(runner.layer, makeFakeSecrets().layer, Layer.succeed(Redactor, { redact: (event) => Effect.succeed(event), redactText: (text) => Effect.succeed(text) }))
      yield* Effect.scoped(Effect.gen(function* () {
        yield* registerRebuildCheck()
        const project = yield* decisionSetup(); yield* completedSetup(project.projectId)
        const diff = '@@ -1 +1 @@\n-old rule\n+new rule'
        const secondHunk = '@@ -8 +8 @@\n-second old rule\n+second new rule'
        const complete = translate({ sessionUpdate: 'tool_call_update', toolCallId: 'edit', status: 'completed', content: [diff, secondHunk].map((text) => ({ type: 'diff', changes: [{ path: 'notes/MEMORY.md' }], patch: { text, format: 'git_patch' } })) }, false)
        const output = Array.from({ length: 40 }, (_, n) => `check ${n}: passed`).join('\n')
        const question = 'Which rule should we keep?\n- The new rule\n- The old rule'
        const item = (content: any, itemId = 'message') => eventOf({ _tag: 'ThreadItem', agentId: project.leadId, itemId, replay: false, content })
        yield* appendSetup([
          item({ _tag: 'text', text: 'Earlier reply.\n- First item\n- Second `item`\n\n```ts\nconst safe = "<b>literal</b>";\n```' }),
          item({ _tag: 'tool-call', callId: 'edit', title: 'Edit', status: 'in_progress', input: '{"file_path":"notes/MEMORY.md","old_string":"old rule","new_string":"new rule"}' }),
          item({ _tag: 'file-change', callId: 'edit', path: 'notes/MEMORY.md', diff }),
          item({ _tag: 'file-change', callId: 'edit', path: 'notes/MEMORY.md', diff }),
          ...complete.map((event) => item(event.content)),
          item({ _tag: 'tool-call', callId: 'command', title: 'Bash', input: '{"command":"pnpm typecheck"}', output: JSON.stringify([{ type: 'content', content: { type: 'text', text: output } }]), status: 'completed' }),
          item({ _tag: 'text', text: question }, 'question-echo'),
          eventOf({ _tag: 'QuestionAsked', agentId: project.leadId, questionId: 'chat-row-question', holder: { _tag: 'owner' }, deliveryId: 'chat-question', title: 'Choose a rule', text: question }),
          item({ _tag: 'text', text: 'Later reply.' }),
        ])
        yield* serve({ password: Redacted.make(password), staticRoot: studioRoot, lists: engineLists, settings: { enabled: true, port, heartbeatSeconds: 25, proxyMode: false } })
        yield* Effect.promise(async () => {
          const root = `http://127.0.0.1:${port}/`
          await page.goto(root)
          await page.locator('#login-password').fill(password)
          await page.getByRole('button', { name: 'Sign in', exact: true }).click()
          await expect(page.locator('#login-password')).toHaveCount(0)
          await page.goto(`${root}#/p/${project.projectId}/chat/${project.leadId}`)
          const thread = page.locator('.chat-main .thread')
          const edit = thread.locator('details.tools').filter({ has: page.locator('summary', { hasText: 'Edited MEMORY.md' }) })
          const command = thread.locator('details.tools').filter({ has: page.locator('summary', { hasText: 'Ran pnpm typecheck' }) })
          await expect(edit).toHaveCount(1)
          await expect(edit).not.toHaveAttribute('open')
          await expect(command).not.toHaveAttribute('open')
          await expect(thread.locator('.body li')).toHaveText(['First item', 'Second item'])
          await expect(thread.locator('.body pre code')).toHaveText('const safe = "<b>literal</b>";')
          await expect(thread.locator('.body b')).toHaveCount(0)
          await edit.locator('summary').click()
          await expect(edit.locator('pre')).toHaveCount(1)
          expect(await edit.locator('pre').textContent()).toBe(diff + '\n' + secondHunk)
          await expect(edit.locator('.del')).toHaveText(['-old rule', '-second old rule'])
          await expect(edit.locator('.add')).toHaveText(['+new rule', '+second new rule'])
          await expect(edit.locator('.tool-line')).toContainText('completed')
          await expect(edit.getByRole('button', { name: 'Copy diff', exact: true })).toBeVisible()
          expect((await edit.textContent())!.split('notes/MEMORY.md')).toHaveLength(2)
          await command.locator('summary').click()
          await expect(command.getByRole('button', { name: 'Show more', exact: true })).toBeVisible()
          await command.getByRole('button', { name: 'Show more', exact: true }).click()
          expect(await command.locator('pre').nth(1).textContent()).toBe(output)
          const card = thread.locator('#req-chat-row-question')
          await expect(card).toContainText('Which rule should we keep?')
          await expect(card.locator('li')).toHaveText(['The new rule', 'The old rule'])
          await card.getByRole('button', { name: 'Details', exact: true }).click()
          expect((await thread.textContent())!.split('Which rule should we keep?')).toHaveLength(2)
          expect(await thread.locator(':scope > .msg').evaluateAll((rows) => rows.map((row) => row.querySelector('.body')?.textContent || row.querySelector('summary')?.textContent || row.querySelector('.req')?.getAttribute('aria-label')))).toEqual([
            expect.stringContaining('Earlier reply.'), 'Edited MEMORY.md', 'Ran pnpm typecheck', 'Question: Choose a rule', 'Later reply.',
          ])
          // Reconnect and history hydration keep the same rows and deduplication.
          await page.reload()
          await expect(thread.locator('summary', { hasText: 'Edited MEMORY.md' })).toHaveCount(1)
          await expect(thread.locator('#req-chat-row-question')).toContainText('Which rule should we keep?')
          expect((await thread.textContent())!.split('Which rule should we keep?')).toHaveLength(2)
        })
      })).pipe(Effect.provide(buildServices(rig, engineLists).pipe(Layer.provide(dependencies))), Effect.provide(runner.layer))
    })).pipe(Effect.provide(SqliteClient.layer({ filename: rig.database, readerWorker: true })), Effect.scoped) as Effect.Effect<void, unknown, never>)
    expect(errors).toEqual([])
  } finally {
    if (previous === undefined) delete process.env.LGTM_HOME; else process.env.LGTM_HOME = previous
    removeRig(rig)
  }
}
