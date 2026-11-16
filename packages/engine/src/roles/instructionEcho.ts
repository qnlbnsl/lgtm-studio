// Remove only the exact, ephemeral prefix supplied to this turn. Buffer at
// most that prefix, across arbitrary user-message chunks. A truncated echo
// of instructions remains private. A mismatch releases ordinary user text.
export const instructionEcho = (prefix: string) => {
  let matched = '', done = prefix.length === 0
  return { push: (text: string): string => {
    if (done) return text
    const candidate = matched + text
    if (candidate.startsWith(prefix)) { done = true; matched = ''; return candidate.slice(prefix.length) }
    if (prefix.startsWith(candidate)) { matched = candidate; return '' }
    done = true; matched = ''; return candidate
  } }
}
