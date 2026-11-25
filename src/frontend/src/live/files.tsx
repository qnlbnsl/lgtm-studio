import { errorText } from './reads';
// The engine Files screen. The standalone sample continues to use screens/files.js.
import { useState, useEffect, useMemo, useRef } from 'preact/hooks';
import { Icon, cx, go, href, useMedia, plural, isMac, store } from '../lib';
import { machineOf, agentOf } from '../data';
import * as engine from '../engine';
import { Menu, Dialog, fileSize } from '../ui';
import { CodeEditor } from '../editor';
import type { Id, State, Project, Route, Agent } from '../types';
import type { Worktree } from '../screens/git';

// ---- the shapes this module reads -------------------------------------------------
// The engine's file records, as packages/contracts/src/project/git.ts defines them.
type FileRecord =
  | { kind: 'text'; path: string; size: number; version: string; text: string; editable: boolean }
  | { kind: 'image'; path: string; size: number; version: string; mediaType: string; data: string }
  | { kind: 'binary' | 'large'; path: string; size: number; version?: undefined };
type TextFile = Extract<FileRecord, { kind: 'text' }>;
/** A folder's entries, as packages/studio/src/files.ts maps them, or the read in progress. */
interface FolderEntry { name: string; path: string; kind: 'folder' | 'file' | string; size?: number | null; change?: string | null; status?: string | null }
interface FolderState { folder?: string; items?: FolderEntry[]; truncated?: boolean; loading?: boolean; error?: string }
interface FoundPaths { paths: string[]; truncated?: boolean; error?: string }
interface FilesContext { worktreeId: Id; projectId: Id; kind: string; machine: Id; online: boolean }
/** Unsaved text, with the version of the file it was written against. */
interface Draft { text: string; file: TextFile }
interface Opened { loading?: boolean; file?: FileRecord; error?: string }
// The typed mappings from packages/studio that this module uses.
interface FilesMaps {
  mapListing(listing: unknown): FolderState;
  searchPaths(result: unknown): FoundPaths;
  readOnlyReason(context: FilesContext, file?: FileRecord | null): string;
  worktreeFor(state: State, project: Project): { w: Worktree | undefined; missing: string | null };
}
export interface FilesModel {
  context: FilesContext; folders: Record<string, FolderState>; openFolders: Set<string>; drafts: Map<string | null, Draft>;
  selected: string | null; opened: Opened | null; found: FoundPaths | null;
  editorRevision: number; currentUnavailable: string | null;
  searching: boolean; saving: boolean; committing: boolean; conflict: FileRecord | null; notice: string;
  shown(): string; dirty(): boolean; readOnlyReason(): string; edit(text: string): void;
  folder(folder: string): Promise<void>; search(query: string): Promise<void>; open(path: string | null): Promise<void>;
  takeCurrent(): void; discardDraft(): void; save(version?: string): Promise<void>;
  refreshFolders(path?: string): Promise<void>; refresh(): Promise<void>; close(): void; commit(message: string): Promise<boolean>;
}

let mapping: FilesMaps | undefined;
export function setMapping(value: FilesMaps): void { mapping = value; }
// Engine replies are loose records; each caller reads the fields it needs.
const result = (reply: { result?: unknown } | null | undefined): any => {
  if (!reply || !('result' in reply)) throw new Error('The service returned no usable file record.');
  return reply.result;
};
const draftsByWorktree = new Map<Id, Map<string | null, Draft>>();
export function observe(patch: Partial<State>): void {
  if (!(['out', 'expired', 'rejected', 'logout-unconfirmed'] as (string | undefined)[]).includes(patch.session?.status)) return;
  for (const drafts of draftsByWorktree.values()) drafts.clear();
  draftsByWorktree.clear();
}
const draftsFor = (id: Id): Map<string | null, Draft> => {
  if (!draftsByWorktree.has(id)) draftsByWorktree.set(id, new Map());
  return draftsByWorktree.get(id)!;
};
// Drafts outlive mounted views and retain the version that the owner opened.
export function createFilesModel(context: FilesContext, changed: () => void, valid: () => boolean = () => true): FilesModel {
  let opening = 0, searching = 0, closed = false;
  let refreshing: Promise<void> | null = null, requested = false, lastRefresh = -Infinity, timer: ReturnType<typeof setTimeout> | undefined, wake: (() => void) | undefined;
  const active = () => !closed && valid();
  const m: FilesModel = {
    context, folders: {}, openFolders: new Set(), drafts: draftsFor(context.worktreeId), selected: null, opened: null, found: null,
    editorRevision: 0, currentUnavailable: null,
    searching: false, saving: false, committing: false, conflict: null, notice: '',
    shown() { return m.drafts.get(m.selected)?.text ?? (m.opened?.file?.kind === 'text' ? m.opened.file.text : ''); },
    dirty() { return m.drafts.has(m.selected); },
    readOnlyReason() { return mapping!.readOnlyReason(context, m.conflict || m.opened?.file); },
    edit(text) {
      const file = m.opened?.file;
      if (m.readOnlyReason() || file?.kind !== 'text' || m.saving) return;
      if (text === file.text) m.drafts.delete(m.selected);
      else m.drafts.set(m.selected, { text, file });
      changed();
    },
    async folder(folder) {
      if (!context.online || context.machine !== 'local' || !active()) return;
      const slot: FolderState = { ...(m.folders[folder] || {}), loading: true };
      m.folders[folder] = slot; changed();
      try {
        const listing = mapping!.mapListing(result(await engine.request('project.files.list', { worktreeId: context.worktreeId, ...(folder ? { folder } : {}) })));
        if (active() && m.folders[folder] === slot) m.folders[folder] = listing;
      } catch (e) { if (active() && m.folders[folder] === slot) m.folders[folder] = { ...slot, loading: false, error: errorText(e) }; }
      finally { if (active()) changed(); }
    },
    async search(query) {
      const turn = ++searching;
      m.found = null; m.searching = !!query.trim(); changed();
      if (!query.trim() || !context.online || context.machine !== 'local') return;
      try {
        const reply = await engine.request('project.context.search', { projectId: context.projectId, worktreeId: context.worktreeId, query, scope: 'files', limit: 40 });
        if (active() && turn === searching) m.found = mapping!.searchPaths(result(reply));
      } catch (e) { if (active() && turn === searching) { m.found = { paths: [], error: errorText(e) }; m.notice = errorText(e); } }
      finally { if (active() && turn === searching) { m.searching = false; changed(); } }
    },
    async open(path) {
      const turn = ++opening;
      m.selected = path; m.conflict = null; m.currentUnavailable = null; m.notice = ''; m.opened = path ? { loading: true } : null; changed();
      if (!path || !context.online || context.machine !== 'local') return;
      try {
        const file = result(await engine.request('project.files.read', { worktreeId: context.worktreeId, path }));
        if (active() && turn === opening) {
          // Retain the original version until a save can check it. A fresh read must
          // never silently authorize replacing somebody else's change with a draft.
          const draft = m.drafts.get(path);
          m.opened = { file: draft?.file || file };
        }
      } catch (e) {
        if (active() && turn === opening) {
          const draft = m.drafts.get(path);
          m.opened = draft ? { file: draft.file } : { error: errorText(e) };
          if (draft) m.currentUnavailable = errorText(e);
        }
      }
      finally { if (active() && turn === opening) changed(); }
    },
    takeCurrent() {
      if (!m.conflict || m.saving) return;
      m.drafts.delete(m.selected); m.opened = { file: m.conflict }; m.conflict = null; m.currentUnavailable = null; m.editorRevision++; m.notice = ''; changed();
    },
    discardDraft() {
      if (m.saving || !m.currentUnavailable) return;
      m.drafts.delete(m.selected); m.opened = { error: m.currentUnavailable };
      m.currentUnavailable = null; m.conflict = null; m.notice = ''; changed();
    },
    async save(version) {
      if (m.saving || m.committing || !m.dirty() || m.readOnlyReason() || !active()) return;
      const path = m.selected!, file = m.opened!.file as TextFile, text = m.shown(), draft = m.drafts.get(path);
      m.saving = true; m.notice = ''; changed();
      try {
        const saved = result(await engine.request('project.files.write', { projectId: context.projectId, worktreeId: context.worktreeId, path, version: version || file.version, text }));
        if (!active()) return;
        if (m.drafts.get(path) === draft) m.drafts.delete(path);
        if (m.selected === path) { m.opened = { file: { ...file, text, version: saved.version, size: new TextEncoder().encode(text).length } }; m.conflict = null; m.currentUnavailable = null; }
        m.notice = `Saved ${path.split('/').pop()}.${saved.told ? ` ${saved.told} working agent${saved.told === 1 ? ' was' : 's were'} told which file you changed.` : ''}`;
        await m.refreshFolders(path);
      } catch (e) {
        if (!active()) return;
        m.notice = errorText(e);
        if ((e as Error).message?.startsWith('the file changed since you opened it')) {
          try {
            const current = result(await engine.request('project.files.read', { worktreeId: context.worktreeId, path }));
            if (active() && m.selected === path) {
              m.conflict = current;
              m.notice = 'The file changed since you opened it. Your text is kept. Compare it with the current file below.';
            }
          } catch (readError) { if (active() && m.selected === path) { m.currentUnavailable = errorText(readError); m.notice = errorText(readError); } }
        }
      } finally { m.saving = false; if (active()) changed(); }
    },
    // A save or a commit waits for one direct listing of the file's own folder.
    // The throttled refresh of every open folder runs in the background, so
    // pushed events cannot keep the editor busy (issue 928).
    refreshFolders(path = '') {
      void m.refresh();
      return Promise.resolve(m.folder(path.includes('/') ? path.slice(0, path.lastIndexOf('/')) : '')).catch(() => {});
    },
    refresh() {
      if (!active() || !context.online || context.machine !== 'local') return Promise.resolve();
      requested = true;
      if (refreshing) return refreshing;
      refreshing = (async () => {
        while (requested && active() && context.online && context.machine === 'local') {
          requested = false;
          const delay = Math.max(0, lastRefresh + 1000 - Date.now());
          if (delay) await new Promise<void>((resolve) => { wake = resolve; timer = setTimeout(resolve, delay); });
          // Events during the interval belong to this trailing refresh.
          requested = false;
          if (!active() || !context.online || context.machine !== 'local') break;
          lastRefresh = Date.now();
          const visible = [...m.openFolders].filter((path) => {
            const parts = path.split('/');
            return parts.every((_, i) => i === 0 || m.openFolders.has(parts.slice(0, i).join('/')));
          });
          await Promise.all(['', ...visible].map((folder) => m.folder(folder)));
        }
      })().finally(() => { refreshing = null; });
      return refreshing;
    },
    close() { closed = true; clearTimeout(timer); wake?.(); },
    async commit(message) {
      if (m.committing || m.saving || m.dirty() || m.readOnlyReason() || !m.selected || !message.trim() || !active()) return false;
      const path = m.selected;
      m.committing = true; m.notice = ''; changed();
      try {
        const saved = result(await engine.request('project.commit', { worktreeId: context.worktreeId, projectId: context.projectId, path, message: message.trim() }));
        if (!active()) return false;
        m.notice = `Committed ${path.split('/').pop()} as ${saved.head.slice(0, 7)}.`;
        await m.refreshFolders(path); return true;
      } catch (e) { if (active()) m.notice = errorText(e); return false; }
      finally { m.committing = false; if (active()) changed(); }
    },
  };
  return m;
}

const STATUS: Record<string, [string, string]> = { A: ['add', 'Added'], M: ['mod', 'Changed'], D: ['mod', 'Deleted'], R: ['mod', 'Renamed'], U: ['mod', 'Conflict'] };
const extOf = (p: string) => (p.match(/\.([a-z0-9]+)$/i)?.[1] || '').toLowerCase();
const ICON: Record<string, string> = { md: 'file-text', css: 'palette', svg: 'image', png: 'image', zip: 'archive', ts: 'file-code', tsx: 'file-code' };
const ltr = (t: string | undefined) => '\u200e' + (t || '');
interface TreeProps { model: FilesModel; folder?: string; depth?: number; open: Set<string>; toggle: (path: string) => void; pick: (path: string) => void }
function Tree({ model, folder = '', depth = 0, open, toggle, pick }: TreeProps) {
  const state = model.folders[folder];
  if (!state || (state.loading && !state.items)) return <li class="fx-note" role="status">Loading…</li>;
  const error = state.error && <li class="fx-note" role="alert">{state.error} <button class="btn sm" onClick={() => model.folder(folder)}>Retry</button></li>;
  return <>
    {error}
    {(state.items || []).map((entry) => entry.kind === 'folder' ? <li key={entry.path} role="treeitem" aria-label={entry.name} aria-expanded={open.has(entry.path)}>
    <button type="button" class="fx-row dir" aria-label={entry.name} style={`--d:${depth}`} onClick={() => toggle(entry.path)}>
      <Icon name="chevron-right" size={13} class="fx-chev" />
      <Icon name="folder" size={14} class="faint" />
      <span class="truncate">{entry.name}</span>
      {entry.change && <i class="fx-dot" title="Has changes" />}
    </button>
    {open.has(entry.path) && <ul role="group"><Tree model={model} folder={entry.path} depth={depth + 1} open={open} toggle={toggle} pick={pick} /></ul>}
  </li>
    : <li key={entry.path} role="treeitem" aria-label={entry.name} aria-selected={model.selected === entry.path}>
      <button type="button" class={cx('fx-row', model.selected === entry.path && 'on')} aria-label={entry.name} style={`--d:${depth}`} onClick={() => pick(entry.path)}>
        <span class="fx-chev" />
        <Icon name={ICON[extOf(entry.path)] || 'file'} size={14} class="faint" />
        <span class="truncate">{entry.name}</span>
        {model.drafts.has(entry.path) && <i class="fx-unsaved" title="Not saved" />}
        {entry.status && <span class={cx('fx-st', STATUS[entry.status][0])} title={STATUS[entry.status][1]}>{entry.status}</span>}
      </button>
    </li>)}
    {state.items && !state.items.length && <li class="fx-note">Empty</li>}
    {state.truncated && <li class="fx-note">Some entries are not shown.</li>}
  </>;
}
function CommitFile({ model, w, close }: { model: FilesModel; w: Worktree; close: () => void }) {
  const [message, setMessage] = useState('');
  const submit = async () => { if (await model.commit(message)) close(); };
  const disabled = model.committing || !!model.readOnlyReason();
  return <Dialog title={'Commit ' + model.selected!.split('/').pop()} size="narrow" onClose={() => { if (!model.committing) close(); }} onSubmit={submit} foot={<>
      <button type="button" class="btn" disabled={model.committing} onClick={close}>Cancel</button>
      <button type="submit" class="btn primary" disabled={disabled || !message.trim()}>
        <Icon name="check" size={14} />
        {model.committing ? 'Committing…' : 'Commit'}
      </button>
    </>}>
    <div class="field">
      <div class="row">
        <label for="fx-live-commit-msg" class="grow">Commit message</label>
        <span>
          <button type="button" class="btn sm ghost" disabled title="Engine work needed"><Icon name="sparkles" size={13} />Generate</button>
        </span>
      </div>
      <textarea id="fx-live-commit-msg" class="textarea" rows={3} autofocus value={message} disabled={disabled} maxLength={8192} onInput={(e) => setMessage((e.target as HTMLTextAreaElement).value)} onKeyDown={(e) => { if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) { e.preventDefault(); void submit(); } }} />
    </div>
    <p class="muted fx-commit-note">
      {'Only '}
      <span class="mono">{model.selected}</span>
      {' is committed, to '}
      <span class="mono">{w.branch}</span>
      , as you. Other staged files stay staged. For anything bigger, ask the project lead.
    </p>
    {model.notice && <p role="status">{model.notice}</p>}
  </Dialog>;
}
// The parent selects the inspected worktree. A changed worktree gets a fresh model.
interface TabProps { s: State; project: Project; route: Route }
export function LiveFilesTab({ s, project, route }: TabProps) {
  const { w, missing } = mapping!.worktreeFor(s, project);
  if (!w) return <div class="fx-state" data-empty><strong>No worktree</strong></div>;
  return <WorktreeFiles key={w.id} s={s} project={project} route={route} w={w} missing={missing} />;
}
function WorktreeFiles({ s, project, route, w, missing }: TabProps & { w: Worktree; missing: string | null }) {
  const phone = useMedia('(max-width: 860px)'), machine = machineOf(s, w.machine);
  const offline = !machine || machine.status !== 'online' || s.conn.status !== 'live';
  const remote = w.machine !== 'local';
  const [, bump] = useState(0), alive = useRef(true);
  const context = useMemo(() => ({ worktreeId: w.id, projectId: project.id, kind: w.kind, machine: w.machine, online: !offline }), [w.id]);
  Object.assign(context, { projectId: project.id, online: !offline, kind: w.kind, machine: w.machine });
  const model = useMemo(() => createFilesModel(context, () => bump((n) => n + 1), () => alive.current && store.state === s && s.session.status === 'in'), [w.id]);
  const [open, setOpen] = useState(() => new Set<string>()), [query, setQuery] = useState(''), [wrapPref, setWrap] = useState<boolean | null>(null), [committing, setCommitting] = useState(false);
  const wrap = wrapPref ?? phone, sel = route.query.get('f') || (!phone && model.folders['']?.items?.some((entry) => entry.path === 'README.md' && entry.kind === 'file') ? 'README.md' : null);
  model.openFolders = open;
  useEffect(() => { alive.current = true; return () => { alive.current = false; model.close(); }; }, []);
  useEffect(() => {
    if (offline || remote) return;
    void model.refresh();
  }, [offline, remote, s.boardRevision]);
  useEffect(() => {
    const refresh = () => { void model.refresh(); };
    addEventListener('focus', refresh); return () => removeEventListener('focus', refresh);
  }, [model]);
  useEffect(() => {
    void model.open(sel);
    if (!sel || offline || remote) return;
    const parents = sel.split('/').slice(0, -1).map((_, i, a) => a.slice(0, i + 1).join('/'));
    setOpen((o) => new Set([...o, ...parents]));
    for (const folder of parents) if (!model.folders[folder]) void model.folder(folder);
  }, [sel, offline, remote]);
  useEffect(() => { const timer = setTimeout(() => model.search(query), 150); return () => clearTimeout(timer); }, [query, offline]);
  const pick = (path: string) => { if (!model.saving && !model.committing) go(`/p/${project.id}/files?f=${encodeURIComponent(path)}`); };
  const toggle = (path: string) => { if (!open.has(path)) void model.folder(path); setOpen((o) => { const next = new Set(o); next.has(path) ? next.delete(path) : next.add(path); return next; }); };
  const file = model.opened?.file, textFile = file?.kind === 'text', shown = model.shown(), dirty = model.dirty(), reason = model.readOnlyReason();
  const lines = textFile ? shown.replace(/\n$/, '').split('\n').length : 0;
  const change = Object.values(model.folders).flatMap((f) => f.items || []).find((f) => f.path === sel)?.status;
  const worker = (w.owner && agentOf(s, w.owner)) as Agent | null;
  const image = useMemo(() => {
    if (file?.kind !== 'image') return null;
    const bytes = Uint8Array.from(atob(file.data), (c) => c.charCodeAt(0));
    return URL.createObjectURL(new Blob([bytes], { type: file.mediaType }));
  }, [file]);
  useEffect(() => () => { if (image) URL.revokeObjectURL(image); }, [image]);
  const side = <aside class="fx-side" aria-label="Files">
    <div class="fx-where">
      <div class="row" style="gap:6px;min-width:0">
        <Icon name="git-branch" size={13} class="faint" />
        <span class="mono truncate-start" title={w.branch}>{ltr(w.branch)}</span>
      </div>
      <div class="fx-mach">
        <i class={cx('dot', offline ? 'failed' : 'working')} />
        <span>{machine?.name || 'No machine'}</span>
        <span class="mono truncate-start faint" title={w.folder}>{ltr(w.folder)}</span>
      </div>
    </div>
    <div class="fx-find search">
      <Icon name="search" size={13} />
      <input class="input" placeholder="Go to file" aria-label="Go to file" value={query} maxLength={256} disabled={offline || remote || model.saving || model.committing} onInput={(e) => setQuery((e.target as HTMLInputElement).value)} />
    </div>
    {missing && <p class="fx-note"><span class="mono">{missing}</span> has no worktree yet. This is <span class="mono">{w.branch}</span>.</p>}
    <div class="fx-tree" data-main>
      {!offline && !remote && (query.trim() ? <ul class="fx-found">
      {model.searching || !model.found ? <li class="fx-note" role="status">Searching…</li> : <>
      {model.found.paths.map((path) => <li key={path}>
      <button type="button" class={cx('fx-row', sel === path && 'on')} aria-label={path} onClick={() => { pick(path); setQuery(''); }}>
        <Icon name={ICON[extOf(path)] || 'file'} size={14} class="faint" />
        <span class="truncate-start mono" title={path}>{ltr(path)}</span>
      </button>
    </li>)}
      {!model.found.paths.length && <li class="fx-none">{model.found.error || 'No file matches'}</li>}
      {model.found.truncated && <li class="fx-note">More files match. Type more of the name.</li>}
    </>}
    </ul>
    : <ul role="tree" aria-label={'Files in ' + w.branch}><Tree model={model} open={open} toggle={toggle} pick={pick} /></ul>)}
    </div>
  </aside>;
  const crumbs = sel?.split('/') || [];
  const head = <div class="fx-head">
    {phone && <a class="icon-btn" href={href(`/p/${project.id}/files`)} aria-label="Back to the files"><Icon name="arrow-left" size={16} /></a>}
    <div class="fx-crumbs mono" title={sel!}>
      {crumbs.length > 1 && <><span class="fx-dir muted">{ltr(crumbs.slice(0, -1).join(' / '))}</span><span class="faint">/</span></>}
      <b>{crumbs.at(-1)}</b>
    </div>
    {change && <span class={cx('fx-st', STATUS[change][0])}>{STATUS[change][1]}</span>}
    <span class="fx-gap" />
    {file && <span class="fx-size muted">{lines ? plural(lines, 'line') + ' · ' : ''}{fileSize(file.size)}</span>}
    {textFile && reason && <span class="fx-ro muted" title={reason}><Icon name="lock" size={12} />Read-only</span>}
    {change && <a class="btn sm" href={href(`/p/${project.id}/git`)} aria-label="Open in Git" title="Open in Git">
      <Icon name="git-branch" size={13} />
      <span class="fx-lbl">Open in Git</span>
    </a>}
    {textFile && !reason && <button type="button" class={cx('btn sm', dirty && 'primary')} disabled={!dirty || model.saving || model.committing} onClick={() => model.save()} data-tip={isMac ? '⌘S' : 'Ctrl+S'}>
      {model.saving ? 'Saving…' : 'Save'}
    </button>}
    {textFile && !reason && change && !dirty && <button type="button" class="btn sm primary" aria-label="Commit" disabled={model.saving || model.committing} onClick={() => setCommitting(true)}>
      <Icon name="git-commit-horizontal" size={13} />
      <span class="fx-lbl">Commit</span>
    </button>}
    <Menu class="icon-btn" align="end" label="More" trigger={<Icon name="ellipsis" size={16} />} width={220} items={[...(lines ? [{ label: 'Wrap long lines', checked: wrap, onSelect: () => setWrap(!wrap) }, { sep: true }] : []), { label: 'Copy path', icon: 'copy', onSelect: () => navigator.clipboard?.writeText(sel!).catch(() => {}) }, { label: 'Download', disabled: true, why: 'Engine work needed' }]} />
  </div>;
  let body;
  if (offline) body = <div class="fx-state" data-empty>
    <Icon name="monitor" size={20} class="faint" />
    <strong>{machine?.name || 'The machine'} is offline</strong>
    <span>Its files show again when it reconnects.</span>
  </div>;
  else if (remote) body = <div class="fx-state" data-empty>
    <Icon name="monitor" size={20} class="faint" />
    <strong>Files on this machine are awaiting machine support.</strong>
    <span>Only worktrees on this computer can be read today.</span>
  </div>;
  else if (!sel) body = <div class="fx-state" data-empty><Icon name="file" size={20} class="faint" /><strong>No file open</strong></div>;
  else if (model.opened?.loading || !model.opened || !file && !model.opened.error) body = <div class="fx-state" role="status">
    <Icon name="loader-circle" size={18} class="spin faint" />
    <span>Reading from {machine.name}…</span>
  </div>;
  else if (model.opened.error) body = <div class="fx-state" role="alert">
    <strong>{model.opened.error}</strong>
    <span class="mono">{sel}</span>
    <button class="btn" onClick={() => model.open(sel)}>Retry</button>
  </div>;
  else if (file!.kind === 'image') body = <div class="fx-pic"><img src={image!} alt={sel} /></div>;
  else if (file!.kind === 'binary' || file!.kind === 'large') body = <div class="fx-state">
    <Icon name={file!.kind === 'binary' ? 'archive' : 'file-text'} size={20} class="faint" />
    <strong>{file!.kind === 'binary' ? 'Binary file' : 'Too large to show'} · {fileSize(file!.size)}</strong>
    <span>{file!.kind === 'binary' ? 'It cannot be read here.' : 'Files over 512 KB are not shown here.'}</span>
    <span><button type="button" class="btn" disabled title="Engine work needed">Download</button></span>
  </div>;
  else body = <>
    {worker?.state === 'working' && dirty && <p class="fx-busy">
    <Icon name="info" size={13} />
    {worker.name}
    {' is working in this worktree. When you save, it is told which file you changed.'}
  </p>}
    {reason && <p class="fx-note" role="note">Read-only. {reason}</p>}
    <CodeEditor key={w.id + ':' + sel + ':' + model.editorRevision} doc={shown} path={sel} wrap={wrap} readOnly={!!reason} {...{ busy: model.saving || model.committing }} onChange={(text) => model.edit(text)} onSave={() => model.save()} />
    {model.currentUnavailable && <section class="fx-note" role="alert">
      <p>{model.currentUnavailable} Your text is kept.</p>
      <button type="button" class="btn sm" disabled={model.saving} onClick={() => model.discardDraft()}>Discard my text</button>
    </section>}
    {model.conflict && <section class="fx-note fx-current" aria-label="The file as it is now">
      <strong>The file as it is now</strong>
      <div class="row">
        <button type="button" class="btn sm" disabled={model.saving} onClick={() => model.takeCurrent()}>Use the current file</button>
        <button type="button" class="btn sm" disabled={!!reason || model.saving || model.conflict.kind !== 'text'} onClick={() => model.save(model.conflict!.version)}>
          Save my text over it
        </button>
      </div>
      <pre>{model.conflict.kind === 'text' ? model.conflict.text : 'The current file is ' + model.conflict.kind + '.'}</pre>
    </section>}
  </>;
  const main = <section class="fx-main" aria-label="Open file">
    {sel && head}
    {model.notice && <p class="fx-note" role="status">{model.notice}</p>}
    {body}
  </section>;
  return <div class={cx('fx', phone && 'phone')} aria-busy={model.saving || model.committing}>
    {phone ? sel ? main : side : <>{side}{main}</>}
    {committing && <CommitFile model={model} w={w} close={() => setCommitting(false)} />}
  </div>;
}
