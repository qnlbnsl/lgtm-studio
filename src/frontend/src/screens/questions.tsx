// The Questions view of a project: one question in full beside the conversation, and the list of
// the project's questions on the far right. A question's card anywhere else opens it here.
import type { ComponentChildren } from 'preact';
import { useState, useEffect, useRef } from 'preact/hooks';
import { Icon, cx, href, useMedia, ago, clock, dayLabel, useNow, HOUR } from '../lib';
import { agentOf, agentLabel, questionRoute, taskPathById } from '../data';
import * as act from '../actions';
import { Avatar, Back, Empty, Cited, CodeBox, ListResizer, Menu } from '../ui';
import { defineOf } from '../define';
import type { Id, Project, Request, Route, State } from '../types';

// The questions and the approvals of one project that are the owner's to answer, or were. One a
// lead holds for its team is not among them. Approvals are here for now, by the owner's word of
// 2026-11-22. They are an agent's actions, not questions, and their own design comes later.
export const questionsOf = (s: State, pid: Id): Request[] => s.requests.filter((r) => r.project === pid && (r.kind === 'question' || r.kind === 'approval') && (r.status !== 'open' || !r.heldBy));

export const waitingQuestions = (s: State, pid: Id): Request[] => questionsOf(s, pid).filter((r) => r.status === 'open').sort((x, y) => x.createdAt - y.createdAt);

// A section's text as short points, one claim each, so the eye can jump from point to point.
const Points = ({ list }: { list: string | string[] | undefined }) => { const pts = (Array.isArray(list) ? list : [list]).filter((t): t is string => !!t);
  return <ul class={cx('qd-points qd-body', pts.length === 1 && 'one')}>{pts.map((t, i) => <li key={i}><Cited text={t} /></li>)}</ul>; };

interface Choice { id: string; label: string; note?: string | undefined; rec: boolean }
const OWN = '__own'; const REC = '__rec';

// One question as a report to skim. The question stands alone at the top, over a block of facts.
// Ruled sections follow: its background and its impact, each as short points. The answers are one
// choice among radio buttons, or several among checkboxes, and one button sends the answer. Deny on
// a card opens the page with the agent's recommendation declined.
function QuestionDoc({ s, r, declined }: { s: State; r: Request; declined: boolean }) {
  const now = useNow(); const a = agentOf(s, r.agent); const own = useRef<HTMLInputElement>(null);
  const [pick, setPick] = useState<string[]>([]); const [text, setText] = useState('');
  const done = r.status !== 'open'; const off = r.status === 'cancelled'; const opts = r.options || []; const isA = r.kind === 'approval';
  const multi = !isA && !!r.multi;
  const rec = isA ? null : opts.find((o) => o.primary) || null; const recText = isA ? null : r.recShort || r.recommendation || null;
  const title = r.brief || r.title; const why = r.whyText || r.why;
  const picked = done && !off && !isA ? opts.find((o) => o.label === r.answer) || null : null;
  // An answer that is none of the listed ones is the owner's own words, or the agent's recommendation in its words.
  const typed = done && !off && !picked && !isA ? r.answer : null;
  const choices: Choice[] = isA ? [] : [
    ...(rec ? [{ id: rec.id, label: rec.label, note: r.recWhy || rec.note, rec: true }]
      : recText && !(done && typed === recText) ? [{ id: REC, label: recText, note: r.recWhy, rec: true }] : []),
    ...opts.filter((o) => o !== rec).map((o) => ({ id: o.id, label: o.label, note: o.note, rec: false })),
  ];
  useEffect(() => { if (declined && !done) own.current?.focus(); }, [r.id, declined]);
  const toggle = (id: string) => setPick(multi ? (pick.includes(id) ? pick.filter((x) => x !== id) : [...pick, id]) : [id]);
  const ownOn = pick.includes(OWN); const ready = pick.length > 0 && (!ownOn || !!text.trim());
  const send = () => {
    if (!ready) return;
    const only = pick.length === 1 ? pick[0]! : null;
    if (only && only !== OWN && only !== REC) return act.answerRequest(r.id, only);
    act.answerRequest(r.id, null, pick.map((id) => (id === OWN ? text.trim() : choices.find((c) => c.id === id)!.label)).join(', '));
  };
  const when = done && r.answeredAt != null ? (dayLabel(r.answeredAt, now) === 'Today' ? clock(r.answeredAt) : dayLabel(r.answeredAt, now)) : null;
  const mine = <span class="qd-mine"><Icon name="check" size={13} />Answer</span>;
  const kind = multi ? 'checkbox' : 'radio';
  const row = (c: Choice) => { const struck = c.rec && declined && !done; const isMine = picked?.id === c.id;
    return <li key={c.id} class={cx(struck && 'declined', done && !isMine && 'off')}>
      <label class="qd-opt">
        <input type={kind} name={'qd-' + r.id} checked={done ? isMine : pick.includes(c.id)} disabled={done || struck} onChange={() => toggle(c.id)} />
        <span class="qd-opt-t">
          <b><span class="qd-label">{c.label}</span>{c.rec && !struck && <span class="qd-rec"><Icon name="sparkles" size={12} />Recommended</span>}{struck && <span class="qc-tag">Declined</span>}</b>
          {c.note && <p><Cited text={c.note} /></p>}
        </span>
      </label>
      {isMine && mine}
    </li>; };
  const background = r.what || (r.body && r.body !== title ? r.body : undefined);
  const st = r.statement ? defineOf(s, r.project)?.statements.find((x) => x.n === r.statement)?.lead || s.workflow?.[r.project]?.result.statements[r.statement - 1]?.text : null;
  const asker = <span>{agentLabel(s, a, { withTeam: true })}</span>;
  const discuss = <button type="button" class="btn sm qd-thread" onClick={() => act.openSidePanel(r.agent)}><Icon name="message-square" size={13} />Discuss</button>;
  const blocks = r.blocks && (r.task ? <a class="link mono" href={href(taskPathById(r.project, r.task))}>{r.blocks}</a> : <span class="mono">{r.blocks}</span>);
  const about = st && <a class="link" href={href(`/p/${r.project}/end-result`)}>{r.statement}. {st}</a>;
  const state: [string, ComponentChildren, string?] = off ? ['Status', 'Cancelled'] : done ? ['Answered', when || 'Yes', 'num']
    : ['Waiting', ago(r.createdAt, now), cx('num', now - r.createdAt >= HOUR && 'long')];
  // One line of facts under the title. The owner chose it over a list of labelled rows on 2026-11-23.
  const facts = <p class="qd-meta">{asker}{blocks && <span><i>Blocks</i>{blocks}</span>}{isA && r.tool && <span><i>Tool</i>{r.tool}</span>}<span class={state[2]}><i>{state[0]}</i>{state[1]}</span>{about && <span class="qd-about"><i>Statement</i>{about}</span>}</p>;
  // A section is a ruled part of the report.
  const sec = (name: string, body: ComponentChildren) => <section class="qd-sec" key={name}><h3>{name}</h3>{body}</section>;
  const command = isA && sec('Command', <div class="qd-body qd-cmd">
    {r.command ? <CodeBox text={r.command} label="command" /> : <p>Hidden by the harness. You cannot see this tool's input.</p>}
    <h4>Folder</h4>
    <CodeBox text={r.folder} label="folder" />
  </div>);
  const reasons = [background !== undefined && sec('Background', <Points list={r.what || r.body} />), why && sec('Impact', <Points list={why} />)];
  // An approval is allowed or denied once. The arrow beside each button makes the answer a standing one.
  const allowLabel = opts.find((o) => o.id === 'allow')?.label;
  const split = (label: string, icon: string, o: string, always: string, alwaysId: string) => <span class="split">
    <button type="button" class="btn" onClick={() => act.answerRequest(r.id, o)}><Icon name={icon} size={14} />{label}</button>
    <Menu class="btn split-more" label={'More ways to ' + o} trigger={<Icon name="chevron-down" size={14} />} items={[{ label: always, onSelect: () => act.answerRequest(r.id, alwaysId) }]} />
  </span>;
  const answers = isA
    ? sec('Decision', done ? <p class="qd-body qd-decided">{off ? 'Cancelled' : <>{r.answer}{mine}</>}</p> : <div class="qd-body qd-decide">
      {split(allowLabel && allowLabel !== 'Allow' ? allowLabel : 'Allow', 'check', 'allow', 'Always allow', 'always')}
      {split('Deny', 'x', 'deny', 'Always deny', 'never')}
    </div>)
    : (choices.length > 0 || typed || !done) && sec('Answers', <div class="qd-body">
      <ul class="qd-choices" role={multi ? 'group' : 'radiogroup'} aria-label="Answers">
        {typed && <li><span class="qd-opt"><Icon name="check" size={14} class="qd-tick" /><span class="qd-opt-t"><b>{typed}</b></span></span>{mine}</li>}
        {choices.map(row)}
        {!done && <li class="own"><span class="qd-opt">
          <input type={kind} name={'qd-' + r.id} aria-label="Your own answer" checked={ownOn} onChange={() => { toggle(OWN); own.current?.focus(); }} />
          <input ref={own} class="input" placeholder="Your own answer" aria-label="Your own answer, in your words" value={text}
            onInput={(e) => { setText(e.currentTarget.value); if (!ownOn) setPick(multi ? [...pick, OWN] : [OWN]); }}
            onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); send(); } }} />
        </span></li>}
      </ul>
      {!done && <button type="button" class="btn qd-send" disabled={!ready} onClick={send}><Icon name="send" size={13} />Send answer</button>}
    </div>);
  const refs = (r.refs?.length ?? 0) > 0 && sec('References', <ol class="qd-refs qd-body">{r.refs!.map((f, i) => <li key={f}><span class="qc-cite">[{i + 1}]</span><a class="link mono" href={href(`/p/${r.project}/files?f=${encodeURIComponent(f)}`)}>{f}</a></li>)}</ol>);
  return <article class={cx('qd', done && 'done')} id={'req-' + r.id} aria-labelledby={'qd-' + r.id}>
    <header class="qd-head"><h2 id={'qd-' + r.id}>{title}</h2>{discuss}</header>
    {facts}
    {command}
    {reasons}
    {answers}
    {refs}
  </article>;
}

export function QuestionsTab({ s, project, route }: { s: State; project: Project; route: Route }) {
  const now = useNow(); const phone = useMedia('(max-width: 860px)');
  const waiting = waitingQuestions(s, project.id);
  const answered = questionsOf(s, project.id).filter((r) => r.status !== 'open').sort((x, y) => (y.answeredAt ?? y.createdAt) - (x.answeredAt ?? x.createdAt));
  const all = [...waiting, ...answered]; const asked = route.parts[3];
  // With no question in the address, the one that has waited longest shows. A phone shows the list first.
  const cur = all.find((r) => r.id === asked) || (phone ? null : all[0] || null);
  if (!all.length) return <div class="qv none"><Empty title="No questions" /></div>;
  const row = (r: Request) => { const a = agentOf(s, r.agent); const done = r.status !== 'open';
    return <li key={r.id}>
      <a class={cx('qv-row', r.status === 'cancelled' && 'off')} aria-current={cur?.id === r.id ? 'page' : undefined} href={href(questionRoute(r))}>
        <span class="qv-t" title={(r.brief || r.title).length > 60 ? r.brief || r.title : ''}>{r.brief || r.title}</span>
        <span class="qv-m">
          <Avatar agent={a} s={s} size="sm" status={false} />
          {done ? <span class="qv-said" title={r.status !== 'cancelled' && (r.answer?.length ?? 0) > 60 ? r.answer : ''}>
          {r.status === 'cancelled' ? 'Cancelled' : r.answer}
        </span> : <span class="truncate">{r.kind === 'approval' && 'Approval · '}{agentLabel(s, a, { withTeam: true })}</span>}
          <time class={cx('num', !done && now - r.createdAt >= HOUR && 'long')}>
            {ago(done ? r.answeredAt ?? r.createdAt : r.createdAt, now)}
          </time>
        </span>
      </a>
    </li>; };
  const group = (label: string, list: Request[]) => list.length > 0 && <><h3>{label}</h3><ul>{list.map(row)}</ul></>;
  const declined = !!cur && !!s.ui.declined?.[cur.id];
  return <div class={cx('qv', phone && (cur ? 'one' : 'all'))}>
    {!phone && <ListResizer s={s} fallback={280} />}
    {cur && <div class="qv-main" data-space-waive="empty-pane">
      {phone && <a class="btn sm ghost qv-back" href={href(`/p/${project.id}/questions`)}><Icon name="arrow-left" size={13} />Questions</a>}
      {!phone && <Back path={`/p/${project.id}/questions`} class="qv-back" />}
      <QuestionDoc key={cur.id} s={s} r={cur} declined={declined} />
    </div>}
    {(!phone || !cur) && <nav class="qv-list" aria-label="Questions and approvals of this project" data-main data-space-waive="empty-pane">
      {group('Waiting', waiting)}
      {group('Answered', answered)}
    </nav>}
  </div>;
}
