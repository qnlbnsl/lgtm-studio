// The live Files screen consumes engine file records without sample data.
import type { FileContent, FileListing, ContextSearchResult } from '@lgtm/contracts'
const statuses = { added: 'A', modified: 'M', deleted: 'D', renamed: 'R', untracked: 'A', conflict: 'U' } as const
export function mapListing(listing: typeof FileListing.Type) {
  return { ...listing, items: listing.items.map((entry) => ({ ...entry, status: entry.change === null ? null : statuses[entry.change] })) }
}
export function searchPaths(result: typeof ContextSearchResult.Type) {
  return { paths: result.items.flatMap((item) => item.reference._tag === 'file' ? [item.reference.path] : []), truncated: result.truncated }
}
export function readOnlyReason(context: { kind?: string; machine?: string; online: boolean }, file?: FileContent) {
  if (context.machine !== 'local') return 'Files on this machine are awaiting machine support.'
  if (!context.online) return 'The machine is offline. Its files show again when it reconnects.'
  if (context.kind === 'base') return 'The base branch is read-only. Agents never edit it directly.'
  if (file?.kind === 'text' && !file.editable) return 'The engine marked this file as not editable because its text contains private data.'
  return ''
}
interface Worktree { id: string; repo: string; project?: string | null; branch: string; kind: string }
export function worktreeFor<W extends Worktree>(state: { worktrees?: ReadonlyArray<W>; inspecting: Record<string, string> }, project: { id: string; repo: string; branch: string }) {
  const all = (state.worktrees ?? []).filter((w) => w.repo === project.repo)
  const want = state.inspecting[project.id] || project.branch
  const w = all.find((x) => x.branch === want && (!x.project || x.project === project.id)) || all.find((x) => x.project === project.id && x.kind === 'project')
  return { w: w || all.find((x) => x.kind === 'base'), missing: w ? null : want }
}
