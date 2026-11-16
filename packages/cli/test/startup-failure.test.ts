import { expect, it } from 'vitest'
import * as Cause from 'effect/Cause'
import * as Redacted from 'effect/Redacted'
import { startupFailure } from '../../engine/src/api/startupFailure.ts'
import { makeRedactor } from '../../engine/src/identity/redactor.ts'

it('uses fixed categories and the normal redactor without copying underlying causes or paths', () => {
  const secret = 'web port unavailable'
  const redactor = makeRedactor({ secrets: () => [Redacted.make(secret)], tokenHashes: () => new Set() })
  expect(startupFailure(Cause.fail(new Error(secret)), redactor.redactText)).toBe('[REDACTED]')
  expect(startupFailure(Cause.fail(new Error('private credential file contents')), redactor.redactText)).toBe('service failed')
  expect(startupFailure(Cause.die(new Error('/private/credential/path')), redactor.redactText)).toBe('service failed')
  expect(startupFailure(Cause.fail(new Error(secret)), () => 'x'.repeat(10000))).toHaveLength(512)
})
