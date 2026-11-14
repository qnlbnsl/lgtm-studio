// Project settings use the existing engine commands. Only returned facts enter the store.
import { store, live, engineWrite } from '../lib.js';
import * as engine from '../engine.js';
import { run } from './board.js';
export { renameProject, saveGoal } from './board.js';
let mapping;
export function installMappings(value) { mapping = value; }
const valid = (state, connection) => live.on && store.state === state && engine.connection() === connection && state.session.status === 'in';
export function loadLead(agentId) {
  if (!mapping) return Promise.resolve();
  return run('read-project-lead:' + agentId, async () => {
    const state = store.state, connection = engine.connection();
    try {
      const report = await engine.request('client.agent.configuration', { agentId });
      if (valid(state, connection)) {
        const settings = mapping.mapLeadSettings(report);
        engineWrite({ boardAgents: { ...state.boardAgents, [agentId]: { ...state.boardAgents?.[agentId], ...settings, settings } } });
      }
      return report;
    } catch (error) {
      if (valid(state, connection)) engineWrite({ boardAgents: { ...state.boardAgents, [agentId]: { ...state.boardAgents?.[agentId], settings: null } } });
      throw error;
    }
  });
}
export function configureLead(agentId, patch) {
  return run('configure:' + agentId, async () => {
    const state = store.state, connection = engine.connection();
    try { return await engine.request('configure', { agentId, ...patch }); }
    finally { if (valid(state, connection)) await loadLead(agentId); }
  }, 'Lead configuration requested');
}
