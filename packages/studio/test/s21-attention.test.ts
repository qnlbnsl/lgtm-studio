import { readFileSync } from 'node:fs'
import { beforeAll, afterAll, expect, it } from 'vitest'

let data: any, live: any
beforeAll(async () => {
  data = await import('../../../design/mock/js/data.js' as string)
  live = (await import('../../../design/mock/js/lib.js' as string)).live
  live.on = true
})
afterAll(() => { live.on = false })
const worker = { id: 'worker', name: 'Worker', project: 'project', team: 'team', role: 'builder', state: 'working' }
const project = { id: 'project', repo: 'repo' }, team = { id: 'team' }
const state = { projects: [project], agents: [worker, { id: 'lead', name: 'Lead', project: 'project' }], requests: [
  { id: 'held', agent: 'worker', project: 'project', status: 'open', kind: 'approval', heldBy: 'lead', canAnswer: true, createdAt: 1 },
] }
// Run each caller's actual query against the same lead-held permission.
// This catches accidental changes to the shared query's audience as well as
// a caller switching to the wrong audience. The browser checks the screens.
for (const file of ['app.js', 'ui.js', 'actions.js', 'data.js', 'screens/home.js', 'screens/workflow.js', 'screens/chat.js', 'screens/messages.js', 'screens/service.js']) {
  const source = readFileSync(`${process.cwd()}/design/mock/js/${file}`, 'utf8')
  const queries = [...source.matchAll(/(?:openRequests|answerableRequests)\((?:s|st)(?:, \{[^}]*\})?\)/g)]
  for (const [index, match] of queries.entries()) {
    const line = source.slice(0, match.index).split('\n').length
    const expected = file === 'app.js' || file === 'screens/home.js' || file === 'data.js' && index === 3 ? 1 : 0
    it(`s21 request audience: ${file}:${line} query ${index + 1} includes ${expected} lead-held permissions`, () => {
      const query = new Function('s', 'st', 'a', 'p', 'project', 't', 'agentId', 'openRequests', 'answerableRequests', 'r', `return ${match[0]}`)
      expect(query(state, state, worker, project, project, team, 'worker', data.openRequests, data.answerableRequests, state.requests[0])).toHaveLength(expected)
    })
  }
}
it('s21 worker status retains its master status while a lead holds permission', () => {
  expect(data.agentStatus(state, { ...worker, state: 'waiting' }).word).toBe('Idle')
})
it('s21 lead status retains its master waiting-request count', () => {
  expect(data.agentStatus(state, { ...worker, id: 'lead', role: 'project-lead' }).word).toBe('Working')
})
it('s21 crash stays in the Messages needs lane', async () => {
  const { situation } = await import('../../../design/mock/js/screens/messages.js' as string)
  expect(situation(state, { ...worker, crashed: true, state: 'stopped', activity: 'Crashed' }, Date.now())).toMatchObject({ lane: 'needs', tone: 'error' })
})
it('s21 crash stays in the Workflow Waiting group', async () => {
  const { agentGroups } = await import('../../../design/mock/js/screens/workflow.js' as string)
  const groups = agentGroups({ ...state, agents: [{ ...worker, crashed: true, state: 'stopped' }] }, project)
  expect(groups.map((g: any) => [g.id, g.agents.map((a: any) => a.id)])).toEqual([['waiting', ['worker']]])
})

it('s21 live stop dialog keeps its master count including a lead-held permission', () => {
  const source = readFileSync(`${process.cwd()}/design/mock/js/live/chat.js`, 'utf8')
  const expression = source.match(/const approvals = (S\(\)\.requests\.filter\([^;]+);/)!;
  const query = new Function('S', 'agentId', `return ${expression[1]}`)
  expect(query(() => state, 'worker')).toHaveLength(1)
})
