import { mkdirSync, writeFileSync, chmodSync } from 'node:fs';
import { join } from 'node:path';
import { it } from '@effect/vitest';
import * as Effect from 'effect/Effect';
import { afterEach, expect } from 'vitest';
import { makeServiceRoot, startService, waitServing, ownerClient, requestId, stopAllServices, fakeHarness, writeFakeAdapter } from '../support/serviceProcess.ts';
import { rowsIn } from '../support/harnessLoginRig.ts';
import { registerRebuildCheck } from '../support/rebuildCheck.ts';
registerRebuildCheck({noLog:true});
afterEach(stopAllServices);
const token = 's19-fake-private-unlaunched-account-credential';
for (const operation of ['add','change','enable','restart'] as const) {
  it.live(`s19 ${operation} registers an unlaunched account before publication`, () => Effect.scoped(Effect.gen(function*(){
    const root=makeServiceRoot('/tmp/s19-pre-');
    const home=join(root.home,'private'); mkdirSync(home);
    writeFileSync(join(home,'auth.json'),JSON.stringify({tokens:{access_token:token}}));
    let service=startService(root);
    yield* waitServing(root,service);
    let client=yield* ownerClient(root);
    const account={id:'private',harness:'codex',enabled:true,home,environment:{CODEX_HOME:home}};
    if(operation !== 'add') yield* client.accounts({requestId:requestId(),operation:'add',account:{...account, enabled:operation !== 'enable', ...(operation === 'change'? {home:join(root.home,'other'),environment:{CODEX_HOME:join(root.home,'other')}}:{})}} as never);
    if(operation === 'restart') {
      service.child.kill('SIGTERM'); yield* Effect.promise(()=>service.exit);
      service=startService(root); yield* waitServing(root,service); client=yield* ownerClient(root);
    }
    if(operation === 'restart') {
      // A harness change does not invoke the account mutation hook. Only the
      // startup registry can protect this challenge from an unlaunched home.
      const harness=fakeHarness(writeFakeAdapter(root));
      yield* client.harnesses({requestId:requestId(),operation:'set',harness:{...harness,adapterArguments:['safe-startup '+token]}} as never);
      expect(rowsIn(root).some(row=>row.payload_json.includes(token))).toBe(false);
    }
    // The account-change event itself must already pass the new registry.
    yield* client.accounts({requestId:requestId(),operation:operation==='add'?'add':'set',account:{...account,displayName:'safe-before '+token+' safe-after'}} as never);
    const rows=rowsIn(root);
    expect(rows.some(row=>row.payload_json.includes('safe-before'))).toBe(true);
    expect(rows.some(row=>row.payload_json.includes(token))).toBe(false);
  })),60_000);
}
it.live('s19 missing and unreadable unlaunched stores do not prevent a restart',()=>Effect.scoped(Effect.gen(function*(){
  const root=makeServiceRoot('/tmp/s19-skip-');
  const home=join(root.home,'private');mkdirSync(home);
  const path=join(home,'auth.json');writeFileSync(path,'{');chmodSync(path,0);
  let service=startService(root);yield* waitServing(root,service);
  const client=yield* ownerClient(root);
  yield* client.accounts({requestId:requestId(),operation:'add',account:{id:'private',harness:'codex',enabled:true,home,environment:{CODEX_HOME:home}}} as never);
  service.child.kill('SIGTERM');yield* Effect.promise(()=>service.exit);
  service=startService(root);yield* waitServing(root,service);
})),60_000);
