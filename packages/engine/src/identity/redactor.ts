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
  // below cover shell newlines and surrounding documents with the same gate.
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
const renderParts = (text: string, intervals: ReadonlyArray<Interval>, from: number, to: number) => {
  const parts: Array<RedactionPart> = [];
  let at = from;
  for (const [start, end] of intervals) {
    if (end <= at) continue;
    if (start >= to) break;
    if (start > at) parts.push({ length: start - at, text: text.slice(at, start), replacement: false });
    parts.push({ length: end - Math.max(at, start), text: start >= from ? replacement : "", replacement: true });
    at = end;
  }
  if (at < to) parts.push({ length: to - at, text: text.slice(at, to), replacement: false });
  return { parts, covered: Math.max(at, from) };
};

// Longest proper prefix of a form that is also a suffix of the current text.
// The prefix table keeps the work linear even for a large repetitive secret.
const prefixTail = (text: string, form: string): number => {
  const limit = Math.min(text.length, form.length - 1);
  if (limit <= 0) return 0;
  const failure = new Uint32Array(limit);
  for (let at = 1, matched = 0; at < limit; at++) {
    while (matched > 0 && form[at] !== form[matched]) matched = failure[matched - 1]!;
    if (form[at] === form[matched]) matched++;
    failure[at] = matched;
  }
  let matched = 0;
  for (let at = text.length - limit; at < text.length; at++) {
    while (matched > 0 && (matched === limit || text[at] !== form[matched])) matched = failure[matched - 1]!;
    if (text[at] === form[matched]) matched++;
  }
  return matched;
};

const undecidedTail = (text: string, forms: Forms, tokenHashes: ReadonlySet<string>): number => {
  // Every supported hash-only credential form consists entirely of these
  // characters. A final delimiter outside that alphabet ends every possible
  // candidate. Otherwise retain its contiguous suffix, capped at the hash-probe window.
  const credentialTail = tokenHashes.size > 0
    ? /[A-Za-z0-9_+\/=%-]+$/.exec(text.slice(-(longestCredentialForm - 1)))?.[0].length ?? 0 : 0;
  let held = credentialTail;
  for (const form of forms.exact) held = Math.max(held, prefixTail(text, form));
  if (forms.hex.length > 0) {
    const folded = foldHex(text);
    for (const form of forms.hex) held = Math.max(held, prefixTail(folded, form));
  }
  if (forms.percent.length > 0) {
    // A chunk may end inside an escape; fold its first hexadecimal digit too.
    const folded = foldEscapes(text).replace(/%[0-9A-Fa-f]$/, escape => escape.toLowerCase());
    for (const form of forms.percent) held = Math.max(held, prefixTail(folded, form));
  }
  return held;
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
} => {
  const currentForms = (additional: ReadonlyArray<Redacted.Redacted<string>> = []): Forms => {
    const exact = new Set<string>();
    const hex = new Set<string>();
    const percent = new Set<string>();
    for (const secret of [...registry.secrets(), ...additional]) addForms(Redacted.value(secret), exact, hex, percent);
    return { exact: [...exact], hex: [...hex], percent: [...percent] };
  };

  const redactWith = (forms: Forms, tokenHashes: ReadonlySet<string>) => (text: string): string => {
    const intervals = findMatches(text, forms, tokenHashes);
    return intervals.length === 0 ? text : render(text, intervals, 0, text.length).output;
  };
  const redactText = (text: string): string => redactWith(currentForms(), registry.tokenHashes())(text);

  // Streamed text is joined per item (decision row 57). Each call releases
  // text except a suffix that could continue a registered form, bounded by
  // (window - 1) characters. Safe delimiters release a complete ordinary
  // chunk, preserving consumers that read a complete report from one chunk. A decided replacement is released at once
  // even when it reaches into the held-back text; the carry then records how
  // much of it is already covered. The carry never exceeds (window - 1)
  // characters, however many matches arrive (fix round 1, ruling 5).
  const carries = new Map<string, Carry>();
  const redactParts = (itemId: string, text: string, final: boolean, additional: ReadonlyArray<Redacted.Redacted<string>> = []): ReadonlyArray<RedactionPart> => {
    const carry = carries.get(itemId) ?? { text: "", covered: 0 };
    const joined = carry.text + text;
    const forms = currentForms(additional);
    const tokenHashes = registry.tokenHashes();
    const intervals = findMatches(joined, forms, tokenHashes);
    if (final) {
      carries.delete(itemId);
      return renderParts(joined, intervals, carry.covered, joined.length).parts;
    }
    const window = Math.max(
      tokenHashes.size > 0 ? longestCredentialForm : 1,
      ...[...forms.exact, ...forms.hex, ...forms.percent].map((form) => form.length),
    );
    const held = undecidedTail(joined, forms, tokenHashes);
    // Retain an undecided short chunk whole, so its event boundary survives.
    // A large chunk still releases all but at most one bounded window.
    const cut = held > 0 && joined.length < window ? 0 : Math.max(0, joined.length - held);
    const released = cut > carry.covered
      ? renderParts(joined, intervals, carry.covered, cut)
      : { parts: [], covered: carry.covered };
    carries.set(itemId, { text: joined.slice(cut), covered: Math.max(0, released.covered - cut) });
    return released.parts;
  };

  const redactChunk = (itemId: string, text: string, final: boolean, additional: ReadonlyArray<Redacted.Redacted<string>> = []): string =>
    redactParts(itemId, text, final, additional).map(part => part.text).join("");
  const layer = Layer.succeed(Redactor, {
    redactPartsWithSecrets: (itemId: string, text: string, final: boolean, additional: ReadonlyArray<Redacted.Redacted<string>>) =>
      Effect.sync(() => redactParts(itemId, text, final, additional)),
    redactChunkWithSecrets: (itemId: string, text: string, final: boolean, additional: ReadonlyArray<Redacted.Redacted<string>>) =>
      Effect.sync(() => redactChunk(itemId, text, final, additional)),
    redactChunk: (itemId: string, text: string, final: boolean) => Effect.sync(() => redactChunk(itemId, text, final)),
    redactText: (text: string) => Effect.sync(() => redactText(text)),
    redact: (event: Event) => Effect.sync(() =>
      redactValue(event, redactWith(currentForms(), registry.tokenHashes()), eventStructure) as Event),
  });

  return { layer, redactText, redactChunk, redactParts };
};
