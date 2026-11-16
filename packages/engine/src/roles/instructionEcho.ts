// Once the engine header is recognized, a changed echo remains private.
// Keep only a closing-marker-sized tail while searching for the block end.
export const instructionEcho = (prefix: string) => {
  const header = prefix.startsWith('Engine role instructions [') ? 'Engine role instructions [' : prefix.slice(0, prefix.indexOf('\n') + 1)
  const closing = prefix.trimEnd().split('\n').at(-1)!
  let pending = '', state: 'header' | 'block' | 'separator' | 'done' = prefix.length === 0 ? 'done' : 'header'
  let newlines = 0
  return { push: (text: string): string => {
    if (state === 'done') return text
    pending += text
    if (state === 'header') {
      if (header && pending.replaceAll('\r\n', '\n').startsWith(header)) state = 'block'
      else if (prefix.startsWith(pending) || (header && header.startsWith(pending.replaceAll('\r\n', '\n').replace(/\r$/, '')))) return ''
      else { state = 'done'; const ordinary = pending; pending = ''; return ordinary }
    }
    if (state === 'block') {
      const end = pending.indexOf(closing)
      if (end < 0) { pending = pending.slice(-closing.length); return '' }
      pending = pending.slice(end + closing.length)
      state = 'separator'
    }
    if (state === 'separator') {
      while (pending.length > 0 && newlines < 2) {
        if (pending[0] === '\r') { pending = pending.slice(1); continue }
        if (pending[0] !== '\n') break
        newlines++; pending = pending.slice(1)
      }
      if (pending.length === 0 && newlines < 2) return ''
      state = 'done'
    }
    const caller = pending; pending = ''; return caller
  } }
}
