import { afterEach, expect, it, vi } from 'vitest'
import * as Redacted from 'effect/Redacted'
import { makeRedactor } from '../../src/identity/redactor.ts'
import { mintCredential } from '../../src/identity/credential.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
registerRebuildCheck({ noLog: true })
afterEach(() => vi.restoreAllMocks())
it('releases only the ambiguous lead after idle and retains covered bytes for a later encoded token', () => {
  let now = 0; vi.spyOn(Date, 'now').mockImplementation(() => now)
  const token = mintCredential('shp_a_'), encoded = Buffer.from('xy' + Redacted.value(token.token)).toString('base64')
  const redactor = makeRedactor({ secrets: () => [Redacted.make('registered-terminal-secret')], tokenHashes: () => new Set([token.tokenHash]) })
  expect(redactor.redactChunk('echo', 'ls', false, true)).toBe('')
  now = 29; expect(redactor.redactIdle('echo')).toBe('')
  now = 31; expect(redactor.redactIdle('echo')).toBe('ls')
  expect(redactor.redactIdle('echo')).toBe('')
  expect(redactor.redactChunk('echo', ' -a\r\n', false, true)).toBe(' -a\r\n')
  expect(redactor.redactChunk('encoded', encoded.slice(0, 3), false, true)).toBe('')
  now += 40; expect(redactor.redactIdle('encoded')).toBe(encoded.slice(0, 3))
  const rest = redactor.redactChunk('encoded', encoded.slice(3) + '\r\n', false, true) + redactor.redactChunk('encoded', '', true, true)
  expect(rest).not.toContain(encoded.slice(3)); expect(rest.replace('[REDACTED]', '')).toBe('\r\n')
})
it.each(['registered-terminal-', 'shp_', 'shp_a_', '7368705f', 'c2hwX2Ff', '%73', 's%68', '%73%68%70%5f', '%7'])('never releases a real secret or credential prefix on idle: %s', prefix => {
  let now = 0; vi.spyOn(Date, 'now').mockImplementation(() => now)
  const token = mintCredential('shp_a_')
  const redactor = makeRedactor({ secrets: () => [Redacted.make('registered-terminal-secret')], tokenHashes: () => new Set([token.tokenHash]) })
  redactor.redactChunk('strict', prefix, false, true)
  now = 1000; expect(redactor.redactIdle('strict')).toBe('')
  if (prefix === 'registered-terminal-') expect(redactor.redactChunk('strict', 'secret\r\n', false, true)).toBe('[REDACTED]\r\n')
})
