// Keeps the store in step with the server.
//
// A collection that has moved to the contract is read here and nowhere else. The server sends
// it once and then sends each change. Compatibility workspaces also hold sample collections.
import { Code, ConnectError } from '@connectrpc/connect';
import { timestampMs } from '@bufbuild/protobuf/wkt';
import { ClientState, type Client } from '@lgtm/api/clients';
import { MapState } from '@lgtm/api/map';
import { PreviewState } from '@lgtm/api/preview';
import { ProjectService } from '@lgtm/api/projects';
import { SignInResult } from '@lgtm/api/session';
import { FRAMED, persist, store, update } from '../lib';
import type { JoiningClient, Machine } from '../types';
import { BadAddress, SAMPLE, addressLabel, fixedAddress, rememberAddress, rememberedAddress, serverUrl } from './address';
import { WrongServer, api, onServer, openServer } from './api';

const toMachine = (c: Client): Machine => ({
  id: c.id, name: c.name, role: c.hostsServer ? 'master' : 'worker',
  status: c.state === ClientState.ONLINE ? 'online' : c.state === ClientState.OFFLINE ? 'offline' : 'pairing',
  here: c.hostsServer, slots: { used: 0, limit: c.agentLimit }, lastSeen: c.lastSeen ? timestampMs(c.lastSeen) : 0,
  address: c.address, workspace: c.workspace, os: c.os, connectedFor: c.connectedAt ? Date.now() - timestampMs(c.connectedAt) : 0,
  cpu: c.cpuPercent ?? 0, mem: c.memoryPercent ?? 0, cores: c.cores, ram: c.memoryGigabytes, version: c.version,
  harnesses: c.harnessesScanned ? Object.fromEntries(c.harnesses.map((h) => [h.harness, h.version ?? null])) : null,
});
const toJoining = (c: Client): JoiningClient => ({ id: c.id, host: c.name, address: c.address, os: c.os, cores: c.cores, ram: c.memoryGigabytes, tokenId: c.tokenId });
const waits = (c: Client) => c.state === ClientState.WAITING_FOR_APPROVAL;

/** What a failed call says, as a sentence for the page. */
export function reason(e: unknown): string {
  if (e instanceof BadAddress || e instanceof WrongServer) return e.message;
  if (e instanceof ConnectError) return e.rawMessage || 'The server refused the request.';
  return 'The server did not answer.';
}

let watching: AbortController | null = null;
function stopWatching(): void { watching?.abort(); watching = null; }

// Reads the collections the server owns, and follows their changes until the session ends.
function watch(): void {
  stopWatching();
  const mine = watching = new AbortController();
  void (async () => {
    for (let wait = 500; !mine.signal.aborted; wait = Math.min(wait * 2, 8000)) {
      try {
        for await (const { change } of api().clients.watchClients({}, { signal: mine.signal })) {
          if (mine.signal.aborted) return;
          wait = 500;
          update((s) => {
            if (change.case === 'all') { s.machines = change.value.clients.filter((c) => !waits(c)).map(toMachine); s.joining = change.value.clients.filter(waits).map(toJoining); s.server.synced = true; s.conn.status = 'live'; }
            else if (change.case === 'saved') {
              const c = change.value; s.machines = s.machines.filter((m) => m.id !== c.id); s.joining = s.joining.filter((j) => j.id !== c.id);
              if (waits(c)) s.joining.push(toJoining(c)); else s.machines.push(toMachine(c));
            } else if (change.case === 'removedId') { s.machines = s.machines.filter((m) => m.id !== change.value); s.joining = s.joining.filter((j) => j.id !== change.value); }
            s.conn.lastHeard = Date.now();
          });
        }
      } catch (e) {
        if (mine.signal.aborted) return;
        if (e instanceof ConnectError && e.code === Code.Unauthenticated) { update((s) => { s.session.status = s.server.synced ? 'expired' : 'out'; s.server.synced = false; }); return; }
        update((s) => { s.conn.status = 'reconnecting'; });
      }
      // The stream ended or failed. Wait a little longer each time, and read everything again.
      await new Promise((done) => setTimeout(done, wait));
    }
  })();
}

/** Starts reading the server's records, when nothing reads them yet. */
export function ensureReading(): void { if (!watching) watch(); }

let selectWorkspace: ((repositoryMode: boolean) => void) | null = null;

/** Connects to a server and, when the browser is signed in there, starts reading it. */
export async function connect(text: string): Promise<void> {
  const select = selectWorkspace;
  if (!select) return;
  let url: string;
  try { url = serverUrl(text); } catch (e) { update((s) => { s.server = { ...s.server, status: 'ask', error: reason(e) }; }); return; }
  stopWatching();
  update((s) => { s.server = { status: 'connecting', address: text, label: addressLabel(url), sample: url === SAMPLE, repositoryMode: s.server.repositoryMode, synced: false, fixed: fixedAddress() !== null }; });
  try {
    // The sample inside the page has no cookie, so it remembers the session the way the mock always did.
    const server = await openServer(url, { signedIn: persist('session', 'in') === 'in' });
    if (selectWorkspace !== select) return;
    const sample = url === SAMPLE || server.sample;
    const repositoryMode = !sample && server.services.includes(ProjectService.typeName);
    if (!fixedAddress()) rememberAddress(text);
    // A framed copy of the website, in the Design view, may ask for a signed-out screen.
    const forced = FRAMED ? new URLSearchParams(location.hash.split('?')[1] || '').get('session') : null;
    select(repositoryMode);
    update((s) => {
      s.server.status = 'ready';
      s.server.sample = sample;
      s.server.repositoryMode = repositoryMode;
      s.session.status = forced || (server.signedIn ? 'in' : 'out');
    });
    if (store.state.session.status === 'in') watch();
  } catch (e) {
    if (selectWorkspace !== select) return;
    update((s) => { s.server = { ...s.server, status: 'unreachable', error: e instanceof WrongServer ? e.message : `No LGTM Studio server answers at ${addressLabel(url)}.` }; });
  }
}

/** Starts the website: connects to the fixed or the remembered address, or asks for one. */
export function start(select: (repositoryMode: boolean) => void): void {
  selectWorkspace = select;
  const address = fixedAddress() ?? rememberedAddress();
  if (address) void connect(address);
  else update((s) => { s.server.status = 'ask'; });
}

/** Ends standalone discovery and reading when the website leaves its element. */
export function stop(): void {
  selectWorkspace = null;
  stopWatching();
}

/** Forgets the server, and asks for an address again. */
export function changeServer(): void {
  stopWatching(); rememberAddress(null);
  update((s) => { s.server = { ...s.server, status: 'ask', synced: false, error: undefined }; s.machines = []; s.joining = []; });
}

/** Signs in with the server's password. Resolves to the session's new status. */
export async function signIn(password: string): Promise<'in' | 'rejected' | 'unavailable'> {
  try {
    const { result } = await api().session.signIn({ password });
    if (result !== SignInResult.SIGNED_IN) return 'rejected';
    watch();
    return 'in';
  } catch { return 'unavailable'; }
}
export async function signOut(): Promise<void> {
  stopWatching();
  try { await api().session.signOut({}); } catch { /* The page forgets the session either way. */ }
  update((s) => { s.server.synced = false; s.machines = []; s.joining = []; });
}

// ---- the clients of the server, which the website calls machines ---------------------
// `address` is the address inside the token, which the machine dials. A sample server leaves it empty.
export interface ClientToken { id: string; secret: string; expiresAt: number; address: string }
export async function createClientToken(): Promise<ClientToken> {
  const t = await api().clients.createClientToken({});
  return { id: t.tokenId, secret: t.token, expiresAt: t.expiresAt ? timestampMs(t.expiresAt) : Date.now(), address: t.address };
}
export const revokeClientToken = (tokenId: string) => api().clients.revokeClientToken({ tokenId }).then(() => {}, () => {});
export const approveClient = (id: string, name: string) => api().clients.approveClient({ id, name }).then(() => {});
export const renameClient = (id: string, name: string) => api().clients.renameClient({ id, name }).then(() => {});
export const scanClientHarnesses = (id: string) => api().clients.scanClientHarnesses({ id }).then(() => {});
export const reconnectClient = (id: string) => api().clients.reconnectClient({ id }).then(() => {});
export const removeClient = (id: string) => api().clients.removeClient({ id }).then(() => {});

// A map or a preview belongs to a project, or to the main checkout of a repository. The screens
// name the second as "repository:" and the repository's id.
const place = (id: string) => (id.startsWith('repository:') ? { projectId: '', repositoryId: id.slice(11) } : { projectId: id, repositoryId: '' });

// ---- the codebase map of a project ---------------------------------------------------
export interface MapView {
  state: 'none' | 'ready'; building: boolean; sizeBytes: number; modifiedAt: number;
  facts: { generatedAt: number; files?: number | undefined; lines?: number | undefined; parts?: number | undefined; links?: number | undefined } | null;
}
export interface MapFrame { id: string; url: string; expiresAt: number }
/** Follows one project's map until the returned function is called. */
export function watchMap(projectId: string, onChange: (map: MapView) => void, onError: (text: string) => void): () => void {
  const stop = new AbortController();
  void (async () => {
    for (let wait = 500; !stop.signal.aborted; wait = Math.min(wait * 2, 8000)) {
      try {
        for await (const { map } of api().maps.watchMap(place(projectId), { signal: stop.signal })) {
          if (!map) continue;
          wait = 500;
          const f = map.facts;
          onChange({
            state: map.state === MapState.READY ? 'ready' : 'none', building: map.building, sizeBytes: Number(map.sizeBytes), modifiedAt: map.modifiedAt ? timestampMs(map.modifiedAt) : 0,
            facts: f ? { generatedAt: f.generatedAt ? timestampMs(f.generatedAt) : 0, files: f.files, lines: f.lines, parts: f.parts, links: f.links } : null,
          });
        }
      } catch (e) {
        if (stop.signal.aborted) return;
        onError(reason(e));
      }
      await new Promise((done) => setTimeout(done, wait));
    }
  })();
  return () => stop.abort();
}
export const buildMap = (projectId: string) => api().maps.buildMap(place(projectId)).then(() => {});
export async function openMap(projectId: string): Promise<MapFrame> {
  const { frame } = await api().maps.openMap(place(projectId));
  if (!frame) throw new Error('The server gave no address.');
  return { id: frame.id, url: onServer(frame.url), expiresAt: frame.expiresAt ? timestampMs(frame.expiresAt) : 0 };
}
export const releaseMap = (id: string) => api().maps.releaseMap({ id }).then(() => {}, () => {});

// ---- the running product of a project -------------------------------------------------
export interface PreviewView {
  id: string; name: string; command: string; port: number; machine: string; machineId: string; branch: string;
  state: 'stopped' | 'starting' | 'running' | 'failed'; error: string; startedAt: number;
}
export interface PreviewFrame { id: string; url: string; expiresAt: number }
const PREVIEW_STATES: Record<number, PreviewView['state']> = { [PreviewState.STARTING]: 'starting', [PreviewState.RUNNING]: 'running', [PreviewState.FAILED]: 'failed' };
/** Follows one project's previews until the returned function is called. */
export function watchPreviews(projectId: string, onChange: (list: PreviewView[]) => void, onError: (text: string) => void): () => void {
  const stop = new AbortController();
  void (async () => {
    while (!stop.signal.aborted) {
      try {
        for await (const { previews } of api().previews.watchPreviews(place(projectId), { signal: stop.signal })) {
          onChange(previews.map((p) => ({
            id: p.id, name: p.name, command: p.command, port: p.port, machine: p.clientName, machineId: p.clientId, branch: p.branch,
            state: PREVIEW_STATES[p.state] ?? 'stopped', error: p.error, startedAt: p.startedAt ? timestampMs(p.startedAt) : 0,
          })));
        }
      } catch (e) {
        if (stop.signal.aborted) return;
        onError(reason(e));
      }
      await new Promise((r) => setTimeout(r, 2000));
    }
  })();
  return () => stop.abort();
}
export const startPreview = (id: string) => api().previews.startPreview({ id }).then(() => {});
export const stopPreview = (id: string) => api().previews.stopPreview({ id }).then(() => {});
export async function openPreview(id: string): Promise<PreviewFrame> {
  const { frame } = await api().previews.openPreview({ id });
  if (!frame) throw new Error('The server gave no address for the preview.');
  return { id: frame.id, url: onServer(frame.url), expiresAt: frame.expiresAt ? timestampMs(frame.expiresAt) : 0 };
}
export const releasePreview = (id: string) => api().previews.releasePreview({ id }).then(() => {}, () => {});
/** Adds a preview when `id` is empty, and changes one otherwise. */
export const savePreview = (p: { id?: string; projectId: string; name: string; command: string; port: number; machineId?: string }) =>
  api().previews.savePreview({ id: p.id ?? '', ...place(p.projectId), name: p.name, command: p.command, port: p.port, clientId: p.machineId ?? '' }).then(() => {});
export const removePreview = (id: string) => api().previews.removePreview({ id }).then(() => {});
