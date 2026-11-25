import { expect, it, vi } from 'vitest'
import * as Effect from 'effect/Effect'
import { engineConnection } from '../src/connection.ts'

it('clears composer memory and storage at authentication boundaries, preserving an outage', async () => {
  const { composerDrafts } = await import('../../../design/mock/js/drafts.js' as string)
  const { store } = await import('../../../design/mock/js/lib.js' as string)
  const { reset } = await import('../../../design/mock/js/live/chat.js' as string)
  for (const status of ['unavailable', 'out', 'expired', 'logout-unconfirmed']) {
    sessionStorage.clear()
    const drafts = composerDrafts('live')
    drafts['thread:agent'] = 'private draft'; drafts['board:channel'] = 'private post'
    store.state = { drafts, ui: {}, session: { status: 'in' } }
    reset(status)
    expect(Object.keys(drafts)).toHaveLength(status === 'unavailable' ? 2 : 0)
    expect(sessionStorage.getItem('lgtm:composer:live') !== null).toBe(status === 'unavailable')
  }
  sessionStorage.setItem('lgtm:composer:live', '{}')
  store.state = { drafts: composerDrafts('live'), ui: {}, session: { status: 'in' } }
  reset('expired')
  expect(sessionStorage.getItem('lgtm:composer:live')).toBeNull()
})

it('keeps an overlong composer in memory, stores none of it and warns only once', async () => {
  const { composerDrafts } = await import('../../../design/mock/js/drafts.js' as string)
  sessionStorage.clear()
  const notice = vi.fn(), drafts = composerDrafts('test', false, notice)
  drafts['thread:agent'] = 'short'
  const text = 'x'.repeat(8193)
  drafts['thread:agent'] = text
  drafts['thread:agent'] += 'y'
  expect(drafts['thread:agent']).toBe(text + 'y')
  expect(composerDrafts('test')['thread:agent']).toBeUndefined()
  expect(notice).toHaveBeenCalledTimes(1)
  drafts['thread:agent'] = ''
  drafts['thread:agent'] = text
  expect(notice).toHaveBeenCalledTimes(2)
})

it('clamps excessive startup counts instead of losing the authenticated starting state', async () => {
  vi.useFakeTimers()
  const state: Record<string, any> = {}
  const c = engineConnection({ location: { protocol: 'http:', host: '127.0.0.1:12345' }, debug() {},
    connect: (() => Effect.fail(new Error('held'))) as never,
    fetch: (async () => new Response(JSON.stringify({ _tag: 'ServiceStarting', recovered: 4, total: 2 }), { status: 503 })) as typeof fetch })
  try {
    c.open(patch => Object.assign(state, patch)); await vi.advanceTimersByTimeAsync(1000)
    expect(state.session.status).toBe('in')
    expect(state.conn.starting).toMatchObject({ recovered: 2, total: 2 })
  } finally { c.close(); vi.useRealTimers() }
})

it('keeps unknown accounts selectable in the shared subscription table and assignment proposal', async () => {
  const { SubscriptionTable } = await import('../../../design/mock/js/dialogs.js' as string)
  const { html, render } = await import('../../../design/mock/js/lib.js' as string)
  const { proposeAssignment } = await import('../../../design/mock/js/data.js' as string)
  const account = { id: 'unknown', harness: 'codex', name: 'Unknown', machine: 'local', enabled: true, signedIn: false, signInKnown: false }
  const s = { subscriptions: [account], machines: [{ id: 'local', name: 'Local' }] }
  const element = document.createElement('div')
  try {
    render(html`<${SubscriptionTable} s=${s} machine="local" />`, element)
    expect(element.querySelector<HTMLInputElement>('input')?.disabled).toBe(false)
    expect(element.textContent).toContain('Not reported')
    expect(proposeAssignment(s)).toEqual(proposeAssignment({ ...s, subscriptions: [{ ...account, signedIn: true, signInKnown: true }] }))
  } finally { render(null, element) }
})
