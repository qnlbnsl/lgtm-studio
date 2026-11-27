import { create, toJson, type MessageInitShape } from '@bufbuild/protobuf';
import { Code, createClient, createRouterTransport } from '@connectrpc/connect';
import { ClientSchema, ClientState, type Client, type WatchClientsResponseSchema } from '@lgtm/api/clients';
import {
  AddRepositoryResponseSchema, FolderFactsSchema, InspectFolderResponseSchema, ProjectService, RepositoryFolderSchema,
  RepositorySchema, type Repository, type WatchRepositoriesResponseSchema,
} from '@lgtm/api/projects';
import { expect, type Page } from '@playwright/test';
import { addRepository, folderIntent, inspectFolder } from '../src/server/folders';
import {
  controlledWatch, observeRepositoryWatch, procedure, serveRepositorySession, serveWebsite, sessionRegistry, test,
} from './support/repository-server';

const firstHost = 'first.lgtm.test';
const otherHost = 'other.lgtm.test';
const exact = { exact: true };
const prompt = { timeout: 2500 };
const getPath = (method: string) => '/' + ProjectService.typeName + '/' + method;
const methods = ProjectService.method;
const unaryPaths = { inspect: getPath(methods.inspectFolder.name), add: getPath(methods.addRepository.name) };
const clean = 'A git repository with a clean working tree';
const dirty = 'This Git repository has uncommitted changes.';
const removed = 'The machine was removed.';
const missing = 'The repository was not found.';
const generic = 'The server refused the request.';
const network = 'The server did not answer.';
const wait = 'The machine did not answer the folder request.';
const mismatch = 'The folder does not belong to the selected repository.';
const unreadable = 'The folder cannot be read.';
const signedOut = 'Sign in to continue.';
const typed = '~/Checkout with spaces ';
const alias = '/checkouts/Through a symlink';
const resolved = '/checkouts/canonical';
const goal = 'Keep this goal when the watched folder is attached.';

const machine = (id: string, name: string, state = ClientState.ONLINE) =>
  create(ClientSchema, { id, name, state, workspace: '/checkouts' });
const offline = machine('offline', 'Offline worker', ClientState.OFFLINE);
const first = machine('first', 'Build worker');
const second = machine('second', 'Second build worker with a long machine name for the narrow dialog');
const unrelated = machine('unrelated', 'Unrelated worker');
const newMachine = machine('new-session', 'Replacement session worker');
const queuedMachine = machine('queued', 'Obsolete queued worker');
const target = create(RepositorySchema, {
  id: '018f2ad2-bcde-7456-8123-000000000010', name: 'Watched checkout', mainBranch: 'trunk',
});
const folder = create(RepositoryFolderSchema, {
  id: '018f2ad2-bcde-7456-8123-000000000012', clientId: first.id, resolvedPath: resolved,
});
const otherFolder = create(RepositoryFolderSchema, {
  id: '018f2ad2-bcde-7456-8123-000000000011', clientId: first.id, resolvedPath: '/checkouts/other',
});
const saved = create(RepositorySchema, { ...target, folders: [otherFolder, folder] });
const partial = create(RepositorySchema, { ...target, folders: [otherFolder] });
// The alternate frames have one correct identity at a time.
const wrongMachine = create(RepositorySchema, {
  ...target, folders: [otherFolder, { ...folder, clientId: second.id }],
});
const wrongRepository = create(RepositorySchema, {
  ...target, id: '018f2ad2-bcde-7456-8123-000000000099', name: 'Different checkout', folders: [folder],
});

type MachineFrame = MessageInitShape<typeof WatchClientsResponseSchema>;
type RepositoryFrame = MessageInitShape<typeof WatchRepositoriesResponseSchema>;
type Procedure = keyof typeof unaryPaths;
type FolderBody = { clientId: string; path: string; repositoryId?: string };
type Unary = { procedure: Procedure; body: FolderBody; keyClass: number; signalClass: number; aborted: boolean };
type UnaryResult = { body: string; status: number } | 'network';
type HeldDelivery = { ready: boolean; finished: boolean; gate: Promise<void>; release(): void };
type HeldFolderReply = HeldDelivery & { procedure: Procedure; signal?: AbortSignal };

declare global {
  interface Window {
    addRepositoryProbe: {
      calls(): Unary[];
      reply(index: number, result: UnaryResult): void;
      holdMachineFrame(): HeldDelivery;
      takeMachineFrame(): HeldDelivery | undefined;
      holdFolderReply(procedure: Procedure): void;
      takeFolderReply(procedure: Procedure, signal?: AbortSignal): HeldFolderReply | undefined;
      heldMachine?: HeldDelivery;
      heldFolder?: HeldFolderReply;
    };
  }
}

type Rig = {
  web: string;
  origin: string;
  machines: ReturnType<typeof controlledWatch<MachineFrame>>;
  repositories: ReturnType<typeof controlledWatch<RepositoryFrame>>;
};

const folderTest = test.extend<{ rig: Rig }>({
  rig: async ({ website }, use) => {
    const web = await serveWebsite(website, firstHost);
    const machines = controlledWatch<MachineFrame>();
    const repositories = controlledWatch<RepositoryFrame>();
    try {
      const listener = await serveRepositorySession({
        pageOrigin: web.origin, hostname: firstHost, listener: 'folders', registry: sessionRegistry(),
        controlled: { services: [ProjectService.typeName], machines: machines.read, repositories: repositories.read },
      });
      try {
        await use({ web: web.origin, origin: listener.origin, machines, repositories });
      } finally {
        await listener.stop();
      }
    } finally {
      await web.stop();
    }
  },
});

folderTest.use({ launchOptions: {
  args: [`--host-resolver-rules=MAP ${firstHost} 127.0.0.1, MAP ${otherHost} 127.0.0.1`, '--no-proxy-server'],
} });

function instrument(page: Page, origins: string[]) {
  return page.addInitScript(({ origins, routes }) => {
    const fetch = window.fetch.bind(window);
    const keys = new Map<string, number>();
    const signals = new Map<AbortSignal, number>();
    const calls: {
      procedure: Procedure; body: FolderBody; keyClass: number; signal?: AbortSignal | null;
      settle(result: UnaryResult): void;
    }[] = [];
    let nextMachine: HeldDelivery | undefined;
    let nextFolder: HeldFolderReply | undefined;
    function hold() {
      let release!: () => void;
      const gate = new Promise<void>((done) => { release = done; });
      return { ready: false, finished: false, gate, release };
    }
    window.addRepositoryProbe = {
      calls: () => calls.map(({ procedure, body, keyClass, signal }) => ({
        procedure, body, keyClass, signalClass: signal ? signals.get(signal)! : 0, aborted: signal?.aborted ?? false,
      })),
      reply(index, result) {
        const call = calls[index];
        if (!call) throw new Error('The fixture received no folder call at that index.');
        call.settle(result);
      },
      holdMachineFrame() {
        nextMachine = hold();
        return window.addRepositoryProbe.heldMachine = nextMachine;
      },
      takeMachineFrame() {
        const held = nextMachine;
        nextMachine = undefined;
        if (held) held.ready = true;
        return held;
      },
      holdFolderReply(procedure) {
        nextFolder = { ...hold(), procedure };
        window.addRepositoryProbe.heldFolder = nextFolder;
      },
      takeFolderReply(procedure, signal) {
        if (nextFolder?.procedure !== procedure) return undefined;
        const held = nextFolder;
        nextFolder = undefined;
        held.signal = signal;
        return held;
      },
    };
    window.fetch = (input, init) => {
      const url = new URL(input instanceof Request ? input.url : String(input), location.href);
      const kind = url.pathname === routes.inspect ? 'inspect' : url.pathname === routes.add ? 'add' : null;
      if (!origins.includes(url.origin) || !kind) return fetch(input, init);
      const key = new Headers(init?.headers).get('Lgtm-Request-Key');
      if (key && !keys.has(key)) keys.set(key, keys.size + 1);
      if (init?.signal && !signals.has(init.signal)) signals.set(init.signal, signals.size + 1);
      const payload = init?.body;
      const body = JSON.parse(typeof payload === 'string' ? payload : new TextDecoder().decode(payload as Uint8Array));
      // Keep the response releasable even after abort to model a completion queued for the browser.
      return new Promise<Response>((done, reject) => {
        calls.push({
          procedure: kind, body, keyClass: key ? keys.get(key)! : 0,
          signal: init?.signal,
          settle(result) {
            if (result === 'network') reject(new TypeError('The fixture connection was lost.'));
            else done(new Response(result.body, {
              status: result.status, headers: { 'Content-Type': 'application/json' },
            }));
          },
        });
      });
    };
  }, { origins, routes: unaryPaths });
}

const allMachines = (clients: Client[], value = 1n): MachineFrame =>
  ({ change: { case: 'all', value: { clients } }, position: { value } });
const machineSaved = (client: Client, value = 2n): MachineFrame =>
  ({ change: { case: 'saved', value: client }, position: { value } });
const machineRemoved = (id: string, value = 3n): MachineFrame =>
  ({ change: { case: 'removedId', value: id }, position: { value } });
const allRepositories = (repositories: Repository[], value = 1n): RepositoryFrame =>
  ({ change: { case: 'all', value: { repositories } }, position: { value } });
const repositorySaved = (repo: Repository, value = 2n): RepositoryFrame =>
  ({ change: { case: 'saved', value: repo }, position: { value } });
const repositoryRemoved = (id: string, value = 3n): RepositoryFrame =>
  ({ change: { case: 'removedId', value: id }, position: { value } });

const inspectReply = (facts: MessageInitShape<typeof FolderFactsSchema> = {}): UnaryResult => ({
  status: 200,
  body: JSON.stringify(toJson(InspectFolderResponseSchema, create(InspectFolderResponseSchema, {
    facts: { resolvedPath: resolved, mainBranch: 'trunk', name: 'canonical', clean: true, ...facts },
  }))),
});
const addReply = (repo = saved, match = folder): UnaryResult => ({
  status: 200,
  body: JSON.stringify(toJson(AddRepositoryResponseSchema, create(AddRepositoryResponseSchema, {
    repository: repo, folder: match,
  }))),
});
const refusal = (code: string, message: string, status: number): UnaryResult =>
  ({ status, body: JSON.stringify({ code, message }) });

const dialog = (page: Page) => page.getByRole('dialog').filter({ has: page.getByRole('button', { name: 'Add repository', ...exact }) });
// The progress label can temporarily replace the submit label.
const form = (page: Page) => page.getByRole('dialog').filter({ has: page.getByText('Base branch', exact) });
const pathField = (page: Page) => form(page).getByRole('textbox', { name: /^Folder(?: on .+)?$/ });
const chooser = (page: Page) => form(page).getByRole('combobox', { name: 'Machine', ...exact });
const submit = (page: Page) => form(page).locator('button[type="submit"]');
const footer = (page: Page) => form(page).locator('.dialog-foot');
const fact = (page: Page, label: string) => form(page).locator('dt').filter({ hasText: new RegExp('^' + label + '$') }).locator('+ dd');
const calls = (page: Page) => page.evaluate(() => window.addRepositoryProbe.calls());
const reply = (page: Page, index: number, result: UnaryResult) =>
  page.evaluate(({ index, result }) => window.addRepositoryProbe.reply(index, result), { index, result });
const holdReply = (page: Page, kind: Procedure) =>
  page.evaluate((kind) => window.addRepositoryProbe.holdFolderReply(kind), kind);
const replyReady = (page: Page) =>
  expect.poll(() => page.evaluate(() => window.addRepositoryProbe.heldFolder?.ready), prompt).toBe(true);
const replyCanceled = (page: Page) =>
  expect.poll(() => page.evaluate(() => window.addRepositoryProbe.heldFolder?.signal?.aborted), prompt).toBe(true);
async function deliverReply(page: Page) {
  await page.evaluate(() => window.addRepositoryProbe.heldFolder!.release());
  await expect.poll(() => page.evaluate(() => window.addRepositoryProbe.heldFolder?.finished), prompt).toBe(true);
  await flush(page);
}

async function nextCall(page: Page, index: number, kind: Procedure, body: FolderBody) {
  await expect.poll(async () => (await calls(page)).length, {
    ...prompt, message: `The ${kind} request is dispatched through the generated client.`,
  }).toBe(index + 1);
  const request = (await calls(page))[index]!;
  expect(request.procedure).toBe(kind);
  expect(request.body).toEqual(body);
  expect(request.keyClass > 0, 'The request has a nonempty intent key.').toBe(true);
  expect(request.signalClass > 0, 'The request carries a cancellation signal.').toBe(true);
  expect(request.aborted).toBe(false);
  return request;
}

async function flush(page: Page) {
  await page.evaluate(() => new Promise<void>((done) => {
    const channel = new MessageChannel();
    channel.port1.onmessage = () => { channel.port1.close(); channel.port2.close(); done(); };
    channel.port2.postMessage(null);
  }));
  await page.clock.runFor(48);
}

async function waitForRender(page: Page, observed: () => Promise<boolean>, message: string) {
  await expect.poll(async () => {
    await flush(page);
    return observed();
  }, { ...prompt, message }).toBe(true);
}

async function signIn(page: Page) {
  await expect(page.getByLabel('Password', exact)).toBeVisible();
  await page.evaluate(() => { void window.lgtm.act.signIn('browser fixture input'); });
}

async function initialize(page: Page, rig: Rig, clients: Client[], repositories: Repository[] = [], attempt = 0) {
  await expect.poll(() => rig.machines.attempts.length, prompt).toBe(attempt + 1);
  await expect.poll(() => rig.repositories.attempts.length, prompt).toBe(attempt + 1);
  rig.machines.attempts[attempt]!.send(allMachines(clients));
  rig.repositories.attempts[attempt]!.send(allRepositories(repositories));
  await expect(page.getByRole('heading', { name: 'Machines', ...exact })).toBeVisible();
}

async function visit(page: Page, rig: Rig, clients = [first, second], repositories: Repository[] = [], others: string[] = []) {
  const origins = [rig.origin, ...others];
  const trace = await observeRepositoryWatch(page, [rig.web, ...origins]);
  await instrument(page, origins);
  await page.goto(rig.web + '/?data=worst#/machines');
  await expect.poll(() => page.evaluate(() => Boolean(window.repositoryWatchProbe.bridge)), prompt).toBe(true);
  await page.evaluate((origin) => { void window.repositoryWatchProbe.bridge!.connect(origin); }, rig.origin);
  await signIn(page);
  await initialize(page, rig, clients, repositories);
  return trace;
}

async function openAdd(page: Page, options: { machine?: string; repo?: string; back?: boolean } = { machine: first.id }) {
  await page.evaluate(({ machine, repo, back, goal }) => {
    window.lgtm.act.openDialog({
      type: 'add-repository', machine, repo,
      ...(back && { back: { type: 'new-project', machine, repo, goal } }),
    });
  }, { ...options, goal });
  await expect(dialog(page)).toBeVisible();
  await flush(page);
}

async function edit(page: Page, path: string) {
  await pathField(page).fill(path);
  await page.clock.runFor(800);
}

async function inspected(page: Page, path = typed, index = 0, client = first) {
  await edit(page, path);
  await nextCall(page, index, 'inspect', { clientId: client.id, path });
  await reply(page, index, inspectReply());
  await expect(fact(page, 'Base branch')).toHaveText('trunk');
  await expect(submit(page)).toBeEnabled();
}

async function saving(page: Page, index = 1, path = typed, repositoryId?: string) {
  await submit(page).click();
  return nextCall(page, index, 'add', { clientId: first.id, path, ...(repositoryId && { repositoryId }) });
}

const ids = (page: Page) => page.evaluate(() => window.lgtm.store.state.repos.map((repo) => ({
  repo: repo.id,
  folders: (repo.folderRecords ?? []).map(({ id, clientId, resolvedPath }) => ({ id, clientId, resolvedPath })),
})));
const identity = (repo: Repository) => ({
  repo: repo.id,
  folders: repo.folders.map(({ id, clientId, resolvedPath }) => ({ id, clientId, resolvedPath })),
});
const machineState = (page: Page, id: string) => page.evaluate((id) =>
  window.lgtm.store.state.machines.find((machine) => machine.id === id)?.status ?? null, id);

async function success(page: Page, back: boolean) {
  await waitForRender(page, async () => await form(page).count() === 0, 'The matched save closes the folder dialog.');
  if (back) {
    const previous = page.getByRole('dialog', { name: 'New project', ...exact });
    await expect(previous.getByRole('textbox', { name: 'Goal', ...exact })).toHaveValue(goal);
    await expect(previous.getByRole('combobox', { name: 'Repository', ...exact })).toHaveValue(saved.id);
  }
  await expect(page.getByRole('status').getByText(`Added ${saved.name} on ${first.name}`, exact)).toHaveCount(1);
  await expect.poll(() => page.evaluate((id) => window.lgtm.store.state.expanded['repo:' + id], saved.id)).toBe(true);
}

folderTest.describe('inspection follows the latest input', () => {
  for (const late of ['success', 'refusal'] as const) {
    folderTest(`a late ${late} leaves the current machine, path and facts alone`, async ({ page, rig }) => {
      await visit(page, rig);
      await openAdd(page);
      await pathField(page).fill('relative/path');
      await page.clock.runFor(800);
      expect(await calls(page)).toHaveLength(0);
      await expect(submit(page)).toBeDisabled();

      await holdReply(page, 'inspect');
      await pathField(page).fill(typed);
      await page.clock.runFor(650);
      expect(await calls(page), 'The inspection waits for the input debounce.').toHaveLength(0);
      await page.clock.runFor(150);
      const firstCall = await nextCall(page, 0, 'inspect', { clientId: first.id, path: typed });
      await expect(submit(page)).toBeDisabled();
      await reply(page, 0, late === 'success'
        ? inspectReply({ mainBranch: 'obsolete-branch', remote: 'https://example.com/obsolete.git' })
        : refusal('failed_precondition', unreadable, 400));
      await replyReady(page);

      await pathField(page).fill(alias);
      await chooser(page).selectOption(second.id);
      await flush(page);
      await expect(submit(page)).toBeDisabled();
      await replyCanceled(page);
      await page.clock.runFor(800);
      const current = await nextCall(page, 1, 'inspect', { clientId: second.id, path: alias });
      expect(current.keyClass).not.toBe(firstCall.keyClass);
      await reply(page, 1, inspectReply({ mainBranch: 'detached-base', clean: false, remoteOmitted: true }));
      await expect(fact(page, 'Base branch')).toHaveText('detached-base');
      await expect(fact(page, 'Remote')).toHaveText('—');
      await expect(fact(page, 'Status')).toHaveText(dirty);
      await expect(submit(page)).toBeEnabled();

      await deliverReply(page);
      await expect(fact(page, 'Base branch')).toHaveText('detached-base');
      await expect(fact(page, 'Status')).toHaveText(dirty);
      await expect(footer(page).getByText(unreadable, exact)).toHaveCount(0);

      await chooser(page).selectOption(first.id);
      await edit(page, typed);
      const restored = await nextCall(page, 2, 'inspect', { clientId: first.id, path: typed });
      expect(restored.keyClass).not.toBe(firstCall.keyClass);
      await reply(page, 2, inspectReply({ checkedOutBranch: 'topic' }));
      await expect(fact(page, 'Base branch')).toHaveText('trunk');
      await expect(fact(page, 'Remote')).toHaveText('No remote');
      await expect(fact(page, 'Status')).toHaveText(clean);
      expect(await ids(page), 'Inspection leaves the watched inventory alone.').toEqual([]);
    });
  }

  for (const outcome of ['settles', 'stops after the second wait', 'recovers from transport failure'] as const) {
    folderTest(`one automatic inspection retry ${outcome}`, async ({ page, rig }) => {
      await visit(page, rig);
      await openAdd(page);
      await edit(page, typed);
      const body = { clientId: first.id, path: typed };
      const firstCall = await nextCall(page, 0, 'inspect', body);
      const waitRefusal = refusal('deadline_exceeded', wait, 504);
      await reply(page, 0, outcome === 'recovers from transport failure' ? 'network' : waitRefusal);
      await page.clock.runFor(1500);
      const retry = await nextCall(page, 1, 'inspect', body);
      expect(retry.keyClass).toBe(firstCall.keyClass);
      expect(retry.signalClass).not.toBe(firstCall.signalClass);

      if (outcome === 'stops after the second wait') {
        await reply(page, 1, waitRefusal);
        await expect(footer(page).getByText(wait, exact)).toBeVisible();
        await expect(submit(page)).toBeDisabled();
        await page.clock.runFor(3000);
        expect(await calls(page)).toHaveLength(2);
        await edit(page, alias);
        const afterEdit = await nextCall(page, 2, 'inspect', { clientId: first.id, path: alias });
        expect(afterEdit.keyClass).not.toBe(retry.keyClass);
      } else {
        const origin = 'https://example.com/acme/current.git';
        await reply(page, 1, inspectReply({ remote: origin, checkedOutBranch: 'topic' }));
        await expect(fact(page, 'Remote')).toHaveText(origin);
        await expect(fact(page, 'Status')).toHaveText(clean);
        await expect(submit(page)).toBeEnabled();
      }
    });
  }

  folderTest('uses a registered opener, retains a manual offline selection and falls back when it is removed', async ({ page, rig }) => {
    await visit(page, rig, [offline, first, second]);
    await openAdd(page, { machine: offline.id });
    await expect(chooser(page)).toHaveValue(offline.id);
    await expect(submit(page)).toBeDisabled();
    await form(page).getByRole('button', { name: 'Cancel', ...exact }).click();
    await openAdd(page, { machine: 'unregistered' });
    await expect(chooser(page)).toHaveValue(first.id);
    await chooser(page).selectOption(second.id);
    await inspected(page, typed, 0, second);

    rig.machines.attempts[0]!.send(machineSaved({ ...second, state: ClientState.OFFLINE }));
    await expect.poll(() => machineState(page, second.id)).toBe('offline');
    await expect(chooser(page)).toHaveValue(second.id);
    await expect(pathField(page)).toHaveValue(typed);
    await expect(submit(page)).toBeDisabled();

    rig.machines.attempts[0]!.send(machineSaved(unrelated, 3n));
    await expect.poll(() => machineState(page, unrelated.id)).toBe('online');
    await expect(chooser(page)).toHaveValue(second.id);
    rig.machines.attempts[0]!.send(machineRemoved(second.id, 4n));
    await expect.poll(() => machineState(page, second.id)).toBeNull();
    await flush(page);
    await expect(chooser(page)).toHaveValue(first.id);
    await expect(pathField(page)).toHaveValue(typed);
    await expect(fact(page, 'Base branch')).not.toHaveText('trunk');
    await expect(submit(page)).toBeDisabled();
  });

  folderTest('a missing machine leaves a blank choice and an editable path', async ({ page, rig }) => {
    await visit(page, rig, []);
    await openAdd(page, {});
    await expect(chooser(page)).toHaveValue('');
    await expect(chooser(page).locator('option:checked')).toHaveText('');
    await expect(pathField(page)).toHaveAccessibleName('Folder');
    await expect(pathField(page)).not.toHaveAttribute('placeholder', /\S/);
    await expect(pathField(page)).toBeEditable();
    await expect(submit(page)).toBeDisabled();
    await edit(page, typed);
    expect(await calls(page)).toHaveLength(0);

    rig.machines.attempts[0]!.send(machineSaved(first));
    await expect.poll(() => machineState(page, first.id)).toBe('online');
    await flush(page);
    await expect(chooser(page)).toHaveValue(first.id);
    await expect(pathField(page)).toHaveValue(typed);
    await page.clock.runFor(800);
    await nextCall(page, 0, 'inspect', { clientId: first.id, path: typed });
  });
});

folderTest.describe('add joins the watched identity in either order', () => {
  for (const order of ['reply first', 'watch first'] as const) {
    folderTest(`${order}, matching both identifiers and the machine with a second folder on the same machine`, async ({ page, rig }) => {
      await visit(page, rig, [first, second], [partial]);
      const repos = rig.repositories.attempts[0]!;
      if (order === 'reply first') {
        await page.getByRole('button', { name: 'Show details for ' + first.name, ...exact }).click();
        await page.locator('.svc-detail').getByRole('button', { name: 'Add repository', ...exact }).click();
        await flush(page);
      } else {
        await openAdd(page, { machine: first.id, repo: target.id, back: true });
      }
      const path = order === 'reply first' ? typed : alias;
      const repositoryId = order === 'reply first' ? undefined : target.id;
      await inspected(page, path);
      await saving(page, 1, path, repositoryId);
      // A repeat submit event covers keyboard or queued events as well as a disabled button.
      await form(page).dispatchEvent('submit');
      await flush(page);
      expect((await calls(page)).filter((call) => call.procedure === 'add')).toHaveLength(1);

      if (order === 'reply first') {
        await reply(page, 1, addReply());
        await flush(page);
        expect(await ids(page), 'A unary reply cannot write watched records.').toEqual([identity(partial)]);
        await expect(form(page), 'The other folder on the same machine cannot settle the save.').toBeVisible();
        repos.send(repositorySaved(wrongRepository));
        await expect.poll(() => ids(page)).toEqual([identity(partial), identity(wrongRepository)]);
        await flush(page);
        await expect(form(page), 'The matching folder in the wrong repository cannot settle the save.').toBeVisible();
        repos.send(repositoryRemoved(wrongRepository.id));
        repos.send(repositorySaved(wrongMachine, 4n));
        await expect.poll(() => ids(page)).toEqual([identity(wrongMachine)]);
        await flush(page);
        await expect(form(page), 'A watched folder belonging to the wrong machine cannot settle the save.').toBeVisible();
      } else {
        repos.send(repositorySaved(saved));
        await expect.poll(() => ids(page)).toEqual([identity(saved)]);
        await expect(form(page), 'A watched save alone cannot settle a pending unary call.').toBeVisible();
        await reply(page, 1, addReply());
      }
      if (order === 'reply first') repos.send(repositorySaved(saved, 5n));
      await success(page, order === 'watch first');
      await expect.poll(() => ids(page)).toEqual([identity(saved)]);
      expect((await calls(page)).filter((call) => call.procedure === 'add')).toHaveLength(1);
      expect((await calls(page))[1]?.body.path).not.toBe(resolved);

      if (order === 'reply first') {
        await page.clock.runFor(3700);
        await openAdd(page);
        await inspected(page, alias, 2);
        await saving(page, 3, alias);
        await reply(page, 3, addReply());
        await success(page, false);
        await expect.poll(() => ids(page)).toEqual([identity(saved)]);

        await page.reload();
        await initialize(page, rig, [first, second], [saved], 1);
        const menu = page.getByRole('button', { name: 'Open navigation', ...exact });
        if (await menu.isVisible()) await menu.click();
        const tree = page.getByRole('complementary', { name: 'Navigation', ...exact });
        const entry = tree.getByRole('link', { name: saved.name, ...exact });
        await expect(entry).toHaveCount(1);
        await expect(entry).toBeVisible();
        await expect(entry).toHaveAttribute('href', '#/r/' + saved.id);
        expect(await ids(page)).toEqual([identity(saved)]);
      }
    });
  }

  folderTest('a reply folder for a different machine cannot settle the submitted save', async ({ page, rig }) => {
    await visit(page, rig, [first, second], [wrongMachine]);
    await openAdd(page);
    await inspected(page);
    await saving(page);
    const mismatchedFolder = wrongMachine.folders.find(({ id }) => id === folder.id)!;
    await reply(page, 1, addReply(wrongMachine, mismatchedFolder));
    await flush(page);
    await expect(form(page)).toBeVisible();
    await expect(page.getByRole('status').getByText(/^Added /)).toHaveCount(0);
    expect(await ids(page)).toEqual([identity(wrongMachine)]);
    expect(await calls(page)).toHaveLength(2);
  });

  for (const clock of ['wall', 'monotonic'] as const) {
    folderTest(`a lost add replays offline and retires from first send using the ${clock} clock`, async ({ page, rig }) => {
      await visit(page, rig);
      await openAdd(page);
      await inspected(page);
      const original = await saving(page);
      const started = await page.evaluate(() => ({ wall: Date.now(), monotonic: performance.now() }));
      await reply(page, 1, 'network');
      await expect(footer(page).getByText(network, exact)).toBeVisible();
      rig.machines.attempts[0]!.send(machineSaved({ ...first, state: ClientState.OFFLINE }));
      await expect.poll(() => machineState(page, first.id)).toBe('offline');

      const replayWindow = 23 * 60 * 60 * 1000;
      const age = async (elapsed: number) => {
        if (clock === 'wall') await page.clock.setSystemTime(started.wall + elapsed);
        else {
          const now = await page.evaluate(() => performance.now());
          await page.clock.fastForward(Math.max(0, started.monotonic + elapsed - now));
          await page.clock.setSystemTime(started.wall + 6000);
        }
      };
      await age(replayWindow - 60_000);
      const replay = await saving(page, 2);
      expect(replay.keyClass, 'An offline replay uses the key of the lost call.').toBe(original.keyClass);
      expect(replay.signalClass).not.toBe(original.signalClass);
      await reply(page, 2, 'network');
      await expect(footer(page).getByText(network, exact)).toBeVisible();

      rig.machines.attempts[0]!.send(machineSaved(first, 3n));
      await expect.poll(() => machineState(page, first.id)).toBe('online');
      await age(replayWindow + 60_000);
      await expect(pathField(page)).toHaveValue(typed);
      await expect(submit(page)).toBeEnabled();
      await submit(page).click();
      await page.clock.runFor(800);
      const fresh = await nextCall(page, 3, 'inspect', { clientId: first.id, path: typed });
      expect(fresh.keyClass).not.toBe(original.keyClass);
      await expect(submit(page)).toBeDisabled();
      await reply(page, 3, inspectReply({ resolvedPath: '/checkouts/replaced', mainBranch: 'elsewhere' }));
      await expect(fact(page, 'Base branch')).toHaveText('elsewhere');
      await expect(submit(page)).toBeEnabled();
      expect(await calls(page), 'Retirement and fresh facts issue no automatic add.').toHaveLength(4);
      const next = await saving(page, 4);
      expect(next.keyClass, 'A subsequent explicit Add starts a fresh intent.').not.toBe(original.keyClass);
    });
  }

  folderTest('a lost watch ignores retained rows, tries one refresh and rechecks a known save without sending it again', async ({ page, rig }) => {
    await visit(page, rig);
    await openAdd(page, { machine: first.id, back: true });
    await inspected(page);
    await saving(page);
    const firstWatch = rig.repositories.attempts[0]!;
    firstWatch.send(repositorySaved(saved));
    await expect.poll(() => ids(page)).toEqual([identity(saved)]);
    firstWatch.send(Code.Aborted);
    await expect.poll(() => page.evaluate(() => window.repositoryWatchProbe.bridge!.currentRepositorySnapshot() === undefined)).toBe(true);
    await reply(page, 1, addReply());
    await flush(page);
    await expect(form(page), 'Rows retained after watch loss cannot complete a save.').toBeVisible();
    await expect(page.getByRole('status').getByText(/^Added /)).toHaveCount(0);

    await page.clock.runFor(600);
    await expect.poll(() => rig.repositories.attempts.length, prompt).toBe(2);
    const failedReplacement = rig.repositories.attempts[1]!;
    const backoffs = await page.evaluate(() => window.discoveryProbe.timeouts[1000] ?? 0);
    failedReplacement.send(Code.Unavailable);
    await expect.poll(() => page.evaluate(() => window.discoveryProbe.timeouts[1000] ?? 0)).toBeGreaterThan(backoffs);
    await page.clock.runFor(1000);
    await expect.poll(() => rig.repositories.attempts.length, prompt).toBe(3);
    const beforeRefresh = rig.repositories.attempts[2]!;
    expect(beforeRefresh.aborted).toBe(false);
    await page.clock.runFor(500);
    await expect.poll(() => beforeRefresh.aborted, prompt).toBe(true);
    await expect.poll(() => rig.repositories.attempts.length, prompt).toBe(4);
    expect(rig.machines.attempts).toHaveLength(1);
    const silent = rig.repositories.attempts[3]!;
    await page.clock.runFor(3500);
    await expect(submit(page), 'The join is still waiting before the six-second deadline.').toBeDisabled();
    await page.clock.runFor(500);
    await expect(footer(page).getByText(network, exact)).toBeVisible();
    await expect(submit(page)).toBeEnabled();
    await expect(pathField(page)).toHaveValue(typed);
    expect(rig.repositories.attempts).toHaveLength(4);
    expect(rig.machines.attempts).toHaveLength(1);
    expect(silent.aborted).toBe(false);

    await submit(page).click();
    await flush(page);
    expect((await calls(page)).filter((call) => call.procedure === 'add'), 'A known save is rechecked through the watch only.').toHaveLength(1);
    await expect(submit(page)).toBeDisabled();
    await page.clock.runFor(6100);
    await expect(footer(page).getByText(network, exact)).toBeVisible();
    await expect(submit(page)).toBeEnabled();
    await submit(page).click();
    await flush(page);
    await expect.poll(() => rig.repositories.attempts.at(-1)?.aborted).toBe(false);
    rig.repositories.attempts.at(-1)!.send(allRepositories([saved], 7n));
    await success(page, true);
    await expect.poll(() => ids(page)).toEqual([identity(saved)]);
    expect((await calls(page)).filter((call) => call.procedure === 'add')).toHaveLength(1);
  });

  for (const replacement of ['repository absent', 'folder absent'] as const) {
    folderTest(`a later full snapshot proves ${replacement}`, async ({ page, rig }) => {
      await visit(page, rig, [first, second], [partial]);
      await openAdd(page);
      await inspected(page);
      await saving(page);
      await reply(page, 1, addReply());
      await flush(page);
      await expect(form(page)).toBeVisible();
      expect(await ids(page)).toEqual([identity(partial)]);
      expect(rig.repositories.attempts).toHaveLength(1);
      await page.clock.runFor(2100);
      await expect.poll(() => rig.repositories.attempts.length, prompt).toBe(2);
      expect(rig.machines.attempts).toHaveLength(1);
      rig.repositories.attempts[1]!.send(allRepositories(replacement === 'repository absent' ? [] : [partial], 5n));
      const reason = footer(page).getByText(replacement === 'repository absent' ? missing : generic, exact);
      await waitForRender(page, () => reason.isVisible(), 'The later snapshot settles the join with the fixed refusal.');
      await expect(pathField(page)).toHaveValue(typed);
    });
  }
});

folderTest.describe('add refusal and expiry preserve or clear the form', () => {
  for (const order of ['watch first', 'refusal first'] as const) {
    folderTest(`removed machine settles once with ${order}`, async ({ page, rig }) => {
      await visit(page, rig);
      await openAdd(page);
      await inspected(page);
      await saving(page);
      const refuseRemoval = () => reply(page, 1, refusal('not_found', removed, 404));
      const removeMachine = () => rig.machines.attempts[0]!.send(machineRemoved(first.id));
      const requireRefusal = () => expect(footer(page).getByText(removed, exact)).toHaveCount(1);
      const requireFallback = () => waitForRender(page, () => chooser(page).evaluate(
        (node, id) => (node as HTMLSelectElement).value === id, second.id,
      ), 'Watched removal selects the remaining machine.');

      if (order === 'watch first') {
        removeMachine();
        await requireFallback();
        await requireRefusal();
        await refuseRemoval();
      } else {
        await refuseRemoval();
        await requireRefusal();
        removeMachine();
        await requireFallback();
      }

      await page.clock.runFor(800);
      const fallback = (await calls(page)).findIndex((call) => call.procedure === 'inspect' && call.body.clientId === second.id);
      if (fallback >= 0) {
        await reply(page, fallback, inspectReply({ mainBranch: 'fallback' }));
        await flush(page);
      }
      await expect(pathField(page)).toHaveValue(typed);
      await requireRefusal();
      await expect(fact(page, 'Base branch')).not.toHaveText('trunk');
      expect((await calls(page)).filter((call) => call.procedure === 'add')).toHaveLength(1);
      await edit(page, alias);
      await expect(footer(page).getByText(removed, exact)).toHaveCount(0);
    });
  }

  for (const order of ['machine first', 'repository first'] as const) {
    folderTest(`a late successful reply cannot restore its removed machine with ${order}`, async ({ page, rig }) => {
      await visit(page, rig);
      await openAdd(page);
      await inspected(page);
      await saving(page);
      const machines = rig.machines.attempts[0]!;
      const repos = rig.repositories.attempts[0]!;
      repos.send(repositorySaved(saved));
      await expect.poll(() => ids(page)).toEqual([identity(saved)]);
      const removeMachine = () => machines.send(machineRemoved(first.id));
      const removeRepo = () => repos.send(repositoryRemoved(saved.id));
      if (order === 'repository first') {
        removeRepo();
        await expect.poll(() => ids(page)).toEqual([]);
      }
      repos.send(Code.Aborted);
      await expect.poll(() => page.evaluate(() => window.repositoryWatchProbe.bridge!.currentRepositorySnapshot() === undefined)).toBe(true);
      removeMachine();
      await expect.poll(() => machineState(page, first.id)).toBeNull();
      await flush(page);
      await expect(chooser(page)).toHaveValue(second.id);
      await reply(page, 1, addReply());
      await flush(page);
      await expect(footer(page).getByText(removed, exact)).toHaveCount(1);
      await expect(pathField(page)).toHaveValue(typed);
      await expect(chooser(page)).toHaveValue(second.id);
      await expect(page.getByRole('status').getByText(/^Added /)).toHaveCount(0);
      await page.clock.runFor(600);
      await expect.poll(() => rig.repositories.attempts.length, prompt).toBe(2);
      rig.repositories.attempts[1]!.send(allRepositories([], 5n));
      await expect.poll(() => ids(page)).toEqual([]);
      expect((await calls(page)).filter((call) => call.procedure === 'add')).toHaveLength(1);
    });
  }

  folderTest('an explicit target survives input capture and a watched repository removal', async ({ page, rig }) => {
    await visit(page, rig, [first, second], [target]);
    await openAdd(page, { machine: first.id, repo: target.id });
    await inspected(page);
    await saving(page, 1, typed, target.id);
    await reply(page, 1, refusal('failed_precondition', mismatch, 400));
    await expect(footer(page).getByText(mismatch, exact)).toBeVisible();
    await expect(pathField(page)).toHaveValue(typed);
    expect(await ids(page)).toEqual([identity(target)]);

    await inspected(page, alias, 2);
    rig.repositories.attempts[0]!.send(repositoryRemoved(target.id));
    await expect.poll(() => ids(page)).toEqual([]);
    await saving(page, 3, alias, target.id);
    await reply(page, 3, refusal('not_found', missing, 404));
    await expect(footer(page).getByText(missing, exact)).toBeVisible();
    await expect(pathField(page)).toHaveValue(alias);
    expect(await ids(page)).toEqual([]);
  });

  const refusals = [
    ['inspect', 'unavailable', 'The machine is not connected.', 503],
    ['inspect', 'invalid_argument', 'The machine, repository, or folder path is invalid.', 400],
    ['inspect', 'invalid_argument', 'The request key is invalid.', 400],
    ['inspect', 'invalid_argument', 'The request body is invalid.', 400],
    ['inspect', 'already_exists', 'That request key was already used for a different request.', 409],
    ['inspect', 'not_found', 'The machine was not found.', 404],
    ['inspect', 'failed_precondition', 'The path is not a folder.', 400],
    ['inspect', 'failed_precondition', 'The folder is not a Git repository.', 400],
    ['inspect', 'failed_precondition', unreadable, 400],
    ['inspect', 'failed_precondition', 'The folder path cannot be used.', 400],
    ['inspect', 'deadline_exceeded', 'The machine did not take the folder command.', 504],
    ['inspect', 'deadline_exceeded', 'The outcome of the folder request is unknown.', 504],
    ['add', 'already_exists', 'The folder is already saved for another repository.', 409],
  ] as const;

  for (const [procedure, code, message, status] of refusals) {
    folderTest(`the footer shows “${message}”`, async ({ page, rig }) => {
      await visit(page, rig, [first, second], [target]);
      await openAdd(page, { machine: first.id, ...(procedure === 'add' && { repo: target.id }) });
      if (procedure === 'add') {
        await inspected(page);
        await saving(page, 1, typed, target.id);
      } else {
        await edit(page, typed);
        await nextCall(page, 0, 'inspect', { clientId: first.id, path: typed });
      }
      await reply(page, procedure === 'add' ? 1 : 0, refusal(code, message, status));
      await expect(footer(page).getByText(message, exact)).toHaveCount(1);
      await expect(pathField(page)).toHaveValue(typed);
      if (procedure === 'inspect') await expect(submit(page)).toBeDisabled();
      await page.clock.runFor(2500);
      expect(await calls(page)).toHaveLength(procedure === 'add' ? 2 : 1);
      expect(await ids(page)).toEqual([identity(target)]);
    });
  }

  for (const kind of ['inspect', 'add'] as const) {
    folderTest(`current ${kind} expiry clears the form, aborts the session and starts clean after sign-in`, async ({ page, rig }) => {
      const trace = await visit(page, rig, [first, second], [target]);
      await openAdd(page);
      const index = kind === 'add' ? 1 : 0;
      if (kind === 'add') {
        await inspected(page);
        await saving(page);
      } else {
        await edit(page, typed);
        await nextCall(page, index, 'inspect', { clientId: first.id, path: typed });
      }
      await reply(page, index, refusal('unauthenticated', signedOut, 401));
      await expect(page.getByLabel('Password', exact)).toBeVisible();
      await expect(form(page)).toHaveCount(0);
      await expect.poll(() => ids(page)).toEqual([]);
      await expect.poll(() => machineState(page, first.id)).toBeNull();
      await expect.poll(() => rig.machines.attempts[0]?.aborted && rig.repositories.attempts[0]?.aborted).toBe(true);
      await signIn(page);
      await initialize(page, rig, [first, second], [], 1);
      await openAdd(page);
      await expect(pathField(page)).not.toHaveValue(typed);
      await expect(fact(page, 'Base branch')).not.toHaveText('trunk');
      await expect(submit(page)).toBeDisabled();
      expect(trace.errors).toEqual([]);
    });

    folderTest(`watch expiry aborts the pending ${kind} and discards its queued success`, async ({ page, rig }) => {
      const trace = await visit(page, rig);
      await openAdd(page);
      const index = kind === 'add' ? 1 : 0;
      if (kind === 'add') {
        await inspected(page);
        await saving(page);
      } else {
        await edit(page, typed);
        await nextCall(page, 0, 'inspect', { clientId: first.id, path: typed });
      }
      rig.repositories.attempts[0]!.send(Code.Unauthenticated);
      await expect(page.getByLabel('Password', exact)).toBeVisible();
      await expect.poll(async () => (await calls(page))[index]?.aborted).toBe(true);
      await expect.poll(() => rig.machines.attempts[0]?.aborted).toBe(true);
      await expect.poll(() => page.evaluate(({ machines, repositories }) => window.repositoryWatchProbe.requests.filter(
        ({ path, signal }) => (path === machines || path === repositories) && signal?.aborted,
      ).length, procedure)).toBe(2);
      await signIn(page);
      await initialize(page, rig, [second], [], 1);
      await openAdd(page, { machine: second.id });
      await reply(page, index, kind === 'add' ? addReply() : inspectReply({ mainBranch: 'obsolete-branch' }));
      await flush(page);
      await expect(chooser(page)).toHaveValue(second.id);
      await expect(pathField(page)).not.toHaveValue(typed);
      await expect(fact(page, 'Base branch')).not.toHaveText('obsolete-branch');
      await expect(submit(page)).toBeDisabled();
      await expect.poll(() => ids(page)).toEqual([]);
      expect(trace.errors).toEqual([]);
    });
  }

  folderTest('a replacement Add repository dialog does not accept the old save or its authentication refusal', async ({ page, rig }) => {
    await visit(page, rig);
    await openAdd(page);
    await inspected(page);
    await holdReply(page, 'add');
    await saving(page);
    await reply(page, 1, refusal('unauthenticated', signedOut, 401));
    await replyReady(page);
    await openAdd(page, { machine: second.id });
    await expect(chooser(page)).toHaveValue(second.id);
    await replyCanceled(page);
    await inspected(page, alias, 2, second);
    await deliverReply(page);
    await expect(pathField(page)).toHaveValue(alias);
    await expect(fact(page, 'Base branch')).toHaveText('trunk');
    await expect(submit(page)).toBeEnabled();
    await expect(page.getByLabel('Password', exact)).toHaveCount(0);
    expect(await ids(page)).toEqual([]);
  });

  folderTest('long inspected values wrap with visible focus and reachable actions', async ({ page, rig }) => {
    await visit(page, rig);
    await openAdd(page, { machine: second.id });
    await expect(form(page).locator(':focus')).toHaveCount(1);
    const longBranch = 'feature/' + 'branch'.repeat(20);
    const longRemote = 'https://example.com/' + 'checkout'.repeat(33) + '.git';
    await edit(page, typed);
    await nextCall(page, 0, 'inspect', { clientId: second.id, path: typed });
    await reply(page, 0, inspectReply({ mainBranch: longBranch, remote: longRemote }));
    const branch = fact(page, 'Base branch');
    const remote = fact(page, 'Remote');
    await expect(branch).toHaveText(longBranch);
    await expect(remote).toHaveText(longRemote);
    await expect(pathField(page)).toBeFocused();
    for (const value of [branch, remote]) {
      expect(await value.evaluate((node) => {
        const range = document.createRange();
        range.selectNodeContents(node);
        return range.getClientRects().length > 1 && node.scrollWidth <= node.clientWidth + 1;
      }), 'The full fact wraps inside the available width.').toBe(true);
    }
    expect(await form(page).evaluate((node) => {
      const body = node.querySelector('.dialog-body')!;
      return [node, body, document.documentElement].every((element) => element.scrollWidth <= element.clientWidth + 1);
    }), 'The dialog, its body, and the document fit the viewport.').toBe(true);
    const cancel = form(page).getByRole('button', { name: 'Cancel', ...exact });
    await page.keyboard.press('Tab');
    await expect(cancel).toBeFocused();
    await page.keyboard.press('Tab');
    await expect(submit(page)).toBeFocused();
    await expect(cancel).toBeInViewport({ ratio: 1 });
    await expect(submit(page)).toBeInViewport({ ratio: 1 });
  });
});

folderTest('issue 1692: a decoded machine frame cannot reach the next signed-in session', async ({ page, rig }) => {
  const machines = controlledWatch<MachineFrame>();
  const repositories = controlledWatch<RepositoryFrame>();
  const next = await serveRepositorySession({
    pageOrigin: rig.web, hostname: otherHost, listener: 'next', registry: sessionRegistry(),
    controlled: { services: [ProjectService.typeName], machines: machines.read, repositories: repositories.read },
  });
  try {
    const trace = await visit(page, rig, [first], [target], [next.origin]);
    await page.evaluate(() => { window.addRepositoryProbe.holdMachineFrame(); });
    rig.machines.attempts[0]!.send(machineSaved(queuedMachine));
    await expect.poll(() => page.evaluate(() => window.addRepositoryProbe.heldMachine?.ready), prompt).toBe(true);
    await page.evaluate((origin) => { void window.repositoryWatchProbe.bridge!.connect(origin); }, next.origin);
    await signIn(page);
    await initialize(page, { web: rig.web, origin: next.origin, machines, repositories }, [newMachine], [saved]);
    await page.evaluate((name) => {
      window.lgtm.act.openDialog({ type: 'rename', title: 'Replacement dialog', value: name, onRename() {} });
    }, newMachine.name);
    const replacement = page.getByRole('dialog', { name: 'Replacement dialog', ...exact });
    await expect(replacement).toBeVisible();
    await expect.poll(() => rig.machines.attempts[0]?.aborted && rig.repositories.attempts[0]?.aborted).toBe(true);
    await page.evaluate(() => window.addRepositoryProbe.heldMachine!.release());
    await expect.poll(() => page.evaluate(() => window.addRepositoryProbe.heldMachine?.finished), prompt).toBe(true);

    await expect(replacement).toBeVisible();
    expect(await page.evaluate(() => ({
      address: window.lgtm.store.state.server.address,
      session: window.lgtm.store.state.session.status,
      synced: window.lgtm.store.state.server.synced,
      ids: window.lgtm.store.state.machines.map(({ id }) => id),
      hash: location.hash,
    }))).toEqual({ address: next.origin, session: 'in', synced: true, ids: [newMachine.id], hash: '#/machines' });
    expect(await ids(page)).toEqual([identity(saved)]);
    expect(trace.errors).toEqual([]);
  } finally {
    await next.stop();
  }
});

for (const [procedure, invoke] of [['inspect', inspectFolder], ['add', addRepository]] as const) {
  test(`issue 1692: an already obsolete ${procedure} intent never dispatches`, async () => {
    let requests = 0;
    const transport = createRouterTransport((router) => router.service(ProjectService, {
      inspectFolder: () => { requests++; return {}; },
      addRepository: () => { requests++; return {}; },
    }));
    const { signal } = new AbortController();
    const result = await invoke(folderIntent({ procedure, clientId: first.id, path: typed }), {
      signal, client: createClient(ProjectService, transport), isCurrent: () => false,
    });
    expect(result === undefined, 'The obsolete wrapper resolves without a reply.').toBe(true);
    expect(signal.aborted).toBe(false);
    expect(requests, 'An obsolete intent produces zero requests even while its signal is live.').toBe(0);
  });
}
