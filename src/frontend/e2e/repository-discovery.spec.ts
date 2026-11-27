import { create } from '@bufbuild/protobuf';
import { ClientSchema, ClientState } from '@lgtm/api/clients';
import { expect, type Page } from '@playwright/test';
import {
  observeWebsite, procedure, serveRepositoryDiscovery, serveWebsite, startStandaloneMock, test,
} from './support/repository-server';

const projectService = 'lgtm.v1.ProjectService';
const otherService = 'lgtm.v1.FutureService';
const compatibilityId = 'console';
const workerId = 'discovery-worker';
const simulationPeriod = 4200;

const listSlices = [
  'machines', 'joining', 'repos', 'projects', 'teams', 'agents', 'requests', 'tasks', 'epics', 'sprints',
  'channels', 'posts', 'subscriptions', 'notifications', 'worktrees',
] as const;
const countSlices = [
  ...listSlices, 'threads', 'define', 'workflow', 'design', 'mocks', 'usage rows', 'usage limits',
  'usage findings', 'terminal sessions',
];

async function workspaceCounts(page: Page) {
  return page.evaluate((keys) => {
    const state = window.lgtm.store.state;
    return {
      ...Object.fromEntries(keys.map((key) => [key, state[key]?.length ?? 0])),
      threads: Object.keys(state.threads ?? {}).length,
      define: Object.keys(state.define ?? {}).length,
      workflow: Object.keys(state.workflow ?? {}).length,
      design: Object.keys(state.design ?? {}).length,
      mocks: Object.keys(state.mocks ?? {}).length,
      'usage rows': state.usage?.rows?.length ?? 0,
      'usage limits': Object.keys(state.usage?.limits ?? {}).length,
      'usage findings': state.usage?.review?.findings?.length ?? 0,
      'terminal sessions': Object.keys(state.terminal?.byProject ?? {}).length,
    };
  }, listSlices);
}

function emptyCounts(watchedMachines = 0) {
  return { ...Object.fromEntries(countSlices.map((name) => [name, 0])), machines: watchedMachines };
}

async function activeIntervals(page: Page) {
  return page.evaluate(() => window.discoveryProbe.simulationIntervals());
}

async function connect(page: Page, origin: string) {
  await page.getByLabel('Server address', { exact: true }).fill(origin);
  await page.getByRole('button', { name: 'Connect', exact: true }).click();
}

async function signIn(page: Page) {
  // Both sample and controlled servers accept arbitrary fixture input.
  await page.getByLabel('Password', { exact: true }).fill('browser fixture input');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
}

async function signOut(page: Page) {
  // The same action backs the sidebar button at both viewport widths.
  await page.evaluate(() => window.lgtm.act.signOut());
  await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible();
}

async function changeServer(page: Page) {
  await page.getByRole('button', { name: /^Change the server\./ }).click();
}

async function navigate(page: Page, hash: string) {
  await page.evaluate((value) => { location.hash = value; }, hash);
}

async function expectSample(page: Page) {
  await expect.poll(() => page.evaluate((project) => {
    const state = window.lgtm.store.state;
    return state.server.synced && state.session.status === 'in' &&
      state.machines.length > 0 && state.projects.some((record) => record.id === project);
  }, compatibilityId)).toBe(true);
}

async function observeTrackRepository(page: Page) {
  const navigation = page.getByRole('button', { name: 'Open navigation', exact: true });
  if (await navigation.isVisible()) await navigation.click();
  await page.getByRole('button', { name: 'Add a repository', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'Add a repository', exact: true });
  await expect(dialog).toBeVisible();
  const search = dialog.getByRole('textbox', { name: 'Search the repositories, or paste an HTTPS or SSH link' });
  await search.fill('https://example.invalid/website/check-out.git');
  await dialog.getByRole('button', { name: 'Add', exact: true }).click();
  await expect.poll(() => page.evaluate(() => window.lgtm.store.state.repos.some((r) => r.id === 'check-out'))).toBe(true);
  await expect(page).toHaveURL(/#\/r\/check-out(?:\/|$)/);
}

async function totalTurns(page: Page) {
  return page.evaluate(() => window.lgtm.store.state.agents.reduce((sum, agent) => sum + agent.turns, 0));
}

async function chooseInPageSample(page: Page) {
  await page.getByRole('button', { name: 'Use sample data', exact: true }).click();
}

test.describe('discovery chooses the workspace and simulation', () => {
  const discoveryCases = [
    { name: 'exact entry first in a mixed list', services: [projectService, otherService], real: true },
    { name: 'exact entry last in a mixed list', services: [otherService, projectService], real: true },
    { name: 'duplicate exact entry', services: [projectService, projectService], real: true },
    { name: 'unknown entry only', services: [otherService], real: false },
    { name: 'lookalike entries only', services: ['lgtm.v1.projectservice', 'lgtm.v1.ProjectServiceExtra'], real: false },
    { name: 'absent services', real: false },
    { name: 'empty services', services: [], real: false },
    { name: 'sample with the exact entry', services: [projectService], sample: true, real: false },
  ];

  for (const scenario of discoveryCases) {
    test(scenario.name, async ({ page, website }) => {
      const web = await serveWebsite(website);
      const server = await serveRepositoryDiscovery(web.origin, {
        services: scenario.services,
        sample: scenario.sample,
      });
      try {
        const trace = await observeWebsite(page, [web.origin, server.origin]);
        await page.goto(web.origin + '/?data=worst#/home');
        await connect(page, server.origin);
        await server.discoveryRequested;
        expect.soft(await workspaceCounts(page), 'The discovery reply is still held, so no workspace records exist.').toEqual(emptyCounts());
        expect.soft(await activeIntervals(page), 'The discovery reply is still held, so simulation has not started.').toBe(0);

        server.releaseDiscovery();
        await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible();
        const isReal = await page.evaluate(() => !!(window.lgtm.store.state.server as { repositoryMode?: boolean }).repositoryMode);
        expect.soft(isReal, 'The advertised capability selects the correct workspace.').toBe(scenario.real);
        if (scenario.real) {
          expect.soft(await workspaceCounts(page), 'The real workspace has no sample collections.').toEqual(emptyCounts());
          expect.soft(await activeIntervals(page), 'The real workspace has no simulation interval.').toBe(0);
        } else {
          expect(await page.evaluate(() => window.lgtm.store.state.projects.some((record) => record.id === 'console'))).toBe(true);
          expect(await activeIntervals(page)).toBe(1);
        }
        expect(server.calls).toEqual([{ path: procedure.discovery, signedIn: false }]);
        expect(trace.errors).toEqual([]);
      } finally {
        await server.stop();
        await web.stop();
      }
    });
  }

  for (const invalid of [{ product: 'unrelated-product' }, { contract: 2 }]) {
    test(`validates ${'product' in invalid ? 'product' : 'revision'} before loading a workspace`, async ({ page, website }) => {
      const web = await serveWebsite(website);
      const server = await serveRepositoryDiscovery(web.origin, { services: [projectService], ...invalid });
      try {
        await observeWebsite(page, [web.origin, server.origin]);
        await page.goto(web.origin + '/#/home');
        server.releaseDiscovery();
        await connect(page, server.origin);
        await expect.poll(() => page.evaluate(() => window.lgtm.store.state.server.status)).toBe('unreachable');
        expect(await workspaceCounts(page)).toEqual(emptyCounts());
        expect(await activeIntervals(page)).toBe(0);
      } finally {
        await server.stop();
        await web.stop();
      }
    });
  }

  test('the in-page address takes precedence over discovery flags', async ({ page, website }) => {
    const web = await serveWebsite(website);
    try {
      const trace = await observeWebsite(page, [web.origin]);
      await page.goto(web.origin + '/#/home');
      await expect(page.getByLabel('Server address', { exact: true })).toBeVisible();
      expect.soft(await activeIntervals(page)).toBe(0);
      await page.evaluate((service) => {
        window.discoveryProbe.localDiscovery = { sample: false, services: [service] };
      }, projectService);
      await chooseInPageSample(page);
      await expectSample(page);
      expect(await activeIntervals(page)).toBe(1);
      expect(trace.calls).toEqual([]);
    } finally {
      await web.stop();
    }
  });

  test('a worst-case reload still shows only watched records with in-memory drafts', async ({ page, website }) => {
    const web = await serveWebsite(website);
    const server = await serveRepositoryDiscovery(web.origin, { services: [projectService] }, [create(ClientSchema, {
      id: workerId, name: 'Discovery worker', state: ClientState.ONLINE, agentLimit: 3,
    })]);
    try {
      const trace = await observeWebsite(page, [web.origin, server.origin]);
      await page.goto(web.origin + '/?data=worst#/home');
      await connect(page, server.origin);
      server.releaseDiscovery();
      await signIn(page);
      await server.machinesRequested;
      server.releaseMachines();
      const machines = () => page.evaluate(() => window.lgtm?.store.state.machines.map((machine) => machine.id));
      await expect.poll(machines).toEqual([workerId]);
      const draft = await page.evaluate(() => {
        const { act, store } = window.lgtm;
        act.setPref('theme', 'dark');
        act.setDraft('thread:discovery-fixture', 'in-memory draft');
        return {
          kept: store.state.drafts['thread:discovery-fixture'] === 'in-memory draft',
          stored: Object.values(sessionStorage).some((value) => value.includes('in-memory draft')),
        };
      });
      expect.soft(draft).toEqual({ kept: true, stored: false });
      await page.reload();
      await expect.poll(machines).toEqual([workerId]);
      expect.soft(await workspaceCounts(page)).toEqual(emptyCounts(1));
      expect.soft(await activeIntervals(page)).toBe(0);
      const settings = await page.evaluate(() => ({
        hasDraft: !!window.lgtm.store.state.drafts['thread:discovery-fixture'],
        theme: window.lgtm.store.state.prefs.theme,
      }));
      expect.soft(settings).toEqual({ hasDraft: false, theme: 'dark' });
      expect(trace.errors).toEqual([]);
    } finally {
      await server.stop();
      await web.stop();
    }
  });

  for (const mode of ['in-page sample', 'standalone mock', 'older server'] as const) {
    test(`${mode} keeps sign-in, sample work and simulation`, async ({ page, website }) => {
      const web = await serveWebsite(website);
      try {
        const older = mode === 'older server'
          ? await serveRepositoryDiscovery(web.origin, {}, [
              create(ClientSchema, { id: 'studio', name: 'Studio', state: ClientState.ONLINE, hostsServer: true, agentLimit: 3 }),
              create(ClientSchema, { id: 'buildbox', name: 'Buildbox', state: ClientState.ONLINE, agentLimit: 3 }),
              create(ClientSchema, { id: 'laptop', name: 'Laptop', state: ClientState.OFFLINE, agentLimit: 3 }),
            ])
          : undefined;
        const server = older ?? (mode === 'standalone mock' ? await startStandaloneMock(web.origin) : undefined);
        try {
          older?.releaseDiscovery();
          older?.releaseMachines();
          const trace = await observeWebsite(page, server ? [web.origin, server.origin] : [web.origin]);
          await page.addInitScript(() => localStorage.setItem('lgtm-mock:session', JSON.stringify('out')));
          await page.goto(web.origin + '/#/home');
          await expect(page.getByLabel('Server address', { exact: true })).toBeVisible();
          expect.soft(await workspaceCounts(page), 'The chooser starts without a workspace.').toEqual(emptyCounts());

          if (server) await connect(page, server.origin);
          else await chooseInPageSample(page);
          await signIn(page);
          await expectSample(page);

          await navigate(page, '/p/console');
          const breadcrumbs = page.getByRole('navigation', { name: 'Where you are' });
          await expect(breadcrumbs.getByText('Agent console', { exact: true })).toBeVisible();
          await navigate(page, '/home');
          await expect(breadcrumbs.getByText('Home', { exact: true })).toBeVisible();
          await observeTrackRepository(page);
          expect(await activeIntervals(page)).toBe(1);
          const previousTurns = await totalTurns(page);
          await page.clock.runFor(simulationPeriod);
          expect(await totalTurns(page)).toBe(previousTurns + 1);

          await page.evaluate(() => window.lgtm.act.retryService());
          await expectSample(page);
          expect(await page.evaluate(() => window.lgtm.store.state.repos.some((r) => r.id === 'check-out'))).toBe(true);
          expect(await activeIntervals(page)).toBe(1);
          await signOut(page);
          expect(await page.evaluate(() => window.lgtm.store.state.session.status)).toBe('out');
          expect(trace.errors).toEqual([]);

          const expected = [procedure.discovery, procedure.signIn, procedure.machines];
          if (!server) {
            const localCalls = await page.evaluate(() => window.discoveryProbe.calls);
            expect(localCalls.slice(0, 3)).toEqual(expected);
            expect(localCalls.filter((path) => path.startsWith('/' + projectService + '/'))).toEqual([]);
            expect(trace.calls).toEqual([]);
          } else {
            expect(trace.calls.slice(0, 3)).toEqual(expected);
            expect(trace.calls.filter((path) => path.startsWith('/' + projectService + '/'))).toEqual([]);
          }
        } finally {
          await server?.stop();
        }
      } finally {
        await web.stop();
      }
    });
  }

  test('switching workspaces discards delayed sample writes and restarts one interval', async ({ page, website }) => {
    const web = await serveWebsite(website);
    const server = await serveRepositoryDiscovery(web.origin, { services: [projectService] }, [create(ClientSchema, {
      id: workerId, name: 'Discovery worker', state: ClientState.ONLINE, agentLimit: 3,
      harnessesScanned: true, harnesses: [{ harness: 'codex', version: '1.2.3' }],
    })]);
    try {
      const trace = await observeWebsite(page, [web.origin, server.origin]);
      await page.goto(web.origin + '/?data=worst#/home');
      await chooseInPageSample(page);
      await expectSample(page);
      expect(await activeIntervals(page)).toBe(1);
      await page.evaluate(() => window.lgtm.act.setPref('theme', 'dark'));

      const scheduledBefore = await page.evaluate(() => ({ ...window.discoveryProbe.timeouts }));
      const answered = await page.evaluate(() => {
        const { act, store } = window.lgtm;
        const request = store.state.requests.find((record) => record.kind === 'question' && record.status === 'open')!;
        act.answerRequest(request.id, null, 'Check delayed request response');
        return request.status === 'answered';
      });
      expect(answered).toBe(true);
      await navigate(page, '/p/console/message-board/ch-console-leads');
      const composer = page.locator('.board-composer textarea');
      await composer.fill('Check delayed board responses');
      await composer.press('Enter');
      expect(await page.evaluate(() => window.lgtm.store.state.posts.some((post) => post.text === 'Check delayed board responses'))).toBe(true);

      await navigate(page, '/harnesses');
      await page.getByRole('button', { name: 'Add harness', exact: true }).click();
      await page.getByRole('menuitem', { name: 'Gemini CLI', exact: true }).click();
      await page.getByRole('dialog', { name: 'Install Gemini CLI' }).getByRole('button', { name: /^Install on / }).click();
      expect(await page.evaluate(() => window.lgtm.store.state.ui.installing)).toBe('gemini');
      expect(await page.evaluate(() => window.discoveryProbe.harnessInstalled!('gemini'))).toBe(false);
      const scheduledAfter = await page.evaluate(() => ({ ...window.discoveryProbe.timeouts }));
      for (const [action, delay] of [['request reply', 1400], ['canned post', 1700], ['harness install', 2200]] as const) {
        expect(scheduledAfter[delay] ?? 0, `The ${action} callback is staged.`).toBeGreaterThan(scheduledBefore[delay] ?? 0);
      }

      await signOut(page);
      await changeServer(page);
      await connect(page, server.origin);
      await server.discoveryRequested;
      server.releaseDiscovery();
      await signIn(page);
      await server.machinesRequested;
      server.releaseMachines();
      const inventory = () => page.evaluate(() => window.lgtm.store.state.machines.map(({ id, harnesses }) => ({ id, harnesses })));
      const watched = [{ id: workerId, harnesses: { codex: '1.2.3' } }];
      await expect.poll(inventory).toEqual(watched);
      expect(server.calls.filter((call) => call.path !== procedure.repositories).slice(0, 3)).toEqual([
        { path: procedure.discovery, signedIn: false },
        { path: procedure.signIn, signedIn: false },
        { path: procedure.machines, signedIn: true },
      ]);

      const clockErrors = await page.clock.runFor(simulationPeriod * 2).then(() => 0, () => 1);
      expect.soft(await workspaceCounts(page), 'Delayed sample actions leave real records empty.').toEqual(emptyCounts(1));
      expect.soft(await inventory(), 'The watched harness inventory is unchanged.').toEqual(watched);
      expect.soft(await page.evaluate(() => window.discoveryProbe.harnessInstalled!('gemini')), 'The shared catalog is unchanged.').toBe(false);
      expect.soft({ clockErrors, pageErrors: trace.errors }, 'No sample callback raised an asynchronous page error.')
        .toEqual({ clockErrors: 0, pageErrors: [] });
      expect.soft(await activeIntervals(page), 'No sample interval survives the switch.').toBe(0);
      expect.soft(await page.evaluate(() => window.lgtm.store.state.prefs.theme)).toBe('dark');

      await signOut(page);
      await server.machinesAborted;
      await changeServer(page);
      await chooseInPageSample(page);
      await signIn(page);
      await expectSample(page);
      expect(await activeIntervals(page), 'Returning to sample starts exactly one interval.').toBe(1);
      const previousTurns = await totalTurns(page);
      await page.clock.runFor(simulationPeriod);
      expect(await totalTurns(page)).toBe(previousTurns + 1);
    } finally {
      await server.stop();
      await web.stop();
    }
  });
});
