// The same four controls run on sample data and on the authenticated website.
import { html, useState, useEffect, live, Icon } from './lib.js';
import { HARNESSES, usableSubscription } from './data.js';
import { Menu } from './ui.js';
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
  // The engine treats the permission of a requested lead as final. It comes from the saved default as the engine reports it.
  const changeHarness = (id) => {
    const account = accounts.find((a) => a.harness === id);
    setSelected({ harness: id, accountId: account?.id || '', model: '', effort: '' });
  };
  const selectedAccount = available.find((a) => a.id === selected.accountId);
  const modelLabel = models.find((o) => o.id === model)?.label || model || 'Model';
  const effortLabel = efforts.find((o) => o.id === effort)?.label || effort || 'Effort';
  const trigger = (label) => html`<span class="truncate" title=${label}>${label}</span><${Icon} name="chevron-down" size=${12} />`;
  const fields = html`<div class="field new-project-lead"><span class="label">Project lead</span>
    <div class="new-project-lead-row" role="group" aria-label="Project lead">
      <${Menu} class="pick" label="Harness" disabled=${!initialized || !accounts.length} trigger=${trigger(harness?.name || 'Harness')}
        items=${[...new Set(accounts.map((a) => a.harness))].map((id) => ({ label: HARNESSES[id].name, checked: id === selected.harness, onSelect: () => changeHarness(id) }))} />
      <${Menu} class="pick" label="Subscription" disabled=${!initialized || !available.length} trigger=${trigger(selectedAccount?.name || 'Subscription')}
        items=${available.map((a) => ({ label: a.name, hint: a.id, checked: a.id === selected.accountId, onSelect: () => setSelected({ ...selected, accountId: a.id, model: '', effort: '' }) }))} />
      <${Menu} class="pick" label="Model" disabled=${pending || !models.length} trigger=${trigger(modelLabel)}
        items=${[{ label: 'Harness default', checked: !model, onSelect: () => setSelected({ ...selected, model: '', effort: '' }) }, ...models.map((o) => ({ label: o.label || o.id, checked: o.id === model, onSelect: () => setSelected({ ...selected, model: o.id, effort: '' }) }))]} />
      <${Menu} class="pick" label="Effort" disabled=${pending || !efforts.length} trigger=${trigger(effortLabel)}
        items=${[{ label: 'Harness default', checked: !effort, onSelect: () => setSelected({ ...selected, effort: '' }) }, ...efforts.map((o) => ({ label: o.label || o.id, checked: o.id === effort, onSelect: () => setSelected({ ...selected, effort: o.id }) }))]} />
    </div>
    ${pending && html`<span class="muted" aria-live="polite">Reading the project lead choices…</span>`}
    ${live.on && !pending && !models.length && html`<span class="muted">${current?.reason || 'This harness did not report available models.'}</span>`}
    ${live.on && !pending && !efforts.length && html`<span class="muted">${current?.effortReason || 'This model did not report an effort setting.'}</span>`}
  </div>`;
  return { lead, why, fields };
}
