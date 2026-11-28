import { create, type MessageInitShape } from '@bufbuild/protobuf';
import { Code } from '@connectrpc/connect';
import { ClientSchema, ClientState, InstalledHarnessSchema, type Client, type WatchClientsResponseSchema } from '@lgtm/api/clients';
import { ProjectService, RepositorySchema, type Repository, type WatchRepositoriesResponseSchema } from '@lgtm/api/projects';
import { expect, type Page } from '@playwright/test';
import {
  controlledWatch, observeRepositoryWatch, observeWebsite, procedure, serveRepositoryDiscovery, serveRepositorySession, serveWebsite,
  sessionProcedure, sessionRegistry, startStandaloneMock, test,
} from './support/repository-server';

type MachineFrame = MessageInitShape<typeof WatchClientsResponseSchema>;
type RepositoryFrame = MessageInitShape<typeof WatchRepositoriesResponseSchema>;

const worker = create(ClientSchema, {
  id: 'routes-worker', name: 'Route fixture worker', state: ClientState.ONLINE, hostsServer: false,
  workspace: '/checkouts', os: 'Linux', version: '0.42.7-fixture', agentLimit: 13, cores: 6, memoryGigabytes: 24,
});
const scannedWorker = create(ClientSchema, {
  ...worker, harnessesScanned: true, harnesses: [create(InstalledHarnessSchema, { harness: 'codex', version: '7.8.9-fixture' })],
});
const firstPath = '/checkouts/first-copy';
const secondPath = '/checkouts/second-copy';
const otherPath = '/checkouts/other-machine';
const checkout = create(RepositorySchema, {
  id: '018f2ad2-bcde-7456-8123-000000000401', name: 'Route fixture repository', mainBranch: 'trunk',
  folders: [
    { id: '018f2ad2-bcde-7456-8123-000000000413', clientId: 'another-worker', resolvedPath: otherPath },
    { id: '018f2ad2-bcde-7456-8123-000000000412', clientId: worker.id, resolvedPath: secondPath },
    { id: '018f2ad2-bcde-7456-8123-000000000411', clientId: worker.id, resolvedPath: firstPath },
  ],
});
const survivingCheckout = create(RepositorySchema, {
  ...checkout, folders: checkout.folders.filter(({ clientId }) => clientId !== worker.id),
});

const tries = { timeout: 1800 };
const unavailable = 'The service does not send this to the website yet.';
const expired = 'You were signed out because the session expired. Sign in again and you return to where you were.';
const exact = { exact: true };
const readProcedures: readonly string[] = [procedure.discovery, procedure.signIn, procedure.machines, procedure.repositories];

type Rig = {
  web: string;
  server: Awaited<ReturnType<typeof serveRepositorySession>>;
  machines: ReturnType<typeof controlledWatch<MachineFrame>>;
  repositories: ReturnType<typeof controlledWatch<RepositoryFrame>>;
};

const routeTest = test.extend<{ rig: Rig }>({
  rig: async ({ website }, use) => {
    const web = await serveWebsite(website);
    const machines = controlledWatch<MachineFrame>();
    const repositories = controlledWatch<RepositoryFrame>();
    const server = await serveRepositorySession({
      pageOrigin: web.origin, hostname: '127.0.0.1', listener: 'routes', registry: sessionRegistry(),
      controlled: { services: [ProjectService.typeName], machines: machines.read, repositories: repositories.read },
    });
    try {
      await use({ web: web.origin, server, machines, repositories });
    } finally {
      await Promise.all([web.stop(), server.stop()]);
    }
  },
});

const busy = (page: Page) => page.locator('main[aria-busy="true"]');
const setupSteps = (page: Page) => page.locator('[aria-label="Setup steps"]');
const signInButton = (page: Page) => page.getByRole('button', { name: 'Sign in', ...exact });
const settings = (page: Page) => page.getByRole('navigation', { name: 'Settings', ...exact });

async function navigate(page: Page, address: string) {
  await page.evaluate((hash) => {
    if (location.hash === '#' + hash) return;
    return new Promise<void>((resolve) => {
      addEventListener('hashchange', () => resolve(), { once: true });
      location.hash = hash;
    });
  }, address);
}

async function selectServer(page: Page, address: string) {
  await page.getByLabel('Server address', exact).fill(address);
  await page.getByRole('button', { name: 'Connect', ...exact }).click();
}

async function signIn(page: Page) {
  await page.getByLabel('Password', exact).fill('browser fixture input');
  await signInButton(page).click();
}

async function start(page: Page, rig: Rig, address: string) {
  const trace = await observeRepositoryWatch(page, [rig.web, rig.server.origin]);
  await page.clock.resume();
  await page.goto(rig.web + '/?data=worst#' + address);
  await expect(page.getByText('Connect to a server', exact)).toBeVisible();
  await selectServer(page, rig.server.origin);
  await expect(page.getByText('Sign in to the server', exact)).toBeVisible();
  return trace;
}

function machinesSnapshot(clients: Client[], value = 1n): MachineFrame {
  return { change: { case: 'all', value: { clients } }, position: { value } };
}

function repositoriesSnapshot(repositories: Repository[], value = 1n): RepositoryFrame {
  return { change: { case: 'all', value: { repositories } }, position: { value } };
}

async function bothWatches(rig: Rig, index: number) {
  await expect.poll(() => [rig.machines.attempts.length, rig.repositories.attempts.length], tries)
    .toEqual([index + 1, index + 1]);
  return { machines: rig.machines.attempts[index]!, repositories: rig.repositories.attempts[index]! };
}

async function signedIn(page: Page, rig: Rig, clients: Client[]) {
  const trace = await start(page, rig, '/home');
  await signIn(page);
  const watches = await bothWatches(rig, 0);
  watches.machines.send(machinesSnapshot(clients));
  watches.repositories.send(repositoriesSnapshot([]));
  await expect(page.getByRole('heading', { name: 'Needs attention' })).toBeVisible();
  return { trace, watches };
}

async function reloadAt(page: Page, rig: Rig, address: string, clients: Client[], repositories: Repository[] = []) {
  const index = rig.machines.attempts.length;
  // Reload the requested hash before the current document can handle its alias.
  await page.evaluate((path) => history.replaceState(history.state, '', '#' + path), address);
  await page.reload();
  const watches = await bothWatches(rig, index);
  watches.machines.send(machinesSnapshot(clients));
  watches.repositories.send(repositoriesSnapshot(repositories));
  return watches;
}

async function openNavigation(page: Page) {
  const open = page.getByRole('button', { name: 'Open navigation', ...exact });
  if (await open.isVisible() && !await page.locator('.drawer-open').count()) await open.click();
}

async function shellLink(page: Page, hash: string) {
  await openNavigation(page);
  await page.getByRole('complementary', { name: 'Navigation', ...exact }).locator('a[href="#' + hash + '"]:visible').first().click();
}

async function expectUnavailable(page: Page, subject = 'This page') {
  await Promise.all([
    expect.soft(page.getByText(subject + ' is not connected yet', exact), subject + ' uses the unavailable title.').toBeVisible(tries),
    expect.soft(page.getByText(unavailable, exact), subject + ' uses the unavailable body.').toBeVisible(tries),
  ]);
}

function storedFacts(page: Page) {
  return page.evaluate(() => {
    const s = window.lgtm.store.state;
    return {
      machines: s.machines.map(({ id, slots, harnesses }) => ({ id, limit: slots.limit, harnesses })),
      joining: s.joining.length, repositories: s.repos.map(({ id }) => id),
      projects: s.projects.length, teams: s.teams.length, agents: s.agents.length, requests: s.requests.length,
      subscriptions: s.subscriptions.length, notifications: s.notifications.length, phoneOffer: s.phoneOffer,
      workflow: Object.keys(s.workflow), worktrees: s.worktrees.length,
      usageRows: s.usage.rows.length, terminalSessions: Object.keys(s.terminal?.byProject ?? {}),
      terminalPlace: s.ui.termPlace ?? null, setupData: Boolean(s.setupData), installing: Boolean(s.ui.installing),
    };
  });
}

// Compare sensitive settings and persistence inside the browser so assertion output contains only a boolean.
async function rememberSettings(page: Page) {
  return page.evaluateHandle(() => {
    const snapshot = () => JSON.stringify({
      settings: window.lgtm.store.state.settings,
      local: Object.entries(localStorage).sort(),
    });
    const initial = snapshot();
    return () => snapshot() === initial;
  });
}

async function assertReadOnly(trace: Awaited<ReturnType<typeof observeWebsite>>, allowed = readProcedures) {
  expect.soft(trace.calls.filter((path) => !allowed.includes(path)), 'Only supported procedures were sent.').toEqual([]);
  expect.soft(trace.errors).toEqual([]);
}

const noNotifications = async (page: Page) => {
  await expect(page.getByText('No notifications', exact)).toBeVisible();
  const forms = page.locator('form:is([aria-label="ntfy delivery"], [aria-label="Email delivery"], [aria-label="Quiet hours"])');
  await expect.soft(forms, 'The empty body does not mount delivery or quiet-hours forms.').toHaveCount(0, tries);
  await expect.soft(page.getByRole('switch')).toHaveCount(0, tries);
  await expect.soft(page.getByRole('button', { name: /^(Dismiss|Dismiss all|Mark all read)$/ })).toHaveCount(0, tries);
  await expect.soft(page.getByText('Alerts reach your phone', { exact: false })).toHaveCount(0);
};

const serviceSections = [
  ['defaults', 'Defaults'], ['remote', 'Remote access'], ['stall-watch', 'Stall watch'],
  ['workflow', 'Workflow'], ['git', 'Git messages'], ['connections', 'Connections'], ['about', 'About'],
] as const;

for (const [inventory, clients] of [['worker without a host', [worker]], ['empty inventory', []]] as const) {
  routeTest.describe('machines-only routes have no sample data with ' + inventory, () => {
    routeTest('service links and hash reload use the unavailable bodies', async ({ page, rig }) => {
      routeTest.setTimeout(95_000);
      const { trace } = await signedIn(page, rig, [...clients]);
      const before = await storedFacts(page);
      await shellLink(page, '/settings');
      await expect(settings(page)).toBeVisible();

      const appearance = async () => {
        await expect(page.getByRole('heading', { name: 'Appearance', ...exact })).toBeVisible();
        await expect(page.getByRole('group', { name: 'Theme', ...exact })).toBeVisible();
        await expect(page.getByRole('switch', { name: 'Fold the sidebar', ...exact })).toBeVisible();
      };
      await appearance();
      for (const address of ['/settings?section=appearance', '/settings?section=unknown']) {
        await reloadAt(page, rig, address, [...clients]);
        await appearance();
      }

      const sameAfterReload = await rememberSettings(page);
      for (const [section, label] of serviceSections) {
        await routeTest.step(label, async () => {
          const link = settings(page).getByRole('link', { name: label, ...exact });
          await link.click();
          await expect(link).toHaveAttribute('aria-current', 'page');
          await expectUnavailable(page);
          const pane = page.locator('.svc-pane');
          await expect.soft(pane.locator('form, input, select, textarea, [role="switch"]')).toHaveCount(0, tries);
          await expect.soft(pane.getByRole('button', { name: /check|restart|save|connect/i })).toHaveCount(0, tries);
        });
      }
      await settings(page).getByRole('link', { name: 'Harnesses', ...exact }).click();
      await expectUnavailable(page);
      const service = page.locator('.svc-page');
      await expect.soft(service.getByRole('button', { name: /add harness|check again|sign in|install/i })).toHaveCount(0, tries);
      await expect.soft(service.getByRole('heading', { name: 'Accounts', ...exact })).toHaveCount(0, tries);
      expect(await sameAfterReload.evaluate((same) => same())).toBe(true);
      await sameAfterReload.dispose();
      await reloadAt(page, rig, '/subscriptions', [...clients]);
      await expect(page).toHaveURL(/#\/harnesses$/);
      await expectUnavailable(page);

      await settings(page).getByRole('link', { name: 'Notifications', ...exact }).click();
      await noNotifications(page);
      await reloadAt(page, rig, '/settings?section=notifications', [...clients]);
      await expect(page).toHaveURL(/#\/notifications$/);
      const sameAfterAlias = await rememberSettings(page);
      await noNotifications(page);
      await settings(page).getByRole('link', { name: 'Workflows', ...exact }).click();
      await expect(settings(page).getByRole('link', { name: 'Workflows', ...exact })).toHaveAttribute('aria-current', 'page');
      await expectUnavailable(page);
      await expect.soft(service.getByLabel('Repository', exact)).toHaveCount(0, tries);
      await expect.soft(service.getByRole('button', { name: /run|load|clone|validate|save/i })).toHaveCount(0, tries);
      await shellLink(page, '/usage');
      await expectUnavailable(page);
      await expect.soft(page.locator('.main').getByRole('heading', { name: 'Findings', ...exact })).toHaveCount(0, tries);
      await expect.soft(page.locator('.main').getByRole('button', { name: /^(Day|Week|Month|Today|This week|This month)$/ })).toHaveCount(0, tries);

      await expect(storedFacts(page)).resolves.toEqual(before);
      expect(await sameAfterAlias.evaluate((same) => same())).toBe(true);
      await sameAfterAlias.dispose();
      await assertReadOnly(trace);
    });
  });
}

const projectAddresses = [
  '', 'center', 'board', 'end-result', 'questions', 'status', 'actions', 'design', 'files', 'git',
  'settings', 'conversation', 'sprint', 'task', 'team', 'chat', 'message-board', 'agents', 'map', 'terminal',
  'board/task/example', 'design/screen/example', 'files/src/index.ts', 'git/branch/main', 'settings/agents',
  'conversation/lead', 'sprint/example/tasks/example', 'task/example', 'team/example', 'message-board/post/example',
];

for (const [inventory, clients] of [['worker without a host', [worker]], ['empty inventory', []]] as const) {
  routeTest.describe('machines-only routes have no sample data with ' + inventory, () => {
    routeTest('the shell opens empty Home and projects and offers no simulated terminal', async ({ page, rig }) => {
      const { trace } = await signedIn(page, rig, [...clients]);
      const before = await storedFacts(page);
      const homeTable = page.locator('.main').getByRole('table');
      await expect(homeTable.getByText('No open requests', exact)).toBeVisible();

      await navigate(page, '/');
      await expect(page).toHaveURL(/#\/home$/);
      await expect(homeTable.getByText('No open requests', exact)).toBeVisible();
      await reloadAt(page, rig, '/home/questions', [...clients]);
      await expect(homeTable.getByText('No open request matches.', { exact: false })).toBeVisible();
      await navigate(page, '/home?type=approval');
      await expect(homeTable.getByRole('button', { name: 'Clear the filters', ...exact })).toBeVisible();
      await expect(page.getByRole('group', { name: 'Type', ...exact }).getByRole('button', { name: /approval/i }))
        .toHaveAttribute('aria-pressed', 'true');

      await shellLink(page, '/projects');
      const projects = page.locator('.main');
      await expect(projects.getByRole('heading', { name: 'All projects', ...exact })).toBeVisible();
      await expect(projects.getByText('No project matches.', exact)).toBeVisible();
      const newProject = projects.getByRole('button', { name: 'New project', ...exact });
      await expect(newProject).toBeEnabled();
      // Observe the entry point in the same turn. Item 7 owns the dialog's machine choice.
      const opensNewProject = await newProject.evaluate((button: HTMLButtonElement) => {
        const { act, store } = window.lgtm;
        button.click();
        const opened = store.state.dialog?.type === 'new-project';
        act.closeDialog();
        return opened;
      });
      expect(opensNewProject).toBe(true);
      await reloadAt(page, rig, '/home/projects', [...clients]);
      await expect(page).toHaveURL(/#\/projects$/);
      await expect(projects.getByText('No project matches.', exact)).toBeVisible();

      for (const tab of projectAddresses) {
        const address = '/p/console' + (tab ? '/' + tab : '');
        await navigate(page, address);
        await expect(page.getByText('This project does not exist', exact)).toBeVisible();
        await expect(page.locator('.workspace, .panel, [aria-label="Project views"]'), address).toHaveCount(0);
      }
      await reloadAt(page, rig, '/p/console/design/screen/example', [...clients]);
      await expect(page.getByText('This project does not exist', exact)).toBeVisible();
      await expect(page.locator('.workspace, .panel, [aria-label="Project views"]')).toHaveCount(0);
      await navigate(page, '/unrecognized');
      await expect(page.getByText('This page does not exist', exact)).toBeVisible();
      await page.getByRole('link', { name: 'Go to Home', ...exact }).click();
      await expect(homeTable.getByText('No open requests', exact)).toBeVisible();
      await navigate(page, '/login');
      await expect(signInButton(page)).toBeVisible();
      await expect(page.getByRole('button', { name: /^Change the server\./ })).toBeVisible();
      await navigate(page, '/home');
      await expect(homeTable.getByText('No open requests', exact)).toBeVisible();

      const expectTerminalUnavailable = async () => {
        await expectUnavailable(page, 'The terminal');
        await expect.soft(page.locator('[role="dialog"][aria-label="Open a terminal in"]')).toHaveCount(0, tries);
        await expect.soft(page.locator('.term, .term-none, [aria-label="Terminal sessions"]')).toHaveCount(0, tries);
        await expect.soft(page).toHaveURL(/#\/home$/, tries);
      };

      await page.keyboard.press('Control+Backquote');
      await expectTerminalUnavailable();
      await reloadAt(page, rig, '/home', [...clients]);
      await expect(homeTable.getByText('No open requests', exact)).toBeVisible();
      await expect(page.getByText('The terminal is not connected yet', exact)).toHaveCount(0);
      await openNavigation(page);
      const sidebar = page.getByRole('complementary', { name: 'Navigation', ...exact });
      await sidebar.getByRole('button', { name: /^Terminal/ }).click();
      await expectTerminalUnavailable();

      await expect(storedFacts(page)).resolves.toEqual(before);
      await assertReadOnly(trace);
    });
  });
}

routeTest('machines-only routes have no sample data in watched machine detail', async ({ page, rig }) => {
  const { trace } = await signedIn(page, rig, [worker]);
  await shellLink(page, '/settings');
  await settings(page).getByRole('link', { name: /^Machines/ }).click();
  await expect(page.getByRole('heading', { name: 'Machines', ...exact })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Add machine', ...exact })).toBeEnabled();
  const machines = page.locator('.svc-machines');
  const details = page.locator('.svc-detail');
  const openDetails = () => machines.getByRole('button', { name: 'Show details for ' + worker.name, ...exact }).click();
  await expect(machines.locator('tbody > tr')).toHaveCount(1);
  await expect(machines.getByText('Worker', exact)).toHaveCount(1);
  await expect(machines.getByText('Master', exact)).toHaveCount(0);
  await openDetails();
  await expect(details.getByText('No repositories', exact)).toBeVisible();
  await expect(details.getByText('No projects', exact)).toBeVisible();
  await expect(details.getByText(worker.workspace, exact)).toBeVisible();
  await expect.soft(details.getByText(worker.version, exact)).toBeVisible(tries);
  await expect.soft(details.getByText(/older than the service|up to date|latest version/i)).toHaveCount(0, tries);
  const limit = details.locator('dd').filter({ hasText: /\b13\b/ });
  await expect.soft(limit).toBeVisible(tries);
  await expect.soft(details.locator('dd input, dd select, dd textarea, dd [contenteditable="true"]')).toHaveCount(0, tries);
  await expect.soft(details.getByText(/not installed|signed (?:in|out)|npm install/i)).toHaveCount(0, tries);
  await expect.soft(details.getByText('Codex', exact)).toHaveCount(0, tries);
  await expect(details.getByRole('button', { name: 'Add repository', ...exact })).toBeEnabled();

  const watches = await reloadAt(page, rig, '/machines', [worker]);
  await openDetails();
  await expect(details.getByText('No repositories', exact)).toBeVisible();
  const actions = machines.getByRole('button', { name: 'Actions for ' + worker.name, ...exact });
  await actions.click();
  const menu = page.getByRole('menu');
  for (const name of ['Rename', 'Scan harnesses again', 'Add repository', 'Remove machine']) {
    await expect(menu.getByRole('menuitem', { name, ...exact })).toBeEnabled();
  }
  await menu.getByRole('menuitem', { name: 'Scan harnesses again', ...exact }).click();
  await expect.poll(() => rig.server.calls().filter(({ method }) => method === 'scan').length).toBe(1);
  watches.machines.send({ change: { case: 'saved', value: scannedWorker }, position: { value: 2n } });
  watches.repositories.send({ change: { case: 'saved', value: checkout }, position: { value: 2n } });
  await expect.soft(details.getByText('7.8.9-fixture', { exact: false })).toBeVisible(tries);
  await expect.soft(details.getByText(/signed (?:in|out)|not installed|npm install/i)).toHaveCount(0, tries);
  await expect.soft(details.locator('button').filter({ hasText: /install|sign in/i })).toHaveCount(0, tries);
  await expect(details.getByRole('link', { name: checkout.name, ...exact }).first()).toHaveAttribute('href', '#/r/' + checkout.id);
  for (const path of [firstPath, secondPath]) {
    await expect.soft(details.getByText(path, exact), 'Machine detail includes the watched folder ' + path + '.').toBeVisible(tries);
  }
  await expect.soft(details.getByText(otherPath, exact)).toHaveCount(0);
  await expect(details.getByText('No projects', exact)).toBeVisible();

  watches.repositories.send({ change: { case: 'saved', value: survivingCheckout }, position: { value: 3n } });
  await expect(details.getByText('No repositories', exact)).toBeVisible();
  await expect(details.getByText(firstPath, exact)).toHaveCount(0);
  await expect(details.getByText(secondPath, exact)).toHaveCount(0);
  watches.machines.send({ change: { case: 'removedId', value: worker.id }, position: { value: 3n } });
  await expect(machines.locator('tbody > tr')).toHaveCount(0);
  await expect(details).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Add machine', ...exact })).toBeEnabled();

  const reverse = await reloadAt(page, rig, '/machines', [scannedWorker], [checkout]);
  await openDetails();
  await expect(details.getByText(firstPath, exact)).toBeVisible();
  reverse.machines.send({ change: { case: 'removedId', value: worker.id }, position: { value: 2n } });
  await expect(details).toHaveCount(0);
  await expect(actions).toHaveCount(0);
  await expect(storedFacts(page)).resolves.toMatchObject({ machines: [], repositories: [checkout.id] });
  reverse.repositories.send({ change: { case: 'saved', value: survivingCheckout }, position: { value: 2n } });
  await expect.poll(() => page.evaluate((id) => {
    const repo = window.lgtm.store.state.repos.find((record) => record.id === id);
    return repo?.folderRecords?.map(({ resolvedPath }) => resolvedPath);
  }, checkout.id)).toEqual([otherPath]);

  await reloadAt(page, rig, '/machines', []);
  await expect(page.getByRole('heading', { name: 'Machines', ...exact })).toBeVisible();
  await expect(machines.locator('tbody > tr')).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Add machine', ...exact })).toBeEnabled();
  await assertReadOnly(trace, [...readProcedures, sessionProcedure.scan]);
});

routeTest('machines-only routes have no sample data in the connection readout', async ({ page, rig }) => {
  const { trace, watches } = await signedIn(page, rig, [worker]);
  const live = page.getByRole('button', { name: 'Connection: Live', ...exact });
  await live.click();
  await expect(page.getByRole('menu').getByText(/Connected to the service/)).toBeVisible();
  await page.keyboard.press('Escape');
  watches.repositories.send(Code.Aborted);

  const reconnecting = page.getByRole('button', { name: 'Connection: Reconnecting', ...exact });
  await reconnecting.click();
  const menu = page.getByRole('menu');
  await expect(menu.getByText('Reconnecting', exact)).toBeVisible();
  const connection = await page.evaluate(() => ({ ...window.lgtm.store.state.conn }));
  const reconnect = menu.getByRole('menuitem', { name: 'Reconnect now', ...exact });
  await expect.soft(reconnect, 'Repository mode does not offer a simulated reconnect.').toHaveCount(0, tries);
  if (await reconnect.count()) await reconnect.click();
  await expect.soft(reconnecting).toBeVisible(tries);
  await expect(storedFacts(page)).resolves.toMatchObject({ machines: [{ id: worker.id }] });
  expect.soft(await page.evaluate(() => ({ ...window.lgtm.store.state.conn }))).toEqual(connection);
  await assertReadOnly(trace);
});

routeTest('machines-only routes have no sample data on direct setup and after expiry', async ({ page, rig, browser, viewport }) => {
  await routeTest.step('Compatibility setup keeps its exemption and steps', async () => {
    const host = create(ClientSchema, {
      id: 'compat-host', name: 'Compatibility host', hostsServer: true, state: ClientState.ONLINE, workspace: '/checkouts',
    });
    const older = await serveRepositoryDiscovery(rig.web, { services: [] }, [host]);
    try {
      const standalone = await startStandaloneMock(rig.web);
      try {
        older.releaseDiscovery();
        older.releaseMachines();
        for (const [mode, address] of [
          ['in-page sample', null], ['standalone mock', standalone.origin], ['older server', older.origin],
        ] as const) {
          await routeTest.step(mode, async () => {
            const context = await browser.newContext({ viewport });
            try {
              const control = await context.newPage();
              const trace = await observeWebsite(control, [rig.web, ...(address ? [address] : [])]);
              await control.clock.resume();
              await control.goto(rig.web + '/#/login');
              if (address) await selectServer(control, address);
              else await control.getByRole('button', { name: 'Use sample data', ...exact }).click();
              await expect.poll(() => control.evaluate(() => window.lgtm.store.state.server.status)).toBe('ready');
              if (await control.evaluate(() => window.lgtm.store.state.session.status) !== 'in') await signIn(control);
              await expect.poll(() => control.evaluate(() => window.lgtm.store.state.server.synced)).toBe(true);
              await navigate(control, '/setup');
              await expect(setupSteps(control)).toBeVisible();
              await expect(control.getByRole('heading', { name: 'Harnesses', ...exact })).toBeVisible();
              for (const status of ['out', 'expired']) {
                // The sample controls set these states without restarting the compatibility workspace.
                await control.evaluate((next) => window.lgtm.act.setSession(next), status);
                await navigate(control, '/setup');
                await expect(setupSteps(control)).toBeVisible();
                await expect(control.getByText(expired, exact)).toHaveCount(0);
              }
              expect(trace.errors).toEqual([]);
              expect(trace.calls).not.toContain(procedure.repositories);
              expect(await control.evaluate((path) => window.discoveryProbe.calls.includes(path), procedure.repositories)).toBe(false);
            } finally {
              await context.close();
            }
          });
        }
      } finally {
        await standalone.stop();
      }
    } finally {
      await older.stop();
    }
  });

  const trace = await start(page, rig, '/setup');
  const before = await storedFacts(page);
  const same = await rememberSettings(page);
  const assertLoading = async () => {
    await expect(busy(page)).toBeVisible();
    await expect(setupSteps(page)).toHaveCount(0);
    await expect(page.getByText(unavailable, exact)).toHaveCount(0);
  };
  const signInToSetup = async (index: number) => {
    await signIn(page);
    const watches = await bothWatches(rig, index);
    await expect.soft(page, 'Sign-in keeps the requested setup address.').toHaveURL(/#\/setup$/, tries);
    // Reopen the same address to exercise its readiness gates even if sign-in redirected away.
    await navigate(page, '/setup');
    await assertLoading();
    watches.repositories.send(repositoriesSnapshot([]));
    await expect.poll(() => page.evaluate(() => {
      return window.repositoryWatchProbe.bridge?.currentRepositorySnapshot()?.repos.length;
    }), tries).toBe(0);
    await assertLoading();
    watches.machines.send(machinesSnapshot([]));
    await expect(busy(page)).toHaveCount(0);
    await expectUnavailable(page);
    await expect(setupSteps(page)).toHaveCount(0);
    await expect(page.getByRole('button', { name: /add harness|skip setup|check folder/i })).toHaveCount(0);
    return watches;
  };

  await expect(signInButton(page)).toBeVisible();
  await expect(setupSteps(page)).toHaveCount(0);
  expect(trace.calls).toEqual([procedure.discovery]);
  const initial = await signInToSetup(0);
  await navigate(page, '/home');
  await expect(page.getByRole('heading', { name: 'Needs attention' })).toBeVisible();

  initial.machines.send(Code.Unauthenticated);
  await expect(page.getByText(expired, exact)).toBeVisible();
  await navigate(page, '/setup');
  await expect(page.getByText(expired, exact)).toBeVisible();
  await expect(signInButton(page)).toBeVisible();
  await expect(setupSteps(page)).toHaveCount(0);
  await signInToSetup(1);

  await expect(storedFacts(page)).resolves.toEqual(before);
  expect(await same.evaluate((unchanged) => unchanged())).toBe(true);
  await same.dispose();
  await assertReadOnly(trace);
});

routeTest('watched machine folder paths wrap without hiding either suffix', async ({ page, rig }) => {
  const machine = create(ClientSchema, { ...worker, name: 'Worker' });
  const directory = '/checkouts/' + 'repository'.repeat(20) + '/shared/workspace/';
  const copies = [directory + 'first-copy', directory + 'second-copy'];
  const shortPath = '/checkouts/short';
  const repository = create(RepositorySchema, {
    id: checkout.id, name: 'Wrap', mainBranch: 'trunk',
    folders: [shortPath, ...copies].map((resolvedPath, index) => ({
      id: '018f2ad2-bcde-7456-8123-00000000042' + index, clientId: machine.id, resolvedPath,
    })),
  });

  const { watches } = await signedIn(page, rig, [machine]);
  watches.repositories.send({ change: { case: 'saved', value: repository }, position: { value: 2n } });
  await navigate(page, '/machines');
  await page.getByRole('button', { name: 'Show details for ' + machine.name, ...exact }).click();
  const details = page.locator('.svc-detail');
  const measure = async (path: string) => {
    const row = details.getByRole('listitem').filter({ has: page.getByText(path, exact) });
    const value = row.getByText(path, exact);
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

  const short = await measure(shortPath);
  expect(short.lines, 'The short folder path stays on one line.').toBe(1);
  expect(short.fits, 'The short folder path fits in its box.').toBe(true);
  for (const path of copies) {
    await routeTest.step('Folder ' + path.slice(directory.length), async () => {
      const wrapped = await measure(path);
      expect.soft(wrapped.scrollWidth, 'The folder path has no horizontal overflow.').toBeLessThanOrEqual(wrapped.clientWidth);
      expect.soft(wrapped.fits, 'The full folder path fits on screen in its box with text-overflow ' + wrapped.textOverflow + '.').toBe(true);
      expect.soft(wrapped.lines, 'The folder path wraps onto further lines.').toBeGreaterThan(1);
      expect.soft(wrapped.height, 'The folder row can grow beyond a single line.').toBeGreaterThan(short.height);
    });
  }
  const table = page.locator('.table-wrap').filter({ has: page.locator('.svc-machines') });
  const tableOverflow = await table.evaluate((element) => element.scrollWidth - element.clientWidth);
  expect.soft(tableOverflow, 'The Machines table has no horizontal scroll.').toBeLessThanOrEqual(0);
  const pageOverflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  expect(pageOverflow, 'The Machines page has no horizontal scroll.').toBeLessThanOrEqual(0);
});
