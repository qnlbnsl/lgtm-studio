// The Define record of a project: the draft of the end result, the questions that decide
// it, and what the owner agreed. The Project view reads it beside the lead's conversation.
//
// The sample is the Define conversation of the project "We're trying issue 940", which ran
// on 2026-11-19. The owner's and the lead's words are theirs. The lead's two proposal cards
// are recut here: the proposal sits in the draft, and each decision is its own question.
import { store, update, uid, live, plural, MIN } from './lib';
import { agentOf, projectOf, threadOf } from './data';
import { sampleLater } from './actions';
import * as engine from './engine';
import type { Id, Ms, Project, State, ThreadItem, ToolLine } from './types';

// ---- the record --------------------------------------------------------------------
/** One statement of the end result: its lead sentence, the rest, and what changed. */
export interface DefineStatement {
  n: number; lead: string; rest: string; state: string; was: string | null; question: Id | null;
  /** The owner's notes on the statement, and whether one asked for a change after agreement. */
  notes?: string[]; asked?: boolean;
}
export interface DefineOption { id: string; label: string; example?: string; rec?: boolean }
export interface DefineAnswer { option: string | null; text: string; note: string; ts: Ms }
/** A question that decides a statement. */
export interface DefineQuestion {
  id: Id; statement: number; status: string; askedAt: Ms; title: string; text: string; options: DefineOption[]; answer?: DefineAnswer;
  /** A team's request has a kind. A Define question has none. */
  kind?: undefined;
}
/** The Define record of one project. */
export interface DefineRecord {
  draft: number; changedAt: Ms | null; agreedAt: Ms | null; goal: string; statements: DefineStatement[]; leftOut: string[];
  screens: string | null; next: { stage: string; text: string }[]; agents: string | null; questions: DefineQuestion[];
  /** Changes the owner asked for after agreeing. */
  changes?: number;
}
/** A line of a Define activity item: a tool call, and whether it failed. */
export interface ActivityLine extends Omit<ToolLine, 'res'> { failed: boolean }

const SEC = 1000;
const PID = 'naming'; const LEAD = 'naming-lead';

const GOAL = "We're trying to fix issue 940 here. Essentially, when we make a new project, the lead should be able to rename it to a more appropriate name. Right now, it just takes the first few words that fit and uses that.";

const st = (n: number, lead: string, rest: string, o: Partial<DefineStatement> = {}): DefineStatement => ({ n, lead, rest, state: 'same', was: null, question: null, ...o });
const DRAFT_1 = [
  'When a new project opens in Define, its lead checks whether the generated name describes the goal. If it reads like a fragment of the opening message, the lead proposes one short title.',
  'After you agree to a title, the lead uses the existing rename command. The title changes in the project, and the goal, Git branch, and worktree folder keep their existing values. If you decline, the current title stays.',
  'The creation form and temporary title generator are outside this fix.',
];

// ---- the sample --------------------------------------------------------------------
export function seedDefine(state: State): void {
  const T = state.T; const at = (ms: number) => T - ms; const t0 = 7 * MIN;
  state.projects.push({ id: PID, repo: 'lgtm', name: 'We’re trying issue 940', prefix: 'PN', goal: GOAL, stage: 'Define', phase: 'define', branch: 'we-re-trying-issue-940', folder: '~/projects/lgtm-studio/.worktrees/we-re-trying-issue-940', machine: 'studio', lead: LEAD, shell: 'zsh', createdAt: at(t0), hasMocks: false, tests: 'on', agentReview: 'on' });
  state.agents.push({ id: LEAD, project: PID, team: null, role: 'project-lead', name: 'Project lead', harness: 'codex', subscription: 'sub-codex', model: 'GPT-6.1 Sol', effort: 'High', permission: 'Automatic review',
    state: 'waiting', activity: 'Waiting for your answers', task: null, context: 21, turns: 4, tokens: 0.3, workState: 'waiting', stalled: false, startedAt: at(t0) });
  const run = (verb: string, arg: string, failed?: boolean): ActivityLine => ({ verb, arg, failed: !!failed });
  // Define's activity lines carry no result, so these items are wider than ThreadItem.
  state.threads[LEAD] = ([
    { id: 'n1', type: 'user', ts: at(t0), text: GOAL },
    { id: 'n2', type: 'agent', ts: at(t0 - 4 * SEC), text: 'Issue 940 is about letting the project lead replace the name generated from the opening prompt with a short, descriptive name the owner approves. I’ll check how project naming works today.' },
    { id: 'n3', type: 'activity', ts: at(t0 - 8 * SEC), secs: 30, text: 'Read 4 files, searched 3 times and ran 3 commands', lines: [
      run('Ran', 'cat README.md && git status --short && git rev-parse --short HEAD', true),
      run('Ran', 'cat README.md && git status --short && git rev-parse --short HEAD'),
      run('Searched', 'docs, packages and design for “940” and “project.rename”'),
      run('Read', 'its role instructions'),
      run('Ran', 'git log -8 --oneline'),
      run('Read', 'the project’s records'),
      run('Read', 'docs/issues.md'),
      run('Read', 'packages/engine/src/project/commands.ts'),
      run('Searched', 'packages for the code that names a new project'),
      run('Searched', 'design/mock/js/live for “project.create”'),
      run('Read', 'design/mock/js/live/board.js'),
    ] },
    { id: 'n4', type: 'agent', ts: at(t0 - 50 * SEC), text: 'The website already generates the initial name, and the engine already lets the project’s own lead rename it. I propose a small change I can make directly: put the name check near the start of Define and tell the lead exactly how to ask for approval and apply the chosen name. I’d leave the creation form and Git checkout alone.' },
    { id: 'n5', type: 'note', ts: at(t0 - 60 * SEC), icon: 'file-text', text: 'Wrote draft 1 of the end result · 3 statements' },
    { id: 'n6', type: 'user', ts: at(t0 - 141 * SEC), text: 'While I agree with your options, it doesn’t even need to check, honestly, because by default it gets those first few words on there. Right now, your project is called “we’re trying issue 940.” Also, I don’t want this to happen. Instead, each project should get a unique ID that shows up, followed by a colloquial name, which is technically a label that we set.' },
    { id: 'n7', type: 'agent', ts: at(t0 - 148 * SEC), text: 'You want each project to show a stable ID followed by a label we choose, with no title pulled from the opening sentence. I’ll check what the existing project ID can support and where the website currently displays the project name.' },
    { id: 'n8', type: 'activity', ts: at(t0 - 154 * SEC), secs: 35, text: 'Read 2 files and searched 4 times', lines: [
      run('Searched', 'design/mock/js for every place a project name is shown'),
      run('Searched', 'packages/contracts and packages/studio for “ProjectId”'),
      run('Searched', 'packages/engine/src/project/lifecycle.ts for branch and worktree names'),
      run('Read', 'packages/contracts/src/project/schema.ts'),
      run('Read', 'packages/studio/src/board.ts'),
      run('Searched', 'packages/engine/src/project/view.ts for “project.Created”'),
    ] },
    { id: 'n9', type: 'agent', ts: at(t0 - 190 * SEC), text: 'I rewrote the end result. The sidebar, project list, and project header show a stable ID followed by an editable label, for example **P-42 · Project Naming**.\n\nI compared displaying the existing long identifier, adding a short ID, and changing only the title suggestion. I recommend the short ID so you can recognize and refer to projects across repositories.\n\nTwo questions decide the rest.' },
    { id: 'n10', type: 'note', ts: at(t0 - 190 * SEC), icon: 'file-text', text: 'Rewrote the end result as draft 2 · 3 statements changed' },
    { id: 'n11', type: 'question', ts: at(t0 - 190 * SEC), q: 'ids', text: 'How should project IDs be numbered?' },
    { id: 'n12', type: 'question', ts: at(t0 - 190 * SEC), q: 'label', text: 'Who may change a project’s label?' },
  ] as unknown[]) as ThreadItem[];
  const records: Record<Id, DefineRecord> = {
    [PID]: {
      draft: 2, changedAt: at(t0 - 190 * SEC), agreedAt: null, goal: GOAL,
      statements: [
        st(1, 'Every project gets a short ID such as P-42, unique across this LGTM Studio installation and kept for the life of the project.', 'Existing projects gain IDs and keep their current labels. Existing links still work.', { state: 'changed', was: DRAFT_1[0], question: 'ids' }),
        st(2, 'A new project shows its ID on its own until a label is set.', 'During Define, the lead proposes a short label based on the goal and applies it after you agree. You can also edit the label in Project settings. Labels can repeat because each project has its own ID.', { state: 'changed', was: DRAFT_1[1], question: 'label' }),
        st(3, 'The opening message is kept as the goal in full.', 'Editing the label leaves the ID, links, Git branch, and worktree folder in place. A new project created without a label can use the ID for its checkout name.', { state: 'changed', was: DRAFT_1[2] }),
      ],
      leftOut: ['A name field on the creation form.', 'New names for the Git branches and worktree folders of existing projects.'],
      screens: 'The sidebar, the project list, and the project header',
      next: [
        { stage: 'Design', text: 'The sidebar, the project list, and the project header change. A Design team draws them first, and you approve the mocks.' },
        { stage: 'Build', text: 'One sprint and one build team. This changes both stored project records and the website. Engine and browser checks cover new and existing projects.' },
      ],
      agents: 'Codex · Automatic review',
      questions: [
        { id: 'ids', statement: 1, status: 'open', askedAt: at(t0 - 190 * SEC), title: 'How should project IDs be numbered?', text: 'The ID appears before the label everywhere a project is named.',
          options: [{ id: 'install', label: 'One counter for the whole installation', example: 'P-42', rec: true }, { id: 'repo', label: 'A prefix for each repository, then a number', example: 'LG-12' }] },
        { id: 'label', statement: 2, status: 'open', askedAt: at(t0 - 190 * SEC), title: 'Who may change a project’s label?', text: 'The ID never changes. The label is the name people read.',
          options: [{ id: 'both', label: 'You and the project lead', rec: true }, { id: 'owner', label: 'Only you' }] },
      ],
    },
  };
  state.define = records;
  state.expanded['project:' + PID] = false;
}

// ---- reading -----------------------------------------------------------------------
const S = () => store.state;
export const defineOf = (s: State, pid: Id): DefineRecord | null => ((s.define && s.define[pid]) as DefineRecord | undefined) || null;
export const openQuestions = (d: DefineRecord | null | undefined): DefineQuestion[] => (d ? d.questions.filter((q) => q.status === 'open') : []);
// Every project opens on its Project view, decided by the owner on 2026-11-20. The live
// website keeps opening the board until the Project view is wired to the engine.
// Callers pass the state and the project. The choice does not depend on them yet.
// A project opens on its command center, decided by the owner on 2026-11-23. One that is still being
// defined has nothing to report yet, so it opens on its end result. The live website opens the board.
export const homeTab = (_s?: State, p?: Project): string => (live.on ? 'board' : p?.stage === 'Define' ? 'end-result' : 'center');
// A project made while the page is open starts with an empty draft.
export function startDefine(s: State, pid: Id, goal: string): void {
  s.define = s.define || {};
  s.define[pid] = { draft: 0, changedAt: null, agreedAt: null, goal, statements: [], leftOut: [], screens: null, next: [], agents: null, questions: [] };
}

// ---- what the owner does -------------------------------------------------------------
// The lead's replies below are scripted for the sample project. Each one changes the draft
// the way the real lead changed its proposal after the same answer.
const EFFECTS: Record<string, (d: DefineRecord) => string> = {
  'ids:repo': (d) => { rewrite(d, 1, 'Every project gets a short ID made of its repository’s prefix and a number, such as LG-12, kept for the life of the project.', 'The engine assigns the number, so it survives restarts and label changes. Existing projects gain IDs and keep their current labels. Existing links still work.');
    return 'I’ll carry forward repository prefixes with per-repository numbers, assigned by the engine so they survive restarts and label changes.'; },
  'ids:install': () => 'Statement 1 stays as written: one counter for the whole installation.',
  'label:both': (d) => { rewrite(d, 2, 'A new project shows its ID on its own until a label is set.', 'During Define, the lead proposes a short label based on the goal and applies it after you agree. You and the project lead can edit the label at any time. Labels can repeat because each project has its own ID.');
    return 'Yes. You and the project lead can edit the label while the project ID stays fixed.'; },
  'label:owner': (d) => { rewrite(d, 2, 'A new project shows its ID on its own until a label is set.', 'During Define, the lead proposes a short label based on the goal, and you set it. Only you can edit the label, in Project settings. Labels can repeat because each project has its own ID.');
    return 'Only you can edit the label. The lead proposes one, and you set it.'; },
};
const OWN_WORDS: Record<string, string> = { ids: 'repo', label: 'both' };
function rewrite(d: DefineRecord, n: number, lead: string, rest: string): void {
  const x = d.statements.find((y) => y.n === n)!;
  Object.assign(x, { was: x.lead + ' ' + x.rest, lead, rest, state: 'changed' });
  d.draft += 1; d.changedAt = Date.now();
}
function leadSays(pid: Id, delay: number, fn: (d: DefineRecord, s: State) => string, { quiet = false, then = null as (() => void) | null } = {}): void {
  const lead = agentOf(S(), projectOf(S(), pid).lead);
  update((s) => { s.pending[lead.id] = true; lead.state = 'working'; lead.activity = 'Updating the end result'; });
  sampleLater(() => {
    update((s) => {
      const d = defineOf(s, pid); if (!d) return;
      delete s.pending[lead.id];
      const before = d.draft;
      const text = fn(d, s);
      const thread = threadOf(s, lead.id);
      if (d.draft !== before) thread.push({ id: uid('m'), type: 'note', ts: Date.now(), icon: 'file-text', text: `Changed the end result · draft ${d.draft}` });
      const open = openQuestions(d).length;
      const tail = d.agreedAt || open || quiet ? '' : '\n\nNothing else is open.';
      const reply = { id: uid('m'), type: 'agent', ts: Date.now(), text: text + tail };
      thread.push(reply); engine.push('agent.message', { agentId: lead.id, message: reply });
      lead.turns += 1; lead.context = Math.min(96, lead.context + 1);
      lead.state = open ? 'waiting' : 'idle'; lead.activity = open ? `Waiting for ${open === 1 ? 'your answer' : 'your answers'}` : '';
    });
    then?.();
  }, delay);
}
// The owner's sent answer to a Define question: one of its options, a note, or both.
export function answerQuestion(pid: Id, qid: Id, optionId: string | null | undefined, noteText?: string | null): void {
  const d = defineOf(S(), pid); const q = d?.questions.find((x) => x.id === qid); if (!q || q.status !== 'open') return;
  const note = (noteText || '').trim(); const opt = q.options.find((o) => o.id === optionId);
  if (!opt && !note) return;
  engine.call('answer', { questionId: qid, ...(opt ? { optionId: opt.id } : {}), text: [opt?.label, note].filter(Boolean).join('. ') });
  update(() => {
    Object.assign(q, { status: 'answered', answer: { option: opt?.id || null, text: opt ? opt.label + (opt.example ? `, such as ${opt.example}` : '') : note, note: opt ? note : '', ts: Date.now() } });
    const x = d!.statements.find((y) => y.n === q.statement); if (x) x.question = null;
  });
  leadSays(pid, 2200, (dd) => {
    // An answer in the owner's own words leads where the owner's real answer led.
    const effect = EFFECTS[`${qid}:${opt ? opt.id : OWN_WORDS[qid]}`];
    return effect ? effect(dd) : `I rewrote statement ${q.statement} from your answer.`;
  });
}
// The message box on the Project view speaks about one statement when the owner chose one.
// Anything else is an ordinary message.
export function ownerSaid(agentId: Id, text: string): boolean {
  const s = S(); const a = agentOf(s, agentId); const d = a && a.role === 'project-lead' ? defineOf(s, a.project) : null;
  const about = d && (s.ui.about as number | null | undefined);
  if (!about || !d.statements.some((x) => x.n === about)) return false;
  engine.call('prompt', { agentId, text, statement: about });
  update((st) => {
    threadOf(st, agentId).push({ id: uid('m'), type: 'user', ts: Date.now(), text, about });
    st.drafts['thread:' + agentId] = ''; st.ui.about = null;
  });
  leadSays(a.project, 2000, (dd) => {
    const x = dd.statements.find((y) => y.n === about)!; x.notes = [...(x.notes || []), text];
    if (!dd.agreedAt) return `I put your note on statement ${about}. I will work it into the next draft.`;
    x.asked = true; dd.changes = (dd.changes || 0) + 1;
    return `I put your note on statement ${about}. It is a change inside the agreed end result, so the project stays where it is. I will tell you if it grows past that.`;
  }, { quiet: true });
  return true;
}
export function setAbout(n: number | null): void { update((s) => { s.ui.about = n; }); }
// Agreeing to the end result ends Define. The project moves to Design when screens change,
// and to Plan when they do not. `startDesign` starts the Design team in the first case.
export function agree(pid: Id, startDesign?: (() => void) | null): void {
  const d = defineOf(S(), pid); if (!d || d.agreedAt || openQuestions(d).length) return;
  const p = projectOf(S(), pid);
  engine.call('answer', { projectId: pid, approval: 'end-result' });
  update((s) => {
    d.agreedAt = Date.now(); for (const x of d.statements) { x.state = 'agreed'; }
    if (d.screens) p.stage = 'Design'; else { p.stage = 'Plan'; p.phase = 'plan'; }
    threadOf(s, p.lead).push({ id: uid('m'), type: 'note', ts: Date.now(), icon: 'circle-check', tone: 'success', text: `You agreed to the end result · ${plural(d.statements.length, 'statement')}` });
  });
  leadSays(pid, 1600, () => (d.screens
    ? `The end result is agreed, and the project is in Design. ${d.screens} change, so I started a Design team. The first screen arrives in the Mocks tab.\n\nFrom here I work inside these ${plural(d.statements.length, 'statement')}.`
    : `The end result is agreed, and the project is in Plan.\n\nFrom here I work inside these ${plural(d.statements.length, 'statement')}.`),
  { then: d.screens ? startDesign ?? null : null });
}
