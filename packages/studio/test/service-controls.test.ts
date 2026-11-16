import { readFileSync } from 'node:fs'
import { expect, it } from 'vitest'
import { serviceSettingsSource } from '../scripts/serviceSettings.ts'
import { harnessChoicesSource } from '../scripts/harnessChoices.ts'
import { harnessIntegrationSource } from '../scripts/harnessIntegration.ts'
import { gitActionsSource } from '../scripts/gitActions.ts'
const original = readFileSync('design/mock/js/screens/service.js', 'utf8')
const copied = (input = original) => serviceSettingsSource('js/screens/service.js', gitActionsSource('js/screens/service.js', harnessIntegrationSource('js/screens/service.js', harnessChoicesSource('js/screens/service.js', input))))
async function draw(section: string, sample = false) {
  const kit = await import('../../../design/mock/vendor/htm-preact/standalone.module.js' as string)
  const source = (sample ? original : copied()).replace(/^import[\s\S]*?from ['"][^'"\n]+['"];$/gm, '').replace(/export /g, '')
  const s = { ui: { serviceReady: true }, settings: { notifyOn: {}, webEnabled: true, port: 12345, checkIn: 25, publicAddress: '' }, prefs: {}, subscriptions: [], repos: [], machines: [{ role: 'master', name: 'Sample' }], conn: { status: 'live' } }
  const context = { ...kit, useEffect: () => {}, live: { on: !sample }, HARNESSES: {}, KIND_LABEL: {}, PERMISSION_MODES: [], registerDialog: () => {}, href: (x: string) => x, cx: (...v: unknown[]) => v.filter(Boolean).join(' '), Icon: () => null, liveGit: { busy: () => false }, serviceSettings: { busy: () => false }, act: { PR_TITLE_LIMIT: 72, commitMessage: () => '' }, refuse: () => {} }
  const Component = new Function(...Object.keys(context), `${source}; return ServiceSettings;`)(...Object.values(context))
  const root = document.createElement('div')
  kit.render(kit.h(Component, { s, route: { query: new URLSearchParams({ section }) } }), root)
  return { root, close: () => kit.render(null, root) }
}
for (const [section, labels] of [
  ['defaults', ['Need-to-know messages', 'Remove agent credits when merging']],
  ['workflow', ['Write tests first', 'Agent review', 'Review rounds']],
  ['stall-watch', ['Standing permission rules']],
  ['about', ['Update the service']],
] as const) {
  for (const label of labels) it(`omits the inert ${label} control only from the website`, async () => {
    const live = await draw(section), sample = await draw(section, true)
    try {
      if (label === 'Update the service') expect([...live.root.querySelectorAll('button')].some(b => b.textContent === label)).toBe(false)
      else expect(live.root.querySelector('.svc-pane')!.textContent).not.toContain(label)
      if (label !== 'Update the service') expect(sample.root.querySelector('.svc-pane')!.textContent).toContain(label)
    } finally { live.close(); sample.close() }
  })
}
it('omits the empty Git messages section and Approvals heading only from the website', async () => {
  const live = await draw('stall-watch'), sample = await draw('stall-watch', true)
  try {
    expect(live.root.querySelector('a[href="/settings?section=git"]')).toBeNull()
    expect(live.root.textContent).not.toContain('Approvals')
    expect(sample.root.querySelector('a[href="/settings?section=git"]')).not.toBeNull()
    expect(sample.root.textContent).toContain('Approvals')
  } finally { live.close(); sample.close() }
})

for (const anchor of [
  '<${Row} label="Need-to-know messages"', '<${Row} label="Remove agent credits when merging"',
  '<${Row} label="Standing permission rules"', '<${Row} label="Write tests first"', '<${Row} label="Agent review"', '<${Row} label="Review rounds"',
  '<h3>Approvals</h3>', "  ['git', 'Git messages', 'git-commit-horizontal', GitMessages, 'The model that writes commit and pull request messages.'],",
]) {
  it(`fails the build when the removed control anchor is absent or duplicated: ${anchor}`, () => {
    expect(original).toContain(anchor)
    expect(() => copied(original.replaceAll(anchor, 'moved'))).toThrow()
    expect(() => copied(original.replaceAll(anchor, anchor + anchor))).toThrow()
  })
}
