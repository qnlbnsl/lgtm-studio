// Live actions for Git and the service pages. Only replies and fresh reads write records.
import { store, live, engineWrite, update, notify, refuse, go } from '../lib.js';
import * as engine from '../engine.js';
import * as act from '../actions.js';
import { HARNESSES } from '../data.js';
let mapping;
export function installMappings(value) { mapping = value; }
const S = () => store.state;
export const busy = (key) => !!S()?.ui?.gitPending?.[key];
const pending = (key, value) => { S().ui.gitPending ||= {}; S().ui.gitPending[key] = value; notify(); };
export async function run(key, work) {
  if (busy(key)) return false;
  pending(key, true);
  if (key === 'git') holdGit();
  const generation = sessionGeneration, visit = key.startsWith('git') ? gitController : undefined;
  const current = () => generation === sessionGeneration;
  try { await work(); return true; }
  catch (e) { if (current() && (!visit || gitController === visit && visit.enabled) && S().session.status === 'in') act.toast(e.message || 'The service did not complete the request.', 'error'); return false; }
  finally { if (current()) { pending(key, false); if (key === 'git') scheduleGit(); } }
}
const mutatesGit = (tag) => /^project\.(stage|unstage|discard$|commit$|pull$|push$|fetch$|integrate$|update$|merge\.abort$|conflict\.(choose|resolve|reopen)$|review\.(prepare|approve|decline)$)/.test(tag);
async function request(tag, payload = {}, options = {}) {
  const c = gitController;
  if (!c || !tag.startsWith('project.')) return Object.keys(options).length ? engine.request(tag, payload, options) : engine.request(tag, payload);
  const mutation = mutatesGit(tag);
  if (mutation) {
    const w = wt(payload.worktreeId), headChange = /\.(commit|pull|push|fetch|integrate|update|abort|approve)$/.test(tag);
    const ids = tag === 'project.fetch' ? [...c.records.keys()] : w ? headChange ? affectedGit(w, wt(payload.destinationId) || (tag === 'project.review.approve' ? destination(w) : undefined)) : [w.id] : [];
    if (tag === 'project.pull' && w) ids.push(...S().worktrees.filter((x) => x.repo === w.repo && !x.parentId).map((x) => x.id));
    for (const id of ids) { const r = c.records.get(id); if (r) { r.epoch++; mapping.dirtyGitRecord(r, 'summary', Date.now()); if (headChange || tag.includes('review.')) mapping.dirtyGitRecord(r, 'review', Date.now()); } }
  }
  const call = { abort: new AbortController(), id: payload.worktreeId }; c.calls.add(call);
  const signal = options.signal ? AbortSignal.any([options.signal, call.abort.signal]) : call.abort.signal;
  try { return await engine.request(tag, payload, { ...options, signal, onRequestId: (id) => { if (mutation) c.ownIds.add(id); options.onRequestId?.(id); } }); }
  finally { c.calls.delete(call); }
}
const draftValues = () => Object.fromEntries([...settingsDrafts].map(([key, draft]) => [key, draft.value]));
const settingsPatch = (patch) => engineWrite({ settings: { ...S().settings, ...patch, ...draftValues() } });
let sessionGeneration = 0;
let settingsQueue = { items: [], running: false };
const settingsDrafts = new Map();
let service;
let accounts;
let statuses;
let subscriptionsLoaded = false;
let screen;
let revision = 0;
let serviceRead = 0;
let refreshTimer;
let retryPending;
// A connection snapshot can replace the skeletons. Keep already-read detail while refreshing.
export function observe(patch) {
  if (patch.session && patch.session.status !== 'in') {
    sessionGeneration++;
    for (const item of settingsQueue.items) for (const done of item.done) done(false);
    settingsQueue = { items: [], running: false }; settingsDrafts.clear();
    S().ui.gitPending = {};
    stopGit();
    service = accounts = statuses = retryPending = undefined; subscriptionsLoaded = false; S().ui.serviceReady = false; revision++; clearTimeout(refreshTimer);
    Object.assign(patch, { notifications: [], settings: { ntfyServer: '', ntfyTopic: '', ntfyToken: '', tokenSet: false, ntfyEnabled: false, email: {}, notifyOn: {}, quiet: {} } });
  }
  if (patch.settings && S().session.status === 'in') patch.settings = { ...patch.settings, ...draftValues() };
  if (patch.worktrees) patch.worktrees = patch.worktrees.map((w) => ({ ...w, ...(S().worktrees?.find((x) => x.id === w.id && x.folder === w.folder && x.branch === w.branch) || {}), ...Object.fromEntries(Object.entries(w).filter(([k]) => !['files', 'commits', 'ahead', 'behind', 'merge'].includes(k))) }));
  if (patch.subscriptions && subscriptionsLoaded) patch.subscriptions = S().subscriptions;
  if (screen?.name === 'git') {
    if (patch.worktrees) reconcileGit(patch.worktrees);
    if (patch.git && gitController) {
      if (patch.git.requestId && gitController.ownIds.has(patch.git.requestId)) { return; }
      for (const [id, r] of gitController.records) if (!patch.git.worktreeIds.length || patch.git.worktreeIds.includes(id)) mapping.dirtyGitRecord(r, patch.git.group, Date.now());
      scheduleGit();
    }
    if (patch.conn?.status === 'live' && S().conn?.status !== 'live') dirtyAllGit();
  }
  if (screen && screen.name !== 'git' && (patch.conn?.status === 'live' || patch.projects)) {
    clearTimeout(refreshTimer); refreshTimer = setTimeout(() => refreshScreen(), 100);
  }
}
export function openScreen(name, project) {
  if (!live.on) return () => {};
  stopGit();
  if (name === 'git') engineWrite({ worktrees: S().worktrees.map((w) => ({ ...w, reviewLoaded: false })) });
  screen = { name, project };
  if (name === 'git') startGit(project); else void refreshScreen();
  const visit = screen;
  const refresh = () => { if (screen === visit) { if (name === 'git') dirtyAllGit(); else void refreshScreen(); } };
  addEventListener('focus', refresh);
  const visibility = () => { if (screen !== visit || name !== 'git') return; if (typeof document !== 'undefined' && document.hidden) suspendGit(); else { if (gitController) { gitController.enabled = true; dirtyAllGit(); } } };
  if (typeof document !== 'undefined') document.addEventListener('visibilitychange', visibility);
  return () => { if (screen !== visit) return; stopGit(); screen = undefined; revision++; clearTimeout(refreshTimer); removeEventListener('focus', refresh); if (typeof document !== 'undefined') document.removeEventListener('visibilitychange', visibility); };
}
async function refreshScreen() {
  if (!mapping || !screen || S().session.status !== 'in') return;
  if (screen.name === 'git') return;
  if (busy('settings') || busy('read-service')) { clearTimeout(refreshTimer); refreshTimer = setTimeout(refreshScreen, 200); return; }
  return run('read-service', () => loadService(screen?.name));
}
async function loadService(name, refreshHarnesses = false) {
  const version = revision, readVersion = ++serviceRead;
  const currentRead = () => version === revision && readVersion === serviceRead && S().session.status === 'in';
  if (name === 'notifications' || name === 'phone-offer') {
    const settings = await request('client.alerts.settings');
    if (name === 'phone-offer') {
      if (currentRead()) {
        settingsPatch(mapping.mapAlertSettings(settings));
        engineWrite({ phoneOffer: mapping.mapPhoneOffer(settings) });
      }
      return;
    }
    const items = []; let after = 0;
    for (;;) { const page = await request('client.alerts.list', { after, limit: 200 }); items.push(...page.items); if (page.next === undefined || page.next <= after) break; after = page.next; }
    const requests = []; after = 0;
    for (;;) { const page = await request('client.requests', { after, limit: 200 }); requests.push(...page.items); if (page.next === undefined || page.next <= after) break; after = page.next; }
    if (!currentRead()) return;
    settingsPatch(mapping.mapAlertSettings(settings));
    engineWrite({ notifications: mapping.mapAlerts(items, S().agents, requests), phoneOffer: mapping.mapPhoneOffer(settings) });
    return;
  }
  const [accountReply, harnessReply, project, roles, web, roleFile, snapshot] = await Promise.all([
    request('accounts', { operation: 'list' }), request('harnesses', { operation: 'list' }), request('project.settings'),
    request('roles.settings.get'), request('client.web.settings'), request('roles.role-file.get'), request('client.snapshot')
  ]);
  if (!currentRead()) return;
  const installationReply = await request('client.harness.status', { refresh: refreshHarnesses });
  const recipes = await Promise.all(installationReply.harnesses.map(async (row) => {
    try { return (await request('client.harness.recipe', { harnessId: row.harnessId })).recipe; } catch { return null; }
  }));
  if (!currentRead()) return;
  service = { harnesses: harnessReply.harnesses, project: project.settings, roles: roles.settings, web, roleFile: roleFile.file };
  accounts = accountReply.accounts; statuses = snapshot.accounts;
  Object.keys(HARNESSES).forEach((k) => { delete HARNESSES[k]; });
  Object.assign(HARNESSES, mapping.mapHarnesses(service.harnesses));
  Object.assign(HARNESSES, mapping.mapInstalledHarnesses(HARNESSES, installationReply.harnesses, recipes));
  settingsPatch(mapping.mapServiceSettings(service.project, service.harnesses, service.roles, service.web, service.roleFile));
  engineWrite({ subscriptions: mapping.mapAccounts(accounts, statuses, service.harnesses, service.project) });
  subscriptionsLoaded = true; S().ui.serviceReady = true; notify();
  return true;
}
export const refreshAccounts = () => loadService('subscriptions');
export const accountAction = (id, operation, patch = {}) => run('subscriptions', async () => {
  const account = accounts?.find((a) => a.id === id);
  if (!account) throw new Error('Refresh subscriptions before changing this account.');
  await request('accounts', operation === 'set' ? { operation, account: { ...account, ...patch } } : { operation, accountId: id });
  await loadService('subscriptions');
});
export const addAccount = (harness, name, home) => run('subscriptions', async () => {
  const h = HARNESSES[harness];
  if (!h?.installed) throw new Error('Install the harness before adding an account.');
  if (!service?.harnesses.some((row) => row.id === harness) && h.settings) await request('harnesses', { operation: 'set', harness: h.settings });
  await request('accounts', { operation: 'add', account: { id: 'account-' + crypto.randomUUID(), harness, displayName: name, enabled: true, ...(home ? { home } : {}) } });
  await loadService('subscriptions'); act.closeDialog();
});
export const harnessRecipe = async (harnessId) => (await request('client.harness.recipe', { harnessId })).recipe;
export const installHarness = async (harnessId) => {
  const result = await request('client.harness.install', { harnessId });
  await run('read-service', () => loadService('harnesses'));
  return result;
};
export const checkHarnesses = () => run('read-service', () => loadService('harnesses', true));
export const checkAccount = async (accountId) => {
  const result = await request('client.harness.account.check', { accountId });
  engineWrite({ subscriptions: S().subscriptions.map((a) => a.id === accountId ? { ...a, signedIn: result.state === 'local-credential' } : a) });
  return result;
};
export const harnessSetting = (harnessId, patch) => run('subscriptions', async () => {
  const previous = service?.harnesses.find((h) => h.id === harnessId) || HARNESSES[harnessId]?.settings;
  if (!previous) { refuse(); return; }
  const harness = { ...previous, ...patch };
  for (const field of ['defaultModel', 'defaultEffort']) if (harness[field] === '') delete harness[field];
  await request('harnesses', { operation: 'set', harness });
  await loadService('harnesses');
});
export const makeDefault = (id) => run('subscriptions', async () => {
  const account = accounts?.find((a) => a.id === id), h = service?.harnesses.find((h) => h.id === account?.harness);
  if (!service?.project || !h) throw new Error('Configure project lead defaults first.');
  const lead = service.project.lead?.harness === h.id ? service.project.lead : { harness: h.id, permission: h.defaultPermission,
    ...(h.defaultModel ? { model: h.defaultModel } : {}), ...(h.defaultEffort ? { effort: h.defaultEffort } : {}) };
  await request('project.settings.set', { settings: { ...service.project, lead: { ...lead, account: { _tag: 'named', accountId: id } } } });
  await loadService('subscriptions');
});
export const accountModel = (harness, model) => run('subscriptions', async () => {
  const h = service?.harnesses.find((x) => x.id === harness);
  if (!h) throw new Error('Refresh harness settings first.');
  await request('harnesses', { operation: 'set', harness: { ...h, defaultModel: model } }); await loadService('subscriptions');
});
export const markAvailable = (id) => run('subscriptions', async () => { await request('accounts mark-available', { accountId: id }); await loadService('subscriptions'); });
// Retain edits immediately. Only unsent values of the same field coalesce;
// each write waits for the preceding write and its complete readback.
export const saveSettings = (patch) => {
  const queue = settingsQueue, key = Object.keys(patch)[0], edit = {};
  for (const [field, value] of Object.entries(patch)) settingsDrafts.set(field, { value, edit });
  settingsPatch(patch);
  const previous = queue.items.find(item => item.key === key);
  if (previous) queue.items.splice(queue.items.indexOf(previous), 1);
  return new Promise(resolve => {
    queue.items.push({ key, patch: { ...patch }, edit, done: [...(previous?.done || []), resolve] });
    if (!queue.running) void drainSettings(queue);
  });
};
async function drainSettings(queue) {
  queue.running = true; pending('settings', true);
  const connection = engine.connection();
  const current = () => queue === settingsQueue && connection === engine.connection() && S().session.status === 'in';
  try {
    while (queue.items.length && current()) {
      const item = queue.items.shift(); let saved = false;
      try { revision++; await writeSettings(item.patch, current); saved = current(); }
      catch (e) { if (current()) act.toast(e.message || 'The service did not complete the request.', 'error'); }
      if (current()) {
        for (const field of Object.keys(item.patch)) if (settingsDrafts.get(field)?.edit === item.edit) settingsDrafts.delete(field);
        if (service) settingsPatch(mapping.mapServiceSettings(service.project, service.harnesses, service.roles, service.web, service.roleFile));
      }
      for (const done of item.done) done(saved);
    }
  } finally {
    for (const item of queue.items.splice(0)) for (const done of item.done) done(false);
    queue.running = false;
    if (queue === settingsQueue) pending('settings', false);
  }
}
async function writeSettings(patch, current) {
  if (!service) throw new Error('Wait for the service settings to load.');
  const key = Object.keys(patch)[0];
  const limits = { stallWorker: 'stallMinutes', capacityBackoff: 'capacityBackoffMinutes', capacityRetries: 'capacityRetries', crashRetries: 'crashRetries', teamIdle: 'teamIdleMinutes' };
  if (key in limits || ['stallWatch', 'standingRules'].includes(key)) {
    const settings = key in limits ? { ...service.roles, limits: { ...service.roles.limits, [limits[key]]: patch[key] } } : { ...service.roles, [key]: patch[key] };
    await request('roles.settings.set', { settings });
  } else if (['defaultHarness', 'defaultModel', 'defaultEffort', 'defaultPermission'].includes(key)) {
    if (!service.project) throw new Error('Configure the project workspace and shell first.');
    const h = service.harnesses.find((h) => h.id === (patch.defaultHarness || service.project.lead?.harness));
    if (!h) throw new Error('Choose a harness first.');
    const permission = { 'Read only': 'read-only', 'Ask before every change': 'ask', 'Edit files without asking': 'edit', 'Full access': 'full' };
    const lead = key === 'defaultHarness' || !service.project.lead ? { harness: h.id, account: { _tag: 'any' }, permission: h.defaultPermission,
      ...(h.defaultModel ? { model: h.defaultModel } : {}), ...(h.defaultEffort ? { effort: h.defaultEffort } : {}) } : { ...service.project.lead };
    if (key === 'defaultModel') lead.model = patch[key];
    if (key === 'defaultEffort') lead.effort = patch[key];
    if (key === 'defaultPermission') lead.permission = permission[patch[key]];
    await request('project.settings.set', { settings: { ...service.project, lead } });
  } else if (key === 'terminalEnabled') {
    const { requestId, serverState, ...previous } = service.web;
    await request('client.web.settings.set', { settings: { ...previous, terminalEnabled: patch.terminalEnabled } });
  } else if (key === 'webEnabled') {
    const { requestId, serverState, ...previous } = service.web;
    const settings = { ...previous, enabled: patch.webEnabled, port: patch.port, heartbeatSeconds: patch.checkIn, proxyMode: patch.proxyMode };
    if (patch.publicAddress) settings.publicAddress = patch.publicAddress; else delete settings.publicAddress;
    await request('client.web.settings.set', { settings });
  } else { refuse(); return; }
  // Navigation can invalidate a read already in flight. Finish a current
  // readback before the next queued write spreads these service records.
  while (current() && !await loadService('settings')) {}
}
export const saveCarrier = (form) => run('notifications', async () => {
  const carrier = form.server ? { _tag: 'ntfy', server: form.server, topic: form.topic } : { _tag: 'none' };
  await request('client.alerts.carrier.set', { carrier });
  if (carrier._tag === 'none') await request('client.alerts.carrier.clear-token');
  else if (form.token) await request('client.alerts.carrier.set-token', { origin: new URL(form.server).origin, nonce: crypto.randomUUID(), value: form.token });
  await loadService('notifications');
});
export const clearToken = () => run('notifications', async () => { await request('client.alerts.carrier.clear-token'); await loadService('notifications'); });
export const notifyKind = (kind, push) => run('notifications', async () => {
  await request('client.alerts.settings.set', { kind: kind === 'failed' ? 'failure' : kind, push }); await loadService('notifications');
});
export const sendTest = (name = 'ntfy') => run('notifications', async () => {
  const r = await request('client.alerts.test', { name });
  if (!r.accepted) throw new Error(r.reason === 'token missing' ? 'Set a password or access token first.' : r.reason === 'service missing' ? 'Configure this carrier first.' : 'The carrier delivery failed. Check your settings and try again.');
  act.toast(name === 'email' ? 'Your mail server accepted the test email.' : 'The ntfy server accepted the test notification.');
});
export const saveEmail = (form) => run('notifications', async () => {
  const { secret, enabled, tokenSet, ...fields } = form;
  const carrier = { _tag: 'email', host: fields.host.trim(), port: Number(fields.port), security: fields.security, username: fields.username.trim(), from: fields.from.trim(), to: fields.to.trim() };
  const saved = S().settings.email || {};
  const credentialChanged = ['host', 'port', 'security', 'username'].some((key) => String(saved[key]) !== String(carrier[key]));
  if (saved.tokenSet && credentialChanged && !secret) throw new Error('Changing the mail server or user name will clear the stored password. Enter a password or app key before saving.');
  if (!saved.tokenSet && !secret) throw new Error('Enter a password or app key before saving email.');
  const result = await request('client.alerts.carrier.set', { carrier });
  if (secret) await request('client.alerts.carrier.set-token', { name: 'email', origin: JSON.stringify([carrier.host, carrier.port, carrier.security, carrier.username]), nonce: crypto.randomUUID(), value: secret });
  // Initial setup turns email on after its password is stored. An edited
  // destination restores only a carrier that the owner previously had on.
  if (secret && (!saved.host || saved.enabled && result.enabled === false)) await request('client.alerts.carrier.enabled.set', { name: 'email', enabled: true });
  await loadService('notifications');
});
export const removeEmail = () => run('notifications', async () => { await request('client.alerts.carrier.remove', { name: 'email' }); await loadService('notifications'); });
export const carrierEnabled = (name, enabled) => run('notifications', async () => { await request('client.alerts.carrier.enabled.set', { name, enabled }); await loadService('notifications'); });
export const clearEmailSecret = () => run('notifications', async () => { await request('client.alerts.carrier.clear-token', { name: 'email' }); await loadService('notifications'); });
export const saveQuiet = (quiet) => run('notifications', async () => {
  await request('client.alerts.quiet.set', { quiet: { enabled: quiet.on, start: quiet.from, end: quiet.to, timeZone: quiet.timeZone, failureOverride: quiet.failed } }); await loadService('notifications');
});
export const retryAlert = (n) => run('notifications', async () => {
  if (retryPending && retryPending.alertId !== n.id) throw new Error('Recover the pending notification retry first.');
  retryPending ||= { alertId: n.id, generation: n.generation, attempt: n.attempt, ...(n.carrier ? { name: n.carrier } : {}) };
  let r;
  try { r = await request('client.alerts.retry', retryPending); }
  catch (e) { if (e.refused) retryPending = undefined; throw e; }
  retryPending = undefined;
  await loadService('notifications');
  if (!r.accepted) throw new Error(r.reason);
});
export const dismissAlerts = () => run('notifications', async () => { for (const n of S().notifications.filter((n) => !n.read)) await request('client.alerts.dismiss', { alertId: n.id }); await loadService('notifications'); });
export const dismissOffer = () => run('notifications', async () => { await request('client.alerts.prompt.dismiss'); await loadService('notifications'); });
// Git reads and actions follow below.
const wt = (id) => S().worktrees.find((w) => w.id === id);
const scope = (w) => ({ worktreeId: w.id });
const read = (reply) => { if (!('result' in reply)) throw new Error('The service returned no usable Git record.'); return reply.result; };
const writeWt = (id, patch) => engineWrite({ worktrees: S().worktrees.map((w) => w.id === id ? { ...w, ...patch } : w) });
function local(w) { if (!w || w.machine !== 'local') throw new Error('Git is available only for a workspace on local.'); }
function destination(w) { return S().worktrees.find((x) => x.repo === w.repo && x.id === w.parentId); }
// A changed head affects its checkout and each direct child's parent-relative counts.
// Include both endpoints for operations that can change the destination too.
function affectedGit(w, target) {
  const roots = new Set(target ? [w.id, target.id] : [w.id]);
  return S().worktrees.filter((x) => x.repo === w.repo && (roots.has(x.id) || roots.has(x.parentId))).map((x) => x.id);
}
// All automatic admission lives in this visit's controller and its single timer.
let gitController;
let lastAutomaticStart = null;
const identity = (w) => JSON.stringify([w.repo, w.folder, w.branch, w.machine, w.parentId]);
const gitRows = (project, rows = S().worktrees) => rows.filter((w) => w.machine === 'local' && (w.project === project || w.kind === 'base' && w.repo === S().projects.find((p) => p.id === project)?.repo));
const selectedGit = (project) => gitRows(project).find((w) => w.branch === S().inspecting[project]) || gitRows(project).find((w) => w.kind === 'project') || gitRows(project)[0];
function recordFor(w, c = gitController) {
  if (!c) { const r = mapping.gitRefreshRecord(identity(w), Date.now()); r.reviewDirty = w.reviewLoaded ? 0 : 1; return r; }
  let r = c.records.get(w.id);
  if (!r || r.identity !== identity(w)) {
    r?.summaryAbort?.abort(); r?.detailAbort?.abort();
    for (const call of c.calls) if (call.id === w.id) call.abort.abort();
    r = mapping.gitRefreshRecord(identity(w), Date.now(), (r?.epoch || 0) + 1); c.records.set(w.id, r);
  }
  return r;
}
function reconcileGit(rows = S().worktrees) {
  const c = gitController; if (!c) return;
  const list = gitRows(c.project, rows), ids = new Set(list.map((w) => w.id));
  for (const [id, r] of c.records) if (!ids.has(id)) { r.epoch++; r.summaryAbort?.abort(); r.detailAbort?.abort(); for (const call of c.calls) if (call.id === id) call.abort.abort(); c.records.delete(id); }
  for (const w of list) recordFor(w, c);
  scheduleGit();
}
function holdGit() { if (gitController) clearTimeout(gitController.timer); }
function suspendGit() {
  const c = gitController; if (!c) return;
  c.enabled = false; clearTimeout(c.timer);
  for (const r of c.records.values()) {
    r.epoch++; r.summaryAbort?.abort(); r.detailAbort?.abort();
    if (r.phase === 'reading') { r.phase = 'due'; r.nextAttempt = Date.now(); }
  }
  for (const call of c.calls) call.abort.abort();
  c.reading = null; pending('git-read', false);
}
function stopGit() {
  const c = gitController; if (!c) return;
  suspendGit(); c.ownIds.clear(); gitController = undefined;
}
function startGit(project) {
  if (!mapping) return;
  const c = gitController = { ...mapping.createGitController(lastAutomaticStart), project, calls: new Set(), ownIds: new Set() };
  reconcileGit();
  if (typeof document !== 'undefined' && document.hidden) { suspendGit(); return; }
  // Opening the screen is an owner demand. It must not disable its controls.
  clearTimeout(c.timer); pending('git-read', true); c.reading = 'initial';
  void loadGit(project).catch(() => {}).finally(() => { if (gitController === c) { c.reading = null; pending('git-read', false); scheduleGit(); } });
}
function dirtyAllGit() {
  const c = gitController; if (!c) return;
  for (const r of c.records.values()) mapping.dirtyGitRecord(r, 'summary', Date.now());
  scheduleGit();
}
function scheduleGit() {
  const c = gitController; if (!c) return;
  clearTimeout(c.timer); c.ownerBusy = busy('git'); c.selected = selectedGit(c.project)?.id;
  const next = mapping.nextAutomaticRead(Date.now(), c); if (!next) return;
  c.timer = setTimeout(async () => {
    if (gitController !== c) return;
    c.timer = undefined;
    c.ownerBusy = busy('git');
    const admitted = mapping.nextAutomaticRead(Date.now(), c);
    if (!admitted || admitted.at > Date.now()) { scheduleGit(); return; }
    const r = c.records.get(admitted.id), w = wt(admitted.id);
    if (!r || !w) { reconcileGit(); return; }
    c.lastStart = lastAutomaticStart = Date.now(); c.reading = w.id;
    pending('git-read', true);
    try {
      await readGitRecord(w, c.project, r, c, false);
      if (gitController === c && c.selected === w.id && !busy('git')) await loadGitDetail(w.id, undefined, true);
    } finally { if (gitController === c) { c.reading = null; pending('git-read', false); scheduleGit(); } }
  }, Math.max(0, next.at - Date.now()));
}
async function readGitRecord(w, project, r, c, forced, reviews = false) {
  if (forced) { r.epoch++; r.failures = 0; r.summaryDirty++; if (reviews) r.reviewDirty++; }
  const epoch = r.epoch, summary = r.summaryDirty, reviewCount = r.reviewDirty;
  const valid = () => r.epoch === epoch && S().session.status === 'in' && (!c || gitController === c && c.enabled && c.records.get(w.id) === r);
  r.summaryAbort?.abort(); const abort = r.summaryAbort = new AbortController();
  const ask = async (tag, payload) => { if (!valid()) return; const reply = await request(tag, payload, { signal: abort.signal }); return valid() ? read(reply) : undefined; };
  r.phase = 'reading'; let failure = null;
  try {
    let state;
    if (forced || summary || !r.lastSuccess) {
      state = await ask('project.git.state', { ...scope(w), ...(w.parent ? { target: w.parent } : {}) }); if (!state) return;
      const changes = await ask('project.changes', scope(w)); if (!changes) return;
      const old = wt(w.id), headChanged = old.head !== state.head || old.mergeHead !== state.mergeHead;
      const files = mapping.mapGitChanges(changes.items).map((f) => {
        const cached = !headChanged && old.files.find((x) => x.id === f.id && x.path === f.path && x.staged === f.staged && x.status === f.status);
        return cached ? { ...f, kind: cached.kind, hunks: cached.hunks, add: cached.add, del: cached.del, diffError: cached.diffError, loaded: cached.loaded } : f;
      });
      let resolved = { items: [], truncated: false };
      if (state.mergeHead) try { resolved = await ask('project.conflict.resolved', scope(w)); if (!resolved) return; } catch (e) { failure = e; }
      if (!valid()) return;
      const merge = state.mergeHead || changes.items.some((x) => x.conflict) ? { from: state.mergeHead || '', by: null, startedAt: null,
        files: [...changes.items.filter((x) => x.conflict), ...resolved.items.map((f) => ({ ...f, resolved: true }))].map((x) => {
          const cached = !headChanged && old.merge?.files.find((f) => f.path === x.path && !!f.resolved === !!x.resolved);
          return { ...cached, id: x.path, path: x.path, parts: cached?.parts || [], resolved: !!x.resolved, fingerprint: x.fingerprint || cached?.fingerprint, loaded: !!cached?.loaded };
        }) } : null;
      writeWt(w.id, { ...mapping.mapGitState(state, w.parent), files, merge, ready: !changes.truncated, truncated: changes.truncated || resolved.truncated });
      if (!failure) r.summaryDirty = Math.max(0, r.summaryDirty - summary);
    }
    if (w.kind === 'project' && (reviews || reviewCount || !wt(w.id).reviewLoaded || wt(w.id).review?.status === 'open')) {
      try {
        const status = await ask('project.review.status', { ...scope(w), projectId: project }); if (!status) return;
        writeWt(w.id, { reviewLoaded: true });
        if (status.review) {
          const fresh = mapping.mapMainReview(status.review, undefined, S().projects.find((p) => p.id === project)?.lead || '');
          const old = wt(w.id).review;
          const review = old?.id === fresh.id && old.head === fresh.head && old.baseHead === fresh.baseHead ? { ...old, ...fresh, files: old.files, evidence: { ...old.evidence, tests: fresh.evidence.tests, review: fresh.evidence.review } } : fresh;
          writeWt(w.id, { review, merged: status.review.outcome === 'merged' && status.review.head === (state?.head || wt(w.id).head) ? status.review.decidedAt : null });
        }
      } catch (e) { failure ||= e; }
    }
    if (valid() && !failure) r.reviewDirty = Math.max(0, r.reviewDirty - reviewCount);
  } catch (e) { failure = e; }
  finally {
    if (valid()) {
      mapping.finishGitRecord(r, Date.now(), failure ? failure.message || 'The worktree could not be refreshed.' : null);
      if (failure && (r.lastToast === null || Date.now() - r.lastToast >= 60000)) { r.lastToast = Date.now(); act.toast('This worktree could not be refreshed.', 'error'); }
    }
    if (r.summaryAbort === abort) r.summaryAbort = undefined;
  }
  return failure;
}
async function loadGit(project, ids, reviews = false) {
  if (!mapping) return;
  const c = gitController; let failure;
  for (const w of gitRows(project).filter((w) => !ids || ids.includes(w.id))) {
    if (c && gitController !== c) return;
    const error = await readGitRecord(w, project, recordFor(w, c), c, true, reviews); failure ||= error;
  }
  const selected = selectedGit(project);
  if (selected && (!ids || ids.includes(selected.id)) && (!c || gitController === c)) await loadGitDetail(selected.id);
  if (failure) throw failure;
}
async function loadGitDetail(id, selectedCommit, automatic = false) {
  const w = wt(id); local(w); if (!mapping) return;
  const c = gitController, r = recordFor(w, c), epoch = r.epoch;
  r.detailAbort?.abort(); const abort = r.detailAbort = new AbortController();
  const valid = () => !abort.signal.aborted && r.epoch === epoch && wt(id) && S().session.status === 'in' && (!c || gitController === c && c.enabled && c.records.get(id) === r);
  const ask = async (tag, payload) => { if (!valid()) return; const value = read(await request(tag, payload, { signal: abort.signal })); return valid() ? value : undefined; };
  const ui = S().gitUi[id] || {}, tab = ui.tab || (w.merge ? 'conflicts' : 'changes');
  const failed = () => { if (valid()) act.toast('The file detail could not be loaded.', 'error'); };
  try {
    if (tab === 'history') {
      if (automatic) return;
      const history = await ask('project.history', { ...scope(w), limit: 200 }); if (!history) return;
      const branches = await ask('project.branches', { repoId: w.repo }); if (!branches) return;
      const commits = history.items.map((entry) => ({ ...wt(id).commits?.find((old) => old.hash === entry.hash), ...mapping.mapGitCommit(entry, branches.items) }));
      writeWt(id, { commits, historyTruncated: history.truncated });
      const commit = selectedCommit || ui.commit || commits[0]?.hash; if (!commit) return;
      const existing = commits.find((entry) => entry.hash === commit); if (existing?.loaded) return;
      const inventory = await ask('project.commit.files', { ...scope(w), commit }); if (!inventory) return;
      const files = mapping.mapCommitFiles(inventory.items);
      writeWt(id, { commits: wt(id).commits.map((entry) => entry.hash === commit ? { ...entry, files, loaded: true } : entry) });
      for (const f of files) { try { const diff = await ask('project.diff', { ...scope(w), path: f.path, commit }); if (!diff) return;
        writeWt(id, { commits: wt(id).commits.map((entry) => entry.hash === commit ? { ...entry, files: entry.files.map((x) => x.id === f.id ? { ...x, ...mapping.mapGitDiff(diff), loaded: true } : x) } : entry) });
      } catch { failed(); } }
    } else {
      const conflicts = tab === 'conflicts' && w.merge;
      const files = conflicts ? w.merge.files : w.files;
      const selected = files.find((f) => f.id === ui[conflicts ? 'cfile' : 'file']) || files.find((f) => conflicts || !f.conflict);
      const ordered = [selected, ...files.filter((f) => f !== selected)].filter(Boolean);
      for (const f of automatic ? ordered.slice(0, 1) : ordered) {
        if (!valid()) return;
        if (conflicts && f.resolved || !conflicts && f.conflict || !automatic && f.loaded) continue;
        try {
          const body = await ask(conflicts ? 'project.conflict' : 'project.diff', { ...scope(w), path: f.path, ...(!conflicts ? { staged: f.staged } : {}) }); if (!body) return;
          if (conflicts) {
            const old = wt(id).merge?.files.find((x) => x.id === f.id); if (!old) return;
            if (old.selection?.fingerprint === body.fingerprint && old.selection.head === body.head) body.choices = old.selection.choices;
            writeWt(id, { merge: { ...wt(id).merge, files: wt(id).merge.files.map((x) => x.id === f.id ? { ...mapping.mapConflict(body), loaded: true } : x) } });
          } else writeWt(id, { files: wt(id).files.map((x) => x.id === f.id ? { ...x, ...mapping.mapGitDiff(body), loaded: true, diffError: undefined } : x) });
        } catch { failed(); }
      }
    }
  } catch { failed(); }
  finally { if (r.detailAbort === abort) r.detailAbort = undefined; }
}
export function refreshGit(project = screen?.project, background = false) { return run(background ? 'git-read' : 'git', () => loadGit(project, undefined, !background)); }
export function gitDetail(id, commit) { return run('git', () => loadGitDetail(id, commit)); }
export function selectGit(id, tab, commit) {
  update((s) => { const w = wt(id); s.inspecting[w.project || screen?.project] = w.branch; s.gitUi[id] = { ...s.gitUi[id], ...(tab ? { tab } : {}), ...(commit ? { commit } : {}) }; });
  return gitDetail(id, commit);
}
export function stage(id, ids, staged, all = false) { return run('git', async () => {
  const w = wt(id); local(w); revision++;
  if (all) await request(staged ? 'project.stage.all' : 'project.unstage.all', scope(w));
  else for (const fid of ids) { const f = w.files.find((f) => f.id === fid); if (f) await request(staged ? 'project.stage' : 'project.unstage', { ...scope(w), path: f.path }); }
  await loadGit(screen?.project);
}); }
export function discard(w, files) { return run('git-plan', async () => {
  local(w);
  const plan = read(await request('project.discard.plan', { ...scope(w), paths: [...new Set(files.map((f) => f.path))] }));
  act.confirm({ title: 'Discard changes?', body: 'The planned changes will be discarded. This cannot be undone.', lost: plan.paths, action: 'Discard', onConfirm: () => run('git', async () => {
    revision++; await request('project.discard', { ...scope(w), plan, confirmed: true }); await loadGit(screen?.project);
  }) });
}); }
export function commit(id, message) { return run('git', async () => {
  const w = wt(id); local(w); revision++;
  await request('project.commit', { ...scope(w), message });
  update((s) => { s.gitUi[id] = { ...s.gitUi[id], message: '' }; });
  await loadGit(screen?.project, affectedGit(w), true);
}); }
export function syncGit(id, operation) { return run('git', async () => {
  const w = wt(id); local(w); revision++;
  try { await request('project.' + operation, scope(w)); } finally {
    const ids = operation === 'fetch' ? undefined : affectedGit(w);
    // Pull fetches every origin branch, so every parentless row's upstream count can change.
    if (operation === 'pull') ids.push(...S().worktrees.filter((x) => x.repo === w.repo && !x.parentId && !ids.includes(x.id)).map((x) => x.id));
    await loadGit(screen?.project, ids, true);
  }
}); }
export function integrate(id) { return run('git', async () => {
  const w = wt(id); local(w); const target = destination(w); local(target);
  if (w.kind !== 'team') { refuse(); return; }
  revision++;
  try { await request('project.integrate', { ...scope(w), destinationId: target.id, expectedHead: w.head }); } finally { await loadGit(screen?.project, affectedGit(w, target), true); }
}); }
export function openReview(w, project) { return run('git', async () => {
  local(w); const target = destination(w); local(target); revision++;
  const prepared = read(await request('project.review.prepare', { ...scope(w), projectId: project.id, destinationId: target.id }));
  writeWt(w.id, { review: mapping.mapMainReview(prepared.review, prepared.comparison, project.lead) });
  act.openDialog({ type: 'merge-review', worktreeId: w.id, requestId: prepared.review.reviewId });
}); }
export function answerReview(id, option) {
  if (option !== 'merge') { refuse(); return; }
  return run('git', async () => {
    const w = S().worktrees.find((w) => w.review?.id === id); local(w); const r = w.review; revision++;
    const review = read(await request('project.review.approve', { ...scope(w), projectId: r.project, reviewId: r.id, expectedHead: r.head, expectedBaseHead: r.baseHead, confirmed: true,
      decision: r.evidence.tests.ok && r.evidence.review.ok ? 'approve' : 'approve_without_evidence' }));
    writeWt(w.id, { review: mapping.mapMainReview(review, undefined, r.agent) });
    if (review.outcome === 'merged') act.closeDialog();
    await loadGit(r.project, affectedGit(w, wt(r.destinationId) || destination(w)), true);
    if (review.outcome !== 'merged') throw new Error(review.reason || 'The merge was refused.');
  });
}
export function choose(id, fid, kid, choice) { return run('git', async () => {
  const w = wt(id); local(w); const file = w.merge.files.find((f) => f.id === fid), part = file.parts.find((p) => p.id === kid);
  if (!choice || !file?.selection) { refuse(); return; }
  const conflict = read(await request('project.conflict.choose', { ...scope(w), selection: file.selection, hunk: part.hunk, choice }));
  writeWt(id, { merge: { ...w.merge, files: w.merge.files.map((f) => f.id === fid ? { ...mapping.mapConflict(conflict), loaded: true } : f) } });
}); }
export function resolve(id, fid, value) {
  if (!value) { refuse(); return; }
  return run('git', async () => {
    const w = wt(id); local(w); const f = w.merge.files.find((x) => x.id === fid); revision++;
    await request('project.conflict.resolve', { ...scope(w), selection: f.selection });
    update((s) => { s.gitUi[id] = { ...s.gitUi[id], tab: 'changes' }; });
    await loadGit(screen?.project);
    act.toast('File resolved. Stage it before continuing the merge.');
  });
}
export function continueMerge(id) {
  const message = S().gitUi[id]?.message?.trim();
  if (!message) { act.toast('Write the merge commit message in Changes, then stage the resolved files.', 'error'); return; }
  return commit(id, message);
}
