// Criterion 7.4 (unit 7a; rows 52 step 1, 71, 74; PB-A9): `lgtm serve
// --detach` starts one service; two services started at once against one
// LGTM_HOME leave exactly one running and the other exits 0 reporting
// it; `serve --detach` detects a running service by connecting to the
// socket. Every service here is a real process
// (`packages/engine/src/service.ts`); `serveDetach` is the function behind
// `lgtm serve --detach` (`packages/cli/src/serve.ts`).
import { spawn, type ChildProcess } from "node:child_process";
import { existsSync, mkdirSync, writeFileSync, chmodSync, mkdtempSync, readdirSync, readFileSync, realpathSync, rmSync } from "node:fs";
import { createServer, request } from "node:http";
import { join } from "node:path";
import { DatabaseSync } from "node:sqlite";
import { fileURLToPath } from "node:url";

import { afterEach, expect, it } from "@effect/vitest";
import * as Effect from "effect/Effect";

import { connect } from "@lgtm/client";
import { randomUUID } from "node:crypto";

import { serveDetach } from "../src/serve.ts";

const servicePath = fileURLToPath(new URL("../../engine/src/service.ts", import.meta.url));

const roots: Array<string> = [];
const children: Array<ChildProcess> = [];

const makeRoot = () => {
  const root = realpathSync(mkdtempSync("/tmp/sh7s-"));
  roots.push(root);
  mkdirSync(join(root, "home"), { mode: 0o700 });
  return { root, state: join(root, "state"), socket: join(root, "state", "engine.sock") };
};
type Root = ReturnType<typeof makeRoot>;

const environmentFor = (root: Root): Record<string, string> => ({
  PATH: process.env.PATH ?? "/usr/bin:/bin", HOME: join(root.root, "home"), LGTM_HOME: root.state,
  XDG_STATE_HOME: join(root.root, "xdg"), CODEX_HOME: join(root.root, "codex"), TMPDIR: "/tmp", LANG: "C.UTF-8",
});

const readText = (path: string) => { try { return readFileSync(path, "utf8"); } catch { return ""; } };

// Live service processes for this root: the service script, with this root's state folder.
const servicesOf = (root: Root): Array<number> => readdirSync("/proc").filter((name) => /^\d+$/.test(name)).map(Number).filter((pid) => {
  const command = readText(`/proc/${pid}/cmdline`);
  const stat = readText(`/proc/${pid}/stat`);
  return command.includes(servicePath) && readText(`/proc/${pid}/environ`).includes(`LGTM_HOME=${root.state}\u0000`)
    && stat.slice(stat.lastIndexOf(")") + 2)[0] !== "Z";
});

const answers = (socket: string) => new Promise<boolean>((resolve) => {
  const call = request({ socketPath: socket, path: "/", method: "GET", timeout: 2_000 }, (response) => { response.resume(); resolve(true); });
  call.once("error", () => resolve(false));
  call.once("timeout", () => { call.destroy(); resolve(false); });
  call.end();
});

const waitFor = <A>(what: string, check: () => A | undefined | false | Promise<A | undefined | false>, limit = 30_000) => Effect.gen(function* () {
  const deadline = Date.now() + limit;
  for (;;) {
    const value = yield* Effect.promise(async () => await check());
    if (value !== undefined && value !== false) return value as A;
    if (Date.now() > deadline) return yield* Effect.die(new Error(`Timed out waiting for ${what}`));
    yield* Effect.sleep(25);
  }
});

const stopPid = async (pid: number) => {
  try { process.kill(pid, "SIGTERM"); } catch { return; }
  const deadline = Date.now() + 10_000;
  while (Date.now() < deadline) {
    const stat = readText(`/proc/${pid}/stat`);
    if (stat === "" || stat.slice(stat.lastIndexOf(")") + 2)[0] === "Z") return;
    await new Promise((resolve) => setTimeout(resolve, 25));
  }
  try { process.kill(pid, "SIGKILL"); } catch { /* gone */ }
};

// Every service this file started, directly or through serveDetach, is
// stopped; a pid is signalled only if it runs the service for one of this
// file's roots.
afterEach(async () => {
  for (const child of children.splice(0)) if (child.exitCode === null && child.signalCode === null) await stopPid(child.pid!);
  for (const root of roots.splice(0)) {
    for (const pid of servicesOf(makeRootView(root))) await stopPid(pid);
    rmSync(root, { recursive: true, force: true });
  }
});
const makeRootView = (root: string): Root => ({ root, state: join(root, "state"), socket: join(root, "state", "engine.sock") });

const spawnService = (root: Root) => {
  const child = spawn(process.execPath, [servicePath], { env: environmentFor(root), stdio: ["ignore", "pipe", "pipe"] });
  children.push(child);
  let stdout = "";
  child.stdout!.setEncoding("utf8").on("data", (chunk: string) => { stdout += chunk; });
  child.stderr!.resume();
  const exit = new Promise<{ code: number | null; signal: NodeJS.Signals | null }>((resolve) => child.once("close", (code, signal) => resolve({ code, signal })));
  return { child, exit, stdout: () => stdout };
};

const live = (name: string, test: () => Effect.Effect<unknown, unknown, any>, timeout = 90_000) =>
  it.live(name, () => test() as Effect.Effect<void, unknown, never>, timeout);

const statFields = (pid: number) => { const text = readText(`/proc/${pid}/stat`); return text.slice(text.lastIndexOf(")") + 2).split(" "); };

live("7.4 serve --detach starts exactly one service in its own session and returns once its socket answers (breaks if the service is not detached, the command returns before listening, or it starts more than one)", () =>
  Effect.gen(function* () {
    const root = makeRoot();
    const outcome = yield* serveDetach({ environment: environmentFor(root) });
    expect(outcome.outcome).toBe("started");
    const pid = (outcome as { pid: number }).pid;
    expect(yield* Effect.promise(() => answers(root.socket))).toBe(true);
    expect(servicesOf(root)).toEqual([pid]);
    // Detached: the service leads its own session and process group.
    const fields = statFields(pid);
    expect([Number(fields[2]), Number(fields[3])]).toEqual([pid, pid]);
  }));

live("7.4 serve --detach with a service already answering starts none, and says so (breaks if every call spawns a service, or a second service is left running)", () =>
  Effect.gen(function* () {
    const root = makeRoot();
    const first = yield* serveDetach({ environment: environmentFor(root) });
    expect(first.outcome).toBe("started");
    const again = yield* serveDetach({ environment: environmentFor(root) });
    expect(again.outcome).toBe("already running");
    expect(servicesOf(root)).toEqual([(first as { pid: number }).pid]);
  }));

live("7.4 serve --detach detects a service by connecting: a stale socket file is not a service, and anything answering on the socket is (breaks if detection reads the lock or the socket file's existence instead)", () =>
  Effect.gen(function* () {
    // A stale socket: its listener was killed, so the file stays and nothing answers.
    const stale = makeRoot();
    mkdirSync(stale.state, { mode: 0o700 });
    const holder = spawn(process.execPath, ["-e", `require("node:net").createServer().listen(${JSON.stringify(stale.socket)}, () => process.stdout.write("up"))`], { stdio: ["ignore", "pipe", "ignore"] });
    children.push(holder);
    yield* waitFor("the stale listener", () => existsSync(stale.socket));
    holder.kill("SIGKILL");
    yield* Effect.promise(() => new Promise((resolve) => holder.once("close", resolve)));
    expect(existsSync(stale.socket)).toBe(true);
    expect(yield* Effect.promise(() => answers(stale.socket))).toBe(false);
    const started = yield* serveDetach({ environment: environmentFor(stale) });
    expect(started.outcome).toBe("started");
    expect(yield* Effect.promise(() => answers(stale.socket))).toBe(true);

    // A listener that answers but holds no lock still counts as running.
    const answering = makeRoot();
    mkdirSync(answering.state, { mode: 0o700 });
    const server = createServer((_request, response) => { response.statusCode = 503; response.end("recovering"); });
    yield* Effect.promise(() => new Promise<void>((resolve) => server.listen(answering.socket, resolve)));
    try {
      const outcome = yield* serveDetach({ environment: environmentFor(answering) });
      expect(outcome.outcome).toBe("already running");
      expect(servicesOf(answering)).toEqual([]);
      expect(existsSync(join(answering.state, "engine.lock"))).toBe(false);
    } finally {
      yield* Effect.promise(() => new Promise((resolve) => server.close(resolve)));
    }
  }));

live("7.4 two services started at once against one LGTM_HOME leave exactly one running; the other prints 'already running' and exits 0, and the survivor holds an exclusive SQLite lock on engine.lock (breaks if both run, the loser exits non-zero, or the lock is not SQLite's exclusive lock on engine.lock)", () =>
  Effect.gen(function* () {
    for (let round = 0; round < 3; round++) {
      const root = makeRoot();
      const a = spawnService(root);
      const b = spawnService(root);
      const loser = yield* Effect.promise(() => Promise.race([a.exit.then(() => a), b.exit.then(() => b)]));
      const winner = loser === a ? b : a;
      const exit = yield* Effect.promise(() => loser.exit);
      expect([round, exit.code, exit.signal]).toEqual([round, 0, null]);
      expect(loser.stdout()).toContain("already running");
      yield* waitFor("the surviving service to answer", () => answers(root.socket));
      expect(winner.child.exitCode).toBeNull();
      expect(servicesOf(root)).toEqual([winner.child.pid]);
      // Rows 52 and 74: a raw connection with timeout 0 cannot take the lock: SQLITE_BUSY, error code 5.
      const probe = new DatabaseSync(join(root.state, "engine.lock"), { timeout: 0 });
      try {
        let code: unknown;
        try { probe.exec("BEGIN EXCLUSIVE"); probe.exec("ROLLBACK"); } catch (error) { code = (error as { errcode?: unknown }).errcode; }
        expect(code).toBe(5);
      } finally { probe.close(); }
    }
  }), 120_000);

live("7.4 two serve --detach calls at once leave exactly one service, one reporting it started and the other that one is running (breaks if both report started, or the loser's spawned service is left behind)", () =>
  Effect.gen(function* () {
    const root = makeRoot();
    const outcomes = yield* Effect.all([serveDetach({ environment: environmentFor(root) }), serveDetach({ environment: environmentFor(root) })], { concurrency: 2 });
    expect(outcomes.map((outcome: { outcome: string }) => outcome.outcome).sort()).toEqual(["already running", "started"]);
    const started = outcomes.find((outcome: { outcome: string }) => outcome.outcome === "started") as { pid: number };
    yield* waitFor("only the started service to remain", () => {
      const pids = servicesOf(root);
      return pids.length === 1 && pids[0] === started.pid;
    }, 15_000);
  }));

const runDetach = (environment: Record<string, string>) => new Promise<{ code: number | null; text: string }>((resolve) => {
  const child = spawn(process.execPath, [fileURLToPath(new URL('../src/bin.ts', import.meta.url)), 'serve', '--detach'], { env: environment, stdio: ['ignore', 'pipe', 'pipe'] });
  children.push(child);
  let text = '';
  child.stdout!.on('data', chunk => { text += chunk });
  child.stderr!.on('data', chunk => { text += chunk });
  child.once('close', code => resolve({ code, text }));
});

it('reports refused state permissions and redacts a matching web password', async () => {
  const root = makeRoot();
  mkdirSync(root.state, { mode: 0o750 }); chmodSync(root.state, 0o750);
  const first = await runDetach(environmentFor(root));
  expect(first.code).toBe(1);
  expect(first.text).toContain('state folder mode is not 0700');
  const secret = 'state folder mode is not 0700';
  const redacted = await runDetach({ ...environmentFor(root), LGTM_WEB_PASSWORD: secret, API_TOKEN: 's17-private-token' });
  expect(redacted.code).toBe(1);
  expect(redacted.text).toContain('[REDACTED]');
  expect(redacted.text).not.toContain(secret);
  expect(redacted.text).not.toContain('s17-private-token');
  expect(redacted.text.length).toBeLessThan(1024);
});

it('reports a state folder that cannot be created without exposing paths or contents', async () => {
  const root = makeRoot();
  writeFileSync(root.state, 's17-private-file-content');
  const result = await runDetach({ ...environmentFor(root), LGTM_HOME: join(root.state, 'missing') });
  expect(result.code).toBe(1);
  expect(result.text).toContain('state folder cannot be inspected');
  expect(result.text).not.toContain(root.state);
  expect(result.text).not.toContain('s17-private-file-content');
});

live('keeps the engine and its outage alert running when the configured web port is occupied', () => Effect.gen(function* () {
  const root = makeRoot(), env = environmentFor(root);
  // Start headless to persist an ephemeral port through the authenticated API.
  const first = yield* serveDetach({ environment: env });
  yield* waitFor('the authenticated API to finish recovery', () => new Promise<boolean>(resolve => {
    const probe = request({ socketPath: root.socket, path: '/' }, response => { response.resume(); resolve(response.statusCode !== 503) });
    probe.once('error', () => resolve(false)); probe.end();
  }));
  const server = createServer();
  yield* Effect.promise(() => new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve)));
  try {
    const port = (server.address() as import('node:net').AddressInfo).port;
    yield* Effect.scoped(Effect.gen(function* () {
      const client = yield* connect({ environment: env });
      yield* client['client.web.settings.set']({ requestId: randomUUID() as never, settings: { enabled: true, port, heartbeatSeconds: 25, proxyMode: false } });
    }));
    yield* Effect.promise(() => stopPid((first as { pid: number }).pid));
    const result = yield* Effect.promise(() => runDetach({ ...env, LGTM_WEB_PASSWORD: 's17-test-password' }));
    expect(result.code).toBe(0);
    expect(result.text).toMatch(/^started \(pid \d+\)\n$/);
    yield* waitFor('the API and web outage alert', async () => {
      try {
        return await Effect.runPromise(Effect.scoped(Effect.gen(function* () {
          const client = yield* connect({ environment: env });
          const state = yield* client['client.web.settings']({ requestId: randomUUID() as never });
          const alerts = yield* client['client.alerts.list']({ requestId: randomUUID() as never, after: 0 as never, limit: 200 });
          return state.serverState === 'port-taken' && alerts.items.some(alert => alert.kind === 'client.web-down');
        })));
      } catch { return false; }
    });
    const db = new DatabaseSync(join(root.state, 'events.sqlite'), { readOnly: true });
    try { expect(db.prepare("SELECT COUNT(*) AS count FROM events WHERE event_type = 'client.WebServerDown'").get()!.count).toBe(1); }
    finally { db.close(); }
    expect(result.text).not.toContain('s17-test-password');
  } finally {
    yield* Effect.promise(() => new Promise<void>(resolve => server.close(() => resolve())));
  }
}));

it('reports unreadable state permissions and a refused long socket address', async () => {
  const root = makeRoot();
  mkdirSync(root.state, { mode: 0o700 }); chmodSync(root.state, 0o000);
  try {
    const unreadable = await runDetach(environmentFor(root));
    expect(unreadable.code).toBe(1);
    expect(unreadable.text).toContain('state folder mode is not 0700');
  } finally { chmodSync(root.state, 0o700); }
  const tooLong = await runDetach({ ...environmentFor(root), LGTM_HOME: join(root.state, 'long-address-'.repeat(10)) });
  expect(tooLong.code).toBe(1);
  expect(tooLong.text).toContain('socket path exceeds 100 bytes');
  expect(tooLong.text).not.toContain(root.state);
});

// Loader hooks affect only this test's own child. No production test switch is added.
const serviceHook = (root: Root, from: string, to: string) => {
  const loader = join(root.root, 'service-hook.mjs');
  writeFileSync(loader, `import { registerHooks } from 'node:module';
registerHooks({ load(url, context, next) {
 const loaded = next(url, context);
 if (url === ${JSON.stringify(new URL('../../engine/src/service.ts', import.meta.url).href)}) {
  const source = String(loaded.source);
  if (!source.includes(${JSON.stringify(from)})) throw new Error('Service test hook did not match');
  return { ...loaded, source: source.replace(${JSON.stringify(from)}, ${JSON.stringify(to)}) };
 }
 return loaded;
} });
`);
  return { ...environmentFor(root), NODE_OPTIONS: `--import=${new URL('file://' + loader).href}` };
};

it('prints started and exits before recovery exceeds the launcher deadline, leaving the service alive', async () => {
  const root = makeRoot(), marker = join(root.root, 'recovery-finished');
  const env = serviceHook(root, 'yield* (yield* Restart.Restart).recover;',
    `yield* Effect.sleep(31_000); (yield* Effect.promise(() => import('node:fs'))).writeFileSync(${JSON.stringify(marker)}, 'done'); yield* (yield* Restart.Restart).recover;`);
  const result = await runDetach(env);
  expect(result.code).toBe(0);
  expect(result.text).toMatch(/^started \(pid \d+\)\n$/);
  expect(existsSync(marker)).toBe(false);
  await new Promise(resolve => setTimeout(resolve, 31_000));
  await Effect.runPromise(waitFor('recovery beyond the launch deadline', () => existsSync(marker)));
  expect(servicesOf(root)).toHaveLength(1);
  expect(await answers(root.socket)).toBe(true);
}, 90_000);

it('keeps serving when its launcher disconnects just before the lock-acquired send', async () => {
  const root = makeRoot();
  const env = serviceHook(root, 'if (process.connected) process.send?.({ _tag: "LgtmLockAcquired" }, () => {});',
    `yield* Effect.promise(() => new Promise(resolve => { process.send?.({ _tag: 'TestBeforeLockSend' }, () => {}); process.once('disconnect', resolve); })); if (process.connected) process.send?.({ _tag: "LgtmLockAcquired" }, () => {});`);
  const child = spawn(process.execPath, [servicePath], { env, stdio: ['ignore', 'ignore', 'ignore', 'ipc'] });
  children.push(child);
  await new Promise<void>(resolve => child.on('message', message => { if ((message as { _tag?: string })._tag === 'TestBeforeLockSend') { child.disconnect(); resolve(); } }));
  await Effect.runPromise(waitFor('the orphaned service socket', () => child.exitCode !== null ? true : answers(root.socket)));
  expect(child.exitCode).toBeNull();
  expect(await answers(root.socket)).toBe(true);
});
