import * as Effect from 'effect/Effect';
import * as Redacted from 'effect/Redacted';
import { expect, it, vi } from 'vitest';
import { makeCredentialRegistry, type CredentialSource } from '../../src/accounts/credentialSource.ts';
import { registerRebuildCheck } from '../support/rebuildCheck.ts';
registerRebuildCheck({noLog:true});
it('s19 a stalled filesystem respects the total budget and two-read bound across repeated batches',async()=>{
  vi.useFakeTimers();
  try {
    const registry=makeCredentialRegistry();
    let started=0;
    const sources:CredentialSource[]=Array.from({length:10},(_,i)=>({paths:[`/tmp/s19-never-opened-${i}`],read:Effect.succeed([]),warm:()=>{started++;return new Promise(()=>{});}}));
    const first=Effect.runPromise(registry.preload(sources));
    await vi.advanceTimersByTimeAsync(500);await first;
    expect(started).toBe(2);
    await Effect.runPromise(registry.preload(sources));
    expect(started).toBe(2);
  } finally {vi.useRealTimers();}
});
it('s19 duplicate stores warm once and keep leaves after account removal',async()=>{
  const registry=makeCredentialRegistry();let reads=0;
  const leaves=[Redacted.make('s19-fake-retained-credential')];
  const source:CredentialSource={paths:['/tmp/s19-never-opened'],read:Effect.succeed(leaves),warm:async()=>{reads++;return leaves;}};
  await Effect.runPromise(registry.preload([source,source]));
  await Effect.runPromise(registry.preload([]));
  expect(reads).toBe(1);
  expect(registry.secrets().length).toBe(1);
});
