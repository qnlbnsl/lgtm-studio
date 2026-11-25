// The mock server's engine: it holds sample data in memory and answers the contract in src/contracts.
//
// It has no file, no network and no Node API in it, so the same engine runs in two places.
// serve.ts puts it behind a network port. The website can also run it inside the page, for
// design work with no server at all.
//
// The real server is a separate program. It answers the same contract, and nothing here is copied into it.
import { clone } from '@bufbuild/protobuf';
import { timestampFromMs } from '@bufbuild/protobuf/wkt';
import { Code, ConnectError, type ConnectRouter, type HandlerContext } from '@connectrpc/connect';
import { ClientSchema, ClientService, ClientState, type Client } from '@lgtm/api/clients';
import { MapService, MapState, type CodebaseMap } from '@lgtm/api/map';
import { PreviewService, PreviewState, type Preview } from '@lgtm/api/preview';
import { SessionService, SignInResult } from '@lgtm/api/session';
import { PLACEHOLDER_FACTS, PLACEHOLDER_MAP, PLACEHOLDER_PRODUCT, sampleClients, sampleJoiner, samplePreviews } from './sample.ts';

export interface EngineOptions {
  /** The password SignIn accepts. With none, it accepts any password except "wrong". */
  password?: string | undefined;
  /** True when the engine runs inside the website's page. There is one visitor, and no cookie. */
  inPage?: boolean;
  /** Whether that one visitor starts signed in. */
  signedIn?: boolean;
  /** How long a made-up event takes, as a factor. Tests pass 0. */
  pace?: number;
  /** Where the sample map's page comes from. With none, the map is a placeholder page inside a data address. */
  map?: MapSource | undefined;
  /** The projects that have a map from the start. Another project gets one when the owner asks for it. */
  mappedProjects?: string[];
  /** The address a running product's sample page is read from, for one frame's token. With none, it is a data address. */
  previewUrl?: ((token: string) => string) | undefined;
}

/** What the engine needs to know about the sample map's page. The engine itself reads no file. */
export interface MapSource {
  /** The page's size and time, or undefined when there is no page. */
  info(): MapInfo | undefined;
  /** The address the page is read from, for one frame's token. */
  url(token: string): string;
}
export interface MapInfo {
  sizeBytes: number; modifiedAt: number;
  facts?: { generatedAt?: number | undefined; files?: number | undefined; lines?: number | undefined; parts?: number | undefined; links?: number | undefined } | undefined;
}

/** The revision of the contract this engine speaks. */
export const CONTRACT = 1;
const COOKIE = 'lgtm_session';
// The sample has one session, so a restart of the mock server does not sign the owner out.
const SESSION = 'sample-session';
const TOKEN_LIFE = 10 * 60_000;

type ClientChange = { case: 'all'; value: { clients: Client[] } } | { case: 'saved'; value: Client } | { case: 'removedId'; value: string };
interface Token { id: string; secret: string; expiresAt: number; used: boolean }
type MapInit = Omit<CodebaseMap, '$typeName' | '$unknown'>;
const FRAME_LIFE = 10 * 60_000;

export function createEngine(options: EngineOptions = {}) {
  const pace = options.pace ?? 1;
  let pageSignedIn = options.signedIn ?? false;
  const clients = new Map<string, Client>(sampleClients(Date.now()).map((c) => [c.id, c]));
  const tokens = new Map<string, Token>();
  const watchers = new Set<(change: ClientChange) => void>();
  let n = 0;
  const id = (prefix: string) => `${prefix}-${Date.now().toString(36)}-${(n++).toString(36)}`;
  const later = (ms: number, run: () => void) => { if (pace === 0) run(); else setTimeout(run, ms * pace); };

  const signedIn = (ctx: HandlerContext): boolean => options.inPage
    ? pageSignedIn
    : (ctx.requestHeader.get('cookie') || '').split(';').some((part) => part.trim() === `${COOKIE}=${SESSION}`);
  const needSession = (ctx: HandlerContext) => { if (!signedIn(ctx)) throw new ConnectError('Sign in first.', Code.Unauthenticated); };
  const setCookie = (ctx: HandlerContext, value: string, age: number) =>
    ctx.responseHeader.set('Set-Cookie', `${COOKIE}=${value}; Path=/; HttpOnly; SameSite=Strict; Max-Age=${age}`);

  // ---- the codebase map ------------------------------------------------------------
  const started = Date.now();
  const source: MapSource = options.map ?? {
    info: () => ({ sizeBytes: PLACEHOLDER_MAP.length, modifiedAt: started, facts: PLACEHOLDER_FACTS(started) }),
    url: () => 'data:text/html;charset=utf-8,' + encodeURIComponent(PLACEHOLDER_MAP),
  };
  // The sample starts with a map of one project, and of the main checkout of its repository.
  const mapped = new Set(options.mappedProjects ?? ['console', 'repository:lgtm']);
  const building = new Set<string>();
  const builtAt = new Map<string, number>();
  // One frame is one address of a page. A frame of a preview also names its preview.
  const frames = new Map<string, { token: string; expiresAt: number; preview?: string }>();
  const mapWatchers = new Map<string, Set<(map: MapInit) => void>>();
  // A map or a preview belongs to a project, or to the main checkout of a repository. One key names either.
  const placeOf = (x: { projectId: string; repositoryId: string }): string => (x.repositoryId ? 'repository:' + x.repositoryId : x.projectId);
  const idsOf = (place: string) => (place.startsWith('repository:') ? { projectId: '', repositoryId: place.slice(11) } : { projectId: place, repositoryId: '' });
  const mapOf = (projectId: string): MapInit => {
    const info = mapped.has(projectId) ? source.info() : undefined;
    const facts = info?.facts;
    return {
      ...idsOf(projectId), state: info ? MapState.READY : MapState.NONE, building: building.has(projectId),
      sizeBytes: BigInt(info?.sizeBytes ?? 0),
      // A file's time has fractions of a millisecond, and a timestamp has none.
      modifiedAt: info ? timestampFromMs(Math.round(Math.max(info.modifiedAt, builtAt.get(projectId) ?? 0))) : undefined,
      facts: facts ? {
        $typeName: 'lgtm.v1.MapFacts', generatedAt: facts.generatedAt === undefined ? undefined : timestampFromMs(Math.round(Math.max(facts.generatedAt, builtAt.get(projectId) ?? 0))),
        files: facts.files, lines: facts.lines, parts: facts.parts, links: facts.links,
      } : undefined,
    };
  };
  const tellMap = (projectId: string) => { for (const w of mapWatchers.get(projectId) ?? []) w(mapOf(projectId)); };
  /** Which page a frame's token may still read, or undefined when its address has ended. serve.ts asks this for each request. */
  const frameKind = (token: string): 'map' | 'preview' | undefined => {
    const f = [...frames.values()].find((x) => x.token === token && x.expiresAt > Date.now());
    return f && (f.preview ? 'preview' : 'map');
  };

  // ---- the running product ---------------------------------------------------------
  // The sample has two previews for every project: one that runs, and one that is stopped.
  const previews = new Map<string, Preview[]>();
  const previewWatchers = new Map<string, Set<(list: Preview[]) => void>>();
  const previewsOf = (projectId: string): Preview[] => {
    if (!previews.has(projectId)) previews.set(projectId, samplePreviews(projectId, Date.now()));
    return previews.get(projectId)!;
  };
  const tellPreviews = (projectId: string) => { for (const w of previewWatchers.get(projectId) ?? []) w(previewsOf(projectId).map((p) => ({ ...p }))); };
  const needPreview = (previewId: string): Preview => {
    for (const list of previews.values()) { const p = list.find((x) => x.id === previewId); if (p) return p; }
    throw new ConnectError(`No preview has the id "${previewId}".`, Code.NotFound);
  };
  const previewUrl = options.previewUrl ?? (() => 'data:text/html;charset=utf-8,' + encodeURIComponent(PLACEHOLDER_PRODUCT));

  const tell = (change: ClientChange) => { for (const w of watchers) w(change); };
  const save = (c: Client) => { clients.set(c.id, c); tell({ case: 'saved', value: clone(ClientSchema, c) }); };
  const need = (clientId: string): Client => {
    const c = clients.get(clientId);
    if (!c) throw new ConnectError(`No client has the id "${clientId}".`, Code.NotFound);
    return c;
  };

  // The name a client may take: not empty, and not the name of another approved client.
  const uniqueName = (clientId: string, typed: string): string => {
    const name = typed.trim();
    if (!name) throw new ConnectError('A client needs a name.', Code.InvalidArgument);
    const taken = [...clients.values()].some((x) => x.id !== clientId && x.state !== ClientState.WAITING_FOR_APPROVAL && x.name.toLowerCase() === name.toLowerCase());
    if (taken) throw new ConnectError(`A client is already named "${name}".`, Code.AlreadyExists);
    return name;
  };

  const routes = (router: ConnectRouter): void => {
    router.service(SessionService, {
      getServer(_req, ctx) {
        return { product: 'lgtm-studio', version: '0.9.0-sample', contract: CONTRACT, sample: true, signedIn: signedIn(ctx) };
      },
      signIn(req, ctx) {
        const ok = options.password === undefined ? req.password.trim().toLowerCase() !== 'wrong' : req.password === options.password;
        if (!ok) return { result: SignInResult.REJECTED };
        if (options.inPage) pageSignedIn = true; else setCookie(ctx, SESSION, 30 * 24 * 3600);
        return { result: SignInResult.SIGNED_IN };
      },
      signOut(_req, ctx) {
        // The sample has one session, so signing out ends every open address of a map.
        frames.clear();
        if (options.inPage) pageSignedIn = false; else setCookie(ctx, '', 0);
        return {};
      },
    });

    router.service(ClientService, {
      async *watchClients(_req, ctx) {
        needSession(ctx);
        const queue: ClientChange[] = [{ case: 'all', value: { clients: [...clients.values()].map((c) => clone(ClientSchema, c)) } }];
        let wake = () => {};
        const push = (change: ClientChange) => { queue.push(change); wake(); };
        const stop = () => wake();
        watchers.add(push);
        ctx.signal.addEventListener('abort', stop);
        try {
          while (!ctx.signal.aborted) {
            while (queue.length) yield { change: queue.shift()! };
            await new Promise<void>((resolve) => { wake = resolve; if (queue.length || ctx.signal.aborted) resolve(); });
          }
        } finally {
          watchers.delete(push);
          ctx.signal.removeEventListener('abort', stop);
        }
      },
      createClientToken(_req, ctx) {
        needSession(ctx);
        const token: Token = { id: id('token'), secret: secret(), expiresAt: Date.now() + TOKEN_LIFE, used: false };
        tokens.set(token.id, token);
        // The sample has no real client. A made-up one uses the token a moment later.
        later(3200, () => {
          const t = tokens.get(token.id);
          if (!t || t.used || t.expiresAt < Date.now()) return;
          t.used = true;
          save(sampleJoiner(id('client'), t.id, Date.now()));
        });
        return { tokenId: token.id, token: token.secret, expiresAt: timestampFromMs(token.expiresAt) };
      },
      revokeClientToken(req, ctx) {
        needSession(ctx);
        tokens.delete(req.tokenId);
        // A client that already used the token and still waits is refused with it.
        for (const c of clients.values()) if (c.tokenId === req.tokenId && c.state === ClientState.WAITING_FOR_APPROVAL) { clients.delete(c.id); tell({ case: 'removedId', value: c.id }); }
        return {};
      },
      approveClient(req, ctx) {
        needSession(ctx);
        const c = need(req.id);
        if (c.state !== ClientState.WAITING_FOR_APPROVAL) throw new ConnectError(`${c.name} does not wait for approval.`, Code.FailedPrecondition);
        const name = uniqueName(c.id, req.name);
        tokens.delete(c.tokenId);
        const joined = clone(ClientSchema, c);
        Object.assign(joined, { name, state: ClientState.JOINING, tokenId: '' });
        save(joined);
        later(2600, () => {
          const now = clients.get(joined.id);
          if (!now || now.state !== ClientState.JOINING) return;
          const ready = clone(ClientSchema, now);
          Object.assign(ready, { state: ClientState.ONLINE, cpuPercent: 7, memoryPercent: 28, harnessesScanned: true, lastSeen: timestampFromMs(Date.now()) });
          ready.harnesses = [{ $typeName: 'lgtm.v1.InstalledHarness', harness: 'claude', version: '2.4.1' }, { $typeName: 'lgtm.v1.InstalledHarness', harness: 'codex' }];
          save(ready);
        });
        return { client: clone(ClientSchema, joined) };
      },
      renameClient(req, ctx) {
        needSession(ctx);
        const c = need(req.id); const name = uniqueName(c.id, req.name);
        const renamed = clone(ClientSchema, c); renamed.name = name;
        save(renamed);
        return {};
      },
      scanClientHarnesses(req, ctx) {
        needSession(ctx);
        const c = need(req.id);
        if (c.state !== ClientState.ONLINE) throw new ConnectError(`${c.name} is not online.`, Code.FailedPrecondition);
        // The sample's clients find the same harnesses every time.
        save(clone(ClientSchema, c));
        return {};
      },
      reconnectClient(req, ctx) {
        needSession(ctx);
        const c = need(req.id);
        if (c.state !== ClientState.OFFLINE) throw new ConnectError(`${c.name} is not offline.`, Code.FailedPrecondition);
        later(1500, () => {
          const now = clients.get(c.id);
          if (!now || now.state !== ClientState.OFFLINE) return;
          const back = clone(ClientSchema, now);
          Object.assign(back, { state: ClientState.ONLINE, cpuPercent: 9, memoryPercent: 31, lastSeen: timestampFromMs(Date.now()), connectedAt: timestampFromMs(Date.now()) });
          save(back);
        });
        return {};
      },
      removeClient(req, ctx) {
        needSession(ctx);
        const c = need(req.id);
        if (c.hostsServer) throw new ConnectError('The machine that runs the server cannot be removed.', Code.FailedPrecondition);
        clients.delete(c.id);
        tell({ case: 'removedId', value: c.id });
        return {};
      },
    });

    router.service(MapService, {
      async *watchMap(req, ctx) {
        needSession(ctx);
        const queue: MapInit[] = [mapOf(placeOf(req))];
        let wake = () => {};
        const push = (map: MapInit) => { queue.push(map); wake(); };
        const stop = () => wake();
        const set = mapWatchers.get(placeOf(req)) ?? new Set();
        mapWatchers.set(placeOf(req), set); set.add(push);
        ctx.signal.addEventListener('abort', stop);
        try {
          while (!ctx.signal.aborted) {
            while (queue.length) yield { map: { $typeName: 'lgtm.v1.CodebaseMap' as const, ...queue.shift()! } };
            await new Promise<void>((resolve) => { wake = resolve; if (queue.length || ctx.signal.aborted) resolve(); });
          }
        } finally {
          set.delete(push);
          ctx.signal.removeEventListener('abort', stop);
        }
      },
      buildMap(req, ctx) {
        needSession(ctx);
        if (!placeOf(req)) throw new ConnectError('Name the project.', Code.InvalidArgument);
        if (building.has(placeOf(req))) return {};
        building.add(placeOf(req)); tellMap(placeOf(req));
        // The sample has no agent. The map is "built" a moment later, from the same page.
        later(5000, () => { building.delete(placeOf(req)); mapped.add(placeOf(req)); builtAt.set(placeOf(req), Date.now()); tellMap(placeOf(req)); });
        return {};
      },
      openMap(req, ctx) {
        needSession(ctx);
        if (!mapped.has(placeOf(req)) || !source.info()) throw new ConnectError('This project has no map yet.', Code.FailedPrecondition);
        const token = secret().replaceAll('-', '') + secret().replaceAll('-', '');
        const frame = { id: id('frame'), token, expiresAt: Date.now() + FRAME_LIFE };
        frames.set(frame.id, frame);
        return { frame: { id: frame.id, url: source.url(token), expiresAt: timestampFromMs(frame.expiresAt) } };
      },
      releaseMap(req, ctx) {
        needSession(ctx);
        frames.delete(req.id);
        return {};
      },
    });

    router.service(PreviewService, {
      async *watchPreviews(req, ctx) {
        needSession(ctx);
        const queue: Preview[][] = [previewsOf(placeOf(req)).map((p) => ({ ...p }))];
        let wake = () => {};
        const push = (list: Preview[]) => { queue.push(list); wake(); };
        const stop = () => wake();
        const set = previewWatchers.get(placeOf(req)) ?? new Set();
        previewWatchers.set(placeOf(req), set); set.add(push);
        ctx.signal.addEventListener('abort', stop);
        try {
          while (!ctx.signal.aborted) {
            while (queue.length) yield { previews: queue.shift()! };
            await new Promise<void>((resolve) => { wake = resolve; if (queue.length || ctx.signal.aborted) resolve(); });
          }
        } finally {
          set.delete(push);
          ctx.signal.removeEventListener('abort', stop);
        }
      },
      startPreview(req, ctx) {
        needSession(ctx);
        const p = needPreview(req.id);
        if (p.state === PreviewState.RUNNING || p.state === PreviewState.STARTING) return {};
        p.state = PreviewState.STARTING; p.error = ''; tellPreviews(placeOf(p));
        // The sample runs no command. The preview "runs" a moment later.
        later(2500, () => { if (p.state !== PreviewState.STARTING) return; p.state = PreviewState.RUNNING; p.startedAt = timestampFromMs(Date.now()); tellPreviews(placeOf(p)); });
        return {};
      },
      stopPreview(req, ctx) {
        needSession(ctx);
        const p = needPreview(req.id);
        p.state = PreviewState.STOPPED; p.error = ''; delete p.startedAt;
        for (const [frameId, f] of frames) if (f.preview === p.id) frames.delete(frameId);
        tellPreviews(placeOf(p));
        return {};
      },
      openPreview(req, ctx) {
        needSession(ctx);
        const p = needPreview(req.id);
        if (p.state !== PreviewState.RUNNING) throw new ConnectError('This preview is not running.', Code.FailedPrecondition);
        const token = secret().replaceAll('-', '') + secret().replaceAll('-', '');
        const frame = { id: id('frame'), token, expiresAt: Date.now() + FRAME_LIFE, preview: p.id };
        frames.set(frame.id, frame);
        return { frame: { id: frame.id, url: previewUrl(token), expiresAt: timestampFromMs(frame.expiresAt) } };
      },
      releasePreview(req, ctx) {
        needSession(ctx);
        frames.delete(req.id);
        return {};
      },
      savePreview(req, ctx) {
        needSession(ctx);
        const old = req.id ? needPreview(req.id) : undefined;
        const projectId = old ? placeOf(old) : placeOf(req);
        if (!projectId) throw new ConnectError('Name the project.', Code.InvalidArgument);
        const name = req.name.trim(); const command = req.command.trim();
        if (!name) throw new ConnectError('A preview needs a name.', Code.InvalidArgument);
        if (!command) throw new ConnectError('A preview needs a command.', Code.InvalidArgument);
        if (!Number.isInteger(req.port) || req.port < 1 || req.port > 65535) throw new ConnectError('The port is a number from 1 to 65535.', Code.InvalidArgument);
        const list = previewsOf(projectId);
        if (list.some((x) => x.id !== old?.id && x.name.toLowerCase() === name.toLowerCase())) throw new ConnectError(`Another preview is named "${name}".`, Code.AlreadyExists);
        const machine = req.clientId ? need(req.clientId) : undefined;
        if (old) {
          const moved = old.command !== command || old.port !== req.port || (machine && machine.id !== old.clientId);
          old.name = name; old.command = command; old.port = req.port;
          if (machine) { old.clientId = machine.id; old.clientName = machine.name; }
          // The command that runs is the old one, so the preview stops and its addresses end.
          if (moved && old.state !== PreviewState.STOPPED) { old.state = PreviewState.STOPPED; old.error = ''; delete old.startedAt; for (const [frameId, f] of frames) if (f.preview === old.id) frames.delete(frameId); }
          tellPreviews(projectId);
          return { preview: { ...old } };
        }
        const first = list[0];
        const made: Preview = { $typeName: 'lgtm.v1.Preview', id: id('pv'), ...idsOf(projectId), name, command, port: req.port, clientId: machine?.id ?? first?.clientId ?? '', clientName: machine?.name ?? first?.clientName ?? '', branch: first?.branch ?? '', state: PreviewState.STOPPED, error: '' };
        list.push(made); tellPreviews(projectId);
        return { preview: { ...made } };
      },
      removePreview(req, ctx) {
        needSession(ctx);
        const p = needPreview(req.id);
        for (const [frameId, f] of frames) if (f.preview === p.id) frames.delete(frameId);
        previews.set(placeOf(p), previewsOf(placeOf(p)).filter((x) => x.id !== p.id));
        tellPreviews(placeOf(p));
        return {};
      },
    });
  };
  return { routes, frameKind };
}

// A token in the shape the Machines page shows: five groups of four letters and digits, with no look-alikes.
function secret(): string {
  const letters = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789';
  const bytes = crypto.getRandomValues(new Uint8Array(20));
  return [0, 1, 2, 3, 4].map((g) => [0, 1, 2, 3].map((i) => letters[bytes[g * 4 + i]! % letters.length]).join('')).join('-');
}
