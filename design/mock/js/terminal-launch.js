import { html, Icon } from './lib.js';

// The mock and the connected screen use the same labelled control.
export function TerminalLaunch({ options, value, onChange, onOpen, disabled = false, className = 'btn sm' }) {
  const selected = options.find(option => option.value === value);
  return html`<div class="term-launch">
    <label>Open in <select value=${value} disabled=${disabled || !options.length} onChange=${event => onChange(event.currentTarget.value)}>
      ${options.map(option => html`<option key=${option.value} value=${option.value} disabled=${!!option.reason}>${option.label} · ${option.folder}${option.reason ? ' — ' + option.reason : ''}</option>`)}
    </select></label>
    <button type="button" class=${className} disabled=${disabled || !selected || !!selected.reason} onClick=${onOpen}><${Icon} name="plus" size=${14} />New session</button>
  </div>`;
}
