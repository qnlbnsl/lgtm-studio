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
  expect(filter.push('lish message')).toBe('')
  expect(filter.push(' after')).toBe('')
  expect(filter.finish()).toBe('English message after')
})
it('s22: truncated instruction echoes disclose no partial prefix', () => {
  const filter = instructionEcho('Engine instructions')
  expect(filter.push('Engine inst')).toBe('')
})
it.each(['changed body', 'line endings', 'changed opening marker', 'changed closing marker'])('s22 r2: %s inside an echo never releases instructions', (change) => {
  const prefix = 'Engine role instructions [random]:\nPRIVATE INSTRUCTIONS\nEnd engine role instructions [random]\n\n'
  const caller = 'Caller brief.'
  const text = (change === 'changed body' ? prefix.replace('PRIVATE', 'MODIFIED PRIVATE') : change === 'line endings' ? prefix.replaceAll('\n', '\r\n') : change === 'changed opening marker' ? prefix.replace('[random]', '[changed]') : prefix.replace('End engine', 'End changed engine')) + caller
  for (let split = 0; split <= text.length; split++) {
    const filter = instructionEcho(prefix)
    const output = filter.push(text.slice(0, split)) + filter.push(text.slice(split))
    expect(output).not.toContain('PRIVATE')
    expect(output).not.toContain('Engine role instructions')
    if (change !== 'changed closing marker') expect(output).toBe(caller)
  }
})
it.each(['\n', '\uFEFF', '<user>\n'])('s22 r3: a decorated echo %j is removed at every split boundary', (decoration) => {
  const prefix = 'Engine role instructions [random]:\nPRIVATE INSTRUCTIONS\nEnd engine role instructions [random]\n\n'
  const caller = 'Caller brief.', text = decoration + prefix + caller
  for (let split = 0; split <= text.length; split++) {
    const filter = instructionEcho(prefix)
    expect(filter.push(text.slice(0, split)) + filter.push(text.slice(split))).toBe(caller)
  }
})
