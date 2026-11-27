import type { Repository, WatchRepositoriesResponse } from '@lgtm/api/projects';
import type { Repo, State } from '../types';

function toRepo(repository: Repository): Repo {
  const folderRecords = repository.folders
    .map(({ id, clientId, resolvedPath }) => ({ id, clientId, resolvedPath }))
    .sort((a, b) => a.id < b.id ? -1 : a.id > b.id ? 1 : 0);
  const folders = new Map<string, string>();
  for (const { clientId, resolvedPath } of folderRecords) {
    if (!folders.has(clientId)) folders.set(clientId, resolvedPath);
  }
  return {
    id: repository.id,
    name: repository.name,
    main: repository.mainBranch,
    remote: repository.remote,
    remoteOmitted: repository.remoteOmitted,
    firstCommit: repository.firstCommit,
    folders: Object.fromEntries(folders),
    folderRecords,
  };
}

export function applyRepositoryChange(s: State, { change }: WatchRepositoriesResponse): void {
  if (change.case === 'all') {
    s.repos = change.value.repositories.map(toRepo);
  } else if (change.case === 'saved') {
    const repo = toRepo(change.value);
    const index = s.repos.findIndex((r) => r.id === repo.id);
    s.repos = index === -1 ? [...s.repos, repo] : s.repos.map((r, i) => i === index ? repo : r);
  } else if (change.case === 'removedId') {
    s.repos = s.repos.filter((r) => r.id !== change.value);
  }
}
