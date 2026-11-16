import { expect, it } from 'vitest';
import * as Effect from 'effect/Effect';
import { lifecycleStateNames } from '@lgtm/contracts';
import { Views } from '../../src/views/Views.ts';
import { appendSetup, call, eventOf, failureOf, headSequence, logAfter, ownerCredential, seedAgent, success } from '../support/handlerRig.ts';
import { registerRebuildCheck } from '../support/rebuildCheck.ts';
import { decisionProject } from '../support/projectDecisionFixtures.ts';
import { projectKey, projectView } from '../../src/project/view.ts';
import { run, read, questionFor, snapshotFor } from './support/readRig.ts';

// Each lifecycle row checks the wire lists, counts, durable question, and answer command.
for (const state of [...lifecycleStateNames, 'signed-out'] as const) it(`question availability for ${state}`, async () => run(Effect.gen(function* () {
  yield* seedAgent({ id: 'agent-a', state: state === 'signed-out' ? 'paused' : state });
  if (state === 'signed-out') yield* appendSetup([eventOf({ _tag: 'StateChanged', agentId: 'agent-a', state: { _tag: 'paused', reason: 'signed-out', midTurn: false } })]);
  yield* appendSetup([eventOf({ _tag: 'project.Created', projectId: 'p1', repoId: 'r1', worktreeId: 'w1', name: 'Availability', goal: 'Offer actionable questions.', machineId: 'local', leadAgentId: 'agent-a' })]);
  yield* appendSetup([questionFor('q1'), eventOf({ _tag: 'client.AlertRaised', alertId: 'q-alert', alertKey: 'question:q1', kind: 'question', title: 'A question needs your answer', sourceSequence: 1, agentId: 'agent-a', subject: { _tag: 'question', id: 'q1' } })]);
  const available = state !== 'stopped' && state !== 'crashed';
  const snapshot = yield* snapshotFor();
  expect(snapshot.questions.map((r: any) => r.id)).toEqual(available ? ['q1'] : []);
  expect(snapshot.questionsWithoutDisplay).toBe(0);
  expect(snapshot.project.agents[0].status).toBe(available ? 'Needs you' : 'Ready');
  expect(snapshot.agents[0].openQuestionsAsked).toBe(available ? 1 : 0);
  expect((yield* read('client.requests', { after: 0, limit: 200 })).items.map((r: any) => r.id)).toEqual(available ? ['q1'] : []);
  expect((yield* read('client.alerts.list', { after: 0, limit: 200 })).items.map((r: any) => r.alertId)).toEqual(available ? ['q-alert'] : []);
  expect((yield* (yield* Views).snapshot).questions.q1?.status).toBe('open');
  const before = yield* headSequence;
  const result = yield* call(ownerCredential, 'answer', { kind: 'question', id: 'q1', text: 'Use trunk.' });
  if (available) success(result);
  else {
    expect(failureOf(result)).toMatchObject({ _tag: 'CommandRefused', reason: 'The asker cannot receive an answer.' });
    const written = yield* logAfter(before);
    expect(written.some(r => r.event._tag === 'DeliveryQueued' || r.event._tag === 'QuestionAnswered')).toBe(false);
    expect((yield* (yield* Views).snapshot).questions.q1?.status).toBe('open');
  }
}), registerRebuildCheck()));

for (const absence of ['missing agent', 'deleted project'] as const) it(`omits questions for a ${absence}`, async () => run(Effect.gen(function* () {
  if (absence !== 'missing agent') yield* seedAgent({ id: 'agent-a', state: absence === 'deleted project' ? 'working' : 'stopped', groups: ['team-a'] });
  yield* appendSetup([questionFor('q1')]);
  if (absence === 'deleted project') expect((yield* snapshotFor()).questions.map((r: any) => r.id)).toEqual(['q1']);
  if (absence === 'deleted project') yield* appendSetup([eventOf({ _tag: 'project.Deleted', projectId: 'project-a', agentIds: ['agent-a'], channelIds: [], postIds: [], worktreeIds: [], alertIds: [], deletedAt: 1 })]);
  expect((yield* snapshotFor()).questions).toEqual([]);
  expect((yield* read('client.requests', { after: 0, limit: 200 })).items).toEqual([]);
  expect(failureOf(yield* call(ownerCredential, 'answer', { kind: 'question', id: 'q1', text: 'Too late.' }))?._tag).toBe('CommandRefused');
}), registerRebuildCheck()));

it('preserves the lifecycle rule through a real team deletion and later resume', async () => run(Effect.gen(function* () {
  const f = yield* decisionProject();
  const questionId = success(yield* call(f.workerCredential, 'ask', { text: 'Keep this question open.' })).questionId;
  const deletion = { projectId: f.projectId, teamId: f.team.teamId };
  expect((yield* snapshotFor()).questions.map((q: any) => q.id)).toEqual([questionId]);
  expect(failureOf(yield* call(ownerCredential, 'project.team.delete', deletion))?.reason).toBe('Stop the team’s agents before deleting the team.');
  expect(projectView(yield* (yield* Views).snapshot).teams[projectKey(f.projectId, f.team.teamId)]).toBeDefined();
  success(yield* call(ownerCredential, 'stop', { agentId: f.workerId }));
  expect((yield* snapshotFor()).questions).toEqual([]);
  expect((yield* read('client.requests', { after: 0, limit: 200 })).items).toEqual([]);
  const before = yield* headSequence;
  success(yield* call(ownerCredential, 'project.team.delete', deletion));
  const state = yield* (yield* Views).snapshot;
  expect((yield* logAfter(before)).map(row => row.event._tag)).toContain('project.TeamDeleted');
  expect(projectView(state).teams[projectKey(f.projectId, f.team.teamId)]).toBeUndefined();
  expect(state.agents[f.workerId]!.groups).not.toContain(f.team.group);
  expect(state.questions[questionId]?.status).toBe('open');
  expect((yield* snapshotFor()).questions).toEqual([]);
  expect((yield* read('client.requests', { after: 0, limit: 200 })).items).toEqual([]);
  expect(failureOf(yield* call(ownerCredential, 'answer', { kind: 'question', id: questionId, text: 'Too late.' }))?._tag).toBe('CommandRefused');
  success(yield* call(ownerCredential, 'resume', { agentId: f.workerId }));
  expect((yield* snapshotFor()).questions.map((q: any) => q.id)).toEqual([questionId]);
}), registerRebuildCheck()));

it('offers the same still-open question again when its asker resumes', async () => run(Effect.gen(function* () {
  yield* seedAgent({ id: 'agent-a', state: 'working' });
  yield* appendSetup([questionFor('q1')]);
  expect((yield* snapshotFor()).questions.map((r: any) => r.id)).toEqual(['q1']);
  yield* appendSetup([eventOf({ _tag: 'AgentStopped', agentId: 'agent-a' })]);
  expect((yield* snapshotFor()).questions).toEqual([]);
  yield* appendSetup([eventOf({ _tag: 'StateChanged', agentId: 'agent-a', state: { _tag: 'starting' } })]);
  expect((yield* snapshotFor()).questions.map((r: any) => r.id)).toEqual(['q1']);
  expect((yield* read('client.requests', { after: 0, limit: 200 })).items).toEqual([expect.objectContaining({ id: 'q1', canAnswer: true })]);
}), registerRebuildCheck()));

for (const history of ['cleared', 'answered', 'cleared and answered'] as const) it(`retains a ${history} question alert after its asker stops`, async () => run(Effect.gen(function* () {
  yield* seedAgent({ id: 'agent-a', state: 'working' });
  yield* appendSetup([questionFor('q1'), eventOf({ _tag: 'client.AlertRaised', alertId: 'q-alert', alertKey: 'question:q1', kind: 'question', title: 'A question needs your answer', sourceSequence: 1, agentId: 'agent-a', subject: { _tag: 'question', id: 'q1' } })]);
  if (history !== 'cleared') success(yield* call(ownerCredential, 'answer', { kind: 'question', id: 'q1', text: 'Use trunk.' }));
  if (history !== 'answered') yield* appendSetup([eventOf({ _tag: 'client.AlertCleared', alertId: 'q-alert', cause: 'dismissed' })]);
  yield* appendSetup([eventOf({ _tag: 'AgentStopped', agentId: 'agent-a' })]);
  expect((yield* read('client.alerts.list', { after: 0, limit: 200 })).items).toEqual([
    expect.objectContaining({ alertId: 'q-alert', status: history === 'answered' ? 'open' : 'cleared' }),
  ]);
}), registerRebuildCheck()));
