import type { Event } from '@lgtm/contracts'
import type { Snapshot } from '../commands/internal.ts'
import { own, projectKey, projectView } from './view.ts'

// Append explicit progress only to new command batches. Reducers keep the
// historical meaning of stored creation, task and agent events during replay.
export const withProjectProgress = (snapshot: Snapshot, events: ReadonlyArray<Event>): ReadonlyArray<Event> => {
  const view = projectView(snapshot), moves = new Map<string, 'plan' | 'build'>()
  const move = (projectId: string | undefined, stage: 'plan' | 'build') => {
    if (projectId === undefined) return
    const project = own(view.projects, projectId)
    if (project === undefined || !(stage === 'plan' ? project.stage === 'define' : project.stage === 'define' || project.stage === 'plan')) return
    if (moves.get(projectId) !== 'build') moves.set(projectId, stage)
  }
  for (const event of events) {
    switch (event._tag) {
      case 'project.SprintAdded': case 'project.TeamAdded': move(event.projectId, 'plan'); break
      case 'project.TaskAdded': move(event.projectId, event.state === 'queue' ? 'plan' : 'build'); break
      case 'project.TaskChanged':
        if (event.state !== undefined && event.state !== 'queue' && own(view.tasks, projectKey(event.projectId, event.taskId))?.state === 'queue') move(event.projectId, 'build')
        break
      case 'AgentStarted':
        if (event.parent._tag === 'agent') move(own(view.agentProjects, event.parent.agentId), 'build')
        break
      case 'StateChanged': {
        const id = own(view.agentProjects, event.agentId)
        if (event.state._tag === 'starting' && id !== undefined && own(view.projects, id)?.leadAgentId !== event.agentId) move(id, 'build')
        break
      }
    }
  }
  return [...events, ...Array.from(moves, ([projectId, stage]) => ({ _tag: 'project.StageChanged' as const, projectId: view.projects[projectId]!.projectId, stage }))]
}
