import * as Redacted from "effect/Redacted";
import { expect, it } from "vitest";
import { makeRedactor } from "../../src/identity/redactor.ts";
import { sha256Hex } from "../../src/identity/credential.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";
registerRebuildCheck({ noLog: true });

const tokenOf = (payload: Buffer) => `shp_a_${payload.toString("base64url")}_${sha256Hex(payload).slice(0, 8)}`;
// Exact planted reviewer token. Its payload contains a second public prefix.
const nested = tokenOf(Buffer.from("AAAAAAAAAAAAAAAAAAAAshp_BBBBBBBBBBBBBBBBBBB", "base64url"));
const create = (values: readonly string[], tokens: readonly string[] = []) => makeRedactor({
  secrets: () => values.map(value => Redacted.make(value)), tokenHashes: () => new Set(tokens.map(sha256Hex)),
});
for (const byValue of [false, true]) for (const spelling of ["raw", "hex", "one-escape"] as const) {
  it(`r4 B1 checks every prefix occurrence: ${spelling}, byValue=${byValue}`, () => {
    const encoded = spelling === "hex" ? Buffer.from(nested).toString("hex") : spelling === "one-escape"
      ? nested.slice(0, 30) + "%42" + nested.slice(31) : nested;
    const text = "t=" + encoded + " ok", publicEnd = spelling === "hex" ? 8 : 4;
    let unsafe = 0;
    const r = create(byValue ? [nested] : [], [nested]);
    const run = (cuts: readonly number[]) => {
      let previous = 0, offset = 0, bad = false;
      const consume = (parts: ReturnType<typeof r.redactParts>) => {
        for (const part of parts) {
          if (!part.replacement) {
            if (part.text !== text.slice(offset, offset + part.length)) bad = true;
            for (let i = offset; i < offset + part.length; i++) if (i >= 2 + publicEnd && i < 2 + encoded.length) bad = true;
          }
          offset += part.length;
        }
      };
      for (const end of [...cuts, text.length]) { consume(r.redactParts("nested", text.slice(previous, end), false)); previous = end; }
      consume(r.redactParts("nested", "", true));
      if (bad || offset !== text.length) unsafe++;
    };
    for (const size of [1, 2, 3, 7]) run(Array.from({ length: Math.floor((text.length - 1) / size) }, (_, i) => (i + 1) * size));
    for (let cut = 0; cut <= text.length; cut++) run([cut]);
    expect(unsafe, "unsafe chunkings of the planted nested-prefix token").toBe(0);
  });
}
const token = tokenOf(Buffer.alloc(32, 7));
const value = "s19-fake-login-value-0123456789-abcdefghijklmnop-XY";
const first = "s19-fake-AAAA-0123456789abcdefghij";
const cases = [
  { name: "hash token after withhold", values: ["e-s19-fake-value-secret"], tokens: [token], calls: ["see " + token.slice(0, 57), null, token.slice(57), " ok"], expected: "see [W] ok" },
  { name: "value after withhold", values: [value, "p-XY-s19-fake-second"], tokens: [], calls: ["key=" + value.slice(0, -4), null, "p-XY", "."], expected: "key=[W]." },
  { name: "overlap without withhold", values: [first, "0123456789abcdefghij-KLMN", "-KLMN-s19-fake-third"], tokens: [], calls: ["pre " + first, "-KLMN", " post"], expected: "pre [REDACTED] post" },
  { name: "two-character value after withhold", values: [value, "XY-s19-fake-second-secret"], tokens: [], calls: ["key=" + value.slice(0, -2), null, "XY", "."], expected: "key=[W]." },
];
for (const fixture of cases) it(`r4 B2 preserves completed coverage: ${fixture.name}`, () => {
  const r = create(fixture.values, fixture.tokens);
  const parts = fixture.calls.flatMap(text => text === null ? r.withholdParts("coverage", "[W]") : r.redactParts("coverage", text, false));
  parts.push(...r.redactParts("coverage", "", true));
  expect(parts.map(p => p.text).join("") === fixture.expected, "completed private match must not lose its covered tail").toBe(true);
  expect(parts.reduce((n, p) => n + p.length, 0)).toBe(fixture.calls.reduce((n, text) => n + (text?.length ?? 0), 0));
});
it("r4 percent-escape normalization is independent of the secret-set size", () => {
  for (const count of [0, 20, 300]) {
    const r = create(["a", ...Array.from({ length: count }, (_, i) => `s19-fake-irrelevant-${i}-not-a-real-secret`)]);
    expect(r.redactText("%4A") === "%4[REDACTED]", `extra values=${count}`).toBe(true);
  }
});
it("r4 lone surrogate values retain their raw and JSON protection", () => {
  for (const value of ["s19-fake-\ud800", "s19-fake-\udc00", "s19-fake-\ud800X\udc00"]) {
    const r = create([value]);
    for (const text of [value, JSON.stringify(value).slice(1, -1), encodeURIComponent(value.toWellFormed())]) {
      expect(r.redactText(text) === "[REDACTED]").toBe(true);
      for (let cut = 0; cut <= text.length; cut++) {
        expect((r.redactChunk("surrogate", text.slice(0, cut), false) + r.redactChunk("surrogate", text.slice(cut), true)) === "[REDACTED]").toBe(true);
      }
    }
  }
});
