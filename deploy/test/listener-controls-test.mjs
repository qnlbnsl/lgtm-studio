// Run with the repository's Node.js version: node deploy/test/listener-controls-test.mjs
import assert from 'node:assert/strict';
import childProcess from 'node:child_process';
import { writeFileSync } from 'node:fs';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { createServer } from 'node:http';
import { registerHooks, syncBuiltinESMExports } from 'node:module';
import net from 'node:net';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { promisify } from 'node:util';

const here = fileURLToPath(new URL('.', import.meta.url));
const run = promisify(childProcess.execFile);
const host = '127.0.0.1';

function controlBrowser(control, scratch) {
  // Exercise the normal modes entry point without building or launching a browser.
  const modules = {
    vite: 'export async function build() {}',
    '@playwright/test': `export const chromium = { async launch() {
      return {
        async newContext() { throw new Error('Browser checks started'); },
        async close() {},
      };
    } };`,
  };
  registerHooks({
    resolve(specifier, context, next) {
      if (Object.hasOwn(modules, specifier)) {
        return { url: 'data:text/javascript,' + encodeURIComponent(modules[specifier]), shortCircuit: true };
      }
      return next(specifier, context);
    },
  });

  let planted;
  let requests = 0;
  let mockStarts = 0;
  let mockStderr = '';
  const listen = net.createServer;
  net.createServer = (...args) => {
    net.createServer = listen;
    syncBuiltinESMExports();
    const probe = listen(...args);
    const address = probe.address.bind(probe);
    if (control === 'reserved') {
      probe.address = () => ({ ...address(), port: 8787 });
    } else {
      const close = probe.close.bind(probe);
      probe.close = callback => {
        const port = address().port;
        return close(error => {
          if (error) return callback(error);
          assert.notEqual(port, 8787);
          // Hold the close callback until this run's listener owns the released port.
          planted = createServer((_request, response) => {
            requests++;
            response.writeHead(200, { 'Content-Type': 'application/json' });
            response.end('{"sample":true}');
          });
          planted.once('error', callback);
          planted.listen(port, host, () => callback());
          planted.unref();
        });
      };
    }
    return probe;
  };

  const spawn = childProcess.spawn;
  childProcess.spawn = (command, args, options) => {
    if (args[0] !== 'src/serve.ts') return spawn(command, args, options);
    mockStarts++;
    assert.notEqual(options.env.LGTM_MOCK_PORT, '8787', 'The helper tried to start a mock on port 8787');
    const child = spawn(command, args, { ...options, stdio: [...options.stdio.slice(0, 2), 'pipe'] });
    child.stderr.setEncoding('utf8').on('data', chunk => { mockStderr += chunk; });
    return child;
  };
  syncBuiltinESMExports();

  const fetch = globalThis.fetch;
  globalThis.fetch = (url, options) => {
    // On a regression, the only permitted destination is the listener this test started.
    assert.ok(planted?.listening && new URL(url).origin === `http://${host}:${planted.address().port}`);
    return fetch(url, options);
  };
  process.once('exit', () => {
    writeFileSync(join(scratch, 'observed.json'), JSON.stringify({
      mockStarts, requests, plantedAlive: planted?.listening ?? false,
      bindFailed: mockStderr.includes('EADDRINUSE'),
    }));
  });
}

function runControl(command, args, env) {
  return run(command, args, { env, timeout: 30_000 }).then(
    result => ({ ...result, code: 0 }),
    error => error,
  );
}

async function browserControl(scratch, control, expected) {
  const env = { ...process.env, LGTM_LISTENER_CONTROL: control, LGTM_LISTENER_SCRATCH: scratch };
  const exit = await runControl(process.execPath, [
    '--import', fileURLToPath(import.meta.url), join(here, 'checks-s4-browser.mjs'),
    'modes', 'http://unused.invalid', scratch,
  ], env);
  const observed = JSON.parse(await readFile(join(scratch, 'observed.json'), 'utf8'));
  assert.deepEqual(observed, expected);
  const stage = await readFile(join(scratch, 'browser-stage.log'), 'utf8');
  assert.equal(exit.code, 22, stage);
}

async function postgresControl(scratch) {
  const fixture = join(scratch, 'docker.bash');
  const observed = join(scratch, 'docker-observed');
  await writeFile(fixture, `
docker() {
    case "$1 $2" in
        'volume create') printf '%s\\n' "$5" ;;
        'volume rm') ;;
        'run --detach')
            local flag
            while (( $# )); do
                flag=$1
                shift
                if [[ $flag == --publish ]]; then
                    printf 'publish %s\\n' "$1" >> "$LISTENER_CONTROL_OBSERVED"
                    break
                fi
            done
            printf '%064d\\n' 1 ;;
        'ps -aq') printf '%064d\\n' 1 ;;
        'rm -f') ;;
        'exec -i') cat >/dev/null; printf '1\\n' ;;
        'port '*)
            printf 'lookup %s %s\\n' "$2" "$3" >> "$LISTENER_CONTROL_OBSERVED"
            printf '127.0.0.1:8787\\n' ;;
        *) return 99 ;;
    esac
}
# Keep timed Docker calls inside this fixture.
timeout() {
    shift
    "$@"
}
`);
  await run('bash', ['-n', fixture]);
  const marker = join(scratch, 'wrapped-command-ran');
  const exit = await runControl('bash', [
    join(here, 'with-postgres.sh'), '--', 'bash', '-c', 'touch -- "$1"', '_', marker,
  ], { ...process.env, BASH_ENV: fixture, LISTENER_CONTROL_OBSERVED: observed });
  await assert.rejects(readFile(marker), { code: 'ENOENT' }, 'The wrapped command ran with a reserved port');
  const [publish, lookup] = (await readFile(observed, 'utf8')).trim().split('\n');
  assert.match(publish, /^publish 127\.0\.0\.1:(?!8787:)\d+:5432$/);
  assert.equal(lookup, 'lookup ' + '1'.padStart(64, '0') + ' 5432/tcp');
  assert.equal(exit.code, 1, exit.stderr || exit.message);
}

async function main() {
  const controls = [
    ['browser rejects probe-selected port 8787', scratch => browserControl(scratch, 'reserved', {
      mockStarts: 0, requests: 0, plantedAlive: false, bindFailed: false,
    })],
    ['PostgreSQL rejects a reported binding on port 8787 before running the command', postgresControl],
    ['browser leaves the planted listener running with zero requests', scratch => browserControl(scratch, 'collision', {
      mockStarts: 1, requests: 0, plantedAlive: true, bindFailed: true,
    })],
  ];

  for (const [index, [description, control]] of controls.entries()) {
    const scratch = await mkdtemp(join(tmpdir(), 'lgtm-listener-control-'));
    try {
      await control(scratch);
      console.log(`ok ${index + 1} - ${description}`);
    } catch (error) {
      console.log(`not ok ${index + 1} - ${description}`);
      console.error(error);
      process.exitCode = 1;
    } finally {
      await rm(scratch, { recursive: true, force: true });
    }
  }
}

if (process.env.LGTM_LISTENER_CONTROL) {
  controlBrowser(process.env.LGTM_LISTENER_CONTROL, process.env.LGTM_LISTENER_SCRATCH);
} else {
  await main();
}
