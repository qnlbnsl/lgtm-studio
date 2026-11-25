import { readFileSync } from 'node:fs'
import { expect, it } from 'vitest'
import { smallfixSource } from '../scripts/smallfixSource.ts'

it('fails the build when the roster preview consumer changes', () => {
  const source = readFileSync('design/mock/js/screens/messages.js', 'utf8')
  expect(smallfixSource('js/screens/messages.js', source)).toContain("why: board.currentActivity(a)")
  expect(() => smallfixSource('js/screens/messages.js', '')).toThrow('Roster activity integration no longer matches')
})

for (const destination of ['team/build', 'lead/checkout']) it(`the mock Integrate control names ${destination}`, async () => {
  const source = readFileSync('design/mock/js/screens/git.js', 'utf8')
  const component = source.slice(source.indexOf('function Integrate('), source.indexOf('// ---- the tab'))
  const kit = await import('../../../design/mock/vendor/htm-preact/standalone.module.js' as string)
  const Integrate = new Function('html', 'useNow', 'Icon', 'cx', 'plural', 'baseName', 'integrate', `${component}\nreturn Integrate;`)(
    kit.html, () => Date.now(), () => null, (...v: unknown[]) => v.filter(Boolean).join(' '), (n: number) => `${n} commits`, (v: string) => v.split('/').at(-1), () => {})
  const element = document.createElement('div')
  kit.render(kit.h(Integrate, { s: {}, w: { kind: 'agent', parent: destination, files: [], ahead: 1 }, project: {} }), element)
  expect(element.querySelector('button')?.textContent).toBe(`Integrate into ${destination}`)
  kit.render(null, element)
})
