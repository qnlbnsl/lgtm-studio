import { randomBytes } from "node:crypto";
import { readFile, realpath, stat } from "node:fs/promises";
import { isAbsolute, relative, resolve, extname } from "node:path";
import * as Effect from "effect/Effect";
import * as HttpServerResponse from "effect/http/HttpServerResponse";

// Images may also come from blob URLs: the Files tab shows a worktree's image
// from bytes it read through the client API, never from a repository URL.
// A page may also carry one style element that a script adds at run time,
// such as the code editor's. Each HTML response gets a fresh random nonce,
// named in its policy and in a meta element, and only a style element that
// carries it is applied. Inline scripts and inline style attributes stay
// refused.
const policyFor = (nonce?: string) => `default-src 'self'; script-src 'self'; style-src 'self'${nonce === undefined ? "" : ` 'nonce-${nonce}'`}; img-src 'self' blob:; frame-ancestors 'none'; base-uri 'none'; form-action 'none'`;
export const styleNonceMeta = "lgtm-style-nonce";
const types: Record<string, string> = { ".html": "text/html; charset=utf-8", ".js": "text/javascript; charset=utf-8", ".css": "text/css; charset=utf-8", ".json": "application/json", ".svg": "image/svg+xml", ".png": "image/png", ".ico": "image/x-icon", ".woff2": "font/woff2" };
export const staticResponse = (root: string, raw: string, head: boolean, displayOrigin?: string) => Effect.promise(async () => {
  try {
    const path = decodeURIComponent(raw.split("?")[0]!);
    if (!path.startsWith("/") || path.includes("\\") || path.includes("\0") || path.includes("%") || path.split("/").includes("..")) throw new Error();
    const base = await realpath(root);
    const file = await realpath(resolve(base, `.${path === "/" ? "/index.html" : path}`));
    const diff = relative(base, file);
    if (diff === ".." || diff.startsWith("../") || isAbsolute(diff) || !(await stat(file)).isFile()) throw new Error();
    let data: Uint8Array = head ? new Uint8Array() : await readFile(file);
    const nonce = extname(file) === ".html" ? randomBytes(18).toString("base64") : undefined;
    if (nonce !== undefined && !head) data = Buffer.from(Buffer.from(data).toString("utf8").replace(/<head>|<!doctype html>(?![\s\S]*<head>)/i, (found) => `${found}<meta name="${styleNonceMeta}" content="${nonce}">`), "utf8");
    const policy = policyFor(nonce);
    // The caller supplies only the exact validated production configuration.
    // Repository content and request parameters never select a frame origin.
    const framePolicy = displayOrigin === undefined ? policy : `${policy}; frame-src ${displayOrigin}`;
    return HttpServerResponse.uint8Array(data, { headers: { "content-type": types[extname(file)] ?? "application/octet-stream", "content-security-policy": framePolicy, "x-content-type-options": "nosniff", "cache-control": "no-store" } });
  } catch { return HttpServerResponse.empty({ status: 404 }); }
});
