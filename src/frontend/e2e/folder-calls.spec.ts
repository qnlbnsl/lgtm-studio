import { isDeepStrictEqual } from 'node:util';
import { create, toJson, type JsonValue } from '@bufbuild/protobuf';
import { Code, ConnectError, createClient, createRouterTransport, type Transport } from '@connectrpc/connect';
import { AddRepositoryResponseSchema, InspectFolderResponseSchema, ProjectService } from '@lgtm/api/projects';
import { expect, test } from '@playwright/test';
import { addRepository, folderIntent, inspectFolder, type FolderIntent } from '../src/server/folders';

const clientId = '018f8715-8800-7000-8000-000000000001';
const repositoryId = '018f8715-8800-7000-8000-000000000002';
const path = '~/Work tree/feature ';
const resolvedPath = '/work/Work tree/feature';
const inspected = create(InspectFolderResponseSchema, {
  facts: { resolvedPath, name: 'feature', mainBranch: 'main', checkedOutBranch: 'topic', remoteOmitted: true, clean: false },
});
const folder = { id: '018f8715-8800-7000-8000-000000000003', clientId, resolvedPath };
const added = create(AddRepositoryResponseSchema, {
  repository: { id: repositoryId, name: 'feature', mainBranch: 'main', remoteOmitted: true, folders: [folder] },
  folder,
});
const waitMessage = 'The machine did not answer the folder request.';
const expiryMessage = 'Sign in to continue.';

interface FolderRequest {
  method: string;
  body: JsonValue;
  key: string | null;
  signal: AbortSignal | undefined;
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { resolve, promise };
}

function controlledClient() {
  const routerTransport = createRouterTransport((router) => router.service(ProjectService, {
    inspectFolder: () => inspected,
    addRepository: () => added,
  }));
  const newGate = () => ({
    started: deferred<FolderRequest>(),
    reply: deferred<ConnectError | undefined>(),
  });
  const gates: ReturnType<typeof newGate>[] = [];
  const transport: Transport = {
    stream: routerTransport.stream,
    async unary(method, signal, timeoutMs, header, input, contextValues) {
      const gate = gates.shift();
      if (!gate) throw new Error('The folder call had no scheduled reply.');
      gate.started.resolve({
        method: method.name,
        body: toJson(method.input, create(method.input, input)),
        key: new Headers(header).get('Lgtm-Request-Key'),
        signal,
      });
      const error = await gate.reply.promise;
      if (error) throw error;
      return routerTransport.unary(method, signal, timeoutMs, header, input, contextValues);
    },
  };
  return {
    client: createClient(ProjectService, transport),
    hold() {
      const gate = newGate();
      gates.push(gate);
      return { started: gate.started.promise, release: gate.reply.resolve };
    },
  };
}

async function begin(
  fixture: ReturnType<typeof controlledClient>,
  invoke: typeof inspectFolder | typeof addRepository,
  intent: FolderIntent,
  isCurrent = () => true,
) {
  const { started, release } = fixture.hold();
  const { signal } = new AbortController();
  const settled = invoke(intent, { client: fixture.client, signal, isCurrent }).then(
    (value) => ({ returned: true, value, error: undefined }),
    (error: unknown) => ({ returned: false, value: undefined, error }),
  );
  // A wrapper that settles without dispatching fails here, without waiting for a timeout.
  const request = await Promise.race([started, settled.then(() => undefined)]);
  expect(Boolean(request), 'The folder wrapper dispatched a generated request.').toBe(true);
  return { request: request!, signal, release, settled };
}

const procedures = [
  { procedure: 'inspect', method: 'InspectFolder', invoke: inspectFolder, response: inspected },
  { procedure: 'add', method: 'AddRepository', invoke: addRepository, response: added },
] as const;

for (const { procedure, method, invoke, response } of procedures) {
  const targets: { label: string; target?: string }[] = procedure === 'inspect'
    ? [{ label: 'inspection' }]
    : [{ label: 'add with an omitted target' }, { label: 'add with an explicit target', target: repositoryId }];

  for (const { label, target } of targets) {
    test.describe(label, () => {
      test('folder procedures keep the intent key', async () => {
        const fixture = controlledClient();
        const fields = target === undefined ? { clientId, path } : { clientId, path, repositoryId: target };
        const intent = folderIntent({ procedure, ...fields });
        const first = await begin(fixture, invoke, intent);
        first.release(new ConnectError(waitMessage, Code.DeadlineExceeded));
        const { error } = await first.settled;
        expect(error instanceof ConnectError && error.code === Code.DeadlineExceeded && error.rawMessage === waitMessage,
          'The current wait refusal reaches the caller.').toBe(true);

        const retry = await begin(fixture, invoke, intent);
        retry.release(undefined);
        const result = await retry.settled;
        for (const attempt of [first, retry]) {
          expect(attempt.request.method).toBe(method);
          expect(isDeepStrictEqual(attempt.request.body, fields),
            'The request preserves the machine, exact typed path and target presence.').toBe(true);
          expect(attempt.request.signal === attempt.signal, 'The call uses the attempt signal.').toBe(true);
          expect(attempt.request.key !== null && attempt.request.key === intent.key,
            'The request has exactly the intent key in its header.').toBe(true);
        }
        expect(retry.request.signal === first.request.signal, 'A retry uses a fresh signal.').toBe(false);
        expect(retry.request.key === first.request.key, 'A retry repeats the original key.').toBe(true);
        expect(result.returned && isDeepStrictEqual(result.value, response),
          'The current reply reaches the caller with the generated facts or saved identities.').toBe(true);
      });
    });
  }

  test.describe(method, () => {
    for (const late of ['success', 'expiry'] as const) {
      test(`an obsolete ${late} is discarded`, async () => {
        const fixture = controlledClient();
        const intent = folderIntent({ procedure, clientId, path });
        let current = true;
        const call = await begin(fixture, invoke, intent, () => current);
        current = false;
        // A transport can finish after replacement but before it observes cancellation.
        call.release(late === 'expiry' ? new ConnectError(expiryMessage, Code.Unauthenticated) : undefined);
        const { returned, value } = await call.settled;
        expect(returned && value === undefined, 'An obsolete attempt resolves to undefined.').toBe(true);
      });
    }
  });
}

test('new folder intents keep their own inputs and keys', () => {
  const input = { procedure: 'add' as const, clientId, path };
  const first = folderIntent(input);
  input.path = '~/Another checkout';
  const edited = folderIntent(input);
  input.path = path;
  const restored = folderIntent(input);
  const reopened = folderIntent({ ...input });
  const inspection = folderIntent({ procedure: 'inspect', clientId, path });
  const attached = folderIntent({ ...input, repositoryId });
  const keys = [first, edited, restored, reopened, inspection, attached].map((intent) => intent.key);

  expect(first.procedure === 'add' && first.clientId === clientId && first.path === path && first.repositoryId === undefined,
    'An intent keeps the original input after an edit.').toBe(true);
  expect(edited.path === '~/Another checkout' && restored.path === path && inspection.procedure === 'inspect' && attached.repositoryId === repositoryId,
    'New intents keep the edited path, changed procedure and explicit target.').toBe(true);
  expect(keys.every((key) => typeof key === 'string' && key.length > 0 && new TextEncoder().encode(key).length <= 256),
    'Every intent has a nonempty request key within the contract limit.').toBe(true);
  expect(new Set(keys).size, 'Activation, input and procedure changes get distinct keys.').toBe(keys.length);
});
