import { readFileSync } from 'node:fs'
import { expect, it, vi } from 'vitest'
import { serviceSettingsSource } from '../scripts/serviceSettings.ts'
import { harnessChoicesSource } from '../scripts/harnessChoices.ts'

const gate = () => { let release!: () => void; const held = new Promise<void>(r => { release = r }); return { held, release } }
function fixture() {
  const backend: any = { project: { lead: { harness: 'first', account: { _tag: 'any' }, permission: 'ask' } }, roles: { limits: { stallMinutes: 5, teamIdleMinutes: 15 } } }
  const state: any = { session: { status: 'in' }, ui: {}, settings: {}, subscriptions: [] }
  const writes: any[] = [], toasts: string[] = []
  let beforeWrite = async () => {}, beforeProjectRead = async (_value: any) => {}
  const request = async (tag: string, payload?: any) => {
    if (tag === 'accounts') return { accounts: [] }
    if (tag === 'harnesses') return { harnesses: ['first', 'second'].map(id => ({ id, defaultPermission: 'ask' })) }
    if (tag === 'project.settings') { const settings = structuredClone(backend.project); await beforeProjectRead(settings); return { settings } }
    if (tag === 'roles.settings.get') return { settings: structuredClone(backend.roles) }
    if (tag === 'client.web.settings') return { enabled: false }
    if (tag === 'roles.role-file.get') return { file: {} }
    if (tag === 'client.snapshot') return { accounts: [] }
    if (tag === 'client.harness.status') return { harnesses: [] }
    if (tag === 'client.harness.choices') return { models: { options: [] }, efforts: { options: [] } }
    if (tag === 'roles.settings.set' || tag === 'project.settings.set') {
      await beforeWrite(); writes.push(structuredClone({ tag, settings: payload.settings }))
      backend[tag === 'roles.settings.set' ? 'roles' : 'project'] = structuredClone(payload.settings)
      return {}
    }
    throw new Error(tag)
  }
  const connection = { request }
  const source = serviceSettingsSource('js/live/git.js', harnessChoicesSource('js/live/git.js', readFileSync('design/mock/js/live/git.js', 'utf8')))
    .replace(/^import .*;$/gm, '').replace(/export /g, '')
  const actions = new Function('store', 'live', 'engineWrite', 'notify', 'engine', 'act', 'HARNESSES', 'serviceSettings', 'addEventListener', 'removeEventListener',
    `${source}\nreturn { installMappings, refreshAccounts, saveSettings, busy, openScreen, observe };`)(
    { state }, { on: true }, (patch: object) => Object.assign(state, patch), () => {}, { request, connection: () => connection },
    { toast: (text: string) => toasts.push(text) }, {}, { loadVersion: async () => {} }, () => {}, () => {})
  actions.installMappings({ mapHarnesses: (h: any[]) => Object.fromEntries(h.map(x => [x.id, x])), mapInstalledHarnesses: () => ({}), mapAccounts: () => [],
    mapServiceSettings: (project: any, _h: any, roles: any) => ({ workspaceFolder: project.workspaceFolder, defaultHarness: project.lead.harness, stallWorker: roles.limits.stallMinutes, teamIdle: roles.limits.teamIdleMinutes }) })
  return { state, backend, actions, writes, toasts, holdWrites: (f: typeof beforeWrite) => { beforeWrite = f }, holdReads: (f: typeof beforeProjectRead) => { beforeProjectRead = f } }
}

it('saves a workspace through the settings queue and preserves the complete project record and permission marker', async () => {
  const f = fixture()
  Object.assign(f.backend.project, { workspaceFolder: '/home/maya/projects/personal', shell: '/bin/sh', displayName: 'Maya Okafor',
    workflowFolder: '/home/maya/workflows', reviewDefaults: { reviewers: 3, differentVendor: false } })
  f.backend.project.lead.permissionChosen = true
  const previous = structuredClone(f.backend.project)
  await f.actions.refreshAccounts()
  expect(await f.actions.saveSettings({ workspaceFolder: '/home/maya/projects' })).toBe(true)
  expect(f.backend.project).toEqual({ ...previous, workspaceFolder: '/home/maya/projects' })
  expect(f.state.settings.workspaceFolder).toBe('/home/maya/projects')
  expect(f.writes).toEqual([{ tag: 'project.settings.set', settings: f.backend.project }])
})

it('shows the engine workspace refusal and restores the saved folder', async () => {
  const f = fixture(), reason = 'The workspace folder must contain the registered repository "Product".'
  f.backend.project.workspaceFolder = '/home/maya/projects'
  await f.actions.refreshAccounts()
  f.holdWrites(async () => { throw new Error(reason) })
  expect(await f.actions.saveSettings({ workspaceFolder: '/home/maya/other' })).toBe(false)
  expect(f.toasts.map(String)).toEqual([`Error: ${reason}`])
  expect(f.state.settings.workspaceFolder).toBe('/home/maya/projects')
  expect(f.writes).toEqual([])
})

it('keeps a focused workspace draft while refreshed settings arrive', async () => {
  const source = readFileSync('design/mock/js/screens/service.js', 'utf8')
  const start = source.indexOf('function Defaults('), end = source.indexOf('\nfunction Remote(', start)
  const kit = await import('../../../design/mock/vendor/htm-preact/standalone.module.js' as string)
  const save = vi.fn(), Row = ({ children }: any) => kit.html`<div>${children}</div>`
  const Defaults = new Function('html', 'useState', 'useRef', 'Row', 'PermissionPicker', 'Switch', 'setS', `${source.slice(start, end)}\nreturn Defaults;`)(
    kit.html, kit.useState, kit.useRef, Row, () => null, () => null, save)
  const root = document.createElement('div'); document.body.append(root)
  const draw = (workspaceFolder: string) => kit.render(kit.h(Defaults, { s: { settings: { workspaceFolder } } }), root)
  try {
    draw('/home/maya/projects'); const input = root.querySelector('input')!; input.focus()
    input.value = '/home/maya/pro'; input.dispatchEvent(new Event('input', { bubbles: true }))
    await vi.waitFor(() => expect(input.value).toBe('/home/maya/pro'))
    draw('/home/maya/refreshed'); expect(input.value).toBe('/home/maya/pro')
    expect(save).not.toHaveBeenCalled()
    input.dispatchEvent(new Event('change', { bubbles: true }))
    expect(save).toHaveBeenLastCalledWith({ workspaceFolder: '/home/maya/pro' })
    input.blur()
    await vi.waitFor(() => expect(input.value).toBe('/home/maya/refreshed'))
  } finally { kit.render(null, root); root.remove() }
})

it('queues rapid settings changes through readback and preserves both fields and the following harness', async () => {
  const f = fixture(), pending = gate(); let started = 0
  await f.actions.refreshAccounts()
  f.holdWrites(async () => { if (++started === 1) await pending.held })
  const first = f.actions.saveSettings({ stallWorker: 7 })
  await vi.waitFor(() => expect(started).toBe(1))
  const second = f.actions.saveSettings({ teamIdle: 35 }), third = f.actions.saveSettings({ defaultHarness: 'second' })
  try {
    expect(f.state.settings).toMatchObject({ stallWorker: 7, teamIdle: 35, defaultHarness: 'second' })
    expect(f.actions.busy('settings')).toBe(true)
  } finally { pending.release(); await Promise.all([first, second, third]) }
  expect(f.backend.roles.limits).toEqual({ stallMinutes: 7, teamIdleMinutes: 35 })
  expect(f.backend.project.lead.harness).toBe('second')
  expect(f.writes).toHaveLength(3); expect(f.toasts).toEqual([])
  expect(f.actions.busy('settings')).toBe(false)
})

it('applies only the latest reload when an older reply arrives last', async () => {
  const f = fixture(), pending = gate(); let reads = 0
  await f.actions.refreshAccounts()
  f.holdReads(async () => { if (++reads === 1) await pending.held })
  const older = f.actions.refreshAccounts()
  await vi.waitFor(() => expect(reads).toBe(1))
  f.backend.project.lead.harness = 'second'
  try {
    await f.actions.refreshAccounts()
    expect(f.state.settings.defaultHarness).toBe('second')
  } finally { pending.release(); await older }
  expect(f.state.settings.defaultHarness).toBe('second')
})

it('retains newer edits across a background reload and coalesces unsent values of the same field', async () => {
  const f = fixture(), pending = gate(); let started = 0
  await f.actions.refreshAccounts()
  f.holdWrites(async () => { if (++started === 1) await pending.held })
  const first = f.actions.saveSettings({ stallWorker: 7 })
  await vi.waitFor(() => expect(started).toBe(1))
  const next = f.actions.saveSettings({ stallWorker: 8 }), latest = f.actions.saveSettings({ stallWorker: 12 })
  try {
    await f.actions.refreshAccounts()
    expect(f.state.settings.stallWorker).toBe(12)
  } finally { pending.release(); await Promise.all([first, next, latest]) }
  expect(f.writes.map(w => w.settings.limits.stallMinutes)).toEqual([7, 12])
  expect(f.state.settings.stallWorker).toBe(12)
})

it('finishes a current readback after navigation invalidates a queued save reload', async () => {
  const f = fixture(), pending = gate(); let reads = 0
  const close = f.actions.openScreen('settings')
  await vi.waitFor(() => expect(f.actions.busy('read-service')).toBe(false))
  f.holdReads(async () => { if (++reads === 1) await pending.held })
  const first = f.actions.saveSettings({ stallWorker: 7 }), next = f.actions.saveSettings({ teamIdle: 35 })
  await vi.waitFor(() => expect(reads).toBe(1))
  close(); pending.release()
  await Promise.all([first, next])
  expect(f.backend.roles.limits).toEqual({ stallMinutes: 7, teamIdleMinutes: 35 })
  expect(reads).toBe(3)
})

it('cancels unsent edits on sign-out and clears pending state for the next session', async () => {
  const f = fixture(), pending = gate(); let writes = 0
  await f.actions.refreshAccounts()
  f.holdWrites(async () => { writes++; await pending.held })
  const first = f.actions.saveSettings({ stallWorker: 7 }), next = f.actions.saveSettings({ teamIdle: 35 })
  await vi.waitFor(() => expect(writes).toBe(1))
  f.actions.observe({ session: { status: 'out' } })
  expect(await next).toBe(false)
  expect(f.actions.busy('settings')).toBe(false)
  pending.release(); expect(await first).toBe(false)
  expect(writes).toBe(1)
})

it('keeps a focused partial number while a background reload changes its stored value', async () => {
  const source = readFileSync('design/mock/js/screens/service.js', 'utf8')
  const start = source.indexOf('function Num('), end = source.indexOf('\nfunction Section(', start)
  const kit = await import('../../../design/mock/vendor/htm-preact/standalone.module.js' as string)
  const Num = new Function('html', 'useState', 'useRef', `${source.slice(start, end)}\nreturn Num;`)(kit.html, kit.useState, kit.useRef)
  const root = document.createElement('div'); document.body.append(root)
  const change = vi.fn(), draw = (value: number) => kit.render(kit.h(Num, { id: 'number', value, min: 5, onChange: change }), root)
  try {
    draw(5); const input = root.querySelector('input')!; input.focus()
    input.value = '1'; input.dispatchEvent(new Event('input', { bubbles: true }))
    await vi.waitFor(() => expect(input.value).toBe('1'))
    draw(20); expect(input.value).toBe('1'); expect(change).not.toHaveBeenCalled()
    input.value = '17'; input.dispatchEvent(new Event('input', { bubbles: true }))
    expect(change).toHaveBeenLastCalledWith(17)
  } finally { kit.render(null, root); root.remove() }
})

async function remoteForm() {
  const source = readFileSync('design/mock/js/screens/service.js', 'utf8')
  const start = source.indexOf('function Remote('), end = source.indexOf('\nfunction StallWatch(', start)
  const kit = await import('../../../design/mock/vendor/htm-preact/standalone.module.js' as string)
  const save = vi.fn(), Row = (props: any) => kit.html`<div>${props.children}</div>`
  const Remote = new Function('html', 'useState', 'useEffect', 'Row', 'Switch', 'Command', 'live', 'setS', 'act', `${source.slice(start, end)}\nreturn Remote;`)(kit.html, kit.useState, kit.useEffect, Row, () => null, () => null, { on: true }, save, {})
  const root = document.createElement('div'); document.body.append(root)
  const base = { webEnabled: true, port: 8797, checkIn: 25, proxyMode: false, publicAddress: '' }
  const draw = (settings: object) => kit.render(kit.h(Remote, { s: { settings, machines: [] } }), root)
  draw(base)
  return { root, base, draw, save, close: () => { kit.render(null, root); root.remove() } }
}
it('remote access refreshes untouched fields without inventing unsaved changes', async () => {
  const f = await remoteForm()
  try {
    f.draw({ ...f.base, port: 8899, checkIn: 45 })
    await vi.waitFor(() => expect((f.root.querySelector('[aria-label="Port"]') as HTMLInputElement).value).toBe('8899'))
    expect((f.root.querySelector('#ra-check') as HTMLInputElement).value).toBe('45')
    expect((f.root.querySelector('[type=submit]') as HTMLButtonElement).disabled).toBe(true)
    expect(f.root.textContent).toContain('No unsaved changes.')
  } finally { f.close() }
})
it('remote access keeps a real edit while refreshing other fields and never saves their old values', async () => {
  const f = await remoteForm()
  try {
    const check = f.root.querySelector('#ra-check') as HTMLInputElement
    check.value = '37'; check.dispatchEvent(new Event('input', { bubbles: true }))
    await vi.waitFor(() => expect((f.root.querySelector('[type=submit]') as HTMLButtonElement).disabled).toBe(false))
    f.draw({ ...f.base, port: 8899 })
    await vi.waitFor(() => expect((f.root.querySelector('[aria-label="Port"]') as HTMLInputElement).value).toBe('8899'))
    expect(check.value).toBe('37')
    f.root.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    expect(f.save).toHaveBeenCalledWith({ ...f.base, port: 8899, checkIn: 37 })
  } finally { f.close() }
})
