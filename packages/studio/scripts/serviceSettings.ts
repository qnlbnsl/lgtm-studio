import { removeSettingsRow } from './settingsControls.ts'

// Wire the copied live screen without changing the frozen sample design.
export function serviceSettingsSource(path: string, source: string): string {
  const replace = (from: string, to: string, count = 1) => {
    if (source.split(from).length - 1 !== count) throw new Error(`Service settings integration no longer matches ${path}: ${from}`)
    source = source.replaceAll(from, to)
  }
  if (path === 'js/live/git.js') {
    source = "import * as serviceSettings from './service-settings.js';\n" + source
    replace('subscriptionsLoaded = true; S().ui.serviceReady = true; notify();', 'subscriptionsLoaded = true; S().ui.serviceReady = true; notify();\n  await serviceSettings.loadVersion();')
    replace("} else if (['defaultHarness'", "} else if (['workflowPath', 'reviewers', 'differentVendor'].includes(key)) {\n    if (!service.project) throw new Error('Configure the project workspace and shell first.');\n    await request('project.settings.set', { ...(key === 'workflowPath' && !patch.workflowPath ? { clearWorkflowFolder: true } : {}), settings: { ...service.project, ...(key === 'workflowPath' ? (patch.workflowPath ? { workflowFolder: patch.workflowPath } : {}) : { reviewDefaults: { reviewers: 2, differentVendor: true, ...service.project.reviewDefaults, [key]: patch[key] } }) } });\n  } else if (['defaultHarness'")
    replace("['stallWatch', 'standingRules'].includes(key)", "['stallWatch', 'standingRules', 'assignedStallMinutes', 'leadStallMinutes', 'autoNudge', 'nudgeAttempts', 'replaceAfter'].includes(key)")
    replace("const key = Object.keys(patch)[0];", "if ('stallAssigned' in patch) patch = { assignedStallMinutes: patch.stallAssigned };\n  if ('stallLead' in patch) patch = { leadStallMinutes: patch.stallLead };\n  const key = Object.keys(patch)[0];")
  }
  if (path === 'js/screens/service.js') {
    source = "import * as serviceSettings from '../live/service-settings.js';\n" + source
    replace('onChange=${(e) => setS({ workspaceFolder: e.target.value })}', 'onChange=${(e) => setS({ workspaceFolder: e.target.value })} disabled=${live.on && !s.ui.serviceReady}')
    replace('The service stall time. Role files can override it.', 'Idle with unanswered board work. Active turns use the separate runaway timer. Role files can override this value.')
    replace('<${Row} label="Assigned, not started" id=', '<${Row} label="Assigned, not started" hint="Idle with unanswered board work before the first recorded turn." id=')
    replace('<${Row} label="Lead, no output" id=', '<${Row} label="Lead, no output" hint="Idle with unanswered board work. Role files can override this value." id=')
    replace('onInput=${(e) => setS({ workflowPath: e.target.value })}', 'onChange=${(e) => setS({ workflowPath: e.target.value })}')
    replace('<button type="button" class="btn" onClick=${() => { if (live.on) return refuse(); setChecked', '<button type="button" class="btn" onClick=${() => setS({ workflowPath: String() })}>Use the default folder</button><button type="button" class="btn" onClick=${() => { if (live.on) return refuse(); setChecked')
    replace("if (live.on) return refuse(); setChecked('loading');", "if (live.on) return serviceSettings.checkWorkflows(); setChecked('loading');")
    replace('<${Row} label="Write tests first" hint="New projects', '<${Row} label="Default reviewers" id="wf-reviewers" hint="Saved for future reviewer assignment. Automatic reviewer selection is not built yet."><${Num} id="wf-reviewers" value=${set.reviewers} min=${1} max=${9} onChange=${(v) => setS({ reviewers: v })} /><//>\n    <${Row} label="Reviewer from another vendor" hint="Saved for future reviewer assignment. Automatic reviewer selection is not built yet."><${Switch} on=${set.differentVendor} label="Reviewer from another vendor" onChange=${(v) => setS({ differentVendor: v })} /><//>\n    <${Row} label="Write tests first" hint="New projects')
    replace('<fieldset aria-busy=', "<fieldset disabled=${live.on && (serviceSettings.busy('settings') || serviceSettings.busy('service-restart'))} aria-busy=")
    replace("<fieldset disabled=${live.on && liveGit.busy('subscriptions')}", "<fieldset disabled=${live.on && (liveGit.busy('subscriptions') || serviceSettings.busy('settings') || serviceSettings.busy('service-restart'))}")
    replace("liveGit.busy('settings')", "(liveGit.busy('settings') || serviceSettings.busy('settings') || serviceSettings.busy('service-restart'))", 3)
    replace("if (act.refused()) return; setChk('loading');", "return serviceSettings.loadVersion(true); setChk('loading');")
    replace("chk === 'loading' ? 'Checking…'", "serviceSettings.busy('service-version') ? 'Checking…'")
    replace("${chk === 'done' &&", "${set.versionComparison === 'same' &&")
    replace('<${Row} label="Service"><span', "${set.versionReason && html`<p class=\"muted\">${set.versionReason}</p>`}${set.versionComparison === 'different' && html`<p class=\"muted\">The tracked remote branch differs from the running commit. Review the changes before updating.</p>`}\n    <${Row} label=\"Service\"><span")
    replace('Agents keep their place. Web connections close and reconnect.', 'Web connections close and reconnect. You stay signed in. Terminals close.')
    replace('Agents stop at the end of their turn and resume after the restart. This page reconnects on its own.', 'Running agents are stopped and terminals close. This page reconnects and stays signed in.')
    replace("if (act.refused()) return; act.setConnection('reconnecting');", "return serviceSettings.restart(); act.setConnection('reconnecting');")
    replace('<${Row} label="Restart the service"', '<${Row} label="Update the service" hint="Run these commands yourself in the checkout shown below. Review local changes and remote commits before merging." top><span class="mono">${set.checkout}</span><pre class="code">${(set.updateCommands || []).join(String.fromCharCode(10))}</pre><button type="button" class="btn" onClick=${refuse}>Update the service</button><//>\n    <${Row} label="Restart the service"')
    replace('<${Row} label="Wake it once first"', '<${Row} label="Nudge a stalled agent automatically"')
    replace('label="Wake it once first" onChange=', 'label="Nudge a stalled agent automatically" onChange=')
    replace('A stuck agent gets one wake. Ten minutes later its lead is told. A stuck lead goes to the project lead, and a stuck project lead comes to you.', 'A stalled agent receives up to the saved number of nudges. Further stalls go to its lead. Replacement is advice to the lead.')
    replace('<${Row} label="Replace the worker"', '<${Row} label="Automatic nudges" id="sd-n"><${Num} id="sd-n" value=${set.nudgeAttempts} min=${0} max=${9} unit="at most" onChange=${(v) => setS({ nudgeAttempts: v })} /><//>\n      <${Row} label="Replace the worker"')
    // Reuse Project settings' checked removal of whole rows in the live copy.
    const removeFrom = (name: string, next: string, labels: string[]) => {
      const start = source.indexOf('function ' + name + '('), end = source.indexOf('function ' + next + '(', start)
      if (start < 0 || end < start) throw new Error(`Service settings integration: ${name} moved`)
      let screen = source.slice(start, end)
      for (const label of labels) screen = removeSettingsRow(screen, label, 'Service settings integration')
      source = source.slice(0, start) + screen + source.slice(end)
    }
    removeFrom('Defaults', 'Remote', ['Need-to-know messages', 'Remove agent credits when merging'])
    removeFrom('StallWatch', 'WorkflowSettings', ['Standing permission rules'])
    replace('<h3>Approvals</h3>', '')
    removeFrom('WorkflowSettings', 'GitMessages', ['Write tests first', 'Agent review', 'Review rounds'])
    // The message model was the only control in this section.
    const gitStart = source.indexOf('function GitMessages('), gitEnd = source.indexOf('function About(', gitStart)
    if (gitStart < 0 || gitEnd < gitStart) throw new Error('Service settings integration: Git messages moved')
    source = source.slice(0, gitStart) + source.slice(gitEnd)
    replace("  ['git', 'Git messages', 'git-commit-horizontal', GitMessages, 'The model that writes commit and pull request messages.'],", '')
    replace('<button type="button" class="btn" onClick=${refuse}>Update the service</button>', '')

  }
  return source
}
