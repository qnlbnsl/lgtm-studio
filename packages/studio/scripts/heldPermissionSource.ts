// Require both existing request presenters to retain the owner's permission
// answer path when the standalone mock is copied into the live build.
export function heldPermissionSource(path: string, source: string): string {
  if (path === 'js/data.js') {
    const marker = "live.on && r.kind === 'approval' && r.canAnswer"
    if (!source.includes(marker)) throw new Error(`Held permission index no longer matches ${path}`)
    return source.replace(marker, "live.on && r.kind === 'approval' && r.canAnswer === true")
  }
  if (path !== 'js/ui.js') return source
  const marker = "liveMode.on && r.kind === 'approval' && r.canAnswer"
  if (source.split(marker).length !== 3) throw new Error(`Held permission controls no longer match ${path}`)
  return source.replaceAll(marker, "liveMode.on && r.kind === 'approval' && r.canAnswer === true")
}
