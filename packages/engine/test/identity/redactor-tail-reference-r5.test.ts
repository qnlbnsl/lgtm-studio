import { expect, it } from "vitest";
import { compileForms, maximumIndexFormCharacters, type FormLists } from "../../src/identity/redactionForms.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";
registerRebuildCheck({ noLog: true });

// Independent exhaustive suffix checks, including the linear path's one-byte
// prefixes and prefixes longer than a typical streamed chunk.
for (const strategy of ["indexed", "linear"] as const) it(`r5 tail matches a naive proper-prefix reference: ${strategy}`, () => {
  const forms: FormLists = {
    exact: ["e-s19-fake-private", "s19-fake-" + "q".repeat(360), "shp_" + "b".repeat(58), "ababababababac", "AbCd-fake", "密钥🔑fake", "\ud800fake",
      ...(strategy === "linear" ? ["budget-" + "z".repeat(maximumIndexFormCharacters)] : [])],
    hex: ["aabbccddeeff0123456789"], percent: ["%41%4a-s19-fake", "a-s19-percent", "%f0%9f%94%91fake"],
  };
  const publicLengths = new Map([[forms.exact[2]!, 4]]), matcher = compileForms(forms, publicLengths);
  expect(matcher.strategy).toBe(strategy);
  const all = (["exact", "hex", "percent"] as const).flatMap(kind => forms[kind].map(form => ({ kind, form })));
  const check = (text: string) => {
    const views = { exact: text, hex: text.replace(/[A-F]/g, c => c.toLowerCase()), percent: text.replace(/%[0-9a-f]{2}/gi, e => e.toLowerCase()).replace(/%[0-9a-f]$/i, e => e.toLowerCase()) };
    let expected = 0;
    for (const { kind, form } of all) for (let length = Math.min(form.length - 1, text.length); length > expected; length--) {
      if (length > (publicLengths.get(form) ?? 0) && views[kind].endsWith(form.slice(0, length))) { expected = length; break; }
    }
    expect(matcher.tail(text).held, `strategy=${strategy}, input length=${text.length}, required hold=${expected}`).toBe(expected);
  };
  for (const { kind, form } of all) {
    const cuts = [...new Set([0, 1, 2, 3, 4, 5, 7, 11, 199, 200, 201, 240, Math.min(500, form.length - 1), form.length <= 500 ? form.length - 1 : 500])];
    for (const length of cuts.filter(length => length < form.length)) {
      const prefix = form.slice(0, length);
      for (const before of ["", "ordinary. ", "x".repeat(300) + ". "]) {
        check(before + prefix);
        if (kind === "hex") check(before + prefix.toUpperCase());
        if (kind === "percent") check(before + prefix.replace(/%[0-9a-f]{1,2}/g, e => e.toUpperCase()));
      }
    }
  }
  for (const text of ["%4A", "%4", "%F", "safe.", "e", "shp", "shp_", "shp_b", "AbC", "abababa"]) check(text);
});

it("r5 exact-only candidate matching folds its own index query", () => {
  const matcher = compileForms({ exact: ["S19-AbC-private"], hex: [], percent: [] }), found: Array<readonly [number, number]> = [];
  matcher.match("safe S19-AbC-private.", found);
  expect(found).toEqual([[5, 20]]);
});
