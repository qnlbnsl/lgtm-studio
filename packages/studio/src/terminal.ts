export { Terminal } from '@xterm/xterm'
export function terminalSize(width: number, height: number, cellWidth: number, cellHeight: number) {
  return { cols: Math.min(240, Math.max(20, Math.floor(width / Math.max(1, cellWidth)))), rows: Math.min(100, Math.max(5, Math.floor(height / Math.max(1, cellHeight)))) }
}
interface Worktree { id: string; repo: string; project?: string | null; machine: string; checkout?: string; parentId?: string | null }
export function terminalWorktrees<W extends Worktree>(state: { worktrees?: ReadonlyArray<W>; teams?: ReadonlyArray<{ project: string; worktree?: string }> }, project: { id: string; repo: string; worktree?: string }) {
  return (state.worktrees ?? []).filter(w => w.repo === project.repo && (w.id === project.worktree || w.project === project.id || w.checkout === 'main'
    || state.teams?.some(t => t.project === project.id && (t.worktree === w.id || t.worktree === w.parentId && w.checkout === 'worktree'))))
}
export function terminalState(enabled: boolean, connection: string, machine: string) {
  return !enabled ? 'off' : connection !== 'live' ? 'offline' : machine !== 'local' ? 'remote' : 'ready'
}

// Keep the current choice while it exists. A new tab starts in the project's
// worktree, as the former folder chooser did.
export function terminalSelection<W extends { id: string }>(worktrees: ReadonlyArray<W>, selectedId: string | undefined, projectWorktree: string | undefined): W | undefined {
  return worktrees.find(w => w.id === selectedId) ?? worktrees.find(w => w.id === projectWorktree) ?? worktrees[0]
}

// Match engine identifiers to the same registered worktrees used by New session.
export function terminalAttachments<W extends { id: string }>(sessions: ReadonlyArray<{ sessionId: string; projectId: string; worktreeId: string }>, worktrees: ReadonlyArray<W>, projectId: string) {
  return sessions.filter(session => session.projectId === projectId).flatMap(session => { const worktree = worktrees.find(w => w.id === session.worktreeId); return worktree ? [{ sessionId: session.sessionId, worktree }] : [] })
}
