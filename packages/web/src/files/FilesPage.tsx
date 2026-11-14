// The project's Files tab: the inspected worktree as a folder tree, and the
// open file beside it. The service lists one folder or reads one file at a
// time through the client API, so a large repository never loads at once. The
// owner can edit a text file, save it, and commit that one file. Anything
// bigger is work for the project lead.
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react"
import type { FileContent, Project, RosterEntry, Workspace } from "@lgtm/contracts"
import * as Effect from "effect/Effect"
import { requestId, withClient } from "../thread/client.ts"
import { visibleText } from "../thread/text.tsx"
import { probeSession } from "../git/state.ts"
import type { ConnectionState } from "../watchSituation.ts"
import { CodeEditor } from "./editor.tsx"
import "./files.css"

type Entry = { readonly name: string; readonly path: string; readonly kind: "folder" | "file"; readonly size: number | null; readonly change: string | null }
type Listing = { readonly folder: string; readonly items: ReadonlyArray<Entry>; readonly truncated: boolean }
type FolderState = { readonly status: "loading" } | { readonly status: "ready"; readonly listing: Listing } | { readonly status: "failed" }
type Opened = { readonly path: string; readonly status: "loading" } | { readonly path: string; readonly status: "ready"; readonly file: FileContent } | { readonly path: string; readonly status: "failed"; readonly message: string }

export interface FilesPageProps {
  readonly project: Project
  readonly workspace: Workspace
  readonly file?: string | undefined
  readonly onOpen: (path: string | undefined) => void
  readonly gitHref: string
  readonly agents: ReadonlyArray<RosterEntry>
  readonly mutationDisabled: boolean
  readonly connection: ConnectionState | "Offline"
  readonly onSessionExpired: () => void
}

// Unsaved text lives only in this page, and survives opening another file or
// another tab. It never reaches browser storage.
const drafts = new Map<string, string>()
const draftKey = (worktreeId: string, path: string) => `${worktreeId}\u0000${path}`
const changeLabels: Readonly<Record<string, readonly [string, string]>> = {
  added: ["A", "Added"], modified: ["M", "Changed"], deleted: ["D", "Deleted"], renamed: ["R", "Renamed"], untracked: ["U", "New file"], conflict: ["C", "Conflict"]
}
const sizeLabel = (bytes: number) => bytes < 1024 ? `${bytes} B` : bytes < 1024 * 1024 ? `${(bytes / 1024).toFixed(1)} KB` : `${(bytes / 1024 / 1024).toFixed(1)} MB`
const parentOf = (path: string) => path.includes("/") ? path.slice(0, path.lastIndexOf("/")) : ""
const plural = (count: number, word: string) => `${count.toLocaleString()} ${word}${count === 1 ? "" : "s"}`
const isMac = typeof navigator !== "undefined" && /Mac|iPhone|iPad/u.test(navigator.platform)
const result = <A,>(reply: { readonly result: A } | { readonly data: string }): A => {
  if ("result" in reply) return reply.result
  throw new Error("The reply was unavailable")
}
const failureReason = (failure: unknown) => typeof failure === "object" && failure !== null && "_tag" in failure && failure._tag === "CommandError"
  && "reason" in failure && typeof failure.reason === "string" ? failure.reason : undefined
const readMessage = (failure: unknown) => failureReason(failure) === "Git: invalid_path"
  ? "This file cannot be shown. It may be ignored by Git, a link, or a private file."
  : "The file could not be read. Check the connection, then refresh."

export function FilesPage(props: FilesPageProps) {
  // A different worktree or machine starts a fresh tree and file.
  return <FilesWorkspace key={`${props.workspace.worktreeId}\u0000${props.workspace.machineId}`} {...props} />
}

function FilesWorkspace({ project, workspace, file: selected, onOpen, gitHref, agents, mutationDisabled, connection, onSessionExpired }: FilesPageProps) {
  const remote = workspace.machineId !== "local"
  const live = connection === "Live" && !remote
  const base = ["main", "master"].includes(workspace.branch)
  const [folders, setFolders] = useState<Readonly<Record<string, FolderState>>>({})
  const [open, setOpen] = useState<ReadonlySet<string>>(() => new Set())
  const [revision, setRevision] = useState(0)
  const [opened, setOpened] = useState<Opened>()
  const [, setDraftVersion] = useState(0)
  const [wrap, setWrap] = useState(false)
  const [notice, setNotice] = useState("")
  const [saving, setSaving] = useState(false)
  const [conflict, setConflict] = useState<Extract<FileContent, { kind: "text" }>>()
  const [committing, setCommitting] = useState(false)
  const [query, setQuery] = useState("")
  const [found, setFound] = useState<{ readonly query: string; readonly paths: ReadonlyArray<string>; readonly truncated: boolean }>()
  const expired = useRef(onSessionExpired); expired.current = onSessionExpired
  const controller = useRef(new AbortController())
  useEffect(() => () => controller.current.abort(), [])

  const loadFolder = (folder: string) => {
    setFolders((previous) => previous[folder]?.status === "ready" ? previous : { ...previous, [folder]: { status: "loading" } })
    const signal = controller.current.signal
    void withClient((client) => client["project.files.list"]({ requestId: requestId(), worktreeId: workspace.worktreeId,
      ...(folder === "" ? {} : { folder }) }).pipe(Effect.result), signal).then((outcome) => {
      if (signal.aborted) return
      if (outcome._tag === "Success") setFolders((previous) => ({ ...previous, [folder]: { status: "ready", listing: result(outcome.success) as Listing } }))
      else { setFolders((previous) => ({ ...previous, [folder]: { status: "failed" } })); void probeSession(signal, expired.current) }
    }).catch(() => { if (!signal.aborted) setFolders((previous) => ({ ...previous, [folder]: { status: "failed" } })) })
  }
  const loadFile = (path: string, keep = false) => {
    if (!keep) setOpened({ path, status: "loading" })
    const signal = controller.current.signal
    return withClient((client) => client["project.files.read"]({ requestId: requestId(), worktreeId: workspace.worktreeId, path }).pipe(Effect.result), signal)
      .then((outcome) => {
        if (signal.aborted) return undefined
        if (outcome._tag === "Success") {
          const file = result(outcome.success) as FileContent
          if (!keep) setOpened({ path, status: "ready", file })
          return file
        }
        if (!keep) setOpened({ path, status: "failed", message: readMessage(outcome.failure) })
        void probeSession(signal, expired.current)
        return undefined
      }).catch(() => { if (!signal.aborted && !keep) setOpened({ path, status: "failed", message: readMessage(undefined) }); return undefined })
  }

  // The root and every open folder are read again on each refresh. Opening a
  // file opens the folders that hold it.
  useEffect(() => {
    if (!live) return
    for (const folder of ["", ...open]) loadFolder(folder)
  }, [live, revision])
  useEffect(() => {
    if (selected === undefined) return
    const parents = selected.split("/").slice(0, -1).map((_, index, parts) => parts.slice(0, index + 1).join("/"))
    setOpen((previous) => { const next = new Set(previous); for (const folder of parents) next.add(folder); return next })
    if (live) for (const folder of parents) loadFolder(folder)
  }, [selected, live])
  useEffect(() => {
    setConflict(undefined); setNotice("")
    if (selected === undefined || !live) { setOpened(undefined); return }
    void loadFile(selected)
  }, [selected, live, revision])
  useEffect(() => {
    const q = query.trim()
    if (q === "" || !live) { setFound(undefined); return }
    const signal = new AbortController()
    const timer = setTimeout(() => {
      void withClient((client) => client["project.context.search"]({ requestId: requestId(), projectId: project.projectId, worktreeId: workspace.worktreeId,
        query: q, scope: "files", limit: 50 }), signal.signal).then((reply) => {
        if (signal.signal.aborted || !("result" in reply)) return
        setFound({ query: q, paths: reply.result.items.flatMap((item) => item.reference._tag === "file" ? [item.reference.path] : []), truncated: reply.result.truncated })
      }).catch(() => { if (!signal.signal.aborted) setFound({ query: q, paths: [], truncated: false }) })
    }, 150)
    return () => { clearTimeout(timer); signal.abort() }
  }, [query, live])

  const toggle = (folder: string) => {
    setOpen((previous) => { const next = new Set(previous); if (next.has(folder)) next.delete(folder); else next.add(folder); return next })
    if (!open.has(folder) && live) loadFolder(folder)
  }
  const entryOf = (path: string) => {
    const folder = folders[parentOf(path)]
    return folder?.status === "ready" ? folder.listing.items.find((row) => row.path === path) : undefined
  }
  const dirtyPaths = new Set([...drafts.keys()].filter((key) => key.startsWith(`${workspace.worktreeId}\u0000`)).map((key) => key.slice(workspace.worktreeId.length + 1)))
  const working = agents.filter((agent) => agent.folder === workspace.path && agent.state._tag === "working")
  const textFile = opened?.status === "ready" && opened.file.kind === "text" ? opened.file : undefined
  const key = textFile && draftKey(workspace.worktreeId, textFile.path)
  const shown = textFile ? drafts.get(key!) ?? textFile.text : ""
  const dirty = textFile !== undefined && drafts.has(key!)
  const readOnlyReason = !textFile ? "" : base ? "Agents never edit the base branch directly."
    : !textFile.editable ? "This file holds private data, so it is shown redacted and cannot be saved here."
    : mutationDisabled ? "Saving waits for the connection." : ""
  const readOnly = readOnlyReason !== "" || saving
  const change = opened ? entryOf(opened.path)?.change ?? null : null

  const edit = (text: string) => {
    if (!textFile || readOnly) return
    const was = drafts.has(key!)
    if (text === textFile.text) drafts.delete(key!); else drafts.set(key!, text)
    if (was !== drafts.has(key!)) setDraftVersion((value) => value + 1)
  }
  const save = async (version = textFile?.version) => {
    if (!textFile || readOnly || !drafts.has(key!) || version === undefined) return
    const text = drafts.get(key!)!, path = textFile.path, signal = controller.current.signal
    setSaving(true); setNotice("")
    try {
      const outcome = await withClient((client) => client["project.files.write"]({ requestId: requestId(), worktreeId: workspace.worktreeId,
        projectId: project.projectId, path, version, text }).pipe(Effect.result), signal)
      if (signal.aborted) return
      if (outcome._tag === "Success") {
        const saved = outcome.success.result
        drafts.delete(key!); setConflict(undefined)
        setOpened({ path, status: "ready", file: { ...textFile, text, version: saved.version, size: new TextEncoder().encode(text).length } })
        setNotice(`Saved ${path.split("/").pop()}${saved.told > 0 ? `. ${saved.told === 1 ? "The agent working here was" : `${saved.told} agents working here were`} told which file you changed.` : "."}`)
        loadFolder(parentOf(path))
        return
      }
      const reason = failureReason(outcome.failure)
      if (reason?.startsWith("the file changed since you opened it")) {
        // Both versions stay visible: the owner's text in the editor, and the
        // file as it is now below it.
        const current = await loadFile(path, true)
        if (!signal.aborted && current?.kind === "text") setConflict(current)
        setNotice("The file changed since you opened it. Your text is kept. Compare it with the current file below.")
      } else setNotice(reason === undefined || reason === "command refused" ? "The save was not confirmed. Reload the file to check what it holds." : `The save was refused: ${reason}.`)
    } catch { if (!signal.aborted) setNotice("The save was not confirmed. Reload the file to check what it holds.") }
    finally { if (!signal.aborted) { setSaving(false); setDraftVersion((value) => value + 1) } }
  }
  const takeCurrent = () => {
    if (!conflict) return
    drafts.delete(key!); setOpened({ path: conflict.path, status: "ready", file: conflict }); setConflict(undefined); setNotice("")
  }

  const tree = (folder: string, depth: number): ReactNode => {
    const state = folders[folder]
    if (state === undefined || state.status === "loading") return <li className="files-note" style={{ paddingLeft: 8 + depth * 14 }}>Loading…</li>
    if (state.status === "failed") return <li className="files-note" style={{ paddingLeft: 8 + depth * 14 }}>Could not read this folder. <button className="inline-link" onClick={() => loadFolder(folder)}>Retry</button></li>
    if (state.listing.items.length === 0) return <li className="files-note" style={{ paddingLeft: 8 + depth * 14 }}>Empty</li>
    return <>{state.listing.items.map((entry) => {
      const label = changeLabels[entry.change ?? ""]
      if (entry.kind === "folder") {
        const expanded = open.has(entry.path)
        return <li key={entry.path} role="treeitem" aria-expanded={expanded} aria-label={entry.name}>
          <button className="files-row folder" style={{ paddingLeft: 8 + depth * 14 }} onClick={() => toggle(entry.path)}>
            <span className="files-chevron" aria-hidden="true">{expanded ? "▾" : "▸"}</span><span className="files-name">{visibleText(entry.name)}</span>
            {entry.change !== null && <i className="files-dot" title="Has changes" />}
          </button>
          {expanded && <ul role="group">{tree(entry.path, depth + 1)}</ul>}
        </li>
      }
      return <li key={entry.path} role="treeitem" aria-selected={selected === entry.path} aria-label={entry.name}>
        <button className={`files-row${selected === entry.path ? " on" : ""}`} style={{ paddingLeft: 22 + depth * 14 }} onClick={() => onOpen(entry.path)}>
          <span className="files-name">{visibleText(entry.name)}</span>
          {dirtyPaths.has(entry.path) && <i className="files-unsaved" title="Not saved" />}
          {label && <span className={`files-change ${entry.change}`} title={label[1]}>{label[0]}</span>}
        </button>
      </li>
    })}{state.listing.truncated && <li className="files-note" style={{ paddingLeft: 8 + depth * 14 }}>Some entries are not shown.</li>}</>
  }

  const crumbs = selected?.split("/") ?? []
  const lines = textFile ? shown.replace(/\n$/u, "").split("\n").length : 0
  const imageUrl = useMemo(() => {
    if (opened?.status !== "ready" || opened.file.kind !== "image") return undefined
    const bytes = Uint8Array.from(atob(opened.file.data), (character) => character.charCodeAt(0))
    return URL.createObjectURL(new Blob([bytes], { type: opened.file.mediaType }))
  }, [opened])
  useEffect(() => () => { if (imageUrl) URL.revokeObjectURL(imageUrl) }, [imageUrl])

  let body: ReactNode
  if (remote) body = <div className="files-state"><strong>Files on this machine are awaiting machine support.</strong><span>Only worktrees on this computer can be read today.</span></div>
  else if (connection !== "Live") body = <div className="files-state"><strong>Offline</strong><span>The files show again when the connection returns.</span></div>
  else if (selected === undefined) body = <div className="files-state"><strong>No file open</strong><span>Choose a file in the tree.</span></div>
  else if (opened === undefined || opened.status === "loading") body = <div className="files-state" role="status">Reading the file…</div>
  else if (opened.status === "failed") body = <div className="files-state" role="alert"><strong>{opened.message}</strong><code>{visibleText(opened.path)}</code></div>
  else if (opened.file.kind === "image") body = <div className="files-picture">{imageUrl && <img src={imageUrl} alt={visibleText(opened.file.path)} />}</div>
  else if (opened.file.kind === "binary" || opened.file.kind === "large") body = <div className="files-state">
    <strong>{opened.file.kind === "binary" ? "Binary file" : "Too large to show"} · {sizeLabel(opened.file.size)}</strong>
    <span>{opened.file.kind === "binary" ? "It cannot be read here." : "Files over 512 KB are not shown here."}</span></div>
  else body = <>
    {working.length > 0 && <p className="files-busy" role="note">{working.map((agent) => visibleText(agent.name)).join(", ")} {working.length === 1 ? "is" : "are"} working in this worktree. When you save, {working.length === 1 ? "it is" : "they are"} told which file you changed.</p>}
    {readOnlyReason && <p className="files-readonly" role="note">Read-only. {readOnlyReason}</p>}
    <CodeEditor key={`${workspace.worktreeId}:${opened.file.path}:${opened.file.version}`} doc={shown} path={opened.file.path} wrap={wrap} readOnly={readOnly} onChange={edit} onSave={() => { void save() }} />
    {conflict && <section className="files-conflict" aria-label="The file as it is now">
      <header><strong>The file as it is now</strong>
        <button onClick={takeCurrent}>Use the current file</button>
        <button disabled={readOnly} onClick={() => { void save(conflict.version) }}>Save my text over it</button></header>
      <pre>{visibleText(conflict.text)}</pre>
    </section>}
  </>

  const side = <aside className="files-side" aria-label="Files">
    <div className="files-where"><code title={visibleText(workspace.branch)}>{visibleText(workspace.branch)}</code>
      <small title={visibleText(workspace.path)}>{visibleText(workspace.machineId === "local" ? "This computer" : workspace.machineId)} · {visibleText(workspace.path)}</small></div>
    <label className="files-find"><span className="sr-only">Go to file</span><input placeholder="Go to file" value={query} maxLength={256} disabled={!live} onChange={(event) => setQuery(event.target.value)} /></label>
    <div className="files-tree">
      {found && query.trim() !== "" ? <ul aria-label="Matching files">{found.paths.map((path) => <li key={path}>
          <button className={`files-row${selected === path ? " on" : ""}`} onClick={() => { onOpen(path); setQuery("") }}><span className="files-name">{visibleText(path)}</span></button></li>)}
          {found.paths.length === 0 && <li className="files-note">No file matches</li>}
          {found.truncated && <li className="files-note">More files match. Type more of the name.</li>}</ul>
        : live ? <ul role="tree" aria-label={`Files in ${visibleText(workspace.branch)}`}>{tree("", 0)}</ul> : null}
    </div>
  </aside>

  const head = selected !== undefined && <header className="files-head">
    <button className="files-back" aria-label="Back to the files" onClick={() => onOpen(undefined)}>←</button>
    <p className="files-crumbs" title={visibleText(selected)}>{crumbs.length > 1 && <span>{visibleText(crumbs.slice(0, -1).join(" / "))} / </span>}<b>{visibleText(crumbs.at(-1) ?? "")}</b></p>
    {change !== null && changeLabels[change] && <span className={`files-change ${change}`}>{changeLabels[change]![1]}</span>}
    <span className="files-gap" />
    {opened?.status === "ready" && opened.file.kind !== "binary" && opened.file.kind !== "large" && <small className="files-size">{lines ? `${plural(lines, "line")} · ` : ""}{sizeLabel(opened.file.size)}</small>}
    {textFile && <label className="files-wrap"><input type="checkbox" checked={wrap} onChange={(event) => setWrap(event.target.checked)} />Wrap</label>}
    {change !== null && <a className="files-button" href={gitHref}>Open in Git</a>}
    {textFile && !readOnlyReason && <button className={dirty ? "files-primary" : ""} disabled={!dirty || saving} title={isMac ? "⌘S" : "Ctrl+S"} onClick={() => { void save() }}>{saving ? "Saving…" : "Save"}</button>}
    {textFile && !readOnlyReason && change !== null && !dirty && <button className="files-primary" disabled={mutationDisabled} onClick={() => setCommitting(true)}>Commit</button>}
    <button onClick={() => setRevision((value) => value + 1)} disabled={!live || dirty}>Refresh</button>
  </header>

  return <section className={`files-page${selected !== undefined ? " has-file" : ""}`} aria-label="Files">
    {side}
    <section className="files-main" aria-label="Open file">{head}{notice && <p className="files-notice" role="status">{notice}</p>}{body}</section>
    {committing && textFile && <CommitFile workspace={workspace} path={textFile.path} disabled={mutationDisabled}
      close={() => setCommitting(false)} done={(head) => { setCommitting(false); setNotice(`Committed ${textFile.path.split("/").pop()} as ${head.slice(0, 7)}.`); loadFolder(parentOf(textFile.path)) }}
      onSessionExpired={onSessionExpired} />}
  </section>
}

// The commit dialog for one file. Only that file is committed, as the owner.
function CommitFile({ workspace, path, disabled, close, done, onSessionExpired }: { readonly workspace: Workspace; readonly path: string; readonly disabled: boolean
  readonly close: () => void; readonly done: (head: string) => void; readonly onSessionExpired: () => void }) {
  const dialog = useRef<HTMLDialogElement>(null)
  const [message, setMessage] = useState("")
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  useEffect(() => {
    const previous = document.activeElement
    dialog.current?.showModal?.()
    return () => { if (previous instanceof HTMLElement && previous.isConnected) previous.focus({ preventScroll: true }) }
  }, [])
  const submit = async () => {
    if (busy || disabled || message.trim() === "") return
    setBusy(true); setError("")
    const signal = new AbortController().signal
    try {
      const outcome = await withClient((client) => client["project.commit"]({ requestId: requestId(), worktreeId: workspace.worktreeId, message: message.trim(), path }).pipe(Effect.result), signal)
      if (outcome._tag === "Success") { done(outcome.success.result.head); return }
      const reason = failureReason(outcome.failure)
      setError(reason === "Git: nothing_staged" ? "This file has no change to commit." : reason === "Git: conflicts" ? "Resolve the conflicts in Git first." : "The commit was not confirmed. Check the Git tab before trying again.")
      void probeSession(signal, onSessionExpired)
    } catch { setError("The commit was not confirmed. Check the Git tab before trying again.") }
    finally { setBusy(false) }
  }
  return <dialog ref={dialog} className="files-dialog" aria-label={`Commit ${path.split("/").pop()}`} onCancel={(event) => { event.preventDefault(); close() }}>
    <form onSubmit={(event) => { event.preventDefault(); void submit() }}>
      <h2>Commit {visibleText(path.split("/").pop() ?? path)}</h2>
      <label htmlFor="files-commit-message">Commit message</label>
      <textarea id="files-commit-message" rows={3} autoFocus maxLength={8192} value={message} onChange={(event) => setMessage(event.target.value)}
        onKeyDown={(event) => { if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) { event.preventDefault(); void submit() } }} />
      <p>Only <code>{visibleText(path)}</code> is committed, to <code>{visibleText(workspace.branch)}</code>, as you. Other staged files stay staged. For anything bigger, ask the project lead.</p>
      {error && <p role="alert">{error}</p>}
      <footer><button type="button" onClick={close}>Cancel</button><button type="submit" className="files-primary" disabled={busy || disabled || message.trim() === ""}>{busy ? "Committing…" : "Commit"}</button></footer>
    </form>
  </dialog>
}
