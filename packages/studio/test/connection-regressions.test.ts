import { afterEach, describe, expect, it, vi } from "vitest"
import { ClientError, type connect, type Client, type ClientEnvelope } from "@lgtm/client/browser"
import type { ClientSnapshot } from "@lgtm/contracts"
import * as Effect from "effect/Effect"
import * as Stream from "effect/Stream"
import { engineConnection, type StorePatch } from "../src/connection.ts"
import { engineSnapshot } from "./fixtures.ts"

const connections: ReturnType<typeof engineConnection>[] = []
afterEach(() => { for (const connection of connections.splice(0)) connection.close(); vi.useRealTimers() })

function harness(options: {
  snapshot?: ClientSnapshot; next?: ClientSnapshot; items?: unknown[]; logout?: number | "network";
  open?: typeof connect
} = {}) {
  let reads = 0
  const posts: string[] = []
  const state: StorePatch = {}
  const client = {
    "client.snapshot": () => Effect.succeed(++reads === 1 ? options.snapshot ?? engineSnapshot : options.next ?? engineSnapshot),
    "project.settings": () => Effect.succeed({}),
    "client.subscribe": () => Stream.concat(Stream.fromIterable((options.items ?? []) as ClientEnvelope[]), Stream.never)
  } as unknown as Client
  const connection = engineConnection({
    location: { protocol: "http:", host: "127.0.0.1:8787" }, debug() {},
    connect: options.open ?? (() => Effect.succeed(client)) as typeof connect,
    fetch: (async (path) => {
      posts.push(String(path))
      if (path === "/api/logout" && options.logout === "network") throw new Error("network")
      return { status: path === "/api/logout" ? options.logout ?? 204 : 204 }
    }) as typeof fetch
  })
  connections.push(connection)
  connection.open((patch) => Object.assign(state, patch))
  return { connection, state, posts, reads: () => reads }
}

describe("logout confirmation (findings 1 and 8)", () => {
  for (const result of [204, 403, 500, "network"] as const) it(`clears every mapped collection after real logout with ${result}`, async () => {
    const h = harness({ logout: result })
    await vi.waitFor(() => expect(h.state.projects?.length).toBe(2))
    ;(h.state as any).projectLeadSettings = { lead: { model: "private setting" } }
    await h.connection.signOut()
    expect((h.state as any).projectLeadSettings).toEqual({})
    expect(h.posts).toContain("/api/logout")
    for (const key of ["machines", "subscriptions", "repos", "projects", "teams", "agents", "requests", "worktrees", "sprints", "tasks"] as const) expect(h.state[key], key).toEqual([])
    expect(h.state.owner).toEqual({ name: "Owner", initials: "O" })
    expect(h.state.session?.status).toBe(result === 204 ? "out" : "logout-unconfirmed")
  })
})

describe("stream state (findings 3 and 4)", () => {
  for (const tag of ["AccountChanged", "AccountNeedsSignIn", "AccountAvailable", "AgentConfigurationChosen"]) {
    for (const oversized of [false, true]) it(`refreshes ${tag}${oversized ? " when oversized" : ""}`, async () => {
      const next = { ...engineSnapshot, sequence: 43, accounts: [{ id: "acct-main", enabled: false, status: "needs-sign-in", environment: [] }],
        agents: engineSnapshot.agents.map((a) => ({ ...a, model: "new-model", effort: "low" })) } as unknown as ClientSnapshot
      const item = oversized ? { _tag: "client.Oversized", sequence: 43, tag } : { sequence: 43, occurredAt: 1, event: { _tag: tag } }
      const h = harness({ next, items: [item] })
      await vi.waitFor(() => expect(h.reads()).toBe(2))
      expect(h.state.subscriptions?.[0]).toMatchObject({ enabled: false, signedIn: false })
      expect(h.state.agents?.[0]).toMatchObject({ model: "new-model", effort: "low" })
    })
  }
  it("recomputes activity when a signed-out agent resumes", async () => {
    const snapshot = { ...engineSnapshot, agents: engineSnapshot.agents.map((a) => a.agentId === "ag-lead" ? { ...a, state: { _tag: "paused", reason: "signed-out", midTurn: false }, displayedState: "Signed out" } : a) } as ClientSnapshot
    const h = harness({ snapshot, items: [{ sequence: 43, occurredAt: 1, event: { _tag: "StateChanged", agentId: "ag-lead", state: { _tag: "working" } } }] })
    await vi.waitFor(() => expect(h.state.agents?.[0]?.state).toBe("working"))
    // The board mapping shows the recomputed label. The stale one must be gone.
    expect(h.state.agents?.[0]?.activity).toBe("Working")
  })
})

it("becomes offline after 30 seconds of failed retries without inventing contact (finding 5)", async () => {
  vi.useFakeTimers()
  vi.setSystemTime(100_000)
  let attempts = 0
  const h = harness({ open: (() => { attempts++; return Effect.fail(new ClientError({ message: "unavailable" })) }) as unknown as typeof connect })
  await vi.advanceTimersByTimeAsync(35_000)
  expect(attempts).toBeGreaterThan(3)
  expect(h.state.conn?.status).toBe("offline")
  expect(h.state.conn?.lastHeard).toBe(0)
})

it("retains the last successful contact throughout an outage (finding 5)", async () => {
  vi.useFakeTimers()
  vi.setSystemTime(100_000)
  const client = { "client.snapshot": () => Effect.succeed(engineSnapshot), "project.settings": () => Effect.succeed({}),
    "client.subscribe": () => Stream.fail({ _tag: "TransportError" }) } as unknown as Client
  let attempts = 0
  const h = harness({ open: (() => ++attempts === 1 ? Effect.succeed(client) : Effect.fail(new ClientError({ message: "unavailable" }))) as typeof connect })
  await vi.advanceTimersByTimeAsync(35_000)
  expect(h.state.conn).toEqual({ status: "offline", lastHeard: 100_000 })
})

it("forgets records before logout returns and ignores an older session probe (findings 1 and 8)", async () => {
  let resolveProbe!: (value: { status: number }) => void
  let resolveLogout!: (value: { status: number }) => void
  let probes = 0
  const state: StorePatch = {}
  const client = { "client.snapshot": () => Effect.succeed(engineSnapshot), "project.settings": () => Effect.succeed({}), "client.subscribe": () => Stream.never } as unknown as Client
  const connection = engineConnection({
    location: { protocol: "http:", host: "127.0.0.1:8787" }, debug() {}, connect: (() => Effect.succeed(client)) as typeof connect,
    fetch: ((path: unknown) => {
      if (path === "/api/logout") return new Promise<{ status: number }>((resolve) => { resolveLogout = resolve })
      return ++probes === 1 ? Promise.resolve({ status: 204 }) : new Promise<{ status: number }>((resolve) => { resolveProbe = resolve })
    }) as typeof fetch
  })
  connections.push(connection)
  connection.open((patch) => Object.assign(state, patch))
  await vi.waitFor(() => expect(state.projects?.length).toBe(2))
  connection.reconnect()
  const signingOut = connection.signOut()
  expect(state.projects).toEqual([])
  resolveLogout({ status: 204 }); await signingOut
  resolveProbe({ status: 204 })
  await new Promise((resolve) => setTimeout(resolve, 30))
  expect(state.session?.status).toBe("out")
  expect(state.projects).toEqual([])
})


for (const tag of ["project.GitFinished", "project.GitEvidenceReported", "project.MainReviewOpened", "project.MainReviewDecided", "client.AlertRaised"]) {
  for (const oversized of [false, true]) it(`invalidates Git only for a Git event: ${tag}${oversized ? " oversized" : ""}`, async () => {
    const next = { ...engineSnapshot, sequence: 43 } as ClientSnapshot
    const item = oversized ? { _tag: "client.Oversized", sequence: 43, tag } : { sequence: 43, occurredAt: 1, event: { _tag: tag } }
    const h = harness({ next, items: [item] })
    await vi.waitFor(() => expect(h.reads()).toBe(2))
    expect(h.state.gitRevision).toBe(tag === "client.AlertRaised" ? 0 : 1)
  })
}


for (const tag of ["TurnEnded", "StateChanged"]) for (const oversized of [false, true]) {
  it(`maps Git activity for ${tag}${oversized ? " oversized" : ""}`, async () => {
    const next = { ...engineSnapshot, sequence: 43, agents: engineSnapshot.agents.map(a => a.agentId === "ag-lead" ? { ...a, state: { _tag: "working" } } : a) } as unknown as ClientSnapshot;
    const event = tag === "StateChanged" ? { _tag: tag, agentId: "ag-lead", state: { _tag: "working" } } : { _tag: tag, agentId: "ag-lead", compact: false };
    const item = oversized ? { _tag: "client.Oversized", sequence: 43, tag, agentId: "ag-lead" } : { sequence: 43, occurredAt: 1, event };
    const h = harness({ next, items: [item] });
    await vi.waitFor(() => expect(h.state.gitRevision).toBe(1));
    expect((h.state as any).gitChange).toEqual({ agentId: "ag-lead", ...(tag === "StateChanged" ? { scoped: true } : {}) });
  });
}

it("retains the mutation identity of a completion instead of its cleanup envelope's identity", async () => {
  const next = { ...engineSnapshot, sequence: 43 } as ClientSnapshot;
  const h = harness({ next, items: [{ sequence: 43, occurredAt: 1, requestId: "cleanup", event: { _tag: "project.GitFinished", operationRequestId: "original" } }] });
  await vi.waitFor(() => expect(h.state.gitRevision).toBe(1));
  expect((h.state as any).gitChange).toEqual({ requestId: "original" });
});

it("retains another review caller's request identity", async () => {
  const next = { ...engineSnapshot, sequence: 43 } as ClientSnapshot;
  const h = harness({ next, items: [{ sequence: 43, occurredAt: 1, requestId: "review-decision", event: { _tag: "project.MainReviewDecided" } }] });
  await vi.waitFor(() => expect(h.state.gitRevision).toBe(1));
  expect((h.state as any).gitChange).toEqual({ requestId: "review-decision" });
});

it("publishes an unchanged-sequence heartbeat for the Git fallback", async () => {
  const h = harness({ items: [{ _tag: "client.Checkpoint", sequence: 42 }] });
  await vi.waitFor(() => expect((h.state as any).gitHeartbeat).toBeTypeOf("number"));
  expect(h.state.gitRevision).toBe(0);
});

for (const oversized of [false, true]) for (const [before, after, refresh] of [
  ["idle", "working", true], ["working", "idle", true], ["idle", "paused", false], ["working", "working", false]
] as const) it(`scopes only working transitions ${before} to ${after}${oversized ? " oversized" : ""}`, async () => {
  const agents = (state: string) => engineSnapshot.agents.map(a => a.agentId === "ag-lead" ? { ...a, state: { _tag: state } } : a);
  const snapshot = { ...engineSnapshot, agents: agents(before) } as ClientSnapshot;
  const next = { ...engineSnapshot, sequence: 43, agents: agents(after) } as unknown as ClientSnapshot;
  const item = oversized ? { _tag: "client.Oversized", sequence: 43, tag: "StateChanged", agentId: "ag-lead" }
    : { sequence: 43, occurredAt: 1, event: { _tag: "StateChanged", agentId: "ag-lead", state: { _tag: after } } };
  const h = harness({ snapshot, next, items: [item] });
  await vi.waitFor(() => expect(h.state.agents?.[0]?.state).toBe(after === "paused" ? "paused" : after));
  expect(h.state.gitRevision).toBe(refresh ? 1 : 0);
  if (refresh) expect(h.state.gitChange).toEqual({ agentId: "ag-lead", scoped: true });
});
