// Keep the roster's live preview tied to the sample screen's existing working lane.
export function smallfixSource(path: string, source: string): string {
  if (path !== 'js/screens/messages.js') return source
  const marker = 'live.on ? board.currentActivity(a) : a.activity'
  if (source.split(marker).length !== 2) throw new Error(`Roster activity integration no longer matches ${path}`)
  return source.replace(marker, 'board.currentActivity(a)')
}
