// The website's connection to the engine.
//
// In the real product, every change the owner makes is a client API call: an Effect RPC
// request named by its tag in @lgtm/contracts, such as "prompt" or "answer". Every update
// the engine pushes arrives on the "subscribe" stream. The mock has no engine, so it applies
// each change in the browser, and it reports the call here as well.
//
// A page that loads the mock as the package `lgtm` can pass its own connection to mount().
// That connection receives every call and every pushed update, so it can show them, or carry
// them over a different route, without the screens changing. The default connection is the
// service on this computer, and it does nothing.
//
// Live mode. A page that passes { mode: 'live' } to mount() gives a connection that also has:
//   open(write)        starts reading the engine. It calls write(patch) with whole store keys
//                      in the store's shape, such as { session }, { conn } or { repos, projects,
//                      teams, agents, requests, machines, owner }. Only this route changes them.
//   signIn(password)   resolves to the session's new status: 'in', 'rejected' or 'unavailable'
//   signOut()          ends this browser's signed-in session
//   reconnect()        tries the engine again at once
//   close()            stops reading, when the website is unmounted
// The live connection is written in TypeScript in packages/studio. No file here knows which
// transport it uses.

import type { State } from './types';

/** A set of mapping functions the live connection lends to a live module. Their shapes belong to packages/studio. */
export type LiveMaps = Record<string, (...args: any[]) => any>;
/** The connection a page passes to mount(). Only `call` and `push` exist in the mock; the rest is live mode. */
export interface Connection {
  kind?: string; label?: string;
  call(tag: string, payload: unknown): void;
  push(kind: string, payload: unknown): void;
  request?(tag: string, payload?: unknown, options?: unknown): Promise<any>;
  open?(write: (patch: Partial<State>) => void): void;
  signIn?(password: string): Promise<string>;
  signOut?(): Promise<unknown> | void;
  reconnect?(): void;
  close?(): void;
  signInOutput?(sessionId: string, onItem: (item: any) => void, onClose?: () => void): () => void;
  terminalOutput?(id: string, onEvent: (event: any) => void, onClose?: () => void): () => void;
  board?: LiveMaps; mocks?: LiveMaps;
}

const local: Connection = { kind: 'local', label: 'the service on this computer', call() {}, push() {} };
let current: Connection = local;

export function setConnection(connection: Connection | null | undefined): void { current = connection || local; }
export function connection(): Connection { return current; }

// A request from the website to the engine. `tag` is the RPC's tag in @lgtm/contracts.
export function call(tag: string, payload: unknown): void {
  try { current.call(tag, payload); } catch (e) { console.error('The connection failed to send', tag, e); }
}

// An update from the engine to the website, as an item on the "subscribe" stream.
export function push(kind: string, payload: unknown): void {
  try { current.push(kind, payload); } catch (e) { console.error('The connection failed to deliver', kind, e); }
}

// The live connection owns request ids and the transport.
// Resolves with the engine reply, or rejects with its refusal reason.
// In the mock there is no request(), and a call throws, as it did before the port.
export function request(tag: string, payload?: unknown, options?: unknown): Promise<any> { return current.request!(tag, payload, options); }
