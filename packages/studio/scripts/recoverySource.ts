// Keep the mock's existing stopped composer and the website's crash label
// together. A changed source must be reviewed before the live build succeeds.
export function recoverySource(path: string, source: string): string {
  if (path === 'js/screens/messages.js') {
    const marker = "if (a.crashed) return { lane: 'needs', why: a.failure?.reason || 'Failed to start', tone: 'error' };"
    if (source.split(marker).length !== 2) throw new Error(`Recovery lane no longer matches ${path}`)
    return source.replace(marker, "if (a.crashed === true) return { lane: 'needs', why: a.failure?.reason || 'Failed to start', tone: 'error' };")
  }
  if (path === 'js/screens/workflow.js') {
    const waiting = "a.crashed || a.state === 'waiting' || a.state === 'failed'"
    const idle = "!a.crashed && (a.state === 'idle' || a.state === 'stopped')"
    if (source.split(waiting).length !== 2 || source.split(idle).length !== 2) throw new Error(`Recovery groups no longer match ${path}`)
    return source.replace(waiting, "a.crashed === true || a.state === 'waiting' || a.state === 'failed'").replace(idle, "a.crashed !== true && (a.state === 'idle' || a.state === 'stopped')")
  }
  if (path === 'js/data.js') {
    const marker = "if (a.crashed) return { dot: 'failed', word: a.activity || 'Crashed', tone: 'error' };"
    if (source.split(marker).length !== 2) throw new Error(`Recovery status no longer matches ${path}`)
    return source.replace(marker, "if (a.crashed === true) return { dot: 'failed', word: a.activity || 'Crashed', tone: 'error' };")
  }
  if (path !== 'js/ui.js') return source
  const marker = "a.crashed ? a.name + ': ' + a.activity : a.name + ' is stopped.'"
  if (source.split(marker).length !== 2) throw new Error(`Recovery composer no longer matches ${path}`)
  return source.replace(marker, "a.crashed ? a.name + ': ' + (a.activity || 'Crashed') : a.name + ' is stopped.'")
}
