import { Account } from '@lgtm/contracts'
import * as Schema from 'effect/Schema'
import * as Effect from 'effect/Effect'
import { expect, it } from 'vitest'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
import { run, read } from './support/readRig.ts'
import { appendSetup, eventOf, seedAgent, call, ownerCredential, success } from '../support/handlerRig.ts'
import { recordSignInCheck, signInEvidence } from '../../src/accounts/signInEvidence.ts'
import { Views } from '../../src/views/Views.ts'

it('the real snapshot handler needs successful-session evidence and replays it without changing events', async () => run(Effect.gen(function* () {
  const account = (reply: any) => reply.accounts.find((row: any) => row.id === 'primary')
  expect(account(yield* read('client.snapshot'))).not.toHaveProperty('signedIn')
  yield* seedAgent({ id: 'starting-only', state: 'starting' })
  expect(account(yield* read('client.snapshot'))).not.toHaveProperty('signedIn')
  yield* seedAgent({ id: 'opened', state: 'idle' })
  expect(account(yield* read('client.snapshot'))).toMatchObject({ signedIn: true })
  yield* appendSetup([eventOf({ _tag: 'AccountNeedsSignIn', accountId: 'primary', reason: 'signed out' })])
  expect(account(yield* read('client.snapshot'))).toMatchObject({ signedIn: false })
  yield* appendSetup([eventOf({ _tag: 'AccountAvailable', accountId: 'primary', cause: 'owner cleared' })])
  expect(account(yield* read('client.snapshot'))).toMatchObject({ signedIn: false })
  yield* seedAgent({ id: 'opened-again' })
  expect(account(yield* read('client.snapshot'))).toMatchObject({ signedIn: true })
  const original = (yield* (yield* Views).snapshot).accounts.primary!
  success(yield* call(ownerCredential, 'accounts', { operation: 'set', account: { id: original.id, harness: original.harness, environment: { TEST_ACCOUNT: 'changed' } } }))
  expect(account(yield* read('client.snapshot'))).not.toHaveProperty('signedIn')
}), registerRebuildCheck()))

it('an unavailable check is not evidence of sign-out, and a changed row invalidates a local check', () => {
  const account = Schema.decodeUnknownSync(Account)({ id: 'checked', harness: 'claude' })
  recordSignInCheck(account, 'unavailable')
  expect(signInEvidence(account)).toBeUndefined()
  recordSignInCheck(account, 'local-credential')
  expect(signInEvidence(account)).toBe(true)
  recordSignInCheck(account, 'missing-executable')
  expect(signInEvidence(account)).toBe(true)
  expect(signInEvidence({ ...account })).toBeUndefined()
  recordSignInCheck(account, 'signed-out')
  expect(signInEvidence(account)).toBe(false)
})
