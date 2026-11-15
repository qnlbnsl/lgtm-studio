import { projectApprovalEffects, projectApprovalOutcomes, type ProjectApprovalOutcome } from '@lgtm/contracts'
import type { Event, Project, QuestionMetadata, LeadPermission } from '@lgtm/contracts'
import * as Effect from 'effect/Effect'
import { refused, type Input, type Snapshot } from '../commands/internal.ts'
import { own, projectView } from './view.ts'

export const permissionSetting = (project: Project, permission: LeadPermission) => (permission === 'planning' ? project.planningPermission : project.workPermission) ?? 'allowed'
export const permissionGranted = (project: Project, permission: LeadPermission) => permission === 'planning'
  ? project.endResultApproval !== undefined || project.skipPlanningApproval !== undefined : project.buildApproval !== undefined
const permissionName = (permission: LeadPermission) => permission === 'planning' ? 'Project planning' : 'Starting work'
export const permissionOffReason = (permission: LeadPermission) => `The owner has turned off ${permissionName(permission)}. This permission cannot be requested.`
export const endResultReason = 'Project planning requires permission; use ask with projectApproval set to "end-result" or "skip-planning".'
export const buildingReason = 'Starting work requires permission; use ask with projectApproval set to "build".'
const requirePermission = (input: Input, project: Project, permission: LeadPermission) => {
  const setting = permissionSetting(project, permission)
  return input.actor._tag === 'owner' || setting === 'allowed' || setting === 'ask' && permissionGranted(project, permission)
    ? Effect.void : Effect.fail(refused(setting === 'denied' ? permissionOffReason(permission) : permission === 'planning' ? endResultReason : buildingReason))
}
export const requireEndResult = (input: Input, project: Project) => requirePermission(input, project, 'planning')
export const requireBuilding = (input: Input, project: Project) => requirePermission(input, project, 'starting-work')
export const requireAgentBuilding = (input: Input, agentId: string) => {
  const view = projectView(input.snapshot), id = own(view.agentProjects, agentId)
  const project = id === undefined ? undefined : own(view.projects, id)
  return project === undefined ? Effect.void : requireBuilding(input, project)
}
const requestedPermissions = (approval: NonNullable<QuestionMetadata['projectApproval']>): ReadonlyArray<LeadPermission> => approval === 'build' ? ['starting-work']
  : approval === 'skip-planning-and-build' ? ['planning', 'starting-work'] : ['planning']
const changedAt = (project: Project, permission: LeadPermission) => (permission === 'planning' ? project.planningPermissionChangedAt : project.workPermissionChangedAt) ?? 0
const allowed = (project: Project, permission: LeadPermission) => permissionSetting(project, permission) === 'allowed'
  || permissionSetting(project, permission) === 'ask' && permissionGranted(project, permission)

// Fixed effects and exact acceptance protect the owner from misleading agent prose.
export const approvalQuestion = (snapshot: Snapshot, agentId: string, metadata: QuestionMetadata) => Effect.gen(function* () {
  if (metadata.projectApproval === undefined) return {}
  const view = projectView(snapshot), projectId = own(view.agentProjects, agentId)
  const project = projectId === undefined ? undefined : own(view.projects, projectId)
  if (project === undefined || project.leadAgentId !== agentId || metadata.projectId !== undefined && metadata.projectId !== projectId
    || metadata.taskId !== undefined || metadata.sprintId !== undefined) return yield* refused('Only the project lead can request its owner’s project approval.')
  if (project.archiving || project.archivedAt !== undefined) return yield* refused('The project is archived or archiving.')
  const permissions = requestedPermissions(metadata.projectApproval)
  for (const permission of permissions) if (permissionSetting(project, permission) === 'denied') return yield* refused(permissionOffReason(permission))
  const needed = permissions.filter((permission) => !allowed(project, permission))
  if (needed.length === 0) return yield* refused('This lead permission is already allowed or granted and needs no card.')
  if (Object.values(snapshot.questions).some((q) => q.status === 'open' && q.projectId === projectId && q.projectApproval !== undefined
    && requestedPermissions(q.projectApproval).some((permission) => needed.includes(permission) && q.sourceSequence > changedAt(project, permission)))) return yield* refused('The owner already has a request for this approval.')
  return { projectApproval: metadata.projectApproval, projectApprovalEffect: projectApprovalEffects[metadata.projectApproval], proposedAnswer: 'Accepted' }
})

export const approvalAnswer = (input: Input, questionId: string, text: string): Effect.Effect<{ readonly events: ReadonlyArray<Event>; readonly outcome?: ProjectApprovalOutcome }, ReturnType<typeof refused>> => Effect.gen(function* () {
  const question = own(input.snapshot.questions, questionId)
  if (question?.projectApproval === undefined) return { events: [] }
  if (input.actor._tag !== 'owner') return yield* refused('Only the owner can decide a project approval.')
  const project = question.projectId === undefined ? undefined : own(projectView(input.snapshot).projects, question.projectId)
  if (project === undefined || project.leadAgentId !== question.agentId || project.archiving || project.archivedAt !== undefined) return yield* refused('The project approval is no longer available.')
  const permissions = requestedPermissions(question.projectApproval)
  // A setting change or withdrawal invalidates earlier cards, including after toggling back to ask.
  const granted = permissions.filter((permission) => text === 'Accepted' && question.sourceSequence > changedAt(project, permission)
    && permissionSetting(project, permission) === 'ask')
  const recorded = permissions.every((permission) => allowed(project, permission) || granted.includes(permission))
  return { events: [{ _tag: 'project.PermissionDecided', projectId: project.projectId, questionId: question.questionId as import('@lgtm/contracts').QuestionId,
    granted }], outcome: recorded ? projectApprovalOutcomes.recorded : projectApprovalOutcomes.notRecorded }
})
