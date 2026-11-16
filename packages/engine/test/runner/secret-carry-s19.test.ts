import { join } from 'node:path';
import { writeFileSync } from 'node:fs';
import { it } from '@effect/vitest';
import * as Effect from 'effect/Effect';
import * as Redacted from 'effect/Redacted';
import { afterEach, beforeEach, expect, vi } from 'vitest';
import { accountFor, deliveryFor, echoFile, harnessFor, makeRunnerRig, removeRunnerRig, spawnFor, withAcpRunner, writeFakeAdapter, type RunnerRig } from '../support/acpRunnerRig.ts';
import { registerRebuildCheck } from '../support/rebuildCheck.ts';
import type { RunnerEvent } from '../../src/runner/Runner.ts';
registerRebuildCheck({noLog:true});
let rig:RunnerRig;
beforeEach(()=>{rig=makeRunnerRig();vi.stubEnv('HOME',rig.home);vi.stubEnv('LGTM_HOME',rig.stateFolder);});
afterEach(()=>{vi.unstubAllEnvs();removeRunnerRig(rig);});
const secret='s19-fake-long-streamed-credential-7919';
const frame=(update:unknown)=>({step:'raw_frame',frame:JSON.stringify({jsonrpc:'2.0',method:'session/update',params:{sessionId:'${session}',update}})});
const middle: Record<string,Record<string,unknown>> = {
  'tool call':frame({sessionUpdate:'tool_call',toolCallId:'s19-tool',title:'safe tool',kind:'execute',status:'pending'}),
  'tool update':{step:'tool_call',toolCallId:'s19-tool',title:'safe tool',kind:'execute',status:'completed',output:'safe output'},
  plan:{step:'plan',entries:[{content:'safe plan',priority:'high',status:'pending'}]},
  'permission request':{step:'raw_frame',frame:JSON.stringify({jsonrpc:'2.0',id:'s19-permission',method:'session/request_permission',params:{sessionId:'${session}',options:[{optionId:'allow',name:'Allow',kind:'allow_once'}],toolCall:{toolCallId:'s19-tool',title:'safe permission'}}})},
  'mode change':{step:'mode_change',configId:'mode',value:'read-only'},
  thought:frame({sessionUpdate:'agent_thought_chunk',content:{type:'text',text:'safe thought'}}),
};
for(const [name,update] of Object.entries(middle)) {
  it.live(`s19 carry survives a ${name} between two secret chunks`,()=>withAcpRunner(rig,{secrets:{'secret:s19':secret}},context=>Effect.gen(function*(){
    const gate=join(rig.root,'go');
    const echo=echoFile(rig);
    const script={...(name==='tool call'?{protocolVersion:1}:{}),turns:[[frame({sessionUpdate:'agent_message_chunk',messageId:'s19-before',content:{type:'text',text:'safe-before '+secret.slice(0,17)}}),{step:'notification_barrier'},update,{step:'wait_for_file',path:gate},frame({sessionUpdate:'agent_message_chunk',messageId:'s19-after',content:{type:'text',text:secret.slice(17)+' safe-after'}})]]};
    const spawn=yield* spawnFor(rig,harnessFor('first',writeFakeAdapter(rig)),accountFor('first',script,echo));
    yield* context.runner.start(spawn.input);
    const delivery=deliveryFor(spawn.agentId,'go');
    yield* context.runner.prompt({agentId:spawn.agentId,incarnation:spawn.incarnation,delivery,sentSequence:1});
    if(name==='permission request') {
      const request=yield* context.waitFor(e=>e._tag==='permissionRequested');
      if(request._tag==='permissionRequested') yield* context.runner.answerPermission({agentId:spawn.agentId,requestId:request.permissionRequestId,optionId:'allow'});
    }
    writeFileSync(gate,'go');
    yield* context.waitFor(e=>e._tag==='turnEnded' && e.deliveryId===delivery.id);
    const updates=context.events.filter((e):e is Extract<RunnerEvent,{_tag:'deliveryUpdate'}>=>e._tag==='deliveryUpdate'&&e.deliveryId===delivery.id);
    const text=updates.filter(e=>e.kind==='message').map(e=>e.text).join('');
    expect(text==='safe-before [REDACTED] safe-after').toBe(true);
    const contentKinds=updates.filter(e=>e.kind!=='available_commands_update').map(e=>e.kind);
    if(name==='tool call'||name==='tool update'||name==='plan') {
      expect(contentKinds).toEqual(['message','message',name==='plan'?'plan':'tool-call','message']);
      expect(updates[0]?.text).toBe('safe-before ');
      expect(updates[1]?.text).toBe('[REDACTED]');
      expect(updates[0]?.itemId).toBe(updates[1]?.itemId);
      expect(updates.at(-1)?.text).toBe(' safe-after');
    }
  })),60_000);
}
for(const end of ['end of turn','stop','crash','cancel'] as const) {
  it.live(`s19 ${end} flushes a genuine secret prefix and its queued tool`,()=>withAcpRunner(rig,{secrets:{'secret:s19':secret}},context=>Effect.gen(function*(){
    const gate=join(rig.root,'terminate'), idleGate=join(rig.root,'cancel-idle');
    const prefix=secret.slice(0,14);
    const update=end==='crash'?{step:'crash',exitCode:3}:{step:'end_turn',stopReason:'end_turn'};
    const spawn=yield* spawnFor(rig,harnessFor('first',writeFakeAdapter(rig)),accountFor('first',{cancelIdleGate:idleGate,turns:[[
      {step:'text',text:prefix},{step:'tool_call',toolCallId:'queued',title:'safe tool',kind:'read',status:'pending',output:'safe output'},
      {step:'notification_barrier'},{step:'mode_change',configId:'mode',value:'read-only'},{step:'wait_for_file',path:gate},update,
    ]]},echoFile(rig)));
    yield* context.runner.start(spawn.input);
    const delivery=deliveryFor(spawn.agentId,'go');
    yield* context.runner.prompt({agentId:spawn.agentId,incarnation:spawn.incarnation,delivery,sentSequence:1});
    yield* context.waitFor(e=>e._tag==='modeReported');
    const updates=()=>context.events.filter((e):e is Extract<RunnerEvent,{_tag:'deliveryUpdate'}>=>e._tag==='deliveryUpdate'&&e.deliveryId===delivery.id);
    expect(updates()).toEqual([]);
    if(end==='stop') yield* context.runner.stop({agentId:spawn.agentId});
    else if(end==='cancel') yield* context.runner.cancel({agentId:spawn.agentId});
    else writeFileSync(gate,'go');
    if(end==='crash') yield* context.waitFor(e=>e._tag==='exited');
    else if(end==='end of turn') yield* context.waitFor(e=>e._tag==='turnEnded');
    // No idle report can rescue a removed cancel flush.
    if(end==='cancel') {
      yield* context.waitFor(e=>e._tag==='deliveryUpdate'&&e.kind==='tool-call');
      expect(context.events.some(e=>e._tag==='turnEnded')).toBe(false);
    }
    expect(updates().map(e=>e.kind)).toEqual(['message','tool-call']);
    expect(updates()[0]?.text===prefix).toBe(true);
    if(end==='cancel') {writeFileSync(idleGate,'go');yield* context.waitFor(e=>e._tag==='turnEnded');}
    if(end==='crash'||end==='end of turn') {
      const last=context.events.indexOf(updates().at(-1)!);
      const ended=context.events.findIndex(e=>e._tag===(end==='crash'?'exited':'turnEnded'));
      expect(last).toBeLessThan(ended);
    }
  })),60_000);
}
it.live('s19 short replacements before a trailing tool keep all text and its event position',()=>withAcpRunner(rig,{secrets:{'secret:short':'~'}},context=>Effect.gen(function*(){
  const spawn=yield* spawnFor(rig,harnessFor('first',writeFakeAdapter(rig)),accountFor('first',{turns:[[
    {step:'text',text:'a~b'},{step:'tool_call',toolCallId:'tail-tool',title:'safe tool',kind:'execute',status:'completed',output:'safe tool output'},
  ]]},echoFile(rig)));
  yield* context.runner.start(spawn.input);
  const delivery=deliveryFor(spawn.agentId,'go');
  yield* context.runner.prompt({agentId:spawn.agentId,incarnation:spawn.incarnation,delivery,sentSequence:1});
  yield* context.waitFor(e=>e._tag==='turnEnded');
  const updates=context.events.filter((e):e is Extract<RunnerEvent,{_tag:'deliveryUpdate'}>=>e._tag==='deliveryUpdate'&&e.deliveryId===delivery.id);
  expect(updates.map(e=>e.kind)).toEqual(['message','tool-call']);
  expect(updates[0]?.text).toBe('a[REDACTED]b');
})),60_000);
it.live('s19 decided ordinary text streams before a pause while only one window remains held',()=>{
  const longestSecret='s19-fake-long-credential-'+ 'q'.repeat(200);
  return withAcpRunner(rig,{secrets:{'secret:s19':longestSecret}},context=>Effect.gen(function*(){
    const gate=join(rig.root,'hold-large-chunk');
    const ordinary='ordinary content. '.repeat(1000);
    const text=ordinary+Buffer.from(longestSecret).toString('hex').slice(0,-1);
    const spawn=yield* spawnFor(rig,harnessFor('first',writeFakeAdapter(rig)),accountFor('first',{turns:[[
      {step:'text',text},{step:'notification_barrier'},{step:'mode_change',configId:'mode',value:'read-only'},{step:'wait_for_file',path:gate},
    ]]},echoFile(rig)));
    yield* context.runner.start(spawn.input);
    const delivery=deliveryFor(spawn.agentId,'go');
    yield* context.runner.prompt({agentId:spawn.agentId,incarnation:spawn.incarnation,delivery,sentSequence:1});
    yield* context.waitFor(e=>e._tag==='modeReported');
    const streamed=()=>context.events.filter(e=>e._tag==='deliveryUpdate'&&e.deliveryId===delivery.id&&e.kind==='message').map(e=>e._tag==='deliveryUpdate'?e.text:'').join('');
    const longest=2*Math.max(Redacted.value(spawn.input.token).length,longestSecret.length);
    expect(text.length-streamed().length).toBe(longest-1);
    expect(streamed()).toBe(ordinary);
    writeFileSync(gate,'go');
    yield* context.waitFor(e=>e._tag==='turnEnded');
    expect(streamed()).toBe(text);
  }));
},60_000);
