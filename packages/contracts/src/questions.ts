import * as Schema from "effect/Schema"
import { ProjectId, SprintId, TaskId } from "./project/schema.ts"

// Project identity and completion checks belong to the project piece.
// Optional fields keep existing questions and stored events readable.
export const ProjectRequestLinkFields = {
  projectId: Schema.optionalKey(ProjectId),
  taskId: Schema.optionalKey(TaskId),
  sprintId: Schema.optionalKey(SprintId)
}
export type ProjectRequestLink = { readonly projectId?: typeof ProjectId.Type; readonly taskId?: typeof TaskId.Type; readonly sprintId?: typeof SprintId.Type }
export const projectApprovalEffects = {
  "end-result": 'Accepting grants Project planning. The lead can create sprints, teams and tasks.',
  "build": 'Accepting grants Starting work. The lead can start workers and move tasks out of the queue.',
  "skip-planning": 'Accepting grants Project planning. The lead can create sprints, teams and tasks.',
  "skip-planning-and-build": 'Accepting grants Project planning and Starting work. The lead can create sprints, teams and tasks. The lead can start workers and move tasks out of the queue.'
} as const
export const projectApprovalOutcomes = {
  recorded: "Approval recorded.",
  notRecorded: "Approval not recorded. Ask again if you still need it."
} as const
// Like the effect sentence, the engine-written outcome remains redactable text.
export const ProjectApprovalOutcome = Schema.String
export type ProjectApprovalOutcome = typeof ProjectApprovalOutcome.Type
export const QuestionMetadataFields = {
  ...ProjectRequestLinkFields,
  projectApproval: Schema.optionalKey(Schema.Literals(["end-result", "build", "skip-planning", "skip-planning-and-build"])),
  projectApprovalEffect: Schema.optionalKey(Schema.String),
  title: Schema.optionalKey(Schema.String),
  recommendation: Schema.optionalKey(Schema.String),
  proposedAnswer: Schema.optionalKey(Schema.String)
}
export const QuestionMetadata = Schema.Struct(QuestionMetadataFields)
export type QuestionMetadata = typeof QuestionMetadata.Type
