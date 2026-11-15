// Project settings use the existing engine commands. Only returned facts enter the store.
import { store, live, engineWrite } from '../lib.js';
import * as engine from '../engine.js';
import { run } from './board.js';
export { saveGoal } from './board.js';
export function renameProject(projectId, name) {
  return run('project:' + projectId, () => engine.request('project.rename', mapping.projectRenamePayload(projectId, name)), 'Project renamed');
}
export function deleteProject(projectId) {
  return run('project:' + projectId, () => engine.request('project.delete', mapping.projectDeletePayload(projectId)), 'Project deleted');
}
export function deleteTeam(projectId, teamId) {
  return run('project:' + projectId, () => engine.request('project.team.delete', mapping.teamDeletePayload(projectId, teamId)), 'Team deleted');
}
export function changePermission(projectId, permission, setting) {
  return run('project:' + projectId, () => engine.request('project.permission.set', mapping.projectPermissionPayload(projectId, permission, setting)), 'Lead permission saved');
}
export function changeApproval(projectId, approval, approved) {
  return run('project:' + projectId, () => engine.request('project.approval.' + (approved ? 'grant' : 'withdraw'), mapping.projectApprovalPayload(projectId, approval)), approved ? 'Permission granted' : 'Permission withdrawn');
}
let mapping;
const latestReads = new WeakMap();
export function installMappings(value) { mapping = value; }
const valid = (state, connection) => live.on && store.state === state && engine.connection() === connection && state.session.status === 'in';
export function loadLead(agentId, { afterWrite = false } = {}) {
  if (!mapping) return Promise.resolve();
  // A post-write read must run even while a background read is still pending.
  return run((afterWrite ? 'read-project-lead-after-write:' : 'read-project-lead:') + agentId, async () => {
    const state = store.state, connection = engine.connection();
    const versions = latestReads.get(state) || new Map();
    latestReads.set(state, versions);
    const version = (versions.get(agentId) || 0) + 1;
    versions.set(agentId, version);
    const current = () => valid(state, connection) && versions.get(agentId) === version;
    try {
      const report = await engine.request('client.agent.configuration', { agentId });
      if (current()) engineWrite({ projectLeadSettings: { ...state.projectLeadSettings, [agentId]: mapping.mapLeadSettings(report) } });
      return report;
    } catch (error) {
      // Keep existing choices usable if a background refresh fails.
      if (current() && !state.projectLeadSettings?.[agentId]) engineWrite({ projectLeadSettings: { ...state.projectLeadSettings, [agentId]: null } });
      throw error;
    }
  });
}
export function configureLead(agentId, patch) {
  return run('configure:' + agentId, async () => {
    const state = store.state, connection = engine.connection();
    try { return await engine.request('configure', { agentId, ...patch }); }
    finally { if (valid(state, connection)) await loadLead(agentId, { afterWrite: true }); }
  }, 'Lead configuration requested');
}
