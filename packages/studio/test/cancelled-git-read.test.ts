import { afterEach, expect, it, vi } from 'vitest'
import * as policy from '../src/gitRefresh.ts'
import { fixture, flush } from './cancelled-read-fixture.ts'
let f: Awaited<ReturnType<typeof fixture>>, stop: (() => void) | undefined
afterEach(() => { stop?.(); stop = undefined; f?.close(); vi.unstubAllGlobals(); vi.useRealTimers() })
async function start() {
  f = await fixture(); stop = f.git.openScreen('git', 'p'); await f.cancel()
  return f.state.worktrees[0]
}
const cases: [string, (w: any) => Promise<unknown>][] = [
  ['refreshGit', () => f.git.refreshGit('p')], ['gitDetail', () => f.git.gitDetail('w')],
  ['selectGit', () => f.git.selectGit('w', 'changes')], ['stage', () => f.git.stage('w', [], true, true)],
  ['discard', w => f.git.discard(w, w.files)], ['commit', () => f.git.commit('w', 'message')],
  ['syncGit', () => f.git.syncGit('w', 'fetch')], ['integrate', () => f.git.integrate('w')],
  ['answerReview', () => f.git.answerReview('review', 'merge')], ['resolve', () => f.git.resolve('w', 'file', true)],
  ['continueMerge', () => { f.state.gitUi.w.message = 'merge'; return f.git.continueMerge('w') }],
]
for (const [name, action] of cases) it(`${name} releases its Git latch after a cancelled waiting read`, async () => {
  const w = await start(); w.files = [{ id: 'file', path: 'file' }]; w.merge = { files: [{ id: 'file', selection: {} }] }
  const pending = action(w); await flush()
  const key = name === 'discard' ? 'git-plan' : 'git'; expect(f.git.busy(key)).toBe(true)
  let done = false; void pending.then(() => { done = true }); await f.cancel()
  expect(done).toBe(true); expect(f.git.busy(key)).toBe(false)
})
it('C: hiding the tab finishes an explicit refresh, admits the next owner action and resumes automatic reads', async () => {
  vi.useFakeTimers(); await start()
  const pending = f.git.refreshGit('p'); await flush(); expect(f.git.busy('git')).toBe(true)
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(true); document.dispatchEvent(new Event('visibilitychange'))
  let done = false; void pending.then(() => { done = true }); await flush()
  expect(done).toBe(true); expect(f.git.busy('git')).toBe(false)
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false); document.dispatchEvent(new Event('visibilitychange'))
  f.online(); await f.git.stage('w', [], true, true)
  expect(f.calls.some(c => c.tag === 'project.stage.all')).toBe(true)
  const c = f.git.status().gitController
  policy.dirtyGitRecord(c.records.get('w'), 'summary', Date.now());
  expect(policy.nextAutomaticRead(Date.now() + policy.fallbackMs, c)).not.toBeNull()
  await vi.advanceTimersByTimeAsync(policy.fallbackMs)
  expect(c.records.get('w').phase).not.toBe('reading')
})
it('D: an automatic detail cancelled by selecting a file retires its record and admits the next read', async () => {
  vi.useFakeTimers(); await start()
  const c = f.git.status().gitController, r = c.records.get('w')
  // Summaries have arrived; reconnect begins immediately before the detail.
  const send = async (tag: string, payload: any, options: any) => tag === 'project.diff'
    ? f.engine.request(tag, payload, options)
    : { result: tag === 'project.git.state' ? { mergeHead: null } : tag === 'project.review.status' ? { review: null } : { items: [{ id: 'file', path: 'file' }], truncated: false } }
  const automatic = f.git.readGitRecord(f.state.worktrees[0], 'p', r, c, false, false, send)
  await flush(); expect(r.phase).toBe('reading'); expect(r.detailAbort).toBeDefined()
  const selection = f.git.gitDetail('w'); await flush()
  let retired = false; void automatic.then(() => { retired = true }); await flush()
  expect(retired).toBe(true); expect(r.phase).not.toBe('reading'); expect(r.summaryAbort).toBeUndefined()
  await f.cancel(); await selection
  expect(policy.nextAutomaticRead(Date.now() + policy.fallbackMs, c)).not.toBeNull()
})
it('discard confirmation releases git after cancellation of its readback', async () => {
  const w = await start(); f.online()
  // A real discard plan is required before opening the confirmation.
  const original = f.env.engine.request
  f.env.engine.request = (tag: string, p: any, o: any) => tag === 'project.discard.plan' ? Promise.resolve({ result: { paths: ['file'] } }) : original(tag, p, o)
  await f.git.discard(w, [{ path: 'file' }]); f.waiting()
  const pending = f.dialogs[0].onConfirm(); await flush(); expect(f.git.busy('git')).toBe(true)
  await f.cancel(); await pending; expect(f.git.busy('git')).toBe(false)
})
it('loadGitDetail releases its detail controller after a cancelled waiting read', async () => {
  const w = await start(); w.files = [{ id: 'file', path: 'file' }]
  const c = f.git.status().gitController, r = c.records.get('w')
  const work = f.git.loadGitDetail('w'); await flush(); expect(r.detailAbort).toBeDefined()
  let done = false; void work.then(() => { done = true }); await f.cancel()
  expect(done).toBe(true); expect(r.detailAbort).toBeUndefined()
})
