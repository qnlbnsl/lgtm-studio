import { it } from '@effect/vitest';
import * as Effect from 'effect/Effect';
import { expect, vi } from 'vitest';
import { deliveryRig, send, spawn } from '../support/deliveryRig.ts';
import { agentCredential, call, makeRig, ownerCredential, removeRig, success } from '../support/handlerRig.ts';
import { requestPermission, snapshot } from '../support/routingRig.ts';
import { registerRebuildCheck } from '../support/rebuildCheck.ts';

it.live('s21 owner answers a lead-held permission, withdraws its lead delivery and answers the worker once', () => {
  const rig = makeRig();
  vi.stubEnv('HOME', rig.home); vi.stubEnv('LGTM_HOME', rig.stateFolder);
  return deliveryRig(rig, (runner) => Effect.gen(function* () {
    yield* registerRebuildCheck();
    const lead = yield* spawn(rig, runner), worker = yield* spawn(rig, runner, lead);
    yield* send(runner, lead, 'Hold routed requests.');
    yield* send(runner, worker, 'Ask for permission.');
    const permission = yield* requestPermission(runner, worker);
    expect(permission.row.holder).toEqual({ _tag: 'agent', agentId: lead.agentId });
    const indexed = success(yield* call(ownerCredential, 'client.requests', { after: 0, limit: 100 })).items.find((r: { id: string }) => r.id === permission.id);
    const optionId = permission.row.options[0]!.id;
    success(yield* call(ownerCredential, 'answer', { kind: 'permission', id: permission.id, optionId }));
    const state = yield* snapshot;
    expect(state.requests[permission.id]).toMatchObject({ status: 'answered', answeredBy: { _tag: 'owner' }, answer: { _tag: 'selected', optionId } });
    expect(state.deliveries[permission.row.deliveryId]?.status).toBe('withdrawn');
    expect(runner.callsOf('answerPermission').map((c) => c.input)).toEqual([{ agentId: worker.agentId, requestId: permission.id, optionId }]);
    expect((yield* call(agentCredential(lead.token), 'answer', { kind: 'permission', id: permission.id, optionId }))._tag).toBe('Failure');
    expect(runner.callsOf('answerPermission')).toHaveLength(1);
    // Assert presentation only after proving the engine accepts the owner's
    // answer and retires the lead's outstanding request.
    expect(indexed.canAnswer).toBe(true);
  })).pipe(Effect.ensuring(Effect.sync(() => { vi.unstubAllEnvs(); removeRig(rig); })));
});
