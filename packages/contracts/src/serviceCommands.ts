import * as Schema from 'effect/Schema'
import * as Rpc from 'effect/rpc/Rpc'
import * as RpcGroup from 'effect/rpc/RpcGroup'
import { RequestId } from './ids.ts'
const request = { requestId: RequestId }
const Refused = Schema.TaggedStruct('CommandError', { ...request, reason: Schema.String })
// Issue 1575: whether the service can still save. When failing, `reason` and
// `since` (milliseconds since the epoch) name the first failure of the run,
// and `nextAttempt` says when the next write goes to the database again.
// `heldRunnerEvents` counts agent events waiting in memory for writes to work
// again, and `heldAgents` counts the agents they belong to.
export const WriteHealth = Schema.Struct({
  failing: Schema.Boolean, reason: Schema.optionalKey(Schema.String), since: Schema.optionalKey(Schema.Number),
  sqliteCode: Schema.optionalKey(Schema.Number), failures: Schema.optionalKey(Schema.Number),
  nextAttempt: Schema.optionalKey(Schema.Number), restartNeeded: Schema.optionalKey(Schema.Boolean),
  restartReason: Schema.optionalKey(Schema.String),
  heldRunnerEvents: Schema.optionalKey(Schema.Number), heldAgents: Schema.optionalKey(Schema.Number),
})
export const ServiceControlRpcGroup = RpcGroup.make(
  Rpc.make('version', { payload: { ...request, check: Schema.optionalKey(Schema.Boolean) }, success: Schema.Struct({ ...request,
    version: Schema.String, commit: Schema.String, dataFolder: Schema.String, checkout: Schema.String,
    comparison: Schema.Literals(['unchecked', 'same', 'different', 'unavailable']), remoteCommit: Schema.optionalKey(Schema.String),
    reason: Schema.optionalKey(Schema.String), trackedBranch: Schema.optionalKey(Schema.String), updateCommands: Schema.Array(Schema.String),
    writes: Schema.optionalKey(WriteHealth),
  }), error: Refused }),
  Rpc.make('restart', { payload: { ...request, confirmed: Schema.Boolean }, success: Schema.Struct({ ...request, restarting: Schema.Boolean }), error: Refused }),
).prefix('client.service.')
export const serviceControlAccess = { 'client.service.version': ['owner'], 'client.service.restart': ['owner'] } as const
export const serviceControlCommandNames = ['client.service.version', 'client.service.restart'] as const
