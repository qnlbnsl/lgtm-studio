import { expect, it } from 'vitest'
import type { EventEnvelope } from '@lgtm/contracts'
import { mapThread, questionReply } from '../src/chat.ts'
import { toolRow } from '../src/chatTools.ts'
import { descriptions } from '../../mcp/src/tools.ts'
import { projectLeadInstruction } from '../../engine/src/project/lifecycle.ts'

const row = (title: string, command?: string) => toolRow({ id: '1', ts: 1 }, { _tag: 'tool-call', callId: 'c', title, ...(command === undefined ? {} : { input: JSON.stringify({ command }) }) })
const event = (sequence: number, value: object) => ({ sequence, occurredAt: sequence * 1000, event: value }) as EventEnvelope
const text = (sequence: number, value: string, agentId = 'lead') => event(sequence, { _tag: 'ThreadItem', agentId, itemId: 'message', content: { _tag: 'text', text: value } })
// The orchestrator supplied this observed message in fixes-r1.md. These are
// constructed durable events, not an exported harness recording.
const question = { _tag: 'QuestionAsked', agentId: 'lead', questionId: 'q', title: 'Test denial or approval next?', text: 'Should the next delete-operation test exercise denial or approval?', recommendation: 'Denial, to verify that rejecting a request prevents execution.', holder: { _tag: 'owner' }, deliveryId: 'd' } as Extract<EventEnvelope['event'], { _tag: 'QuestionAsked' }>
const announcement = `**${question.title}**\n\nRecommendation: ${question.recommendation}\n\nSent through LGTM Studio.`

it('unwraps only the specified shell executables, with either quote style, including title-only Bash', () => {
  for (const folder of ['', '/bin/', '/usr/bin/', '/usr/local/bin/']) for (const shell of ['sh', 'bash', 'zsh']) for (const flag of ['-lc', '-c']) for (const quote of ['"', "'"]) {
    const command = `${folder}${shell} ${flag} ${quote}git status${quote}`
    for (const result of [row(command), row('Shell', command)]) expect(result).toMatchObject({ summary: 'Ran git status', details: [{ label: 'Command', text: command }] })
  }
})
it('retains unusual shell paths, empty wrappers, extra arguments and redirects in the collapsed title', () => {
  for (const command of ['/tmp/evil/zsh -lc "ls"', '/home/agent/.cache/x/sh -c "ls"', './zsh -lc "ls"', '../bin/sh -c "ls"', 'zsh -lc ""', "bash -c ''", 'zsh -lc "ls" extra', 'zsh -lc "ls" && echo more', "sh -c 'ls' 2>/dev/null", "sh -c 'echo '\\''ls'", 'sh -c "ls" # comment']) {
    for (const result of [row(command), row('Shell', command)]) expect(result).toMatchObject({ summary: `Ran ${command}`, details: [{ label: 'Command', text: command }] })
  }
})
it('uses generic readable names for tools outside the engine table and retains the raw name', () => {
  for (const title of ['mcp.other.project.read', 'mcp__other__project_read']) expect(row(title)).toMatchObject({ summary: 'Read project', raw: title })
  for (const title of ['mcp.lgtm.new_action', 'mcp__lgtm__new_action']) expect(row(title)).toMatchObject({ summary: 'Used new action', raw: title })
  expect(row("Search for 'it's ready' in web").summary).toBe("Searched for it's ready in web")
  expect(row("Search for 'x'").summary).toBe('Searched for x')
  expect(row("List files in 'web'").summary).toBe('Listed files in web')
})

it('folds the observed title, recommendation and sign-off only for that same-turn question', () => {
  for (const heading of [question.title, `**${question.title}**`, `## ${question.title}`]) {
    const value = `${heading}\n\nRecommendation: Denial, to verify refusal first\n\nSent through LGTM Studio.`
    expect(questionReply(value, [question])).toBe('')
    expect(questionReply(value + '\n\nIndependent progress.', [question])).toBe('Independent progress.')
  }
  expect(mapThread([event(1, question), text(2, announcement)], 'lead').items).toEqual([{ id: '1', ts: 1000, type: 'request', requestId: 'q' }])
})
it('preserves ordinary messages and title mentions, even with the same sign-off', () => {
  for (const value of ['Sent through LGTM Studio.', 'The report was\nSent through LGTM Studio.', 'Status.\nSent through LGTM Studio.\nMore.', `I mentioned ${question.title}\n\nSent through LGTM Studio.`, `I asked the owner:\n\n${announcement}`, question.text]) {
    expect(questionReply(value, [question])).toBe(value)
    expect(questionReply(value, [])).toBe(value)
  }
})
it('preserves announcements in another turn, another session or another agent thread', () => {
  for (const boundary of ['TurnEnded', 'SessionOpened', 'AgentStopped', 'AgentInterrupted', 'AgentCrashed', 'AgentStarted']) {
    const events = [event(1, question), event(2, { _tag: boundary, agentId: 'lead' }), text(3, announcement)]
    expect(mapThread(events, 'lead').items).toContainEqual({ id: '3', ts: 3000, type: 'agent', text: announcement })
    const reused = [event(1, question), text(2, 'Earlier progress.'), event(3, { _tag: boundary, agentId: 'lead' }), text(4, announcement)]
    expect(mapThread(reused, 'lead').items).toContainEqual({ id: '4', ts: 4000, type: 'agent', text: announcement })
    const before = [text(1, announcement), event(2, { _tag: boundary, agentId: 'lead' }), event(3, question)]
    expect(mapThread(before, 'lead').items).toContainEqual({ id: '1', ts: 1000, type: 'agent', text: announcement })
  }
  expect(mapThread([event(1, { ...question, agentId: 'other' }), text(2, announcement)], 'lead').items).toEqual([{ id: '2', ts: 2000, type: 'agent', text: announcement }])
})
it('preserves a recommendation paragraph if the authoritative card has no recommendation', () => {
  const value = `${question.title}\n\nRecommendation: Denial.\n\nSent through LGTM Studio.`
  const { recommendation, ...withoutRecommendation } = question
  expect(questionReply(value, [withoutRecommendation])).toBe('Recommendation: Denial.\n\nSent through LGTM Studio.')
})
it('names asks from their matched holders and leaves an uncorrelated ask generic', () => {
  const ask = (sequence: number, title: string, input?: object) => event(sequence, { _tag: 'ThreadItem', agentId: 'lead', itemId: 'tool', content: { _tag: 'tool-call', callId: String(sequence), title, ...(input ? { input: JSON.stringify(input) } : {}) } })
  expect(row('mcp.lgtm.ask').summary).toBe('Asked a question')
  const events = [ask(1, 'mcp.lgtm.ask', { arguments: { text: question.text } }), event(2, question), ask(3, 'mcp__lgtm__ask', { text: 'Worker question' }), event(4, { ...question, questionId: 'q2', text: 'Worker question', holder: { _tag: 'agent', agentId: 'parent' } })]
  expect(mapThread(events, 'lead').items.filter(item => item.type === 'tools').map(item => item.summary)).toEqual(['Asked the owner', 'Asked the lead'])
})
it('tells agents to write nothing about a question after ask and scopes lead status updates before it', () => {
  expect(descriptions.ask).toContain('After calling ask, write nothing about the question; the owner sees it on a card.')
  expect(projectLeadInstruction('Sample', 'Check')).toContain('After calling ask, write nothing about the question; the owner sees it on a card.')
  expect(projectLeadInstruction('Sample', 'Check')).not.toMatch(/summari[sz]e.*question/i)
})
