import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { copyFile, mkdir, stat, symlink, writeFile } from 'node:fs/promises';
import { createServer } from 'node:net';
import { dirname, join, resolve, sep } from 'node:path';
import { createInterface } from 'node:readline';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';

const root = fileURLToPath(new URL('../../', import.meta.url));
const frontend = join(root, 'src/frontend');
const wait = 15_000;
const getServer = '/lgtm.v1.SessionService/GetServer';
const signIn = '/lgtm.v1.SessionService/SignIn';
const watchClients = '/lgtm.v1.ClientService/WatchClients';
const projectService = 'lgtm.v1.ProjectService';
const watchRepositories = '/' + projectService + '/WatchRepositories';
const sampleProject = 'console';
const fixturePassword = 'Sprint four sample';

function requireObservation(condition, message) {
  if (!condition) throw new Error(message);
}

// This interceptor follows the local router through Connect without collecting request bodies.
function traceSampleRouter() {
  const api = join(frontend, 'src/server/api.ts');
  const virtual = '\0sprint-four-sample-router';
  return {
    name: 'sprint-four-sample-router',
    enforce: 'pre',
    resolveId(source, importer) {
      if (source === '@connectrpc/connect' && importer === api) return virtual;
    },
    async load(id) {
      if (id !== virtual) return;
      const original = await this.resolve('@connectrpc/connect', api, { skipSelf: true });
      if (!original) throw new Error('The website did not resolve its Connect transport.');
      const module = JSON.stringify(original.id);
      return `
        export * from ${module};
        import { createRouterTransport as create } from ${module};
        export function createRouterTransport(routes, options) {
          const trace = next => async request => {
            const path = '/' + request.service.typeName + '/' + request.method.name;
            window.sprintFourSample.calls.push(path);
            const response = await next(request);
            if (path === ${JSON.stringify(getServer)}) {
              const { contract, sample, services } = response.message;
              window.sprintFourSample.info = { contract, sample, services };
            }
            return response;
          };
          const transport = options?.transport;
          return create(routes, {
            ...options,
            transport: { ...transport, interceptors: [trace, ...(transport?.interceptors ?? [])] },
          });
        }
      `;
    },
  };
}

async function buildWebsite(scratch) {
  const { build } = await import('vite');
  const mirror = join(scratch, 'build');
  const config = join(mirror, 'src/frontend/vite.config.ts');
  const outDir = join(scratch, 'website');
  // The config creates its development certificate beside itself. Keep that write in scratch.
  await mkdir(dirname(config), { recursive: true });
  await copyFile(join(frontend, 'vite.config.ts'), config);
  for (const from of ['node_modules', 'src/frontend/node_modules', 'design', 'third_party']) {
    await symlink(join(root, from), join(mirror, from), 'dir');
  }
  process.env.VITE_LGTM_SERVER = '';
  await build({
    root: frontend,
    configFile: config,
    configLoader: 'runner',
    cacheDir: join(scratch, 'vite-cache'),
    plugins: [traceSampleRouter()],
    build: { outDir },
  });
  return outDir;
}

async function listenPort() {
  const socket = createServer();
  socket.listen(0, '127.0.0.1');
  await once(socket, 'listening');
  const { port } = socket.address();
  await new Promise((done, reject) => socket.close(error => error ? reject(error) : done()));
  return port;
}

async function waitForMock(child, origin) {
  const pending = new AbortController();
  const { signal } = pending;
  const lines = createInterface({ input: child.stdout });
  const readyLine = `The LGTM Studio mock server answers at ${origin}. It holds sample data only.`;
  const readiness = (async () => {
    for await (const line of lines) {
      if (line === readyLine) return;
    }
    throw new Error('The mock closed its output before reporting readiness.');
  })();
  try {
    await Promise.race([
      readiness,
      once(child, 'exit', { signal }).then(([code, exitSignal]) => {
        throw new Error(`The mock exited before reporting readiness (${exitSignal ?? code}).`);
      }),
      delay(wait, undefined, { signal }).then(() => {
        throw new Error('The mock did not report readiness in time.');
      }),
    ]);
    requireObservation(child.exitCode === null && child.signalCode === null,
      'The mock exited during setup.');
  } finally {
    pending.abort();
    lines.close();
    child.stdout.resume();
  }
}

async function startMock() {
  const port = await listenPort();
  requireObservation(port !== 8787, 'The probe selected reserved port 8787.');
  const origin = 'http://127.0.0.1:' + port;
  const child = spawn(process.execPath, ['src/serve.ts'], {
    cwd: join(root, 'src/mock-server'),
    env: {
      ...process.env,
      LGTM_MOCK_HOST: '127.0.0.1',
      LGTM_MOCK_PORT: String(port),
      LGTM_WEB_PASSWORD: fixturePassword,
    },
    stdio: ['ignore', 'pipe', 'ignore'],
  });
  const stop = async () => {
    if (!child.pid || child.exitCode !== null || child.signalCode !== null) return;
    const exited = once(child, 'exit');
    child.kill('SIGKILL');
    await exited;
  };
  try {
    await waitForMock(child, origin);
    return { origin, stop };
  } catch (error) {
    await stop();
    throw error;
  }
}

async function openPage(context, origin, website) {
  const page = await context.newPage();
  page.setDefaultTimeout(wait);
  const calls = new Set();
  const answers = new Set();
  page.on('request', request => {
    const address = new URL(request.url());
    if (address.origin === origin && address.pathname.startsWith('/lgtm.v1.')) calls.add(address.pathname);
  });
  page.on('response', response => {
    const address = new URL(response.url());
    if (address.origin === origin && address.pathname.startsWith('/lgtm.v1.') && response.ok()) {
      answers.add(address.pathname);
    }
  });
  if (website) {
    // Give the same static build the server's origin so its ordinary cookie and origin rules apply.
    await page.route(origin + '/**', async route => {
      const { pathname } = new URL(route.request().url());
      if (pathname.startsWith('/lgtm.v1.')) return route.continue();
      const file = resolve(website, '.' + (pathname === '/' ? '/index.html' : pathname));
      if (file.startsWith(website + sep) && await stat(file).then(entry => entry.isFile(), () => false)) {
        return route.fulfill({ path: file });
      }
      return route.fulfill({ status: 404, body: '' });
    });
  }
  await page.goto(origin + '/', { waitUntil: 'domcontentloaded' });
  return { page, calls, answers };
}

async function login(page, password) {
  await page.getByLabel('Password', { exact: true }).fill(password);
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
}

async function connectRemote(page, origin) {
  const discovery = page.waitForResponse(origin + getServer);
  // Await both promises so a failed form interaction also handles the response wait.
  const [response] = await Promise.all([discovery, (async () => {
    await page.getByLabel('Server address', { exact: true }).fill('/');
    await page.getByRole('button', { name: 'Connect', exact: true }).click();
  })()]);
  requireObservation(response.ok(), 'GetServer did not return a successful reply.');
  return response.json();
}

async function showSample(page) {
  await page.waitForFunction(project => {
    const state = window.lgtm?.store.state;
    return state?.server.synced && state.server.sample && state.session.status === 'in' &&
      state.machines.length > 0 && state.projects.length > 1 &&
      state.projects.some(record => record.id === project);
  }, sampleProject, { timeout: wait });
}

function reportsSample(info) {
  return info?.contract === 1 && info.sample === true && (info.services ?? []).length === 0;
}

async function checkInPage(context, origin, website) {
  await context.addInitScript(() => {
    window.sprintFourSample = { calls: [] };
    localStorage.setItem('lgtm-mock:session', JSON.stringify('out'));
  });
  const { page, calls } = await openPage(context, origin, website);
  await page.getByRole('button', { name: 'Use sample data', exact: true }).click();
  await login(page, fixturePassword);
  await showSample(page);
  const sample = await page.evaluate(() => window.sprintFourSample);
  requireObservation(reportsSample(sample.info) &&
    [getServer, signIn, watchClients].every(path => sample.calls.includes(path)) &&
    !sample.calls.includes(watchRepositories) && calls.size === 0,
  'The in-page router did not discover, sign in, and watch machines locally.');
}

async function checkMock(context, origin, website) {
  const { page, calls } = await openPage(context, origin, website);
  const info = await connectRemote(page, origin);
  await login(page, fixturePassword);
  await showSample(page);
  requireObservation(reportsSample(info) && [getServer, signIn, watchClients].every(path => calls.has(path)) &&
    !calls.has(watchRepositories), 'The mock did not discover, sign in, and watch machines without a repository watch.');
}

async function modes(browser, origin, scratch, website) {
  let status = 22;
  let mock;
  try {
    mock = await startMock();
    status = 23;
    const inPage = await browser.newContext();
    try { await checkInPage(inPage, origin, website); } finally { await inPage.close(); }
    status = 24;
    const remoteMock = await browser.newContext();
    try { await checkMock(remoteMock, mock.origin, website); } finally { await remoteMock.close(); }
    status = 25;
    const real = await browser.newContext();
    const { page, calls, answers } = await openPage(real, origin, website);
    const info = await connectRemote(page, origin);
    requireObservation(info.contract === 1 && info.sample !== true && info.services?.includes(projectService),
      'The real server did not advertise the repository service at website contract revision 1.');
    status = 26;
    await login(page, process.env.LGTM_WEB_PASSWORD);
    await page.waitForFunction(() => {
      const state = window.lgtm?.store.state;
      return state?.server.synced && !state.server.sample && state.session.status === 'in' &&
        state.projects.length === 0;
    }, undefined, { timeout: wait });
    requireObservation([signIn, watchClients, watchRepositories].every(path => calls.has(path) && answers.has(path)),
      'The browser did not sign in and open both watches on the real server.');
    return 0;
  } catch (error) {
    // The Bash caller keeps this file private and prints a fixed sentence.
    await writeFile(join(scratch, 'browser-stage.log'), String(error));
    return status;
  } finally {
    await mock?.stop();
  }
}

async function removal(browser, origin, scratch, ids) {
  const context = await browser.newContext();
  const { page } = await openPage(context, origin);
  await login(page, process.env.LGTM_WEB_PASSWORD);
  const [orphan, shared, primary, secondary, orphanPath, sharedPath, keptPath] = ids;
  const expected = { orphan, shared, primary, secondary, orphanPath, sharedPath, keptPath };
  await page.waitForFunction(fixture => {
    const state = window.lgtm?.store.state;
    if (!state?.server.synced) return false;
    const orphan = state.repos.find(repo => repo.id === fixture.orphan);
    const shared = state.repos.find(repo => repo.id === fixture.shared);
    return orphan?.folders[fixture.primary] === fixture.orphanPath &&
      shared?.folders[fixture.primary] === fixture.sharedPath &&
      shared.folders[fixture.secondary] === fixture.keptPath;
  }, expected, { timeout: wait });
  await writeFile(join(scratch, 'page-ready'), '');
  await page.waitForFunction(fixture => {
    const state = window.lgtm.store.state;
    const shared = state.repos.find(repo => repo.id === fixture.shared);
    return !state.machines.some(machine => machine.id === fixture.primary) &&
      !state.repos.some(repo => repo.id === fixture.orphan) &&
      shared?.folders[fixture.secondary] === fixture.keptPath && !(fixture.primary in shared.folders);
  }, expected, { timeout: 40_000 });
  return 0;
}

const [mode, origin, scratch, ...ids] = process.argv.slice(2);
let status = 20;
let browser;
try {
  const website = mode === 'modes' ? await buildWebsite(scratch) : undefined;
  status = 21;
  const { chromium } = await import('@playwright/test');
  browser = await chromium.launch();
  status = mode === 'modes' ? await modes(browser, origin, scratch, website) :
    await removal(browser, origin, scratch, ids);
} catch (error) {
  await writeFile(join(scratch, 'browser-stage.log'), String(error));
} finally {
  await browser?.close();
}
process.exitCode = status;
