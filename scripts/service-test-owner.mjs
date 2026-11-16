// Test-only ownership follows services through launchers, explicit environments,
// detached groups and execve. A service signals itself, never a recycled PID.
import childProcess from 'node:child_process';
import { syncBuiltinESMExports } from 'node:module';
import { existsSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { setTimeout as delay } from 'node:timers/promises';

const identity = (pid) => {
  try {
    const fields = readFileSync(`/proc/${pid}/stat`, 'utf8').split(') ').at(-1).split(' ');
    return fields[0] === 'Z' ? undefined : fields[19];
  } catch { return undefined; }
};
let owner;
try { owner = JSON.parse(process.env.LGTM_TEST_OWNER || 'null'); } catch {}
const originalSpawn = childProcess.spawn, originalFork = childProcess.fork;
const guarded = (options = {}, command, args = []) => {
  if (!owner) return options;
  const env = { ...(options.env ?? process.env) };
  const serviceLaunch = [command, ...args].some(arg => typeof arg === 'string' && arg.endsWith('/engine/src/service.ts'))
    || [command, ...args].some(arg => typeof arg === 'string' && arg.endsWith('/cli/src/bin.ts')) && args.includes('serve');
  // The runner has already applied its credential allow-list to agent children.
  // Services own those children through their existing scoped runner teardown.
  if (typeof env.LGTM_SOCKET === 'string' && !serviceLaunch) return options;
  env.LGTM_TEST_OWNER = JSON.stringify(owner);
  const hook = `--import=${import.meta.url}`;
  env.NODE_OPTIONS = [hook, env.NODE_OPTIONS].filter(Boolean).join(' ');
  return { ...options, env };
};
childProcess.spawn = function (command, args, options) {
  return Array.isArray(args) ? originalSpawn.call(this, command, args, guarded(options, command, args)) : originalSpawn.call(this, command, guarded(args, command));
};
childProcess.fork = function (module, args, options) {
  return Array.isArray(args) ? originalFork.call(this, module, args, guarded(options, module, args)) : originalFork.call(this, module, guarded(args, module));
};
syncBuiltinESMExports();

const isService = process.argv[1]?.endsWith('/engine/src/service.ts');
const isLauncher = process.argv[1]?.endsWith('/cli/src/bin.ts') && process.argv[2] === 'serve';
if (owner && (isService || isLauncher)) {
  const birth = identity(process.pid);
  const record = join(owner.root, `${process.pid}-${birth}`);
  let stopping = false;
  const check = () => {
    if (stopping) return;
    if (existsSync(join(owner.root, 'active')) && identity(owner.pid) === owner.birth) return;
    stopping = true;
    // SIGKILL cannot run an afterAll hook. This watcher lives in the owned
    // process, so it also closes services when a test worker is interrupted.
    setTimeout(() => process.kill(process.pid, 'SIGKILL'), 5000).unref();
    process.kill(process.pid, 'SIGTERM');
  };
  try { writeFileSync(record, '', { mode: 0o600 }); } catch {}
  const timer = setInterval(check, 100).unref();
  process.once('exit', () => { clearInterval(timer); try { rmSync(record); } catch {} });
  check();
}

export function installServiceTestOwner() {
  const root = mkdtempSync(join(tmpdir(), 'lgtm-test-owner-'));
  owner = { pid: process.pid, birth: identity(process.pid), root };
  writeFileSync(join(root, 'active'), '', { mode: 0o600 });
  return { async close() {
    rmSync(join(root, 'active'), { force: true });
    const deadline = Date.now() + 10000;
    for (;;) {
      const live = readdirSync(root).some(name => {
        const [pid, birth] = name.split('-');
        return identity(Number(pid)) === birth;
      });
      if (!live) break;
      if (Date.now() >= deadline) throw new Error('A test-owned service did not stop after teardown.');
      await delay(25);
    }
    rmSync(root, { recursive: true, force: true });
  } };
}
