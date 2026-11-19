// The same four controls run on sample data and on the authenticated website.
import { html, useState, useEffect, live } from './lib.js';
import { HARNESSES, usableSubscription } from './data.js';
import * as liveGit from './live/git.js';
import { loadProjectLeadChoices } from './live/board.js';

export function useProjectLead(s, machine) {
  const accounts = s.subscriptions.filter((a) => a.enabled && usableSubscription(a) && a.machine === machine && HARNESSES[a.harness]);
  const defaults = s.settings.projectLead || s.setupData?.lead;
  const serviceReady = !live.on || s.ui.serviceReady === true;
  const [initialized, setInitialized] = useState(serviceReady);
  const initial = () => {
    const harness = defaults?.harness || s.settings.defaultHarness;
    const account = accounts.find((a) => a.harness === harness && (defaults?.account?._tag === 'named' ? a.id === defaults.account.accountId : a.useForNew))
      || accounts.find((a) => a.harness === harness) || accounts[0];
    return { harness: account?.harness || '', accountId: account?.id || '',
      model: defaults?.model || s.settings.defaultModel || '', effort: defaults?.effort || s.settings.defaultEffort || '' };
  };
  const [selected, setSelected] = useState(initial);
  useEffect(() => { if (live.on) void liveGit.run('read-project-lead', () => liveGit.refreshAccounts()); }, []);
  useEffect(() => {
    if (serviceReady && !initialized) { setSelected(initial()); setInitialized(true); }
  }, [serviceReady, initialized]);
  const [report, setReport] = useState(null);
  const harness = HARNESSES[selected.harness];
  const key = JSON.stringify([selected.harness, selected.accountId, selected.model]);
  const available = accounts.filter((a) => a.harness === selected.harness);
  const accountValid = available.some((a) => a.id === selected.accountId);
  useEffect(() => {
    if (!initialized || accountValid || !accounts.length) return;
    const account = available[0] || accounts[0];
    setSelected({ harness: account.harness, accountId: account.id, model: '', effort: '' });
  }, [initialized, machine, accountValid, accounts.map((a) => a.id + ':' + a.harness).join('|')]);
  useEffect(() => {
    if (!live.on || !initialized || !accountValid) return;
    let current = true;
    loadProjectLeadChoices(selected.harness, selected.accountId, selected.model).then((choices) => {
      if (current) setReport({ key, ...choices });
    }).catch(() => {
      if (current) setReport({ key, models: { options: [] }, efforts: { options: [] }, reason: 'The harness could not report its models on this subscription.', effortReason: 'The harness could not report its effort choices on this subscription.' });
    });
    return () => { current = false; };
  }, [key, initialized, accountValid]);
  const pending = live.on && accountValid && report?.key !== key;
  const current = report?.key === key ? report : null;
  const options = (axis) => {
    const rows = live.on ? current?.[axis]?.options || [] : (harness?.[axis] || []).map((id) => ({ id, label: id }));
    const allowed = harness?.[axis === 'models' ? 'configuredModels' : 'configuredEfforts'];
    return allowed?.length ? rows.filter((o) => allowed.includes(o.id)) : rows;
  };
  const models = options('models');
  const efforts = !live.on && harness?.noEffort?.includes(selected.model) ? [] : options('efforts');
  // A different subscription or model can report a different set of efforts.
  // Never submit an old option while its replacement report is pending.
  const model = models.some((o) => o.id === selected.model) ? selected.model : '';
  const effort = efforts.some((o) => o.id === selected.effort) ? selected.effort : '';
  const why = !initialized ? 'Reading the project lead settings…' : !accountValid ? 'Choose a signed-in subscription for the project lead.' : pending ? 'Reading the project lead choices…' : null;
  const lead = accountValid ? { harness: selected.harness, account: { _tag: 'named', accountId: selected.accountId },
    ...(model ? { model } : {}), ...(effort ? { effort } : {}), permission: defaults?.permission || 'auto' } : undefined;
  const changeHarness = (id) => {
    const account = accounts.find((a) => a.harness === id);
    setSelected({ harness: id, accountId: account?.id || '', model: '', effort: '' });
  };
  const fields = html`<div class="field"><span class="label">Project lead</span>
    <div style="display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:var(--space-3)">
      <div class="field" style="min-width:0"><label for="np-lead-harness">Harness</label>
        <select id="np-lead-harness" class="select" value=${selected.harness} onChange=${(e) => changeHarness(e.target.value)} disabled=${!initialized || !accounts.length}>
          ${!accounts.length && html`<option value="">No signed-in harness</option>`}
          ${[...new Set(accounts.map((a) => a.harness))].map((id) => html`<option value=${id}>${HARNESSES[id].name}</option>`)}</select></div>
      <div class="field" style="min-width:0"><label for="np-lead-account">Subscription</label>
        <select id="np-lead-account" class="select" value=${selected.accountId} disabled=${!initialized || !available.length} onChange=${(e) => setSelected({ ...selected, accountId: e.target.value, model: '', effort: '' })}>
          ${!available.length && html`<option value="">No signed-in subscription</option>`}
          ${available.map((a) => html`<option value=${a.id}>${a.name}</option>`)}</select></div>
      <div class="field" style="min-width:0"><label for="np-lead-model">Model</label>
        <select id="np-lead-model" class="select" value=${model} disabled=${pending || !models.length} onChange=${(e) => setSelected({ ...selected, model: e.target.value, effort: '' })}>
          <option value="">Harness default</option>${models.map((o) => html`<option value=${o.id}>${o.label || o.id}</option>`)}</select>
        ${live.on && !pending && !models.length && html`<span class="muted">${current?.reason || 'This harness did not report available models.'}</span>`}</div>
      <div class="field" style="min-width:0"><label for="np-lead-effort">Effort</label>
        <select id="np-lead-effort" class="select" value=${effort} disabled=${pending || !efforts.length} onChange=${(e) => setSelected({ ...selected, effort: e.target.value })}>
          <option value="">Harness default</option>${efforts.map((o) => html`<option value=${o.id}>${o.label || o.id}</option>`)}</select>
        ${live.on && !pending && !efforts.length && html`<span class="muted">${current?.effortReason || 'This model did not report an effort setting.'}</span>`}</div>
    </div><span class="muted" aria-live="polite">${pending ? 'Reading the project lead choices…' : 'These choices apply to this project.'}</span>
  </div>`;
  return { lead, why, fields };
}
