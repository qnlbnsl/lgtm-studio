import { registerRebuildCheck } from '../support/rebuildCheck.ts'
import { expect, it, vi } from 'vitest'
import * as Cause from 'effect/Cause'
import * as Redacted from 'effect/Redacted'
import { startupFailure } from '../../src/api/startupFailure.ts'
import { makeRedactor } from '../../src/identity/redactor.ts'

registerRebuildCheck({ noLog: true })

it('uses fixed categories and the normal redactor without copying underlying causes or paths', () => {
  const secret = 'state folder refused'
  const redactor = makeRedactor({ secrets: () => [Redacted.make(secret)], tokenHashes: () => new Set() })
  expect(startupFailure(Cause.fail(new Error(secret)), redactor.redactText)).toBe('[REDACTED]')
  expect(startupFailure(Cause.fail(new Error('private credential file contents')), redactor.redactText)).toBe('service failed')
  expect(startupFailure(Cause.die(new Error('/private/credential/path')), redactor.redactText)).toBe('service failed')
  expect(startupFailure(Cause.fail(new Error(secret)), () => 'x'.repeat(10000))).toHaveLength(512)
})

it('does not invoke the redactor for an interruption-only shutdown', () => {
  const redact = vi.fn(() => { throw new Error('shutdown redactor must not run') })
  expect(startupFailure(Cause.interrupt(), redact)).toBeUndefined()
  expect(redact).not.toHaveBeenCalled()
})

it('still reports failures accompanied by interruption and excludes the withdrawn web category', () => {
  const redact = vi.fn((text: string) => text)
  const cause = Cause.fromReasons([...Cause.interrupt().reasons, ...Cause.fail(new Error('state folder refused')).reasons])
  expect(startupFailure(cause, redact)).toBe('state folder refused')
  expect(redact).toHaveBeenCalledOnce()
  expect(startupFailure(Cause.fail(new Error('web port unavailable')), redact)).toBe('service failed')
})
