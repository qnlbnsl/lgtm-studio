import { describe, expect, it } from 'vitest'
import { terminalSize, terminalWorktrees, terminalState, terminalSelection } from '../src/terminal.ts'
import { terminalSource, xtermSource } from '../scripts/terminalSource.ts'
import { readFileSync } from 'node:fs'
describe('project terminal mapping and build boundary', () => {
  it('bounds the renderer grid and identifies local and offline worktrees', () => {
    expect(terminalSize(10000, 10000, 8, 16)).toEqual({ cols: 240, rows: 100 })
    expect(terminalSize(1, 1, 8, 16)).toEqual({ cols: 20, rows: 5 })
    const state = { worktrees: [{ id: 'a', repo: 'r', project: 'p', machine: 'local' }, { id: 'b', repo: 'other', machine: 'local' }, { id: 'unassigned', repo: 'r', machine: 'local', checkout: 'worktree' }, { id: 'base', repo: 'r', machine: 'local', checkout: 'main' }] }
    expect(terminalWorktrees(state, { id: 'p', repo: 'r', worktree: 'a' }).map(w => w.id)).toEqual(['a', 'base'])
    expect(terminalState(true, 'offline', 'local')).toBe('offline')
    expect(terminalState(true, 'live', 'remote')).toBe('remote')
    expect(terminalState(false, 'live', 'local')).toBe('off')
  })
  it('keeps the selected worktree across tab mounts and defaults to the project folder', () => {
    const worktrees = [{ id: 'base' }, { id: 'project' }, { id: 'remote' }]
    expect(terminalSelection(worktrees, undefined, 'project')?.id).toBe('project')
    expect(terminalSelection(worktrees, 'base', 'project')?.id).toBe('base')
    expect(terminalSelection(worktrees, 'removed', 'project')?.id).toBe('project')
    expect(terminalSelection([], undefined, 'project')).toBeUndefined()
  })
  it('refuses missing or duplicated screen integration anchors', () => {
    const source = readFileSync('design/mock/js/screens/terminal.js', 'utf8')
    expect(terminalSource('js/screens/terminal.js', source)).toContain('LiveTerminalTab')
    expect(() => terminalSource('js/screens/terminal.js', source + source)).toThrow()
    expect(() => terminalSource('js/screens/terminal.js', '')).toThrow()
  })
  it('adds the page nonce before each renderer style element can be attached', () => {
    const source = readFileSync('packages/studio/node_modules/@xterm/xterm/lib/xterm.mjs', 'utf8')
    const built = xtermSource(source)
    expect(built).toContain('lgtm-style-nonce')
    expect(built).toContain('t.style.cssText+=e+";"')
    expect(() => xtermSource(source.replace('setAttribute("style"', 'setAttribute("changed"'))).toThrow()
    expect(built.match(/lgtmTerminalStyle\(/g)).toHaveLength(4)
    expect(() => xtermSource(source + source)).toThrow()
    expect(() => xtermSource('')).toThrow()
  })
})

it('includes allocated project workers and excludes another project in the same repository', () => {
  const state = { worktrees: [
    { id: 'project', repo: 'r', project: 'p', machine: 'local', checkout: 'worktree' },
    { id: 'worker', repo: 'r', project: 'p', machine: 'local', checkout: 'worktree', parentId: 'project' },
    { id: 'other-worker', repo: 'r', project: 'other', machine: 'local', checkout: 'worktree', parentId: 'other' },
  ] }
  expect(terminalWorktrees(state, { id: 'p', repo: 'r', worktree: 'project' }).map(w => w.id)).toEqual(['project', 'worker'])
})
