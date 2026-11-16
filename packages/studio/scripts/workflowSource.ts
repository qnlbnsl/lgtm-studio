// Strip the sample clock from the live build, even if a mapped record lacks timing.
export function workflowSource(source: string): string {
  const pattern = /function phaseTimes\(s, p, cur\) \{[\s\S]*?\n\}/g
  const matches = [...source.matchAll(pattern)]
  if (matches.length !== 1) throw new Error('Workflow timing integration no longer matches')
  return source.replace(pattern, 'function phaseTimes(s, p, cur) { return recordedPhaseTimes(p); }')
}
