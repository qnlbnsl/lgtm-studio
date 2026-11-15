// The lead's requested configuration and applied configuration are separate facts.
import type { AgentChoices, AgentConfiguration, ProjectId, TeamId } from '@lgtm/contracts'
export function mapLeadSettings(report: AgentChoices & { readonly configuration: AgentConfiguration }) {
  const options = (list: AgentChoices['models']) => list.options.map((option) => ({ id: option.id, label: option.label ?? option.id }))
  return {
    model: report.configuration.desired.model ?? '', effort: report.configuration.desired.effort ?? '',
    applied: report.configuration.applied, status: report.configuration.status,
    statusLabel: report.configuration.status === 'applied' ? 'Applied' : report.configuration.status === 'pending' ? 'Waiting for the agent' : 'Not applied',
    reason: report.configuration.reason ?? '',
    models: options(report.models), efforts: options(report.efforts),
    modelReason: report.models.reason ?? '', effortReason: report.efforts.reason ?? ''
  }
}

export const projectRenamePayload = (projectId: string, name: string) => ({ projectId: projectId as ProjectId, name: name.trim() })
export const teamDeletePayload = (projectId: string, teamId: string) => ({ projectId: projectId as ProjectId, teamId: teamId as TeamId })
