// @vitest-environment jsdom
import { mkdtempSync, rmSync } from 'node:fs'
import { expect, it } from 'vitest'
import { InstructionStore } from '../../src/roles/instructions.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'

registerRebuildCheck({ noLog: true })
it('s22: Node filesystem resources load in an embedded browser test environment', async () => {
  const home = mkdtempSync('/tmp/s22-browser-resources-')
  try { expect(await new InstructionStore(home).compose('builder')).toContain('# Builder') }
  finally { rmSync(home, { recursive: true, force: true }) }
})
