export const terminalReasons = {
  owner: 'Only the owner can run a project terminal.',
  connection: 'This terminal belongs to another connection.',
  missing: 'The terminal session is no longer available.',
  busy: 'Eight project terminal sessions are already open.',
  failed: 'The project shell could not run.',
  ended: 'The terminal has ended.',
  bounds: 'The terminal input or size is invalid.',
  stream: 'This terminal already has an output stream.',
  takeover: 'Output is open in another owner connection. Typing still works.',
  off: 'The project terminal is turned off.',
  remote: 'Project terminals are available only on the local machine.',
  workspace: 'The project worktree is unavailable.'
} as const
