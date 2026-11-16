// A turn can split an echo across message identifiers or decorate its opening.
// Hold the prelude until the random closing marker establishes the boundary.
export const instructionEcho = (prefix: string) => {
  const header = prefix.startsWith('Engine role instructions [') ? 'Engine role instructions [' : prefix.split('\n')[0]!
  const closing = prefix.trimEnd().split('\n').at(-1)!
  const limit = prefix.length + 4096
  let pending = '', state: 'search' | 'separator' | 'done' = prefix.length === 0 ? 'done' : 'search'
  let recognized = false, overflow = false, newlines = 0
  return {
    push: (text: string): string => {
      if (state === 'done') return text
      pending += text
      if (state === 'search') {
        recognized ||= pending.includes(header)
        const end = pending.indexOf(closing)
        if (end < 0) {
          if (recognized || pending.length > limit) {
            overflow ||= pending.length > limit
            pending = pending.slice(-Math.max(closing.length, header.length))
          }
          return ''
        }
        pending = pending.slice(end + closing.length)
        state = 'separator'
      }
      while (pending.length > 0 && newlines < 2) {
        if (pending[0] === '\r') { pending = pending.slice(1); continue }
        if (pending[0] !== '\n') break
        newlines++; pending = pending.slice(1)
      }
      if (pending.length === 0 && newlines < 2) return ''
      state = 'done'
      const caller = pending; pending = ''; return caller
    },
    finish: (): string => {
      // Ordinary text with no instruction marker is still the caller's text.
      // A partial header or an unclosed block must never disclose instructions.
      const partial = Array.from({ length: Math.min(header.length - 1, pending.length) }, (_, n) => n + 1)
        .some((n) => pending.endsWith(header.slice(0, n)))
      const ordinary = state === 'search' && !recognized && !overflow && !partial ? pending : ''
      pending = ''; state = 'done'; return ordinary
    },
  }
}
