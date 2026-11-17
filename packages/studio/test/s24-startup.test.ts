import { expect, it, vi } from 'vitest'
import type { EventEnvelope, Account, ClientSnapshot } from '@lgtm/contracts'
import { mapThread } from '../src/chat.ts'
import { mapSubscriptions } from '../src/mapping.ts'
import { engineSnapshot } from './fixtures.ts'
import { mapAccounts } from '../src/git.ts'

it('keeps the latest plan at its first position within each turn, live and on replay', () => {
  const item = (sequence: number, content: object) => ({ sequence, occurredAt: sequence, event: { _tag: 'ThreadItem', agentId: 'agent', itemId: String(sequence), content } }) as EventEnvelope
  const plan = (sequence: number, status: string) => item(sequence, { _tag: 'plan', entries: [{ content: 'Check the change', status, priority: 'high' }] })
  const first = plan(1, 'pending'), text = item(2, { _tag: 'text', text: 'Working.' }), last = plan(3, 'completed')
  expect(mapThread([first, text], 'agent').items[0]).toMatchObject({ id: '1', text: '- pending: Check the change' })
  const expected = [{ id: '1', text: '- completed: Check the change' }, { id: '2', text: 'Working.' }]
  expect(mapThread([first, text, last], 'agent').items).toMatchObject(expected)
  expect(mapThread([last, first, text, last].map(row => ({ ...row, event: { ...row.event, replay: true } })) as EventEnvelope[], 'agent').items).toMatchObject(expected)
  for (const tag of ['TurnEnded', 'SessionOpened', 'AgentInterrupted', 'AgentStopped', 'AgentCrashed', 'TurnStarted']) {
    const boundary = { sequence: 4, occurredAt: 4, event: { _tag: tag, agentId: 'agent' } } as EventEnvelope
    expect(mapThread([first, last, boundary, plan(5, 'pending')], 'agent').items.filter(row => String(row.text).includes('Check the change'))).toHaveLength(2)
  }
})

it('requires positive sign-in evidence instead of availability', () => {
  const account = { id: 'account', harness: 'codex' } as Account
  const rows = (value: object) => [value] as ClientSnapshot['accounts']
  for (const status of ['available', 'out-of-usage', undefined]) {
    expect(mapAccounts([account], status ? rows({ id: 'account', status }) : [], [])[0]).toMatchObject({ signedIn: false, signInKnown: false })
  }
  expect(mapAccounts([account], rows({ id: 'account', status: 'available', signedIn: true }), [])[0]).toMatchObject({ signedIn: true, signInKnown: true })
  expect(mapAccounts([account], rows({ id: 'account', status: 'needs-sign-in', signedIn: true }), [])[0]).toMatchObject({ signedIn: false, signInKnown: true })
})

it('stores only bounded composer drafts in per-tab storage, and removes sent drafts', async () => {
  const path = '../../../design/mock/js/drafts.js'; const drafts = await import(path)
  sessionStorage.clear()
  const first = drafts.composerDrafts('test')
  first['thread:alice'] = 'Unsent message'
  first['board:channel'] = 'Unsent post'
  first['password:login'] = 'never keep this'
  expect(drafts.composerDrafts('test')).toMatchObject({ 'thread:alice': 'Unsent message', 'board:channel': 'Unsent post' })
  expect(JSON.stringify(sessionStorage)).not.toContain('never keep this')
  expect(drafts.composerDrafts('other')).toEqual({})
  first['thread:alice'] = ''
  expect(drafts.composerDrafts('test')['thread:alice']).toBeUndefined()
  for (let i = 0; i < 100; i++) first['thread:' + i] = 'x'.repeat(100_000)
  expect((sessionStorage.getItem('lgtm:composer:test') || '').length).toBeLessThanOrEqual(65_536)
})

it('never infers sign-in from an agent account reference in a snapshot', () => {
  const agents = engineSnapshot.agents
  expect(mapSubscriptions([], agents, 'local').every(row => !row.signedIn)).toBe(true)
  const rows = [{ id: agents[0]!.account, status: 'available', enabled: true, environment: [] }]
  expect(mapSubscriptions(rows, agents, 'local')[0]).toMatchObject({ signedIn: false, signInKnown: false })
  expect(mapSubscriptions([{ ...rows[0]!, signedIn: true }], agents, 'local')[0]).toMatchObject({ signedIn: true, signInKnown: true })
})


it('treats authenticated startup replies as contact through a long recovery and detects a later outage', async () => {
  vi.useFakeTimers()
  let unavailable = false
  const { engineConnection } = await import('../src/connection.ts')
  const Effect = await import('effect/Effect')
  const state: import('../src/connection.ts').StorePatch = {}
  const connection = engineConnection({ location: { protocol: 'http:', host: '127.0.0.1:12345' }, debug() {},
    connect: (() => Effect.fail(new Error('not ready'))) as never,
    fetch: (async () => {
      if (unavailable) throw new Error('connection lost')
      return new Response(JSON.stringify({ _tag: 'ServiceStarting', recovered: 1, total: 2 }), { status: 503 })
    }) as typeof fetch })
  try {
    connection.open(patch => Object.assign(state, patch))
    await vi.advanceTimersByTimeAsync(60_000)
    expect(state.session?.status).toBe('in')
    expect(state.conn).toMatchObject({ status: 'reconnecting', starting: { recovered: 1, total: 2 } })
    unavailable = true
    await vi.advanceTimersByTimeAsync(40_000)
    expect(state.conn?.status).toBe('offline')
    expect(state.conn?.starting).toBeUndefined()
  } finally { connection.close(); vi.useRealTimers() }
})
