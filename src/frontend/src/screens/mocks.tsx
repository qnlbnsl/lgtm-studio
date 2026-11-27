// The Design view of a project, which was the Mocks tab until 2026-11-23. It follows the Workflow tab's pattern: one secondary tab row,
// the same page padding, and the same structure when there is nothing in it yet.
// A mock screen here is drawn from the same tokens and parts as the page itself.
import { useState, useEffect, useRef, useLayoutEffect } from 'preact/hooks';
import { live, refuse, Icon, cx, href, go, ago, clock, useNow, plural, update, uid, MIN, HOUR, DAY } from '../lib';
import { byId, agentOf, projectOf, repoOf, leadOf, threadOf, routeForAgent, placePath, isMain, MAIN } from '../data';
import * as act from '../actions';
import * as mocks from '../live/mocks';
import { Avatar, Back, OwnerAvatar, Dialog, registerDialog, Empty } from '../ui';
import { useCanvas, Canvas, ZoomControls, RepoDesign, type CanvasState } from './design';
import { MapTab } from './map';
import { BrowserPage } from './browser';
import type { ComponentType } from 'preact';
import type { DisplaySelection } from '../live/mocks';
import type { Id, Ms, Project, Route, State } from '../types';
import type { SeededSlices } from '../data';
import type { RepoDesignRecord } from './design';

interface MockComment { from: Id; ts: Ms; text: string }
/** One drawn screen of a project's mocks. */
export interface MockScreen {
  id: Id; name: string; kind: string; status: 'draft' | 'changed' | 'approved' | string; rev: number; updatedAt: Ms; note: string;
  comments: MockComment[]; replying?: boolean; selection?: DisplaySelection;
}
interface MockChange { id: Id; screen: Id; from: number; to: number; ts: Ms; by: Id; seen: boolean; compare?: boolean; text: string; name?: string }
interface MockField { field: string; on: string; sample: string; schema: 'matches' | 'differs' | 'new' | 'unavailable' | string; note?: string }
interface MockComponent { name: string; real: boolean; uses: number; on: string[]; demo: string; note?: string }
/** A project's mocks: the Mocks tab's slice of the store, for one project. Live mode adds the engine's revision fields. */
export interface Mocks {
  approvedAt: Ms | null; screens: MockScreen[]; changes: MockChange[]; data: MockField[]; components: MockComponent[];
  revisionId?: string; designRevision?: string; sources?: unknown[]; artifacts?: unknown[];
}

const VIEWPORTS: Record<string, [number, number]> = { phone: [390, 780], desktop: [1180, 780] };
const STATUS: Record<string, [string, string]> = { draft: ['', 'Draft'], changed: ['warn', 'Changed'], approved: ['success', 'Approved'] };

// ---- the drawn screens ---------------------------------------------------------------
const ROWS = [
  ['Question', 'Where should an agent\'s thread open?', 'Build lead · Agent console', '18 min'],
  ['Approval', 'Run the web tests', 'Builder 2 · Agent console', '6 min'],
  ['Approval', 'Read the review trace', 'Reviewer · Agent console', '2 min'],
  ['Merge', 'Merge docs/api-reference into main', 'Project lead · API reference', '1 hr 25 min'],
  ['Question', 'Statement 7 is stuck. Keep, change or drop it?', 'Planning lead · Standing permission rules', '34 min'],
];
function PhoneTabs({ at }: { at: string }) {
  return <nav class="mu-tabs">
    {[['bell', 'Requests'], ['kanban', 'Projects'], ['settings', 'Settings']].map(([i, l]) => <span key={l} class={cx(at === l && 'on')}><Icon name={i} size={18} />{l}</span>)}
  </nav>;
}
function RequestBody({ before }: { before?: boolean }) {
  return <div class="mu-req">
    <span class="mu-kind">Approval</span>
    <h4>Run the web tests</h4>
    <dl>
      <dt>Command</dt>
      <dd>pnpm --filter @lgtm/web test</dd>
      <dt>Folder</dt>
      <dd>~/projects/lgtm-studio/.worktrees/agent-console-shell</dd>
    </dl>
    <p class="mu-agent">
      <span class="avatar sm"><Icon name="hammer" size={12} /></span>
      <b>Builder 2</b>
      <span>Build team · Shell</span>
      <span class="grow" />
      <span>Waiting 6 min</span>
    </p>
    <div class="mu-actions">
      <span class="btn primary lg">Allow</span>
      {!before && <span class="btn lg">Always allow</span>}
      <span class="btn lg ghost">Deny</span>
    </div>
  </div>;
}
function ListUI({ before }: { before?: boolean }) {
  return <div class="mu mu-split">
    <div class="mu-main">
      <header class="mu-bar"><b>Requests</b><span class="count warn">5</span><span class="grow" /><Icon name="search" size={18} /></header>
      {!before && <div class="mu-chips"><span class="on">All 5</span><span>Questions 2</span><span>Approvals 2</span><span>Merges 1</span></div>}
      <ul class="mu-list">
        {ROWS.map((r, i) => <li key={i} class={cx(i === 1 && 'sel')}>
        <span class="mu-kind">{r[0]}</span>
        {!before && <time>{r[3]}</time>}
        <b>{r[1]}</b>
        <span>{r[2]}</span>
      </li>)}
      </ul>
      <PhoneTabs at="Requests" />
    </div>
    <div class="mu-side"><RequestBody before={before} /></div>
  </div>;
}
function RequestUI({ before }: { before?: boolean }) {
  return <div class="mu">
    <div class="mu-main">
      <header class="mu-bar">
        <Icon name="chevron-left" size={20} />
        <b>Request</b>
        <span class="grow" />
        <span class="mu-of">2 of 5</span>
      </header>
      <RequestBody before={before} />
      <PhoneTabs at="Requests" />
    </div>
  </div>;
}
function SettingsUI() {
  const rows: [string, string, boolean][] = [['An agent asks you a question', 'Sent at once.', true], ['An approval has waited 10 minutes', 'One alert for each approval.', true], ['A stalled agent reached you', 'Only after its lead could not help.', true], ['A full run failed', 'Nightly runs are in the morning summary.', false]];
  return <div class="mu">
    <div class="mu-main">
      <header class="mu-bar"><b>Notifications</b></header>
      <div class="mu-form">
        <h5>Alerts on this phone</h5>
        {rows.map(([t, h, on]) => <div class="mu-set" key={t}><div><b>{t}</b><span>{h}</span></div><span class="switch" aria-checked={on} /></div>)}
        <h5>Quiet hours</h5>
        <div class="mu-set">
          <div><b>22:00 to 07:00</b><span>Failed starts still come through.</span></div>
          <Icon name="chevron-right" size={16} />
        </div>
        <h5>Server</h5>
        <div class="mu-set">
          <div><b>ntfy.example.net</b><span>Topic lgtm-owner · last alert 6 min ago</span></div>
          <span class="badge success">Connected</span>
        </div>
      </div>
      <PhoneTabs at="Settings" />
    </div>
  </div>;
}
function BoardUI() {
  const cols: [string, [string, string][]][] = [['Planned', [['LS-A01-5', 'Project board'], ['LS-A01-6', 'Git and merge review']]], ['In progress', [['LS-A01-9', 'Fix the flaky reconnect test'], ['LS-A01-3', 'Sidebar and attention'], ['LS-A01-2', 'Thread view']]], ['Review', [['LS-A01-4', 'Narrow-screen pass']]], ['Done', [['LS-A01-1', 'Navigation foundation']]]];
  return <div class="mu">
    <div class="mu-main">
      <header class="mu-bar">
        <b>Agent console</b>
        <span class="badge info">Build</span>
        <span class="grow" />
        <Icon name="message-square" size={18} />
      </header>
      <div class="mu-board">
        {cols.map(([c, cards]) => <section key={c}>
      <h5>{c} <span>{cards.length}</span></h5>
      {cards.map(([l, n], i) => <div class="mu-card" key={l}>
      <span class="tag">{l}</span>
      <b>{n}</b>
      <div class="progress"><i style={{ width: (c === 'Done' ? 100 : c === 'Planned' ? 0 : 30 + i * 22) + '%' }} /></div>
    </div>)}
    </section>)}
      </div>
      <PhoneTabs at="Projects" />
    </div>
  </div>;
}
function ChatUI() {
  return <div class="mu">
    <div class="mu-main">
      <header class="mu-bar">
        <Icon name="chevron-left" size={20} />
        <span class="avatar sm lead"><Icon name="crown" size={12} /></span>
        <b>Project lead</b>
        <span class="grow" />
        <span class="mu-of">Waiting on 3</span>
      </header>
      <div class="mu-thread">
        <p class="me">Good. Start the shell and threads teams. Keep the review team small.</p>
        <p>Started <b>Build team · Shell</b> and <b>Build team · Threads</b>, with one reviewer in the Review team.</p>
        <p>Three things need you before the shell team can finish.</p>
        <div class="mu-mini"><span class="mu-kind">Question</span><b>Where should an agent's thread open?</b></div>
        <div class="mu-mini"><span class="mu-kind">Approval</span><b>Run the web tests</b></div>
      </div>
      <div class="mu-compose"><span>Message the project lead…</span><i><Icon name="arrow-up" size={15} /></i></div>
    </div>
  </div>;
}
const DRAW: Record<string, ComponentType<{ before?: boolean }>> = { list: ListUI, request: RequestUI, settings: SettingsUI, board: BoardUI, chat: ChatUI };

// A screen at its real size, scaled to fit the space it is given.
interface FrameProps { kind: string; viewport?: string; before?: boolean; fit?: 'width' | 'both'; max?: number }
function Frame({ kind, viewport = 'phone', before, fit = 'width', max = 1 }: FrameProps) {
  const box = useRef<HTMLDivElement>(null); const [k, setK] = useState(0); const [w, h] = VIEWPORTS[viewport]!; const UI = DRAW[kind] || ListUI;
  useLayoutEffect(() => {
    const el = box.current!; const measure = () => { const r = el.getBoundingClientRect(); if (r.width) setK(Math.max(.05, Math.min(max, r.width / w, fit === 'both' ? r.height / h : 9))); };
    measure(); const ro = new ResizeObserver(measure); ro.observe(el); return () => ro.disconnect();
  }, [w, h, fit, max]);
  return <div ref={box} class={cx('mk-box', fit)}>
    <div class="mk-fit" style={{ width: w * k + 'px', height: h * k + 'px', visibility: k ? undefined : 'hidden' }}>
      <div class={cx('mk-frame', viewport)} style={{ width: w + 'px', height: h + 'px', transform: `scale(${k})` }} aria-hidden="true" inert>
        {<UI before={before} />}
      </div>
    </div>
  </div>;
}

// ---- sample records ------------------------------------------------------------------
function blank(): Mocks { return { approvedAt: null, screens: [], changes: [], data: [], components: [] }; }
// The live website gives every project an empty record until the engine holds one.
export const emptyMocks = () => blank();
function ensure(s: State, p: Project): Mocks {
  s.mocks = s.mocks || {};
  if (!s.mocks[p.id]) { const m = blank(); if (p.hasMocks) m.screens.push({ id: 'first', name: 'First screen', kind: 'list', status: 'draft', rev: 1, updatedAt: Date.now(), note: 'The Designer is drawing this screen now.', comments: [] }); s.mocks[p.id] = m; }
  return s.mocks[p.id];
}
const COMPONENTS: MockComponent[] = [
  { name: 'Request block', real: true, uses: 9, on: ['Requests', 'Request'], demo: 'req' },
  { name: 'Button', real: true, uses: 14, on: ['Request', 'Notification settings'], demo: 'button' },
  { name: 'Switch', real: true, uses: 4, on: ['Notification settings'], demo: 'switch' },
  { name: 'Role avatar', real: true, uses: 7, on: ['Requests', 'Request'], demo: 'avatar' },
  { name: 'Badge', real: true, uses: 6, on: ['Requests', 'Notification settings'], demo: 'badge' },
  { name: 'Filter chips', real: false, uses: 1, on: ['Requests'], demo: 'chips', note: 'New in revision 3. Not in the repository yet.' },
  { name: 'Bottom tab bar', real: false, uses: 3, on: ['Requests', 'Request', 'Notification settings'], demo: 'tabs', note: 'The page has no phone tab bar today.' },
];
export function seedMocks(state: Omit<State, SeededSlices> & { mocks?: Record<Id, Mocks>; design?: Record<Id, RepoDesignRecord> }): void {
  const T = state.T; const at = (ms: number) => T - ms; const D = 'design-d1';
  state.mocks = {};
  state.mocks.phone = {
    approvedAt: null,
    screens: [
      { id: 'requests', name: 'Requests', kind: 'list', status: 'changed', rev: 3, updatedAt: at(52 * MIN), note: 'Filter chips above the list, and the wait on every row.', comments: [
        { from: 'owner', ts: at(2 * HOUR), text: 'I cannot tell which request has waited longest.' },
        { from: D, ts: at(52 * MIN), text: 'Revision 3 puts the wait on every row and sorts the oldest first. I also added filter chips, so a long list stays short.' }] },
      { id: 'request', name: 'Request', kind: 'request', status: 'changed', rev: 2, updatedAt: at(3 * HOUR), note: 'A third choice, Always allow.', comments: [
        { from: D, ts: at(3 * HOUR), text: 'Revision 2 adds Always allow. It makes a standing rule, so it depends on the Standing permission rules project.' }] },
      { id: 'notify', name: 'Notification settings', kind: 'settings', status: 'draft', rev: 1, updatedAt: at(4 * HOUR), note: 'The four alerts, quiet hours, and the server.', comments: [] },
    ],
    changes: [
      { id: 'c1', screen: 'requests', from: 2, to: 3, ts: at(52 * MIN), by: D, seen: false, compare: true, text: 'Added filter chips for questions, approvals and merges. Every row now shows how long it has waited.' },
      { id: 'c2', screen: 'request', from: 1, to: 2, ts: at(3 * HOUR), by: D, seen: false, compare: true, text: 'Added Always allow between Allow and Deny.' },
      { id: 'c3', screen: 'notify', from: 0, to: 1, ts: at(4 * HOUR), by: D, seen: true, text: 'First drawing of the screen.' },
    ],
    data: [
      { field: 'request.title', on: 'Requests, Request', sample: 'Run the web tests', schema: 'matches' },
      { field: 'request.kind', on: 'Requests, Request', sample: 'approval', schema: 'matches' },
      { field: 'request.createdAt', on: 'Requests, Request', sample: '6 min ago', schema: 'matches' },
      { field: 'request.command', on: 'Request', sample: 'pnpm --filter @lgtm/web test', schema: 'matches' },
      { field: 'request.folder', on: 'Request', sample: '~/projects/lgtm-studio/.worktrees/…', schema: 'matches' },
      { field: 'agent.role', on: 'Requests, Request', sample: 'Builder 2', schema: 'differs', note: 'The schema has no number. The mock adds one to tell builders apart.' },
      { field: 'request.rule', on: 'Request', sample: 'always', schema: 'new', note: 'Needs the Standing permission rules project.' },
      { field: 'alerts.stalledAgent', on: 'Notification settings', sample: 'on', schema: 'new', note: 'Phone alerts are not stored yet.' },
      { field: 'alerts.quietHours', on: 'Notification settings', sample: '22:00 to 07:00', schema: 'new', note: 'Phone alerts are not stored yet.' },
      { field: 'ntfy.lastAlert', on: 'Notification settings', sample: '6 min ago', schema: 'unavailable', note: 'No review was supplied for this field.' },
    ],
    components: COMPONENTS,
  };
  state.mocks.console = {
    approvedAt: at(3 * DAY + 2 * HOUR),
    screens: [
      { id: 'home', name: 'Home', kind: 'list', status: 'approved', rev: 4, updatedAt: at(3 * DAY + 3 * HOUR), note: 'Requests across every project, oldest first, with the wait on every row.', comments: [{ from: 'owner', ts: at(3 * DAY + 5 * HOUR), text: 'Put the wait on every row. I want to see the oldest request first.' }, { from: 'console-lead', ts: at(3 * DAY + 3 * HOUR), text: 'Done in revision 4. Every row shows its wait, and the oldest is first.' }] },
      { id: 'board', name: 'Project board', kind: 'board', status: 'approved', rev: 3, updatedAt: at(3 * DAY + 4 * HOUR), note: 'Sprints in four columns, with the lead beside them.', comments: [] },
      { id: 'thread', name: 'Agent thread', kind: 'chat', status: 'approved', rev: 2, updatedAt: at(3 * DAY + 6 * HOUR), note: 'One thread for each agent, with requests in line.', comments: [] },
      { id: 'request', name: 'Request', kind: 'request', status: 'approved', rev: 2, updatedAt: at(3 * DAY + 6 * HOUR), note: 'One block for questions, approvals, merges and failed starts.', comments: [] },
    ],
    changes: [
      { id: 'c1', screen: 'home', from: 3, to: 4, ts: at(3 * DAY + 3 * HOUR), by: 'console-lead', seen: true, compare: true, text: 'Request rows show their wait, and filters sit above the list.' },
      { id: 'c2', screen: 'board', from: 2, to: 3, ts: at(3 * DAY + 4 * HOUR), by: 'console-lead', seen: true, text: 'The lead chat moved into a panel on the right. The Board row left the tree.' },
    ],
    data: [
      { field: 'request.title', on: 'Home, Request', sample: 'Run the web tests', schema: 'matches' },
      { field: 'request.createdAt', on: 'Home, Request', sample: '6 min ago', schema: 'matches' },
      { field: 'sprint.column', on: 'Project board', sample: 'In progress', schema: 'matches' },
      { field: 'sprint.progress', on: 'Project board', sample: '2 of 4', schema: 'differs', note: 'The engine counts tasks. The mock shows a share.' },
      { field: 'agent.contextUse', on: 'Agent thread', sample: '34%', schema: 'unavailable', note: 'Some harnesses do not report it.' },
    ],
    components: COMPONENTS.slice(0, 5).map((c) => ({ ...c, on: ['Home', 'Request'] })),
  };
  // The main checkout of a repository holds every screen that is drawn in it. A project holds
  // only the screens it draws or changes.
  const merged = at(3 * DAY + 2 * HOUR);
  state.mocks[MAIN + 'lgtm'] = { approvedAt: merged, changes: [], data: [], components: [], screens: [
    { id: 'home', name: 'Home', kind: 'list', status: 'approved', rev: 4, updatedAt: merged, note: 'Requests across every project, oldest first, with the wait on every row.', comments: [] },
    { id: 'board', name: 'Project board', kind: 'board', status: 'approved', rev: 3, updatedAt: merged, note: 'Sprints in four columns, with the lead beside them.', comments: [] },
    { id: 'thread', name: 'Agent thread', kind: 'chat', status: 'approved', rev: 2, updatedAt: merged, note: 'One thread for each agent, with requests in line.', comments: [] },
    { id: 'request', name: 'Request', kind: 'request', status: 'approved', rev: 1, updatedAt: at(9 * DAY), note: 'One block for questions, approvals, merges and failed starts.', comments: [] },
    { id: 'messages', name: 'Message board', kind: 'chat', status: 'approved', rev: 2, updatedAt: at(11 * DAY), note: 'One channel for each team, and one for the whole project.', comments: [] },
    { id: 'settings', name: 'Service settings', kind: 'settings', status: 'approved', rev: 1, updatedAt: at(14 * DAY), note: 'Appearance, defaults and remote access.', comments: [] },
  ] };
  state.design = {
    lgtm: { file: 'design/lgtm.fig', size: '4.8 megabytes', pages: 6, frames: 41, changedAt: at(52 * MIN), changedBy: D, commit: '4f9ae0a' },
  };
}

// ---- actions -------------------------------------------------------------------------
const designerOf = (s: State, p: Project) => s.agents.find((a) => a.project === p.id && a.role === 'designer') || leadOf(s, p);
function comment(s: State, p: Project, sc: MockScreen, text: string): void {
  if (live.on) { refuse(); return; }
  const d = designerOf(s, p);
  update(() => { sc.comments.push({ from: 'owner', ts: Date.now(), text }); sc.replying = true; });
  act.sampleLater(() => update(() => {
    sc.replying = false; if (ensure(s, p).approvedAt) { sc.comments.push({ from: d.id, ts: Date.now(), text: 'The design is approved, so I passed this to the project lead as a change request.' }); return; }
    const from = sc.rev; sc.rev += 1; sc.status = 'changed'; sc.updatedAt = Date.now(); sc.note = 'Changed after your comment.';
    sc.comments.push({ from: d.id, ts: Date.now(), text: `Done in revision ${sc.rev}. Open Changes to compare it with revision ${from}.` });
    ensure(s, p).changes.unshift({ id: uid('c'), screen: sc.id, from, to: sc.rev, ts: Date.now(), by: d.id, seen: false, text: `Changed after your comment: "${text}"` });
  }), 1800);
}
function approveAll(s: State, p: Project): void {
  if (live.on) return mocks.approveMocks(p);
  const m = ensure(s, p);
  act.confirm({ title: 'Approve the design?', danger: false, action: 'Approve design',
    body: `${plural(m.screens.length, 'screen')} become the reference for ${p.name}. The project moves to Plan, and the project lead starts a Planning team.`,
    onConfirm: () => { update((st) => {
      m.approvedAt = Date.now(); for (const sc of m.screens) sc.status = 'approved'; for (const c of m.changes) c.seen = true;
      const pr = projectOf(st, p.id); pr.stage = 'Plan'; pr.phase = 'design';
      for (const a of st.agents) if (a.project === p.id && a.team && byId(st.teams, a.team)?.stage === 'Design') { a.state = 'idle'; a.activity = ''; a.workState = 'done'; }
      const tid = p.id + '-plan';
      if (!byId(st.teams, tid)) {
        st.teams.push({ id: tid, project: p.id, stage: 'Plan', area: null, branch: pr.branch + '/plan', needToKnow: false });
        const mk = (id: Id, role: string, name: string, activity: string) => ({ id, project: p.id, team: tid, role, name, harness: 'claude', subscription: 'sub-max', model: role === 'team-lead' ? 'Opus 5.5' : 'Sonnet 5.5', effort: 'High', permission: 'Edit files without asking', state: 'working', activity, task: null, context: 4, turns: 1, tokens: 0.1, workState: 'working', stalled: false, startedAt: Date.now() });
        st.agents.push(mk(tid + '-lead', 'team-lead', 'Planning lead', 'Reading the approved design'), mk(tid + '-p1', 'planner', 'Planner 1', 'Listing the fields each screen uses'));
        st.channels.push({ id: 'ch-' + tid, project: p.id, kind: 'team', team: tid, name: 'planning', members: [tid + '-lead', tid + '-p1'] });
        st.expanded['team:' + tid] = true;
      }
      const lead = leadOf(st, pr); lead.state = 'working'; lead.activity = 'Splitting the work into sprints';
      threadOf(st, lead.id).push({ id: uid('m'), type: 'note', ts: Date.now(), text: 'You approved the design.' }, { id: uid('m'), type: 'agent', ts: Date.now() + 1, text: `The ${plural(m.screens.length, 'screen')} are now the reference. I started a **Planning team** and I am splitting the work into sprints. They appear on the board as they are sized.` });
    }); act.toast('Design approved. The project moved to Plan.'); go(`/p/${p.id}/design`); } });
}
registerDialog('mock-changes', ({ d, s }) => {
  const p = projectOf(s, d.project); const m = ensure(s, p); const des = designerOf(s, p);
  const [text, setText] = useState(''); const [sid, setSid] = useState(d.screen || '');
  const send = () => { if (!text.trim()) return; const sc = byId(m.screens, sid); if (live.on) return mocks.requestChanges(p, sc, text, des); act.closeDialog();
    if (sc) comment(s, p, sc, text.trim());
    act.sendMessage(des.id, sc ? `Change the ${sc.name} screen: ${text.trim()}` : `Change the mocks: ${text.trim()}`);
    act.toast(`Sent to the ${des.name}`); };
  return <Dialog title="Request changes" onClose={act.closeDialog} onSubmit={send} foot={<>
      <span class="why">Goes to the {des.name} as a message.</span>
      <button type="button" class="btn" onClick={act.closeDialog}>Cancel</button>
      <button type="submit" class="btn primary" disabled={!text.trim() || live.on && mocks.busy('mock-changes:' + p.id)}>
        {live.on && mocks.busy('mock-changes:' + p.id) ? 'Sending…' : 'Send'}
      </button>
    </>}>
    <div class="field">
      <label for="mc-screen">Screen</label>
      <select id="mc-screen" class="select" value={sid} onChange={(e) => setSid((e.target as HTMLSelectElement).value)}>
        <option value="">All screens</option>
        {m.screens.map((x) => <option key={x.id} value={x.id}>{x.name}</option>)}
      </select>
    </div>
    <div class="field">
      <label for="mc-text">Requested change</label>
      <textarea id="mc-text" class="textarea" rows={4} autofocus placeholder="Say what is wrong, or what you expected to see." value={text} onInput={(e) => setText((e.target as HTMLTextAreaElement).value)} />
    </div>
  </Dialog>;
});

// ---- views ---------------------------------------------------------------------------
function Status({ sc }: { sc: MockScreen }) { const [tone, word] = STATUS[sc.status]!; return <span class={cx('badge', tone)}>{sc.status === 'approved' && <Icon name="check" size={11} />}{word}</span>; }
const since = (ts: Ms, now: Ms): string => (ago(ts, now) === 'now' ? 'just now' : ago(ts, now) + ' ago');

interface MocksViewProps { s: State; p: Project; m: Mocks }
// The projects of a repository that are changing a screen of its main checkout.
const changing = (s: State, p: Project, id: Id): Project[] => s.projects.filter((x) => x.repo === p.repo && !x.archived && s.mocks?.[x.id]?.screens.some((sc: MockScreen) => sc.id === id && sc.status !== 'approved'));
function ChangingIn({ s, p, id }: { s: State; p: Project; id: Id }) {
  return <>{changing(s, p, id).map((x, n) => <>{n > 0 && ', '}<a key={x.id} class="link" href={href(`/p/${x.id}/design/screen/${id}`)} onClick={(e) => e.stopPropagation()}>{x.name}</a></>)}</>;
}
function Screens({ s, p, m }: MocksViewProps) {
  const now = useNow(); const main = isMain(p);
  return <div class="table-wrap">
    <table class="table mk-screens">
      <thead>
        <tr>
          <th class="mk-th-thumb" />
          <th>Screen</th>
          <th>{main ? 'Changing in' : 'Status'}</th>
          <th class="num">Revision</th>
          {!main && <th class="num">Comments</th>}
          <th class="num">Updated</th>
        </tr>
      </thead>
      <tbody>
        {m.screens.length === 0 && <tr><td /><td colspan={main ? 4 : 5} class="muted" data-empty>No screens</td></tr>}
        {m.screens.map((sc) => <tr key={sc.id} class={cx('click', sc.status === 'changed' && 's-warn')} onClick={() => go(`${placePath(p)}/design/screen/${sc.id}`)}>
      <td class="mk-td-thumb"><span class="mk-mini">{!live.on && <Frame kind={sc.kind} fit="both" max={.12} />}</span></td>
      <td class="wrap mk-note">
        <a href={href(`${placePath(p)}/design/screen/${sc.id}`)} style="font-weight:var(--weight-medium)">{sc.name}</a>
        {sc.note && <small class="mk-small">{sc.note}</small>}
      </td>
      <td>{main ? <ChangingIn s={s} p={p} id={sc.id} /> : sc.status !== 'approved' && <Status sc={sc} />}</td>
      <td class="num">{sc.rev}</td>
      {!main && <td class="num muted">{sc.comments.length || ''}</td>}
      <td class="num muted">{since(sc.updatedAt, now)}</td>
    </tr>)}
      </tbody>
    </table>
  </div>;
}

function ScreenView({ s, p, m, id }: MocksViewProps & { id: Id | undefined }) {
  const now = useNow(); const sc = byId(m.screens, id); const [vp, setVp] = useState('phone'); const [text, setText] = useState(''); const des = designerOf(s, p); const main = isMain(p);
  if (!sc) return <div class="page">
    <Empty title="This screen does not exist"><a class="link" href={href(`${placePath(p)}/design`)}>Screens</a></Empty>
  </div>;
  const i = m.screens.indexOf(sc); const prev = m.screens[i - 1], next = m.screens[i + 1];
  const send = (e: Event) => { e.preventDefault(); if (!text.trim()) return; comment(s, p, sc, text.trim()); setText(''); };
  return <div class="mk-detail">
    <div class="mk-stagecol">
      <div class="mk-stagebar">
        <Back path={`${placePath(p)}/design`} or={<a class="icon-btn" aria-label="Screens" href={href(`${placePath(p)}/design`)}><Icon name="arrow-left" size={15} /></a>} />
        <b class="mk-name">{sc.name}</b>
        {!main && <Status sc={sc} />}
        <span class="muted">Revision {sc.rev}</span>
        <span class="grow" />
        <div class="seg">
          {[['phone', 'smartphone', 'Phone'], ['desktop', 'monitor', 'Desktop']].map(([v, ic, l]: string[]) => <button type="button" key={v} aria-pressed={vp === v} aria-label={l} data-tip={l} onClick={() => setVp(v!)}>
          <Icon name={ic!} size={13} />
        </button>)}
        </div>
        <StageZoom />
        <a class="icon-btn" aria-label="Previous screen" aria-disabled={!prev} href={prev ? href(`${placePath(p)}/design/screen/${prev.id}`) : undefined}>
          <Icon name="chevron-left" size={16} />
        </a>
        <a class="icon-btn" aria-label="Next screen" aria-disabled={!next} href={next ? href(`${placePath(p)}/design/screen/${next.id}`) : undefined}>
          <Icon name="chevron-right" size={16} />
        </a>
      </div>
      {live.on ? <mocks.Display key={sc.id + sc.rev} selection={sc.selection} viewport={vp} title={sc.name + ' preview'} /> : <Stage key={vp + sc.id + sc.rev} kind={sc.kind} vp={vp} />}
    </div>
    {/* A screen of the main checkout is changed in a project, so it has no approval and no comments here. */}
    {main ? <aside class="mk-aside">
      <section>
        <p class="note">{sc.note}</p>
        {changing(s, p, sc.id).length > 0 && <dl class="kv"><dt>Changing in</dt><dd><ChangingIn s={s} p={p} id={sc.id} /></dd></dl>}
      </section>
    </aside> : <aside class="mk-aside">
      <section>
        <p class="note">{sc.note}</p>
        <div class="row" style="flex-wrap:wrap">
          {sc.status !== 'approved' && <button type="button" class="btn primary" onClick={() => { if (live.on) { refuse(); return; } update(() => { sc.status = 'approved'; for (const c of m.changes) if (c.screen === sc.id) c.seen = true; }); act.toast(`${sc.name} approved`); }}>
            <Icon name="check" size={14} />
            Approve
          </button>}
          {live.on ? <mocks.Display key={m.designRevision} title="Mock design editor" action="Edit design" disabled={!m.designRevision} selection={m.designRevision ? { _tag: 'editor', repoId: p.repo, projectId: p.id, revisionId: m.designRevision } : null} /> : <button type="button" class="btn ghost" onClick={() => act.toast(`Opening ${sc.name} in OpenPencil`)}>
            <Icon name="pencil-ruler" size={14} />
            Edit design
          </button>}
        </div>
      </section>
      <section class="mk-comments">
        <h3>Comments</h3>
        {sc.comments.length === 0 && !sc.replying && <p class="muted" data-empty>No comments</p>}
        <ol>
          {sc.comments.map((c, n) => { const a = c.from === 'owner' ? null : agentOf(s, c.from);
          return <li key={n}>
            {a ? <Avatar agent={a} s={s} size="sm" status={false} /> : <OwnerAvatar s={s} size="sm" />}
            <div><p class="who"><b>{a ? a.name : 'You'}</b><time>{since(c.ts, now)}</time></p><p>{c.text}</p></div>
          </li>; })}
          {sc.replying && <li><Avatar agent={des} s={s} size="sm" status={false} /><div><span class="typing"><i /><i /><i /></span></div></li>}
        </ol>
        <form class="mk-say" onSubmit={send}>
          <input class="input" placeholder={`Request a change from the ${des.name.toLowerCase()}`} aria-label="Comment" value={text} onInput={(e) => setText((e.target as HTMLInputElement).value)} />
          <button type="submit" class="btn" disabled={!text.trim()}>Send</button>
        </form>
      </section>
    </aside>}
  </div>;
}
// The large preview uses the Design page's canvas: the screen at its real size, with zoom and Fit.
const stage: { c: CanvasState | null; listeners: Set<() => void> } = { c: null, listeners: new Set() };
function StageZoom() {
  const [, force] = useState(0);
  useLayoutEffect(() => { const f = () => force((n) => n + 1); stage.listeners.add(f); return () => { stage.listeners.delete(f); }; }, []);
  return stage.c ? <ZoomControls c={stage.c} /> : null;
}
function Stage({ kind, vp }: { kind: string; vp: string }) {
  const c = useCanvas({ initial: 'fit' }); stage.c = c;
  useLayoutEffect(() => { for (const f of stage.listeners) f(); });
  const [w, h] = VIEWPORTS[vp]!; const UI = DRAW[kind] || ListUI;
  return <Canvas c={c}>
    <div class={cx('mk-frame flat', vp)} style={{ width: w + 'px', height: h + 'px' }} aria-hidden="true" inert><UI /></div>
  </Canvas>;
}

function Changes({ s, p, m }: MocksViewProps) {
  const now = useNow(); const [open, setOpen] = useState<Id | null>(null);
  return <div class="table-wrap">
    <table class="table mk-changes">
      <thead><tr><th>Screen</th><th>Revision</th><th>Change</th><th>By</th><th class="num">Date</th></tr></thead>
      <tbody>
        {m.changes.length === 0 && <tr><td colspan={5} class="muted" data-empty>No changes</td></tr>}
        {m.changes.map((c) => { const sc = byId(m.screens, c.screen); const by = agentOf(s, c.by); if (!sc && !live.on) return null; const ex = open === c.id;
      return <>
        <tr key={c.id} class={cx(c.compare && 'click', !live.on && !c.seen && 's-warn')} onClick={() => c.compare && setOpen(ex ? null : c.id)}>
          <td class="state-cell">
            <a class="link" href={href(sc ? `${placePath(p)}/design/screen/${sc.id}` : `/p/${p.id}/design`)} onClick={(e) => e.stopPropagation()}>
              {sc ? sc.name : c.name}
            </a>
            {!live.on && !c.seen && <> <span class="badge warn">New</span></>}
          </td>
          <td class="mono muted">{live.on ? c.to : c.from ? `${c.from} → ${c.to}` : '1'}</td>
          <td class="wrap">{c.text}{c.compare && <> <span class="link mk-cmp">{ex ? 'Hide' : 'Compare'}</span></>}</td>
          <td class="muted">{by ? by.name : live.on ? (c.by === 'owner' ? 'Owner' : c.by) : 'Designer'}</td>
          <td class="num muted">{since(c.ts, now)}</td>
        </tr>
        {ex && <tr class="mk-cmprow">
          <td colspan={5}>
            <div class="mk-pair">
              <figure>
                <figcaption>Revision {c.from}</figcaption>
                <div class="mk-thumb"><Frame kind={sc!.kind} before={true} max={.42} /></div>
              </figure>
              <figure><figcaption>Revision {c.to}</figcaption><div class="mk-thumb"><Frame kind={sc!.kind} max={.42} /></div></figure>
            </div>
          </td>
        </tr>}
      </>; })}
      </tbody>
    </table>
  </div>;
}

const SCHEMA: Record<string, [string, string]> = { matches: ['success', 'Matches'], differs: ['warn', 'Differs'], new: ['info', 'New'], unavailable: ['', 'Unavailable'] };
function Data({ m }: { m: Mocks }) {
  const [f, setF] = useState('all'); const rows = m.data.filter((d) => f === 'all' || d.schema === f);
  return <>
    <div class="wf-bar">
      <div class="seg">
        {['all', 'new', 'differs', 'matches', 'unavailable'].map((k) => <button type="button" key={k} aria-pressed={f === k} onClick={() => setF(k)}>
    {k === 'all' ? 'All' : SCHEMA[k]![1]}
    <span class="faint num">{k === 'all' ? m.data.length : m.data.filter((d) => d.schema === k).length}</span>
  </button>)}
      </div>
    </div>
    <div class="table-wrap">
      <table class="table">
        <thead><tr><th>Field</th><th>Schema</th><th>Sample value</th><th>Shown on</th><th>Note</th></tr></thead>
        <tbody>
          {rows.length === 0 && <tr><td colspan={5} class="muted" data-empty>No fields</td></tr>}
          {rows.map((d) => <tr key={d.field} class={cx(d.schema === 'differs' && 's-warn')}>
        <td class="state-cell mono">{d.field}</td>
        <td><span class={cx('badge', SCHEMA[d.schema]![0])}>{SCHEMA[d.schema]![1]}</span></td>
        <td class="mono muted truncate" style="max-width:220px">{d.sample}</td>
        <td class="muted">{d.on}</td>
        <td class="wrap muted mk-note">{d.note || ''}</td>
      </tr>)}
        </tbody>
      </table>
    </div>
  </>;
}
function Demo({ kind }: { kind: string }) {
  if (kind === 'button') return <span class="row"><span class="btn sm primary">Allow</span><span class="btn sm">Deny</span></span>;
  if (kind === 'switch') return <span class="row"><span class="switch" aria-checked="true" /><span class="switch" aria-checked="false" /></span>;
  if (kind === 'avatar') return <span class="row" style="gap:4px">
    <span class="avatar sm lead"><Icon name="crown" size={12} /></span>
    <span class="avatar sm"><Icon name="flag" size={12} /></span>
    <span class="avatar sm"><Icon name="hammer" size={12} /></span>
  </span>;
  if (kind === 'badge') return <span class="row" style="gap:4px"><span class="badge warn">Waiting</span><span class="badge success">Connected</span></span>;
  if (kind === 'chips') return <span class="mu-chips demo"><span class="on">All 5</span><span>Questions 2</span></span>;
  if (kind === 'tabs') return <span class="row muted" style="gap:10px">
    <Icon name="bell" size={15} />
    <Icon name="kanban" size={15} />
    <Icon name="settings" size={15} />
  </span>;
  return <span class="mk-reqdemo"><b>Approval</b><i /><i /></span>;
}
function Components({ m }: { m: Mocks }) {
  return <div class="table-wrap">
    <table class="table mk-comps">
      <thead><tr><th>Component</th><th>Source</th><th>Preview</th><th class="num">Uses</th><th>Screens</th></tr></thead>
      <tbody>
        {m.components.length === 0 && <tr><td colspan={5} class="muted" data-empty>No components</td></tr>}
        {m.components.map((c) => <tr key={c.name} class={cx(!c.real && 's-warn')}>
        <td class="state-cell wrap">
          <b style="font-weight:var(--weight-medium)">{c.name}</b>
          {c.note && <small class="mk-small">{c.note}</small>}
        </td>
        <td><span class={cx('badge', c.real ? 'success' : 'warn')}>{c.real ? 'Real' : 'Draft'}</span></td>
        <td>{!live.on && <Demo kind={c.demo} />}</td>
        <td class="num">{c.uses}</td>
        <td class="muted">{c.on.join(', ')}</td>
      </tr>)}
      </tbody>
    </table>
  </div>;
}

/** The drawn screens of a repository's main checkout: all of them, or the one that is open. */
export const mocksOf = (s: State, p: Project): Mocks => ensure(s, p);
export function RepoScreens({ s, project: p, open }: { s: State; project: Project; open?: Id | null }) {
  const m = ensure(s, p);
  return open ? <ScreenView key={open} s={s} p={p} m={m} id={open} /> : <div class="wf-body"><Screens s={s} p={p} m={m} /></div>;
}
export function DesignTab({ s, project: p, route }: { s: State; project: Project; route: Route }) {
  const m = ensure(s, p); const now = useNow();
  useEffect(() => { if (live.on) void mocks.loadMocks(p.id); }, [p.id, s.boardRevision]);
  const view = route.parts[3] || 'screens';
  const unseen = live.on ? 0 : m.changes.filter((c) => !c.seen).length;
  // The first four pages are the drawn design. The design file is the whole file on a canvas. The
  // browser shows the product as it runs, and the architecture is the map of its code.
  const tabs: [string, string, number, string?][] = [['screens', 'Screens', m.screens.length], ['changes', 'Changes', unseen, 'warn'], ['components', 'Components', m.components.length], ['data', 'Data', m.data.length],
    ...(live.on ? [] : [['file', 'Design file', 0], ['browser', 'Browser', 0], ['architecture', 'Architecture', 0]] as [string, string, number][])];
  const open = m.screens.filter((x) => x.status !== 'approved').length;
  const row = useRef<HTMLElement>(null);
  useEffect(() => { row.current?.querySelector('[aria-current="page"]')?.scrollIntoView({ block: 'nearest', inline: 'nearest' }); }, [view]);
  const nav = <nav class="wf-nav mk-nav" aria-label="Design views" ref={row}>
    <Back path={`/p/${p.id}/design`} class="wf-back" />
    {tabs.flatMap(([id, label, n, tone]) => [
      (id === 'browser' || id === 'architecture') && <span key={id + '-sep'} class="mk-sep" aria-hidden="true" />,
      <a key={id} class="wf-tab" aria-current={(view === 'screen' ? 'screens' : view) === id ? 'page' : undefined} href={href(`/p/${p.id}/design${id === 'screens' ? '' : '/' + id}`)}>
        {label}
        {n > 0 && <span class={cx('count', tone)}>{n}</span>}
      </a>,
    ])}
  </nav>;
  if (view === 'screen') return <div class="wf mk-full">{nav}<ScreenView key={route.parts[4]} s={s} p={p} m={m} id={route.parts[4]} /></div>;
  if (!live.on && view === 'architecture') return <div class="wf mk-full">{!s.ui.focus && nav}<MapTab s={s} project={p} /></div>;
  if (!live.on && view === 'file') return <div class="wf mk-full">{nav}<RepoDesign s={s} repoId={p.repo} within={{ base: `/p/${p.id}/design/file`, cat: route.parts[4], item: route.parts[5] }} /></div>;
  if (!live.on && view === 'browser') return <div class="wf mk-full">{nav}<BrowserPage s={s} project={p} route={route} /></div>;
  const body = view === 'changes' ? <Changes s={s} p={p} m={m} /> : view === 'data' ? <Data m={m} /> : view === 'components' ? <Components m={m} /> : <Screens s={s} p={p} m={m} />;
  return <div class="wf" data-main aria-busy={live.on && mocks.busy('read-mocks:' + p.id)}>
    {nav}
    <div class="wf-body">
      <div class="wf-status">
        {m.approvedAt ? <><span class="badge success"><Icon name="check" size={11} />Approved</span><span>{since(m.approvedAt, now)}</span></>
          : <span><b class="num">{m.screens.length - open}</b> of {plural(m.screens.length, 'screen')} approved</span>}
        {unseen > 0 && <span class="warn-text">{plural(unseen, 'change')} not seen</span>}
        <span class="grow" />
        {live.on && <button type="button" class="icon-btn sm" aria-label="Refresh the design" disabled={mocks.busy('read-mocks:' + p.id)} onClick={() => mocks.loadMocks(p.id)}>
          <Icon name="refresh-cw" size={13} />
        </button>}
        {view === 'changes' && unseen > 0 && <button type="button" class="btn sm ghost" onClick={() => update(() => { for (const c of m.changes) c.seen = true; })}>
          <Icon name="check" size={13} />
          Mark seen
        </button>}
        <button type="button" class="btn sm" disabled={!m.screens.length} onClick={() => act.openDialog({ type: 'mock-changes', project: p.id, screen: '' })}>
          <Icon name="message-square" size={13} />
          Request changes
        </button>
        {!m.approvedAt && <button type="button" class="btn sm primary" disabled={!m.screens.length || live.on && mocks.busy('approve-mocks:' + p.id)} onClick={() => approveAll(s, p)}>
          <Icon name="check" size={13} />
          {live.on && mocks.busy('approve-mocks:' + p.id) ? 'Approving…' : 'Approve design'}
        </button>}
      </div>
      {body}
    </div>
  </div>;
}
