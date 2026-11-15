// Test-only request timing. Retain only tags, synthetic worktree identifiers,
// request correlation identifiers, and monotonic timestamps, never RPC bodies.
import { mkdirSync, writeFileSync } from 'node:fs'
import { loadavg } from 'node:os'
import { join } from 'node:path'
import type { Page } from '@playwright/test'
import * as Effect from 'effect/Effect'
import { CommandHandler } from '../../../engine/src/commands/CommandHandler.ts'
import { engineLists } from '../../../engine/src/pieces/lists.ts'
import type { ReadReply } from '../../../engine/src/api/reads.ts'

const gitTags = new Set([
  'project.git.state', 'project.changes', 'project.diff', 'project.history', 'project.branches',
  'project.commit.files', 'project.conflict', 'project.conflict.resolved', 'project.conflict.choose',
  'project.discard.plan', 'project.stage', 'project.unstage', 'project.stage.all', 'project.unstage.all',
  'project.discard', 'project.commit', 'project.fetch', 'project.pull', 'project.push', 'project.integrate',
  'project.update', 'project.merge.abort', 'project.conflict.reopen', 'project.conflict.resolve',
  'project.review.prepare', 'project.review.approve', 'project.review.decline', 'project.review.report', 'project.review.status'
])
type Sample = { tag: string; worktree: string; start: number; end?: number }
type EngineSample = Sample & { requestId?: string }
const unionMs = (samples: ReadonlyArray<Sample>) => {
  const intervals = samples.filter((s) => s.end !== undefined).map((s) => [s.start, s.end!] as const).sort((a, b) => a[0] - b[0])
  let end = 0, total = 0
  for (const [start, finish] of intervals) { total += Math.max(0, finish - Math.max(start, end)); end = Math.max(end, finish) }
  return total
}
const aggregate = (samples: ReadonlyArray<Sample>) => {
  const rows = new Map<string, { tag: string; worktree: string; count: number; completed: number; durationMs: number; maxMs: number }>()
  for (const s of samples) {
    const key = JSON.stringify([s.tag, s.worktree]), row = rows.get(key) ?? { tag: s.tag, worktree: s.worktree, count: 0, completed: 0, durationMs: 0, maxMs: 0 }
    row.count++
    if (s.end !== undefined) { const duration = s.end - s.start; row.completed++; row.durationMs += duration; row.maxMs = Math.max(row.maxMs, duration) }
    rows.set(key, row)
  }
  return [...rows.values()].sort((a, b) => a.tag.localeCompare(b.tag) || a.worktree.localeCompare(b.worktree))
}

export function measureGitJourney(page: Page) {
  const directory = process.env.LGTM_GIT_MEASURE_DIR
  if (!directory) return {
    lists: engineLists,
    server<A, E, R>(effect: Effect.Effect<A, E, R>) { return effect },
    finish() {}
  }
  const started = performance.now(), initialLoad = loadavg(), requests: Sample[] = [], engines: EngineSample[] = []
  const browserIds = new Set<string>()
  const stamp = () => performance.now() - started
  const frames = (payload: string | Buffer): ReadonlyArray<Record<string, unknown>> => {
    try { const value: unknown = JSON.parse(String(payload)); return (Array.isArray(value) ? value : [value]).filter((v): v is Record<string, unknown> => typeof v === 'object' && v !== null) }
    catch { return [] }
  }
  const socketOpened = (socket: import('@playwright/test').WebSocket) => {
    const pending = new Map<string, Sample>()
    socket.on('framesent', ({ payload }) => {
      for (const frame of frames(payload)) {
        if (frame._tag !== 'Request' || typeof frame.tag !== 'string' || frame.tag.endsWith('subscribe')) continue
        const body = frame.payload as { worktreeId?: unknown; requestId?: unknown } | undefined
        const sample: Sample = { tag: frame.tag, worktree: typeof body?.worktreeId === 'string' ? body.worktreeId : 'none', start: stamp() }
        requests.push(sample); pending.set(String(frame.id), sample)
        if (typeof body?.requestId === 'string') browserIds.add(body.requestId)
      }
    })
    socket.on('framereceived', ({ payload }) => {
      for (const frame of frames(payload)) {
        if (frame._tag !== 'Exit') continue
        const sample = pending.get(String(frame.requestId))
        if (sample) { sample.end = stamp(); pending.delete(String(frame.requestId)) }
      }
    })
  }
  page.on('websocket', socketOpened)
  const timed = <A, E, R>(tag: string, payload: unknown, effect: Effect.Effect<A, E, R>): Effect.Effect<A, E, R> => Effect.suspend(() => {
    const body = payload as { worktreeId?: unknown; requestId?: unknown }
    const sample: EngineSample = { tag, worktree: typeof body.worktreeId === 'string' ? body.worktreeId : 'none', start: stamp(),
      ...(typeof body.requestId === 'string' ? { requestId: body.requestId } : {}) }
    engines.push(sample)
    return effect.pipe(Effect.ensuring(Effect.sync(() => { sample.end = stamp() })))
  })
  const lists = { ...engineLists, reads: Object.fromEntries(Object.entries(engineLists.reads).map(([tag, read]) => [tag,
    gitTags.has(tag) ? ((input, payload) => timed(tag, payload, read(input, payload) as Effect.Effect<ReadReply, unknown, any>)) as typeof read : read
  ])) }
  return {
    lists,
    server<A, E, R>(effect: Effect.Effect<A, E, R>) {
      return Effect.flatMap(CommandHandler, (handler) => effect.pipe(Effect.provideService(CommandHandler, { ...handler,
        execute: (credential, tag, payload) => gitTags.has(tag) ? timed(tag, payload, handler.execute(credential, tag, payload)) : handler.execute(credential, tag, payload)
      })))
    },
    finish() {
      try {
        page.off('websocket', socketOpened)
        const elapsedMs = stamp(), gitRequests = requests.filter((r) => gitTags.has(r.tag))
        const gitEngine = engines.filter((s) => s.requestId !== undefined && browserIds.has(s.requestId))
        const report = {
          elapsedMs, loadStart: initialLoad, loadEnd: loadavg(), requests: aggregate(requests), gitRequests: aggregate(gitRequests), engineGit: aggregate(gitEngine),
          browserRequestActiveMs: unionMs(requests), browserGitActiveMs: unionMs(gitRequests), engineGitActiveMs: unionMs(gitEngine),
          beforeFirstBrowserRequestMs: requests[0]?.start ?? elapsedMs, incompleteRequests: requests.filter((s) => s.end === undefined).length
        }
        mkdirSync(directory, { recursive: true }); writeFileSync(join(directory, 'requests.json'), JSON.stringify(report, null, 2) + '\n')
        console.log('Git journey measurement:', JSON.stringify({ elapsedMs, loadStart: report.loadStart, loadEnd: report.loadEnd,
          stateRequests: gitRequests.filter((s) => s.tag === 'project.git.state').length, gitRequests: gitRequests.length,
          engineGitActiveMs: report.engineGitActiveMs, browserRequestActiveMs: report.browserRequestActiveMs, incompleteRequests: report.incompleteRequests }))
      } catch { /* Measurement must never replace the journey's own failure. */ }
    }
  }
}
