// The project board, sprint and task pages, the team page, and the repository page.
import { useState, useRef, useEffect } from 'preact/hooks';
import { Icon, cx, href, go, swap, useQuery, useMedia, ago, clock, dayLabel, useNow, plural, update, uid, useKey, store, DAY, live } from '../lib';
import {
  STAGES, PHASES, COLUMNS, PRIORITIES, TASK_STATES, sprintsOf, epicsOf, tasksOfSprint, tasksOfTeam, teamOf, teamName, teamsOf, agentOf, agentsOfTeam, agentsOfProject,
  requestsOfSprint, requestsOfTask, openRequests, sprintProgress, agentStatus, byId, repoOf, machineOf, leadOf, routeForAgent, agentLabel, HARNESSES, subLabel,
  moveNeeds, nextLane, LANE_STAGE,
} from '../data';
import * as act from '../actions';
import * as board from '../live/board';
import { Avatar, Back, RequestBlock, Empty, StateWord, AgentLine, Menu } from '../ui';
import { ProjectTable } from './home';
import { AgentChanges } from './git';
import type { ComponentChildren, JSX, TargetedDragEvent, TargetedKeyboardEvent } from 'preact';
import type { LaneNeed } from '../data';
import { taskPath, nextLabel } from '../data';
import type { Agent, Id, Project, Repo, Route, Sprint, State, Task, TaskEvent } from '../types';

// ---- status line ----------------------------------------------------------------
// The project's stage and phase as one quiet line that links to Workflow.
export function StatusLine({ s, project }: { s: State; project: Project }) {
  if (live.on) return <a class="status-link" href={href(`/p/${project.id}/status`)}>
    <span>{project.stage}</span>
    <Icon name="arrow-right" size={12} class="faint" />
  </a>;
  const phase = PHASES.find((p) => p.id === project.phase)!;
  return <a class="status-link" href={href(`/p/${project.id}/status`)} title="Open Workflow">
    <span>{phase.name}</span>
    <span class="muted">· Phase {phase.n} of 8</span>
    <Icon name="arrow-right" size={12} class="faint" />
  </a>;
}

// ---- board ----------------------------------------------------------------------
// One project, three views of the same items, after GitHub Projects: Board groups by
// status, Table sorts and filters, Roadmap lays epics and sprints along a time line.
const PRIO_ICON: Record<string, string> = { Urgent: 'zap', High: 'arrow-up', Medium: 'minus', Low: 'arrow-down' };
function Priority({ p, label = false }: { p: string; label?: boolean }) {
  if (live.on) return null;
  return <span class={cx('prio', 'p-' + p.toLowerCase())} title={'Priority: ' + p}>
    <Icon name={PRIO_ICON[p]!} size={12} />
    {label && <span>{p}</span>}
  </span>;
}
interface CardTip { show: (sp: Sprint, el: HTMLElement, now?: boolean) => void; hide: () => void; el: JSX.Element | null }
interface CardDrag { can: boolean; cls: string; start: (e: DragEvent) => void; end: () => void }
function SprintCard({ s, sp, hover, dnd, inRepo }: { s: State; sp: Sprint; hover: CardTip | null; dnd: CardDrag | false; inRepo?: boolean }) {
  const pr = sprintProgress(s, sp); const reqs = requestsOfSprint(s, sp.id);
  const t = sp.team ? teamOf(s, sp.team) : undefined; const ep = sp.epic ? byId(s.epics, sp.epic) : undefined;
  const failed = reqs.some((r) => r.kind === 'failed');
  return <a class={cx('sprint-card', reqs.length && (failed ? 'error' : 'warn'), sp.column === 'Done' && 'done', dnd && dnd.cls)} href={href(`/p/${sp.project}/sprint/${sp.id}`)} data-card={sp.id} draggable={(dnd && dnd.can ? 'true' : 'false') as unknown as boolean} onDragStart={dnd && dnd.can ? dnd.start : undefined} onDragEnd={dnd && dnd.can ? dnd.end : undefined} onMouseEnter={hover ? ((e) => hover.show(sp, e.currentTarget)) : undefined} onMouseLeave={hover ? hover.hide : undefined} onFocus={hover ? ((e) => hover.show(sp, e.currentTarget, true)) : undefined} onBlur={hover ? hover.hide : undefined}>
    <div class="card-top">
      <span class="tag">{sp.label}</span>
      {inRepo ? <span class="epic-chip truncate">{byId(s.projects, sp.project)?.name}</span> : ep && <span class="epic-chip truncate">{ep.name}</span>}
      <span class="grow" />
      <Priority p={sp.priority || 'Medium'} />
    </div>
    <h3>{sp.name}</h3>
    {reqs.length > 0 && <div class={cx('need', failed && 'error')}>
      <Icon name={failed ? 'circle-alert' : 'bell'} size={13} />
      {reqs.length === 1 ? ({ question: 'Awaiting answer', approval: 'Awaiting approval', merge: 'Awaiting merge', failed: 'Failed start' } as Record<string, string>)[reqs[0]!.kind] : `${reqs.length} open requests`}
    </div>}
    <div class="card-foot">
      <span class="stage-word">{sp.phase}</span>
      {t ? <span class="muted truncate">{t.area || teamName(t)}</span> : <span class="faint">No team</span>}
      <span class="grow" />
      {pr.total > 0 && <span class="faint num">{pr.done}/{pr.total}</span>}
    </div>
  </a>;
}
function useBoardFilter(s: State, project: Project) {
  // The search and the epic are in the address, so Back from a sprint returns to them.
  const [qv, setQ] = useQuery('q'); const [epic, setEpic] = useQuery('epic'); const q = qv || '';
  const all = sprintsOf(s, project.id);
  const items = all.filter((x) => (!epic || x.epic === epic) && (!q || `${x.label} ${x.name} ${x.goal}`.toLowerCase().includes(q.toLowerCase())));
  return { q, setQ, epic, setEpic, items, all };
}
// Hovering a card shows the sprint's TL;DR under it. The note is fixed to the window, so the
// scrolling lanes do not clip it, and it goes away when the lanes scroll.
const canHover = typeof matchMedia === 'function' && matchMedia('(hover: hover)').matches;
interface TipState { sp: Sprint; left: number; width: number; top?: number; bottom?: number }
function useCardTip(): CardTip {
  const [tip, setTip] = useState<TipState | null>(null); const timer = useRef<ReturnType<typeof setTimeout> | number>(0);
  const hide = () => { clearTimeout(timer.current); setTip(null); };
  const show = (sp: Sprint, el: HTMLElement, now?: boolean) => { if (!canHover || !sp.goal) return; clearTimeout(timer.current);
    timer.current = setTimeout(() => { const r = el.getBoundingClientRect(); const below = r.bottom + 140 < innerHeight;
      const box = (el.closest('.board-lanes') || document.body).getBoundingClientRect();
      const left = Math.max(r.left, box.left + 8), right = Math.min(r.right, box.right - 8); if (right - left < 120) return;
      setTip({ sp, left, width: right - left, ...(below ? { top: r.bottom + 6 } : { bottom: innerHeight - r.top + 6 }) }); }, now ? 0 : 350); };
  const el = tip ? <div class="card-tldr" role="tooltip" style={{ left: tip.left + 'px', width: tip.width + 'px', top: tip.top != null ? tip.top + 'px' : 'auto', bottom: tip.bottom != null ? tip.bottom + 'px' : 'auto' }}>
    <b>TL;DR:</b>
    {' '}
    {tip.sp.goal}
  </div> : null;
  return { show, hide, el };
}
// A card drags to any lane. A lane whose needs the card has not met takes no card and says what
// it still needs. A forced move is made from the sprint's own page.
function BoardView({ s, items, inRepo }: { s: State; items: Sprint[]; inRepo?: boolean }) {
  const [col, setCol] = useState('In progress'); const tip = useCardTip();
  const lanes = useRef<HTMLDivElement>(null); const [edge, setEdge] = useState({ l: false, r: false });
  const [drag, setDrag] = useState<Id | null>(null); const [over, setOver] = useState<{ c: string; before: Id | null } | null>(null);
  // The edges fade where lanes are cut off by the scroll.
  const readEdge = () => { const el = lanes.current; if (!el) return; const l = el.scrollLeft > 2, r = el.scrollLeft + el.clientWidth < el.scrollWidth - 2; setEdge((e) => (e.l === l && e.r === r ? e : { l, r })); };
  useEffect(() => { readEdge(); const el = lanes.current; if (!el || typeof ResizeObserver === 'undefined') return undefined; const ro = new ResizeObserver(readEdge); ro.observe(el); return () => ro.disconnect(); }, []);
  const sorted = (c: string) => items.filter((x) => x.column === c).sort((a, b) => (a.rank ?? 1e9) - (b.rank ?? 1e9) || PRIORITIES.indexOf(a.priority) - PRIORITIES.indexOf(b.priority) || requestsOfSprint(s, b.id).length - requestsOfSprint(s, a.id).length);
  const canDrag = !live.on; const dragged = drag && byId(s.sprints, drag);
  const lacks = (c: string): LaneNeed[] => (dragged && dragged.column !== c ? moveNeeds(s, dragged, c).filter((n) => !n.met) : []);
  const onOver = (c: string) => (e: TargetedDragEvent<HTMLElement>) => {
    if (!drag) return;
    if (lacks(c).length) { setOver(null); return; }
    e.preventDefault(); e.dataTransfer!.dropEffect = 'move';
    const cards = [...e.currentTarget.querySelectorAll<HTMLElement>('[data-card]')].filter((x) => x.dataset.card !== drag);
    const before = cards.find((x) => { const r = x.getBoundingClientRect(); return e.clientY < r.top + r.height / 2; })?.dataset.card || null;
    setOver((o) => (o && o.c === c && o.before === before ? o : { c, before }));
  };
  const onDrop = (c: string) => (e: TargetedDragEvent<HTMLElement>) => {
    if (!drag || lacks(c).length) return;
    e.preventDefault(); const id = drag; const before = over?.c === c ? over.before : null;
    const order = sorted(c).map((x) => x.id).filter((x) => x !== id); order.splice(before ? order.indexOf(before) : order.length, 0, id);
    update((st) => { for (const sp of st.sprints) { const n = order.indexOf(sp.id); if (n >= 0) sp.rank = n; } });
    act.moveSprint(id, c);
    setDrag(null); setOver(null);
  };
  const dndOf = (sp: Sprint): CardDrag | false => canDrag && ({ can: true, cls: cx(drag === sp.id && 'dragging', over?.before === sp.id && 'drop-before'),
    start: (e: DragEvent) => { e.dataTransfer!.setData('text/plain', sp.label); e.dataTransfer!.effectAllowed = 'move'; tip.hide(); setDrag(sp.id); }, end: () => { setDrag(null); setOver(null); } });
  return <>
    <div class="col-jump only-phone-flex" role="tablist" aria-label="Board lanes">
      {COLUMNS.map((c) => <button type="button" key={c} class="tab" aria-selected={col === c} onClick={() => { setCol(c); document.getElementById('lane-' + c.replace(' ', '-'))?.scrollIntoView({ behavior: 'smooth', inline: 'start', block: 'nearest' }); }}>
      {c}
      <span class="count">{items.filter((x) => x.column === c).length}</span>
    </button>)}
    </div>
    <div class={cx('board-scroll', edge.l && 'more-left', edge.r && 'more-right')}>
      <div ref={lanes} class={cx('board-lanes', drag && 'dragging')} data-main onScroll={readEdge} onScrollCapture={tip.hide}>
        {COLUMNS.map((c) => { const list = sorted(c); const need = lacks(c);
        return <section class={cx('board-lane', need.length > 0 && 'locked', over?.c === c && !over.before && 'drop-end')} key={c} id={'lane-' + c.replace(' ', '-')} aria-label={c} onDragOver={onOver(c)} onDrop={onDrop(c)}>
          <header><h2>{c}</h2><span class="count">{list.length}</span></header>
          {need.length > 0 && <p class="lane-why"><Icon name="lock" size={12} /><span>Needs {need.map((n) => n.short).join(', ')}</span></p>}
          <div class="cards">{list.map((sp) => <SprintCard key={sp.id} s={s} sp={sp} hover={drag ? null : tip} dnd={dndOf(sp)} inRepo={inRepo} />)}</div>
        </section>; })}
      </div>
    </div>
    {tip.el}
  </>;
}
interface TCol { id: string; label: string; key: (s: State, x: Sprint) => string | number; num?: boolean }
const TCOLS: TCol[] = [
  { id: 'title', label: 'Title', key: (s, x) => x.label },
  { id: 'epic', label: 'Epic', key: (s, x) => byId(s.epics, x.epic)?.name || '' },
  { id: 'priority', label: 'Priority', key: (s, x) => PRIORITIES.indexOf(x.priority) },
  { id: 'team', label: 'Team', key: (s, x) => (x.team ? teamName(teamOf(s, x.team)) : '~') },
  { id: 'stage', label: 'Stage', key: (s, x) => STAGES.indexOf(x.phase) },
  { id: 'status', label: 'Status', key: (s, x) => COLUMNS.indexOf(x.column) },
  { id: 'progress', label: 'Tasks', key: (s, x) => sprintProgress(s, x).done, num: true },
];
const PCOL: TCol = { id: 'epic', label: 'Project', key: (s, x) => byId(s.projects, x.project)?.name || '' };
function TableView({ s, items, inRepo }: { s: State; items: Sprint[]; inRepo?: boolean }) {
  const [sort, setSort] = useState({ col: 'status', dir: 'asc' });
  // A repository's board names the project of each sprint where a project's board names its epic.
  const cols = inRepo ? TCOLS.map((x) => (x.id === 'epic' ? PCOL : x)) : TCOLS;
  const c = cols.find((x) => x.id === sort.col)!;
  const rows = [...items].sort((a, b) => { const x = c.key(s, a), y = c.key(s, b); return (x < y ? -1 : x > y ? 1 : 0) * (sort.dir === 'asc' ? 1 : -1); });
  return <div class="view-scroll" data-main>
    <table class="table board-table">
      <thead>
        <tr>
          {cols.map((col) => <th key={col.id} class={col.num ? 'num' : undefined} aria-sort={sort.col === col.id ? (sort.dir === 'asc' ? 'ascending' : 'descending') : undefined}>
      <button type="button" class="th-sort" onClick={() => setSort(sort.col === col.id ? { col: col.id, dir: sort.dir === 'asc' ? 'desc' : 'asc' } : { col: col.id, dir: 'asc' })}>
        {col.label}
        {sort.col === col.id && <Icon name={sort.dir === 'asc' ? 'arrow-up' : 'arrow-down'} size={11} />}
      </button>
    </th>)}
        </tr>
      </thead>
      <tbody>
        {rows.map((x) => { const reqs = requestsOfSprint(s, x.id); const pr = sprintProgress(s, x); const t = x.team ? teamOf(s, x.team) : undefined;
      return <tr key={x.id} class={cx('click', reqs.length && (reqs.some((r) => r.kind === 'failed') ? 's-error' : 's-warn'))} onClick={() => go(`/p/${x.project}/sprint/${x.id}`)}>
        <td class="state-cell">
          <span class="tag">{x.label}</span>
          {' '}
          <a href={href(`/p/${x.project}/sprint/${x.id}`)} onClick={(e) => e.stopPropagation()} style="font-weight:var(--weight-medium)">
            {x.name}
          </a>
          {reqs.length > 0 && <> <Icon name="bell" size={12} class="warn-text" /></>}
        </td>
        <td class="muted">{inRepo ? <a href={href(`/p/${x.project}`)} onClick={(e) => e.stopPropagation()}>{byId(s.projects, x.project)?.name}</a> : byId(s.epics, x.epic)?.name || '—'}</td>
        <td><Priority p={x.priority || 'Medium'} label={true} /></td>
        <td class="muted">{t ? teamName(t) : '—'}</td>
        <td>{x.phase}</td>
        <td><span class={cx('status-word', 'st-' + x.column.replace(' ', '-').toLowerCase())}>{x.column}</span></td>
        <td class="num muted">{pr.total ? `${pr.done} of ${pr.total}` : '—'}</td>
      </tr>; })}
        {rows.length === 0 && <tr><td colspan={TCOLS.length} class="muted" data-empty>No sprints</td></tr>}
      </tbody>
    </table>
  </div>;
}
// The roadmap: one row for each epic, with its sprints under it, along a time line in days.
// A repository's roadmap has one row for each of its projects where a project's has one for each epic.
interface RoadmapGroup { id: Id; name: string; to?: string }
interface RoadmapRow { k: 'goal' | 'epic' | 'sprint'; id?: Id; e?: RoadmapGroup | null; n?: number; st?: number | null; en?: number | null; sp?: Sprint }
function RoadmapView({ s, project, items, repo }: { s: State; project?: Project; items: Sprint[]; repo?: Repo }) {
  const DAYPX = 96; const now = Date.now();
  const epics: RoadmapGroup[] = repo ? projectsOfRepo(s, repo.id).map((p) => ({ id: p.id, name: p.name, to: `/p/${p.id}` })) : epicsOf(s, project!.id);
  const keyOf = (sp: Sprint) => (repo ? sp.project : sp.epic);
  const span = items.length ? [Math.min(-1, ...items.map((x) => x.start ?? 0)) - 1, Math.max(7, ...items.map((x) => x.end ?? 1)) + 1] : [-2, 8];
  const from = Math.floor(span[0]), to = Math.ceil(span[1]); const days = Array.from({ length: to - from }, (_, i) => from + i);
  const x = (d: number) => (d - from) * DAYPX;
  const day0 = new Date(now); day0.setHours(0, 0, 0, 0);
  const label = (d: number) => { const t = new Date(day0.getTime() + d * DAY); return d === 0 ? 'Today' : t.toLocaleDateString('en-GB', { weekday: 'short', day: 'numeric' }); };
  const groups: { e: RoadmapGroup | null; list: Sprint[] }[] = [...epics.map((e) => ({ e, list: items.filter((sp) => keyOf(sp) === e.id) })), { e: null, list: items.filter((sp) => !keyOf(sp)) }].filter((g) => g.e || g.list.length);
  const todayX = x((now - day0.getTime()) / DAY);
  // Two dedicated columns: the names stay put on the left, and only the time line scrolls sideways under its own header.
  const rows: RoadmapRow[] = repo ? [] : [{ k: 'goal' }];
  for (const { e, list } of groups) {
    const st = list.length ? Math.min(...list.map((sp) => sp.start ?? 0)) : null; const en = list.length ? Math.max(...list.map((sp) => sp.end ?? 1)) : null;
    rows.push({ k: 'epic', id: e ? e.id : 'none', e, n: list.length, st, en });
    for (const sp of [...list].sort((a, b) => (a.start ?? 0) - (b.start ?? 0))) rows.push({ k: 'sprint', id: sp.id, sp });
  }
  const sprintHref = (sp: Sprint) => href(`/p/${sp.project}/sprint/${sp.id}`);
  const name = (r: RoadmapRow) => r.k === 'goal' ? <div class="rm-cell rm-goal" key="goal"><Icon name="target" size={13} class="faint" /><span class="truncate">{project?.goal}</span></div>
    : r.k === 'epic' ? <div class="rm-cell epic" key={r.id}>
      <Icon name="layers" size={13} class="faint" />
      {r.e?.to ? <a class="truncate" href={href(r.e.to)}><b>{r.e.name}</b></a> : <b class="truncate">{r.e ? r.e.name : 'No epic'}</b>}
      <span class="count">{r.n}</span>
    </div>
    : <div class="rm-cell" key={r.id}><span class="tag">{r.sp!.label}</span><a class="truncate" href={sprintHref(r.sp!)}>{r.sp!.name}</a></div>;
  const track = (r: RoadmapRow) => <div class={cx('rm-cell rm-track', r.k === 'epic' && 'epic')} key={r.k + (r.id || '')}>
    {r.k === 'epic' && !live.on && r.st != null && <span class="rm-bar epic" style={{ left: x(r.st) + 'px', width: (r.en! - r.st) * DAYPX + 'px' }} />}
    {r.k === 'sprint' && (!live.on || (r.sp!.start != null && r.sp!.end != null)) && <a class={cx('rm-bar', 'st-' + r.sp!.column.replace(' ', '-').toLowerCase())} href={sprintHref(r.sp!)} style={{ left: x(r.sp!.start ?? 0) + 'px', width: Math.max(0.3, (r.sp!.end ?? 1) - (r.sp!.start ?? 0)) * DAYPX + 'px' }} title={`${r.sp!.label} ${r.sp!.name} · ${r.sp!.column}`}>
      <span class="truncate">{r.sp!.name}</span>
    </a>}
  </div>;
  return <div class="roadmap" data-main>
    <div class="rm-grid" style={{ '--rm-w': (to - from) * DAYPX + 'px', '--day': DAYPX + 'px' }}>
      <div class="rm-names">
        <div class="rm-cell rm-head">{repo ? 'Project and sprint' : 'Epic and sprint'}</div>
        {rows.map(name)}
        {groups.length === 0 && <div class="rm-cell muted" data-empty>{repo ? 'No projects' : 'No epics'}</div>}
      </div>
      <div class="rm-time">
        <div class="rm-cell rm-head rm-axis">
          {days.map((d) => <span key={d} class={cx(d === 0 && 'today')} style={{ left: x(d) + 'px' }}>{label(d)}</span>)}
        </div>
        {rows.map(track)}
        {groups.length === 0 && <div class="rm-cell rm-track" />}
        <i class="rm-now" style={{ left: todayX + 'px' }} />
      </div>
    </div>
  </div>;
}
const projectsOfRepo = (s: State, rid: Id): Project[] => s.projects.filter((p) => p.repo === rid && !p.archived);
// The board of a repository: the sprints of all its projects on one board. A card names its
// project where a project's card names its epic, and the filter is by project.
export function RepoBoard({ s, repo, route }: { s: State; repo: Repo; route: Route }) {
  const view = ['table', 'roadmap'].includes(route.parts[3]!) ? route.parts[3] : 'board';
  const [qv, setQ] = useQuery('q'); const [pid, setPid] = useQuery('project'); const q = qv || ''; const focus = !!s.ui.focus;
  const ps = projectsOfRepo(s, repo.id); const all = s.sprints.filter((x) => ps.some((p) => p.id === x.project));
  const only = ps.find((p) => p.id === pid);
  const items = all.filter((x) => (!only || x.project === only.id) && (!q || `${x.label} ${x.name} ${x.goal}`.toLowerCase().includes(q.toLowerCase())));
  useKey((e) => { if (e.key === 'Escape' && store.state.ui.focus && !store.state.dialog) update((st) => { st.ui.focus = false; }); }, []);
  const views = [['board', 'Board', 'kanban'], ['table', 'Table', 'table-2'], ['roadmap', 'Roadmap', 'calendar']];
  return <div class="board-page">
    <div class="board-bar">
      <nav class="view-tabs" aria-label="Board views">
        {views.map(([id, label, icon]) => <a key={id} class="view-tab" aria-current={view === id ? 'page' : undefined} href={href(`/r/${repo.id}/board${id === 'board' ? '' : '/' + id}`)}>
        <Icon name={icon!} size={14} />
        {label}
      </a>)}
      </nav>
      <div class="search">
        <Icon name="search" size={14} />
        <input class="input" placeholder="Filter" aria-label="Filter sprints" value={q} onInput={(e) => setQ(e.currentTarget.value)} />
      </div>
      <Menu class={cx('btn filter-btn', only && 'set')} label="Project" width={240} trigger={<><span class="muted">Project</span><b>{only ? only.name : 'All'}</b><Icon name="chevron-down" size={12} /></>} items={[{ label: 'All', sub: String(all.length), checked: !only, onSelect: () => setPid(null) }, { sep: true }, ...ps.map((p) => ({ label: p.name, sub: String(all.filter((x) => x.project === p.id).length), checked: only?.id === p.id, onSelect: () => setPid(p.id) }))]} />
      <span class="grow" />
      <button type="button" class="btn icon" aria-pressed={focus} aria-label={focus ? 'Leave focus' : 'Focus on the board'} data-tip={focus ? 'Leave focus · Esc' : 'Focus'} data-tip-side="left" onClick={() => update((st) => { st.ui.focus = !st.ui.focus; })}>
        <Icon name={focus ? 'minimize-2' : 'maximize-2'} size={14} />
      </button>
    </div>
    {view === 'board' && <BoardView s={s} items={items} inRepo />}
    {view === 'table' && <TableView s={s} items={items} inRepo />}
    {view === 'roadmap' && <RoadmapView s={s} repo={repo} items={items} />}
  </div>;
}
export function Board({ s, project, route }: { s: State; project: Project; route: Route }) {
  const view = ['table', 'roadmap'].includes(route.parts[3]!) ? route.parts[3] : 'board';
  const f = useBoardFilter(s, project); const focus = !!s.ui.focus;
  const epics = epicsOf(s, project.id);
  useKey((e) => { if (e.key === 'Escape' && store.state.ui.focus && !store.state.dialog) update((st) => { st.ui.focus = false; }); }, []);
  if (project.archived) return <div class="page">
    <Empty title="This project is archived" icon="archive">
      <button type="button" class="btn sm" disabled={live.on && board.busy('project:' + project.id)} aria-busy={live.on && board.busy('project:' + project.id)} onClick={() => act.archiveProject(project.id, false)}>
        Restore
      </button>
    </Empty>
  </div>;
  const views = [['board', 'Board', 'kanban'], ['table', 'Table', 'table-2'], ['roadmap', 'Roadmap', 'calendar']];
  return <div class="board-page">
    <div class="board-bar">
      <nav class="view-tabs" aria-label="Board views">
        {views.map(([id, label, icon]) => <a key={id} class="view-tab" aria-current={view === id ? 'page' : undefined} href={href(`/p/${project.id}/board${id === 'board' ? '' : '/' + id}`)}>
        <Icon name={icon!} size={14} />
        {label}
      </a>)}
      </nav>
      <div class="search">
        <Icon name="search" size={14} />
        <input class="input" placeholder="Filter" aria-label="Filter sprints" value={f.q} onInput={(e) => f.setQ(e.currentTarget.value)} />
      </div>
      <Menu class={cx('btn filter-btn', f.epic && 'set')} label="Epic" width={240} trigger={<><span class="muted">Epic</span><b>{f.epic ? byId(s.epics, f.epic)!.name : 'All'}</b><Icon name="chevron-down" size={12} /></>} items={[{ label: 'All', sub: String(f.all.length), checked: !f.epic, onSelect: () => f.setEpic(null) }, { sep: true }, ...epics.map((e) => ({ label: e.name, sub: String(f.all.filter((x) => x.epic === e.id).length), checked: f.epic === e.id, onSelect: () => f.setEpic(e.id) }))]} />
      <span class="grow" />
      <StatusLine s={s} project={project} />
      <button type="button" class="btn icon" aria-pressed={focus} aria-label={focus ? 'Leave focus' : 'Focus on the board'} data-tip={focus ? 'Leave focus · Esc' : 'Focus'} data-tip-side="left" onClick={() => update((st) => { st.ui.focus = !st.ui.focus; })}>
        <Icon name={focus ? 'minimize-2' : 'maximize-2'} size={14} />
      </button>
    </div>
    {view === 'board' && <BoardView s={s} items={f.items} />}
    {view === 'table' && <TableView s={s} items={f.items} />}
    {view === 'roadmap' && <RoadmapView s={s} project={project} items={f.items} />}
  </div>;
}

// ---- tables ---------------------------------------------------------------------
const BLOCKED_BY: Record<string, string> = { approval: 'Blocked by an approval', question: 'Blocked by a question', failed: 'Blocked by a failed start' };
function TaskState({ t }: { t: Task }) {
  const tone = ({ done: 'success', blocked: 'warn', active: 'info', review: 'info', queue: '' } as Record<string, string>)[t.state];
  const word = t.state === 'blocked' ? BLOCKED_BY[t.blockedBy!] || 'Blocked' : TASK_STATES[t.state];
  return <span class={cx('badge', tone)}>{word}</span>;
}
export function TaskTable({ s, tasks, showSprint = false }: { s: State; tasks: Task[]; showSprint?: boolean }) {
  if (!tasks.length) return <Empty title="No tasks" />;
  return <table class="table">
    <thead>
      <tr><th>Task</th><th>Title</th><th>State</th><th>Owner</th>{showSprint && <th>Sprint</th>}<th class="num">Points</th></tr>
    </thead>
    <tbody>
      {tasks.map((t) => { const o = t.owner ? agentOf(s, t.owner) : undefined; const sp = byId(s.sprints, t.sprint);
      return <tr key={t.id} class={cx('click', t.state === 'blocked' && 's-warn')} onClick={() => go(taskPath(t))}>
        <td class="state-cell">
          <a class="mono" href={href(taskPath(t))} onClick={(e) => e.stopPropagation()}>{t.label}</a>
        </td>
        <td class="truncate" style="max-width:420px">{t.title}</td>
        <td><TaskState t={t} /></td>
        <td>
          {o ? <span class="row" style="gap:6px"><Avatar agent={o} s={s} size="sm" /><span>{o.name}</span></span> : <span class="faint">Unassigned</span>}
        </td>
        {showSprint && <td class="muted">{sp?.label}</td>}
        <td class="num muted">{t.points}</td>
      </tr>; })}
    </tbody>
  </table>;
}

// ---- sprint page ----------------------------------------------------------------
// A sprint page has two parts. The middle holds the title and one tab at a time: Requests,
// Tasks or Plan. The rail on the right holds everything about the sprint: the next lane with
// what it needs and the button that moves the sprint, the goal, the facts and the evidence.
// The plan is an HTML page, drawn in a frame. A narrow page has no room for a rail, so the
// rail's parts sit under the title there.
function TaskWord({ t }: { t: Task }) {
  const word = t.state === 'blocked' ? BLOCKED_BY[t.blockedBy!] || 'Blocked' : TASK_STATES[t.state];
  return <span class={cx('tk-state', t.state === 'blocked' && 'warn-text', t.state === 'done' && 'muted')}>
    {t.state === 'done' && <Icon name="check" size={12} />}
    {word}
  </span>;
}
const STATEMENT_WORD: Record<string, string> = { holds: 'Holds', building: 'Being built', review: 'In review', stuck: 'Stuck', open: 'Not started' };
export function SprintPage({ s, project, sprintId, tab, task }: { s: State; project: Project; sprintId: Id; tab?: string; task?: string }) {
  const sp = byId(s.sprints, sprintId); const [adding, setAdding] = useState(false); const [title, setTitle] = useState('');
  // The open tab is in the address, so a link can open the plan. A switch replaces the entry, so Back returns to the board.
  const setTab = (id: string) => swap(`/p/${project.id}/sprint/${sprintId}/${id}`);
  const frame = useRef<HTMLIFrameElement>(null); const pending = useRef<string | null>(null);
  if (!sp) return <div class="page">
    <Empty title="This sprint does not exist">
      <a class="link" href={href(`/p/${project.id}/board`)}>Back to the board</a>
    </Empty>
  </div>;
  const tasks = tasksOfSprint(s, sp.id); const reqs = requestsOfSprint(s, sp.id); const t = sp.team ? teamOf(s, sp.team) : undefined; const pr = sprintProgress(s, sp);
  const done = sp.column === 'Done'; const epic = epicsOf(s, project.id).find((e) => e.id === sp.epic);
  const adds = live.on && board.busy('task:' + sp.id);
  const add = async (e: Event) => { e.preventDefault(); if (!title.trim()) return;
    if (live.on) { const reply = await board.addTask(project, sp, title); if (reply) { setTitle(''); setAdding(false); } return; }
    update((st) => {
      st.tasks.push({ id: uid('t'), project: project.id, label: nextLabel(st, project), title: title.trim(), sprint: sp.id, team: sp.team, owner: null, state: 'queue', points: 2, files: [], history: [{ ts: Date.now(), who: 'owner', text: 'You added this task. The project lead was told.' }] }); });
    act.toast('Task added. The project lead was told.'); setTitle(''); setAdding(false); };
  // The tabs. Requests is there only while a request waits, and it opens first. Otherwise Tasks opens, or Plan when there is a plan and no task.
  const tabs = ([reqs.length > 0 && ['requests', 'Requests', reqs.length], ['tasks', 'Tasks', null], ['plan', 'Plan', null]] as (false | [string, string, number | null])[]).filter((x): x is [string, string, number | null] => !!x);
  const first = reqs.length ? 'requests' : tasks.length || !sp.plan ? 'tasks' : 'plan';
  const cur = tabs.some(([id]) => id === tab) ? tab : first;
  // An address with a tab the sprint does not have moves to the tab that opens.
  if (tab && tab !== cur) queueMicrotask(() => swap(`/p/${project.id}/sprint/${sprintId}/${cur}`));
  // The open task. It takes the pane of the Tasks tab, and the rail lists the sprint's tasks.
  const open = cur === 'tasks' && task ? tasks.find((k) => k.id === task) : undefined;
  const tabKey = (e: TargetedKeyboardEvent<HTMLElement>) => { const step = ({ ArrowRight: 1, ArrowLeft: -1 } as Record<string, number>)[e.key]; if (!step) return; e.preventDefault();
    const i = (tabs.findIndex(([id]) => id === cur) + step + tabs.length) % tabs.length; setTab(tabs[i]![0]); (e.currentTarget.children[i] as HTMLElement | undefined)?.focus(); };
  // The plan's frame takes the page's theme and left edge, and drops the heading that repeats the sprint's name.
  const jump = (id: string) => { const h = frame.current?.contentDocument?.getElementById(id); if (!h) return;
    h.scrollIntoView({ behavior: 'smooth', block: 'start' }); h.classList.remove('hit'); void h.offsetWidth; h.classList.add('hit'); };
  const onPlan = () => { const d = frame.current?.contentDocument; if (!d) return;
    d.documentElement.setAttribute('data-theme', document.documentElement.getAttribute('data-theme') || 'light');
    const h1 = d.querySelector('h1'); if (h1 && h1.textContent!.includes(sp.label)) h1.hidden = true;
    const art = d.querySelector<HTMLElement>('.plan'); if (art) { art.style.margin = '0'; art.style.padding = '4px 20px 48px'; }
    if (pending.current) { const id = pending.current; pending.current = null; setTimeout(() => jump(id), 60); } };
  // A task's plan link opens the plan at that part and marks it for a moment.
  const toSection = (task: Task) => { if (cur === 'plan') jump(task.section!); else { pending.current = task.section!; setTab('plan'); } };
  const deps = sp.dependsOn.map((id) => byId(s.sprints, id)).filter((d): d is Sprint => !!d);
  // A move whose needs are met happens at once. Any other move asks first, and names what it skips.
  const lacks = (lane: string) => moveNeeds(s, sp, lane).filter((n) => !n.met);
  // A move into another stage stops the team that works on the sprint. The owner then says how it stops.
  const stops = (lane: string) => (t && t.stage !== LANE_STAGE[lane] && agentsOfTeam(s, t.id).some((a) => a.state === 'working') ? t : null);
  const moveTo = (lane: string) => { const miss = lacks(lane); const team = stops(lane); const go = () => act.moveSprint(sp.id, lane, { forced: true });
    if (!miss.length && !team) { act.moveSprint(sp.id, lane); return; }
    if (team) act.confirm({ title: `Move ${sp.label} to ${lane}?`, body: `${sp.phase} is in progress. ${teamName(team)} gets a stop signal.`, lost: miss.map((n) => n.miss), action: 'Safe shutdown', danger: false, onConfirm: go, second: { action: 'Terminate', onConfirm: go } });
    else act.confirm({ title: `Move ${sp.label} to ${lane}?`, lost: miss.map((n) => n.miss), action: 'Move anyway', onConfirm: go }); };
  const next = !live.on && nextLane(sp); const needs = next ? moveNeeds(s, sp, next) : []; const ready = needs.every((n) => n.met);
  // One place moves a sprint: the button at the top of the rail. It names the lane the sprint is in, and its menu
  // lists every lane. A lane with unmet needs lists them, and choosing it asks first. The agents move a sprint
  // as the workflow says, so a move by hand is the exception and no button leads to it.
  const laneItems = () => COLUMNS.map((c) => { const miss = c === sp.column ? [] : lacks(c);
    return miss.length ? { label: c, why: `Needs ${miss.map((n) => n.short).join(', ')}`, onSelect: () => moveTo(c) } : { label: c, checked: c === sp.column, onSelect: () => moveTo(c) }; });
  const teamLink = () => (t ? <a class="link" href={href(`/p/${project.id}/team/${t.id}`)}>{teamName(t)}</a> : null);
  const depLinks = () => deps.map((d, n) => <>{n > 0 && ', '}<a class="link" href={href(`/p/${project.id}/sprint/${d.id}`)}>{d.label}</a></>);
  const none = <span class="muted">None</span>;
  const facts: [string, ComponentChildren][] = [...(live.on ? [['Status', sp.column] as [string, ComponentChildren]] : []), ['Phase', sp.phase], ['Priority', sp.priority || none], ['Epic', epic ? epic.name : none], ['Team', teamLink() || none], ['Depends on', deps.length ? depLinks() : none]];
  const nextFor = () => <span class="tk-gate-for">
    <span class="muted">Before {next}</span>
    <span class="muted num">{needs.filter((n) => n.met).length} of {needs.length}</span>
  </span>;
  const needList = () => needs.length > 0 && <ul class="tk-needs" aria-label={`Requirements for ${next}`}>
    {needs.map((n, i) => <li key={i} class={cx(n.met && 'met')}>
    <Icon name={n.met ? 'circle-check' : 'clock'} size={14} title={n.met ? 'Met' : 'Not met'} />
    <span title={n.sprint ? `${n.sprint.name}, now in ${n.sprint.column}` : ''}>
      {n.sprint && <><span class="mono">{n.sprint.label}</span> </>}
      {n.met ? n.text : n.lack}
    </span>
  </li>)}
  </ul>;
  const statusButton = () => <Menu class="btn tk-status" width={260} label={`Status: ${sp.column}. Move to another lane`} trigger={<><span>{sp.column}</span><Icon name="chevron-down" size={14} /></>} items={laneItems} />;
  // The statements of the end result that this sprint delivers, by their numbers.
  const result = s.workflow?.[project.id]?.result.statements || [];
  const delivers = (sp.statements || []).map((n) => ({ n, text: result[n - 1]?.text || '', state: result[n - 1]?.state || 'open' })).filter((x) => x.text);
  return <div class="tk">
    <div class="tk-main">
      <header class="tk-head">
        <div class="tk-titlerow">
          {/* Back returns to the view the owner came from. With none, the arrow opens the board. */}
          <Back path={`/p/${project.id}/sprint`} or={<a class="icon-btn" href={href(`/p/${project.id}/board`)} aria-label="Back to the board" data-tip="Board">
            <Icon name="arrow-left" size={16} />
          </a>} />
          <span class="tag">{sp.label}</span>
          <h1 class="tk-name">{sp.name}</h1>
          {sp.fix && <span class="badge info">Fix sprint</span>}
        </div>
        <div class="tk-narrow tk-statusline">
          {live.on && <span class="tk-now">{sp.column}</span>}
          <span>{sp.phase}</span>
          {sp.priority && <span>{sp.priority} priority</span>}
          {epic && <span>{epic.name}</span>}
          {teamLink()}
          {deps.length > 0 && <span>After {depLinks()}</span>}
        </div>
        <p class="tk-narrow tk-goal">{sp.goal}</p>
        {!live.on && <div class="tk-narrow tk-gate">{statusButton()}{needs.length > 0 && <div class="tk-gate-what">{nextFor()}{needList()}</div>}</div>}
      </header>
      <div class="tabrow tk-tabrow">
        <div class="tk-tabs" role="tablist" aria-label="Parts of the sprint" onKeyDown={tabKey}>
          {tabs.map(([id, label, n]) => <button key={id} type="button" role="tab" class="tab" aria-selected={cur === id} tabindex={cur === id ? 0 : -1} onClick={() => setTab(id)}>
          {label}
          {n != null && <span class={cx('count', id === 'requests' && 'warn')}>{n}</span>}
        </button>)}
        </div>
        <span class="grow" />
        {cur === 'tasks' && pr.total > 0 && <span class="muted num tk-tabmeta">{pr.done} of {pr.total} done</span>}
        {cur === 'tasks' && !open && !done && <button type="button" class="btn" aria-expanded={adding} onClick={() => setAdding(!adding)}>
          <Icon name="plus" size={13} />
          Add task
        </button>}
        {cur === 'plan' && sp.plan && <>
          <span class="mono muted truncate tk-tabmeta">{sp.plan.file}</span>
          <span class="badge" title={'Written by ' + sp.plan.by}>{sp.plan.state}</span>
        </>}
      </div>
      <div class={cx('tk-pane', cur === 'plan' && sp.plan && 'framed')} data-space-waive="empty-pane" role="tabpanel" aria-label={tabs.find(([id]) => id === cur)![1]}>
        {cur === 'requests' && <div class="tk-pad">
          <div class="stack tk-reqs">
            {reqs.map((r) => <RequestBlock key={r.id} s={s} request={r} openThread={() => act.openSidePanel(r.agent, r.id)} />)}
          </div>
        </div>}
        {open && <div class="tk-pad"><TaskDoc s={s} project={project} t={open} onClose={() => setTab('tasks')} /></div>}
        {cur === 'tasks' && !open && <div class="tk-pad tasks">
          {adding && <form class="row tk-add" onSubmit={add}>
            <input class="input" autofocus placeholder="What should be done?" value={title} onInput={(e) => setTitle(e.currentTarget.value)} aria-label="Task title" />
            <button type="submit" class="btn" disabled={!title.trim() || adds} aria-busy={adds}>{adds ? 'Adding…' : 'Add'}</button>
            <button type="button" class="btn ghost" onClick={() => setAdding(false)}>Cancel</button>
          </form>}
          <div class="tk-tablewrap" data-main>
            <table class="table tk-tasks">
              <thead>
                <tr>
                  <th class="tk-c-label">Task</th>
                  <th>Title</th>
                  <th class="tk-c-status">Status</th>
                  <th class="tk-c-owner">Owner</th>
                  <th class="num tk-wide tk-c-points">Points</th>
                  <th class="tk-c-sec" aria-label="Part of the plan" />
                </tr>
              </thead>
              <tbody>
                {tasks.length === 0 && <tr class="tk-empty"><td colspan={6} class="muted" data-empty>No tasks</td></tr>}
                {tasks.map((k) => { const o = k.owner ? agentOf(s, k.owner) : undefined; return <tr key={k.id} class="click" onClick={() => go(taskPath(k))}>
              <td class="mono tk-t-label">
                <a class="link" href={href(taskPath(k))} onClick={(e) => e.stopPropagation()}>{k.label}</a>
              </td>
              <td class="tk-t-title" title={k.title}>{k.title}</td>
              <td class="tk-t-status"><TaskWord t={k} /></td>
              <td class="tk-t-owner">
                {o ? <span class="row tk-owner" title={o.name}>
                <Avatar agent={o} s={s} size="sm" />
                <span class="tk-wide tk-oname">{o.name}</span>
              </span> : <span class="faint">
                <span class="tk-wide tk-oname">Unassigned</span>
                <span class="tk-tight" aria-label="Unassigned">–</span>
              </span>}
              </td>
              <td class="num muted tk-wide">{k.points}</td>
              <td class="tk-t-sec">
                {k.section && <button type="button" class="icon-btn" aria-label={`Show ${k.label} in the plan: ${k.sectionName}`} title={`In the plan: ${k.sectionName}`} onClick={(e) => { e.stopPropagation(); toSection(k); }}>
                <Icon name="file-text" size={14} />
              </button>}
              </td>
            </tr>; })}
              </tbody>
            </table>
          </div>
        </div>}
        {cur === 'plan' && (sp.plan ? <iframe ref={frame} class="tk-frame" src={sp.plan.src} title={'Plan for ' + sp.label} onLoad={onPlan} /> : <div class="tk-pad"><p class="tk-none muted" data-empty>No plan</p></div>)}
      </div>
    </div>
    {open ? <nav class="tk-side tk-list" aria-label="Tasks of this sprint">
      <ul>
        {tasks.map((k) => <li key={k.id}>
          <a class="qv-row" aria-current={k.id === open.id ? 'page' : undefined} href={href(taskPath(k))}>
            <span class="qv-t">{k.title}</span>
            <span class="qv-m"><span class="mono">{k.label}</span><span class={cx('truncate', k.state === 'blocked' && 'warn-text')}>{k.state === 'blocked' ? BLOCKED_BY[k.blockedBy!] || 'Blocked' : TASK_STATES[k.state]}</span></span>
          </a>
        </li>)}
      </ul>
    </nav> : <aside class="tk-side" data-space-waive="empty-pane" aria-label="About this sprint">
      {!live.on && <div class="tk-next">{statusButton()}{needs.length > 0 && nextFor()}{needList()}</div>}
      <div class="tk-about"><span class="muted">Goal</span><p>{sp.goal}</p></div>
      {delivers.length > 0 && <div class="tk-about">
        <span class="muted">Statements</span>
        <ul class="tk-states" aria-label="Statements of the end result that this sprint delivers">
          {delivers.map((x) => <li key={x.n}>
            <a href={href(`/p/${project.id}/end-result`)}>
              <span class="num faint">{x.n}</span>
              <span>{x.text} <small class={x.state === 'holds' ? 'success-text' : x.state === 'review' ? 'info' : 'muted'}>{STATEMENT_WORD[x.state] || 'Not started'}</small></span>
            </a>
          </li>)}
        </ul>
      </div>}
      <dl class="tk-facts">{facts.map(([k, v]) => <div key={k}><dt>{k}</dt><dd>{v}</dd></div>)}</dl>
      {sp.evidence && <div class="tk-about">
        <span class="muted">Evidence</span>
        <ul class="tk-proof" aria-label="Evidence">
          {Object.values(sp.evidence).map((e) => <li><Icon name="circle-check" size={14} class="success-text" /><span>{e}</span></li>)}
        </ul>
      </div>}
    </aside>}
  </div>;
}

// ---- task page ------------------------------------------------------------------
/** One task in full. It is shown inside the Tasks tab of its sprint. */
function TaskDoc({ s, project, t, onClose }: { s: State; project: Project; t: Task; onClose: () => void }) {
  const now = useNow(); const taskId = t.id;
  useEffect(() => { if (live.on && t.worktreeId) void board.loadTask(project, t); }, [taskId, t.worktreeId, s.boardRevision]);
  const detail = live.on ? s.taskDetails?.[taskId] : t;
  const files: string[] = detail?.files || [], history: TaskEvent[] = detail?.history || [];
  const o = t.owner ? agentOf(s, t.owner) : undefined; const team = t.team ? teamOf(s, t.team) : undefined; const reqs = requestsOfTask(s, t.id);
  return <article class="tkd" aria-labelledby={'tkd-' + t.id}>
    <header class="tkd-head">
      <span class="tag">{t.label}</span>
      <h2 class="tkd-title" id={'tkd-' + t.id}>{t.title}</h2>
      {o && <a class="btn" href={href(routeForAgent(s, o))}>
        <Icon name="message-square" size={14} />
        {'Message '}
        {o.name}
      </a>}
      <button type="button" class="icon-btn" aria-label="Close the task" data-tip="All tasks" onClick={onClose}><Icon name="x" size={15} /></button>
    </header>
    <p class="status-line">
      <TaskState t={t} />
      <span>{t.points == null ? '' : t.points + ' points'}</span>
    </p>
    {reqs.length > 0 && <div class="stack">
      {reqs.map((r) => <RequestBlock key={r.id} s={s} request={r} openThread={() => act.openSidePanel(r.agent, r.id)} />)}
    </div>}
    <dl class="dl facts">
      <dt>Owner</dt>
      <dd>{o ? <AgentLine s={s} agent={o} /> : <span class="muted">Unassigned</span>}</dd>
      <dt>Team</dt>
      <dd>
        {team ? <a class="link" href={href(`/p/${project.id}/team/${team.id}`)}>{teamName(team)}</a> : <span class="muted">None</span>}
      </dd>
      <dt>Worktree</dt>
      <dd class="mono truncate-start" title={live.on ? detail?.branch || '' : team ? team.branch : project.branch}>
        {live.on ? detail?.branch || '' : team ? team.branch : project.branch}
      </dd>
      {o && o.activity && <><dt>Doing now</dt><dd>{o.activity}</dd></>}
    </dl>
    {live.on && detail?.folder && <p class="mono muted">{detail.folder}</p>}
    {live.on && board.busy('read-task:' + taskId) && <p class="muted" role="status">Loading task details…</p>}
    <div class="section-head" style="margin-top:var(--gap-group)">
      <h2>Changed files</h2>
      <span class="grow" />
      {files.length > 0 && <a class="link" href={href(`/p/${project.id}/git`)}>Open in Git</a>}
    </div>
    {files.length ? <ul class="file-list">
      {files.map((f) => <li key={f}><Icon name="file-code" size={14} class="faint" /><span class="mono truncate-start">{f}</span></li>)}
    </ul> : <p class="muted">No changed files</p>}
    <div class="section-head" style="margin-top:var(--gap-group)"><h2>Work and handoff</h2></div>
    {history.length ? <ol class="timeline">
      {history.map((h, i) => { const w = h.who === 'owner' ? null : agentOf(s, h.who);
      return <li key={i}>
        <time>{dayLabel(h.ts, now) === 'Today' ? clock(h.ts) : dayLabel(h.ts, now) + ' ' + clock(h.ts)}</time>
        <span><b>{w ? w.name : live.on ? '' : 'You'}</b> {h.text}</span>
      </li>; })}
    </ol>
      : <p class="muted">No activity</p>}
  </article>;
}
/** The old address of a task. A task that exists moves to its sprint, so only a missing task is shown here. */
export function TaskPage({ project }: { s: State; project: Project; taskId: Id }) {
  return <div class="page">
    <Empty title="This task does not exist">
      <a class="link" href={href(`/p/${project.id}/board`)}>Back to the board</a>
    </Empty>
  </div>;
}

// ---- team page ------------------------------------------------------------------
// What one agent is doing, beside its conversation: what it works on, what it changed, and its tasks.
// A team lead shows its team in place of the changes.
export function AgentPage({ s, project, agent: a }: { s: State; project: Project; agent: Agent }) {
  const team = a.team ? teamOf(s, a.team) as ReturnType<typeof teamOf> | undefined : undefined; const lead = a.role === 'team-lead';
  const task = a.task ? s.tasks.find((x) => x.label === a.task && x.project === a.project) : undefined;
  const sprint = task && byId(s.sprints, task.sprint);
  const tasks = lead && team ? tasksOfTeam(s, team.id) : s.tasks.filter((t) => t.owner === a.id);
  const members = lead && team ? agentsOfTeam(s, team.id).filter((x) => x.id !== a.id) : [];
  const ch = lead && team ? s.channels.find((c) => c.team === team.id) : undefined;
  // The conversation beside this page names the agent, its team and its state, and it holds the agent's
  // open request. The page does not say them again. A narrow window shows one of the two, so there the page names the agent.
  const alone = useMedia('(max-width: 1100px)');
  return <div class="page agp" data-main>
    {alone && <h2 class="agp-who"><Avatar agent={a} s={s} size="sm" /><span>{a.name}</span>{team && <span class="muted">{teamName(team)}</span>}</h2>}
    <p class="agp-facts muted">
      <span>{HARNESSES[a.harness]?.name || a.harness} · {a.model}{a.effort ? ' · ' + a.effort : ''}</span>
      <span class={cx('num', a.context >= 70 && 'warn-text')}>{a.context}% context</span>
      <span class="num">{plural(a.turns, 'turn')}</span>
      {team && <span class="mono">{team.branch}</span>}
    </p>
    <div class="section-head"><h2>Working on</h2></div>
    {task ? <a class="agp-now" href={href(taskPath(task))}>
      <span class="tag">{task.label}</span>
      <span class="what">
        <b>{task.title}</b>
        <small class="muted">{a.activity}</small>
      </span>
      {sprint && <span class="muted where">{sprint.label} {sprint.name}</span>}
      <Icon name="chevron-right" size={14} />
    </a> : <p class="muted agp-none" data-empty>{a.activity || 'No task'}</p>}
    {!lead && <AgentChanges s={s} agent={a} taskFiles={task ? task.files : []} project={project} />}
    {members.length > 0 && <>
      <div class="section-head">
        <h2>Team</h2>
        <span class="count">{members.length}</span>
        <span class="grow" />
        {ch && <a class="btn sm" href={href(`/p/${project.id}/message-board/${ch.id}`)}><Icon name="hash" size={14} />Team channel</a>}
      </div>
      <ul class="agp-team">
        {members.map((m) => { const ms = agentStatus(s, m); return <li key={m.id}>
          <a href={href(routeForAgent(s, m))}>
            <Avatar agent={m} s={s} size="sm" />
            <b>{m.name}</b>
            <span class={cx('truncate', ms.tone === 'warn' ? 'warn-text' : 'muted')}>{m.activity || ms.word}</span>
            <Icon name="chevron-right" size={14} />
          </a>
        </li>; })}
      </ul>
    </>}
    <div class="section-head"><h2>{lead ? 'Tasks of the team' : 'Tasks'}</h2><span class="count">{tasks.length}</span></div>
    <div class="table-wrap">
      {tasks.length ? <TaskTable s={s} tasks={tasks} /> : <p class="muted agp-none" data-empty>No tasks</p>}
    </div>
  </div>;
}

export function TeamPage({ s, project, teamId }: { s: State; project: Project; teamId: Id }) {
  const t = teamOf(s, teamId) as ReturnType<typeof teamOf> | undefined;
  const memberIds = agentsOfTeam(s, teamId).map((a) => a.id).join(',');
  useEffect(() => { if (live.on && memberIds) void board.loadAgents(memberIds.split(',')); }, [teamId, memberIds, s.boardRevision]);
  if (!t) return <div class="page">
    <Empty title="This team does not exist">
      <a class="link" href={href(`/p/${project.id}/board`)}>Back to the board</a>
    </Empty>
  </div>;
  const ags = agentsOfTeam(s, t.id); const tasks = tasksOfTeam(s, t.id); const ch = s.channels.find((c) => c.team === t.id);
  return <div class="page">
    <Back path={`/p/${project.id}/team`} class="page-back" />
    <div class="page-head">
      <h1 class="page-title">{teamName(t)}</h1>
      <span class="sub mono">{t.branch}</span>
      <span class="grow" />
      {ch && <a class="btn" href={href(`/p/${project.id}/message-board/${ch.id}`)}><Icon name="hash" size={14} />Team channel</a>}
      <a class="btn" href={href(`/p/${project.id}/git`)}><Icon name="git-branch" size={14} />Git</a>
    </div>
    <div class="section-head"><h2>Agents</h2><span class="count">{ags.length}</span></div>
    <div class="table-wrap">
      <table class="table">
        <thead>
          <tr><th>Role</th><th>Status</th><th>Doing now</th><th>Task</th><th>Harness and model</th><th class="num">Context</th></tr>
        </thead>
        <tbody>
          {ags.map((a) => { const st = agentStatus(s, a); const task = a.task ? s.tasks.find((x) => x.label === a.task && x.project === a.project) : undefined;
        return <tr key={a.id} class={cx('click', st.tone === 'warn' && 's-warn', st.tone === 'error' && 's-error')} onClick={() => go(routeForAgent(s, a))}>
          <td class="state-cell">
            <span class="row" style="gap:8px">
              <Avatar agent={a} s={s} size="sm" />
              <a href={href(routeForAgent(s, a))} style="font-weight:var(--weight-medium)" onClick={(e) => e.stopPropagation()}>{a.name}</a>
            </span>
          </td>
          <td><StateWord status={st} /></td>
          <td class="muted truncate" style="max-width:340px">{a.activity || '—'}</td>
          <td>
            {task ? <a class="mono" href={href(taskPath(task))} onClick={(e) => e.stopPropagation()}>{task.label}</a> : <span class="faint">—</span>}
          </td>
          <td class="muted">
            {HARNESSES[a.harness]?.name || a.harness}
            {' · '}
            {live.on ? s.boardAgents?.[a.id]?.models.find((m: { id: string; label?: string }) => m.id === a.model)?.label || a.model : a.model}
            {' · '}
            {live.on ? s.boardAgents?.[a.id]?.efforts.find((e: { id: string; label?: string }) => e.id === a.effort)?.label || a.effort : a.effort}
          </td>
          <td class={cx('num', a.context >= 70 ? 'warn-text' : 'muted')}>
            {live.on ? (s.boardAgents?.[a.id]?.context == null ? '' : s.boardAgents[a.id].context + '%') : a.context + '%'}
          </td>
        </tr>; })}
        </tbody>
      </table>
    </div>
    <div class="section-head" style="margin-top:var(--gap-group)"><h2>Tasks</h2><span class="count">{tasks.length}</span></div>
    <div class="table-wrap" data-main><TaskTable s={s} tasks={tasks} showSprint={true} /></div>
  </div>;
}

// ---- repository page ------------------------------------------------------------
export function RepoPage({ s, repoId }: { s: State; repoId: Id }) {
  const r = byId(s.repos, repoId);
  if (!r) return <div class="view-body">
    <div class="page"><Empty title="This repository is not known"><a class="link" href={href('/machines')}>Machines</a></Empty></div>
  </div>;
  const ps = s.projects.filter((p) => p.repo === r.id).sort((a, b) => (a.archived ? 1 : 0) - (b.archived ? 1 : 0));
  return <div class="view">
    <div class="tabrow" role="tablist">
      <a class="tab" role="tab" aria-selected="true" href={href('/r/' + r.id)}>
        Projects
        <span class="count">{ps.filter((p) => !p.archived).length}</span>
      </a>
      <a class="tab" role="tab" aria-selected="false" href={href(`/r/${r.id}/design`)}>Design</a>
      <div class="tab-end">
        <button type="button" class="btn sm primary" onClick={() => act.openDialog({ type: 'new-project', repo: r.id })}>
          <Icon name="plus" size={14} />
          New project
        </button>
      </div>
    </div>
    <div class="view-body">
      <div class="repo-facts">
        <dl class="dl">
          <dt>Remote</dt>
          <dd class="mono">{live.on ? r.remote : r.remote || <span class="muted" style="font-family:var(--font-body)">None</span>}</dd>
          <dt>Base branch</dt>
          <dd class="mono">{r.main}</dd>
          <dt>Folders</dt>
          <dd class="row" style="flex-wrap:wrap;gap:4px 14px">
            {Object.entries(r.folders).map(([m, f]) => <span key={m}>{machineOf(s, m).name} <span class="mono muted">{f}</span></span>)}
          </dd>
        </dl>
        <section class="repo-desc" aria-label="Product description">
          <h2>Product</h2>
          {r.description ? <>
            <p>{r.description}</p>
            {r.describedAt && <small class="faint">
            {'Updated '}
            {ago(r.describedAt)}
            {' ago by the project lead of '}
            {(s.projects.find((p) => p.lead === r.describedBy) || { name: undefined }).name}
          </small>}
          </>
            : live.on ? null : <p class="muted">Not written</p>}
        </section>
      </div>
      <div data-main><ProjectTable s={s} projects={ps} showRepo={false} /></div>
    </div>
  </div>;
}
