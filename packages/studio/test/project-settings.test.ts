import { readFileSync } from 'node:fs'
import { afterEach, beforeAll, expect, it, vi } from 'vitest'
import * as mapping from '../src/projectSettings.ts'
import { projectSettingsSource } from '../scripts/projectSettings.ts'
import { harnessChoicesSource } from '../scripts/harnessChoices.ts'
import { serviceSettingsSource } from '../scripts/serviceSettings.ts'
import { harnessIntegrationSource } from '../scripts/harnessIntegration.ts'
import { gitActionsSource } from '../scripts/gitActions.ts'
import type { AgentConfiguration, AgentChoices } from '@lgtm/contracts'

const report = { configuration: { desired: { model: 'desired', effort: 'low' }, applied: { model: 'old', effort: 'high' }, status: 'pending' },
  models: { source: 'negotiated', options: [{ id: 'desired', label: 'Reported model' }, { id: 'next' }] },
  efforts: { source: 'negotiated', options: [{ id: 'low', label: 'Low' }] } } satisfies AgentChoices & { configuration: AgentConfiguration }
it('maps desired lead settings, exact engine choices and pending acknowledgement separately', () => {
  expect(mapping.mapLeadSettings(report)).toEqual({ model: 'desired', effort: 'low', applied: report.configuration.applied, status: 'pending', statusLabel: 'Waiting for the agent', reason: '',
    models: [{ id: 'desired', label: 'Reported model' }, { id: 'next', label: 'next' }], efforts: [{ id: 'low', label: 'Low' }], modelReason: '', effortReason: '' })
  expect(mapping.mapLeadSettings({ ...report, configuration: { desired: {}, status: 'unavailable' }, models: { source: 'unavailable', options: [], reason: 'Sign in first' } }))
    .toMatchObject({ model: '', effort: '', models: [], modelReason: 'Sign in first' })
})
const current = () => {
  const path = 'js/screens/service.js', source = readFileSync('design/mock/' + path, 'utf8')
  return serviceSettingsSource(path, gitActionsSource(path, harnessIntegrationSource(path, harnessChoicesSource(path, source))))
}
it('patches only the live output and fails closed if the target screen moves', () => {
  const source = current(), result = projectSettingsSource('js/screens/service.js', source)
  expect(result).toContain('projectSettings.loadLead')
  expect(result).toContain('id="ps-name"')
  expect(result).toContain('Save name')
  expect(result).toContain('projectSettings.deleteTeam')
  expect(result).toContain('projectSettings.changeApproval')
  expect(result).toContain('projectSettings.changePermission')
  expect(() => projectSettingsSource('js/screens/service.js', source.replace('board.changePermission', 'board.movedPermission'))).toThrow('Project settings integration')
  expect(() => projectSettingsSource('js/screens/service.js', source.replace('board.changeApproval', 'board.movedApproval'))).toThrow('Project settings integration')
  expect(result.slice(result.indexOf('export function ProjectSettings'))).not.toContain('Engine work needed')
  expect(result.slice(result.indexOf('export function ProjectSettings'))).not.toContain('refuse()')
  expect(() => projectSettingsSource('js/screens/service.js', source.replace('export function ProjectSettings', 'function MovedSettings'))).toThrow('Project settings integration')
  expect(projectSettingsSource('js/screens/other.js', 'unchanged')).toBe('unchanged')
})

let lib: any, engine: any, actions: any
beforeAll(async () => {
  globalThis.matchMedia ??= ((query: string) => ({ matches: false, media: query, addEventListener() {}, removeEventListener() {} })) as unknown as typeof matchMedia
  lib = await import('../../../design/mock/js/lib.js' as string)
  engine = await import('../../../design/mock/js/engine.js' as string)
  actions = await import('../../../design/mock/js/live/project-settings.js' as string)
  actions.installMappings(mapping)
})
afterEach(() => { vi.clearAllTimers(); vi.useRealTimers(); lib.live.on = false; engine.setConnection(null) })
function setup(request: (tag: string, payload: any) => Promise<unknown>) {
  vi.useFakeTimers(); lib.live.on = true
  lib.store.state = lib.guardStore({ session: { status: 'in' }, ui: {}, toasts: [], boardAgents: {}, projectLeadSettings: {}, projects: [{ id: 'project', name: 'Original', goal: 'Original goal', lead: 'lead' }], agents: [{ id: 'lead', model: 'old', effort: 'high' }] })
  engine.setConnection({ request })
}
for (const [label, tag, key, action] of [
  ['change permission', 'project.permission.set', 'project:project', () => actions.changePermission('project', 'planning', 'ask')],
  ['withdraw approval', 'project.approval.withdraw', 'project:project', () => actions.changeApproval('project', 'build', false)],
  ['rename', 'project.rename', 'project:project', () => actions.renameProject('project', 'New name')],
  ['goal', 'project.goal.set', 'project:project', () => actions.saveGoal('project', 'New goal')],
  ['lead model', 'configure', 'configure:lead', () => actions.configureLead('lead', { model: 'next' })],
  ['lead effort', 'configure', 'configure:lead', () => actions.configureLead('lead', { effort: 'low' })],
] as const) for (const refused of [false, true]) it(`${label} keeps records authoritative and shows pending state on ${refused ? 'refusal' : 'acknowledgement'}`, async () => {
  let finish!: () => void
  const held = new Promise<void>((resolve) => { finish = resolve })
  const request = vi.fn(async (command: string) => {
    if (command === tag) { await held; if (refused) throw new Error('The engine refused this specific change'); return { requestId: 'ack' } }
    if (command === 'client.agent.configuration') return report
    throw new Error('Unexpected command ' + command)
  })
  setup(request)
  const before = JSON.stringify([lib.store.state.projects, lib.store.state.agents])
  const pending = action()
  expect(lib.store.state.ui.boardBusy[key]).toBe(true)
  expect(JSON.stringify([lib.store.state.projects, lib.store.state.agents])).toBe(before)
  await action()
  expect(request.mock.calls.filter(([command]) => command === tag)).toHaveLength(1)
  finish(); await pending
  expect(lib.store.state.ui.boardBusy[key]).toBeUndefined()
  expect(JSON.stringify([lib.store.state.projects, lib.store.state.agents])).toBe(before)
  if (refused) expect(lib.store.state.toasts.at(-1)).toMatchObject({ text: 'The engine refused this specific change', tone: 'error' })
  else if (tag === 'configure') expect(lib.store.state.projectLeadSettings.lead).toMatchObject({ model: 'desired', status: 'pending' })
})
it('a lead read arriving after sign-out cannot restore private settings', async () => {
  let finish!: (value: unknown) => void
  const held = new Promise((resolve) => { finish = resolve })
  setup(() => held)
  const pending = actions.loadLead('lead')
  lib.engineWrite({ session: { status: 'out' }, boardAgents: {}, projectLeadSettings: {} })
  finish(report); await pending
  expect(lib.store.state.projectLeadSettings).toEqual({})
})

it('omits every unsupported setting and empty section, while keeping facts and teams', async () => {
  const kit = await import('../../../design/mock/vendor/htm-preact/standalone.module.js' as string)
  const data = await import('../../../design/mock/js/data.js' as string)
  const ui = await import('../../../design/mock/js/ui.js' as string)
  const { mapSnapshot } = await import('../src/mapping.ts')
  const board = await import('../src/board.ts')
  const liveBoard = await import('../../../design/mock/js/live/board.js' as string)
  const { engineSnapshot } = await import('./fixtures.ts')
  const state: any = { ...board.mapBoardRecords(engineSnapshot, mapSnapshot(engineSnapshot, 1)), ui: {}, settings: {}, projectLeadSettings: { 'ag-lead': mapping.mapLeadSettings(report) }, boardAgents: { 'ag-lead': { settings: mapping.mapLeadSettings(report), models: report.models.options, efforts: report.efforts.options } }, sprints: [], tasks: [], requests: [] }
  const source = projectSettingsSource('js/screens/service.js', current()).replace(/^import[\s\S]*?from ['"][^'"\n]+['"];$/gm, '').replace(/export /g, '')
  const context = { ...lib, ...data, ...ui, useEffect() {}, registerDialog() {}, act: {}, board: liveBoard, liveGit: {}, mocks: {}, projectSettings: actions }
  const Component = new Function(...Object.keys(context), source + '\nreturn ProjectSettings;')(...Object.values(context))
  lib.live.on = true
  const element = document.createElement('div'); document.body.append(element)
  try {
    kit.render(kit.h(Component, { s: state, project: state.projects[0] }), element)
    for (const selector of ['#ps-base', '#ps-sub', '[name=ps-perm]', '[aria-label="Write tests first"]', '[aria-label="Agent review"]', '[aria-label="Remove agent credits when merging"]', '#ps-all', '[aria-label="Plan harness"]', '[aria-label="Plan model"]', '[aria-label="Plan effort"]', '[aria-label="Need-to-know for Alpha team"]', '#ps-prefix']) {
      const controls = element.querySelectorAll<HTMLInputElement>(selector)
      expect(controls.length, selector).toBe(0)
    }
    for (const label of ['Approve', 'Apply', 'Delete project']) {
      expect([...element.querySelectorAll<HTMLButtonElement>('button')].find((b) => b.textContent?.trim() === label), label).toBeUndefined()
    }
    expect(element.querySelector('[title="Engine work needed"]')).toBeNull()
    expect(element.querySelector('.svc-assign')).toBeNull()
    expect(element.querySelector('.svc-label small')).toBeNull()
    expect([...element.querySelectorAll('h2')].map((h) => h.textContent)).not.toContain('Agents')
    expect([...element.querySelectorAll('h2')].map((h) => h.textContent)).not.toContain('Assignment')
    expect(element.querySelector('#ps-machine')).toBeNull()
    expect(element.textContent).toContain('feature/harvest/alpha')
    expect(element.querySelectorAll('.svc-teams tbody a')).toHaveLength(4)
    expect(element.textContent).toContain('/srv/repos/orchard/.worktrees/harvest')
    expect(element.querySelector('[aria-label="Lead model"]')).not.toBeNull()
  } finally { kit.render(null, element); element.remove() }
})

it('the standalone mock changes each permission and withdraws a sample grant independently', async () => {
  const kit = await import('../../../design/mock/vendor/htm-preact/standalone.module.js' as string)
  const data = await import('../../../design/mock/js/data.js' as string)
  const screen = await import('../../../design/mock/js/screens/service.js' as string)
  lib.live.on = false; vi.useFakeTimers()
  const state = { ...data.seed(), ui: {}, toasts: [] }; screen.seedService(state); lib.store.state = state
  const project = state.projects[0], stage = project.stage
  const element = document.createElement('div'); document.body.append(element)
  const draw = () => kit.render(kit.h(screen.ProjectSettings, { s: state, project }), element)
  const control = (permission: string) => element.querySelector<HTMLSelectElement>('#ps-permission-' + permission)!
  const change = (permission: string, value: string) => { control(permission).value = value; control(permission).dispatchEvent(new Event('change', { bubbles: true })); draw() }
  try {
    draw()
    for (const permission of ['planning', 'starting-work']) {
      expect(control(permission).value).toBe('allowed')
      expect([...control(permission).options].map((o) => o.text)).toEqual(['Allowed', 'Not allowed', 'Always ask for permission'])
    }
    change('planning', 'denied')
    expect(project.permissions.planning).toEqual({ setting: 'denied', granted: false })
    expect(control('starting-work').value).toBe('allowed')
    change('planning', 'ask')
    expect(element.querySelector('[aria-label="Project planning grant"]')?.textContent).toBe('Not granted')
    expect(element.querySelector('[aria-label="Withdraw Project planning"]')).toBeNull()
    project.permissions = { planning: { setting: 'ask', granted: true }, 'starting-work': { setting: 'ask', granted: true } }; draw()
    element.querySelector<HTMLButtonElement>('[aria-label="Withdraw Project planning"]')!.click(); draw()
    expect(project.permissions.planning.granted).toBe(false)
    expect(project.permissions['starting-work'].granted).toBe(true)
    change('starting-work', 'allowed')
    expect(element.querySelector('[aria-label="Starting work grant"]')).toBeNull()
    expect(project.stage).toBe(stage)
  } finally { kit.render(null, element); element.remove() }
})
