// Fail closed if the copied dialogs stop loading the private sign-in panel.
export function signInSource(path: string, source: string): string {
  if (path === 'js/live/git.js') {
    const reset = 'Object.keys(HARNESSES).forEach((k) => { delete HARNESSES[k]; });'
    if (!source.includes(reset)) throw new Error(`Chat harness metadata no longer matches ${path}`)
    return source.replace(reset, "Object.keys(HARNESSES).forEach((k) => { if (!(S().agents || []).some(a => a.harness === k)) delete HARNESSES[k]; });")
  }
  if (path !== 'js/screens/service.js') return source
  const marker = "import * as signIn from '../live/signin.js';"
  if (!source.includes(marker)) throw new Error(`Sign-in integration no longer matches ${path}`)
  const confirmation = "body: live.on ? 'The account is removed. Its sign-in and running agents are kept.' : 'The subscription and its sign-in are removed from LGTM Studio.'"
  if (!source.includes(confirmation)) throw new Error(`Sign-in removal confirmation no longer matches ${path}`)
  return source.replace(confirmation, "body: 'The account is removed from LGTM Studio. Its login folder is kept: ' + signIn.folderLabel(x) + '. Running agents are kept.'")
}
