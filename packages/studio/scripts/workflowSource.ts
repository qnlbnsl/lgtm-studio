// The live build cannot use the mock's weighted sample clock.
export function workflowSource(source: string): string {
  const from = 'const times = phaseTimes(s, p, cur);'
  if (source.split(from).length !== 2) throw new Error('Workflow timing integration no longer matches')
  return source.replace(from, 'const times = recordedPhaseTimes(p);')
}
