import { expect, it } from 'vitest'
import { helpText } from '../src/commands/registry.ts'
it('r2 restart help preserves sign-in and closes terminals, while terminal help explains the local connection boundary', () => {
  const help = helpText('client.service.restart')
  expect(help).toContain('stay signed in')
  expect(help).toContain('terminals close')
  expect(help).not.toContain('sign-out of every page')
  expect(helpText('client.terminal.list')).toContain('local-socket connection')
})
