import { readFileSync } from 'node:fs'
import { expect, it } from 'vitest'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
registerRebuildCheck({ noLog: true })
it('review 6: the security contract states executable, environment and process-group limits', () => {
  const security = readFileSync('SECURITY.md', 'utf8').split("## The owner's harness sign-in terminal")[1]!
  expect(security).toContain('adapterPath')
  expect(security).toContain('secret:')
  expect(security).toContain('engine shutdown')
  expect(security).toContain('starts its own session')
  expect(security).not.toContain('The page cannot supply a command, executable')
})
