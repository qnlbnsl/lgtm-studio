// The sample data of the mock server. It is a design aid: the records are invented.
import { create } from '@bufbuild/protobuf';
import { timestampFromMs } from '@bufbuild/protobuf/wkt';
import { PreviewSchema, PreviewState, type Preview } from '@lgtm/api/preview';
import { ClientSchema, ClientState, type Client } from '@lgtm/api/clients';

const MIN = 60_000, HOUR = 60 * MIN, DAY = 24 * HOUR;

/** The three clients every sample starts with. `now` is the moment the server started. */
export function sampleClients(now: number): Client[] {
  const both = [{ harness: 'claude', version: '2.4.1' }, { harness: 'codex', version: '0.61.0' }];
  return [
    create(ClientSchema, {
      id: 'studio', name: 'Workstation', state: ClientState.ONLINE, hostsServer: true, address: '127.0.0.1:8787', workspace: '~/projects', os: 'Linux',
      version: '0.9.0', agentLimit: 16, cores: 16, memoryGigabytes: 64, cpuPercent: 46, memoryPercent: 62, lastSeen: timestampFromMs(now),
      connectedAt: timestampFromMs(now - 6 * DAY), harnessesScanned: true, harnesses: both,
    }),
    create(ClientSchema, {
      id: 'buildbox', name: 'Buildbox', state: ClientState.ONLINE, address: '192.168.1.24', workspace: '~/Work', os: 'Linux',
      version: '0.9.0', agentLimit: 8, cores: 24, memoryGigabytes: 128, cpuPercent: 21, memoryPercent: 38, lastSeen: timestampFromMs(now - 20_000),
      connectedAt: timestampFromMs(now - (2 * DAY + 4 * HOUR)), harnessesScanned: true, harnesses: both,
    }),
    create(ClientSchema, {
      id: 'laptop', name: 'Laptop', state: ClientState.OFFLINE, address: '192.168.1.31', workspace: '~/code', os: 'Linux',
      version: '0.8.4', agentLimit: 4, cores: 8, memoryGigabytes: 16, lastSeen: timestampFromMs(now - (2 * HOUR + 12 * MIN)),
      harnessesScanned: true, harnesses: [{ harness: 'claude', version: '2.3.8' }, { harness: 'codex' }],
    }),
  ];
}

/** The machine that uses a new token in the sample, a few seconds after the token is made. */
export function sampleJoiner(id: string, tokenId: string, now: number): Client {
  return create(ClientSchema, {
    id, name: 'mini.lan', state: ClientState.WAITING_FOR_APPROVAL, address: '192.168.1.52', workspace: '~/code', os: 'macOS',
    version: '0.9.0', agentLimit: 4, cores: 10, memoryGigabytes: 32, lastSeen: timestampFromMs(now), connectedAt: timestampFromMs(now), tokenId,
  });
}

/**
 * The page the sample shows as a codebase map when it has no real one.
 * A real map is built by Floorplan, and the mock server shows one when LGTM_MOCK_MAP names its folder.
 */
export const PLACEHOLDER_MAP = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'">
<title>Sample map</title>
<style>
  :root { color-scheme: light dark; font: 14px/1.5 system-ui, sans-serif; }
  body { margin: 0; min-height: 100vh; display: grid; place-content: center; gap: 24px; padding: 24px; box-sizing: border-box; }
  .parts { display: grid; grid-template-columns: repeat(3, 150px); gap: 12px; }
  .part { border: 1px solid color-mix(in srgb, currentColor 30%, transparent); border-radius: 8px; padding: 12px; }
  .part b { display: block; } .part span { opacity: .65; font-size: 12px; }
  p { margin: 0; opacity: .65; text-align: center; }
</style></head>
<body>
  <div class="parts">
    <div class="part"><b>Website</b><span>41 files</span></div>
    <div class="part"><b>Server</b><span>128 files</span></div>
    <div class="part"><b>Client</b><span>37 files</span></div>
  </div>
  <p>A placeholder for a map built by Floorplan.</p>
</body></html>`;

/** The counts that go with the placeholder map. It was "built" three hours before the sample started. */
export const PLACEHOLDER_FACTS = (now: number) => ({ generatedAt: now - 3 * 3_600_000, files: 206, lines: 48_210, parts: 3, links: 412 });

// ---- the running product ------------------------------------------------------------

/** The previews of one project in the sample: a website that runs, and a phone app that is stopped. */
export function samplePreviews(place: string, now: number): Preview[] {
  // A place is a project's id, or "repository:" and a repository's id for its main checkout.
  const main = place.startsWith('repository:'); const projectId = main ? '' : place; const repositoryId = main ? place.slice(11) : '';
  const one = (key: string, name: string, command: string, port: number, state: PreviewState): Preview => create(PreviewSchema, {
    id: `pv-${place.replace(':', '-')}-${key}`, projectId, repositoryId, name, command, port, clientId: 'studio', clientName: 'Workstation', branch: main ? 'master' : 'feat/agent-console/shell', state,
    ...(state === PreviewState.RUNNING ? { startedAt: timestampFromMs(now - 42 * 60_000) } : {}),
  });
  return [one('web', 'Website', 'pnpm dev', 5173, PreviewState.RUNNING), one('phone', 'Phone app', 'npx expo start --web', 8081, PreviewState.STOPPED)];
}

/**
 * The page a sample preview shows. It is a small page of its own, with no code from any product.
 * It reads nothing and stores nothing, so it works inside a frame that has no origin.
 */
export const PLACEHOLDER_PRODUCT = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'">
<title>Sample product</title>
<style>
  :root { color-scheme: light dark; --bg: #f7f7f5; --fg: #1c1b1a; --muted: #6b6965; --line: #dddad4; --card: #fff; --accent: #1f6f5c; }
  @media (prefers-color-scheme: dark) { :root { --bg: #1b181c; --fg: #ece8ea; --muted: #a39ca3; --line: #3a343b; --card: #241f25; --accent: #7fd1bb; } }
  * { box-sizing: border-box; } body { margin: 0; font: 15px/1.5 system-ui, sans-serif; background: var(--bg); color: var(--fg); }
  header { display: flex; align-items: center; gap: 12px; padding: 12px 16px; border-bottom: 1px solid var(--line); }
  header b { font-size: 16px; } header span { margin-left: auto; color: var(--muted); font-size: 13px; font-variant-numeric: tabular-nums; }
  main { padding: 16px; display: grid; gap: 12px; max-width: 720px; margin: 0 auto; }
  ul { list-style: none; margin: 0; padding: 0; border: 1px solid var(--line); border-radius: 8px; background: var(--card); }
  li { display: flex; gap: 12px; padding: 10px 12px; } li + li { border-top: 1px solid var(--line); } li span { margin-left: auto; color: var(--muted); }
  button { font: inherit; padding: 8px 14px; border-radius: 6px; border: 1px solid var(--accent); background: transparent; color: var(--accent); justify-self: start; cursor: pointer; }
  p { margin: 0; color: var(--muted); }
</style></head>
<body>
<header><b>Requests</b><span id="size"></span></header>
<main>
  <ul><li>Where should an agent's thread open?<span>18 min</span></li><li>Run the web tests<span>6 min</span></li><li>Read the review trace<span>2 min</span></li></ul>
  <button id="more" type="button">Answered 0</button>
  <p>This is a sample page. A real preview shows the product that the command serves.</p>
</main>
<script>
  var n = 0, size = document.getElementById('size'), more = document.getElementById('more');
  function show() { size.textContent = innerWidth + ' × ' + innerHeight; }
  addEventListener('resize', show); show();
  more.addEventListener('click', function () { n += 1; more.textContent = 'Answered ' + n; });
</script>
</body></html>`;
