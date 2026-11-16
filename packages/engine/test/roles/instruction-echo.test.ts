import { expect, it } from 'vitest'
import { instructionEcho } from '../../src/roles/instructionEcho.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'

registerRebuildCheck({ noLog: true })
it('s22: instruction echoes are removed at every possible chunk boundary and caller text remains', () => {
  const prefix = 'Engine instructions [random]\n/base\nEnd [random]\n\n', caller = 'Caller brief.'
  for (let split = 0; split <= prefix.length + caller.length; split++) {
    const filter = instructionEcho(prefix), text = prefix + caller
    expect(filter.push(text.slice(0, split)) + filter.push(text.slice(split))).toBe(caller)
  }
})
it('s22: ordinary user messages and mismatching partial prefixes remain whole', () => {
  const filter = instructionEcho('Engine instructions')
  expect(filter.push('Eng')).toBe('')
  expect(filter.push('lish message')).toBe('English message')
  expect(filter.push(' after')).toBe(' after')
})
it('s22: truncated instruction echoes disclose no partial prefix', () => {
  const filter = instructionEcho('Engine instructions')
  expect(filter.push('Engine inst')).toBe('')
})
