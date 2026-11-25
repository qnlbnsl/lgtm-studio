// The Project view: the conversation with the project lead in the centre, and the end
// result beside it. In Define the end result is a draft that the lead rewrites as the owner
// answers. Each question holds one decision. An answered question stays where it was asked.
import { useState, useEffect, useLayoutEffect, useRef } from 'preact/hooks';
import { Icon, cx, href, go, update, clock, dayLabel, ago, useNow, useMedia, plural } from '../lib';
import { STAGES, ROLES, agentsOfProject, agentOf, leadOf, threadOf, byId, teamsOf, teamName, agentsOfTeam, agentStatus, agentLabel, answerableRequests, routeForRequest, questionRoute, questionsView, KIND_LABEL } from '../data';
import { defineOf, openQuestions, answerQuestion, setAbout, agree } from '../define';
import { Avatar, Rich, Paras, RequestBlock, ToolGroup, Composer, Attachments, Resizer, Menu, canMessage, ReadOnlyBar } from '../ui';
import { pasteCommand } from './terminal';
import * as act from '../actions';
import type { ComponentChildren, TextareaHTMLAttributes } from 'preact';
import type { Agent, Id, Ms, Project, Request, State, ThreadItem, ToolLine, UiState } from '../types';

/** The Define record of a project, as define.ts holds it. */
type Define = NonNullable<ReturnType<typeof defineOf>>;
type DefineQuestion = Define['questions'][number];
/** An answer marked but not sent yet. */
interface Pick { option: string | null; note: string; always: boolean }
/** The fields of the page's own state that the Project view keeps. */
type MissionUi = UiState & { picks?: Record<string, Pick>; asks?: Record<string, boolean>; docks?: Record<Id, boolean>; mcPane?: string; about?: number | null };
/** A thread item as the Project view reads it: Define adds questions, notes and activity lines. */
type MissionItem = ThreadItem & { q?: Id; about?: number; tone?: string; items?: ThreadItem[] };
interface ActivityLine extends ToolLine { failed?: boolean }
interface ActivityItem { id: Id; text?: string | undefined; secs?: unknown; lines: ActivityLine[] }
interface AskOption { id: string; label: string; example?: string; rec?: boolean; note?: string | null | undefined }
interface Answered { text?: string | undefined; note?: string | undefined; ts?: Ms | undefined; option: string | null }
/** One statement of the end result: a Define draft statement, or one of the workflow's. */
interface StatementRow { n: number; lead: string; rest?: string | null | undefined; state: string; was?: string | null; question?: string | null; asked?: boolean; notes?: string[] }

const RESULT_WORD: Record<string, [string, string]> = { holds: ['success', 'Holds'], building: ['plain', 'Being built'], review: ['info', 'In review'], stuck: ['warn', 'Stuck'], dropped: ['', 'Dropped'], open: ['', 'Not started'] };
const DRAFT_WORD: Record<string, [string, string]> = { changed: ['info', 'Changed'], agreed: ['success', 'Agreed'] };

const since = (ts: number, now: number) => { const a = ago(ts, now); return a === 'now' ? 'just now' : a + ' ago'; };
const setPane = (pane: string) => update((s) => { (s.ui as MissionUi).mcPane = pane; });
// Shows the other half of a decision: the statement a question is about, or the question
// that holds a statement open. On a narrow screen it switches to that half first.
function jump(id: string, pane: string) {
  setPane(pane);
  setTimeout(() => {
    const el = document.getElementById(id); if (!el) return;
    el.scrollIntoView({ block: 'center', behavior: 'smooth' });
    el.classList.remove('mc-flash'); void (el as HTMLElement).offsetWidth; el.classList.add('mc-flash');
  }, 30);
}
const focusBox = (agentId: Id) => setTimeout(() => document.getElementById('composer-' + agentId)?.focus(), 30);

// ---- the conversation ----------------------------------------------------------------
function Activity({ item, legacy }: { item?: ActivityItem; legacy?: ThreadItem[] | undefined }) {
  const n = legacy ? legacy.length : item!.lines.length;
  const failed = legacy ? 0 : item!.lines.filter((l) => l.failed).length;
  const label = legacy ? (n === 1 ? legacy[0]!.summary : `${legacy[0]!.summary} and ${n - 1} more`) : item!.text;
  return <details class="mc-act">
    <summary>
      <Icon name="chevron-right" size={13} class="chev" />
      <span class="truncate">{label}</span>
      {!legacy && <span class="faint num">· {String(item!.secs)} s{failed ? ` · ${failed} failed` : ''}</span>}
    </summary>
    {legacy
      ? <div class="mc-act-old">{legacy.map((it) => <ToolGroup key={it.id} item={it} />)}</div>
      : <ol>
        {item!.lines.map((l, i) => <li key={i} class={cx(l.failed && 'failed')}><b>{l.verb}</b><span>{l.arg}</span>{l.failed && <i>failed</i>}</li>)}
      </ol>}
  </details>;
}

// ---- one decision that waits for the owner ---------------------------------------------
// A question of the Define phase, a question from a team, and an approval all read the same
// way: one line that opens and closes on its own, and under it the decision, its answers as
// rows, and one button that sends. Choosing a row only marks it. The decisions that wait sit
// above the message box. An answered one stays in the conversation where it was asked.
const pick = (key: string, patch: Partial<Pick>) => update((s) => { (s.ui as MissionUi).picks = { ...((s.ui as MissionUi).picks || {}), [key]: { option: null, note: '', always: false, ...(((s.ui as MissionUi).picks || {})[key] as Pick | undefined), ...patch } }; });
const setAsk = (id: string, open: boolean) => update((s) => { (s.ui as MissionUi).asks = { ...((s.ui as MissionUi).asks || {}), [id]: open }; });
const forget = (id: string) => update((s) => { if ((s.ui as MissionUi).picks) delete (s.ui as MissionUi).picks![id]; if ((s.ui as MissionUi).asks) delete (s.ui as MissionUi).asks![id]; });
// A text box that grows with its text to six lines, and scrolls after that.
function GrowText(props: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  const el = useRef<HTMLTextAreaElement>(null);
  useLayoutEffect(() => {
    const t = el.current; if (!t) return;
    t.style.height = '';
    const cs = getComputedStyle(t); const edges = t.offsetHeight - t.clientHeight;
    const most = 6 * parseFloat(cs.lineHeight) + parseFloat(cs.paddingTop) + parseFloat(cs.paddingBottom) + edges;
    t.style.height = Math.min(Math.max(t.scrollHeight + edges, t.offsetHeight), Math.ceil(most)) + 'px';
  }, [props.value]);
  return <textarea ref={el} {...props} />;
}

// The dock of one project is one line until it is opened.
const setDock = (pid: Id, open: boolean) => update((s) => { (s.ui as MissionUi).docks = { ...((s.ui as MissionUi).docks || {}), [pid]: open }; });
const showAsk = (id: string, pid: Id) => { setDock(pid, true); setAsk(id, true); jump(id, 'talk'); };
const recut = (r: Request | undefined) => r && (r.kind === 'question' || r.kind === 'approval');
const askId = (x: { id: Id; kind?: string }) => (x.kind ? 'mc-r-' : 'mc-q-') + x.id;
// Everything in the lead's conversation that waits for an answer, in the order it was asked.
function waitingOf(s: State, agent: Agent, d: Define | null): (DefineQuestion | Request)[] {
  const list: (DefineQuestion | Request)[] = [];
  for (const it of threadOf(s, agent.id) as MissionItem[]) {
    if (it.type === 'question') { const q = d?.questions.find((x: DefineQuestion) => x.id === it.q); if (q?.status === 'open') list.push(q); }
    for (const rid of it.type === 'request' ? [it.requestId] : it.requests || []) { const r = byId(s.requests, rid); if (recut(r) && r!.status === 'open' && !r!.heldBy) list.push(r!); }
  }
  return list;
}

interface AskProps {
  s: State; id: string; open: boolean; answered?: Answered | null; kind?: string | undefined; from?: Agent | null; askedAt: Ms; title: string;
  children?: ComponentChildren; options: AskOption[]; own?: boolean; standing?: Record<string, string> | null; why?: ComponentChildren;
  thread?: (() => void) | null; details?: string | null; onSend: (option: string | null, note: string, always: boolean) => void;
}
function Ask({ s, id, open, answered = null, kind = 'Question', from, askedAt, title, children, options, own = true, standing = null, why = null, thread = null, details = null, onSend }: AskProps) {
  const now = useNow(); const [whyOpen, setWhyOpen] = useState(false);
  const who = from && agentLabel(s, from, { withTeam: true });
  const chosen: Pick = answered ? { option: answered.option, note: '', always: false } : ((s.ui as MissionUi).picks || {})[id] || { option: null, note: '', always: false };
  const isOwn = chosen.option === 'own'; const ready = isOwn ? !!chosen.note.trim() : !!chosen.option;
  const marked = options.find((o) => o.id === chosen.option);
  const send = () => { if (!ready) return; forget(id); onSend(isOwn ? null : chosen.option, chosen.note.trim(), chosen.always); };
  // A row holds only the short answer. What the marked answer leads to is shown under the rows.
  const row = (o: AskOption, cls?: string) => <button type="button" key={o.id} role="radio" aria-checked={chosen.option === o.id} disabled={!!answered} class={cx('mc-opt', cls)} onClick={() => { pick(id, { option: o.id }); if (o.id === 'own') setTimeout(() => document.getElementById(id + '-note')?.focus(), 30); }}>
    <Icon name={chosen.option === o.id ? 'circle-dot' : 'circle'} size={14} class="radio" />
    <span class="grow">{o.label}{o.example && <> <span class="mono eg">{o.example}</span></>}</span>
    {o.rec && <span class="mc-rec"><Icon name="sparkles" size={12} />Recommended</span>}
  </button>;
  const body = open && <div class="mc-ask-body">
    {children}
    <div class={cx('mc-opts', !own && options.length === 2 && 'pair')} role="radiogroup" aria-label="Answers">
      {options.map((o) => row(o))}
      {own && !answered && row({ id: 'own', label: 'Other' }, 'own')}
    </div>
    {marked?.note && <p class="mc-opt-note">{marked.note}</p>}
    {isOwn && <GrowText id={id + '-note'} class="textarea mc-q-note" rows={2} aria-label="Your answer" placeholder="Your answer" value={chosen.note} onInput={(e) => pick(id, { note: e.currentTarget.value })} onKeyDown={(e) => { if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) { e.preventDefault(); send(); } }} />}
    {why && whyOpen && <div class="mc-why">{why}</div>}
    {(why || thread || !answered) && <footer>
      {why && <button type="button" class="mc-was-toggle" aria-expanded={whyOpen} onClick={() => setWhyOpen(!whyOpen)}>
        <Icon name="chevron-right" size={12} class="chev" />
        Impact
      </button>}
      {thread && <button type="button" class="btn ghost sm" onClick={thread}><Icon name="message-square" size={13} />Open thread</button>}
      {!answered && <span class="mc-q-go">
        {standing && marked && <label class="check mc-standing">
        <input type="checkbox" checked={chosen.always} onChange={(e) => pick(id, { always: e.currentTarget.checked })} />
        {standing[chosen.option!]}
      </label>}
        <button type="button" class="btn mc-send" disabled={!ready} onClick={send}><Icon name="send" size={13} />Send answer</button>
      </span>}
    </footer>}
  </div>;
  const chev = <Icon name="chevron-down" size={13} class="chev" />;
  if (answered) return <div class={cx('mc-q done', open && 'open')} id={id}>
    <button type="button" class="mc-q-asked" aria-expanded={!!open} onClick={() => setAsk(id, !open)}>
      <Icon name="check" size={14} />
      <span>{title}</span>
      {who && <small>{who}</small>}
      {chev}
    </button>
    <div class="mc-q-said">
      <span class="who">You</span>
      <p>{answered.text}{answered.note && <span>{answered.note}</span>}</p>
      <time>{clock(answered.ts!)}</time>
    </div>
    {body}
  </div>;
  // An open decision is two lines: what is asked, then its kind, its asker and its wait. The title has the
  // whole width, so a long one wraps instead of being cut. The arrow beside the fold opens it in the Questions view.
  return <section class={cx('mc-ask', open && 'open', details && 'goes')} id={id} aria-label={`${kind}: ${title}`}>
    <div class="mc-ask-top">
      <button type="button" class="mc-ask-head" aria-expanded={!!open} onClick={() => setAsk(id, !open)}>
        <span class="mc-ask-t">{title}</span>
        <span class="mc-ask-m">
          <span class="mc-label warn">{kind}</span>
          {who && <span class="who">{who}</span>}
          <time>{ago(askedAt, now)}</time>
        </span>
        <Icon name="chevrons-up-down" size={13} class="chev both" />
      </button>
      {details && <a class="icon-btn sm mc-ask-go" href={href(details)} aria-label={`Open in Questions: ${title}`} data-tip="Open in Questions" data-tip-side="left"><Icon name="arrow-up-right" size={14} /></a>}
    </div>
    {body}
  </section>;
}

function Question({ s, project, q, open }: { s: State; project: Project; q: DefineQuestion | undefined; open: boolean }) {
  if (!q) return null;
  const done = q.status !== 'open';
  return <Ask s={s} id={askId(q)} open={open} askedAt={q.askedAt} title={q.title} options={q.options} answered={done ? { text: q.answer!.text, note: q.answer!.note, ts: q.answer!.ts, option: q.answer!.option } : null} onSend={(option: string | null, note: string) => answerQuestion(project.id, q.id, option, note)}>
    <p>
      {q.text}
      {' '}
      <button type="button" class="mc-ref" onClick={() => jump(`mc-st-${project.id}-${q.statement}`, 'doc')}>
        {'Statement '}
        {q.statement}
      </button>
    </p>
  </Ask>;
}

// A question or an approval that a team sent up. The owner answers it here. A question has
// Details, which opens it in the Questions view. Where the shell has that view, an approval has
// Details too, in place of its reason and its thread.
const ALWAYS: Record<string, string> = { allow: 'Always allow in this project', deny: 'Always deny in this project' };
function TeamAsk({ s, project, r, lead, open }: { s: State; project: Project; r: Request; lead: Agent; open: boolean }) {
  const a = agentOf(s, r.agent); const isQ = r.kind === 'question'; const id = askId(r);
  const title = r.brief || r.title; const from = a && a.id !== lead.id ? a : null;
  if (r.status === 'open' && r.heldBy) return <div class="mc-note" id={id}><Icon name="clock" size={13} /><span>{title} · waiting on {agentOf(s, r.heldBy)?.name}</span></div>;
  const text = r.summary || (r.body && r.body !== title ? r.body : null);
  const first = <T,>(x: T | T[] | undefined): T | null => (Array.isArray(x) ? x[0] : x) || null;
  const options: AskOption[] = isQ
    ? (r.options || []).map((o) => ({ id: o.id, label: o.label, note: o.primary ? first(r.recWhy) : o.note, rec: !!o.primary && !!(r.recommendation || r.recShort) }))
    : [{ id: 'allow', label: 'Allow once' }, { id: 'deny', label: 'Deny' }];
  const send = (option: string | null, note: string, always: boolean) => {
    if (!isQ) return act.answerRequest(r.id, always ? (option === 'deny' ? 'never' : 'always') : option);
    const opt = (r.options || []).find((o) => o.id === option);
    act.answerRequest(r.id, opt?.id || null, opt ? undefined : note);
  };
  const answered = r.status === 'open' ? null : { text: r.answer, ts: r.answeredAt,
    option: isQ ? (r.options || []).find((o) => o.label === r.answer)?.id || null : /^Allowed/.test(r.answer || '') ? 'allow' : /^Denied/.test(r.answer || '') ? 'deny' : null };
  const why = r.whyText || r.why;
  return <Ask s={s} id={id} open={open} answered={answered} kind={KIND_LABEL[r.kind]} from={from} askedAt={r.createdAt} title={title} options={options} own={isQ} standing={isQ ? null : ALWAYS} onSend={send} thread={from && !isQ && !questionsView.on ? () => act.openSidePanel(r.agent, r.id) : null} details={isQ || questionsView.on ? questionRoute(r) : null} why={!isQ && !questionsView.on && (why || r.blocks) && <><Paras list={why} />{r.blocks && <p>Blocks {r.blocks}</p>}</>}>
    {text && <p>
      {text}
      {r.statement && <>
      {' '}
      <button type="button" class="mc-ref" onClick={() => jump(`mc-st-${project.id}-${r.statement}`, 'doc')}>
        {'Statement '}
        {r.statement}
      </button>
    </>}
    </p>}
    {!isQ && <div class="mc-cmd">
      {r.command ? <pre>{r.command}</pre> : <pre class="hid">Input hidden by the harness</pre>}
      <small>{r.tool ? r.tool + ' in ' : 'In '}<span class="mono">{r.folder}</span></small>
    </div>}
  </Ask>;
}
// The decisions that wait, above the message box. Each line opens and closes on its own.
// The first one starts open.
function Dock({ s, project, lead, d }: { s: State; project: Project; lead: Agent; d: Define | null }) {
  const list = waitingOf(s, lead, d);
  if (!list.length) return null;
  const shown = !!((s.ui as MissionUi).docks || {})[project.id];
  const approvals = list.filter((x) => x.kind === 'approval').length; const questions = list.length - approvals;
  const kinds = [questions && plural(questions, 'question'), approvals && plural(approvals, 'approval')].filter(Boolean).join(' · ');
  return <div class={cx('mc-dock', shown && 'open')}>
    <button type="button" class="mc-dock-head" aria-expanded={shown} onClick={() => setDock(project.id, !shown)}>
      <b>Waiting on {plural(list.length, 'answer')}</b>
      {approvals > 0 && <span class="faint">{kinds}</span>}
      <span class="grow" />
      <Icon name="chevron-down" size={13} class="chev" />
    </button>
    {shown && list.map((x) => {
      // A lone decision opens with the dock. Several start closed, and each opens on its own.
      const open = ((s.ui as MissionUi).asks || {})[askId(x)] ?? list.length === 1;
      return x.kind ? <TeamAsk key={askId(x)} s={s} project={project} r={x as Request} lead={lead} open={open} /> : <Question key={askId(x)} s={s} project={project} q={x as DefineQuestion} open={open} />;
    })}
  </div>;
}

function Conversation({ s, project, agent, d }: { s: State; project: Project; agent: Agent; d: Define | null }) {
  const items = threadOf(s, agent.id) as MissionItem[]; const now = useNow();
  const box = useRef<HTMLDivElement>(null); const atEnd = useRef(true);
  const count = items.length + (s.pending[agent.id] ? 1 : 0);
  useLayoutEffect(() => { const el = box.current; if (el) el.scrollTop = el.scrollHeight; }, [agent.id]);
  useLayoutEffect(() => { const el = box.current; if (el && atEnd.current) el.scrollTop = el.scrollHeight; }, [count]);
  const onScroll = (e: { currentTarget: HTMLDivElement }) => { const el = e.currentTarget; atEnd.current = el.scrollHeight - el.scrollTop - el.clientHeight < 80; };
  // The decisions above the message box change the height of this box. It stays at its end.
  useEffect(() => {
    const el = box.current; if (!el || typeof ResizeObserver === 'undefined') return undefined;
    const ro = new ResizeObserver(() => { if (atEnd.current) el.scrollTop = el.scrollHeight; });
    ro.observe(el); return () => ro.disconnect();
  }, [agent.id]);
  // Consecutive tool calls fold into one quiet line.
  const rows: MissionItem[] = [];
  for (const it of items) {
    const last = rows[rows.length - 1];
    if (it.type === 'tools' && last?.type === 'toolrun') last.items!.push(it);
    else if (it.type === 'tools') rows.push({ type: 'toolrun', id: 'run-' + it.id, ts: it.ts, items: [it] });
    else rows.push(it);
  }
  // A decision that still waits sits above the message box, not here.
  const isOpen = (id: string) => !!((s.ui as MissionUi).asks || {})[id];
  const block = (r: Request, who = true) => (!recut(r)
    ? <div class="mc-reqs" key={r.id}>
      <RequestBlock s={s} request={r} openThread={r.agent === agent.id ? null : () => act.openSidePanel(r.agent, r.id)} who={who} />
    </div>
    : r.status === 'open' && !r.heldBy ? null
      : <TeamAsk key={r.id} s={s} project={project} r={r} lead={agent} open={isOpen(askId(r))} />);
  let lastDay: string | null = null; let lastSpeaker: string | null | undefined = null;
  return <div ref={box} class="mc-thread" onScroll={onScroll} tabindex={0} aria-label={`Conversation with the ${agent.name.toLowerCase()}`}>
    <div class="mc-col">
      {rows.length === 0 && <div class="empty" data-empty><strong>No messages</strong></div>}
      {rows.map((it) => {
        const day = it.ts == null ? null : dayLabel(it.ts, now); const head = day !== lastDay ? <div class="day" key={'d' + it.id}>{day}</div> : null; lastDay = day;
        let body: ComponentChildren = null;
        if (it.type === 'user' && !it.from) {
          lastSpeaker = 'you';
          body = <div class="mc-msg mine" key={it.id}>
            {it.about && <button type="button" class="mc-ref" onClick={() => jump(`mc-st-${project.id}-${it.about}`, 'doc')}>
              {'About statement '}
              {it.about}
            </button>}
            <div class="mc-prose">{it.text && <Rich text={it.text} pid={project.id} />}<Attachments files={it.files} sent={true} /></div>
            <time>{clock(it.ts)}</time>
          </div>;
        } else if (it.type === 'agent' || it.type === 'user') {
          const from = it.type === 'agent' ? agent : agentOf(s, it.from); const who = it.type === 'agent' ? 'lead' : it.from;
          const meta = lastSpeaker !== who; lastSpeaker = who;
          body = <div class="mc-msg" key={it.id}>
            {meta && <div class="mc-who"><Avatar agent={from} s={s} size="sm" status={false} /><b>{from?.name}</b><time>{clock(it.ts)}</time></div>}
            <div class="mc-prose">
              <Rich text={it.text} pid={project.id} onCommand={(cmd: string) => { pasteCommand(s, project, cmd); go(`/p/${project.id}/terminal`); }} />
            </div>
            {it.requests?.some((id) => { const r = byId(s.requests, id); return r && !(recut(r) && r.status === 'open' && !r.heldBy); }) && <div class="mc-asks">{it.requests!.map((id) => { const r = byId(s.requests, id); return r ? block(r) : null; })}</div>}
          </div>;
        } else if (it.type === 'toolrun') body = <Activity key={it.id} legacy={it.items} />;
        else if (it.type === 'activity') body = <Activity key={it.id} item={it as ActivityItem} />;
        else if (it.type === 'question') { const q = d?.questions.find((x: DefineQuestion) => x.id === it.q);
          if (q && q.status !== 'open') { lastSpeaker = 'you'; body = <Question key={it.id} s={s} project={project} q={q} open={isOpen(askId(q))} />; } }
        else if (it.type === 'request') { const r = byId(s.requests, it.requestId);
          body = r ? block(r, r.agent !== agent.id) : null; }
        else if (it.type === 'note') body = <div class={cx('mc-note', it.tone)} key={it.id}>
          <Icon name={it.icon || 'dot'} size={13} />
          <span>{it.text}</span>
          <time>{clock(it.ts)}</time>
        </div>;
        else if (it.text) body = <div class="mc-note" key={it.id}><span>{it.text}</span><time>{clock(it.ts)}</time></div>;
        return [head, body];
      })}
      {(s.pending[agent.id] || agent.state === 'working') && <div class="mc-working" role="status"><i class="dot working pulse" /><span>{agent.activity || 'Working'}</span></div>}
    </div>
  </div>;
}

// The line that says the next message is about one statement.
function ReplyTarget({ s, d }: { s: State; d: Define | null }) {
  const about = d && (s.ui as MissionUi).about;
  if (!about) return null;
  return <div class="mc-target">
    <Icon name="corner-down-right" size={13} />
    <span class="truncate">About <b>statement {about}</b></span>
    <button type="button" class="icon-btn sm" aria-label="Cancel" onClick={() => setAbout(null)}><Icon name="x" size={13} /></button>
  </div>;
}

// ---- the end result ----------------------------------------------------------------
function Statement({ project, x, d, agentId, draft, asks = [] }: { project: Project; x: StatementRow; d: Define | null; agentId: Id; draft: boolean; asks?: Request[] }) {
  const [was, setWas] = useState(false);
  const [tone, word] = (x.asked ? ['warn', 'Change requested'] : draft ? DRAFT_WORD[x.state] : RESULT_WORD[x.state] || RESULT_WORD.open) || [];
  const q = draft && x.question && d!.questions.find((y: DefineQuestion) => y.id === x.question && y.status === 'open');
  const showWas = draft && x.was && x.state === 'changed';
  const canChange = draft;
  return <li id={`mc-st-${project.id}-${x.n}`} class={cx('mc-st', x.state, q && 'asks')}>
    <span class="n num">{x.n}</span>
    <div class="t">
      <p>{x.rest || draft ? <b>{x.lead}</b> : x.lead}{x.rest && <> {x.rest}</>}</p>
      {(q || showWas || canChange || asks.length > 0) && <div class="mc-st-foot">
        {asks.map((r) => <a key={r.id} class="mc-open-q" href={href(questionRoute(r))}>
          <Icon name="circle-help" size={13} />
          {r.brief || r.title}
        </a>)}
        {q && <button type="button" class="mc-open-q" onClick={() => showAsk('mc-q-' + q.id, project.id)}>
          <Icon name="circle-help" size={13} />
          {q.title}
        </button>}
        {showWas && <button type="button" class="mc-was-toggle" aria-expanded={was} onClick={() => setWas(!was)}>
          <Icon name="chevron-right" size={12} class="chev" />
          Previous wording
        </button>}
        {canChange && <button type="button" class="mc-change" onClick={() => { setAbout(x.n); setPane('talk'); focusBox(agentId); }}>
          <Icon name="pencil" size={12} />
          Ask for a change
        </button>}
      </div>}
      {was && showWas && <p class="mc-was">{x.was}</p>}
      {draft && x.notes?.map((t, i) => <p key={i} class="mc-st-note"><span>Note</span>{t}</p>)}
    </div>
    {word ? <span class={cx('mc-st-state', tone)}>
      {word}
      {!x.asked && (x.state === 'agreed' || x.state === 'holds') ? <Icon name="check" size={12} /> : null}
    </span> : <span />}
  </li>;
}

function EndResult({ s, project, d, lead }: { s: State; project: Project; d: Define | null; lead: Agent }) {
  const now = useNow();
  const w = s.workflow?.[project.id];
  const statements: StatementRow[] = d ? d.statements : (w?.result.statements || []).map((x, i) => ({ n: i + 1, lead: x.text, rest: x.note, state: x.state }));
  const open = openQuestions(d); const teams = teamsOf(s, project.id);
  const needs = d ? [] : answerableRequests(s, { project: project.id });
  const badge = !d ? null : d.agreedAt ? ['success', 'Agreed'] : d.draft ? ['', `Draft ${d.draft}`] : ['', 'Not written'];
  const nextStage = d?.screens ? 'Design' : 'Plan';
  // How far the end result is, in the words its statements carry. A draft has no such count.
  const tally: [number, string][] = d ? [] : (['holds', 'building', 'review', 'stuck', 'open'] as const)
    .map((k): [number, string] => [statements.filter((x) => (RESULT_WORD[x.state] ? x.state : 'open') === k).length, RESULT_WORD[k]![1].toLowerCase()]).filter(([n]) => n > 0);
  // A waiting question or approval that names a statement shows at that statement.
  const asksOf = (n: number) => needs.filter((r) => r.statement === n);
  return <aside class="mc-doc" aria-label="End result">
    <header class="mc-doc-head">
      <h2>End result</h2>
      {badge && <span class={cx('badge', badge[0])}>{badge[1]}</span>}
      {tally.length > 0 && <span class="mc-tally">{tally.map(([n, word]) => <span key={word}><b class="num">{n}</b> {word}</span>)}</span>}
      <span class="grow" />
      {needs.length > 0 && <a class="btn sm mc-needs" href={href(`/p/${project.id}/questions`)}>Open requests<span class="num">{needs.length}</span></a>}
      {d?.changedAt && !d.agreedAt && <span class="faint">changed {since(d.changedAt, now)}</span>}
      {d?.agreedAt && <span class="faint">{d.changes ? `${plural(d.changes, 'change')} asked since · ` : ''}agreed at {clock(d.agreedAt)}</span>}
    </header>
    <div class="mc-doc-body">
      {d?.goal && <section><h3 class="mc-label">Goal</h3><p class="mc-goal">{d.goal}</p></section>}
      <section>
        <h3 class="mc-label">Statements <span class="num">{statements.length || ''}</span></h3>
        <ol class="mc-sts">
          {statements.length === 0 && <li class="mc-st none" data-empty><span class="n" /><div class="t"><p class="muted">No statements</p></div></li>}
          {statements.map((x) => <Statement key={x.n} project={project} x={x} d={d} agentId={lead.id} draft={!!d} asks={asksOf(x.n)} />)}
        </ol>
      </section>
      {d && <section>
        <h3 class="mc-label">Out of scope</h3>
        {d.leftOut.length ? <ul class="mc-out">{d.leftOut.map((t: string, i: number) => <li key={i}>{t}</li>)}</ul> : <p class="muted">None</p>}
      </section>}
      {(d?.next?.length ?? 0) > 0 && <section>
        <h3 class="mc-label">Next</h3>
        <dl class="mc-path">
          {d!.next!.map((x: { stage: string; text: string }) => <><dt key={x.stage}>{x.stage}</dt><dd>{x.text}</dd></>)}
          <dt>Agents</dt>
          <dd>{d!.agents}</dd>
        </dl>
      </section>}
      {teams.length > 0 && <section>
        <h3 class="mc-label">Teams <span class="num">{teams.length}</span></h3>
        <ul class="mc-rows">
          {teams.map((t) => { const tl = agentsOfTeam(s, t.id).find((a) => a.role === 'team-lead'); const st = tl ? agentStatus(s, tl) : null; const n = answerableRequests(s, { team: t.id }).length;
          return <li key={t.id}>
            <a href={href(`/p/${project.id}/team/${t.id}`)}>
              {tl && <Avatar agent={tl} s={s} size="sm" />}
              <span class="truncate grow">{teamName(t)}</span>
              <span class={cx('truncate', st?.tone === 'warn' ? 'warn-text' : 'faint')}>
                {n ? plural(n, 'request') : tl?.activity || st?.word}
              </span>
            </a>
          </li>; })}
        </ul>
      </section>}
    </div>
    {d && statements.length > 0 && <footer class="mc-doc-foot">
      {d.agreedAt ? <>
        <span class="mc-foot-say"><Icon name="circle-check" size={15} class="success-text" /><b>Agreed</b></span>
        {project.hasMocks ? <a class="btn" href={href(`/p/${project.id}/design`)}>Open the design</a> : <a class="btn" href={href(`/p/${project.id}/board`)}>Open the board</a>}
      </>
        : <>
          <span class="mc-foot-say">
            {open.length > 0 && <><Icon name="circle-help" size={15} class="warn-text" /><b>{plural(open.length, 'question')} open</b></>}
          </span>
          <button type="button" class="btn primary" disabled={open.length > 0} onClick={() => agree(project.id, () => act.startDesignTeam(project.id))}>
            <Icon name="check" size={13} />
            {'Agree and start '}
            {nextStage}
          </button>
        </>}
    </footer>}
  </aside>;
}

// ---- the view ----------------------------------------------------------------------
// The end result as the first view on the right of a project.
export function EndResultView({ s, project }: { s: State; project: Project }) {
  return <EndResult s={s} project={project} d={defineOf(s, project.id)} lead={leadOf(s, project)} />;
}

// The button at the left end of the row of views. On a wide screen it shows or hides the conversation
// beside the view. On a narrow screen one half shows at a time, and it switches between the two.
// `locked` says another agent's thread holds the conversation's column, which then cannot be hidden.
export function TalkToggle({ s, home, locked, who = 'Project lead', short = 'Lead' }: { s: State; home: boolean; locked: boolean; who?: string; short?: string }) {
  const narrow = useMedia('(max-width: 1100px)'); const pane = (s.ui as MissionUi).mcPane || (home ? 'talk' : 'doc');
  const on = narrow ? pane === 'talk' : !!s.prefs.panelOpen || locked; const label = on ? 'Hide the conversation' : 'Show the conversation';
  return <button type="button" class="btn sm ghost tab-start" aria-pressed={on} disabled={!narrow && locked} aria-label={`${label} with the ${who.toLowerCase()}`} title={label} onClick={() => (narrow ? setPane(on ? 'doc' : 'talk') : act.setPref('panelOpen', !s.prefs.panelOpen))}>
    <Icon name="message-square" size={15} />
    <span class="tab-start-k">{who}</span>
    {/* A narrow window has no room for the whole name, so the button says a short one. */}
    <span class="tab-start-s" aria-hidden="true">{short}</span>
  </button>;
}
/**
 * Who the conversation beside a view is with: the project lead, or the lead of one team.
 * The owner asked on 2026-11-23 to message a build lead from any view.
 */
export function TalkPick({ s, project, current }: { s: State; project: Project; current: Agent }) {
  const lead = leadOf(s, project); const leads = agentsOfProject(s, project.id).filter((a) => a.role === 'team-lead');
  if (!leads.length) return null;
  const choose = (a: Agent) => { if (a.id === lead.id) act.closeSidePanel(); else act.openSidePanel(a.id); };
  return <Menu class="icon-btn tab-pick" align="start" width={250} label={`Switch lead. Current: ${agentLabel(s, current, { withTeam: true })}`} trigger={<Icon name="chevron-down" size={14} />}
    items={[lead, ...leads].map((a) => ({ label: agentLabel(s, a, { withTeam: true }), icon: ROLES[a.role]?.icon, checked: a.id === current.id, onSelect: () => choose(a) }))} />;
}
// Choosing a view on a narrow screen shows the view, also when it is the view already chosen.
export const showView = () => setPane('doc');

// The right end of the row of views. `mid` is the list button of a view that has a list. The stage
// is no longer here: the Status tab carries it, and the Status view shows every phase.
export function StageMark({ mid }: { s: State; project: Project; mid?: ComponentChildren }) {
  return <>{mid}</>;
}

// A project in the mock: the lead's conversation keeps to the left, and the chosen view takes the rest.
// `tabrow` is the row of views, `other` is another agent's thread in place of the lead's, and
// `children` is the chosen view. On a narrow screen one half shows at a time.
// `list` says whether the chosen view shows its own list on the far right.
interface ProjectShellProps {
  s: State; project: Project; path: string; home?: boolean; tabrow?: ComponentChildren; other?: ComponentChildren;
  flush?: boolean; focus?: boolean; list?: boolean; children?: ComponentChildren;
}
export function ProjectShell({ s, project, path, home, tabrow, other, flush, focus, list = true, children }: ProjectShellProps) {
  const lead = leadOf(s, project); const d = defineOf(s, project.id);
  // A narrow screen opens on the conversation where the end result is the project's first view, which is a
  // project in Define. It opens on the view for any other address.
  const narrow = useMedia('(max-width: 1100px)'); const pane = (s.ui as MissionUi).mcPane || (home ? 'talk' : 'doc');
  useEffect(() => () => setAbout(null), [project.id]);
  // Following a link to another view on a narrow screen shows that view.
  const first = useRef(true);
  useEffect(() => { if (first.current) { first.current = false; return; } if (narrow) setPane('doc'); }, [path]);
  // Where the row is too narrow for every tab, the chosen view's tab is brought into sight, clear of the row's end.
  const root = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const row = root.current?.querySelector(':scope > .tabrow'); const tab = row?.querySelector('.tab[aria-selected="true"]'); if (!row || !tab) return;
    const end = row.querySelector<HTMLElement>('.tab-end'); const r = row.getBoundingClientRect(); const t = tab.getBoundingClientRect();
    const start = row.querySelector('.tab-start');
    const hi = (end?.offsetParent ? end.getBoundingClientRect().left : r.right) - 16; const lo = (start ? start.getBoundingClientRect().right : r.left) + 10;
    if (t.right > hi) row.scrollLeft += t.right - hi; else if (t.left < lo) row.scrollLeft -= lo - t.left;
  }, [path, narrow]);
  const talk = !focus && (narrow ? pane === 'talk' : s.prefs.panelOpen || !!other);
  const side = focus || !narrow || pane === 'doc';
  // The divider between the two parts drags. It needs the widths that are on the page now.
  const both = talk && side && !narrow;
  const cols = useRef<HTMLDivElement>(null); const talkEl = useRef<HTMLElement>(null); const [box, setBox] = useState({ talk: 520, all: 1200 });
  useLayoutEffect(() => {
    const el = cols.current; if (!el || typeof ResizeObserver === 'undefined') return undefined;
    const read = () => setBox((b) => { const n = { talk: talkEl.current?.offsetWidth || b.talk, all: el.offsetWidth }; return n.talk === b.talk && n.all === b.all ? b : n; });
    read(); const ro = new ResizeObserver(read); ro.observe(el); if (talkEl.current) ro.observe(talkEl.current);
    return () => ro.disconnect();
  }, [both]);
  return <div ref={root} class={cx('mc', narrow && 'narrow')}>
    {!focus && tabrow}
    <div ref={cols} class={cx('mc-cols', !(talk && side) && 'solo')} style={both && s.prefs.talkWidth ? { '--talk-w': s.prefs.talkWidth + 'px' } : undefined}>
      {talk && <section ref={talkEl} class="mc-talk" aria-label={other ? 'Agent thread' : lead.name}>
        {other || <>
        <Conversation s={s} project={project} agent={lead} d={d} />
        <div class="mc-box">
          <div class="mc-unit">
            <Dock s={s} project={project} lead={lead} d={d} />
            <ReplyTarget s={s} d={d} />
            {canMessage(s, lead) ? <Composer s={s} agentId={lead.id} bare /> : <ReadOnlyBar s={s} agent={lead} />}
          </div>
        </div>
      </>}
        {both && <Resizer side="right" label="Resize the conversation" value={box.talk} min={320} max={Math.max(320, box.all - 360)} reset={null} onChange={(v) => act.setPref('talkWidth', v)} />}
      </section>}
      {side && <div class={cx('mc-pane', !list && 'no-list')} style={s.prefs.listWidth ? { '--list-w': s.prefs.listWidth + 'px' } : undefined}>
        <div class="view-body" style={flush ? 'overflow:hidden;display:flex;flex-direction:column' : ''}>{children}</div>
      </div>}
    </div>
  </div>;
}
