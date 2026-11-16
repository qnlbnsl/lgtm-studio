import { it } from '@effect/vitest';
import * as Effect from 'effect/Effect';
import * as Redacted from 'effect/Redacted';
import { afterEach, expect, vi } from 'vitest';
import { accept, crash, deliveryRig, notice, send, spawn } from '../support/deliveryRig.ts';
import { agentRow, appendSetup, call, eventOf, makeRig, ownerCredential, removeRig, success } from '../support/handlerRig.ts';
import { endTurn, eventually, snapshot } from '../support/routingRig.ts';
import { registerRebuildCheck } from '../support/rebuildCheck.ts';
import { RunnerRejected } from '../../src/runner/Runner.ts';
import { incarnationOf, sessionFor } from '../support/fakeRunner.ts';

afterEach(() => vi.unstubAllEnvs());
for (const ending of ['finished', 'errored', 'cancelled', 'stopped', 'crashed mid-turn', 'crashed idle', 'crashed after an errored turn'] as const) {
  it.live(`s21 turn boundary: ${ending}`, () => {
    const rig = makeRig();
    vi.stubEnv('HOME', rig.home); vi.stubEnv('LGTM_HOME', rig.stateFolder);
    return deliveryRig(rig, (runner) => Effect.gen(function* () {
      yield* registerRebuildCheck();
      const agent = yield* spawn(rig, runner);
      if (ending !== 'crashed idle') {
        const id = yield* send(runner, agent, 'Do this task once.');
        yield* accept(runner, agent, id);
        if (ending === 'finished') yield* endTurn(runner, agent);
        if (ending === 'cancelled') {
          success(yield* call(ownerCredential, 'cancel-reply', { agentId: agent.agentId }));
          yield* runner.emit({ _tag: 'turnEnded', agentId: agent.agentId, incarnation: incarnationOf(agent.token), stopped: true });
          yield* eventually(agentRow(agent.agentId), (a) => a?.state._tag === 'idle');
        }
        if (ending === 'errored' || ending === 'crashed after an errored turn') {
          yield* notice(runner, agent, { _tag: 'promptReply', deliveryId: id, outcome: { _tag: 'error', error: 'The task failed.' } });
          expect((yield* eventually(agentRow(agent.agentId), (a) => a?.state._tag === 'idle'))?.state._tag).toBe('idle');
          if (ending === 'crashed after an errored turn') yield* runner.emit({ _tag: 'turnEnded', agentId: agent.agentId, incarnation: incarnationOf(agent.token) });
          expect((yield* snapshot).threads[agent.agentId]?.some((row) => row.event._tag === 'TurnEnded')).toBe(false);
        }
      }
      if (ending === 'stopped') success(yield* call(ownerCredential, 'stop', { agentId: agent.agentId }));
      else yield* crash(runner, agent);
      const mark = runner.callsOf('prompt').length;
      success(yield* call(ownerCredential, 'resume', { agentId: agent.agentId }));
      const prompts = runner.callsOf('prompt').slice(mark);
      expect(prompts).toHaveLength(ending === 'crashed mid-turn' ? 1 : 0);
      if (prompts.length) {
        expect(prompts[0]!.input.delivery.content).toMatchObject({ _tag: 'carry-on', text: expect.stringContaining('agent crashed') });
        expect(JSON.stringify(prompts[0]!.input.delivery.content)).not.toContain('engine restarted');
      }
      expect((yield* agentRow(agent.agentId))?.state._tag).toBe(ending === 'crashed mid-turn' ? 'working' : 'idle');
    })).pipe(Effect.ensuring(Effect.sync(() => removeRig(rig))));
  });
}
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


it.live('s21 a session-open idle transition does not end unfinished work before a later fresh resume', () => {
  const rig = makeRig();
  vi.stubEnv('HOME', rig.home); vi.stubEnv('LGTM_HOME', rig.stateFolder);
  return deliveryRig(rig, (runner) => Effect.gen(function* () {
    yield* registerRebuildCheck();
    const agent = yield* spawn(rig, runner);
    const id = yield* send(runner, agent, 'Complete this unfinished task.');
    yield* accept(runner, agent, id);
    yield* crash(runner, agent);
    success(yield* call(ownerCredential, 'resume', { agentId: agent.agentId }));
    const token = Redacted.value(runner.callsOf('load').at(-1)!.input.token);
    yield* crash(runner, { ...agent, token });
    const mark = runner.callsOf('prompt').length;
    success(yield* call(ownerCredential, 'resume', { agentId: agent.agentId, fresh: true }));
    expect(runner.callsOf('prompt').slice(mark).map((c) => c.input.delivery.id)).toEqual([id]);
  })).pipe(Effect.ensuring(Effect.sync(() => removeRig(rig))));
});
