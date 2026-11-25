import * as Schema from "effect/Schema"
import { ProjectId, RepositoryId, WorkspaceId } from "./schema.ts"

// At least one registered identity is required by the engine. Combinations
// must agree; a project alone selects that project's own checkout.
export const MapSelection = {
  repoId: Schema.optionalKey(RepositoryId),
  projectId: Schema.optionalKey(ProjectId),
  worktreeId: Schema.optionalKey(WorkspaceId)
}
export type MapSelection = { readonly repoId?: typeof RepositoryId.Type; readonly projectId?: typeof ProjectId.Type; readonly worktreeId?: typeof WorkspaceId.Type }
