import { create, toJson, type MessageInitShape } from '@bufbuild/protobuf';
import { ClientSchema, ClientState, type WatchClientsResponseSchema } from '@lgtm/api/clients';
import {
  AddRepositoryResponseSchema, InspectFolderResponseSchema, ProjectService, RepositoryFolderSchema,
  RepositorySchema, type WatchRepositoriesResponseSchema,
} from '@lgtm/api/projects';
import { expect, type Page } from '@playwright/test';
import {
  controlledWatch, observeRepositoryWatch, serveRepositorySession, serveWebsite, sessionRegistry, test,
} from './support/repository-server';

const first = create(ClientSchema, { id: 'first', name: 'First worker', state: ClientState.ONLINE });
const second = create(ClientSchema, { id: 'second', name: 'Second worker', state: ClientState.ONLINE });
const originalPath = '~/Original checkout ';
const editedPath = '/checkouts/another';
const savedFolder = create(RepositoryFolderSchema, {
  id: '018f2ad2-bcde-7456-8123-000000000012', clientId: first.id, resolvedPath: '/checkouts/original',
});
const savedRepository = create(RepositorySchema, {
  id: '018f2ad2-bcde-7456-8123-000000000010', name: 'Original checkout', folders: [savedFolder],
});
const savedReply = toJson(AddRepositoryResponseSchema, create(AddRepositoryResponseSchema, {
  repository: savedRepository, folder: savedFolder,
}));
const inspectionReply = (mainBranch: string, clean = true) =>
  toJson(InspectFolderResponseSchema, create(InspectFolderResponseSchema, {
    facts: { resolvedPath: savedFolder.resolvedPath, mainBranch, clean },
  }));
const originalFacts = inspectionReply('original-branch');
const currentFacts = inspectionReply('current-branch', false);
const noAnswer = 'The server did not answer.';
const deadlineMs = 30_000;
const boundaryMarginMs = 50;

type Procedure = 'inspect' | 'add';
type InputEdit = { control: 'path' | 'machine'; value: string };
type FolderCall = {
  procedure: Procedure;
  body: { clientId: string; path: string };
  keyClass: number;
  startedAt: number;
};

declare global {
  interface Window {
    folderReviewProbe: {
      calls(): FolderCall[];
      reply(index: number, status: number, body: string, edit?: InputEdit): Promise<void>;
    };
  }
}

type MachineFrame = MessageInitShape<typeof WatchClientsResponseSchema>;
type RepositoryFrame = MessageInitShape<typeof WatchRepositoriesResponseSchema>;
const reviewTest = test.extend<{ rig: {
  websiteOrigin: string;
  serverOrigin: string;
  machines: ReturnType<typeof controlledWatch<MachineFrame>>;
  repositories: ReturnType<typeof controlledWatch<RepositoryFrame>>;
} }>({
  rig: async ({ website }, use) => {
    const web = await serveWebsite(website);
    const machines = controlledWatch<MachineFrame>();
    const repositories = controlledWatch<RepositoryFrame>();
    try {
      const server = await serveRepositorySession({
        pageOrigin: web.origin, hostname: new URL(web.origin).hostname,
        listener: 'folder-review', registry: sessionRegistry(),
        controlled: { services: [ProjectService.typeName], machines: machines.read, repositories: repositories.read },
      });
      try {
        await use({ websiteOrigin: web.origin, serverOrigin: server.origin, machines, repositories });
      } finally {
        await server.stop();
      }
    } finally {
      await web.stop();
    }
  },
});

function interceptFolderCalls(page: Page, serverOrigin: string) {
  const pathFor = (name: string) => `/${ProjectService.typeName}/${name}`;
  return page.addInitScript(({ serverOrigin, inspectPath, addPath }) => {
    const fetch = window.fetch.bind(window);
    const requests: { call: FolderCall; deliver(response: Response): void }[] = [];
    const keys = new Map<string, number>();

    function editAfterEligibilityCheck(edit: InputEdit, applied: () => void) {
      const state = window.lgtm.store.state;
      const machines = Object.getOwnPropertyDescriptor(state, 'machines')!;
      // The wrapper's final eligibility check reads the registered machines. Queue the edit
      // from that read so the check still succeeds and the dialog's await resumes after the edit.
      Object.defineProperty(state, 'machines', {
        configurable: true,
        enumerable: machines.enumerable,
        get() {
          Object.defineProperty(state, 'machines', machines);
          queueMicrotask(() => {
            const selector = edit.control === 'path' ? '#ar-path' : '#ar-machine';
            const field = document.querySelector<HTMLInputElement | HTMLSelectElement>(selector)!;
            field.value = edit.value;
            field.dispatchEvent(new Event(edit.control === 'path' ? 'input' : 'change', { bubbles: true }));
            applied();
          });
          return machines.value;
        },
      });
    }

    window.folderReviewProbe = {
      calls: () => requests.map(({ call }) => call),
      async reply(index, status, body, edit) {
        const response = new Response(body, { status, headers: { 'Content-Type': 'application/json' } });
        const request = requests[index]!;
        if (!edit) {
          request.deliver(response);
          return;
        }
        await new Promise<void>((applied) => {
          response.json = async () => {
            const decoded: unknown = JSON.parse(body);
            editAfterEligibilityCheck(edit, applied);
            return decoded;
          };
          request.deliver(response);
        });
      },
    };

    window.fetch = (input, init) => {
      const url = new URL(input instanceof Request ? input.url : String(input), location.href);
      const procedure = url.pathname === inspectPath ? 'inspect' : url.pathname === addPath ? 'add' : undefined;
      if (url.origin !== serverOrigin || !procedure) return fetch(input, init);
      const key = new Headers(init?.headers).get('Lgtm-Request-Key');
      if (key && !keys.has(key)) keys.set(key, keys.size + 1);
      const payload = init?.body;
      const body = JSON.parse(typeof payload === 'string' ? payload : new TextDecoder().decode(payload as Uint8Array));
      const call: FolderCall = { procedure, body, keyClass: key ? keys.get(key)! : 0, startedAt: performance.now() };
      return new Promise<Response>((resolve, reject) => {
        const signal = init?.signal;
        const abort = () => reject(signal?.reason);
        signal?.addEventListener('abort', abort, { once: true });
        requests.push({ call, deliver(response) {
          signal?.removeEventListener('abort', abort);
          resolve(response);
        } });
        if (signal?.aborted) abort();
      });
    };
  }, { serverOrigin, inspectPath: pathFor(ProjectService.method.inspectFolder.name),
    addPath: pathFor(ProjectService.method.addRepository.name) });
}

const dialog = (page: Page) => page.getByRole('dialog', { name: 'Add repository', exact: true });
const folder = (page: Page) => dialog(page).getByRole('textbox', { name: /^Folder on / });
const machine = (page: Page) => dialog(page).getByRole('combobox', { name: 'Machine', exact: true });
const submit = (page: Page) => dialog(page).locator('button[type="submit"]');
const feedback = (page: Page) => dialog(page).locator('.dialog-foot .why');
const fact = (page: Page, label: string) => dialog(page).locator('dt').filter({ hasText: new RegExp(`^${label}$`) }).locator('+ dd');
const calls = (page: Page) => page.evaluate(() => window.folderReviewProbe.calls());
const reply = (page: Page, index: number, body: unknown, status = 200, edit?: InputEdit) =>
  page.evaluate(({ index, status, body, edit }) => window.folderReviewProbe.reply(index, status, body, edit),
    { index, status, body: JSON.stringify(body), edit });

async function callAt(page: Page, index: number, procedure: Procedure, clientId = first.id, path = originalPath) {
  await expect.poll(async () => (await calls(page)).length).toBe(index + 1);
  const call = (await calls(page))[index]!;
  expect(call.procedure).toBe(procedure);
  expect(call.body).toEqual({ clientId, path });
  expect(call.keyClass).toBeGreaterThan(0);
  return call;
}

async function firstInspection(page: Page) {
  await folder(page).fill(originalPath);
  await page.clock.runFor(700);
  return callAt(page, 0, 'inspect');
}

async function startSave(page: Page) {
  await firstInspection(page);
  await reply(page, 0, originalFacts);
  await expect(fact(page, 'Base branch')).toHaveText('original-branch');
  await submit(page).click();
  return callAt(page, 1, 'add');
}

async function advanceToBeforeDeadline(page: Page, attempt: FolderCall) {
  const elapsed = await page.evaluate((startedAt) => performance.now() - startedAt, attempt.startedAt);
  const beforeDeadline = deadlineMs - boundaryMarginMs;
  expect(elapsed).toBeLessThan(beforeDeadline);
  await page.clock.runFor(beforeDeadline - elapsed);
}

reviewTest.beforeEach(async ({ page, rig }) => {
  await observeRepositoryWatch(page, [rig.websiteOrigin, rig.serverOrigin]);
  await interceptFolderCalls(page, rig.serverOrigin);
  await page.goto(rig.websiteOrigin + '/#/machines');
  await expect.poll(() => page.evaluate(() => !!window.repositoryWatchProbe.bridge)).toBe(true);
  await page.evaluate((origin) => { void window.repositoryWatchProbe.bridge!.connect(origin); }, rig.serverOrigin);
  await expect(page.getByLabel('Password', { exact: true })).toBeVisible();
  await page.evaluate(() => { void window.lgtm.act.signIn('browser fixture input'); });
  await expect.poll(() => rig.machines.attempts.length).toBe(1);
  await expect.poll(() => rig.repositories.attempts.length).toBe(1);
  rig.machines.attempts[0]!.send({ change: { case: 'all', value: { clients: [first, second] } }, position: { value: 1n } });
  rig.repositories.attempts[0]!.send({ change: { case: 'all', value: { repositories: [] } }, position: { value: 1n } });
  await expect(page.getByRole('heading', { name: 'Machines', exact: true })).toBeVisible();
  await page.evaluate((id) => window.lgtm.act.openDialog({ type: 'add-repository', machine: id }), first.id);
  await expect(dialog(page)).toBeVisible();
});

for (const edit of [{ control: 'path', value: editedPath }, { control: 'machine', value: second.id }] as const) {
  const clientId = edit.control === 'machine' ? second.id : first.id;
  const path = edit.control === 'path' ? editedPath : originalPath;

  reviewTest(`an inspection reply past its wrapper check cannot restore old facts after a ${edit.control} edit`, async ({ page }) => {
    const original = await firstInspection(page);
    await reply(page, 0, originalFacts, 200, edit);
    await expect(folder(page)).toHaveValue(path);
    await expect(machine(page)).toHaveValue(clientId);
    await expect(fact(page, 'Base branch'), 'The edited input does not show the obsolete branch.').not.toHaveText('original-branch');
    await expect(submit(page)).toBeDisabled();

    await page.clock.runFor(700);
    const current = await callAt(page, 1, 'inspect', clientId, path);
    expect(current.keyClass).not.toBe(original.keyClass);
    await reply(page, 1, currentFacts);
    await expect(fact(page, 'Base branch')).toHaveText('current-branch');
    await expect(fact(page, 'Status')).toHaveText('This Git repository has uncommitted changes.');
    await expect(submit(page)).toBeEnabled();
  });

  reviewTest(`a save reply past its wrapper check cannot close the dialog after a ${edit.control} edit`, async ({ page, rig }) => {
    await startSave(page);
    rig.repositories.attempts[0]!.send({ change: { case: 'saved', value: savedRepository }, position: { value: 2n } });
    await expect.poll(() => page.evaluate((id) =>
      window.repositoryWatchProbe.bridge!.currentRepositorySnapshot()?.repos[0]?.folderRecords?.[0]?.id === id,
    savedFolder.id)).toBe(true);
    await reply(page, 1, savedReply, 200, edit);
    await expect(dialog(page), 'The watched identity from the obsolete save does not close the edited dialog.').toBeVisible();
    await expect(folder(page)).toHaveValue(path);
    await expect(machine(page)).toHaveValue(clientId);
    await expect(submit(page)).toBeDisabled();

    await page.clock.runFor(700);
    await callAt(page, 2, 'inspect', clientId, path);
    await reply(page, 2, currentFacts);
    await expect(fact(page, 'Base branch')).toHaveText('current-branch');
    await expect(submit(page)).toBeEnabled();
    await expect(page.getByRole('status').getByText(/^Added /)).toHaveCount(0);
  });
}

reviewTest('a silent inspection retries once and both attempts settle at the 30-second deadline', async ({ page }) => {
  const original = await firstInspection(page);
  await advanceToBeforeDeadline(page, original);
  await expect(feedback(page)).toHaveText('Looking at the folder…');
  await expect(submit(page)).toBeDisabled();
  expect(await calls(page)).toHaveLength(1);

  await page.clock.runFor(2 * boundaryMarginMs);
  await expect.poll(async () => (await calls(page)).length, {
    message: 'A silent inspection dispatches its automatic retry as browser time crosses 30 seconds.',
  }).toBe(2);
  const retry = await callAt(page, 1, 'inspect');
  expect(retry.keyClass).toBe(original.keyClass);

  await advanceToBeforeDeadline(page, retry);
  await expect(feedback(page)).toHaveText('Looking at the folder…');
  await expect(submit(page)).toBeDisabled();
  expect(await calls(page)).toHaveLength(2);

  await page.clock.runFor(2 * boundaryMarginMs);
  await expect(feedback(page)).toHaveText(noAnswer);
  await expect(submit(page)).toBeDisabled();
  expect(await calls(page)).toHaveLength(2);
});

reviewTest('a silent save unlocks at the 30-second deadline per attempt and replays with the same key', async ({ page }) => {
  const original = await startSave(page);
  await advanceToBeforeDeadline(page, original);
  await expect(submit(page)).toHaveText('Adding…');
  await expect(submit(page)).toBeDisabled();
  expect(await calls(page)).toHaveLength(2);

  await page.clock.runFor(2 * boundaryMarginMs);
  await expect(submit(page), 'A silent save releases Add as browser time crosses 30 seconds.').toBeEnabled();
  await expect(feedback(page)).toHaveText(noAnswer);
  expect(await calls(page)).toHaveLength(2);

  await submit(page).click();
  const retry = await callAt(page, 2, 'add');
  expect(retry.keyClass).toBe(original.keyClass);
  await advanceToBeforeDeadline(page, retry);
  await expect(submit(page)).toHaveText('Adding…');
  await expect(submit(page)).toBeDisabled();
  expect(await calls(page)).toHaveLength(3);

  await page.clock.runFor(2 * boundaryMarginMs);
  await expect(submit(page)).toBeEnabled();
  await expect(feedback(page)).toHaveText(noAnswer);
  expect(await calls(page)).toHaveLength(3);
});

reviewTest('an unknown message under a recognized refusal code shows the exact generic footer', async ({ page }) => {
  const message = 'Unexpected diagnostic for review fixture.';
  await firstInspection(page);
  await reply(page, 0, { code: 'failed_precondition', message }, 400);
  await expect(feedback(page)).toHaveText('The server refused the request.');
  await expect(page.getByText(message, { exact: true })).toHaveCount(0);
  await expect(submit(page)).toBeDisabled();
  expect(await calls(page)).toHaveLength(1);
});
