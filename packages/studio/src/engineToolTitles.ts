// Explicit inventory: the test compares these names with the agent MCP surface.
export const engineToolTitles: Readonly<Record<string, string>> = {
  start: 'Started an agent', prompt: 'Sent a prompt', answer: 'Answered a request',
  'pass-up': 'Passed up a request', ask: 'Asked a question', wait: 'Waited for agents',
  'cancel-reply': 'Cancelled a reply', stop: 'Stopped an agent', resume: 'Resumed an agent',
  release: 'Released an agent', compact: 'Compacted context', group: 'Updated group membership',
  'client.snapshot': 'Read the team snapshot', 'client.roster': 'Read the roster',
  'client.thread': 'Read a thread', 'client.event': 'Read an event',
  'board.open': 'Opened a channel', 'board.members': 'Updated channel members',
  'board.post': 'Posted to the board', 'board.close': 'Closed a post',
  'board.acknowledge': 'Acknowledged a post', 'board.read': 'Read the board',
  'board.channels': 'Listed channels', 'board.status': 'Read board status',
  'project.context.search': 'Searched project context',
  'project.design.record': 'Recorded the design', 'project.design.read': 'Read the design',
  'project.mocks.record': 'Recorded the mocks', 'project.mocks.read': 'Read the mocks',
  'project.mocks.screen.read': 'Read screen feedback', 'project.mocks.reviews.read': 'Read mock reviews',
  'project.mocks.artifact': 'Read a mock artifact', 'project.changes': 'Read changed files',
  'project.diff': 'Read a diff', 'project.history': 'Read commit history',
  'project.commit.files': 'Read commit files', 'project.git.state': 'Read Git state',
  'project.review': 'Read review evidence', 'project.review.report': 'Recorded a review report',
  'project.review.status': 'Read review status', 'project.conflict': 'Read conflicts',
  'project.stage.set': 'Updated the project stage', 'project.requests': 'Read project requests',
  'project.read': 'Read the project', 'project.rename': 'Renamed the project',
  'project.team.delete': 'Deleted a team', 'project.task.delete': 'Deleted a task',
  'project.team.create': 'Created team', 'project.sprint.create': 'Created sprint',
  'project.sprint.update': 'Updated sprint', 'project.sprint.complete': 'Completed sprint',
  'project.task.create': 'Created task', 'project.task.update': 'Updated task',
  'project.task.note': 'Added a task note', 'project.task.history': 'Read task history',
  'roles.orient': 'Read role guidance', 'roles.handoff': 'Read a handoff',
}
const aliases = new Map(Object.entries(engineToolTitles).map(([name, title]) => [name.replace(/[. -]/g, '_'), title]))
export function engineToolTitle(title: string, holder?: 'owner' | 'agent'): string | undefined {
  const name = /^(?:mcp__lgtm__|mcp\.lgtm\.)(.+)$/.exec(title)?.[1]
  if (name === 'ask' && holder) return holder === 'owner' ? 'Asked the owner' : 'Asked the lead'
  return name === undefined ? undefined : aliases.get(name.replace(/[. -]/g, '_'))
}
