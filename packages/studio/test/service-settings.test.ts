// @vitest-environment node
import { readFileSync } from 'node:fs'
import { expect, test } from 'vitest'
import { serviceSettingsSource } from '../scripts/serviceSettings.ts'
import { harnessChoicesSource } from '../scripts/harnessChoices.ts'
import { harnessIntegrationSource } from '../scripts/harnessIntegration.ts'
import { gitActionsSource } from '../scripts/gitActions.ts'
const transformer = readFileSync('packages/studio/scripts/serviceSettings.ts', 'utf8')
const needles = [...transformer.matchAll(/(?:source\.)?replace(?:All)?\(((?:"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'))/g)]
  .map((match) => new Function(`return ${match[1]}`)() as string)
for (const path of ['js/live/git.js', 'js/screens/service.js']) {
  const original = readFileSync(`design/mock/${path}`, 'utf8')
  const current = gitActionsSource(path, harnessIntegrationSource(path, harnessChoicesSource(path, original)))
  test(`all settings patches match the merged ${path}`, () => {
    const result = serviceSettingsSource(path, current)
    expect(result).toContain('serviceSettings.')
    if (path.includes('screens')) expect(result).toContain('Notifications')
  })
  for (const from of new Set(needles.filter((needle) => current.includes(needle)))) {
    test(`refuses absent or duplicated settings patch ${path}: ${from.slice(0, 50)}`, () => {
      expect(() => serviceSettingsSource(path, current.split(from).join(''))).toThrow('Service settings integration')
      expect(() => serviceSettingsSource(path, current + from)).toThrow('Service settings integration')
    })
  }
}

test('a version reply preserves settings saved while it was in flight and ignores extra reply settings', async () => {
  const source = readFileSync('design/mock/js/live/service-settings.js', 'utf8').replace(/^import .*;$/gm, '').replace(/^export \{.*;$/gm, '').replace(/export /g, '')
  let release!: (value: unknown) => void
  const held = new Promise((resolve) => { release = resolve })
  const connection = {}, state = { session: { status: 'in' }, settings: { stallWorker: 10, version: 'old' } }, store = { state }
  const loadVersion = new Function('store', 'engine', 'engineWrite', 'run', 'toast', `${source}; return loadVersion;`)(
    store, { request: () => held, connection: () => connection }, (patch: any) => Object.assign(store.state, patch), (_key: string, operation: () => unknown) => operation(), () => {})
  const loading = loadVersion(true)
  state.settings = { stallWorker: 31, version: 'old' }
  release({ version: 'new', commit: 'abc', comparison: 'same', dataFolder: '/state', checkout: '/checkout', updateCommands: [], stallWorker: 999 })
  await loading
  expect(state.settings).toMatchObject({ stallWorker: 31, version: 'new', commit: 'abc', versionComparison: 'same' })
})

test('settings fieldsets block workflow checks and restarts, while ordinary saves and reloads remain editable', () => {
  const original = readFileSync('design/mock/js/screens/service.js', 'utf8')
  const current = gitActionsSource('js/screens/service.js', harnessIntegrationSource('js/screens/service.js', harnessChoicesSource('js/screens/service.js', original)))
  const result = serviceSettingsSource('js/screens/service.js', current)
  const fields = [...result.matchAll(/<fieldset\s+([^>]+)>/g)].filter(m => m[1]!.includes("liveGit.busy('subscriptions')") || m[1]!.includes("liveGit.busy('read-service')"))
  expect(fields).toHaveLength(2)
  for (const field of fields) {
    const condition = /disabled=\$\{(.*?)\}/.exec(field[1]!)?.[1]
    expect(condition).toBeDefined()
    const disabled = new Function('live', 'liveGit', 'serviceSettings', `return ${condition}`)
    for (const operation of ['settings', 'service-restart']) expect(disabled({ on: true }, { busy: () => false }, { busy: (key: string) => key === operation })).toBe(true)
    for (const operation of ['settings', 'read-service']) expect(disabled({ on: true }, { busy: (key: string) => key === operation }, { busy: () => false })).toBe(false)
  }
})

for (const status of [204, 401]) test(`s28 service restart reconnects when the session probe returns ${status}`, async () => {
  const source = readFileSync('design/mock/js/live/service-settings.js', 'utf8').replace(/^import .*;$/gm, '').replace(/^export \{.*;$/gm, '').replace(/export /g, '')
  let reconnected = 0, probes = 0
  const messages: string[] = [], connection = { reconnect: () => { reconnected++ } }
  const restart = new Function('store', 'engine', 'engineWrite', 'run', 'toast', 'fetch', 'setTimeout', `${source}; return restart;`)(
    { state: { session: { status: 'in' } } }, { request: async () => ({ restarting: true }), connection: () => connection }, () => {},
    (_key: string, operation: () => unknown) => operation(), (message: string) => messages.push(message),
    async () => { probes++; return { status } }, (resume: () => void) => resume())
  await restart()
  expect(probes).toBe(1)
  expect(reconnected).toBe(1)
  expect(messages.some(message => message.includes('every page is signed out'))).toBe(false)
  expect(messages.at(-1)).toBe(status === 204 ? 'The service is responding. Your sign-in is still valid.' : 'The service is back. Sign in again.')
})
