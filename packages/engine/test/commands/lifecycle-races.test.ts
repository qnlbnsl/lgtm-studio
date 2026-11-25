import { it } from "@effect/vitest";
import * as Deferred from "effect/Deferred";
import * as Effect from "effect/Effect";
import * as Fiber from "effect/Fiber";
import * as Redacted from "effect/Redacted";
import * as SqlClient from "effect/sql/SqlClient";
import { afterEach, beforeEach, expect, vi } from "vitest";

import { verifyAgentToken } from "../../src/identity/tokens.ts";
import { EventLog } from "../../src/log/EventLog.ts";
import { preempt } from "../../src/routing/lanes.ts";
import { RunnerRejected } from "../../src/runner/Runner.ts";
import { send } from "../support/deliveryRig.ts";
import { incarnationOf, makeFakeRunner } from "../support/fakeRunner.ts";
import {
  agentRow, call, freshRequest, makeFakeSecrets, makeRig, ownerCredential, provideRig,
  readLog, removeRig, seedAccount, seedHarness, startPayload, success, type Fakes, type Rig,
} from "../support/handlerRig.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";
import { deliveryRow, eventually, pause, tagged } from "../support/routingRig.ts";

// Unit f2-lifecycle: races between a start, a stop and a resume of one agent.

let rig: Rig;
beforeEach(() => {
  rig = makeRig();
  vi.stubEnv("HOME", rig.home);
  vi.stubEnv("LGTM_HOME", rig.stateFolder);
});
afterEach(() => {
  vi.unstubAllEnvs();
  removeRig(rig);
});

const withRig = <A, E, R>(body: (fakes: Fakes) => Effect.Effect<A, E, R>) =>
  Effect.gen(function* () {
    const fakes: Fakes = { runner: yield* makeFakeRunner(), secrets: makeFakeSecrets() };
    return yield* provideRig(rig, fakes, Effect.gen(function* () {
      yield* registerRebuildCheck();
      yield* seedAccount();
      yield* seedHarness();
      return yield* body(fakes);
    }));
  });

const startedAgent = (fakes: Fakes) => Effect.gen(function* () {
  const calls = yield* eventually(Effect.sync(() => fakes.runner.callsOf("start")), (rows) => rows.length >= 1);
  return calls[0]!.input.agentId;
});

const tokenWorks = (raw: string) => verifyAgentToken(raw).pipe(Effect.result, Effect.map((result) => result._tag === "Success"));

// Issue 267: a start cancelled by a stop that was then refused runs its own
// fallback cleanup. That cleanup must not act on a later stop or on a later
// `resume --fresh` of the same agent.
it.live("267: catches a cancelled start's late fallback writing a second terminal event, revoking a resumed agent's new token, or stopping its new process", () =>
  withRig((fakes) => Effect.gen(function* () {
    // The first setup never returns; the resume's setup succeeds.
    let starts = 0;
    fakes.runner.script("start", (input) => ++starts === 1
      ? Effect.never
      : Effect.succeed({ session: { _tag: "acp", sessionId: `fresh-${input.agentId}` }, protocolVersion: 2, reloadable: true }));
    // The first process stop is the fallback's. It is held at the transport.
    const entered = yield* Deferred.make<void>();
    const release = yield* Deferred.make<void>();
    let stops = 0;
    fakes.runner.script("stop", () => ++stops === 1
      ? Deferred.succeed(entered, undefined).pipe(Effect.andThen(Deferred.await(release)))
      : Effect.void);
    yield* Effect.addFinalizer(() => Deferred.succeed(release, undefined));

    const requestId = freshRequest();
    const starting = yield* call(ownerCredential, "start", startPayload(rig, { requestId })).pipe(Effect.forkScoped);
    const agentId = yield* startedAgent(fakes);
    // A stop refused for a reused request id still cancels the setup, so the
    // start's own fallback must end the agent.
    const refusedStop = yield* call(ownerCredential, "stop", { agentId, requestId });
    expect(refusedStop._tag).toBe("Failure");
    yield* Deferred.await(entered);

    // A later, valid stop and a fresh resume of the same agent.
    const later = yield* Effect.gen(function* () {
      const stopped = yield* call(ownerCredential, "stop", { agentId });
      const resumed = yield* call(ownerCredential, "resume", { agentId, fresh: true });
      return { stopped, resumed };
    }).pipe(Effect.forkScoped);
    // Either the later commands finish while the fallback is held, or they
    // queue behind it; both orders must end in the same consistent state.
    yield* Effect.raceFirst(Fiber.await(later), pause(300));
    yield* Deferred.succeed(release, undefined);
    const outcome = yield* Fiber.join(later);
    yield* Fiber.join(starting);
    success(outcome.resumed);
    yield* eventually(agentRow(agentId), (row) => row?.state._tag === "idle");

    const rows = yield* readLog;
    const ends = rows.filter((row) => (row.event._tag === "AgentStopped" || row.event._tag === "AgentStartFailed") && row.event.agentId === agentId);
    expect(ends, "one terminal event for the cancelled start").toHaveLength(1);
    const opened = rows.findLastIndex((row) => row.event._tag === "SessionOpened" && row.event.agentId === agentId);
    expect(opened).toBeGreaterThan(-1);
    expect(rows.slice(opened + 1).filter((row) => row.event._tag === "AgentStartFailed" || row.event._tag === "AgentStopped")).toEqual([]);
    expect((yield* agentRow(agentId))?.state._tag).toBe("idle");
    // The resumed incarnation's token still works, and its process was not stopped.
    const resumedToken = fakes.runner.callsOf("start").at(-1)!.input.token;
    expect(yield* tokenWorks(Redacted.value(resumedToken))).toBe(true);
    const methods = fakes.runner.calls.filter((entry) => entry.input.agentId === agentId).map((entry) => entry.method);
    expect(methods.lastIndexOf("stop")).toBeLessThan(methods.lastIndexOf("start"));
    expect(tagged(rows, "AgentStartFailed").every((event) => event.reason === "could-not-start")).toBe(true);
    const sql = yield* SqlClient.SqlClient;
    const live = yield* sql<{ token_hash: string }>`SELECT token_hash FROM agent_tokens WHERE agent_id = ${agentId} AND revoked_at IS NULL`;
    expect(live).toHaveLength(1);
  })).pipe(Effect.orDie), 30_000);

// Issue 265: a send whose adapter stops reading holds the agent's lane while
// the runner waits for the write. A stop the caller could commit must not
// wait behind it.
const spawnIdle = (fakes: Fakes) => Effect.gen(function* () {
  const reply = success(yield* call(ownerCredential, "start", startPayload(rig)));
  yield* eventually(agentRow(reply.agentId), (row) => row?.state._tag === "idle");
  expect((yield* agentRow(reply.agentId))?.state._tag).toBe("idle");
  const started = fakes.runner.callsOf("start").find((entry) => entry.input.agentId === reply.agentId)!;
  return { agentId: reply.agentId as string, token: Redacted.value(started.input.token) };
});

it.live("265: catches a stop that waits behind a stalled prompt send", () =>
  withRig((fakes) => Effect.gen(function* () {
    const target = yield* spawnIdle(fakes);
    const reached: Array<string> = [];
    fakes.runner.script("prompt", (input) => Effect.sync(() => { reached.push(input.delivery.id); }));
    const held = fakes.runner.holdNext("prompt");
    yield* Effect.addFinalizer(() => held.release);
    const deliveryId = success(yield* call(ownerCredential, "prompt", { agentId: target.agentId, text: "A large prompt." })).deliveryId as string;
    yield* held.entered;

    // Well inside any bound: only a stop that pre-empts the send returns.
    const outcome = yield* call(ownerCredential, "stop", { agentId: target.agentId }).pipe(Effect.timeoutOption(5_000));
    expect(outcome._tag, "the stop returned while the send was stalled").toBe("Some");
    if (outcome._tag === "Some") success(outcome.value);
    expect((yield* agentRow(target.agentId))?.state._tag).toBe("stopped");
    expect(fakes.runner.callsOf("stop").map((entry) => entry.input.agentId)).toContain(target.agentId);

    // The stalled write never reaches the stopped process once it moves.
    yield* held.release;
    yield* eventually(Effect.sync(() => fakes.runner.callsOf("prompt").length), (count) => count === 1);
    expect(reached).toEqual([]);
    const rows = yield* readLog;
    expect(tagged(rows, "DeliveryWithdrawn")).toContainEqual(expect.objectContaining({ deliveryId }));
    expect(yield* tokenWorks(target.token)).toBe(false);
  })).pipe(Effect.orDie), 30_000);

it.live("265: catches a refused stop that abandons a stalled send it pre-empted", () =>
  withRig((fakes) => Effect.gen(function* () {
    const target = yield* spawnIdle(fakes);
    const reached: Array<string> = [];
    fakes.runner.script("prompt", (input) => Effect.sync(() => { reached.push(input.delivery.id); }));
    const held = fakes.runner.holdNext("prompt");
    yield* Effect.addFinalizer(() => held.release);
    const requestId = freshRequest();
    const deliveryId = success(yield* call(ownerCredential, "prompt", { agentId: target.agentId, text: "Still wanted.", requestId })).deliveryId as string;
    yield* held.entered;
    // The same request id makes the stop's transaction refuse it.
    const refusedStop = yield* call(ownerCredential, "stop", { agentId: target.agentId, requestId }).pipe(Effect.timeoutOption(5_000));
    expect(refusedStop._tag).toBe("Some");
    if (refusedStop._tag === "Some") expect(refusedStop.value._tag).toBe("Failure");
    expect((yield* agentRow(target.agentId))?.state._tag).toBe("working");
    // The send still completes for the live process, and nothing returned it.
    yield* held.release;
    expect(yield* eventually(Effect.sync(() => reached), (list) => list.length === 1)).toEqual([deliveryId]);
    const rows = yield* readLog;
    expect(tagged(rows, "DeliveryReturned")).toEqual([]);
    expect(tagged(rows, "DeliveryWithdrawn")).toEqual([]);
    expect((yield* agentRow(target.agentId))?.state._tag).toBe("working");
  })).pipe(Effect.orDie), 30_000);

// Issue 251: once a start, stop or resume has committed, its process work
// runs even when the caller goes away. The caller can go away during the
// work, or between the commit and the work.
const holdAfterNextCommit = (gate: { armed: boolean; readonly entered: Deferred.Deferred<void>; readonly release: Deferred.Deferred<void> }) =>
  (real: Parameters<NonNullable<Fakes["wrapLog"]>>[0]) => ({ ...real, transact: ((transaction: Parameters<typeof real.transact>[0]) => {
    if (!gate.armed) return real.transact(transaction);
    gate.armed = false;
    return Effect.gen(function* () {
      const result = yield* real.transact(transaction);
      yield* Deferred.succeed(gate.entered, undefined);
      yield* Deferred.await(gate.release);
      return result;
    });
  }) as typeof real.transact });

const withGate = <A, E, R>(body: (fakes: Fakes, gate: { armed: boolean; readonly entered: Deferred.Deferred<void>; readonly release: Deferred.Deferred<void> }) => Effect.Effect<A, E, R>) =>
  Effect.gen(function* () {
    const gate = { armed: false, entered: yield* Deferred.make<void>(), release: yield* Deferred.make<void>() };
    const fakes: Fakes = { runner: yield* makeFakeRunner(), secrets: makeFakeSecrets(), wrapLog: holdAfterNextCommit(gate) };
    return yield* provideRig(rig, fakes, Effect.gen(function* () {
      yield* registerRebuildCheck();
      yield* seedAccount();
      yield* seedHarness();
      yield* Effect.addFinalizer(() => Deferred.succeed(gate.release, undefined));
      return yield* body(fakes, gate);
    }));
  });

// A runner method that signals when it is reached, waits for the test, and
// records whether it ran to its end.
const heldMethod = () => Effect.gen(function* () {
  const entered = yield* Deferred.make<void>();
  const release = yield* Deferred.make<void>();
  const state = { finished: false };
  return {
    entered, release, state,
    run: <A>(value: A): Effect.Effect<A> => Effect.gen(function* () {
      yield* Deferred.succeed(entered, undefined);
      yield* Deferred.await(release);
      state.finished = true;
      return value;
    }),
  };
});

it.live("251 stop: catches a caller that goes away during the process stop leaving a live process for a stopped agent", () =>
  withRig((fakes) => Effect.gen(function* () {
    const target = yield* spawnIdle(fakes);
    const held = yield* heldMethod();
    yield* Effect.addFinalizer(() => Deferred.succeed(held.release, undefined));
    fakes.runner.script("stop", () => held.run(undefined));
    const stopping = yield* call(ownerCredential, "stop", { agentId: target.agentId }).pipe(Effect.forkScoped);
    yield* Deferred.await(held.entered);
    // The interruption is delivered before the held work is released.
    stopping.interruptUnsafe();
    yield* Deferred.succeed(held.release, undefined);
    yield* Fiber.await(stopping);
    expect((yield* eventually(Effect.sync(() => held.state.finished), (done) => done)), "the process stop ran to its end").toBe(true);
    expect((yield* agentRow(target.agentId))?.state._tag).toBe("stopped");
  })).pipe(Effect.orDie), 30_000);

it.live("251 stop: catches a caller that goes away just after the commit skipping the process stop", () =>
  withGate((fakes, gate) => Effect.gen(function* () {
    const target = yield* spawnIdle(fakes);
    const before = fakes.runner.callsOf("stop").length;
    gate.armed = true;
    const stopping = yield* call(ownerCredential, "stop", { agentId: target.agentId }).pipe(Effect.forkScoped);
    // The stop's transaction has committed; the caller has not seen it yet.
    yield* Deferred.await(gate.entered);
    expect((yield* agentRow(target.agentId))?.state._tag).toBe("stopped");
    yield* Fiber.interrupt(stopping);
    yield* Deferred.succeed(gate.release, undefined);
    const calls = yield* eventually(Effect.sync(() => fakes.runner.callsOf("stop").length), (count) => count > before);
    expect(calls, "the committed stop reached the process").toBe(before + 1);
  })).pipe(Effect.orDie), 30_000);

it.live("251 start: catches a caller that goes away during setup leaving a committed agent starting with no process", () =>
  withRig((fakes) => Effect.gen(function* () {
    const held = yield* heldMethod();
    yield* Effect.addFinalizer(() => Deferred.succeed(held.release, undefined));
    fakes.runner.script("start", (input) => held.run({ session: { _tag: "acp", sessionId: `s-${input.agentId}` }, protocolVersion: 2, reloadable: true }));
    const starting = yield* call(ownerCredential, "start", startPayload(rig)).pipe(Effect.forkScoped);
    yield* Deferred.await(held.entered);
    const agentId = yield* startedAgent(fakes);
    // The interruption is delivered before the held work is released.
    starting.interruptUnsafe();
    yield* Deferred.succeed(held.release, undefined);
    yield* Fiber.await(starting);
    const row = yield* eventually(agentRow(agentId), (row) => row?.state._tag === "idle");
    expect(row?.state._tag, "the committed start opened its session").toBe("idle");
    expect(held.state.finished).toBe(true);
  })).pipe(Effect.orDie), 30_000);

it.live("251 resume: catches a caller that goes away during the reload leaving the agent starting", () =>
  withRig((fakes) => Effect.gen(function* () {
    const target = yield* spawnIdle(fakes);
    success(yield* call(ownerCredential, "stop", { agentId: target.agentId }));
    const held = yield* heldMethod();
    yield* Effect.addFinalizer(() => Deferred.succeed(held.release, undefined));
    fakes.runner.script("load", (input) => held.run({ session: input.session, protocolVersion: 2, reloadable: true }));
    const resuming = yield* call(ownerCredential, "resume", { agentId: target.agentId }).pipe(Effect.forkScoped);
    yield* Deferred.await(held.entered);
    // The interruption is delivered before the held work is released.
    resuming.interruptUnsafe();
    yield* Deferred.succeed(held.release, undefined);
    yield* Fiber.await(resuming);
    const row = yield* eventually(agentRow(target.agentId), (row) => row?.state._tag === "idle");
    expect(row?.state._tag, "the committed resume opened its session").toBe("idle");
  })).pipe(Effect.orDie), 30_000);

// Review round 1 of unit f2-lifecycle, item 1: a send a stop abandoned runs
// in the engine's scope, so engine shutdown interrupts it. A refused stop
// leaves the process live, so only the shutdown can end the held send here.
it.live("265 review: catches an abandoned send that engine shutdown does not interrupt", () => Effect.gen(function* () {
  const held = yield* withRig((fakes) => Effect.gen(function* () {
    const target = yield* spawnIdle(fakes);
    const held = fakes.runner.holdNext("prompt");
    const requestId = freshRequest();
    success(yield* call(ownerCredential, "prompt", { agentId: target.agentId, text: "Never read.", requestId }));
    yield* held.entered;
    expect((yield* call(ownerCredential, "stop", { agentId: target.agentId, requestId }))._tag).toBe("Failure");
    expect((yield* agentRow(target.agentId))?.state._tag).toBe("working");
    return held;
  }));
  // The rig's scope has closed. A detached send would still be waiting.
  const interrupted = yield* held.interrupted.pipe(Effect.timeoutOption(5_000));
  yield* held.release;
  expect(interrupted._tag, "engine shutdown interrupted the abandoned send").toBe("Some");
}).pipe(Effect.orDie), 30_000);

// Review round 1, item 2: the late failure of an abandoned send acts only on
// the send it belongs to. A stop withdraws every pending delivery, so the
// path that sends the same delivery again is a refused stop, then a sign-out
// pause that puts it back, then the account's return.
it.live("265 review: catches an abandoned send's late failure returning the same delivery's later send", () => Effect.gen(function* () {
  const watch = { count: 0, after: Number.POSITIVE_INFINITY, handled: Deferred.makeUnsafe<void>() };
  const fakes: Fakes = { runner: yield* makeFakeRunner(), secrets: makeFakeSecrets(), wrapLog: (real) => ({ ...real,
    transact: ((use: Parameters<typeof real.transact>[0]) => real.transact(use).pipe(Effect.ensuring(Effect.sync(() => {
      watch.count += 1;
      if (watch.count >= watch.after) Deferred.doneUnsafe(watch.handled, Effect.void);
    })))) as typeof real.transact }) };
  yield* provideRig(rig, fakes, Effect.gen(function* () {
    yield* registerRebuildCheck();
    yield* seedAccount();
    yield* seedHarness();
    const target = yield* spawnIdle(fakes);
    const held = fakes.runner.holdNext("prompt");
    yield* Effect.addFinalizer(() => held.release);
    const requestId = freshRequest();
    const deliveryId = success(yield* call(ownerCredential, "prompt", { agentId: target.agentId, text: "Still wanted.", requestId })).deliveryId as string;
    yield* held.entered;
    expect((yield* call(ownerCredential, "stop", { agentId: target.agentId, requestId }))._tag).toBe("Failure");
    const sends = () => readLog.pipe(Effect.map((rows) => rows.filter((row) => row.event._tag === "DeliverySent" && row.event.deliveryId === deliveryId)));
    const first = (yield* sends())[0]!.sequence;
    // The abandoned write fails only once the test lets it, after the re-send.
    fakes.runner.script("prompt", (input) => input.sentSequence === first
      ? Effect.suspend(() => { watch.after = watch.count + 1; return Effect.fail(new RunnerRejected({ reason: "late transport failure" })); })
      : Effect.void);
    yield* fakes.runner.emit({ _tag: "promptReply", agentId: target.agentId as never, incarnation: incarnationOf(target.token), deliveryId,
      outcome: { _tag: "pause", reason: "signed-out", error: "fixture sign-in" } });
    yield* eventually(agentRow(target.agentId), (row) => row?.state._tag === "paused");
    success(yield* call(ownerCredential, "accounts mark-available", { accountId: "primary" }));
    const again = yield* eventually(sends(), (rows) => rows.length === 2);
    yield* eventually(Effect.sync(() => fakes.runner.callsOf("prompt").filter((entry) => entry.input.sentSequence === again[1]!.sequence)), (rows) => rows.length === 1);
    yield* held.release;
    yield* Deferred.await(watch.handled);
    const rows = yield* readLog;
    expect(tagged(rows, "DeliveryReturned").filter((event) => event.deliveryId === deliveryId), "the later send of the same delivery was returned").toEqual([]);
    expect((yield* agentRow(target.agentId))?.state._tag).toBe("working");
    expect((yield* deliveryRow(deliveryId))?.status).toBe("sent");
  }));
}).pipe(Effect.orDie), 30_000);

// Review round 1, item 4: the fallback's token check. A resume paused by a
// sign-out leaves the agent "paused" with the resume's token live, so the
// state alone cannot tell the fallback that its own start is over. A stop
// the test holds open keeps the fallback waiting until the resume is done.
it.live("267 review: catches a cancelled start's fallback acting on a later resume that a sign-out left paused", () =>
  withRig((fakes) => Effect.gen(function* () {
    const reporter = yield* spawnIdle(fakes);
    const resumeEntered = yield* Deferred.make<void>();
    const resumeRelease = yield* Deferred.make<void>();
    let starts = 0;
    fakes.runner.script("start", (input) => ++starts === 1
      ? Effect.never
      : Deferred.succeed(resumeEntered, undefined).pipe(Effect.andThen(Deferred.await(resumeRelease)),
        Effect.as({ session: { _tag: "acp" as const, sessionId: `fresh-${input.agentId}` }, protocolVersion: 2, reloadable: true })));
    yield* Effect.addFinalizer(() => Deferred.succeed(resumeRelease, undefined));
    const starting = yield* call(ownerCredential, "start", startPayload(rig)).pipe(Effect.forkScoped);
    const agentId = yield* eventually(Effect.sync(() => fakes.runner.callsOf("start").map((entry) => entry.input.agentId as string).filter((id) => id !== reporter.agentId)), (ids) => ids.length === 1).pipe(Effect.map((ids) => ids[0]!));
    // A stop that has signalled and not yet ended: the start gives up its
    // setup, and its fallback waits for this stop before it takes the lane.
    const settleHeld = yield* preempt(yield* EventLog, agentId);
    success(yield* call(ownerCredential, "stop", { agentId }));
    const resuming = yield* call(ownerCredential, "resume", { agentId, fresh: true }).pipe(Effect.forkScoped);
    yield* Deferred.await(resumeEntered);
    // The account signs out while the resume's session opens.
    const reported = yield* send(fakes.runner, reporter as never, "Check the sign-in.");
    yield* fakes.runner.emit({ _tag: "promptReply", agentId: reporter.agentId as never, incarnation: incarnationOf(reporter.token), deliveryId: reported,
      outcome: { _tag: "pause", reason: "signed-out", error: "fixture sign-in" } });
    yield* eventually(agentRow(agentId), (row) => row?.state._tag === "paused");
    yield* Deferred.succeed(resumeRelease, undefined);
    yield* Fiber.join(resuming);
    const resumedToken = Redacted.value(fakes.runner.callsOf("start").at(-1)!.input.token);
    expect(yield* tokenWorks(resumedToken)).toBe(true);
    const stopsBefore = fakes.runner.callsOf("stop").filter((entry) => entry.input.agentId === agentId).length;
    // The held stop ends; the fallback now takes the lane and must do nothing.
    yield* settleHeld;
    yield* Fiber.join(starting);
    const rows = yield* readLog;
    expect(tagged(rows, "AgentStartFailed").filter((event) => event.agentId === agentId), "the fallback wrote a start failure for the resumed agent").toEqual([]);
    expect(yield* tokenWorks(resumedToken), "the fallback revoked the resumed agent's token").toBe(true);
    expect(fakes.runner.callsOf("stop").filter((entry) => entry.input.agentId === agentId)).toHaveLength(stopsBefore);
    expect((yield* agentRow(agentId))?.state._tag).toBe("paused");
  })).pipe(Effect.orDie), 30_000);
