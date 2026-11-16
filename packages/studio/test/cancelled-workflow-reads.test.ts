import { readFileSync } from 'node:fs'
import { afterEach, expect, it } from 'vitest'
import { fixture, flush } from './cancelled-read-fixture.ts'
let f: Awaited<ReturnType<typeof fixture>>
afterEach(() => f?.close())
for (const name of ['open', 'initial list effect', 'validation effect', 'save', 'clone', 'remove callback']) it(`${name} releases the workflow spinner after a cancelled waiting read`, async () => {
  f = await fixture()
  const states: any[] = [], effects: (() => void)[] = []; let index = 0
  const source = readFileSync('design/mock/js/screens/workflows.js', 'utf8')
  const begin = source.indexOf('export function Workflows('), end = source.indexOf('  return html`<div class="wfl"', begin)
  const body = source.slice(begin, end).replace('export ', '') + '\nreturn {open, save, clone, remove}; }\nreturn Workflows;'
  const useState = (initial: any) => {
    const n = index++; states[n] = n === 2 ? f.workflowRecord : n === 3 ? { ...f.workflowRecord.definition, name: 'edited' } : initial
    return [states[n], (v: any) => { states[n] = v }]
  }
  const Workflows = Function('useState', 'useRef', 'useEffect', 'live', 'request', 'confirm', 'copy', 'errorText', body)(useState, (value: any) => ({ current: value }), (fn: any) => effects.push(fn), { on: true },
    (tag: string, p: any) => f.engine.request('client.workflows.' + tag, p), (d: any) => f.dialogs.push(d), (v: any) => structuredClone(v), f.errorText)
  const actions = Workflows({ s: f.state })
  let pending: Promise<unknown> | undefined
  if (name === 'initial list effect') effects[0]!()
  else if (name === 'validation effect') effects[1]!()
  else if (name === 'remove callback') { actions.remove(); pending = f.dialogs[0].onConfirm() }
  else pending = name === 'open' ? actions.open(f.workflowRecord) : name === 'clone' ? actions.clone('copy', 'user') : actions.save()
  await flush(); const latch = name === 'validation effect' ? 8 : 9; expect(states[latch]).toBe(true)
  await f.cancel(); await pending
  expect(states[latch]).toBe(false); expect(states[6]).toBe(''); expect(states[7]).toBe(''); expect(states[12]).toBe('')
})
it('terminal load releases its shared loading promise and admits the next load', async () => {
  f = await fixture()
  const terminal = f.load('live/terminal.js', 'load, status: () => loading')
  const pending = terminal.load().catch((e: any) => e)
  expect(terminal.status()).toBeInstanceOf(Promise)
  await f.cancel(); expect(await pending).toMatchObject({ cancelledRead: true }); expect(terminal.status()).toBeNull()
  f.online(); await terminal.load(); expect(f.calls.filter(c => c.tag === 'client.web.settings')).toHaveLength(2)
})
it('the shared toast presenter ignores cancellation and preserves real failures', async () => {
  f = await fixture(); const text = readFileSync('design/mock/js/actions.js', 'utf8')
  const begin = text.indexOf('export function toast('), end = text.indexOf('// In live mode', begin)
  const toast = Function('errorText', 'uid', 'update', text.slice(begin, end).replace('export ', '') + '\nreturn toast;')(f.errorText, () => 'toast', (fn: any) => fn(f.state))
  toast(Object.assign(new Error('cancelled'), { cancelledRead: true }), 'error'); expect(f.state.toasts).toEqual([])
  toast(new Error('Service refused'), 'error'); expect(f.state.toasts.map((t: any) => t.text)).toEqual(['Service refused'])
})
it('terminal settings keep their original Reading wording after a cancelled waiting read', async () => {
  f = await fixture()
  Object.assign(f.env, {
    useState: (v: any) => [v, () => {}], useEffect: () => {}, Icon: () => null,
    html: (parts: TemplateStringsArray, ...values: any[]) => parts.reduce((s, part, n) => s + part + (typeof values[n] === 'string' ? values[n] : ''), ''),
  })
  const terminal = f.load('live/terminal.js', 'load, LiveTerminalTab, setMapping')
  terminal.setMapping({ terminalWorktrees: () => [], terminalSelection: () => undefined })
  const render = () => terminal.LiveTerminalTab({ s: f.state, project: { id: 'p', machine: 'local' } })
  const pending = terminal.load().catch((e: any) => e)
  expect(render()).toContain('Reading terminal settings…')
  await f.cancel(); expect(await pending).toMatchObject({ cancelledRead: true })
  expect(render()).toContain('Reading terminal settings…')
})
