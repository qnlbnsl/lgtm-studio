import { readFileSync } from 'node:fs'
import * as Effect from 'effect/Effect'
import { engineConnection } from '../src/connection.ts'
import { isReadRequest } from '../src/readRequests.ts'
import { gitActionsSource } from '../scripts/gitActions.ts'
import * as policy from '../src/gitRefresh.ts'
import * as files from '../src/files.ts'

export const flush = async () => { for (let i = 0; i < 80; i++) await Promise.resolve() }
export async function fixture() {
  const { errorText } = await import('../../../design/mock/js/live/reads.js' as string)
  const waitingConnection = () => engineConnection({ location: { protocol: 'http:', host: '127.0.0.1:12345' }, debug() {},
    fetch: (async () => new Response(null, { status: 204 })) as typeof fetch, connect: (() => Effect.never) as never })
  let pending = waitingConnection(); pending.open(() => {})
  const source = readFileSync('design/mock/js/engine.js', 'utf8')
  const boundary = Function(source.replaceAll('export ', '') + '\nreturn { request, setConnection };')()
  let waiting = true
  const calls: { tag: string; payload: any }[] = []
  const reply = (tag: string, p: any): any => {
    if (tag === 'accounts') return { accounts: [] }
    if (tag === 'harnesses') return { harnesses: [] }
    if (tag === 'client.snapshot') return { accounts: [] }
    if (tag === 'client.harness.status') return { harnesses: [] }
    if (tag === 'project.settings' || tag === 'roles.settings.get') return { settings: {} }
    if (tag === 'roles.role-file.get') return { file: null }
    if (tag === 'project.git.state') return { result: { mergeHead: null } }
    if (tag === 'project.review.status') return { result: { review: null } }
    if (tag === 'project.review.approve') return { result: { ...state.worktrees[0].review, outcome: 'merged' } }
    if (tag === 'project.files.list') return { result: { folder: p.folder || '', items: [], truncated: false } }
    if (tag === 'project.files.read') return { result: { kind: 'text', path: 'file', text: 'old', version: 'v1', editable: true } }
    if (tag === 'project.files.write') return { result: { version: 'v2' } }
    if (tag === 'project.commit') return { result: { head: 'head' } }
    if (tag === 'client.workflows.save' || tag === 'client.workflows.clone') return { workflow: workflowRecord }
    return { result: { items: [], truncated: false }, items: [], accepted: true }
  }
  const connection: any = { request(tag: string, payload: any, options: any) {
    calls.push({ tag, payload })
    return waiting && isReadRequest(tag, payload) ? pending.request(tag as never, payload, options) : Promise.resolve(reply(tag, payload))
  }, board: { projectPayload: () => ({}), mapTaskDetails: () => ({}), postPayload: () => ({}) }, mocks: {} }
  boundary.setConnection(connection)
  const engine = { ...boundary, connection: () => connection }
  const review = { id: 'review', project: 'p', head: 'head', baseHead: 'base', evidence: { tests: { ok: true }, review: { ok: true } } }
  const state: any = { session: { status: 'in' }, conn: { status: 'live' }, ui: {}, settings: { email: {} }, toasts: [], drafts: {},
    agents: [{ id: 'a', project: 'p' }], tasks: [], teams: [], projects: [{ id: 'p', repo: 'repo', lead: 'a' }], subscriptions: [], requests: [{ id: 'q', kind: 'question', agent: 'a', sourceSequence: 1, options: [] }],
    inspecting: { p: 'topic' }, gitUi: { w: { tab: 'changes', file: 'file' } }, worktrees: [{ id: 'w', project: 'p', repo: 'repo', machine: 'local', kind: 'team', parentId: 'base', branch: 'topic', head: 'head', files: [{ id: 'file', path: 'file' }], review,
      merge: { files: [{ id: 'file', selection: {}, parts: [] }] } }, { id: 'base', repo: 'repo', machine: 'local', kind: 'base', branch: 'main', head: 'base' }],
    channels: [{ id: 'ch', project: 'p' }], posts: [{ id: 'post', channel: 'ch' }], notifications: [], mocks: { p: { revisionId: 'revision' } }, threads: {}, pending: {} }
  const toasts: string[] = [], dialogs: any[] = []
  const toast = (e: any, _tone?: string, fallback?: string) => { const text = errorText(e, fallback); if (text) toasts.push(text) }
  const env: any = { store: { state }, live: { on: true }, update: (fn: any) => fn(state), engineWrite: (p: any) => Object.assign(state, p), notify() {}, refuse() {}, go() {}, engine, errorText,
    toast, confirm: (d: any) => dialogs.push(d), closeDialog() {}, openDialog() {}, nameFromGoal: () => 'project', HARNESSES: { h: { installed: true, settings: { id: 'h' } } },
    registerDialog() {}, persist: (_k: string, fallback: any) => fallback, addEventListener: window.addEventListener.bind(window), removeEventListener: window.removeEventListener.bind(window) }
  env.act = { toast, confirm: env.confirm, closeDialog: env.closeDialog, openDialog: env.openDialog }
  const load = (file: string, exports: string, extra = '') => {
    let source = gitActionsSource('js/' + file, readFileSync('design/mock/js/' + file, 'utf8')).replace(/^import .*;$/gm, '').replace(/^export \{[^\n]*$/gm, '').replaceAll('export ', '')
    if (file === 'live/files.js') source = source.replace('  return m;', '  return Object.assign(m, { refreshState: () => ({ refreshing, requested, timer, wake }) });');
    const names = Object.keys(env).filter(name => !new RegExp('^(?:const|let|function|async function) ' + name + '\\b', 'm').test(source))
    return Function(...names, source + '\n' + extra + '\nreturn {' + exports + '};')(...names.map(name => env[name]))
  }
  const board = load('live/board.js', 'run, busy, createProject, loadTask, loadAgents, configureLead, loadSetup, advanceSetup, saveSetupSettings, loadMessages, sendPost, setClosed, messageReads')
  env.run = board.run; env.busy = board.busy
  const chat = load('live/chat.js', 'setMapping, busy, loadRequest, readRequests, readAccountNames, watchThread, watchRoster, setAgentConfig, answerRequest, loadLarge, declinePhoneOffer, reset, status: () => ({ loadingRequests, accountRead })')
  chat.setMapping({ mapSubscriptionNames: () => [], mapAgentDetails: () => ({}), mapThread: () => ({ items: [] }), mapRosterDetails: () => ({}), mapFailedStart: () => null, mapHomeRequest: () => null, mapPhoneOffer: () => ({}) })
  const git = load('live/git.js', 'installMappings, busy, openScreen, refreshScreen, checkHarnesses, accountAction, addAccount, harnessSetting, makeDefault, accountModel, markAvailable, saveSettings, saveCarrier, clearToken, notifyKind, saveEmail, removeEmail, carrierEnabled, clearEmailSecret, saveQuiet, retryAlert, dismissAlerts, dismissOffer, installHarness, refreshGit, gitDetail, selectGit, stage, discard, commit, syncGit, integrate, answerReview, resolve, continueMerge, readGitRecord, loadGitDetail, status: () => ({ gitController, refreshTimer, settingsQueue, retryPending })', `service = { project: { lead: { harness: 'h' } }, harnesses: [{ id: 'h' }], roles: {}, web: {} }; accounts = [{ id: 'account', harness: 'h' }];`)
  git.installMappings({ ...policy, mapGitState: (r: any) => r, mapGitChanges: (r: any) => r, mapGitCommit: (r: any) => r, mapGitDiff: (r: any) => r, mapMainReview: (r: any) => r, mapAlertSettings: () => ({}), mapAlerts: () => [], mapPhoneOffer: () => ({}),
    mapHarnesses: () => ({}), mapInstalledHarnesses: () => ({}), mapServiceSettings: () => ({}), mapAccounts: () => [] })
  const mocks = load('live/mocks.js', 'loadMocks, approveMocks, loadDesign, loadMachines')
  const project = load('live/project-settings.js', 'loadLead, configureLead, installMappings'); project.installMappings({ mapLeadSettings: () => ({}) })
  const service = load('live/service-settings.js', 'checkWorkflows, loadVersion')
  const fileModule = load('live/files.js', 'createFilesModel, setMapping'); fileModule.setMapping(files)
  const model = fileModule.createFilesModel({ worktreeId: 'w', projectId: 'p', kind: 'project', machine: 'local', online: true }, () => {})
  const workflowRecord = { scope: 'user', id: 'flow', definition: { phases: [{ id: 'phase', blocks: [] }] } }
  return { reply, connection, state, calls, toasts, dialogs, board, chat, git, mocks, project, service, model, env, load, engine, errorText, workflowRecord,
    online: () => { waiting = false }, waiting: () => { waiting = true }, cancel: async () => { pending.close(); pending = waitingConnection(); pending.open(() => {}); await flush() }, close: () => { model.close(); chat.reset(); pending.close() } }
}
