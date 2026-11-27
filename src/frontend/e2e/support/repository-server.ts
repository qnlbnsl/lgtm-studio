import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { createReadStream } from 'node:fs';
import { copyFile, mkdir, mkdtemp, rm, stat, symlink } from 'node:fs/promises';
import { createServer, type RequestListener, type Server } from 'node:http';
import { createRequire } from 'node:module';
import { tmpdir } from 'node:os';
import { dirname, join, resolve, sep } from 'node:path';
import { createInterface } from 'node:readline';
import { fileURLToPath } from 'node:url';

import { Code, ConnectError, cors, type ConnectRouter, type ConnectRouterOptions } from '@connectrpc/connect';
import { ClientService, type Client } from '@lgtm/api/clients';
import { ProjectService } from '@lgtm/api/projects';
import { SessionService, SignInResult, type GetServerResponse } from '@lgtm/api/session';
import { test as base, type Page } from '@playwright/test';
import { build, type Plugin } from 'vite';

const root = fileURLToPath(new URL('../../../../', import.meta.url));
const frontend = join(root, 'src/frontend');
const mockServer = join(root, 'src/mock-server');
const requireFromMock = createRequire(join(mockServer, 'package.json'));
const { connectNodeAdapter } = requireFromMock('@connectrpc/connect-node') as {
  connectNodeAdapter: (options: ConnectRouterOptions & { routes: (router: ConnectRouter) => void }) => RequestListener;
};

export const procedure = {
  discovery: `/${SessionService.typeName}/${SessionService.method.getServer.name}`,
  signIn: `/${SessionService.typeName}/${SessionService.method.signIn.name}`,
  signOut: `/${SessionService.typeName}/${SessionService.method.signOut.name}`,
  machines: `/${ClientService.typeName}/${ClientService.method.watchClients.name}`,
  repositories: `/${ProjectService.typeName}/${ProjectService.method.watchRepositories.name}`,
};

interface DiscoveryProbe {
  calls: string[];
  localDiscovery?: { sample: boolean; services: string[] };
  simulationIntervals: () => number;
  timeouts: Record<number, number>;
  harnessInstalled?: (id: string) => boolean;
}

declare global {
  interface Window {
    discoveryProbe: DiscoveryProbe;
  }
}

function observeBrowserCalls(): Plugin {
  const api = join(frontend, 'src/server/api.ts');
  const data = join(frontend, 'src/data.ts');
  const probe = '\0discovery-connect-probe';
  return {
    name: 'discovery-connect-probe',
    enforce: 'pre',
    resolveId(source, importer) {
      if (importer === api && source === '@connectrpc/connect') return probe;
    },
    async load(id) {
      if (id !== probe) return;
      const original = await this.resolve('@connectrpc/connect', api, { skipSelf: true });
      if (!original) throw new Error('The Connect transport is missing.');
      const module = JSON.stringify(original.id);
      return `
        export * from ${module};
        import { createRouterTransport as create } from ${module};
        export function createRouterTransport(routes, options) {
          const transport = options?.transport;
          const observe = next => async request => {
            const path = '/' + request.service.typeName + '/' + request.method.name;
            window.discoveryProbe?.calls.push(path);
            const response = await next(request);
            const discovery = window.discoveryProbe?.localDiscovery;
            if (discovery && path === ${JSON.stringify(procedure.discovery)}) Object.assign(response.message, discovery);
            return response;
          };
          return create(routes, { ...options, transport: {
            ...transport, interceptors: [observe, ...(transport?.interceptors ?? [])],
          } });
        }
      `;
    },
    transform(source, id) {
      if (id === data) return source + '\nif (window.discoveryProbe) window.discoveryProbe.harnessInstalled = id => Boolean(HARNESSES[id]?.installed);\n';
    },
  };
}

export default async function buildWebsite() {
  const scratch = await mkdtemp(join(tmpdir(), 'lgtm-discovery-'));
  const cleanup = () => rm(scratch, { recursive: true, force: true });
  const previousAddress = process.env.VITE_LGTM_SERVER;
  try {
    const mirror = join(scratch, 'config');
    const config = join(mirror, 'src/frontend/vite.config.ts');
    const outDir = join(scratch, 'website');
    await mkdir(dirname(config), { recursive: true });
    await copyFile(join(frontend, 'vite.config.ts'), config);
    for (const entry of ['node_modules', 'src/frontend/node_modules', 'design', 'third_party']) {
      await symlink(join(root, entry), join(mirror, entry), 'dir');
    }
    try {
      process.env.VITE_LGTM_SERVER = '';
      await build({
        root: frontend,
        configFile: config,
        configLoader: 'runner',
        cacheDir: join(scratch, 'vite-cache'),
        plugins: [observeBrowserCalls()],
        build: { outDir },
        logLevel: 'error',
      });
    } finally {
      if (previousAddress === undefined) delete process.env.VITE_LGTM_SERVER;
      else process.env.VITE_LGTM_SERVER = previousAddress;
    }
    process.env.LGTM_DISCOVERY_WEBSITE = outDir;
    return cleanup;
  } catch (error) {
    await cleanup();
    throw error;
  }
}

export const test = base.extend<{}, { website: string }>({
  website: [async ({}, use) => {
    const directory = process.env.LGTM_DISCOVERY_WEBSITE;
    if (!directory) throw new Error('The Website test bundle was not built.');
    await use(directory);
  }, { scope: 'worker' }],
});

function barrier() {
  let open!: () => void;
  const reached = new Promise<void>((done) => { open = done; });
  return { open, reached };
}

function untilAborted(signal: AbortSignal): Promise<void> {
  return signal.aborted ? Promise.resolve() : new Promise((done) => {
    signal.addEventListener('abort', () => done(), { once: true });
  });
}

async function listen(server: Server): Promise<string> {
  const ready = once(server, 'listening');
  server.listen(0, '127.0.0.1');
  await ready;
  const address = server.address();
  if (!address || typeof address === 'string' || address.port === 8787) {
    await stop(server);
    throw new Error('The fixture needs a free loopback port.');
  }
  return 'http://127.0.0.1:' + address.port;
}

function stop(server: Server): Promise<void> {
  return new Promise((done, fail) => {
    server.close((error) => error ? fail(error) : done());
    server.closeAllConnections();
  });
}

type DiscoveryFields = Pick<GetServerResponse, 'product' | 'contract' | 'sample' | 'services'>;

export async function serveRepositoryDiscovery(pageOrigin: string, discovery: Partial<DiscoveryFields>, clients: Client[] = []) {
  const requested = barrier();
  const reply = barrier();
  const machinesRequested = barrier();
  const machinesAborted = barrier();
  const machineSnapshot = barrier();
  const calls: { path: string; signedIn: boolean }[] = [];
  let signedIn = false;
  // The static test response only marks a session. No test records request bodies or cookie values.
  const cookie = 'lgtm-discovery-session=active';
  const accepts = (headers: Headers) => signedIn && (headers.get('cookie') ?? '').split(/; */).includes(cookie);
  // Protocol Buffers normally omit repeated fields with no entries in JSON.
  const handler = connectNodeAdapter({
    jsonOptions: { alwaysEmitImplicit: discovery.services?.length === 0 },
    routes: register,
  });
  function register(router: ConnectRouter) {
    router.service(SessionService, {
      async getServer(_request, context) {
        calls.push({ path: procedure.discovery, signedIn: accepts(context.requestHeader) });
        requested.open();
        await Promise.race([reply.reached, untilAborted(context.signal)]);
        return { product: 'lgtm-studio', contract: 1, signedIn: accepts(context.requestHeader), ...discovery };
      },
      signIn(_request, context) {
        calls.push({ path: procedure.signIn, signedIn: accepts(context.requestHeader) });
        signedIn = true;
        context.responseHeader.set('Set-Cookie', cookie + '; Path=/; HttpOnly; SameSite=Strict');
        return { result: SignInResult.SIGNED_IN };
      },
      signOut(_request, context) {
        calls.push({ path: procedure.signOut, signedIn: accepts(context.requestHeader) });
        signedIn = false;
        context.responseHeader.set('Set-Cookie', cookie + '; Path=/; HttpOnly; SameSite=Strict; Max-Age=0');
        return {};
      },
    });
    function recordWatch(kind: 'machines' | 'repositories', headers: Headers) {
      const authenticated = accepts(headers);
      calls.push({ path: procedure[kind], signedIn: authenticated });
      if (!authenticated) throw new ConnectError('The discovery fixture has no session.', Code.Unauthenticated);
    }
    router.service(ClientService, {
      async *watchClients(_request, context) {
        recordWatch('machines', context.requestHeader);
        const aborted = untilAborted(context.signal);
        void aborted.then(machinesAborted.open);
        machinesRequested.open();
        await Promise.race([machineSnapshot.reached, aborted]);
        if (!context.signal.aborted) yield { change: { case: 'all', value: { clients } } };
        await aborted;
      },
    });
    router.service(ProjectService, {
      async *watchRepositories(_request, context) {
        recordWatch('repositories', context.requestHeader);
        yield { change: { case: 'all', value: { repositories: [] } } };
        await untilAborted(context.signal);
      },
    });
  }
  const server = createServer((request, response) => {
    if (request.headers.origin === pageOrigin) {
      response.setHeader('Access-Control-Allow-Origin', pageOrigin);
      response.setHeader('Access-Control-Allow-Credentials', 'true');
      response.setHeader('Access-Control-Expose-Headers', cors.exposedHeaders.join(', '));
      response.setHeader('Vary', 'Origin');
      if (request.method === 'OPTIONS') {
        response.setHeader('Access-Control-Allow-Methods', cors.allowedMethods.join(', '));
        response.setHeader('Access-Control-Allow-Headers', cors.allowedHeaders.join(', '));
        response.writeHead(204).end();
        return;
      }
    }
    handler(request, response);
  });
  const origin = await listen(server);
  return {
    origin, calls,
    discoveryRequested: requested.reached, releaseDiscovery: reply.open,
    machinesRequested: machinesRequested.reached, machinesAborted: machinesAborted.reached,
    releaseMachines: machineSnapshot.open,
    stop: () => stop(server),
  };
}

export async function serveWebsite(directory: string) {
  const types: Record<string, string> = { html: 'text/html', js: 'text/javascript', css: 'text/css', woff2: 'font/woff2' };
  const server = createServer(async (request, response) => {
    const url = new URL(request.url ?? '/', 'http://127.0.0.1');
    const file = resolve(directory, '.' + (url.pathname === '/' ? '/index.html' : url.pathname));
    if (!file.startsWith(directory + sep) || !await stat(file).then((entry) => entry.isFile(), () => false)) {
      response.writeHead(404).end();
      return;
    }
    response.setHeader('Content-Type', types[file.split('.').at(-1)!] ?? 'application/octet-stream');
    createReadStream(file).pipe(response);
  });
  return { origin: await listen(server), stop: () => stop(server) };
}

export async function startStandaloneMock(pageOrigin: string) {
  const reservation = createServer();
  const origin = await listen(reservation);
  await stop(reservation);

  const child = spawn('pnpm', ['--filter', '@lgtm/mock-server', 'start'], {
    cwd: root,
    detached: true,
    stdio: ['ignore', 'pipe', 'ignore'],
    env: { ...process.env, LGTM_MOCK_HOST: '127.0.0.1', LGTM_MOCK_PORT: new URL(origin).port,
      LGTM_WEBSITE_ORIGIN: pageOrigin, LGTM_WEB_PASSWORD: '', LGTM_MOCK_MAP: '' },
  });
  const output = createInterface({ input: child.stdout });
  const exited = new Promise<void>((done) => {
    child.once('error', () => done());
    child.once('exit', () => done());
  });
  const close = async () => {
    output.close();
    if (child.pid && child.exitCode === null && child.signalCode === null) {
      process.kill(-child.pid, 'SIGTERM');
      await exited;
    }
  };
  try {
    await new Promise<void>((done, fail) => {
      const timer = setTimeout(() => fail(new Error('The standalone mock did not start.')), 10_000);
      output.on('line', (line) => {
        if (line.includes(`mock server answers at ${origin}.`)) { clearTimeout(timer); done(); }
      });
      void exited.then(() => { clearTimeout(timer); fail(new Error('The standalone mock exited.')); });
    });
    return { origin, stop: close };
  } catch (error) {
    await close();
    throw error;
  }
}

export async function observeWebsite(page: Page, origins: string[]) {
  const calls: string[] = [];
  const errors: string[] = [];
  page.on('pageerror', () => errors.push('page error'));
  page.on('request', (request) => {
    const url = new URL(request.url());
    if (origins.includes(url.origin) && url.pathname.startsWith('/lgtm.v1.') && request.method() !== 'OPTIONS') {
      calls.push(url.pathname);
    }
  });
  await page.route('**/*', (route) => {
    const url = new URL(route.request().url());
    return origins.includes(url.origin) && url.port !== '8787' ? route.continue() : route.abort();
  });

  const now = new Date('2026-11-27T12:00:00Z');
  await page.clock.install({ time: now });
  await page.clock.pauseAt(new Date(now.getTime() + 60_000));
  await page.addInitScript(() => {
    const browser: Window = window;
    const intervals = new Set<number>();
    const timeouts: Record<number, number> = {};
    const setInterval = browser.setInterval.bind(browser);
    const clearInterval = browser.clearInterval.bind(browser);
    const setTimeout = browser.setTimeout.bind(browser);
    window.discoveryProbe = { calls: [], simulationIntervals: () => intervals.size, timeouts };
    browser.setInterval = (handler, delay, ...args) => {
      const handle = setInterval(handler, delay, ...args);
      if (delay === 4200) intervals.add(handle);
      return handle;
    };
    browser.clearInterval = (handle) => {
      if (handle !== undefined) intervals.delete(handle);
      clearInterval(handle);
    };
    browser.setTimeout = (handler, delay, ...args) => {
      if (delay !== undefined) timeouts[delay] = (timeouts[delay] ?? 0) + 1;
      return setTimeout(handler, delay, ...args);
    };
  });
  return { calls, errors };
}
