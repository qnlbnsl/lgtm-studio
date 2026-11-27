import { setDraftNotice } from './drafts';
import { connectionWord } from './live/startup';
import { Workflows } from './screens/workflows';
import * as liveGit from './live/git';
import * as chat from './live/chat';
// The application shell: sidebar, top bar, the project tab row, the lead panel, and routing.
import { render, type ComponentChildren, type ComponentType } from 'preact';
import { useState, useEffect, useRef } from 'preact/hooks';
import {
  Icon, cx, clamp, store, update, useStore, useRoute, go, href, ago, useNow, useKey, useMedia, HOUR, plural, isMac, live, guardStore, engineWrite, refusal, refuse, swap, parseRoute,
} from './lib';
import {
  seed, emptyState, projectOf, repoOf, agentOf, teamOf, teamName, teamsOf, agentsOfTeam, leadOf, answerableRequests, agentStatus, projectStatus,
  KIND_LABEL, routeForRequest, routeForAgent, byId, agentLabel, machineOf, HARNESSES, questionsView, MAIN,
} from './data';
import * as act from './actions';
import * as engine from './engine';
import { Avatar, Menu, DialogHost, Toasts, Resizer, Thread, Composer, Tabs, Kbd, StateWord, canMessage, ReadOnlyBar, Empty, type TabItem } from './ui';
import './dialogs';
import { Home, AllProjects } from './screens/home';
import { Board, SprintPage, TaskPage, TeamPage, AgentPage, RepoPage } from './screens/project';
import { ChatTab } from './screens/chat';
import { QuestionsTab, questionsOf, waitingQuestions } from './screens/questions';
import { ProjectShell, StageMark, TalkToggle, TalkPick, showView, EndResultView } from './screens/mission';
import { seedDefine, homeTab } from './define';
import { GitTab, seedGit } from './screens/git';
import { FilesTab } from './screens/files';
import { TerminalTab, TerminalPanel, termPlace, seedTerminal } from './screens/terminal';
import { DesignTab, seedMocks } from './screens/mocks';
import { RepoDesign } from './screens/design';
import { RepoView } from './screens/repo';
import { ProjectCenter, ProjectWidgets } from './screens/center';
import { MessagesTab, AgentsTab, seedBoard } from './screens/messages';
import { WorkflowSettings, WorkflowTab, ActionsTab, StatusNav, actionsFailed, seedWorkflow } from './screens/workflow';
import { Machines, Harnesses, Notifications, ServiceSettings, SettingsShell, ProjectSettings, seedService, Saved } from './screens/service';
import { PreviewSettings } from './screens/browser';
import { Login, FirstRun, seedAuth } from './screens/auth';
import { Connect } from './screens/connect';
import * as server from './server/sync';
import { Usage, seedUsage } from './screens/usage';
import { emptyWorkflow } from './screens/workflow';
import { emptyMocks } from './screens/mocks';
import * as mocks from './live/mocks';
import type { Agent, Project, Route, State } from './types';
import type { Connection } from './engine';

// ---- named places ---------------------------------------------------------------
// A page that mounts the website can fill these places with its own components, without
// editing the website. Each component receives the store's state as `s` and the route.
// An empty place renders its fallback, which is the website's own content, or nothing.
//   sidebar.head        below the brand at the top of the sidebar
//   sidebar.account     above the owner's row at the foot of the sidebar
//   topbar.connection   the connection badge at the right of the top bar
/** A component a mounting page puts in one of the named places. */
export type SlotFill = ComponentType<{ s: State; route: Route }>;
let slots: Record<string, SlotFill> = {};
function Slot({ name, s, route, fallback = null }: { name: string; s: State; route: Route; fallback?: ComponentChildren }) {
  const Fill = slots[name];
  return Fill ? <Fill s={s} route={route} /> : fallback;
}

// ---- sidebar ------------------------------------------------------------------
interface TreeRowProps {
  kind: string; depth: number; open?: boolean; onToggle?: () => void; to: string; current: boolean; icon: ComponentChildren; label: string;
  state?: ComponentChildren; tone?: string | null; children?: ComponentChildren; title?: string; action?: ComponentChildren;
}
function TreeRow({ kind, depth, open, onToggle, to, current, icon, label, state, tone, children, title, action }: TreeRowProps) {
  // A single click on the row opens it. The arrow, or a double click on the row, folds it.
  return <div class={cx('tree-row', kind)} aria-current={current ? 'page' : undefined} style={{ paddingLeft: 4 + depth * 14 + 'px' }} onDblClick={onToggle ? (e) => { if (!(e.target as Element).closest('.twist, .hover-act')) { e.preventDefault(); onToggle(); } } : undefined}>
    {onToggle
      ? <button type="button" class="twist" aria-expanded={!!open} aria-label={(open ? 'Collapse ' : 'Expand ') + (title || label)} onClick={onToggle}>
        <Icon name="chevron-right" size={13} />
      </button>
      : <span class="twist none" />}
    <a href={href(to)} title={title || label}>{icon}<span class="label" data-space-waive="clipped-value">{label}</span></a>
    {state && <span class={cx('state', tone, !!action && 'hover-hide')}>{state}</span>}
    {action}
  </div>;
}
function ProjectTree({ s, route }: { s: State; route: Route }) {
  const path = route.path; const pinned = s.prefs.pinned || [];
  const only = s.prefs.onlyNeedingYou;
  const repos = s.repos.map((r) => ({ r, projects: s.projects.filter((p) => p.repo === r.id && !p.archived && (!only || answerableRequests(s, { project: p.id }).length)) })).filter((x) => !only || x.projects.length);
  if (!repos.length) return <div class="empty" data-empty style="padding:12px 8px">
    <strong>No open requests</strong>
  </div>;
  return repos.map(({ r, projects }) => {
    const rk = 'repo:' + r.id; const ropen = s.expanded[rk] ?? false;
    const rn = answerableRequests(s).filter((q) => projectOf(s, q.project).repo === r.id).length;
    return <div key={r.id}>
      <TreeRow kind="repo" depth={0} open={ropen} onToggle={() => act.toggleExpanded(rk)} to={'/r/' + r.id} current={path === '/r/' + r.id || path.startsWith(`/r/${r.id}/`)} icon={<Icon name="folder-git-2" size={14} class="faint" />} label={r.name} state={!Object.keys(r.folders).length ? <span class="faint">Not set up</span> : !ropen && rn ? rn : null} tone={Object.keys(r.folders).length ? 'warn' : null} action={Object.keys(r.folders).length > 0 && <button type="button" class="icon-btn sm hover-act" aria-label={'New project in ' + r.name} data-tip="New project here" data-tip-side="left" onClick={() => act.openDialog({ type: 'new-project', repo: r.id })}>
          <Icon name="plus" size={13} />
        </button>} />
      {ropen && <div class="tree-children" style="--guide:13px">
        {!live.on && r.agent && byId(s.agents, r.agent) && <TreeRow kind="agent" depth={1} to={'/r/' + r.id} current={false} icon={<Avatar agent={byId(s.agents, r.agent)!} s={s} size="sm" />} label="Repository agent" />}
        {projects.length === 0 && <div class="tree-row" style="padding-left:38px"><span class="faint">No projects</span></div>}
        {projects.map((p) => {
          const pk = 'project:' + p.id; const popen = s.expanded[pk] ?? false;
          const ps = projectStatus(s, p); const lead = leadOf(s, p); const ls = agentStatus(s, lead);
          const inProject = route.parts[0] === 'p' && route.parts[1] === p.id;
          const onLead = inProject && route.parts[2] === 'conversation' && (!route.parts[3] || route.parts[3] === lead.id);
          return <div key={p.id}>
            <TreeRow kind={cx('project', p.naming && 'naming')} depth={1} open={popen} onToggle={() => act.toggleExpanded(pk)} to={`/p/${p.id}/${homeTab(s, p)}`} current={inProject && !['chat', 'team'].includes(route.parts[2]!)} label={p.name} icon={null} state={ps.n ? ps.n : <span class="faint">{p.stage}</span>} tone={ps.n ? 'warn' : null} action={!live.on && <button type="button" class="icon-btn sm hover-act" aria-pressed={pinned.includes(p.id)} aria-label={(pinned.includes(p.id) ? 'Unpin ' : 'Pin ') + p.name} data-tip={pinned.includes(p.id) ? 'Unpin' : 'Pin'} data-tip-side="left" onClick={() => act.togglePin(p.id)}>
                <Icon name={pinned.includes(p.id) ? 'pin-off' : 'pin'} size={13} />
              </button>} />
            {popen && <div class="tree-children" style="--guide:27px">
              <TreeRow kind="agent" depth={2} to={`/p/${p.id}/conversation/${lead.id}`} current={onLead} icon={<Avatar agent={lead} s={s} size="sm" />} label="Project lead" state={ls.tone ? ls.word.replace('Waiting on ', '') : null} tone={ls.tone} />
              {teamsOf(s, p.id).map((t) => {
                const tk = 'team:' + t.id; const topen = s.expanded[tk] ?? false; const tn = answerableRequests(s, { team: t.id }).length;
                return <div key={t.id}>
                  <TreeRow kind="team" depth={2} open={topen} onToggle={() => act.toggleExpanded(tk)} to={`/p/${p.id}/team/${t.id}`} current={inProject && route.parts[2] === 'team' && route.parts[3] === t.id} icon={<Icon name="users" size={13} class="faint" />} label={teamName(t)} state={!topen && tn ? tn : null} tone="warn" />
                  {topen && <div class="tree-children" style="--guide:41px">
                    {agentsOfTeam(s, t.id).map((a) => {
                    const st = agentStatus(s, a);
                    return <TreeRow key={a.id} kind="agent" depth={3} to={routeForAgent(s, a)} current={inProject && route.parts[2] === 'conversation' && route.parts[3] === a.id} icon={<Avatar agent={a} s={s} size="sm" />} label={a.name} title={a.activity || st.word} state={st.tone ? st.word.replace('Stopped · ', '') : null} tone={st.tone} />;
                  })}
                  </div>}
                </div>;
              })}
            </div>}
          </div>;
        })}
      </div>}
    </div>;
  });
}
// Where the terminal of the open address opens. A project and a repository that is set up have one place.
// Any other page has the place the owner chose, or none yet.
function terminalHere(s: State, route: Route): { fixed: boolean; place: Project | null } {
  if (route.parts[0] === 'p') return { fixed: true, place: projectOf(s, route.parts[1]) || null };
  const own = route.parts[0] === 'r' ? termPlace(s, MAIN + route.parts[1]) : null;
  return own ? { fixed: true, place: own } : { fixed: false, place: termPlace(s, s.ui.termPlace) };
}
const terminalShown = (s: State, route: Route): boolean => { const t = terminalHere(s, route); return !!s.prefs.termOpen && !!t.place; };
function toggleTerminal(s: State, route: Route): void {
  const t = terminalHere(s, route);
  if (t.fixed || terminalShown(s, route)) act.setPref('termOpen', !s.prefs.termOpen);
  else act.openDialog({ type: 'terminal-where' });
}
function MockControls({ s }: { s: State }) {
  return <Menu class="icon-btn" label="Mock controls" align="end" side="top" width={260} trigger={<Icon name="sliders-horizontal" size={15} />} items={[
      { heading: 'Show a state' },
      { label: 'Connection: live', checked: s.conn.status === 'live', onSelect: () => act.setConnection('live') },
      { label: 'Connection: reconnecting', checked: s.conn.status === 'reconnecting', onSelect: () => act.setConnection('reconnecting') },
      { label: 'Connection: offline', checked: s.conn.status === 'offline', onSelect: () => act.setConnection('offline') },
      { sep: true },
      { label: 'Peek at the folded sidebar', checked: s.prefs.peek, onSelect: () => act.setPref('peek', !s.prefs.peek) },
      { sep: true },
      { label: 'Login page', icon: 'log-in', onSelect: () => { act.setSession('out'); go('/login'); } },
      { label: 'Login rejected', onSelect: () => { act.setSession('rejected'); go('/login'); } },
      { label: 'Session expired', onSelect: () => { act.setSession('expired'); go('/login'); } },
      { label: 'Service unavailable', onSelect: () => { act.setSession('unavailable'); go('/login'); } },
      { label: 'Login over an insecure connection', onSelect: () => { act.setSession('out'); go('/login?insecure=1'); } },
      { label: 'First run', icon: 'sparkles', onSelect: () => go('/setup') },
      { sep: true },
      { label: 'Reset the sample', icon: 'rotate-ccw', hint: 'Puts every request, message and setting back.', onSelect: act.resetMock },
    ]} />;
}
// The pointer rests on the left edge for this long before the folded sidebar peeks, so a pass over the edge opens nothing.
const PEEK_DWELL = 1500;
interface SidebarProps { s: State; route: Route; onNavigate?: () => void; peek: boolean; setPeek: (on: boolean) => void }
function Sidebar({ s, route, onNavigate, peek, setPeek }: SidebarProps) {
  const n = answerableRequests(s).length; const failedNotes = (s.notifications || []).filter((x) => x.delivery === 'failed').length;
  const is = (p: string) => route.path === p || route.path.startsWith(p + '/');
  const nav = (to: string, icon: string, label: string, extra?: ComponentChildren) => <a class="nav-item" href={href(to)} aria-current={is(to) ? 'page' : undefined}>
    <Icon name={icon} size={15} />
    <span class="truncate">{label}</span>
    {extra}
  </a>;
  const folded = s.prefs.sidebarFolded;
  const inSettings = ['settings', 'machines', 'harnesses', 'subscriptions', 'notifications', 'workflows'].includes(route.parts[0] || '');
  // The terminal sits with Home and the search, above the repositories.
  const terminalItem = !live.on && <button type="button" class="nav-item" aria-pressed={terminalShown(s, route)} onClick={() => toggleTerminal(s, route)}>
    <Icon name="square-terminal" size={15} />
    <span class="truncate">Terminal</span>
    <kbd style="margin-left:auto">Ctrl `</kbd>
  </button>;
  const rail = (to: string, icon: string, label: string, pip?: boolean) => <a class="icon-btn" href={href(to)} aria-label={label} data-tip={label} data-tip-side="right" aria-current={is(to) ? 'page' : undefined}>
    <Icon name={icon} size={16} />
    {pip && <i class="pip" />}
  </a>;
  // The folded sidebar keeps the pinned projects, and the open project when it is not pinned.
  const pins = live.on ? [] : (s.prefs.pinned || []).map((id) => s.projects.find((p) => p.id === id)).filter((p): p is Project => !!p && !p.archived);
  const here = !live.on && route.parts[0] === 'p' ? s.projects.find((p) => p.id === route.parts[1]) : null;
  const railProjects = here && !pins.includes(here) ? [...pins, here] : pins;
  // A strip shows on the sidebar's edge while the pointer is on it. Its arrow keeps to the pointer's height.
  // The arrow sits above the pointer, so the pointer's own shape never covers it.
  const onEdge = (e: PointerEvent) => { const el = e.currentTarget as HTMLElement; const r = el.getBoundingClientRect(); const y = e.clientY - r.top; el.style.setProperty('--y', (y < 48 ? y + 30 : y - 30) + 'px'); };
  // A drag to the right on the folded sidebar's edge opens it.
  const dragOpen = (e: PointerEvent) => {
    const x0 = e.clientX;
    const done = () => { removeEventListener('pointermove', move); removeEventListener('pointerup', done); };
    const move = (ev: PointerEvent) => { if (ev.clientX - x0 > 24) { done(); act.setPref('sidebarFolded', false); } };
    addEventListener('pointermove', move); addEventListener('pointerup', done);
  };
  // The peek: with the sidebar folded, the pointer at the left edge of the window slides the whole
  // sidebar over the page. It slides away soon after the pointer leaves it.
  const wait = useRef<ReturnType<typeof setTimeout> | number>(0);
  const later = (fn: () => void, ms: number) => { clearTimeout(wait.current); wait.current = setTimeout(fn, ms); };
  useEffect(() => {
    if (!folded) { clearTimeout(wait.current); setPeek(false); return undefined; }
    // The peek closes when the pointer leaves the window, and when the window loses the keyboard.
    const out = () => later(() => setPeek(false), 280);
    const gone = () => { clearTimeout(wait.current); setPeek(false); };
    const key = (e: KeyboardEvent) => { if (e.key === 'Escape') setPeek(false); };
    document.documentElement.addEventListener('mouseleave', out); addEventListener('blur', gone); addEventListener('keydown', key);
    return () => { document.documentElement.removeEventListener('mouseleave', out); removeEventListener('blur', gone); removeEventListener('keydown', key); };
  }, [folded]);
  const arrow = <span class="strip-icon"><Icon name={folded ? 'arrow-right-from-line' : 'arrow-left-to-line'} size={12} /></span>;
  return <aside class="sidebar" aria-label="Navigation" onPointerEnter={() => clearTimeout(wait.current)} onPointerLeave={() => { if (peek) later(() => setPeek(false), 280); }} onClick={(e) => { if ((e.target as Element).closest('a')) onNavigate?.(); }}>
    <div class="side-head">
      <a class="brand" href={href('/home')}><span class="brand-mark">LG</span><span>LGTM Studio</span></a>
      <button type="button" class="icon-btn not-phone" aria-label={folded ? 'Open the sidebar' : 'Fold the sidebar'} data-tip={(folded ? 'Open' : 'Fold') + ' sidebar · ' + (isMac ? '⌘' : 'Ctrl ') + 'B'} data-tip-side={folded ? 'right' : 'bottom'} onClick={() => act.setPref('sidebarFolded', !folded)}>
        <Icon name="panel-left" size={16} />
      </button>
    </div>
    <Slot name="sidebar.head" s={s} route={route} />
    <div class="side-rail">
      {rail('/home', 'house', n ? `Home · ${n} open ${n === 1 ? 'request' : 'requests'}` : 'Home', n > 0)}
      {rail('/projects', 'table-2', 'All projects')}
      <button type="button" class="icon-btn" aria-label="Find project or agent" data-tip="Find project or agent" data-tip-side="right" onClick={() => act.openDialog({ type: 'switcher' })}>
        <Icon name="search" size={16} />
      </button>
      <button type="button" class="icon-btn" aria-label="New project" data-tip="New project" data-tip-side="right" onClick={() => act.openDialog({ type: 'new-project' })}>
        <Icon name="plus" size={16} />
      </button>
      {railProjects.length > 0 && <div class="rail-projects" role="group" aria-label="Pinned projects">
        {railProjects.map((p) => { const pn = projectStatus(s, p).n; // the same count as the project's row in the open sidebar
        return <a key={p.id} class={cx('rail-proj', !pins.includes(p) && 'loose')} href={href(`/p/${p.id}/${homeTab(s, p)}`)} aria-current={here === p ? 'page' : undefined} aria-label={p.name + (pn ? ` · ${pn} open` : '')} data-tip={p.name + (pn ? ` · ${pn} open` : ` · ${p.stage}`)} data-tip-side="right">
          {(p.prefix || p.name.slice(0, 2)).toUpperCase()}
          {pn! > 0 && <i class="pip" />}
        </a>; })}
      </div>}
      <span class="spacer" />
      {live.on && rail('/workflows', 'workflow', 'Workflows')}
      {live.on && rail('/machines', 'server', 'Machines')}
      {live.on && rail('/harnesses', 'key-round', 'Harnesses')}
      {!live.on && rail('/usage', 'gauge', 'Usage')}
      {live.on && rail('/notifications', 'smartphone', 'Notifications', failedNotes > 0)}
      {live.on ? rail('/settings', 'settings', 'Service settings') : <a class="icon-btn" href={href('/settings')} aria-label="Settings" data-tip="Settings" data-tip-side="right" aria-current={inSettings ? 'page' : undefined}><Icon name="settings" size={16} />{failedNotes > 0 && <i class="pip" />}</a>}
      {!live.on && <button type="button" class="icon-btn" aria-label="Terminal" data-tip="Terminal" data-tip-side="right" aria-pressed={terminalShown(s, route)} onClick={() => toggleTerminal(s, route)}><Icon name="square-terminal" size={16} /></button>}
    </div>
    <div class="side-top">
      {nav('/home', 'house', 'Home', n > 0 && <span class="count warn">{n}</span>)}
      <button type="button" class="nav-item" onClick={() => act.openDialog({ type: 'switcher' })}>
        <Icon name="search" size={15} />
        <span class="truncate">Find project or agent</span>
        <Kbd k="K" />
      </button>
      {terminalItem}
    </div>
    <div class="side-scroll">
      <div class="side-label" style="margin-top:0">
        <span>Repositories</span>
        <a class="icon-btn sm" href={href('/projects')} aria-label="All projects" data-tip="All projects" data-tip-side="left" aria-current={is('/projects') ? 'page' : undefined}>
          <Icon name="table-2" size={14} />
        </a>
        <button type="button" class="icon-btn sm" aria-pressed={s.prefs.onlyNeedingYou} aria-label="Only projects needing you" data-tip="Only projects needing you" data-tip-side="left" onClick={() => update((st) => { st.prefs.onlyNeedingYou = !st.prefs.onlyNeedingYou; })}>
          <Icon name="filter" size={13} />
        </button>
        {live.on
          ? <button type="button" class="icon-btn sm" aria-label="New project" data-tip="New project" data-tip-side="left" onClick={() => act.openDialog({ type: 'new-project' })}>
            <Icon name="plus" size={14} />
          </button>
          : <button type="button" class="icon-btn sm" aria-label="Add a repository" data-tip="Add a repository" data-tip-side="left" onClick={() => act.openDialog({ type: 'track-repository' })}>
            <Icon name="plus" size={14} />
          </button>}
      </div>
      <nav aria-label="Repositories"><ProjectTree s={s} route={route} /></nav>
    </div>
    <div class="side-foot">
      {live.on && nav('/machines', 'server', 'Machines', <span class="state faint" style="margin-left:auto;font-size:var(--text-xs)">
        {s.machines.filter((m) => m.status === 'online').length}
        {' of '}
        {s.machines.length}
        {' online'}
      </span>)}
      {live.on && nav('/harnesses', 'key-round', 'Harnesses')}
      {!live.on && nav('/usage', 'gauge', 'Usage')}
      {live.on && nav('/notifications', 'smartphone', 'Notifications', failedNotes > 0 && <span class="count error">{failedNotes}</span>)}
      {live.on && nav('/workflows', 'workflow', 'Workflows')}
      {live.on && nav('/settings', 'settings', 'Service settings')}
      <Slot name="sidebar.account" s={s} route={route} />
      <div class="row" style="height:var(--row);padding:0 2px 0 8px;margin-top:4px;gap:8px">
        {/* One person uses this website, so it names no one. The foot says which server it talks to. */}
        <Icon name="server" size={15} class="faint" />
        <span class="mono truncate grow side-server" title={`Connected to ${s.server.label || location.host}`}>{s.server.label || location.host}</span>
        {/* Settings holds the machines, the harnesses, the notifications and the workflows. */}
        {!live.on && <a class="icon-btn" href={href('/settings')} aria-label="Settings" data-tip="Settings" data-tip-side="top" aria-current={inSettings ? 'page' : undefined}>
          <Icon name="settings" size={15} />
          {failedNotes > 0 && <i class="pip" />}
        </a>}
        {!live.on && <MockControls s={s} />}
        <button type="button" class="icon-btn" aria-label="Log out" data-tip="Log out" data-tip-side="left" onClick={act.signOut}>
          <Icon name="log-out" size={15} />
        </button>
      </div>
    </div>
    {folded
      ? <>
        {!peek && s.prefs.peek && <div class="peek-hot not-phone" onPointerEnter={() => later(() => setPeek(true), PEEK_DWELL)} onPointerLeave={() => clearTimeout(wait.current)} />}
        <button type="button" class="resizer right edge not-phone" aria-label="Open the sidebar" onPointerMove={onEdge} onPointerDown={dragOpen} onClick={() => act.setPref('sidebarFolded', false)}>
          {arrow}
        </button>
      </>
      : <Resizer side="right" class="edge not-phone" label="Fold or resize the sidebar" value={s.prefs.sidebarWidth} min={220} max={420} reset={264} onChange={(v) => act.setPref('sidebarWidth', v)} onTap={() => act.setPref('sidebarFolded', true)} onUnder={(w) => { act.setPref('sidebarWidth', w); act.setPref('sidebarFolded', true); }} onPointerMove={onEdge}>
        {arrow}
      </Resizer>}
  </aside>;
}

// ---- top bar ------------------------------------------------------------------
function Bell({ s }: { s: State }) {
  const now = useNow(); const open = answerableRequests(s);
  return <Menu class="icon-btn bell" label={open.length ? `${open.length} open ${open.length === 1 ? 'request' : 'requests'}` : 'No open requests'} align="end" width={360} trigger={<><Icon name="bell" size={16} />{open.length > 0 && <span class="count warn">{open.length}</span>}</>} header={<div class="menu-label">{open.length ? 'Open requests, longest wait first' : 'No open requests'}</div>} items={[
      ...open.slice(0, 6).map((r) => ({ label: <><span class={cx('kind', r.kind)} style="margin-right:6px">{KIND_LABEL[r.kind]}</span>{r.title}</>, sub: <span class={now - r.createdAt >= HOUR ? 'warn-text' : ''}>{ago(r.createdAt, now)}</span>, onSelect: () => go(routeForRequest(s, r)) })),
      { sep: true },
      { label: open.length > 6 ? `View all ${open.length}` : 'Open Home', icon: 'house', onSelect: () => go('/home') },
    ]} />;
}
function Live({ s }: { s: State }) {
  const now = useNow(5000); const c = s.conn;
  const word = connectionWord(c);
  return <Menu class={cx('live', c.status)} align="end" width={280} label={'Connection: ' + word} trigger={<>
      <i class={cx('dot', c.status === 'live' ? 'working' : c.status === 'reconnecting' ? 'waiting' : 'failed')} />
      <span class="not-phone">{word}</span>
    </>} header={<div style="padding:8px 8px 6px;line-height:1.5">
      <b>{word}</b>
      <div class="muted" style="font-size:var(--text-sm)">
        {c.notice || (c.status === 'live' ? `Connected to the service on ${(s.machines.find((m) => m.here) || s.machines[0])?.name || 'this computer'}.` : c.lastHeard ? `Last heard from the service ${ago(c.lastHeard, now)} ago. You can read, and changes wait until it is back.` : 'The service has not answered yet.')}
      </div>
    </div>} items={c.status === 'live' ? [{ label: 'Service settings', icon: 'settings', onSelect: () => go('/settings') }] : [{ label: 'Reconnect now', icon: 'refresh-cw', onSelect: () => { act.setConnection('live'); if (!live.on) act.toast('Connected'); } }]} />;
}
function TopBar({ s, route, project, onMenu }: { s: State; route: Route; project: Project | null; onMenu: () => void }) {
  const crumbs: { to: string; label: ComponentChildren; cls?: string }[] = [];
  if (project) {
    const r = repoOf(s, project);
    crumbs.push({ to: '/r/' + r.id, label: r.name, cls: 'repo' }, { to: `/p/${project.id}/${homeTab(s, project)}`, label: project.name });
    if (route.parts[2] === 'team') { const t = teamOf(s, route.parts[3]!); if (t) crumbs.push({ to: route.path, label: teamName(t) }); }
    if (route.parts[2] === 'conversation' && route.parts[3]) { const a = agentOf(s, route.parts[3]); if (a) { if (a.team) crumbs.push({ to: `/p/${project.id}/team/${a.team}`, label: teamName(teamOf(s, a.team)!) }); crumbs.push({ to: route.path, label: <><Avatar agent={a} s={s} size="sm" status={false} />{a.name}</>, cls: 'agent' }); } }
    if (route.parts[2] === 'sprint') { const sp = byId(s.sprints, route.parts[3]); const k = route.parts[5] ? byId(s.tasks, route.parts[5]) : undefined;
      if (sp) crumbs.push({ to: k ? `/p/${project.id}/sprint/${sp.id}/tasks` : route.path, label: sp.label }); if (sp && k) crumbs.push({ to: route.path, label: k.label }); }
    if (route.parts[2] === 'task') { const t = byId(s.tasks, route.parts[3]); if (t) crumbs.push({ to: route.path, label: t.label }); }
  } else if (route.parts[0] === 'r') { const r = byId(s.repos, route.parts[1]); crumbs.push({ to: '/r/' + route.parts[1], label: r?.name || 'Repository', cls: 'repo' }); }
  else { if (!live.on && ['machines', 'subscriptions', 'harnesses', 'notifications', 'workflows', 'settings'].includes(route.parts[0]!)) crumbs.push({ to: '/settings', label: 'Settings' });
    crumbs.push({ to: route.path, label: ({ home: 'Home', projects: 'All projects', machines: 'Machines', subscriptions: 'Harnesses', harnesses: 'Harnesses', usage: 'Usage', notifications: 'Notifications', settings: live.on ? 'Service settings' : 'Service', workflows: 'Workflows' } as Record<string, string>)[route.parts[0]!] || 'Home' }); }
  return <header class="topbar">
    <button type="button" class="icon-btn only-phone" aria-label="Open navigation" onClick={onMenu}><Icon name="menu" size={18} /></button>
    <nav class="crumbs" aria-label="Where you are">
      {crumbs.map((c, i) => <>
        {i > 0 && <Icon name="chevron-right" size={12} class="sep" />}
        {i === crumbs.length - 1 ? <span class={cx('here', c.cls)} aria-current="page">{c.label}</span> : <a class={c.cls} href={href(c.to)}>{c.label}</a>}
      </>)}
    </nav>
    <Bell s={s} />
    <Slot name="topbar.connection" s={s} route={route} fallback={<Live s={s} />} />
  </header>;
}

// ---- the project workspace ------------------------------------------------------
// The panel beside a project view. It shows the project lead, or, after "Open thread" on a
// question, the asking agent's thread with a way back to the lead.
// `inline` puts the thread in the conversation's column of the project, in place of the lead's.
interface SidePanelProps { s: State; agent: Agent; project: Project; inline?: boolean; onBack?: () => void; focusReq?: string | null }
function SidePanel({ s, agent, project, inline = false, onBack = act.closeSidePanel, focusReq = null }: SidePanelProps) {
  const st = agentStatus(s, agent); const isLead = agent.role === 'project-lead';
  const t = agent.team && teamOf(s, agent.team);
  const close = () => (matchMedia('(max-width: 1180px)').matches ? update((x) => { x.ui.panelDrawer = false; }) : act.setPref('panelOpen', false));
  // A div or an aside. Both take the same attributes here, so the type names one of them.
  const Root = (inline ? 'div' : 'aside') as 'div';
  return <Root class={inline ? 'mc-other' : 'panel'} aria-label={agent.name + ' thread'} style={inline ? undefined : { '--panel-w': s.prefs.panelWidth + 'px' }}>
    {!inline && <Resizer side="left" label="Resize the thread panel" value={s.prefs.panelWidth} min={340} max={640} reset={420} onChange={(v) => act.setPref('panelWidth', v)} />}
    <div class={cx('panel-head', !isLead && 'other')}>
      {!isLead && <button type="button" class="icon-btn" aria-label="Back to the project lead" data-tip="Back to the project lead" data-tip-side="right" onClick={onBack}>
        <Icon name="arrow-left" size={16} />
      </button>}
      <Avatar agent={agent} s={s} />
      <span class="who truncate">{agent.name}</span>
      {t && <span class="muted truncate" style="font-size:var(--text-sm)">{teamName(t)}</span>}
      <span class="what truncate grow"><StateWord status={st} /></span>
      {inline
        ? <Menu class="icon-btn" align="end" label="Agent actions" trigger={<Icon name="ellipsis" size={16} />} items={[
          agent.state !== 'stopped' && agent.state !== 'failed' && { label: 'Stop agent', icon: 'circle-stop', onSelect: () => act.stopAgent(agent.id) },
          agent.state === 'stopped' && { label: 'Resume', icon: 'play', onSelect: () => act.resumeAgent(agent.id) },
          { label: 'Open its terminal', icon: 'square-terminal', onSelect: () => (live.on ? go(`/p/${project.id}/terminal`) : act.setPref('termOpen', true)) },
          { label: 'Open its worktree in Git', icon: 'git-branch', onSelect: () => go(`/p/${project.id}/git`) },
        ].filter(Boolean)} />
        : <>
          <a class="icon-btn" href={href(routeForAgent(s, agent) + (s.ui.panelReq && !isLead ? '?req=' + s.ui.panelReq : ''))} aria-label="Open this thread in the main view" data-tip="Open in the main view" data-tip-side="left">
            <Icon name="maximize-2" size={14} />
          </a>
          <button type="button" class="icon-btn only-phone" aria-label="Close" onClick={close}><Icon name="x" size={16} /></button>
        </>}
    </div>
    <Thread key={agent.id} s={s} agentId={agent.id} focusReq={isLead ? null : focusReq || s.ui.panelReq} onOpenThread={act.openSidePanel} />
    {canMessage(s, agent) ? <Composer s={s} agentId={agent.id} /> : <ReadOnlyBar s={s} agent={agent} onLead={onBack} />}
  </Root>;
}
// The Agents view of the mock: the agents, and the board the agents write to each other on.
// The message board of a project in the mock. A link from the lead to one message opens that message's channel and thread.
function BoardView({ s, project, route }: { s: State; project: Project; route: Route }) {
  const post = route.parts[3] === 'post' ? s.posts.find((p) => p.id === route.parts[4]) : null;
  useEffect(() => {
    if (post) { go(`/p/${project.id}/message-board/${post.channel}?t=${post.replyTo || post.id}`); setTimeout(() => document.getElementById('post-' + post.id)?.scrollIntoView({ block: 'center' }), 80); }
  }, [post?.id]);
  return <div class="ag-view">
    <StatusNav s={s} project={project} current="messages" />
    <MessagesTab s={s} project={project} route={route} />
  </div>;
}
function ProjectView({ s, route, project }: { s: State; route: Route; project: Project }) {
  useEffect(() => { if (live.on && route.parts[2] !== 'design') void mocks.loadMocks(project.id); }, [project.id, s.boardRevision]);
  const tab = route.parts[2] || homeTab(s, project); const lead = leadOf(s, project);
  // An old address moves to the address of the page that took its place, so a page has one address.
  const oldTask = tab === 'task' ? byId(s.tasks, route.parts[3]) : undefined;
  // The four words that were in an address before 2026-11-23. Each keeps the rest of its address.
  const renamed = ({ workflow: 'status', project: 'end-result', messages: 'message-board', chat: 'conversation', mocks: 'design' } as Record<string, string>)[tab];
  const p3 = route.parts[3]; const q = route.query.toString();
  const moved = oldTask ? `sprint/${oldTask.sprint}/tasks/${oldTask.id}`
    : renamed ? [renamed, ...route.parts.slice(3).map(encodeURIComponent)].join('/') + (q ? '?' + q : '')
    : live.on ? null : tab === 'map' ? 'design/architecture' : tab === 'agents' ? 'status' : tab === 'status' && p3 === 'runs' ? 'actions' : tab === 'status' && p3 === 'result' ? 'end-result'
    : tab === 'terminal' ? 'board' : (tab === 'board' || tab === 'center') && homeTab(s, project) === 'end-result' ? 'end-result' : null;
  useEffect(() => {
    if (!moved) return;
    if (tab === 'terminal') act.setPref('termOpen', true);
    swap(`/p/${project.id}/${moved}`);
  }, [moved, project.id]);
  // The name of the page in the browser's tab and history.
  const pageName = (): string => {
    if (tab === 'sprint') return (route.parts[5] && byId(s.tasks, route.parts[5])?.label) || byId(s.sprints, p3)?.label || 'Sprint';
    if (tab === 'task') return byId(s.tasks, p3)?.label || 'Task';
    if (tab === 'team') { const tm = teamOf(s, p3!); return tm ? teamName(tm) : 'Team'; }
    if (tab === 'conversation') return (p3 && agentOf(s, p3)?.name) || 'Project lead';
    if (tab === 'status') return p3 === 'review' ? 'Reviews' : 'Status';
    if (tab === 'settings') return ({ agents: 'Agent settings', workflow: 'Workflow settings', previews: 'Preview settings' } as Record<string, string>)[p3 || ''] || 'Settings';
    if (tab === 'design') return ({ architecture: 'Architecture', browser: 'Browser', file: 'Design file' } as Record<string, string>)[p3 || ''] || 'Design';
    return ({ center: 'Command center', board: 'Board', 'end-result': 'End result', questions: 'Questions', actions: 'Actions', 'message-board': 'Message board', files: 'Files', git: 'Git' } as Record<string, string>)[tab] || 'Not found';
  };
  useEffect(() => { document.title = `${pageName()} · ${project.name} · LGTM Studio`; return () => { document.title = 'LGTM Studio'; }; }, [route.path, project.name]);
  // An address that is moving shows nothing for the one frame before it moves.
  if (moved) return null;
  const chatAgent = tab === 'conversation' ? agentOf(s, route.parts[3] || lead.id) : null;
  const other = s.ui.panelAgent && agentOf(s, s.ui.panelAgent);
  const side = other && other.project === project.id && other.id !== chatAgent?.id ? other : null;
  const narrow = useMedia('(max-width: 1180px)');
  const open = answerableRequests(s, { project: project.id }).length;
  const tabs: TabItem[] = [
    // One place: the end result while the project is being defined, and the board from then on.
    // A defined project's end result opens from Status.
    // The command center is the first view of a project that is past Define.
    !live.on && homeTab(s, project) === 'center' && { id: 'center', label: 'Command center', icon: 'layout-dashboard', short: true },
    (live.on || homeTab(s, project) !== 'end-result') && { id: 'board', label: 'Board' },
    !live.on && homeTab(s, project) === 'end-result' && { id: 'end-result', label: 'End result' },
    // The mock has a view for the questions of a project. The live website does not have it yet.
    !live.on && { id: 'questions', label: 'Questions', count: waitingQuestions(s, project.id).length, tone: 'warn' },
    // The view says where the project is, so its tab carries the stage. The address keeps its old name.
    { id: 'status', label: <>Status<span class="tab-word">{project.stage}</span></> },
    // Actions says in a word when the last full run failed. Its old address was a tab of Status.
    { id: 'actions', label: actionsFailed(s, project) ? <>Actions<i class="dot failed" role="img" aria-label="The last full run failed" data-tip="The last full run failed" /></> : 'Actions' },
    // Design holds the drawn screens, the design file, the running product and the architecture.
    (!live.on || project.hasMocks || project.stage === 'Design' || s.mocks[project.id]?.screens.length) && { id: 'design', label: 'Design' },
    // The mock has no Chat tab: the conversation keeps to the left of every view.
    live.on && (chatAgent && chatAgent.role !== 'project-lead'
      ? { id: 'conversation', label: agentLabel(s, chatAgent, { withTeam: true }), icon: 'eye' }
      : { id: 'conversation', label: 'Chat', count: open, tone: 'warn' }),
    // In the mock the message board is a tab of Status, and the terminal is a panel under the view.
    live.on && { id: 'message-board', label: 'Messages' },
    live.on && { id: 'agents', label: 'Agents' },
    live.on && { id: 'terminal', label: 'Terminal' },
    { id: 'files', label: 'Files' },
    { id: 'git', label: 'Git' },
    { id: 'settings', label: !live.on && project.assignment?.state !== 'approved' ? <>Settings<i class="dot waiting" role="img" aria-label="The model assignment waits for your approval" data-tip="The model assignment waits for your approval" /></> : 'Settings' },
  ].filter(Boolean);
  const actions = tab === 'actions' || tab === 'status' && route.parts[3] === 'runs';
  // An address that names an agent the project does not have says so.
  const lost = !live.on && tab === 'conversation' && !!p3 && (!chatAgent || chatAgent.project !== project.id);
  // Another agent's page stands beside its conversation. The old address of the Agents view opens Status.
  const viewed = !live.on && tab === 'conversation' && chatAgent && chatAgent.project === project.id && chatAgent.id !== lead.id ? chatAgent : null;
  // After Define the end result is a tab of Status, and the lead's own address shows the board.
  const defined = !live.on && homeTab(s, project) === 'center';
  const status = tab === 'status' || !live.on && (tab === 'agents' || tab === 'message-board') || defined && tab === 'end-result';
  // The terminal is a panel under the view, as in a code editor. Its old address opens the board with the panel.
  const term = !live.on && (!!s.prefs.termOpen || tab === 'terminal');
  const current = actions ? 'actions' : viewed ? null : status ? 'status' : ['sprint', 'task'].includes(tab) ? 'board' : tab === 'team' ? (live.on ? null : 'status') : lost ? null : !live.on && tab === 'conversation' ? (defined ? 'center' : 'end-result') : tab;
  // The panel docks the lead beside every view except Chat, which is the thread itself.
  // The Project view holds the lead's conversation itself.
  const docked = side || (tab === 'conversation' || tab === 'end-result' ? null : lead);
  const wantPanel = docked && s.prefs.panelOpen && !(narrow && s.ui?.panelAuto !== true);
  const showPanel = docked && (narrow ? s.ui?.panelDrawer : s.prefs.panelOpen || !!side);
  const togglePanel = () => (narrow ? update((st) => { st.ui = { ...(st.ui || {}), panelDrawer: !st.ui?.panelDrawer }; }) : act.setPref('panelOpen', !s.prefs.panelOpen));
  let body: ComponentChildren;
  if (lost) body = <div class="page">
    <Empty title="This agent does not exist">
      <a class="link" href={href(`/p/${project.id}/status`)}>Project agents</a>
    </Empty>
  </div>;
  else if (viewed) body = <AgentPage s={s} project={project} agent={viewed} />;
  else if (defined && tab === 'end-result') body = <div class="ag-view">
    <StatusNav s={s} project={project} current="result" />
    <EndResultView s={s} project={project} />
  </div>;
  else if (defined && (tab === 'conversation' || tab === 'center')) body = <ProjectCenter s={s} project={project} />;
  else if ((tab === 'end-result' || tab === 'conversation') && !live.on) body = <EndResultView s={s} project={project} />;
  else if (tab === 'board') body = <Board s={s} project={project} route={route} />;
  else if (tab === 'sprint') body = <SprintPage s={s} project={project} sprintId={route.parts[3]!} tab={route.parts[4]} task={route.parts[5]} />;
  else if (tab === 'task') body = <TaskPage s={s} project={project} taskId={route.parts[3]!} />;
  else if (tab === 'team') body = <TeamPage s={s} project={project} teamId={route.parts[3]!} />;
  else if (!live.on && tab === 'questions') body = <QuestionsTab s={s} project={project} route={route} />;
  else if (actions) body = <ActionsTab s={s} project={project} route={route} />;
  else if (status && tab !== 'message-board' && tab !== 'end-result') body = <WorkflowTab s={s} project={project} route={route} />;
  else if (tab === 'design') body = <DesignTab s={s} project={project} route={route} />;
  else if (tab === 'conversation') body = <ChatTab s={s} project={project} agentId={chatAgent?.id || route.parts[3]} route={route} />;
  else if (!live.on && tab === 'message-board') body = <BoardView s={s} project={project} route={route} />;
  else if (!live.on && tab === 'terminal') body = <Board s={s} project={project} route={route} />;
  else if (tab === 'message-board') body = <MessagesTab s={s} project={project} route={route} />;
  else if (tab === 'agents') body = <AgentsTab s={s} project={project} />;
  else if (live.on && ['terminal', 'files'].includes(tab)) body = <NotConnected what={tab === 'files' ? 'Files' : 'The terminal'} />;
  else if (tab === 'terminal') body = <TerminalTab s={s} project={project} route={route} />;
  else if (tab === 'files') body = <FilesTab s={s} project={project} route={route} />;
  else if (tab === 'git') body = <GitTab s={s} project={project} />;
  else if (tab === 'settings') { const page = live.on ? '' : ['agents', 'workflow', 'previews'].includes(p3 || '') ? p3! : 'general';
    // Settings has four short pages, so no page needs a long scroll: the project and its code, who
    // works on it, the rules of its workflow, and what the Browser page can run.
    const proposed = !live.on && project.assignment?.state !== 'approved';
    body = live.on ? <ProjectSettings s={s} project={project} /> : <div class="wf" data-main>
      <nav class="wf-nav" aria-label="Settings pages">
        {[['general', 'General', ''], ['agents', 'Agents', '/agents'], ['workflow', 'Workflow', '/workflow'], ['previews', 'Previews', '/previews']].map(([id, label, path]) => <a key={id} class="wf-tab" aria-current={page === id ? 'page' : undefined} href={href(`/p/${project.id}/settings${path}`)}>
          {label}
          {id === 'agents' && proposed && <i class="dot waiting" role="img" aria-label="The model assignment waits for your approval" data-tip="The model assignment waits for your approval" />}
        </a>)}
        <span class="grow" />
        <Saved s={s} />
      </nav>
      {page === 'workflow' ? <WorkflowSettings s={s} project={project} /> : page === 'previews' ? <PreviewSettings s={s} project={project} />
        : <ProjectSettings s={s} project={project} page={page as 'general' | 'agents'} />}
    </div>; }
  else if (tab === 'board' || live.on) body = <Board s={s} project={project} route={route} />;
  else body = <div class="page">
    <Empty title="This page does not exist">
      <a class="link" href={href(`/p/${project.id}/${homeTab(s, project)}`)}>Open project</a>
    </Empty>
  </div>;
  const flush = ['end-result', 'conversation', 'terminal', 'git', 'files', 'message-board', 'design', 'board', 'sprint', 'questions'].includes(tab);
  // With the terminal open, the view keeps the upper part and the terminal takes the lower part.
  const fill = !viewed && (flush || tab === 'conversation');
  if (term) body = <div class="with-term">
    <div class={cx('wt-view', fill && 'fill')}>{body}</div>
    <TerminalPanel s={s} place={project} onClose={() => { if (tab === 'terminal') go(`/p/${project.id}/board`); }} />
  </div>;
  const focus = !!s.ui.focus && (tab === 'board' || tab === 'design' && p3 === 'architecture');
  if (!live.on) {
    // Another agent's thread takes the lead's place in the conversation's column, from its address or from "Open thread".
    const named = tab === 'conversation' && chatAgent && chatAgent.project === project.id && chatAgent.id !== lead.id ? chatAgent : null;
    const otherAgent = named || side;
    const back = () => { act.closeSidePanel(); if (named) { if (history.length > 1) history.back(); else go(`/p/${project.id}/${homeTab(s, project)}`); } };
    // Git, Files, the message board and Questions have a list of their own. It sits on the far right, and it can be hidden.
    const hasList = ['git', 'files', 'message-board'].includes(tab) || (tab === 'questions' && questionsOf(s, project.id).length > 0);
    return <div class="workspace">
      <div class="view">
        <ProjectShell s={s} project={project} path={route.path} home={current === 'end-result' && homeTab(s, project) === 'end-result'} flush={term || fill} focus={focus} other={otherAgent && <SidePanel s={s} agent={otherAgent} project={project} inline onBack={back} focusReq={route.query.get('req')} />} list={hasList && s.prefs.listOpen} tabrow={<div class="tabrow" role="tablist" aria-label="Project views" onClick={(e) => { if ((e.target as Element).closest('.tab')) showView(); }}>
          <TalkToggle s={s} home={current === 'end-result' && homeTab(s, project) === 'end-result'} locked={!!otherAgent} who={otherAgent ? agentLabel(s, otherAgent, { withTeam: true }) : 'Project lead'} short={otherAgent ? otherAgent.name : 'Lead'} />
          {!named && <TalkPick s={s} project={project} current={otherAgent || lead} />}
          <Tabs tabs={tabs} current={current} base={`/p/${project.id}/`} />
          <div class="tab-end not-phone">
            {current === 'center' && <ProjectWidgets s={s} />}
            <StageMark s={s} project={project} mid={hasList && <button type="button" class="icon-btn" aria-pressed={!!s.prefs.listOpen} aria-label={s.prefs.listOpen ? 'Hide the list' : 'Show the list'} title={s.prefs.listOpen ? 'Hide the list' : 'Show the list'} onClick={() => act.setPref('listOpen', !s.prefs.listOpen)}>
              <Icon name="panel-right" size={15} />
            </button>} />
          </div>
        </div>}>
          {body}
        </ProjectShell>
      </div>
    </div>;
  }
  return <div class="workspace">
    <div class="view">
      {!focus && <div class="tabrow" role="tablist" aria-label="Project views">
        <Tabs tabs={tabs} current={current} base={`/p/${project.id}/`} />
        {docked && !side && <div class="tab-end">
          <button type="button" class="btn sm" aria-pressed={!!showPanel} onClick={togglePanel} data-tip={showPanel ? 'Hide the project lead' : 'Show the project lead'} data-tip-side="left">
            <Avatar agent={lead} s={s} size="sm" />
            <span>Project lead</span>
            <Icon name="panel-right" size={14} />
          </button>
        </div>}
      </div>}
      <div class="view-body" style={flush ? 'overflow:hidden;display:flex;flex-direction:column' : ''}>{body}</div>
    </div>
    {showPanel && !focus && <>
      {narrow && <div class="drawer-scrim" style="z-index:71" onClick={togglePanel} />}
      <SidePanel s={s} agent={docked!} project={project} />
    </>}
  </div>;
}

// A screen whose records the engine cannot give yet, in live mode. It never shows the sample.
function NotConnected({ what }: { what: string }) {
  return <div class="page"><Empty title={`${what} is not connected yet`}>The service does not send this to the website yet.</Empty></div>;
}

// ---- the app --------------------------------------------------------------------
function App() {
  const s = useStore(); const route = useRoute();
  const compatibility = !live.on && !s.server.repositoryMode;
  // The mock has the Questions view, so a question's card opens there.
  questionsView.on = compatibility;
  const [drawer, setDrawer] = useState(false); const [peek, setPeek] = useState(false);
  useKey((e) => {
    const mod = isMac ? e.metaKey : e.ctrlKey;
    if (mod && e.key.toLowerCase() === 'k') { e.preventDefault(); act.openDialog({ type: 'switcher' }); }
    if (mod && e.key.toLowerCase() === 'b') { e.preventDefault(); act.setPref('sidebarFolded', !store.state.prefs.sidebarFolded); }
    // Control and the backtick key open and close the terminal, on every keyboard, as in a code editor.
    if (!live.on && !store.state.server.repositoryMode && e.ctrlKey && e.code === 'Backquote') { e.preventDefault(); toggleTerminal(store.state, parseRoute()); }
  }, []);
  useEffect(() => {
    if (!route.parts.length) go('/home');
    if (route.parts[0] === 'subscriptions') go('/harnesses');
    // The old Home tabs keep working: Projects moved to its own page, and Questions and
    // Approvals became the Type filter on Needs attention.
    if (route.path === '/home/projects') go('/projects');
  }, [route.path]);
  // A project's page names itself. Every other page keeps the product's name.
  useEffect(() => { if (route.parts[0] !== 'p' && (live.on || route.parts[0] !== 'r')) document.title = 'LGTM Studio'; }, [route.parts[0]]);
  // The live website opens Home once the engine says this browser is signed in.
  useEffect(() => { if (live.on && s.session.status === 'in' && (route.parts[0] === 'login' || s.session.firstRun && route.parts[0] !== 'setup')) go(s.session.firstRun ? '/setup' : '/home'); }, [s.session.status, s.session.firstRun, route.path]);
  const top = route.parts[0] || 'home';
  // A signed-in page reads the server's records before it shows a screen that needs them.
  useEffect(() => { if (!live.on && s.server.status === 'ready' && s.session.status === 'in' && !s.server.synced) server.ensureReading(); }, [s.server.status, s.session.status, s.server.synced]);
  // The website has no server until one answers at an address.
  if (!live.on && s.server.status !== 'ready') return <><Connect s={s} /><Toasts s={s} /></>;
  if (!live.on && s.session.status === 'in' && !s.server.synced) return <main class="auth" aria-busy="true" />;
  // The live website asks the engine whether this browser is signed in before it shows anything.
  if (s.session.status === 'checking') return <main class="auth" aria-busy="true" />;
  if (top === 'login' || (s.session.status !== 'in' && (top !== 'setup' || !compatibility))) return <><Login s={s} /><Toasts s={s} /></>;
  const waiting = live.on && !s.snapshotReady;
  if (top === 'setup' && !s.server.repositoryMode && !waiting) return <><FirstRun s={s} /><DialogHost s={s} /><Toasts s={s} /></>;
  const project = top === 'p' ? projectOf(s, route.parts[1]) : null;
  let content: ComponentChildren;
  if (waiting) content = <div class="view-body">
    <div class="page" role="status" aria-busy={!s.conn.notice || !!s.conn.starting}>
      <Empty title={!s.conn.starting && s.conn.notice || 'Loading…'} />
    </div>
  </div>;
  else if (top === 'p' && project) content = <ProjectView s={s} route={route} project={project} />;
  else if (top === 'p') content = <div class="view-body">
    <div class="page">
      <div class="empty" data-empty>
        <strong>This project does not exist</strong>
        <span><a class="link" href={href('/home')}>Go to Home</a></span>
      </div>
    </div>
  </div>;
  // The live website keeps the older repository page until the engine knows a repository agent.
  else if (top === 'r') content = !live.on ? <RepoView s={s} repoId={route.parts[1]!} route={route} /> : route.parts[2] === 'design' ? <RepoDesign s={s} repoId={route.parts[1]!} route={route} /> : <RepoPage s={s} repoId={route.parts[1]!} />;
  else if (top === 'projects') content = <AllProjects s={s} />;
  else if (!live.on && ['machines', 'subscriptions', 'harnesses', 'notifications', 'workflows'].includes(top)) content = <SettingsShell s={s} current={top === 'subscriptions' ? 'harnesses' : top}>
    <div class="svc-page">{top === 'machines' ? <Machines s={s} /> : top === 'notifications' ? <Notifications s={s} /> : top === 'workflows' ? <Workflows s={s} /> : <Harnesses s={s} />}</div>
  </SettingsShell>;
  else if (top === 'machines') content = <Machines s={s} />;
  else if (top === 'subscriptions' || top === 'harnesses') content = <Harnesses s={s} />;
  else if (top === 'usage' && !live.on) content = <Usage s={s} />;
  else if (top === 'notifications') content = <Notifications s={s} />;
  else if (top === 'workflows') content = <Workflows s={s} />;
  else if (top === 'settings') content = <ServiceSettings s={s} route={route} />;
  else if (top === 'home' || top === 'usage') content = <Home s={s} route={route} />;
  else content = <div class="view-body">
    <div class="page">
      <Empty title="This page does not exist"><a class="link" href={href('/home')}>Go to Home</a></Empty>
    </div>
  </div>;
  // Outside a project and a repository, the terminal opens under the page, in the place the owner chose.
  const loose = compatibility && !waiting && top !== 'p' && !terminalHere(s, route).fixed && terminalShown(s, route) ? terminalHere(s, route).place : null;
  if (loose) content = <div class="with-term"><div class="wt-view fill">{content}</div><TerminalPanel s={s} place={loose} /></div>;
  // Focus hides the sidebar, the top bar, the tab row and the lead panel, so the board fills the window.
  const focus = s.ui.focus && (top === 'p' && ((route.parts[2] || homeTab()) === 'board' || route.parts[2] === 'design' && route.parts[3] === 'architecture') || top === 'r' && route.parts[2] === 'board');
  return <div class={cx('app', s.prefs.sidebarFolded && 'folded', s.prefs.sidebarFolded && peek && 'peek', drawer && 'drawer-open', focus && 'focus')} style={{ '--sidebar-w': s.prefs.sidebarWidth + 'px' }}>
    <button type="button" class="btn skip" onClick={() => { const m = document.querySelector<HTMLElement>('.view-body'); m?.setAttribute('tabindex', '-1'); m?.focus(); }}>
      Skip to content
    </button>
    <Sidebar s={s} route={route} onNavigate={() => setDrawer(false)} peek={peek} setPeek={setPeek} />
    {drawer && <div class="drawer-scrim" onClick={() => setDrawer(false)} />}
    <div class="main"><TopBar s={s} route={route} project={project} onMenu={() => setDrawer(true)} />{content}</div>
    <DialogHost s={s} />
    <Toasts s={s} />
  </div>;
}

// ---- start ----------------------------------------------------------------------
// The mock starts through mount(), so that another page can load it as a package.
// This folder's own page calls it from main.js. A page that loads the package can pass:
//   connection   receives every engine call and pushed update; see engine.js
//   slots        components for the named places above, by name
export { App };
export interface MountOptions { connection?: Connection | null; slots?: Record<string, SlotFill>; mode?: 'mock' | 'live' | string }
export function mount(el: Element, { connection, slots: fills, mode = 'mock' }: MountOptions = {}) {
  setDraftNotice(() => act.toast('This draft is too long to keep after a reload.', 'error'));
  engine.setConnection(connection);
  slots = fills || {};
  act.stopSimulation();
  live.on = mode === 'live';
  if (live.on) return mountLive(el);
  store.state = emptyWorkspace();
  act.applyPrefs();
  matchMedia('(prefers-color-scheme: dark)').addEventListener('change', act.applyPrefs);
  let hasSampleWorkspace = false;
  server.start((repositoryMode) => {
    if (!repositoryMode && hasSampleWorkspace) return;
    act.stopSimulation();
    const previous = store.state;
    const next = repositoryMode ? emptyWorkspace() : sampleWorkspace();
    next.server = { ...previous.server };
    next.prefs = { ...previous.prefs, pinned: next.prefs.pinned };
    store.state = next;
    hasSampleWorkspace = !repositoryMode;
    if (hasSampleWorkspace) {
      act.startSimulation();
      engine.call('snapshot', {});
      engine.call('subscribe', { after: 0 });
    }
  });
  render(<App />, el);
  return { store, act };
}

function sampleWorkspace(): State {
  const state = seed() as State;
  state.ui = {};
  for (const add of [seedDefine, seedGit, seedTerminal, seedMocks, seedBoard, seedWorkflow, seedService, seedUsage, seedAuth]) add(state);
  return state;
}

function emptyWorkspace(): State {
  const state = emptyState() as State;
  seedTerminal(state);
  return Object.assign(state, {
    ui: {}, define: {}, worktrees: [], gitUi: {}, boardRead: {}, boardUnread: {}, boardRevision: 0,
    boardDetails: {}, taskDetails: {}, setupData: null, boardAgents: {}, projectLeadSettings: {}, notifications: [],
    workflow: {}, mocks: {}, design: {}, machineDetails: null,
    usage: { rows: [], limits: {}, review: { ranAt: 0, findings: [] } },
  });
}

// ---- live mode -------------------------------------------------------------------
// The page that loads the website chooses live mode with mount(el, { connection, mode: 'live' }).
// The store starts empty, nothing from seed() is read, and the simulation never starts. The
// connection fills the store from the engine. It calls the function it is given with a patch
// of whole store keys, such as { repos, projects, agents } or { session } or { conn }, already
// in the store's shape. See engine.js for what the connection provides.
/** A patch from the connection: store keys, and a few signals that are not store keys. */
type LivePatch = Partial<State> & { chatEvent?: Parameters<typeof chat.receive>[0]; git?: Parameters<typeof liveGit.observe>[0]['git'] };
function mountLive(el: Element) {
  const state = emptyWorkspace();
  for (const key of Object.keys(state.settings)) state.settings[key] = typeof state.settings[key] === 'boolean' || typeof state.settings[key] === 'number' ? null : typeof state.settings[key] === 'string' ? '' : {};
  state.settings.quiet = { on: null, from: '', to: '', failed: null };
  store.state = guardStore(state);
  act.applyPrefs();
  matchMedia('(prefers-color-scheme: dark)').addEventListener('change', act.applyPrefs);
  // A screen that changes an engine record outside update() is refused the same way.
  addEventListener('error', onRefusedError);
  addEventListener('unhandledrejection', onRefusedError);
  engine.connection().open?.((patch: LivePatch) => {
    if (patch.session && patch.session.status !== 'checking' && patch.session.status !== 'in') {
      chat.reset(patch.session.status); engineWrite({ threads: {}, pending: {} });
    }
    if (patch.snapshotReady === undefined && patch.projects) patch.snapshotReady = true;
    if (patch.chatEvent) { if (patch.git) liveGit.observe({ git: patch.git }); chat.receive(patch.chatEvent); return; }
    chat.enrich(patch);
    // The Workflow and Mocks tabs keep one record per project. Each live project gets an
    // empty one, frozen like the rest, so the tabs show their empty structure.
    if (patch.projects) Object.assign(patch, {
      workflow: Object.fromEntries(patch.projects.map((p) => [p.id, store.state.workflow[p.id] || emptyWorkflow()])),
      mocks: Object.fromEntries(patch.projects.map((p) => [p.id, store.state.mocks[p.id] || emptyMocks()])),
    });
    // A harness the page has no words for is named by the engine's id for it.
    for (const x of [...(patch.agents || []), ...(patch.subscriptions || [])]) {
      if (x.harness && !HARNESSES[x.harness]) HARNESSES[x.harness] = { name: x.harness, models: [], efforts: [], noEffort: [], runMode: 'ACP' };
    }
    liveGit.observe(patch);
    engineWrite(patch);
    if (patch.conn?.status === 'live') chat.connected();
    // The sidebar opens each repository the first time the engine names it, as the sample does.
    if (patch.repos) for (const r of patch.repos) if (store.state.expanded['repo:' + r.id] === undefined) store.state.expanded['repo:' + r.id] = true;
  });
  render(<App />, el);
  return { store, act };
}
function onRefusedError(e: Event) {
  const error = (e as ErrorEvent).error || (e as PromiseRejectionEvent).reason;
  if (live.on && refusal(error)) { e.preventDefault(); refuse(); }
}

// Takes the website out of the element and returns to the default connection.
export function unmount(el: Element): void {
  render(null, el);
  act.stopSimulation();
  if (live.on) { chat.reset(); engine.connection().close?.(); removeEventListener('error', onRefusedError); removeEventListener('unhandledrejection', onRefusedError); live.on = false; }
  engine.setConnection(null);
  slots = {};
}
