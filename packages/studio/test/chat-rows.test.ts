import { readFileSync } from 'node:fs'
import { afterEach, beforeAll, expect, it } from 'vitest'
import type { EventEnvelope } from '@lgtm/contracts'
import { mapThread } from '../src/chat.ts'
import { toolRow } from '../src/chatTools.ts'
import { chatSource } from '../scripts/chatSource.ts'
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
  const items = [last, question, envelope(2, { _tag: 'text', text: 'Which rule?' }, { itemId: 'echo' }), first]
  expect(mapThread([...items, first], 'ag-worker').items).toMatchObject([
    { id: '1', ts: 1000, text: 'Earlier reply.' }, { id: '3', type: 'request', requestId: 'q' }, { id: '4', ts: 4000, text: 'Later reply.' },
  ])
  expect(mapThread([first, tool(2, { title: 'Read', input: '{"file_path":"src/gitApi.ts"}' }), last], 'ag-worker').items.map((item) => item.id)).toEqual(['1', '2', '4'])
})

it('does not combine calls whose ids are reused in another delivery', () => {
  const boundary = { sequence: 2, occurredAt: 2000, event: { _tag: 'DeliverySent', agentId: 'ag-worker', deliveryId: 'new' } } as EventEnvelope
  expect(mapThread([change(1, diff), boundary, change(3, 'next turn')], 'ag-worker').items).toHaveLength(2)
})

it('names reads, writes, commands, searches and engine tools while retaining the raw tool', () => {
  const row = (title: string, input?: object) => toolRow({ id: '1', ts: 1 }, { _tag: 'tool-call', callId: 'c', title, ...(input ? { input: JSON.stringify(input) } : {}) })
  expect(row('Read', { file_path: '/work/src/gitApi.ts' })).toMatchObject({ summary: 'Read gitApi.ts', raw: 'Read' })
  expect(row('Write', { file_path: '/work/notes/sprint.md', content: '# Sprint' })).toMatchObject({ summary: 'Wrote sprint.md' })
  expect(row('Bash', { command: 'pnpm typecheck\npnpm test' })).toMatchObject({ summary: 'Ran pnpm typecheck', details: [{ label: 'Command', text: 'pnpm typecheck\npnpm test' }] })
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

it('guards the live chat binding and keeps all sample calls as individual tool rows without an engine', async () => {
  const source = readFileSync('design/mock/js/ui.js', 'utf8')
  expect(chatSource('js/ui.js', source)).toContain('liveMode.on ? chatLive.watchThread(agentId) : undefined')
  expect(() => chatSource('js/ui.js', source.replace('useEffect(() => chatLive.watchThread(agentId), [agentId]);', 'moved'))).toThrow('binding moved')
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
