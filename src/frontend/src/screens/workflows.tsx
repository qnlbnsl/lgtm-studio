import { errorText } from '../live/reads';
// Workflow definitions are edited here; project execution remains the build loop.
import { useState, useEffect, useRef } from 'preact/hooks';
import { live, href, ago, Icon, store } from '../lib';
import { Dialog, Menu } from '../ui';
import * as engine from '../engine';
import { confirm } from '../actions';
import { captureSession } from '../server/sync';
import { validateWorkflow as sampleError, workflowRoles as roles } from '../workflow-contracts';
import type { Repo, State } from '../types';

// The shape of a workflow file, as packages/contracts/src/workflows.ts defines it.
export interface WorkflowStep { from: string; to: string; when: string; role: string }
export interface WorkflowBlock { id: string; name: string; role: string; steps: WorkflowStep[]; end?: boolean }
export interface WorkflowPhase { id: string; name: string; blocks: WorkflowBlock[] }
export interface WorkflowMove { from: string; to: string; when: string }
export interface WorkflowDefinition { name: string; phases: WorkflowPhase[]; moves: WorkflowMove[]; settings: Record<string, boolean | number | null> }
// `version` counts the saves of a workflow. The first save of a clone is version 1.
export interface WorkflowSummary { scope: string; id: string; name: string; readOnly: boolean; error?: string; canDelete?: boolean; version?: number }
/** One older version of a workflow, kept beside its file. The owner decided on 2026-11-23 to keep them. */
export interface WorkflowVersion { version: number; savedAt: number }
export interface WorkflowRecord extends WorkflowSummary { definition: WorkflowDefinition }
interface WorkflowFolder { scope: string; error?: string; truncated?: boolean; limit?: number }
// `version` on a save is the version that was edited. `version` on `readVersion` is the one to read.
interface WorkflowParams { repoId?: string; scope?: string; id?: string; name?: string; target?: string; definition?: WorkflowDefinition; version?: number }
/** What a workflow request answers. Each request fills only its own fields. */
interface WorkflowReply { items?: WorkflowSummary[]; folders?: WorkflowFolder[]; workflow?: WorkflowRecord; valid?: boolean; error?: string; versions?: WorkflowVersion[] }

const copy = <T,>(value: T): T => JSON.parse(JSON.stringify(value));
const key = 'lgtm-mock:workflows';
// The sample keeps what the engine will keep in a history folder beside each workflow file:
// the number of the current version, and whole copies of the versions before it.
const historyKey = 'lgtm-mock:workflow-history';
const KEPT = 20;
interface Kept { version: number; history: (WorkflowVersion & { definition: WorkflowDefinition })[] }
const id = () => 'item-' + crypto.randomUUID().slice(0, 12);
const label = (role: string) => role.split('-').map((s) => s[0]!.toUpperCase() + s.slice(1)).join(' ');
async function sample(tag: string, p: WorkflowParams): Promise<WorkflowReply | undefined> {
  const workspace = store.state;
  const scope = captureSession();
  const defaultDefinition: WorkflowDefinition = await fetch(new URL('../../../../design/mock/workflows/build-loop.json', import.meta.url)).then((r) => { if (!r.ok) throw Error('The default workflow is unavailable.'); return r.json(); });
  if (store.state !== workspace || live.on || (scope && !scope.isCurrent())) throw Error('The workflow is unavailable.');
  const data: Record<string, WorkflowDefinition> = JSON.parse(localStorage.getItem(key) || '{}');
  const kept: Record<string, Kept> = JSON.parse(localStorage.getItem(historyKey) || '{}');
  const keyOf = (scope: string, id: string) => (scope === 'user' ? 'user' : p.repoId) + ':' + id;
  const keptOf = (k: string): Kept => kept[k] || { version: 1, history: [] };
  const summary = (scope: string, id: string, definition: WorkflowDefinition): WorkflowSummary => ({ scope, id, name: definition.name, readOnly: scope === 'default', ...(scope === 'default' ? {} : { version: keptOf(keyOf(scope, id)).version }) });
  const records = [{ ...summary('default', 'build-loop', defaultDefinition), definition: defaultDefinition }, ...Object.entries(data).filter(([k]) => k.startsWith((p.repoId || 'user') + ':') || k.startsWith('user:')).map(([k, d]) => ({ ...summary(k.startsWith('user:') ? 'user' : 'repository', k.split(':')[1]!, d), definition: d }))];
  const current = records.find((r) => r.scope === p.scope && r.id === p.id);
  const save = () => {
    if (workspace.server.repositoryMode) throw Error('The workflow is unavailable.');
    localStorage.setItem(key, JSON.stringify(data));
    localStorage.setItem(historyKey, JSON.stringify(kept));
  };
  if (tag === 'list') return { items: records.map(({ definition, ...r }) => r) };
  if (tag === 'validate') { const error = sampleError(p.definition); return { valid: !error, error }; }
  if (!current) throw Error('The workflow is unavailable.');
  if (tag === 'read') return { workflow: copy(current) };
  if (tag === 'clone') {
    const newId = p.name!.trim().toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '').slice(0, 64) || 'workflow-' + crypto.randomUUID();
    const k = (p.target === 'user' ? 'user' : p.repoId) + ':' + newId;
    if (data[k]) throw Error('That workflow already exists.');
    const definition = { ...copy(current.definition), name: p.name! };
    const error = sampleError(definition); if (error) throw Error(error);
    data[k] = definition; save(); return { workflow: { ...summary(p.target!, newId, definition), definition } };
  }
  if (p.scope === 'default') throw Error('The default workflow is read-only.');
  const k = (p.scope === 'user' ? 'user' : p.repoId) + ':' + p.id;
  const mine = keptOf(k);
  if (tag === 'versions') return { versions: mine.history.map(({ version, savedAt }) => ({ version, savedAt })).reverse() };
  if (tag === 'readVersion') {
    const old = mine.history.find((h) => h.version === p.version); if (!old) throw Error('That version is no longer kept.');
    return { workflow: { ...summary(p.scope!, p.id!, current.definition), definition: copy(old.definition) } };
  }
  if (tag === 'save') {
    const error = sampleError(p.definition); if (error) throw Error(error);
    // A save names the version it edited. A newer version on disk means someone else saved first.
    if (p.version != null && p.version !== mine.version) throw Error('The workflow changed since you opened it. Open it again, and make your change to the newer version.');
    kept[k] = { version: mine.version + 1, history: [...mine.history, { version: mine.version, savedAt: Date.now(), definition: copy(current.definition) }].slice(-KEPT) };
    data[k] = copy(p.definition!); save(); return { workflow: { ...summary(p.scope!, p.id!, p.definition!), definition: p.definition! } };
  }
  if (tag === 'delete') { delete data[k]; delete kept[k]; save(); return {}; }
}
// Every tag answers with its own fields, so a caller reads only the fields its tag fills.
const request = (tag: string, p: WorkflowParams): Promise<WorkflowReply> => (live.on ? engine.request('client.workflows.' + tag, p) : sample(tag, p)) as Promise<WorkflowReply>;

interface CloneDialogProps { repositories: Repo[]; repoId: string; onClone: (name: string, target: string) => void; close: () => void; pending: boolean; error: string }
function CloneDialog({ repositories, repoId, onClone, close, pending, error }: CloneDialogProps) {
  const [name, setName] = useState(''); const [target, setTarget] = useState('user');
  return <Dialog title="Clone workflow" onClose={() => !pending && close()} onSubmit={() => onClone(name, target)} foot={<>
      <button type="button" class="btn" disabled={pending} onClick={close}>Cancel</button>
      <button class="btn primary" type="submit" disabled={pending || !name.trim()}>Create copy</button>
    </>}>
    <div class="field">
      <label for="wfl-clone-name">Workflow name</label>
      <input id="wfl-clone-name" class="input" value={name} onInput={(e) => setName(e.currentTarget.value)} disabled={pending} autofocus />
    </div>
    <div class="field">
      <label for="wfl-target">Save in</label>
      <select id="wfl-target" class="input" value={target} disabled={pending} onChange={(e) => setTarget(e.currentTarget.value)}>
        <option value="user">Your workflows</option>
        <option value="repository" disabled={!repoId}>
          This repository
          {repoId ? ': ' + repositories.find((r) => r.id === repoId)?.name : ' (choose a repository first)'}
        </option>
      </select>
    </div>
    {error && <p role="alert">{error}</p>}
  </Dialog>;
}
function OrderButtons({ name, index, count, move }: { name: string; index: number; count: number; move: (delta: number) => void }) {
  return <>
    <button type="button" class="btn sm" aria-label={'Move ' + name + ' earlier'} disabled={index === 0} onClick={() => move(-1)}>
      ← Earlier
    </button>
    <button type="button" class="btn sm" aria-label={'Move ' + name + ' later'} disabled={index === count - 1} onClick={() => move(1)}>
      Later →
    </button>
  </>;
}
export function Workflows({ s }: { s: State }) {
  const [folders, setFolders] = useState<WorkflowFolder[]>([]);
  const [items, setItems] = useState<WorkflowSummary[]>([]), [record, setRecord] = useState<WorkflowRecord | null>(null), [draft, setDraft] = useState<WorkflowDefinition | null>(null);
  const [selected, setSelected] = useState<string | null | undefined>(null), [repoId, setRepoId] = useState('');
  const [error, setError] = useState(''), [invalid, setInvalid] = useState(''), [checking, setChecking] = useState(false), [pending, setPending] = useState(false), [status, setStatus] = useState('');
  const [cloning, setCloning] = useState(false), [cloneError, setCloneError] = useState('');
  const [from, setFrom] = useState(''), [to, setTo] = useState(''), [when, setWhen] = useState('');
  // The older versions of the open workflow, newest first, and the one that was put into the editor.
  const [versions, setVersions] = useState<WorkflowVersion[]>([]), [restored, setRestored] = useState<number | null>(null);
  const generation = useRef(0), editor = useRef<HTMLFieldSetElement>(null), loadedLocation = useRef<string | null>(null);
  const connected = !live.on || s.conn.status === 'live';
  const dirty = !!record && !!draft && JSON.stringify(record.definition) !== JSON.stringify(draft);
  const location: WorkflowParams = repoId ? { repoId } : {};
  const loadVersions = (r: WorkflowRecord) => { setVersions([]); if (live.on || r.readOnly) return; request('versions', { ...(repoId ? { repoId } : {}), scope: r.scope, id: r.id }).then((x) => setVersions(x.versions || []), () => {}); };
  const accept = (r: WorkflowRecord) => { setRecord(r); setDraft(copy(r.definition)); setSelected(r.definition.phases[0]?.blocks[0]?.id); setInvalid(''); setError(''); setStatus(''); setRestored(null); loadVersions(r); };
  // An older version goes into the editor as an unsaved change. Saving it makes it the newest version.
  const restore = async (v: number) => {
    setPending(true); setError('');
    try { const r = await request('readVersion', { ...location, scope: record!.scope, id: record!.id, version: v }); setDraft(copy(r.workflow!.definition)); setSelected(r.workflow!.definition.phases[0]?.blocks[0]?.id); setRestored(v); setStatus(''); }
    catch (e) { setError(errorText(e)); } finally { setPending(false); }
  };
  const loadList = async () => { const r = await request('list', location); setItems(r.items!); setFolders(r.folders || []); return r.items!; };
  const open = async (item: WorkflowSummary | undefined) => {
    // A workflow that failed to load stands in as the record, without a definition.
    if (item!.error) { setRecord(item as WorkflowRecord); setDraft(null); setError(item!.error); return; }
    setPending(true); setError('');
    try { accept((await request('read', { ...location, scope: item!.scope, id: item!.id })).workflow!); } catch (e) { setError(errorText(e)); } finally { setPending(false); }
  };
  const leave = (action: () => unknown) => dirty ? confirm({ title: 'Discard changes?', body: 'Your unsaved workflow changes will be lost.', action: 'Discard changes', onConfirm: action }) : action();
  useEffect(() => {
    if (!connected || loadedLocation.current === repoId) return;
    let active = true; setPending(true); setRecord(null); setDraft(null); setError('');
    request('list', repoId ? { repoId } : {}).then(async (r) => {
      const first = r.items!.find((i) => !i.error);
      const read = first && await request('read', { ...(repoId ? { repoId } : {}), scope: first.scope, id: first.id });
      if (active) { loadedLocation.current = repoId; setItems(r.items!); setFolders(r.folders || []); if (read) accept(read.workflow!); }
    }).catch((e) => active && setError(errorText(e))).finally(() => active && setPending(false));
    return () => { active = false; };
  }, [repoId, connected]);
  useEffect(() => {
    if (!draft || record?.readOnly) return;
    const g = ++generation.current; setChecking(true);
    request('validate', { definition: draft }).then((r) => { if (g === generation.current) setInvalid(r.valid ? '' : r.error || 'The workflow is invalid.'); }).catch((e) => { if (g === generation.current) setInvalid(errorText(e)); }).finally(() => { if (g === generation.current) setChecking(false); });
    return () => { generation.current++; };
  }, [draft]);
  useEffect(() => { const f = (e: BeforeUnloadEvent) => { if (dirty) { e.preventDefault(); e.returnValue = ''; } }; window.addEventListener('beforeunload', f); return () => window.removeEventListener('beforeunload', f); }, [dirty]);
  const edit = (fn: (d: WorkflowDefinition) => unknown) => { if (record!.readOnly) return; const d = copy(draft!); fn(d); setDraft(d); setStatus(''); setChecking(true); };
  const phase = draft?.phases.find((p) => p.id === selected || p.blocks.some((b) => b.id === selected)) || draft?.phases[0];
  const block = phase?.blocks.find((b) => b.id === selected);
  const allBlocks = draft?.phases.flatMap((p) => p.blocks.map((b) => ({ ...b, phase: p.name }))) || [];
  const order = new Map(allBlocks.map((b, i) => [b.id, i]));
  const moveName = (m: string) => allBlocks.find((b) => b.id === m)?.name || m;
  const reorder = <T,>(array: T[], index: number, delta: number) => { const [item] = array.splice(index, 1); array.splice(index + delta, 0, item!); };
  const save = async () => {
    if (pending || checking || invalid || !dirty || record!.readOnly) return;
    setPending(true); setError('');
    try { const r = await request('save', { ...location, scope: record!.scope, id: record!.id, definition: draft!, ...(record!.version != null ? { version: record!.version } : {}) }); accept(r.workflow!); await loadList(); setStatus(r.workflow!.version != null ? `Saved as version ${r.workflow!.version}` : 'Saved'); }
    catch (e) { setError(errorText(e)); } finally { setPending(false); }
  };
  const clone = async (name: string, target: string) => {
    if (pending) return; setPending(true); setCloneError('');
    try { const r = await request('clone', { ...location, scope: record!.scope, id: record!.id, target, name }); accept(r.workflow!); await loadList(); setCloning(false); }
    catch (e) { setCloneError(errorText(e)); } finally { setPending(false); }
  };
  const remove = () => confirm({ title: 'Delete workflow?', body: 'Delete this workflow file permanently.', action: 'Delete workflow', danger: true, waitForReply: true,
    onConfirm: async () => { setPending(true); try { await request('delete', { ...location, scope: record!.scope, id: record!.id }); const list = await loadList(); await open(list[0]); return true; } catch (e) { setError(errorText(e)); return false; } finally { setPending(false); } } });
  return <div class="wfl" data-main>
    <div class="page-head"><h1>Workflows</h1></div>
    <div class="wfl-toolbar">
      <label for="wfl-repository">Repository</label>
      <select id="wfl-repository" class="input" disabled={pending} value={repoId} onChange={(e) => { const value = e.currentTarget.value; e.currentTarget.value = repoId; leave(() => setRepoId(value)); }}>
        <option value="">Your workflows</option>
        {(s.repos || []).map((r) => <option key={r.id} value={r.id}>{r.name}</option>)}
      </select>
    </div>
    {error && <p role="alert">{error}</p>}
    {folders.map((folder) => <p role="status">
      {folder.scope === 'user' ? 'Your workflows' : 'Repository workflows'}
      {': '}
      {folder.error || (folder.truncated ? 'Only the first ' + folder.limit + ' folder entries were checked. Move older entries out of this workflow folder to see more.' : '')}
    </p>)}
    <div class="wfl-layout" aria-busy={pending}>
      <ul class="wfl-list" aria-label="Workflows">
        {items.map((item) => <li key={item.scope + item.id}>
        <button type="button" disabled={pending} aria-current={record?.id === item.id && record?.scope === item.scope ? 'true' : undefined} onClick={() => leave(() => open(item))}>
          {item.name}
        </button>
        <small>{item.readOnly ? 'Default, read-only' : item.scope === 'repository' ? 'Repository workflow' : 'Your workflow'}</small>
        {item.error && <p>{item.error}</p>}
      </li>)}
      </ul>
      <div class="wfl-content">
        {record?.error && record.canDelete && <button type="button" class="btn danger" disabled={pending} onClick={remove}>Delete</button>}
        {draft && record && <>
        <div class="wfl-toolbar">
          <h2>{draft.name}</h2>
          {record.version != null && (versions.length > 0
            ? <Menu class="btn sm" align="start" width={240} disabled={pending} trigger={<><Icon name="history" size={14} />Version {record.version}<Icon name="chevron-down" size={13} /></>}
              items={versions.map((v) => ({ label: `Version ${v.version}`, hint: Date.now() - v.savedAt < 60000 ? 'Just now' : ago(v.savedAt, Date.now()) + ' ago', checked: restored === v.version, onSelect: () => leave(() => restore(v.version)) }))} />
            : <span class="muted">Version {record.version}</span>)}
          <span class="grow" />
          <button class="btn" type="button" disabled={pending} onClick={() => leave(() => { setCloneError(''); setCloning(true); })}>
            Clone
          </button>
          {!record.readOnly && <>
            <button type="button" class="btn primary" disabled={pending || checking || !!invalid || !dirty} onClick={save}>{record.version != null ? `Save as version ${record.version + 1}` : 'Save'}</button>
            <button type="button" class="btn" disabled={pending || !dirty} onClick={() => { accept(record); }}>Discard changes</button>
            <button type="button" class="btn danger" disabled={pending} onClick={remove}>Delete</button>
          </>}
        </div>
        <p class="wfl-state" role="status">
          {record.readOnly ? 'Default, read-only' : invalid || (checking ? 'Validating…' : status || (dirty ? (restored != null ? `Version ${restored} is in the editor, not saved` : 'Unsaved changes') : 'No unsaved changes'))}
        </p>
        <p class="muted">→ forward · ↶ back</p>
        <div class="wfl-picture" tabindex={0} role="region" aria-label="Workflow phases">
          {draft.phases.map((p, pi) => <section class="wfl-phase" key={p.id}>
            <h3><button type="button" onClick={() => setSelected(p.id)}>{pi + 1}. <span>{p.name}</span> →</button></h3>
            <ol>
              {p.blocks.map((b) => <li key={b.id}>
              <button id={'wfl-' + b.id} class="wfl-block" type="button" aria-pressed={selected === b.id} onClick={() => setSelected(b.id)}>
                <b>{b.name}</b>
                <small>{label(b.role)}</small>
                {b.steps.length > 0 && <small>{b.steps.length} inner moves</small>}
              </button>
              {draft.moves.filter((m) => m.from === b.id).map((m, mi) => <div class="wfl-edge" key={mi}>
                <span aria-label={order.get(m.to)! <= order.get(m.from)! ? 'Go back' : 'Go forward'}>
                  {order.get(m.to)! <= order.get(m.from)! ? '↶' : '→'}
                </span>
                {' '}
                <button type="button" onClick={() => { setSelected(m.to); document.getElementById('wfl-' + m.to)?.scrollIntoView({ block: 'nearest', inline: 'center' }); }}>
                  {moveName(m.to)}
                </button>
                <small>{m.when}</small>
              </div>)}
            </li>)}
            </ol>
          </section>)}
        </div>
        <fieldset class="wfl-editor" ref={editor} disabled={pending}>
          <legend>{record.readOnly ? 'Block details' : 'Visual editor'}</legend>
          {!record.readOnly && <>
            <div class="field">
              <label for="wfl-name">Workflow name</label>
              <input id="wfl-name" class="input" value={draft.name} onInput={(e) => edit((d) => { d.name = e.currentTarget.value; })} />
            </div>
            <div class="wfl-toolbar">
              <button type="button" class="btn" onClick={() => { const p: WorkflowPhase = { id: id(), name: 'New phase', blocks: [{ id: id(), name: 'New block', role: 'builder', steps: [] }] }; edit((d) => d.phases.push(p)); setSelected(p.blocks[0]!.id); }}>
                Add phase
              </button>
            </div>
          </>}
          {phase && <section class="wfl-section">
            <h3>Phase: {phase.name}</h3>
            {!record.readOnly && <>
            <div class="field">
              <label for="wfl-phase-name">Phase name</label>
              <input id="wfl-phase-name" class="input" value={phase.name} onInput={(e) => edit((d) => { d.phases.find((p) => p.id === phase!.id)!.name = e.currentTarget.value; })} />
            </div>
            <div class="wfl-toolbar">
              <OrderButtons name="phase" index={draft.phases.indexOf(phase)} count={draft.phases.length} move={(delta) => edit((d) => reorder(d.phases, draft.phases.indexOf(phase), delta))} />
              <button type="button" class="btn" onClick={() => { const ids = new Set(phase.blocks.map((b) => b.id)); edit((d) => { d.phases = d.phases.filter((p) => p.id !== phase.id); d.moves = d.moves.filter((m) => !ids.has(m.from) && !ids.has(m.to)); }); setSelected(null); }}>
                Remove phase
              </button>
              <button type="button" class="btn" onClick={() => { const b: WorkflowBlock = { id: id(), name: 'New block', role: 'builder', steps: [] }; edit((d) => d.phases.find((p) => p.id === phase!.id)!.blocks.push(b)); setSelected(b.id); }}>
                Add block
              </button>
            </div>
          </>}
          </section>}
          {block && <section class="wfl-section">
            <h3>Block: {block.name}</h3>
            {!record.readOnly && <>
            <div class="field">
              <label for="wfl-block-name">Block name</label>
              <input id="wfl-block-name" class="input" value={block.name} onInput={(e) => edit((d) => { d.phases.find((p) => p.id === phase!.id)!.blocks.find((b) => b.id === block.id)!.name = e.currentTarget.value; })} />
            </div>
            <div class="field">
              <label for="wfl-role">Role</label>
              <select id="wfl-role" class="input" value={block.role} onChange={(e) => edit((d) => { d.phases.find((p) => p.id === phase!.id)!.blocks.find((b) => b.id === block.id)!.role = e.currentTarget.value; })}>
                {roles.map((r) => <option key={r} value={r}>{label(r)}</option>)}
              </select>
            </div>
            <div class="wfl-toolbar">
              <OrderButtons name="block" index={phase!.blocks.indexOf(block)} count={phase!.blocks.length} move={(delta) => edit((d) => reorder(d.phases.find((p) => p.id === phase!.id)!.blocks, phase!.blocks.indexOf(block), delta))} />
              <button type="button" class="btn" onClick={() => { edit((d) => { d.phases.find((p) => p.id === phase!.id)!.blocks = d.phases.find((p) => p.id === phase!.id)!.blocks.filter((b) => b.id !== block.id); d.moves = d.moves.filter((m) => m.from !== block.id && m.to !== block.id); }); setSelected(phase!.id); }}>
                Remove block
              </button>
            </div>
          </>}
            <h3>Moves from this block</h3>
            <ul>
              {draft.moves.filter((m) => m.from === block.id).map((m) => <li class="wfl-toolbar">
              <span>{order.get(m.to)! <= order.get(m.from)! ? '↶' : '→'} {moveName(m.to)}: {m.when}</span>
              {!record.readOnly && <button type="button" class="btn sm" aria-label={'Remove move to ' + moveName(m.to)} onClick={() => edit((d) => { d.moves.splice(d.moves.findIndex((x) => x.from === m.from && x.to === m.to && x.when === m.when), 1); })}>
              Remove move
            </button>}
            </li>)}
            </ul>
            {block.steps.length > 0 && <details>
              <summary>Inner moves in {block.name}</summary>
              <ol>{block.steps.map((st) => <li>{st.from} → {st.to}: {st.when} · {label(st.role)}</li>)}</ol>
            </details>}
          </section>}
          {!record.readOnly && <section class="wfl-section">
            <h3>Add a move</h3>
            <div class="wfl-toolbar">
              <label>
                {'From '}
                <select class="input" aria-label="Move from" value={from} onChange={(e) => setFrom(e.currentTarget.value)}>
                  <option value="">Choose a block</option>
                  {allBlocks.map((b) => <option key={b.id} value={b.id}>{b.phase}: {b.name}</option>)}
                </select>
              </label>
              <span>→</span>
              <label>
                {'To '}
                <select class="input" aria-label="Move to" value={to} onChange={(e) => setTo(e.currentTarget.value)}>
                  <option value="">Choose a block</option>
                  {allBlocks.map((b) => <option key={b.id} value={b.id}>{b.phase}: {b.name}</option>)}
                </select>
              </label>
            </div>
            <div class="field">
              <label for="wfl-when">Move when</label>
              <input id="wfl-when" class="input" value={when} onInput={(e) => setWhen(e.currentTarget.value)} />
            </div>
            <button type="button" class="btn" disabled={!allBlocks.some((b) => b.id === from) || !allBlocks.some((b) => b.id === to) || !when.trim()} onClick={() => { edit((d) => d.moves.push({ from, to, when })); setWhen(''); }}>
              Add move
            </button>
          </section>}
          <section class="wfl-section">
            <h3>Settings</h3>
            {Object.entries(draft.settings).map(([k, v]) => <div class="field" key={k}>
            <label for={'wfl-setting-' + k}>
              {({ tests: 'Run tests', agentReview: 'Agent review', fullReviewRounds: 'Full review rounds', approvalAfterRound: 'Ask for approval after round', maxReviewRounds: 'Maximum review rounds (empty means no limit)', stuckPoints: 'Stuck statement points', stuckShare: 'Stuck statement share (%)', stuckRounds: 'Stuck statement rounds', autoMergeFix: 'Merge fixes users cannot see' } as Record<string, string>)[k]}
            </label>
            {typeof v === 'boolean' ? <input id={'wfl-setting-' + k} type="checkbox" checked={v} disabled={record.readOnly} onChange={(e) => edit((d) => { d.settings[k] = e.currentTarget.checked; })} /> : <input id={'wfl-setting-' + k} class="input" type="number" value={v ?? ''} disabled={record.readOnly} onInput={(e) => edit((d) => { d.settings[k] = k === 'maxReviewRounds' && e.currentTarget.value === '' ? null : Number(e.currentTarget.value); })} />}
          </div>)}
          </section>
        </fieldset>
      </>}
      </div>
    </div>
    {cloning && <CloneDialog repositories={s.repos || []} repoId={repoId} pending={pending} error={cloneError} close={() => setCloning(false)} onClone={clone} />}
  </div>;
}
