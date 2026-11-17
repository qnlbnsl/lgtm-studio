// Encoded forms shorter than this are not matched on their own. Only the
// partial base64 runs of a very short secret are that short, and matching
// them would blank unrelated text. The full forms are always matched.
const minimumPartialLength = 8;

// The characters of base64 output that depend only on `bytes` when they start
// `alignment` bytes into a longer encoded value (decision row 73: all three
// alignments). The leading and trailing characters that mix in neighbouring
// bytes are left out.
export const base64Run = (bytes: Buffer, alignment: number): string => {
  const padded = Buffer.concat([Buffer.alloc(alignment), bytes, Buffer.alloc(2)]);
  const first = Math.ceil((8 * alignment) / 6);
  const end = Math.floor((8 * (alignment + bytes.byteLength)) / 6);
  return padded.toString("base64").slice(first, end);
};

export const toBase64url = (base64: string): string => base64.replaceAll("+", "-").replaceAll("/", "_");

// Every form of one secret (decision rows 57 and 73, fix round 1 rulings 1
// and 2): raw; as it appears inside a JSON string; base64 and base64url,
// whole and at each byte alignment inside a longer value; URL-encoded, with
// `%20` or `+` for a space; and hexadecimal.
export const addForms = (secret: string, exact: Set<string>, hex: Set<string>, percent: Set<string>): void => {
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
  const url = encodeURIComponent(secret.toWellFormed()).replace(/%[0-9A-F]{2}/g, (escape) => escape.toLowerCase());
  percent.add(url);
  percent.add(url.replaceAll("%20", "+"));
  hex.add(bytes.toString("hex"));
};

// A single candidate index serves both complete matches and proper-prefix
// suffixes. Only four head characters enter the index. Beyond either budget
// the same literal checks use native indexOf loops without an allocated trie.
export const maximumIndexFormCharacters = 262144;
export const maximumIndexHeadCharacters = 32768;
export const maximumCachedFormCharacters = 1048576;
const headLength = 4;
const kinds = ["exact", "hex", "percent"] as const;
type Kind = typeof kinds[number];
export interface FormLists {
  readonly exact: ReadonlyArray<string>;
  readonly hex: ReadonlyArray<string>;
  readonly percent: ReadonlyArray<string>;
}
export interface Tail { readonly held: number }
const emptyTail: Tail = { held: 0 };
export const foldHex = (text: string) => text.replace(/[A-F]/g, c => c.toLowerCase());
export const foldEscapes = (text: string) => text.replace(/%[0-9A-Fa-f]{2}/g, e => e.toLowerCase());
interface Pattern { readonly form: string; readonly kind: Kind; readonly publicLength: number }
interface Node { readonly children: Map<string, Node>; terminal: boolean }

export const compileForms = (forms: FormLists, publicLengths: ReadonlyMap<string, number> = new Map()) => {
  const patterns: Array<Pattern> = [];
  let formCharacters = 0, headCharacters = 0, longest = 0;
  for (const kind of kinds) for (const form of forms[kind]) {
    if (form.length === 0) continue;
    patterns.push({ kind, form, publicLength: publicLengths.get(form) ?? 0 });
    formCharacters += form.length;
    headCharacters += Math.min(headLength, form.length);
    longest = Math.max(longest, form.length - 1);
  }
  let indexed = formCharacters <= maximumIndexFormCharacters && headCharacters <= maximumIndexHeadCharacters;
  const buckets = new Map<string, Array<Pattern>>();
  const short = new Map<string, number>();
  let candidates: RegExp | undefined, candidateSource: string | undefined;
  if (indexed && patterns.length > 0) {
    const root: Node = { children: new Map(), terminal: false };
    for (const pattern of patterns) {
      const { form, kind, publicLength } = pattern;
      const key = foldHex(form.slice(0, headLength)), bucket = buckets.get(key) ?? [];
      bucket.push(pattern); buckets.set(key, bucket);
      let node = root;
      for (let i = 0; i < key.length; i++) {
        const c = key[i]!;
        let next = node.children.get(c);
        if (next === undefined) { next = { children: new Map(), terminal: false }; node.children.set(c, next); }
        node = next;
      }
      node.terminal = true;
      for (let length = 1; length < Math.min(headLength, form.length); length++) {
        const key = kind + form.slice(0, length), held = length > publicLength ? length : 0;
        short.set(key, Math.max(short.get(key) ?? 0, held));
      }
    }
    const expression = (node: Node): string => {
      if (node.terminal) return "";
      const branches = Array.from(node.children, ([c, child]) => c.replace(/[.*+?^${}()|[\]\\]/g, "\\$&") + expression(child));
      return branches.length === 1 ? branches[0]! : `(?:${branches.join("|")})`;
    };
    const source = expression(root);
    if (source.length <= maximumIndexHeadCharacters) candidateSource = source;
    else { indexed = false; buckets.clear(); short.clear(); }
  }
  const headLengths = [...new Set([...buckets.keys()].map(key => key.length))];
  const views = (text: string, hex = foldHex(text)) => ({ exact: text, hex, percent: forms.percent.length > 0 ? foldEscapes(text) : text });
  const visit = (query: string, callback: (start: number, pattern: Pattern) => void, from = 0) => {
    if (buckets.size === 0) return;
    // A short message uses direct head lookups. Native expression compilation
    // is deferred until a long query needs it, rather than blocking first use.
    if (query.length - from <= 256) {
      for (let start = from; start < query.length; start++) {
        for (const length of headLengths) {
          if (length > query.length - start) continue;
          for (const pattern of buckets.get(query.slice(start, start + length)) ?? []) callback(start, pattern);
        }
      }
      return;
    }
    candidates ??= new RegExp(candidateSource!, "g");
    candidates.lastIndex = from;
    for (let hit = candidates.exec(query); hit !== null; hit = candidates.exec(query)) {
      const start = hit.index;
      // A short terminal head can shadow descendants in the expression.
      // Check every bucket beginning there, including longer shared heads.
      for (let length = hit[0].length; length <= Math.min(headLength, query.length - start); length++) {
        for (const pattern of buckets.get(query.slice(start, start + length)) ?? []) callback(start, pattern);
      }
      candidates.lastIndex = start + 1;
    }
  };
  const match = (text: string, found: Array<readonly [number, number]>): string => {
    const view = views(text), hex = view.hex;
    if (indexed) visit(hex, (start, { kind, form }) => {
      if (view[kind].startsWith(form, start)) found.push([start, start + form.length]);
    });
    else for (const { kind, form } of patterns) {
      for (let start = view[kind].indexOf(form); start !== -1; start = view[kind].indexOf(form, start + 1)) found.push([start, start + form.length]);
    }
    return hex;
  };
  const tail = (text: string): Tail => {
    if (longest === 0) return emptyTail;
    // Two leading context characters allow normalization of an escape whose
    // last digit lies just inside the suffix. They are never held themselves.
    const suffix = text.slice(-(longest + 2)), from = Math.max(0, suffix.length - longest);
    const hex = foldHex(suffix), view = views(suffix, hex);
    view.percent = view.percent.replace(/%[0-9A-Fa-f]$/, e => e.toLowerCase());
    let result = emptyTail;
    const take = (length: number, publicLength: number) => {
      const held = length > publicLength ? length : 0;
      if (held > result.held) result = { held };
    };
    if (indexed) {
      for (const kind of kinds) for (let length = 1; length <= Math.min(headLength - 1, longest, suffix.length); length++) {
        const candidate = short.get(kind + view[kind].slice(-length));
        if (candidate !== undefined && candidate > 0) take(candidate, 0);
      }
      visit(hex, (start, { kind, form, publicLength }) => {
        const length = suffix.length - start;
        if (length < form.length && form.startsWith(view[kind].slice(start))) take(length, publicLength);
      }, from);
    } else for (const { kind, form, publicLength } of patterns) {
      const value = view[kind], head = form.slice(0, headLength);
      for (let length = 1; length < Math.min(headLength, form.length, value.length + 1); length++) {
        if (form.startsWith(value.slice(-length))) take(length, publicLength);
      }
      for (let start = value.indexOf(head, Math.max(from, value.length - form.length + 1)); start !== -1; start = value.indexOf(head, start + 1)) {
        const length = value.length - start;
        if (length < form.length && form.startsWith(value.slice(start))) take(length, publicLength);
      }
    }
    return result;
  };
  return { match, tail, strategy: indexed ? "indexed" as const : "linear" as const, formCharacters, headCharacters };
};
