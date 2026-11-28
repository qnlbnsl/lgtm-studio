// Keeps the store in step with the server.
//
// A collection that has moved to the contract is read here and nowhere else. The server sends
// it once and then sends each change. Compatibility workspaces also hold sample collections.
import { Code, ConnectError } from '@connectrpc/connect';
import { timestampMs } from '@bufbuild/protobuf/wkt';
import { ClientState, type Client, type WatchClientsResponse } from '@lgtm/api/clients';
import { MapState } from '@lgtm/api/map';
import { PreviewState } from '@lgtm/api/preview';
import { ProjectService } from '@lgtm/api/projects';
import { SignInResult } from '@lgtm/api/session';
import { FRAMED, persist, store, update } from '../lib';
import type { JoiningClient, Machine, Repo, State } from '../types';
import { BadAddress, SAMPLE, addressLabel, fixedAddress, rememberAddress, rememberedAddress, serverUrl } from './address';
import { WrongServer, api, onServer, openServer, selectServer, type Api, type ServerCandidate } from './api';
import { applyRepositoryChange } from './repositories';

const toMachine = (c: Client, repositoryMode: boolean): Machine => ({
  id: c.id, name: c.name, role: c.hostsServer ? 'master' : 'worker',
  status: c.state === ClientState.ONLINE ? 'online' : c.state === ClientState.OFFLINE ? 'offline' : 'pairing',
  eventHistoryNeedsRecovery: c.eventHistoryNeedsRecovery,
  here: c.hostsServer, slots: { used: 0, limit: c.agentLimit },
  lastSeen: c.lastSeen ? timestampMs(c.lastSeen) : repositoryMode ? Number.NaN : 0,
  address: c.address, workspace: c.workspace, os: c.os, connectedFor: c.connectedAt ? Date.now() - timestampMs(c.connectedAt) : 0,
  cpu: repositoryMode ? c.cpuPercent : c.cpuPercent ?? 0, mem: repositoryMode ? c.memoryPercent : c.memoryPercent ?? 0,
  cores: c.cores, ram: c.memoryGigabytes, version: c.version,
  harnesses: c.harnessesScanned ? Object.fromEntries(c.harnesses.map((h) => [h.harness, h.version ?? null])) : null,
});
const toJoining = (c: Client): JoiningClient => ({
  id: c.id, host: c.name, address: c.address, os: c.os, cores: c.cores, ram: c.memoryGigabytes, tokenId: c.tokenId,
  eventHistoryNeedsRecovery: c.eventHistoryNeedsRecovery,
});
const waits = (c: Client) => c.state === ClientState.WAITING_FOR_APPROVAL;

/** What a failed call says, as a sentence for the page. */
export function reason(e: unknown): string {
  if (e instanceof BadAddress || e instanceof WrongServer) return e.message;
  if (e instanceof ConnectError) return e.rawMessage || 'The server refused the request.';
  return 'The server did not answer.';
}

let selectWorkspace: ((repositoryMode: boolean) => void) | null = null;
let generation = 0;
let selected: ServerCandidate | null = null;
let authIntent = 0;
const credentialTimeout = 6000;
const cookieWriters = new Map<string, Promise<void>>();
const cookieHost = (url: string) => url === SAMPLE ? SAMPLE : new URL(url).hostname;

function writeCookie<T>(url: string, write: () => Promise<T>): Promise<T> {
  const host = cookieHost(url);
  const reply = (cookieWriters.get(host) ?? Promise.resolve()).then(write);
  const tail = reply.then(() => {}, () => {});
  cookieWriters.set(host, tail);
  void tail.then(() => { if (cookieWriters.get(host) === tail) cookieWriters.delete(host); });
  return reply;
}

export interface SessionScope {
  clients: Api;
  url: string;
  signal: AbortSignal;
  isCurrent(): boolean;
}

let session: SessionScope | null = null;
let controller: AbortController | null = null;

interface WatchAttempt { openedAt: number; initialized: boolean }
interface WatchFeed { attempt?: WatchAttempt; controller: AbortController }
let machineFeed: WatchFeed | null = null;
let repositoryFeed: WatchFeed | null = null;

/** Captures the clients and lifetime that own a protected operation. */
export function captureSession(): SessionScope | null { return session; }

/** Returns the repository list once the current attempt has received its initial snapshot. */
export function currentRepositorySnapshot(): { openedAt: number; repos: readonly Repo[] } | undefined {
  const attempt = repositoryFeed?.attempt;
  if (attempt?.initialized) return { openedAt: attempt.openedAt, repos: store.state.repos };
}

/** Opens a fresh repository watch without restarting the machine watch. */
export function refreshRepositories(): void {
  const scope = captureSession();
  if (!scope || !repositoryFeed) return;
  repositoryFeed.controller.abort();
  startRepositoryWatch(scope);
  update(updateWatchConnection);
}

function stopWatching(): void {
  session = null;
  machineFeed = null;
  repositoryFeed = null;
  controller?.abort();
  controller = null;
}

function startSession(server: ServerCandidate): void {
  const signal = (controller = new AbortController()).signal;
  const scope: SessionScope = { clients: server.clients, url: server.url, signal, isCurrent: () => session === scope };
  session = scope;
}

function clearMachines(s: State): void {
  s.server.synced = false;
  s.machines = [];
  s.joining = [];
}

function clearRepositorySession(s: State): void {
  clearMachines(s);
  s.repos = [];
  Object.assign(s, {
    expanded: {}, inspecting: {}, ui: {}, dialog: null, pending: {}, toasts: [], drafts: Object.create(null),
    phoneOffer: null, machineDetails: null, subscriptions: [], projects: [], teams: [], agents: [],
    epics: [], sprints: [], tasks: [], requests: [], threads: {}, channels: [], posts: [], notifications: [],
    define: {}, worktrees: [], gitUi: {}, mocks: {}, design: {}, workflow: {}, boardRead: {}, boardUnread: {},
    boardRevision: 0, boardDetails: {}, taskDetails: {}, setupData: null, boardAgents: {}, projectLeadSettings: {},
    usage: { rows: [], limits: {}, review: { ranAt: 0, findings: [] } },
  });
  s.terminal.byProject = {};
  s.conn = { status: 'reconnecting', lastHeard: 0 };
}

/** Ends the current session when a protected request reports that it has expired. */
export function expireSession(scope: SessionScope, error: unknown): boolean {
  if (!scope.isCurrent() || !(error instanceof ConnectError) || error.code !== Code.Unauthenticated) return false;
  authIntent++;
  stopWatching();
  update((s) => {
    s.session.status = s.server.synced ? 'expired' : 'out';
    if (s.server.repositoryMode) clearRepositorySession(s);
    else s.server.synced = false;
  });
  return true;
}

function waitForRetry(signal: AbortSignal, delay: number): Promise<void> {
  if (signal.aborted) return Promise.resolve();
  return new Promise((done) => {
    const timer = setTimeout(finish, delay);
    signal.addEventListener('abort', finish, { once: true });
    function finish() {
      clearTimeout(timer);
      signal.removeEventListener('abort', finish);
      done();
    }
  });
}

function applyClientChange(s: State, { change }: WatchClientsResponse): void {
  if (change.case === 'all') {
    s.machines = change.value.clients.filter((c) => !waits(c)).map((c) => toMachine(c, s.server.repositoryMode));
    s.joining = change.value.clients.filter(waits).map(toJoining);
  } else if (change.case === 'saved') {
    const c = change.value;
    s.machines = s.machines.filter((m) => m.id !== c.id);
    s.joining = s.joining.filter((j) => j.id !== c.id);
    if (waits(c)) s.joining.push(toJoining(c));
    else s.machines.push(toMachine(c, s.server.repositoryMode));
  } else if (change.case === 'removedId') {
    s.machines = s.machines.filter((m) => m.id !== change.value);
    s.joining = s.joining.filter((j) => j.id !== change.value);
  }
}

function updateWatchConnection(s: State): void {
  const live = machineFeed?.attempt?.initialized && repositoryFeed?.attempt?.initialized;
  if (live) s.server.synced = true;
  s.conn.status = live ? 'live' : 'reconnecting';
}

async function followWatch<T extends { change: { case: string | undefined } }>(
  scope: SessionScope,
  feed: WatchFeed,
  open: (signal: AbortSignal) => AsyncIterable<T>,
  apply: (s: State, frame: T) => void,
): Promise<void> {
  const signal = AbortSignal.any([scope.signal, feed.controller.signal]);
  const repositoryMode = store.state.server.repositoryMode;
  for (let wait = 500; !signal.aborted; wait = Math.min(wait * 2, 8000)) {
    const attempt = { openedAt: performance.now(), initialized: false };
    feed.attempt = attempt;
    const current = () => scope.isCurrent() && !signal.aborted && feed.attempt === attempt;
    try {
      for await (const frame of open(signal)) {
        if (!current()) return;
        const kind = frame.change.case;
        if (kind === 'all') attempt.initialized = true;
        if (repositoryMode && (!attempt.initialized || kind === undefined)) continue;
        wait = 500;
        update((s) => {
          apply(s, frame);
          if (repositoryMode) updateWatchConnection(s);
          else if (kind === 'all') {
            s.server.synced = true;
            s.conn.status = 'live';
          }
          s.conn.lastHeard = Date.now();
        });
      }
    } catch (e) {
      if (!current() || expireSession(scope, e)) return;
      if (!repositoryMode) update((s) => { s.conn.status = 'reconnecting'; });
    }
    if (!current()) return;
    feed.attempt = undefined;
    if (repositoryMode) update(updateWatchConnection);
    await waitForRetry(signal, wait);
  }
}

function startRepositoryWatch(scope: SessionScope): void {
  const feed = { controller: new AbortController() };
  repositoryFeed = feed;
  void followWatch(scope, feed, (signal) => scope.clients.projects.watchRepositories({}, { signal }), applyRepositoryChange);
}

// Reads the collections the server owns and follows their changes until the session ends.
function watch(scope: SessionScope): void {
  const feed = { controller: new AbortController() };
  machineFeed = feed;
  void followWatch(scope, feed, (signal) => scope.clients.clients.watchClients({}, { signal }), applyClientChange);
  if (store.state.server.repositoryMode) startRepositoryWatch(scope);
}

/** Starts reading the server's records, when nothing reads them yet. */
export function ensureReading(): void {
  const scope = captureSession();
  if (scope && !machineFeed && store.state.session.status === 'in') watch(scope);
}

function forgetSelection(): number {
  generation++;
  stopWatching();
  selected = null;
  selectServer(null);
  return generation;
}

/** Connects to a server and, when the browser is signed in there, starts reading it. */
export async function connect(text: string, onReady?: () => void): Promise<void> {
  const select = selectWorkspace;
  if (!select) return;
  const mine = forgetSelection();
  if (store.state.server.repositoryMode) update(clearRepositorySession);
  let url: string;
  try { url = serverUrl(text); } catch (e) { update((s) => { s.server = { ...s.server, status: 'ask', error: reason(e) }; }); return; }
  update((s) => { s.server = { status: 'connecting', address: text, label: addressLabel(url), sample: url === SAMPLE, repositoryMode: s.server.repositoryMode, synced: false, fixed: fixedAddress() !== null }; });
  try {
    // Discovery observes the cookie left by the writers queued before it.
    await cookieWriters.get(cookieHost(url));
    if (generation !== mine) return;
    // The sample inside the page has no cookie, so it remembers the session the way the mock always did.
    const candidate = await openServer(url, { signedIn: persist('session', 'in') === 'in' });
    if (generation !== mine) return;
    const { server } = candidate;
    const sample = url === SAMPLE || server.sample;
    const repositoryMode = !sample && server.services.includes(ProjectService.typeName);
    if (!fixedAddress()) rememberAddress(text);
    // A framed copy of the website, in the Design view, may ask for a signed-out screen.
    const forced = FRAMED ? new URLSearchParams(location.hash.split('?')[1] || '').get('session') : null;
    select(repositoryMode);
    selected = candidate;
    selectServer(candidate);
    startSession(candidate);
    update((s) => {
      s.server.status = 'ready';
      s.server.sample = sample;
      s.server.repositoryMode = repositoryMode;
      s.session.status = forced || (server.signedIn ? 'in' : 'out');
    });
    ensureReading();
    onReady?.();
  } catch (e) {
    if (generation !== mine) return;
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
  forgetSelection();
}

/** Forgets the server, and asks for an address again. */
export function changeServer(): void {
  forgetSelection();
  rememberAddress(null);
  update((s) => {
    if (s.server.repositoryMode) clearRepositorySession(s);
    else clearMachines(s);
    s.server = { ...s.server, status: 'ask', error: undefined };
  });
}

type SignInStatus = 'in' | 'rejected' | 'unavailable';

/** Signs in with the selected server and applies the result while its intent is current. */
export async function signIn(password: string, apply: (status: SignInStatus) => void): Promise<SignInStatus | 'obsolete'> {
  const server = selected;
  if (!server) return 'obsolete';
  const mine = ++authIntent;
  let status: SignInStatus;
  try {
    const { result } = await writeCookie(server.url, () => server.clients.session.signIn({ password }, { timeoutMs: credentialTimeout }));
    status = result === SignInResult.SIGNED_IN ? 'in' : 'rejected';
  } catch { status = 'unavailable'; }
  if (selected !== server || authIntent !== mine) return 'obsolete';
  if (status === 'in') {
    stopWatching();
    if (store.state.server.repositoryMode) update(clearRepositorySession);
    startSession(server);
  }
  apply(status);
  if (selected === server && authIntent === mine) ensureReading();
  return status;
}

export async function signOut(apply: () => void): Promise<void> {
  const server = selected;
  if (!server) return;
  const mine = ++authIntent;
  stopWatching();
  if (store.state.server.repositoryMode) update((s) => {
    clearRepositorySession(s);
    s.session.status = 'out';
  });
  try {
    await writeCookie(server.url, () => server.clients.session.signOut({}, { timeoutMs: credentialTimeout }));
  } catch { /* The page forgets the session either way. */ }
  if (selected !== server || authIntent !== mine) return;
  update(clearMachines);
  apply();
}

// ---- the clients of the server, which the website calls machines ---------------------
async function machineCall<T>(scope: SessionScope, call: (clients: Api['clients']) => Promise<T>): Promise<T> {
  scope.signal.throwIfAborted();
  try { return await call(scope.clients.clients); }
  catch (e) {
    if (store.state.server.repositoryMode) expireSession(scope, e);
    throw e;
  }
}

// `address` is the address inside the token, which the machine dials. A sample server leaves it empty.
export interface ClientToken { id: string; secret: string; expiresAt: number; address: string }
export async function createClientToken(scope: SessionScope): Promise<ClientToken> {
  const t = await machineCall(scope, (clients) => clients.createClientToken({}, { signal: scope.signal }));
  return { id: t.tokenId, secret: t.token, expiresAt: t.expiresAt ? timestampMs(t.expiresAt) : Date.now(), address: t.address };
}
export const revokeClientToken = (scope: SessionScope, tokenId: string) =>
  machineCall(scope, (clients) => clients.revokeClientToken({ tokenId }, { signal: scope.signal })).then(() => {}, () => {});
export const approveClient = (scope: SessionScope, id: string, name: string) =>
  machineCall(scope, (clients) => clients.approveClient({ id, name }, { signal: scope.signal })).then(() => {});
export const renameClient = (scope: SessionScope, id: string, name: string) =>
  machineCall(scope, (clients) => clients.renameClient({ id, name }, { signal: scope.signal })).then(() => {});
export const scanClientHarnesses = (scope: SessionScope, id: string) =>
  machineCall(scope, (clients) => clients.scanClientHarnesses({ id }, { signal: scope.signal })).then(() => {});
export const reconnectClient = (scope: SessionScope, id: string) =>
  machineCall(scope, (clients) => clients.reconnectClient({ id }, { signal: scope.signal })).then(() => {});
export const removeClient = (scope: SessionScope, id: string) =>
  machineCall(scope, (clients) => clients.removeClient({ id }, { signal: scope.signal })).then(() => {});

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
  const scope = captureSession();
  if (!scope) return () => {};
  const stop = new AbortController();
  const signal = AbortSignal.any([scope.signal, stop.signal]);
  void (async () => {
    for (let wait = 500; !signal.aborted; wait = Math.min(wait * 2, 8000)) {
      try {
        for await (const { map } of scope.clients.maps.watchMap(place(projectId), { signal })) {
          if (signal.aborted) return;
          if (!map) continue;
          wait = 500;
          const f = map.facts;
          onChange({
            state: map.state === MapState.READY ? 'ready' : 'none', building: map.building, sizeBytes: Number(map.sizeBytes), modifiedAt: map.modifiedAt ? timestampMs(map.modifiedAt) : 0,
            facts: f ? { generatedAt: f.generatedAt ? timestampMs(f.generatedAt) : 0, files: f.files, lines: f.lines, parts: f.parts, links: f.links } : null,
          });
        }
      } catch (e) {
        if (signal.aborted) return;
        onError(reason(e));
      }
      await waitForRetry(signal, wait);
    }
  })();
  return () => stop.abort();
}
export const buildMap = (projectId: string) => api().maps.buildMap(place(projectId)).then(() => {});
export async function openMap(projectId: string): Promise<MapFrame> {
  const scope = captureSession();
  if (!scope) throw new Error('The website has no server yet.');
  const { frame } = await scope.clients.maps.openMap(place(projectId), { signal: scope.signal });
  scope.signal.throwIfAborted();
  if (!frame) throw new Error('The server gave no address.');
  return { id: frame.id, url: onServer(frame.url, scope.url), expiresAt: frame.expiresAt ? timestampMs(frame.expiresAt) : 0 };
}
export async function releaseMap(id: string): Promise<void> {
  try { await captureSession()?.clients.maps.releaseMap({ id }); } catch { /* The frame can expire on the server. */ }
}

// ---- the running product of a project -------------------------------------------------
export interface PreviewView {
  id: string; name: string; command: string; port: number; machine: string; machineId: string; branch: string;
  state: 'stopped' | 'starting' | 'running' | 'failed'; error: string; startedAt: number;
}
export interface PreviewFrame { id: string; url: string; expiresAt: number }
const PREVIEW_STATES: Record<number, PreviewView['state']> = { [PreviewState.STARTING]: 'starting', [PreviewState.RUNNING]: 'running', [PreviewState.FAILED]: 'failed' };
/** Follows one project's previews until the returned function is called. */
export function watchPreviews(projectId: string, onChange: (list: PreviewView[]) => void, onError: (text: string) => void): () => void {
  const scope = captureSession();
  if (!scope) return () => {};
  const stop = new AbortController();
  const signal = AbortSignal.any([scope.signal, stop.signal]);
  void (async () => {
    while (!signal.aborted) {
      try {
        for await (const { previews } of scope.clients.previews.watchPreviews(place(projectId), { signal })) {
          if (signal.aborted) return;
          onChange(previews.map((p) => ({
            id: p.id, name: p.name, command: p.command, port: p.port, machine: p.clientName, machineId: p.clientId, branch: p.branch,
            state: PREVIEW_STATES[p.state] ?? 'stopped', error: p.error, startedAt: p.startedAt ? timestampMs(p.startedAt) : 0,
          })));
        }
      } catch (e) {
        if (signal.aborted) return;
        onError(reason(e));
      }
      await waitForRetry(signal, 2000);
    }
  })();
  return () => stop.abort();
}
export const startPreview = (id: string) => api().previews.startPreview({ id }).then(() => {});
export const stopPreview = (id: string) => api().previews.stopPreview({ id }).then(() => {});
export async function openPreview(id: string): Promise<PreviewFrame> {
  const scope = captureSession();
  if (!scope) throw new Error('The website has no server yet.');
  const { frame } = await scope.clients.previews.openPreview({ id }, { signal: scope.signal });
  scope.signal.throwIfAborted();
  if (!frame) throw new Error('The server gave no address for the preview.');
  return { id: frame.id, url: onServer(frame.url, scope.url), expiresAt: frame.expiresAt ? timestampMs(frame.expiresAt) : 0 };
}
export async function releasePreview(id: string): Promise<void> {
  try { await captureSession()?.clients.previews.releasePreview({ id }); } catch { /* The frame can expire on the server. */ }
}
/** Adds a preview when `id` is empty, and changes one otherwise. */
export const savePreview = (p: { id?: string; projectId: string; name: string; command: string; port: number; machineId?: string }) =>
  api().previews.savePreview({ id: p.id ?? '', ...place(p.projectId), name: p.name, command: p.command, port: p.port, clientId: p.machineId ?? '' }).then(() => {});
export const removePreview = (id: string) => api().previews.removePreview({ id }).then(() => {});
