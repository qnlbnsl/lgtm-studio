import { it } from '@effect/vitest';
import * as Effect from 'effect/Effect';
import * as Redacted from 'effect/Redacted';
import type * as SqlClient from 'effect/sql/SqlClient';
import type { Views } from '../../src/views/Views.ts';
import { afterEach, expect, vi } from 'vitest';
import { accept, crash, deliveryRig, notice, promptsTo, send, spawn } from '../support/deliveryRig.ts';
import { agentRow, appendSetup, call, eventOf, makeRig, ownerCredential, removeRig, success } from '../support/handlerRig.ts';
import { deliveryRow, endTurn, eventually, snapshot } from '../support/routingRig.ts';
import { registerRebuildCheck } from '../support/rebuildCheck.ts';
import { RunnerRejected } from '../../src/runner/Runner.ts';
import { incarnationOf, sessionFor } from '../support/fakeRunner.ts';

afterEach(() => vi.unstubAllEnvs());
for (const ending of ['finished', 'errored', 'cancelled', 'stopped', 'crashed mid-turn', 'crashed idle', 'crashed after an errored turn', 'crashed paused mid-turn', 'crashed capacity-held', 'crashed capacity-held before acceptance', 'crashed during resume starting'] as const) {
  it.live(`s21 turn boundary: ${ending}`, () => {
    const rig = makeRig();
    vi.stubEnv('HOME', rig.home); vi.stubEnv('LGTM_HOME', rig.stateFolder);
    return deliveryRig(rig, (runner) => Effect.gen(function* () {
      yield* registerRebuildCheck();
      const agent = yield* spawn(rig, runner);
      if (ending !== 'crashed idle') {
        const id = yield* send(runner, agent, 'Do this task once.');
        if (ending !== 'crashed capacity-held before acceptance') yield* accept(runner, agent, id);
        if (ending === 'crashed paused mid-turn') {
          yield* notice(runner, agent, { _tag: 'promptReply', deliveryId: id, outcome: { _tag: 'pause', reason: 'out-of-usage', error: 'Usage limit.' } });
          expect((yield* eventually(agentRow(agent.agentId), (a) => a?.state._tag === 'paused'))?.state).toMatchObject({ _tag: 'paused', midTurn: true });
        }
        if (ending === 'crashed capacity-held' || ending === 'crashed capacity-held before acceptance') {
          yield* notice(runner, agent, { _tag: 'promptReply', deliveryId: id, outcome: { _tag: 'capacity', error: 'At capacity.' } });
          expect((yield* eventually(agentRow(agent.agentId), (a) => a?.capacityHeld === true))?.state._tag).toBe('idle');
          expect(Object.values((yield* snapshot).deliveries).filter((d) => d.content._tag === 'carry-on' && d.status === 'queued')).toHaveLength(ending === 'crashed capacity-held' ? 1 : 0);
        }
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
      if (ending === 'crashed during resume starting') {
        const services = yield* Effect.context<Views | SqlClient.SqlClient>();
        runner.script('load', (input) => Effect.gen(function* () {
          expect((yield* agentRow(agent.agentId))?.state._tag).toBe('starting');
          yield* crash(runner, { ...agent, token: Redacted.value(input.token) });
          return sessionFor(input.agentId);
        }).pipe(Effect.provide(services)));
        success(yield* call(ownerCredential, 'resume', { agentId: agent.agentId }));
        expect((yield* agentRow(agent.agentId))?.state._tag).toBe('crashed');
        runner.script('load', ({ agentId }) => Effect.succeed(sessionFor(agentId)));
      }
      if (ending === 'crashed paused mid-turn') {
        const accountId = (yield* agentRow(agent.agentId))!.account;
        yield* appendSetup([eventOf({ _tag: 'AccountAvailable', accountId, cause: 'owner cleared' })]);
      }
      const mark = runner.callsOf('prompt').length;
      success(yield* call(ownerCredential, 'resume', { agentId: agent.agentId }));
      const prompts = runner.callsOf('prompt').slice(mark);
      const unfinished = ['crashed mid-turn', 'crashed paused mid-turn', 'crashed capacity-held', 'crashed during resume starting'].includes(ending);
      expect(prompts).toHaveLength(unfinished ? 1 : 0);
      if (prompts.length) {
        expect(prompts[0]!.input.delivery.content).toMatchObject({ _tag: 'carry-on', text: expect.stringContaining('agent crashed') });
        expect(JSON.stringify(prompts[0]!.input.delivery.content)).not.toContain('engine restarted');
      }
      expect((yield* agentRow(agent.agentId))?.state._tag).toBe(unfinished ? 'working' : 'idle');
    }), undefined, { capacityHoldCondition: () => true }).pipe(Effect.ensuring(Effect.sync(() => removeRig(rig))));
  });
}

for (const resolved of ['finished recovered turn', 'owner stopped crash'] as const) {
  it.live(`s21 turn boundary: ${resolved} then setup crash: no carry-on`, () => {
    const rig = makeRig();
    vi.stubEnv('HOME', rig.home); vi.stubEnv('LGTM_HOME', rig.stateFolder);
    return deliveryRig(rig, (runner) => Effect.gen(function* () {
      yield* registerRebuildCheck();
      const agent = yield* spawn(rig, runner);
      yield* accept(runner, agent, yield* send(runner, agent, 'Finish D1.'));
      yield* crash(runner, agent);
      if (resolved === 'finished recovered turn') {
        success(yield* call(ownerCredential, 'resume', { agentId: agent.agentId }));
        const resumed = { ...agent, token: Redacted.value(runner.callsOf('load').at(-1)!.input.token) };
        const carry = promptsTo(runner, agent).at(-1)!.input.delivery;
        expect(carry.content._tag).toBe('carry-on');
        yield* accept(runner, resumed, carry.id);
        yield* endTurn(runner, resumed);
        expect((yield* agentRow(agent.agentId))?.state._tag).toBe('idle');
      }
      success(yield* call(ownerCredential, 'stop', { agentId: agent.agentId }));
      expect((yield* agentRow(agent.agentId))?.state._tag).toBe('stopped');
      const mark = promptsTo(runner, agent).length;
      const services = yield* Effect.context<Views | SqlClient.SqlClient>();
      runner.script('load', (input) => Effect.gen(function* () {
        expect((yield* agentRow(agent.agentId))?.state._tag).toBe('starting');
        yield* crash(runner, { ...agent, token: Redacted.value(input.token) });
        return sessionFor(input.agentId);
      }).pipe(Effect.provide(services)));
      success(yield* call(ownerCredential, 'resume', { agentId: agent.agentId }));
      expect((yield* agentRow(agent.agentId))?.state._tag).toBe('crashed');
      expect(promptsTo(runner, agent)).toHaveLength(mark);
      runner.script('load', ({ agentId }) => Effect.succeed(sessionFor(agentId)));
      success(yield* call(ownerCredential, 'resume', { agentId: agent.agentId }));
      expect(promptsTo(runner, agent).slice(mark)).toEqual([]);
      expect((yield* agentRow(agent.agentId))?.state._tag).toBe('idle');
    })).pipe(Effect.ensuring(Effect.sync(() => removeRig(rig))));
  });
}

it.live('s21 turn boundary: crashed while working before acceptance: no carry-on', () => {
  const rig = makeRig();
  vi.stubEnv('HOME', rig.home); vi.stubEnv('LGTM_HOME', rig.stateFolder);
  return deliveryRig(rig, (runner) => Effect.gen(function* () {
    yield* registerRebuildCheck();
    const lead = yield* spawn(rig, runner);
    const agent = yield* spawn(rig, runner, lead);
    // An older accepted delivery must not stand in for this turn's acceptance.
    const finished = yield* send(runner, agent, 'Earlier completed work.', lead);
    yield* accept(runner, agent, finished);
    yield* endTurn(runner, agent);
    const id = yield* send(runner, agent, 'D1 from the lead.', lead);
    expect((yield* deliveryRow(id))?.status).toBe('sent');
    yield* crash(runner, agent);
    const state = yield* snapshot;
    expect(state.deliveries[finished]?.status).toBe('accepted');
    expect(state.deliveries[id]?.status).toBe('returned');
    expect(state.threads[agent.agentId]?.some(({ event }) => event._tag === 'DeliveryWithdrawn' && event.deliveryId === id)).toBe(true);
    expect(Object.values(state.deliveries).some((d) => d.target === lead.agentId && d.content._tag === 'prompt' && d.content.text.includes(id) && d.content.text.includes('not delivered'))).toBe(true);
    const mark = promptsTo(runner, agent).length;
    success(yield* call(ownerCredential, 'resume', { agentId: agent.agentId }));
    expect(promptsTo(runner, agent).slice(mark)).toEqual([]);
    expect((yield* agentRow(agent.agentId))?.state._tag).toBe('idle');
    const retried = yield* send(runner, agent, 'D1 from the lead.', lead);
    expect(promptsTo(runner, agent).slice(mark).map((c) => c.input.delivery.id)).toEqual([retried]);
    expect(Object.values((yield* snapshot).deliveries).filter((d) => d.target === agent.agentId && d.content._tag === 'carry-on')).toEqual([]);
  })).pipe(Effect.ensuring(Effect.sync(() => removeRig(rig))));
});

for (const ending of ['crashed paused outside a turn', 'crashed after a released capacity turn finished'] as const) {
  it.live(`s21 turn boundary: ${ending}: no carry-on`, () => {
    const rig = makeRig();
    vi.stubEnv('HOME', rig.home); vi.stubEnv('LGTM_HOME', rig.stateFolder);
    return deliveryRig(rig, (runner) => Effect.gen(function* () {
      yield* registerRebuildCheck();
      const agent = yield* spawn(rig, runner);
      const id = yield* send(runner, agent, 'Work before the pause or hold.');
      if (ending === 'crashed paused outside a turn') {
        yield* notice(runner, agent, { _tag: 'promptReply', deliveryId: id, outcome: { _tag: 'pause', reason: 'out-of-usage', error: 'Usage limit.' } });
        expect((yield* eventually(agentRow(agent.agentId), (a) => a?.state._tag === 'paused'))?.state).toMatchObject({ _tag: 'paused', midTurn: false });
      } else {
        yield* accept(runner, agent, id);
        yield* notice(runner, agent, { _tag: 'promptReply', deliveryId: id, outcome: { _tag: 'capacity', error: 'At capacity.' } });
        yield* eventually(agentRow(agent.agentId), (a) => a?.capacityHeld === true);
        success(yield* call(ownerCredential, 'release', { agentId: agent.agentId }));
        yield* eventually(Effect.sync(() => promptsTo(runner, agent).length), (n) => n === 2);
        const carry = promptsTo(runner, agent).at(-1)!.input.delivery;
        expect(carry.content._tag).toBe('carry-on');
        yield* accept(runner, agent, carry.id);
        yield* endTurn(runner, agent);
        expect((yield* agentRow(agent.agentId))?.capacityHeld).toBe(false);
        expect((yield* agentRow(agent.agentId))?.state._tag).toBe('idle');
      }
      yield* crash(runner, agent);
      if (ending === 'crashed paused outside a turn') yield* appendSetup([eventOf({
        _tag: 'AccountAvailable', accountId: (yield* agentRow(agent.agentId))!.account, cause: 'owner cleared',
      })]);
      const mark = promptsTo(runner, agent).length;
      success(yield* call(ownerCredential, 'resume', { agentId: agent.agentId }));
      expect(promptsTo(runner, agent).slice(mark)).toEqual([]);
      expect((yield* agentRow(agent.agentId))?.state._tag).toBe('idle');
    }), undefined, { capacityHoldCondition: () => true }).pipe(Effect.ensuring(Effect.sync(() => removeRig(rig))));
  });
}

for (const ending of ['stopped mid-turn', 'capacity-held then crashed', 'errored then crashed'] as const) {
  it.live(`s21 fresh replay boundary: ${ending}`, () => {
    const rig = makeRig();
    vi.stubEnv('HOME', rig.home); vi.stubEnv('LGTM_HOME', rig.stateFolder);
    return deliveryRig(rig, (runner) => Effect.gen(function* () {
      yield* registerRebuildCheck();
      const agent = yield* spawn(rig, runner);
      yield* appendSetup([eventOf({ _tag: 'SessionOpened', agentId: agent.agentId,
        session: { _tag: 'acp', sessionId: 'cannot-reload' }, protocolVersion: 2, reloadable: false })]);
      const id = yield* send(runner, agent, 'Finish the accepted delivery.');
      yield* accept(runner, agent, id);
      if (ending === 'stopped mid-turn') success(yield* call(ownerCredential, 'stop', { agentId: agent.agentId }));
      else {
        yield* notice(runner, agent, { _tag: 'promptReply', deliveryId: id, outcome: ending === 'capacity-held then crashed'
          ? { _tag: 'capacity', error: 'At capacity.' } : { _tag: 'error', error: 'The turn failed.' } });
        yield* eventually(agentRow(agent.agentId), (a) => a?.state._tag === 'idle');
        yield* crash(runner, agent);
      }
      expect((yield* deliveryRow(id))?.status).toBe(ending === 'errored then crashed' ? 'returned' : 'accepted');
      const mark = runner.callsOf('prompt').length;
      success(yield* call(ownerCredential, 'resume', { agentId: agent.agentId, fresh: true }));
      const prompts = runner.callsOf('prompt').slice(mark);
      expect(prompts.map((c) => c.input.delivery.id)).toEqual(ending === 'errored then crashed' ? [] : [id]);
      if (prompts.length) expect(prompts[0]!.input.delivery.content).toMatchObject({ _tag: 'prompt', text: 'Finish the accepted delivery.' });
      expect((yield* agentRow(agent.agentId))?.state._tag).toBe(ending === 'errored then crashed' ? 'idle' : 'working');
    }), undefined, { capacityHoldCondition: () => true }).pipe(Effect.ensuring(Effect.sync(() => removeRig(rig))));
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

for (const setupCrash of [false, true]) it.live(setupCrash
  ? 's21 a failed crash reload followed by a setup crash retains the unfinished turn'
  : 's21 a failed crash reload retains the unfinished turn for the next resume', () => {
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
    if (setupCrash) {
      const services = yield* Effect.context<Views | SqlClient.SqlClient>();
      runner.script('load', (input) => Effect.gen(function* () {
        expect((yield* agentRow(agent.agentId))?.state._tag).toBe('starting');
        yield* crash(runner, { ...agent, token: Redacted.value(input.token) });
        return sessionFor(input.agentId);
      }).pipe(Effect.provide(services)));
      success(yield* call(ownerCredential, 'resume', { agentId: agent.agentId }));
      expect((yield* agentRow(agent.agentId))?.state._tag).toBe('crashed');
      expect(runner.callsOf('prompt')).toHaveLength(mark);
    }
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
