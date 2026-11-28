import { create, type MessageInitShape } from '@bufbuild/protobuf';
import { ClientSchema, ClientState, type Client, type WatchClientsResponseSchema } from '@lgtm/api/clients';
import { ProjectService, type WatchRepositoriesResponseSchema } from '@lgtm/api/projects';
import { expect, type Page } from '@playwright/test';
import {
  controlledWatch, observeWebsite, serveRepositorySession, serveWebsite, sessionProcedure, sessionRegistry, test,
} from './support/repository-server';

const exact = { exact: true };
type MachineFrame = MessageInitShape<typeof WatchClientsResponseSchema>;
type RepositoryFrame = MessageInitShape<typeof WatchRepositoriesResponseSchema>;
type MachineWatch = ReturnType<typeof controlledWatch<MachineFrame>>['attempts'][number];
type OpenMachines = (client: Client) => Promise<{
  watch: MachineWatch;
  trace: Awaited<ReturnType<typeof observeWebsite>>;
}>;

const recoveryTest = test.extend<{ openMachines: OpenMachines }>({
  openMachines: async ({ page, website }, use) => {
    const web = await serveWebsite(website);
    const machines = controlledWatch<MachineFrame>();
    const repositories = controlledWatch<RepositoryFrame>();
    try {
      const server = await serveRepositorySession({
        pageOrigin: web.origin, hostname: '127.0.0.1', listener: 'recovery-review', registry: sessionRegistry(),
        controlled: { services: [ProjectService.typeName], machines: machines.read, repositories: repositories.read },
      });
      try {
        await use(async (client) => {
          const trace = await observeWebsite(page, [web.origin, server.origin]);
          await page.goto(web.origin + '/?data=worst#/home');
          await page.getByLabel('Server address', exact).fill(server.origin);
          await page.getByRole('button', { name: 'Connect', ...exact }).click();
          await page.getByLabel('Password', exact).fill('browser fixture input');
          await page.getByRole('button', { name: 'Sign in', ...exact }).click();
          await expect.poll(() => machines.attempts.length).toBe(1);
          await expect.poll(() => repositories.attempts.length).toBe(1);
          const watch = machines.attempts[0]!;
          watch.send({ change: { case: 'all', value: { clients: [client] } }, position: { value: 1n } });
          repositories.attempts[0]!.send({ change: { case: 'all', value: { repositories: [] } }, position: { value: 1n } });
          await expect(page.getByRole('button', { name: 'Connection: Live', ...exact })).toBeVisible();
          await page.evaluate(() => { location.hash = '/machines'; });
          await expect(page.getByRole('heading', { name: 'Machines', ...exact })).toBeVisible();
          return { watch, trace };
        });
      } finally {
        await server.stop();
      }
    } finally {
      await web.stop();
    }
  },
});

const machineRow = (page: Page, name: string) => page.getByRole('row').filter({ has: page.getByText(name, exact) });

recoveryTest('open removal confirmation follows a watched recovery flag', async ({ page, openMachines }) => {
  const worker = create(ClientSchema, { id: 'offline-worker', name: 'Offline worker', state: ClientState.OFFLINE });
  const { watch, trace } = await openMachines(worker);
  const row = machineRow(page, worker.name);
  await expect(row.getByRole('cell', { name: 'Offline', ...exact })).toBeVisible();
  await row.getByRole('button', { name: 'Actions for ' + worker.name, ...exact }).click();
  await page.getByRole('menuitem', { name: 'Remove machine', ...exact }).click();

  const title = `Remove ${worker.name}?`;
  const body = `${worker.name} is disconnected and forgotten. Its folders stay on its disk.`;
  const dialog = page.getByRole('dialog', { name: title, ...exact });
  await expect(dialog).toBeVisible();
  await expect(dialog.locator('.dialog-body')).toHaveText(body + ' Pair it again to bring it back.');

  const flagged = create(ClientSchema, { ...worker, eventHistoryNeedsRecovery: true });
  watch.send({ change: { case: 'saved', value: flagged }, position: { value: 2n } });
  await expect(row.getByRole('cell', { name: 'Out of sync', ...exact })).toBeVisible();
  await expect.poll(() => page.getByRole('dialog').evaluateAll((dialogs, approved) => {
    if (dialogs.length === 0) return [approved];
    return dialogs.map((element) => ({
      title: element.getAttribute('aria-label'),
      body: element.querySelector('.dialog-body')?.textContent?.trim(),
    }));
  }, { title, body }), { message: 'The open confirmation shows the approved flagged wording or closes.' })
    .toEqual([{ title, body }]);
  expect(trace.calls, 'The watch does not confirm removal.').not.toContain(sessionProcedure.remove);
});

recoveryTest('a saved frame clears the warning and keeps the waiting machine', async ({ page, openMachines }) => {
  const waiting = create(ClientSchema, {
    id: 'waiting-worker', name: 'Waiting worker', state: ClientState.WAITING_FOR_APPROVAL,
    tokenId: 'pair-1', eventHistoryNeedsRecovery: true,
  });
  const { watch } = await openMachines(waiting);
  const row = machineRow(page, waiting.name);
  await expect(row.getByRole('cell', { name: 'Out of sync', ...exact })).toBeVisible();

  const ready = create(ClientSchema, { ...waiting, eventHistoryNeedsRecovery: false });
  watch.send({ change: { case: 'saved', value: ready }, position: { value: 2n } });
  await expect(row, 'The saved false flag removes the waiting machine’s diagnostic row.').toHaveCount(0);
  const joining = await page.evaluate((id) => window.lgtm.store.state.joining
    .filter((machine) => machine.id === id)
    .map(({ host, tokenId }) => ({ host, tokenId })), waiting.id);
  expect(joining, 'The same machine still waits for approval with its pairing token.')
    .toEqual([{ host: waiting.name, tokenId: waiting.tokenId }]);
});
