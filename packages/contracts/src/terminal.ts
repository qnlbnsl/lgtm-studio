import * as Schema from 'effect/Schema'
import * as Rpc from 'effect/rpc/Rpc'
import * as RpcGroup from 'effect/rpc/RpcGroup'
import { RequestId } from './ids.ts'
import { ProjectId, WorkspaceId } from './project/schema.ts'
const request = { requestId: RequestId }, session = { ...request, sessionId: Schema.String }
const Refused = Schema.TaggedStruct('CommandError', { ...request, reason: Schema.String })
const size = { cols: Schema.Int.check(Schema.isBetween({ minimum: 20, maximum: 240 })), rows: Schema.Int.check(Schema.isBetween({ minimum: 5, maximum: 100 })) }
const strict = <F extends Schema.Struct.Fields>(fields: F) => Schema.Unknown.check(Schema.makeFilter((value) => typeof value === 'object' && value !== null && !Array.isArray(value) && Object.keys(value).every(key => Object.hasOwn(fields, key)), { expected: 'only the declared terminal request fields' })).pipe(Schema.decodeTo(Schema.Struct(fields)))
export const TerminalOutput = Schema.Union([
  Schema.TaggedStruct('output', { text: Schema.String }), Schema.TaggedStruct('ended', { reason: Schema.String })
])
export type TerminalOutput = typeof TerminalOutput.Type
export const TerminalRpcGroup = RpcGroup.make(
  Rpc.make('open', { payload: strict({ ...request, projectId: ProjectId, worktreeId: WorkspaceId, ...size }), success: Schema.Struct(session), error: Refused }),
  Rpc.make('write', { payload: strict({ ...session, text: Schema.String.check(Schema.isMaxLength(4096)) }), success: Schema.Struct(request), error: Refused }),
  Rpc.make('resize', { payload: strict({ ...session, ...size }), success: Schema.Struct(request), error: Refused }),
  Rpc.make('close', { payload: strict(session), success: Schema.Struct(request), error: Refused }),
  Rpc.make('subscribe', { payload: strict(session), success: TerminalOutput, stream: true, error: Refused }).setError(Refused)
).prefix('client.terminal.')
export const terminalCommandNames = ['client.terminal.open', 'client.terminal.write', 'client.terminal.resize', 'client.terminal.close', 'client.terminal.subscribe'] as const
export const terminalAccess = Object.fromEntries(terminalCommandNames.map(name => [name, ['owner']])) as unknown as { readonly [K in typeof terminalCommandNames[number]]: readonly ['owner'] }
