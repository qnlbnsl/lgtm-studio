import * as Redacted from 'effect/Redacted';
import { expect, it } from 'vitest';
import { makeRedactor } from '../../src/identity/redactor.ts';
import { registerRebuildCheck } from '../support/rebuildCheck.ts';
registerRebuildCheck({ noLog: true });
const json = (s: string) => JSON.stringify(s).slice(1,-1);
const base64 = (s: string) => Buffer.from(s).toString('base64');
const forms = {
  doubleJson: (s: string) => json(json(s)),
  base64Json: (s: string) => base64(json(s)),
  jsonBase64: (s: string) => json(base64(s)),
  urlBase64: (s: string) => encodeURIComponent(base64(s)),
  doubleBase64: (s: string) => base64(base64(s)),
};
for (const [name, encode] of Object.entries(forms)) {
  it(`s19 ${name} is redacted whole and across streamed chunks`, () => {
    const secret = 's19-fake-+ /"\\-漢-credential-7919';
    const redactor = makeRedactor({ secrets: () => [Redacted.make(secret)], tokenHashes: () => new Set() });
    const encoded = encode(secret);
    expect(redactor.redactText('before '+ encoded +' after') === 'before [REDACTED] after').toBe(true);
    const split = Math.floor(encoded.length/2);
    const result = redactor.redactChunk(name, 'before '+encoded.slice(0,split), false)
      + redactor.redactChunk(name, encoded.slice(split)+' after', true);
    expect(result === 'before [REDACTED] after').toBe(true);
  });
}
it('s19 short explicit secrets do not register partial base64 fragments', () => {
  const redactor = makeRedactor({ secrets: () => [Redacted.make('~')], tokenHashes: () => new Set() });
  expect(redactor.redactText('ordinary abc XYZ 123')).toBe('ordinary abc XYZ 123');
  expect(redactor.redactText(base64(base64('~')))).toBe('[REDACTED]');
});
