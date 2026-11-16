import { expect, it } from 'vitest'
import { connectionSource } from '../scripts/connectionSource.ts'

it('fails the live build if the connection notice integration anchor changes', () => {
  expect(() => connectionSource('changed source')).toThrow('Connection notice integration')
  expect(connectionSource('${c.notice || (c.status ===')).toContain('${connectionNotice(c) || (c.status ===')
})
