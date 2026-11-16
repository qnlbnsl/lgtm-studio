import { it } from '@effect/vitest';
import * as Effect from 'effect/Effect';
import { afterEach, expect, vi } from 'vitest';
import { accept, crash, deliveryRig, send, spawn } from '../support/deliveryRig.ts';
import { agentRow, appendSetup, call, eventOf, makeRig, ownerCredential, removeRig, success } from '../support/handlerRig.ts';
import { eventually } from '../support/routingRig.ts';
import { registerRebuildCheck } from '../support/rebuildCheck.ts';
import { RunnerRejected } from '../../src/runner/Runner.ts';
import { sessionFor } from '../support/fakeRunner.ts';

afterEach(() => vi.unstubAllEnvs());
for (const unfinished of [true, false]) for (const queued of [true, false]) {
  it.live(`s21 resume: crashed agent with unfinished turn ${unfinished} and queue ${queued} receives only its remaining work`, () => {
    const rig = makeRig();
    vi.stubEnv('HOME', rig.home); vi.stubEnv('LGTM_HOME', rig.stateFolder);
    return deliveryRig(rig, (runner) => Effect.gen(function* () {
      yield* registerRebuildCheck();
      const agent = yield* spawn(rig, runner);
      if (unfinished) {
        const id = yield* send(runner, agent, 'Finish this task.');
        yield* accept(runner, agent, id);
      }
      yield* crash(runner, agent);
      // Crash withdraws pre-crash queued deliveries. Only deliveries still
      // queued at resume may be sent; never revive a returned delivery.
      if (queued) yield* appendSetup([eventOf({ _tag: 'DeliveryQueued', delivery: {
        id: 'remaining', target: agent.agentId, sender: { _tag: 'owner' }, content: { _tag: 'prompt', text: 'Next task.' },
      } })]);
      const mark = runner.callsOf('prompt').length;
      success(yield* call(ownerCredential, 'resume', { agentId: agent.agentId }));
      if (unfinished || queued) yield* eventually(Effect.sync(() => runner.callsOf('prompt').length), (n) => n > mark);
      const prompts = runner.callsOf('prompt').slice(mark);
      expect(prompts).toHaveLength(unfinished || queued ? 1 : 0);
      if (prompts.length) expect(prompts[0]!.input.delivery.content._tag).toBe(unfinished ? 'carry-on' : 'prompt');
      expect((yield* agentRow(agent.agentId))?.state._tag).toBe(unfinished || queued ? 'working' : 'idle');
    })).pipe(Effect.ensuring(Effect.sync(() => removeRig(rig))));
  });
}

it.live('s21 a failed crash reload retains the unfinished turn for the next resume', () => {
  const rig = makeRig();
  vi.stubEnv('HOME', rig.home); vi.stubEnv('LGTM_HOME', rig.stateFolder);
  return deliveryRig(rig, (runner) => Effect.gen(function* () {
    yield* registerRebuildCheck();
    const agent = yield* spawn(rig, runner);
    yield* accept(runner, agent, yield* send(runner, agent, 'Finish this task.'));
    yield* crash(runner, agent);
    const mark = runner.callsOf('prompt').length;
    runner.script('load', () => Effect.fail(new RunnerRejected({ reason: 'test reload refused' })));
    success(yield* call(ownerCredential, 'resume', { agentId: agent.agentId }));
    expect((yield* agentRow(agent.agentId))?.state).toEqual({ _tag: 'interrupted', previous: 'working' });
    expect(runner.callsOf('prompt')).toHaveLength(mark);
    runner.script('load', ({ agentId }) => Effect.succeed(sessionFor(agentId)));
    success(yield* call(ownerCredential, 'resume', { agentId: agent.agentId }));
    expect(runner.callsOf('prompt').slice(mark).map((c) => c.input.delivery.content._tag)).toEqual(['carry-on']);
  })).pipe(Effect.ensuring(Effect.sync(() => removeRig(rig))));
});

it.live('s21 a fresh resume cannot bypass the existing paused-state contract', () => {
  const rig = makeRig();
  vi.stubEnv('HOME', rig.home); vi.stubEnv('LGTM_HOME', rig.stateFolder);
  return deliveryRig(rig, (runner) => Effect.gen(function* () {
    yield* registerRebuildCheck();
    const agent = yield* spawn(rig, runner);
    yield* appendSetup([eventOf({ _tag: 'StateChanged', agentId: agent.agentId, state: { _tag: 'paused', reason: 'limit', midTurn: false } })]);
    expect((yield* call(ownerCredential, 'resume', { agentId: agent.agentId, fresh: true }))._tag).toBe('Failure');
    expect((yield* agentRow(agent.agentId))?.state._tag).toBe('paused');
    expect(runner.callsOf('load')).toHaveLength(0);
    expect(runner.callsOf('start')).toHaveLength(1);
  })).pipe(Effect.ensuring(Effect.sync(() => removeRig(rig))));
});
