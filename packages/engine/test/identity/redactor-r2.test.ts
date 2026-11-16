import * as Redacted from 'effect/Redacted';
import { spawnSync } from 'node:child_process';
import { expect, it } from 'vitest';
import { makeRedactor } from '../../src/identity/redactor.ts';
import { mintAgentToken } from '../../src/identity/tokens.ts';
import { registerRebuildCheck } from '../support/rebuildCheck.ts';
registerRebuildCheck({noLog:true});
it('r2 large credential values keep prefix compilation within a bounded heap',()=>{
  const source = new URL('../../src/identity/redactor.ts', import.meta.url).href;
  const result = spawnSync(process.execPath, ['--max-old-space-size=128', '--input-type=module', '--eval', `
    import assert from 'node:assert/strict';
    import * as Redacted from 'effect/Redacted';
    import { makeRedactor } from ${JSON.stringify(source)};
    const secret = 's19-fake-memory-' + 'abcdEFGH1234-漢'.repeat(3000);
    const redactor = makeRedactor({secrets:()=>[Redacted.make(secret)],tokenHashes:()=>new Set()});
    assert.equal(redactor.redactText('ordinary output.'), 'ordinary output.');
    assert.equal(redactor.redactChunk('large', 'safe: '+secret.slice(0,-1), false), 'safe: ');
    assert.equal(redactor.redactChunk('large', secret.slice(-1)+'.', true), '[REDACTED].');
    assert.equal(redactor.redactText(secret), '[REDACTED]');
  `], { encoding: 'utf8', cwd: new URL('../../../../', import.meta.url) });
  expect(result.status).toBe(0);
});
it('r2 ordinary chunks release as they arrive with a stored token hash',()=>{
  const minted=mintAgentToken();
  const redactor=makeRedactor({secrets:()=>[],tokenHashes:()=>new Set([minted.tokenHash])});
  for(const text of ['I will',' run the',' tests now','. Next',' step']) expect(redactor.redactChunk('ordinary',text,false)).toBe(text);
  expect(redactor.redactChunk('ordinary','',true)).toBe('');
});
it('r2 releases the decided prefix of even a short chunk',()=>{
  const redactor=makeRedactor({secrets:()=>[Redacted.make('Zebra-planted-fake-secret')],tokenHashes:()=>new Set()});
  expect(redactor.redactChunk('prefix','safe prefix: Zeb',false)).toBe('safe prefix: ');
  expect(redactor.redactChunk('prefix','ra-planted-fake-secret.',true)).toBe('[REDACTED].');
});
it('r2 protects every split of the hash-only token prefix in raw and encoded forms',()=>{
  const minted=mintAgentToken();
  const token=Redacted.value(minted.token);
  const encodings=[(s:string)=>s,(s:string)=>Buffer.from(s).toString('hex'),(s:string)=>Buffer.from(s).toString('base64'),(s:string)=>Buffer.from(s).toString('base64url'),(s:string)=>[...s].map(c=>'%'+c.charCodeAt(0).toString(16)).join('')];
  for(const encode of encodings){
    const encoded=encode(token);
    for(let split=1;split<encoded.length;split++){
      const redactor=makeRedactor({secrets:()=>[],tokenHashes:()=>new Set([minted.tokenHash])});
      const first=redactor.redactChunk('token','safe: '+encoded.slice(0,split),false);
      const clean=first+redactor.redactChunk('token',encoded.slice(split)+'.',true);
      const publicPart = encode('shp_');
      // Base64's last mixed character and padding are not all public bits.
      const publicLength = encode === encodings[2] || encode === encodings[3] ? 5 : publicPart.length;
      // Once private characters arrive in the same chunk, keep the candidate
      // whole. Public characters from an earlier chunk stay published.
      const published=split <= publicLength ? split : 0;
      const alreadyMatched = first === 'safe: [REDACTED]';
      expect(alreadyMatched || first === 'safe: '+encoded.slice(0,published), `encoding ${encodings.indexOf(encode)} split ${split}`).toBe(true);
      expect(clean === 'safe: '+(alreadyMatched ? '' : encoded.slice(0,published))+'[REDACTED].').toBe(true);
    }
  }
});

it('r2 Unicode secret prefixes use the same UTF-16 spans as their chunks',()=>{
  const secret='🐢-s19-fake-secret';
  for(let split=1;split<secret.length;split++){
    const redactor=makeRedactor({secrets:()=>[Redacted.make(secret)],tokenHashes:()=>new Set()});
    const first=redactor.redactChunk('unicode','safe: '+secret.slice(0,split),false);
    expect(first).toBe('safe: ');
    expect((first+redactor.redactChunk('unicode',secret.slice(split)+'.',true))==='safe: [REDACTED].').toBe(true);
  }
});
it('r2 hash-only base64 alignment context survives every split without releasing private bits',()=>{
  const minted=mintAgentToken(),token=Redacted.value(minted.token);
  for(const offset of [1,2])for(const alphabet of ['base64','base64url'] as const){
    const encoded=Buffer.from('x'.repeat(offset)+token).toString(alphabet);
    const publicLength=Math.floor(8*(offset+4)/6);
    for(let split=1;split<encoded.length;split++){
      const redactor=makeRedactor({secrets:()=>[],tokenHashes:()=>new Set([minted.tokenHash])});
      const first=redactor.redactChunk('aligned',encoded.slice(0,split),false);
      const clean=first+redactor.redactChunk('aligned',encoded.slice(split)+'.',true);
      const pieces=clean.split('[REDACTED]');
      expect(pieces.length,`offset ${offset} ${alphabet} split ${split}`).toBe(2);
      expect(pieces[0]!.length<=publicLength&&encoded.startsWith(pieces[0]!)).toBe(true);
      expect(/^=*\.$/.test(pieces[1]!)).toBe(true);
    }
  }
});
