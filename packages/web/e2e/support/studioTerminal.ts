// The project terminal through the engine, including its real content security policy.
import { randomBytes } from 'node:crypto'
import { execFileSync } from 'node:child_process'
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test, type Page } from '@playwright/test'
import * as Effect from 'effect/Effect'
import * as Layer from 'effect/Layer'
import * as Redacted from 'effect/Redacted'
import * as SqliteClient from '../../../../third_party/t3code/node-sqlite/src/nodeSqliteClient.ts'
import { buildServices } from '../../../engine/src/service.ts'
import { serve } from '../../../engine/src/client/web/server.ts'
import { engineLists } from '../../../engine/src/pieces/lists.ts'
import { Views } from '../../../engine/src/views/Views.ts'
import { projectView } from '../../../engine/src/project/view.ts'
import { mintCredential } from '../../../engine/src/identity/credential.ts'
import { makeRedactor } from '../../../engine/src/identity/redactor.ts'
import { unusedPort } from '../../../engine/test/support/browserWeb.ts'
import { makeFakeRunner } from '../../../engine/test/support/fakeRunner.ts'
import { appendSetup, call, eventOf, harnessSettings, makeFakeSecrets, makeRig, ownerCredential, removeRig, seedAccount, success } from '../../../engine/test/support/handlerRig.ts'
import { registerRebuildCheck } from '../../../engine/test/support/rebuildCheck.ts'
import { buildStudio } from '../../../studio/scripts/build.ts'
import { completedSetup } from './completedSetup.ts'
const git = (folder: string, ...args: string[]) => execFileSync('/usr/bin/git', ['-C', folder, ...args], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim()
export async function exerciseStudioTerminal(page: Page) {
  test.setTimeout(120_000)
  const rig = makeRig(), previous = process.env.LGTM_HOME, previousShell = process.env.SHELL
  process.env.LGTM_HOME = rig.stateFolder; process.env.SHELL = '/bin/sh'
  const password = randomBytes(32).toString('base64url'), port = await unusedPort(), errors: string[] = []
  page.on('pageerror', e => { errors.push(e.message); console.error('terminal page error:', e.message) })
  await page.addInitScript(() => {
    ;(window as any).terminalViolations = []
    document.addEventListener('securitypolicyviolation', e => { (window as any).terminalViolations.push(e.violatedDirective + ':' + e.blockedURI) })
  })
  try {
    git(rig.work, 'init', '-b', 'main'); git(rig.work, 'config', 'user.name', 'Owner'); git(rig.work, 'config', 'user.email', 'owner@example.invalid'); git(rig.work, 'config', 'commit.gpgsign', 'false')
    writeFileSync(join(rig.work, 'README.md'), 'Terminal worktree\n'); git(rig.work, 'add', '--all'); git(rig.work, 'commit', '-m', 'Initial')
    const studioRoot = await buildStudio(join(rig.root, 'studio'))
    await Effect.runPromise(Effect.scoped(Effect.gen(function* () {
      const runner = yield* makeFakeRunner()
      const token = mintCredential('shp_a_')
      const redactor = makeRedactor({ secrets: () => [Redacted.make('sk-registered-browser-terminal-secret')], tokenHashes: () => new Set([token.tokenHash]) })
      const dependencies = Layer.mergeAll(runner.layer, makeFakeSecrets().layer, redactor.layer)
      yield* Effect.scoped(Effect.gen(function* () {
        yield* registerRebuildCheck(); yield* seedAccount('launch', 'cursor')
        success(yield* call(ownerCredential, 'harnesses', { operation: 'set', harness: harnessSettings('cursor', { adapterPath: process.execPath, reportedModes: [{ id: 'default', level: 'ask' }] }) }))
        success(yield* call(ownerCredential, 'accounts set-secret', { accountId: 'launch', name: 'CURSOR_API_KEY', nonce: 'terminal', value: Redacted.make('synthetic-terminal-key') }))
        success(yield* call(ownerCredential, 'project.settings.set', { settings: { workspaceFolder: rig.root, shell: 'sh' } }))
        const repoId = success(yield* call(ownerCredential, 'project.repository.add', { name: 'Terminal repository', path: rig.work, machineId: 'local' })).repoId
        success(yield* call(ownerCredential, 'project.workspace.register', { repoId, path: rig.work, branch: 'main', machineId: 'local', checkout: 'main' }))
        const created = success(yield* call(ownerCredential, 'project.create', { name: 'Terminal project', goal: 'Connect terminal', machineId: 'local', repository: { _tag: 'existing', repoId }, useWorktree: true }))
        const project = projectView(yield* (yield* Views).snapshot).projects[created.projectId]!
        const projectViewPathValue = projectView(yield* (yield* Views).snapshot).workspaces[project.worktreeId]!.path
        yield* completedSetup(project.projectId)
        yield* appendSetup([eventOf({ _tag: 'project.WorktreeCreated', repoId, worktreeId: 'remote-terminal', branch: 'remote-terminal', path: '/unavailable/terminal', machineId: 'remote-terminal', checkout: 'main' })])
        yield* serve({ password: Redacted.make(password), staticRoot: studioRoot, lists: engineLists, settings: { enabled: true, port, heartbeatSeconds: 25, proxyMode: false } })
        yield* Effect.promise(async () => {
          const base = `http://127.0.0.1:${port}/`, url = base + `#/p/${project.projectId}/terminal`
          await page.goto(url); await page.locator('#login-password').fill(password); await page.getByRole('button', { name: 'Sign in', exact: true }).click()
          await expect(page.locator('.tree-row.project')).toBeVisible(); await expect(page).toHaveURL(/#\/home/); await page.getByRole('link', { name: project.name, exact: true }).click(); await page.getByRole('tab', { name: 'Terminal', exact: true }).click()
          await expect(page.locator('.term')).toContainText('The project terminal is turned off.')
          await page.getByRole('link', { name: 'Open Service settings', exact: true }).click()
          const toggle = page.getByRole('switch', { name: 'Project terminal', exact: true })
          await expect(toggle).toHaveAttribute('aria-checked', 'false'); await toggle.click(); await expect(toggle).toHaveAttribute('aria-checked', 'true')
          await page.goto(url)
          const folder = page.getByRole('combobox', { name: 'Open in', exact: true })
          await expect(folder).toHaveValue(project.worktreeId)
          await expect(folder.locator('option')).toHaveCount(3)
          const remote = folder.locator('option[value="remote-terminal"]')
          await expect(remote).toHaveJSProperty('disabled', true)
          await expect(remote).toContainText('Engine work needed: local machine only')
          await page.getByRole('button', { name: 'New session', exact: true }).click()
          await expect(page.getByRole('dialog')).toHaveCount(0)
          const input = page.locator('.xterm-helper-textarea')
          await expect(input).toBeAttached(); await expect(page.locator('.term-cwd')).toHaveText(projectViewPathValue); await input.focus()
          const prompt = page.locator('.xterm-rows > div').filter({ has: page.locator('.xterm-cursor') })
          await expect(prompt).toHaveText(/\S/)
          const prefix = (await prompt.textContent())!.trimEnd().replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
          for (const typed of ['ls', 'chmod 777']) {
            await page.keyboard.type(typed)
            await expect(prompt).toHaveText(new RegExp('^' + prefix + '\\s*' + typed + '\\s*$'))
            await page.keyboard.press('Control+U')
            await expect(prompt).toHaveText(new RegExp('^' + prefix + '\\s*$'))
          }
          await page.keyboard.type("printf '\\033[31mBROWSER_TERMINAL_OK\\033[0m sk-registered-browser-terminal-secret\\n'")
          await page.keyboard.press('Enter')
          // DOM renderer text proves output was drawn, not merely received.
          await expect(page.locator('.xterm-rows')).toContainText('BROWSER_TERMINAL_OK')
          await expect(page.locator('.xterm-rows')).toContainText('sk-registered-browser-terminal-secret')
          await expect(page.locator('.xterm-rows')).not.toContainText('[REDACTED]')
          await page.keyboard.type("printf '\\033[38;2;255;100;0mTRUE_FORE\\033[0m \\033[48;2;12;34;56mTRUE_BACK\\033[0m\\n'"); await page.keyboard.press('Enter')
          await expect(page.locator('.xterm-rows span').filter({ hasText: /^TRUE_FORE$/ }).first()).toHaveCSS('color', 'rgb(255, 100, 0)')
          await expect(page.locator('.xterm-rows span').filter({ hasText: /^TRUE_BACK$/ }).first()).toHaveCSS('background-color', 'rgb(12, 34, 56)')
          // The shell variable and PID prove this is the same process after reload.
          await page.keyboard.type("S28_SHELL=$$; echo S28_BEFORE_RELOAD:$S28_SHELL"); await page.keyboard.press('Enter')
          await expect(page.locator('.xterm-rows')).toContainText(/S28_BEFORE_RELOAD:\d+/)
          const prior = (await page.locator('.xterm-rows').innerText()).match(/S28_BEFORE_RELOAD:(\d+)/)![1]
          await page.reload()
          await expect(page.locator('.term-tab')).toHaveCount(1)
          await expect(page.locator('.xterm-rows')).toContainText('BROWSER_TERMINAL_OK')
          await input.focus(); await page.keyboard.type('echo S28_AFTER_RELOAD:$S28_SHELL:$$'); await page.keyboard.press('Enter')
          await expect(page.locator('.xterm-rows')).toContainText('S28_AFTER_RELOAD:' + prior + ':' + prior)
          const before = await page.locator('.term-status').innerText()
          await page.setViewportSize({ width: 1100, height: 740 })
          await expect(page.locator('.term-status')).not.toHaveText(before)
          await expect(page.locator('.xterm-rows span').filter({ hasText: /^BROWSER_TERMINAL_OK$/ }).first()).toHaveCSS('color', 'rgb(204, 0, 0)')
          expect(await page.evaluate(() => (window as any).terminalViolations)).toEqual([])
          const nonce = await page.evaluate(() => { const n = document.querySelector('meta[name="lgtm-style-nonce"]')!.getAttribute('content'); return [...document.querySelectorAll('.xterm style')].every(s => (s as HTMLStyleElement).nonce === n) })
          expect(nonce).toBe(true)
          await page.getByRole('button', { name: /^Close / }).last().click()
          await expect(page.locator('.term')).toContainText('No sessions are open')
          await folder.selectOption({ label: (await folder.locator('option').filter({ hasText: /^main ·/ }).textContent())! })
          const selectedFolder = await folder.inputValue()
          await page.getByRole('button', { name: 'New session', exact: true }).click()
          await expect(page.locator('.term-cwd')).toHaveText(rig.work)
          await page.locator('.xterm-helper-textarea').focus(); await page.keyboard.type('pwd'); await page.keyboard.press('Enter')
          await expect(page.locator('.xterm-rows')).toContainText(rig.work)
          // A refusal must not trap a tab in this page. Cancelling its reader
          // still releases the engine session through the normal stream cleanup.
          await page.evaluate(async () => {
            const engine = await import('/js/engine.js' as string)
            const connection = engine.connection(), request = connection.request.bind(connection)
            connection.request = (tag: string, payload: unknown) => tag === 'client.terminal.close'
              ? Promise.reject(new Error('Synthetic close refusal')) : request(tag, payload)
          })
          await page.getByRole('button', { name: /^Close / }).last().click()
          await expect(page.locator('.term')).toContainText('No sessions are open')
          await expect(folder).toHaveValue(selectedFolder)
          await page.getByRole('tab', { name: 'Chat', exact: true }).click()
          await page.getByRole('tab', { name: 'Terminal', exact: true }).click()
          await expect(folder).toHaveValue(selectedFolder)
          await expect(folder.locator('option[value="remote-terminal"]')).toHaveJSProperty('disabled', true)
          await expect(folder.locator('option[value="remote-terminal"]')).toContainText('Engine work needed')
          // Simulate a remembered choice becoming unavailable. Native selection
          // itself cannot choose a disabled option with a mouse or keyboard.
          await folder.evaluate(element => {
            const select = element as HTMLSelectElement
            select.value = 'remote-terminal'; select.dispatchEvent(new Event('change', { bubbles: true }))
          })
          await expect(folder).toHaveValue('remote-terminal')
          await expect(page.getByRole('button', { name: 'New session', exact: true })).toBeDisabled()
          await folder.selectOption(selectedFolder)
          await expect(page.getByRole('button', { name: 'New session', exact: true })).toBeEnabled()
          await page.getByRole('button', { name: 'New session', exact: true }).click()
          await expect(page.locator('.term-cwd')).toHaveText(rig.work)
          await page.getByRole('button', { name: /^Close / }).last().click()
          await page.goto(base + '#/settings?section=remote'); await expect(toggle).toHaveAttribute('aria-checked', 'true'); await toggle.click(); await expect(toggle).toHaveAttribute('aria-checked', 'false')
          await page.goto(url); await expect(page.locator('.term')).toContainText('The project terminal is turned off.')
          expect(await page.evaluate(() => (window as any).terminalViolations)).toEqual([])
        })
      })).pipe(Effect.provide(buildServices(rig, engineLists).pipe(Layer.provide(dependencies))), Effect.provide(runner.layer))
    })).pipe(Effect.provide(SqliteClient.layer({ filename: rig.database, readerWorker: true })), Effect.scoped) as Effect.Effect<void, unknown, never>)
    expect(errors).toEqual([])
  } finally { if (previous === undefined) delete process.env.LGTM_HOME; else process.env.LGTM_HOME = previous; if (previousShell === undefined) delete process.env.SHELL; else process.env.SHELL = previousShell; removeRig(rig) }
}
