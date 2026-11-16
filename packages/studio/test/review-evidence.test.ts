import { readFileSync } from 'node:fs'
import { expect, it, vi } from 'vitest'
import type { GitMainReview, GitEvidenceSet } from '@lgtm/contracts'
import * as mapping from '../src/git.ts'
import { gitActionsSource } from '../scripts/gitActions.ts'

const evidenceStates = ['none', 'passing current', 'failed current', 'passing stale', 'failed stale'] as const
type FreshEvidence = typeof evidenceStates[number] | 'passing newer'
function evidence(name: FreshEvidence): GitEvidenceSet {
  const slot = name === 'none' ? { state: 'missing', report: null } : { state: name.endsWith('stale') ? 'stale' : 'current', report: {
    head: name === 'passing newer' ? 'newer-head' : name.endsWith('stale') ? 'older' : 'head', status: name.startsWith('passing') ? 'passed' : 'failed', summary: name, reportedBy: { _tag: 'owner' } } }
  return { tests: slot, review: slot } as GitEvidenceSet
}
async function fixture(stored: typeof evidenceStates[number], fresh: FreshEvidence, refusal?: string, rejection?: Error) {
  const review = { reviewId: 'review', projectId: 'project', worktreeId: 'w', destinationId: 'base', branch: 'topic', base: 'main', head: 'head', baseHead: 'base-head', outcome: 'open', evidence: evidence(stored) } as GitMainReview
  const w = { id: 'w', project: 'project', repo: 'repo', kind: 'project', machine: 'local', head: 'head', branch: 'topic', files: [], review: { ...mapping.mapMainReview(review, undefined, 'lead'), files: [{ path: 'cached' }], evidence: { ...mapping.mapMainReview(review, undefined, 'lead').evidence, commits: 3 } } }
  const state = { session: { status: 'in' }, ui: {}, toasts: [] as { id: string; text: string; tone: string }[], projects: [{ id: 'project' }], worktrees: [w], gitUi: {} }
  const toasts: any[] = []
  let decided = review
  const request = vi.fn(async (tag: string) => {
    if (tag === 'project.review.status') return { result: { review: decided, evidence: evidence(fresh) } }
    if (tag === 'project.review.approve') { if (rejection) throw rejection; decided = { ...review, outcome: refusal ? 'refused' : 'merged', reason: refusal ?? null }; return { result: decided } }
    if (tag === 'project.git.state') return { result: { head: 'head' } }
    if (tag === 'project.changes') return { result: { items: [] } }
    throw new Error(tag)
  })
  const { errorText } = await import('../../../design/mock/js/live/reads.js' as string)
  const actionsSource = readFileSync('design/mock/js/actions.js', 'utf8')
  const toastStart = actionsSource.indexOf('export function toast('), toastEnd = actionsSource.indexOf('\n}', toastStart) + 2
  const timers: ReturnType<typeof setTimeout>[] = []
  const toast = new Function('errorText', 'uid', 'update', 'setTimeout', actionsSource.slice(toastStart, toastEnd).replace('export ', '') + '; return toast;')(
    errorText, () => 'notice', (change: (value: typeof state) => void) => change(state), (cb: () => void, ms: number) => { const timer = setTimeout(cb, ms); timers.push(timer); return timer })
  const closeDialog = vi.fn()
  const source = gitActionsSource('js/live/git.js', readFileSync('design/mock/js/live/git.js', 'utf8')).replace(/^import .*;$/gm, '').replace(/export /g, '')
  const actions = new Function('store', 'engineWrite', 'notify', 'engine', 'act', 'live', `${source}; return { installMappings, readGitRecord, answerReview, busy };`)(
    { state }, (patch: object) => Object.assign(state, patch), () => {}, { request }, { toast: (error: unknown, tone: string, fallback?: string) => { toasts.push(error); toast(error, tone, fallback) }, closeDialog }, { on: false })
  actions.installMappings(mapping)
  const r = mapping.gitRefreshRecord(JSON.stringify([w.repo, undefined, w.branch, w.machine, undefined]), 0)
  r.summaryDirty = 0; r.lastSuccess = 0
  await actions.readGitRecord(w, 'project', r, undefined, false, true)
  const kit = await import('../../../design/mock/vendor/htm-preact/standalone.module.js' as string)
  const dialogs = gitActionsSource('js/dialogs.js', readFileSync('design/mock/js/dialogs.js', 'utf8'))
  const component = dialogs.slice(dialogs.indexOf('function MergeReview('), dialogs.indexOf("registerDialog('merge-review'"))
  const MergeReview = new Function('liveGit', 'live', 'html', 'useState', 'cx', 'act', 'Dialog', 'Icon', 'MergeFacts', `${component}; return MergeReview;`)(
    actions, { on: true }, kit.html, kit.useState, (...v: unknown[]) => v.filter(Boolean).join(' '), {}, ({ foot }: any) => foot, () => null, () => null)
  const root = document.createElement('div')
  const draw = () => kit.render(kit.h(MergeReview, { s: state, d: { worktreeId: 'w' } }), root)
  draw()
  return { state, request, root, toasts, actions, draw, closeDialog, close: () => { kit.render(null, root); timers.forEach(clearTimeout) } }
}
for (const stored of evidenceStates) for (const fresh of evidenceStates) {
  it(`stored ${stored}; fresh ${fresh}: offers and records the fresh decision`, async () => {
    const f = await fixture(stored, fresh)
    try {
      const approve = [...f.root.querySelectorAll<HTMLButtonElement>('button')].find(b => b.textContent!.startsWith('Approve'))!
      expect(approve.textContent).toBe(fresh === 'passing current' ? 'Approve and merge' : 'Approve without evidence')
      expect(f.state.worktrees[0]!.review.evidence).toMatchObject({ tests: mapping.mapMainReview({ head: 'head', evidence: evidence(fresh) } as GitMainReview, undefined, '').evidence.tests, review: mapping.mapMainReview({ head: 'head', evidence: evidence(fresh) } as GitMainReview, undefined, '').evidence.review })
      expect(f.state.worktrees[0]!.review.files).toEqual([{ path: 'cached' }])
      expect(f.state.worktrees[0]!.review.evidence.commits).toBe(3)
      approve.click()
      await vi.waitFor(() => expect(f.request).toHaveBeenCalledWith('project.review.approve', expect.objectContaining({ decision: fresh === 'passing current' ? 'approve' : 'approve_without_evidence' })))
    } finally { f.close() }
  })
}
it('records the decision rendered on the pressed control even when the store has refreshed first', async () => {
  const f = await fixture('passing current', 'passing current')
  try {
    const approve = [...f.root.querySelectorAll<HTMLButtonElement>('button')].find(b => b.textContent === 'Approve and merge')!
    f.state.worktrees[0]!.review = { ...f.state.worktrees[0]!.review, head: 'new-head', baseHead: 'new-base', evidence: { ...f.state.worktrees[0]!.review.evidence, tests: { ok: false } as any } }
    approve.click()
    await vi.waitFor(() => expect(f.request).toHaveBeenCalledWith('project.review.approve', expect.objectContaining({ decision: 'approve', expectedHead: 'head', expectedBaseHead: 'base-head' })))
  } finally { f.close() }
})

it('shows the engine refusal through the existing error path after a displayed passing approval', async () => {
  const f = await fixture('none', 'passing current', 'Git: stale_head')
  try {
    const approve = [...f.root.querySelectorAll<HTMLButtonElement>('button')].find(b => b.textContent === 'Approve and merge')!
    approve.click()
    await vi.waitFor(() => expect(f.toasts).toContainEqual(expect.objectContaining({ message: 'Git: stale_head' })))
    expect(f.state.worktrees[0]!.review).toMatchObject({ status: 'answered', outcome: 'refused', answer: 'Git: stale_head' })
    expect(f.request.mock.calls.filter(([tag]) => tag === 'project.review.approve')).toHaveLength(1)
  } finally { f.close() }
})

it('offers approval without evidence when passing status reports belong to a newer head than the review', async () => {
  const f = await fixture('none', 'passing newer')
  try {
    expect(f.state.worktrees[0]!.review.evidence).toMatchObject({ tests: { ok: false, value: 'stale: passed' }, review: { ok: false, value: 'stale: passed' } })
    const approve = [...f.root.querySelectorAll<HTMLButtonElement>('button')].find(b => b.textContent === 'Approve without evidence')!
    expect(approve).toBeDefined()
    approve.click()
    await vi.waitFor(() => expect(f.request).toHaveBeenCalledWith('project.review.approve', expect.objectContaining({ decision: 'approve_without_evidence', expectedHead: 'head' })))
  } finally { f.close() }
})

it('shows the request-level stale-pair refusal and leaves the open dialog usable', async () => {
  const message = 'main review is unavailable; prepare a new review'
  const rejection = Object.assign(new Error(message), { name: 'CommandRefused', _tag: 'CommandRefused', reason: message })
  const f = await fixture('none', 'passing current', undefined, rejection)
  try {
    const approve = () => [...f.root.querySelectorAll<HTMLButtonElement>('button')].find(b => b.textContent === 'Approve and merge')!
    approve().click()
    await vi.waitFor(() => expect(f.state.toasts).toContainEqual(expect.objectContaining({ text: message, tone: 'error' })))
    f.draw()
    expect(f.state.worktrees[0]!.review).toMatchObject({ status: 'open', outcome: 'open' })
    expect(f.actions.busy('git')).toBe(false)
    expect(approve().disabled).toBe(false)
    expect([...f.root.querySelectorAll<HTMLButtonElement>('button')].find(b => b.textContent === 'Cancel')!.disabled).toBe(false)
    expect(f.closeDialog).not.toHaveBeenCalled()
    expect(f.request.mock.calls.filter(([tag]) => tag === 'project.review.approve')).toHaveLength(1)
  } finally { f.close() }
})
