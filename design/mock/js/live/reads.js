// A queued read belongs to the screen that asked for it. Query changes keep it.
let page = new AbortController();
const route = () => typeof location === 'undefined' ? '' : location.hash.split('?')[0].replace(/^#\/subscriptions$/, '#/harnesses');
let path = route();
const reset = () => { page.abort(); page = new AbortController(); path = route(); };
const navigate = () => { if (route() !== path) reset(); };
if (typeof addEventListener === 'function') {
  addEventListener('hashchange', navigate);
  addEventListener('pagehide', reset);
}
export function readOptions(options) {
  navigate();
  return { ...options, readSignal: page.signal };
}

// Cancellation still rejects, so every reader can release its state in finally.
// Only presenters discard the mark; genuine failures retain their message.
export function errorText(error, fallback = 'The service did not complete the request.') {
  if (error?.cancelledRead === true) return '';
  return typeof error === 'string' ? error : error?.message || fallback;
}
