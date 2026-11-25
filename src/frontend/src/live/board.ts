// Live actions for projects, the board and first run. Transport stays in the connection.
import { store, update, engineWrite, live, refuse, go } from '../lib';
import * as engine from '../engine';
import { toast, nameFromGoal, closeDialog, openDialog } from '../actions';
import type { Channel, DialogState, Id, Project, Sprint, State, Task } from '../types';
const S = () => store.state;
// The live connection lends its mapping functions; the mock never calls them.
const maps = () => engine.connection().board!;
export const channelArchived = (channel: Channel): boolean => channel.archived === true;
export const busy = (key: string): boolean => !!S()?.ui?.boardBusy?.[key];
const valid = (state: State, connection: engine.Connection) => live.on && S() === state && engine.connection() === connection && state.session.status === 'in';
export async function run<T>(key: string, work: () => Promise<T>, success?: string): Promise<T | undefined> {
  if (busy(key)) return;
  const state = S(), connection = engine.connection();
  update((s) => { (s.ui.boardBusy ||= {})[key] = true; });
  try {
    const reply = await work();
    if (!valid(state, connection)) return;
    if (success) toast(success);
    return reply;
  } catch (error) {
    if (valid(state, connection)) toast(error, 'error', 'The service could not confirm the request.');
  } finally { if (S() === state) update((s) => { delete s.ui.boardBusy![key]; }); }
}
export function currentActivity(agent: { id: Id }): any { return maps().workPreview(agent, S().threads[agent.id] || []); }
export async function pages(tag: string, payload: Record<string, unknown> = {}): Promise<any[]> {
  const rows: any[] = [], cursors = new Set<unknown>(); let after: unknown;
  for (;;) {
    const reply = await engine.request(tag, { ...payload, limit: 200, ...(after ? { after } : {}) });
    if (!Array.isArray(reply.items)) throw new Error('The service did not return the requested records.');
    rows.push(...reply.items);
    if (!reply.next) return rows;
    if (!reply.items.length || cursors.has(reply.next)) throw new Error('The service returned a repeated page.');
    cursors.add(reply.next); after = reply.next;
  }
}
export function archiveProject(projectId: Id, archived: boolean) {
  return run('project:' + projectId, () => engine.request(archived ? 'project.archive' : 'project.restore', { projectId }), archived ? 'Project archived' : 'Project restored');
}
export function renameProject(projectId: Id, name: string) {
  return run('project:' + projectId, () => engine.request('project.rename', { projectId, name }), 'Project renamed');
}
export function saveGoal(projectId: Id, goal: string) {
  return run('project:' + projectId, () => engine.request('project.goal.set', { projectId, goal }), 'Goal saved');
}
export function loadProjectLeadChoices(harnessId: string, accountId: string, model?: string | null) {
  return engine.request('client.harness.choices', { harnessId, accountId, ...(model ? { model } : {}) });
}
export function createProject(input: { goal: string; [field: string]: unknown }, { setup = false } = {}) {
  return run('create-project', async () => {
    const payload = maps().projectPayload(input, nameFromGoal(input.goal));
    await engine.request('project.placement.preview', payload);
    const reply = await engine.request('project.create', payload);
    if (setup) { update((s) => { s.ui.setupCreated = reply.projectId; }); await engine.request('project.setup.complete', { projectId: reply.projectId }); engineWrite({ session: { status: 'in', firstRun: false } }); }
    closeDialog(); go(`/p/${reply.projectId}/board`);
    return reply.projectId;
  });
}
export async function addRepository({ path, name, repo, back }: { path: string; name?: string; repo?: Id | null; back?: DialogState | null }) {
  return run('add-repository', async () => {
    const reply = await engine.request(repo ? 'project.repository.locate' : 'project.repository.add',
      { ...(repo ? { repoId: repo } : { name }), path: path.trim(), machineId: 'local' });
    if (back) openDialog({ ...back, machine: 'local', repo: repo || reply.repoId }); else closeDialog();
    return repo || reply.repoId;
  }, 'Repository added');
}
export function addTask(project: Project, sprint: Sprint, title: string, teamId?: Id | null) {
  return run('task:' + sprint.id, async () => {
    const team = S().teams.find((t) => t.id === (teamId || sprint.team)) || S().teams.find((t) => t.project === project.id);
    if (!team?.worktreeId) throw new Error('A team with a worktree is needed before adding a task.');
    return engine.request('project.task.create', { projectId: project.id, sprintId: sprint.id, title: title.trim(),
      owner: project.lead, teamId: team.id, worktreeId: team.worktreeId, state: 'queue' });
  }, 'Task added. The project lead was told.');
}
export function loadTask(project: Project, task: Task) {
  return run('read-task:' + task.id, async () => {
    const state = S(), connection = engine.connection();
    const [history, changes] = await Promise.allSettled([pages('project.task.history', { projectId: project.id, taskId: task.id }),
      engine.request('project.changes', { projectId: project.id, worktreeId: task.worktreeId })]);
    const workspace = S().worktrees.find((w) => w.id === task.worktreeId);
    const detail = maps().mapTaskDetails(history.status === 'fulfilled' ? history.value : [], { files: changes.status === 'fulfilled' ? changes.value.result?.items || [] : [] }, workspace && { branch: workspace.branch, path: workspace.folder });
    if (valid(state, connection)) engineWrite({ taskDetails: { ...S().taskDetails, [task.id]: detail } });
    for (const result of [history, changes]) if (result.status === 'rejected') toast(result.reason, 'error');
    return detail;
  });
}
export function loadAgents(ids: Id[]) {
  return run('read-agents:' + ids.join(','), async () => {
    const state = S(), connection = engine.connection();
    const entries = await Promise.all(ids.map(async (agentId) => {
      const [choices, context] = await Promise.all([engine.request('client.agent.configuration', { agentId }), engine.request('client.agent.context', { agentId })]);
      return [agentId, maps().mapAgentDetails(choices, context.context)];
    }));
    if (valid(state, connection)) engineWrite({ boardAgents: { ...S().boardAgents, ...Object.fromEntries(entries) } });
  });
}
export function configureLead(agentId: Id, patch: Record<string, unknown>) {
  return run('configure:' + agentId, async () => {
    const reply = await engine.request('configure', { agentId, ...patch });
    await loadAgents([agentId]); return reply;
  }, 'Configuration requested');
}
type MessageRead = { next: [Id, (Id | null)?, (Id | null)?] | null; promise: Promise<boolean | undefined> | null };
const messageReads = new Map<Id, MessageRead>();
export function loadMessages(projectId: Id, channelId?: Id | null, threadId?: Id | null): Promise<boolean | undefined> {
  const previous = messageReads.get(projectId);
  if (previous) { previous.next = [projectId, channelId, threadId]; return previous.promise!; }
  const slot: MessageRead = { next: null, promise: null };
  const operation = run('read-board:' + projectId, async () => {
    const state = S(), connection = engine.connection();
    const rows = await pages('board.channels');
    const channels = maps().mapChannels(rows, S());
    const own: Channel[] = channels.filter((c: Channel) => c.project === projectId);
    const selected = channelId || own[0]?.id;
    const targets = selected === 'waiting' ? own : own.filter((c) => c.id === selected);
    const all = await Promise.all(targets.map((c) => pages('board.peek', { channelId: c.id })));
    let posts = all.flat();
    if (threadId) posts.push(...await pages('board.read', { repliesTo: threadId }));
    const unique = [...new Map(posts.map((p) => [p.postId, p])).values()];
    // Times belong to event envelopes, not BoardPost. Read the matching event lazily.
    const mapped = await Promise.all(unique.map(async (p) => {
      const cached = S().posts.find((row) => row.id === p.postId && row.ts != null);
      const event = cached ? null : await engine.request('client.event', { sequence: p.sequence });
      return maps().mapPost(p, cached?.ts ?? event?.envelope?.occurredAt);
    }));
    if (selected && selected !== 'waiting') await pages('board.read', { channelId: selected });
    if (valid(state, connection)) engineWrite({ channels, posts: [...S().posts.filter((p) => !targets.some((c) => c.id === p.channel)), ...mapped] });
    return true;
  });
  slot.promise = operation.finally(() => { messageReads.delete(projectId); if (slot.next && live.on && S().session.status === 'in') void loadMessages(...slot.next); });
  messageReads.set(projectId, slot);
  return slot.promise!;
}
export function sendPost(input: { channel: Id; replyTo?: Id | null; [field: string]: unknown }) {
  return run('post:' + (input.replyTo || input.channel), async () => {
    const ch = S().channels.find((c) => c.id === input.channel);
    const reply = await engine.request('board.post', maps().postPayload(input, ch?.members || []));
    await loadMessages(ch!.project, ch!.id, input.replyTo);
    return reply;
  });
}
export function setClosed(id: Id, closed: boolean) {
  if (!closed) { refuse(); return; }
  return run('close-post:' + id, async () => {
    const reply = await engine.request('board.close', { postId: id });
    const post = S().posts.find((p) => p.id === id), ch = S().channels.find((c) => c.id === post?.channel);
    if (ch) await loadMessages(ch.project, ch.id);
    return reply;
  }, 'Post withdrawn');
}
export function loadSetup() {
  return run('read-setup', async () => {
    const state = S(), connection = engine.connection();
    const [setup, discovery, settings] = await Promise.all([engine.request('project.setup', {}), engine.request('project.setup.discover', {}), engine.request('project.settings', {})]);
    const data = maps().mapSetup(setup.setup, discovery.result, settings.settings);
    if (valid(state, connection)) engineWrite({ setupData: data });
    return data;
  });
}
export function advanceSetup(step: number) {
  return run('setup', async () => { await engine.request('project.setup.advance', { step: ['welcome', 'found', 'project'][step] }); return loadSetup(); });
}
export function saveSetupSettings(patch: Record<string, unknown>) {
  return run('setup-settings', async () => {
    const current = await engine.request('project.settings', {});
    const settings = { ...current.settings, ...patch };
    await engine.request('project.settings.set', { settings });
    return loadSetup();
  }, 'Settings saved');
}
export function registerSetupRepository(path: string) {
  return run('setup-repository', async () => {
    const known = S().repos.find((r) => Object.values(r.folders).includes(path.trim()));
    if (known) return { repo: known.id, path: path.trim(), name: known.name };
    const reply = await engine.request('project.repository.add', { path: path.trim(), name: path.trim().replace(/\/+$/, '').split('/').pop(), machineId: 'local' });
    return { repo: reply.repoId, path: path.trim() };
  });
}
