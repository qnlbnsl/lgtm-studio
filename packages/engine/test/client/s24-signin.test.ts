import { Account } from '@lgtm/contracts'
import * as Schema from 'effect/Schema'
import * as Effect from 'effect/Effect'
import { expect, it } from 'vitest'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
import { run, read } from './support/readRig.ts'
import { appendSetup, eventOf, seedAgent, call, ownerCredential, success, harnessSettings } from '../support/handlerRig.ts'
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
  expect(account(yield* read('client.snapshot'))).not.toHaveProperty('signedIn')
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
  expect(signInEvidence(account)).toBe(true)
})

for (const harness of ['cursor', 'custom-login']) it(`a ${harness} account with no recognised local key stays unknown after an automatic check`, async () => run(Effect.gen(function* () {
  yield* appendSetup([
    eventOf({ _tag: 'HarnessChanged', harness: harnessSettings(harness, { adapterPath: process.execPath }) }),
    eventOf({ _tag: 'AccountChanged', accountId: 'unchecked', account: { id: 'unchecked', harness } })
  ])
  yield* read('client.harness.status')
  expect((yield* read('client.snapshot')).accounts.find((row: any) => row.id === 'unchecked')).not.toHaveProperty('signedIn')
}), registerRebuildCheck()))

it('SessionOpened wins over an empty local store, while owner-cleared sign-in stays unknown until new evidence arrives', async () => run(Effect.gen(function* () {
  yield* appendSetup([eventOf({ _tag: 'HarnessChanged', harness: harnessSettings('first', { adapterPath: process.execPath }) })])
  yield* seedAgent({ id: 'real-session' })
  expect((yield* read('client.harness.account.check', { accountId: 'primary' })).state).toBe('signed-out')
  expect((yield* read('client.snapshot')).accounts.find((row: any) => row.id === 'primary')).toMatchObject({ signedIn: true })
  yield* appendSetup([eventOf({ _tag: 'AccountNeedsSignIn', accountId: 'primary', reason: 'expired' })])
  const row = (yield* (yield* Views).snapshot).accounts.primary!
  recordSignInCheck(row, 'local-credential')
  expect((yield* read('client.snapshot')).accounts.find((row: any) => row.id === 'primary')).toMatchObject({ signedIn: false })
  success(yield* call(ownerCredential, 'accounts mark-available', { accountId: 'primary' }))
  yield* read('client.harness.status')
  expect((yield* read('client.snapshot')).accounts.find((row: any) => row.id === 'primary')).not.toHaveProperty('signedIn')
}), registerRebuildCheck()))
