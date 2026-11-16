import * as Effect from 'effect/Effect';
import { expect, it } from 'vitest';
import { append } from '../support/alertsRig.ts';
import { withCarrier } from '../support/carrierRig.ts';
import { call, ownerCredential, success } from '../support/handlerRig.ts';
import { decisionProject, decisionSetup } from '../support/projectDecisionFixtures.ts';
import { registerRebuildCheck } from '../support/rebuildCheck.ts';
import { snapshot } from '../support/routingRig.ts';
import { alertPayload } from '../../src/client/alerts/send.ts';
import { clientView } from '../../src/client/alerts/view.ts';

for (const scenario of ['reply to reply', 'another project channel'] as const) it(`s21 board alert opens ${scenario}`, async () => {
  await withCarrier(() => Effect.gen(function* () {
    yield* registerRebuildCheck();
    const one = yield* decisionSetup(), two = yield* decisionProject('two');
    const target = scenario === 'another project channel' ? two : one;
    const channelId = target.team.channelId;
    const post = (text: string, replyTo?: string) => call(target.leadCredential, 'board.post', { channelId, text, to: [{ _tag: 'owner' }], reply: 'any', ...(replyTo ? { replyTo } : {}) });
    const root = success(yield* post('Root subject.')).postId;
    const reply = success(yield* post('First reply.', root)).postId;
    const nested = success(yield* post('Reply to a reply.', reply)).postId;
    yield* append({ _tag: 'client.WebSettingsChanged', settings: { enabled: true, port: 18787, heartbeatSeconds: 25, proxyMode: true, publicAddress: 'https://alerts.example' } });
    yield* append({ _tag: 'client.AlertRaised', alertId: 'board-nested', alertKey: 'board:nested', kind: 'board.post', agentId: one.leadId, title: 'Board reply', sourceSequence: 1, subject: { _tag: 'piece', namespace: 'board', id: nested } });
    const state = yield* snapshot;
    const link = alertPayload(clientView(state.client).alerts['board-nested']!, state).link;
    expect(link).toBe(`https://alerts.example/#/p/${target.projectId}/messages/${channelId}?t=${reply}`);
  }));
});

it('s21 group-only channel alert uses the website resolved members', async () => {
  await withCarrier(() => Effect.gen(function* () {
    yield* registerRebuildCheck();
    const project = yield* decisionSetup();
    const channelId = success(yield* call(ownerCredential, 'board.open', { name: 'group-only', group: 'phone-group', anchorAgentId: project.workerId })).channelId;
    const postId = success(yield* call(project.workerCredential, 'board.post', { channelId, text: 'Group subject.', to: [{ _tag: 'owner' }], reply: 'any' })).postId;
    const channels = success(yield* call(ownerCredential, 'board.channels', {}));
    expect(channels.items.find((row: { channelId: string }) => row.channelId === channelId)?.members).toContainEqual({ _tag: 'agent', agentId: project.workerId });
    yield* append({ _tag: 'client.WebSettingsChanged', settings: { enabled: true, port: 18787, heartbeatSeconds: 25, proxyMode: true, publicAddress: 'https://alerts.example' } });
    yield* append({ _tag: 'client.AlertRaised', alertId: 'group-only', alertKey: 'group:only', kind: 'board.post', agentId: project.workerId, title: 'Board subject', sourceSequence: 1, subject: { _tag: 'piece', namespace: 'board', id: postId } });
    const state = yield* snapshot;
    expect(alertPayload(clientView(state.client).alerts['group-only']!, state).link).toBe(`https://alerts.example/#/p/${project.projectId}/messages/${channelId}?t=${postId}`);
  }));
});
