// The engine Files screen. The standalone sample continues to use screens/files.js.
import { html, useState, useEffect, useMemo, useRef, Icon, cx, go, href, useMedia, plural, isMac, store } from '../lib.js';
import { machineOf, agentOf } from '../data.js';
import * as engine from '../engine.js';
import { Menu, Dialog, fileSize } from '../ui.js';
import { CodeEditor } from '../editor.js';
let mapping;
export function setMapping(value) { mapping = value; }
const result = (reply) => {
  if (!reply || !('result' in reply)) throw new Error('The service returned no usable file record.');
  return reply.result;
};
const parentOf = (path) => path.split('/').slice(0, -1).join('/');
// A model belongs to one mounted worktree and signed-in session. Drafts retain their
// opened version, including when the owner navigates away and returns to a file.
export function createFilesModel(context, changed, valid = () => true) {
  let opening = 0, searching = 0;
  const m = {
    context, folders: {}, drafts: new Map(), selected: null, opened: null, found: null,
    searching: false, saving: false, committing: false, conflict: null, notice: '',
    shown() { return m.drafts.get(m.selected)?.text ?? (m.opened?.file?.kind === 'text' ? m.opened.file.text : ''); },
    dirty() { return m.drafts.has(m.selected); },
    readOnlyReason() { return mapping.readOnlyReason(context, m.conflict || m.opened?.file); },
    edit(text) {
      const file = m.opened?.file;
      if (m.readOnlyReason() || file?.kind !== 'text' || m.saving) return;
      if (text === file.text) m.drafts.delete(m.selected);
      else m.drafts.set(m.selected, { text, file });
      changed();
    },
    async folder(folder) {
      if (!context.online || context.machine !== 'local' || !valid()) return;
      const slot = { ...(m.folders[folder] || {}), loading: true };
      m.folders[folder] = slot; changed();
      try {
        const listing = mapping.mapListing(result(await engine.request('project.files.list', { worktreeId: context.worktreeId, ...(folder ? { folder } : {}) })));
        if (valid() && m.folders[folder] === slot) m.folders[folder] = listing;
      } catch (e) { if (valid() && m.folders[folder] === slot) m.folders[folder] = { error: e.message }; }
      finally { if (valid()) changed(); }
    },
    async search(query) {
      const turn = ++searching;
      m.found = null; m.searching = !!query.trim(); changed();
      if (!query.trim() || !context.online || context.machine !== 'local') return;
      try {
        const reply = await engine.request('project.context.search', { projectId: context.projectId, worktreeId: context.worktreeId, query, scope: 'files', limit: 40 });
        if (valid() && turn === searching) m.found = mapping.searchPaths(result(reply));
      } catch (e) { if (valid() && turn === searching) { m.found = { paths: [], error: e.message }; m.notice = e.message; } }
      finally { if (valid() && turn === searching) { m.searching = false; changed(); } }
    },
    async open(path) {
      const turn = ++opening;
      m.selected = path; m.conflict = null; m.notice = ''; m.opened = path ? { loading: true } : null; changed();
      if (!path || !context.online || context.machine !== 'local') return;
      try {
        const file = result(await engine.request('project.files.read', { worktreeId: context.worktreeId, path }));
        if (valid() && turn === opening) {
          // Retain the original version until a save can check it. A fresh read must
          // never silently authorize replacing somebody else's change with a draft.
          const draft = m.drafts.get(path);
          m.opened = { file: draft?.file || file };
        }
      } catch (e) { if (valid() && turn === opening) m.opened = { error: e.message }; }
      finally { if (valid() && turn === opening) changed(); }
    },
    takeCurrent() {
      if (!m.conflict || m.saving) return;
      m.drafts.delete(m.selected); m.opened = { file: m.conflict }; m.conflict = null; m.notice = ''; changed();
    },
    async save(version) {
      if (m.saving || m.committing || !m.dirty() || m.readOnlyReason() || !valid()) return;
      const path = m.selected, file = m.opened.file, text = m.shown(), draft = m.drafts.get(path);
      m.saving = true; m.notice = ''; changed();
      try {
        const saved = result(await engine.request('project.files.write', { projectId: context.projectId, worktreeId: context.worktreeId, path, version: version || file.version, text }));
        if (!valid()) return;
        if (m.drafts.get(path) === draft) m.drafts.delete(path);
        if (m.selected === path) { m.opened = { file: { ...file, text, version: saved.version, size: new TextEncoder().encode(text).length } }; m.conflict = null; }
        m.notice = `Saved ${path.split('/').pop()}.${saved.told ? ` ${saved.told} working agent${saved.told === 1 ? ' was' : 's were'} told which file you changed.` : ''}`;
        await m.refreshFolders(path);
      } catch (e) {
        if (!valid()) return;
        m.notice = e.message;
        if (e.message?.startsWith('the file changed since you opened it')) {
          try {
            const current = result(await engine.request('project.files.read', { worktreeId: context.worktreeId, path }));
            if (valid() && m.selected === path) {
              m.conflict = current;
              m.notice = 'The file changed since you opened it. Your text is kept. Compare it with the current file below.';
            }
          } catch (readError) { if (valid()) m.notice = readError.message; }
        }
      } finally { if (valid()) { m.saving = false; changed(); } }
    },
    async refreshFolders(path) {
      // Reload only already-read folders. This updates ancestor change dots too.
      const parents = new Set(['', parentOf(path)]);
      let folder = parentOf(path);
      while (folder) { parents.add(folder); folder = parentOf(folder); }
      await Promise.all([...parents].filter((p) => m.folders[p]).map((p) => m.folder(p)));
    },
    async commit(message) {
      if (m.committing || m.saving || m.dirty() || m.readOnlyReason() || !m.selected || !message.trim() || !valid()) return false;
      const path = m.selected;
      m.committing = true; m.notice = ''; changed();
      try {
        const saved = result(await engine.request('project.commit', { worktreeId: context.worktreeId, projectId: context.projectId, path, message: message.trim() }));
        if (!valid()) return false;
        m.notice = `Committed ${path.split('/').pop()} as ${saved.head.slice(0, 7)}.`;
        await m.refreshFolders(path); return true;
      } catch (e) { if (valid()) m.notice = e.message; return false; }
      finally { if (valid()) { m.committing = false; changed(); } }
    },
  };
  return m;
}

const STATUS = { A: ['add', 'Added'], M: ['mod', 'Changed'], D: ['mod', 'Deleted'], R: ['mod', 'Renamed'], U: ['mod', 'Conflict'] };
const extOf = (p) => (p.match(/\.([a-z0-9]+)$/i)?.[1] || '').toLowerCase();
const ICON = { md: 'file-text', css: 'palette', svg: 'image', png: 'image', zip: 'archive', ts: 'file-code', tsx: 'file-code' };
const ltr = (t) => '\u200e' + (t || '');
function Tree({ model, folder = '', depth = 0, open, toggle, pick }) {
  const state = model.folders[folder];
  if (!state || state.loading) return html`<li class="fx-note" role="status">Loading…</li>`;
  if (state.error) return html`<li class="fx-note" role="alert">${state.error} <button class="btn sm" onClick=${() => model.folder(folder)}>Retry</button></li>`;
  return html`${state.items.map((entry) => entry.kind === 'folder' ? html`<li key=${entry.path} role="treeitem" aria-label=${entry.name} aria-expanded=${open.has(entry.path)}>
    <button type="button" class="fx-row dir" aria-label=${entry.name} style=${`--d:${depth}`} onClick=${() => toggle(entry.path)}>
      <${Icon} name="chevron-right" size=${13} class="fx-chev" /><${Icon} name="folder" size=${14} class="faint" /><span class="truncate">${entry.name}</span>${entry.change && html`<i class="fx-dot" title="Has changes"></i>`}</button>
    ${open.has(entry.path) && html`<ul role="group"><${Tree} model=${model} folder=${entry.path} depth=${depth + 1} open=${open} toggle=${toggle} pick=${pick} /></ul>`}</li>`
    : html`<li key=${entry.path} role="treeitem" aria-label=${entry.name} aria-selected=${model.selected === entry.path}>
    <button type="button" class=${cx('fx-row', model.selected === entry.path && 'on')} aria-label=${entry.name} style=${`--d:${depth}`} onClick=${() => pick(entry.path)}>
      <span class="fx-chev"></span><${Icon} name=${ICON[extOf(entry.path)] || 'file'} size=${14} class="faint" /><span class="truncate">${entry.name}</span>
      ${model.drafts.has(entry.path) && html`<i class="fx-unsaved" title="Not saved"></i>`}${entry.status && html`<span class=${cx('fx-st', STATUS[entry.status][0])} title=${STATUS[entry.status][1]}>${entry.status}</span>`}</button></li>`)}
    ${!state.items.length && html`<li class="fx-note">Empty</li>`}${state.truncated && html`<li class="fx-note">Some entries are not shown.</li>`}`;
}
function CommitFile({ model, w, close }) {
  const [message, setMessage] = useState('');
  const submit = async () => { if (await model.commit(message)) close(); };
  const disabled = model.committing || !!model.readOnlyReason();
  return html`<${Dialog} title=${'Commit ' + model.selected.split('/').pop()} size="narrow" onClose=${() => { if (!model.committing) close(); }} onSubmit=${submit}
    foot=${html`<button type="button" class="btn" disabled=${model.committing} onClick=${close}>Cancel</button><button type="submit" class="btn primary" disabled=${disabled || !message.trim()}><${Icon} name="check" size=${14} />${model.committing ? 'Committing…' : 'Commit'}</button>`}>
    <div class="field"><div class="row"><label for="fx-live-commit-msg" class="grow">Commit message</label><span title="Engine work needed" data-tip="Engine work needed"><button type="button" class="btn sm ghost" disabled title="Engine work needed"><${Icon} name="sparkles" size=${13} />Generate</button></span></div>
      <textarea id="fx-live-commit-msg" class="textarea" rows="3" autofocus value=${message} disabled=${disabled} maxLength=${8192} onInput=${(e) => setMessage(e.target.value)} onKeyDown=${(e) => { if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) { e.preventDefault(); void submit(); } }}></textarea></div>
    <p class="muted fx-commit-note">Only <span class="mono">${model.selected}</span> is committed, to <span class="mono">${w.branch}</span>, as you. Other staged files stay staged. For anything bigger, ask the project lead.</p>
    ${model.notice && html`<p role="status">${model.notice}</p>`}
  <//>`;
}
// The parent selects the inspected worktree. A changed worktree gets a fresh model.
export function LiveFilesTab({ s, project, route }) {
  const { w, missing } = mapping.worktreeFor(s, project);
  if (!w) return html`<div class="fx-state" data-empty><strong>No worktree</strong></div>`;
  return html`<${WorktreeFiles} key=${w.id} s=${s} project=${project} route=${route} w=${w} missing=${missing} />`;
}
function WorktreeFiles({ s, project, route, w, missing }) {
  const phone = useMedia('(max-width: 860px)'), machine = machineOf(s, w.machine);
  const offline = !machine || machine.status !== 'online' || s.conn.status !== 'live';
  const remote = w.machine !== 'local';
  const [, bump] = useState(0), alive = useRef(true);
  const context = useMemo(() => ({ worktreeId: w.id, projectId: project.id, kind: w.kind, machine: w.machine, online: !offline }), [w.id]);
  Object.assign(context, { online: !offline, kind: w.kind, machine: w.machine });
  const model = useMemo(() => createFilesModel(context, () => bump((n) => n + 1), () => alive.current && store.state === s && s.session.status === 'in'), [w.id]);
  const [open, setOpen] = useState(() => new Set()), [query, setQuery] = useState(''), [wrapPref, setWrap] = useState(null), [committing, setCommitting] = useState(false);
  const wrap = wrapPref ?? phone, sel = route.query.get('f') || (phone ? null : 'README.md');
  useEffect(() => { alive.current = true; return () => { alive.current = false; model.drafts.clear(); }; }, []);
  useEffect(() => {
    if (offline || remote) return;
    void model.folder('');
    const refresh = () => { for (const folder of Object.keys(model.folders)) void model.folder(folder); };
    addEventListener('focus', refresh); return () => removeEventListener('focus', refresh);
  }, [offline, remote, s.boardRevision]);
  useEffect(() => {
    void model.open(sel);
    if (!sel || offline || remote) return;
    const parents = sel.split('/').slice(0, -1).map((_, i, a) => a.slice(0, i + 1).join('/'));
    setOpen((o) => new Set([...o, ...parents]));
    for (const folder of parents) if (!model.folders[folder]) void model.folder(folder);
  }, [sel, offline, remote]);
  useEffect(() => { const timer = setTimeout(() => model.search(query), 150); return () => clearTimeout(timer); }, [query, offline]);
  const pick = (path) => { if (!model.saving && !model.committing) go(`/p/${project.id}/files?f=${encodeURIComponent(path)}`); };
  const toggle = (path) => { setOpen((o) => { const next = new Set(o); next.has(path) ? next.delete(path) : next.add(path); return next; }); if (!model.folders[path]) void model.folder(path); };
  const file = model.opened?.file, textFile = file?.kind === 'text', shown = model.shown(), dirty = model.dirty(), reason = model.readOnlyReason();
  const lines = textFile ? shown.replace(/\n$/, '').split('\n').length : 0;
  const change = Object.values(model.folders).flatMap((f) => f.items || []).find((f) => f.path === sel)?.status;
  const worker = w.owner && agentOf(s, w.owner);
  const image = useMemo(() => {
    if (file?.kind !== 'image') return null;
    const bytes = Uint8Array.from(atob(file.data), (c) => c.charCodeAt(0));
    return URL.createObjectURL(new Blob([bytes], { type: file.mediaType }));
  }, [file]);
  useEffect(() => () => { if (image) URL.revokeObjectURL(image); }, [image]);
  const side = html`<aside class="fx-side" aria-label="Files"><div class="fx-where">
    <div class="row" style="gap:6px;min-width:0"><${Icon} name="git-branch" size=${13} class="faint" /><span class="mono truncate-start" title=${w.branch}>${ltr(w.branch)}</span></div>
    <div class="fx-mach"><i class=${cx('dot', offline ? 'failed' : 'working')}></i><span>${machine?.name || 'No machine'}</span><span class="mono truncate-start faint" title=${w.folder}>${ltr(w.folder)}</span></div></div>
    <div class="fx-find search"><${Icon} name="search" size=${13} /><input class="input" placeholder="Go to file" aria-label="Go to file" value=${query} maxLength=${256} disabled=${offline || remote || model.saving || model.committing} onInput=${(e) => setQuery(e.target.value)} /></div>
    ${missing && html`<p class="fx-note"><span class="mono">${missing}</span> has no worktree yet. This is <span class="mono">${w.branch}</span>.</p>`}
    <div class="fx-tree" data-main>${!offline && !remote && (query.trim() ? html`<ul class="fx-found">${model.searching || !model.found ? html`<li class="fx-note" role="status">Searching…</li>` : html`${model.found.paths.map((path) => html`<li key=${path}><button type="button" class=${cx('fx-row', sel === path && 'on')} aria-label=${path} onClick=${() => { pick(path); setQuery(''); }}><${Icon} name=${ICON[extOf(path)] || 'file'} size=${14} class="faint" /><span class="truncate-start mono" title=${path}>${ltr(path)}</span></button></li>`)}${!model.found.paths.length && html`<li class="fx-none">${model.found.error || 'No file matches'}</li>`}${model.found.truncated && html`<li class="fx-note">More files match. Type more of the name.</li>`}`}</ul>`
    : html`<ul role="tree" aria-label=${'Files in ' + w.branch}><${Tree} model=${model} open=${open} toggle=${toggle} pick=${pick} /></ul>`)}</div></aside>`;
  const crumbs = sel?.split('/') || [];
  const head = html`<div class="fx-head">${phone && html`<a class="icon-btn" href=${href(`/p/${project.id}/files`)} aria-label="Back to the files"><${Icon} name="arrow-left" size=${16} /></a>`}
    <div class="fx-crumbs mono" title=${sel}>${crumbs.length > 1 && html`<span class="fx-dir muted">${ltr(crumbs.slice(0, -1).join(' / '))}</span><span class="faint">/</span>`}<b>${crumbs.at(-1)}</b></div>
    ${change && html`<span class=${cx('fx-st', STATUS[change][0])}>${STATUS[change][1]}</span>`}<span class="fx-gap"></span>
    ${file && html`<span class="fx-size muted">${lines ? plural(lines, 'line') + ' · ' : ''}${fileSize(file.size)}</span>`}
    ${textFile && reason && html`<span class="fx-ro muted" title=${reason}><${Icon} name="lock" size=${12} />Read-only</span>`}
    ${change && html`<a class="btn sm" href=${href(`/p/${project.id}/git`)} aria-label="Open in Git" title="Open in Git"><${Icon} name="git-branch" size=${13} /><span class="fx-lbl">Open in Git</span></a>`}
    ${textFile && !reason && html`<button type="button" class=${cx('btn sm', dirty && 'primary')} disabled=${!dirty || model.saving || model.committing} onClick=${() => model.save()} data-tip=${isMac ? '⌘S' : 'Ctrl+S'}>${model.saving ? 'Saving…' : 'Save'}</button>`}
    ${textFile && !reason && change && !dirty && html`<button type="button" class="btn sm primary" aria-label="Commit" disabled=${model.saving || model.committing} onClick=${() => setCommitting(true)}><${Icon} name="git-commit-horizontal" size=${13} /><span class="fx-lbl">Commit</span></button>`}
    <${Menu} class="icon-btn" align="end" label="More" trigger=${html`<${Icon} name="ellipsis" size=${16} />`} width=${220} items=${[...(lines ? [{ label: 'Wrap long lines', checked: wrap, onSelect: () => setWrap(!wrap) }, { sep: true }] : []), { label: 'Copy path', icon: 'copy', onSelect: () => navigator.clipboard?.writeText(sel).catch(() => {}) }, { label: 'Download', disabled: true, why: 'Engine work needed' }]} /></div>`;
  let body;
  if (offline) body = html`<div class="fx-state" data-empty><${Icon} name="monitor" size=${20} class="faint" /><strong>${machine?.name || 'The machine'} is offline</strong><span>Its files show again when it reconnects.</span></div>`;
  else if (remote) body = html`<div class="fx-state" data-empty><${Icon} name="monitor" size=${20} class="faint" /><strong>Files on this machine are awaiting machine support.</strong><span>Only worktrees on this computer can be read today.</span></div>`;
  else if (!sel) body = html`<div class="fx-state" data-empty><${Icon} name="file" size=${20} class="faint" /><strong>No file open</strong></div>`;
  else if (model.opened?.loading || !model.opened) body = html`<div class="fx-state" role="status"><${Icon} name="loader-circle" size=${18} class="spin faint" /><span>Reading from ${machine.name}…</span></div>`;
  else if (model.opened.error) body = html`<div class="fx-state" role="alert"><strong>${model.opened.error}</strong><span class="mono">${sel}</span><button class="btn" onClick=${() => model.open(sel)}>Retry</button></div>`;
  else if (file.kind === 'image') body = html`<div class="fx-pic"><img src=${image} alt=${sel} /></div>`;
  else if (file.kind === 'binary' || file.kind === 'large') body = html`<div class="fx-state"><${Icon} name=${file.kind === 'binary' ? 'archive' : 'file-text'} size=${20} class="faint" /><strong>${file.kind === 'binary' ? 'Binary file' : 'Too large to show'} · ${fileSize(file.size)}</strong><span>${file.kind === 'binary' ? 'It cannot be read here.' : 'Files over 512 KB are not shown here.'}</span><span title="Engine work needed" data-tip="Engine work needed"><button type="button" class="btn" disabled title="Engine work needed">Download</button></span></div>`;
  else body = html`${worker?.state === 'working' && dirty && html`<p class="fx-busy"><${Icon} name="info" size=${13} />${worker.name} is working in this worktree. When you save, it is told which file you changed.</p>`}
    ${reason && html`<p class="fx-note" role="note">Read-only. ${reason}</p>`}
    <${CodeEditor} key=${w.id + ':' + sel + ':' + file.version} doc=${shown} path=${sel} wrap=${wrap} readOnly=${!!reason || model.saving || model.committing} onChange=${(text) => model.edit(text)} onSave=${() => model.save()} />
    ${model.conflict && html`<section class="fx-note fx-current" aria-label="The file as it is now"><strong>The file as it is now</strong>
      <div class="row"><button type="button" class="btn sm" disabled=${model.saving} onClick=${() => model.takeCurrent()}>Use the current file</button><button type="button" class="btn sm" disabled=${!!reason || model.saving || model.conflict.kind !== 'text'} onClick=${() => model.save(model.conflict.version)}>Save my text over it</button></div>
      <pre>${model.conflict.kind === 'text' ? model.conflict.text : 'The current file is ' + model.conflict.kind + '.'}</pre></section>`}`;
  const main = html`<section class="fx-main" aria-label="Open file">${sel && head}${model.notice && html`<p class="fx-note" role="status">${model.notice}</p>`}${body}</section>`;
  return html`<div class=${cx('fx', phone && 'phone')} aria-busy=${model.saving || model.committing}>${phone ? sel ? main : side : html`${side}${main}`}${committing && html`<${CommitFile} model=${model} w=${w} close=${() => setCommitting(false)} />`}</div>`;
}
