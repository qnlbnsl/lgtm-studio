import type { CallOptions, Client } from '@connectrpc/connect';
import type { AddRepositoryResponse, InspectFolderResponse, ProjectService } from '@lgtm/api/projects';

type FolderInput = { procedure: 'inspect' | 'add'; clientId: string; path: string; repositoryId?: string };
export type FolderIntent = Readonly<FolderInput & { key: string }>;

type FolderAttempt = {
  client: Pick<Client<typeof ProjectService>, 'inspectFolder' | 'addRepository'>;
  signal: AbortSignal;
  isCurrent: () => boolean;
};

export function folderIntent({ procedure, clientId, path, repositoryId }: FolderInput): FolderIntent {
  const key = Array.from(crypto.getRandomValues(new Uint8Array(16)), (byte) =>
    byte.toString(16).padStart(2, '0'),
  ).join('');
  const target = procedure === 'add' && repositoryId !== undefined ? { repositoryId } : {};
  return Object.freeze({ procedure, clientId, path, ...target, key });
}

async function callFolder<T>(
  intent: FolderIntent,
  { signal, isCurrent }: FolderAttempt,
  call: (options: CallOptions) => Promise<T>,
): Promise<T | undefined> {
  if (!isCurrent()) return undefined;
  let reply: T;
  try {
    reply = await call({ signal, headers: { 'Lgtm-Request-Key': intent.key } });
  } catch (error) {
    if (!isCurrent()) return undefined;
    throw error;
  }
  return isCurrent() ? reply : undefined;
}

export function inspectFolder(intent: FolderIntent, attempt: FolderAttempt): Promise<InspectFolderResponse | undefined> {
  const { clientId, path } = intent;
  return callFolder(intent, attempt, (options) => attempt.client.inspectFolder({ clientId, path }, options));
}

export function addRepository(intent: FolderIntent, attempt: FolderAttempt): Promise<AddRepositoryResponse | undefined> {
  const { clientId, path, repositoryId } = intent;
  const target = repositoryId === undefined ? {} : { repositoryId };
  return callFolder(intent, attempt, (options) => attempt.client.addRepository({ clientId, path, ...target }, options));
}
