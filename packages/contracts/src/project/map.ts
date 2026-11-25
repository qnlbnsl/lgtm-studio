import * as Schema from "effect/Schema"
import * as Rpc from "effect/rpc/Rpc"
import * as RpcGroup from "effect/rpc/RpcGroup"
import { RequestId } from "../ids.ts"
import { DisplayFrame } from "./display.ts"

import { MapSelection } from "./mapSelection.ts"
export { MapSelection } from "./mapSelection.ts"

const count = Schema.Int.check(Schema.isGreaterThanOrEqualTo(0))
const facts = Schema.Struct({ generatedAt: Schema.optionalKey(Schema.String), files: Schema.optionalKey(count),
  loc: Schema.optionalKey(count), nodes: Schema.optionalKey(count), links: Schema.optionalKey(count) })
export const MapStatus = Schema.Struct({ exists: Schema.Boolean, size: Schema.optionalKey(count),
  modifiedAt: Schema.optionalKey(Schema.Number), facts: Schema.optionalKey(facts) })
const request = { requestId: RequestId }
const error = Schema.TaggedStruct("CommandError", { ...request, reason: Schema.String })
export const ProjectMapRpcGroup = RpcGroup.make(
  Rpc.make("map", { payload: { ...request, ...MapSelection }, success: Schema.Struct({ ...request, result: MapStatus }), error }),
  Rpc.make("map.open", { payload: { ...request, ...MapSelection }, success: Schema.Struct({ ...request, frame: DisplayFrame }), error }),
  Rpc.make("map.release", { payload: { ...request, id: Schema.NonEmptyString.check(Schema.isMaxLength(100)) }, success: Schema.Struct(request), error })
).prefix("project.")
export const projectMapAccess = { "project.map": ["owner"], "project.map.open": ["owner"], "project.map.release": ["owner"] } as const
