import { create, type MessageInitShape } from '@bufbuild/protobuf';
import { ClientSchema, ClientState, type WatchClientsResponseSchema } from '@lgtm/api/clients';
import { ProjectService, RepositorySchema, type Repository, type WatchRepositoriesResponseSchema } from '@lgtm/api/projects';
import { expect, type Page } from '@playwright/test';
import { controlledWatch, serveRepositorySession, serveWebsite, sessionRegistry, test } from './support/repository-server';

type RepositoryFrame = MessageInitShape<typeof WatchRepositoriesResponseSchema>;
type RepositoryWatch = ReturnType<typeof controlledWatch<RepositoryFrame>>['attempts'][number];

const exact = { exact: true };
const machine = create(ClientSchema, { id: 'review-worker', name: 'Navigation review worker', state: ClientState.ONLINE });

function repository(suffix: string, name: string, path: string) {
  return create(RepositorySchema, {
    id: '018f2ad2-bcde-7456-8123-000000000' + suffix,
    name,
    mainBranch: 'main',
    folders: [{ id: '018f2ad2-bcde-7456-8123-000000001' + suffix, clientId: machine.id, resolvedPath: '/checkouts/' + path }],
  });
}

const first = repository('941', 'Switchyard Amber', 'amber');
const selected = repository('942', 'Switchyard Cobalt', 'cobalt');
const last = repository('943', 'Switchyard Quartz', 'quartz');
const added = repository('944', 'Switchyard Priority', 'priority');
const repoPath = (repo: Repository) => '/r/' + repo.id;

const reviewTest = test.extend<{
  openWorkspace: (initial: Repository[]) => Promise<RepositoryWatch>;
}>({
  openWorkspace: async ({ page, website }, use) => {
    const web = await serveWebsite(website);
    const machines = controlledWatch<MessageInitShape<typeof WatchClientsResponseSchema>>();
    const repositories = controlledWatch<RepositoryFrame>();
    const server = await serveRepositorySession({
      pageOrigin: web.origin, hostname: '127.0.0.1', listener: 'navigation-review', registry: sessionRegistry(),
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
      await Promise.all([web.stop(), server.stop()]);
    }
  },
});

const switcherCases = [
  {
    name: 'keeps the middle selection when the first of three matches is removed',
    initial: [first, selected, last], change: { case: 'removedId', value: first.id },
    remaining: [selected, last], destination: selected,
  },
  {
    name: 'keeps the last selection when the first of two matches is removed',
    initial: [first, selected], change: { case: 'removedId', value: first.id },
    remaining: [selected], destination: selected,
  },
  {
    name: 'selects the visible match when the selected repository is removed',
    initial: [first, selected], change: { case: 'removedId', value: selected.id },
    remaining: [first], destination: first,
  },
  {
    name: 'keeps the selection when a higher-ranked match is added',
    initial: [first, selected, last], change: { case: 'saved', value: added },
    remaining: [added, first, selected, last], destination: selected,
  },
  {
    name: 'keeps the selection when a watch snapshot reorders other matches',
    initial: [first, selected, last], change: { case: 'all', value: { repositories: [last, first, selected] } },
    remaining: [last, first, selected], destination: selected,
  },
] satisfies { name: string; initial: Repository[]; change: RepositoryFrame['change']; remaining: Repository[]; destination: Repository }[];

for (const { name, initial, change, remaining, destination } of switcherCases) {
  reviewTest(`post-9 1: the keyboard Switcher ${name}`, async ({ page, openWorkspace }) => {
    const watch = await openWorkspace(initial);
    // Keep the pointer outside the results while testing keyboard selection.
    await page.mouse.move(0, 0);
    await page.keyboard.press('ControlOrMeta+k');
    const dialog = page.getByRole('dialog', { name: 'Find project or agent', ...exact });
    const search = dialog.getByRole('textbox');
    const options = dialog.getByRole('option');
    const active = dialog.getByRole('option', { selected: true });
    await expect(search).toBeFocused();
    await search.fill('Switchyard priority');
    await expect(options).toContainText(initial.map(({ name }) => name));
    await expect(async () => {
      await search.press('ArrowUp');
      await search.press('ArrowDown');
      await expect(active.getByText(selected.name, exact)).toBeVisible({ timeout: 250 });
    }).toPass({ timeout: 3000 });
    await expect(search).toBeFocused();

    watch.send({ change, position: { value: 2n } });
    await expect(options).toHaveCount(remaining.length);
    await expect(options).toContainText(remaining.map(({ name }) => name));
    await expect(search).toBeFocused();
    await expect.soft(active.getByText(destination.name, exact), 'The intended visible result is active.').toBeVisible({ timeout: 1000 });
    await page.keyboard.press('Enter');
    await expect(page, 'Enter opens the active repository.').toHaveURL(new URL('#' + repoPath(destination), page.url()).href);
    await expect(dialog).toBeHidden();
  });
}

async function openRepositoryAddress(page: Page, child: string) {
  const address = new URL('#' + repoPath(first) + '/' + child, page.url());
  await page.goto(address.href);
  await expect(page).toHaveURL(address.href);
}

async function expectMissingPage(page: Page) {
  const missing = page.locator('.main [data-empty]').filter({ has: page.getByText('This page does not exist', exact) });
  await expect(missing).toBeVisible();
  await expect(missing.getByRole('link', { name: 'Go to Home', ...exact })).toHaveAttribute('href', '#/home');
  await expect(page.getByRole('tablist', { name: 'Repository views', ...exact })).toHaveCount(0);
}

for (const leaf of ['center', 'board', 'terminal']) {
  reviewTest(`post-9 4: an unknown ${leaf} descendant opens the missing page`, async ({ page, openWorkspace }) => {
    await openWorkspace([first]);
    await openRepositoryAddress(page, `${leaf}/unknown`);
    await expectMissingPage(page);
  });
}

reviewTest('post-9 4: Design, Files and Git descendants keep their repository views', async ({ page, openWorkspace }) => {
  await openWorkspace([first]);
  await openRepositoryAddress(page, 'unknown');
  await expectMissingPage(page);

  const tabs = page.getByRole('tablist', { name: 'Repository views', ...exact });
  const design = page.getByRole('navigation', { name: 'Design views', ...exact });
  const unavailable = page.locator('.main [data-empty]').filter({
    has: page.getByText('The service does not send this to the website yet.', exact),
  });

  for (const [child, section] of [
    ['design/screens', 'Screens'], ['design/screen/example', 'Screens'],
    ['design/file', 'Design file'], ['design/file/palette/colors', 'Design file'],
    ['design/palette/colors', 'Design file'], ['design/browser', 'Browser'], ['design/architecture', 'Architecture'],
  ] as const) {
    await test.step(`Open ${child}`, async () => {
      await openRepositoryAddress(page, child);
      await expect(tabs.getByRole('tab', { name: 'Design', selected: true, ...exact })).toBeVisible();
      await expect(design.getByRole('link', { name: section, ...exact })).toHaveAttribute('aria-current', 'page');
      await expect(unavailable.getByText('This page is not connected yet', exact)).toBeVisible();
    });
  }

  for (const [child, tab, subject] of [
    ['files/src/index.ts', 'Files', 'Files'], ['git/branch/main', 'Git', 'This page'],
  ] as const) {
    await test.step(`Open ${child}`, async () => {
      await openRepositoryAddress(page, child);
      await expect(tabs.getByRole('tab', { name: tab, selected: true, ...exact })).toBeVisible();
      await expect(unavailable.getByText(`${subject} is not connected yet`, exact)).toBeVisible();
    });
  }
});
