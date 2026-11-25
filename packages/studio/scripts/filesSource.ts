// Only the engine build connects Files. Missing integration anchors fail the build.
function declaresFilesBinding(source: string): boolean {
  const name = '(?:live|LiveFilesTab)\\b'
  // Fail conservatively for destructuring and comma-separated declarations too.
  const declaration = new RegExp('\\b(?:const|let|var|function|class)\\s+' + name)
  const pattern = new RegExp('\\b(?:const|let|var)\\s+[\\{\\[][^=;]*\\b' + name)
  const sequence = new RegExp('\\b(?:const|let|var)\\s+[^;\\n]*,\\s*' + name)
  const imports = [...source.matchAll(/\bimport\s+([\s\S]*?)\s+from\s*['"]/g)]
  return declaration.test(source) || pattern.test(source) || sequence.test(source)
    || imports.some(([, clause]) => clause!.split(/[{},]/).some((part) => /^(?:\*\s+as\s+)?(?:live|LiveFilesTab)$/.test(part.trim()) || /\bas\s+(?:live|LiveFilesTab)$/.test(part.trim())))
}

export function filesSource(path: string, source: string): string {
  const replace = (from: string, to: string) => {
    if (source.split(from).length !== 2) throw new Error(`Files integration no longer matches ${path}: ${from}`)
    source = source.replace(from, to)
  }
  if (path === 'index.html') replace('<link rel="stylesheet" href="css/files.css">', '<link rel="stylesheet" href="css/files.css">\n<link rel="stylesheet" href="css/live-files.css">')
  if (path === 'js/app.js') {
    replace("['terminal', 'files'].includes(tab)", "tab === 'terminal'")
    replace('liveGit.observe(patch);', 'liveFiles.observe(patch);\n    liveGit.observe(patch);')
    source = "import * as liveFiles from './live/files.js';\n" + source
  }
  if (path === 'js/screens/files.js') {
    if (declaresFilesBinding(source)) throw new Error(`Files integration already declares live or LiveFilesTab in ${path}`)
    source = "import { LiveFilesTab } from '../live/files.js';\nimport { live } from '../lib.js';\n" + source
    replace('export function FilesTab({ s, project, route }) {', 'export function FilesTab({ s, project, route }) {\n  if (live.on) return html`<${LiveFilesTab} s=${s} project=${project} route=${route} />`;')
  }
  if (path === 'js/ui.js') replace('aria-checked=${it.checked ?? null} disabled=${it.disabled}', 'aria-checked=${it.checked ?? null} disabled=${it.disabled} title=${it.disabled && it.why === "Engine work needed" ? it.why : null}')
  if (path === 'js/editor.js') {
    replace('readOnly = false, wrap = false, onChange, onSave', 'readOnly = false, busy = false, wrap = false, onChange, onSave')
    replace('const lockOf = (ro) => [EditorState.readOnly.of(ro), EditorView.editable.of(!ro)];', 'const lockOf = (ro) => [EditorState.readOnly.of(ro || busy), EditorView.editable.of(!ro)];')
    replace('}, [readOnly]);', '}, [readOnly, busy]);')
    replace('const p = parts.current;', 'const p = parts.current;\n    const nonce = document.querySelector(\'meta[name="lgtm-style-nonce"]\')?.getAttribute(\'content\');')
    replace('lineNumbers(),', '...(nonce ? [EditorView.cspNonce.of(nonce)] : []),\n          lineNumbers(),')
  }
  return source
}
