// Adapted from the foundry reviewer's f1.ts, using fixed work instead of a
// wall-clock budget. Diagnostics contain positions and counts, never values.
import * as Redacted from "effect/Redacted";
import { expect, it } from "vitest";
import { makeRedactor } from "../../src/identity/redactor.ts";
import { sha256Hex } from "../../src/identity/credential.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";
registerRebuildCheck({ noLog: true });

const exercise = (seedValue: number) => {
let seed = seedValue;
const rnd = () => { seed |= 0; seed = seed + 0x6D2B79F5 | 0; let t = Math.imul(seed ^ seed >>> 15, 1 | seed); t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t; return ((t ^ t >>> 14) >>> 0) / 4294967296; };
const ri = (n: number) => Math.floor(rnd() * n);
const pick = <T>(xs: readonly T[]): T => xs[ri(xs.length)]!;
const rbytes = (n: number) => Buffer.from(Array.from({ length: n }, () => ri(256)));
const alnum = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789";
const rstr = (n: number, alpha = alnum) => Array.from({ length: n }, () => alpha[ri(alpha.length)]).join("");
const tokenFor = (payload = rbytes(32)) => `shp_a_${payload.toString("base64url")}_${sha256Hex(payload).slice(0, 8)}`;
const pct = (c: string) => "%" + c.charCodeAt(0).toString(16).padStart(2, "0");
const frags = ["s", "sh", "shp", "shp_", "shp_a_", "7368", "7368705f", "c2hw", "c2hwX", "%73", "%73%68", "hwX", "p_a_", "hp_", "_a_", "NocF9", "zaHBf", "Y2hw", "%", "%7", "=", "6", "f"];

interface Plant { start: number; end: number; allowed: number; kind: string }
const b64embed = (body: Buffer, url: boolean, publicBytes: number) => {
  const a = ri(3), pre = rbytes(a + 3 * ri(2)), post = rbytes(ri(5));
  const text = Buffer.concat([pre, body, post]).toString(url ? "base64url" : "base64");
  const start = Math.floor(8 * pre.length / 6);
  return { text, start, end: Math.ceil(8 * (pre.length + body.length) / 6), allowed: Math.floor(8 * (pre.length + publicBytes) / 6) - start };
};
// Token spellings recognised from a stored hash.
const tokenSpell = (T: string): { text: string; start: number; end: number; allowed: number; kind: string } => {
  const k = ri(9);
  if (k === 0) return { text: T, start: 0, end: T.length, allowed: 4, kind: "raw" };
  if (k === 1) { const h = Buffer.from(T).toString("hex"); const t = [...h].map(c => rnd() < 0.5 ? c.toUpperCase() : c).join(""); return { text: rnd() < 0.5 ? h : t, start: 0, end: h.length, allowed: 8, kind: "hex" }; }
  if (k === 2 || k === 3) { const e = b64embed(Buffer.from(T), k === 3, 4); return { ...e, kind: "b64" + (k === 3 ? "url" : "") }; }
  const mode = k;
  const p = rnd();
  const parts = [...T].map((c, i) => {
    const esc = mode === 4 ? true : mode === 5 ? i >= 4 : mode === 6 ? i < 4 : mode === 7 ? rnd() < p : i === ri(T.length);
    return esc ? (rnd() < 0.3 ? pct(c).toUpperCase() : pct(c)) : c;
  });
  return { text: parts.join(""), start: 0, end: parts.join("").length, allowed: parts.slice(0, 4).join("").length, kind: "pct" + mode };
};
const valueSpell = (v: string): { text: string; kind: string } => {
  const k = ri(10), b = Buffer.from(v);
  if (k === 0 || k === 1) return { text: v, kind: "raw" };
  if (k === 2) return { text: JSON.stringify(v).slice(1, -1), kind: "json" };
  if (k === 3) return { text: b.toString("base64"), kind: "b64" };
  if (k === 4) return { text: b.toString("base64url"), kind: "b64url" };
  if (k === 5) return { text: b.toString("hex"), kind: "hex" };
  if (k === 6) return { text: b.toString("hex").toUpperCase(), kind: "HEX" };
  if (k === 7) return { text: encodeURIComponent(v), kind: "pct" };
  if (k === 8) return { text: Buffer.from(b.toString("base64")).toString("base64"), kind: "b64b64" };
  return { text: encodeURIComponent(v).replace(/%[0-9A-F]{2}/g, e => rnd() < 0.5 ? e.toLowerCase() : e), kind: "pctmixed" };
};

const mkSet = (nValues: number, nTokens: number) => {
  const tokens = Array.from({ length: nTokens }, () => tokenFor());
  const values: string[] = []; const valueTokensForced: string[] = [];
  while (values.length < nValues) {
    const k = ri(12);
    let v: string;
    if (k === 0) v = pick(frags) + "-s19-fake-" + rstr(4 + ri(20));
    else if (k === 1) v = "s19-fake-" + rstr(4 + ri(20)) + pick(frags);
    else if (k === 2 && values.length > 0) { const o = pick(values); v = o.slice(0, 1 + ri(o.length)); }
    else if (k === 3 && values.length > 0) { const o = pick(values); v = o.slice(ri(o.length)); }
    else if (k === 4) v = "s19-fake-" + rstr(3) + "shp_" + rstr(6 + ri(10));
    else if (k === 5 && tokens.length > 0) { const T = pick(tokens); const a = ri(20); v = T.slice(a, a + 8 + ri(30)); }
    else if (k === 6) v = "s19 fake/pa ss+" + rstr(3 + ri(8)) + pick(["=", "%", "\"", "\\", "é", "\n", " "]) + rstr(3);
    else if (k === 7) v = Buffer.from("s19-fake-" + rstr(6 + ri(30))).toString(pick(["base64", "hex"] as const));
    else if (k === 8) { v = tokenFor(); tokens.push(v); valueTokensForced.push(v); }
    else if (k === 9) v = pick(frags) + rstr(1 + ri(3));
    else v = "s19-fake-" + rstr(8 + ri(80), alnum + "-_");
    if (v.length >= 2 && !values.includes(v)) values.push(v);
  }
  const valueTokens = tokens.filter(t => !valueTokensForced.includes(t) && rnd() < 0.3);
  const all = [...values, ...valueTokens];
  const hashes = new Set(tokens.map(t => sha256Hex(t)));

  const r = makeRedactor({ secrets: () => all.map(v => Redacted.make(v)), tokenHashes: () => hashes });
  return { r, tokens, values };
};
const words = "I will run the tests now and check the file shp sh s 7368 c2hw config.json token=  : \" ' = / + % %7 %73 0x a f 6 Done. see curl -H Authorization Bearer export KEY= base64 ok\n".split(" ");
const mkText = (set: ReturnType<typeof mkSet>) => {
  let text = ""; const plants: Plant[] = [];
  const n = 2 + ri(8);
  for (let i = 0; i < n; i++) {
    const k = ri(10);
    if (k < 3) text += pick(words) + pick([" ", " ", "", "="]);
    else if (k < 6 && set.tokens.length > 0) {
      const s = tokenSpell(pick(set.tokens));
      plants.push({ start: text.length + s.start, end: text.length + s.end, allowed: s.allowed, kind: "tok-" + s.kind });
      text += s.text + pick(["", "", " ", ".", "=", "%"]);
    } else if (k < 8 && set.values.length > 0) {
      const vv = pick(set.values); const s = valueSpell(vv);
      plants.push({ start: text.length, end: text.length + s.text.length, allowed: 0, kind: "val-" + s.kind });
      if (/^shp_a_[A-Za-z0-9_-]{43}_[a-f0-9]{8}$/.test(vv)) plants[plants.length - 1] = { start: text.length, end: text.length + s.text.length, allowed: ({ raw: 4, json: 4, pct: 4, pctmixed: 4, hex: 8, HEX: 8, b64: 5, b64url: 5, b64b64: 6 } as any)[s.kind], kind: "tok-value-" + s.kind };
      text += s.text + pick(["", "", " ", ".", "="]);
    } else if (k === 8 && set.tokens.length > 0) { // truncated decoy
      const s = tokenSpell(pick(set.tokens)); text += s.text.slice(0, 1 + ri(Math.max(1, s.end - 3))) + pick(["", " "]);
    } else text += pick(frags) + pick(["", "", " "]);
  }
  return { text, plants };
};


  let streams = 0, texts = 0, unsafe = 0, withheld = 0, missing = 0, oracleMismatch = 0;
  let firstFailure = "", id = 0;
  const check = (set: ReturnType<typeof mkSet>, text: string, plants: Plant[], directed?: { cuts: number[]; withholdAt?: number[] }) => {
    texts++;
    const whole = set.r.redactParts("oracle", text, true), hidden = new Uint8Array(text.length), allowed = new Uint8Array(text.length);
    let offset = 0;
    for (const part of whole) { if (part.replacement) hidden.fill(1, offset, offset + part.length); offset += part.length; }
    if (offset !== text.length || whole.map(p => p.text).join("") !== set.r.redactText(text)) oracleMismatch++;
    for (const plant of plants) {
      const token = plant.kind.startsWith("tok-");
      if (token) allowed.fill(1, plant.start, plant.start + plant.allowed);
      const boundary = plant.kind.includes("b64") ? 1 : 0;
      for (let i = plant.start + (token ? plant.allowed : 0); i < plant.end - boundary; i++) if (!hidden[i]) { missing++; break; }
    }
    const run = (cuts: readonly number[], withhold: boolean, forced?: ReadonlySet<number>) => {
      const item = "stream-" + id++; let previous = 0, offset = 0, failure = -1;
      const consume = (parts: ReturnType<typeof set.r.redactParts>) => {
        for (const part of parts) {
          if (part.replacement && !["[REDACTED]", "[W]", ""].includes(part.text)) failure = offset;
          if (!part.replacement) {
            if (part.text !== text.slice(offset, offset + part.length)) failure = offset;
            for (let i = offset; i < offset + part.length; i++) if (hidden[i] && !allowed[i]) failure = i;
          }
          offset += part.length;
        }
      };
      for (const end of [...cuts, text.length]) {
        consume(set.r.redactParts(item, text.slice(previous, end), false)); previous = end;
        if (forced ? forced.has(end) : withhold && rnd() < 0.15) { consume(set.r.withholdParts(item, "[W]")); withheld++; }
      }
      consume(set.r.redactParts(item, "", true)); streams++;
      if (failure >= 0 || offset !== text.length) { unsafe++; firstFailure ||= `text=${texts}, stream=${streams}, at=${failure}, withhold=${withhold}`; }
    };
    for (const size of [1, 2, 3, 7]) for (const phase of [0, 1, 2].slice(0, Math.min(size, 3))) {
      const cuts: number[] = []; for (let end = phase || size; end < text.length; end += size) cuts.push(end);
      run(cuts, false); run(cuts, true);
    }
    for (let mode = 0; mode < 4; mode++) {
      const cuts: number[] = []; for (let end = 1 + ri(20); end < text.length; end += 1 + ri(20)) cuts.push(end);
      run(cuts, mode % 2 === 1);
    }
    const step = Math.max(1, Math.floor(text.length / 40));
    for (let cut = 0; cut <= text.length; cut += step) run([cut], cut % 2 === 0);
    for (let i = 0; i < 10; i++) { const a = ri(text.length + 1), b = ri(text.length + 1); run([Math.min(a, b), Math.max(a, b)], i % 2 === 0); }
    if (directed) run(directed.cuts, false, new Set(directed.withholdAt));
  };
  for (let round = 0; round < 3; round++) for (const count of [0, 1, 3, 20, 70]) {
    const set = mkSet(count, 2);
    for (let i = 0; i < 6; i++) { const { text, plants } = mkText(set); check(set, text, plants); }
  }
  // Explicit two-token, nested-prefix, value-first and adjacent-item plants.
  const nested = tokenFor(Buffer.from("AAAAAAAAAAAAAAAAAAAAshp_BBBBBBBBBBBBBBBBBBB", "base64url"));
  const second = tokenFor(Buffer.alloc(32, 7));
  const login = "s19-fake-login-value-0123456789-abcdefghijklmnop-XY";
  const values = [login, "p-XY-s19-fake-second", "s19-fake-AAAA-0123456789abcdefghij", "0123456789abcdefghij-KLMN", "-KLMN-s19-fake-third", "e-s19-fake-value-secret"];
  const tokens = [nested, second], r = makeRedactor({ secrets: () => values.map(v => Redacted.make(v)), tokenHashes: () => new Set(tokens.map(sha256Hex)) });
  const set = { r, tokens, values };
  for (const text of [nested + second, values[2]! + nested, "pre " + values[2] + "-KLMN post", "see " + second + " ok", nested + values[3] + second, "key=" + login + "."]) {
    const plants: Plant[] = [];
    for (const token of tokens) for (let start = text.indexOf(token); start !== -1; start = text.indexOf(token, start + 1)) plants.push({ start, end: start + token.length, allowed: 4, kind: "tok-raw" });
    // Exact B2 schedules: replacement overflow just before completion, and
    // overlapping completed values without an overflow replacement.
    const directed = text.startsWith("see ") ? { cuts: [61, 62], withholdAt: [61] }
      : text.startsWith("key=") ? { cuts: [4 + login.length - 4, 4 + login.length], withholdAt: [4 + login.length - 4] }
      : text.startsWith("pre ") ? { cuts: [4 + values[2]!.length, 9 + values[2]!.length], withholdAt: [] }
      : undefined;
    check(set, text, plants, directed);
  }
  // An inner public prefix at the same encoding alignment must not shadow
  // the token's real start in hexadecimal or base64 either.
  const aligned = tokenFor(Buffer.from("A".repeat(18) + "shp_" + "B".repeat(21), "base64url"));
  const hashSet = { r: makeRedactor({ secrets: () => [], tokenHashes: () => new Set([nested, aligned].map(sha256Hex)) }), tokens: [nested, aligned], values: [] };
  for (const token of hashSet.tokens) for (const encoding of ["hex", "base64"] as const) {
    const encoded = Buffer.from(token).toString(encoding), text = "pre " + encoded + " ok";
    check(hashSet, text, [{ start: 4, end: 4 + encoded.length, allowed: encoding === "hex" ? 8 : 5, kind: "tok-" + encoding }]);
  }
  expect(oracleMismatch, `seed=${seedValue}`).toBe(0);
  expect(missing, `whole plant misses, seed=${seedValue}`).toBe(0);
  expect(unsafe, `seed=${seedValue}; ${firstFailure}`).toBe(0);
  expect(withheld).toBeGreaterThan(100);
  expect(streams).toBeGreaterThan(5000);
};
for (const seed of [20261116, 1911, 0x194019]) it(`r4 streamed reference plants values, two tokens, overlaps and random withholding: seed=${seed}`, () => exercise(seed), 90_000);

it("r4 item identifiers keep independent carries across interleaved and reused items", () => {
  const values = ["s19-fake-first-private-secret", "s19-fake-second-private-secret"];
  const r = makeRedactor({ secrets: () => values.map(v => Redacted.make(v)), tokenHashes: () => new Set() });
  const result = ["", ""];
  for (let round = 0; round < 2; round++) {
    for (let i = 0; i < 2; i++) result[i] += r.redactChunk(`item-${i}`, "safe: " + values[i]!.slice(0, 18), false);
    for (let i = 1; i >= 0; i--) result[i] += r.redactChunk(`item-${i}`, values[i]!.slice(18) + ".", true);
  }
  expect(result.every(text => text === "safe: [REDACTED].safe: [REDACTED].")).toBe(true);
});
