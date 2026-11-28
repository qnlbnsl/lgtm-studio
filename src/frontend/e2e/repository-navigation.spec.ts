import { create, type MessageInitShape } from '@bufbuild/protobuf';
import { ClientSchema, ClientState, type Client, type WatchClientsResponseSchema } from '@lgtm/api/clients';
import {
  ProjectService, RepositoryFolderSchema, RepositorySchema, type Repository, type WatchRepositoriesResponseSchema,
} from '@lgtm/api/projects';
import { expect, type Locator, type Page } from '@playwright/test';
import {
  controlledWatch, observeRepositoryWatch, procedure, serveRepositorySession, serveWebsite, sessionRegistry, test,
} from './support/repository-server';

type MachineFrame = MessageInitShape<typeof WatchClientsResponseSchema>;
type RepositoryFrame = MessageInitShape<typeof WatchRepositoriesResponseSchema>;

const worker = create(ClientSchema, { id: 'navigation-worker', name: 'Navigation worker', state: ClientState.ONLINE });
const survivor = create(ClientSchema, { id: 'navigation-survivor', name: 'Navigation survivor', state: ClientState.ONLINE });
const origin = 'https://example.com/navigation/watched.git';
const id = (suffix: string) => '018f2ad2-bcde-7456-8123-000000000' + suffix;
const folder = (suffix: string, machine: Client, leaf: string) =>
  create(RepositoryFolderSchema, { id: id(suffix), clientId: machine.id, resolvedPath: '/checkouts/' + leaf });
const first = folder('931', worker, 'navigation-primary');
const second = folder('932', worker, 'navigation-second-copy');
const elsewhere = folder('933', survivor, 'navigation-surviving-copy');

const present = create(RepositorySchema, {
  id: id('901'), name: 'Navigation Quartz', mainBranch: 'fixture-trunk', remote: origin,
  folders: [second, elsewhere, first],
});
const absent = create(RepositorySchema, {
  id: id('902'), name: 'Navigation Amber', mainBranch: 'main', folders: [folder('934', worker, 'navigation-amber')],
});
const withheld = create(RepositorySchema, {
  id: id('903'), name: 'Navigation Cobalt', mainBranch: 'main', remoteOmitted: true,
  folders: [folder('935', survivor, 'navigation-cobalt')],
});
const saved = [present, absent, withheld];
const clients = [worker, survivor];

const exact = { exact: true };
const quick = { timeout: 1000 };
const unavailable = 'The service does not send this to the website yet.';
const missing = 'This repository is not known';
const repoPath = (repository: Repository, child = '') => '/r/' + repository.id + (child ? '/' + child : '');
const tree = (page: Page) => page.getByRole('navigation', { name: 'Repositories', ...exact });
const content = (page: Page) => page.locator('.main');
const tabs = (page: Page) => page.getByRole('tablist', { name: 'Repository views', ...exact });
const designNav = (page: Page) => page.getByRole('navigation', { name: 'Design views', ...exact });
const widget = (page: Page, name: string) => content(page).getByRole('region', { name, ...exact });
const busy = (page: Page) => page.locator('main[aria-busy="true"]');
const noAgent = '.mc, .mc-talk, .panel, [aria-label*="with the repository agent" i]';
const noTerminal = '.term, .term-none, [aria-label="Terminal sessions"], [aria-label="Open a terminal in"]';
const tabLabels = ['Command center', 'Board', 'Design', 'Files', 'Git'];

const routeFamilies = [
  { tab: 'Board', child: 'board', subject: 'This page', sample: '.board-page, .board-lanes' },
  { tab: 'Design', child: 'design', subject: 'This page', sample: '.wf-body, .mk-detail, .dz, .bw, .map, iframe' },
  { tab: 'Files', child: 'files', subject: 'Files', sample: '.fx, textarea, [contenteditable="true"], [aria-label$="the list"]' },
  { tab: 'Git', child: 'git', subject: 'This page', sample: '.git, .git-log, .git-commit, [aria-label$="the list"]' },
];
const designChildren = [
  ['design/screens', 'Screens'],
  ['design/screen/example', 'Screens'],
  ['design/file', 'Design file'],
  ['design/file/palette/colors', 'Design file'],
  ['design/palette/colors', 'Design file'],
  ['design/browser', 'Browser'],
  ['design/architecture', 'Architecture'],
] as const;

type Rig = {
  web: string;
  server: Awaited<ReturnType<typeof serveRepositorySession>>;
  machines: ReturnType<typeof controlledWatch<MachineFrame>>;
  repositories: ReturnType<typeof controlledWatch<RepositoryFrame>>;
};

const navigationTest = test.extend<{ rig: Rig }>({
  rig: async ({ website }, use) => {
    const web = await serveWebsite(website);
    const machines = controlledWatch<MachineFrame>();
    const repositories = controlledWatch<RepositoryFrame>();
    const server = await serveRepositorySession({
      pageOrigin: web.origin, hostname: '127.0.0.1', listener: 'navigation', registry: sessionRegistry(),
      controlled: { services: [ProjectService.typeName], machines: machines.read, repositories: repositories.read },
    });
    try {
      await use({ web: web.origin, server, machines, repositories });
    } finally {
      await Promise.all([web.stop(), server.stop()]);
    }
  },
});

async function navigate(page: Page, path: string) {
  await page.evaluate((next) => {
    if (location.hash === '#' + next) return;
    return new Promise<void>((done) => {
      addEventListener('hashchange', () => done(), { once: true });
      location.hash = next;
    });
  }, path);
}

async function openNavigation(page: Page) {
  const open = page.getByRole('button', { name: 'Open navigation', ...exact });
  if (await open.isVisible() && !await page.locator('.drawer-open').count()) await open.click();
}

async function observe(page: Page, rig: Rig) {
  const trace = await observeRepositoryWatch(page, [rig.web, rig.server.origin]);
  await page.clock.resume();
  await page.goto(rig.web + '/?data=worst#/home');
  return trace;
}

async function connect(page: Page, rig: Rig) {
  await page.getByLabel('Server address', exact).fill(rig.server.origin);
  await page.getByRole('button', { name: 'Connect', ...exact }).click();
  await expect(page.getByText('Sign in to the server', exact)).toBeVisible();
  await page.getByLabel('Password', exact).fill('browser fixture input');
  await page.getByRole('button', { name: 'Sign in', ...exact }).click();
}

async function startWatches(page: Page, rig: Rig, index: number, machines: Client[], repos: Repository[]) {
  await expect.poll(() => [rig.machines.attempts.length, rig.repositories.attempts.length]).toEqual([index + 1, index + 1]);
  await expect(busy(page)).toBeVisible();
  const watch = rig.repositories.attempts[index]!;
  rig.machines.attempts[index]!.send({ change: { case: 'all', value: { clients: machines } }, position: { value: 1n } });
  watch.send({ change: { case: 'all', value: { repositories: repos } }, position: { value: 1n } });
  await expect(page.getByRole('button', { name: 'Connection: Live', ...exact })).toBeVisible();
  return watch;
}

async function start(page: Page, rig: Rig, repos = saved, machines = clients) {
  const trace = await observe(page, rig);
  await connect(page, rig);
  const watch = await startWatches(page, rig, 0, machines, repos);
  return { trace, watch };
}

async function reloadAt(page: Page, rig: Rig, path: string, repos: Repository[], machines = clients) {
  const index = rig.machines.attempts.length;
  await page.evaluate((next) => history.replaceState(history.state, '', '#' + next), path);
  await page.reload();
  const watch = await startWatches(page, rig, index, machines, repos);
  expect(new URL(page.url()).hash).toBe('#' + path);
  return watch;
}

async function switcher(page: Page) {
  await page.keyboard.press('ControlOrMeta+k');
  const dialog = page.getByRole('dialog', { name: 'Find project or agent', ...exact });
  await expect(dialog).toBeVisible();
  const search = dialog.getByRole('textbox');
  await expect(search).toBeFocused();
  return { dialog, search };
}

function repositoryOption(dialog: Locator, repository: Repository) {
  return dialog.getByRole('option').filter({ has: dialog.page().getByText(repository.name, exact) });
}

async function expectUnavailable(scope: Locator, subject: string | RegExp, label: string) {
  const title = typeof subject === 'string' ? subject + ' is not connected yet' : subject;
  const body = scope.locator('[data-empty]').filter({ has: scope.page().getByText(title, exact) });
  await expect.soft(body.getByText(unavailable, exact), label + ' uses the existing unavailable body.').toBeVisible(quick);
}

async function expectNoSampleShell(page: Page, label: string) {
  await expect.soft(page.locator(noAgent + ', ' + noTerminal), label + ' has no agent or terminal UI.').toHaveCount(0, quick);
}

async function expectRepositoryTabs(page: Page, repository: Repository, active: string) {
  await expect(tabs(page)).toBeVisible();
  await expect(tabs(page).getByRole('tab')).toHaveCount(tabLabels.length);
  for (const [index, child] of ['center', 'board', 'design', 'files', 'git'].entries()) {
    await expect(tabs(page).getByRole('tab', { name: tabLabels[index], ...exact }))
      .toHaveAttribute('href', '#' + repoPath(repository, child));
  }
  await expect(tabs(page).getByRole('tab', { name: active, ...exact })).toHaveAttribute('aria-selected', 'true');
}

async function expectDesignNav(page: Page, active: string) {
  await expect(designNav(page)).toBeVisible();
  await expect(designNav(page).getByRole('link')).toHaveText(['Screens', 'Design file', 'Browser', 'Architecture']);
  await expect(designNav(page).getByRole('link', { name: active, ...exact })).toHaveAttribute('aria-current', 'page');
}

async function expectProjectIntent(button: Locator, repository: Repository) {
  // Item 7 owns rendering the dialog. Observe the intent before it renders.
  const opensForRepository = await button.evaluate((element: HTMLButtonElement, repositoryId) => {
    const { act, store } = window.lgtm;
    element.click();
    const dialog = store.state.dialog;
    const opened = dialog?.type === 'new-project' && dialog.repo === repositoryId;
    act.closeDialog();
    return opened;
  }, repository.id);
  expect(opensForRepository).toBe(true);
}

function expectReadOnly(trace: Awaited<ReturnType<typeof observeRepositoryWatch>>) {
  const allowed: readonly string[] = [procedure.discovery, procedure.signIn, procedure.machines, procedure.repositories];
  expect.soft(trace.calls.filter((path) => !allowed.includes(path)), 'Navigation sends no unsupported call.').toEqual([]);
  expect.soft(trace.errors).toEqual([]);
}

async function expectUnchangedRecords(page: Page) {
  const counts = await page.evaluate(() => {
    const s = window.lgtm.store.state;
    return [
      s.projects.length, s.requests.length, s.agents.length, s.tasks.length, s.worktrees.length,
      Object.keys(s.design).length, Object.keys(s.mocks).length, Object.keys(s.terminal?.byProject ?? {}).length,
    ];
  });
  expect.soft(counts, 'Navigation leaves the sample record collections empty.').toEqual([0, 0, 0, 0, 0, 0, 0, 0]);
}

navigationTest.describe('saved repositories open from tree and search', () => {
  navigationTest('the tree opens each saved identity after reload with a sample filter selected before switching', async ({ page, rig }) => {
    const trace = await observe(page, rig);
    await page.getByRole('button', { name: 'Use sample data', ...exact }).click();
    await expect(page.getByRole('heading', { name: 'Needs attention', ...exact })).toBeVisible();
    await openNavigation(page);
    const filter = page.getByRole('button', { name: 'Only projects needing you', ...exact });
    await filter.click();
    await expect(filter).toHaveAttribute('aria-pressed', 'true');

    await page.evaluate(() => window.lgtm.act.signOut());
    await page.getByRole('button', { name: /^Change the server\./ }).click();
    await connect(page, rig);
    const watch = await startWatches(page, rig, 0, clients, []);
    expect(await page.evaluate(() => window.lgtm.store.state.prefs.onlyNeedingYou)).toBe(true);
    await openNavigation(page);
    await expect(tree(page).getByText('No open requests', exact)).toBeVisible();
    watch.send({ change: { case: 'saved', value: present }, position: { value: 2n } });
    watch.send({ change: { case: 'saved', value: absent }, position: { value: 3n } });

    for (const repository of [present, absent]) {
      const row = tree(page).getByRole('link', { name: repository.name, ...exact });
      await expect.soft(row, 'The sample filter cannot hide ' + repository.name + '.').toHaveAttribute('href', '#' + repoPath(repository), quick);
    }
    await expect(content(page).getByRole('table').getByText('No open requests', exact)).toBeVisible();

    await reloadAt(page, rig, '/home', [present, absent]);
    for (const repository of [present, absent]) {
      await openNavigation(page);
      await expect(tree(page).getByRole('link')).toHaveCount(2);
      const row = tree(page).getByRole('link', { name: repository.name, ...exact });
      await expect(row).toHaveAttribute('href', '#' + repoPath(repository));
      await expectProjectIntent(tree(page).getByLabel('New project in ' + repository.name, exact), repository);
      await row.click();
      expect(new URL(page.url()).hash).toBe('#' + repoPath(repository));
      await expect(page.locator('.drawer-open')).toHaveCount(0);
      await expectRepositoryTabs(page, repository, 'Command center');
      await expect(widget(page, 'Checkouts')).toBeVisible();
      await expectNoSampleShell(page, repository.name);
    }
    await navigate(page, '/p/console/design/screen/example');
    await expect(content(page).getByText('This project does not exist', exact)).toBeVisible();
    await expect(content(page).locator('.workspace, [aria-label="Project views"]')).toHaveCount(0);
    await expectNoSampleShell(page, 'The old project address with watched repositories present');
    await expectUnchangedRecords(page);
    expectReadOnly(trace);
  });

  navigationTest('the keyboard Switcher shows the three origin states and reopens both saved routes after a reload', async ({ page, rig }) => {
    const { trace, watch } = await start(page, rig, []);
    const empty = await switcher(page);
    await expect(empty.dialog.getByText('Nothing matches ""', exact)).toBeVisible();
    await empty.search.fill('Machines');
    await expect(empty.dialog.getByRole('option')).toHaveCount(1);
    await empty.search.press('Enter');
    await expect(page.getByRole('heading', { name: 'Machines', ...exact })).toBeVisible();
    await navigate(page, '/home');

    for (const [index, repository] of saved.entries()) {
      watch.send({ change: { case: 'saved', value: repository }, position: { value: BigInt(index + 2) } });
    }
    const { dialog, search } = await switcher(page);
    await search.fill('Navigation');
    await expect(dialog.getByRole('option')).toHaveCount(3);
    const presentRow = repositoryOption(dialog, present);
    await expect(presentRow.getByText(origin, exact)).toBeVisible();
    await expect(presentRow.locator('a')).toHaveCount(0);
    await expect(repositoryOption(dialog, absent).getByText('No remote', exact)).toBeVisible();
    const withheldRow = repositoryOption(dialog, withheld);
    await expect.soft(withheldRow.getByText('—', exact), 'Switcher marks a withheld origin with a dash.').toBeVisible(quick);
    await expect.soft(withheldRow.getByText('No remote', exact), 'Switcher keeps the absent and withheld labels distinct.').toHaveCount(0, quick);
    await search.press('Escape');

    await reloadAt(page, rig, '/home', saved);
    for (const repository of [present, absent]) {
      const next = await switcher(page);
      await next.search.fill(repository === present ? 'watched.git' : 'Amber');
      await expect(next.dialog.getByRole('option')).toHaveCount(1);
      await expect(repositoryOption(next.dialog, repository)).toHaveAttribute('aria-selected', 'true');
      await next.search.press('Enter');
      await expect(next.dialog).toHaveCount(0);
      expect(new URL(page.url()).hash).toBe('#' + repoPath(repository));
      await expect(widget(page, 'Checkouts')).toBeVisible();
      await expectNoSampleShell(page, repository.name);
    }
    expectReadOnly(trace);
  });

  navigationTest('Checkouts shows every resolved folder and all three origin states', async ({ page, rig }) => {
    const { trace, watch } = await start(page, rig);
    const checkouts = widget(page, 'Checkouts');
    await navigate(page, repoPath(present, 'center'));
    await expect(checkouts).toBeVisible();
    await expect(checkouts.getByText('Remote', exact)).toBeVisible();
    await expect(checkouts.getByText(origin, exact)).toBeVisible();
    await expect(checkouts.getByRole('link', { name: origin, ...exact })).toHaveCount(0);
    const cells = checkouts.locator('dl > :is(dt, dd)');
    await expect(cells, 'Checkouts shows the complete machine and path for each watched folder.').toHaveText([
      'Remote', origin, worker.name, first.resolvedPath, worker.name, second.resolvedPath, survivor.name, elsewhere.resolvedPath,
    ], { useInnerText: true });

    rig.machines.attempts[0]!.send({ change: { case: 'removedId', value: worker.id }, position: { value: 2n } });
    watch.send({ change: { case: 'removedId', value: absent.id }, position: { value: 3n } });
    const remaining = create(RepositorySchema, {
      id: present.id, name: present.name, mainBranch: present.mainBranch, remote: origin, folders: [elsewhere],
    });
    watch.send({ change: { case: 'saved', value: remaining }, position: { value: 4n } });
    await expect(cells, 'Checkouts keeps only the surviving machine and its complete path.').toHaveText([
      'Remote', origin, survivor.name, elsewhere.resolvedPath,
    ], { useInnerText: true });
    expect(new URL(page.url()).hash).toBe('#' + repoPath(present, 'center'));

    await navigate(page, repoPath(withheld, 'center'));
    await expect.soft(checkouts.getByText('—', exact), 'Checkouts marks a withheld origin with a dash.').toBeVisible(quick);
    await expect.soft(checkouts.getByText('None', exact), 'Checkouts keeps the absent and withheld labels distinct.').toHaveCount(0, quick);
    await reloadAt(page, rig, repoPath(absent, 'center'), [absent], [worker]);
    await expect(checkouts.getByText('None', exact)).toBeVisible();
    expectReadOnly(trace);
  });

  navigationTest('Command center keeps its empty real widgets and its repository-specific project entry', async ({ page, rig }) => {
    const { trace } = await start(page, rig);
    await navigate(page, repoPath(present));
    await expectRepositoryTabs(page, present, 'Command center');
    const projects = widget(page, 'Projects');
    await expect(projects.getByText('No projects', exact)).toBeVisible();
    await expect(widget(page, 'Open requests').getByText('Nothing waits for you in ' + present.name + '.', exact)).toBeVisible();

    const newProject = projects.getByRole('button', { name: 'New project', ...exact });
    await expect(newProject).toBeEnabled();
    await expectProjectIntent(newProject, present);

    for (const [name, sample] of [
      ['Plan', 'No goals'], ['Sprints merged', 'Last 4 weeks'], ['Full runs', 'Passed, last 12 weeks'],
      ['Usage by project', 'Tokens used'], ['Base branch', 'Last commit'],
    ] as const) {
      const region = widget(page, name);
      await expectUnavailable(region, 'This page', name);
      await expect.soft(region.getByText(sample, exact), name + ' has no sample metric or content.').toHaveCount(0, quick);
    }

    await page.evaluate(() => window.lgtm.act.setPref('panelOpen', false));
    await expectNoSampleShell(page, 'Command center with its agent panel preference closed');
    await page.evaluate(() => window.lgtm.act.setPref('panelOpen', true));
    await expectNoSampleShell(page, 'Command center with its agent panel preference open');
    await expectUnchangedRecords(page);
    expectReadOnly(trace);
  });

  navigationTest('saved route tabs and deep links keep navigation and show unavailable bodies', async ({ page, rig }) => {
    const { trace } = await start(page, rig, [present]);
    await navigate(page, repoPath(present, 'center'));
    for (const { tab, child, subject, sample } of routeFamilies) {
      await tabs(page).getByRole('tab', { name: tab, ...exact }).click();
      expect(new URL(page.url()).hash).toBe('#' + repoPath(present, child));
      await expectRepositoryTabs(page, present, tab);
      if (child === 'design') await expectDesignNav(page, 'Screens');
      await expectUnavailable(content(page), subject, child);
      await expect.soft(content(page).locator(sample), child + ' mounts no sample controls or content.').toHaveCount(0, quick);
      await expectNoSampleShell(page, child);
    }

    const design = routeFamilies[1]!;
    for (const [child, active] of designChildren) {
      if (active === 'Browser' || active === 'Architecture' || child === 'design/file') {
        await designNav(page).getByRole('link', { name: active, ...exact }).click();
      } else if (child === 'design/screen/example') {
        await reloadAt(page, rig, repoPath(present, child), [present], []);
      } else {
        await navigate(page, repoPath(present, child));
      }
      expect(new URL(page.url()).hash).toBe('#' + repoPath(present, child));
      await expectRepositoryTabs(page, present, 'Design');
      await expectDesignNav(page, active);
      await expectUnavailable(content(page), design.subject, child);
      await expect.soft(content(page).locator(design.sample), child + ' mounts no canvas, map or preview.').toHaveCount(0, quick);
      await expectNoSampleShell(page, child);
    }

    for (const { tab, child, subject, sample } of routeFamilies.slice(2)) {
      const nested = child === 'files' ? 'files/src/index.ts' : 'git/branch/fixture-trunk';
      await reloadAt(page, rig, repoPath(present, nested), [present]);
      await expectRepositoryTabs(page, present, tab);
      await expectUnavailable(content(page), subject, nested);
      await expect.soft(content(page).locator(sample), nested + ' mounts no sample controls or content.').toHaveCount(0, quick);
      await expectNoSampleShell(page, nested);
    }

    await page.evaluate(() => window.lgtm.act.setPref('termOpen', true));
    await reloadAt(page, rig, repoPath(present, 'terminal'), [present]);
    await expectUnavailable(content(page), 'The terminal', 'The direct Terminal address');
    await expect.soft(widget(page, 'Projects'), 'The terminal address renders its own body.').toHaveCount(0, quick);
    await expectNoSampleShell(page, 'The direct Terminal address with the sample terminal preference open');
    await expectUnchangedRecords(page);
    expectReadOnly(trace);
  });

  navigationTest('missing, folderless and removed repository addresses have distinct bodies', async ({ page, rig }) => {
    const { trace } = await start(page, rig, [], []);
    const addresses = ['', 'center', 'board', 'design/screen/example', 'files/src/index.ts', 'git/branch/main', 'terminal'];
    for (const child of addresses) {
      await navigate(page, repoPath(present, child));
      await expect(content(page).getByText(missing, exact)).toBeVisible();
      await expect(tabs(page)).toHaveCount(0);
    }

    const address = repoPath(present, 'design/screen/example');
    const current = await reloadAt(page, rig, address, [], []);
    await expect(content(page).getByText(missing, exact)).toBeVisible();
    await content(page).getByRole('link', { name: 'Go to Home', ...exact }).click();
    await expect(page.getByRole('heading', { name: 'Needs attention', ...exact })).toBeVisible();

    const folderless = create(RepositorySchema, { ...present, folders: [] });
    current.send({ change: { case: 'saved', value: folderless }, position: { value: 2n } });
    current.send({ change: { case: 'saved', value: absent }, position: { value: 3n } });
    await openNavigation(page);
    const row = tree(page).getByRole('link', { name: present.name, ...exact });
    await row.click();
    const noSetup = '.page.rs, [aria-label="Machine"]';
    const unavailableFolderless = /^(This page|Files|The terminal) is not connected yet$/;
    await expectUnavailable(content(page), 'This page', 'The folderless tree link');
    for (const child of addresses) {
      await navigate(page, repoPath(present, child));
      const label = 'The folderless address ' + (child || 'root');
      await expectUnavailable(content(page), unavailableFolderless, label);
      await expect.soft(content(page).locator(noSetup), label + ' has no folder setup UI.').toHaveCount(0, quick);
      await expect.soft(content(page).getByRole('button', { name: /^(Set up on |Stop tracking$)/ })).toHaveCount(0, quick);
      await expectNoSampleShell(page, label);
    }

    const reloaded = await reloadAt(page, rig, address, [folderless, absent], []);
    await expectUnavailable(content(page), 'This page', 'The reloaded folderless deep link');
    reloaded.send({ change: { case: 'saved', value: present }, position: { value: 2n } });
    await expectRepositoryTabs(page, present, 'Design');
    reloaded.send({ change: { case: 'removedId', value: present.id }, position: { value: 3n } });
    await expect(content(page).getByText(missing, exact)).toBeVisible();
    expect(new URL(page.url()).hash).toBe('#' + address);
    await expect(tabs(page)).toHaveCount(0);
    await reloadAt(page, rig, address, [absent], []);
    await expect(content(page).getByText(missing, exact)).toBeVisible();
    await openNavigation(page);
    await expect(tree(page).getByRole('link')).toHaveCount(1);
    await expect(tree(page).getByRole('link', { name: absent.name, ...exact })).toHaveAttribute('href', '#' + repoPath(absent));
    await tree(page).getByRole('link', { name: absent.name, ...exact }).click();
    await expectRepositoryTabs(page, absent, 'Command center');
    const { dialog, search } = await switcher(page);
    await search.fill('Quartz');
    await expect(dialog.getByText('Nothing matches "Quartz"', exact)).toBeVisible();
    await expectUnchangedRecords(page);
    expectReadOnly(trace);
  });
});

navigationTest('watched repository origins wrap in the Switcher without hiding either suffix', async ({ page, rig }) => {
  const group = 'https://example.com/' + 'repository'.repeat(20) + '/shared/workspace/';
  const fixture = (remote: string, index: number) => ({
    remote,
    repository: create(RepositorySchema, {
      id: id('96' + index), name: 'Wrap ' + index, mainBranch: 'trunk', remote,
      folders: [folder('97' + index, worker, 'wrap-' + index)],
    }),
  });
  const longOrigins = [fixture(group + 'first.git', 1), fixture(group + 'second.git', 2)];
  const shortOrigin = fixture('https://example.com/r', 3);
  const repositories = [...longOrigins, shortOrigin].map(({ repository }) => repository);

  await start(page, rig, repositories, [worker]);
  const { dialog, search } = await switcher(page);
  await search.fill('Wrap');
  const list = dialog.getByRole('listbox');
  await expect(list.getByRole('option')).toHaveCount(repositories.length);

  const measure = async ({ repository, remote }: ReturnType<typeof fixture>) => {
    const row = repositoryOption(dialog, repository);
    const value = row.getByText(remote, exact);
    await expect(value).toBeVisible();
    const layout = await value.evaluate((element) => {
      const range = document.createRange();
      range.selectNodeContents(element);
      const box = element.getBoundingClientRect();
      const text = range.getBoundingClientRect();
      const fits = text.left >= box.left - 1 && text.right <= box.right + 1 &&
        text.top >= box.top - 1 && text.bottom <= box.bottom + 1 &&
        text.left >= 0 && text.right <= document.documentElement.clientWidth;
      return {
        lines: new Set(Array.from(range.getClientRects(), ({ top }) => top)).size,
        scrollWidth: element.scrollWidth,
        clientWidth: element.clientWidth,
        fits,
        textOverflow: getComputedStyle(element).textOverflow,
      };
    });
    const height = await row.evaluate((element) => Math.round(element.getBoundingClientRect().height));
    return { ...layout, height };
  };

  const short = await measure(shortOrigin);
  expect.soft(short.lines, 'The short origin stays on one line.').toBe(1);
  expect.soft(short.fits, 'The short origin fits on screen in its box.').toBe(true);
  for (const origin of longOrigins) {
    await navigationTest.step('Origin ' + origin.remote.slice(group.length), async () => {
      const wrapped = await measure(origin);
      expect.soft(wrapped.scrollWidth, 'The origin has no horizontal overflow.').toBeLessThanOrEqual(wrapped.clientWidth);
      expect.soft(wrapped.fits, 'The full origin fits on screen in its box with text-overflow ' + wrapped.textOverflow + '.').toBe(true);
      expect.soft(wrapped.lines, 'The origin wraps onto further lines.').toBeGreaterThan(1);
      expect.soft(wrapped.height, 'The Switcher option can grow beyond a single line.').toBeGreaterThan(short.height);
    });
  }
  const listOverflow = await list.evaluate((element) => element.scrollWidth - element.clientWidth);
  expect.soft(listOverflow, 'The Switcher list has no horizontal scroll.').toBeLessThanOrEqual(0);
  const pageOverflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  expect(pageOverflow, 'The page has no horizontal scroll with the Switcher open.').toBeLessThanOrEqual(0);
});
