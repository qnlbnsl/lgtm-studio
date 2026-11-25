import { Icon } from './lib';
import { Menu } from './ui';

/** One place a terminal can open, with the reason it cannot when it is off. */
export interface TerminalOption { value: string; label: string; folder: string; reason?: string | null | undefined }
export interface TerminalLaunchProps {
  options: TerminalOption[]; onOpen: (value: string) => void; disabled?: boolean; className?: string;
  /** The control is a plus alone, and its name is a tooltip. The row of sessions uses it. */
  iconOnly?: boolean;
}

// The mock and the connected screen use the same control.
// "New session" asks where to open. With one place to open, it opens there at once.
export function TerminalLaunch({ options, onOpen, disabled = false, className = 'btn sm', iconOnly = false }: TerminalLaunchProps) {
  const face = iconOnly ? <Icon name="plus" size={15} /> : <><Icon name="plus" size={14} />New session</>;
  const only = options.length === 1 ? options[0]! : null;
  if (only || !options.length) {
    return <button type="button" class={className} disabled={disabled || !only || !!only.reason} aria-label={iconOnly ? 'New session' : undefined} data-tip={only?.reason || (iconOnly ? 'New session' : null)} onClick={() => only && onOpen(only.value)}>{face}</button>;
  }
  return <Menu class={className} label={iconOnly ? 'New session' : undefined} tip={iconOnly ? 'New session' : undefined} disabled={disabled} width={340} trigger={face} items={[
    { heading: 'Open in' },
    ...options.map((option) => ({ label: option.label, icon: 'square-terminal', disabled: !!option.reason, hint: <span class="mono">{option.folder}</span>, why: option.reason, onSelect: () => onOpen(option.value) })),
  ]} />;
}
