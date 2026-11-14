// Only the engine build connects Terminal. Every anchor must occur exactly once.
export function terminalSource(path: string, source: string): string {
  const replace = (from: string, to: string) => {
    if (source.split(from).length !== 2) throw new Error(`Terminal integration no longer matches ${path}: ${from}`)
    source = source.replace(from, to)
  }
  if (path === 'js/app.js') {
    replace("  else if (live.on && tab === 'terminal') body = html`<${NotConnected} what=${tab === 'files' ? 'Files' : 'The terminal'} />`;\n", '')
    replace('liveFiles.observe(patch);', 'liveTerminal.observe(patch);\n    liveFiles.observe(patch);')
    source = "import * as liveTerminal from './live/terminal.js';\n" + source
  }
  if (path === 'js/screens/terminal.js') {
    source = "import { LiveTerminalTab } from '../live/terminal.js';\nimport { live as liveTerminalMode } from '../lib.js';\n" + source
    replace('export function TerminalTab({ s, project }) {', 'export function TerminalTab({ s, project }) {\n  if (liveTerminalMode.on) return html`<${LiveTerminalTab} s=${s} project=${project} />`;')
  }
  if (path === 'js/screens/service.js') replace('<${Row} label="Web page"', '<${Row} label="Project terminal" hint="Turning it on lets signed-in owner pages run shells as you on the service machine."><${Switch} on=${set.terminalEnabled === true} label="Project terminal" onChange=${(value) => liveGit.saveSettings({ terminalEnabled: value })} /><//>\n      <${Row} label="Web page"')
  if (path === 'js/live/git.js') replace("} else if (key === 'webEnabled') {", "} else if (key === 'terminalEnabled') {\n    const { requestId, serverState, ...previous } = service.web;\n    await request('client.web.settings.set', { settings: { ...previous, terminalEnabled: patch.terminalEnabled } });\n  } else if (key === 'webEnabled') {")
  if (path === 'index.html') replace('<link rel="stylesheet" href="css/terminal.css">', '<link rel="stylesheet" href="css/terminal.css">\n<link rel="stylesheet" href="css/xterm.css">\n<link rel="stylesheet" href="css/live-terminal.css">')
  return source
}
// xterm 6 creates three DOM renderer styles. Add the page nonce before they
// are attached. This dependency transform fails closed when its source changes.
// The dependency remains an ordinary npm package; no library code is copied.
export function xtermSource(source: string): string {
  let count = 0
  const output = source.replace(/(\b[\w]+(?:\.[\w]+)*)\.createElement\("style"\)/g, (_, doc) => { count++; return `lgtmTerminalStyle(${doc})` })
  if (count !== 3) throw new Error(`xterm style integration expected 3 anchors, found ${count}`)
  return `function lgtmTerminalStyle(doc) { const element = doc.createElement('style'); const nonce = doc.querySelector('meta[name="lgtm-style-nonce"]')?.getAttribute('content'); if (!nonce) throw new Error('The terminal style nonce is missing.'); element.nonce = nonce; return element; }\n` + output
}
