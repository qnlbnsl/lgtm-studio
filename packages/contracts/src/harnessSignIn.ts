import * as Schema from 'effect/Schema'
import * as Rpc from 'effect/rpc/Rpc'
import * as RpcGroup from 'effect/rpc/RpcGroup'
import { Account } from './accounts.ts'
import { AccountId, HarnessId, RequestId } from './ids.ts'
const request = { requestId: RequestId }, session = { ...request, sessionId: Schema.String }
const Refused = Schema.TaggedStruct('CommandError', { ...request, reason: Schema.String })
export const SignInCheck = Schema.Struct({ state: Schema.Literals(['local-credential', 'signed-out', 'unavailable', 'missing-executable']) })
export const SignInOutput = Schema.Union([
  Schema.TaggedStruct('output', { text: Schema.String }),
  Schema.TaggedStruct('ended', { reason: Schema.String, check: SignInCheck })
])
export type SignInOutput = typeof SignInOutput.Type
export const SignInRpcGroup = RpcGroup.make(
  Rpc.make('account.add', { payload: { ...request, harnessId: HarnessId, name: Schema.NonEmptyString, folder: Schema.Literals(['existing', 'separate', 'advanced']), home: Schema.optionalKey(Schema.String) }, success: Schema.Struct({ ...request, account: Account }), error: Refused }),
  Rpc.make('signin.start', { payload: { ...request, accountId: AccountId }, success: Schema.Struct({ ...session }), error: Refused }),
  Rpc.make('signin.write', { payload: { ...session, text: Schema.String.check(Schema.isMaxLength(4096)) }, success: Schema.Struct(request), error: Refused }),
  Rpc.make('signin.resize', { payload: { ...session, cols: Schema.Int.check(Schema.isBetween({ minimum: 20, maximum: 240 })), rows: Schema.Int.check(Schema.isBetween({ minimum: 5, maximum: 100 })) }, success: Schema.Struct(request), error: Refused }),
  Rpc.make('signin.stop', { payload: session, success: Schema.Struct(request), error: Refused }),
  Rpc.make('signin.subscribe', { payload: session, success: SignInOutput, stream: true, error: Refused }).setError(Refused)
).prefix('client.harness.')
export const signInCommandNames = ['client.harness.account.add', 'client.harness.signin.start', 'client.harness.signin.write', 'client.harness.signin.resize', 'client.harness.signin.stop', 'client.harness.signin.subscribe'] as const
export const signInAccess = Object.fromEntries(signInCommandNames.map(name => [name, ['owner']])) as unknown as { readonly [K in typeof signInCommandNames[number]]: readonly ['owner'] }
