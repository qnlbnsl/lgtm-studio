// The Vite set-up of the website.
// The fonts, CodeMirror, the plans and the default workflow are still in design/mock/,
// so the website reads them from there until they move here.
import { defineConfig, type Plugin } from 'vite';
import preact from '@preact/preset-vite';
import { execFileSync } from 'node:child_process';
import { chmodSync, createReadStream, existsSync, mkdirSync, readFileSync, readdirSync, statSync, writeFileSync } from 'node:fs';
import { hostname, networkInterfaces } from 'node:os';
import { readFile } from 'node:fs/promises';
import { dirname, extname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const repo = resolve(here, '../..');
const mock = resolve(repo, 'design/mock');

// The pages a sprint's Plan tab opens in a frame, served from design/mock/plans/ at the same
// relative address the mock uses.
const SHARED = ['plans'];
const TYPES: Record<string, string> = { '.html': 'text/html; charset=utf-8', '.css': 'text/css; charset=utf-8', '.json': 'application/json' };
function mockFolders(): Plugin {
  return {
    name: 'lgtm-mock-folders',
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const path = decodeURIComponent((req.url || '').split('?')[0] || '');
        const dir = SHARED.find((d) => path.startsWith(`/${d}/`));
        if (!dir) return next();
        const file = resolve(mock, '.' + path);
        if (!file.startsWith(join(mock, dir) + '/') || !existsSync(file) || !statSync(file).isFile()) return next();
        res.setHeader('Content-Type', TYPES[extname(file)] || 'application/octet-stream');
        createReadStream(file).pipe(res);
      });
    },
    async generateBundle() {
      for (const dir of SHARED) {
        for (const name of readdirSync(join(mock, dir))) {
          this.emitFile({ type: 'asset', fileName: `${dir}/${name}`, source: await readFile(join(mock, dir, name)) });
        }
      }
    },
  };
}

// The development server speaks HTTPS, so a browser on another computer may send the password.
// It signs its own certificate with openssl and keeps it in .cert/, which git ignores.
// The certificate names this computer's addresses, and it is made again when they change or after 300 days.
function ownCertificate(): { key: Buffer; cert: Buffer } | undefined {
  const dir = resolve(here, '.cert'); const key = join(dir, 'key.pem'); const cert = join(dir, 'cert.pem'); const list = join(dir, 'names');
  const addresses = Object.values(networkInterfaces()).flat().filter((a) => a && a.family === 'IPv4').map((a) => a!.address);
  const names = ['DNS:localhost', `DNS:${hostname()}`, ...[...new Set(['127.0.0.1', ...addresses])].sort().map((a) => `IP:${a}`)].join(',');
  const fresh = existsSync(key) && existsSync(cert) && existsSync(list) && readFileSync(list, 'utf8') === names && Date.now() - statSync(cert).mtimeMs < 300 * 86_400_000;
  if (!fresh) {
    try {
      mkdirSync(dir, { recursive: true });
      execFileSync('openssl', ['req', '-x509', '-newkey', 'ec', '-pkeyopt', 'ec_paramgen_curve:prime256v1', '-nodes', '-days', '365', '-subj', '/CN=LGTM Studio development',
        '-addext', `subjectAltName=${names}`, '-keyout', key, '-out', cert], { stdio: 'ignore' });
      chmodSync(key, 0o600); writeFileSync(list, names);
    } catch {
      console.warn('The website is served over http, because openssl could not make a certificate.');
      return undefined;
    }
  }
  return { key: readFileSync(key), cert: readFileSync(cert) };
}
const https = ownCertificate();

// The host of the website can pass the contract's calls on to a server it reaches itself.
// The browser then talks to one address only, and the address "/" on the first screen means that.
// LGTM_SERVER_BEHIND names the server, and the mock server on this computer is used when it is not set.
// The page of a codebase map is read from a frame's address on the server, so that address is passed on too.
const server = process.env.LGTM_SERVER_BEHIND || 'http://127.0.0.1:4180';
const behind = { '/lgtm.v1.': { target: server }, '/frames/': { target: server } };

export default defineConfig({
  plugins: [preact(), mockFolders()],
  resolve: { alias: { '@mock': mock } },
  server: {
    host: '0.0.0.0',
    port: 4179,
    strictPort: true,
    proxy: behind,
    ...(https ? { https } : {}),
    fs: {
      strict: true,
      allow: [here, mock, resolve(repo, 'src/contracts'), resolve(repo, 'src/mock-server'), resolve(repo, 'design/fonts'), resolve(repo, 'third_party/inter'), resolve(repo, 'third_party/jetbrains-mono'), resolve(repo, 'third_party/codemirror'), resolve(repo, 'node_modules')],
    },
  },
  preview: { proxy: behind, ...(https ? { https } : {}) },
  build: { outDir: 'dist', emptyOutDir: true },
});
