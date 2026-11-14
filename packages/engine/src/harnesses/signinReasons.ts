export const signInReasons = {
  owner: 'Only the owner can run a sign-in session.',
  connection: 'This sign-in belongs to another connection.',
  missing: 'The sign-in session is no longer available.',
  busy: 'A sign-in is already running for this account.',
  command: 'No usable sign-in command is shipped for this harness.',
  installed: 'The harness program is not installed.',
  failed: 'The sign-in program could not run.',
  ended: 'The sign-in has ended.',
  bounds: 'The terminal input or size is invalid.',
  stream: 'This sign-in already has an output stream.'
} as const
