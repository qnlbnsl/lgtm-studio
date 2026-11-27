import { create } from '@bufbuild/protobuf';
import { ClientSchema, ClientState } from '@lgtm/api/clients';
import { ProjectService } from '@lgtm/api/projects';
import { expect, type Page } from '@playwright/test';
import { observeRepositorySession, observeWebsite, serveRepositoryDiscovery, serveWebsite, test } from './support/repository-server';

const watchedWorker = (id: string, version?: string) => create(ClientSchema, {
  id, name: id, state: ClientState.ONLINE, agentLimit: 2, harnessesScanned: true,
  harnesses: [version === undefined ? { harness: 'codex' } : { harness: 'codex', version }],
});
type Discovery = Awaited<ReturnType<typeof serveRepositoryDiscovery>>;

function navigate(page: Page, route: string) {
  return page.evaluate((hash) => { location.hash = hash; }, route);
}

async function selectServer(page: Page, server: Discovery) {
  await page.getByLabel('Server address', { exact: true }).fill(server.origin);
  await page.getByRole('button', { name: 'Connect', exact: true }).click();
  await server.discoveryRequested;
  server.releaseDiscovery();
  await page.getByLabel('Password', { exact: true }).fill('browser fixture input');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await server.machinesRequested;
  server.releaseMachines();
  await expect.poll(() => page.evaluate(() => {
    const s = window.lgtm.store.state;
    return s.server.repositoryMode && s.server.synced && s.session.status === 'in';
  })).toBe(true);
}

async function leaveServer(page: Page) {
  await page.evaluate(() => window.lgtm.act.signOut());
  await page.getByRole('button', { name: /^Change the server\./ }).click();
}

function workflowStorage(page: Page) {
  return page.evaluate(() => ['lgtm-mock:workflows', 'lgtm-mock:workflow-history']
    .map((key) => localStorage.getItem(key) !== null));
}

test('post-1b 2: a held sample workflow clone cannot write after switching servers', async ({ page, website }) => {
  const web = await serveWebsite(website);
  try {
    const real = await serveRepositoryDiscovery(web.origin, { services: [ProjectService.typeName] }, [
      watchedWorker('Review worker', '1.2.3'),
    ]);
    try {
      const trace = await observeRepositorySession(page, [web.origin, real.origin]);
      await page.goto(web.origin + '/#/home');
      await page.getByRole('button', { name: 'Use sample data', exact: true }).click();
      await expect.poll(() => page.evaluate(() => {
        const s = window.lgtm.store.state;
        return s.session.status === 'in' && s.server.synced && !s.server.repositoryMode;
      })).toBe(true);
      await navigate(page, '/workflows');
      const buildLoop = page.getByRole('heading', { name: 'Build loop', exact: true });
      await expect.poll(async () => {
        await page.clock.runFor(128);
        return buildLoop.isVisible();
      }).toBe(true);
      await page.getByRole('button', { name: 'Clone', exact: true }).click();
      const clone = page.getByRole('dialog', { name: 'Clone workflow', exact: true });
      const name = 'Clone from the departed workspace';
      await clone.getByLabel('Workflow name', { exact: true }).fill(name);
      await page.evaluate((origin) => {
        window.repositorySessionProbe.reply = window.repositorySessionProbe.hold(origin, 'build-loop');
      }, web.origin);
      await clone.getByRole('button', { name: 'Create copy', exact: true }).click();
      await expect.poll(() => page.evaluate(() => window.repositorySessionProbe.reply?.ready)).toBe(true);
      expect(await workflowStorage(page)).toEqual([false, false]);

      await leaveServer(page);
      await selectServer(page, real);
      expect(await workflowStorage(page)).toEqual([false, false]);
      await page.evaluate(() => window.repositorySessionProbe.reply!.release());
      await navigate(page, '/workflows');
      await page.clock.runFor(128);
      await expect(page).toHaveURL(/#\/workflows$/);
      expect.soft(await workflowStorage(page), 'The released sample clone leaves both storage records absent.').toEqual([false, false]);
      await expect(page.getByRole('button', { name, exact: true })).toHaveCount(0);
      expect(trace.errors).toEqual([]);
    } finally {
      await real.stop();
    }
  } finally {
    await web.stop();
  }
});

test('post-1b 4: the Codex row uses the first available watched version', async ({ page, website }) => {
  const web = await serveWebsite(website);
  const alternatives: Discovery[] = [];
  try {
    const cases = [
      { first: '1.2.3', second: '9.8.7' },
      { first: undefined, second: '9.8.7' },
    ];
    for (const { first, second } of cases) {
      alternatives.push(await serveRepositoryDiscovery(web.origin, { services: [ProjectService.typeName] }, [
        watchedWorker('First watched worker', first),
        watchedWorker('Second watched worker', second),
      ]));
    }
    const trace = await observeWebsite(page, [web.origin, ...alternatives.map((server) => server.origin)]);
    await page.goto(web.origin + '/#/home');
    for (const [index, { first, second }] of cases.entries()) {
      await test.step(first ? 'Both workers report versions' : 'The first worker omits its version', async () => {
        const server = alternatives[index]!;
        await selectServer(page, server);
        expect(await page.evaluate(() => window.lgtm.store.state.machines.map(({ harnesses }) => harnesses?.codex)))
          .toEqual([first ?? null, second]);
        await navigate(page, '/machines');
        const detail = page.locator('.svc-detail');
        const installed = detail.locator('h3:text-is("Harnesses") + ul').getByRole('listitem');
        for (const [name, version] of [['First watched worker', first], ['Second watched worker', second]] as const) {
          await page.getByRole('button', { name: `Show details for ${name}`, exact: true }).click();
          await expect(detail.getByRole('heading', { name: 'Harnesses', exact: true })).toBeVisible();
          await expect(installed).toHaveCount(version === undefined ? 0 : 1);
          if (version !== undefined) {
            await expect(installed.getByText('Codex', { exact: true })).toBeVisible();
            await expect(installed.getByText(version, { exact: true })).toBeVisible();
          }
          await page.getByRole('button', { name: `Hide details for ${name}`, exact: true }).click();
          await expect(detail).toHaveCount(0);
        }
        if (index === 0) await leaveServer(page);
      });
    }
    expect(trace.errors).toEqual([]);
  } finally {
    await Promise.all([web.stop(), ...alternatives.map((server) => server.stop())]);
  }
});
