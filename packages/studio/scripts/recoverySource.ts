// Keep the mock's existing stopped composer and the website's crash label
// together. A changed source must be reviewed before the live build succeeds.
export function recoverySource(path: string, source: string): string {
  if (path === 'js/data.js') {
    const marker = "if (a.crashed) return { dot: 'failed', word: a.activity || 'Crashed', tone: 'error' };"
    if (!source.includes(marker)) throw new Error(`Recovery status no longer matches ${path}`)
    return source.replace(marker, "if (a.crashed === true) return { dot: 'failed', word: a.activity || 'Crashed', tone: 'error' };")
  }
  if (path !== 'js/ui.js') return source
  const marker = "a.crashed ? a.name + ': ' + a.activity : a.name + ' is stopped.'"
  if (!source.includes(marker)) throw new Error(`Recovery composer no longer matches ${path}`)
  return source.replace(marker, "a.crashed ? a.name + ': ' + (a.activity || 'Crashed') : a.name + ' is stopped.'")
}
