import { EventEnvelope, type ProjectView } from "@lgtm/contracts"
import * as Schema from "effect/Schema"
import { expect, it } from "vitest"
import { apply, emptyViews } from "../../src/views/apply.ts"
import { projectKey, projectView } from "../../src/project/view.ts"
import { registerRebuildCheck } from "../support/rebuildCheck.ts"
registerRebuildCheck({ noLog: true })

const envelope = (sequence: number, event: unknown, occurredAt = 1000 + sequence) => Schema.decodeUnknownSync(EventEnvelope)({
  sequence, eventId: `event-${sequence}`, occurredAt, caller: { _tag: "owner" }, event
})
it("uses recorded times for every story step and keeps project-scoped IDs and prototype keys distinct", () => {
  const events = [
    envelope(1, { _tag: "project.Created", projectId: "__proto__", name: "One", goal: "", repoId: "r", worktreeId: "w", machineId: "local", leadAgentId: "constructor" }),
    envelope(2, { _tag: "project.TeamAdded", projectId: "__proto__", teamId: "build", name: "One team", worktreeId: "w" }),
    envelope(3, { _tag: "project.TeamAdded", projectId: "other", teamId: "build", name: "Other team", worktreeId: "v" }),
    envelope(4, { _tag: "project.TaskAdded", projectId: "__proto__", taskId: "toString", title: "A task", owner: "constructor", teamId: "build", sprintId: "sp", state: "queue", worktreeId: "w",
      story: { summary: "", done: [], activity: [{ title: "Earlier", detail: "Explicit time", at: 42 }, { title: "Assigned", detail: "Uses the event time" }] } }, 2000),
    envelope(5, { _tag: "project.TaskChanged", projectId: "__proto__", taskId: "toString", state: "active", note: "Working" }, 3000),
    envelope(6, { _tag: "project.TaskNoted", projectId: "__proto__", taskId: "toString", title: "Handoff", detail: "Review it" }, 4000),
    // A missing opening event must not make replay throw.
    envelope(7, { _tag: "project.TaskNoted", projectId: "missing", taskId: "missing", title: "Lost", detail: "" })
  ]
  const state = events.reduce(apply, emptyViews())
  const replay = events.map((row) => Schema.decodeUnknownSync(EventEnvelope)(JSON.parse(JSON.stringify(row)))).reduce(apply, emptyViews())
  expect(replay).toEqual(state)
  const view: ProjectView = projectView(state)
  expect(Object.keys(view.projects)).toEqual(["__proto__"])
  expect(view.teams[projectKey("__proto__", "build")]?.name).toBe("One team")
  expect(view.teams[projectKey("other", "build")]?.name).toBe("Other team")
  expect(view.tasks[projectKey("__proto__", "toString")]?.history.map((entry) => entry.at)).toEqual([2000, 42, 2000, 3000, 4000])
  expect(view.tasks[projectKey("__proto__", "toString")]?.story?.activity.every((step) => step.at !== undefined)).toBe(true)
})

it('projects workflow times from event envelopes, including approvals, replay and repeated stages', () => {
  const events = [
    envelope(1, { _tag: 'project.Created', projectId: 'p', name: 'One', goal: '', repoId: 'r', worktreeId: 'w', machineId: 'local', leadAgentId: 'lead', stage: 'define' }, 1000),
    envelope(2, { _tag: 'project.ApprovalGranted', projectId: 'p', requestId: 'approve', approval: 'end-result' }, 2000),
    envelope(3, { _tag: 'project.StageChanged', projectId: 'p', stage: 'plan' }, 3000),
    envelope(4, { _tag: 'project.StageChanged', projectId: 'p', stage: 'build' }, 4000),
    envelope(5, { _tag: 'project.StageChanged', projectId: 'p', stage: 'plan' }, 5000)
  ]
  const view = projectView(events.reduce(apply, emptyViews()))
  expect(view.projects.p).toMatchObject({ createdAt: 1000, stage: 'plan', phaseTimes: [
    { phase: 'define', start: 1000, end: 2000 },
    { phase: 'build', start: 4000, end: 5000 }, { phase: 'plan', start: 5000 }
  ] })
  expect(events.map(row => Schema.decodeUnknownSync(EventEnvelope)(JSON.parse(JSON.stringify(row)))).reduce(apply, emptyViews())).toEqual(events.reduce(apply, emptyViews()))
})
