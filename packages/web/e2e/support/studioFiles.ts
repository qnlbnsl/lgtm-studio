// Files through the engine, including the policy that controls CodeMirror styles.
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
import { Redactor } from '../../../engine/src/log/Redactor.ts'
import { unusedPort } from '../../../engine/test/support/browserWeb.ts'
import { makeFakeRunner } from '../../../engine/test/support/fakeRunner.ts'
import { appendSetup, call, eventOf, harnessSettings, makeFakeSecrets, makeRig, ownerCredential, removeRig, seedAccount, success } from '../../../engine/test/support/handlerRig.ts'
import { registerRebuildCheck } from '../../../engine/test/support/rebuildCheck.ts'
import { buildStudio } from '../../../studio/scripts/build.ts'
import { completedSetup } from './completedSetup.ts'
const git = (folder: string, ...args: string[]) => execFileSync('/usr/bin/git', ['-C', folder, ...args], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim()
export async function exerciseStudioFiles(page: Page) {
  test.setTimeout(120_000)
  const rig = makeRig(), previous = process.env.LGTM_HOME
  process.env.LGTM_HOME = rig.stateFolder
  const password = randomBytes(32).toString('base64url'), port = await unusedPort()
  const errors: string[] = []
  page.on('pageerror', (error) => errors.push(error.message))
  await page.addInitScript(() => {
    (window as any).fileStyleViolations = []
    document.addEventListener('securitypolicyviolation', (e) => { if (e.violatedDirective.startsWith('style-src-elem')) (window as any).fileStyleViolations.push(e.blockedURI) })
  })
  try {
    git(rig.work, 'init', '-b', 'main'); git(rig.work, 'config', 'user.name', 'Owner'); git(rig.work, 'config', 'user.email', 'owner@example.invalid'); git(rig.work, 'config', 'commit.gpgsign', 'false')
    mkdirSync(join(rig.work, 'src')); mkdirSync(join(rig.work, 'untouched'))
    writeFileSync(join(rig.work, 'README.md'), 'The file text must sit beside the line numbers.\nSecond line.\n')
    writeFileSync(join(rig.work, 'src/file.ts'), 'export const count = 1;\n')
    writeFileSync(join(rig.work, 'untouched/deep.txt'), 'Lazy folder.\n')
    writeFileSync(join(rig.work, 'binary.bin'), Buffer.from([0, 1, 2, 3]))
    writeFileSync(join(rig.work, 'large.txt'), 'x'.repeat(512 * 1024 + 1))
    writeFileSync(join(rig.work, 'image.svg'), '<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20"><rect width="20" height="20" fill="red"/></svg>')
    writeFileSync(join(rig.work, 'private.txt'), 'hidden-file-text\n')
    git(rig.work, 'add', '--all'); git(rig.work, 'commit', '-m', 'Initial')
    const studioRoot = await buildStudio(join(rig.root, 'studio')), staticRoot = join(rig.root, 'web')
    mkdirSync(staticRoot); writeFileSync(join(staticRoot, 'index.html'), '<title>Test</title>')
    await Effect.runPromise(Effect.scoped(Effect.gen(function* () {
      const runner = yield* makeFakeRunner()
      const dependencies = Layer.mergeAll(runner.layer, makeFakeSecrets().layer, Layer.succeed(Redactor, {
        redact: (event) => Effect.succeed(event), redactText: (value) => Effect.succeed(value.replaceAll('hidden-file-text', '[redacted]'))
      }))
      yield* Effect.scoped(Effect.gen(function* () {
        yield* registerRebuildCheck(); yield* seedAccount('launch', 'cursor')
        success(yield* call(ownerCredential, 'harnesses', { operation: 'set', harness: harnessSettings('cursor', { adapterPath: process.execPath, reportedModes: [{ id: 'default', level: 'ask' }] }) }))
        success(yield* call(ownerCredential, 'accounts set-secret', { accountId: 'launch', name: 'CURSOR_API_KEY', nonce: 'studio-files', value: Redacted.make('synthetic-files-key') }))
        success(yield* call(ownerCredential, 'project.settings.set', { settings: { workspaceFolder: rig.root, shell: 'sh' } }))
        const repoId = success(yield* call(ownerCredential, 'project.repository.add', { name: 'Files repository', path: rig.work, machineId: 'local' })).repoId
        success(yield* call(ownerCredential, 'project.workspace.register', { repoId, path: rig.work, branch: 'main', machineId: 'local', checkout: 'main' }))
        const created = success(yield* call(ownerCredential, 'project.create', { name: 'Files project', goal: 'Connect files', machineId: 'local', repository: { _tag: 'existing', repoId }, useWorktree: true }))
        const view = projectView(yield* (yield* Views).snapshot), project = view.projects[created.projectId]!, workspace = view.workspaces[project.worktreeId]!
        yield* completedSetup(project.projectId)
        writeFileSync(join(workspace.path, 'src/file.ts'), 'export const count = 2;\n')
        writeFileSync(join(workspace.path, 'other.txt'), 'Staged separately.\n'); git(workspace.path, 'add', 'other.txt')
        // A registered remote worktree is visible, but its files must never be requested.
        yield* appendSetup([eventOf({ _tag: 'project.WorktreeCreated', repoId, worktreeId: 'remote-files', branch: 'remote-files', path: '/unavailable/files', machineId: 'remote-files', checkout: 'worktree', parentId: workspace.worktreeId })])
        yield* serve({ password: Redacted.make(password), staticRoot, studioRoot, lists: engineLists, settings: { enabled: true, port, heartbeatSeconds: 25, proxyMode: false } })
        yield* Effect.promise(async () => {
          const url = `http://127.0.0.1:${port}/studio/#/p/${project.projectId}/files`
          await page.goto(url); await page.locator('#login-password').fill(password); await page.getByRole('button', { name: 'Sign in', exact: true }).click()
          await expect(page.locator('.tree-row.project')).toBeVisible(); await page.goto(url)
          const editor = page.getByRole('textbox', { name: 'Contents of README.md', exact: true })
          await expect(editor).toContainText('The file text must sit beside the line numbers.')
          await expect(page.locator('.cm-lineNumbers')).toContainText('1')
          // This catches the line-numbers-only failure: text and gutter share a row,
          // and the dynamically added CodeMirror style has the engine's page nonce.
          const geometry = await page.evaluate(() => {
            const line = document.querySelector('.cm-line')!.getBoundingClientRect()
            const gutter = document.querySelector('.cm-lineNumbers')!.getBoundingClientRect()
            const nonce = document.querySelector('meta[name="lgtm-style-nonce"]')!.getAttribute('content')
            return { left: line.left, right: gutter.right, top: line.top, gutterTop: gutter.top, styles: [...document.querySelectorAll('style')].map((s) => s.nonce === nonce), violations: (window as any).fileStyleViolations }
          })
          expect(geometry.left).toBeGreaterThanOrEqual(geometry.right - 1)
          expect(Math.abs(geometry.top - geometry.gutterTop)).toBeLessThan(20)
          expect(geometry.styles.length).toBeGreaterThan(0); expect(geometry.styles.every(Boolean)).toBe(true); expect(geometry.violations).toEqual([])
          await page.getByRole('button', { name: 'More', exact: true }).click()
          await expect(page.getByRole('menuitem', { name: /Download/ })).toBeDisabled()
          await expect(page.getByRole('menuitem', { name: /Download/ })).toHaveAttribute('title', 'Engine work needed')
          await page.keyboard.press('Escape')
          await expect(page.getByRole('treeitem', { name: 'src', exact: true }).locator('.fx-dot')).toHaveCount(1)
          await expect(page.getByRole('treeitem', { name: 'untouched', exact: true })).toHaveAttribute('aria-expanded', 'false')
          await page.getByRole('button', { name: 'src', exact: true }).click()
          await expect(page.getByRole('treeitem', { name: 'file.ts', exact: true }).locator('.fx-st')).toHaveText('M')
          await page.getByRole('textbox', { name: 'Go to file', exact: true }).fill('file.ts')
          await page.getByRole('button', { name: 'src/file.ts', exact: true }).click()
          const contents = page.getByRole('textbox', { name: 'Contents of src/file.ts', exact: true })
          await expect(contents).toContainText('count = 2')
          const type = async (value: string) => { await contents.fill(value); await expect(page.getByRole('button', { name: 'Save', exact: true })).toBeEnabled() }
          await type('export const count = 3;'); await contents.press('Control+s')
          await expect(page.getByRole('button', { name: 'Save', exact: true })).toBeDisabled(); expect(readFileSync(join(workspace.path, 'src/file.ts'), 'utf8')).toBe('export const count = 3;')
          await type('export const count = 4;'); writeFileSync(join(workspace.path, 'src/file.ts'), 'Changed outside the page.\n'.repeat(120))
          await page.getByRole('button', { name: 'Save', exact: true }).click()
          const conflict = page.getByRole('region', { name: 'The file as it is now' })
          await expect(conflict).toContainText('Changed outside the page.'); await expect(contents).toContainText('count = 4')
          expect((await page.locator('.fx-editor').boundingBox())!.height).toBeGreaterThan(100)
          expect((await conflict.boundingBox())!.height).toBeLessThan((await page.locator('.fx-main').boundingBox())!.height * 0.5)
          await page.getByRole('button', { name: 'Use the current file', exact: true }).click(); await expect(contents).toContainText('Changed outside the page.')
          await type('export const count = 5;'); writeFileSync(join(workspace.path, 'src/file.ts'), 'Another outside change.\n')
          await page.getByRole('button', { name: 'Save', exact: true }).click(); await expect(conflict).toContainText('Another outside change.')
          await page.getByRole('button', { name: 'Save my text over it', exact: true }).click(); await expect(conflict).toHaveCount(0)
          expect(readFileSync(join(workspace.path, 'src/file.ts'), 'utf8')).toBe('export const count = 5;')
          await page.getByRole('button', { name: 'Commit', exact: true }).click()
          await expect(page.getByRole('dialog').getByRole('button', { name: 'Generate', exact: true })).toBeDisabled()
          await expect(page.getByRole('dialog').getByRole('button', { name: 'Generate', exact: true })).toHaveAttribute('title', 'Engine work needed')
          await page.getByRole('textbox', { name: 'Commit message', exact: true }).fill('Commit the open file')
          await page.getByRole('dialog').getByRole('button', { name: 'Commit', exact: true }).click(); await expect(page.getByRole('dialog')).toHaveCount(0)
          expect(git(workspace.path, 'show', '--format=', '--name-only', 'HEAD')).toBe('src/file.ts'); expect(git(workspace.path, 'diff', '--cached', '--name-only')).toBe('other.txt')
          const open = async (path: string) => { await page.goto(url + '?f=' + encodeURIComponent(path)) }
          await open('image.svg'); await expect(page.locator('.fx-pic img')).toBeVisible(); expect(await page.locator('.fx-pic img').evaluate((img: HTMLImageElement) => img.complete && img.naturalWidth > 0)).toBe(true)
          for (const [path, notice] of [['binary.bin', 'Binary file'], ['large.txt', 'Too large to show']]) {
            await open(path!); await expect(page.locator('.fx-state')).toContainText(notice!); await expect(page.getByRole('button', { name: 'Download', exact: true })).toBeDisabled(); await expect(page.getByRole('button', { name: 'Download', exact: true })).toHaveAttribute('title', 'Engine work needed')
          }
          await open('private.txt'); await expect(page.locator('.fx-note')).toContainText('private data'); await expect(page.getByRole('textbox', { name: 'Contents of private.txt', exact: true })).toHaveAttribute('contenteditable', 'false')
          await expect(page.locator('.fx-main')).not.toContainText('hidden-file-text')
          // The worktree selector is shared by the project's tabs.
          await page.getByRole('button', { name: 'Inspecting: the worktree whose code you look at', exact: true }).click(); await page.getByRole('menuitemradio', { name: /^main/ }).click()
          await open('README.md'); await expect(page.locator('.fx-note')).toContainText('base branch is read-only')
          await page.getByRole('button', { name: 'Inspecting: the worktree whose code you look at', exact: true }).click(); await page.getByRole('menuitemradio', { name: /remote-files/ }).click()
          await expect(page.locator('.fx-state')).toContainText('awaiting machine support')
          const lib = new URL('./js/lib.js', url.split('#')[0]).href
          const connection = new URL('./js/engine.js', url.split('#')[0]).href
          // Use the real connection's close to prevent reconnecting while proving the
          // screen's existing offline state; no engine records are invented.
          await page.evaluate(async ({ lib, connection }) => { const engine = await import(connection); engine.connection().close(); const state = await import(lib); state.engineWrite({ conn: { status: 'offline', lastHeard: 0 } }) }, { lib, connection })
          await expect(page.locator('.fx-state')).toContainText('is offline')
        })
      })).pipe(Effect.provide(buildServices(rig, engineLists).pipe(Layer.provide(dependencies))), Effect.provide(runner.layer))
    })).pipe(Effect.provide(SqliteClient.layer({ filename: rig.database, readerWorker: true })), Effect.scoped) as Effect.Effect<void, unknown, never>)
    expect(errors).toEqual([])
  } finally { if (previous === undefined) delete process.env.LGTM_HOME; else process.env.LGTM_HOME = previous; removeRig(rig) }
}
