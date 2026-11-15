import { registerRebuildCheck } from '../support/rebuildCheck.ts'
import { expect, it } from 'vitest'
import { replacementDiff, translate } from '../../src/runner/translate.ts'

it('preserves both sides and newlines of legacy ACP edits and new files', () => {
  expect(replacementDiff('old\n', 'new\nnext\n')).toBe('@@ Change (line numbers unavailable) @@\n-old\n+new\n+next')
  expect(replacementDiff('', 'new')).toBe('@@ New file (line numbers unavailable) @@\n+new')
  expect(replacementDiff('old\n', '')).toBe('@@ Change (line numbers unavailable) @@\n-old')
  expect(replacementDiff('same\n', 'same\n')).toBe('')
  const content = translate({ sessionUpdate: 'tool_call_update', toolCallId: 'edit', content: [{ type: 'diff', path: 'MEMORY.md', oldText: 'before\n', newText: 'after\n' }] }, false)[0]!.content
  expect(content).toEqual({ _tag: 'file-change', callId: 'edit', path: 'MEMORY.md', diff: '@@ Change (line numbers unavailable) @@\n-before\n+after' })
})

registerRebuildCheck({ noLog: true })

it('keeps two diff parts for one path in order and emits the completing tool patch', () => {
  const first = '@@ -2 +2 @@\n-first old\n+first new'
  const second = '@@ -8 +8 @@\n-second old\n+second new'
  const result = translate({ sessionUpdate: 'tool_call_update', toolCallId: 'edit', title: 'Edit', status: 'completed', content: [first, second, first].map((text) => ({ type: 'diff', changes: [{ path: 'src/a.ts' }], patch: { text, format: 'git_patch' } })) }, false)
  expect(result.map((item) => item.content)).toEqual([
    { _tag: 'file-change', callId: 'edit', path: 'src/a.ts', diff: first + '\n' + second },
    { _tag: 'tool-call', callId: 'edit', title: 'Edit', status: 'completed' },
  ])
})

it('does not invent earlier text when oldText is omitted', () => {
  expect(translate({ sessionUpdate: 'tool_call_update', toolCallId: 'c', content: [{ type: 'diff', path: 'notes.md', newText: '- a list\n+ plain content' }] }, false)[0]!.content).toMatchObject({ diff: '- a list\n+ plain content' })
})
