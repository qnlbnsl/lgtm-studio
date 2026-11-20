// Pure translations for projects, sprints, task details, the board and first run.
import type { ClientSnapshot, BoardPost, BoardAuthor, TaskHistoryEntry, AgentChoices, ContextUsage, SetupState, Discovery, ProjectSettings } from "@lgtm/contracts"
import type { StoreSprint, EngineRecords, StoreAgent } from "./mapping.ts"
type Section = NonNullable<ClientSnapshot["project"]>
type Channel = { displayName?: string; archived?: boolean; channelId: string; name: string; kind: string; members: ReadonlyArray<BoardAuthor>; group?: string; anchorAgentId?: string }
// Task titles have already passed the engine's redactor. Bound by code points,
// so shortening a preview cannot split a Unicode character.
export function doingNow(snapshot: ClientSnapshot, agentId: string, projectId: string): string {
  if (snapshot.agents.find((a) => a.agentId === agentId)?.state._tag !== "working") return ""
  const title = snapshot.project?.tasks.find((t) => t.projectId === projectId && t.owner === agentId && t.state === "active")?.title ?? ""
  return shortWorkText(title)
}
function shortWorkText(text: string): string {
  const points = Array.from(text.replace(/[\u061c\u200e\u200f\u202a-\u202e\u2066-\u2069]/gu, "").replace(/[\u0000-\u001f\u007f-\u009f]/gu, " ").replace(/\s+/gu, " ").trim())
  return points.length > 160 ? points.slice(0, 159).join("") + "…" : points.join("")
}
// The roster already reads these redacted, mapped thread rows for output times.
// Replies and raw tool details are not descriptions of the current work.
export function workPreview(agent: Pick<StoreAgent, "state" | "doingNow" | "activity">, thread: ReadonlyArray<Record<string, unknown>>): string {
  if (agent.state !== "working") return agent.activity
  if (agent.doingNow) return agent.doingNow
  for (let i = thread.length - 1; i >= 0; i--) {
    const row = thread[i]!
    const text = row.type === "tools" ? row.summary : row.type === "user" ? row.text : undefined
    const preview = typeof text === "string" ? shortWorkText(text) : ""
    if (preview) return preview
  }
  return agent.activity
}
export const sprintColumn = (sprint: Section["sprints"][number], tasks: Section["tasks"]): StoreSprint["column"] =>
  sprint.completion !== undefined ? "Done" : tasks.some((t) => t.sprintId === sprint.sprintId && ["active", "blocked"].includes(t.state)) ? "In progress"
    : tasks.some((t) => t.sprintId === sprint.sprintId && t.state === "review") ? "Review" : "Planned"

export function mapBoardRecords(snapshot: ClientSnapshot, records: EngineRecords): EngineRecords {
  const section = snapshot.project
  const stalls = mapStalls(snapshot.roles)
  return {
    ...records,
    repos: records.repos.map((r) => ({ ...r, main: "" })),
    projects: records.projects.map((p) => ({ ...p, prefix: "", tests: "", agentReview: "",
      archiving: section?.projects.find((r) => r.projectId === p.id)?.archiving ?? false,
      worktreeId: section?.projects.find((r) => r.projectId === p.id)?.worktreeId ?? "" })),
    teams: records.teams.map((t) => ({ ...t, channelId: section?.teams.find((r) => r.teamId === t.id)?.channelId ?? null,
      worktreeId: section?.teams.find((r) => r.teamId === t.id)?.worktreeId ?? "" })),
    sprints: records.sprints.map((s) => {
      const row = section!.sprints.find((r) => r.sprintId === s.id)!
      return { ...s, column: sprintColumn(row, section!.tasks), owner: row.owner,
        evidence: row.evidence === undefined && row.completion === undefined ? null : { ...(row.completion === undefined ? {} : { completion: row.completion }), ...(row.evidence === undefined ? {} : { evidence: row.evidence }) } }
    }),
    tasks: records.tasks.map((t) => ({ ...t, label: section?.tasks.find((r) => r.taskId === t.id)?.label ?? "",
      worktreeId: section?.tasks.find((r) => r.taskId === t.id)?.worktreeId ?? "",
      blockedBy: records.requests.find((r) => r.task === t.id)?.kind ?? null })),
    requests: records.requests.map((r) => ({ ...r, sprint: [...snapshot.requests, ...snapshot.questions].find((row) => row.id === r.id)?.display?.sprintId ?? null })),
    agents: records.agents.map((a) => ({ ...a, doingNow: doingNow(snapshot, a.id, a.project), stalled: stalls.agents.includes(a.id), activity: snapshot.agents.find((row) => row.agentId === a.id)?.displayedState ?? "", name: a.role === "project-lead" ? "Project lead" : a.role === "team-lead" ? "Team lead" : a.name, task: section?.tasks.find((t) => t.owner === a.id && t.state !== "done")?.label ?? null }))
  }
}
export const authorId = (author: BoardAuthor): string => author._tag === "agent" ? author.agentId : author._tag === "owner" ? "owner" : `service:${author.service}`
export function mapChannels(rows: ReadonlyArray<Channel>, records: Pick<EngineRecords, "projects" | "teams" | "agents">) {
  return rows.flatMap((row) => {
    const team = records.teams.find((t) => (t as typeof t & { channelId?: string }).channelId === row.channelId)
    const members = row.members.map(authorId)
    if (row.group !== undefined) for (const a of records.agents) if (a.team === team?.id && !members.includes(a.id)) members.push(a.id)
    const anchorProject = records.projects.find((project) => project.lead === row.anchorAgentId)
    const projects = team ? [team.project] : anchorProject ? [anchorProject.id] : [...new Set(records.agents.filter((a) => members.includes(a.id) || a.id === row.anchorAgentId).map((a) => a.project))]
    return projects.map((project) => ({ id: row.channelId, project, kind: row.kind === "direct" ? "dm" : team ? "team" : "plain",
      name: team?.name ?? row.displayName ?? row.name, archived: row.archived === true, team: team?.id ?? null, members }))
  })
}
export function mapPost(post: BoardPost, occurredAt?: number) {
  return { id: post.postId, channel: post.channelId, from: authorId(post.author), text: post.text, ts: occurredAt ?? null,
    sequence: post.sequence, kind: null, action: post.reply === "none" ? "fyi" : "needs-reply", to: post.to.map(authorId),
    replyTo: post.replyTo ?? null, closed: post.status === "closed", outcome: post.outcome ?? null, reply: post.reply,
    dispositions: post.dispositions, acknowledged: post.acknowledged.map(authorId) }
}
export function mapUnread(board: unknown): Record<string, number> {
  if (!board || typeof board !== "object" || !("unread" in board) || !Array.isArray(board.unread)) return {}
  return Object.fromEntries(board.unread.filter((r) => r.member?._tag === "owner").map((r) => [r.channelId, r.count]))
}
export function mapTaskDetails(history: ReadonlyArray<TaskHistoryEntry>, changes: { files: ReadonlyArray<{ path: string }> }, workspace?: Section["workspaces"][number]) {
  return { files: changes.files.map((f) => f.path), history: history.map((h) => ({ ts: h.at, who: null, text: h.detail ? `${h.title}: ${h.detail}` : h.title, state: h.state })),
    branch: workspace?.branch ?? "", folder: workspace?.path ?? "" }
}
export function mapAgentDetails(choices: AgentChoices, context?: ContextUsage) {
  return { ...(choices.models.reason ? { modelReason: choices.models.reason } : {}), ...(choices.efforts.reason ? { effortReason: choices.efforts.reason } : {}), models: choices.models.options.map((o) => ({ id: o.id, label: o.label ?? o.id })), efforts: choices.efforts.options.map((o) => ({ id: o.id, label: o.label ?? o.id })),
    context: context?._tag === "available" ? context.percent : null }
}
export function mapSetup(setup: SetupState, discovery: typeof Discovery.Type, settings?: ProjectSettings) {
  return { step: ["welcome", "found", "project"].indexOf(setup.step), completed: setup.completedAt !== undefined, projectId: setup.projectId ?? null,
    repositories: discovery.repositories, harnesses: discovery.harnesses, lead: discovery.readyDefaults ?? settings?.lead ?? null,
    workspace: settings?.workspaceFolder ?? "", shell: settings?.shell ?? "", ownerName: settings?.displayName ?? discovery.ownerNameSuggestion ?? "",
    available: discovery.state === "available", reason: discovery.reason ?? "" }
}
export function projectPayload(input: { repo?: string; newRepo?: string; goal: string; lead?: NonNullable<ProjectSettings["lead"]> }, name: string) {
  return { name, goal: input.goal, machineId: "local", ...(input.lead ? { lead: input.lead } : {}), repository: input.newRepo ? { _tag: "new", name: input.newRepo } : { _tag: "existing", repoId: input.repo } }
}
export function postPayload(input: { channel: string; text: string; action: string; to: string[]; replyTo?: string }, members: ReadonlyArray<string>) {
  const to = input.to.length ? input.to : input.action === "needs-reply" ? members.filter((id) => id !== "owner" && !id.startsWith("service:")) : []
  return { channelId: input.channel, text: input.text.trim(), reply: input.action === "needs-reply" ? "any" : "none",
    to: to.map((id) => id === "owner" ? { _tag: "owner" } : { _tag: "agent", agentId: id }), ...(input.replyTo ? { replyTo: input.replyTo } : {}) }
}

export function snapshotChannels(snapshot: ClientSnapshot, records: EngineRecords) {
  const section = snapshot.board
  if (!section || typeof section !== "object" || !("channels" in section) || !Array.isArray(section.channels)) return []
  return mapChannels(section.channels as Channel[], records)
}

// The roles section retains the latest action of each kind for each current agent.
// Compare durable positions rather than inferring a stall from elapsed browser time.
export function mapStalls(roles: unknown) {
  const agents: string[] = [], posts: string[] = []
  if (!roles || typeof roles !== "object" || !("agents" in roles)) return { agents, posts }
  const rows = roles.agents as { items?: ReadonlyArray<{ agentId: string; actions: ReadonlyArray<{ _tag: string; sequence: number; stalledPostId?: string }> }> }
  for (const row of rows?.items ?? []) {
    const seen = row.actions.filter((a) => a._tag === "roles.StallSeen").sort((a, b) => b.sequence - a.sequence)[0]
    const ended = row.actions.filter((a) => a._tag === "roles.StallEnded").sort((a, b) => b.sequence - a.sequence)[0]
    if (seen && (!ended || seen.sequence > ended.sequence)) { agents.push(row.agentId); if (seen.stalledPostId) posts.push(seen.stalledPostId) }
  }
  return { agents, posts }
}
