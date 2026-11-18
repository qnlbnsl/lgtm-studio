import { expect, it } from 'vitest'
import { TerminalHistory } from '../../src/harnesses/terminalHistory.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
registerRebuildCheck({ noLog: true })
it('r2 the chunk ring bounds UTF-8 bytes and chunk metadata, retaining recent output', () => {
  const history = new TerminalHistory()
  history.append('OLD')
  for (let i = 0; i < 70000; i++) history.append('é')
  history.append('RECENT')
  const replay = history.replay()
  expect(Buffer.byteLength(replay)).toBeLessThanOrEqual(64 * 1024)
  expect(Buffer.byteLength(replay)).toBeGreaterThan(60 * 1024)
  expect(replay.endsWith('RECENT')).toBe(true)
  expect(replay).not.toContain('OLD')
  expect(replay).not.toContain('\uFFFD')
  history.clear(); expect(history.replay()).toBe('')
})
it('r2 a large native chunk retains a bounded suffix without breaking Unicode', () => {
  const history = new TerminalHistory()
  history.append('🤖'.repeat(50000) + 'LATEST')
  expect(Buffer.byteLength(history.replay())).toBeLessThanOrEqual(65536)
  expect(history.replay().endsWith('LATEST')).toBe(true)
  expect(history.replay()).not.toContain('\uFFFD')
})
it('r2 replay skips a truncated CSI or OSC prefix even when it crosses chunks', () => {
  for (const [prefix, tail] of [['\x1b[', '31mSAFE'], ['\x1b]0;', 'title\x07SAFE'], ['\x1bP', 'payload\x1b\\SAFE']]) {
    const history = new TerminalHistory()
    history.append(prefix! + (prefix === '\x1b[' ? '1' : 'x').repeat(65536))
    history.append(tail!)
    expect(history.replay()).toBe('SAFE')
  }
})
