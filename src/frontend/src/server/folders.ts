import { Code, ConnectError, type CallOptions, type Client } from '@connectrpc/connect';
import type { AddRepositoryResponse, InspectFolderResponse, ProjectService } from '@lgtm/api/projects';
import type { Machine } from '../types';

/** Prefers the registered opener, then the first online machine. */
export function defaultFolderMachine(machines: readonly Machine[], opener?: string): string {
  return machines.find((m) => m.id === opener)?.id ?? machines.find((m) => m.status === 'online')?.id ?? '';
}

export const folderMachineRemoved = 'The machine was removed.';
export const folderRepositoryMissing = 'The repository was not found.';
export const folderRejected = 'The server refused the request.';
export const folderNetwork = 'The server did not answer.';
const folderWait = 'The machine did not answer the folder request.';
// The server can wait 25 seconds for the machine. Allow five more seconds for transport.
const folderAttemptTimeout = 30_000;

// The build contract pairs each public sentence with its Connect code.
const folderRefusals: Partial<Record<Code, readonly string[]>> = {
  [Code.Unavailable]: ['The machine is not connected.'],
  [Code.InvalidArgument]: [
    'The machine, repository, or folder path is invalid.', 'The request key is invalid.', 'The request body is invalid.',
  ],
  [Code.AlreadyExists]: [
    'That request key was already used for a different request.', 'The folder is already saved for another repository.',
  ],
  [Code.NotFound]: ['The machine was not found.', folderRepositoryMissing, folderMachineRemoved],
  [Code.FailedPrecondition]: [
    'The path is not a folder.', 'The folder is not a Git repository.', 'The folder cannot be read.',
    'The folder path cannot be used.', 'The folder does not belong to the selected repository.',
  ],
  [Code.DeadlineExceeded]: [
    folderWait, 'The machine did not take the folder command.', 'The outcome of the folder request is unknown.',
  ],
};

/** Displays only contract sentences or the existing generic feedback. */
export function folderFailure(error: unknown): { text: string; retry: boolean } {
  if (error instanceof ConnectError) {
    const { code, rawMessage } = error;
    if (folderRefusals[code]?.includes(rawMessage)) return { text: rawMessage, retry: rawMessage === folderWait };
    if (code === Code.DeadlineExceeded || code === Code.Unavailable ||
        (code === Code.Unknown && error.cause instanceof TypeError)) {
      return { text: folderNetwork, retry: true };
    }
    return { text: folderRejected, retry: false };
  }
  return { text: folderNetwork, retry: true };
}

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
    reply = await call({ signal, timeoutMs: folderAttemptTimeout, headers: { 'Lgtm-Request-Key': intent.key } });
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
