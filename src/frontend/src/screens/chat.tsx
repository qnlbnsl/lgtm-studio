import { live } from '../lib';
import { compact, busy } from '../live/chat';
// The Chat tab: one agent's thread at full size, with what it is working on beside it.
import { Icon, cx, href, useMedia, ago, useNow } from '../lib';
import {
  agentOf, teamOf, teamName, agentStatus, HARNESSES, subLabel, machineOf, sprintsOf, sprintProgress, agentsOfTeam, agentsOfProject, routeForAgent,
  openRequests, PERMISSION_MODES, roleLabel,
  taskPath,
} from '../data';
import * as act from '../actions';
import { Avatar, Thread, Composer, Menu, Empty, StateWord, canMessage, ReadOnlyBar } from '../ui';
import type { Agent, Id, Project, Route, State } from '../types';

function Aside({ s, project, a }: { s: State; project: Project; a: Agent }) {
  const task = a.task && s.tasks.find((t) => t.label === a.task && t.project === a.project);
  const lead = a.role === 'project-lead'; const now = useNow();
  const peers = lead ? agentsOfProject(s, project.id).filter((x) => x.role === 'team-lead') : a.team ? agentsOfTeam(s, a.team).filter((x) => x.id !== a.id) : [];
  const inFlight = sprintsOf(s, project.id).filter((x) => x.column === 'In progress' || x.column === 'Review');
  return <aside class="chat-aside" aria-label="About this agent">
    {task && <section>
      <h3>Current task</h3>
      <a class="list-row" style="padding:0 8px;margin:0 -8px" href={href(taskPath(task))}>
        <span class="tag">{task.label}</span>
        <span class="truncate">{task.title}</span>
      </a>
      {task.files.length > 0 && <>
        <ul class="file-list">
          {task.files.map((f) => <li key={f}><Icon name="file-code" size={13} class="faint" /><span class="mono truncate-start" title={f}>{f}</span></li>)}
        </ul>
        <a class="link" style="font-size:var(--text-sm)" href={href(`/p/${project.id}/git`)}>Open the changes in Git</a>
      </>}
    </section>}
    {lead && inFlight.length > 0 && <section>
      <h3>Active sprints</h3>
      {inFlight.map((sp) => { const pr = sprintProgress(s, sp);
      return <a key={sp.id} class="list-row" style="padding:0 8px;margin:0 -8px" href={href(`/p/${project.id}/sprint/${sp.id}`)}>
        <span class="tag">{sp.label}</span>
        <span class="truncate grow">{sp.name}</span>
        <span class="faint num" style="font-size:var(--text-xs)">{pr.done}/{pr.total}</span>
      </a>; })}
    </section>}
    <section>
      <h3>Configuration</h3>
      <dl class="dl" style="font-size:var(--text-sm)">
        <dt>Harness</dt>
        <dd>
          {HARNESSES[a.harness].name}
          {' '}
          <span class="faint">· {live.on ? a.runMode === 'acp' ? 'ACP' : a.runMode || '' : HARNESSES[a.harness].runMode} mode</span>
        </dd>
        <dt>Subscription</dt>
        <dd>{subLabel(s, a.subscription)}</dd>
        <dt>Model</dt>
        <dd>{a.model} · {a.effort}</dd>
        <dt>Permission</dt>
        <dd>
          <Menu class="pick" label="Permission mode" trigger={<><span>{a.permission}</span><Icon name="chevron-down" size={12} /></>} items={[{ heading: 'Narrowest to widest' }, ...PERMISSION_MODES.map((m) => ({ label: m, checked: m === a.permission, onSelect: () => act.setAgentConfig(a.id, { permission: m }) }))]} />
        </dd>
        <dt>Machine</dt>
        <dd>{machineOf(s, project.machine).name}</dd>
        <dt>Context</dt>
        <dd class="row">
          <span class="progress" style="width:72px"><i class={a.context >= 80 ? 'warn' : ''} style={{ width: a.context + '%' }} /></span>
          <span class="num">{a.context == null ? '—' : a.context + '%'}</span>
        </dd>
        <dt>Session</dt>
        <dd class="num">{live.on ? '' : a.turns + ' turns · ' + a.tokens.toFixed(1) + 'M tokens · '}started {ago(a.startedAt, now)} ago</dd>
      </dl>
    </section>
    {peers.length > 0 && <section>
      <h3>{lead ? 'Team leads' : 'Same team'}</h3>
      {peers.map((p) => { const st = agentStatus(s, p); const t = p.team && teamOf(s, p.team);
      return <a key={p.id} class="list-row" style="padding:0 8px;margin:0 -8px" href={href(routeForAgent(s, p))}>
        <Avatar agent={p} s={s} size="sm" />
        <span class="truncate grow">{p.name}{lead && t ? <> <span class="faint">· {t.area || t.stage}</span></> : ''}</span>
        <span class={cx('faint', st.tone === 'warn' && 'warn-text', st.tone === 'error' && 'error-text')} style="font-size:var(--text-xs);white-space:nowrap">
          {st.word}
        </span>
      </a>; })}
    </section>}
  </aside>;
}

export function ChatTab({ s, project, agentId, route }: { s: State; project: Project; agentId: Id; route: Route }) {
  const a = agentOf(s, agentId); const wide = useMedia('(min-width: 1500px)');
  if (!a || a.project !== project.id) return <div class="page">
    <Empty title="This agent is not in this project">
      <a class="link" href={href(`/p/${project.id}/conversation`)}>Open the project lead</a>
    </Empty>
  </div>;
  const st = agentStatus(s, a); const t = a.team && teamOf(s, a.team);
  const scope = a.role === 'project-lead' ? openRequests(s, { project: project.id }) : [];
  return <div class="chat">
    <div class="chat-main">
      <header class="chat-head">
        <Avatar agent={a} s={s} size="lg" />
        <div class="col grow">
          <div class="row" style="gap:8px"><h1>{a.name}</h1><span class="muted truncate">{t ? teamName(t) : project.name}</span></div>
          <div class="row" style="gap:8px;font-size:var(--text-sm)">
            <StateWord status={st} />
            {a.activity && a.activity !== st.word && <span class="muted truncate">· {a.activity}</span>}
          </div>
        </div>
        {a.stalled && <span class="badge warn" title={live.on ? 'The stall watch reported that this agent may be stuck.' : 'The stall watch saw no output for 14 minutes and nudged this agent once.'}>
          {live.on ? 'May be stuck' : 'No output for 14 min · nudged'}
        </span>}
        <Menu class="icon-btn" align="end" label="Agent actions" trigger={<Icon name="ellipsis" size={16} />} items={[
          a.state !== 'stopped' && a.state !== 'failed' && { label: 'Stop agent', disabled: busy('agent:' + a.id), icon: 'circle-stop', onSelect: () => act.stopAgent(a.id) },
          (a.state === 'stopped') && { label: 'Resume', disabled: busy('agent:' + a.id), icon: 'play', onSelect: () => act.resumeAgent(a.id) },
          live.on && { label: 'Compact context', disabled: busy('agent:' + a.id), onSelect: () => compact(a.id) },
          { label: 'Open its terminal, read-only', icon: 'square-terminal', onSelect: () => { location.hash = `#/p/${project.id}/terminal`; } },
          { label: 'Open its worktree in Git', icon: 'git-branch', onSelect: () => { location.hash = `#/p/${project.id}/git`; } },
        ].filter(Boolean)} />
      </header>
      <Thread key={a.id} s={s} agentId={a.id} center={true} focusReq={route.query.get('req')} onOpenThread={act.openSidePanel} />
      {canMessage(s, a) ? <Composer key={'c' + a.id} s={s} agentId={a.id} center={true} /> : <ReadOnlyBar s={s} agent={a} center={true} />}
    </div>
    {wide && <Aside s={s} project={project} a={a} />}
  </div>;
}
