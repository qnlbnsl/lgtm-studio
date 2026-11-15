import { expect, it } from 'vitest'
import type { EventEnvelope } from '@lgtm/contracts'
import { toolNames } from '../../mcp/src/tools.ts'
import { translate } from '../../engine/src/runner/translate.ts'
import { toolRow, commandTitle } from '../src/chatTools.ts'
import { engineToolTitles } from '../src/engineToolTitles.ts'
import { mapThread, questionReply } from '../src/chat.ts'

const row = (update: object) => {
  const content = translate({ sessionUpdate: 'tool_call', toolCallId: 'call', ...update }, false)[0]!.content
  if (content._tag !== 'tool-call') throw new Error('Expected a tool')
  return toolRow({ id: '1', ts: 1 }, content)
}
// Inventory is derived from the engine command surface, rather than a recording.
it('names every exposed engine tool in Codex dotted and Claude double-underscore forms', () => {
  expect(Object.keys(engineToolTitles).sort()).toEqual([...toolNames].sort())
  for (const name of toolNames) {
    for (const title of [`mcp.lgtm.${name}`, `mcp__lgtm__${name}`, `mcp__lgtm__${name.replaceAll('.', '_')}`]) {
      expect(row({ title }).summary, title).toBe(engineToolTitles[name])
      expect(row({ title }).raw).toBe(title)
    }
  }
})
// Reconstructed from codex-acp 2.6.0 commandActionFacts, McpToolReporter,
// and AcpToolCallRenderer. Paths and commands are synthetic, not recordings.
it('maps Codex read, list and MCP reports through the engine translator', () => {
  expect(row({ title: "Read file '/work/SKILL.md'", kind: 'read', locations: [{ path: '/work/SKILL.md' }] })).toMatchObject({ summary: 'Read SKILL.md', details: [{ label: 'Path', text: '/work/SKILL.md' }] })
  expect(row({ title: "Read file '/work/2026-11-06-website-and-engine-design.md'", kind: 'read' }).summary).toBe('Read 2026-11-06-website-and-engine-design.md')
  expect(row({ title: "Read SKILL.md'" }).summary).toBe('Read SKILL.md')
  expect(row({ title: "List files in 'web'", kind: 'read' }).summary).toBe('Listed files in web')
  expect(row({ title: 'mcp.lgtm.project.design.read', kind: 'execute', rawInput: { server: 'lgtm', tool: 'project.design.read', arguments: { projectId: 'p' } } }).summary).toBe('Read the design')
})
// Reconstructed from claude-agent-acp 0.99.1 ReadReporter, BashReporter and
// renderer: title, kind, rawInput and locations. No code is copied.
it('maps Claude read and Bash reports through the same translator', () => {
  expect(row({ title: 'Read `src/file.ts` (1 - 40)', kind: 'read', rawInput: { file_path: '/work/src/file.ts', limit: 40 }, locations: [{ path: '/work/src/file.ts', line: 1 }] }).summary).toBe('Read file.ts')
  const command = 'printf one\nprintf two'
  expect(row({ title: command, kind: 'execute', rawInput: { command } })).toMatchObject({ summary: 'Ran printf one…', details: [{ label: 'Command', text: command }] })
})
// Exact wrapper and multiline edge cases are constructed from the owner's
// examples; the Codex reporter preserves the original command in rawInput.
it('unwraps only one exact quoted shell invocation and keeps complete input', () => {
  for (const shell of ['sh', 'bash', 'zsh', '/bin/sh', '/usr/bin/zsh']) for (const flag of ['-lc', '-c']) {
    const command = `${shell} ${flag} "rg --files -g AGENTS.md"`
    expect(row({ title: 'rg --files -g AGENTS.md', rawInput: { command } })).toMatchObject({ summary: 'Ran rg --files -g AGENTS.md', details: [{ label: 'Command', text: command }] })
  }
  for (const command of ['fish -c "ls"', 'zsh -ic "ls"', 'zsh -lc "ls"; touch /tmp/never', 'env zsh -c "ls"', 'zsh -c ls', '/bin/not-zsh -c "ls"']) expect(commandTitle(command)).toBe(command)
  const command = '/usr/bin/zsh -lc "printf \\\"%s\\\" \\$(never) \\`never\\`"'
  expect(row({ title: command, rawInput: { command } })).toMatchObject({ summary: 'Ran printf "%s" $(never) `never`', details: [{ text: command }] })
  const multiline = "printf '%s\\n' \\\n  one two"
  expect(row({ title: 'Shell', rawInput: { command: multiline } })).toMatchObject({ summary: "Ran printf '%s\\n'…", details: [{ text: multiline }] })
  expect(row({ title: '/usr/bin/zsh -lc "nl -ba file.ts"' })).toMatchObject({ summary: 'Ran nl -ba file.ts', details: [{ label: 'Command', text: '/usr/bin/zsh -lc "nl -ba file.ts"' }] })
})
// Constructed durable events following the announcement supplied in fixes-r1.md.
// QuestionAsked comes from routing/questions.ts; text chunks from translate.ts.
it('folds an answered same-turn announcement without losing independent prose', () => {
  const q = { _tag: 'QuestionAsked', agentId: 'lead', questionId: 'q', title: 'Test denial or approval next?', text: 'Choose the next test.', recommendation: 'Denial, then approval.', holder: { _tag: 'owner' }, deliveryId: 'd' } as Extract<EventEnvelope['event'], { _tag: 'QuestionAsked' }>
  const echo = `## ${q.title}\n\nRecommendation: ${q.recommendation}\n\nSent through LGTM Studio.`
  expect(questionReply(echo, [q])).toBe('')
  expect(questionReply(echo + '\n\nIndependent progress.', [q])).toBe('Independent progress.')
  const distinct = `${q.title}\n\nRecommendation: Something different.\n\nSent through LGTM Studio.`
  expect(questionReply(distinct, [q])).toBe('')
  expect(questionReply('Sent through LGTM Studio.', [])).toBe('Sent through LGTM Studio.')
  expect(questionReply('Independent progress.\n\nSent through LGTM Studio.', [])).toBe('Independent progress.\n\nSent through LGTM Studio.')
  const events = [
    { sequence: 1, occurredAt: 100, event: q },
    { sequence: 2, occurredAt: 200, event: { _tag: 'ThreadItem', agentId: 'lead', itemId: 'm', content: { _tag: 'text', text: echo } } },
    { sequence: 3, occurredAt: 900, event: { _tag: 'QuestionAnswered', agentId: 'lead', questionId: 'q', text: 'Denial' } },
    { sequence: 4, occurredAt: 1000, event: { _tag: 'ThreadItem', agentId: 'lead', itemId: 'next', content: { _tag: 'text', text: 'Next reply.' } } },
  ] as EventEnvelope[]
  const mapped = mapThread(events, 'lead').items
  expect(mapped).toEqual([{ id: '1', ts: 100, type: 'request', requestId: 'q' }, { id: '4', ts: 1000, type: 'agent', text: 'Next reply.' }])
  expect(mapThread([...events].reverse(), 'lead').items).toEqual(mapped)
})
