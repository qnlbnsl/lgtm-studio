// The mock's small toolkit: rendering, a store, a hash router, and time helpers.
// Rendering comes from Preact 11. Screens import it from 'preact' and 'preact/hooks' directly.
import { options, type VNode } from 'preact';
import { useState, useEffect, useLayoutEffect, useReducer } from 'preact/hooks';
import { ICONS } from './icons';
import type { Route, State } from './types';

// ---- false data attributes ------------------------------------------------
// The mock's Preact 10 build removed a data- attribute whose value is false. Preact 11 writes
// it as "false". Screens pass `data-tip=${cond && 'text'}` and the styles select [data-tip],
// so this keeps the old rule: a false data- attribute is left out.
const previousVNode = options.vnode;
options.vnode = (vnode: VNode) => {
  if (typeof vnode.type === 'string') {
    const props = vnode.props as Record<string, unknown>;
    for (const k in props) if (props[k] === false && k.startsWith('data-')) props[k] = undefined;
  }
  previousVNode?.(vnode);
};

// ---- class names ---------------------------------------------------------
export type ClassPart = string | false | null | undefined | 0 | ClassPart[] | Record<string, unknown>;
export function cx(...parts: ClassPart[]): string {
  const out: string[] = [];
  for (const p of parts) {
    if (!p) continue;
    if (typeof p === 'string') out.push(p);
    else if (Array.isArray(p)) out.push(cx(...p));
    else for (const k in p) if (p[k]) out.push(k);
  }
  return out.join(' ');
}

// ---- icons ---------------------------------------------------------------
export interface IconProps { name: string; size?: number; class?: string | undefined; title?: string | undefined }
export function Icon({ name, size = 16, class: cls, title }: IconProps) {
  const inner = ICONS[name];
  if (!inner) return <span class="icon-missing" title={name}>?</span>;
  return <svg class={cx('icon', cls)} width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden={title ? undefined : 'true'} role={title ? 'img' : undefined} aria-label={title || undefined} dangerouslySetInnerHTML={{ __html: inner }} />;
}

// ---- store ---------------------------------------------------------------
// One mutable state object. `update` runs a mutation and tells every subscriber.
const listeners = new Set<() => void>();
// The state is set by mount() before anything renders.
export const store: { state: State } = { state: null as unknown as State };
let scheduled = false;
export function notify() {
  if (scheduled) return;
  scheduled = true;
  queueMicrotask(() => { scheduled = false; for (const l of [...listeners]) l(); });
}
export function update(fn: (s: State) => void): void {
  if (!live.on) { fn(store.state); notify(); return; }
  // In live mode, a change that the engine does not take yet is refused as a whole. Nothing
  // else changes in the same turn either, so no "Saved" or success notice follows it.
  if (live.refusedTurn) return;
  try { fn(store.state); } catch (e) { if (!refusal(e)) throw e; refuse(); }
  notify();
}

// ---- live mode -------------------------------------------------------------
// In live mode the engine owns the records in the store. The connection writes them through
// engineWrite(), and they are frozen, so a screen that tries to change one is refused, with
// a short notice, instead of changing the browser's copy. The page's own state, such as the
// preferences, the open dialog and the drafts, stays writable.
export const ENGINE_OWNED = new Set<string | symbol>([
  'session', 'conn', 'machines', 'subscriptions', 'repos', 'projects', 'teams', 'agents', 'epics', 'sprints', 'tasks',
  'machineDetails', 'phoneOffer', 'accountLimits', 'threads', 'pending',
  'boardUnread', 'boardRevision', 'boardDetails', 'taskDetails', 'setupData', 'boardAgents', 'projectLeadSettings', 'requests', 'channels', 'posts', 'worktrees', 'workflow', 'mocks', 'design', 'notifications', 'settings',
]);
export const live: { on: boolean; refusedTurn: boolean; onRefuse: (() => void) | null } = { on: false, refusedTurn: false, onRefuse: null };
let engineWriting = false;
class Refused extends Error {}
export function freeze<T>(value: T): T {
  if (value && typeof value === 'object' && !Object.isFrozen(value)) { Object.freeze(value); for (const k of Object.keys(value)) freeze((value as Record<string, unknown>)[k]); }
  return value;
}
// Wraps the live store, so that only engineWrite() can replace a record the engine owns.
export function guardStore(state: State): State {
  const raw = state as unknown as Record<string | symbol, unknown>;
  for (const k of ENGINE_OWNED) if (k in raw) freeze(raw[k]);
  return new Proxy(raw, {
    // Writing back the value a key already holds changes nothing, so it is allowed.
    set(target, key, value: unknown) { if (ENGINE_OWNED.has(key) && !engineWriting && target[key] !== value) throw new Refused(String(key)); target[key] = value; return true; },
    deleteProperty(target, key) { if (ENGINE_OWNED.has(key) && !engineWriting) throw new Refused(String(key)); delete target[key]; return true; },
  }) as unknown as State;
}
// The connection's only way to change the records it owns. Each value it writes is frozen.
export function engineWrite(patch: Partial<State>): void {
  engineWriting = true;
  try { for (const [k, v] of Object.entries(patch)) (store.state as unknown as Record<string, unknown>)[k] = freeze(v); } finally { engineWriting = false; }
  notify();
}
// True for the error a refused change throws: a write to a frozen record, or a replaced one.
export function refusal(e: unknown): boolean {
  return e instanceof Refused || (e instanceof TypeError && /read[- ]only|not extensible|Cannot (add|delete|define|assign)|can't (define|delete|assign)/i.test(e.message));
}
export function refuse() {
  if (live.refusedTurn) return;
  live.refusedTurn = true;
  setTimeout(() => { live.refusedTurn = false; }, 0);
  live.onRefuse?.();
}
export function useStore() {
  const [, bump] = useReducer((n: number, _: void) => n + 1, 0);
  useLayoutEffect(() => { const l = () => bump(); listeners.add(l); return () => { listeners.delete(l); }; }, []);
  return store.state;
}

// ---- router --------------------------------------------------------------
// The address after # is the route, so a refresh or a shared link keeps the place.
export function parseRoute(): Route {
  const raw = location.hash.replace(/^#\/?/, '');
  const [path = '', query = ''] = raw.split('?');
  const parts = path.split('/').filter(Boolean).map(decodeURIComponent);
  return { parts, query: new URLSearchParams(query), path: '/' + parts.join('/') };
}
export function go(path: string): void {
  const target = '#' + (path.startsWith('/') ? path : '/' + path);
  if (location.hash === target) return;
  location.hash = target;
}
export function href(path: string): string { return '#' + (path.startsWith('/') ? path : '/' + path); }

// ---- where the owner came from --------------------------------------------
// A view's Back button returns to the address the owner came from, as the browser's own Back does.
// Each entry of the browser's history that this page made carries two things: its place in the
// row (0 is the first), and the address the owner followed a link from. A move through the row of
// views, the sidebar, the top bar's path or a row of sub-tabs is a switch, not a link, so it leaves
// no such address. design/flows.md lists every move and its kind.
type Placed = { i?: number; from?: string } | null;
const SWITCHES = '.tabrow, .sidebar, .crumbs, .wf-nav, .view-tabs';
let place = (history.state as Placed)?.i ?? 0; let origin: string | undefined = (history.state as Placed)?.from;
let last = location.hash; let swapping = false; let carried: string | undefined; let linkedAt = -1e9;
const mark = () => { try { history.replaceState({ ...(history.state as object | null), i: place, from: origin }, ''); } catch { /* the place is a convenience */ } };
mark();
// Only a click outside the standing lists of places is a link, and only when the address changes
// right after it. An address typed by hand is not a link.
addEventListener('click', (e) => { linkedAt = (e.target as Element | null)?.closest?.(SWITCHES) ? -1e9 : performance.now(); }, true);
// This listener is added before any screen's, so the place is right when the screen draws.
addEventListener('hashchange', () => {
  const st = history.state as Placed;
  if (typeof st?.i === 'number') { place = st.i; origin = st.from; } // the owner went back or forward
  else if (swapping) { origin = carried; mark(); }
  else { place += 1; origin = performance.now() - linkedAt < 1000 ? last : undefined; mark(); }
  swapping = false; linkedAt = -1e9; last = location.hash;
});
/** Puts a new address in the place of the present one, so Back skips the present one. */
export function swap(path: string): void {
  const target = href(path);
  if (location.hash === target) return;
  swapping = true; carried = origin; location.replace(target);
}
/** The address the owner followed a link from to get here, without its `#`. Null when there is none. */
export const cameFrom = (): string | null => (place > 0 && origin ? origin.replace(/^#/, '') : null);
/**
 * A choice of a view that is kept in the address, so that Back from another view returns to it.
 * A change takes the place of the last one, so Back still leaves the view in one step.
 */
export function useQuery(key: string): [string | null, (v: string | null | undefined) => void] {
  const route = useRoute();
  const set = (v: string | null | undefined) => { const now = parseRoute(); if (v) now.query.set(key, v); else now.query.delete(key); const q = now.query.toString(); swap(now.path + (q ? '?' + q : '')); };
  return [route.query.get(key), set];
}
export function useRoute() {
  const [route, setRoute] = useState(parseRoute);
  useEffect(() => {
    const on = () => setRoute(parseRoute());
    addEventListener('hashchange', on);
    return () => removeEventListener('hashchange', on);
  }, []);
  return route;
}

// ---- time ----------------------------------------------------------------
// The mock's clock is real time, so waits grow while the page is open.
export const MIN = 60_000, HOUR = 60 * MIN, DAY = 24 * HOUR;
export function ago(ts: number, now = Date.now()): string {
  const d = Math.max(0, now - ts);
  if (d < MIN) return 'now';
  if (d < HOUR) return Math.floor(d / MIN) + ' min';
  if (d < DAY) { const hrs = Math.floor(d / HOUR), m = Math.floor((d % HOUR) / MIN); return m && hrs < 6 ? `${hrs} hr ${m} min` : `${hrs} hr`; }
  const days = Math.floor(d / DAY);
  return days === 1 ? '1 day' : days + ' days';
}
export function clock(ts: number): string {
  const d = new Date(ts);
  return d.getHours().toString().padStart(2, '0') + ':' + d.getMinutes().toString().padStart(2, '0');
}
export function dayLabel(ts: number, now = Date.now()): string {
  const a = new Date(ts), b = new Date(now);
  const same = a.toDateString() === b.toDateString();
  if (same) return 'Today';
  const y = new Date(now - DAY);
  if (a.toDateString() === y.toDateString()) return 'Yesterday';
  return a.toLocaleDateString('en-GB', { day: 'numeric', month: 'short' });
}
export function useNow(every = 30_000) {
  const [now, setNow] = useState(Date.now());
  useEffect(() => { const t = setInterval(() => setNow(Date.now()), every); return () => clearInterval(t); }, [every]);
  return now;
}

// ---- small helpers -------------------------------------------------------
export const isMac = /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent);
export function plural(n: number, one: string, many?: string): string { return n + ' ' + (n === 1 ? one : (many || one + 's')); }
export function uid(prefix = 'id') { return prefix + '-' + Math.random().toString(36).slice(2, 9); }
export function clamp(n: number, lo: number, hi: number): number { return Math.min(hi, Math.max(lo, n)); }
// Reads a value the page remembered, or the fallback. The stored value is trusted to have the fallback's type.
export function persist<T>(key: string, fallback: T): T {
  try { const v = localStorage.getItem('lgtm-mock:' + key); return v == null ? fallback : JSON.parse(v); } catch { return fallback; }
}
// True when the page is drawn inside a frame on the repository Design page. A framed copy
// never writes to storage, so it cannot change the page around it.
export const FRAMED = (() => { try { return window.top !== window.self; } catch { return true; } })();
export function remember(key: string, value: unknown): void {
  if (FRAMED) return;
  try { localStorage.setItem('lgtm-mock:' + key, JSON.stringify(value)); } catch { /* storage is optional */ }
}
export function useMedia(query: string): boolean {
  const [on, setOn] = useState(() => matchMedia(query).matches);
  useEffect(() => { const m = matchMedia(query); const f = () => setOn(m.matches); m.addEventListener('change', f); return () => m.removeEventListener('change', f); }, [query]);
  return on;
}
export function useKey(handler: (e: KeyboardEvent) => void, deps: unknown[] = []): void {
  useEffect(() => { addEventListener('keydown', handler); return () => removeEventListener('keydown', handler); }, deps);
}
export function useOutside(ref: { current: Element | null }, onOutside: (e: PointerEvent) => void, active = true): void {
  useEffect(() => {
    if (!active) return;
    const f = (e: PointerEvent) => { if (ref.current && !ref.current.contains(e.target as Node)) onOutside(e); };
    const t = setTimeout(() => addEventListener('pointerdown', f), 0);
    return () => { clearTimeout(t); removeEventListener('pointerdown', f); };
  }, [active]);
}
