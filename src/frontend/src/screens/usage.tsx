// The Usage page: what each account has used against its limits, where the tokens went,
// and what a review of that use found. It is a draft on sample data, in the mock only.
import { Icon, cx, href, update, ago, useNow, HOUR, DAY } from '../lib';
import { HARNESSES, projectOf, repoOf, agentOf, routeForAgent, subLabel } from '../data';
import * as act from '../actions';
import type { ComponentChildren } from 'preact';
import type { Id, Ms, State, Subscription } from '../types';

/** One row: what one agent used in one stage of one project, in millions of tokens. */
export interface UsageRow {
  project: Id; stage: string; team: string | null; agent: string; sub: Id; model: string; agentId: Id | null;
  t: Record<Period, number>;
}
export interface UsageLimit { label: string; pct: number; resetsAt: Ms }
export interface UsageFinding {
  id: Id; project: Id; stage: string; who: string; model: string; tokens: number; title: string;
  cause: string; guardrail: string; target: string; scope: string; state?: string | null;
}
/** The Usage page's sample: rows, the limits only one harness reports, and the usage review. */
export interface UsageState { rows: UsageRow[]; limits: Record<Id, UsageLimit[]>; review: { ranAt: Ms; findings: UsageFinding[] } }
type Period = 'day' | 'week' | 'month';
type Level = 'project' | 'stage' | 'sub' | 'agent';

// Sample prices in dollars for a million tokens, input and output blended. They are invented.
const PRICE: Record<string, number> = { 'Fable 5.2': 42, 'Opus 5.5': 28, 'Sonnet 5.5': 6, 'Haiku 5.5': 1.6, 'GPT-6.1 Sol': 18, 'GPT-6 Luna': 3.5 };
const PERIODS: [string, string][] = [['day', 'Today'], ['week', 'This week'], ['month', '30 days']];
const BY: [string, string][] = [['project', 'Project'], ['stage', 'Stage'], ['sub', 'Account']];
const LEVELS: Record<string, Level[]> = { project: ['project', 'stage', 'agent'], stage: ['stage', 'project', 'agent'], sub: ['sub', 'project', 'agent'] };

export function seedUsage(state: State): void {
  const T = state.T;
  // One row is what one agent used in one stage of one project, in millions of tokens.
  const R = (project: Id, stage: string, team: string | null, agent: string, sub: Id, model: string, day: number, week: number, agentId: Id | null = null, month: number | null = null): UsageRow => ({ project, stage, team, agent, sub, model, agentId, t: { day, week, month: month ?? +(week * 2.6).toFixed(1) } });
  const max = 'sub-max', team = 'sub-team', cx_ = 'sub-codex';
  state.usage = {
    rows: [
      R('console', 'Define', null, 'Project lead', max, 'Opus 5.5', 0, 1.1, 'console-lead', 1.1),
      R('console', 'Design', 'Design team', 'Design lead', max, 'Sonnet 5.5', 0, 0, null, 2.4),
      R('console', 'Design', 'Design team', 'Designer', max, 'Sonnet 5.5', 0, 0, null, 3.8),
      R('console', 'Plan', null, 'Project lead', max, 'Opus 5.5', 0, 1.8, 'console-lead', 1.8),
      R('console', 'Plan', 'Planning team', 'Planning lead', max, 'Opus 5.5', 0, 3.2, null, 3.2),
      R('console', 'Plan', 'Planning team', 'Planner 1', max, 'Opus 5.5', 0, 9.6, null, 9.6),
      R('console', 'Plan', 'Planning team', 'Planner 2', cx_, 'GPT-6.1 Sol', 0, 4.1, null, 4.1),
      R('console', 'Plan', 'Planning team', 'Reviewer', max, 'Sonnet 5.5', 0, 2.3, null, 2.3),
      R('console', 'Build', null, 'Project lead', max, 'Opus 5.5', 1.4, 5.9, 'console-lead', 5.9),
      R('console', 'Build', 'Build team · Shell', 'Build lead', max, 'Opus 5.5', 0.9, 3.1, 'shell-lead', 3.1),
      R('console', 'Build', 'Build team · Shell', 'Builder 1', max, 'Sonnet 5.5', 0.8, 2.2, 'shell-b1', 2.2),
      R('console', 'Build', 'Build team · Shell', 'Builder 2', max, 'Sonnet 5.5', 1.0, 2.6, 'shell-b2', 2.6),
      R('console', 'Build', 'Build team · Shell', 'Test runner', max, 'Haiku 5.5', 0.1, 0.3, 'shell-test', 0.3),
      R('console', 'Build', 'Build team · Threads', 'Build lead', cx_, 'GPT-6.1 Sol', 0.7, 2.4, 'threads-lead', 2.4),
      R('console', 'Build', 'Build team · Threads', 'Builder 1', cx_, 'GPT-6.1 Sol', 0.9, 2.9, 'threads-b1', 2.9),
      R('console', 'Build', 'Build team · Threads', 'Test writer', cx_, 'GPT-6 Luna', 0.3, 0.8, 'threads-tw', 0.8),
      R('console', 'Review', 'Review team', 'Review lead', max, 'Opus 5.5', 0.4, 1.3, 'review-lead', 1.3),
      R('console', 'Review', 'Review team', 'Reviewer', max, 'Sonnet 5.5', 0.6, 1.9, 'review-r1', 1.9),
      R('rules', 'Define', null, 'Project lead', max, 'Opus 5.5', 0, 0.6, 'rules-lead', 0.6),
      R('rules', 'Plan', null, 'Project lead', max, 'Opus 5.5', 0.5, 2.0, 'rules-lead', 2.0),
      R('rules', 'Plan', 'Planning team', 'Planning lead', team, 'Opus 5.5', 0.8, 3.4, 'plan-lead', 3.4),
      R('rules', 'Plan', 'Planning team', 'Planner 1', team, 'Sonnet 5.5', 1.2, 5.8, 'plan-p1', 5.8),
      R('rules', 'Plan', 'Planning team', 'Planner 2', team, 'Sonnet 5.5', 1.0, 4.9, 'plan-p2', 4.9),
      R('rules', 'Plan', 'Planning team', 'Reviewer', team, 'Sonnet 5.5', 0.9, 4.4, 'plan-r1', 4.4),
      R('phone', 'Define', null, 'Project lead', max, 'Opus 5.5', 0.1, 0.4, 'phone-lead', 0.4),
      R('phone', 'Design', null, 'Project lead', max, 'Opus 5.5', 0.2, 0.6, 'phone-lead', 0.6),
      R('phone', 'Design', 'Design team', 'Design lead', max, 'Sonnet 5.5', 0.3, 0.9, 'design-lead', 0.9),
      R('phone', 'Design', 'Design team', 'Designer', max, 'Sonnet 5.5', 0.5, 1.6, 'design-d1', 1.6),
      R('naming', 'Define', null, 'Project lead', cx_, 'GPT-6.1 Sol', 0.3, 0.3, 'naming-lead', 0.3),
      R('apiref', 'Build', 'Build team', 'Builder 1', max, 'Sonnet 5.5', 0, 3.4),
      R('apiref', 'Build', 'Build team', 'Builder 2', max, 'Sonnet 5.5', 0, 2.8),
      R('apiref', 'Review', 'Review team', 'Review lead', max, 'Opus 5.5', 0.2, 1.1, 'apiref-rl'),
      R('apiref', 'Review', 'Review team', 'Reviewer', max, 'Sonnet 5.5', 0.3, 1.7, 'apiref-r1'),
      R('apiref', 'Hand over', null, 'Project lead', max, 'Opus 5.5', 0.2, 0.5, 'apiref-lead'),
      R('exports', 'Build', null, 'Project lead', team, 'Opus 5.5', 0.3, 1.2, 'exports-lead'),
      R('exports', 'Build', 'Build team', 'Build lead', team, 'Opus 5.5', 0.4, 1.8, 'exports-bl'),
      R('exports', 'Build', 'Build team', 'Builder 1', team, 'Sonnet 5.5', 0.6, 2.7, 'exports-b1'),
      R('exports', 'Build', 'Build team', 'Test runner', team, 'Haiku 5.5', 0.1, 0.4, 'exports-test'),
      R('docsrefresh', 'Define', null, 'Project lead', max, 'Opus 5.5', 0, 0.2, 'docsrefresh-lead'),
    ],
    // A limit that only one harness reports is listed here, beside the two every account has.
    limits: { 'sub-max': [{ label: 'This week · Opus', pct: 58, resetsAt: T + 3 * DAY }] },
    review: { ranAt: T - 2 * HOUR, findings: [
      { id: 'f1', project: 'rules', stage: 'Plan', who: 'Planning team', model: 'Sonnet 5.5', tokens: 11.2, title: 'Plan review ran three rounds on one statement',
        cause: 'Statement 7 drew a major finding in every round. Each round reread the whole plan and the code it cites.', guardrail: 'Ask the owner when one statement draws a major finding in two rounds.', target: 'Build loop · Plan review', scope: 'all' },
      { id: 'f2', project: 'console', stage: 'Plan', who: 'Planning team · Planner 1', model: 'Opus 5.5', tokens: 5.4, title: 'Planner 1 reread 212 files after each of four context resets',
        cause: 'The planner kept no notes outside its context, so every reset started from the repository again.', guardrail: 'A planner writes its code notes to a file and starts from that file after a reset.', target: 'Build loop · Planner instructions', scope: 'all' },
      { id: 'f3', project: 'console', stage: 'Build', who: 'Project lead', model: 'Opus 5.5', tokens: 2.1, title: 'The full suite ran nine times for one flaky test',
        cause: 'The reconnect test failed at random. Each failure started a full run of 7,804 tests.', guardrail: 'Rerun the reconnect test on its own first, and start a full run when it passes.', target: 'Build loop · Full test run', scope: 'repo' },
    ] },
  };
}

const fmtTok = (m: number): string => (m >= 100 ? Math.round(m) + 'M' : m >= 0.95 ? m.toFixed(1) + 'M' : m > 0 ? Math.round(m * 1000) + 'K' : '—');
const fmtUsd = (d: number): string => (d <= 0 ? '—' : d < 1 ? '<$1' : '$' + Math.round(d).toLocaleString('en-US'));
const cost = (r: UsageRow, period: Period): number => r.t[period] * (PRICE[r.model] || 0);
const sum = <T,>(rows: T[], f: (r: T) => number): number => rows.reduce((n, r) => n + f(r), 0);
const until = (ts: number, now: number): string => ago(2 * now - ts, now);

function Share({ part, whole }: { part: number; whole: number }) {
  const pct = whole ? Math.round((part / whole) * 100) : 0;
  return <span class="usg-share">
    <span class="progress" role="img" aria-label={pct + ' percent'}><i style={{ width: pct + '%' }} /></span>
    <span class="num">{pct}%</span>
  </span>;
}

// ---- accounts and their limits -----------------------------------------------------------
function Limits({ s, x, now }: { s: State; x: Subscription; now: number }) {
  if (!x.signedIn) return <span class="faint">Signed out</span>;
  const list = [x.window && { label: '5 hours', ...x.window }, x.usage && { label: 'This week', ...x.usage }, ...(s.usage.limits[x.id] || [])].filter((l): l is UsageLimit => !!l);
  if (!list.length) return <span class="faint">Not reported</span>;
  return <div class="usg-limits">
    {list.map((l) => <div key={l.label} class="usg-limit">
    <span>{l.label}</span>
    <span class="progress" role="img" aria-label={l.pct + ' percent used'}>
      <i class={l.pct >= 100 ? 'full' : l.pct >= 80 ? 'warn' : undefined} style={{ width: Math.min(100, l.pct) + '%' }} />
    </span>
    <span class={cx('num', l.pct >= 100 ? 'error-text' : l.pct >= 80 && 'warn-text')}>{l.pct}%</span>
    <span class="faint">Resets in {until(l.resetsAt, now)}</span>
  </div>)}
  </div>;
}
interface TableProps { s: State; rows: UsageRow[]; period: Period; total: number }
function Accounts({ s, rows, period, total }: TableProps) {
  const now = useNow(30_000);
  const harnesses = Object.entries(HARNESSES).filter(([hid]) => s.subscriptions.some((x) => x.harness === hid));
  return <div class="table-wrap">
    <table class="table usg-acc">
      <thead><tr><th>Account</th><th>Limits</th><th class="num">Tokens</th><th>Share</th><th class="num">API price</th></tr></thead>
      <tbody>
        {harnesses.map(([hid, h]) => { const subs = s.subscriptions.filter((x) => x.harness === hid); const mine = rows.filter((r) => subs.some((x) => x.id === r.sub));
      return [<tr key={hid} class="usg-group">
        <td colspan={2}><b>{h.name}</b></td>
        <td class="num">{fmtTok(sum(mine, (r) => r.t[period]))}</td>
        <td><Share part={sum(mine, (r) => r.t[period])} whole={total} /></td>
        <td class="num">{fmtUsd(sum(mine, (r) => cost(r, period)))}</td>
      </tr>,
        ...subs.map((x) => { const own = rows.filter((r) => r.sub === x.id); const tok = sum(own, (r) => r.t[period]);
          return <tr key={x.id}>
            <td class="wrap"><b class="usg-name">{x.name}</b>{x.account && <small class="muted">{x.account}</small>}</td>
            <td class="wrap"><Limits s={s} x={x} now={now} /></td>
            <td class="num">{fmtTok(tok)}</td>
            <td>{tok > 0 && <Share part={tok} whole={total} />}</td>
            <td class="num">{fmtUsd(sum(own, (r) => cost(r, period)))}</td>
          </tr>; })]; })}
      </tbody>
    </table>
  </div>;
}

// ---- breakdown ---------------------------------------------------------------------------
const KEY: Record<Level, (r: UsageRow) => string> = { project: (r) => r.project, stage: (r) => r.stage, sub: (r) => r.sub, agent: (r) => [r.team, r.agent, r.model].join('|') };
interface UsageNode { level: Level; key: string; id: string; rows: UsageRow[]; tokens: number; cost: number; children: UsageNode[] }
function nodes(rows: UsageRow[], levels: Level[], period: Period, path = ''): UsageNode[] {
  const [level, ...rest] = levels as [Level, ...Level[]]; const groups = new Map<string, UsageRow[]>();
  for (const r of rows) { const k = KEY[level](r); if (!groups.has(k)) groups.set(k, []); groups.get(k)!.push(r); }
  return [...groups].map(([k, list]) => ({ level, key: k, id: path + '/' + k, rows: list, tokens: sum(list, (r) => r.t[period]), cost: sum(list, (r) => cost(r, period)),
    children: rest.length ? nodes(list, rest, period, path + '/' + k) : [] })).sort((a, b) => b.tokens - a.tokens);
}
function Name({ s, n }: { s: State; n: UsageNode }) {
  if (n.level === 'project') { const p = projectOf(s, n.key) as ReturnType<typeof projectOf> | undefined; return <><b>{p?.name || n.key}</b>{p && <small class="faint mono">{repoOf(s, p).name}</small>}</>; }
  if (n.level === 'stage') return <b>{n.key}</b>;
  if (n.level === 'sub') return <b>{subLabel(s, n.key)}</b>;
  const r = n.rows[0]!; const a = r.agentId && agentOf(s, r.agentId);
  return <>
    {a ? <a class="link" href={href(routeForAgent(s, a))}>{r.agent}</a> : <span>{r.agent}</span>}
    {r.team && <small class="faint">{r.team}</small>}
  </>;
}
function Tree({ s, rows, by, period, total }: TableProps & { by: string }) {
  const top = nodes(rows, LEVELS[by]!, period);
  const open = (s.ui.usageOpen as Record<string, boolean> | null | undefined) || (top[0] ? { [top[0].id]: true } : {});
  const toggle = (id: string) => update((st) => { st.ui.usageOpen = { ...open, [id]: !open[id] }; });
  const out: ComponentChildren[] = [];
  const finds = by === 'sub' ? [] : s.usage.review.findings.filter((f) => !f.state);
  const walk = (list: UsageNode[], depth: number, whole: number, at: Partial<Record<Level, string>> = {}) => { for (const n of list) {
    const here: Partial<Record<Level, string>> = { ...at, [n.level]: n.key };
    const about = n.level === 'agent' ? [] : finds.filter((f) => (here.project ?? f.project) === f.project && (here.stage ?? f.stage) === f.stage);
    const flag = about.length > 0 && (depth === 1 || !open[n.id]);
    out.push(<tr key={n.id} class={cx(depth === 0 && 'usg-top', n.children.length > 0 && 'click')} onClick={n.children.length ? () => toggle(n.id) : undefined}>
      <td>
        <span class="usg-node" style={{ paddingLeft: depth * 22 + 'px' }}>
          {n.children.length > 0
        ? <button type="button" class={cx('svc-twist', open[n.id] && 'open')} aria-expanded={!!open[n.id]} aria-label={open[n.id] ? 'Close' : 'Open'} onClick={(e) => { e.stopPropagation(); toggle(n.id); }}>
          <Icon name="chevron-right" size={13} />
        </button>
        : <span class="usg-leaf" />}
          <Name s={s} n={n} />
          {flag && <button type="button" class="usg-flag" onClick={(e) => { e.stopPropagation(); showFinding(about[0]!); }}>
          <Icon name="shield-alert" size={12} />
          {about.length === 1 ? '1 finding' : about.length + ' findings'}
        </button>}
        </span>
      </td>
      <td class="muted">{n.level === 'agent' ? n.rows[0]!.model : ''}</td>
      <td class="num">{fmtTok(n.tokens)}</td>
      <td><Share part={n.tokens} whole={whole} /></td>
      <td class="num">{fmtUsd(n.cost)}</td>
    </tr>);
    if (open[n.id]) walk(n.children, depth + 1, n.tokens, here);
  } };
  walk(top, 0, total);
  return <div class="table-wrap">
    <table class="table usg-tree">
      <thead>
        <tr>
          <th>{BY.find(([id]) => id === by)![1]}</th>
          <th>Model</th>
          <th class="num">Tokens</th>
          <th>Share of parent</th>
          <th class="num">API price</th>
        </tr>
      </thead>
      <tbody>{out}</tbody>
    </table>
  </div>;
}

// ---- what a review of the use found --------------------------------------------------------
const findId = (f: UsageFinding): string => 'usg-f-' + f.id;
const setFind = (id: Id, open: boolean) => update((st) => { st.ui.usageFinds = { ...(st.ui.usageFinds || {}), [id]: open }; });
const scrollTo = (id: string) => requestAnimationFrame(() => document.getElementById(id)?.scrollIntoView({ block: 'center', behavior: 'smooth' }));
const showFinding = (f: UsageFinding) => { setFind(f.id, true); scrollTo(findId(f)); };
// Opens the breakdown at the stage of the project the finding is about.
const showWhere = (f: UsageFinding) => { update((st) => { st.ui.usageBy = 'project'; st.ui.usagePeriod = 'week'; st.ui.usageOpen = { ['/' + f.project]: true, ['/' + f.project + '/' + f.stage]: true }; }); scrollTo('usg-where'); };

function Finding({ s, f, open, whole }: { s: State; f: UsageFinding; open: boolean; whole: number }) {
  const p = projectOf(s, f.project) as ReturnType<typeof projectOf> | undefined; const repo = p ? repoOf(s, p).name : 'This repository';
  const set = (state: string | null, note?: string) => { update(() => { f.state = state; }); if (note) act.toast(note); };
  return [<tr key={f.id} id={findId(f)} class={cx('click usg-find', open && 'open')} onClick={() => setFind(f.id, !open)}>
    <td class="wrap">
      <span class="usg-node">
        <button type="button" class={cx('svc-twist', open && 'open')} aria-expanded={open} aria-label={open ? 'Close' : 'Open'} onClick={(e) => { e.stopPropagation(); setFind(f.id, !open); }}>
          <Icon name="chevron-right" size={13} />
        </button>
        <span class="usg-find-what"><b>{f.title}</b><small class="muted">{[p?.name, f.stage, f.who].join(' · ')}</small></span>
        {f.state && <span class={cx('badge', f.state === 'added' ? 'success' : 'outline')}>
          {f.state === 'added' ? 'Guardrail added' : 'Dismissed'}
        </span>}
      </span>
    </td>
    <td class="num">{fmtTok(f.tokens)}</td>
    <td><Share part={f.tokens} whole={whole} /></td>
    <td class="num">{fmtUsd(f.tokens * PRICE[f.model]!)}</td>
  </tr>,
    open && <tr key={f.id + '-body'} class="usg-find-open">
      <td colspan={4} class="wrap">
        <div class="usg-find-body">
          <div class="usg-cause">
            <h3>Cause</h3>
            <p>{f.cause}</p>
            <button type="button" class="btn sm" onClick={() => showWhere(f)}><Icon name="arrow-up" size={12} />Show breakdown</button>
          </div>
          <div class={cx('usg-rail', f.state)}>
            <h3>Guardrail</h3>
            <p>{f.guardrail}</p>
            <div class="usg-rail-for">
              <small class="muted">{f.target}</small>
              {f.state
          ? <small class="muted">{f.scope === 'all' ? 'Every repository' : repo}</small>
          : <div class="seg">
            {[['repo', repo], ['all', 'Every repository']].map(([id, label]) => <button type="button" key={id} aria-pressed={f.scope === id} onClick={() => update(() => { f.scope = id; })}>{label}</button>)}
          </div>}
            </div>
            <div class="row">
              {f.state
          ? <button type="button" class="btn sm" onClick={() => set(null)}>Reopen</button>
          : <>
            <button type="button" class="btn sm primary" onClick={() => set('added', 'Guardrail added')}>
              <Icon name="shield-check" size={13} />
              Add guardrail
            </button>
            <button type="button" class="btn sm ghost" onClick={() => set('dismissed', 'Finding dismissed')}>Dismiss</button>
          </>}
            </div>
          </div>
        </div>
      </td>
    </tr>];
}
function Findings({ s, whole }: { s: State; whole: number }) {
  const all = s.usage.review.findings; const waiting = all.filter((f) => !f.state); const handled = all.filter((f) => f.state);
  const opened = (s.ui.usageFinds || {}) as Record<Id, boolean>; const showHandled = !!s.ui.usageHandled;
  const row = (f: UsageFinding, i: number) => <Finding key={f.id} s={s} f={f} whole={whole} open={opened[f.id] ?? (!f.state && i === 0)} />;
  return <div class="table-wrap">
    <table class="table usg-finds">
      <thead><tr><th>Finding</th><th class="num">Tokens</th><th>Weekly share</th><th class="num">API price</th></tr></thead>
      <tbody>
        {waiting.map(row)}
        {handled.length > 0 && <tr class="click usg-handled" onClick={() => update((st) => { st.ui.usageHandled = !showHandled; })}>
        <td colspan={4}>
          <span class="usg-node">
            <button type="button" class={cx('svc-twist', showHandled && 'open')} aria-expanded={showHandled} aria-label={showHandled ? 'Close' : 'Open'} onClick={(e) => { e.stopPropagation(); update((st) => { st.ui.usageHandled = !showHandled; }); }}>
              <Icon name="chevron-right" size={13} />
            </button>
            <span>Handled</span>
            <span class="faint num">{handled.length}</span>
          </span>
        </td>
      </tr>}
        {showHandled && handled.map(row)}
      </tbody>
    </table>
  </div>;
}

export function Usage({ s }: { s: State }) {
  const now = useNow(60_000);
  const period = (s.ui.usagePeriod || 'week') as Period; const by = (s.ui.usageBy || 'project') as string;
  const rows = s.usage.rows.filter((r) => r.t[period] > 0);
  const total = sum(rows, (r) => r.t[period]); const dollars = sum(rows, (r) => cost(r, period));
  const seg = (list: [string, string][], value: string, key: string) => <div class="seg">
    {list.map(([id, label]) => <button type="button" key={id} aria-pressed={value === id} onClick={() => update((st) => { st.ui[key] = id; if (key === 'usageBy') st.ui.usageOpen = null; })}>
    {label}
  </button>)}
  </div>;
  return <div class="view-body">
    <div class="page usg">
      <div class="page-head"><h1 class="page-title">Usage</h1><span class="grow" />{seg(PERIODS, period, 'usagePeriod')}</div>
      <p class="usg-total"><b>{fmtTok(total)}</b> tokens<span class="faint">·</span><b>{fmtUsd(dollars)}</b> at API prices</p>
      <section><h2 class="usg-h">Accounts</h2><Accounts s={s} rows={rows} period={period} total={total} /></section>
      <section id="usg-where">
        <h2 class="usg-h">Breakdown<span class="grow" />{seg(BY, by, 'usageBy')}</h2>
        <Tree s={s} rows={rows} by={by} period={period} total={total} />
      </section>
      <section>
        <h2 class="usg-h">Findings<small class="faint">Usage review · {ago(s.usage.review.ranAt, now)} ago</small></h2>
        <Findings s={s} whole={sum(s.usage.rows, (r) => r.t.week)} />
      </section>
    </div>
  </div>;
}
