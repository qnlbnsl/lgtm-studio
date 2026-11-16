export function alertLinksSource(path: string, source: string): string {
  if (path !== 'js/screens/auth.js') return source
  const marker = "live.on ? alertsLive.afterSignIn(s.session.firstRun) : s.session.firstRun ? '/setup' : '/home'"
  if (!source.includes(marker)) throw new Error(`Notification sign-in route no longer matches ${path}`)
  return source.replace(marker, "alertsLive.afterSignIn(s.session.firstRun)")
}
