// The command center: the first view of a project, and the first view of a repository.
// It is made of widgets. A widget is one status the owner skims: what waits, who works, how far
// the work is, how much it costs. Files, Git and the terminal are views, not widgets.
//
// The widgets stand on a grid, and a menu at the end of the row of views hides and shows them.
// Every number that is not in the sample's records is made up from the project's id, and it
// stays the same between visits.
import { createContext, type ComponentChildren } from 'preact';
import { useState, useContext, useEffect } from 'preact/hooks';
import { Icon, cx, href, ago, plural, useNow, update, DAY } from '../lib';
import {
  PHASES, KIND_LABEL, byId, agentOf, agentLabel, agentsOfProject, answerableRequests, sprintsOf, tasksOfSprint, machineOf,
  projectStatus, routeForAgent, routeForRequest, subLabel, defineQuestions, leadOf,
} from '../data';
import * as act from '../actions';
import { Avatar, Menu } from '../ui';
import { workflowOf, agentGroups, lastFullRun } from './workflow';
import type { Agent, Id, Project, Repo, State } from '../types';
import { showCommit, type Worktree } from './git';

// ---- made-up numbers that stay the same --------------------------------------------
function rng(seed: string): () => number {
  let h = 2166136261;
  for (let i = 0; i < seed.length; i++) h = Math.imul(h ^ seed.charCodeAt(i), 16777619);
  return () => { h ^= h << 13; h ^= h >>> 17; h ^= h << 5; return ((h >>> 0) % 1000) / 1000; };
}
const series = (seed: string, n: number, lo: number, hi: number): number[] => { const r = rng(seed); return Array.from({ length: n }, () => Math.round(lo + r() * (hi - lo))); };
const sum = (xs: number[]) => xs.reduce((a, b) => a + b, 0);
const fmt = (n: number) => n.toLocaleString('en-US');
const tokens = (m: number) => (m >= 100 ? Math.round(m) + 'M' : m >= 1 ? m.toFixed(1) + 'M' : Math.round(m * 1000) + 'k');
const dayName = (ts: number) => new Date(ts).toLocaleDateString('en-US', { weekday: 'short', day: 'numeric', month: 'short' });
const lastDays = (now: number, n: number) => Array.from({ length: n }, (_, i) => dayName(now - (n - 1 - i) * DAY));
const lastWeeks = (now: number, n: number) => Array.from({ length: n }, (_, i) => 'Week of ' + new Date(now - (n - 1 - i) * 7 * DAY).toLocaleDateString('en-US', { day: 'numeric', month: 'short' }));

// ---- the parts of a chart -----------------------------------------------------------
/**
 * A row of thin bars on one baseline. Each bar names its value on hover. Bars before `from` belong
 * to the window the figure compares with, and they are quieter. The ends of the time axis are named.
 */
function Bars({ data, labels, unit, label, from = 0, last }: { data: number[]; labels: string[]; unit: string; label: string; from?: number; last: string }) {
  const max = Math.max(1, ...data); const [table, setTable] = useState(false);
  return <div class="cc-chart">
    {table ? <div class="cc-values" tabindex={0} role="region" aria-label={label}><table>
      <tbody>{data.map((v, i) => <tr key={i}><th scope="row">{labels[i]}</th><td class="num">{v}</td></tr>).reverse()}</tbody>
    </table></div> : <>
      <div class="cc-bars" role="img" aria-label={`${label}: ${data.map((v, i) => `${labels[i]} ${v}`).join(', ')}`}>
        {data.map((v, i) => <i key={i} class={cx(!v && 'zero', i < from && 'was')} style={{ height: v ? Math.max(6, v / max * 100) + '%' : undefined }} data-tip={`${labels[i]} · ${plural(v, unit)}`} data-tip-side="top" />)}
      </div>
      <div class="cc-axis" aria-hidden="true"><span>{labels[0]}</span><span>{last}</span></div>
    </>}
    <button type="button" class="cc-as link" aria-pressed={table} onClick={() => setTable(!table)}>{table ? 'Show the bars' : 'Show the numbers'}</button>
  </div>;
}
/** A small line of the last days, with a dot on the last one. It shows a shape; the figure beside it is a total. */
function Spark({ data, label }: { data: number[]; label: string }) {
  const w = 88; const h = 24; const max = Math.max(1, ...data); const min = Math.min(...data); const span = Math.max(1, max - min);
  const pts = data.map((v, i) => [3 + i * (w - 6) / Math.max(1, data.length - 1), h - 3 - (v - min) / span * (h - 6)] as const);
  const end = pts[pts.length - 1]!;
  return <svg class="cc-spark" width={w} height={h} viewBox={`0 0 ${w} ${h}`} role="img" aria-label={label}>
    <polyline points={pts.map((p) => p.join(',')).join(' ')} />
    <circle cx={end[0]} cy={end[1]} r="3" />
  </svg>;
}
/** One number with its name, and how it moved. */
function Figure({ name, value, unit, was, than = 'the week before' }: { name: string; value: ComponentChildren; unit?: string; was?: number | undefined; than?: string }) {
  return <div class="cc-fig">
    <span class="k">{name}</span>
    <span class="v"><b class="num">{value}</b>{unit && <span class="u">{unit}</span>}</span>
    {was != null && <span class="d"><Icon name={was > 0 ? 'arrow-up' : was < 0 ? 'arrow-down' : 'minus'} size={11} />{was === 0 ? 'No change' : `${Math.abs(was)} ${was > 0 ? 'more' : 'fewer'} than ${than}`}</span>}
  </div>;
}
/** Rows of one bar each, on one scale. */
function Shares({ rows, unit }: { rows: { name: string; to?: string; value: number; text: string }[]; unit: string }) {
  const max = Math.max(1, ...rows.map((r) => r.value));
  return <ul class="cc-shares" aria-label={unit}>
    {rows.map((r) => <li key={r.name}>
      {r.to ? <a class="truncate" href={href(r.to)}>{r.name}</a> : <span class="truncate">{r.name}</span>}
      <span class="track"><i style={{ width: Math.max(2, r.value / max * 100) + '%' }} /></span>
      <b class="num">{r.text}</b>
    </li>)}
  </ul>;
}

// ---- one widget ---------------------------------------------------------------------
/** What a widget knows about its place on the grid. The grid sets it; a widget only reads it. */
interface Slot { edit: boolean; wide: boolean | undefined; first: boolean; last: boolean; move: (by: number) => void; size: (wide: boolean) => void; hide: () => void }
const SlotOf = createContext<Slot | null>(null);
interface WidgetProps { title: string; n?: number | undefined; tone?: string | undefined; to?: string | undefined; end?: ComponentChildren; wide?: boolean; children?: ComponentChildren }
function Widget({ title, n, tone, to, end, wide, children }: WidgetProps) {
  const slot = useContext(SlotOf); const isWide = slot?.wide ?? !!wide; const edit = !!slot?.edit;
  return <section class={cx('cc-w', isWide && 'wide', tone === 'warn' && !!n && 'waits', edit && 'edit')} aria-label={title}>
    <header class="cc-head">
      <h2>{to && !edit ? <a href={href(to)}>{title}<Icon name="chevron-right" size={13} /></a> : title}</h2>
      {n != null && n > 0 && <span class={cx('count', tone)}>{n}</span>}
      {edit && slot ? <span class="cc-end cc-tools">
        <button type="button" class="icon-btn" disabled={slot.first} aria-label={`Move ${title} earlier`} data-tip="Earlier" onClick={() => slot.move(-1)}><Icon name="arrow-left" size={14} /></button>
        <button type="button" class="icon-btn" disabled={slot.last} aria-label={`Move ${title} later`} data-tip="Later" onClick={() => slot.move(1)}><Icon name="arrow-right" size={14} /></button>
        <button type="button" class="icon-btn" aria-label={isWide ? `Make ${title} one column wide` : `Make ${title} two columns wide`} data-tip={isWide ? 'One column' : 'Two columns'} onClick={() => slot.size(!isWide)}><Icon name={isWide ? 'minimize-2' : 'maximize-2'} size={14} /></button>
        <button type="button" class="icon-btn" aria-label={`Hide ${title}`} data-tip="Hide" onClick={slot.hide}><Icon name="eye-off" size={14} /></button>
      </span> : end && <span class="cc-end">{end}</span>}
    </header>
    {children}
  </section>;
}
const None = ({ children }: { children: ComponentChildren }) => <p class="cc-none muted" data-empty>{children}</p>;

// ---- what a project's widgets read ---------------------------------------------------
function facts(s: State, p: Project, now: number) {
  // The groups and the run are the ones the Status view and the Actions tab show, so the views agree.
  const agents = agentsOfProject(s, p.id); const groups = agentGroups(s, p);
  const lane = (l: string) => groups.find((g) => g.id === l)?.agents || [];
  const sps = sprintsOf(s, p.id); const done = sps.filter((x) => x.column === 'Done').length;
  const w = workflowOf(s, p); const run = lastFullRun(w); const bad = !!run && run.failed > 0 && !run.expected && !run.running; const cur = Math.max(0, PHASES.findIndex((x) => x.id === p.phase));
  // A project with no sprint has finished no task, so its chart is flat.
  const active = sps.length > 0;
  const daily = active ? series('tasks:' + p.id, 14, 0, 7) : Array(14).fill(0) as number[];
  const spend = series('tokens:' + p.id, 14, 4, 22).map((v) => (agents.length ? v : 0));
  void now;
  return { agents, lane, sps, done, w, run, bad, cur, daily, spend, reqs: answerableRequests(s, { project: p.id }) };
}
type Facts = ReturnType<typeof facts>;
interface PW { s: State; p: Project; f: Facts; now: number }

function NeedsYou({ s, p, f, now }: PW) {
  return <Widget title="Open requests" n={f.reqs.length} tone="warn" to={`/p/${p.id}/questions`} wide>
    {f.reqs.length === 0 ? <None>Nothing waits for you.</None> : <ul class="cc-rows">
      {f.reqs.slice(0, 6).map((r) => { const a = agentOf(s, r.agent);
        return <li key={r.id}>
          <a class="cc-row" href={href(routeForRequest(s, r))}>
            <Avatar agent={a} s={s} size="sm" status={false} />
            <span class="who truncate">{agentLabel(s, a, { withTeam: true })}</span>
            <span class="what truncate" title={r.title}>{r.title}</span>
            <span class="kind">{KIND_LABEL[r.kind] || r.kind}</span>
            <span class="age num">{ago(r.createdAt, now)}</span>
          </a>
        </li>; })}
    </ul>}
    {f.reqs.length > 6 && <a class="cc-more link" href={href(`/p/${p.id}/questions`)}>{f.reqs.length - 6} more</a>}
  </Widget>;
}
function Working({ s, p, f }: PW) {
  const list = f.lane('working'); const waiting = f.lane('waiting').length; const idle = f.lane('idle').length;
  const rest = [waiting && `${waiting} waiting`, idle && `${idle} idle`].filter(Boolean).join(' · ');
  return <Widget title="Active agents" n={list.length} to={`/p/${p.id}/status`} end={rest && <span class="muted">{rest}</span>} wide>
    {list.length === 0 ? <None>No active agents</None> : <ul class="cc-rows">
      {list.map((a: Agent) => <li key={a.id}>
        <a class="cc-row agent" href={href(routeForAgent(s, a))}>
          <Avatar agent={a} s={s} size="sm" status={false} />
          <span class="who truncate">{agentLabel(s, a, { withTeam: true })}</span>
          <span class="what truncate">{a.activity}</span>
          {a.task && <span class="tag">{a.task}</span>}
        </a>
      </li>)}
    </ul>}
  </Widget>;
}
/** Where the project is in the build loop. */
function Phase({ p, f, now }: PW) {
  const ph = PHASES[f.cur]!; const holds = f.w.result.statements.filter((x) => x.state === 'holds').length; const all = f.w.result.statements.length;
  return <Widget title="Progress" to={`/p/${p.id}/status`}>
    <ol class="cc-phase" aria-label={`Phase ${f.cur + 1} of ${PHASES.length}: ${ph.name}`}>
      {PHASES.map((x, i) => <li key={x.id} class={i < f.cur ? 'done' : i === f.cur ? 'now' : undefined} data-tip={x.name} data-tip-side="top" />)}
    </ol>
    <p class="cc-line"><b>{ph.name}</b><span class="muted">phase {f.cur + 1} of {PHASES.length}{p.createdAt ? ` · day ${Math.max(1, Math.ceil((now - p.createdAt) / DAY))}` : ''}</span></p>
    <dl class="cc-pairs">
      <dt>Sprints</dt><dd>{f.sps.length ? <><b class="num">{f.done}</b> of {f.sps.length} done</> : <span class="muted">None</span>}</dd>
      <dt>End result</dt><dd>{all ? <a class="link" href={href(`/p/${p.id}/end-result`)}><b class="num">{holds}</b> of {all} hold</a> : <span class="muted">Not written</span>}</dd>
    </dl>
  </Widget>;
}
function Sprints({ s, p, f }: PW) {
  const list = f.sps.filter((x) => x.column === 'In progress' || x.column === 'Review');
  return <Widget title="Active sprints" n={list.length} to={`/p/${p.id}/board`} wide>
    {list.length === 0 ? <None>No active sprints</None> : <ul class="cc-rows">
      {list.map((sp) => { const ts = tasksOfSprint(s, sp.id); const d = ts.filter((t) => t.state === 'done').length;
        return <li key={sp.id}>
          <a class="cc-row sprint" href={href(`/p/${p.id}/sprint/${sp.id}`)}>
            <span class="tag">{sp.label}</span>
            <span class="what truncate" title={sp.name}>{sp.name}</span>
            <span class="meter"><span class="progress"><i style={{ width: (ts.length ? d / ts.length * 100 : 0) + '%' }} /></span><span class="num">{d}/{ts.length}</span></span>
          </a>
        </li>; })}
    </ul>}
  </Widget>;
}
function Throughput({ f, now }: PW) {
  const week = sum(f.daily.slice(7)); const before = sum(f.daily.slice(0, 7));
  return <Widget title="Completed tasks">
    <Figure name="Last 7 days" value={week} was={week - before} />
    <Bars data={f.daily} labels={lastDays(now, 14)} from={7} last="Today" unit="task" label="Tasks completed on each of the last 14 days" />
  </Widget>;
}
function Checks({ p, f, now }: PW) {
  const run = f.run; const rv = f.w.reviews[0]; const round = rv?.rounds[rv.rounds.length - 1];
  const pts = round ? round.findings.reduce((n, x) => n + ({ minor: 1, major: 3, blocker: 9 } as Record<string, number>)[x.sev]!, 0) : 0;
  return <Widget title="Checks" to={`/p/${p.id}/actions`}>
    <ul class="cc-rows">
      <li><a class="cc-row check" href={href(`/p/${p.id}/actions`)}>
        <i class={cx('dot', !run ? 'idle' : f.bad ? 'failed' : 'working')} />
        <span class="who">{run ? run.kind : 'Full run'}</span>
        <span class={cx('what truncate', f.bad && 'error-text')}>{!run ? 'No run' : run.running ? 'Running' : run.failed ? `${plural(run.failed, 'failure')} of ${fmt(run.passed + run.failed)}` : `${fmt(run.passed)} passed`}</span>
        {run && <span class="age num">{ago(run.ts, now)} ago</span>}
      </a></li>
      <li><a class="cc-row check" href={href(`/p/${p.id}/status/review`)}>
        <i class={cx('dot', !rv ? 'idle' : pts ? 'waiting' : 'working')} />
        <span class="who">Review</span>
        <span class="what truncate">{rv ? `Round ${rv.rounds.length} of ${f.w.settings.maxRounds} · ${pts ? plural(pts, 'point') + ' open' : 'clean'}` : 'No review'}</span>
      </a></li>
    </ul>
  </Widget>;
}
function Spend({ s, f, now }: PW) {
  const total = sum(f.agents.map((a) => a.tokens)); const subs = [...new Set(f.agents.map((a) => a.subscription).filter((x): x is Id => !!x))].map((id) => byId(s.subscriptions, id)!).filter(Boolean);
  return <Widget title="Usage" to="/usage">
    <div class="cc-split">
      <Figure name="Tokens used" value={tokens(total)} />
      <Spark data={f.spend} label={`Tokens on each of the last 14 days, up to ${dayName(now)}`} />
    </div>
    <ul class="cc-limits" aria-label="Used of each subscription's limit">
      {subs.length > 0 && <li class="k">Limit used</li>}
      {subs.map((x) => <li key={x.id}>
        <span class="truncate">{subLabel(s, x.id)}</span>
        {x.usage ? <span class="meter"><span class="progress"><i class={x.usage.pct >= 85 ? 'warn' : undefined} style={{ width: x.usage.pct + '%' }} /></span><span class={cx('num', x.usage.pct >= 85 && 'warn-text')}>{x.usage.pct}%</span></span> : <span class="muted">Signed out</span>}
      </li>)}
    </ul>
  </Widget>;
}
function Recent({ s, p, now }: PW) {
  const commits = ((s.worktrees || []) as Worktree[]).filter((w) => w.project === p.id).flatMap((w) => w.commits.map((c) => ({ c, w }))).sort((a, b) => b.c.ts - a.c.ts)
    .filter((x, i, all) => all.findIndex((y) => y.c.subject === x.c.subject) === i).slice(0, 5);
  return <Widget title="Latest commits" to={`/p/${p.id}/git`} wide>
    {commits.length === 0 ? <None>No commits</None> : <ul class="cc-rows">
      {commits.map(({ c, w }) => <li key={c.hash}>
        <a class="cc-row commit" href={href(`/p/${p.id}/git`)} onClick={() => showCommit(s, p, c.hash)}>
          <Icon name={/^Merge/.test(c.subject) ? 'git-merge' : 'git-commit-horizontal'} size={14} class="faint" />
          <span class="what truncate">{c.subject}</span>
          <span class="kind mono truncate">{w.branch.split('/').pop()}</span>
          <span class="age num">{ago(c.ts, now)}</span>
        </a>
      </li>)}
    </ul>}
  </Widget>;
}

// ---- a project's command center --------------------------------------------------------
// The owner chose the grid on 2026-11-24, out of three layouts. The order below is the order
// on the grid. A wide widget takes two columns.
const PROJECT_WIDGETS: { id: string; name: string }[] = [
  { id: 'needs', name: 'Open requests' }, { id: 'phase', name: 'Progress' }, { id: 'working', name: 'Active agents' }, { id: 'checks', name: 'Checks' },
  { id: 'sprints', name: 'Active sprints' }, { id: 'tasks', name: 'Completed tasks' }, { id: 'usage', name: 'Usage' }, { id: 'recent', name: 'Latest commits' },
];
type Named = { id: string; name: string };
/** The four things the browser remembers about one command center. */
interface Keys { hidden: 'centerHidden' | 'repoCenterHidden'; order: 'centerOrder' | 'repoCenterOrder'; wide: 'centerWide' | 'repoCenterWide' }
const PROJECT_KEYS: Keys = { hidden: 'centerHidden', order: 'centerOrder', wide: 'centerWide' };
const REPO_KEYS: Keys = { hidden: 'repoCenterHidden', order: 'repoCenterOrder', wide: 'repoCenterWide' };
/** The owner's order, with any widget the owner has never placed at its usual place after it. */
const orderOf = (s: State, k: Keys, all: Named[]): string[] => { const saved = (s.prefs[k.order] || []).filter((id) => all.some((x) => x.id === id)); return [...saved, ...all.map((x) => x.id).filter((id) => !saved.includes(id))]; };
const setEdit = (on: boolean) => update((s) => { s.ui.ccEdit = on; });
/**
 * The menu of a command center: which widgets show, and the way into arranging them.
 * While the owner arranges, the menu gives way to one button that ends it.
 */
function WidgetMenu({ s, k, all }: { s: State; k: Keys; all: Named[] }) {
  const hidden = s.prefs[k.hidden] || []; const changed = hidden.length > 0 || (s.prefs[k.order] || []).length > 0 || Object.keys(s.prefs[k.wide] || {}).length > 0;
  if (s.ui.ccEdit) return <button type="button" class="btn sm primary cc-menu" onClick={() => setEdit(false)}><Icon name="check" size={14} />Done</button>;
  return <Menu class="btn sm cc-menu" align="end" width={230} trigger={<><Icon name="sliders-horizontal" size={14} /><span class="cc-menu-k" data-space-waive="clipped-value">Widgets</span><span class="count">{all.length - hidden.length}</span></>} items={[
    ...all.map((x) => ({ label: x.name, checked: !hidden.includes(x.id), onSelect: () => act.setPref(k.hidden, hidden.includes(x.id) ? hidden.filter((h) => h !== x.id) : [...hidden, x.id]) })),
    { sep: true },
    { label: 'Arrange widgets', icon: 'chevrons-up-down', disabled: hidden.length === all.length, onSelect: () => setEdit(true) },
    { label: 'Reset layout', icon: 'rotate-ccw', disabled: !changed, onSelect: () => { act.setPref(k.hidden, []); act.setPref(k.order, []); act.setPref(k.wide, {}); } },
  ]} />;
}
/** The menus stand at the end of the row of views, so the grid starts at the top of the pane. */
export const ProjectWidgets = ({ s }: { s: State }) => <WidgetMenu s={s} k={PROJECT_KEYS} all={PROJECT_WIDGETS} />;
export const RepoWidgets = ({ s }: { s: State }) => <WidgetMenu s={s} k={REPO_KEYS} all={REPO_WIDGETS} />;
/**
 * The grid of one command center. It puts the widgets in the owner's order, leaves out the hidden
 * ones, and tells each widget its place. Arranging ends when the owner leaves the command center.
 */
function Grid({ s, k, all, parts }: { s: State; k: Keys; all: Named[]; parts: Record<string, ComponentChildren> }) {
  useEffect(() => () => setEdit(false), []);
  const hidden = s.prefs[k.hidden] || []; const order = orderOf(s, k, all); const shown = order.filter((id) => !hidden.includes(id)); const wide = s.prefs[k.wide] || {};
  const move = (id: string, by: number) => { const at = shown.indexOf(id); const other = shown[at + by]; if (!other) return; const next = [...order]; const a = next.indexOf(id), b = next.indexOf(other); next[a] = other; next[b] = id; act.setPref(k.order, next); };
  return <div class={cx('cc', s.ui.ccEdit && 'arranging')} data-main>
    <div class="cc-grid">
      {shown.map((id, i) => <SlotOf.Provider key={id} value={{ edit: !!s.ui.ccEdit, wide: wide[id], first: i === 0, last: i === shown.length - 1, move: (by) => move(id, by), size: (w) => act.setPref(k.wide, { ...wide, [id]: w }), hide: () => act.setPref(k.hidden, [...hidden, id]) }}>{parts[id]}</SlotOf.Provider>)}
    </div>
    {shown.length === 0 && <p class="cc-none muted" data-empty>No widgets shown</p>}
  </div>;
}

export function ProjectCenter({ s, project: p }: { s: State; project: Project }) {
  const now = useNow(); const f = facts(s, p, now); const w: PW = { s, p, f, now };
  return <Grid s={s} k={PROJECT_KEYS} all={PROJECT_WIDGETS} parts={{
    needs: <NeedsYou {...w} />, phase: <Phase {...w} />, working: <Working {...w} />, checks: <Checks {...w} />,
    sprints: <Sprints {...w} />, tasks: <Throughput {...w} />, usage: <Spend {...w} />, recent: <Recent {...w} />,
  }} />;
}

// ---- what a repository's widgets read ---------------------------------------------------
function repoFacts(s: State, r: Repo, now: number) {
  const projects = s.projects.filter((p) => p.repo === r.id && !p.archived);
  const rows = projects.map((p) => { const f = facts(s, p, now); return { p, f, st: projectStatus(s, p) }; });
  const base = ((s.worktrees || []) as Worktree[]).find((w) => w.repo === r.id && w.kind === 'base');
  const weekly = projects.length ? series('merged:' + r.id, 12, 0, 5) : Array(12).fill(0) as number[];
  const runs = series('runs:' + r.id, 12, 3, 9); const failed = runs.map((v, i) => (i % 5 === 3 ? 1 : 0));
  const reqs = answerableRequests(s).filter((q) => projects.some((p) => p.id === q.project));
  // Open questions of Define wait for the owner too. The sidebar and the Projects table count them.
  const defs = projects.map((p) => ({ p, n: defineQuestions(s, p.id) })).filter((x) => x.n > 0);
  const agent = r.agent ? byId(s.agents, r.agent) : undefined;
  return { projects, rows, base, weekly, runs, failed, reqs, defs, agent };
}
type RepoFacts = ReturnType<typeof repoFacts>;
interface RW { s: State; r: Repo; f: RepoFacts; now: number }

function Projects({ s, r, f }: RW) {
  return <Widget title="Projects" n={f.projects.length} wide end={<button type="button" class="btn sm" onClick={() => act.openDialog({ type: 'new-project', repo: r.id })}><Icon name="plus" size={13} />New project</button>}>
    {f.rows.length === 0 ? <None>No projects</None> : <table class="cc-table">
      <thead><tr><th>Project</th><th>Phase</th><th>Sprints</th><th>Activity</th></tr></thead>
      <tbody>
        {f.rows.map(({ p, f: pf, st }) => <tr key={p.id}>
          <td><a class="truncate" href={href(`/p/${p.id}/center`)} title={p.name}>{p.name}</a></td>
          <td class="muted">{PHASES[pf.cur]!.name}</td>
          <td>{pf.sps.length ? <span class="meter"><span class="progress"><i style={{ width: pf.done / pf.sps.length * 100 + '%' }} /></span><span class="num">{pf.done}/{pf.sps.length}</span></span> : <span class="muted">None</span>}</td>
          <td><span class="cc-now"><i class={cx('dot', st.tone === 'warn' ? 'waiting' : pf.lane('working').length ? 'working' : 'idle')} /><span class={cx('truncate', st.tone === 'warn' && 'warn-text')}>{st.tone === 'warn' ? st.word : pf.lane('working').length ? `${pf.lane('working').length} working` : 'Idle'}</span></span></td>
        </tr>)}
      </tbody>
    </table>}
  </Widget>;
}
/** What the owner wants from the repository. The repository agent keeps it, and makes the projects. */
function Plan({ s, r }: RW) {
  const plan = r.plan || []; const open = plan.filter((g) => g.state === 'open').length;
  // A long plan shows its first eight goals, so the widgets after it stay near.
  const [all, setAll] = useState(false); const shown = all ? plan : plan.slice(0, 8);
  return <Widget title="Plan" wide end={open > 0 && <span class="muted">{plural(open, 'goal')} without a project</span>}>
    {plan.length === 0 ? <None>No goals</None> : <ul class="cc-rows">
      {shown.map((g) => { const p = g.project ? byId(s.projects, g.project) : undefined;
        return <li key={g.id}>
          <div class={cx('cc-row goal', g.state)}>
            <Icon name={g.state === 'done' ? 'circle-check' : g.state === 'project' ? 'circle-dot' : 'circle'} size={14} class={g.state === 'done' ? 'success-text' : 'faint'} />
            <span class="what truncate">{g.title}</span>
            {g.state === 'open' ? <span class="kind truncate">{g.note || 'No project'}</span> : p && <a class="kind link truncate" href={href(`/p/${p.id}/center`)}>{g.state === 'done' ? g.note || p.name : p.name}</a>}
          </div>
          {g.state === 'open' && <button type="button" class="btn sm" onClick={() => act.openDialog({ type: 'new-project', repo: r.id, goal: g.title })}>New project</button>}
        </li>; })}
    </ul>}
    {plan.length > 8 && <button type="button" class="cc-more link" aria-expanded={all} onClick={() => setAll(!all)}>{all ? 'Show the first 8' : `Show all ${plan.length} goals`}</button>}
  </Widget>;
}
function Merged({ f, now }: RW) {
  const month = sum(f.weekly.slice(8)); const before = sum(f.weekly.slice(4, 8)); const weeks = lastWeeks(now, 12);
  return <Widget title="Sprints merged">
    <Figure name="Last 4 weeks" value={month} was={month - before} than="the 4 weeks before" />
    <Bars data={f.weekly.slice(4)} labels={weeks.slice(4)} from={4} last="This week" unit="sprint" label="Sprints merged into the base branch in each of the last 8 weeks" />
  </Widget>;
}
function UsageBy({ r, f }: RW) {
  const rows = [...f.rows.map(({ p, f: pf }) => ({ name: p.name, to: `/p/${p.id}/center`, value: sum(pf.agents.map((a) => a.tokens)) })), ...(f.agent ? [{ name: 'Repository agent', to: `/r/${r.id}`, value: f.agent.tokens }] : [])]
    .sort((a, b) => b.value - a.value).map((x) => ({ ...x, text: tokens(x.value) }));
  return <Widget title="Usage by project" to="/usage">
    <Figure name="Tokens used" value={tokens(sum(rows.map((x) => x.value)))} />
    <Shares rows={rows} unit="Tokens of each project" />
  </Widget>;
}
function Base({ s, r, f, now }: RW) {
  const b = f.base; const c = b?.commits[0];
  return <Widget title="Base branch" to={`/r/${r.id}/git`}>
    <p class="cc-line"><b class="mono">{r.main}</b>{b && <span class="muted truncate">{machineOf(s, b.machine)?.name}</span>}</p>
    <dl class="cc-pairs">
      <dt>Last commit</dt><dd class="truncate" title={c?.subject}>{c ? <>{c.subject} <span class="muted">· {ago(c.ts, now)} ago</span></> : <span class="muted">None</span>}</dd>
      <dt>Full run</dt><dd><i class="dot working" /> Passed <span class="muted">· 7,796 tests</span></dd>
    </dl>
  </Widget>;
}
function RepoNeeds({ s, r, f, now }: RW) {
  const n = f.reqs.length + sum(f.defs.map((x) => x.n)); const rows = f.reqs.slice(0, 5); const defs = f.defs.slice(0, Math.max(1, 6 - rows.length));
  const more = f.reqs.length - rows.length + f.defs.length - defs.length;
  return <Widget title="Open requests" n={n} tone="warn" wide>
    {n === 0 ? <None>Nothing waits for you in {r.name}.</None> : <ul class="cc-rows">
      {rows.map((q) => <li key={q.id}>
        <a class="cc-row" href={href(routeForRequest(s, q))}>
          <Avatar agent={agentOf(s, q.agent)} s={s} size="sm" status={false} />
          <span class="who truncate">{byId(s.projects, q.project)?.name}</span>
          <span class="what truncate">{q.title}</span>
          <span class="kind">{KIND_LABEL[q.kind] || q.kind}</span>
          <span class="age num">{ago(q.createdAt, now)}</span>
        </a>
      </li>)}
      {defs.map(({ p, n: k }) => <li key={'define:' + p.id}>
        <a class="cc-row" href={href(`/p/${p.id}/end-result`)}>
          <Avatar agent={leadOf(s, p)} s={s} size="sm" status={false} />
          <span class="who truncate">{p.name}</span>
          <span class="what truncate">{plural(k, 'question')} about the end result</span>
          <span class="kind">Define</span>
        </a>
      </li>)}
    </ul>}
    {more > 0 && <a class="cc-more link" href={href('/home')}>{more} more</a>}
  </Widget>;
}
function Runs({ f, now }: RW) {
  const all = sum(f.runs); const bad = sum(f.failed);
  return <Widget title="Full runs">
    <Figure name="Passed, last 12 weeks" value={Math.round((all - bad) / Math.max(1, all) * 100)} unit="%" />
    <Bars data={f.runs} labels={lastWeeks(now, 12)} last="This week" unit="run" label="Full runs of the base branch in each of the last 12 weeks" />
  </Widget>;
}
/** Where the repository lives: its remote, and its folder on each machine. */
function Checkouts({ s, r }: RW) {
  return <Widget title="Checkouts" to="/machines" wide>
    <dl class="cc-pairs">
      <dt>Remote</dt><dd class="mono truncate">{r.remote || <span class="muted">None</span>}</dd>
      {Object.entries(r.folders).map(([m, folder]) => { const mc = machineOf(s, m);
        return [<dt key={m}>{mc?.name || m}</dt>, <dd key={m + 'f'}><span class="mono truncate">{folder}</span>{mc && mc.status !== 'online' && <span class="muted"> · {mc.status === 'offline' ? 'Offline' : 'Pairing'}</span>}</dd>]; })}
    </dl>
  </Widget>;
}

// The order below is the order on the grid: what to open, what waits, the plan, then the numbers.
const REPO_WIDGETS: { id: string; name: string }[] = [
  { id: 'projects', name: 'Projects' }, { id: 'needs', name: 'Open requests' }, { id: 'plan', name: 'Plan' }, { id: 'merged', name: 'Sprints merged' },
  { id: 'runs', name: 'Full runs' }, { id: 'usage', name: 'Usage by project' }, { id: 'base', name: 'Base branch' }, { id: 'about', name: 'Checkouts' },
];

export function RepoCenter({ s, repo: r }: { s: State; repo: Repo }) {
  const now = useNow(); const f = repoFacts(s, r, now); const w: RW = { s, r, f, now };
  return <Grid s={s} k={REPO_KEYS} all={REPO_WIDGETS} parts={{
    projects: <Projects {...w} />, needs: <RepoNeeds {...w} />, plan: <Plan {...w} />, merged: <Merged {...w} />,
    runs: <Runs {...w} />, usage: <UsageBy {...w} />, base: <Base {...w} />, about: <Checkouts {...w} />,
  }} />;
}
