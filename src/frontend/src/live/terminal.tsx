import { errorText } from './reads';
// Terminal renderers stay in this page's memory. The standalone sample never loads this module.
import { useState, useEffect, useLayoutEffect, useRef } from 'preact/hooks';
import { Icon, cx, notify, href, persist, remember } from '../lib';
import { machineOf } from '../data';
import { Menu } from '../ui';
import { TerminalLaunch } from '../terminal-launch';
import * as engine from '../engine';
import * as act from '../actions';
import type { LiveMaps } from '../engine';
import type { Id, Project, State } from '../types';

/** The parts of an xterm.js terminal this tab uses. The live connection lends the class. */
interface XTerm {
  options: { disableStdin?: boolean; fontSize?: number }; element?: HTMLElement; cols: number; rows: number;
  buffer: { active: { length: number; type: string; getLine(row: number): { translateToString(trimRight?: boolean): string } | undefined } };
  open(el: HTMLElement): void; resize(cols: number, rows: number): void; focus(): void; dispose(): void; clear(): void; write(text: string): void;
  onData(listener: (text: string) => void): unknown;
}
/** A worktree a shell can open in, as terminalWorktrees maps it. */
interface ShellWorktree { id: Id; kind: string; branch: string; folder: string; machine: Id }
interface Shell {
  id: string; terminal: XTerm; label: string; worktree: ShellWorktree; dim: { cols: number; rows: number }; ended: string;
  ctrl?: boolean; readingElsewhere?: boolean; sending?: Promise<void>; cancel?: () => void;
}
interface ProjectShells { sessions: Shell[]; active: string | null; dismissed: Set<string> }
/** The parts of a store patch that change the open shells. */
interface ShellPatch { chatEvent?: { event?: { _tag?: string; settings?: { terminalEnabled?: boolean } } }; conn?: { status: string }; session?: { status: string } }

let mapping: LiveMaps | undefined, enabled: boolean | undefined, loading: Promise<void> | null = null, fontSize = Math.max(11, Math.min(18, persist('termFont', 13)));
const projects = new Map<Id, ProjectShells>();
export function setMapping(value: LiveMaps | undefined): void { mapping = value; }
function end(session: Shell, reason: string): void {
  if (session.ended) return;
  session.ended = reason; session.terminal.options.disableStdin = true; session.cancel?.(); notify();
}
export function observe(patch: ShellPatch): void {
  if (patch.chatEvent?.event?._tag === 'client.WebSettingsChanged') { enabled = patch.chatEvent.event.settings!.terminalEnabled === true; notify(); }
  if (patch.conn && patch.conn.status !== 'live' || patch.session && patch.session.status !== 'in') {
    for (const project of projects.values()) for (const session of project.sessions) end(session, 'The connection ended. Reconnecting checks for this shell.');
  }
  if (patch.session && ['out', 'expired', 'rejected', 'logout-unconfirmed'].includes(patch.session.status)) {
    for (const project of projects.values()) for (const session of project.sessions) session.terminal.dispose();
    projects.clear(); enabled = undefined;
  }
}
export function isConnectionLoss(reason: unknown): boolean { return typeof reason === 'string' && reason.startsWith('The connection ended.'); }
export function outputEnded(item: Shell, reason: string): void {
  if (reason === 'Output is open in another owner connection. Typing still works.') { item.readingElsewhere = true; notify(); }
  else end(item, reason);
}
export function outputClosed(item: Shell): void {
  if (!item.readingElsewhere) end(item, 'The connection ended. Reconnecting checks for this shell.');
}
export function reconcileTerminals(state: ProjectShells, rows: { sessionId: string }[]): void {
  const live = new Set(rows.map(row => row.sessionId));
  for (const item of state.sessions) {
    if (live.has(item.id)) continue;
    if (isConnectionLoss(item.ended)) { item.ended = 'This shell has ended. Open a new session.'; notify(); }
    else if (item.readingElsewhere) end(item, 'This shell has ended. Open a new session.');
  }
}
async function load(): Promise<void> {
  if (loading) return loading;
  loading = engine.request('client.web.settings').then(settings => { enabled = settings.terminalEnabled === true; notify(); }).finally(() => { loading = null; });
  return loading;
}
function textOf(terminal: XTerm): string {
  const buffer = terminal.buffer.active, lines: string[] = [];
  for (let row = 0; row < buffer.length; row++) lines.push(buffer.getLine(row)?.translateToString(true) || '');
  return lines.join('\n');
}
function TerminalView({ session, active, sizeChanged }: { session: Shell; active: boolean; sizeChanged: () => void }) {
  const host = useRef<HTMLDivElement>(null);
  useLayoutEffect(() => {
    const el = host.current!, term = session.terminal;
    if (!term.element) term.open(el); else el.appendChild(term.element);
    const fit = () => {
      if (!active || !el.clientWidth || !el.clientHeight) return;
      term.options.fontSize = fontSize;
      const screen = term.element!.querySelector('.xterm-screen')!;
      const style = getComputedStyle(el);
      const width = el.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight) - 14;
      const height = el.clientHeight - parseFloat(style.paddingTop) - parseFloat(style.paddingBottom);
      const dim = mapping!.terminalSize(width, height, screen.clientWidth / term.cols || fontSize * 0.6, screen.clientHeight / term.rows || fontSize * 1.45);
      if (term.cols !== dim.cols || term.rows !== dim.rows) {
        term.resize(dim.cols, dim.rows); session.dim = dim; sizeChanged();
        if (!session.ended) engine.request('client.terminal.resize', { sessionId: session.id, ...dim }).catch(e => end(session, errorText(e)));
      }
    };
    fit(); const observer = new ResizeObserver(fit); observer.observe(el); document.fonts?.ready.then(fit);
    if (active) term.focus();
    return () => observer.disconnect();
  }, [session.id, active, fontSize]);
  return <div ref={host} class="term-host" hidden={!active} aria-label={'Shell in ' + session.label} />;
}
const keys: [label: string, text: string | null][] = [['Esc', '\x1b'], ['Tab', '\t'], ['Ctrl', null], ['←', '\x1b[D'], ['↓', '\x1b[B'], ['↑', '\x1b[A'], ['→', '\x1b[C'], ['|', '|'], ['/', '/'], ['-', '-'], ['~', '~'], [':', ':']];
export function LiveTerminalTab({ s, project }: { s: State; project: Project }) {
  const [, changed] = useState(0), [notice, setNotice] = useState(''), [opening, setOpening] = useState(false);
  const redraw = () => changed(n => n + 1);
  if (!projects.has(project.id)) projects.set(project.id, { sessions: [], active: null, dismissed: new Set() });
  const state = projects.get(project.id)!, session = state.sessions.find(x => x.id === state.active) || state.sessions[0];
  useEffect(() => { if (s.conn.status === 'live') load().catch(e => setNotice(errorText(e))); }, [s.conn.status]);
  const worktrees: ShellWorktree[] = mapping!.terminalWorktrees(s, project);
  const close = async (item: Shell) => {
    try { if (!item.ended && s.conn.status === 'live') await engine.request('client.terminal.close', { sessionId: item.id }); }
    catch (e) { setNotice(errorText(e)); }
    state.dismissed.add(item.id); item.cancel?.(); item.terminal.dispose(); state.sessions = state.sessions.filter(x => x !== item);
    if (state.active === item.id) state.active = state.sessions.at(-1)?.id || null;
    redraw();
  };
  const write = (text: string) => {
    if (!session || session.ended || s.conn.status !== 'live') return;
    if (session.ctrl && text.length === 1) { text = String.fromCharCode(text.toUpperCase().charCodeAt(0) & 31); session.ctrl = false; redraw(); }
    engine.request('client.terminal.write', { sessionId: session.id, text }).catch(e => end(session, errorText(e)));
  };
  const attach = (worktree: ShellWorktree, sessionId: string) => {
    const terminal = new (mapping!.Terminal as unknown as new (options: object) => XTerm)({ cols: 80, rows: 24, fontSize, fontFamily: getComputedStyle(document.documentElement).getPropertyValue('--font-mono').trim() || 'monospace', lineHeight: 1.45, scrollback: 1500, cursorBlink: true, logLevel: 'off', theme: { background: getComputedStyle(document.documentElement).getPropertyValue('--color-term-bg').trim() || '#181818', foreground: getComputedStyle(document.documentElement).getPropertyValue('--color-term-text').trim() || '#dddddd' } });
    const item: Shell = { id: sessionId, terminal, label: worktree.kind === 'project' ? project.name : worktree.branch, worktree, dim: { cols: 80, rows: 24 }, ended: '' };
    terminal.onData(text => {
      if (item.ended) return;
      if (item.ctrl && text.length === 1) { text = String.fromCharCode(text.toUpperCase().charCodeAt(0) & 31); item.ctrl = false; notify(); }
      // Bound each API input in bytes, including large pastes. Preserve order.
      item.sending = (item.sending || Promise.resolve()).then(async () => {
        let part = '', bytes = 0;
        for (const ch of text) {
          const length = new TextEncoder().encode(ch).length;
          if (bytes + length > 4096) { await engine.request('client.terminal.write', { sessionId: item.id, text: part }); part = ''; bytes = 0; }
          part += ch; bytes += length;
        }
        if (part) await engine.request('client.terminal.write', { sessionId: item.id, text: part });
      }).catch(e => end(item, errorText(e)));
    });
    state.sessions.push(item); state.active = item.id;
    item.cancel = engine.connection().terminalOutput!(item.id, event => {
      if (event._tag === 'output') terminal.write(event.text);
      else outputEnded(item, event.reason);
    }, () => outputClosed(item));
    redraw();
  };
  const open = async (worktree: ShellWorktree) => {
    const status = mapping!.terminalState(enabled, s.conn.status, worktree.machine);
    if (status !== 'ready') { setNotice(status === 'remote' ? 'Project terminals are available only on the local machine.' : status === 'off' ? 'The project terminal is turned off.' : 'The machine is offline. Reconnect within five minutes to restore open shells.'); return; }
    setOpening(true); setNotice('');
    try {
      const reply = await engine.request('client.terminal.open', { projectId: project.id, worktreeId: worktree.id, cols: 80, rows: 24 });
      attach(worktree, reply.sessionId);
    } catch (e) { setNotice(errorText(e)); }
    finally { setOpening(false); }
  };
  useEffect(() => {
    if (s.conn.status !== 'live' || !enabled) return;
    let cancelled = false;
    engine.request('client.terminal.list').then((reply) => {
      if (cancelled) return;
      reconcileTerminals(state, reply.sessions);
      for (const { sessionId, worktree } of mapping!.terminalAttachments(reply.sessions, worktrees, project.id)) {
        if (state.dismissed.has(sessionId)) continue;
        const existing = state.sessions.find(item => item.id === sessionId);
        if (existing && !existing.ended && !existing.readingElsewhere) continue;
        if (existing) { existing.cancel?.(); existing.terminal.dispose(); state.sessions = state.sessions.filter(item => item !== existing); }
        attach(worktree, sessionId);
      }
    }).catch(e => { if (!cancelled) setNotice(errorText(e)); });
    return () => { cancelled = true; };
  }, [s.conn.status, enabled, worktrees.map(worktree => worktree.id).join(',')]);
  useEffect(() => {
    if (s.conn.status !== 'live' || !enabled) return;
    let cancelled = false, pending = false;
    // The displaced output stream has ended. Check only status, without
    // subscribing again and taking the other browser's output back.
    const timer = setInterval(async () => {
      if (pending || !state.sessions.some(item => item.readingElsewhere && !item.ended)) return;
      pending = true;
      try {
        const reply = await engine.request('client.terminal.list');
        if (!cancelled) reconcileTerminals(state, reply.sessions);
      } catch {} // Connection and session observers handle transport loss.
      finally { pending = false; }
    }, 1000);
    return () => { cancelled = true; clearInterval(timer); };
  }, [s.conn.status, enabled, project.id]);
  const options = worktrees.map(w => ({ value: w.id, label: (w.kind === 'project' ? project.name : w.branch) + ' · ' + (machineOf(s, w.machine)?.name || w.machine), folder: w.folder, reason: w.machine !== 'local' ? 'Engine work needed: local machine only' : null }));
  const newSession = (className = 'btn sm') => <TerminalLaunch options={options} disabled={opening || s.conn.status !== 'live'} onOpen={(value) => { const place = worktrees.find(w => w.id === value); if (place) open(place); }} className={className} />;
  const font = (n: number) => { fontSize = Math.max(11, Math.min(18, n)); remember('termFont', fontSize); redraw(); };
  const rename = (item: Shell) => act.openDialog({ type: 'rename', title: 'Rename session', value: item.label, taken: state.sessions.filter(x => x !== item).map(x => x.label), note: 'The name is only a label for this tab. The folder and the machine stay the same.', onRename: (value: string) => { item.label = value; redraw(); } });
  if (enabled === false) return <div class="term">
    <div class="term-none">
      <Icon name="square-terminal" size={22} class="faint" />
      <strong>The project terminal is turned off.</strong>
      <a class="btn" href={href('/settings?section=remote')}>Open Service settings</a>
    </div>
  </div>;
  if (!session) return <div class="term">
    <div class="term-none">
      <Icon name="square-terminal" size={22} class="faint" />
      <strong>
        {enabled === undefined ? 'Reading terminal settings…' : s.conn.status !== 'live' ? 'The machine is offline.' : project.machine !== 'local' ? 'Project terminals are available only on the local machine.' : 'No sessions are open'}
      </strong>
      <span class="muted">
        A session is a shell in one worktree on one machine. Reloading this page restores the shell and recent output. Disconnected shells stay available for five minutes.
      </span>
      {notice && <span role="status">{notice}</span>}
      {enabled && newSession('btn primary')}
    </div>
  </div>;
  const canReattach = !session.ended || isConnectionLoss(session.ended);
  const ended = session.ended || s.conn.status !== 'live';
  return <div class="term">
    <div class="term-tabs">
      <div class="term-tablist" role="tablist" aria-label="Terminal sessions">
        {state.sessions.map(item => <div key={item.id} class={cx('term-tab', item === session && 'on')}>
        <button type="button" role="tab" aria-selected={item === session} class="term-tab-main" title={item.worktree.folder} onClick={() => { state.active = item.id; redraw(); }} onDblClick={() => rename(item)}>
          <i class={cx('dot', item.ended ? 'waiting' : 'working')} />
          <span class="truncate">{item.label}</span>
        </button>
        <button type="button" class="icon-btn sm term-tab-x" aria-label={'Close ' + item.label} onClick={() => close(item)}>
          <Icon name="x" size={12} />
        </button>
      </div>)}
      </div>
      {newSession()}
      <span class="grow" />
      <div class="term-tools">
        <div class="term-font not-phone" role="group" aria-label="Text size">
          <button type="button" class="icon-btn" aria-label="Smaller text" disabled={fontSize <= 11} onClick={() => font(fontSize - 1)}>
            <Icon name="minus" size={14} />
          </button>
          <span class="num">{fontSize}</span>
          <button type="button" class="icon-btn" aria-label="Larger text" disabled={fontSize >= 18} onClick={() => font(fontSize + 1)}>
            <Icon name="plus" size={14} />
          </button>
        </div>
        <Menu class="icon-btn" label="Session actions" align="end" width={230} trigger={<Icon name="ellipsis" size={16} />} items={[
      { label: 'Rename session', icon: 'pencil', onSelect: () => rename(session) },
      { label: 'Copy all output', icon: 'copy', onSelect: () => navigator.clipboard.writeText(textOf(session.terminal)).catch(() => setNotice('Copying is blocked in this browser')) },
      { label: 'Clear scrollback', icon: 'rotate-ccw', disabled: session.terminal.buffer.active.type === 'alternate', onSelect: () => session.terminal.clear() },
      { label: 'Watch agent session', icon: 'eye', disabled: true, why: 'Engine work needed' },
      { label: 'Take over agent session', disabled: true, why: 'Engine work needed' },
      { label: 'Larger text', icon: 'plus', disabled: fontSize >= 18, onSelect: () => font(fontSize + 1) },
      { label: 'Smaller text', icon: 'minus', disabled: fontSize <= 11, onSelect: () => font(fontSize - 1) },
      { sep: true }, { label: 'Close session', icon: 'x', danger: true, onSelect: () => close(session) }
    ]} />
      </div>
    </div>
    {ended && <div class="term-bar warn">
      <Icon name="wifi-off" size={14} />
      <span class="grow">
        {session.ended || 'The machine is offline.'}
        {canReattach && ' Disconnected shells stay available for five minutes. Reload to reattach.'}
      </span>
      <button type="button" class="btn sm" disabled title={canReattach ? 'Reload to reattach within five minutes.' : 'A closed shell cannot be restarted.'}>
        Reconnect
      </button>
    </div>}
    {!ended && session.readingElsewhere && <div class="term-bar warn">
      <Icon name="monitor" size={14} />
      <span class="grow">This terminal is open in another browser or tab. Reload to take back its output. Typing still works.</span>
    </div>}
    {notice && <div class="term-bar error" role="status">{notice}</div>}
    {state.sessions.map(item => <TerminalView key={item.id} session={item} active={item === session} sizeChanged={redraw} />)}
    <div class="term-keys only-phone-keys" role="group" aria-label="Extra keys">
      {keys.map(([label, key]) => <button type="button" aria-label={label} disabled={!!ended} aria-pressed={key === null ? !!session.ctrl : undefined} onPointerDown={e => e.preventDefault()} onClick={() => { if (key === null) { session.ctrl = !session.ctrl; redraw(); } else { write(key); session.terminal.focus(); } }}>
      {label}
    </button>)}
    </div>
    <div class="term-status">
      <span class="term-state"><i class={cx('dot', ended ? 'waiting' : 'working')} />{ended ? 'Ended' : 'Ready'}</span>
      <span class="term-cwd mono grow" title={session.worktree.folder}>{session.worktree.folder}</span>
      <span class="mono not-phone"><Icon name="git-branch" size={12} /> {session.worktree.branch}</span>
      <span class="mono num">{session.dim.cols}×{session.dim.rows}</span>
    </div>
  </div>;
}
