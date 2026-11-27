import { create } from '@bufbuild/protobuf';
import { timestampFromMs } from '@bufbuild/protobuf/wkt';
import { ClientSchema, ClientService, ClientState } from '@lgtm/api/clients';
import { ProjectService } from '@lgtm/api/projects';
import { expect, type Page } from '@playwright/test';
import {
  observeWebsite, serveRepositoryDiscovery, serveRepositorySession, serveWebsite, sessionRegistry, test,
} from './support/repository-server';

const exact = { exact: true };
const check = { timeout: 1800 };
const reconnectProcedure = `/${ClientService.typeName}/${ClientService.method.reconnectClient.name}`;
const connected = create(ClientSchema, {
  id: 'review-worker', name: 'Route review worker', state: ClientState.ONLINE, agentLimit: 3,
  connectedAt: timestampFromMs(Date.parse('2026-11-27T11:30:00Z')),
});

const reviewTest = test.extend<{ web: string }>({
  web: async ({ website }, use) => {
    const web = await serveWebsite(website);
    try { await use(web.origin); } finally { await web.stop(); }
  },
});

async function openMachines(page: Page, web: string, repositoryMode: boolean, address?: string) {
  await page.goto(web + '/#/home');
  if (address) {
    await page.getByLabel('Server address', exact).fill(address);
    await page.getByRole('button', { name: 'Connect', ...exact }).click();
    await page.getByLabel('Password', exact).fill('browser fixture input');
    await page.getByRole('button', { name: 'Sign in', ...exact }).click();
  } else {
    await page.getByRole('button', { name: 'Use sample data', ...exact }).click();
  }
  await expect.poll(() => page.evaluate(() => {
    const { server, session } = window.lgtm.store.state;
    return { repositoryMode: server.repositoryMode, synced: server.synced, session: session.status };
  })).toEqual({ repositoryMode, synced: true, session: 'in' });
  await page.evaluate(() => { location.hash = '/machines'; });
  await expect(page.getByRole('heading', { name: 'Machines', ...exact })).toBeVisible();
}

const machineActions = (page: Page, name: string) => page.getByRole('button', { name: `Actions for ${name}`, ...exact });
const machineRow = (page: Page, name: string) => page.locator('.svc-machines tbody > tr').filter({ has: machineActions(page, name) });
const calling = (page: Page, name: string) => page.locator('.toasts').getByText(`Calling ${name}…`, exact);

async function recordReconnect(page: Page, address: string, web: string) {
  const requestedIds: string[] = [];
  await page.route(address + reconnectProcedure, async (route) => {
    if (route.request().method() !== 'POST') return route.fallback();
    requestedIds.push(route.request().postDataJSON().id);
    await route.fulfill({ json: {}, headers: {
      'Access-Control-Allow-Origin': web, 'Access-Control-Allow-Credentials': 'true',
    } });
  });
  return requestedIds;
}

reviewTest('post-4 1: watched machines offer neither reconnect nor restart', async ({ page, web }) => {
  const server = await serveRepositorySession({ pageOrigin: web, listener: 'routes-review',
    registry: sessionRegistry(), clients: [connected] });
  try {
    const trace = await observeWebsite(page, [web, server.origin]);
    const requestedIds = await recordReconnect(page, server.origin, web);
    await openMachines(page, web, true, server.origin);
    const row = machineRow(page, connected.name);
    const menu = page.getByRole('menu');
    const reconnectAction = menu.getByRole('menuitem', { name: /\b(?:reconnect|restart)\b/i });

    for (const status of ['Online', 'Offline'] as const) {
      await test.step(`Watch reports the machine ${status.toLowerCase()}`, async () => {
        if (status === 'Offline') {
          server.send({ case: 'saved', value: create(ClientSchema, {
            ...connected, state: ClientState.OFFLINE, connectedAt: undefined,
          }) });
        }
        await expect(row.getByText(status, exact)).toBeVisible();
        await machineActions(page, connected.name).click();
        await expect(menu.getByRole('menuitem', { name: 'Rename', ...exact })).toBeEnabled();
        await expect.soft(reconnectAction, `${status} watched machine has no reconnect or restart entry.`).toHaveCount(0, check);
        // Exercise a leaked action so the test can also observe its side effects.
        if (await reconnectAction.count() === 1 && await reconnectAction.isEnabled()) await reconnectAction.click();
        await expect.soft(calling(page, connected.name), `${status} watched machine has no calling toast.`).toHaveCount(0, check);
        await page.keyboard.press('Escape');
        await expect(menu).toHaveCount(0);
        expect.soft(requestedIds, `${status} watched machine receives no reconnect request.`).toEqual([]);
      });
    }
    expect(trace.errors).toEqual([]);
  } finally {
    await server.stop();
  }
});

async function readings(page: Page, name: string) {
  const labels = await page.locator('.svc-machines thead th').allTextContents();
  const row = machineRow(page, name);
  await expect(row).toBeVisible();
  return Object.fromEntries(['CPU', 'Memory', 'Last seen'].map((label) => {
    const column = labels.indexOf(label);
    expect(column, `The Machines table has a ${label} column.`).toBeGreaterThanOrEqual(0);
    // These columns are hidden by the existing phone layout.
    return [label, row.locator('td').nth(column)];
  }));
}

reviewTest('post-4 2: watched machine readings preserve omitted values and measured zero', async ({ page, web }) => {
  const server = await serveRepositoryDiscovery(web, { services: [ProjectService.typeName] }, [
    create(ClientSchema, { ...connected, id: 'no-processor', name: 'No processor reading', memoryPercent: 0 }),
    create(ClientSchema, { ...connected, id: 'no-memory', name: 'No memory reading', cpuPercent: 0 }),
    create(ClientSchema, { id: 'no-contact', name: 'No last contact', state: ClientState.OFFLINE }),
    create(ClientSchema, { id: 'with-contact', name: 'Reported last contact', state: ClientState.OFFLINE,
      lastSeen: timestampFromMs(Date.parse('2026-11-27T11:46:00Z')) }),
  ]);
  try {
    const trace = await observeWebsite(page, [web, server.origin]);
    server.releaseDiscovery();
    server.releaseMachines();
    await openMachines(page, web, true, server.origin);

    const noProcessor = await readings(page, 'No processor reading');
    const noMemory = await readings(page, 'No memory reading');
    await expect.soft(noProcessor.CPU, 'An omitted processor measurement is shown as a dash.').toHaveText('—', check);
    await expect.soft(noProcessor.Memory, 'Measured zero memory use is shown as zero percent.').toHaveText('0%', check);
    await expect.soft(noMemory.CPU, 'Measured zero processor use is shown as zero percent.').toHaveText('0%', check);
    await expect.soft(noMemory.Memory, 'An omitted memory measurement is shown as a dash.').toHaveText('—', check);

    const noContact = await readings(page, 'No last contact');
    const reportedContact = await readings(page, 'Reported last contact');
    await expect.soft(noContact['Last seen'], 'An omitted last contact has no elapsed time.').toHaveText(/^(?:—)?$/, check);
    // The fixture clock is paused at 12:01 UTC.
    await expect(reportedContact['Last seen']).toHaveText('15 min ago');
    expect(trace.errors).toEqual([]);
  } finally {
    await server.stop();
  }
});

reviewTest('post-4 1 and 2: sample data keeps the offline reconnect and machine readings', async ({ page, web }) => {
  const trace = await observeWebsite(page, [web]);
  await openMachines(page, web, false);
  const workstation = await readings(page, 'Workstation');
  const laptop = await readings(page, 'Laptop');
  await expect(workstation.CPU).toHaveText('46%');
  await expect(workstation.Memory).toHaveText('62%');
  await expect(workstation['Last seen']).toHaveText('Now');
  await expect(laptop.CPU).toHaveText('—');
  await expect(laptop.Memory).toHaveText('—');
  await expect(laptop['Last seen']).toHaveText('2 hr 12 min ago');

  await machineActions(page, 'Laptop').click();
  await page.getByRole('menuitem', { name: 'Try to reconnect', ...exact }).click();
  await expect(calling(page, 'Laptop')).toBeVisible();
  await expect.poll(() => page.evaluate((path) => window.discoveryProbe.calls.includes(path), reconnectProcedure)).toBe(true);
  await expect.poll(() => page.evaluate(() => window.discoveryProbe.timeouts[1500] ?? 0)).toBeGreaterThan(0);
  await page.clock.runFor(1500);
  await expect(machineRow(page, 'Laptop').getByText('Online', exact)).toBeVisible();
  expect(trace.errors).toEqual([]);
});

reviewTest('post-4 1: an older server keeps the offline reconnect action', async ({ page, web }) => {
  const server = await serveRepositoryDiscovery(web, { services: [] }, [create(ClientSchema, {
    id: 'compatibility-worker', name: 'Compatibility worker', state: ClientState.OFFLINE,
  })]);
  try {
    const trace = await observeWebsite(page, [web, server.origin]);
    const requestedIds = await recordReconnect(page, server.origin, web);
    server.releaseDiscovery();
    server.releaseMachines();
    await openMachines(page, web, false, server.origin);
    await expect(machineRow(page, 'Compatibility worker').getByText('Offline', exact)).toBeVisible();
    await machineActions(page, 'Compatibility worker').click();
    await page.getByRole('menuitem', { name: 'Try to reconnect', ...exact }).click();
    await expect(calling(page, 'Compatibility worker')).toBeVisible();
    await expect.poll(() => requestedIds).toEqual(['compatibility-worker']);
    expect(trace.errors).toEqual([]);
  } finally {
    await server.stop();
  }
});
