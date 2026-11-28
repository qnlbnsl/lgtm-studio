import { create, type MessageInitShape } from '@bufbuild/protobuf';
import { ClientSchema, ClientState, type WatchClientsResponseSchema } from '@lgtm/api/clients';
import { ProjectService, RepositorySchema, type Repository, type WatchRepositoriesResponseSchema } from '@lgtm/api/projects';
import { expect, type Page } from '@playwright/test';
import { controlledWatch, serveRepositorySession, serveWebsite, sessionRegistry, test } from './support/repository-server';

type RepositoryFrame = MessageInitShape<typeof WatchRepositoriesResponseSchema>;
type RepositoryWatch = ReturnType<typeof controlledWatch<RepositoryFrame>>['attempts'][number];

const exact = { exact: true };
const machine = create(ClientSchema, { id: 'entry-worker', name: 'Repository review worker', state: ClientState.ONLINE });

function repository(number: number, title: string) {
  const suffix = String(number).padStart(3, '0');
  return create(RepositorySchema, {
    id: '018f2ad2-bcde-7456-8123-000000002' + suffix,
    name: 'Switchyard ' + title,
    mainBranch: 'main',
    folders: [{ id: '018f2ad2-bcde-7456-8123-000000003' + suffix, clientId: machine.id, resolvedPath: '/checkouts/' + suffix }],
  });
}

const reviewTest = test.extend<{
  openWorkspace: (initial: Repository[]) => Promise<RepositoryWatch>;
}>({
  openWorkspace: async ({ page, website }, use) => {
    const web = await serveWebsite(website);
    const machines = controlledWatch<MessageInitShape<typeof WatchClientsResponseSchema>>();
    const repositories = controlledWatch<RepositoryFrame>();
    try {
      const server = await serveRepositorySession({
        pageOrigin: web.origin, hostname: new URL(web.origin).hostname, listener: 'entry-review', registry: sessionRegistry(),
        controlled: { services: [ProjectService.typeName], machines: machines.read, repositories: repositories.read },
      });
      try {
        await use(async (initial) => {
          await page.goto(web.origin + '/#/home');
          await page.getByLabel('Server address', exact).fill(server.origin);
          await page.getByRole('button', { name: 'Connect', ...exact }).click();
          await page.getByLabel('Password', exact).fill('browser fixture input');
          await page.getByRole('button', { name: 'Sign in', ...exact }).click();
          await expect.poll(() => [machines.attempts.length, repositories.attempts.length]).toEqual([1, 1]);
          const watch = repositories.attempts[0]!;
          machines.attempts[0]!.send({ change: { case: 'all', value: { clients: [machine] } }, position: { value: 1n } });
          watch.send({ change: { case: 'all', value: { repositories: initial } }, position: { value: 1n } });
          await expect(page.getByRole('button', { name: 'Connection: Live', ...exact })).toBeVisible();
          return watch;
        });
      } finally {
        await server.stop();
      }
    } finally {
      await web.stop();
    }
  },
});

async function openSwitcher(page: Page, query: string) {
  await page.keyboard.press('ControlOrMeta+k');
  const dialog = page.getByRole('dialog', { name: 'Find project or agent', ...exact });
  const search = dialog.getByRole('textbox');
  await expect(search).toBeFocused();
  await search.fill(query);
  return { dialog, search, options: dialog.getByRole('option'), active: dialog.getByRole('option', { selected: true }) };
}

reviewTest('post-7 1: the keyboard selection survives a ranked insertion at the Switcher limit', async ({ page, openWorkspace }) => {
  const limit = 14;
  const selected = repository(14, 'Cobalt');
  const overflow = repository(15, 'Quartz');
  const better = repository(16, 'Priority');
  const initial = [
    ...Array.from({ length: limit - 1 }, (_, index) => repository(index + 1, 'Checkout ' + (index + 1))),
    selected, overflow,
  ];
  const watch = await openWorkspace(initial);
  await page.mouse.move(0, 0);
  const { dialog, search, options, active } = await openSwitcher(page, 'Switchyard Priority');
  await expect(options).toHaveCount(limit);
  await expect(options.last().getByText(selected.name, exact)).toBeVisible();
  for (let index = 1; index < limit; index++) await search.press('ArrowDown');
  await expect(active.getByText(selected.name, exact)).toBeInViewport();

  watch.send({ change: { case: 'saved', value: better }, position: { value: 2n } });
  await expect(options.first().getByText(better.name, exact)).toBeVisible();
  await expect(options).toHaveCount(limit);
  await expect.soft(active.getByText(selected.name, exact), 'The keyboard selection stays active and in view after the insertion.')
    .toBeInViewport({ timeout: 1500 });
  await expect(search).toBeFocused();
  await page.keyboard.press('Enter');
  const destination = new URL('#/r/' + selected.id, page.url()).href;
  await expect.soft(page, 'Enter opens the repository selected before the watch update.').toHaveURL(destination, { timeout: 1500 });
  await expect(dialog).toBeHidden();

  const reopened = await openSwitcher(page, 'Switchyard Cobalt');
  await expect(reopened.active.getByText(selected.name, exact)).toBeInViewport();
  watch.send({ change: { case: 'removedId', value: selected.id }, position: { value: 3n } });
  await expect(reopened.dialog.getByText(selected.name, exact)).toHaveCount(0);
  await expect(reopened.options).toHaveCount(limit);
  await expect(reopened.active, 'A visible match becomes active when the selected repository is removed.').toBeInViewport();
});

reviewTest('post-7 2: New project keeps its goal and can submit after the last repository is removed', async ({ page, openWorkspace }) => {
  const attached = repository(1, 'Attached');
  const watch = await openWorkspace([attached]);
  await page.goto(new URL('#/projects', page.url()).href);
  await page.locator('.main').getByRole('button', { name: 'New project', ...exact }).click();
  const form = page.getByRole('dialog', { name: 'New project', ...exact });
  const choice = form.getByRole('combobox', { name: 'Repository', ...exact });
  const goal = form.getByRole('textbox', { name: 'Goal', ...exact });
  const createProject = form.getByRole('button', { name: 'Create project', ...exact });
  const draft = 'Keep this goal while the repository watch changes.';
  const fallback = 'New repository…';
  await expect(choice).toHaveValue(attached.id);
  await goal.fill(draft);
  await expect(createProject).toBeEnabled();

  watch.send({ change: { case: 'removedId', value: attached.id }, position: { value: 2n } });
  await expect(choice.locator('option')).toHaveText([fallback]);
  await expect(goal).toHaveValue(draft);
  await expect(choice.locator('option:checked'), 'The remaining repository option is selected automatically.').toHaveText(fallback);
  await expect(createProject).toBeDisabled();
  const name = form.getByRole('textbox', { name: 'New repository name', ...exact });
  await name.fill('Replacement checkout');
  await expect(createProject).toBeEnabled();
  await createProject.click();
  await expect(form.getByRole('alert').getByText('Projects are not connected to this server yet.', exact)).toBeVisible();
  await expect(goal).toHaveValue(draft);
  await expect(name).toHaveValue('Replacement checkout');
});
