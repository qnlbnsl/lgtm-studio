import { readFileSync } from 'node:fs'
import { projectSettingsSource } from '../scripts/projectSettings.ts'
import { beforeAll, expect, it } from 'vitest'
let kit: any, StageDots: any
beforeAll(async () => {
  globalThis.matchMedia ??= ((query: string) => ({ matches: false, media: query, addEventListener() {}, removeEventListener() {} })) as unknown as typeof matchMedia
  kit = await import('../../../design/mock/vendor/htm-preact/standalone.module.js' as string)
  StageDots = (await import('../../../design/mock/js/screens/home.js' as string)).StageDots
})
it('Define is the current first stage dot', () => {
  const root = document.createElement('div')
  try {
    kit.render(kit.h(StageDots, { stage: 'Define' }), root)
    expect(root.querySelectorAll('i.now')).toHaveLength(1)
    expect(root.querySelector('i:first-child')?.classList.contains('now')).toBe(true)
    expect(root.querySelectorAll('i.done')).toHaveLength(0)
  } finally { kit.render(null, root) }
})

it('stage and archive display changes have guarded live replacements', () => {
  const stages = readFileSync('design/mock/js/data.js', 'utf8')
  expect(projectSettingsSource('js/data.js', stages)).toContain("STAGES = Object.freeze(['Define'")
  expect(() => projectSettingsSource('js/data.js', stages.replace("['Define', 'Design'", "['Design'"))).toThrow('stages moved')
  const messages = readFileSync('design/mock/js/screens/messages.js', 'utf8')
  expect(() => projectSettingsSource('js/screens/messages.js', messages.replace('>Archived</span>', '>Changed</span>'))).toThrow('mark moved')
})
