// Runs the mock server on a network port: node src/serve.ts
//
//   LGTM_MOCK_PORT       the port, 4180 when not set
//   LGTM_MOCK_HOST       the address to listen on, 127.0.0.1 when not set
//   LGTM_WEB_PASSWORD    the password to sign in with; any password except "wrong" when not set
//   LGTM_WEBSITE_ORIGIN  more origins a website may call from, separated by commas
//   LGTM_MOCK_MAP        a `floorplan` folder whose map the sample shows; a placeholder page when not set
import { createReadStream, lstatSync, readFileSync } from 'node:fs';
import { createServer } from 'node:http';
import { join, resolve } from 'node:path';
import { cors } from '@connectrpc/connect';
import { connectNodeAdapter } from '@connectrpc/connect-node';
import { createEngine, type MapInfo } from './engine.ts';
import { PLACEHOLDER_FACTS, PLACEHOLDER_MAP, PLACEHOLDER_PRODUCT } from './sample.ts';

const port = Number(process.env.LGTM_MOCK_PORT || 4180);
const host = process.env.LGTM_MOCK_HOST || '127.0.0.1';
const extra = (process.env.LGTM_WEBSITE_ORIGIN || '').split(',').map((x) => x.trim()).filter(Boolean);

// ---- the sample map's page ------------------------------------------------------------
// The page comes from a folder that Floorplan wrote, and it is read as it is. It is an agent's
// output, so it is served only at a frame's own address, with a sandbox, and never over 16 mebibytes.
const started = Date.now();
const mapFolder = process.env.LGTM_MOCK_MAP ? resolve(process.env.LGTM_MOCK_MAP) : null;
const MAP_LIMIT = 16 * 1024 * 1024;
let factsOf: { mtime: number; facts: MapInfo['facts'] } | null = null;
function pageFile(): { path: string; size: number; mtime: number } | undefined {
  if (!mapFolder) return undefined;
  try {
    const path = join(mapFolder, 'index.html'); const info = lstatSync(path);
    return info.isFile() && info.size <= MAP_LIMIT ? { path, size: info.size, mtime: info.mtimeMs } : undefined;
  } catch { return undefined; }
}
// The counts the map records about itself, read from graph.json when it is there and not too large.
function graphFacts(): MapInfo['facts'] {
  try {
    const path = join(mapFolder!, 'graph.json'); const info = lstatSync(path);
    if (!info.isFile() || info.size > MAP_LIMIT) return undefined;
    if (factsOf?.mtime === info.mtimeMs) return factsOf.facts;
    const graph = JSON.parse(readFileSync(path, 'utf8')) as { meta?: { generatedAt?: string; counts?: { files?: number; loc?: number } }; nodes?: unknown[]; links?: unknown[] };
    const made = Date.parse(graph.meta?.generatedAt ?? '');
    const facts = { generatedAt: Number.isNaN(made) ? undefined : made, files: graph.meta?.counts?.files, lines: graph.meta?.counts?.loc, parts: graph.nodes?.length, links: graph.links?.length };
    factsOf = { mtime: info.mtimeMs, facts };
    return facts;
  } catch { return undefined; }
}
const mapInfo = (): MapInfo | undefined => {
  if (!mapFolder) return { sizeBytes: Buffer.byteLength(PLACEHOLDER_MAP), modifiedAt: started, facts: PLACEHOLDER_FACTS(started) };
  const file = pageFile();
  return file && { sizeBytes: file.size, modifiedAt: file.mtime, facts: graphFacts() };
};

const engine = createEngine({ password: process.env.LGTM_WEB_PASSWORD || undefined, map: { info: mapInfo, url: (token) => `/frames/${token}` }, previewUrl: (token) => `/frames/${token}` });
const handler = connectNodeAdapter({ routes: engine.routes });

// A website on another origin may call this server only when it is on the same host or on
// this computer. The session cookie is sent with each call, so no other page may read a reply.
const LOOPBACK = new Set(['localhost', '127.0.0.1', '[::1]']);
function allowed(origin: string | undefined, hostHeader: string | undefined): boolean {
  if (!origin) return false;
  if (extra.includes(origin)) return true;
  try {
    const from = new URL(origin).hostname;
    const here = new URL(`http://${hostHeader || 'localhost'}`).hostname;
    return from === here || LOOPBACK.has(from);
  } catch { return false; }
}

createServer((req, res) => {
  const origin = req.headers.origin;
  if (allowed(origin, req.headers.host)) {
    res.setHeader('Access-Control-Allow-Origin', origin!);
    res.setHeader('Access-Control-Allow-Credentials', 'true');
    res.setHeader('Access-Control-Expose-Headers', cors.exposedHeaders.join(', '));
    res.setHeader('Vary', 'Origin');
    if (req.method === 'OPTIONS') {
      res.setHeader('Access-Control-Allow-Methods', cors.allowedMethods.join(', '));
      res.setHeader('Access-Control-Allow-Headers', cors.allowedHeaders.join(', '));
      res.setHeader('Access-Control-Max-Age', '7200');
      res.writeHead(204).end();
      return;
    }
  }
  // A frame's address is its own key: the page in a sandboxed frame sends no session cookie.
  const frame = /^\/frames\/([A-Z0-9]{20,})$/.exec((req.url || '').split('?')[0]!);
  if (frame && req.method === 'GET') {
    const kind = engine.frameKind(frame[1]!);
    const file = kind === 'map' ? pageFile() : undefined;
    if (!kind || (kind === 'map' && mapFolder && !file)) { res.writeHead(404, { 'Content-Type': 'text/plain' }).end('This address has ended.'); return; }
    res.writeHead(200, {
      'Content-Type': 'text/html; charset=utf-8', 'Cache-Control': 'no-store', 'Referrer-Policy': 'no-referrer', 'X-Content-Type-Options': 'nosniff',
      // The sandbox holds also when someone opens the address outside a frame. A product's page may
      // also send a form. Neither page gets the origin of this server, so neither can use the session.
      'Content-Security-Policy': `sandbox allow-scripts allow-popups${kind === 'preview' ? ' allow-forms' : ''}; frame-ancestors *`,
    });
    if (kind === 'preview') res.end(PLACEHOLDER_PRODUCT); else if (file) createReadStream(file.path).pipe(res); else res.end(PLACEHOLDER_MAP);
    return;
  }
  handler(req, res);
}).listen(port, host, () => {
  console.log(`The LGTM Studio mock server answers at http://${host}:${port}. It holds sample data only.`);
});
