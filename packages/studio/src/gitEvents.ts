import type { ClientEnvelope } from '@lgtm/client/browser'
import type { ClientSnapshot } from '@lgtm/contracts'

export interface GitEventPatch {
  readonly group: 'summary' | 'review'
  readonly worktreeIds: ReadonlyArray<string>
  readonly requestId?: string | undefined
  readonly completed?: boolean
}
// A completion without scope conservatively invalidates the visible project.
// No attempt or receipt table is retained in the browser.
export function gitEventPatch(item: ClientEnvelope, snapshot?: ClientSnapshot): GitEventPatch | undefined {
  const event = item.event, requestId = item.requestId
  switch (event._tag) {
    case 'project.GitAttempted': {
      const id = event.attempt.worktreeId
      const rows = snapshot?.project?.workspaces ?? []
      const source = rows.find((w) => w.worktreeId === id)
      const roots = [id, ...(source?.parentId ? [source.parentId] : [])]
      return { group: 'summary', worktreeIds: [...new Set([...roots, ...rows.filter((w) => w.parentId && roots.includes(w.parentId)).map((w) => w.worktreeId)])], requestId: event.attempt.requestId }
    }
    case 'project.GitFinished': return { group: 'summary', worktreeIds: [], requestId: event.operationRequestId, completed: true }
    case 'project.GitEvidenceReported': return { group: 'review', worktreeIds: [event.report.worktreeId], requestId }
    case 'project.MainReviewOpened':
    case 'project.MainReviewDecided': return { group: 'review', worktreeIds: [event.review.worktreeId], requestId, completed: true }
    case 'TurnEnded': {
      const agent = snapshot?.agents.find((a) => a.agentId === event.agentId)
      if (!agent) return undefined
      const ids = snapshot?.project?.workspaces.filter((w) => w.path === agent?.folder).map((w) => w.worktreeId) ?? []
      return ids.length ? { group: 'summary', worktreeIds: ids, requestId } : undefined
    }
    default: return undefined
  }
}
