// Cancellation still rejects, so every reader can release its state in finally.
// Only presenters discard the mark; genuine failures retain their message.
export function errorText(error, fallback = 'The service did not complete the request.') {
  if (error?.cancelledRead === true) return '';
  return typeof error === 'string' ? error : error?.message || fallback;
}
