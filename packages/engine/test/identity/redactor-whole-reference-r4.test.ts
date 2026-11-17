// Adapted from the foundry reviewer's w1.ts. The reference deliberately
// uses only indexOf and source masks; it shares the production form list.
import * as Redacted from "effect/Redacted";
import { expect, it } from "vitest";
import { makeRedactor } from "../../src/identity/redactor.ts";
import { addForms, compileForms, maximumIndexFormCharacters, maximumIndexHeadCharacters, type FormLists } from "../../src/identity/redactionForms.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";
registerRebuildCheck({ noLog: true });

const exercise = (seedValue: number) => {
let seed = seedValue;
const rnd = () => { seed |= 0; seed = seed + 0x6D2B79F5 | 0; let t = Math.imul(seed ^ seed >>> 15, 1 | seed); t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t; return ((t ^ t >>> 14) >>> 0) / 4294967296; };
const ri = (n: number) => Math.floor(rnd() * n);
const pick = <T>(xs: readonly T[]): T => xs[ri(xs.length)]!;
const alnum = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789";
const rstr = (n: number, alpha = alnum) => Array.from({ length: n }, () => alpha[ri(alpha.length)]).join("");
const foldHex = (t: string) => t.replace(/[A-F]/g, c => c.toLowerCase());
const foldEscapes = (t: string) => t.replace(/%[0-9A-Fa-f]{2}/g, e => e.toLowerCase());
const mkValues = (n: number, maxLen: number) => {
  const values: string[] = []; const head = "s19-fake-shared-head-0123456789-abcdefghij-";
  while (values.length < n) {
    const k = ri(16); let v: string;
    const len = rnd() < 0.05 ? 8 + ri(maxLen - 7) : 8 + ri(Math.min(maxLen, 90));
    if (k === 0) v = head.slice(0, pick([3, 7, 8, 9, 31, 32, 33, 40])) + rstr(len);
    else if (k === 1 && values.length) { const o = pick(values); v = o.slice(0, 1 + ri(o.length)); }
    else if (k === 2 && values.length) { const o = pick(values); v = o.slice(ri(o.length)); }
    else if (k === 3 && values.length) { const o = pick(values); const a = ri(o.length); v = o.slice(a, a + 1 + ri(o.length - a)); }
    else if (k === 4) v = pick(["a", "ab", "aab", "abc", "0", "f", "A"]).repeat(2 + ri(12));
    else if (k === 5) v = "s19" + pick([".*", "+?", "^$", "{}", "()", "|", "[]", "\\", "\\d", "(?:", "[a-z]", ".", "$&"]) + rstr(len % 20) + pick([".*", ")", "]", "\\", "$"]);
    else if (k === 6) v = pick(["s19-fäke-", "密钥-", "🔑s19", "s19fake🔑", "s19-fak🔑🔑", " s19", "é"]) + rstr(len % 30) + pick(["🔑", "ü", "", "\n", "\u0000"]);
    else if (k === 7) v = "s19 fake/pa ss+" + rstr(len % 20) + pick(["=", "%", "\"", "\\", " ", "%41", "%zz", "+"]) + rstr(3);
    else if (k === 8 && values.length) v = pick(values).slice(-(1 + ri(10))) + rstr(4 + ri(10)); // staggered overlap
    else if (k === 9) v = rstr(1 + ri(3));
    else if (k === 10) v = rstr(len, "abcdef0123456789");
    else if (k === 11) v = rstr(len, "ABCDEF0123456789");
    else if (k === 12 && values.length) v = Buffer.from(pick(values)).toString(pick(["base64", "hex", "base64url"] as const));
    else v = "s19-fake-" + rstr(len, alnum + "-_");
    if (v.length >= 1 && !values.includes(v) && v.isWellFormed()) values.push(v);
  }
  return values;
};

  for (const size of [1, 20, 300]) {
    const stem = "s19-fake-shared-head-0123456789-abcdefghij";
    const values = size === 1 ? ["a"] : [...mkValues(size, 300), "a", stem, stem.slice(0, 12), stem.slice(-10), stem.slice(7, 25),
      "abababababababababa", ".*+?^${}()|[]\\s19-fake", "密钥🔑é-s19-fake", "s19-fake-\ud800"];
    const exact = new Set<string>(), hex = new Set<string>(), percent = new Set<string>();
    for (const value of values) addForms(value, exact, hex, percent);
    const forms: FormLists = { exact: [...exact], hex: [...hex], percent: [...percent] };
    const all = (["exact", "hex", "percent"] as const).flatMap(kind => forms[kind].map(form => ({ kind, form })));
    // These cohorts crossed all three former thresholds: <=256, 257..4096,
    // and >4096 forms. The replacement has one bounded index and a fallback.
    expect(size === 1 ? all.length <= 256 : size === 20 ? all.length > 256 && all.length <= 4096 : all.length > 4096).toBe(true);
    const matcher = compileForms(forms);
    expect(matcher.strategy, `seed=${seedValue}, cohort=${size}`).toBe(size === 300 ? "linear" : "indexed");
    const r = makeRedactor({ secrets: () => values.map(v => Redacted.make(v)), tokenHashes: () => new Set() });
    const check = (text: string, label: string) => {
      const ref = new Uint8Array(text.length), direct = new Uint8Array(text.length), got = new Uint8Array(text.length);
      const views = { exact: text, hex: foldHex(text), percent: foldEscapes(text) };
      for (const { kind, form } of all) for (let start = views[kind].indexOf(form); start !== -1; start = views[kind].indexOf(form, start + 1)) ref.fill(1, start, start + form.length);
      const found: Array<readonly [number, number]> = [];
      matcher.match(text, found);
      for (const [start, end] of found) direct.fill(1, start, end);
      let offset = 0, changedOutside = false;
      const parts = r.redactParts("reference", text, true);
      for (const part of parts) {
        if (part.replacement) {
          got.fill(1, offset, offset + part.length);
          if (part.text !== "[REDACTED]") changedOutside = true;
        }
        else if (part.text !== text.slice(offset, offset + part.length)) changedOutside = true;
        offset += part.length;
      }
      const context = `seed=${seedValue}, cohort=${size}, ${label}, strategy=${matcher.strategy}`;
      expect(Buffer.from(direct).equals(ref), `direct matcher differs from naive spans: ${context}`).toBe(true);
      expect(Buffer.from(got).equals(ref), `redactor differs from naive spans: ${context}`).toBe(true);
      expect(offset === text.length && !changedOutside && parts.map(p => p.text).join("") === r.redactText(text), `outside text or span accounting changed: ${context}`).toBe(true);
    };
    // Every registered form occurs, including at both text boundaries.
    check(all.map(p => p.form).join("\nseparator|"), "all forms at boundaries");
    for (let i = 0; i < 24; i++) {
      let text = i % 2 ? "%4A" : "";
      for (let j = 0; j < 8; j++) {
        const { kind, form } = pick(all), choice = ri(8);
        if (choice < 3) text += kind === "hex" ? [...form].map(c => rnd() < 0.5 ? c.toUpperCase() : c).join("") : kind === "percent" ? form.replace(/%[0-9a-f]{2}/g, e => rnd() < 0.5 ? e.toUpperCase() : e) : form;
        else if (choice === 3) text += form.slice(0, ri(form.length));
        else if (choice === 4) text += form.slice(ri(form.length));
        else if (choice === 5) text += pick(["a", "ab", "0", "f", "A"]).repeat(1 + ri(30));
        else text += pick(["%4A", "%41", "\n", "🔑", "ordinary text.", "[]", "\\", " "]);
      }
      check(text, `random text=${i}`);
    }
  }
};
for (const seed of [1911, 1912, 20261116]) it(`r4 whole-text reference covers former strategy cohorts: seed=${seed}`, () => exercise(seed), 90_000);

it("r4 index budgets select a correct literal fallback without dropping forms", () => {
  for (const exact of [
    ["s19-fake-" + "x".repeat(maximumIndexFormCharacters)],
    Array.from({ length: Math.ceil(maximumIndexHeadCharacters / 4) + 1 }, (_, i) => `s19-fake-head-budget-${i}`),
  ]) {
    const matcher = compileForms({ exact, hex: [], percent: [] });
    expect(matcher.strategy).toBe("linear");
    const text = "safe: " + exact.at(-1)! + ".", found: Array<readonly [number, number]> = [];
    matcher.match(text, found);
    const expected = new Uint8Array(text.length), actual = new Uint8Array(text.length);
    for (const form of exact) for (let start = text.indexOf(form); start !== -1; start = text.indexOf(form, start + 1)) expected.fill(1, start, start + form.length);
    for (const [start, end] of found) actual.fill(1, start, end);
    expect(Buffer.from(actual).equals(expected)).toBe(true);
    expect(matcher.tail(exact.at(-1)!.slice(0, -1)).held).toBe(exact.at(-1)!.length - 1);
  }
});

it("r4 naive reference catches percent folding inside an escape in every cohort", () => {
  for (const extra of [0, 20, 300]) {
    const values = ["a", ...Array.from({ length: extra }, (_, i) => `s19-fake-unrelated-value-${i}`)];
    const exact = new Set<string>(), hex = new Set<string>(), percent = new Set<string>();
    for (const value of values) addForms(value, exact, hex, percent);
    const forms = { exact: [...exact], hex: [...hex], percent: [...percent] }, text = "%4A";
    const expected = new Uint8Array(text.length), actual = new Uint8Array(text.length);
    for (const kind of ["exact", "hex", "percent"] as const) {
      const view = kind === "exact" ? text : kind === "hex" ? text.toLowerCase() : text.replace(/%[0-9a-f]{2}/gi, e => e.toLowerCase());
      for (const form of forms[kind]) for (let start = view.indexOf(form); start !== -1; start = view.indexOf(form, start + 1)) expected.fill(1, start, start + form.length);
    }
    const matcher = compileForms(forms), found: Array<readonly [number, number]> = [];
    matcher.match(text, found);
    for (const [start, end] of found) actual.fill(1, start, end);
    expect(expected[2]).toBe(1);
    expect(Buffer.from(actual).equals(expected), `extra values=${extra}, path=${matcher.strategy}`).toBe(true);
    const r = makeRedactor({ secrets: () => values.map(value => Redacted.make(value)), tokenHashes: () => new Set() });
    const product = new Uint8Array(text.length);
    let offset = 0;
    for (const part of r.redactParts("percent", text, true)) {
      if (part.replacement) product.fill(1, offset, offset + part.length);
      offset += part.length;
    }
    expect(Buffer.from(product).equals(expected), `redactor, extra values=${extra}`).toBe(true);
  }
});
