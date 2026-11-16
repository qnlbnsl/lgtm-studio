import type { Event } from "@lgtm/contracts";
import * as Effect from "effect/Effect";
import * as Layer from "effect/Layer";
import * as Redacted from "effect/Redacted";

import { Redactor, type RedactionPart } from "../log/Redactor.ts";
import { eventStructure, fieldStructure, type RedactionStructure } from "../log/redactionStructure.ts";
import { credentialLength, hasValidChecksum, sha256Hex } from "./credential.ts";

const replacement = "[REDACTED]";

// Encoded forms shorter than this are not matched on their own. Only the
// partial base64 runs of a very short secret are that short, and matching
// them would blank unrelated text. The full forms are always matched.
const minimumPartialLength = 8;

// The longest text an encoded credential can occupy: every character
// percent-escaped. Hexadecimal (2 per byte) and base64 are shorter.
const longestCredentialForm = 3 * credentialLength;

// The characters of base64 output that depend only on `bytes` when they start
// `alignment` bytes into a longer encoded value (decision row 73: all three
// alignments). The leading and trailing characters that mix in neighbouring
// bytes are left out.
const base64Run = (bytes: Buffer, alignment: number): string => {
  const padded = Buffer.concat([Buffer.alloc(alignment), bytes, Buffer.alloc(2)]);
  const first = Math.ceil((8 * alignment) / 6);
  const end = Math.floor((8 * (alignment + bytes.byteLength)) / 6);
  return padded.toString("base64").slice(first, end);
};

const toBase64url = (base64: string): string => base64.replaceAll("+", "-").replaceAll("/", "_");

// The matched forms of every live secret, by the text they are searched in.
// `exact` forms are matched as written. `hex` forms are lower-case and are
// searched in a copy of the text with A-F folded to lower case, so
// hexadecimal matches in any case. `percent` forms have lower-case escapes
// and are searched in a copy with only the digits of each `%xx` escape
// folded, so the escapes match in any case and literal letters do not.
interface Forms {
  readonly exact: ReadonlyArray<string>;
  readonly hex: ReadonlyArray<string>;
  readonly percent: ReadonlyArray<string>;
  readonly publicLengths: ReadonlyMap<string, number>;
}

// Every form of one secret (decision rows 57 and 73, fix round 1 rulings 1
// and 2): raw; as it appears inside a JSON string; base64 and base64url,
// whole and at each byte alignment inside a longer value; URL-encoded, with
// `%20` or `+` for a space; and hexadecimal.
const addForms = (secret: string, exact: Set<string>, hex: Set<string>, percent: Set<string>): void => {
  if (secret.length === 0) return;
  const bytes = Buffer.from(secret, "utf8");
  exact.add(secret);
  const json = JSON.stringify(secret).slice(1, -1);
  const base64 = bytes.toString("base64");
  exact.add(json);
  // Bounded two-step set (issue 232): JSON(JSON(secret)), base64(JSON(secret)),
  // JSON(base64(secret)), URL(base64(secret)), and base64(base64(secret)).
  // JSON leaves base64 unchanged. Do not recursively compose forms or add
  // partial runs except for double base64, whose inner and outer alignments
  // below cover contiguous runs; GNU line wrapping is handled separately.
  exact.add(JSON.stringify(json).slice(1, -1));
  exact.add(Buffer.from(json, "utf8").toString("base64"));
  exact.add(JSON.stringify(base64).slice(1, -1));
  percent.add(encodeURIComponent(base64).replace(/%[0-9A-F]{2}/g, (escape) => escape.toLowerCase()));
  exact.add(Buffer.from(base64, "utf8").toString("base64"));
  exact.add(bytes.toString("base64"));
  exact.add(bytes.toString("base64url"));
  for (const alignment of [0, 1, 2]) {
    const run = base64Run(bytes, alignment);
    if (run.length < minimumPartialLength) continue;
    exact.add(run);
    exact.add(toBase64url(run));
  }
  // Three inner byte alignments, each at three outer byte alignments.
  // Standard and URL-safe outer runs add at most eighteen forms per secret.
  for (const innerAlignment of [0, 1, 2]) {
    const inner = base64Run(bytes, innerAlignment);
    if (inner.length < minimumPartialLength) continue;
    for (const outerAlignment of [0, 1, 2]) {
      const run = base64Run(Buffer.from(inner, "ascii"), outerAlignment);
      if (run.length < minimumPartialLength) continue;
      exact.add(run);
      exact.add(toBase64url(run));
    }
  }
  // GNU base64 uses 76 columns and a final LF. Full pipeline forms cover
  // the entire value, including internal line breaks, with or without the
  // final output LF. They add at most six forms. No recursive expansion.
  const gnu = (value: string): string => (Buffer.from(value).toString("base64").match(/.{1,76}/g) ?? []).join("\n") + "\n";
  for (const value of [secret, secret + "\n"]) {
    const inner = gnu(value);
    exact.add(inner.slice(0, -1));
    exact.add(gnu(inner).slice(0, -1));
    exact.add(Buffer.from(inner).toString("base64"));
  }
  // The first and last inner line segments at each byte alignment also
  // contribute bounded partial runs. At most 36 additional forms; the
  // existing eight-character gate applies to both inner and outer runs.
  for (const alignment of [0, 1, 2]) {
    const inner = base64Run(bytes, alignment);
    if (inner.length <= 76) continue;
    const firstColumn = Math.ceil(8 * alignment / 6);
    const firstLength = 76 - firstColumn;
    const lastStart = firstLength + 76 * Math.floor((inner.length - firstLength - 1) / 76);
    for (const segment of new Set([inner.slice(0, firstLength), inner.slice(lastStart)])) {
      if (segment.length < minimumPartialLength) continue;
      for (const outer of [0, 1, 2]) {
        const run = base64Run(Buffer.from(segment, "ascii"), outer);
        if (run.length < minimumPartialLength) continue;
        exact.add(run); exact.add(toBase64url(run));
      }
    }
  }
  const url = encodeURIComponent(secret).replace(/%[0-9A-F]{2}/g, (escape) => escape.toLowerCase());
  percent.add(url);
  percent.add(url.replaceAll("%20", "+"));
  hex.add(bytes.toString("hex"));
};

// Where an encoded credential can start (fix round 1, ruling 3): the
// base64 and base64url runs of each six-byte prefix at each alignment, and
// the hexadecimal of the shared `shp_` prefix.
interface Base64Probe {
  readonly run: string;
  readonly alignment: number;
  readonly alphabet: RegExp;
}
const base64Probes: ReadonlyArray<Base64Probe> = ["shp_a_"].flatMap((prefix) =>
  [0, 1, 2].flatMap((alignment) => {
    const run = base64Run(Buffer.from(prefix, "latin1"), alignment);
    return [
      { run, alignment, alphabet: /^[A-Za-z0-9+/]*$/ },
      { run: toBase64url(run), alignment, alphabet: /^[A-Za-z0-9_-]*$/ },
    ];
  }));
const hexPrefix = Buffer.from("shp_", "latin1").toString("hex");

type Interval = readonly [start: number, end: number];

// A copy of the text, of the same length, with A-F folded to lower case.
const foldHex = (text: string): string => text.replace(/[A-F]/g, (letter) => letter.toLowerCase());

// A copy of the text, of the same length, with only the digits of each
// `%xx` escape folded to lower case.
const foldEscapes = (text: string): string =>
  text.replace(/%[0-9A-Fa-f]{2}/g, (escape) => escape.toLowerCase());

const occurrences = (text: string, form: string, found: Array<Interval>): void => {
  for (let at = text.indexOf(form); at !== -1; at = text.indexOf(form, at + 1)) found.push([at, at + form.length]);
};

// The fixed-cost gate before any credential hash (decision row 73): only a
// decoded candidate with a credential prefix, the exact shape and a matching
// checksum is hashed and compared with the stored hashes.
const isStoredCredential = (candidate: string, tokenHashes: ReadonlySet<string>): boolean =>
  hasValidChecksum(candidate) && tokenHashes.has(sha256Hex(candidate));

// Stored agent tokens, raw or encoded. Every
// candidate is bounded: it starts at a prefix occurrence and has the fixed
// length of its encoding.
const findCredentials = (
  text: string, hexFolded: string, tokenHashes: ReadonlySet<string>, found: Array<Interval>,
): void => {
  // Raw. Every `shp_` is tried, so a decoy prefix cannot hide a real token.
  for (let at = text.indexOf("shp_"); at !== -1; at = text.indexOf("shp_", at + 1)) {
    if (isStoredCredential(text.slice(at, at + credentialLength), tokenHashes)) found.push([at, at + credentialLength]);
  }
  // Hexadecimal, in any case.
  for (let at = hexFolded.indexOf(hexPrefix); at !== -1; at = hexFolded.indexOf(hexPrefix, at + 1)) {
    const digits = hexFolded.slice(at, at + 2 * credentialLength);
    if (digits.length !== 2 * credentialLength || !/^[0-9a-f]+$/.test(digits)) continue;
    if (isStoredCredential(Buffer.from(digits, "hex").toString("latin1"), tokenHashes)) {
      found.push([at, at + 2 * credentialLength]);
    }
  }
  // Base64 and base64url at each alignment. The run marks a prefix that
  // starts `alignment` bytes into a three-byte group; the candidate is the
  // group-aligned text that covers the whole credential.
  for (const { run, alignment, alphabet } of base64Probes) {
    const lead = Math.ceil((8 * alignment) / 6);
    const length = Math.ceil((8 * (alignment + credentialLength)) / 6);
    for (let at = text.indexOf(run); at !== -1; at = text.indexOf(run, at + 1)) {
      const groupStart = at - lead;
      if (groupStart < 0) continue;
      const encoded = text.slice(groupStart, groupStart + length);
      if (encoded.length !== length || !alphabet.test(encoded)) continue;
      const decoded = Buffer.from(encoded, "base64").subarray(alignment, alignment + credentialLength);
      if (!isStoredCredential(decoded.toString("latin1"), tokenHashes)) continue;
      let end = groupStart + length;
      while (end < text.length && end < groupStart + length + 2 && text[end] === "=") end++;
      found.push([groupStart + Math.floor((8 * alignment) / 6), end]);
    }
  }
  // URL-encoded. A token needs no escaping, so this finds credentials with
  // some or all characters escaped anyway, in a decoded view that maps each
  // decoded character back to its span in the text.
  if (text.includes("%")) {
    let decoded = "";
    const starts: Array<number> = [];
    const ends: Array<number> = [];
    for (let at = 0; at < text.length;) {
      const escape = text[at] === "%" && /^[0-9A-Fa-f]{2}$/.test(text.slice(at + 1, at + 3));
      const width = escape ? 3 : 1;
      decoded += escape ? String.fromCharCode(Number.parseInt(text.slice(at + 1, at + 3), 16)) : text[at];
      starts.push(at);
      ends.push(at + width);
      at += width;
    }
    for (let at = decoded.indexOf("shp_"); at !== -1; at = decoded.indexOf("shp_", at + 1)) {
      if (isStoredCredential(decoded.slice(at, at + credentialLength), tokenHashes)) {
        found.push([starts[at]!, ends[at + credentialLength - 1]!]);
      }
    }
  }
};

// Every occurrence of every form, merged into disjoint runs in order.
// Overlapping matches merge; adjacent ones stay apart, so a stream of
// repeated secrets releases one marker per secret as it goes.
const findMatches = (text: string, forms: Forms, tokenHashes: ReadonlySet<string>): Array<Interval> => {
  const found: Array<Interval> = [];
  for (const form of forms.exact) occurrences(text, form, found);
  const hexFolded = forms.hex.length > 0 || tokenHashes.size > 0 ? foldHex(text) : text;
  for (const form of forms.hex) occurrences(hexFolded, form, found);
  if (forms.percent.length > 0) {
    const escapesFolded = foldEscapes(text);
    for (const form of forms.percent) occurrences(escapesFolded, form, found);
  }
  if (tokenHashes.size > 0) findCredentials(text, hexFolded, tokenHashes, found);
  found.sort((a, b) => a[0] - b[0]);
  const merged: Array<[number, number]> = [];
  for (const [start, end] of found) {
    const last = merged[merged.length - 1];
    if (last !== undefined && start < last[1]) last[1] = Math.max(last[1], end);
    else merged.push([start, end]);
  }
  return merged;
};

// Renders text[from, to) with each run replaced. A run that starts before
// `from` continues a replacement already released, so it adds no second
// marker. Returns how far the released replacements reach, which can pass
// `to` when a decided run reaches into the text still held back.
const render = (
  text: string, intervals: ReadonlyArray<Interval>, from: number, to: number,
): { readonly output: string; readonly covered: number } => {
  let output = "";
  let at = from;
  for (const [start, end] of intervals) {
    if (end <= at) continue;
    if (start >= to) break;
    if (start > at) output += text.slice(at, start);
    if (start >= from) output += replacement;
    at = end;
  }
  if (at < to) output += text.slice(at, to);
  return { output, covered: Math.max(at, from) };
};

// Preserve each decided span's original length. Message chunk boundaries and
// intervening content use these lengths rather than the shorter or longer
// replacement text, so redaction cannot move ordinary text across an update.
const renderParts = (text: string, intervals: ReadonlyArray<Interval>, from: number, to: number, publicContext = false) => {
  const parts: Array<RedactionPart> = [];
  let at = from;
  for (const [start, end] of intervals) {
    if (end <= at) continue;
    if (start >= to) break;
    if (start > at) parts.push({ length: start - at, text: text.slice(at, start), replacement: false });
    parts.push({ length: end - Math.max(at, start), text: start >= from || publicContext ? replacement : "", replacement: true });
    at = end;
  }
  if (at < to) parts.push({ length: to - at, text: text.slice(at, to), replacement: false });
  return { parts, covered: Math.max(at, from) };
};

// A public credential prefix can be published while staying as match context.
// Every base64 prefix here contains only bits of `shp_`; the last mixed
// character is excluded. No private token character is released this way.
const publicBytes = Buffer.from("shp_", "ascii");
const publicBase64 = [0, 1, 2].map(alignment => base64Run(publicBytes, alignment));
const publicPercent = Array.from({ length: 16 }, (_, mask) => [..."shp_"].map((c, i) =>
  mask & (1 << i) ? `%${c.charCodeAt(0).toString(16)}` : c).join(""));
const publicPrefixes = [...new Set([
  "shp_", hexPrefix, ...publicPercent, ...publicBase64, ...publicBase64.map(toBase64url),
  ...publicBase64.flatMap(inner => [0, 1, 2].map(alignment => base64Run(Buffer.from(inner, "ascii"), alignment))),
])].sort((a, b) => b.length - a.length);
interface Tail { readonly held: number; readonly context: number }
const noTail: Tail = { held: 0, context: 0 };
const largerTail = (a: Tail, b: Tail): Tail => a.held > b.held || a.held === b.held && a.context >= b.context ? a : b;

// A cached failure trie finds the longest undecided suffix in one pass. A
// node is a proper prefix, never a whole matched form. Public token-prefix
// nodes retain context without holding output; explicit secret nodes hold it.
// Limit its Map nodes for large credential leaves. Their fallback stores one
// four-byte failure entry per form character instead of a Map per character.
const prefixMatcher = (forms: ReadonlyArray<string>, publicLengths: ReadonlyMap<string, number>) => {
  if (forms.reduce((sum, form) => sum + form.length, 0) > 32768) {
    const patterns = forms.map(form => {
      const failure = new Uint32Array(form.length);
      for (let i = 1, matched = 0; i < form.length; i++) {
        while (matched > 0 && form[i] !== form[matched]) matched = failure[matched - 1]!;
        if (form[i] === form[matched]) matched++;
        failure[i] = matched;
      }
      return { form, failure, publicLength: publicLengths.get(form) ?? 0 };
    });
    return (text: string): Tail => {
      let tail = noTail;
      for (const { form, failure, publicLength } of patterns) {
        let matched = 0;
        for (let i = 0; i < text.length; i++) {
          while (matched > 0 && text[i] !== form[matched]) matched = failure[matched - 1]!;
          if (text[i] === form[matched]) matched++;
          if (matched === form.length) matched = failure[matched - 1]!;
        }
        tail = largerTail(tail, { held: matched > publicLength ? matched : 0, context: Math.min(matched, publicLength) });
      }
      return tail;
    };
  }
  const nodes = [{ next: new Map<string, number>(), fail: 0, tail: noTail }];
  for (const form of forms) {
    let at = 0;
    const publicLength = publicLengths.get(form) ?? 0;
    for (let length = 1; length < form.length; length++) {
      const c = form[length - 1]!;
      let next = nodes[at]!.next.get(c);
      if (next === undefined) {
        next = nodes.length;
        nodes.push({ next: new Map(), fail: 0, tail: noTail });
        nodes[at]!.next.set(c, next);
      }
      at = next;
      nodes[at]!.tail = largerTail(nodes[at]!.tail, { held: length > publicLength ? length : 0, context: Math.min(length, publicLength) });
    }
  }
  const queue = [...nodes[0]!.next.values()];
  for (let head = 0; head < queue.length; head++) {
    const at = queue[head]!;
    const node = nodes[at]!;
    node.tail = largerTail(node.tail, nodes[node.fail]!.tail);
    for (const [c, next] of node.next) {
      let fail = node.fail;
      while (fail > 0 && !nodes[fail]!.next.has(c)) fail = nodes[fail]!.fail;
      nodes[next]!.fail = nodes[fail]!.next.get(c) ?? 0;
      queue.push(next);
    }
  }
  return (text: string): Tail => {
    let at = 0;
    for (let i = 0; i < text.length; i++) {
      const c = text[i]!;
      while (at > 0 && !nodes[at]!.next.has(c)) at = nodes[at]!.fail;
      at = nodes[at]!.next.get(c) ?? 0;
    }
    return nodes[at]!.tail;
  };
};
const hashPrefixes = [
  { prefix: "shp_", max: credentialLength, alphabet: /^[A-Za-z0-9_-]*$/, fold: false },
  { prefix: hexPrefix, max: 2 * credentialLength, alphabet: /^[0-9a-f]*$/, fold: true },
  ...publicBase64.flatMap((prefix, alignment) => [prefix, toBase64url(prefix)].map(prefix => ({
    prefix, max: Math.ceil(8 * (alignment + credentialLength) / 6) - Math.ceil(8 * alignment / 6) + 2,
    alphabet: /^[A-Za-z0-9_+/=-]*$/, fold: false,
  }))),
];
const hashPublicMatcher = prefixMatcher(publicPrefixes.map(prefix => prefix + "\u0000"), new Map(publicPrefixes.map(prefix => [prefix + "\u0000", prefix.length])));
const hashTail = (text: string): Tail => {
  const suffix = text.slice(-(longestCredentialForm - 1));
  const hex = foldHex(suffix);
  let tail = hashPublicMatcher(suffix);
  for (const spec of hashPrefixes) {
    const view = spec.fold ? hex : suffix;
    const start = view.lastIndexOf(spec.prefix);
    if (start < 0 || view.length - start >= spec.max) continue;
    const rest = view.slice(start + spec.prefix.length);
    if (spec.alphabet.test(rest)) tail = largerTail(tail, { held: rest.length > 0 ? spec.prefix.length + rest.length : 0, context: spec.prefix.length });
  }
  // Mixed literal and percent-escaped public prefixes are public too. Partial
  // escapes after the prefix stay held until their decoded byte is known.
  const percent = foldEscapes(suffix).replace(/%[0-9A-Fa-f]$/, escape => escape.toLowerCase());
  tail = largerTail(tail, hashPublicMatcher(percent));
  for (const prefix of publicPercent) {
    if (!prefix.includes("%")) continue;
    const start = percent.lastIndexOf(prefix);
    if (start < 0) continue;
    const rest = percent.slice(start + prefix.length);
    const decoded = rest.replace(/%[0-9a-f]{2}/g, escape => String.fromCharCode(parseInt(escape.slice(1), 16)));
    if (/^[A-Za-z0-9_-]*(?:%[0-9a-f]?)?$/.test(decoded) && decoded.replace(/%[0-9a-f]?$/, "").length < credentialLength - 4) {
      tail = largerTail(tail, { held: rest.length > 0 ? prefix.length + rest.length : 0, context: prefix.length });
    }
  }
  return tail;
};

// Copy string values, preserving keys, tags and schema-typed identities.
// An explicit secret still wins in every other string, in every encoding.
const redactValue = (
  value: unknown, redactText: (text: string) => string, structure?: RedactionStructure,
): unknown => {
  if (typeof value === "string") return structure?.identity === true ? value : redactText(value);
  if (Array.isArray(value)) return value.map((item) => redactValue(item, redactText, structure?.elements));
  if (typeof value === "object" && value !== null) {
    const copy: Record<string, unknown> = {};
    for (const [key, item] of Object.entries(value)) {
      // Defined, not assigned, so a `__proto__` key stays an ordinary field.
      Object.defineProperty(copy, key, {
        value: redactValue(item, redactText, fieldStructure(structure, key, "_tag" in value ? value._tag : undefined)), enumerable: true, writable: true, configurable: true,
      });
    }
    return copy;
  }
  return value;
};

// What a stream item holds back: its last characters, and how many of them
// at the start are already covered by a released replacement.
interface Carry {
  readonly text: string;
  readonly covered: number;
  readonly context?: string;
}

// The real redactor (task 5a.3). Both registries are read on every call, so a
// secret or a token hash added after construction is redacted at once.
// `layer` provides the `log/Redactor.ts` service that `EventLog.append` runs
// on every event before it is stored or returned (decision row 59).
export const makeRedactor = (registry: {
  readonly secrets: () => ReadonlyArray<Redacted.Redacted<string>>;
  readonly tokenHashes: () => ReadonlySet<string>;
}): {
  readonly layer: Layer.Layer<Redactor>;
  readonly redactText: (text: string) => string;
  readonly redactParts: (itemId: string, text: string, final: boolean) => ReadonlyArray<RedactionPart>;
  readonly redactChunk: (itemId: string, text: string, final: boolean) => string;
  readonly withholdParts: (itemId: string, marker: string) => ReadonlyArray<RedactionPart>;
} => {
  const compiled: Array<{ values: ReadonlyArray<string>; forms: Forms; tail: (text: string) => Tail }> = [];
  const currentForms = (additional: ReadonlyArray<Redacted.Redacted<string>> = []) => {
    // Registries may return fresh arrays and Redacted wrappers. Compare their
    // values, not wrapper identities, and keep only four compiled states.
    const values = [...registry.secrets(), ...additional].map(Redacted.value);
    const cached = compiled.find(state => state.values.length === values.length && state.values.every((value, i) => value === values[i]));
    if (cached !== undefined) return cached;
    const exact = new Set<string>(), hex = new Set<string>(), percent = new Set<string>();
    const publicLengths = new Map<string, number>();
    for (const value of values) {
      const localExact = new Set<string>(), localHex = new Set<string>(), localPercent = new Set<string>();
      addForms(value, localExact, localHex, localPercent);
      const credential = hasValidChecksum(value);
      for (const [local, all] of [[localExact, exact], [localHex, hex], [localPercent, percent]] as const) for (const form of local) {
        all.add(form);
        const length = credential ? publicPrefixes.find(prefix => form.startsWith(prefix))?.length ?? 0 : 0;
        publicLengths.set(form, Math.min(publicLengths.get(form) ?? length, length));
      }
    }
    const forms = { exact: [...exact], hex: [...hex], percent: [...percent], publicLengths };
    const matchExact = prefixMatcher(forms.exact, publicLengths), matchHex = prefixMatcher(forms.hex, publicLengths), matchPercent = prefixMatcher(forms.percent, publicLengths);
    const tail = (text: string): Tail => largerTail(matchExact(text), largerTail(matchHex(foldHex(text)), matchPercent(foldEscapes(text).replace(/%[0-9A-Fa-f]$/, escape => escape.toLowerCase()))));
    const state = { values, forms, tail };
    compiled.unshift(state);
    if (compiled.length > 4) compiled.pop();
    return state;
  };

  const redactWith = (forms: Forms, tokenHashes: ReadonlySet<string>) => (text: string): string => {
    const intervals = findMatches(text, forms, tokenHashes);
    return intervals.length === 0 ? text : render(text, intervals, 0, text.length).output;
  };
  const redactText = (text: string): string => redactWith(currentForms().forms, registry.tokenHashes())(text);

  // Streamed text is joined per item (decision row 57). Each call releases
  // text except a suffix that could continue a registered form, bounded by
  // (window - 1) characters. Safe delimiters release a complete ordinary
  // chunk. A decided replacement is released at once
  // even when it reaches into the held-back text; the carry then records how
  // much of it is already covered. The carry never exceeds (window - 1)
  // characters, however many matches arrive (fix round 1, ruling 5).
  const carries = new Map<string, Carry>();
  const redactParts = (itemId: string, text: string, final: boolean, additional: ReadonlyArray<Redacted.Redacted<string>> = []): ReadonlyArray<RedactionPart> => {
    const carry = carries.get(itemId) ?? { text: "", covered: 0 };
    const context = carry.context ?? "";
    const joined = context + carry.text + text;
    const state = currentForms(additional);
    const forms = state.forms;
    const from = context.length + carry.covered;
    const publicContext = context.length > 0 && carry.covered === 0;
    const tokenHashes = registry.tokenHashes();
    const intervals = findMatches(joined, forms, tokenHashes);
    if (final) {
      carries.delete(itemId);
      return renderParts(joined, intervals, from, joined.length, publicContext).parts;
    }
    const tail = tokenHashes.size > 0 ? largerTail(state.tail(joined), hashTail(joined)) : state.tail(joined);
    // Keep a candidate whole once it contains private characters. If an
    // earlier chunk published public prefix characters, never hold them again.
    const cut = Math.max(context.length, joined.length - tail.held);
    const released = cut > from
      ? renderParts(joined, intervals, from, cut, publicContext)
      : { parts: [], covered: from };
    // Published public bytes are context only. They neither own a pending
    // event span nor require a second marker when the private remainder ends.
    // A base64 decoder also needs up to three already-published characters
    // before its public-prefix probe to reconstruct a byte-aligned group.
    // This context never delays output or contributes a pending event span.
    const contextLength = tail.context + (tokenHashes.size > 0 ? 3 : 0);
    const retainedContext = joined.slice(Math.max(0, cut - contextLength), cut);
    carries.set(itemId, { text: joined.slice(cut), covered: Math.max(0, released.covered - cut), context: retainedContext });
    return released.parts;
  };

  const redactChunk = (itemId: string, text: string, final: boolean, additional: ReadonlyArray<Redacted.Redacted<string>> = []): string =>
    redactParts(itemId, text, final, additional).map(part => part.text).join("");
  const withholdParts = (itemId: string, marker: string): ReadonlyArray<RedactionPart> => {
    const carry = carries.get(itemId);
    if (carry === undefined || carry.covered >= carry.text.length) return [];
    const length = carry.text.length - carry.covered;
    carries.set(itemId, { ...carry, covered: carry.text.length });
    return [{ length, text: marker, replacement: true }];
  };
  const layer = Layer.succeed(Redactor, {
    withholdParts: (itemId: string, marker: string) => Effect.sync(() => withholdParts(itemId, marker)),
    redactPartsWithSecrets: (itemId: string, text: string, final: boolean, additional: ReadonlyArray<Redacted.Redacted<string>>) =>
      Effect.sync(() => redactParts(itemId, text, final, additional)),
    redactChunkWithSecrets: (itemId: string, text: string, final: boolean, additional: ReadonlyArray<Redacted.Redacted<string>>) =>
      Effect.sync(() => redactChunk(itemId, text, final, additional)),
    redactChunk: (itemId: string, text: string, final: boolean) => Effect.sync(() => redactChunk(itemId, text, final)),
    redactText: (text: string) => Effect.sync(() => redactText(text)),
    redact: (event: Event) => Effect.sync(() =>
      redactValue(event, redactWith(currentForms().forms, registry.tokenHashes()), eventStructure) as Event),
  });

  return { layer, redactText, redactChunk, redactParts, withholdParts };
};
