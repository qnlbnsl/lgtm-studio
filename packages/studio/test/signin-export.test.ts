import { readFileSync } from 'node:fs'
import { expect, it } from 'vitest'
import { signInSource } from '../scripts/signInSource.ts'
it('review 10: the standalone mock exports account refresh without build injection', () => {
  const source = readFileSync('design/mock/js/live/git.js', 'utf8')
  expect(source).toMatch(/export const refreshAccounts =/)
  const built = signInSource('js/live/git.js', source)
  expect(built.match(/export const refreshAccounts =/g)).toHaveLength(1)
})
