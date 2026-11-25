import { QuestionId, RequestId } from "../ids.ts"
import * as Schema from "effect/Schema"
import { GitAttempt, GitEvidence, GitMainReview } from "./git.ts"
import { CreationFolderIdentity, FileSaveNotice, LeadPermission, LeadPermissionSetting, OwnerTaskNotification, Project, ProjectCreation, ProjectDeletion, ProjectId, ProjectName, ProjectSettings, WorktreePreparation, ProjectStage, ProjectText, Repository, RepositoryLocation, Sprint, SprintId, Task, TaskId, TaskState, Team, Workspace } from "./schema.ts"

import { ProjectMockEvent } from "./mocks.ts"
import { ProjectDesignEvent } from "./design.ts"

export const ProjectEvent = Schema.Union([
  Schema.TaggedStruct("project.GitAttempted", { attempt: GitAttempt }),
  Schema.TaggedStruct("project.GitFinished", { caller: GitAttempt.fields.caller, operationRequestId: GitAttempt.fields.requestId,
    outcome: Schema.optionalKey(Schema.Literal("uncertain")),
    refusal: Schema.optionalKey(Schema.Literals(["conflicts", "push_rejected", "pull_diverged", "network_failed", "authentication_required"])) }),
  Schema.TaggedStruct("project.GitEvidenceReported", { report: GitEvidence }),
  Schema.TaggedStruct("project.MainReviewOpened", { review: GitMainReview }),
  Schema.TaggedStruct("project.MainReviewDecided", { review: GitMainReview }),
  ...ProjectMockEvent.members,
  ProjectDesignEvent,
  Schema.TaggedStruct("project.SettingsChanged", { settings: ProjectSettings }),
  Schema.TaggedStruct("project.RepositoryAdded", Repository.fields),
  Schema.TaggedStruct("project.RepositoryPreparationChanged", { repoId: Repository.fields.repoId, preparation: WorktreePreparation }),
  Schema.TaggedStruct("project.RepositoryBaseChanged", { repoId: Repository.fields.repoId, baseBranch: ProjectName }),
  Schema.TaggedStruct("project.RepositoryLocated", RepositoryLocation.fields),
  Schema.TaggedStruct("project.WorktreeCreated", Workspace.fields),
  Schema.TaggedStruct("project.Created", Project.fields),
  Schema.TaggedStruct("project.CreationPlanned", { creation: ProjectCreation }),
  Schema.TaggedStruct("project.CreationClaimed", { creationRequestId: ProjectCreation.fields.requestId, kind: Schema.Literals(["repository", "folder"]), identity: CreationFolderIdentity }),
  Schema.TaggedStruct("project.CreationFinished", { creationRequestId: ProjectCreation.fields.requestId, detail: Schema.optionalKey(ProjectText) }),
  Schema.TaggedStruct("project.GoalChanged", { projectId: ProjectId, goal: ProjectText }),
  Schema.TaggedStruct("project.ArchiveStarted", { projectId: ProjectId }),
  Schema.TaggedStruct("project.Archived", { projectId: ProjectId }),
  Schema.TaggedStruct("project.Restored", { projectId: ProjectId }),
  Schema.TaggedStruct("project.Deleted", ProjectDeletion.fields),
  Schema.TaggedStruct("project.Placed", { projectId: ProjectId, machineId: ProjectName, worktreeId: Workspace.fields.worktreeId }),
  Schema.TaggedStruct("project.PreferencesChanged", { projectId: ProjectId, shell: ProjectName }),
  Schema.TaggedStruct("project.Renamed", { projectId: ProjectId, name: ProjectName }),
  Schema.TaggedStruct("project.PermissionChanged", { projectId: ProjectId, permission: LeadPermission, setting: LeadPermissionSetting }),
  Schema.TaggedStruct("project.PermissionDecided", { projectId: ProjectId, questionId: QuestionId, granted: Schema.Array(LeadPermission).check(Schema.isMaxLength(2)) }),
  Schema.TaggedStruct("project.PermissionWithdrawn", { projectId: ProjectId, permission: LeadPermission }),
  Schema.TaggedStruct("project.ApprovalGranted", { projectId: ProjectId, approval: Schema.Literals(["end-result", "build"]), requestId: RequestId }),
  Schema.TaggedStruct("project.ApprovalWithdrawn", { projectId: ProjectId, approval: Schema.Literals(["end-result", "build"]) }),
  Schema.TaggedStruct("project.ApprovalDecided", { projectId: ProjectId, questionId: QuestionId, approval: Schema.Literals(["end-result", "build", "skip-planning", "skip-planning-and-build"]), approved: Schema.Boolean }),
  Schema.TaggedStruct("project.TeamDeleted", { projectId: ProjectId, teamId: Team.fields.teamId }),
  Schema.TaggedStruct("project.TaskDeleted", { projectId: ProjectId, taskId: TaskId }),
  Schema.TaggedStruct("project.StageChanged", { projectId: ProjectId, stage: ProjectStage }),
  Schema.TaggedStruct("project.TeamAdded", Team.fields),
  Schema.TaggedStruct("project.SprintAdded", Sprint.fields),
  Schema.TaggedStruct("project.SprintChanged", { projectId: ProjectId, sprintId: SprintId,
    name: Schema.optionalKey(ProjectName), goal: Schema.optionalKey(ProjectText), phase: Schema.optionalKey(Sprint.fields.phase),
    owner: Schema.optionalKey(Sprint.fields.owner), needs: Schema.optionalKey(Sprint.fields.needs) }),
  Schema.TaggedStruct("project.SprintCompleted", { projectId: ProjectId, sprintId: SprintId, completion: ProjectName, evidence: ProjectText }),
  Schema.TaggedStruct("project.TaskAdded", Task.fields),
  Schema.TaggedStruct("project.OwnerTaskNotificationPlanned", OwnerTaskNotification.fields),
  Schema.TaggedStruct("project.FileSaveNoticePlanned", FileSaveNotice.fields),
  Schema.TaggedStruct("project.TaskChanged", { projectId: ProjectId, taskId: TaskId,
    state: Schema.optionalKey(TaskState), title: Schema.optionalKey(ProjectName), owner: Schema.optionalKey(Task.fields.owner),
    teamId: Schema.optionalKey(Task.fields.teamId), sprintId: Schema.optionalKey(SprintId), worktreeId: Schema.optionalKey(Task.fields.worktreeId),
    note: Schema.optionalKey(ProjectText) }),
  Schema.TaggedStruct("project.TaskNoted", { projectId: ProjectId, taskId: TaskId, title: ProjectName, detail: ProjectText })
])
export type ProjectEvent = typeof ProjectEvent.Type
export const projectVisibility = Object.fromEntries(ProjectEvent.members.map((event) => [event.fields._tag.schema.literal, "owner-only"])) as Record<ProjectEvent["_tag"], "owner-only">

export const projectEventTags = ProjectEvent.members.map((event) => event.fields._tag.schema.literal)
