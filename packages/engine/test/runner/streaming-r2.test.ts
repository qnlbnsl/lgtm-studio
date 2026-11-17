import { join } from 'node:path';
import { chmodSync, writeFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
import { it } from '@effect/vitest';
import * as Effect from 'effect/Effect';
import { afterEach,beforeEach,expect,vi } from 'vitest';
import type { RunnerEvent } from '../../src/runner/Runner.ts';
import { modeOption,fakeAgentPath,accountFor,deliveryFor,echoFile,harnessFor,makeRunnerRig,removeRunnerRig,spawnFor,withAcpRunner,writeFakeAdapter,type RunnerRig } from '../support/acpRunnerRig.ts';
import { registerRebuildCheck } from '../support/rebuildCheck.ts';
registerRebuildCheck({noLog:true});
let rig:RunnerRig;
beforeEach(()=>{rig=makeRunnerRig();vi.stubEnv('HOME',rig.home);vi.stubEnv('LGTM_HOME',rig.stateFolder);});
afterEach(()=>{vi.unstubAllEnvs();removeRunnerRig(rig);});
const tool=(output='safe tool output',id='r2-tool')=>({step:'tool_call',toolCallId:id,title:'Run checks',kind:'execute',status:'in_progress',output});
const secret='Zebra-planted-fake-secret-for-r2';
const messages=(events:ReadonlyArray<RunnerEvent>)=>events.filter((e):e is Extract<RunnerEvent,{_tag:'deliveryUpdate'}>=>e._tag==='deliveryUpdate');
for(const sentence of ['I will run the tests','I will run the tests.']){
  it.live(`r2 ordinary text and tool publish before the adapter gate: ${sentence}`,()=>{
    const hashes=new Set<string>();
    return withAcpRunner(rig,{tokenHashes:hashes},context=>Effect.gen(function*(){
      const gate=join(rig.root,'release');
      const spawn=yield* spawnFor(rig,harnessFor('first',writeFakeAdapter(rig)),accountFor('first',{turns:[[
        {step:'text',text:sentence},tool(),{step:'notification_barrier'},{step:'mode_change',configId:'mode',value:'read-only'},{step:'wait_for_file',path:gate},
      ]]},echoFile(rig)));
      hashes.add(spawn.incarnation);
      yield* context.runner.start(spawn.input);
      const delivery=deliveryFor(spawn.agentId,'go');
      yield* context.runner.prompt({agentId:spawn.agentId,incarnation:spawn.incarnation,delivery,sentSequence:1});
      yield* context.waitFor(e=>e._tag==='modeReported');
      const published=messages(context.events);
      expect(published.map(e=>e.kind)).toEqual(['message','tool-call']);
      expect(published[0]?.text).toBe(sentence);
      writeFileSync(gate,'go');
      yield* context.waitFor(e=>e._tag==='turnEnded');
    }));
  },60_000);
}
for(const withoutWithholdParts of [false,true]) for(const overflow of ['count','bytes'] as const){
  it.live(`r2 ${overflow} overflow (withoutWithholdParts=${withoutWithholdParts}) replaces the undecided tail and releases queued tools in order`,()=>withAcpRunner(rig,{withoutWithholdParts,secrets:{'secret:r2':secret}},context=>Effect.gen(function*(){
    const gate=join(rig.root,'release');
    const reports=overflow==='count'?Array.from({length:1025},(_,i)=>tool(`row ${i}.`,`r2-${i}`)):[tool('x'.repeat(1024*1024+1)+'.')];
    const script=join(rig.root,'large-script.json');
    writeFileSync(script,JSON.stringify({configOptions:[modeOption()],turns:[[
      {step:'text',text:'safe: Zebra-'},...reports,{step:'notification_barrier'},{step:'mode_change',configId:'mode',value:'read-only'},{step:'wait_for_file',path:gate},
      {step:'text',text:'planted-fake-secret-for-r2.'},
    ]]}));
    const adapter=join(rig.bin,'large-adapter');
    writeFileSync(adapter,`#!${process.execPath}\nimport { readFileSync } from 'node:fs';\nprocess.env.FAKE_AGENT_SCRIPT=readFileSync(${JSON.stringify(script)},'utf8');\nimport(${JSON.stringify(pathToFileURL(fakeAgentPath).href)});\n`);chmodSync(adapter,0o755);
    const spawn=yield* spawnFor(rig,harnessFor('first',adapter),accountFor('first',{},echoFile(rig)));
    yield* context.runner.start(spawn.input);
    const delivery=deliveryFor(spawn.agentId,'go');
    yield* context.runner.prompt({agentId:spawn.agentId,incarnation:spawn.incarnation,delivery,sentSequence:1});
    yield* context.waitFor(e=>e._tag==='modeReported');
    const before=messages(context.events);
    expect(before.some(e=>e.text.includes('[output withheld: account credentials cannot be inspected]'))).toBe(true);
    expect(before.filter(e=>e.kind==='tool-call')).toHaveLength(reports.length);
    expect(JSON.stringify(before).includes('Zebra-')).toBe(false);
    expect(before.filter(e=>e.kind==='tool-call').map(e=>e.content?._tag==='tool-call'?e.content.callId:'')).toEqual(reports.map(r=>r.toolCallId));
    writeFileSync(gate,'go');yield* context.waitFor(e=>e._tag==='turnEnded');
    expect(JSON.stringify(messages(context.events)).includes(secret)).toBe(false);
    expect(JSON.stringify(messages(context.events)).includes('planted-fake-secret-for-r2')).toBe(false);
  })),60_000);
}

it.live('s19 r3 a genuine one-character value prefix still queues its tool for the owner decision',()=>withAcpRunner(rig,{secrets:{'secret:r3':'snake-s19-planted-private-secret'}},context=>Effect.gen(function*(){
  const gate=join(rig.root,'value-prefix');
  const spawn=yield* spawnFor(rig,harnessFor('first',writeFakeAdapter(rig)),accountFor('first',{turns:[[
    {step:'text',text:'I will run the tests'},tool(),{step:'notification_barrier'},
    {step:'mode_change',configId:'mode',value:'read-only'},{step:'wait_for_file',path:gate},
    {step:'text',text:'nake-s19-planted-private-secret.'},
  ]]},echoFile(rig)));
  yield* context.runner.start(spawn.input);
  const delivery=deliveryFor(spawn.agentId,'go');
  yield* context.runner.prompt({agentId:spawn.agentId,incarnation:spawn.incarnation,delivery,sentSequence:1});
  yield* context.waitFor(e=>e._tag==='modeReported');
  expect(messages(context.events).map(e=>({kind:e.kind,text:e.text}))).toEqual([{kind:'message',text:'I will run the test'}]);
  writeFileSync(gate,'go');yield* context.waitFor(e=>e._tag==='turnEnded');
  expect(messages(context.events).filter(e=>e.kind==='message').map(e=>e.text).join('')).toBe('I will run the test[REDACTED].');
})),60_000);
