import { expect, it } from "vitest"
import { automaticGapMs, createGitController, dirtyGitRecord, fallbackMs, finishGitRecord, forceGitRecord,
  gitRefreshRecord, maximumAutomaticRequests, nextAutomaticRead } from "../src/gitRefresh.ts"

it("new identities start due with independent summary and review dirtiness", () => {
  expect(gitRefreshRecord("repo/folder/branch", 42)).toMatchObject({ identity: "repo/folder/branch", phase: "due",
    summaryDirty: 1, reviewDirty: 1, epoch: 0, failures: 0, nextAttempt: 42, lastSuccess: null, lastError: null })
})

it("the pure selector enforces the gap and the maximum pass width", () => {
  const c = createGitController()
  for (let i = 0; i < 10; i++) c.records.set(String(i), gitRefreshRecord(String(i), 0))
  expect(automaticGapMs).toBeGreaterThanOrEqual(2_000)
  expect((Math.ceil(60_000 / automaticGapMs) + 1) * maximumAutomaticRequests).toBeLessThanOrEqual(120)
  let previous = -automaticGapMs
  for (let now = 0; now <= 180_000; now += 50) {
    const before = structuredClone(c), next = nextAutomaticRead(now, c)!
    expect(c).toEqual(before)
    expect(next.at).toBeGreaterThanOrEqual(previous + automaticGapMs)
    if (next.at > now) continue
    previous = now; c.lastStart = now
    // Even a zero-duration pass that stays dirty cannot get an earlier start.
    c.records.get(next.id)!.nextAttempt = now
  }
})

it("continuous fifty-millisecond events preserve an already due deadline", () => {
  const c = createGitController(), record = gitRefreshRecord("one", 0)
  c.records.set("one", record); record.summaryDirty = record.reviewDirty = 0
  finishGitRecord(record, 0, null)
  dirtyGitRecord(record, "summary", 1_000)
  for (let now = 1_050; now < 2_000; now += 50) dirtyGitRecord(record, "summary", now)
  expect(nextAutomaticRead(1_050, c)).toEqual({ id: "one", at: 1_100 })
})

it("retry deadlines follow the ladder and healthy worktrees continue", () => {
  const c = createGitController(), failed = gitRefreshRecord("bad", 0), healthy = gitRefreshRecord("good", 0)
  c.records.set("bad", failed); c.records.set("good", healthy)
  let now = 0
  for (const delay of [1_000, 2_000, 4_000, 8_000, 16_000, 32_000, 60_000, 60_000]) {
    finishGitRecord(failed, now, "failed")
    expect(failed.nextAttempt).toBe(now + delay)
    for (let n = 0; n < 10; n++) dirtyGitRecord(failed, "summary", now + n * 50)
    expect(failed.nextAttempt).toBe(now + delay)
    expect(nextAutomaticRead(now, c)?.id).toBe("good")
    now += delay
  }
})

it("a successful idle record becomes due at the sixty-second fallback", () => {
  const c = createGitController(), record = gitRefreshRecord("one", 0)
  c.records.set("one", record); record.summaryDirty = record.reviewDirty = 0
  finishGitRecord(record, 100, null)
  expect(nextAutomaticRead(101, c)).toEqual({ id: "one", at: 100 + fallbackMs })
})

it("forced refresh clears backoff without changing the automatic gap", () => {
  const c = createGitController(500), record = gitRefreshRecord("one", 0)
  c.records.set("one", record); finishGitRecord(record, 500, "failed")
  forceGitRecord(record, 600)
  expect(record).toMatchObject({ phase: "due", failures: 0, nextAttempt: 600, epoch: 1, lastError: "failed" })
  expect(nextAutomaticRead(600, c)).toEqual({ id: "one", at: 500 + automaticGapMs })
})

it("selection breaks due-time ties without starving an older worktree", () => {
  const c = createGitController(); c.selected = "selected"
  c.records.set("old", gitRefreshRecord("old", 0)); c.records.set("selected", gitRefreshRecord("selected", 0))
  expect(nextAutomaticRead(0, c)?.id).toBe("selected")
  c.records.get("selected")!.nextAttempt = 1
  expect(nextAutomaticRead(0, c)?.id).toBe("old")
})

it("a held pass, owner action, or hidden screen has no automatic selection", () => {
  const c = createGitController(); c.records.set("one", gitRefreshRecord("one", 0))
  c.reading = "one"; expect(nextAutomaticRead(0, c)).toBeUndefined()
  c.reading = null; c.ownerBusy = true; expect(nextAutomaticRead(0, c)).toBeUndefined()
  c.ownerBusy = false; c.enabled = false; expect(nextAutomaticRead(0, c)).toBeUndefined()
})
