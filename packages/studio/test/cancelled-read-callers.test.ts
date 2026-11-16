import { afterEach, expect, it, vi } from 'vitest'
import { fixture, flush } from './cancelled-read-fixture.ts'

let f: Awaited<ReturnType<typeof fixture>>
afterEach(() => { f?.close(); vi.useRealTimers() })
const settled = async (work: Promise<unknown>) => {
  let done = false
  void work.finally(() => { done = true })
  await f.cancel()
  expect(done).toBe(true)
}
const boardCases: [string, string, (f: any) => Promise<unknown>][] = [
  ['createProject', 'create-project', f => f.board.createProject({ goal: 'test' })],
  ['loadTask', 'read-task:t', f => f.board.loadTask({ id: 'p' }, { id: 't', worktreeId: 'w' })],
  ['loadAgents', 'read-agents:a', f => f.board.loadAgents(['a'])],
  ['configureLead', 'configure:a', f => f.board.configureLead('a', {})],
  ['loadSetup', 'read-setup', f => f.board.loadSetup()],
  ['advanceSetup', 'setup', f => f.board.advanceSetup(0)],
  ['saveSetupSettings', 'setup-settings', f => f.board.saveSetupSettings({})],
  ['loadMessages', 'read-board:p', f => f.board.loadMessages('p', 'ch')],
  ['sendPost', 'post:ch', f => f.board.sendPost({ channel: 'ch' })],
  ['setClosed', 'close-post:post', f => f.board.setClosed('post', true)],
]
for (const [name, key, start] of boardCases) it(`board ${name} releases its latch after a cancelled waiting read`, async () => {
  f = await fixture(); const work = start(f); await flush()
  expect(f.board.busy(key)).toBe(true)
  await settled(work)
  expect(f.board.busy(key)).toBe(false); expect(f.board.messageReads.size).toBe(0); expect(f.toasts).toEqual(name === 'configureLead' ? ['Configuration requested'] : name === 'setClosed' ? ['Post withdrawn'] : [])
})
const chatCases: [string, string, (f: any) => Promise<unknown>][] = [
  ['loadRequest', 'request:q', f => f.chat.loadRequest('q')],
  ['setAgentConfig', 'agent:a', f => f.chat.setAgentConfig('a', { model: 'model' })],
  ['answerRequest', 'request:q', f => f.chat.answerRequest('q', '', 'answer')],
  ['loadLarge', 'large:1', f => f.chat.loadLarge({ sequence: 1 })],
  ['declinePhoneOffer', 'phone-offer', f => f.chat.declinePhoneOffer()],
]
for (const [name, key, start] of chatCases) it(`chat ${name} releases its latch after a cancelled waiting read`, async () => {
  f = await fixture(); const work = start(f); await flush(); expect(f.chat.busy(key)).toBe(true)
  await settled(work); expect(f.chat.busy(key)).toBe(false); expect(f.toasts).toEqual([])
})
it('B: loadingRequests releases and an owner answer completes after cancellation', async () => {
  f = await fixture(); const work = f.chat.readRequests(); expect(f.chat.status().loadingRequests).not.toBeNull()
  await settled(work); expect(f.chat.status().loadingRequests).toBeNull()
  const answer = f.chat.answerRequest('q', '', 'answer'); await flush(); await settled(answer)
  expect(f.chat.busy('request:q')).toBe(false); expect(f.calls.some(c => c.tag === 'answer')).toBe(true)
})
it('B: accountRead releases and admits a fresh account read', async () => {
  f = await fixture(); await settled(f.chat.readAccountNames()); expect(f.chat.status().accountRead).toBeNull()
  await settled(f.chat.readAccountNames()); expect(f.calls.filter(c => c.tag === 'accounts')).toHaveLength(2)
})
it('B: a thread releases its busy key on connection close and reopens', async () => {
  f = await fixture(); const stop = f.chat.watchThread('a'); expect(f.chat.busy('thread:a')).toBe(true)
  stop(); await f.cancel(); expect(f.chat.busy('thread:a')).toBe(false)
  const stopAgain = f.chat.watchThread('a'); await f.cancel(); stopAgain()
  expect(f.calls.filter(c => c.tag === 'client.thread')).toHaveLength(2); expect(f.toasts).toEqual([])
})
it('watchRoster polling belongs to unwatch, independently of read cancellation', async () => {
  vi.useFakeTimers(); f = await fixture(); const baseline = vi.getTimerCount(); const stop = f.chat.watchRoster('p'); expect(vi.getTimerCount()).toBe(baseline + 1); await f.cancel(); expect(vi.getTimerCount()).toBe(baseline + 1); stop(); expect(vi.getTimerCount()).toBe(baseline)
  const n = f.calls.length; await vi.advanceTimersByTimeAsync(10000); expect(f.calls).toHaveLength(n); expect(f.toasts).toEqual([])
})
for (const [name, key, start] of [
  ['loadMocks', 'read-mocks:p', (f: any) => f.mocks.loadMocks('p')],
  ['approveMocks', 'approve-mocks:p', (f: any) => { f.mocks.approveMocks({ id: 'p' }); return f.dialogs[0].onConfirm() }],
  ['loadDesign', 'read-design:repo', (f: any) => f.mocks.loadDesign('repo')],
  ['loadMachines', 'read-machines', (f: any) => f.mocks.loadMachines()],
  ['loadLead', 'read-project-lead:a', (f: any) => f.project.loadLead('a')],
  ['configureLead', 'configure:a', (f: any) => f.project.configureLead('a', {})],
  ['checkWorkflows', 'settings', (f: any) => f.service.checkWorkflows()],
  ['loadVersion', 'service-version', (f: any) => f.service.loadVersion()],
] as const) it(`${name} releases ${key} after a cancelled waiting read`, async () => {
  f = await fixture(); const work = start(f); await flush(); expect(f.board.busy(key)).toBe(true)
  await settled(work); expect(Object.values(f.state.ui.boardBusy)).not.toContain(true); expect(f.toasts).toEqual(name === 'configureLead' ? ['Lead configuration requested'] : [])
})
const serviceCases: [string, string, (f: any) => Promise<unknown>][] = [
  ['checkHarnesses', 'read-service', f => f.git.checkHarnesses()],
  ['installHarness', 'read-service', f => f.git.installHarness('h')],
  ['accountAction', 'subscriptions', f => f.git.accountAction('account', 'set')],
  ['addAccount', 'subscriptions', f => f.git.addAccount('h', 'name')],
  ['harnessSetting', 'subscriptions', f => f.git.harnessSetting('h', {})],
  ['makeDefault', 'subscriptions', f => f.git.makeDefault('account')],
  ['accountModel', 'subscriptions', f => f.git.accountModel('h', 'model')],
  ['markAvailable', 'subscriptions', f => f.git.markAvailable('account')],
  ['saveSettings', 'settings', f => f.git.saveSettings({ terminalEnabled: true })],
  ['saveCarrier', 'notifications', f => f.git.saveCarrier({})],
  ['clearToken', 'notifications', f => f.git.clearToken()],
  ['notifyKind', 'notifications', f => f.git.notifyKind('failure', true)],
  ['saveEmail', 'notifications', f => f.git.saveEmail({ host: 'host', port: 465, username: '', from: 'a', to: 'b', secret: 'secret' })],
  ['removeEmail', 'notifications', f => f.git.removeEmail()],
  ['carrierEnabled', 'notifications', f => f.git.carrierEnabled('email', true)],
  ['clearEmailSecret', 'notifications', f => f.git.clearEmailSecret()],
  ['saveQuiet', 'notifications', f => f.git.saveQuiet({})],
  ['retryAlert', 'notifications', f => f.git.retryAlert({ id: 'alert' })],
  ['dismissAlerts', 'notifications', f => f.git.dismissAlerts()],
  ['dismissOffer', 'notifications', f => f.git.dismissOffer()],
]
for (const [name, key, start] of serviceCases) it(`service ${name} releases ${key} after a cancelled waiting read`, async () => {
  f = await fixture(); const work = start(f)
  if (name === 'retryAlert') expect(f.git.status().retryPending).toMatchObject({ alertId: 'alert' })
  await flush(); expect(f.git.busy(key)).toBe(true)
  if (name === 'retryAlert') expect(f.git.status().retryPending).toBeUndefined()
  await settled(work); expect(f.git.busy(key)).toBe(false); expect(f.git.status().settingsQueue.running).toBe(false); expect(f.toasts).toEqual([])
})
it('A: Settings cleanup stops retries; connection close releases read-service, then Notifications loads', async () => {
  vi.useFakeTimers(); f = await fixture(); const timer = vi.spyOn(globalThis, 'setTimeout'); const stop = f.git.openScreen('settings')
  void f.git.refreshScreen(); expect(f.git.busy('read-service')).toBe(true)
  stop(); await f.cancel(); expect(f.git.busy('read-service')).toBe(false)
  const retries = () => timer.mock.calls.filter(call => call[1] === 200).length
  const count = retries(); await vi.advanceTimersByTimeAsync(1000); expect(retries()).toBe(count)
  f.online(); const next = f.git.openScreen('notifications'); await flush()
  expect(f.calls.some(c => c.tag === 'client.alerts.list')).toBe(true); expect(f.git.busy('read-service')).toBe(false); next()
})
for (const name of ['folder', 'search', 'open', 'save', 'commit', 'refresh']) it(`files ${name} releases loading state after a cancelled waiting read`, async () => {
  f = await fixture(); const m = f.model
  if (name === 'save' || name === 'commit') { f.online(); await m.open('file'); if (name === 'save') m.edit('new'); f.waiting() }
  const work = name === 'folder' ? m.folder('') : name === 'search' ? m.search('query') : name === 'open' ? m.open('file') : name === 'save' ? m.save() : name === 'commit' ? m.commit('message') : m.refresh()
  await flush()
  if (name === 'folder') expect(m.folders[''].loading).toBe(true)
  else if (name === 'search') expect(m.searching).toBe(true)
  else if (name === 'open') expect(m.opened.loading).toBe(true)
  else if (name === 'save') expect(m.saving).toBe(true)
  else if (name === 'commit') expect(m.committing).toBe(true)
  else { expect(m.refreshState().refreshing).toBe(work); expect(m.refreshState().requested).toBe(false) }
  await settled(work)
  if (name === 'refresh') expect(m.refreshState().refreshing).toBeNull()
  expect(m.searching || m.saving || m.committing || m.opened?.loading || Object.values(m.folders).some((s: any) => s.loading)).toBeFalsy()
  f.online(); await m.refresh(); expect(f.toasts).toEqual([]); expect(m.notice).not.toContain('cancelled')
})

it('files refresh coalesces requested work, then close clears its throttle timer and wakes it', async () => {
  vi.useFakeTimers(); f = await fixture(); const m = f.model, baseline = vi.getTimerCount()
  const work = m.refresh(); await flush()
  expect(m.refreshState().refreshing).toBe(work); expect(m.refreshState().requested).toBe(false)
  expect(m.refresh()).toBe(work); expect(m.refreshState().requested).toBe(true)
  await f.cancel()
  expect(m.refreshState().timer).toBeDefined(); expect(m.refreshState().requested).toBe(false)
  expect(vi.getTimerCount()).toBe(baseline + 1)
  m.close(); await work
  expect(vi.getTimerCount()).toBe(baseline); expect(m.refreshState().refreshing).toBeNull()
})
