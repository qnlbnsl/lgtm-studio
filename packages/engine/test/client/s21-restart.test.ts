import { it } from '@effect/vitest';
import * as Effect from 'effect/Effect';
import { afterEach, expect, vi } from 'vitest';
import { installRestarter } from '../../src/client/serviceControl.ts';
import { makeFakeRunner } from '../support/fakeRunner.ts';
import { agentRow, call, makeRig, ownerCredential, removeRig, seedAccount, seedAgent, seedHarness, success } from '../support/handlerRig.ts';
import { provideRestart, recover } from '../support/restartRig.ts';
import { registerRebuildCheck } from '../support/rebuildCheck.ts';

afterEach(() => { installRestarter(undefined); vi.unstubAllEnvs(); });
const rows = [
  ['starting', 'idle', 'start'], ['working', 'working', 'load'], ['idle', 'idle', 'load'],
  ['paused', 'paused', undefined], ['interrupted', 'idle', 'load'],
  ['crashed', 'crashed', undefined], ['stopped', 'stopped', undefined],
] as const;
for (const via of ['website', 'SIGTERM'] as const) for (const [before, after, method] of rows) {
  it.live(`s21 ${via} restart: ${before} becomes ${after}`, () => {
    const rig = makeRig();
    vi.stubEnv('HOME', rig.home); vi.stubEnv('LGTM_HOME', rig.stateFolder);
    return Effect.gen(function* () {
      const first = yield* makeFakeRunner();
      yield* provideRestart(rig, first.layer, Effect.gen(function* () {
        yield* registerRebuildCheck();
        yield* seedAccount(); yield* seedHarness();
        // A deliberate stop is a real command, rather than an invented
        // interpretation of the lifecycle state.
        yield* seedAgent({ id: 'restart-agent', state: before === 'stopped' ? 'idle' : before });
        if (before === 'stopped') success(yield* call(ownerCredential, 'stop', { agentId: 'restart-agent' }));
        if (via === 'website') {
          let requested = false;
          installRestarter(() => { requested = true; }, { execPath: process.execPath, scriptPath: import.meta.filename });
          success(yield* call(ownerCredential, 'client.service.restart', { confirmed: true }));
          expect(requested).toBe(true);
        }
        expect((yield* agentRow('restart-agent'))?.state._tag).toBe(before);
        // Scope teardown closes the engine and its runner, as the service's
        // SIGTERM handler does. The next engine reads only the durable log.
      }), { recovering: false });
      installRestarter(undefined);
      const second = yield* makeFakeRunner();
      yield* provideRestart(rig, second.layer, Effect.gen(function* () {
        yield* registerRebuildCheck();
        yield* recover;
        expect((yield* agentRow('restart-agent'))?.state._tag).toBe(after);
        expect(second.callsOf('start').length).toBe(method === 'start' ? 1 : 0);
        expect(second.callsOf('load').length).toBe(method === 'load' ? 1 : 0);
        expect(second.callsOf('prompt').length).toBe(before === 'working' ? 1 : 0);
      }));
    }).pipe(Effect.ensuring(Effect.sync(() => removeRig(rig)))) as Effect.Effect<void, unknown, never>;
  });
}
