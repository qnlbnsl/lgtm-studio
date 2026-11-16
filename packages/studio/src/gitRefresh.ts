// The only policy that chooses an automatic Git read. It has no clock or I/O.
export const automaticGapMs = 3_000
export const fallbackMs = 60_000
export const maximumAutomaticRequests = 5
export type GitPhase = "idle" | "due" | "reading" | "backing off"
export interface GitRefreshRecord {
  identity: string
  phase: GitPhase
  summaryDirty: number
  reviewDirty: number
  epoch: number
  failures: number
  nextAttempt: number
  lastSuccess: number | null
  lastError: string | null
  lastToast: number | null
  // The live reader owns these controllers. They never decide admission.
  summaryAbort?: AbortController | undefined
  detailAbort?: AbortController | undefined
}
export interface GitRefreshController {
  records: Map<string, GitRefreshRecord>
  selected?: string
  lastStart: number | null
  readonly reading: string | null
  enabled: boolean
  ownerBusy: boolean
}
export function createGitController(lastStart: number | null = null): GitRefreshController {
  return { records: new Map(), lastStart,
    get reading() { return [...this.records].find(([, record]) => record.phase === "reading")?.[0] ?? null },
    enabled: true, ownerBusy: false }
}
export function gitRefreshRecord(identity: string, now: number, epoch = 0): GitRefreshRecord {
  return { identity, phase: "due", summaryDirty: 1, reviewDirty: 1, epoch, failures: 0,
    nextAttempt: now, lastSuccess: null, lastError: null, lastToast: null }
}
export function dirtyGitRecord(record: GitRefreshRecord, group: "summary" | "review", now: number): void {
  if (group === "summary") record.summaryDirty++
  else record.reviewDirty++
  if (record.phase === "idle") { record.phase = "due"; record.nextAttempt = Math.min(record.nextAttempt, now + 100) }
}
export function finishGitRecord(record: GitRefreshRecord, now: number, error: string | null, summarySucceeded = error === null): void {
  if (summarySucceeded) record.lastSuccess = now
  record.lastError = error
  if (error !== null) {
    record.failures++
    record.phase = "backing off"
    record.nextAttempt = now + Math.min(1_000 * 2 ** Math.min(record.failures - 1, 6), fallbackMs)
  } else {
    record.failures = 0
    record.phase = record.summaryDirty || record.reviewDirty ? "due" : "idle"
    record.nextAttempt = now + (record.phase === "idle" ? fallbackMs : 100)
  }
}
export function nextAutomaticRead(now: number, controller: GitRefreshController): { id: string; at: number } | undefined {
  if (!controller.enabled || controller.ownerBusy || controller.reading !== null) return undefined
  let choice: { id: string; due: number } | undefined
  for (const [id, record] of controller.records) {
    if (record.phase === "reading") continue
    const fallback = record.lastSuccess === null ? record.nextAttempt : record.lastSuccess + fallbackMs
    const due = record.phase === "idle" ? fallback : record.summaryDirty === 0 ? Math.min(record.nextAttempt, fallback) : record.nextAttempt
    if (!choice || due < choice.due || due === choice.due && id === controller.selected) choice = { id, due }
  }
  if (!choice) return undefined
  return { id: choice.id, at: Math.max(now, choice.due, controller.lastStart === null ? now : controller.lastStart + automaticGapMs) }
}
