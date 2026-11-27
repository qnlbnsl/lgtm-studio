import { create } from '@bufbuild/protobuf';
import { ClientSchema, ClientState } from '@lgtm/api/clients';
import { ProjectService } from '@lgtm/api/projects';
import { expect, type Page } from '@playwright/test';
import {
  observeWebsite, procedure, serveRepositoryDiscovery, serveWebsite, test,
} from './support/repository-server';
import { serveRepositoryRemount } from './support/repository-remount';

const workerId = 'review-worker';
const watchedVersion = '1.2.3';
const installDelay = 2200;
const commitMessageDelay = 900;

const workspaceTest = test.extend<{
  workspace: { web: string; server: Awaited<ReturnType<typeof serveRepositoryDiscovery>> };
}>({
  workspace: async ({ website }, use) => {
    const web = await serveWebsite(website);
    try {
      const server = await serveRepositoryDiscovery(web.origin, { services: [ProjectService.typeName] }, [
        create(ClientSchema, {
          id: workerId, name: 'Review worker', state: ClientState.ONLINE, agentLimit: 3,
          harnessesScanned: true, harnesses: [{ harness: 'codex', version: watchedVersion }],
        }),
      ]);
      try { await use({ web: web.origin, server }); } finally { await server.stop(); }
    } finally {
      await web.stop();
    }
  },
});

async function navigate(page: Page, path: string) {
  await page.evaluate((value) => { location.hash = value; }, path);
}

async function chooseSample(page: Page) {
  await page.getByRole('button', { name: 'Use sample data', exact: true }).click();
}

async function expectSample(page: Page) {
  await expect.poll(() => page.evaluate(() => {
    const state = window.lgtm.store.state;
    return state.server.synced && !state.server.repositoryMode && state.session.status === 'in' &&
      state.machines.length > 0 && state.projects.some((project) => project.id === 'console');
  })).toBe(true);
}

async function signIn(page: Page) {
  await page.getByLabel('Password', { exact: true }).fill('browser fixture input');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
}

async function leaveServer(page: Page) {
  await page.evaluate(() => window.lgtm.act.signOut());
  await page.getByRole('button', { name: /^Change the server\./ }).click();
}

async function beginSwitch(page: Page, server: Awaited<ReturnType<typeof serveRepositoryDiscovery>>) {
  await leaveServer(page);
  await page.getByLabel('Server address', { exact: true }).fill(server.origin);
  await page.getByRole('button', { name: 'Connect', exact: true }).click();
  await server.discoveryRequested;
}

async function openWatchedWorkspace(page: Page, server: Awaited<ReturnType<typeof serveRepositoryDiscovery>>) {
  server.releaseDiscovery();
  await signIn(page);
  await server.machinesRequested;
  server.releaseMachines();
  await expect.poll(() => page.evaluate(() => {
    const state = window.lgtm.store.state;
    return state.session.status === 'in' && state.server.repositoryMode && state.server.synced
      ? state.machines.map(({ id, harnesses }) => ({ id, harnesses })) : [];
  })).toEqual([{ id: workerId, harnesses: { codex: watchedVersion } }]);
}

function scheduledCallbacks(page: Page, delay: number) {
  return page.evaluate((ms) => window.discoveryProbe.timeouts[ms] ?? 0, delay);
}

function geminiInstalled(page: Page) {
  return page.getByRole('region', { name: 'Gemini CLI', exact: true }).getByText(/^Installed(?: · .+)?$/);
}

async function scheduleGeminiInstall(page: Page) {
  await navigate(page, '/harnesses');
  const before = await scheduledCallbacks(page, installDelay);
  await page.getByRole('button', { name: 'Add harness', exact: true }).click();
  await page.getByRole('menuitem', { name: 'Gemini CLI', exact: true }).click();
  await page.getByRole('dialog', { name: 'Install Gemini CLI', exact: true })
    .getByRole('button', { name: /^Install on / }).click();
  expect(await scheduledCallbacks(page, installDelay)).toBeGreaterThan(before);
  expect(await page.evaluate(() => window.lgtm.store.state.ui.installing)).toBe('gemini');
  await expect(geminiInstalled(page)).toHaveCount(0);
}

workspaceTest('post-1 1: a delayed Git message leaves the watched workspace empty', async ({ page, workspace }) => {
  const { web, server } = workspace;
  const trace = await observeWebsite(page, [web, server.origin]);
  await page.goto(web + '/#/home');
  await chooseSample(page);
  await expectSample(page);
  await navigate(page, '/p/console/git');
  const generate = page.getByRole('button', { name: 'Generate', exact: true });
  const before = await scheduledCallbacks(page, commitMessageDelay);
  await generate.click();
  expect(await scheduledCallbacks(page, commitMessageDelay)).toBeGreaterThan(before);

  await beginSwitch(page, server);
  await openWatchedWorkspace(page, server);
  const gitState = () => page.evaluate(() => ({
    worktrees: window.lgtm.store.state.worktrees.length,
    gitViews: Object.keys(window.lgtm.store.state.gitUi),
  }));
  const emptyGit = { worktrees: 0, gitViews: [] };
  expect(await gitState()).toEqual(emptyGit);
  await page.clock.runFor(commitMessageDelay);
  expect(await gitState(), 'The old Git callback leaves the watched workspace empty.').toEqual(emptyGit);
  expect(trace.errors).toEqual([]);
});

for (const timing of ['before switching', 'while discovery is held'] as const) {
  workspaceTest(`post-1 2: an install completed ${timing} stays in sample`, async ({ page, workspace }) => {
    const { web, server } = workspace;
    const trace = await observeWebsite(page, [web, server.origin]);
    await page.goto(web + '/#/home');
    await chooseSample(page);
    await expectSample(page);
    await scheduleGeminiInstall(page);
    if (timing === 'before switching') {
      await page.clock.runFor(installDelay);
      await expect(geminiInstalled(page)).toBeVisible();
    }

    await beginSwitch(page, server);
    if (timing === 'while discovery is held') {
      await page.clock.runFor(installDelay);
      expect(server.calls).toEqual([{ path: procedure.discovery, signedIn: false }]);
    }
    await openWatchedWorkspace(page, server);
    await navigate(page, '/machines');
    await page.getByRole('button', { name: 'Show details for Review worker', exact: true }).click();
    const installed = page.locator('.svc-detail h3:text-is("Harnesses") + ul').getByRole('listitem');
    await expect(installed, 'Only the installation from the machine watch is displayed.')
      .toHaveCount(1);
    await expect(installed.getByText('Codex', { exact: true })).toBeVisible();
    await expect(installed.getByText(watchedVersion, { exact: true })).toBeVisible();
    expect(trace.errors).toEqual([]);
  });
}

test('post-1 3: discovery from an unmounted view leaves the legacy engine mounted', async ({ page }) => {
  const web = await serveRepositoryRemount();
  try {
    const olderServer = await serveRepositoryDiscovery(web.origin, {});
    try {
      const trace = await observeWebsite(page, [web.origin, olderServer.origin]);
      await page.goto(web.origin + '/#/home');
      await page.evaluate((address) => window.repositoryRemount.startDiscovery(address), olderServer.origin);
      await olderServer.discoveryRequested;
      await page.evaluate(() => window.repositoryRemount.mountLegacy());
      const legacyStore = await page.evaluateHandle(() => window.lgtm.store.state);
      const state = () => page.evaluate((legacy) => {
        const current = window.lgtm.store.state;
        return {
          sameStore: current === legacy, session: current.session.status,
          projects: current.projects.length, guarded: Object.isFrozen(current.projects),
          simulationIntervals: window.discoveryProbe.simulationIntervals(),
        };
      }, legacyStore);
      const expected = { sameStore: true, session: 'in', projects: 0, guarded: true, simulationIntervals: 0 };
      expect(await state()).toEqual(expected);

      olderServer.releaseDiscovery();
      await page.evaluate(() => window.repositoryRemount.discoveryFinished());
      expect(await state(), 'The old discovery cannot replace the guarded legacy store.').toEqual(expected);
      expect(trace.errors).toEqual([]);
    } finally {
      await olderServer.stop();
    }
  } finally {
    await web.stop();
  }
});

workspaceTest('post-1 4: a delayed install cannot change a fresh sample workspace', async ({ page, workspace }) => {
  const { web, server } = workspace;
  const trace = await observeWebsite(page, [web, server.origin]);
  await page.goto(web + '/#/home');
  await chooseSample(page);
  await expectSample(page);
  const firstSample = await page.evaluateHandle(() => window.lgtm.store.state);
  await scheduleGeminiInstall(page);

  await beginSwitch(page, server);
  await openWatchedWorkspace(page, server);
  await leaveServer(page);
  await server.machinesAborted;
  await chooseSample(page);
  await signIn(page);
  await expectSample(page);
  expect(await page.evaluate((first) => window.lgtm.store.state === first, firstSample)).toBe(false);
  await navigate(page, '/harnesses');
  await expect(page.getByRole('heading', { name: 'Harnesses', exact: true })).toBeVisible();
  await expect(geminiInstalled(page)).toHaveCount(0);
  const inventory = () => page.evaluate(() => ({
    installing: window.lgtm.store.state.ui.installing ?? null,
    machines: window.lgtm.store.state.machines.map(({ id, harnesses }) => ({ id, harnesses })),
  }));
  const before = await inventory();
  await page.clock.runFor(installDelay);
  expect.soft(await inventory(), 'The fresh sample machine inventory is unchanged.').toEqual(before);
  await expect(geminiInstalled(page), 'The fresh sample does not display the old installation.').toHaveCount(0);
  expect(trace.errors).toEqual([]);
});
