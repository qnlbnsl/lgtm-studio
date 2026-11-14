export { Terminal } from '@xterm/xterm'
export function terminalSize(width: number, height: number, cellWidth: number, cellHeight: number) {
  return { cols: Math.min(240, Math.max(20, Math.floor(width / Math.max(1, cellWidth)))), rows: Math.min(100, Math.max(5, Math.floor(height / Math.max(1, cellHeight)))) }
}
interface Worktree { id: string; repo: string; project?: string | null; machine: string }
export function terminalWorktrees<W extends Worktree>(state: { worktrees?: ReadonlyArray<W> }, project: { id: string; repo: string }) {
  return (state.worktrees ?? []).filter(w => w.repo === project.repo && (!w.project || w.project === project.id))
}
export function terminalState(enabled: boolean, connection: string, machine: string) {
  return !enabled ? 'off' : connection !== 'live' ? 'offline' : machine !== 'local' ? 'remote' : 'ready'
}
