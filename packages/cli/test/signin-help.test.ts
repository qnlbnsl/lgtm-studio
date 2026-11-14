import { expect, it } from 'vitest'
import { helpText } from '../src/commands/registry.ts'
for (const method of ['write', 'resize', 'stop', 'subscribe'] as const) it(`review 9: ${method} help explains standalone connection limits`, () => {
  expect(helpText(`client.harness.signin.${method}`)).toContain('Standalone invocations open a new connection and cannot control an existing session.')
})
it('review 9: write help warns about shell history', () => {
  expect(helpText('client.harness.signin.write')).toContain('--text puts keys in shell history')
})
