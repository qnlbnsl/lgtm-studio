import { readFileSync } from 'node:fs'
import { afterEach, beforeAll, expect, it } from 'vitest'
import type { EventEnvelope } from '@lgtm/contracts'
import { mapThread } from '../src/chat.ts'
import { toolRow } from '../src/chatTools.ts'
import { translate } from '../../engine/src/runner/translate.ts'
import { mapSnapshot } from '../src/mapping.ts'
import { engineSnapshot } from './fixtures.ts'

const envelope = (sequence: number, content: object, extra = {}) => ({ sequence, occurredAt: sequence * 1000, event: { _tag: 'ThreadItem', agentId: 'ag-worker', itemId: 'message', content, ...extra } }) as EventEnvelope
const tool = (sequence: number, content: object) => envelope(sequence, { _tag: 'tool-call', callId: 'edit', ...content })
const change = (sequence: number, diff: string, callId = 'edit') => envelope(sequence, { _tag: 'file-change', callId, path: 'notes/MEMORY.md', diff })
const diff = '@@ -1 +1 @@\n-old rule\n+new rule'

for (const shape of ['path', 'changes']) it(`folds repeated ${shape} diff snapshots into their call once, retaining later status and order`, () => {
  const update = { sessionUpdate: 'tool_call_update', toolCallId: 'edit', content: [shape === 'path'
    ? { type: 'diff', path: 'notes/MEMORY.md', oldText: 'old rule\n', newText: 'new rule\n' }
    : { type: 'diff', changes: [{ path: 'notes/MEMORY.md' }], patch: { text: diff, format: 'git_patch' } }] }
  const translated = translate(update, false)[0]!.content
  const items = [tool(1, { title: 'Edit', input: JSON.stringify({ file_path: 'notes/MEMORY.md', old_string: 'old rule', new_string: 'new rule' }) }),
    envelope(2, translated), envelope(3, translated), tool(4, { title: '', status: 'completed', output: JSON.stringify(update.content) }),
    envelope(5, { _tag: 'text', text: 'Updated the rule.' })]
  const mapped = mapThread([...items].reverse(), 'ag-worker').items
  expect(mapped).toHaveLength(2)
  expect(mapped[0]).toMatchObject({ id: '1', ts: 1000, type: 'tools', summary: 'Edited MEMORY.md', raw: 'Edit', status: 'completed', details: [{ label: 'notes/MEMORY.md', kind: 'diff' }] })
  const detail = (mapped[0]!.details as { text: string }[])
  expect(detail).toHaveLength(1)
  expect(detail[0]!.text).toContain('-old rule\n+new rule')
  expect(mapped[1]).toMatchObject({ id: '5', text: 'Updated the rule.' })
})

it('keeps distinct edits of the same path, and renders a multi-file patch only once', () => {
  const patch = 'diff --git a/a.ts b/a.ts\n--- a/a.ts\n+++ b/a.ts\n@@ -1 +1 @@\n-a\n+b'
  const items = [change(1, diff), change(2, 'another edit', 'second'),
    envelope(3, { _tag: 'file-change', callId: 'multi', path: 'a.ts', diff: patch }),
    envelope(4, { _tag: 'file-change', callId: 'multi', path: 'b.ts', diff: patch })]
  const mapped = mapThread(items, 'ag-worker').items
  expect(mapped).toHaveLength(3)
  expect(mapped[0]).toMatchObject({ summary: 'Edited MEMORY.md' })
  expect(mapped[1]).toMatchObject({ summary: 'Edited MEMORY.md', details: [{ text: 'another edit' }] })
  expect(mapped[2]).toMatchObject({ summary: 'Edited a.ts and 1 more file', details: [{ text: patch }] })
  expect(mapped[2]!.details).toHaveLength(1)
})

it('orders by engine sequence, splits a reused text id at actions, and shows question echoes only on the card', () => {
  const question = { sequence: 3, occurredAt: 9000, event: { _tag: 'QuestionAsked', agentId: 'ag-worker', questionId: 'q', title: 'Choose', text: 'Which rule?' } } as EventEnvelope
  const first = envelope(1, { _tag: 'text', text: 'Earlier reply.' })
  const last = envelope(4, { _tag: 'text', text: 'Later reply.' })
  const items = [last, question, envelope(2, { _tag: 'text', text: '**Choose**' }, { itemId: 'echo' }), first]
  expect(mapThread([...items, first], 'ag-worker').items).toMatchObject([
    { id: '1', ts: 1000, text: 'Earlier reply.' }, { id: '3', type: 'request', requestId: 'q' }, { id: '4', ts: 4000, text: 'Later reply.' },
  ])
  expect(mapThread([first, tool(2, { title: 'Read', input: '{"file_path":"src/gitApi.ts"}' }), last], 'ag-worker').items.map((item) => item.id)).toEqual(['1', '2', '4'])
})

it('does not combine calls whose ids are reused after a turn ends', () => {
  const boundary = { sequence: 2, occurredAt: 2000, event: { _tag: 'DeliverySent', agentId: 'ag-worker', deliveryId: 'new' } } as EventEnvelope
  const end = { sequence: 1.5, occurredAt: 1500, event: { _tag: 'TurnEnded', agentId: 'ag-worker', compact: false } } as EventEnvelope
  expect(mapThread([change(1, diff), end, boundary, change(3, 'next turn')], 'ag-worker').items).toHaveLength(2)
})

it('names reads, writes, commands, searches and engine tools while retaining the raw tool', () => {
  const row = (title: string, input?: object) => toolRow({ id: '1', ts: 1 }, { _tag: 'tool-call', callId: 'c', title, ...(input ? { input: JSON.stringify(input) } : {}) })
  expect(row('Read', { file_path: '/work/src/gitApi.ts' })).toMatchObject({ summary: 'Read gitApi.ts', raw: 'Read' })
  expect(row('Write', { file_path: '/work/notes/sprint.md', content: '# Sprint' })).toMatchObject({ summary: 'Wrote sprint.md' })
  expect(row('Bash', { command: 'pnpm typecheck\npnpm test' })).toMatchObject({ summary: 'Ran pnpm typecheck…', details: [{ label: 'Command', text: 'pnpm typecheck\npnpm test' }] })
  expect(row('Grep', { pattern: 'ThreadItem', path: '/work/src' })).toMatchObject({ summary: 'Searched src' })
  expect(row('Read /work/src/gitApi.ts')).toMatchObject({ summary: 'Read gitApi.ts', raw: 'Read', details: [{ label: 'Path', text: '/work/src/gitApi.ts' }] })
  expect(row('mcp__lgtm__project_sprint_create')).toMatchObject({ summary: 'Created sprint', raw: 'mcp__lgtm__project_sprint_create' })
})

globalThis.matchMedia ??= ((query: string) => ({ matches: false, media: query, addEventListener() {}, removeEventListener() {} })) as unknown as typeof matchMedia
let kit: any, ui: any, lib: any
const mounts: HTMLElement[] = []
beforeAll(async () => {
  kit = await import('../../../design/mock/vendor/htm-preact/standalone.module.js' as string)
  ui = await import('../../../design/mock/js/ui.js' as string)
  lib = await import('../../../design/mock/js/lib.js' as string)
})
afterEach(() => { for (const element of mounts.splice(0)) { kit.render(null, element); element.remove() }; lib.live.on = false })
function render(component: any, props: object) {
  const element = document.createElement('div'); document.body.append(element); mounts.push(element)
  kit.render(kit.h(component, props), element); return element
}
const tick = () => new Promise((resolve) => setTimeout(resolve, 0))

it('keeps paragraphs, adjacent ordered and nested lists, inline code and fenced code without interpreting HTML or URLs', () => {
  const text = 'First paragraph.\n\nNext:\n- One `file.ts`\n  - Nested\n- Two\n1. Ordered\n2. Again\n```ts\nconst literal = "<script>bad()</script>";\n\nnext();\n```\n[unsafe](javascript:bad())\n<img src=x onerror=bad()>'
  const element = render(ui.Rich, { text })
  expect(element.querySelectorAll('ul')).toHaveLength(2)
  expect(element.querySelectorAll('ol > li')).toHaveLength(2)
  expect(element.querySelector('li code')?.textContent).toBe('file.ts')
  expect(element.querySelector('pre code')?.textContent).toBe('const literal = "<script>bad()</script>";\n\nnext();')
  expect(element.querySelectorAll('script,img,a')).toHaveLength(0)
  expect(element.textContent).toContain('<img src=x onerror=bad()>')
})

it('starts tool rows collapsed and expands long output and diffs without losing newlines or visible signs', async () => {
  const output = Array.from({ length: 80 }, (_, i) => `line ${i}`).join('\n') + '\n<script>END</script>'
  const element = render(ui.ToolGroup, { item: { summary: 'Edited MEMORY.md', raw: 'Edit', status: 'completed', details: [{ label: 'notes/MEMORY.md', kind: 'diff', text: diff }, { label: 'Output', text: output }] } })
  expect(element.querySelector('details')?.open).toBe(false)
  element.querySelector('summary')!.click(); await tick()
  expect(element.querySelector('details')?.open).toBe(true)
  expect(element.querySelector('pre')?.textContent).toBe(diff)
  expect(element.querySelector('.del')?.textContent).toBe('-old rule\n')
  expect(element.querySelector('.add')?.textContent).toBe('+new rule')
  expect(element.textContent).not.toContain('line 79')
  ;[...element.querySelectorAll('button')].find((button) => button.textContent?.trim() === 'Show more')!.click(); await tick()
  expect(element.querySelectorAll('pre')[1]!.textContent).toBe(output)
  expect(element.querySelector('script')).toBeNull()
  expect(element.textContent!.split('notes/MEMORY.md')).toHaveLength(2)
})

it('shows a formatted question once before and after Details opens', async () => {
  const state = { ...mapSnapshot(engineSnapshot, 1), ui: {} }
  const request = { id: 'q', agent: 'ag-worker', kind: 'question', title: 'Choose a rule', body: 'Which rule?\n- One\n- Two', status: 'open', options: [], createdAt: 1 }
  const element = render(ui.RequestBlock, { s: state, request })
  expect(element.textContent!.split('Which rule?')).toHaveLength(2)
  expect(element.querySelectorAll('ul li')).toHaveLength(2)
  const details = [...element.querySelectorAll('button')].find((button) => button.textContent?.trim() === 'Details')!
  details.click(); await tick()
  expect(element.textContent!.split('Which rule?')).toHaveLength(2)
  expect(element.textContent).not.toContain('The question')
})

it('keeps the direct live binding and all sample calls as individual rows without an engine', async () => {
  const source = readFileSync('design/mock/js/ui.js', 'utf8')
  expect(source).toContain('useEffect(() => chatLive.watchThread(agentId), [agentId]);')
  expect(readFileSync('packages/studio/scripts/build.ts', 'utf8')).not.toContain('chatSource(')
  const { seed } = await import('../../../design/mock/js/data.js' as string)
  const sample = seed()
  const rows = sample.threads['console-lead'].filter((item: any) => item.type === 'tools')
  for (const prefix of ['Read ', 'Searched ', 'Edited ', 'Wrote ', 'Ran ', 'Created ']) expect(rows.some((item: any) => item.summary.startsWith(prefix))).toBe(true)
  expect(sample.owner.name).toBe('Maya Okafor')
  for (const item of rows) {
    expect(item.details.length).toBeGreaterThan(0)
    const element = render(ui.ToolGroup, { item })
    expect(element.querySelectorAll('details')).toHaveLength(1)
  }
})

it('keeps the full question once when an older event used its body as its title', () => {
  const state = { ...mapSnapshot(engineSnapshot, 1), ui: {} }
  const body = 'Which rule?\n- One\n- Two'
  const element = render(ui.RequestBlock, { s: state, request: { id: 'q-legacy', agent: 'ag-worker', kind: 'question', title: 'Which rule?', body, status: 'open', options: [], createdAt: 1 } })
  expect(element.querySelector('.qc-q')?.textContent).toBe('Question')
  expect(element.textContent!.split('Which rule?')).toHaveLength(2)
})

it('retains distinct successive hunks and folds both individual and cumulative repeats once', () => {
  const next = '@@ -8 +8 @@\n-second old\n+second new'
  const result = mapThread([tool(1, { title: 'Edit', status: 'pending' }), change(2, diff), change(3, next), change(4, diff + '\n' + next), change(5, next), tool(6, { title: '', status: 'completed' })], 'ag-worker').items
  expect(result).toHaveLength(1)
  expect(result[0]).toMatchObject({ summary: 'Edited MEMORY.md', raw: 'Edit', status: 'completed', details: [{ text: diff + '\n' + next }] })
})

it('keeps an unfinished call across a mid-turn delivery and preserves its raw name', () => {
  const boundary = { sequence: 2, occurredAt: 2000, event: { _tag: 'DeliverySent', agentId: 'ag-worker', deliveryId: 'steer' } } as EventEnvelope
  const result = mapThread([tool(1, { title: 'Edit', status: 'pending' }), boundary, change(3, diff), tool(4, { title: '', status: 'completed' })], 'ag-worker').items
  expect(result).toHaveLength(1)
  expect(result[0]).toMatchObject({ id: '1', raw: 'Edit', status: 'completed', details: [{ text: diff }] })
})

it('titles a diff-first creation as a write even when its completing title is empty', () => {
  const creation = translate({ sessionUpdate: 'tool_call_update', toolCallId: 'edit', content: [{ type: 'diff', path: 'notes/MEMORY.md', oldText: null, newText: 'created' }] }, false)[0]!.content
  for (const first of [change(1, '@@ -0,0 +1 @@\n+created'), envelope(1, creation)]) {
    const result = mapThread([first, tool(2, { title: '', status: 'completed' })], 'ag-worker').items
    expect(result).toHaveLength(1)
    expect(result[0]).toMatchObject({ summary: 'Wrote MEMORY.md', raw: 'Write', status: 'completed' })
  }
})

it('cleans quoted paths, ranges, engine tool names, array commands, and quoted commands', () => {
  const row = (title: string, input?: object) => toolRow({ id: '1', ts: 1 }, { _tag: 'tool-call', callId: 'c', title, ...(input ? { input: JSON.stringify(input) } : {}) })
  expect(row('Read `/w/src/gitApi.ts`')).toMatchObject({ summary: 'Read gitApi.ts', details: [{ label: 'Path', text: '/w/src/gitApi.ts' }] })
  expect(row('Read /w/a.ts (1 - 40)')).toMatchObject({ summary: 'Read a.ts', details: [{ label: 'Path', text: '/w/a.ts' }] })
  expect(row('mcp__lgtm__ask').summary).toBe('Asked a question')
  expect(row('mcp__lgtm__project_task_list').summary).toBe('Listed tasks')
  expect(row('Shell', { command: ['bash', '-lc', 'pnpm test'] })).toMatchObject({ summary: 'Ran bash -lc pnpm test', details: [{ label: 'Command', text: 'bash -lc pnpm test' }] })
  expect(row('`pnpm typecheck`')).toMatchObject({ summary: 'Ran pnpm typecheck', details: [{ label: 'Command', text: 'pnpm typecheck' }] })
})

it('cleans titles and labels, caps summaries, and retains the full title inside', () => {
  const title = 'Read /w/\u202egnp.exe\nAsked the owner' + 'x'.repeat(160)
  const result = toolRow({ id: '1', ts: 1 }, { _tag: 'tool-call', callId: 'c', title })
  expect(result.summary.length).toBeLessThanOrEqual(120)
  expect(result.summary).not.toMatch(/[\n\r\u202a-\u202e\u2066-\u2069]/)
  expect(result.details.some((detail) => detail.text.includes('Asked the owner' + 'x'.repeat(160)))).toBe(true)
  expect(result.details.every((detail) => !/[\n\r\u202a-\u202e\u2066-\u2069]/.test(detail.label))).toBe(true)
})

it('keeps the full path label even if a hunk mentions its own path', () => {
  const result = mapThread([change(1, '@@ -1 +1 @@\n-old\n+notes/MEMORY.md')], 'ag-worker').items[0]!
  expect(result.details).toMatchObject([{ label: 'notes/MEMORY.md' }])
})

it('keeps a sample question brief closed and its full body once inside Details', async () => {
  const { seed } = await import('../../../design/mock/js/data.js' as string)
  const state = seed(); state.ui = {}; const request = state.requests.find((row: any) => row.id === 'q-stuck')
  const element = render(ui.RequestBlock, { s: state, request })
  const bodyText = request.body.replace(/`/g, '')
  expect(element.textContent).toContain(request.summary)
  expect(element.textContent).not.toContain(bodyText)
  ;[...element.querySelectorAll('button')].find((button) => button.textContent?.trim() === 'Details')!.click(); await tick()
  expect(element.textContent!.split(bodyText)).toHaveLength(2)
  expect(element.querySelectorAll('.qc-part.q')).toHaveLength(1)
})

it('marks hunk lines only, leaves old content plain, and gives Copy a readable name', () => {
  const patch = '--- notes/MEMORY.md\n+++ notes/MEMORY.md\n@@ -1 +1 @@\n-old\n+new'
  const result = mapThread([change(1, patch), change(2, '- a Markdown list\n+ plain text', 'legacy')], 'ag-worker').items
  const element = render(ui.ToolGroup, { item: result[0] })
  expect([...element.querySelectorAll('.add,.del')].map((line) => line.textContent)).toEqual(['-old\n', '+new'])
  expect(element.querySelector('[aria-label="Copy diff"]')).not.toBeNull()
  expect(element.querySelector('h5')?.textContent).toBe('notes/MEMORY.md')
  const legacy = render(ui.ToolGroup, { item: result[1] })
  expect(legacy.querySelectorAll('.add,.del')).toHaveLength(0)
  expect(legacy.textContent).toContain('The earlier text was not recorded.')
  expect(legacy.querySelector('pre')?.textContent).toBe('- a Markdown list\n+ plain text')
  const path = render(ui.ToolGroup, { item: toolRow({ id: 'path', ts: 1 }, { _tag: 'tool-call', callId: 'path', title: 'Read `/w/a.ts`' }) })
  expect(path.querySelector('[aria-label="Copy path"]')).not.toBeNull()
})

it('marks literal triple-sign lines inside a hunk while leaving file headers unmarked', () => {
  const patch = '--- a/file\n+++ b/file\n@@ -1 +1 @@\n--- old heading\n+++ new heading'
  const element = render(ui.ToolGroup, { item: toolRow({ id: '1', ts: 1 }, { _tag: 'tool-call', callId: 'c', title: 'Edit' }, new Map([['file', patch]])) })
  expect([...element.querySelectorAll('.add,.del')].map((line) => line.textContent)).toEqual(['--- old heading\n', '+++ new heading'])
})

it('shows both identical replace-all parts and deduplicates only a later whole-content snapshot', () => {
  const part = { type: 'diff', path: 'notes/MEMORY.md', oldText: 'old', newText: 'new' }
  const translated = translate({ sessionUpdate: 'tool_call_update', toolCallId: 'edit', status: 'completed', content: [part, part] }, false)
  const fragment = '@@ Change (line numbers unavailable) @@\n-old\n+new'
  const both = fragment + '\n' + fragment
  const snapshot = translated.find((item) => item.content._tag === 'file-change')!.content
  expect(snapshot).toMatchObject({ diff: both })
  const items = translated.map((item, i) => envelope(i + 1, item.content))
  const mapped = mapThread([...items, envelope(3, snapshot), change(4, fragment)], 'ag-worker').items
  expect(mapped).toHaveLength(1)
  expect(mapped[0]).toMatchObject({ summary: 'Edited MEMORY.md', status: 'completed', details: [{ text: both + '\n' + fragment }] })
  const element = render(ui.ToolGroup, { item: mapThread(items, 'ag-worker').items[0] })
  expect(element.querySelector('pre')?.textContent).toBe(both)
  expect(element.querySelectorAll('.del')).toHaveLength(2)
  expect(element.querySelectorAll('.add')).toHaveLength(2)
})

it('dates an answered card at the question time while retaining its recorded answer', () => {
  const state = { ...mapSnapshot(engineSnapshot, 1), ui: {} }
  const createdAt = new Date(2026, 10, 15, 11, 9).getTime(), answeredAt = new Date(2026, 10, 15, 11, 21).getTime()
  const element = render(ui.RequestBlock, { s: state, request: { id: 'q-time', agent: 'ag-worker', kind: 'question', title: 'Test denial or approval next?', body: 'Choose a test.', status: 'answered', options: [], answer: 'Denial', recommendation: 'Test denial first.', createdAt, answeredAt } })
  expect(element.querySelector('.qc-age')?.textContent).toBe(lib.clock(createdAt))
  expect(element.textContent).toContain('Denial')
  expect(element.textContent).toContain('Answered at ' + lib.clock(answeredAt))
  expect(element.querySelector('.qc-rec')?.textContent).toContain('Test denial first.')
})

it('uses the shared folder dropdown in the sample and live terminal and opens directly', async () => {
  const { TerminalLaunch } = await import('../../../design/mock/js/terminal-launch.js' as string)
  const options = [{ value: 'project', label: 'Project', folder: '/work/project' }, { value: 'base', label: 'main', folder: '/work' }, { value: 'remote', label: 'remote', folder: '/unavailable', reason: 'Engine work needed: local machine only' }]
  let selected = 'project', opened = ''
  const element = render(TerminalLaunch, { options, value: selected, onChange: (value: string) => { selected = value }, onOpen: () => { opened = selected } })
  const select = element.querySelector('select')!
  expect(select.closest('label')?.textContent).toContain('Open in')
  expect([...select.options].map(option => option.value)).toEqual(['project', 'base', 'remote'])
  expect(select.options[2]!.disabled).toBe(true)
  expect(select.options[2]!.textContent).toContain('Engine work needed')
  select.value = 'base'; select.dispatchEvent(new Event('change', { bubbles: true })); await tick()
  element.querySelector('button')!.click(); await tick()
  expect(opened).toBe('base')
  expect(element.querySelector('[role="dialog"]')).toBeNull()
  for (const file of ['design/mock/js/screens/terminal.js', 'design/mock/js/live/terminal.js']) {
    const source = readFileSync(file, 'utf8')
    expect(source).toContain('<${TerminalLaunch}')
    expect(source).not.toContain('Open a shell in a worktree')
  }
})


it('disables New session when the selected folder is unavailable', async () => {
  const { TerminalLaunch } = await import('../../../design/mock/js/terminal-launch.js' as string)
  const element = render(TerminalLaunch, { options: [{ value: 'remote', label: 'Remote', folder: '/unavailable', reason: 'Engine work needed: local machine only' }], value: 'remote', onChange() {}, onOpen() { throw new Error('A disabled folder must not open') } })
  expect(element.querySelector('option')!.disabled).toBe(true)
  expect(element.querySelector('option')!.textContent).toContain('Engine work needed: local machine only')
  expect(element.querySelector('button')!.disabled).toBe(true)
  element.querySelector('button')!.click(); await tick()
})

for (const status of ['open', 'answered']) it(`keeps a ${status} held question's recommendation on its card`, () => {
  lib.live.on = true
  const state = { ...mapSnapshot(engineSnapshot, 1), ui: {} }
  const element = render(ui.RequestBlock, { s: state, request: { id: 'held-q', agent: 'ag-worker', heldBy: 'ag-worker', kind: 'question', title: 'Choose a test', body: 'Test what?', status, options: [], recommendation: 'Denial first.', createdAt: 1, answeredAt: status === 'answered' ? 2 : null } })
  expect(element.textContent).toContain('Denial first.')
})

it('s19 r2 renders two durable deltas for one message as one chat bubble after replay', () => {
  const first = { sequence: 9001, occurredAt: 1000, event: { _tag: 'ThreadItem', agentId: 'ag-worker', itemId: 'r2-split', content: { _tag: 'text', text: 'Decided prose. ' } } } as EventEnvelope;
  const second = { sequence: 9002, occurredAt: 2000, event: { _tag: 'ThreadItem', agentId: 'ag-worker', itemId: 'r2-split', content: { _tag: 'text', text: '[REDACTED]' } } } as EventEnvelope;
  const state = { ...mapSnapshot(engineSnapshot, 1), pending: {}, ui: {} };
  for (const events of [[first], [first, second], [second, first, first, second]]) {
    const items = mapThread(events, 'ag-worker').items;
    const s = { ...state, threads: { 'ag-worker': items } };
    const element = render(ui.Thread, { s, agentId: 'ag-worker' });
    expect(element.querySelectorAll('.msg .body')).toHaveLength(1);
    expect(element.querySelector('.msg .body')?.textContent).toBe('Decided prose. ' + (events.length > 1 ? '[REDACTED]' : ''));
  }
});
