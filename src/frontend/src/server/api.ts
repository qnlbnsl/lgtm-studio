// The website's side of the contract in src/contracts.
//
// This file is the only one that knows how a call reaches the server. Every screen calls a
// typed client from here, and the server may be the real one, the mock server on a port, or
// the sample engine inside this page.
import { createClient, createRouterTransport, type Client, type Transport } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { ClientService } from '@lgtm/api/clients';
import { MapService } from '@lgtm/api/map';
import { PreviewService } from '@lgtm/api/preview';
import { SessionService, type GetServerResponse } from '@lgtm/api/session';
import { SAMPLE } from './address';

/** The revision of the contract this website speaks. */
export const CONTRACT = 1;
export interface Api { session: Client<typeof SessionService>; clients: Client<typeof ClientService>; maps: Client<typeof MapService>; previews: Client<typeof PreviewService> }
let current: Api | null = null;
let base = '';

/** Turns an address the server gave, which may be a path on the server, into a full address. */
export function onServer(address: string): string { return new URL(address, base === SAMPLE || !base ? location.href : base + '/').href; }

/** The typed clients of the server the website is connected to. */
export function api(): Api {
  if (!current) throw new Error('The website has no server yet.');
  return current;
}

/** Thrown when something answers at the address and it is not a server this website can use. */
export class WrongServer extends Error {}

/** Connects to the server at an address, and returns what it says about itself. */
export async function openServer(url: string, sample: { signedIn: boolean }): Promise<GetServerResponse> {
  let transport: Transport;
  if (url === SAMPLE) {
    // The sample engine is loaded only when it is used, so a website for a real server does not carry it.
    const { createEngine } = await import('@lgtm/mock-server/engine');
    transport = createRouterTransport(createEngine({ inPage: true, signedIn: sample.signedIn }).routes);
  } else {
    // The session is a cookie of the server, so each call sends the server's cookies.
    transport = createConnectTransport({ baseUrl: url, fetch: (input, init) => fetch(input, { ...init, credentials: 'include' }) });
  }
  const next: Api = { session: createClient(SessionService, transport), clients: createClient(ClientService, transport), maps: createClient(MapService, transport), previews: createClient(PreviewService, transport) };
  const server = await next.session.getServer({}, { timeoutMs: 6000 });
  if (server.product !== 'lgtm-studio') throw new WrongServer('What answers there is not an LGTM Studio server.');
  if (server.contract !== CONTRACT) throw new WrongServer(`The server speaks revision ${server.contract} of the contract, and this website speaks revision ${CONTRACT}.`);
  current = next; base = url;
  return server;
}
