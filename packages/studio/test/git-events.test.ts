import { expect, it } from 'vitest'
import { gitEventPatch } from '../src/gitEvents.ts'
import type { ClientEnvelope } from '@lgtm/client/browser'
import type { ClientSnapshot } from '@lgtm/contracts'
const item = (event: object) => ({ event, requestId: 'caller' }) as ClientEnvelope
const snapshot = { agents: [{ agentId: 'worker', folder: '/team' }], project: { workspaces: [
  { worktreeId: 'source', path: '/team', parentId: 'parent' }, { worktreeId: 'parent' }, { worktreeId: 'sibling', parentId: 'parent' }
] } } as unknown as ClientSnapshot
it('forwards review and evidence invalidations without changing their contents', () => {
  for (const event of [{ _tag: 'project.GitEvidenceReported', report: { worktreeId: 'source' } },
    { _tag: 'project.MainReviewOpened', review: { worktreeId: 'source' } }, { _tag: 'project.MainReviewDecided', review: { worktreeId: 'source' } }]) {
    expect(gitEventPatch(item(event))).toMatchObject({ group: 'review', worktreeIds: ['source'], requestId: 'caller' })
  }
})
it('ignores attempts and maps only completions to the visible project', () => {
  expect(gitEventPatch(item({ _tag: 'project.GitAttempted', attempt: { worktreeId: 'source', requestId: 'operation' } }), snapshot)).toBeUndefined()
  expect(gitEventPatch(item({ _tag: 'project.GitFinished', operationRequestId: 'operation' }))).toEqual({ group: 'summary', worktreeIds: [], requestId: 'operation' })
})
it('a turn uses current placement and needs no waiting collection', () => {
  expect(gitEventPatch(item({ _tag: 'TurnEnded', agentId: 'worker' }), snapshot)?.worktreeIds).toEqual(['source'])
  expect(gitEventPatch(item({ _tag: 'TurnEnded', agentId: 'unplaced' }), snapshot)).toBeUndefined()
  expect(gitEventPatch(item({ _tag: 'StateChanged' }), snapshot)).toBeUndefined()
})
