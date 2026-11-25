import * as Cause from 'effect/Cause';

// Service-owned categories only. System messages and causes may contain secrets.
const reasons = new Set([
  'home folder is unavailable', 'state folder refused', 'state folder cannot be inspected', 'state folder cannot be created',
  'state folder is a symbolic link', 'state folder mode is not 0700', 'state folder is owned by another user',
  'state folder is not a directory', 'socket path exceeds 100 bytes', 'stale socket removal failed',
  'service lock failed', 'database initialization failed',
  'owner credential is a symbolic link', 'owner credential cannot be opened', 'owner credential is not a regular file',
  'owner credential is owned by another user', 'owner credential mode is not 0600', 'owner credential is damaged',
  'owner credential cannot be created', 'owner credential cannot be loaded',
]);

export const startupFailure = (cause: Cause.Cause<unknown>, redact: (text: string) => string): string | undefined => {
  if (Cause.hasInterruptsOnly(cause)) return undefined;
  const reason = cause.reasons.flatMap(item => {
    if (item._tag !== 'Fail' || typeof item.error !== 'object' || item.error === null) return [];
    const error = item.error as { message?: unknown; reason?: unknown };
    const text = typeof error.reason === 'string' ? error.reason : error.message;
    return typeof text === 'string' && reasons.has(text) ? [text] : [];
  })[0] ?? 'service failed';
  // Redact the complete category before bounding it, so truncation cannot cut a secret.
  return redact(reason).slice(0, 512);
};
