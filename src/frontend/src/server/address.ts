// The address of the server this website talks to.
//
// The build may fix it with VITE_LGTM_SERVER. When that is not set, the website asks the owner
// once and keeps the answer in this browser.
const KEY = 'lgtm:server';
/** The address that means "the sample server inside this page". */
export const SAMPLE = 'sample';

/** The address the build fixed, or null when the website asks for one. */
export function fixedAddress(): string | null {
  const v = import.meta.env.VITE_LGTM_SERVER;
  return typeof v === 'string' && v.trim() ? v.trim() : null;
}
export function rememberedAddress(): string | null { try { return localStorage.getItem(KEY); } catch { return null; } }
export function rememberAddress(text: string | null): void {
  try { if (text === null) localStorage.removeItem(KEY); else localStorage.setItem(KEY, text); } catch { /* A private window keeps nothing. */ }
}

const LOOPBACK = /^(localhost|127(\.\d{1,3}){3}|\[::1\])$/i;
/** Thrown with a sentence the page can show, when the text is not an address. */
export class BadAddress extends Error {}

/**
 * Turns what the owner typed into the server's address, such as "https://lgtm.example.com".
 * "localhost:4180" and "lgtm.example.com" need no scheme: this computer is reached over http,
 * port 80 is http, and every other address is https. ":4180" means that port on the page's own host,
 * and "/" means the page's own address.
 */
export function serverUrl(text: string, page: { protocol: string; hostname: string; host: string } = location): string {
  const typed = text.trim();
  if (!typed) throw new BadAddress('Enter the server’s address.');
  if (typed === SAMPLE) return SAMPLE;
  // The website's own host passes the calls on to a server.
  if (typed === '/') return `${page.protocol}//${page.host}`;
  if (/^:\d{1,5}$/.test(typed)) return `${page.protocol}//${page.hostname}${typed}`;
  const hasScheme = /^[a-z][a-z0-9+.-]*:\/\//i.test(typed);
  let url: URL;
  try { url = new URL(hasScheme ? typed : 'https://' + typed); } catch { throw new BadAddress('That is not an address. Use a name or an IP address, with a port when it has one.'); }
  if (hasScheme && url.protocol !== 'http:' && url.protocol !== 'https:') throw new BadAddress('The address must start with http or https.');
  if (url.username || url.password) throw new BadAddress('The address must not hold a name or a password.');
  if (!hasScheme && (LOOPBACK.test(url.hostname) || /:80$/.test(typed.split('/')[0]!))) url.protocol = 'http:';
  return url.origin + url.pathname.replace(/\/+$/, '');
}
/** The address as the page shows it: the host, and the port when it is not the usual one. */
export function addressLabel(url: string): string {
  if (url === SAMPLE) return 'Sample data';
  try { const u = new URL(url); return u.host + u.pathname.replace(/\/+$/, ''); } catch { return url; }
}
