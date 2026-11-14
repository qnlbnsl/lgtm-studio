import { readFileSync } from 'node:fs'
import { expect, it, vi } from 'vitest'
import * as maps from '../src/files.ts'
import { filesSource } from '../scripts/filesSource.ts'
const text = (version = 'v1', body = 'opened\n', editable = true) => ({ kind: 'text', path: 'src/file.ts', version, text: body, editable, size: body.length })
const listing = { folder: '', items: [{ name: 'src', path: 'src', kind: 'folder', change: 'modified', size: null }], truncated: false }
function fixture(request = vi.fn(async (tag: string, p: any): Promise<any> => {
  if (tag === 'project.files.list') return { result: { ...listing, folder: p.folder || '' } }
  if (tag === 'project.files.read') return { result: text() }
  if (tag === 'project.context.search') return { result: { items: [{ reference: { _tag: 'file', path: 'src/file.ts' } }], truncated: true } }
  if (tag === 'project.files.write') return { result: { version: 'saved', told: 1 } }
  if (tag === 'project.commit') return { result: { head: 'abcdef123' } }
  throw new Error(tag)
})) {
  const source = readFileSync('design/mock/js/live/files.js', 'utf8').replace(/^import .*;$/gm, '').replace(/export /g, '')
  const factory = new Function('engine', source + '\nreturn { createFilesModel, setMapping };')({ request })
  factory.setMapping(maps)
  const context = { worktreeId: 'work', projectId: 'project', kind: 'project', machine: 'local', online: true }
  let valid = true
  const model = factory.createFilesModel(context, vi.fn(), () => valid)
  return { model, request, context, invalidate: () => { valid = false } }
}
it('maps one engine folder and all change marks without inventing child entries', () => {
  expect(maps.mapListing(listing as any)).toEqual({ ...listing, items: [{ ...listing.items[0], status: 'M' }] })
  expect(maps.searchPaths({ items: [{ reference: { _tag: 'task' } }, { reference: { _tag: 'file', path: 'a' } }], truncated: true } as any)).toEqual({ paths: ['a'], truncated: true })
})
it('selects the inspected worktree, project fallback, and base with a missing-branch notice', () => {
  const p = { id: 'p', repo: 'r', branch: 'topic' }, w = [{ id: 'base', repo: 'r', kind: 'base', branch: 'main' }, { id: 'project', repo: 'r', project: 'p', branch: 'topic', kind: 'project' }, { id: 'team', repo: 'r', project: 'p', branch: 'team', kind: 'team' }]
  expect(maps.worktreeFor({ worktrees: w, inspecting: { p: 'team' } } as any, p).w?.id).toBe('team')
  expect(maps.worktreeFor({ worktrees: w, inspecting: {} } as any, p).w?.id).toBe('project')
  expect(maps.worktreeFor({ worktrees: w.slice(0, 1), inspecting: {} } as any, p)).toEqual({ w: w[0], missing: 'topic' })
})
it('reads only folders requested, retains marks and exposes loading', async () => {
  const f = fixture(); const read = f.model.folder(''); expect(f.model.folders[''].loading).toBe(true); await read
  expect(f.request.mock.calls).toEqual([['project.files.list', { worktreeId: 'work' }]])
  expect(f.model.folders[''].items[0].status).toBe('M'); await f.model.folder('src'); expect(f.request.mock.calls).toHaveLength(2)
})
it('searches the selected project worktree and ignores older results', async () => {
  const f = fixture(); await f.model.search('file'); expect(f.model.found).toEqual({ paths: ['src/file.ts'], truncated: true })
  expect(f.request).toHaveBeenCalledWith('project.context.search', { projectId: 'project', worktreeId: 'work', query: 'file', scope: 'files', limit: 40 })
  await f.model.search(''); expect(f.model.found).toBeNull()
})
for (const kind of ['text', 'image', 'binary', 'large']) it(`opens the engine's ${kind} response`, async () => {
  const file = kind === 'text' ? text() : { kind, path: 'src/file.ts', size: 17, mediaType: 'image/png', data: 'YQ==' }
  const f = fixture(vi.fn(async () => ({ result: file }))); await f.model.open('src/file.ts'); expect(f.model.opened.file).toEqual(file)
})
it('keeps the opened version with a draft across navigation and waits for save acknowledgement', async () => {
  let release!: (v: any) => void; const f = fixture(); await f.model.open('src/file.ts'); f.model.edit('typed')
  await f.model.open('other'); await f.model.open('src/file.ts'); expect(f.model.shown()).toBe('typed')
  f.request.mockImplementationOnce(() => new Promise((r) => { release = r }))
  const pending = f.model.save(); expect(f.model.saving).toBe(true); expect(f.model.shown()).toBe('typed'); expect(f.model.dirty()).toBe(true)
  await f.model.save(); expect(f.request.mock.calls.filter(([t]: any) => t === 'project.files.write')).toHaveLength(1)
  expect(f.request).toHaveBeenCalledWith('project.files.write', { projectId: 'project', worktreeId: 'work', path: 'src/file.ts', version: 'v1', text: 'typed' })
  release({ result: { version: 'v2', told: 1 } }); await pending; expect(f.model.dirty()).toBe(false); expect(f.model.notice).toContain('Saved')
})
it('keeps typed text, displays the current version and saves over only that version', async () => {
  const f = fixture(); await f.model.open('src/file.ts'); f.model.edit('mine')
  f.request.mockImplementationOnce(async () => { throw new Error('the file changed since you opened it; reload it and save again') })
  f.request.mockImplementationOnce(async () => ({ result: text('current', 'theirs') }))
  await f.model.save(); expect(f.model.conflict.text).toBe('theirs'); expect(f.model.shown()).toBe('mine')
  await f.model.save(f.model.conflict.version); expect(f.request).toHaveBeenCalledWith('project.files.write', expect.objectContaining({ version: 'current', text: 'mine' }))
})
it('can use the current file and clears only its draft', async () => {
  const f = fixture(); await f.model.open('src/file.ts'); f.model.edit('mine'); f.model.conflict = text('current', 'theirs'); f.model.takeCurrent()
  expect(f.model.shown()).toBe('theirs'); expect(f.model.dirty()).toBe(false)
})
it('reports save and commit refusals without losing the draft or closing the dialog', async () => {
  const f = fixture(); await f.model.open('src/file.ts'); f.model.edit('mine'); f.request.mockRejectedValueOnce(new Error('disk is full')); await f.model.save()
  expect(f.model.notice).toBe('disk is full'); expect(f.model.shown()).toBe('mine'); expect(f.model.dirty()).toBe(true)
  f.model.takeCurrent(); f.model.drafts.clear(); f.request.mockRejectedValueOnce(new Error('Git: conflicts')); expect(await f.model.commit('message')).toBe(false); expect(f.model.notice).toBe('Git: conflicts')
})
it('commits only the selected file and keeps a pending control', async () => {
  const f = fixture(); await f.model.open('src/file.ts'); const pending = f.model.commit('One file'); expect(f.model.committing).toBe(true)
  expect(await pending).toBe(true); expect(f.request).toHaveBeenCalledWith('project.commit', { worktreeId: 'work', projectId: 'project', path: 'src/file.ts', message: 'One file' }); expect(f.model.committing).toBe(false)
})
for (const reason of ['base', 'redacted', 'remote', 'offline']) it(`blocks mutations for ${reason} and explains why`, async () => {
  const f = fixture(); await f.model.open('src/file.ts'); f.model.edit('mine')
  if (reason === 'base') f.context.kind = 'base'; if (reason === 'redacted') f.model.opened.file.editable = false
  if (reason === 'remote') f.context.machine = 'other'; if (reason === 'offline') f.context.online = false
  expect(f.model.readOnlyReason()).toBeTruthy(); await f.model.save(); await f.model.commit('No'); expect(f.request.mock.calls.some(([t]: any) => ['project.files.write', 'project.commit'].includes(t))).toBe(false)
})
it('does not publish a held reply after the screen or session ends', async () => {
  let release!: (v: any) => void; const f = fixture(vi.fn(() => new Promise((r) => { release = r }))); const pending = f.model.open('src/file.ts'); f.invalidate(); release({ result: text() }); await pending; expect(f.model.opened?.file).toBeUndefined()
})
it('guards the live replacements, preserves sample code and wires the style nonce', () => {
  const screen = readFileSync('design/mock/js/screens/files.js', 'utf8'), editor = readFileSync('design/mock/js/editor.js', 'utf8'), app = readFileSync('design/mock/js/app.js', 'utf8')
  expect(filesSource('js/screens/files.js', screen)).toContain('LiveFilesTab')
  expect(filesSource('js/editor.js', editor)).toContain('EditorView.cspNonce.of(nonce)')
  expect(filesSource('js/app.js', app)).not.toContain("['terminal', 'files'].includes(tab)")
  for (const [path, input] of [['js/screens/files.js', screen], ['js/editor.js', editor], ['js/app.js', app]]) expect(() => filesSource(path!, input!.replaceAll(path === 'js/editor.js' ? 'lineNumbers(),' : path === 'js/app.js' ? "['terminal', 'files'].includes(tab)" : 'export function FilesTab({ s, project, route }) {', 'gone'))).toThrow()
  expect(screen).toContain('saveFile(wid, sel)')
})

it('does not let an older folder read overwrite the latest listing', async () => {
  let first!: (v: any) => void; const f = fixture(); f.request.mockImplementationOnce(() => new Promise((r) => { first = r }))
  const old = f.model.folder(''); await f.model.folder(''); first({ result: { ...listing, items: [] } }); await old
  expect(f.model.folders[''].items[0].path).toBe('src')
})
it('keeps the newest selected file when an older open completes later', async () => {
  let first!: (v: any) => void; const f = fixture(); f.request.mockImplementationOnce(() => new Promise((r) => { first = r }))
  const old = f.model.open('old'); await f.model.open('new'); first({ result: text('old', 'old text') }); await old
  expect(f.model.selected).toBe('new'); expect(f.model.shown()).toBe('opened\n')
})
it('ignores a search response for the previous query', async () => {
  let first!: (v: any) => void; const f = fixture(); f.request.mockImplementationOnce(() => new Promise((r) => { first = r }))
  const old = f.model.search('old'); await f.model.search('new'); first({ result: { items: [], truncated: false } }); await old
  expect(f.model.found.paths).toEqual(['src/file.ts'])
})
it('does not replace the version protecting a retained draft with a fresh read', async () => {
  const f = fixture(); await f.model.open('src/file.ts'); f.model.edit('mine'); await f.model.open('other')
  f.request.mockResolvedValueOnce({ result: text('v2', 'outside change') }); await f.model.open('src/file.ts'); await f.model.save()
  expect(f.request).toHaveBeenCalledWith('project.files.write', expect.objectContaining({ text: 'mine', version: 'v1' }))
})
it('keeps the draft while refusing to save over current private data', async () => {
  const f = fixture(); await f.model.open('src/file.ts'); f.model.edit('mine'); f.model.conflict = text('v2', '[redacted]', false)
  await f.model.save('v2'); expect(f.model.shown()).toBe('mine'); expect(f.model.readOnlyReason()).toContain('private data')
  expect(f.request.mock.calls.some(([tag]: any) => tag === 'project.files.write')).toBe(false)
})
