// Require both existing request presenters to retain the owner's permission
// answer path when the standalone mock is copied into the live build.
export function heldPermissionSource(path: string, source: string): string {
  if (path === 'js/app.js' || path === 'js/screens/home.js' || path === 'js/screens/workflow.js') {
    const expected = path === 'js/app.js' ? 6 : 1
    if (source.split('answerableRequests(').length !== expected + 1) throw new Error(`Owner request audience no longer matches ${path}`)
    return source
  }
  if (path === 'js/data.js') {
    const marker = "live.on && r.kind === 'approval' && r.canAnswer"
    if (source.split(marker).length !== 2) throw new Error(`Held permission index no longer matches ${path}`)
    return source.replace(marker, "live.on && r.kind === 'approval' && r.canAnswer === true")
  }
  if (path !== 'js/ui.js') return source
  const held = "liveMode.on && r.kind === 'approval' && r.status === 'open' && r.heldBy"
  if (source.split(held).length !== 3) throw new Error(`Held permission holder no longer matches ${path}`)
  const marker = "liveMode.on && r.kind === 'approval' && r.canAnswer"
  if (source.split(marker).length !== 3) throw new Error(`Held permission controls no longer match ${path}`)
  return source.replaceAll(marker, "liveMode.on && r.kind === 'approval' && r.canAnswer === true")
}
