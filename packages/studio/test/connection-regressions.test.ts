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

for (const [error, suffix] of [
  [{ _tag: 'CommandError', requestId: 'snapshot-1', reason: 'command refused' }, ' command refused'],
    [{ _tag: 'CommandError', requestId: 'snapshot-1', reason: 'x'.repeat(1000) }, '']
] as const) it(`shows a bounded snapshot refusal, keeps retrying and clears it after recovery (${suffix || 'generic'})`, async () => {
  vi.useFakeTimers()
  let reads = 0
  const client = { 'client.snapshot': () => ++reads < 3 ? Effect.fail(error) : Effect.succeed(engineSnapshot),
    'project.settings': () => Effect.succeed({}), 'client.subscribe': () => Stream.never } as unknown as Client
  const h = harness({ open: (() => Effect.succeed(client)) as typeof connect })
  await vi.advanceTimersByTimeAsync(0)
  expect(h.state.conn).toMatchObject({ status: 'reconnecting', notice: 'The service could not send its data.' + suffix })
  expect(h.state.conn?.lastHeard).toBe(0)
  await vi.advanceTimersByTimeAsync(250)
  expect(reads).toBe(2)
  expect(h.state.conn).toHaveProperty('notice')
  await vi.advanceTimersByTimeAsync(500)
  expect(reads).toBe(3)
  expect(h.state.conn?.status).toBe('live')
  expect(h.state.conn).not.toHaveProperty('notice')
})

it('keeps the existing connection notice on a snapshot transport drop', async () => {
  vi.useFakeTimers()
  const client = { 'client.snapshot': () => Effect.fail(new ClientError({ message: 's17-private-transport-detail' })) } as unknown as Client
  const h = harness({ open: (() => Effect.succeed(client)) as typeof connect })
  await vi.advanceTimersByTimeAsync(35_000)
  expect(h.state.conn).toEqual({ status: 'offline', lastHeard: 0 })
})

it('keeps a snapshot refusal visible throughout the next retry and session probe', async () => {
  vi.useFakeTimers()
  let probes = 0
  let finishProbe!: (response: Response) => void
  const state: StorePatch = {}, patches: StorePatch[] = []
  const client = { 'client.snapshot': () => Effect.fail({ _tag: 'CommandError', requestId: 'snapshot-1', reason: 'command refused' }) } as unknown as Client
  const connection = engineConnection({ location: { protocol: 'http:', host: '127.0.0.1:12345' }, debug() {},
    connect: (() => Effect.succeed(client)) as typeof connect,
    fetch: (async () => ++probes < 3 ? new Response(null, { status: 204 }) : new Promise<Response>(resolve => { finishProbe = resolve })) as typeof fetch })
  connections.push(connection); connection.open(patch => { patches.push(patch); Object.assign(state, patch) })
  await vi.advanceTimersByTimeAsync(0)
  expect(state.conn?.notice).toBe('The service could not send its data. command refused')
  patches.length = 0
  await vi.advanceTimersByTimeAsync(250)
  expect(probes).toBe(3)
  expect(state.conn?.notice).toBe('The service could not send its data. command refused')
  finishProbe(new Response(null, { status: 204 }))
  await vi.advanceTimersByTimeAsync(0)
  expect(patches.filter(patch => patch.conn !== undefined).every(patch => patch.conn?.notice === 'The service could not send its data. command refused')).toBe(true)
})

it('removes an earlier snapshot refusal when the next attempt loses its transport', async () => {
  vi.useFakeTimers()
  let reads = 0
  const client = { 'client.snapshot': () => Effect.fail(++reads === 1
    ? { _tag: 'CommandError', requestId: 'snapshot-1', reason: 'command refused' }
    : new ClientError({ message: 'connection lost' })) } as unknown as Client
  const h = harness({ open: (() => Effect.succeed(client)) as typeof connect })
  await vi.advanceTimersByTimeAsync(0)
  expect(h.state.conn?.notice).toBe('The service could not send its data. command refused')
  await vi.advanceTimersByTimeAsync(250)
  expect(h.state.conn).toEqual({ status: 'reconnecting', lastHeard: 0 })
})

it('keeps snapshot authorization failures on the existing expired-session path', async () => {
  vi.useFakeTimers()
  let probes = 0
  const state: StorePatch = {}, patches: StorePatch[] = []
  const client = { 'client.snapshot': () => Effect.fail({ _tag: 'CommandError', requestId: 'snapshot-1', reason: 'command refused' }) } as unknown as Client
  const connection = engineConnection({ location: { protocol: 'http:', host: '127.0.0.1:12345' }, debug() {},
    connect: (() => Effect.succeed(client)) as typeof connect,
    fetch: (async () => new Response(null, { status: ++probes === 1 ? 204 : 401 })) as typeof fetch })
  connections.push(connection); connection.open(patch => { patches.push(patch); Object.assign(state, patch) })
  await vi.advanceTimersByTimeAsync(1000)
  expect(patches.some(patch => patch.conn?.notice !== undefined)).toBe(false)
  expect(state.session?.status).toBe('expired')
  expect(state.projects).toEqual([])
  expect(state.conn).not.toHaveProperty('notice')

})

it('per-call cancellation interrupts only that RPC and reports its transport identifier', async () => {
  let interrupted = 0, dispatched = 0
  const client = {
    'client.snapshot': () => Effect.succeed(engineSnapshot),
    'project.settings': () => Effect.succeed({}),
    'client.subscribe': () => Stream.never,
    'project.git.state': () => { dispatched++; return Effect.never.pipe(Effect.ensuring(Effect.sync(() => { interrupted++ }))) }
  } as unknown as Client
  const h = harness({ open: (() => Effect.succeed(client)) as typeof connect })
  await vi.waitFor(() => expect(h.state.conn?.status).toBe('live'))
  const abort = new AbortController(), ids: string[] = []
  const reply = h.connection.request('project.git.state', {}, { signal: abort.signal, onRequestId: (id) => ids.push(id) }).catch(() => 'aborted')
  await vi.waitFor(() => expect(dispatched).toBe(1))
  expect(ids).toHaveLength(1)
  abort.abort()
  expect(await reply).toBe('aborted')
  expect(interrupted).toBe(1)
  expect(await h.connection.request('project.settings', {})).toEqual({})
  await expect(h.connection.request('project.git.state', {}, { signal: abort.signal })).rejects.toBeDefined()
  expect(dispatched).toBe(1)
})

for (const state of ['stopped', 'crashed', 'starting'] as const) it(`refreshes offered questions across a ${state} transition`, async () => {
  const snapshot = { ...engineSnapshot, agents: engineSnapshot.agents.map(a => a.agentId === 'ag-lead' ? { ...a, state: { _tag: state === 'starting' ? 'stopped' : 'working' } } : a) } as ClientSnapshot
  const next = { ...snapshot, sequence: 43, questions: [], agents: snapshot.agents.map(a => a.agentId === 'ag-lead' ? { ...a, state: { _tag: state }, openQuestionsAsked: 0 } : a) } as unknown as ClientSnapshot
  const h = harness({ snapshot, next, items: [{ sequence: 43, occurredAt: 1, event: { _tag: 'StateChanged', agentId: 'ag-lead', state: { _tag: state } } }] })
  await vi.waitFor(() => expect(h.reads()).toBe(2))
  expect(h.state.requests?.some(r => r.kind === 'question')).toBe(false)
})
