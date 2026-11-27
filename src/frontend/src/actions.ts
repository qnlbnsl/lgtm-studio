import { errorText } from './live/reads';
import * as liveGit from './live/git';
import * as server from './server/sync';
import * as chat from './live/chat';
// Everything that changes the mock's state, and the small simulation that makes agents
// answer and move. Nothing here talks to a service. Each change the real product sends to the
// engine is also reported to engine.js, under the RPC tag the real client API uses.
import { store, update, notify, uid, remember, go, MIN, live, refuse, engineWrite } from './lib';
import * as engine from './engine';
import * as board from './live/board';
import { ownerSaid, startDefine } from './define';
import {
  agentOf, projectOf, teamOf, threadOf, openRequests, agentLabel, byId, tasksOfSprint, teamName, HARNESSES, leadOf, moveNeeds, LANE_STAGE, LANE_PHASE,
  nextCode,
} from './data';
import type { Agent, Attachment, DialogState, Id, Prefs, Request, State, Team, ThreadItem } from './types';

const S = () => store.state;

/** Runs a delayed sample action only in the workspace that scheduled it. */
export function sampleLater(fn: () => void, delay: number): ReturnType<typeof setTimeout> {
  const workspace = S();
  return setTimeout(() => {
    if (S() === workspace && !live.on && !workspace.server.repositoryMode) fn();
  }, delay);
}

// ---- notices and dialogs -----------------------------------------------------
export function toast(error: unknown, tone = 'ok', fallback?: string): void {
  const text = errorText(error, fallback);
  if (!text) return;
  const id = uid('toast');
  const workspace = S();
  update((s) => { s.toasts.push({ id, text, tone }); });
  setTimeout(() => {
    if (S() === workspace) update((s) => { s.toasts = s.toasts.filter((t) => t.id !== id); });
  }, 3600);
}
// In live mode, an action that the engine does not take yet changes nothing and says so.
// The notice is written directly, because every other change in a refused turn is dropped.
export const NOT_CONNECTED = 'This is not connected to the service yet. Nothing was changed.';
live.onRefuse = () => {
  const workspace = S();
  if (workspace.ui.saved) workspace.ui.saved = null;
  const id = uid('toast');
  workspace.toasts.push({ id, text: NOT_CONNECTED, tone: 'error' }); notify();
  setTimeout(() => {
    if (S() === workspace) { workspace.toasts = workspace.toasts.filter((t) => t.id !== id); notify(); }
  }, 3600);
};
// True, after the notice, when the website is live and this action is not connected yet.
export function refused(): boolean { if (!live.on) return false; refuse(); return true; }
export function openDialog(dialog: DialogState): void { if (['pair-machine', 'signin'].includes(dialog.type) && refused()) return; update((s) => { s.dialog = dialog; }); }
export function closeDialog(): void { update((s) => { s.dialog = null; }); }
// Asks before a destructive change. `onConfirm` runs only after the owner agrees.
/** `second` is another way to do the same thing, drawn as a second button before the first. */
export interface ConfirmOptions { title: string; body?: string; lost?: string[]; action: string; danger?: boolean; waitForReply?: boolean; pendingLabel?: string; onConfirm: () => unknown; second?: { action: string; onConfirm: () => unknown } }
export function confirm({ title, body, lost = [], action, danger = true, onConfirm, second }: ConfirmOptions): void {
  openDialog({ type: 'confirm', title, body, lost, action, danger, onConfirm, second });
}

// ---- preferences ---------------------------------------------------------------
export function setPref<K extends keyof Prefs & string>(key: K, value: Prefs[K]): void { update((s) => { s.prefs[key] = value; }); remember(key, value); applyPrefs(); }
// Pin a project to the folded sidebar, or take it off.
export function togglePin(pid: Id): void { const now = S().prefs.pinned || []; setPref('pinned', now.includes(pid) ? now.filter((x) => x !== pid) : [...now, pid]); }
export function applyPrefs(): void {
  const { theme, density } = S().prefs;
  const dark = theme === 'dark' || (theme === 'system' && matchMedia('(prefers-color-scheme: dark)').matches);
  document.documentElement.dataset.theme = dark ? 'dark' : 'light';
  document.documentElement.dataset.density = density;
}
export function toggleExpanded(key: string, force?: boolean): void { update((s) => { s.expanded[key] = force ?? !s.expanded[key]; }); }

// ---- the side panel ------------------------------------------------------------
// The panel beside a project view shows the project lead. "Open thread" on a question
// swaps in the asking agent's thread, scrolled to the question, until it is answered.
export function openSidePanel(agentId: Id, requestId: Id | null = null): void {
  update((s) => {
    s.ui.panelAgent = agentId; s.ui.panelReq = requestId;
    if (matchMedia('(max-width: 1180px)').matches) s.ui.panelDrawer = true; else s.prefs.panelOpen = true;
  });
}
export function closeSidePanel(): void { update((s) => { s.ui.panelAgent = null; s.ui.panelReq = null; }); }
// A question opens in the Questions view of its project. Deny opens it with the recommendation declined.
export function openQuestion(r: Request, declined = false): void {
  if (declined) update((s) => { s.ui.declined = { ...(s.ui.declined || {}), [r.id]: true }; });
  go(`/p/${r.project}/questions/${r.id}`);
}

// ---- threads -------------------------------------------------------------------
const CANNED: Record<string, string[]> = {
  'repository-agent': [
    'Noted. I put that on the plan. It has no project yet, so it shows with "New project" in the command center.',
    'I checked the base branch. Its last full run passed, and no project is behind it by more than a day.',
    'That belongs to a project that exists. I passed it to its lead, and the lead will answer you there.',
  ],
  'project-lead': [
    'Understood. I will pass that to the team leads and tell you when the plan changes.',
    'Noted. That changes one sprint. I will rewrite its goal and ask the team lead to re-plan it.',
    'I checked with the leads. Nothing is blocked by this, so work carries on.',
  ],
  'team-lead': [
    'Got it. I will re-assign the affected subitem and post the change to the team channel.',
    'That fits the plan as written. No re-plan needed.',
  ],
  default: [
    'Understood. I will apply that and commit when the tests pass.',
    'Done. The change is in my worktree and the tests still pass.',
  ],
};
function replyFor(a: Agent, text: string): string {
  const t = text.toLowerCase();
  if (/status|progress|where are we|update/.test(t)) {
    const s = S();
    const open = openRequests(s, { project: a.project }).length;
    const working = s.agents.filter((x) => x.project === a.project && x.state === 'working').length;
    return `${working} agent${working === 1 ? ' is' : 's are'} working. ${open ? `${open} request${open === 1 ? '' : 's'} wait for you.` : 'Nothing waits for you.'}`;
  }
  if (a.role === 'repository-agent' && /goal|plan|project/.test(t)) {
    const s = S(); const r = s.repos.find((x) => x.agent === a.id); const open = (r?.plan || []).filter((g) => g.state === 'open');
    return open.length ? `${open.length === 1 ? 'One goal has' : open.length + ' goals have'} no project: ${open.map((g) => g.title.toLowerCase()).join(', and ')}. Say which one to start, and I will make its project and brief its lead.` : 'Every goal on the plan has a project.';
  }
  if (/screen|design|mock|page|look/.test(t) && a.role === 'project-lead') {
    return 'Screens change, so the Design team will draw them first. Open the Design tab to watch them arrive, and ask me for changes while a screen is open.';
  }
  if (/hotfix/.test(t) && a.role === 'project-lead') return 'I can start a hotfix team for that. It gets one builder and one test runner, and it merges after a passing full run.';
  const list = CANNED[a.role] || CANNED.default!;
  return list[Math.floor(Math.random() * list.length)]!;
}
export function setDraft(key: string, text: string): void { S().drafts[key] = text; }
/** Something the owner attached to a message: a statement, a file or a request. */
export type MessageContext = { id?: string; label?: string; [more: string]: unknown } | string;
export function sendMessage(agentId: Id, text: string, context: MessageContext[] = [], files: Attachment[] = []): unknown {
  if (live.on) return chat.sendMessage(agentId, text, context, files);
  text = text.trim();
  if (!text && !files.length) return;
  // On the Project view a message can answer a question or speak about one statement.
  if (text && ownerSaid(agentId, text)) return;
  const a = agentOf(S(), agentId);
  engine.call('prompt', { agentId, text, context: context.map((c) => (typeof c === 'object' && (c.label || c.id)) || String(c)), files: files.map((f) => ({ name: f.name, size: f.size })) });
  update((s) => {
    threadOf(s, agentId).push({ id: uid('m'), type: 'user', ts: Date.now(), text, context, files });
    s.drafts['thread:' + agentId] = '';
    s.pending[agentId] = true;
    if (a.state === 'idle') { a.state = 'working'; a.activity = 'Working on your message'; }
  });
  const timer = sampleLater(() => {
    update((s) => {
      if (!s.pending[agentId]) return;
      delete s.pending[agentId];
      const got = files.length ? `I have ${files.length === 1 ? files[0]!.name : files.length + ' files'}. ` : '';
      const reply: ThreadItem = { id: uid('m'), type: 'agent', ts: Date.now(), text: text ? got + replyFor(a, text) : got + 'What should I do with ' + (files.length === 1 ? 'it' : 'them') + '?' };
      engine.push('agent.message', { agentId, message: reply });
      threadOf(s, agentId).push(reply);
      if (a.activity === 'Working on your message') { a.state = 'idle'; a.activity = ''; }
      a.turns += 1; a.context = Math.min(96, a.context + 1);
    });
  }, 1600 + Math.random() * 900);
  S().pending[agentId] = timer;
}
// Stops the reply that is being written. The agent keeps running.
export function stopReply(agentId: Id): unknown {
  if (live.on) return chat.stopReply(agentId);
  engine.call('cancel-reply', { agentId });
  update((s) => {
    clearTimeout(s.pending[agentId]); delete s.pending[agentId];
    const a = agentOf(s, agentId);
    if (a.activity === 'Working on your message') { a.state = 'idle'; a.activity = ''; }
    threadOf(s, agentId).push({ id: uid('m'), type: 'note', ts: Date.now(), text: 'You stopped this reply.' });
  });
}
export function stopAgent(agentId: Id): unknown {
  if (live.on) return chat.stopAgent(agentId);
  const s = S(); const a = agentOf(s, agentId);
  const approvals = openRequests(s, { agent: agentId }).filter((r) => r.kind === 'approval');
  const doStop = () => update((st) => {
    engine.call('stop', { agentId });
    const ag = agentOf(st, agentId);
    ag.prevState = ag.state; ag.prevActivity = ag.activity; ag.state = 'stopped'; ag.activity = 'Stopped by you';
    for (const r of approvals) { r.status = 'answered'; r.answer = 'Cancelled'; r.answeredAt = Date.now(); }
    clearTimeout(st.pending[agentId]); delete st.pending[agentId];
    threadOf(st, agentId).push({ id: uid('m'), type: 'note', ts: Date.now(), text: `You stopped ${ag.name}.${approvals.length ? ' Its approvals were cancelled. Its questions stay open.' : ''}` });
  });
  if (a.state === 'working' || approvals.length) {
    confirm({
      title: `Stop ${a.name}?`,
      body: 'The agent stops where it is. Resume continues from its last turn.',
      lost: [a.state === 'working' && a.activity ? `Running work: ${a.activity}` : null, approvals.length ? `${approvals.length} waiting approval${approvals.length === 1 ? '' : 's'} will be cancelled` : null].filter((x): x is string => !!x),
      action: 'Stop agent', onConfirm: doStop,
    });
  } else doStop();
}
export function resumeAgent(agentId: Id): unknown {
  if (live.on) return chat.resumeAgent(agentId);
  engine.call('resume', { agentId });
  update((s) => {
    const a = agentOf(s, agentId);
    a.state = a.prevState && a.prevState !== 'stopped' ? a.prevState : 'idle';
    a.activity = a.state === 'working' ? (a.prevActivity || 'Picking up where it stopped') : '';
    a.failure = null;
    threadOf(s, agentId).push({ id: uid('m'), type: 'note', ts: Date.now(), text: `You resumed ${a.name}.` });
  });
}
export function setAgentConfig(agentId: Id, patch: Partial<Agent>): unknown { if (live.on) return chat.setAgentConfig(agentId, patch); engine.call('configure', { agentId, ...patch }); update((s) => { Object.assign(agentOf(s, agentId), patch); }); }

// ---- requests ------------------------------------------------------------------
export function answerRequest(requestId: Id, optionId: string | null, text?: string): unknown {
  if (live.on && S().worktrees?.some((w) => w.review?.id === requestId)) return liveGit.answerReview(requestId, optionId!);
  if (live.on) return chat.answerRequest(requestId, optionId, text);
  const s = S(); const r = byId(s.requests, requestId); if (!r || r.status !== 'open') return;
  const a = agentOf(s, r.agent); const opt = (r.options || []).find((o) => o.id === optionId);
  let answer: string;
  if (r.kind === 'approval') answer = optionId === 'deny' ? 'Denied' : optionId === 'never' ? 'Denied, always' : optionId === 'always' ? 'Allowed, always' : 'Allowed';
  else if (r.kind === 'merge') answer = optionId === 'decline' ? 'Sent back' : 'Merged';
  else if (r.kind === 'failed') answer = 'Resumed';
  else answer = text ? text : (opt ? opt.label : 'Answered');
  if (r.kind === 'approval') engine.call('answer', { kind: 'permission', id: requestId, optionId });
  else if (r.kind === 'question') engine.call('answer', { kind: 'question', id: requestId, text: answer });
  else if (r.kind === 'merge') engine.call(optionId === 'decline' ? 'review.report' : 'integrate', { requestId, source: r.source, target: r.target, ...(text ? { reason: text } : {}) });
  else engine.call('resume', { agentId: r.agent });
  update((st) => {
    r.status = 'answered'; r.answer = answer; r.answeredAt = Date.now();
    const task = r.task && byId(st.tasks, r.task);
    if (task && task.state === 'blocked') {
      task.state = 'active'; task.blockedBy = null;
      task.history.push({ ts: Date.now(), who: 'owner', text: r.kind === 'question' ? `You answered: ${answer}.` : `You ${answer.toLowerCase()} "${r.title}".` });
    }
    if (a.state !== 'stopped') {
      a.state = 'working'; a.failure = null;
      a.activity = r.kind === 'approval' ? (optionId === 'deny' ? 'Finding another way' : `Running ${r.command || r.title}`) : r.kind === 'merge' ? 'Writing the finished page' : 'Applying your answer';
    }
    if (optionId === 'always' || optionId === 'never') st.settings.standingRules = true;
    if (r.kind === 'merge' && optionId === 'decline' && text) { r.reason = text; threadOf(st, a.id).push({ id: uid('m'), type: 'user', ts: Date.now(), text: `Sent back "${r.title}": ${text}` }); }
    if (r.kind === 'merge' && optionId !== 'decline') { const p = projectOf(st, r.project); p.merged = true; for (const sp of st.sprints) if (sp.project === p.id) sp.column = 'Done'; }
    const lead = agentOf(st, projectOf(st, r.project).lead);
    const left = openRequests(st, { project: r.project }).length;
    if (lead && lead.id !== a.id) {
      threadOf(st, lead.id).push({ id: uid('m'), type: 'agent', ts: Date.now() + 1, text: `You ${r.kind === 'question' ? 'answered' : r.kind === 'failed' ? 'fixed' : answer.toLowerCase()} "${r.brief || r.title}". ${left ? `${left} request${left === 1 ? '' : 's'} still wait${left === 1 ? 's' : ''} for you.` : 'Nothing else waits for you on this project.'}` });
      if (!left && lead.state === 'waiting') { lead.state = 'working'; lead.activity = 'Checking in with the team leads'; }
      else if (left && /^Waiting on \d+ request/.test(lead.activity || '')) lead.activity = `Waiting on ${left} request${left === 1 ? '' : 's'}`;
    }
  });
  // The side panel goes back to the project lead once the question it opened for is answered.
  if (s.ui?.panelReq === requestId) sampleLater(() => { if (S().ui.panelReq === requestId) closeSidePanel(); }, 1200);
  toast(({ approval: `${answer}: ${r.title}`, question: 'Answer sent', merge: answer === 'Merged' ? `Merged ${r.source} into ${r.target}` : 'Sent back to the project lead', failed: 'Agent resumed' } as Record<string, string>)[r.kind]);
  // The agent reacts a moment later.
  sampleLater(() => update((st) => {
    const ag = agentOf(st, r.agent); if (ag.state === 'stopped') return;
    const lines: Record<string, string> = {
      approval: optionId === 'deny' ? 'Denied. I will look for a way that does not need that command.' : `Allowed. ${r.command ? 'The command ran and exited with 0.' : 'I have read it.'} Carrying on.`,
      question: `Thanks. Going with: ${answer}.`,
      merge: answer === 'Merged' ? `Merged into \`${r.target}\`. I am writing the finished page.` : 'Understood. I will ask what to change.',
      failed: 'Signed in and started. Reading the task now.',
    };
    const message: ThreadItem = { id: uid('m'), type: 'agent', ts: Date.now(), text: lines[r.kind] };
    engine.push('agent.message', { agentId: ag.id, message });
    threadOf(st, ag.id).push(message);
  }), 1400);
}
export function declinePhoneOffer(): unknown { if (live.on) return chat.declinePhoneOffer(); update((s) => { s.phoneOffer = 'declined'; }); remember('phoneOffer', 'declined'); }

// ---- projects ------------------------------------------------------------------
export function slug(name: string): string { return name.toLowerCase().trim().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'project'; }
// The name a project lead gives a project from its goal, in its first turn.
export function nameFromGoal(goal: string): string {
  const stop = /^(make|build|add|create|write|fix|let|give|turn|draw|design|ship|start|please|i|we|want|need|to|a|an|the|me|us|for|of|in|on|at|and|or|with|so|that|it|its|our|my|from|into)$/i;
  const w = goal.replace(/[.!?].*$/s, '').replace(/[,:;()"]/g, ' ').trim().split(/\s+/).filter((x) => x && !stop.test(x));
  const name = w.slice(0, 4).join(' ');
  return name ? name[0]!.toUpperCase() + name.slice(1) : 'New project';
}
export interface NewProject { repo: Id; goal: string; machine: Id; subscription: Id; newRepo?: string | null | undefined; lead?: { model?: string; effort?: string } }
export function createProject({ repo, goal, machine, subscription, newRepo, lead }: NewProject): unknown {
  if (live.on) return board.createProject({ repo, goal, machine, subscription, newRepo, lead });
  const s = S();
  let repoId = repo;
  if (newRepo) {
    repoId = slug(newRepo);
    update((st) => { st.repos.push({ id: repoId, name: repoId, main: 'main', remote: null, folders: { [machine]: `${byId(st.machines, machine)!.workspace}/${repoId}` } }); });
  }
  const id = uid('p');
  const r = byId(S().repos, repoId)!;
  const leadId = id + '-lead';
  const sub = (byId(s.subscriptions, subscription) || s.subscriptions[0])!;
  engine.call('create', { repositoryId: repoId, goal, machineId: machine, subscriptionId: sub.id, ...(newRepo ? { newRepository: newRepo } : {}) });
  update((st) => {
    st.projects.push({ id, repo: repoId, name: 'Naming…', naming: true, prefix: nextCode(st.projects.filter((x) => x.repo === repoId).map((x) => x.prefix)), goal, stage: 'Define', phase: 'define', branch: '', folder: '', machine, lead: leadId, shell: 'zsh', createdAt: Date.now(), hasMocks: false, tests: 'on', agentReview: 'on', fresh: true });
    st.agents.push({ id: leadId, project: id, team: null, role: 'project-lead', name: 'Project lead', harness: sub.harness, subscription: sub.id, model: lead?.model || HARNESSES[sub.harness]!.models[0]!, effort: lead?.effort || '', permission: st.settings.defaultPermission || 'Automatic review', state: 'working', activity: 'Reading the goal', task: null, context: 2, turns: 1, tokens: 0, workState: 'working', startedAt: Date.now() });
    st.threads[leadId] = [{ id: uid('m'), type: 'user', ts: Date.now(), text: goal }];
    st.pending[leadId] = true;
    st.expanded['repo:' + repoId] = true; st.expanded['project:' + id] = true;
    startDefine(st, id, goal);
  });
  go(`/p/${id}/end-result`);
  // The lead's first turn: it names the project, then asks what it needs to plan.
  sampleLater(() => update((st) => {
    const p = projectOf(st, id); const lead = agentOf(st, leadId); if (!p) return;
    const name = nameFromGoal(goal); const sl = slug(name);
    Object.assign(p, { name, naming: false, branch: 'feat/' + sl, folder: `${r.folders[machine]}/.worktrees/${sl}` });
    delete st.pending[leadId];
    lead.state = 'waiting'; lead.activity = 'Waiting for your answer';
    threadOf(st, leadId).push(
      { id: uid('m'), type: 'tools', ts: Date.now(), summary: 'Named the project', raw: 'project.name', status: 'completed', details: [{ label: 'Name', text: name }] },
      { id: uid('m'), type: 'tools', ts: Date.now(), summary: 'Created worktree', raw: 'project.worktree.create', status: 'completed', details: [{ label: 'Branch', text: 'feat/' + sl }, { label: 'Base', text: r.main }] },
      { id: uid('m'), type: 'agent', ts: Date.now(), text: `I named this project **${name}**.\n\nTwo questions before I plan. Who uses this? Does it change any screens? If it does, a Design team draws them first.` });
  }), 2600);
  return id;
}
export function archiveProject(pid: Id, archived = true): unknown {
  if (live.on) return board.archiveProject(pid, archived);
  engine.call(archived ? 'archive' : 'restore', { projectId: pid });
  update((s) => {
    const p = projectOf(s, pid); p.archived = archived;
    if (archived) for (const a of s.agents) if (a.project === pid && a.state !== 'stopped') { a.prevState = a.state; a.state = 'stopped'; a.activity = 'Archived'; }
  });
  toast(archived ? 'Project archived' : 'Project restored');
}
// Moving a sprint into a lane starts the team of that lane's stage, and the project lead is told.
// A move whose needs are not met happens only when it is forced. The mock makes the team itself.
const TEAM_CAST: Record<string, [string, string, string, string]> = { Plan: ['Planning lead', 'planner', 'Planner 1', 'Reading the goal'], Build: ['Build lead', 'builder', 'Builder 1', 'Reading the plan'], Review: ['Review lead', 'reviewer', 'Reviewer 1', 'Reading the diff'] };
export function moveSprint(id: Id, lane: string, { forced = false } = {}): void {
  if (live.on) return;
  update((s) => {
    const sp = byId(s.sprints, id); if (!sp || sp.column === lane) return;
    const missing = moveNeeds(s, sp, lane).filter((n) => !n.met); if (missing.length && !forced) return;
    const p = projectOf(s, sp.project); const old: Team | undefined = sp.team ? teamOf(s, sp.team) : undefined; const stage = LANE_STAGE[lane];
    sp.column = lane; sp.phase = LANE_PHASE[lane]; let said = '';
    // A team that was started for this sprint alone ends with its lane.
    const end = () => { if (old?.forSprint === sp.id) { s.agents = s.agents.filter((x) => x.team !== old.id); s.teams = s.teams.filter((t) => t.id !== old.id); } };
    if (lane === 'Backlog') { if (old) said = ` ${teamName(old)} stopped.`; end(); sp.team = null; }
    else if (stage && (!old || old.stage !== stage)) {
      end();
      const tid = `${sp.id}-${stage.toLowerCase()}`; const [leadName, role, name, doing] = TEAM_CAST[stage]!;
      const team: Team = { id: tid, project: p.id, stage, area: sp.name, branch: `${p.branch}/${sp.label.toLowerCase()}`, needToKnow: false, forSprint: sp.id };
      const mk = (aid: Id, r: string, n: string): Agent => ({ id: aid, project: p.id, team: tid, role: r, name: n, harness: 'claude', subscription: 'sub-max', model: 'Sonnet 5.5', effort: 'Medium', permission: 'Edit files without asking', state: 'working', activity: doing, task: null, context: 3, turns: 1, tokens: 0.1, workState: 'working', startedAt: Date.now() });
      s.teams.push(team); s.agents.push(mk(tid + '-lead', 'team-lead', leadName), mk(tid + '-1', role, name));
      sp.team = tid; said = ` ${teamName(team)} started.`;
    }
    const lead = leadOf(s, p);
    if (lead) threadOf(s, lead.id).push({ id: uid('m'), type: 'note', ts: Date.now(), icon: 'kanban',
      text: `You moved ${sp.label} to ${lane}.${said}${missing.length ? ` Forced past: ${missing.map((n) => n.short).join(', ')}.` : ''}` });
  });
}
// A lead that hears about screens starts a Design team and the Design tab fills.
export function startDesignTeam(pid: Id): void {
  if (refused()) return;
  update((s) => {
    const p = projectOf(s, pid); if (p.hasMocks) return;
    engine.call('team.create', { projectId: pid, stage: 'Design' });
    p.hasMocks = true; p.stage = 'Design';
    const tid = pid + '-design';
    s.teams.push({ id: tid, project: pid, stage: 'Design', area: null, branch: p.branch + '/design', needToKnow: false });
    const mk = (id: Id, role: string, name: string): Agent => ({ id, project: pid, team: tid, role, name, harness: 'claude', subscription: 'sub-max', model: 'Sonnet 5.5', effort: 'Medium', permission: 'Edit files without asking', state: 'working', activity: 'Drawing the first screen', task: null, context: 6, turns: 2, tokens: 0.1, workState: 'working', startedAt: Date.now() });
    s.agents.push(mk(pid + '-dl', 'team-lead', 'Design lead'), mk(pid + '-d1', 'designer', 'Designer'));
    s.channels.push({ id: 'ch-' + tid, project: pid, kind: 'team', team: tid, name: 'design', members: [pid + '-dl', pid + '-d1'] });
  });
}

// ---- the simulation ------------------------------------------------------------
const ACTIVITY: Record<string, string[]> = {
  builder: ['Reading the plan statement', 'Editing {file}', 'Running the unit tests', 'Editing {file}', 'Committing', 'Running pnpm typecheck'],
  'test-writer': ['Writing a failing test', 'Checking the test fails on its assertion', 'Writing stubs'],
  tester: ['Running check', 'Filling in the check report'],
  reviewer: ['Reading the diff', 'Tying a finding to statement {n}', 'Breaking the code on a temporary branch'],
  planner: ['Citing the code for statement {n}', 'Rewriting statement {n}', 'Checking citations'],
  designer: ['Drawing a screen', 'Recording a revision', 'Listing the fields a screen uses'],
  'team-lead': ['Reading a check report', 'Assigning the next subitem', 'Merging a builder into the team branch', 'Posting to the team channel'],
  'project-lead': ['Reading the leads channel', 'Comparing plans across teams', 'Checking in with the team leads'],
};
let ticking: ReturnType<typeof setInterval> | null = null;
export function startSimulation() {
  if (ticking !== null) return;
  const workspace = S();
  ticking = setInterval(() => {
    const s = S();
    if (s !== workspace || live.on || s.server.repositoryMode || s.conn.status !== 'live' || document.hidden) return;
    const working = s.agents.filter((a) => a.state === 'working' && !a.stalled && !s.pending[a.id]);
    if (!working.length) return;
    const a = working[Math.floor(Math.random() * working.length)]!;
    const list = ACTIVITY[a.role] || ACTIVITY.builder!;
    const task = a.task && s.tasks.find((t) => t.label === a.task && t.project === a.project);
    const file = task && task.files.length ? task.files[Math.floor(Math.random() * task.files.length)] : 'src/index.ts';
    a.activity = list[Math.floor(Math.random() * list.length)]!.replace('{file}', file!).replace('{n}', String(3 + Math.floor(Math.random() * 9)));
    a.turns += 1; a.tokens = Math.round((a.tokens + 0.02) * 100) / 100;
    if (Math.random() < 0.5) a.context = Math.min(95, a.context + 1);
    engine.push('agent.activity', { agentId: a.id, activity: a.activity, turns: a.turns, tokens: a.tokens, context: a.context });
    s.conn.lastHeard = Date.now();
    notify();
  }, 4200);
}
export function stopSimulation(): void {
  if (ticking === null) return;
  clearInterval(ticking);
  ticking = null;
}

// ---- preview states (the mock's own controls) ------------------------------------
export function setConnection(status: string): void {
  // In live mode the connection reports its own state. "Reconnect now" asks it to try again.
  if (live.on) { if (status === 'live') engine.connection().reconnect?.(); return; }
  update((s) => { s.conn.status = status; if (status === 'live') s.conn.lastHeard = Date.now(); });
}
export function setSession(status: string): void {
  // In live mode only the engine signs the page in. The page may still clear a rejected login.
  if (live.on) { if (status !== 'in') engineWrite({ session: { ...S().session, status } }); return; }
  update((s) => { s.session.status = status; });
  if (!S().server.repositoryMode) remember('session', status === 'in' ? 'in' : 'out');
}
// Signs in with the server's password. The sample server accepts any password except "wrong".
// Resolves to the session's new status.
export async function signIn(password: string): Promise<string> {
  if (live.on) return engine.connection().signIn!(password);
  const status = await server.signIn(password);
  setSession(status);
  return status;
}
// Asks again whether the service answers, after the Login page said it does not.
export async function retryService(): Promise<void> {
  if (live.on) return engine.connection().reconnect?.();
  await server.connect(S().server.address);
  if (S().server.status === 'ready') toast('The server is answering again');
}
export async function signOut(): Promise<void> {
  if (live.on) { await engine.connection().signOut!(); go('/login'); return; }
  await server.signOut();
  setSession('out'); go('/login');
}
export function resetMock(): void { try { for (const k of Object.keys(localStorage)) if (k.startsWith('lgtm-mock:')) localStorage.removeItem(k); } catch { /* storage is optional */ } location.hash = '#/home'; location.reload(); }
export { MIN, teamName, tasksOfSprint, agentLabel, teamOf };

// ---- Git messages ------------------------------------------------------------------
// The real product sends the staged diff, or the list of commits, to the cheap model the
// owner picked under Service settings. Its prompt asks for a short conventional commit and
// forbids attribution lines. The mock imitates the result from the file paths alone.
export const PR_TITLE_LIMIT = 72;
const typeOf = (paths: string[]): string => (paths.every((p) => /(^|\/)(test|tests)\/|\.test\.|\.spec\./.test(p)) ? 'test'
  : paths.every((p) => /\.(md|mdx|txt)$/.test(p) || /^docs\//.test(p)) ? 'docs'
  : paths.every((p) => /\.(css|scss)$/.test(p)) ? 'style'
  : paths.some((p) => /fix|flaky/i.test(p)) ? 'fix' : 'feat');
const scopeOf = (paths: string[]): string | null => {
  const dirs = paths.map((p) => p.split('/').slice(0, -1).filter((d) => !/^(src|packages|test|tests|lib|public)$/.test(d)).pop()).filter(Boolean);
  const n: Record<string, number> = {}; for (const d of dirs as string[]) n[d] = (n[d] || 0) + 1;
  const [top, c] = Object.entries(n).sort((a, b) => b[1] - a[1])[0] || [];
  return c! > paths.length / 2 ? top! : null;
};
const stemName = (p: string): string => p.split('/').pop()!.replace(/\.(test|spec)\./, '.').replace(/\.[a-z]+$/, '');
export function commitMessage(paths: string[]): string {
  if (!paths.length) return '';
  const type = typeOf(paths); const scope = scopeOf(paths);
  const seen = new Set<string>(); const names = paths.map(stemName).filter((n) => !seen.has(n.toLowerCase()) && seen.add(n.toLowerCase()));
  const verb = ({ test: 'cover', docs: 'update', style: 'restyle', fix: 'fix', feat: 'update' } as Record<string, string>)[type];
  const what = names.length <= 2 ? names.join(' and ') : `${names.slice(0, 2).join(', ')} and ${names.length - 2} more`;
  return `${type}${scope ? `(${scope})` : ''}: ${verb} ${what}`.slice(0, PR_TITLE_LIMIT);
}
export function pullRequestText(subjects: string[], evidence: { tests: { text: string }; review: { text: string } }): { title: string; body: string } {
  const lead = subjects.find((x) => !/^merge /i.test(x)) || subjects[0] || 'Changes';
  const scope = /^\w+(\([^)]*\))?:/.test(lead) ? '' : 'feat: ';
  let title = (scope + lead[0]!.toLowerCase() + lead.slice(1)).replace(/\.$/, '');
  if (title.length > PR_TITLE_LIMIT) title = title.slice(0, PR_TITLE_LIMIT - 1).replace(/\s+\S*$/, '') + '…';
  const body = ['## Summary', '', ...subjects.slice(0, 8).map((x) => `- ${x}`), ...(subjects.length > 8 ? [`- and ${subjects.length - 8} more commits`] : []),
    '', '## Evidence', '', `- ${evidence.tests.text}`, `- ${evidence.review.text}`].join('\n');
  return { title, body };
}
