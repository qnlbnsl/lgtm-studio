import { readFileSync } from 'node:fs'
import { afterEach, beforeAll, expect, it, vi } from 'vitest'
import * as mapping from '../src/projectSettings.ts'
import * as source from '../scripts/projectSettings.ts'
import { harnessChoicesSource } from '../scripts/harnessChoices.ts'
import { serviceSettingsSource } from '../scripts/serviceSettings.ts'
import { harnessIntegrationSource } from '../scripts/harnessIntegration.ts'
import { gitActionsSource } from '../scripts/gitActions.ts'
import { mapBoardRecords } from '../src/board.ts'
import { mapSnapshot } from '../src/mapping.ts'
import { engineSnapshot } from './fixtures.ts'
import type { AgentConfiguration, AgentChoices } from '@lgtm/contracts'
const agentId = 'ag-lead'
const report = { configuration: { desired: { model: 'current', effort: 'low' }, status: 'applied' },
  models: { source: 'negotiated', options: [{ id: 'current', label: 'Current model' }, { id: 'next', label: 'Next model' }] },
  efforts: { source: 'negotiated', options: [{ id: 'low', label: 'Low' }, { id: 'high', label: 'High' }] }
} satisfies AgentChoices & { configuration: AgentConfiguration }
const next = { ...report, configuration: { ...report.configuration, desired: { model: 'next', effort: 'high' } } }
let lib: any, engine: any, actions: any, board: any, kit: any, data: any, ui: any
const roots: HTMLElement[] = []
const flush = async () => { for (let i = 0; i < 10; i++) await Promise.resolve() }
beforeAll(async () => {
  globalThis.matchMedia ??= ((query: string) => ({ matches: false, media: query, addEventListener() {}, removeEventListener() {} })) as unknown as typeof matchMedia
  lib = await import('../../../design/mock/js/lib.js' as string)
  engine = await import('../../../design/mock/js/engine.js' as string)
  actions = await import('../../../design/mock/js/live/project-settings.js' as string)
  board = await import('../../../design/mock/js/live/board.js' as string)
  kit = await import('../../../design/mock/vendor/htm-preact/standalone.module.js' as string)
  data = await import('../../../design/mock/js/data.js' as string)
  ui = await import('../../../design/mock/js/ui.js' as string)
  actions.installMappings(mapping)
})
afterEach(() => {
  for (const root of roots.splice(0)) { kit.render(null, root); root.remove() }
  lib.live.on = false; engine.setConnection(null); vi.useRealTimers()
})
function copied() {
  const path = 'js/screens/service.js'
  return serviceSettingsSource(path, gitActionsSource(path, harnessIntegrationSource(path, harnessChoicesSource(path, readFileSync('design/mock/' + path, 'utf8')))))
}
function fixture(options: { request?: (tag: string, payload: any) => Promise<unknown>; state?: object; effects?: boolean; raw?: boolean } = {}) {
  const settings = mapping.mapLeadSettings(report)
  lib.live.on = true
  lib.store.state = lib.guardStore({ ...mapBoardRecords(engineSnapshot, mapSnapshot(engineSnapshot, 1)), session: { status: 'in' }, settings: {}, ui: {}, toasts: [],
    boardAgents: { [agentId]: { ...settings, settings } }, projectLeadSettings: { [agentId]: settings }, ...options.state })
  const request = vi.fn(options.request ?? (async (tag: string) => {
    if (tag === 'client.agent.configuration') return report
    if (tag === 'client.agent.context') return { context: { _tag: 'unavailable', reason: 'not-live' } }
    throw new Error('Unexpected request ' + tag)
  }))
  engine.setConnection({ request, board: { mapAgentDetails: (r: any) => ({ models: r.models.options, efforts: r.efforts.options, context: null }) } })
  const screen = source.projectSettingsSource('js/screens/service.js', options.raw ? readFileSync('design/mock/js/screens/service.js', 'utf8') : copied())
    .replace(/^import[\s\S]*?from ['"][^'"\n]+['"];$/gm, '').replace(/export /g, '')
  // Layout effects run synchronously here, while retaining the real dependency comparison.
  const context = { ...lib, ...data, ...ui, useEffect: options.effects ? kit.useLayoutEffect : () => {}, registerDialog() {}, act: {}, board, liveGit: {}, mocks: {}, projectSettings: actions }
  const Component = new Function(...Object.keys(context), screen + '\nreturn ProjectSettings;')(...Object.values(context))
  const root = document.createElement('div'); document.body.append(root); roots.push(root)
  const draw = () => kit.render(kit.h(Component, { s: lib.store.state, project: lib.store.state.projects[0] }), root)
  const model = () => root.querySelector<HTMLSelectElement>('[aria-label="Lead model"]')!
  const effort = () => root.querySelector<HTMLSelectElement>('[aria-label="Lead effort"]')!
  draw()
  return { request, root, draw, model, effort, state: lib.store.state }
}
it('R1.1 an unchanged lead in a fresh roster causes no read or disabled picker', async () => {
  let release!: (r: unknown) => void
  const held = new Promise((resolve) => { release = resolve })
  let reads = 0
  const f = fixture({ effects: true, request: async () => ++reads === 1 ? report : held })
  try {
    await flush(); f.draw()
    expect(reads).toBe(1)
    lib.engineWrite({ agents: f.state.agents.map((a: any) => ({ ...a })) })
    f.draw(); f.draw()
    expect(reads).toBe(1)
    expect(f.model().disabled).toBe(false)
    expect(f.effort().disabled).toBe(false)
  } finally { release(report); await flush() }
})
for (const [axis, value] of [['model', 'next'], ['effort', 'low'], ['state', 'stopped']]) {
  it(`R1.1 refreshes when the lead ${axis} changes`, async () => {
    const f = fixture({ effects: true })
    await flush(); f.draw()
    lib.engineWrite({ agents: f.state.agents.map((a: any) => a.id === agentId ? { ...a, [axis!]: value } : a) })
    f.draw(); await flush(); f.draw()
    expect(f.request.mock.calls.filter(([tag]) => tag === 'client.agent.configuration')).toHaveLength(2)
    expect(f.model().disabled).toBe(false)
  })
}
it('R1.1 existing choices remain editable during a background read', () => {
  const f = fixture({ state: { ui: { boardBusy: { ['read-project-lead:' + agentId]: true } } } })
  expect(f.model().disabled).toBe(false)
  expect(f.effort().disabled).toBe(false)
})
it('R1.2 a write refreshes independently of an older read and cannot be overwritten by it', async () => {
  let release!: (r: unknown) => void
  const held = new Promise((resolve) => { release = resolve })
  let reads = 0
  const f = fixture({ request: async (tag) => tag === 'configure' ? { requestId: 'ack' } : ++reads === 1 ? held : next })
  const older = actions.loadLead(agentId)
  try {
    await actions.configureLead(agentId, { model: 'next' })
    expect(reads).toBe(2)
    expect(f.state.projectLeadSettings[agentId].model).toBe('next')
  } finally { release(report); await older }
  expect(f.state.projectLeadSettings[agentId].model).toBe('next')
})
it('R1.3 a refused model change reads again and reverts the visible picker to the engine value', async () => {
  const f = fixture({ request: async (tag) => { if (tag === 'configure') throw new Error('command refused'); return report } })
  f.model().value = 'next'; f.model().dispatchEvent(new Event('change', { bubbles: true }))
  await flush(); f.draw()
  expect(f.request.mock.calls.filter(([tag]) => tag === 'client.agent.configuration')).toHaveLength(1)
  expect(f.request.mock.calls.find(([tag]) => tag === 'configure')?.[1]).toMatchObject({ model: 'next' })
  expect(f.model().value).toBe('current')
  expect(f.model().options).toHaveLength(2)
  expect(f.state.toasts.at(-1)).toMatchObject({ text: 'command refused', tone: 'error' })
})
it('R1.4 a pending project write disables Rename, the goal, and both lead pickers and says Saving', async () => {
  const f = fixture()
  const goal = f.root.querySelector<HTMLTextAreaElement>('#ps-goal')!
  goal.value = 'A goal draft'; goal.dispatchEvent(new Event('input', { bubbles: true }))
  await flush()
  lib.update((s: any) => { s.ui.boardBusy = { ['project:' + s.projects[0].id]: true } })
  f.draw()
  const rename = [...f.root.querySelectorAll('button')].find((b) => b.textContent?.trim() === 'Rename')!
  expect(rename.disabled).toBe(true)
  expect(goal.disabled).toBe(true)
  expect(f.model().disabled).toBe(true)
  expect(f.effort().disabled).toBe(true)
  const save = [...f.root.querySelectorAll('button')].find((b) => b.textContent?.trim() === 'Saving…')!
  expect(save?.disabled).toBe(true)
})
it('R1.5 rejects a missing switch row before slicing the wrong control', () => {
  expect(() => source.projectSettingsSource('js/screens/service.js', copied().replaceAll('<${Row} label="Write tests first"', '<${Row} label="Moved setting"'))).toThrow('switch moved')
})
for (const markup of ['<${Row} label="Write tests first"><div />', '<${Row} label="Write tests first"><${Switch} label="Write tests first"']) {
  it('R1.5 rejects a negative switch boundary', () => {
    expect(() => source.wrapProjectSettingsSwitch(markup, 'Write tests first')).toThrow('switch moved')
  })
}
it('R1.6 renders before harness-choice replacements when choices are absent', () => {
  expect(() => fixture({ raw: true, state: { boardAgents: {}, projectLeadSettings: {} } })).not.toThrow()
})
it('R1.6 rejects live output without the lead loader', () => {
  expect(() => source.assertProjectSettingsSource('js/screens/service.js', '<main>disconnected</main>')).toThrow('projectSettings.loadLead')
})
it('R1.7 a failed lead read cannot corrupt the shared board agent records', async () => {
  const f = fixture({ request: async () => { throw new Error('read refused') } })
  const before = f.state.boardAgents
  await actions.loadLead(agentId)
  expect(f.state.boardAgents).toBe(before)
  expect(lib.ENGINE_OWNED.has('projectLeadSettings')).toBe(true)
})
it('R1.7 the board agent loader cannot drop private lead settings', async () => {
  const f = fixture({ state: { projectLeadSettings: {} } })
  await actions.loadLead(agentId)
  await board.loadAgents([agentId])
  expect(f.state.projectLeadSettings[agentId]?.model).toBe('current')
})
for (const [status, text] of [['applied', 'Applied'], ['pending', 'Waiting for the agent'], ['refused', 'Not applied'], ['unsupported', 'Not applied'], ['unavailable', 'Not applied']] as const) {
  it(`R1.8 explains ${status} in plain words`, () => {
    const settings = mapping.mapLeadSettings({ ...report, configuration: { ...report.configuration, status, reason: 'not-acknowledged' } })
    const f = fixture({ state: { projectLeadSettings: { [agentId]: settings }, boardAgents: { [agentId]: { ...settings, settings } } } })
    expect(f.root.textContent).toContain(text)
    expect(f.root.textContent).not.toContain('not-acknowledged')
  })
}
