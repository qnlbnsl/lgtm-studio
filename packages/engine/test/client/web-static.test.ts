import { symlinkSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import * as Effect from "effect/Effect";
import { expect, it } from "vitest";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";
import { freshSecret, http, withWeb } from "../support/webRig.ts";

registerRebuildCheck({ noLog: true });

it("5.10 static policy: catches weakening the browser policy or serving inline scripts", async () => withWeb((ctx) => Effect.gen(function* () {
  const seen = new Set<string>();
  for (const path of ["/", "/index.html", "/app.js"]) {
    const reply = yield* Effect.promise(() => http(ctx, path, { headers: { Origin: undefined } }));
    expect(reply.status).toBe(200);
    // An HTML page names one fresh style nonce, in its policy and in a meta element. No other file gets one.
    const policy = reply.headers["content-security-policy"] as string, nonce = /'nonce-([A-Za-z0-9+/]{24})'/.exec(policy)?.[1];
    if (path === "/app.js") expect(nonce).toBeUndefined();
    else { expect(nonce).toBeDefined(); expect(reply.body).toContain(`<meta name="lgtm-style-nonce" content="${nonce}">`); seen.add(nonce!); }
    expect(policy.replace(` 'nonce-${nonce}'`, "")).toBe("default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' blob:; frame-ancestors 'none'; base-uri 'none'; form-action 'none'");
    expect(policy).not.toMatch(/unsafe-inline|unsafe-eval/);
    if (path !== "/app.js") expect(reply.body).not.toMatch(/<script(?![^>]*\bsrc=)[^>]*>[\s\S]*?\S[\s\S]*?<\/script>/i);
  }
  expect(seen.size).toBe(2);
})));

for (const path of ["/../outside.txt", "/%2e%2e/outside.txt", "/%2e%2e%2foutside.txt", "/%252e%252e%252foutside.txt", "/..%5coutside.txt", "/linked.txt", "/linked-dir/outside.txt"]) {
  it(`5.10 confinement ${path}: catches a traversal, decoding or symbolic-link escape from the static root`, async () => withWeb((ctx) => Effect.gen(function* () {
    const marker = freshSecret();
    writeFileSync(join(ctx.rig.root, "outside.txt"), marker);
    symlinkSync(join(ctx.rig.root, "outside.txt"), join(ctx.staticRoot, "linked.txt"));
    symlinkSync(ctx.rig.root, join(ctx.staticRoot, "linked-dir"), "dir");
    const reply = yield* Effect.promise(() => http(ctx, path));
    expect(reply.body.includes(marker), "static response escaped its build folder").toBe(false);
  })));
}
