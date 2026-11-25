// The Browser page of a project's Design view: the product as it runs, in a frame.
// A preview is a command that a machine runs in a worktree. The server passes the product's
// pages on, so the owner sees the product from any device. The page comes through the contract
// only; see src/contracts/proto/lgtm/v1/preview.proto.
import { useEffect, useLayoutEffect, useRef, useState } from 'preact/hooks';
import { Icon, cx, href, swap } from '../lib';
import * as act from '../actions';
import * as server from '../server/sync';
import { Menu } from '../ui';
import { Section } from './service';
import { placePath } from '../data';
import type { Project, Route, State } from '../types';

// The sizes are words, because the icon set has no tablet.
const SIZES: { id: string; label: string; w: number; h: number }[] = [
  { id: 'phone', label: 'Phone', w: 390, h: 844 },
  { id: 'tablet', label: 'Tablet', w: 820, h: 1180 },
  { id: 'full', label: 'Full', w: 0, h: 0 },
];
const STATE_WORD: Record<server.PreviewView['state'], string> = { running: 'Running', starting: 'Starting', stopped: 'Stopped', failed: 'Failed' };

export function BrowserPage({ project, route }: { s: State; project: Project; route: Route }) {
  // The list is kept with its project's id, so a change of project never shows the last project's product.
  const [held, setHeld] = useState<{ project: string; list: server.PreviewView[] } | null>(null);
  const list = held?.project === project.id ? held.list : null;
  const [error, setError] = useState('');
  useEffect(() => { setError(''); return server.watchPreviews(project.id, (l) => { setHeld({ project: project.id, list: l }); setError(''); }, setError); }, [project.id]);

  // The shown preview and the size are in the address. A choice takes the place of the last one.
  const cur = list?.find((p) => p.id === route.query.get('show')) || list?.find((p) => p.state === 'running') || list?.[0] || null;
  const size = SIZES.find((x) => x.id === route.query.get('size')) || SIZES[2]!;
  const show = (id: string | undefined, sz: string) => { const q = new URLSearchParams(); if (id) q.set('show', id); if (sz !== 'full') q.set('size', sz);
    swap(`${placePath(project)}/design/browser${q.size ? '?' + q : ''}`); };

  // One address is opened for the running preview, and given back when the page closes. Reload opens a new one.
  const [frame, setFrame] = useState<server.PreviewFrame | null>(null); const [turn, setTurn] = useState(0); const [picking, setPicking] = useState(false);
  const running = cur?.state === 'running' ? cur.id : '';
  useEffect(() => {
    setFrame(null);
    if (!running) return undefined;
    let closed = false; let opened: server.PreviewFrame | null = null;
    server.openPreview(running).then((f) => { if (closed) void server.releasePreview(f.id); else { opened = f; setFrame(f); } }, (e) => { if (!closed) setError(server.reason(e)); });
    return () => { closed = true; if (opened) void server.releasePreview(opened.id); };
  }, [running, turn]);

  // A phone or a tablet is drawn at its own size, and made smaller when the pane is smaller.
  const stage = useRef<HTMLDivElement>(null); const [room, setRoom] = useState({ w: 0, h: 0 });
  useLayoutEffect(() => {
    const el = stage.current; if (!el) return undefined;
    const read = () => setRoom({ w: el.clientWidth, h: el.clientHeight });
    read(); const ro = new ResizeObserver(read); ro.observe(el);
    return () => ro.disconnect();
  }, [!!frame, size.id]);
  const scale = size.w ? Math.min(1, (room.w - 32) / size.w, (room.h - 32) / size.h) : 1;

  const start = () => { if (cur) void server.startPreview(cur.id).catch((e) => act.toast(server.reason(e), 'error')); };
  const stop = () => { if (cur) void server.stopPreview(cur.id).catch((e) => act.toast(server.reason(e), 'error')); };

  if (error && !list) return <div class="bw"><div class="bw-none"><Icon name="triangle-alert" size={20} class="error-text" /><strong>{error}</strong></div></div>;
  if (!list) return <div class="bw" aria-busy="true" />;
  if (!cur) return <div class="bw">
    <div class="bw-none" data-empty>
      <Icon name="monitor" size={20} class="faint" />
      <strong>No previews</strong>
    </div>
  </div>;
  const pick = <>
    <b>{cur.name}</b>
    {list.length > 1 && <Icon name="chevrons-up-down" size={12} class="faint" />}
  </>;
  return <div class="bw">
    <div class="bw-bar">
      {list.length > 1
        ? <Menu class="bw-pick" width={320} trigger={pick} items={list.map((p) => ({ label: p.name, sub: STATE_WORD[p.state], hint: p.command, checked: p.id === cur.id, onSelect: () => show(p.id, size.id) }))} />
        : <span class="bw-pick">{pick}</span>}
      <span class={cx('bw-state', cur.state)}><i class={cx('dot', cur.state === 'running' ? 'working' : cur.state === 'failed' ? 'failed' : 'idle')} />{STATE_WORD[cur.state]}</span>
      <span class="mono muted truncate bw-where" title={`${cur.command} on ${cur.machine}, in ${cur.branch}`}>{cur.machine}:{cur.port}</span>
      {error && <span class="error-text truncate" role="alert">{error}</span>}
      <span class="grow" />
      {cur.state === 'running' && <>
        <div class="seg bw-sizes" role="group" aria-label="Size">
          {SIZES.map((x) => <button key={x.id} type="button" aria-pressed={size.id === x.id} data-tip={x.w ? `${x.w} × ${x.h}` : undefined} onClick={() => show(cur.id, x.id)}>{x.label}</button>)}
        </div>
        {/* The picker is not built: it needs the browser extension. The button only shows where it will be. */}
        <button type="button" class="icon-btn" aria-label="Pick an element" aria-pressed={picking} data-tip="Pick an element" onClick={() => setPicking(!picking)}><Icon name="target" size={14} /></button>
        <button type="button" class="icon-btn" aria-label="Reload" data-tip="Reload" onClick={() => setTurn(turn + 1)}><Icon name="refresh-cw" size={14} /></button>
        {frame && !frame.url.startsWith('data:') && <a class="icon-btn" href={frame.url} target="_blank" rel="noopener noreferrer" aria-label="Open in a new tab" data-tip="Open in a new tab" data-tip-side="left">
          <Icon name="external-link" size={14} />
        </a>}
        <button type="button" class="btn sm" onClick={stop}><Icon name="square" size={12} />Stop</button>
      </>}
    </div>
    {cur.state === 'running'
      ? <div class={cx('bw-stage', size.w ? 'sized' : 'full')} ref={stage}>
        {frame && <div class="bw-device" style={size.w ? { width: size.w * scale + 'px', height: size.h * scale + 'px' } : undefined}>
          {/* The product's page is code that agents wrote. It runs without this page's origin, so it cannot use the session. */}
          <iframe key={frame.id} class="bw-frame" src={frame.url} title={`${cur.name} of ${project.name}`} sandbox="allow-scripts allow-forms allow-popups" referrerpolicy="no-referrer"
            style={size.w ? { width: size.w + 'px', height: size.h + 'px', transform: `scale(${scale})` } : undefined} />
        </div>}
      </div>
      : <div class="bw-none">
        {cur.state === 'starting' ? <>
          <strong>Starting {cur.name}</strong>
          <span class="bw-busy" role="status"><Icon name="loader-circle" size={14} class="auth-spin" /><span class="mono">{cur.command}</span></span>
        </> : <>
          <strong>{cur.state === 'failed' ? `${cur.name} failed to start` : `${cur.name} is not running`}</strong>
          {cur.error && <span class="error-text">{cur.error}</span>}
          <button type="button" class="btn primary" onClick={start}><Icon name="play" size={13} />Run <span class="mono">{cur.command}</span></button>
        </>}
      </div>}
  </div>;
}

// ---- the previews of a project, in its Settings -----------------------------------------
// A preview is set here and shown on the Browser page. Each field saves when it loses the cursor,
// or on Enter. The list comes through the contract, as it does on the Browser page.
interface Draft { name: string; command: string; port: string; machineId: string }
// A refused value is named on the field that holds it. The reason comes from the server.
const fieldOf = (reason: string): 'name' | 'command' | 'port' => /port/i.test(reason) ? 'port' : /command/i.test(reason) ? 'command' : 'name';
function PreviewRow({ s, project, p, onError }: { s: State; project: Project; p: server.PreviewView | null; onError: (text: string) => void }) {
  const [refused, setRefused] = useState<{ field: 'name' | 'command' | 'port'; text: string } | null>(null);
  const blank: Draft = { name: '', command: '', port: '', machineId: project.machine };
  const from = (): Draft => p ? { name: p.name, command: p.command, port: String(p.port), machineId: p.machineId } : blank;
  const [d, setD] = useState<Draft>(from);
  // The row follows the server, except while the owner types in it.
  useEffect(() => { setD(from()); }, [p?.name, p?.command, p?.port, p?.machineId]);
  const changed = !p || d.name.trim() !== p.name || d.command.trim() !== p.command || d.port !== String(p.port) || d.machineId !== p.machineId;
  const ready = !!d.name.trim() && !!d.command.trim() && /^\d+$/.test(d.port);
  const save = (next: Draft = d) => {
    if (!next.name.trim() || !next.command.trim() || !/^\d+$/.test(next.port)) { if (p) setD(from()); return; }
    void server.savePreview({ ...(p ? { id: p.id } : {}), projectId: project.id, name: next.name.trim(), command: next.command.trim(), port: Number(next.port), machineId: next.machineId })
      .then(() => { setRefused(null); if (!p) setD(blank); act.toast(p ? 'Preview saved' : `Added ${next.name.trim()}`); }, (e) => { const text = server.reason(e); setRefused({ field: fieldOf(text), text }); });
  };
  // An existing preview saves a field when the cursor leaves it. A new one waits for its button.
  const field = (k: 'name' | 'command' | 'port', label: string, cls: string, placeholder: string) => <input class={cx('input', cls, refused?.field === k && 'invalid')} aria-invalid={refused?.field === k || undefined} aria-label={p ? `${label} of ${p.name}` : `${label} of the new preview`} placeholder={p ? undefined : placeholder}
    inputMode={k === 'port' ? 'numeric' : undefined} maxLength={k === 'port' ? 5 : 200} value={d[k]} onInput={(e) => { if (refused?.field === k) setRefused(null); setD({ ...d, [k]: k === 'port' ? e.currentTarget.value.replace(/\D/g, '') : e.currentTarget.value }); }}
    onBlur={() => { if (p && changed) save(); }} onKeyDown={(e) => { if (e.key === 'Enter') { if (p) e.currentTarget.blur(); else if (ready) save(); } else if (e.key === 'Escape' && p && changed) { e.stopPropagation(); setRefused(null); setD(from()); } }} />;
  const machines = s.machines.some((m) => m.id === d.machineId) || !p ? s.machines : [...s.machines, { id: p.machineId, name: p.machine, status: 'online' } as State['machines'][number]];
  return <><tr class={cx(refused && 'pv-bad')}>
    <td>{field('name', 'Name', '', 'Name')}</td>
    <td>{field('command', 'Command', 'mono', 'Command, such as pnpm dev')}</td>
    <td>{field('port', 'Port', 'mono pv-port', 'Port')}</td>
    <td>
      <select class="select" aria-label={p ? `Machine of ${p.name}` : 'Machine of the new preview'} value={d.machineId} onChange={(e) => { const next = { ...d, machineId: e.currentTarget.value }; setD(next); if (p) save(next); }}>
        {machines.map((m) => <option key={m.id} value={m.id}>{m.name}</option>)}
      </select>
    </td>
    <td class="pv-state">
      {p ? <a class={cx('bw-state', p.state)} href={href(`${placePath(project)}/design/browser?show=${p.id}`)} data-tip="Open in Browser"><i class={cx('dot', p.state === 'running' ? 'working' : p.state === 'failed' ? 'failed' : 'idle')} />{STATE_WORD[p.state]}</a>
        : <button type="button" class="btn sm" disabled={!ready} onClick={() => save()}><Icon name="plus" size={13} />Add preview</button>}
    </td>
    <td class="c-act">
      {p && <button type="button" class="icon-btn sm" aria-label={`Remove ${p.name}`} data-tip="Remove preview" data-tip-side="left"
        onClick={() => act.confirm({ title: `Remove ${p.name}?`, body: p.state === 'running' ? 'It stops first. The command and the port are forgotten.' : 'The command and the port are forgotten.', action: 'Remove preview', danger: true,
          onConfirm: () => { void server.removePreview(p.id).catch((e) => onError(server.reason(e))); } })}>
        <Icon name="trash-2" size={14} />
      </button>}
    </td>
  </tr>
    {refused && <tr class="pv-refused"><td colSpan={6} role="alert"><span><Icon name="circle-alert" size={13} />{refused.text}{refused.field === 'name' ? ' Give this one another name.' : ''}</span></td></tr>}
  </>;
}
export function PreviewSettings({ s, project }: { s: State; project: Project }) {
  const [held, setHeld] = useState<{ project: string; list: server.PreviewView[] } | null>(null);
  const list = held?.project === project.id ? held.list : null;
  const [error, setError] = useState('');
  useEffect(() => { setError(''); return server.watchPreviews(project.id, (l) => { setHeld({ project: project.id, list: l }); setError(''); }, setError); }, [project.id]);
  const fail = (text: string) => act.toast(text, 'error');
  return <div class="page svc-project" aria-busy={!list && !error}>
    <Section title="Previews" sub={list ? (list.length ? null : 'None') : error ? <span class="error-text">{error}</span> : null}>
      {list && <div class="table-wrap pv-table">
        <table class="table">
          <thead><tr><th>Name</th><th>Command</th><th>Port</th><th>Machine</th><th>State</th><th class="c-act" aria-label="Remove" /></tr></thead>
          <tbody>
            {list.map((p) => <PreviewRow key={p.id} s={s} project={project} p={p} onError={fail} />)}
            <PreviewRow key="new" s={s} project={project} p={null} onError={fail} />
          </tbody>
        </table>
      </div>}
    </Section>
  </div>;
}
