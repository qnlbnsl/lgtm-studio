import { errorText } from '../live/reads';
import { AddHarness, SUPPORTED } from './harness-catalog';
import * as liveGit from '../live/git';
import * as signInUi from '../live/signin';
// The service pages: Machines, Harnesses, Notifications and Service settings,
// plus one project's Settings tab and the Add repository dialog.
import { useState, useEffect, useRef } from 'preact/hooks';
import {
  Icon, cx, update, store, go, href, ago, clock, dayLabel, useNow, uid, plural, MIN, HOUR, DAY, live, refuse,
} from '../lib';
import {
  usableSubscription, HARNESSES, PERMISSION_MODES, KIND_LABEL, byId, projectOf, repoOf, agentOf, teamsOf, teamName, agentsOfTeam, agentsOfProject,
  machineOf, subOf, subLabel, agentStatus, routeForAgent, routeForRequest, openRequests, hasEffort, harnessOfModel, WORK_KINDS, proposeAssignment,
  repoPrefix,
} from '../data';
import { homeTab } from '../define';
import * as act from '../actions';
import * as board from '../live/board';
import * as mocks from '../live/mocks';
import * as server from '../server/sync';
import { Avatar, Menu, Dialog, registerDialog, Empty, StateWord, CodeBox, SettingHelp } from '../ui';
import type { ComponentChildren, ComponentType, TargetedEvent } from 'preact';
import type { Agent, DialogState, Harness, HarnessRecipe, Id, Loose, Machine, Ms, Notification, Project, ProjectAssignment, Route, Settings, State, Subscription, Team } from '../types';
import type { SignInCheck } from '../live/signin';
import type * as liveSettings from '../live/project-settings';

interface ScreenProps { s: State }
interface DialogProps { s: State; d: DialogState }
// The source calls these three on the board module, which does not export them. The website's
// build replaces each call with live/project-settings; the mock never calls them. Kept as found.
const boardLive = board as typeof board & Pick<typeof liveSettings, 'changePermission' | 'changeApproval' | 'deleteTeam'>;
type Field = TargetedEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>;
// `null!` keeps a null the source passes where the attribute types want undefined. Preact 11
// writes title={null} as an empty attribute and leaves title={undefined} out, so the value stays.
// spellcheck="false" is passed as the string the source has. Preact sets it as the property, so
// spellcheck stays on, as it does in the mock.

// ---- seed ----------------------------------------------------------------------
export function seedService(st: State): void {
  const T = st.T;

  const subMore: Record<string, Partial<Subscription>> = {
    'sub-max': { account: 'owner@example.com', window: { pct: 34, resetsAt: T + 2 * HOUR + 10 * MIN }, model: 'Opus 5.5' },
    'sub-team': { account: 'owner@acme.example', window: { pct: 78, resetsAt: T + 48 * MIN }, model: 'Sonnet 5.5' },
    'sub-codex': { account: 'owner@example.com', window: { pct: 12, resetsAt: T + 3 * HOUR + 30 * MIN }, model: 'GPT-6.1 Sol' },
    'sub-codex-work': { account: null, window: null, model: 'GPT-6.1 Sol' },
  };
  for (const x of st.subscriptions) Object.assign(x, subMore[x.id]);
  st.subscriptions.splice(2, 0, {
    id: 'sub-pro', harness: 'claude', name: 'Pro', machine: 'studio', enabled: true, signedIn: true, useForNew: false, account: 'owner@example.com',
    usage: { pct: 67, resetsAt: T + 4 * DAY }, weekly: 67, window: { pct: 100, resetsAt: T + 1 * HOUR + 40 * MIN }, model: 'Sonnet 5.5',
  });

  Object.assign(st.harnessCatalog, {
    gemini: { name: 'Gemini CLI', models: [], efforts: [], noEffort: [], runMode: 'ACP' },
    cursor: { name: 'Cursor', models: [], efforts: [], noEffort: [], runMode: 'ACP' },
  });
  for (const [id, h] of Object.entries(st.harnessCatalog)) {
    const pkg = ({ claude: '@anthropic-ai/claude-code@2.1.325', gemini: '@google/gemini-cli@0.68.0' } as Record<string, string>)[id];
    Object.assign(h, { installed: ['claude', 'codex'].includes(id), version: ({ claude: '2.4.1', codex: '0.61.0' } as Record<string, string>)[id] ?? null, defaultModel: h.models[0] || '', defaultEffort: h.efforts[1] || '' });
    h.recipe = { automatic: id === 'gemini', packages: pkg ? [pkg] : [], commands: id === 'codex' ? [] : [id === 'claude' ? `npm install -g ${pkg}` : pkg ? `'/usr/local/bin/node' '/usr/local/lib/node_modules/npm/bin/npm-cli.js' 'ci' '--prefix' '/home/maya/.local/share/lgtm/harnesses/${id}' '--registry' 'https://registry.npmjs.org' '--ignore-scripts' '--no-audit' '--no-fund'` : 'curl https://cursor.com/install -fsS | bash'], source: ({ claude: 'https://code.claude.com/docs/en/setup', codex: '', gemini: 'https://geminicli.com/docs/get-started/installation/', cursor: 'https://cursor.com/docs/cli/installation' } as Record<string, string>)[id], signInCommand: ({ claude: 'claude auth login', codex: 'codex login', gemini: 'gemini', cursor: 'agent login' } as Record<string, string>)[id], reason: id === 'codex' ? 'Codex is bundled. There is nothing to install.' : ['claude', 'cursor'].includes(id) ? 'Run the official installer in your terminal, then check again.' : 'Install scripts are disabled. The committed lockfile fixes the dependency tree and integrity hashes.' };
  }

  // What the bell has shown, newest first, and whether each one reached the phone.
  const notes: Notification[] = st.requests.map((r) => ({ id: 'n-' + r.id, ts: r.createdAt, kind: r.kind, title: r.title, request: r.id, project: r.project, agent: r.agent, read: r.status !== 'open', delivery: 'off' }));
  notes.push({ id: 'n-sp1', ts: T - 26 * HOUR, kind: 'sprint', title: 'LS-A01-1 Navigation foundation is merged', project: 'console', agent: 'console-lead', to: '/p/console/sprint/sp1', read: true, delivery: 'off' });
  st.notifications = notes.sort((a, b) => b.ts - a.ts);

  Object.assign(st.settings, {
    notifyOn: { question: true, approval: true, merge: true, failed: true, sprint: false },
    quiet: { on: false, from: '22:00', to: '07:00', timeZone: 'UTC', failed: true },
    workspaceFolder: '/home/maya/projects', terminalEnabled: false, defaultHarness: 'claude', defaultModel: 'Sonnet 5.5', defaultEffort: 'Medium', defaultPermission: 'Automatic review',
    needToKnow: false, stallAssigned: 5, stallWorker: 10, stallLead: 20, autoNudge: true, replaceAfter: 3, capacityRetries: 12,
    workflowPath: '~/.lgtm', maxRounds: 6, testsDefault: true, reviewDefault: true,
    version: '0.9.0', commit: '55e1bbd', dataFolder: '~/.local/state/lgtm',
  });
}

// ---- small parts ---------------------------------------------------------------
const until = (ts: Ms, now: Ms): string => ago(2 * now - ts, now);
const machineAgents = (s: State, mid: Id): Agent[] => s.agents.filter((a) => projectOf(s, a.project)?.machine === mid && !projectOf(s, a.project)?.archived);
const running = (a: Agent): boolean => ['working', 'waiting', 'starting'].includes(a.state);
const subAgents = (s: State, sid: Id): Agent[] => s.agents.filter((a) => a.subscription === sid && !projectOf(s, a.project)?.archived);

const savedTimers = new WeakMap<State, ReturnType<typeof setTimeout>>();
export function markSaved() {
  const workspace = store.state;
  const ui = workspace.ui;
  update((s) => { s.ui.saved = Date.now(); });
  clearTimeout(savedTimers.get(workspace));
  const clear = () => {
    savedTimers.delete(workspace);
    if (store.state === workspace && workspace.ui === ui) update((s) => { s.ui.saved = null; });
  };
  const timer = live.on || workspace.server.repositoryMode ? setTimeout(clear, 1800) : act.sampleLater(clear, 1800);
  savedTimers.set(workspace, timer);
}
const setS = (patch: Partial<Settings>) => { if (live.on) return liveGit.saveSettings(patch); update((s) => { Object.assign(s.settings, patch); }); markSaved(); };
export function Saved({ s }: ScreenProps) {
  return <span class={cx('svc-saved', s.ui.saved && 'on')} role="status" aria-live="polite">
    {s.ui.saved && <><Icon name="check" size={13} />Saved</>}
  </span>;
}
interface SwitchProps { on: boolean | null | undefined; onChange: (on: boolean) => unknown; label: string; disabled?: boolean }
export function Switch({ on, onChange, label, disabled }: SwitchProps) {
  return <button type="button" class="switch" role="switch" aria-checked={!!on} title={live.on && on == null ? 'Not set by the service' : null!} aria-label={label} disabled={disabled} onClick={() => onChange(!on)} />;
}
interface SegOption { id: string; label: string; icon?: string }
function Seg({ value, options, onChange, label }: { value: string; options: SegOption[]; onChange: (id: string) => unknown; label: string }) {
  return <div class="seg" role="group" aria-label={label}>
    {options.map((o) => <button key={o.id} type="button" aria-pressed={value === o.id} onClick={() => onChange(o.id)}>
    {o.icon && <Icon name={o.icon} size={13} />}
    {o.label}
  </button>)}
  </div>;
}
function Meter({ pct, wide }: { pct: number | undefined; wide?: boolean }) {
  // A machine whose figure is not known yet shows a dash, as an offline machine does.
  if (typeof pct !== 'number') return <span class="faint">—</span>;
  const tone = pct >= 100 ? 'full' : pct >= 80 ? 'warn' : null;
  return <span class={cx('svc-meter', wide && 'wide')}>
    <span class="progress" role="img" aria-label={pct + ' percent used'}>
      <i class={tone!} style={{ width: Math.min(100, pct) + '%' }} />
    </span>
    <span class={cx('num', tone === 'full' && 'error-text', tone === 'warn' && 'warn-text')}>{pct}%</span>
  </span>;
}
// One setting: its name and a short explanation on the left, the control on the right.
interface RowProps { label: string; hint?: ComponentChildren; children?: ComponentChildren; id?: string; top?: boolean }
function Row({ label, hint, children, id, top }: RowProps) {
  return <div class={cx('svc-row', top && 'top')}>
    <div class="svc-label"><label for={id}>{label}</label>{hint && <small>{hint}</small>}</div>
    <div class="svc-control">{children}</div>
  </div>;
}
interface NumProps { id?: string; value: number | string | undefined; unit?: string; min?: number; max?: number; onChange: (value: number) => unknown }
function Num({ id, value, unit, min = 0, max = 999, onChange }: NumProps) {
  const editing = useRef(false), [draft, setDraft] = useState(String(value ?? ''));
  return <span class="row" style="gap:6px">
    <input id={id} class="input num-field" type="number" inputmode="numeric" min={min} max={max} value={editing.current ? draft : value ?? ''} onFocus={() => { editing.current = true; setDraft(String(value ?? '')); }} onBlur={() => { editing.current = false; setDraft(String(value ?? '')); }} onInput={(e) => { setDraft(e.currentTarget.value); const v = +e.currentTarget.value; if (e.currentTarget.value !== '' && v >= min && v <= max) onChange(v); }} />
    {unit && <span class="muted">{unit}</span>}
  </span>;
}
export function Section({ title, sub, children, action }: { title: string; sub?: ComponentChildren; children?: ComponentChildren; action?: ComponentChildren }) {
  return <section class="svc-section">
    <div class="section-head">
      <h2>{title}</h2>
      {sub && <span class="faint" style="font-size:var(--text-sm)">{sub}</span>}
      <span class="grow" />
      {action}
    </div>
    {children}
  </section>;
}
const copy = (text: string) => { navigator.clipboard?.writeText(text); act.toast('Copied'); };
function Command({ text }: { text: string }) {
  return <div class="svc-cmd">
    <code class="mono">{text}</code>
    <button type="button" class="icon-btn sm" aria-label="Copy the command" data-tip="Copy" data-tip-side="left" onClick={() => copy(text)}>
      <Icon name="copy" size={13} />
    </button>
  </div>;
}

// ---- machines --------------------------------------------------------------------
function MachineDetail({ s, m, now }: { s: State; m: Machine; now: Ms }) {
  const ags = machineAgents(s, m.id); const runningAgents = ags.filter(running);
  const details = s.machineDetails;
  const repos = s.repos.filter((r) => r.folders[m.id]);
  const trees = s.projects.filter((p) => p.machine === m.id && !p.archived);
  const subs = s.subscriptions.filter((x) => x.machine === m.id);
  return <div class="svc-detail">
    {m.status === 'offline' && <div class="callout warn" style="grid-column:1/-1">
      <Icon name="wifi-off" size={15} />
      <span class="grow">
        {m.name}
        {' stopped answering at '}
        {clock(m.lastSeen)}
        {'. Its agents are paused, and they start again when it reconnects. On '}
        {m.name}
        {', run '}
        <code>lgtm connect</code>
        .
      </span>
    </div>}
    {m.status === 'pairing' && <div class="callout info" style="grid-column:1/-1">
      <Icon name="loader-circle" size={15} class="svc-spin" />
      <span class="grow">{m.name} is connected. LGTM Studio is scanning it for harnesses and repositories.</span>
    </div>}
    <div>
      <h3>This machine</h3>
      <dl class="dl">
        <dt>Workspace</dt>
        <dd class="mono">{live.on ? details?.workspace : m.workspace}</dd>
        <dt>System</dt>
        <dd>{!live.on && <>{m.os} · {m.cores} cores · {m.ram} GB</>}</dd>
        <dt>LGTM Studio</dt>
        <dd>
          {!live.on && m.version}
          {!live.on && m.version !== s.settings.version ? <> <span class="warn-text">· older than the service ({s.settings.version})</span></> : ''}
        </dd>
        {m.connectedFor && m.status === 'online' && <><dt>Connected</dt><dd>{ago(now - m.connectedFor!, now)}</dd></>}
        <dt>Agent limit</dt>
        <dd>
          <Num id={'lim-' + m.id} value={live.on ? '' : m.slots.limit} min={1} max={64} unit="at once" onChange={(v) => update(() => { m.slots.limit = v; })} />
        </dd>
      </dl>
    </div>
    <div>
      <h3>Harnesses</h3>
      <ul class="svc-lines">
        {live.on ? (details?.harnesses || []).map((h: Loose) => <li key={h.id}>
        <Icon name={h.installed ? 'circle-check' : 'circle-alert'} size={14} />
        <b>{HARNESSES[h.id]?.name || h.id}</b>
        <span class="muted">{h.installed ? `Installed · ${h.signedIn} signed in` : 'Not installed'}</span>
      </li>) : Object.keys(HARNESSES).map((h) => { const v = m.harnesses?.[h]; const n = subs.filter((x) => x.harness === h && x.signedIn).length;
        return <>
          <li key={h}>
            <Icon name={v ? 'circle-check' : 'circle-alert'} size={14} class={v ? 'success-text' : 'warn-text'} />
            <b>{HARNESSES[h]!.name}</b>
            {v ? <span class="muted">{v} · {n ? n + ' signed in' : 'not signed in'}</span> : <span class="muted">Not installed</span>}
          </li>
          {!v && <li><Command text={h === 'codex' ? 'npm install -g @openai/codex' : 'npm install -g @anthropic-ai/claude-code'} /></li>}
        </>; })}
      </ul>
      <h3 style="margin-top:var(--space-4)">Repositories</h3>
      <ul class="svc-lines">
        {repos.map((r) => <li key={r.id}>
        <Icon name="folder-git-2" size={14} class="faint" />
        <a class="link mono" href={href('/r/' + r.id)}>{r.name}</a>
        <span class="mono muted truncate">{r.folders[m.id]}</span>
      </li>)}
        {repos.length === 0 && <li class="muted">No repositories</li>}
      </ul>
      <button type="button" class="btn sm" style="margin-top:6px" disabled={m.status !== 'online'} onClick={() => act.openDialog({ type: 'add-repository', machine: m.id })}>
        <Icon name="plus" size={13} />
        Add repository
      </button>
    </div>
    <div>
      <h3>Worktrees and agents</h3>
      {trees.length === 0 && <p class="muted">No projects</p>}
      <ul class="svc-lines">
        {trees.map((p) => { const pa = agentsOfProject(s, p.id); const n = pa.filter(running).length;
        return <li key={p.id}>
          <Icon name="git-branch" size={14} class="faint" />
          <a class="link truncate" href={href(`/p/${p.id}/${homeTab()}`)} title={p.branch}>{p.name}</a>
          <span class="grow" />
          <span class="muted" style="white-space:nowrap">
            {live.on ? s.worktrees.filter((w: Loose) => w.project === p.id || w.id === p.worktreeId).length : 1 + teamsOf(s, p.id).length}
            {' worktrees · '}
            {n}
            {' of '}
            {pa.length}
            {' running'}
          </span>
        </li>; })}
      </ul>
      {runningAgents.length > 0 && <ul class="svc-agents">
        {runningAgents.map((a) => { const st = agentStatus(s, a);
        return <li key={a.id}>
          <a class="row" href={href(routeForAgent(s, a))}>
            <Avatar agent={a} s={s} size="sm" />
            <b>{a.name}</b>
            <span class="muted truncate">{projectOf(s, a.project).name}</span>
          </a>
          <span class="truncate" style="font-size:var(--text-sm)"><StateWord status={st} /></span>
        </li>; })}
      </ul>}
    </div>
  </div>;
}
/** Shows why the server refused a change. */
const said = (e: unknown) => act.toast(server.reason(e), 'error');
function machineAction(scope: server.SessionScope | null, call: (scope: server.SessionScope) => Promise<void>, done?: () => void): void {
  if (!scope?.isCurrent()) return;
  void call(scope).then(() => { if (scope.isCurrent()) done?.(); }, (e) => { if (scope.isCurrent()) said(e); });
}
export function Machines({ s }: ScreenProps) {
  const now = useNow(10_000);
  useEffect(() => { if (live.on) void mocks.loadMachines(); }, [s.boardRevision]);
  const online = s.machines.filter((m) => m.status === 'online').length;
  const remove = (m: Machine) => { const ags = machineAgents(s, m.id).filter(running); const subs = s.subscriptions.filter((x) => x.machine === m.id);
    const scope = server.captureSession();
    act.confirm({ title: `Remove ${m.name}?`, body: `${m.name} is disconnected and forgotten. Its folders stay on its disk. Pair it again to bring it back.`, action: 'Remove machine',
      lost: [ags.length ? `${plural(ags.length, 'running agent')} will stop` : null, subs.length ? `${plural(subs.length, 'subscription')} on it will be removed` : null, s.projects.some((p) => p.machine === m.id && !p.archived) ? 'Projects that run here need another machine' : null].filter(Boolean) as string[],
      onConfirm: () => machineAction(scope, (scope) => server.removeClient(scope, m.id), () => {
        if (!store.state.server.repositoryMode) update((st) => { st.subscriptions = st.subscriptions.filter((x) => x.machine !== m.id);
          for (const a of st.agents) if (projectOf(st, a.project)?.machine === m.id && running(a)) { a.prevState = a.state; a.state = 'stopped'; a.activity = `Stopped · ${m.name} was removed`; } });
        act.toast(`Removed ${m.name}`); }) }); };
  const rename = (m: Machine) => {
    const scope = server.captureSession();
    act.openDialog({ type: 'rename', title: `Rename ${m.name}`, value: m.name, taken: s.machines.map((x) => x.name),
      onRename: (name: string) => machineAction(scope, (scope) => server.renameClient(scope, m.id, name), () => act.toast('Machine renamed')) });
  };
  return <div class="view-body">
    <div class="page">
      <div class="page-head">
        <h1 class="page-title">Machines</h1>
        <span class="sub">{!live.on && <>{online} of {s.machines.length} online</>}</span>
        <span class="grow" />
        <button type="button" class="btn primary" onClick={() => live.on ? refuse() : act.openDialog({ type: 'pair-machine' })}>
          <Icon name="plus" size={14} />
          Add machine
        </button>
      </div>
      <div class="table-wrap" data-main aria-busy={live.on && mocks.busy('read-machines')}>
        <table class="table svc-machines">
          <thead>
            <tr>
              <th>Machine</th>
              <th class="c-mid">Role</th>
              <th>Status</th>
              <th class="c-wide">{live.on ? 'Web port' : 'Address'}</th>
              <th class="c-wide">Harnesses</th>
              <th>Agents</th>
              <th class="c-mid">CPU</th>
              <th class="c-mid">Memory</th>
              <th class="c-mid">Last seen</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {s.machines.filter((m) => !live.on || m.id === 'local').map((m) => { const k = 'machine:' + m.id; const open = !!s.expanded[k]; const off = m.status === 'offline'; const n = machineAgents(s, m.id).filter(running).length;
        const full = n >= m.slots.limit;
        return <>
          <tr key={m.id} class={cx('click', off && 's-warn', off && 'off')} aria-expanded={open} onClick={() => act.toggleExpanded(k)}>
            <td class="state-cell">
              <span class="row" style="gap:8px">
                <button type="button" class={cx('svc-twist', open && 'open')} aria-expanded={open} aria-label={(open ? 'Hide' : 'Show') + ' details for ' + m.name} onClick={(e) => { e.stopPropagation(); act.toggleExpanded(k); }}>
                  <Icon name="chevron-right" size={13} />
                </button>
                <Icon name="server" size={14} class="faint" />
                <b>{m.name}</b>
                {m.here && !live.on && <span class="badge outline not-phone">This computer</span>}
              </span>
            </td>
            <td class="muted c-mid">{live.on ? '' : m.role === 'master' ? 'Master' : 'Worker'}</td>
            <td>
              {!live.on && <span class="row" style="gap:6px">
              <i class={cx('dot', off ? 'waiting' : m.status === 'pairing' ? 'starting' : 'working')} />
              <span class={cx(off && 'warn-text')} style={off ? 'font-weight:var(--weight-medium)' : ''}>
                {off ? 'Offline' : m.status === 'pairing' ? 'Pairing' : 'Online'}
              </span>
            </span>}
            </td>
            <td class="mono muted c-wide">{live.on ? s.machineDetails?.port ?? '' : m.address}</td>
            <td class="c-wide">
              {live.on ? (s.machineDetails?.harnesses || []).filter((h: Loose) => h.installed).map((h: Loose) => <span key={h.id} class="tag" style="margin-right:4px">{HARNESSES[h.id]?.name || h.id}</span>) : <>
              {Object.keys(HARNESSES).map((h) => m.harnesses?.[h] ? <span key={h} class="tag" style="margin-right:4px">{HARNESSES[h]!.name}</span> : null)}
              {m.harnesses && !m.harnesses.codex && <span class="warn-text" style="font-size:var(--text-sm)">Codex missing</span>}
              {!m.harnesses && <span class="muted">Scanning…</span>}
            </>}
            </td>
            <td class="num" style="text-align:left">
              <span class={cx(!live.on && full && 'warn-text')}>{n}</span>
              {!live.on && <span class="muted"> of {m.slots.limit}</span>}
            </td>
            <td class="c-mid">{off || m.status === 'pairing' ? <span class="faint">—</span> : <Meter pct={m.cpu} />}</td>
            <td class="c-mid">{off || m.status === 'pairing' ? <span class="faint">—</span> : <Meter pct={m.mem} />}</td>
            <td class={cx('c-mid', off ? 'warn-text' : 'muted')}>{live.on ? '' : off ? ago(m.lastSeen, now) + ' ago' : 'Now'}</td>
            <td class="svc-acts" onClick={(e) => e.stopPropagation()}>
              <Menu class="icon-btn sm" label={'Actions for ' + m.name} align="end" width={230} trigger={<Icon name="ellipsis" size={15} />} items={[
                { label: 'Rename', icon: 'pencil', onSelect: () => rename(m) },
                { label: 'Scan harnesses again', icon: 'scan-search', disabled: off || live.on && mocks.busy('read-machines'), why: off ? `${m.name} is offline.` : null,
                  onSelect: () => live.on ? mocks.loadMachines() : machineAction(server.captureSession(), (scope) => server.scanClientHarnesses(scope, m.id), () => act.toast(`Scanned ${m.name}. Nothing changed.`)) },
                { label: 'Add repository', icon: 'folder-git-2', disabled: off, why: off ? `${m.name} is offline.` : null, onSelect: () => act.openDialog({ type: 'add-repository', machine: m.id }) },
                off && { label: 'Try to reconnect', icon: 'refresh-cw', onSelect: () => { if (act.refused()) return; act.toast(`Calling ${m.name}…`); machineAction(server.captureSession(), (scope) => server.reconnectClient(scope, m.id)); } },
                { sep: true },
                { label: 'Remove machine', icon: 'trash-2', danger: true, disabled: !live.on && m.role === 'master', why: !live.on && m.role === 'master' ? 'The master runs the service and cannot be removed.' : null, onSelect: () => live.on ? refuse() : remove(m) },
              ].filter(Boolean)} />
            </td>
          </tr>
          {open && <tr key={m.id + '-d'} class="svc-detail-row"><td colspan={10}><MachineDetail s={s} m={m} now={now} /></td></tr>}
        </>; })}
          </tbody>
        </table>
      </div>
      {live.on && <p class="svc-foot-note">The service accepts only the local machine.</p>}
    </div>
  </div>;
}

function PairMachine({ s, d }: ScreenProps & { d: DialogState }) {
  // The server makes the token. A client uses it once, and then waits here for the owner's approval.
  const [token, setToken] = useState<server.ClientToken | null>(null); const [failed, setFailed] = useState('');
  const now = useNow(1000); const [typed, setTyped] = useState<string | null>(null); const [busy, setBusy] = useState(false);
  const scope = server.captureSession();
  useEffect(() => {
    if (!scope?.isCurrent()) return;
    let closed = false;
    let made: string | null = null;
    setToken(null); setFailed(''); setTyped(null); setBusy(false);
    server.createClientToken(scope).then((t) => {
      if (!scope.isCurrent()) return;
      if (closed || store.state.dialog !== d) void server.revokeClientToken(scope, t.id);
      else { made = t.id; setToken(t); }
    }, (e) => { if (!closed && scope.isCurrent() && store.state.dialog === d) setFailed(server.reason(e)); });
    // A token that is still open when the dialog closes is ended. An approved client has used its token up.
    return () => { closed = true; if (made && scope.isCurrent()) void server.revokeClientToken(scope, made); };
  }, [d, scope]);
  const found = token ? s.joining.find((j) => j.tokenId === token.id) : undefined;
  const step = found ? 1 : 0; const code = token?.secret || '';
  // The first part of the host's name is offered as the machine's name.
  const offered = found ? found.host.split('.')[0]!.replace(/^./, (c) => c.toUpperCase()) : '';
  const name = typed ?? offered; const setName = setTyped;
  const left = token ? Math.max(0, token.expiresAt - now) : 0; const mm = Math.floor(left / MIN), ss = Math.floor((left % MIN) / 1000);
  const taken = s.machines.some((m) => m.name.toLowerCase() === name.trim().toLowerCase());
  const confirm = () => {
    if (!found || !name.trim() || taken || busy) return;
    if (!scope?.isCurrent() || store.state.dialog !== d) return;
    const current = () => scope.isCurrent() && store.state.dialog === d;
    setBusy(true);
    server.approveClient(scope, found.id, name.trim()).then(() => {
      if (!current()) return;
      const { machines, joining, server: selected } = store.state;
      const watched = machines.some((m) => m.id === found.id);
      if (selected.repositoryMode && !watched && !joining.some((j) => j.id === found.id)) { act.closeDialog(); return; }
      update((st) => { st.expanded['machine:' + found.id] = true; });
      act.closeDialog(); act.toast(`${name.trim()} is paired`);
    }, (e) => { if (current()) { setBusy(false); setFailed(server.reason(e)); } });
  };
  return <Dialog title="Add a machine" onClose={act.closeDialog} onSubmit={confirm} foot={<>
      <span class="why">
        {failed || (step === 0 ? `The code works once and expires in ${mm}:${String(ss).padStart(2, '0')}.` : taken ? 'A machine already has that name.' : 'Confirm only a machine you started yourself.')}
      </span>
      <button type="button" class="btn" onClick={act.closeDialog}>Cancel</button>
      <button type="submit" class="btn primary" disabled={step === 0 || !name.trim() || taken || busy}>
        {'Confirm '}
        {step === 1 && name.trim() ? name.trim() : 'machine'}
      </button>
    </>}>
    <ol class="svc-steps">
      <li class="done">
        <span class="n">1</span>
        <div><b>Install LGTM Studio on the other machine.</b>
          {/* The Go client has no published installer yet. It is built from a checkout of the repository. */}
          <Command text="go build -o ~/.local/bin/lgtm-client ./src/client/cmd/lgtm-client" /></div>
      </li>
      <li class={step > 0 ? 'done' : 'now'}>
        <span class="n">2</span>
        <div>
          <b>Run this command there.</b>
          {/* A sample server gives no address a client could reach, so the page shows an example. */}
          <Command text={`lgtm-client join ${token?.address || 'https://workstation.lan:8787'} ${code}`} />
        </div>
      </li>
      <li class={step > 0 ? 'now' : ''}>
        <span class="n">3</span>
        <div>
          <b>Confirm the machine here.</b>
          {step === 0 ? <p class="row muted"><Icon name="loader-circle" size={14} class="svc-spin" />Waiting for a machine to use the code…</p>
          : <>
            <div class="svc-found">
              <Icon name="server" size={16} />
              <div class="grow">
                <b>{found?.host}</b>
                <span class="muted"> wants to connect</span>
                <div class="mono muted" style="font-size:var(--text-sm)">{found && `${found.address} · ${found.os} · ${found.cores} cores · ${found.ram} GB`}</div>
              </div>
              <span class="badge success">Code matched</span>
            </div>
            <div class="field" style="margin-top:8px">
              <label for="pair-name">Name</label>
              <input id="pair-name" class="input" value={name} onInput={(e) => setName(e.currentTarget.value)} autofocus />
            </div>
          </>}
        </div>
      </li>
    </ol>
  </Dialog>;
}
registerDialog('pair-machine', PairMachine);

// ---- add repository ------------------------------------------------------------
function AddRepository({ s, d }: DialogProps) {
  const known = d.repo ? byId(s.repos, d.repo) : null;
  const [machine, setMachine] = useState(live.on ? 'local' : d.machine || 'studio'); const m = machineOf(s, machine);
  const [path, setPath] = useState(() => `${machineOf(s, live.on ? 'local' : d.machine || 'studio').workspace}/${known ? known.name : ''}`);
  const [found, setFound] = useState<{ main: string; remote?: string | null } | null>(null);
  useEffect(() => { if (!known) return; setPath(live.on ? known.folders[machine] || '' : `${m.workspace}/${known.name}`); }, [machine]);
  const name = path.replace(/\/+$/, '').split('/').pop() || '';
  const valid = /^(~|\/)/.test(path.trim()) && name && name !== '~' && path.trim() !== m.workspace;
  useEffect(() => {
    setFound(null); if (live.on || !valid) return;
    const t = setTimeout(() => setFound(known ? { main: known.main, remote: known.remote } : { main: 'main', remote: `github.com/acme/${name}` }), 700);
    return () => clearTimeout(t);
  }, [path, machine]);
  const id = act.slug(name); const dup = !known && s.repos.some((r) => r.id === id);
  const here = known ? known.folders[machine] : dup && byId(s.repos, id)!.folders[machine];
  const why = m.status !== 'online' ? `${m.name} is offline.` : !valid ? 'Give the folder of a git repository.' : here ? `${name} is already on ${m.name}.` : !live.on && !found ? 'Looking at the folder…' : null;
  const add = () => {
    if (why) return;
    if (live.on) return board.addRepository({ path, name, repo: known?.id, back: d.back });
    update((st) => { const r = known || (dup && byId(st.repos, id));
      if (r) r.folders[machine] = path.trim();
      else st.repos.push({ id, name, main: found!.main, remote: found!.remote, folders: { [machine]: path.trim() } });
      st.expanded['repo:' + (r ? r.id : id)] = true; });
    act.toast(`Added ${name} on ${m.name}`);
    if (d.back) act.openDialog({ ...d.back, machine, repo: known ? known.id : id }); else act.closeDialog();
  };
  const close = () => (d.back ? act.openDialog(d.back) : act.closeDialog());
  return <Dialog title={known ? `Add ${known.name} to a machine` : 'Add repository'} onClose={close} onSubmit={add} foot={<>
      {why && <span class="why">{why}</span>}
      <button type="button" class="btn" onClick={close}>{d.back ? 'Back' : 'Cancel'}</button>
      <button type="submit" class="btn primary" disabled={!!why || (live.on && board.busy('add-repository'))} aria-busy={live.on && board.busy('add-repository')}>
        {live.on && board.busy('add-repository') ? 'Adding…' : 'Add repository'}
      </button>
    </>}>
    <div class="field">
      <label for="ar-machine">Machine</label>
      <select id="ar-machine" class="select" value={machine} onChange={(e) => setMachine(e.currentTarget.value)}>
        {s.machines.filter((x) => !live.on || x.id === 'local').map((x) => <option value={x.id} disabled={x.status !== 'online'}>
          {x.name}
          {x.status !== 'online' ? ' (offline)' : x.here ? ' (this computer)' : ''}
        </option>)}
      </select>
    </div>
    <div class="field">
      <label for="ar-path">Folder on {m.name}</label>
      <input id="ar-path" class="input mono" value={path} onInput={(e) => setPath(e.currentTarget.value)} placeholder={m.workspace + '/my-repository'} autofocus spellcheck={'false' as unknown as boolean} />
    </div>
    <dl class="dl svc-detected">
      <dt>Base branch</dt>
      <dd class="mono">{found ? found.main : <span class="faint">{live.on ? '' : valid ? 'Detecting…' : '—'}</span>}</dd>
      <dt>Remote</dt>
      <dd class="mono" style="overflow-wrap:anywhere">
        {found ? (found.remote || 'No remote') : <span class="faint">{live.on ? '' : valid ? 'Detecting…' : '—'}</span>}
      </dd>
      <dt>Status</dt>
      <dd>
        {found ? <span class="row success-text"><Icon name="circle-check" size={14} />A git repository with a clean working tree</span> : <span class="faint">—</span>}
      </dd>
    </dl>
  </Dialog>;
}
registerDialog('add-repository', AddRepository);

// ---- a repository the server tracks -------------------------------------------------
// The owner decided on 2026-11-24 that the plus beside "Repositories" adds a repository. The server
// only tracks a repository by its link. "Set up" clones it on one machine and starts its repository
// agent. The repositories of a connected GitHub account can be added from a list.
/** The host, the path and the name of an HTTPS or SSH link to a repository, or null. */
function parseLink(link: string): { remote: string; name: string } | null {
  const m = /^(?:https?:\/\/|ssh:\/\/(?:[\w.-]+@)?|[\w.-]+@)([\w.-]+)[:/]+([\w.~/-]+?)(?:\.git)?\/?$/.exec(link.trim());
  const name = m?.[2]?.split('/').pop();
  return m && name ? { remote: `${m[1]}/${m[2]}`, name } : null;
}
interface GitHubRepo { name: string; days: number; priv?: boolean }
const G = (name: string, days: number, priv = false): GitHubRepo => ({ name, days, priv });
const GITHUB: Record<string, { kind: string; all: boolean; repos: GitHubRepo[] }> = {
  mayaokafor: { kind: 'Personal', all: true, repos: [G('lgtm-studio', 0), G('field-notes', 3, true), G('recipe-box', 19, true), G('dotfiles', 41), G('advent-of-code', 340)] },
  acme: { kind: 'Organization', all: false, repos: [G('billing-api', 1, true), G('atlas-docs', 2), G('storefront', 2, true), G('payments-gateway', 6, true), G('design-tokens', 12), G('infra', 30, true), G('status-page', 75)] },
};
/** The GitHub accounts the server's GitHub App is installed on. */
const accountsOf = (s: State): string[] => (s.ui.github || []).filter((a) => GITHUB[a]);
const connectGitHub = (account?: string) => update((st) => {
  const have = (st.ui.github || []).filter((a) => GITHUB[a]); const next = account || Object.keys(GITHUB).find((a) => !have.includes(a));
  if (next && !have.includes(next)) { st.ui.github = [...have, next]; act.toast(`GitHub connected to ${next}`); }
});
// The dialog has one field, after the "New project" page of Vercel: it searches the repositories
// of the connected accounts, and a pasted link becomes the first row of the list.
function TrackRepository({ s }: DialogProps) {
  const accounts = accountsOf(s); const [q, setQ] = useState(''); const [pick, setPick] = useState<string | null>(null);
  const account = pick && accounts.includes(pick) ? pick : accounts[0]; const now = Date.now();
  const parsed = parseLink(q); const looksLikeLink = /[:/@]/.test(q.trim()); const word = looksLikeLink ? '' : q.trim().toLowerCase();
  const track = (remote: string, name: string) => {
    const id = act.slug(name); const known = s.repos.find((r) => r.id === id || r.remote === remote);
    if (!known) update((st) => { st.repos.push({ id, name, main: 'main', remote, folders: {} }); });
    act.closeDialog(); go('/r/' + (known ? known.id : id));
  };
  const list = account ? GITHUB[account]!.repos.filter((r) => r.name.includes(word)) : [];
  const more = Object.keys(GITHUB).some((a) => !accounts.includes(a));
  return <Dialog title="Add a repository" size="track" onClose={act.closeDialog} onSubmit={() => { if (parsed) track(parsed.remote, parsed.name); }} foot={<>
      <a class="btn ghost" style="margin-right:auto" href={href('/settings?section=connections')} onClick={act.closeDialog}><Icon name="settings" size={14} />Connections</a>
      <button type="button" class="btn ghost" onClick={() => act.openDialog({ type: 'add-repository' })}>Use a folder on a machine</button>
      <button type="button" class="btn" onClick={act.closeDialog}>Close</button>
    </>}>
    <div class="tr-find">
      <Icon name="search" size={15} class="faint" />
      <input id="tr-link" class="input" autocomplete="off" spellcheck={false} aria-label="Search the repositories, or paste an HTTPS or SSH link" placeholder="Search, or paste an HTTPS or SSH link" value={q} onInput={(e) => setQ(e.currentTarget.value)} aria-invalid={looksLikeLink && !parsed} aria-describedby={looksLikeLink && !parsed ? 'tr-link-err' : undefined} />
    </div>
    {looksLikeLink && !parsed && <span class="error" id="tr-link-err">This is not an HTTPS or SSH link to a repository.</span>}
    {parsed && <ul class="tr-list">
      <li>
        <Icon name="link" size={14} class="faint" />
        <span class="mono truncate">{parsed.remote}</span>
        <span />
        {s.repos.some((r) => r.remote === parsed.remote) ? <span class="faint tr-added">Added</span> : <button type="submit" class="btn sm primary">Add</button>}
      </li>
    </ul>}
    {!parsed && accounts.length === 0 && <div class="tr-off">
      <Icon name="folder-git-2" size={20} class="faint" />
      <strong>GitHub is not connected</strong>
      <button type="button" class="btn primary" onClick={() => connectGitHub()}>Connect GitHub</button>
    </div>}
    {!parsed && accounts.length > 0 && <>
      <div class="tr-accounts">
        <div class="seg" role="group" aria-label="GitHub account">
          {accounts.map((a) => <button type="button" key={a} aria-pressed={a === account} onClick={() => setPick(a)}>{a}</button>)}
        </div>
        {more && <button type="button" class="icon-btn" aria-label="Connect another account" data-tip="Connect another account" onClick={() => connectGitHub()}><Icon name="plus" size={15} /></button>}
        <span class="grow" />
        <span class="muted num">{plural(list.length, 'repository', 'repositories')}</span>
      </div>
      <ul class="tr-list" data-main>
        {list.length === 0 && <li class="muted" data-empty>No repository matches</li>}
        {list.map((r) => { const remote = `github.com/${account}/${r.name}`; const known = s.repos.find((x) => x.remote === remote);
          return <li key={r.name}>
            <Icon name={r.priv ? 'lock' : 'folder-git-2'} size={14} class="faint" aria-label={r.priv ? 'Private' : 'Public'} />
            <span class="mono truncate">{r.name}</span>
            <span class="muted num">{r.days ? ago(now - r.days * DAY, now) + ' ago' : 'today'}</span>
            {known ? <a class="link tr-added" href={href('/r/' + known.id)} onClick={act.closeDialog}>Open</a> : <button type="button" class="btn sm" aria-label={`Add ${r.name}`} onClick={() => track(remote, r.name)}>Add</button>}
          </li>; })}
      </ul>
    </>}
  </Dialog>;
}
registerDialog('track-repository', TrackRepository);
/** The page of a repository that is on no machine yet. */
export function RepoSetup({ s, repo: r }: { s: State; repo: Loose & { id: Id; name: string; remote?: string | null } }) {
  const [machine, setMachine] = useState(() => (s.machines.find((m) => m.status === 'online') || s.machines[0])?.id || '');
  const m = byId(s.machines, machine); const busy = s.ui.settingUp?.[r.id]; const on = byId(s.machines, busy || machine);
  useEffect(() => { document.title = `${r.name} · LGTM Studio`; return () => { document.title = 'LGTM Studio'; }; }, [r.name]);
  const start = () => {
    if (!m || m.status !== 'online' || busy) return;
    update((st) => { st.ui.settingUp = { ...(st.ui.settingUp || {}), [r.id]: m.id }; });
    act.sampleLater(() => update((st) => {
      const x = byId(st.repos, r.id); if (!x) return;
      const folder = `${m.workspace}/${x.name}`; const aid = x.id + '-repo';
      x.folders = { [m.id]: folder }; x.agent = aid; st.expanded['repo:' + x.id] = true;
      st.agents.push({ id: aid, project: 'repository:' + x.id, team: null, role: 'repository-agent', name: 'Repository agent', harness: 'claude', subscription: 'sub-max', model: 'Opus 5.5', effort: 'Medium', permission: 'Edit files without asking',
        state: 'working', activity: 'Reading the repository', task: null, context: 3, turns: 1, tokens: 0.1, workState: 'working', stalled: false, startedAt: Date.now() });
      st.threads[aid] = [{ id: uid('m'), type: 'agent', ts: Date.now(), text: `I cloned **${x.name}** into \`${folder}\` on ${m.name}, and I am reading it now.\n\nTell me what you want from this repository. I keep it as a plan, and I make a project for each goal.` }];
      delete st.ui.settingUp![r.id];
    }), 2600);
  };
  const remove = () => act.confirm({ title: `Stop tracking ${r.name}?`, action: 'Stop tracking', body: 'The repository leaves the list. Nothing was cloned, so nothing is deleted.', lost: [], onConfirm: () => { update((st) => { st.repos = st.repos.filter((x) => x.id !== r.id); }); go('/home'); } });
  return <div class="view-body">
    <div class="page rs" data-main>
      <h1 class="mono">{r.name}</h1>
      <dl class="rs-facts">
        <dt>Link</dt><dd class="mono">{r.remote}</dd>
        <dt>State</dt><dd>{busy ? `Cloning on ${on?.name}…` : 'Tracked. On no machine.'}</dd>
        <dt>Folder</dt><dd class="mono">{on ? `${on.workspace}/${r.name}` : '—'}</dd>
      </dl>
      <div class="rs-act">
        <select class="select" aria-label="Machine" value={busy || machine} disabled={!!busy} onChange={(e) => setMachine(e.currentTarget.value)}>
          {s.machines.map((x) => <option key={x.id} value={x.id} disabled={x.status !== 'online'}>{x.name}{x.status !== 'online' ? ' (offline)' : ''}</option>)}
        </select>
        <button type="button" class="btn primary" disabled={!!busy || !m || m.status !== 'online'} aria-busy={!!busy} onClick={start}>{busy ? 'Setting up…' : `Set up on ${m?.name || 'a machine'}`}</button>
        <span class="grow" />
        <button type="button" class="btn ghost" disabled={!!busy} onClick={remove}>Stop tracking</button>
      </div>
    </div>
  </div>;
}

// ---- subscriptions ---------------------------------------------------------------
function subState(x: Subscription): { word: string; tone: string | null; dot: string } {
  if (x.status === 'out-of-usage') return { word: 'Limit reached', tone: 'error', dot: 'failed' };
  if (!usableSubscription(x)) return { word: 'Signed out', tone: 'warn', dot: 'waiting' };
  if (!x.enabled) return { word: 'Turned off', tone: null, dot: 'idle' };
  if (x.window && x.window.pct >= 100) return { word: 'Limit reached', tone: 'error', dot: 'failed' };
  if (x.usage && x.usage.pct >= 100) return { word: 'Weekly limit reached', tone: 'error', dot: 'failed' };
  if ((x.window && x.window.pct >= 80) || (x.usage && x.usage.pct >= 80)) return { word: 'Near its limit', tone: 'warn', dot: 'waiting' };
  if (x.signInKnown === false) return { word: 'Not reported', tone: null, dot: 'idle' };
  return { word: 'Signed in', tone: null, dot: 'working' };
}
function SubRow({ s, x, now }: { s: State; x: Subscription; now: Ms }) {
  const st = subState(x); const m = machineOf(s, x.machine) || { name: '' }; const h = HARNESSES[x.harness]; const ags = subAgents(s, x.id); const active = ags.filter(running).length;
  const k = 'sub:' + x.id; const open = !!s.expanded[k];
  const makeDefault = () => { if (live.on) return liveGit.makeDefault(x.id); update((stt) => { for (const y of stt.subscriptions) if (y.harness === x.harness && y.machine === x.machine) y.useForNew = y.id === x.id; }); act.toast(`New ${h.name} agents on ${m.name} use ${x.name}`); };
  const signOut = () => live.on ? refuse() : act.confirm({ title: `Sign out of ${h.name} · ${x.name}?`, body: `The sign-in on ${m.name} is removed. Sign in again at any time.`, action: 'Sign out',
    lost: ags.length ? [`${plural(ags.length, 'agent')} ${ags.length === 1 ? 'uses' : 'use'} it and will pause at the next turn`] : [],
    onConfirm: () => { update(() => { x.signInKnown = true; x.signedIn = false; x.usage = null; x.window = null; x.useForNew = false; }); act.toast(`Signed out of ${h.name} · ${x.name}`); } });
  const remove = () => act.confirm({ title: `Remove ${h.name} · ${x.name}?`, body: live.on ? 'The account is removed. Its sign-in and running agents are kept.' : 'The subscription and its sign-in are removed from LGTM Studio.', action: 'Remove subscription',
    lost: ags.length ? [`${plural(ags.length, 'agent')} will need another subscription`] : [],
    onConfirm: () => { if (live.on) return liveGit.accountAction(x.id, 'remove'); update((stt) => { stt.subscriptions = stt.subscriptions.filter((y) => y.id !== x.id); }); act.toast('Subscription removed'); } });
  const signIn = () => { if (live.on) return act.openDialog({ type: 'harness-signin', account: x.id }); update(() => { x.signInKnown = true; x.signedIn = true; x.account = 'owner@example.com'; }); act.toast(`Signed in to ${h.name} · ${x.name}`); };
  return <div class={cx('svc-sub', st.tone === 'warn' && 'warn', st.tone === 'error' && 'error', !x.enabled && 'dim')}>
    <div class="svc-sub-row">
      <div class="c-name">
        <button type="button" class={cx('svc-twist', open && 'open')} aria-expanded={open} aria-label={(open ? 'Hide' : 'Show') + ' agents on ' + x.name} onClick={() => act.toggleExpanded(k)}>
          <Icon name="chevron-right" size={13} />
        </button>
        <div class="col">
          <span class="row" style="gap:6px">
            <b>{x.name}</b>
            {x.useForNew && usableSubscription(x) && <span class="badge outline" data-tip={`New ${h.name} agents on ${m.name} use it`}>Default</span>}
          </span>
          <span class="muted truncate" style="font-size:var(--text-sm)">{live.on ? '' : (x.account || 'No account') + ' · ' + m.name}</span>
        </div>
      </div>
      <div class="c-state">
        <span class="row" style="gap:6px">
          <i class={cx('dot', st.dot)} />
          <span class={cx(st.tone === 'warn' && 'warn-text', st.tone === 'error' && 'error-text')} style={st.tone ? 'font-weight:var(--weight-medium)' : ''}>
            {st.word}
          </span>
        </span>
      </div>
      <div class="c-use" data-label="5 hours">
        {live.on && s.accountLimits?.[x.id] ? <small class="muted">Resets in {until(s.accountLimits[x.id], now)}</small> : x.window ? <>
        <Meter pct={x.window.pct} wide />
        <small class={cx(x.window.pct >= 100 ? 'error-text' : 'muted')}>Resets in {until(x.window.resetsAt, now)}</small>
      </> : <span class="faint">Not reported</span>}
      </div>
      <div class="c-use" data-label="This week">
        {x.usage ? <>
        <Meter pct={x.usage.pct} wide />
        <small class="muted">
          {'Resets '}
          {x.usage.resetsAt - now < DAY ? 'in ' + until(x.usage.resetsAt, now) : new Date(x.usage.resetsAt).toLocaleDateString('en-GB', { weekday: 'long' })}
        </small>
      </> : <span class="faint">Not reported</span>}
      </div>
      <div class="c-agents" data-label="Agents">
        {ags.length ? <>
        <button type="button" class="link" onClick={() => act.toggleExpanded(k, true)}>{ags.length}</button>
        <span class="muted"> · {active} running</span>
      </> : <span class="faint">None</span>}
      </div>
      <div class="c-model" data-label="Model for new agents">
        <select class="select" aria-label={'Model for new agents on ' + x.name} value={x.model} disabled={!usableSubscription(x)} onChange={(e) => { const v = e.currentTarget.value; if (live.on) return liveGit.accountModel(x.harness, v); update(() => { x.model = v; }); markSaved(); }}>
          {live.on && !x.model && <option value="" />}
          {h.models.map((mo) => <option value={mo}>{mo}</option>)}
        </select>
      </div>
      <div class="c-act">
        {!usableSubscription(x) ? <button type="button" class="btn sm primary" onClick={signIn}>Sign in</button> : null}
        <Menu class="icon-btn sm" label={'Actions for ' + x.name} align="end" width={250} trigger={<Icon name="ellipsis" size={15} />} items={[
            usableSubscription(x) && { label: live.on ? 'Use for new project leads' : `Use for new agents on ${m.name}`, icon: 'star', checked: x.useForNew, disabled: !x.enabled, onSelect: makeDefault },
            { label: x.enabled ? 'Turn off' : 'Turn on', icon: x.enabled ? 'pause' : 'play', hint: x.enabled ? 'No agent starts on it. Running agents finish their turn.' : null, onSelect: () => { if (live.on) return liveGit.accountAction(x.id, x.enabled ? 'disable' : 'set', { enabled: true }); update(() => { x.enabled = !x.enabled; if (!x.enabled) x.useForNew = false; }); act.toast(x.enabled ? 'Subscription turned on' : 'Subscription turned off'); } },
            { label: 'Rename', icon: 'pencil', onSelect: () => act.openDialog({ type: 'rename', title: 'Rename subscription', value: x.name, taken: s.subscriptions.filter((y) => y.harness === x.harness).map((y) => y.name), onRename: (v: string) => { if (live.on) return liveGit.accountAction(x.id, 'set', { displayName: v }); update(() => { x.name = v; }); act.toast('Subscription renamed'); } }) },
            { label: 'Check sign-in again', icon: 'refresh-cw', onSelect: () => act.openDialog({ type: 'harness-signin', account: x.id }) },
            live.on && { label: 'Account works again', icon: 'play', onSelect: () => liveGit.markAvailable(x.id) },
            { sep: true },
            usableSubscription(x) ? { label: 'Sign out', icon: 'log-out', onSelect: signOut } : { label: 'Sign in', icon: 'log-in', onSelect: signIn },
            { label: 'Remove', icon: 'trash-2', danger: true, onSelect: remove },
          ].filter(Boolean)} />
      </div>
    </div>
    {!live.on && st.word === 'Limit reached' && <div class="svc-sub-note error-text">
      <Icon name="circle-alert" size={14} />
      <span>
        {'The five-hour limit is used up. Agents on this subscription wait, and try again every '}
        {s.settings.capacityBackoff}
        {' minutes until '}
        {clock(x.window!.resetsAt)}
        .
      </span>
    </div>}
    {!x.signedIn && x.signInKnown !== false && <div class="svc-sub-note warn-text">
      <Icon name="triangle-alert" size={14} />
      <span>
        {'No sign-in on '}
        {m.name}
        {'. '}
        {ags.length ? `${plural(ags.length, 'agent')} cannot start until you sign in.` : 'No agent can start on it.'}
      </span>
    </div>}
    {open && <div class="svc-sub-agents">
      {ags.length === 0 ? <span class="muted">No agent uses this subscription.</span> : ags.map((a) => { const as = agentStatus(s, a);
      return <a key={a.id} class="list-row" href={href(routeForAgent(s, a))}>
        <Avatar agent={a} s={s} size="sm" />
        <b style="font-weight:var(--weight-medium)">{a.name}</b>
        <span class="muted truncate grow">{projectOf(s, a.project).name}</span>
        <span class="muted not-phone">{a.model} · {a.effort}</span>
        <span style="min-width:110px;text-align:right;font-size:var(--text-sm)"><StateWord status={as} /></span>
      </a>; })}
    </div>}
  </div>;
}
// This page keeps the account rows and the harness defaults together.
function watchedHarnesses(s: State): [string, Harness][] {
  const harnesses = new Map<string, Harness>();
  for (const machine of s.machines) {
    for (const [id, version] of Object.entries(machine.harnesses ?? {})) {
      const existing = harnesses.get(id);
      if (existing) {
        existing.version ??= version;
        continue;
      }
      harnesses.set(id, {
        name: s.harnessCatalog[id]?.name || SUPPORTED[id]?.name || id,
        installed: true, version, models: [], efforts: [], noEffort: [], runMode: '',
      });
    }
  }
  return [...harnesses];
}

export function Harnesses({ s }: ScreenProps) {
  useEffect(() => liveGit.openScreen('harnesses'), []);
  const now = useNow(30_000);
  const ready = !live.on || s.ui.serviceReady;
  const harnesses = ready ? s.server.repositoryMode ? watchedHarnesses(s) : Object.entries(s.harnessCatalog) : [];
  const unknown = s.subscriptions.filter((x) => x.signInKnown === false).length;
  const signed = s.subscriptions.filter((x) => x.signedIn).length, limited = s.subscriptions.filter((x) => /limit reached/i.test(subState(x).word)).length;
  return <div class="view-body">
    <div class="page">
      <div class="page-head">
        <h1 class="page-title">Harnesses</h1>
        <Saved s={s} />
        <span class="grow" />
        <AddHarness s={s} />
        <button type="button" class="btn" disabled={s.server.repositoryMode || live.on && liveGit.busy('read-service')} onClick={() => live.on ? liveGit.checkHarnesses() : act.toast('Sample harnesses checked.')}>
          <Icon name="refresh-cw" size={14} />
          Check again
        </button>
      </div>
      <p class="muted">
        {s.subscriptions.length === unknown ? `${unknown} Not reported${limited ? `, ${limited} at its limit` : ''}` : `${signed} of ${s.subscriptions.length - unknown} signed in${unknown ? `, ${unknown} Not reported` : ''}, ${limited} at its limit`}
      </p>
      {ready && <nav class="svc-jump" aria-label="Harnesses on this page">
      {harnesses.map(([hid, h]) => { const n = s.subscriptions.filter((x) => x.harness === hid).length;
      return <a key={hid} class="svc-jump-item" href={'#harness-' + hid} onClick={(e) => { e.preventDefault(); document.getElementById('harness-' + hid)?.scrollIntoView({ behavior: 'smooth', block: 'start' }); }}>
        <i class={cx('dot', !h.installed ? 'waiting' : n ? 'working' : 'idle')} />
        <b>{h.name}</b>
        <span class="muted">{!h.installed ? 'Not installed' : n ? plural(n, 'account') : 'No account'}</span>
      </a>; })}
    </nav>}
      {live.on && (liveGit.busy('subscriptions') || liveGit.busy('settings')) && <span role="status">Saving…</span>}
      <fieldset disabled={s.server.repositoryMode || live.on && liveGit.busy('subscriptions')} style="border:0;padding:0;margin:0;min-width:0" aria-busy={live.on && liveGit.busy('read-service')}>
        <div data-main>
          {ready && <div class="svc-card svc-card-defaults"><Section title="Defaults for new project leads"><HarnessDefaults s={s} /></Section></div>}
          {harnesses.map(([hid, h]) => { const list = s.subscriptions.filter((x) => x.harness === hid);
      const save = (patch: Partial<Harness>) => { if (live.on) return liveGit.harnessSetting(hid, patch); update(() => Object.assign(h, patch)); markSaved(); };
      return <section key={hid} id={'harness-' + hid} class="svc-harness svc-card" aria-label={h.name}>
        <div class="section-head svc-card-head">
          <h2>{h.name}</h2>
          <span class={cx('badge outline', !h.installed && 'warn')}>
            {h.installed ? 'Installed' + (h.version ? ' · ' + h.version : '') : 'Not installed'}
          </span>
          <span class="grow" />
          <button type="button" class="btn sm" onClick={() => act.openDialog({ type: 'add-subscription', harness: hid })}>
            <Icon name="plus" size={13} />
            Add account
          </button>
        </div>
        <div class="svc-seg">
          <h3 class="svc-seg-title">Program</h3>
          {h.installationReason ? <p>{h.installationReason}</p> : <p class="muted">{h.installed ? 'Installed on this machine.' : 'Not installed on this machine.'}</p>}
          {h.autoPermissionReason && <p class="muted">{h.autoPermissionReason}</p>}
          {h.path && <details class="svc-path"><summary>Install location</summary><p class="mono muted">{h.path}</p></details>}
        </div>
        <div class="svc-seg">
          <h3 class="svc-seg-title">Defaults for this harness</h3>
          <div class="svc-rows svc-trio">
            <Row label="Run mode" id={'hm-' + hid}>
              <select id={'hm-' + hid} class="select" value={h.runMode?.toLowerCase()} onChange={(e) => save({ runMode: e.currentTarget.value })}>
                <option value="acp">Agent Client Protocol (ACP)</option>
                <option value="interactive">Interactive</option>
              </select>
            </Row>
            <Row label="Default model" id={'hmodel-' + hid}>
              {h.models.length ? <select id={'hmodel-' + hid} class="select" value={h.defaultModel || ''} onChange={(e) => save({ defaultModel: e.currentTarget.value })}>
            <option value="">Harness default</option>
            {h.models.map((m) => <option value={m}>{h.modelLabels?.[m] || m}</option>)}
          </select> : <span class="muted">{h.reason || 'This harness did not report model choices.'}</span>}
            </Row>
            <Row label="Default effort" id={'heffort-' + hid}>
              {h.efforts.length ? <select id={'heffort-' + hid} class="select" value={h.defaultEffort || ''} onChange={(e) => save({ defaultEffort: e.currentTarget.value })}>
            <option value="">Harness default</option>
            {h.efforts.map((e) => <option value={e}>{h.effortLabels?.[e] || e}</option>)}
          </select> : <span class="muted">{h.effortReason || 'This harness did not report effort choices.'}</span>}
            </Row>
          </div>
        </div>
        <div class="svc-seg">
          <h3 class="svc-seg-title">Accounts</h3>
          <div class="svc-subs">
            <div class="svc-sub-row svc-sub-head" aria-hidden="true">
              <div class="c-name">Account</div>
              <div>Status</div>
              <div>5 hours</div>
              <div>This week</div>
              <div>Agents</div>
              <div>Model for new agents</div>
              <div />
            </div>
            {list.map((x) => <SubRow key={x.id} s={s} x={x} now={now} />)}
            {list.length === 0 && <Empty title={`No ${h.name} account`} />}
          </div>
        </div>
      </section>; })}
        </div>
      </fieldset>
      <p class="svc-foot-note">
        Sign in through the harness's own program. Check again reads local sign-in evidence. Provider authentication is checked when an agent starts.
      </p>
    </div>
  </div>;
}
// Old bookmarks keep opening the combined page.
export const Subscriptions = Harnesses;
/** What installing a harness returned. */
interface InstallResult { state: string; installation?: { installed?: boolean; version?: string }; output?: string; reason?: string }
function AddSubscription({ s, d }: DialogProps) {
  const harness: string = d.harness || Object.keys(HARNESSES)[0], h = HARNESSES[harness]!;
  const [name, setName] = useState(''), [home, setHome] = useState(''), [machine, setMachine] = useState('studio');
  const [recipe, setRecipe] = useState<HarnessRecipe | null | undefined>(live.on ? null : h.recipe), [result, setResult] = useState<InstallResult | null>(null), [step, setStep] = useState(h.installed ? 'account' : 'install');
  const [pending, setPending] = useState(false), [error, setError] = useState(''), [account, setAccount] = useState<Subscription | null>(null);
  const separateSupported = ['claude', 'codex', 'gemini'].includes(harness);
  const [folder, setFolder] = useState(separateSupported && s.subscriptions.some(x => x.harness === harness && !x.home) ? 'separate' : 'existing');
  const terminal = signInUi.useSignIn();
  const close = () => { terminal.stop(); act.closeDialog(); };
  useEffect(() => { if (live.on && step === 'install') liveGit.harnessRecipe(harness).then(setRecipe).catch((e) => setError(errorText(e))); }, [harness]);
  const install = async () => { setPending(true); setError(''); try {
    if (live.on) setResult(await liveGit.installHarness(harness));
    else { const next: InstallResult = { state: 'installed', installation: { installed: true, version: 'Sample installation' }, output: '' }; update(() => { Object.assign(h, next.installation); const m = machineOf(s, machine); if (m?.status === 'online') { m.harnesses ||= {}; m.harnesses[harness] = 'Sample installation'; } }); setResult(next); }
  } catch (e) { setError(errorText(e)); } finally { setPending(false); } };
  const check = async () => { setPending(true); try { if (live.on) await liveGit.checkHarnesses(); if (h.installed || HARNESSES[harness]?.installed) { setStep('account'); setResult(null); } } finally { setPending(false); } };
  const dup = s.subscriptions.some((x) => x.id !== account?.id && x.harness === harness && (live.on || x.machine === machine) && x.name.toLowerCase() === name.trim().toLowerCase());
  const selectedMachine = machineOf(s, machine);
  const why = !live.on && selectedMachine?.status !== 'online' ? `${selectedMachine?.name || 'This machine'} is offline.` : !live.on && !selectedMachine?.harnesses?.[harness] ? `${h.name} is not installed on ${selectedMachine?.name || 'this machine'}.` : !name.trim() ? 'Give the account a name.' : dup ? 'This harness already has an account with that name.' : folder === 'advanced' && !home.startsWith('/') ? 'Use an absolute login folder.' : '';
  const add = async (run = false) => { if (why || step !== 'account' || pending) return; setPending(true); setError('');
    const scope = server.captureSession();
    const current = () => store.state === s && s.dialog === d && (scope?.isCurrent() ?? true);
    try {
      const added = account || await signInUi.addAccount(harness, name.trim(), folder, home.trim(), machine);
      if (!current()) return;
      setAccount(added);
      if (run) await terminal.start(added);
      else close();
    } catch (e) { if (current()) setError(errorText(e)); } finally { if (current()) setPending(false); } };
  return <Dialog title={'Add account · ' + h.name} size={terminal.session ? 'signin-wide' : ''} onClose={pending ? () => {} : close} onSubmit={step === 'account' ? () => add(false) : () => {}} foot={<>
      <button type="button" class="btn" disabled={pending} onClick={close}>{account ? 'Done' : 'Cancel'}</button>
      {step === 'install' ? result?.state === 'installed' ? <button type="button" class="btn primary" onClick={() => setStep('account')}>Continue</button> : <>
        <button type="button" class="btn" disabled={pending} onClick={check}>Check again</button>
        <button type="button" class="btn primary" disabled={pending || !recipe?.automatic} onClick={install}>
          {pending ? 'Installing…' : 'Install'}
        </button>
      </> : <button type="submit" class="btn primary" disabled={!!why || pending || !!account}>Add account</button>}
    </>}>
    {step === 'install' ? <>
      <p>{h.name} must be installed before adding an account.</p>
      {recipe && <>
        <CodeBox text={recipe.commands.join('\n')} label="install recipe" />
        <p class="muted">{recipe.packages.join(', ')}</p>
        <p><a class="link" href={recipe.source} target="_blank" rel="noreferrer">Official installation documentation</a></p>
        {recipe.reason && <p>{recipe.reason}</p>}
      </>}
      {result && <>
        <p role="status" class={result.state === 'installed' ? 'success-text' : 'error-text'}>
          {result.state === 'installed' ? 'Installed · ' + (result.installation!.version || 'Version not reported') : result.reason}
        </p>
        {result.output && <CodeBox text={result.output} label="install output" />}
      </>}
    </> : <>
        <div class="field">
          <label for="as-name">Name</label>
          <input id="as-name" class="input" disabled={!!account} value={name} onInput={(e) => setName(e.currentTarget.value)} autofocus />
          {name && why && <span class="error">{why}</span>}
        </div>
        {!live.on && <div class="field">
        <label for="as-machine">Machine</label>
        <select id="as-machine" class="select" value={machine} onChange={(e) => setMachine(e.currentTarget.value)}>
          {s.machines.map((x) => <option value={x.id} disabled={x.status !== 'online'}>{x.name}{x.status !== 'online' ? ' (offline)' : ''}</option>)}
        </select>
      </div>}
        <fieldset class="signin-folders" disabled={!!account || pending}>
          <legend>Login folder</legend>
          <label>
            <input type="radio" name="as-folder" value="existing" checked={folder === 'existing'} onChange={() => setFolder('existing')} />
            Use my existing sign-in
          </label>
          <label>
            <input type="radio" name="as-folder" value="separate" disabled={!separateSupported} checked={folder === 'separate'} onChange={() => setFolder('separate')} />
            A separate sign-in
          </label>
          <details>
            <summary>Advanced login folder</summary>
            <label>
              <input type="radio" name="as-folder" checked={folder === 'advanced'} onChange={() => setFolder('advanced')} />
              Use an absolute folder
            </label>
            <input id="as-home" aria-label="Absolute login folder" class="input mono" value={home} onInput={(e) => { setFolder('advanced'); setHome(e.currentTarget.value); }} />
          </details>
        </fieldset>
        <p>Sign in here, or run the command in your own terminal.</p>
        <CodeBox text={h.signInCommand || h.recipe?.signInCommand || 'Use the harness’s own sign-in command.'} label="sign-in command" />
        <button type="button" class="btn primary" disabled={!!why || pending || terminal.pending || terminal.running} onClick={() => add(true)}>
          Sign in here
        </button>
        {h.recipe?.signInSource && <p><a class="link" href={h.recipe.signInSource} target="_blank" rel="noreferrer">Official sign-in documentation</a></p>}
      </>}
    {terminal.session && <signInUi.TerminalPanel terminal={terminal} />}
    {!terminal.session && terminal.error && <p role="alert">{terminal.error} You can use the command in your own terminal.</p>}
    {error && <p role="alert" class="error-text">{error}</p>}
  </Dialog>;
}
registerDialog('add-subscription', AddSubscription);

function HarnessSignIn({ s, d }: DialogProps) {
  const account = s.subscriptions.find((x) => x.id === d.account), h: Harness | undefined = HARNESSES[account?.harness as string];
  const [check, setCheck] = useState<SignInCheck | null>(null), [error, setError] = useState(''), [pending, setPending] = useState(false);
  const terminal = signInUi.useSignIn();
  const close = () => { terminal.stop(); act.closeDialog(); };
  const again = async () => { setPending(true); setError(''); try { setCheck(live.on ? await liveGit.checkAccount(account!.id) : { state: account!.signedIn ? 'local-credential' : 'signed-out', signInCommand: h!.recipe?.signInCommand }); } catch (e) { setError(errorText(e)); } finally { setPending(false); } };
  useEffect(() => { void again(); }, [d.account]);
  return <Dialog title={'Sign in · ' + (account?.name || '')} size={terminal.session ? 'signin-wide' : ''} onClose={close} foot={<>
    <button type="button" class="btn" onClick={close}>Done</button>
    <button type="button" class="btn" disabled={pending || terminal.running} onClick={again}>Check again</button>
  </>}>
    <p>Sign in here, or run this command in your terminal.</p>
    <CodeBox text={check?.signInCommand || h?.signInCommand || h?.recipe?.signInCommand || ''} label="sign-in command" />
    <p class="hint">Login folder: {signInUi.folderLabel(account)}.</p>
    <button type="button" class="btn primary" disabled={pending || terminal.pending || terminal.running || !account} onClick={() => terminal.start(account!)}>
      Sign in here
    </button>
    {h?.recipe?.signInSource && <p><a class="link" href={h.recipe.signInSource} target="_blank" rel="noreferrer">Official sign-in documentation</a></p>}
    {terminal.session ? <signInUi.TerminalPanel terminal={terminal} /> : check && <p role="status">{signInUi.checkWords(check)}</p>}
    {(error || terminal.error) && !terminal.session && <p role="alert">{error || terminal.error} Use the command in your terminal if sign-in cannot run here.</p>}
  </Dialog>;
}
registerDialog('harness-signin', HarnessSignIn);

// ---- notifications ---------------------------------------------------------
const EVENTS: [string, string, string][] = [
  ['question', 'Question from an agent', 'Sent when a question needs your answer.'],
  ['approval', 'Approval waiting', 'Sent when permission needs your approval.'],
  ['merge', 'Merge ready', 'A project is done and reviewed, and the merge is yours.'],
  ['failed', 'Agent start failure', 'A sign-in is missing or a machine is offline.'],
  ['sprint', 'Sprint finished', 'Sprint alerts stay here until you dismiss them.'],
];
const EXTRA_EVENTS: [string, string][] = [
  ['account', 'Account attention'],
  ['other', 'Other attention'],
  ['client.web-down', 'Web server down'],
  ['board.post', 'Board post awaiting reply'],
  ['board.no-answer', 'Unanswered board post'],
  ['roles.stall', 'Stalled agent'],
  ['roles.replace', 'Worker replacement'],
  ['roles.runaway', 'Overdue work'],
  ['roles.rule-first-use', 'First use of a permission rule'],
];
const NOTE_KIND: Record<string, string> = { ...KIND_LABEL, sprint: 'Sprint done', test: 'Test' };
const DELIVERY: Record<string, [string, string]> = { sent: ['Sent', 'muted'], off: ['Not sent', 'faint'], failed: ['Failed', 'error-text'], quiet: ['Held until quiet hours end', 'muted'], muted: ['Not sent · turned off', 'faint'] };
function sendTest(name = "ntfy") { if (live.on) return liveGit.sendTest(name);
  if (name === "email") { act.toast("Sample email accepted. No email was sent."); return; }
  const s = store.state; const server = s.settings.ntfyServer;
  const ok = /\./.test(server) && !/localhost/.test(server);
  update((st) => { st.notifications.unshift({ id: uid('n'), ts: Date.now(), kind: 'test', title: 'Test notification from LGTM Studio', read: true, delivery: ok ? 'sent' : 'failed', error: ok ? null : `Could not reach ${server}` }); });
  act.toast(ok ? `Test sent to ${s.settings.ntfyTopic}` : `Could not reach ${server}`, ok ? 'ok' : 'error');
}
function NtfyForm({ s }: ScreenProps) {
  const set = s.settings; const on = !!set.ntfyServer;
  const [f, setF] = useState({ server: set.ntfyServer, topic: set.ntfyTopic, token: '' }); const [show, setShow] = useState(false);
  const dirty = f.server !== set.ntfyServer || f.topic !== set.ntfyTopic || !!f.token;
  const bad = f.server.trim() && !/^https?:\/\/\S+$/.test(f.server.trim()) ? 'Start the address with https://' : null;
  const badTopic = f.topic && !/^[\w-]+$/.test(f.topic) ? 'A topic has letters, digits, hyphens and underscores only.' : null;
  const can = f.server.trim() && f.topic.trim() && !bad && !badTopic && dirty;
  const save = async () => { if (!can) return; if (live.on) { const token = f.token; setF({ ...f, token: '' }); await liveGit.saveCarrier({ ...f, token }); return; } setS({ ntfyServer: f.server.trim(), ntfyTopic: f.topic.trim(), ntfyToken: '', tokenSet: !!f.token || set.tokenSet, ntfyEnabled: true }); setF({ ...f, token: '' }); act.toast('Notifications are on'); };
  const off = () => act.confirm({ title: 'Turn off notifications?', body: 'The server, topic and token are forgotten. Requests still show on Home and in the bell.', action: 'Turn off', onConfirm: () => { if (live.on) return liveGit.saveCarrier({ server: '', topic: '', token: '' }); setS({ ntfyServer: '', ntfyTopic: '', ntfyToken: '' }); setF({ server: '', topic: '', token: '' }); } });
  const fld = (k: string) => (e: Field) => setF({ ...f, [k]: e.currentTarget.value });
  return <form class="svc-form" aria-label="ntfy delivery" onSubmit={(e) => { e.preventDefault(); save(); }}>
    {on && <Row label="ntfy delivery">
      <Switch on={set.ntfyEnabled !== false} label="ntfy enabled" onChange={(enabled) => live.on ? liveGit.carrierEnabled('ntfy', enabled) : setS({ ntfyEnabled: enabled })} />
    </Row>}
    <div class="field">
      <label for="nt-server">ntfy server</label>
      <input id="nt-server" class="input mono" value={f.server} onInput={fld('server')} placeholder="https://ntfy.example.com" spellcheck={'false' as unknown as boolean} autocapitalize="off" />
      {bad && <span class="error">{bad}</span>}
    </div>
    <div class="field">
      <label for="nt-topic">Topic</label>
      <input id="nt-topic" class="input mono" value={f.topic} onInput={fld('topic')} placeholder="lgtm-alerts" spellcheck={'false' as unknown as boolean} autocapitalize="off" />
      {badTopic ? <span class="error">{badTopic}</span> : <span class="hint">
        Subscribe to the same topic in the ntfy app on your phone. Anyone who knows the topic can read the alerts, so pick a long one.
      </span>}
    </div>
    <div class="field">
      <label for="nt-token">Access token <span class="faint" style="font-weight:var(--weight-body)">· optional</span></label>
      <span class="row">
        <input id="nt-token" class="input mono grow" type={(show ? 'text' : 'password') as 'text'} value={f.token} onInput={fld('token')} placeholder={set.tokenSet ? 'Token is set. Enter a replacement.' : 'tk_…'} autocomplete="off" spellcheck={'false' as unknown as boolean} />
        <button type="button" class="btn icon" aria-label={show ? 'Hide the token' : 'Show the token'} aria-pressed={show} onClick={() => setShow(!show)}>
          <Icon name={show ? 'eye-off' : 'eye'} size={14} />
        </button>
      </span>
    </div>
    <div class="row" style="flex-wrap:wrap">
      <button type="submit" class="btn primary" disabled={!can || (live.on && liveGit.busy('notifications'))}>
        {live.on && liveGit.busy('notifications') ? 'Saving…' : on ? 'Save changes' : 'Turn on'}
      </button>
      <button type="button" class="btn" disabled={!on || dirty || ((!set.tokenSet || set.ntfyEnabled === false || (live.on && liveGit.busy('notifications'))))} data-tip={!on ? 'Turn notifications on first' : dirty ? 'Save your changes first' : null!} onClick={() => sendTest('ntfy')}>
        <Icon name="send" size={13} />
        Send a test
      </button>
      <span class="grow" />
      {set.tokenSet && <button type="button" class="btn ghost" onClick={() => live.on ? liveGit.clearToken() : setS({ tokenSet: false })}>
        Clear token
      </button>}
      {on && <button type="button" class="btn ghost" onClick={off}>Turn off</button>}
    </div>
  </form>;
}
function EmailForm({ s }: ScreenProps) {
  const saved: Settings['email'] = s.settings.email || {};
  const values = { host: saved.host || '', port: saved.port || 465, security: saved.security || 'tls', username: saved.username || '', from: saved.from || '', to: saved.to || '' };
  const [f, setF] = useState({ ...values, secret: '' });
  const dirty = (Object.keys(values) as (keyof typeof values)[]).some((k) => String(f[k]) !== String(values[k])) || !!f.secret;
  const credentialChanged = !!saved.host && (['host', 'port', 'security', 'username'] as const).some((k) => String(f[k]).trim() !== String(values[k]));
  const passwordRequired = !saved.tokenSet || credentialChanged;
  const can = (['host', 'username', 'from', 'to'] as const).every((k) => f[k].trim()) && Number(f.port) > 0 && Number(f.port) <= 65535 && (dirty || !saved.host) && (!passwordRequired || !!f.secret);
  const fld = (k: string) => (e: Field) => setF({ ...f, [k]: e.currentTarget.value });
  const save = async () => {
    if (!can) return;
    const form = { ...f }; setF({ ...f, secret: '' });
    if (live.on) return liveGit.saveEmail(form);
    setS({ email: { ...values, ...form, port: Number(form.port), secret: undefined, tokenSet: !!form.secret || saved.tokenSet, enabled: saved.host ? saved.enabled : true } });
    act.toast('Email settings saved.');
  };
  const enabled = (v: boolean) => live.on ? liveGit.carrierEnabled('email', v) : setS({ email: { ...saved, enabled: v } });
  const clear = () => live.on ? liveGit.clearEmailSecret() : setS({ email: { ...saved, tokenSet: false, enabled: false } });
  const remove = () => act.confirm({ title: 'Remove email settings?', body: 'The mail server settings and stored password are forgotten. Other delivery carriers stay as they are.', action: 'Remove email', onConfirm: () => live.on ? liveGit.removeEmail() : setS({ email: {} as Settings['email'] }) });
  return <form class="svc-form" aria-label="Email delivery" onSubmit={(e) => { e.preventDefault(); save(); }}>
    {saved.host && <Row label="Email delivery"><Switch on={saved.enabled} label="Email enabled" onChange={enabled} /></Row>}
    <div class="field">
      <label for="em-host">SMTP host</label>
      <input id="em-host" class="input mono" value={f.host} onInput={fld('host')} placeholder="smtp.example.com" autocomplete="off" spellcheck={'false' as unknown as boolean} />
    </div>
    <div class="field">
      <label for="em-port">SMTP port</label>
      <input id="em-port" class="input mono" type="number" min="1" max="65535" value={f.port} onInput={fld('port')} />
    </div>
    <div class="field">
      <label for="em-security">Connection security</label>
      <select id="em-security" class="input" value={f.security} onChange={fld('security')}>
        <option value="tls">TLS from connection (usually 465)</option>
        <option value="starttls">Require STARTTLS (usually 587)</option>
        <option value="plain">Plain connection on 127.0.0.1 only</option>
      </select>
    </div>
    <div class="field">
      <label for="em-username">Mail user name</label>
      <input id="em-username" class="input" value={f.username} onInput={fld('username')} autocomplete="off" spellcheck={'false' as unknown as boolean} />
    </div>
    <div class="field">
      <label for="em-secret">Password or app key</label>
      <input id="em-secret" class="input mono" type="password" required={passwordRequired} value={f.secret} onInput={fld('secret')} placeholder={saved.tokenSet ? 'Password is set. Enter a replacement.' : 'Enter a password or app key'} autocomplete="new-password" spellcheck={'false' as unknown as boolean} />
      <span class="hint">
        {credentialChanged && saved.tokenSet ? 'Changing the mail server or user name will clear the stored password. Enter a password or app key before saving.' : 'A stored password is never shown. Leave this blank to keep it for the same mail server and user name.'}
      </span>
    </div>
    <div class="field">
      <label for="em-from">Sender address</label>
      <input id="em-from" class="input" type="email" value={f.from} onInput={fld('from')} autocomplete="off" />
    </div>
    <div class="field">
      <label for="em-to">Recipient address</label>
      <input id="em-to" class="input" type="email" value={f.to} onInput={fld('to')} autocomplete="off" />
    </div>
    <div class="row" style="flex-wrap:wrap">
      <button type="submit" class="btn primary" disabled={!can}>Save email</button>
      <button type="button" class="btn" disabled={!saved.enabled || !saved.tokenSet || dirty} onClick={() => sendTest('email')}>
        <Icon name="send" size={13} />
        Send a test
      </button>
      {saved.tokenSet && <button type="button" class="btn ghost" onClick={clear}>Clear password</button>}
      {saved.host && <button type="button" class="btn ghost" onClick={remove}>Remove email</button>}
    </div>
  </form>;
}
function QuietForm({ s }: ScreenProps) {
  const saved = s.settings.quiet;
  const [f, setF] = useState({ ...saved });
  const dirty = (['on', 'from', 'to', 'timeZone', 'failed'] as const).some((k) => f[k] !== saved[k]);
  const patch = (p: Partial<Settings['quiet']>) => setF({ ...f, ...p });
  const save = () => live.on ? liveGit.saveQuiet(f) : setS({ quiet: { ...f } });
  return <form class="svc-form" aria-label="Quiet hours" onSubmit={(e) => { e.preventDefault(); save(); }}>
    <Row label="Hold alerts at night" hint="Open alerts are sent when quiet hours end.">
      <Switch on={f.on} label="Quiet hours" onChange={(on) => patch({ on })} />
    </Row>
    <div class="field">
      <label for="q-from">From</label>
      <input id="q-from" class="input" type="time" value={f.from} onInput={(e) => patch({ from: e.currentTarget.value })} />
    </div>
    <div class="field">
      <label for="q-to">Quiet hours end</label>
      <input id="q-to" class="input" type="time" value={f.to} onInput={(e) => patch({ to: e.currentTarget.value })} />
    </div>
    <div class="field">
      <label for="q-zone">Time zone</label>
      <input id="q-zone" class="input" value={f.timeZone || ''} onInput={(e) => patch({ timeZone: e.currentTarget.value })} placeholder="America/Chicago" spellcheck={'false' as unknown as boolean} />
    </div>
    <Row label="Let urgent alerts through" hint="Agent failures, account problems, and a web server outage can reach you during quiet hours.">
      <Switch on={f.failed} label="Let urgent alerts through" onChange={(failed) => patch({ failed })} />
    </Row>
    <button type="submit" class="btn primary" disabled={!dirty || !f.from || !f.to || !f.timeZone}>Save quiet hours</button>
  </form>;
}

export function Notifications({ s }: ScreenProps) {
  useEffect(() => liveGit.openScreen('notifications'), []);
  const now = useNow(); const set = s.settings; const on = (!!set.ntfyServer && set.ntfyEnabled !== false) || !!set.email?.enabled;
  const list = s.notifications; const unread = list.filter((n) => !n.read).length; const failed = list.filter((n) => n.delivery === 'failed').length;
  const [showAll, setShowAll] = useState(false); const shown = showAll ? list : list.slice(0, 12);
  const open = (n: Notification) => { if (live.on) { const r = n.request && byId(s.requests, n.request); if (r) go(routeForRequest(s, r)); else if (n.to) go(n.to); return; } update(() => { n.read = true; }); const r = n.request && byId(s.requests, n.request); if (r) go(routeForRequest(s, r)); else if (n.to) go(n.to); };
  const retry = (n: Notification) => { if (live.on) return liveGit.retryAlert(n); act.toast('Sending again…'); act.sampleLater(() => { const ok = /\./.test(set.ntfyServer) && !/localhost/.test(set.ntfyServer); update(() => { n.delivery = ok ? 'sent' : 'failed'; }); act.toast(ok ? 'Sent to phone' : n.error, ok ? 'ok' : 'error'); }, 700); };
  const setOn = (k: string, v: boolean) => live.on ? liveGit.notifyKind(k, v) : setS({ notifyOn: { ...set.notifyOn, [k]: v } });
  let lastDay: string | null = null;
  return <div class="view-body">
    <div class="page">
      <div class="page-head">
        <h1 class="page-title">Notifications</h1>
        <span class="sub row" style="gap:6px">
          <i class={cx('dot', on ? (failed ? 'failed' : 'working') : 'idle')} />
          {on ? (set.ntfyServer && set.ntfyEnabled !== false ? <>Sending to <span class="mono">{set.ntfyTopic}</span>{set.email?.enabled ? ' and email' : ''}</> : 'Sending by email') : 'Not set up'}
        </span>
        <Saved s={s} />
      </div>
      <fieldset class="svc-split" disabled={live.on && liveGit.busy('notifications')} aria-busy={live.on && liveGit.busy('notifications')} style="border:0;padding:0;margin:0;min-width:0">
        <div class="svc-side">
          <Section title="ntfy" sub="Send alerts to your ntfy topic">
            {!on && s.phoneOffer === 'open' && <div class="callout info" style="align-items:center;margin-bottom:var(--space-3)">
            <Icon name="smartphone" size={15} />
            <span class="grow">
              Alerts reach your phone when an agent waits for you. Until then, requests show here, on Home and in the bell.
            </span>
            <button type="button" class="icon-btn sm" aria-label="Dismiss" onClick={act.declinePhoneOffer}>
              <Icon name="x" size={13} />
            </button>
          </div>}
            {(!live.on || s.settings.notifyOn && Object.keys(s.settings.notifyOn).length > 0) && <NtfyForm key={live.on ? set.ntfyServer + set.ntfyTopic : 'mock'} s={s} />}
          </Section>
          <Section title="Email" sub="Send through your own mail server">
            {(!live.on || Object.keys(set.notifyOn || {}).length > 0) && <EmailForm key={JSON.stringify(set.email || {})} s={s} />}
          </Section>
          <Section title="Events">
            <div class={cx('svc-rows', !on && 'svc-off')}>
              {(live.on ? Object.keys(set.notifyOn).map((k) => [k, [...EVENTS, ...EXTRA_EVENTS].find((x) => x[0] === k)?.[1] || 'Attention needed', EVENTS.find((x) => x[0] === k)?.[2] || '']) : EVENTS).map(([k, label, hint]) => <Row key={k} label={label} hint={hint}><Switch on={set.notifyOn[k]} label={label} onChange={(v) => setOn(k, v)} /></Row>)}
            </div>
          </Section>
          <Section title="Quiet hours">
            {(!live.on || Object.keys(set.notifyOn || {}).length > 0) && <QuietForm key={JSON.stringify(set.quiet)} s={s} />}
          </Section>
        </div>
        <div class="svc-main" data-main>
          <Section title="Recent notifications" sub={unread ? `${unread} ${live.on ? 'open' : 'unread'}` : live.on ? 'All cleared' : 'All read'} action={<button type="button" class="btn sm ghost" disabled={!unread} onClick={() => live.on ? liveGit.dismissAlerts() : update((st) => { for (const n of st.notifications) n.read = true; })}>
            <Icon name="check" size={13} />
            {live.on ? 'Dismiss all' : 'Mark all read'}
          </button>}>
            {list.length === 0 && <Empty title="No notifications" icon="bell" />}
            <ul class="svc-notes">
              {shown.map((n) => { const d = dayLabel(n.ts, now); const head = d !== lastDay ? <li class="svc-day" key={'d' + n.id}>{d}</li> : null; lastDay = d;
            const a = n.agent && agentOf(s, n.agent); const p = n.project && projectOf(s, n.project); const r = n.request && byId(s.requests, n.request);
            const del = n.delivery === 'off' && on ? (set.notifyOn[n.kind] === false ? 'muted' : 'off') : n.delivery; const [dw, dc] = DELIVERY[del] || DELIVERY.off;
            const clickable = !!(r || n.to);
            return <>
              {head}
              <li key={n.id} class={cx('svc-note', !n.read && 'unread', n.kind === 'failed' && 'failed')}>
                <i class="pip" aria-label={n.read ? null! : live.on ? 'Open' : 'Unread'} />
                <span class={cx('kind', n.kind === 'failed' ? 'failed' : r && r.status === 'open' ? n.kind : 'closed')}>
                  {NOTE_KIND[n.kind] || n.kind}
                </span>
                {clickable ? <a class="title truncate" href={href(r ? routeForRequest(s, r) : n.to!)} onClick={(e) => { e.preventDefault(); open(n); }}>
                {n.title}
              </a> : <span class="title truncate">{n.title}</span>}
                <span class="from muted truncate">
                  {a ? a.name : 'LGTM Studio'}
                  {p ? ' · ' + p.name : ''}
                  {r && r.status !== 'open' ? ' · ' + r.answer : n.answer ? ' · ' + n.answer : ''}
                </span>
                <span class={cx('del', dc)} title={n.deliveries?.length ? n.deliveries.map((d) => `${d.carrier}: ${d.status === 'delivered' ? 'Sent' : d.reason}`).join(' · ') : n.error || (del === 'off' ? 'Notifications were not set up' : null!)}>
                  {del === 'failed' ? <Icon name="circle-alert" size={13} /> : del === 'sent' ? <Icon name="check" size={13} /> : null}
                  {dw}
                  {del === 'failed' && <button type="button" class="link" onClick={() => retry(n)}>Retry</button>}
                </span>
                <time class="muted">{now - n.ts < DAY ? clock(n.ts) : ago(n.ts, now) + ' ago'}</time>
              </li>
            </>; })}
            </ul>
            {list.length > 12 && <button type="button" class="btn sm ghost" style="margin-top:6px" onClick={() => setShowAll(!showAll)}>
            {showAll ? 'Show fewer' : `Show all ${list.length}`}
          </button>}
          </Section>
        </div>
      </fieldset>
    </div>
  </div>;
}

// ---- service settings ------------------------------------------------------------
const PERMISSION_HINT: Record<string, string> = {
  'Read only': 'The agent reads and searches. Every change and command is refused.',
  'Ask before every change': 'Each edit and each command becomes an approval for you.',
  'Edit files without asking': 'Edits inside the worktree are free. Commands still ask.',
  'Automatic review': 'The harness may approve, ask or refuse an action. Uses edit mode when no automatic mapping is configured.',
  'Full access': 'Edits and commands inside the worktree run without asking.',
};
function PermissionPicker({ value, onChange, name }: { value: string | null; onChange: (mode: string) => unknown; name: string }) {
  return <div class="svc-radios" role="radiogroup" aria-label="Permission">
    {PERMISSION_MODES.map((p) => <label key={p} class={cx('svc-radio', value === p && 'on')}>
    <input type="radio" name={name} checked={value === p} onChange={() => onChange(p)} />
    <span><b>{p}</b><small>{PERMISSION_HINT[p]}</small></span>
  </label>)}
  </div>;
}
function Appearance({ s }: ScreenProps) {
  const p = s.prefs;
  return <div class="svc-rows">
    <Row label="Theme" hint="Field is the light palette. Dusk is the dark palette.">
      <Seg label="Theme" value={p.theme} onChange={(v) => { act.setPref('theme', v); markSaved(); }} options={[{ id: 'light', label: 'Field', icon: 'sun' }, { id: 'dark', label: 'Dusk', icon: 'moon' }, { id: 'system', label: 'Match system', icon: 'monitor' }]} />
    </Row>
    <Row label="Density" hint="Compact fits more rows. Regular gives each row more room.">
      <Seg label="Density" value={p.density} onChange={(v) => { act.setPref('density', v); markSaved(); }} options={[{ id: 'compact', label: 'Compact' }, { id: 'regular', label: 'Regular' }]} />
    </Row>
    <Row label="Sidebar" hint={'Fold it to a rail of icons.'}>
      <Switch on={p.sidebarFolded} label="Fold the sidebar" onChange={(v) => { act.setPref('sidebarFolded', v); markSaved(); }} />
    </Row>
    <Row label="Preview" top>
      <div class="svc-preview" aria-hidden="true">
        <div class="list-row" aria-selected="true">
          <i class="dot waiting" />
          <b style="font-weight:var(--weight-medium)">Build lead</b>
          <span class="muted grow truncate">Waiting for your answer</span>
          <span class="warn-text" style="font-size:var(--text-sm)">18 min</span>
        </div>
        <div class="list-row">
          <i class="dot working" />
          <b style="font-weight:var(--weight-medium)">Builder 1</b>
          <span class="muted grow truncate">Editing Sidebar.tsx</span>
          <span class="tag">LS-A01-14</span>
        </div>
        <div class="list-row">
          <i class="dot idle" />
          <b style="font-weight:var(--weight-medium)">Test runner</b>
          <span class="muted grow truncate">Idle</span>
        </div>
        <div class="row" style="padding:8px var(--inset) 4px">
          <button type="button" class="btn primary" tabindex={-1}>Allow</button>
          <button type="button" class="btn" tabindex={-1}>Deny</button>
          <input class="input" tabindex={-1} placeholder="Or type another answer" />
        </div>
      </div>
    </Row>
  </div>;
}
function HarnessDefaults({ s }: ScreenProps) {
  const set = s.settings; const h = HARNESSES[set.defaultHarness] || { models: [], efforts: [] };
  return <div class="svc-rows">
    <Row label="Harness" hint="The program a new agent runs in.">
      <Seg label="Harness" value={set.defaultHarness} onChange={(v) => setS({ defaultHarness: v, defaultModel: live.on ? undefined : HARNESSES[v]!.models[HARNESSES[v]!.models.length > 2 ? 2 : 0], defaultEffort: 'Medium' })} options={Object.entries(HARNESSES).map(([id, x]) => ({ id, label: x.name }))} />
    </Row>
    <Row label="Model" id="df-model" hint={live.on ? 'Defaults for new project leads.' : 'Leads start one step above this model.'}>
      <select id="df-model" class="select" style="width:200px" value={set.defaultModel} onChange={(e) => setS({ defaultModel: e.currentTarget.value })}>
        {live.on && !set.defaultModel && <option value="" />}
        {h.models.map((m: string) => <option value={m}>{m}</option>)}
      </select>
    </Row>
    <Row label="Effort">
      <Seg label="Effort" value={set.defaultEffort} onChange={(v) => setS({ defaultEffort: v })} options={h.efforts.map((e: string) => ({ id: e, label: e }))} />
    </Row>
  </div>;
}
function Defaults({ s }: ScreenProps) {
  const set = s.settings;
  const workspaceEditing = useRef(false), [workspaceDraft, setWorkspaceDraft] = useState('');
  return <div class="svc-rows">
    <Row label="Where is your workspace?" id="df-workspace" hint="LGTM Studio's Git commands work in every repository under this folder.">
      <input id="df-workspace" class="input mono" value={workspaceEditing.current ? workspaceDraft : set.workspaceFolder}
        onFocus={() => { workspaceEditing.current = true; setWorkspaceDraft(set.workspaceFolder || ''); }}
        onInput={(e) => setWorkspaceDraft(e.currentTarget.value)}
        onBlur={() => { workspaceEditing.current = false; setWorkspaceDraft(''); }}
        onChange={(e) => { workspaceEditing.current = false; setWorkspaceDraft(''); setS({ workspaceFolder: e.currentTarget.value.trim() }); }} />
    </Row>
    <Row label="Permission" hint="What a new agent may do without asking. A project can set its own." top>
      <PermissionPicker name="df-perm" value={set.defaultPermission} onChange={(v) => setS({ defaultPermission: v })} />
    </Row>
    <Row label="Need-to-know messages" hint="New teams keep direct messages private to the agents in them. You always see everything.">
      <Switch on={set.needToKnow} label="Need-to-know messages" onChange={(v) => setS({ needToKnow: v })} />
    </Row>
    <Row label="Remove agent credits when merging" hint="Strips the co-author lines agents add to their commits.">
      <Switch on={set.removeCredits} label="Remove agent credits when merging" onChange={(v) => setS({ removeCredits: v })} />
    </Row>
  </div>;
}
function Remote({ s }: ScreenProps) {
  const set = s.settings;
  // Only fields edited here override refreshed store values.
  const [edits, setEdits] = useState<Partial<Settings>>({});
  const f = { webEnabled: set.webEnabled, port: set.port, checkIn: set.checkIn, proxyMode: set.proxyMode, publicAddress: set.publicAddress, ...edits };
  const pending = (draft: Partial<Settings>) => Object.fromEntries(Object.entries(draft).filter(([key, value]) => value !== set[key])) as Partial<Settings>;
  const setF = (patch: Partial<Settings>) => setEdits((draft) => pending({ ...draft, ...patch }));
  useEffect(() => setEdits((draft) => pending(draft)), [set.webEnabled, set.port, set.checkIn, set.proxyMode, set.publicAddress]);
  const dirty = (Object.keys(f) as (keyof typeof f)[]).some((k) => f[k] !== set[k]);
  const bad = f.proxyMode && !/^https:\/\/\S+\.\S+/.test(f.publicAddress.trim()) ? (f.publicAddress.trim() ? 'The public address must start with https://. Plain HTTP is refused.' : 'Give the address your proxy serves.') : null;
  const badPort = !(f.port >= (live.on ? 1 : 1024) && f.port <= 65535) ? 'Use a port from 1024 to 65535.' : null;
  const save = () => { if (bad || badPort || !dirty) return; if (live.on) return setS({ ...f, publicAddress: f.publicAddress.trim() }); const portChanged = f.port !== set.port; setS({ ...f, publicAddress: f.publicAddress.trim() }); act.toast(portChanged ? 'Saved. The new port applies at the next start.' : 'Saved. Web connections were closed and reopened.'); };
  const master = s.machines.find((m) => m.role === 'master');
  const fwd = live.on ? '' : `ssh -L ${f.port}:127.0.0.1:${f.port} ${master?.name.toLowerCase()}`;
  return <form onSubmit={(e) => { e.preventDefault(); save(); }}>
    <div class="svc-rows">
      <Row label="Project terminal" hint="Turning it on lets signed-in owner pages run shells as you on the service machine.">
        <Switch on={set.terminalEnabled === true} label="Project terminal" onChange={(value) => setS({ terminalEnabled: value })} />
      </Row>
      <Row label="Web page" hint="Turn it off to use LGTM Studio from the terminal only.">
        <Switch on={f.webEnabled} label="Web page" onChange={(v) => setF({ webEnabled: v })} />
      </Row>
      <Row label="Listens on" hint="This computer only. The service never listens on the network.">
        <span class="row" style="gap:6px">
          <span class="svc-fixed mono">http://127.0.0.1:</span>
          <input class="input num-field mono" type="number" aria-label="Port" value={f.port} onInput={(e) => setF({ port: +e.currentTarget.value })} />
        </span>
        {badPort && <span class="svc-err">{badPort}</span>}
      </Row>
      <Row label="Check-in" id="ra-check" hint="How often the page asks the service whether it is alive.">
        <span class="row" style="gap:6px">
          <input id="ra-check" class="input num-field" type="number" min="5" max="120" value={f.checkIn} onInput={(e) => setF({ checkIn: +e.currentTarget.value })} />
          <span class="muted">seconds</span>
        </span>
      </Row>
      <Row label="Behind your HTTPS proxy" hint="Turn on when a proxy such as Caddy serves this page over HTTPS.">
        <Switch on={f.proxyMode} label="Behind your HTTPS proxy" onChange={(v) => setF({ proxyMode: v })} />
      </Row>
      {f.proxyMode && <Row label="Public address" id="ra-addr" hint="Phone alerts link to a thread through this address.">
        <input id="ra-addr" class="input mono" value={f.publicAddress} onInput={(e) => setF({ publicAddress: e.currentTarget.value })} placeholder="https://lgtm.home.example" spellcheck={'false' as unknown as boolean} autocapitalize="off" />
        {bad && f.publicAddress.trim() && <span class="svc-err">{bad}</span>}
      </Row>}
      {!f.proxyMode && <Row label="From another device" hint="Until a proxy is set up, forward the port over Secure Shell (SSH)." top>
        <Command text={fwd} />
        <span class="svc-hint">Then open <span class="mono">http://127.0.0.1:{f.port}</span> on that device.</span>
      </Row>}
    </div>
    <div class="svc-save">
      <button type="submit" class="btn primary" disabled={!dirty || !!bad || !!badPort}>Save</button>
      {dirty && <button type="button" class="btn ghost" onClick={() => setEdits({})}>Discard</button>}
      <span class="muted" style="font-size:var(--text-sm)">
        {!dirty ? 'No unsaved changes.' : bad || badPort ? 'Fix the marked field to save.' : 'Saving closes open web connections. They reconnect on their own.'}
      </span>
    </div>
  </form>;
}
function StallWatch({ s }: ScreenProps) {
  const set = s.settings; const off = live.on ? set.stallWatch === false : !set.stallWatch;
  return <div class="svc-rows">
    <Row label="Stall watch" hint="The part of the service that keeps every agent moving without spending its turns. It never decides what the work is.">
      <Switch on={set.stallWatch} label="Stall watch" onChange={(v) => setS({ stallWatch: v })} />
    </Row>
    <div class={cx('svc-group', off && 'svc-off')}>
      <h3>Stall detection</h3>
      <Row label="Assigned, not started" id="sd-a">
        <Num id="sd-a" value={set.stallAssigned} min={1} max={120} unit="minutes" onChange={(v) => setS({ stallAssigned: v })} />
      </Row>
      <Row label="Worker, no output" hint={live.on ? 'The service stall time. Role files can override it.' : null} id="sd-w">
        <Num id="sd-w" value={set.stallWorker} min={live.on ? 5 : 1} max={live.on ? 240 : 120} unit="minutes" onChange={(v) => setS({ stallWorker: v })} />
      </Row>
      <Row label="Lead, no output" id="sd-l">
        <Num id="sd-l" value={set.stallLead} min={1} max={240} unit="minutes" onChange={(v) => setS({ stallLead: v })} />
      </Row>
      <Row label="Wake it once first" hint="A stuck agent gets one wake. Ten minutes later its lead is told. A stuck lead goes to the project lead, and a stuck project lead comes to you.">
        <Switch on={set.autoNudge} label="Wake it once first" onChange={(v) => setS({ autoNudge: v })} />
      </Row>
      <Row label="Replace the worker" id="sd-r" hint="After this many stalls in a row on the same work.">
        <Num id="sd-r" value={set.replaceAfter} min={1} max={9} unit="stalls" onChange={(v) => setS({ replaceAfter: v })} />
      </Row>
      <h3>Failure handling</h3>
      <Row label="Wait after a usage limit" id="sd-c">
        <Num id="sd-c" value={set.capacityBackoff} min={1} max={live.on ? 60 : 120} unit="minutes" onChange={(v) => setS({ capacityBackoff: v })} />
      </Row>
      <Row label="Try again" id="sd-cr" hint="Then the agent stops and the failure comes to you.">
        <Num id="sd-cr" value={set.capacityRetries} min={0} max={live.on ? 12 : 99} unit="times" onChange={(v) => setS({ capacityRetries: v })} />
      </Row>
      <Row label="Restart after a crash" id="sd-x">
        <Num id="sd-x" value={set.crashRetries} min={0} max={live.on ? 1 : 9} unit="times" onChange={(v) => setS({ crashRetries: v })} />
      </Row>
      <Row label="Close an idle team" id="sd-i" hint="A team with no work for this long is stopped. Its branch stays.">
        <Num id="sd-i" value={set.teamIdle} min={live.on ? 15 : 5} max={1440} unit="minutes" onChange={(v) => setS({ teamIdle: v })} />
      </Row>
      <h3>Approvals</h3>
      <Row label="Standing permission rules" hint={'"Always allow" stores a rule. Later requests that match it are allowed without asking.'}>
        <Switch on={set.standingRules} label="Standing permission rules" onChange={(v) => setS({ standingRules: v })} />
      </Row>
    </div>
  </div>;
}
function WorkflowSettings({ s }: ScreenProps) {
  const set = s.settings; const [checked, setChecked] = useState<string | number | null>(null);
  return <div class="svc-rows">
    <Row label="Workflow" hint="LGTM Studio ships one workflow. Override it for every repository, or for one.">
      <span class="row"><b style="font-weight:var(--weight-medium)">Build loop</b><span class="badge outline">Default</span></span>
    </Row>
    <Row label="Your overrides" id="wf-path" hint="Files here replace the default workflow for every repository." top>
      <span class="row">
        <input id="wf-path" class="input mono grow" value={set.workflowPath} onInput={(e) => setS({ workflowPath: e.currentTarget.value })} spellcheck={'false' as unknown as boolean} />
        <button type="button" class="btn" onClick={() => { if (live.on) return refuse(); setChecked('loading'); setTimeout(() => setChecked(Date.now()), 600); }}>
          {checked === 'loading' ? 'Checking…' : 'Check again'}
        </button>
      </span>
      {live.on ? <pre class="code">{set.roleFile ? JSON.stringify(set.roleFile, null, 2) : ''}</pre> : <>
        <ul class="svc-lines" style="margin-top:8px">
          <li>
            <Icon name="circle" size={14} class="faint" />
            <span class="mono">{set.workflowPath}/workflow.md</span>
            <span class="muted">Not found. The default is used.</span>
          </li>
          <li>
            <Icon name="circle-check" size={14} class="success-text" />
            <span class="mono">{set.workflowPath}/roles/reviewer.md</span>
            <span class="muted">Replaces the Reviewer's instructions.</span>
          </li>
        </ul>
        {checked && checked !== 'loading' && <span class="svc-hint">Checked at {clock(checked as number)}. Nothing changed.</span>}
      </>}
    </Row>
    <Row label="One repository" hint="A .lgtm folder in a repository wins over your overrides." top>
      <ul class="svc-lines">
        {s.repos.map((r) => <li key={r.id}>
        <Icon name="folder-git-2" size={14} class="faint" />
        <a class="link mono" href={href('/r/' + r.id)}>{r.name}</a>
        <span class="muted">{live.on ? '' : r.id === 'billing-api' ? '.lgtm/workflow.md turns agent review off' : 'No .lgtm folder'}</span>
      </li>)}
      </ul>
    </Row>
    <Row label="Write tests first" hint="New projects start with tests on. A project lead can turn them off when you say so.">
      <Switch on={set.testsDefault} label="Write tests first" onChange={(v) => setS({ testsDefault: v })} />
    </Row>
    <Row label="Agent review" hint="Reviewers score each change in rounds.">
      <Switch on={set.reviewDefault} label="Agent review" onChange={(v) => setS({ reviewDefault: v })} />
    </Row>
    <Row label="Review rounds" id="wf-r" hint="A minor finding scores 1 point, a major 3, and a blocker 9.">
      <Num id="wf-r" value={set.maxRounds} min={1} max={12} unit="at most" onChange={(v) => setS({ maxRounds: v })} />
    </Row>
  </div>;
}
// The server reaches GitHub through a GitHub App. The owner decided on 2026-11-24 that agents push
// and open pull requests as the app's bot unless the owner chooses "me", here or for one project.
const GITHUB_AS: [id: string, label: string][] = [['bot', 'The bot'], ['me', 'Me']];
// Connections lists what the server is connected to, after the Connections pages of Linear and
// Lindy: one row for each account, with what uses it and a way to remove it.
function Connections({ s }: ScreenProps) {
  const as = (s.settings.githubAs as string) || 'bot'; const accounts = accountsOf(s);
  const more = Object.keys(GITHUB).some((a) => !accounts.includes(a));
  const tracked = (a: string) => s.repos.filter((r) => r.remote?.startsWith(`github.com/${a}/`));
  const remove = (a: string) => { const n = tracked(a).length;
    act.confirm({ title: `Remove ${a}?`, body: n ? `${plural(n, 'repository', 'repositories')} of ${a} ${n === 1 ? 'stays' : 'stay'} on the server. Agents can no longer push to ${n === 1 ? 'it' : 'them'}.` : `The server can no longer read the repositories of ${a}.`, action: 'Remove', danger: true,
      onConfirm: () => update((st) => { st.ui.github = (st.ui.github || []).filter((x) => x !== a); }) }); };
  return <div class="svc-rows">
    <div class="cn-block">
      <div class="cn-head">
        <h3>GitHub</h3>
        {(more || !accounts.length) && <button type="button" class="btn sm" onClick={() => connectGitHub()}><Icon name="plus" size={13} />{accounts.length ? 'Connect another account' : 'Connect GitHub'}</button>}
      </div>
      <table class="table cn-table">
        <thead><tr><th>Account</th><th>Access</th><th class="num">Repositories here</th><th /></tr></thead>
        <tbody>
          {accounts.length === 0 && <tr><td colspan={4} class="muted" data-empty>Not connected</td></tr>}
          {accounts.map((a) => { const g = GITHUB[a]!; const n = tracked(a).length;
            return <tr key={a}>
              <td><span class="mono">{a}</span> <span class="muted">{g.kind}</span></td>
              <td>{g.all ? 'All repositories' : plural(g.repos.length, 'repository', 'repositories')}</td>
              <td class="num">{n || '—'}</td>
              <td class="cn-act"><button type="button" class="btn sm ghost" aria-label={`Remove ${a}`} onClick={() => remove(a)}>Remove</button></td>
            </tr>; })}
        </tbody>
      </table>
    </div>
    <Row label="Bot"><span class="mono">{accounts.length ? 'lgtm-studio-mayaokafor[bot]' : '—'}</span></Row>
    <Row label="Pushes and pull requests appear as">
      <div class="seg" role="group" aria-label="Pushes and pull requests appear as">
        {GITHUB_AS.map(([id, label]) => <button type="button" key={id} aria-pressed={as === id} onClick={() => setS({ githubAs: id })}>{label}</button>)}
      </div>
    </Row>
  </div>;
}
function GitMessages({ s }: ScreenProps) {
  const set = s.settings; const m = set.gitModel || (live.on ? '' : 'Haiku 5.5'); const h = harnessOfModel(m) || (live.on ? '' : 'claude');
  const usable = new Set(s.subscriptions.filter(usableSubscription).map((x) => x.harness));
  const signed = new Set(s.subscriptions.filter((x) => x.signedIn).map((x) => x.harness));
  return <div class="svc-rows">
    <Row label="Model" id="gm-model" hint="Writes commit messages from the staged diff, and pull request titles and descriptions from the commits.">
      <select id="gm-model" class="select" style="width:240px" value={m} onChange={(e) => setS({ gitModel: e.currentTarget.value })}>
        {live.on && <option value="" />}
        {Object.entries(HARNESSES).map(([id, x]) => <optgroup label={x.name + (signed.has(id) ? '' : usable.has(id) ? ' (Not reported)' : ' (signed out)')}>
          {x.models.map((mo) => <option value={mo} disabled={!usable.has(id)}>{mo}</option>)}
        </optgroup>)}
      </select>
    </Row>
    <Row label="Runs on"><span>{HARNESSES[h]?.name || ''}</span></Row>
    <Row label="Commit message">
      <span class="row" style="gap:6px">
        <span class="badge outline">Conventional commit</span>
        <span class="badge outline">One line</span>
        <span class="badge outline">No attribution</span>
      </span>
    </Row>
    <Row label="Pull request title" id="gm-limit">
      <span class="row" style="gap:6px"><span class="num">{act.PR_TITLE_LIMIT}</span><span class="muted">characters at most</span></span>
    </Row>
    <Row label="Example" top>
      <pre class="code svc-example">
        {live.on ? '' : act.commitMessage(['packages/web/src/shell/Sidebar.tsx', 'packages/web/src/shell/NavRail.tsx'])}
      </pre>
    </Row>
  </div>;
}
function About({ s }: ScreenProps) {
  const set = s.settings; const [chk, setChk] = useState<string | null>(null); const master = s.machines.find((m) => m.role === 'master');
  return <div class="svc-rows">
    <Row label="Version">
      <span class="row">
        <b style="font-weight:var(--weight-medium)">LGTM Studio {set.version}</b>
        <span class="tag">{set.commit}</span>
        <button type="button" class="btn sm" onClick={() => { if (act.refused()) return; setChk('loading'); setTimeout(() => setChk('done'), 900); }}>
          {chk === 'loading' ? 'Checking…' : 'Check for an update'}
        </button>
        {chk === 'done' && <span class="row success-text" style="gap:4px;font-size:var(--text-sm)"><Icon name="check" size={13} />Up to date</span>}
      </span>
    </Row>
    <Row label="Service">
      <span class="row" style="gap:6px">
        <i class={cx('dot', s.conn.status === 'live' ? 'working' : 'waiting')} />
        {s.conn.status === 'live' ? (live.on ? 'Running' : `Running on ${master!.name}`) : 'Not answering'}
        <span class="muted">· Node.js 26</span>
      </span>
    </Row>
    <Row label="Data folder"><span class="mono">{set.dataFolder}</span></Row>
    <Row label="Machines">
      <a class="link" href={href('/machines')}>{s.machines.filter((m) => m.status === 'online').length} of {s.machines.length} online</a>
    </Row>
    <Row label="Licence"><span>{live.on ? '' : 'Apache License 2.0'}</span></Row>
    <Row label="Restart the service" hint="Agents keep their place. Web connections close and reconnect.">
      <button type="button" class="btn" onClick={() => act.confirm({ title: 'Restart the service?', body: 'Agents stop at the end of their turn and resume after the restart. This page reconnects on its own.', action: 'Restart', danger: false,
        onConfirm: () => { if (act.refused()) return; act.setConnection('reconnecting'); act.sampleLater(() => { act.setConnection('live'); act.toast('The service restarted'); }, 2400); } })}>
        <Icon name="rotate-ccw" size={13} />
        Restart
      </button>
    </Row>
  </div>;
}
const SECTIONS: [id: string, label: string, icon: string, body: ComponentType<ScreenProps>, sub: string][] = [
  ['appearance', 'Appearance', 'palette', Appearance, 'Theme, density and sidebar.'],
  ['defaults', 'Defaults', 'sliders-horizontal', Defaults, 'Starting settings for new agents.'],
  ['remote', 'Remote access', 'wifi', Remote, 'Access from other devices.'],
  ['stall-watch', 'Stall watch', 'timer', StallWatch, 'Detection of stalled agents.'],
  ['workflow', 'Workflow', 'workflow', WorkflowSettings, 'The build loop and its overrides.'],
  ['git', 'Git messages', 'git-commit-horizontal', GitMessages, 'Model for commit and pull request messages.'],
  ['connections', 'Connections', 'plug', Connections, 'Connected accounts and services.'],
  ['about', 'About', 'info', About, 'Version and service status.'],
];
// The owner asked on 2026-11-24 to move Machines, Harnesses, Notifications and Workflows into Settings.
// Each keeps its own address and opens beside the same list as the sections of the service's settings.
const SETTINGS_PAGES: [id: string, label: string, icon: string][] = [['machines', 'Machines', 'server'], ['harnesses', 'Harnesses', 'key-round'], ['notifications', 'Notifications', 'smartphone'], ['workflows', 'Workflows', 'workflow']];
/** Settings: one list of its pages and sections on the left, and the open one beside it. */
export function SettingsShell({ s, current, children }: { s: State; current: string; children: ComponentChildren }) {
  const failed = (s.notifications || []).filter((x) => x.delivery === 'failed').length;
  return <div class="view-body">
    <div class="svc-settings">
      <nav class="svc-nav" aria-label="Settings">
        {SETTINGS_PAGES.map(([id, label, icon]) => <a key={id} class="nav-item" href={href('/' + id)} aria-current={current === id ? 'page' : undefined}>
          <Icon name={icon} size={15} />
          <span>{label}</span>
          {id === 'machines' && <span class="muted svc-nav-n">{s.machines.filter((m) => m.status === 'online').length} of {s.machines.length} online</span>}
          {id === 'notifications' && failed > 0 && <span class="count error">{failed}</span>}
        </a>)}
        <span class="svc-nav-sep" />
        {SECTIONS.map(([id, label, icon]) => <a key={id} class="nav-item" href={href('/settings?section=' + id)} aria-current={current === id ? 'page' : undefined}>
          <Icon name={icon} size={15} />
          <span>{label}</span>
        </a>)}
      </nav>
      {children}
    </div>
  </div>;
}
export function ServiceSettings({ s, route }: { s: State; route: Route }) {
  useEffect(() => liveGit.openScreen('settings'), []);
  const want = route.query.get('section');
  useEffect(() => { if (want === 'notifications') go('/notifications'); }, [want]);
  // The GitHub section is drawn on sample data only. The service has no GitHub connection yet.
  const sections = SECTIONS.filter((x) => !live.on || x[0] !== 'github');
  const cur = sections.find((x) => x[0] === want) || sections[0]!; const Body = cur[3];
  if (!live.on) return <SettingsShell s={s} current={cur[0]}>
    <div class="svc-pane" data-main>
      <div class="page-head"><h1 class="page-title">{cur[1]}</h1><span class="sub">{cur[4]}</span><Saved s={s} /></div>
      <Body key={cur[0]} s={s} />
    </div>
  </SettingsShell>;
  return <div class="view-body">
    <div class="svc-settings">
      <nav class="svc-nav" aria-label="Settings sections">
        {sections.map(([id, label, icon]) => <a key={id} class="nav-item" href={href('/settings?section=' + id)} aria-current={cur[0] === id ? 'page' : null!}>
      <Icon name={icon} size={15} />
      <span>{label}</span>
    </a>)}
        <span class="svc-nav-sep" />
        <a class="nav-item" href={href('/notifications')}>
          <Icon name="smartphone" size={15} />
          <span>Notifications</span>
          <Icon name="arrow-up-right" size={12} class="faint" />
        </a>
      </nav>
      <div class="svc-pane" data-main>
        <div class="page-head"><h1 class="page-title">{cur[1]}</h1><span class="sub">{cur[4]}</span><Saved s={s} /></div>
        <fieldset aria-busy={live.on && (liveGit.busy('settings') || liveGit.busy('read-service'))} style="border:0;padding:0;margin:0;min-width:0">
          {live.on && liveGit.busy('settings') && <span role="status">Saving…</span>}
          {(!live.on || s.ui.serviceReady || cur[0] === 'appearance') && <Body key={cur[0]} s={s} />}
        </fieldset>
      </div>
    </div>
  </div>;
}

// ---- one project's settings --------------------------------------------------------
export const ProjectSettingRow = ({ label, hint, children, id, top }: RowProps) => <div class={cx('svc-row', top && 'top')}>
  <div class="svc-label">
    <span class="setting-label"><label for={id}>{label}</label>{hint && <SettingHelp label={label}>{hint}</SettingHelp>}</span>
  </div>
  <div class="svc-control">{children}</div>
</div>;
/** A long path or branch is cut at its start, and this mark keeps its letters in reading order. */
const ltr = (text: string | undefined) => '\u200e' + (text || '');
/**
 * The settings of one project. `page` shows one of its two pages: 'general' is the project itself
 * and where its code is, 'agents' is who works on it and what they may do. With no page, every
 * section shows on one page, which is what the older live layout uses.
 */
export function ProjectSettings({ s, project: p, page }: { s: State; project: Project; page?: 'general' | 'agents' }) {
  const general = !page || page === 'general'; const agentsPage = !page || page === 'agents';
  const Row = ProjectSettingRow;
  const r = repoOf(s, p); const m = machineOf(s, p.machine); const lead = agentOf(s, p.lead); const ags = agentsOfProject(s, p.id); const teams = teamsOf(s, p.id);
  const [goal, setGoal] = useState(p.goal);
  const [name, setName] = useState(p.name);
  useEffect(() => setName(p.name), [p.id, p.name]);
  useEffect(() => { if (live.on) void board.loadAgents([p.lead]); }, [p.id, p.lead]);
  const choices = s.boardAgents?.[p.lead];
  const projectBusy = live.on && board.busy('project:' + p.id);
  const configBusy = live.on && board.busy('configure:' + p.lead);
  useEffect(() => setGoal(p.goal), [p.id]);
  const perms = [...new Set(ags.map((a) => a.permission))]; const perm = perms.length === 1 ? perms[0] : null;
  const set = (patch: Partial<Project>) => { if (live.on) { refuse(); return; } update(() => { Object.assign(p, patch); }); markSaved(); };
  const branches = live.on ? [] : [...new Set([r.main, 'develop', 'release/next'])];
  const subs = s.subscriptions.filter((x) => x.machine === p.machine || x.id === lead.subscription);
  const running_ = ags.filter(running).length;
  // The name saves when the field loses the cursor, or on Enter. An empty or unchanged name is put back.
  const rename = () => { const value = name.trim(); if (!value || value === p.name) { setName(p.name); return; } if (live.on) return board.renameProject(p.id, value); if (!value || s.projects.some((x) => x.id !== p.id && x.repo === p.repo && x.name.toLowerCase() === value.toLowerCase())) return act.toast('Choose a unique project name.', 'error'); update(() => { p.name = value; }); act.toast('Project renamed'); };
  const changePermission = (permission: string, setting: string) => {
    if (live.on) return boardLive.changePermission(p.id, permission, setting);
    update(() => { p.permissions = { ...p.permissions, [permission]: { setting, granted: false } }; });
    act.toast('Lead permission saved');
  };
  const changeApproval = (approval: string, approved: boolean) => {
    if (live.on) return boardLive.changeApproval(p.id, approval, approved);
    const permission = approval === 'build' ? 'starting-work' : 'planning';
    update(() => { p.permissions = { ...p.permissions, [permission]: { ...p.permissions?.[permission], granted: approved } }; });
    act.toast(approved ? 'Permission granted' : 'Permission withdrawn');
  };
  const deleteTeam = (team: Team) => act.confirm({ title: `Delete ${teamName(team)}?`, waitForReply: live.on, action: 'Delete team', danger: true,
    body: 'Stop its agents and delete or move its tasks first. Its board channel becomes read-only history. Its branch and worktree folder are kept.',
    onConfirm: () => { if (live.on) return boardLive.deleteTeam(p.id, team.id); if (agentsOfTeam(s, team.id).some(running)) return act.toast('Stop the team’s agents before deleting the team.', 'error'); if (s.tasks.some((x) => x.team === team.id)) return act.toast('Delete or move the team’s tasks before deleting the team.', 'error'); update((st) => { st.teams = st.teams.filter((x) => x.id !== team.id); for (const c of st.channels.filter((x) => x.team === team.id)) c.archived = true; }); act.toast('Team deleted'); return true; } });
  const archive = () => act.confirm({ title: `Archive ${p.name}?`, waitForReply: live.on, pendingLabel: 'Archiving…', body: 'The project leaves the sidebar and its agents stop. Its branch, worktrees and threads are kept, and you can restore it.', action: 'Archive project', danger: false,
    lost: [running_ ? `${plural(running_, 'running agent')} will stop` : null, openRequests(s, { project: p.id }).length ? `${plural(openRequests(s, { project: p.id }).length, 'open request')} will be hidden` : null].filter(Boolean) as string[],
    onConfirm: async () => {
      const scope = server.captureSession();
      const reply = await act.archiveProject(p.id);
      if (store.state !== s || (scope && !scope.isCurrent())) return false;
      if (!live.on || reply) go('/r/' + r.id);
      return reply;
    } });
  const del = () => act.confirm({ title: `Delete ${p.name}?`, waitForReply: live.on, pendingLabel: 'Deleting…', danger: true,
    body: 'The project, its stopped agents, threads, private channels, sprints and tasks disappear from all views. Its open review, questions and requests go too. Its agents’ shared-channel posts go. Other authors’ posts and replies remain stored in the channel. Posts with references to this project no longer appear in channel reads. Those references no longer open. Its exclusive worktree registrations and their children go. Close terminals in those worktrees first. Removal stops at the repository checkout and worktrees named by other projects or teams. Unrelated registrations stay. Design registrations stay recorded. Their files cannot be opened through the engine if their worktree registration goes. Stored events stay in the log. The worktree folders and every branch stay on disk. This cannot be undone.', action: 'Delete project',
    onConfirm: async () => { if (live.on) return; if (!p.archived) return false;
      const id = p.id;
      update((st) => { const ch = new Set(st.channels.filter((c) => c.project === id).map((c) => c.id));
        st.projects = st.projects.filter((x) => x.id !== id);
        const lists = st as unknown as Record<string, { project: Id }[]>;
        for (const k of ['agents', 'teams', 'sprints', 'tasks', 'requests', 'channels']) lists[k] = lists[k]!.filter((x) => x.project !== id);
        st.posts = st.posts.filter((x) => !ch.has(x.channel)); st.notifications = st.notifications.filter((n) => n.project !== id); });
      go('/r/' + r.id); act.toast(`Deleted ${p.name}`); return true; } });
  const signed = new Set(s.subscriptions.filter((x) => usableSubscription(x) && x.enabled).map((x) => x.harness));
  const asg: ProjectAssignment = p.assignment || { state: 'proposed', rows: proposeAssignment(s), changed: [] };
  const saveAsg = (next: ProjectAssignment) => { if (live.on) { refuse(); return; } update(() => { p.assignment = next; }); markSaved(); };
  const setRow = (k: string, patch: Partial<ProjectAssignment['rows'][string]>) => { const row = { ...asg.rows[k]!, ...patch };
    if (patch.harness) { row.model = HARNESSES[patch.harness]!.models[0]!; }
    row.effort = hasEffort(row.harness, row.model) ? row.effort || 'Medium' : null;
    saveAsg({ ...asg, rows: { ...asg.rows, [k]: row }, changed: [...new Set([...(asg.changed || []), k])] }); };
  const [allModel, setAllModel] = useState(() => live.on ? '' : asg.rows.Plan!.model);
  const applyAll = () => { if (live.on) { refuse(); return; } const h = harnessOfModel(allModel)!; const rows: ProjectAssignment['rows'] = {}; for (const k of WORK_KINDS) rows[k] = { harness: h, model: allModel, effort: hasEffort(h, allModel) ? asg.rows[k]!.effort || 'Medium' : null };
    saveAsg({ ...asg, rows, changed: [...WORK_KINDS] }); act.toast(`Every row uses ${allModel}`); };
  const approve = () => { saveAsg({ ...asg, state: 'approved' }); act.toast('Assignment approved. The project lead was told.'); };
  const applyPerm = (v: string) => { update((st) => { for (const a of st.agents) if (a.project === p.id) a.permission = v; }); markSaved(); };
  // Full access lets every agent run commands unasked, so it is confirmed. Every other mode is set at once.
  const setPerm = (v: string) => { if (live.on) { refuse(); return; } if (v !== 'Full access') { applyPerm(v); return; }
    act.confirm({ title: `Give full access to ${plural(ags.length, 'agent')}?`, body: PERMISSION_HINT[v]!, action: 'Give full access', danger: true, onConfirm: () => { applyPerm(v); } }); };
  const setTeam = (t: Team, patch: { harness?: string; model?: string; needToKnow?: boolean }) => { if (live.on) { refuse(); return; } update((st) => { for (const a of agentsOfTeam(st, t.id)) { if (patch.harness && patch.harness !== a.harness) { const hs = HARNESSES[patch.harness]!; const sub = st.subscriptions.find((x) => x.harness === patch.harness && usableSubscription(x) && x.machine === p.machine) || st.subscriptions.find((x) => x.harness === patch.harness && usableSubscription(x)); a.harness = patch.harness; a.subscription = sub ? sub.id : a.subscription; a.model = hs.models[0]!; a.effort = 'Medium'; } if (patch.model) a.model = patch.model; } if ('needToKnow' in patch) byId(st.teams, t.id)!.needToKnow = patch.needToKnow; }); markSaved(); };
  const leadRow = <tr key="lead">
    <td><b style="font-weight:var(--weight-medium)">Lead</b></td>
    <td>
      <select id="ps-sub" class="select" aria-label="Lead subscription" value={lead.subscription!} onChange={(e) => { if (live.on) { refuse(); return; } const x = subOf(s, e.currentTarget.value)!; update(() => { lead.subscription = x.id; if (lead.harness !== x.harness) { lead.harness = x.harness; lead.model = HARNESSES[x.harness]!.models[0]!; lead.effort = hasEffort(x.harness, lead.model) ? lead.effort || 'High' : null; } }); markSaved(); }}>
        {subs.map((x) => <option value={x.id} disabled={!usableSubscription(x)}>
          {subLabel(s, x.id)}
          {x.signInKnown === false ? ' (Not reported)' : !x.signedIn ? ' (signed out)' : x.usage ? ` · ${x.usage.pct}% used` : ''}
        </option>)}
      </select>
    </td>
    <td>
      <select class="select" aria-label="Lead model" disabled={configBusy} value={lead.model} onChange={(e) => { const v = e.currentTarget.value; if (live.on) return board.configureLead(p.lead, { model: v }); update(() => { lead.model = v; lead.effort = hasEffort(lead.harness, v) ? lead.effort || 'High' : null; }); markSaved(); }}>
        {(live.on ? choices?.models || [] : HARNESSES[lead.harness]!.models.map((id) => ({ id, label: id }))).map((mo: { id: string; label: string }) => <option value={mo.id}>{mo.label}</option>)}
      </select>
    </td>
    <td>
      {(live.on ? choices?.efforts.length > 0 : hasEffort(lead.harness, lead.model))
        ? <select class="select" aria-label="Lead effort" disabled={configBusy} value={lead.effort || 'High'} onChange={(e) => { const v = e.currentTarget.value; if (live.on) return board.configureLead(p.lead, { effort: v }); update(() => { lead.effort = v; }); markSaved(); }}>
          {(live.on ? choices?.efforts || [] : HARNESSES[lead.harness]!.efforts.map((id) => ({ id, label: id }))).map((x: { id: string; label: string }) => <option value={x.id}>{x.label}</option>)}
        </select>
        : <span class="muted">None</span>}
    </td>
  </tr>;
  const modes = <dl class="ps-modes">{PERMISSION_MODES.map((mode) => <div key={mode}><dt>{mode}</dt><dd>{PERMISSION_HINT[mode]}</dd></div>)}</dl>;
  return <div class="page svc-project" aria-busy={projectBusy || configBusy}>
    {p.archived && <div class="callout" style="align-items:center;margin-bottom:var(--space-3);max-width:860px">
      <Icon name="archive" size={15} />
      <span class="grow">This project is archived. Its agents are stopped.</span>
      <button type="button" class="btn sm" aria-busy={projectBusy} disabled={projectBusy} onClick={() => act.archiveProject(p.id, false)}>
        <Icon name="archive-restore" size={13} />
        Restore
      </button>
    </div>}
    {general && <Section title="Project">
      <div class="svc-rows">
        <Row label="Name" id="ps-name" hint="The branch and the worktree folder keep their names.">
          <input id="ps-name" class="input" maxlength={128} value={name} disabled={projectBusy} onInput={(e) => setName(e.currentTarget.value)} onBlur={rename}
            onKeyDown={(e) => { if (e.key === 'Enter') e.currentTarget.blur(); else if (e.key === 'Escape' && name !== p.name) { e.stopPropagation(); setName(p.name); } }} />
        </Row>
        <Row label="Goal" id="ps-goal" hint="The project lead reads this at the start of every turn." top>
          <textarea id="ps-goal" class="textarea" rows={3} value={goal} onInput={(e) => setGoal(e.currentTarget.value)} />
          {goal.trim() !== p.goal && <span class="row" style="margin-top:6px">
            <button type="button" class="btn sm primary" disabled={!goal.trim() || projectBusy} aria-busy={projectBusy} onClick={() => { if (live.on) return board.saveGoal(p.id, goal.trim()); set({ goal: goal.trim() }); act.toast('Goal saved. The project lead was told.'); }}>
              Save goal
            </button>
            <button type="button" class="btn sm ghost" onClick={() => setGoal(p.goal)}>Discard</button>
          </span>}
        </Row>
      </div>
    </Section>}
    {general && <Section title="Code">
      {/* What cannot be changed here is one line of facts, not four rows. */}
      <dl class="ps-facts">
        <div><dt>Repository</dt><dd><a class="link mono" href={href('/r/' + r.id)}>{r.name}</a></dd></div>
        <div><dt>Branch</dt><dd class="mono truncate-start" title={p.branch}>{ltr(p.branch)}</dd></div>
        <div><dt>Folder</dt><dd class="mono truncate-start" title={p.folder}>{ltr(p.folder)}</dd></div>
        <div><dt>Identifiers</dt><dd class="mono">{repoPrefix(s, r)}-{p.prefix}</dd></div>
      </dl>
      <div class="svc-rows">
        <Row label="Merges into" id="ps-base" hint="Finished work merges into this branch, with your approval.">
          <select id="ps-base" class="select mono" value={p.base || r.main} onChange={(e) => set({ base: e.currentTarget.value })}>
            {branches.map((b) => <option value={b}>{b}{b === r.main ? ' (repository default)' : ''}</option>)}
          </select>
        </Row>
        {!live.on && <Row label="On GitHub, appear as" id="ps-github" hint="Who pushes and opens pull requests for this project.">
          <select id="ps-github" class="select" value={(p.githubAs as string) || ''} onChange={(e) => set({ githubAs: e.currentTarget.value || undefined })}>
            <option value="">Service setting ({(s.settings.githubAs as string) === 'me' ? 'me' : 'the bot'})</option>
            {GITHUB_AS.map(([id, label]) => <option key={id} value={id}>{label}</option>)}
          </select>
        </Row>}
        <Row label="Runs on" id="ps-machine" hint="New agents start here. Running agents stay where they are.">
          <select id="ps-machine" class="select" value={p.machine} onChange={(e) => { const v = e.currentTarget.value; set({ machine: v, folder: `${r.folders[v]}/.worktrees/${p.branch.split('/').pop()}` }); }}>
            {s.machines.filter((x) => !live.on || x.id === 'local').map((x) => <option value={x.id} disabled={x.status !== 'online' || !r.folders[x.id]}>
              {x.name}
              {x.status !== 'online' ? ' (offline)' : !r.folders[x.id] ? ` (${r.name} is not on it)` : ''}
            </option>)}
          </select>
          {m.status !== 'online' && <span class="svc-err">{m.name} is offline.</span>}
        </Row>
      </div>
    </Section>}
    {agentsPage && <Section title="Models" sub={live.on ? null : asg.state === 'approved' ? 'Approved by you' : <span class="warn-text">Proposed by the project lead</span>} action={asg.state !== 'approved' && <button type="button" class="btn sm primary" onClick={approve}><Icon name="check" size={13} />Approve assignment</button>}>
      <div class="table-wrap svc-assign">
        <table class="table">
          <thead><tr><th>Work</th><th>Harness</th><th>Model</th><th>Effort</th></tr></thead>
          <tbody>
            {leadRow}
            {(live.on ? [] : WORK_KINDS).map((k) => { const row = asg.rows[k]!; const h = HARNESSES[row.harness]!;
          return <tr key={k} class={cx(asg.changed?.includes(k) && 'changed')}>
            <td><b style="font-weight:var(--weight-medium)">{k}</b></td>
            <td>
              <select class="select" aria-label={k + ' harness'} value={row.harness} onChange={(e) => setRow(k, { harness: e.currentTarget.value })}>
                {Object.entries(HARNESSES).map(([id, x]) => <option value={id} disabled={!signed.has(id)}>{x.name}{signed.has(id) ? '' : ' (signed out)'}</option>)}
              </select>
            </td>
            <td>
              <select class="select" aria-label={k + ' model'} value={row.model} onChange={(e) => setRow(k, { model: e.currentTarget.value })}>
                {h.models.map((mo) => <option value={mo}>{mo}</option>)}
              </select>
            </td>
            <td>
              {hasEffort(row.harness, row.model)
              ? <select class="select" aria-label={k + ' effort'} value={row.effort || 'Medium'} onChange={(e) => setRow(k, { effort: e.currentTarget.value })}>
                {h.efforts.map((x) => <option value={x}>{x}</option>)}
              </select>
              : <span class="muted">None</span>}
            </td>
          </tr>; })}
          </tbody>
        </table>
      </div>
      {!live.on && <div class="svc-assign-all row">
        <select id="ps-all" class="select" aria-label="A model for every kind of work" value={allModel} onChange={(e) => setAllModel(e.currentTarget.value)}>
          {Object.entries(HARNESSES).filter(([id]) => signed.has(id)).map(([id, x]) => <optgroup label={x.name}>{x.models.map((mo) => <option value={mo}>{mo}</option>)}</optgroup>)}
        </select>
        <button type="button" class="btn sm" onClick={applyAll}>Use for Plan, Build, Review and Test</button>
      </div>}
    </Section>}
    {agentsPage && <Section title="Permissions">
      <div class="svc-rows">
        <Row label="Agents may" id="ps-perm" hint={modes}>
          <select id="ps-perm" class="select" value={perm || ''} onChange={(e) => { const v = e.currentTarget.value; e.currentTarget.value = perm || ''; setPerm(v); }}>
            {!perm && <option value="" disabled>Differs between agents</option>}
            {PERMISSION_MODES.map((mode) => <option value={mode}>{mode}</option>)}
          </select>
        </Row>
        {[
          ['planning', 'Project planning', 'Lead may plan', 'The lead can create sprints, teams and tasks.'],
          ['starting-work', 'Starting work', 'Lead may start work', 'The lead can start workers and move tasks out of the queue.']
        ].map(([permission, name_, label, help]) => {
          const value = p.permissions?.[permission!] || { setting: 'allowed', granted: false };
          return <Row key={permission} label={label!} id={'ps-permission-' + permission} hint={help}>
            <span class="row ps-grant">
              <select id={'ps-permission-' + permission} class="select" aria-label={name_ + ' permission'} value={value.setting} disabled={projectBusy} onChange={(e) => changePermission(permission!, e.currentTarget.value)}>
                <option value="allowed">Allowed</option>
                <option value="denied">Not allowed</option>
                <option value="ask">Always ask for permission</option>
              </select>
              {value.setting === 'ask' && <>
                <span role="status" class="muted" aria-label={name_ + ' grant'}>{value.granted ? 'Granted' : 'Not granted'}</span>
                {value.granted && <button type="button" class="btn sm" aria-label={'Withdraw ' + name_} disabled={projectBusy} onClick={() => changeApproval(permission === 'planning' ? 'end-result' : 'build', false)}>
                  Withdraw
                </button>}
              </>}
            </span>
          </Row>;
        })}
      </div>
    </Section>}
    {agentsPage && <Section title="Teams" sub={teams.length ? null : 'None'}>
      {teams.length > 0 && <div class="table-wrap svc-teams">
        <table class="table">
          <thead><tr>
            <th>Team</th><th>Branch</th>
            <th><span class="setting-label">Need-to-know<SettingHelp label="Need-to-know">The team's agents see only the tasks and the posts that are theirs.</SettingHelp></span></th>
            <th class="c-act" aria-label="Delete" />
          </tr></thead>
          <tbody>
            {teams.map((t) => <tr key={t.id}>
            <td><a class="link" href={href(`/p/${p.id}/team/${t.id}`)}>{teamName(t)}</a></td>
            <td class="mono muted truncate-start" title={t.branch}>{ltr(t.branch)}</td>
            <td><Switch on={t.needToKnow} label={'Need-to-know for ' + teamName(t)} onChange={(v) => setTeam(t, { needToKnow: v })} /></td>
            <td class="c-act">
              <button type="button" class="icon-btn sm" aria-label={'Delete team ' + teamName(t)} data-tip="Delete team" data-tip-side="left" disabled={projectBusy} onClick={() => deleteTeam(t)}>
                <Icon name="trash-2" size={14} />
              </button>
            </td>
          </tr>)}
          </tbody>
        </table>
      </div>}
    </Section>}
    {general && <Section title="Archive and delete">
      <div class="svc-rows svc-danger">
        {p.archived
          ? <Row label="Restore this project" hint="It returns to the sidebar. Its agents stay stopped until you resume them.">
            <button type="button" class="btn" aria-busy={projectBusy} disabled={projectBusy} onClick={() => act.archiveProject(p.id, false)}>
              <Icon name="archive-restore" size={13} />
              Restore
            </button>
          </Row>
          : <Row label="Archive this project" hint="Stops its agents and hides it. Nothing is deleted.">
            <button type="button" class="btn" disabled={projectBusy} onClick={archive}><Icon name="archive" size={13} />Archive</button>
          </Row>}
        {/* Delete is always in sight. It is off until the project is archived, and its tip says so. */}
        <Row label="Delete this project" hint="Hides the project and its history. Stored events, worktree folders and branches are kept.">
          <span data-tip={p.archived ? undefined : 'Archive the project first'} data-tip-side="left">
            <button type="button" class="btn danger" disabled={projectBusy || !p.archived} onClick={del}>
              <Icon name="trash-2" size={13} />
              Delete project
            </button>
          </span>
        </Row>
      </div>
    </Section>}
  </div>;
}
