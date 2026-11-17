// Use a separate draft namespace in the live copy. Guard the shared shell
// and sign-in projection too, so source drift fails the build for review.
export function startupSource(path: string, source: string): string {
  const markers: Record<string, string> = {
    'js/app.js': 'const word = connectionWord(c);',
    'js/data.js': "drafts: composerDrafts(live.on ? 'live' : 'mock', FRAMED)",
    'js/screens/service.js': "if (x.signInKnown === false)",
  }
  const marker = markers[path]
  if (!marker) return source
  if (source.split(marker).length !== 2) throw new Error(`Startup integration no longer matches ${path}`)
  return path === 'js/data.js' ? source.replace(marker, "drafts: composerDrafts('live', FRAMED)") : source
}
