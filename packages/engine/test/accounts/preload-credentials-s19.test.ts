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
    // Retrying the same abandoned paths does not start duplicate reads.
    await Effect.runPromise(registry.preload(sources.slice(0,2)));
    expect(started).toBe(2);
    // Other paths can use the abandoned logical slots in a later batch.
    const next=Effect.runPromise(registry.preload(sources));
    await vi.advanceTimersByTimeAsync(500);await next;
    expect(started).toBe(4);
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

it('s19 r2 an abandoned read does not prevent the next healthy preload batch',async()=>{
  vi.useFakeTimers();
  try {
    const registry=makeCredentialRegistry();
    const hung:CredentialSource={paths:['/tmp/s19-r2-never-opened'],read:Effect.succeed([]),warm:()=>new Promise(()=>{})};
    const first=Effect.runPromise(registry.preload([hung,{...hung,paths:['/tmp/s19-r2-second-never-opened']} ]));
    await vi.advanceTimersByTimeAsync(500);await first;
    let reads=0;
    const healthy:CredentialSource={paths:['/tmp/s19-r2-healthy-not-opened'],read:Effect.succeed([]),warm:async()=>{reads++;return [];}};
    await Effect.runPromise(registry.preload([healthy]));
    expect(reads).toBe(1);
  } finally {vi.useRealTimers();}
});
