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
import { ClientService, ClientState, type Client, type WatchClientsResponse } from '@lgtm/api/clients';
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
  const main = join(frontend, 'src/main.ts');
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
      if (id === main) return source + `
        import { unmount } from './app';
        import { connect } from './server/sync';
        if (window.repositorySessionProbe) window.repositorySessionProbe.bridge = {
          connect,
          remount() {
            const container = document.getElementById('app');
            unmount(container);
            window.lgtm = mount(container);
          },
        };
      `;
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

export async function serveWebsite(directory: string, hostname?: string) {
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
  return { origin: withHostname(await listen(server), hostname), stop: () => stop(server) };
}

function withHostname(origin: string, hostname?: string) {
  const url = new URL(origin);
  if (hostname) url.hostname = hostname;
  return url.origin;
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

export const sessionProcedure = {
  discovery: procedure.discovery,
  signIn: procedure.signIn,
  signOut: procedure.signOut,
  watch: procedure.machines,
  create: `/${ClientService.typeName}/${ClientService.method.createClientToken.name}`,
  revoke: `/${ClientService.typeName}/${ClientService.method.revokeClientToken.name}`,
  approve: `/${ClientService.typeName}/${ClientService.method.approveClient.name}`,
  remove: `/${ClientService.typeName}/${ClientService.method.removeClient.name}`,
  scan: `/${ClientService.typeName}/${ClientService.method.scanClientHarnesses.name}`,
} as const;
type SessionMethod = keyof typeof sessionProcedure;
type CookieObservation = 'missing' | 'accepted' | 'unexpected';
interface SessionCall { listener: string; method: SessionMethod; cookie: CookieObservation }

export function sessionRegistry() {
  return { active: false, value: crypto.randomUUID(), calls: [] as SessionCall[] };
}

// Each listener can hold one request per procedure. Call records contain only cookie classifications.
export async function serveRepositorySession({ pageOrigin, hostname, listener, registry, clients = [], sample = false }: {
  pageOrigin: string; hostname: string; listener: string; registry: ReturnType<typeof sessionRegistry>;
  clients?: Client[]; sample?: boolean;
}) {
  const cookieName = 'lgtm-session-fixture';
  const cookieFlags = '; Path=/; HttpOnly; SameSite=Strict';
  type Held = { arrived: ReturnType<typeof barrier>; release: ReturnType<typeof barrier>; aborted: ReturnType<typeof barrier> };
  const holds = new Map<SessionMethod, Held>();
  type StreamStep = WatchClientsResponse['change'] | Code;
  const steps: StreamStep[] = [];
  let stepReady = barrier();
  const watchAborts: number[] = [];
  let watchCount = 0;
  let tokenCount = 0;
  const refusals = new Map<SessionMethod, Code>();

  async function record(method: SessionMethod, headers: Headers, signal: AbortSignal, protect = true) {
    const cookie = (headers.get('cookie') ?? '').split(/; */).find((pair) => pair.startsWith(cookieName + '='));
    const accepted = registry.active && cookie === cookieName + '=' + registry.value;
    registry.calls.push({ listener, method, cookie: accepted ? 'accepted' : cookie ? 'unexpected' : 'missing' });
    if (protect && !accepted) throw new ConnectError('The fixture session is missing.', Code.Unauthenticated);
    const hold = holds.get(method);
    if (hold) {
      const aborted = untilAborted(signal);
      holds.delete(method);
      void aborted.then(hold.aborted.open);
      hold.arrived.open();
      await Promise.race([hold.release.reached, aborted]);
      if (signal.aborted) throw new ConnectError('The fixture call ended.', Code.Canceled);
    }
    const refusal = refusals.get(method);
    if (refusal !== undefined) throw new ConnectError('The fixture refused this call.', refusal);
    return accepted;
  }

  const handler = connectNodeAdapter({
    routes(router) {
      router.service(SessionService, {
        async getServer(_request, context) {
          const signedIn = await record('discovery', context.requestHeader, context.signal, false);
          return { product: 'lgtm-studio', contract: 1, sample, signedIn, services: sample ? [] : [ProjectService.typeName] };
        },
        async signIn(_request, context) {
          await record('signIn', context.requestHeader, context.signal, false);
          registry.active = true;
          context.responseHeader.set('Set-Cookie', cookieName + '=' + registry.value + cookieFlags);
          return { result: SignInResult.SIGNED_IN };
        },
        async signOut(_request, context) {
          await record('signOut', context.requestHeader, context.signal);
          registry.active = false;
          context.responseHeader.set('Set-Cookie', cookieName + '=' + cookieFlags + '; Max-Age=0');
          return {};
        },
      });
      router.service(ClientService, {
        async *watchClients(_request, context) {
          await record('watch', context.requestHeader, context.signal);
          const watch = ++watchCount;
          const aborted = untilAborted(context.signal);
          void aborted.then(() => watchAborts.push(watch));
          yield { change: { case: 'all', value: { clients } } };
          while (!context.signal.aborted) {
            if (steps.length === 0) await Promise.race([stepReady.reached, aborted]);
            if (context.signal.aborted) return;
            const step = steps.shift();
            if (steps.length === 0) stepReady = barrier();
            if (typeof step === 'number') throw new ConnectError('The fixture ended the watch.', step);
            if (step) yield { change: step };
          }
        },
        async createClientToken(_request, context) {
          const tokenId = 'pair-' + ++tokenCount;
          await record('create', context.requestHeader, context.signal);
          // The identifier is enough to match an arriving machine. The fixture sends no secret.
          return { tokenId, expiresAt: { seconds: 1795784400n }, address: '10.2.3.4' };
        },
        async revokeClientToken(_request, context) {
          await record('revoke', context.requestHeader, context.signal);
          return {};
        },
        async approveClient(request, context) {
          await record('approve', context.requestHeader, context.signal);
          return { client: { id: request.id, name: request.name, state: ClientState.ONLINE } };
        },
        async removeClient(_request, context) {
          await record('remove', context.requestHeader, context.signal);
          return {};
        },
        async scanClientHarnesses(_request, context) {
          await record('scan', context.requestHeader, context.signal);
          return {};
        },
      });
      router.service(ProjectService, {
        async *watchRepositories(_request, context) {
          if (!registry.active || !(context.requestHeader.get('cookie') ?? '').split(/; */).includes(cookieName + '=' + registry.value)) {
            throw new ConnectError('The fixture session is missing.', Code.Unauthenticated);
          }
          yield { change: { case: 'all', value: { repositories: [] } } };
          await untilAborted(context.signal);
        },
      });
    },
  });
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
  return {
    origin: withHostname(await listen(server), hostname),
    listener,
    calls: () => registry.calls.filter((call) => call.listener === listener),
    watchAborts,
    refuse(method: SessionMethod, code: Code) { refusals.set(method, code); },
    hold(method: SessionMethod) {
      const hold = { arrived: barrier(), release: barrier(), aborted: barrier() };
      holds.set(method, hold);
      return { arrived: hold.arrived.reached, release: hold.release.open, aborted: hold.aborted.reached };
    },
    send(step: StreamStep) { steps.push(step); stepReady.open(); },
    stop: () => stop(server),
  };
}

interface BrowserReply {
  ready: boolean;
  release: () => void;
}

declare global {
  interface Window {
    repositorySessionProbe: {
      hold: (origin: string, match: string) => BrowserReply;
      reply?: BrowserReply;
      requests: string[];
      watchSignals: { origin: string; signal: AbortSignal | null | undefined }[];
      bridge?: { connect: (address: string) => Promise<void>; remount: () => void };
    };
  }
}

export async function observeRepositorySession(page: Page, origins: string[]) {
  const trace = await observeWebsite(page, origins);
  await page.addInitScript((watchPath) => {
    const send = window.fetch.bind(window);
    const watchSignals: Window['repositorySessionProbe']['watchSignals'] = [];
    let held: { origin: string; match: string; ready: boolean; release: () => void; gate: Promise<void> } | undefined;
    window.repositorySessionProbe = {
      watchSignals, requests: [],
      hold(origin, match) {
        let release!: () => void;
        const gate = new Promise<void>((done) => { release = done; });
        return held = { origin, match, ready: false, gate, release };
      },
    };
    window.fetch = async (input, init) => {
      const url = new URL(input instanceof Request ? input.url : String(input), location.href);
      if (url.pathname.startsWith('/lgtm.v1.')) window.repositorySessionProbe.requests.push(url.origin + url.pathname);
      const reply = held && url.origin === held.origin && url.pathname.includes(held.match) ? held : undefined;
      if (reply) held = undefined;
      if (url.pathname === watchPath) watchSignals.push({ origin: url.origin, signal: init?.signal });
      const response = await send(input, init);
      if (!reply) return response;

      // Hold a fully received body at the application boundary, even if its old request is later aborted.
      const body = await response.arrayBuffer();
      reply.ready = true;
      await reply.gate;
      // Keep body reads on the promise queue once the barrier opens.
      return Object.assign(new Response(null, response), {
        arrayBuffer: async () => body,
        json: async () => JSON.parse(new TextDecoder().decode(body)),
      });
    };
  }, procedure.machines);
  return trace;
}
