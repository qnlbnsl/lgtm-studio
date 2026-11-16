import { expect, it } from 'vitest'
import { mapProject } from '../src/mapping.ts'
import { engineSnapshot } from './fixtures.ts'

it('maps recorded creation and phase times without inventing missing phases', () => {
  const row = { ...engineSnapshot.project!.projects[0]!, stage: 'define', createdAt: 1000,
    phaseTimes: [{ phase: 'define', start: 1000 }] }
  expect(mapProject(row as never, [])).toMatchObject({ phase: 'define', createdAt: 1000, phaseTimes: row.phaseTimes })
  expect(mapProject(engineSnapshot.project!.projects[0]!, [])).toMatchObject({ createdAt: null, phaseTimes: [] })
})

it('renders a fresh Define phase and leaves unknown durations blank, while the mock keeps samples', async () => {
  const lib = await import('../../../design/mock/js/lib.js' as string)
  const { seed } = await import('../../../design/mock/js/data.js' as string)
  const { WorkflowTab } = await import('../../../design/mock/js/screens/workflow.js' as string)
  const s = seed(), p = s.projects[0]
  const element = document.createElement('div')
  const draw = () => lib.render(lib.h(WorkflowTab, { s, project: p, route: { parts: [] } }), element)
  draw()
  expect(element.querySelector('.wf-time')?.textContent).toMatch(/Started|took/)
  p.phase = 'define'; p.createdAt = Date.now(); p.phaseTimes = [{ phase: 'define', start: p.createdAt }]
  draw()
  expect(element.querySelector('.wf-status')?.textContent).toContain('Phase 1 of 8')
  expect(element.querySelector('.wf-time')?.textContent).toMatch(/Started .* ago/)
  p.phase = 'build'; p.phaseTimes = []
  draw()
  expect(element.querySelector('.wf-time')?.textContent).toBe('')
  expect([...element.querySelectorAll('.wf-rail .meta')].map(x => x.textContent)).toEqual(['', '', '', 'Now', '', '', '', ''])
  lib.render(null, element)
})

it('fails the live build if the workflow timing anchor changes', async () => {
  const { workflowSource } = await import('../scripts/workflowSource.ts')
  expect(() => workflowSource('changed source')).toThrow('Workflow timing integration')
  expect(workflowSource('const times = phaseTimes(s, p, cur);')).toBe('const times = recordedPhaseTimes(p);')
})
