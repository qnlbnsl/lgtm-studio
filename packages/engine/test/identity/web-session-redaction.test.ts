import { expect, it } from 'vitest'
import * as Redacted from 'effect/Redacted'
import { hasValidChecksum, mintCredential } from '../../src/identity/credential.ts'
import { makeRedactor } from '../../src/identity/redactor.ts'
import { registerRebuildCheck } from '../support/rebuildCheck.ts'
registerRebuildCheck({ noLog: true })
it('s28 hash-only web tokens redact raw and encoded forms while remaining invalid agent tokens', () => {
  const minted = mintCredential('shp_w_'), token = Redacted.value(minted.token)
  expect(hasValidChecksum(token, 'shp_a_')).toBe(false)
  const redactor = makeRedactor({ secrets: () => [], tokenHashes: () => new Set([minted.tokenHash]) })
  const forms = [token, Buffer.from(token).toString('base64'), Buffer.from(token).toString('base64url'), Buffer.from(token).toString('hex'), [...token].map(c => '%' + c.charCodeAt(0).toString(16)).join('')]
  for (const form of forms) {
    expect(redactor.redactText('before ' + form + ' after')).toBe('before [REDACTED] after')
    let text = ''
    for (let i = 0; i < form.length; i++) text += redactor.redactChunk(form, form[i]!, false)
    text += redactor.redactChunk(form, '', true)
    expect(text.includes(form)).toBe(false)
    expect(text).toContain('[REDACTED]')
  }
})
