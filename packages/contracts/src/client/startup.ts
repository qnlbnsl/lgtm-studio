import * as Schema from 'effect/Schema'

// Authenticated HTTP replies, including a refused WebSocket upgrade, share
// this state. It contains counts only, never agent identities or credentials.
export const ServiceStarting = Schema.TaggedStruct('ServiceStarting', {
  recovered: Schema.Int.check(Schema.isGreaterThanOrEqualTo(0)),
  total: Schema.Int.check(Schema.isGreaterThanOrEqualTo(0)),
})
export type ServiceStarting = typeof ServiceStarting.Type
