import * as liveGit from './live/git';
import { live, refuse } from './lib';
// Dialogs that belong to no single screen: the quick switcher, New project,
// the merge review, and provider sign-in.
import { useState, useEffect, useLayoutEffect, useRef } from 'preact/hooks';
import { Icon, cx, go, update, useKey } from './lib';
import {
  usableSubscription, projectOf, repoOf, agentOf, teamOf, teamName, machineOf, subOf, byId, HARNESSES, openRequests, agentStatus, projectStatus, routeForAgent,
} from './data';
import { homeTab } from './define';
import * as act from './actions';
import * as board from './live/board';
import * as mocks from './live/mocks';
import { useProjectLead } from './project-lead';
import { Dialog, registerDialog, Avatar, MergeFacts } from './ui';
import type { Agent, DialogState, Id, State } from './types';
import type { GitFile, GitReview } from './screens/git';

// The sample's merge request and the live review share these fields. Only the live review has the heads.
type ReviewView = Omit<GitReview, 'head' | 'baseHead'> & { head?: string; baseHead?: string };
// A commit file of the sample: a record, or a [path, , { add, del }] row.
type SampleFile = GitFile & { 0?: string; 2?: { add?: number; del?: number } };

// ---- quick switcher ---------------------------------------------------------------
// The real product ranks results with a small embedding model. The mock imitates it here,
// in one function: the query's words, stemmed and with filler words removed, are matched
// against each result's name and its wider text (a project's goal and sprints, a
// repository's description, an agent's role, team and activity). Results that cover more
// of the query rank higher; a word in the name counts more than a word in the text; a
// word that names a kind ("project", "agent", "repository") favours that kind; a project
// lead edges out other agents whose names match; and ties
// rank repositories, then projects, then agents.
const STOP = new Set('a an the of on in at to for and or is was were be been which what who where when how that this it its with from by my me i we our any some all was'.split(' '));
const KIND_WORDS: Record<string, string> = { repository: 'Repository', repositories: 'Repository', repo: 'Repository', repos: 'Repository', project: 'Project', projects: 'Project', agent: 'Agent', agents: 'Agent' };
const KIND_ORDER: Record<string, number> = { Repository: 0, Project: 1, Agent: 2, Page: 3 };
const stem = (w: string) => w.replace(/(ing|ed|es|s)$/, '').replace(/(.)\1$/, '$1');
const words = (t: string | null | undefined) => (t || '').toLowerCase().split(/[^a-z0-9]+/).filter((w) => w && !STOP.has(w));
/** One row of the quick switcher. */
export interface SearchItem {
  kind: string; label: string; sub: string; text: string; to: string; icon?: string; mono?: boolean; note?: string; tone?: string | null;
  boost?: number; agent?: Agent;
}
export function searchScore(query: string, item: Pick<SearchItem, 'label' | 'text' | 'kind'> & { boost?: number | undefined }): number {
  const qw = words(query); if (!qw.length) return 0;
  const kinds = qw.filter((w) => KIND_WORDS[w]).map((w) => KIND_WORDS[w]);
  const terms = qw.filter((w) => !KIND_WORDS[w]).map(stem); if (!terms.length && !kinds.length) return 0;
  const name = new Set(words(item.label).map(stem)); const text = new Set(words(item.text).map(stem));
  const has = (set: Set<string>, t: string) => set.has(t) || (t.length > 3 && [...set].some((w) => w.startsWith(t)));
  let covered = 0, weight = 0;
  for (const t of terms) { if (has(name, t)) { covered++; weight += 3; } else if (has(text, t)) { covered++; weight += 1; } }
  if (terms.length && !covered) return 0;
  const cover = terms.length ? covered / terms.length : 1;
  return cover * 10 + weight + (kinds.includes(item.kind) ? 4 : 0) + (weight >= 3 ? item.boost || 0 : 0);
}
function Switcher({ s }: { s: State }) {
  const [q, setQ] = useState(''); const [i, setI] = useState(0); const list = useRef<HTMLDivElement>(null);
  const [selectedTo, setSelectedTo] = useState<string>();
  const repositoryMode = s.server.repositoryMode;
  const all: SearchItem[] = [];
  const text = live.on ? mocks.searchText(s) : null;
  for (const r of s.repos) {
    const missingRemote = live.on ? '' : s.server.repositoryMode && r.remoteOmitted ? '—' : 'No remote';
    all.push({ kind: 'Repository', label: r.name, sub: r.remote || missingRemote, text: live.on ? text.repositories[r.id] : `${r.description || ''} ${r.remote || ''}`, to: '/r/' + r.id, icon: 'folder-git-2', mono: true });
  }
  for (const p of s.projects.filter((x) => !x.archived)) {
    const st = projectStatus(s, p); const sp = s.sprints.filter((x) => x.project === p.id);
    all.push({ kind: 'Project', label: p.name, sub: repoOf(s, p).name, text: live.on ? text.projects[p.id] : `${repoOf(s, p).name} ${p.goal || ''} ${p.branch || ''} ${sp.map((x) => x.name + ' ' + x.goal).join(' ')}`, note: st.tone === 'warn' ? st.word : p.stage, tone: st.tone, to: `/p/${p.id}/${homeTab()}`, icon: 'kanban' });
  }
  for (const a of s.agents) {
    const p = projectOf(s, a.project); if (!p || p.archived) continue; const st = agentStatus(s, a); const t = a.team && teamOf(s, a.team);
    all.push({ kind: 'Agent', label: a.name, sub: `${p.name}${t ? ' · ' + teamName(t) : ''}`, text: live.on ? text.agents[a.id] : `${a.role} ${t ? teamName(t) : ''} ${a.activity || ''} ${p.name} ${repoOf(s, p).name}`, boost: a.role === 'project-lead' ? 0.5 : 0, note: st.word, tone: st.tone, to: routeForAgent(s, a), agent: a });
  }
  for (const [to, label, icon] of [['/home', 'Home', 'house'], ['/projects', 'All projects', 'table-2'], ['/machines', 'Machines', 'server'], ['/harnesses', 'Harnesses', 'key-round'], ['/notifications', 'Notifications', 'smartphone'], ['/settings', 'Service settings', 'settings']]) all.push({ kind: 'Page', label, sub: '', text: '', to, icon });
  const ql = q.trim();
  const found = ql
    ? all.map((x): [number, SearchItem] => [searchScore(ql, x), x]).filter(([n]) => n > 0).sort((a, b) => b[0] - a[0] || KIND_ORDER[a[1].kind]! - KIND_ORDER[b[1].kind]!).map(([, x]) => x)
    : all.filter((x) => x.kind === 'Repository' || x.kind === 'Project' || (x.kind === 'Agent' && x.tone === 'warn'));
  const shown = found.slice(0, 14);
  const activeIndex = repositoryMode ? Math.max(0, shown.findIndex((x) => x.to === selectedTo)) : i;
  const active = shown[activeIndex];
  const watchedActiveTo = repositoryMode ? active?.to : undefined;
  useLayoutEffect(() => {
    if (repositoryMode && selectedTo !== watchedActiveTo) setSelectedTo(watchedActiveTo);
  }, [repositoryMode, selectedTo, watchedActiveTo]);
  useEffect(() => setI(0), [q]);
  useEffect(() => { list.current?.querySelector('[data-active="true"]')?.scrollIntoView({ block: 'nearest' }); }, [activeIndex, watchedActiveTo]);
  const search = (value: string) => {
    if (repositoryMode && value !== q) setSelectedTo(undefined);
    setQ(value);
  };
  const select = (index: number) => {
    if (repositoryMode) setSelectedTo(shown[index]?.to);
    else setI(index);
  };
  const pick = (x: SearchItem) => { act.closeDialog(); go(x.to); };
  const key = (e: KeyboardEvent) => {
    if (e.key === 'ArrowDown') { e.preventDefault(); select(Math.min(shown.length - 1, activeIndex + 1)); }
    else if (e.key === 'ArrowUp') { e.preventDefault(); select(Math.max(0, activeIndex - 1)); }
    else if (e.key === 'Enter' && active) { e.preventDefault(); pick(active); }
    else if (e.key === 'Escape') act.closeDialog();
  };
  return <div class="scrim" style="place-items:start center;padding-top:12vh" onPointerDown={(e) => { if (e.target === e.currentTarget) act.closeDialog(); }}>
    <div class="dialog" role="dialog" aria-modal="true" aria-label="Find project or agent" style="width:min(620px,100%)">
      <div class="search" style="padding:10px 12px;border-bottom:1px solid var(--color-line)">
        <Icon name="search" size={16} class="faint" />
        <input autofocus class="input lg" style="border:0;box-shadow:none;padding-left:34px;font-size:var(--text-lg)" placeholder="Search repositories, projects and agents" aria-label="Find project or agent" value={q} onInput={(e) => search(e.currentTarget.value)} onKeyDown={key} ref={(el) => { el && setTimeout(() => el.focus(), 0); }} />
        <kbd>Esc</kbd>
      </div>
      <div ref={list} style="overflow:auto;padding:4px;max-height:min(52vh,460px)" role="listbox">
        {shown.length === 0 && <div class="empty" data-empty><strong>Nothing matches "{q}"</strong></div>}
        {shown.map((x, n) => <button type="button" key={x.kind + x.to} role="option" aria-selected={n === activeIndex} data-active={n === activeIndex} class="menu-item" style="min-height:34px" onMouseEnter={() => select(n)} onClick={() => pick(x)}>
          {x.agent ? <Avatar agent={x.agent} s={s} size="sm" /> : <Icon name={x.icon!} size={15} class="faint" />}
          <span class={cx(x.mono && 'mono')} style="font-weight:var(--weight-medium)">{x.label}</span>
          <span class="muted truncate grow">{x.sub}</span>
          {x.note && <span class={cx(x.tone === 'warn' && 'warn-text', x.tone === 'error' && 'error-text', !x.tone && 'faint')} style="font-size:var(--text-sm)">
            {x.note}
          </span>}
          <span class="faint" style="font-size:var(--text-xs);width:64px;text-align:right">{x.kind}</span>
        </button>)}
      </div>
      <div class="dialog-foot" style="justify-content:flex-start;color:var(--color-muted);font-size:var(--text-sm);padding-block:8px">
        <span class="row"><kbd>↑</kbd><kbd>↓</kbd> move</span>
        <span class="row"><kbd>Enter</kbd> open</span>
      </div>
    </div>
  </div>;
}
registerDialog('switcher', Switcher);

// ---- new project ------------------------------------------------------------------
// A project starts from a goal, which is the first message to its project lead. The lead
// names the project in its first turn, so the owner never has to invent a name.
export interface SubscriptionTableProps { s: State; machine?: Id | null | undefined; value: Id | null | undefined; onPick: (id: Id) => void; name?: string }
export function SubscriptionTable({ s, machine, value, onPick, name = 'lead-sub' }: SubscriptionTableProps) {
  const rows = s.subscriptions.filter((x) => x.enabled);
  return <div class="table-wrap">
    <table class="table sub-table">
      <thead><tr><th /><th>Harness</th><th>Subscription</th><th>Machine</th><th>Models</th></tr></thead>
      <tbody>
        {rows.map((x) => { const ok = usableSubscription(x) && (!machine || x.machine === machine); const h = HARNESSES[x.harness]!;
      return <tr key={x.id} class={cx(ok && 'click', !ok && 'off')} aria-selected={value === x.id} onClick={() => ok && onPick(x.id)}>
        <td style="width:28px">
          <input type="radio" name={name} aria-label={`${h.name} · ${x.name}`} checked={value === x.id} disabled={!ok} onChange={() => onPick(x.id)} />
        </td>
        <td>{h.name}</td>
        <td>
          {x.name}
          {x.signInKnown === false ? <> <span class="faint">· Not reported</span></> : !x.signedIn ? <> <span class="faint">· signed out</span></> : x.usage ? <> <span class="faint num">· {x.usage.pct}% used</span></> : ''}
        </td>
        <td class="muted">{machineOf(s, x.machine).name}</td>
        <td class="muted wrap">{h.models.join(', ')}</td>
      </tr>; })}
      </tbody>
    </table>
  </div>;
}
interface DialogProps { s: State; d: DialogState }
function NewProject({ s, d }: DialogProps) {
  const [machine, setMachine] = useState<string>(live.on ? 'local' : d.machine || 'studio');
  const [repo, setRepo] = useState<string>(d.repo || s.repos[0]?.id || '__new');
  const [newRepo, setNewRepo] = useState('');
  const [goal, setGoal] = useState<string>(d.goal || '');
  const selection = useProjectLead(s, machine);
  const creating = live.on && board.busy('create-project');
  const m = machineOf(s, machine); const r = byId(s.repos, repo);
  const isNew = repo === '__new';
  const hasRepo = isNew || !!r?.folders[machine];
  const why = m.status !== 'online' ? `${m.name} is offline.` : !hasRepo ? null : !goal.trim() ? 'Write the goal.' : isNew && !newRepo.trim() ? 'Name the new repository.' : selection.why;
  const create = () => { if (why || !hasRepo || creating) return; if (!live.on) act.closeDialog(); act.createProject({ repo, goal: goal.trim(), machine, subscription: selection.lead?.account.accountId as Id, lead: selection.lead, newRepo: isNew ? newRepo.trim() : null }); };
  return <Dialog title="New project" size="new-project" onClose={act.closeDialog} onSubmit={create} foot={<>
      {why && <span class="why">{why}</span>}
      <button type="button" class="btn" onClick={act.closeDialog}>Cancel</button>
      <button type="submit" class="btn primary" disabled={!!why || !hasRepo || creating} aria-busy={creating}>
        {creating ? 'Creating…' : 'Create project'}
      </button>
    </>}>
    <div class="field">
      <label for="np-goal">Goal</label>
      <textarea id="np-goal" class="textarea" rows={3} value={goal} onInput={(e) => setGoal((e.target as HTMLTextAreaElement).value)} placeholder="Make a mock for the LGTM Studio phone app" autofocus onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) { e.preventDefault(); create(); } }} />
    </div>
    <div class="new-project-rows">
      <div class="field new-project-row">
        <label for="np-machine">Run on</label>
        <select id="np-machine" class="select" value={machine} onChange={(e) => setMachine((e.target as HTMLSelectElement).value)}>
          {s.machines.filter((x) => !live.on || x.id === 'local').map((x) => <option value={x.id} disabled={x.status !== 'online'}>
            {x.name}
            {x.status !== 'online' ? ' (offline)' : x.here ? ' (this computer)' : ''}
          </option>)}
        </select>
      </div>
      <div class="field new-project-row">
        <label for="np-repo">Repository</label>
        <select id="np-repo" class="select" value={repo} onChange={(e) => setRepo((e.target as HTMLSelectElement).value)}>
          {s.repos.map((x) => <option value={x.id}>{x.name}{x.folders[machine] ? '' : ` (not on ${m.name})`}</option>)}
          <option value="__new">New repository…</option>
        </select>
      </div>
    </div>
    {isNew && <div class="field new-project-row">
      <label for="np-newrepo">New repository name</label>
      <input id="np-newrepo" class="input" value={newRepo} onInput={(e) => setNewRepo((e.target as HTMLInputElement).value)} placeholder="notes-site" />
    </div>}
    {!hasRepo && r && <div class="callout warn">
      <Icon name="triangle-alert" size={15} />
      <span class="grow">{r.name} is not on {m.name}.</span>
      <button type="button" class="btn sm" onClick={() => { act.openDialog({ type: 'add-repository', machine, repo, back: { type: 'new-project', machine, repo, goal } }); }}>
        {'Add to '}
        {m.name}
      </button>
    </div>}
    {selection.fields}
  </Dialog>;
}
registerDialog('new-project', NewProject);

// ---- merge review -----------------------------------------------------------------
function MergeReview({ s, d }: DialogProps) {
  const r = (live.on && d.worktreeId ? s.worktrees.find((w) => w.id === d.worktreeId)?.review : byId(s.requests, d.requestId)) as ReviewView | undefined; if (!r) return null;
  const ev = r.evidence; const ok = ev.tests.ok && ev.review.ok;
  const docs = [['docs/reference/index.md', 212, 0], ['docs/reference/invoices.md', 148, 0], ['docs/reference/customers.md', 131, 0], ['scripts/build-reference.ts', 96, 12], ['docs/api/invoices.md', 0, 304], ['docs/api/customers.md', 0, 271], ['redirects.json', 37, 0]];
  const wt = (s.worktrees || []).find((w) => w.project === r.project && w.branch === r.source);
  // The sample's commit files are [path, add, del] rows or { path, add, del } records.
  const seen = new Map<string, [string, number, number]>();
  for (const c of wt?.commits || []) for (const f of (c.files || []) as SampleFile[]) { const k = f.path || f[0]!; const o = seen.get(k) || [k, 0, 0] as [string, number, number]; o[1] += f.add ?? f[2]?.add ?? 0; o[2] += f.del ?? f[2]?.del ?? 0; seen.set(k, o); }
  const files = live.on ? (r.files || []).map((f) => [f.path, '', '']) : r.project === 'apiref' || !seen.size ? (r.project === 'apiref' ? docs : []) : [...seen.values()].slice(0, 8);
  const subjects = (wt?.commits || []).slice(0, Math.max(1, ev.commits)).map((k) => k.subject);
  const first = live.on ? { title: r.prTitle || '', body: r.body || '' } : act.pullRequestText(subjects.length ? subjects : [r.title], ev);
  const [title, setTitle] = useState(first.title); const [body, setBody] = useState(first.body); const [writing, setWriting] = useState(false);
  const regen = () => { if (live.on) return refuse(); setWriting(true); setTimeout(() => { const t = act.pullRequestText(subjects.length ? subjects : [r.title], ev); setTitle(t.title); setBody(t.body); setWriting(false); }, 900); };
  const over = title.length > act.PR_TITLE_LIMIT;
  const approval = { head: r.head!, baseHead: r.baseHead!, decision: ok ? 'approve' : 'approve_without_evidence' };
  const merge = () => { if (live.on) return liveGit.answerReview(r.id, 'merge', undefined, approval); if (over || !title.trim()) return; act.closeDialog(); act.answerRequest(r.id, 'merge'); };
  return <Dialog title={r.title} size="wide" onClose={act.closeDialog} foot={<>
      <span class="why">Only you can merge into {r.target}.</span>
      <button type="button" class="btn" onClick={act.closeDialog}>Cancel</button>
      <button type="button" class={cx('btn', ok && 'primary')} disabled={live.on ? liveGit.busy('git') || r.status !== 'open' : over || !title.trim()} onClick={merge}>
        {live.on && liveGit.busy('git') ? 'Merging…' : ok ? 'Approve and merge' : 'Approve without evidence'}
      </button>
    </>}>
    <p class="mg-brief">{r.brief || r.body}</p>
    {r.notice && <p class="mg-notice"><b>User-visible change:</b> {r.notice}</p>}
    {r.delivery && <p class="mg-brief">
      {r.delivery.mode === 'github' ? <>
      {'GitHub pull request '}
      <a class="link" href={r.delivery.url} target="_blank" rel="noopener noreferrer">#{r.delivery.number}</a>
      {' · '}
      {r.delivery.state}
    </> : r.delivery.reason}
    </p>}
    {r.localSync && <p class="mg-brief">{r.localSync}</p>}
    <div class="pr-text">
      <div class="field">
        <div class="row">
          <label for="pr-title" class="grow">Pull request title</label>
          <span class={cx('num pr-count', over && 'error-text')} aria-live="polite">{title.length} / {act.PR_TITLE_LIMIT}</span>
          <button type="button" class="btn sm ghost" disabled={writing} data-tip={live.on ? null : `Writes the title and description with ${s.settings.gitModel || 'Haiku 5.5'}`} onClick={regen}>
            <Icon name={writing ? 'loader-circle' : 'sparkles'} size={13} class={writing ? 'spin' : ''} />
            Generate
          </button>
        </div>
        <input id="pr-title" class={cx('input', over && 'invalid')} value={title} disabled={writing} onInput={(e) => live.on ? refuse() : setTitle((e.target as HTMLInputElement).value)} />
        {over && <span class="error" role="alert">Keep the title to {act.PR_TITLE_LIMIT} characters.</span>}
      </div>
      <div class="field">
        <label for="pr-body">Description</label>
        <textarea id="pr-body" class="textarea mono" rows={7} value={body} disabled={writing} onInput={(e) => live.on ? refuse() : setBody((e.target as HTMLTextAreaElement).value)} />
      </div>
    </div>
    <MergeFacts s={s} r={r} />
    {live.on && r.truncated && <p role="alert">The comparison is truncated.</p>}
    <div>
      <div class="section-head">
        <h3>{ev.files} files changed</h3>
        <span class="grow" />
        <a class="link" href={`#/p/${r.project}/git`} onClick={act.closeDialog}>Open every diff in Git</a>
      </div>
      <table class="table">
        <tbody>
          {files.map(([f, add, del]: (string | number)[]) => <tr>
        <td class="mono truncate">{f}</td>
        <td class="num success-text">{add === '' ? '' : '+' + add}</td>
        <td class="num error-text">{del === '' ? '' : '−' + del}</td>
      </tr>)}
          {ev.files > files.length && <tr><td class="muted">{files.length ? 'and ' : ''}{ev.files - files.length}{files.length ? ' more' : ' files'}</td><td /><td /></tr>}
        </tbody>
      </table>
    </div>
  </Dialog>;
}
registerDialog('merge-review', MergeReview);

// ---- provider sign-in: the harness's own login, in a terminal on the right machine ---
function SignIn({ s, d }: DialogProps) {
  const sub = subOf(s, d.subscription)!; const m = machineOf(s, sub.machine); const h = HARNESSES[sub.harness]!;
  const [step, setStep] = useState(0);
  const cmd = sub.harness === 'codex' ? 'codex login' : 'claude /login';
  useEffect(() => { if (step === 1) { const t = setTimeout(() => setStep(2), 1800); return () => clearTimeout(t); } }, [step]);
  const done = () => {
    update((st) => { const x = subOf(st, sub.id)!; x.signInKnown = true; x.signedIn = true; x.usage = x.usage || { pct: 3, resetsAt: Date.now() + 6 * 864e5 }; });
    act.closeDialog(); act.toast(`Signed in to ${h.name} · ${sub.name} on ${m.name}`);
    if (d.then) act.answerRequest(d.then, 'resume');
  };
  return <Dialog title={`Sign in to ${h.name} · ${sub.name}`} onClose={act.closeDialog} foot={<>
      <span class="why">The sign-in runs on {m.name}, in the harness's own program.</span>
      <button type="button" class="btn" onClick={act.closeDialog}>Cancel</button>
      {step < 2 ? <button type="button" class="btn primary" disabled={step === 1} onClick={() => setStep(1)}>
        {step === 1 ? 'Waiting for the browser…' : 'Open the sign-in link'}
      </button> : <button type="button" class="btn primary" onClick={done}>Done</button>}
    </>}>
    <pre class="code" style="background:var(--color-term-bg);color:var(--color-term-text);min-height:150px;white-space:pre-wrap">
      {`you@${m.name.toLowerCase()} ~ $ ${cmd}\n`}
      {`Open this link in a browser to sign in:\n  https://auth.example/device?code=`}
      <span style="color:var(--color-term-amber)">QTRX-7H2M</span>
      {step >= 1 ? '\n\nWaiting for the browser…' : ''}
      {step >= 2 ? <><span style="color:var(--color-term-green)">Signed in as owner@example.com.</span>{`you@${m.name.toLowerCase()} ~ $ `}</> : ''}
    </pre>
  </Dialog>;
}
registerDialog('signin', SignIn);
