// Only the engine build connects Files. Missing integration anchors fail the build.
export function filesSource(path: string, source: string): string {
  const replace = (from: string, to: string) => {
    if (source.split(from).length !== 2) throw new Error(`Files integration no longer matches ${path}: ${from}`)
    source = source.replace(from, to)
  }
  if (path === 'js/app.js') replace("['terminal', 'files'].includes(tab)", "tab === 'terminal'")
  if (path === 'js/screens/files.js') {
    source = "import { LiveFilesTab } from '../live/files.js';\nimport { live } from '../lib.js';\n" + source
    replace('export function FilesTab({ s, project, route }) {', 'export function FilesTab({ s, project, route }) {\n  if (live.on) return html`<${LiveFilesTab} s=${s} project=${project} route=${route} />`;')
  }
  if (path === 'js/editor.js') {
    replace('const p = parts.current;', 'const p = parts.current;\n    const nonce = document.querySelector(\'meta[name="lgtm-style-nonce"]\')?.getAttribute(\'content\');')
    replace('lineNumbers(),', '...(nonce ? [EditorView.cspNonce.of(nonce)] : []),\n          lineNumbers(),')
  }
  return source
}
