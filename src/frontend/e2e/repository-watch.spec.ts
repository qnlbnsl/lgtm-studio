import { create, type MessageInitShape } from '@bufbuild/protobuf';
import { Code } from '@connectrpc/connect';
import { ClientSchema, ClientService, ClientState, type Client, type WatchClientsResponseSchema } from '@lgtm/api/clients';
import {
  ProjectService, RepositorySchema, type Repository, type WatchRepositoriesResponseSchema,
} from '@lgtm/api/projects';
import { expect, type Page } from '@playwright/test';
import {
  controlledWatch, observeRepositoryWatch, procedure, serveRepositorySession, serveWebsite, sessionRegistry, test,
} from './support/repository-server';

const firstHost = 'first.lgtm.test';
const otherHost = 'other.lgtm.test';
const repoId = '018f2ad2-bcde-7456-8123-000000000001';
const orphanId = '018f2ad2-bcde-7456-8123-000000000002';
const staleId = '018f2ad2-bcde-7456-8123-000000000003';
const machine = (id: string, name: string) => create(ClientSchema, { id, name, state: ClientState.ONLINE });
const worker = machine('first-worker', 'First worker');
const survivor = machine('next-worker', 'Next worker');
const waiting = create(ClientSchema, {
  id: 'waiting-worker', name: 'Waiting worker', state: ClientState.WAITING_FOR_APPROVAL, tokenId: 'pair-1',
});
const watchedApproval = create(ClientSchema, {
  id: waiting.id, name: 'Name from the watch', state: ClientState.OFFLINE, agentLimit: 9,
});
const folder = (suffix: string, client: Client, name: string) => ({
  id: '018f2ad2-bcde-7456-8123-0000000001' + suffix, clientId: client.id, resolvedPath: '/checkouts/' + name,
});
const firstFolder = folder('01', worker, 'first');
const secondFolder = folder('02', worker, 'second');
const survivingFolder = folder('03', survivor, 'surviving');
const origin = 'https://example.com/acme/watched.git';
const commit = '0123456789abcdef0123456789abcdef01234567';
const initialFacts = {
  id: repoId, name: 'Watched checkout', mainBranch: 'trunk', remote: origin, firstCommit: commit,
};
const initialRepo = create(RepositorySchema, { ...initialFacts, folders: [secondFolder, survivingFolder, firstFolder] });
const orphan = create(RepositorySchema, {
  id: orphanId, name: 'Orphan checkout', mainBranch: 'main', folders: [folder('04', worker, 'orphan')],
});
const stale = create(RepositorySchema, { id: staleId, name: 'Obsolete checkout', mainBranch: 'main' });

type MachineFrame = MessageInitShape<typeof WatchClientsResponseSchema>;
type RepositoryFrame = MessageInitShape<typeof WatchRepositoriesResponseSchema>;
type Listener = Awaited<ReturnType<typeof serveRepositorySession>>;

const watchTest = test.extend<{
  rig: { web: string; first: WatchListener; other: WatchListener };
}>({
  rig: async ({ website }, use) => {
    const web = await serveWebsite(website, firstHost);
    const listeners: Listener[] = [];
    const services = [ProjectService.typeName, ClientService.typeName, 'lgtm.v1.ProjectServiceLater'];
    const open = async (hostname: string, listener: string, advertised: string[]) => {
      const machines = controlledWatch<MachineFrame>();
      const repositories = controlledWatch<RepositoryFrame>();
      const server = await serveRepositorySession({
        pageOrigin: web.origin, hostname, listener, registry: sessionRegistry(),
        controlled: { services: advertised, machines: machines.read, repositories: repositories.read },
      });
      listeners.push(server);
      return { server, machines, repositories };
    };
    try {
      const first = await open(firstHost, 'first', services);
      const other = await open(otherHost, 'other', [...services].reverse());
      await use({ web: web.origin, first, other });
    } finally {
      await Promise.all([web.stop(), ...listeners.map((listener) => listener.stop())]);
    }
  },
});

type WatchListener = {
  server: Listener;
  machines: ReturnType<typeof controlledWatch<MachineFrame>>;
  repositories: ReturnType<typeof controlledWatch<RepositoryFrame>>;
};

watchTest.use({ launchOptions: {
  args: [`--host-resolver-rules=MAP ${firstHost} 127.0.0.1, MAP ${otherHost} 127.0.0.1`, '--no-proxy-server'],
} });

const tries = { timeout: 2500 };
const missingRepository = 'This repository is not known';

const allMachines = (clients: Client[], value = 1n): MachineFrame =>
  ({ change: { case: 'all', value: { clients } }, position: { value } });
const savedMachine = (client: Client, value = 2n): MachineFrame =>
  ({ change: { case: 'saved', value: client }, position: { value } });
const allRepositories = (repositories: Repository[], value = 1n): RepositoryFrame =>
  ({ change: { case: 'all', value: { repositories } }, position: { value } });
const savedRepository = (repository: Repository, value = 2n): RepositoryFrame =>
  ({ change: { case: 'saved', value: repository }, position: { value } });

const busy = (page: Page) => page.locator('main[aria-busy="true"]');
const liveBadge = (page: Page) => page.getByRole('button', { name: 'Connection: Live', exact: true });

async function visit(page: Page, rig: { web: string; first: WatchListener; other: WatchListener }, hash = '#/machines') {
  const trace = await observeRepositoryWatch(page, [rig.web, rig.first.server.origin, rig.other.server.origin]);
  await page.goto(rig.web + '/?data=worst' + hash);
  await expect.poll(() => page.evaluate(() => Boolean(window.repositoryWatchProbe.bridge))).toBe(true);
  return trace;
}

function choose(page: Page, origin: string) {
  return page.evaluate((value) => { void window.repositoryWatchProbe.bridge!.connect(value); }, origin);
}

async function selectAndSignIn(page: Page, site: WatchListener) {
  await choose(page, site.server.origin);
  await expect.poll(() => page.evaluate(() => ({
    address: window.lgtm.store.state.server.address, status: window.lgtm.store.state.server.status,
  }))).toEqual({ address: site.server.origin, status: 'ready' });
  await expect(page.getByLabel('Password', { exact: true })).toBeVisible();
  // The loopback fixtures use two same-site hostnames over plain HTTP.
  await page.evaluate(() => { void window.lgtm.act.signIn('browser fixture input'); });
  await expect.poll(() => site.machines.attempts.length, tries).toBe(1);
}

async function requireBothWatches(site: WatchListener) {
  await expect.poll(() => site.repositories.attempts.length, {
    ...tries, message: 'The advertising server starts a repository watch beside the machine watch.',
  }).toBe(1);
  const cookies = site.server.calls().filter((call) => call.method === 'watch' || call.method === 'repositories');
  expect(cookies.map((call) => call.cookie)).toEqual(['accepted', 'accepted']);
}

function projection(page: Page) {
  return page.evaluate(() => window.lgtm.store.state.repos.map((repo) => ({
    id: repo.id, name: repo.name, main: repo.main, remote: repo.remote,
    remoteOmitted: repo.remoteOmitted, firstCommit: repo.firstCommit, folders: repo.folders,
    folderRecords: ((repo.folderRecords ?? []) as Array<{ id: string; clientId: string; resolvedPath: string }>).map(
      ({ id, clientId, resolvedPath }) => ({ id, clientId, resolvedPath }),
    ),
  })));
}

function snapshot(page: Page) {
  return page.evaluate(() => {
    const snapshot = window.repositoryWatchProbe.bridge!.currentRepositorySnapshot();
    return snapshot ? { openedAt: snapshot.openedAt, ids: snapshot.repos.map((repo) => repo.id) } : null;
  });
}

function workspace(page: Page) {
  return page.evaluate(() => {
    const s = window.lgtm.store.state;
    return {
      address: s.server.address, auth: s.session.status, synced: s.server.synced, hash: location.hash,
      machines: s.machines.map((item) => item.id), joining: s.joining.map((item) => item.id),
      repos: s.repos.map((item) => item.id), dialog: s.dialog?.type ?? null, expansions: Object.keys(s.expanded),
      sampleCounts: [s.projects.length, s.teams.length, s.agents.length, s.tasks.length, s.requests.length, Object.keys(s.threads).length],
      simulation: window.discoveryProbe.simulationIntervals(),
    };
  });
}

async function openWorkspace(page: Page, site: WatchListener) {
  await selectAndSignIn(page, site);
  await requireBothWatches(site);
  site.machines.attempts[0]!.send(allMachines([worker, survivor, waiting]));
  site.repositories.attempts[0]!.send(allRepositories([initialRepo, orphan]));
  await expect(liveBadge(page)).toBeVisible();
}

async function browserFrame(page: Page) {
  const frame = await page.evaluateHandle(() => window.repositoryWatchProbe.holdNextRepositoryFrame());
  return {
    ready: () => expect.poll(() => frame.evaluate((held) => held.ready), tries).toBe(true),
    async release() {
      await frame.evaluate((held) => held.release());
      await expect.poll(() => frame.evaluate((held) => held.finished), tries).toBe(true);
      await frame.dispose();
    },
  };
}

function watchSignals(page: Page, listener: Listener) {
  return page.evaluate(({ origin, machines, repositories }) => {
    const requests = window.repositoryWatchProbe.requests.filter((request) => request.origin === origin);
    return [machines, repositories].map((path) => requests.filter((request) => request.path === path).map(
      (request) => request.signal?.aborted ?? null,
    ));
  }, { origin: listener.origin, machines: procedure.machines, repositories: procedure.repositories });
}

watchTest.describe('workspace waits for both snapshots', () => {
  for (const order of ['machines first', 'repositories first'] as const) {
    watchTest(order, async ({ page, rig }) => {
      const hash = '#/r/' + staleId;
      const trace = await visit(page, rig, hash);
      const site = order === 'machines first' ? rig.first : rig.other;
      await selectAndSignIn(page, site);
      const machines = site.machines.attempts[0]!;
      const expected = { auth: 'in', synced: false, hash, repos: [], sampleCounts: [0, 0, 0, 0, 0, 0], simulation: 0 };
      await expect(busy(page)).toBeVisible();
      expect(await workspace(page)).toMatchObject({ ...expected, machines: [] });
      expect(await snapshot(page)).toBeNull();

      if (order === 'machines first') {
        machines.send(allMachines([worker]));
        await expect.poll(() => workspace(page)).toMatchObject({ machines: [worker.id] });
        expect.soft(await workspace(page), 'The machine snapshot alone cannot open a deep link.').toMatchObject(expected);
        await expect.soft(busy(page)).toBeVisible({ timeout: 1000 });
      }
      await requireBothWatches(site);
      const repositories = site.repositories.attempts[0]!;
      if (order === 'repositories first') {
        repositories.send(allRepositories([initialRepo]));
        await expect.poll(() => snapshot(page), tries).toMatchObject({ ids: [repoId] });
        expect(await workspace(page)).toMatchObject({ ...expected, machines: [], repos: [repoId] });
        await expect(busy(page)).toBeVisible();
        machines.send(allMachines([]));
      } else {
        expect(await snapshot(page)).toBeNull();
        repositories.send(allRepositories([]));
      }

      await expect(busy(page)).toHaveCount(0);
      await expect(liveBadge(page)).toBeVisible();
      await expect(page.getByText(missingRepository, { exact: true })).toBeVisible();
      expect(await workspace(page)).toMatchObject({ ...expected, synced: true, repos: order === 'machines first' ? [] : [repoId] });
      expect(await snapshot(page)).toMatchObject({ ids: order === 'machines first' ? [] : [repoId] });

      await page.reload();
      await expect.poll(() => [site.machines.attempts.length, site.repositories.attempts.length], tries).toEqual([2, 2]);
      await expect(busy(page)).toBeVisible();
      expect(await workspace(page)).toMatchObject({ ...expected, machines: [] });
      site.machines.attempts[1]!.send(allMachines([]));
      site.repositories.attempts[1]!.send(allRepositories([]));
      await expect(page.getByText(missingRepository, { exact: true })).toBeVisible();
      expect(await workspace(page)).toMatchObject({ ...expected, synced: true });
      expect(trace.errors).toEqual([]);
    });
  }
});

watchTest.describe('repository reconnect replaces and removal converges', () => {
  watchTest('saved replaces all facts and machine removal drops exactly the affected folders', async ({ page, rig }) => {
    await visit(page, rig);
    await openWorkspace(page, rig.first);
    const machines = rig.first.machines.attempts[0]!;
    const repositories = rig.first.repositories.attempts[0]!;
    await expect.poll(() => projection(page), tries).toEqual(expect.arrayContaining([
      {
        id: repoId, name: initialRepo.name, main: 'trunk', remote: origin, remoteOmitted: false, firstCommit: commit,
        folders: { [worker.id]: firstFolder.resolvedPath, [survivor.id]: survivingFolder.resolvedPath },
        folderRecords: expect.arrayContaining([firstFolder, secondFolder, survivingFolder]),
      },
      expect.objectContaining({ id: orphanId, remote: undefined, remoteOmitted: false, firstCommit: undefined }),
    ]));
    const originalFolders = (await projection(page)).find((repo) => repo.id === repoId)!.folderRecords;
    expect(originalFolders).toHaveLength(3);
    const reordered = create(RepositorySchema, {
      ...initialFacts, name: 'Watched checkout reordered', folders: [firstFolder, secondFolder, survivingFolder],
    });
    repositories.send(savedRepository(reordered));
    await expect.poll(() => projection(page), tries).toContainEqual(expect.objectContaining({
      id: repoId, name: reordered.name, folderRecords: originalFolders,
    }));

    const replacement = create(RepositorySchema, {
      id: repoId, name: 'Watched checkout after removal', mainBranch: 'next', remoteOmitted: true,
      folders: [survivingFolder],
    });
    machines.send({ change: { case: 'removedId', value: worker.id }, position: { value: 3n } });
    repositories.send(savedRepository(replacement, 4n));
    repositories.send({ change: { case: 'removedId', value: orphanId }, position: { value: 5n } });
    await expect.poll(() => projection(page), tries).toStrictEqual([{
      id: repoId, name: replacement.name, main: 'next', remote: undefined, remoteOmitted: true, firstCommit: undefined,
      folders: { [survivor.id]: survivingFolder.resolvedPath }, folderRecords: [survivingFolder],
    }]);
    await expect(page.getByText(worker.name, { exact: true })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Show details for ' + survivor.name, exact: true })).toBeVisible();
    expect(await snapshot(page)).toMatchObject({ ids: [repoId] });
  });

  watchTest('an aborted watch retains rows until an empty snapshot replaces them', async ({ page, rig }) => {
    await visit(page, rig);
    const site = rig.first;
    await openWorkspace(page, site);
    const opening = await snapshot(page);
    expect(opening).toMatchObject({ openedAt: expect.any(Number), ids: [repoId, orphanId] });
    const timers = () => page.evaluate(() => window.discoveryProbe.timeouts[500] ?? 0);
    const before = await timers();
    site.repositories.attempts[0]!.send(Code.Aborted);
    await expect.poll(timers, tries).toBeGreaterThan(before);
    await expect(liveBadge(page)).toHaveCount(0);
    expect(await snapshot(page), 'A retained list does not represent an initialized attempt.').toBeNull();
    expect(await workspace(page)).toMatchObject({ synced: true, repos: [repoId, orphanId] });
    await expect(page.getByRole('button', { name: 'Show details for ' + worker.name, exact: true })).toBeVisible();

    const machines = site.machines.attempts[0]!;
    machines.send({ position: { value: 2n } });
    const heard = machine(worker.id, 'Worker on the healthy stream');
    machines.send(savedMachine(heard, 3n));
    await expect(page.getByText(heard.name, { exact: true })).toBeVisible();
    await expect(liveBadge(page), 'The healthy sibling cannot make the connection live on its own.').toHaveCount(0);

    await page.clock.runFor(499);
    expect(site.repositories.attempts).toHaveLength(1);
    await page.clock.runFor(1);
    await expect.poll(() => site.repositories.attempts.length, tries).toBe(2);
    expect(site.machines.attempts).toHaveLength(1);

    const replacement = site.repositories.attempts[1]!;
    const delta = await browserFrame(page);
    replacement.send(savedRepository(stale, 4n));
    await delta.ready();
    await delta.release();
    expect(await workspace(page)).toMatchObject({ repos: [repoId, orphanId] });
    expect(await snapshot(page)).toBeNull();
    replacement.send(allRepositories([], 5n));
    await expect.poll(() => projection(page), tries).toEqual([]);
    await expect(liveBadge(page)).toBeVisible();
    const current = await snapshot(page);
    expect(current).toMatchObject({ ids: [] });
    expect(current!.openedAt - opening!.openedAt).toBeGreaterThanOrEqual(500);
  });

  watchTest('refresh opens only the repository watch and rejects an already queued frame from the old attempt', async ({ page, rig }) => {
    await visit(page, rig);
    const site = rig.first;
    await openWorkspace(page, site);
    const original = await snapshot(page);
    expect(original).toMatchObject({ ids: [repoId, orphanId] });
    const queued = await browserFrame(page);
    site.repositories.attempts[0]!.send(savedRepository(stale, 2n));
    await queued.ready();
    await page.clock.runFor(125);
    await page.clock.setSystemTime(new Date('2026-11-28T12:00:00Z'));
    await page.evaluate(() => window.repositoryWatchProbe.bridge!.refreshRepositories());
    await expect.poll(() => site.repositories.attempts.length, tries).toBe(2);
    await expect.poll(() => watchSignals(page, site.server), tries).toEqual([[false], [true, false]]);
    expect(await snapshot(page)).toBeNull();
    expect(await workspace(page)).toMatchObject({ synced: true, repos: [repoId, orphanId] });

    await page.clock.runFor(750);
    site.repositories.attempts[1]!.send(allRepositories([], 3n));
    await expect.poll(() => snapshot(page), tries).toMatchObject({ ids: [] });
    expect((await snapshot(page))!.openedAt - original!.openedAt, 'The elapsed timestamp belongs to the start of the attempt.').toBe(125);
    await queued.release();
    expect(await projection(page), 'An already queued frame from a replaced attempt cannot revive rows.').toEqual([]);
    expect(await snapshot(page)).toMatchObject({ ids: [] });
    await expect(liveBadge(page)).toBeVisible();
  });

  watchTest('clean disconnects retry at 500 milliseconds and back off to eight seconds', async ({ page, rig }) => {
    await visit(page, rig);
    const site = rig.first;
    await openWorkspace(page, site);
    for (const [index, delay] of [500, 1000, 2000, 4000, 8000, 8000].entries()) {
      const timers = () => page.evaluate((ms) => window.discoveryProbe.timeouts[ms] ?? 0, delay);
      const before = await timers();
      site.repositories.attempts[index]!.send('end');
      await expect.poll(timers, tries).toBeGreaterThan(before);
      await page.clock.runFor(delay - 1);
      expect(site.repositories.attempts).toHaveLength(index + 1);
      await page.clock.runFor(1);
      await expect.poll(() => site.repositories.attempts.length, tries).toBe(index + 2);
    }
    expect(await workspace(page)).toMatchObject({ synced: true, repos: [repoId, orphanId] });
    expect(await snapshot(page)).toBeNull();
    await expect(liveBadge(page)).toHaveCount(0);
  });
});

const cleared = {
  synced: false, machines: [], joining: [], repos: [], dialog: null, expansions: [],
};

watchTest.describe('either watch ends the repository session', () => {
  for (const boundary of ['machine watch expiry', 'repository watch expiry (issue 1689 point 3)', 'stalled sign-out', 'server switch'] as const) {
    watchTest(boundary, async ({ page, rig }) => {
      const trace = await visit(page, rig);
      const site = rig.first;
      await openWorkspace(page, site);
      await page.evaluate(() => {
        window.lgtm.act.toggleExpanded('machine:first-worker');
        window.lgtm.act.openDialog({ type: 'rename', title: 'Departing dialog', value: 'First worker' });
      });
      const dialog = page.getByRole('dialog', { name: 'Departing dialog', exact: true });
      await expect(dialog).toBeVisible();
      expect(await workspace(page)).toMatchObject({
        machines: [worker.id, survivor.id], joining: [waiting.id], repos: [repoId, orphanId],
        dialog: 'rename', expansions: ['machine:' + worker.id],
      });

      const signOut = boundary === 'stalled sign-out' ? site.server.hold('signOut') : undefined;
      const discovery = boundary === 'server switch' ? rig.other.server.hold('discovery') : undefined;
      const signOutAction = signOut && await page.evaluateHandle(() => ({ done: window.lgtm.act.signOut() }));
      if (signOut) {
        await signOut.arrived;
      } else if (discovery) {
        await choose(page, rig.other.server.origin);
        await discovery.arrived;
      } else if (boundary === 'machine watch expiry') {
        site.machines.attempts[0]!.send(Code.Unauthenticated);
      } else {
        site.repositories.attempts[0]!.send(Code.Unauthenticated);
      }

      await expect.poll(() => workspace(page), tries).toMatchObject(cleared);
      expect(await snapshot(page)).toBeNull();
      await expect(dialog).toHaveCount(0);
      await expect.poll(() => watchSignals(page, site.server), tries).toEqual([[true], [true]]);
      await expect.poll(() => [site.machines.attempts[0]!.aborted, site.repositories.attempts[0]!.aborted], tries)
        .toEqual([true, true]);

      if (discovery) {
        discovery.release();
        await expect(page.getByLabel('Password', { exact: true })).toBeVisible();
      } else if (signOut) {
        await expect(page.getByLabel('Password', { exact: true })).toBeVisible();
        // A cookie writer on another hostname can finish while the previous sign-out is held.
        await openWorkspace(page, rig.other);
        const selected = await workspace(page);
        expect(selected).toMatchObject({ address: rig.other.server.origin, auth: 'in', synced: true, hash: '#/machines' });
        signOut.release();
        await signOutAction!.evaluate((action) => action.done);
        await signOutAction!.dispose();
        expect(await workspace(page)).toEqual(selected);
        expect(await watchSignals(page, rig.other.server)).toEqual([[false], [false]]);
        await expect(liveBadge(page)).toBeVisible();
      } else {
        expect(await workspace(page)).toMatchObject({ auth: 'expired' });
        await expect(page.getByLabel('Password', { exact: true })).toBeVisible();
        await page.clock.runFor(9000);
        expect(await watchSignals(page, site.server)).toEqual([[true], [true]]);
      }
      expect(trace.errors).toEqual([]);
    });
  }
});

async function approveInBothOrders(
  page: Page,
  rig: { web: string; first: WatchListener; other: WatchListener },
  verify: (browser: Page, beforeWatch: boolean) => Promise<void>,
) {
  for (const order of ['watch first', 'approval first'] as const) {
    await test.step(order, async () => {
      const browser = order === 'watch first' ? page : await page.context().newPage();
      try {
        const site = order === 'watch first' ? rig.first : rig.other;
        await visit(browser, rig);
        await selectAndSignIn(browser, site);
        const machines = site.machines.attempts[0]!;
        machines.send(allMachines([worker, waiting]));
        // These issue checks also run before the repository watch is built.
        let released = false;
        await expect.poll(async () => {
          const repositories = site.repositories.attempts[0];
          if (!released && repositories) {
            repositories.send(allRepositories([]));
            released = true;
          }
          return browser.getByRole('button', { name: 'Add machine', exact: true }).isVisible();
        }, tries).toBe(true);
        await browser.getByRole('button', { name: 'Add machine', exact: true }).click();
        const dialog = browser.getByRole('dialog', { name: 'Add a machine', exact: true });
        const name = dialog.getByLabel('Name', { exact: true });
        await expect.poll(async () => {
          await browser.clock.runFor(16);
          return name.isVisible();
        }, tries).toBe(true);
        expect(site.server.calls().filter((call) => call.method === 'create').map((call) => call.cookie)).toEqual(['accepted']);
        await name.fill('Name from approval');
        const approval = site.server.hold('approve');
        const confirm = dialog.getByRole('button', { name: 'Confirm Name from approval', exact: true });
        await expect(confirm).toBeEnabled();
        await confirm.click();
        await approval.arrived;
        await expect(dialog).toBeVisible();
        const sendWatch = async () => {
          machines.send(savedMachine(watchedApproval));
          await expect(browser.getByText(watchedApproval.name, { exact: true })).toBeVisible();
        };
        if (order === 'watch first') {
          await sendWatch();
          await expect(dialog).toBeVisible();
        }
        approval.release();
        await expect(dialog).toHaveCount(0);
        if (order === 'approval first') {
          await verify(browser, true);
          await sendWatch();
        }
        await verify(browser, false);
      } finally {
        if (browser !== page) await browser.close();
      }
    });
  }
}

watchTest('issue 1689 point 1: completing the open approval leaves machine records to the watch', async ({ page, rig }) => {
  await approveInBothOrders(page, rig, async (browser, beforeWatch) => {
    const state = await browser.evaluate(() => ({
      rows: window.lgtm.store.state.machines.map((item) => ({
        id: item.id, name: item.name, status: item.status, limit: item.slots.limit,
      })),
      joining: window.lgtm.store.state.joining.map((item) => item.id),
    }));
    const original = { id: worker.id, name: worker.name, status: 'online', limit: 0 };
    const watched = { id: waiting.id, name: watchedApproval.name, status: 'offline', limit: 9 };
    expect(state).toEqual({ rows: beforeWatch ? [original] : [original, watched], joining: beforeWatch ? [waiting.id] : [] });
  });
});

watchTest('issue 1689 point 2: the approved row opens in both arrival orders', async ({ page, rig }) => {
  await approveInBothOrders(page, rig, async (browser, beforeWatch) => {
    if (beforeWatch) return;
    const row = browser.getByRole('row').filter({ has: browser.getByText(watchedApproval.name, { exact: true }) });
    await expect.soft(row).toHaveAttribute('aria-expanded', 'true', { timeout: 1000 });
  });
});
