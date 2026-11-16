import { execFileSync } from 'node:child_process';
import * as Redacted from 'effect/Redacted';
import { expect, it } from 'vitest';
import { makeRedactor } from '../../src/identity/redactor.ts';
import { registerRebuildCheck } from '../support/rebuildCheck.ts';
registerRebuildCheck({ noLog: true });
const base64 = (text: string) => Buffer.from(text).toString('base64');
const gnu = (text: string) => (base64(text).match(/.{1,76}/g) ?? []).join('\n') + '\n';
for (const length of [58, 108, 209]) {
  const secret = ('s19-fake-realistic-credential-' + 'aB9_-xY2'.repeat(30)).slice(0, length);
  it(`s19 r3 GNU model matches the real tool for ${length} bytes`, () => {
    const real = execFileSync('/usr/bin/base64', { input: secret, encoding: 'utf8' });
    expect(real === gnu(secret)).toBe(true);
    expect(execFileSync('/usr/bin/base64', { input: real, encoding: 'utf8' }) === gnu(real)).toBe(true);
  });
  for (const [name, encoded] of [
    ['single', gnu(secret)], ['printf pipeline', gnu(gnu(secret))], ['echo pipeline', gnu(gnu(secret + '\n'))],
    ['unwrapped outer', base64(gnu(secret))], ['echo unwrapped outer', base64(gnu(secret + '\n'))],
  ] as const) it(`s19 r3 hides GNU ${name} at ${length} bytes in whole text and every split`, () => {
    const redactor = makeRedactor({ secrets: () => [Redacted.make(secret)], tokenHashes: () => new Set() });
    const safe = (output: string) => output.includes('[REDACTED]') && output.replaceAll('[REDACTED]', '').trim() === '';
    expect(safe(redactor.redactText(encoded))).toBe(true);
    for (let split = 0; split <= encoded.length; split++) {
      const output = redactor.redactChunk('gnu', encoded.slice(0, split), false)
        + redactor.redactChunk('gnu', encoded.slice(split), false) + redactor.redactChunk('gnu', '', true);
      expect(safe(output), `split ${split}`).toBe(true);
    }
    const output = [...encoded].map(c => redactor.redactChunk('chars', c, false)).join('') + redactor.redactChunk('chars', '', true);
    expect(safe(output)).toBe(true);
  });
}
