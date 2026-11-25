// Review round 2 of unit f1-eventlog, item 7. The alert worker remembers an
// alert the carrier accepted until its record is written (review round 1,
// item 3). A remembered alert that no longer waits for that destination and
// token must be forgotten, or the memory grows for the life of the service.
// Here the record cannot be written, and the token rotates while the worker
// is paused; the old token's entry must go.
import { randomUUID } from "node:crypto";
import * as Effect from "effect/Effect";
import * as SqlClient from "effect/sql/SqlClient";
import * as TestClock from "effect/testing/TestClock";
import { expect, it } from "vitest";

import { unrecordedAlerts } from "../../src/client/alerts/delivery.ts";
import { firstRetryMillis, maxRetryMillis } from "../../src/log/EventLog.ts";
import { Views } from "../../src/views/Views.ts";
import { configure, ntfyServer, posted, raiseQuestion, setToken, withCarrier } from "../support/carrierRig.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";
import { eventually } from "../support/routingRig.ts";

it("item 7 forgotten: an accepted alert whose token was rotated is no longer remembered", async () => {
  const server = await ntfyServer();
  try {
    await withCarrier(() => Effect.gen(function* () {
      yield* registerRebuildCheck();
      const views = yield* Views;
      const remembered = unrecordedAlerts(views);
      yield* configure(server.origin);
      yield* setToken(server.origin, randomUUID(), "first-token");
      const sql = yield* SqlClient.SqlClient;
      yield* sql.unsafe(`CREATE TRIGGER planted_record BEFORE INSERT ON events WHEN NEW.event_type = 'client.AlertDelivered'
        BEGIN SELECT RAISE(ABORT, 'planted record failure'); END`);
      yield* raiseQuestion("forget-7");
      yield* posted(server);
      const first = yield* eventually(Effect.sync(() => [...remembered.keys()]), (keys) => keys.length === 1);
      expect(first).toHaveLength(1);

      // Pass the writer's pause, so the token command is taken.
      yield* TestClock.adjust(firstRetryMillis + 1_000);
      yield* setToken(server.origin, randomUUID(), "second-token");
      yield* TestClock.adjust(maxRetryMillis + 1_000);
      yield* posted(server, 2);
      const later = yield* eventually(Effect.sync(() => [...remembered.keys()]), (keys) => keys.length === 1 && keys[0] !== first[0]);
      expect(later, "the rotated token's acceptance was never forgotten").not.toContain(first[0]);
    }), { clockAt: 1_800_000_000_000 });
  } finally { await server.close(); }
});
