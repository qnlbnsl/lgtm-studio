// A queued read belongs to the page that asked for it. Writes ignore this signal.
let page = new AbortController();
let hash = typeof location === 'undefined' ? '' : location.hash;
const reset = () => { page.abort(); page = new AbortController(); hash = typeof location === 'undefined' ? '' : location.hash; };
if (typeof addEventListener === 'function') {
  addEventListener('hashchange', reset);
  addEventListener('pagehide', reset);
}
export function readOptions(options) {
  if (typeof location !== 'undefined' && location.hash !== hash) reset();
  return { ...options, readSignal: page.signal };
}
