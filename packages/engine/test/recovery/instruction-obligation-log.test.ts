import { it } from "@effect/vitest";
import type { Event } from "@lgtm/contracts";
import { expect } from "vitest";

import type { Snapshot } from "../../src/commands/internal.ts";
import { instructionsOwed } from "../../src/recovery/reload.ts";
import { owedAfter } from "../../src/roles/instructionObligation.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";

// Review round 1 of unit f2-lifecycle, item 6: the engine reads the runner's
// instruction rule from the log. These cases pin where the log could
// disagree with the runner.

registerRebuildCheck({ noLog: true });
const agentId = "agent-1";
const session = { _tag: "acp", sessionId: "s-1" };
const opened = { _tag: "SessionOpened", agentId, session, protocolVersion: 2, reloadable: true };
const sent = (deliveryId: string) => ({ _tag: "DeliverySent", agentId, deliveryId, method: "acp" });
const ended = (compact = false, stopped?: true) => ({ _tag: "TurnEnded", agentId, compact, ...(stopped === undefined ? {} : { stopped }) });
const owed = (...events: ReadonlyArray<Record<string, unknown>>) =>
  instructionsOwed({ threads: { [agentId]: events.map((event) => ({ event: event as unknown as Event })) } } as unknown as Snapshot, agentId as never);

it("review 6: the shared rule", () => {
  expect(owedAfter(false, { _tag: "conversationOpened" })).toBe(true);
  expect(owedAfter(true, { _tag: "turnSucceeded", compact: false, carried: true })).toBe(false);
  expect(owedAfter(true, { _tag: "turnSucceeded", compact: false, carried: false })).toBe(true);
  expect(owedAfter(false, { _tag: "turnSucceeded", compact: true, carried: false })).toBe(true);
});

it("review 6: catches the log discharging the instructions for a turn the runner counts as failed", () => {
  expect(owed(opened), "a new session owes them").toBe(true);
  expect(owed(opened, sent("d-1"), ended()), "a turn that ended normally discharged them").toBe(false);
  expect(owed(opened, sent("d-1")), "an interrupted first turn leaves them owed").toBe(true);
  expect(owed(opened, sent("d-1"), { _tag: "DeliveryAccepted", agentId, deliveryId: "d-1" }), "acceptance alone does not discharge them").toBe(true);
  expect(owed(opened, sent("d-1"), ended(false, true)), "a stopped turn leaves them owed").toBe(true);
  // A capacity hold keeps the agent working, so the engine still writes the
  // turn end, but the runner counted the turn as failed.
  expect(owed(opened, sent("d-1"), { _tag: "DeliveryRequeued", agentId, deliveryId: "d-1", reason: "model at capacity" },
    { _tag: "DeliveryHeld", agentId, deliveryId: "d-1", reason: "capacity" }, ended()), "a capacity-held turn leaves them owed").toBe(true);
  expect(owed(opened, sent("d-1"), ended(), sent("c-1"), ended(true)), "a completed compaction owes them again").toBe(true);
  expect(owed(opened, sent("d-1"), ended(), opened), "a load of the same session keeps them discharged").toBe(false);
  expect(owed(opened, sent("d-1"), ended(), { ...opened, session: { _tag: "acp", sessionId: "s-2" } }), "a new session owes them again").toBe(true);
});
