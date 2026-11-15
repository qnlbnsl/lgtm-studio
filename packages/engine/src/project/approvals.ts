import { projectApprovalEffects, projectApprovalOutcomes, type ProjectApprovalOutcome } from '@lgtm/contracts'
import type { Event, Project, QuestionMetadata } from '@lgtm/contracts'
import * as Effect from 'effect/Effect'
import { refused, type Input, type Snapshot } from '../commands/internal.ts'
import { own, projectView } from './view.ts'

export const endResultReason = 'The owner must approve the end result or skipping planning before work is planned; use ask with projectApproval set to "end-result" or "skip-planning".'
export const buildingReason = 'The owner must approve building before work starts; use ask with projectApproval set to "build".'
export const requireEndResult = (input: Input, project: Project) => input.actor._tag === 'owner' || (project.endResultApproval !== undefined || project.skipPlanningApproval !== undefined)
  ? Effect.void : Effect.fail(refused(endResultReason))
export const requireBuilding = (input: Input, project: Project) => input.actor._tag === 'owner' || project.buildApproval !== undefined
  ? Effect.void : Effect.fail(refused(buildingReason))
export const requireAgentBuilding = (input: Input, agentId: string) => {
  const view = projectView(input.snapshot), id = own(view.agentProjects, agentId)
  const project = id === undefined ? undefined : own(view.projects, id)
  return project === undefined ? Effect.void : requireBuilding(input, project)
}

// Approval questions use the ordinary owner card and answer delivery. The fixed
// acceptance choice avoids treating arbitrary prose, such as "not accepted", as consent.
export const approvalQuestion = (snapshot: Snapshot, agentId: string, metadata: QuestionMetadata) => Effect.gen(function* () {
  if (metadata.projectApproval === undefined) return {}
  const view = projectView(snapshot), projectId = own(view.agentProjects, agentId)
  const project = projectId === undefined ? undefined : own(view.projects, projectId)
  if (project === undefined || project.leadAgentId !== agentId || metadata.projectId !== undefined && metadata.projectId !== projectId
    || metadata.taskId !== undefined || metadata.sprintId !== undefined) return yield* refused('Only the project lead can request its owner’s project approval.')
  if (project.archiving || project.archivedAt !== undefined) return yield* refused('The project is archived or archiving.')
  if (metadata.projectApproval === 'build' && project.endResultApproval === undefined && project.skipPlanningApproval === undefined) return yield* refused(endResultReason)
  if (metadata.projectApproval === 'build' ? project.buildApproval !== undefined : project.endResultApproval !== undefined || project.skipPlanningApproval !== undefined) return yield* refused('The owner has already approved this step.')
  if (Object.values(snapshot.questions).some((q) => q.status === 'open' && q.projectId === projectId && (metadata.projectApproval === 'build' ? q.projectApproval === 'build' : q.projectApproval !== undefined && q.projectApproval !== 'build'))) return yield* refused('The owner already has a request for this approval.')
  return { projectApproval: metadata.projectApproval, projectApprovalEffect: projectApprovalEffects[metadata.projectApproval], proposedAnswer: 'Accepted' }
})

export const approvalAnswer = (input: Input, questionId: string, text: string): Effect.Effect<{ readonly events: ReadonlyArray<Event>; readonly outcome?: ProjectApprovalOutcome }, ReturnType<typeof refused>> => Effect.gen(function* () {
  const question = own(input.snapshot.questions, questionId)
  if (question?.projectApproval === undefined) return { events: [] }
  if (input.actor._tag !== 'owner') return yield* refused('Only the owner can decide a project approval.')
  const project = question.projectId === undefined ? undefined : own(projectView(input.snapshot).projects, question.projectId)
  if (project === undefined || project.leadAgentId !== question.agentId || project.archiving || project.archivedAt !== undefined) return yield* refused('The project approval is no longer available.')
  const scope = project.endResultApproval !== undefined || project.skipPlanningApproval !== undefined
  if (question.projectApproval === 'build' && !scope && text === 'Accepted') return { events: [], outcome: projectApprovalOutcomes.scopeWithdrawn }
  const approved = text === 'Accepted'
  const scopeAfter = scope || approved && question.projectApproval !== 'build'
  const buildingAfter = project.buildApproval !== undefined || approved && ['build', 'skip-planning-and-build'].includes(question.projectApproval)
  const recorded = question.projectApproval === 'build' ? buildingAfter
    : question.projectApproval === 'skip-planning-and-build' ? scopeAfter && buildingAfter : scopeAfter
  return { events: [{ _tag: 'project.ApprovalDecided', projectId: project.projectId, questionId: question.questionId as import("@lgtm/contracts").QuestionId,
    approval: question.projectApproval, approved }], outcome: recorded ? projectApprovalOutcomes.recorded : projectApprovalOutcomes.notRecorded }
})
