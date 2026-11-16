const { errorText } = await import('../../../design/mock/js/live/reads.js' as string)
import { readFileSync } from 'node:fs'
import { afterEach, beforeAll, expect, it, vi } from 'vitest'
import * as mapping from '../src/files.ts'
import { filesSource } from '../scripts/filesSource.ts'
let kit: any
beforeAll(async () => { kit = await import('../../../design/mock/vendor/htm-preact/standalone.module.js' as string) })
afterEach(() => { vi.useRealTimers(); document.body.replaceChildren() })
const text = (version = 'opened', body = 'original') => ({ kind: 'text', path: 'file.txt', version, text: body, editable: true, size: body.length })
const list = (folder = '') => ({ folder, items: [{ name: 'file.txt', path: folder ? folder + '/file.txt' : 'file.txt', kind: 'file', size: 8, change: null }], truncated: false })
function fixture(reply?: (tag: string, payload: any) => Promise<any>) {
  const request = vi.fn(reply || (async (tag: string, payload: any) => {
    if (tag === 'project.files.list') return { result: list(payload.folder) }
    if (tag === 'project.files.read') return { result: text() }
    if (tag === 'project.files.write') return { result: { version: 'saved', told: 0 } }
    if (tag === 'project.context.search') return { result: { items: [], truncated: false } }
    throw new Error(tag)
  }))
  const s: any = { session: { status: 'in' }, conn: { status: 'live' }, boardRevision: 0 }
  let editor: any
  const env = { ...kit, errorText, engine: { request }, store: { state: s }, useMedia: () => false, machineOf: () => ({ name: 'local', status: 'online' }), agentOf: () => null,
    Icon: () => null, cx: (...values: any[]) => values.filter(Boolean).join(' '), href: (s: string) => s, go: vi.fn(), plural: (n: number) => `${n} lines`, isMac: false,
    Menu: () => null, Dialog: ({ children, foot }: any) => kit.html`<section>${children}${foot}</section>`, fileSize: (n: number) => `${n} bytes`,
    CodeEditor: (props: any) => { editor = props; return kit.html`<div class="test-editor">${props.doc}</div>` } }
  const source = readFileSync('design/mock/js/live/files.js', 'utf8').replace(/^import .*;$/gm, '').replace(/export /g, '')
  const compiled = new Function(...Object.keys(env), source + '\nsetMapping(arguments[arguments.length - 1]); return { createFilesModel, WorktreeFiles, CommitFile, Tree, observe: typeof observe === "function" ? observe : undefined };')(...Object.values(env), mapping)
  const context = { projectId: 'project', worktreeId: 'work', kind: 'project', machine: 'local', online: true }
  const model = compiled.createFilesModel(context, vi.fn())
  const host = document.createElement('div'); document.body.append(host)
  const w = { id: 'work', branch: 'topic', kind: 'project', machine: 'local', folder: '/work' }
  const render = (projectId = 'project', path?: string) => kit.render(kit.h(compiled.WorktreeFiles, { s, project: { id: projectId }, w, route: { query: new URLSearchParams(path ? { f: path } : {}) } }), host)
  return { request, model, context, edit: (value: string) => editor.onChange(value), save: () => editor.onSave(), module: compiled, s, host, w, render, unmount: () => kit.render(null, host) }
}
it('R1.1 retains text and the opened version when a worktree model is remounted, and forgets it only on session end', async () => {
  const f = fixture(); await f.model.open('file.txt'); f.model.edit('my draft')
  const returned = f.module.createFilesModel({ ...f.context }, vi.fn()); f.request.mockResolvedValueOnce({ result: text('newer', 'outside change') }); await returned.open('file.txt')
  expect(returned.shown()).toBe('my draft'); expect(returned.opened.file.version).toBe('opened')
  expect(typeof f.module.observe).toBe('function')
  f.module.observe({ session: { status: 'checking' } }); f.module.observe({ session: { status: 'unavailable' } })
  expect(returned.shown()).toBe('my draft')
  f.module.observe({ session: { status: 'out' } })
  const signedOut = f.module.createFilesModel({ ...f.context }, vi.fn()); await signedOut.open('file.txt')
  expect(signedOut.dirty()).toBe(false); expect(returned.drafts.has('file.txt')).toBe(false)
})
it('R1.1 keeps the actual view draft when leaving the tab and returning', async () => {
  const f = fixture(); f.render('project', 'file.txt')
  await vi.waitFor(() => expect(f.host.querySelector('.test-editor')?.textContent).toBe('original'))
  f.edit('kept across tab')
  f.unmount()
  f.render('project', 'file.txt')
  await vi.waitFor(() => expect(f.host.querySelector('.test-editor')?.textContent).toBe('kept across tab'))
  f.unmount(); f.render('project', 'file.txt')
  await vi.waitFor(() => expect(f.host.querySelector('.test-editor')?.textContent).toBe('kept across tab'))
  f.unmount()
})
it('R1.1 isolates drafts by worktree and retains them when switching back', async () => {
  const f = fixture(); await f.model.open('file.txt'); f.model.edit('work draft')
  const other = f.module.createFilesModel({ ...f.context, worktreeId: 'other' }, vi.fn()); await other.open('file.txt')
  expect(other.shown()).toBe('original'); other.edit('other draft')
  const returned = f.module.createFilesModel({ ...f.context }, vi.fn()); await returned.open('file.txt')
  expect(returned.shown()).toBe('work draft'); expect(other.shown()).toBe('other draft')
})
it('R1.2 coalesces ten revisions into at most two refreshes per open folder, retaining existing rows', async () => {
  vi.useFakeTimers(); let finish!: () => void; let held = false
  const gate = new Promise<void>((resolve) => { finish = resolve })
  const f = fixture(async (tag, payload) => { if (tag !== 'project.files.list') throw new Error(tag); if (held) await gate; return { result: list(payload.folder) } })
  await f.model.folder(''); await f.model.folder('open'); await f.model.folder('collapsed')
  expect(typeof f.model.refresh).toBe('function')
  f.model.openFolders = new Set(['open', 'collapsed/child']); f.request.mockClear(); held = true
  const first = f.model.refresh()
  for (let n = 0; n < 10; n++) void f.model.refresh()
  expect(f.model.folders[''].items).toHaveLength(1)
  kit.render(kit.h(f.module.Tree, { model: f.model, open: new Set(), toggle() {}, pick() {} }), f.host)
  expect(f.host.textContent).toContain('file.txt'); expect(f.host.textContent).not.toContain('Loading…')
  finish(); await vi.advanceTimersByTimeAsync(999)
  expect(f.request.mock.calls.filter(([, p]) => !p.folder)).toHaveLength(1)
  await vi.advanceTimersByTimeAsync(1); await first
  expect(f.request.mock.calls.filter(([, p]) => !p.folder)).toHaveLength(2)
  expect(f.request.mock.calls.filter(([, p]) => p.folder === 'open')).toHaveLength(2)
  expect(f.request.mock.calls.some(([, p]) => p.folder === 'collapsed')).toBe(false)
  expect(f.request.mock.calls.some(([, p]) => p.folder === 'collapsed/child')).toBe(false)
  kit.render(null, f.host)
})
it('R1.2 keeps the rows visible during a direct folder refresh', async () => {
  const f = fixture(); await f.model.folder(''); let release!: (value: any) => void
  f.request.mockImplementationOnce(() => new Promise((r) => { release = r }))
  const pending = f.model.folder('')
  try {
    kit.render(kit.h(f.module.Tree, { model: f.model, open: new Set(), toggle() {}, pick() {} }), f.host)
    expect(f.host.textContent).toContain('file.txt'); expect(f.host.textContent).not.toContain('Loading…')
  } finally { release({ result: list() }); await pending; kit.render(null, f.host) }
})
it('R1.3 increments the editor replacement counter only when accepting the current file', async () => {
  const f = fixture(); await f.model.open('file.txt'); const original = f.model.editorRevision
  f.model.edit('mine'); await f.model.save(); expect(f.model.editorRevision).toBe(original)
  f.model.conflict = text('current', 'outside'); f.model.takeCurrent()
  expect(f.model.editorRevision).toBe((original ?? 0) + 1)
})
for (const [path, anchor] of [
  ['index.html', '<link rel="stylesheet" href="css/files.css">'],
  ['js/ui.js', 'aria-checked=${it.checked ?? null} disabled=${it.disabled}'],
  ['js/editor.js', 'const p = parts.current;'],
  ['js/editor.js', 'readOnly = false, wrap = false, onChange, onSave'],
  ['js/editor.js', 'const lockOf = (ro) => [EditorState.readOnly.of(ro), EditorView.editable.of(!ro)];'],
  ['js/editor.js', '}, [readOnly]);'],
  ['js/app.js', 'liveGit.observe(patch);']
]) it(`R1.4 guards the ${path} replacement anchor`, () => {
  const source = readFileSync('design/mock/' + path, 'utf8')
  expect(() => filesSource(path!, source.replace(anchor!, 'removed'))).toThrow()
})
it('R1.4 rejects a duplicate anchor', () => {
  const source = readFileSync('design/mock/index.html', 'utf8'), anchor = '<link rel="stylesheet" href="css/files.css">'
  expect(() => filesSource('index.html', source.replace(anchor, anchor + anchor))).toThrow()
})
for (const declaration of ['const live = {};', 'let LiveFilesTab;', "import { live } from '../lib.js';", "import { FilesTab as LiveFilesTab } from './elsewhere.js';", 'const { live } = {};', 'let other, LiveFilesTab;', 'function live() {}', 'class LiveFilesTab {}'])
  it(`R1.5 refuses an existing binding: ${declaration}`, () => {
    const source = readFileSync('design/mock/js/screens/files.js', 'utf8')
    expect(() => filesSource('js/screens/files.js', declaration + '\n' + source)).toThrow(/already declares/)
  })
it('R1.6 opens no default file when the root has no README.md', async () => {
  const f = fixture(); f.render()
  try {
    await vi.waitFor(() => expect(f.host.querySelector('[role="tree"]')?.textContent).toContain('file.txt'))
    expect(f.host.textContent).toContain('No file open')
    expect(f.request.mock.calls.some(([tag]) => tag === 'project.files.read')).toBe(false)
  } finally { f.unmount() }
})
it('R1.7 offers a way to discard a draft when the current file was deleted outside', async () => {
  const f = fixture(); await f.model.open('file.txt'); f.model.edit('mine')
  f.request.mockRejectedValueOnce(new Error('the file changed since you opened it; reload it and save again'))
  f.request.mockRejectedValueOnce(new Error('Git: missing_file'))
  await f.model.save()
  expect(f.model.shown()).toBe('mine'); expect(f.model.currentUnavailable).toBe('Git: missing_file')
  expect(typeof f.model.discardDraft).toBe('function'); f.model.discardDraft()
  expect(f.model.drafts.has('file.txt')).toBe(false)
})
it('R1.7 renders Discard my text after the current file cannot be read', async () => {
  let deleted = false
  const f = fixture(async (tag, payload) => {
    if (tag === 'project.files.list') return { result: list(payload.folder) }
    if (tag === 'project.files.read') { if (deleted) throw new Error('Git: missing_file'); return { result: text() } }
    if (tag === 'project.files.write') { deleted = true; throw new Error('the file changed since you opened it; reload it and save again') }
    throw new Error(tag)
  })
  f.render('project', 'file.txt')
  try {
    await vi.waitFor(() => expect(f.host.querySelector('.test-editor')).not.toBeNull())
    f.edit('mine'); await f.save()
    await vi.waitFor(() => expect(f.host.textContent).toContain('Discard my text'))
    expect(f.host.querySelector('.test-editor')?.textContent).toBe('mine')
    ;[...f.host.querySelectorAll('button')].find((button) => button.textContent === 'Discard my text')!.click()
    await vi.waitFor(() => expect(f.host.querySelector('.test-editor')).toBeNull())
    expect(f.model.drafts.has('file.txt')).toBe(false)
  } finally { f.unmount() }
})
it('R1.8 updates the model project context when the mounted view changes projects', async () => {
  const f = fixture(); f.render('first', 'file.txt')
  try {
    await vi.waitFor(() => expect(f.host.querySelector('.test-editor')).not.toBeNull())
    f.render('second', 'file.txt')
    const input = f.host.querySelector('input')!; input.value = 'file'; input.dispatchEvent(new Event('input', { bubbles: true }))
    await vi.waitFor(() => expect(f.request).toHaveBeenCalledWith('project.context.search', expect.objectContaining({ projectId: 'second' })))
  } finally { f.unmount() }
})
it('R1.9 gives Generate a single hover explanation', () => {
  const f = fixture(); f.model.selected = 'file.txt'
  kit.render(kit.h(f.module.CommitFile, { model: f.model, w: f.w, close() {} }), f.host)
  expect(f.host.querySelectorAll('[title="Engine work needed"], [data-tip="Engine work needed"]')).toHaveLength(1)
  kit.render(null, f.host)
})
it('R1.9 gives a binary Download control a single hover explanation', async () => {
  const f = fixture(async (tag, payload) => ({ result: tag === 'project.files.list' ? list(payload.folder) : { kind: 'binary', path: 'binary.bin', size: 3 } }))
  f.render('project', 'binary.bin')
  try {
    await vi.waitFor(() => expect(f.host.textContent).toContain('Binary file'))
    expect(f.host.querySelectorAll('[title="Engine work needed"], [data-tip="Engine work needed"]')).toHaveLength(1)
  } finally { f.unmount() }
})
it('R1.9 gives the Download menu a single hover explanation', async () => {
  const source = filesSource('js/ui.js', readFileSync('design/mock/js/ui.js', 'utf8'))
  const start = source.indexOf('export function Menu('), end = source.indexOf('// ---- dialogs', start)
  const env = { ...kit, clamp: (x: number, a: number, b: number) => Math.max(a, Math.min(x, b)), Icon: () => null, cx: () => '' }
  const Menu = new Function(...Object.keys(env), source.slice(start, end).replace('export ', '') + '\nreturn Menu;')(...Object.values(env))
  const host = document.createElement('div'); document.body.append(host)
  try {
    kit.render(kit.h(Menu, { label: 'More', trigger: 'More', items: [{ label: 'Download', disabled: true, why: 'Engine work needed' }] }), host)
    host.querySelector('button')!.click()
    await vi.waitFor(() => expect(host.querySelector('[role="menuitem"]')).not.toBeNull())
    const item = host.querySelector('[role="menuitem"]')!
    expect(Number(item.hasAttribute('title')) + Number(item.hasAttribute('data-tip'))).toBe(1)
  } finally { kit.render(null, host) }
})
it('keeps the original Reading state when a cancelled open has no file', async () => {
  const cancelled = Object.assign(new DOMException('The read was cancelled.', 'AbortError'), { cancelledRead: true })
  const f = fixture(async (tag, p) => {
    if (tag === 'project.files.list') return { result: list(p.folder) }
    throw cancelled
  })
  try {
    f.render('project', 'cancelled.txt')
    await vi.waitFor(() => expect(f.request).toHaveBeenCalledWith('project.files.read', expect.anything()))
    await new Promise(resolve => setTimeout(resolve, 0))
    expect(f.host.textContent).toContain('Reading from')
    expect(f.host.querySelector('.fx-main')).not.toBeNull()
    expect(f.host.querySelector('[role="alert"]')).toBeNull()
    expect(f.host.querySelector('.test-editor')).toBeNull()
  } finally { f.unmount() }
})
