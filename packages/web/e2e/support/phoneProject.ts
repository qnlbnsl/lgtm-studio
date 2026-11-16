import * as Effect from "effect/Effect"
import { appendSetup, eventOf } from "../../../engine/test/support/handlerRig.ts"
import { completedSetup } from "./completedSetup.ts"

// Attach the launched agent before it raises its request. The production alert
// mapper and the website must agree on its project, without a seeded substitute.
export const phoneProject = (leadAgentId: string, path: string) => Effect.gen(function* () {
  const projectId = "phone-project", repoId = "phone-repository", worktreeId = "phone-worktree"
  yield* appendSetup([
    eventOf({ _tag: "project.RepositoryAdded", repoId, name: "Phone repository", path }),
    eventOf({ _tag: "project.RepositoryLocated", repoId, machineId: "local", path }),
    eventOf({ _tag: "project.WorktreeCreated", repoId, worktreeId, branch: "phone", path, machineId: "local" }),
    eventOf({ _tag: "project.Created", projectId, repoId, worktreeId, leadAgentId,
      name: "Phone project", goal: "Answer the phone notification", machineId: "local" }),
  ])
  yield* completedSetup(projectId)
  return projectId
})
