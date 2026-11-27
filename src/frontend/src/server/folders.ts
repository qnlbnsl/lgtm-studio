import type { Client } from '@connectrpc/connect';
import type { AddRepositoryResponse, InspectFolderResponse, ProjectService } from '@lgtm/api/projects';

type FolderInput = { procedure: 'inspect' | 'add'; clientId: string; path: string; repositoryId?: string };
export type FolderIntent = Readonly<FolderInput & { key: string }>;

type FolderAttempt = {
  client: Pick<Client<typeof ProjectService>, 'inspectFolder' | 'addRepository'>;
  signal: AbortSignal;
  isCurrent: () => boolean;
};

// This is a stub from the test writer.
export function folderIntent(input: FolderInput): FolderIntent {
  return { ...input, key: '' };
}

// This is a stub from the test writer.
export async function inspectFolder(_intent: FolderIntent, _attempt: FolderAttempt): Promise<InspectFolderResponse | undefined> {
  return undefined;
}

// This is a stub from the test writer.
export async function addRepository(_intent: FolderIntent, _attempt: FolderAttempt): Promise<AddRepositoryResponse | undefined> {
  return undefined;
}
