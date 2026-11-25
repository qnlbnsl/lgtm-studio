import { it } from "@effect/vitest";
import * as Effect from "effect/Effect";
import * as Option from "effect/Option";
import { expect } from "vitest";

import { preempt, withPreemption } from "../../src/routing/lanes.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";

// Review round 1 of unit f2-lifecycle, item 3: a stop's cancel signal belongs
// to the stops that raised it. A cancelled start keeps holding the record
// while its fallback waits for the lane; a resumed agent's send that
// registers in that window must not be cancelled by a stop that has ended.

registerRebuildCheck({ noLog: true });
type Log = Parameters<typeof preempt>[0];

it.effect("review 3: catches a send that registers after the stop ended being cancelled by that stop", () => Effect.gen(function* () {
  const log = {} as Log;
  const later = yield* withPreemption(log, "agent")((start) => Effect.gen(function* () {
    const settle = yield* preempt(log, "agent");
    expect(yield* start.cancellable(Effect.succeed("setup")), "the stop cancels the running start").toEqual(Option.none());
    yield* settle;
    // The start still holds the record for its fallback; a later send registers.
    return yield* withPreemption(log, "agent")((send) => send.cancellable(Effect.succeed("sent")));
  }));
  expect(later, "the ended stop cancelled a send it never saw").toEqual(Option.some("sent"));
}));

it.effect("review 3: catches a stop that is still running losing its signal for a holder that registers late", () => Effect.gen(function* () {
  const log = {} as Log;
  const settle = yield* preempt(log, "agent");
  const late = yield* withPreemption(log, "agent")((start) => start.cancellable(Effect.succeed("setup")));
  expect(late, "a stop that has not ended still cancels a start that registers after it signalled").toEqual(Option.none());
  yield* settle;
  // Once every stop has ended, nothing is left signalled.
  expect(yield* withPreemption(log, "agent")((send) => send.cancellable(Effect.succeed("sent")))).toEqual(Option.some("sent"));
}));

it.effect("review 3: catches the signal clearing while one of two stops is still running", () => Effect.gen(function* () {
  const log = {} as Log;
  const result = yield* withPreemption(log, "agent")(() => Effect.gen(function* () {
    const first = yield* preempt(log, "agent");
    const second = yield* preempt(log, "agent");
    yield* first;
    const during = yield* withPreemption(log, "agent")((send) => send.cancellable(Effect.succeed("sent")));
    yield* second;
    const after = yield* withPreemption(log, "agent")((send) => send.cancellable(Effect.succeed("sent")));
    return { during, after };
  }));
  expect(result.during).toEqual(Option.none());
  expect(result.after).toEqual(Option.some("sent"));
}));
