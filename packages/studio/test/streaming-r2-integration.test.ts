// @vitest-environment node
import { join } from 'node:path';
import { writeFileSync } from 'node:fs';
import { it } from '@effect/vitest';
import * as Effect from 'effect/Effect';
import { afterEach,expect } from 'vitest';
import { mapThread } from '../src/chat.ts';
import { makeServiceRoot,startService,waitServing,ownerClient,requestId,stopAllServices,startFakeAgent,waitFor,turnEnded } from '../../engine/test/support/serviceProcess.ts';
import { rowsIn } from '../../engine/test/support/harnessLoginRig.ts';
import { decodeRow } from '../../engine/src/log/subscribe.ts';
import { registerRebuildCheck } from '../../engine/test/support/rebuildCheck.ts';
registerRebuildCheck({noLog:true});
afterEach(stopAllServices);
for(const repeat of [1,50]) it.live(`r2 decided text (${repeat} phrases) precedes permission and keeps its order on replay and reload`,()=>Effect.scoped(Effect.gen(function*(){
  const root=makeServiceRoot('/tmp/s19-r2-order-');
  let service=startService(root);yield* waitServing(root,service);
  let owner=yield* ownerClient(root);
  const secret='Zebra-planted-fake-secret-for-r2';
  const ordinary='Decided ordinary text. '.repeat(repeat);
  const gate=join(root.root,'continue');
  const agent=yield* startFakeAgent(root,owner,owner,{secrets:{R2_TOKEN:secret},prompt:'go',script:{turns:[[
    {step:'text',text:ordinary+'Zebra-'},
    {step:'tool_call',toolCallId:'queued',title:'Run checks',kind:'execute',status:'pending',output:'safe output'},
    {step:'notification_barrier'},
    {step:'raw_frame',frame:JSON.stringify({jsonrpc:'2.0',id:'r2-permission',method:'session/request_permission',params:{sessionId:'${session}',options:[{optionId:'allow',name:'Allow',kind:'allow_once'}],toolCall:{toolCallId:'queued',title:'Run checks'}}})},
    {step:'wait_for_file',path:gate},
    {step:'text',text:'planted-fake-secret-for-r2.'},
  ]]}});
  const decode=()=>Effect.forEach(rowsIn(root),decodeRow);
  yield* waitFor('the genuine-prefix permission',()=>rowsIn(root).some(row=>JSON.parse(row.payload_json)._tag==='PermissionRequested'));
  const before=yield* decode();
  const permission=before.find(row=>row.event._tag==='PermissionRequested')!;
  const prior=before.filter(row=>row.event._tag==='ThreadItem');
  expect(prior.map(row=>row.event._tag==='ThreadItem'?{kind:row.event.content._tag,ordinary:row.event.content._tag==='text'&&row.event.content.text===ordinary,length:row.event.content._tag==='text'?row.event.content.text.length:0}:{})).toEqual([{kind:'text',ordinary:true,length:ordinary.length}]);
  expect(prior[0]?.event).toMatchObject({content:{_tag:'text',text:ordinary}});
  expect(prior[0]!.sequence).toBeLessThan(permission.sequence);
  expect(JSON.stringify(prior).includes('Zebra-')).toBe(false);
  const asked=permission.event;
  if(asked._tag!=='PermissionRequested')throw Error('Missing request');
  yield* owner.answer({requestId:requestId(),kind:'permission',id:asked.permissionRequestId,optionId:'allow'} as never);
  writeFileSync(gate,'go');
  yield* turnEnded(owner,agent.agentId);
  const page=yield* owner['client.thread']({requestId:requestId(),agentId:agent.agentId,after:0,limit:500} as never);
  if(!('items' in page))throw Error('Expected owner thread');
  const items=page.items;
  const ordered=items.filter(row=>row.event._tag==='ThreadItem'||row.event._tag==='PermissionRequested');
  expect(ordered.map(row=>row.event._tag==='PermissionRequested'?'permission':row.event._tag==='ThreadItem'?row.event.content._tag:'')).toEqual(['text','permission','text','tool-call','text']);
  const text=items.filter(row=>row.event._tag==='ThreadItem'&&row.event.content._tag==='text');
  expect(text[0]?.event._tag==='ThreadItem'&&text[1]?.event._tag==='ThreadItem'&&text[0].event.itemId===text[1].event.itemId).toBe(true);
  const mapped=mapThread(items,agent.agentId).items;
  expect(mapped.filter(item=>typeof item.text==='string'&&item.text.startsWith(ordinary))).toHaveLength(1);
  expect(mapped.find(item=>typeof item.text==='string'&&item.text.startsWith(ordinary))?.text).toBe(ordinary);
  expect(mapped.map(item=>item.type)).toContain('request');
  expect(mapped.filter(item=>item.type==='agent').map(item=>item.text)).toEqual([ordinary,'[REDACTED]','.']);
  expect(mapThread([...items,...items].reverse(),agent.agentId).items).toEqual(mapped);
  service.child.kill('SIGTERM');yield* Effect.promise(()=>service.exit);
  service=startService(root);yield* waitServing(root,service);owner=yield* ownerClient(root);
  const reloaded=yield* owner['client.thread']({requestId:requestId(),agentId:agent.agentId,after:0,limit:500} as never);
  if(!('items' in reloaded))throw Error('Expected reloaded owner thread');
  const priorHead=items.at(-1)!.sequence;
  const replay=reloaded.items.filter(item=>item.sequence<=priorHead);
  expect(replay).toEqual(items);
  expect(mapThread(replay,agent.agentId).items).toEqual(mapped);
})),60_000);

it.live('r2 a split large chunk is one website message on replay and actual service reload',()=>Effect.scoped(Effect.gen(function*(){
  const root=makeServiceRoot('/tmp/s19-r2-delta-');
  let service=startService(root);yield* waitServing(root,service);
  let owner=yield* ownerClient(root);
  const gate=join(root.root,'continue');
  const ordinary='Decided ordinary text. '.repeat(50);
  const agent=yield* startFakeAgent(root,owner,owner,{secrets:{R2_TOKEN:'Zebra-planted-fake-secret-for-r2'},prompt:'go',script:{turns:[[
    {step:'text',text:ordinary+'Zebra-'},{step:'notification_barrier'},{step:'wait_for_file',path:gate},{step:'text',text:'planted-fake-secret-for-r2.'},
  ]]}});
  yield* waitFor('the decided first delta',()=>rowsIn(root).some(row=>{const e=JSON.parse(row.payload_json);return e._tag==='ThreadItem'&&e.content._tag==='text'&&e.content.text===ordinary;}));
  writeFileSync(gate,'go');yield* turnEnded(owner,agent.agentId);
  const read=()=>owner['client.thread']({requestId:requestId(),agentId:agent.agentId,after:0,limit:500} as never);
  const page=yield* read();if(!('items' in page))throw Error('Expected thread');
  const texts=page.items.filter(row=>row.event._tag==='ThreadItem'&&row.event.content._tag==='text');
  expect(texts).toHaveLength(3);
  expect(new Set(texts.map(row=>row.event._tag==='ThreadItem'?row.event.itemId:'')).size).toBe(1);
  expect(texts.map(row=>row.event._tag==='ThreadItem'&&row.event.content._tag==='text'?row.event.content.text:'')).toEqual([ordinary,'[REDACTED]','.']);
  const mapped=mapThread(page.items,agent.agentId).items.filter(item=>item.type==='agent');
  expect(mapped).toHaveLength(1);expect(mapped[0]?.text).toBe(ordinary+'[REDACTED].');
  expect(mapThread([...page.items,...page.items].reverse(),agent.agentId).items.filter(item=>item.type==='agent')).toEqual(mapped);
  service.child.kill('SIGTERM');yield* Effect.promise(()=>service.exit);
  service=startService(root);yield* waitServing(root,service);owner=yield* ownerClient(root);
  const reloaded=yield* read();if(!('items' in reloaded))throw Error('Expected reloaded thread');
  expect(mapThread(reloaded.items,agent.agentId).items.filter(item=>item.type==='agent')).toEqual(mapped);
})),60_000);
