// Cancellation still rejects, so every reader can release its state in finally.
// Only presenters discard the mark; genuine failures retain their message.
export function errorText(error: unknown, fallback = 'The service did not complete the request.'): string {
  const e = error as { cancelledRead?: boolean; message?: string } | null | undefined;
  if (e?.cancelledRead === true) return '';
  return typeof error === 'string' ? error : e?.message || fallback;
}
