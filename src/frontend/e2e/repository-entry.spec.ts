import { create, toJson, type MessageInitShape } from '@bufbuild/protobuf';
import { ClientSchema, ClientState, type Client, type WatchClientsResponseSchema } from '@lgtm/api/clients';
import {
  AddRepositoryResponseSchema, InspectFolderResponseSchema, ProjectService, RepositoryFolderSchema,
  RepositorySchema, type Repository, type WatchRepositoriesResponseSchema,
} from '@lgtm/api/projects';
import { expect, type Browser, type Locator, type Page } from '@playwright/test';
import {
  controlledWatch, observeWebsite, procedure, serveRepositoryDiscovery, serveRepositorySession,
  serveWebsite, sessionRegistry, startStandaloneMock, test,
} from './support/repository-server';

const exact = { exact: true };
const inspectPath = '/' + ProjectService.typeName + '/' + ProjectService.method.inspectFolder.name;
const addPath = '/' + ProjectService.typeName + '/' + ProjectService.method.addRepository.name;
const reads: readonly string[] = [procedure.discovery, procedure.signIn, procedure.machines, procedure.repositories];
const unavailable = 'Projects are not connected to this server yet.';

const machine = (id: string, name: string, state = ClientState.ONLINE) =>
  create(ClientSchema, { id, name, state, workspace: '/workspaces/' + id });
const offline = machine('offline', 'Offline build machine', ClientState.OFFLINE);
const first = machine('studio', 'Online machine for repository integration and build verification');
const second = machine('worker', 'Online machine for cross-platform checkout and browser verification');
const waiting = machine('waiting', 'Machine awaiting approval', ClientState.WAITING_FOR_APPROVAL);
const fleet = [offline, first, second, waiting];
const namedRepository = (id: string, name: string) => create(RepositorySchema, { id, name, mainBranch: 'trunk' });

function checkout(repository: Repository, id: string, clientId: string, resolvedPath: string) {
  const folder = create(RepositoryFolderSchema, { id, clientId, resolvedPath });
  return { repository: create(RepositorySchema, { ...repository, folders: [...repository.folders, folder] }), folder };
}

const projects = namedRepository('018f2ad2-abcd-7456-8123-000000000101', 'Repository for reviewing watched attachments and restored project forms');
const existing = checkout(projects, '018f2ad2-abcd-7456-8123-000000000102', first.id, '/workspaces/studio/project');
const attached = checkout(existing.repository, '018f2ad2-abcd-7456-8123-000000000103', second.id, '/workspaces/worker/project');

type MachineFrame = MessageInitShape<typeof WatchClientsResponseSchema>;
type RepositoryFrame = MessageInitShape<typeof WatchRepositoriesResponseSchema>;
type Rig = {
  web: string;
  origin: string;
  machines: ReturnType<typeof controlledWatch<MachineFrame>>;
  repositories: ReturnType<typeof controlledWatch<RepositoryFrame>>;
};

const entryTest = test.extend<{ rig: Rig }>({
  rig: async ({ website }, use) => {
    const web = await serveWebsite(website);
    const machines = controlledWatch<MachineFrame>();
    const repositories = controlledWatch<RepositoryFrame>();
    try {
      const server = await serveRepositorySession({
        pageOrigin: web.origin, hostname: '127.0.0.1', listener: 'entry', registry: sessionRegistry(),
        controlled: { services: [ProjectService.typeName], machines: machines.read, repositories: repositories.read },
      });
      try {
        await use({ web: web.origin, origin: server.origin, machines, repositories });
      } finally {
        await server.stop();
      }
    } finally {
      await web.stop();
    }
  },
});

type Trace = Awaited<ReturnType<typeof observeWebsite>>;

const addDialog = (page: Page) => page.getByRole('dialog', { name: /^Add (?:repository$|.+ to a machine$)/ });
const projectDialog = (page: Page) => page.getByRole('dialog', { name: 'New project', ...exact });
const machineChoice = (form: Locator, name: 'Machine' | 'Run on') => form.getByRole('combobox', { name, ...exact });
const repositoryChoice = (form: Locator) => form.getByRole('combobox', { name: 'Repository', ...exact });
const goalField = (form: Locator) => form.getByRole('textbox', { name: 'Goal', ...exact });
const createButton = (form: Locator) => form.getByRole('button', { name: 'Create project', ...exact });
const addButton = (form: Locator) => form.getByRole('button', { name: 'Add repository', ...exact });
const plus = (page: Page) => page.getByRole('button', { name: 'Add a repository', ...exact });
const cancel = (form: Locator) => form.getByRole('button', { name: 'Cancel', ...exact });
const attachButton = (form: Locator) => form.getByRole('button', { name: /^Add to(?: |$)/ });
const fact = (form: Locator, label: string) => form.getByText(label, exact).locator('+ dd');

async function signIn(page: Page, origin?: string) {
  if (origin) {
    await page.getByLabel('Server address', exact).fill(origin);
    await page.getByRole('button', { name: 'Connect', ...exact }).click();
  } else {
    await page.getByRole('button', { name: 'Use sample data', ...exact }).click();
  }
  await page.getByLabel('Password', exact).fill('browser fixture input');
  await page.getByRole('button', { name: 'Sign in', ...exact }).click();
}

async function snapshot(rig: Rig, clients: Client[], repositories: Repository[], index = 0) {
  await expect.poll(() => rig.machines.attempts.length).toBe(index + 1);
  await expect.poll(() => rig.repositories.attempts.length).toBe(index + 1);
  rig.machines.attempts[index]!.send({ change: { case: 'all', value: { clients } }, position: { value: 1n } });
  rig.repositories.attempts[index]!.send({ change: { case: 'all', value: { repositories } }, position: { value: 1n } });
}

async function visit(page: Page, rig: Rig, address: string, clients: Client[], repos: Repository[] = []) {
  const trace = await observeWebsite(page, [rig.web, rig.origin]);
  await page.goto(rig.web + '/?data=worst#/home');
  await signIn(page, rig.origin);
  await snapshot(rig, clients, repos);
  await expect(page.getByRole('button', { name: 'Connection: Live', ...exact })).toBeVisible();
  await page.evaluate((address) => { location.hash = address; }, address);
  return trace;
}

async function openNavigation(page: Page) {
  const open = page.getByRole('button', { name: 'Open navigation', ...exact });
  if (page.viewportSize()?.width === 390 && !await page.locator('.drawer-open').count()) {
    await open.click();
    await expect(page.locator('.drawer-open')).toBeVisible();
  }
}

async function expectLayout(page: Page, form: Locator, ...controls: Locator[]) {
  await page.clock.runFor(48);
  await expect(page.locator('[role="dialog"]'), 'Exactly one dialog is mounted.').toHaveCount(1);
  await expect(form.locator(':focus'), 'Focus is visible inside the active dialog.').toBeInViewport();
  const overflow = await form.evaluate((dialog) => {
    const regions = { page: document.documentElement, dialog, body: dialog.querySelector('.dialog-body')! };
    return Object.entries(regions).flatMap(([name, { scrollWidth, clientWidth }]) =>
      scrollWidth <= clientWidth + 1 ? [] : [{ name, scrollWidth, clientWidth }]);
  });
  expect.soft(overflow, 'The page, dialog and dialog body fit horizontally.').toEqual([]);
  for (const control of controls) {
    await control.scrollIntoViewIfNeeded();
    await expect(control).toBeInViewport({ ratio: 0.9 });
  }
}

async function finishDialogRender(page: Page) {
  await page.clock.runFor(80);
  await expect(page.locator('.app')).not.toHaveAttribute('inert', '');
}

async function expectCalls(page: Page, trace: Trace, folderCalls: string[] = []) {
  await page.clock.runFor(1000);
  expect(trace.calls.filter((path) => !reads.includes(path)), 'Only the planned folder calls can leave the page.').toEqual(folderCalls);
  expect(trace.errors, 'The browser reports no asynchronous error.').toEqual([]);
  expect(await page.evaluate(() => window.lgtm.store.state.projects.length), 'No project is created in the browser store.').toBe(0);
}

async function compatibilityPlus(browser: Browser, page: Page, website: string) {
  const web = await serveWebsite(website);
  try {
    for (const mode of ['in-page sample', 'standalone mock', 'older server'] as const) {
      await test.step(mode + ' keeps Track repository', async () => {
        const context = await browser.newContext({ viewport: page.viewportSize(), serviceWorkers: 'block' });
        let server: { origin: string; stop(): Promise<void> } | undefined;
        try {
          if (mode === 'older server') {
            const old = await serveRepositoryDiscovery(web.origin, {}, [first]);
            server = old;
            old.releaseDiscovery();
            old.releaseMachines();
          } else if (mode === 'standalone mock') {
            server = await startStandaloneMock(web.origin);
          }
          const sample = await context.newPage();
          const trace = await observeWebsite(sample, server ? [web.origin, server.origin] : [web.origin]);
          await sample.addInitScript(() => localStorage.setItem('lgtm-mock:session', JSON.stringify('out')));
          await sample.goto(web.origin + '/?data=worst#/home');
          await signIn(sample, server?.origin);
          await expect(sample.getByRole('heading', { name: 'Needs attention', ...exact })).toBeVisible();
          await openNavigation(sample);
          await expect(plus(sample)).toHaveAttribute('data-tip', 'Add a repository');
          await plus(sample).click();
          const track = sample.getByRole('dialog', { name: 'Add a repository', ...exact });
          await expect(track.getByRole('textbox', { name: 'Search the repositories, or paste an HTTPS or SSH link' })).toBeVisible();
          expect(trace.calls.some((path) => path.startsWith('/' + ProjectService.typeName + '/'))).toBe(false);
          expect(trace.errors).toEqual([]);
        } finally {
          await context.close();
          await server?.stop();
        }
      });
    }
  } finally {
    await web.stop();
  }
}

async function saveFolder(page: Page, rig: Rig, value: ReturnType<typeof checkout>, options: {
  typed: string;
  position: bigint;
  target?: string;
  clean: boolean;
  requestKeys: Set<string>;
}) {
  const form = addDialog(page);
  const { folder, repository } = value;
  const remote = 'https://example.invalid/repositories/' + folder.resolvedPath.split('/').at(-1) + '.git';
  const inspectReply = create(InspectFolderResponseSchema, {
    facts: { mainBranch: 'trunk', checkedOutBranch: 'review', clean: options.clean, remote, resolvedPath: folder.resolvedPath },
  });
  const reply = create(AddRepositoryResponseSchema, value);
  const inspectUrl = rig.origin + inspectPath;
  const addUrl = rig.origin + addPath;
  await page.route(inspectUrl, (route) => route.fulfill({ json: toJson(InspectFolderResponseSchema, inspectReply) }));
  let release!: () => void;
  const allowed = new Promise<void>((resolve) => { release = resolve; });
  await page.route(addUrl, async (route) => {
    await allowed;
    await route.fulfill({ json: toJson(AddRepositoryResponseSchema, reply) });
  });
  try {
    const inspect = page.waitForRequest(inspectUrl);
    await form.getByRole('textbox', { name: /^Folder on / }).fill(options.typed);
    await page.clock.runFor(800);
    const inspection = (await inspect).postDataJSON();
    expect(inspection.clientId === folder.clientId && inspection.path === options.typed,
      'The inspection uses the selected machine and exact path.').toBe(true);
    await expect(fact(form, 'Base branch')).toHaveText('trunk');
    await expect(fact(form, 'Remote')).toHaveText(remote);
    await expect(fact(form, 'Status')).toHaveText(options.clean
      ? 'A git repository with a clean working tree' : 'This Git repository has uncommitted changes.');
    const pending = page.waitForRequest(addUrl);
    await addButton(form).click();
    const request = await pending;
    const key = request.headers()['lgtm-request-key'];
    expect(Boolean(key && !options.requestKeys.has(key)), 'Each save has its own nonempty request key.').toBe(true);
    if (key) options.requestKeys.add(key);
    const add = request.postDataJSON();
    expect(add.clientId === folder.clientId && add.path === options.typed && add.repositoryId === options.target,
      'The save uses the selected machine, exact path and optional target.').toBe(true);
    rig.repositories.attempts[0]!.send({ change: { case: 'saved', value: repository }, position: { value: options.position } });
    await expect.poll(() => page.evaluate(({ repositoryId, folderId }) =>
      window.lgtm.store.state.repos.find((row) => row.id === repositoryId)?.folderRecords?.find((row) => row.id === folderId),
    { repositoryId: repository.id, folderId: folder.id })).toMatchObject({
      id: folder.id, clientId: folder.clientId, resolvedPath: folder.resolvedPath,
    });
    await expect(form.getByRole('button', { name: 'Adding…', ...exact })).toBeDisabled();
    release();
    await expect(form).toHaveCount(0);
  } finally {
    release();
    await page.unroute(inspectUrl);
    await page.unroute(addUrl);
  }
}

async function opensSavedCheckout(page: Page, value: ReturnType<typeof checkout>) {
  const { repository, folder } = value;
  const address = '/r/' + repository.id;
  await openNavigation(page);
  const tree = page.getByRole('navigation', { name: 'Repositories', ...exact });
  const link = tree.getByRole('link', { name: repository.name, ...exact });
  await expect(link).toHaveCount(1);
  await expect(link).toHaveAttribute('href', '#' + address);
  await link.click();
  await expect(page.getByText(folder.resolvedPath, exact)).toBeVisible();

  await page.evaluate(() => { location.hash = '/home'; });
  await expect(page).toHaveURL(/#\/home$/);
  await page.keyboard.press('ControlOrMeta+k');
  const switcher = page.getByRole('dialog', { name: 'Find project or agent', ...exact });
  const search = switcher.getByRole('textbox');
  await search.fill(repository.name);
  const option = switcher.getByRole('option').filter({ has: page.getByText(repository.name, exact) });
  await expect(option).toHaveCount(1);
  await option.click();
  await expect(switcher).toHaveCount(0);
  await expect(page).toHaveURL(new RegExp('#' + address + '(?:/center)?$'));
  await expect(page.getByText(folder.resolvedPath, exact)).toBeVisible();
  await expect(page.locator('.main').getByText('No projects', exact)).toBeVisible();
  await expect(page.locator('.term, .term-none, [aria-label="Terminal sessions"], [aria-label="Project views"]')).toHaveCount(0);
}

entryTest('add uses the sidebar or the opening machine', async ({ browser, page, rig, website }) => {
  await compatibilityPlus(browser, page, website);
  const trace = await visit(page, rig, '/machines', fleet);
  const form = addDialog(page);
  const a = checkout(namedRepository('018f2ad2-abcd-7456-8123-000000000111', 'Sidebar checkout'),
    '018f2ad2-abcd-7456-8123-000000000112', second.id, '/workspaces/worker/sidebar-resolved');
  const b = checkout(namedRepository('018f2ad2-abcd-7456-8123-000000000121', 'Machines checkout'),
    '018f2ad2-abcd-7456-8123-000000000122', second.id, '/workspaces/worker/machines-resolved');
  const requestKeys = new Set<string>();
  const chooser = machineChoice(form, 'Machine');

  await page.getByRole('button', { name: 'Show details for ' + second.name, ...exact }).click();
  await page.getByRole('button', { name: 'Add repository', ...exact }).click();
  await expect(chooser, 'The opened machine takes precedence over the first online machine.').toHaveValue(second.id);
  await cancel(form).click();
  await finishDialogRender(page);
  await page.getByRole('button', { name: 'Actions for ' + second.name, ...exact }).click();
  await page.getByRole('menuitem', { name: 'Add repository', ...exact }).click();
  await expect(chooser, 'The machine menu supplies its own opener.').toHaveValue(second.id);
  await saveFolder(page, rig, b, { typed: '/workspaces/worker/machines alias', position: 2n, clean: false, requestKeys });
  await finishDialogRender(page);

  await openNavigation(page);
  await expect(plus(page)).toHaveAttribute('data-tip', 'Add a repository');
  await plus(page).click();
  await expect(form, 'The advertising server opens Add repository directly from the sidebar.').toBeVisible();
  await expect(chooser).toHaveValue(first.id);
  await expect(chooser.getByRole('option', { name: waiting.name })).toHaveCount(0);
  await chooser.selectOption(second.id);
  const unrelated = machine('unrelated', 'Newly watched unrelated machine');
  rig.machines.attempts[0]!.send({ change: { case: 'saved', value: unrelated }, position: { value: 2n } });
  await expect(chooser.getByRole('option', { name: unrelated.name, ...exact })).toHaveCount(1);
  await expect(chooser).toHaveValue(second.id);
  await expectLayout(page, form, chooser, addButton(form), cancel(form));
  await saveFolder(page, rig, a, { typed: '/workspaces/worker/sidebar alias', position: 3n, clean: true, requestKeys });

  await page.reload();
  await snapshot(rig, fleet, [a.repository, b.repository], 1);
  for (const value of [a, b]) await opensSavedCheckout(page, value);
  await expectCalls(page, trace, [inspectPath, addPath, inspectPath, addPath]);
});

async function invalidProject(form: Locator, message: string) {
  const goal = goalField(form);
  const button = createButton(form);
  await expect(form.getByText(message, exact)).toBeVisible();
  await expect(button).toBeDisabled();
  // The native click exercises the disabled submit without Playwright waiting for enablement.
  await button.evaluate((control: HTMLButtonElement) => control.click());
  await goal.press('Enter');
  await expect(form.getByText(message, exact)).toBeVisible();
  await expect(form.getByText(unavailable, exact)).toHaveCount(0);
}

async function expectUnavailable(page: Page, form: Locator, goal: string, repository: string) {
  const message = form.getByText(unavailable, exact);
  await expect(form.locator('.dialog-body [data-empty]').getByText(unavailable, exact)).toBeVisible();
  await expect(form.locator('.dialog-foot').getByText(unavailable, exact)).toHaveCount(0);
  await expect(page.getByRole('status').getByText(unavailable, exact)).toHaveCount(0);
  const followsFields = await message.evaluate((label) => {
    const body = label.closest('.dialog-body')!;
    return Array.from(body.querySelectorAll('textarea, select, input, [role="group"]')).every((field) =>
      Boolean(field.compareDocumentPosition(label) & Node.DOCUMENT_POSITION_FOLLOWING));
  });
  expect(followsFields, 'The unavailable response follows the form fields.').toBe(true);
  await page.clock.runFor(10_000);
  await expect(message).toBeVisible();
  await expect(goalField(form)).toHaveValue(goal);
  await expect(machineChoice(form, 'Run on')).toHaveValue(second.id);
  await expect(repositoryChoice(form)).toHaveValue(repository);
  await expectLayout(page, form, goalField(form), createButton(form), cancel(form));
}

entryTest('new project keeps the goal and reports unavailable', async ({ page, rig }) => {
  const trace = await visit(page, rig, '/r/' + existing.repository.id, fleet, [existing.repository]);
  const form = projectDialog(page);
  const add = addDialog(page);
  const goal = 'Keep the unique goal for attachment and return verification.';
  const repo = repositoryChoice(form);
  await page.locator('.main').getByRole('button', { name: 'New project', ...exact }).click();
  await expect(repo).toHaveValue(existing.repository.id);
  await expect(machineChoice(form, 'Run on')).toHaveValue(first.id);
  await invalidProject(form, 'Write the goal.');
  await goalField(form).fill(goal);
  await repo.selectOption({ label: 'New repository…' });
  await expect(form.getByText(unavailable, exact)).toHaveCount(0);
  await invalidProject(form, 'Name the new repository.');
  await repo.selectOption(existing.repository.id);
  await machineChoice(form, 'Run on').selectOption(second.id);
  await invalidProject(form, existing.repository.name + ' is not on ' + second.name + '.');
  await expectLayout(page, form, attachButton(form), createButton(form));
  await expectCalls(page, trace);

  await attachButton(form).click();
  await expect(add.getByRole('heading', { name: 'Add ' + existing.repository.name + ' to a machine', ...exact })).toBeVisible();
  await expect(machineChoice(add, 'Machine')).toHaveValue(second.id);
  const back = add.getByRole('button', { name: 'Back', ...exact });
  await expectLayout(page, add, back, addButton(add));
  await back.click();
  await expect(goalField(form), 'Back restores the exact goal.').toHaveValue(goal);
  await expect(repo).toHaveValue(existing.repository.id);
  await expect(machineChoice(form, 'Run on')).toHaveValue(second.id);
  await expectLayout(page, form, attachButton(form));

  await attachButton(form).click();
  // The initial path can start its own inspection. Install the fixture before advancing the debounce.
  await saveFolder(page, rig, attached, {
    typed: '/workspaces/worker/project alias', position: 2n, target: existing.repository.id, clean: true, requestKeys: new Set(),
  });
  await expect(goalField(form), 'The matched watched attachment restores the exact goal.').toHaveValue(goal);
  await expect(repo).toHaveValue(attached.repository.id);
  await expect(machineChoice(form, 'Run on')).toHaveValue(second.id);
  await expect(attachButton(form)).toHaveCount(0);

  const lead = form.getByRole('group', { name: 'Project lead', ...exact });
  for (const name of ['Harness', 'Subscription', 'Model', 'Effort']) {
    await expect(lead.getByRole('button', { name, ...exact })).toBeDisabled();
  }
  await expect(createButton(form), 'An attached repository and goal are sufficient without a lead.').toBeEnabled();
  await createButton(form).click();
  await expectUnavailable(page, form, goal, attached.repository.id);
  const edited = goal + ' Keep the revised wording.';
  await goalField(form).fill(edited);
  await expect(form.getByText(unavailable, exact)).toHaveCount(0);
  await goalField(form).press('Enter');
  await expectUnavailable(page, form, edited, attached.repository.id);

  await repo.selectOption({ label: 'New repository…' });
  await expect(form.getByText(unavailable, exact)).toHaveCount(0);
  const newRepo = form.getByRole('textbox', { name: 'New repository name', ...exact });
  await newRepo.fill('Repository creation stays unavailable');
  await expect(createButton(form)).toBeEnabled();
  await createButton(form).click();
  await expect(form.locator('.dialog-body [data-empty]').getByText(unavailable, exact)).toBeVisible();
  await expect(newRepo).toHaveValue('Repository creation stays unavailable');
  await expect(goalField(form)).toHaveValue(edited);
  await expectCalls(page, trace, [inspectPath, addPath]);
  expect(await page.evaluate(() => window.lgtm.store.state.repos.map((repo) => repo.id))).toEqual([attached.repository.id]);
  await cancel(form).click();
  await expect(form).toHaveCount(0);
  await finishDialogRender(page);
  await page.locator('.main').getByRole('button', { name: 'New project', ...exact }).click();
  await expect(form).toBeVisible();
  await expect(form.getByText(unavailable, exact)).toHaveCount(0);
});

for (const [name, initial] of [['zero machines', []], ['offline-only machines', [offline, waiting]]] as const) {
  entryTest('both dialogs have an empty machine choice with ' + name, async ({ page, rig }) => {
    const trace = await visit(page, rig, '/projects', [...initial], [existing.repository]);
    const form = projectDialog(page);
    const add = addDialog(page);
    const addMachine = machineChoice(add, 'Machine');
    const runOn = machineChoice(form, 'Run on');
    const newProject = page.locator('.main').getByRole('button', { name: 'New project', ...exact });
    // Item 6 already supplies the folder form. The sidebar entry is tested separately.
    await page.evaluate(() => window.lgtm.act.openDialog({ type: 'add-repository' }));
    const path = add.getByRole('textbox', { name: 'Folder', ...exact });
    await expect(addMachine).toHaveValue('');
    await expect(addMachine.locator('option:checked')).toHaveText('');
    expect(await path.getAttribute('placeholder') ?? '').toBe('');
    await expect(path).toBeEditable();
    await path.fill('/workspaces/machine-not-yet-online');
    await expect(addButton(add)).toBeDisabled();
    await path.press('Enter');
    await expectLayout(page, add, path, addButton(add));
    await expectCalls(page, trace);
    await path.fill('');
    rig.machines.attempts[0]!.send({ change: { case: 'saved', value: second }, position: { value: 2n } });
    await expect(addMachine).toHaveValue(second.id);
    await cancel(add).click();
    await finishDialogRender(page);
    rig.machines.attempts[0]!.send({ change: { case: 'removedId', value: second.id }, position: { value: 3n } });
    await expect.poll(() => page.evaluate((id) => window.lgtm.store.state.machines.some((row) => row.id === id), second.id)).toBe(false);

    await newProject.click();
    await expect(form, 'New project remains usable with no eligible machine.').toBeVisible();
    await expect(runOn).toHaveValue('');
    await expect(runOn.locator('option:checked')).toHaveText('');
    await expect(runOn.getByRole('option', { name: waiting.name })).toHaveCount(0);
    await expect(createButton(form)).toBeDisabled();
    await expect(attachButton(form).and(form.locator(':enabled'))).toHaveCount(0);
    await goalField(form).fill('Keep this goal when a machine becomes available.');
    await goalField(form).press('Enter');
    await expect(form.getByText(unavailable, exact)).toHaveCount(0);
    await expectLayout(page, form, goalField(form), createButton(form));
    await expectCalls(page, trace);

    // The first worker is deliberately different from the compatibility workspace's hardcoded machine.
    const watch = rig.machines.attempts[0]!;
    watch.send({ change: { case: 'saved', value: second }, position: { value: 4n } });
    await expect(runOn.getByRole('option', { name: second.name, ...exact })).toHaveCount(1);
    await page.clock.runFor(48);
    await expect(runOn).toHaveValue(second.id);
    watch.send({ change: { case: 'saved', value: first }, position: { value: 5n } });
    await runOn.selectOption(first.id);
    await expect(runOn).toHaveValue(first.id);
    const unrelated = machine('unrelated', 'Newly watched unrelated machine');
    watch.send({ change: { case: 'saved', value: unrelated }, position: { value: 6n } });
    await expect(runOn.getByRole('option', { name: unrelated.name, ...exact })).toHaveCount(1);
    await page.clock.runFor(48);
    await expect(runOn, 'A later watch save preserves a still-present manual choice.').toHaveValue(first.id);

    await cancel(form).click();
    await finishDialogRender(page);
    await newProject.click();
    await expect(runOn, 'Generic New project chooses the first online machine.').toHaveValue(second.id);
    await cancel(form).click();
    await finishDialogRender(page);
    // A restored form can carry a machine and a repository in its dialog intent.
    await page.evaluate(({ machine, repo }) => window.lgtm.act.openDialog({ type: 'new-project', machine, repo }),
      { machine: first.id, repo: existing.repository.id });
    await expect(runOn, 'New project prefers its registered opener.').toHaveValue(first.id);
    await expect(repositoryChoice(form)).toHaveValue(existing.repository.id);
    await expectCalls(page, trace);
  });
}
