import { it } from "@effect/vitest";
import { HarnessSettings } from "@lgtm/contracts";
import * as Effect from "effect/Effect";
import * as Schema from "effect/Schema";
import * as NodeServices from "@effect/platform-node/NodeServices";
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { afterEach, beforeEach, expect, vi } from "vitest";

import { makeFakeRunner } from "../support/fakeRunner.ts";
import {
  agentCredential, call, failureOf, harnessSettings, headSequence, logAfter, makeFakeSecrets, makeRig, ownerCredential,
  provideRig, readLog, removeRig, seedAccount, seedAgent, startPayload, success, type Fakes, type Rig,
} from "../support/handlerRig.ts";
import { registerRebuildCheck } from "../support/rebuildCheck.ts";
import { resolveHarnessDefaults } from "../../src/runner/harnesses.ts";

// Fix round 1, N-1 with A16 (owner choices, 2026-11-03): the service loads the
// harness defaults itself on its first start, `harnesses reset` loads them
// again on the owner's request, and an empty model or effort list means "not
// checked", never "refuse".

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

const decode = Schema.decodeUnknownSync(HarnessSettings);
const defaultClaude = decode(harnessSettings("first", { adapterPath: "/opt/adapters/claude", models: [], efforts: [] }));
const defaultCodex = decode(harnessSettings("second", { adapterPath: "/opt/adapters/codex", models: [], efforts: [] }));
const defaults = Effect.succeed([defaultClaude, defaultCodex] as ReadonlyArray<HarnessSettings>);

// One service start: a new layer over the same database file.
const serviceStart = <A, E, R>(
  options: { readonly defaults?: Effect.Effect<ReadonlyArray<HarnessSettings>> },
  body: (fakes: Fakes) => Effect.Effect<A, E, R>,
) => Effect.gen(function* () {
  const fakes: Fakes = {
    runner: yield* makeFakeRunner(), secrets: makeFakeSecrets(),
    ...(options.defaults === undefined ? {} : { harnessDefaults: options.defaults }),
  };
  return yield* provideRig(rig, fakes, Effect.gen(function* () {
    yield* registerRebuildCheck();
    return yield* body(fakes);
  }));
});

const listed = Effect.gen(function* () {
  return success(yield* call(ownerCredential, "harnesses", { operation: "list" })).harnesses as ReadonlyArray<HarnessSettings>;
});
const harnessEvents = readLog.pipe(Effect.map((rows) => rows.filter((row) => row.event._tag === "HarnessChanged")));

it.effect("N-1 first start: catches a service that leaves a fresh state folder with no harness settings, so the first start is refused", () =>
  serviceStart({ defaults }, (fakes) => Effect.gen(function* () {
    // No `harnesses set` anywhere in this test.
    expect(yield* listed).toEqual([defaultClaude, defaultCodex]);
    const events = yield* harnessEvents;
    expect(events.map((row) => row.caller)).toEqual([{ _tag: "owner" }, { _tag: "owner" }]);
    yield* seedAccount("primary", "first");
    const { model: _model, effort: _effort, ...withoutConfiguration } = startPayload(rig);
    const reply = success(yield* call(ownerCredential, "start", withoutConfiguration));
    expect(typeof reply.agentId).toBe("string");
    expect(fakes.runner.callsOf("start")).toHaveLength(1);
  })).pipe(Effect.orDie));

it.effect("N-1 first start: catches a first start that loads nothing for the harnesses it cannot find, or fails on them", () =>
  serviceStart({ defaults: Effect.succeed([defaultClaude]) }, () => Effect.gen(function* () {
    expect(yield* listed).toEqual([defaultClaude]);
  })).pipe(Effect.orDie));

it.effect("N-1 restart: catches a later start that loads the defaults over settings that already exist", () =>
  Effect.gen(function* () {
    const edited = decode(harnessSettings("first", { adapterPath: "/opt/adapters/edited", adapterArguments: ["--edited"], models: [], efforts: [] }));
    yield* serviceStart({ defaults }, () => Effect.gen(function* () {
      success(yield* call(ownerCredential, "harnesses", { operation: "set", harness: edited }));
    }));
    const before = yield* serviceStart({ defaults }, () => headSequence);
    // The defaults now differ and the owner's edit stands.
    const changed = Effect.succeed([decode({ ...defaultClaude, adapterPath: "/opt/adapters/other" })] as ReadonlyArray<HarnessSettings>);
    yield* serviceStart({ defaults: changed }, () => Effect.gen(function* () {
      expect(yield* headSequence).toBe(before);
      expect(yield* harnessEvents).toHaveLength(3);
      expect(yield* listed).toEqual([edited, defaultCodex]);
    }));
  }).pipe(Effect.orDie));

it.effect("N-1 restart: catches a start that loads the defaults beside settings the owner wrote by hand", () =>
  Effect.gen(function* () {
    // The first service start has no defaults to load; the owner sets one.
    yield* serviceStart({}, () => Effect.gen(function* () {
      success(yield* call(ownerCredential, "harnesses", { operation: "set", harness: defaultCodex }));
    }));
    yield* serviceStart({ defaults }, () => Effect.gen(function* () {
      expect(yield* listed).toEqual([defaultCodex]);
      expect(yield* harnessEvents).toHaveLength(1);
    }));
  }).pipe(Effect.orDie));

it.effect("N-1 reset: catches a reset that does not restore the defaults after an edit, or that touches a harness the defaults do not name", () =>
  serviceStart({ defaults }, () => Effect.gen(function* () {
    const edited = decode({ ...defaultClaude, adapterPath: "/opt/adapters/edited", defaultPermission: "full" });
    const custom = decode(harnessSettings("custom", { adapterPath: "/opt/adapters/custom" }));
    success(yield* call(ownerCredential, "harnesses", { operation: "set", harness: edited }));
    success(yield* call(ownerCredential, "harnesses", { operation: "set", harness: custom }));
    expect(yield* listed).toContainEqual(edited);
    const before = yield* headSequence;
    const reply = success(yield* call(ownerCredential, "harnesses", { operation: "reset" }));
    expect(reply.harnesses).toEqual([defaultClaude, defaultCodex, custom]);
    expect(yield* listed).toEqual([defaultClaude, defaultCodex, custom]);
    expect((yield* logAfter(before)).map((row) => row.event._tag)).toEqual(["HarnessChanged", "HarnessChanged"]);
  })).pipe(Effect.orDie));

it.effect("N-1 reset: catches a reset that succeeds on a service that has no defaults to load", () =>
  serviceStart({}, () => Effect.gen(function* () {
    const before = yield* headSequence;
    expect(failureOf(yield* call(ownerCredential, "harnesses", { operation: "reset" }))?.reason).toBe("harness defaults are unavailable");
    expect((yield* logAfter(before)).map((row) => row.event._tag)).toEqual(["CommandRefused"]);
  })).pipe(Effect.orDie));

it.effect("N-1 reset: catches a reset an agent can run", () =>
  serviceStart({ defaults }, () => Effect.gen(function* () {
    const { token } = yield* seedAgent({ id: "lead" });
    const before = yield* headSequence;
    expect(failureOf(yield* call(agentCredential(token), "harnesses", { operation: "reset" }))?._tag).toBe("CommandRefused");
    expect((yield* logAfter(before)).map((row) => row.event._tag)).toEqual(["CommandRefused"]);
  })).pipe(Effect.orDie));

it.effect("A16 empty lists: catches a start that refuses a model or an effort because the harness lists none", () =>
  serviceStart({}, (fakes) => Effect.gen(function* () {
    success(yield* call(ownerCredential, "harnesses", { operation: "set", harness: defaultClaude }));
    yield* seedAccount("primary", "first");
    const reply = success(yield* call(ownerCredential, "start", startPayload(rig, { model: "any-model-at-all", effort: "any-effort" })));
    expect(typeof reply.agentId).toBe("string");
    const started = fakes.runner.callsOf("start")[0]?.input;
    expect(started?.model).toBe("any-model-at-all");
    expect(started?.effort).toBe("any-effort");
  })).pipe(Effect.orDie));

it.effect("A16 filled lists: catches a start that accepts a model the harness does not list", () =>
  serviceStart({}, (fakes) => Effect.gen(function* () {
    success(yield* call(ownerCredential, "harnesses", { operation: "set", harness: decode(harnessSettings("first", { models: ["listed"], efforts: ["high"] })) }));
    yield* seedAccount("primary", "first");
    expect(failureOf(yield* call(ownerCredential, "start", startPayload(rig, { model: "unlisted", effort: "high" })))?._tag).toBe("CommandRefused");
    expect(fakes.runner.callsOf("start")).toHaveLength(0);
  })).pipe(Effect.orDie));

// Frozen factory shape from d4fb1877, runner/harnesses.ts. These are the
// actual four old mappings, arguments, deduplicated modes and empty choices.
const oldDefinitions = [
  ['claude', 'claude-agent-acp', [], ['plan', 'default', 'acceptEdits', 'bypassPermissions']],
  ['codex', 'codex-acp', [], ['read-only', 'read-only', 'workspace-write', 'agent-full-access']],
  ['gemini', 'gemini', ['--acp'], ['plan', 'default', 'autoEdit', 'yolo']],
  ['cursor', 'agent', ['acp'], ['ask', 'ask', 'agent', 'agent']],
] as const;
const oldFactory = (id: string, directory: string) => {
  const [harness, command, args, rawModes] = oldDefinitions.find(row => row[0] === id)!;
  const modes: ReadonlyArray<string> = rawModes;
  const levels = ['read-only', 'ask', 'edit', 'full'] as const;
  return decode({ id: harness, runMode: 'acp', adapterPath: join(directory, command), adapterArguments: [...args],
    permissionMapping: Object.fromEntries(levels.map((level, index) => [level, modes[index]])),
    reportedModes: levels.flatMap((level, index) => modes.indexOf(modes[index]!) === index ? [{ id: modes[index], level }] : []),
    defaultPermission: 'ask', models: [], efforts: [], denyReadMechanism: { _tag: 'none' } });
};
const currentFactories = () => {
  const binDirectory = join(rig.root, 'factory-adapters'); mkdirSync(binDirectory);
  for (const [, command] of oldDefinitions) writeFileSync(join(binDirectory, command), '#!/bin/sh\nexit 0\n', { mode: 0o700 });
  return resolveHarnessDefaults({ binDirectory, path: binDirectory }).pipe(Effect.provide(NodeServices.layer), Effect.map(result => result.harnesses));
};
for (const [id] of oldDefinitions) {
  it.effect(`s26 r3 M1: ${id} upgrades factory permission fields despite model, effort, adapter and run-mode edits`, () => Effect.gen(function* () {
    const factories = yield* currentFactories(), current = factories.find(row => row.id === id)!;
    const legacy = oldFactory(id, join(rig.root, 'factory-adapters'));
    const edited = decode({ ...legacy, adapterPath: '/opt/owner/' + id, defaultModel: 'owner-model', defaultEffort: 'owner-effort', runMode: 'interactive' });
    yield* serviceStart({}, () => Effect.gen(function* () {
      success(yield* call(ownerCredential, 'harnesses', { operation: 'set', harness: edited }));
      yield* seedAgent({ id: 'old-agent', permission: 'ask' });
    }));
    const effective = { ...edited, permissionMapping: current.permissionMapping, reportedModes: current.reportedModes, defaultPermission: 'auto' };
    yield* serviceStart({ defaults: Effect.succeed(factories) }, fakes => Effect.gen(function* () {
      expect(yield* listed).toEqual([effective]);
      expect(yield* harnessEvents).toHaveLength(1);
      yield* seedAccount('native-account', id);
      const payload: Record<string, unknown> = { ...startPayload(rig), harness: id, role: 'lead:project', account: { _tag: 'any' }, runMode: 'interactive' };
      delete payload.permission; delete payload.model; delete payload.effort;
      success(yield* call(ownerCredential, 'start', payload));
      expect(fakes.runner.callsOf('start').at(-1)!.input).toMatchObject({ permission: 'auto', harness: effective, model: 'owner-model', effort: 'owner-effort' });
      const parent = yield* seedAgent({ id: 'auto-parent', folder: rig.work, permission: 'auto' });
      success(yield* call(agentCredential(parent.token), 'start', { ...payload, name: 'Implicit worker', role: 'builder:typescript' }));
      expect(fakes.runner.callsOf('start').at(-1)!.input.permission).toBe('auto');
      expect((yield* readLog).find(row => row.event._tag === 'AgentStarted' && row.event.agentId === 'old-agent')!.event).toMatchObject({ permission: 'ask' });
    }));
  }).pipe(Effect.orDie));

  it.effect(`s26 r3 M1: ${id} keeps non-ask defaults and custom permission fields`, () => Effect.gen(function* () {
    const factories = yield* currentFactories(), current = factories.find(row => row.id === id)!;
    const legacy = oldFactory(id, join(rig.root, 'factory-adapters'));
    for (const permission of ['read-only', 'edit', 'full'] as const) {
      const saved = decode({ ...legacy, defaultPermission: permission, defaultModel: 'owner-model' });
      yield* serviceStart({}, () => call(ownerCredential, 'harnesses', { operation: 'set', harness: saved }));
      yield* serviceStart({ defaults: Effect.succeed(factories) }, () => Effect.gen(function* () {
        expect(yield* listed).toEqual([{ ...saved, permissionMapping: current.permissionMapping, reportedModes: current.reportedModes }]);
      }));
    }
    for (const override of [
      { permissionMapping: { ...legacy.permissionMapping, edit: 'owner-edit' } },
      { reportedModes: [...legacy.reportedModes, { id: 'owner-mode', level: 'ask' }] },
    ]) {
      const saved = decode({ ...legacy, ...override });
      yield* serviceStart({}, () => call(ownerCredential, 'harnesses', { operation: 'set', harness: saved }));
      yield* serviceStart({ defaults: Effect.succeed(factories) }, () => Effect.gen(function* () {
        expect(yield* listed).toEqual([{ ...saved, defaultPermission: 'auto' }]);
      }));
    }
  }).pipe(Effect.orDie));
}

it.effect('s26 r3 m4: listing and starting do not repeat installed-default discovery', () => Effect.gen(function* () {
  let probes = 0;
  const current = decode({ ...defaultClaude, defaultPermission: 'auto' });
  yield* serviceStart({ defaults: Effect.sync(() => { probes++; return [current]; }) }, () => Effect.gen(function* () {
    expect(yield* listed).toEqual([current]);
    yield* seedAccount('primary', 'first');
    success(yield* call(ownerCredential, 'start', startPayload(rig)));
    expect(yield* listed).toEqual([current]);
    expect(probes).toBe(1);
  }));
}).pipe(Effect.orDie));
