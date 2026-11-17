import { createHash } from "node:crypto";
import * as Redacted from "effect/Redacted";
import { expect, it, vi } from "vitest";
import * as Forms from "../../src/identity/redactionForms.ts";
import { makeRedactor } from "../../src/identity/redactor.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";
registerRebuildCheck({ noLog: true });

for (const [count, length] of [[14, 2000], [20, 2000], [200, 134], [100, 2000]]) {
  it(`r5 retains the newest oversized form list: ${count} by ${length}`, () => {
    const values = Array.from({ length: count! }, (_, i) => Array.from({ length: Math.ceil(length! / 32) }, (_, block) =>
      createHash("sha256").update(`s19-fake-cache-${i}-${block}`).digest()).reduce((a, b) => Buffer.concat([a, b]), Buffer.alloc(0)).toString("base64url").slice(0, length));
    const compile = vi.spyOn(Forms, "compileForms");
    try {
      const r = makeRedactor({ secrets: () => values.map(value => Redacted.make(value)), tokenHashes: () => new Set() });
      for (let call = 0; call < 4; call++) {
        expect(r.redactText("ordinary message.")).toBe("ordinary message.");
        expect(r.redactChunk("cached", "ordinary message.", false)).toBe("ordinary message.");
        expect(r.redactChunk("cached", "", true)).toBe("");
      }
      expect(compile.mock.results[0]!.value.formCharacters).toBeGreaterThan(Forms.maximumCachedFormCharacters);
      expect(compile).toHaveBeenCalledTimes(1);
    } finally { compile.mockRestore(); }
  });
}
