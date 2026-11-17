import * as Redacted from 'effect/Redacted';
import { expect, it } from 'vitest';
import { makeRedactor } from '../../src/identity/redactor.ts';
import { registerRebuildCheck } from '../support/rebuildCheck.ts';
registerRebuildCheck({ noLog: true });
for (const count of [0, 270]) it(`s19 r3 preserves short, longer and offset-overlapping matches with ${count} extra values`, () => {
  const values = ['ab', 'abcdefghij', 'cdefghijk', ...Array.from({ length: count }, (_, i) => `s19-fake-unrelated-${i}-private`)];
  const redactor = makeRedactor({ secrets: () => values.map(value => Redacted.make(value)), tokenHashes: () => new Set() });
  const text = 'safe: abcdefghijk done.';
  expect(redactor.redactText(text)).toBe('safe: [REDACTED] done.');
  for (let split = 0; split <= text.length; split++) {
    expect(redactor.redactChunk('overlap', text.slice(0, split), false) + redactor.redactChunk('overlap', text.slice(split), true)).toBe('safe: [REDACTED] done.');
  }
});
