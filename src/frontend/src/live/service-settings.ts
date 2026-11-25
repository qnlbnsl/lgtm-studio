// The sample never calls these actions. Replies alone change service records.
import { run } from './board';
import * as engine from '../engine';
import { toast } from '../actions';
export const checkWorkflows = () => run('settings', async () => {
  const reply = await engine.request('client.workflows.list', {});
  const user = reply.folders?.find((f: { scope: string }) => f.scope === 'user');
  if (user?.error) throw new Error(user.error);
  toast(`Checked the workflow folder. ${reply.items.filter((w: { scope: string }) => w.scope === 'user').length} workflow files found.`);
});

import { store, engineWrite } from '../lib';
export { busy } from './board';
export const loadVersion = (check = false) => run('service-version', async () => {
  const state = store.state, connection = engine.connection();
  const reply = await engine.request('client.service.version', { check });
  if (store.state !== state || engine.connection() !== connection || state.session.status !== 'in') return;
  engineWrite({ settings: { ...store.state.settings, version: reply.version, commit: reply.commit, dataFolder: reply.dataFolder,
    versionComparison: reply.comparison, versionReason: reply.reason || '', remoteCommit: reply.remoteCommit || '', trackedBranch: reply.trackedBranch || '',
    updateCommands: reply.updateCommands, checkout: reply.checkout } });
});
export const restart = () => run('service-restart', async () => {
  const connection = engine.connection(), state = store.state;
  const reply = await engine.request('client.service.restart', { confirmed: true });
  if (!reply.restarting) throw new Error('The service did not confirm the restart.');
  toast('The service is restarting. Running agents stop and terminals close. Your sign-in stays valid.');
  // A successful session probe keeps the existing sign-in. An expired or
  // revoked session still returns to the existing sign-in flow.
  for (let attempt = 0; attempt < 60; attempt++) {
    await new Promise((resolve) => setTimeout(resolve, 500));
    if (store.state !== state || engine.connection() !== connection) return;
    try {
      const response = await fetch('/api/session', { method: 'POST', credentials: 'same-origin', cache: 'no-store', signal: AbortSignal.timeout(1500) });
      if (response.status === 204) { connection.reconnect!(); toast('The service is responding. Your sign-in is still valid.'); return; }
      if (response.status === 401) { connection.reconnect!(); toast('The service is back. Sign in again.'); return; }
    } catch {}
  }
  toast('The service has not returned yet. Retry the connection when it is available.', 'error');
});
