import * as chatLive from './live/chat';
import { live as liveMode, refuse } from './lib';
// Shared parts: avatars, menus, dialogs, the request block, the thread and the composer.
// These are built by hand, because the mock has no build step and so no component library.
import type { ComponentChildren, ComponentType, HTMLAttributes, VNode } from 'preact';
import { useState, useEffect, useLayoutEffect, useRef } from 'preact/hooks';
import {
  Icon, cx, ago, clock, dayLabel, useNow, useStore, update, go, href, useKey, HOUR, clamp, plural, isMac, uid, cameFrom,
} from './lib';
import {
  usableSubscription, agentOf, projectOf, teamOf, teamName, agentStatus, roleIcon, roleLabel, KIND_LABEL, HARNESSES, subOf, subLabel, machineOf,
  threadOf, openRequests, byId, threadRouteForRequest, routeForAgent, agentLabel, questionRoute, questionsView, taskPathById,
} from './data';
import * as act from './actions';
import type { Agent, Attachment, DialogState, Id, Request, RequestOption, State, Status, ThreadItem, ToolDetail as ToolDetailRecord } from './types';

// Shared help for settings. A tap pins the explanation until dismissal.
export interface SettingHelpProps { label: string; children?: ComponentChildren }
export function SettingHelp({ label, children }: SettingHelpProps) {
  const [id] = useState(() => uid('setting-help'));
  const [pinned, setPinned] = useState(false), [hovered, setHovered] = useState(false), [focused, setFocused] = useState(false);
  const root = useRef<HTMLSpanElement>(null), button = useRef<HTMLButtonElement>(null);
  const [dismissed, setDismissed] = useState(false);
  const open = !dismissed && (pinned || hovered || focused);
  const dismiss = () => { setPinned(false); setDismissed(true); };
  const escape = (event: KeyboardEvent) => { if (event.key === 'Escape') { event.stopPropagation(); button.current?.focus(); dismiss(); } };
  useEffect(() => {
    if (!open) return;
    const outside = (event: PointerEvent) => { if (!root.current?.contains(event.target as Node)) dismiss(); };
    document.addEventListener('pointerdown', outside);
    return () => { document.removeEventListener('pointerdown', outside); };
  }, [open]);
  return <span class="setting-help" ref={root} onKeyDown={escape} onFocusCapture={() => { setFocused(true); setDismissed(false); }} onBlurCapture={(event) => { if (!root.current?.contains(event.relatedTarget as Node)) setFocused(false); }} onMouseEnter={() => { setHovered(true); setDismissed(false); }} onMouseLeave={() => setHovered(false)}>
    <button ref={button} type="button" class="setting-help-button" aria-label={'About ' + label} aria-describedby={id} aria-expanded={open} onClick={() => { if (pinned) dismiss(); else { setPinned(true); setDismissed(false); } }}>
      ?
    </button>
    <span class="setting-help-panel" hidden={!open}>
      <span id={id} role="tooltip" class="setting-help-text" hidden={!open}>{children}</span>
      <button type="button" class="setting-help-close" aria-label={'Dismiss help for ' + label} onClick={() => { button.current?.focus(); dismiss(); }}>
        ×
      </button>
    </span>
  </span>;
}

// ---- identity ---------------------------------------------------------------
export interface AvatarProps { agent: Agent | null | undefined; s?: State; size?: string; status?: boolean }
export function Avatar({ agent, s, size, status = true }: AvatarProps) {
  if (!agent) return <span class={cx('avatar', size)}><Icon name="bot" size={12} /></span>;
  const st = status && s ? agentStatus(s, agent) : null;
  const lead = agent.role === 'project-lead';
  return <span class={cx('avatar', size, lead && 'lead')} title={roleLabel(agent)}>
    <Icon name={roleIcon(agent)} size={size === 'lg' ? 15 : size === 'sm' ? 12 : 13} />
    {st && <i class={cx('dot', st.dot!.replace(' pulse', ''))} />}
  </span>;
}
// The owner has no name on the website: one person uses it. The owner's mark is a plain figure.
export function OwnerAvatar({ size }: { s?: State; size?: string }) { return <span class={cx('avatar owner', size)} role="img" aria-label="You"><Icon name="user" size={size === 'sm' ? 11 : 13} /></span>; }
// "Builder 2 · Build team · Shell": the role first, then where it works.
export interface AgentLineProps { s: State; agent: Agent; link?: boolean }
export function AgentLine({ s, agent, link = true }: AgentLineProps) {
  const t = agent.team && teamOf(s, agent.team);
  const name = <b style="font-weight:var(--weight-medium)">{agent.name}</b>;
  return <span class="row" style="gap:6px">
    <Avatar agent={agent} s={s} size="sm" />
    {link ? <a class="truncate" href={href(routeForAgent(s, agent))}>{name}</a> : name}
    {t && <span class="muted truncate">{teamName(t)}</span>}
  </span>;
}

// ---- text ---------------------------------------------------------------------
// A safe Markdown subset: paragraphs, lists, fenced code, **bold**, `code`, and internal references.
function inline(text: string, pid: Id | undefined) {
  const out: (string | VNode)[] = []; let rest = text; let k = 0;
  const re = /(\*\*([^*]+)\*\*)|(`([^`]+)`)|(\[\[(\w+):([\w-]+)\|([^\]]+)\]\])/;
  while (rest) {
    const m = re.exec(rest);
    if (!m) { out.push(rest); break; }
    if (m.index) out.push(rest.slice(0, m.index));
    if (m[1]) out.push(<b key={k++} style="font-weight:var(--weight-strong)">{m[2]}</b>);
    else if (m[3]) out.push(<code key={k++}>{m[4]}</code>);
    else {
      const to = m[6] === 'task' ? taskPathById(pid!, m[7]!) : m[6] === 'sprint' ? `/p/${pid}/sprint/${m[7]}` : m[6] === 'tab' ? `/p/${pid}/${m[7]}`
        : m[6] === 'agent' ? `/p/${pid}/conversation/${m[7]}` : m[6] === 'post' ? `/p/${pid}/message-board/post/${m[7]}` : `/p/${pid}/board`;
      out.push(<a key={k++} class="ref" href={href(to)}>{m[8]}</a>);
    }
    rest = rest.slice(m.index + m[0].length);
  }
  return out;
}
// `onCommand` takes the text of a fenced shell block. With it, the block gets a button that pastes the command.
const SHELLS = ['sh', 'bash', 'zsh', 'shell', 'console'];
export interface RichProps { text: string | null | undefined; pid?: Id | undefined; onCommand?: ((command: string) => void) | undefined }
export function Rich({ text, pid, onCommand }: RichProps) {
  const lines = String(text).replace(/\r\n?/g, '\n').split('\n');
  const blocks: VNode[] = []; let i = 0;
  const list = (line: string) => /^(\s*)([-+*]|\d+[.)])\s+(.*)$/.exec(line);
  const fence = (line: string) => /^\s*(`{3,}|~{3,})(.*)$/.exec(line);
  const phrasing = (value: string) => value.split('\n').map((line, n) => [n > 0 && <br />, inline(line, pid)]);
  const readList = (indent: number): VNode => {
    const ordered = /^\d/.test(list(lines[i]!)![2]!); const entries: VNode[] = [];
    const start = ordered ? Number.parseInt(list(lines[i]!)![2]!, 10) : null;
    while (i < lines.length) {
      const match = list(lines[i]!);
      if (!match || match[1]!.length !== indent || /^\d/.test(match[2]) !== ordered) break;
      let value = match[3]!; i++; const children: VNode[] = [];
      while (i < lines.length && lines[i]!.trim()) {
        const next = list(lines[i]!);
        if (next) {
          if (next[1]!.length <= indent) break;
          children.push(readList(next[1]!.length));
        } else if (/^\s+/.test(lines[i]!)) { value += '\n' + lines[i]!.trim(); i++; }
        else break;
      }
      entries.push(<li>{phrasing(value)}{children}</li>);
    }
    return ordered ? <ol class="bul" start={start ?? undefined}>{entries}</ol> : <ul class="bul">{entries}</ul>;
  };
  while (i < lines.length) {
    if (!lines[i]!.trim()) { i++; continue; }
    const opening = fence(lines[i]!);
    if (opening) {
      const code: string[] = []; i++;
      while (i < lines.length && !(fence(lines[i]!)?.[1]![0] === opening[1]![0] && fence(lines[i]!)![1]!.length >= opening[1]!.length && !fence(lines[i]!)![2]!.trim())) code.push(lines[i++]!);
      i++; const pre = <pre class="code" tabindex={0}><code>{code.join('\n')}</code></pre>;
      blocks.push(onCommand && SHELLS.includes(opening[2]!.trim().toLowerCase())
        ? <div class="cmd">
          {pre}
          <button type="button" class="btn sm" onClick={() => onCommand(code.join('\n'))}>
            <Icon name="square-terminal" size={13} />
            Paste in terminal
          </button>
        </div> : pre);
      continue;
    }
    const match = list(lines[i]!);
    if (match) { blocks.push(readList(match[1]!.length)); continue; }
    const paragraph = [lines[i++]!];
    while (i < lines.length && lines[i]!.trim() && !list(lines[i]!) && !fence(lines[i]!)) paragraph.push(lines[i++]!);
    blocks.push(<p>{phrasing(paragraph.join('\n'))}</p>);
  }
  return blocks;
}

// ---- menus --------------------------------------------------------------------
// items: { label, icon, onSelect, checked, danger, disabled, why, sub, hint } | { sep: true } | { heading }
export interface MenuItem {
  label?: ComponentChildren; icon?: string | undefined; onSelect?: (() => unknown) | undefined; checked?: boolean | null | undefined; danger?: boolean | undefined;
  disabled?: boolean | undefined; why?: ComponentChildren; sub?: ComponentChildren; hint?: ComponentChildren; sep?: boolean; heading?: ComponentChildren;
}
export interface MenuProps {
  trigger: ComponentChildren; items: MenuItem[] | (() => MenuItem[]); align?: string; side?: string; class?: string | undefined; label?: string | undefined;
  width?: number | undefined; header?: ComponentChildren; disabled?: boolean | undefined;
  /** A tooltip on the button while the menu is closed. */
  tip?: string | undefined;
}
export function Menu({ trigger, items, align = 'start', side = 'bottom', class: cls, label, width, header, disabled = false, tip }: MenuProps) {
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<{ left: number; top: number } | null>(null);
  const [active, setActive] = useState(-1);
  const btn = useRef<HTMLButtonElement>(null); const menu = useRef<HTMLDivElement>(null);
  const list = typeof items === 'function' ? (open ? items() : []) : items;
  const place = () => {
    const r = btn.current!.getBoundingClientRect(); const m = menu.current!.getBoundingClientRect();
    let left = align === 'end' ? r.right - m.width : r.left;
    left = clamp(left, 8, innerWidth - m.width - 8);
    let top = side === 'top' ? r.top - m.height - 4 : r.bottom + 4;
    if (top + m.height > innerHeight - 8) top = Math.max(8, r.top - m.height - 4);
    if (top < 8) top = 8;
    setPos({ left, top });
  };
  const now = useRef({} as { list: MenuItem[]; active: number }); now.current = { list, active };
  useLayoutEffect(() => { if (open) place(); else { setPos(null); setActive(-1); } }, [open]);
  // The active item holds the focus, so a screen reader says it as the arrow keys move.
  useEffect(() => { if (open && active >= 0) menu.current?.querySelector<HTMLElement>('[data-active="true"]')?.focus({ preventScroll: true }); }, [open, active]);
  // The key listener is attached once for as long as the menu is open, so a fast second press is not lost.
  useEffect(() => {
    if (!open) return;
    const away = (e: PointerEvent) => { if (!menu.current?.contains(e.target as Node) && !btn.current?.contains(e.target as Node)) setOpen(false); };
    const key = (e: KeyboardEvent) => {
      const { list, active } = now.current;
      const idx = list.map((it, i) => (it.sep || it.heading || it.disabled ? -1 : i)).filter((i) => i >= 0);
      if (e.key === 'Escape') { e.stopPropagation(); setOpen(false); btn.current?.focus(); }
      else if (e.key === 'Tab') { btn.current?.focus(); setOpen(false); }
      else if (!idx.length) return;
      else if (e.key === 'ArrowDown') { e.preventDefault(); setActive((a) => idx[(idx.indexOf(a) + 1) % idx.length]!); }
      else if (e.key === 'ArrowUp') { e.preventDefault(); setActive((a) => (a < 0 ? idx[idx.length - 1]! : idx[(idx.indexOf(a) - 1 + idx.length) % idx.length]!)); }
      else if (e.key === 'Home') { e.preventDefault(); setActive(idx[0]!); }
      else if (e.key === 'End') { e.preventDefault(); setActive(idx[idx.length - 1]!); }
      else if (e.key === 'Enter' && active >= 0) { e.preventDefault(); const it = list[active]!; setOpen(false); btn.current?.focus(); it.onSelect?.(); }
    };
    addEventListener('pointerdown', away, true); addEventListener('keydown', key, true); addEventListener('resize', place);
    return () => { removeEventListener('pointerdown', away, true); removeEventListener('keydown', key, true); removeEventListener('resize', place); };
  }, [open]);
  return <>
    <button ref={btn} type="button" class={cls} aria-haspopup="menu" aria-expanded={open} aria-label={label} data-tip={!open && tip ? tip : undefined} disabled={disabled} onClick={(e) => { e.stopPropagation(); setOpen(!open); }}>
      {trigger}
    </button>
    {open && <div ref={menu} class="menu" role="menu" style={{ left: (pos?.left ?? -9999) + 'px', top: (pos?.top ?? -9999) + 'px', width: width ? width + 'px' : null }} onClick={(e) => e.stopPropagation()}>
      {header}
      {list.map((it, i) => it.sep ? <div class="menu-sep" key={i} />
        : it.heading ? <div class="menu-label" key={i}>{it.heading}</div>
        : <button key={i} type="button" role={it.checked != null ? 'menuitemradio' : 'menuitem'} aria-checked={it.checked ?? undefined} disabled={it.disabled} class={cx('menu-item', it.danger && 'danger', !!(it.hint || it.why) && 'two')} data-active={active === i} onMouseEnter={() => setActive(i)} onClick={() => { setOpen(false); it.onSelect?.(); }}>
          {it.hint || it.why ? <>
              <span class="row">{it.icon && <Icon name={it.icon} size={14} />}<span>{it.label}</span></span>
              <small>{it.why || it.hint}</small>
            </>
              : <>
                {it.icon && <Icon name={it.icon} size={14} />}
                <span class="grow truncate">{it.label}</span>
                {it.sub && <span class="sub">{it.sub}</span>}
                {it.checked && <Icon name="check" size={14} />}
              </>}
        </button>)}
    </div>}
  </>;
}

// ---- dialogs ------------------------------------------------------------------
export interface DialogProps {
  title: string; onClose: () => void; children?: ComponentChildren; foot?: ComponentChildren; size?: string | undefined;
  onSubmit?: ((e: Event) => unknown) | undefined;
}
export function Dialog({ title, onClose, children, foot, size, onSubmit }: DialogProps) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const prev = document.activeElement as HTMLElement | null;
    // The page behind a dialog is inert, so the keyboard and a screen reader stay inside the dialog.
    const held: HTMLElement[] = [];
    for (let n = ref.current!.parentElement; n?.parentElement && n !== document.body; n = n.parentElement)
      for (const sib of n.parentElement.children as HTMLCollectionOf<HTMLElement>) if (sib !== n && !sib.inert && sib.getAttribute('aria-live') == null) { sib.inert = true; held.push(sib); }
    const el = ref.current!.querySelector<HTMLElement>('[autofocus], input:not([type=hidden]), textarea, select, .dialog-foot .btn.primary, button');
    el?.focus();
    return () => { for (const h of held) h.inert = false; prev?.focus?.(); };
  }, []);
  const key = (e: KeyboardEvent) => {
    if (e.key === 'Escape') { e.stopPropagation(); onClose(); }
    if (e.key === 'Tab') {
      const f = [...ref.current!.querySelectorAll<HTMLElement>('button:not([disabled]), input, textarea, select, a[href], [tabindex="0"]')];
      if (!f.length) return;
      const first = f[0]!, last = f[f.length - 1]!;
      if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
      else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
    }
  };
  // A form or a div. Both take the same attributes here, so the type names one of them.
  const Tag = (onSubmit ? 'form' : 'div') as 'div';
  return <div class="scrim" onPointerDown={(e) => { if (e.target === e.currentTarget) onClose(); }}>
    <Tag ref={ref} class={cx('dialog', size)} role="dialog" aria-modal="true" aria-label={title} onKeyDown={key} onSubmit={onSubmit && ((e) => { e.preventDefault(); onSubmit(e); })}>
      <div class="dialog-head">
        <h2>{title}</h2>
        <button type="button" class="icon-btn" aria-label="Close" onClick={onClose}><Icon name="x" /></button>
      </div>
      <div class="dialog-body">{children}</div>
      {foot && <div class="dialog-foot">{foot}</div>}
    </Tag>
  </div>;
}
/** A dialog drawn by DialogHost for the open dialog of its type. */
export type DialogComponent = ComponentType<{ d: DialogState; s: State }>;
const dialogTypes: Record<string, DialogComponent> = {};
export function registerDialog(type: string, component: DialogComponent): void { dialogTypes[type] = component; }
function ConfirmDialog({ d }: { d: DialogState }) {
  const [pending, setPending] = useState(false);
  const close = () => { if (!pending) act.closeDialog(); };
  const confirm = async () => {
    if (pending) return;
    if (!d.waitForReply) { act.closeDialog(); d.onConfirm?.(); return; }
    setPending(true);
    try { if (await d.onConfirm?.()) act.closeDialog(); } finally { setPending(false); }
  };
  return <Dialog title={d.title} size="narrow" onClose={close} foot={<>
      <button type="button" class="btn" disabled={pending} onClick={close}>Cancel</button>
      {d.second && <button type="button" class="btn danger" disabled={pending} onClick={() => { act.closeDialog(); d.second.onConfirm(); }}>{d.second.action}</button>}
      <button type="button" class={cx('btn', d.danger ? 'danger' : 'primary')} aria-busy={pending} disabled={pending} onClick={confirm}>
        {pending ? d.pendingLabel || 'Working…' : d.action}
      </button>
    </>}>
    {d.body && <p style="line-height:1.55">{d.body}</p>}
    {d.lost?.length > 0 && <ul class="col" style="gap:4px">
      {d.lost.map((l: string) => <li class="row"><Icon name="triangle-alert" size={14} class="warn-text" /><span>{l}</span></li>)}
    </ul>}
  </Dialog>;
}
function RenameDialog({ d }: { d: DialogState }) {
  const [v, setV] = useState<string>(d.value); const taken = d.taken?.includes(v.trim()) && v.trim() !== d.value;
  const bad = !v.trim() ? 'A name is needed.' : taken ? 'That name is already in use.' : null;
  const [pending, setPending] = useState(false);
  const close = () => { if (!pending) act.closeDialog(); };
  const rename = async () => {
    if (bad || pending) return;
    if (!d.waitForReply) { act.closeDialog(); d.onRename(v.trim()); return; }
    setPending(true);
    try { if (await d.onRename(v.trim())) act.closeDialog(); } finally { setPending(false); }
  };
  return <Dialog title={d.title || 'Rename'} size="narrow" onClose={close} onSubmit={rename} foot={<>
      <button type="button" class="btn" disabled={pending} onClick={close}>Cancel</button>
      <button type="submit" class="btn primary" disabled={!!bad || pending} aria-busy={pending}>{pending ? 'Renaming…' : 'Rename'}</button>
    </>}>
    <div class="field">
      <label for="rename-field">Name</label>
      <input id="rename-field" class="input" disabled={pending} value={v} onInput={(e) => setV((e.target as HTMLInputElement).value)} autofocus />
      {bad && v !== d.value && <span class="error">{bad}</span>}
    </div>
    {d.note && <p class="muted" style="font-size:var(--text-sm)">{d.note}</p>}
  </Dialog>;
}
registerDialog('confirm', ConfirmDialog);
registerDialog('rename', RenameDialog);
export function DialogHost({ s }: { s: State }) {
  if (!s.dialog) return null;
  const C = dialogTypes[s.dialog.type];
  return C ? <C d={s.dialog} s={s} /> : null;
}
export function Toasts({ s }: { s: State }) {
  return <div class="toasts" role="status" aria-live="polite">
    {s.toasts.map((t) => <div key={t.id} class={cx('toast', t.tone === 'error' && 'error')}>
    <Icon name={t.tone === 'error' ? 'circle-alert' : 'circle-check'} size={15} />
    <span>{t.text}</span>
  </div>)}
  </div>;
}

// ---- resizing -----------------------------------------------------------------
// With `onTap`, a press that does not move is a click: the sidebar's edge folds on a click and resizes on a drag.
// With `onUnder`, a drag to half the narrowest width hands back the width the drag started from.
export type ResizerProps<R extends number | null> = Omit<HTMLAttributes<HTMLDivElement>, 'onChange' | 'label' | 'value' | 'min' | 'max' | 'class'> & {
  side: string; value: number; min: number; max: number; onChange: (v: number | R) => void; reset: R; label: string;
  onTap?: (() => void) | undefined; onUnder?: ((v: number) => void) | undefined; class?: string | undefined; children?: ComponentChildren;
};
export function Resizer<R extends number | null>({ side, value, min, max, onChange, reset, label, onTap, onUnder, class: cls, children, ...rest }: ResizerProps<R>) {
  const [drag, setDrag] = useState(false);
  const start = (e: PointerEvent) => {
    e.preventDefault();
    const x0 = e.clientX, v0 = value; let moved = !onTap;
    if (moved) setDrag(true);
    const move = (ev: PointerEvent) => {
      if (!moved && Math.abs(ev.clientX - x0) < 4) return;
      if (!moved) { moved = true; setDrag(true); }
      const raw = v0 + (side === 'right' ? ev.clientX - x0 : x0 - ev.clientX);
      if (onUnder && raw < min / 2) { end(); onUnder(v0); return; }
      onChange(clamp(raw, min, max));
    };
    const end = () => { setDrag(false); removeEventListener('pointermove', move); removeEventListener('pointerup', up); };
    const up = () => { end(); if (!moved) onTap!(); };
    addEventListener('pointermove', move); addEventListener('pointerup', up);
  };
  const key = (e: KeyboardEvent) => {
    const step = side === 'right' ? 16 : -16;
    if (e.key === 'ArrowRight') onChange(clamp(value + step, min, max));
    else if (e.key === 'ArrowLeft') onChange(clamp(value - step, min, max));
    else if (e.key === 'Home') onChange(reset);
    else if (e.key === 'Enter' && onTap) onTap();
  };
  return <div {...rest} class={cx('resizer', side, cls, drag && 'dragging')} role="separator" aria-orientation="vertical" aria-label={label} aria-valuenow={value} tabindex={0} onPointerDown={start} onKeyDown={key} onDblClick={onTap ? undefined : () => onChange(reset)} title={onTap ? '' : 'Drag to resize. Double-click to reset.'}>
    {children}
  </div>;
}

// The drag handle of a view's own list, which sits on the far right of a project view in the mock.
// One width serves the list of Git and the tree of Files, and the project's shell hands it down as
// `--list-w`. `fallback` is the width before any drag.
export function ListResizer({ s, fallback }: { s: State; fallback: number }) {
  if (liveMode.on) return null;
  return <Resizer side="left" class="at-list" label="Resize the list" value={s.prefs.listWidth ?? fallback} min={200} max={520} reset={null} onChange={(v) => act.setPref('listWidth', v)} />;
}

// ---- the request block ----------------------------------------------------------
// One block for questions, approvals, merges and failed starts. It has two shapes:
// "card" in a thread, with its own heading, and "detail" inside an expanded Home row,
// where the row already shows the title, the agent and the wait.
// `openThread` is an address or a function. It is left out when the block already sits
// in the asking agent's own thread.
// A question or an approval in a thread starts as a brief: what is asked and how long it
// has waited, one line of description, and the answer it needs. Details adds a short report
// under the brief without replacing it: who asked, the full question, what it is about, why
// it matters, the other answers, and the references.
export function Cited({ text }: { text: string }) {
  return text.split(/(\[\d+\]|`[^`]+`)/).map((t, i) => /^\[\d+\]$/.test(t) ? <sup class="qc-cite" key={i}>{t}</sup>
    : /^`/.test(t) ? <code key={i}>{t.slice(1, -1)}</code> : t.replace(/ $/, ' '));
}
export function Paras({ list }: { list: string | string[] | null | undefined }) { return (Array.isArray(list) ? list : [list]).filter(Boolean).map((t, i) => <p key={i}><Cited text={t!} /></p>); }
// A command or a folder in a code box. A long one scrolls inside the box, and the button in
// its corner copies the whole text.
export interface CodeBoxProps { text?: string | null | undefined; label?: string; copyText?: string | null | undefined; children?: ComponentChildren; copyLabel?: string | undefined }
export function CodeBox({ text, label, copyText = text, children, copyLabel }: CodeBoxProps) {
  const [copied, setCopied] = useState(false);
  const copy = () => { navigator.clipboard?.writeText(copyText || ''); setCopied(true); setTimeout(() => setCopied(false), 1500); };
  return <div class="qc-code">
    <pre class="code" tabindex={0}>{children ?? text}</pre>
    <button type="button" class="qc-copy" aria-label={copyLabel ? (copied ? 'Copied ' : 'Copy ') + copyLabel : (copied ? 'Copied the ' : 'Copy the ') + label} title={copied ? 'Copied' : 'Copy'} onClick={copy}>
      <Icon name={copied ? 'check' : 'copy'} size={14} />
    </button>
  </div>;
}
// What an answered card says about its answer: a short label, and the owner's words when the
// answer was typed.
export interface RequestOutcome { label: string; text: string; icon: string; tone: string; typed?: boolean }
export function requestOutcome(r: Request, rec: RequestOption | null | undefined, shown: string | null | undefined): RequestOutcome {
  const a = (r.answer || '').trim();
  if (r.status === 'cancelled') return { label: 'Cancelled', text: a && a !== 'Cancelled' ? a : '', icon: 'x', tone: 'off' };
  if (r.kind !== 'question') {
    const byKind = ({ allow_once: 'Allowed', allow_always: 'Allowed, always', reject_once: 'Denied', reject_always: 'Denied, always' } as Record<string, string>)[r.answerKind!];
    const label = byKind || a || 'Answered'; const no = /^(denied|sent back)/i.test(label);
    return { label, text: '', icon: no ? 'x' : 'check', tone: no ? 'no' : 'yes' };
  }
  // The accepted answer is named when the card does not already show the same words.
  if (rec && a === rec.label) return { label: 'Accepted', text: a === (shown || '').trim() ? '' : a, icon: 'check', tone: 'yes' };
  return { label: 'Answer', text: a, typed: true, icon: 'check', tone: 'yes' };
}
// Where the shell has a Questions view, a card has no report of its own. Its Details opens the
// question or the approval in that view. Deny on a question opens it there too.
/** Where "Open thread" leads: an address, or a function that opens the thread in place. */
export type OpenThread = string | (() => void) | null | undefined;
interface CardProps { s: State; r: Request; openThread?: OpenThread; who?: boolean }
function BriefCard({ s, r, openThread, who }: CardProps) {
  const now = useNow(); const a = agentOf(s, r.agent); const isQ = r.kind === 'question';
  const toView = (isQ || r.kind === 'approval') && questionsView.on && !liveMode.on;
  const [open, setOpen] = useState(() => !toView && s.ui.panelReq === r.id);
  const [declined, setDeclined] = useState(false);
  const [text, setText] = useState(''); const own = useRef<HTMLInputElement>(null);
  const live = r.status === 'open' && (!r.heldBy || liveMode.on && r.kind === 'approval' && r.canAnswer); const waited = now - r.createdAt;
  const approvalEffect = r.projectApprovalEffect;
  const approvalOutcome = r.projectApprovalOutcome || (r.answer === 'Accepted' ? 'Approval recorded.' : 'Approval not recorded. Ask again if you still need it.');
  const opts = r.options || [];
  const rec = isQ ? opts.find((o) => o.primary) || null : null;
  const heading = r.brief || r.title;
  const brief = isQ && r.body && (heading === r.body || r.body.startsWith(heading + '\n')) ? 'Question' : heading;
  const summary = r.summary && r.summary !== r.body ? r.summary : null;
  const recShort = isQ ? r.recShort || r.recommendation : null; const id = 'qc-' + r.id;
  // Two cards on one page have buttons of the same name. Each button is described by its card's question.
  const about = 'qq-' + r.id;
  // The answers on the brief, and the rest in the report.
  const others = isQ ? opts.filter((o) => o !== rec) : [];
  // An approval answers with Allow and Deny. Each has a menu that makes the answer standing.
  const answer = (o: string) => act.answerRequest(r.id, o);
  // The default icon is a check for Allow and a cross for Deny. The sparkles icon replaces it
  // only on an action the agent recommends.
  const split = (label: string, icon: string, always: string, o: string) => <span class="split">
    <button type="button" class="btn" aria-describedby={about} onClick={() => answer(o)}><Icon name={icon} size={14} />{label}</button>
    <Menu class="btn split-more" label={'More ways to ' + label.toLowerCase()} trigger={<Icon name="chevron-down" size={14} />} items={[{ label: always, onSelect: () => answer(o === 'deny' ? 'never' : 'always') }]} />
  </span>;
  const deny = () => { if (toView) { act.openQuestion(r, true); return; } setDeclined(true); setOpen(true); requestAnimationFrame(() => own.current?.focus()); };
  // The live website answers with the options the harness sent. The one-time answer is the
  // button. A standing answer sits in its menu, with the harness's own wording as the note.
  const liveSplit = (label: string, icon: string, always: string, onceKind: string, alwaysKind: string) => {
    const once = opts.find((o) => o.kind === onceKind); if (!once) return null;
    const standing = opts.filter((o) => o.kind === alwaysKind); const busy = chatLive.requestBusy(r);
    return <span class="split">
      <button type="button" class="btn" disabled={busy} title={once.label} aria-describedby={about} onClick={() => answer(once.id)}>
        <Icon name={icon} size={14} />
        {label}
      </button>
      {standing.length > 0 && <Menu class="btn split-more" label={'More ways to ' + label.toLowerCase()} trigger={<Icon name="chevron-down" size={14} />} items={standing.map((o) => ({ label: always, why: o.label, disabled: busy, onSelect: () => answer(o.id) }))} width={320} />}
    </span>;
  };
  if (liveMode.on && r.status === 'open' && r.heldBy && !live) return <section class="req qc" id={'req-' + r.id}>
    <b>{brief}</b>
    <Rich text={r.body} pid={r.project} />
    {recShort && <p class="qc-rec"><Rich text={recShort} pid={r.project} /></p>}
    <p class="muted">Waiting on {agentOf(s, r.heldBy).name}</p>
  </section>;
  // An answered card keeps everything it showed. The answer buttons leave, and one line says
  // what the answer was.
  const done = r.status !== 'open';
  if (!live && !done) return <section class="req qc done" id={'req-' + r.id} tabindex={-1} aria-label={'Held: ' + brief}>
    <div class="qc-top"><b class="qc-q">{brief}</b></div>
  </section>;
  // Read only when the request is done.
  const outcome = (done ? requestOutcome(r, rec, null) : null)!;
  const thread = openThread && !toView && (typeof openThread === 'string'
    ? <a class="btn ghost qc-thread" href={href(openThread)} aria-label="Open thread" title="Open thread" aria-describedby={about}>
      <Icon name="message-square" size={14} />
      <span class="qc-thread-word">Open thread</span>
    </a>
    : <button type="button" class="btn ghost qc-thread" aria-label="Open thread" title="Open thread" aria-describedby={about} onClick={openThread}>
      <Icon name="message-square" size={14} />
      <span class="qc-thread-word">Open thread</span>
    </button>);
  const report = <div class="qc-report" id={id}>
    <p class="qc-meta">
      {who && <><Avatar agent={a} s={s} size="sm" status={false} /><span>Asked by {agentLabel(s, a, { withTeam: true })}</span></>}
      {r.blocks && <span>Blocks {r.blocks}</span>}
      {!isQ && r.tool && <span>Tool {r.tool}</span>}
    </p>
    {isQ && summary && r.body && <section class="qc-part q"><h5>Question</h5><Rich text={r.body} pid={r.project} /></section>}
    {!isQ && <section class="qc-part">
        <h5>Command</h5>
        {r.command ? <CodeBox text={r.command} label="command" /> : <p class="flat">Hidden by the harness. You cannot see this tool's input.</p>}
        <h5 class="qc-sub">Folder</h5>
        <CodeBox text={r.folder} label="folder" />
      </section>}
    {r.what && <section class="qc-part"><h5>Background</h5><Paras list={r.what} /></section>}
    {(r.whyText || r.why) && <section class="qc-part why"><h5>Impact</h5><Paras list={r.whyText || r.why} /></section>}
    {r.recWhy && <section class="qc-part reco"><h5><Icon name="sparkles" size={13} />Rationale</h5><Paras list={r.recWhy} /></section>}
    {(others.length > 0 || (isQ && !done)) && <section class="qc-part">
        <h5>Other answers</h5>
        <ul class="qc-opts">
          {others.map((o) => <li key={o.id}>
            <div><b>{o.label}</b>{o.note && <p>{o.note}</p>}</div>
            {!done && <button disabled={liveMode.on && chatLive.requestBusy(r)} type="button" class="btn sm" onClick={() => act.answerRequest(r.id, o.id)}>
              Choose
            </button>}
          </li>)}
          {isQ && !done && <li class="own">
            {r.projectApproval && text.trim() && text.trim() !== 'Accepted' && <p role="status">This answer does not approve. Use Accept to grant approval.</p>}
            <form onSubmit={(e) => { e.preventDefault(); if (text.trim()) act.answerRequest(r.id, null, text.trim()); }}>
              <input ref={own} class="input" placeholder="Your own answer" aria-label="Your own answer" value={text} onInput={(e) => setText((e.target as HTMLInputElement).value)} />
              <button type="submit" class="btn sm" disabled={!text.trim() || (liveMode.on && chatLive.requestBusy(r))}>Send</button>
            </form>
          </li>}
        </ul>
      </section>}
    {r.refs?.length! > 0 && <section class="qc-part refs">
        <h5>References</h5>
        <ol>{r.refs!.map((f, i) => <li key={f}><span class="qc-cite">[{i + 1}]</span><span class="mono">{f}</span></li>)}</ol>
      </section>}
  </div>;
  return <section aria-busy={liveMode.on && chatLive.requestBusy(r)} class={cx('req qc', r.kind, open && 'open', done && 'done')} id={'req-' + r.id} tabindex={-1} aria-label={(done ? 'Answered: ' : isQ ? 'Question: ' : 'Approval: ') + brief}>
    {liveMode.on && r.kind === 'approval' && r.status === 'open' && r.heldBy && <p class="muted">Waiting on {agentOf(s, r.heldBy).name}</p>}
    {approvalEffect && <p class="qc-sum">{approvalEffect}</p>}
    <div class="qc-top">
      <b class="qc-q" id={about}>{brief}</b>
      {done ? <span class="qc-age">{clock(isQ ? r.createdAt : r.answeredAt!)}</span> : <span class={cx('qc-age', waited >= HOUR && 'long')}>{ago(r.createdAt, now)}</span>}
    </div>
    {summary && <div class="qc-sum"><Rich text={summary} pid={r.project} /></div>}
    {isQ && r.body && !summary && r.body !== brief && <div class="qc-sum"><Rich text={r.body} pid={r.project} /></div>}
    {recShort && <p class={cx('qc-rec', declined && 'declined')}>
      <Icon name="sparkles" size={14} class="qc-star" />
      <span>{recShort}</span>
      {declined && <span class="qc-tag">Declined</span>}
    </p>}
    {open && report}
    {done && isQ && r.answeredAt != null && <p class="qc-meta"><span>Answered at <time>{clock(r.answeredAt)}</time></span></p>}
    {done && r.projectApproval && <p role="status">{approvalOutcome}</p>}
    <div class="req-actions qc-actions">
      {done && <>
        {(!isQ || rec) && <button type="button" disabled class={cx('btn', outcome.tone === 'yes' && !outcome.typed && 'chosen yes')}>
        <Icon name={isQ ? 'sparkles' : 'check'} size={14} />
        {outcome.tone === 'yes' && !outcome.typed ? outcome.label : isQ ? 'Accept' : 'Allow'}
      </button>}
        {(!isQ || rec) && <button type="button" disabled class={cx('btn', outcome.tone === 'no' && 'chosen no')}>
        <Icon name="x" size={14} />
        {outcome.tone === 'no' ? outcome.label : 'Deny'}
      </button>}
        {(outcome.text || outcome.tone === 'off' || (isQ && !rec)) && <span class={cx('qc-outcome', outcome.tone)}>
        {(outcome.typed || outcome.tone === 'off' || !rec) && <b>{outcome.label}</b>}
        {outcome.text && <span>{outcome.text}</span>}
      </span>}
      </>}
      {!done && <>
        {!isQ && liveMode.on && liveSplit('Allow', 'check', 'Always allow', 'allow_once', 'allow_always')}
        {!isQ && liveMode.on && liveSplit('Deny', 'x', 'Always deny', 'reject_once', 'reject_always')}
        {!isQ && liveMode.on && opts.filter((o) => !['allow_once', 'allow_always', 'reject_once', 'reject_always'].includes(o.kind!)
        || (o.kind === 'allow_always' && !opts.some((x) => x.kind === 'allow_once')) || (o.kind === 'reject_always' && !opts.some((x) => x.kind === 'reject_once')))
        .map((o) => <button key={o.id} disabled={chatLive.requestBusy(r)} type="button" class="btn" onClick={() => answer(o.id)}>{o.label}</button>)}
        {rec && !declined && <button disabled={liveMode.on && chatLive.requestBusy(r)} type="button" class="btn" aria-describedby={about} onClick={() => act.answerRequest(r.id, rec.id)}>
        <Icon name="sparkles" size={14} class="qc-star" />
        Accept
      </button>}
        {rec && !declined && <button type="button" class="btn" aria-describedby={about} onClick={deny}><Icon name="x" size={14} />Deny</button>}
        {!isQ && !liveMode.on && split('Allow', 'check', 'Always allow', 'allow')}
        {!isQ && !liveMode.on && split('Deny', 'x', 'Always deny', 'deny')}
      </>}
      {toView && <a class="btn" href={href(questionRoute(r))} aria-describedby={about}>Details<Icon name="chevron-right" size={14} /></a>}
      {!toView && <button type="button" class="btn" aria-expanded={open} aria-controls={id} aria-describedby={about} onClick={() => setOpen(!open)}>
        {open ? 'Hide details' : 'Details'}
        <Icon name={open ? 'chevron-up' : 'chevron-down'} size={14} />
      </button>}
      {thread}
    </div>
  </section>;
}

// ---- merge brief --------------------------------------------------------------------
// A merge reads as a brief, the reason it needs the owner, one row of evidence, and the
// buttons. The same row of evidence sits in the merge review dialog.
export function MergeFacts({ s, r }: { s: State; r: Pick<Request, 'evidence' | 'source' | 'target'> }) {
  const ev = r.evidence!; const rv = ev.review.agent && agentOf(s, ev.review.agent);
  const cell = (k: string, tone: string | null, value: ComponentChildren, sub: ComponentChildren) => <div key={k} class={cx(tone === 'warn' && 'warn')}>
    <dt>{k}</dt>
    <dd>{tone && <Icon name={tone === 'ok' ? 'circle-check' : 'triangle-alert'} size={13} />}<span>{value}</span></dd>
    {sub && <dd class="sub">{sub}</dd>}
  </div>;
  const cf = ev.conflicts;
  return <dl class="mg-facts">
    {cell('Tests', ev.tests.ok ? 'ok' : 'warn', ev.tests.value || (ev.tests.ok ? 'Passed' : 'Not run'), ev.tests.sub)}
    {cell('Review', ev.review.ok ? 'ok' : 'warn', ev.review.value || (ev.review.ok ? 'Clean' : 'Not finished'), rv ? rv.name : ev.review.sub)}
    {cell('Changes', null, `${plural(ev.commits, 'commit')} · ${plural(ev.files, 'file')}`, ev.add != null && <><span class="success-text">+{ev.add}</span> <span class="error-text">−{ev.del}</span></>)}
    {cell('Into ' + r.target, cf == null ? null : cf ? 'warn' : 'ok', cf == null ? 'Not checked' : cf ? plural(cf, 'conflict') : 'No conflicts', <span class="mono">{r.source}</span>)}
  </dl>;
}
function MergeBrief({ s, r, openThread }: CardProps) {
  const [back, setBack] = useState(false); const [why, setWhy] = useState(''); const box = useRef<HTMLInputElement>(null);
  const live = r.status === 'open' && !r.heldBy;
  const thread = openThread && (typeof openThread === 'string'
    ? <a class="btn ghost qc-thread" href={href(openThread)} aria-label="Open thread" title="Open thread">
      <Icon name="message-square" size={14} />
      <span class="qc-thread-word">Open thread</span>
    </a>
    : <button type="button" class="btn ghost qc-thread" aria-label="Open thread" title="Open thread" onClick={openThread}>
      <Icon name="message-square" size={14} />
      <span class="qc-thread-word">Open thread</span>
    </button>);
  const send = (e: Event) => { e.preventDefault(); if (why.trim()) act.answerRequest(r.id, 'decline', why.trim()); };
  return <section aria-busy={liveMode.on && chatLive.requestBusy(r)} class={cx('req mg', !live && 'closed')} id={'req-' + r.id} tabindex={-1} aria-label={'Merge: ' + r.title}>
    <p class="mg-brief">{r.brief || r.body}</p>
    {r.notice && <p class="mg-notice"><b>Users will notice:</b> {r.notice}</p>}
    <MergeFacts s={s} r={r} />
    {live && !back && <div class="req-actions qc-actions">
      <button type="button" class="btn primary" onClick={() => act.openDialog({ type: 'merge-review', requestId: r.id })}>
        <Icon name="git-merge" size={14} />
        Review and merge
      </button>
      <button type="button" class="btn" onClick={() => { setBack(true); requestAnimationFrame(() => box.current?.focus()); }}>
        <Icon name="undo-2" size={14} />
        Send back
      </button>
      {thread}
    </div>}
    {live && back && <form class="mg-back" onSubmit={send}>
      <label for={'why-' + r.id}>Requested change</label>
      <div class="row">
        <input id={'why-' + r.id} ref={box} class="input" value={why} onInput={(e) => setWhy((e.target as HTMLInputElement).value)} placeholder="The redirects should keep the old anchors" />
        <button type="submit" class="btn" disabled={!why.trim()}><Icon name="undo-2" size={14} />Send back</button>
        <button type="button" class="btn ghost" onClick={() => { setBack(false); setWhy(''); }}>Cancel</button>
      </div>
    </form>}
    {!live && <div class="answer">
      <Icon name="check" size={14} />
      <span>{r.answer} at {clock(r.answeredAt!)}{r.reason ? ': ' + r.reason : ''}</span>
    </div>}
  </section>;
}

export interface RequestBlockProps { s: State; request: Request; variant?: string; openThread?: OpenThread; primary?: boolean; who?: boolean }
export function RequestBlock({ s, request: r, variant = 'card', openThread = null, primary = false, who = true }: RequestBlockProps) {
  if (variant === 'card' && (r.kind === 'question' || r.kind === 'approval')) return <BriefCard s={s} r={r} openThread={openThread} who={who} />;
  if (variant === 'detail' && r.kind === 'merge') return <MergeBrief s={s} r={r} openThread={openThread} />;
  const now = useNow();
  const a = agentOf(s, r.agent); const p = projectOf(s, r.project);
  const [text, setText] = useState('');
  const open = r.status === 'open' && (!r.heldBy || liveMode.on && r.kind === 'approval' && r.canAnswer);
  const waited = now - r.createdAt;
  const fill = (yes: boolean | undefined) => cx('btn', yes && primary && 'primary');
  const thread = openThread && (typeof openThread === 'string'
    ? <a class="btn ghost" href={href(openThread)}><Icon name="message-square" size={14} />Open thread</a>
    : <button type="button" class="btn ghost" onClick={openThread}><Icon name="message-square" size={14} />Open thread</button>);
  const approvalEffect = r.projectApprovalEffect;
  const approvalOutcome = r.projectApprovalOutcome || (r.answer === 'Accepted' ? 'Approval recorded.' : 'Approval not recorded. Ask again if you still need it.');
  const opts = r.options || [];
  const rec = r.kind === 'question' && (r.recommendation || liveMode.on) ? opts.find((o) => o.primary) : null;
  const sub = a.failure && subOf(s, a.failure.subscription);
  const fixed = r.kind === 'failed' && usableSubscription(sub);
  return <section aria-busy={liveMode.on && chatLive.requestBusy(r)} class={cx('req', variant, r.kind === 'failed' && 'failed', !open && 'closed')} id={'req-' + r.id} tabindex={-1} aria-label={KIND_LABEL[r.kind] + ': ' + r.title}>
    {liveMode.on && r.kind === 'approval' && r.status === 'open' && r.heldBy && <p class="muted">Waiting on {agentOf(s, r.heldBy).name}</p>}
    {approvalEffect && <p class="qc-sum">{approvalEffect}</p>}
    {variant === 'card' && <>
      <div class="req-head">
        <span class={cx('kind', open ? r.kind : 'closed')}>{KIND_LABEL[r.kind]}</span>
        {who && <span class="req-who"><Avatar agent={a} s={s} size="sm" status={false} />{agentLabel(s, a, { withTeam: true })}</span>}
        <span class={cx('age', open && waited >= HOUR && 'long')}>
          {open ? ago(r.createdAt, now) : r.answer + ' · ' + clock(r.answeredAt!)}
        </span>
      </div>
      <h4>{r.title}</h4>
    </>}
    {r.body && <div class="req-body">{variant === 'detail' && <span class="req-label">Description</span>}<p>{r.body}</p></div>}
    {r.recommendation && <p class="rec"><Icon name="sparkles" size={14} /><span><b>Recommended:</b> {r.recommendation}</span></p>}
    {r.kind === 'approval' && <dl class="kv">
      <dt>Command</dt>
      <dd>{r.command || <span class="muted" style="font-family:var(--font-body)">Hidden by the harness</span>}</dd>
      <dt>Folder</dt>
      <dd>{r.folder}</dd>
    </dl>}
    {r.kind === 'merge' && <div class="evidence">
      <div class={cx('ev', !r.evidence!.tests.ok && 'missing')}>
        <Icon name={r.evidence!.tests.ok ? 'circle-check' : 'triangle-alert'} size={14} />
        <span>{r.evidence!.tests.text}</span>
      </div>
      <div class={cx('ev', !r.evidence!.review.ok && 'missing')}>
        <Icon name={r.evidence!.review.ok ? 'circle-check' : 'triangle-alert'} size={14} />
        <span>{r.evidence!.review.text}</span>
      </div>
      <div class="ev">
        <Icon name="git-commit-horizontal" size={14} class="muted" />
        <span class="mono">{r.source}</span>
        <Icon name="arrow-right" size={12} class="muted" />
        <span class="mono">{r.target}</span>
        <span class="muted">{r.evidence!.commits} commits · {r.evidence!.files} files</span>
      </div>
    </div>}
    {open && r.kind === 'question' && <div class="req-actions">
      {rec && <button disabled={liveMode.on && chatLive.requestBusy(r)} type="button" class={fill(true)} onClick={() => act.answerRequest(r.id, rec.id)}>
        <Icon name="check" size={14} />
        {r.projectApproval ? 'Accept' : 'Accept recommendation'}
      </button>}
      {opts.filter((o) => o !== rec).map((o) => <button disabled={liveMode.on && chatLive.requestBusy(r)} key={o.id} type="button" class="btn" onClick={() => act.answerRequest(r.id, o.id)}>
        {o.label}
      </button>)}
      {r.projectApproval && text.trim() && text.trim() !== 'Accepted' && <p role="status">This answer does not approve. Use Accept to grant approval.</p>}
      <form aria-busy={liveMode.on && chatLive.requestBusy(r)} class="row grow" style="flex:1 1 200px" onSubmit={(e) => { e.preventDefault(); if (text.trim()) act.answerRequest(r.id, null, text.trim()); }}>
        <input class="input" placeholder="Another answer" aria-label="Another answer" value={text} onInput={(e) => setText((e.target as HTMLInputElement).value)} />
        {text.trim() && <button disabled={liveMode.on && chatLive.requestBusy(r)} class="btn" type="submit">Send</button>}
      </form>
      {thread}
    </div>}
    {open && r.kind === 'approval' && <div class="req-actions">
      {opts.map((o) => <button disabled={liveMode.on && chatLive.requestBusy(r)} key={o.id} type="button" class={cx(fill(o.primary && !r.blind), o.id === 'deny' && 'ghost')} onClick={() => act.answerRequest(r.id, o.id)}>
        {o.label}
      </button>)}
      {thread}
    </div>}
    {open && r.kind === 'merge' && <div class="req-actions">
      <button type="button" class={fill(true)} onClick={() => act.openDialog({ type: 'merge-review', requestId: r.id })}>
        Review and merge
      </button>
      <button disabled={liveMode.on && chatLive.requestBusy(r)} type="button" class="btn" onClick={() => act.answerRequest(r.id, 'decline')}>
        Send back
      </button>
      {thread}
    </div>}
    {open && r.kind === 'failed' && <div class="req-actions">
      {liveMode.on || fixed
        ? <button disabled={liveMode.on && chatLive.requestBusy(r)} type="button" class={fill(true)} onClick={() => act.answerRequest(r.id, 'resume')}>
          <Icon name="play" size={13} />
          Resume
        </button>
        : <button type="button" class={fill(true)} onClick={() => act.openDialog({ type: 'signin', subscription: a.failure?.subscription || a.subscription, then: r.id })}>
          {'Sign in on '}
          {machineOf(s, a.failure?.machine || p.machine).name}
        </button>}
      {thread}
    </div>}
    {!open && r.projectApproval && <p role="status">{approvalOutcome}</p>}
    {!open && variant === 'detail' && <div class="answer"><Icon name="check" size={14} /><span>{r.answer} at {clock(r.answeredAt!)}</span></div>}
  </section>;
}

// ---- threads ------------------------------------------------------------------
function ToolDetail({ detail }: { detail: ToolDetailRecord }) {
  const [expanded, setExpanded] = useState(false);
  const text = detail.text || '';
  const lines = text.split('\n').filter((line) => detail.kind !== 'diff' || line !== '\\ No newline at end of file');
  const cut = lines.length > 24 || text.length > 2400;
  const shown = expanded ? lines.join('\n') : lines.slice(0, 24).join('\n').slice(0, 2400);
  let inHunk = false, before: number | null = null, after: number | null = null;
  const shownLines = shown.split('\n').map((line) => {
    const header = /^@@ -\d+(?:,(\d+))? \+\d+(?:,(\d+))? @@/.exec(line);
    if (header) { before = Number(header[1] ?? 1); after = Number(header[2] ?? 1); inHunk = true; }
    else if (/^@@(?:\s|$)/.test(line)) { before = after = null; inHunk = true; }
    else if (line.startsWith('diff --git ')) inHunk = false;
    const mark = inHunk && !line.startsWith('@@') ? line.startsWith('+') ? 'add' : line.startsWith('-') ? 'del' : '' : '';
    if (inHunk && before !== null && after !== null && !line.startsWith('@@')) {
      if (line.startsWith('-') || line.startsWith(' ')) before--;
      if (line.startsWith('+') || line.startsWith(' ')) after--;
      if (before <= 0 && after <= 0) inHunk = false;
    }
    return { text: line, mark };
  });
  return <section class="tool-box">
    <h5>{detail.label}</h5>
    {detail.note && <p class="muted">{detail.note}</p>}
    <div class={detail.kind === 'diff' ? 'tool-diff' : ''}>
      <CodeBox text={shown} copyText={text} label={detail.label.toLowerCase()} copyLabel={detail.copyLabel || (detail.kind === 'diff' ? 'diff' : detail.label.toLowerCase())}>
        {detail.kind === 'diff' ? shownLines.map((line, i) => <span class={cx('diff-line', line.mark)}>{line.text}{i < shownLines.length - 1 ? '\n' : ''}</span>) : shown}
      </CodeBox>
    </div>
    {cut && <button type="button" class="btn sm ghost" aria-expanded={expanded} onClick={() => setExpanded(!expanded)}>
      {expanded ? 'Show less' : 'Show more'}
    </button>}
  </section>;
}
export function ToolGroup({ item }: { item: ThreadItem }) {
  const details = item.details || [];
  return <details class="tools">
    <summary><Icon name="wrench" size={13} /><span>{item.summary}</span><Icon name="chevron-right" size={14} class="chev" /></summary>
    {(item.raw || item.status) && <div class="tool-line"><span class="grow mono">{item.raw}</span><span class="res">{item.status}</span></div>}
    {details.map((detail, i) => <ToolDetail key={i} detail={detail} />)}
  </details>;
}
function LargeItem({ item }: { item: ThreadItem }) {
  const [state, setState] = useState('closed');
  if (liveMode.on) return <div class="tools">
    <span>{item.title} ({item.size})</span>
    <button type="button" class="btn sm" disabled={chatLive.busy('large:' + item.sequence)} onClick={() => chatLive.loadLarge(item)}>
      Load
    </button>
  </div>;
  const sample = ['round 2 · statement 11 · keyboard order at 390px', '  finding F-21 minor: focus leaves the drawer on Tab', '  finding F-22 major: Escape does not return focus to the menu button', 'round 2 · statement 12 · visible focus ring', '  no findings', 'round 2 · statement 13 · bottom key row', '  finding F-23 minor: Ctrl stays latched after Escape', 'round 2 · statement 14 · drawer order', '  finding F-24 minor: service pages come before agents', 'score: 6 points (1 major, 3 minor)', 'stuck statements: none', 'next: round 3 only if a major stays open'];
  return <div class="tools" style="grid-column:2">
    <div class="tool-line" style="border-top:0;min-height:var(--row)">
      <Icon name="file-text" size={14} class="muted" />
      <span class="grow"><b style="font-weight:var(--weight-medium)">{item.title}</b> <span class="muted">({item.size})</span></span>
      {state === 'closed' && <button type="button" class="btn sm" onClick={() => { setState('loading'); setTimeout(() => setState('open'), 600); }}>Load</button>}
      {state === 'loading' && <span class="muted">Loading…</span>}
      {state === 'open' && <>
        <button type="button" class="btn sm ghost" onClick={() => { navigator.clipboard?.writeText(sample.join('\n')); act.toast('Copied'); }}>
          <Icon name="copy" size={13} />
          Copy
        </button>
        <button type="button" class="btn sm ghost" onClick={() => act.openDialog({ type: 'large-item', title: item.title, lines: sample, total: item.lines })}>
          Open in full
        </button>
      </>}
    </div>
    {state === 'open' && <pre class="code" style="border-radius:0;max-height:240px">{sample.join('\n')}… {(item.lines as number) - 12} more lines</pre>}
  </div>;
}
registerDialog('large-item', ({ d }) => <Dialog title={d.title} size="wide" onClose={act.closeDialog} foot={<button type="button" class="btn" onClick={act.closeDialog}>Close</button>}>
  <pre class="code" style="max-height:60vh">{d.lines.join('\n')}… {d.total - d.lines.length} more lines</pre>
</Dialog>);

// The owner's messages sit on the right. Agents sit on the left.
// A request inside a thread is a card. Its "Open thread" goes through `onOpenThread`,
// which the project view points at the side panel.
export interface ThreadProps { s: State; agentId: Id; center?: boolean | undefined; focusReq?: Id | null | undefined; onOpenThread?: ((agent: Id, request: Id) => void) | undefined }
export function Thread({ s, agentId, center, focusReq, onOpenThread }: ThreadProps) {
  const a = agentOf(s, agentId); const items = threadOf(s, agentId); const now = useNow();
  useEffect(() => chatLive.watchThread(agentId), [agentId]);
  const box = useRef<HTMLDivElement>(null); const atEnd = useRef(true); const [away, setAway] = useState(false);
  const count = items.length + (s.pending[agentId] ? 1 : 0);
  // A thread opens at its latest message, and follows new ones while you are at the end.
  useLayoutEffect(() => {
    const el = box.current; if (!el) return;
    el.style.scrollBehavior = 'auto';
    if (focusReq) { const t = el.querySelector('#req-' + focusReq); if (t) { t.scrollIntoView({ block: 'center' }); (t as HTMLElement).focus?.({ preventScroll: true }); el.style.scrollBehavior = ''; return; } }
    el.scrollTop = el.scrollHeight; el.style.scrollBehavior = '';
  }, [agentId, focusReq]);
  useLayoutEffect(() => { const el = box.current; if (el && atEnd.current && !focusReq) el.scrollTop = el.scrollHeight; }, [count, items]);
  const onScroll = (e: Event) => { const el = e.currentTarget as HTMLDivElement; atEnd.current = el.scrollHeight - el.scrollTop - el.clientHeight < 60; setAway(!atEnd.current); };
  const below = away ? openRequests(s, { agent: agentId }).length : 0;
  const opener = (r: Request) => (r.agent === agentId ? null : onOpenThread ? () => onOpenThread(r.agent, r.id) : threadRouteForRequest(s, r));
  let lastDay: string | null = null;
  return <div style="position:relative;display:flex;flex-direction:column;flex:1;min-height:0">
    <div ref={box} class={cx('thread', center && 'center')} onScroll={onScroll} tabindex={0} aria-busy={liveMode.on && chatLive.busy('thread:' + agentId)} aria-label={'Thread with ' + a.name}>
      {items.length === 0 && <div class="empty" data-empty>
        <strong>{liveMode.on && chatLive.busy('thread:' + agentId) ? 'Loading thread…' : 'No messages'}</strong>
      </div>}
      {items.map((it) => {
        const d = it.ts == null ? null : dayLabel(it.ts, now); const head = d !== lastDay ? <div class="day" key={'d' + it.id}>{d}</div> : null; lastDay = d;
        let body: VNode | null;
        if (it.type === 'user' && !it.from) {
          body = <div class="msg mine" key={it.id}>
            <div class="meta"><time>{clock(it.ts)}</time></div>
            <div class="body">
              {it.text && <Rich text={it.text} pid={a.project} />}
              <Attachments files={it.files} sent={true} />
              {it.context?.length! > 0 && <div class="row" style="flex-wrap:wrap;gap:4px">{it.context!.map((c) => <span class="tag">{c as string}</span>)}</div>}
            </div>
          </div>;
        } else if (it.type === 'user') {
          const from = agentOf(s, it.from);
          body = <div class="msg peer" key={it.id}>
            <Avatar agent={from} s={s} status={false} />
            <div class="meta"><span class="who">{agentLabel(s, from, { withTeam: true })}</span><time>{clock(it.ts)}</time></div>
            <div class="body"><Rich text={it.text} pid={a.project} /></div>
          </div>;
        } else if (it.type === 'agent') {
          body = <div class="msg" key={it.id}>
            <Avatar agent={a} s={s} status={false} />
            <div class="meta"><span class="who">{a.name}</span><time>{clock(it.ts)}</time></div>
            <div class="body"><Rich text={it.text} pid={a.project} /></div>
            {it.requests?.length! > 0 && <div class="req-stack">
              {it.requests!.map((id) => {
              const r = byId(s.requests, id); if (!r) return null;
              return <RequestBlock key={id} s={s} request={r} openThread={opener(r)} />;
            })}
            </div>}
          </div>;
        } else if (it.type === 'tools') body = <div class="msg" key={it.id}><span /><ToolGroup item={it} /></div>;
        else if (it.type === 'large') body = <div class="msg" key={it.id}><span /><LargeItem item={it} /></div>;
        else if (it.type === 'request') { const r = byId(s.requests, it.requestId); body = r ? <div class="msg" key={it.id}>
          <span />
          <div class="req-stack"><RequestBlock s={s} request={r} openThread={opener(r)} who={r.agent !== agentId} /></div>
        </div> : null; }
        else body = <div class="day" key={it.id} style="text-transform:none;letter-spacing:0">{it.text} · {clock(it.ts)}</div>;
        return [head, body];
      })}
      {s.pending[agentId] && <div class="msg">
        <Avatar agent={a} s={s} status={false} />
        <div class="meta"><span class="who">{a.name}</span></div>
        <div class="body"><span class="typing" aria-label="Writing a reply"><i /><i /><i /></span></div>
      </div>}
    </div>
    {away && <button type="button" class="btn sm" style="position:absolute;right:16px;bottom:8px;box-shadow:var(--shadow-1)" onClick={() => { box.current!.scrollTop = box.current!.scrollHeight; }}>
      <Icon name="arrow-down" size={13} />
      Latest
      {below ? ` · ${plural(below, 'request')} below` : ''}
    </button>}
  </div>;
}

// Only the project lead takes messages. Another agent's thread is read-only, and gains a
// reply box only while that agent has an open question for the owner.
export function canMessage(s: State, a: Agent): boolean {
  return liveMode.on || a.role === 'project-lead' || a.role === 'repository-agent' || a.role === 'team-lead' || openRequests(s, { agent: a.id }).some((r) => r.kind === 'question');
}
export interface ReadOnlyBarProps { s: State; agent: Agent; center?: boolean | undefined; onLead?: (() => void) | undefined }
export function ReadOnlyBar({ s, agent, center, onLead }: ReadOnlyBarProps) {
  const st = agentStatus(s, agent); const p = projectOf(s, agent.project);
  return <div class={cx('composer-wrap', center && 'center')}>
    <div class="readonly-bar">
      <Icon name="eye" size={15} class="muted" />
      <span class="grow truncate"><b>Read only</b><span class="muted"> · {agent.activity || st.word}</span></span>
      {onLead ? <button type="button" class="btn sm" onClick={onLead}><Icon name="crown" size={13} />Project lead</button>
      : <a class="btn sm" href={href(`/p/${p.id}/conversation`)}><Icon name="crown" size={13} />Project lead</a>}
    </div>
  </div>;
}

// ---- composer -----------------------------------------------------------------
/** A task or a file the owner can attach to a message. The live website adds the engine's reference. */
export type ContextItem = { k: string; label: string; icon: string; reference?: { _tag: string; [more: string]: unknown } };
type ContextChoice = string | ContextItem;
function ContextPicker({ s, agent, onAdd, onClose }: { s: State; agent: Agent; onAdd: (x: ContextChoice) => void; onClose: () => void }) {
  const [q, setQ] = useState('');
  const [found, setFound] = useState<ContextItem[]>([]);
  useEffect(() => {
    if (!liveMode.on) return;
    let active = true;
    chatLive.searchContext(agent, q).then((items: ContextItem[]) => { if (active) setFound(items); }).catch((e) => { if (active) { setFound([]); act.toast(e, 'error'); } });
    return () => { active = false; };
  }, [agent.id, q]);
  const tasks: ContextItem[] = s.tasks.filter((t) => t.project === agent.project).map((t) => ({ k: t.label, label: `${t.label} ${t.title}`, icon: 'list-checks' }));
  const files: ContextItem[] = [...new Set(s.tasks.filter((t) => t.project === agent.project).flatMap((t) => t.files))].map((f) => ({ k: f, label: f, icon: 'file-code' }));
  const match = (x: ContextItem) => x.label.toLowerCase().includes(q.toLowerCase());
  const T = liveMode.on ? found.filter((x) => x.reference!._tag === 'task') : tasks.filter(match).slice(0, 6), F = liveMode.on ? found.filter((x) => x.reference!._tag === 'file') : files.filter(match).slice(0, 6);
  useKey((e) => { if (e.key === 'Escape') { e.stopPropagation(); onClose(); } }, []);
  const row = (x: ContextItem) => <button type="button" key={x.k} class="menu-item" onClick={() => onAdd(liveMode.on ? x : x.k)}>
    <Icon name={x.icon} size={14} />
    <span class="truncate">{x.label}</span>
  </button>;
  return <div class="menu" style="position:absolute;left:8px;bottom:calc(100% + 4px);width:min(380px,calc(100% - 16px))">
    <div class="search" style="padding:2px">
      <Icon name="search" size={14} />
      <input class="input" autofocus placeholder="Search tasks and files" value={q} onInput={(e) => setQ((e.target as HTMLInputElement).value)} />
    </div>
    {T.length > 0 && <><div class="menu-label">Tasks</div>{T.map(row)}</>}
    {F.length > 0 && <><div class="menu-label">Files</div>{F.map(row)}</>}
    {!T.length && !F.length && <div class="menu-label">Nothing matches "{q}".</div>}
  </div>;
}
// Files and images go with a message. They come from the paperclip button, a drop on the
// message box, or a paste. A large photo is resized on this device before it is sent: the
// long edge becomes at most 2,048 pixels and the photo is saved as a JPEG. Redrawing it also
// drops the camera details, such as where the photo was taken. "Send original" keeps the
// file as it was. A photo the browser cannot read, such as a raw camera file, goes as a file.
const FILE_LIMIT = 100 * 1024 * 1024;
const EDGE = 2048; const SHRINK_OVER = 2 * 1024 * 1024;
const HEIC = /image\/hei[cf]|\.hei[cf]$/i;
export const fileSize = (n: number): string => n < 1024 ? n + ' B' : n < 1024 * 1024 ? Math.round(n / 1024) + ' KB' : (n / 1024 / 1024).toFixed(1) + ' MB';
async function shrink(file: File): Promise<Blob | null> {
  const bmp = await createImageBitmap(file);
  const k = Math.min(1, EDGE / Math.max(bmp.width, bmp.height));
  if (k === 1 && file.size <= SHRINK_OVER) { bmp.close(); return null; }
  const w = Math.round(bmp.width * k), h = Math.round(bmp.height * k);
  const c = document.createElement('canvas'); c.width = w; c.height = h; const g = c.getContext('2d')!;
  g.fillStyle = '#fff'; g.fillRect(0, 0, w, h); g.drawImage(bmp, 0, 0, w, h); bmp.close();
  const blob = await new Promise<Blob | null>((r) => c.toBlob(r, 'image/jpeg', 0.85));
  return blob && blob.size < file.size ? blob : null;
}
// What a staged attachment sends: the resized copy, unless the owner chose the original.
const sentForm = (f: Attachment): Attachment => (f.small && !f.useOriginal ? { ...f, size: f.small.size, url: f.small.url, name: f.small.name, from: f.size } : f);
export interface AttachmentsProps { files: Attachment[] | null | undefined; onRemove?: (id: Id) => void; onOriginal?: (id: Id) => void; sent?: boolean }
export function Attachments({ files, onRemove, onOriginal, sent }: AttachmentsProps) {
  if (!files?.length) return null;
  const x = (f: Attachment) => onRemove && <button type="button" class="att-x" aria-label={'Remove ' + f.name} onClick={() => onRemove(f.id!)}><Icon name="x" size={11} /></button>;
  if (sent) {
    const pics = files.filter((f) => f.image); const docs = files.filter((f) => !f.image);
    return <div class="att sent">
      {pics.length > 0 && <div class="att-pics">
        {pics.map((f) => <figure class="att-shot" key={f.id}>
        <a class="att-pic" href={f.url ?? undefined} target="_blank" rel="noopener" title={f.name}><img src={f.url!} alt={f.name} /></a>
        <figcaption>{fileSize(f.size)}{f.from ? ', resized from ' + fileSize(f.from) : ''}</figcaption>
      </figure>)}
      </div>}
      {docs.map((f) => <span class="att-file" key={f.id}>
        <Icon name="file" size={14} class="faint" />
        <span class="truncate">{f.name}</span>
        <span class="faint">{fileSize(f.size)}{f.note ? ' · ' + f.note : ''}</span>
      </span>)}
    </div>;
  }
  return <div class="att">
    {files.map((f) => {
    if (!f.image) return <span class="att-img" key={f.id}>
      <span class="att-thumb file"><Icon name="file" size={16} /></span>
      <span class="att-txt">
        <span class="truncate">{f.name}</span>
        <span class="muted">{fileSize(f.size)}{f.note ? ' · ' + f.note : ''}</span>
      </span>
      {x(f)}
    </span>;
    const v = sentForm(f);
    const line = f.working ? 'Resizing…' : f.small ? (f.useOriginal ? `Original, ${fileSize(f.size)}` : `Resized from ${fileSize(f.size)} to ${fileSize(f.small.size)}`) : fileSize(f.size);
    return <span class="att-img" key={f.id}>
      <span class="att-thumb">{v.url && <img src={v.url} alt="" />}</span>
      <span class="att-txt"><span class="truncate">{v.name}</span><span class="muted">{line}</span></span>
      {f.small && <button type="button" class="link att-orig" onClick={() => onOriginal!(f.id!)}>
        {f.useOriginal ? 'Send resized' : 'Send original'}
      </button>}
      {x(f)}
    </span>; })}
  </div>;
}
// `bare` leaves out the status line above the box. The send button then stops a working agent.
export interface ComposerProps { s: State; agentId: Id; center?: boolean | undefined; bare?: boolean }
export function Composer({ s, agentId, center, bare = false }: ComposerProps) {
  const a = agentOf(s, agentId); const key = 'thread:' + agentId;
  const [, bump] = useState(0); const [picking, setPicking] = useState(false); const [ctx, setCtx] = useState<ContextChoice[]>([]);
  const [files, setFiles] = useState<Attachment[]>([]); const [over, setOver] = useState(false); const fileIn = useRef<HTMLInputElement>(null);
  const ta = useRef<HTMLTextAreaElement>(null);
  const draft = s.drafts[key] || '';
  const st = agentStatus(s, a); const pending = !!s.pending[agentId];
  const fit = () => { const el = ta.current; if (!el) return; el.style.height = 'auto'; el.style.height = Math.min(220, el.scrollHeight) + 'px'; };
  useLayoutEffect(fit, [draft, agentId]);
  useEffect(() => { setCtx([]); setPicking(false); setFiles([]); }, [agentId]);
  const patch = (id: Id, ch: Partial<Attachment>) => setFiles((cur) => cur.map((x) => (x.id === id ? { ...x, ...ch } : x)));
  const addFiles = (list: FileList | File[]) => {
    if (liveMode.on) { refuse(); return; }
    const ok = [...list].filter((f) => { if (f.size <= FILE_LIMIT) return true; act.toast(`${f.name} is larger than 100 MB`, 'error'); return false; });
    const items = ok.map((f) => { const pic = f.type.startsWith('image/') && !/gif|svg/.test(f.type);
      return { id: uid('f'), file: f, name: f.name || 'Pasted image.png', size: f.size, image: pic, working: pic, url: null as string | null }; });
    setFiles((cur) => [...cur, ...items]);
    for (const it of items.filter((x) => x.image)) {
      shrink(it.file).then((blob) => patch(it.id, { working: false, url: URL.createObjectURL(it.file),
        small: blob && { size: blob.size, url: URL.createObjectURL(blob), name: it.name.replace(/\.[^.]+$/, '') + '.jpg' } }))
        .catch(() => patch(it.id, { working: false, image: false, note: HEIC.test(it.file.type + ' ' + it.name) ? 'Converted by the service' : 'Sent as a file' }));
    }
    ta.current?.focus();
  };
  const dropOn = { onDragOver: (e: DragEvent) => { if ([...(e.dataTransfer?.types || [])].includes('Files')) { e.preventDefault(); setOver(true); } },
    onDragLeave: (e: DragEvent) => { if (!(e.currentTarget as Element).contains(e.relatedTarget as Node)) setOver(false); },
    onDrop: (e: DragEvent) => { e.preventDefault(); setOver(false); if (e.dataTransfer?.files?.length) addFiles(e.dataTransfer.files); } };
  const onPaste = (e: ClipboardEvent) => { const fl = e.clipboardData?.files; if (fl?.length) { if (!e.clipboardData!.getData('text/plain')) e.preventDefault(); addFiles(fl); } };
  const live = s.conn.status === 'live';
  const ready = (draft.trim() || files.length) && !chatLive.busy('agent:' + agentId) && live && !files.some((f) => f.working);
  const send = async () => { if (!ready) return; if ((!draft.trim() && !files.length) || files.some((f) => f.working)) return; const sent = await act.sendMessage(agentId, draft, ctx, files.map(sentForm).map(({ file, small, working, useOriginal, ...f }) => f)); if (liveMode.on && !sent) return; setCtx([]); setFiles([]); bump((n) => n + 1); };
  const onInput = (e: Event) => {
    const v = (e.target as HTMLTextAreaElement).value; act.setDraft(key, v); bump((n) => n + 1);
    const pos = (e.target as HTMLTextAreaElement).selectionStart; if (v[pos - 1] === '@' && (pos === 1 || /\s/.test(v[pos - 2]))) setPicking(true);
  };
  const onKey = (e: KeyboardEvent) => { if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) { e.preventDefault(); send(); } };
  const addCtx = (k: ContextChoice) => { setCtx((c) => (c.some((x) => ((x as ContextItem).k || x) === ((k as ContextItem).k || k)) ? c : [...c, k])); setPicking(false); act.setDraft(key, draft.replace(/@$/, '')); bump((n) => n + 1); ta.current?.focus(); };
  const h = HARNESSES[a.harness]!;
  const subs = s.subscriptions.filter((x) => x.harness === a.harness);
  const busyLine = liveMode.on && a.configuration?.status === 'pending' ? 'Model and effort apply from the next turn' : liveMode.on && ['refused', 'unsupported', 'unavailable'].includes(a.configuration?.status as string) ? a.configuration!.status + ': ' + (a.configuration!.reason || '') : pending ? 'Working on your message' : (a.state === 'working' || a.state === 'waiting') && a.activity ? a.activity : null;
  const canStop = a.state === 'working' || a.state === 'waiting';

  if (a.state === 'failed') {
    return <div class={cx('composer-wrap', center && 'center')}>
      <div class="composer-state">
        <Icon name="circle-alert" class="error-text" />
        <div class="txt"><b>{a.name} failed to start.</b><small>{a.failure?.reason}. {draft ? 'Your draft is kept.' : ''}</small></div>
        <button type="button" class="btn primary" onClick={() => act.openDialog({ type: 'signin', subscription: a.failure?.subscription || a.subscription, then: openRequests(s, { agent: a.id })[0]?.id })}>
          {'Sign in on '}
          {machineOf(s, a.failure?.machine || (liveMode.on ? projectOf(s, a.project).machine : 'studio')).name}
        </button>
        <button type="button" class="btn" onClick={() => { const r = openRequests(s, { agent: a.id })[0]; r ? act.answerRequest(r.id, 'resume') : act.resumeAgent(a.id); }}>
          Resume
        </button>
      </div>
    </div>;
  }
  if (a.state === 'stopped') {
    return <div class={cx('composer-wrap', center && 'center')}>
      <div class="composer-state">
        <Icon name="circle-stop" class="muted" />
        <div class="txt">
          <b>{a.crashed ? a.name + ': ' + a.activity : a.name + ' is stopped.'}</b>
          <small>{draft ? 'Your draft is kept. ' : ''}Resume continues from its last turn.</small>
        </div>
        <button type="button" disabled={liveMode.on && chatLive.busy('agent:' + a.id)} class="btn primary" onClick={() => act.resumeAgent(a.id)}>
          <Icon name="play" size={13} />
          Resume
        </button>
      </div>
    </div>;
  }
  return <div class={cx('composer-wrap', center && 'center')}>
    {!bare && (busyLine || canStop) && <div class="statusline">
      {busyLine && <><i class={cx('dot', st.dot)} /><span class="truncate">{busyLine}</span></>}
      {canStop && <button type="button" disabled={liveMode.on && chatLive.busy('agent:' + a.id)} class="btn sm" onClick={() => act.stopAgent(a.id)}>
        <Icon name="square" size={11} />
        Stop agent
      </button>}
    </div>}
    <div class={cx('composer', over && 'drop')} style="position:relative" {...dropOn}>
      {over && <div class="drop-note" aria-hidden="true"><Icon name="upload" size={16} />Drop to attach</div>}
      {picking && <ContextPicker s={s} agent={a} onAdd={addCtx} onClose={() => setPicking(false)} />}
      {ctx.length > 0 && <div class="ctx">
        {ctx.map((c) => <span class="chip" key={(c as ContextItem).k || c}>
        <span class="mono">{(c as ContextItem).label || c}</span>
        <button type="button" aria-label={'Remove ' + ((c as ContextItem).label || c)} onClick={() => setCtx(ctx.filter((x) => x !== c))}>
          <Icon name="x" size={11} />
        </button>
      </span>)}
      </div>}
      <Attachments files={files} onRemove={(id) => setFiles(files.filter((f) => f.id !== id))} onOriginal={(id) => patch(id, { useOriginal: !files.find((f) => f.id === id)!.useOriginal })} />
      <input ref={fileIn} type="file" multiple hidden onChange={(e) => { addFiles((e.target as HTMLInputElement).files!); (e.target as HTMLInputElement).value = ''; }} />
      <label class="sr" for={'composer-' + agentId}>Message {a.name}. Enter sends, Shift and Enter adds a line.</label>
      <textarea ref={ta} id={'composer-' + agentId} rows={1} placeholder={`Message the ${a.name.toLowerCase()}…`} value={draft} onInput={onInput} onKeyDown={onKey} onPaste={onPaste} disabled={!live} />
      <div class="tools-row">
        <button type="button" class="pick" aria-expanded={picking} onClick={() => setPicking(!picking)} aria-label="Add a task or a file" data-tip="Add a task or a file" data-tip-side="top">
          <Icon name="at-sign" size={14} />
          <span class="not-narrow">Context</span>
        </button>
        <button type="button" class="pick" aria-label="Attach files or images" data-tip="Attach files or images" data-tip-side="top" disabled={!live} onClick={() => fileIn.current?.click()}>
          <Icon name="paperclip" size={14} />
        </button>
        <Menu class="pick" side="top" label="Subscription" trigger={<><span class="truncate">{subLabel(s, a.subscription)}</span><Icon name="chevron-down" size={12} /></>} items={[{ heading: 'Subscription' }, ...subs.map((x) => ({ label: `${h.name} · ${x.name}`, sub: machineOf(s, x.machine).name, checked: x.id === a.subscription, disabled: !usableSubscription(x), why: x.signInKnown === false ? 'Not reported' : x.signedIn ? null : 'Signed out', onSelect: () => act.setAgentConfig(a.id, { subscription: x.id }) }))]} />
        <Menu class="pick" side="top" label="Model" trigger={<><span>{a.model}</span><Icon name="chevron-down" size={12} /></>} items={[{ heading: 'Model, from the next turn' }, ...(liveMode.on ? a.models || [] : h.models).map((m: any) => ({ label: m.label || m.id || m, checked: (m.id || m) === a.model, disabled: chatLive.busy('agent:' + a.id), onSelect: () => act.setAgentConfig(a.id, { model: m.id || m }) }))]} />
        <Menu class="pick" side="top" label="Effort" trigger={<><span>{a.effort}</span><Icon name="chevron-down" size={12} /></>} items={[{ heading: 'Effort, from the next turn' }, ...(liveMode.on ? a.efforts || [] : h.efforts).map((m: any) => ({ label: m.label || m.id || m, checked: (m.id || m) === a.effort, disabled: chatLive.busy('agent:' + a.id), onSelect: () => act.setAgentConfig(a.id, { effort: m.id || m }) }))]} />
        <span class={cx('ctxuse', a.context >= 80 && 'warn-text')} data-tip="Share of the context window in use" data-tip-side="top">
          {a.context == null ? '—' : a.context + '%'}
        </span>
        {pending
          ? <button type="button" class="send stop" disabled={chatLive.busy('agent:' + agentId)} aria-label="Stop this reply" data-tip="Stop this reply" data-tip-side="left" onClick={() => act.stopReply(agentId)}>
            <Icon name="square" size={12} />
          </button>
          : bare && a.state === 'working' && !ready
            ? <button type="button" class="send stop" aria-label="Stop agent" data-tip="Stop agent" data-tip-side="left" onClick={() => act.stopAgent(a.id)}>
              <Icon name="square" size={12} />
            </button>
            : <button type="button" class="send" aria-label="Send" disabled={!ready} onClick={send}>
              <Icon name="arrow-up" size={15} />
            </button>}
      </div>
    </div>
  </div>;
}

// ---- misc ---------------------------------------------------------------------
// `short` marks a tab that shows only its icon in a narrow window. Its name stays for a screen reader.
export interface TabItem { id: string; label: ComponentChildren; icon?: string | undefined; count?: number | undefined; tone?: string | null | undefined; short?: boolean }
export function Tabs({ tabs, current, base }: { tabs: TabItem[]; current: string | null | undefined; base: string }) {
  return tabs.map((t) => <a key={t.id} class={cx('tab', t.short && 'short')} role="tab" aria-selected={current === t.id} href={href(base + t.id)}>
    {t.icon && <Icon name={t.icon} size={14} />}
    <span>{t.label}</span>
    {t.count! > 0 && <span class={cx('count', t.tone)}>{t.count}</span>}
  </a>);
}
export function Kbd({ k }: { k: string }) { return <kbd>{isMac ? <Icon name="command" size={10} /> : 'Ctrl'} {k}</kbd>; }
export interface EmptyProps { title: ComponentChildren; children?: ComponentChildren; icon?: string | undefined }
// ---- Back ---------------------------------------------------------------
const VIEW_NAMES: Record<string, string> = { board: 'Board', sprint: 'Sprint', task: 'Task', questions: 'Questions', status: 'Status', 'end-result': 'End result', actions: 'Actions',
  design: 'Design', files: 'Files', git: 'Git', settings: 'Settings', conversation: 'Conversation', team: 'Team', 'message-board': 'Message board' };
const PAGE_NAMES: Record<string, string> = { home: 'Home', machines: 'Machines', harnesses: 'Harnesses', usage: 'Usage', notifications: 'Notifications', workflows: 'Workflows', settings: 'Service settings', r: 'Repository' };
/**
 * The Back button of a view's own header. It shows only when the owner followed a link from
 * another view to get here, and it names that view. It returns to the address they came from.
 */
// Status, the message board and the end result are pages of one view: its sub-tabs move between them.
const FAMILY: Record<string, string> = { 'message-board': 'status', 'end-result': 'status' };
/** `or` is what the header shows in Back's place when Back does not show, such as an arrow to a parent. */
export function Back({ path, class: cls, or = null }: { path: string; class?: string; or?: ComponentChildren }) {
  const from = cameFrom(); if (!from) return <>{or}</>;
  const parts = (p: string) => p.split('?')[0]!.split('/').filter(Boolean);
  const a = parts(from), b = parts(path); const view = (x: string[]) => (x[0] === 'p' ? FAMILY[x[2] || ''] || x[2] : x[1]);
  // A move inside one view is not a way back out of it.
  if (a[0] === b[0] && a[1] === b[1] && view(a) === view(b)) return <>{or}</>;
  const name = a[0] !== 'p' ? PAGE_NAMES[a[0] || ''] : a[2] === 'sprint' && a[4] === 'tasks' && a[5] ? 'Task' : a[2] === 'questions' && a[3] ? 'Question' : VIEW_NAMES[a[2] || ''];
  return <button type="button" class={cx('btn sm ghost back-btn', cls)} aria-label={name ? `Back to ${name}` : 'Back'} onClick={() => history.back()}>
    <Icon name="arrow-left" size={14} />
    <span>{name || 'Back'}</span>
  </button>;
}

export function Empty({ title, children, icon }: EmptyProps) {
  return <div class="empty" data-empty>{icon && <Icon name={icon} size={18} />}<strong>{title}</strong>{children != null && children !== false && <span>{children}</span>}</div>;
}
export function StateWord({ status }: { status: Status }) { return <span class={cx(status.tone === 'warn' && 'warn-text', status.tone === 'error' && 'error-text', !status.tone && 'muted')} style={status.tone ? 'font-weight:var(--weight-medium)' : ''}>
  {status.word}
</span>; }
export { go };
