// The Files tab's editor. It is CodeMirror 6, from the one bundle in
// third_party/codemirror (MIT License), loaded only when a file is opened so
// the rest of the page stays small. Colours come from the page's tokens, so the
// editor follows the light and dark modes without a theme of its own. It is a
// small editor for minor changes: no autocomplete or other language features.
import { useEffect, useRef, useState } from "react"

type Bundle = typeof import("../../../../third_party/codemirror/codemirror.bundle.js")
let loading: Promise<Bundle> | undefined
const load = () => loading ??= import("../../../../third_party/codemirror/codemirror.bundle.js")

const extensionOf = (path: string) => (/\.([a-z0-9]+)$/i.exec(path)?.[1] ?? "").toLowerCase()
function languageFor(cm: Bundle, path: string) {
  const ext = extensionOf(path)
  if (["ts", "tsx", "mts", "cts"].includes(ext)) return cm.javascript({ typescript: true, jsx: ext === "tsx" })
  if (["js", "jsx", "mjs", "cjs"].includes(ext)) return cm.javascript({ jsx: ext === "jsx" })
  if (ext === "md" || ext === "mdx") return cm.markdown()
  if (ext === "css") return cm.css()
  if (ext === "json") return cm.json()
  if (ext === "yaml" || ext === "yml") return cm.yaml()
  return []
}
function looks(cm: Bundle) {
  const t = cm.tags
  const colours = cm.HighlightStyle.define([
    { tag: [t.keyword, t.controlKeyword, t.definitionKeyword, t.moduleKeyword, t.operatorKeyword, t.modifier, t.self], color: "var(--color-accent)" },
    { tag: [t.string, t.special(t.string), t.regexp], color: "var(--color-success)" },
    { tag: [t.comment, t.lineComment, t.blockComment, t.docComment], color: "var(--color-muted)", fontStyle: "italic" },
    { tag: [t.number, t.bool, t.null, t.atom, t.unit], color: "var(--color-warn)" },
    { tag: [t.typeName, t.className, t.tagName, t.namespace], color: "var(--color-primary)" },
    { tag: t.heading, color: "var(--color-primary)", fontWeight: "var(--weight-strong)" },
    { tag: t.strong, fontWeight: "var(--weight-strong)" },
    { tag: t.emphasis, fontStyle: "italic" },
    { tag: [t.link, t.url], color: "var(--color-accent)", textDecoration: "underline" },
    { tag: t.invalid, color: "var(--color-error)" }
  ])
  const theme = cm.EditorView.theme({
    "&": { height: "100%", fontSize: "var(--text-sm, 13px)", color: "var(--color-text)", backgroundColor: "var(--color-surface)" },
    "&.cm-focused": { outline: "none" },
    ".cm-scroller": { fontFamily: "var(--font-mono, monospace)", lineHeight: "1.6", fontVariantLigatures: "none" },
    ".cm-content": { padding: "8px 0 32px", caretColor: "var(--color-text)" },
    ".cm-gutters": { backgroundColor: "var(--color-surface)", color: "var(--color-muted)", border: "none" },
    ".cm-activeLine, .cm-activeLineGutter": { backgroundColor: "var(--color-raised)" },
    ".cm-cursor, .cm-dropCursor": { borderLeftColor: "var(--color-text)" },
    "&.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection": { backgroundColor: "var(--color-selected)" },
    ".cm-panels": { backgroundColor: "var(--color-sidebar)", color: "var(--color-text)" }
  })
  return [cm.syntaxHighlighting(colours), theme]
}

export interface CodeEditorProps {
  readonly doc: string
  readonly path: string
  readonly readOnly: boolean
  readonly wrap: boolean
  readonly onChange: (text: string) => void
  readonly onSave: () => void
}
// One editor for one file. The parent gives it a `key` for each file, so
// opening another file starts a fresh editor with its own undo history.
export function CodeEditor({ doc, path, readOnly, wrap, onChange, onSave }: CodeEditorProps) {
  const host = useRef<HTMLDivElement>(null)
  const view = useRef<any>(undefined)
  const parts = useRef<{ lock: any; wrap: any; cm: Bundle } | undefined>(undefined)
  const calls = useRef({ onChange, onSave }); calls.current = { onChange, onSave }
  const latest = useRef({ readOnly, wrap }); latest.current = { readOnly, wrap }
  const [failed, setFailed] = useState(false)
  useEffect(() => {
    let cancelled = false
    void load().then((cm) => {
      if (cancelled || host.current === null) return
      const lock = new cm.Compartment(), lines = new cm.Compartment()
      parts.current = { lock, wrap: lines, cm }
      const lockOf = (value: boolean) => [cm.EditorState.readOnly.of(value), cm.EditorView.editable.of(!value)]
      view.current = new cm.EditorView({
        parent: host.current,
        state: cm.EditorState.create({ doc, extensions: [
          cm.lineNumbers(), cm.highlightActiveLineGutter(), cm.highlightSpecialChars(), cm.history(), cm.drawSelection(),
          cm.indentOnInput(), cm.bracketMatching(), cm.highlightActiveLine(), cm.highlightSelectionMatches(),
          ...looks(cm), languageFor(cm, path),
          cm.keymap.of([{ key: "Mod-s", preventDefault: true, run: () => { calls.current.onSave(); return true } },
            ...cm.defaultKeymap, ...cm.historyKeymap, ...cm.searchKeymap, cm.indentWithTab]),
          lock.of(lockOf(latest.current.readOnly)), lines.of(latest.current.wrap ? cm.EditorView.lineWrapping : []),
          cm.EditorView.contentAttributes.of({ "aria-label": `Contents of ${path}` }),
          cm.EditorView.updateListener.of((update: any) => { if (update.docChanged) calls.current.onChange(update.state.doc.toString()) })
        ] })
      })
    }).catch(() => { if (!cancelled) setFailed(true) })
    return () => { cancelled = true; view.current?.destroy(); view.current = undefined }
  }, [])
  useEffect(() => {
    const p = parts.current
    if (p !== undefined) view.current?.dispatch({ effects: p.lock.reconfigure([p.cm.EditorState.readOnly.of(readOnly), p.cm.EditorView.editable.of(!readOnly)]) })
  }, [readOnly])
  useEffect(() => {
    const p = parts.current
    if (p !== undefined) view.current?.dispatch({ effects: p.wrap.reconfigure(wrap ? p.cm.EditorView.lineWrapping : []) })
  }, [wrap])
  if (failed) return <p role="alert">The editor could not load. Reload the page.</p>
  return <div className="files-editor" ref={host} />
}
