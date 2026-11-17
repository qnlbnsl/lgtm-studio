import type { Event } from "@lgtm/contracts";
import * as Effect from "effect/Effect";
import * as Layer from "effect/Layer";
import * as Redacted from "effect/Redacted";

import { Redactor, type RedactionPart } from "../log/Redactor.ts";
import { eventStructure, fieldStructure, type RedactionStructure } from "../log/redactionStructure.ts";
import { credentialLength, hasValidChecksum, sha256Hex } from "./credential.ts";
import { addForms, base64Run, compileForms, toBase64url, maximumCachedFormCharacters, type FormLists, type Tail, foldHex, foldEscapes } from "./redactionForms.ts";

const replacement = "[REDACTED]";

const longestCredentialForm = 3 * credentialLength;
interface Forms extends FormLists {
  readonly publicLengths: ReadonlyMap<string, number>;
  readonly match: (text: string, found: Array<Interval>) => string;
}

// Where an encoded credential can start (fix round 1, ruling 3): the
// base64 and base64url runs of each six-byte prefix at each alignment, and
// the hexadecimal of the shared `shp_` prefix.
interface Base64Probe {
  readonly run: string;
  readonly alignment: number;
  readonly alphabet: RegExp;
}
const base64Probes: ReadonlyArray<Base64Probe> = ["shp_a_", "shp_w_"].flatMap((prefix) =>
  [0, 1, 2].flatMap((alignment) => {
    const run = base64Run(Buffer.from(prefix, "latin1"), alignment);
    return [
      { run, alignment, alphabet: /^[A-Za-z0-9+/]*$/ },
      { run: toBase64url(run), alignment, alphabet: /^[A-Za-z0-9_-]*$/ },
    ];
  }));
const hexPrefix = Buffer.from("shp_", "latin1").toString("hex");

type Interval = readonly [start: number, end: number];

// The fixed-cost gate before any credential hash (decision row 73): only a
// decoded candidate with a credential prefix, the exact shape and a matching
// checksum is hashed and compared with the stored hashes.
const isStoredCredential = (candidate: string, tokenHashes: ReadonlySet<string>): boolean =>
  hasValidChecksum(candidate) && tokenHashes.has(sha256Hex(candidate));

// Stored agent and web session tokens, raw or encoded. Every
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
  const hexFolded = forms.match(text, found);
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
const publishedContextLength = publicPrefixes.reduce((longest, prefix) => Math.max(longest, prefix.length), 0) + 3;
const noTail: Tail = { held: 0 };
const largerTail = (a: Tail, b: Tail): Tail => a.held >= b.held ? a : b;

const hashPrefixes = [
  { prefix: "shp_", max: credentialLength, alphabet: /^[A-Za-z0-9_-]*$/, fold: false },
  { prefix: hexPrefix, max: 2 * credentialLength, alphabet: /^[0-9a-f]*$/, fold: true },
  ...publicBase64.flatMap((prefix, alignment) => [prefix, toBase64url(prefix)].map(prefix => ({
    prefix, max: Math.ceil(8 * (alignment + credentialLength) / 6) - Math.ceil(8 * alignment / 6) + 2,
    alphabet: /^[A-Za-z0-9_+/=-]*$/, fold: false,
  }))),
];
const hashTail = (text: string): Tail => {
  const suffix = text.slice(-(longestCredentialForm - 1));
  const hex = foldHex(suffix);
  let tail = noTail;
  for (const spec of hashPrefixes) {
    const view = spec.fold ? hex : suffix;
    for (let start = view.indexOf(spec.prefix); start !== -1; start = view.indexOf(spec.prefix, start + 1)) {
      if (view.length - start >= spec.max) continue;
      const rest = view.slice(start + spec.prefix.length);
      if (spec.alphabet.test(rest)) tail = largerTail(tail, { held: rest.length > 0 ? spec.prefix.length + rest.length : 0 });
    }
  }
  // Mixed literal and percent-escaped public prefixes are public too. Partial
  // escapes after the prefix stay held until their decoded byte is known.
  const percent = foldEscapes(suffix).replace(/%[0-9A-Fa-f]$/, escape => escape.toLowerCase());
  for (const prefix of publicPercent) {
    for (let start = percent.indexOf(prefix); start !== -1; start = percent.indexOf(prefix, start + 1)) {
      const rest = percent.slice(start + prefix.length);
      const decoded = rest.replace(/%[0-9a-f]{2}/g, escape => String.fromCharCode(parseInt(escape.slice(1), 16)));
      if (/^[A-Za-z0-9_-]*(?:%[0-9a-f]?)?$/.test(decoded) && decoded.replace(/%[0-9a-f]?$/, "").length < credentialLength - 4) {
        tail = largerTail(tail, { held: rest.length > 0 ? prefix.length + rest.length : 0 });
      }
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
  const compiled: Array<{ values: ReadonlyArray<string>; forms: Forms; tail: (text: string) => Tail; characters: number }> = [];
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
    const lists = { exact: [...exact], hex: [...hex], percent: [...percent] };
    const matcher = compileForms(lists, publicLengths);
    const forms = { ...lists, publicLengths, match: matcher.match };
    const tail = matcher.tail;
    const state = { values, forms, tail, characters: matcher.formCharacters };
    // Always retain the newest state, including one larger than the budget.
    // The budget evicts older states; it must not force every subsequent
    // chunk to regenerate the current list. The index has its own cap.
    compiled.unshift(state);
    while (compiled.length > 1 && (compiled.length > 4 || compiled.reduce((sum, state) => sum + state.characters, 0) > maximumCachedFormCharacters)) compiled.pop();
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
    // Even with no ordinary span to release, a completed overlapping match
    // can extend the previous replacement. Keep its entire covered reach.
    const released = renderParts(joined, intervals, from, Math.max(from, cut), publicContext);
    // Published public bytes are context only. They neither own a pending
    // event span nor require a second marker when the private remainder ends.
    // A base64 decoder also needs up to three already-published characters
    // before its public-prefix probe to reconstruct a byte-aligned group.
    // This context never delays output or contributes a pending event span.
    // Keep this independent of the selected private suffix: another secret
    // may start inside the public prefix. Its hold must not discard context.
    const contextLength = publishedContextLength;
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
