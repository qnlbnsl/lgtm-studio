import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { build } from 'vite';
import { serveWebsite } from './repository-server';

declare global {
  interface Window {
    repositoryRemount: {
      startDiscovery: (address: string) => void;
      discoveryFinished: () => Promise<void>;
      mountLegacy: () => void;
    };
  }
}

// The standalone entry only imports mount. An embedding page also needs the public unmount.
export async function serveRepositoryRemount() {
  const scratch = await mkdtemp(join(tmpdir(), 'lgtm-repository-remount-'));
  const cleanup = () => rm(scratch, { recursive: true, force: true });
  const source = (path: string) => JSON.stringify(fileURLToPath(new URL('../../' + path, import.meta.url)));
  const outDir = join(scratch, 'website');
  try {
    await writeFile(join(scratch, 'index.html'), '<div id="app"></div><script type="module" src="/embed.js"></script>');
    await writeFile(join(scratch, 'embed.js'), `
      import { mount, unmount } from ${source('src/app.tsx')};
      import { connect } from ${source('src/server/sync.ts')};
      const container = document.getElementById('app');
      let discovery;
      window.lgtm = mount(container);
      window.repositoryRemount = {
        startDiscovery(address) { discovery = connect(address); },
        discoveryFinished() { return discovery; },
        mountLegacy() {
          unmount(container);
          window.lgtm = mount(container, {
            mode: 'live',
            connection: {
              call() {},
              push() {},
              open(write) { write({ session: { status: 'in', firstRun: false }, projects: [] }); },
            },
          });
        },
      };
    `);
    await build({
      configFile: false,
      envDir: false,
      root: scratch,
      define: { 'import.meta.env.VITE_LGTM_SERVER': JSON.stringify('') },
      resolve: { alias: { '@mock': fileURLToPath(new URL('../../../../design/mock', import.meta.url)) } },
      build: { outDir },
      logLevel: 'error',
    });
    const web = await serveWebsite(outDir);
    return {
      origin: web.origin,
      async stop() {
        try { await web.stop(); } finally { await cleanup(); }
      },
    };
  } catch (error) {
    await cleanup();
    throw error;
  }
}
