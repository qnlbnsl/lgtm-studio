import type { Event, ProjectView } from '@lgtm/contracts'
import type { Snapshot } from '../commands/internal.ts'

// Tombstones contain identifiers only. Events remain in the log, but no read
// may reintroduce a deleted project's records by reading the log directly.
const emptyIndex = () => Object.fromEntries(['projectId', 'agentId', 'channelId', 'postId', 'worktreeId', 'alertId'].map((key) => [key, new Set<string>()]))
const absent = emptyIndex()
const cache = new WeakMap<NonNullable<ProjectView['deletions']>, Record<string, Set<string>>>()
const index = (snapshot: Snapshot) => {
  if (snapshot.project === null || typeof snapshot.project !== 'object') return absent
  const deletions = (snapshot.project as ProjectView).deletions
  if (deletions === undefined) return absent
  let ids = cache.get(deletions)
  if (ids !== undefined) return ids
  ids = emptyIndex()
  for (const row of Object.values(deletions)) {
    ids.projectId!.add(row.projectId)
    for (const [field, values] of [['agentId', row.agentIds], ['channelId', row.channelIds], ['postId', row.postIds], ['alertId', row.alertIds], ['worktreeId', row.worktreeIds]] as const)
      for (const value of values) ids[field]!.add(value)
  }
  cache.set(deletions, ids)
  return ids
}
export const deletedAgent = (snapshot: Snapshot, agentId: string) => index(snapshot).agentId!.has(agentId)
export const deletedEvent = (snapshot: Snapshot, event: Event): boolean => {
  // The deletion itself carries no prose and tells connected clients to reload.
  if (event._tag === 'project.Deleted') return false
  const ids = index(snapshot)
  const aliases: Record<string, string> = { anchorAgentId: 'agentId', threadAgentId: 'agentId', leadAgentId: 'agentId',
    target: 'agentId', targetAgentId: 'agentId', authorAgentId: 'agentId', parentId: 'worktreeId', destinationId: 'worktreeId', replyToPostId: 'postId', replyTo: 'postId' }
  // Inspect protocol metadata, never agent tool input or message content.
  const nested = new Set(['caller', 'parent', 'sender', 'delivery', 'firstPromptDelivery', 'review', 'report', 'attempt', 'creation', 'workspace', 'context', 'to', 'members', 'manager', 'opener', 'author', 'closer', 'answeredBy'])
  const linked = (value: unknown): boolean => {
    if (Array.isArray(value)) return value.some(linked)
    if (value === null || typeof value !== 'object') return false
    return Object.entries(value).some(([key, child]) => typeof child === 'string'
      ? ids[aliases[key] ?? key]?.has(child) === true : nested.has(key) && linked(child))
  }
  // Other authors retain their shared-channel posts, even replies or context
  // references to removed records. Those references grant no read authority.
  if (event._tag === 'board.PostWritten') {
    const { context: _context, replyToPostId: _reply, to: _to, ...owned } = event
    return linked(owned)
  }
  return linked(event)
}
