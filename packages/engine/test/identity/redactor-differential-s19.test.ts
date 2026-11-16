import * as Redacted from 'effect/Redacted';
import { describe, expect, it } from 'vitest';
import { makeRedactor } from '../../src/identity/redactor.ts';
import { sha256Hex } from '../../src/identity/credential.ts';
import { registerRebuildCheck } from '../support/rebuildCheck.ts';
registerRebuildCheck({ noLog: true });

// Deterministic fake credentials. Diagnostics name only spelling and split
// positions; a failed assertion must never print a credential or its output.
const bytes = (seed: number, length: number) => {
  let state = seed;
  return Buffer.from(Array.from({ length }, () => {
    state = (Math.imul(state, 1664525) + 1013904223) >>> 0;
    return state >>> 24;
  }));
};
const tokenFor = (seed: number) => {
  const payload = bytes(seed, 32);
  return `shp_a_${payload.toString('base64url')}_${sha256Hex(payload).slice(0, 8)}`;
};
const escape = (c: string) => `%${c.charCodeAt(0).toString(16)}`;
interface Spelling { name: string; text: string; publicEnd: number }
const spellings = (token: string): Spelling[] => {
  const percent = (name: string, escaped: (i: number) => boolean, upper = false): Spelling => {
    const encode = (text: string) => [...text].map((c, i) => escaped(i) ? upper ? escape(c).toUpperCase() : escape(c) : c).join('');
    return { name, text: encode(token), publicEnd: encode(token.slice(0, 4)).length };
  };
  return [
    { name: 'raw', text: token, publicEnd: 4 },
    ...(['hex', 'HEX'] as const).map(name => ({ name, text: name === 'hex' ? Buffer.from(token).toString('hex') : Buffer.from(token).toString('hex').toUpperCase(), publicEnd: 8 })),
    ...(['base64', 'base64url'] as const).flatMap(alphabet => [0, 1, 2].map(offset => ({
      name: `${alphabet}-${offset}`, text: Buffer.from('x'.repeat(offset) + token + 'z').toString(alphabet), publicEnd: Math.floor(8 * (offset + 4) / 6),
    }))),
    percent('percent-all', () => true), percent('percent-upper', () => true, true),
    percent('literal-prefix-escaped-rest', i => i >= 4),
    percent('escaped-prefix-literal-rest', i => i < 4),
    percent('one-escape', i => i === 10), percent('last-escape', i => i === token.length - 1),
    percent('alternating', i => i % 2 === 0), percent('alternating-other', i => i % 2 === 1),
  ];
};

const check = (token: string, values: readonly string[], spelling: Spelling, exhaustive: boolean) => {
  const redactor = makeRedactor({ secrets: () => values.map(value => Redacted.make(value)), tokenHashes: () => new Set([sha256Hex(token)]) });
  const text = 't=' + spelling.text + '.';
  const whole = redactor.redactText(text);
  const parts = redactor.redactParts('whole', text, true);
  if (parts.map(part => part.text).join('') !== whole || !whole.includes('[REDACTED]')) throw Error('Invalid whole-text oracle');
  const protectedAt = new Uint8Array(text.length);
  let at = 0;
  for (const part of parts) {
    if (part.replacement) protectedAt.fill(1, at, at + part.length);
    at += part.length;
  }
  // Only token characters made entirely from its fixed public prefix (and
  // preceding non-secret base64 alignment bytes) are excepted by the design.
  protectedAt.fill(0, 2, 2 + spelling.publicEnd);
  let chunkings = 0, failures = 0;
  let firstFailure = '';
  const stream = (cuts: readonly number[]) => {
    let offset = 0, previous = 0, bad = false;
    const consume = (output: ReturnType<typeof redactor.redactParts>) => {
      for (const part of output) {
        if (!part.replacement) {
          if (part.text !== text.slice(offset, offset + part.length)) bad = true;
          for (let i = offset; i < offset + part.length; i++) if (protectedAt[i]) bad = true;
        }
        offset += part.length;
      }
    };
    for (const end of [...cuts, text.length]) {
      consume(redactor.redactParts('stream', text.slice(previous, end), false));
      previous = end;
    }
    consume(redactor.redactParts('stream', '', true));
    if (offset !== text.length) bad = true;
    chunkings++;
    if (bad) { failures++; firstFailure ||= cuts.join(','); }
  };
  if (exhaustive) {
    for (let i = 0; i <= text.length; i++) {
      stream([i]);
      for (let j = i; j <= text.length; j++) stream([i, j]);
    }
  } else {
    for (let i = 0; i <= text.length; i++) stream([i]);
  }
  stream(Array.from({ length: text.length - 1 }, (_, i) => i + 1));
  expect(failures, `${spelling.name}: ${failures}/${chunkings} unsafe chunkings; first cuts ${firstFailure}`).toBe(0);
};

it('s19 r3 differential B1: literal prefix with escaped remainder', () => {
  const token = tokenFor(0x193019);
  check(token, [], spellings(token).find(s => s.name === 'literal-prefix-escaped-rest')!, false);
});
it('s19 r3 differential B2: value suffix cannot displace public context', () => {
  const token = tokenFor(0x193019);
  check(token, ['6-s19-fake-context-secret'], spellings(token).find(s => s.name === 'percent-all')!, false);
});
for (const count of [0, 1, 3, 20]) describe(`s19 r3 differential with ${count} fake value secrets`, () => {
  const token = tokenFor(0x193019 + count);
  const starts = '6Qsc2hY7%5aZbP1_+/';
  const values = Array.from({ length: count }, (_, i) => starts[i % starts.length]! + '-s19-fake-' + bytes(0x193019 + i, 8).toString('hex'));
  for (const spelling of spellings(token)) it(`protects every two-way, three-way and character split: ${spelling.name}`, () => check(token, values, spelling, true), 120_000);
});
