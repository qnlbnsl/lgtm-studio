import { readFileSync } from 'node:fs'
import { projectSettingsSource } from '../scripts/projectSettings.ts'
import { afterEach, beforeAll, expect, it } from 'vitest'
import { mapHomeRequest } from '../src/chat.ts'
import { mapSnapshot } from '../src/mapping.ts'
import { engineSnapshot } from './fixtures.ts'

globalThis.matchMedia ??= ((query: string) => ({ matches: false, media: query, addEventListener() {}, removeEventListener() {} })) as unknown as typeof matchMedia
let kit: any, lib: any, RequestBlock: any
beforeAll(async () => {
  kit = await import('../../../design/mock/vendor/htm-preact/standalone.module.js' as string)
  lib = await import('../../../design/mock/js/lib.js' as string)
  RequestBlock = (await import('../../../design/mock/js/ui.js' as string)).RequestBlock
})
afterEach(() => { lib.live.on = false })
for (const variant of ['card', 'detail']) it(`approval ${variant} states its fixed effect above lead text, labels Accept and warns before another answer`, async () => {
  const state: any = { ...mapSnapshot(engineSnapshot, 1), ui: { panelReq: engineSnapshot.questions[0]!.id } }
  const original = engineSnapshot.questions[0]!
  const request: any = mapHomeRequest({ ...original, kind: 'question', status: 'open', canAnswer: true,
    display: { ...original.display, projectApproval: 'skip-planning-and-build', proposedAnswer: 'Accepted', title: 'A harmless title' } } as never, state.agents)
  expect(request.projectApproval).toBe('skip-planning-and-build')
  expect(request.projectApprovalEffect).toBe('Accepting lets the lead skip planning, create sprints, teams and tasks, and start building.')
  const element = document.createElement('div'); document.body.append(element)
  try {
    kit.render(kit.h(RequestBlock, { s: state, request, variant }), element)
    expect(element.textContent!.indexOf(request.projectApprovalEffect)).toBeLessThan(element.textContent!.indexOf(variant === 'card' ? request.title : request.body))
    const buttons = [...element.querySelectorAll<HTMLButtonElement>('button')]
    expect(buttons.some((button) => button.textContent?.trim() === 'Accept')).toBe(true)
    expect(element.textContent).not.toContain('Accept recommendation')
    const answer = element.querySelector<HTMLInputElement>('input[aria-label="Your own answer"], input[aria-label="Another answer"]')!
    answer.value = 'Yes, accepted. Go ahead.'; answer.dispatchEvent(new Event('input', { bubbles: true }))
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(element.textContent).toContain('This answer does not approve. Use Accept to grant approval.')
  } finally { kit.render(null, element); element.remove() }
})

it('both approval layouts have guarded live replacements', () => {
  const source = readFileSync('design/mock/js/ui.js', 'utf8')
  expect(projectSettingsSource('js/ui.js', source).split('chatLive.approvalEffect(r)').length).toBe(3)
  expect(() => projectSettingsSource('js/ui.js', source.replace('const approvalEffect = r.projectApprovalEffect;', 'moved'))).toThrow('cards moved')
})
