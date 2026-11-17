import { it } from "@effect/vitest"
import * as Effect from "effect/Effect"
import { afterEach, beforeEach, expect, vi } from "vitest"
import type { ClientSnapshot } from "@lgtm/contracts"
import { mapBoardRecords, workPreview } from "../../../studio/src/board.ts"
import { mapThread } from "../../../studio/src/chat.ts"
import { mapSnapshot } from "../../../studio/src/mapping.ts"
import { provideBoard } from "../board/wakeFixtures.ts"
import { appendSetup, eventOf, call, makeRig, ownerCredential, removeRig, success, type Rig } from "../support/handlerRig.ts"
import { decisionSetup } from "../support/projectDecisionFixtures.ts"
import { registerRebuildCheck } from "../support/rebuildCheck.ts"

let rig: Rig
beforeEach(() => { rig = makeRig(); vi.stubEnv("HOME", rig.home); vi.stubEnv("LGTM_HOME", rig.stateFolder) })
afterEach(() => { vi.unstubAllEnvs(); removeRig(rig) })

it.live("the roster's active task preview uses the real handler's redacted snapshot", () => provideBoard(rig, Effect.gen(function* () {
  yield* registerRebuildCheck()
  const p = yield* decisionSetup()
  const secret = "s25-synthetic-private-value"
  const task = success(yield* call(p.leadCredential, "project.task.create", { ...p.task, title: `Check ${secret}\n${"x".repeat(80)}` }))
  const snapshot = success(yield* call(ownerCredential, "client.snapshot", {})) as unknown as ClientSnapshot
  const records = mapBoardRecords(snapshot, mapSnapshot(snapshot, 1))
  const preview = records.agents.find((a) => a.id === p.workerId)!.doingNow!
  expect(preview).toContain("Check [REDACTED]")
  expect(preview).not.toContain(secret)
  expect(preview).not.toContain("\n")
  expect(Array.from(preview).length).toBeLessThanOrEqual(160)
  success(yield* call(p.leadCredential, "project.task.update", { projectId: p.projectId, taskId: task.taskId, state: "queue" }))
  yield* appendSetup([eventOf({ _tag: "ThreadItem", agentId: p.workerId, itemId: "s25-work-tool", replay: false,
    content: { _tag: "tool-call", callId: "s25-work-tool", title: `Inspect ${secret}` } })])
  const updated = success(yield* call(ownerCredential, "client.snapshot", {})) as unknown as ClientSnapshot
  const worker = mapBoardRecords(updated, mapSnapshot(updated, 1)).agents.find((a) => a.id === p.workerId)!
  const thread = success(yield* call(ownerCredential, "client.thread", { agentId: p.workerId, after: 0, limit: 100 }))
  expect(worker.doingNow).toBe("")
  expect(workPreview(worker, mapThread(thread.items, p.workerId).items)).toBe("Inspect [REDACTED]")
}), ["s25-synthetic-private-value"]))
