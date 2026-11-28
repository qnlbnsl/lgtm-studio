import { create, type MessageInitShape } from '@bufbuild/protobuf';
import { Code } from '@connectrpc/connect';
import { ClientSchema, ClientState, type Client, type WatchClientsResponseSchema } from '@lgtm/api/clients';
import { ProjectService, type WatchRepositoriesResponseSchema } from '@lgtm/api/projects';
import { expect, type Page } from '@playwright/test';
import {
  controlledWatch, observeWebsite, serveRepositorySession, serveWebsite, sessionProcedure, sessionRegistry, test,
} from './support/repository-server';

const exact = { exact: true };
const diagnosis = "This machine's saved history is older than the server's.";
const oldCallouts = /stopped answering at|lgtm connect|agents are paused|scanning it for harnesses|wants to connect|confirm the machine|pair (?:it|this machine) again/i;
const offline = create(ClientSchema, {
  id: 'lagging-worker', name: 'Lagging worker', state: ClientState.OFFLINE, eventHistoryNeedsRecovery: true,
  lastSeen: { seconds: 1795780000n },
});
const waiting = create(ClientSchema, {
  id: 'lagging-waiting', name: 'Lagging waiting', state: ClientState.WAITING_FOR_APPROVAL, eventHistoryNeedsRecovery: true,
});
const host = create(ClientSchema, {
  id: 'lagging-host', name: 'Lagging host', state: ClientState.ONLINE, hostsServer: true, eventHistoryNeedsRecovery: true,
});
const older = create(ClientSchema, {
  id: 'older-worker', name: 'Older worker', state: ClientState.OFFLINE, lastSeen: { seconds: 1795780000n },
  workspace: '/workspaces/older', version: '1.2.3-older', harnessesScanned: true,
  harnesses: [{ harness: 'codex', version: '2.3.4-older' }],
});
const worker = create(ClientSchema, { id: 'normal-worker', name: 'Normal worker', state: ClientState.ONLINE });

type MachineFrame = MessageInitShape<typeof WatchClientsResponseSchema>;
type RepositoryFrame = MessageInitShape<typeof WatchRepositoriesResponseSchema>;
type Server = Awaited<ReturnType<typeof serveRepositorySession>>;
type Rig = {
  web: string;
  server: Server;
  machines: ReturnType<typeof controlledWatch<MachineFrame>>;
  repositories: ReturnType<typeof controlledWatch<RepositoryFrame>>;
};

const recoveryTest = test.extend<{ rig: Rig }>({
  rig: async ({ website }, use) => {
    const web = await serveWebsite(website);
    const machines = controlledWatch<MachineFrame>();
    const repositories = controlledWatch<RepositoryFrame>();
    try {
      const server = await serveRepositorySession({
        pageOrigin: web.origin, hostname: '127.0.0.1', listener: 'recovery', registry: sessionRegistry(),
        controlled: { services: [ProjectService.typeName], machines: machines.read, repositories: repositories.read },
      });
      try {
        await use({ web: web.origin, server, machines, repositories });
      } finally {
        await server.stop();
      }
    } finally {
      await web.stop();
    }
  },
});

const machineRow = (page: Page, name: string) => page.getByRole('row').filter({ has: page.getByText(name, exact) });
const details = (page: Page, name: string) => machineRow(page, name).locator('+ tr');
const toggleDetails = (page: Page, name: string, verb: 'Show' | 'Hide') =>
  machineRow(page, name).getByRole('button', { name: verb + ' details for ' + name, ...exact });
const status = (page: Page, name: string, text: string) => machineRow(page, name).getByRole('cell', { name: text, ...exact });
const onlineCount = (page: Page, text: string) => page.getByText(text, exact).and(page.locator(':visible')).first();

function snapshot(clients: Client[], position: bigint): MachineFrame {
  return { change: { case: 'all', value: { clients } }, position: { value: position } };
}

function save(client: Client, position: bigint): MachineFrame {
  return { change: { case: 'saved', value: client }, position: { value: position } };
}

function remove(id: string, position: bigint): MachineFrame {
  return { change: { case: 'removedId', value: id }, position: { value: position } };
}

async function openMachines(page: Page, rig: Rig, clients: Client[]) {
  const trace = await observeWebsite(page, [rig.web, rig.server.origin]);
  await page.goto(rig.web + '/?data=worst#/home');
  await page.getByLabel('Server address', exact).fill(rig.server.origin);
  await page.getByRole('button', { name: 'Connect', ...exact }).click();
  await page.getByLabel('Password', exact).fill('browser fixture input');
  await page.getByRole('button', { name: 'Sign in', ...exact }).click();
  await expect.poll(() => rig.machines.attempts.length).toBe(1);
  await expect.poll(() => rig.repositories.attempts.length).toBe(1);
  const watch = rig.machines.attempts[0]!;
  watch.send(snapshot(clients, 1n));
  rig.repositories.attempts[0]!.send({ change: { case: 'all', value: { repositories: [] } }, position: { value: 1n } });
  await expect(page.getByRole('button', { name: 'Connection: Live', ...exact })).toBeVisible();
  await page.evaluate(() => { location.hash = '/machines'; });
  await expect(page.getByRole('heading', { name: 'Machines', ...exact })).toBeVisible();
  return { trace, watch };
}

async function expectRecovery(page: Page, name: string) {
  await expect(machineRow(page, name), name + ' has exactly one watched row.').toHaveCount(1);
  await expect(status(page, name, 'Out of sync'), name + ' shows the watched recovery status.').toBeVisible();
  const show = toggleDetails(page, name, 'Show');
  if (await show.isVisible()) await show.click();
  const reason = details(page, name).getByText(diagnosis, exact);
  await expect(reason, name + ' shows exactly one approved recovery reason.').toHaveCount(1);
  await expect(reason).toBeVisible();
  await expect(details(page, name).getByText(oldCallouts), 'The recovery reason replaces the old callouts.').toHaveCount(0);
}

async function confirmRemoval(page: Page, server: Server, client: Client) {
  await machineRow(page, client.name).getByRole('button', { name: 'Actions for ' + client.name, ...exact }).click();
  const menu = page.getByRole('menu');
  await expect(menu.getByRole('menuitem', { name: /reconnect/i })).toHaveCount(0);
  const action = menu.getByRole('menuitem', { name: 'Remove machine', ...exact });
  await expect(action).toBeInViewport();
  await expect(action).toBeEnabled();
  await action.click();

  const dialog = page.getByRole('dialog', { name: 'Remove ' + client.name + '?', ...exact });
  await expect(dialog.locator('.dialog-body'), 'The flagged removal confirmation contains only the approved facts.')
    .toHaveText(client.name + ' is disconnected and forgotten. Its folders stay on its disk.');
  const request = page.waitForRequest(server.origin + sessionProcedure.remove);
  await dialog.getByRole('button', { name: 'Remove machine', ...exact }).click();
  expect((await request).postDataJSON()?.id, 'The removal targets the watched machine.').toBe(client.id);
  await expect(page.getByText('Removed ' + client.name, exact)).toBeVisible();
  await expectRecovery(page, client.name);
  await page.clock.runFor(3600);
}

recoveryTest('machine recovery follows snapshots and updates', async ({ page, rig }) => {
  const { trace, watch } = await openMachines(page, rig, [offline, waiting, host, older, worker]);
  await expect(onlineCount(page, '2 of 4 online')).toBeVisible();
  await expect(status(page, older.name, 'Offline')).toBeVisible();
  await toggleDetails(page, older.name, 'Show').click();
  const olderDetails = details(page, older.name);
  await expect(olderDetails.getByText(older.workspace, exact)).toBeVisible();
  await expect(olderDetails.getByText(older.version, exact)).toBeVisible();
  await expect(olderDetails.getByText('2.3.4-older', exact)).toBeVisible();
  await expect(olderDetails.getByText(/stopped answering at/)).toBeVisible();
  await expect(olderDetails.getByText(diagnosis, exact)).toHaveCount(0);
  await toggleDetails(page, older.name, 'Hide').click();

  for (const client of [offline, waiting, host]) {
    await test.step(client.name + ' shows the approved diagnosis', async () => {
      await expectRecovery(page, client.name);
      await expect(page.getByText(diagnosis, exact)).toHaveCount(1);
      const add = details(page, client.name).getByRole('button', { name: 'Add repository', ...exact });
      if (client.state === ClientState.ONLINE) await expect(add).toBeEnabled();
      else await expect(add.and(page.locator(':enabled'))).toHaveCount(0);
      await toggleDetails(page, client.name, 'Hide').click();
      await expect(page.getByText(diagnosis, exact)).toHaveCount(0);
    });
  }

  await toggleDetails(page, worker.name, 'Show').click();
  const add = details(page, worker.name).getByRole('button', { name: 'Add repository', ...exact });
  await add.click();
  const repository = page.getByRole('dialog', { name: 'Add repository', ...exact });
  const chooser = repository.getByRole('combobox', { name: 'Machine', ...exact });
  await expect(chooser).toHaveValue(worker.id);
  await expect(chooser.locator('option').filter({ hasText: waiting.name })).toHaveCount(0);
  await repository.getByRole('button', { name: 'Cancel', ...exact }).click();
  await expect(repository).toHaveCount(0);
  await page.evaluate(() => { location.hash = '/projects'; });
  await page.locator('.main').getByRole('button', { name: 'New project', ...exact }).click();
  const project = page.getByRole('dialog', { name: 'New project', ...exact });
  const runOn = project.getByRole('combobox', { name: 'Run on', ...exact });
  await expect(runOn.getByRole('option', { name: worker.name, ...exact })).toHaveCount(1);
  await expect(runOn.locator('option').filter({ hasText: waiting.name })).toHaveCount(0);
  await project.getByRole('button', { name: 'Cancel', ...exact }).click();
  await expect(project).toHaveCount(0);
  await page.evaluate(() => { location.hash = '/machines'; });

  const renamed = create(ClientSchema, { ...worker, name: 'Updated worker' });
  watch.send(save(renamed, 2n));
  await expect(machineRow(page, renamed.name)).toHaveCount(1);
  const waitingUpdate = create(ClientSchema, { ...waiting, name: 'Updated waiting' });
  watch.send(save(waitingUpdate, 3n));
  await expectRecovery(page, waitingUpdate.name);
  await expect(machineRow(page, waiting.name)).toHaveCount(0);
  await toggleDetails(page, waitingUpdate.name, 'Hide').click();
  for (const [index, client] of [offline, waitingUpdate, host].entries()) {
    await test.step(client.name + ' stays until watched removal', async () => {
      await expectRecovery(page, client.name);
      await confirmRemoval(page, rig.server, client);
      watch.send(remove(client.id, BigInt(index + 4)));
      await expect(machineRow(page, client.name)).toHaveCount(0);
      await expect(page.getByText(diagnosis, exact)).toHaveCount(0);
    });
  }
  await expect(onlineCount(page, '1 of 2 online')).toBeVisible();
  expect(trace.errors).toEqual([]);
});

async function holdApproval(page: Page, server: Server, client: Client, name: string) {
  await expect(machineRow(page, client.name), 'An unflagged waiting machine stays in pairing.').toHaveCount(0);
  await page.getByRole('button', { name: 'Add machine', ...exact }).click();
  const dialog = page.getByRole('dialog', { name: 'Add a machine', ...exact });
  await expect.poll(async () => {
    await page.clock.runFor(16);
    return dialog.getByText(client.name, exact).isVisible();
  }, { message: 'Pairing receives the unflagged waiting machine.' }).toBe(true);
  await dialog.getByLabel('Name', exact).fill(name);
  const held = server.hold('approve');
  await dialog.getByRole('button', { name: 'Confirm ' + name, ...exact }).click();
  await held.arrived;
  await expect(dialog).toBeVisible();
  return async () => {
    const response = page.waitForResponse(server.origin + sessionProcedure.approve);
    held.release();
    await (await response).finished();
    await expect(dialog).toHaveCount(0);
  };
}

async function replacementWatch(page: Page, rig: Rig, clients: Client[], position: bigint) {
  const prior = rig.machines.attempts.length;
  rig.machines.attempts[prior - 1]!.send(Code.Aborted);
  await expect.poll(async () => {
    await page.clock.runFor(500);
    return rig.machines.attempts.length;
  }).toBe(prior + 1);
  const watch = rig.machines.attempts[prior]!;
  watch.send(snapshot(clients, position));
  return watch;
}

recoveryTest('recovery state survives stale approval and clears on watched readiness', async ({ page, rig }) => {
  const pending = create(ClientSchema, {
    id: 'pending-first', name: 'Pending first', state: ClientState.WAITING_FOR_APPROVAL, tokenId: 'pair-1',
  });
  const pendingRemoval = create(ClientSchema, {
    id: 'pending-second', name: 'Pending second', state: ClientState.WAITING_FOR_APPROVAL, tokenId: 'pair-2',
  });
  const { trace, watch } = await openMachines(page, rig, [pending, pendingRemoval, worker]);

  const release = await holdApproval(page, rig.server, pending, 'Name from old approval');
  const lagging = create(ClientSchema, { ...offline, id: pending.id, name: 'Name from watch' });
  watch.send(save(lagging, 2n));
  await expect(page.getByText(lagging.name, exact)).toBeVisible();
  await release();
  await expectRecovery(page, lagging.name);
  await expect(machineRow(page, 'Name from old approval')).toHaveCount(0);

  const updated = create(ClientSchema, { ...worker, name: 'Updated unrelated worker' });
  watch.send(save(updated, 3n));
  await expect(machineRow(page, updated.name)).toHaveCount(1);
  await expectRecovery(page, lagging.name);

  const ready = create(ClientSchema, {
    ...lagging, state: ClientState.ONLINE, eventHistoryNeedsRecovery: false,
  });
  watch.send(save(ready, 4n));
  await expect(status(page, ready.name, 'Online')).toBeVisible();
  await expect(page.getByText(diagnosis, exact)).toHaveCount(0);
  watch.send(save(lagging, 5n));
  await expectRecovery(page, lagging.name);

  const reconnected = create(ClientSchema, { ...updated, name: 'Worker on replacement snapshot' });
  await replacementWatch(page, rig, [lagging, pendingRemoval, reconnected], 5n);
  await expect(machineRow(page, reconnected.name)).toHaveCount(1);
  await expectRecovery(page, lagging.name);

  const next = await replacementWatch(page, rig, [ready, pendingRemoval, reconnected], 6n);
  await expect(status(page, ready.name, 'Online')).toBeVisible();
  await expect(page.getByText(diagnosis, exact)).toHaveCount(0);

  // The earlier approval notice can cover Add machine while the clock is paused.
  await page.clock.runFor(3600);
  const late = await holdApproval(page, rig.server, pendingRemoval, 'Name from late approval');
  const removed = create(ClientSchema, { ...offline, id: pendingRemoval.id, name: 'Removed by the watch' });
  next.send(save(removed, 7n));
  await expect(page.getByText(removed.name, exact)).toBeVisible();
  next.send(remove(removed.id, 8n));
  await expect(page.getByText(removed.name, exact)).toHaveCount(0);
  await late();
  await expect(machineRow(page, 'Name from late approval')).toHaveCount(0);
  await expect(machineRow(page, removed.name)).toHaveCount(0);
  await expect(onlineCount(page, '2 of 2 online')).toBeVisible();
  expect(await page.evaluate((id) => {
    const { machines, joining } = window.lgtm.store.state;
    return machines.some((machine) => machine.id === id) || joining.some((machine) => machine.id === id);
  }, pendingRemoval.id), 'The late reply cannot restore either watched projection.').toBe(false);
  expect(trace.errors).toEqual([]);
});
