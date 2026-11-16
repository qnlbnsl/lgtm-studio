// A queued read belongs to the screen that asked for it. Query changes keep it.
let page = new AbortController();
const route = () => typeof location === 'undefined' ? '' : location.hash.split('?')[0];
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

// Dropping a cancelled read must run neither its success nor its error UI.
// The abandoned promise has no listeners left on the connection or page.
export function silenceCancelledRead(reply) {
  return reply.catch(error => {
    if (error?.name === 'AbortError' && error.cancelledRead === true) return new Promise(() => {});
    throw error;
  });
}
