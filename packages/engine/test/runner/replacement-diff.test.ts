import { registerRebuildCheck } from '../support/rebuildCheck.ts'
import { expect, it } from 'vitest'
import { replacementDiff, translate } from '../../src/runner/translate.ts'

it('preserves both sides and newlines of legacy ACP edits and new files', () => {
  expect(replacementDiff('old\n', 'new\nnext\n')).toBe('@@ -1,1 +1,2 @@\n-old\n+new\n+next')
  expect(replacementDiff('', 'new')).toBe('@@ -0,0 +1,1 @@\n+new\n\\ No newline at end of file')
  expect(replacementDiff('old\n', '')).toBe('@@ -1,1 +0,0 @@\n-old')
  expect(replacementDiff('same\n', 'same\n')).toBe('')
  const content = translate({ sessionUpdate: 'tool_call_update', toolCallId: 'edit', content: [{ type: 'diff', path: 'MEMORY.md', oldText: 'before\n', newText: 'after\n' }] }, false)[0]!.content
  expect(content).toEqual({ _tag: 'file-change', callId: 'edit', path: 'MEMORY.md', diff: '@@ -1,1 +1,1 @@\n-before\n+after' })
})

registerRebuildCheck({ noLog: true })
