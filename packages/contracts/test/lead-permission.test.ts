import * as Schema from "effect/Schema"
import { expect, it } from "vitest"
import { ProjectLeadDefaults, effectiveLeadPermission } from "../src/index.ts"

// Issue 1463: a saved lead default of "ask" overrode the owner's "auto" default.
it("a saved ask that the owner did not choose follows the harness default", () => {
  expect(effectiveLeadPermission({ permission: "ask" }, "auto")).toBe("auto")
  expect(effectiveLeadPermission({ permission: "ask", permissionChosen: false }, "auto")).toBe("auto")
  expect(effectiveLeadPermission({ permission: "ask" }, "ask")).toBe("ask")
})
it("an ask that the owner chose is kept", () => {
  expect(effectiveLeadPermission({ permission: "ask", permissionChosen: true }, "auto")).toBe("ask")
})
it("every other saved level is kept, chosen or not", () => {
  for (const level of ["read-only", "edit", "auto", "full"] as const) expect(effectiveLeadPermission({ permission: level }, "auto")).toBe(level)
})
it("stored lead defaults without the marker still decode, and the marker survives a round trip", () => {
  const decode = Schema.decodeUnknownSync(ProjectLeadDefaults)
  const old = { harness: "claude", account: { _tag: "any" }, permission: "ask" }
  expect(decode(old)).toEqual(old)
  expect(decode({ ...old, permissionChosen: true }).permissionChosen).toBe(true)
})
