// Use a separate draft namespace in the live copy. Guard the shared shell
// and sign-in projection too, so source drift fails the build for review.
export function startupSource(path: string, source: string): string {
  const markers: Record<string, string> = {
    'js/app.js': 'const word = connectionWord(c);',
    'js/dialogs.js': 'const ok = usableSubscription(x)',
    'js/screens/messages.js': "x.signInKnown === false ? 'Not reported'",
    'js/screens/auth.js': '&& usableSubscription(x)',
    'js/ui.js': 'disabled: !usableSubscription(x)',
    'js/data.js': "drafts: composerDrafts(live.on ? 'live' : 'mock', FRAMED)",
    'js/screens/service.js': "if (x.signInKnown === false)",
  }
  const marker = markers[path]
  if (path === 'js/app.js') {
    for (const required of ['if (waiting) content =', 'chat.reset(patch.session.status)']) {
      if (source.split(required).length !== 2) throw new Error(`Startup integration no longer matches ${path}`)
    }
  }
  if (path === 'js/screens/service.js' && !source.includes('usableSubscription(x)')) throw new Error(`Startup integration no longer matches ${path}`)
  if (!marker) return source
  if (source.split(marker).length !== 2) throw new Error(`Startup integration no longer matches ${path}`)
  return path === 'js/data.js' ? source.replace(marker, "drafts: composerDrafts('live', FRAMED)") : source
}
