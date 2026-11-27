// The Workflow tab: where a project is in the default build loop, its review rounds,
// its test runs, its end-result page, and the settings of the loop.
import { useState } from 'preact/hooks';
import { Icon, cx, href, go, swap, useQuery, ago, clock, dayLabel, useNow, plural, update, uid, live, MIN, HOUR, DAY } from '../lib';
import { PHASES, byId, agentOf, agentsOfTeam, sprintsOf, answerableRequests, teamsOf, teamName, leadOf, repoOf, routeForAgent, routeForRequest, KIND_LABEL, agentLabel } from '../data';
import { ACTIONS, actionPath, type ActionFile } from '../actions-data';
import * as act from '../actions';
import { recordedPhaseTimes, type PhaseTime } from '../live/workflow';
import { Avatar, Back, Empty, Menu } from '../ui';
import { ProjectSettingRow, Section, Switch, markSaved } from './service';
import type { ComponentChildren } from 'preact';
import type { Agent, Id, Ms, Project, Route, State } from '../types';

// ---- the workflow slice: one record for each project ---------------------------------
export type Severity = 'minor' | 'major' | 'blocker';
export interface ReviewFinding { id: string; statement: number; sev: Severity; text: string; status: string; by: number }
export interface ReviewRound { n: number; ts: Ms; findings: ReviewFinding[] }
/** `reviewers` names the agents that review, and a finding's `by` is the place of its reviewer in that list. */
export interface WorkflowReview { id: Id; subject: string; kind: string; team: Id; state: string; next: string; reviewers: string[]; rounds: ReviewRound[] }
export interface RunFailure { name: string; file: string; message: string; note?: string }
export interface TestRun {
  id: Id; kind: string; why: string; commit: string; branch: string; dur: string; passed: number; failed: number; skipped: number;
  failures: RunFailure[]; ts: Ms; msAgo?: number; by?: Id | null; note?: string; expected?: boolean; running?: boolean; skippedRun?: boolean;
}
export interface FixSprint { sprint?: Id; label?: string; name?: string; why: string; auto: boolean; state: string; to?: string; column?: string }
export interface StuckStatement { requestId: Id; review: Id; statement: number; text: string; points: number; of: number; round: number; rounds: number; decision?: string }
export interface ResultStatement { text: string; state: string; note?: string | undefined }
export interface WorkflowPr { number: number; title: string; requestId?: Id; source: string; target: string; commits: number; files: number; merged?: boolean; mergedAt?: Ms }
export interface WorkflowSettings { fullRun: string; check: string; maxRounds: number; stuckPoints: number; autoMergeFix: boolean; nightly: boolean }
export interface ProjectWorkflow {
  reviews: WorkflowReview[]; runs: TestRun[]; fixes: FixSprint[]; stuck: StuckStatement | null;
  result: { statements: ResultStatement[]; open: string[]; pr: WorkflowPr | null; draft?: boolean };
  settings: WorkflowSettings;
}
interface TabProps { s: State; p: Project; w: ProjectWorkflow }

const POINTS: Record<Severity, number> = { minor: 1, major: 3, blocker: 9 };
const SEV_LABEL: Record<Severity, string> = { minor: 'Minor', major: 'Major', blocker: 'Blocker' };
const fmt = (n: number) => n.toLocaleString('en-US');
const when = (ts: number, now: number) => (dayLabel(ts, now) === 'Today' ? clock(ts) : dayLabel(ts, now) + ', ' + clock(ts));

// ---- sample records --------------------------------------------------------------
const fnd = (statement: number, sev: Severity, text: string, status = 'fixed') => ({ statement, sev, text, status });
const REVIEWERS = ['Reviewer 1', 'Reviewer 2', 'Reviewer 3'];
function review(id: Id, subject: string, kind: string, team: Id, state: string, next: string, rounds: { ts: Ms; findings: Omit<ReviewFinding, 'id' | 'by'>[] }[]): WorkflowReview {
  let n = 0;
  // The sample gives each finding to one of three reviewers in turn.
  return { id, subject, kind, team, state, next, reviewers: REVIEWERS, rounds: rounds.map((r, i) => ({ n: i + 1, ts: r.ts, findings: r.findings.map((f) => ({ ...f, by: n % REVIEWERS.length, id: 'F-' + String(++n).padStart(2, '0') })) })) };
}
const roundPoints = (r: ReviewRound) => r.findings.reduce((sum, f) => sum + POINTS[f.sev], 0);
const roundCount = (r: ReviewRound, sev: Severity) => r.findings.filter((f) => f.sev === sev).length;
// The time of a sample run is set from msAgo once the sample's clock is known.
const run = (kind: string, why: string, commit: string, branch: string, msAgo: number, dur: string, passed: number, failed: number, skipped: number, o: Partial<TestRun> = {}) => ({ id: uid('run'), kind, why, commit, branch, dur, passed, failed, skipped, failures: [], ...o, msAgo }) as TestRun;
const st = (text: string, state = 'open', note?: string): ResultStatement => ({ text, state, note });

function blank(p?: Project): ProjectWorkflow {
  return {
    reviews: [], runs: [], fixes: [], stuck: null,
    result: { statements: [], open: [], pr: null },
    settings: { fullRun: 'pnpm check', check: 'pnpm test --related', maxRounds: 6, stuckPoints: 12, autoMergeFix: true, nightly: true },
  };
}
// The live website gives every project an empty record until the engine holds one.
export const emptyWorkflow = () => blank();
// A project made while the page is open gets an empty record.
export const workflowOf = (s: State, p: Project): ProjectWorkflow => ensure(s, p);
function ensure(s: State, p: Project): ProjectWorkflow { s.workflow = s.workflow || {}; return s.workflow[p.id] || (s.workflow[p.id] = blank(p)); }

export function seedWorkflow(state: State): void {
  const T = state.T; const at = (ms: number) => T - ms;
  const stamp = (w: ProjectWorkflow) => { for (const r of w.runs) { r.ts = at(r.msAgo!); } return w; };
  state.workflow = {};
  const add = (pid: Id, fill: (w: ProjectWorkflow) => void) => { const w = blank(byId(state.projects, pid)); fill(w); state.workflow[pid] = stamp(w); };

  add('console', (w) => {
    w.reviews = [review('rv-sp4', 'LS-A01-4 Narrow-screen pass', 'Code review', 'console-review', 'scoring',
      'Round 3 runs next. After round 3, another round runs only while a blocker or a major is open.', [
        { ts: at(5 * HOUR), findings: [
          fnd(11, 'blocker', 'Tab order skips the composer at 390 pixels.'),
          fnd(12, 'major', 'The focus ring is clipped by the drawer edge.'),
          fnd(13, 'major', 'The bottom key row covers the last line of the terminal.'),
          fnd(14, 'major', 'The drawer opens behind the lead panel.'),
          fnd(11, 'minor', 'The skip link lands on the sidebar, not on the view.'),
          fnd(12, 'minor', 'The focus ring uses a literal colour.'),
          fnd(13, 'minor', 'The Ctrl key has no pressed state.'),
          fnd(14, 'minor', 'The drawer has no label for screen readers.')] },
        { ts: at(35 * MIN), findings: [
          fnd(11, 'major', 'Escape does not return focus to the menu button.', 'open'),
          fnd(11, 'minor', 'Focus leaves the drawer on Tab.', 'open'),
          fnd(13, 'minor', 'Ctrl stays latched after Escape.', 'open'),
          fnd(14, 'minor', 'Service pages come before agents in the drawer.', 'open')] },
      ])];
    w.runs = [
      run('Check', 'LS-A01-24 · Reconnect test waits for the socket to close', 'a41c0d2', 'feat/agent-console/shell', 8 * MIN, '48 s', 40, 0, 0, { by: 'shell-test', note: 'The reconnect test ran 40 times in a row.' }),
      run('Check', 'LS-A01-19 · Tests for scroll position and the Latest button', '7d20f1c', 'feat/agent-console/threads', 46 * MIN, '21 s', 0, 9, 0, { by: 'threads-tw', expected: true, note: 'Written before the code. All 9 fail on their assertion, as they should.' }),
      run('Full run', 'Starting commit for LS-A01-3', '55e1bbd', 'feat/agent-console', 50 * MIN, '6 min 12 s', 7791, 1, 12, { by: 'shell-test', failures: [
        { name: 'reconnects after the socket closes', file: 'packages/client/test/reconnect.test.ts:88', message: 'Expected status "live" within 2000 ms, received "reconnecting".', note: 'Flaky: it failed 3 of 40 runs. LS-A01-9 fixes it.' }] }),
      run('Full run', 'Merge of LS-A01-1 Navigation foundation', '5e12347', 'feat/agent-console', 26 * HOUR, '6 min 3 s', 7792, 0, 12, { by: 'shell-test' }),
      run('Build', 'Merge of LS-A01-1 Navigation foundation', '5e12347', 'feat/agent-console', 26 * HOUR - 7 * MIN, '1 min 41 s', 0, 0, 0, { by: null }),
      run('Full run', 'Starting commit of the project', '79cd51d', 'master', 4 * DAY - HOUR, '5 min 48 s', 7380, 0, 12, { by: 'shell-test' }),
      run('Build', 'Starting commit of the project', '79cd51d', 'master', 4 * DAY - HOUR - 6 * MIN, '1 min 38 s', 0, 0, 0, { by: null }),
    ];
    w.fixes = [
      { sprint: 'fx1', why: 'The starting commit for LS-A01-3 failed its full run.', auto: true, state: 'Check passed. A full run comes next.' },
      { label: 'LS-A01-10', name: 'Reconnect banner says Offline too early', why: 'The banner text changes, and users would notice that.', auto: false, state: 'Waits behind LS-A01-9.' },
    ];
    w.result.statements = [
      st('The sidebar lists repositories, their projects, teams and agents as one tree.', 'holds'),
      st('Each level of the tree opens and closes on its own.', 'holds'),
      st('Control K opens a switcher that finds any project or agent.', 'holds'),
      st('A thread keeps its draft and its scroll position when you leave it.', 'holds'),
      st('A thread opens at its latest message.', 'building'),
      st('Tool calls in a thread fold under one line.', 'building'),
      st('A large item loads only when you ask for it.'),
      st('The sidebar folds to a 48 pixel rail.', 'building'),
      st('The sidebar, the bell and Home count the same open requests.', 'building'),
      st('An agent thread opens from the board.', 'building'),
      st('Keyboard order at 390 pixels follows the visual order.', 'review'),
      st('Every focused control shows a focus ring.', 'review'),
      st('The terminal key row never covers the last line.', 'review'),
      st('The drawer lists the project before the service pages.', 'review'),
    ];
  });

  add('rules', (w) => {
    w.reviews = [review('rv-rules', 'LS-A02-4 Rule storage and matching', 'Plan review', 'rules-plan', 'stuck',
      'Round 4 starts when statement 7 is settled. Rounds 5 and 6 each need your approval.', [
        { ts: at(21 * HOUR), findings: [
          fnd(7, 'blocker', 'Exact command text will not match the same command with other arguments.', 'open'),
          fnd(3, 'blocker', 'The plan stores rules in the settings file, which agents can write.'),
          fnd(2, 'major', 'No citation for where approvals are answered today.'),
          fnd(7, 'major', 'No test names a command with a changed filter.', 'open'),
          fnd(5, 'minor', 'The rule record has no creation time.'),
          fnd(9, 'minor', 'The Rules page is named two ways.'),
          fnd(1, 'minor', 'The statement cites a file that has moved.'),
          fnd(11, 'minor', 'Pause and remove are not told apart.')] },
        { ts: at(9 * HOUR), findings: [
          fnd(7, 'blocker', 'Matching is still by exact text.', 'open'),
          fnd(7, 'major', 'A rule for `pnpm test` would not cover `pnpm --filter web test`.', 'open'),
          fnd(4, 'major', 'A denied request can still create a rule.'),
          fnd(6, 'major', 'Matching runs after the prompt is shown, not before.'),
          fnd(8, 'major', 'Nothing limits how many rules one approval creates.'),
          fnd(3, 'minor', 'The store path is written in two forms.'),
          fnd(10, 'minor', 'Remove has no confirmation in the plan.'),
          fnd(12, 'minor', 'The rule count is not sourced.')] },
        { ts: at(2 * HOUR), findings: [
          fnd(7, 'blocker', 'Exact text misses the command the owner approved when its filter differs.', 'open'),
          fnd(7, 'major', 'The plan has no answer for leading arguments.', 'open'),
          fnd(7, 'minor', 'The example uses a command that no harness sends.', 'open'),
          fnd(7, 'minor', 'The statement does not say how letter case is treated.', 'open'),
          fnd(4, 'blocker', 'A rule made on one machine applies on every machine without saying so.', 'open'),
          fnd(6, 'major', 'Rule lookup has no test that fails first.', 'open'),
          fnd(10, 'major', 'Removing a rule does not cancel a request it already allowed.', 'open'),
          fnd(2, 'minor', 'A citation points at a line that has moved.', 'open'),
          fnd(12, 'minor', 'The end-result page does not name the rule count.', 'open')] },
      ])];
    w.stuck = { requestId: 'q-stuck', review: 'rv-rules', statement: 7, text: 'A rule matches by exact command text.', points: 14, of: 31, round: 3, rounds: 3 };
    w.runs = [run('Full run', 'Starting commit of the project', '4be2d65', 'master', 26 * HOUR, '6 min 9 s', 7792, 0, 12, { by: 'plan-r1' })];
    w.result.statements = [
      st('You can turn an approval into a standing rule with one choice.'),
      st('A standing rule is stored outside any file an agent can write.'),
      st('A rule records the command, the folder and the agent role it covers.'),
      st('A rule applies only on the machine where you made it.'),
      st('A rule records when it was made and by which request.'),
      st('A request that a rule covers is allowed before any prompt is shown.'),
      st('A rule matches by exact command text.', 'stuck', 'Stuck after round 3. Waiting for your decision.'),
      st('One approval makes at most one rule.'),
      st('The Rules page lists every standing rule.'),
      st('You can pause a rule, and you can remove it.'),
      st('A paused rule asks again, and keeps its record.'),
      st('The Rules page shows how often each rule was used.'),
    ];
  });

  add('phone', (w) => {
    w.result.statements = [
      st('The phone shows every request that waits for you, oldest first.'),
      st('You can answer a question or an approval without opening the laptop.'),
      st('You choose which of the four alerts reach the phone.'),
      st('A hidden approval says that its input cannot be seen.'),
    ];
    w.result.draft = true;
  });

  add('apiref', (w) => {
    w.reviews = [review('rv-apiref', 'docs/api-reference', 'Code review', 'apiref-review', 'clean', 'The review is clean. No blocker and no major is open.', [
      { ts: at(26 * HOUR), findings: [
        fnd(2, 'major', 'Three endpoints have no generated example.'),
        fnd(4, 'major', 'Old addresses return 404 instead of redirecting.'),
        fnd(5, 'major', 'The generator runs on every page load in development.'),
        fnd(2, 'minor', 'Two page titles differ from the OpenAPI summary.'),
        fnd(4, 'minor', 'The redirect list is not sorted.'),
        fnd(3, 'minor', 'One code sample is indented with tabs.'),
        fnd(1, 'minor', 'The sidebar order differs from the file.')] },
      { ts: at(2 * HOUR), findings: [] },
    ])];
    w.runs = [
      run('Full run', 'Before the merge into main', '9c1e7aa', 'docs/api-reference', 1 * HOUR + 40 * MIN, '1 min 52 s', 104, 0, 0, { by: 'apiref-r1' }),
      run('Full run', 'Merge of AD-A02-1 Generate the reference', '31f0be4', 'docs/api-reference', 2 * DAY, '1 min 37 s', 96, 0, 0, { by: 'apiref-r1' }),
    ];
    w.result.statements = [
      st('Every endpoint in openapi.yaml has a reference page.', 'holds'),
      st('Each page shows a request example and a response example.', 'holds'),
      st('Code samples come from the OpenAPI file, not from hand-written text.', 'holds'),
      st('Every old page address redirects to its new page.', 'holds'),
      st('The reference is built once, at build time.', 'holds'),
      st('The hand-written reference pages are gone.', 'holds'),
    ];
    w.result.open = ['The search index still lists two removed pages. It refreshes with the nightly run.', 'The changelog entry for this change is not written yet.'];
    w.result.pr = { number: 212, title: 'Generate the API reference from openapi.yaml', requestId: 'q-merge', source: 'docs/api-reference', target: 'main', commits: 14, files: 37 };
  });

  add('exports', (w) => {
    w.runs = [
      run('Check', 'BA-A01-4 · CSV writer with voided invoices', 'c07a5e1', 'feat/invoice-exports/build', 15 * MIN, '34 s', 31, 2, 0, { by: 'exports-b1', failures: [
        { name: 'a voided invoice keeps its original number', file: 'src/exports/csv.test.ts:41', message: 'Expected "INV-0042", received "".' },
        { name: 'the total row leaves out voided amounts', file: 'src/exports/csv.test.ts:67', message: 'Expected 1840.00, received 2310.00.' }] }),
      run('Full run', 'Starting commit of the project', '8e2b9d0', 'main', 2 * DAY - HOUR, '3 min 20 s', 1244, 0, 3, { by: 'exports-b1' }),
    ];
    w.settings.fullRun = 'make check';
    w.settings.check = 'pnpm vitest related';
    w.result.statements = [
      st('A month of invoices exports as one CSV file.', 'building'),
      st('Voided invoices are in the export, and are marked as voided.', 'building'),
      st('Month boundaries follow the account time zone.', 'building', 'Its builder failed to start on Buildbox.'),
      st('The same month exports as one PDF, with one invoice on each page.'),
      st('A large export downloads as a stream.'),
    ];
  });

  add('docsrefresh', (w) => {
    w.runs = [
      run('Nightly', 'No code changed. The run was skipped.', 'b7710c4', 'main', 9 * HOUR, '0 s', 0, 0, 0, { by: null, skippedRun: true }),
      run('Nightly', 'One commit since the last run', 'b7710c4', 'main', DAY + 9 * HOUR, '1 min 12 s', 88, 0, 1, { by: null, note: 'One skipped test was checked: it waits for the new search index.' }),
      run('Full run', 'Before the merge into main', '6f3d1aa', 'docs/refresh', 19 * DAY, '1 min 9 s', 88, 0, 0, { by: null }),
    ];
    w.result.statements = [
      st('The getting-started guide fits on one page.', 'holds'),
      st('Every command in the guide runs as written.', 'holds'),
      st('The guide links to the API reference, not to removed pages.', 'holds'),
    ];
    w.result.pr = { number: 198, title: 'Rewrite the getting-started guide', merged: true, mergedAt: at(19 * DAY), source: 'docs/refresh', target: 'main', commits: 9, files: 6 };
  });
}

// ---- phases ----------------------------------------------------------------------
interface MadeItem { icon: string; label: string; sub: string; to?: string }
interface PhaseInfo { ran: string[]; yours: string; ends: string; made: (s: State, p: Project, w: ProjectWorkflow) => (MadeItem | false | undefined)[] }
const INFO: Record<string, PhaseInfo> = {
  define: { ran: ['Project lead'], yours: 'Say what to change. Read the end-result page and answer "Build this?".', ends: 'Ends when you agree to the end-result page.',
    made: (s, p, w) => [
      { icon: 'file-text', label: 'End-result page', sub: w.result.statements.length ? plural(w.result.statements.length, 'numbered statement') : 'Not written', to: `/p/${p.id}/end-result` },
      { icon: 'file-code', label: '.lgtm config file', sub: `Tests ${p.tests}, agent review ${p.agentReview}`, to: `/p/${p.id}/settings/workflow` },
      p.hasMocks && { icon: 'frame', label: 'Design', sub: 'Screens drawn by the Design team', to: `/p/${p.id}/design` }] },
  design: { ran: ['Project lead', 'Test writer'], yours: 'Decide the sprints and the number of teams, only if the lead asks.', ends: 'Ends when every team has been started and sent its plan.',
    made: (s, p, w) => { const sp = sprintsOf(s, p.id), tm = teamsOf(s, p.id); return [
      { icon: 'test-tube-diagonal', label: 'Full-run report for the starting commit', sub: w.runs.length ? 'Kept with the runs in Actions' : 'No run', to: `/p/${p.id}/actions` },
      { icon: 'kanban', label: plural(sp.length, 'sprint'), sub: 'Sized in points and ordered by what depends on what', to: `/p/${p.id}/board` },
      { icon: 'users', label: plural(tm.length, 'team'), sub: tm.map(teamName).join(', ') || 'None', to: `/p/${p.id}/status` },
      { icon: 'route', label: 'User-flow tests', sub: 'Written before any code' }]; } },
  plan: { ran: ['Planning lead', 'Planners', 'Reviewer'], yours: 'Answer a planning question. Settle a stuck statement. Approve rounds 5 and 6.', ends: 'Ends when each plan review is clean and the lead has compared the plans across teams.',
    made: (s, p, w) => [
      { icon: 'clipboard-list', label: 'One plan for each team', sub: 'Every statement cites the code it changes' },
      { icon: 'gauge', label: 'Plan review rounds', sub: 'Findings scored in points', to: `/p/${p.id}/status/review` }] },
  build: { ran: ['Build leads', 'Builders', 'Test writer', 'Test runner'], yours: 'Answer questions and approvals as they come.', ends: 'Ends when every sprint has merged into the project branch with a passing report.',
    made: (s, p, w) => [
      { icon: 'list-checks', label: 'Tests that failed first', sub: 'Each test fails on its assertion before code is written' },
      { icon: 'git-commit-horizontal', label: 'Commits on the team branches', sub: 'One writer for each set of files', to: `/p/${p.id}/git` },
      { icon: 'test-tube-diagonal', label: 'Check reports', sub: 'A merge needs a passing report for that exact commit', to: `/p/${p.id}/actions` }] },
  verify: { ran: ['Test runner'], yours: 'Nothing, unless a user flow needs you.', ends: 'Ends when the smoke test, the full suite and the user flows pass.',
    made: (s, p, w) => [
      { icon: 'play', label: 'Install and smoke test', sub: 'The product starts from a clean checkout' },
      { icon: 'test-tube-diagonal', label: 'Full-suite report', sub: 'On the merged project branch', to: `/p/${p.id}/actions` },
      { icon: 'image', label: 'User-flow screenshots', sub: 'Web, phone size and the Mac simulator' }] },
  review: { ran: ['Review lead', 'Reviewer'], yours: 'Sort findings if asked. Settle a stuck statement.', ends: 'Ends when no blocker and no major is open.',
    made: (s, p, w) => [
      { icon: 'scan-search', label: 'Review findings', sub: 'Each one tied to a statement of the end-result page', to: `/p/${p.id}/status/review` },
      { icon: 'shield-check', label: 'Verdict', sub: 'Shown as evidence on the merge' }] },
  handover: { ran: ['Project lead'], yours: 'Try the product. Approve the merge for anything users would notice.', ends: 'Ends when the work is merged into the base branch.',
    made: (s, p, w) => [
      { icon: 'git-pull-request', label: 'Pull request to the base branch', sub: w.result.pr ? `#${w.result.pr.number}` : 'Opened when the review is clean', to: `/p/${p.id}/status/result` },
      { icon: 'file-text', label: 'Finished page', sub: 'Deliverables and evidence', to: `/p/${p.id}/status/result` }] },
  running: { ran: ['Test runner'], yours: 'Read the one-line summary.', ends: 'Runs every night while the code changes.',
    made: (s, p, w) => [
      { icon: 'history', label: 'Nightly report', sub: 'Failures are explained before you are told', to: `/p/${p.id}/actions` },
      { icon: 'book-open', label: 'Lessons', sub: 'Written into the skills the agents read' }] },
};
function phaseTimes(s: State, p: Project, cur: number): (PhaseTime | null)[] {
  if (p.phaseTimes !== undefined) return recordedPhaseTimes(p);
  const end = p.archived ? p.createdAt + 2 * DAY : s.T; const wt = [1, 2, 6, 9, 3, 4, 2, 1]; const total = wt.slice(0, cur + 1).reduce((a, b) => a + b, 0);
  let t = p.createdAt;
  return PHASES.map((_, i) => { if (i > cur) return null; const start = t; t += (end - p.createdAt) * wt[i]! / total; return { start, end: i < cur ? t : null }; });
}

function StuckCard({ s, p, w }: TabProps) {
  const now = useNow(); const k = w.stuck; if (!k) return null;
  const r = byId(s.requests, k.requestId); if (!r) return null;
  const a = agentOf(s, r.agent); const open = r.status === 'open';
  const settle = (id: string) => { act.answerRequest(r.id, id); update(() => {
    k.decision = id; const stt = w.result.statements[k.statement - 1]; const rv = byId(w.reviews, k.review);
    if (stt) { stt.state = id === 'drop' ? 'dropped' : 'open'; stt.note = id === 'drop' ? 'You dropped it after round 3.' : id === 'change' ? 'The Planning lead is rewriting it.' : 'You kept it as written.'; }
    if (rv) { rv.state = 'running'; rv.next = 'Round 4 is running. Rounds 5 and 6 each need your approval.'; } }); };
  return <section class={cx('req wf-stuck', !open && 'closed')} aria-label="Stuck statement">
    <div class="req-head">
      <span class={cx('kind', open ? 'question' : 'closed')}>Stuck statement</span>
      <span class="tag">statement {k.statement}</span>
      <span class="age">{open ? 'Waiting ' + ago(r.createdAt, now) : r.answer + ' · ' + clock(r.answeredAt!)}</span>
    </div>
    <blockquote class="wf-quote">{k.text}</blockquote>
    <ul class="wf-why">
      <li>
        <b class="num">{k.points} of {k.of}</b>
        {' points in round '}
        {k.round}
        {' '}
        <span class="muted">({Math.round(k.points / k.of * 100)}%, the limit is {w.settings.stuckPoints} points and 30%)</span>
      </li>
      <li>A major or a blocker in each of <b class="num">{k.rounds}</b> rounds</li>
    </ul>
    {r.recommendation && <p class="rec"><Icon name="sparkles" size={14} /><span><b>{a.name} recommends:</b> {r.recommendation}</span></p>}
    {open ? <div class="wf-choices">
      <button type="button" class="wf-choice primary" onClick={() => settle('change')}>
        <b>Rewrite</b>
        <span>The Planning lead rewrites the statement, and round 4 reviews it.</span>
      </button>
      <button type="button" class="wf-choice" onClick={() => settle('keep')}>
        <b>Keep</b>
        <span>It stays as written. Reviewers stop scoring it.</span>
      </button>
      <button type="button" class="wf-choice" onClick={() => settle('drop')}>
        <b>Drop</b>
        <span>It leaves the end-result page and the plan.</span>
      </button>
    </div> : <div class="answer">
      <Icon name="check" size={14} />
      <span>You chose to {k.decision || 'answer'}{k.decision ? ' it' : ''}. Round 4 is running.</span>
    </div>}
    <div class="req-foot">
      <Avatar agent={a} s={s} size="sm" />
      <span class="muted">{a.name}</span>
      <span class="grow" />
      <button type="button" class="btn sm ghost" onClick={() => act.openSidePanel(a.id, r.id)}>
        <Icon name="message-square" size={13} />
        Open thread
      </button>
    </div>
  </section>;
}

// The project's agents in three groups. Waiting hides when empty. Working and Idle show
// whenever the project has agents, and one hides only when it is empty and another shows.
export interface AgentGroup { id: string; label: string; agents: Agent[] }
export function agentGroups(s: State, p: Project): AgentGroup[] {
  const all = s.agents.filter((a) => a.project === p.id);
  if (!all.length) return [{ id: 'all', label: 'Agents', agents: [] }];
  const g: AgentGroup[] = [
    { id: 'waiting', label: 'Waiting', agents: all.filter((a) => a.crashed || a.state === 'waiting' || a.state === 'failed') },
    { id: 'working', label: 'Working', agents: all.filter((a) => a.state === 'working') },
    { id: 'idle', label: 'Idle', agents: all.filter((a) => !a.crashed && (a.state === 'idle' || a.state === 'stopped')) },
  ];
  const shown = (x: AgentGroup) => x.agents.length > 0;
  return g.filter((x) => x.id === 'waiting' ? shown(x) : shown(x) || !g.some((y) => y !== x && shown(y)));
}
function Agents({ s, p }: { s: State; p: Project }) {
  const groups = agentGroups(s, p);
  return <section class="wf-agents" aria-label="Agents">
    {groups.map((g) => <div key={g.id} class={cx('wf-group', g.id)}>
      <div class="wf-group-head"><h3>{g.label}</h3>{g.id !== 'all' && <span class="count">{g.agents.length}</span>}</div>
      {g.agents.length === 0 ? <p class="muted wf-none" data-empty>No agents running</p>
      : <ul class="wf-made">
        {g.agents.map((a) => <li key={a.id}>
        <a class={cx('wf-item', g.id === 'waiting' && 'warn')} href={href(routeForAgent(s, a))}>
          <Avatar agent={a} s={s} size="sm" />
          <span class="lbl">{agentLabel(s, a, { withTeam: true })}</span>
          <span class="sub truncate">{a.activity}</span>
          <Icon name="chevron-right" size={14} class="go" />
        </a>
      </li>)}
      </ul>}
    </div>)}
  </section>;
}

// The two checks of the build loop, each as one line that opens its own tab.
function Checks({ p, w }: { p: Project; w: ProjectWorkflow }) {
  const now = useNow(); const rv = w.reviews[0]; const round = rv?.rounds[rv.rounds.length - 1];
  const run = lastFullRun(w);
  const failed = !!run && run.failed > 0 && !run.expected && !run.running;
  return <section class="wf-agents" aria-label="Checks">
    <div class="wf-group">
      <div class="wf-group-head"><h3>Checks</h3></div>
      <ul class="wf-made">
        <li>
          <a class="wf-item" href={href(`/p/${p.id}/status/review`)}>
            <Icon name="scan-search" size={15} />
            <span class="lbl">Review</span>
            <span class="sub truncate">{rv ? `${rv.subject} · round ${rv.rounds.length} of ${w.settings.maxRounds}${round ? ' · ' + (roundPoints(round) ? plural(roundPoints(round), 'point') : 'clean') : ''}` : 'No review'}</span>
            <Icon name="chevron-right" size={14} class="go" />
          </a>
        </li>
        <li>
          <a class={cx('wf-item', failed && 'error')} href={href(`/p/${p.id}/actions`)}>
            <Icon name="test-tube-diagonal" size={15} />
            <span class="lbl">Actions</span>
            <span class="sub truncate">{run ? `${run.kind} · ${run.why} · ${run.running ? 'running now' : run.failed ? plural(run.failed, 'failure') : run.passed.toLocaleString('en-US') + ' passed'} · ${ago(run.ts, now)} ago` : 'No run'}</span>
            <Icon name="chevron-right" size={14} class="go" />
          </a>
        </li>
      </ul>
    </div>
  </section>;
}

function Overview({ s, p, w }: TabProps) {
  const now = useNow(); const cur = Math.max(0, PHASES.findIndex((x) => x.id === p.phase));
  // The open phase is in the address, so Back returns to it.
  const [pq, setPq] = useQuery('phase'); const at = PHASES.findIndex((x) => x.id === pq); const sel = at < 0 ? null : at; const setSel = (n: number | null) => setPq(n == null ? null : PHASES[n]!.id);
  const i = sel == null ? cur : sel; const ph = PHASES[i]!; const info = INFO[ph.id]!;
  const times = phaseTimes(s, p, cur); const tm = times[i]; const status = i < cur ? 'done' : i === cur ? 'now' : 'next';
  const reqs = answerableRequests(s, { project: p.id });
  const ran = ph.id === 'define' && p.hasMocks ? [...info.ran, 'Design lead', 'Designer'] : info.ran;
  const made = info.made(s, p, w).filter((m): m is MadeItem => !!m);
  const sps = sprintsOf(s, p.id); const done = sps.filter((x) => x.column === 'Done').length; const doing = sps.filter((x) => x.column === 'In progress' || x.column === 'Review').length;
  const fixes = w.fixes.map((f): FixSprint => { const sp = f.sprint && byId(s.sprints, f.sprint); return sp ? { ...f, label: sp.label, name: sp.name, to: `/p/${p.id}/sprint/${sp.id}`, column: sp.column } : f; });
  const started = times[cur];
  return <div class="wf-body">
    <ol class="wf-strip" aria-label="Phases of the build loop">
      {PHASES.map((x, n) => { const stt = n < cur ? 'done' : n === cur ? 'now' : 'next';
        return <li key={x.id} class={stt}>
          <button type="button" aria-expanded={sel === n} aria-current={n === cur ? 'step' : undefined} aria-label={`Phase ${x.n}, ${x.name}: ${stt === 'done' ? 'done' : stt === 'now' ? 'now' : 'not started'}`} onClick={() => setSel(sel === n ? null : n)}>
            <span class="mark">{stt === 'done' ? <Icon name="check" size={11} /> : stt === 'now' ? <i /> : x.n}</span>
            <span class="name">{x.name}</span>
          </button>
        </li>; })}
    </ol>
    <p class="wf-status">
      {p.archived ? <span class="badge">Archived</span> : null}
      <span>Phase <b class="num">{cur + 1}</b> of 8{started ? ` · started ${ago(started.start, now)} ago` : ''}</span>
      <span>
        {sps.length ? <><b class="num">{done}</b> of {plural(sps.length, 'sprint')} done · <b class="num">{doing}</b> active</> : 'No sprints'}
      </span>
      {w.result.statements.length > 0 && <a class="link" href={href(`/p/${p.id}/end-result`)}>
        <b class="num">{w.result.statements.filter((x) => x.state === 'holds').length}</b> of {w.result.statements.length} statements hold
      </a>}
    </p>
    {sel != null && <section class="wf-detail wf-open" aria-label={ph.name}>
        <header>
          <h2><span class="faint num">{ph.n}</span> {ph.name}</h2>
          {status !== 'now' && <span class="muted">{status === 'done' ? 'Done' : 'Not started'}</span>}
          <span class="grow" />
          <span class="muted wf-time">
            {status === 'done' ? (tm?.end != null ? `${when(tm.start, now)} · took ${ago(tm.start, tm.end)}` : '') : status === 'now' ? (tm ? `Started ${ago(tm.start, now)} ago` : '') : `After ${PHASES[i - 1]!.name}`}
          </span>
        </header>
        <dl class="wf-parts">
          <dt>Description</dt>
          <dd>{ph.what}</dd>
          <dt>Agents</dt>
          <dd class="row" style="flex-wrap:wrap;gap:4px">{ran.map((r) => <span class="badge outline" key={r}>{r}</span>)}</dd>
          <dt>Owner input</dt>
          <dd>{info.yours}</dd>
          <dt>Exit condition</dt>
          <dd>{info.ends.replace(/^Ends when /, 'When ').replace(/^Runs /, 'It runs ')}</dd>
          <dt>{status === 'next' ? 'Planned outputs' : 'Outputs'}</dt>
          <dd>
            <ul class="wf-made">
              {made.map((m) => { const C = (m.to && status !== 'next' ? 'a' : 'div') as 'a';
            return <li key={m.label}>
              <C class="wf-item" href={m.to && status !== 'next' ? href(m.to) : undefined}>
                <Icon name={m.icon} size={15} />
                <span class="lbl">{m.label}</span>
                <span class="sub truncate">{status === 'next' ? '' : m.sub}</span>
                {m.to && status !== 'next' && <Icon name="chevron-right" size={14} class="go" />}
              </C>
            </li>; })}
            </ul>
          </dd>
        </dl>
      </section>}
    <StuckCard s={s} p={p} w={w} />
    <Agents s={s} p={p} />
    <Checks p={p} w={w} />
    <div class="section-head wf-sec"><h2>Fix sprints</h2><span class="count">{fixes.length}</span></div>
    <div class="table-wrap">
      <table class="table wf-fixes">
        <thead><tr><th>Sprint</th><th>Name</th><th>Reason</th><th>Merge</th><th>State</th></tr></thead>
        <tbody>
          {fixes.length === 0 ? <tr><td colspan={5} class="muted" data-empty>No fix sprints</td></tr>
      : fixes.map((f) => <tr key={f.label} class={cx(f.to && 'click', !f.auto && 's-warn')} onClick={() => f.to && go(f.to)}>
        <td class="wrap"><span class="tag">{f.label}</span></td>
        <td class="wrap">{f.to ? <a class="wf-strong" href={href(f.to)}>{f.name}</a> : <span class="wf-strong">{f.name}</span>}</td>
        <td class="wrap muted">{f.why}</td>
        <td class="wrap">
          {f.auto && w.settings.autoMergeFix ? <span class="muted">Automatic</span> : <span class="warn-text">Approval required</span>}
        </td>
        <td class="wrap muted">{f.column === 'Done' ? 'Merged.' : f.state}</td>
      </tr>)}
        </tbody>
      </table>
    </div>
  </div>;
}

// ---- review rounds -----------------------------------------------------------------
function Review({ s, p, w }: TabProps) {
  const now = useNow(); const [rid, setRid] = useQuery('round'); const [fq, setFq] = useQuery('finding'); const [sq, setSq] = useQuery('show');
  // The round, the open finding and the filter are in the address, so Back returns to them.
  const sel = fq == null ? null : +fq; const setSel = (n: number | null) => setFq(n == null ? null : String(n)); const filter = sq || 'all'; const setFilter = (v: string) => setSq(v === 'all' ? null : v);
  // With no review yet, the page keeps its rounds and its findings table, empty.
  if (!w.reviews.length) return <div class="wf-body">
    <p class="wf-status">
      <span>No review</span>
      <span class="grow" />
      <span class="badge">{p.agentReview === 'off' ? 'Agent review off' : 'Not started'}</span>
    </p>
    <div class="wf-rounds">
      {Array.from({ length: w.settings.maxRounds }, (_, k) => <div key={k} class="wf-round empty"><div class="bar" /><b>Round {k + 1}</b><span>{k + 1 >= 5 ? 'Approval required' : '—'}</span></div>)}
    </div>
    <p class="muted wf-none" data-empty>No findings</p>
  </div>;
  const rv = byId(w.reviews, rid) || w.reviews[0]!; const max = w.settings.maxRounds;
  const n = Math.min(sel || rv.rounds.length, rv.rounds.length); const round = rv.rounds[n - 1]!;
  // The tallest bar of any round sets the scale, so the rounds can be compared.
  const top = Math.max(1, ...rv.rounds.flatMap((r) => rv.reviewers.map((_, who) => r.findings.filter((f) => f.by === who).reduce((sum, f) => sum + POINTS[f.sev], 0)))); const team = byId(s.teams, rv.team);
  const list = round.findings.filter((f) => filter === 'all' || f.status === filter).sort((a, b) => POINTS[b.sev] - POINTS[a.sev] || a.statement - b.statement);
  const openN = round.findings.filter((f) => f.status === 'open').length;
  const state = ({ stuck: ['warn', 'Stuck'], scoring: ['info', 'Being scored'], running: ['info', 'Round running'], clean: ['success', 'Clean'] } as Record<string, string[]>)[rv.state] || ['', rv.state];
  return <div class="wf-body">
    {w.reviews.length > 1 && <div class="seg" style="justify-self:start">
      {w.reviews.map((x) => <button type="button" key={x.id} aria-pressed={x.id === rv.id} onClick={() => { setRid(x.id); setSel(null); }}>{x.subject}</button>)}
    </div>}
    <header class="wf-rvhead">
      <div class="grow">
        <h2>{rv.subject}</h2>
        <p class="muted">
          {rv.kind}
          {team ? <> by the <a class="link" href={href(`/p/${p.id}/team/${team.id}`)}>{teamName(team)}</a></> : ''}
          {' · round '}
          {rv.rounds.length}
          {' of '}
          {max}
        </p>
      </div>
      <span class={cx('badge', state[0])}>{state[1]}</span>
    </header>
    <StuckCard s={s} p={p} w={w.stuck && w.stuck.review === rv.id ? w : { ...w, stuck: null }} />
    <div class="wf-rounds" role="tablist" aria-label="Review rounds">
      {Array.from({ length: Math.max(max, rv.rounds.length) }, (_, k) => { const r = rv.rounds[k]; const pts = r ? roundPoints(r) : 0;
        if (!r) return <div key={k} class="wf-round empty">
          <div class="bar" />
          <b>Round {k + 1}</b>
          <span>
            {k + 1 >= 5 ? 'Approval required' : k === rv.rounds.length ? (rv.state === 'clean' ? 'Not needed' : rv.state === 'running' ? 'Running' : 'Next') : 'Only if needed'}
          </span>
        </div>;
        const per = rv.reviewers.map((_, who) => r.findings.filter((f) => f.by === who).reduce((sum, f) => sum + POINTS[f.sev], 0));
        return <button type="button" key={k} role="tab" class="wf-round" aria-selected={n === k + 1} onClick={() => setSel(k + 1)}>
          <div class="bar" role="img" aria-label={rv.reviewers.map((name, who) => `${name}: ${plural(per[who]!, 'point')}`).join(', ')}>
            {pts === 0 ? <Icon name="check" size={14} class="success-text" /> : per.map((v, who) => <span key={who} class="wf-col">
              <span class="num">{v}</span>
              <i class={'by' + who} style={{ height: v ? Math.max(3, v / top * 100) + '%' : '1px' }} />
            </span>)}
          </div>
          <b>Round {k + 1}</b>
          <span class="num">{pts === 0 ? 'Clean' : plural(pts, 'point')}</span>
        </button>; })}
    </div>
    <p class="wf-legend">
      {rv.reviewers.map((name, who) => <span key={name}><i class={'sw by' + who} />{name}</span>)}
    </p>
    <div class="section-head wf-sec">
      <h2>Round {n} findings</h2>
      <span class="count">{round.findings.length}</span>
      <span class="muted wf-time">Scored {when(round.ts, now)}</span>
      <span class="grow" />
      {round.findings.length > 0 && <div class="seg">
        {[['all', 'All'], ['open', `Not fixed ${openN}`], ['fixed', `Fixed ${round.findings.length - openN}`]].map(([id, label]) => <button type="button" key={id} aria-pressed={filter === id} onClick={() => setFilter(id)}>{label}</button>)}
      </div>}
    </div>
    {list.length === 0 ? <p class="muted wf-none" data-empty>{round.findings.length === 0 ? 'No findings. The review is clean.' : `No ${filter} findings`}</p>
    : <ul class="wf-finds">
      {list.map((f) => <li key={f.id} class={cx(f.status === 'fixed' && 'fixed')}>
        <span class={cx('wf-sev', f.sev)}><i />{SEV_LABEL[f.sev]}<span class="num">{POINTS[f.sev]} {POINTS[f.sev] === 1 ? 'pt' : 'pts'}</span></span>
        <div>
          <p>{f.text}</p>
          <small>
            <span class="mono">{f.id}</span>
            <a class="link" href={href(`/p/${p.id}/end-result`)} title={w.result.statements[f.statement - 1]?.text}>Statement {f.statement}</a>
            <span><i class={'sw by' + f.by} />{rv.reviewers[f.by]}</span>
            {w.stuck && w.stuck.statement === f.statement && w.stuck.review === rv.id && !w.stuck.decision && <span class="warn-text">Stuck</span>}
          </small>
        </div>
        {f.status === 'fixed' ? <span class="wf-find-state success-text"><Icon name="check" size={13} />Fixed</span> : <span class="wf-find-state">Not fixed</span>}
      </li>)}
    </ul>}
  </div>;
}

// ---- actions -----------------------------------------------------------------------
// The runs of the repository's actions, laid out like GitHub Actions: the runs on the
// left, and the selected run's jobs, their steps, and the log of the selected step on the right.
const fileOf = (kind: string) => ACTIONS.find((x) => x.kind === kind) || ACTIONS[0]!;
// The newest run that says whether the whole product passes its tests.
export const lastFullRun = (w: ProjectWorkflow) => w.runs.filter((r) => r.kind !== 'Check' && r.kind !== 'Build' && !r.skippedRun).sort((a, b) => b.ts - a.ts)[0];
const toneOf = (r: TestRun) => (r.running ? 'run' : r.skippedRun ? 'skip' : r.failed ? (r.expected ? 'exp' : 'fail') : 'pass');
const TONE_ICON: Record<string, string> = { run: 'loader-circle', skip: 'minus', exp: 'circle-dot', fail: 'circle-x', pass: 'circle-check' };
const TONE_WORD: Record<string, string> = { run: 'Running', skip: 'Skipped', exp: 'Failed as expected', fail: 'Failed', pass: 'Passed' };
const secs = (d: string) => { const m = /(?:(\d+) min)?\s*(?:(\d+) s)?/.exec(d || '')!; return (+(m[1] || 0)) * 60 + (+(m[2] || 0)); };
const dur = (n: number) => (n >= 60 ? `${Math.floor(n / 60)} min ${n % 60} s` : `${n} s`);
// The jobs and steps of a run, made from its record so every run has the same shape.
interface RunStep { name: string; tone: string; dur: string; log: string[] }
interface RunJob { name: string; tone: string; steps: RunStep[] }
function jobsOf(s: State, p: Project, r: TestRun): RunJob[] {
  const t = toneOf(r); const total = secs(r.dur); const cmd = r.kind === 'Check' ? ensure(s, p).settings.check : ensure(s, p).settings.fullRun;
  const st = (name: string, tone: string, n: number, log: string[]): RunStep => ({ name, tone, dur: dur(Math.max(1, Math.round(n))), log });
  const setup = (n: number) => [
    st('Check out', 'pass', 2, [`Checking out ${r.commit} on ${r.branch}`, `HEAD is now at ${r.commit}`]),
    st('Install', 'pass', n, ['Lockfile is up to date', 'Packages are installed from the cache', 'Done']),
  ];
  if (r.skippedRun) return [{ name: 'Changes', tone: 'skip', steps: [st('Compare with the last run', 'skip', 1, [`No commit since ${r.commit}`, 'The run was skipped'])] }];
  const failLog = r.failures.flatMap((f) => [`FAIL ${f.file}`, `  ✕ ${f.name}`, `    ${f.message}`, '']);
  const testTone = t === 'run' ? 'run' : t;
  const testLog = [`> ${cmd}`, '', ...failLog,
    t === 'run' ? 'Running…' : `Tests: ${r.failed ? r.failed + ' failed, ' : ''}${r.passed} passed${r.skipped ? ', ' + r.skipped + ' skipped' : ''}, ${r.passed + r.failed + r.skipped} total`,
    ...(r.note ? ['', r.note] : [])];
  if (r.kind === 'Build') return [{ name: 'Build', tone: testTone, steps: [...setup(total * 0.2), st('Build the website', testTone, total * 0.6, ['> pnpm build', '', t === 'run' ? 'Building…' : 'Built 4 packages']), st('Keep the build', t === 'run' ? 'queued' : 'pass', total * 0.15, ['Saved the build of commit ' + r.commit])] }];
  if (r.kind === 'Check') return [{ name: 'Check', tone: testTone, steps: [...setup(total * 0.2), st('Run the related tests', testTone, total * 0.75, testLog), st('Write the report', t === 'run' ? 'queued' : 'pass', 1, ['Report saved for commit ' + r.commit])] }];
  const unit = Math.round(total * 0.55), flows = Math.round(total * 0.3);
  return [
    { name: 'Set up', tone: 'pass', steps: setup(total * 0.1) },
    { name: 'Tests', tone: testTone, steps: [st('Run the full suite', testTone, unit, testLog), st('Write the report', t === 'run' ? 'queued' : 'pass', 1, ['Report saved for commit ' + r.commit])] },
    { name: 'User flows', tone: t === 'run' ? 'queued' : 'pass', steps: [st('Start the product', t === 'run' ? 'queued' : 'pass', flows * 0.3, ['The product started from a clean checkout']), st('Run the user flows', t === 'run' ? 'queued' : 'pass', flows * 0.7, ['Web, phone size and the Mac simulator', 'All flows passed'])] },
  ];
}
const StepIcon = ({ tone, size = 14 }: { tone: string; size?: number }) => tone === 'queued' ? <span class="wf-res queued"><Icon name="circle" size={size} /></span>
  : <span class={cx('wf-res', tone)}><Icon name={TONE_ICON[tone]} size={size} class={tone === 'run' ? 'wf-spin' : ''} /></span>;
function RunDetail({ s, p, r, now }: { s: State; p: Project; r: TestRun | undefined; now: number }) {
  const jobs = r ? jobsOf(s, p, r) : [];
  const firstBad = jobs.findIndex((j) => j.tone === 'fail' || j.tone === 'run');
  const [ji, setJi] = useState(Math.max(0, firstBad));
  const job = jobs[ji] || jobs[0];
  const pick = (j: RunJob | undefined) => { if (!j) return -1; const k = j.steps.findIndex((x) => x.tone === 'fail' || x.tone === 'run'); return k >= 0 ? k : j.steps.findIndex((x) => /^(Run|Compare|Build)/.test(x.name)); };
  const [open, setOpen] = useState(() => pick(job));
  const by = r && r.by && agentOf(s, r.by);
  return <section class="wf-rundetail" aria-label="Run">
    <header class="wf-runhead">
      {r ? <>
      <StepIcon tone={toneOf(r)} size={16} />
      <b>{r.kind}</b>
      <span class="truncate">{r.why}</span>
      <span class="grow" />
      <span class="tag commit" title={r.branch}>{r.commit}</span>
    </> : <span class="muted">No run selected</span>}
    </header>
    {r && <div class="wf-runmeta">
      <span class={cx('wf-res', toneOf(r))}>{TONE_WORD[toneOf(r)]}</span>
      {!r.running && !r.skippedRun && fileOf(r.kind).tests && <>
        <span class="num">{fmt(r.passed)} passed</span>
        {r.failed > 0 && <span class={cx('num', r.expected ? 'muted' : 'error-text')}>{fmt(r.failed)} failed</span>}
        {r.skipped > 0 && <span class="num muted">{fmt(r.skipped)} skipped</span>}
      </>}
      <span class="muted"><span class="mono">{r.branch}</span></span>
      <span class="muted num">
        {r.running ? 'Started' : r.dur + ' ·'}
        {' '}
        {ago(r.ts, now) === 'now' ? 'just now' : ago(r.ts, now) + ' ago'}
      </span>
      <span class="muted">{by ? by.name : 'LGTM Studio'}</span>
      <span class="muted mono">.github/workflows/{fileOf(r.kind).file}</span>
    </div>}
    <div class="wf-jobs">
      <nav class="wf-joblist" aria-label="Jobs">
        <h3>Jobs</h3>
        {jobs.length === 0 && <p class="muted wf-none" data-empty>No jobs</p>}
        {jobs.map((j, k) => <button type="button" key={j.name} aria-current={k === ji ? 'true' : undefined} onClick={() => { setJi(k); setOpen(pick(j)); }}>
          <StepIcon tone={j.tone} />
          <span class="truncate">{j.name}</span>
        </button>)}
      </nav>
      <div class="wf-steps">
        <h3>{job ? job.name : 'Steps'}</h3>
        {!job && <p class="muted wf-none" data-empty>No steps</p>}
        {job && job.steps.map((x, k) => <div key={x.name} class={cx('wf-step-row', open === k && 'open')}>
          <button type="button" aria-expanded={open === k} onClick={() => setOpen(open === k ? -1 : k)}>
            <Icon name="chevron-right" size={13} class="twist" />
            <StepIcon tone={x.tone} />
            <span class="grow truncate">{x.name}</span>
            <span class="muted num">{x.tone === 'queued' ? '' : x.dur}</span>
          </button>
          {open === k && <pre class="wf-log">
            {x.log.map((line, n) => <span key={n} class={cx(/^(FAIL|  ✕)/.test(line) && 'bad')}><i>{n + 1}</i>{line}</span>)}
          </pre>}
        </div>)}
      </div>
    </div>
  </section>;
}
// The Actions view: the runs on the left with the open run beside them, and a rail on the right
// that lists the actions themselves: each file, when it runs, and its jobs and steps.
// A run started by hand copies the numbers of the last run of its action.
function startRun(w: ProjectWorkflow, p: Project, wf: ActionFile): Id | null {
  if (act.refused()) return null;
  const id = uid('run');
  const prev = w.runs.filter((r) => r.kind === wf.kind && !r.skippedRun && !r.running).sort((a, b) => b.ts - a.ts)[0];
  update(() => { w.runs.push({ id, kind: wf.kind, why: 'Started by you', commit: (prev && prev.commit) || 'HEAD', branch: p.branch, ts: Date.now(), dur: '—', passed: 0, failed: 0, skipped: 0, failures: [], running: true, by: null }); });
  act.sampleLater(() => update(() => { const r = byId(w.runs, id)!; r.running = false; r.passed = wf.tests ? (prev ? prev.passed + prev.failed : 1240) : 0; r.skipped = prev ? prev.skipped : 0; r.dur = prev ? prev.dur : '3 min 2 s'; r.ts = Date.now(); act.toast(`${wf.kind} passed`); }), 3200);
  return id;
}
const slugOf = (a: ActionFile) => a.file.replace(/\.ya?ml$/, '');
function Runs({ s, p, w, route }: TabProps & { route: Route }) {
  const now = useNow(5000);
  // The chosen action and the open run are in the address, so Back from another view returns to them.
  // A choice takes the place of the last one, so Back leaves the view in one step.
  const kind = ACTIONS.find((a) => slugOf(a) === route.query.get('action'))?.kind ?? 'all'; const rid = route.query.get('run');
  const show = (k: string, r: Id | null) => { const a = ACTIONS.find((x) => x.kind === k); const q = new URLSearchParams(); if (a) q.set('action', slugOf(a)); if (r) q.set('run', r);
    swap(`/p/${p.id}/actions${q.size ? '?' + q : ''}`); };
  const setKind = (k: string) => show(k, null); const setRid = (r: Id) => show(kind, r);
  const runs = w.runs.filter((r) => kind === 'all' || r.kind === kind).sort((a, b) => b.ts - a.ts);
  const sel = runs.find((r) => r.id === rid) || runs[0];
  const busy = w.runs.some((r) => r.running);
  const start = (wf: ActionFile) => { const id = startRun(w, p, wf); if (id) show('all', id); };
  const off = busy || p.archived;
  return <div class="wf-body wf-wide ac-outer">
    <div class="wf-bar">
      <Back path={route.path} />
      <div class="seg">
        {['all', ...ACTIONS.map((x) => x.kind)].map((k) => <button type="button" key={k} aria-pressed={kind === k} onClick={() => setKind(k)}>
        {k === 'all' ? 'All' : k}
        <span class="faint num">{k === 'all' ? w.runs.length : w.runs.filter((r) => r.kind === k).length}</span>
      </button>)}
      </div>
      <span class="grow" />
      {p.tests === 'off' && <span class="muted wf-time">Tests are off</span>}
      <Menu class="btn" label="Run an action" align="end" width={248} disabled={off} trigger={<><Icon name="play" size={13} />Run action<Icon name="chevron-down" size={13} /></>} items={ACTIONS.map((x) => ({ label: x.kind, hint: x.file, disabled: x.tests && p.tests === 'off', onSelect: () => start(x) }))} />
    </div>
    <div class="ac-wrap">
      <div class="wf-actions">
        <ul class="wf-runlist" aria-label="Runs">
          {runs.length === 0 && <li class="muted wf-none" data-empty>No runs</li>}
          {runs.map((r) => { const t = toneOf(r); return <li key={r.id}>
            <button type="button" aria-current={sel && sel.id === r.id ? 'true' : undefined} onClick={() => setRid(r.id)}>
              <StepIcon tone={t} />
              <span class="what">
                <b title={r.why}>{r.why}</b>
                <small class="muted">
                  <span class={t === 'fail' ? 'error-text' : undefined}>{TONE_WORD[t]}</span>
                  {' · '}
                  <span>{r.kind}</span>
                  {' · '}
                  <span class="mono">{r.commit}</span>
                  {' · '}
                  {ago(r.ts, now) === 'now' ? 'just now' : ago(r.ts, now) + ' ago'}
                </small>
              </span>
            </button>
          </li>; })}
        </ul>
        <RunDetail key={sel ? sel.id + (sel.running ? 'r' : '') : 'none'} s={s} p={p} r={sel} now={now} />
      </div>
      <aside class="ac-rail" aria-label="Actions">
        <h3>Actions</h3>
        {ACTIONS.map((a) => <div key={a.file} class="ac-def">
          {/* The name shows the runs of this action, and a second click shows every run again. */}
          <button type="button" class="ac-def-name" aria-pressed={kind === a.kind} data-tip={a.on} data-tip-side="left" onClick={() => setKind(kind === a.kind ? 'all' : a.kind)}>{a.kind}</button>
          <a class="link mono ac-def-file" href={href(`/p/${p.id}/files?f=${encodeURIComponent(actionPath(a))}`)} aria-label={`Open ${actionPath(a)}`}>{a.file}</a>
        </div>)}
      </aside>
    </div>
  </div>;
}

// ---- end-result page ---------------------------------------------------------------
const ST_WORD: Record<string, [string, string]> = { holds: ['success', 'Holds'], building: ['info', 'Being built'], review: ['info', 'In review'], stuck: ['warn', 'Stuck'], dropped: ['', 'Dropped'], open: ['', 'Not started'] };
function Result({ s, p, w }: TabProps) {
  const now = useNow(); const res = w.result; const sts = res.statements; const lead = leadOf(s, p);
  const live = sts.filter((x) => x.state !== 'dropped'); const holds = live.filter((x) => x.state === 'holds').length;
  const finished = p.phase === 'handover' || p.phase === 'running'; const pr = res.pr; const merged = pr && (pr.merged || p.merged);
  const req = pr && pr.requestId && byId(s.requests, pr.requestId);
  const sprints = sprintsOf(s, p.id); const lastFull = lastFullRun(w); const rv = w.reviews[0];
  return <div class="wf-body">
    <div class="wf-status">
      {res.draft ? <span class="badge">Draft</span> : finished ? <span class={cx('badge', merged ? 'success' : 'info')}>{merged ? 'Merged' : 'Ready'}</span> : null}
      <span><b class="num">{holds}</b> of {plural(live.length, 'statement')} hold</span>
      <span class="grow" />
      <div class="progress wf-prog" title={`${holds} of ${live.length}`}>
        {live.map((x, i) => <i key={i} class={cx(x.state === 'stuck' && 'warn', (x.state === 'building' || x.state === 'review') && 'active')} style={{ flex: 1, background: x.state === 'open' ? 'transparent' : undefined }} />)}
      </div>
    </div>
    {pr && <section class={cx('wf-pr', merged ? 'merged' : 'wait')}>
      <Icon name={merged ? 'git-merge' : 'git-pull-request'} size={18} />
      <div class="grow">
        <b>{pr.title} <span class="muted num">#{pr.number}</span></b>
        <p class="muted">
          <span class="mono">{pr.source}</span>
          {' into '}
          <span class="mono">{pr.target}</span>
          {' · '}
          {pr.commits}
          {' commits · '}
          {pr.files}
          {' files · '}
          {merged ? `merged ${pr.mergedAt ? ago(pr.mergedAt, now) + ' ago' : 'by you'}` : 'waiting for you'}
        </p>
      </div>
      {merged ? <span class="badge success">Merged</span> : req && req.status === 'open' ? <button type="button" class="btn primary" onClick={() => act.openDialog({ type: 'merge-review', requestId: req.id })}>
        Review and merge
      </button> : <span class="badge">{req ? req.answer : 'Open'}</span>}
    </section>}
    {finished && <div class="wf-two">
      <section>
        <h3>Deliverables</h3>
        <ul class="wf-built">
          {sprints.map((sp) => <li key={sp.id}><a class="link" href={href(`/p/${p.id}/sprint/${sp.id}`)}>{sp.label} {sp.name}</a><span>{sp.goal}</span></li>)}
          {sprints.length === 0 && <li><span>{p.goal}</span></li>}
        </ul>
      </section>
      <section>
        <h3>Evidence</h3>
        <ul class="wf-ev">
          {lastFull && <li>
          <Icon name={lastFull.failed ? 'triangle-alert' : 'circle-check'} size={14} class={lastFull.failed ? 'warn-text' : 'success-text'} />
          <span>
            {'Full run '}
            {lastFull.failed ? 'failed' : 'passed'}
            {' · '}
            {fmt(lastFull.passed)}
            {' tests · commit '}
            <span class="mono">{lastFull.commit}</span>
          </span>
          <a class="link" href={href(`/p/${p.id}/actions`)}>Runs</a>
        </li>}
          {rv && <li>
          <Icon name={rv.state === 'clean' ? 'circle-check' : 'triangle-alert'} size={14} class={rv.state === 'clean' ? 'success-text' : 'warn-text'} />
          <span>Review {rv.state === 'clean' ? `clean after round ${rv.rounds.length}` : `open in round ${rv.rounds.length}`}</span>
          <a class="link" href={href(`/p/${p.id}/status/review`)}>Rounds</a>
        </li>}
          <li><Icon name="circle-check" size={14} class="success-text" /><span>{holds} of {live.length} statements hold</span></li>
        </ul>
        <h3 style="margin-top:var(--space-4)">Open items</h3>
        {res.open.length ? <ul class="wf-open">{res.open.map((o, i) => <li key={i}>{o}</li>)}</ul> : <p class="muted">None</p>}
      </section>
    </div>}
    <div class="section-head wf-sec">
      <h2>End-result page</h2>
      <span class="count">{sts.length}</span>
      <span class="grow" />
      <a class="btn sm ghost" href={href(routeForAgent(s, lead))}><Icon name="message-square" size={13} />Request change</a>
    </div>
    <ol class="wf-statements">
      {sts.length === 0 && <li class="open" data-empty><span class="n num" /><div class="t"><span>No statements</span></div><span /></li>}
      {sts.map((x, i) => { const [tone, word] = ST_WORD[x.state] || ST_WORD.open!;
      return <li key={i} class={x.state}>
        <span class="n num">{i + 1}</span>
        <div class="t"><span>{x.text}</span>{x.note && <small>{x.note}</small>}</div>
        <span class={cx('badge', tone)}>{x.state === 'holds' && <Icon name="check" size={11} />}{word}</span>
      </li>; })}
    </ol>
  </div>;
}

// ---- settings --------------------------------------------------------------------
// The page has the same rows as the project's other settings: the name at the left, and its control
// in one column beside it. A setting is on this page only, under one name.
function Settings({ s, p, w }: TabProps) {
  const Row = ProjectSettingRow;
  const set = w.settings;
  const save = <K extends keyof WorkflowSettings>(k: K, v: WorkflowSettings[K]) => { if (set[k] === v) return; update(() => { set[k] = v; }); markSaved(); };
  const proj = (k: 'tests' | 'agentReview', v: boolean) => { update(() => { p[k] = v ? 'on' : 'off'; }); markSaved(); };
  const step = (k: 'maxRounds' | 'stuckPoints', d: number, lo: number, hi: number) => save(k, Math.min(hi, Math.max(lo, set[k] + d)));
  const command = (k: 'fullRun' | 'check', label: string) => <input id={'wf-' + k} class="input mono" aria-label={label} disabled={p.tests !== 'on'} value={set[k]}
    onBlur={(e) => save(k, e.currentTarget.value.trim() || set[k])} onKeyDown={(e) => e.key === 'Enter' && e.currentTarget.blur()} />;
  const stepper = (k: 'maxRounds' | 'stuckPoints', lo: number, hi: number, less: string, more: string, unit?: string) => <div class="wf-step">
    <button type="button" class="btn icon sm" aria-label={less} disabled={set[k] <= lo} onClick={() => step(k, -1, lo, hi)}><Icon name="minus" size={13} /></button>
    <b class="num">{set[k]}</b>
    <button type="button" class="btn icon sm" aria-label={more} disabled={set[k] >= hi} onClick={() => step(k, 1, lo, hi)}><Icon name="plus" size={13} /></button>
    {unit && <span class="muted">{unit}</span>}
  </div>;
  return <div class="page svc-project">
    <dl class="ps-facts">
      <div><dt>Workflow</dt><dd><a class="link" href={href('/workflows')}>Build loop</a></dd></div>
      <div><dt>Saved to</dt><dd class="mono">.lgtm/config</dd></div>
      <div><dt>On</dt><dd class="mono truncate-start" title={p.branch}>{'‎' + p.branch}</dd></div>
    </dl>
    <Section title="Tests">
      <div class="svc-rows">
        <Row label="Write tests first" hint="Tests are written before the code, and they must fail first. A merge needs a passing report.">
          <Switch on={p.tests === 'on'} onChange={(v) => proj('tests', v)} label="Write tests first" />
        </Row>
        <Row label="Full-run command" id="wf-fullRun" hint="Runs the whole suite on a starting commit, before a merge, and every night.">{command('fullRun', 'Full-run command')}</Row>
        <Row label="Check command" id="wf-check" hint="Runs the tests of one task after its builder commits.">{command('check', 'Check command')}</Row>
        <Row label="Nightly full run" hint="Runs when the code changed that day. You get a one-line summary.">
          <Switch on={set.nightly} onChange={(v) => save('nightly', v)} label="Nightly full run" />
        </Row>
      </div>
    </Section>
    <Section title="Review">
      <div class="svc-rows">
        <Row label="Agent review" hint="Reviewers score each plan and each change in rounds. A minor finding scores 1 point, a major 3 and a blocker 9. The Review phase runs even when this is off.">
          <Switch on={p.agentReview === 'on'} onChange={(v) => proj('agentReview', v)} label="Agent review" />
        </Row>
        <Row label="Most review rounds" hint="At the last round you cut, split or accept. Rounds 5 and 6 always ask you first.">
          {stepper('maxRounds', 1, 6, 'One round fewer', 'One round more')}
        </Row>
        <Row label="Stuck at" hint="A statement is stuck when it has this many points and 30% of a round, or a major or blocker in 3 rounds. You then keep, change or drop it.">
          {stepper('stuckPoints', 9, 27, 'Fewer points', 'More points', 'points')}
        </Row>
      </div>
    </Section>
    <Section title="Merging">
      <div class="svc-rows">
        <Row label="Fix sprints merge on their own" hint="Only a fix that users would not notice, and only after a passing full run. Anything users would notice comes to you, as does every merge into the base branch.">
          <Switch on={set.autoMergeFix} onChange={(v) => save('autoMergeFix', v)} label="Fix sprints merge on their own" />
        </Row>
        <Row label="Remove agent credits" hint="A merge strips the co-author lines that agents add to their commits.">
          <Switch on={p.removeCredits ?? s.settings.removeCredits} label="Remove agent credits" onChange={(v) => { update(() => { p.removeCredits = v; }); markSaved(); }} />
        </Row>
      </div>
    </Section>
  </div>;
}

// ---- the tab ---------------------------------------------------------------------
// The workflow's settings, as a page of the project's Settings.
export function WorkflowSettings({ s, project }: { s: State; project: Project }) {
  return <Settings s={s} p={project} w={ensure(s, project)} />;
}
// Actions is a view of its own in the row of views: it is where a run is started and read.
export const actionsFailed = (s: State, project: Project) => { const last = lastFullRun(ensure(s, project)); return !!last && last.failed > 0 && !last.expected && !last.running; };
export function ActionsTab({ s, project, route }: { s: State; project: Project; route: Route }) {
  return <div class="wf" data-main><Runs key={project.id} s={s} p={project} w={ensure(s, project)} route={route} /></div>;
}
// The tabs of the Status view. The message board is the third one in the mock: Status is where the
// agents are, and the board is where they talk. Its address is its own.
export function StatusNav({ s, project, current }: { s: State; project: Project; current: string }) {
  const w = ensure(s, project); const stuck = w.stuck && byId(s.requests, w.stuck.requestId)?.status === 'open';
  const views = [
    { id: 'overview', label: 'Overview', to: `/p/${project.id}/status` },
    { id: 'review', label: 'Reviews', to: `/p/${project.id}/status/review`, mark: !!stuck },
    ...(live.on ? [] : [{ id: 'messages', label: 'Message board', to: `/p/${project.id}/message-board` }]),
    // A project in Define has the end result as its first view, so it is not a tab here.
    ...(live.on || project.stage === 'Define' ? [] : [{ id: 'result', label: 'End result', to: `/p/${project.id}/end-result` }]),
  ];
  const here = current === 'messages' ? 'message-board' : current === 'result' ? 'end-result' : 'status';
  return <nav class="wf-nav" aria-label="Status views">
    <Back path={`/p/${project.id}/${here}`} class="wf-back" />
    {views.map((v) => <a key={v.id} class="wf-tab" aria-current={current === v.id ? 'page' : undefined} href={href(v.to)}>
      {v.label}
      {'mark' in v && v.mark && <span class="warn-text">Stuck</span>}
    </a>)}
  </nav>;
}
export function WorkflowTab({ s, project, route }: { s: State; project: Project; route: Route }) {
  const w = ensure(s, project); const view = route.parts[3] || 'overview';
  // The end result and Actions are views of the project, and the settings are a page of the project's Settings.
  // None is a tab here. The finished page of a project that is handed over still opens at its old address.
  const V = ({ review: Review, result: Result } as Record<string, (props: TabProps) => ComponentChildren>)[view] || Overview;
  return <div class="wf" data-main>
    <StatusNav s={s} project={project} current={V === Overview ? 'overview' : view} />
    <V key={project.id + view} s={s} p={project} w={w} />
  </div>;
}
