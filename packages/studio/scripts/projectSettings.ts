export function assertProjectSettingsSource(path: string, source: string): string {
  if (path === 'js/screens/service.js' && !source.includes('projectSettings.loadLead')) throw new Error('Project settings integration: output is missing projectSettings.loadLead')
  return source
}

export function wrapProjectSettingsSwitch(screen: string, label: string): string {
  const row = screen.indexOf('<${Row} label="' + label + '"')
  const start = screen.indexOf('<${Switch}', row)
  const end = screen.indexOf(' />', start)
  if (row < 0 || start < 0 || end < 0) throw new Error(`Project settings integration: ${label} switch moved`)
  const control = screen.slice(start, end + 3)
  return screen.replace(control, '<span title=${live.on ? "Engine work needed" : null}>' + control + '</span>')
}

// The standalone sample is unchanged. Apply these checks to the copied live screen.
export function projectSettingsSource(path: string, source: string): string {
  if (path === 'js/ui.js') {
    const anchor = 'const approvalEffect = r.projectApprovalEffect;';
    if (source.split(anchor).length !== 3) throw new Error('Project approval integration: cards moved');
    return source.replaceAll(anchor, 'const approvalEffect = chatLive.approvalEffect(r);');
  }
  if (path === 'js/data.js') {
    const anchor = "export const STAGES = ['Define', 'Design', 'Plan', 'Build', 'Review', 'Hand over'];";
    if (source.split(anchor).length !== 2) throw new Error('Project stages integration: stages moved');
    return source.replace(anchor, anchor.replace('= [', '= Object.freeze([').replace('];', ']);'));
  }
  if (path === 'js/screens/messages.js') {
    const anchor = 'const archived = ch.archived === true;';
    if (source.split(anchor).length !== 2) throw new Error('Team archive integration: composer moved');
    const mark = '${c.archived && html`<span class="badge">Archived</span>`}';
    if (source.split(mark).length !== 2) throw new Error('Team archive integration: mark moved');
    return source.replace(anchor, 'const archived = board.channelArchived(ch);').replace(mark, mark.replace('c.archived', 'board.channelArchived(c)'));
  }
  if (path !== 'js/screens/service.js') return source
  const start = source.indexOf('export function ProjectSettings(')
  if (start < 0 || source.indexOf('export function ProjectSettings(', start + 1) >= 0) throw new Error('Project settings integration: screen moved')
  const before = source.slice(0, start)
  let screen = source.slice(start)
  const replace = (from: string, to: string, count = 1) => {
    if (screen.split(from).length - 1 !== count) throw new Error(`Project settings integration no longer matches ${path}: ${from}`)
    screen = screen.replaceAll(from, to)
  }
  replace('void board.loadAgents([p.lead]); }, [p.id, p.lead]', 'void projectSettings.loadLead(p.lead); }, [p.lead, lead.model, lead.effort, lead.state]')
  replace('const choices = s.boardAgents?.[p.lead];', 'const choices = s.projectLeadSettings?.[p.lead];')
  replace("const configBusy = live.on && board.busy('configure:' + p.lead);", "const configBusy = live.on && (projectBusy || board.busy('configure:' + p.lead) || !choices || p.archiving);")
  replace('useEffect(() => setGoal(p.goal), [p.id]);', 'useEffect(() => setGoal(p.goal), [p.id, p.goal]);')
  replace("const projectBusy = live.on && board.busy('project:' + p.id);", "const projectBusy = live.on && board.busy('project:' + p.id);\n  const settingsLocked = projectBusy || (live.on && p.archiving);")
  for (const action of ['renameProject', 'saveGoal', 'configureLead', 'deleteTeam', 'changeApproval']) replace('board.' + action, 'projectSettings.' + action, action === 'configureLead' ? 2 : 1)
  // Unsupported callbacks are unreachable behind native disabled controls.
  replace('if (live.on) { refuse(); return; }', 'if (live.on) return;', 7)
  replace('const asg = p.assignment ||', "const asg = live.on ? { state: 'unavailable', rows: {} } : p.assignment ||")
  replace('<span class="tag" data-tip="Task labels start with this">${p.prefix}</span>', '${!live.on && html`<span class="tag" data-tip="Task labels start with this">${p.prefix}</span>`}')
  replace('<${Row} label="Goal"', '${live.on && html`<${Row} label="Task label prefix" id="ps-prefix"><input id="ps-prefix" class="input mono" disabled title="Engine work needed" placeholder="Not set" /><//>`}\n        <${Row} label="Goal"')
  replace('<textarea id="ps-goal"', '<textarea disabled=${settingsLocked} id="ps-goal"')
  replace('disabled=${!goal.trim() || projectBusy}', 'disabled=${!goal.trim() || settingsLocked}')
  replace('disabled=${projectBusy || !name.trim() || name.trim() === p.name}', 'disabled=${settingsLocked || !name.trim() || name.trim() === p.name}')
  replace('value=${name} disabled=${projectBusy}', 'value=${name} disabled=${settingsLocked}')
  replace('disabled=${projectBusy} onClick=${() => changeApproval', 'disabled=${settingsLocked} onClick=${() => changeApproval')
  replace('disabled=${projectBusy || !p.approvals?.endResult}', 'disabled=${settingsLocked || !p.approvals?.endResult}')
  replace('disabled=${projectBusy} onClick=${() => deleteTeam(t)}', 'disabled=${settingsLocked} onClick=${() => deleteTeam(t)}')
  replace('>Save goal</button>', ">${projectBusy ? 'Saving…' : 'Save goal'}</button>")
  replace('<select id="ps-base"', '<select disabled=${live.on} title=${live.on ? "Engine work needed" : null} id="ps-base"')
  replace('${branches.map', '${live.on && html`<option value="">Not set</option>`}${branches.map')
  const machineStart = screen.indexOf('<select id="ps-machine"'), machineEnd = screen.indexOf('</select>', machineStart) + '</select>'.length
  if (machineStart < 0 || machineEnd < '</select>'.length) throw new Error('Project settings integration: machine picker moved')
  const machine = screen.slice(machineStart, machineEnd)
  replace(machine, '${live.on ? html`<span>${m.name || p.machine}</span>` : html`' + machine + '`}')
  replace('<select id="ps-sub"', '<select disabled=${live.on} title=${live.on ? "Engine work needed" : null} id="ps-sub"')
  replace('${subs.map((x)', '${live.on && !subs.some((x) => x.id === lead.subscription) && html`<option value=${lead.subscription}>${lead.subscription || "Not reported"}</option>`}${subs.map((x)')
  replace('value=${lead.model}', 'value=${live.on ? choices?.model || "" : lead.model}')
  replace('value=${lead.effort || \'High\'}', 'value=${live.on ? choices?.effort || "" : lead.effort || \'High\'}')
  replace('${(live.on ? choices?.models || []', '${live.on && !choices?.models?.some((mo) => mo.id === choices?.model) && html`<option value=${choices?.model}>${choices?.model || "Not chosen"}</option>`}${(live.on ? choices?.models || []')
  replace('${(live.on ? choices?.efforts || []', '${live.on && !choices?.efforts?.some((x) => x.id === choices?.effort) && html`<option value=${choices?.effort}>${choices?.effort || "Not chosen"}</option>`}${(live.on ? choices?.efforts || []')
  replace('choices?.efforts.length > 0', 'choices?.efforts?.length > 0')
  replace('          </div><//>\n      </div>\n    <//>\n    <${Section} title="Agents"', `          </div>
          \${live.on && html\`<span role="status" class="muted">\${board.busy('configure:' + p.lead) ? 'Saving the lead setting…' : choices ? choices.statusLabel + (choices.applied?.model ? ' · applied ' + choices.applied.model : '') : 'Lead configuration unavailable.'}</span>\`}<//>
      </div>
    <//>
    <\${Section} title="Agents"`)
  replace('<${PermissionPicker} name="ps-perm" value=${perm} onChange=${setPerm} />', '<fieldset disabled=${live.on} title=${live.on ? "Engine work needed" : null} style="border:0;margin:0;padding:0;min-width:0"><${PermissionPicker} name="ps-perm" value=${perm} onChange=${setPerm} /></fieldset>')
  for (const label of ['Write tests first', 'Agent review', 'Remove agent credits when merging']) {
    const marker = 'label="' + label + '" onChange='
    replace(marker, 'disabled=${live.on} label="' + label + '" onChange=')
    screen = wrapProjectSettingsSwitch(screen, label)
  }
  replace('class="btn sm" onClick=${approve}', 'class="btn sm" disabled=${live.on} title=${live.on ? "Engine work needed" : null} onClick=${approve}')
  replace('(live.on ? [] : WORK_KINDS).map((k) => { const row', `WORK_KINDS.map((k) => { if (live.on) return html\`<tr key=\${k}><td><b style="font-weight:var(--weight-medium)">\${k}</b></td>\${['harness', 'model', 'effort'].map((axis) => html\`<td><select class="select" aria-label=\${k + ' ' + axis} disabled title="Engine work needed"><option>Not set</option></select></td>\`)}</tr>\`; const row`)
  replace('<select id="ps-all"', '<select disabled=${live.on} title=${live.on ? "Engine work needed" : null} id="ps-all"')
  replace('${Object.entries(HARNESSES).filter(([id]) => !live.on', '${live.on && html`<option value="">Not set</option>`}${Object.entries(HARNESSES).filter(([id]) => !live.on')
  replace('class="btn sm" onClick=${applyAll}', 'class="btn sm" disabled=${live.on} title=${live.on ? "Engine work needed" : null} onClick=${applyAll}')
  replace('<${Switch} on=${t.needToKnow}', '<span title=${live.on ? "Engine work needed" : null}><${Switch} disabled=${live.on} on=${t.needToKnow}')
  replace('onChange=${(v) => setTeam(t, { needToKnow: v })} /></td>', 'onChange=${(v) => setTeam(t, { needToKnow: v })} /></span></td>')
  replace('class="btn danger" onClick=${del}', 'class="btn danger" disabled=${live.on} title=${live.on ? "Engine work needed" : null} onClick=${del}')
  return assertProjectSettingsSource(path, "import * as projectSettings from '../live/project-settings.js';\n" + before + screen)
}
