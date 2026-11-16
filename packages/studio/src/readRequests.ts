// Only named readers may wait. New or unknown commands keep immediate write behaviour.
import type { Client } from "@lgtm/client/browser"

const reads = new Set<string>([
  'client.snapshot', 'client.roster', 'client.requests', 'client.thread', 'client.event', 'client.sequence',
  'client.agent.configuration', 'client.agent.context', 'client.web.settings',
  'client.alerts.settings', 'client.alerts.list', 'client.harness.status', 'client.harness.recipe', 'client.harness.choices',
  'client.service.version', 'client.workflows.list', 'client.workflows.read', 'client.workflows.validate',
  'roles.settings.get', 'roles.role-file.get', 'roles.actions.list',
  'board.channels', 'board.peek', 'board.read',
  'project.settings', 'project.setup', 'project.setup.discover', 'project.placement.preview',
  'project.repositories', 'project.workspaces', 'project.read', 'project.requests', 'project.task.history',
  'project.context.search', 'project.files.list', 'project.files.read',
  'project.git.state', 'project.changes', 'project.diff', 'project.branches',
  'project.history', 'project.commit.files', 'project.conflict', 'project.conflict.resolved', 'project.review.status', 'project.discard.plan',
  'project.mocks.read', 'project.mocks.reviews.read', 'project.mocks.artifact', 'project.design.read'
] satisfies ReadonlyArray<keyof Client>)
export const isReadRequest = (tag: string, payload: object): boolean => reads.has(tag)
  || (tag === 'accounts' || tag === 'harnesses') && 'operation' in payload && payload.operation === 'list'
